package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/wojas/genericr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
)

type UncachedObjects []ctrlclient.Object

func ProvideUncachedObjects() UncachedObjects {
	return nil
}

type TiltServerControllerManager struct {
	config          *rest.Config
	scheme          *runtime.Scheme
	deferredClient  *DeferredClient
	uncachedObjects UncachedObjects

	manager ctrl.Manager
	cancel  context.CancelFunc
}

var _ store.SetUpper = &TiltServerControllerManager{}
var _ store.Subscriber = &TiltServerControllerManager{}
var _ store.TearDowner = &TiltServerControllerManager{}

func NewTiltServerControllerManager(config *server.APIServerConfig, scheme *runtime.Scheme, deferredClient *DeferredClient, uncachedObjects UncachedObjects) (*TiltServerControllerManager, error) {
	return &TiltServerControllerManager{
		config:          config.GenericConfig.LoopbackClientConfig,
		scheme:          scheme,
		deferredClient:  deferredClient,
		uncachedObjects: uncachedObjects,
	}, nil
}

func (m *TiltServerControllerManager) GetManager() ctrl.Manager {
	return m.manager
}

func (m *TiltServerControllerManager) GetClient() ctrlclient.Client {
	if m.manager == nil {
		return nil
	}
	return m.manager.GetClient()
}

func (m *TiltServerControllerManager) SetUp(ctx context.Context, st store.RStore) error {
	ctx, m.cancel = context.WithCancel(ctx)

	// controller-runtime internals don't really make use of verbosity levels, so in lieu of a better
	// mechanism, all its logs are redirected to a custom logger that filters out logs
	// we don't care about.
	//
	// V(3) was picked because while controller-runtime is a bit chatty at startup, once steady state
	// is reached, most of the logging is generally useful (e.g. reconciler errors)
	ctxLog := logger.Get(ctx)
	logr := genericr.New(func(e genericr.Entry) {
		// We don't care about the startup or teardown sequence.
		if e.Message == "Starting EventSource" ||
			e.Message == "error received after stop sequence was engaged" {
			return
		}
		if e.Level <= 3 {
			ctxLog.Debugf("%s", e.String())
		}
	})
	timeout := time.Duration(0)

	mgr, err := ctrl.NewManager(m.config, ctrl.Options{
		Scheme: m.scheme,
		// controller manager lazily listens on a port if a webhook is registered; this functionality
		// is currently NOT used; to prevent it from listening on a default port (9443) and potentially
		// causing conflicts running multiple instances of tilt, this is set to an invalid value
		Port: -1,
		// disable metrics + health probe by setting them to "0"
		MetricsBindAddress:     "0",
		HealthProbeBindAddress: "0",
		// leader election is unnecessary as a single manager instance is run in-process with
		// the apiserver
		LeaderElection:   false,
		LeaderElectionID: "tilt-apiserver-ctrl",

		ClientDisableCacheFor:   m.uncachedObjects,
		Logger:                  logr,
		GracefulShutdownTimeout: &timeout,
	})
	if err != nil {
		return fmt.Errorf("unable to create controller manager: %v", err)
	}

	// provide the deferred client with the real client now that it has been initialized
	m.deferredClient.initialize(mgr.GetClient())

	go func() {
		if err := mgr.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			err = fmt.Errorf("controller manager stopped unexpectedly: %v", err)
			st.Dispatch(store.NewErrorAction(err))
		}
	}()

	m.manager = mgr

	return nil
}

func (m *TiltServerControllerManager) TearDown(_ context.Context) {
	if m.cancel != nil {
		m.cancel()
	}
}

// OnChange is a no-op but used to get initialized in upper along with the API server
func (m *TiltServerControllerManager) OnChange(_ context.Context, _ store.RStore, _ store.ChangeSummary) error {
	return nil
}

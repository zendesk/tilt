package controllers

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/tilt-dev/tilt/internal/controllers/core"
	"github.com/tilt-dev/tilt/internal/store"
	manifests "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type TiltServerControllerManager struct {
	store         store.RStore
	apiserverHost model.WebHost
	apiserverPort model.WebPort
}

func ProvideTiltServerControllerManager(store store.RStore, apiserverHost model.WebHost, apiserverPort model.WebPort) *TiltServerControllerManager {
	return &TiltServerControllerManager{
		store:         store,
		apiserverHost: apiserverHost,
		apiserverPort: apiserverPort,
	}
}

func (m *TiltServerControllerManager) Start(ctx context.Context) error {
	scheme := runtime.NewScheme()

	w := logger.Get(ctx).Writer(logger.DebugLvl)
	ctrl.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(w)))

	utilruntime.Must(manifests.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme

	config := &rest.Config{
		Host: fmt.Sprintf("http://%s:%d", string(m.apiserverHost), int(m.apiserverPort)),
	}

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:           scheme,
		Port:             9443,
		LeaderElection:   false,
		LeaderElectionID: "b69659b9.",
	})
	if err != nil {
		return fmt.Errorf("unable to start manager: %v", err)
	}

	if err = (&core.ManifestReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("core").WithName("Manifest"),
		Scheme: mgr.GetScheme(),
		Store:  m.store,
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create %q controller: %v", "Manifest", err)
	}

	go func() {
		if err := mgr.Start(ctx.Done()); err != nil {
			m.store.Dispatch(store.NewErrorAction(err))
		}
	}()

	return nil
}

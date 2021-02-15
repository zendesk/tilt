package server

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	genericapiserver "k8s.io/apiserver/pkg/server"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/start"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/assets"
	"github.com/tilt-dev/tilt/pkg/model"
)

type HeadsUpServerController struct {
	port             model.WebPort
	hudServer        *HeadsUpServer
	assetServer      assets.Server
	webURL           model.WebURL
	initDone         bool
	apiServerOptions *start.TiltServerOptions

	shutdown   func()
	shutdownCh <-chan struct{}
}

func ProvideHeadsUpServerController(
	port model.WebPort,
	apiServerOptions *start.TiltServerOptions,
	hudServer *HeadsUpServer,
	assetServer assets.Server,
	webURL model.WebURL) *HeadsUpServerController {

	emptyCh := make(chan struct{})
	close(emptyCh)

	return &HeadsUpServerController{
		port:             port,
		hudServer:        hudServer,
		assetServer:      assetServer,
		webURL:           webURL,
		apiServerOptions: apiServerOptions,
		shutdown:         func() {},
		shutdownCh:       emptyCh,
	}
}

func (s *HeadsUpServerController) TearDown(ctx context.Context) {
	s.shutdown()
	<-s.shutdownCh
	s.assetServer.TearDown(ctx)
}

// Merge the APIServer and the Tilt Web server into a single handler,
// and attach them both to the public listener.
//
// TODO(nick): We could move this to SetUp if SetUp had error-handling.
func (s *HeadsUpServerController) Start(ctx context.Context) error {
	stopCh := ctx.Done()
	o := s.apiServerOptions
	config, err := o.Config()
	if err != nil {
		return err
	}

	server, err := config.Complete().New()
	if err != nil {
		return err
	}

	err = server.GenericAPIServer.AddPostStartHook("start-tilt-server-informers", func(context genericapiserver.PostStartHookContext) error {
		if config.GenericConfig.SharedInformerFactory != nil {
			config.GenericConfig.SharedInformerFactory.Start(context.StopCh)
		}
		return nil
	})
	if err != nil {
		return err
	}

	prepared := server.GenericAPIServer.PrepareRun()
	apiserverHandler := prepared.Handler
	serving := config.ExtraConfig.DeprecatedInsecureServingInfo

	r := mux.NewRouter()
	r.Path("/api").Handler(http.NotFoundHandler())
	r.PathPrefix("/apis").Handler(apiserverHandler)
	r.PathPrefix("/healthz").Handler(apiserverHandler)
	r.PathPrefix("/livez").Handler(apiserverHandler)
	r.PathPrefix("/metrics").Handler(apiserverHandler)
	r.PathPrefix("/openapi").Handler(apiserverHandler)
	r.PathPrefix("/readyz").Handler(apiserverHandler)
	r.PathPrefix("/swagger").Handler(apiserverHandler)
	r.PathPrefix("/version").Handler(apiserverHandler)
	r.PathPrefix("/").Handler(s.hudServer.Router())

	stoppedCh, err := genericapiserver.RunServer(&http.Server{
		Addr:           serving.Listener.Addr().String(),
		Handler:        r,
		MaxHeaderBytes: 1 << 20,
	}, serving.Listener, prepared.ShutdownTimeout, stopCh)
	if err != nil {
		return err
	}
	server.GenericAPIServer.RunPostStartHooks(stopCh)

	go func() {
		err := s.assetServer.Serve(ctx)
		if err != nil && ctx.Err() == nil {
			s.hudServer.store.Dispatch(store.NewErrorAction(err))
		}
	}()

	s.shutdownCh = stoppedCh

	return nil
}

var _ store.TearDowner = &HeadsUpServerController{}

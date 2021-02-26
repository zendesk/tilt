package controllers

import (
	"github.com/google/wire"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/core"
)

var controllerSet = wire.NewSet(
	core.NewFileWatchController,

	ProvideControllers,
)

func ProvideControllers(fileWatch *core.FileWatchController) []Controller {
	return []Controller{
		fileWatch,
	}
}

var WireSet = wire.NewSet(
	NewTiltServerControllerManager,

	NewScheme,
	NewControllerBuilder,

	ProvideDeferredClient,
	wire.Bind(new(ctrlclient.Client), new(*DeferredClient)),

	controllerSet,
)

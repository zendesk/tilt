package engine

import (
	"context"
	"sync"

	"github.com/windmilleng/wmclient/pkg/dirs"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/store"
)

type TelemetryController struct {
	clock build.Clock
	// lock is a lock that is used to control access from the span exporter and the telemetry controller to the file
	lock sync.Locker
	dir  *dirs.WindmillDir
}

func NewTelemetryController(lock sync.Locker, clock build.Clock, dir *dirs.WindmillDir) *TelemetryController {
	return &TelemetryController{
		lock:  lock,
		clock: clock,
		dir:   dir,
	}
}

func (t *TelemetryController) OnChange(ctx context.Context, st store.RStore) {
	// store in the engine state the last time we sent data. If it's been more than N minutes since then:
	//   if there's a script, exec it
	// if it completed, dispatch action to set time
	// (error handling)
	//   regardless: truncate the file
}

// TODO(dbentley) file based span exporter

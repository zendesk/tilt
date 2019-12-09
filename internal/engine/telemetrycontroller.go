package engine

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/windmilleng/wmclient/pkg/dirs"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/store"
)

const AnalyticsFilename = "analytics"

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
	state := st.RLockState()
	lastTelemetryRun := state.LastTelemetryScriptRun
	script := state.TelemetryScriptPath
	st.RUnlockState()

	// store in the engine state the last time we sent data. If it's been more than N minutes since then:
	if lastTelemetryRun.Add(1 * time.Hour).Before(t.clock.Now()) {
		if script != "" {
			// TODO
		}
		// if there's a script, exec it, passing in the contents of the file
		// if it completed, dispatch action to set time
		// TODO(dmiller): error handling

		// regardless: truncate the file
		err := t.truncateAnalyticsFile()
		if err != nil {
			// TODO(dmiller): error handling
		}
	}
}

func (t *TelemetryController) truncateAnalyticsFile() error {
	t.lock.Lock()
	defer t.lock.Unlock()

	return os.Truncate(filepath.Join(t.dir.Root(), AnalyticsFilename), 0)
}

// TODO(dbentley) file based span exporter

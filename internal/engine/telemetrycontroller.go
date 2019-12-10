package engine

import (
	"context"
	"os"
	"path/filepath"
	// "time"

	"github.com/windmilleng/wmclient/pkg/dirs"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tracer"
)

const AnalyticsFilename = "analytics"

type TelemetryController struct {
	clock build.Clock
	// lock is a lock that is used to control access from the span exporter and the telemetry controller to the file
	lock tracer.Locker
	dir  *dirs.WindmillDir
}

func NewTelemetryController(lock tracer.Locker, clock build.Clock, dir *dirs.WindmillDir) *TelemetryController {
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
		if script == "" {
			return
		}
		t.lock.Lock()
		defer t.lock.Unlock()
		// exec the script, passing in the contents of the file

		// dispatch action to set time
		// TODO(dmiller): error handling

		// truncate the file if the script succeeded
		err := t.truncateAnalyticsFile()
		if err != nil {
			// TODO(dmiller): error handling
		}
	}
}

func (t *TelemetryController) truncateAnalyticsFile() error {
	return dir.WriteFile(tracer.OutgoingFilename, "")
}

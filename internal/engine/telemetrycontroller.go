package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/store"

	export "go.opentelemetry.io/otel/sdk/export/trace"
)

/* TODO
1. Move init in to here
2. Make it be on by default, with a script that just writes to a file
3. Check that it gets written corrrectly
4. Disable it (now no way to enable it)
5 PR that

Then do tiltfile changes to specify script
*/

type TelemetryController struct{}

func NewTelemetryController() *TelemetryController {
	return &TelemetryController{}
}

func (t *TelemetryController) OnChange(ctx context.Context, st store.RStore) {
	// If there's a script set: initialize opentel (tracer.Init())

	// If there's a script set start a go routine that will produce InvokeTelemetryScriptActions every minute

	// If there isn't a script set uninitialize opentel? TODO(dmiller): think about this more

	// If there isn't a secript set cancel go routine that produces InvokeTelemetryScriptActions
}

// SpanExporter is an implementation of trace.SpanExporter that writes spans to the engine state via an action dispatch.
type SpanExporter struct {
	store store.RStore
}

func NewSpanExporter(s store.RStore) (*SpanExporter, error) {
	return &SpanExporter{
		store: s,
	}, nil
}

// ExportSpan writes a SpanData in to the engine state
func (e *SpanExporter) ExportSpan(ctx context.Context, data *export.SpanData) {
	e.store.Dispatch(SpanAction{data: data})
}

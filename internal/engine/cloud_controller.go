package engine

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/exporter/trace/jaeger"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tracer"

	"github.com/windmilleng/tilt/tools/devlog"
)

type CloudController struct {
	active map[int]sdktrace.SpanProcessor
}

func NewCloudController() *CloudController {
	return &CloudController{active: make(map[int]sdktrace.SpanProcessor)}
}

func (c *CloudController) OnChange(ctx context.Context, st store.RStore) {
	state := st.RLockState()
	defer st.RUnlockState()

	var toAdd []int

	seen := make(map[int]bool)

	for _, port := range state.CloudPorts {
		if _, ok := c.active[port]; ok {
			seen[port] = true
		} else {
			toAdd = append(toAdd, port)
		}
	}

	for port, sp := range c.active {
		if !seen[port] {
			delete(c.active, port)
			sdktrace.UnregisterSpanProcessor(sp)
			tracer.TP.UnregisterSpanProcessor(sp)
		}
	}

	for _, port := range toAdd {
		devlog.Logf("adding?! %v", port)
		exp, _ := jaeger.NewExporter(
			jaeger.WithAgentEndpoint(fmt.Sprintf("localhost:%d", port)),
		)
		sp := sdktrace.NewSimpleSpanProcessor(exp)
		c.active[port] = sp
		tracer.TP.RegisterSpanProcessor(sp)
		sdktrace.RegisterSpanProcessor(sp)
	}
}

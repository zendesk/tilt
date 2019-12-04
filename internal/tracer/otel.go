package tracer

import (
	"go.opentelemetry.io/otel/api/global"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var TP *sdktrace.Provider

func InitOpenTelemetry() error {
	tp, err := sdktrace.NewProvider(
		sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}))
	if err != nil {
		return err
	}
	global.SetTraceProvider(tp)
	TP = tp
	return nil
}

package otelfeature

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/nussjustin/feature"
)

const (
	namespace = "github.com/nussjustin/feature/otelfeature"
	version   = "0.5.1"
)

const (
	// AttributeFeatureEnabled is true if a flag was enabled or if running the experimental case in an Experiment.
	AttributeFeatureEnabled = attribute.Key("true")

	// AttributeFeatureName contains the name of the used feature flag.
	AttributeFeatureName = attribute.Key("feature.name")

	// AttributeExperimentSuccess is true if an experiment ran with not errors and the results are considered equal.
	AttributeExperimentSuccess = attribute.Key("feature.experiment.success")
)

// Opts can be used to customize the [feature.Tracer] returned by [Tracer].
//
// All fields are optional.
type Opts struct {
	// MeterProvider is used for creating meters for all tracked metrics.
	//
	// If nil the global provider is used.
	MeterProvider metric.MeterProvider

	// TracerProvider is used for creating a [trace.Tracer].
	//
	// If nil the global provider is used.
	TracerProvider trace.TracerProvider
}

// Tracer returns a pre-configured [feature.Tracer] that automatically traces all [feature.Flag] and [feature.Case]
// usage while also gathering metrics.
//
// An optional [Opts] struct can be given to customize the providers for the created [metric.Meter] and [trace.Tracer].
func Tracer(opts *Opts) (feature.Tracer, error) {
	metric, err := newMetricTracer(opts)
	if err != nil {
		return feature.Tracer{}, err
	}

	trace := newTraceTracer(opts)

	return combineTracers(metric, trace), nil
}

func combineTracers(metric, trace feature.Tracer) feature.Tracer {
	return feature.Tracer{
		Decision: func(ctx context.Context, flag *feature.Flag, enabled bool) {
			trace.Decision(ctx, flag, enabled)
			metric.Decision(ctx, flag, enabled)
		},
		Experiment: func(ctx context.Context, flag *feature.Flag, enabled bool) (context.Context, func(result any, err error, success bool)) {
			ctx, traceDone := trace.Experiment(ctx, flag, enabled)
			ctx, metricDone := metric.Experiment(ctx, flag, enabled)
			return ctx, func(result any, err error, success bool) {
				metricDone(result, err, success)
				traceDone(result, err, success)
			}
		},
		ExperimentBranch: func(ctx context.Context, flag *feature.Flag, enabled bool) (context.Context, func(result any, err error)) {
			ctx, traceDone := trace.ExperimentBranch(ctx, flag, enabled)
			ctx, metricDone := metric.ExperimentBranch(ctx, flag, enabled)
			return ctx, func(result any, err error) {
				metricDone(result, err)
				traceDone(result, err)
			}
		},
		Switch: func(ctx context.Context, flag *feature.Flag, enabled bool) (context.Context, func(result any, err error)) {
			ctx, traceDone := trace.Switch(ctx, flag, enabled)
			ctx, metricDone := metric.Switch(ctx, flag, enabled)
			return ctx, func(result any, err error) {
				metricDone(result, err)
				traceDone(result, err)
			}
		},
	}
}

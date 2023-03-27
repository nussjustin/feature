package otelfeature

import (
	"context"

	"github.com/nussjustin/feature"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	namespace = "github.com/nussjustin/feature/otelfeature"
	version   = "0.1.0"
)

const (
	// AttributeFeatureEnabled is true if a flag was enabled or if running the experimental case in an Experiment.
	AttributeFeatureEnabled = attribute.Key("feature.enabled")

	// AttributeFeatureName contains the name of the used feature flag.
	AttributeFeatureName = attribute.Key("feature.name")

	// AttributeExperimentSuccess is true if an experiment ran with not errors and the results are considered equal.
	AttributeExperimentSuccess = attribute.Key("feature.experiment.success")

	// AttributeRecoveredValue contains the recovered value from a panic converted into a string using fmt.Sprint.
	AttributeRecoveredValue = attribute.Key("feature.case.recovered")
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
		Decision: func(ctx context.Context, flag *feature.Flag, decision feature.Decision) {
			trace.Decision(ctx, flag, decision)
			metric.Decision(ctx, flag, decision)
		},
		Branch: func(ctx context.Context, flag *feature.Flag, decision feature.Decision) (context.Context, func(result any, err error)) {
			ctx, traceDone := trace.Branch(ctx, flag, decision)
			ctx, metricDone := metric.Branch(ctx, flag, decision)
			return ctx, func(result any, err error) {
				metricDone(result, err)
				traceDone(result, err)
			}
		},
		BranchPanicked: func(ctx context.Context, flag *feature.Flag, decision feature.Decision, panicError *feature.PanicError) {
			trace.BranchPanicked(ctx, flag, decision, panicError)
			metric.BranchPanicked(ctx, flag, decision, panicError)
		},
		Experiment: func(ctx context.Context, flag *feature.Flag) (context.Context, func(d feature.Decision, result any, err error, success bool)) {
			ctx, traceDone := trace.Experiment(ctx, flag)
			ctx, metricDone := metric.Experiment(ctx, flag)
			return ctx, func(d feature.Decision, result any, err error, success bool) {
				metricDone(d, result, err, success)
				traceDone(d, result, err, success)
			}
		},
		Run: func(ctx context.Context, flag *feature.Flag) (context.Context, func(d feature.Decision, result any, err error)) {
			ctx, traceDone := trace.Run(ctx, flag)
			ctx, metricDone := metric.Run(ctx, flag)
			return ctx, func(d feature.Decision, result any, err error) {
				metricDone(d, result, err)
				traceDone(d, result, err)
			}
		},
	}
}

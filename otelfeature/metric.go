package otelfeature

import (
	"context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	"github.com/nussjustin/feature"
)

func newMetricTracer(opts *Opts) (feature.Tracer, error) {
	provider := otel.GetMeterProvider()
	if opts != nil && opts.MeterProvider != nil {
		provider = opts.MeterProvider
	}

	meter := provider.Meter(namespace, metric.WithInstrumentationVersion(version))

	decisionCounter, err := meter.Int64Counter("feature.decisions",
		metric.WithDescription("Number of decisions by flag name and decision"))

	if err != nil {
		return feature.Tracer{}, err
	}

	return feature.Tracer{
		Decision: createMetricDecisionCallback(decisionCounter),
		Switch:   createMetricSwitchCallback(),
	}, nil
}

func createMetricDecisionCallback(
	decisionCounter metric.Int64Counter,
) func(context.Context, *feature.Flag, bool) {
	return func(ctx context.Context, flag *feature.Flag, enabled bool) {
		decisionCounter.Add(ctx, 1, metric.WithAttributes(
			AttributeFeatureEnabled.Bool(enabled),
			AttributeFeatureName.String(flag.Name())))
	}
}

func createMetricSwitchCallback() func(context.Context, *feature.Flag, bool) (context.Context, func(result any, err error)) {
	return func(ctx context.Context, _ *feature.Flag, _ bool) (context.Context, func(result any, err error)) {
		return ctx, func(any, error) {}
	}
}

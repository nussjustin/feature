package otelfeature

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/nussjustin/feature"
)

func newTraceTracer(opts *Opts) feature.Tracer {
	provider := otel.GetTracerProvider()
	if opts != nil && opts.TracerProvider != nil {
		provider = opts.TracerProvider
	}

	tracer := provider.Tracer(namespace, trace.WithInstrumentationVersion(version))

	return feature.Tracer{
		Decision: createTraceDecisionCallback(),
		Switch:   createTraceSwitchCallback(tracer),
	}
}

func createTraceDecisionCallback() func(context.Context, *feature.Flag, bool) {
	return func(ctx context.Context, flag *feature.Flag, enabled bool) {
		span := trace.SpanFromContext(ctx)
		span.AddEvent("decision", trace.WithAttributes(
			AttributeFeatureEnabled.Bool(enabled),
			AttributeFeatureName.String(flag.Name())))
	}
}

func createTraceSwitchCallback(t trace.Tracer) func(context.Context, *feature.Flag, bool) (context.Context, func(result any, err error)) {
	return func(ctx context.Context, flag *feature.Flag, enabled bool) (context.Context, func(result any, err error)) {
		ctx, span := t.Start(ctx, flag.Name(),
			trace.WithAttributes(
				AttributeFeatureEnabled.Bool(enabled),
				AttributeFeatureName.String(flag.Name()),
			),
		)

		return ctx, func(_ any, err error) {
			if err != nil {
				span.SetStatus(codes.Error, err.Error())
			} else {
				span.SetStatus(codes.Ok, "")
			}

			span.End()
		}
	}
}

func nameFromDecision(enabled bool) string {
	if enabled {
		return "Enabled"
	}
	return "Disabled"
}

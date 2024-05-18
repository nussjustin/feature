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
		Decision:         createTraceDecisionCallback(),
		Experiment:       createTraceExperimentCallback(tracer),
		ExperimentBranch: createTraceExperimentBranchCallback(tracer),
		Switch:           createTraceSwitchCallback(tracer),
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

func createTraceExperimentCallback(t trace.Tracer) func(context.Context, *feature.Flag, bool) (context.Context, func(result any, err error, success bool)) {
	return func(ctx context.Context, flag *feature.Flag, enabled bool) (context.Context, func(result any, err error, success bool)) {
		ctx, span := t.Start(ctx, flag.Name(),
			trace.WithAttributes(
				AttributeFeatureEnabled.Bool(enabled),
				AttributeFeatureName.String(flag.Name()),
			),
		)

		return ctx, func(_ any, err error, success bool) {
			span.SetAttributes(AttributeExperimentSuccess.Bool(success))

			if err != nil {
				span.SetStatus(codes.Error, err.Error())
			} else {
				span.SetStatus(codes.Ok, "")
			}

			span.End()
		}
	}
}

func createTraceExperimentBranchCallback(t trace.Tracer) func(context.Context, *feature.Flag, bool) (context.Context, func(result any, err error)) {
	return func(ctx context.Context, flag *feature.Flag, enabled bool) (context.Context, func(result any, err error)) {
		ctx, span := t.Start(ctx, nameFromDecision(enabled),
			trace.WithAttributes(
				AttributeFeatureEnabled.Bool(enabled),
				AttributeFeatureName.String(flag.Name())))

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

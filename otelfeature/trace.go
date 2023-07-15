package otelfeature

import (
	"context"
	"fmt"

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
		Decision:       createTraceDecisionCallback(),
		Branch:         createTraceCaseCallback(tracer),
		BranchPanicked: createTraceCasePanickedCallback(),
		Experiment:     createTraceExperimentCallback(tracer),
		Run:            createTraceRunCallback(tracer),
	}
}

func createTraceDecisionCallback() func(context.Context, *feature.Flag, feature.Decision) {
	return func(ctx context.Context, flag *feature.Flag, decision feature.Decision) {
		span := trace.SpanFromContext(ctx)
		span.AddEvent("decision", trace.WithAttributes(
			AttributeFeatureEnabled.Bool(decision == feature.Enabled),
			AttributeFeatureName.String(flag.Name())))
	}
}

func createTraceCaseCallback(t trace.Tracer) func(context.Context, *feature.Flag, feature.Decision) (context.Context, func(result any, err error)) {
	return func(ctx context.Context, flag *feature.Flag, decision feature.Decision) (context.Context, func(result any, err error)) {
		ctx, span := t.Start(ctx, nameFromDecision(decision),
			trace.WithAttributes(
				AttributeFeatureEnabled.Bool(decision == feature.Enabled),
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

func createTraceCasePanickedCallback() func(context.Context, *feature.Flag, feature.Decision, *feature.PanicError) {
	return func(ctx context.Context, flag *feature.Flag, decision feature.Decision, err *feature.PanicError) {
		span := trace.SpanFromContext(ctx)

		if span.IsRecording() {
			formatted := fmt.Sprint(err.Recovered)

			span.AddEvent("panic", trace.WithAttributes(
				AttributeRecoveredValue.String(formatted)))
		}
	}
}

func createTraceExperimentCallback(t trace.Tracer) func(context.Context, *feature.Flag) (context.Context, func(d feature.Decision, result any, err error, success bool)) {
	return func(ctx context.Context, flag *feature.Flag) (context.Context, func(d feature.Decision, result any, err error, success bool)) {
		ctx, span := t.Start(ctx, flag.Name(),
			trace.WithAttributes(AttributeFeatureName.String(flag.Name())))

		return ctx, func(decision feature.Decision, _ any, err error, success bool) {
			span.SetAttributes(
				AttributeFeatureEnabled.Bool(decision == feature.Enabled),
				AttributeExperimentSuccess.Bool(success))

			if err != nil {
				span.SetStatus(codes.Error, err.Error())
			} else {
				span.SetStatus(codes.Ok, "")
			}

			span.End()
		}
	}
}

func createTraceRunCallback(t trace.Tracer) func(context.Context, *feature.Flag) (context.Context, func(d feature.Decision, result any, err error)) {
	return func(ctx context.Context, flag *feature.Flag) (context.Context, func(d feature.Decision, result any, err error)) {
		ctx, span := t.Start(ctx, flag.Name(),
			trace.WithAttributes(AttributeFeatureName.String(flag.Name())))

		return ctx, func(decision feature.Decision, result any, err error) {
			span.SetAttributes(AttributeFeatureEnabled.Bool(decision == feature.Enabled))

			if err != nil {
				span.SetStatus(codes.Error, err.Error())
			} else {
				span.SetStatus(codes.Ok, "")
			}

			span.End()
		}
	}
}

func nameFromDecision(d feature.Decision) string {
	if d == feature.Enabled {
		return "Enabled"
	}
	return "Disabled"
}

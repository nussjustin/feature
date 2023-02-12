package otelfeature

import (
	"context"

	"github.com/nussjustin/feature"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	tracerName = "github.com/nussjustin/feature/otelfeature"
)

var (
	// AttributeFeatureEnabled is true if a flag was enabled or if running the experimental case in an Experiment.
	AttributeFeatureEnabled = attribute.Key("feature.enabled")

	// AttributeFeatureName contains the name of the used feature flag.
	AttributeFeatureName = attribute.Key("feature.name")

	// AttributeExperimentSuccess is true if an experiment ran with not errors and the results are considered equal.
	AttributeExperimentSuccess = attribute.Key("feature.experiment.success")
)

func Tracer(tp trace.TracerProvider) feature.Tracer {
	if tp == nil {
		tp = otel.GetTracerProvider()
	}

	tracer := tp.Tracer(tracerName)

	return feature.Tracer{
		Case:       createCaseCallback(tracer),
		Experiment: createExperimentCallback(tracer),
		Run:        createRunCallback(tracer),
	}
}

func createCaseCallback(t trace.Tracer) func(context.Context, *feature.Flag, feature.Decision) (context.Context, func(result any, err error)) {
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

func createExperimentCallback(t trace.Tracer) func(context.Context, *feature.Flag) (context.Context, func(d feature.Decision, result any, err error, success bool)) {
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

func createRunCallback(t trace.Tracer) func(context.Context, *feature.Flag) (context.Context, func(d feature.Decision, result any, err error)) {
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

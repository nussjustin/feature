package otelfeature

import (
	"context"
	"errors"

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

	caseCounter, err1 := meter.Int64Counter("feature.case",
		metric.WithDescription("Number of case executions by flag name and decision"))

	caseFailedCounter, err2 := meter.Int64Counter("feature.case.failed",
		metric.WithDescription("Number of failed case executions by flag name and decision"))

	caseRecoveredCounter, err3 := meter.Int64Counter("feature.case.recovered",
		metric.WithDescription("Number of panics recovered in cases by flag name and decision"))

	decisionCounter, err4 := meter.Int64Counter("feature.decisions",
		metric.WithDescription("Number of decisions by flag name and decision"))

	experimentCounter, err5 := meter.Int64Counter("feature.experiments",
		metric.WithDescription("Number of experiment executions by flag name, decision and success"))

	experimentErrorsCounter, err6 := meter.Int64Counter("feature.experiments.errors",
		metric.WithDescription("Number of experiment that returned errors by flag name and decision"))

	if err := errors.Join(err1, err2, err3, err4, err5, err6); err != nil {
		return feature.Tracer{}, err
	}

	return feature.Tracer{
		Decision:       createMetricDecisionCallback(decisionCounter),
		Branch:         createMetricCaseCallback(caseCounter, caseFailedCounter),
		BranchPanicked: createMetricCasePanickedCallback(caseRecoveredCounter),
		Experiment:     createMetricExperimentCallback(experimentCounter, experimentErrorsCounter),
		Run:            createMetricRunCallback(),
	}, nil
}

func createMetricDecisionCallback(
	decisionCounter metric.Int64Counter,
) func(context.Context, *feature.Flag, feature.Decision) {
	return func(ctx context.Context, flag *feature.Flag, decision feature.Decision) {
		decisionCounter.Add(ctx, 1, metric.WithAttributes(
			AttributeFeatureEnabled.Bool(decision == feature.Enabled),
			AttributeFeatureName.String(flag.Name())))
	}
}

func createMetricCaseCallback(
	caseCounter metric.Int64Counter,
	caseFailedCounter metric.Int64Counter,
) func(context.Context, *feature.Flag, feature.Decision) (context.Context, func(any, error)) {
	return func(ctx context.Context, flag *feature.Flag, decision feature.Decision) (context.Context, func(any, error)) {
		return ctx, func(_ any, err error) {
			attributes := metric.WithAttributes(
				AttributeFeatureEnabled.Bool(decision == feature.Enabled),
				AttributeFeatureName.String(flag.Name()))

			caseCounter.Add(ctx, 1, attributes)

			if err != nil {
				caseFailedCounter.Add(ctx, 1, attributes)
			}
		}
	}
}

func createMetricCasePanickedCallback(
	caseRecoveredCounter metric.Int64Counter,
) func(context.Context, *feature.Flag, feature.Decision, *feature.PanicError) {
	return func(ctx context.Context, flag *feature.Flag, decision feature.Decision, _ *feature.PanicError) {
		caseRecoveredCounter.Add(ctx, 1, metric.WithAttributes(
			AttributeFeatureEnabled.Bool(decision == feature.Enabled),
			AttributeFeatureName.String(flag.Name())))
	}
}

func createMetricExperimentCallback(
	experimentCounter metric.Int64Counter,
	experimentErrorsCounter metric.Int64Counter,
) func(context.Context, *feature.Flag) (context.Context, func(feature.Decision, any, error, bool)) {
	return func(ctx context.Context, flag *feature.Flag) (context.Context, func(feature.Decision, any, error, bool)) {
		return ctx, func(d feature.Decision, _ any, err error, success bool) {
			experimentCounter.Add(ctx, 1, metric.WithAttributes(
				AttributeExperimentSuccess.Bool(success),
				AttributeFeatureEnabled.Bool(d == feature.Enabled),
				AttributeFeatureName.String(flag.Name())))

			if err != nil {
				experimentErrorsCounter.Add(ctx, 1, metric.WithAttributes(
					AttributeFeatureEnabled.Bool(d == feature.Enabled),
					AttributeFeatureName.String(flag.Name())))
			}
		}
	}
}

func createMetricRunCallback() func(context.Context, *feature.Flag) (context.Context, func(feature.Decision, any, error)) {
	return func(ctx context.Context, flag *feature.Flag) (context.Context, func(feature.Decision, any, error)) {
		// We count cases so no need to check the run itself
		return ctx, func(d feature.Decision, _ any, err error) {}
	}
}

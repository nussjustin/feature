package otelfeature

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"

	"github.com/nussjustin/feature"
)

func newMetricTracer(opts *Opts) (feature.Tracer, error) {
	provider := global.MeterProvider()
	if opts != nil && opts.MeterProvider != nil {
		provider = opts.MeterProvider
	}

	meter := provider.Meter(namespace, metric.WithInstrumentationVersion(version))

	caseCounter, err1 := meter.Int64Counter("feature.case",
		instrument.WithDescription("Number of case executions by flag name and decision"))

	caseFailedCounter, err2 := meter.Int64Counter("feature.case.failed",
		instrument.WithDescription("Number of failed case executions by flag name and decision"))

	caseRecoveredCounter, err3 := meter.Int64Counter("feature.case.recovered",
		instrument.WithDescription("Number of panics recovered in cases by flag name and decision"))

	decisionCounter, err4 := meter.Int64Counter("feature.decisions",
		instrument.WithDescription("Number of decisions by flag name and decision"))

	experimentCounter, err5 := meter.Int64Counter("feature.experiments",
		instrument.WithDescription("Number of experiment executions by flag name, decision and success"))

	experimentErrorsCounter, err6 := meter.Int64Counter("feature.experiments.errors",
		instrument.WithDescription("Number of experiment that returned errors by flag name and decision"))

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
	decisionCounter instrument.Int64Counter,
) func(context.Context, *feature.Flag, feature.Decision) {
	return func(ctx context.Context, flag *feature.Flag, decision feature.Decision) {
		decisionCounter.Add(ctx, 1,
			AttributeFeatureEnabled.Bool(decision == feature.Enabled),
			AttributeFeatureName.String(flag.Name()))
	}
}

func createMetricCaseCallback(
	caseCounter instrument.Int64Counter,
	caseFailedCounter instrument.Int64Counter,
) func(context.Context, *feature.Flag, feature.Decision) (context.Context, func(any, error)) {
	return func(ctx context.Context, flag *feature.Flag, decision feature.Decision) (context.Context, func(any, error)) {
		return ctx, func(_ any, err error) {
			caseCounter.Add(ctx, 1,
				AttributeFeatureEnabled.Bool(decision == feature.Enabled),
				AttributeFeatureName.String(flag.Name()))

			if err != nil {
				caseFailedCounter.Add(ctx, 1,
					AttributeFeatureEnabled.Bool(decision == feature.Enabled),
					AttributeFeatureName.String(flag.Name()))
			}
		}
	}
}

func createMetricCasePanickedCallback(
	caseRecoveredCounter instrument.Int64Counter,
) func(context.Context, *feature.Flag, feature.Decision, *feature.PanicError) {
	return func(ctx context.Context, flag *feature.Flag, decision feature.Decision, _ *feature.PanicError) {
		caseRecoveredCounter.Add(ctx, 1,
			AttributeFeatureEnabled.Bool(decision == feature.Enabled),
			AttributeFeatureName.String(flag.Name()))
	}
}

func createMetricExperimentCallback(
	experimentCounter instrument.Int64Counter,
	experimentErrorsCounter instrument.Int64Counter,
) func(context.Context, *feature.Flag) (context.Context, func(feature.Decision, any, error, bool)) {
	return func(ctx context.Context, flag *feature.Flag) (context.Context, func(feature.Decision, any, error, bool)) {
		return ctx, func(d feature.Decision, _ any, err error, success bool) {
			experimentCounter.Add(ctx, 1,
				AttributeExperimentSuccess.Bool(success),
				AttributeFeatureEnabled.Bool(d == feature.Enabled),
				AttributeFeatureName.String(flag.Name()))

			if err != nil {
				experimentErrorsCounter.Add(ctx, 1,
					AttributeFeatureEnabled.Bool(d == feature.Enabled),
					AttributeFeatureName.String(flag.Name()))
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

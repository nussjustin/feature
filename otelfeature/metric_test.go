package otelfeature_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/embedded"

	"github.com/nussjustin/feature"
	"github.com/nussjustin/feature/otelfeature"
)

func TestTracer_Metrics(t *testing.T) {
	createTracer := func(tb testing.TB) (feature.Tracer, *testMeter) {
		tb.Helper()

		provider := &testMeterProvider{tb: tb}

		tracer, err := otelfeature.Tracer(&otelfeature.Opts{MeterProvider: provider})
		if err != nil {
			tb.Fatalf("got error creating tracer: %s", err)
		}

		return tracer, provider.Meter("").(*testMeter)
	}

	ctx := context.Background()

	t.Run("Error", func(t *testing.T) {
		provider := &testMeterProvider{tb: t}

		want := errors.New("")

		meter := provider.Meter("").(*testMeter)
		meter.err = want

		_, got := otelfeature.Tracer(&otelfeature.Opts{MeterProvider: provider})
		if !errors.Is(got, want) {
			t.Fatalf("got error %v, want %v", got, want)
		}
	})

	t.Run("Decision", func(t *testing.T) {
		flag := newFlag(t)

		tracer, meter := createTracer(t)
		tracer.Decision(ctx, flag, feature.Disabled)
		tracer.Decision(ctx, flag, feature.Enabled)
		tracer.Decision(ctx, flag, feature.Enabled)

		meter.assertOnly("feature.decisions")

		meter.assertInt64("feature.decisions", 3)

		meter.assertInt64("feature.decisions", 2,
			otelfeature.AttributeFeatureEnabled.Bool(true),
			otelfeature.AttributeFeatureName.String(flag.Name()))

		meter.assertInt64("feature.decisions", 1,
			otelfeature.AttributeFeatureEnabled.Bool(false),
			otelfeature.AttributeFeatureName.String(flag.Name()))
	})

	t.Run("Case", func(t *testing.T) {
		flag := newFlag(t)

		tracer, meter := createTracer(t)
		_, f1 := tracer.Branch(ctx, flag, feature.Disabled)
		f1(nil, nil)
		_, f2 := tracer.Branch(ctx, flag, feature.Enabled)
		f2(nil, errors.New("err2"))
		_, f3 := tracer.Branch(ctx, flag, feature.Enabled)
		f3(nil, nil)
		_, f4 := tracer.Branch(ctx, flag, feature.Disabled)
		f4(nil, nil)

		meter.assertOnly("feature.case", "feature.case.failed")

		meter.assertInt64("feature.case", 4)
		meter.assertInt64("feature.case.failed", 1)

		meter.assertInt64("feature.case", 2,
			otelfeature.AttributeFeatureEnabled.Bool(true),
			otelfeature.AttributeFeatureName.String(flag.Name()))

		meter.assertInt64("feature.case.failed", 1,
			otelfeature.AttributeFeatureEnabled.Bool(true),
			otelfeature.AttributeFeatureName.String(flag.Name()))

		meter.assertInt64("feature.case", 2,
			otelfeature.AttributeFeatureEnabled.Bool(false),
			otelfeature.AttributeFeatureName.String(flag.Name()))

		meter.assertInt64("feature.case.failed", 0,
			otelfeature.AttributeFeatureEnabled.Bool(false),
			otelfeature.AttributeFeatureName.String(flag.Name()))
	})

	t.Run("Case Panicked", func(t *testing.T) {
		flag := newFlag(t)

		tracer, meter := createTracer(t)
		tracer.BranchPanicked(ctx, flag, feature.Enabled, &feature.PanicError{})

		meter.assertOnly("feature.case.recovered")

		meter.assertInt64("feature.case.recovered", 1)

		meter.assertInt64("feature.case.recovered", 1,
			otelfeature.AttributeFeatureEnabled.Bool(true),
			otelfeature.AttributeFeatureName.String(flag.Name()))
	})

	t.Run("Experiment", func(t *testing.T) {
		flag := newFlag(t)

		tracer, meter := createTracer(t)
		_, f1 := tracer.Experiment(ctx, flag)
		f1(feature.Enabled, nil, nil, true)
		_, f2 := tracer.Experiment(ctx, flag)
		f2(feature.Enabled, nil, nil, false)
		_, f3 := tracer.Experiment(ctx, flag)
		f3(feature.Enabled, nil, nil, false)
		_, f4 := tracer.Experiment(ctx, flag)
		f4(feature.Disabled, nil, nil, false)
		_, f5 := tracer.Experiment(ctx, flag)
		f5(feature.Disabled, nil, errors.New("err5"), false)

		meter.assertOnly("feature.experiments", "feature.experiments.errors")

		meter.assertInt64("feature.experiments", 5)
		meter.assertInt64("feature.experiments.errors", 1)

		meter.assertInt64("feature.experiments", 1,
			otelfeature.AttributeExperimentSuccess.Bool(true),
			otelfeature.AttributeFeatureEnabled.Bool(true),
			otelfeature.AttributeFeatureName.String(flag.Name()))

		meter.assertInt64("feature.experiments", 2,
			otelfeature.AttributeExperimentSuccess.Bool(false),
			otelfeature.AttributeFeatureEnabled.Bool(true),
			otelfeature.AttributeFeatureName.String(flag.Name()))

		meter.assertInt64("feature.experiments.errors", 0,
			otelfeature.AttributeFeatureEnabled.Bool(true),
			otelfeature.AttributeFeatureName.String(flag.Name()))

		meter.assertInt64("feature.experiments", 2,
			otelfeature.AttributeFeatureEnabled.Bool(false),
			otelfeature.AttributeFeatureName.String(flag.Name()))

		meter.assertInt64("feature.experiments.errors", 1,
			otelfeature.AttributeFeatureEnabled.Bool(false),
			otelfeature.AttributeFeatureName.String(flag.Name()))
	})

	t.Run("Switch", func(t *testing.T) {
		flag := newFlag(t)

		tracer, meter := createTracer(t)
		tracer.Run(ctx, flag)

		meter.assertOnly()
	})
}

type testInt64CounterAdded struct {
	incr  int64
	attrs attribute.Set
}

type testInt64Counter struct {
	metric.Int64Counter
	tb   testing.TB
	name string

	added []*testInt64CounterAdded
}

func (t *testInt64Counter) Add(_ context.Context, incr int64, options ...metric.AddOption) {
	attrs := metric.NewAddConfig(options).Attributes()
	t.added = append(t.added, &testInt64CounterAdded{incr, attrs})
}

func (t *testInt64Counter) forEach(attrs []attribute.KeyValue, f func(added *testInt64CounterAdded)) {
	for _, added := range t.added {
		if containsAll(added.attrs, attrs) {
			f(added)
		}
	}
}

func (t *testInt64Counter) assert(want int64, attrs ...attribute.KeyValue) {
	t.tb.Helper()

	var got int64

	t.forEach(attrs, func(added *testInt64CounterAdded) {
		got += added.incr
	})

	if got != want {
		var formattedAttrs []string
		for _, attr := range attrs {
			formattedAttrs = append(formattedAttrs, fmt.Sprintf("%s=%v", attr.Key, attr.Value.AsInterface()))
		}
		t.tb.Errorf("got total %d of counter %q, want %d %s", got, t.name, want, formattedAttrs)
	}
}

type testMeter struct {
	metric.Meter
	tb testing.TB

	err error

	int64Counters map[string]*testInt64Counter
}

func (t *testMeter) Int64Counter(name string, _ ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	if t.int64Counters == nil {
		t.int64Counters = make(map[string]*testInt64Counter)
	}
	if t.int64Counters[name] == nil {
		t.int64Counters[name] = &testInt64Counter{tb: t.tb, name: name}
	}
	return t.int64Counters[name], t.err
}

func (t *testMeter) assertInt64(name string, want int64, attrs ...attribute.KeyValue) {
	t.tb.Helper()
	t.int64Counters[name].assert(want, attrs...)
}

func (t *testMeter) assertOnly(expected ...string) {
	for name, c := range t.int64Counters {
		if contains(expected, name) {
			continue
		}
		c.assert(0)
	}
}

type testMeterProvider struct {
	embedded.MeterProvider
	meter *testMeter
	tb    testing.TB
}

func (t *testMeterProvider) Meter(name string, opts ...metric.MeterOption) metric.Meter {
	if t.meter == nil {
		t.meter = &testMeter{tb: t.tb}
	}
	return t.meter
}

func contains[T comparable](set []T, value T) bool {
	for i := range set {
		if set[i] == value {
			return true
		}
	}
	return false
}

func containsAll(set attribute.Set, attrs []attribute.KeyValue) bool {
	if len(attrs) == 0 {
		return true
	}

	if len(attrs) > set.Len() {
		return false
	}

	for _, attr := range attrs {
		got, ok := set.Value(attr.Key)
		if !ok || got.Emit() != attr.Value.Emit() {
			return false
		}

	}

	return true
}

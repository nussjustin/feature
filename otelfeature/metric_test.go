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
		tracer.Decision(ctx, flag, false)
		tracer.Decision(ctx, flag, true)
		tracer.Decision(ctx, flag, true)

		meter.assertOnly("feature.decisions")

		meter.assertInt64("feature.decisions", 3)

		meter.assertInt64("feature.decisions", 2,
			otelfeature.AttributeFeatureEnabled.Bool(true),
			otelfeature.AttributeFeatureName.String(flag.Name()))

		meter.assertInt64("feature.decisions", 1,
			otelfeature.AttributeFeatureEnabled.Bool(false),
			otelfeature.AttributeFeatureName.String(flag.Name()))
	})

	t.Run("Switch", func(t *testing.T) {
		flag := newFlag(t)

		tracer, meter := createTracer(t)
		tracer.Switch(ctx, flag, true)

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
	t.tb.Helper()
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

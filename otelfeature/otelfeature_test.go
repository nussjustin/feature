package otelfeature_test

import (
	"context"
	"errors"
	"testing"

	"github.com/nussjustin/feature"
	"github.com/nussjustin/feature/otelfeature"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTracer(t *testing.T) {
	t.Run("Case", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			flag := feature.RegisterFlag(
				&feature.Set{},
				"Case",
				"",
				nil,
				feature.DefaultDisabled,
			)

			spanRecorder := tracetest.NewSpanRecorder()
			provider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))
			tracer := otelfeature.Tracer(provider)

			_, done := tracer.Case(context.Background(), flag, feature.Enabled)
			done(nil, nil)

			recordedSpan := getSpan(t, spanRecorder, "Enabled")
			assertAttributeBool(t, recordedSpan, otelfeature.AttributeFeatureEnabled, true)
			assertAttributeString(t, recordedSpan, otelfeature.AttributeFeatureName, flag.Name())
			assertSpanOk(t, recordedSpan)
		})

		t.Run("Error", func(t *testing.T) {
			flag := feature.RegisterFlag(
				&feature.Set{},
				"Case",
				"",
				nil,
				feature.DefaultDisabled,
			)

			spanRecorder := tracetest.NewSpanRecorder()
			provider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))
			tracer := otelfeature.Tracer(provider)

			_, done := tracer.Case(context.Background(), flag, feature.Disabled)
			done(nil, errors.New("some error"))

			recordedSpan := getSpan(t, spanRecorder, "Disabled")
			assertAttributeBool(t, recordedSpan, otelfeature.AttributeFeatureEnabled, false)
			assertAttributeString(t, recordedSpan, otelfeature.AttributeFeatureName, flag.Name())
			assertSpanError(t, recordedSpan, "some error")
		})
	})

	t.Run("Experiment", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			flag := feature.RegisterFlag(
				&feature.Set{},
				"Experiment",
				"",
				nil,
				feature.DefaultDisabled,
			)

			spanRecorder := tracetest.NewSpanRecorder()
			provider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))
			tracer := otelfeature.Tracer(provider)

			_, done := tracer.Experiment(context.Background(), flag)
			done(feature.Enabled, nil, nil, true)

			recordedSpan := getSpan(t, spanRecorder, flag.Name())
			assertAttributeBool(t, recordedSpan, otelfeature.AttributeFeatureEnabled, true)
			assertAttributeString(t, recordedSpan, otelfeature.AttributeFeatureName, flag.Name())
			assertAttributeBool(t, recordedSpan, otelfeature.AttributeExperimentSuccess, true)
			assertSpanOk(t, recordedSpan)
		})

		t.Run("Error", func(t *testing.T) {
			flag := feature.RegisterFlag(
				&feature.Set{},
				"Experiment",
				"",
				nil,
				feature.DefaultDisabled,
			)

			spanRecorder := tracetest.NewSpanRecorder()
			provider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))
			tracer := otelfeature.Tracer(provider)

			_, done := tracer.Experiment(context.Background(), flag)
			done(feature.Enabled, nil, errors.New("failed"), false)

			recordedSpan := getSpan(t, spanRecorder, flag.Name())
			assertAttributeBool(t, recordedSpan, otelfeature.AttributeFeatureEnabled, true)
			assertAttributeString(t, recordedSpan, otelfeature.AttributeFeatureName, flag.Name())
			assertAttributeBool(t, recordedSpan, otelfeature.AttributeExperimentSuccess, false)
			assertSpanError(t, recordedSpan, "failed")
		})
	})

	t.Run("Run", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			flag := feature.RegisterFlag(
				&feature.Set{},
				"Run",
				"",
				nil,
				feature.DefaultDisabled,
			)

			spanRecorder := tracetest.NewSpanRecorder()
			provider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))
			tracer := otelfeature.Tracer(provider)

			_, done := tracer.Run(context.Background(), flag)
			done(feature.Enabled, nil, nil)

			recordedSpan := getSpan(t, spanRecorder, flag.Name())
			assertAttributeBool(t, recordedSpan, otelfeature.AttributeFeatureEnabled, true)
			assertAttributeString(t, recordedSpan, otelfeature.AttributeFeatureName, flag.Name())
			assertSpanOk(t, recordedSpan)
		})

		t.Run("Error", func(t *testing.T) {
			flag := feature.RegisterFlag(
				&feature.Set{},
				"Run",
				"",
				nil,
				feature.DefaultDisabled,
			)

			spanRecorder := tracetest.NewSpanRecorder()
			provider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))
			tracer := otelfeature.Tracer(provider)

			_, done := tracer.Run(context.Background(), flag)
			done(feature.Enabled, nil, errors.New("failed"))

			recordedSpan := getSpan(t, spanRecorder, flag.Name())
			assertAttributeBool(t, recordedSpan, otelfeature.AttributeFeatureEnabled, true)
			assertAttributeString(t, recordedSpan, otelfeature.AttributeFeatureName, flag.Name())
			assertSpanError(t, recordedSpan, "failed")
		})
	})
}

func ExampleTracer() {
	feature.SetTracer(otelfeature.Tracer(nil))
}

func assertAttributeBool(tb testing.TB, span trace.ReadOnlySpan, key attribute.Key, want bool) {
	tb.Helper()

	if got := getAttributeOfType(tb, span, key, attribute.BOOL).AsBool(); got != want {
		tb.Errorf("got %s = %t, want %t", key, got, want)
	}
}

func assertAttributeString(tb testing.TB, span trace.ReadOnlySpan, key attribute.Key, want string) {
	tb.Helper()

	if got := getAttributeOfType(tb, span, key, attribute.STRING).AsString(); got != want {
		tb.Errorf("got %s = %q, want %q", key, got, want)
	}
}

func getAttribute(tb testing.TB, span trace.ReadOnlySpan, key attribute.Key) attribute.Value {
	tb.Helper()

	for _, attr := range span.Attributes() {
		if attr.Key != key {
			continue
		}

		if !attr.Valid() {
			tb.Errorf("attribute %s is not valid", key)
		}

		return attr.Value
	}

	tb.Fatalf("attribute not found: %s", key)
	return attribute.Value{}
}

func getAttributeOfType(tb testing.TB, span trace.ReadOnlySpan, key attribute.Key, type_ attribute.Type) attribute.Value {
	tb.Helper()

	value := getAttribute(tb, span, key)

	if value.Type() != type_ {
		tb.Fatalf("attribute %s has wrong type: %s", key, value.Type())

	}
	return value
}

func getSpan(tb testing.TB, sr *tracetest.SpanRecorder, name string) trace.ReadOnlySpan {
	for _, span := range sr.Ended() {
		if span.Name() == name {
			return span
		}
	}

	tb.Fatalf("span not found: %s", name)
	return nil
}

func assertSpanError(tb testing.TB, span trace.ReadOnlySpan, description string) {
	tb.Helper()

	if got, want := span.Status().Code, codes.Error; got != want {
		tb.Errorf("got status %q, want %s", got, want)
	}

	if got, want := span.Status().Description, description; got != want {
		tb.Errorf("got description %q, want %q", got, want)
	}
}

func assertSpanOk(tb testing.TB, span trace.ReadOnlySpan) {
	tb.Helper()

	if got, want := span.Status().Code, codes.Ok; got != want {
		tb.Errorf("got status %q, want %s", got, want)
	}
}

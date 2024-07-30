package otelfeature_test

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/nussjustin/feature"
	"github.com/nussjustin/feature/otelfeature"
)

func newFlag(tb testing.TB) *feature.Flag {
	return (&feature.Set{}).New(tb.Name())
}

func TestTracer_Tracing(t *testing.T) {
	t.Run("Decision", func(t *testing.T) {
		t.Run("Enabled", func(t *testing.T) {
			flag := newFlag(t)

			spanRecorder := tracetest.NewSpanRecorder()
			provider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))
			tracer, _ := otelfeature.Tracer(&otelfeature.Opts{TracerProvider: provider})

			ctx, span := provider.Tracer("").Start(context.Background(), "test")
			tracer.Decision(ctx, flag, true)
			span.End()

			assertEvent(t, getSpan(t, spanRecorder, "test"), "decision",
				otelfeature.AttributeFeatureEnabled.Bool(true),
				otelfeature.AttributeFeatureName.String(flag.Name()))
		})

		t.Run("Disabled", func(t *testing.T) {
			flag := newFlag(t)

			spanRecorder := tracetest.NewSpanRecorder()
			provider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))
			tracer, _ := otelfeature.Tracer(&otelfeature.Opts{TracerProvider: provider})

			ctx, span := provider.Tracer("").Start(context.Background(), "test")
			tracer.Decision(ctx, flag, false)
			span.End()

			assertEvent(t, getSpan(t, spanRecorder, "test"), "decision",
				otelfeature.AttributeFeatureEnabled.Bool(false),
				otelfeature.AttributeFeatureName.String(flag.Name()))
		})
	})

	t.Run("Switch", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			flag := newFlag(t)

			spanRecorder := tracetest.NewSpanRecorder()
			provider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))
			tracer, _ := otelfeature.Tracer(&otelfeature.Opts{TracerProvider: provider})

			_, done := tracer.Switch(context.Background(), flag, true)
			done(nil, nil)

			recordedSpan := getSpan(t, spanRecorder, flag.Name())
			assertAttributeBool(t, recordedSpan.Attributes(), otelfeature.AttributeFeatureEnabled, true)
			assertAttributeString(t, recordedSpan.Attributes(), otelfeature.AttributeFeatureName, flag.Name())
			assertSpanOk(t, recordedSpan)
		})

		t.Run("Error", func(t *testing.T) {
			flag := newFlag(t)

			spanRecorder := tracetest.NewSpanRecorder()
			provider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))
			tracer, _ := otelfeature.Tracer(&otelfeature.Opts{TracerProvider: provider})

			_, done := tracer.Switch(context.Background(), flag, true)
			done(nil, errors.New("failed"))

			recordedSpan := getSpan(t, spanRecorder, flag.Name())
			assertAttributeBool(t, recordedSpan.Attributes(), otelfeature.AttributeFeatureEnabled, true)
			assertAttributeString(t, recordedSpan.Attributes(), otelfeature.AttributeFeatureName, flag.Name())
			assertSpanError(t, recordedSpan, "failed")
		})
	})
}

func assertAttributeBool(tb testing.TB, attrs []attribute.KeyValue, key attribute.Key, want bool) {
	tb.Helper()

	if got := getAttributeOfType(tb, attrs, key, attribute.BOOL).AsBool(); got != want {
		tb.Errorf("got %s = %t, want %t", key, got, want)
	}
}

func assertAttributeString(tb testing.TB, attrs []attribute.KeyValue, key attribute.Key, want string) {
	tb.Helper()

	if got := getAttributeOfType(tb, attrs, key, attribute.STRING).AsString(); got != want {
		tb.Errorf("got %s = %q, want %q", key, got, want)
	}
}

func getAttribute(tb testing.TB, attrs []attribute.KeyValue, key attribute.Key) attribute.Value {
	tb.Helper()

	for _, attr := range attrs {
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

func getAttributeOfType(tb testing.TB, attrs []attribute.KeyValue, key attribute.Key, type_ attribute.Type) attribute.Value {
	tb.Helper()

	value := getAttribute(tb, attrs, key)

	if value.Type() != type_ {
		tb.Fatalf("attribute %s has wrong type: %s", key, value.Type())
	}
	return value
}

func getEvent(tb testing.TB, span trace.ReadOnlySpan, name string) trace.Event {
	tb.Helper()

	for _, event := range span.Events() {
		if event.Name == name {
			return event
		}
	}

	tb.Fatalf("event not found: %s", name)
	return trace.Event{}
}

func assertEvent(tb testing.TB, span trace.ReadOnlySpan, name string, attrs ...attribute.KeyValue) {
	tb.Helper()

	event := getEvent(tb, span, name)

	for _, attr := range attrs {
		switch attr.Value.Type() {
		case attribute.BOOL:
			assertAttributeBool(tb, event.Attributes, attr.Key, attr.Value.AsBool())
		case attribute.STRING:
			assertAttributeString(tb, event.Attributes, attr.Key, attr.Value.AsString())
		default:
			tb.Fatalf("type not handled: %s", attr.Value.Type())
		}
	}
}

func getSpan(tb testing.TB, sr *tracetest.SpanRecorder, name string) trace.ReadOnlySpan {
	tb.Helper()

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

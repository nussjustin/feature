package feature_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/nussjustin/feature"
)

func TestCase_Run_Tracing(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		var set feature.Set

		f := set.New("some flag")

		set.SetStrategy(feature.FixedStrategy(true))
		set.SetTracer(feature.Tracer{
			Decision: assertTracedDecision(t, true),
			Switch:   assertTracedSwitch(t, true, 2, nil),
		})

		_, _ = feature.Switch(context.Background(), f,
			func(ctx context.Context) (int, error) { return 2, nil },
			func(ctx context.Context) (int, error) { return 1, nil })
	})

	t.Run("Error", func(t *testing.T) {
		var set feature.Set

		f := set.New("some flag")

		err1, err2 := errors.New("error 1"), errors.New("error 2")

		set.SetStrategy(feature.FixedStrategy(false))
		set.SetTracer(feature.Tracer{
			Decision: assertTracedDecision(t, false),
			Switch:   assertTracedSwitch(t, false, 1, err1),
		})

		_, _ = feature.Switch(context.Background(), f,
			func(ctx context.Context) (int, error) { return 2, err2 },
			func(ctx context.Context) (int, error) { return 1, err1 })
	})
}

func TestFlag_Enabled_Tracing(t *testing.T) {
	var set feature.Set

	flag := set.New("tracing")
	set.SetStrategy(feature.FixedStrategy(false))

	set.SetTracer(feature.Tracer{Decision: assertTracedDecision(t, false)})
	assertDisabled(t, flag)

	set.SetStrategy(feature.FixedStrategy(true))

	set.SetTracer(feature.Tracer{Decision: assertTracedDecision(t, true)})
	assertEnabled(t, flag)
}

func assertCalled(tb testing.TB, name string) func() {
	var called bool

	tb.Cleanup(func() {
		tb.Helper()

		if !called {
			tb.Errorf("%s not called", name)
		}
	})

	return func() {
		tb.Helper()

		if called {
			tb.Errorf("%s called multiple times", name)
		}

		called = true
	}
}

func assertTracedDecision(
	tb testing.TB,
	want bool,
) func(context.Context, *feature.Flag, bool) {
	called := assertCalled(tb, "Decision")

	return func(ctx context.Context, flag *feature.Flag, got bool) {
		tb.Helper()

		called()

		if got != want {
			tb.Errorf("got %t, want %t", got, want)
		}
	}
}

func assertTracedSwitch(
	tb testing.TB,
	wantDecision bool,
	want any,
	wantErr error,
) func(context.Context, *feature.Flag, bool) (context.Context, func(any, error)) {
	called := assertCalled(tb, "Switch")

	return func(ctx context.Context, _ *feature.Flag, gotDecision bool) (context.Context, func(any, error)) {
		return ctx, func(got any, gotErr error) {
			called()

			if gotDecision != wantDecision {
				tb.Errorf("got decision %t, want %t", gotDecision, wantDecision)
			}

			if !reflect.DeepEqual(got, want) {
				tb.Errorf("got result %v, want %v", got, want)
			}

			assertError(tb, wantErr, gotErr)
		}
	}
}

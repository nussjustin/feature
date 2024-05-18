package feature_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/nussjustin/feature"
)

func TestCase_Experiment_Tracing(t *testing.T) {
	run := func(name string, enabled, disabled func() (int, error), tracerCallback func(testing.TB) feature.Tracer) {
		t.Run(name, func(t *testing.T) {
			var set feature.Set

			set.SetStrategy(feature.FixedStrategy(true))
			set.SetTracer(tracerCallback(t))

			f := set.New("some flag")

			_, _ = feature.Experiment(context.Background(), f,
				func(context.Context) (int, error) { return enabled() },
				func(context.Context) (int, error) { return disabled() },
				feature.Equals[int])
		})
	}

	caseFunc := func(result int, err error) func() (int, error) {
		return func() (int, error) {
			return result, err
		}
	}

	// We return a callback so that the assertions are associated with the correct *testing.T from the subtests.
	tracerCallback := func(
		wantEnabled int,
		wantEnabledErr error,

		wantDisabled int,
		wantDisabledErr error,

		wantSuccess bool,
	) func(testing.TB) feature.Tracer {
		return func(tb testing.TB) feature.Tracer {
			return feature.Tracer{
				Decision:         assertTracedDecision(tb, true),
				Experiment:       assertTracedExperiment(tb, true, wantEnabled, wantEnabledErr, wantSuccess),
				ExperimentBranch: assertTracedExperimentBranch(tb, wantEnabled, wantEnabledErr, wantDisabled, wantDisabledErr),
				Switch:           assertNoTracedSwitch(tb),
			}
		}
	}

	run("Success",
		caseFunc(1, nil),
		caseFunc(1, nil),
		tracerCallback(1, nil, 1, nil, true),
	)

	run("Mismatch",
		caseFunc(2, nil),
		caseFunc(1, nil),
		tracerCallback(2, nil, 1, nil, false),
	)

	errFailed := errors.New("failed")

	run("Experiment failed",
		caseFunc(-1, errFailed),
		caseFunc(1, nil),
		tracerCallback(-1, errFailed, 1, nil, false),
	)

	run("Control failed",
		caseFunc(2, nil),
		caseFunc(-1, errFailed),
		tracerCallback(2, nil, -1, errFailed, false),
	)
}

func TestCase_Run_Tracing(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		var set feature.Set

		f := set.New("some flag")

		set.SetStrategy(feature.FixedStrategy(true))
		set.SetTracer(feature.Tracer{
			Decision:   assertTracedDecision(t, true),
			Experiment: assertNoTracedExperiment(t),
			Switch:     assertTracedSwitch(t, true, 2, nil),
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
			Decision:   assertTracedDecision(t, false),
			Experiment: assertNoTracedExperiment(t),
			Switch:     assertTracedSwitch(t, false, 1, err1),
		})

		_, _ = feature.Switch(context.Background(), f,
			func(ctx context.Context) (int, error) { return 2, err2 },
			func(ctx context.Context) (int, error) { return 1, err1 })
	})
}

func TestFlag_Enabled_Tracing(t *testing.T) {
	var set feature.Set

	flag := set.New("tracing")

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

func assertTracedCase(
	tb testing.TB,
	wantDecision bool,
	want any,
	wantErr error,
) func(context.Context, *feature.Flag, bool) (context.Context, func(any, error)) {
	called := assertCalled(tb, "Case")

	return func(ctx context.Context, _ *feature.Flag, gotDecision bool) (context.Context, func(any, error)) {
		if gotDecision != wantDecision {
			tb.Errorf("got decision %t, want %t", gotDecision, wantDecision)
		}

		return ctx, func(gotResult any, gotErr error) {
			called()

			if !reflect.DeepEqual(gotResult, want) {
				tb.Errorf("got result %v, want %v", gotResult, want)
			}

			assertError(tb, wantErr, gotErr)
		}
	}
}

func assertTracedExperimentBranch(
	tb testing.TB,

	wantEnabled any,
	wantEnabledErr error,

	wantDisabled any,
	wantDisabledErr error,
) func(context.Context, *feature.Flag, bool) (context.Context, func(any, error)) {
	onDisabled := assertTracedCase(tb, false, wantDisabled, wantDisabledErr)
	onEnabled := assertTracedCase(tb, true, wantEnabled, wantEnabledErr)

	return func(ctx context.Context, f *feature.Flag, decision bool) (context.Context, func(any, error)) {
		if decision == true {
			return onEnabled(ctx, f, decision)
		}
		return onDisabled(ctx, f, decision)
	}
}

func assertNoTracedExperiment(tb testing.TB) func(context.Context, *feature.Flag, bool) (context.Context, func(any, error, bool)) {
	return func(context.Context, *feature.Flag, bool) (context.Context, func(any, error, bool)) {
		tb.Fatal("Experiment called")
		return nil, nil
	}
}

func assertTracedExperiment(
	tb testing.TB,
	wantDecision bool,
	want any,
	wantErr error,
	wantSuccess bool,
) func(context.Context, *feature.Flag, bool) (context.Context, func(any, error, bool)) {
	called := assertCalled(tb, "Experiment")

	return func(_ context.Context, _ *feature.Flag, gotDecision bool) (context.Context, func(any, error, bool)) {
		return nil, func(got any, gotErr error, gotSuccess bool) {
			called()

			if gotDecision != wantDecision {
				tb.Errorf("got decision %t, want %t", gotDecision, wantDecision)
			}

			if !reflect.DeepEqual(got, want) {
				tb.Errorf("got result %v, want %v", got, want)
			}

			assertError(tb, wantErr, gotErr)

			if gotSuccess != wantSuccess {
				tb.Errorf("got success %t, want %t", gotSuccess, wantSuccess)
			}
		}
	}
}

func assertNoTracedSwitch(tb testing.TB) func(context.Context, *feature.Flag, bool) (context.Context, func(any, error)) {
	return func(context.Context, *feature.Flag, bool) (context.Context, func(any, error)) {
		tb.Fatal("Switch called")
		return nil, nil
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

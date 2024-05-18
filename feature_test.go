package feature_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"maps"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/nussjustin/feature"
)

func ExampleIf() {
	testerStrategy = feature.StrategyFunc(func(ctx context.Context, _ *feature.Flag) bool {
		// Enable all flags for testers
		return IsTester(ctx)
	})

	feature.SetStrategy(testerStrategy)
}

func TestDecision_Enabled(t *testing.T) {
	assertDecision(t, feature.FixedStrategy(false), "test", false)
	assertDecision(t, feature.FixedStrategy(true), "test", true)
}

func ExampleSetStrategy() {
	// Read initial configuration from local file
	flags := readFlags("flags.json")

	feature.SetStrategy(feature.DecisionMap(flags))

	go func() {
		// Reload flags on SIGUSR1
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGUSR1)

		for range signals {
			flags := readFlags("flags.json")

			feature.SetStrategy(feature.DecisionMap(flags))
		}
	}()

	// Main logic...
}

func TestSetFlags(t *testing.T) {
	if got, want := len(feature.Flags()), 0; got != want {
		t.Errorf("got %d flags, want %d", got, want)
	}

	f1 := feature.New("TestSetFlags/f1")
	f3 := feature.New("TestSetFlags/f3")
	f2 := feature.New("TestSetFlags/f2")

	fs := feature.Flags()

	if got, want := len(fs), 3; got != want {
		t.Errorf("got %d flags, want %d", got, want)
	}

	if fs[0] != f1 {
		t.Errorf("got flag %s at index %d, want %s", fs[0].Name(), 0, f1.Name())
	}

	if fs[1] != f2 {
		t.Errorf("got flag %s at index %d, want %s", fs[1].Name(), 0, f2.Name())
	}

	if fs[2] != f3 {
		t.Errorf("got flag %s at index %d, want %s", fs[2].Name(), 0, f3.Name())
	}
}

func TestSetStrategy(t *testing.T) {
	trim := func(s string) string {
		return strings.TrimPrefix(s, "TestSetStrategy/")
	}

	lower := feature.StrategyFunc(func(_ context.Context, f *feature.Flag) bool {
		return strings.ToLower(trim(f.Name())) == trim(f.Name())
	})

	upper := feature.StrategyFunc(func(_ context.Context, f *feature.Flag) bool {
		return strings.ToUpper(trim(f.Name())) == trim(f.Name())
	})

	lowerFlag := feature.New("TestSetStrategy/lower")
	mixedFlag := feature.New("TestSetStrategy/Mixed")
	upperFlag := feature.New("TestSetStrategy/UPPER")

	// Set first strategy
	feature.SetStrategy(lower)

	assertEnabled(t, lowerFlag)
	assertDisabled(t, mixedFlag)
	assertDisabled(t, upperFlag)

	// Test changing from already set strategy
	feature.SetStrategy(upper)

	assertDisabled(t, lowerFlag)
	assertDisabled(t, mixedFlag)
	assertEnabled(t, upperFlag)
}

func TestSetTracer(t *testing.T) {
	var caseCount, decisionCount, runCount int

	feature.SetTracer(feature.Tracer{
		Decision: func(context.Context, *feature.Flag, bool) {
			decisionCount++
		},
		ExperimentBranch: func(context.Context, *feature.Flag, bool) (context.Context, func(any, error)) {
			caseCount++
			return context.Background(), func(any, error) {}
		},
		Switch: func(context.Context, *feature.Flag, bool) (context.Context, func(any, error)) {
			runCount++
			return context.Background(), func(any, error) {}
		},
	})

	f := feature.New("TestSetTracerProvider")

	_, _ = feature.Switch(context.Background(), f,
		func(context.Context) (int, error) { return 2, nil },
		func(context.Context) (int, error) { return 1, nil })

	if got, want := caseCount, 0; got != want {
		t.Errorf("got %d calls to Case, want %d", got, want)
	}

	if got, want := decisionCount, 1; got != want {
		t.Errorf("got %d calls to Decision, want %d", got, want)
	}

	if got, want := runCount, 1; got != want {
		t.Errorf("got %d calls to Experiment, want %d", got, want)
	}
}

func ExampleExperiment() {
	optimizationFlag := feature.New("optimize-posts-loading",
		feature.WithDescription("enables new query for loading posts"))

	// later

	post, err := feature.Experiment(myCtx, optimizationFlag,
		func(ctx context.Context) (Post, error) { return loadPostOptimized(ctx, postId) },
		func(ctx context.Context) (Post, error) { return loadPost(ctx, postId) },
		feature.Equals[Post])
	if err != nil {
		panic(err)
	}

	fmt.Println(post)
}

func TestCase_Experiment(t *testing.T) {
	newMatchTest := func(want int, equals bool, d bool) func(*testing.T) {
		return func(t *testing.T) {
			var set feature.Set
			set.SetStrategy(feature.FixedStrategy(d))

			f := set.New("case")

			got, err := feature.Experiment(context.Background(), f,
				func(context.Context) (int, error) { return 2, nil },
				func(context.Context) (int, error) { return 1, nil },
				// pretend that both results are equal
				func(new, old int) bool { return equals })
			if err != nil {
				t.Errorf("got error %q, want nil", err)
			}
			if got != want {
				t.Errorf("got n = %d, want %d", got, want)
			}
		}
	}

	t.Run("Match", func(t *testing.T) {
		t.Run("ReturnsOldWhenDisabled", newMatchTest(1, true, false))
		t.Run("ReturnsNewWhenEnabled", newMatchTest(2, true, true))
	})

	t.Run("Mismatch", func(t *testing.T) {
		t.Run("ReturnsOldWhenDisabled", newMatchTest(1, false, false))
		t.Run("ReturnsNewWhenEnabled", newMatchTest(2, false, true))
	})

	t.Run("EqualsIsCalledOnSuccess", func(t *testing.T) {
		var set feature.Set

		set.SetStrategy(feature.FixedStrategy(true))

		f := set.New("case")

		var equalsCalled bool

		_, _ = feature.Experiment(context.Background(), f,
			func(context.Context) (int, error) { return 2, nil },
			func(context.Context) (int, error) { return 1, nil },
			// pretend that both results are equal
			func(new, old int) bool {
				equalsCalled = true

				return true
			})

		if !equalsCalled {
			t.Errorf("equals was not called")
		}
	})

	t.Run("EqualsIsNotCalledOnError", func(t *testing.T) {
		var set feature.Set

		set.SetStrategy(feature.FixedStrategy(true))

		f := set.New("case")

		for _, d := range []bool{false, true} {
			var equalsCalled bool

			equals := func(new, old int) bool {
				equalsCalled = true
				return true
			}

			// Equals should not be called if any of the functions returns an error, even if it's not the enabled
			// function.

			{
				set.SetStrategy(feature.FixedStrategy(d))

				_, _ = feature.Experiment(context.Background(), f,
					func(context.Context) (int, error) { return 2, errors.New("error 2") },
					func(context.Context) (int, error) { return 1, nil },
					equals)

				if equalsCalled {
					t.Errorf("equals was called")
				}
			}

			{
				set.SetStrategy(feature.FixedStrategy(d))

				_, _ = feature.Experiment(context.Background(), f,
					func(context.Context) (int, error) { return 2, nil },
					func(context.Context) (int, error) { return 1, errors.New("error 1") },
					equals)

				if equalsCalled {
					t.Errorf("equals was called")
				}
			}
		}
	})

	t.Run("StrategyIsUsed", func(t *testing.T) {
		var set feature.Set

		set.SetStrategy(feature.FixedStrategy(true))

		f := set.New("case")

		got, err := feature.Experiment(context.Background(), f,
			func(context.Context) (int, error) { return 2, nil },
			func(context.Context) (int, error) { return 1, nil },
			// pretend that both results are equal
			func(new, old int) bool { return true })
		if err != nil {
			t.Errorf("got error %q, want nil", err)
		}
		if want := 2; got != want {
			t.Errorf("got n = %d, want %d", got, want)
		}
	})

	t.Run("FunctionsAreCalledConcurrently", func(t *testing.T) {
		var set feature.Set
		set.SetStrategy(feature.FixedStrategy(false))

		f := set.New("case")

		ping := make(chan int)
		pong := make(chan int)

		got, err := feature.Experiment(context.Background(), f,
			func(context.Context) (int, error) {
				select {
				case ping <- 1:
				case <-time.After(time.Second):
					return 0, errors.New("timeout sending in new")
				}

				select {
				case <-pong:
				case <-time.After(time.Second):
					return 0, errors.New("timeout receiving in new")
				}

				return 2, nil
			},
			func(context.Context) (int, error) {
				select {
				case <-ping:
				case <-time.After(time.Second):
					return 0, errors.New("timeout receiving in old")
				}

				select {
				case pong <- 2:
				case <-time.After(time.Second):
					return 0, errors.New("timeout sending in old")
				}

				return 1, nil
			},
			feature.Equals[int])
		if err != nil {
			t.Errorf("got error %q, want nil", err)
		}
		if want := 1; got != want {
			t.Errorf("got n = %d, want %d", got, want)
		}
	})

	t.Run("OldError", func(t *testing.T) {
		var set feature.Set
		set.SetStrategy(feature.FixedStrategy(false))

		f := set.New("case")

		got, err := feature.Experiment(context.Background(), f,
			func(context.Context) (int, error) { return 2, nil },
			func(context.Context) (int, error) { return 1, errors.New("old failed") },
			feature.Equals[int])
		if err == nil {
			t.Error("got no error, want error")
		}
		if want := 1; got != want {
			t.Errorf("got n = %d, want %d", got, want)
		}

		set.SetStrategy(feature.FixedStrategy(true))

		got, err = feature.Experiment(context.Background(), f,
			func(context.Context) (int, error) { return 2, nil },
			func(context.Context) (int, error) { return 1, errors.New("old failed") },
			feature.Equals[int])
		if err != nil {
			t.Errorf("got error %q, want nil", err)
		}
		if want := 2; got != want {
			t.Errorf("got n = %d, want %d", got, want)
		}
	})

	t.Run("NewError", func(t *testing.T) {
		var set feature.Set
		set.SetStrategy(feature.FixedStrategy(false))

		f := set.New("case")

		got, err := feature.Experiment(context.Background(), f,
			func(context.Context) (int, error) { return 2, errors.New("old failed") },
			func(context.Context) (int, error) { return 1, nil },
			feature.Equals[int])
		if err != nil {
			t.Errorf("got error %q, want nil", err)
		}
		if want := 1; got != want {
			t.Errorf("got n = %d, want %d", got, want)
		}

		set.SetStrategy(feature.FixedStrategy(true))

		got, err = feature.Experiment(context.Background(), f,
			func(context.Context) (int, error) { return 2, errors.New("old failed") },
			func(context.Context) (int, error) { return 1, nil },
			feature.Equals[int])
		if err == nil {
			t.Error("got no error, want error")
		}
		if want := 2; got != want {
			t.Errorf("got n = %d, want %d", got, want)
		}
	})
}

func ExampleSwitch() {
	optimizationFlag := feature.New("optimize-posts-loading",
		feature.WithDescription("enables new query for loading posts"))

	// later

	post, err := feature.Switch(myCtx, optimizationFlag,
		func(ctx context.Context) (Post, error) { return loadPostOptimized(ctx, postId) },
		func(ctx context.Context) (Post, error) { return loadPost(ctx, postId) })
	if err != nil {
		panic(err)
	}

	fmt.Println(post)
}

func TestCase_Switch(t *testing.T) {
	type result struct {
		N     int
		Error error
	}

	testCases := []struct {
		Name     string
		Decision bool
		Old      result
		New      result
		Expected result
	}{
		{
			Name:     "Old by default",
			Old:      result{N: 1},
			Expected: result{N: 1},
		},
		{
			Name:     "Old",
			Decision: false,
			Old:      result{N: 1},
			Expected: result{N: 1},
		},
		{
			Name:     "Old error",
			Decision: false,
			Old:      result{Error: errors.New("test")},
			Expected: result{Error: errors.New("test")},
		},
		{
			Name:     "New",
			Decision: true,
			New:      result{N: 2},
			Expected: result{N: 2},
		},
		{
			Name:     "New error",
			Decision: true,
			New:      result{Error: errors.New("test")},
			Expected: result{Error: errors.New("test")},
		},
	}

	for i := range testCases {
		testCase := testCases[i]

		t.Run(testCase.Name, func(t *testing.T) {
			var set feature.Set

			set.SetStrategy(feature.FixedStrategy(testCase.Decision))

			ctx := context.Background()

			f := set.New("switch")

			n, err := feature.Switch(ctx, f,
				func(ctx context.Context) (int, error) { return testCase.New.N, testCase.New.Error },
				func(ctx context.Context) (int, error) { return testCase.Old.N, testCase.Old.Error })
			if !reflect.DeepEqual(err, testCase.Expected.Error) {
				t.Errorf("got error %v, want %v", err, testCase.Expected.Error)
			}
			if n != testCase.Expected.N {
				t.Errorf("got n = %d, want %d", n, testCase.Expected.N)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	if feature.Equals(1, 3) {
		t.Error("got feature.Equals(1, 3) = true")
	}

	if feature.Equals("test", "Test") {
		t.Error("got feature.Equals(\"test\", \"Test\") = true")
	}

	if !feature.Equals("test", "test") {
		t.Error("got feature.Equals(\"test\", \"test\") = false")
	}
}

func ExampleFlag() {
	// Register flag. Most of the time this will be done globally.
	newUiFlag := feature.New("new-ui", feature.WithDescription("enables the new web ui"))

	// Load old and new UI templates
	oldUI := template.Must(template.ParseGlob("templates/old/*.gotmpl"))
	newUI := template.Must(template.ParseGlob("templates/new/*.gotmpl"))

	http.HandleFunc("/ui", func(w http.ResponseWriter, r *http.Request) {
		// Choose UI based on flag.
		if newUiFlag.Enabled(r.Context()) {
			_ = newUI.Execute(w, nil)
		} else {
			_ = oldUI.Execute(w, nil)
		}
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func TestNewFlag(t *testing.T) {
	t.Run("FailsOnDuplicate", func(t *testing.T) {
		feature.New("TestNewFlag/FailsOnDuplicate")

		assertPanic(t, func() {
			feature.New("TestNewFlag/FailsOnDuplicate")
		})
	})

	t.Run("FailsEmptyName", func(t *testing.T) {
		assertPanic(t, func() {
			feature.New("")
		})
	})

	t.Run("HasMetadata", func(t *testing.T) {
		wantedLabels := map[string]any{"labelA": 1, "labelB": "2"}

		labels := maps.Clone(wantedLabels)

		f := feature.New("TestNewFlag/HasMetadata",
			feature.WithDescription("some description"),
			feature.WithLabels(labels))

		if got, want := f.Name(), "TestNewFlag/HasMetadata"; got != want {
			t.Errorf("got f.Name() = %q, want %q", got, want)
		}

		if got, want := f.Description(), "some description"; got != want {
			t.Errorf("got f.Config() = %q, want %q", got, want)
		}

		labels["labelA"] = 2

		labels = f.Labels()
		labels["labelA"] = 3

		if got, want := f.Labels(), wantedLabels; !maps.Equal(got, want) {
			t.Errorf("got f.Config() = %#v, want %#v", got, want)
		}
	})
}

func TestRegisterFlag(t *testing.T) {
	t.Run("FailsOnCaseWithSameName", func(t *testing.T) {
		var set feature.Set
		set.New("FailsOnCaseWithSameName")

		assertPanic(t, func() {
			set.New("FailsOnCaseWithSameName")
		})
	})

	t.Run("FailsOnDuplicate", func(t *testing.T) {
		var set feature.Set
		set.New("FailsOnDuplicate")

		assertPanic(t, func() {
			set.New("FailsOnDuplicate")
		})
	})
}

func TestFlag_Labels(t *testing.T) {
	var set feature.Set

	f := set.New(t.Name(), feature.WithLabels(map[string]any{"a": "b"}))

	if !maps.Equal(map[string]any{"a": "b"}, f.Labels()) {
		t.Error("flag labels do not match configured labels")
	}

	m := f.Labels()
	delete(m, "a")
	m["c"] = "d"

	if !maps.Equal(map[string]any{"a": "b"}, f.Labels()) {
		t.Error("flag labels were modified")
	}
}

func TestFlag_Enabled(t *testing.T) {
	t.Run("NoStrategy", func(t *testing.T) {
		var set feature.Set

		assertPanic(t, func() {
			assertDisabled(t, set.New("disabled"))
		})
	})

	t.Run("StrategyOnSet", func(t *testing.T) {
		var set feature.Set
		set.SetStrategy(feature.DecisionMap{
			"enabled":  true,
			"disabled": false,
		})
		assertDisabled(t, set.New("disabled"))
		assertEnabled(t, set.New("enabled"))
	})
}

func TestFixedStrategy(t *testing.T) {
	assertDecision(t, feature.FixedStrategy(false), "test", false)
	assertDecision(t, feature.FixedStrategy(true), "test", true)
}

func TestStrategyFunc_Enabled(t *testing.T) {
	s := feature.StrategyFunc(func(_ context.Context, f *feature.Flag) bool {
		return f.Name() == "Rob"
	})

	assertDecision(t, s, "Brad", false)
	assertDecision(t, s, "Rob", true)
}

func ExampleDecisionMap() {
	staticFlagsJSON, err := os.ReadFile("flags.json")
	if err != nil {
		log.Fatalf("failed to read flags JSON: %s", err)
	}

	var staticFlags map[string]bool
	if err := json.Unmarshal(staticFlagsJSON, &staticFlags); err != nil {
		log.Fatalf("failed to parse flags JSON: %s", err)
	}

	strategy := make(feature.DecisionMap, len(staticFlags))
	for name, enabled := range staticFlags {
		strategy[name] = enabled
	}

	feature.SetStrategy(strategy)
}

func TestDecisionMap_Enabled(t *testing.T) {
	s := feature.DecisionMap{
		"Brad": false,
		"Rob":  true,
	}

	assertDecision(t, s, "Brad", false)
	assertDecision(t, s, "Rob", true)

	assertPanic(t, func() {
		assertDecision(t, s, "Disabled", false)
	})
}

func assertEnabled(tb testing.TB, f *feature.Flag) {
	tb.Helper()

	if !f.Enabled(context.Background()) {
		tb.Errorf("flag %q is not enabled", f.Name())
	}
}

func assertDisabled(tb testing.TB, f *feature.Flag) {
	tb.Helper()

	if f.Enabled(context.Background()) {
		tb.Errorf("flag %q is enabled", f.Name())
	}
}

func assertDecision(tb testing.TB, s feature.Strategy, name string, want bool) {
	tb.Helper()

	var set feature.Set

	f := set.New(name)

	if got := s.Enabled(context.Background(), f); got != want {
		tb.Errorf("got %t, want %T", got, want)
	}
}

func assertError(tb testing.TB, want, got error) {
	tb.Helper()

	switch {
	case want == nil && got == nil:
	case want == nil && got != nil:
		tb.Errorf("got error %q, want no error", got)
	case want != nil && got == nil:
		tb.Errorf("got no error, want %s", want)
	case !errors.Is(want, got) && !errors.Is(got, want):
		tb.Errorf("got error %q, want %q", got, want)
	}
}

func assertPanic(tb testing.TB, f func()) {
	tb.Helper()

	defer func() {
		if v := recover(); v == nil {
			tb.Error("no panic was caught, expected a panic to be raised")
		}
	}()

	f()
}

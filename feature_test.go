package feature_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"go.opentelemetry.io/otel/codes"

	"go.opentelemetry.io/otel/sdk/trace"

	"github.com/nussjustin/feature"

	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func ExampleIf() {
	testerStrategy = feature.StrategyFunc(func(ctx context.Context, _ string) feature.Decision {
		// Enable all flags for testers
		return feature.If(IsTester(ctx))
	})

	feature.SetStrategy(testerStrategy)
}

func TestDecision_Enabled(t *testing.T) {
	assertDecision(t, feature.Default, "", feature.Default)
	assertDecision(t, feature.Disabled, "", feature.Disabled)
	assertDecision(t, feature.Enabled, "", feature.Enabled)
}

func ExampleSet_SetStrategy() {
	// Read initial configuration from local file
	flags := readFlags("flags.json")

	feature.SetStrategy(feature.StrategyMap(flags))

	go func() {
		// Reload flags on SIGUSR1
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGUSR1)

		for range signals {
			flags := readFlags("flags.json")

			feature.SetStrategy(feature.StrategyMap(flags))
		}
	}()

	// Main logic...
}

func TestSetStrategy(t *testing.T) {
	trim := func(s string) string {
		return strings.TrimPrefix(s, "TestSetStrategy/")
	}

	lower := feature.StrategyFunc(func(_ context.Context, name string) feature.Decision {
		return feature.If(strings.ToLower(trim(name)) == trim(name))
	})

	upper := feature.StrategyFunc(func(_ context.Context, name string) feature.Decision {
		return feature.If(strings.ToUpper(trim(name)) == trim(name))
	})

	lowerFlag := feature.NewFlag("TestSetStrategy/lower", "", nil, feature.DefaultDisabled)
	mixedFlag := feature.NewFlag("TestSetStrategy/Mixed", "", nil, feature.DefaultDisabled)
	upperFlag := feature.NewFlag("TestSetStrategy/UPPER", "", nil, feature.DefaultDisabled)

	assertDisabled(t, lowerFlag)
	assertDisabled(t, mixedFlag)
	assertDisabled(t, upperFlag)

	feature.SetStrategy(lower)

	assertEnabled(t, lowerFlag)
	assertDisabled(t, mixedFlag)
	assertDisabled(t, upperFlag)

	feature.SetStrategy(upper)

	assertDisabled(t, lowerFlag)
	assertDisabled(t, mixedFlag)
	assertEnabled(t, upperFlag)
}

func TestSetTracerProvider(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	provider := trace.NewTracerProvider(trace.WithSpanProcessor(sr))

	feature.SetTracerProvider(provider)

	c := feature.NewCase[int]("TestSetTracerProvider", "", nil, feature.DefaultDisabled)

	_, _ = c.Run(context.Background(),
		func(context.Context) (int, error) { return 2, nil },
		func(context.Context) (int, error) { return 1, nil })

	if got, want := len(sr.Ended()), 1; got < want {
		t.Errorf("got %d spans, want at least %d", got, want)
	}
}

func TestSet_SetStrategy(t *testing.T) {
	var set feature.Set

	lower := feature.StrategyFunc(func(_ context.Context, name string) feature.Decision {
		return feature.If(strings.ToLower(name) == name)
	})

	upper := feature.StrategyFunc(func(_ context.Context, name string) feature.Decision {
		return feature.If(strings.ToUpper(name) == name)
	})

	lowerFlag := feature.RegisterFlag(&set, "lower", "", nil, feature.DefaultDisabled)
	mixedFlag := feature.RegisterFlag(&set, "Mixed", "", nil, feature.DefaultDisabled)
	upperFlag := feature.RegisterFlag(&set, "UPPER", "", nil, feature.DefaultDisabled)

	assertDisabled(t, lowerFlag)
	assertDisabled(t, mixedFlag)
	assertDisabled(t, upperFlag)

	set.SetStrategy(lower)

	assertEnabled(t, lowerFlag)
	assertDisabled(t, mixedFlag)
	assertDisabled(t, upperFlag)

	set.SetStrategy(upper)

	assertDisabled(t, lowerFlag)
	assertDisabled(t, mixedFlag)
	assertEnabled(t, upperFlag)
}

func ExampleCaseFor() {
	// CaseFor is useful if you have a flag that is already used somewhere and that can not be changed
	// into a Case directly.

	newUiFlag := feature.NewFlag("new-ui", "enables the new web ui", nil, feature.DefaultEnabled)

	// Load old and new UI templates
	oldUI := template.Must(template.ParseGlob("templates/old/*.gotmpl"))
	newUI := template.Must(template.ParseGlob("templates/new/*.gotmpl"))

	http.HandleFunc("/ui", func(w http.ResponseWriter, r *http.Request) {
		_, _ = feature.CaseFor[any](newUiFlag).Run(r.Context(),
			func(ctx context.Context) (any, error) {
				return nil, newUI.Execute(w, nil)
			},
			func(ctx context.Context) (any, error) {
				return nil, oldUI.Execute(w, nil)
			})
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func TestNewCase(t *testing.T) {
	t.Run("FailsOnFlagWithSameName", func(t *testing.T) {
		feature.NewFlag("TestNewCase/FailsOnCaseWithSameName", "", nil, feature.DefaultDisabled)

		assertPanic(t, func() {
			feature.NewCase[any]("TestNewCase/FailsOnCaseWithSameName", "", nil, feature.DefaultDisabled)
		})
	})

	t.Run("FailsOnDuplicate", func(t *testing.T) {
		feature.NewCase[any]("TestNewCase/FailsOnDuplicate", "", nil, feature.DefaultDisabled)

		assertPanic(t, func() {
			feature.NewCase[any]("TestNewCase/FailsOnDuplicate", "", nil, feature.DefaultDisabled)
		})
	})
}

func TestRegisterCase(t *testing.T) {
	t.Run("FailsOnFlagWithSameName", func(t *testing.T) {
		var set feature.Set
		feature.RegisterFlag(&set, "FailsOnCaseWithSameName", "", nil, feature.DefaultDisabled)

		assertPanic(t, func() {
			feature.RegisterCase[any](&set, "FailsOnCaseWithSameName", "", nil, feature.DefaultDisabled)
		})
	})

	t.Run("FailsOnDuplicate", func(t *testing.T) {
		var set feature.Set
		feature.RegisterCase[any](&set, "FailsOnDuplicate", "", nil, feature.DefaultDisabled)

		assertPanic(t, func() {
			feature.RegisterCase[any](&set, "FailsOnDuplicate", "", nil, feature.DefaultDisabled)
		})
	})
}

func ExampleCase_Experiment() {
	optimizationCase := feature.NewCase[Post](
		"optimize-posts-loading",
		"enables new query for loading posts",
		nil,
		feature.DefaultEnabled,
	)

	// later

	post, err := optimizationCase.Experiment(myCtx,
		func(ctx context.Context) (Post, error) { return loadPostOptimized(ctx, postId) },
		func(ctx context.Context) (Post, error) { return loadPost(ctx, postId) },
		feature.Equals[Post])

	if err != nil {
		panic(err)
	}

	fmt.Println(post)
}

func TestCase_Experiment(t *testing.T) {
	newMatchTest := func(want int, equals bool, d feature.DefaultDecision) func(*testing.T) {
		return func(t *testing.T) {
			var set feature.Set

			c := feature.RegisterCase[int](&set, "case", "", nil, d)

			got, err := c.Experiment(context.Background(),
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
		t.Run("ReturnsOldWhenDisabled", newMatchTest(1, true, feature.DefaultDisabled))
		t.Run("ReturnsNewWhenEnabled", newMatchTest(2, true, feature.DefaultEnabled))
	})

	t.Run("Mismatch", func(t *testing.T) {
		t.Run("ReturnsOldWhenDisabled", newMatchTest(1, false, feature.DefaultDisabled))
		t.Run("ReturnsNewWhenEnabled", newMatchTest(2, false, feature.DefaultEnabled))
	})

	t.Run("StrategyIsUsed", func(t *testing.T) {
		var set feature.Set

		set.SetStrategy(feature.Enabled)

		c := feature.RegisterCase[int](&set, "case", "", nil, feature.DefaultDisabled)

		got, err := c.Experiment(context.Background(),
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

		c := feature.RegisterCase[int](&set, "case", "", nil, feature.DefaultDisabled)

		ping := make(chan int)
		pong := make(chan int)

		got, err := c.Experiment(context.Background(),
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

		c := feature.RegisterCase[int](&set, "case", "", nil, feature.DefaultDisabled)

		got, err := c.Experiment(context.Background(),
			func(context.Context) (int, error) { return 2, nil },
			func(context.Context) (int, error) { return 1, errors.New("old failed") },
			feature.Equals[int])
		if err == nil {
			t.Error("got no error, want error")
		}
		if want := 1; got != want {
			t.Errorf("got n = %d, want %d", got, want)
		}

		set.SetStrategy(feature.Enabled)

		got, err = c.Experiment(context.Background(),
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

	t.Run("PanicInOld", func(t *testing.T) {
		var set feature.Set

		c := feature.RegisterCase[int](&set, "case", "", nil, feature.DefaultDisabled)

		got, err := c.Experiment(context.Background(),
			func(context.Context) (int, error) { return 2, nil },
			func(context.Context) (int, error) { panic("old failed") },
			feature.Equals[int])
		if err == nil {
			t.Error("got no error, want error")
		}
		if want := 0; got != want {
			t.Errorf("got n = %d, want %d", got, want)
		}

		set.SetStrategy(feature.Enabled)

		got, err = c.Experiment(context.Background(),
			func(context.Context) (int, error) { return 2, nil },
			func(context.Context) (int, error) { panic("old failed") },
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

		c := feature.RegisterCase[int](&set, "case", "", nil, feature.DefaultDisabled)

		got, err := c.Experiment(context.Background(),
			func(context.Context) (int, error) { return 2, errors.New("old failed") },
			func(context.Context) (int, error) { return 1, nil },
			feature.Equals[int])
		if err != nil {
			t.Errorf("got error %q, want nil", err)
		}
		if want := 1; got != want {
			t.Errorf("got n = %d, want %d", got, want)
		}

		set.SetStrategy(feature.Enabled)

		got, err = c.Experiment(context.Background(),
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

	t.Run("PanicInNew", func(t *testing.T) {
		var set feature.Set

		c := feature.RegisterCase[int](&set, "case", "", nil, feature.DefaultDisabled)

		got, err := c.Experiment(context.Background(),
			func(context.Context) (int, error) { panic("old failed") },
			func(context.Context) (int, error) { return 1, nil },
			feature.Equals[int])
		if err != nil {
			t.Errorf("got error %q, want nil", err)
		}
		if want := 1; got != want {
			t.Errorf("got n = %d, want %d", got, want)
		}

		set.SetStrategy(feature.Enabled)

		got, err = c.Experiment(context.Background(),
			func(context.Context) (int, error) { panic("old failed") },
			func(context.Context) (int, error) { return 1, nil },
			feature.Equals[int])
		if err == nil {
			t.Error("got no error, want error")
		}
		if want := 0; got != want {
			t.Errorf("got n = %d, want %d", got, want)
		}
	})
}

func TestCase_Experiment_Tracing(t *testing.T) {
	var set feature.Set

	sr := tracetest.NewSpanRecorder()
	provider := trace.NewTracerProvider(trace.WithSpanProcessor(sr))

	set.SetTracerProvider(provider)

	const name = "some case"

	c := feature.RegisterCase[int](&set, name, "", nil, feature.DefaultEnabled)

	// Success
	_, _ = c.Experiment(context.Background(),
		func(context.Context) (int, error) { return 1, nil },
		func(context.Context) (int, error) { return 1, nil },
		feature.Equals[int])

	// Mismatch
	_, _ = c.Experiment(context.Background(),
		func(context.Context) (int, error) { return 2, nil },
		func(context.Context) (int, error) { return 1, nil },
		feature.Equals[int])

	// Experiment failed
	_, _ = c.Experiment(context.Background(),
		func(context.Context) (int, error) { return 0, errors.New("failed") },
		func(context.Context) (int, error) { return 1, nil },
		feature.Equals[int])

	// Control failed
	_, _ = c.Experiment(context.Background(),
		func(context.Context) (int, error) { return 2, nil },
		func(context.Context) (int, error) { return 0, errors.New("failed") },
		feature.Equals[int])

	// Experiment panicked
	_, _ = c.Experiment(context.Background(),
		func(context.Context) (int, error) { panic("failed") },
		func(context.Context) (int, error) { return 1, nil },
		feature.Equals[int])

	// Control panicked
	_, _ = c.Experiment(context.Background(),
		func(context.Context) (int, error) { return 2, nil },
		func(context.Context) (int, error) { panic("failed") },
		feature.Equals[int])

	spans := sr.Ended()

	const (
		numberOfCases = 6
		spansPerTrace = 3
	)

	check := func(i int, overall, experimental, control codes.Code) {
		t.Helper()

		byName := map[string]trace.ReadOnlySpan{}
		for _, span := range spans[i*spansPerTrace:][:spansPerTrace] {
			byName[span.Name()] = span
		}

		if got, want := byName["Experiment"].Status().Code, overall; got != want {
			t.Errorf("experiment %d: got Status().Code = %q, want %q", i, got, want)
		}

		assertFeatureAttributes(t, byName["Experiment"].Attributes(), name, true)

		if overall == codes.Ok {
			assertAttributeBool(t, byName["Experiment"].Attributes(), "feature.experiment.match", overall == codes.Ok)
		}

		if got, want := byName["Experimental"].Status().Code, experimental; got != want {
			t.Errorf("experiment %d: got Status().Code = %q, want %q", i, got, want)
		}

		if got, want := byName["Control"].Status().Code, control; got != want {
			t.Errorf("experiment %d: got Status().Code = %q, want %q", i, got, want)
		}
	}

	if got, want := len(spans), numberOfCases*spansPerTrace; got != want {
		t.Fatalf("got %d spans, want %d", got, want)
	}

	// Success
	check(0, codes.Ok, codes.Ok, codes.Ok)

	// Mismatch
	check(1, codes.Error, codes.Ok, codes.Ok)

	// Experiment error
	check(2, codes.Error, codes.Error, codes.Ok)

	// Control error
	check(3, codes.Error, codes.Ok, codes.Error)

	// Experiment panic
	check(4, codes.Error, codes.Error, codes.Ok)

	// Control panic
	check(5, codes.Error, codes.Ok, codes.Error)
}

func ExampleCase_Run() {
	optimizationCase := feature.NewCase[Post](
		"optimize-posts-loading",
		"enables new query for loading posts",
		nil,
		feature.DefaultEnabled,
	)

	// later

	post, err := optimizationCase.Run(myCtx,
		func(ctx context.Context) (Post, error) { return loadPostOptimized(ctx, postId) },
		func(ctx context.Context) (Post, error) { return loadPost(ctx, postId) })

	if err != nil {
		panic(err)
	}

	fmt.Println(post)
}

func TestCase_Run(t *testing.T) {
	type result struct {
		N     int
		Error error
	}

	testCases := []struct {
		Name     string
		Strategy feature.Strategy
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
			Name:     "Old by default via strategy",
			Strategy: feature.Default,
			Old:      result{N: 1},
			Expected: result{N: 1},
		},
		{
			Name:     "Old",
			Strategy: feature.Disabled,
			Old:      result{N: 1},
			Expected: result{N: 1},
		},
		{
			Name:     "Old error",
			Strategy: feature.Disabled,
			Old:      result{Error: errors.New("test")},
			Expected: result{Error: errors.New("test")},
		},
		{
			Name:     "New",
			Strategy: feature.Enabled,
			New:      result{N: 2},
			Expected: result{N: 2},
		},
		{
			Name:     "New error",
			Strategy: feature.Enabled,
			New:      result{Error: errors.New("test")},
			Expected: result{Error: errors.New("test")},
		},
	}

	for i := range testCases {
		testCase := testCases[i]

		t.Run(testCase.Name, func(t *testing.T) {
			var set feature.Set

			set.SetStrategy(testCase.Strategy)

			ctx := context.Background()

			c := feature.RegisterCase[int](&set, "case", "", nil, feature.DefaultDisabled)

			n, err := c.Run(ctx,
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

func TestCase_Run_Tracing(t *testing.T) {
	var set feature.Set

	sr := tracetest.NewSpanRecorder()
	provider := trace.NewTracerProvider(trace.WithSpanProcessor(sr))

	set.SetTracerProvider(provider)

	ctx := context.Background()

	const name = "case name"

	c := feature.RegisterCase[int](&set, name, "", nil, feature.DefaultDisabled)

	for _, strategy := range []feature.Decision{feature.Disabled, feature.Enabled} {
		set.SetStrategy(strategy)

		_, _ = c.Run(ctx,
			func(ctx context.Context) (int, error) { return 2, nil },
			func(ctx context.Context) (int, error) { return 1, nil })

		_, _ = c.Run(ctx,
			func(ctx context.Context) (int, error) { return 2, errors.New("error 2") },
			func(ctx context.Context) (int, error) { return 1, errors.New("error 1") })
	}

	spans := sr.Ended()

	checkSpan := func(i int, enabled bool, code codes.Code) {
		span := spans[i]

		if got, want := span.Name(), "Run"; got != want {
			t.Errorf("got spans[%d].Name() = %q, want %q", i, got, want)
		}

		if got, want := span.Status().Code, code; got != want {
			t.Errorf("got spans[%d].Status().Code = %q, want %q", i, got, want)
		}

		assertFeatureAttributes(t, span.Attributes(), name, enabled)
	}

	if got, want := len(spans), 4; got != want {
		t.Fatalf("got %d spans, want %d", got, want)
	}

	checkSpan(0, false, codes.Ok)
	checkSpan(1, false, codes.Error)

	checkSpan(2, true, codes.Ok)
	checkSpan(3, true, codes.Error)
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
	newUiFlag := feature.NewFlag("new-ui", "enables the new web ui", nil, feature.DefaultEnabled)

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
	t.Run("FailsOnCaseWithSameName", func(t *testing.T) {
		feature.NewCase[any]("TestNewFlag/FailsOnCaseWithSameName", "", nil, feature.DefaultDisabled)

		assertPanic(t, func() {
			feature.NewFlag("TestNewFlag/FailsOnCaseWithSameName", "", nil, feature.DefaultDisabled)
		})
	})

	t.Run("FailsOnDuplicate", func(t *testing.T) {
		feature.NewFlag("TestNewFlag/FailsOnDuplicate", "", nil, feature.DefaultDisabled)

		assertPanic(t, func() {
			feature.NewFlag("TestNewFlag/FailsOnDuplicate", "", nil, feature.DefaultDisabled)
		})
	})

	t.Run("HasMetadata", func(t *testing.T) {
		f := feature.NewFlag("TestNewFlag/HasMetadata", "some description", nil, feature.DefaultEnabled)

		if got, want := f.Name(), "TestNewFlag/HasMetadata"; got != want {
			t.Errorf("got f.Name() = %q, want %q", got, want)
		}

		if got, want := f.Description(), "some description"; got != want {
			t.Errorf("got f.Description() = %q, want %q", got, want)
		}
	})
}

func TestRegisterFlag(t *testing.T) {
	t.Run("FailsOnCaseWithSameName", func(t *testing.T) {
		var set feature.Set
		feature.RegisterCase[any](&set, "FailsOnCaseWithSameName", "", nil, feature.DefaultDisabled)

		assertPanic(t, func() {
			feature.RegisterFlag(&set, "FailsOnCaseWithSameName", "", nil, feature.DefaultDisabled)
		})
	})

	t.Run("FailsOnDuplicate", func(t *testing.T) {
		var set feature.Set
		feature.RegisterFlag(&set, "FailsOnDuplicate", "", nil, feature.DefaultDisabled)

		assertPanic(t, func() {
			feature.RegisterFlag(&set, "FailsOnDuplicate", "", nil, feature.DefaultDisabled)
		})
	})
}

func TestFlag_Enabled(t *testing.T) {
	t.Run("NoStrategy", func(t *testing.T) {
		var set feature.Set
		assertDisabled(t, feature.RegisterFlag(&set, "disabled", "", nil, feature.DefaultDisabled))
		assertEnabled(t, feature.RegisterFlag(&set, "enabled", "", nil, feature.DefaultEnabled))
	})

	t.Run("StrategyOnFlag", func(t *testing.T) {
		var set feature.Set
		assertEnabled(t, feature.RegisterFlag(&set, "disabled", "", feature.Enabled, feature.DefaultDisabled))
		assertDisabled(t, feature.RegisterFlag(&set, "enabled", "", feature.Disabled, feature.DefaultEnabled))
		assertDisabled(t, feature.RegisterFlag(&set, "unknown", "", feature.Default, feature.DefaultDisabled))
	})

	t.Run("StrategyOnSet", func(t *testing.T) {
		var set feature.Set
		set.SetStrategy(feature.StrategyMap{
			"disabled": feature.Enabled,
			"enabled":  feature.Disabled,
		})
		assertEnabled(t, feature.RegisterFlag(&set, "disabled", "", nil, feature.DefaultDisabled))
		assertDisabled(t, feature.RegisterFlag(&set, "enabled", "", nil, feature.DefaultEnabled))
		assertDisabled(t, feature.RegisterFlag(&set, "unknown", "", nil, feature.DefaultDisabled))
	})

	t.Run("Fallback", func(t *testing.T) {
		var set feature.Set
		set.SetStrategy(feature.StrategyMap{
			"disabled1": feature.Default,
			"disabled2": feature.Disabled,
			"enabled1":  feature.Default,
			"enabled3":  feature.Default,
		})
		assertDisabled(t, feature.RegisterFlag(&set, "disabled1", "", nil, feature.DefaultDisabled))
		assertDisabled(t, feature.RegisterFlag(&set, "disabled2", "", feature.Default, feature.DefaultEnabled))
		assertEnabled(t, feature.RegisterFlag(&set, "enabled1", "", nil, feature.DefaultEnabled))
		assertEnabled(t, feature.RegisterFlag(&set, "enabled2", "", nil, feature.DefaultEnabled))
	})
}

func TestStrategyFunc_Enabled(t *testing.T) {
	s := feature.StrategyFunc(func(_ context.Context, name string) feature.Decision {
		return feature.If(name == "Rob")
	})

	assertDecision(t, s, "Brad", feature.Disabled)
	assertDecision(t, s, "Rob", feature.Enabled)
}

func ExampleStrategyMap() {
	staticFlagsJSON, err := os.ReadFile("flags.json")
	if err != nil {
		log.Fatalf("failed to read flags JSON: %s", err)
	}

	var staticFlags map[string]bool
	if err := json.Unmarshal(staticFlagsJSON, &staticFlags); err != nil {
		log.Fatalf("failed to parse flags JSON: %s", err)
	}

	staticStrategy := make(feature.StrategyMap, len(staticFlags))
	for name, enabled := range staticFlags {
		staticStrategy[name] = feature.If(enabled)
	}

	feature.SetStrategy(staticStrategy)
}

func TestStrategyMap_Enabled(t *testing.T) {
	s := feature.StrategyMap{
		"Brad": feature.Disabled,
		"Rob":  feature.Enabled,
	}

	assertDecision(t, s, "Brad", feature.Disabled)
	assertDecision(t, s, "Ian", feature.Default)
	assertDecision(t, s, "Rob", feature.Enabled)
}

func assertAttributeBool(tb testing.TB, attrs []attribute.KeyValue, name string, want bool) {
	tb.Helper()

	for _, attr := range attrs {
		if string(attr.Key) != name {
			continue
		}

		if !attr.Valid() {
			tb.Errorf("attribute %q is invalid", attr.Key)
		}

		if got := attr.Value.AsBool(); got != want {
			tb.Errorf("got value %t for attribute %q, want %t", got, attr.Key, want)
		}

		return
	}

	tb.Errorf("attribute %q not found", name)
}

func assertFeatureAttributes(tb testing.TB, attrs []attribute.KeyValue, name string, enabled bool) {
	tb.Helper()

	if got, want := len(attrs), 2; got < want {
		tb.Errorf("got %d attributes for experiment/run span, want at least %d", got, want)
	}

	for _, attr := range attrs {
		if !attr.Valid() {
			tb.Errorf("attribute %q of experiment/run span is invalid", attr.Key)
		}

		switch attr.Key {
		case "feature.enabled":
			if got, want := attr.Value.AsBool(), enabled; got != want {
				tb.Errorf("got attribute value %t for attribute %q, want %t", got, attr.Key, want)
			}
		case "feature.name":
			if got, want := attr.Value.AsString(), name; got != want {
				tb.Errorf("got attribute value %q for attribute %q, want %s", got, attr.Key, want)
			}
		}
	}
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

func assertDecision(tb testing.TB, s feature.Strategy, name string, want feature.Decision) {
	tb.Helper()

	if got := s.Enabled(context.Background(), name); got != want {
		tb.Errorf("got %q, want %q", got, want)
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

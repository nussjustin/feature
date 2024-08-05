package feature_test

import (
	"context"
	"errors"
	"testing"

	"github.com/nussjustin/feature"

	"github.com/google/go-cmp/cmp"
)

var testRegistry = &feature.SimpleRegistry{
	BoolFunc: func(context.Context, string) bool {
		return true
	},
	FloatFunc: func(context.Context, string) float64 {
		return 2.5
	},
	IntFunc: func(context.Context, string) int {
		return 1
	},
	StringFunc: func(context.Context, string) string {
		return "string"
	},
}

func TestFlagSet_All(t *testing.T) {
	var set feature.FlagSet

	set.Int("int",
		feature.WithDescription("int value"),
		feature.WithLabel("type", "int"))

	set.Bool("bool",
		feature.WithDescription("bool value"),
		feature.WithLabel("type", "bool"))

	set.String("string",
		feature.WithDescription("string value"),
		feature.WithLabel("type", "string"))

	want := make([]feature.Flag, 3)
	want[0], _ = set.Lookup("bool")
	want[1], _ = set.Lookup("int")
	want[2], _ = set.Lookup("string")

	assertEquals(t, want, slicesCollect(set.All), "")
}

func TestFlagSet_Lookup(t *testing.T) {
	var set feature.FlagSet

	set.Bool("flagA")
	set.Bool("flagB", feature.WithDescription("description"))

	flagA, okA := set.Lookup("flagA")
	assertEquals(t, "flagA", flagA.Name, "flagA name mismatch")
	assertEquals(t, "", flagA.Description, "flagA description mismatch")
	assertEquals(t, true, okA, "flagA not marked as ok")

	flagB, okB := set.Lookup("flagB")
	assertEquals(t, "flagB", flagB.Name, "flagB name mismatch")
	assertEquals(t, "description", flagB.Description, "flagB name mismatch")
	assertEquals(t, true, okB, "flagB not marked as ok")

	flagC, okC := set.Lookup("flagC")
	assertEquals(t, "", flagC.Name, "flagC name mismatch")
	assertEquals(t, "", flagC.Description, "flagC name mismatch")
	assertEquals(t, false, okC, "flagC marked as ok")
}

func TestFlagSet_Bool(t *testing.T) {
	t.Run("Duplicate", func(t *testing.T) {
		var set feature.FlagSet
		set.String("test")

		assertPanic(t, feature.ErrDuplicateFlag, func() {
			set.Bool("test")
		})
	})

	t.Run("Register", func(t *testing.T) {
		ctx := context.Background()

		var set feature.FlagSet
		v := set.Bool("test")
		v2 := mustLookup(t, &set, "test").Func.(func(context.Context) bool)

		assertEquals(t, false, v(ctx), "")
		assertEquals(t, false, v2(ctx), "")

		set.SetRegistry(&feature.SimpleRegistry{BoolFunc: func(context.Context, string) bool {
			return true
		}})

		assertEquals(t, true, v(ctx), "")
		assertEquals(t, true, v2(ctx), "")

		set.SetRegistry(&feature.SimpleRegistry{BoolFunc: func(context.Context, string) bool {
			return false
		}})

		assertEquals(t, false, v(ctx), "")
		assertEquals(t, false, v2(ctx), "")
	})
}

func TestFlagSet_Float(t *testing.T) {
	t.Run("Duplicate", func(t *testing.T) {
		var set feature.FlagSet
		set.Bool("test")

		assertPanic(t, feature.ErrDuplicateFlag, func() {
			set.Float("test")
		})
	})

	t.Run("Register", func(t *testing.T) {
		ctx := context.Background()

		var set feature.FlagSet
		v := set.Float("test")
		v2 := mustLookup(t, &set, "test").Func.(func(context.Context) float64)

		assertEquals(t, 0.0, v(ctx), "")
		assertEquals(t, 0.0, v2(ctx), "")

		set.SetRegistry(&feature.SimpleRegistry{FloatFunc: func(context.Context, string) float64 {
			return 1.0
		}})

		assertEquals(t, 1.0, v(ctx), "")
		assertEquals(t, 1.0, v2(ctx), "")

		set.SetRegistry(&feature.SimpleRegistry{FloatFunc: func(context.Context, string) float64 {
			return 2.0
		}})

		assertEquals(t, 2.0, v(ctx), "")
		assertEquals(t, 2.0, v2(ctx), "")
	})
}

func TestFlagSet_Int(t *testing.T) {
	t.Run("Duplicate", func(t *testing.T) {
		var set feature.FlagSet
		set.Float("test")

		assertPanic(t, feature.ErrDuplicateFlag, func() {
			set.Int("test")
		})
	})

	t.Run("Register", func(t *testing.T) {
		ctx := context.Background()

		var set feature.FlagSet
		v := set.Int("test")
		v2 := mustLookup(t, &set, "test").Func.(func(context.Context) int)

		assertEquals(t, 0, v(ctx), "")
		assertEquals(t, 0, v2(ctx), "")

		set.SetRegistry(&feature.SimpleRegistry{IntFunc: func(context.Context, string) int {
			return 1
		}})

		assertEquals(t, 1, v(ctx), "")
		assertEquals(t, 1, v2(ctx), "")

		set.SetRegistry(&feature.SimpleRegistry{IntFunc: func(context.Context, string) int {
			return 2
		}})

		assertEquals(t, 2, v(ctx), "")
		assertEquals(t, 2, v2(ctx), "")
	})
}

func TestFlagSet_String(t *testing.T) {
	t.Run("Duplicate", func(t *testing.T) {
		var set feature.FlagSet
		set.Int("test")

		assertPanic(t, feature.ErrDuplicateFlag, func() {
			set.String("test")
		})
	})

	t.Run("Register", func(t *testing.T) {
		ctx := context.Background()

		var set feature.FlagSet
		v := set.String("test")
		v2 := mustLookup(t, &set, "test").Func.(func(context.Context) string)

		assertEquals(t, "", v(ctx), "")
		assertEquals(t, "", v2(ctx), "")

		set.SetRegistry(&feature.SimpleRegistry{StringFunc: func(context.Context, string) string {
			return "one"
		}})

		assertEquals(t, "one", v(ctx), "")
		assertEquals(t, "one", v2(ctx), "")

		set.SetRegistry(&feature.SimpleRegistry{StringFunc: func(context.Context, string) string {
			return "two"
		}})

		assertEquals(t, "two", v(ctx), "")
		assertEquals(t, "two", v2(ctx), "")
	})
}

func TestLabels(t *testing.T) {
	var s feature.FlagSet

	s.Bool("test",
		feature.WithLabel("labelC", "C"),
		feature.WithLabel("labelB", "B"),
		feature.WithLabel("labelE", "E"),
		feature.WithLabels(map[string]string{
			"labelA": "A",
			"labelE": "E2",
			"labelF": "F",
			"labelD": "D",
		}))

	f := mustLookup(t, &s, "test")

	keys := make([]string, 0, f.Labels.Len())
	labels := make(map[string]string, f.Labels.Len())

	f.Labels.All(func(key string, value string) bool {
		keys = append(keys, key)
		labels[key] = value
		return true
	})

	assertEquals(t, 6, len(keys), "unexpected number of labels")
	assertEquals(t, []string{"labelA", "labelB", "labelC", "labelD", "labelE", "labelF"}, keys, "labels not sorted")
	assertEquals(t, map[string]string{
		"labelA": "A",
		"labelB": "B",
		"labelC": "C",
		"labelD": "D",
		"labelE": "E2",
		"labelF": "F",
	}, labels, "labels do not match")
	assertEquals(t, 6, f.Labels.Len(), "wrong number of labels reported")
}

var globalBool bool

func BenchmarkFlagSet_Bool(b *testing.B) {
	ctx := context.Background()

	var set feature.FlagSet
	set.SetRegistry(testRegistry)

	f := set.Bool("test")
	b.ReportAllocs()

	for range b.N {
		globalBool = f(ctx)
	}
}

var globalFloat float64

func BenchmarkFlagSet_Float(b *testing.B) {
	ctx := context.Background()

	var set feature.FlagSet
	set.SetRegistry(testRegistry)

	f := set.Float("test")
	b.ReportAllocs()

	for range b.N {
		globalFloat = f(ctx)
	}
}

var globalInt int

func BenchmarkFlagSet_Int(b *testing.B) {
	ctx := context.Background()

	var set feature.FlagSet
	set.SetRegistry(testRegistry)

	f := set.Int("test")
	b.ReportAllocs()

	for range b.N {
		globalInt = f(ctx)
	}
}

var globalString string

func BenchmarkFlagSet_String(b *testing.B) {
	ctx := context.Background()

	var set feature.FlagSet
	set.SetRegistry(testRegistry)

	f := set.String("test")
	b.ReportAllocs()

	for range b.N {
		globalString = f(ctx)
	}
}

func assertEquals[T any](tb testing.TB, want, got T, msg string) {
	tb.Helper()

	if msg == "" {
		msg = "result mismatch"
	}

	labelsComparer := cmp.Comparer(func(x, y feature.Labels) bool {
		return cmp.Equal(mapsCollect(x.All), mapsCollect(y.All))
	})

	flagComparer := cmp.Comparer(func(x, y feature.Flag) bool {
		return x.Name == y.Name && x.Description == y.Description && cmp.Equal(x.Labels, y.Labels, labelsComparer)
	})

	if diff := cmp.Diff(want, got, flagComparer, labelsComparer); diff != "" {
		tb.Errorf("%s (-want +got):\n%s", msg, diff)
	}
}

func assertPanic(tb testing.TB, want error, f func()) {
	defer func() {
		got := recover()
		if got == nil {
			tb.Errorf("expected panic with error %q, call did not panic", want)
		}
		gotErr, ok := got.(error)
		if !ok {
			tb.Fatalf("recovered value is not an error: %#v", got)
		}
		if !errors.Is(gotErr, want) {
			tb.Errorf("expected error %q, got %q", want, gotErr)
		}
	}()

	f()
}

func mustLookup(tb testing.TB, set *feature.FlagSet, name string) feature.Flag {
	tb.Helper()

	f, ok := set.Lookup(name)
	if !ok {
		tb.Fatalf("flag %q not found", name)
	}

	return f
}

type iterSeq[V any] func(yield func(V) bool)
type iterSeq2[K, V any] func(yield func(K, V) bool)

func mapsCollect[K comparable, V any](seq iterSeq2[K, V]) map[K]V {
	m := make(map[K]V)

	seq(func(k K, v V) bool {
		m[k] = v
		return true
	})

	return m
}

func slicesCollect[E any](seq iterSeq[E]) []E {
	var s []E
	seq(func(e E) bool {
		s = append(s, e)
		return true
	})
	return s
}

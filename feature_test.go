package feature_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/nussjustin/feature"
)

func TestFlagSet_All(t *testing.T) {
	var set feature.FlagSet

	set.Int("int", 0,
		feature.WithDescription("int value"),
		feature.WithLabel("type", "int"))

	set.Bool("bool", false,
		feature.WithDescription("bool value"),
		feature.WithLabel("type", "bool"))

	set.String("string", "",
		feature.WithDescription("string value"),
		feature.WithLabel("type", "string"))

	set.Float("float", 0.0,
		feature.WithDescription("float value"),
		feature.WithLabel("type", "float"))

	set.Uint("uint", 0,
		feature.WithDescription("uint value"),
		feature.WithLabel("type", "uint"))

	want := make([]feature.Flag, 5)
	want[0], _ = set.Lookup("bool")
	want[1], _ = set.Lookup("float")
	want[2], _ = set.Lookup("int")
	want[3], _ = set.Lookup("string")
	want[4], _ = set.Lookup("uint")

	assertEquals(t, want, slicesCollect(set.All), "")
}

func TestFlagSet_Lookup(t *testing.T) {
	var set feature.FlagSet

	set.Bool("flagA", false)
	set.String("flagB", "test", feature.WithDescription("description"))

	flagA, okA := set.Lookup("flagA")
	assertEquals(t, feature.FlagKindBool, flagA.Kind, "flagA kind mismatch")
	assertEquals(t, "flagA", flagA.Name, "flagA name mismatch")
	assertEquals(t, false, flagA.Value, "flagA value mismatch")
	assertEquals(t, "", flagA.Description, "flagA description mismatch")
	assertEquals(t, true, okA, "flagA not marked as ok")

	flagB, okB := set.Lookup("flagB")
	assertEquals(t, feature.FlagKindString, flagB.Kind, "flagB kind mismatch")
	assertEquals(t, "flagB", flagB.Name, "flagB name mismatch")
	assertEquals(t, "test", flagB.Value, "flagB value mismatch")
	assertEquals(t, "description", flagB.Description, "flagB name mismatch")
	assertEquals(t, true, okB, "flagB not marked as ok")

	flagC, okC := set.Lookup("flagC")
	assertEquals(t, feature.FlagKindInvalid, flagC.Kind, "flagC kind mismatch")
	assertEquals(t, "", flagC.Name, "flagC name mismatch")
	assertEquals(t, nil, flagC.Value, "flagC value mismatch")
	assertEquals(t, "", flagC.Description, "flagC name mismatch")
	assertEquals(t, false, okC, "flagC marked as ok")
}

func TestFlagSet_Context(t *testing.T) {
	t.Run("Ignores unknown", func(t *testing.T) {
		var set feature.FlagSet

		set.Context(t.Context(), feature.StringValue("test", "value"))
	})

	t.Run("Panics on wrong type", func(t *testing.T) {
		var set feature.FlagSet
		set.Int("test", 5)

		assertPanicErrorString(t, `invalid value kind for flag "test"`, func() {
			set.Context(t.Context(), feature.StringValue("test", "value"))
		})
	})
}

func TestFlagSet_Bool(t *testing.T) {
	t.Run("Duplicate", func(t *testing.T) {
		var set feature.FlagSet
		set.String("test", "")

		assertPanic(t, feature.ErrDuplicateFlag, func() {
			set.Bool("test", false)
		})
	})

	t.Run("Default", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v := set.Bool("test", false)

		assertEquals(t, false, v(ctx), "")
	})

	t.Run("Value", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v1 := set.Bool("test1", false)
		v2 := set.Bool("test2", true)

		ctx = set.Context(ctx,
			feature.BoolValue("test1", true),
			feature.BoolValue("test2", false))

		var otherSet feature.FlagSet
		ctx = otherSet.Context(ctx,
			feature.BoolValue("test1", false),
			feature.BoolValue("test2", true))

		assertEquals(t, true, v1(ctx), "")
		assertEquals(t, false, v2(ctx), "")
	})
}

func TestFlagSet_Float(t *testing.T) {
	t.Run("Duplicate", func(t *testing.T) {
		var set feature.FlagSet
		set.Bool("test", false)

		assertPanic(t, feature.ErrDuplicateFlag, func() {
			set.Float("test", 0.0)
		})
	})

	t.Run("Default", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v := set.Float("test", 5.0)

		assertEquals(t, 5.0, v(ctx), "")
	})

	t.Run("Value", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v1 := set.Float("test1", 5.0)
		v2 := set.Float("test2", 10.0)

		ctx = set.Context(ctx,
			feature.FloatValue("test1", 15.0),
			feature.FloatValue("test2", 20.0))

		var otherSet feature.FlagSet
		ctx = otherSet.Context(ctx,
			feature.FloatValue("test1", 25.0),
			feature.FloatValue("test2", 30.0))

		assertEquals(t, 15.0, v1(ctx), "")
		assertEquals(t, 20.0, v2(ctx), "")
	})
}

func TestFlagSet_Int(t *testing.T) {
	t.Run("Duplicate", func(t *testing.T) {
		var set feature.FlagSet
		set.Float("test", 0.0)

		assertPanic(t, feature.ErrDuplicateFlag, func() {
			set.Int("test", 0)
		})
	})

	t.Run("Default", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v := set.Int("test", 5)

		assertEquals(t, 5, v(ctx), "")
	})

	t.Run("Value", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v1 := set.Int("test1", 5)
		v2 := set.Int("test2", 10)

		ctx = set.Context(ctx,
			feature.IntValue("test1", 15),
			feature.IntValue("test2", 20))

		var otherSet feature.FlagSet
		ctx = otherSet.Context(ctx,
			feature.IntValue("test1", 25),
			feature.IntValue("test2", 30))

		assertEquals(t, 15, v1(ctx), "")
		assertEquals(t, 20, v2(ctx), "")
	})
}

func TestFlagSet_String(t *testing.T) {
	t.Run("Duplicate", func(t *testing.T) {
		var set feature.FlagSet
		set.Int("test", 0)

		assertPanic(t, feature.ErrDuplicateFlag, func() {
			set.String("test", "")
		})
	})

	t.Run("Default", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v := set.String("test", "default")

		assertEquals(t, "default", v(ctx), "")
	})

	t.Run("Value", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v1 := set.String("test1", "test1")
		v2 := set.String("test2", "test2")

		ctx = set.Context(ctx,
			feature.StringValue("test1", "test1 changed"),
			feature.StringValue("test2", "test2 changed"))

		var otherSet feature.FlagSet
		ctx = otherSet.Context(ctx,
			feature.StringValue("test1", "test1 changed again"),
			feature.StringValue("test2", "test2 changed again"))

		assertEquals(t, "test1 changed", v1(ctx), "")
		assertEquals(t, "test2 changed", v2(ctx), "")
	})
}

func TestFlagSet_Uint(t *testing.T) {
	t.Run("Duplicate", func(t *testing.T) {
		var set feature.FlagSet
		set.Float("test", 0.0)

		assertPanic(t, feature.ErrDuplicateFlag, func() {
			set.Uint("test", 0)
		})
	})

	t.Run("Default", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v := set.Uint("test", 5)

		assertEquals(t, 5, v(ctx), "")
	})

	t.Run("Value", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v1 := set.Uint("test1", 5)
		v2 := set.Uint("test2", 10)

		ctx = set.Context(ctx,
			feature.UintValue("test1", 15),
			feature.UintValue("test2", 20))

		var otherSet feature.FlagSet
		ctx = otherSet.Context(ctx,
			feature.UintValue("test1", 25),
			feature.UintValue("test2", 30))

		assertEquals(t, 15, v1(ctx), "")
		assertEquals(t, 20, v2(ctx), "")
	})
}

func TestLabels(t *testing.T) {
	var s feature.FlagSet

	s.Bool("test", false,
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

func BenchmarkFlagSet_Bool(b *testing.B) {
	b.Run("Context", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.Bool("test", false)
		ctx := set.Context(b.Context(), feature.BoolValue("test", true))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})

	b.Run("Default", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.Bool("test", false)
		ctx := set.Context(b.Context(), feature.BoolValue("unused", false))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})
}

func BenchmarkFlagSet_Float(b *testing.B) {
	b.Run("Context", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.Float("test", 5.0)
		ctx := set.Context(b.Context(), feature.FloatValue("test", 5.0))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})

	b.Run("Default", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.Float("test", 5.0)
		ctx := set.Context(b.Context(), feature.FloatValue("unused", 0.0))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})
}

func BenchmarkFlagSet_Int(b *testing.B) {
	b.Run("Context", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.Int("test", 5.0)
		ctx := set.Context(b.Context(), feature.IntValue("test", 5))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})

	b.Run("Default", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.Int("test", 5.0)
		ctx := set.Context(b.Context(), feature.IntValue("unused", 0))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})
}

func BenchmarkFlagSet_String(b *testing.B) {
	b.Run("Context", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.String("test", "test")
		ctx := set.Context(b.Context(), feature.StringValue("test", "test"))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})

	b.Run("Default", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.String("test", "test")
		ctx := set.Context(b.Context(), feature.StringValue("unused", ""))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})
}

func BenchmarkFlagSet_Uint(b *testing.B) {
	b.Run("Context", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.Uint("test", 5)
		ctx := set.Context(b.Context(), feature.UintValue("test", 5))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})

	b.Run("Default", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.Uint("test", 5)
		ctx := set.Context(b.Context(), feature.UintValue("unused", 0))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})
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
	tb.Helper()

	defer func() {
		tb.Helper()

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

func assertPanicErrorString(tb testing.TB, want string, f func()) {
	tb.Helper()

	defer func() {
		tb.Helper()

		got := recover()
		if got == nil {
			tb.Errorf("expected panic with error %q, call did not panic", want)
		}
		gotErr, ok := got.(error)
		if !ok {
			tb.Fatalf("recovered value is not an error: %#v", got)
		}
		if gotErr.Error() != want {
			tb.Errorf("expected error %q, got %q", want, gotErr.Error())
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

type (
	iterSeq[V any]     func(yield func(V) bool)
	iterSeq2[K, V any] func(yield func(K, V) bool)
)

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

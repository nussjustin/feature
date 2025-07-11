package feature_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/nussjustin/feature"
)

func TestFlagSet_All(t *testing.T) {
	var set feature.FlagSet

	set.Bool("bool", "bool value", false)
	set.BoolFunc("bool-func", "bool value", func(context.Context) bool {
		return false
	})

	set.Duration("duration", "duration value", 0)
	set.DurationFunc("duration-func", "duration value", func(context.Context) time.Duration {
		return 0
	})

	set.Float64("float64", "float64 value", 0.0)
	set.Float64Func("float64-func", "float64 value", func(context.Context) float64 {
		return 0.0
	})

	set.Int("int", "int value", 0)
	set.IntFunc("int-func", "int value", func(context.Context) int {
		return 0
	})

	set.String("string", "string value", "")
	set.StringFunc("string-func", "string value", func(context.Context) string {
		return ""
	})

	set.Uint("uint", "uint value", 0)
	set.UintFunc("uint-func", "uint value", func(context.Context) uint {
		return 0
	})

	want := make([]feature.Flag, 12)
	want[0], _ = set.Lookup("bool")
	want[1], _ = set.Lookup("bool-func")
	want[2], _ = set.Lookup("duration")
	want[3], _ = set.Lookup("duration-func")
	want[4], _ = set.Lookup("float64")
	want[5], _ = set.Lookup("float64-func")
	want[6], _ = set.Lookup("int")
	want[7], _ = set.Lookup("int-func")
	want[8], _ = set.Lookup("string")
	want[9], _ = set.Lookup("string-func")
	want[10], _ = set.Lookup("uint")
	want[11], _ = set.Lookup("uint-func")

	assertEquals(t, want, slices.Collect(set.All), "")
}

func TestFlagSet_Lookup(t *testing.T) {
	var set feature.FlagSet

	set.Bool("flagA", "", false)
	set.String("flagB", "description", "test")

	flagA, okA := set.Lookup("flagA")
	assertEquals(t, feature.FlagKindBool, flagA.Kind, "flagA kind mismatch")
	assertEquals(t, "flagA", flagA.Name, "flagA name mismatch")
	assertEquals(t, "", flagA.Description, "flagA description mismatch")
	assertEquals(t, true, okA, "flagA not marked as ok")

	flagB, okB := set.Lookup("flagB")
	assertEquals(t, feature.FlagKindString, flagB.Kind, "flagB kind mismatch")
	assertEquals(t, "flagB", flagB.Name, "flagB name mismatch")
	assertEquals(t, "description", flagB.Description, "flagB name mismatch")
	assertEquals(t, true, okB, "flagB not marked as ok")

	flagC, okC := set.Lookup("flagC")
	assertEquals(t, feature.FlagKindInvalid, flagC.Kind, "flagC kind mismatch")
	assertEquals(t, "", flagC.Name, "flagC name mismatch")
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
		set.Int("test", "test flag", 5)

		assertPanicErrorString(t, `invalid value kind for flag "test"`, func() {
			set.Context(t.Context(), feature.StringValue("test", "value"))
		})
	})
}

func TestFlagSet_Bool(t *testing.T) {
	t.Run("Duplicate", func(t *testing.T) {
		var set feature.FlagSet
		set.String("test", "test flag", "")

		assertPanic(t, feature.ErrDuplicateFlag, func() {
			set.Bool("test", "test flag", false)
		})
	})

	t.Run("Default", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v := set.Bool("test", "test flag", false)

		assertEquals(t, false, v(ctx), "")
	})

	t.Run("Override", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v1 := set.Bool("test1", "test flag", false)
		v2 := set.Bool("test2", "test flag", true)

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

func TestFlagSet_Duration(t *testing.T) {
	t.Run("Duplicate", func(t *testing.T) {
		var set feature.FlagSet
		set.Float64("test", "test flag", 0.0)

		assertPanic(t, feature.ErrDuplicateFlag, func() {
			set.Duration("test", "test flag", 0)
		})
	})

	t.Run("Default", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v := set.Duration("test", "test flag", 5)

		assertEquals(t, 5, v(ctx), "")
	})

	t.Run("Override", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v1 := set.Duration("test1", "test flag", 5*time.Second)
		v2 := set.Duration("test2", "test flag", 10*time.Second)

		ctx = set.Context(ctx,
			feature.DurationValue("test1", 15*time.Second),
			feature.DurationValue("test2", 20*time.Second))

		var otherSet feature.FlagSet
		ctx = otherSet.Context(ctx,
			feature.DurationValue("test1", 25*time.Second),
			feature.DurationValue("test2", 30*time.Second))

		assertEquals(t, 15*time.Second, v1(ctx), "")
		assertEquals(t, 20*time.Second, v2(ctx), "")
	})
}

func TestFlagSet_Float(t *testing.T) {
	t.Run("Duplicate", func(t *testing.T) {
		var set feature.FlagSet
		set.Bool("test", "test flag", false)

		assertPanic(t, feature.ErrDuplicateFlag, func() {
			set.Float64("test", "test flag", 0.0)
		})
	})

	t.Run("Default", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v := set.Float64("test", "test flag", 5.0)

		assertEquals(t, 5.0, v(ctx), "")
	})

	t.Run("Override", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v1 := set.Float64("test1", "test flag", 5.0)
		v2 := set.Float64("test2", "test flag", 10.0)

		ctx = set.Context(ctx,
			feature.Float64Value("test1", 15.0),
			feature.Float64Value("test2", 20.0))

		var otherSet feature.FlagSet
		ctx = otherSet.Context(ctx,
			feature.Float64Value("test1", 25.0),
			feature.Float64Value("test2", 30.0))

		assertEquals(t, 15.0, v1(ctx), "")
		assertEquals(t, 20.0, v2(ctx), "")
	})
}

func TestFlagSet_Int(t *testing.T) {
	t.Run("Duplicate", func(t *testing.T) {
		var set feature.FlagSet
		set.Float64("test", "test flag", 0.0)

		assertPanic(t, feature.ErrDuplicateFlag, func() {
			set.Int("test", "test flag", 0)
		})
	})

	t.Run("Default", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v := set.Int("test", "test flag", 5)

		assertEquals(t, 5, v(ctx), "")
	})

	t.Run("Override", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v1 := set.Int("test1", "test flag", 5)
		v2 := set.Int("test2", "test flag", 10)

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
		set.Int("test", "test flag", 0)

		assertPanic(t, feature.ErrDuplicateFlag, func() {
			set.String("test", "test flag", "")
		})
	})

	t.Run("Default", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v := set.String("test", "test flag", "default")

		assertEquals(t, "default", v(ctx), "")
	})

	t.Run("Override", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v1 := set.String("test1", "test flag", "test1")
		v2 := set.String("test2", "test flag", "test2")

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
		set.Float64("test", "test flag", 0.0)

		assertPanic(t, feature.ErrDuplicateFlag, func() {
			set.Uint("test", "test flag", 0)
		})
	})

	t.Run("Default", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v := set.Uint("test", "test flag", 5)

		assertEquals(t, 5, v(ctx), "")
	})

	t.Run("Override", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v1 := set.Uint("test1", "test flag", 5)
		v2 := set.Uint("test2", "test flag", 10)

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

func BenchmarkFlagSet_Bool(b *testing.B) {
	b.Run("Context", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.Bool("test", "test flag", false)
		ctx := set.Context(b.Context(), feature.BoolValue("test", true))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})

	b.Run("Default", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.Bool("test", "test flag", false)
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
		flag := set.Float64("test", "test flag", 5.0)
		ctx := set.Context(b.Context(), feature.Float64Value("test", 5.0))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})

	b.Run("Default", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.Float64("test", "test flag", 5.0)
		ctx := set.Context(b.Context(), feature.Float64Value("unused", 0.0))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})
}

func BenchmarkFlagSet_Int(b *testing.B) {
	b.Run("Context", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.Int("test", "test flag", 5.0)
		ctx := set.Context(b.Context(), feature.IntValue("test", 5))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})

	b.Run("Default", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.Int("test", "test flag", 5.0)
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
		flag := set.String("test", "test flag", "test")
		ctx := set.Context(b.Context(), feature.StringValue("test", "test"))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})

	b.Run("Default", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.String("test", "test flag", "test")
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
		flag := set.Uint("test", "test flag", 5)
		ctx := set.Context(b.Context(), feature.UintValue("test", 5))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})

	b.Run("Default", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.Uint("test", "test flag", 5)
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

	flagComparer := cmp.Comparer(func(x, y feature.Flag) bool {
		return x.Name == y.Name && x.Description == y.Description
	})

	if diff := cmp.Diff(want, got, flagComparer); diff != "" {
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

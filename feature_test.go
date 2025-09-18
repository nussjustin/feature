package feature_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/nussjustin/feature"
)

func TestFlagSet_All(t *testing.T) {
	var set feature.FlagSet

	set.Any("any", "any value", nil)
	set.AnyFunc("any-func", "any value", func(context.Context) any {
		return nil
	})

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

	want := make([]feature.Flag, 14)
	want[0], _ = set.Lookup("any")
	want[1], _ = set.Lookup("any-func")
	want[2], _ = set.Lookup("bool")
	want[3], _ = set.Lookup("bool-func")
	want[4], _ = set.Lookup("duration")
	want[5], _ = set.Lookup("duration-func")
	want[6], _ = set.Lookup("float64")
	want[7], _ = set.Lookup("float64-func")
	want[8], _ = set.Lookup("int")
	want[9], _ = set.Lookup("int-func")
	want[10], _ = set.Lookup("string")
	want[11], _ = set.Lookup("string-func")
	want[12], _ = set.Lookup("uint")
	want[13], _ = set.Lookup("uint-func")

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
	assertEquals(t, false, flagA.Func.(feature.Func[bool])(t.Context()), "flagA func returned wrong value")
	assertEquals(t, true, okA, "flagA not marked as ok")

	flagB, okB := set.Lookup("flagB")
	assertEquals(t, feature.FlagKindString, flagB.Kind, "flagB kind mismatch")
	assertEquals(t, "flagB", flagB.Name, "flagB name mismatch")
	assertEquals(t, "description", flagB.Description, "flagB name mismatch")
	assertEquals(t, "test", flagB.Func.(feature.Func[string])(t.Context()), "flagA func returned wrong value")
	assertEquals(t, true, okB, "flagB not marked as ok")

	flagC, okC := set.Lookup("flagC")
	assertEquals(t, feature.FlagKindInvalid, flagC.Kind, "flagC kind mismatch")
	assertEquals(t, "", flagC.Name, "flagC name mismatch")
	assertEquals(t, "", flagC.Description, "flagC name mismatch")
	assertEquals(t, nil, flagC.Func, "flagC func mismatch")
	assertEquals(t, false, okC, "flagC marked as ok")
}

func TestFlagSet_WithValue(t *testing.T) {
	t.Run("Panics on unknown", func(t *testing.T) {
		var set feature.FlagSet

		assertPanicErrorString(t, `flag "test" not found`, func() {
			set.WithValue(t.Context(), feature.StringValue("test", "value"))
		})
	})

	t.Run("Panics on wrong type", func(t *testing.T) {
		var set feature.FlagSet
		set.Int("test", "test flag", 5)

		assertPanicErrorString(t, `invalid value kind for flag "test"`, func() {
			set.WithValue(t.Context(), feature.StringValue("test", "value"))
		})
	})
}

func TestFlagSet_WithValues(t *testing.T) {
	t.Run("Panics on unknown", func(t *testing.T) {
		var set feature.FlagSet

		assertPanicErrorString(t, `flag "test" not found`, func() {
			set.WithValues(t.Context(), feature.StringValue("test", "value"))
		})
	})

	t.Run("Panics on wrong type", func(t *testing.T) {
		var set feature.FlagSet
		set.Int("test", "test flag", 5)

		assertPanicErrorString(t, `invalid value kind for flag "test"`, func() {
			set.WithValues(t.Context(), feature.StringValue("test", "value"))
		})
	})
}

var testFlagKey = new(int)

func hasTestFlag(ctx context.Context) bool {
	return ctx.Value(testFlagKey) != nil
}

func withTestFlag(ctx context.Context) context.Context {
	return context.WithValue(ctx, testFlagKey, true)
}

func TestFlagSet_Any(t *testing.T) {
	t.Run("Duplicate", func(t *testing.T) {
		var set feature.FlagSet
		set.String("test", "test flag", "")

		assertPanic(t, feature.ErrDuplicateFlag, func() {
			set.Any("test", "test flag", nil)
		})
	})

	t.Run("Default", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v := set.Any("test", "test flag", nil)

		assertEquals(t, nil, v(ctx), "")
	})

	t.Run("Func", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v := set.AnyFunc("test", "test flag", func(ctx context.Context) any {
			if hasTestFlag(ctx) {
				return "test"
			}
			return nil
		})

		assertEquals(t, nil, v(ctx), "")
		assertEquals(t, "test", v(withTestFlag(ctx)), "")
	})

	t.Run("Override", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v1 := set.Any("test1", "test flag", false)
		v2 := set.Any("test2", "test flag", true)

		ctx = set.WithValues(ctx,
			feature.AnyValue("test1", true),
			feature.AnyValue("test2", false))

		assertEquals(t, true, v1(ctx), "")
		assertEquals(t, false, v2(ctx), "")
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

	t.Run("Func", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v := set.BoolFunc("test", "test flag", func(ctx context.Context) bool {
			return hasTestFlag(ctx)
		})

		assertEquals(t, false, v(ctx), "")
		assertEquals(t, true, v(withTestFlag(ctx)), "")
	})

	t.Run("Override", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v1 := set.Bool("test1", "test flag", false)
		v2 := set.Bool("test2", "test flag", true)

		ctx = set.WithValues(ctx,
			feature.BoolValue("test1", true),
			feature.BoolValue("test2", false))

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

	t.Run("Func", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v := set.DurationFunc("test", "test flag", func(ctx context.Context) time.Duration {
			if hasTestFlag(ctx) {
				return time.Second
			}
			return 0
		})

		assertEquals(t, 0, v(ctx), "")
		assertEquals(t, time.Second, v(withTestFlag(ctx)), "")
	})

	t.Run("Override", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v1 := set.Duration("test1", "test flag", 5*time.Second)
		v2 := set.Duration("test2", "test flag", 10*time.Second)

		ctx = set.WithValues(ctx,
			feature.DurationValue("test1", 15*time.Second),
			feature.DurationValue("test2", 20*time.Second))

		assertEquals(t, 15*time.Second, v1(ctx), "")
		assertEquals(t, 20*time.Second, v2(ctx), "")
	})
}

func TestFlagSet_Float64(t *testing.T) {
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

	t.Run("Func", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v := set.Float64Func("test", "test flag", func(ctx context.Context) float64 {
			if hasTestFlag(ctx) {
				return 1
			}
			return 0
		})

		assertEquals(t, 0, v(ctx), "")
		assertEquals(t, 1, v(withTestFlag(ctx)), "")
	})

	t.Run("Override", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v1 := set.Float64("test1", "test flag", 5.0)
		v2 := set.Float64("test2", "test flag", 10.0)

		ctx = set.WithValues(ctx,
			feature.Float64Value("test1", 15.0),
			feature.Float64Value("test2", 20.0))

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

	t.Run("Func", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v := set.IntFunc("test", "test flag", func(ctx context.Context) int {
			if hasTestFlag(ctx) {
				return 1
			}
			return 0
		})

		assertEquals(t, 0, v(ctx), "")
		assertEquals(t, 1, v(withTestFlag(ctx)), "")
	})

	t.Run("Override", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v1 := set.Int("test1", "test flag", 5)
		v2 := set.Int("test2", "test flag", 10)

		ctx = set.WithValues(ctx,
			feature.IntValue("test1", 15),
			feature.IntValue("test2", 20))

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

	t.Run("Func", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v := set.StringFunc("test", "test flag", func(ctx context.Context) string {
			if hasTestFlag(ctx) {
				return "test"
			}
			return ""
		})

		assertEquals(t, "", v(ctx), "")
		assertEquals(t, "test", v(withTestFlag(ctx)), "")
	})

	t.Run("Override", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v1 := set.String("test1", "test flag", "test1")
		v2 := set.String("test2", "test flag", "test2")

		ctx = set.WithValues(ctx,
			feature.StringValue("test1", "test1 changed"),
			feature.StringValue("test2", "test2 changed"))

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

	t.Run("Func", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v := set.UintFunc("test", "test flag", func(ctx context.Context) uint {
			if hasTestFlag(ctx) {
				return 1
			}
			return 0
		})

		assertEquals(t, 0, v(ctx), "")
		assertEquals(t, 1, v(withTestFlag(ctx)), "")
	})

	t.Run("Override", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v1 := set.Uint("test1", "test flag", 5)
		v2 := set.Uint("test2", "test flag", 10)

		ctx = set.WithValues(ctx,
			feature.UintValue("test1", 15),
			feature.UintValue("test2", 20))

		assertEquals(t, 15, v1(ctx), "")
		assertEquals(t, 20, v2(ctx), "")
	})
}

type testStruct struct{ value int }

func TestTyped(t *testing.T) {
	t.Run("Duplicate", func(t *testing.T) {
		var set feature.FlagSet
		set.Float64("test", "test flag", 0.0)

		assertPanic(t, feature.ErrDuplicateFlag, func() {
			feature.Typed(&set, "test", "test flag", testStruct{})
		})
	})

	t.Run("Default", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v := feature.Typed(&set, "test", "test flag", testStruct{value: 5})

		assertEquals(t, testStruct{value: 5}, v(ctx), "")
	})

	t.Run("Func", func(t *testing.T) {
		ctx := t.Context()

		var set feature.FlagSet
		v := feature.TypedFunc(&set, "test", "test flag", func(ctx context.Context) testStruct {
			if hasTestFlag(ctx) {
				return testStruct{value: 5}
			}
			return testStruct{}
		})

		assertEquals(t, testStruct{}, v(ctx), "")
		assertEquals(t, testStruct{value: 5}, v(withTestFlag(ctx)), "")
	})
}

func BenchmarkFlagSet_Any(b *testing.B) {
	b.Run("Context", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.Any("test", "test flag", false)
		ctx := set.WithValues(b.Context(), feature.AnyValue("test", true))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})

	b.Run("Default", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.Any("test", "test flag", false)
		ctx := set.WithValues(b.Context(), feature.AnyValue("unused", false))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})
}

func BenchmarkFlagSet_Bool(b *testing.B) {
	b.Run("Context", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.Bool("test", "test flag", false)
		ctx := set.WithValue(b.Context(), feature.BoolValue("test", true))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})

	b.Run("Default", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.Bool("test", "test flag", false)
		ctx := set.WithValues(b.Context(), feature.BoolValue("unused", false))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})
}

func BenchmarkFlagSet_Float64(b *testing.B) {
	b.Run("Context", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.Float64("test", "test flag", 5.0)
		ctx := set.WithValue(b.Context(), feature.Float64Value("test", 5.0))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})

	b.Run("Default", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.Float64("test", "test flag", 5.0)
		ctx := set.WithValue(b.Context(), feature.Float64Value("unused", 0.0))

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
		ctx := set.WithValue(b.Context(), feature.IntValue("test", 5))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})

	b.Run("Default", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.Int("test", "test flag", 5.0)
		ctx := set.WithValue(b.Context(), feature.IntValue("unused", 0))

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
		ctx := set.WithValue(b.Context(), feature.StringValue("test", "test"))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})

	b.Run("Default", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.String("test", "test flag", "test")
		ctx := set.WithValue(b.Context(), feature.StringValue("unused", ""))

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
		ctx := set.WithValue(b.Context(), feature.UintValue("test", 5))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})

	b.Run("Default", func(b *testing.B) {
		var set feature.FlagSet
		flag := set.Uint("test", "test flag", 5)
		ctx := set.WithValue(b.Context(), feature.UintValue("unused", 0))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})
}

func BenchmarkTyped(b *testing.B) {
	b.Run("Context", func(b *testing.B) {
		var set feature.FlagSet
		flag := feature.Typed(&set, "test", "test flag", testStruct{value: 5})
		ctx := set.WithValue(b.Context(), feature.AnyValue("test", testStruct{value: 5}))

		b.ReportAllocs()

		for b.Loop() {
			flag(ctx)
		}
	})

	b.Run("Default", func(b *testing.B) {
		var set feature.FlagSet
		flag := feature.Typed(&set, "test", "test flag", testStruct{value: 5})
		ctx := set.WithValue(b.Context(), feature.AnyValue("unused", 0))

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

	if diff := cmp.Diff(want, got, flagComparer, cmpopts.EquateComparable(testStruct{})); diff != "" {
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

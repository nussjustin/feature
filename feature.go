package feature

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
	"sync/atomic"
	"time"
)

type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

// ErrDuplicateFlag is thrown by methods like [FlagSet.Bool] if a flag with a given name is already registered.
var ErrDuplicateFlag = errors.New("duplicate flag")

// Flag represents a flag registered with a [FlagSet].
type Flag struct {
	// Kind contains the flags kind or type.
	Kind FlagKind

	// Name is the name of the feature flag.
	Name string

	// Description is an optional description specified using [WithDescription].
	Description string
}

// FlagKind is an enum of potential flag kinds.
type FlagKind uint8

const (
	// FlagKindInvalid is the zero value of FlagKind and is not considered valid value.
	FlagKindInvalid FlagKind = iota

	// FlagKindAny is used for flags created via [FlagSet.Any] and [FlagSet.AnyFunc].
	FlagKindAny

	// FlagKindBool is used for flags created via [FlagSet.Bool] and [FlagSet.BoolFunc].
	FlagKindBool

	// FlagKindDuration is used for flags created via [FlagSet.Duration] and [FlagSet.DurationFunc].
	FlagKindDuration

	// FlagKindInt is used for flags created via [FlagSet.Int] and [FlagSet.IntFunc].
	FlagKindInt

	// FlagKindFloat64 is used for flags created via [FlagSet.Float64] and [FlagSet.Float64Func].
	FlagKindFloat64

	// FlagKindString is used for flags created via [FlagSet.String] and [FlagSet.StringFunc].
	FlagKindString

	// FlagKindUint is used for flags created via [FlagSet.Uint] and [FlagSet.UintFunc].
	FlagKindUint
)

// FlagSet represents a set of defined feature flags.
//
// A FlagSet must not be copied and should instead be passed around via pointer.
type FlagSet struct {
	noCopy noCopy

	flagsMu sync.Mutex   // only used when writing to flags
	flags   atomic.Value // of sortedMap[Flag]
}

// Func specifies the signature for functions that return feature flag values.
type Func[T any] func(ctx context.Context, name string) T

// Value specifies a custom value for a feature flag, which can be assigned to a [context.Context].
//
// A Value must be created using one of [BoolValue], [DurationValue], [Float64Value], [IntValue], [StringValue] or
// [UintValue].
type Value struct {
	name string

	kind     FlagKind
	any      any
	bool     bool
	duration time.Duration
	int      int
	float64  float64
	string   string
	uint     uint
}

type valuesMap map[string]Value

type valuesMapKey FlagSet

// All yields all registered flags sorted by name.
func (s *FlagSet) All(yield func(Flag) bool) {
	flags, _ := s.flags.Load().(sortedMap[Flag])

	for _, key := range flags.keys {
		if !yield(flags.m[key]) {
			return
		}
	}
}

// Lookup returns the flag with the given name.
func (s *FlagSet) Lookup(name string) (Flag, bool) {
	flags, _ := s.flags.Load().(sortedMap[Flag])

	f, ok := flags.m[name]
	return f, ok
}

// Context returns a new context based on ctx which will use the given values when checking feature flags of this set.
//
// If a values type does not match the flags type, Context will panic.
//
// Values with no matching flag are ignored.
func (s *FlagSet) Context(ctx context.Context, values ...Value) context.Context {
	if len(values) == 0 {
		return ctx
	}

	flags, _ := s.flags.Load().(sortedMap[Flag])

	m, ok := ctx.Value((*valuesMapKey)(s)).(valuesMap)
	if !ok {
		m = make(valuesMap, len(values))
	} else {
		m = maps.Clone(m)
	}

	for _, v := range values {
		f, ok := flags.m[v.name]
		if !ok {
			continue
		}

		if f.Kind != v.kind {
			panic(fmt.Errorf("invalid value kind for flag %q", v.name))
		}

		m[v.name] = v
	}

	return context.WithValue(ctx, (*valuesMapKey)(s), m)
}

func (s *FlagSet) value(ctx context.Context, name string, kind FlagKind) (Value, bool) {
	m, ok := ctx.Value((*valuesMapKey)(s)).(valuesMap)
	if !ok {
		return Value{}, false
	}
	v, ok := m[name]
	if !ok || v.kind != kind {
		return Value{}, false
	}
	return v, true
}

func (s *FlagSet) add(kind FlagKind, name string, desc string) {
	f := Flag{Kind: kind, Name: name, Description: desc}

	s.flagsMu.Lock()
	defer s.flagsMu.Unlock()

	flags, _ := s.flags.Load().(sortedMap[Flag])

	if _, ok := flags.m[f.Name]; ok {
		panic(fmt.Errorf("%w: %s", ErrDuplicateFlag, f.Name))
	}

	s.flags.Store(flags.add(f.Name, f))
}

// AnyValue returns a Value that can be passed to [FlagSet.Context] to override the value for the given flag.
func AnyValue(name string, value any) Value {
	return Value{name: name, kind: FlagKindAny, any: value}
}

// Any registers a new flag that represents an arbitrary value.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) Any(name string, desc string, value any) func(context.Context) any {
	return s.AnyFunc(name, desc, func(context.Context, string) any { return value })
}

// AnyFunc registers a new flag that represents an arbitrary value produced by calling the given function.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) AnyFunc(name string, desc string, valueFn Func[any]) func(context.Context) any {
	f := func(ctx context.Context) any {
		v, ok := s.value(ctx, name, FlagKindAny)
		if ok {
			return v.any
		}
		return valueFn(ctx, name)
	}

	s.add(FlagKindAny, name, desc)

	return f
}

// BoolValue returns a Value that can be passed to [FlagSet.Context] to override the value for the given flag.
func BoolValue(name string, value bool) Value {
	return Value{name: name, kind: FlagKindBool, bool: value}
}

// Bool registers a new flag that represents a boolean value.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) Bool(name string, desc string, value bool) func(context.Context) bool {
	return s.BoolFunc(name, desc, func(context.Context, string) bool { return value })
}

// BoolFunc registers a new flag that represents a boolean value produced by calling the given function.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) BoolFunc(name string, desc string, valueFn Func[bool]) func(context.Context) bool {
	f := func(ctx context.Context) bool {
		v, ok := s.value(ctx, name, FlagKindBool)
		if ok {
			return v.bool
		}
		return valueFn(ctx, name)
	}

	s.add(FlagKindBool, name, desc)

	return f
}

// DurationValue returns a Value that can be passed to [FlagSet.Context] to override the value for the given flag.
func DurationValue(name string, value time.Duration) Value {
	return Value{name: name, kind: FlagKindDuration, duration: value}
}

// Duration registers a new flag that represents a duration value.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) Duration(name string, desc string, value time.Duration) func(context.Context) time.Duration {
	return s.DurationFunc(name, desc, func(context.Context, string) time.Duration { return value })
}

// DurationFunc registers a new flag that represents a duration value produced by calling the given function.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) DurationFunc(name string, desc string, valueFn Func[time.Duration]) func(context.Context) time.Duration {
	f := func(ctx context.Context) time.Duration {
		v, ok := s.value(ctx, name, FlagKindDuration)
		if ok {
			return v.duration
		}
		return valueFn(ctx, name)
	}

	s.add(FlagKindDuration, name, desc)

	return f
}

// Float64Value returns a Value that can be passed to [FlagSet.Context] to override the value for the given flag.
func Float64Value(name string, value float64) Value {
	return Value{name: name, kind: FlagKindFloat64, float64: value}
}

// Float64 registers a new flag that represents a floating point value.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) Float64(name string, desc string, value float64) func(context.Context) float64 {
	return s.Float64Func(name, desc, func(context.Context, string) float64 { return value })
}

// Float64Func registers a new flag that represents a floating point value produced by calling the given function.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) Float64Func(name string, desc string, valueFn Func[float64]) func(context.Context) float64 {
	f := func(ctx context.Context) float64 {
		v, ok := s.value(ctx, name, FlagKindFloat64)
		if ok {
			return v.float64
		}
		return valueFn(ctx, name)
	}

	s.add(FlagKindFloat64, name, desc)

	return f
}

// IntValue returns a Value that can be passed to [FlagSet.Context] to override the value for the given flag.
func IntValue(name string, value int) Value {
	return Value{name: name, kind: FlagKindInt, int: value}
}

// Int registers a new flag that represents an integer value.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) Int(name string, desc string, value int) func(context.Context) int {
	return s.IntFunc(name, desc, func(context.Context, string) int { return value })
}

// IntFunc registers a new flag that represents an integer value produced by calling the given function.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) IntFunc(name string, desc string, valueFn Func[int]) func(context.Context) int {
	f := func(ctx context.Context) int {
		v, ok := s.value(ctx, name, FlagKindInt)
		if ok {
			return v.int
		}
		return valueFn(ctx, name)
	}

	s.add(FlagKindInt, name, desc)

	return f
}

// StringValue returns a Value that can be passed to [FlagSet.Context] to override the value for the given flag.
func StringValue(name string, value string) Value {
	return Value{name: name, kind: FlagKindString, string: value}
}

// String registers a new flag that represents a string value.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) String(name string, desc string, value string) func(context.Context) string {
	return s.StringFunc(name, desc, func(context.Context, string) string { return value })
}

// StringFunc registers a new flag that represents a string value produced by calling the given function.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) StringFunc(name string, desc string, valueFn Func[string]) func(context.Context) string {
	f := func(ctx context.Context) string {
		v, ok := s.value(ctx, name, FlagKindString)
		if ok {
			return v.string
		}
		return valueFn(ctx, name)
	}

	s.add(FlagKindString, name, desc)

	return f
}

// UintValue returns a Value that can be passed to [FlagSet.Context] to override the value for the given flag.
func UintValue(name string, value uint) Value {
	return Value{name: name, kind: FlagKindUint, uint: value}
}

// Uint registers a new flag that represents an unsigned integer value.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) Uint(name string, desc string, value uint) func(context.Context) uint {
	return s.UintFunc(name, desc, func(context.Context, string) uint { return value })
}

// UintFunc registers a new flag that represents an unsigned integer value produced by calling the given function.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) UintFunc(name string, desc string, valueFn Func[uint]) func(context.Context) uint {
	f := func(ctx context.Context) uint {
		v, ok := s.value(ctx, name, FlagKindUint)
		if ok {
			return v.uint
		}
		return valueFn(ctx, name)
	}

	s.add(FlagKindUint, name, desc)

	return f
}

// Typed registers a new flag that represents a value of type T.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func Typed[T any](s *FlagSet, name string, desc string, value T) func(context.Context) T {
	return TypedFunc(s, name, desc, func(context.Context, string) T { return value })
}

// TypedFunc registers a new flag that represents a value of type T value produced by calling the given function.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func TypedFunc[T any](s *FlagSet, name string, desc string, value Func[T]) func(context.Context) T {
	f := s.AnyFunc(name, desc, func(ctx context.Context, name string) any {
		return value(ctx, name)
	})

	return func(ctx context.Context) T {
		return f(ctx).(T)
	}
}

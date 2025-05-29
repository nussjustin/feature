package feature

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
	"sync/atomic"
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

	// Value is the default value for the flag as specified on creation.
	Value any

	// Description is an optional description specified using [WithDescription].
	Description string
}

// FlagKind is an enum of potential flag kinds.
type FlagKind uint8

const (
	// FlagKindInvalid is the zero value of FlagKind and is not considered valid value.
	FlagKindInvalid FlagKind = iota

	// FlagKindBool denotes a boolean flag created using [FlagSet.Bool].
	FlagKindBool

	// FlagKindInt denotes a boolean flag created using [FlagSet.Int].
	FlagKindInt

	// FlagKindFloat64 denotes a boolean flag created using [FlagSet.Float64].
	FlagKindFloat64

	// FlagKindString denotes a boolean flag created using [FlagSet.String].
	FlagKindString

	// FlagKindUint denotes a boolean flag created using [FlagSet.Uint].
	FlagKindUint
)

// FlagSet represents a set of defined feature flags.
//
// The zero value is valid and returns zero values for all flags.
//
// A FlagSet must not be copied and should instead be passed around via pointer.
type FlagSet struct {
	noCopy noCopy

	flagsMu sync.Mutex   // only used when writing to flags
	flags   atomic.Value // of sortedMap[Flag]
}

// Value specifies a custom value for a feature flag, which can be assigned to a [context.Context].
//
// A Value must be created using one of [BoolValue], [Float64Value], [IntValue], [StringValue] or [UintValue].
type Value struct {
	name string

	kind   FlagKind
	bool   bool
	int    int
	float  float64
	string string
	uint   uint
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

func (s *FlagSet) add(kind FlagKind, name string, value any, desc string) {
	f := Flag{Kind: kind, Name: name, Value: value, Description: desc}

	s.flagsMu.Lock()
	defer s.flagsMu.Unlock()

	flags, _ := s.flags.Load().(sortedMap[Flag])

	if _, ok := flags.m[f.Name]; ok {
		panic(fmt.Errorf("%w: %s", ErrDuplicateFlag, f.Name))
	}

	s.flags.Store(flags.add(f.Name, f))
}

// BoolValue returns a Value that can be passed to [FlagSet.Context] to override the value for the given flag.
func BoolValue(name string, value bool) Value {
	return Value{name: name, kind: FlagKindBool, bool: value}
}

// Bool registers a new flag that represents a boolean value.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) Bool(name string, value bool, desc string) func(context.Context) bool {
	f := func(ctx context.Context) bool {
		v, ok := s.value(ctx, name, FlagKindBool)
		if ok {
			return v.bool
		}
		return value
	}

	s.add(FlagKindBool, name, value, desc)

	return f
}

// Float64Value returns a Value that can be passed to [FlagSet.Context] to override the value for the given flag.
func Float64Value(name string, value float64) Value {
	return Value{name: name, kind: FlagKindFloat64, float: value}
}

// Float64 registers a new flag that represents a float value.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) Float64(name string, value float64, desc string) func(context.Context) float64 {
	f := func(ctx context.Context) float64 {
		v, ok := s.value(ctx, name, FlagKindFloat64)
		if ok {
			return v.float
		}
		return value
	}

	s.add(FlagKindFloat64, name, value, desc)

	return f
}

// IntValue returns a Value that can be passed to [FlagSet.Context] to override the value for the given flag.
func IntValue(name string, value int) Value {
	return Value{name: name, kind: FlagKindInt, int: value}
}

// Int registers a new flag that represents an int value.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) Int(name string, value int, desc string) func(context.Context) int {
	f := func(ctx context.Context) int {
		v, ok := s.value(ctx, name, FlagKindInt)
		if ok {
			return v.int
		}
		return value
	}

	s.add(FlagKindInt, name, value, desc)

	return f
}

// StringValue returns a Value that can be passed to [FlagSet.Context] to override the value for the given flag.
func StringValue(name string, value string) Value {
	return Value{name: name, kind: FlagKindString, string: value}
}

// String registers a new flag that represents a string value.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) String(name string, value string, desc string) func(context.Context) string {
	f := func(ctx context.Context) string {
		v, ok := s.value(ctx, name, FlagKindString)
		if ok {
			return v.string
		}
		return value
	}

	s.add(FlagKindString, name, value, desc)

	return f
}

// UintValue returns a Value that can be passed to [FlagSet.Context] to override the value for the given flag.
func UintValue(name string, value uint) Value {
	return Value{name: name, kind: FlagKindUint, uint: value}
}

// Uint registers a new flag that represents an uint value.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) Uint(name string, value uint, desc string) func(context.Context) uint {
	f := func(ctx context.Context) uint {
		v, ok := s.value(ctx, name, FlagKindUint)
		if ok {
			return v.uint
		}
		return value
	}

	s.add(FlagKindUint, name, value, desc)

	return f
}

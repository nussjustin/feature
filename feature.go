package feature

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
)

// ErrDuplicateFlag is returned by if a flag with a given name is already registered.
var ErrDuplicateFlag = errors.New("duplicate flag")

// Flag represents a flag registered with a [FlagSet].
type Flag struct {
	// Name is the name of the feature flag.
	Name string

	// Value is the default value for the flag as specified on creation.
	Value any

	// Description is an optional description specified using [WithDescription].
	Description string

	// Labels contains the labels specified via [WithLabels].
	Labels Labels

	// Func is the function returned when the flag was registered.
	Func any
}

// FlagSet represents a set of defined feature flags.
//
// The zero value is valid and returns zero values for all flags.
type FlagSet struct {
	flagsMu sync.Mutex
	flags   sortedMap[Flag]
}

// Labels is a read only map collection of labels associated with a feature flag.
type Labels struct {
	m sortedMap[string]
}

// All yields all labels.
func (l *Labels) All(yield func(string, string) bool) {
	for _, key := range l.m.keys {
		if !yield(key, l.m.m[key]) {
			return
		}
	}
}

// Len returns the number of labels.
func (l *Labels) Len() int {
	return len(l.m.keys)
}

// Value specifies a custom value for a feature flag, which can be assigned to a [context.Context].
//
// A Value must be created using one of [BoolValue], [FloatValue], [IntValue], [StringValue] or [UintValue].
type Value struct {
	name string

	kind   valueKind
	bool   bool
	int    int
	float  float64
	string string
	uint   uint
}

type valuesMap map[string]Value

type valuesMapKey FlagSet

type valueKind uint8

const (
	valueKindBool valueKind = iota
	valueKindInt
	valueKindFloat
	valueKindString
	valueKindUint
)

// All yields all registered flags sorted by name.
func (s *FlagSet) All(yield func(Flag) bool) {
	s.flagsMu.Lock()
	flags := s.flags
	s.flagsMu.Unlock()

	for _, key := range flags.keys {
		if !yield(flags.m[key]) {
			return
		}
	}
}

// Lookup returns the flag with the given name.
func (s *FlagSet) Lookup(name string) (Flag, bool) {
	s.flagsMu.Lock()
	defer s.flagsMu.Unlock()

	f, ok := s.flags.m[name]
	return f, ok
}

// Context returns a new context based on ctx which will use the given values when checking feature flags.
//
// If a values type does not match the flags type, Context will panic.
//
// Values with no matching flag are ignored.
func (s *FlagSet) Context(ctx context.Context, values ...Value) context.Context {
	if len(values) == 0 {
		return ctx
	}

	s.flagsMu.Lock()
	flags := s.flags
	s.flagsMu.Unlock()

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

		switch v.kind {
		case valueKindBool:
			_, ok = f.Value.(bool)
		case valueKindInt:
			_, ok = f.Value.(int)
		case valueKindFloat:
			_, ok = f.Value.(float64)
		case valueKindString:
			_, ok = f.Value.(string)
		case valueKindUint:
			_, ok = f.Value.(uint)
		default:
			panic("unreachable")
		}

		if !ok {
			panic(fmt.Errorf("invalid value for flag %q", v.name))
		}

		m[v.name] = v
	}

	return context.WithValue(ctx, (*valuesMapKey)(s), m)
}

func (s *FlagSet) value(ctx context.Context, name string, kind valueKind) (Value, bool) {
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

func (s *FlagSet) add(name string, value any, fun any, opts ...Option) {
	f := Flag{Name: name, Value: value, Func: fun}
	for _, opt := range opts {
		opt(&f)
	}

	s.flagsMu.Lock()
	defer s.flagsMu.Unlock()

	if _, ok := s.flags.m[f.Name]; ok {
		panic(fmt.Errorf("%w: %s", ErrDuplicateFlag, f.Name))
	}

	s.flags = s.flags.add(f.Name, f)
}

// BoolValue returns a Value that can be passed to [FlagSet.Context] to override the value for the given flag.
func BoolValue(name string, value bool) Value {
	return Value{name: name, kind: valueKindBool, bool: value}
}

// Bool registers a new flag that represents a boolean value.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) Bool(name string, value bool, opts ...Option) func(context.Context) bool {
	f := func(ctx context.Context) bool {
		v, ok := s.value(ctx, name, valueKindBool)
		if ok {
			return v.bool
		}
		return value
	}

	s.add(name, value, f, opts...)

	return f
}

// FloatValue returns a Value that can be passed to [FlagSet.Context] to override the value for the given flag.
func FloatValue(name string, value float64) Value {
	return Value{name: name, kind: valueKindFloat, float: value}
}

// Float registers a new flag that represents a float value.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) Float(name string, value float64, opts ...Option) func(context.Context) float64 {
	f := func(ctx context.Context) float64 {
		v, ok := s.value(ctx, name, valueKindFloat)
		if ok {
			return v.float
		}
		return value
	}

	s.add(name, value, f, opts...)

	return f
}

// IntValue returns a Value that can be passed to [FlagSet.Context] to override the value for the given flag.
func IntValue(name string, value int) Value {
	return Value{name: name, kind: valueKindInt, int: value}
}

// Int registers a new flag that represents an int value.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) Int(name string, value int, opts ...Option) func(context.Context) int {
	f := func(ctx context.Context) int {
		v, ok := s.value(ctx, name, valueKindInt)
		if ok {
			return v.int
		}
		return value
	}

	s.add(name, value, f, opts...)

	return f
}

// StringValue returns a Value that can be passed to [FlagSet.Context] to override the value for the given flag.
func StringValue(name string, value string) Value {
	return Value{name: name, kind: valueKindString, string: value}
}

// String registers a new flag that represents a string value.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) String(name string, value string, opts ...Option) func(context.Context) string {
	f := func(ctx context.Context) string {
		v, ok := s.value(ctx, name, valueKindString)
		if ok {
			return v.string
		}
		return value
	}

	s.add(name, value, f, opts...)

	return f
}

// UintValue returns a Value that can be passed to [FlagSet.Context] to override the value for the given flag.
func UintValue(name string, value uint) Value {
	return Value{name: name, kind: valueKindUint, uint: value}
}

// Uint registers a new flag that represents an uint value.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) Uint(name string, value uint, opts ...Option) func(context.Context) uint {
	f := func(ctx context.Context) uint {
		v, ok := s.value(ctx, name, valueKindUint)
		if ok {
			return v.uint
		}
		return value
	}

	s.add(name, value, f, opts...)

	return f
}

// Option defines options for new flags which can be passed to [Register].
type Option func(*Flag)

// WithDescription sets the description for a flag.
//
// if given multiple times, only the last value is used.
func WithDescription(desc string) Option {
	return func(f *Flag) {
		f.Description = desc
	}
}

// WithLabel adds a label to a flag.
func WithLabel(key, value string) Option {
	return func(f *Flag) {
		f.Labels.m = f.Labels.m.add(key, value)
	}
}

// WithLabels adds labels to a flag.
//
// If used multiple times, the maps will be merged with later values replacing prior ones.
func WithLabels(labels map[string]string) Option {
	return func(f *Flag) {
		f.Labels.m = f.Labels.m.addMany(labels)
	}
}

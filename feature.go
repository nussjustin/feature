package feature

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

// ErrDuplicateFlag is returned by if a flag with a given name is already registered.
var ErrDuplicateFlag = errors.New("duplicate flag")

// Flag represents a flag registered with a [FlagSet].
type Flag struct {
	// Name is the name of the feature as passed to [Register].
	Name string

	// Description is an optional description specified using [WithDescription].
	Description string

	// Labels contains the labels specified via [WithLabels].
	Labels Labels

	// Func is callback that returns the value for the flag and is either a [BoolFunc], [IntFunc] or [StringFunc].
	Func any
}

// FlagSet represents a set of defined feature flags.
//
// The zero value is valid and returns zero values for all flags.
type FlagSet struct {
	registry atomic.Pointer[Registry]

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

// SetRegistry sets the Registry to be used for looking up flag values.
//
// A nil value will cause all flags to return zero values.
func (s *FlagSet) SetRegistry(r Registry) {
	if r == nil {
		s.registry.Store(nil)
	} else {
		s.registry.Store(&r)
	}
}

func (s *FlagSet) add(name string, fun any, opts ...Option) {
	f := Flag{Name: name, Func: fun}
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

// Bool registers a new flag that represents a boolean value.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) Bool(name string, opts ...Option) func(context.Context) bool {
	f := func(ctx context.Context) bool {
		r := s.registry.Load()
		if r == nil {
			return false
		}
		return (*r).Bool(ctx, name)
	}

	s.add(name, f, opts...)

	return f
}

// Float registers a new flag that represents a float value.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) Float(name string, opts ...Option) func(context.Context) float64 {
	f := func(ctx context.Context) float64 {
		r := s.registry.Load()
		if r == nil {
			return 0.0
		}
		return (*r).Float(ctx, name)
	}

	s.add(name, f, opts...)

	return f
}

// Int registers a new flag that represents an int value.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) Int(name string, opts ...Option) func(context.Context) int {
	f := func(ctx context.Context) int {
		r := s.registry.Load()
		if r == nil {
			return 0
		}
		return (*r).Int(ctx, name)
	}

	s.add(name, f, opts...)

	return f
}

// String registers a new flag that represents a string value.
//
// If a [Flag] with the same name is already registered, the call will panic with an error that is [ErrDuplicateFlag].
func (s *FlagSet) String(name string, opts ...Option) func(context.Context) string {
	f := func(ctx context.Context) string {
		r := s.registry.Load()
		if r == nil {
			return ""
		}
		return (*r).String(ctx, name)
	}

	s.add(name, f, opts...)

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

// Registry defines method for getting the feature flag values by name.
//
// Calling a method when the corresponding struct field is not set will cause the call to panic.
type Registry interface {
	// Bool returns the boolean value for the flag with the given name.
	Bool(ctx context.Context, name string) bool

	// Float returns the float value for the flag with the given name.
	Float(ctx context.Context, name string) float64

	// Int returns the integer value for the flag with the given name.
	Int(ctx context.Context, name string) int

	// String returns the string value for the flag with the given name.
	String(ctx context.Context, name string) string
}

// SimpleRegistry implements a [Registry] using callbacks set as struct fields.
type SimpleRegistry struct {
	// BoolFunc contains the implementation for the Registry.Bool function.
	BoolFunc func(ctx context.Context, name string) bool

	// FloatFunc contains the implementation for the Registry.Float function.
	FloatFunc func(ctx context.Context, name string) float64

	// IntFunc contains the implementation for the Registry.Int function.
	IntFunc func(ctx context.Context, name string) int

	// StringFunc contains the implementation for the Registry.String function.
	StringFunc func(ctx context.Context, name string) string
}

// Bool implements the [Registry] interface by calling s.BoolFunc and returning the result.
func (s *SimpleRegistry) Bool(ctx context.Context, name string) bool {
	return s.BoolFunc(ctx, name)
}

// Float implements the [Registry] interface by calling s.FloatFunc and returning the result.
func (s *SimpleRegistry) Float(ctx context.Context, name string) float64 {
	return s.FloatFunc(ctx, name)
}

// Int implements the [Registry] interface by calling s.IntFunc and returning the result.
func (s *SimpleRegistry) Int(ctx context.Context, name string) int {
	return s.IntFunc(ctx, name)
}

// String implements the [Registry] interface by calling s.StringFunc and returning the result.
func (s *SimpleRegistry) String(ctx context.Context, name string) string {
	return s.StringFunc(ctx, name)
}

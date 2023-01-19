package feature

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// Decision is an enum of the potential decisions a [Strategy] can make on whether a [Flag] should be enabled or not.
//
// Decision also implements the [Strategy] interface which can be useful when writing custom [Strategy] implementations
// or tests.
//
// See the comment on [Decision.Enabled] for more information.
type Decision string

const (
	// Default hands over the decision to the Flag.
	Default Decision = "default"
	// Disabled disables a [Flag] and the new code path of the corresponding case.
	Disabled Decision = "disabled"
	// Enabled enables a [Flag] and the new code path of the corresponding case.
	Enabled Decision = "enabled"
)

var _ Strategy = Default

// If returns ifTrue if cond is true, otherwise ifFalse is returned.
func If(cond bool, ifTrue, ifFalse Decision) Decision {
	if cond {
		return ifTrue
	}
	return ifFalse
}

// Enabled implements the Strategy.
//
// This can be useful when writing custom [Strategy] implementations or in tests.
func (d Decision) Enabled(context.Context, string) Decision {
	return d
}

// DefaultDecision is set when creating a [Flag] or [Case] and is used when [Default] is returned by the [Strategy].
type DefaultDecision string

const (
	// DefaultDisabled causes a [Flag] or [Case] to treat a [Decision] of [Default] as [Disabled].
	DefaultDisabled = DefaultDecision(Disabled)
	// DefaultEnabled causes a [Flag] or [Case] to treat a [Decision] of [Default] as [Enabled].
	DefaultEnabled = DefaultDecision(Enabled)
)

// Set manages feature flags and provides the [Strategy] (using [SetStrategy]) for making dynamic decisions about
// a flags' status.
//
// The zero value is usable as is, using the default decision for each flag.
type Set struct {
	strategy atomic.Pointer[Strategy]

	mu    sync.Mutex
	flags map[string]*Flag
}

var globalSet Set

// SetStrategy sets or removes the [Strategy] for the global [Set].
//
// See [Set.SetStrategy] for more information.
func SetStrategy(strategy Strategy) {
	globalSet.SetStrategy(strategy)
}

// SetStrategy sets or removes the [Strategy] used by s to make decisions.
func (s *Set) SetStrategy(strategy Strategy) {
	if strategy == nil {
		s.strategy.Store(nil)
	} else {
		s.strategy.Store(&strategy)
	}
}

func (s *Set) newFlag(name, description string, defaultDecision DefaultDecision) *Flag {
	f := &Flag{set: s, name: name, description: description, defaultDecision: defaultDecision}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.flags == nil {
		s.flags = map[string]*Flag{}
	}

	if _, ok := s.flags[name]; ok {
		panic(fmt.Sprintf("name %q already in use", name))
	}

	s.flags[name] = f

	return f
}

// Case can be used to simplify running code paths dynamically based on the whether a feature is enabled.
//
// Additionally, _experiments_ can be run using the [Case.Experiment] method, which can compare the results of two
// functions, most commonly a new and a old implementation of some logic, while assuring that problems in one
// function (the _new_ implementation) do not cause problems by falling back to the result of the other function
// in case of an error or some other unexpected differences.
//
// A Case must be obtained using either [CaseFor], [NewCase] or [RegisterCase]. The zero value is invalid.
type Case[T any] struct {
	flag *Flag
}

// CaseFor returns a new [Case] for the given registered [Flag].
func CaseFor[T any](f *Flag) *Case[T] {
	return &Case[T]{flag: f}
}

// NewCase registers and returns a new [Case] with the global [Set].
//
// If the given name is already is use by another case or flag, NewCase will panic.
func NewCase[T any](name string, description string, defaultDecision DefaultDecision) *Case[T] {
	return RegisterCase[T](&globalSet, name, description, defaultDecision)
}

// RegisterCase registers and returns a new [Flag] with the given [Set].
//
// If the given name is already is use by another case or flag, RegisterCase will panic.
func RegisterCase[T any](set *Set, name string, description string, defaultDecision DefaultDecision) *Case[T] {
	return CaseFor[T](RegisterFlag(set, name, description, defaultDecision))
}

// Equals returns a function that compares to values of the same type using ==.
//
// This can be used with [Case.Experiment] when T is a comparable type.
func Equals[T comparable](a, b T) bool {
	return a == b
}

// PanicError is used to wrap the values of recovered panics in [Case.Experiment].
type PanicError struct {
	name string

	// Value is the value returned by [recover].
	Value any
}

var _ error = (*PanicError)(nil)

// Error implements the error interface.
func (p *PanicError) Error() string {
	return fmt.Sprintf("%s: caught panic(%v)", p.name, p.Value)
}

func catchPanic[T any](name string, f func() (T, error)) (t T, paniced bool, err error) {
	defer func() {
		if v := recover(); v != nil {
			paniced = true
			err = &PanicError{name: name, Value: v}
		}
	}()
	t, err = f()
	return
}

// Experiment runs new and old concurrently and compares their results using cmp.
//
// If the feature flag is enabled, the result from new will be returned, otherwise the result from old will be returned.
//
// When a function panics the panic is caught and converted into an error that is or wraps a [PanicError] and treated
// like a normal error.
//
// When using values of a type that is comparable using ==, the global function [Equals] can be used to create the
// comparison function.
//
// Example:
//
//	c.Experiment(ctx, newFunc, oldFunc, feature.Equals[User])
func (c *Case[T]) Experiment(ctx context.Context,
	new func(context.Context) (T, error),
	old func(context.Context) (T, error),
	equals func(new, old T) bool,
) (T, error) {
	// TODO: tracing
	// TODO: timing metrics
	var wg sync.WaitGroup
	var (
		newT   T
		newErr error

		oldT   T
		oldErr error
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		newT, _, newErr = catchPanic("new", func() (T, error) { return new(ctx) })
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		oldT, _, oldErr = catchPanic("old", func() (T, error) { return old(ctx) })
	}()

	// Check status before while the experiment runs. This can save some time if the used Strategy is slow.
	isEnabled := c.flag.Enabled(ctx)

	wg.Wait()

	// TODO: error metrics
	// TODO: panic metrics
	// TODO: difference metrics
	// TODO: status metrics

	_ = equals(newT, oldT)

	if isEnabled {
		return newT, newErr
	}

	return oldT, oldErr
}

// Run checks if the associated flag is enabled and runs either ifEnabled or ifDisabled and returns their result.
//
// Example:
//
//	user, err := c.Run(ctx, getUserV2, getUser)
func (c *Case[T]) Run(ctx context.Context,
	ifEnabled func(context.Context) (T, error),
	ifDisabled func(context.Context) (T, error),
) (T, error) {
	// TODO: tracing
	if c.flag.Enabled(ctx) {
		return ifEnabled(ctx)
	}
	return ifDisabled(ctx)
}

// Flag represents a feature flag that can be enabled or disabled based on some kind of logic.
//
// Example:
//
//	    if trackingFlag.Enabled(ctx) {
//	        trackUser(ctx, user)
//		}
//
// In many cases a [Case] can be used to simplify working with a Flag. See the documentation and examples for [Case]
// for more information on how to use a [Case].
//
// A Flag must be obtained using either [NewFlag] or [RegisterFlag]. The zero value is invalid.
type Flag struct {
	set *Set

	name            string
	description     string
	defaultDecision DefaultDecision
}

// NewFlag registers and returns a new [Flag] with the global [Set].
//
// If the given name is already is use by another case or flag, NewFlag will panic.
func NewFlag(name string, description string, defaultDecision DefaultDecision) *Flag {
	return RegisterFlag(&globalSet, name, description, defaultDecision)
}

// RegisterFlag registers and returns a new [Flag] with the given [Set].
//
// If the given name is already is use by another case or flag, RegisterFlag will panic.
func RegisterFlag(set *Set, name string, description string, defaultDecision DefaultDecision) *Flag {
	return set.newFlag(name, description, defaultDecision)
}

// Enabled returns true if the feature is enabled for the given context, using the [Strategy] of the associated [Set]
// (or the [DefaultDecision] given to [NewFlag]/[RegisterFlag] as fallback).
//
// Example:
//
//	    if trackingFlag.Enabled(ctx) {
//	        trackUser(ctx, user)
//		}
func (f *Flag) Enabled(ctx context.Context) bool {
	d := Default

	if h := f.set.strategy.Load(); h != nil {
		d = (*h).Enabled(ctx, f.name)
	}

	if d == Default {
		d = Decision(f.defaultDecision)
	}

	return d == Enabled
}

// Name returns the name passed to [NewFlag] or [RegisterFlag].
func (f *Flag) Name() string {
	return f.name
}

// Description returns the description passed to [NewFlag] or [RegisterFlag].
func (f *Flag) Description() string {
	return f.description
}

// Strategy defines an interface used for deciding on whether a feature is enabled or not.
type Strategy interface {
	// Enabled takes the name of a feature flag and returns a [Decision] on whether the feature should be enabled.
	Enabled(ctx context.Context, name string) Decision
}

// StrategyFunc implements a [Strategy] that uses the function as implementation of [Strategy.Enabled].
type StrategyFunc func(ctx context.Context, name string) Decision

var _ Strategy = (StrategyFunc)(nil)

// Enabled implements the [Strategy] interface.
func (f StrategyFunc) Enabled(ctx context.Context, name string) Decision {
	return f(ctx, name)
}

// StrategyMap implements a simple [Strategy] using a map of strategies by feature name.
type StrategyMap map[string]Strategy

var _ Strategy = (StrategyMap)(nil)

// Enabled implements the [Strategy] interface.
//
// If a feature with the given name is not found, [Default] is returned.
func (m StrategyMap) Enabled(ctx context.Context, name string) Decision {
	if s, ok := m[name]; ok {
		return s.Enabled(ctx, name)
	}
	return Default
}

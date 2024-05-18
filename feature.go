package feature

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"sync"
	"sync/atomic"
)

// DecisionMap implements a simple [Strategy] that returns a fixed value for each flag by its name.
//
// Checking a flag that is not in the map will panic.
type DecisionMap map[string]bool

var _ Strategy = (DecisionMap)(nil)

// Enabled implements the [Strategy] interface.
//
// If a feature with the given name is not found, Enabled will panic.
func (m DecisionMap) Enabled(_ context.Context, flag *Flag) bool {
	if d, ok := m[flag.Name()]; ok {
		return d
	}
	panic(fmt.Sprintf("strategy for feature %q not configured", flag.Name()))
}

// Set manages feature flags and provides a [Strategy] (using [SetStrategy]) for making dynamic decisions about
// a flags' status.
//
// A Set with no associated [Strategy] is invalid and checking a flag will panic.
type Set struct {
	strategy atomic.Pointer[Strategy]
	tracer   atomic.Pointer[Tracer]

	mu    sync.Mutex
	flags map[string]*Flag
}

var globalSet Set

// New registers and returns a new [Flag] with the global [Set].
//
// See [Set.New] for more details.
func New(name string, opts ...FlagOpt) *Flag {
	return globalSet.New(name, opts...)
}

// New registers and returns a new [Flag] on s.
//
// If the given name is empty or already registered, New will panic.
func (s *Set) New(name string, opts ...FlagOpt) *Flag {
	if name == "" {
		panic("missing name for flag")
	}

	return s.newFlag(name, opts)
}

// SetStrategy sets the [Strategy] for the global [Set].
func SetStrategy(strategy Strategy) {
	globalSet.SetStrategy(strategy)
}

// SetStrategy sets the [Strategy] used by s to make decisions.
func (s *Set) SetStrategy(strategy Strategy) {
	if s == nil {
		panic("strategy must not be nil")
	}

	s.strategy.Store(&strategy)
}

// SetTracer sets the [Tracer] used for the global [Set].
//
// See [Tracer] for more information.
func SetTracer(tracer Tracer) {
	globalSet.SetTracer(tracer)
}

// SetTracer sets the [Tracer] used by the [Set].
//
// See [Tracer] for more information.
func (s *Set) SetTracer(tracer Tracer) {
	s.tracer.Store(&tracer)
}

func (s *Set) getTracer() Tracer {
	if t := s.tracer.Load(); t != nil {
		return *t
	}
	return Tracer{}
}

// Flags returns a slice containing all flags registered with the global [Set].
//
// See [Set.Flags] for more information.
func Flags() []*Flag {
	return globalSet.Flags()
}

// Flags returns a slice containing all registered flags order by name.
func (s *Set) Flags() []*Flag {
	s.mu.Lock()

	fs := make([]*Flag, 0, len(s.flags))
	for _, f := range s.flags {
		fs = append(fs, f)
	}

	s.mu.Unlock()

	sort.Slice(fs, func(i, j int) bool {
		return fs[i].name < fs[j].name
	})

	return fs
}

func (s *Set) newFlag(name string, opts []FlagOpt) *Flag {
	f := &Flag{
		set:  s,
		name: name,
	}

	for _, opt := range opts {
		opt(f)
	}

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

// Tracer can be used to trace the use of calls to [Flag.Enabled] as well as the global helper functions [Experiment]
// and [Switch].
//
// See the documentation on each field for information on what can be traced.
//
// All fields are optional.
//
// A basic, pre-configured [Tracer] using OpenTelemetry can be found in the otelfeature subpackage.
type Tracer struct {
	// Decision is called every time [Flag.Enabled] is called.
	Decision func(ctx context.Context, f *Flag, enabled bool)

	// Experiment is called at the beginning of every call to [Experiment].
	//
	// The returned function is called after both functions given to [Experiment] have returned and is passed
	// the values that will be returned as well as a boolean that indicates if the experiment was successful (the
	// results were equal and no errors occurred).
	//
	// The returned function can be nil.
	Experiment func(ctx context.Context, f *Flag, enabled bool) (context.Context, func(result any, err error, success bool))

	// ExperimentBranch is called for each called function during [Experiment] as well as for the function called by [Switch].
	//
	// The returned function is called after the called function has returned with the values returned by the function.
	//
	// The returned function can be nil.
	ExperimentBranch func(ctx context.Context, f *Flag, enabled bool) (context.Context, func(result any, err error))

	// Switch is called at the beginning of every call to [Switch].
	//
	// The returned function is called with the result that will be returned.
	//
	// The returned function can be nil.
	Switch func(ctx context.Context, f *Flag, enabled bool) (context.Context, func(result any, err error))
}

// Equals returns a function that compares to values of the same type using ==.
//
// This can be used with [Experiment] when T is a comparable type.
func Equals[T comparable](a, b T) bool {
	return a == b
}

// Experiment runs both an experimental and a control function concurrently and compares their results using equals.
//
// If the feature flag is enabled, the result of the experimental function will be returned, otherwise the result of the
// control function will be returned.
//
// The given equals function is only called if there was no error.
//
// When using values of a type that is comparable using ==, the global function [Equals] can be used to create the
// comparison function.
func Experiment[T any](ctx context.Context, flag *Flag,
	experimental func(context.Context) (T, error),
	control func(context.Context) (T, error),
	equals func(new, old T) bool,
) (T, error) {
	enabled := flag.Enabled(ctx)

	var done func(result any, err error, success bool)
	if t := flag.set.getTracer(); t.Experiment != nil {
		ctx, done = t.Experiment(ctx, flag, enabled)
	}

	var wg sync.WaitGroup
	var (
		experimentT   T
		experimentErr error

		controlT   T
		controlErr error
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		experimentT, experimentErr = run(ctx, flag, true, experimental)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		controlT, controlErr = run(ctx, flag, false, control)
	}()

	wg.Wait()

	var result T
	var err error

	if enabled {
		result, err = experimentT, experimentErr
	} else {
		result, err = controlT, controlErr
	}

	// Always compare, even if we don't use the result (done is nil).
	ok := controlErr == nil && experimentErr == nil && equals(experimentT, controlT)

	if done != nil {
		done(result, err, ok)
	}

	return result, err
}

// Switch checks if the associated flag is enabled and runs either ifEnabled or ifDisabled and returns their result.
func Switch[T any](ctx context.Context, flag *Flag,
	ifEnabled func(context.Context) (T, error),
	ifDisabled func(context.Context) (T, error),
) (T, error) {
	enabled := flag.Enabled(ctx)

	var done func(any, error)
	if t := flag.set.getTracer(); t.Switch != nil {
		ctx, done = t.Switch(ctx, flag, enabled)
	}

	fn := ifDisabled
	if enabled {
		fn = ifEnabled
	}

	resultT, err := fn(ctx)

	if done != nil {
		done(resultT, err)
	}

	return resultT, err
}

func run[T any](ctx context.Context, flag *Flag, enabled bool, f func(context.Context) (T, error)) (result T, err error) {
	t := flag.set.getTracer()

	if t.ExperimentBranch != nil {
		var done func(any, error)
		ctx, done = t.ExperimentBranch(ctx, flag, enabled)

		if done != nil {
			defer func() { done(result, err) }()
		}
	}

	return f(ctx)
}

type FlagOpt func(*Flag)

// WithDescription sets the description for a new flag.
func WithDescription(desc string) FlagOpt {
	return func(f *Flag) {
		f.description = desc
	}
}

// WithLabels adds the given labels to a new flag.
func WithLabels(l map[string]any) FlagOpt {
	return func(f *Flag) {
		if f.labels == nil {
			f.labels = maps.Clone(l)
		} else {
			maps.Copy(f.labels, l)
		}
	}
}

// Flag represents a feature flag that can be enabled or disabled (toggled) dynamically at runtime and used to control
// the behaviour of an application, for example by dynamically changing code paths (see [Experiment] and [Switch]).
//
// A Flag must be obtained using either [New] or [Set.New].
type Flag struct {
	set *Set

	name        string
	description string
	labels      map[string]any
}

func (f *Flag) trace(ctx context.Context, enabled bool) {
	if t := f.set.getTracer(); t.Decision != nil {
		t.Decision(ctx, f, enabled)
	}
}

// Enabled returns true if the feature is enabled for the given context.
//
// Example:
//
//	if trackingFlag.Enabled(ctx) {
//	   trackUser(ctx, user)
//	}
func (f *Flag) Enabled(ctx context.Context) bool {
	s := f.set.strategy.Load()
	if s == nil {
		panic("no Strategy configured for set")
	}
	enabled := (*s).Enabled(ctx, f)
	f.trace(ctx, enabled)
	return enabled
}

// Name returns the name of the feature flag.
func (f *Flag) Name() string {
	return f.name
}

// Description returns the description of the defined feature.
func (f *Flag) Description() string {
	return f.description
}

// Labels returns a copy of the labels associated with this feature.
func (f *Flag) Labels() map[string]any {
	return maps.Clone(f.labels)
}

// Strategy defines an interface used for deciding on whether a feature is enabled or not.
//
// A Strategy must be safe for concurrent use.
type Strategy interface {
	// Enabled takes the name of a feature flag and returns true if the feature is enabled or false otherwise.
	Enabled(ctx context.Context, flag *Flag) bool
}

type fixedStrategy struct {
	d bool
}

var _ Strategy = fixedStrategy{}

// Enabled implements the Strategy interface.
func (f fixedStrategy) Enabled(context.Context, *Flag) bool {
	return f.d
}

// FixedStrategy returns a [Strategy] that always returns the given boolean decision.
func FixedStrategy(enabled bool) Strategy {
	return fixedStrategy{enabled}
}

// StrategyFunc implements a [Strategy] by calling itself.
type StrategyFunc func(ctx context.Context, flag *Flag) bool

var _ Strategy = (StrategyFunc)(nil)

// Enabled implements the [Strategy] interface.
func (f StrategyFunc) Enabled(ctx context.Context, flag *Flag) bool {
	return f(ctx, flag)
}

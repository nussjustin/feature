package feature

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"sync"
	"sync/atomic"
)

// Config contains configuration for a feature flag including the flag name and a description.
type Config struct {
	// Name defines the name for the flag used for this feature.
	Name string

	// Description contains an optional, human-readable description of the feature.
	Description string

	// Labels can be used to add additional metadata to a feature.
	Labels map[string]any

	// DefaultEnabled, if true, causes the feature to be enabled by default when no explicit defaultDecision can be made for
	// (either because no [Strategy] was set or because the final defaultDecision was [NoDecision]).
	DefaultEnabled bool
}

// Decision is an enum of the potential decisions a [Strategy] can make on whether a [Flag] should be enabled or not.
//
// Through the global [FixedStrategy] function, a [Decision] can be used directly as [Strategy], which can be useful
// for defining a global fallback.
type Decision string

const (
	// NoDecision indicates that a Strategy could not make a final Decision for a Flag.
	//
	// If this is the final value returned by a Strategy, or no Strategy was found for a Set, Flag.Enabled will behave
	// as if the Decision was Disabled.
	NoDecision Decision = "no_decision"
	// Disabled disables a feature flag and the new code path of the corresponding branch.
	Disabled Decision = "disabled"
	// Enabled enables a feature flag and the new code path of the corresponding branch.
	Enabled Decision = "enabled"
)

// If returns Enabled when the first argument is true, or Disabled otherwise.
func If(cond bool) Decision {
	if cond {
		return Enabled
	}
	return Disabled
}

// DecisionMap implements a simple [Strategy] that returns a fixed value for each flag by its name.
type DecisionMap map[string]Decision

var _ Strategy = (DecisionMap)(nil)

// Enabled implements the [Strategy] interface.
//
// If a feature with the given name is not found, [NoDecision] is returned.
func (m DecisionMap) Enabled(_ context.Context, flag *Flag) Decision {
	if d, ok := m[flag.Name()]; ok {
		return d
	}
	return NoDecision
}

// Set manages feature flags and can provide a [Strategy] (using [SetStrategy]) for making dynamic decisions about
// a flags' status.
//
// The zero value is usable as is, defaulting to all flags being disabled unless [Config.DefaultEnabled] is set.
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
func New(c Config) *Flag {
	return globalSet.New(c)
}

// New registers and returns a new [Flag] on s.
//
// If the given name is empty or already registered, New will panic.
func (s *Set) New(c Config) *Flag {
	if c.Name == "" {
		panic("missing name for flag")
	}

	return s.newFlag(c)
}

// SetStrategy sets or removes the [Strategy] for the global [Set].
//
// If more than one non-nil [Strategy] is given they will be checked in the order given, using the first result that
// is not [NoDecision] as the final result.
//
// See [Set.SetStrategy] for more information.
func SetStrategy(strategy Strategy, others ...Strategy) {
	globalSet.SetStrategy(strategy, others...)
}

// SetStrategy sets or removes the [Strategy] used by s to make decisions.
//
// If more than one non-nil [Strategy] is given they will be checked in the order given, using the first result that
// is not [NoDecision] as the final result.
func (s *Set) SetStrategy(strategy Strategy, others ...Strategy) {
	if len(others) > 0 {
		strategy = chainStrategies(append([]Strategy{strategy}, others...))
	}

	if strategy == nil {
		s.strategy.Store(nil)
	} else {
		s.strategy.Store(&strategy)
	}
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

func (s *Set) newFlag(c Config) *Flag {
	f := &Flag{
		set:             s,
		name:            c.Name,
		description:     c.Description,
		defaultDecision: If(c.DefaultEnabled),
		labels:          maps.Clone(c.Labels),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.flags == nil {
		s.flags = map[string]*Flag{}
	}

	if _, ok := s.flags[c.Name]; ok {
		panic(fmt.Sprintf("name %q already in use", c.Name))
	}

	s.flags[c.Name] = f

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
	Decision func(context.Context, *Flag, Decision)

	// Experiment is called at the beginning of every call to [Experiment].
	//
	// The returned function is called after both functions given to [Experiment] have returned and is passed
	// the values that will be returned as well as a boolean that indicates if the experiment was successful (the
	// results were equal and no errors occurred).
	//
	// The returned function can be nil.
	Experiment func(context.Context, *Flag, Decision) (context.Context, func(result any, err error, success bool))

	// ExperimentBranch is called for each called function during [Experiment] as well as for the function called by [Switch].
	//
	// The returned function is called after the called function has returned with the values returned by the function.
	//
	// The returned function can be nil.
	ExperimentBranch func(context.Context, *Flag, Decision) (context.Context, func(result any, err error))

	// Switch is called at the beginning of every call to [Switch].
	//
	// The returned function is called with the result that will be returned.
	//
	// The returned function can be nil.
	Switch func(context.Context, *Flag, Decision) (context.Context, func(result any, err error))
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
	isEnabled := flag.Enabled(ctx)

	var done func(result any, err error, success bool)
	if t := flag.set.getTracer(); t.Experiment != nil {
		ctx, done = t.Experiment(ctx, flag, If(isEnabled))
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
		experimentT, experimentErr = run(ctx, flag, Enabled, experimental)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		controlT, controlErr = run(ctx, flag, Disabled, control)
	}()

	wg.Wait()

	var result T
	var err error

	if isEnabled {
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
		ctx, done = t.Switch(ctx, flag, If(enabled))
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

func run[T any](ctx context.Context, flag *Flag, d Decision, f func(context.Context) (T, error)) (result T, err error) {
	t := flag.set.getTracer()

	if t.ExperimentBranch != nil {
		var done func(any, error)
		ctx, done = t.ExperimentBranch(ctx, flag, d)

		if done != nil {
			defer func() { done(result, err) }()
		}
	}

	return f(ctx)
}

// Flag represents a feature flag that can be enabled or disabled (toggled) dynamically at runtime and used to control
// the behaviour of an application, for example by dynamically changing code paths (see [Experiment] and [Switch]).
//
// A Flag must be obtained using either [New] or [Set.New].
//
// The zero value is not valid.
type Flag struct {
	set *Set

	name            string
	description     string
	defaultDecision Decision
	labels          map[string]any
}

func (f *Flag) trace(ctx context.Context, d Decision) {
	if t := f.set.getTracer(); t.Decision != nil {
		t.Decision(ctx, f, d)
	}
}

// Enabled returns true if the feature is enabled for the given context.
//
// A feature is considered enabled when the final [Decision], made by considering the [Strategy] set on the [Set] and
// the default [Decision] configured for the [Flag], is [Enabled].
//
// Example:
//
//	if trackingFlag.Enabled(ctx) {
//	   trackUser(ctx, user)
//	}
func (f *Flag) Enabled(ctx context.Context) bool {
	d := NoDecision
	if s := f.set.strategy.Load(); s != nil {
		d = (*s).Enabled(ctx, f)
	}
	if d == NoDecision {
		d = f.Default()
	}
	f.trace(ctx, d)
	return d == Enabled
}

// Name returns the name of the feature flag.
func (f *Flag) Name() string {
	return f.name
}

// Description returns the description of the defined feature.
func (f *Flag) Description() string {
	return f.description
}

// Labels returns the labels associated with this feature.
func (f *Flag) Labels() map[string]any {
	return maps.Clone(f.labels)
}

// Default returns the default defaultDecision configured for this feature.
func (f *Flag) Default() Decision {
	return f.defaultDecision
}

// Strategy defines an interface used for deciding on whether a feature is enabled or not.
//
// A Strategy must be safe for concurrent use.
type Strategy interface {
	// Enabled takes the name of a feature flag and returns a Decision that determines if the flag should be enabled.
	Enabled(ctx context.Context, flag *Flag) Decision
}

type chainStrategy []Strategy

func (c chainStrategy) Enabled(ctx context.Context, flag *Flag) Decision {
	for _, s := range c {
		if d := s.Enabled(ctx, flag); d != NoDecision {
			return d
		}
	}
	return NoDecision
}

func chainStrategies(strategies []Strategy) Strategy {
	chain := make([]Strategy, 0, len(strategies))

	for _, strategy := range strategies {
		if strategy != nil {
			chain = append(chain, strategy)
		}
	}

	if len(chain) == 0 {
		return nil
	}

	return chainStrategy(chain)
}

type fixedStrategy struct {
	d Decision
}

var _ Strategy = fixedStrategy{}

// Enabled implements the Strategy interface.
func (f fixedStrategy) Enabled(context.Context, *Flag) Decision {
	return f.d
}

// FixedStrategy returns a [Strategy] that always returns the given [Decision] d.
func FixedStrategy(d Decision) Strategy {
	return fixedStrategy{d}
}

// StrategyFunc implements a [Strategy] by calling itself.
type StrategyFunc func(ctx context.Context, flag *Flag) Decision

var _ Strategy = (StrategyFunc)(nil)

// Enabled implements the [Strategy] interface.
func (f StrategyFunc) Enabled(ctx context.Context, flag *Flag) Decision {
	return f(ctx, flag)
}

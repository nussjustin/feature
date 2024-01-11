package feature

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
)

// Config contains configuration for a feature flag including the flag name and a description.
type Config struct {
	// Flag defines the name for the flag used for this feature.
	Flag string

	// Description contains an optional, human-readable description of the feature.
	Description string

	// Default defines the default [Decision] (status) for this flag if no strategy was set or no final decision was made.
	//
	// An empty value is treated as [NoDecision].
	Default Decision
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
// The zero value is usable as is, using the default decision for each flag.
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
// If the given name is already is use by another flag, Register will panic.
func (s *Set) New(c Config) *Flag {
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
	if c.Default == "" {
		c.Default = NoDecision
	}

	f := &Flag{set: s, name: c.Flag, description: c.Description, decision: c.Default}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.flags == nil {
		s.flags = map[string]*Flag{}
	}

	if _, ok := s.flags[c.Flag]; ok {
		panic(fmt.Sprintf("name %q already in use", c.Flag))
	}

	s.flags[c.Flag] = f

	return f
}

// Tracer can be used to trace the use of both [Case] and [Flag] types for example to implement tracing or to collect
// metrics.
//
// See the documentation of the fields for information on what can be traced.
//
// All fields are optional.
//
// A basic, pre-configured [Tracer] using OpenTelemetry can be found in the otelfeature subpackage.
type Tracer struct {
	// Decision is called every time [Flag.Enabled] is called.
	Decision func(context.Context, *Flag, Decision)

	// Branch is called for each called function during [Experiment] as well as for the function called by [Switch].
	//
	// The returned function is called after the called function has returned with the values returned by the function.
	//
	// The returned function can be nil.
	Branch func(context.Context, *Flag, Decision) (context.Context, func(result any, err error))

	// BranchPanicked is called when a panic was caught as part of a function called by a [Case].
	BranchPanicked func(ctx context.Context, flag *Flag, decision Decision, panicError *PanicError)

	// Experiment is called at the beginning of every call to [Experiment].
	//
	// The returned function is called after both functions given to [Experiment] have returned and is passed
	// the [Decision] made by the given [Flag] and the values that will be returned as well as a boolean that indicates
	// if the experiment was successful (the results were equal and no errors occurred).
	//
	// The returned function can be nil.
	Experiment func(context.Context, *Flag) (context.Context, func(d Decision, result any, err error, success bool))

	// Run is called at the beginning of every call to [Switch].
	//
	// The returned function is called with the [Decision] made by the given [Flag] as well and the result that will
	// be returned.
	//
	// The returned function can be nil.
	Run func(context.Context, *Flag) (context.Context, func(d Decision, result any, err error))
}

// Equals returns a function that compares to values of the same type using ==.
//
// This can be used with [Experiment] when T is a comparable type.
func Equals[T comparable](a, b T) bool {
	return a == b
}

// PanicError holds the error recovered from one of the called functions when running an experiment.
type PanicError struct {
	// Recovered is the value recovered from the panic.
	Recovered any
}

var _ error = (*PanicError)(nil)

// Error implements the error interface.
func (p *PanicError) Error() string {
	return fmt.Sprintf("recovered: %v", p.Recovered)
}

// Unwrap returns Recovered if Recovered is an error or nil otherwise.
func (p *PanicError) Unwrap() error {
	if err, ok := p.Recovered.(error); ok {
		return err
	}

	return nil
}

// Experiment runs both an experimental and a control function concurrently and compares their results using equals.
//
// If the feature flag is enabled, the result of the experimental function will be returned, otherwise the result of the
// control function will be returned.
//
// When a function panics the panic is caught and converted into an error that is or wraps a [PanicError] and treated
// like a normal error.
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
	var done func(d Decision, result any, err error, success bool)
	if t := flag.set.getTracer(); t.Experiment != nil {
		ctx, done = t.Experiment(ctx, flag)
	}

	// Check status before while the experiment runs. This can save some time if the used Strategy is slow.
	isEnabled := flag.Enabled(ctx)

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
		done(If(isEnabled), result, err, ok)
	}

	return result, err
}

// Switch checks if the associated flag is enabled and runs either ifEnabled or ifDisabled and returns their result.
func Switch[T any](ctx context.Context, flag *Flag,
	ifEnabled func(context.Context) (T, error),
	ifDisabled func(context.Context) (T, error),
) (T, error) {
	var done func(Decision, any, error)
	if t := flag.set.getTracer(); t.Run != nil {
		ctx, done = t.Run(ctx, flag)
	}

	enabled := flag.Enabled(ctx)

	fn, d := ifDisabled, Disabled
	if enabled {
		fn, d = ifEnabled, Enabled
	}

	resultT, err := run(ctx, flag, d, fn)

	if done != nil {
		done(d, resultT, err)
	}

	return resultT, err
}

func run[T any](ctx context.Context, flag *Flag, d Decision, f func(context.Context) (T, error)) (result T, err error) {
	t := flag.set.getTracer()

	if t.Branch != nil {
		var done func(any, error)
		ctx, done = t.Branch(ctx, flag, d)

		if done != nil {
			defer func() { done(result, err) }()
		}
	}

	defer func() {
		if v := recover(); v != nil {
			panicErr := &PanicError{Recovered: v}

			if t.BranchPanicked != nil {
				t.BranchPanicked(ctx, flag, d, panicErr)
			}

			err = panicErr
		}
	}()

	return f(ctx)
}

// Flag represents a feature flag that can be enabled or disabled (toggled) dynamically at runtime and used to control
// the behaviour of an application, for example by dynamically changing code paths (see [Case]).
//
// In many cases a [Case] can be used to simplify working with a [Flag]. See the documentation and examples for [Case]
// for more information on how to use a [Case].
//
// A Flag must be obtained using either [New] or [Register].
//
// The zero value is not valid.
type Flag struct {
	set *Set

	name        string
	description string
	decision    Decision
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

// Default returns the default decision configured for this feature.
func (f *Flag) Default() Decision {
	return f.decision
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

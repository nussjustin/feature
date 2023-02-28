package feature

import (
	"context"
	"fmt"
	"sort"
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
	// Disabled disables a feature flag and the new code path of the corresponding case.
	Disabled Decision = "disabled"
	// Enabled enables a feature flag and the new code path of the corresponding case.
	Enabled Decision = "enabled"
)

var _ Strategy = Default

// If returns Enabled when the first argument is true, or Disabled otherwise.
func If(cond bool) Decision {
	if cond {
		return Enabled
	}
	return Disabled
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
	// DefaultDisabled causes flags and cases to treat a Default decision as Disabled.
	DefaultDisabled DefaultDecision = "disabled"
	// DefaultEnabled causes flags and cases to treat a Default decision as Enabled.
	DefaultEnabled DefaultDecision = "enabled"
)

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

// SetStrategy sets or removes the [Strategy] for the global [Set].
//
// If more than one non-nil [Strategy] is given they will be checked in the order given, using the first non-[Default]
// decision as the final result.
//
// See [Set.SetStrategy] for more information.
func SetStrategy(strategy Strategy, others ...Strategy) {
	globalSet.SetStrategy(strategy, others...)
}

// SetStrategy sets or removes the [Strategy] used by s to make decisions.
//
// If more than one non-nil [Strategy] is given they will be checked in the order given, using the first non-[Default]
// decision as the final result.
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

	// Case is called for each called function during [Case.Experiment] as well as for the function called by [Case.Run].
	//
	// The returned function is called after the called function has returned with the values returned by the function.
	//
	// The returned function can be nil.
	Case func(context.Context, *Flag, Decision) (context.Context, func(result any, err error))

	// CasePanicked is called when a panic was caught as part of a function called by a [Case].
	CasePanicked func(ctx context.Context, flag *Flag, decision Decision, panicError *PanicError)

	// Experiment is called at the beginning of every call to [Case.Experiment].
	//
	// The returned function is called after both functions given to [Case.Experiment] have returned and is passed
	// the [Decision] made by the given [Flag] and the values that will be returned as well as a boolean that indicates
	// if the experiment was successful (the results were equal and no errors occurred).
	//
	// The returned function can be nil.
	Experiment func(context.Context, *Flag) (context.Context, func(d Decision, result any, err error, success bool))

	// Run is called at the beginning of every call to [Case.Run].
	//
	// The returned function is called with the [Decision] made by the given [Flag] as well and the result that will
	// be returned.
	//
	// The returned function can be nil.
	Run func(context.Context, *Flag) (context.Context, func(d Decision, result any, err error))
}

// Case can be used to simplify running code paths dynamically based on whether a feature is enabled.
//
// Additionally, _experiments_ can be run using the [Case.Experiment] method, which compare the results of two
// functions, while safely returning the correct value based on the status of the feature.
//
// See [Flag.Enabled] for an explanation on how a [Case] determines whether to return hew result from the first
// (feature flag enabled) or second (feature flag disabled) function.
//
// A Case must be obtained using either [CaseFor], [NewCase] or [RegisterCase]
//
// The zero value is not valid.
type Case[T any] struct {
	flag *Flag
}

// CaseFor returns a new [Case] for the given registered [Flag].
func CaseFor[T any](f *Flag) *Case[T] {
	return &Case[T]{flag: f}
}

// NewCase registers and returns a new [Case] with the global [Set].
//
// See [RegisterCase] for more details.
func NewCase[T any](name string, description string, defaultDecision DefaultDecision) *Case[T] {
	return RegisterCase[T](&globalSet, name, description, defaultDecision)
}

// RegisterCase registers and returns a new [Flag] with the given [Set].
//
// A nil [Strategy] is equivalent to passing [Default].
//
// If the given name is already is use by another case or flag, RegisterCase will panic.
func RegisterCase[T any](set *Set, name string, description string, defaultDecision DefaultDecision) *Case[T] {
	return CaseFor[T](Register(set, name, description, defaultDecision))
}

// Equals returns a function that compares to values of the same type using ==.
//
// This can be used with [Case.Experiment] when T is a comparable type.
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

func (c *Case[T]) run(ctx context.Context, d Decision, f func(context.Context) (T, error)) (result T, err error) {
	if t := c.flag.set.getTracer(); t.Case != nil {
		var done func(any, error)
		ctx, done = t.Case(ctx, c.flag, d)

		if done != nil {
			defer func() { done(result, err) }()
		}
	}

	defer func() {
		if v := recover(); v != nil {
			panicErr := &PanicError{Recovered: v}

			if t := c.flag.set.getTracer(); t.CasePanicked != nil {
				t.CasePanicked(ctx, c.flag, d, panicErr)
			}

			err = panicErr
		}
	}()

	return f(ctx)
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
func (c *Case[T]) Experiment(ctx context.Context,
	experimental func(context.Context) (T, error),
	control func(context.Context) (T, error),
	equals func(new, old T) bool,
) (T, error) {
	var done func(d Decision, result any, err error, success bool)
	if t := c.flag.set.getTracer(); t.Experiment != nil {
		ctx, done = t.Experiment(ctx, c.flag)
	}

	// Check status before while the experiment runs. This can save some time if the used Strategy is slow.
	isEnabled := c.flag.Enabled(ctx)

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
		experimentT, experimentErr = c.run(ctx, Enabled, experimental)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		controlT, controlErr = c.run(ctx, Disabled, control)
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

// Run checks if the associated flag is enabled and runs either ifEnabled or ifDisabled and returns their result.
func (c *Case[T]) Run(ctx context.Context,
	ifEnabled func(context.Context) (T, error),
	ifDisabled func(context.Context) (T, error),
) (T, error) {
	var done func(Decision, any, error)
	if t := c.flag.set.getTracer(); t.Run != nil {
		ctx, done = t.Run(ctx, c.flag)
	}

	enabled := c.flag.Enabled(ctx)

	fn := ifDisabled
	if enabled {
		fn = ifEnabled
	}

	resultT, err := c.run(ctx, If(enabled), fn)

	if done != nil {
		done(If(enabled), resultT, err)
	}

	return resultT, err
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

	name            string
	description     string
	defaultDecision DefaultDecision
}

// New registers and returns a new [Flag] with the global [Set].
//
// See [Register] for more details.
func New(name string, description string, defaultDecision DefaultDecision) *Flag {
	return Register(&globalSet, name, description, defaultDecision)
}

// Register registers and returns a new [Flag] with the given [Set].
//
// A nil [Strategy] is equivalent to passing [Default].
//
// If the given name is already is use by another case or flag, Register will panic.
func Register(set *Set, name string, description string, defaultDecision DefaultDecision) *Flag {
	return set.newFlag(name, description, defaultDecision)
}

func (f *Flag) trace(ctx context.Context, d Decision) {
	if t := f.set.getTracer(); t.Decision != nil {
		t.Decision(ctx, f, d)
	}
}

// Enabled returns true if the feature is enabled for the given context.
//
// The status of the flag is determined as follows:
//
//  1. The [Strategy] of the [Flag] is checked. If no [Strategy] is set on the [Flag] or the [Strategy] returns [Default]
//     Enabled will continue to the next step.
//
//  2. The [Strategy] of the associated [Set] is checked. If no [Strategy] is set on the [Set] or the [Strategy] returns
//     [Default], Enabled will continue to the next step.
//
//  3. If the previous steps did not result in a final decision ([Enabled] or [Disabled]), the [DefaultDecision] of the
//     flag is used.
//
// Example:
//
//	if trackingFlag.Enabled(ctx) {
//	   trackUser(ctx, user)
//	}
func (f *Flag) Enabled(ctx context.Context) bool {
	if s := f.set.strategy.Load(); s != nil {
		if d := (*s).Enabled(ctx, f.name); d != Default {
			f.trace(ctx, d)
			return d == Enabled
		}
	}

	d := Decision(f.defaultDecision)

	f.trace(ctx, d)
	return d == Enabled
}

// Name returns the name passed to [New] or [Register].
func (f *Flag) Name() string {
	return f.name
}

// Description returns the description passed to [New] or [Register].
func (f *Flag) Description() string {
	return f.description
}

// Strategy defines an interface used for deciding on whether a feature is enabled or not.
//
// A Strategy must be safe for concurrent use.
type Strategy interface {
	// Enabled takes the name of a feature flag and returns a Decision that determines if the flag should be enabled.
	Enabled(ctx context.Context, name string) Decision
}

type chainStrategy []Strategy

func (c chainStrategy) Enabled(ctx context.Context, name string) Decision {
	for _, s := range c {
		if d := s.Enabled(ctx, name); d != Default {
			return d
		}
	}
	return Default
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

// StrategyFunc implements a [Strategy] by calling itself.
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

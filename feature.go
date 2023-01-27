package feature

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	tracerName = "github.com/nussjustin/feature"
)

var (
	attributeFeatureEnabled  = attribute.Key("feature.enabled")
	attributeFeatureName     = attribute.Key("feature.name")
	attributeExperimentMatch = attribute.Key("feature.experiment.match")
	attributeExperimentPanic = attribute.Key("feature.experiment.panic")
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

// Set manages feature flags and can provide a [Strategy] (using [SetStrategy]) for making dynamic decisions about
// a flags' status.
//
// The zero value is usable as is, using the default decision for each flag.
type Set struct {
	strategy atomic.Pointer[Strategy]
	tracer   atomic.Pointer[trace.Tracer]

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

// SetTracerProvider sets the provider used for creating a tracer for the global [Set].
//
// See [Set.SetTracerProvider] for more information.
func SetTracerProvider(tp trace.TracerProvider) {
	globalSet.SetTracerProvider(tp)
}

// SetTracerProvider sets the provider used for creating a tracer.
func (s *Set) SetTracerProvider(tp trace.TracerProvider) {
	t := tp.Tracer(tracerName)

	s.tracer.Store(&t)
}

func (s *Set) getTracer() trace.Tracer {
	ptr := s.tracer.Load()

	if ptr != nil {
		return *ptr
	}

	tracer := otel.GetTracerProvider().Tracer(tracerName)

	// No need for CAW since the tracer is always the same
	s.tracer.Store(&tracer)

	return tracer
}

func (s *Set) newFlag(name, description string, strategy Strategy, defaultDecision DefaultDecision) *Flag {
	f := &Flag{set: s, name: name, description: description, strategy: strategy, defaultDecision: defaultDecision}

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
// Additionally, _experiments_ can be run using the [Case.Experiment] method, which compare the results of two
// functions, while safely returning the correct value based on the status of the feature.
//
// Functions executed via [Case.Run] and experiments run via [Case.Experiment] are traced using spans created
// via the tracer obtained from the [otel.TracerProvider] set using the [Set.SetTracerProvider] method (or the
// global [SetTracerProvider] function, if using the global [Set]).
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
func NewCase[T any](name string, description string, strategy Strategy, defaultDecision DefaultDecision) *Case[T] {
	return RegisterCase[T](&globalSet, name, description, strategy, defaultDecision)
}

// RegisterCase registers and returns a new [Flag] with the given [Set].
//
// A nil [Strategy] is equivalent to passing [Default].
//
// If the given name is already is use by another case or flag, RegisterCase will panic.
func RegisterCase[T any](set *Set, name string, description string, strategy Strategy, defaultDecision DefaultDecision) *Case[T] {
	return CaseFor[T](RegisterFlag(set, name, description, strategy, defaultDecision))
}

// Equals returns a function that compares to values of the same type using ==.
//
// This can be used with [Case.Experiment] when T is a comparable type.
func Equals[T comparable](a, b T) bool {
	return a == b
}

func (c *Case[T]) startSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return c.flag.set.getTracer().Start(ctx, spanName, opts...)
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

func (c *Case[T]) run(ctx context.Context, name string, f func(context.Context) (T, error)) (t T, err error) {
	ctx, span := c.startSpan(ctx, name, trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	defer func() {
		var panicked bool

		if v := recover(); v != nil {
			panicked = true
			err = &PanicError{name: name, Value: v}
		}

		span.SetAttributes(attributeExperimentPanic.Bool(panicked))

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}
	}()

	t, err = f(ctx)
	return
}

// Experiment runs both an experimental and a control function concurrently and compares their results using equals.
//
// If the feature flag is enabled, the result of the experimental function will be returned, otherwise the result of the
// control function will be returned.
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
	experimental func(context.Context) (T, error),
	control func(context.Context) (T, error),
	equals func(new, old T) bool,
) (T, error) {
	ctx, span := c.startSpan(ctx, "Experiment", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	// Check status before while the experiment runs. This can save some time if the used Strategy is slow.
	isEnabled := c.flag.Enabled(ctx)

	span.SetAttributes(
		attributeFeatureEnabled.Bool(isEnabled),
		attributeFeatureName.String(c.flag.name))

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
		experimentT, experimentErr = c.run(ctx, "Experimental", experimental)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		controlT, controlErr = c.run(ctx, "Control", control)
	}()

	wg.Wait()

	handleExperimentResult(span, experimentT, experimentErr, controlT, controlErr, isEnabled, equals)

	if isEnabled {
		return experimentT, experimentErr
	}

	return controlT, controlErr
}

func handleExperimentResult[T any](
	span trace.Span,

	experimentT T,
	experimentErr error,

	controlT T,
	controlErr error,

	isEnabled bool,

	equals func(new T, old T) bool,
) {
	switch {
	// If both failed, the whole operation failed
	case experimentErr != nil && controlErr != nil:
		span.SetStatus(codes.Error, "control and experiment failed")

	// If the active (enabled) path failed, we make sure to explicitly state this.
	case experimentErr != nil && isEnabled:
		span.SetStatus(codes.Error, "active code path (experiment) failed")
	case controlErr != nil && !isEnabled:
		span.SetStatus(codes.Error, "active code path (control) failed")

	// Otherwise on error we explicitly state that only the inactive path failed.
	case experimentErr != nil:
		span.SetStatus(codes.Error, "inactive code path (experiment) failed")
	case controlErr != nil:
		span.SetStatus(codes.Error, "inactive code path (experiment) failed")

	// If there was no error we can finally compare the results
	case !equals(experimentT, controlT):
		span.SetAttributes(attributeExperimentMatch.Bool(false))
		span.SetStatus(codes.Error, "result mismatch")

	default:
		span.SetAttributes(attributeExperimentMatch.Bool(true))
		span.SetStatus(codes.Ok, "")
	}
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
	ctx, span := c.startSpan(ctx, "Run", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	enabled := c.flag.Enabled(ctx)

	span.SetAttributes(
		attributeFeatureEnabled.Bool(enabled),
		attributeFeatureName.String(c.flag.name))

	var resultT T
	var err error

	if enabled {
		resultT, err = ifEnabled(ctx)
	} else {
		resultT, err = ifDisabled(ctx)
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	return resultT, err
}

// Flag represents a feature flag that can be enabled or disabled based on some kind of logic and used to control
// the behaviour of an application for example by dynamically changing code paths (see [Case]).
//
// Example:
//
//	    if trackingFlag.Enabled(ctx) {
//	        trackUser(ctx, user)
//		}
//
// In many cases a [Case] can be used to simplify working with a [Flag]. See the documentation and examples for [Case]
// for more information on how to use a [Case].
//
// A Flag must be obtained using either [NewFlag] or [RegisterFlag].
//
// The zero value is not valid.
type Flag struct {
	set *Set

	name            string
	description     string
	strategy        Strategy
	defaultDecision DefaultDecision
}

// NewFlag registers and returns a new [Flag] with the global [Set].
//
// See [RegisterFlag] for more details.
func NewFlag(name string, description string, strategy Strategy, defaultDecision DefaultDecision) *Flag {
	return RegisterFlag(&globalSet, name, description, strategy, defaultDecision)
}

// RegisterFlag registers and returns a new [Flag] with the given [Set].
//
// A nil [Strategy] is equivalent to passing [Default].
//
// If the given name is already is use by another case or flag, RegisterFlag will panic.
func RegisterFlag(set *Set, name string, description string, strategy Strategy, defaultDecision DefaultDecision) *Flag {
	return set.newFlag(name, description, strategy, defaultDecision)
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
//	    if trackingFlag.Enabled(ctx) {
//	        trackUser(ctx, user)
//		}
func (f *Flag) Enabled(ctx context.Context) bool {
	if f.strategy != nil {
		if d := f.strategy.Enabled(ctx, f.name); d != Default {
			return d == Enabled
		}
	}

	if s := f.set.strategy.Load(); s != nil {
		if d := (*s).Enabled(ctx, f.name); d != Default {
			return d == Enabled
		}
	}

	return Decision(f.defaultDecision) == Enabled
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

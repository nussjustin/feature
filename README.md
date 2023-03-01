# feature [![Go Reference](https://pkg.go.dev/badge/github.com/nussjustin/feature.svg)](https://pkg.go.dev/github.com/nussjustin/feature) [![Lint](https://github.com/nussjustin/feature/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/nussjustin/feature/actions/workflows/golangci-lint.yml) [![Test](https://github.com/nussjustin/feature/actions/workflows/test.yml/badge.svg)](https://github.com/nussjustin/feature/actions/workflows/test.yml)

Package feature provides a simple, easy to use abstraction for working with feature flags in Go.

## Examples

### Defining and checking a flag

To define a flag use the global [New](https://pkg.go.dev/github.com/nussjustin/feature#New) function or, when using a
custom [Set](https://pkg.go.dev/github.com/nussjustin/feature#Set),
[Set.New](https://pkg.go.dev/github.com/nussjustin/feature#Set.New).

`New` takes a name for the flag and an optional description.

```go
var newUIFlag = feature.New("new-ui", "enables the new UI")
```

The status of the flag can be checked via the [Enabled](https://pkg.go.dev/github.com/nussjustin/feature#Flag.Enabled)
method which returns either `true` or `false`.

```go
var tmpl *template.Template

if newUIFlag.Enabled(ctx) {
    tmpl = template.Must(template.Parse("new-ui/*.gotmpl")))
} else {
    tmpl = template.Must(template.Parse("old-ui/*.gotmpl")))
}
```

### Using `Switch` to switch between code paths

A common use case for flags is switching between code paths, for example using a `if`/`else` combination.

The global [Switch](https://pkg.go.dev/github.com/nussjustin/feature#Switch) function provides an abstraction for this.

When using the provided `Switch` function both the decision making and the results of the call can be traced using the
builtin tracing functionality.

To use `Switch` first define a flag:

```go
var newUIFlag = feature.New("new-ui", "enables the new UI")
```

Later in your code, just call `Switch` and pass the flag together with 2 callbacks, one for when the flag is enabled and
one for when it is not.

```go
tmpl, err := feature.Switch(ctx, newUIFlag, 
	func(context.Context) (*template.Template, error) { return template.Parse("new-ui/*.gotmpl") },
    func(context.Context) (*template.Template, error) { return template.Parse("old-ui/*.gotmpl") })
```

### Running an experiment

In addition to simply switching between two functions using `Switch`, it is also possible to run both two functions
concurrently and compare their results in order to compare code paths.

To do this use the global [Experiment](https://pkg.go.dev/github.com/nussjustin/feature#Experiment) function and pass
the feature flag, two functions that will be run and a callback to compare the results.

Calling `Experiment` will automatically run both functions and compare the results and pass the result of the
comparison to the `Tracer.Experiment` function of the configured `Tracer`, if any.

The result of `Experiment` depends on the result of checking the flags status. If the flag is enabled, the results of
the first function is returned. Otherwise the results of the second function are returned.

Example:

```go
result, err := feature.Experiment(ctx, optimizationFlag, optimizedFunction, unoptimizedFunction, feature.Equals)
```

### Using different sets of flags

All flags and cases created via `New` belong to a single global set of flags.

In some cases applications may want to have multiple sets, for example when extracting a piece of code into its own
package and importing packages not wanting to clobber the global flag set with the imported, by not necessarily used
flags.

For this and similar scenarios it is possible to create a custom
[Set](https://pkg.go.dev/github.com/nussjustin/feature#Set) which acts as its own separate namespace of flags.

When using a custom `Set` instead of using the `New` function, the `Set.New` method must be used.

Example:

```go
var mySet feature.Set // zero value is valid

var optimizationFlag = mySet.New("new-ui", "enables the new UI")
```

### Using dynamic strategies for controlling flags

By default, all flags are considered disabled.

In order to enable flags, either statically or dynamically, a
[Strategy](https://pkg.go.dev/github.com/nussjustin/feature#Strategy) must be created an associated with the `Set` using
`Set.SetStrategy` or, if using the global set, the global `SetStrategy` function.

Example:

```go
func main() {
    feature.SetStrategy(myCustomStrategy)
}
```

Or when using a custom `Set`:

```go
func main() {
    mySet.SetStrategy(myCustomStrategy)
}
```

Both the global function and the method take zero or more strategies. If no strategies are given, all flags are
disabled. Otherwise, the strategies are checked in order until one returns a final decision.

The `Strategy` interface is defined as follows:

```go
type Strategy interface {
    Enabled(ctx context.Context, flag *Flag) Decision
}
```

### Changing strategies at runtime

The `Strategy` can be changed at any time during runtime. This can be useful for applications that keep cached states
of all states in memory and periodically receive new states.

For cases like this the [StrategyMap](https://pkg.go.dev/github.com/nussjustin/feature#StrategyMap) type can be useful,
which delegates the check for each flag to a different `Strategy` based on the name of the feature.

Example:

```go
func main() {
	feature.SetStrategy(loadFlags())
	
	go func() {
		// Update flags every minute
		for range time.Ticker(time.Minute) {
			feature.SetStrategy(loadFlags())
		}
	}()
}
```

## Tracing

It is possible to trace the use of `Flag`s using the [Tracer](https://pkg.go.dev/github.com/nussjustin/feature#Tracer)
type.

In order to trace the use of this package simply use the global
[SetStrategy](https://pkg.go.dev/github.com/nussjustin/feature#SetStrategy) function to register a `Tracer`:

```go
func main() {
    feature.SetTracer(myTracer)
}
```

Or when using a custom `Set`:

```go
func main() {
    mySet.SetTracer(myCustomStrategy)
}
```

### OpenTelemetry integration

# TODO: Document metrics

The `otelfeature` package found at
[github.com/nussjustin/feature/otelfeature](https://pkg.go.dev/github.com/nussjustin/feature/otelfeature) exposes a
function that returns pre-configured `Tracer` that implements basic metrics and tracing for `Flag`s as well as the
global `Switch` and `Experiment` functions using [OpenTelemetry](https://opentelemetry.io/).

In order to enable metrics collection and tracing use the global
[otelfeature.Tracer](https://pkg.go.dev/github.com/nussjustin/feature/otelfeature#Tracer) function to create a new
`feature.Tracer` that can be passed to either the global `SetTracer` function or the `Set.SetStrategy` method.

```go
func main() {
    tracer, err := otelfeature.Tracer(nil)
    if err != nil {
        // Handle the error
    }
    feature.SetTracer(tracer)
}
```

## Contributing
Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

Please make sure to update tests as appropriate.

## License
[MIT](https://choosealicense.com/licenses/mit/)

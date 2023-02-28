# feature [![Go Reference](https://pkg.go.dev/badge/github.com/nussjustin/feature.svg)](https://pkg.go.dev/github.com/nussjustin/feature) [![Lint](https://github.com/nussjustin/feature/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/nussjustin/feature/actions/workflows/golangci-lint.yml) [![Test](https://github.com/nussjustin/feature/actions/workflows/test.yml/badge.svg)](https://github.com/nussjustin/feature/actions/workflows/test.yml)

Package feature provides a simple, easy to use abstraction for working with feature flags in Go.

## Examples

### Defining and checking a flag

To define a flag use the [New](https://pkg.go.dev/github.com/nussjustin/feature#New) or
[Register](https://pkg.go.dev/github.com/nussjustin/feature#Register) functions.

Both functions take a name for the flag, and optional description, an optional
[Strategy](https://pkg.go.dev/github.com/nussjustin/feature#Strategy) that can be used to toggle the flag dynamically
and a default _decision_ (whether the feature should be enabled or disabled).

```go
var newUIFlag = feature.New("new-ui", "enables the new UI", feature.DefaultDisabled)
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

### Using `Case` to switch between code paths

A common use case for flags is switching between code paths, for example using a `if`/`else` combination.

Instead of manually checking the flag state, a [Case](https://pkg.go.dev/github.com/nussjustin/feature#Case) can be used
to abstract the switch.

In addition to abstracting the condition, a `Case` also makes it possible to trace the decision making and results of
the called function.

There are 2 ways to obtain a `Case`:

#### Using `CaseFor` with an existing `Flag`:

The global [CaseFor](https://pkg.go.dev/github.com/nussjustin/feature#CaseFor) function can be used to create a `Case`
from an existing feature flag like this:

```go
var newUICase = feature.CaseFor[*template.Template](
	feature.New("new-ui", "enables the new UI", feature.DefaultDisabled),
)
```

The main method of `Case` is [Run](https://pkg.go.dev/github.com/nussjustin/feature#Case.Run) which takes two functions,
one of which will be called depending on whether the flag is enabled.

```go
tmpl, err := newUICase.Run(ctx, 
	func(context.Context) (*template.Template, error) { return template.Parse("new-ui/*.gotmpl") },
    func(context.Context) (*template.Template, error) { return template.Parse("old-ui/*.gotmpl") })
```

#### Directly creating a `Case` using `NewCase` or `RegisterCase`

The functions [NewCase](https://pkg.go.dev/github.com/nussjustin/feature#NewCase) and
[RegisterCase](https://pkg.go.dev/github.com/nussjustin/feature#RegisterCase) allow creating a `Case` directly.

The previous example thus could also be written as

```go
var newUICase = feature.NewCase[*template.Template]("new-ui", "enables the new UI", feature.DefaultDisabled)
```

Note that in this case the underlying `Flag` is completely hidden and can not be accessed directly.

### Running an experiment

In addition to simply switching between two functions, a `Case` can also be used to run two functions and compare their
result using a custom comparison function.

Both result of the experiment and the result of the comparison can be traced using the built in tracing functionality.

Example:

```go
result, err := optimizationCase.Experiment(ctx, optimizedFunction, unoptimizedFunction)
```

Both functions will be called concurrently and the results compared.

By default, that is if the associated feature flag is disabled, the result of the second (control) function is returned
no matter the result.

If the feature flag is enabled the returned values will instead be the result of the first (experimental) function.

This makes it possible to seamlessly switch over to using the new result while still allowing
for a quick fallback to the old result by simply toggling the flag.

### Using different sets of flags

All flags and cases created via `New` or `NewCase` belong to a single global set of flags.

In some cases applications may want to have multiple sets, for example when extracting a piece of code into its own
package and importing packages not wanting to clobber the global flag set with the imported, by not necessarily used
flags.

For this and similar scenarios it is possible to create a custom
[Set](https://pkg.go.dev/github.com/nussjustin/feature#Set) which acts as its own separate namespace of flags.

When using a custom `Set` instead of using the `New` or `NewCase` functions, the `Register` and `RegisterCase`
functions must be used, which take the `Set` as first parameter.

Example:

```go
var mySet feature.Set // zero value is valid

var optimizationFlag = feature.Register(
	&mySet,
	"new-ui",
	"enables the new UI", 
	nil,
	feature.DefaultDisabled,
)

var newUICase = feature.RegisterCase[*template.Template](
	&mySet,
	"new-ui",
	"enables the new UI",
	nil,
	feature.DefaultDisabled,
)
```

`Flag`s registered on a custom `Set` can still use `CaseFor` to obtain a `Case`. 

### Using dynamic strategies for controlling flags

By default, all flags are enabled or disabled simply based on the
[DefaultDecision](https://pkg.go.dev/github.com/nussjustin/feature#DefaultDecision) given to `New`/`Register`/
`NewCase`/`RegisterCase`.

In many cases such static values are not enough, for example for applications that want to be able to change flags at
runtime without having to restart the application or when wanting to enable only for some set of operations or users (
e.g. for A/B testing).

This can be achieved using a [Strategy](https://pkg.go.dev/github.com/nussjustin/feature#Strategy).

The `Strategy` interface is defined as follows:

```go
type Strategy interface {
    Enabled(ctx context.Context, name string) Decision
}
```

By implementing the `Strategy` interface applications can make dynamic decisions for flags.

#### Per set/global `Strategy`

It is also possible to set a per set `Strategy` using the
[Set.SetStrategy](https://pkg.go.dev/github.com/nussjustin/feature#Set.SetStrategy) method or, when using the global
set, the global [SetStrategy](https://pkg.go.dev/github.com/nussjustin/feature#SetStrategy) function.

For example:

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

#### Order of checks

When checking if a feature flag is enabled, first the `Strategy` associated with the `Flag`/`Case` is checked.

If the `Strategy` is nil or the result is `Default`, the `Strategy` associated with the set is used.

If this is also nil or the result is also `Default`, the `DefaultDecision` of the `Flag/`/`Case` is used.

## Tracing

It is possible to trace the use `Flag`s and `Case`s using the
[Tracer](https://pkg.go.dev/github.com/nussjustin/feature#Tracer) type.

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
function that returns pre-configured `Tracer` that implements basic metrics and tracing for `Flag`s and `Case`s using
[OpenTelemetry](https://opentelemetry.io/).

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

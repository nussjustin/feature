# feature [![Go Reference](https://pkg.go.dev/badge/github.com/nussjustin/feature.svg)](https://pkg.go.dev/github.com/nussjustin/feature) [![Lint](https://github.com/nussjustin/feature/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/nussjustin/feature/actions/workflows/golangci-lint.yml) [![Test](https://github.com/nussjustin/feature/actions/workflows/test.yml/badge.svg)](https://github.com/nussjustin/feature/actions/workflows/test.yml)

Package feature implements a simple abstraction for feature flags with arbitrary values.

## Examples

### Registering a flag

A flag is registered on a [FlagSet][0].

Flags are created using a specific method based on the type of the value of the flag, named after the type.

Currently, the supported methods are

* [FlagSet.Any][14] and [FlagSet.AnyFunc][15] for flags with arbitrary values,
* [FlagSet.Bool][1] and [FlagSet.BoolFunc][8] for boolean flags,
* [FlagSet.Duration][7] and [FlagSet.DurationFunc][9] for duration flags,
* [FlagSet.Float64][2] and [FlagSet.Float64Func][10] for float flags,
* [FlagSet.Int][3] and [FlagSet.IntFunc][11] for int flags,
* [FlagSet.String][4] and [FlagSet.StringFunc][12] for string flags,
* [FlagSet.Uint][5] and [FlagSet.UintFunc][13] for uint flags.

Each method will return a callback that takes a `context.Context` and returns a value of the specific type.

There is also [feature.Typed][16] and [feature.TypedFunc][17] which can be used to register flags using an
arbitrary, generic type. Note that you can not currently override typed values for a context in a safe way.

For example:

```go
package main

import (
	"context"

	"github.com/nussjustin/feature"
)

func main() {
	var set feature.FlagSet

	myFeature := set.Bool("my-feature", "some new feature", false)

	if myFeature(context.Background()) {
		println("my-feature enabled") // never runs, see next section
	}
}
```

It is also possible to register a flag with a callback that is used to determine the flag value dynamically:

```go
package main

import (
	"context"

	"github.com/nussjustin/feature"
)

func main() {
	var set feature.FlagSet

	myFeature := set.BoolFunc("my-feature", "some new feature", func(ctx context.Context) bool {
		// ... do something with ctx ...
		return false
	})

	if myFeature(context.Background()) {
		println("my-feature enabled") // never runs, see next section
	}
}
```

### Context-specific values

By default, the values returned for each flag will be the default value specified when creating the flag.

The [FlagSet.Context][6] method can be used to set custom values for feature flags on a per-context basis.

Example:

```go
package main

import (
	"context"

	"github.com/nussjustin/feature"
)

func main() {
	var set feature.FlagSet

	myFeature := set.Bool("my-feature", "some new feature", false)

	// Enable the feature for our context
	ctx := set.Context(context.Background(),
		feature.BoolValue("my-feature", true))
	
	if myFeature(ctx) {
		println("my-feature enabled")
	}
}

```

## Contributing
Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

Please make sure to update tests as appropriate.

## License
[MIT](https://choosealicense.com/licenses/mit/)

[0]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet
[1]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.Bool
[2]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.Float64
[3]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.Int
[4]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.String
[5]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.Uint
[6]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.Context
[7]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.Duration
[8]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.BoolFunc
[9]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.DurationFunc
[10]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.Float64Func
[11]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.IntFunc
[12]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.StringFunc
[13]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.UintFunc
[14]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.Any
[15]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.AnyFunc
[16]: https://pkg.go.dev/github.com/nussjustin/feature/#Typed
[17]: https://pkg.go.dev/github.com/nussjustin/feature/#TypedFunc
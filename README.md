# feature [![Go Reference](https://pkg.go.dev/badge/github.com/nussjustin/feature.svg)](https://pkg.go.dev/github.com/nussjustin/feature) [![Lint](https://github.com/nussjustin/feature/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/nussjustin/feature/actions/workflows/golangci-lint.yml) [![Test](https://github.com/nussjustin/feature/actions/workflows/test.yml/badge.svg)](https://github.com/nussjustin/feature/actions/workflows/test.yml)

Package feature implements a simple abstraction for feature flags with arbitrary values.

## Examples

### Registering a flag

A flag is registered on a [FlagSet][0].

Flags are created using a specific method based on the type of the value of the flag, named after the type.

Currently, the supported methods are

* [FlagSet.Bool][1] for boolean flags,
* [FlagSet.Float][2] for float flags,
* [FlagSet.Int][3] for int flags and
* [FlagSet.String][4] for string flags.

Each method will return a callback that takes a `context.Context` and returns a value of the specific type.

Additionally each method can take an arbitrary number of options for adding metadata to the flag.

For example:

```go
package main

import (
	"context"

	"github.com/nussjustin/feature"
)

func main() {
	var set feature.FlagSet

	myFeature := set.Bool("my-feature", flag.WithDescription("enables the new feature"))

	if myFeature(context.Background()) {
		println("my-feature enabled") // never runs, see next section
	}
}


```

### Configuring a registry

By default, the values returned for each flag will be the zero value for the specific type.

A [Registry][5] can be used to dynamically generate / fetch values for each flag.

The package currently ships with a single implementation [SimpleStrategy][6]. External implementations are currently
not supported.

Once created, a registry can used by calling the [FlagSet.SetRegistry][7] method.

Example:

```go
package main

import (
	"context"

	"github.com/nussjustin/feature"
)

func main() {
	var set feature.FlagSet

	set.SetStrategy(&feature.SimpleStrategy{
		BoolFunc: func(ctx context.Context, name string) bool {
			return name == "my-feature"
		},
	})

	myFeature := set.Bool("my-feature", flag.WithDescription("enables the new feature"))

	if myFeature(context.Background()) {
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
[2]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.Float
[3]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.Int
[4]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.String
[5]: https://pkg.go.dev/github.com/nussjustin/feature/#Registry
[6]: https://pkg.go.dev/github.com/nussjustin/feature/#SimpleStrategy
[7]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.SetRegistry
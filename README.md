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
* [FlagSet.Uint][5] for uint flags.

Each method will return a callback that takes a `context.Context` and returns a value of the specific type.

Additionally, each method can take an arbitrary number of options for adding metadata to the flag.

For example:

```go
package main

import (
	"context"

	"github.com/nussjustin/feature"
)

func main() {
	var set feature.FlagSet

	myFeature := set.Bool("my-feature", false, "some new feature")

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

	myFeature := set.Bool("my-feature", false, "some new feature")

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
[2]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.Float
[3]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.Int
[4]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.String
[5]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.Uint
[6]: https://pkg.go.dev/github.com/nussjustin/feature/#FlagSet.Context

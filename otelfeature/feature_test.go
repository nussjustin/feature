package otelfeature_test

import (
	"github.com/nussjustin/feature"
	"github.com/nussjustin/feature/otelfeature"
)

func ExampleTracer() {
	tracer, err := otelfeature.Tracer(nil)
	if err != nil {
		// Handle the error
	}
	feature.SetTracer(tracer)
}

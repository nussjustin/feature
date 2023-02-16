package feature_test

import (
	"context"

	"github.com/nussjustin/feature"
)

// For use in examples
type Post struct{}

// For use in examples
var myCtx = context.Background()

// For use in examples
var postId int

// For use in examples
func loadPost(context.Context, int) (Post, error)          { return Post{}, nil }
func loadPostOptimized(context.Context, int) (Post, error) { return Post{}, nil }

// For use in examples
var testerStrategy feature.Strategy

// For use in examples
func readFlags(string) map[string]feature.Strategy {
	return nil
}

func IsTester(context.Context) bool {
	return false
}

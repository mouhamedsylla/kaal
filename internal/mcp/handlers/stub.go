package handlers

import (
	"context"
	"fmt"
)

// Stub is a placeholder handler for tools not yet implemented.
// Replace each stub with a real implementation as features are built.
func Stub(name string) func(context.Context, map[string]any) (any, error) {
	return func(_ context.Context, _ map[string]any) (any, error) {
		return nil, fmt.Errorf("%s: not yet implemented", name)
	}
}

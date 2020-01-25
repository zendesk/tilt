package starkit

import (
	"go.starlark.net/starlark"
)

// LoadHelper allows an Extension to intercept a load to set the contents based on the requested path.

type LoadHelper interface {
	// LocalPath returns the path that the Tiltfile code should be read from.
	// Must be stable, because it's used as a cache key
	// Ensure the content is present in the path returned
	LocalPath(t *starlark.Thread, path string) (string, error)
}

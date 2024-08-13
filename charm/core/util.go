package core

import "go.starlark.net/starlark"

// StringsToStarlarkList converts a slice of strings to a Starlark list.
func StringsToStarlarkList(values []string) *starlark.List {
	result := make([]starlark.Value, len(values))
	for i, v := range values {
		result[i] = starlark.String(v)
	}
	return starlark.NewList(result)
}

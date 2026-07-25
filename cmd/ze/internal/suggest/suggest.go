// Design: docs/architecture/core-design.md — CLI command suggestions
//
// Package suggest is a thin compatibility shim over internal/core/suggest.
//
// The "did you mean?" helper moved to internal/core so that command owners
// under internal/component and internal/plugins can offer suggestions without
// importing anything beneath cmd/ze. This shim re-exports the API at the old
// import path so existing cmd/ze callers keep compiling; new code should import
// github.com/ze-software/ze/internal/core/suggest directly.
package suggest

import core "github.com/ze-software/ze/internal/core/suggest"

// Command returns the closest match from candidates for the given input, or ""
// if no candidate is close enough.
func Command(input string, candidates []string) string {
	return core.Command(input, candidates)
}

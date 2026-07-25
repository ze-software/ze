// Package helpfmt is a thin compatibility shim over internal/core/helpfmt.
//
// The help-formatting helpers moved to internal/core so that command owners
// under internal/component and internal/plugins can render help without
// importing anything beneath cmd/ze. This shim re-exports the API at the old
// import path so existing cmd/ze callers keep compiling; new code should import
// github.com/ze-software/ze/internal/core/helpfmt directly.
//
// Design: docs/architecture/core-design.md — CLI help formatting
package helpfmt

import (
	"io"

	core "github.com/ze-software/ze/internal/core/helpfmt"
)

// Re-exported types. Aliases preserve the Page methods (Write, WriteTo).
type (
	// Page is a structured help page for a CLI command.
	Page = core.Page
	// HelpSection is a named group of entries in a help page.
	HelpSection = core.HelpSection
	// HelpEntry is a single command, flag, or option in a help section.
	HelpEntry = core.HelpEntry
	// RenderWriter is the shared error-capturing writer for CLI render paths.
	RenderWriter = core.RenderWriter
)

// NewRenderWriter returns a RenderWriter over w.
func NewRenderWriter(w io.Writer) *RenderWriter {
	return core.NewRenderWriter(w)
}

// WriteError writes a colored error message to w.
func WriteError(w io.Writer, color bool, format string, a ...any) {
	core.WriteError(w, color, format, a...)
}

// WriteHint writes a colored hint message to w.
func WriteHint(w io.Writer, color bool, format string, a ...any) {
	core.WriteHint(w, color, format, a...)
}

// Summary returns the first sentence of a description.
func Summary(s string) string {
	return core.Summary(s)
}

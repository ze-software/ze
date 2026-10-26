// Design: docs/contributing/spec-workflow.md -- the roadmap Profiles column.
// Related: rfccorpus.go -- Read, the reader registered here.

// Package rfccorpus reads one revision's RFC corpus for the roadmap's
// Profiles column, through the parsers of internal/le/rfc. It is a separate
// package so the roadmap's own tests do not link internal/le/rfc, whose
// dependencies register actions in most le namespaces.
package rfccorpus

import "github.com/ze-software/ze/internal/le/spec/roadmap"

func init() {
	roadmap.RegisterCorpusReader(Read)
}

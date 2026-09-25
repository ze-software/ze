// Design: docs/guide/mrt-analysis.md -- the MRT analysis commands
// Related: register.go -- the registration of this command

// Package mrt is `le mrt`, the MRT analysis commands of internal/analyze.
package mrt

import (
	"github.com/ze-software/ze/internal/analyze"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// name is the command as a developer types it after `le`.
const name = "mrt"

// Answer runs one analyze subcommand, or lists them all when args is empty.
//
// A subcommand writes its own output and answers its own exit code, so the
// payload is nil for it: the output of `mrt show` is a record dump, not a value
// for a pipe operator to render. The bare word answers the subcommand registry
// as data, because that listing is the one answer here that is a value.
func Answer(args []string) (any, int) {
	if len(args) == 0 {
		return analyze.Targets(), 0
	}
	return nil, analyze.Dispatch(args)
}

// subcommands answers the subcommand names in the form help renders as the
// command's actions, derived from the analyze registry rather than listed here.
func subcommands() string {
	var buffer textbuf.Buffer
	for i, target := range analyze.Targets() {
		if i > 0 {
			buffer.Str(" | ")
		}
		buffer.Str(target.Name)
	}
	return buffer.String()
}

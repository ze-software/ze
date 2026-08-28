// Design: docs/architecture/testing/verify-freshness-scope.md -- what a scoped run covers
// Overview: changed.go -- the other question this area answers
//
// scope.go publishes the selector through the changed area's structured action
// surface and lets a verify run reuse one precomputed package answer.
//
// EVERY failure route that can safely continue WIDENS. A selector refusal must
// not mean "no package to verify." An EMPTY answer is not a widening. It means
// that no changed path is compiled by a Go package.

package changed

import (
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
)

// ScopeFileKey is the dot-notation spelling of ZE_VERIFY_SCOPE_PACKAGES. A
// verify run computes the change set ONCE before the first stage. It publishes
// the resulting filename here. Thus, every scoped stage uses one tree and pays
// for one `go list`.
const ScopeFileKey = "ze.verify.scope.packages"

var scopeFileEntry = env.MustRegister(env.EnvEntry{
	Key:         ScopeFileKey,
	Type:        "string",
	Default:     "",
	Description: "file holding this verify run's precomputed changed-package list",
	// Private keeps the key out of `ze env list`: it is a build-host variable,
	// and a tool imported into ze must not advertise one to an operator.
	Private: true,
})

// ScopeReport is the selector's structured answer.
//
// Print controls only the plain-text rendering. Packages and Tags both remain
// available to JSON and YAML consumers, including in the legacy tags-only mode.
type ScopeReport struct {
	Packages []string `json:"packages"`
	Tags     []string `json:"tags,omitempty"`
	Print    string   `json:"print,omitempty"`
	Widened  bool     `json:"widened"`
	Reason   string   `json:"reason,omitempty"`
}

// Text preserves the deleted producer's three print modes exactly.
func (r ScopeReport) Text() string {
	switch printMode(r.Print) {
	case printTags:
		return lineText(r.Tags)
	case printBoth:
		var tb textbuf.Buffer
		return tb.Str("# packages\n").Str(lineText(r.Packages)).
			Str("# tags\n").Str(lineText(r.Tags)).String()
	default:
		return lineText(r.Packages)
	}
}

func lineText(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	var tb textbuf.Buffer
	for _, line := range lines {
		tb.Str(line).Byte('\n')
	}
	return tb.String()
}

// Scope resolves the change set for one checkout.
type Scope struct {
	// Root is the checkout the answer is about.
	Root string
	// File is the precomputed package answer this verify run published, or empty
	// when there is no run. The zero value reads the native selector.
	File string
}

// newScope reads the scope for the checkout at root, honoring the file a verify
// run published.
func newScope(root string) Scope {
	return Scope{Root: root, File: env.Get(scopeFileEntry.Key)}
}

// Resolve answers the native selector's change set and exit code.
//
// args use the deleted producer's flag grammar. The le action translates its
// closed keywords to these arguments at its boundary. A precomputed package
// answer applies only to the argument-free package query.
func (s Scope) Resolve(args []string) (ScopeReport, int) {
	if len(args) == 0 && s.File != "" {
		return s.fromFile(), 0
	}
	return s.resolveSelector(args)
}

// fromFile hands back the answer a verify run already computed.
//
// The read is the guard. The shell half tested the path with `[ -r ]` but
// ignored `cat`'s exit status. A readable directory therefore returned nothing
// and exited 0.
func (s Scope) fromFile() ScopeReport {
	body, err := os.ReadFile(s.File) //nolint:gosec // the path is this run's own published artifact
	if err != nil {
		var tb textbuf.Buffer
		return widen(tb.Str("the precomputed package list at ").Str(s.File).
			Str(" could not be read: ").Err(err).String())
	}
	return ScopeReport{Packages: lines(body)}
}

// widen answers for every route that fails to resolve a precomputed package
// set. The reason goes to stderr and the structured payload.
func widen(reason string) ScopeReport {
	var tb textbuf.Buffer
	gaterun.Note(tb.Str("changed: ").Str(reason).Str(", so every package is selected").String())
	return ScopeReport{
		Packages: []string{everyPackage},
		Widened:  true,
		Reason:   reason,
	}
}

// lines splits a command's answer into the non-empty lines it holds, in the
// order it gave them. The selector sorts its own answer, and re-sorting here
// would hide a producer that stopped.
func lines(body []byte) []string {
	var out []string
	for line := range strings.SplitSeq(string(body), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

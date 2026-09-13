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
	"fmt"
	"os"
	"path/filepath"
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

// ScopeTagsKey is the dot-notation spelling of ZE_VERIFY_SCOPE_TAGS: the file
// holding this run's feature-tag answer. It sits beside ScopeFileKey because
// the two are one contract, published by one run from one walk of one graph:
// this package produces both answers, so this package names both files.
//
// Its consumer is the Staticcheck feature matrix, which subtracts the rows the
// answer cannot move. Unset means every row is judged, which is what a
// standalone gate invocation gets.
const ScopeTagsKey = "ze.verify.scope.tags"

var _ = env.MustRegister(env.EnvEntry{
	Key:         ScopeTagsKey,
	Type:        "string",
	Default:     "",
	Description: "the file naming the feature tags this verify run's change set reaches; unset judges every matrix row",
	// Private keeps the key out of `ze env list`. It is a build-host path the
	// verify runner owns, and an operator has nothing to do with it.
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

// scopeRootHeader opens the package answer with the checkout it is about. A
// verify run names its answer to every child process it starts, and a child
// asking about a DIFFERENT checkout must get its own answer rather than this
// one: `go test` is such a child, and a unit test driving the selector over a
// fixture directory was handed the gate's own package list, which made the
// fixture's verdict depend on whether a verify run was in progress.
const scopeRootHeader = "root "

// WriteScopePackages writes the package answer one verify run publishes for one
// checkout: the root it was selected for on the first line, then one
// ./-prefixed package on each line after it.
//
// The writer lives beside fromFile, which reads it back, so the format is
// declared once. A verify run, the functional suite fixture and the unit tests
// all publish through here.
func WriteScopePackages(path, root string, packages []string) error {
	var body textbuf.Buffer
	body.Str(scopeRootHeader).Str(root).Byte('\n')
	for _, name := range packages {
		body.Str(name).Byte('\n')
	}
	if err := os.WriteFile(path, body.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
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

// Packages answers the change-set package selection for the checkout at root,
// honoring the file a verify run published.
//
// It is the entry point for a consumer outside this package. The functional
// stage's suite selection reads it, and `le changed packages` answers with it,
// so a stage and an operator asking what that stage will do cannot hold two
// change sets that disagree.
func Packages(root string) (ScopeReport, int) { return newScope(root).Resolve(nil) }

// Resolve answers the native selector's change set and exit code.
//
// args use the deleted producer's flag grammar. The le action translates its
// closed keywords to these arguments at its boundary. A precomputed package
// answer applies only to the argument-free package query.
func (s Scope) Resolve(args []string) (ScopeReport, int) {
	if len(args) == 0 && s.File != "" {
		// A published answer that is about another checkout answers nothing
		// here, so this question falls through to the selector, exactly as it
		// would with no answer published at all.
		if report, mine := s.fromFile(); mine {
			return report, 0
		}
	}
	return s.resolveSelector(args)
}

// fromFile hands back the answer a verify run already computed, and says
// whether that answer is about THIS checkout. A false second result means the
// answer belongs to another checkout and this caller must select its own.
//
// The read is the guard. The shell half tested the path with `[ -r ]` but
// ignored `cat`'s exit status. A readable directory therefore returned nothing
// and exited 0.
func (s Scope) fromFile() (ScopeReport, bool) {
	body, err := os.ReadFile(s.File) //nolint:gosec // the path is this run's own published artifact
	if err != nil {
		var tb textbuf.Buffer
		return widen(tb.Str("the precomputed package list at ").Str(s.File).
			Str(" could not be read: ").Err(err).String()), true
	}
	recorded := lines(body)
	if len(recorded) == 0 {
		var tb textbuf.Buffer
		return widen(tb.Str("the precomputed package list at ").Str(s.File).
			Str(" is empty, so it names no checkout and cannot be matched to one").String()), true
	}
	checkout, named := strings.CutPrefix(recorded[0], scopeRootHeader)
	if !named {
		var tb textbuf.Buffer
		return widen(tb.Str("the precomputed package list at ").Str(s.File).
			Str(" names no checkout on its first line, so it cannot be matched to one").String()), true
	}
	if !sameCheckout(checkout, s.Root) {
		return ScopeReport{}, false
	}
	return ScopeReport{Packages: recorded[1:]}, true
}

// sameCheckout answers whether two paths name one directory. Symbolic links are
// resolved because the publisher and the reader reach the checkout by different
// routes: a stage runs with the worktree as its working directory and a caller
// can hold the path it was given, and on macOS one of the two can arrive with
// /private in front of it. A path that will not resolve is compared as it
// stands, which is the answer for a directory that no longer exists.
func sameCheckout(recorded, asked string) bool {
	return resolvedPath(recorded) == resolvedPath(asked)
}

func resolvedPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return filepath.Clean(path)
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

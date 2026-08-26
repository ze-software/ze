// Design: docs/architecture/testing/verify-freshness-scope.md -- what a scoped run covers
// Overview: changed.go -- the other question this area answers
//
// scope.go answers which Go packages a scoped verify stage must cover.
//
// IT HOLDS NO SELECTION LOGIC. Exactly one program computes the change set:
// scripts/checks/verify_scope_selector.go. It uses a tag-aware reverse import
// graph and a classification table for non-Go file kinds. This file dispatches
// between two ways to reach that answer. A verify run can supply a precomputed
// file, and an independent caller can start a fresh selector run.
//
// EVERY failure route WIDENS. A selector refusal must not mean "no package to
// verify." That result would let the scoped gate cover nothing and report
// success (ai/rules/evidence.md -- a zero value must never be a valid-looking
// answer). An EMPTY answer is not a widening. It means that no changed path is
// compiled by a Go package.

package changed

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/letools/gaterun"
)

// everyPackage is the widest answer. `go test`, `go build` and `golangci-lint`
// all accept it, and it is the same word the selector itself widens with.
const everyPackage = "./..."

// selectorPath names the only change-set producer relative to the checkout. It
// remains a `//go:build ignore` program under scripts/, so this command starts
// it instead of calling it. Porting the selector is a separate migration step
// (plan/spec-le-is-a-ze-binary.md, amendment A).
const selectorPath = "scripts/checks/verify_scope_selector.go"

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

// ScopeReport is what `le changed packages` answers.
//
// The payload includes Widened and Reason because Packages alone does not show
// the verdict. `./...` is a legitimate narrow answer when a change reaches
// everything. It is a widening when the selector fails. Both cases look the
// same on stdout, so a reader needs the other fields to distinguish them.
type ScopeReport struct {
	Packages []string `json:"packages"`
	Widened  bool     `json:"widened"`
	Reason   string   `json:"reason,omitempty"`
}

// Text names one package per line, which is what every scoped recipe consumes.
// An empty answer renders the empty string rather than a blank line: a caller
// reading `$(...)` cannot tell a blank line from a package name.
func (r ScopeReport) Text() string {
	if len(r.Packages) == 0 {
		return ""
	}
	var tb textbuf.Buffer
	for _, pkg := range r.Packages {
		tb.Str(pkg).Byte('\n')
	}
	return tb.String()
}

// Scope resolves the changed-package set for one checkout.
type Scope struct {
	// Root is the checkout the answer is about.
	Root string
	// File is the precomputed answer this verify run published, or empty when
	// there is no run. The zero value reads the environment.
	File string
	// Run runs one command. The zero value means RunCommand.
	Run Run
}

// NewScope reads the scope for the checkout at root, honoring the file a verify
// run published.
func NewScope(root string) Scope {
	return Scope{Root: root, File: env.Get(scopeFileEntry.Key)}
}

// run answers through the scope's command runner, defaulted.
func (s Scope) run(dir string, argv []string) (string, error) {
	if s.Run == nil {
		return RunCommand(dir, argv)
	}
	return s.Run(dir, argv)
}

// Resolve answers the packages a scoped stage must cover.
//
// args are the selector's own arguments (--depth=, --paths-from=, --drop-log=).
// An argument asks a different question than the one the run precomputed, so
// the precomputed answer is taken only when there is none.
func (s Scope) Resolve(args []string) ScopeReport {
	if len(args) == 0 && s.File != "" {
		return s.fromFile()
	}
	return s.fromSelector(args)
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

// fromSelector runs the one producer of the change set.
func (s Scope) fromSelector(args []string) ScopeReport {
	out, err := s.run(s.Root, selectorArgv(s.Root, args))
	if err != nil {
		var tb textbuf.Buffer
		return widen(tb.Str("the selector could not answer: ").Err(err).String())
	}
	return ScopeReport{Packages: lines([]byte(out))}
}

// selectorArgv is the command line that reaches the selector.
//
// The path is absolute. Thus, a caller can run this command from any directory,
// and the answer still describes the selected checkout.
//
// CGO_ENABLED travels as an `env` prefix instead of this process's environment.
// A command's environment is part of the command. A process-wide setting would
// affect every other command that this binary starts.
func selectorArgv(root string, args []string) []string {
	argv := make([]string, 0, len(args)+6)
	argv = append(argv, "env", "CGO_ENABLED=0", "go", "run",
		filepath.Join(root, selectorPath), "--print=packages")
	return append(argv, args...)
}

// widen answers for every route that fails to resolve a change set.
//
// The reason goes to stderr and the payload, matching the shell half. stdout is
// the package list that a recipe consumes. A sentence there would become a
// package name.
func widen(reason string) ScopeReport {
	var tb textbuf.Buffer
	gaterun.Note(tb.Str("changed: ").Str(reason).Str(", so every package is selected").String())
	return ScopeReport{Packages: []string{everyPackage}, Widened: true, Reason: reason}
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

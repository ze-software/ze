// Design: docs/architecture/core-design.md -- how a program dispatches le's commands
// Related: dispatch.go -- the loop that rewrites a retired name before it resolves
//
// This file is the ONE declaration of every name the subject-first rename
// retires (plan/spec-le-subject-first-command-tree.md): the le commands, the
// programs folded into le, the build tags, the harness file names and the
// harness variables. Dispatch reads it to run a retired command as its new
// one, and `le doc check retired-commands` reads it to find the callers that
// still name an old form. No other surface lists an old name.
//
// The rewrite is a time-bounded exception to ai/rules/no-layering.md, approved
// by the owner on 2026-09-24: peer sessions call the old names while the rename
// lands. Phase 3 of that spec deletes the rewrite, and the sweep becomes a gate.

package leroot

import (
	"os"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Rename is one retired le command, or one retired action of a command, and
// the command that answers it now.
//
// A row is a word sequence: Retired then RetiredAction is what a caller typed,
// and Command then Action is what runs instead, followed by the caller's own
// remaining words. One shape covers a rename (`test-unit` to `test unit`), a
// merge (`verify lock` to `job`) and a verb split (`build-artifacts
// installer-arm64` to `build installer arm64`).
type Rename struct {
	// Retired is the retired command, as it was registered: `docs-to-code`.
	Retired string `json:"retired"`
	// RetiredAction is the action of Retired this row covers. It is empty
	// when the row covers the whole command and every action carries on.
	RetiredAction string `json:"retired-action,omitempty"`
	// Command is the registered command that answers the row now.
	Command string `json:"command"`
	// Action is the action of Command placed before the caller's own words.
	// It is empty when the caller's own action carries on unchanged.
	Action string `json:"action,omitempty"`
}

// Old answers the words a caller typed, one per element.
func (r Rename) Old() []string {
	words := strings.Fields(r.Retired)
	return append(words, strings.Fields(r.RetiredAction)...)
}

// New answers the words that replace Old, one per element.
func (r Rename) New() []string {
	words := strings.Fields(r.Command)
	return append(words, strings.Fields(r.Action)...)
}

// renames is the command half of the rename map. Dispatch reads it on every
// invocation, so a test that swaps it MUST restore it before it returns.
//
//nolint:goconst // a row reads whole: a constant per repeated word hides which command a row names
var renames = []Rename{
	{Retired: "verify lock", Command: "job"},
	{Retired: "spec session", Command: "spec"},
	{Retired: "journal", Command: "spec journal"},
	{Retired: "evidence", Command: "verify evidence"},
	{Retired: "go-extract", Command: "go extract"},
	{Retired: "module", Command: "go module"},
	{Retired: "go-version", Command: "go version-pin"},
	{Retired: "verify lint", Command: "go lint"},
	{Retired: "platform-vet", Command: "go vet-platforms"},
	{Retired: "staticcheck-feature-matrix", Command: "go staticcheck"},
	{Retired: "repository", Command: "repo"},
	{Retired: "repository tracked-build", Command: "repo tracked-build"},
	{Retired: "tracked", Command: "repo tracked-le"},
	{Retired: "changed", Command: "repo changed"},
	{Retired: "working-tree", Command: "repo working-tree"},
	{Retired: "inventory", Command: "repo inventory"},
	{Retired: "arch-map", Command: "repo arch-map"},
	{Retired: "discovery-index", Command: "repo package-map"},
	{Retired: "feature-tags", Command: "repo feature-tags"},
	{Retired: "source-rewrite", Command: "repo rewrite"},
	{Retired: "tier", Command: "arch tier"},
	{Retired: "enumeration", Command: "arch enumeration"},
	{Retired: "fs-persistence", Command: "arch fs-persistence"},
	{Retired: "iface-resolution", Command: "arch iface-resolution"},
	{Retired: "cli-grammar", Command: "cli grammar"},
	{Retired: "ci-dispatch", Command: "cli dispatch"},
	{Retired: "dash-stdio", Command: "cli stdio"},
	{Retired: "command ownership", Command: "cli ownership"},
	{Retired: "command list", Command: "cli list"},
	{Retired: "wiki-catalog", Command: "cli catalog"},
	{Retired: "port-defaults", Command: "config ports"},
	{Retired: "yang leaf-mentions", Command: "config unread-leaves"},
	{Retired: "docs-to-code", Command: "doc index"},
	{Retired: "docs-to-code", RetiredAction: "check", Command: "doc index", Action: "check"},
	{Retired: "docs-to-code", RetiredAction: "index-check", Command: "doc index", Action: "check"},
	{Retired: "docs-to-code", RetiredAction: "update", Command: "doc index", Action: "write"},
	{Retired: "docs-to-code", RetiredAction: "index-update", Command: "doc index", Action: "write"},
	{Retired: "docvalid", Command: "doc yang-contract"},
	{Retired: "consistency", Command: "doc consistency"},
	{Retired: "ste", Command: "doc ste"},
	{Retired: "terminal-demo", Command: "site terminal-demo"},
	{Retired: "web-assets", Command: "web assets"},
	{Retired: "vendor-web", Command: "web vendor"},
	{Retired: "htmx-upgrade", Command: "web htmx"},
	{Retired: "ai", RetiredAction: "skills-sync", Command: "ai sync", Action: "write"},
	{Retired: "ai", RetiredAction: "sync-check", Command: "ai sync", Action: "check"},
	{Retired: "ai", RetiredAction: "sync-preview", Command: "ai sync", Action: "preview"},
	{Retired: "rules", Command: "ai rules"},
	{Retired: "hook-check", Command: "ai hooks"},
	{Retired: "digest", Command: "ai digest"},
	{Retired: "token-economy", Command: "ai tokens"},
	{Retired: "protocol-skeleton", Command: "rfc skeletons"},
	{Retired: "test-unit", Command: "test unit"},
	{Retired: "functional", Command: "test functional"},
	{Retired: "integration", Command: "test integration"},
	{Retired: "deployment", Command: "test deployment"},
	{Retired: "qemu", Command: "test qemu"},
	{Retired: "fuzz", Command: "test fuzz"},
	{Retired: "stress-repro", Command: "test stress-repro"},
	{Retired: "test-helper", Command: "test fixture"},
	{Retired: "netlab", Command: "test netlab"},
	{Retired: "mutation", Command: "test mutation"},
	{Retired: "test-health", Command: "test health"},
	{Retired: "test-sensitivity", Command: "test sensitivity"},
	{Retired: "test-weakened", Command: "test weakened"},
	{Retired: "test-chaos", Command: "chaos selftest"},
	{Retired: "perf-bench", Command: "perf"},
	{Retired: "perf-bench", RetiredAction: "suggestion-report", Command: "perf", Action: "suggest"},
	{Retired: "build-artifacts", RetiredAction: "host", Command: "build host-driver"},
	{Retired: "build-artifacts", RetiredAction: "installer-amd64", Command: "build installer", Action: "amd64"},
	{Retired: "build-artifacts", RetiredAction: "installer-arm64", Command: "build installer", Action: "arm64"},
	{Retired: "gokrazy-gosum", Command: "build gosum"},
	{Retired: "iana-asn", Command: "data asn-delegation"},
}

// Renames answers a copy of the command half of the rename map, in
// declaration order.
func Renames() []Rename {
	return slices.Clone(renames)
}

// RetiredKind says what sort of name a Retirement retires. The zero value is
// no kind, so a row that names none is visibly incomplete.
type RetiredKind uint8

const (
	RetiredKindUnspecified RetiredKind = iota
	// RetiredProgram is a developer program folded into le: `ze-chaos`.
	RetiredProgram
	// RetiredTag is a Go build tag: `ze_chaos`.
	RetiredTag
	// RetiredFile is a file name a builder writes or a container carries.
	RetiredFile
	// RetiredVariable is a registered environment key, in its dotted form.
	RetiredVariable
)

// String names the kind for a report.
func (k RetiredKind) String() string {
	switch k {
	case RetiredProgram:
		return "program"
	case RetiredTag:
		return "build-tag"
	case RetiredFile:
		return "file"
	case RetiredVariable:
		return "variable"
	case RetiredKindUnspecified:
		return "unspecified"
	}
	return "unspecified"
}

// Retirement is one retired name that is not an le command. Replacement is
// what a caller writes instead, for the report to print. It is text for a
// person, never parsed.
type Retirement struct {
	Kind        RetiredKind `json:"-"`
	Old         string      `json:"old"`
	Replacement string      `json:"replacement"`
}

// retirements is the non-command half of the rename map.
//
//nolint:goconst // a row reads whole: a constant per repeated word hides which name a row retires
var retirements = []Retirement{
	{Kind: RetiredProgram, Old: "ze-test", Replacement: "le test harness"},
	{Kind: RetiredProgram, Old: "ze-chaos", Replacement: "le chaos run"},
	{Kind: RetiredProgram, Old: "ze-perf", Replacement: "le perf send | report | track"},
	{Kind: RetiredProgram, Old: "ze-perf-run", Replacement: "le perf run"},
	{Kind: RetiredProgram, Old: "ze-analyze", Replacement: "le mrt"},
	{Kind: RetiredProgram, Old: "ze-gok", Replacement: "le build gokrazy"},
	{Kind: RetiredProgram, Old: "ze-terminal-pty", Replacement: "le site terminal-demo pty"},
	{Kind: RetiredTag, Old: "ze_test", Replacement: "le_test"},
	{Kind: RetiredTag, Old: "ze_chaos", Replacement: "none: le chaos run"},
	{Kind: RetiredTag, Old: "ze_analyze", Replacement: "none: le mrt"},
	{Kind: RetiredTag, Old: "ze_perf", Replacement: "none: le perf"},
	{Kind: RetiredFile, Old: "bin/ze-test", Replacement: "bin/le-test"},
	{Kind: RetiredFile, Old: "bin/ze-test-linux-", Replacement: "bin/le-test-linux-"},
	{Kind: RetiredFile, Old: "test/interop/ze-test-linux", Replacement: "test/interop/le-test-linux"},
	{Kind: RetiredFile, Old: "/usr/local/bin/ze-test", Replacement: "/usr/local/bin/le-test"},
	{Kind: RetiredFile, Old: "bin/ze-perf", Replacement: "none: a linux le at /usr/local/bin/le"},
	{Kind: RetiredFile, Old: "bin/ze-perf-linux", Replacement: "none: a linux le at /usr/local/bin/le"},
	{Kind: RetiredVariable, Old: "ze.perf.bin", Replacement: "none: the perf runner builds le"},
	{Kind: RetiredVariable, Old: "ze.test.bin", Replacement: "le.test.bin"},
	{Kind: RetiredVariable, Old: "ze.qemu.test.bin", Replacement: "le.qemu.test.bin"},
	{Kind: RetiredVariable, Old: "ze.test.no.build", Replacement: "le.test.no.build"},
}

// Retirements answers a copy of the non-command half of the rename map.
func Retirements() []Retirement {
	return slices.Clone(retirements)
}

// retiredRewrite answers the argv a retired command now runs as, and the row
// that rewrote it. It answers false when argv names no retired command, or
// when the command that replaces it is not registered yet: until a family
// moves, its old name still runs its own handler, so a peer session calling
// it keeps working.
//
// The longest matching row wins, so `docs-to-code check` takes its own row
// over the row for `docs-to-code`.
func retiredRewrite(args []string) ([]string, Rename, bool) {
	own, _ := splitChain(args)
	var best Rename
	bestWords := 0
	for _, row := range renames {
		old := row.Old()
		if len(old) <= bestWords {
			continue
		}
		if len(own) < len(old) {
			continue
		}
		if !slices.Equal(own[:len(old)], old) {
			continue
		}
		best = row
		bestWords = len(old)
	}
	if bestWords == 0 {
		return nil, Rename{}, false
	}
	if LookupCommand(best.Command) == nil {
		return nil, Rename{}, false
	}

	fresh := best.New()
	rewritten := make([]string, 0, len(fresh)+len(args)-bestWords)
	rewritten = append(rewritten, fresh...)
	rewritten = append(rewritten, args[bestWords:]...)
	return rewritten, best, true
}

// noteRetired writes the one stderr line a retired name owes: the words the
// caller typed and the words that replace them. Phase 2 of the rename finds
// callers a text search cannot see by this line, in CI logs and test output.
func noteRetired(program string, row Rename) {
	var tb textbuf.Buffer
	tb.Str("warning: ").Str(program).Byte(' ').Join(row.Old(), " ").
		Str(" is renamed: run ").Str(program).Byte(' ').Join(row.New(), " ").Byte('\n')
	os.Stderr.WriteString(tb.String()) //nolint:errcheck // CLI output
}

// Design: docs/architecture/core-design.md -- native documentation verifier actions
// Related: retired.go -- the sweep that reads this map and gates on what it finds
//
// This file is the ONE declaration of every name the subject-first rename
// retired (spec-le-subject-first-command-tree): the le commands, the
// programs folded into le, the build tags, the harness file names and the
// harness variables. `le doc check retired-commands` reads it to find the
// callers that still name an old form. No other surface lists an old name.
//
// Dispatch does not read it. The old names ran as their new commands while
// the rename landed, and Phase 3 deleted that rewrite, so an old name answers
// `unknown command` like any other word le does not register.

package doccheck

import (
	"slices"
	"strings"
)

// Rename is one retired le command, or one retired action of a command, and
// the command that answers it now.
//
// A row is a word sequence: Retired then RetiredAction is what a caller
// typed, and Command then Action is what a caller types now, followed by the
// same remaining words. One shape covers a rename (`test-unit` to `test unit`), a
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

// renames is the command half of the rename map.
//
//nolint:goconst // a row reads whole: a constant per repeated word hides which command a row names
var renames = []Rename{
	{Retired: "verify lock", Command: "job"},
	// A bare `spec session` answered the claimed spec, so it runs `spec current`;
	// every action row below is longer, so it wins over this one.
	{Retired: "spec session", Command: "spec current"},
	{Retired: "spec session", RetiredAction: "claim", Command: "spec claim"},
	{Retired: "spec session", RetiredAction: "current", Command: "spec current"},
	{Retired: "spec session", RetiredAction: "model", Command: "spec model"},
	{Retired: "spec session", RetiredAction: "release", Command: "spec release"},
	{Retired: "spec session", RetiredAction: "review", Command: "spec review"},
	{Retired: "spec session", RetiredAction: "state", Command: "spec state"},
	{Retired: "spec session", RetiredAction: "wip", Command: "spec wip"},
	{Retired: "journal", Command: "spec journal"},
	{Retired: "evidence", Command: "verify evidence"},
	{Retired: "go-extract", Command: "go extract"},
	{Retired: "module", Command: "go module"},
	{Retired: "go-version", Command: "go version-pin"},
	{Retired: "verify lint", Command: "go lint"},
	{Retired: "platform-vet", Command: "go vet-platforms"},
	{Retired: "staticcheck-feature-matrix", Command: "go staticcheck"},
	{Retired: "repository", Command: "repo"},
	{Retired: "repository tracked-build", Command: "repo compiles"},
	{Retired: "repo tracked-build", Command: "repo compiles"},
	{Retired: "tracked", Command: "repo bootstraps"},
	{Retired: "repo tracked-le", Command: "repo bootstraps"},
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
	// Every harness command is a member of `test` (D-8), so the row names the
	// namespace: `test harness peer x` runs `test peer x`.
	{Retired: "test harness", Command: "test"},
	{Retired: "netlab", Command: "test netlab"},
	{Retired: "mutation", Command: "test mutation"},
	{Retired: "test-health", Command: "test health"},
	{Retired: "test-sensitivity", Command: "test sensitivity"},
	{Retired: "test-weakened", Command: "test weakened"},
	// The hyphenated harness names whose left word was another test member
	// (`le cli grammar` R9): the wire suites and the scale test became areas
	// whose first word picks the member. Each is listed under `test` and under
	// the retired `test harness` namespace, which cannot resolve them itself.
	{Retired: "test isis-wire", Command: "test wire", Action: "isis"},
	{Retired: "test ospf-wire", Command: "test wire", Action: "ospf"},
	{Retired: "test l2tp-wire", Command: "test wire", Action: "l2tp"},
	{Retired: "test l2tp-scale", Command: "test scale", Action: "l2tp"},
	{Retired: "test static-http", Command: "test httpd"},
	{Retired: "test vpp-stub", Command: "test vpp", Action: "stub"},
	{Retired: "test harness isis-wire", Command: "test wire", Action: "isis"},
	{Retired: "test harness ospf-wire", Command: "test wire", Action: "ospf"},
	{Retired: "test harness l2tp-wire", Command: "test wire", Action: "l2tp"},
	{Retired: "test harness l2tp-scale", Command: "test scale", Action: "l2tp"},
	{Retired: "test harness static-http", Command: "test httpd"},
	{Retired: "test harness vpp-stub", Command: "test vpp", Action: "stub"},
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
//
// ProgramPositionOnly marks a program name that is also an ordinary fixture
// spelling: `ze-test` is a hostname, a NAS id, a module name and a wire method
// in tests that have nothing to do with the harness. The sweep then matches the
// name only where it is run or shipped as a program, never as a word in text.
type Retirement struct {
	Kind                RetiredKind `json:"-"`
	Old                 string      `json:"old"`
	Replacement         string      `json:"replacement"`
	ProgramPositionOnly bool        `json:"-"`
}

// retirements is the non-command half of the rename map.
//
//nolint:goconst // a row reads whole: a constant per repeated word hides which name a row retires
var retirements = []Retirement{
	{Kind: RetiredProgram, Old: "ze-test", Replacement: "le test", ProgramPositionOnly: true},
	{Kind: RetiredProgram, Old: "le-test", Replacement: "le test"},
	// ze-peer is retired as a runner exec head only: the word also names peers
	// in configs and prose that have nothing to do with the harness.
	{Kind: RetiredProgram, Old: "ze-peer", Replacement: "le test peer", ProgramPositionOnly: true},
	{Kind: RetiredProgram, Old: "ze-chaos", Replacement: "le chaos run"},
	{Kind: RetiredProgram, Old: "ze-perf", Replacement: "le perf send | report | track"},
	{Kind: RetiredProgram, Old: "ze-perf-run", Replacement: "le perf run"},
	{Kind: RetiredProgram, Old: "ze-analyze", Replacement: "le mrt"},
	{Kind: RetiredProgram, Old: "ze-gok", Replacement: "le build gokrazy"},
	{Kind: RetiredProgram, Old: "ze-terminal-pty", Replacement: "le site terminal-demo pty"},
	{Kind: RetiredTag, Old: "ze_test", Replacement: "none: le carries the harness"},
	{Kind: RetiredTag, Old: "ze_chaos", Replacement: "none: le chaos run"},
	{Kind: RetiredTag, Old: "ze_analyze", Replacement: "none: le mrt"},
	{Kind: RetiredTag, Old: "ze_perf", Replacement: "none: le perf"},
	{Kind: RetiredFile, Old: "bin/ze-test", Replacement: "none: le test <name>"},
	{Kind: RetiredFile, Old: "bin/ze-test-linux-", Replacement: "none: a linux le built by internal/le/linuxle"},
	{Kind: RetiredFile, Old: "test/interop/ze-test-linux", Replacement: "none: a linux le at test/interop/le-linux"},
	{Kind: RetiredFile, Old: "/usr/local/bin/ze-test", Replacement: "none: a linux le at /usr/local/bin/le"},
	{Kind: RetiredFile, Old: "bin/le-test", Replacement: "none: le test <name>"},
	{Kind: RetiredFile, Old: "bin/le-test-linux-", Replacement: "none: a linux le built by internal/le/linuxle"},
	{Kind: RetiredFile, Old: "test/interop/le-test-linux", Replacement: "none: a linux le at test/interop/le-linux"},
	{Kind: RetiredFile, Old: "/usr/local/bin/le-test", Replacement: "none: a linux le at /usr/local/bin/le"},
	{Kind: RetiredFile, Old: "bin/ze-perf", Replacement: "none: a linux le at /usr/local/bin/le"},
	{Kind: RetiredFile, Old: "bin/ze-perf-linux", Replacement: "none: a linux le at /usr/local/bin/le"},
	{Kind: RetiredVariable, Old: "ze.perf.bin", Replacement: "none: the perf runner builds le"},
	{Kind: RetiredVariable, Old: "ze.test.bin", Replacement: "none: the runner runs its own executable"},
	{Kind: RetiredVariable, Old: "le.test.bin", Replacement: "none: the runner runs its own executable"},
	{Kind: RetiredVariable, Old: "ze.test.binary", Replacement: "none: l2tp scale runs its own executable"},
	{Kind: RetiredVariable, Old: "le.test.binary", Replacement: "none: l2tp scale runs its own executable"},
	{Kind: RetiredVariable, Old: "ze.qemu.test.bin", Replacement: "none: the qemu action builds the guest le"},
	{Kind: RetiredVariable, Old: "le.qemu.test.bin", Replacement: "none: the qemu action builds the guest le"},
	{Kind: RetiredVariable, Old: "ze.test.no.build", Replacement: "le.test.no.build"},
}

// Retirements answers a copy of the non-command half of the rename map.
func Retirements() []Retirement {
	return slices.Clone(retirements)
}

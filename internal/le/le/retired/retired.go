// Design: docs/architecture/core-design.md -- the rename map, read by the sweep and by dispatch
// Related: ../root/dispatch.go -- refuses a typed retired spelling, naming its successor
// Related: ../../doc/check/retired.go -- the sweep that finds callers still naming an old form
//
// Package leretired is the ONE declaration of every name le retired: the le
// commands, the programs folded into le, the build tags, the harness file names
// and the harness variables. Two readers use it and no other surface lists an
// old name. `le doc check retired-commands` scans the tracked files for callers
// that still name an old form, and leroot.Dispatch refuses a typed old spelling
// before any parameter is parsed, naming the command that answers it now
// (D-retired of spec-le-cli-usability).
//
// It is a leaf: it imports nothing from le, so the dispatcher and the
// documentation gate can both read it without either importing the other.
package leretired

import (
	"slices"
	"strconv"
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
	// The area word became the whole command (D-P4-areaword): a value spelled
	// `spec` is the retired verb, since a spec path ends in .md.
	{Retired: "spec claim spec", Command: "spec claim"},
	{Retired: "rollback newest", Command: "rollback"},
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
	{Retired: "repo generate", Command: "repo", Action: "update"},
	{Retired: "repo generated-check", Command: "repo", Action: "check generated"},
	{Retired: "repo tree-check", Command: "repo", Action: "check tree"},
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
	{Retired: "arch tier write-baseline", Command: "arch tier", Action: "update"},
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
	{Retired: "docs-to-code", RetiredAction: "update", Command: "doc index", Action: "update"},
	{Retired: "docs-to-code", RetiredAction: "index-update", Command: "doc index", Action: "update"},
	{Retired: "docvalid", Command: "doc yang-contract"},
	{Retired: "doc yang-contract pipe-operators-update", Command: "doc yang-contract", Action: "update"},
	{Retired: "consistency", Command: "doc consistency"},
	{Retired: "ste", Command: "doc ste"},
	{Retired: "terminal-demo", Command: "site terminal-demo"},
	{Retired: "web-assets", Command: "web assets"},
	{Retired: "vendor-web", Command: "web vendor"},
	{Retired: "htmx-upgrade", Command: "web htmx"},
	{Retired: "ai", RetiredAction: "skills-sync", Command: "ai sync", Action: "update"},
	{Retired: "ai", RetiredAction: "sync-check", Command: "ai sync", Action: "check"},
	{Retired: "ai", RetiredAction: "sync-preview", Command: "ai sync", Action: "preview"},
	{Retired: "rules", Command: "ai rules"},
	{Retired: "ai rules condensed-check", Command: "ai rules", Action: "check condensed"},
	{Retired: "ai rules index-check", Command: "ai rules", Action: "check index"},
	{Retired: "ai rules lint", Command: "ai rules", Action: "check metadata"},
	{Retired: "ai rules points-roundtrip-check", Command: "ai rules", Action: "check points-roundtrip"},
	{Retired: "ai rules render-check", Command: "ai rules", Action: "check render"},
	{Retired: "ai rules coverage-report", Command: "ai rules report", Action: "coverage"},
	{Retired: "ai rules gate-map-report", Command: "ai rules report", Action: "gate-map"},
	{Retired: "ai rules payload-report", Command: "ai rules report", Action: "payload"},
	{Retired: "ai rules router-report", Command: "ai rules report", Action: "router"},
	{Retired: "ai rules condensed-update", Command: "ai rules", Action: "update condensed"},
	{Retired: "ai rules index-update", Command: "ai rules", Action: "update index"},
	{Retired: "ai rules render-update", Command: "ai rules", Action: "update render"},
	{Retired: "hook-check", Command: "ai hooks"},
	{Retired: "digest", Command: "ai digest"},
	{Retired: "token-economy", Command: "ai tokens"},
	{Retired: "protocol-skeleton", Command: "rfc skeletons"},
	{Retired: "rfc index-update", Command: "rfc", Action: "update"},
	{Retired: "rfc discriminate-record", Command: "rfc", Action: "record"},
	{Retired: "rfc rename", Command: "rfc", Action: "move"},
	{Retired: "rfc extraction-create", Command: "rfc extraction", Action: "create"},
	{Retired: "rfc extraction-classify", Command: "rfc extraction", Action: "classify"},
	{Retired: "rfc extraction-status", Command: "rfc extraction", Action: "report"},
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
	{Retired: "test netlab render-check", Command: "test netlab", Action: "check"},
	{Retired: "test netlab render-update", Command: "test netlab", Action: "update"},
	{Retired: "mutation", Command: "test mutation"},
	{Retired: "test mutation record-history", Command: "test mutation", Action: "record"},
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
	{Retired: "test static-http", Command: "mock httpd"},
	{Retired: "test vpp-stub", Command: "test vpp", Action: "stub"},
	{Retired: "test harness isis-wire", Command: "test wire", Action: "isis"},
	{Retired: "test harness ospf-wire", Command: "test wire", Action: "ospf"},
	{Retired: "test harness l2tp-wire", Command: "test wire", Action: "l2tp"},
	{Retired: "test harness l2tp-scale", Command: "test scale", Action: "l2tp"},
	{Retired: "test harness static-http", Command: "mock httpd"},
	{Retired: "test harness vpp-stub", Command: "test vpp", Action: "stub"},
	{Retired: "test-chaos", Command: "chaos selftest"},
	{Retired: "perf-bench", Command: "perf"},
	{Retired: "perf-bench", RetiredAction: "suggestion-report", Command: "perf", Action: "suggest"},
	{Retired: "perf report", Command: "perf", Action: "compare"},
	{Retired: "perf evidence-record", Command: "perf", Action: "record evidence"},
	{Retired: "perf history-record", Command: "perf", Action: "record history"},
	{Retired: "build-artifacts", RetiredAction: "host", Command: "build host-driver"},
	{Retired: "build-artifacts", RetiredAction: "installer-amd64", Command: "build installer", Action: "amd64"},
	{Retired: "build-artifacts", RetiredAction: "installer-arm64", Command: "build installer", Action: "arm64"},
	{Retired: "gokrazy-gosum", Command: "build gosum"},
	{Retired: "iana-asn", Command: "data asn-delegation"},
	// Phase 4: `update` regenerates a committed artifact from its source, and
	// `write` left the vocabulary (owner ruling); a verdict is recorded; a
	// rebase is not a regeneration.
	{Retired: "ai sync write", Command: "ai sync", Action: "update"},
	{Retired: "data asn-delegation write", Command: "data asn-delegation", Action: "update"},
	{Retired: "data ietf-reference write", Command: "data ietf-reference", Action: "update"},
	{Retired: "data ip-special-purpose write", Command: "data ip-special-purpose", Action: "update"},
	{Retired: "doc index write", Command: "doc index", Action: "update"},
	{Retired: "plugin gated-table write", Command: "plugin gated-table", Action: "update"},
	{Retired: "plugin imports write", Command: "plugin imports", Action: "update"},
	{Retired: "repo feature-tags write", Command: "repo feature-tags", Action: "update"},
	{Retired: "web assets write", Command: "web assets", Action: "update"},
	{Retired: "yang glue write", Command: "yang glue", Action: "update"},
	{Retired: "verify status write", Command: "verify status", Action: "record"},
	{Retired: "worktree update", Command: "worktree", Action: "rebase"},
	// Two siblings sharing the left word docker made it a namespace (rule R9
	// of le cli grammar), so the hyphenated areas split into two tokens.
	{Retired: "setup docker-host", Command: "setup docker host"},
	{Retired: "setup docker-kernel", Command: "setup docker kernel"},
	// Mocks and stand-in servers left test, the namespace of suites, for
	// their own (D-Q10); a `-mock` suffix the namespace now says is dropped.
	{Retired: "test cymru", Command: "mock cymru"},
	{Retired: "test dns", Command: "mock dns"},
	{Retired: "test irr", Command: "mock irr"},
	{Retired: "test peeringdb", Command: "mock peeringdb"},
	{Retired: "test rpki", Command: "mock rpki"},
	{Retired: "test rtr-mock", Command: "mock rtr"},
	{Retired: "test radius-mock", Command: "mock radius"},
	{Retired: "test tacacs-mock", Command: "mock tacacs"},
	{Retired: "test syslog", Command: "mock syslog"},
	{Retired: "test httpd", Command: "mock httpd"},
	{Retired: "test lg", Command: "mock lg"},
	{Retired: "test engine-steps", Command: "mock engine-steps"},
	{Retired: "test record-plugin", Command: "mock record-plugin"},
	{Retired: "test text-plugin", Command: "mock text-plugin"},
	{Retired: "test plugin-external", Command: "mock plugin-external"},
	// Compound words: the verb of `scratch` is the whole action word.
	{Retired: "scratch cache-clean", Command: "scratch", Action: "clean"},
	{Retired: "scratch store-trim", Command: "scratch", Action: "trim"},
	{Retired: "scratch links-ensure", Command: "scratch", Action: "ensure"},
	// `reap` and `clean` shared one meaning.
	{Retired: "session reap", Command: "session", Action: "clean"},
	// `cleanup` and `clean` shared one meaning.
	{Retired: "test integration cleanup", Command: "test integration", Action: "clean"},
	// `append` and `record` shared one meaning.
	{Retired: "verify summary append", Command: "verify summary", Action: "record"},
	// `validate` and `check` shared one meaning.
	{Retired: "spec journal validate", Command: "spec journal", Action: "check"},
	// `verify` there meant every check in order, which is the shared name `all`.
	{Retired: "doc check verify", Command: "doc check", Action: "all"},
	// A test-weakening check belongs to the test area that owns the ledger.
	{Retired: "commit audit", Command: "test weakened", Action: "check"},
	// `review-check` embedded the verb in the action word.
	{Retired: "commit review-check", Command: "commit", Action: "check"},
	// It prints what a gating run would start, which is the verb `preview`.
	{Retired: "test functional select", Command: "test functional", Action: "preview"},
	// A suite spelled as the vocabulary verb `install`.
	{Retired: "test functional install", Command: "test functional", Action: "installer"},
	// `*-test` words are nouns to restructure (owner ruling).
	{Retired: "test functional exabgp-test", Command: "test functional", Action: "exabgp"},
	{Retired: "setup web-test", Command: "setup", Action: "web"},
	// It posts to Discord; `update` regenerates a committed artifact.
	{Retired: "weekly update", Command: "weekly", Action: "publish"},
	// `bump` and `sync` rewrite a committed copy from upstream, which is `update`.
	{Retired: "lab pin bump", Command: "lab pin", Action: "update"},
	{Retired: "web vendor sync", Command: "web vendor", Action: "update"},
	// Compound words: the verb is the whole action word, or the name embeds a
	// retired verb.
	{Retired: "web vendor update-report", Command: "web vendor", Action: "report"},
	{Retired: "feature record-run", Command: "feature", Action: "run"},
	{Retired: "setup proto-generate", Command: "setup", Action: "proto"},
	{Retired: "site update-talk", Command: "site", Action: "talk"},
	// One verb over several targets takes the target as its first value
	// (D-P4-positional), and the whole gallery is render's `all` switch.
	{Retired: "site terminal-demo check-all", Command: "site terminal-demo", Action: "check artifacts"},
	{Retired: "site terminal-demo release-check-all", Command: "site terminal-demo", Action: "check release"},
	{Retired: "site terminal-demo validation-check-all", Command: "site terminal-demo", Action: "check validation"},
	{Retired: "site terminal-demo render-all", Command: "site terminal-demo", Action: "render all"},
	{Retired: "site terminal-demo binaries-build-ze", Command: "site terminal-demo", Action: "build binaries"},
	{Retired: "site terminal-demo image-build", Command: "site terminal-demo", Action: "build image"},
	// `*-test` words are nouns to restructure (owner ruling); `install-*` kept
	// the vocabulary verb install inside a name, and the whole run is `all`.
	{Retired: "test qemu all-tests", Command: "test qemu", Action: "all"},
	{Retired: "test qemu install-test", Command: "test qemu", Action: "installer"},
	{Retired: "test qemu install-iso-test", Command: "test qemu", Action: "installer-iso"},
	{Retired: "test qemu install-scenarios-test", Command: "test qemu", Action: "installer-scenarios"},
	{Retired: "test qemu install-ventoy-test", Command: "test qemu", Action: "installer-ventoy"},
	{Retired: "test qemu ipsec-mobike-test", Command: "test qemu", Action: "ipsec-mobike"},
	{Retired: "test qemu mpls-boot-test", Command: "test qemu", Action: "mpls-boot"},
	{Retired: "test qemu netns-test", Command: "test qemu", Action: "netns"},
	{Retired: "test qemu pppoe-accel-test", Command: "test qemu", Action: "pppoe-accel"},
	{Retired: "test qemu pppoe-test", Command: "test qemu", Action: "pppoe"},
	{Retired: "test qemu vpp-hugepages-test", Command: "test qemu", Action: "vpp-hugepages"},
	{Retired: "test qemu vrrp-keepalived-test", Command: "test qemu", Action: "vrrp-keepalived"},
	{Retired: "test deployment docker-l2tp-ppp-test", Command: "test deployment", Action: "docker-l2tp-ppp"},
	{Retired: "test deployment docker-pppoe-accel-test", Command: "test deployment", Action: "docker-pppoe-accel"},
	{Retired: "test deployment gokrazy-l2tp-ppp-test", Command: "test deployment", Action: "gokrazy-l2tp-ppp"},
	{Retired: "test deployment ipsec-mobike-test", Command: "test deployment", Action: "ipsec-mobike"},
	{Retired: "test deployment l2tp-ppp-test", Command: "test deployment", Action: "l2tp-ppp"},
	{Retired: "test deployment l2tp-scale-test", Command: "test deployment", Action: "l2tp-scale"},
	{Retired: "test deployment l2tp-test", Command: "test deployment", Action: "l2tp"},
	{Retired: "test deployment vpp-iface-test", Command: "test deployment", Action: "vpp-iface"},
	{Retired: "test deployment vpp-test", Command: "test deployment", Action: "vpp"},
}

// Renames answers a copy of the command half of the rename map, in
// declaration order.
func Renames() []Rename {
	return slices.Clone(renames)
}

// renameTable is a rename map with each row's old words split once, so the
// Match that every dispatch and every Tab runs allocates nothing per row.
type renameTable struct {
	rows []Rename
	old  [][]string
}

// newRenameTable splits the old words of every row of rows.
func newRenameTable(rows []Rename) renameTable {
	table := renameTable{rows: rows, old: make([][]string, len(rows))}
	for index, row := range rows {
		table.old[index] = row.Old()
	}
	return table
}

// table is the rename map Match and Successor read.
var table = newRenameTable(renames)

// Match answers the row a typed line names, and whether one does. A row
// matches when its old words open the line; the longest such row wins, so
// `spec session claim` answers its own row and not the bare `spec session`
// one. words is the line le was given, its own words only (no pipe).
func Match(words []string) (Rename, bool) {
	return table.match(words)
}

// match is Match over this table.
func (t renameTable) match(words []string) (Rename, bool) {
	best := -1
	for index, old := range t.old {
		if len(old) > len(words) {
			continue
		}
		if !slices.Equal(words[:len(old)], old) {
			continue
		}
		if best >= 0 && len(old) <= len(t.old[best]) {
			continue
		}
		best = index
	}
	if best < 0 {
		return Rename{}, false
	}
	return t.rows[best], true
}

// Successor answers the line a caller types now in place of words, which row
// matched: the row's new words, then every word the caller typed after the old
// ones, so `verify lock run x` answers `job run x`. A line a later rename
// retired in turn is followed to the line le holds now, so `spec session claim
// spec x` answers `spec claim x` and never the retired `spec claim spec x`:
// a refusal that names a retired line refuses its reader twice.
func (r Rename) Successor(words []string) []string {
	return table.successor(r, words)
}

// successorHopsMax bounds the hops successor follows that do not shorten the
// line. A hop that shortens it needs no bound of its own: the line keeps at
// least one word, so shortening hops run out once they have removed every word
// the caller typed and every word the counted hops added, and a caller may type
// as many as they like (`test harness harness ...` shortens one word a hop).
// The row count is no bound on the counted hops either: a chain may reuse a
// row while the caller's trailing words grow (a->b, b->a q, then b q q->z takes
// six hops over three rows), so a chain that ends can be longer than the map.
// The declared map's chains hold a few such hops, and
// TestSuccessorIsNeverARetiredLine walks every row of it with tails derived
// from the map, so a declaration whose chain outgrew this bound panics in that
// test before it reaches a session's le.
const successorHopsMax = 64

// successor is Successor over this table.
func (t renameTable) successor(r Rename, words []string) []string {
	line := append(r.New(), words[len(r.Old()):]...)
	// Each hop applies one row of the map, and every line is checked for a
	// repeat before the bound, so a cycle met late still panics as a cycle. A
	// line met twice is a cycle; a line that never repeats yet keeps or grows
	// its length for successorHopsMax hops grows without end (a row whose new
	// words re-open its own old ones). What a caller types decides only how
	// many shortening hops there are, which the bound does not count, so only
	// a Ze defect in the declaration above reaches either BUG panic.
	var seen [][]string
	kept := 0
	for {
		next, retired := t.match(line)
		if !retired {
			return line
		}
		for _, earlier := range seen {
			if slices.Equal(earlier, line) {
				panic("BUG: leretired: the rename map holds a cycle through `" + strings.Join(line, " ") + "`")
			}
		}
		if kept == successorHopsMax {
			panic("BUG: leretired: the rename of `" + strings.Join(words, " ") + "` did not settle within " +
				strconv.Itoa(successorHopsMax) + " hops, at `" + strings.Join(line, " ") + "`")
		}
		seen = append(seen, line)
		after := append(next.New(), line[len(next.Old()):]...)
		if len(after) >= len(line) {
			kept++
		}
		line = after
	}
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
	default:
		panic("BUG: retired name has an unknown kind")
	}
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

// VALIDATES: the reverse-engineered CLI grammar rules R1-R9 (ai/rules/cli.md)
// as pure functions over command paths and command.Node structure.
// PREVENTS: grammar drift -- a new command that is noun-first, uses a --flag,
// carries an untyped value slot, mis-orders value-before-keyword, uses a config
// mutation verb, types an identifier numerically, or hyphenates a namespace member
// as a fake compound (R9) must be caught here.

package grammar

import (
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// ruleOf returns true if any finding cites the given rule.
func hasRule(findings []Finding, rule string) bool {
	for _, f := range findings {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

func TestCheckNameVerbFirst(t *testing.T) { // R1
	if f := CheckName("show bgp summary"); len(f) != 0 {
		t.Errorf("valid verb-first command flagged: %v", f)
	}
	if f := CheckName("metrics pool"); !hasRule(f, "R1") {
		t.Errorf("noun-first 'metrics pool' not flagged R1: %v", f)
	}
	if f := CheckName("config archive"); !hasRule(f, "R1") {
		t.Errorf("noun-first 'config archive' not flagged R1: %v", f)
	}
}

func TestCheckNameTokenForm(t *testing.T) { // R2
	if f := CheckName("show foo-bar baz"); len(f) != 0 {
		t.Errorf("valid kebab tokens flagged: %v", f)
	}
	if f := CheckName("show Foo"); !hasRule(f, "R2") {
		t.Errorf("uppercase token not flagged R2: %v", f)
	}
	if f := CheckName("show foo-"); !hasRule(f, "R2") {
		t.Errorf("trailing-hyphen token not flagged R2: %v", f)
	}
	if f := CheckName("show  foo"); !hasRule(f, "R2") {
		t.Errorf("repeated whitespace not flagged R2: %v", f)
	}
}

func TestCheckNameNoFlag(t *testing.T) { // R3
	if f := CheckName("show route limit 50"); len(f) != 0 {
		t.Errorf("keyword-filter form flagged: %v", f)
	}
	if f := CheckName("show route --limit 50"); !hasRule(f, "R3") {
		t.Errorf("--flag token not flagged R3: %v", f)
	}
}

func TestCheckNameMutationVerb(t *testing.T) { // R7 (path-level mutation tokens)
	if f := CheckName("clear interface counters"); len(f) != 0 {
		t.Errorf("clear command flagged: %v", f)
	}
	// create is a runtime-lifecycle verb, not a mutation token.
	if f := CheckName("create interface dummy"); hasRule(f, "R7") {
		t.Errorf("'create' verb wrongly flagged R7: %v", f)
	}
	if f := CheckName("request interface addr add"); !hasRule(f, "R7") {
		t.Errorf("'add' mutation token not flagged R7: %v", f)
	}
	if f := CheckName("request interface addr remove"); !hasRule(f, "R7") {
		t.Errorf("'remove' mutation token not flagged R7: %v", f)
	}
	// `del` is only the completion prefix of the `delete` verb, never a command.
	if f := CheckName("delete interface"); hasRule(f, "R7") {
		t.Errorf("delete wrongly flagged R7: %v", f)
	}
}

func TestCheckNodeKeywordBeforeValue(t *testing.T) { // R5
	good := &command.Node{
		Name:       "detail",
		WireMethod: "ze-show:x",
		ArgDefs:    []command.ArgDef{{Name: "name", Kind: command.ArgString}},
	}
	if f := CheckNode("show interface name detail", good); hasRule(f, "R5") {
		t.Errorf("typed selector value flagged R5: %v", f)
	}
	bad := &command.Node{
		Name:       "x",
		WireMethod: "ze-show:x",
		ArgDefs:    []command.ArgDef{{Name: "", Kind: command.ArgString}},
	}
	if f := CheckNode("show x", bad); !hasRule(f, "R5") {
		t.Errorf("unnamed free-form value not flagged R5: %v", f)
	}
}

func TestCheckNodeValueBeforeKeyword(t *testing.T) { // R6
	bad := &command.Node{
		Name:       "cache",
		WireMethod: "",
		ArgDefs:    []command.ArgDef{{Name: "id", Kind: command.ArgString, Mandatory: true}},
		Children:   map[string]*command.Node{"retain": {Name: "retain", WireMethod: "ze-bgp:cache-retain"}},
	}
	if f := CheckNode("cache", bad); !hasRule(f, "R6") {
		t.Errorf("value-before-keyword (cache <id> retain) not flagged R6: %v", f)
	}
	// An OPTIONAL value coexisting with subcommands is NOT a violation.
	okFork := &command.Node{
		Name:       "route",
		WireMethod: "ze-show:route",
		ArgDefs:    []command.ArgDef{{Name: "prefix", Kind: command.ArgString, Mandatory: false}},
		Children:   map[string]*command.Node{"lookup": {Name: "lookup", WireMethod: "ze-show:route-lookup"}},
	}
	if f := CheckNode("show route", okFork); hasRule(f, "R6") {
		t.Errorf("optional value + subcommand fork wrongly flagged R6: %v", f)
	}
	// A typed SELECTOR node carrying both the value and sub-actions is the correct
	// `... name <n> unit ...` shape and must NOT be flagged.
	okSelector := &command.Node{
		Name:       "name",
		WireMethod: "ze-iface:interface-create-dummy",
		ArgDefs:    []command.ArgDef{{Name: "value", Kind: command.ArgString, Mandatory: true}},
		Children:   map[string]*command.Node{"unit": {Name: "unit", WireMethod: "ze-iface:interface-unit-add"}},
	}
	if f := CheckNode("create interface dummy name", okSelector); hasRule(f, "R6") {
		t.Errorf("typed selector node with value + sub-action wrongly flagged R6: %v", f)
	}
}

func TestCheckNodeStringIdentifier(t *testing.T) { // R8
	good := &command.Node{Name: "id", WireMethod: "ze-x:y", ArgDefs: []command.ArgDef{{Name: "session-id", Kind: command.ArgString}}}
	if f := CheckNode("show l2tp session id", good); hasRule(f, "R8") {
		t.Errorf("string session-id flagged R8: %v", f)
	}
	bad := &command.Node{Name: "id", WireMethod: "ze-x:y", ArgDefs: []command.ArgDef{{Name: "session-id", Kind: command.ArgUint}}}
	if f := CheckNode("show l2tp session id", bad); !hasRule(f, "R8") {
		t.Errorf("numeric session-id not flagged R8: %v", f)
	}
}

func TestCheckSiblings(t *testing.T) { // R9
	// `traffic` is a real namespace, so `traffic-stat` / `traffic-feature` are members
	// masquerading as compounds and must be flagged; the bare `traffic` and unrelated
	// `bgp` must not.
	names := []string{"traffic", "traffic-stat", "traffic-feature", "bgp"}
	f := CheckSiblings("show", names)
	if !hasRule(f, "R9") {
		t.Fatalf("traffic-stat/traffic-feature collision not flagged R9: %v", f)
	}
	flagged := map[string]bool{}
	for _, x := range f {
		flagged[x.Command] = true
	}
	if !flagged["show traffic-stat"] || !flagged["show traffic-feature"] {
		t.Errorf("expected show traffic-stat and show traffic-feature flagged, got %v", f)
	}
	if flagged["show traffic"] || flagged["show bgp"] {
		t.Errorf("bare namespace / unrelated sibling wrongly flagged: %v", f)
	}

	// A genuine compound with no colliding sibling is never flagged (low false positive).
	if f := CheckSiblings("resolve peeringdb", []string{"as-set", "max-prefix"}); len(f) != 0 {
		t.Errorf("genuine compounds flagged R9: %v", f)
	}

	// Conservative by design: without a bare `session`/`tunnel` sibling AT THIS LEVEL,
	// the flat show forms are left to human review, not mechanically flagged.
	if f := CheckSiblings("show l2tp", []string{"session-history", "session-traffic", "tunnel-history"}); len(f) != 0 {
		t.Errorf("no sibling namespace present, must not flag: %v", f)
	}

	// Empty parent path (root level) still produces a bare command path.
	if f := CheckSiblings("", []string{"traffic", "traffic-stat"}); len(f) != 1 || f[0].Command != "traffic-stat" {
		t.Errorf("root-level path wrong: %v", f)
	}
}

func TestRootNamespaceGrammar(t *testing.T) { // R9 across surfaces, root namespace
	// The cross-surface namespace set: YANG object containers + YANG verbs.
	ns := map[string]bool{
		"traffic": true, "isis": true, "ospf": true, // config/command containers
		"update": true, "show": true, "set": true, "clear": true, // verbs
	}

	// Pre-split root set: the feeder MUST flag all four hyphenated roots. A gate
	// that cannot catch the bug it was written for is not a gate (R-2). This is
	// the red-then-green that matters most in this spec.
	pre := []string{"pipe", "traffic-control", "isis-decode", "ospf-decode", "update-serve", "start", "version", "--plugins"}
	got := CheckRootNamespace(pre, ns)
	flagged := map[string]bool{}
	for _, f := range got {
		flagged[f.Command] = true
		if f.Rule != "R9" {
			t.Errorf("finding %q has rule %q, want R9", f.Command, f.Rule)
		}
	}
	for _, want := range []string{"traffic-control", "isis-decode", "ospf-decode", "update-serve"} {
		if !flagged[want] {
			t.Errorf("pre-split root %q not flagged by the root-namespace feeder", want)
		}
	}
	// A non-hyphenated root and a flag-shaped root (leading '-') must not fire.
	for _, ok := range []string{"pipe", "start", "version", "--plugins"} {
		if flagged[ok] {
			t.Errorf("root %q wrongly flagged", ok)
		}
	}
	if len(got) != 4 {
		t.Errorf("pre-split set: expected exactly 4 findings, got %d (%v)", len(got), got)
	}

	// Post-split root set: zero findings.
	post := []string{"pipe", "traffic", "isis", "ospf", "start", "version", "--plugins"}
	if got := CheckRootNamespace(post, ns); len(got) != 0 {
		t.Errorf("post-split set: expected 0 findings, got %v", got)
	}

	// A fresh hyphenated-namespace root introduced later is caught (the gate
	// prevents tomorrow's drift, not just today's four).
	if got := CheckRootNamespace([]string{"bgp-summary"}, map[string]bool{"bgp": true}); len(got) != 1 || got[0].Command != "bgp-summary" {
		t.Errorf("fresh namespace-collision root not caught: %v", got)
	}

	// A genuine compound with no colliding namespace (`as-set` with no `as`
	// namespace) is never flagged -- low false positive, matching CheckSiblings.
	if got := CheckRootNamespace([]string{"as-set"}, ns); len(got) != 0 {
		t.Errorf("genuine compound wrongly flagged: %v", got)
	}
}

func TestExemptCategory(t *testing.T) {
	cases := map[string]string{
		"ze-bgp:announce":        "bridge",
		"ze-bgp:withdraw":        "bridge",
		"ze-bgp:peer-raw":        "bridge",
		"ze-plugin:command-list": "wire-protocol",
		"ze-system:command-list": "wire-protocol",
		"ze-bgp:plugin-encoding": "wire-protocol",
		"ze-editor:mode-command": "editor",
	}
	for wm, wantCat := range cases {
		cat, ok := ExemptCategory(wm)
		if !ok || cat != wantCat {
			t.Errorf("ExemptCategory(%q) = (%q,%v), want (%q,true)", wm, cat, ok, wantCat)
		}
	}
	// A normal operator command is not exempt.
	if _, ok := ExemptCategory("ze-show:interface"); ok {
		t.Errorf("ze-show:interface should not be exempt")
	}
}

// VALIDATES: the reverse-engineered CLI grammar rules R1-R8 (ai/rules/cli-grammar.md)
// as pure functions over command paths and command.Node structure.
// PREVENTS: grammar drift -- a new command that is noun-first, uses a --flag,
// carries an untyped value slot, mis-orders value-before-keyword, uses a config
// mutation verb, or types an identifier numerically must be caught here.

package grammar

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/command"
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

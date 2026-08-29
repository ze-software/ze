// The rendering rule table as tests. Each case states one row of it: where a
// value sits, how an optional leaf gains its keyword, what a closed value set
// renders as, and that the token list and the string cannot disagree.

package command

import (
	"encoding/json"
	"strings"
	"testing"
)

// VALIDATES: a leaf whose name equals a container on the path renders right
// after that container's keyword, not at the end of the line.
// PREVENTS: `create interface dummy name unit <name>`, which is what a flat
// trailing argument list produces and what no operator can type.
func TestUsagePlacesValueAfterDeclaringKeyword(t *testing.T) {
	for _, tc := range []struct {
		name string
		path []string
		defs []ArgDef
		want string
	}{
		{
			name: "the value belongs to the keyword that declares it",
			path: []string{"create", "interface", "dummy", "name", "unit"},
			defs: []ArgDef{{Name: "name", Kind: ArgString, Mandatory: true}},
			want: "create interface dummy name <name> unit",
		},
		{
			name: "two anchored leaves follow the path, not the alphabet",
			path: []string{"request", "l2tp", "outgoing-call", "remote", "called"},
			defs: []ArgDef{
				{Name: "remote", Kind: ArgString, Mandatory: true},
				{Name: "called", Kind: ArgString, Mandatory: true},
			},
			want: "request l2tp outgoing-call remote <remote> called <called>",
		},
		{
			name: "a leaf matching no container is appended after the keywords",
			path: []string{"show", "tcp-check"},
			defs: []ArgDef{
				{Name: "host", Kind: ArgString, Mandatory: true},
				{Name: "port", Kind: ArgUint, UintBits: 16, Mandatory: true},
			},
			want: "show tcp-check <host> <port>",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			node := &Node{Name: tc.path[len(tc.path)-1], WireMethod: "ze-test:command", ArgDefs: tc.defs}
			if got := UsageLine(Usage(tc.path, node)); got != tc.want {
				t.Errorf("the line reads %q, want %q", got, tc.want)
			}
		})
	}
}

// VALIDATES: an optional leaf renders as a bracketed keyword and value, never
// as a bare optional positional.
// PREVENTS: a generated line that breaks the keyword-before-value rule
// (ai/rules/cli.md), which is the rule the prose already broke.
func TestUsageRendersOptionalLeafWithKeyword(t *testing.T) {
	node := &Node{
		Name:       "sockets",
		WireMethod: "ze-show:system-sockets",
		ArgDefs: []ArgDef{
			{Name: "protocol", Kind: ArgEnum, EnumValues: []string{"tcp", "udp"}},
			{Name: "state", Kind: ArgString},
			{Name: "port", Kind: ArgUint, UintBits: 32},
		},
	}
	want := "show system sockets [protocol <tcp|udp>] [state <state>] [port <port>]"
	if got := UsageLine(Usage([]string{"show", "system", "sockets"}, node)); got != want {
		t.Errorf("the line reads %q, want %q", got, want)
	}
}

// VALIDATES: every mandatory value precedes every optional group, whatever the
// order the definitions arrive in.
// PREVENTS: an operator reading a form that puts an omissible group before a
// value the command cannot run without.
func TestUsageRendersMandatoryBeforeOptional(t *testing.T) {
	node := &Node{
		Name:       "ping",
		WireMethod: "ze-resolve:ping",
		ArgDefs: []ArgDef{
			{Name: "count", Kind: ArgUint, UintBits: 32},
			{Name: "target", Kind: ArgString, Mandatory: true},
		},
	}
	want := "resolve ping <target> [count <count>]"
	if got := UsageLine(Usage([]string{"resolve", "ping"}, node)); got != want {
		t.Errorf("the line reads %q, want %q", got, want)
	}
}

// VALIDATES: a closed value set replaces the value name, for an enumeration and
// for each member form of a union.
// PREVENTS: `<protocol>` where the model already states that tcp and udp are
// the only two answers.
func TestUsageRendersEnumValueSet(t *testing.T) {
	for _, tc := range []struct {
		name string
		def  ArgDef
		want string
	}{
		{
			name: "an enumeration renders its whole value set",
			def:  ArgDef{Name: "protocol", Kind: ArgEnum, EnumValues: []string{"tcp", "udp"}, Mandatory: true},
			want: "show system sockets <tcp|udp>",
		},
		{
			name: "a union renders its member forms",
			def: ArgDef{Name: "id", Kind: ArgUnion, Mandatory: true, EnumValues: []string{"all"}, UnionDefs: []ArgDef{
				{Name: "id", Kind: ArgUint, UintBits: 32},
				{Name: "id", Kind: ArgEnum, EnumValues: []string{"all"}},
			}},
			want: "show system sockets <id|all>",
		},
		{
			name: "a union names its value once when two members state no set",
			def: ArgDef{Name: "id", Kind: ArgUnion, Mandatory: true, UnionDefs: []ArgDef{
				{Name: "id", Kind: ArgUint, UintBits: 32},
				{Name: "id", Kind: ArgString},
			}},
			want: "show system sockets <id>",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			node := &Node{Name: "sockets", WireMethod: "ze-show:system-sockets", ArgDefs: []ArgDef{tc.def}}
			if got := UsageLine(Usage([]string{"show", "system", "sockets"}, node)); got != tc.want {
				t.Errorf("the line reads %q, want %q", got, tc.want)
			}
		})
	}
}

// VALIDATES: the token list and the rendered string are one producer, so a
// machine reader and an operator cannot be told different grammars.
// PREVENTS: the published `grammar` drifting from the published `usage`.
func TestUsageGrammarRendersToUsageString(t *testing.T) {
	node := &Node{
		Name:       "unit",
		WireMethod: "ze-iface:interface-unit-add",
		ArgDefs: []ArgDef{
			{Name: "name", Kind: ArgString, Mandatory: true},
			{Name: "protocol", Kind: ArgEnum, EnumValues: []string{"tcp", "udp"}},
		},
	}
	path := []string{"create", "interface", "dummy", "name", "unit"}
	tokens := Usage(path, node)

	wantKinds := []UsageKind{
		UsageKeyword, UsageKeyword, UsageKeyword, UsageKeyword, UsageValue, UsageKeyword, UsageOption,
	}
	if len(tokens) != len(wantKinds) {
		t.Fatalf("the grammar holds %d tokens, want %d: %+v", len(tokens), len(wantKinds), tokens)
	}
	for i, want := range wantKinds {
		if tokens[i].Kind != want {
			t.Errorf("token %d is %v, want %v", i, tokens[i].Kind, want)
		}
	}

	line := UsageLine(tokens)
	if !strings.HasSuffix(line, "unit [protocol <tcp|udp>]") {
		t.Errorf("the line reads %q", line)
	}
	if line != UsageLine(Usage(path, node)) {
		t.Error("two renderings of one node disagree")
	}
}

// VALIDATES: AC-12 over EVERY kind the catalog publishes. A node carrying a
// required group, a valueless group, a choice, a once group and a repeat group
// renders one line, and the token list the catalog publishes beside it renders
// back to that line byte for byte, including after a trip through JSON.
// PREVENTS: a kind that renders in the string producer and not in the token
// producer, which is a catalog whose two projections disagree. A reader that
// builds an invocation from `grammar` would then type a line `usage` never
// showed.
func TestUsageGrammarRoundTripsEveryKind(t *testing.T) {
	node := &Node{
		Name:       "opaque",
		WireMethod: "ze-debug:ospf-inject",
		ArgDefs:    []ArgDef{{Name: "instance", Kind: ArgString, Mandatory: true}, {Name: "area", Kind: ArgString}},
		Children: map[string]*Node{
			"scope": {
				Name: "scope", Modifier: ModifierRequired, ModifierOrder: 1,
				ArgDefs: []ArgDef{{Name: "scope", Kind: ArgEnum, EnumValues: []string{"link", "area", "as"}, Mandatory: true}},
			},
			"type": {
				Name: "type", Modifier: ModifierOnce, ModifierOrder: 2,
				ArgDefs: []ArgDef{{Name: "type", Kind: ArgUint, UintBits: 8, Mandatory: true}},
			},
			"tlv": {
				Name: "tlv", Modifier: ModifierRepeat, ModifierOrder: 3,
				ArgDefs: []ArgDef{{Name: "type", Kind: ArgString, Mandatory: true}, {Name: "value-hex", Kind: ArgString, Mandatory: true}},
			},
			"form": {
				Name: "form", Modifier: ModifierChoice, ModifierOrder: 4,
				ArgDefs: []ArgDef{{Name: "form", Kind: ArgEnum, EnumValues: []string{"hex", "text"}}},
			},
			"withdraw": {Name: "withdraw", Modifier: ModifierOnce, ModifierOrder: 5},
		},
	}
	path := []string{"debug", "ip", "ospf", "inject", "opaque"}
	tokens := Usage(path, node)

	const want = "debug ip ospf inject opaque <instance> [area <area>] scope <link|area|as> " +
		"[type <type>] [tlv <type> <value-hex> ...] [hex|text] [withdraw]"
	line := UsageLine(tokens)
	if line != want {
		t.Fatalf("the line reads %q, want %q", line, want)
	}

	seen := make(map[UsageKind]bool, len(tokens))
	for i := range tokens {
		seen[tokens[i].Kind] = true
	}
	for _, kind := range []UsageKind{UsageKeyword, UsageValue, UsageOption, UsageGroup, UsageGroupRepeat, UsageChoice} {
		if !seen[kind] {
			t.Errorf("the grammar states no %v token, so this case does not cover it", kind)
		}
	}

	encoded, err := json.Marshal(tokens)
	if err != nil {
		t.Fatalf("the grammar does not publish: %v", err)
	}
	var decoded []UsageToken
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("the published grammar does not read back: %v", err)
	}
	if got := UsageLine(decoded); got != want {
		t.Errorf("the published grammar renders %q, want %q", got, want)
	}
}

// VALIDATES: every kind the table holds reads back from its own published
// word, and a word the table does not hold is refused.
// PREVENTS: a kind unmarshalling out of range, which is a token nobody wrote
// reading as a valid one inside a published artifact.
func TestUsageKindReadsBackFromItsWord(t *testing.T) {
	for _, kind := range []UsageKind{
		UsageUnspecified, UsageKeyword, UsageValue, UsageOption,
		UsageGroup, UsageGroupRepeat, UsageChoice,
	} {
		encoded, err := json.Marshal(kind)
		if err != nil {
			t.Fatalf("the kind %v does not publish: %v", kind, err)
		}
		var got UsageKind
		if err := json.Unmarshal(encoded, &got); err != nil {
			t.Fatalf("the kind %v does not read back: %v", kind, err)
		}
		if got != kind {
			t.Errorf("%s read back as %s", kind, got)
		}
	}
	var got UsageKind
	if err := json.Unmarshal([]byte(`"repeat-group"`), &got); err == nil {
		t.Errorf("a word the table does not hold read back as %s", got)
	}
}

// VALIDATES: a grouping node has no invocation form, and a node that runs a
// command always has one.
// PREVENTS: a usage line for a path an operator cannot execute.
func TestUsageAnswersNothingForAGroupingNode(t *testing.T) {
	grouping := &Node{Name: "interface", Children: map[string]*Node{"dummy": {Name: "dummy"}}}
	if tokens := Usage([]string{"create", "interface"}, grouping); tokens != nil {
		t.Errorf("a grouping node rendered %+v", tokens)
	}
	if line := UsageLine(nil); line != "" {
		t.Errorf("an empty token list rendered %q", line)
	}
}

// VALIDATES: the kind names published in the catalog are the ones this package
// declares, and the zero value names none of them.
// PREVENTS: a token built by mistake reading as a valid one.
func TestUsageKindNamesItself(t *testing.T) {
	for kind, want := range map[UsageKind]string{
		UsageUnspecified: "unspecified",
		UsageKeyword:     "keyword",
		UsageValue:       "value",
		UsageOption:      "option",
		UsageGroup:       "group",
		UsageGroupRepeat: "group-repeat",
		UsageChoice:      "choice",
	} {
		if got := kind.String(); got != want {
			t.Errorf("the kind %d names itself %q, want %q", kind, got, want)
		}
	}
}

// VALIDATES: a child container carrying `ze:modifier` renders as one bracketed
// group, its own name first and then every value it declares, after every leaf
// of the command itself.
// PREVENTS: modeling a two-value group as two independent optional leaves,
// which renders `[key <key>] [value <value>]` and tells an operator the two can
// be supplied apart. `announce ... tag <key> <value>` is one group or it is
// nothing.
func TestUsageRendersModifierGroup(t *testing.T) {
	for _, tc := range []struct {
		name string
		node *Node
		want string
	}{
		{
			name: "a group supplied at most once",
			node: &Node{
				Name:       "announce",
				WireMethod: "ze-bgp:announce",
				Children: map[string]*Node{
					"tag": {
						Name:     "tag",
						Modifier: ModifierOnce,
						ArgDefs: []ArgDef{
							{Name: "key", Kind: ArgString, Mandatory: true},
							{Name: "value", Kind: ArgString, Mandatory: true},
						},
					},
				},
			},
			want: "announce [tag <key> <value>]",
		},
		{
			name: "a group the operator repeats",
			node: &Node{
				Name:       "metrics",
				WireMethod: "ze-show:metrics",
				Children: map[string]*Node{
					"label": {
						Name:     "label",
						Modifier: ModifierRepeat,
						ArgDefs:  []ArgDef{{Name: "name", Kind: ArgString, Mandatory: true}, {Name: "value", Kind: ArgString, Mandatory: true}},
					},
				},
			},
			want: "show metrics [label <name> <value> ...]",
		},
		{
			name: "every leaf of the command comes before every group",
			node: &Node{
				Name:       "announce",
				WireMethod: "ze-bgp:announce",
				ArgDefs:    []ArgDef{{Name: "for", Kind: ArgString}},
				Children: map[string]*Node{
					"tag": {
						Name:     "tag",
						Modifier: ModifierOnce,
						ArgDefs:  []ArgDef{{Name: "key", Kind: ArgString, Mandatory: true}},
					},
				},
			},
			want: "announce [for <for>] [tag <key>]",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := []string{"announce"}
			if tc.node.Name == "metrics" {
				path = []string{"show", "metrics"}
			}
			if got := UsageLine(Usage(path, tc.node)); got != tc.want {
				t.Errorf("the line reads %q, want %q", got, tc.want)
			}
		})
	}
}

// VALIDATES: a child container that runs a command of its own is a SUBCOMMAND,
// never a modifier group, whatever else it carries.
// PREVENTS: the `withdraw tag` shape being swept into its parent's usage line.
// It is its own command with its own line (AC-11).
func TestUsageIgnoresAChildThatIsItsOwnCommand(t *testing.T) {
	node := &Node{
		Name:       "withdraw",
		WireMethod: "ze-bgp:withdraw-all",
		Children: map[string]*Node{
			"tag": {Name: "tag", WireMethod: "ze-bgp:withdraw-tag", ArgDefs: []ArgDef{{Name: "key", Kind: ArgString, Mandatory: true}}},
		},
	}
	if got := UsageLine(Usage([]string{"withdraw"}, node)); got != "withdraw" {
		t.Errorf("the line reads %q, want %q", got, "withdraw")
	}
}

// VALIDATES: two modifier groups render in the order the module declares them,
// not in the order a map yields.
// PREVENTS: a usage line that changes between two runs of the same binary.
func TestUsageRendersModifierGroupsInDeclarationOrder(t *testing.T) {
	node := &Node{
		Name:       "announce",
		WireMethod: "ze-bgp:announce",
		Children: map[string]*Node{
			"tag":  {Name: "tag", Modifier: ModifierOnce, ModifierOrder: 1, ArgDefs: []ArgDef{{Name: "key", Kind: ArgString, Mandatory: true}}},
			"also": {Name: "also", Modifier: ModifierOnce, ModifierOrder: 2, ArgDefs: []ArgDef{{Name: "peer", Kind: ArgString, Mandatory: true}}},
		},
	}
	want := "announce [tag <key>] [also <peer>]"
	for range 8 {
		if got := UsageLine(Usage([]string{"announce"}, node)); got != want {
			t.Fatalf("the line reads %q, want %q", got, want)
		}
	}
}

// VALIDATES: a child container carrying `ze:modifier "required"` renders its
// own keyword and every value it declares, with no brackets, before every
// group the command runs without.
// PREVENTS: `debug ip ospf inject opaque <opaque-id>`, which is what a bare
// mandatory leaf produces. `parseOpaqueInject`
// (internal/plugins/ospf/inject.go) reads the literal `id` token, so a line
// without it is a line the daemon refuses.
func TestUsageRendersRequiredModifierGroup(t *testing.T) {
	node := &Node{
		Name:       "opaque",
		WireMethod: "ze-debug:ospf-inject",
		Children: map[string]*Node{
			"scope": {
				Name: "scope", Modifier: ModifierRequired, ModifierOrder: 1,
				ArgDefs: []ArgDef{{Name: "scope", Kind: ArgEnum, EnumValues: []string{"link", "area", "as"}, Mandatory: true}},
			},
			"id": {
				Name: "id", Modifier: ModifierRequired, ModifierOrder: 2,
				ArgDefs: []ArgDef{{Name: "opaque-id", Kind: ArgUint, UintBits: 32, Mandatory: true}},
			},
			"withdraw": {Name: "withdraw", Modifier: ModifierOnce, ModifierOrder: 3},
		},
	}
	const want = "debug ip ospf inject opaque scope <link|area|as> id <opaque-id> [withdraw]"
	if got := UsageLine(Usage([]string{"debug", "ip", "ospf", "inject", "opaque"}, node)); got != want {
		t.Errorf("the line reads %q, want %q", got, want)
	}
}

// VALIDATES: a modifier group that declares no value renders as its keyword
// alone, bracketed.
// PREVENTS: a presence-only flag being modeled as a leaf. `parseOpaqueInject`
// (internal/plugins/ospf/inject.go) reads `withdraw` and does NOT read a value
// after it, and validateCommandArgs
// (internal/component/plugin/server/command.go) demands a value for every
// declared leaf name it meets, so a `withdraw` LEAF would reject the shipped
// invocation with "withdraw requires a value".
func TestUsageRendersValuelessGroupAsAFlag(t *testing.T) {
	node := &Node{
		Name:       "name",
		WireMethod: "ze-show:pki-certificate",
		ArgDefs:    []ArgDef{{Name: "name", Kind: ArgString, Mandatory: true}},
		Children: map[string]*Node{
			"pem": {Name: "pem", Modifier: ModifierOnce, ModifierOrder: 1},
		},
	}
	const want = "show pki certificate name <name> [pem]"
	if got := UsageLine(Usage([]string{"show", "pki", "certificate", "name"}, node)); got != want {
		t.Errorf("the line reads %q, want %q", got, want)
	}
}

// VALIDATES: a child container carrying `ze:modifier "choice"` renders the
// closed set of its one leaf, bracketed, with no keyword of its own.
// PREVENTS: `[direction <export|import>]`, which states a keyword
// `handleShowPolicyChain` (internal/component/bgp/plugins/cmd/policy/handler.go)
// never reads: it compares args[0] against the literal words `import` and
// `export`, so those words ARE the tokens and there is no value after them.
func TestUsageRendersBareChoice(t *testing.T) {
	node := &Node{
		Name:       "peer",
		WireMethod: "ze-show:policy-chain",
		ArgDefs:    []ArgDef{{Name: "selector", Kind: ArgString, Mandatory: true}},
		Children: map[string]*Node{
			"direction": {
				Name: "direction", Modifier: ModifierChoice, ModifierOrder: 1,
				ArgDefs: []ArgDef{{Name: "direction", Kind: ArgEnum, EnumValues: []string{"import", "export"}}},
			},
		},
	}
	const want = "show policy chain peer <selector> [import|export]"
	if got := UsageLine(Usage([]string{"show", "policy", "chain", "peer"}, node)); got != want {
		t.Errorf("the line reads %q, want %q", got, want)
	}
}

// VALIDATES: a choice group that states no closed set falls back to its own
// keyword rather than rendering an empty bracket pair.
// PREVENTS: `[]` reaching an operator, which names nothing and cannot be typed.
func TestUsageChoiceWithoutAValueSetRendersItsKeyword(t *testing.T) {
	node := &Node{
		Name:       "peer",
		WireMethod: "ze-show:policy-chain",
		Children: map[string]*Node{
			"direction": {Name: "direction", Modifier: ModifierChoice, ModifierOrder: 1},
		},
	}
	const want = "show policy chain peer [direction]"
	if got := UsageLine(Usage([]string{"show", "policy", "chain", "peer"}, node)); got != want {
		t.Errorf("the line reads %q, want %q", got, want)
	}
}

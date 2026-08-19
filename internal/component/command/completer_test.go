package command

import (
	"slices"
	"testing"
)

func testCommandTree() *Node {
	return &Node{
		Children: map[string]*Node{
			"peer": {
				Name:        "peer",
				Description: "Peer operations",
				Children: map[string]*Node{
					"list": {Name: "list", Description: "List all peers"},
					"show": {Name: "show", Description: "Show peer details", Children: map[string]*Node{
						"capabilities": {Name: "capabilities", Description: "Show peer capabilities"},
						"statistics":   {Name: "statistics", Description: "Show peer statistics"},
					}},
				},
			},
			"daemon": {
				Name:        "daemon",
				Description: "Daemon operations",
				Children: map[string]*Node{
					"status": {Name: "status", Description: "Show daemon status"},
				},
			},
			"rib": {
				Name:        "rib",
				Description: "RIB operations",
				Children: map[string]*Node{
					"show": {Name: "show", Description: "Show RIB entries"},
				},
			},
		},
	}
}

// VALIDATES: Tab with empty input shows top-level commands.
// PREVENTS: missing completions for operational commands.
func TestCommandModeCompletions(t *testing.T) {
	cc := NewTreeCompleter(testCommandTree())

	comps := cc.Complete("")
	if len(comps) != 3 {
		t.Fatalf("expected 3 top-level completions, got %d: %v", len(comps), comps)
	}

	// Should be sorted: daemon, peer, rib
	want := []string{"daemon", "peer", "rib"}
	for i, w := range want {
		if comps[i].Text != w {
			t.Errorf("completion[%d] = %q, want %q", i, comps[i].Text, w)
		}
		if comps[i].Type != "command" {
			t.Errorf("completion[%d].Type = %q, want %q", i, comps[i].Type, "command")
		}
	}
}

// VALIDATES: "peer " + Tab shows peer subcommands.
// PREVENTS: missing subcommand completions after space.
func TestCommandModeSubcommandCompletions(t *testing.T) {
	cc := NewTreeCompleter(testCommandTree())

	comps := cc.Complete("peer ")
	if len(comps) != 2 {
		t.Fatalf("expected 2 peer subcommands, got %d: %v", len(comps), comps)
	}

	// Sorted: list, show
	want := []string{"list", "show"}
	for i, w := range want {
		if comps[i].Text != w {
			t.Errorf("completion[%d] = %q, want %q", i, comps[i].Text, w)
		}
	}
}

// VALIDATES: partial word matches correct command.
// PREVENTS: partial prefix not finding valid completions.
func TestCommandModePartialMatch(t *testing.T) {
	cc := NewTreeCompleter(testCommandTree())

	comps := cc.Complete("pe")
	if len(comps) != 1 {
		t.Fatalf("expected 1 completion for 'pe', got %d", len(comps))
	}
	if comps[0].Text != "peer" {
		t.Errorf("expected 'peer', got %q", comps[0].Text)
	}
}

// VALIDATES: nonexistent command returns no completions.
// PREVENTS: spurious completions for invalid input.
func TestCommandModeNoMatch(t *testing.T) {
	cc := NewTreeCompleter(testCommandTree())

	comps := cc.Complete("xyz")
	if len(comps) != 0 {
		t.Errorf("expected 0 completions for 'xyz', got %d", len(comps))
	}
}

// VALIDATES: ghost text works for operational commands.
// PREVENTS: inline completion preview showing wrong suffix.
func TestCommandModeGhostText(t *testing.T) {
	cc := NewTreeCompleter(testCommandTree())

	tests := []struct {
		input string
		want  string
	}{
		{"pe", "er"},                     // "pe" → "peer"
		{"peer l", "ist"},                // "peer l" → "peer list"
		{"peer ", ""},                    // trailing space → no ghost
		{"", ""},                         // empty → no ghost
		{"daemon s", "tatus"},            // "daemon s" → "daemon status"
		{"peer list | j", "son"},         // pipe operator ghost
		{"peer list | json c", "ompact"}, // pipe sub-arg ghost
	}

	for _, tt := range tests {
		got := cc.GhostText(tt.input)
		if got != tt.want {
			t.Errorf("GhostText(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// VALIDATES: nil root produces empty completions.
// PREVENTS: nil pointer dereference with uninitialized completer.
func TestCommandModeNilRoot(t *testing.T) {
	cc := NewTreeCompleter(nil)
	comps := cc.Complete("")
	if len(comps) != 0 {
		t.Errorf("expected 0 completions with nil root, got %d", len(comps))
	}
	ghost := cc.GhostText("pe")
	if ghost != "" {
		t.Errorf("expected empty ghost with nil root, got %q", ghost)
	}
}

// VALIDATES: pipe completions appear after | character.
// PREVENTS: pipe operators not offered during completion.
func TestCommandModePipeCompletion(t *testing.T) {
	cc := NewTreeCompleter(testCommandTree())

	comps := cc.Complete("peer list | ")
	if len(comps) != len(PipeOperators) {
		t.Fatalf("expected %d pipe completions, got %d", len(PipeOperators), len(comps))
	}
	for _, c := range comps {
		if c.Type != "pipe" {
			t.Errorf("pipe completion %q should have type 'pipe', got %q", c.Text, c.Type)
		}
	}
}

// VALIDATES: registered pipe filters are added to completions.
// PREVENTS: filters being hardcoded into global pipe completion.
func TestCommandModePipeCompletion_WithFilters(t *testing.T) {
	ResetPipeFiltersForTest()
	t.Cleanup(ResetPipeFiltersForTest)
	RegisterPipeFilters([]string{"rib show"}, PipeFilter{Name: "source", Description: "Select source"})

	cc := NewTreeCompleter(testCommandTree())
	comps := cc.Complete("rib show | sou")
	if len(comps) != 1 {
		t.Fatalf("expected 1 command pipe completion, got %d: %v", len(comps), comps)
	}
	if comps[0].Text != "source" {
		t.Fatalf("completion = %q, want source", comps[0].Text)
	}
}

// VALIDATES: pipe filters are scoped to their registered command.
// PREVENTS: one command's filters leaking into every completion.
func TestCommandModePipeCompletion_NoFilters(t *testing.T) {
	ResetPipeFiltersForTest()
	t.Cleanup(ResetPipeFiltersForTest)
	RegisterPipeFilters([]string{"rib show"}, PipeFilter{Name: "source", Description: "Select source"})

	cc := NewTreeCompleter(testCommandTree())
	comps := cc.Complete("peer list | sou")
	if len(comps) != 0 {
		t.Fatalf("expected no source completion for peer list, got %v", comps)
	}
}

// VALIDATES: partial pipe operator input filters correctly.
// PREVENTS: wrong pipe operators shown for partial input.
func TestCommandModePipePartialCompletion(t *testing.T) {
	cc := NewTreeCompleter(testCommandTree())

	comps := cc.Complete("peer list | ma")
	if len(comps) != 1 {
		t.Fatalf("expected 1 pipe completion for 'ma', got %d", len(comps))
	}
	if comps[0].Text != "match" {
		t.Errorf("expected 'match', got %q", comps[0].Text)
	}
}

// VALIDATES: json pipe operator offers compact/pretty sub-arguments.
// PREVENTS: "json <tab>" duplicating to "json json" instead of showing sub-args.
func TestCommandModePipeJsonSubArgs(t *testing.T) {
	cc := NewTreeCompleter(testCommandTree())

	// "json " (with space) should offer sub-arguments, not repeat "json".
	comps := cc.Complete("peer list | json ")
	if len(comps) != 2 {
		t.Fatalf("expected 2 json sub-arg completions, got %d: %v", len(comps), comps)
	}
	want := map[string]bool{"compact": true, "pretty": true}
	for _, c := range comps {
		if !want[c.Text] {
			t.Errorf("unexpected json sub-arg %q", c.Text)
		}
		if c.Type != "pipe" {
			t.Errorf("sub-arg %q should have type 'pipe', got %q", c.Text, c.Type)
		}
	}

	// "json c" should filter to "compact" only.
	comps = cc.Complete("peer list | json c")
	if len(comps) != 1 || comps[0].Text != "compact" {
		t.Errorf("expected [compact], got %v", comps)
	}

	// "count " (no sub-args) should return nothing.
	comps = cc.Complete("peer list | count ")
	if len(comps) != 0 {
		t.Errorf("expected 0 completions after 'count ', got %d", len(comps))
	}
}

// VALIDATES: AC-2,AC-3 — ValueHints returned by matchChildren alongside static children.
// PREVENTS: value completions missing from nodes that have both children and value hints.
func TestValueHintsIncludedInMatchChildren(t *testing.T) {
	tree := &Node{
		Children: map[string]*Node{
			"rib": {
				Name:        "rib",
				Description: "RIB operations",
				Children: map[string]*Node{
					"show": {Name: "show", Description: "Show RIB entries"},
				},
				ValueHints: func() []Suggestion {
					return []Suggestion{
						{Text: "ipv4/unicast", Description: "IPv4 unicast family", Type: "value"},
						{Text: "ipv6/unicast", Description: "IPv6 unicast family", Type: "value"},
					}
				},
			},
		},
	}

	cc := NewTreeCompleter(tree)
	comps := cc.Complete("rib ")

	// Should include static child "show" plus 2 value hints = 3 total.
	if len(comps) != 3 {
		t.Fatalf("expected 3 completions (1 child + 2 value hints), got %d: %v", len(comps), comps)
	}

	// Check types: "show" is command, families are value.
	typeMap := make(map[string]string)
	for _, c := range comps {
		typeMap[c.Text] = c.Type
	}
	if typeMap["show"] != "command" {
		t.Errorf("show should have type 'command', got %q", typeMap["show"])
	}
	if typeMap["ipv4/unicast"] != "value" {
		t.Errorf("ipv4/unicast should have type 'value', got %q", typeMap["ipv4/unicast"])
	}
	if typeMap["ipv6/unicast"] != "value" {
		t.Errorf("ipv6/unicast should have type 'value', got %q", typeMap["ipv6/unicast"])
	}
}

// VALIDATES: AC-2,AC-3 — ValueHint prefix filtering works.
// PREVENTS: value hints ignoring the partial word typed by the user.
func TestValueHintsPrefixFiltered(t *testing.T) {
	tree := &Node{
		Children: map[string]*Node{
			"rib": {
				Name: "rib",
				ValueHints: func() []Suggestion {
					return []Suggestion{
						{Text: "ipv4/unicast", Description: "IPv4 unicast", Type: "value"},
						{Text: "ipv6/unicast", Description: "IPv6 unicast", Type: "value"},
					}
				},
			},
		},
	}

	cc := NewTreeCompleter(tree)

	// "rib ipv4" should filter to ipv4/unicast only.
	comps := cc.Complete("rib ipv4")
	if len(comps) != 1 {
		t.Fatalf("expected 1 completion for prefix 'ipv4', got %d: %v", len(comps), comps)
	}
	if comps[0].Text != "ipv4/unicast" {
		t.Errorf("expected 'ipv4/unicast', got %q", comps[0].Text)
	}
}

// VALIDATES: AC-10 — Node without ValueHints behaves exactly as before.
// PREVENTS: regression in existing completion behavior.
func TestNodeWithoutValueHintsUnchanged(t *testing.T) {
	cc := NewTreeCompleter(testCommandTree())

	// Existing behavior: "peer " shows list, show.
	comps := cc.Complete("peer ")
	if len(comps) != 2 {
		t.Fatalf("expected 2 completions, got %d", len(comps))
	}
	want := []string{"list", "show"}
	for i, w := range want {
		if comps[i].Text != w {
			t.Errorf("completion[%d] = %q, want %q", i, comps[i].Text, w)
		}
	}
}

// VALIDATES: DynamicChildren and ValueHints combine correctly on the same node.
// PREVENTS: one callback shadowing the other when both are set.
func TestDynamicChildrenAndValueHintsCombined(t *testing.T) {
	tree := &Node{
		Children: map[string]*Node{
			"peer": {
				Name: "peer",
				Children: map[string]*Node{
					"list": {Name: "list", Description: "List peers"},
				},
				DynamicChildren: func() []Suggestion {
					return []Suggestion{
						{Text: "10.0.0.1", Description: "peer ip", Type: "selector"},
					}
				},
				ValueHints: func() []Suggestion {
					return []Suggestion{
						{Text: "ipv4/unicast", Description: "family", Type: "value"},
					}
				},
			},
		},
	}

	cc := NewTreeCompleter(tree)
	comps := cc.Complete("peer ")

	// Should include: list (child) + 10.0.0.1 (dynamic) + ipv4/unicast (value) = 3.
	if len(comps) != 3 {
		t.Fatalf("expected 3 completions (child + dynamic + value), got %d: %v", len(comps), comps)
	}

	types := make(map[string]string)
	for _, c := range comps {
		types[c.Text] = c.Type
	}
	if types["list"] != "command" {
		t.Errorf("list Type = %q, want 'command'", types["list"])
	}
	if types["10.0.0.1"] != "selector" {
		t.Errorf("10.0.0.1 Type = %q, want 'selector'", types["10.0.0.1"])
	}
	if types["ipv4/unicast"] != "value" {
		t.Errorf("ipv4/unicast Type = %q, want 'value'", types["ipv4/unicast"])
	}
}

// VALIDATES: AC-6 — node with Backend ["netlink"] excluded when active is "vpp".
// PREVENTS: backend-specific commands shown for wrong backend.
func TestCommandCompleterBackendFilter(t *testing.T) {
	tree := &Node{
		Children: map[string]*Node{
			"vpp": {
				Name:        "vpp",
				Description: "VPP operations",
				Backend:     []string{"vpp"},
			},
			"general": {
				Name:        "general",
				Description: "General operations",
			},
			"netlink-only": {
				Name:        "netlink-only",
				Description: "Netlink operations",
				Backend:     []string{"netlink"},
			},
		},
	}

	cc := NewTreeCompleter(tree)
	cc.SetActiveBackends(map[string]string{"interface": "netlink"})

	comps := cc.Complete("")
	names := make([]string, len(comps))
	for i, c := range comps {
		names[i] = c.Text
	}

	if len(comps) != 2 {
		t.Fatalf("expected 2 completions (general + netlink-only), got %d: %v", len(comps), names)
	}
	// Sorted: general, netlink-only
	want := []string{"general", "netlink-only"}
	for i, w := range want {
		if comps[i].Text != w {
			t.Errorf("completion[%d] = %q, want %q", i, comps[i].Text, w)
		}
	}
}

// VALIDATES: AC-7 — node with nil Backend always shown.
// PREVENTS: unrestricted nodes filtered out.
func TestCommandCompleterBackendUnrestricted(t *testing.T) {
	tree := &Node{
		Children: map[string]*Node{
			"peer": {
				Name:        "peer",
				Description: "Peer operations",
			},
		},
	}

	cc := NewTreeCompleter(tree)
	cc.SetActiveBackends(map[string]string{"interface": "vpp"})

	comps := cc.Complete("")
	if len(comps) != 1 || comps[0].Text != "peer" {
		t.Errorf("expected [peer], got %v", comps)
	}
}

// VALIDATES: GhostText works with ValueHints suggestions.
// PREVENTS: inline preview ignoring value hint completions.
func TestGhostTextWithValueHints(t *testing.T) {
	tree := &Node{
		Children: map[string]*Node{
			"rib": {
				Name: "rib",
				ValueHints: func() []Suggestion {
					return []Suggestion{
						{Text: "ipv4/unicast", Type: "value"},
						{Text: "ipv6/unicast", Type: "value"},
					}
				},
			},
		},
	}

	cc := NewTreeCompleter(tree)

	// "rib ipv4" should ghost-complete to "/unicast".
	ghost := cc.GhostText("rib ipv4")
	if ghost != "/unicast" {
		t.Errorf("GhostText('rib ipv4') = %q, want '/unicast'", ghost)
	}

	// "rib ipv" should ghost-complete common prefix "/".
	// Both ipv4/unicast and ipv6/unicast share prefix "ipv" -> next differs at index 3.
	ghost = cc.GhostText("rib ipv")
	if ghost != "" {
		// Common prefix of "ipv4/unicast" and "ipv6/unicast" after "ipv" is empty
		// because the next chars differ ('4' vs '6').
		t.Errorf("GhostText('rib ipv') = %q, want '' (ambiguous)", ghost)
	}
}

// TestCompleterArgDefsEnumSuggestions verifies that enum values from ArgDefs
// appear as value suggestions in the completer output.
//
// VALIDATES: AC-5 -- enum values from ArgDefs appear as suggestions.
func TestCompleterArgDefsEnumSuggestions(t *testing.T) {
	root := &Node{
		Children: map[string]*Node{
			"show": {
				Name: "show",
				Children: map[string]*Node{
					"goroutines": {
						Name:       "goroutines",
						WireMethod: "ze-show:system-goroutines",
						ArgDefs: []ArgDef{
							{Name: "mode", Kind: ArgEnum, EnumValues: []string{"blocked", "full", "summary"}},
						},
					},
				},
			},
		},
	}

	cc := NewTreeCompleter(root)
	completions := cc.Complete("show goroutines ")

	texts := make(map[string]bool)
	for _, s := range completions {
		texts[s.Text] = true
	}
	for _, want := range []string{"blocked", "full", "summary"} {
		if !texts[want] {
			t.Errorf("missing enum suggestion %q", want)
		}
	}
}

// TestCompleterArgDefsKeywordSuggestions verifies that keyword arg names from
// ArgDefs appear as suggestions.
//
// VALIDATES: AC-6 -- leaf names appear as keyword suggestions.
func TestCompleterArgDefsKeywordSuggestions(t *testing.T) {
	root := &Node{
		Children: map[string]*Node{
			"show": {
				Name: "show",
				Children: map[string]*Node{
					"audit": {
						Name:       "audit",
						WireMethod: "ze-show:audit",
						ArgDefs: []ArgDef{
							{Name: "action", Kind: ArgString},
							{Name: "count", Kind: ArgUint, UintBits: 32},
						},
					},
				},
			},
		},
	}

	cc := NewTreeCompleter(root)
	completions := cc.Complete("show audit ")

	texts := make(map[string]bool)
	for _, s := range completions {
		texts[s.Text] = true
	}
	for _, want := range []string{"action", "count"} {
		if !texts[want] {
			t.Errorf("missing keyword suggestion %q", want)
		}
	}
}

// TestCompleterArgDefsPrefixFilter verifies that ArgDef suggestions are prefix-filtered.
func TestCompleterArgDefsPrefixFilter(t *testing.T) {
	root := &Node{
		Children: map[string]*Node{
			"show": {
				Name: "show",
				Children: map[string]*Node{
					"goroutines": {
						Name:       "goroutines",
						WireMethod: "ze-show:system-goroutines",
						ArgDefs: []ArgDef{
							{Name: "mode", Kind: ArgEnum, EnumValues: []string{"blocked", "full", "summary"}},
						},
					},
				},
			},
		},
	}

	cc := NewTreeCompleter(root)
	completions := cc.Complete("show goroutines b")

	if len(completions) != 1 {
		t.Fatalf("expected 1 completion for prefix 'b', got %d", len(completions))
	}
	if completions[0].Text != "blocked" {
		t.Errorf("expected 'blocked', got %q", completions[0].Text)
	}
}

// TestCompleterArgDefsDedup verifies that overlapping ValueHints and ArgDefs
// do not produce duplicate suggestions.
//
// VALIDATES: Review fix -- deduplication of ValueHints and ArgDefs.
func TestCompleterArgDefsDedup(t *testing.T) {
	root := &Node{
		Children: map[string]*Node{
			"test": {
				Name:       "test",
				WireMethod: "ze-test:cmd",
				ValueHints: func() []Suggestion {
					return []Suggestion{
						{Text: "max", Type: "value"},
					}
				},
				ArgDefs: []ArgDef{
					{Name: "limit", Kind: ArgUnion, EnumValues: []string{"max"}},
				},
			},
		},
	}

	cc := NewTreeCompleter(root)
	completions := cc.Complete("test ")

	count := 0
	for _, s := range completions {
		if s.Text == "max" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 'max' once, got %d times", count)
	}
}

// columnCompleterTree is the smallest tree that resolves `show test peers` to a
// command, which is what carries the declared column order into completion.
func columnCompleterTree() *Node {
	return &Node{
		Children: map[string]*Node{
			"show": {Name: "show", Children: map[string]*Node{
				"test": {Name: "test", Children: map[string]*Node{
					"peers": {Name: "peers"},
				}},
			}},
		},
	}
}

// suggestionTexts returns the text of each suggestion, in order.
func suggestionTexts(items []Suggestion) []string {
	texts := make([]string, len(items))
	for i, item := range items {
		texts[i] = item.Text
	}
	return texts
}

// VALIDATES: the field names after `| display` come from the column registry
// (AC-8, first field).
// PREVENTS: an operator having to read the docs to learn what a command's
// answer carries. The registry is the only in-process list of those names.
func TestCompleteDisplayFieldsFromRegistry(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)
	RegisterColumns([]string{"show test peers"}, ColumnOrder{"address", "state", "uptime"})

	cc := NewTreeCompleter(columnCompleterTree())
	got := suggestionTexts(cc.Complete("show test peers | display "))
	want := []string{"address", "state", "uptime"}
	if !slices.Equal(got, want) {
		t.Errorf("completions = %v, want the declared names %v", got, want)
	}

	if got := suggestionTexts(cc.Complete("show test peers | display st")); !slices.Equal(got, []string{"state"}) {
		t.Errorf("completions = %v, want [state]", got)
	}

	// A command that declared no order offers no field names.
	ResetColumnsForTest()
	if got := cc.Complete("show test peers | display "); len(got) != 0 {
		t.Errorf("a command that declared no order offered %v", suggestionTexts(got))
	}
}

// VALIDATES: a second field name completes as well as the first, because the
// match is on the LAST token typed (AC-8, A-2).
// PREVENTS: completePipe matching the whole tail after the operator name, which
// answers nothing once more than one field is typed.
func TestCompleteDisplayFieldsAfterFirst(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)
	RegisterColumns([]string{"show test peers"}, ColumnOrder{"address", "state", "uptime"})

	cc := NewTreeCompleter(columnCompleterTree())

	got := suggestionTexts(cc.Complete("show test peers | display address "))
	if want := []string{"state", "uptime"}; !slices.Equal(got, want) {
		t.Errorf("completions = %v, want the remaining names %v", got, want)
	}
	if got := suggestionTexts(cc.Complete("show test peers | display address st")); !slices.Equal(got, []string{"state"}) {
		t.Errorf("completions = %v, want [state]", got)
	}
	if got := suggestionTexts(cc.Complete("show test peers | display address state up")); !slices.Equal(got, []string{"uptime"}) {
		t.Errorf("completions = %v, want [uptime]", got)
	}
}

// VALIDATES: a field already typed is not offered again (AC-8).
// PREVENTS: `| display address address`, which displays one column twice and
// hides the names the operator has not used yet.
func TestCompleteDisplaySkipsTypedFields(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)
	RegisterColumns([]string{"show test peers"}, ColumnOrder{"address", "state", "uptime"})

	cc := NewTreeCompleter(columnCompleterTree())
	if got := suggestionTexts(cc.Complete("show test peers | display address state ")); !slices.Equal(got, []string{"uptime"}) {
		t.Errorf("completions = %v, want [uptime]", got)
	}
	if got := suggestionTexts(cc.Complete("show test peers | display address a")); len(got) != 0 {
		t.Errorf("a name already typed was offered again: %v", got)
	}
}

// VALIDATES: `| fill` offers its keywords and never a field name (AC-8).
// PREVENTS: the two argument sets being mixed, which is the failure the split
// into two operators exists to remove: a token cannot be a field name in one
// position and a keyword in another.
func TestCompleteFillOffersKeywordsOnly(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)
	RegisterColumns([]string{"show test peers"}, ColumnOrder{"address", "state", "uptime"})

	cc := NewTreeCompleter(columnCompleterTree())
	got := suggestionTexts(cc.Complete("show test peers | fill "))
	want := []string{"alpha", "overall", "reverse"}
	if !slices.Equal(got, want) {
		t.Errorf("completions = %v, want %v", got, want)
	}

	if got := suggestionTexts(cc.Complete("show test peers | fill ov")); !slices.Equal(got, []string{"overall"}) {
		t.Errorf("completions = %v, want [overall]", got)
	}
	if got := suggestionTexts(cc.Complete("show test peers | fill overall re")); !slices.Equal(got, []string{"reverse"}) {
		t.Errorf("completions = %v, want [reverse]: the match is on the last token", got)
	}
	if got := suggestionTexts(cc.Complete("show test peers | fill add")); len(got) != 0 {
		t.Errorf("| fill offered a field name: %v", got)
	}
}

// VALIDATES: an alias completes in the pipe position, beside the operators and
// the command's own filters.
// PREVENTS: a name an operator can type but never discover. An alias exists to
// save the operator the fields it stands for, which they must be offered first.
func TestCommandModePipeCompletion_WithAliases(t *testing.T) {
	resetAliasTables(t)

	RegisterAliases([]string{"peer list"}, Alias{Name: "peers", Description: "The peer rows", Expansion: "display state"})

	cc := NewTreeCompleter(testCommandTree())

	comps := cc.Complete("peer list | pe")
	if len(comps) != 1 {
		t.Fatalf("completions = %v, want the one alias", comps)
	}
	if comps[0].Text != "peers" || comps[0].Type != "pipe" {
		t.Errorf("completion = %+v, want the peers alias typed pipe", comps[0])
	}
	if comps[0].Description != "The peer rows" {
		t.Errorf("description = %q, want the one the registration carries", comps[0].Description)
	}

	// The alias is scoped to its command path, like a filter.
	if other := cc.Complete("rib show | pe"); len(other) != 0 {
		t.Errorf("the alias leaked to another command: %v", other)
	}

	// An alias takes no argument, so nothing follows its name.
	if args := cc.Complete("peer list | peers "); len(args) != 0 {
		t.Errorf("an alias offered an argument: %v", args)
	}
}

package command

import (
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

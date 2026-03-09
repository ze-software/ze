// VALIDATES: AC-3, AC-4 — command mode completions from RPC command tree
// PREVENTS: missing completions for operational commands

package editor

import (
	"testing"
)

func testCommandTree() *CommandNode {
	return &CommandNode{
		Children: map[string]*CommandNode{
			"peer": {
				Name:        "peer",
				Description: "Peer operations",
				Children: map[string]*CommandNode{
					"list": {Name: "list", Description: "List all peers"},
					"show": {Name: "show", Description: "Show peer details", Children: map[string]*CommandNode{
						"capabilities": {Name: "capabilities", Description: "Show peer capabilities"},
						"statistics":   {Name: "statistics", Description: "Show peer statistics"},
					}},
				},
			},
			"daemon": {
				Name:        "daemon",
				Description: "Daemon operations",
				Children: map[string]*CommandNode{
					"status": {Name: "status", Description: "Show daemon status"},
				},
			},
			"rib": {
				Name:        "rib",
				Description: "RIB operations",
				Children: map[string]*CommandNode{
					"show": {Name: "show", Description: "Show RIB entries"},
				},
			},
		},
	}
}

func TestCommandModeCompletions(t *testing.T) {
	// VALIDATES: AC-3 — Tab with empty input shows top-level commands
	cc := NewCommandCompleter(testCommandTree())

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

func TestCommandModeSubcommandCompletions(t *testing.T) {
	// VALIDATES: AC-4 — "peer " + Tab shows peer subcommands
	cc := NewCommandCompleter(testCommandTree())

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

func TestCommandModePartialMatch(t *testing.T) {
	cc := NewCommandCompleter(testCommandTree())

	comps := cc.Complete("pe")
	if len(comps) != 1 {
		t.Fatalf("expected 1 completion for 'pe', got %d", len(comps))
	}
	if comps[0].Text != "peer" {
		t.Errorf("expected 'peer', got %q", comps[0].Text)
	}
}

func TestCommandModeNoMatch(t *testing.T) {
	cc := NewCommandCompleter(testCommandTree())

	comps := cc.Complete("xyz")
	if len(comps) != 0 {
		t.Errorf("expected 0 completions for 'xyz', got %d", len(comps))
	}
}

func TestCommandModeGhostText(t *testing.T) {
	// VALIDATES: ghost text works for operational commands
	cc := NewCommandCompleter(testCommandTree())

	tests := []struct {
		input string
		want  string
	}{
		{"pe", "er"},          // "pe" → "peer"
		{"peer l", "ist"},     // "peer l" → "peer list"
		{"peer ", ""},         // trailing space → no ghost
		{"", ""},              // empty → no ghost
		{"daemon s", "tatus"}, // "daemon s" → "daemon status"
	}

	for _, tt := range tests {
		got := cc.GhostText(tt.input)
		if got != tt.want {
			t.Errorf("GhostText(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCommandModeNilRoot(t *testing.T) {
	cc := NewCommandCompleter(nil)
	comps := cc.Complete("")
	if len(comps) != 0 {
		t.Errorf("expected 0 completions with nil root, got %d", len(comps))
	}
	ghost := cc.GhostText("pe")
	if ghost != "" {
		t.Errorf("expected empty ghost with nil root, got %q", ghost)
	}
}

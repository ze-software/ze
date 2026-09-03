// VALIDATES: AC-3, AC-4 — command mode completions from RPC command tree
// PREVENTS: missing completions for operational commands

package cli

import (
	"testing"
)

func testCommandTree() *commandNode {
	return &commandNode{
		Children: map[string]*commandNode{
			"peer": {
				Name:        "peer",
				Description: "Peer operations",
				Children: map[string]*commandNode{
					"list": {Name: "list", Description: "List all peers"},
					"show": {Name: "show", Description: "Show peer details", Children: map[string]*commandNode{
						"capabilities": {Name: "capabilities", Description: "Show peer capabilities"},
						"statistics":   {Name: "statistics", Description: "Show peer statistics"},
					}},
				},
			},
			"daemon": {
				Name:        "daemon",
				Description: "Daemon operations",
				Children: map[string]*commandNode{
					"status": {Name: "status", Description: "Show daemon status"},
				},
			},
			"rib": {
				Name:        "rib",
				Description: "RIB operations",
				Children: map[string]*commandNode{
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

// TestBothCompleterImplementationsAnswerHelp holds every CommandModeCompleter to
// one contract. The long explanation is text the command DECLARES. An input that
// names no command answers false rather than empty text.
//
// The same table runs over both implementations, so a third one cannot answer a
// different way.
//
// VALIDATES: AC-4, AC-10 — Explain answers declared text, or false.
// PREVENTS: a completer answering ("", true), which the caller renders as a
// command whose author wrote an empty explanation.
func TestBothCompleterImplementationsAnswerHelp(t *testing.T) {
	tree := testCommandTree()
	tree.Children["peer"].Children["list"].LongHelp = "List every peer, with its state and its uptime."
	commands := NewCommandCompleter(tree)
	methods := newPluginCompleter()

	cases := []struct {
		name      string
		completer CommandModeCompleter
		input     string
		want      string
		wantOK    bool
	}{
		{
			name:      "command declares an explanation",
			completer: commands,
			input:     "peer list",
			want:      "List every peer, with its state and its uptime.",
			wantOK:    true,
		},
		{
			name:      "a trailing space still names the command",
			completer: commands,
			input:     "peer list ",
			want:      "List every peer, with its state and its uptime.",
			wantOK:    true,
		},
		{
			name:      "command declares no explanation",
			completer: commands,
			input:     "peer show",
			wantOK:    false,
		},
		{
			name:      "input names no command",
			completer: commands,
			input:     "peer nonesuch",
			wantOK:    false,
		},
		{
			name:      "empty input names no command",
			completer: commands,
			input:     "",
			wantOK:    false,
		},
		{
			name:      "plugin method declares both halves",
			completer: methods,
			input:     "decode-nlri",
			want:      "decode-nlri <family> <hex>\nDecode NLRI from hex",
			wantOK:    true,
		},
		{
			name:      "plugin method that takes no argument",
			completer: methods,
			input:     "unsubscribe-events",
			want:      "unsubscribe-events\nUnsubscribe from engine events",
			wantOK:    true,
		},
		{
			name:      "input names no plugin method",
			completer: methods,
			input:     "no-such-method",
			wantOK:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := tc.completer.Explain(tc.input)
			if ok != tc.wantOK {
				t.Fatalf("Explain(%q) ok = %v, want %v", tc.input, ok, tc.wantOK)
			}
			if got != tc.want {
				t.Errorf("Explain(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

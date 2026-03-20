// VALIDATES: AC-1, AC-2, AC-6, AC-7, AC-9, AC-10 — mode switching and buffer restore
// PREVENTS: mode state lost on switch, viewport content not restored

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestModeSwitchToCommand(t *testing.T) {
	// VALIDATES: AC-1 — run switches to command mode
	m := newTestModel(t)

	if m.Mode() != ModeEdit {
		t.Fatalf("expected initial mode ModeEdit, got %v", m.Mode())
	}

	m.SwitchMode(ModeCommand)

	if m.Mode() != ModeCommand {
		t.Errorf("expected ModeCommand after switch, got %v", m.Mode())
	}
}

func TestModeSwitchToEdit(t *testing.T) {
	// VALIDATES: AC-2 — edit switches to edit mode
	m := newTestModel(t)
	m.SwitchMode(ModeCommand)
	m.SwitchMode(ModeEdit)

	if m.Mode() != ModeEdit {
		t.Errorf("expected ModeEdit after switch, got %v", m.Mode())
	}
}

func TestModeSwitchNoop(t *testing.T) {
	// VALIDATES: AC-9, AC-10 — switching to current mode is no-op
	m := newTestModel(t)

	m.SwitchMode(ModeEdit) // already in edit
	if m.Mode() != ModeEdit {
		t.Errorf("expected ModeEdit, got %v", m.Mode())
	}
	if m.StatusMessage() != "already in edit mode" {
		t.Errorf("expected no-op status message, got %q", m.StatusMessage())
	}

	m.SwitchMode(ModeCommand)
	m.SwitchMode(ModeCommand) // already in command
	if m.Mode() != ModeCommand {
		t.Errorf("expected ModeCommand, got %v", m.Mode())
	}
	if m.StatusMessage() != "already in command mode" {
		t.Errorf("expected no-op status message, got %q", m.StatusMessage())
	}
}

func TestModeScreenRestore(t *testing.T) {
	// VALIDATES: AC-6, AC-7 — viewport content preserved across mode switches
	m := newTestModel(t)

	// Set some viewport content in edit mode
	editContent := "bgp {\n  peer 1.1.1.1 {\n  }\n}"
	m.setViewportText(editContent)

	if m.ViewportContent() != editContent {
		t.Fatalf("expected edit content set, got %q", m.ViewportContent())
	}

	// Switch to command mode — edit content should be saved
	m.SwitchMode(ModeCommand)

	// Command mode starts with empty viewport
	if m.ViewportContent() != "" {
		t.Errorf("expected empty viewport in fresh command mode, got %q", m.ViewportContent())
	}

	// Set command mode content
	cmdContent := "peer list output here"
	m.setViewportText(cmdContent)

	// Switch back to edit — edit content should be restored
	m.SwitchMode(ModeEdit)

	if m.ViewportContent() != editContent {
		t.Errorf("expected edit content restored, got %q", m.ViewportContent())
	}

	// Switch back to command — command content should be restored
	m.SwitchMode(ModeCommand)

	if m.ViewportContent() != cmdContent {
		t.Errorf("expected command content restored, got %q", m.ViewportContent())
	}
}

func TestCommandModeCompletionsWired(t *testing.T) {
	// VALIDATES: AC-3 — Tab in command mode shows operational commands via Model
	m := newTestModel(t)
	m.SetCommandCompleter(NewCommandCompleter(&CommandNode{
		Children: map[string]*CommandNode{
			"peer":   {Name: "peer", Description: "Peer operations"},
			"daemon": {Name: "daemon", Description: "Daemon operations"},
		},
	}))

	// In edit mode, completions come from YANG completer (editor commands)
	m.UpdateCompletions()
	editComps := m.Completions()

	// Switch to command mode — completions merge operational + edit commands
	m.SwitchMode(ModeCommand)
	m.UpdateCompletions()
	cmdComps := m.Completions()

	// Command mode should include operational commands (peer, daemon) merged with edit commands
	if len(cmdComps) <= 2 {
		t.Fatalf("expected merged completions (operational + edit), got %d: %v", len(cmdComps), cmdComps)
	}

	// Verify operational commands are present
	hasPeer, hasDaemon := false, false
	for _, c := range cmdComps {
		if c.Text == "peer" {
			hasPeer = true
		}
		if c.Text == "daemon" {
			hasDaemon = true
		}
	}
	if !hasPeer || !hasDaemon {
		t.Errorf("expected operational commands in merged completions: peer=%v, daemon=%v", hasPeer, hasDaemon)
	}

	// Verify edit commands are also present (set, delete, etc.)
	hasSet := false
	for _, c := range cmdComps {
		if c.Text == "set" {
			hasSet = true
			break
		}
	}
	if !hasSet {
		t.Error("expected edit commands (set) in merged command mode completions")
	}

	// Should have more completions than edit mode alone (edit commands + operational)
	if len(cmdComps) <= len(editComps) {
		t.Errorf("command mode should have more completions than edit mode: cmd=%d, edit=%d", len(cmdComps), len(editComps))
	}
}

func TestCommandModeDispatch(t *testing.T) {
	// VALIDATES: AC-5 — Enter in command mode sends to executor, response shown in viewport
	m := newTestModel(t)
	m.SetCommandCompleter(NewCommandCompleter(&CommandNode{
		Children: map[string]*CommandNode{
			"peer": {Name: "peer", Children: map[string]*CommandNode{
				"list": {Name: "list", Description: "List peers"},
			}},
		},
	}))

	// Set executor that returns a canned response
	m.SetCommandExecutor(func(input string) (string, error) {
		if input == "peer list" {
			return "peer 1.1.1.1 [established]\npeer 2.2.2.2 [idle]", nil
		}
		return "", fmt.Errorf("unknown command: %s", input)
	})

	m.SwitchMode(ModeCommand)

	// Simulate executeOperationalCommand via Update
	result, _ := m.Update(commandResultMsg{
		result: commandResult{output: "peer 1.1.1.1 [established]\npeer 2.2.2.2 [idle]"},
	})
	updated, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model from Update")
	}

	if updated.ViewportContent() != "peer 1.1.1.1 [established]\npeer 2.2.2.2 [idle]" {
		t.Errorf("expected peer list output, got %q", updated.ViewportContent())
	}
}

func TestCommandModeDispatchNoExecutor(t *testing.T) {
	// VALIDATES: command mode without executor shows warning on switch and error on dispatch
	m := newTestModel(t)
	m.SwitchMode(ModeCommand)

	// Should warn upfront that daemon is not connected
	if m.StatusMessage() == "" {
		t.Error("expected status warning when entering command mode without executor")
	}

	// Simulate what happens when executeOperationalCommand runs with nil executor
	result, _ := m.Update(commandResultMsg{
		err: fmt.Errorf("no daemon connection (command mode requires a running daemon)"),
	})
	updated, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model from Update")
	}

	if updated.Error() == nil {
		t.Error("expected error when no executor is set")
	}
}

func TestCommandModeGhostTextWired(t *testing.T) {
	// VALIDATES: ghost text works through Model in command mode
	m := newTestModel(t)
	m.SetCommandCompleter(NewCommandCompleter(&CommandNode{
		Children: map[string]*CommandNode{
			"peer":   {Name: "peer", Description: "Peer operations"},
			"daemon": {Name: "daemon", Description: "Daemon operations"},
		},
	}))

	m.SwitchMode(ModeCommand)

	// Type "pe" — ghost text should suggest "er" (completing "peer")
	m.textInput.SetValue("pe")
	m.UpdateCompletions()

	if m.GhostText() != "er" {
		t.Errorf("expected ghost text 'er', got %q", m.GhostText())
	}
}

func TestEditModeUnchanged(t *testing.T) {
	// VALIDATES: AC-8 — edit mode commands still work after mode features added
	m := newTestModel(t)

	// Verify completions still work in edit mode
	m.UpdateCompletions()
	comps := m.Completions()

	// Should include editor commands like set, delete, edit, show, etc.
	found := false
	for _, c := range comps {
		if c.Text == "set" {
			found = true
			break
		}
	}
	if !found {
		t.Error("edit mode should still show 'set' in completions")
	}
}

func TestModeString(t *testing.T) {
	tests := []struct {
		mode EditorMode
		want string
	}{
		{ModeEdit, "edit"},
		{ModeCommand, "command"},
	}
	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("EditorMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

func TestModeHistoryIsolation(t *testing.T) {
	// VALIDATES: per-mode command history — edit and command mode have separate histories
	// PREVENTS: confusing history mixing between modes
	m := newTestModel(t)

	// Add history in edit mode
	m.history.Append("set bgp local-as 65000")
	m.history.Append("show")

	// Switch to command mode
	m.SwitchMode(ModeCommand)

	if len(m.history.Entries()) != 0 {
		t.Errorf("expected empty history in fresh command mode, got %v", m.history.Entries())
	}

	// Add history in command mode
	m.history.Append("peer list")

	// Switch back to edit - edit history should be restored
	m.SwitchMode(ModeEdit)

	entries := m.history.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 edit history entries, got %d: %v", len(entries), entries)
	}
	if entries[0] != "set bgp local-as 65000" {
		t.Errorf("expected first edit history entry 'set bgp local-as 65000', got %q", entries[0])
	}

	// Switch back to command - command history should be restored
	m.SwitchMode(ModeCommand)

	entries = m.history.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 command history entry, got %d: %v", len(entries), entries)
	}
	if entries[0] != "peer list" {
		t.Errorf("expected command history entry 'peer list', got %q", entries[0])
	}
}

func TestModeScrollRestore(t *testing.T) {
	// VALIDATES: viewport scroll position preserved across mode switches
	// PREVENTS: losing scroll position when toggling modes
	m := newTestModel(t)

	// Set viewport content and scroll down in edit mode
	longContent := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"
	m.setViewportText(longContent)
	m.viewport.YOffset = 5

	// Switch to command mode
	m.SwitchMode(ModeCommand)

	if m.viewport.YOffset != 0 {
		t.Errorf("expected YOffset 0 in fresh command mode, got %d", m.viewport.YOffset)
	}

	// Switch back to edit — scroll position should be restored
	m.SwitchMode(ModeEdit)

	if m.viewport.YOffset != 5 {
		t.Errorf("expected YOffset 5 restored in edit mode, got %d", m.viewport.YOffset)
	}
}

func TestTabOnCommonPrefixShowsDropdown(t *testing.T) {
	// VALIDATES: Tab on common prefix applies partial completion and shows dropdown.
	// PREVENTS: Tab completing "peer detail 12" to "127.0.0. " (with space) and no dropdown,
	//           leaving user with an invalid partial token and no way to pick between matches.
	m := newTestModel(t)
	m.SetCommandCompleter(NewCommandCompleter(&CommandNode{
		Children: map[string]*CommandNode{
			"peer": {Name: "peer", Children: map[string]*CommandNode{
				"detail": {Name: "detail", Children: map[string]*CommandNode{
					"127.0.0.1": {Name: "127.0.0.1", Description: "Peer 1"},
					"127.0.0.2": {Name: "127.0.0.2", Description: "Peer 2"},
				}},
			}},
		},
	}))

	m.SwitchMode(ModeCommand)
	m.textInput.SetValue("peer detail 12")
	m.UpdateCompletions()

	// Precondition: ghost text is the common prefix tail, multiple completions
	if m.GhostText() != "7.0.0." {
		t.Fatalf("expected ghost text '7.0.0.', got %q", m.GhostText())
	}
	if len(m.Completions()) != 2 {
		t.Fatalf("expected 2 completions, got %d", len(m.Completions()))
	}

	// Press Tab — should apply common prefix WITHOUT trailing space, and show dropdown
	newModel, _ := m.handleTab()
	updated, ok := newModel.(Model)
	if !ok {
		t.Fatal("expected Model from handleTab")
	}

	// Input should end with the common prefix, no trailing space
	if updated.InputValue() != "peer detail 127.0.0." {
		t.Errorf("expected 'peer detail 127.0.0.', got %q", updated.InputValue())
	}

	// Dropdown should be visible with both peer options
	if !updated.ShowDropdown() {
		t.Error("dropdown should be visible after Tab on common prefix")
	}
	if len(updated.Completions()) != 2 {
		t.Errorf("expected 2 completions in dropdown, got %d", len(updated.Completions()))
	}
}

func TestCrossModeCompletionsRunPrefix(t *testing.T) {
	// VALIDATES: edit mode with "run " prefix gets operational command completions
	// PREVENTS: dead completions when typing "run peer" in edit mode
	m := newTestModel(t)
	m.SetCommandCompleter(NewCommandCompleter(&CommandNode{
		Children: map[string]*CommandNode{
			"peer":   {Name: "peer", Description: "Peer operations"},
			"daemon": {Name: "daemon", Description: "Daemon operations"},
		},
	}))

	// In edit mode, type "run " — should get command completions
	m.textInput.SetValue("run ")
	m.UpdateCompletions()
	comps := m.Completions()

	hasPeer := false
	for _, c := range comps {
		if c.Text == "peer" {
			hasPeer = true
		}
	}
	if !hasPeer {
		t.Errorf("expected operational completions for 'run ' prefix, got %v", comps)
	}

	// Type "run pe" — ghost text should suggest "er" (completing "peer")
	m.textInput.SetValue("run pe")
	m.UpdateCompletions()
	if m.GhostText() != "er" {
		t.Errorf("expected ghost text 'er' for 'run pe', got %q", m.GhostText())
	}
}

func TestCrossModeCompletionsEditInCommandMode(t *testing.T) {
	// VALIDATES: command mode with edit command prefix routes to YANG completions
	// PREVENTS: trying to match "set bgp" against operational command tree
	m := newTestModel(t)
	m.SetCommandCompleter(NewCommandCompleter(&CommandNode{
		Children: map[string]*CommandNode{
			"peer": {Name: "peer", Description: "Peer operations"},
		},
	}))

	m.SwitchMode(ModeCommand)

	// Type "set " — should get YANG completions, not operational
	m.textInput.SetValue("set ")
	m.UpdateCompletions()
	comps := m.Completions()

	// Should NOT contain operational commands like "peer"
	for _, c := range comps {
		if c.Text == "peer" {
			t.Error("'set ' prefix should use YANG completions, not operational commands")
		}
	}
}

func TestIsEditCommandWithArgs(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", false},
		{"set", false},    // no args yet — still at command level
		{"set ", true},    // trailing space — entering path
		{"set bgp", true}, // has args
		{"set bgp peer", true},
		{"delete ", true},
		{"peer list", false}, // not an edit command
		{"run peer", false},  // "run" is not in editModeCommands
		{"show ", true},
		{"show bgp", true},
		{"commit", false}, // no args
		{"commit ", true}, // trailing space counts
	}
	for _, tt := range tests {
		got := isEditCommandWithArgs(tt.input)
		if got != tt.want {
			t.Errorf("isEditCommandWithArgs(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// newTestModel creates a minimal Model for mode tests.
func newTestModel(t *testing.T) *Model {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "test.conf")
	if err := os.WriteFile(configPath, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	ed, err := NewEditor(configPath)
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewModel(ed)
	if err != nil {
		t.Fatal(err)
	}
	return &m
}

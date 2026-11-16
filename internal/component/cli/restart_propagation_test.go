package cli

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestRestartFuncSurvivesUpdateChain verifies that commandExecutor set via
// SetCommandExecutor survives the bubbletea Update copy chain.
// VALIDATES: .et lifecycle infrastructure -- commandExecutor propagation.
// PREVENTS: commandExecutor lost during Update, causing "restart not available".
func TestRestartFuncSurvivesUpdateChain(t *testing.T) {
	m := NewCommandModel(FilesystemAuthorityOperatorLocal)

	// Initialize with window size (same as NewHeadlessCommandModel).
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = newM.(Model) //nolint:errcheck // type assertion always succeeds

	// Set commandExecutor.
	m.SetCommandExecutor(func(string) (CommandOutput, error) { return CommandOutput{}, nil })

	// Verify it's set before any rune updates.
	if m.commandExecutor == nil {
		t.Fatal("commandExecutor nil immediately after SetCommandExecutor")
	}

	// Type "restart" character by character (same as TypeText).
	for i, r := range "restart" {
		newM, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m, _ = newM.(Model) //nolint:errcheck // type assertion always succeeds
		if m.commandExecutor == nil {
			t.Fatalf("commandExecutor nil after typing character %d (%c)", i, r)
		}
	}

	// Press Enter.
	newM, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m, _ = newM.(Model) //nolint:errcheck // type assertion always succeeds

	t.Logf("status after Enter: %q", m.statusMessage)

	if m.commandExecutor == nil {
		t.Error("commandExecutor nil after Enter -- lost during Update chain")
	}
	if m.statusMessage == msgRestartNotAvailable {
		t.Error("got 'not available' despite commandExecutor being set")
	}
}

// TestRestartFuncSurvivesWithCmdProcessing verifies commandExecutor survives
// when commands returned by Update are also processed (same as headless model).
// VALIDATES: .et lifecycle -- commandExecutor through processCmd chain.
// PREVENTS: command callback replacing model without commandExecutor.
func TestRestartFuncSurvivesWithCmdProcessing(t *testing.T) {
	m := NewCommandModel(FilesystemAuthorityOperatorLocal)

	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = newM.(Model) //nolint:errcheck // type assertion always succeeds
	m.SetCommandExecutor(func(string) (CommandOutput, error) { return CommandOutput{}, nil })

	// Process each character AND its returned command (simulates headless model).
	for i, r := range "restart" {
		newM, cmd := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m, _ = newM.(Model) //nolint:errcheck // type assertion always succeeds

		// Process command (same as headless processCmd).
		if cmd != nil {
			msg := cmd()
			if msg != nil {
				newM, _ = m.Update(msg)
				m, _ = newM.(Model) //nolint:errcheck // type assertion always succeeds
			}
		}

		if m.commandExecutor == nil {
			t.Fatalf("commandExecutor nil after char %d (%c) + cmd processing", i, r)
		}
	}

	// Press Enter + process cmd.
	newM, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m, _ = newM.(Model) //nolint:errcheck // type assertion always succeeds
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			newM, _ = m.Update(msg)
			m, _ = newM.(Model) //nolint:errcheck // type assertion always succeeds
		}
	}

	t.Logf("status after Enter+cmd: %q, commandExecutor nil: %v", m.statusMessage, m.commandExecutor == nil)

	if m.statusMessage == msgRestartNotAvailable {
		t.Error("got 'not available' despite commandExecutor being set -- command processing lost it")
	}
}

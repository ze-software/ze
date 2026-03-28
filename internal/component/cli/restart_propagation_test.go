package cli

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestRestartFuncSurvivesUpdateChain verifies that restartFunc set via
// SetRestartFunc survives the bubbletea Update copy chain.
// VALIDATES: .et lifecycle infrastructure -- restartFunc propagation.
// PREVENTS: restartFunc lost during Update, causing "restart not available".
func TestRestartFuncSurvivesUpdateChain(t *testing.T) {
	m := NewCommandModel()

	// Initialize with window size (same as NewHeadlessCommandModel).
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = newM.(Model) //nolint:errcheck // type assertion always succeeds

	// Set restartFunc.
	m.SetRestartFunc(func() {})

	// Verify it's set before any rune updates.
	if m.restartFunc == nil {
		t.Fatal("restartFunc nil immediately after SetRestartFunc")
	}

	// Type "restart" character by character (same as TypeText).
	for i, r := range "restart" {
		newM, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m, _ = newM.(Model) //nolint:errcheck // type assertion always succeeds
		if m.restartFunc == nil {
			t.Fatalf("restartFunc nil after typing character %d (%c)", i, r)
		}
	}

	// Press Enter.
	newM, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m, _ = newM.(Model) //nolint:errcheck // type assertion always succeeds

	t.Logf("status after Enter: %q", m.statusMessage)

	if m.restartFunc == nil {
		t.Error("restartFunc nil after Enter -- lost during Update chain")
	}
	if m.statusMessage == msgRestartNotAvailable {
		t.Error("got 'not available' despite restartFunc being set")
	}
}

// TestRestartFuncSurvivesWithCmdProcessing verifies restartFunc survives
// when commands returned by Update are also processed (same as headless model).
// VALIDATES: .et lifecycle -- restartFunc through processCmd chain.
// PREVENTS: command callback replacing model without restartFunc.
func TestRestartFuncSurvivesWithCmdProcessing(t *testing.T) {
	m := NewCommandModel()

	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = newM.(Model) //nolint:errcheck // type assertion always succeeds
	m.DisableBlink()
	m.SetRestartFunc(func() {})

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

		if m.restartFunc == nil {
			t.Fatalf("restartFunc nil after char %d (%c) + cmd processing", i, r)
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

	t.Logf("status after Enter+cmd: %q, restartFunc nil: %v", m.statusMessage, m.restartFunc == nil)

	if m.statusMessage == msgRestartNotAvailable {
		t.Error("got 'not available' despite restartFunc being set -- command processing lost it")
	}
}

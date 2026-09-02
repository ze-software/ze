package client

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	unicli "github.com/ze-software/ze/internal/component/cli"
)

// attachedTestConfig is a config the YANG schema accepts, so the editor parses
// it and the assembled model has a tree to edit.
const attachedTestConfig = "set system host-name attached\n"

// attachedTestDispatch answers every command. The runtime command tree falls
// back to the compiled one because this text is not the JSON command list.
func attachedTestDispatch(string) (unicli.CommandOutput, error) {
	return unicli.CommandOutput{Text: "ok"}, nil
}

// attachedTestEditor builds a storage-backed editor over a temporary config.
func attachedTestEditor(t *testing.T) *unicli.Editor {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.conf")
	if err := os.WriteFile(path, []byte(attachedTestConfig), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	ed, err := unicli.NewEditor(path)
	if err != nil {
		t.Fatalf("new editor: %v", err)
	}
	t.Cleanup(func() { ed.Close() }) //nolint:errcheck // test cleanup
	return ed
}

// pressEnter types the command and sends Enter through the model's own update
// loop, which is the path a keystroke takes in the running console.
func pressEnter(t *testing.T, m *unicli.Model, command string) unicli.Model {
	t.Helper()
	m.SetInput(command)
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated, ok := next.(unicli.Model)
	if !ok {
		t.Fatal("Update must return a cli.Model")
	}
	return updated
}

// TestAttachedModelOpensOperational: the attached console keeps its operational
// character. The editor gives it config mode, and `configure` is what reaches
// it, exactly as over SSH.
//
// VALIDATES: `ze start --cli` opens at the operational prompt.
// PREVENTS: the attach console opening in the config editor.
func TestAttachedModelOpensOperational(t *testing.T) {
	m := newAttachedModel(attachedTestDispatch, unicli.CommandExecutor(attachedTestDispatch), attachedTestEditor(t))

	if m.Mode() != unicli.ModeOperational {
		t.Errorf("mode = %v, want operational", m.Mode())
	}
}

// TestAttachedModelConfigureReachesConfigMode: typing `configure` in the
// attached console reaches config mode, which is the whole point of giving the
// console an editor.
//
// VALIDATES: `ze start --cli` reaches configuration mode.
// PREVENTS: the attach console answering "config mode not available".
func TestAttachedModelConfigureReachesConfigMode(t *testing.T) {
	m := newAttachedModel(attachedTestDispatch, unicli.CommandExecutor(attachedTestDispatch), attachedTestEditor(t))

	updated := pressEnter(t, &m, "configure")

	if updated.Mode() != unicli.ModeConfig {
		t.Errorf("mode = %v after configure, want config (status %q)", updated.Mode(), updated.StatusMessage())
	}
}

// TestAttachedModelWithoutEditorRefusesConfigure: a caller with no editor gets
// the command-only console and the refusal it has always answered.
//
// VALIDATES: a nil editor keeps the previous behavior.
// PREVENTS: a nil editor reaching config mode with no config to edit.
func TestAttachedModelWithoutEditorRefusesConfigure(t *testing.T) {
	m := newAttachedModel(attachedTestDispatch, unicli.CommandExecutor(attachedTestDispatch), nil)

	updated := pressEnter(t, &m, "configure")

	if updated.Mode() != unicli.ModeOperational {
		t.Errorf("mode = %v after configure with no editor, want operational", updated.Mode())
	}
	const want = "config mode not available (no config file loaded)"
	if updated.StatusMessage() != want {
		t.Errorf("status = %q, want %q", updated.StatusMessage(), want)
	}
}

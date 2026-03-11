package testing

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testConfig = `bgp {
  local-as 65000
  router-id 1.2.3.4
  peer 1.1.1.1 {
    peer-as 65001
  }
}
`

// TestHeadlessModelCreate verifies headless model creation.
//
// VALIDATES: Headless model can be created from config file.
// PREVENTS: Test framework can't initialize cli.
func TestHeadlessModelCreate(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testConfig), 0o600)
	require.NoError(t, err)

	hm, err := NewHeadlessModel(configPath)
	require.NoError(t, err)
	require.NotNil(t, hm)

	// Should have model state
	assert.NotNil(t, hm.Model())
}

// TestHeadlessModelSendKey verifies key message sending.
//
// VALIDATES: Key messages are processed by the model.
// PREVENTS: Input not reaching cli.
func TestHeadlessModelSendKey(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testConfig), 0o600)
	require.NoError(t, err)

	hm, err := NewHeadlessModel(configPath)
	require.NoError(t, err)

	// Send some key messages
	err = hm.SendMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	require.NoError(t, err)
	err = hm.SendMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	require.NoError(t, err)

	// Input should be captured (model processes it)
	// We can verify by checking the text input contains "ed"
	assert.Contains(t, hm.InputValue(), "ed")
}

// TestHeadlessModelContext verifies context path access.
//
// VALIDATES: Context path is accessible after navigation.
// PREVENTS: Context assertions failing.
func TestHeadlessModelContext(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testConfig), 0o600)
	require.NoError(t, err)

	hm, err := NewHeadlessModel(configPath)
	require.NoError(t, err)

	// Initially at root
	assert.Empty(t, hm.ContextPath())

	// Type "edit bgp" and press enter
	hm.TypeText("edit bgp")
	hm.PressEnter()

	// Should be in bgp context
	assert.Equal(t, []string{"bgp"}, hm.ContextPath())
}

// TestHeadlessModelCompletions verifies completion access.
//
// VALIDATES: Completions are accessible after input.
// PREVENTS: Completion assertions failing.
func TestHeadlessModelCompletions(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testConfig), 0o600)
	require.NoError(t, err)

	hm, err := NewHeadlessModel(configPath)
	require.NoError(t, err)

	// Type "set " to trigger completions
	hm.TypeText("set ")

	comps := hm.Completions()
	// Should have YANG-based completions
	assert.NotEmpty(t, comps)
}

// TestHeadlessModelGhostText verifies ghost text access.
//
// VALIDATES: Ghost text is accessible.
// PREVENTS: Ghost text assertions failing.
func TestHeadlessModelGhostText(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testConfig), 0o600)
	require.NoError(t, err)

	hm, err := NewHeadlessModel(configPath)
	require.NoError(t, err)

	// Ghost text is available through accessor
	// (actual content depends on input state)
	_ = hm.GhostText() // Should not panic
}

// TestHeadlessModelValidationErrors verifies error access.
//
// VALIDATES: Validation errors are accessible.
// PREVENTS: Error count assertions failing.
func TestHeadlessModelValidationErrors(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	// Write valid config
	err := os.WriteFile(configPath, []byte(testConfig), 0o600)
	require.NoError(t, err)

	hm, err := NewHeadlessModel(configPath)
	require.NoError(t, err)

	// Valid config should have no errors
	assert.Empty(t, hm.ValidationErrors())
}

// TestHeadlessModelDirty verifies dirty flag access.
//
// VALIDATES: Dirty flag is accessible.
// PREVENTS: Dirty state assertions failing.
func TestHeadlessModelDirty(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testConfig), 0o600)
	require.NoError(t, err)

	hm, err := NewHeadlessModel(configPath)
	require.NoError(t, err)

	// Initially not dirty
	assert.False(t, hm.Dirty())
}

// TestHeadlessModelStatusMessage verifies status message access.
//
// VALIDATES: Status message is accessible after commands.
// PREVENTS: Status message assertions failing.
func TestHeadlessModelStatusMessage(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testConfig), 0o600)
	require.NoError(t, err)

	hm, err := NewHeadlessModel(configPath)
	require.NoError(t, err)

	// Status message accessor should work
	_ = hm.StatusMessage() // Should not panic
}

// TestHeadlessModelError verifies command error access.
//
// VALIDATES: Command errors are accessible.
// PREVENTS: Error message assertions failing.
func TestHeadlessModelError(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testConfig), 0o600)
	require.NoError(t, err)

	hm, err := NewHeadlessModel(configPath)
	require.NoError(t, err)

	// Initially no error
	assert.Nil(t, hm.Error())

	// Execute invalid command
	hm.TypeText("invalidcmd")
	hm.PressEnter()

	// Should have error
	assert.NotNil(t, hm.Error())
	assert.Contains(t, hm.Error().Error(), "unknown")
}

// TestHeadlessModelIsTemplate verifies template mode access.
//
// VALIDATES: Template mode flag is accessible.
// PREVENTS: Template mode assertions failing.
func TestHeadlessModelIsTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testConfig), 0o600)
	require.NoError(t, err)

	hm, err := NewHeadlessModel(configPath)
	require.NoError(t, err)

	// Initially not in template mode
	assert.False(t, hm.IsTemplate())
}

// TestHeadlessModelShowDropdown verifies dropdown visibility access.
//
// VALIDATES: Dropdown visibility is accessible.
// PREVENTS: Dropdown assertions failing.
func TestHeadlessModelShowDropdown(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testConfig), 0o600)
	require.NoError(t, err)

	hm, err := NewHeadlessModel(configPath)
	require.NoError(t, err)

	// Initially dropdown not showing
	assert.False(t, hm.ShowDropdown())
}

// TestHeadlessModelWorkingContent verifies content access.
//
// VALIDATES: Working content is accessible.
// PREVENTS: Content assertions failing.
func TestHeadlessModelWorkingContent(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testConfig), 0o600)
	require.NoError(t, err)

	hm, err := NewHeadlessModel(configPath)
	require.NoError(t, err)

	// Should have the original content
	content := hm.WorkingContent()
	assert.Contains(t, content, "bgp")
	assert.Contains(t, content, "local-as 65000")
}

// TestHeadlessModelTypeAndEnter verifies helper methods.
//
// VALIDATES: TypeText and PressEnter helpers work.
// PREVENTS: Verbose test code for simple operations.
func TestHeadlessModelTypeAndEnter(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testConfig), 0o600)
	require.NoError(t, err)

	hm, err := NewHeadlessModel(configPath)
	require.NoError(t, err)

	// Type and execute command
	hm.TypeText("top")
	assert.Contains(t, hm.InputValue(), "top")

	hm.PressEnter()
	// Input should be cleared after command
	assert.Empty(t, hm.InputValue())
}

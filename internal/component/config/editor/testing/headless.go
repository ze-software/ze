// Design: docs/architecture/config/yang-config-design.md — editor test infrastructure

package testing

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"codeberg.org/thomas-mangin/ze/internal/component/config/editor"
)

// HeadlessModel wraps the editor Model for headless testing.
// It provides direct access to model state without TTY rendering.
type HeadlessModel struct {
	model  editor.Model
	editor *editor.Editor
}

// NewHeadlessModel creates a headless model from a config file path.
func NewHeadlessModel(configPath string) (*HeadlessModel, error) {
	ed, err := editor.NewEditor(configPath)
	if err != nil {
		return nil, fmt.Errorf("creating editor: %w", err)
	}

	model, err := editor.NewModel(ed)
	if err != nil {
		return nil, fmt.Errorf("creating model: %w", err)
	}

	// Initialize with a reasonable window size - don't process the Init command
	// as it contains cursor blink which blocks forever
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m, ok := newModel.(editor.Model); ok {
		model = m
	}

	hm := &HeadlessModel{
		model:  model,
		editor: ed,
	}

	// Trigger initial completion population
	hm.model.UpdateCompletions()

	return hm, nil
}

// Model returns the underlying editor.Model.
func (hm *HeadlessModel) Model() *editor.Model {
	return &hm.model
}

// SendMsg sends a tea.Msg to the model and processes it.
func (hm *HeadlessModel) SendMsg(msg tea.Msg) error {
	newModel, cmd := hm.model.Update(msg)
	model, ok := newModel.(editor.Model)
	if !ok {
		return fmt.Errorf("update returned unexpected type: %T", newModel)
	}
	hm.model = model

	// Process any commands that return messages
	hm.processCmd(cmd)

	return nil
}

// processCmd processes commands that return messages, with depth limiting.
func (hm *HeadlessModel) processCmd(cmd tea.Cmd) {
	hm.processCmdWithDepth(cmd, 0)
}

// processCmdWithDepth processes commands with depth tracking to prevent infinite loops.
// For headless testing, we use a goroutine with timeout to handle blocking commands.
func (hm *HeadlessModel) processCmdWithDepth(cmd tea.Cmd, depth int) {
	if cmd == nil || depth > 5 {
		return // Depth limit to prevent infinite recursion
	}

	// Execute the command in a goroutine with timeout
	done := make(chan tea.Msg, 1)
	go func() {
		done <- cmd()
	}()

	// Wait for the command to complete. Most commands (including file I/O
	// for commit/load) finish quickly, but cursor blink and other blocking
	// commands need to be skipped. 50ms is long enough for I/O under the
	// race detector, but short enough that accumulated timeouts from many
	// cursor blinks don't cause the test to time out (ET tests generate
	// thousands of keystrokes, each potentially spawning a blink command).
	select {
	case msg := <-done:
		if msg == nil {
			return
		}
		hm.processMsg(msg, depth)
	case <-time.After(50 * time.Millisecond):
		// Command would block (like cursor blink), skip it
		return
	}
}

// processMsg processes a message from a command.
func (hm *HeadlessModel) processMsg(msg tea.Msg, depth int) {
	// Handle batch commands
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			hm.processCmdWithDepth(c, depth+1)
		}
		return
	}

	// Skip WindowSizeMsg - already set at initialization
	if _, ok := msg.(tea.WindowSizeMsg); ok {
		return
	}

	// Process the resulting message
	newModel, nextCmd := hm.model.Update(msg)
	if model, ok := newModel.(editor.Model); ok {
		hm.model = model
	}

	// Recursively process resulting commands with depth limit
	if nextCmd != nil && depth < 3 {
		hm.processCmdWithDepth(nextCmd, depth+1)
	}
}

// TypeText sends each character as a KeyRunes message.
func (hm *HeadlessModel) TypeText(text string) {
	for _, r := range text {
		_ = hm.SendMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

// PressEnter sends an Enter key message.
func (hm *HeadlessModel) PressEnter() {
	_ = hm.SendMsg(tea.KeyMsg{Type: tea.KeyEnter})
}

// PressTab sends a Tab key message.
func (hm *HeadlessModel) PressTab() {
	_ = hm.SendMsg(tea.KeyMsg{Type: tea.KeyTab})
}

// PressEsc sends an Escape key message.
func (hm *HeadlessModel) PressEsc() {
	_ = hm.SendMsg(tea.KeyMsg{Type: tea.KeyEscape})
}

// ClearInput sends Ctrl+U to clear the input line.
func (hm *HeadlessModel) ClearInput() {
	_ = hm.SendMsg(tea.KeyMsg{Type: tea.KeyCtrlU})
}

// --- State Accessors ---

// InputValue returns the current text input value.
func (hm *HeadlessModel) InputValue() string {
	return hm.model.InputValue()
}

// ContextPath returns the current context path.
func (hm *HeadlessModel) ContextPath() []string {
	return hm.model.ContextPath()
}

// Completions returns the current completion list.
func (hm *HeadlessModel) Completions() []editor.Completion {
	return hm.model.Completions()
}

// GhostText returns the current ghost text suggestion.
func (hm *HeadlessModel) GhostText() string {
	return hm.model.GhostText()
}

// ValidationErrors returns the current validation errors.
func (hm *HeadlessModel) ValidationErrors() []editor.ConfigValidationError {
	return hm.model.ValidationErrors()
}

// ValidationWarnings returns the current validation warnings.
func (hm *HeadlessModel) ValidationWarnings() []editor.ConfigValidationError {
	return hm.model.ValidationWarnings()
}

// Dirty returns true if there are unsaved changes.
func (hm *HeadlessModel) Dirty() bool {
	return hm.model.Dirty()
}

// StatusMessage returns the current status message.
func (hm *HeadlessModel) StatusMessage() string {
	return hm.model.StatusMessage()
}

// Error returns the current command error.
func (hm *HeadlessModel) Error() error {
	return hm.model.Error()
}

// IsTemplate returns true if in template editing mode.
func (hm *HeadlessModel) IsTemplate() bool {
	return hm.model.IsTemplate()
}

// ShowDropdown returns true if the completion dropdown is visible.
func (hm *HeadlessModel) ShowDropdown() bool {
	return hm.model.ShowDropdown()
}

// WorkingContent returns the current working config content.
func (hm *HeadlessModel) WorkingContent() string {
	return hm.editor.WorkingContent()
}

// SelectedIndex returns the currently selected dropdown index.
func (hm *HeadlessModel) SelectedIndex() int {
	return hm.model.SelectedIndex()
}

// ViewportContent returns the content currently displayed in the viewport.
func (hm *HeadlessModel) ViewportContent() string {
	return hm.model.ViewportContent()
}

// ConfirmTimerActive returns true if a commit confirm timer is active.
func (hm *HeadlessModel) ConfirmTimerActive() bool {
	return hm.model.ConfirmTimerActive()
}

// TriggerCompletions forces an update of the completion list.
func (hm *HeadlessModel) TriggerCompletions() {
	hm.model.UpdateCompletions()
}

// Mode returns the current editor mode.
func (hm *HeadlessModel) Mode() editor.EditorMode {
	return hm.model.Mode()
}

// SetReloadNotifier configures a reload notifier on the underlying editor.
// Used by the .et test runner to simulate daemon reload behavior.
func (hm *HeadlessModel) SetReloadNotifier(fn editor.ReloadNotifier) {
	hm.editor.SetReloadNotifier(fn)
}

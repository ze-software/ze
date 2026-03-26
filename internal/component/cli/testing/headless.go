// Design: docs/architecture/config/yang-config-design.md — editor test infrastructure

package testing

import (
	"fmt"
	"runtime"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"codeberg.org/thomas-mangin/ze/internal/component/cli"
)

// HeadlessModel wraps the editor Model for headless testing.
// It provides direct access to model state without TTY rendering.
type HeadlessModel struct {
	model   cli.Model
	editor  *cli.Editor
	pending []<-chan tea.Msg // timer commands that exceeded the processing deadline
	tmpDir  string           // temp directory for file expectations
}

// NewHeadlessModel creates a headless model from a config file path.
func NewHeadlessModel(configPath string) (*HeadlessModel, error) {
	ed, err := cli.NewEditor(configPath)
	if err != nil {
		return nil, fmt.Errorf("creating editor: %w", err)
	}

	model, err := cli.NewModel(ed)
	if err != nil {
		return nil, fmt.Errorf("creating model: %w", err)
	}

	// Initialize with a reasonable window size - don't process the Init command
	// as it contains cursor blink which blocks forever
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m, ok := newModel.(cli.Model); ok {
		model = m
	}

	hm := &HeadlessModel{
		model:  model,
		editor: ed,
	}

	// Disable cursor blink: eliminates ~530ms timer goroutines per keystroke
	// that serve no purpose in headless mode.
	hm.model.DisableBlink()

	// Trigger initial completion population
	hm.model.UpdateCompletions()

	return hm, nil
}

// NewHeadlessModelWithSession creates a headless model with session identity activated.
func NewHeadlessModelWithSession(configPath, user, origin string) (*HeadlessModel, error) {
	ed, err := cli.NewEditor(configPath)
	if err != nil {
		return nil, fmt.Errorf("creating editor: %w", err)
	}

	session := cli.NewEditSession(user, origin)
	ed.SetSession(session)

	model, err := cli.NewModel(ed)
	if err != nil {
		return nil, fmt.Errorf("creating model: %w", err)
	}

	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m, ok := newModel.(cli.Model); ok {
		model = m
	}

	hm := &HeadlessModel{
		model:  model,
		editor: ed,
	}

	hm.model.DisableBlink()
	hm.model.UpdateCompletions()

	return hm, nil
}

// NewHeadlessCommandModel creates a command-only headless model (no editor).
// Used for testing ze cli behavior where no config file is loaded.
func NewHeadlessCommandModel() *HeadlessModel {
	model := cli.NewCommandModel()

	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m, ok := newModel.(cli.Model); ok {
		model = m
	}

	hm := &HeadlessModel{
		model: model,
	}

	hm.model.DisableBlink()

	return hm
}

// TmpDir returns the temp directory for file expectations.
func (hm *HeadlessModel) TmpDir() string {
	return hm.tmpDir
}

// SetTmpDir sets the temp directory for file expectations.
func (hm *HeadlessModel) SetTmpDir(dir string) {
	hm.tmpDir = dir
}

// Model returns the underlying cli.Model.
func (hm *HeadlessModel) Model() *cli.Model {
	return &hm.model
}

// SendMsg sends a tea.Msg to the model and processes it.
func (hm *HeadlessModel) SendMsg(msg tea.Msg) error {
	// Drain completed pending timer commands before model.Update().
	// Timer goroutines don't access editor/model state (they only sit in
	// time.After), so draining them here is race-free.
	hm.settle()

	newModel, cmd := hm.model.Update(msg)
	model, ok := newModel.(cli.Model)
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
//
// Each command runs in a goroutine so blocking timer commands (validation tick,
// confirm countdown, draft poll) don't stall the test. The goroutine either:
//   - completes and we process the result synchronously (no concurrent access), or
//   - exceeds the deadline and is orphaned (timer-only; doesn't access state).
//
// No pending/drain mechanism: every command either completes within this call
// or is abandoned. This eliminates races where a still-running goroutine writes
// editor state while a later drain triggers model.Update() which reads it.
func (hm *HeadlessModel) processCmdWithDepth(cmd tea.Cmd, depth int) {
	if cmd == nil || depth > 5 {
		return // Depth limit to prevent infinite recursion
	}

	// Execute the command in a goroutine with timeout.
	done := make(chan tea.Msg, 1)
	go func() {
		done <- cmd()
	}()

	// Fast path: yield multiple times for microsecond-level commands
	// (tree ops, config lookups, batch wrappers). Most commands complete
	// during these yields and we pick up the result without hitting the
	// timer.
	for range 10 {
		runtime.Gosched()
		select {
		case msg := <-done:
			if msg != nil {
				hm.processMsg(msg, depth)
			}
			return
		default: // non-blocking check: continue yielding if not ready yet
		}
	}

	// Slow path: wait for state-mutating commands (set, save, commit, load --
	// including file I/O under race detector with 140 concurrent tests).
	// 900ms catches all state-mutating commands even under extreme I/O
	// contention. Only pure timer commands (confirm countdown ~1s, draft
	// poll ~2s) exceed this; they go to pending and are drained later by
	// settle/SettleWait. Timer goroutines only block in time.After, never
	// accessing model/editor state, so draining them later is race-free.
	// Cursor blink is eliminated via DisableBlink().
	select {
	case msg := <-done:
		if msg != nil {
			hm.processMsg(msg, depth)
		}
	case <-time.After(900 * time.Millisecond):
		hm.pending = append(hm.pending, done)
		return
	}
}

// settle non-blocking drains timer commands that have completed since they
// were added to pending. Called before model.Update() in SendMsg.
func (hm *HeadlessModel) settle() {
	if len(hm.pending) == 0 {
		return
	}
	remaining := hm.pending[:0]
	for _, ch := range hm.pending {
		select {
		case msg := <-ch:
			if msg != nil {
				hm.processMsg(msg, 0)
			}
		default: // channel not ready: timer goroutine still in time.After
			remaining = append(remaining, ch)
		}
	}
	hm.pending = remaining
}

// SettleWait blocks until pending timer commands complete or deadline expires.
// Called before expectation checks to ensure timer-driven state (countdown
// ticks, draft poll, validation debounce) has been applied.
func (hm *HeadlessModel) SettleWait() {
	if len(hm.pending) == 0 {
		return
	}

	deadline := time.Now().Add(3 * time.Second)

	for len(hm.pending) > 0 && time.Now().Before(deadline) {
		drained := false
		remaining := hm.pending[:0]
		for _, ch := range hm.pending {
			select {
			case msg := <-ch:
				if msg != nil {
					hm.processMsg(msg, 0)
				}
				drained = true
			default: // channel not ready: timer goroutine still in time.After
				remaining = append(remaining, ch)
			}
		}
		hm.pending = remaining

		if !drained && len(hm.pending) > 0 {
			time.Sleep(5 * time.Millisecond)
		}
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
	if model, ok := newModel.(cli.Model); ok {
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
func (hm *HeadlessModel) Completions() []cli.Completion {
	return hm.model.Completions()
}

// GhostText returns the current ghost text suggestion.
func (hm *HeadlessModel) GhostText() string {
	return hm.model.GhostText()
}

// ValidationErrors returns the current validation errors.
func (hm *HeadlessModel) ValidationErrors() []cli.ConfigValidationError {
	return hm.model.ValidationErrors()
}

// ValidationWarnings returns the current validation warnings.
func (hm *HeadlessModel) ValidationWarnings() []cli.ConfigValidationError {
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
// Returns empty string for command-only models (no editor).
func (hm *HeadlessModel) WorkingContent() string {
	if hm.editor == nil {
		return ""
	}
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
func (hm *HeadlessModel) Mode() cli.EditorMode {
	return hm.model.Mode()
}

// SetReloadNotifier configures a reload notifier on the underlying cli.
// Used by the .et test runner to simulate daemon reload behavior.
// No-op for command-only models (no editor).
func (hm *HeadlessModel) SetReloadNotifier(fn cli.ReloadNotifier) {
	if hm.editor == nil {
		return
	}
	hm.editor.SetReloadNotifier(fn)
}

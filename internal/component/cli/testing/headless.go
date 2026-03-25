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
	pending []<-chan tea.Msg // commands that timed out but may still complete
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

	hm.model.UpdateCompletions()

	return hm, nil
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
	// Drain any completed pending commands before model.Update() to
	// prevent data races between orphaned cmd goroutines (still
	// accessing editor state) and the upcoming Update call.
	hm.Settle()

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
// For headless testing, we use a goroutine with timeout to handle blocking commands.
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
	// timer. A single Gosched is insufficient under heavy CPU load or
	// GC pressure — the scheduler may not pick the command goroutine
	// on the first yield when many timer goroutines (from per-keystroke
	// cursor blink and validation debounce) are competing.
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

	// Slow path: command didn't complete during yields — likely a blocking
	// timer (cursor blink ~530ms, validation tick ~100ms, confirm
	// countdown ~1s). 50ms catches file I/O that takes longer under
	// concurrent test load with race detector overhead. Only pure timer
	// commands (which sit in time.After, not accessing model state)
	// should exceed this window.
	select {
	case msg := <-done:
		if msg != nil {
			hm.processMsg(msg, depth)
		}
	case <-time.After(50 * time.Millisecond):
		// Command didn't complete in time — likely a blocking timer.
		// Save the channel so SettleWait() can drain the result
		// before expectation checks.
		hm.pending = append(hm.pending, done)
		return
	}
}

// Settle non-blocking drains any commands that timed out in
// processCmdWithDepth but have since completed. Called before
// model.Update() in SendMsg to prevent data races between orphaned
// goroutines and new Update calls.
func (hm *HeadlessModel) Settle() {
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
		default: // channel not ready: timer goroutine still in time.After — retain for later drain
			remaining = append(remaining, ch)
		}
	}
	hm.pending = remaining
}

// SettleWait blocks until pending commands complete or timeout expires.
// Under concurrent test load with race detector, file I/O commands can
// exceed the processCmdWithDepth timeout. SettleWait polls pending
// channels with brief sleeps, giving goroutines time to finish.
// Timer commands (cursor blink ~530ms) will not complete within the
// deadline and remain in pending — this is expected and safe since
// they only sit in time.After, not accessing model state.
func (hm *HeadlessModel) SettleWait() {
	if len(hm.pending) == 0 {
		return
	}

	deadline := time.Now().Add(2 * time.Second)

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
			default: // channel not ready: timer goroutine still in time.After — retain for later drain
				remaining = append(remaining, ch)
			}
		}
		hm.pending = remaining

		if !drained && len(hm.pending) > 0 {
			// Yield to let goroutines complete, then retry.
			// 5ms gives sufficient time under race-detector overhead.
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
func (hm *HeadlessModel) Mode() cli.EditorMode {
	return hm.model.Mode()
}

// SetReloadNotifier configures a reload notifier on the underlying cli.
// Used by the .et test runner to simulate daemon reload behavior.
func (hm *HeadlessModel) SetReloadNotifier(fn cli.ReloadNotifier) {
	hm.editor.SetReloadNotifier(fn)
}

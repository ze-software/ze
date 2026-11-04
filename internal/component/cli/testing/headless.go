// Design: docs/architecture/config/yang-config-design.md — editor test infrastructure

package testing

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/yang"
)

// errCommandTreeIsEmpty says the YANG modules that declare commands did not
// reach this binary. No command then exists to complete against.
//
// It is an error rather than an empty tree. An empty tree answers every
// completion expectation with an empty list. That passes an .et file which
// asserts nothing was found, and reports coverage the test never had.
var errCommandTreeIsEmpty = errors.New(
	"the command tree holds no command: no -cmd YANG module registered, " +
		"so the binary running this test is missing its plugin/all import")

// headlessCommandTree answers the command tree an operational-mode test
// completes against, built from the registered -cmd YANG modules. It is the
// same tree `ze cli` builds, so a test asserts on the commands an operator
// really types.
//
// It is built once for the package: BuildCommandTree parses every registered
// module, and one .et file creates a model for each session it opens.
//
// The tree carries no value hints and no plugin commands. A hint provider reads
// the live RIB, and a plugin command comes from the live dispatcher. A headless
// test has neither, so a completion over a value stays empty here.
var headlessCommandTree = sync.OnceValues(func() (*command.Node, error) {
	schema, err := yang.DefaultLoader()
	if err != nil {
		return nil, fmt.Errorf("loading the YANG modules for the command tree: %w", err)
	}
	tree := yang.BuildCommandTree(schema)
	if len(tree.Children) == 0 {
		return nil, errCommandTreeIsEmpty
	}
	return tree, nil
})

// headlessModel wraps the editor Model for headless testing.
// It provides direct access to model state without TTY rendering.
type headlessModel struct {
	model        cli.Model
	editor       *cli.Editor
	store        storage.Storage
	pending      []pendingTick
	stopped      chan struct{}
	canceled     <-chan struct{} // Optional caller cancellation; only the driver closes the model.
	stopOnce     sync.Once
	ready        chan struct{}
	timerWorkers sync.WaitGroup
	tmpDir       string // temp directory for file expectations
}

type pendingTick struct {
	request cli.TimerMsg
	result  <-chan tea.Msg
}

// newHeadlessModel creates a headless model from a config file path, reading
// and writing it through store.
func newHeadlessModel(store storage.Storage, configPath string) (*headlessModel, error) {
	ed, err := cli.NewEditorWithStorage(store, configPath)
	if err != nil {
		return nil, fmt.Errorf("creating editor: %w", err)
	}

	model, err := cli.NewModel(ed, cli.FilesystemAuthorityOperatorLocal)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("creating model: %w", err), ed.Close())
	}

	// Initialize sizing before starting the model's declared timer work.
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m, ok := newModel.(cli.Model); ok {
		model = m
	}

	hm := &headlessModel{
		stopped: make(chan struct{}),
		model:   model,
		editor:  ed,
		store:   store,
	}

	// Trigger initial completion population
	hm.model.UpdateCompletions()
	hm.processCmd(hm.model.Init())

	return hm, nil
}

// newHeadlessModelWithSession creates a headless model with session identity activated.
func newHeadlessModelWithSession(store storage.Storage, configPath, user, origin string) (*headlessModel, error) {
	ed, err := cli.NewEditorWithStorage(store, configPath)
	if err != nil {
		return nil, fmt.Errorf("creating editor: %w", err)
	}

	session := cli.NewEditSession(user, origin)
	ed.SetSession(session)

	model, err := cli.NewModel(ed, cli.FilesystemAuthorityOperatorLocal)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("creating model: %w", err), ed.Close())
	}

	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m, ok := newModel.(cli.Model); ok {
		model = m
	}

	hm := &headlessModel{
		stopped: make(chan struct{}),
		model:   model,
		editor:  ed,
		store:   store,
	}

	hm.model.UpdateCompletions()
	hm.processCmd(hm.model.Init())

	return hm, nil
}

// newHeadlessBackupModel creates a headless model over a backup artifact,
// built by the constructor `ze config edit --backup` uses, so the test drives
// the same session, draft and offline publication the command does.
func newHeadlessBackupModel(store storage.Storage, configPath, user string) (*headlessModel, error) {
	if user == "" {
		return nil, errors.New("option=storage:value=blob needs option=session:user=<name>")
	}
	ed, err := cli.NewOfflineSessionEditor(store, configPath, user)
	if err != nil {
		return nil, fmt.Errorf("creating backup editor: %w", err)
	}
	model, err := cli.NewModel(ed, cli.FilesystemAuthorityOperatorLocal)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("creating model: %w", err), ed.Close())
	}
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m, ok := newModel.(cli.Model); ok {
		model = m
	}
	hm := &headlessModel{
		stopped: make(chan struct{}),
		model:   model,
		editor:  ed,
		store:   store,
	}
	hm.model.UpdateCompletions()
	hm.processCmd(hm.model.Init())
	return hm, nil
}

// newHeadlessCommandModel creates a command-only headless model (no editor).
// Used for testing ze cli behavior where no config file is loaded.
//
// The model completes against the real command tree, so an operational-mode .et
// file drives the same completion an operator drives over SSH. A tree that
// cannot be built is returned as an error and never as an empty tree.
func newHeadlessCommandModel() (*headlessModel, error) {
	tree, err := headlessCommandTree()
	if err != nil {
		return nil, err
	}

	model := cli.NewCommandModel(cli.FilesystemAuthorityOperatorLocal)
	model.SetCommandCompleter(cli.NewCommandCompleter(tree))

	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m, ok := newModel.(cli.Model); ok {
		model = m
	}

	hm := &headlessModel{
		stopped: make(chan struct{}),
		model:   model,
	}

	return hm, nil
}

// TmpDir returns the temp directory for file expectations.
func (hm *headlessModel) TmpDir() string {
	return hm.tmpDir
}

// setTmpDir sets the temp directory for file expectations.
func (hm *headlessModel) setTmpDir(dir string) {
	hm.tmpDir = dir
}

// ReadKey reads the decoded value from the model's lifetime store.
func (hm *headlessModel) ReadKey(key string) ([]byte, error) {
	if hm.store == nil {
		return nil, errors.New("key expectation requires a store")
	}
	return hm.store.ReadKey(key)
}

// ListKeys lists the model's lifetime store keys under a literal prefix.
func (hm *headlessModel) ListKeys(prefix string) ([]string, error) {
	if hm.store == nil {
		return nil, errors.New("key expectation requires a store")
	}
	return hm.store.ListKeys(prefix)
}

// Model returns the underlying cli.Model.
func (hm *headlessModel) Model() *cli.Model {
	return &hm.model
}

// sendMsg sends a tea.Msg to the model and processes it.
func (hm *headlessModel) sendMsg(msg tea.Msg) error {
	// Only declared timers run concurrently; their results are applied here.
	hm.settle()

	newModel, cmd := hm.model.Update(msg)
	model, ok := newModel.(cli.Model)
	if !ok {
		return fmt.Errorf("update returned unexpected type: %T", newModel)
	}
	hm.model = model

	hm.processCmd(cmd)

	return nil
}

// processCmd applies a command and all state work its result produces before
// returning. TimerMsg is the sole asynchronous path; duration never classifies
// a command as a timer.
func (hm *headlessModel) processCmd(cmd tea.Cmd) {
	if cmd != nil {
		hm.processMsg(cmd())
	}
}

// settle non-blocking drains timer commands that have completed since they
// were added to pending. Called before model.Update() in SendMsg.
func (hm *headlessModel) settle() {
	if len(hm.pending) == 0 {
		return
	}
	// A recurring timer may rearm itself. Only ready entry timers are drained;
	// every state command they produce is applied synchronously by processMsg.
	snapshot := hm.pending
	hm.pending = nil

	for _, tick := range snapshot {
		select {
		case msg := <-tick.result:
			hm.processMsg(msg)
		default:
			hm.pending = append(hm.pending, tick)
		}
	}
}

// settleWait preserves implicit validation before expectations, without
// waiting for recurring display/poll timers. State commands are already fully
// applied, including work spawned by their results. The latest validation
// timer targets the current generation, not whichever older timer fires first.
func (hm *headlessModel) settleWait() error {
	hm.settle()
	return hm.waitValidation()
}

func (hm *headlessModel) waitValidation() error {
	for i, tick := range slices.Backward(hm.pending) {
		if !tick.request.IsValidation() {
			continue
		}
		hm.pending = append(hm.pending[:i], hm.pending[i+1:]...)
		select {
		case msg := <-tick.result:
			if msg == nil {
				return errors.New("validation wait canceled")
			}
			hm.processMsg(msg)
		case <-hm.stopped:
			// Keep the join obligation if shutdown won the receive.
			hm.pending = append(hm.pending, tick)
			return errors.New("validation wait canceled")
		case <-hm.canceled:
			hm.pending = append(hm.pending, tick)
			return errors.New("validation wait canceled")
		}
		hm.settle()
		return nil
	}
	return nil
}

// processMsg processes a message from a command.
func (hm *headlessModel) processMsg(msg tea.Msg) {
	if msg == nil {
		return
	}
	if tick, ok := msg.(cli.TimerMsg); ok {
		_, wait := hm.model.Update(tick)
		if wait == nil {
			return
		}
		if hm.ready == nil {
			hm.ready = make(chan struct{}, 1)
		}
		ready := hm.ready
		done := make(chan tea.Msg, 1)
		hm.pending = append(hm.pending, pendingTick{request: tick, result: done})
		hm.timerWorkers.Go(func() {
			done <- wait()
			select {
			case ready <- struct{}{}:
			default:
			}
		})
		return
	}
	// Handle batch commands
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			hm.processCmd(c)
		}
		return
	}
	// Bubble Tea keeps sequenceMsg private. Match only its exact runtime type,
	// obtained through the public constructor, then convert its []tea.Cmd shape.
	if reflect.TypeOf(msg) == sequenceMessageType {
		commands := reflect.ValueOf(msg).Convert(reflect.TypeFor[[]tea.Cmd]()).Interface().([]tea.Cmd) //nolint:errcheck,forcetypeassert // Convert returns exactly the requested []tea.Cmd type.
		for _, command := range commands {
			hm.processCmd(command)
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

	hm.processCmd(nextCmd)
}

var sequenceMessageType = reflect.TypeOf(tea.Sequence(func() tea.Msg { return nil }, func() tea.Msg { return nil })())

// close ends the model, drains its history and joins declared timers before
// releasing the editor. The runner MUST close every model, including replacements.
func (hm *headlessModel) close() error {
	hm.stopOnce.Do(func() {
		if hm.stopped != nil {
			close(hm.stopped)
		}
	})
	err := hm.model.Close()
	for _, tick := range hm.pending {
		<-tick.result
	}
	hm.pending = nil
	hm.timerWorkers.Wait()
	if hm.editor != nil {
		err = errors.Join(err, hm.editor.Close())
	}
	return err
}

// typeText sends each character as a KeyPressMsg.
func (hm *headlessModel) typeText(text string) {
	for _, r := range text {
		_ = hm.sendMsg(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

// pressEnter sends an Enter key message.
func (hm *headlessModel) pressEnter() {
	_ = hm.sendMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
}

// --- State Accessors ---

// InputValue returns the current text input value.
func (hm *headlessModel) InputValue() string {
	return hm.model.InputValue()
}

// ContextPath returns the current context path.
func (hm *headlessModel) ContextPath() []string {
	return hm.model.ContextPath()
}

// Completions returns the current completion list.
func (hm *headlessModel) Completions() []cli.Completion {
	return hm.model.Completions()
}

// GhostText returns the current ghost text suggestion.
func (hm *headlessModel) GhostText() string {
	return hm.model.GhostText()
}

// ValidationErrors returns the current validation errors.
func (hm *headlessModel) ValidationErrors() []cli.ConfigValidationError {
	return hm.model.ValidationErrors()
}

// ValidationWarnings returns the current validation warnings.
func (hm *headlessModel) ValidationWarnings() []cli.ConfigValidationError {
	return hm.model.ValidationWarnings()
}

// Dirty returns true if there are unsaved changes.
func (hm *headlessModel) Dirty() bool {
	return hm.model.Dirty()
}

// StatusMessage returns the current status message.
func (hm *headlessModel) StatusMessage() string {
	return hm.model.StatusMessage()
}

// Error returns the current command error.
func (hm *headlessModel) Error() error {
	return hm.model.Error()
}

// IsTemplate returns true if in template editing mode.
func (hm *headlessModel) IsTemplate() bool {
	return hm.model.IsTemplate()
}

// ShowDropdown returns true if the completion dropdown is visible.
func (hm *headlessModel) ShowDropdown() bool {
	return hm.model.ShowDropdown()
}

// MessageHint returns the unstyled text of the second message line, which
// carries the completion hint and the selected candidate's summary.
func (hm *headlessModel) MessageHint() string {
	return hm.model.MessageHint()
}

// Explanation returns the long explanation now revealed, empty when none is.
func (hm *headlessModel) Explanation() string {
	return hm.model.Explanation()
}

// WorkingContent returns the current working config content.
// Returns empty string for command-only models (no editor).
func (hm *headlessModel) WorkingContent() string {
	if hm.editor == nil {
		return ""
	}
	return hm.editor.WorkingContent()
}

// SelectedIndex returns the currently selected dropdown index.
func (hm *headlessModel) SelectedIndex() int {
	return hm.model.SelectedIndex()
}

// ViewportContent returns the content currently displayed in the viewport.
func (hm *headlessModel) ViewportContent() string {
	return hm.model.ViewportContent()
}

// ConfirmTimerActive returns true if a commit confirm timer is active.
func (hm *headlessModel) ConfirmTimerActive() bool {
	return hm.model.ConfirmTimerActive()
}

// TriggerCompletions forces an update of the completion list.
func (hm *headlessModel) TriggerCompletions() {
	hm.model.UpdateCompletions()
}

// Mode returns the current editor mode.
func (hm *headlessModel) Mode() cli.EditorMode {
	return hm.model.Mode()
}

// SetReloadNotifier configures a reload notifier on the underlying cli.
// Used by the .et test runner to simulate daemon reload behavior.
// No-op for command-only models (no editor).
func (hm *headlessModel) SetReloadNotifier(fn cli.ReloadNotifier) {
	if hm.editor == nil {
		return
	}
	hm.editor.SetReloadNotifier(fn)
}

// SetShutdownFunc configures the shutdown callback on the model.
// Used by the .et test runner to simulate daemon stop behavior.
func (hm *headlessModel) SetShutdownFunc(fn func()) {
	hm.model.SetShutdownFunc(fn)
}

// SetRestartFunc configures the restart callback on the model.
// Used by the .et test runner to simulate daemon restart behavior (GR marker write + stop).
func (hm *headlessModel) SetRestartFunc(fn func()) {
	hm.model.SetRestartFunc(fn)
}

// Design: docs/features/cli-commands.md -- history persistence must not stall the CLI.
// Related: model_timer_lifetime_test.go -- deterministic timerProbe used below.

package cli

import (
	"io/fs"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	tea "charm.land/bubbletea/v2"
)

type blockedHistoryWrite struct {
	key     string
	content string
}

// blockedHistoryWriter gates only history I/O. The model, editor, dispatch and
// timer handlers remain the production implementations.
type blockedHistoryWriter struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	mu      sync.Mutex
	writes  []blockedHistoryWrite
}

func (*blockedHistoryWriter) ReadFile(string) ([]byte, error) {
	return nil, fs.ErrNotExist
}

func (w *blockedHistoryWriter) WriteFile(key string, data []byte, _ fs.FileMode) error {
	w.once.Do(func() {
		close(w.entered)
		<-w.release
	})
	w.mu.Lock()
	w.writes = append(w.writes, blockedHistoryWrite{key: key, content: string(data)})
	w.mu.Unlock()
	return nil
}

type blockedHistoryProgress struct {
	inputAfterEnter string
	inputAfterKey   string
	mode            EditorMode
	secondsLeft     int
	output          string
	entries         []string
}

// TestModelHistoryBlockedWriterKeepsUpdateProgress holds the actual History
// writer and drives each of handleEnter's three persistence branches through
// Model.Update. A following key, a declared timer expiry and a real command
// result must all be applied before the writer is released. Synctest observes
// durable blocking rather than guessing how long a filesystem write takes.
//
// VALIDATES: useful input/timer/result progress while history publication waits,
// followed by the exact accepted command being published under its real mode.
// PREVENTS: the native SSH stall in which History.Save holds Bubble Tea's serial
// event loop in fsync before it can consume another message.
func TestModelHistoryBlockedWriterKeepsUpdateProgress(t *testing.T) {
	const operationalAnswer = "operational result while history is held"
	cases := []struct {
		name   string
		mode   EditorMode
		input  string
		output string
	}{
		{name: "config command", mode: ModeConfig, input: "show", output: emptyConfiguration},
		{name: "config run", mode: ModeConfig, input: "run show", output: operationalAnswer},
		{name: "operational format", mode: ModeOperational, input: "set cli format text", output: operationalAnswer},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Construct the real editor before the synctest bubble; only history
			// WriteFile is replaced, not the editor's configuration/mode behavior.
			model := newTestModel(t)
			t.Cleanup(func() {
				if err := model.editor.Close(); err != nil {
					t.Error(err)
				}
			})
			synctest.Test(t, func(t *testing.T) {
				writer := &blockedHistoryWriter{
					entered: make(chan struct{}),
					release: make(chan struct{}),
				}
				model.SetHistory(NewHistory(writer, "blocked-user"))
				model.SetCommandExecutor(func(string) (CommandOutput, error) {
					return CommandOutput{Text: operationalAnswer}, nil
				})
				if model.Mode() != tc.mode {
					model.SetMode(tc.mode)
				}
				probe := &timerProbe{
					started: make(chan time.Duration, 1),
					expired: make(chan time.Time, 1),
					stopped: make(chan struct{}),
				}
				model.dispatch.clock = probe
				model.confirmTimerActive = true
				model.confirmSecondsLeft = 9
				model.SetInput(tc.input)

				progress := make(chan blockedHistoryProgress, 1)
				driverDone := make(chan struct{})
				var releaseOnce sync.Once
				release := func() { releaseOnce.Do(func() { close(writer.release) }) }
				defer func() {
					// Release and join even when the pre-fix progress assertion fails.
					// Close must run inside the bubble that owns any future worker.
					release()
					<-driverDone
					if err := model.Close(); err != nil {
						t.Error(err)
					}
					writer.mu.Lock()
					defer writer.mu.Unlock()
					if len(writer.writes) != 1 {
						t.Errorf("history writes = %v, want the one accepted command", writer.writes)
						return
					}
					wantKey := historyKeyPrefix + "blocked-user/" + tc.mode.String()
					got := writer.writes[0]
					if got.key != wantKey {
						t.Errorf("history key = %q, want %q", got.key, wantKey)
					}
					if got.content != tc.input {
						t.Errorf("history content = %q, want %q", got.content, tc.input)
					}
				}()

				go func() {
					defer close(driverDone)
					current := *model
					command := applyHistoryProgressMessage(t, &current, tea.KeyPressMsg{Code: tea.KeyEnter})
					observed := blockedHistoryProgress{inputAfterEnter: current.InputValue(), mode: current.Mode()}
					applyHistoryProgressMessage(t, &current, tea.KeyPressMsg{Code: 'x', Text: "x"})
					observed.inputAfterKey = current.InputValue()

					// Route actual TimerMsg admission, Wait and expiry back through
					// Update; no direct call to the countdown handler substitutes.
					request := current.tick(time.Second, confirmCountdownMsg{})()
					wait := applyHistoryProgressMessage(t, &current, request)
					if wait == nil {
						t.Error("the model refused its own declared timer")
						return
					}
					expired := make(chan tea.Msg, 1)
					go func() { expired <- wait() }()
					<-probe.started
					probe.expired <- time.Time{}
					applyHistoryProgressMessage(t, &current, <-expired)
					observed.secondsLeft = current.confirmSecondsLeft

					// Config show/run execute the command Enter actually returned.
					// Format changes are inline, so an independently completed
					// operational command supplies their result-message control.
					if command == nil {
						command = current.executeOperationalCommand("show")
					}
					applyHistoryProgressMessage(t, &current, command())
					observed.output = current.viewportContent
					observed.entries = slices.Clone(current.history.Entries())
					progress <- observed
				}()

				synctest.Wait()
				select {
				case <-writer.entered:
				default:
					t.Error("accepted command never reached history WriteFile")
				}
				select {
				case got := <-progress:
					if got.inputAfterEnter != "" {
						t.Errorf("input after Enter = %q, want cleared input", got.inputAfterEnter)
					}
					if got.inputAfterKey != "x" {
						t.Errorf("input while history is held = %q, want the following key x", got.inputAfterKey)
					}
					if got.mode != tc.mode {
						t.Errorf("mode = %v, want the actual %v mode", got.mode, tc.mode)
					}
					if got.secondsLeft != 8 {
						t.Errorf("countdown while history is held = %d, want 8", got.secondsLeft)
					}
					if got.output != tc.output {
						t.Errorf("command output while history is held = %q, want %q", got.output, tc.output)
					}
					if !slices.Equal(got.entries, []string{tc.input}) {
						t.Errorf("browsable history = %v, want [%q]", got.entries, tc.input)
					}
				default:
					t.Error("Model.Update is blocked by history WriteFile: following input, timer expiry and command result made no progress")
				}
			})
		})
	}
}

// applyHistoryProgressMessage keeps the test driver serial like Bubble Tea's
// event loop while executing the actual Update dispatch and returned Model.
func applyHistoryProgressMessage(t *testing.T, model *Model, msg tea.Msg) tea.Cmd {
	t.Helper()
	next, command := model.Update(msg)
	updated, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", next)
	}
	*model = updated
	return command
}

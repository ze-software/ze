// Design: docs/architecture/testing/ci-format.md -- authored editor waits.

package testing

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
)

func runWaitFile(t *testing.T, steps string) *TestResult {
	t.Helper()
	content := "tmpfs=test.conf:terminator=END\nbgp { router-id 1.2.3.4; session { asn { local 65000; } } }\nEND\noption=file:path=test.conf\n" + steps
	path := filepath.Join(t.TempDir(), "wait.et")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return RunETFile(path, 1)
}

func TestETWaitMillisecondsExecutes(t *testing.T) {
	const observer = "wait-elapsed-proof"
	var start time.Time
	previous, existed := validExpectationTypes[observer]
	t.Cleanup(func() {
		if existed {
			validExpectationTypes[observer] = previous
		} else {
			delete(validExpectationTypes, observer)
		}
	})
	validExpectationTypes[observer] = func(exp Expectation, _ State) error {
		if exp.Values["stage"] == "start" {
			start = time.Now()
			return nil
		}
		if elapsed := time.Since(start); elapsed < 80*time.Millisecond {
			return fmt.Errorf("authored 80ms wait returned after %s", elapsed)
		}
		return nil
	}
	result := runWaitFile(t, "expect=wait-elapsed-proof:stage=start\nwait=ms:80\nexpect=wait-elapsed-proof:stage=end\n")
	require.True(t, result.Passed, "%s", result.Error)
}

func TestETWaitTimerRequiresActiveEntry(t *testing.T) {
	for _, before := range []string{"", "input=type:text=commit accept\ninput=enter\n", "input=type:text=commit abort\ninput=enter\n"} {
		t.Run(before, func(t *testing.T) {
			steps := before
			if before != "" {
				steps = "input=type:text=set bgp router-id 5.6.7.8\ninput=enter\ninput=type:text=commit confirmed 60 force\ninput=enter\nexpect=error:none\nexpect=timer:active\n" + before
			}
			result := runWaitFile(t, steps+"wait=timer:expire\n")
			require.False(t, result.Passed, "inactive, accepted or aborted is not expiry")
			require.Contains(t, result.Error, "(wait)")
			require.Contains(t, result.Error, "active")
		})
	}
}

func TestETWaitTimerExpiresAndRestoresConfiguration(t *testing.T) {
	result := runWaitFile(t, "input=type:text=set bgp router-id 5.6.7.8\ninput=enter\n"+
		"input=type:text=commit confirmed 1 force\ninput=enter\nexpect=error:none\nexpect=timer:active\n"+
		"wait=timer:expire\nexpect=timer:inactive\nexpect=status:contains=Timeout: configuration automatically rolled back.\n"+
		"expect=content:contains=1.2.3.4\nexpect=content:not-contains=5.6.7.8\n")
	require.True(t, result.Passed, "%s", result.Error)
}

func TestETWaitRejectsInvalidActions(t *testing.T) {
	for _, action := range []string{"ms:-1", "ms:bad", "ms:9223372036854775807", "timer:other", "unknown", "validation:other", "ms:1:ignored", "ms=1:ignored"} {
		t.Run(action, func(t *testing.T) {
			result := runWaitFile(t, "wait="+action+"\n")
			require.False(t, result.Passed)
			require.Contains(t, result.Error, "wait")
		})
	}
}

type waitWindowRecorder struct{}

func (*waitWindowRecorder) Save(confirm.Pending) error { return nil }
func (*waitWindowRecorder) Clear() error               { return nil }

func TestETWaitObservesRealDaemonExpiry(t *testing.T) {
	const observer = "wait-daemon-proof"
	previous, existed := validExpectationTypes[observer]
	t.Cleanup(func() {
		if existed {
			validExpectationTypes[observer] = previous
		} else {
			delete(validExpectationTypes, observer)
		}
	})
	var window *confirm.Window
	validExpectationTypes[observer] = func(exp Expectation, state State) error {
		hm, ok := state.(*headlessModel)
		if !ok {
			return fmt.Errorf("daemon-expiry observer received %T, expected headless editor model", state)
		}
		path := hm.editor.OriginalPath()
		switch exp.Values["stage"] {
		case "wire":
			window = confirm.NewWindow(func(rollback []byte) error {
				if _, err := storage.WriteCandidateVersion(hm.store, path, rollback, time.Now()); err != nil {
					return err
				}
				return storage.PromoteCandidate(hm.store, path)
			}, &waitWindowRecorder{})
			t.Cleanup(window.Stop)
			hm.editor.SetConfirmWindow(func() *confirm.Window { return window })
			hm.editor.SetReloadNotifier(func() error { return storage.PromoteCandidate(hm.store, path) })
		case "active":
			if _, open := window.Status(); !open {
				return fmt.Errorf("daemon window did not open")
			}
		case "expired":
			count, err := window.Timeouts()
			if err != nil {
				return fmt.Errorf("reading daemon timeout count: %w", err)
			}
			if count != 1 {
				return fmt.Errorf("daemon timeout count=%d, want 1", count)
			}
			active, err := storage.ReadActiveConfig(hm.store, path)
			if err != nil {
				return err
			}
			if !strings.Contains(string(active), "1.2.3.4") || strings.Contains(string(active), "5.6.7.8") {
				return fmt.Errorf("daemon did not restore active config: %s", active)
			}
		}
		return nil
	}
	result := runWaitFile(t, "option=session:user=alice:origin=local\nexpect=wait-daemon-proof:stage=wire\n"+
		"input=type:text=set bgp router-id 5.6.7.8\ninput=enter\ninput=type:text=commit confirmed 1 force\ninput=enter\nexpect=error:none\n"+
		"expect=wait-daemon-proof:stage=active\nwait=timer:expire\nexpect=wait-daemon-proof:stage=expired\nexpect=timer:inactive\n")
	require.True(t, result.Passed, "%s", result.Error)
}

func TestETWaitReportsRealDaemonRevertFailure(t *testing.T) {
	const observer = "wait-revert-failure-proof"
	previous, existed := validExpectationTypes[observer]
	t.Cleanup(func() {
		if existed {
			validExpectationTypes[observer] = previous
		} else {
			delete(validExpectationTypes, observer)
		}
	})
	cause := errors.New("rollback storage write refused")
	wired := make(chan *headlessModel, 1)
	var window *confirm.Window
	validExpectationTypes[observer] = func(_ Expectation, state State) error {
		hm, ok := state.(*headlessModel)
		if !ok {
			return fmt.Errorf("revert-failure observer received %T, expected headless editor model", state)
		}
		window = confirm.NewWindow(func([]byte) error { return cause }, &waitWindowRecorder{})
		t.Cleanup(window.Stop)
		hm.editor.SetConfirmWindow(func() *confirm.Window { return window })
		hm.editor.SetReloadNotifier(func() error { return storage.PromoteCandidate(hm.store, hm.editor.OriginalPath()) })
		wired <- hm
		return nil
	}
	finished := make(chan *TestResult, 1)
	go func() {
		finished <- runWaitFile(t, "option=timeout:value=20s\noption=session:user=operator:origin=ssh\nexpect=wait-revert-failure-proof\n"+
			"input=type:text=set bgp router-id 5.6.7.8\ninput=enter\ninput=type:text=commit confirmed 1 force\ninput=enter\nexpect=error:none\nexpect=timer:active\nwait=timer:expire\n")
	}()
	hm := <-wired
	var result *TestResult
	select {
	case result = <-finished:
	case <-time.After(6 * time.Second):
		// Stop the wait on a red too, so the proof joins its worker instead of
		// leaving it to the outer runner timeout. No model state is touched.
		hm.stopOnce.Do(func() { close(hm.stopped) })
		result = <-finished
	}
	require.False(t, result.Passed)
	require.Contains(t, result.Error, "(wait)")
	require.Contains(t, result.Error, cause.Error(), "report the rollback cause rather than cancellation or the outer timeout")
	require.ErrorIs(t, hm.model.Error(), cause, "the cause must remain structured")
	status, open := window.Status()
	require.True(t, open, "a failed rollback must leave the daemon window open")
	require.ErrorIs(t, status.RevertFailed, cause)
}

func TestETWaitValidationPrecedesNextStateCommand(t *testing.T) {
	const observer = "wait-validation-seed"
	previous, existed := validExpectationTypes[observer]
	t.Cleanup(func() {
		if existed {
			validExpectationTypes[observer] = previous
		} else {
			delete(validExpectationTypes, observer)
		}
	})
	called, validated := false, false
	validExpectationTypes[observer] = func(_ Expectation, state State) error {
		hm, ok := state.(*headlessModel)
		if !ok {
			return fmt.Errorf("validation observer received %T, expected headless editor model", state)
		}
		if err := hm.editor.SetValue([]string{"bgp"}, "router-id", "invalid"); err != nil {
			return err
		}
		hm.Model().SetCommandExecutor(func(string) (cli.CommandOutput, error) {
			called = true
			validated = len(hm.ValidationErrors()) > 0
			return cli.CommandOutput{}, nil
		})
		// Every keystroke schedules a generation. The executor observes state
		// before a later expectation can implicitly settle validation.
		hm.typeText("run show version")
		return nil
	}
	result := runWaitFile(t, "expect=wait-validation-seed\nwait=validation\ninput=enter\n")
	require.True(t, result.Passed, "%s", result.Error)
	require.True(t, called, "the next state command must execute")
	require.True(t, validated, "the wait must apply current validation before that command")
}

func TestETWaitDroppedConfirmationTickFails(t *testing.T) {
	const observer = "wait-drop-tick"
	var dropped []<-chan tea.Msg
	previous, existed := validExpectationTypes[observer]
	t.Cleanup(func() {
		if existed {
			validExpectationTypes[observer] = previous
		} else {
			delete(validExpectationTypes, observer)
		}
	})
	validExpectationTypes[observer] = func(_ Expectation, state State) error {
		hm, ok := state.(*headlessModel)
		if !ok {
			return fmt.Errorf("countdown observer received %T, expected headless editor model", state)
		}
		kept := hm.pending[:0]
		for _, tick := range hm.pending {
			if tick.request.IsConfirmation() {
				dropped = append(dropped, tick.result)
			} else {
				kept = append(kept, tick)
			}
		}
		hm.pending = kept
		return nil
	}
	result := runWaitFile(t, "input=type:text=set bgp router-id 5.6.7.8\ninput=enter\ninput=type:text=commit confirmed 60 force\ninput=enter\n"+
		"expect=error:none\nexpect=timer:active\nexpect=wait-drop-tick\nwait=timer:expire\n")
	// RunETFile closed the owner. Join the deliberately removed timer too.
	for _, result := range dropped {
		<-result
	}
	require.Len(t, dropped, 1, "the proof must actually drop the countdown")
	require.False(t, result.Passed)
	require.Contains(t, result.Error, "no scheduled countdown")
}

func TestHeadlessWaitCancellationAndZeroDuration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.conf")
	require.NoError(t, os.WriteFile(path, []byte(testConfig), 0o600))
	hm, err := newHeadlessModel(headlessTestStore(t, path), path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, hm.close()) })
	require.NoError(t, hm.wait(WaitAction{Values: map[string]string{"ms": "0"}}))
	hm.typeText("show")
	require.NoError(t, hm.model.Close())
	require.ErrorContains(t, hm.wait(WaitAction{Values: map[string]string{"validation": ""}}), "canceled")
	require.NoError(t, hm.close())
	require.ErrorContains(t, hm.wait(WaitAction{Values: map[string]string{"ms": "100"}}), "canceled")
}

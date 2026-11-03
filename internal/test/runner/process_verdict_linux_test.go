// Design: docs/architecture/testing/runner-architecture.md -- lifecycle failures govern verdicts.
// Related: process_identity_linux_test.go -- retained identity and observation syscall seams.

package runner

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// TestRunnerLifecycleFailureCannotPass runs actual runner verdict paths with real
// children. WNOWAIT first observes the actual exit; the existing observeExit
// syscall seam then reports a failure without changing that exit status. Each
// case also runs without the fault to pin ordinary process/assertion semantics.
func TestRunnerLifecycleFailureCannotPass(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind string
		code int
	}{
		{"helper-file-zero", "helper", 0},
		{"helper-file-nonzero", "helper", 7},
		{"helper-file-signal", "helper", -1},
		{"helper-command-nonzero", "command", 7},
		{"quick-file-zero", "quick", 0},
		{"quick-command-nonzero", "quick-command", 7},
		{"quick-self-validated", "self", 0},
		{"daemon-file-nonzero", "daemon", 7},
		{"fence-success", "fence", 7},
		{"fence-failure", "fence-failure", 0},
		{"plain-fence-success", "plain-fence", 0},
		{"scaffold-peer-self-validated", "scaffold", 0},
		{"check-peer", "peer", 0},
		{"named-stop-peer", "stop-peer", 0},
		{"background-self-validated", "background", 0},
		{"named-stop", "stop", 0},
		{"early-return-cleanup", "early", 0},
		{"legacy-client", "legacy-client", 0},
		{"legacy-peer", "legacy-peer", 0},
		{"exit-mismatch", "mismatch", 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, inject := range []bool{false, true} {
				t.Run(strconv.FormatBool(inject), func(t *testing.T) {
					path := filepath.Join(t.TempDir(), "child.sh")
					body := "#!/bin/sh\n"
					if tc.code >= 0 {
						body += "trap 'exit " + strconv.Itoa(tc.code) + "' TERM\n"
					}
					body += "echo proof\necho 'listening on fixture'\necho successful\ntouch daemon.ready\n"
					if tc.kind != "fence-failure" {
						body += "echo fence-ready >&2\n"
					}
					if tc.code == -1 {
						body += "kill -KILL $$\n"
					} else {
						body += "exit " + strconv.Itoa(tc.code) + "\n"
					}
					if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
						t.Fatal(err)
					}
					cmd := RunCommand{Mode: modeForeground, Seq: 1, Exec: `sh "` + path + `"`}
					r, rec := lastHelperRecord(t, tc.name, "5s", &cmd)
					rec.ExpectExitCode = &tc.code
					r.zePath = path
					wantPass := true
					wantCause := ""
					switch tc.kind {
					case "command":
						rec.RunCommands[0].ExitCode = &tc.code
					case "quick", "quick-command", "self":
						rec.RunCommands[0].Exec = "ze config validate"
						if tc.kind == "quick-command" {
							rec.RunCommands[0].ExitCode = &tc.code
						}
						if tc.kind == "self" {
							rec.ExpectExitCode = nil
							rec.RunCommands[0].ExpectStdout = []string{"proof"}
						}
					case "daemon", "fence", "fence-failure", "plain-fence":
						rec.RunCommands[0].Exec = "ze start"
						if tc.kind != "daemon" {
							rec.AwaitStderr = []string{"fence-ready"}
							rec.AwaitThenStop = true
							rec.AwaitStderrTimeout = "100ms"
							if tc.kind == "plain-fence" {
								rec.AwaitThenStop = false
							}
						}
						if tc.kind == "fence-failure" {
							wantPass, wantCause = false, "never contained"
						}
					case "background", "stop", "early":
						rec.RunCommands[0].Mode = modeBackground
						rec.RunCommands[0].Ready = "proof"
						rec.RunCommands[0].Name = "owned"
						// The observer publishes this file only after WNOWAIT saw exit.
						// Polling is bounded by the runner deadline, not a guessed delay.
						rec.RunCommands = append(rec.RunCommands, RunCommand{Mode: modeForeground, Seq: 2,
							Exec: `sh -c 'while [ ! -f observed ]; do sleep 0.01; done'`})
						if tc.kind == "background" {
							rec.ExpectExitCode = nil
							rec.RunCommands[0].ExpectStdout = []string{"proof"}
						}
						if tc.kind == "stop" {
							rec.RunCommands = append(rec.RunCommands, RunCommand{Mode: modeStop, Seq: 3, Name: "owned", Signal: signalKill})
						}
						if tc.kind == "early" {
							rec.RunCommands = append(rec.RunCommands, RunCommand{Mode: modeForeground, Seq: 3, Exec: "true", Stdin: "absent"})
							wantPass, wantCause = false, "not found"
						}
					case "scaffold", "peer", "stop-peer":
						r.lePath = path
						rec.RunCommands[0] = RunCommand{Mode: modeBackground, Seq: 1, Name: "owned", Exec: "le test peer --dial"}
						if tc.kind == "scaffold" {
							rec.RunCommands[0].Exec += " --mode sink"
						}
						if tc.kind == "peer" {
							rec.ExpectExitCode = nil
						}
						rec.RunCommands = append(rec.RunCommands, RunCommand{Mode: modeForeground, Seq: 2,
							Exec: `sh -c 'while [ ! -f observed ]; do sleep 0.01; done'`})
						if tc.kind == "stop-peer" {
							rec.RunCommands = append(rec.RunCommands, RunCommand{Mode: modeStop, Seq: 3, Name: "owned", Signal: signalKill})
						}
					case "legacy-client", "legacy-peer":
						rec.RunCommands = nil
						other := filepath.Join(t.TempDir(), "other.sh")
						if err := os.WriteFile(other, []byte(body), 0o700); err != nil {
							t.Fatal(err)
						}
						r.lePath = other
						if tc.kind == "legacy-peer" {
							r.lePath, r.zePath = path, other
						}
						peerGate := "client-ready"
						if tc.kind == "legacy-client" {
							peerGate = "observed"
						}
						// The peer lets the client install its TERM handler (or exit)
						// before finishing; no scheduling delay decides the status.
						peerBody := "#!/bin/sh\necho 'listening on fixture'\nwhile [ ! -f " + peerGate + " ]; do sleep 0.01; done\necho successful\n"
						clientBody := "#!/bin/sh\ntrap 'exit 0' TERM\ntouch client-ready\necho proof\nexit 0\n"
						if err := os.WriteFile(r.lePath, []byte(peerBody), 0o700); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(r.zePath, []byte(clientBody), 0o700); err != nil {
							t.Fatal(err)
						}
					case "mismatch":
						zero := 0
						rec.ExpectExitCode = &zero
						wantPass, wantCause = false, "expected exit code 0, got 7"
					}
					fault := errors.New("injected exit observation failure")
					observed := make(chan *ownedProcess, 1)
					r.observeProcessExit = func(p *ownedProcess) error {
						selected := p.cmd.Path == path || slices.Contains(p.cmd.Args, path)
						if !selected {
							return p.beforeReap()
						}
						defer func() {
							if err := os.WriteFile(filepath.Join(p.cmd.Dir, "observed"), nil, 0o600); err != nil {
								t.Error(err)
							}
							observed <- p
						}()
						if !inject {
							return p.beforeReap()
						}
						return p.observeExit(func() error {
							var info unix.Siginfo
							if err := unix.Waitid(unix.P_PID, p.cmd.Process.Pid, &info, unix.WEXITED|unix.WNOWAIT, nil); err != nil {
								return err
							}
							return fault
						})
					}
					passed := r.runTest(t.Context(), rec, &RunOptions{})
					if passed != (wantPass && !inject) {
						t.Errorf("runner passed=%v, error=%v", passed, rec.Error)
					}
					if inject {
						if !errors.Is(rec.Error, fault) {
							t.Errorf("runner lost lifecycle cause: %v", rec.Error)
						}
						if rec.FailureType != "process_lifecycle_failed" {
							t.Errorf("failure type=%q", rec.FailureType)
						}
					}
					if wantCause != "" {
						if rec.Error == nil || !strings.Contains(rec.Error.Error(), wantCause) {
							t.Errorf("lost original verdict cause %q: %v", wantCause, rec.Error)
						}
					}
					select {
					case p := <-observed:
						if got := p.cmd.ProcessState.ExitCode(); got != tc.code {
							t.Errorf("actual exit=%d, want %d", got, tc.code)
						}
						waitErr := p.Wait()
						if tc.code == 0 {
							if waitErr != nil {
								t.Errorf("lifecycle failure contaminated exit status: %v", waitErr)
							}
						} else {
							var exit *exec.ExitError
							if !errors.As(waitErr, &exit) || exit.ExitCode() != tc.code {
								t.Errorf("cached exit=%v, want %d", waitErr, tc.code)
							}
						}
					default:
						t.Fatal("runner never joined the selected observer")
					}
				})
			}
		})
	}
}

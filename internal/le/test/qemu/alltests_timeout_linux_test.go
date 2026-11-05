//go:build linux

// Design: docs/architecture/testing/qemu-integration.md -- native suite deadlines.
// Related: alltests.go -- the actual suite consumer and summary tally.

package testqemu

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestQEMUAllSuiteDeadlineUsesNativeOwnership drives the actual suite consumer
// with a timeout executable that refuses -k, like Ze's minimal image. Normal
// exits retain their status and tally; expiration sends TERM to the group,
// grants a resistant parent the full 15-second grace, then kills its child too.
func TestQEMUAllSuiteDeadlineUsesNativeOwnership(t *testing.T) {
	for _, mode := range []string{"normal-zero", "normal-17", "term", "resistant"} {
		t.Run(mode, func(t *testing.T) {
			directory := t.TempDir()
			timeoutCalled := filepath.Join(directory, "external-timeout-called")
			termObserved := filepath.Join(directory, "term-observed")
			childTermObserved := filepath.Join(directory, "child-term-observed")
			rescued := filepath.Join(directory, "deadline-rescued")
			parentPID := filepath.Join(directory, "parent-pid")
			childPID := filepath.Join(directory, "child-pid")
			writeTimeoutExecutable(t, filepath.Join(directory, "timeout"),
				"printf called > "+shellQuote(timeoutCalled)+"\nprintf 'timeout: unrecognized option: k\\n' >&2\nexit 125\n")
			t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
			script := "printf 'pass  7/7  100.0%%  1.0s\\n'\n"
			if strings.HasPrefix(mode, "normal") {
				if mode == "normal-17" {
					script += "exit 17\n"
				} else {
					script += "exit 0\n"
				}
			} else {
				script += "printf '%s' \"$$\" > " + shellQuote(parentPID) + "\n"
				childScript := "trap " + shellQuote("printf term > "+shellQuote(childTermObserved)+"; exit 0") + " TERM\n"
				if mode == "resistant" {
					childScript = "trap " + shellQuote("printf term > "+shellQuote(childTermObserved)+"; /bin/sleep 60 & wait \"$!\"; printf rescued > "+shellQuote(rescued)+"; exit 93") + " TERM\n"
				}
				childScript += "/bin/sleep 60 & wait \"$!\"\nprintf rescued > " + shellQuote(rescued) + "\nexit 93\n"
				childPath := filepath.Join(directory, "child")
				writeTimeoutExecutable(t, childPath, childScript)
				script += "trap " + shellQuote("printf term > "+shellQuote(termObserved)) + " TERM\n"
				script += shellQuote(childPath) + " &\nchild=$!\nprintf '%s' \"$child\" > " + shellQuote(childPID) + "\n"
				if mode == "term" {
					script += "trap " + shellQuote("printf term > "+shellQuote(termObserved)+"; wait \"$child\"; exit 0") + " TERM\n"
				}
				// Each helper has a finite rescue path. Reaching it is a failure,
				// rather than an elapsed-time upper assertion on a loaded host.
				script += "wait \"$child\"\nwait \"$child\"\nprintf rescued > " + shellQuote(rescued) + "\nexit 94\n"
			}
			writeTimeoutExecutable(t, filepath.Join(directory, leName), script)
			run := &allTestsRun{Workspace: directory, BinDir: directory, Parallel: "1", Timeout: "1s", Note: func(string) {}}
			if strings.HasPrefix(mode, "normal") {
				run.Timeout = "900s"
			}
			started := time.Now()
			result := run.suite(vmSuite{Name: "deadline-fixture", Args: []string{"fixture"}, Namespace: guestRoot}, os.Environ())
			elapsed := time.Since(started)
			wantCode := 124
			if mode == "normal-zero" {
				wantCode = 0
			}
			if mode == "normal-17" {
				wantCode = 17
			}
			if mode == "resistant" {
				wantCode = 137
			}
			if result.Code != wantCode {
				t.Errorf("suite exit = %d, want %d", result.Code, wantCode)
			}
			if !result.Counted {
				t.Error("native child output did not reach the suite tally")
			}
			if result.Tests != 7 {
				t.Errorf("suite tally = %d, want seven tests", result.Tests)
			}
			if _, err := os.Stat(timeoutCalled); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("suite invoked the incompatible external timeout: %v", err)
			}
			if strings.HasPrefix(mode, "normal") {
				return
			}
			if observed, err := os.ReadFile(termObserved); err != nil {
				t.Errorf("deadline did not deliver TERM: %v", err)
			} else if string(observed) != "term" {
				t.Errorf("TERM observation = %q", observed)
			}
			if observed, err := os.ReadFile(childTermObserved); err != nil {
				t.Errorf("deadline did not deliver TERM to the same-group child: %v", err)
			} else if string(observed) != "term" {
				t.Errorf("child TERM observation = %q", observed)
			}
			if _, err := os.Stat(rescued); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("suite reached its test-only rescue instead of native termination: %v", err)
			}
			assertTimeoutProcessStopped(t, parentPID)
			assertTimeoutProcessStopped(t, childPID)
			if mode == "resistant" {
				if elapsed < 15*time.Second {
					t.Errorf("resistant parent was killed before its 15-second TERM grace: %s", elapsed)
				}
			}
		})
	}
}

// TestQEMUAllSuiteDeadlineRefusesUnboundedBudgets requires malformed, zero and
// negative caps to fail before either the suite or an external timeout starts.
func TestQEMUAllSuiteDeadlineRefusesUnboundedBudgets(t *testing.T) {
	for _, budget := range []string{"", "0", "0s", "-1s", "invalid"} {
		t.Run(budget, func(t *testing.T) {
			directory := t.TempDir()
			called := filepath.Join(directory, "child-called")
			body := "printf called > " + shellQuote(called) + "\nexit 125\n"
			writeTimeoutExecutable(t, filepath.Join(directory, "timeout"), body)
			writeTimeoutExecutable(t, filepath.Join(directory, leName), body)
			t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
			run := &allTestsRun{Workspace: directory, BinDir: directory, Timeout: budget, Note: func(string) {}}
			result := run.suite(vmSuite{Name: "invalid-budget", Args: []string{"fixture"}, Namespace: guestRoot}, os.Environ())
			if result.Code != 127 {
				t.Errorf("invalid budget %q answered %d, want cannot-start 127", budget, result.Code)
			}
			if _, err := os.Stat(called); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("invalid budget %q started a child: %v", budget, err)
			}
		})
	}
}

func writeTimeoutExecutable(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o700); err != nil {
		t.Fatal(err)
	}
}

// assertTimeoutProcessStopped accepts a gone or zombie process: orphan reaping
// belongs to PID 1, but neither state can execute after the suite returns.
func assertTimeoutProcessStopped(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("child never published its PID: %v", err)
		return
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
		return
	} else if err != nil {
		t.Errorf("cannot observe child %d: %v", pid, err)
		return
	}
	status, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		t.Errorf("cannot read child %d status: %v", pid, err)
		return
	}
	_, fields, ok := strings.Cut(string(status), ") ")
	if !ok {
		t.Errorf("unreadable process stat: %q", status)
		return
	}
	if strings.HasPrefix(fields, "Z ") {
		return
	}
	t.Errorf("suite returned while child %d still executes: %s", pid, status)
}

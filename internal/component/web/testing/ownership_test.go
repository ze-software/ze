// Design: docs/architecture/testing/runner-architecture.md -- owned browser cleanup.
package webtesting

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestBrowserOwnedSessionOverridesAliases checks both the environment boundary
// and the identity delivered to an executable despite poisoned inherited aliases.
func TestBrowserOwnedSessionOverridesAliases(t *testing.T) {
	logPath := installFakeAgentBrowser(t)
	for _, key := range []string{"AGENT_BROWSER_SESSION", "agent.browser.session", "Agent_Browser_Session"} {
		t.Setenv(key, "foreign-do-not-touch")
	}
	browser := newBrowserWithSession("http://127.0.0.1:1", "owned-precedence")
	for _, entry := range browser.agentEnv() {
		if strings.Contains(entry, "foreign-do-not-touch") {
			t.Errorf("inherited session alias escaped into owned command: %s", entry)
		}
	}
	if err := browser.runAgent("eval", "true"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(logPath), "agent-browser-env.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(strings.Split(string(data), "\n"), "AGENT_BROWSER_SESSION=owned-precedence") {
		t.Fatalf("executable did not receive owned identity: %s", data)
	}
}

// TestBrowserCloseHasSeparateBound checks cleanup after case cancellation and
// a hung owned close. Both commands run against a private executable fixture.
func TestBrowserCloseHasSeparateBound(t *testing.T) {
	for _, hang := range []bool{false, true} {
		t.Run(map[bool]string{false: "expired-case", true: "hung-close"}[hang], func(t *testing.T) {
			dir := t.TempDir()
			body := "#!/bin/sh\nexit 0\n"
			if hang {
				body = "#!/bin/sh\nexec sleep 60\n"
			}
			if err := os.WriteFile(filepath.Join(dir, agentBrowserBin), []byte(body), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			previous := agentTimeout
			agentTimeout = 100 * time.Millisecond
			t.Cleanup(func() { agentTimeout = previous })
			browser := newBrowserWithSession("", "owned-bounded-close")
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			browser.ctx = ctx
			err := browser.Close()
			if hang {
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("hung close lost its own deadline: %v", err)
				}
			} else if err != nil {
				t.Fatalf("expired case prevented cleanup: %v", err)
			}
			if browser.ctx != ctx {
				t.Fatal("cleanup replaced the execution context")
			}
		})
	}
}

// TestBrowserAllocatesOwnedSession checks that unnamed instances cannot share
// an inherited session and that each close targets only its own identity.
func TestBrowserAllocatesOwnedSession(t *testing.T) {
	logPath := installFakeAgentBrowser(t)
	first, second := newBrowser(""), newBrowser("")
	if first.session == "" {
		t.Fatal("standalone browser has no owned identity")
	}
	if first.session == second.session {
		t.Fatal("standalone browsers share a session")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	for _, command := range readAgentLog(t, logPath) {
		if strings.Contains(command, "--all") {
			t.Fatalf("owned close targeted foreign sessions: %s", command)
		}
	}
	unnamed := &Browser{ctx: context.Background()}
	before := len(readAgentLog(t, logPath))
	if err := unnamed.Close(); err == nil {
		t.Fatal("unowned browser cleanup did not refuse")
	}
	if len(readAgentLog(t, logPath)) != before {
		t.Fatal("unowned cleanup invoked a command")
	}
}

// TestWBCaseKeepsExecutionAndCleanupFailures ensures failed cleanup cannot
// replace the timeout that caused execution to stop, or discard its steps.
func TestWBCaseKeepsExecutionAndCleanupFailures(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, agentBrowserBin), []byte("#!/bin/sh\nexit 13\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	tc, err := ParseWBFile("option=timeout:value=20ms\naction=wait:ms=80\n")
	if err != nil {
		t.Fatal(err)
	}
	result := runWBTestCase(tc, "http://127.0.0.1:1", "owned-both-errors")
	if result.Passed {
		t.Fatal("execution and cleanup failures passed")
	}
	for _, want := range []string{"option=timeout", "cleanup incomplete", "exit status 13"} {
		if !strings.Contains(result.Error, want) {
			t.Errorf("result %q lost %q", result.Error, want)
		}
	}
	if len(result.Steps) != 1 {
		t.Fatalf("failed step was discarded: %+v", result.Steps)
	}
}

// TestWBCaseReportsCleanupFailure runs a successful step followed by an owned
// close command that exits nonzero. Cleanup failure must defeat a passing case.
func TestWBCaseReportsCleanupFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, agentBrowserBin), []byte("#!/bin/sh\nexit 13\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	tc, err := ParseWBFile("action=wait:ms=1\n")
	if err != nil {
		t.Fatal(err)
	}
	result := runWBTestCase(tc, "http://127.0.0.1:1", "owned-close-error")
	if result.Passed {
		t.Fatal("successful action hid failed owned cleanup")
	}
	for _, want := range []string{"cleanup incomplete", "exit status 13", "owned-close-error"} {
		if !strings.Contains(result.Error, want) {
			t.Errorf("cleanup error %q missing %q", result.Error, want)
		}
	}
	if len(result.Steps) != 1 {
		t.Fatalf("cleanup failure discarded completed steps: %+v", result.Steps)
	}
}

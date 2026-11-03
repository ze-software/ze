// Design: docs/architecture/testing/runner-architecture.md -- browser case deadlines.
package webtesting

import (
	"context"
	"errors"
	"flag"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestWBDeadlineInterruptsWait checks the last action too: a wait that consumes
// the whole budget must fail rather than pass because no next step checks time.
func TestWBDeadlineInterruptsWait(t *testing.T) {
	verifyMode(t, false)
	installFakeAgentBrowser(t)
	tc, err := ParseWBFile("option=timeout:value=20ms\naction=wait:ms=80")
	if err != nil {
		t.Fatal(err)
	}
	result := runWBTestCase(tc, "http://127.0.0.1:1", "deadline-wait")
	if result.Passed {
		t.Fatal("last action passed after exhausting the case deadline")
	}
	if !strings.Contains(result.Error, "option=timeout") {
		t.Fatalf("missing budget cause: %s", result.Error)
	}
}

// TestWBCaseContextPolicy checks scheduler headroom and nesting against exact
// deadlines, without using elapsed time to infer which policy was selected.
func TestWBCaseContextPolicy(t *testing.T) {
	for _, verify := range []bool{false, true} {
		verifyMode(t, verify)
		for _, concurrency := range []int{1, 4} {
			ctx, cancel, err := CaseContext(context.Background(), time.Second, concurrency)
			if err != nil {
				t.Fatal(err)
			}
			want := time.Second
			if concurrency > 1 {
				want *= 3
			}
			if got := Bound(ctx, time.Second); got != want {
				t.Errorf("verify=%t concurrency=%d bound=%s want=%s", verify, concurrency, got, want)
			}
			deadline, _ := ctx.Deadline()
			browser := newBrowser("")
			browser.ctx = ctx
			restore := browser.bounded(time.Minute)
			inner, _ := browser.ctx.Deadline()
			if !inner.Equal(deadline) {
				t.Error("inner operation reset the case deadline")
			}
			restore()
			cancel()
		}
	}
	for _, input := range []struct {
		timeout     time.Duration
		concurrency int
	}{{-1, 1}, {time.Duration(math.MaxInt64), 4}, {time.Second, 0}} {
		if _, cancel, err := CaseContext(context.Background(), input.timeout, input.concurrency); err == nil {
			cancel()
			t.Errorf("invalid policy accepted: %+v", input)
		}
	}
}

// TestWBWaitLoadRefusesNonsettling drives the real command boundary with an
// owned executable whose observed state never settles.
func TestWBWaitLoadRefusesNonsettling(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "agent-browser"), []byte("#!/bin/sh\nprintf 'false\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	browser := newBrowserWithSession("http://127.0.0.1:1", "wait-load-owned")
	browser.ctx = ctx
	err := browser.waitLoad()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("nonsettling state did not fail at deadline: %v", err)
	}
	if !strings.Contains(err.Error(), "did not settle") {
		t.Fatalf("lost last observed state: %v", err)
	}
}

var webDeadlineNative = flag.Bool("web-deadline-native", false, "run the owned local browser deadline proof")

// TestWBDeadlineNativeBrowser proves a real local page opens and a deliberately
// nonsettling state fails. This opt-in test uses only its own named session.
func TestWBDeadlineNativeBrowser(t *testing.T) {
	if !*webDeadlineNative {
		t.Skip("native proof requires -web-deadline-native")
	}
	binary, err := exec.LookPath(agentBrowserBin)
	if err != nil {
		t.Fatal(err)
	}
	owned, err := os.MkdirTemp(os.Getenv("XDG_RUNTIME_DIR"), "ze4a-")
	if err != nil {
		t.Fatal(err)
	}
	// Retain only this run's directory for failure diagnosis. The short runtime
	// path also fits the installed CLI's Unix socket length limit.
	t.Logf("owned native artifacts=%q actual executable=%q", owned, binary)
	t.Setenv("AGENT_BROWSER_SOCKET_DIR", owned)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte("<!doctype html><title>Owned deadline proof</title><p>owned fixture</p>"))
		if err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	session := "phase4a-" + filepath.Base(t.TempDir()) + "-" + time.Now().Format("150405.000000000")
	t.Logf("owned browser session=%q url=%q", session, server.URL)
	t.Setenv("AGENT_BROWSER_SESSION", session)
	t.Setenv("AGENT_BROWSER_DEBUG", "1")
	browser := newBrowserWithSession(server.URL, session)
	logNativeBrowserEnvironment(t, browser)
	defer logNativeBrowserFailure(t, session)
	defer func() {
		if err := browser.Close(); err != nil {
			t.Errorf("owned browser cleanup incomplete: %v", err)
		}
	}()
	ctx, cancel, err := CaseContext(context.Background(), 30*time.Second, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	browser.ctx = ctx
	if err := browser.Open("/"); err != nil {
		t.Errorf("owned browser open failed: %v", err)
		logNativeBrowserFailure(t, session)
		return
	}
	if err := browser.runAgent("eval", "window.__zeInflight=1"); err != nil {
		t.Fatal(err)
	}
	if err := browser.waitLoad(); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("real browser nonsettling state passed: %v", err)
	}
}

// logNativeBrowserFailure reads only this test's exact sidecar names. It never
// invokes session discovery, which can clean up other sessions' stale files.
func logNativeBrowserFailure(t *testing.T, session string) {
	t.Helper()
	if !t.Failed() {
		return
	}
	dir := os.Getenv("AGENT_BROWSER_SOCKET_DIR")
	if dir == "" {
		if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				t.Logf("cannot locate owned browser artifacts: %v", err)
				return
			}
			dir = filepath.Join(home, ".agent-browser")
		} else {
			dir = filepath.Join(runtimeDir, "agent-browser")
		}
	}
	for _, suffix := range []string{".log", ".pid", ".version"} {
		path := filepath.Join(dir, session+suffix)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Logf("owned artifact %q: %v", path, err)
			continue
		}
		t.Logf("owned artifact %q: %s", path, data)
	}
}

// TestWBDeadlineNativePreflight inspects the installed CLI without starting a
// daemon. The session command only parses flags and prints the selected name.
func TestWBDeadlineNativePreflight(t *testing.T) {
	if !*webDeadlineNative {
		t.Skip("native preflight requires -web-deadline-native")
	}
	session := "phase4a-preflight-" + time.Now().Format("150405000000000")
	t.Setenv("AGENT_BROWSER_SESSION", session)
	browser := newBrowserWithSession("", session)
	logNativeBrowserEnvironment(t, browser)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	browser.ctx = ctx
	out, err := browser.runAgentOutput("session")
	if err != nil {
		t.Fatalf("local session query failed: %v", err)
	}
	if strings.TrimSpace(out) != session {
		t.Fatalf("session query=%q want owned identity %q", out, session)
	}
	t.Logf("local session query=%q", strings.TrimSpace(out))
}

func logNativeBrowserEnvironment(t *testing.T, browser *Browser) {
	t.Helper()
	binary, err := exec.LookPath(agentBrowserBin)
	if err != nil {
		t.Fatal(err)
	}
	real, err := filepath.EvalSymlinks(binary)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("owned session=%q executable=%q resolved=%q", browser.session, binary, real)
	for _, entry := range browser.agentEnv() {
		name, value, _ := strings.Cut(entry, "=")
		normalized := strings.ToUpper(strings.ReplaceAll(name, ".", "_"))
		if strings.HasPrefix(normalized, "AGENT_BROWSER_") {
			// Only identity and local path settings are printed, never credentials.
			switch normalized {
			case "AGENT_BROWSER_SESSION", "AGENT_BROWSER_SOCKET_DIR", "AGENT_BROWSER_INIT_SCRIPTS", "AGENT_BROWSER_IDLE_TIMEOUT_MS":
				t.Logf("browser environment %s=%q", name, value)
			default:
				t.Logf("browser environment contains %s", name)
			}
			if normalized == "AGENT_BROWSER_DAEMON" {
				t.Fatal("inherited daemon mode would start a daemon during preflight")
			}
			if normalized == "AGENT_BROWSER_DASHBOARD" {
				t.Fatal("inherited dashboard mode would start a server during preflight")
			}
		}
	}
}

// TestWBDeadlineIgnoresVerifyMode proves verify environment cannot grant a
// serial case extra time. The same over-budget last action fails in both modes.
func TestWBDeadlineIgnoresVerifyMode(t *testing.T) {
	for _, verify := range []bool{false, true} {
		verifyMode(t, verify)
		installFakeAgentBrowser(t)
		tc := &WBTestCase{
			Timeout: 20 * time.Millisecond,
			Actions: []WBAction{{Kind: "wait", Values: map[string]string{"ms": "40"}}},
			Steps:   []WBStep{{Type: WBStepAction, ActionIndex: 0}},
		}
		if result := runWBTestCase(tc, "http://127.0.0.1:1", "deadline-verify"); result.Passed {
			t.Fatalf("verify=%t granted time past the serial budget", verify)
		}
	}
}

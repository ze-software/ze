// Design: docs/architecture/testing/runner-architecture.md -- native command admission.
//go:build integration && linux

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
)

var webCommandBinary = flag.String("web-command-binary", "", "current-source named le executable for command-path proof")
var webCommandCoverDir = flag.String("web-command-cover-dir", "", "retain instrumented command counters in this existing directory")

// TestWebNativeCommand drives the real le test web dispatcher in owned discovery
// roots. Its absent dependencies must not hide malformed or skipped cases; its
// executable fixtures record actual startup and owned browser close.
func TestWebNativeCommand(t *testing.T) {
	if *webCommandBinary == "" {
		t.Skip("command proof requires -web-command-binary")
	}
	for _, mode := range []string{"malformed", "skip", "success", "failure", "deadline", "deadline-auth"} {
		t.Run(mode, func(t *testing.T) {
			var mu sync.Mutex
			var events []webOwnedEvent
			control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				var event webOwnedEvent
				if err := json.NewDecoder(req.Body).Decode(&event); err != nil {
					t.Error(err)
					return
				}
				mu.Lock()
				events = append(events, event)
				mu.Unlock()
				w.WriteHeader(http.StatusOK)
			}))
			defer control.Close()
			root := t.TempDir()
			bins := installWebOwnedExecutables(t, root, control.URL, mode, mode == "failure")
			suite := filepath.Join(root, "test", "web")
			if err := os.MkdirAll(suite, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module owned-web-command\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			body := "action=open:path=/\n"
			switch mode {
			case "malformed":
				body = "option=skip:reason=example\noption=needs-linux:caps=net-admn\n"
			case "skip":
				body = "option=skip:reason=owned-skip\n"
			case "deadline":
				body = "option=timeout:value=1ms\naction=open:path=/\n"
			case "deadline-auth":
				body = "option=timeout:value=5ms\noption=auth:user=owned:password=owned-password:role=admin\naction=open:path=/\n"
			}
			if err := os.WriteFile(filepath.Join(suite, "case.wb"), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			path := root
			if mode == "malformed" || mode == "skip" {
				path = t.TempDir()
			}
			ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, *webCommandBinary, "test", "web", "--all", "--verbose")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "PATH="+path, "ZE_BIN="+bins.ze, "LE_TEST_NO_BUILD=1")
			if *webCommandCoverDir != "" {
				cmd.Env = append(cmd.Env, "GOCOVERDIR="+*webCommandCoverDir)
			}
			plugin.KillGroupOnCancel(cmd)
			cmd.WaitDelay = 2 * time.Second
			var countersBefore int
			if *webCommandCoverDir != "" {
				countersBefore = webCommandCounterCount(t)
			}
			var capture bytes.Buffer
			cmd.Stdout, cmd.Stderr = &capture, &capture
			if err := plugin.StartChild(cmd); err != nil {
				t.Fatal(err)
			}
			err := cmd.Wait()
			output := capture.Bytes()
			if *webCommandCoverDir != "" && webCommandCounterCount(t) <= countersBefore {
				t.Fatalf("native command emitted no coverage counters: %v\n%s", err, output)
			}
			deadlineCase := strings.HasPrefix(mode, "deadline")
			wantFailure := mode == "malformed" || mode == "failure" || deadlineCase
			if (err != nil) != wantFailure {
				t.Fatalf("native command verdict: %v\n%s", err, output)
			}
			if wantFailure {
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 1 {
					t.Fatalf("command did not report test failure: %v\n%s", err, output)
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if mode == "malformed" || mode == "skip" || deadlineCase {
				if len(events) != 0 {
					t.Fatalf("refused command started fixtures: %+v", events)
				}
				cause := "owned-skip"
				if mode == "malformed" {
					cause = "net-admn"
				}
				if deadlineCase {
					cause = "option=timeout"
				}
				if !strings.Contains(string(output), cause) {
					t.Fatalf("command lost %q: %s", cause, output)
				}
				return
			}
			if len(events) < 3 || events[0].Phase != "ready" || events[len(events)-1].Phase != "close" {
				t.Fatalf("command missed startup/browser/close: %+v\n%s", events, output)
			}
			if mode == "failure" && !strings.Contains(string(output), "action open") {
				t.Fatalf("command lost browser error: %s", output)
			}
		})
	}
}

func webCommandCounterCount(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir(*webCommandCoverDir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "covcounters.") {
			count++
		}
	}
	return count
}

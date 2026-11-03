// Design: docs/architecture/testing/runner-architecture.md -- real web callback ownership.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var webOwnedRole = flag.String("web-owned-role", "", "owned web fixture role")
var webOwnedControl = flag.String("web-owned-control", "", "owned web fixture control URL")
var webOwnedID = flag.String("web-owned-id", "", "owned web fixture identity")
var webOwnedFailure = flag.Bool("web-owned-failure", false, "refuse owned browser open")
var webOwnedPIDPath = flag.String("web-owned-pid", "", "owned server pid artifact")

type webOwnedEvent struct {
	ID        string `json:"id"`
	Phase     string `json:"phase"`
	Directory string `json:"directory"`
	PID       int    `json:"pid"`
	Address   string `json:"address"`
	Command   string `json:"command,omitempty"`
}

// TestWebOwnedFixture runs as the server/browser executable reached by the real
// callback. The controller can hold browser close while the server stays owned.
func TestWebOwnedFixture(t *testing.T) {
	if *webOwnedRole == "" {
		return
	}
	if *webOwnedRole == "server" {
		if *webOwnedPIDPath != "" {
			if err := os.WriteFile(*webOwnedPIDPath, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		args := flag.Args()
		for i, arg := range args {
			if arg != "--web" && arg != "--listen" {
				continue
			}
			address := args[i+1]
			if arg == "--web" {
				if !filepath.IsAbs(os.Getenv("ze.config.dir")) {
					t.Fatal("owned server fixture did not receive its absolute config directory")
				}
				address = "127.0.0.1:" + address
			}
			var listenerConfig net.ListenConfig
			listener, err := listenerConfig.Listen(context.Background(), "tcp", address)
			if err != nil {
				t.Fatal(err)
			}
			var ready sync.Once
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				ready.Do(func() {
					if err := webOwnedNotify(req.Context(), webOwnedEvent{ID: *webOwnedID, Phase: "ready", Directory: os.Getenv("ze.config.dir"), PID: os.Getpid(), Address: listener.Addr().String()}); err != nil {
						t.Error(err)
					}
				})
				w.WriteHeader(http.StatusOK)
			}))
			if err := server.Listener.Close(); err != nil {
				t.Fatal(err)
			}
			server.Listener = listener
			if arg == "--web" {
				server.StartTLS()
			} else {
				server.Start()
			}
			t.Cleanup(server.Close)
			// Only the owning callback stops this fixture. A watchdog prevents
			// a broken test from leaving the executable alive indefinitely.
			<-time.After(45 * time.Second)
			t.Fatal("server owner did not stop its fixture")
		}
		t.Fatal("server fixture has no --web port")
	}
	args := strings.Join(flag.Args(), " ")
	if strings.HasSuffix(args, "close") {
		if err := webOwnedNotify(context.Background(), webOwnedEvent{ID: *webOwnedID, Phase: "close"}); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err := webOwnedNotify(context.Background(), webOwnedEvent{ID: *webOwnedID, Phase: "browser", Command: args}); err != nil {
		t.Fatal(err)
	}
	if *webOwnedFailure && strings.Contains(args, "open ") {
		t.Fatal("owned browser open failure")
	}
	if _, err := os.Stdout.WriteString("true\n"); err != nil {
		t.Fatal(err)
	}
}

func webOwnedNotify(ctx context.Context, event webOwnedEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, *webOwnedControl, strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	_, readErr := io.Copy(io.Discard, resp.Body)
	return errors.Join(readErr, resp.Body.Close())
}

// installWebOwnedExecutables creates only private boundary fixtures. Each command
// is still launched, canceled and reaped by the production callback's owners.
func installWebOwnedExecutables(t *testing.T, dir, control, id string, fail bool) zeTestWebBinaries {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"server", "agent-browser"} {
		// Bash retains dotted environment keys; dash drops ze.config.dir before
		// exec, which would make this fixture observe a different startup contract.
		command := strconv.Quote(binary) +
			" -test.run=^TestWebOwnedFixture$ -web-owned-role=" + role +
			" -web-owned-control=" + strconv.Quote(control) + " -web-owned-id=" + strconv.Quote(id) +
			" -web-owned-pid=" + strconv.Quote(filepath.Join(dir, "server.pid")) +
			" -web-owned-failure=" + strconv.FormatBool(fail) + " -- \"$@\"\n"
		script := "#!/bin/bash\nexport GORACE=atexit_sleep_ms=0\nexec " + command
		if role == "agent-browser" {
			// The test harness adds PASS to stdout; only the browser protocol
			// value may reach runAgentOutput's exact boolean check.
			script = "#!/bin/bash\nexport GORACE=atexit_sleep_ms=0\n" + strings.TrimSuffix(command, "\n") +
				" >/dev/null\nstatus=$?\nif [ \"$status\" -eq 0 ]; then printf 'true\\n'; fi\nexit \"$status\"\n"
		}
		if err := os.WriteFile(filepath.Join(dir, role), []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return zeTestWebBinaries{ze: filepath.Join(dir, "server"), zeTest: filepath.Join(dir, "server")}
}

// TestWebCallbackServerCleanupVerdict drives actual TLS startup, browser commands
// and owned server stop. An inaccessible owned subdirectory forces RemoveAll's
// negative branch after the child is joined, for passing and failing execution.
func TestWebCallbackServerCleanupVerdict(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("directory permission failure requires an unprivileged test process")
	}
	for _, fail := range []bool{false, true} {
		t.Run(strconv.FormatBool(fail), func(t *testing.T) {
			var directory string
			var mu sync.Mutex
			coordinator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				var event webOwnedEvent
				if err := json.NewDecoder(req.Body).Decode(&event); err != nil {
					t.Error(err)
					return
				}
				mu.Lock()
				defer mu.Unlock()
				if event.Phase == "ready" {
					directory = event.Directory
				}
				if event.Phase == "close" {
					if !filepath.IsAbs(directory) {
						t.Error("browser cleanup preceded owned server directory discovery")
						return
					}
					blocked := filepath.Join(directory, "owned-cleanup-failure")
					if err := os.Mkdir(blocked, 0o700); err != nil {
						t.Error(err)
						return
					}
					if err := os.WriteFile(filepath.Join(blocked, "marker"), []byte("owned"), 0o600); err != nil {
						t.Error(err)
					}
					if err := os.Chmod(blocked, 0); err != nil {
						t.Error(err)
					}
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer coordinator.Close()
			defer func() {
				mu.Lock()
				defer mu.Unlock()
				if directory != "" {
					if err := os.Chmod(filepath.Join(directory, "owned-cleanup-failure"), 0o700); err != nil {
						t.Error(err)
					}
					if err := os.RemoveAll(directory); err != nil {
						t.Error(err)
					}
				}
			}()
			dir := t.TempDir()
			bins := installWebOwnedExecutables(t, dir, coordinator.URL, "cleanup", fail)
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			body := "action=wait:ms=1\n"
			if fail {
				body += "action=open:path=/\n"
			}
			path := filepath.Join(dir, "owned.wb")
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			test := discoverWebCase(path, 1)
			passed, err := zeTestRunWebTest(context.Background(), test, bins, 1)
			if passed || err == nil || !strings.Contains(err.Error(), "remove server directory") {
				t.Fatalf("server cleanup failure did not fail callback: passed=%v err=%v", passed, err)
			}
			wantSteps := 1
			if fail {
				wantSteps = 2
				if !strings.Contains(err.Error(), "action open") {
					t.Fatalf("cleanup replaced browser execution failure: %v", err)
				}
			}
			if len(test.Steps) != wantSteps || !test.Steps[0].Passed {
				t.Fatalf("cleanup lost completed checker steps: %+v", test.Steps)
			}
			if fail && test.Steps[1].Passed {
				t.Fatal("cleanup changed failed browser step to success")
			}
			if test.GetError() == nil || test.GetError().Error() != err.Error() {
				t.Fatalf("callback error and scheduler payload disagree: %v / %v", err, test.GetError())
			}
		})
	}
}

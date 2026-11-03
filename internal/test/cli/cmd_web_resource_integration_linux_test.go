// Design: docs/architecture/testing/runner-architecture.md -- native web admission and cleanup leases.
//go:build integration && linux

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/test/runner"
)

var webAdmissionRoot = flag.String("web-admission-root", "", "owned web discovery root")
var webAdmissionMode = flag.String("web-admission-mode", "", "owned web scheduler fixture mode")
var webAdmissionCoverDir = flag.String("web-admission-cover-dir", "", "retain native worker count profiles in this existing directory")

type webAdmissionResult struct {
	Error       string `json:"error"`
	Skip        string `json:"skip"`
	ParseFailed bool   `json:"parse-failed"`
	Steps       int    `json:"steps"`
	Reaped      bool   `json:"reaped"`
}

// TestWebAdmissionWorker is a native subprocess using the production discovery,
// scheduler, web callback and process owners. Its result is written only after
// the scheduler returns and an independent WNOWAIT observation verifies reaping.
func TestWebAdmissionWorker(t *testing.T) {
	if *webAdmissionRoot == "" {
		return
	}
	root := *webAdmissionRoot
	if *webAdmissionMode == "unknown-source" {
		if err := unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, ""); err != nil {
			t.Fatalf("private capability-source mount: %v", err)
		}
		if err := unix.Mount(filepath.Join(root, "status"), "/proc/self/status", "", unix.MS_BIND, ""); err != nil {
			t.Fatalf("owned capability-source bind: %v", err)
		}
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	tests := runner.NewTestSet[*zeTestWebTest]()
	if err := discoverWebTests(tests, root, root, webConcurrency); err != nil {
		t.Fatal(err)
	}
	tests.EnableAll()
	if tests.Count() != 1 {
		t.Fatalf("expected one owned case, found %d", tests.Count())
	}
	if *webAdmissionMode == "contend" {
		var stop context.CancelFunc
		ctx, stop = context.WithTimeout(ctx, 350*time.Millisecond)
		defer stop()
	}
	selected := tests.Selected()
	err := runWebTests(ctx, selected, zeTestWebBinaries{ze: filepath.Join(root, "server"), zeTest: filepath.Join(root, "server")}, root, false, true)
	test := selected[0]
	result := webAdmissionResult{Skip: test.admission.SkipReason, ParseFailed: test.admission.ParseFailed, Steps: len(test.Steps)}
	if test.GetError() != nil {
		result.Error = test.GetError().Error()
	} else if err != nil {
		result.Error = err.Error()
	}
	if data, err := os.ReadFile(filepath.Join(root, "server.pid")); err == nil {
		pid, err := strconv.Atoi(string(data))
		if err != nil {
			t.Fatal(err)
		}
		var info unix.Siginfo
		err = unix.Waitid(unix.P_PID, pid, &info, unix.WEXITED|unix.WNOWAIT|unix.WNOHANG, nil)
		result.Reaped = errors.Is(err, syscall.ECHILD)
		if !result.Reaped {
			t.Errorf("callback returned before owned server %d was reaped: %v", pid, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "result.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

type webAdmissionProcess struct {
	cmd    *exec.Cmd
	done   chan error
	root   string
	joined bool
}

// startWebAdmissionProcess owns one native runner, its output file and deadline.
// Cleanup cancels and joins this exact process before its directories are removed.
func startWebAdmissionProcess(t *testing.T, control, id, body, mode string, namespace bool) *webAdmissionProcess {
	t.Helper()
	root := t.TempDir()
	installWebOwnedExecutables(t, root, control, id, mode == "failure")
	if err := os.WriteFile(filepath.Join(root, "case.wb"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "status"), []byte("CapEff:\tbroken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	cmd := exec.CommandContext(ctx, binary, "-test.run=^TestWebAdmissionWorker$", "-web-admission-root="+root, "-web-admission-mode="+mode)
	if *webAdmissionCoverDir != "" {
		if testing.CoverMode() != "count" {
			cancel()
			t.Fatal("native worker profiles require -covermode=count")
		}
		profile, err := os.CreateTemp(*webAdmissionCoverDir, id+"-"+mode+"-*.cover")
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		if err := profile.Close(); err != nil {
			cancel()
			t.Fatal(err)
		}
		cmd.Args = append(cmd.Args, "-test.coverprofile="+profile.Name())
		t.Logf("native worker count profile: %s", profile.Name())
	}
	cmd.Env = append(os.Environ(), "GORACE=atexit_sleep_ms=0")
	plugin.KillGroupOnCancel(cmd)
	cmd.WaitDelay = 2 * time.Second
	if namespace {
		cmd.SysProcAttr.Cloneflags = syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS
		cmd.SysProcAttr.UidMappings = []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}}
		cmd.SysProcAttr.GidMappings = []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}}
		cmd.SysProcAttr.GidMappingsEnableSetgroups = false
	}
	log, err := os.Create(filepath.Join(root, "worker.log"))
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	cmd.Stdout, cmd.Stderr = log, log
	if err := plugin.StartChild(cmd); err != nil {
		cancel()
		if closeErr := log.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatalf("start owned runner (user namespace=%v): %v", namespace, err)
	}
	p := &webAdmissionProcess{cmd: cmd, done: make(chan error, 1), root: root}
	go func() { p.done <- cmd.Wait() }()
	t.Cleanup(func() {
		cancel()
		if !p.joined {
			<-p.done
		}
		if err := log.Close(); err != nil {
			t.Error(err)
		}
	})
	return p
}

func (p *webAdmissionProcess) result(t *testing.T) webAdmissionResult {
	t.Helper()
	err := <-p.done
	p.joined = true
	if err != nil {
		data, readErr := os.ReadFile(filepath.Join(p.root, "worker.log"))
		t.Fatalf("owned runner: %v log-read=%v\n%s", err, readErr, data)
	}
	data, err := os.ReadFile(filepath.Join(p.root, "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result webAdmissionResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

// TestWebNativeExclusiveCleanup uses independent native runners and the actual
// callback's browser-close rendezvous. Same-group admission is blocked during
// cleanup while different groups finish; after success/failure/cancel the lease
// is reusable and the server's sole owner must have reaped it.
func TestWebNativeExclusiveCleanup(t *testing.T) {
	for _, mode := range []string{"success", "failure", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			events := make(chan webOwnedEvent, 64)
			release := make(chan struct{})
			var releaseOnce sync.Once
			defer releaseOnce.Do(func() { close(release) })
			coordinator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				var event webOwnedEvent
				if err := json.NewDecoder(req.Body).Decode(&event); err != nil {
					t.Error(err)
					return
				}
				events <- event
				if event.ID == "holder" && event.Phase == "close" {
					select {
					case <-release:
					case <-req.Context().Done():
					}
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer coordinator.Close()
			// Registered after Close so even a fatal assertion releases handlers.
			defer releaseOnce.Do(func() { close(release) })
			group := t.TempDir()
			body := "option=server:kind=lg-no-engine\noption=exclusive:group=" + group + "\naction=wait:ms=1\n"
			holderBody := body
			if mode == "failure" {
				holderBody += "action=open:path=/\n"
			}
			if mode == "cancel" {
				holderBody += "action=open:path=/\naction=wait:ms=60000\n"
			}
			holder := startWebAdmissionProcess(t, coordinator.URL, "holder", holderBody, mode, false)
			waitEvent := func(id, phase string) webOwnedEvent {
				t.Helper()
				deadline := time.NewTimer(15 * time.Second)
				defer deadline.Stop()
				for {
					select {
					case event := <-events:
						if event.ID == id && event.Phase == phase {
							return event
						}
					case <-deadline.C:
						t.Fatalf("no %s/%s rendezvous", id, phase)
					}
				}
			}
			ready := waitEvent("holder", "ready")
			if mode == "cancel" {
				waitEvent("holder", "browser")
				if err := holder.cmd.Process.Signal(syscall.SIGTERM); err != nil {
					t.Fatal(err)
				}
			}
			waitEvent("holder", "close")
			_, portText, err := net.SplitHostPort(ready.Address)
			if err != nil {
				t.Fatal(err)
			}
			port, err := strconv.Atoi(portText)
			if err != nil {
				t.Fatal(err)
			}
			reservation, acquired, err := runner.LockPort(port)
			if err != nil {
				t.Fatal(err)
			}
			if acquired {
				reservation.Release()
				t.Fatal("server port lease was released during browser cleanup")
			}
			contender := startWebAdmissionProcess(t, coordinator.URL, "same", body, "contend", false)
			result := contender.result(t)
			if result.Error == "" || !strings.Contains(result.Error, "deadline exceeded") || result.Steps != 0 || result.Reaped {
				t.Fatalf("same group entered during cleanup: %+v", result)
			}
			if _, err := os.Stat(filepath.Join(contender.root, "server.pid")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("waiting same group started a server: %v", err)
			}
			broken := startWebAdmissionProcess(t, coordinator.URL, "broken", "option=exclusive:group="+group+"\noption=skip:reason=example\noption=needs-linux:caps=net-admn\n", "contend", false)
			result = broken.result(t)
			if !result.ParseFailed || !strings.Contains(result.Error, "net-admn") || result.Steps != 0 {
				t.Fatalf("held lease or skip hid malformed capability: %+v", result)
			}
			if _, err := os.Stat(filepath.Join(broken.root, "server.pid")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("malformed case started a server: %v", err)
			}
			other := startWebAdmissionProcess(t, coordinator.URL, "different", strings.Replace(body, group, group+"-other", 1), "success", false)
			result = other.result(t)
			if result.Error != "" || !result.Reaped || result.Steps != 1 {
				t.Fatalf("different group did not finish independently: %+v", result)
			}
			releaseOnce.Do(func() { close(release) })
			result = holder.result(t)
			if !result.Reaped {
				t.Fatalf("holder returned before server reap: %+v", result)
			}
			if (result.Error == "") != (mode == "success") {
				t.Fatalf("holder lost its execution verdict: %+v", result)
			}
			next := startWebAdmissionProcess(t, coordinator.URL, "next", body, "success", false)
			result = next.result(t)
			if result.Error != "" || !result.Reaped || result.Steps != 1 {
				t.Fatalf("cleanup did not release its lease: %+v", result)
			}
		})
	}
}

// TestWebNativeCapabilityAdmission reads real effective capabilities in native
// runner processes. A private user namespace supplies the capable target; a
// private mount namespace supplies an invalid effective-capability source.
// Neither namespace changes the host's capabilities or procfs mount.
func TestWebNativeCapabilityAdmission(t *testing.T) {
	var host runner.Record
	if _, err := runner.ParseResourceOption(&host, "needs-linux:caps=net-admin"); err != nil {
		t.Fatal(err)
	}
	if host.SkipReason == "" {
		t.Skip("missing-capability control requires a host without effective net-admin")
	}
	for _, mode := range []string{"missing", "capable", "unknown-source", "malformed"} {
		t.Run(mode, func(t *testing.T) {
			var mu sync.Mutex
			var events []webOwnedEvent
			coordinator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
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
			defer coordinator.Close()
			body := "option=server:kind=lg-no-engine\noption=needs-linux:caps=net-admin\naction=wait:ms=1\n"
			if mode == "malformed" {
				body = "option=skip:reason=example\noption=needs-linux:caps=net-admn\n"
			}
			process := startWebAdmissionProcess(t, coordinator.URL, mode, body, mode, mode == "capable" || mode == "unknown-source")
			result := process.result(t)
			mu.Lock()
			defer mu.Unlock()
			if mode == "capable" {
				if result.Skip != "" || result.Error != "" || result.ParseFailed || !result.Reaped || result.Steps != 1 {
					t.Fatalf("selected effective capability did not execute: %+v", result)
				}
				if len(events) < 2 || events[0].Phase != "ready" || events[len(events)-1].Phase != "close" {
					t.Fatalf("capable target missed server/browser execution: %+v", events)
				}
				return
			}
			if len(events) != 0 {
				t.Fatalf("refused target started a server or browser: %+v", events)
			}
			if _, err := os.Stat(filepath.Join(process.root, "server.pid")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("refused target created a server marker: %v", err)
			}
			if result.Steps != 0 || result.Reaped {
				t.Fatalf("refused target reached execution: %+v", result)
			}
			if mode == "missing" {
				if result.Error != "" || result.Skip == "" || result.ParseFailed {
					t.Fatalf("missing capability was not an explicit skip: %+v", result)
				}
				return
			}
			cause := "cannot parse CapEff"
			if mode == "malformed" {
				cause = "net-admn"
			}
			if !result.ParseFailed || !strings.Contains(result.Error, cause) || result.Skip != "" {
				t.Fatalf("invalid capability source/declaration was not a failure: %+v", result)
			}
		})
	}
}

// TestWebNativeRunnableDiscoverySnapshot replaces or removes a runnable authored
// file after discovery, then drives the real callback with owned executables.
// Its action, passing expectation and final verdict must come from the snapshot.
// MUTATION: Reread test.Path instead of passing tc to RunWBCase in zeTestRunWebTest.
func TestWebNativeRunnableDiscoverySnapshot(t *testing.T) {
	for _, mode := range []string{"replace", "remove"} {
		t.Run(mode, func(t *testing.T) {
			for _, verdict := range []string{"pass", "fail"} {
				t.Run(verdict, func(t *testing.T) {
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
					bins := installWebOwnedExecutables(t, root, control.URL, mode+"-"+verdict, false)
					t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
					body := "option=server:kind=lg-no-engine\n" +
						"action=open:path=/snapshot-original\n" +
						"expect=element:text=true\n"
					if verdict == "fail" {
						body += "expect=element:not-text=true\n"
					}
					path := filepath.Join(root, "snapshot.wb")
					if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
						t.Fatal(err)
					}
					tests := runner.NewTestSet[*zeTestWebTest]()
					if err := discoverWebTests(tests, root, root, 1); err != nil {
						t.Fatal(err)
					}
					if tests.Count() != 1 {
						t.Fatalf("discovered %d cases, want one runnable snapshot", tests.Count())
					}
					tests.EnableAll()
					test := tests.Selected()[0]
					if test.admission.ParseFailed {
						t.Fatal(test.GetError())
					}
					if test.admission.SkipReason != "" {
						t.Fatalf("runnable snapshot was skipped: %s", test.admission.SkipReason)
					}
					if mode == "replace" {
						replacement := "option=server:kind=lg-no-engine\n" +
							"action=open:path=/snapshot-replaced\n" +
							"expect=element:text=true\n"
						if verdict == "pass" {
							replacement += "expect=element:not-text=true\n"
						}
						if err := os.WriteFile(path, []byte(replacement), 0o600); err != nil {
							t.Fatal(err)
						}
					} else if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
					passed, err := zeTestRunWebTest(context.Background(), test, bins, 1)
					if verdict == "pass" {
						if !passed || err != nil || test.GetError() != nil {
							t.Fatalf("original passing case changed verdict: passed=%v err=%v retained=%v", passed, err, test.GetError())
						}
					} else {
						if passed || err == nil {
							t.Fatalf("original failing expectation changed verdict: passed=%v err=%v", passed, err)
						}
						if !strings.Contains(err.Error(), "line 4: expect element:") {
							t.Fatalf("original failing expectation location was lost: %v", err)
						}
						if test.GetError() == nil {
							t.Fatal("failed snapshot verdict was not retained")
						}
						if test.GetError().Error() != err.Error() {
							t.Fatalf("retained verdict differs: %v; callback: %v", test.GetError(), err)
						}
					}
					wantSteps := 2
					if verdict == "fail" {
						wantSteps = 3
					}
					if len(test.Steps) != wantSteps {
						t.Fatalf("original action/expectations were lost: %+v", test.Steps)
					}
					for i, step := range test.Steps {
						kind, assertion := "expect", "element"
						if i == 0 {
							kind, assertion = "action", "open"
						}
						if step.Step != i+1 || step.Line != i+2 || step.Kind != kind || step.Assert != assertion {
							t.Fatalf("step %d did not execute the original authored directive: %+v", i+1, step)
						}
						if step.Passed != (i < 2) {
							t.Fatalf("step %d changed original expectation outcome: %+v", i+1, step)
						}
					}
					mu.Lock()
					defer mu.Unlock()
					if len(events) < 3 || events[0].Phase != "ready" || events[len(events)-1].Phase != "close" {
						t.Fatalf("runnable snapshot missed owned startup/browser/close: %+v", events)
					}
					openedOriginal := false
					for _, event := range events {
						if strings.Contains(event.Command, "open ") {
							if !strings.HasSuffix(event.Command, "/snapshot-original") {
								t.Fatalf("browser executed the replacement action: %+v", event)
							}
							openedOriginal = true
						}
					}
					if !openedOriginal {
						t.Fatalf("original navigation never reached the browser executable: %+v", events)
					}
				})
			}
		})
	}
}

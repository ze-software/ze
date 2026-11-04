// Design: docs/contributing/terminal-demos.md -- owned browser journey startup.
package siteterminaldemo

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestWebConfigJourneyOwnsStartup exercises the actual validation and recording
// entries with a real direct child stalled in init, import or daemon readiness.
// The child's finite safety exit keeps the pre-repair regression bounded.
func TestWebConfigJourneyOwnsStartup(t *testing.T) {
	for _, entry := range [][]string{{"validate", "web-config"}, {"run", "web-config", "record"}} {
		for _, stage := range []string{"init", "import", "readiness"} {
			t.Run(strings.Join(entry, "-")+"/"+stage, func(t *testing.T) {
				root := webJourneyFixture(t, stage, "option=timeout:value=1s\naction=wait:ms=1\n")
				result := make(chan string, 1)
				go func() {
					var output, diagnostic bytes.Buffer
					code := RuntimeMain(entry, &output, &diagnostic)
					result <- strconv.Itoa(code) + ":" + diagnostic.String()
				}()
				pid := webJourneyChildPID(t, root, "stalled")
				select {
				case diagnostic := <-result:
					if !strings.Contains(diagnostic, "option=timeout") {
						t.Errorf("lost authored startup deadline: %s", diagnostic)
					}
				case <-time.After(2500 * time.Millisecond):
					t.Error("actual journey entry did not return within startup deadline and cleanup allowance")
					<-result
				}
				webJourneyAssertJoined(t, pid)
				if _, err := os.Stat(filepath.Join(root, "browser-started")); !os.IsNotExist(err) {
					t.Errorf("browser started after stalled startup: %v", err)
				}
			})
		}
	}
}

// TestWebConfigJourneyJoinsDaemon judges direct-child exit and reaping, not a
// PID-file deletion: ordinary stop, TERM resistance, failed readiness and the
// preservation of both an earlier browser action error and a daemon cleanup error.
func TestWebConfigJourneyJoinsDaemon(t *testing.T) {
	for _, mode := range []string{"normal", "resistant", "failed-start", "cleanup-error"} {
		t.Run(mode, func(t *testing.T) {
			journey := "option=timeout:value=30s\naction=wait:ms=1\n"
			if mode == "cleanup-error" {
				journey = "option=timeout:value=30s\naction=press\n"
			}
			root := webJourneyFixture(t, mode, journey)
			var output, diagnostic bytes.Buffer
			code := RuntimeMain([]string{"validate", "web-config"}, &output, &diagnostic)
			pid := webJourneyChildPID(t, root, "daemon")
			webJourneyAssertJoined(t, pid)
			if _, err := os.Stat(filepath.Join(root, "browser-started")); !os.IsNotExist(err) {
				t.Errorf("daemon ownership case unexpectedly launched browser work: %v", err)
			}
			switch mode {
			case "normal", "resistant":
				if code != 0 {
					t.Errorf("owned %s stop failed: %s", mode, diagnostic.String())
				}
			case "failed-start":
				if code == 0 || !strings.Contains(diagnostic.String(), "exit status 23") {
					t.Errorf("failed daemon startup lost its exit error: code=%d %s", code, diagnostic.String())
				}
			case "cleanup-error":
				if code == 0 || !strings.Contains(diagnostic.String(), "press action requires key=") || !strings.Contains(diagnostic.String(), "exit status 23") {
					t.Errorf("browser and cleanup errors were not both retained: code=%d %s", code, diagnostic.String())
				}
			}
		})
	}
}

func webJourneyFixture(t *testing.T, mode, journey string) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("ZE_DEMO_ROOT", root)
	t.Setenv("ZE_DEMO_STATE_ROOT", filepath.Join(root, "state"))
	t.Setenv("ZE_JOURNEY_CHILD_ROOT", root)
	t.Setenv("ZE_JOURNEY_CHILD_MODE", mode)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZE_JOURNEY_TEST_BINARY", executable)
	bin := filepath.Join(root, "tmp", "terminal-demos", "bin")
	mustWrite(t, filepath.Join(bin, "ze"), "#!/bin/sh\nexport GORACE=\"$GORACE atexit_sleep_ms=0\"\nexec \"$ZE_JOURNEY_TEST_BINARY\" -test.run=^TestWebJourneyChild$ -- \"$@\"\n")
	mustWrite(t, filepath.Join(bin, "agent-browser"), "#!/bin/sh\nif [ \"$#\" -eq 1 ] && [ \"$1\" = close ]; then exit 0; fi\nif [ \"$#\" -eq 2 ] && [ \"$1\" = --ignore-https-errors ] && [ \"$2\" = close ]; then exit 0; fi\ntouch \"$ZE_JOURNEY_CHILD_ROOT/browser-started\"\nexit 71\n")
	for _, name := range []string{"ze", "agent-browser"} {
		if err := os.Chmod(filepath.Join(bin, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	mustWrite(t, filepath.Join(root, recordedWebConfigJourney), journey)
	mustWrite(t, filepath.Join(root, "demos", "terminal", "web-config", "ze.conf"), "\n")
	return root
}

func webJourneyChildPID(t *testing.T, root, name string) int {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err == nil {
			pid, err := strconv.Atoi(string(data))
			if err != nil {
				t.Fatal(err)
			}
			return pid
		}
		if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		select {
		case <-deadline:
			t.Fatalf("child never rendezvoused at %s", name)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func webJourneyAssertJoined(t *testing.T, pid int) {
	t.Helper()
	var status syscall.WaitStatus
	child, err := syscall.Wait4(pid, &status, syscall.WNOHANG, nil)
	if !errors.Is(err, syscall.ECHILD) {
		t.Errorf("journey returned without joining direct child %d: wait=%d error=%v", pid, child, err)
		if child == 0 {
			process, findErr := os.FindProcess(pid)
			if findErr != nil {
				t.Fatal(findErr)
			}
			if killErr := process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
				t.Error(killErr)
			}
			if _, waitErr := process.Wait(); waitErr != nil {
				var exited *exec.ExitError
				if !errors.As(waitErr, &exited) {
					t.Error(waitErr)
				}
			}
		}
	}
}

// TestWebJourneyChild is a real subprocess with an observable startup rendezvous
// and signal-driven exit. It never implements browser rendering or assertions.
func TestWebJourneyChild(t *testing.T) {
	root := os.Getenv("ZE_JOURNEY_CHILD_ROOT")
	if root == "" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) < 2 {
		t.Fatal("missing child invocation")
	}
	args = args[1:]
	mode := os.Getenv("ZE_JOURNEY_CHILD_MODE")
	stage := args[0]
	if stage == "config" && len(args) > 1 {
		stage = args[1]
	}
	if stage == "start" {
		if err := os.WriteFile(filepath.Join(root, "daemon"), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
			t.Fatal(err)
		}
		if mode == "failed-start" {
			os.Exit(23)
		}
	}
	if stage == mode || (stage == "start" && mode == "readiness") {
		if err := os.WriteFile(filepath.Join(root, "stalled"), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
			t.Fatal(err)
		}
		time.Sleep(4 * time.Second)
		return
	}
	if stage != "start" {
		return
	}
	stopped := make(chan os.Signal, 1)
	signal.Notify(stopped, syscall.SIGTERM)
	defer signal.Stop(stopped)
	if _, err := os.Stdout.WriteString("web server listening\n"); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case <-stopped:
			if mode == "resistant" {
				continue
			}
			if mode == "cleanup-error" {
				os.Exit(23)
			}
			return
		case <-time.After(5 * time.Second):
			return
		}
	}
}

// Design: docs/architecture/testing/runner-architecture.md -- editor lease lifetime.

//go:build linux

package editorcase

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestEditorSuiteNativeExclusive observes actual child entry through a FIFO.
// A waiter opens the shared group lock before an independent group enters.
// Every release route must reap the holder and remove work before waiter entry.
// MUTATION: drop ExclusiveGroup in the editor scheduler adapter.
func TestEditorSuiteNativeExclusive(t *testing.T) {
	for _, outcome := range []string{"success", "failure", "cancel"} {
		t.Run(outcome, func(t *testing.T) {
			group := t.TempDir()
			holder := startLeaseSuite(t, group, outcome)
			holder.enter(t)
			work := holder.ownedWork(t)
			waiter := startLeaseSuite(t, group, "success")
			waiter.await(t, func() bool {
				if waiter.tryEntry(t) {
					t.Fatal("same-group child entered before group admission")
				}
				return waiter.hasGroupDescriptor(t)
			})
			canceled := startLeaseSuite(t, group, "success")
			canceled.await(t, func() bool { return canceled.hasGroupDescriptor(t) })
			if err := canceled.cmd.Process.Signal(syscall.SIGTERM); err != nil {
				t.Fatal(err)
			}
			canceled.join(t, true)
			if canceled.tryEntry(t) {
				t.Fatal("canceled waiter entered its editor")
			}
			canceled.requireReport(t, "context canceled")
			if err := filepath.WalkDir(canceled.root, func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.Name() == "request.json" {
					t.Errorf("canceled waiter allocated child resources: %s", path)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			other := startLeaseSuite(t, group+"-independent", "success")
			other.enter(t)
			if waiter.tryEntry(t) {
				t.Fatal("same-group child entered while holder and independent child were active")
			}
			other.release(t)
			other.join(t, false)
			if outcome == "cancel" {
				if err := holder.cmd.Process.Signal(syscall.SIGTERM); err != nil {
					t.Fatal(err)
				}
			} else {
				holder.release(t)
			}
			// Check at the successor's first observable editor effect, not after
			// waiting for the holder's suite reporter (which could hide overlap).
			waiter.enter(t)
			if holder.ctx.Err() != nil {
				t.Fatal("holder watchdog invalidated lease observation")
			}
			if _, err := os.Stat(work); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("successor entered before holder cleanup: %v", err)
			}
			holder.join(t, outcome != "success")
			if outcome == "cancel" {
				holder.requireReapedReport(t)
			}
			if outcome == "failure" {
				holder.requireReport(t, "LEASE-MUST-FAIL")
			}
			waiter.release(t)
			waiter.join(t, false)
		})
	}
}

// TestEditorSuiteNativeLinuxTarget runs a bare needs-linux declaration on the
// actual Linux host, alongside an undeclared case. Both must enter their editors
// concurrently and pass, with no capability grant or implicit exclusive group.
func TestEditorSuiteNativeLinuxTarget(t *testing.T) {
	linux := startLeaseSuite(t, "", "success")
	linux.enter(t)
	plain := startLeaseSuite(t, "", "default")
	plain.enter(t)
	linux.release(t)
	plain.release(t)
	linux.join(t, false)
	plain.join(t, false)
}

// leaseSuite owns exactly one native suite and its wait worker. Cleanup signals
// that suite and joins it before removing the test's files. writer holds the
// authored FIFO read until release or parent cancellation.
type leaseSuite struct {
	cmd             *exec.Cmd
	ctx             context.Context
	done            chan error
	root, fifo, log string
	writer          int
	joined          bool
}

func startLeaseSuite(t *testing.T, group, outcome string) *leaseSuite {
	t.Helper()
	executable := admissionExecutable(t)
	root := t.TempDir()
	fifo := filepath.Join(root, "entry.conf")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module lease-proof\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	content := ""
	if outcome != "default" {
		content = "option=needs-linux\n"
	}
	if group != "" {
		content += "option=exclusive:group=" + group + "\n"
	}
	content += "tmpfs=test.conf:terminator=END\nbgp { router-id 1.2.3.4; }\nEND\n" +
		"option=file:path=test.conf\noption=timeout:value=20s\n" +
		"input=type:text=load file absolute replace " + fifo + "\ninput=enter\n"
	if outcome == "failure" {
		content += "expect=content:contains=LEASE-MUST-FAIL\n"
	}
	if err := os.WriteFile(filepath.Join(root, "lease.et"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	log, err := os.Create(filepath.Join(root, "suite.log"))
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, executable, "test", "editor", "--dir", root, "--all")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "TMPDIR="+root, "ze_le_store_trim=off")
	cmd.Stdout, cmd.Stderr = log, log
	plugin.KillGroupOnCancel(cmd)
	if err := plugin.StartChild(cmd); err != nil {
		cancel()
		log.Close() //nolint:errcheck // Preserve the start failure.
		t.Fatal(err)
	}
	suite := &leaseSuite{cmd: cmd, ctx: ctx, done: make(chan error, 1), root: root, fifo: fifo, log: log.Name(), writer: -1}
	go func() { suite.done <- cmd.Wait() }()
	t.Cleanup(func() {
		if !suite.joined {
			cmd.Process.Signal(syscall.SIGTERM) //nolint:errcheck // An exited owned suite needs no signal.
			<-suite.done
		}
		cancel()
		if suite.writer >= 0 {
			if err := syscall.Close(suite.writer); err != nil {
				t.Error(err)
			}
		}
		log.Close() //nolint:errcheck // Output is checked before cleanup.
	})
	return suite
}

func (suite *leaseSuite) await(t *testing.T, ready func() bool) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for !ready() {
		select {
		case err := <-suite.done:
			suite.joined = true
			body, readErr := os.ReadFile(suite.log)
			t.Fatalf("suite exited before rendezvous: %v; read=%v\n%s", err, readErr, body)
		case <-suite.ctx.Done():
			t.Fatal("suite rendezvous needed watchdog")
		case <-ticker.C:
		}
	}
}

func (suite *leaseSuite) tryEntry(t *testing.T) bool {
	t.Helper()
	if suite.writer >= 0 {
		return true
	}
	fd, err := syscall.Open(suite.fifo, syscall.O_WRONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if errors.Is(err, syscall.ENXIO) {
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	suite.writer = fd
	return true
}

func (suite *leaseSuite) enter(t *testing.T) {
	t.Helper()
	suite.await(t, func() bool { return suite.tryEntry(t) })
}

func (suite *leaseSuite) release(t *testing.T) {
	t.Helper()
	if _, err := syscall.Write(suite.writer, []byte("bgp { router-id 1.2.3.4; }\n")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Close(suite.writer); err != nil {
		t.Fatal(err)
	}
	suite.writer = -1
}

func (suite *leaseSuite) join(t *testing.T, fail bool) {
	t.Helper()
	err := <-suite.done
	suite.joined = true
	if suite.ctx.Err() != nil {
		t.Fatal("suite cleanup needed watchdog")
	}
	if (err != nil) != fail {
		body, readErr := os.ReadFile(suite.log)
		t.Fatalf("unexpected suite exit: %v; read=%v\n%s", err, readErr, body)
	}
}

// hasGroupDescriptor witnesses this owned runner reaching the real lease open.
// The holder's active FIFO keeps that lock unavailable; no sleep guesses startup.
func (suite *leaseSuite) hasGroupDescriptor(t *testing.T) bool {
	t.Helper()
	path := filepath.Join("/proc", strconv.Itoa(suite.cmd.Process.Pid), "fd")
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join(path, entry.Name()))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(filepath.Base(target), "group-") {
			return true
		}
	}
	return false
}

// ownedWork reads only this suite's request. The FIFO rendezvous ensures request
// publication and child startup have already happened. Phase 2C proves that the
// owner removes work only after its sole Wait finishes; no proc child scan is used.
func (suite *leaseSuite) ownedWork(t *testing.T) string {
	t.Helper()
	var work string
	if err := filepath.WalkDir(suite.root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Name() != "request.json" {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var req request
		if err := json.Unmarshal(body, &req); err != nil {
			return err
		}
		work = req.Directory
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if work == "" {
		t.Fatal("holder has no owned request")
	}
	return work
}

func (suite *leaseSuite) requireReport(t *testing.T, want string) string {
	t.Helper()
	body, err := os.ReadFile(suite.log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), want) {
		t.Fatalf("missing %q:\n%s", want, body)
	}
	return string(body)
}

func (suite *leaseSuite) requireReapedReport(t *testing.T) {
	t.Helper()
	report := suite.requireReport(t, "editor child pid ")
	_, rest, _ := strings.Cut(report, "editor child pid ")
	identity, _, _ := strings.Cut(rest, ":")
	pid, err := strconv.Atoi(identity)
	if err != nil {
		t.Fatal(err)
	}
	if pid <= 0 {
		t.Fatal("invalid owned child pid")
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("reported child survived reap: %v", err)
	}
}

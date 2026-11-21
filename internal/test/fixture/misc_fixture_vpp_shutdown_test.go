// Design: docs/functional-tests.md -- VPP stub shutdown acceptance.
// Related: misc_fixture_vpp_shutdown.go -- raw daemon exit and cleanup verdict.
package fixture

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const vppShutdownChildReady = "vpp shutdown child ready"

// TestVPPShutdownAcceptsNormalExit starts a real signal-handling child and
// requires its normal completion; the cached raw wait remains observable.
func TestVPPShutdownAcceptsNormalExit(t *testing.T) {
	child := startVPPShutdownChild(t, "clean")
	if err := vppShutdownStopZe(child); err != nil {
		t.Fatal(err)
	}
	if err := waitFixtureProcess(t.Context(), child, 3*time.Second); err != nil {
		t.Fatalf("clean stop consumed or changed the original wait status: %v", err)
	}
}

// TestVPPShutdownRejectsNonzeroExit starts a real child that handles TERM but
// exits with failure instead of letting a completed stop imply success.
func TestVPPShutdownRejectsNonzeroExit(t *testing.T) {
	child := startVPPShutdownChild(t, "failure")
	if err := vppShutdownStopZe(child); err == nil {
		t.Fatal("nonzero daemon exit was accepted as clean shutdown")
	}
	if err := waitFixtureProcess(t.Context(), child, 3*time.Second); err == nil {
		t.Fatal("shutdown check consumed or normalized the nonzero wait status")
	}
	status, ok := errors.AsType[*exec.ExitError](waitFixtureProcess(t.Context(), child, 3*time.Second))
	if !ok {
		t.Fatal("nonzero child has no actual exit status")
	}
	if status.ExitCode() != 17 {
		t.Fatalf("child exit = %d, want 17", status.ExitCode())
	}
}

// TestVPPShutdownRejectsSignalExit lets the kernel's default TERM action end
// the real child; printing the completion text beforehand cannot make it clean.
func TestVPPShutdownRejectsSignalExit(t *testing.T) {
	child := startVPPShutdownChild(t, "signal")
	if err := vppShutdownStopZe(child); err == nil {
		t.Fatal("SIGTERM death was accepted as clean shutdown")
	}
	assertVPPShutdownSignal(t, child, syscall.SIGTERM)
}

// TestVPPShutdownRejectsForcedExit holds a real child past the existing stop
// grace. Its eventual forced kill must remain a failing acceptance result.
func TestVPPShutdownRejectsForcedExit(t *testing.T) {
	child := startVPPShutdownChild(t, "ignore")
	if err := vppShutdownStopZe(child); err == nil {
		t.Fatal("SIGKILL after the stop grace was accepted as clean shutdown")
	}
	assertVPPShutdownSignal(t, child, syscall.SIGKILL)
}

// TestVPPShutdownRefusesPreexistingExit joins a real zero exit before the stop
// request. The verdict must refuse it and preserve the original cached status.
func TestVPPShutdownRefusesPreexistingExit(t *testing.T) {
	child := startVPPShutdownChild(t, "exited")
	if err := waitFixtureProcess(t.Context(), child, 3*time.Second); err != nil {
		t.Fatal(err)
	}
	err := vppShutdownStopZe(child)
	if err == nil {
		t.Fatal("a daemon that exited before shutdown was accepted")
	}
	if !strings.Contains(err.Error(), "before requested shutdown") {
		t.Fatalf("preexisting exit refusal = %v", err)
	}
	if err := waitFixtureProcess(t.Context(), child, 3*time.Second); err != nil {
		t.Fatalf("preexisting exit status was consumed: %v", err)
	}
}

// TestVPPShutdownRequiresCompletionMessage proves a real zero exit alone does
// not establish that the daemon's normal shutdown path reached completion.
func TestVPPShutdownRequiresCompletionMessage(t *testing.T) {
	child := startVPPShutdownChild(t, "silent")
	if err := vppShutdownStopZe(child); err == nil {
		t.Fatal("zero exit without the shutdown completion was accepted")
	}
	if err := waitFixtureProcess(t.Context(), child, 3*time.Second); err != nil {
		t.Fatalf("completion check changed the actual zero exit: %v", err)
	}
}

// TestVPPShutdownRejectsCleanupDiagnostics runs real successful children whose
// output contains a producer-declared cleanup failure beside normal completion.
func TestVPPShutdownRejectsCleanupDiagnostics(t *testing.T) {
	for _, failure := range []string{
		diagnostic.CodeDoctorPluginShutdownOrder,
		diagnostic.CodeDoctorPluginStopCleanupLate,
		diagnostic.CodeDoctorPluginStopUnfinished,
		diagnostic.CodeDoctorIfaceOwnedDeviceCleanup,
		diagnostic.CodeDoctorIfaceBackendClose,
		"warning: shutdown timeout:",
		"vpp manager stopped with an error",
	} {
		t.Run(failure, func(t *testing.T) {
			child := startVPPShutdownChild(t, "warning", failure)
			if err := vppShutdownStopZe(child); err == nil {
				t.Fatalf("cleanup failure %q was accepted beside normal completion", failure)
			}
			if err := waitFixtureProcess(t.Context(), child, 3*time.Second); err != nil {
				t.Fatalf("diagnostic rejection changed the actual zero exit: %v", err)
			}
		})
	}
}

// TestVPPShutdownOwnsDiagnosticCapture exercises the actual logger in fresh
// children. Hostile inherited levels/backend hide each control; the same
// child's real warning must be captured and rejected with the fixture's env.
func TestVPPShutdownOwnsDiagnosticCapture(t *testing.T) {
	for _, question := range []struct {
		subsystem string
		failure   string
	}{
		{"interface", diagnostic.CodeDoctorIfaceOwnedDeviceCleanup},
		{"plugin", diagnostic.CodeDoctorPluginStopCleanupLate},
		{"vpp", "vpp manager stopped with an error"},
	} {
		t.Run(question.subsystem, func(t *testing.T) {
			dir := t.TempDir()
			socket, receipt := startVPPShutdownSyslog(t)
			inherited := childEnvironment(os.Environ(), map[string]string{
				"ZE_LOG":              "disabled",
				"ZE_LOG_INTERFACE":    "disabled",
				"ze.log.plugin":       "disabled",
				"ZE_LOG_VPP":          "disabled",
				"ze.log.relay":        "disabled",
				"ze.log.plugin.relay": "disabled",
				"ZE_LOG_BACKEND":      "syslog",
				"ze.log.destination":  socket,
			})
			disabled := childEnvironment(inherited, map[string]string{"ze.log.backend": "stderr"})
			control := startVPPShutdownLoggerChild(t, disabled, question.subsystem, "logger")
			if err := vppShutdownStopZe(control); err != nil {
				t.Fatalf("disabled logger control failed before capture: %v", err)
			}
			if strings.Contains(control.output.String(), question.failure) {
				t.Fatalf("hostile logging control unexpectedly captured %s", question.failure)
			}
			// Start on stderr to keep package-init loggers out of the sink.
			// The child's role selects the actual syslog backend immediately
			// before constructing its actual warning logger.
			routed := childEnvironment(inherited, map[string]string{
				"ze.log":           logLevelInfo,
				"ze.log.interface": logLevelWarn,
				"ze.log.plugin":    logLevelWarn,
				envLogVPP:          logLevelInfo,
				"ze.log.backend":   "stderr",
			})
			syslogChild := startVPPShutdownLoggerChild(t, routed, question.subsystem, "logger-syslog")
			if err := vppShutdownStopZe(syslogChild); err != nil {
				t.Fatalf("syslog control failed before its real sink verdict: %v", err)
			}
			select {
			case actual := <-receipt:
				if actual.err != nil {
					t.Fatalf("syslog sink could not observe its control: %v", actual.err)
				}
				if !strings.Contains(actual.line, question.failure) {
					t.Fatalf("actual syslog record %q lacks %s", actual.line, question.failure)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("syslog sink did not receive its control; backend evidence is missing")
			}
			if strings.Contains(syslogChild.output.String(), question.failure) {
				t.Fatalf("syslog control unexpectedly reached captured stderr: %s", question.failure)
			}
			owned := childEnvironment(inherited, vppShutdownEnvironment(dir, filepath.Join(dir, "ready")))
			child := startVPPShutdownLoggerChild(t, owned, question.subsystem, "logger")
			if err := vppShutdownStopZe(child); err == nil {
				t.Fatalf("fixture logging hid the actual %s failure", question.subsystem)
			}
			if !strings.Contains(child.output.String(), question.failure) {
				t.Fatalf("fixture did not capture %s:\n%s", question.failure, child.output.String())
			}
			if err := waitFixtureProcess(t.Context(), child, 3*time.Second); err != nil {
				t.Fatalf("actual logger child did not exit zero: %v", err)
			}
		})
	}
}

// vppShutdownSyslogReceipt carries the bounded real sink observation. An error
// is an unobserved control, never proof that the backend suppressed a warning.
type vppShutdownSyslogReceipt struct {
	line string
	err  error
}

// startVPPShutdownSyslog starts one bounded accept/read worker on a short,
// task-owned Unix stream socket, the protocol newSyslogHandler actually uses.
// Its cleanup closes both possible blocking operations and joins the worker
// before removing the socket directory. The listener stays live after its first
// receipt, so removing the fixture's stderr override cannot gain a dial fallback.
func startVPPShutdownSyslog(t *testing.T) (string, <-chan vppShutdownSyslogReceipt) {
	t.Helper()
	dir, socket, _, err := vppWorkPaths("vpp-log-")
	if err != nil {
		t.Fatal(err)
	}
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", socket)
	if err != nil {
		os.RemoveAll(dir) //nolint:errcheck // no worker owns the directory after a failed listen
		t.Fatal(err)
	}
	receipt := make(chan vppShutdownSyslogReceipt, 1)
	done := make(chan struct{})
	var mu sync.Mutex
	var accepted net.Conn
	closed := false
	t.Cleanup(func() {
		mu.Lock()
		closed = true
		listener.Close() //nolint:errcheck // closes the worker's pending Accept on every failure path
		if accepted != nil {
			accepted.Close() //nolint:errcheck // closes the worker's pending read before joining it
		}
		mu.Unlock()
		<-done
		os.RemoveAll(dir) //nolint:errcheck // the joined worker no longer owns the short socket
	})
	go func() {
		defer close(done)
		connection, err := listener.Accept()
		if err != nil {
			receipt <- vppShutdownSyslogReceipt{err: err}
			return
		}
		defer connection.Close() //nolint:errcheck // the sole reader owns this accepted socket
		mu.Lock()
		if closed {
			mu.Unlock()
			receipt <- vppShutdownSyslogReceipt{err: net.ErrClosed}
			return
		}
		accepted = connection
		mu.Unlock()
		if err := connection.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
			receipt <- vppShutdownSyslogReceipt{err: err}
			return
		}
		line, err := bufio.NewReader(io.LimitReader(connection, 16<<10)).ReadString('\n')
		receipt <- vppShutdownSyslogReceipt{line: line, err: err}
	}()
	return socket, receipt
}

// startVPPShutdownLoggerChild keeps inherited and owned logging in separate
// real processes so env and logger caches cannot manufacture a capture result.
// Its caller MUST let t.Cleanup retain stop ownership after every assertion.
func startVPPShutdownLoggerChild(t *testing.T, environ []string, subsystem, mode string) *fixtureProcess {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child, err := startFixtureProcess(t.Context(), environ, executable,
		"-test.run=^TestVPPShutdownChild$", "--", "vpp-shutdown-child", mode, subsystem)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { vppShutdownCleanup(child, 3*time.Second) })
	if !Poll(t.Context(), 200, 25*time.Millisecond, func() bool {
		return strings.Contains(child.output.String(), vppShutdownChildReady)
	}) {
		t.Fatalf("logger child did not publish readiness: %s", child.output.String())
	}
	return child
}

// startVPPShutdownChild starts this binary's isolated child role. Its caller
// MUST let t.Cleanup stop the owned process even when an assertion fails.
func startVPPShutdownChild(t *testing.T, mode string, detail ...string) *fixtureProcess {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"-test.run=^TestVPPShutdownChild$", "--", "vpp-shutdown-child", mode}
	args = append(args, detail...)
	child, err := startFixtureProcess(t.Context(), os.Environ(), executable, args...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { vppShutdownCleanup(child, 3*time.Second) })
	if !Poll(t.Context(), 200, 25*time.Millisecond, func() bool {
		return strings.Contains(child.output.String(), vppShutdownChildReady)
	}) {
		t.Fatalf("child did not publish its installed signal policy: %s", child.output.String())
	}
	return child
}

// assertVPPShutdownSignal reads the real cached Cmd.Wait result, not the
// shutdown helper's error wording, to distinguish TERM from forced KILL.
func assertVPPShutdownSignal(t *testing.T, child *fixtureProcess, wanted syscall.Signal) {
	t.Helper()
	err := waitFixtureProcess(t.Context(), child, 3*time.Second)
	exited, ok := errors.AsType[*exec.ExitError](err)
	if !ok {
		t.Fatalf("raw child wait = %v, want an actual signal exit", err)
	}
	status, ok := exited.Sys().(syscall.WaitStatus)
	if !ok {
		t.Fatalf("child wait status = %T", exited.Sys())
	}
	if !status.Signaled() {
		t.Fatalf("child exited without signal: %v", err)
	}
	if status.Signal() != wanted {
		t.Fatalf("child signal = %v, want %v", status.Signal(), wanted)
	}
}

// TestVPPShutdownChild is the actual subprocess role of the acceptance tests.
// The ordinary package run reaches no child branch and starts no worker.
func TestVPPShutdownChild(t *testing.T) {
	var args []string
	for i, arg := range os.Args {
		if arg == "vpp-shutdown-child" {
			args = os.Args[i+1:]
			break
		}
	}
	if len(args) == 0 {
		return
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	defer signal.Stop(signals)
	mode := args[0]
	if mode == "signal" {
		signal.Reset(syscall.SIGTERM)
		fmt.Println("Ze stopped.")
	}
	if mode == "ignore" {
		signal.Ignore(syscall.SIGTERM)
		fmt.Println("Ze stopped.")
	}
	fmt.Println(vppShutdownChildReady)
	if mode == "exited" {
		fmt.Println("Ze stopped.")
		return
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	select {
	case <-signals:
	case <-ctx.Done():
		t.Fatal("child outlived its owned stop")
	}
	switch mode {
	case "clean":
		fmt.Println("Ze stopped.")
	case "failure":
		fmt.Println("Ze stopped.")
		os.Exit(17) //nolint:gocritic // exitAfterDefer: this child process exits 17 on purpose; its deferred cancel and signal.Stop release nothing a dead process keeps
	case "silent":
	case "warning":
		if len(args) != 2 {
			t.Fatal("warning child needs its producer failure")
		}
		fmt.Println(args[1])
		fmt.Println("Ze stopped.")
	case "logger", "logger-syslog":
		if len(args) != 2 {
			t.Fatal("logger child needs its subsystem")
		}
		if mode == "logger-syslog" {
			if err := env.Set("ze.log.backend", "syslog"); err != nil {
				t.Fatal(err)
			}
		}
		switch args[1] {
		case "interface":
			slogutil.Logger("interface").Warn("VPP shutdown interface cleanup control",
				"code", diagnostic.CodeDoctorIfaceOwnedDeviceCleanup)
		case "plugin":
			slogutil.Logger("plugin").Warn("VPP shutdown plugin cleanup control",
				"code", diagnostic.CodeDoctorPluginStopCleanupLate)
		case "vpp":
			slogutil.Logger("vpp").Error("vpp manager stopped with an error")
		default:
			t.Fatalf("unknown logging subsystem %q", args[1])
		}
		fmt.Println("Ze stopped.")
	default:
		t.Fatalf("unknown shutdown child role %q", mode)
	}
}

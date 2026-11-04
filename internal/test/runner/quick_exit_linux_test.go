//go:build linux

// Design: docs/architecture/testing/ci-format.md -- unreadable quick-ze status.

package runner

import (
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestQuickZeUnreadableExit makes a real escaped descendant hold the output
// pipe. The existing observation seam reduces only WaitDelay, not ownership or
// kernel observation. The test owns the escaped fixture's separate control socket.
func TestQuickZeUnreadableExit(t *testing.T) {
	var listen net.ListenConfig
	listener, err := listen.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Error(err)
		}
	})
	tcpListener, ok := listener.(*net.TCPListener)
	if !ok {
		t.Fatalf("TCP listener has unexpected type %T", listener)
	}
	if err := tcpListener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	r, rec := helperExitRecord(t, "cmd=foreground:seq=1:exec=ze version:exit=0\ncmd=foreground:seq=2:exec=sh -c 'echo must-not-run'\n")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r.zePath = self
	// The test executable accepts Go test flags, so select the fixture while
	// retaining the ze verb that selects the real quick-exit arm.
	rec.RunCommands[0].Exec = "ze -test.run=^TestQuickZeHeldPipeProcess$ version"
	rec.EnvVars = append(rec.EnvVars, "QUICK_ZE_PIPE_ROLE=parent", "QUICK_ZE_PIPE_ADDRESS="+listener.Addr().String())
	var owned *ownedProcess
	var control net.Conn
	r.observeProcessExit = func(p *ownedProcess) error {
		if owned != nil {
			return p.beforeReap()
		}
		owned = p
		p.cmd.WaitDelay = 100 * time.Millisecond
		var acceptErr error
		control, acceptErr = listener.Accept()
		if acceptErr != nil {
			return acceptErr
		}
		return p.beforeReap()
	}
	t.Cleanup(func() {
		if control != nil {
			if err := control.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	if passed := r.runTest(t.Context(), rec, &RunOptions{}); passed {
		t.Fatal("exit=0 accepted unreadable native Wait status")
	}
	if owned == nil || !errors.Is(owned.Wait(), exec.ErrWaitDelay) {
		t.Fatalf("native wait did not produce ErrWaitDelay: %v", rec.Error)
	}
	if rec.FailureType != failTypeExitUnreadable || !errors.Is(rec.Error, exec.ErrWaitDelay) {
		t.Fatalf("failure=%q, error=%v", rec.FailureType, rec.Error)
	}
	for _, want := range []string{"cmd seq=1", "exit=0", "run the test again"} {
		if !strings.Contains(rec.Error.Error(), want) {
			t.Errorf("error %q lacks %q", rec.Error, want)
		}
	}
	if !strings.Contains(rec.ClientOutput, "quick-before-wait") || strings.Contains(rec.ClientOutput, "must-not-run") {
		t.Fatalf("quick output or command boundary lost: %q", rec.ClientOutput)
	}
	if err := control.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := control.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	var ack [1]byte
	if _, err := io.ReadFull(control, ack[:]); err != nil {
		t.Fatalf("escaped fixture did not close its inherited output: %v", err)
	}
}

// TestQuickZeHeldPipeProcess is a native fixture: the parent exits zero only
// after its escaped descendant owns a control socket and both output streams.
func TestQuickZeHeldPipeProcess(t *testing.T) {
	switch os.Getenv("QUICK_ZE_PIPE_ROLE") {
	case "parent":
		ready, notify, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestQuickZeHeldPipeProcess$")
		child.Env = append(os.Environ(), "QUICK_ZE_PIPE_ROLE=descendant")
		child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		child.ExtraFiles = []*os.File{notify}
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		if err := notify.Close(); err != nil {
			t.Fatal(err)
		}
		var one [1]byte
		if _, err := io.ReadFull(ready, one[:]); err != nil {
			t.Fatal(err)
		}
		if err := ready.Close(); err != nil {
			t.Fatal(err)
		}
		os.Exit(0)
	case "descendant":
		dialer := net.Dialer{Timeout: 5 * time.Second}
		control, err := dialer.DialContext(t.Context(), "tcp", os.Getenv("QUICK_ZE_PIPE_ADDRESS"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := control.Close(); err != nil {
				t.Error(err)
			}
		})
		if err := control.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stdout.WriteString("quick-before-wait\n"); err != nil {
			t.Fatal(err)
		}
		notify := os.NewFile(3, "ready")
		if _, err := notify.Write([]byte{1}); err != nil {
			t.Fatal(err)
		}
		if err := notify.Close(); err != nil {
			t.Fatal(err)
		}
		var one [1]byte
		_, readErr := control.Read(one[:])
		if err := os.Stdout.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.Stderr.Close(); err != nil {
			t.Fatal(err)
		}
		if readErr == nil {
			if _, err := control.Write([]byte{1}); err != nil {
				t.Fatal(err)
			}
		}
		if err := control.Close(); err != nil {
			t.Fatal(err)
		}
		os.Exit(0)
	}
}

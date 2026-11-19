//go:build integration && linux

// Design: docs/research/vpp-deployment-reference.md -- atomic child/socket correlation
package vpp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// The child owns one listener and one input lifetime; the parent owns its kill,
// wait and atomic pidfd. It requires no VPP or other binary.
func TestVPPManagedPeerChild(t *testing.T) {
	if len(os.Args) < 3 {
		return
	}
	if os.Args[len(os.Args)-2] != "vpp-managed-peer-child" {
		return
	}
	socket := os.Args[len(os.Args)-1]
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := fmt.Fprintf(os.Stdout, "READY %d\n", os.Getpid()); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		t.Fatal(err)
	}
}

type managedPeerChild struct {
	cmd         *exec.Cmd
	witness     *processWitness
	input       io.WriteCloser
	reportedPID int
	once        sync.Once
	failure     error
}

func (c *managedPeerChild) stop() error {
	c.once.Do(func() {
		inputErr := closeManagedFixturePipe(c.input)
		killErr := c.cmd.Process.Kill()
		if errors.Is(killErr, os.ErrProcessDone) {
			killErr = nil
		}
		waitErr := c.cmd.Wait()
		if _, found := errors.AsType[*exec.ExitError](waitErr); found {
			waitErr = nil
		} // parent deliberately kills its helper
		c.failure = errors.Join(inputErr, killErr, waitErr, c.witness.release())
	})
	return c.failure
}

// exec.Cmd closes its parent pipes on failed Start or Wait. Explicit cleanup
// also covers refusal before Start and tolerates only that confirmed close.
func closeManagedFixturePipe(pipe io.Closer) error {
	err := pipe.Close()
	if errors.Is(err, os.ErrClosed) {
		return nil
	}
	return err
}

func startManagedPeerFixture(t *testing.T, socket string, newNamespace bool) *managedPeerChild {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)
	command := exec.CommandContext(ctx, executable, "-test.run=^TestVPPManagedPeerChild$", "--", "vpp-managed-peer-child", socket)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if newNamespace {
		command.SysProcAttr.Cloneflags = unix.CLONE_NEWPID
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(errors.Join(err, closeManagedFixturePipe(output)))
	}
	witness, err := startManagedProcess(command)
	if err != nil {
		cleanup := errors.Join(closeManagedFixturePipe(input), closeManagedFixturePipe(output))
		if cleanup != nil {
			t.Error(cleanup)
		}
		if newNamespace {
			if errors.Is(err, unix.EPERM) {
				t.Skip("the owned PID namespace requires CAP_SYS_ADMIN")
			}
		}
		t.Fatal(err)
	}
	child := &managedPeerChild{cmd: command, witness: witness, input: input}
	t.Cleanup(func() {
		if err := child.stop(); err != nil {
			t.Error(err)
		}
	})
	line, err := bufio.NewReader(output).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(line, "READY ") {
		t.Fatalf("unexpected child readiness %q", line)
	}
	child.reportedPID, err = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "READY ")))
	if err != nil {
		t.Fatal(err)
	}
	group, err := unix.Getpgid(command.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if group != command.Process.Pid {
		t.Fatal("atomic spawn discarded the requested process group")
	}
	return child
}

func captureManagedFixtureSocket(t *testing.T, socket string) *processWitness {
	t.Helper()
	connection, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	peer, failure := captureProcessWitness(connection)
	failure = errors.Join(failure, connection.Close())
	if failure != nil {
		if peer != nil {
			failure = errors.Join(failure, peer.release())
		}
		t.Fatal(failure)
	}
	t.Cleanup(func() {
		if err := peer.release(); err != nil {
			t.Error(err)
		}
	})
	return peer
}

// Compare descriptors created by clone and by the actual Unix socket. A second
// live child cannot inherit the first child's ownership even before either PID
// has exited or can be reused.
func TestVPPManagedActualChildMatchesOnlyItsSocket(t *testing.T) {
	t.Chdir(t.TempDir())
	child := startManagedPeerFixture(t, "first.sock", false)
	foreign := startManagedPeerFixture(t, "other.sock", false)
	peer := captureManagedFixtureSocket(t, "first.sock")
	otherPeer := captureManagedFixtureSocket(t, "other.sock")
	if peer.identity != child.witness.identity {
		t.Fatal("clone and socket did not retain the same physical child")
	}
	if otherPeer.identity != foreign.witness.identity {
		t.Fatal("foreign child socket capture lost its identity")
	}
	connector := NewConnector("unused")
	connector.conn, connector.connected, connector.generation = &managedPeerTestConnection{peer: otherPeer, test: t}, true, 7
	if _, err := connector.bindManagedProcess(t.Context(), child.witness); err == nil {
		t.Fatal("another live process acquired the managed child's socket receipt")
	}
	connector.conn = &managedPeerTestConnection{peer: peer, test: t}
	t.Cleanup(connector.Close)
	if _, err := connector.bindManagedProcess(t.Context(), child.witness); err != nil {
		t.Fatal(err)
	}
}

// The child reports PID1 in its own namespace while cmd.Process names it in
// the parent namespace. Stable pidfs identity, rather than reported integers,
// accepts this valid server and refuses an unrelated process.
func TestVPPManagedActualChildAcrossPIDNamespace(t *testing.T) {
	t.Chdir(t.TempDir())
	child := startManagedPeerFixture(t, "namespaced.sock", true)
	if child.reportedPID != 1 {
		t.Fatalf("child namespace PID=%d", child.reportedPID)
	}
	if child.cmd.Process.Pid == child.reportedPID {
		t.Fatal("test did not establish distinct namespace PID values")
	}
	peer := captureManagedFixtureSocket(t, "namespaced.sock")
	connector := NewConnector("unused")
	connector.conn, connector.connected, connector.generation = &managedPeerTestConnection{peer: peer, test: t}, true, 7
	t.Cleanup(connector.Close)
	if _, err := connector.bindManagedProcess(t.Context(), child.witness); err != nil {
		t.Fatal(err)
	}
}

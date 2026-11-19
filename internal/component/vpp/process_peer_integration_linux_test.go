//go:build integration && linux

// Design: docs/research/vpp-deployment-reference.md -- actual Linux process witnesses
package vpp

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// The helper creates its own socket in the process whose identity is captured.
// A borrowed listener inherited from a parent would prove the wrong process.
func TestVPPPeerProcessHelper(t *testing.T) {
	arguments := flag.Args()
	if len(arguments) != 2 {
		return
	}
	if arguments[0] == "namespace-client" {
		peerNamespaceClient(t, arguments[1])
		return
	}
	if arguments[0] != "listener" {
		return
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: arguments[1], Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := fmt.Fprintln(os.Stdout, "ready"); err != nil {
		t.Fatal(err)
	}
	// The parent kills and joins this process. Eight accepted sockets bound
	// fixture state; each is independently owned until that process exits.
	for range 8 {
		connection, err := listener.AcceptUnix()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := connection.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		t.Fatal(err)
	}
}

func startPeerProcess(t *testing.T, socket string) *exec.Cmd {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)
	command := exec.CommandContext(ctx, executable, "-test.run=^TestVPPPeerProcessHelper$", "--", "listener", socket)
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if command.ProcessState == nil {
			if err := command.Process.Kill(); err != nil {
				t.Error(err)
			}
			if err := command.Wait(); err == nil {
				t.Error("killed helper exited successfully")
			}
		}
	})
	ready := bufio.NewScanner(output)
	if !ready.Scan() {
		t.Fatalf("peer helper did not start: %v", ready.Err())
	}
	if ready.Text() != "ready" {
		t.Fatalf("peer helper startup=%q", ready.Text())
	}
	return command
}

func capturePeerSocket(t *testing.T, path string) *processWitness {
	t.Helper()
	connection, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	witness, failure := captureProcessWitness(connection)
	failure = errors.Join(failure, connection.Close())
	if failure != nil {
		t.Fatal(failure)
	}
	t.Cleanup(func() {
		if err := witness.release(); err != nil {
			t.Error(err)
		}
	})
	return witness
}

// Use separate processes, real Unix sockets and real pidfds. Socket closure
// alone produces no proof; killing and reaping the original peer does, while
// the independently connected replacement remains live.
func TestVPPActualPeerIdentityAndExit(t *testing.T) {
	directory := t.TempDir()
	firstPath, secondPath := filepath.Join(directory, "first"), filepath.Join(directory, "second")
	first := startPeerProcess(t, firstPath)
	startPeerProcess(t, secondPath)
	firstAPI := capturePeerSocket(t, firstPath)
	firstStats := capturePeerSocket(t, firstPath)
	secondStats := capturePeerSocket(t, secondPath)
	if firstAPI.identity != firstStats.identity {
		t.Fatal("same actual peer has different pidfs identity")
	}
	if firstAPI.identity == secondStats.identity {
		t.Fatal("different actual peers share pidfs identity")
	}
	if err := firstAPI.bind(51, 7, make(chan struct{}, 1)); err != nil {
		t.Fatal(err)
	}
	lease, err := firstAPI.retain()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lease.Close(); err != nil {
			t.Error(err)
		}
	}()
	if proof, err := lease.Exit(); err != nil || proof != nil {
		t.Fatalf("socket closure proved death: %v,%v", proof, err)
	}
	if err := first.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := first.Wait(); err == nil {
		t.Fatal("killed helper exited successfully")
	}
	proof, err := lease.Exit()
	if err != nil {
		t.Fatal(err)
	}
	if proof == nil {
		t.Fatal("reaped actual peer has no kernel exit proof")
	}
	if !proof.Covers(51, 7) {
		t.Fatal("actual death lost its original generation")
	}
	if proof.Covers(51, 8) {
		t.Fatal("actual death covered a replacement generation")
	}
	if err := secondStats.alive(); err != nil {
		t.Fatalf("old death retired the live replacement: %v", err)
	}
}

// The client is PID1 in a child namespace, and the server lives in its parent
// namespace. SO_PEERCRED returns PID0 there; SO_PEERPIDFD still retains and
// compares the actual server, proving that no integer PID fallback is hidden.
func TestVPPActualPeerAcrossPIDNamespaces(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("creating a PID namespace needs root in the Ze integration guest")
	}
	socket := filepath.Join(t.TempDir(), "ancestor")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Error(err)
		}
	}()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^TestVPPPeerProcessHelper$", "--", "namespace-client", socket)
	command.SysProcAttr = &syscall.SysProcAttr{Cloneflags: unix.CLONE_NEWPID}
	output, err := command.CombinedOutput()
	if errors.Is(err, unix.EPERM) {
		t.Skip("this Ze guest lacks permission to create a PID namespace")
	}
	if err != nil {
		t.Fatalf("cross-namespace witness: %v: %s", err, output)
	}
}

func peerNamespaceClient(t *testing.T, socket string) {
	t.Helper()
	if os.Getpid() != 1 {
		t.Fatal("client did not enter its own PID namespace")
	}
	connection, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := connection.Close(); err != nil {
			t.Error(err)
		}
	}()
	raw, err := connection.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var credentials *unix.Ucred
	var credentialErr error
	if err := raw.Control(func(fd uintptr) {
		credentials, credentialErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil {
		t.Fatal(err)
	}
	if credentialErr != nil {
		t.Fatal(credentialErr)
	}
	if credentials.Pid != 0 {
		t.Fatalf("ancestor unexpectedly has namespace-visible PID%d", credentials.Pid)
	}
	first, err := captureProcessWitness(connection)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := first.release(); err != nil {
			t.Error(err)
		}
	}()
	second := capturePeerSocket(t, socket)
	if first.identity != second.identity {
		t.Fatal("same ancestor process did not retain its kernel identity")
	}
	if err := first.alive(); err != nil {
		t.Fatal(err)
	}
}

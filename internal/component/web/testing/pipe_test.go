// Design: docs/architecture/testing/runner-architecture.md -- bounded browser output drains.
package webtesting

import (
	"context"
	"errors"
	"flag"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

var browserPipeSocket = flag.String("browser-pipe-socket", "", "owned command pipe fixture")

// TestBrowserPipeFixture transfers its stdout descriptor to the test owner,
// then exits. No detached child is needed to keep the command's pipe open.
func TestBrowserPipeFixture(t *testing.T) {
	if *browserPipeSocket == "" {
		return
	}
	conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: *browserPipeSocket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer closeBrowserPipeFixture(t, conn)
	if _, _, err := conn.WriteMsgUnix([]byte("owned"), syscall.UnixRights(int(os.Stdout.Fd())), nil); err != nil {
		t.Fatal(err)
	}
}

// TestBrowserOutputBoundsInheritedPipe keeps the actual command stdout open
// after exit using a descriptor owned by this test. Wait must report its pipe
// bound while that descriptor remains open, not wait for the fixture to release it.
func TestBrowserOutputBoundsInheritedPipe(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "pipe.sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer closeBrowserPipeFixture(t, listener)
	if err := listener.SetDeadline(time.Now().Add(8 * time.Second)); err != nil {
		t.Fatal(err)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nexport GORACE=atexit_sleep_ms=0\nexec " + strconv.Quote(binary) +
		" -test.run=^TestBrowserPipeFixture$ -browser-pipe-socket=" + strconv.Quote(socket) + "\n"
	if err := os.WriteFile(filepath.Join(dir, agentBrowserBin), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	browser := newBrowser("")
	browser.ctx = ctx
	result := make(chan error, 1)
	go func() {
		_, err := browser.runAgentOutput("eval", "true")
		result <- err
	}()
	conn, err := listener.AcceptUnix()
	if err != nil {
		t.Fatal(err)
	}
	defer closeBrowserPipeFixture(t, conn)
	if err := conn.SetReadDeadline(time.Now().Add(8 * time.Second)); err != nil {
		t.Fatal(err)
	}
	data, rights := make([]byte, 16), make([]byte, syscall.CmsgSpace(4))
	_, n, _, _, err := conn.ReadMsgUnix(data, rights)
	if err != nil {
		t.Fatal(err)
	}
	messages, err := syscall.ParseSocketControlMessage(rights[:n])
	if err != nil || len(messages) != 1 {
		t.Fatalf("stdout transfer: %v messages=%d", err, len(messages))
	}
	fds, err := syscall.ParseUnixRights(&messages[0])
	if err != nil || len(fds) != 1 {
		t.Fatalf("stdout descriptor: %v descriptors=%v", err, fds)
	}
	pipe := os.NewFile(uintptr(fds[0]), "owned-command-stdout")
	defer closeBrowserPipeFixture(t, pipe)
	select {
	case err := <-result:
		if !errors.Is(err, exec.ErrWaitDelay) {
			t.Fatalf("inherited pipe did not report the drain bound: %v", err)
		}
	case <-ctx.Done():
		// Release the descriptor and join the command even on the source-red path.
		if err := pipe.Close(); err != nil {
			t.Error(err)
		}
		<-result
		t.Fatal("browser output waited for the retained pipe instead of a cleanup bound")
	}
}

func closeBrowserPipeFixture(t *testing.T, owned io.Closer) {
	t.Helper()
	if err := owned.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		t.Error(err)
	}
}

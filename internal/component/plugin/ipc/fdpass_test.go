package ipc

import (
	"bytes"
	"context"
	"net"
	"os"
	"syscall"
	"testing"
)

// testUnixConnPair creates a connected pair of *net.UnixConn for fd passing tests.
func testUnixConnPair(t *testing.T) (sender, receiver net.Conn) {
	t.Helper()

	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	f0 := os.NewFile(uintptr(fds[0]), "sender")
	f1 := os.NewFile(uintptr(fds[1]), "receiver")

	sender, err = net.FileConn(f0)
	if closeErr := f0.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		f1.Close() //nolint:errcheck,gosec // cleanup on error
		t.Fatalf("sender conn: %v", err)
	}

	receiver, err = net.FileConn(f1)
	if closeErr := f1.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		sender.Close() //nolint:errcheck,gosec // cleanup on error
		t.Fatalf("receiver conn: %v", err)
	}

	t.Cleanup(func() {
		sender.Close()   //nolint:errcheck,gosec // test cleanup
		receiver.Close() //nolint:errcheck,gosec // test cleanup
	})

	return sender, receiver
}

// VALIDATES: SendFD and ReceiveFD pass a file descriptor over a Unix socketpair.
// PREVENTS: Regression in SCM_RIGHTS fd passing across platforms (Linux + macOS).
func TestSendReceiveFD(t *testing.T) {
	t.Parallel()

	sender, receiver := testUnixConnPair(t)

	// Create a pipe to pass as the fd payload.
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer pr.Close() //nolint:errcheck // test cleanup
	defer pw.Close() //nolint:errcheck // test cleanup

	// Send the read end of the pipe.
	if err := SendFD(sender, pr); err != nil {
		t.Fatalf("SendFD: %v", err)
	}

	// Receive the fd.
	received, err := ReceiveFD(receiver)
	if err != nil {
		t.Fatalf("ReceiveFD: %v", err)
	}
	defer received.Close() //nolint:errcheck // test cleanup

	// Verify the received fd is usable: write to pipe, read from received fd.
	msg := []byte("hello fd passing")
	if _, err := pw.Write(msg); err != nil {
		t.Fatalf("write to pipe: %v", err)
	}

	buf := make([]byte, len(msg))
	n, err := received.Read(buf)
	if err != nil {
		t.Fatalf("read from received fd: %v", err)
	}
	if !bytes.Equal(buf[:n], msg) {
		t.Fatalf("got %q, want %q", buf[:n], msg)
	}
}

// VALIDATES: SendFD can pass a TCP listen socket fd; receiver can Accept() on it.
// PREVENTS: Listen socket handoff failing due to fd type conversion issues.
func TestSendReceiveFDListenSocket(t *testing.T) {
	t.Parallel()

	sender, receiver := testUnixConnPair(t)

	// Create a TCP listener on a random port.
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()

	// Get the listener's fd.
	tcpLn, ok := ln.(*net.TCPListener)
	if !ok {
		ln.Close() //nolint:errcheck,gosec // cleanup before fatal
		t.Fatal("not a TCPListener")
	}
	lnFile, err := tcpLn.File()
	if err != nil {
		ln.Close() //nolint:errcheck,gosec // cleanup before fatal
		t.Fatalf("listener File(): %v", err)
	}
	// Close the original listener — the fd in lnFile is a dup.
	ln.Close() //nolint:errcheck,gosec // original listener no longer needed

	// Send the listen socket fd.
	if err := SendFD(sender, lnFile); err != nil {
		lnFile.Close() //nolint:errcheck,gosec // cleanup before fatal
		t.Fatalf("SendFD: %v", err)
	}
	lnFile.Close() //nolint:errcheck,gosec // sender closes their copy after send

	// Receive the fd and convert to a listener.
	received, err := ReceiveFD(receiver)
	if err != nil {
		t.Fatalf("ReceiveFD: %v", err)
	}
	newLn, err := net.FileListener(received)
	received.Close() //nolint:errcheck,gosec // fd ownership transferred to newLn
	if err != nil {
		t.Fatalf("FileListener: %v", err)
	}
	defer newLn.Close() //nolint:errcheck // test cleanup

	// Verify the received listener works: connect to it and accept.
	done := make(chan error, 1)
	go func() {
		conn, acceptErr := newLn.Accept()
		if acceptErr != nil {
			done <- acceptErr
			return
		}
		conn.Close() //nolint:errcheck,gosec // test cleanup
		done <- nil
	}()

	var dialer net.Dialer
	client, err := dialer.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	client.Close() //nolint:errcheck,gosec // test cleanup

	if err := <-done; err != nil {
		t.Fatalf("accept: %v", err)
	}
}

// VALIDATES: SendFD returns error for net.Pipe connections (internal plugins).
// PREVENTS: Confusing error when fd passing attempted on in-memory connections.
func TestSendFDInternalPipeError(t *testing.T) {
	t.Parallel()

	a, b := net.Pipe()
	defer a.Close() //nolint:errcheck // test cleanup
	defer b.Close() //nolint:errcheck // test cleanup

	// Create a dummy file to send.
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer pr.Close() //nolint:errcheck // test cleanup
	defer pw.Close() //nolint:errcheck // test cleanup

	err = SendFD(a, pr)
	if err == nil {
		t.Fatal("expected error for net.Pipe, got nil")
	}
}

// VALIDATES: Multiple fds can be sent sequentially over the same connection.
// PREVENTS: State corruption after first fd send/receive cycle.
func TestSendReceiveFDMultiple(t *testing.T) {
	t.Parallel()

	sender, receiver := testUnixConnPair(t)

	// Send 3 pipe read-ends sequentially.
	for i := range 3 {
		pr, pw, err := os.Pipe()
		if err != nil {
			t.Fatalf("pipe %d: %v", i, err)
		}

		if err := SendFD(sender, pr); err != nil {
			pr.Close() //nolint:errcheck,gosec // cleanup before fatal
			pw.Close() //nolint:errcheck,gosec // cleanup before fatal
			t.Fatalf("SendFD %d: %v", i, err)
		}
		pr.Close() //nolint:errcheck,gosec // sender closes after send

		received, err := ReceiveFD(receiver)
		if err != nil {
			pw.Close() //nolint:errcheck,gosec // cleanup before fatal
			t.Fatalf("ReceiveFD %d: %v", i, err)
		}

		// Verify each received fd is independently usable.
		msg := []byte{byte('A' + i)}
		if _, writeErr := pw.Write(msg); writeErr != nil {
			received.Close() //nolint:errcheck,gosec // cleanup before fatal
			t.Fatalf("write %d: %v", i, writeErr)
		}
		pw.Close() //nolint:errcheck,gosec // done writing

		buf := make([]byte, 1)
		n, _ := received.Read(buf)
		received.Close() //nolint:errcheck,gosec // done reading

		if n != 1 || buf[0] != msg[0] {
			t.Fatalf("fd %d: got %q, want %q", i, buf[:n], msg)
		}
	}
}

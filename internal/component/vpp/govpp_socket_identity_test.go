//go:build linux || darwin

// Design: docs/contributing/patching-the-standard-library.md -- original GoVPP connection hooks.
// Related: govpp_socket_transport_test.go -- the actual native framing and create handshake.
package vpp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"go.fd.io/govpp/adapter/socketclient"
)

// The peer receives a hook-only probe before the real create frame on the same
// accepted socket. This proves both original-connection access and ordering;
// a callback on an unused socket or after the handshake cannot satisfy it.
// MUTATION: moving the callback after open admits SockclntCreate before this probe.
func TestGoVPPSocketConnectionHookPrecedesNativeHandshake(t *testing.T) {
	const probe = "accepted-original-hook"
	var calls atomic.Int32
	client, done, err := connectSocketIdentityPeer(t, func(conn *net.UnixConn) error {
		calls.Add(1)
		return writeSocketIdentityProbe(conn, probe)
	}, func(conn *net.UnixConn) error {
		if err := readSocketIdentityProbe(conn, probe); err != nil {
			return err
		}
		if err := socketTransportHandshake(conn); err != nil {
			return err
		}
		frame, err := readSocketTransportFrame(conn)
		if err != nil {
			return err
		}
		if len(frame) != 14 {
			return fmt.Errorf("native client delete after accepted hook=%x", frame)
		}
		if binary.BigEndian.Uint16(frame[:2]) != 42 {
			return fmt.Errorf("native client delete after accepted hook=%x", frame)
		}
		if binary.BigEndian.Uint32(frame[6:10]) != 124 {
			return fmt.Errorf("native client delete after accepted hook=%x", frame)
		}
		if binary.BigEndian.Uint32(frame[10:14]) != 123 {
			return fmt.Errorf("native client delete after accepted hook=%x", frame)
		}
		return nil
	})
	if err != nil {
		t.Fatal("original socket hook did not precede the native create handshake:", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("hook ran %d times, want once for the actual dial", calls.Load())
	}
	if err := client.Disconnect(); err != nil {
		t.Fatal(err)
	}
	requireSocketIdentityPeer(t, done)
}

// The rejecting hook first proves it borrowed this actual accepted socket.
// Native EOF must follow that probe with no API frame; a closed unrelated
// socket, ignored cause or handshake followed by cleanup cannot satisfy it.
// MUTATION: ignoring the hook error sends an API frame and loses the original cause.
// MUTATION: omitting the local conn.Close prevents the required native EOF.
func TestGoVPPSocketConnectionHookRejectionClosesOriginalSocket(t *testing.T) {
	const probe = "rejected-original-hook"
	cause := errors.New("original socket identity was refused")
	var calls atomic.Int32
	client, done, err := connectSocketIdentityPeer(t, func(conn *net.UnixConn) error {
		calls.Add(1)
		if err := writeSocketIdentityProbe(conn, probe); err != nil {
			return err
		}
		return cause
	}, func(conn *net.UnixConn) error {
		if err := readSocketIdentityProbe(conn, probe); err != nil {
			return err
		}
		var first [1]byte
		n, err := conn.Read(first[:])
		if n != 0 {
			return fmt.Errorf("rejected original socket sent API bytes: %x", first[:n])
		}
		if !errors.Is(err, io.EOF) {
			return fmt.Errorf("rejected original socket did not close with EOF: %w", err)
		}
		return nil
	})
	if !errors.Is(err, cause) {
		t.Fatalf("Connect lost the original rejecting-hook cause: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("rejecting hook ran %d times, want once for the actual dial", calls.Load())
	}
	requireSocketIdentityPeer(t, done)
	if err := client.SendMsg(7, make([]byte, 10)); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("rejected original socket admitted a usable sender: %v", err)
	}
}

// This is synchronous use of the borrowed socket during the callback only.
// No test stores, closes or uses that socket after the callback returns.
func writeSocketIdentityProbe(conn *net.UnixConn, probe string) error {
	if conn == nil {
		return errors.New("connection hook received no native Unix socket")
	}
	if conn.RemoteAddr().Network() != "unix" {
		return fmt.Errorf("connection hook did not receive the actual connected peer: %v", conn.RemoteAddr())
	}
	if conn.RemoteAddr().String() != "api.sock" {
		return fmt.Errorf("connection hook did not receive the actual connected peer: %v", conn.RemoteAddr())
	}
	n, err := conn.Write([]byte(probe))
	if err != nil {
		return err
	}
	if n != len(probe) {
		return io.ErrShortWrite
	}
	return nil
}

func readSocketIdentityProbe(conn *net.UnixConn, probe string) error {
	actual := make([]byte, len(probe))
	if _, err := io.ReadFull(conn, actual); err != nil {
		return fmt.Errorf("read original connection hook probe: %w", err)
	}
	if !bytes.Equal(actual, []byte(probe)) {
		return fmt.Errorf("native API bytes preceded the original connection hook: %x", actual)
	}
	return nil
}

// This fixture installs the public hook before the actual public Connect.
// It reuses only the existing native frame/create helpers, leaving the three
// transport tests and their helper byte-exact. Cleanup closes the test peer
// before joining it or a successful native adapter, including assertion failure.
func connectSocketIdentityPeer(t *testing.T, hook func(*net.UnixConn) error,
	serve func(*net.UnixConn) error) (*socketTransportClient, <-chan error, error) {
	t.Helper()
	t.Chdir(t.TempDir())
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: "api.sock", Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	client := &socketTransportClient{Client: socketclient.NewVppClient("api.sock")}
	client.SetConnectionHook(hook)
	connected := false
	accepted := make(chan *net.UnixConn, 1)
	done := make(chan error, 1)
	go func() {
		defer close(accepted)
		defer close(done)
		conn, err := listener.AcceptUnix()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close() //nolint:errcheck // Fixture cleanup closes its test peer and joins this worker.
		accepted <- conn
		if err := conn.SetDeadline(time.Now().Add(socketTransportBound)); err != nil {
			done <- err
			return
		}
		done <- serve(conn)
	}()
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				t.Error(err)
			}
		}
		select {
		case peer := <-accepted:
			if peer != nil {
				closeSocketTransportPeer(t, peer)
			}
		case <-time.After(socketTransportBound):
			t.Error("original socket accept worker did not join")
		}
		if connected {
			if err := client.Disconnect(); err != nil {
				if !t.Failed() {
					t.Error(err)
				}
			}
		}
		select {
		case err := <-done:
			if err != nil {
				if !t.Failed() {
					t.Error(err)
				}
			}
		case <-time.After(socketTransportBound):
			t.Error("original socket peer worker did not join")
		}
	})
	err = client.Connect()
	connected = err == nil
	return client, done, err
}

func requireSocketIdentityPeer(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(socketTransportBound):
		t.Fatal("original socket peer did not confirm its native outcome")
	}
}

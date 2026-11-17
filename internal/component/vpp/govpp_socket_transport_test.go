//go:build linux || darwin

// Design: docs/contributing/patching-the-standard-library.md -- bounded GoVPP socket writes.
// VALIDATES: the real GoVPP Unix adapter bounds blocked writes and joins them during disconnect.
package vpp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"go.fd.io/govpp/adapter/socketclient"
	"go.fd.io/govpp/binapi/memclnt"
	"go.fd.io/govpp/codec"
)

const socketTransportBound = 2 * time.Second
const socketTransportPayload = 16 << 20

// A completed create handshake and the next frame header prove SendMsg reached
// the actual Unix connection. The peer then withholds reads of that large body.
func TestGoVPPSocketWriteDeadline(t *testing.T) {
	blocked := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	client, peer, _ := connectSocketTransportPeer(t, 100*time.Millisecond, func(conn *net.UnixConn) error {
		length, err := readSocketTransportHeader(conn)
		if err != nil {
			return err
		}
		if length != socketTransportPayload {
			return fmt.Errorf("blocked frame length=%d", length)
		}
		close(blocked)
		<-release
		return nil
	})
	result := make(chan error, 1)
	go func() { defer close(result); result <- client.SendMsg(7, make([]byte, socketTransportPayload)) }()
	t.Cleanup(func() {
		closeSocketTransportPeer(t, peer)
		select {
		case <-result:
		case <-time.After(socketTransportBound):
			t.Error("native sender did not join")
		}
	})
	select {
	case <-blocked:
	case <-time.After(socketTransportBound):
		t.Fatal("sender did not reach the native socket")
	}
	select {
	case err := <-result:
		timeout, valid := errors.AsType[net.Error](err)
		if !valid {
			t.Fatalf("blocked native write error=%v", err)
		}
		if !timeout.Timeout() {
			t.Fatalf("blocked native write error=%v", err)
		}
	case <-time.After(socketTransportBound):
		t.Fatal("native write ignored its configured deadline")
	}
	if err := client.SendMsg(8, make([]byte, 10)); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("partially written socket was reused: %v", err)
	}
	if err := client.Disconnect(); err != nil {
		t.Fatal(err)
	}
}

// The ordinary write bound is deliberately longer than the shutdown guard.
// Shutdown must interrupt an active writer and refuse a concurrent sender.
func TestGoVPPSocketDisconnectBoundsBlockedWriters(t *testing.T) {
	blocked := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	client, peer, _ := connectSocketTransportPeer(t, 10*time.Second, func(conn *net.UnixConn) error {
		length, err := readSocketTransportHeader(conn)
		if err != nil {
			return err
		}
		if length != socketTransportPayload {
			return fmt.Errorf("blocked frame length=%d", length)
		}
		close(blocked)
		<-release
		return nil
	})
	client.SetDisconnectTimeout(50 * time.Millisecond)
	first := make(chan error, 1)
	second := make(chan error, 1)
	shutdown := make(chan error, 1)
	go func() { defer close(first); first <- client.SendMsg(9, make([]byte, socketTransportPayload)) }()
	t.Cleanup(func() {
		closeSocketTransportPeer(t, peer)
		for _, result := range []<-chan error{first, second, shutdown} {
			select {
			case <-result:
			case <-time.After(socketTransportBound):
				t.Error("native transport worker did not join")
			}
		}
	})
	select {
	case <-blocked:
	case <-time.After(socketTransportBound):
		close(second)
		close(shutdown)
		t.Fatal("sender did not reach the native socket")
	}
	started := make(chan struct{})
	go func() { defer close(second); close(started); second <- client.SendMsg(10, make([]byte, 10)) }()
	<-started
	go func() { defer close(shutdown); shutdown <- client.Disconnect() }()
	select {
	case err := <-shutdown:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(socketTransportBound):
		t.Fatal("disconnect waited behind a blocked native writer")
	}
	for name, result := range map[string]chan error{"active": first, "queued": second} {
		select {
		case err := <-result:
			if err == nil {
				t.Errorf("%s sender succeeded after transport retirement", name)
			}
		case <-time.After(socketTransportBound):
			t.Fatalf("%s sender did not join disconnect", name)
		}
	}
	if err := client.SendMsg(11, make([]byte, 10)); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("disconnected transport accepted another send: %v", err)
	}
}

// Bounding shutdown must preserve a healthy sender and its real client-delete
// request. A blanket close, or a gate that also refuses the delete, fails here.
func TestGoVPPSocketHealthyDisconnectKeepsDeleteHandshake(t *testing.T) {
	payload := []byte{0, 73, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3}
	client, _, done := connectSocketTransportPeer(t, time.Second, func(conn *net.UnixConn) error {
		frame, err := readSocketTransportFrame(conn)
		if err != nil {
			return err
		}
		want := []byte{0, 73, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3}
		binary.BigEndian.PutUint32(want[2:6], 123)
		binary.BigEndian.PutUint32(want[6:10], 77)
		if !bytes.Equal(frame, want) {
			return fmt.Errorf("healthy send bytes=%x, want %x", frame, want)
		}
		frame, err = readSocketTransportFrame(conn)
		if err != nil {
			return err
		}
		if len(frame) != 14 {
			return fmt.Errorf("client delete bytes=%x", frame)
		}
		if binary.BigEndian.Uint16(frame[:2]) != 42 {
			return fmt.Errorf("client delete bytes=%x", frame)
		}
		if binary.BigEndian.Uint32(frame[6:10]) != 124 {
			return fmt.Errorf("client delete bytes=%x", frame)
		}
		if binary.BigEndian.Uint32(frame[10:14]) != 123 {
			return fmt.Errorf("client delete bytes=%x", frame)
		}
		return nil
	})
	if err := client.SendMsg(77, payload); err != nil {
		t.Fatal(err)
	}
	if err := client.Disconnect(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(socketTransportBound):
		t.Fatal("peer did not receive the client-delete handshake")
	}
}

// socketTransportClient keeps the test's native adapter under one joined close
// owner, including assertion failures while a concurrent Disconnect is pending.
// It changes no transport operation: every call reaches the public adapter.
type socketTransportClient struct {
	*socketclient.Client
	closeOnce sync.Once
	closeErr  error
}

func (c *socketTransportClient) Disconnect() error {
	c.closeOnce.Do(func() { c.closeErr = c.Client.Disconnect() })
	return c.closeErr
}

// connectSocketTransportPeer exercises the public adapter and its real native
// framing. A relative path keeps Unix socket names below sun_path's limit;
// testing restores the temporary working directory after all owned workers join.
func connectSocketTransportPeer(t *testing.T, timeout time.Duration,
	serve func(*net.UnixConn) error) (*socketTransportClient, *net.UnixConn, <-chan error) {
	t.Helper()
	t.Chdir(t.TempDir())
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: "api.sock", Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	client := &socketTransportClient{Client: socketclient.NewVppClient("api.sock")}
	connected := false
	accepted := make(chan *net.UnixConn, 1)
	done := make(chan error, 1)
	go func() {
		defer close(done)
		conn, err := listener.AcceptUnix()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close() //nolint:errcheck // The owning test closes this connection and joins this worker.
		accepted <- conn
		if err := conn.SetReadBuffer(4096); err != nil {
			done <- err
			return
		}
		if err := socketTransportHandshake(conn); err != nil {
			done <- err
			return
		}
		done <- serve(conn)
	}()
	var peer *net.UnixConn
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				t.Error(err)
			}
		}
		if peer == nil {
			select {
			case peer = <-accepted:
			default:
			}
		}
		if peer != nil {
			closeSocketTransportPeer(t, peer)
		}
		if connected {
			if err := client.Disconnect(); err != nil {
				t.Error(err)
			}
		}
		select {
		case err := <-done:
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					t.Error(err)
				}
			}
		case <-time.After(socketTransportBound):
			t.Error("native socket peer did not join")
		}
	})
	if err := client.Connect(); err != nil {
		t.Fatal(err)
	}
	connected = true
	client.SetConnectTimeout(timeout)
	select {
	case peer = <-accepted:
	case <-time.After(socketTransportBound):
		t.Fatal("native socket was not accepted")
	}
	return client, peer, done
}

func closeSocketTransportPeer(t *testing.T, peer *net.UnixConn) {
	t.Helper()
	if err := peer.Close(); err != nil {
		if !errors.Is(err, net.ErrClosed) {
			t.Error(err)
		}
	}
}

func socketTransportHandshake(conn *net.UnixConn) error {
	request, err := readSocketTransportFrame(conn)
	if err != nil {
		return err
	}
	if len(request) < 6 {
		return fmt.Errorf("client create header=%x", request)
	}
	if binary.BigEndian.Uint16(request[:2]) != 15 {
		return fmt.Errorf("client create header=%x", request)
	}
	if request[5] != 123 {
		return fmt.Errorf("client create header=%x", request)
	}
	create := new(memclnt.SockclntCreate)
	if err := codec.DefaultCodec.DecodeMsg(request, create); err != nil {
		return err
	}
	if create.Name != socketclient.DefaultClientName {
		return fmt.Errorf("client name=%q", create.Name)
	}
	response := &memclnt.SockclntCreateReply{Index: 123,
		MessageTable: []memclnt.MessageTableEntry{{Index: 42, Name: "sockclnt_delete_8ac76db6"}}}
	message, err := codec.DefaultCodec.EncodeMsg(response, 16)
	if err != nil {
		return err
	}
	var header [16]byte
	binary.BigEndian.PutUint32(header[8:12], uint32(len(message)))
	frame := append(header[:], message...)
	n, err := conn.Write(frame)
	if err != nil {
		return err
	}
	if n != len(frame) {
		return io.ErrShortWrite
	}
	return nil
}

func readSocketTransportHeader(conn *net.UnixConn) (int, error) {
	var header [16]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return 0, err
	}
	return int(binary.BigEndian.Uint32(header[8:12])), nil
}

func readSocketTransportFrame(conn *net.UnixConn) ([]byte, error) {
	length, err := readSocketTransportHeader(conn)
	if err != nil {
		return nil, err
	}
	if length < 6 {
		return nil, fmt.Errorf("test peer received invalid frame length %d", length)
	}
	if length > 65536 {
		return nil, fmt.Errorf("test peer received invalid frame length %d", length)
	}
	message := make([]byte, length)
	if _, err := io.ReadFull(conn, message); err != nil {
		return nil, err
	}
	return message, nil
}

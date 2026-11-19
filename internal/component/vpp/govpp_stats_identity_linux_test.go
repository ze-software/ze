//go:build integration && linux

// Design: docs/contributing/patching-the-standard-library.md -- pinned GoVPP statistics sockets.
// Related: govpp_socket_identity_test.go -- the separate native API socket hook contract.
package vpp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"go.fd.io/govpp/adapter"
	"go.fd.io/govpp/adapter/statsclient"
)

const statsIdentityBound = 3 * time.Second

// A missing validator refuses pinned mode before socket lookup. The successful
// mapping question below prevents a producer that refuses every pinned client.
// MUTATION: dropping the nil-validator admission guard changes this exact refusal.
func TestGoVPPPinnedStatsRequireValidator(t *testing.T) {
	client := statsclient.NewStatsClient(filepath.Join(t.TempDir(), "absent.sock"),
		statsclient.WithPinnedConnection(nil),
		statsclient.SetSocketRetryTimeout(50*time.Millisecond))
	err := client.Connect()
	if err == nil {
		t.Fatal("pinned statistics accepted a missing socket validator")
	}
	if err.Error() != "pinned stats connection requires a socket validator" {
		t.Fatalf("missing validator reached socket lookup: %v", err)
	}
	if _, err := client.ListStats(); !errors.Is(err, adapter.ErrStatsDisconnected) {
		t.Fatalf("missing validator published statistics: %v", err)
	}
	if err := client.Disconnect(); err != nil {
		t.Fatal(err)
	}
}

// The callback probes the original Unixpacket socket synchronously and returns
// its original cause. The accepted peer requires EOF without a descriptor reply.
// MUTATION: ignoring the validator cause admits the descriptor-receive path.
func TestGoVPPPinnedStatsRejectOriginalSocket(t *testing.T) {
	const probe = "rejected-original-stats-hook"
	cause := errors.New("original statistics socket identity was refused")
	var calls atomic.Int32
	path, done, _ := startStatsIdentityPeer(t, func(conn *net.UnixConn) error {
		if err := readStatsIdentityProbe(conn, probe); err != nil {
			return err
		}
		return statsIdentityEOF(conn)
	})
	client := statsclient.NewStatsClient(path,
		statsclient.SetSocketRetryTimeout(time.Second),
		statsclient.WithPinnedConnection(func(conn *net.UnixConn) error {
			calls.Add(1)
			if err := writeStatsIdentityProbe(conn, probe); err != nil {
				return err
			}
			return cause
		}))
	if err := client.Connect(); !errors.Is(err, cause) {
		t.Fatalf("statistics Connect lost the rejecting-validator cause: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("rejecting statistics validator ran %d times, want one", calls.Load())
	}
	requireStatsIdentityPeer(t, done)
	if _, err := client.ListStats(); !errors.Is(err, adapter.ErrStatsDisconnected) {
		t.Fatalf("rejected statistics socket published a mapping: %v", err)
	}
	if err := client.Disconnect(); err != nil {
		t.Fatal(err)
	}
}

// The native peer withholds SCM_RIGHTS after receiving the original-hook probe.
// A descriptor-receive error must arrive while that peer is still open. The
// independent guard only fails the question, then closes and joins both workers.
// MUTATION: removing the descriptor-transfer deadline leaves Connect blocked.
func TestGoVPPPinnedStatsBoundsDescriptorReceipt(t *testing.T) {
	const probe = "withheld-original-stats-hook"
	observed := make(chan struct{})
	var calls atomic.Int32
	path, peerDone, stop := startStatsIdentityPeer(t, func(conn *net.UnixConn) error {
		if err := readStatsIdentityProbe(conn, probe); err != nil {
			return err
		}
		close(observed)
		return statsIdentityEOF(conn)
	})
	client := statsclient.NewStatsClient(path,
		statsclient.SetSocketRetryTimeout(50*time.Millisecond),
		statsclient.WithPinnedConnection(func(conn *net.UnixConn) error {
			calls.Add(1)
			return writeStatsIdentityProbe(conn, probe)
		}))
	connected := make(chan error, 1)
	joined := false
	t.Cleanup(func() {
		stop()
		if !joined {
			select {
			case <-connected:
			case <-time.After(statsIdentityBound):
				t.Error("statistics Connect worker did not join after peer closure")
			}
		}
		if err := client.Disconnect(); err != nil {
			t.Error(err)
		}
	})
	go func() { connected <- client.Connect() }()
	select {
	case <-observed:
	case <-time.After(time.Second):
		t.Fatal("native statistics peer did not receive the original-hook probe")
	}
	select {
	case err := <-connected:
		joined = true
		if err == nil {
			t.Fatal("statistics Connect accepted an absent segment descriptor")
		}
		if !strings.HasPrefix(err.Error(), "getting file descriptor over socket failed:") {
			t.Fatalf("statistics Connect failed outside descriptor receipt: %v", err)
		}
		if !strings.Contains(err.Error(), "i/o timeout") {
			t.Fatalf("statistics descriptor receipt returned a non-deadline error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("pinned statistics descriptor receipt exceeded its configured timeout")
	}
	if calls.Load() != 1 {
		t.Fatalf("withheld-descriptor validator ran %d times, want one", calls.Load())
	}
	requireStatsIdentityPeer(t, peerDone)
	if _, err := client.ListStats(); !errors.Is(err, adapter.ErrStatsDisconnected) {
		t.Fatalf("withheld descriptor published a statistics mapping: %v", err)
	}
}

// A real SCM_RIGHTS transfer publishes a V2 scalar directory only after the
// accepted peer observes the original-hook probe. DumpStats reads the mapped
// scalar through the public adapter; Disconnect must revoke that usable mapping.
// MUTATION: suppressing the validator sends no probe before segment receipt.
func TestGoVPPPinnedStatsMapsOriginalDescriptor(t *testing.T) {
	const probe = "accepted-original-stats-hook"
	file := statsIdentitySegment(t)
	var calls atomic.Int32
	path, done, _ := startStatsIdentityPeer(t, func(conn *net.UnixConn) error {
		if err := readStatsIdentityProbe(conn, probe); err != nil {
			return err
		}
		rights := syscall.UnixRights(int(file.Fd()))
		n, sent, err := conn.WriteMsgUnix([]byte{1}, rights, nil)
		if err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("statistics descriptor packet wrote %d bytes", n)
		}
		if sent != len(rights) {
			return fmt.Errorf("statistics SCM_RIGHTS wrote %d bytes, want %d", sent, len(rights))
		}
		return statsIdentityEOF(conn)
	})
	client := statsclient.NewStatsClient(path,
		statsclient.SetSocketRetryTimeout(time.Second),
		statsclient.WithPinnedConnection(func(conn *net.UnixConn) error {
			calls.Add(1)
			return writeStatsIdentityProbe(conn, probe)
		}))
	t.Cleanup(func() {
		if err := client.Disconnect(); err != nil {
			t.Error(err)
		}
	})
	if err := client.Connect(); err != nil {
		t.Fatal("valid native statistics descriptor was refused:", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("successful statistics validator ran %d times, want one", calls.Load())
	}
	requireStatsIdentityPeer(t, done)
	rows, err := client.DumpStats("^/fixture/scalar$")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("native statistics mapping returned %d scalar entries, want one", len(rows))
	}
	if !bytes.Equal(rows[0].Name, []byte("/fixture/scalar")) {
		t.Fatalf("native scalar name=%q", rows[0].Name)
	}
	if rows[0].Type != adapter.ScalarIndex {
		t.Fatalf("native scalar type=%v", rows[0].Type)
	}
	value, ok := rows[0].Data.(adapter.ScalarStat)
	if !ok {
		t.Fatalf("native scalar data type=%T", rows[0].Data)
	}
	if value != 4242 {
		t.Fatalf("native scalar value=%v, want 4242", value)
	}
	if err := client.Disconnect(); err != nil {
		t.Fatal(err)
	}
	if _, err := client.DumpStats(); !errors.Is(err, adapter.ErrStatsDisconnected) {
		t.Fatalf("disconnected native descriptor retained usable statistics: %v", err)
	}
}

// The 64-bit VPP V2 segment has a 48-byte shared header, a vector length at56
// and one 144-byte directory entry at64. The producer address is relocated by
// the adapter; these bytes are a native mapped segment, never Go private fields.
func statsIdentitySegment(t *testing.T) *os.File {
	t.Helper()
	data := make([]byte, 256)
	binary.NativeEndian.PutUint64(data[0:8], 2)
	binary.NativeEndian.PutUint64(data[8:16], 0x10000000)
	binary.NativeEndian.PutUint64(data[16:24], 1)
	binary.NativeEndian.PutUint64(data[32:40], 0x10000040)
	binary.NativeEndian.PutUint64(data[56:64], 1)
	binary.NativeEndian.PutUint32(data[64:68], 1)
	binary.NativeEndian.PutUint64(data[72:80], 4242)
	copy(data[80:208], "/fixture/scalar")
	file, err := os.CreateTemp(t.TempDir(), "segment-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := file.Write(data); err != nil {
		t.Fatal(err)
	}
	return file
}

// Both helpers use the original borrowed connection synchronously. A callback
// never retains or closes it, and the peer receives no ancillary descriptors.
func writeStatsIdentityProbe(conn *net.UnixConn, probe string) error {
	if conn == nil {
		return errors.New("statistics validator received no original socket")
	}
	if conn.RemoteAddr().Network() != "unixpacket" {
		return fmt.Errorf("statistics validator socket network=%s", conn.RemoteAddr().Network())
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

func readStatsIdentityProbe(conn *net.UnixConn, probe string) error {
	actual := make([]byte, len(probe)+1)
	control := make([]byte, syscall.CmsgSpace(4))
	n, ancillary, _, _, err := conn.ReadMsgUnix(actual, control)
	if err != nil {
		return err
	}
	if ancillary != 0 {
		return errors.New("statistics hook probe sent unexpected ancillary data")
	}
	if !bytes.Equal(actual[:n], []byte(probe)) {
		return fmt.Errorf("statistics transfer did not start with original-hook probe: %q", actual[:n])
	}
	return nil
}

func statsIdentityEOF(conn *net.UnixConn) error {
	var first [1]byte
	n, err := conn.Read(first[:])
	if n != 0 {
		return fmt.Errorf("closed statistics exchange sent unexpected bytes: %x", first[:n])
	}
	if !errors.Is(err, io.EOF) {
		return fmt.Errorf("original statistics exchange did not close with EOF: %w", err)
	}
	return nil
}

// The fixture owns one listener and one peer worker. stop closes those native
// resources and joins that worker once, including failures before admission.
// Only the deadline question additionally owns a Connect worker, which joins
// after stop has released the withheld native receive.
func startStatsIdentityPeer(t *testing.T, serve func(*net.UnixConn) error) (
	string, <-chan error, func()) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stats.sock")
	listener, err := net.ListenUnix("unixpacket", &net.UnixAddr{Name: path, Net: "unixpacket"})
	if err != nil {
		t.Fatal(err)
	}
	if err := listener.SetDeadline(time.Now().Add(statsIdentityBound)); err != nil {
		if closeErr := listener.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal(err)
	}
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
		accepted <- conn
		if err := conn.SetDeadline(time.Now().Add(statsIdentityBound)); err != nil {
			done <- errors.Join(err, conn.Close())
			return
		}
		err = serve(conn)
		closeErr := conn.Close()
		if errors.Is(closeErr, net.ErrClosed) {
			closeErr = nil
		}
		done <- errors.Join(err, closeErr)
	}()
	var stopped sync.Once
	stop := func() {
		stopped.Do(func() {
			if err := listener.Close(); err != nil {
				if !errors.Is(err, net.ErrClosed) {
					t.Error(err)
				}
			}
			select {
			case conn := <-accepted:
				if conn != nil {
					if err := conn.Close(); err != nil {
						if !errors.Is(err, net.ErrClosed) {
							t.Error(err)
						}
					}
				}
			case <-time.After(statsIdentityBound):
				t.Error("native statistics accept worker did not join")
			}
			select {
			case err := <-done:
				if err != nil {
					if !t.Failed() {
						t.Error(err)
					}
				}
			case <-time.After(statsIdentityBound):
				t.Error("native statistics peer worker did not join")
			}
		})
	}
	t.Cleanup(stop)
	return path, done, stop
}

func requireStatsIdentityPeer(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(statsIdentityBound):
		t.Fatal("native statistics peer did not confirm its exchange outcome")
	}
}

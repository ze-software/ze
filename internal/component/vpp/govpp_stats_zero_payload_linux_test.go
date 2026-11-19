//go:build integration && linux

// Design: docs/contributing/patching-the-standard-library.md -- native descriptor receive compatibility.
// Related: govpp_stats_identity_linux_test.go -- the shared segment, original socket probe and joined peer.
package vpp

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"go.fd.io/govpp/adapter"
	"go.fd.io/govpp/adapter/statsclient"
	"golang.org/x/sys/unix"
)

// A zero-data Unixpacket still carries its original SCM_RIGHTS descriptor.
// The native sender bypasses wrappers that insert an ordinary byte, so a
// receiver that discards ancillary data on a synthetic EOF cannot pass.
// MUTATION: returning early on ReadMsgUnix's EOF discards the received rights.
func TestGoVPPPinnedStatsMapsZeroPayloadDescriptor(t *testing.T) {
	const probe = "zero-payload-original-stats-hook"
	file := statsIdentitySegment(t)
	var calls atomic.Int32
	path, done, _ := startStatsIdentityPeer(t, func(conn *net.UnixConn) error {
		if err := readStatsIdentityProbe(conn, probe); err != nil {
			return err
		}
		if err := sendZeroPayloadStatsIdentity(conn, int(file.Fd())); err != nil {
			return err
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
		t.Fatal("zero-data native statistics descriptor was refused:", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("zero-data statistics validator ran %d times, want one", calls.Load())
	}
	requireStatsIdentityPeer(t, done)
	rows, err := client.DumpStats("^/fixture/scalar$")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("zero-data native mapping returned %d scalar entries, want one", len(rows))
	}
	if !bytes.Equal(rows[0].Name, []byte("/fixture/scalar")) {
		t.Fatalf("zero-data scalar name=%q", rows[0].Name)
	}
	if rows[0].Type != adapter.ScalarIndex {
		t.Fatalf("zero-data scalar type=%v", rows[0].Type)
	}
	value, ok := rows[0].Data.(adapter.ScalarStat)
	if !ok {
		t.Fatalf("zero-data scalar data type=%T", rows[0].Data)
	}
	if value != 4242 {
		t.Fatalf("zero-data scalar value=%v, want 4242", value)
	}
	if err := client.Disconnect(); err != nil {
		t.Fatal(err)
	}
	if _, err := client.DumpStats(); !errors.Is(err, adapter.ErrStatsDisconnected) {
		t.Fatalf("disconnected zero-data mapping retained usable statistics: %v", err)
	}
}

// WriteMsgUnix(nil, rights, nil) reports zero payload bytes but inserts one
// ordinary byte on Unixpacket. Use the documented raw sendmsg form with no
// iovecs to send a genuinely zero-data packet. The accepted peer owns conn;
// RawConn.Write borrows its descriptor synchronously and retains its deadline.
func sendZeroPayloadStatsIdentity(conn *net.UnixConn, descriptor int) error {
	rights := unix.UnixRights(descriptor)
	message := unix.Msghdr{Control: &rights[0]}
	message.SetControllen(len(rights))
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var sendErr error
	var sent int
	err = raw.Write(func(socket uintptr) bool {
		for {
			n, _, errno := unix.Syscall(unix.SYS_SENDMSG, socket, uintptr(unsafe.Pointer(&message)), 0)
			if errno == unix.EINTR {
				continue
			}
			if errno == unix.EAGAIN {
				return false
			}
			if errno != 0 {
				sendErr = errno
				return true
			}
			sent = int(n)
			return true
		}
	})
	runtime.KeepAlive(rights)
	if err != nil {
		return err
	}
	if sendErr != nil {
		return sendErr
	}
	if sent != 0 {
		return fmt.Errorf("zero-data statistics packet sent %d ordinary bytes", sent)
	}
	return nil
}

// Ordinary payload without SCM_RIGHTS must fail in descriptor receipt. The
// adapter must not accept an empty result and reach its later no-files check.
// MUTATION: accepting zero ancillary bytes changes the public receive error.
func TestGoVPPPinnedStatsRejectsMissingDescriptor(t *testing.T) {
	const probe = "missing-original-stats-descriptor"
	var calls atomic.Int32
	path, done, _ := startStatsIdentityPeer(t, func(conn *net.UnixConn) error {
		if err := readStatsIdentityProbe(conn, probe); err != nil {
			return err
		}
		n, err := conn.Write([]byte{1})
		if err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("missing-descriptor control wrote %d bytes, want one", n)
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
	err := client.Connect()
	if err == nil {
		t.Fatal("statistics connection accepted a packet without a descriptor")
	}
	if !bytes.HasPrefix([]byte(err.Error()), []byte("getting file descriptor over socket failed:")) {
		t.Fatalf("missing descriptor escaped the receive refusal: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("missing-descriptor validator ran %d times, want one", calls.Load())
	}
	requireStatsIdentityPeer(t, done)
	if _, err := client.DumpStats(); !errors.Is(err, adapter.ErrStatsDisconnected) {
		t.Fatalf("missing descriptor published a usable statistics mapping: %v", err)
	}
}

//go:build docker && linux

// Design: docs/architecture/testing/interop.md -- a package test's independent client container.
// Related: docs/architecture/provisioning/tftp-server.md -- duplicate ACKs do not trigger DATA.
// Related: handler.go -- serve and handleRRQ, then window_send.go serveFile, are the production path.
//
// The relay copies curl's packets; it never constructs an RRQ or an ACK. curl
// owns the client state machine and reassembles the downloaded file itself.
// Linux receive timestamps are required: a late userspace read must not turn
// an immediate duplicate-triggered packet into apparent timeout recovery.
// The SO_TIMESTAMPNS_NEW ABI is fixed-width on both 32-bit and 64-bit Linux.
package tftpserver

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/le/interoplab/labtest"
)

// curl 8.23.0 is the stable release of 2026-10-14:
// https://curl.se/mail/lib-2026-10/0003.html
// Multi-architecture manifest digest from:
// https://hub.docker.com/v2/repositories/curlimages/curl/tags/8.23.0
const tftpCurlImage = "curlimages/curl:8.23.0@sha256:11e64e28c5f58c8d93ee95fcf22ac055f48b0bc440e0ce7d8013bbb5ad6aa8ac"

// TestTFTPCurlDuplicateACKInterop loses DATA 2 in a bounded UDP relay, replays
// curl's ACK 1 while DATA 2 is outstanding, and requires silence followed by
// timeout recovery and curl's exact 1500-byte download. No test client generates
// protocol traffic, and no product timer is shortened.
//
// RFC 1350 Section 5: "All packets other than duplicate ACK's and those used for
// termination are acknowledged unless a timeout occurs [4]."
// RFC 1350 Section 5: "Sending a DATA packet is an acknowledgment for the first
// ACK packet of the previous DATA packet."
// MUTATION: In windowSender.awaitACK, return on an ACK that is not for the
// outstanding block (position 0), restoring send-on-duplicate behavior.
// The relay must fail with "DATA in answer to duplicate ACK" before accepting
// the recovered block; curl's successful download alone is not the verdict.
func TestTFTPCurlDuplicateACKInterop(t *testing.T) {
	version := labtest.Run(t, labtest.Client{
		Name: "tftp-curl-version", Image: tftpCurlImage,
		Command: []string{"--version"}, Timeout: 30 * time.Second,
	})
	if version.ExitCode != 0 {
		t.Fatalf("curl version exited %d: %s", version.ExitCode, version.Stderr)
	}
	if !strings.HasPrefix(version.Stdout, "curl 8.23.0 ") {
		t.Fatalf("unexpected independent client version: %s", version.Stdout)
	}
	if !strings.Contains(version.Stdout, " tftp") {
		t.Fatalf("independent client does not advertise TFTP: %s", version.Stdout)
	}
	t.Logf("independent client: %s", strings.TrimSpace(version.Stdout))

	root := t.TempDir()
	want := patterned(1500)
	if err := os.WriteFile(filepath.Join(root, "boot.bin"), want, 0o600); err != nil {
		t.Fatal(err)
	}
	server := startCurlTFTPServer(t, root)
	relay := startCurlTFTPRelay(t, server)

	result := labtest.Run(t, labtest.Client{
		Name: "tftp-curl-duplicate-ack", Image: tftpCurlImage, HostNetwork: true,
		Command: []string{"--silent", "--show-error", "--noproxy", "*",
			"--max-time", "35", "--tftp-no-options", "--output", "-",
			"tftp://" + relay.conn.LocalAddr().String() + "/boot.bin"},
		Timeout: 45 * time.Second,
	})
	<-relay.done
	if relay.err != nil {
		t.Fatalf("duplicate ACK relay: %v; curl exit=%d stderr=%s", relay.err, result.ExitCode, result.Stderr)
	}
	if result.ExitCode != 0 {
		t.Fatalf("curl transfer exited %d: %s", result.ExitCode, result.Stderr)
	}
	if !bytes.Equal([]byte(result.Stdout), want) {
		t.Fatalf("curl downloaded %d bytes, want the exact %d-byte file", len(result.Stdout), len(want))
	}
	t.Logf("replayed native ACK 1 once; dropped first DATA 2; kernel arrival interval %v; forwarded final ACK 3; curl matched all %d bytes (%d relay datagrams)",
		relay.recoveryAfter, len(want), relay.packets)
}

// startCurlTFTPServer gives the production request loop one transfer slot. Its
// cleanup MUST close the listener, join its worker, end the generation and drain
// that slot so failure during the suppressed DATA cannot leave a transfer behind.
func startCurlTFTPServer(t *testing.T, root string) netip.AddrPort {
	t.Helper()
	// Cleanup MUST close the socket returned by curlTFTPSocket.
	conn := curlTFTPSocket(t)
	generation := newTransferGeneration()
	sem := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		// RFC 1350 Sections 4 and 5: real request and transfer dispatch.
		serve(generation, conn, root, sem, slogutil.DiscardLogger())
	}()
	t.Cleanup(func() {
		closeUDP(t, conn)
		<-done
		generation.end(slogutil.DiscardLogger())
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		select {
		case sem <- struct{}{}:
		case <-timer.C:
			t.Error("TFTP transfer did not release its slot after generation end")
		}
	})
	address, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("TFTP listener has a non-UDP local address")
	}
	return address.AddrPort()
}

// curlTFTPSocket reserves an ephemeral loopback UDP port. Its caller MUST close it.
func curlTFTPSocket(t *testing.T) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

// curlTFTPRelay owns one transfer worker and one socket. Consumers MUST wait for
// done to close before reading its fields. Cleanup MUST close the socket and join
// the worker. The relay permits at most 64 datagrams, 60 seconds to the RRQ, and
// 30 seconds from the RRQ to the final ACK. No datagram is synthesized: ack1 is
// curl's own ACK.
//
// RFC 1350 Section 5 wire layout (network byte order):
//
//	byte 0..1       byte 2..3       byte 4..
//	+---------------+---------------+----------------+
//	| opcode (3/4)  | block number  | DATA payload   |
//	+---------------+---------------+----------------+
//
// ACK ends at byte 4; DATA has at most 512 payload octets without options.
type curlTFTPRelay struct {
	conn          *net.UDPConn
	done          chan struct{}
	err           error
	packets       int
	packet        [65536]byte
	control       [128]byte
	arrival       time.Time
	client        netip.AddrPort
	ack1          [4]byte
	discardACK1   bool
	deadline      time.Time
	recoveryAfter time.Duration
}

func startCurlTFTPRelay(t *testing.T, server netip.AddrPort) *curlTFTPRelay {
	t.Helper()
	// Cleanup MUST close the socket returned by curlTFTPSocket.
	relay := &curlTFTPRelay{conn: curlTFTPSocket(t), done: make(chan struct{})}
	if err := enableCurlTFTPTimestamps(relay.conn); err != nil {
		closeUDP(t, relay.conn)
		t.Fatal(err)
	}
	go func() {
		defer close(relay.done)
		// RFC 1350 Sections 4, 5 and 6: retain curl's transfer and ACKs.
		relay.err = relay.transfer(server)
	}()
	t.Cleanup(func() {
		closeUDP(t, relay.conn)
		<-relay.done
	})
	return relay
}

// transfer duplicates the independent client's ACK rather than implementing a
// client. RFC 1350 Section 5: "All packets other than duplicate ACK's and those
// used for termination are acknowledged unless a timeout occurs [4]."
// RFC 1350 Section 2: "If a packet gets lost in the network, the intended
// recipient will timeout and may retransmit his last packet (which may be data
// or an acknowledgment), thus causing the sender of the lost packet to
// retransmit that lost packet."
func (r *curlTFTPRelay) transfer(server netip.AddrPort) error {
	r.deadline = time.Now().Add(60 * time.Second)
	// RFC 1350 Section 5: relay the independent client's original RRQ.
	request, client, err := r.read(r.deadline)
	if err != nil {
		return fmt.Errorf("read curl RRQ: %w", err)
	}
	// RFC 1350 Section 5: require the native octet RRQ, without options.
	if !bytes.Equal(request, []byte("\x00\x01boot.bin\x00octet\x00")) {
		return fmt.Errorf("curl RRQ = %x, want octet boot.bin without options", request)
	}
	r.client = client
	r.deadline = time.Now().Add(30 * time.Second)
	if _, err := r.conn.WriteToUDPAddrPort(request, server); err != nil {
		return err
	}
	// RFC 1350 Section 4: learn Ze's transfer TID from its first DATA.
	first, tid, err := r.read(r.deadline)
	if err != nil {
		return fmt.Errorf("read first server DATA: %w", err)
	}
	if tid.Addr() != server.Addr() {
		return fmt.Errorf("DATA source %v differs from server %v", tid, server)
	}
	if tid.Port() == server.Port() {
		return fmt.Errorf("DATA came from request port %v, not a transfer TID", tid)
	}
	// RFC 1350 Section 5: inspect and forward DATA 1 unmodified.
	if err := curlTFTPPacket(first, 3, 1, 516); err != nil {
		return err
	}
	if _, err := r.conn.WriteToUDPAddrPort(first, client); err != nil {
		return err
	}
	// RFC 1350 Section 5: save and forward curl's first ACK 1.
	ack, err := r.expect(client, 4, 1, 4)
	if err != nil {
		return err
	}
	copy(r.ack1[:], ack)
	if _, err := r.conn.WriteToUDPAddrPort(ack, tid); err != nil {
		return err
	}
	clockStart := time.Now()
	// RFC 1350 Section 5: DATA 2 proves ACK 1 was consumed before replay.
	second, err := r.expect(tid, 3, 2, 516)
	if err != nil {
		return err
	}
	outstanding := r.arrival
	// Retain only the deliberately lost block across later socket reads.
	var lost [516]byte
	copy(lost[:], second)
	if _, err := r.conn.WriteToUDPAddrPort(r.ack1[:], tid); err != nil {
		return err
	}
	// Fail closed if scheduling delayed replay until near the real timeout.
	// Check AFTER send as well: a pre-send check alone leaves a scheduling gap.
	replayDelay := time.Since(outstanding)
	if replayDelay < 0 {
		return errors.New("kernel DATA 2 timestamp is in the future")
	}
	if replayDelay > time.Second {
		return fmt.Errorf("ACK replay was delayed %v after kernel DATA 2 arrival; no discrimination evidence", replayDelay)
	}
	// Suppress curl's own timeout re-ACKs too: only the single deliberate
	// duplicate reaches Ze while DATA 2 is outstanding.
	r.discardACK1 = true
	// RFC 1350 Sections 2 and 5: the FIRST server datagram after replay must
	// be timeout recovery, not an immediate acknowledgment of duplicate ACK 1.
	// recvmsg supplies kernel arrival time even if this worker was descheduled
	// and the packet waited in the socket queue. No userspace quiet-window
	// timeout can age a queued early packet into successful recovery evidence.
	recovered, err := r.expect(tid, 3, 2, 516)
	if err != nil {
		return fmt.Errorf("timeout recovery: %w", err)
	}
	clockEnd := time.Now()
	// SO_TIMESTAMPNS_NEW uses CLOCK_REALTIME, while Go carries a monotonic
	// component. Refuse a clock step that could distort the packet interval.
	clockStep := time.Duration(clockEnd.UnixNano()-clockStart.UnixNano()) - clockEnd.Sub(clockStart)
	if clockStep < -100*time.Millisecond {
		return fmt.Errorf("wall clock stepped backwards %v during recovery; no timing evidence", clockStep)
	}
	if clockStep > 100*time.Millisecond {
		return fmt.Errorf("wall clock stepped forwards %v during recovery; no timing evidence", clockStep)
	}
	// RFC 1350 Sections 2 and 5: arrival time, never delayed read completion.
	r.recoveryAfter, err = curlTFTPRecoveryInterval(outstanding, r.arrival)
	if err != nil {
		return err
	}
	if !bytes.Equal(recovered, lost[:]) {
		return errors.New("timeout recovery changed DATA 2 bytes")
	}
	if _, err := r.conn.WriteToUDPAddrPort(recovered, client); err != nil {
		return err
	}
	// RFC 1350 Section 5: only curl's fresh ACK 2 advances the server.
	ack, err = r.expect(client, 4, 2, 4)
	if err != nil {
		return err
	}
	if _, err := r.conn.WriteToUDPAddrPort(ack, tid); err != nil {
		return err
	}
	// RFC 1350 Section 6: 476 bytes in DATA 3 ends this 1500-byte transfer.
	last, err := r.expect(tid, 3, 3, 480)
	if err != nil {
		return err
	}
	if _, err := r.conn.WriteToUDPAddrPort(last, client); err != nil {
		return err
	}
	// RFC 1350 Section 6: forward the native client's final ACK before exit.
	ack, err = r.expect(client, 4, 3, 4)
	if err != nil {
		return err
	}
	_, err = r.conn.WriteToUDPAddrPort(ack, tid)
	return err
}

// read preserves whole UDP datagrams and bounds input even if a peer floods.
// The returned slice belongs to the relay and is valid only until the next read.
// RFC 1350 Section 5: "The block number in an ACK echoes the block number of
// the DATA packet being acknowledged." Only an exact copy of curl's ACK 1
// is discarded, and only after the deliberate replay. No other input is hidden.
func (r *curlTFTPRelay) read(deadline time.Time) ([]byte, netip.AddrPort, error) {
	if err := r.conn.SetReadDeadline(deadline); err != nil {
		return nil, netip.AddrPort{}, err
	}
	for r.packets < 64 {
		n, controlN, flags, source, err := r.conn.ReadMsgUDPAddrPort(r.packet[:], r.control[:])
		if err != nil {
			return nil, netip.AddrPort{}, err
		}
		if flags&unix.MSG_TRUNC != 0 {
			return nil, netip.AddrPort{}, errors.New("relay UDP payload was truncated")
		}
		if flags&unix.MSG_CTRUNC != 0 {
			return nil, netip.AddrPort{}, errors.New("relay receive timestamp was truncated")
		}
		arrival, err := curlTFTPReceiveTimestamp(r.control[:controlN])
		if err != nil {
			return nil, netip.AddrPort{}, err
		}
		r.packets++
		if r.discardACK1 && source == r.client && bytes.Equal(r.packet[:n], r.ack1[:]) {
			continue
		}
		r.arrival = arrival
		return r.packet[:n], source, nil
	}
	return nil, netip.AddrPort{}, errors.New("relay exceeded 64 datagrams")
}

// expect checks a frame before forwarding it. RFC 1350 Section 4: "The two
// chosen TID's are then used for the remainder of the transfer."
func (r *curlTFTPRelay) expect(source netip.AddrPort, opcode, block byte, length int) ([]byte, error) {
	// RFC 1350 Section 5: observe native frames without creating ACKs.
	packet, actual, err := r.read(r.deadline)
	if err != nil {
		return nil, err
	}
	if actual != source {
		return nil, fmt.Errorf("packet source %v, want transfer source %v", actual, source)
	}
	// RFC 1350 Section 5: validate the DATA/ACK header and payload length.
	if err := curlTFTPPacket(packet, opcode, block, length); err != nil {
		return nil, err
	}
	return packet, nil
}

// curlTFTPPacket inspects only this three-block scenario's DATA/ACK envelope.
// RFC 1350 Section 5: "DATA packets (opcode = 3) have a block number and data
// field." RFC 1350 Section 5: "Figure 5-3 depicts an ACK packet; the opcode is 4."
func curlTFTPPacket(packet []byte, opcode, block byte, length int) error {
	if len(packet) != length {
		return fmt.Errorf("opcode %d block %d packet length %d, want %d: %x", opcode, block, len(packet), length, packet)
	}
	if !bytes.Equal(packet[:4], []byte{0, opcode, 0, block}) {
		return fmt.Errorf("packet header %x, want opcode %d block %d", packet[:4], opcode, block)
	}
	return nil
}

// enableCurlTFTPTimestamps requests reception timestamps before the relay can
// receive traffic. Missing support is a failed proof, never a wall-clock fallback.
// Linux documents the NEW ABI as __kernel_timespec (two native-endian int64s):
// https://docs.kernel.org/networking/timestamping.html#so-timestampns-also-so-timestampns-old-and-so-timestampns-new
func enableCurlTFTPTimestamps(conn *net.UDPConn) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return fmt.Errorf("relay syscall connection: %w", err)
	}
	var optionErr error
	if err := raw.Control(func(fd uintptr) {
		optionErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_TIMESTAMPNS_NEW, 1)
	}); err != nil {
		return fmt.Errorf("relay timestamp socket control: %w", err)
	}
	if optionErr != nil {
		return fmt.Errorf("relay SO_TIMESTAMPNS_NEW: %w", optionErr)
	}
	return nil
}

// curlTFTPReceiveTimestamp accepts exactly the timestamp option requested above,
// with no unsafe or host-word-size-dependent cast. Ancillary data is kernel ABI,
// not TFTP: byte 0..7 is signed seconds, byte 8..15 signed nanoseconds.
func curlTFTPReceiveTimestamp(control []byte) (time.Time, error) {
	if len(control) < unix.CmsgLen(16) {
		return time.Time{}, fmt.Errorf("missing or short kernel receive timestamp (%d bytes)", len(control))
	}
	header, payload, remainder, err := unix.ParseOneSocketControlMessage(control)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse kernel receive timestamp: %w", err)
	}
	if header.Level != unix.SOL_SOCKET {
		return time.Time{}, fmt.Errorf("receive timestamp control level %d, want SOL_SOCKET", header.Level)
	}
	if header.Type != unix.SO_TIMESTAMPNS_NEW {
		return time.Time{}, fmt.Errorf("receive timestamp control type %d, want SO_TIMESTAMPNS_NEW", header.Type)
	}
	if len(remainder) != 0 {
		return time.Time{}, errors.New("unexpected additional relay control message")
	}
	if len(payload) != 16 {
		return time.Time{}, fmt.Errorf("receive timestamp payload has %d bytes, want 16", len(payload))
	}
	seconds := int64(binary.NativeEndian.Uint64(payload[:8]))
	nanos := int64(binary.NativeEndian.Uint64(payload[8:]))
	if seconds <= 0 {
		return time.Time{}, fmt.Errorf("invalid receive timestamp seconds %d", seconds)
	}
	if nanos < 0 {
		return time.Time{}, fmt.Errorf("invalid receive timestamp nanoseconds %d", nanos)
	}
	if nanos >= int64(time.Second) {
		return time.Time{}, fmt.Errorf("invalid receive timestamp nanoseconds %d", nanos)
	}
	return time.Unix(seconds, nanos), nil
}

// curlTFTPRecoveryInterval judges the first response by its kernel arrival time.
// RFC 1350 Section 5: "All packets other than duplicate ACK's and those used for
// termination are acknowledged unless a timeout occurs [4]."
func curlTFTPRecoveryInterval(first, next time.Time) (time.Duration, error) {
	interval := next.Sub(first)
	// Keep the conservative four-second floor for the five-second timeout.
	// A delayed reader cannot change this interval between kernel timestamps.
	if interval < 4*time.Second {
		return interval, fmt.Errorf("DATA in answer to duplicate ACK after %v, before five-second timeout", interval)
	}
	return interval, nil
}

// TestTFTPCurlQueuedDuplicateCannotAgeIntoRecovery reproduces a late reader:
// two packets arrive together, but the second is read after five seconds. The
// same timestamp decoder and arrival oracle as the interop must still reject
// it as immediate DATA, not turn queue residence into timeout evidence.
func TestTFTPCurlQueuedDuplicateCannotAgeIntoRecovery(t *testing.T) {
	receiver := curlTFTPSocket(t)
	t.Cleanup(func() { closeUDP(t, receiver) })
	if err := enableCurlTFTPTimestamps(receiver); err != nil {
		t.Fatal(err)
	}
	sender := curlTFTPSocket(t)
	t.Cleanup(func() { closeUDP(t, sender) })
	target, ok := receiver.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("timestamp receiver has a non-UDP address")
	}
	packet := []byte{0, 3, 0, 2}
	if _, err := sender.WriteToUDP(packet, target); err != nil {
		t.Fatal(err)
	}
	relay := curlTFTPRelay{conn: receiver}
	deadline := time.Now().Add(15 * time.Second)
	// RFC 1350 Section 5: observe the first DATA envelope's kernel arrival.
	if _, _, err := relay.read(deadline); err != nil {
		t.Fatal(err)
	}
	first := relay.arrival
	if _, err := sender.WriteToUDP(packet, target); err != nil {
		t.Fatal(err)
	}
	// This timer deliberately creates the reader scheduling gap under test;
	// no readiness or protocol state is inferred from waiting for it.
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}
	// RFC 1350 Section 5: the queued duplicate keeps its original arrival.
	if _, _, err := relay.read(deadline); err != nil {
		t.Fatal(err)
	}
	// RFC 1350 Sections 2 and 5: use the interop's actual recovery oracle.
	interval, err := curlTFTPRecoveryInterval(first, relay.arrival)
	if err == nil {
		t.Fatalf("queued duplicate became timeout recovery after %v", interval)
	}
	if !strings.Contains(err.Error(), "DATA in answer to duplicate ACK") {
		t.Fatalf("queued duplicate failed for an unrelated reason: %v", err)
	}
	t.Logf("queued duplicate remained early at kernel arrival interval %v", interval)
}

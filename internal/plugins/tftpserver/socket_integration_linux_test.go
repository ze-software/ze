// Design: docs/architecture/provisioning/tftp-server.md -- integration coverage for SO_BINDTODEVICE
//
// These tests pin socket_linux.go (listenTFTP): they exercise the real
// SO_BINDTODEVICE syscall and a real RRQ/DATA/ACK transfer over UDP, plus the
// device-filtering behavior the option provides. They require root (port 69 is
// privileged) and, for the negative case, CAP_NET_ADMIN to create a dummy
// interface. Both prerequisites are present in the QEMU Alpine VM
// (see ai/rules/platform-linux.md); on hosts without them the tests t.Skip.
//
// Why bind to "lo" for the positive round-trip rather than a veth/dummy:
// locally-routed traffic between two addresses in the same network namespace
// is delivered via the loopback device (skb->dev == lo), so only a socket
// bound to "lo" can both bind AND receive that traffic within one namespace.
// A veth round-trip would require two namespaces, and the TFTP transfer path
// spawns its own goroutine (handleRRQ -> go func -> net.DialUDP) that is not
// pinned to a namespace, which makes cross-namespace delivery unreliable. The
// dummy-interface negative test below proves the option genuinely filters by
// device, which is the property that matters.

//go:build integration && linux

package tftpserver

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// makeTestFile writes n deterministic bytes into dir/name and returns the
// content so the test can compare the transferred bytes byte-for-byte.
func makeTestFile(t *testing.T, dir, name string, n int) []byte {
	t.Helper()
	content := make([]byte, n)
	for i := range content {
		content[i] = byte(i*7 + 3) // deterministic, spans block boundaries
	}
	if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	return content
}

// buildRRQ encodes an RFC 1350 read request for filename in octet mode.
func buildRRQ(filename string) []byte {
	pkt := make([]byte, 0, 2+len(filename)+1+5+1)
	pkt = binary.BigEndian.AppendUint16(pkt, opRRQ)
	pkt = append(pkt, filename...)
	pkt = append(pkt, 0)
	pkt = append(pkt, "octet"...)
	pkt = append(pkt, 0)
	return pkt
}

// tftpFetch performs a minimal RFC 1350 client transfer against srvAddr,
// following the server's transfer TID and ACKing each DATA block. It returns
// the reassembled file, or an error if no DATA was ever received.
func tftpFetch(t *testing.T, srvAddr *net.UDPAddr, filename string) ([]byte, error) {
	t.Helper()

	cli, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("client listen: %v", err)
	}
	defer func() { _ = cli.Close() }()

	if _, err := cli.WriteToUDP(buildRRQ(filename), srvAddr); err != nil {
		t.Fatalf("send RRQ: %v", err)
	}

	var out bytes.Buffer
	var tid *net.UDPAddr
	expect := uint16(1)
	buf := make([]byte, 4+defaultBlockSize)

	for {
		if err := cli.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
			t.Fatalf("set deadline: %v", err)
		}
		n, from, rErr := cli.ReadFromUDP(buf)
		if rErr != nil {
			return nil, rErr // timeout or refusal before the transfer completed
		}
		if n < 4 {
			continue
		}
		op := binary.BigEndian.Uint16(buf[0:2])
		if op == opERROR {
			return nil, errors.New("server returned TFTP error")
		}
		if op != opDATA {
			continue
		}
		block := binary.BigEndian.Uint16(buf[2:4])
		if tid == nil {
			tid = from // first DATA fixes the server transfer TID
		}
		if block != expect {
			continue
		}
		out.Write(buf[4:n])

		// ACK to the transfer TID.
		ack := make([]byte, 4)
		binary.BigEndian.PutUint16(ack[0:2], opACK)
		binary.BigEndian.PutUint16(ack[2:4], block)
		if _, err := cli.WriteToUDP(ack, tid); err != nil {
			t.Fatalf("send ACK: %v", err)
		}

		if n-4 < defaultBlockSize {
			return out.Bytes(), nil // final (short) block
		}
		expect++
	}
}

// createDummyForTest creates a dummy interface and registers cleanup, skipping
// the test if CAP_NET_ADMIN is unavailable.
func createDummyForTest(t *testing.T, name string) {
	t.Helper()
	link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: name}}
	if err := netlink.LinkAdd(link); err != nil {
		t.Skipf("requires CAP_NET_ADMIN: create dummy %q: %v", name, err)
	}
	t.Cleanup(func() { _ = netlink.LinkDel(link) })
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("set dummy %q up: %v", name, err)
	}
}

// TestListenTFTPLoopbackRoundTrip binds the TFTP listener to lo with
// SO_BINDTODEVICE and performs a real multi-block RRQ transfer over UDP.
//
// RFC requirement: RFC1350-4-3 positive -- the transfer completes over the server's distinct
// transfer TID: handleRRQ dials a fresh socket per RRQ (internal/plugins/tftpserver/handler.go:287)
// so DATA arrives from an ephemeral port rather than port 69, and tftpFetch fixes that TID
// from the first DATA and ACKs to it (socket_integration_linux_test.go:104-118) to drive the
// transfer to completion.
func TestListenTFTPLoopbackRoundTrip(t *testing.T) {
	conn, err := listenTFTP("lo")
	if err != nil {
		t.Skipf("cannot bind lo:69 (needs root): %v", err)
	}
	defer func() { _ = conn.Close() }()

	dir := t.TempDir()
	want := makeTestFile(t, dir, "boot.bin", 1100) // 1100 bytes -> 3 blocks (512+512+76)

	sem := make(chan struct{}, 4)
	go serve(conn, dir, sem, slogutil.DiscardLogger())

	srv := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 69}
	got, err := tftpFetch(t, srv, "boot.bin")
	if err != nil {
		t.Fatalf("fetch over lo-bound socket: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("transferred bytes mismatch: got %d bytes, want %d", len(got), len(want))
	}
}

// TestListenTFTPDeviceFilter proves SO_BINDTODEVICE restricts the socket to its
// device: a listener bound to a dummy interface must not answer a request that
// arrives over loopback.
func TestListenTFTPDeviceFilter(t *testing.T) {
	createDummyForTest(t, "zetftpd0")

	conn, err := listenTFTP("zetftpd0")
	if err != nil {
		t.Skipf("cannot bind zetftpd0:69 (needs root): %v", err)
	}
	defer func() { _ = conn.Close() }()

	dir := t.TempDir()
	makeTestFile(t, dir, "boot.bin", 64)

	sem := make(chan struct{}, 4)
	go serve(conn, dir, sem, slogutil.DiscardLogger())

	srv := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 69}
	if got, err := tftpFetch(t, srv, "boot.bin"); err == nil {
		t.Fatalf("expected no response from device-bound socket over lo, got %d bytes", len(got))
	}
}

// TestTFTPServerBindsViaResolveWithoutBackend pins the install-path fix at the
// plugin's actual bind path: bindDeviceFor returns the configured name (no
// iface backend is loaded in the test binary, as in `ze-setup install remote`)
// and listenTFTP binds it for a real RRQ transfer. The pre-fix code logged a
// resolve error and dropped the listener, leaving the TFTP server with none.
func TestTFTPServerBindsViaResolveWithoutBackend(t *testing.T) {
	device, _ := bindDeviceFor("lo")
	if device != "lo" {
		t.Fatalf("bindDeviceFor(%q) = %q, want \"lo\"", "lo", device)
	}

	conn, err := listenTFTP(device)
	if err != nil {
		t.Skipf("cannot bind %s:69 (needs root): %v", device, err)
	}
	defer func() { _ = conn.Close() }()

	dir := t.TempDir()
	want := makeTestFile(t, dir, "boot.bin", 600)

	sem := make(chan struct{}, 4)
	go serve(conn, dir, sem, slogutil.DiscardLogger())

	srv := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 69}
	got, err := tftpFetch(t, srv, "boot.bin")
	if err != nil {
		t.Fatalf("fetch via no-backend bind path: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("transferred bytes mismatch: got %d bytes, want %d", len(got), len(want))
	}
}

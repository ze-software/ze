// Design: docs/architecture/provisioning/tftp-server.md -- panic containment in the request and transfer tasks
// Related: panic_guard.go -- requestTask and transferTask, the boundaries under test
// Related: handler.go -- serveRequests and runTransfer, the code the guards wrap

package tftpserver

import (
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestTFTPRequestPanicContained proves a listener's request task contains a Ze
// defect met while it answers one request: the client is sent an ERROR with
// code 0, the listener answers the next request, and the panic is counted and
// crash-recorded on the real guard.
// METHOD: serve runs with no logger, a state only a defect produces since
// startServer always passes one. A read request reaches handleRRQ's log line
// and panics; a write request, whose answer needs no logger, must then be
// answered with its own ERROR (code 4).
// PREVENTS: a request-driven panic killing the daemon, or ending the listener
// and with it every later PXE boot on that interface.
func TestTFTPRequestPanicContained(t *testing.T) {
	before, declared := tftpGuard.Panics(taskRequest)
	if !declared {
		t.Fatalf("task %q is not declared on the tftp boundary", taskRequest)
	}
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))

	conn, server := listenLoopback(t)
	done := make(chan struct{})
	go func() {
		defer close(done)
		serve(newTransferGeneration(), conn, t.TempDir(), make(chan struct{}, 1), nil)
	}()
	t.Cleanup(func() {
		closeUDP(t, conn)
		<-done
	})
	client, _ := listenLoopback(t)
	t.Cleanup(func() { closeUDP(t, client) })

	if _, err := client.WriteToUDP(buildRRQPacket("ipxe.efi", "octet"), server); err != nil {
		t.Fatalf("send the RRQ that panics: %v", err)
	}
	if code := readErrorCode(t, client); code != errNotDefined {
		t.Fatalf("the panicked request was answered with ERROR code %d, want %d", code, errNotDefined)
	}

	wrq := binary.BigEndian.AppendUint16(nil, opWRQ)
	wrq = append(wrq, "upload\x00octet\x00"...)
	if _, err := client.WriteToUDP(wrq, server); err != nil {
		t.Fatalf("send the WRQ after the panic: %v", err)
	}
	if code := readErrorCode(t, client); code != errIllegalOp {
		t.Fatalf("the WRQ after the panic was answered with ERROR code %d, want %d: the listener did not carry on", code, errIllegalOp)
	}

	if count, _ := tftpGuard.Panics(taskRequest); count != before+1 {
		t.Fatalf("Panics(%q) = %d, want %d", taskRequest, count, before+1)
	}
	assertOneCrash(t, "tftp "+taskRequest+", unit request from "+client.LocalAddr().String())
}

// TestTFTPTransferPanicContained proves one transfer's task contains a Ze
// defect met while it serves a file: its client is sent an ERROR with code 0
// on the transfer's socket, the transfer's slot and socket are released, the
// next transfer completes, and the panic is counted and crash-recorded.
// METHOD: runTransfer is handed a negative block size, a state only a defect
// produces since handleRRQ clamps it to [blksizeMin, blksizeEthernet];
// serveFile's buffer allocation panics on it. The semaphore holds one slot,
// so a read request served through the listener afterwards proves the
// panicked transfer gave its slot back.
// PREVENTS: a transfer-driven panic killing the daemon, or leaking the slot
// and the socket until every later boot is refused as service unavailable.
func TestTFTPTransferPanicContained(t *testing.T) {
	before, declared := tftpGuard.Panics(taskTransfer)
	if !declared {
		t.Fatalf("task %q is not declared on the tftp boundary", taskTransfer)
	}
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ipxe.efi"), []byte("boot"), 0o600); err != nil {
		t.Fatal(err)
	}
	generation := newTransferGeneration()
	sem := make(chan struct{}, 1)
	client, clientAddr := listenLoopback(t)
	t.Cleanup(func() { closeUDP(t, client) })

	sem <- struct{}{} // the slot handleRRQ takes before it starts a transfer
	defective := &transfer{clientAddr: clientAddr, path: filepath.Join(root, "ipxe.efi"), blksize: -1}
	runTransfer(generation, defective, sem, slogutil.DiscardLogger())
	if code := readErrorCode(t, client); code != errNotDefined {
		t.Fatalf("the panicked transfer was answered with ERROR code %d, want %d", code, errNotDefined)
	}
	if held := len(generation.transfers); held != 0 {
		t.Fatalf("the panicked transfer left %d socket(s) in its generation", held)
	}

	conn, server := listenLoopback(t)
	done := make(chan struct{})
	go func() {
		defer close(done)
		serve(generation, conn, root, sem, slogutil.DiscardLogger())
	}()
	t.Cleanup(func() {
		closeUDP(t, conn)
		<-done
	})
	if _, err := client.WriteToUDP(buildRRQPacket("ipxe.efi", "octet"), server); err != nil {
		t.Fatalf("send the RRQ after the panic: %v", err)
	}
	reply := make([]byte, 516)
	if err := client.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, _, err := client.ReadFromUDP(reply)
	if err != nil {
		t.Fatalf("no answer to the RRQ after the panic: %v", err)
	}
	if op := binary.BigEndian.Uint16(reply[:2]); op != opDATA || string(reply[4:n]) != "boot" {
		t.Fatalf("the RRQ after the panic got opcode %d %q, want DATA \"boot\": the slot was not released", op, reply[4:n])
	}

	if count, _ := tftpGuard.Panics(taskTransfer); count != before+1 {
		t.Fatalf("Panics(%q) = %d, want %d", taskTransfer, count, before+1)
	}
	assertOneCrash(t, "tftp "+taskTransfer+", unit transfer of "+defective.path+" to "+clientAddr.String())
}

// listenLoopback opens an unprivileged UDP socket on 127.0.0.1.
func listenLoopback(t *testing.T) (*net.UDPConn, *net.UDPAddr) {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("listener address %T is not a *net.UDPAddr", conn.LocalAddr())
	}
	return conn, addr
}

// readErrorCode reads one packet from conn and answers its ERROR code,
// failing the test when none arrives or it is not an ERROR.
func readErrorCode(t *testing.T, conn *net.UDPConn) uint16 {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 516)
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("no ERROR packet: %v", err)
	}
	if n < 4 || binary.BigEndian.Uint16(buf[:2]) != opERROR {
		t.Fatalf("got %x, want an ERROR packet", buf[:n])
	}
	return binary.BigEndian.Uint16(buf[2:4])
}

// assertOneCrash fails the test unless exactly one recovered crash record
// exists and it contains want.
func assertOneCrash(t *testing.T, want string) {
	t.Helper()
	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 || crashes[0].Kind != crashlog.KindRecovered {
		t.Fatalf("crash records = %+v, want one %q record", crashes, crashlog.KindRecovered)
	}
	if record := crashlog.ReadCrash(crashes[0].Name); !strings.Contains(record, want) {
		t.Fatalf("crash record lacks %q:\n%s", want, record)
	}
}

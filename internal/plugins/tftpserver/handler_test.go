package tftpserver

import (
	"bytes"
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
)

func closeUDP(t *testing.T, c *net.UDPConn) {
	t.Helper()
	if err := c.Close(); err != nil {
		t.Logf("close: %v", err)
	}
}

// syncBuf is a goroutine-safe io.Writer for capturing logger output written by
// the serve goroutine while the test goroutine reads it.
type syncBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// TestTFTPReadRequestLogged verifies a read request is logged at info with the
// filename, so a provisioning operator sees the bootloader being fetched.
func TestTFTPReadRequestLogged(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "ipxe.efi"), []byte("boot"), 0o644); err != nil {
		t.Fatal(err)
	}

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeUDP(t, conn) })

	var buf syncBuf
	log := slogutil.LoggerWithOutput("tftpserver", "info", &buf)
	sem := make(chan struct{}, 4)
	go serve(conn, rootDir, sem, log)

	srvAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("unexpected address type")
	}

	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer closeUDP(t, client)

	rrq := buildRRQPacket("ipxe.efi", "octet")
	if _, err := client.WriteToUDP(rrq, srvAddr); err != nil {
		t.Fatal(err)
	}

	// Wait for the DATA response: the read-request log is emitted synchronously
	// in the serve goroutine before the file transfer begins, so once the client
	// has DATA the log line is already in the buffer.
	rbuf := make([]byte, 516)
	if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := client.ReadFromUDP(rbuf); err != nil {
		t.Fatalf("no DATA response: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "read request") || !strings.Contains(out, "ipxe.efi") {
		t.Errorf("expected a read-request log naming ipxe.efi, got %q", out)
	}
}

// RFC requirement: RFC7440-3-1 positive -- a well-formed RRQ whose filename and mode
// fields each end in a single-byte NUL parses into the correct filename and mode
// (producer parseRRQ splits each field on its NUL terminator,
// internal/plugins/tftpserver/handler.go:61,77,90). RFC 7440 restates the RFC 2347
// wire format; this same NUL-terminated field discipline governs the windowsize
// option field even though ze does not implement that option.
func TestTFTPParseRRQ(t *testing.T) {
	t.Parallel()

	pkt := make([]byte, 0, 30)
	pkt = binary.BigEndian.AppendUint16(pkt, opRRQ)
	pkt = append(pkt, "bootloader.bin"...)
	pkt = append(pkt, 0)
	pkt = append(pkt, "octet"...)
	pkt = append(pkt, 0)

	filename, mode, _, err := parseRRQ(pkt)
	if err != nil {
		t.Fatalf("parseRRQ: %v", err)
	}
	if filename != "bootloader.bin" {
		t.Errorf("filename = %q, want bootloader.bin", filename)
	}
	if mode != "octet" {
		t.Errorf("mode = %q, want octet", mode)
	}
}

// RFC requirement: RFC7440-3-1 negative -- an RRQ whose filename or mode field lacks
// its single-byte NUL terminator is rejected: the "no filename NUL" and "no mode NUL"
// cases carry an unterminated field and parseRRQ returns an error rather than
// accepting it (producer parseRRQ returns "missing filename"/"missing mode" when no
// NUL follows the field, internal/plugins/tftpserver/handler.go:74-75,87-88).
func TestTFTPParseRRQInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pkt  []byte
	}{
		{"too short", []byte{0, 1, 'a'}},
		{"no filename NUL", func() []byte {
			p := make([]byte, 0, 10)
			p = binary.BigEndian.AppendUint16(p, opRRQ)
			p = append(p, "abcd"...)
			return p
		}()},
		{"empty filename", func() []byte {
			p := make([]byte, 0, 10)
			p = binary.BigEndian.AppendUint16(p, opRRQ)
			p = append(p, 0)
			p = append(p, "octet"...)
			p = append(p, 0)
			return p
		}()},
		{"no mode NUL", func() []byte {
			p := make([]byte, 0, 20)
			p = binary.BigEndian.AppendUint16(p, opRRQ)
			p = append(p, "file"...)
			p = append(p, 0)
			p = append(p, "octet"...)
			return p
		}()},
		{"empty mode", func() []byte {
			p := make([]byte, 0, 10)
			p = binary.BigEndian.AppendUint16(p, opRRQ)
			p = append(p, "file"...)
			p = append(p, 0, 0)
			return p
		}()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, _, err := parseRRQ(tc.pkt)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestTFTPBuildDataPacket(t *testing.T) {
	t.Parallel()

	data := []byte("hello")
	pkt := buildData(1, data)

	if len(pkt) != 4+len(data) {
		t.Fatalf("packet length = %d, want %d", len(pkt), 4+len(data))
	}
	if binary.BigEndian.Uint16(pkt[0:2]) != opDATA {
		t.Errorf("opcode = %d, want %d", binary.BigEndian.Uint16(pkt[0:2]), opDATA)
	}
	if binary.BigEndian.Uint16(pkt[2:4]) != 1 {
		t.Errorf("block = %d, want 1", binary.BigEndian.Uint16(pkt[2:4]))
	}
	if string(pkt[4:]) != "hello" {
		t.Errorf("data = %q, want hello", string(pkt[4:]))
	}
}

func TestTFTPBuildErrorPacket(t *testing.T) {
	t.Parallel()

	pkt := buildError(errFileNotFound, "file not found")

	if binary.BigEndian.Uint16(pkt[0:2]) != opERROR {
		t.Errorf("opcode = %d, want %d", binary.BigEndian.Uint16(pkt[0:2]), opERROR)
	}
	if binary.BigEndian.Uint16(pkt[2:4]) != errFileNotFound {
		t.Errorf("error code = %d, want %d", binary.BigEndian.Uint16(pkt[2:4]), errFileNotFound)
	}
	msg := string(pkt[4 : len(pkt)-1])
	if msg != "file not found" {
		t.Errorf("message = %q, want 'file not found'", msg)
	}
	if pkt[len(pkt)-1] != 0 {
		t.Error("missing NUL terminator")
	}
}

func TestTFTPPathTraversal(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "allowed.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		file    string
		wantErr bool
	}{
		{"valid file", "allowed.txt", false},
		{"dotdot", "../etc/passwd", true},
		{"absolute", "/etc/passwd", true},
		{"dotdot nested", "sub/../../etc/passwd", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := resolvePath(rootDir, tc.file)
			if tc.wantErr && err == nil {
				t.Error("expected error for path traversal")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestTFTPSymlinkTraversal(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	outsideDir := t.TempDir()
	secretFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(secretFile, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	linkPath := filepath.Join(rootDir, "escape")
	if err := os.Symlink(outsideDir, linkPath); err != nil {
		t.Skip("symlinks not supported")
	}

	_, err := resolvePath(rootDir, "escape/secret.txt")
	if err == nil {
		t.Error("expected error for symlink traversal")
	}
}

func buildRRQPacket(filename, mode string) []byte {
	pkt := make([]byte, 0, 2+len(filename)+1+len(mode)+1)
	pkt = binary.BigEndian.AppendUint16(pkt, opRRQ)
	pkt = append(pkt, filename...)
	pkt = append(pkt, 0)
	pkt = append(pkt, mode...)
	pkt = append(pkt, 0)
	return pkt
}

func buildRRQPacketWithOptions(filename string, opts ...string) []byte {
	pkt := buildRRQPacket(filename, "octet")
	for _, o := range opts {
		pkt = append(pkt, o...)
		pkt = append(pkt, 0)
	}
	return pkt
}

func buildWRQPacket(filename, mode string) []byte {
	pkt := make([]byte, 0, 2+len(filename)+1+len(mode)+1)
	pkt = binary.BigEndian.AppendUint16(pkt, opWRQ)
	pkt = append(pkt, filename...)
	pkt = append(pkt, 0)
	pkt = append(pkt, mode...)
	pkt = append(pkt, 0)
	return pkt
}

func buildACKPacket(block uint16) []byte {
	pkt := make([]byte, 4)
	binary.BigEndian.PutUint16(pkt[0:2], opACK)
	binary.BigEndian.PutUint16(pkt[2:4], block)
	return pkt
}

func startTestTFTPServer(t *testing.T, rootDir string, maxTransfers int) *net.UDPAddr {
	t.Helper()

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeUDP(t, conn) })

	sem := make(chan struct{}, maxTransfers)
	log := slogutil.DiscardLogger()

	go serve(conn, rootDir, sem, log)

	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("unexpected address type")
	}
	return addr
}

// RFC requirement: RFC1350-4-1 positive -- the first DATA block carries block number 1
// (asserts buf[2:4] == 1; producer numbers from 1 at
// internal/plugins/tftpserver/handler.go:362).
// RFC requirement: RFC1350-5-2 positive -- an octet transfer returns the file bytes
// unchanged (asserts bytes.Equal(buf[4:n], content); producer copies bytes verbatim into
// DATA at internal/plugins/tftpserver/handler.go:360,373).
func TestTFTPReadRequest(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	content := []byte("hello tftp")
	if err := os.WriteFile(filepath.Join(rootDir, "test.bin"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	srvAddr := startTestTFTPServer(t, rootDir, 10)

	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer closeUDP(t, client)

	rrq := buildRRQPacket("test.bin", "octet")
	if _, err := client.WriteToUDP(rrq, srvAddr); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 516)
	if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, _, err := client.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("no DATA response: %v", err)
	}

	if n < 4 {
		t.Fatalf("response too short: %d bytes", n)
	}
	if binary.BigEndian.Uint16(buf[0:2]) != opDATA {
		t.Fatalf("opcode = %d, want %d (DATA)", binary.BigEndian.Uint16(buf[0:2]), opDATA)
	}
	if binary.BigEndian.Uint16(buf[2:4]) != 1 {
		t.Fatalf("block = %d, want 1", binary.BigEndian.Uint16(buf[2:4]))
	}
	if !bytes.Equal(buf[4:n], content) {
		t.Errorf("data = %q, want %q", buf[4:n], content)
	}
}

// RFC requirement: RFC2348-x-4 positive -- a data block shorter than the negotiated
// blocksize signals the end of the transfer: a 1500-byte file over the 512-byte
// blocksize ends after exactly three blocks (512+512+476), the last one short
// (RFC 2348; producer serveFile stops after n < blksize, internal/plugins/tftpserver/handler.go:379).
// RFC requirement: RFC2348-x-5 negative -- when the transfer size is NOT an exact
// multiple of the blocksize (1500 vs 512), NO extra zero-length data packet is sent:
// the short final block itself ends the transfer (only three blocks total).
// RFC requirement: RFC1350-2-1 positive -- a 1500-byte octet file is framed in fixed
// 512-byte blocks with a short final block ending the transfer (512+512+476); producer
// internal/plugins/tftpserver/handler.go:360,373,379.
// RFC requirement: RFC1350-2-2 positive -- lockstep: the client ACKs each DATA block before
// the next is read, so the server advances only after the ACK
// (internal/plugins/tftpserver/handler.go:374,382).
// RFC requirement: RFC1350-4-1 positive -- block numbers are consecutive (1,2,3), asserted
// gotBlock == block each iteration (internal/plugins/tftpserver/handler.go:362,382).
// RFC requirement: RFC1350-5-2 positive -- the reassembled octet bytes are identical to the
// source file, asserted byte-for-byte (internal/plugins/tftpserver/handler.go:360,373).
func TestTFTPReadLargeFile(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	content := make([]byte, 1500)
	for i := range content {
		content[i] = byte(i % 256)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "large.bin"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	srvAddr := startTestTFTPServer(t, rootDir, 10)

	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer closeUDP(t, client)

	rrq := buildRRQPacket("large.bin", "octet")
	if _, err := client.WriteToUDP(rrq, srvAddr); err != nil {
		t.Fatal(err)
	}

	var received []byte
	buf := make([]byte, 516)
	expectedBlocks := 3 // 512 + 512 + 476

	for block := uint16(1); block <= uint16(expectedBlocks); block++ {
		if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatal(err)
		}
		n, senderAddr, readErr := client.ReadFromUDP(buf)
		if readErr != nil {
			t.Fatalf("block %d: no DATA: %v", block, readErr)
		}

		if binary.BigEndian.Uint16(buf[0:2]) != opDATA {
			t.Fatalf("block %d: opcode = %d, want DATA", block, binary.BigEndian.Uint16(buf[0:2]))
		}
		gotBlock := binary.BigEndian.Uint16(buf[2:4])
		if gotBlock != block {
			t.Fatalf("got block %d, want %d", gotBlock, block)
		}

		received = append(received, buf[4:n]...)

		ack := buildACKPacket(block)
		if _, wErr := client.WriteToUDP(ack, senderAddr); wErr != nil {
			t.Fatal(wErr)
		}
	}

	if len(received) != len(content) {
		t.Fatalf("received %d bytes, want %d", len(received), len(content))
	}
	for i := range content {
		if received[i] != content[i] {
			t.Fatalf("mismatch at byte %d: got %d, want %d", i, received[i], content[i])
		}
	}
}

// RFC requirement: RFC2348-x-5 positive -- when the transfer size is an exact multiple
// of the blocksize (512 bytes over the 512-byte block), an extra zero-length data packet
// is sent to end the transfer: block 1 carries 512 bytes and block 2 carries 0 bytes
// (RFC 2348; producer serveFile keeps reading until a short/zero block, internal/plugins/tftpserver/handler.go:379).
// RFC requirement: RFC2348-x-4 negative -- a FULL block (exactly the blocksize) does NOT
// signal end of transfer: block 1 is 512 bytes and the transfer continues to block 2, so
// only a shorter-than-blocksize block ends it.
// RFC requirement: RFC1350-2-1 positive -- a full 512-byte block does not end the transfer:
// block 1 carries 512 bytes and the transfer continues, only a short/zero block ends it
// (internal/plugins/tftpserver/handler.go:379).
// RFC requirement: RFC1350-5-3 positive -- a file that is an exact multiple of the block
// size ends with a zero-length DATA block: a 512-byte file yields block 1 (512B) then
// block 2 (0B) (internal/plugins/tftpserver/handler.go:364-383).
func TestTFTPReadExact512(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	content := make([]byte, 512)
	for i := range content {
		content[i] = byte(i)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "exact.bin"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	srvAddr := startTestTFTPServer(t, rootDir, 10)

	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer closeUDP(t, client)

	rrq := buildRRQPacket("exact.bin", "octet")
	if _, err := client.WriteToUDP(rrq, srvAddr); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 516)

	// Block 1: 512 bytes
	if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, senderAddr, err := client.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("block 1: %v", err)
	}
	if n-4 != 512 {
		t.Fatalf("block 1: data = %d bytes, want 512", n-4)
	}

	ack := buildACKPacket(1)
	if _, err := client.WriteToUDP(ack, senderAddr); err != nil {
		t.Fatal(err)
	}

	// Block 2: 0 bytes (signals end)
	if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, _, err = client.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("block 2: %v", err)
	}
	if binary.BigEndian.Uint16(buf[2:4]) != 2 {
		t.Fatalf("block number = %d, want 2", binary.BigEndian.Uint16(buf[2:4]))
	}
	if n-4 != 0 {
		t.Fatalf("block 2: data = %d bytes, want 0 (end signal)", n-4)
	}
}

// RFC requirement: RFC1350-5-3 positive -- an empty file (the zero-length exact-multiple
// case) is served as a single zero-length DATA block 1 that ends the transfer
// (internal/plugins/tftpserver/handler.go:364-383).
func TestTFTPReadEmptyFile(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "empty.bin"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	srvAddr := startTestTFTPServer(t, rootDir, 10)

	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer closeUDP(t, client)

	rrq := buildRRQPacket("empty.bin", "octet")
	if _, err := client.WriteToUDP(rrq, srvAddr); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 516)
	if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, _, err := client.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("no DATA: %v", err)
	}
	if binary.BigEndian.Uint16(buf[0:2]) != opDATA {
		t.Fatalf("opcode = %d, want DATA", binary.BigEndian.Uint16(buf[0:2]))
	}
	if binary.BigEndian.Uint16(buf[2:4]) != 1 {
		t.Fatalf("block = %d, want 1", binary.BigEndian.Uint16(buf[2:4]))
	}
	if n-4 != 0 {
		t.Fatalf("data = %d bytes, want 0", n-4)
	}
}

func TestTFTPWriteRejected(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	srvAddr := startTestTFTPServer(t, rootDir, 10)

	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer closeUDP(t, client)

	wrq := buildWRQPacket("file.bin", "octet")
	if _, err := client.WriteToUDP(wrq, srvAddr); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 100)
	if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, _, err := client.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("no ERROR response: %v", err)
	}
	if binary.BigEndian.Uint16(buf[0:2]) != opERROR {
		t.Fatalf("opcode = %d, want ERROR", binary.BigEndian.Uint16(buf[0:2]))
	}
	if binary.BigEndian.Uint16(buf[2:4]) != errIllegalOp {
		t.Errorf("error code = %d, want %d (illegal operation)", binary.BigEndian.Uint16(buf[2:4]), errIllegalOp)
	}
	_ = n
}

func TestTFTPFileNotFound(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	srvAddr := startTestTFTPServer(t, rootDir, 10)

	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer closeUDP(t, client)

	rrq := buildRRQPacket("nonexistent.bin", "octet")
	if _, err := client.WriteToUDP(rrq, srvAddr); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 100)
	if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, _, err := client.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("no ERROR response: %v", err)
	}

	opcode := binary.BigEndian.Uint16(buf[0:2])

	// Server may send access violation (from handleRRQ path validation)
	// or file not found (from serveFile). Both are correct.
	if opcode != opERROR {
		t.Fatalf("opcode = %d, want ERROR", opcode)
	}
	errCode := binary.BigEndian.Uint16(buf[2:4])
	if errCode != errFileNotFound && errCode != errAccessViolation {
		t.Errorf("error code = %d, want file-not-found (%d) or access-violation (%d)",
			errCode, errFileNotFound, errAccessViolation)
	}
	_ = n
}

func TestTFTPModeHandling(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "test.bin"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	srvAddr := startTestTFTPServer(t, rootDir, 10)

	// "netascii" mode should be rejected
	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer closeUDP(t, client)

	rrq := buildRRQPacket("test.bin", "netascii")
	if _, err := client.WriteToUDP(rrq, srvAddr); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 100)
	if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, _, err := client.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("no ERROR response: %v", err)
	}
	if binary.BigEndian.Uint16(buf[0:2]) != opERROR {
		t.Fatalf("opcode = %d, want ERROR", binary.BigEndian.Uint16(buf[0:2]))
	}
	if binary.BigEndian.Uint16(buf[2:4]) != errIllegalOp {
		t.Errorf("error code = %d, want %d (illegal operation)", binary.BigEndian.Uint16(buf[2:4]), errIllegalOp)
	}
	_ = n
}

func TestTFTPConcurrentLimit(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	// Create a file large enough that transfers take time
	content := make([]byte, 10*defaultBlockSize)
	if err := os.WriteFile(filepath.Join(rootDir, "big.bin"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	srvAddr := startTestTFTPServer(t, rootDir, 1)

	// Start first transfer (will hold the semaphore)
	client1, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer closeUDP(t, client1)

	rrq1 := buildRRQPacket("big.bin", "octet")
	if _, err := client1.WriteToUDP(rrq1, srvAddr); err != nil {
		t.Fatal(err)
	}

	// Wait for first transfer to start
	buf := make([]byte, 516)
	if err := client1.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := client1.ReadFromUDP(buf); err != nil {
		t.Fatalf("first transfer didn't start: %v", err)
	}
	// RFC requirement: RFC1350-2-2 negative -- the client receives DATA block 1 but sends no
	// ACK, so the server does not advance to block 2: it stays blocked in sendAndWaitACK
	// (internal/plugins/tftpserver/handler.go:386-413) holding the semaphore, which is what
	// lets the second transfer below be rejected.
	// Don't ACK - keeps the transfer alive and holding the semaphore

	// Try second transfer - should get rejected
	client2, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer closeUDP(t, client2)

	rrq2 := buildRRQPacket("big.bin", "octet")
	if _, err := client2.WriteToUDP(rrq2, srvAddr); err != nil {
		t.Fatal(err)
	}

	if err := client2.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, _, err := client2.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("no response for second transfer: %v", err)
	}
	if binary.BigEndian.Uint16(buf[0:2]) != opERROR {
		t.Fatalf("expected ERROR for concurrent limit, got opcode %d", binary.BigEndian.Uint16(buf[0:2]))
	}
	_ = n
}

// RFC requirement: RFC1350-6-1 positive -- when the last DATA goes unacknowledged the server
// retransmits the identical block after the ACK timeout (asserts the retransmit equals the
// original; internal/plugins/tftpserver/handler.go:387-400).
// RFC requirement: RFC1350-7-1 positive -- the server uses a read timeout to detect the
// missing ACK: the deadline expiry (internal/plugins/tftpserver/handler.go:392,398) is what
// triggers the retransmission this test observes.
func TestTFTPRetransmitOnTimeout(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	content := []byte("retransmit test data")
	if err := os.WriteFile(filepath.Join(rootDir, "retry.bin"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	srvAddr := startTestTFTPServer(t, rootDir, 10)

	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer closeUDP(t, client)

	rrq := buildRRQPacket("retry.bin", "octet")
	if _, err := client.WriteToUDP(rrq, srvAddr); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 516)
	if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, senderAddr, err := client.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("first DATA: %v", err)
	}
	if binary.BigEndian.Uint16(buf[0:2]) != opDATA || binary.BigEndian.Uint16(buf[2:4]) != 1 {
		t.Fatalf("expected DATA block 1, got opcode=%d block=%d",
			binary.BigEndian.Uint16(buf[0:2]), binary.BigEndian.Uint16(buf[2:4]))
	}
	firstData := make([]byte, n)
	copy(firstData, buf[:n])

	// Do NOT send ACK. Wait for retransmission (5s timeout + margin).
	if err := client.SetReadDeadline(time.Now().Add(7 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n2, _, err := client.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("retransmit: %v", err)
	}

	if n2 != n {
		t.Fatalf("retransmit size %d != original %d", n2, n)
	}
	if !bytes.Equal(buf[:n2], firstData) {
		t.Error("retransmitted packet differs from original")
	}

	// Now ACK to cleanly end the transfer
	ack := buildACKPacket(1)
	if _, err := client.WriteToUDP(ack, senderAddr); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: RFC 2347 option parsing extracts blksize and tsize from RRQ.
func TestTFTPParseRRQWithOptions(t *testing.T) {
	t.Parallel()

	pkt := buildRRQPacketWithOptions("ipxe.efi",
		"tsize", "0", "blksize", "1468", "windowsize", "4")

	filename, mode, opts, err := parseRRQ(pkt)
	if err != nil {
		t.Fatalf("parseRRQ: %v", err)
	}
	if filename != "ipxe.efi" {
		t.Errorf("filename = %q, want ipxe.efi", filename)
	}
	if mode != "octet" {
		t.Errorf("mode = %q, want octet", mode)
	}
	if opts.blksize != 1468 {
		t.Errorf("blksize = %d, want 1468", opts.blksize)
	}
	if !opts.tsize {
		t.Error("tsize = false, want true")
	}
	if !opts.windowsize {
		t.Error("windowsize = false, want true")
	}
}

// VALIDATES: parseRRQ with no options returns zero-value rrqOptions.
func TestTFTPParseRRQNoOptions(t *testing.T) {
	t.Parallel()

	pkt := buildRRQPacket("file.bin", "octet")
	_, _, opts, err := parseRRQ(pkt)
	if err != nil {
		t.Fatalf("parseRRQ: %v", err)
	}
	if opts.blksize != 0 {
		t.Errorf("blksize = %d, want 0", opts.blksize)
	}
	if opts.tsize {
		t.Error("tsize = true, want false")
	}
}

// VALIDATES: blksize out of RFC 2348 range [8, 65464] is ignored.
func TestTFTPParseRRQBlksizeOutOfRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		val  string
	}{
		{"too small", "4"},
		{"too large", "70000"},
		{"negative", "-1"},
		{"not a number", "abc"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pkt := buildRRQPacketWithOptions("f.bin", "blksize", tc.val)
			_, _, opts, err := parseRRQ(pkt)
			if err != nil {
				t.Fatalf("parseRRQ: %v", err)
			}
			if opts.blksize != 0 {
				t.Errorf("blksize = %d, want 0 (ignored)", opts.blksize)
			}
		})
	}
}

// VALIDATES: RFC 2347 option names are case-insensitive.
func TestTFTPParseRRQOptionsCaseInsensitive(t *testing.T) {
	t.Parallel()

	pkt := buildRRQPacketWithOptions("f.bin", "BLKSIZE", "1024", "TSIZE", "0")
	_, _, opts, err := parseRRQ(pkt)
	if err != nil {
		t.Fatalf("parseRRQ: %v", err)
	}
	if opts.blksize != 1024 {
		t.Errorf("blksize = %d, want 1024", opts.blksize)
	}
	if !opts.tsize {
		t.Error("tsize = false, want true")
	}
}

// VALIDATES: buildOACK produces correct wire format per RFC 2347.
func TestTFTPBuildOACK(t *testing.T) {
	t.Parallel()

	pkt := buildOACK([]string{"blksize", "1468", "tsize", "65536"})

	if binary.BigEndian.Uint16(pkt[0:2]) != opOACK {
		t.Fatalf("opcode = %d, want %d (OACK)", binary.BigEndian.Uint16(pkt[0:2]), opOACK)
	}

	// Parse the option pairs back out.
	payload := pkt[2:]
	var parts []string
	for len(payload) > 0 {
		nul := -1
		for i, b := range payload {
			if b == 0 {
				nul = i
				break
			}
		}
		if nul < 0 {
			break
		}
		parts = append(parts, string(payload[:nul]))
		payload = payload[nul+1:]
	}

	want := []string{"blksize", "1468", "tsize", "65536"}
	if len(parts) != len(want) {
		t.Fatalf("got %d parts, want %d", len(parts), len(want))
	}
	for i := range want {
		if parts[i] != want[i] {
			t.Errorf("part[%d] = %q, want %q", i, parts[i], want[i])
		}
	}
}

// VALIDATES: UEFI-style RRQ with tsize+blksize gets OACK then DATA with negotiated blksize.
func TestTFTPReadWithOACK(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	content := []byte("hello uefi pxe boot")
	if err := os.WriteFile(filepath.Join(rootDir, "ipxe.efi"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	srvAddr := startTestTFTPServer(t, rootDir, 10)

	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer closeUDP(t, client)

	rrq := buildRRQPacketWithOptions("ipxe.efi", "tsize", "0", "blksize", "1468")
	if _, err := client.WriteToUDP(rrq, srvAddr); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 2000)
	if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, senderAddr, err := client.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("no OACK response: %v", err)
	}

	if binary.BigEndian.Uint16(buf[0:2]) != opOACK {
		t.Fatalf("expected OACK (opcode %d), got opcode %d", opOACK, binary.BigEndian.Uint16(buf[0:2]))
	}

	// Verify OACK contains tsize with correct file size.
	oackPayload := string(buf[2:n])
	wantTsize := strconv.Itoa(len(content))
	if !strings.Contains(oackPayload, "tsize\x00"+wantTsize+"\x00") {
		t.Errorf("OACK missing tsize=%s, payload=%q", wantTsize, oackPayload)
	}
	if !strings.Contains(oackPayload, "blksize\x00") {
		t.Error("OACK missing blksize")
	}

	// RFC 2347: ACK block 0 to confirm OACK.
	ack := buildACKPacket(0)
	if _, err := client.WriteToUDP(ack, senderAddr); err != nil {
		t.Fatal(err)
	}

	// Now expect DATA block 1 with the file content.
	if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, _, err = client.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("no DATA after OACK: %v", err)
	}
	if binary.BigEndian.Uint16(buf[0:2]) != opDATA {
		t.Fatalf("expected DATA, got opcode %d", binary.BigEndian.Uint16(buf[0:2]))
	}
	if binary.BigEndian.Uint16(buf[2:4]) != 1 {
		t.Fatalf("block = %d, want 1", binary.BigEndian.Uint16(buf[2:4]))
	}
	if !bytes.Equal(buf[4:n], content) {
		t.Errorf("data = %q, want %q", buf[4:n], content)
	}
}

// VALIDATES: large file transfer with negotiated blksize uses the negotiated block size.
func TestTFTPReadLargeFileWithBlksize(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	content := make([]byte, 3000)
	for i := range content {
		content[i] = byte(i % 256)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "big.efi"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	srvAddr := startTestTFTPServer(t, rootDir, 10)

	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer closeUDP(t, client)

	rrq := buildRRQPacketWithOptions("big.efi", "blksize", "1468")
	if _, err := client.WriteToUDP(rrq, srvAddr); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 2000)

	// Expect OACK first.
	if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, senderAddr, err := client.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("no OACK: %v", err)
	}
	if binary.BigEndian.Uint16(buf[0:2]) != opOACK {
		t.Fatalf("expected OACK, got opcode %d", binary.BigEndian.Uint16(buf[0:2]))
	}

	// ACK block 0.
	if _, err := client.WriteToUDP(buildACKPacket(0), senderAddr); err != nil {
		t.Fatal(err)
	}

	// With blksize=1468: 3000 bytes = block 1 (1468) + block 2 (1468) + block 3 (64).
	var received []byte
	for block := uint16(1); block <= 3; block++ {
		if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatal(err)
		}
		n, addr, readErr := client.ReadFromUDP(buf)
		if readErr != nil {
			t.Fatalf("block %d: %v", block, readErr)
		}
		if binary.BigEndian.Uint16(buf[0:2]) != opDATA {
			t.Fatalf("block %d: expected DATA, got opcode %d", block, binary.BigEndian.Uint16(buf[0:2]))
		}
		if binary.BigEndian.Uint16(buf[2:4]) != block {
			t.Fatalf("expected block %d, got %d", block, binary.BigEndian.Uint16(buf[2:4]))
		}
		received = append(received, buf[4:n]...)

		if _, wErr := client.WriteToUDP(buildACKPacket(block), addr); wErr != nil {
			t.Fatal(wErr)
		}
	}

	if !bytes.Equal(received, content) {
		t.Errorf("received %d bytes, want %d", len(received), len(content))
	}
}

// VALIDATES: RFC 2348 zero-length final block when file is exact multiple of negotiated blksize.
func TestTFTPReadExactBlksizeMultiple(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	content := make([]byte, 1468)
	for i := range content {
		content[i] = byte(i % 256)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "exact-blk.efi"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	srvAddr := startTestTFTPServer(t, rootDir, 10)

	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer closeUDP(t, client)

	rrq := buildRRQPacketWithOptions("exact-blk.efi", "blksize", "1468")
	if _, err := client.WriteToUDP(rrq, srvAddr); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 2000)

	// OACK
	if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, senderAddr, err := client.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("no OACK: %v", err)
	}
	if binary.BigEndian.Uint16(buf[0:2]) != opOACK {
		t.Fatalf("expected OACK, got opcode %d", binary.BigEndian.Uint16(buf[0:2]))
	}
	if _, err := client.WriteToUDP(buildACKPacket(0), senderAddr); err != nil {
		t.Fatal(err)
	}

	// Block 1: 1468 bytes (full block)
	if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, addr, err := client.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("block 1: %v", err)
	}
	if n-4 != 1468 {
		t.Fatalf("block 1: data = %d bytes, want 1468", n-4)
	}
	if _, err := client.WriteToUDP(buildACKPacket(1), addr); err != nil {
		t.Fatal(err)
	}

	// Block 2: 0 bytes (signals end per RFC 2348)
	if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, _, err = client.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("block 2: %v", err)
	}
	if binary.BigEndian.Uint16(buf[0:2]) != opDATA {
		t.Fatalf("block 2: expected DATA, got opcode %d", binary.BigEndian.Uint16(buf[0:2]))
	}
	if binary.BigEndian.Uint16(buf[2:4]) != 2 {
		t.Fatalf("block number = %d, want 2", binary.BigEndian.Uint16(buf[2:4]))
	}
	if n-4 != 0 {
		t.Fatalf("block 2: data = %d bytes, want 0 (end signal)", n-4)
	}
}

// VALIDATES: plain RFC 1350 RRQ (no options) still works after option support added.
func TestTFTPReadPlainRRQStillWorks(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	content := []byte("plain rfc1350 transfer")
	if err := os.WriteFile(filepath.Join(rootDir, "plain.bin"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	srvAddr := startTestTFTPServer(t, rootDir, 10)

	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer closeUDP(t, client)

	rrq := buildRRQPacket("plain.bin", "octet")
	if _, err := client.WriteToUDP(rrq, srvAddr); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 516)
	if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, _, err := client.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("no DATA response: %v", err)
	}

	// Plain RRQ must get DATA directly (no OACK).
	if binary.BigEndian.Uint16(buf[0:2]) != opDATA {
		t.Fatalf("expected DATA (no OACK for plain RRQ), got opcode %d", binary.BigEndian.Uint16(buf[0:2]))
	}
	if binary.BigEndian.Uint16(buf[2:4]) != 1 {
		t.Fatalf("block = %d, want 1", binary.BigEndian.Uint16(buf[2:4]))
	}
	if !bytes.Equal(buf[4:n], content) {
		t.Errorf("data = %q, want %q", buf[4:n], content)
	}
}

func TestTFTPIOErrorMidTransfer(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	filePath := filepath.Join(rootDir, "vanish.bin")
	content := make([]byte, 1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	srvAddr := startTestTFTPServer(t, rootDir, 10)

	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer closeUDP(t, client)

	rrq := buildRRQPacket("vanish.bin", "octet")
	if _, err := client.WriteToUDP(rrq, srvAddr); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 516)
	if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, senderAddr, err := client.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("block 1: %v", err)
	}
	if binary.BigEndian.Uint16(buf[0:2]) != opDATA {
		t.Fatalf("expected DATA, got opcode %d", binary.BigEndian.Uint16(buf[0:2]))
	}

	// Truncate the file to 0 bytes while transfer is in progress
	if err := os.Truncate(filePath, 0); err != nil {
		t.Fatal(err)
	}

	// ACK block 1 to trigger read of block 2 (which should fail or return short)
	ack := buildACKPacket(1)
	if _, err := client.WriteToUDP(ack, senderAddr); err != nil {
		t.Fatal(err)
	}

	if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, _, err := client.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("block 2 response: %v", err)
	}

	opcode := binary.BigEndian.Uint16(buf[0:2])
	// Either an ERROR packet (read error) or a short DATA block (file truncated
	// between reads, Read returns 0 bytes which is < defaultBlockSize, ending transfer)
	// are acceptable. Both correctly handle the I/O anomaly.
	switch opcode {
	case opERROR:
		// Server detected I/O error
	case opDATA:
		dataLen := n - 4
		if dataLen >= defaultBlockSize {
			t.Errorf("expected short DATA block after truncation, got %d bytes", dataLen)
		}
	default:
		t.Fatalf("unexpected opcode %d", opcode)
	}
}

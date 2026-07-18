// VALIDATES: RFC 2347 (TFTP Option Extension) server-side negotiation -- the
// server's OACK carries only options the client requested AND the server
// supports; an option the server does not acknowledge is ignored as if it were
// never requested.
// PREVENTS: a server that volunteers unrequested options in the OACK (which a
// client would reject) or that fails/hangs on an option it does not support.
package tftpserver

import (
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// nextCString splits b at the first NUL, returning the string before it and the
// remainder after it. ok is false when b contains no NUL.
func nextCString(b []byte) (s string, rest []byte, ok bool) {
	for i, c := range b {
		if c == 0 {
			return string(b[:i]), b[i+1:], true
		}
	}
	return "", b, false
}

// readServerOACKOptions reads one datagram from the transfer connection, requires
// it to be an OACK (opcode 6), and returns its option key/value pairs (keys
// lower-cased). Fails the test if the first response is not an OACK.
func readServerOACKOptions(t *testing.T, client *net.UDPConn) map[string]string {
	t.Helper()
	buf := make([]byte, 516)
	if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, _, err := client.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("no OACK response: %v", err)
	}
	if n < 2 || binary.BigEndian.Uint16(buf[0:2]) != opOACK {
		t.Fatalf("first response opcode = %d, want OACK (%d)", binary.BigEndian.Uint16(buf[0:2]), opOACK)
	}
	opts := map[string]string{}
	rest := buf[2:n]
	for len(rest) > 0 {
		key, r1, ok := nextCString(rest)
		if !ok {
			break
		}
		val, r2, ok := nextCString(r1)
		if !ok {
			break
		}
		opts[strings.ToLower(key)] = val
		rest = r2
	}
	return opts
}

func tftpOptionTestServer(t *testing.T) (*net.UDPAddr, *net.UDPConn) {
	t.Helper()
	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "f.bin"), []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	srvAddr := startTestTFTPServer(t, rootDir, 10)
	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeUDP(t, client) })
	return srvAddr, client
}

// RFC requirement: RFC2347-x-1 positive -- the server's OACK includes an option
// the client requested and the server supports: an RRQ requesting blksize gets an
// OACK carrying blksize (RFC 2347 Negotiation Protocol; producer sendOACKAndWait
// assembles oackOpts only from client-requested opts, internal/plugins/tftpserver/handler.go).
// RFC requirement: RFC2347-x-1 negative -- the server MUST NOT include in the OACK
// any option the client did NOT request: requesting only blksize yields an OACK
// with no tsize, so the server never volunteers an unrequested option.
func TestRFC2347ServerOACKOnlyRequestedOptions(t *testing.T) {
	t.Parallel()
	srvAddr, client := tftpOptionTestServer(t)

	rrq := buildRRQPacketWithOptions("f.bin", "blksize", "1200")
	if _, err := client.WriteToUDP(rrq, srvAddr); err != nil {
		t.Fatal(err)
	}

	opts := readServerOACKOptions(t, client)
	if _, ok := opts["blksize"]; !ok {
		t.Errorf("OACK is missing the requested blksize option: %v", opts)
	}
	if _, ok := opts["tsize"]; ok {
		t.Errorf("OACK includes tsize, which the client never requested: %v", opts)
	}
}

// RFC requirement: RFC2347-x-3 positive -- an option the server does not
// acknowledge is ignored as if never requested: an RRQ requesting the unsupported
// windowsize option gets an OACK that omits windowsize entirely, and the server
// falls back to lockstep (internal/plugins/tftpserver/handler.go:276) instead of
// erroring.
// RFC requirement: RFC2347-x-3 negative -- the ignore is specific to unacknowledged
// options: a requested AND supported option (blksize) in the same RRQ IS
// acknowledged in the OACK, so the server does not blanket-drop every option.
func TestRFC2347ServerIgnoresUnacknowledgedOption(t *testing.T) {
	t.Parallel()
	srvAddr, client := tftpOptionTestServer(t)

	rrq := buildRRQPacketWithOptions("f.bin", "blksize", "1200", "windowsize", "4")
	if _, err := client.WriteToUDP(rrq, srvAddr); err != nil {
		t.Fatal(err)
	}

	opts := readServerOACKOptions(t, client)
	if _, ok := opts["windowsize"]; ok {
		t.Errorf("OACK acknowledges windowsize, which the server does not support: %v", opts)
	}
	if _, ok := opts["blksize"]; !ok {
		t.Errorf("OACK dropped the supported blksize option: %v", opts)
	}
}

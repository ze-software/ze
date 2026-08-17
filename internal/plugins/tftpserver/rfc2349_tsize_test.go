// VALIDATES: RFC 2349 (TFTP Transfer Size option) server side -- a tsize option
// requested in an RRQ with the placeholder value "0" is answered in the OACK with
// the ACTUAL file size, not the client's placeholder.
// PREVENTS: a server that echoes the client's "0" back (leaving the client blind
// to the transfer size) or that reports a constant/wrong size.
package tftpserver

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

// tftpServeFile starts a read-only TFTP server exposing a single file with the
// given content and returns the server address plus a bound client socket.
func tftpServeFile(t *testing.T, name string, content []byte) (*net.UDPAddr, *net.UDPConn) {
	t.Helper()
	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, name), content, 0o644); err != nil {
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

// RFC requirement: RFC2349-x-3 positive -- when a client requests the tsize option
// in an RRQ with the value "0" (a size query), the server returns the ACTUAL file
// size in the OACK (RFC 2349 Transfer Size Option Specification; producer handleRRQ
// os.Stat + oackOpts tsize, internal/plugins/tftpserver/handler.go:319-329). A
// 7-byte file yields tsize "7".
// RFC requirement: RFC2349-x-3 negative -- the server does NOT echo the client's
// placeholder "0": a different-sized (20-byte) file yields tsize "20", i.e. the
// real size, never "0" and never a constant.
func TestRFC2349TsizeRRQReturnsActualSize(t *testing.T) {
	t.Parallel()

	t.Run("7-byte file returns 7", func(t *testing.T) {
		t.Parallel()
		srv, client := tftpServeFile(t, "a.bin", []byte("payload")) // exactly 7 bytes
		rrq := buildRRQPacketWithOptions("a.bin", "tsize", "0")
		if _, err := client.WriteToUDP(rrq, srv); err != nil {
			t.Fatal(err)
		}
		opts := readServerOACKOptions(t, client)
		if opts["tsize"] != "7" {
			t.Errorf("tsize = %q, want 7 (actual file size, not the client placeholder 0)", opts["tsize"])
		}
	})

	t.Run("20-byte file returns 20, never the placeholder 0", func(t *testing.T) {
		t.Parallel()
		srv, client := tftpServeFile(t, "b.bin", make([]byte, 20))
		rrq := buildRRQPacketWithOptions("b.bin", "tsize", "0")
		if _, err := client.WriteToUDP(rrq, srv); err != nil {
			t.Fatal(err)
		}
		opts := readServerOACKOptions(t, client)
		if opts["tsize"] == "0" {
			t.Error("server echoed the client placeholder 0 instead of the actual file size")
		}
		if opts["tsize"] != "20" {
			t.Errorf("tsize = %q, want 20 (actual file size)", opts["tsize"])
		}
	})
}

//go:build docker && linux

// Design: docs/architecture/testing/interop.md -- a package test's own server container (labtest)
// Related: ../../../test/interop-tftp/Dockerfile.dnsmasq -- the dnsmasq 2.91 image
// Related: tftp.go -- the client under test
//
// Ze's TFTP client against dnsmasq's TFTP server, an implementation Ze does
// not share code with. dnsmasq runs on the host network so its reply from a
// fresh transfer port reaches the client, as it would on a LAN.
package fetch

import (
	"bytes"
	"crypto/sha256"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/core/tftp"
	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/interoplab/labtest"
)

// TestTFTPClientDnsmasqInterop fetches three files from dnsmasq: a 168 894
// octet file of 116 blocks, a file of exactly three 1468-octet blocks (which
// ends on an empty block), and an empty file. Each transfer must negotiate
// blksize 1468 and tsize equal to the file size, fall back to a window of one
// block because dnsmasq does not acknowledge windowsize, and deliver the exact
// bytes. The last fetch runs through the scheme table with a SHA-256.
//
// MUTATION: make tftp.ParseOACK return Unnegotiated() without reading the
// OACK. The client then reads dnsmasq's 1468-octet blocks as oversized
// against 512 and refuses block 1 with ERROR 4, so the first transfer fails.
func TestTFTPClientDnsmasqInterop(t *testing.T) {
	server := labtest.Start(t, labtest.Spec{
		Name:             "tftp-dnsmasq",
		Image:            "ze-tftp-dnsmasq:interop",
		Dockerfile:       "test/interop-tftp/Dockerfile.dnsmasq",
		Port:             interoplab.UDPPort(69),
		HostNetwork:      true,
		ReadyInContainer: []string{"sh", "-c", "grep -q ':0045 ' /proc/net/udp"},
		ReadyTimeout:     60 * time.Second,
		LogLines:         50,
	})

	var lines strings.Builder
	for i := 1; i <= 30000; i++ {
		lines.WriteString(strconv.Itoa(i))
		lines.WriteByte('\n')
	}
	files := []struct {
		name    string
		content []byte
	}{
		{"seq.txt", []byte(lines.String())},
		{"exact.bin", bytes.Repeat([]byte("z"), 3*tftp.BlksizeEthernet)},
		{"empty.bin", nil},
	}
	serverAddr := &net.UDPAddr{IP: server.Addr().AsSlice(), Port: int(server.Port())}
	for _, file := range files {
		t.Run(file.name, func(t *testing.T) {
			conn, err := net.ListenUDP("udp4", nil)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close() //nolint:errcheck // test cleanup
			var out bytes.Buffer
			transfer := newTFTPTransfer(conn, serverAddr, &out)
			if err := transfer.run(file.name); err != nil {
				t.Fatalf("fetch %s from dnsmasq: %v", file.name, err)
			}
			if !bytes.Equal(out.Bytes(), file.content) {
				t.Fatalf("fetched %d octets, want %d", out.Len(), len(file.content))
			}
			granted := transfer.granted
			if granted.Blksize != tftp.BlksizeEthernet {
				t.Errorf("negotiated blksize %d, want %d", granted.Blksize, tftp.BlksizeEthernet)
			}
			if !granted.HasTsize || granted.Tsize != int64(len(file.content)) {
				t.Errorf("negotiated tsize %d (acknowledged %v), want %d", granted.Tsize, granted.HasTsize, len(file.content))
			}
			if granted.Windowsize != 1 {
				t.Errorf("window %d, want 1: dnsmasq acknowledges no windowsize", granted.Windowsize)
			}
		})
	}

	sum := sha256.Sum256(files[0].content)
	source := "tftp://" + server.String() + "/seq.txt"
	fetcher, err := Resolve(source)
	if err != nil {
		t.Fatalf("Resolve(%q): %v", source, err)
	}
	dest := filepath.Join(t.TempDir(), "seq.txt")
	if err := fetcher(source, dest, textbuf.StringHex(sum[:])); err != nil {
		t.Fatalf("fetch %s: %v", source, err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, files[0].content) {
		t.Fatalf("fetched %d octets through the scheme table, want %d", len(got), len(files[0].content))
	}
}

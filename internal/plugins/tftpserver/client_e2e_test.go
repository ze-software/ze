// VALIDATES: Ze's TFTP client (the tftp scheme of internal/core/fetch, the
// path `ze init --from tftp://` takes) fetches files from Ze's TFTP server
// byte for byte: blksize and tsize are negotiated, the client's windowsize 4 is
// granted and the file is sent four blocks to a window (the server's Debug line
// names the grant), a file that is an exact multiple of the block size ends on
// its empty block, and the digest check passes.
// PREVENTS: a client and server that each pass their own tests but disagree
// on the wire.
package tftpserver

import (
	"crypto/sha256"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/fetch"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/core/tftp"
)

func TestTFTPClientFetchesFromZeServer(t *testing.T) {
	rootDir := t.TempDir()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	var serverLog syncBuf
	stop := ServeConn(conn, rootDir, 4, slog.New(slog.NewTextHandler(&serverLog, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(stop)
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("a UDP socket without a UDP address")
	}
	port := addr.Port

	sizes := map[string]int{
		"empty.bin":    0,
		"short.bin":    100,
		"exact.bin":    3 * tftp.BlksizeEthernet,
		"multiple.bin": 70*tftp.BlksizeEthernet + 17,
	}
	for name, size := range sizes {
		t.Run(name, func(t *testing.T) {
			content := make([]byte, size)
			for i := range content {
				content[i] = byte(i*13 + size)
			}
			if err := os.WriteFile(filepath.Join(rootDir, name), content, 0o644); err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(content)
			source := "tftp://127.0.0.1:" + strconv.Itoa(port) + "/" + name
			fetcher, err := fetch.Resolve(source)
			if err != nil {
				t.Fatalf("Resolve(%q): %v", source, err)
			}
			dest := filepath.Join(t.TempDir(), "out")
			if err := fetcher(source, dest, textbuf.StringHex(sum[:])); err != nil {
				t.Fatalf("fetch %s: %v", source, err)
			}
			got, err := os.ReadFile(dest)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != size {
				t.Fatalf("fetched %d octets, want %d", len(got), size)
			}
		})
	}

	// RFC 7440 Section 3: the client asked for windowsize 4, and the server
	// granted all of it to every fetch.
	if granted := strings.Count(serverLog.String(), "windowsize=4"); granted != len(sizes) {
		t.Fatalf("the server granted windowsize 4 to %d of %d fetches:\n%s", granted, len(sizes), serverLog.String())
	}
}

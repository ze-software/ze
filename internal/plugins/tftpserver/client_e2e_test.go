// VALIDATES: Ze's TFTP client (the tftp scheme of internal/core/fetch, the
// path `ze init --from tftp://` takes) fetches files from Ze's TFTP server
// byte for byte: blksize and tsize are negotiated, the unsupported windowsize
// falls back to lockstep, a file that is an exact multiple of the block size
// ends on its empty block, and the digest check passes.
// PREVENTS: a client and server that each pass their own tests but disagree
// on the wire.
package tftpserver

import (
	"crypto/sha256"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/core/fetch"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/core/tftp"
)

func TestTFTPClientFetchesFromZeServer(t *testing.T) {
	rootDir := t.TempDir()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	stop := ServeConn(conn, rootDir, 4, slogutil.DiscardLogger())
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

}

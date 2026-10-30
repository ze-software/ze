// VALIDATES: the tftp scheme's URL reading (RFC 3617 form, an optional port,
// octet mode only) and the fetch through the scheme table with its digest
// check and retry.
// PREVENTS: a netascii or malformed URL reaching the wire, or a tftp fetch
// that skips the SHA-256 the operator gave.
package fetch

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
)

func TestTFTPURLParse(t *testing.T) {
	accepted := []struct {
		source string
		want   tftpTarget
	}{
		{"tftp://host/f.bin", tftpTarget{host: "host", port: 69, file: "f.bin"}},
		{"TFTP://host:6969/dir/f.bin;mode=octet", tftpTarget{host: "host", port: 6969, file: "dir/f.bin"}},
		{"tftp://host/f.bin;MODE=OCTET", tftpTarget{host: "host", port: 69, file: "f.bin"}},
		{"tftp://[::1]:1/f", tftpTarget{host: "::1", port: 1, file: "f"}},
		{"tftp://10.0.0.1:65535/a%20b", tftpTarget{host: "10.0.0.1", port: 65535, file: "a b"}},
	}
	for _, tc := range accepted {
		got, err := parseTFTPURL(tc.source)
		if err != nil {
			t.Errorf("parseTFTPURL(%q): %v", tc.source, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseTFTPURL(%q) = %+v, want %+v", tc.source, got, tc.want)
		}
	}

	refused := map[string]string{
		"tftp://host/f;mode=netascii": "octet mode only",
		"tftp://host/":                "names no file",
		"tftp:///f":                   "names no server",
		"tftp://u:p@host/f":           "no user name",
		"tftp://host:0/f":             "port 0",
		"tftp://host:65536/f":         "not a number from 1 to 65535",
		"tftp://host/f?x=1":           "no query",
		"tftp://host/f#x":             "no fragment",
	}
	for source, want := range refused {
		_, err := parseTFTPURL(source)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("parseTFTPURL(%q) = %v, want an error naming %q", source, err, want)
		}
	}
}

// TestTFTPFetchThroughSchemeTable resolves a tftp URL through the scheme
// table and fetches it with its digest, then refuses a wrong digest.
func TestTFTPFetchThroughSchemeTable(t *testing.T) {
	shortenTFTP(t)

	content := []byte("database bytes")
	sum := sha256.Sum256(content)
	digest := textbuf.StringHex(sum[:])
	target, wait := startTFTPScript(t, func(p *tftpPeer) {
		p.data(1, content)
		p.expectACK(1)
	})
	source := "tftp://127.0.0.1:" + strconv.Itoa(int(target.port)) + "/f.bin"
	fetcher, err := Resolve(source)
	if err != nil {
		t.Fatalf("Resolve(%q): %v", source, err)
	}
	dest := filepath.Join(t.TempDir(), "out")
	if err := fetcher(source, dest, digest); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	wait()
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("fetched %q", got)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o, want 0600", info.Mode().Perm())
	}

	target, wait = startTFTPScript(t, func(p *tftpPeer) {
		p.data(1, content)
		p.expectACK(1)
	})
	source = "tftp://127.0.0.1:" + strconv.Itoa(int(target.port)) + "/f.bin"
	err = tftpToFile(source, dest, strings.Repeat("0", 64))
	wait()
	if err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("a wrong digest was not refused: %v", err)
	}
}

// TestTFTPFetchRefusesBadURLWithoutRetry: a URL that does not parse fails at
// once, naming it, with no packet sent.
func TestTFTPFetchRefusesBadURLWithoutRetry(t *testing.T) {
	err := tftpToFile("tftp://host/f;mode=netascii", filepath.Join(t.TempDir(), "out"), "")
	if err == nil || !strings.Contains(err.Error(), "tftp://host/f;mode=netascii") || strings.Contains(err.Error(), "attempts") {
		t.Fatalf("netascii URL: %v", err)
	}
}

// TestTFTPFetchRefusalRedactsPassword: a refused URL that carries a password
// is named without it, as every fetch error names its URL (redact.URL). The
// password is the operator's secret, and an installer console is read over a
// serial line and photographed.
func TestTFTPFetchRefusalRedactsPassword(t *testing.T) {
	err := tftpToFile("tftp://admin:hunter2@host/f", filepath.Join(t.TempDir(), "out"), "")
	if err == nil {
		t.Fatal("a tftp URL with a password was accepted")
	}
	if strings.Contains(err.Error(), "hunter2") {
		t.Fatalf("the refusal names the password: %v", err)
	}
	if !strings.Contains(err.Error(), "no user name or password") {
		t.Fatalf("the refusal does not say why: %v", err)
	}
}

// TestTFTPClientReacknowledgesDuplicateBlock: in the middle of a transfer the
// server sends block 1 again, as it does when the ACK of block 1 was lost.
// The client acknowledges block 1 once more, writes nothing twice, and the
// transfer completes with the file intact (AC-16).
func TestTFTPClientReacknowledgesDuplicateBlock(t *testing.T) {
	shortenTFTP(t)

	target, wait := startTFTPScript(t, func(p *tftpPeer) {
		p.data(1, block(1, 512))
		if !p.expectACK(1) {
			return
		}
		p.data(1, block(1, 512))
		if !p.expectACK(1) {
			return
		}
		p.data(2, block(2, 5))
		p.expectACK(2)
	})
	got, err := fetchTFTP(t, target)
	wait()
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	want := append(block(1, 512), block(2, 5)...)
	if !bytes.Equal(got, want) {
		t.Fatalf("fetched %d bytes, want %d: the duplicate block was written again or lost", len(got), len(want))
	}
}

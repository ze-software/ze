// Design: docs/architecture/appliance/on-device-installer.md -- download with retry and integrity check
// Related: scheme.go -- IsRemote, Resolve, Schemes

package fetch

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// VALIDATES: AC-10, the file fetch `ze init --from` uses: a matching digest in
// either letter case passes, a mismatch is refused naming both digests, and a
// digest that is not exactly 64 hex digits is refused before any request.
// PREVENTS: a fetched blob imported with the wrong bytes, or a mistyped digest
// that reaches the network and fails far from its cause.
func TestFetchSHA256(t *testing.T) {
	body := []byte("zefs-blob-bytes")
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Write(body) //nolint:errcheck // test
	}))
	defer srv.Close()
	good := fmt.Sprintf("%x", sha256.Sum256(body))

	dest := filepath.Join(t.TempDir(), "database.zefs")
	for _, digest := range []string{good, strings.ToUpper(good), ""} {
		if err := ToFile(srv.URL, dest, digest); err != nil {
			t.Fatalf("ToFile(sha256 %q): %v", digest, err)
		}
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("fetched file mode = %v, want 0600", info.Mode().Perm())
	}

	wrong := strings.Repeat("0", sha256HexLength)
	err = ToFile(srv.URL, dest, wrong)
	if err == nil || !strings.Contains(err.Error(), "sha256 mismatch") || !strings.Contains(err.Error(), good) {
		t.Fatalf("mismatch not refused naming the digest: %v", err)
	}

	before := requests.Load()
	for _, digest := range []string{good[:63], good + "0", strings.Repeat("g", 64)} {
		if err := ToFile(srv.URL, dest, digest); err == nil {
			t.Fatalf("ToFile accepted malformed sha256 %q", digest)
		}
	}
	if requests.Load() != before {
		t.Fatalf("a malformed sha256 reached the network: %d requests", requests.Load()-before)
	}
	if err := CheckSHA256(dest, good[:63]); err == nil {
		t.Fatal("CheckSHA256 accepted a 63-digit digest")
	}
}

// VALIDATES: AC-10, AC-11: http and https resolve through the one table, any
// other scheme is refused naming it and the supported list, and a source
// without "<scheme>://" is a local path.
// PREVENTS: a second scheme list drifting from the table, or a local path
// with "://" inside it read as a URL.
func TestFetchSchemeTable(t *testing.T) {
	if got := strings.Join(Schemes(), ","); got != "http,https" {
		t.Fatalf("Schemes() = %s, want http,https", got)
	}
	for _, source := range []string{"http://host/x.zefs", "https://host/x.zefs", "HTTPS://host/x.zefs"} {
		if !IsRemote(source) {
			t.Fatalf("IsRemote(%q) = false", source)
		}
		if _, err := Resolve(source); err != nil {
			t.Fatalf("Resolve(%q): %v", source, err)
		}
	}
	for _, source := range []string{"database.zefs", "./a://b", "/srv/x.zefs", "://x"} {
		if IsRemote(source) {
			t.Fatalf("IsRemote(%q) = true, want a local path", source)
		}
	}
	_, err := Resolve("ftp://host/x.zefs")
	if err == nil || !strings.Contains(err.Error(), `"ftp"`) || !strings.Contains(err.Error(), "http, https") {
		t.Fatalf("ftp not refused naming the scheme and the list: %v", err)
	}
}

// VALIDATES: the Security Review row "the fetch helper follows no redirect to
// another host": a same-host redirect is followed, another host is refused.
// PREVENTS: a provisioning server redirecting ze init to bytes from a server
// the operator never named.
func TestFetchRedirectOtherHost(t *testing.T) {
	shortRetryDelay(t)
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("foreign")) //nolint:errcheck // test
	}))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/same":
			http.Redirect(w, r, "/blob", http.StatusFound)
		case "/other":
			http.Redirect(w, r, other.URL+"/blob", http.StatusFound)
		case "/missing":
			http.NotFound(w, r)
		default:
			w.Write([]byte("local")) //nolint:errcheck // test
		}
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "blob")
	if err := ToFile(srv.URL+"/same", dest, ""); err != nil {
		t.Fatalf("same-host redirect: %v", err)
	}
	err := ToFile(srv.URL+"/other", dest, "")
	if err == nil {
		t.Fatal("redirect to another host was followed")
	}
	// The operator of `ze init --from` sees only this error, never the retry
	// log lines, so the error itself names why the fetch failed.
	if !strings.Contains(err.Error(), "it leaves host") {
		t.Fatalf("refusal does not name the reason: %v", err)
	}
	err = ToFile(srv.URL+"/missing", dest, "")
	if err == nil || !strings.Contains(err.Error(), "status 404") {
		t.Fatalf("a 404 is not named in the error: %v", err)
	}
}

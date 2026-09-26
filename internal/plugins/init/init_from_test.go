// Design: docs/architecture/storage-backends.md -- ze init --from imports a blob as it is.
// Related: main.go -- runImport, runFetchImport, importSource

package init

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

// initFromValues is the whole store a seed or a backup carries: credentials,
// identity and a config file.
var initFromValues = map[string]string{
	zefs.KeyLocalAdminUsername.Pattern: "admin",
	zefs.KeyLocalAdminPassword.Pattern: "$2a$10$seedhash",
	zefs.KeyInstanceName.Pattern:       "seeded-router",
	zefs.KeyFileActive.Key("ze.conf"):  "bgp { router-id 10.0.0.1; }\n",
}

// writeSeed writes initFromValues to an exact-fit blob, as the appliance seed
// builders do, and returns its bytes.
func writeSeed(t *testing.T, path string) []byte {
	t.Helper()
	blob, err := storage.CreateBlobPopulated(path, func(seed storage.Storage) error {
		for key, value := range initFromValues {
			if err := seed.WriteKey(key, []byte(value)); err != nil {
				return err
			}
		}
		return nil
	}, false, zefs.Spare(0))
	if err != nil {
		t.Fatalf("CreateBlobPopulated: %v", err)
	}
	if err := blob.Close(); err != nil {
		t.Fatalf("Close seed: %v", err)
	}
	data, err := os.ReadFile(path) //nolint:gosec // test path
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// captureStderr runs fn with os.Stderr redirected and returns what it wrote.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stderr
	os.Stderr = w
	done := make(chan []byte, 1)
	go func() { //nolint:gocritic // test-only pipe drain, ends when the writer closes
		out, _ := io.ReadAll(r)
		done <- out
	}()
	fn()
	os.Stderr = saved
	w.Close() //nolint:errcheck // pipe writer
	return string(<-done)
}

// requireStore checks dir holds a live tree carrying every initFromValues key.
func requireStore(t *testing.T, dir string) {
	t.Helper()
	store, err := storage.OpenReadOnly(dir)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup
	for key, value := range initFromValues {
		assertStoreFile(t, store, key, value)
	}
}

// requireNoStore checks dir holds no tree and no fetch staging folder.
func requireNoStore(t *testing.T, dir string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(dir, "database")); err == nil {
		t.Fatal("a refused import published database/")
	}
	requireNoStaging(t, dir)
}

func requireNoStaging(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), fetchStagePrefix) {
			t.Fatalf("fetched copy left behind: %s", entry.Name())
		}
	}
}

// archiveCount counts the retired names of path; the lock sidecar the
// import moves with each one is not a retired blob.
func archiveCount(t *testing.T, path string) int {
	t.Helper()
	matches, err := filepath.Glob(path + ".replaced-*")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, match := range matches {
		if !strings.HasSuffix(match, ".lock") {
			count++
		}
	}
	return count
}

// VALIDATES: AC-9, A-7: an appliance seed imports through the local-path form
// after the scheme table exists: the tree holds its keys and the source is
// retired as .replaced-<stamp>.
// PREVENTS: the URL work changing storage-1's local import.
func TestInitFromSeed(t *testing.T) {
	source := filepath.Join(t.TempDir(), "database.zefs")
	writeSeed(t, source)
	dir := t.TempDir()
	if code := runImport(source, "", dir, false); code != 0 {
		t.Fatalf("runImport = %d", code)
	}
	requireStore(t, dir)
	if _, err := os.Lstat(source); err == nil {
		t.Fatal("the local source was not retired")
	}
	if n := archiveCount(t, source); n != 1 {
		t.Fatalf("retired names = %d, want 1", n)
	}
}

// VALIDATES: A-7: a `ze data backup` artifact imports exactly as a seed does.
// PREVENTS: the source's origin mattering to ze init --from.
func TestInitFromBackup(t *testing.T) {
	live := t.TempDir()
	store, err := storage.Create(live)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	for key, value := range initFromValues {
		if err := store.WriteKey(key, []byte(value)); err != nil {
			t.Fatalf("WriteKey: %v", err)
		}
	}
	backup := filepath.Join(t.TempDir(), "backup.zefs")
	if _, err := storage.Backup(store, backup, false); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if code := runImport(backup, "", dir, false); code != 0 {
		t.Fatalf("runImport = %d", code)
	}
	requireStore(t, dir)
}

// VALIDATES: AC-11: a new import beside an existing store, local or fetched,
// is refused naming the store, and the fetched copy is removed.
// PREVENTS: a URL import replacing a live store without --force.
func TestInitFromRefusesExisting(t *testing.T) {
	blob := writeSeed(t, filepath.Join(t.TempDir(), "database.zefs"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(blob) //nolint:errcheck // test
	}))
	defer srv.Close()

	dir := t.TempDir()
	existing, err := storage.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := existing.Close(); err != nil {
		t.Fatal(err)
	}
	// The storage layer names the store by its resolved path (/var is a
	// symbolic link on macOS).
	named, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(t.TempDir(), "database.zefs")
	writeSeed(t, local)
	for _, source := range []string{local, srv.URL + "/database.zefs"} {
		var code int
		out := captureStderr(t, func() { code = runImport(source, "", dir, false) })
		if code != 1 || !strings.Contains(out, "database already exists: "+named) {
			t.Fatalf("runImport(%s) = %d, stderr %q: want the existing store named", source, code, out)
		}
		requireNoStaging(t, dir)
	}
	if _, err := os.Lstat(local); err != nil {
		t.Fatalf("a refused import moved the local source: %v", err)
	}
}

// VALIDATES: AC-11 and the sha256 boundary: an unsupported scheme is refused
// naming it and the supported list, a 63- or 65-digit digest and a digest
// without --from are refused, all before a store location is read.
// PREVENTS: a scheme typo read as a local path, or a digest silently ignored.
func TestInitFromScheme(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--from", "ftp://host/database.zefs"}, `unsupported scheme "ftp": supported http, https`},
		{[]string{"--from", "database.zefs", "--sha256", strings.Repeat("a", 63)}, "want 64 hexadecimal digits, got 63"},
		{[]string{"--from", "database.zefs", "--sha256", strings.Repeat("a", 65)}, "want 64 hexadecimal digits, got 65"},
		{[]string{"--sha256", strings.Repeat("a", 64)}, "--sha256 checks the --from source and needs --from"},
	}
	for _, tc := range cases {
		var code int
		out := captureStderr(t, func() { code = Run(tc.args) })
		if code != 1 || !strings.Contains(out, tc.want) {
			t.Fatalf("Run(%v) = %d, stderr %q, want %q", tc.args, code, out, tc.want)
		}
	}
}

// VALIDATES: AC-10: a URL source is fetched, its digest checked, the blob
// checked and imported, and the fetched copy and its retired name are both
// removed; without --sha256 the digest step is skipped.
// PREVENTS: a provisioning fetch that leaves a copy of the credentials behind.
func TestInitFromURL(t *testing.T) {
	blob := writeSeed(t, filepath.Join(t.TempDir(), "database.zefs"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(blob) //nolint:errcheck // test
	}))
	defer srv.Close()
	digest := fmt.Sprintf("%x", sha256.Sum256(blob))

	for _, expected := range []string{digest, ""} {
		dir := t.TempDir()
		if code := runImport(srv.URL+"/install/database.zefs", expected, dir, false); code != 0 {
			t.Fatalf("runImport(sha256 %q) = %d", expected, code)
		}
		requireStore(t, dir)
		requireNoStaging(t, dir)
	}
}

// VALIDATES: AC-10: a digest mismatch, a body that is not a blob, and a blob
// with a corrupt entry each import nothing, remove the fetched copy, and name
// the reason; the corrupt entry is named by its key.
// PREVENTS: a truncated or substituted download becoming the live store.
func TestInitFromURLRefused(t *testing.T) {
	blob := writeSeed(t, filepath.Join(t.TempDir(), "database.zefs"))
	corrupt := bytes.Clone(blob)
	offset := bytes.Index(corrupt, []byte("seeded-router"))
	if offset < 0 {
		t.Fatal("seed value not found in the blob")
	}
	corrupt[offset] ^= 1
	bodies := map[string][]byte{"/blob": blob, "/junk": []byte("not a blob"), "/corrupt": corrupt}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(bodies[r.URL.Path]) //nolint:errcheck // test
	}))
	defer srv.Close()

	cases := []struct {
		path, digest, want string
	}{
		{"/blob", strings.Repeat("0", 64), "sha256 mismatch"},
		{"/junk", "", "invalid magic"},
		{"/corrupt", "", zefs.KeyInstanceName.Pattern},
	}
	for _, tc := range cases {
		dir := t.TempDir()
		var code int
		out := captureStderr(t, func() { code = runImport(srv.URL+tc.path, tc.digest, dir, false) })
		if code != 1 || !strings.Contains(out, tc.want) {
			t.Fatalf("runImport(%s) = %d, stderr %q, want %q", tc.path, code, out, tc.want)
		}
		requireNoStore(t, dir)
	}
}

// VALIDATES: --sha256 pins a local source too: a mismatch imports nothing and
// leaves the source in place.
// PREVENTS: the digest flag silently ignored for the local form.
func TestInitFromPathSHA256(t *testing.T) {
	source := filepath.Join(t.TempDir(), "database.zefs")
	blob := writeSeed(t, source)
	dir := t.TempDir()
	var code int
	out := captureStderr(t, func() { code = runImport(source, strings.Repeat("0", 64), dir, false) })
	if code != 1 || !strings.Contains(out, "sha256 mismatch") {
		t.Fatalf("runImport = %d, stderr %q", code, out)
	}
	requireNoStore(t, dir)
	if _, err := os.Lstat(source); err != nil {
		t.Fatalf("a refused import moved the source: %v", err)
	}
	if code := runImport(source, fmt.Sprintf("%X", sha256.Sum256(blob)), dir, false); code != 0 {
		t.Fatalf("runImport with the matching digest = %d", code)
	}
	requireStore(t, dir)
}

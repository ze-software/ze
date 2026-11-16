package appliance

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
)

func firmwareDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// VALIDATES: every firmware blob the hardware-kms profile compiles into the
// kernel is pinned to a linux-firmware commit and a SHA-256, and is fetched at
// that commit, as the kernel tarball is pinned (TestKernelVersionHasDigestPin).
// PREVENTS: a blob added with no pin, or fetched from the moving head of
// linux-firmware, so two build hosts compile different firmware from one tree.
func TestHardwareKMSFirmwareIsPinned(t *testing.T) {
	blobs := kernelbuilder.HardwareKMSFirmware()
	if len(blobs) == 0 {
		t.Fatal("the hardware-kms profile pins no firmware, and it compiles the i915 firmware in")
	}
	for _, blob := range blobs {
		if blob.Path == "" || strings.HasPrefix(blob.Path, "/") || strings.Contains(blob.Path, "..") {
			t.Errorf("firmware path %q is not a path inside linux-firmware", blob.Path)
		}
		// Lower case, because the digests compared against these come from
		// hex.EncodeToString.
		if decoded, err := hex.DecodeString(blob.Commit); err != nil || len(decoded) != 20 || blob.Commit != strings.ToLower(blob.Commit) {
			t.Errorf("firmware %s commit %q is not a 40-character lower-case git commit id", blob.Path, blob.Commit)
		}
		if decoded, err := hex.DecodeString(blob.SHA256); err != nil || len(decoded) != sha256.Size || blob.SHA256 != strings.ToLower(blob.SHA256) {
			t.Errorf("firmware %s digest %q is not a lower-case SHA-256", blob.Path, blob.SHA256)
		}
		want := "https://git.kernel.org/pub/scm/linux/kernel/git/firmware/linux-firmware.git/plain/" + blob.Path + "?id=" + blob.Commit
		if got := blob.URL(); got != want {
			t.Errorf("firmware %s is fetched from %s, want its pinned commit %s", blob.Path, got, want)
		}
	}
}

// VALIDATES: a firmware download whose bytes do not match the pin is refused,
// naming the URL, both digests and the fetch to compare by hand, and nothing
// lands in the cache directory, neither the blob nor the temporary file.
// PREVENTS: a mirror, a proxy or an attacker on the path choosing the
// firmware compiled into the kernel.
// Method: an httptest server serving other bytes, through downloadFirmwareBlob.
func TestFirmwareDownloadRefusesWrongBytes(t *testing.T) {
	served := []byte("not the pinned firmware")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(served) //nolint:errcheck,gosec // test server
	}))
	defer srv.Close()
	blob := kernelbuilder.FirmwareBlob{Path: "i915/blob.bin", Commit: strings.Repeat("a", 40), SHA256: firmwareDigest([]byte("the pinned firmware"))}
	dest := filepath.Join(t.TempDir(), "i915", "blob.bin")
	url := srv.URL + "/i915/blob.bin?id=" + blob.Commit

	err := downloadFirmwareBlob(url, dest, blob)
	if err == nil {
		t.Fatal("a firmware download whose SHA-256 differs from its pin was cached")
	}
	for _, want := range []string{url, firmwareDigest(served), blob.SHA256, blob.Commit, "was not cached", "curl -fL " + shellArg(url) + " | sha256sum"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q:\n%v", want, err)
		}
	}
	if _, statErr := os.Stat(dest); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("the refused blob is at %s (stat: %v)", dest, statErr)
	}
	entries, readErr := os.ReadDir(filepath.Dir(dest))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Errorf("the refused download left %d files in the cache directory: %v", len(entries), entries)
	}
}

// VALIDATES: a firmware download whose bytes match the pin is moved into the
// cache, with no temporary file left beside it.
// PREVENTS: a digest check that refuses every download, which the refusal
// test alone would not notice.
func TestFirmwareDownloadCachesPinnedBytes(t *testing.T) {
	pinned := []byte("the pinned firmware")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(pinned) //nolint:errcheck,gosec // test server
	}))
	defer srv.Close()
	blob := kernelbuilder.FirmwareBlob{Path: "i915/blob.bin", Commit: strings.Repeat("a", 40), SHA256: firmwareDigest(pinned)}
	dest := filepath.Join(t.TempDir(), "i915", "blob.bin")

	if err := downloadFirmwareBlob(srv.URL+"/i915/blob.bin", dest, blob); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, pinned) {
		t.Errorf("cached firmware = %q, want %q", got, pinned)
	}
	entries, err := os.ReadDir(filepath.Dir(dest))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("the cache directory holds %v, want the blob alone", entries)
	}
}

// VALIDATES: a cached firmware blob is checked against its pin before it is
// used: a matching one is used without a download, and one that differs is
// refused with its path, both digests and the rm that lets the next build
// fetch the pinned blob, and is left in place for the operator to look at.
// PREVENTS: a blob cached once, then altered or replaced by hand, being
// compiled into every later kernel.
// Method: a synthetic pin whose commit linux-firmware does not have, so a
// check that fell through to a download would fail rather than pass.
func TestCachedFirmwareIsVerifiedBeforeUse(t *testing.T) {
	pinned := []byte("the pinned firmware")
	blob := kernelbuilder.FirmwareBlob{Path: "i915/blob.bin", Commit: strings.Repeat("c", 40), SHA256: firmwareDigest(pinned)}
	firmwareDir := t.TempDir()
	dest := filepath.Join(firmwareDir, "i915", "blob.bin")
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, pinned, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ensureFirmwareBlob(firmwareDir, blob); err != nil {
		t.Fatalf("a cached blob matching its pin was refused: %v", err)
	}

	corrupted := []byte("the pinned firmwarX")
	if err := os.WriteFile(dest, corrupted, 0o600); err != nil {
		t.Fatal(err)
	}
	err := ensureFirmwareBlob(firmwareDir, blob)
	if err == nil {
		t.Fatal("a cached blob whose SHA-256 differs from its pin was used")
	}
	for _, want := range []string{dest, firmwareDigest(corrupted), blob.SHA256, blob.Commit, "rm " + shellArg(dest)} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q:\n%v", want, err)
		}
	}
	if got, readErr := os.ReadFile(dest); readErr != nil || !bytes.Equal(got, corrupted) {
		t.Errorf("the refused cached blob was changed or removed: %q, %v", got, readErr)
	}
}

// VALIDATES: ensureFirmware, the step every hardware-kms kernel build runs,
// refuses a corrupted blob at the cache path the build reads from, against the
// real pin table.
// PREVENTS: the verification living in a helper the build does not call.
func TestEnsureFirmwareRefusesCorruptedCache(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var paths []string
	for _, blob := range kernelbuilder.HardwareKMSFirmware() {
		path := filepath.Join(resolveCacheDir(), firmwareCacheDir, filepath.FromSlash(blob.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("corrupted firmware"), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	if len(paths) == 0 {
		t.Fatal("the hardware-kms profile pins no firmware")
	}

	dir, err := ensureFirmware(hardwareKMSProfile)
	if err == nil {
		t.Fatalf("a corrupted firmware cache was handed to the build as %s", dir)
	}
	if !strings.Contains(err.Error(), "rm "+shellArg(paths[0])) {
		t.Errorf("the refusal does not give the rm of %s:\n%v", paths[0], err)
	}
}

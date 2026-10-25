package dockerhost

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writePins writes a pin file naming one docker download for arm64, served
// by the test server at /docker.tgz, and one source tarball, and answers its
// path.
func writePins(t *testing.T, dir, serverURL, dockerSum string) string {
	t.Helper()
	body := `{
  "builder": "alpine@sha256:5291449c3df73caf6ed85e649dec1b9e818b39a5d8c871e97afc13e9cd5e8fa8",
  "downloads": [
    {"name": "docker", "version": "29.9.2", "arch": "arm64", "url": "` + serverURL + `/docker.tgz", "sha256": "` + dockerSum + `"}
  ],
  "sources": [
    {"name": "sudo", "version": "1.9.16p2", "url": "` + serverURL + `/sudo.tar.gz", "sha256": "976aa56d3e3b2a75593307864288addb748c9c136e25d95a9cc699aafa77239c"}
  ]
}`
	path := filepath.Join(dir, "binaries.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func sumOf(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// TestDockerHostBuildRefusesAChecksumMismatch serves a docker tarball whose
// bytes do not match the pinned sha256, and asserts the fetch refuses before
// anything lands in the cache, with a message naming the file, both sums and
// the pin file. A second fetch with the right pin then lands the file, and a
// third, after the cached copy is altered, refuses the cached copy too.
//
// VALIDATES: spec AC-5 (third run) and R-8: a changed upstream binary never
// enters an image, and the operator is told which pin to check.
// PREVENTS: a mismatched download left in the cache, where the next build
// would pick it up without a check.
func TestDockerHostBuildRefusesAChecksumMismatch(t *testing.T) {
	served := []byte("the docker release bytes")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(served) //nolint:errcheck // test server
	}))
	defer server.Close()

	dir := t.TempDir()
	cache := filepath.Join(dir, "cache")
	wrong := strings.Repeat("0", 64)
	pinFile := writePins(t, dir, server.URL, wrong)
	pins, err := LoadPins(pinFile)
	if err != nil {
		t.Fatalf("LoadPins: %v", err)
	}
	pin, err := pins.Download("docker", "arm64")
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	fetcher := Fetcher{CacheDir: cache, Client: server.Client()}

	_, err = fetcher.Fetch(context.Background(), pins, pin)
	if err == nil {
		t.Fatal("Fetch accepted a download whose sha256 differs from the pin")
	}
	msg := err.Error()
	for _, want := range []string{"docker.tgz", wrong, sumOf(served), pinFile} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal %q does not name %q", msg, want)
		}
	}
	if entries, readErr := os.ReadDir(filepath.Join(cache, "downloads")); readErr == nil && len(entries) != 0 {
		t.Errorf("refused download left %d file(s) in the cache", len(entries))
	}

	pinFile = writePins(t, dir, server.URL, sumOf(served))
	pins, err = LoadPins(pinFile)
	if err != nil {
		t.Fatalf("LoadPins: %v", err)
	}
	pin, err = pins.Download("docker", "arm64")
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	path, err := fetcher.Fetch(context.Background(), pins, pin)
	if err != nil {
		t.Fatalf("Fetch with the right pin: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, served) {
		t.Errorf("cached file holds %q, want the served bytes", got)
	}

	if err := os.WriteFile(path, []byte("altered in the cache"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := fetcher.Fetch(context.Background(), pins, pin); err == nil {
		t.Error("Fetch accepted a cached file whose sha256 differs from the pin")
	}
}

// TestDockerHostBuildKeyIsStable computes the image key twice from the same
// inputs, then once with each input changed, and asserts the key is equal in
// the first case and different in every other.
//
// VALIDATES: spec AC-5 (second run builds nothing because the key is the
// same) and AC-7 (an input change gives a new key, so setup recreates the
// profile).
// PREVENTS: a pin, the kernel or the guest program changing while the cached
// image is reused.
func TestDockerHostBuildKeyIsStable(t *testing.T) {
	dir := t.TempDir()
	pins, err := LoadPins(writePins(t, dir, "https://example.invalid", strings.Repeat("a", 64)))
	if err != nil {
		t.Fatalf("LoadPins: %v", err)
	}
	base := KeyInputs{Arch: "arm64", KernelKey: "7.2.9-runtime-arm64-runtime-docker-387d6b76", SourceDigest: "src-1"}
	key, err := ImageKey(pins, base)
	if err != nil {
		t.Fatalf("ImageKey: %v", err)
	}
	again, err := ImageKey(pins, base)
	if err != nil {
		t.Fatalf("ImageKey: %v", err)
	}
	if key != again {
		t.Fatalf("same inputs gave %s then %s", key, again)
	}

	changedPins, err := LoadPins(writePins(t, t.TempDir(), "https://example.invalid", strings.Repeat("b", 64)))
	if err != nil {
		t.Fatalf("LoadPins: %v", err)
	}
	cases := []struct {
		name   string
		pins   *Pins
		inputs KeyInputs
	}{
		{"pinned sha256", changedPins, base},
		{"kernel key", pins, KeyInputs{Arch: base.Arch, KernelKey: "other", SourceDigest: base.SourceDigest}},
		{"guest source", pins, KeyInputs{Arch: base.Arch, KernelKey: base.KernelKey, SourceDigest: "src-2"}},
	}
	for _, tc := range cases {
		other, err := ImageKey(tc.pins, tc.inputs)
		if err != nil {
			t.Fatalf("%s: ImageKey: %v", tc.name, err)
		}
		if other == key {
			t.Errorf("changing the %s kept the key %s", tc.name, key)
		}
	}
	if _, err := ImageKey(pins, KeyInputs{Arch: "amd64", KernelKey: base.KernelKey, SourceDigest: base.SourceDigest}); err == nil {
		t.Error("ImageKey accepted an arch the pin file has no docker download for")
	}
}

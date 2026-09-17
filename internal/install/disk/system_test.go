// VALIDATES: installer preserves an image-baked ze/database.zefs seed instead
// of overwriting it with the image server's localhost-only bootstrap database.
// PREVENTS: regression where a provisioned appliance came up SSH-bound to
// 127.0.0.1:2222 (unreachable over the network) because mountInjectDB always
// downloaded /install/database.zefs over the full seed baked by `ze appliance
// build`.

package disk

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestBakedSeedPresent(t *testing.T) {
	dir := t.TempDir()

	// Absent: no file -> must fall back to the bootstrap database.
	missing := filepath.Join(dir, "absent.zefs")
	if bakedSeedPresent(missing) {
		t.Errorf("bakedSeedPresent(%q) = true, want false for a missing file", missing)
	}

	// Empty: a truncated/failed bake counts as absent so the box does not boot
	// with an unusable zero-length seed.
	empty := filepath.Join(dir, "empty.zefs")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if bakedSeedPresent(empty) {
		t.Errorf("bakedSeedPresent(%q) = true, want false for a zero-length file", empty)
	}

	// A directory at the path is not a seed.
	subdir := filepath.Join(dir, "dir.zefs")
	if err := os.Mkdir(subdir, 0o750); err != nil {
		t.Fatal(err)
	}
	if bakedSeedPresent(subdir) {
		t.Errorf("bakedSeedPresent(%q) = true, want false for a directory", subdir)
	}

	// Present: a non-empty file is the image-baked seed and must be kept.
	seed := filepath.Join(dir, "database.zefs")
	if err := os.WriteFile(seed, []byte("zefs-seed-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !bakedSeedPresent(seed) {
		t.Errorf("bakedSeedPresent(%q) = false, want true for a non-empty seed", seed)
	}
}

// TestMountInjectDBPreservesConvertedStore exercises the mounted-partition path
// with a live tree and no seed, so a basename-only seed check cannot pass.
func TestMountInjectDBPreservesConvertedStore(t *testing.T) {
	mountPoint := t.TempDir()
	tree := filepath.Join(mountPoint, "ze", "database")
	if err := os.MkdirAll(tree, 0o700); err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(tree, "retained")
	if err := os.WriteFile(key, []byte("retained value"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !bakedSeedPresent(tree) {
		t.Fatal("converted store was not recognised")
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	oldMount, oldUnmount, oldSync := mountFS, umountFS, syncFS
	t.Cleanup(func() { mountFS, umountFS, syncFS = oldMount, oldUnmount, oldSync })
	mountFS = func(_, _, _ string, _ bool) error { return nil }
	umountFS = func(string) error { return nil }
	syncFS = func() {}
	if err := mountInjectDB("test-partition", server.URL, mountPoint); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 0 {
		t.Fatal("installer downloaded a bootstrap seed beside the live store")
	}
	if _, err := os.Stat(filepath.Join(mountPoint, "ze", "database.zefs")); !os.IsNotExist(err) {
		t.Fatalf("seed appeared beside live store: %v", err)
	}
	value, err := os.ReadFile(key)
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != "retained value" {
		t.Fatalf("converted store changed: %q", value)
	}
}

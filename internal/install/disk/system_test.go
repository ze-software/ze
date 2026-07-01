// VALIDATES: installer preserves an image-baked ze/database.zefs seed instead
// of overwriting it with the image server's localhost-only bootstrap database.
// PREVENTS: regression where a provisioned appliance came up SSH-bound to
// 127.0.0.1:2222 (unreachable over the network) because mountInjectDB always
// downloaded /install/database.zefs over the full seed baked by `ze appliance
// build`.

package disk

import (
	"os"
	"path/filepath"
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

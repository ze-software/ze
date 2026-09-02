package diskspace_test

import (
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/core/diskspace"
)

// VALIDATES: free space is read for a directory nobody has created yet.
// PREVENTS: a first run on a fresh checkout refusing because the cache, or the
// build output directory, is absent.
func TestFreeWalksUpToAnExistingPath(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "cache", "go-cache")
	free, err := diskspace.Free(absent)
	if err != nil {
		t.Fatalf("free: %v", err)
	}
	if free == 0 {
		t.Error("free = 0 on a temporary directory's device")
	}
}

// VALIDATES: an existing directory reports its own device.
// PREVENTS: the walk-up above masking a real statfs failure by climbing to a
// parent that happens to answer.
func TestFreeReadsAnExistingPath(t *testing.T) {
	free, err := diskspace.Free(t.TempDir())
	if err != nil {
		t.Fatalf("free: %v", err)
	}
	if free == 0 {
		t.Error("free = 0 on a temporary directory's device")
	}
}

// VALIDATES: the rendering an operator compares against their own df output.
// PREVENTS: a guard message that states a byte count nobody can read.
func TestGiBRendersTenthsOfAGibibyte(t *testing.T) {
	cases := []struct {
		bytes uint64
		want  string
	}{
		{0, "0.0G"},
		{diskspace.BytesPerGiB, "1.0G"},
		{40 * diskspace.BytesPerGiB, "40.0G"},
		{diskspace.BytesPerGiB / 2, "0.5G"},
	}
	for _, c := range cases {
		if got := diskspace.GiB(c.bytes); got != c.want {
			t.Errorf("GiB(%d) = %q, want %q", c.bytes, got, c.want)
		}
	}
}

package testdeployment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestColdCacheErrorNamesTheEntryAndTheRemedy verifies the proof's refusal of
// an unusable runtime kernel cache names the entry, each problem, and the
// command that fills it, and tells an existing unusable entry from an absent one.
//
// The kernel package assembly this file used to test moved to
// internal/appliance/instance (TestAssembleKernelPackage): the image build now
// assembles ze's kernel itself, and this proof only checks the cache entry.
//
// VALIDATES: the proof's cold-cache refusal (gokrazyImage, checkRuntimeKernelCache).
// PREVENTS: a refusal an operator cannot act on.
func TestColdCacheErrorNamesTheEntryAndTheRemedy(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "7.2.9-runtime-amd64")
	err := coldCacheError(absent, ArchAMD64, []string{"no vmlinuz at " + absent})
	for _, want := range []string{absent, "no vmlinuz at", "build it once with: ./ze appliance kernel --target runtime --arch amd64"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("absent-entry refusal lacks %q:\n%v", want, err)
		}
	}

	present := t.TempDir()
	if err := os.MkdirAll(present, 0o755); err != nil {
		t.Fatal(err)
	}
	err = coldCacheError(present, ArchARM64, []string{"no PPPoL2TP"})
	if !strings.Contains(err.Error(), "the cache entry exists but is unusable; remove it") {
		t.Errorf("unusable-entry refusal does not say to remove it:\n%v", err)
	}
	if !strings.Contains(err.Error(), "--arch arm64") {
		t.Errorf("refusal does not name the arch:\n%v", err)
	}
}

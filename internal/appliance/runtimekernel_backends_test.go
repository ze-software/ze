// Design: docs/architecture/testing/qemu-integration.md -- Docker labs on the docker-host image
// Related: runtimekernel_cached.go -- runtimeKernelBackends, publishedRuntimeKernel

package appliance

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestRuntimeKernelBackendsNameOnlyTheRequestedOne proves an explicit --builder is
// served its own backend's kernel alone, and that a default resolution tries both
// backends, each once. Method: the three requests a caller can make.
func TestRuntimeKernelBackendsNameOnlyTheRequestedOne(t *testing.T) {
	for _, requested := range []string{builderDocker, builderQEMU} {
		if got := runtimeKernelBackends(requested); !slices.Equal(got, []string{requested}) {
			t.Errorf("requested %s: backends %v, want only %s", requested, got, requested)
		}
	}
	got := runtimeKernelBackends("")
	if len(got) != 2 || got[0] == got[1] {
		t.Fatalf("no backend requested: backends %v, want both, each once", got)
	}
	if !slices.Contains(got, builderDocker) || !slices.Contains(got, builderQEMU) {
		t.Fatalf("no backend requested: backends %v, want docker and qemu", got)
	}
}

// TestPublishedRuntimeKernelServesAnyBuildOfTheSameInputs proves the lookup serves
// the first entry that holds a built kernel and never reports one it does not hold,
// so a kernel either backend built of the same inputs is used instead of built again
// (ai/META-DECISIONS.md, "Test systems and labs"). Method: two cache entries, the
// artifact present in both, only the preferred, only the other, and neither.
func TestPublishedRuntimeKernelServesAnyBuildOfTheSameInputs(t *testing.T) {
	const artifact = "vmlinuz"
	cases := []struct {
		name            string
		inFirst, inLast bool
		wantIndex       int // -1: none found
	}{
		{"both entries built", true, true, 0},
		{"only the preferred backend built", true, false, 0},
		{"only the other backend built", false, true, 1},
		{"neither built", false, false, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			entries := []string{filepath.Join(root, "preferred"), filepath.Join(root, "other")}
			for i, built := range []bool{tc.inFirst, tc.inLast} {
				if err := os.MkdirAll(entries[i], 0o750); err != nil {
					t.Fatal(err)
				}
				if !built {
					continue
				}
				if err := os.WriteFile(filepath.Join(entries[i], artifact), []byte("kernel"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			entry, found, err := publishedRuntimeKernel(entries, artifact)
			if err != nil {
				t.Fatalf("lookup: %v", err)
			}
			if tc.wantIndex < 0 {
				if found {
					t.Fatalf("found %s although no entry holds a kernel", entry)
				}
				return
			}
			if !found {
				t.Fatalf("found nothing, want %s", entries[tc.wantIndex])
			}
			if entry != entries[tc.wantIndex] {
				t.Fatalf("served %s, want %s", entry, entries[tc.wantIndex])
			}
		})
	}
}

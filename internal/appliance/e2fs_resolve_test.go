package appliance

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveE2FSToolFindsSplitPackageLayout proves each e2fsprogs tool is
// resolved independently of the others.
//
// VALIDATES: two tools living in DIFFERENT directories both resolve.
// PREVENTS:  the same-directory assumption that broke every appliance build on a
//
//	distribution that splits the package. Alpine ships debugfs in
//	e2fsprogs-extra, so with e2fsprogs AND e2fsprogs-extra installed no
//	single directory held both mkfs.ext4 and debugfs; the old lookup
//	returned "" and reported every tool absent, and injectZeFS logged
//	"debugfs write silently failed" with the binaries sitting on disk.
//
// Synthetic tool names, so a host that genuinely has e2fsprogs installed cannot
// satisfy the assertion from /sbin and make it vacuous.
func TestResolveE2FSToolFindsSplitPackageLayout(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	const toolA = "ze-fake-mkfs-tool"
	const toolB = "ze-fake-debugfs-tool"
	pathA := filepath.Join(dirA, toolA)
	pathB := filepath.Join(dirB, toolB)
	for _, p := range []string{pathA, pathB} {
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil { //nolint:gosec // test fixture must be executable to be found on PATH
			t.Fatalf("write %s: %v", p, err)
		}
	}

	t.Setenv("PATH", dirA+string(os.PathListSeparator)+dirB)

	if got := resolveE2FSTool(toolA); got != pathA {
		t.Errorf("resolveE2FSTool(%s) = %q, want %q", toolA, got, pathA)
	}
	if got := resolveE2FSTool(toolB); got != pathB {
		t.Errorf("resolveE2FSTool(%s) = %q, want %q; a tool in a DIFFERENT directory from its sibling must still resolve", toolB, got, pathB)
	}
	if got := resolveE2FSTool("ze-fake-absent-tool"); got != "" {
		t.Errorf("resolveE2FSTool of an absent tool = %q, want empty", got)
	}
}

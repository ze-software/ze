package golden

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDirCoverageReportsBothDirections drives the orphan check every capture
// hangs its fixture directory on.
//
// VALIDATES: dirCoverage names a fixture on disk that no case writes, and a
// fixture a case expects that disk does not hold.
// PREVENTS: a fixture whose case was deleted staying on disk, where the next
// reader counts bytes nobody compares as coverage.
func TestDirCoverageReportsBothDirections(t *testing.T) {
	findings := dirCoverage("testCases",
		[]string{"testdata/handler/kept.txt", "testdata/handler/orphan.txt"},
		[]string{"testdata/handler/kept.txt", "testdata/handler/missing.txt"})

	want := []string{
		"fixture testdata/handler/orphan.txt is on disk and no case in testCases writes it; " +
			"delete it or restore its case",
		"testCases captures testdata/handler/missing.txt, which is not on disk; capture it with -update-golden",
	}

	if len(findings) != len(want) {
		t.Fatalf("findings = %v, want %d", findings, len(want))
	}

	for i, w := range want {
		if findings[i].Error() != w {
			t.Errorf("finding %d = %q, want %q", i, findings[i], w)
		}
	}
}

// TestDirCoverageIsSilentWhenTheSetsMatch proves the check above discriminates.
//
// VALIDATES: dirCoverage returns no finding when disk holds exactly the
// fixtures the capture writes.
// PREVENTS: a check that fails for every input, which reports the orphan above
// and proves nothing about it.
func TestDirCoverageIsSilentWhenTheSetsMatch(t *testing.T) {
	findings := dirCoverage("testCases",
		[]string{"testdata/handler/a.txt", "testdata/handler/b.txt"},
		[]string{"testdata/handler/b.txt", "testdata/handler/a.txt"})
	if len(findings) != 0 {
		t.Fatalf("findings = %v, want none", findings)
	}
}

// TestFilesUnderReadsEveryDepth covers the walk that feeds the check above.
//
// VALIDATES: filesUnder returns every file below the root, nested ones
// included, and no directory.
// PREVENTS: a template capture whose fixtures sit one directory down reporting
// every one of them as an orphan, or reporting none of them at all.
func TestFilesUnderReadsEveryDepth(t *testing.T) {
	root := t.TempDir()

	nested := filepath.Join(root, "page")
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatalf("create the probe directory: %v", err)
	}

	for _, path := range []string{filepath.Join(root, "top.txt"), filepath.Join(nested, "login.html")} {
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	files, err := filesUnder(root)
	if err != nil {
		t.Fatalf("filesUnder: %v", err)
	}

	want := []string{filepath.Join(nested, "login.html"), filepath.Join(root, "top.txt")}
	if !equalStrings(files, want) {
		t.Errorf("files = %v, want %v", files, want)
	}
}

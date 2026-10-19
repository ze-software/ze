// VALIDATES: spec-lab-docker-cleanup-and-go-test-containers AC-17 -- the
// wiring check and the test-only import gate read one declaration.
// PREVENTS: a wiring check that demands a non-test reference the tier gate
// refuses, so no export of a test-only helper could ever pass both.

package docwiring

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/arch/testonly"
)

// wiringIssuesFor adds helper.go under dir, exporting Ready, with a test that
// is its only reference, and answers the issues checkWiring reports for it.
func wiringIssuesFor(t *testing.T, dir string) []string {
	t.Helper()
	root := pushedTree(t, map[string]string{"README.md": "fixture\n"})
	source := path.Join(dir, "helper.go")
	pkg := path.Base(dir)
	files := map[string]string{
		source:                           "package " + pkg + "\n\nfunc Ready() {}\n",
		path.Join(dir, "helper_test.go"): "package " + pkg + "\n\nfunc use() { Ready() }\n",
	}
	for rel, body := range files {
		target := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			t.Fatalf("fixture directory: %v", err)
		}
		if err := os.WriteFile(target, []byte(body), 0o600); err != nil {
			t.Fatalf("fixture file: %v", err)
		}
	}

	issues, err := checkWiring(root, "HEAD", []string{source})
	if err != nil {
		t.Fatalf("checkWiring: %v", err)
	}
	return issues
}

// TestATestOnlyPackageExportNeedsNoProductionReference proves the exemption
// is the declared list and nothing wider. Method: the same unreferenced export
// is judged once inside the declared test-only package, where the tier gate
// refuses any non-test importer, and once in an ordinary package.
func TestATestOnlyPackageExportNeedsNoProductionReference(t *testing.T) {
	helper := testonly.Packages[0]
	if issues := wiringIssuesFor(t, helper); len(issues) != 0 {
		t.Errorf("test-only package %s: got %q, want no issue", helper, issues)
	}

	ordinary := path.Join(path.Dir(helper), "ordinaryhelper")
	issues := wiringIssuesFor(t, ordinary)
	if len(issues) != 1 || !strings.Contains(issues[0], "func Ready has no non-test reference") {
		t.Errorf("ordinary package %s: got %q, want one unwired Ready", ordinary, issues)
	}
}

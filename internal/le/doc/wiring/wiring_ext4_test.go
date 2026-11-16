// Design: docs/contributing/documentation-testing.md -- test-only exports and production wiring.
// Related: ../../arch/testonly/testonly.go -- the shared helper declaration.
package docwiring

import (
	"path"
	"strings"
	"testing"
)

// ext4WiringIssues adds the three guest-disk API shapes to an otherwise empty
// committed tree, optionally consumed only by another package's Go test.
func ext4WiringIssues(t *testing.T, helper string, testCaller bool) []string {
	t.Helper()
	root := pushedTree(t, map[string]string{
		"README.md": "fixture\n",
		"go.mod":    "module example.test/wiring\n\ngo 1.24\n",
	})
	source := path.Join(helper, "disk_linux.go")
	body := "package " + path.Base(helper) + "\n\n" +
		"type Disk struct{}\n\n" +
		"func CopyInto() {}\n\n" +
		"func AttachDisk() *Disk { return &Disk{} }\n\n" +
		"func (disk *Disk) KernelPartition() {}\n"
	writeFixture(t, root, source, body)
	if testCaller {
		writeFixture(t, root, "internal/core/consumer/disk_test.go",
			"package consumer\n\nimport fixture \"example.test/wiring/"+helper+"\"\n\n"+
				"func use() { fixture.CopyInto(); disk := fixture.AttachDisk(); disk.KernelPartition() }\n")
	}
	issues, err := checkWiring(root, "HEAD", []string{source})
	if err != nil {
		t.Fatalf("check guest-disk wiring: %v", err)
	}
	return issues
}

// TestExt4FixtureExportsMayBeConsumedOnlyByTests drives the wiring verdict
// over the declared helper and its real function/method usage pattern.
func TestExt4FixtureExportsMayBeConsumedOnlyByTests(t *testing.T) {
	if issues := ext4WiringIssues(t, "internal/test/ext4fixture", true); len(issues) != 0 {
		t.Fatalf("declared guest fixture exports need no production caller: %v", issues)
	}
}

// TestProductionExportsAreNotWiredOnlyByTests keeps the production-only
// oracle: the same API used only by a test is still unwired in a product package.
func TestProductionExportsAreNotWiredOnlyByTests(t *testing.T) {
	issues := ext4WiringIssues(t, "internal/component/diskhelper", true)
	requireExt4WiringFindings(t, issues)
}

// TestUnusedUndeclaredFixtureExportsRemainUnwired refuses the same API in
// an unused fixture package, so internal/test is never a blanket exemption.
func TestUnusedUndeclaredFixtureExportsRemainUnwired(t *testing.T) {
	issues := ext4WiringIssues(t, "internal/test/unusedfixture", false)
	requireExt4WiringFindings(t, issues)
}

func requireExt4WiringFindings(t *testing.T, issues []string) {
	t.Helper()
	if len(issues) != 3 {
		t.Fatalf("unwired disk API: %v, want three findings", issues)
	}
	joined := strings.Join(issues, "\n")
	for _, name := range []string{"CopyInto", "AttachDisk", "KernelPartition"} {
		if !strings.Contains(joined, name+" has no non-test reference") {
			t.Errorf("unwired disk API omits %s: %s", name, joined)
		}
	}
}

// Design: docs/architecture/testing/test-health.md -- approvals cover changes to existing units.
// Related: parity_test.go -- committed baseline and approval fixtures.
package testweakened

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestCheckSemanticApprovalExcludesNewUnits proves that adding a function or
// a file cannot spend its approval, even beside a changed existing test that
// does carry its own verdict. It drives Check against committed fixture text.
func TestCheckSemanticApprovalExcludesNewUnits(t *testing.T) {
	for _, test := range []struct {
		name           string
		changeExisting bool
		newFile        bool
	}{
		{name: "new function only"},
		{name: "new function beside changed existing test", changeExisting: true},
		{name: "entirely new file", newFile: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := newParityRepository(t)
			const path = "pkg/a_test.go"
			// Commit a standalone closing brace so appending a function does
			// not extend TestA's unit span or change its baseline text.
			const baseline = "package a\nfunc TestA(t *testing.T) {\n\trequire.Equal(t, 1, got)\n}\n"
			writeParityFile(t, root, path, baseline)
			if !runSelfTestGit(root, "add", "--", path) ||
				!runSelfTestGit(root, "-c", "user.email=t@t", "-c", "user.name=t",
					"-c", "commit.gpgsign=false", "commit", "-q", "-m", "bounded test baseline") {
				t.Fatal("commit the standalone-brace fixture baseline")
			}
			const added = "\nfunc TestAdded(t *testing.T) {\n\trequire.Equal(t, 1, added)\n}\n"
			paths := []string{path}
			if test.newFile {
				paths = []string{"pkg/new_test.go"}
				writeParityFile(t, root, paths[0], "package a\n"+added)
			} else {
				content := baseline
				if test.changeExisting {
					content = strings.Replace(content, "1, got", "2, got", 1)
				}
				if !test.changeExisting {
					beforeUnit := unitText(unitsByName(baseline), "TestA")
					afterUnit := unitText(unitsByName(content+added), "TestA")
					if beforeUnit == "" || beforeUnit != afterUnit {
						t.Fatalf("adding TestAdded changed TestA's fixture span: before %q, after %q", beforeUnit, afterUnit)
					}
				}
				writeParityFile(t, root, path, content+added)
			}
			const reason = "independent verdict reserved for an existing test change"
			for _, unit := range []string{"pkg.TestA", "pkg.TestAdded"} {
				if _, err := Approve(root, fixtureSession, unit, reason); err != nil {
					t.Fatal(err)
				}
			}
			approvalPath := filepath.Join(root, filepath.FromSlash(fixtureApproval))
			before, err := os.ReadFile(approvalPath)
			if err != nil {
				t.Fatal(err)
			}
			result := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: paths})
			if result.ExitCode() != 0 || len(result.Findings) != 0 {
				t.Fatalf("semantic fixture = code %d, findings %v, problems %v", result.ExitCode(), result.Findings, result.Problems)
			}
			var trailers, dropped []string
			if test.changeExisting {
				trailers = []string{ApprovalTrailerLine("pkg.TestA", reason)}
				dropped = []string{"| pkg.TestA | " + reason + " |"}
			}
			if !slices.Equal(result.Approvals.Trailers, trailers) || !slices.Equal(result.Approvals.Dropped, dropped) {
				t.Fatalf("approval selection = trailers %q, dropped %q; want %q, %q", result.Approvals.Trailers, result.Approvals.Dropped, trailers, dropped)
			}
			after, err := os.ReadFile(approvalPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after, before) {
				t.Fatalf("Check changed approval file: before %q, after %q", before, after)
			}
		})
	}
}

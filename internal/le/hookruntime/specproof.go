// Design: docs/contributing/spec-workflow.md -- daemon changes name real entry-point proof
// Related: lifecycle.go -- validateSpecText consumes this judgment
package hookruntime

import (
	"path/filepath"
	"strings"

	archtier "github.com/ze-software/ze/internal/le/arch/tier"
	specstatus "github.com/ze-software/ze/internal/le/spec/status"
)

// specStatusSkeleton is the status of a spec that holds an idea not yet
// designed, which the spec gates judge more lightly than a design.
const specStatusSkeleton = "skeleton"

// testdataDir is the directory name the go tool skips when it lists
// packages, the one Go reserves for test fixtures.
const testdataDir = "testdata"

// daemonProofProblem distinguishes the changed runtime population from tooling
// and test-only edits. The proof may use any native functional test format.
func daemonProofProblem(root, text string) string {
	status, err := specstatus.MetadataStatus(text)
	if err != nil {
		return "Spec metadata: " + err.Error()
	}
	if status == specStatusSkeleton {
		return ""
	}
	changed := markdownSection(text, "## Files to Modify") + "\n" + markdownSection(text, "## Files to Create")
	daemon := false
	for _, named := range specRepoPaths(root, changed) {
		path, err := checkoutRelative(context{root: root, path: named})
		if err != nil {
			return "Daemon-path proof cannot resolve changed source " + named + ": " + err.Error()
		}
		if daemonProofExempt(path) {
			continue
		}
		if !sourceKind(path) {
			continue
		}
		// Unknown source roots, including planned new files, owe proof.
		// Inspect every named path before granting any exemption or proof.
		daemon = true
	}
	if !daemon {
		return ""
	}
	functional := false
	for _, named := range specRepoPaths(root, text) {
		// Other kinds cannot provide this proof. Selecting candidates here
		// does not grant permission to a raw test/ prefix or an alias.
		if !functionalTestFormat(named) {
			continue
		}
		path, err := checkoutRelative(context{root: root, path: named})
		if err != nil {
			return "Daemon-path proof cannot resolve functional test " + named + ": " + err.Error()
		}
		if !strings.HasPrefix(path, "test/") {
			continue
		}
		if functionalTestFormat(path) {
			functional = true
		}
	}
	if functional {
		return ""
	}
	return "Daemon-path changes must reference a functional test file under test/ (.ci, .et or .wb); add the real entry-point proof to the test plan. A helper-only unit test does not prove the daemon path"
}

// functionalTestFormat selects native proof kinds, without granting that a raw
// path belongs to test/. Only the canonical identity can grant that proof.
func functionalTestFormat(path string) bool {
	switch filepath.Ext(path) {
	case ".ci", ".et", ".wb":
		return true
	default:
		return false
	}
}

// daemonProofExempt uses the architecture gate's explicit nonproduction
// population. Callers MUST pass a canonical checkout-relative identity, resolved
// with checkoutRelative before asking for any exemption.
func daemonProofExempt(path string) bool {
	if strings.HasSuffix(path, "_test.go") {
		return true
	}
	if strings.HasPrefix(path, "internal/le/") {
		return true
	}
	if strings.HasPrefix(path, "test/") {
		return true
	}
	for _, prefix := range archtier.DisableableNonProdPrefixes[:] {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	for segment := range strings.SplitSeq(path, "/") {
		if segment == testdataDir {
			return true
		}
	}
	return false
}

// Design: docs/architecture/core-design.md -- native Go gates run through le
// Related: fixtures.go -- the category probes that name a commit gate
//
// A fixture category whose rule `./le commit create` enforces, rather than a
// hook, asks that commit gate here. The gate's own function answers, so the
// fixture never restates the rule.
package aihooks

import (
	"fmt"
	"strings"

	lecommit "github.com/ze-software/ze/internal/le/commit"
)

// The commit gates a category probe may name, and the files the test-first
// probe tree carries.
const (
	gateTestCoverage  = "test-coverage"
	commitPrepareFile = "internal/le/commit/prepare.go"
	probeGoTestPath   = "internal/probe/probe_test.go"
	// probeGoSource is the body every probe tree gives a Go file it writes.
	probeGoSource = "package probe\n"
)

// commitGates maps each gate name a probe may carry to the commit package's
// own function for it. TestEveryProbeNamesARegisteredCheck refuses a probe
// whose gate is missing here.
//
//nolint:gochecknoglobals // fixture data, read by the category probes
var commitGates = map[string]func(root string, paths []string) ([]string, error){
	gateTestCoverage: lecommit.CoverageProblems,
}

// commitPathsTree builds a checkout holding each space-separated path in value
// as a Go file, so the commit gate reads real working-tree content.
func commitPathsTree(value string) (string, error) {
	files := map[string]string{}
	for path := range strings.FieldsSeq(value) {
		files[path] = probeGoSource
	}
	return probeTree("commit-paths", files)
}

// commitGateVerdict asks the named commit gate about a commit of the
// space-separated paths in value. A problem the gate names is a refusal, exit 2,
// carrying its text; a gate it cannot run answers an error, never a verdict.
func commitGateVerdict(gate, value, root string) (int, string, error) {
	judge, registered := commitGates[gate]
	if !registered {
		return 0, "", fmt.Errorf("the commit gate %q is not in commitGates", gate)
	}
	if root == "" {
		return 0, "", fmt.Errorf("the commit gate %q needs a tree of its own to read", gate)
	}
	problems, err := judge(root, strings.Fields(value))
	if err != nil {
		return 0, "", fmt.Errorf("the commit gate %q could not judge %q: %w", gate, value, err)
	}
	if len(problems) != 0 {
		return 2, strings.Join(problems, "\n"), nil
	}
	return 0, "", nil
}

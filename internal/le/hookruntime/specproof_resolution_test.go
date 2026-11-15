// Design: docs/contributing/spec-workflow.md -- names a spec cites that are no producer
// VALIDATES: a spec naming a directory that resolves outside the checkout (the
// `cache/go-cache` symlink) passes the design gate, while a source file or a
// missing path reached through the same symlink stays refused; and the
// daemon-path refusal names each path it judged, as written, with both fixes.
// PREVENTS: every edit of a spec whose evidence prose names `cache/go-cache`
// being refused, and a daemon-path refusal that does not say which name drew it.
package hookruntime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const resolutionHeader = "# Spec: probe\n\n| Field | Value |\n|-------|-------|\n| Status | design |\n\n"

// resolutionFixture is a checkout whose `cache` entry is a symlink to a
// directory outside it that holds `go-cache/` and `escaped.go`, and in which
// evidenceProbeB is read in this context.
func resolutionFixture(t *testing.T) string {
	t.Helper()
	root, outside := gitFixture(t), t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, "go-cache"), 0o750); err != nil {
		t.Fatal(err)
	}
	writeHookFixture(t, outside, "escaped.go", "package outside\n")
	if err := os.Symlink(outside, filepath.Join(root, "cache")); err != nil {
		t.Fatal(err)
	}
	writeHookFixture(t, root, evidenceProbeB, "package other\n")
	recordObservedRead(t, root, filepath.Join(root, evidenceProbeB), "package other\n", 1, 2, 2)
	return root
}

// TestDesignEvidenceSkipsDirectoryOutsideCheckout checks that a directory a
// spec names is no producer wherever it resolves, and that a source file or a
// missing path reached through the same symlink keeps the confinement refusal.
//
// MUTATION: (internal/le/hookruntime/designevidence.go, "\t\t\t\t\tcontinue\n\t\t\t\t}\n\t\t\t\tif !sourceKind(path)", "\t\t\t\t\treturn nil, err\n\t\t\t\t}\n\t\t\t\tif !sourceKind(path)")
// turns the directory row red: the outside directory refuses the edit again.
// MUTATION: (internal/le/hookruntime/designevidence.go, "\t\t\t\t\tif !directory {", "\t\t\t\t\tif false {")
// turns the file row red: a file outside the checkout is skipped like a directory.
// MUTATION: (internal/le/hookruntime/designevidence.go, "return nil, fmt.Errorf(\"%s: resolve producer: %w\", named, errors.Join(err, statErr))", "continue")
// turns the missing row red: a missing path outside is skipped when stat cannot find it.
func TestDesignEvidenceSkipsDirectoryOutsideCheckout(t *testing.T) {
	root := resolutionFixture(t)
	observedDesignGate(t, root, "## Current Behavior\nRan with an empty `cache/go-cache`.\n", 0)
	for _, named := range []string{"cache/escaped.go", "cache/missing.go"} {
		t.Run(named, func(t *testing.T) {
			message := observedDesignGate(t, root, "## Current Behavior\n`"+named+"`\n", 2)
			if !strings.Contains(message, named+": resolve producer") || !strings.Contains(message, "resolves outside this checkout") {
				t.Fatalf("%s outside the checkout is not refused for confinement: %s", named, message)
			}
		})
	}
}

// TestDaemonProofRefusalNamesEachJudgedPath checks that the refusal names every
// changed path judged as daemon code, as the spec wrote it, a shorthand among
// them, and gives both fixes; and that an exempt path is not named.
//
// MUTATION: (internal/le/hookruntime/specproof.go, "daemon = append(daemon, named)", "daemon = append(daemon, path)")
// turns it red: the canonical internal/component/bgp/reactor/peer.go replaces
// the alias/reactor/peer.go the spec wrote.
// MUTATION: (internal/le/hookruntime/specproof.go, "strings.Join(daemon, \", \")", "\"\"")
// turns it red: the refusal no longer names the paths it judged.
func TestDaemonProofRefusalNamesEachJudgedPath(t *testing.T) {
	root := t.TempDir()
	// alias/ is an in-checkout symlink, so alias/reactor/peer.go is written
	// differently from its canonical identity internal/component/bgp/reactor/peer.go.
	if err := os.MkdirAll(filepath.Join(root, "internal", "component", "bgp"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("internal", "component", "bgp"), filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	problem := daemonProofProblem(root, resolutionHeader+
		"## Files to Create\n`./bgp/run.go`, `alias/reactor/peer.go` and `internal/le/probe/tool.go`\n") // <!-- doc-links: ignore (intentionally nonexistent planned tooling file tests the daemon-proof exemption) -->
	for _, want := range []string{
		"Daemon-path changes must reference a functional test file under test/",
		"Judged as daemon code, from the checkout root: bgp/run.go, alias/reactor/peer.go.",
		"add the real entry-point proof",
		"write that file's full path from the repository root",
	} {
		if !strings.Contains(problem, want) {
			t.Fatalf("refusal lacks %q: %s", want, problem)
		}
	}
	if strings.Contains(problem, "internal/le/probe/tool.go") {
		t.Fatalf("refusal names an exempt path: %s", problem)
	}
}

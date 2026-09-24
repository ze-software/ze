// Helpers the RFC rollup cases share. Nothing here asserts; each function
// answers a corpus or a shape a case in testhealth_test.go then judges.

package testhealth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/rfc"
)

// repoRoot answers the checkout the real-tree cases measure.
func repoRoot(t *testing.T) string {
	t.Helper()

	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the checkout root: %v", err)
	}
	return root
}

// packageGoFiles answers one directory's Go source, test files excluded.
//
// The exclusion is what makes the case that reads these files mean anything: a
// case asserting that no file here names a path has to be allowed to name it
// itself.
func packageGoFiles(t *testing.T, directory string) []string {
	t.Helper()

	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("reading %s: %v", directory, err)
	}
	var found []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		found = append(found, filepath.Join(directory, name))
	}
	if len(found) == 0 {
		t.Fatalf("%s holds no Go source, so nothing was read", directory)
	}
	return found
}

// rowsByRFC keys coverage rows by their RFC stem, which is how two derivations
// of one population are compared: the render sorts by enrolment then by work
// outstanding, and the model answers in requirement order.
func rowsByRFC(rows []coverageRow) map[string]coverageRow {
	out := make(map[string]coverageRow, len(rows))
	for _, row := range rows {
		out[row.rfc] = row
	}
	return out
}

// addGatedRequirements appends one RFC's gated MUSTs to a corpus, in the four
// states rfc.CoverageRows partitions a gated population into: proven in both
// polarities, proven in one, annotated, and carrying no test at all.
//
// The four counts are written out rather than derived from each other, because
// the cases below exist to catch exactly the derivation that conflates two of
// them.
func addGatedRequirements(collected *rfc.Collected, stem string, both, one, annotated, noTest int) {
	next := 0
	nextRID := func() string {
		next++
		var tb textbuf.Buffer
		return tb.Str(rfc.Prefix(stem)).Str("-1-").Int(int64(next)).String()
	}
	gated := func(rid string) rfc.Requirement {
		return rfc.Requirement{RFC: stem, RID: rid, Level: "MUST", Section: "1"}
	}

	for range both {
		rid := nextRID()
		collected.Requirements = append(collected.Requirements, gated(rid))
		collected.Tags = append(collected.Tags,
			rfc.Tag{RID: rid, Polarity: rfc.PolarityPositive, File: "x_test.go"},
			rfc.Tag{RID: rid, Polarity: rfc.PolarityNegative, File: "x_test.go"})
	}
	for range one {
		rid := nextRID()
		collected.Requirements = append(collected.Requirements, gated(rid))
		collected.Tags = append(collected.Tags,
			rfc.Tag{RID: rid, Polarity: rfc.PolarityPositive, File: "x_test.go"})
	}
	for range annotated {
		requirement := gated(nextRID())
		requirement.Annotation = &rfc.Annotation{Kind: rfc.AnnotationGap, Reason: "unbuilt"}
		collected.Requirements = append(collected.Requirements, requirement)
	}
	for range noTest {
		collected.Requirements = append(collected.Requirements, gated(nextRID()))
	}
}

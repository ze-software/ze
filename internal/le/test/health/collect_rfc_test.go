// Helpers the RFC rollup cases share. Nothing here asserts; each function
// answers a corpus or a shape a case in testhealth_test.go then judges.

package testhealth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
	lepath "github.com/ze-software/ze/internal/le/le/path"
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

func TestPartialHealthExplainsScopedEvidence(t *testing.T) {
	for _, tc := range []struct{ name, padding string }{
		{"compact", ""},
		{"space", " "},
		{"tab", "\t"},
		{"nonbreaking-space", "\u00a0"},
		{"next-line", "\u0085"},
		{"em-space", "\u2003"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			const summaryPath = "rfc/short/rfc9999.md"
			marker := "{" + tc.padding + "partial" + tc.padding + `: tested "ignored on receipt."; gap "MUST be zero when sent"; sending absent at internal/sample/widget.go::SendWidget}`
			summary := "# RFC 9999\n\n## Meta\n\n| Field | Value |\n|---|---|\n" +
				"| Title | Widgets |\n| Enrolment | enrolled |\n| Enrolment reason | isolated fixture |\n" +
				"| Implementation | ze |\n| Implementation reason | fixture production boundary |\n" +
				"| Support | bgp-base 10 |\n| Support area | Widgets |\n| Support status | Partial |\n" +
				"| Support coverage | receipt tests |\n| Support remaining | native sending absent |\n\n" +
				"## Compliance Checklist\n\n- [ ] [RFC9999-2-1] [MUST] A widget MUST be zero when sent and ignored on receipt. (§2) " + marker + "\n"
			for rel, body := range map[string]string{
				".github/workflows/fixture.yml": "name: fixture\non: push\njobs: {}\n",
				summaryPath:                     summary,
				"test/plugin/widget.ci":         "# RFC requirement: RFC9999-2-1 positive -- receipt accepts zero\n# RFC requirement: RFC9999-2-1 negative -- receipt ignores nonzero\n",
				"internal/sample/widget.go":     "package sample\n\nfunc SendWidget(n int) int { return n }\n",
			} {
				path := filepath.Join(root, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			tree := newTree(root)
			tree.tracked = map[string]bool{summaryPath: true}
			metric, _, err := collectRFC(tree, qualityFloors{})
			if err != nil {
				t.Fatal(err)
			}
			density, ok := metric.Data.get("proof_density").(object)
			if !ok {
				t.Fatalf("proof_density is not an object: %T", metric.Data.get("proof_density"))
			}
			annotations, ok := metric.Data.get("annotations").(object)
			if !ok {
				t.Fatalf("annotations is not an object: %T", metric.Data.get("annotations"))
			}
			if density.get("numerator") != 0 || density.get("denominator") != 1 || annotations.get(rfc.AnnotationPartial) != 1 ||
				annotations.get(rfc.AnnotationGap) != 0 || metric.Data.get("gated_without_any_test") != 0 {
				t.Fatalf("health collection conflated scope, whole proof or untested gaps: %+v", metric)
			}
			if !strings.Contains(metric.Detail, "partial") || !strings.Contains(metric.Detail, "scoped evidence") {
				t.Fatal("health explanation lost the distinction between clause evidence and whole proof")
			}
			// Removing only the scope changes coverage interpretation, not the source
			// population; the collected metric must reflect that exact difference.
			if err := os.WriteFile(filepath.Join(root, summaryPath), []byte(strings.Replace(summary, " "+marker, "", 1)), 0o600); err != nil {
				t.Fatal(err)
			}
			whole, _, err := collectRFC(tree, qualityFloors{})
			if err != nil {
				t.Fatal(err)
			}
			wholeDensity, ok := whole.Data.get("proof_density").(object)
			if !ok {
				t.Fatalf("whole proof_density is not an object: %T", whole.Data.get("proof_density"))
			}
			if wholeDensity.get("numerator") != 1 || wholeDensity.get("denominator") != density.get("denominator") {
				t.Fatalf("health ignored authored scope or changed its denominator: %+v", whole)
			}
		})
	}
}

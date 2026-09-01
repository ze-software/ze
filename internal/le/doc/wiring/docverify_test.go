package docwiring

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// ledgerFixtureTree writes the smallest checkout the RFC freshness stage can
// judge, then generates every page that stage compares against.
//
// One enrolled summary carrying one gated requirement, its source text, and the
// two manifests the tag scanner reads. The generated pages are written by the
// real writer rather than by hand, because the property under test is that the
// stage notices a page DIVERGING from what that writer produces.
func ledgerFixtureTree(t *testing.T) string {
	t.Helper()

	const summary = "# RFC 9999\n\n## Meta\n\n| Field | Value |\n|-------|-------|\n" +
		"| Title | Widgets |\n| Enrolment | enrolled |\n" +
		"| Enrolment reason | the fixture RFC, gated so the stage has a population |\n" +
		"| Support | bgp-base 10 |\n| Support area | Widgets |\n" +
		"| Support status | Partial |\n| Support coverage | unit tests |\n" +
		"| Support remaining | Zero MUST gaps. |\n\n" +
		"## Compliance Checklist\n\n" +
		"- [ ] [RFC9999-2-1] [MUST] A speaker MUST send the widget (§2)\n"

	root := t.TempDir()
	for rel, body := range map[string]string{
		"rfc/short/rfc9999.md":          summary,
		"rfc/full/rfc9999.txt":          "A speaker MUST send the widget.\n",
		"rfc/drain-budget.txt":          "start 2026-07-29\nrate 0\n",
		"feature-gates.txt":             "ze_widget  internal/widget\n",
		".github/workflows/nightly.yml": "on:\n  schedule:\n    - cron: '0 3 * * *'\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	if _, err := rfc.IndexUpdate(root); err != nil {
		t.Fatalf("generate the fixture ledger pages: %v", err)
	}
	return root
}

// VALIDATES: AC-9 -- a hand edit to any of the three generated ledger files is
// reported stale against the summaries that declare it.
// PREVENTS: the failure mode a generated page has and an authored one does not.
// docs/features/rfc-status.md was authored until 2026-09-01 and is derived now,
// so an author editing it in place loses the edit at the next
// `./le rfc index-update` with nothing saying so. Being told the page is stale
// is what turns a silent loss into a message. The two disposition files carry
// the same hazard and are checked on the same footing.
func TestHandEditedStatusPageReportsStale(t *testing.T) {
	for _, rel := range rfc.LedgerPaths() {
		t.Run(rel, func(t *testing.T) {
			root := ledgerFixtureTree(t)

			page, code := rfcFreshnessStage(root)
			if code != 0 {
				t.Fatalf("a freshly generated tree reported code %d:\n%s", code, pageText(t, page))
			}

			path := filepath.Join(root, filepath.FromSlash(rel))
			body, err := os.ReadFile(path) //nolint:gosec // this test's own fixture tree
			if err != nil {
				t.Fatalf("read %s: %v", rel, err)
			}
			// An edit a person would plausibly make: one more sentence, in
			// the register the file already uses. Truncating the file would
			// prove less, because a reader could believe only a corrupt page
			// is caught.
			edited := string(body) + "\nA sentence an author added by hand.\n"
			if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
				t.Fatalf("hand-edit %s: %v", rel, err)
			}

			page, code = rfcFreshnessStage(root)
			text := pageText(t, page)
			if code == 0 {
				t.Fatalf("a hand edit to %s was not reported:\n%s", rel, text)
			}
			if !strings.Contains(text, rel) {
				t.Errorf("the report does not name %s:\n%s", rel, text)
			}
			if !strings.Contains(text, "./le rfc index-update") {
				t.Errorf("the report does not name the command that repairs it:\n%s", text)
			}
		})
	}
}

// pageText answers the rendered text of whatever the stage returned.
func pageText(t *testing.T, page any) string {
	t.Helper()

	rendered, held := page.(docVerifyPage)
	if !held {
		t.Fatalf("the stage answered %T, want docVerifyPage", page)
	}
	return rendered.text
}

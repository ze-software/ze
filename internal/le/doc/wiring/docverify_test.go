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

// pageText answers the rendered text of whatever the stage returned.
func pageText(t *testing.T, page any) string {
	t.Helper()

	rendered, held := page.(docVerifyPage)
	if !held {
		t.Fatalf("the stage answered %T, want docVerifyPage", page)
	}
	return rendered.text
}

// TestTheDocVerifyStageNoLongerJudgesTheRFCLedger is AC-9 for this package.
//
// The five generated RFC files are derived and untracked, so there is no
// committed copy to compare a re-render against. A stage that still compared
// would report every checkout stale for as long as an input sat unrendered,
// and the author would charge a debt row for a file nobody is expected to hold.
func TestTheDocVerifyStageNoLongerJudgesTheRFCLedger(t *testing.T) {
	root := ledgerFixtureTree(t)
	for _, rel := range []string{"ai/RFC-REQUIREMENTS.md", "rfc/enrolled.txt",
		"rfc/not-enrolled.txt", "docs/features/rfc-status.md"} {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("remove %s: %v", rel, err)
		}
	}
	if err := os.RemoveAll(filepath.Join(root, "rfc", "requirements")); err != nil {
		t.Fatalf("remove rfc/requirements: %v", err)
	}

	page, code := discoveryIndexesStage(root)
	text := pageText(t, page)
	for _, rel := range []string{"ai/RFC-REQUIREMENTS.md", "rfc/requirements", "rfc/enrolled.txt",
		"rfc/not-enrolled.txt", "docs/features/rfc-status.md"} {
		if strings.Contains(text, rel) {
			t.Errorf("the stage still judges %s, which is derived and untracked:\n%s", rel, text)
		}
	}
	if code != 0 {
		t.Errorf("the stage exits %d over a tree holding no generated RFC file:\n%s", code, text)
	}
}

package rfc

// VALIDATES: each summary's lineage cells agree with the RFC headers (AC-14).
// PREVENTS: a Meta link the header does not carry ordering the tree wrongly.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc/graph"
)

// TestCheckRefusesMetaLinkHeaderMismatch holds checkHeaderLinks to AC-14: a
// cell that disagrees with the header is refused naming both values, a cell
// that agrees passes, and an RFC outside the corpus is free in `Updated by`.
// Method: a five-text corpus and one Meta map per case.
func TestCheckRefusesMetaLinkHeaderMismatch(t *testing.T) {
	g := graph.Build(map[string]graph.Text{
		"rfc4271": {Obsoletes: []string{"rfc1771"}},
		"rfc7606": {Updates: []string{"rfc4271"}},
		"rfc3768": {},
		"rfc5798": {Obsoletes: []string{"rfc3768"}},
		"rfc9568": {Obsoletes: []string{"rfc5798"}},
	})
	cases := []struct {
		name    string
		metas   map[string]Meta
		refused []string
	}{
		{
			name: "every cell agrees",
			metas: map[string]Meta{
				"rfc4271": {Obsoletes: []string{"rfc1771"}, UpdatedBy: []string{"rfc7606", "rfc9999"}},
				"rfc7606": {Updates: []string{"rfc4271"}},
				"rfc3768": {},
				"rfc5798": {Obsoletes: []string{"rfc3768"}},
				"rfc9568": {Obsoletes: []string{"rfc5798"}},
			},
		},
		{
			name:    "Updates names an RFC the header does not",
			metas:   map[string]Meta{"rfc7606": {Updates: []string{"rfc4271", "rfc4760"}}},
			refused: []string{"rfc/short/rfc7606.md", "`Updates`", "RFC 4271, RFC 4760", "says RFC 4271"},
		},
		{
			name:    "Obsoletes omits the header's",
			metas:   map[string]Meta{"rfc4271": {UpdatedBy: []string{"rfc7606"}}},
			refused: []string{"rfc/short/rfc4271.md", "`Obsoletes` cell names None", "says RFC 1771"},
		},
		{
			name:  "Updated by may omit a corpus updater: the graph holds the reverse relation",
			metas: map[string]Meta{"rfc4271": {Obsoletes: []string{"rfc1771"}}},
		},
		{
			name:    "Updated by names a corpus RFC whose header does not update it",
			metas:   map[string]Meta{"rfc4271": {Obsoletes: []string{"rfc1771"}, UpdatedBy: []string{"rfc7606", "rfc9568"}}},
			refused: []string{"`Updated by` cell names RFC 7606, RFC 9568", "says RFC 7606"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := checkHeaderLinks(tc.metas, g)
			if len(tc.refused) == 0 {
				if len(errs) != 0 {
					t.Fatalf("agreeing cells refused: %q", errs)
				}
				return
			}
			if len(errs) != 1 {
				t.Fatalf("want one refusal, got %d: %q", len(errs), errs)
			}
			for _, want := range tc.refused {
				if !strings.Contains(errs[0], want) {
					t.Errorf("refusal %q omits %q", errs[0], want)
				}
			}
		})
	}
}

// TestCorpusMetaLinksMatchHeaders holds the real corpus to its headers: every
// summary's lineage cells agree with rfc/full/ and rfc/drafts/. This is the
// test the 24 corrections of 2026-10-25 turned green.
func TestCorpusMetaLinksMatchHeaders(t *testing.T) {
	tree, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	collected, err := Collect(tree)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	g, err := graph.Load(tree)
	if err != nil {
		t.Fatalf("graph.Load: %v", err)
	}
	for _, refusal := range checkHeaderLinks(collected.Metas, g) {
		t.Error(refusal)
	}
}

// TestCheckReportsMetaLinkHeaderMismatch is the wiring half of AC-14: `./le
// rfc check` itself refuses a summary whose `Updates` cell names an RFC its
// own text's header does not, and the refusal names both values.
// Method: the check fixture with one `Updates` row planted in its Meta table;
// its text, rfc/full/rfc9999.txt, carries no header lineage.
func TestCheckReportsMetaLinkHeaderMismatch(t *testing.T) {
	planted := strings.Replace(selftestMeta, "| Title | Widgets |\n", "| Title | Widgets |\n| Updates | RFC 4271 |\n", 1)
	root := checkFixtureTree(t, map[string]string{
		selftestSummaryRel: "# RFC 9999\n\n" + planted + "\n## Compliance Checklist\n\n" +
			"- [ ] [" + selftestRIDSend + "] [MUST] A speaker MUST send the widget (§2)\n",
	})
	report, _ := Check(root, nil)
	found := false
	for _, line := range report.Violations {
		if strings.Contains(line, "the Meta `Updates` cell names RFC 4271, but its own header says None") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the check did not refuse the planted Updates cell:\n%s", report.Text())
	}
}

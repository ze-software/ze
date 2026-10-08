package rfc

import (
	"strings"
	"testing"
)

// VALIDATES: checkReferenceSources refuses a summary or an extraction sign-off whose only
// text is a non-normative document under reference/, and a summary that cites a reference/
// path, while accepting a stem whose text the owner enrolled under rfc/full or rfc/drafts.
// PREVENTS: an IETF BCP, Informational RFC or active draft becoming a requirement source
// without the owner moving it into rfc/full/ or rfc/drafts/ (reference/README.md).
// METHOD: one fixture tree per case, built from path to content, judged by the stage alone;
// each refusal is matched on the file it names and on the README it sends the reader to.
func TestCheckReferenceSources(t *testing.T) {
	const (
		stem      = "draft-ietf-idr-widget"
		summary   = "rfc/short/" + stem + ".md"
		extracted = "rfc/extraction/" + stem + ".json"
		reference = "reference/ietf/idr/" + stem + ".txt"
		body      = "# Widget\n\n## Compliance Checklist\n"
	)
	cases := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"summary whose only text is under reference", map[string]string{
			summary: body, reference: "text\n",
		}, []string{summary}},
		{"extraction whose only text is under reference", map[string]string{
			extracted: "{}\n", reference: "text\n",
		}, []string{extracted}},
		{"summary citing a reference path", map[string]string{
			summary:                       body + "Source: `reference/ietf/idr/" + stem + ".txt`\n",
			"rfc/drafts/" + stem + ".txt": "text\n",
		}, []string{summary}},
		{"text enrolled under rfc/drafts beside the reference copy", map[string]string{
			summary: body, extracted: "{}\n", reference: "text\n",
			"rfc/drafts/" + stem + ".txt": "text\n",
		}, nil},
		{"text enrolled under rfc/full beside the reference copy", map[string]string{
			summary: body, reference: "text\n", "rfc/full/" + stem + ".txt": "text\n",
		}, nil},
		{"summary with no reference text and no citation", map[string]string{
			summary: body + "See docs/reference/widget.md for the operator view.\n",
		}, nil},
		{"no reference tree at all", map[string]string{summary: body}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := t.TempDir()
			writeFixtureFiles(t, tree, tc.files)

			stems, err := summaryStems(tree)
			if err != nil {
				t.Fatalf("summary stems: %v", err)
			}
			errs, err := checkReferenceSources(tree, stems)
			if err != nil {
				t.Fatalf("the stage could not read the fixture tree: %v", err)
			}
			if len(errs) != len(tc.want) {
				t.Fatalf("got %d refusal(s), want %d: %v", len(errs), len(tc.want), errs)
			}
			for i, where := range tc.want {
				for _, needle := range []string{where, referenceReadmeRel} {
					if !strings.Contains(errs[i], needle) {
						t.Errorf("refusal %d does not name %q: %s", i, needle, errs[i])
					}
				}
			}
		})
	}
}

// VALIDATES: the public Check entry point runs the reference stage, so a summary citing a
// reference/ path turns the gate red with a violation naming reference/README.md.
// PREVENTS: a stage that is tested in isolation and never wired into the driver.
func TestRFCCheckRefusesAReferenceCitation(t *testing.T) {
	root := checkFixtureTree(t, nil)
	writeFixtureFiles(t, root, map[string]string{
		"reference/ietf/idr/rfc9999.txt": "text\n",
	})
	clean, _ := Check(root, nil)
	for _, violation := range clean.Violations {
		if strings.Contains(violation, referenceReadmeRel) {
			t.Fatalf("a reference copy beside the enrolled rfc/full text was refused: %s", violation)
		}
	}

	extra := map[string]string{selftestSummaryRel: "# RFC 9999\n\nRead reference/ietf/idr/rfc9999.txt.\n\n" +
		selftestMeta + "\n## Compliance Checklist\n\n" +
		"- [ ] [" + selftestRIDSend + "] [MUST] A speaker MUST send the widget (§2)\n"}
	report, code := Check(checkFixtureTree(t, extra), nil)
	if report.CannotRun != "" {
		t.Fatalf("the fixture tree could not run: %s", report.CannotRun)
	}
	if code != 2 {
		t.Fatalf("a summary citing reference/ answered %d, want 2:\n%s", code, report.Text())
	}
	for _, violation := range report.Violations {
		if strings.Contains(violation, referenceReadmeRel) && strings.Contains(violation, selftestSummaryRel) {
			return
		}
	}
	t.Fatalf("no violation names %s and %s:\n%s", selftestSummaryRel, referenceReadmeRel, report.Text())
}

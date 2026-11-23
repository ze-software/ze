package rfc

import (
	"strings"
	"testing"
)

// VALIDATES: checkDeclinedSources refuses any active corpus text (rfc/full or
// rfc/drafts), summary (rfc/short), or extraction sign-off (rfc/extraction)
// whose stem is filed under rfc/declined/, and refuses any active summary that
// cites an rfc/declined/ path, while ignoring rfc/declined/README.md.
// PREVENTS: a declined RFC or draft re-entering the implementation corpus,
// dependency graph, or conformance ledger (rfc/declined/README.md).
// METHOD: one fixture tree per case, built from path to content, judged by the
// stage alone; each refusal is matched on the offending file and on the README
// it sends the reader to.
func TestCheckDeclinedSources(t *testing.T) {
	const (
		stem         = "draft-ietf-idr-bgp-multisession"
		draftCopy    = "rfc/drafts/" + stem + ".txt"
		fullCopy     = "rfc/full/" + stem + ".txt"
		summary      = "rfc/short/" + stem + ".md"
		extracted    = "rfc/extraction/" + stem + ".json"
		declinedText = "rfc/declined/texts/" + stem + ".txt"
		declinedSum  = "rfc/declined/short/" + stem + ".md"
		otherSummary = "rfc/short/rfc4271.md"
		body         = "# Multisession\n\n## Compliance Checklist\n"
	)
	cases := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"clean declined text and summary with no corpus copy", map[string]string{
			"rfc/declined/README.md": "# Declined\n",
			declinedText:             "text\n",
			declinedSum:              "# Declined summary\n",
			otherSummary:             "# RFC 4271\n",
		}, nil},
		{"declined draft also present in rfc/drafts", map[string]string{
			declinedText: "text\n",
			draftCopy:    "text\n",
		}, []string{draftCopy}},
		{"declined RFC also present in rfc/full", map[string]string{
			declinedSum: "# Declined\n",
			fullCopy:    "text\n",
		}, []string{fullCopy}},
		{"declined stem also has an active summary in rfc/short", map[string]string{
			declinedText: "text\n",
			summary:      body,
		}, []string{summary}},
		{"declined stem also has an extraction sign-off", map[string]string{
			declinedText: "text\n",
			extracted:    "{}\n",
		}, []string{extracted}},
		{"active summary citing an rfc/declined path", map[string]string{
			declinedText: "text\n",
			otherSummary: "# RFC 4271\n\nSee `rfc/declined/texts/" + stem + ".txt`.\n",
		}, []string{otherSummary}},
		{"no declined directory at all", map[string]string{
			otherSummary: "# RFC 4271\n",
		}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := t.TempDir()
			writeFixtureFiles(t, tree, tc.files)

			stems, err := summaryStems(tree)
			if err != nil {
				t.Fatalf("summary stems: %v", err)
			}
			errs, err := checkDeclinedSources(tree, stems)
			if err != nil {
				t.Fatalf("the stage could not read the fixture tree: %v", err)
			}
			if len(errs) != len(tc.want) {
				t.Fatalf("got %d refusal(s), want %d: %v", len(errs), len(tc.want), errs)
			}
			for i, where := range tc.want {
				for _, needle := range []string{where, declinedReadmeRel} {
					if !strings.Contains(errs[i], needle) {
						t.Errorf("refusal %d does not name %q: %s", i, needle, errs[i])
					}
				}
			}
		})
	}
}

// VALIDATES: the public Check entry point runs the declined stage, so a stem
// filed under rfc/declined/ that also sits under rfc/drafts/ turns the gate
// red with a violation naming rfc/declined/README.md.
// PREVENTS: a stage that is tested in isolation and never wired into the driver.
func TestRFCCheckRefusesDeclinedInDrafts(t *testing.T) {
	const (
		stem         = "draft-ietf-idr-bgp-multisession"
		draftCopy    = "rfc/drafts/" + stem + ".txt"
		declinedText = "rfc/declined/texts/" + stem + ".txt"
	)
	cleanRoot := checkFixtureTree(t, nil)
	writeFixtureFiles(t, cleanRoot, map[string]string{
		declinedText: "text\n",
	})
	clean, _ := Check(cleanRoot, nil)
	for _, violation := range clean.Violations {
		if strings.Contains(violation, declinedReadmeRel) {
			t.Fatalf("a declined text alone was refused: %s", violation)
		}
	}

	dupRoot := checkFixtureTree(t, nil)
	writeFixtureFiles(t, dupRoot, map[string]string{
		declinedText: "text\n",
		draftCopy:    "1.  Introduction\n\n   Text.\n\n2.  References\n\n   [RFC4271]\n",
	})
	report, code := Check(dupRoot, nil)
	if report.CannotRun != "" {
		t.Fatalf("the fixture tree could not run: %s", report.CannotRun)
	}
	if code != 1 {
		t.Fatalf("a declined draft also in rfc/drafts answered %d, want 1:\n%s", code, report.Text())
	}
	for _, violation := range report.Violations {
		if strings.Contains(violation, declinedReadmeRel) && strings.Contains(violation, draftCopy) {
			return
		}
	}
	t.Fatalf("no violation names %s and %s:\n%s", draftCopy, declinedReadmeRel, report.Text())
}

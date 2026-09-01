// Design: website/AI.md -- the RFC compliance report is internal/le/rfc's own answer
package site

import (
	"encoding/json"
	"html"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// rfcCompliancePaths lays out one checkout whose gate answer is the snapshot
// the published page was rendered from.
//
// The snapshot is stated rather than derived: a live read answers today's
// requirement counts, so a golden page held against one would move with every
// RFC row somebody edits. The fixture is the published data/rfc-compliance.json
// with its keys re-spelled kebab-case, which is this repository's JSON
// convention; no value is changed and the presentation fields the renderer now
// supplies itself are removed.
func rfcCompliancePaths(t *testing.T) Paths {
	t.Helper()
	return rfcCompliancePathsFrom(t, publishedRFCComplianceRef(t))
}

// publishedRFCCompliance answers the published snapshot, with the verification
// wiring taken from the live stage population.
//
// The published file carries no wiring block: the retired renderer answered
// that question by grepping three files that no longer exist. So the historical
// numbers come from the fixture and the wiring comes from the registry, which
// is what the page now publishes.
func publishedRFCCompliance(t *testing.T) rfcCompliance {
	t.Helper()
	var snapshot rfcCompliance
	if err := json.Unmarshal([]byte(readFixture(t, "published-rfc-compliance.json")), &snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot.Verify = rfcGateStages()
	return snapshot
}

// rfcCompliancePathsFrom lays out a checkout whose gate answer is stated, with
// a two-stem requirement ledger already published beside it.
//
// The producer reads the ledger back out of the artifact, so a test that states
// a gate snapshot has to state a ledger too. Two stems is the smallest input
// that fills both link tables.
func rfcCompliancePathsFrom(t *testing.T, snapshot *rfcCompliance) Paths {
	t.Helper()
	return rfcCompliancePathsWith(t, snapshot, twoStemLedger())
}

// rfcCompliancePathsWith lays out a checkout whose gate answer and requirement
// ledger are both stated.
func rfcCompliancePathsWith(t *testing.T, snapshot *rfcCompliance, ledger rfcLedger) Paths {
	t.Helper()
	root := repositoryRoot(t)
	source := t.TempDir()
	copyFixture(t, filepath.Join(root, "website", "data", "page-links.json"),
		filepath.Join(source, "data", "page-links.json"))

	previous := liveRFCCompliance
	t.Cleanup(func() { liveRFCCompliance = previous })
	liveRFCCompliance = func(string) (rfcCompliance, error) { return *snapshot, nil }

	output := t.TempDir()
	writeLedgerArtifact(t, output, ledger)
	return Paths{Repository: root, Source: source, Output: output}
}

// writeLedgerArtifact publishes one stated ledger where the producer reads it.
func writeLedgerArtifact(t *testing.T, output string, ledger rfcLedger) {
	t.Helper()
	content, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := writeNamedArtifact(output, rfcLedgerFile, string(content)+"\n"); err != nil {
		t.Fatal(err)
	}
}

// twoStemLedger is one enrolled summary and one declined one, which is the
// smallest ledger that fills both of the index's link tables.
func twoStemLedger() rfcLedger {
	return rfcLedger{Stems: []rfcLedgerStem{
		{
			Stem: "rfc9998", Display: "RFC 9998", Title: "The Widget Protocol",
			Enrolled: true, EnrolmentReason: "every MUST carries both polarities",
			PublicStatus: "Supported", PublicRemaining: "RFC9998-2-2 unmet",
			SummaryPath: "rfc/short/rfc9998.md", ShardPath: "rfc/requirements/rfc9998.md",
			Coverage: rfcLedgerCoverage{Requirements: 1, Gated: 1, Both: 1},
			Requirements: []rfcLedgerRequirement{{
				RID: "RFC9998-2-1", Level: "MUST", Section: "2", Text: "A widget MUST be sent.",
				Gated: true, Positive: "`internal/a_test.go` `TestWidget` (unit/verify)",
				Negative: "`internal/a_test.go` `TestNoWidget` (unit/verify)",
				Covers: []rfcLedgerCover{
					{Polarity: "positive", Unit: "internal/a_test.go::TestWidget",
						Carrier: "unit/verify", Tags: 1},
					{Polarity: "negative", Unit: "internal/a_test.go::TestNoWidget",
						Carrier: "unit/verify", Tags: 1},
				},
			}},
		},
		{
			Stem: "rfc9997", Display: "RFC 9997", Title: "Widget Considerations",
			Disposition: &rfcLedgerDisposition{Kind: "backlog",
				Reason: "the extraction is owed"},
			SummaryPath: "rfc/short/rfc9997.md",
		},
	}}
}

// textBetween answers the visible text between two markers, failing when either
// marker is absent so a renamed section cannot silently empty a comparison.
func textBetween(t *testing.T, text, from, to string) string {
	t.Helper()
	start := strings.Index(text, from)
	if start < 0 {
		t.Fatalf("the text carries no %q", from)
	}
	rest := text[start+len(from):]
	end := strings.Index(rest, to)
	if end < 0 {
		t.Fatalf("the text carries no %q after %q", to, from)
	}
	return strings.TrimSpace(rest[:end])
}

// rfcCompliancePage renders the page and answers its <main> visible text.
func rfcCompliancePage(t *testing.T, paths Paths) (string, string) {
	t.Helper()
	routes, err := renderRFCCompliance(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) == 0 || routes[0] != rfcComplianceRoute {
		t.Fatalf("the producer claimed %v, want %s first", routes, rfcComplianceRoute)
	}
	page := readArtifact(t, paths.Output, rfcComplianceDest)
	return page, visibleText(mainContent(t, page))
}

// VALIDATES: the three sections whose inputs did not change read as the
// published page reads.
//
// The requirement buckets, the gap disclosure and the gap clusters are derived
// from rfc/short/*.md, rfc/enrolled.txt, the test tags and the public ledger,
// all of which the retired renderer read too. The method is to render the
// published snapshot and hold each section against the published page.
func TestTheRFCComplianceSectionsReadAsThePublishedPage(t *testing.T) {
	paths := rfcCompliancePaths(t)
	page, text := rfcCompliancePage(t, paths)
	// The retired page wrote a row as one line, so text extraction read the
	// last word of a cell and the first word of the next as one word
	// ("Partial59"). Every table of this family now goes through
	// rfcTableHTML, whose rows put each cell on its own line, so the same
	// content extracts as "Partial 59". Re-spacing the fixture's cells the
	// way the family spaces them holds the CONTENT of the section against
	// the published page without freezing markup the family convention
	// deliberately changed. Nothing else is normalized: every character, in
	// order, and every other space still has to agree.
	published := visibleText(strings.ReplaceAll(
		readFixture(t, "published-rfc-compliance-body.html"), "</td>", "</td>\n"))

	for _, chrome := range []string{
		"<title>RFC Compliance Gate Report - Ze</title>",
		`<link rel="canonical" href="https://ze-software.net/quality/rfc-compliance/" />`,
		`<link rel="stylesheet" href="../../assets/site.css" />`,
		`<main id="top" class="has-page-sidebar" tabindex="-1">`,
		`<section aria-labeledby="rfc-compliance-title" class="md-content reveal cat-observe">`,
		`<h1 id="rfc-compliance-title">RFC Compliance Gate Report</h1>`,
		`<aside class="page-sidebar" aria-label="Related page links">`,
		"<footer>",
	} {
		if !strings.Contains(page, chrome) {
			t.Errorf("the RFC compliance page is missing %q", chrome)
		}
	}

	// The requirement-buckets section is NOT compared here any more. Its output
	// changed by decision on 2026-09-01: the tape carries no text of its own,
	// the words moved to a key beneath it, and the table gained the total row
	// that proves the buckets account for the whole gated population. A frozen
	// comparison against the retired page would refuse the change it was asked
	// for. What that section owes instead is
	// TestTheBucketTableAccountsForEveryGatedRequirement, which checks the
	// arithmetic rather than the bytes.
	for _, section := range []struct{ name, from, mine, theirs string }{
		// The exclusion disclosure was inserted between the two on 2026-09-01,
		// so this section now ends at that heading rather than at the clusters.
		// What it reads is unchanged, which is what the comparison holds.
		{"gap disclosure", "Gap disclosure", "Exclusion disclosure", "Top gap clusters"},
		// The Gate inputs section folded into How this is checked on 2026-09-01:
		// a mechanism belongs in one place, and it was in two.
		{"top gap clusters", "Top gap clusters", "How this is checked", "AI guard and gate inputs"},
	} {
		got := textBetween(t, text, section.from, section.mine)
		want := textBetween(t, published, section.from, section.theirs)
		if got != want {
			t.Errorf("the %s section reads as\n  %q\nthe published one reads as\n  %q",
				section.name, got, want)
		}
	}
}

// VALIDATES: the section names the heading that labels it.
func TestTheRFCCompliancePageIsLabelledByItsOwnHeading(t *testing.T) {
	paths := rfcCompliancePaths(t)
	page, _ := rfcCompliancePage(t, paths)

	if !strings.Contains(page, `aria-labeledby="rfc-compliance-title"`) {
		t.Error("the section carries no aria-labeledby")
	}
	if !strings.Contains(page, `id="rfc-compliance-title"`) {
		t.Error(`aria-labeledby names rfc-compliance-title, which the page does not carry`)
	}
	if !strings.Contains(page, `<div class="rfc-tape" role="img" aria-label="How the obligations that bind Ze are answered">`) {
		t.Error("the satisfaction tape carries no label, so a screen reader meets an unnamed image")
	}
}

// VALIDATES: the four headline cards whose inputs did not change carry the
// numbers the published cards carry.
func TestTheHeadlineCardsCarryThePublishedNumbers(t *testing.T) {
	paths := rfcCompliancePaths(t)
	_, text := rfcCompliancePage(t, paths)

	// The ledger behind this page is twoStemLedger: two summaries, one
	// requirement between them. That is what the Gated MUSTs card names as the
	// population the gate holds a subset of.
	// A RATIO leads and a POPULATION follows. The gated count is still on the
	// page, below them, as the accounting total it is.
	// Each card carries its label, its value, the arithmetic behind the value,
	// and what the measure means, in that order.
	for _, card := range rfcComplianceCards(publishedRFCComplianceRef(t), twoStemLedger()) {
		if !strings.Contains(text, card.Label+card.Value+card.Count+card.Note) {
			t.Errorf("the card grid does not read %q / %q / %q / %q",
				card.Label, card.Value, card.Count, card.Note)
		}
	}
	for _, figure := range []string{
		"Gated MUSTs2,966", "Out of scope839",
		"Tested both ways58.3%1,239 of 2,127 binding obligations",
		"No test at all24.4%518 of 2,127 binding obligations",
		"One polarity plus reason17.4%370 of 2,127 binding obligations",
	} {
		if !strings.Contains(text, figure) {
			t.Errorf("the card grid does not read %q", figure)
		}
	}
	if !strings.Contains(text, "rfc-requirements OK: 2966 gated MUST-level requirement(s) "+
		"across 171 enrolled RFC(s); 3595 test tag(s) resolved.") {
		t.Error("the page does not publish the gate's own summary line")
	}
}

// VALIDATES: the guard block is answered by the live verification stages rather
// than by counting text in three deleted files.
//
// The retired renderer grepped .claude/hooks/pretool-writeedit.py, the Makefile
// and scripts/status/verify_run.go. All three are gone, and the approval token
// it advertised was retired by owner ruling on 2026-08-19, so a port would
// publish a mechanism this repository bans.
func TestTheGuardBlockIsAnsweredByTheVerifyStages(t *testing.T) {
	paths := rfcCompliancePaths(t)
	page, text := rfcCompliancePage(t, paths)

	for _, retired := range []string{
		"rfc-test-change-approved",
		".claude/hooks/pretool-writeedit.py",
		"scripts/dev/rfc_requirements.py",
		"scripts/status/verify_run.go",
		"AI test guard",
	} {
		if strings.Contains(page, retired) {
			t.Errorf("the page still publishes %q, which no longer exists in this repository", retired)
		}
	}
	for _, live := range []string{
		"internal/le/verify/engine/stages.go",
		"./le rfc check",
	} {
		if !strings.Contains(text, live) {
			t.Errorf("the page does not name %q, so the guard row states nothing derived", live)
		}
	}
}

// VALIDATES: the five gate-input rows whose producers did not change read as
// the published rows.
// VALIDATES: AC-44 -- the mechanism sits in ONE place, out of the card grid.
//
// `Pre-commit gate / ON` was a card until 2026-09-01, beside counts and ratios.
// It is not a measure: it answers "how is this enforced" rather than "where
// does Ze stand", and so do the stage count, the reproduce command and the
// input paths. The fact is kept; it just is not a card.
func TestTheGateInputRowsReadAsThePublishedRows(t *testing.T) {
	paths := rfcCompliancePaths(t)
	_, text := rfcCompliancePage(t, paths)

	for _, row := range []string{
		"Requirement source rfc/short/*.md 2,966 gated MUST-level requirements",
		"Enrolment rfc/short/*.md, the | Enrolment | Meta row 171 enrolled RFCs",
		"Test tags internal/, pkg/, test/ 3,595 resolved tags",
		"Public ledger rfc/short/*.md, the | Support | Meta row 80 RFCs with gaps, 4 Supported with Remaining",
		"Semantic audits rfc/audit/*.json 52 fresh, 0 shifted, 0 stale, 2,914 missing",
	} {
		if !strings.Contains(text, row) {
			t.Errorf("the mechanism table does not read %q", row)
		}
	}
	if !strings.Contains(text, "This gate runs before a commit is verified") {
		t.Error("the page dropped the fact the retired Pre-commit gate card carried")
	}
	for _, card := range rfcComplianceCards(publishedRFCComplianceRef(t), twoStemLedger()) {
		if card.Label == "Pre-commit gate" {
			t.Error("the mechanism is back in the card grid, beside counts and ratios")
		}
	}
}

// VALIDATES: AC-45 -- the grid reads in four movements, and a card's movement
// and its color cannot disagree.
//
// A reader met eight cards in one block and had to work out which numbers were
// good news. The heading says it now, and membership is DERIVED from the tone,
// so "What Ze owes" holds a red card because that is the only way in.
func TestTheCardGridReadsInFourMovements(t *testing.T) {
	paths := rfcCompliancePaths(t)
	_, text := rfcCompliancePage(t, paths)
	entry := disclosureLedger().Stems[0]

	for _, heading := range []string{"Overall", "Positive", "Negative"} {
		if !strings.Contains(text, heading) {
			t.Errorf("the grid publishes no %q movement", heading)
		}
	}
	for name, cards := range map[string][]rfcCard{
		"index":  rfcComplianceCards(publishedRFCComplianceRef(t), twoStemLedger()),
		"detail": rfcDetailCards(&entry),
	} {
		placed := map[string]int{}
		for _, group := range rfcCardGroups {
			for _, card := range rfcCardsIn(cards, group.Tone, group.Overall) {
				placed[card.Label]++
				if card.Tone != group.Tone {
					t.Errorf("the %s %s card is in %q and reads %q", name, card.Label,
						group.Heading, card.Tone)
				}
			}
		}
		for _, card := range cards {
			if placed[card.Label] != 1 {
				t.Errorf("the %s %s card lands in %d movements, want exactly one",
					name, card.Label, placed[card.Label])
			}
		}
	}
}

// VALIDATES: the check section publishes the gate's own open issues, and says
// so when there are none.
//
// The Go gate answers one list rather than a count for each check, so the
// published table of ten named checks has no source. A table of ten zeros
// nobody could derive would be a number nobody can trace (ai/rules/evidence.md).
func TestTheCheckSectionPublishesTheGatesOwnIssues(t *testing.T) {
	green := rfcCompliancePaths(t)
	_, text := rfcCompliancePage(t, green)
	if !strings.Contains(text, "No open issue.") {
		t.Error("a green gate does not say it has no open issue")
	}

	snapshot := publishedRFCCompliance(t)
	snapshot.Gate.OK = false
	snapshot.Gate.ErrorCount = 2
	snapshot.Gate.Findings = []rfc.Finding{
		{Message: "rfc/short/rfc9999.md:12: RFC9999-2-1 [MUST] has no test and no annotation: a widget",
			Where: "rfc/short/rfc9999.md:12", RID: "RFC9999-2-1", Level: "MUST", Section: "2",
			Text: "A widget MUST be sent.", Issue: "has no test and no annotation"},
		{Message: "docs/features/rfc-status.md: rfc9998 has no public row"},
	}
	snapshot.Gate.Violations = findingMessagesOf(snapshot.Gate.Findings)
	red := rfcCompliancePathsFrom(t, &snapshot)
	redPage, redText := rfcCompliancePage(t, red)

	// The columns come from the finding's own parts, never from parsing the
	// message back apart.
	for _, cell := range []string{
		"RFC 9999", "RFC9999-2-1", "MUST", "has no test and no annotation",
		"A widget MUST be sent.",
	} {
		if !strings.Contains(redText, cell) {
			t.Errorf("the check table does not carry %q", cell)
		}
	}
	if !strings.Contains(redPage,
		`<a href="rfc9999/#rfc9999-2-1"><code>RFC9999-2-1</code></a>`) {
		t.Error("the requirement is not linked to its own page and row")
	}
	// A finding about no requirement states itself in the one column it fills.
	if !strings.Contains(redText, "rfc9998 has no public row") {
		t.Error("a finding carrying no requirement is not published")
	}
	if strings.Contains(redPage, "<ul>\n<li class=\"rfc-check-bad\"") {
		t.Error("the check section is still a bullet list")
	}
	if !strings.Contains(redText, "Gate verdictRED2 open gate issues") {
		t.Error("a red gate does not read as red on its card")
	}
}

// findingMessagesOf renders the lines the gate prints, the way the gate does.
func findingMessagesOf(findings []rfc.Finding) []string {
	out := make([]string, 0, len(findings))
	for _, finding := range findings {
		out = append(out, finding.Message)
	}
	return out
}

// VALIDATES: a long list of open issues is bounded rather than published whole.
//
// A gate that goes red on a bad merge can answer thousands of diagnostics, and
// a page that inlined all of them would be megabytes of one build's noise.
func TestALongIssueListIsBoundedAndSaysHowManyItLeftOut(t *testing.T) {
	snapshot := publishedRFCCompliance(t)
	snapshot.Gate.OK = false
	for issue := range rfcIssuesShown + 5 {
		snapshot.Gate.Findings = append(snapshot.Gate.Findings,
			rfc.Finding{Message: "issue " + string(rune('a'+issue)),
				Issue: "issue " + string(rune('a'+issue))})
	}
	snapshot.Gate.Violations = findingMessagesOf(snapshot.Gate.Findings)
	snapshot.Gate.ErrorCount = len(snapshot.Gate.Findings)

	_, text := rfcCompliancePage(t, rfcCompliancePathsFrom(t, &snapshot))
	if strings.Count(text, "issue ") > rfcIssuesShown {
		t.Errorf("the page publishes %d issues, want at most %d",
			strings.Count(text, "issue "), rfcIssuesShown)
	}
	if !strings.Contains(text, "5 further findings not shown here") {
		t.Error("the page does not say how many findings it left out")
	}
}

// VALIDATES: the mirror states every number the page states.
func TestTheRFCComplianceMirrorReadsAsThePublishedMirror(t *testing.T) {
	paths := rfcCompliancePaths(t)
	if _, err := renderRFCCompliance(paths); err != nil {
		t.Fatal(err)
	}
	mirror := readArtifact(t, paths.Output, "quality/rfc-compliance/"+pageMirrorFile)

	for _, line := range []string{
		"# RFC Compliance Gate Report",
		"| Gated MUST-level requirements | 2,966 |",
		"| Enrolled RFCs | 171 |",
		"| Resolved test tags | 3,595 |",
		"| Declared gaps | 518 |",
		"| RFCs with declared gaps | 80 |",
		"| Positive and negative tests | 1,239 | 58.3% | `positive tag + negative tag` |",
		"| Partial | 59 |",
		"- **RFC 1350:** RFC1350-2-3 unmet",
		"| `DRAFT-IETF-BESS-MUP-SAFI` | 37 | Partial |",
		"| Semantic audits | `rfc/audit/*.json` | 52 fresh, 0 shifted, 0 stale, 2,914 missing |",
	} {
		if !strings.Contains(mirror, line) {
			t.Errorf("the mirror does not carry %q", line)
		}
	}
	if strings.Contains(mirror, "rfc-test-change-approved") {
		t.Error("the mirror publishes the retired approval token")
	}
}

// VALIDATES: the snapshot the build publishes beside the page is the one the
// page was rendered from.
func TestTheComplianceSnapshotIsPublishedBesideThePage(t *testing.T) {
	paths := rfcCompliancePaths(t)
	if _, err := renderRFCCompliance(paths); err != nil {
		t.Fatal(err)
	}

	published := readArtifact(t, paths.Output, rfcComplianceSnapshot)
	var round rfcCompliance
	if err := json.Unmarshal([]byte(published), &round); err != nil {
		t.Fatalf("the published snapshot is not JSON: %v", err)
	}
	if round.Gate.GatedMust != 2966 || round.Gaps.Requirements != 518 {
		t.Errorf("the published snapshot states %d gated MUSTs and %d gaps, want 2966 and 518",
			round.Gate.GatedMust, round.Gaps.Requirements)
	}
}

// VALIDATES: the producer claims the one route it publishes, and that route is
// one the site publishes.
func TestTheRFCComplianceProducerClaimsItsPublishedRoute(t *testing.T) {
	paths := rfcCompliancePaths(t)
	routes, err := renderRFCCompliance(paths)
	if err != nil {
		t.Fatal(err)
	}
	published := map[string]bool{}
	for _, route := range publishedArtifactRoutes(t) {
		published[route] = true
	}
	if !published[routes[0]] {
		t.Errorf("the producer claims %s, which the published site does not carry", routes[0])
	}
	// The detail routes are this family's own, published for the first time by
	// plan/spec-publish-the-rfc-requirement-ledger.md, so the frozen route list
	// of the retired renderer cannot carry them. What it can still say is that
	// the index route is one the site publishes, and that every other route is
	// a child of it.
	for _, route := range routes[1:] {
		if !strings.HasPrefix(route, rfcComplianceRoute) || !strings.HasSuffix(route, "/") {
			t.Errorf("the producer claims %s, which is not a child of %s", route, rfcComplianceRoute)
		}
	}
}

// VALIDATES: every annotation kind the corpus carries has a bucket on the page.
//
// The bucket keys spell three annotation kinds that internal/le/rfc parses and
// does not export. A fourth kind would otherwise fall into the tag-derived
// buckets and be published as an unexcused requirement, which reads as a
// compliance claim the summaries do not make (ai/rules/rfc-compliance.md). The
// method is to ask the parser what kinds this tree actually holds.
func TestEveryAnnotationKindTheCorpusCarriesHasABucket(t *testing.T) {
	collected, err := rfc.Collect(repositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, requirement := range collected.Requirements {
		if requirement.Annotation == nil {
			continue
		}
		seen[requirement.Annotation.Kind] = true
	}
	if len(seen) == 0 {
		t.Fatal("no requirement of this tree carries an annotation, so this proves nothing")
	}
	buckets := map[string]bool{}
	for _, bucket := range rfcSatisfaction {
		buckets[bucket.Key] = true
	}
	for kind := range seen {
		bucket, named := rfcAnnotationBuckets[kind]
		if !named {
			t.Errorf("annotation kind %q falls in no bucket, so the page counts it as unexcused", kind)
			continue
		}
		if !buckets[bucket] {
			t.Errorf("annotation kind %q maps to bucket %q, which the page does not list", kind, bucket)
		}
	}
}

// VALIDATES: the snapshot this checkout answers agrees with itself.
//
// A golden page cannot hold the live gate, whose counts move with every RFC row
// somebody edits, so the collector is held against its own arithmetic instead:
// the buckets have to account for every gated requirement, the gap count has to
// be the gap bucket, and a verdict is either counted or missing.
func TestTheGateSnapshotAgreesWithItself(t *testing.T) {
	snapshot, err := collectRFCCompliance(repositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}

	counted := 0
	gaps := 0
	for _, bucket := range snapshot.Satisfaction {
		counted += bucket.Count
		if bucket.Key == "gap" {
			gaps = bucket.Count
		}
	}
	if counted != snapshot.Gate.GatedMust {
		t.Errorf("the buckets account for %d requirements, the gate gated %d", counted, snapshot.Gate.GatedMust)
	}
	if gaps != snapshot.Gaps.Requirements {
		t.Errorf("the gap bucket counts %d, the gap disclosure counts %d", gaps, snapshot.Gaps.Requirements)
	}
	if snapshot.Gaps.RFCs == 0 || snapshot.Gate.Enrolled == 0 || snapshot.Gate.TestTags == 0 {
		t.Errorf("the snapshot states %d gap RFCs, %d enrolled and %d tags; a zero here is a reader that found nothing",
			snapshot.Gaps.RFCs, snapshot.Gate.Enrolled, snapshot.Gate.TestTags)
	}
	if snapshot.Audit.Verdicts+snapshot.Audit.Missing != snapshot.Gate.GatedMust {
		t.Errorf("the audit counts %d verdicts and %d missing, against %d gated",
			snapshot.Audit.Verdicts, snapshot.Audit.Missing, snapshot.Gate.GatedMust)
	}
	if snapshot.Audit.Fresh+snapshot.Audit.Shifted+snapshot.Audit.Stale != snapshot.Audit.Verdicts {
		t.Errorf("the audit states %d fresh, %d shifted and %d stale, against %d verdicts",
			snapshot.Audit.Fresh, snapshot.Audit.Shifted, snapshot.Audit.Stale, snapshot.Audit.Verdicts)
	}

	clusters := snapshot.Gaps.TopRFCs
	for index := 1; index < len(clusters); index++ {
		if clusters[index-1].Count < clusters[index].Count {
			t.Errorf("the gap clusters are not ordered by count: %s has %d, %s has %d",
				clusters[index-1].RFC, clusters[index-1].Count, clusters[index].RFC, clusters[index].Count)
		}
	}
	if snapshot.Verify.GateStages == 0 {
		t.Error("no full-mode verification stage runs the RFC gate, so the page would say OFF")
	}
	if snapshot.Verify.Command != "./le rfc check" {
		t.Errorf("the verification stage spells the gate %q, want ./le rfc check", snapshot.Verify.Command)
	}
	if !strings.HasPrefix(snapshot.Gate.Message, "rfc-requirements") {
		t.Errorf("the gate message is %q, which is not the gate's own line", snapshot.Gate.Message)
	}
}

// VALIDATES: the gap clusters keep their parse order when two RFCs tie.
//
// The retired renderer took the twelve heaviest from a counter whose ties broke
// on insertion order, and insertion order was the order the summaries parse in.
// A Go map iterates randomly, so a page that ranked from one would publish a
// different table on every build.
func TestTwoRFCsTyingOnGapsKeepTheParseOrder(t *testing.T) {
	gated := []rfc.Requirement{
		{RFC: "rfc900", RID: "RFC900-1-1", Level: "MUST", Annotation: &rfc.Annotation{Kind: "gap"}},
		{RFC: "rfc100", RID: "RFC100-1-1", Level: "MUST", Annotation: &rfc.Annotation{Kind: "gap"}},
		{RFC: "rfc500", RID: "RFC500-1-1", Level: "MUST", Annotation: &rfc.Annotation{Kind: "gap"}},
	}
	rows := map[string]rfc.LedgerRow{
		"rfc900": {Status: "Partial"}, "rfc100": {Status: "Partial"}, "rfc500": {Status: "Partial"},
	}
	for attempt := range 8 {
		_, gaps := rfcBuckets(gated, nil, rows)
		var order []string
		for _, cluster := range gaps.TopRFCs {
			order = append(order, cluster.RFC)
		}
		want := []string{"RFC 900", "RFC 100", "RFC 500"}
		if strings.Join(order, " ") != strings.Join(want, " ") {
			t.Fatalf("attempt %d ordered the tie %v, want %v", attempt, order, want)
		}
	}
}

// VALIDATES: an RFC declaring a gap with no row on the public page is counted
// as missing rather than as a blank status.
//
// A blank cell reads as "nothing to disclose", which is the opposite of what an
// undisclosed gap means.
func TestAGapWithNoPublicRowIsCountedAsMissing(t *testing.T) {
	gated := []rfc.Requirement{
		{RFC: "rfc900", RID: "RFC900-1-1", Level: "MUST", Annotation: &rfc.Annotation{Kind: "gap"}},
	}
	_, gaps := rfcBuckets(gated, nil, map[string]rfc.LedgerRow{})
	if len(gaps.StatusCounts) != 1 || gaps.StatusCounts[0].Status != rfcMissingRow {
		t.Fatalf("an undisclosed gap counts as %v, want one %q row", gaps.StatusCounts, rfcMissingRow)
	}
	if len(gaps.TopRFCs) != 1 || gaps.TopRFCs[0].Status != rfcMissingRow {
		t.Errorf("the cluster row states %v, want %q", gaps.TopRFCs, rfcMissingRow)
	}
}

// VALIDATES: each gated requirement falls in the bucket its evidence puts it
// in, and an annotation outranks the tags.
//
// A requirement with one tagged polarity counted as a pair would publish a
// compliance claim the summaries do not make, and every gate ratchet would
// still be green (ai/rules/rfc-compliance.md).
func TestEachGatedRequirementFallsInTheBucketItsEvidencePutsItIn(t *testing.T) {
	gated := []rfc.Requirement{
		{RFC: "rfc100", RID: "pair", Level: "MUST"},
		{RFC: "rfc100", RID: "one-side", Level: "MUST"},
		{RFC: "rfc100", RID: "untested", Level: "MUST"},
		{RFC: "rfc100", RID: "excused", Level: "MUST", Annotation: &rfc.Annotation{Kind: "single-polarity"}},
		{RFC: "rfc100", RID: "outside", Level: "MUST", Annotation: &rfc.Annotation{Kind: "not-applicable"}},
		{RFC: "rfc100", RID: "hole", Level: "MUST", Annotation: &rfc.Annotation{Kind: "gap"}},
		// An annotated requirement that also carries both tags stays in its
		// annotation's bucket: the annotation is the author's own statement.
		{RFC: "rfc100", RID: "pair", Level: "MUST", Annotation: &rfc.Annotation{Kind: "gap"}},
	}
	tags := []rfc.Tag{
		{RID: "pair", Polarity: "positive"}, {RID: "pair", Polarity: "negative"},
		{RID: "one-side", Polarity: "positive"},
	}
	buckets, _ := rfcBuckets(gated, tags, map[string]rfc.LedgerRow{"rfc100": {Status: "Partial"}})

	want := map[string]int{
		"both_polarities": 1, "single_polarity": 1, "not_applicable": 1,
		"gap": 2, "one_polarity_unexcused": 1, "missing_unexcused": 1,
	}
	for _, bucket := range buckets {
		if bucket.Count != want[bucket.Key] {
			t.Errorf("bucket %s counts %d, want %d", bucket.Key, bucket.Count, want[bucket.Key])
		}
	}
	if len(buckets) != len(rfcSatisfaction) {
		t.Errorf("the page states %d buckets, want %d", len(buckets), len(rfcSatisfaction))
	}
}

// VALIDATES: the gap disclosure lists a public status in the page's own order,
// puts an unrecognized one after it in name order, and bounds the clusters.
//
// The status order is a reading order, not the ledger's: Partial is the answer
// an operator meets most, and it comes first.
func TestTheGapDisclosureOrdersStatusesAndBoundsTheClusters(t *testing.T) {
	var gated []rfc.Requirement
	rows := map[string]rfc.LedgerRow{}
	// The two Supported stems are declared newest first, so a disclosure that
	// kept the parse order would publish them in the opposite order.
	statuses := []string{"Unsupported", "Not supported", "Supported", "Experimental", "Partial", "Zebra"}
	for index, status := range statuses {
		stem := "rfc" + strconv.Itoa(300-index)
		rows[stem] = rfc.LedgerRow{Status: status, Remaining: "one MUST short"}
		gated = append(gated, rfc.Requirement{RFC: stem, RID: stem + "-1",
			Level: "MUST", Annotation: &rfc.Annotation{Kind: "gap"}})
	}
	for _, stem := range []string{"rfc990", "rfc980"} {
		rows[stem] = rfc.LedgerRow{Status: "Supported", Remaining: "one MUST short"}
		gated = append(gated, rfc.Requirement{RFC: stem, RID: stem + "-1",
			Level: "MUST", Annotation: &rfc.Annotation{Kind: "gap"}})
	}
	for extra := range rfcGapClustersShown + 3 {
		stem := "rfc" + strconv.Itoa(200+extra)
		rows[stem] = rfc.LedgerRow{Status: "Partial"}
		gated = append(gated, rfc.Requirement{RFC: stem, RID: stem + "-1",
			Level: "MUST", Annotation: &rfc.Annotation{Kind: "gap"}})
	}
	_, gaps := rfcBuckets(gated, nil, rows)

	var order []string
	for _, row := range gaps.StatusCounts {
		order = append(order, row.Status)
	}
	want := append(append([]string{}, rfcStatusOrder...), "Zebra")
	if strings.Join(order, "|") != strings.Join(want, "|") {
		t.Errorf("the statuses read %v, want %v", order, want)
	}
	if len(gaps.TopRFCs) != rfcGapClustersShown {
		t.Errorf("the cluster table has %d rows, want the %d it is bounded to",
			len(gaps.TopRFCs), rfcGapClustersShown)
	}
	var disclosed []string
	for _, row := range gaps.SupportedWithRemaining {
		disclosed = append(disclosed, row.RFC)
	}
	if strings.Join(disclosed, "|") != "RFC 298|RFC 980|RFC 990" {
		t.Errorf("the Supported disclosure reads %v, want RFC 298, RFC 980, RFC 990 in that order", disclosed)
	}
}

// VALIDATES: a mechanically re-sealable verdict is counted apart from one that
// needs a human re-read, and a requirement with no verdict counts as missing.
//
// Folding `shifted` into `stale` would report a line shift as a void verdict,
// and folding it into `fresh` would claim a re-read nobody did.
func TestAShiftedVerdictIsCountedApartFromAStaleOne(t *testing.T) {
	gated := []rfc.Requirement{
		{RID: "current"}, {RID: "moved"}, {RID: "changed-unit"},
		{RID: "changed-text"}, {RID: "unaudited"},
	}
	states := map[string]rfc.Freshness{
		"current":      {State: rfc.FreshState},
		"moved":        {State: rfc.ShiftedState},
		"changed-unit": {State: rfc.StaleUnitState},
		"changed-text": {State: rfc.StaleRequirementState},
		// A verdict for a requirement the gate does not gate is not counted.
		"ungated": {State: rfc.FreshState},
	}
	audit := rfcAuditCounts(gated, states, len(gated))

	if audit.Fresh != 1 || audit.Shifted != 1 || audit.Stale != 2 {
		t.Errorf("the audit counts %d fresh, %d shifted and %d stale, want 1, 1 and 2",
			audit.Fresh, audit.Shifted, audit.Stale)
	}
	if audit.Verdicts != 4 || audit.Missing != 1 {
		t.Errorf("the audit counts %d verdicts and %d missing, want 4 and 1",
			audit.Verdicts, audit.Missing)
	}
	if more := rfcAuditCounts(gated, states, 2); more.Missing != 0 {
		t.Errorf("more verdicts than requirements answers %d missing, want 0", more.Missing)
	}
}

// VALIDATES: the gate output the page quotes is the gate's own first line.
//
// The page publishes it as "Current gate output", so a sentence composed here
// would be a quotation of a run that never happened.
func TestTheQuotedGateOutputIsTheGatesOwnLine(t *testing.T) {
	for _, report := range []rfc.CheckReport{
		{Gated: 2966, Enrolled: 171, Tags: 3595},
		{Violations: []string{"one", "two"}},
		{CannotRun: "rfc/enrolled.txt is unreadable"},
	} {
		want, _, _ := strings.Cut(report.Text(), "\n")
		if got := rfcGateSummary(&report); got != want {
			t.Errorf("the page quotes %q, the gate printed %q", got, want)
		}
	}
}

// VALIDATES: a summary stem is printed the way the published page prints it.
func TestASummaryStemIsPrintedAsTheReaderKnowsIt(t *testing.T) {
	for stem, want := range map[string]string{
		"rfc9012":                  "RFC 9012",
		"draft-ietf-bess-mup-safi": "DRAFT-IETF-BESS-MUP-SAFI",
		"sflow-v5":                 "SFLOW-V5",
	} {
		if got := rfcDisplayName(stem); got != want {
			t.Errorf("%s prints as %q, want %q", stem, got, want)
		}
	}
}

// VALIDATES: AC-8 -- the index links every published stem, in the enrolled
// table or in the declined one, so no page of the family is reachable only
// through search.
//
// 39 of the 190 stems carry no row in docs/features/rfc-status.md, which is why
// this index is the family's own rather than the mirror of that page.
func TestTheComplianceIndexLinksEveryPublishedStem(t *testing.T) {
	ledger := twoStemLedger()
	paths := rfcCompliancePathsWith(t, publishedRFCComplianceRef(t), ledger)
	if _, err := renderRFCCompliance(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, rfcComplianceDest)
	mirror := readArtifact(t, paths.Output, rfcComplianceDirectory+"/"+pageMirrorFile)

	for index := range ledger.Stems {
		entry := &ledger.Stems[index]
		href := `href="` + entry.Stem + `/"`
		if !strings.Contains(page, href) {
			t.Errorf("the index does not link %s", entry.Stem)
		}
		if !strings.Contains(mirror, "]("+entry.Stem+"/"+pageMirrorFile+")") {
			t.Errorf("the index mirror does not link %s", entry.Stem)
		}
	}
	if !strings.Contains(page, "<h2>Enrolled RFCs</h2>") ||
		!strings.Contains(page, "<h2>Summaries that are not enrolled</h2>") {
		t.Error("the index carries fewer than the two link tables")
	}
	if !strings.Contains(mirror, "## Enrolled RFCs") ||
		!strings.Contains(mirror, "## Summaries that are not enrolled") {
		t.Error("the index mirror carries fewer than the two link tables")
	}
}

// VALIDATES: AC-8 -- a declined summary states the kind and the reason
// rfc/not-enrolled.txt declares for it, and a summary the file declares nothing
// for says so rather than showing a blank cell.
func TestADeclinedStemStatesItsKindAndReason(t *testing.T) {
	ledger := twoStemLedger()
	paths := rfcCompliancePathsWith(t, publishedRFCComplianceRef(t), ledger)
	if _, err := renderRFCCompliance(paths); err != nil {
		t.Fatal(err)
	}
	text := visibleText(mainContent(t, readArtifact(t, paths.Output, rfcComplianceDest)))
	for _, want := range []string{"backlog", "the extraction is owed"} {
		if !strings.Contains(text, want) {
			t.Errorf("the declined table does not state %q", want)
		}
	}

	// A summary that is not enrolled and declares no disposition is REFUSED by
	// name rather than rendered with a placeholder. The renderer used to print
	// "undeclared"; ParseMeta now refuses a kind with no reason
	// (readEnrolment, internal/le/rfc/meta.go), so the only way that state can
	// reach a page is a truncated or hand-written artifact, and the loader is
	// where it stops (ai/rules/principles.md).
	undeclared := twoStemLedger()
	undeclared.Stems[1].Disposition = nil
	other := rfcCompliancePathsWith(t, publishedRFCComplianceRef(t), undeclared)
	_, err := renderRFCCompliance(other)
	if err == nil {
		t.Fatal("a summary with no declared disposition was published anyway")
	}
	for _, want := range []string{"rfc9997", "is not enrolled and carries no disposition"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal is %q, which does not name %q", err, want)
		}
	}
}

// VALIDATES: AC-8 over this checkout's own corpus -- every summary stem the
// repository carries is linked from the index, including the 39 with no row in
// the public ledger.
func TestTheComplianceIndexLinksEveryStemOfThisCheckout(t *testing.T) {
	root := repositoryRoot(t)
	ledger, err := collectRequirementLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	stems, err := rfcSummaryStems(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Stems) != len(stems) {
		t.Fatalf("the ledger holds %d stems and rfc/short holds %d", len(ledger.Stems), len(stems))
	}
	paths := rfcCompliancePathsWith(t, publishedRFCComplianceRef(t), ledger)
	if _, err := renderRFCCompliance(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, rfcComplianceDest)
	missing := 0
	for _, stem := range stems {
		if !strings.Contains(page, `href="`+stem+`/"`) {
			t.Errorf("the index does not link %s", stem)
			missing++
		}
	}
	t.Logf("the index links %d of %d summary stems", len(stems)-missing, len(stems))
}

// VALIDATES: AC-27 -- the gate's verdict is published as a STATUS carrying its
// tone, not as terminal output to copy.
//
// It was a `<pre class="rfc-command">` until 2026-09-01, which gave a one-line
// verdict terminal styling and the copy button website/assets/js/site.js
// attaches to every `pre`. Both told a reader to paste it into a shell. The
// only thing here a reader runs is the invocation that reproduces it (owner
// review, 2026-09-01).
func TestTheGateVerdictIsAStatusAndNotTerminalOutput(t *testing.T) {
	paths := rfcCompliancePaths(t)
	page, text := rfcCompliancePage(t, paths)
	body := mainContent(t, page)

	if strings.Contains(page, "rfc-command") {
		t.Error("the page still carries the terminal styling the verdict was published in")
	}
	if strings.Contains(body, "<pre") {
		t.Error("the page publishes a pre block, which site.js gives a copy button")
	}
	if !strings.Contains(body, `<div class="rfc-verdict rfc-`) {
		t.Error("the page carries no verdict status block")
	}
	for _, want := range []string{
		"Gate verdict",
		"Every enrolled MUST-level requirement carries both test polarities",
		"Reproduce it with",
		"The gate's own line reads",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the verdict status does not state %q", want)
		}
	}

	// The two link tables this family added to the index scroll inside their own
	// container, as every table on a detail page does. A stem row carries a
	// title and a disposition reason, both of them prose.
	for _, want := range []string{
		`<div class="rfc-table-wrap">` + "\n" + `<table class="rfc-table">` + "\n<thead><tr><th>RFC</th><th>Public status</th>",
		`<div class="rfc-table-wrap">` + "\n" + `<table class="rfc-table">` + "\n<thead><tr><th>RFC</th><th>Disposition</th>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the index carries a link table outside a scrolling container: %q", want)
		}
	}

	red := publishedRFCCompliance(t)
	red.Gate.OK, red.Gate.ErrorCount = false, 158
	redPage, _ := rfcCompliancePage(t, rfcCompliancePathsFrom(t, &red))
	if !strings.Contains(redPage, `<div class="rfc-verdict rfc-`+rfcToneBad+`">`) {
		t.Error("a red gate does not read as red")
	}
	if !strings.Contains(visibleText(mainContent(t, redPage)),
		"158 open gate issues. Check results below names them") {
		t.Error("a red gate does not say how many requirements are open")
	}
}

// VALIDATES: AC-29 -- the bucket table proves its own accounting, and every
// tape segment is legible whatever its width.
//
// The owner read 3,256 beside five buckets and could not tell whether they
// summed to it. They do, exactly, and the page never said so. The method is the
// arithmetic over the live snapshot rather than a pinned number, so a bucket
// that stops partitioning the gated population reddens here.
func TestTheBucketTableAccountsForEveryGatedRequirement(t *testing.T) {
	paths := rfcCompliancePaths(t)
	snapshot := publishedRFCCompliance(t)
	page, text := rfcCompliancePage(t, paths)

	split := rfcBindingOf(&snapshot)
	binding, scope := 0, 0
	for _, bucket := range rfcSatisfaction {
		if bucket.Binds {
			binding += snapshotCount(&snapshot, bucket.Key)
			continue
		}
		scope += snapshotCount(&snapshot, bucket.Key)
	}
	if binding+scope != snapshot.Gate.GatedMust {
		t.Fatalf("the buckets sum to %d and the gate holds %d, so the fixture cannot prove "+
			"the accounting", binding+scope, snapshot.Gate.GatedMust)
	}
	if split.Obligations != binding || split.OutOfScope != scope {
		t.Fatalf("the split answers %d binding and %d out of scope, the buckets carry %d and %d",
			split.Obligations, split.OutOfScope, binding, scope)
	}
	if !strings.Contains(text, rfcBindingLabel+groupThousands(binding)) {
		t.Errorf("the table carries no %q row reading %s", rfcBindingLabel, groupThousands(binding))
	}
	if !strings.Contains(text, rfcGatedLabel+groupThousands(snapshot.Gate.GatedMust)) {
		t.Errorf("the table carries no %q row reading %s", rfcGatedLabel,
			groupThousands(snapshot.Gate.GatedMust))
	}
	if !strings.Contains(text, "every obligation that binds Ze falls in exactly one bucket") {
		t.Error("the page never says the buckets account for the binding population")
	}
	if !strings.Contains(text, "the accounting total: "+groupThousands(binding)+
		" that bind Ze plus "+groupThousands(scope)+" that do not") {
		t.Error("the page never states the gated count as the sum of the two populations")
	}
	mirror := mirrorOf(t, paths)
	for _, want := range []string{
		"| **" + rfcBindingLabel + "** | **" + groupThousands(binding) + "** |",
		"| **" + rfcGatedLabel + "** | **" + groupThousands(snapshot.Gate.GatedMust) + "** |",
	} {
		if !strings.Contains(mirror, want) {
			t.Errorf("the mirror carries no row %q", want)
		}
	}

	// A short bucket used to carry a label wider than its own segment, so the
	// buckets that matter were the unreadable ones. The tape is the proportion
	// now and the key beneath it is the words.
	tape := strings.Index(page, `<div class="rfc-tape"`)
	if tape < 0 {
		t.Fatal("the page carries no tape")
	}
	if body := page[tape : strings.Index(page[tape:], "</div>")+tape]; strings.Contains(body, "<b>") {
		t.Error("the tape still carries text inside its own segments")
	}
	for _, bucket := range rfcSatisfaction {
		// EVERY declared bucket needs a color, not only the ones this fixture
		// populates. missing_unexcused and one_polarity_unexcused had none, so
		// the 148 requirements in the first of them rendered as a blank
		// segment on the live page while the fixture, which carries neither,
		// showed nothing wrong.
		if !strings.Contains(rfcComplianceStyle, ".rfc-tape-"+bucket.Key+" {") {
			t.Errorf(".rfc-tape-%s has no color rule, so that bucket renders blank", bucket.Key)
		}
		if !bucket.Binds || snapshotCount(&snapshot, bucket.Key) == 0 {
			continue
		}
		if !strings.Contains(text, bucket.Label+": "+groupThousands(snapshotCount(&snapshot, bucket.Key))) {
			t.Errorf("the tape key does not name %q with its count", bucket.Label)
		}
	}
	// The bar is the binding population, so the page says in words what it
	// leaves out. A proportion whose population a reader cannot name is the
	// shape this whole pass exists to remove.
	if scope > 0 && !strings.Contains(text, groupThousands(scope)+
		" further gated MUSTs are {not-applicable}") {
		t.Error("the tape never says which obligations it leaves out")
	}
	for _, bucket := range rfcSatisfaction {
		if bucket.Binds {
			continue
		}
		if strings.Contains(page[tape:strings.Index(page[tape:], "</div>")+tape],
			"rfc-tape-"+bucket.Key) {
			t.Errorf("the bar carries %q, which does not bind Ze", bucket.Label)
		}
	}
}

// snapshotCount answers one bucket's count from a stated snapshot.
func snapshotCount(snapshot *rfcCompliance, key string) int {
	for _, bucket := range snapshot.Satisfaction {
		if bucket.Key == key {
			return bucket.Count
		}
	}
	return 0
}

// mirrorOf renders the index and answers its Markdown sibling.
func mirrorOf(t *testing.T, paths Paths) string {
	t.Helper()
	if _, err := renderRFCCompliance(paths); err != nil {
		t.Fatal(err)
	}
	return readArtifact(t, paths.Output, rfcComplianceDirectory+"/"+pageMirrorFile)
}

// VALIDATES: AC-30 -- the Gated MUSTs card says what its number IS.
//
// A reader could not tell whether 3,256 counted what Ze covers or what exists.
// It counts what the GATE HOLDS, and the corpus behind it is larger.
func TestTheGatedMUSTCardNamesThePopulationItIsASubsetOf(t *testing.T) {
	ledger := twoStemLedger()
	totals := rfcTotalsOf(ledger)
	if totals.Summaries != 2 || totals.Requirements != 1 {
		t.Fatalf("the stated ledger totals %d requirements over %d summaries, want 1 over 2",
			totals.Requirements, totals.Summaries)
	}
	cards := rfcComplianceCards(publishedRFCComplianceRef(t), ledger)
	var note string
	for _, card := range cards {
		if card.Label == "Gated MUSTs" {
			note = card.Note
		}
	}
	if !strings.Contains(note, "the gate HOLDS") {
		t.Errorf("the card note %q does not say what the number counts", note)
	}
	var count string
	for _, card := range cards {
		if card.Label == "Gated MUSTs" {
			count = card.Count
		}
	}
	if count != "1 extracted from 2 summaries" {
		t.Errorf("the card counts %q, want the corpus it is a subset of", count)
	}
}

// VALIDATES: AC-34 -- every card declares the rule that chose its tone, the
// page publishes those rules, and a number with no direction takes no color.
func TestEveryCardStatesTheRuleBehindItsColor(t *testing.T) {
	paths := rfcCompliancePaths(t)
	_, text := rfcCompliancePage(t, paths)
	entry := disclosureLedger().Stems[0]

	for name, cards := range map[string][]rfcCard{
		"index":  rfcComplianceCards(publishedRFCComplianceRef(t), twoStemLedger()),
		"detail": rfcDetailCards(&entry),
	} {
		for _, card := range cards {
			if strings.TrimSpace(card.Rule) == "" {
				t.Errorf("the %s %s card carries a color and no rule for it", name, card.Label)
			}
			switch card.Tone {
			case rfcToneOK, rfcToneNeutral, rfcToneWarn, rfcToneBad:
			default:
				t.Errorf("the %s %s card takes tone %q, which is outside the vocabulary",
					name, card.Label, card.Tone)
			}
		}
	}

	if !strings.Contains(text, "How to read the colors") {
		t.Error("the index page publishes no legend for its card colors")
	}
	for _, card := range rfcComplianceCards(publishedRFCComplianceRef(t), twoStemLedger()) {
		if !strings.Contains(text, card.Rule) {
			t.Errorf("the legend does not state the rule for %s", card.Label)
		}
	}

	// A population is a scale, so it takes the neutral tone rather than a color
	// that reads as a verdict.
	for _, card := range rfcComplianceCards(publishedRFCComplianceRef(t), twoStemLedger()) {
		if card.Label == "Gated MUSTs" && card.Tone != rfcToneNeutral {
			t.Errorf("the Gated MUSTs card reads %q, want %q", card.Tone, rfcToneNeutral)
		}
	}
}

// VALIDATES: AC-35 -- a ratio LEADS and a population FOLLOWS, and every ratio
// is taken over the obligations that bind Ze.
//
// The page led with "Gated MUSTs 3,256", a count of obligations JUDGED that
// reads as a count of obligations MET, and 834 of that number never bound Ze at
// all. The owner called the arrangement deceptive on 2026-09-01. This holds the
// order and the denominator, which are the two things that made it so.
func TestARatioLeadsAndAPopulationFollows(t *testing.T) {
	snapshot := publishedRFCCompliance(t)
	cards := rfcComplianceCards(&snapshot, twoStemLedger())
	// SCALE leads and STANDING follows, by owner amendment of 2026-09-01. The
	// population is safe at the head only while it is labeled as scale and the
	// coverage shares sit in the same grid immediately after it, which is the
	// reason the earlier ratio-first order existed and is what this holds.
	for position, label := range []string{"Gated MUSTs", "Out of scope"} {
		if cards[position].Label != label {
			t.Errorf("card %d is %q, want %q: scale leads", position, cards[position].Label, label)
		}
		if cards[position].Tone != rfcToneNeutral {
			t.Errorf("the %s card reads %q, so a scale looks like a result",
				cards[position].Label, cards[position].Tone)
		}
	}
	if !strings.Contains(cards[0].Note, "A population, not a result") {
		t.Errorf("the leading card does not say it is a population: %q", cards[0].Note)
	}
	for position := 2; position < 5; position++ {
		if !strings.HasSuffix(cards[position].Value, "%") {
			t.Errorf("card %d is %q, so a reader meets no coverage share beside the scale",
				position, cards[position].Label)
		}
	}

	// Every ratio's denominator is the binding population, never the gated
	// count. 839 of the fixture's 2,966 gated MUSTs are {not-applicable}.
	split := rfcBindingOf(&snapshot)
	if split.OutOfScope == 0 {
		t.Fatal("the fixture carries no out-of-scope obligation, so this proves nothing")
	}
	if split.Obligations+split.OutOfScope != split.Gated {
		t.Errorf("%d binding plus %d out of scope is not the %d gated",
			split.Obligations, split.OutOfScope, split.Gated)
	}
	for _, card := range cards {
		if !strings.HasSuffix(card.Value, "%") {
			continue
		}
		if strings.Contains(card.Note, "of "+groupThousands(split.Gated)+" ") {
			t.Errorf("the %s ratio is taken over the gated count, which carries %d "+
				"obligations that do not bind Ze", card.Label, split.OutOfScope)
		}
	}

	// The out-of-scope set is reported, and reported as SCOPE.
	var scope rfcCard
	for _, card := range cards {
		if card.Label == "Out of scope" {
			scope = card
		}
	}
	if scope.Value != groupThousands(split.OutOfScope) {
		t.Errorf("the out-of-scope card reads %q, want %s", scope.Value,
			groupThousands(split.OutOfScope))
	}
	if !strings.Contains(scope.Note, "does not bind Ze") ||
		!strings.Contains(scope.Note, "in no share below") {
		t.Errorf("the out-of-scope card does not say what it is: %q", scope.Note)
	}
	if scope.Tone != rfcToneNeutral {
		t.Errorf("the out-of-scope card reads %q, so it looks like a result", scope.Tone)
	}
}

// VALIDATES: AC-39 -- the ratio cards PARTITION the binding population, so a
// reader who adds the shares lands on the whole rather than on 96.7% with
// nowhere to look for the rest.
//
// The owner did that arithmetic on RFC 4271 and it did not close: two of the
// four parts were published and the single-polarity bucket had a row in the
// table and no card. The method is the vocabulary rather than a fixed list, so
// a sixth bucket that lands in no group reddens here.
func TestTheRatioCardsPartitionTheirDenominator(t *testing.T) {
	grouped := map[string]string{}
	for _, group := range rfcStanding {
		for _, key := range group.Keys {
			if held, twice := grouped[key]; twice {
				t.Errorf("%s is in %q and in %q, so its count is published twice",
					key, held, group.Label)
			}
			grouped[key] = group.Label
		}
	}
	for _, bucket := range rfcSatisfaction {
		_, held := grouped[bucket.Key]
		if bucket.Binds && !held {
			t.Errorf("%s binds Ze and is in no ratio card, so the shares do not add up",
				bucket.Key)
		}
		if !bucket.Binds && held {
			t.Errorf("%s does not bind Ze and is inside %q, which inflates that share",
				bucket.Key, grouped[bucket.Key])
		}
	}

	// Over the real snapshot and over one stated stem, the parts sum to the
	// whole and every part names it as its denominator.
	snapshot := publishedRFCCompliance(t)
	split := rfcBindingOf(&snapshot)
	entry := disclosureLedger().Stems[0]
	for name, one := range map[string]struct {
		cards []rfcCard
		whole int
	}{
		"index":  {rfcComplianceCards(&snapshot, twoStemLedger()), split.Obligations},
		"detail": {rfcDetailCards(&entry), entry.Coverage.Binding()},
	} {
		parts := 0
		for _, card := range one.cards {
			if !card.Partition {
				continue
			}
			parts++
			if !strings.HasSuffix(card.Count, "of "+groupThousands(one.whole)+
				" binding obligations") {
				t.Errorf("the %s %s card does not name its denominator: %q", name, card.Label,
					card.Count)
			}
		}
		if parts != len(rfcStanding) {
			t.Errorf("the %s grid marks %d cards as parts, want %d", name, parts,
				len(rfcStanding))
		}
		// The proof ratio is over tagged units, so it must never be counted in.
		for _, card := range one.cards {
			if card.Label == "Proven by a recorded break" && card.Partition {
				t.Errorf("the %s proof ratio is marked as a part of the binding population",
					name)
			}
		}
	}
}

// VALIDATES: AC-40 -- the proof card says what a recorded break IS, and a
// lapsed record is not counted as a proof.
//
// A break is observed ONCE and never re-run. verifyOneDiscrimination re-checks
// that nothing it rested on has moved, so a record whose unit, claim or
// producer changed is a lapsed proof. Counting it would publish a red nobody
// has seen against bytes nobody has checked.
func TestALapsedRecordIsNotCountedAsAProof(t *testing.T) {
	entry := disclosureLedger().Stems[0]
	var proof rfcCard
	for _, card := range rfcDetailCards(&entry) {
		if card.Label == "Proven by a recorded break" {
			proof = card
		}
	}
	for _, want := range []string{
		"observed once under a recorded procedure",
		"still hash to what was recorded",
		"The break is not re-run",
	} {
		if !strings.Contains(proof.Note, want) {
			t.Errorf("the proof card does not say %q: %q", want, proof.Note)
		}
	}
	if !strings.Contains(proof.Count, "tagged units") {
		t.Errorf("the proof card does not name its own denominator: %q", proof.Count)
	}

	lapsed := disclosureLedger().Stems[0]
	lapsed.Coverage.Units, lapsed.Coverage.Records = 4, 4
	lapsed.Coverage.Escapes, lapsed.Coverage.Stale = 0, 4
	if lapsed.Coverage.Proven() != 0 {
		t.Errorf("four lapsed records answer %d proven, want 0", lapsed.Coverage.Proven())
	}
	for _, card := range rfcDetailCards(&lapsed) {
		if card.Label != "Proven by a recorded break" {
			continue
		}
		if card.Value != "0.0%" {
			t.Errorf("four lapsed records read %q, want 0.0%%", card.Value)
		}
		if !strings.Contains(card.Count, "4 lapsed") {
			t.Errorf("the card does not name the lapsed records: %q", card.Count)
		}
	}
}

// VALIDATES: AC-36 -- a test pair is not published as a proof.
//
// The gate itself says so: a tagged unit is proven when a recorded break has
// been observed to redden it, and 3,928 of this checkout's units carry no such
// record. The proof ratio sits beside the test-pair ratio so the two cannot be
// mistaken for each other.
func TestTheProofRatioSitsBesideTheTestPairRatio(t *testing.T) {
	entry := disclosureLedger().Stems[0]
	for name, cards := range map[string][]rfcCard{
		"index":  rfcComplianceCards(publishedRFCComplianceRef(t), twoStemLedger()),
		"detail": rfcDetailCards(&entry),
	} {
		lastPart, proof := -1, -1
		for position, card := range cards {
			if card.Partition {
				lastPart = position
			}
			if card.Label == "Proven by a recorded break" {
				proof = position
			}
		}
		if lastPart < 0 || proof < 0 {
			t.Fatalf("the %s cards carry no partition ratio or no proof ratio", name)
		}
		// Immediately after the shares it corrects, so a reader who has just
		// read "tested both ways" meets it before anything else.
		if proof != lastPart+1 {
			t.Errorf("the %s proof ratio is at %d and the last coverage share at %d, so a "+
				"reader can read one without the other", name, proof, lastPart)
		}
		if !strings.Contains(cards[proof].Note, "A test pair is not a proof until one has") {
			t.Errorf("the %s proof card does not say what it corrects: %q", name,
				cards[proof].Note)
		}
	}

	// An escape claims no break exists, so it is not counted as a proof.
	escaped := disclosureLedger().Stems[0]
	escaped.Coverage.Units, escaped.Coverage.Records, escaped.Coverage.Escapes = 4, 4, 4
	for _, card := range rfcDetailCards(&escaped) {
		if card.Label != "Proven by a recorded break" {
			continue
		}
		if card.Value != "0.0%" {
			t.Errorf("four units carrying nothing but escapes read %q, want 0.0%%", card.Value)
		}
	}
}

// VALIDATES: AC-42 and AC-43 -- the index publishes what the walks DECLINED to
// map, with its own coverage on its face and the presumed-wrong caution beside
// the kind the repository's own rule suspects.
//
// Two mechanisms take an obligation off the gated ledger and the index
// published one of them. `{not-applicable}` annotates a requirement that
// exists; an excluded site never becomes a requirement at all (owner review,
// 2026-09-01). The method is the stated ledger rather than a pinned number, so
// a kind that stops being counted reddens here.
func TestTheExclusionLedgerIsPublishedWithItsCoverage(t *testing.T) {
	ledger := twoStemLedger()
	ledger.Stems[0].Extraction = &rfcLedgerExtraction{
		Path: "rfc/extraction/rfc9998.json", Mapped: 4,
		Exclusions: []rfcLedgerExcludedSite{
			{ID: "2:1", Kind: "binds-another-role", Reason: "a relay is a separate role"},
			{ID: "2:2", Kind: "binds-another-role", Reason: "and so is a proxy"},
			{ID: "3:1", Kind: "feature-out-of-scope", Reason: "widgets over TLS are OPTIONAL"},
		},
	}
	exclusions := rfcExclusionsOf(ledger)
	if exclusions.Sites != 3 || exclusions.Signed != 1 || exclusions.Stems != 2 {
		t.Fatalf("the ledger answers %d sites over %d of %d summaries, want 3 over 1 of 2",
			exclusions.Sites, exclusions.Signed, exclusions.Stems)
	}

	paths := rfcCompliancePathsWith(t, publishedRFCComplianceRef(t), ledger)
	page, text := rfcCompliancePage(t, paths)

	// Every kind the vocabulary declares has a row, and its meaning is the
	// package's own sentence rather than one this file wrote.
	for _, kind := range rfc.ExclusionKinds() {
		meaning, held := rfc.ExclusionKindMeaning(kind)
		if !held {
			t.Fatalf("%s is in the vocabulary and has no meaning", kind)
		}
		if !strings.Contains(text, kind) {
			t.Errorf("the exclusion table carries no row for %q", kind)
		}
		if !strings.Contains(text, meaning) {
			t.Errorf("the exclusion table does not state what %q means", kind)
		}
	}
	// Kind, what it MEANS as a group, the count, the summaries, and the
	// sentence: the group cell sits between the kind and its count so a reader
	// cannot take a debt for scope.
	if !strings.Contains(text,
		"binds-another-role never bound Ze 2 1 the obligation is addressed to a role") {
		t.Error("the exclusion table does not carry the per-kind count beside its meaning")
	}

	// The coverage is on the section's own face: a count over a minority of the
	// corpus read as a count over the corpus is the flattery this page exists
	// to prevent.
	if !strings.Contains(text, "1 of 2 summaries carry an extraction sign-off") {
		t.Error("the section does not state how much of the corpus it covers")
	}
	if !strings.Contains(text, "is not the whole picture") {
		t.Error("the section does not say the count is partial")
	}

	// The largest kind is the one the repository's own rule presumes wrong.
	if !rfc.ExclusionPresumedWrong("binds-another-role") {
		t.Fatal("the package no longer presumes binds-another-role wrong")
	}
	if !strings.Contains(text, "PRESUMED WRONG until it is justified") {
		t.Error("the section publishes the suspect kind without the rule's own caution")
	}
	// Each one is reachable where its justification is written.
	if !strings.Contains(page, `<a href="rfc9998/"><code>RFC 9998</code></a>`) {
		t.Error("the summaries that used a kind are not linked to their own pages")
	}
}

// VALIDATES: AC-47 -- the exclusion kinds partition into what never bound Ze
// and what Ze OWES, and the two are never summed.
//
// `relocated-to-spec` says the obligation is real and unbuilt and a named spec
// owns it. Publishing it under one heading with the five that say the
// obligation never bound Ze files a debt as scope (owner review, 2026-09-01).
// The split comes from the vocabulary, so a seventh kind lands in one group or
// reddens here rather than defaulting to scope.
func TestTheExclusionGroupsPartitionTheVocabulary(t *testing.T) {
	groups := map[string]bool{rfc.ExclusionScope: true, rfc.ExclusionDebt: true}
	for _, kind := range rfc.ExclusionKinds() {
		group, held := rfc.ExclusionKindGroup(kind)
		if !held || group == "" {
			t.Errorf("%s declares no group, so it would read as scope by omission", kind)
			continue
		}
		if !groups[group] {
			t.Errorf("%s declares group %q, which is outside the vocabulary", kind, group)
		}
	}
	if group, _ := rfc.ExclusionKindGroup("relocated-to-spec"); group != rfc.ExclusionDebt {
		t.Errorf("relocated-to-spec reads as %q, so an obligation Ze owes is filed as scope",
			group)
	}
}

// VALIDATES: AC-48 -- a relocated obligation is published as the debt it is,
// with its reserved id, its quoted sentence and the spec that owns it, apart
// from the counts that say an obligation never bound Ze.
func TestARelocatedObligationIsPublishedAsDebt(t *testing.T) {
	ledger := twoStemLedger()
	ledger.Stems[0].Extraction = &rfcLedgerExtraction{
		Path: "rfc/extraction/rfc9998.json", Mapped: 4, Excluded: 3, Relocated: 1,
		Exclusions: []rfcLedgerExcludedSite{
			{ID: "2:1", Kind: "binds-another-role", Reason: "a relay is a separate role"},
			{ID: "3:1", Kind: "feature-out-of-scope", Reason: "widgets over TLS are OPTIONAL"},
			{ID: "5:2", Kind: "relocated-to-spec", Reason: "the spec owns it",
				Quote:       "All widget implementations MUST support Types 1-4.",
				RelocatedTo: "plan/spec-widget-types.md", ReservedID: "RFC9998-5-2"},
		},
	}
	exclusions := rfcExclusionsOf(ledger)
	if exclusions.Sites != 3 || exclusions.Debt != 1 {
		t.Fatalf("the ledger answers %d sites of which %d debt, want 3 and 1",
			exclusions.Sites, exclusions.Debt)
	}
	if len(exclusions.Relocated) != 1 || exclusions.Relocated[0].ID != "RFC9998-5-2" {
		t.Fatalf("the relocated obligations are %v", exclusions.Relocated)
	}

	paths := rfcCompliancePathsWith(t, publishedRFCComplianceRef(t), ledger)
	page, text := rfcCompliancePage(t, paths)

	if !strings.Contains(text, rfcDebtHeading) {
		t.Errorf("the section publishes no %q heading", rfcDebtHeading)
	}
	if !strings.Contains(text, "These 1 sentence are NOT scope") &&
		!strings.Contains(text, "NOT scope") {
		t.Error("the relocated table does not say it is not scope")
	}
	for _, cell := range []string{
		"RFC9998-5-2",
		"All widget implementations MUST support Types 1-4.",
		"plan/spec-widget-types.md",
	} {
		if !strings.Contains(text, cell) {
			t.Errorf("the relocated table does not carry %q", cell)
		}
	}
	if !strings.Contains(page, `<a href="rfc9998/"><code>RFC 9998</code></a>`) {
		t.Error("the relocated obligation does not link to its own RFC's page")
	}

	// The two counts are stated apart. Summing them would publish a debt as
	// scope, which is the whole point of the split.
	if !strings.Contains(text, "2 say the obligation never bound Ze and 1 say Ze owes it") {
		t.Error("the total row sums the debt into the declined count")
	}
	if !strings.Contains(text, "1 of those declines are not scope at all") {
		t.Error("the section's own lead does not name the debt apart")
	}
}

// publishedRFCComplianceRef answers the published snapshot as a pointer, which
// is how every renderer takes it: the struct passed the linter's size floor
// when the gate's findings joined it.
func publishedRFCComplianceRef(t *testing.T) *rfcCompliance {
	t.Helper()
	snapshot := publishedRFCCompliance(t)
	return &snapshot
}

// VALIDATES: AC-68 -- every un-enrolled kind the index shows carries the
// sentence that says what it means, read from the package that holds the
// closed set rather than listed here.
//
// A sixth kind, `out-of-scope`, landed on 2026-09-01 and a page that spelled
// its kinds would have printed a bare word for it. The method is the vocabulary
// itself against the rendered index and the rendered stem page.
func TestEveryDispositionKindOnTheIndexSaysWhatItMeans(t *testing.T) {
	ledger := publishedLedgerOfThisCheckout(t)
	rows := rfcIndexRows(ledger, false)
	if len(rows) == 0 {
		t.Fatal("no summary is declined, so this proves nothing")
	}
	page := rfcDeclinedIndexHTML(ledger)
	mirror := rfcIndexMirror(ledger)
	seen := map[string]bool{}
	for _, entry := range rows {
		kind := entry.Disposition.Kind
		if seen[kind] {
			continue
		}
		seen[kind] = true
		meaning, held := rfc.DispositionKindMeaning(kind)
		if !held {
			t.Errorf("%s declares %q, which the vocabulary does not know",
				entry.Stem, kind)
			continue
		}
		if !strings.Contains(page, html.EscapeString(meaning)) {
			t.Errorf("the index shows %q and does not say what it means", kind)
		}
		if !strings.Contains(mirror, meaning) {
			t.Errorf("the mirror shows %q and does not say what it means", kind)
		}
		if !strings.Contains(rfcEnrolmentText(entry), meaning) {
			t.Errorf("the %s page states %q without its meaning", entry.Stem, kind)
		}
	}
	t.Logf("%d disposition kinds are in use, each with its meaning published", len(seen))
}

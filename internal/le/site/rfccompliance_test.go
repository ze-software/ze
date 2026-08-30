// Design: website/AI.md -- the RFC compliance report is internal/le/rfc's own answer
package site

import (
	"encoding/json"
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
	return rfcCompliancePathsFrom(t, publishedRFCCompliance(t))
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

// rfcCompliancePathsFrom lays out a checkout whose gate answer is stated.
func rfcCompliancePathsFrom(t *testing.T, snapshot rfcCompliance) Paths {
	t.Helper()
	root := repositoryRoot(t)
	source := t.TempDir()
	copyFixture(t, filepath.Join(root, "website", "data", "page-links.json"),
		filepath.Join(source, "data", "page-links.json"))

	previous := liveRFCCompliance
	t.Cleanup(func() { liveRFCCompliance = previous })
	liveRFCCompliance = func(string) (rfcCompliance, error) { return snapshot, nil }
	return Paths{Repository: root, Source: source, Output: t.TempDir()}
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
	if len(routes) != 1 || routes[0] != rfcComplianceRoute {
		t.Fatalf("the producer claimed %v, want [%s]", routes, rfcComplianceRoute)
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
	published := visibleText(readFixture(t, "published-rfc-compliance-body.html"))

	for _, chrome := range []string{
		"<title>RFC Compliance Gate Report - Ze</title>",
		`<link rel="canonical" href="https://ze-software.net/quality/rfc-compliance/" />`,
		`<link rel="stylesheet" href="../../assets/site.css" />`,
		`<main id="top" class="has-page-sidebar" tabindex="-1">`,
		`<section aria-labelledby="rfc-compliance-title" class="md-content reveal cat-observe">`,
		`<h1 id="rfc-compliance-title">RFC Compliance Gate Report</h1>`,
		`<aside class="page-sidebar" aria-label="Related page links">`,
		"<footer>",
	} {
		if !strings.Contains(page, chrome) {
			t.Errorf("the RFC compliance page is missing %q", chrome)
		}
	}

	for _, section := range []struct{ name, from, mine, theirs string }{
		{"requirement buckets", "Requirement buckets", "Gap disclosure", "Gap disclosure"},
		{"gap disclosure", "Gap disclosure", "Top gap clusters", "Top gap clusters"},
		{"top gap clusters", "Top gap clusters", "Gate inputs", "AI guard and gate inputs"},
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

	if !strings.Contains(page, `aria-labelledby="rfc-compliance-title"`) {
		t.Error("the section carries no aria-labelledby")
	}
	if !strings.Contains(page, `id="rfc-compliance-title"`) {
		t.Error(`aria-labelledby names rfc-compliance-title, which the page does not carry`)
	}
	if !strings.Contains(page, `<div class="rfc-tape" role="img" aria-label="RFC requirement satisfaction split">`) {
		t.Error("the satisfaction tape carries no label, so a screen reader meets an unnamed image")
	}
}

// VALIDATES: the four headline cards whose inputs did not change carry the
// numbers the published cards carry.
func TestTheHeadlineCardsCarryThePublishedNumbers(t *testing.T) {
	paths := rfcCompliancePaths(t)
	_, text := rfcCompliancePage(t, paths)

	for _, card := range []string{
		"Gate verdictOK0 open gate issues",
		"Gated MUSTs2,966171 enrolled RFCs, 3,595 resolved test tags",
		"Declared gaps518Across 80 RFCs, all forced into the public ledger",
		"Semantic verdicts520 shifted, 0 stale, 2,914 missing and therefore not claimed",
	} {
		if !strings.Contains(text, card) {
			t.Errorf("the card grid does not read %q", card)
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
func TestTheGateInputRowsReadAsThePublishedRows(t *testing.T) {
	paths := rfcCompliancePaths(t)
	_, text := rfcCompliancePage(t, paths)

	for _, row := range []string{
		"Requirement sourcerfc/short/*.md2,966 gated MUST-level requirements",
		"Enrollmentrfc/enrolled.txt171 enrolled RFCs",
		"Test tagsinternal/, pkg/, test/3,595 resolved tags",
		"Public ledgerdocs/features/rfc-status.md80 RFCs with gaps, 4 Supported with Remaining",
		"Semantic auditsrfc/audit/*.json52 fresh, 0 shifted, 0 stale, 2,914 missing",
	} {
		if !strings.Contains(text, row) {
			t.Errorf("the gate-input table does not read %q", row)
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
	snapshot.Gate.Violations = []string{"rfc9999: a requirement lost a polarity", "rfc9998: no public row"}
	red := rfcCompliancePathsFrom(t, snapshot)
	_, redText := rfcCompliancePage(t, red)
	for _, issue := range snapshot.Gate.Violations {
		if !strings.Contains(redText, issue) {
			t.Errorf("a red gate does not publish %q", issue)
		}
	}
	if !strings.Contains(redText, "Gate verdictRED2 open gate issues") {
		t.Error("a red gate does not read as red on its card")
	}
}

// VALIDATES: a long list of open issues is bounded rather than published whole.
//
// A gate that goes red on a bad merge can answer thousands of diagnostics, and
// a page that inlined all of them would be megabytes of one build's noise.
func TestALongIssueListIsBoundedAndSaysHowManyItLeftOut(t *testing.T) {
	snapshot := publishedRFCCompliance(t)
	snapshot.Gate.OK = false
	for issue := range rfcIssuesShown + 5 {
		snapshot.Gate.Violations = append(snapshot.Gate.Violations, "issue "+string(rune('a'+issue)))
	}
	snapshot.Gate.ErrorCount = len(snapshot.Gate.Violations)

	_, text := rfcCompliancePage(t, rfcCompliancePathsFrom(t, snapshot))
	if strings.Count(text, "issue ") > rfcIssuesShown {
		t.Errorf("the page publishes %d issues, want at most %d",
			strings.Count(text, "issue "), rfcIssuesShown)
	}
	if !strings.Contains(text, "and 5 more") {
		t.Error("the page does not say how many issues it left out")
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
		"| Positive and negative tests | 1,239 | 41.8% | `positive tag + negative tag` |",
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
	if len(routes) != 1 {
		t.Fatalf("the producer claims %d routes, want 1", len(routes))
	}
	if !published[routes[0]] {
		t.Errorf("the producer claims %s, which the published site does not carry", routes[0])
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
		if got := rfcGateSummary(report); got != want {
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

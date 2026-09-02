// Design: website/AI.md -- one page for each RFC summary this repository carries
package site

import (
	"encoding/json"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// disclosureLedger is one summary carrying every bad state the owner ruling of
// 2026-09-01 names, so a page that softened any of them goes red.
//
// The five states, one requirement each: a gated MUST with no test, a `weak`
// audit verdict, a verdict whose freshness is `stale-unit`, a tagged unit with
// no discrimination record, and a `no-break` record. A sixth requirement
// carries a declared gap, and a seventh a `shifted` verdict, because both are
// named in the ruling's table beside the five.
func disclosureLedger() rfcLedger {
	return rfcLedger{Stems: []rfcLedgerStem{{
		Stem: "rfc9999", Display: "RFC 9999", Title: "The Widget Protocol",
		Enrolled: true, EnrolmentReason: "every MUST carries both polarities",
		PublicStatus: "Partial", PublicCoverage: "widgets only",
		PublicRemaining: "RFC9999-6-1 unmet",
		SummaryPath:     "rfc/short/rfc9999.md", ShardPath: "rfc/requirements/rfc9999.md",
		SourcePath: "rfc/full/rfc9999.txt",
		Coverage: rfcLedgerCoverage{Requirements: 7, Gated: 7, Both: 3, One: 1,
			Annotated: 1, Missing: 2, NightlyOnly: 1, Gaps: 1, GatedGaps: 1,
			NotApplicable: 0, SinglePolarity: 0, Tags: 7, Units: 7, Audited: 3,
			Records: 3, Escapes: 1},
		Requirements: []rfcLedgerRequirement{
			{
				RID: "RFC9999-1-1", Level: "MUST", Section: "1", Gated: true,
				Text:     "A widget MUST carry a length.",
				Positive: "--", Negative: "--", Note: "",
			},
			{
				RID: "RFC9999-2-1", Level: "MUST", Section: "2", Gated: true,
				Text:     "A widget MUST be rejected when its length is zero.",
				Positive: "`internal/a_test.go` `TestWidget` (unit/verify)",
				Negative: "`internal/a_test.go` `TestNoWidget` (unit/verify)",
				Note:     "**audit: weak**",
				Covers: []rfcLedgerCover{
					{Polarity: "positive", Unit: "internal/a_test.go::TestWidget",
						Carrier: "unit/verify", Tags: 1,
						Proof: &rfcLedgerProof{Route: "revert", State: "verified",
							Producer: "internal/a.go::Encode", Verified: true, Proves: true}},
					{Polarity: "negative", Unit: "internal/a_test.go::TestNoWidget",
						Carrier: "unit/verify", Tags: 1},
				},
				Audit: &rfcLedgerVerdict{Verdict: "weak",
					Meaning:   "the tests pass over code that does not enforce the requirement",
					Note:      "the assertion checks that the encoder ran, never what it wrote",
					Freshness: "fresh"},
			},
			{
				RID: "RFC9999-3-1", Level: "MUST", Section: "3", Gated: true,
				Text:     "A widget MUST be logged.",
				Positive: "`internal/b_test.go` `TestLogged` (unit/verify)",
				Negative: "--", Note: "",
				Covers: []rfcLedgerCover{
					{Polarity: "positive", Unit: "internal/b_test.go::TestLogged",
						Carrier: "unit/verify", Tags: 1,
						Proof: &rfcLedgerProof{Route: "no-break", State: "verified",
							Reason: "foreign-producer", Verified: true, Proves: false}},
				},
				Audit: &rfcLedgerVerdict{Verdict: "enforced",
					Meaning:   "the tests do what the requirement demands",
					Freshness: "stale-unit",
					Moved:     []string{"internal/b_test.go::TestLogged"}},
			},
			{
				RID: "RFC9999-4-1", Level: "MUST", Section: "4", Gated: true,
				Text:     "A widget MUST NOT be resent.",
				Positive: "`internal/c_test.go` `TestResend` (interop/nightly)",
				Negative: "`internal/c_test.go` `TestNoResend` (interop/nightly)",
				Note:     "**nightly-only**", NightlyOnly: true,
				Covers: []rfcLedgerCover{
					{Polarity: "positive", Unit: "internal/c_test.go::TestResend",
						Carrier: "interop/nightly", Tags: 1},
					{Polarity: "negative", Unit: "internal/c_test.go::TestNoResend",
						Carrier: "interop/nightly", Tags: 1,
						Proof: &rfcLedgerProof{Route: "revert", State: "unit-changed",
							Detail:   "the tagged unit's behavior changed since the red was observed",
							Producer: "internal/c.go::Send", Verified: false, Proves: true}},
				},
				Audit: &rfcLedgerVerdict{Verdict: "enforced",
					Meaning:   "the tests do what the requirement demands",
					Freshness: "shifted", Moved: []string{"internal/c_test.go::TestResend"}},
			},
			{
				RID: "RFC9999-5-1", Level: "MUST", Section: "5", Gated: true,
				Text:     "A widget MUST be counted | tallied <once> & only once.",
				Positive: "--", Negative: "--",
				Note: "{gap} the counter is not implemented | not scheduled",
				Annotation: &rfcLedgerAnnotation{Kind: "gap",
					Reason: "the counter is not implemented | not scheduled"},
			},
			{
				RID: "RFC9999-6-1", Level: "MUST", Section: "6", Gated: true,
				Text:     "A widget MUST be acknowledged.",
				Positive: "`internal/d_test.go` `TestAck` (unit/verify)",
				Negative: "`internal/d_test.go` `TestNoAck` (unit/verify)",
				Covers: []rfcLedgerCover{
					{Polarity: "positive", Unit: "internal/d_test.go::TestAck",
						Carrier: "unit/verify", Tags: 1},
					{Polarity: "negative", Unit: "internal/d_test.go::TestNoAck",
						Carrier: "unit/verify", Tags: 1},
				},
			},
			{
				RID: "RFC9999-7-1", Level: "MUST", Section: "7", Gated: true,
				Text: "A widget MUST be retired.", Positive: "--", Negative: "--",
				Note: "{superseded: restated RFC9998-2-1} the successor states it",
				Superseded: &rfcLedgerSuccessor{Disposition: "restated",
					Target: "RFC9998-2-1", Reason: "the successor states it"},
			},
		},
		Successor: "rfc9998",
		Extraction: &rfcLedgerExtraction{
			Path: "rfc/extraction/rfc9999.json", Reviewer: "a reader",
			SignedOff: "2026-09-01", Register: "protocol",
			SourcePath: "rfc/full/rfc9999.txt", SourceSHA: "0123456789abcdef",
			Mapped: 7, Excluded: 2, Relocated: 1, Unclassified: 0,
			Sections: []rfcLedgerSection{
				{ID: "1", Sites: 3, Disposition: "walked"},
				{ID: "9", Sites: 0, Disposition: "skipped", SkipKind: "references",
					Reason: "the reference list states no obligation"},
			},
			Exclusions: []rfcLedgerExcludedSite{
				{ID: "S3.1", Quote: "An implementation MUST support widgets over TLS.",
					Kind:   "feature-out-of-scope",
					Reason: "widgets over TLS are OPTIONAL and Ze does not offer them"},
				{ID: "S4.2", Quote: "A relay MUST forward the widget.",
					Kind: "relocated-to-spec", Reason: "a relay is a separate role",
					RelocatedTo: "plan/spec-widget-relay.md", ReservedID: "RFC9999-4-2"},
			},
		},
	}}}
}

// renderRFCDetail renders one stem's page and mirror out of a stated ledger.
func renderRFCDetail(t *testing.T, ledger rfcLedger, stem string) (page, mirror string) {
	t.Helper()
	paths := rfcCompliancePathsWith(t, publishedRFCComplianceRef(t), ledger)
	if _, err := renderRFCCompliance(paths); err != nil {
		t.Fatal(err)
	}
	directory := rfcComplianceDirectory + "/" + stem + "/"
	return readArtifact(t, paths.Output, directory+pageIndexFile),
		readArtifact(t, paths.Output, directory+pageMirrorFile)
}

// disclosurePage renders the disclosure fixture's page, its visible text and
// its mirror.
func disclosurePage(t *testing.T) (page, text, mirror string) {
	t.Helper()
	page, mirror = renderRFCDetail(t, disclosureLedger(), "rfc9999")
	return page, visibleText(mainContent(t, page)), mirror
}

// VALIDATES: AC-1 -- the producer answers one route per summary stem plus its
// own index, and answers each of them exactly once.
func TestTheRFCLedgerClaimsEachPublishedRouteOnce(t *testing.T) {
	ledger := twoStemLedger()
	paths := rfcCompliancePathsWith(t, publishedRFCComplianceRef(t), ledger)
	routes, err := renderRFCCompliance(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != len(ledger.Stems)+1 {
		t.Fatalf("the producer claims %d routes over %d stems, want one each plus the index",
			len(routes), len(ledger.Stems))
	}
	seen := map[string]int{}
	for _, route := range routes {
		seen[route]++
	}
	for route, count := range seen {
		if count != 1 {
			t.Errorf("the producer claims %s %d times; Coverage refuses a doubled route", route, count)
		}
	}
	for _, entry := range ledger.Stems {
		route := rfcComplianceRoute + entry.Stem + "/"
		if seen[route] != 1 {
			t.Errorf("the producer claims no route for %s, want %s", entry.Stem, route)
		}
	}
	if seen[rfcComplianceRoute] != 1 {
		t.Errorf("the producer does not claim its own index %s", rfcComplianceRoute)
	}
}

// VALIDATES: AC-13 and A-4 -- every route the producer answers is a page it
// actually published, so the coverage arithmetic finds neither an unclaimed nor
// a doubled route arising from this family.
func TestTheRFCLedgerClaimsOnlyPublishedRoutes(t *testing.T) {
	paths := rfcCompliancePathsWith(t, publishedRFCComplianceRef(t), twoStemLedger())
	routes, err := renderRFCCompliance(paths)
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range routes {
		directory := strings.Trim(route, "/")
		for _, name := range []string{pageIndexFile, pageMirrorFile} {
			path := filepath.Join(paths.Output, filepath.FromSlash(directory), name)
			if info, err := os.Stat(path); err != nil || info.Size() == 0 {
				t.Errorf("the producer claims %s and published no %s for it: %v", route, name, err)
			}
		}
	}
	published, err := pageRegistry(paths.Output)
	if err != nil {
		t.Fatal(err)
	}
	claimed := map[string]bool{}
	for _, route := range routes {
		claimed[route] = true
	}
	for _, page := range published {
		if !claimed[page.Route] {
			t.Errorf("the artifact publishes %s and no producer claimed it", page.Route)
		}
	}
}

// VALIDATES: A-6 -- every published directory is the summary stem itself, so no
// slug function stands between a stem and its page and two stems cannot collide
// on one directory.
func TestASlugIsTheStemItself(t *testing.T) {
	stems, err := rfcSummaryStems(repositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(stems) == 0 {
		t.Fatal("this checkout carries no summary, so this proves nothing")
	}
	safe := regexp.MustCompile(`\A[a-z0-9][a-z0-9.-]*\z`)
	for _, stem := range stems {
		if !safe.MatchString(stem) {
			t.Errorf("the stem %q is not a plain path segment, so it must not be joined into a path", stem)
		}
		route := rfcComplianceRoute + stem + "/"
		if !strings.HasSuffix(route, "/"+stem+"/") {
			t.Errorf("the route for %s is %s, which does not end in the stem", stem, route)
		}
	}
}

// rfcSummaryStems answers the summary stems of one checkout, read off the
// directory rather than through the ledger, so this test does not depend on the
// producer it is about.
func rfcSummaryStems(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "rfc", "short"))
	if err != nil {
		return nil, err
	}
	stems := make([]string, 0, len(entries))
	for _, entry := range entries {
		if stem, isMarkdown := strings.CutSuffix(entry.Name(), ".md"); isMarkdown {
			stems = append(stems, stem)
		}
	}
	return stems, nil
}

// VALIDATES: AC-11 -- a ledger no page could be built from is refused by name.
//
// A skipped entry publishes a family with a silent hole in it, and an empty
// ledger publishes an index that links nothing while every check the artifact
// carries passes (ai/rules/principles.md).
func TestAnUnusableRequirementLedgerIsRefusedByName(t *testing.T) {
	for _, one := range []struct {
		name   string
		ledger rfcLedger
		want   string
	}{
		{"a ledger naming no stem", rfcLedger{}, "names no RFC summary"},
		{"an entry with an empty stem",
			rfcLedger{Stems: []rfcLedgerStem{{Stem: ""}}}, "carries an entry with no stem"},
	} {
		paths := rfcCompliancePathsWith(t, publishedRFCComplianceRef(t), one.ledger)
		_, err := renderRFCCompliance(paths)
		if err == nil {
			t.Errorf("%s was accepted", one.name)
			continue
		}
		if !strings.Contains(err.Error(), one.want) {
			t.Errorf("%s answered %q, want a refusal saying %q", one.name, err, one.want)
		}
		if !strings.Contains(err.Error(), rfcLedgerFile) {
			t.Errorf("%s answered %q, which does not name the file", one.name, err)
		}
	}
}

// VALIDATES: AC-11 -- a build derives the ledger once and publishes it as a
// named artifact, before any producer reads it.
func TestABuildPublishesTheRequirementLedger(t *testing.T) {
	previous := liveRequirementLedger
	t.Cleanup(func() { liveRequirementLedger = previous })
	liveRequirementLedger = func(string) (rfcLedger, error) { return twoStemLedger(), nil }

	output := t.TempDir()
	if err := publishRFCLedger(Paths{Repository: repositoryRoot(t), Output: output}); err != nil {
		t.Fatal(err)
	}
	var round rfcLedger
	if err := json.Unmarshal([]byte(readArtifact(t, output, rfcLedgerFile)), &round); err != nil {
		t.Fatalf("the published ledger is not JSON: %v", err)
	}
	if len(round.Stems) != 2 || round.Stems[0].Stem != "rfc9998" {
		t.Errorf("the published ledger holds %d stem(s): %+v", len(round.Stems), round.Stems)
	}
	named := false
	for _, name := range namedArtifacts {
		if name == rfcLedgerFile {
			named = true
		}
	}
	if !named {
		t.Errorf("%s is not in namedArtifacts, so a build that stopped writing it would pass every check",
			rfcLedgerFile)
	}
	if missing := checkNamedArtifacts(output); len(missing) == 0 {
		t.Error("the named-artifact check found nothing missing in a tree holding only the ledger")
	}
}

// VALIDATES: AC-2, A-2 and R-6 -- the ledger's requirement cells ARE the cells
// rfc/requirements/<stem>.md carries, over this checkout's own corpus.
//
// The site and the repository must not be able to state different things about
// one requirement. rfc.RequirementRows is the ONE producer of those cells since
// 2026-09-01, and this holds the snapshot against the shard that same reading
// renders, cell by cell, so a mis-mapped column or a dropped escape is a red
// test rather than a silent divergence on a published page.
//
// The comparison is against the GENERATED shard rather than the file on disk.
// Whether the checked-in file is current is `./le rfc check`'s question, and
// several sessions share this checkout: a shard somebody has not regenerated
// yet would redden this test over an edit that is not this page's.
func TestARequirementRowMatchesItsGeneratedShard(t *testing.T) {
	root := repositoryRoot(t)
	ledger, err := collectRequirementLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	collected, err := rfc.Collect(root)
	if err != nil {
		t.Fatal(err)
	}
	input, err := rfc.NewRenderInput(root, collected, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	shards := rfc.RenderShards(input)

	checked := 0
	for index := range ledger.Stems {
		entry := &ledger.Stems[index]
		if entry.ShardPath == "" {
			continue
		}
		rows := shardRowsByRID(shards[entry.Stem])
		if len(rows) == 0 {
			t.Errorf("%s declares requirements and renders no shard row", entry.Stem)
			continue
		}
		for position := range entry.Requirements {
			requirement := &entry.Requirements[position]
			want, held := rows[requirement.RID]
			if !held {
				t.Errorf("%s carries %s and the shard does not", entry.Stem, requirement.RID)
				continue
			}
			got := []string{requirement.RID, requirement.Level, requirement.Section,
				requirement.Positive, requirement.Negative, rfc.TableCell(requirement.Note)}
			for cell := range got {
				if got[cell] != want[cell] {
					t.Errorf("%s cell %d reads %q, the shard reads %q",
						requirement.RID, cell, got[cell], want[cell])
				}
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no requirement was compared against a shard, so this proves nothing")
	}
	t.Logf("compared %d requirement rows against their generated shards", checked)
}

// shardRowsByRID reads one generated shard back into its six cells per
// requirement id, with the backticks around the id removed.
func shardRowsByRID(shard string) map[string][6]string {
	out := map[string][6]string{}
	for line := range strings.SplitSeq(shard, "\n") {
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		body := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(line), "|"), "|")
		cells := strings.Split(body, " | ")
		if len(cells) != 6 {
			continue
		}
		cells[0] = strings.Trim(strings.TrimSpace(cells[0]), "`")
		var row [6]string
		for index := range row {
			row[index] = strings.TrimSpace(cells[index])
		}
		out[row[0]] = row
	}
	return out
}

// VALIDATES: AC-3 -- the at-a-glance facts state the public status, the
// enrolment and its reason, the counts, and the three repository paths.
func TestAStemPageStatesItsEnrolmentAndItsPublicStatus(t *testing.T) {
	_, text, mirror := disclosurePage(t)
	for _, fact := range []string{
		"Partial",
		"Enrolled: every MUST carries both polarities",
		"rfc/short/rfc9999.md",
		"rfc/requirements/rfc9999.md",
		"rfc/full/rfc9999.txt",
	} {
		if !strings.Contains(text, fact) {
			t.Errorf("the page does not state %q", fact)
		}
		if !strings.Contains(mirror, fact) {
			t.Errorf("the mirror does not state %q", fact)
		}
	}

	declined := twoStemLedger()
	_, declinedMirror := renderRFCDetail(t, declined, "rfc9997")
	for _, fact := range []string{
		rfcNoPublicRow,
		// The kind is stated WITH its meaning, because `backlog` is this
		// project's word and a reader outside it cannot act on it alone.
		"Not enrolled (backlog, " + rfcDispositionMeaning("backlog") +
			"): the extraction is owed",
		"no requirement declared, so no shard is generated",
		"this checkout does not carry the RFC's own text",
	} {
		if !strings.Contains(declinedMirror, fact) {
			t.Errorf("the declined stem's mirror does not state %q", fact)
		}
	}
}

// VALIDATES: AC-15 -- the heading carries the RFC's title from the summary's
// Meta row, and a summary declaring none shows the display name alone.
func TestAStemPageCarriesTheTitleFromTheSummaryMetaRow(t *testing.T) {
	page, _, mirror := disclosurePage(t)
	if !strings.Contains(page, "RFC 9999 - The Widget Protocol") {
		t.Error("the page heading does not carry the title from the Meta row")
	}
	if !strings.HasPrefix(mirror, "# RFC 9999 - The Widget Protocol\n") {
		t.Errorf("the mirror opens %q", strings.SplitN(mirror, "\n", 2)[0])
	}

	untitled := disclosureLedger()
	untitled.Stems[0].Title = ""
	_, plain := renderRFCDetail(t, untitled, "rfc9999")
	if !strings.HasPrefix(plain, "# RFC 9999\n") {
		t.Errorf("a summary with no title row opens %q, want the display name alone",
			strings.SplitN(plain, "\n", 2)[0])
	}
}

// VALIDATES: AC-4 -- a declared gap shows its kind and its whole reason, beside
// the public ledger's own Remaining cell. A gap is an ISSUE and is never
// rendered as coverage (ai/rules/rfc-compliance.md).
func TestAGapIsShownWithItsReasonAndTheLedgerRemainder(t *testing.T) {
	_, text, mirror := disclosurePage(t)
	// The ledger's Remaining cell is stated under What the public ledger says,
	// beside the Coverage cell it belongs with, and the requirement-by-
	// requirement answer is under Gaps and untested MUSTs (owner review,
	// 2026-09-01). Both are on the page and each is stated once.
	for _, want := range []string{
		"RFC9999-5-1",
		"the counter is not implemented | not scheduled",
		"What the ledger says remains: RFC9999-6-1 unmet",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the page does not state %q", want)
		}
	}
	// The mirror escapes a pipe inside a table cell and writes the fold's label
	// in bold, so it states the same two facts in Markdown's own spelling.
	for _, want := range []string{
		"RFC9999-5-1",
		rfc.TableCell("the counter is not implemented | not scheduled"),
		// The remainder names a requirement this summary declares, so it is
		// linked to its own row rather than left as text.
		"**What the ledger says remains:**\n\n[`RFC9999-6-1`](#rfc9999-6-1) unmet",
	} {
		if !strings.Contains(mirror, want) {
			t.Errorf("the mirror does not state %q", want)
		}
	}

	noRow := disclosureLedger()
	noRow.Stems[0].PublicStatus = ""
	noRow.Stems[0].PublicRemaining = ""
	_, plain := renderRFCDetail(t, noRow, "rfc9999")
	// The summary DECLARES the absence; docs/features/rfc-status.md is
	// generated from that declaration, so the page names the authored fact
	// first and the generated file second.
	if !strings.Contains(plain, rfcNoPublicRow+", so its summary declares `| Support | - |` "+
		"and docs/features/rfc-status.md carries no row for RFC 9999.") {
		t.Error("a stem with no public row does not say where that absence is declared")
	}
}

// VALIDATES: AC-5 -- a recorded verdict appears with the word, its published
// meaning and its freshness state, and what moved where the verdict is not
// current.
func TestAnAuditVerdictAppearsWithItsMeaningAndFreshness(t *testing.T) {
	_, text, mirror := disclosurePage(t)
	for _, want := range []string{
		"RFC9999-2-1",
		"weak (the tests pass over code that does not enforce the requirement), fresh",
		"stale-unit: internal/b_test.go::TestLogged moved",
		"shifted: internal/c_test.go::TestResend moved",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the page does not state %q", want)
		}
		if !strings.Contains(mirror, want) {
			t.Errorf("the mirror does not state %q", want)
		}
	}
}

// VALIDATES: AC-5 -- a requirement nobody has audited says so, rather than
// showing a blank cell a reader would read as "nothing wrong".
func TestARequirementWithNoVerdictSaysSoRatherThanShowingABlank(t *testing.T) {
	_, text, mirror := disclosurePage(t)
	const want = "not audited: no reader has judged these tests"
	if !strings.Contains(text, want) {
		t.Errorf("the page does not state %q for a requirement carrying no verdict", want)
	}
	if !strings.Contains(mirror, want) {
		t.Errorf("the mirror does not state %q", want)
	}
	if strings.Contains(text, "RFC9999-6-1 not audited") {
		return
	}
	if !strings.Contains(text, "RFC9999-6-1") {
		t.Error("the unaudited requirement is not named on the page")
	}
}

// VALIDATES: AC-6 and AC-20 -- a tagged unit with no recorded break reads as
// unproven, in a cell of its own, on a row of its own.
//
// The retired shape joined every unit of one requirement into one cell with
// semicolons and repeated "no discrimination record: unproven" on each entry.
// The words that explain the state are the legend above the table now, stated
// once (owner review, 2026-09-01).
func TestATagWithNoDiscriminationRecordReadsAsUnproven(t *testing.T) {
	_, text, mirror := disclosurePage(t)
	// The row names the TEST, not the file it lives in: the path is machinery
	// and the link already resolves to the line (owner review, 2026-09-01).
	for _, want := range []string{
		"| negative | `TestNoWidget` | unit/verify | " + rfcUnproven + " |",
		"| positive | `TestAck` | unit/verify | " + rfcUnproven + " |",
	} {
		if !strings.Contains(mirror, want) {
			t.Errorf("the mirror carries no row %q", want)
		}
	}
	for _, want := range []string{
		"negative TestNoWidget unit/verify " + rfcUnproven,
		"positive TestAck unit/verify " + rfcUnproven,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the page does not state %q", want)
		}
	}
	if strings.Contains(text, "internal/a_test.go TestNoWidget") {
		t.Error("the proof-state row still prints the file beside the test")
	}
	for name, rendering := range map[string]string{"page": text, "mirror": mirror} {
		if strings.Count(rendering, rfcProofLegend) != 1 {
			t.Errorf("the %s states the proof legend %d times, want once",
				name, strings.Count(rendering, rfcProofLegend))
		}
		if strings.Contains(rendering, "no discrimination record: "+rfcUnproven) {
			t.Errorf("the %s still repeats the whole sentence on a unit row", name)
		}
	}
}

// VALIDATES: AC-6 and AC-20 -- a `no-break` record is named as the escape it is
// and is never counted as a proof.
//
// The escape says no break exists. A page that printed it as a route beside
// `mutant` and `revert` would publish the opposite of what the gate holds
// (docs/contributing/rfc-conformance-gates.md).
func TestANoBreakRecordIsCountedApartFromAProof(t *testing.T) {
	_, text, mirror := disclosurePage(t)
	const escape = "no-break escape (foreign-producer), which is not a proof, verified"
	if !strings.Contains(mirror, "| positive | `TestLogged` | unit/verify | "+escape+" |") {
		t.Error("the mirror carries no escape row for TestLogged")
	}
	if !strings.Contains(text, "positive TestLogged unit/verify "+escape) {
		t.Errorf("the page does not state %q for the escaped unit", escape)
	}
	if !strings.Contains(text, "revert, verified") {
		t.Error("a real proof does not read as a proof, so the two are indistinguishable")
	}
}

// VALIDATES: AC-7 -- the sign-off names the reviewer, the date, the source and
// its fingerprint, one row per section, and every excluded site with its kind
// and its reason.
func TestTheExtractionSignoffNamesEveryExcludedSiteAndItsKind(t *testing.T) {
	_, text, mirror := disclosurePage(t)
	for _, want := range []string{
		"a reader", "2026-09-01", "rfc/full/rfc9999.txt", "0123456789abcdef",
		"rfc/extraction/rfc9999.json",
		"skipped (references)", "the reference list states no obligation",
		"S3.1", "feature-out-of-scope",
		"widgets over TLS are OPTIONAL and Ze does not offer them",
		"S4.2", "relocated-to-spec",
		"a relay is a separate role (relocated to plan/spec-widget-relay.md as RFC9999-4-2)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the sign-off section does not state %q", want)
		}
		if !strings.Contains(mirror, want) {
			t.Errorf("the sign-off mirror does not state %q", want)
		}
	}

	none := disclosureLedger()
	none.Stems[0].Extraction = nil
	_, plain := renderRFCDetail(t, none, "rfc9999")
	if !strings.Contains(plain, "No extraction sign-off exists for RFC 9999") {
		t.Error("a stem with no sign-off does not say so, and an omitted section reads as a fact")
	}
}

// VALIDATES: AC-2's superseded half -- a requirement whose obligation moved
// names the disposition, the target and the reason.
func TestASupersededRequirementNamesWhereItsObligationWent(t *testing.T) {
	_, text, mirror := disclosurePage(t)
	for _, want := range []string{
		"RFC 9999 is obsoleted by RFC 9998.",
		"RFC9999-7-1", "restated", "RFC9998-2-1", "the successor states it",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the superseded section does not state %q", want)
		}
		if !strings.Contains(mirror, want) {
			t.Errorf("the superseded mirror does not state %q", want)
		}
	}
}

// VALIDATES: AC-12 and R-5 -- a pipe, a backtick, an angle bracket and an
// ampersand in quoted RFC prose each land inside their own cell, escaped, and
// break neither the row nor the markup.
func TestAPipeInRequirementTextStaysInItsCell(t *testing.T) {
	page, text, mirror := disclosurePage(t)
	const prose = "A widget MUST be counted | tallied <once> & only once."
	if !strings.Contains(text, prose) {
		t.Errorf("the page does not render %q as one readable cell", prose)
	}
	if !strings.Contains(page, "&lt;once&gt; &amp; only once.") {
		t.Error("the angle brackets and the ampersand are not escaped in the markup")
	}
	if strings.Contains(page, "<once>") {
		t.Error("quoted RFC prose reached the page as raw markup")
	}

	// The mirror is markdown, where a bare pipe closes a cell. The row has to
	// keep the header's own column count with the pipe escaped inside the text
	// cell. The count is READ from the header rather than written here: this
	// table lost two columns and gained one on 2026-09-01, and a number in a
	// test is the second place it can be wrong.
	// The FIRST row for this requirement, which is the requirements table's.
	// The proof-state table below it opens with the same id and carries fewer
	// cells, so taking the last match would judge the wrong row.
	row, header := "", ""
	for line := range strings.SplitSeq(mirror, "\n") {
		if header == "" && strings.HasPrefix(line, "| Requirement | Text | Level |") {
			header = line
		}
		if row == "" && strings.HasPrefix(line, "| `RFC9999-5-1` |") {
			row = line
		}
	}
	if row == "" || header == "" {
		t.Fatal("the mirror carries no requirements header or no row for RFC9999-5-1")
	}
	want := strings.Count(header, "|")
	if got := strings.Count(row, "|") - strings.Count(row, `\|`); got != want {
		t.Errorf("the mirror row splits into %d cells and the header into %d: %s",
			got, want, row)
	}
	if !strings.Contains(row, `counted \| tallied`) {
		t.Errorf("the pipe in the requirement text is not escaped: %s", row)
	}
}

// VALIDATES: AC-9 -- the mirror states every fact the page states.
//
// The at-a-glance facts are built once and read by both renderings, so this
// holds the two against each other over the whole list rather than over a
// sample.
func TestAnRFCPageMirrorReadsAsThePublishedMirror(t *testing.T) {
	entry := disclosureLedger().Stems[0]
	_, text, mirror := disclosurePage(t)
	for _, fact := range rfcGlanceFacts(&entry) {
		plain := rfcPlain(fact[1])
		if !strings.Contains(mirror, plain) {
			t.Errorf("the mirror does not state the %s fact %q", fact[0], plain)
		}
		if !strings.Contains(text, strings.ReplaceAll(plain, "`", "")) {
			t.Errorf("the page does not state the %s fact %q", fact[0], plain)
		}
	}
	for _, heading := range []string{
		"## At a glance", "## Coverage", "## Requirements",
		"## Gaps and untested MUSTs", "## Proof state",
		"## Extraction sign-off", "## Superseded",
	} {
		if !strings.Contains(mirror, heading) {
			t.Errorf("the mirror carries no %q section", heading)
		}
	}
}

// VALIDATES: AC-9 -- the published page carries the shell every page carries
// and the seven sections this family states.
func TestAnRFCPageReadsAsThePublishedPage(t *testing.T) {
	page, _, _ := disclosurePage(t)
	for _, chrome := range []string{
		"<title>RFC 9999 requirement ledger - Ze</title>",
		`<link rel="canonical" href="https://ze-software.net/quality/rfc-compliance/rfc9999/" />`,
		`<link rel="stylesheet" href="../../../assets/site.css" />`,
		`<section aria-labeledby="rfc-detail-title" class="md-content reveal cat-observe">`,
		`<h1 id="rfc-detail-title">RFC 9999 - The Widget Protocol</h1>`,
		"<footer>",
	} {
		if !strings.Contains(page, chrome) {
			t.Errorf("the page is missing %q", chrome)
		}
	}
	for _, heading := range []string{
		"<h2>At a glance</h2>", "<h2>Coverage</h2>", "<h2>Requirements</h2>",
		"<h2>Gaps and untested MUSTs</h2>", "<h2>Proof state</h2>",
		"<h2>Extraction sign-off</h2>", "<h2>Superseded</h2>",
	} {
		if !strings.Contains(page, heading) {
			t.Errorf("the page carries no %s", heading)
		}
	}
}

// VALIDATES: AC-10 and R-7 -- a summary dropped from rfc/short loses its
// published directory on the next build.
//
// A page that survives on the incremental seed alone is frozen content with a
// fresh timestamp, and every other check the artifact carries passes it.
func TestARetiredRFCLosesItsPage(t *testing.T) {
	paths := rfcCompliancePathsWith(t, publishedRFCComplianceRef(t), twoStemLedger())
	if _, err := renderRFCCompliance(paths); err != nil {
		t.Fatal(err)
	}
	retired := filepath.Join(paths.Output, filepath.FromSlash(rfcComplianceDirectory), "rfc9997")
	if _, err := os.Stat(retired); err != nil {
		t.Fatalf("the first build published no page for rfc9997: %v", err)
	}

	shrunk := rfcLedger{Stems: twoStemLedger().Stems[:1]}
	writeLedgerArtifact(t, paths.Output, shrunk)
	if _, err := renderRFCCompliance(paths); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(retired); !os.IsNotExist(err) {
		t.Errorf("the page for a retired summary is still published: %v", err)
	}
	kept := filepath.Join(paths.Output, filepath.FromSlash(rfcComplianceDirectory), "rfc9998")
	if _, err := os.Stat(kept); err != nil {
		t.Errorf("the retirement pass removed a live summary's page: %v", err)
	}
}

// VALIDATES: AC-16 -- the new site code spells no annotation kind, polarity,
// audit verdict, freshness state or discrimination route of its own.
//
// Every one of those words is a vocabulary internal/le/rfc declares and closes.
// A literal here is a second declaration of a closed set, which is where the
// verdict vocabulary drifted before the schema existed (ai/rules/principles.md).
func TestTheSiteReadsItsRFCVocabularyFromThePackage(t *testing.T) {
	vocabulary := append([]string{}, rfc.AnnotationKinds()...)
	vocabulary = append(vocabulary, rfc.Polarities()...)
	vocabulary = append(vocabulary, rfc.AuditVerdicts()...)
	vocabulary = append(vocabulary, rfc.FreshnessStates()...)
	vocabulary = append(vocabulary, rfc.DiscriminationRoutes()...)
	vocabulary = append(vocabulary, rfc.SiteDispositions()...)

	for _, name := range []string{
		"rfcdetail.go", "rfcledger.go", "rfcmarkup.go", "rfcevidence.go",
	} {
		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, word := range vocabulary {
			literal := `"` + word + `"`
			if strings.Contains(string(content), literal) {
				t.Errorf("%s spells %s as a literal; read it from internal/le/rfc instead",
					name, literal)
			}
		}
	}
}

// VALIDATES: AC-17 -- every one of the five states the owner ruling names is on
// the page, under the requirement id it belongs to, in its own word.
//
// Disclosure is FULL by owner ruling of 2026-09-01. None of these is omitted,
// relabeled, softened, shown as a blank cell, or counted as proven. If this
// test goes red, the PAGE is what gets fixed.
func TestEveryDisclosedStateAppearsUnderItsRequirementID(t *testing.T) {
	ledger := disclosureLedger()
	all := rfcAllRIDs(&ledger.Stems[0])
	// The RAW page, not its visible text: the declaration markers that bound
	// one requirement's block are the anchors, and text extraction drops them.
	page, text, mirror := disclosurePage(t)
	for _, one := range []struct{ state, rid, word string }{
		{"a gated MUST with no test", "RFC9999-1-1", "no test"},
		{"a weak audit verdict", "RFC9999-2-1", "weak"},
		{"a stale-unit verdict", "RFC9999-3-1", "stale-unit"},
		{"a shifted verdict", "RFC9999-4-1", "shifted"},
		{"a tagged unit with no discrimination record", "RFC9999-6-1", rfcUnproven},
		{"a no-break record", "RFC9999-3-1", "no-break"},
		{"a declared gap", "RFC9999-5-1", "{gap}"},
	} {
		if !strings.Contains(text, one.rid) {
			t.Errorf("the page does not name %s, which carries %s", one.rid, one.state)
			continue
		}
		for name, rendering := range map[string]string{"page": page, "mirror": mirror} {
			// The word is held against the requirement's OWN block, never
			// against the whole rendering: a page-wide search passes on a
			// word another requirement disclosed.
			if !strings.Contains(rfcDisclosureUnit(rendering, one.rid, all), one.word) {
				t.Errorf("the %s does not carry the word %q under %s, which carries %s",
					name, one.word, one.rid, one.state)
			}
		}
	}
}

// VALIDATES: AC-18 -- for every count of a bad state the page prints, the
// requirement ids that produced it are on the same page.
//
// A page that says "2 requirements carry no test" and does not name the two has
// reproduced the aggregate this family exists to replace. The method is the
// snapshot's own failing rows rather than the page's summary of them.

// rfcBadState names one weakness a requirement carries and the WORD the page
// owes for it under that requirement's own id.
type rfcBadState struct{ family, word string }

// rfcBadStates answers every weakness one requirement carries.
//
// One list, read by the fixture test and by the corpus test, so the two cannot
// hold the page to different disclosures. The word is what the reader sees, so
// a section that stopped rendering goes red here even though the requirement id
// is still printed by the requirements table (independent review, 2026-09-01).
func rfcBadStates(requirement *rfcLedgerRequirement) []rfcBadState {
	var states []rfcBadState
	if requirement.Gated && len(requirement.Covers) == 0 {
		states = append(states, rfcBadState{"gated MUST with no test", "no test"})
	}
	if requirement.NightlyOnly {
		states = append(states, rfcBadState{"nightly-only evidence", "nightly-only"})
	}
	if requirement.Annotation != nil && requirement.Annotation.Kind == rfc.AnnotationGap {
		states = append(states, rfcBadState{"declared gap", "{" + rfc.AnnotationGap + "}"})
	}
	if requirement.Audit != nil && requirement.Audit.Verdict != rfc.VerdictEnforced {
		states = append(states, rfcBadState{"verdict other than enforced",
			requirement.Audit.Verdict})
	}
	if requirement.Audit != nil && requirement.Audit.Freshness != rfc.FreshState {
		states = append(states, rfcBadState{"verdict no longer current",
			requirement.Audit.Freshness})
	}
	for index := range requirement.Covers {
		cover := &requirement.Covers[index]
		if cover.Proof == nil {
			states = append(states, rfcBadState{"tagged unit with no record", rfcUnproven})
			continue
		}
		if !cover.Proof.Proves {
			states = append(states, rfcBadState{"escape that is not a proof",
				cover.Proof.Route + " escape"})
		}
	}
	return states
}

func TestNoBadStateIsPublishedOnlyAsACount(t *testing.T) {
	ledger := disclosureLedger()
	entry := &ledger.Stems[0]
	all := rfcAllRIDs(entry)
	page, _, mirror := disclosurePage(t)

	held := 0
	for index := range entry.Requirements {
		requirement := &entry.Requirements[index]
		for _, state := range rfcBadStates(requirement) {
			held++
			for name, rendering := range map[string]string{"page": page, "mirror": mirror} {
				unit := rfcDisclosureUnit(rendering, requirement.RID, all)
				if unit == "" {
					t.Errorf("the %s counts %s and does not name %s",
						name, state.family, requirement.RID)
					continue
				}
				if !strings.Contains(unit, state.word) {
					t.Errorf("the %s counts %s for %s and says %q nowhere under it",
						name, state.family, requirement.RID, state.word)
				}
			}
		}
	}
	if held == 0 {
		t.Fatal("the disclosure fixture carries no bad state, so this proves nothing")
	}
}

// VALIDATES: A-1 -- the navigation entry for /quality/rfc-compliance/ claims
// every route under it, so 190 child pages need no nav.json edit.
//
// assignPages FAILS THE BUILD on a published page no navigation section claims.
// The method is the real navigation and the real published route set, with this
// family's routes added to it.
func TestEveryRFCDetailRouteBelongsToOneSection(t *testing.T) {
	root := repositoryRoot(t)
	var nav siteNav
	if err := readSourceJSON(filepath.Join(root, "website"), navDataFile, &nav); err != nil {
		t.Fatal(err)
	}
	stems, err := rfcSummaryStems(root)
	if err != nil {
		t.Fatal(err)
	}
	routes := publishedArtifactRoutes(t)
	for _, stem := range stems {
		routes = append(routes, rfcComplianceRoute+stem+"/")
	}
	pages := make([]Page, 0, len(routes))
	for _, route := range routes {
		directory := strings.Trim(route, "/")
		pages = append(pages, Page{
			Route:    route,
			HTML:     filepath.ToSlash(filepath.Join(directory, pageIndexFile)),
			Markdown: filepath.ToSlash(filepath.Join(directory, pageMirrorFile)),
		})
	}
	if _, err := assignPages(pages, nav); err != nil {
		t.Fatalf("the RFC detail routes belong to no navigation section: %v", err)
	}
}

// VALIDATES: AC-17 over this checkout's own corpus rather than over the
// disclosure fixture alone.
//
// A fixture proves the renderer discloses what it is handed. This proves the
// LEDGER hands it the real weaknesses: every gated MUST this repository carries
// no test for is named on the page for its RFC, under its own requirement id.
// The count is logged rather than pinned, because it moves with every
// enrolment; what is pinned is that none of them is missing from a page.
func TestEveryUntestedMustOfThisCheckoutIsNamedOnItsPage(t *testing.T) {
	root := repositoryRoot(t)
	ledger, err := collectRequirementLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	untested := 0
	pages := 0
	for index := range ledger.Stems {
		entry := &ledger.Stems[index]
		var owed []string
		for position := range entry.Requirements {
			requirement := &entry.Requirements[position]
			if requirement.Gated && len(requirement.Covers) == 0 {
				owed = append(owed, requirement.RID)
			}
		}
		if len(owed) == 0 {
			continue
		}
		pages++
		untested += len(owed)
		mirror := rfcDetailMirror(entry)
		body := rfcDetailBody(entry)
		all := rfcAllRIDs(entry)
		for _, rid := range owed {
			// "no test" under the id, not the id alone: every requirement id
			// is emitted by the requirements table whatever else the page
			// says, so naming it proves nothing about the disclosure.
			if !strings.Contains(rfcDisclosureUnit(mirror, rid, all), "no test") {
				t.Errorf("%s carries no test for %s and its mirror does not say so under it",
					entry.Stem, rid)
			}
			if !strings.Contains(rfcDisclosureUnit(body, rid, all), "no test") {
				t.Errorf("%s carries no test for %s and its page does not say so under it",
					entry.Stem, rid)
			}
		}
	}
	if untested == 0 {
		t.Fatal("this checkout carries no untested gated MUST, so this proves nothing")
	}
	t.Logf("%d gated MUST-level requirements carry no test, across %d published pages",
		untested, pages)
}

// VALIDATES: AC-18 over this checkout's own corpus -- every bad state the
// snapshot holds for which a requirement id exists is named on the page, and
// not only counted in the at-a-glance panel.
func TestNoBadStateOfThisCheckoutIsPublishedOnlyAsACount(t *testing.T) {
	root := repositoryRoot(t)
	ledger, err := collectRequirementLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	named := map[string]int{}
	for index := range ledger.Stems {
		entry := &ledger.Stems[index]
		all := rfcAllRIDs(entry)
		mirror := ""
		for position := range entry.Requirements {
			requirement := &entry.Requirements[position]
			states := rfcBadStates(requirement)
			if len(states) == 0 {
				continue
			}
			if mirror == "" {
				mirror = rfcDetailMirror(entry)
			}
			unit := rfcDisclosureUnit(mirror, requirement.RID, all)
			for _, state := range states {
				named[state.family]++
				if !strings.Contains(unit, state.word) {
					t.Errorf("%s carries %s for %s and the page says %q nowhere under it",
						entry.Stem, state.family, requirement.RID, state.word)
				}
			}
		}
	}
	if len(named) == 0 {
		t.Fatal("this checkout holds no bad state at all, so this proves nothing")
	}
	for family, count := range named {
		t.Logf("%d occurrence(s) of %s, each named under its requirement id", count, family)
	}
}

// VALIDATES: AC-19 -- the page opens with the same card grid the index carries,
// rendered by the same function, over one summary's own numbers, and a number
// that is not good news carries a tone that says so.
//
// The method is the tone of each card against the fixture's own counts rather
// than a fixed list of tones, so a card that stopped reading its number goes
// red here.
func TestAStemPageOpensWithTheCardGridTheIndexCarries(t *testing.T) {
	entry := disclosureLedger().Stems[0]
	page, text, mirror := disclosurePage(t)
	cards := rfcDetailCards(&entry)
	if len(cards) == 0 {
		t.Fatal("the page renders no card")
	}
	whole := entry.Coverage.Binding()
	if !strings.Contains(page, rfcCardsHTML(cards, whole)) {
		t.Error("the page does not carry the markup rfcCardsHTML answers for its own cards")
	}
	if !strings.Contains(mirror, rfcCardsMirror(cards, whole)) {
		t.Error("the mirror does not carry the table rfcCardsMirror answers")
	}
	// The percentage and the arithmetic behind it are BOTH on the card, the
	// count under the value (owner ruling, 2026-09-01).
	for _, card := range cards {
		if !strings.Contains(text, card.Label+card.Value+card.Count+card.Note) {
			t.Errorf("the %s card does not read %q / %q / %q", card.Label, card.Value,
				card.Count, card.Note)
		}
		if card.Count == "" {
			t.Errorf("the %s card states a value and no arithmetic behind it", card.Label)
		}
	}

	// The fixture's seven gated MUSTs all bind: none is {not-applicable}. Three
	// carry both polarities, three carry no test at all (one declared gap and
	// two unexcused), and of seven tagged units three carry a record, one of
	// which is an escape, so two are proven.
	tones := map[string]string{}
	for _, card := range cards {
		tones[card.Label] = card.Tone
	}
	// A color names what the measure MEANS, not how well Ze scores on it: a
	// good outcome is green at any value, a bad one is red above zero, and
	// neither a population nor a scope count is an outcome.
	for label, want := range map[string]string{
		"Tested both ways":           rfcToneOK,
		"One polarity plus reason":   rfcToneOK,
		"One polarity, unexcused":    rfcToneBad,
		"No test at all":             rfcToneBad,
		"Proven by a recorded break": rfcToneOK,
		"Out of scope":               rfcToneNeutral,
		"Gated MUSTs":                rfcToneNeutral,
		"Audit verdicts":             rfcToneBad,
	} {
		if tones[label] != want {
			t.Errorf("the %s card reads %q, want %q: a bad number must read as bad",
				label, tones[label], want)
		}
	}

	// SCALE leads and STANDING follows, by owner amendment of 2026-09-01, which
	// supersedes the ratio-first order ruled earlier the same day. The
	// population is safe at the head only while it is labeled as scale and the
	// coverage shares sit in the same grid immediately after it.
	if cards[0].Label != "Gated MUSTs" {
		t.Errorf("the grid opens with %q, want the population it is a scale of", cards[0].Label)
	}
	if cards[0].Tone != rfcToneNeutral {
		t.Errorf("the leading population card reads %q, so it looks like a result",
			cards[0].Tone)
	}
	if !strings.Contains(cards[0].Note, "A population, not a result") {
		t.Errorf("the leading card does not say it is a population: %q", cards[0].Note)
	}
	if !strings.HasSuffix(cards[2].Value, "%") {
		t.Errorf("the third card is %q, so a reader meets no coverage share beside the scale",
			cards[2].Label)
	}

	clean := disclosureLedger()
	for index := range clean.Stems[0].Requirements {
		requirement := &clean.Stems[0].Requirements[index]
		requirement.Annotation, requirement.Audit = nil, nil
		for coverIndex := range requirement.Covers {
			requirement.Covers[coverIndex].Proof = &rfcLedgerProof{Verified: true, Proves: true}
		}
	}
	clean.Stems[0].Coverage.Gaps, clean.Stems[0].Coverage.GatedGaps = 0, 0
	clean.Stems[0].Coverage.Missing, clean.Stems[0].Coverage.NotApplicable = 0, 0
	clean.Stems[0].Coverage.Gated, clean.Stems[0].Coverage.Both = 4, 4
	clean.Stems[0].Coverage.One, clean.Stems[0].Coverage.SinglePolarity = 0, 0
	clean.Stems[0].Coverage.Audited = 4
	clean.Stems[0].Coverage.Units, clean.Stems[0].Coverage.Records = 5, 5
	clean.Stems[0].Coverage.Escapes = 0
	for _, card := range rfcDetailCards(&clean.Stems[0]) {
		if card.Tone == rfcToneBad || card.Tone == rfcToneWarn {
			t.Errorf("a summary with nothing wrong reads %q on the %s card",
				card.Tone, card.Label)
		}
	}
}

// VALIDATES: AC-21 -- a requirement id mentioned away from its own row links to
// that row and carries the requirement's text.
//
// A bare id tells a reader which line of a shard to go and read. This page
// carries the line (owner review, 2026-09-01).
func TestAMentionedRequirementLinksToItsOwnRow(t *testing.T) {
	page, text, mirror := disclosurePage(t)
	if !strings.Contains(page, `<code id="rfc9999-5-1">RFC9999-5-1</code>`) {
		t.Error("the requirement row carries no stable anchor")
	}
	for _, want := range []string{
		`<a href="#rfc9999-5-1"><code>RFC9999-5-1</code></a>`,
		`<a href="#rfc9999-1-1"><code>RFC9999-1-1</code></a>`,
		`<a href="#rfc9999-7-1"><code>RFC9999-7-1</code></a>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page carries no link %q", want)
		}
	}
	if !strings.Contains(mirror, "[`RFC9999-5-1`](#rfc9999-5-1)") {
		t.Error("the mirror mentions a requirement without linking it to its row")
	}
	// The gap row and the superseded row each carry the requirement's own text
	// beside the id.
	for _, want := range []string{
		"A widget MUST be counted | tallied <once> & only once.",
		"A widget MUST be retired.",
	} {
		if strings.Count(text, want) < 2 {
			t.Errorf("%q appears once, so the mention away from the row is bare", want)
		}
	}
}

// VALIDATES: AC-22 -- the at-a-glance table carries only countable facts and
// repository paths, and the public remainder is stated exactly once.
func TestTheGlanceTableCarriesNoProse(t *testing.T) {
	entry := disclosureLedger().Stems[0]
	_, text, mirror := disclosurePage(t)
	for _, fact := range rfcGlanceFacts(&entry) {
		if len(rfcPlain(fact[1])) > 80 {
			t.Errorf("the %s cell is %d characters, which is prose in a two-column table",
				fact[0], len(rfcPlain(fact[1])))
		}
	}
	for _, prose := range []string{
		"Enrolled: every MUST carries both polarities",
		"widgets only",
	} {
		if !strings.Contains(text, prose) {
			t.Errorf("the page dropped %q, which moved out of the table and owes a heading", prose)
		}
		if !strings.Contains(mirror, prose) {
			t.Errorf("the mirror dropped %q", prose)
		}
	}
	if count := strings.Count(mirror, "`RFC9999-6-1`](#rfc9999-6-1) unmet"); count != 1 {
		t.Errorf("the mirror states the public remainder %d times, want once", count)
	}
}

// VALIDATES: AC-23 -- a section with nothing to show says so once.
//
// The retired page printed the Superseded sentence and then a bare "None." on
// the next line, and did the same under Gaps. Two statements of one emptiness
// read as two facts, and only one of them is (owner review, 2026-09-01).
func TestAnEmptySectionStatesItsEmptinessOnce(t *testing.T) {
	empty := disclosureLedger()
	stem := &empty.Stems[0]
	stem.Successor = ""
	stem.PublicRemaining = "nothing outstanding"
	for index := range stem.Requirements {
		requirement := &stem.Requirements[index]
		requirement.Annotation, requirement.Superseded = nil, nil
		if len(requirement.Covers) == 0 {
			requirement.Covers = []rfcLedgerCover{{Polarity: rfc.PolarityPositive,
				Unit: "internal/z_test.go::TestZ", Carrier: "unit/verify", Tags: 1}}
		}
	}
	page, mirror := renderRFCDetail(t, empty, "rfc9999")
	text := visibleText(mainContent(t, page))
	for name, rendering := range map[string]string{"page": text, "mirror": mirror} {
		if strings.Contains(rendering, "None.") {
			t.Errorf("the %s still states an emptiness as a bare \"None.\"", name)
		}
		if !strings.Contains(rendering, "declares no gap, and every gated MUST it carries") {
			t.Errorf("the %s does not say in words that it declares no gap", name)
		}
		if !strings.Contains(rendering,
			"No document obsoletes RFC 9999, so its obligations are stated where they were written.") {
			t.Errorf("the %s does not say in words that no document obsoletes it", name)
		}
	}
}

// VALIDATES: AC-24 -- a coverage bucket carries a count and no list of ids, and
// the membership of each weakness is a labeled list under the table with every
// id linked.
func TestACoverageBucketCarriesACountAndNotAList(t *testing.T) {
	entry := disclosureLedger().Stems[0]
	page, text, mirror := disclosurePage(t)
	for _, bucket := range rfcCoverageBuckets(&entry) {
		if !strings.Contains(mirror, "| "+bucket.Label+" | "+strconv.Itoa(bucket.Count)+" |") {
			t.Errorf("the mirror carries no count row for %q", bucket.Label)
		}
		if len(bucket.IDs) == 0 {
			continue
		}
		label := bucket.Label + " (" + strconv.Itoa(len(bucket.IDs)) + "):"
		if !strings.Contains(text, label) {
			t.Errorf("the page states no membership list for %q", bucket.Label)
		}
		if !strings.Contains(mirror, "**"+label+"**") {
			t.Errorf("the mirror states no membership list for %q", bucket.Label)
		}
		for _, rid := range bucket.IDs {
			if !strings.Contains(page, `<a href="#`+rfcAnchor(rid)+`"><code>`+rid+"</code></a>") {
				t.Errorf("%s is named in %q and does not link to its row", rid, bucket.Label)
			}
		}
	}
}

// VALIDATES: AC-25 -- a requirement row names its section's own title where the
// extraction sign-off states one.
func TestARequirementRowNamesItsSection(t *testing.T) {
	named := disclosureLedger()
	named.Stems[0].Extraction.Sections = []rfcLedgerSection{
		{ID: "1", Name: "Constructing the Widget", Sites: 3, Disposition: "walked",
			Reason: "Constructing the Widget. The only section that binds a speaker."},
	}
	page, mirror := renderRFCDetail(t, named, "rfc9999")
	text := visibleText(mainContent(t, page))
	if !strings.Contains(text, "1 - Constructing the Widget") {
		t.Error("the requirement row names the section number alone")
	}
	if !strings.Contains(mirror, "| 1 - Constructing the Widget |") {
		t.Error("the mirror names the section number alone")
	}
	if !strings.Contains(text, "Constructing the Widget") {
		t.Error("the sign-off table does not carry the section name")
	}
}

// VALIDATES: AC-26 -- every table this family publishes sits in the container
// that scrolls it, so the page body never scrolls sideways.
//
// A test path is one unbreakable token, and a requirement row carries several
// beside quoted RFC prose. The convention is the one .cmd-eq-table-wrap already
// holds for the command family.
func TestEveryTableOnAStemPageScrollsInsideItsOwnContainer(t *testing.T) {
	page, _, _ := disclosurePage(t)
	body := mainContent(t, page)
	tables := strings.Count(body, `<table class="rfc-table">`)
	wrapped := strings.Count(body, `<div class="rfc-table-wrap">`+"\n"+`<table class="rfc-table">`)
	if tables == 0 {
		t.Fatal("the page publishes no table, so this proves nothing")
	}
	if wrapped != tables {
		t.Errorf("%d of %d tables sit in a scrolling container", wrapped, tables)
	}
	if !strings.Contains(page, ".rfc-table-wrap { overflow-x: auto; }") {
		t.Error("the page carries the container and not the rule that scrolls it")
	}
}

// VALIDATES: AC-28 -- a requirement that is both a declared gap and a gated
// MUST with no test takes one row naming both, and quotes its reason once.
//
// The first pass emitted two rows under one id, each carrying the same
// annotation prose. Two rows read as two findings, and there is one (owner
// review, 2026-09-01).
func TestAGapThatIsAlsoUntestedTakesOneRow(t *testing.T) {
	entry := disclosureLedger().Stems[0]
	rows := rfcGapRows(&entry)
	seen := map[string]int{}
	for _, row := range rows {
		seen[row.RID]++
	}
	for rid, count := range seen {
		if count != 1 {
			t.Errorf("%s takes %d rows under Gaps and untested MUSTs, want one", rid, count)
		}
	}
	var gap *rfcGapRow
	for index := range rows {
		if rows[index].RID == "RFC9999-5-1" {
			gap = &rows[index]
		}
	}
	if gap == nil {
		t.Fatal("the declared gap has no row")
	}
	if gap.Kind != "{"+rfc.AnnotationGap+"}, no test" {
		t.Errorf("the row states %q, want both states", gap.Kind)
	}
	_, text, mirror := disclosurePage(t)
	const reason = "the counter is not implemented | not scheduled"
	// Twice, and only twice: the Note cell of the requirement row and the
	// Reason cell of the gap row. The mirror escapes the pipe inside a cell.
	for name, one := range map[string]struct{ rendering, quoted string }{
		"page":   {text, reason},
		"mirror": {mirror, rfc.TableCell(reason)},
	} {
		if count := strings.Count(one.rendering, one.quoted); count != 2 {
			t.Errorf("the %s quotes the gap reason %d times, want twice", name, count)
		}
	}
}

// VALIDATES: AC-57 -- the requirement's sentence LEADS its row, always visible,
// above the level, the section and the tests.
//
// It was behind a `details` in the id cell (AC-31, 2026-09-01) and then in a
// spanning row the same day. Both hid or narrowed the thing the row is ABOUT
// while showing its metadata, which is inside out (owner review). The
// disclosure is gone: the subject is the first thing on the row.
func TestTheRequirementTextLeadsItsRow(t *testing.T) {
	page, text, mirror := disclosurePage(t)
	body := mainContent(t, page)
	head := sliceBetween(t, body, "<h2>Requirements</h2>", "</thead>")

	// The header is the metadata only: the subject is not a column.
	for _, want := range []string{
		"<th>Requirement</th>", "<th>Level</th>", "<th>Section</th>", "<th>Tests</th>",
	} {
		if !strings.Contains(head, want) {
			t.Errorf("the requirements table lost %s", want)
		}
	}
	for _, gone := range []string{"<th>Text</th>", "<th>Note</th>"} {
		if strings.Contains(head, gone) {
			t.Errorf("the requirements table still carries %s", gone)
		}
	}
	if strings.Contains(body, `<details class="rfc-text">`) {
		t.Error("the requirement text is still behind a disclosure")
	}
	const sentence = "A widget MUST be rejected when its length is zero."
	if !strings.Contains(text, sentence) {
		t.Errorf("the page no longer carries %q at all", sentence)
	}
	// The sentence sits BESIDE the id, in the cell next to it, so every
	// sentence on the page starts at the same offset.
	if !strings.Contains(page, `<td class="rfc-subject-id"><code id="rfc9999-2-1">`+
		`RFC9999-2-1</code></td><td colspan="3"><span class="rfc-subject">`) {
		t.Error("the sentence does not sit beside the id in its own cell")
	}
	if !strings.Contains(mirror, sentence) {
		t.Errorf("the mirror no longer carries %q", sentence)
	}
}

// VALIDATES: AC-32 -- a proof-state row links the TEST to the line its tag is
// written on, and the file cell is not a link.
//
// rfc.Tag records the line, so the link lands on the assertion rather than on a
// 900-line file. One repository-blob helper answers the URL for this page and
// for the documentation renderer, so no second URL literal exists.
func TestAProofStateRowLinksTheTestToItsOwnLine(t *testing.T) {
	linked := disclosureLedger()
	cover := &linked.Stems[0].Requirements[1].Covers[0]
	cover.File, cover.Line = "internal/a_test.go", 412
	page, _, mirror := renderRFCDetailPage(t, linked)

	want := repositoryLineURL("internal/a_test.go", 412)
	if want != "https://github.com/ze-software/ze/blob/main/internal/a_test.go#L412" {
		t.Fatalf("the blob helper answers %q, which is not where the repository is published", want)
	}
	// The name is the link text and the full path is the link's TITLE, so a
	// reader who needs the package can hover and nobody pays for it in width.
	if !strings.Contains(page, `<a href="`+want+
		`" title="internal/a_test.go" target="_blank" rel="noopener"><code>TestWidget</code></a>`) {
		t.Error("the test name is not linked to the line its tag is written on")
	}
	if !strings.Contains(mirror, "[`TestWidget`]("+want+")") {
		t.Error("the mirror does not link the test name")
	}
	if strings.Contains(page, `<a href="`+repositoryBlobURL("internal/a_test.go")+`"><code>`) {
		t.Error("the link is on the path, which is what the review asked to move")
	}

	// A tag with no recorded line addresses the file, and a cover with no file
	// at all is stated unlinked rather than linked to nothing.
	if got := repositoryLineURL("internal/a_test.go", 0); got != repositoryBlobURL("internal/a_test.go") {
		t.Errorf("a lineless tag addresses %q, want the file", got)
	}
	bare := disclosureLedger()
	bare.Stems[0].Requirements[1].Covers[0].File = ""
	barePage, _, _ := renderRFCDetailPage(t, bare)
	if strings.Contains(barePage, `rel="noopener"><code>TestWidget</code></a>`) {
		t.Error("a cover with no file was linked anyway")
	}
}

// renderRFCDetailPage renders one stated ledger's rfc9999 page, its visible
// text and its mirror.
func renderRFCDetailPage(t *testing.T, ledger rfcLedger) (page, text, mirror string) {
	t.Helper()
	page, mirror = renderRFCDetail(t, ledger, "rfc9999")
	return page, visibleText(mainContent(t, page)), mirror
}

// VALIDATES: AC-33 -- the public ledger's cell is three labeled facts, the long
// halves are folded, and nothing is dropped.
func TestThePublicLedgerCellIsLabelledAndFolded(t *testing.T) {
	long := disclosureLedger()
	long.Stems[0].PublicCoverage = strings.Repeat("widgets are covered in every direction. ", 12)
	page, text, mirror := renderRFCDetailPage(t, long)

	if !strings.Contains(text, "Status: Partial") {
		t.Error("the section does not label the public status")
	}
	if !strings.Contains(page,
		`<details class="rfc-fold"><summary>What the ledger says is covered</summary>`) {
		t.Error("a long coverage cell is not folded behind a disclosure")
	}
	if !strings.Contains(text, strings.TrimSpace(long.Stems[0].PublicCoverage)) {
		t.Error("folding the coverage cell dropped part of it")
	}
	if !strings.Contains(mirror, "**What the ledger says is covered**") {
		t.Error("the mirror does not label the folded coverage cell")
	}

	// A short cell is stated in front of the reader rather than hidden behind a
	// control they have to find.
	short := disclosureLedger()
	short.Stems[0].PublicCoverage = "widgets only"
	shortPage, shortText, _ := renderRFCDetailPage(t, short)
	if !strings.Contains(shortText, "What the ledger says is covered: widgets only") {
		t.Error("a short coverage cell is not stated plainly")
	}
	if strings.Contains(shortPage,
		`<summary>What the ledger says is covered</summary>`) {
		t.Error("a six-word cell was folded behind a disclosure")
	}

	// The remainder is stated HERE and nowhere else.
	if count := strings.Count(shortText, "RFC9999-6-1 unmet"); count != 1 {
		t.Errorf("the page states the public remainder %d times, want once", count)
	}
}

// sliceBetween answers the markup between two markers, failing when either is
// absent so a renamed section cannot silently empty an assertion.
func sliceBetween(t *testing.T, body, from, to string) string {
	t.Helper()
	start := strings.Index(body, from)
	if start < 0 {
		t.Fatalf("the page carries no %q", from)
	}
	rest := body[start:]
	end := strings.Index(rest, to)
	if end < 0 {
		t.Fatalf("the page carries no %q after %q", to, from)
	}
	return rest[:end]
}

// VALIDATES: AC-51 -- the ledger prose is SPLIT, never rewritten. Rejoining the
// items reproduces the author's bytes, over every cell in the corpus.
//
// These cells are the disclosure. A clause lost to a splitter is disclosure
// lost, so the property this holds is total: every cell of every summary, both
// directions, byte for byte (owner ruling, 2026-09-01).
func TestEveryLedgerCellSplitsWithoutLoss(t *testing.T) {
	ledger := publishedLedgerOfThisCheckout(t)
	cells, split := 0, 0
	for index := range ledger.Stems {
		entry := &ledger.Stems[index]
		for name, prose := range map[string]string{
			"coverage":  entry.PublicCoverage,
			"remaining": entry.PublicRemaining,
		} {
			if strings.TrimSpace(prose) == "" {
				continue
			}
			cells++
			items := rfcProseSplit(prose)
			if got := rfcProseJoin(items); got != prose {
				t.Errorf("%s %s: the claim split loses text\n got %q\nwant %q",
					entry.Stem, name, got, prose)
			}
			// Losslessness alone does not prove the split is in the right
			// places: a naive cut at every semicolon rejoins byte for byte too.
			// A correct one leaves every item balanced.
			for _, item := range items {
				if !rfcProseBalanced(item) {
					t.Errorf("%s %s: the split left an unbalanced item %q",
						entry.Stem, name, item)
				}
			}
			themes := rfcProseThemes(prose)
			if got := rfcProseThemesJoin(themes); got != prose {
				t.Errorf("%s %s: the theme split loses text\n got %q\nwant %q",
					entry.Stem, name, got, prose)
			}
			if len(items) > 1 || len(themes) > 1 {
				split++
			}
		}
	}
	if cells == 0 {
		t.Fatal("this checkout publishes no ledger prose, so this proves nothing")
	}
	t.Logf("%d ledger cells, %d of them split into items", cells, split)
}

// VALIDATES: AC-52 -- every path this prose links resolves to a file in the
// tree, and every requirement id it links is one the RFC declares.
//
// The prose also cites RELATIVE paths, and a link built from one addresses
// nothing. The rule is the repository root, and this is what proves the rule
// rather than trusting it.
func TestEveryLinkedPathExistsInTheTree(t *testing.T) {
	root := repositoryRoot(t)
	ledger := publishedLedgerOfThisCheckout(t)
	linked, skipped := 0, 0
	for index := range ledger.Stems {
		entry := &ledger.Stems[index]
		declared := rfcDeclaredIDs(entry)
		for _, prose := range []string{entry.PublicCoverage, entry.PublicRemaining} {
			for _, path := range rfcRepositoryPath.FindAllString(prose, -1) {
				if !rfcLinkablePath(path) {
					skipped++
					continue
				}
				linked++
				if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
					t.Errorf("%s links %s, which is not a file in this tree", entry.Stem, path)
				}
			}
			// An id this RFC does not declare is left as text: a link to a row
			// the page does not carry is a link nobody can follow.
			for _, id := range rfcRequirementID.FindAllString(prose, -1) {
				if declared[id] {
					continue
				}
				if strings.Contains(rfcProseMirror(prose, declared), "["+"`"+id+"`](#") {
					t.Errorf("%s links %s, which it does not declare", entry.Stem, id)
				}
			}
		}
	}
	if linked == 0 {
		t.Fatal("this checkout links no path from its ledger prose, so this proves nothing")
	}
	t.Logf("%d repository paths linked, %d relative citations left as text", linked, skipped)
}

// publishedLedgerOfThisCheckout derives the real requirement ledger.
func publishedLedgerOfThisCheckout(t *testing.T) rfcLedger {
	t.Helper()
	ledger, err := collectRequirementLedger(repositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	return ledger
}

// VALIDATES: AC-51 -- a Coverage cell renders as its claims and a Remaining
// cell as its themes, with the author's words unchanged.
func TestTheLedgerProseRendersAsItsOwnStructure(t *testing.T) {
	entry := disclosureLedger().Stems[0]
	// The first claim carries a semicolon inside PARENTHESES and another inside
	// a code span. Both are the cuts a naive splitter makes and this one must
	// not.
	entry.PublicCoverage = "the first claim, which cites RFC9999-2-1 " +
		"(internal/a.go; and a `del { med; }` directive); the second claim; the third"
	entry.PublicRemaining = "Two gaps remain. Header handling: RFC9999-1-1 is unreported. " +
		"Timers: nothing re-runs the decision."
	ledger := disclosureLedger()
	ledger.Stems[0] = entry
	page, mirror := renderRFCDetail(t, ledger, "rfc9999")
	text := visibleText(mainContent(t, page))

	// Three claims, and the semicolon inside `{ med; }` did not make a fourth.
	if got := strings.Count(page, "<li>"); got < 3 {
		t.Errorf("the coverage cell renders %d list items, want its three claims", got)
	}
	if !strings.Contains(text, "the second claim") || !strings.Contains(text, "the third") {
		t.Error("a claim was lost in the split")
	}
	if !strings.Contains(text, "del { med; }") {
		t.Error("the split cut inside a code span")
	}
	// The themes carry the author's own labels and nothing invented.
	for _, label := range []string{"Header handling:", "Timers:"} {
		if !strings.Contains(text, label) {
			t.Errorf("the remaining cell does not carry the %q theme", label)
		}
	}
	if !strings.Contains(text, "Two gaps remain.") {
		t.Error("the lead sentence was dropped")
	}
	// The ids and the paths are links.
	if !strings.Contains(page, `<a href="#rfc9999-2-1"><code>RFC9999-2-1</code></a>`) {
		t.Error("a requirement id in the prose is not linked to its row")
	}
	if !strings.Contains(page, repositoryBlobURL("internal/a.go")) {
		t.Error("a repository path in the prose is not linked to its file")
	}
	if !strings.Contains(mirror, "- **Header handling:** ") {
		t.Error("the mirror does not carry the themes as items")
	}
	if !strings.Contains(mirror, "- the second claim") {
		t.Error("the mirror does not carry the claims as items")
	}
}

// VALIDATES: AC-53 -- a test is cited by NAME, never by the file it lives in,
// and the name is the link.
//
// The page printed `internal/component/bgp/message/rfc4271_test.go` beside
// `TestRFC4271MarkerAllOnesOnSend`. The path is machinery: the link already
// resolves to the exact line, so the path added width and no information (owner
// review, 2026-09-01). It stays REACHABLE as the link's title.
func TestATestIsCitedByNameAndNotByItsFile(t *testing.T) {
	cited := disclosureLedger()
	requirement := &cited.Stems[0].Requirements[1]
	requirement.Covers[0].File, requirement.Covers[0].Line = "internal/a_test.go", 42
	page, text, mirror := renderRFCDetailPage(t, cited)

	want := repositoryLineURL("internal/a_test.go", 42)
	if !strings.Contains(page, `<a href="`+want+
		`" title="internal/a_test.go" target="_blank" rel="noopener"><code>TestWidget</code></a>`) {
		t.Error("the citation does not name the test, link the line and keep the path as title")
	}
	if !strings.Contains(mirror, "[`TestWidget`]("+want+")") {
		t.Error("the mirror does not link the test name to its line")
	}
	// The path is nowhere in the reader's line of sight, in either table.
	for _, table := range []string{"<h2>Requirements</h2>", "<h2>Proof state</h2>"} {
		body := sliceBetween(t, mainContent(t, page), table, "</section>")
		if strings.Contains(visibleText(body), "internal/a_test.go") {
			t.Errorf("%s still prints the file beside the test", table)
		}
	}
	if strings.Contains(page, "<th>Test file</th>") {
		t.Error("the proof-state table still carries its Test file column")
	}
	if strings.Contains(text, "the whole file is the unit") {
		t.Error("a scenario still reads as a sentence rather than as its own name")
	}
}

// VALIDATES: AC-53 -- a `.ci` scenario is named by its own file name, because
// it has no function to name.
func TestAScenarioIsCitedByItsFileName(t *testing.T) {
	scenario := disclosureLedger()
	requirement := &scenario.Stems[0].Requirements[1]
	requirement.Covers[0] = rfcLedgerCover{Polarity: rfc.PolarityPositive,
		Unit: "test/plugin/adj-rib-in-replay-rfc2545-next-hop.ci", Carrier: "functional/verify",
		File: "test/plugin/adj-rib-in-replay-rfc2545-next-hop.ci", Line: 17, Tags: 1}
	page, text, mirror := renderRFCDetailPage(t, scenario)

	if !strings.Contains(text, "adj-rib-in-replay-rfc2545-next-hop.ci") {
		t.Error("the scenario is not named by its own file name")
	}
	if strings.Contains(text, "test/plugin/adj-rib-in-replay") {
		t.Error("the scenario still carries its whole path in the reader's line of sight")
	}
	if !strings.Contains(page, `title="test/plugin/adj-rib-in-replay-rfc2545-next-hop.ci"`) {
		t.Error("the whole path is not reachable as the link's title")
	}
	if !strings.Contains(mirror, "[`adj-rib-in-replay-rfc2545-next-hop.ci`]("+
		repositoryLineURL("test/plugin/adj-rib-in-replay-rfc2545-next-hop.ci", 17)+")") {
		t.Error("the mirror does not link the scenario by name")
	}
}

// VALIDATES: AC-54 -- two tests that share a name on one page do not render
// identically.
//
// Three such collisions exist in this corpus and all three put both units on
// ONE page, so dropping the path unqualified would show a reader two different
// tests as one row. The package directory comes back, and only there.
func TestTwoTestsSharingANameAreToldApart(t *testing.T) {
	collide := disclosureLedger()
	entry := &collide.Stems[0]
	entry.Requirements[1].Covers = []rfcLedgerCover{
		{Polarity: rfc.PolarityPositive, Unit: "internal/component/radius/rfc_test.go::TestSame",
			File: "internal/component/radius/rfc_test.go", Line: 10,
			Carrier: "unit/verify", Tags: 1},
		{Polarity: rfc.PolarityNegative,
			Unit: "internal/component/l2tp/plugins/authradius/rfc_test.go::TestSame",
			File: "internal/component/l2tp/plugins/authradius/rfc_test.go", Line: 20,
			Carrier: "unit/verify", Tags: 1},
	}
	if names := rfcAmbiguousNames(entry); !names["TestSame"] {
		t.Fatal("two units sharing a name are not detected as ambiguous")
	}
	_, text, _ := renderRFCDetailPage(t, collide)
	for _, want := range []string{"radius/TestSame", "authradius/TestSame"} {
		if !strings.Contains(text, want) {
			t.Errorf("the page does not tell the two apart: %q is absent", want)
		}
	}

	// A name only one unit carries is NOT qualified: the directory is there to
	// resolve a collision, not to decorate every citation.
	if names := rfcAmbiguousNames(&disclosureLedger().Stems[0]); len(names) != 0 {
		t.Errorf("names %v are called ambiguous on a page that carries no collision", names)
	}
	plain := disclosureLedger()
	plain.Stems[0].Requirements[1].Covers[0].File = "internal/a_test.go"
	_, plainText, _ := renderRFCDetailPage(t, plain)
	if strings.Contains(plainText, "reactor/TestWidget") {
		t.Error("an unambiguous citation was qualified anyway")
	}
}

// VALIDATES: AC-53 -- every citation the shard prints is a tagged unit, so
// rendering the citations from the units renders the same population.
//
// This is what lets the page cite from structured fields rather than re-reading
// the shard's own markdown. Measured over the real corpus: 10,768 cells.
func TestEveryShardCitationIsATaggedUnit(t *testing.T) {
	ledger := publishedLedgerOfThisCheckout(t)
	checked := 0
	for index := range ledger.Stems {
		entry := &ledger.Stems[index]
		for position := range entry.Requirements {
			requirement := &entry.Requirements[position]
			for _, one := range []struct {
				polarity string
				cell     string
			}{
				{rfc.PolarityPositive, requirement.Positive},
				{rfc.PolarityNegative, requirement.Negative},
			} {
				checked++
				cell := strings.TrimSpace(one.cell)
				empty := cell == "" || cell == "--" || cell == "-"
				held := 0
				for _, cover := range requirement.Covers {
					if cover.Polarity != one.polarity {
						continue
					}
					held++
					if !strings.Contains(cell, rfcCitationName(cover.Unit)) {
						t.Errorf("%s cites %s in its units and not in its %s cell",
							requirement.RID, cover.Unit, one.polarity)
					}
				}
				if empty != (held == 0) {
					t.Errorf("%s has %d %s units and a cell reading %q",
						requirement.RID, held, one.polarity, cell)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("this checkout compares no cell against its units, so this proves nothing")
	}
	t.Logf("compared %d shard cells against the tagged units behind them", checked)
}

// VALIDATES: AC-55 and AC-60 -- the tests are a GRID, one row per citation,
// with the kind and tier BEFORE the name, and a row stating an absent polarity.
//
// Two columns each holding a comma-joined run put a requirement's tests in two
// narrow stacks and wrapped `(unit/verify)` away from the name it qualified
// (owner review, 2026-09-01). One row per test gives each name room, and a
// short fixed-width tier in front of it lines the tiers up down the page.
func TestBothPolaritiesShareOneColumn(t *testing.T) {
	page, text, mirror := disclosurePage(t)
	head := sliceBetween(t, mainContent(t, page), "<h2>Requirements</h2>", "</thead>")

	if !strings.Contains(head, "<th>Tests</th>") {
		t.Error("the requirements table carries no single Tests column")
	}
	for _, gone := range []string{"<th>Positive test</th>", "<th>Negative test</th>"} {
		if strings.Contains(head, gone) {
			t.Errorf("the requirements table still carries %s", gone)
		}
	}
	// A grid of divs rather than a nested table: a table inside a cell inherits
	// the outer table's width pressure.
	if !strings.Contains(page, `<div class="rfc-tests" role="table"`) {
		t.Error("the tests are not a grid readable as data")
	}
	if strings.Contains(sliceBetween(t, mainContent(t, page), `<div class="rfc-tests"`,
		"</div>"), "<table") {
		t.Error("the tests block nests a table inside a cell")
	}
	// The tier leads the name it qualifies, in its own cell.
	if !strings.Contains(page, `<span role="cell">positive</span>`+
		`<span role="cell"><code>unit/verify</code></span><span role="cell">`) {
		t.Error("the kind and tier do not lead the test name in their own cells")
	}
	// An absent polarity keeps its row and says so.
	if !strings.Contains(text, "no negative test") {
		t.Error("an absent polarity is not stated, so it reads as an oversight")
	}
	if !strings.Contains(page, `<span role="cell"><code>no test</code></span>`) {
		t.Error("a row stating an absent polarity carries no tier cell")
	}
	if strings.Count(text, "no positive test") == 0 {
		t.Error("an absent positive polarity is not stated")
	}
	// The mirror cannot nest, so it keeps labeled lines with the tier leading.
	if !strings.Contains(mirror, "**negative:** no negative test") {
		t.Error("the mirror does not state an absent polarity")
	}
	if !strings.Contains(mirror, "**positive:** `unit/verify` `") {
		t.Error("the mirror does not lead its citations with the tier")
	}
}

// VALIDATES: AC-56 -- the subject is a row of its own spanning every column,
// and the span is derived from the header rather than written beside it.
func TestTheRequirementTextSpansTheWholeTable(t *testing.T) {
	page, text, mirror := disclosurePage(t)
	span := `<tr class="rfc-span"><td class="rfc-subject-id">` +
		`<code id="rfc9999-2-1">RFC9999-2-1</code></td><td colspan="` +
		strconv.Itoa(len(rfcRequirementColumns)-1) + `">`

	if !strings.Contains(page, span) {
		t.Errorf("the subject is not an id cell beside a spanning cell: want %q", span)
	}
	head := sliceBetween(t, mainContent(t, page), "<h2>Requirements</h2>", "</thead>")
	if got := strings.Count(head, "<th>"); got != len(rfcRequirementColumns) {
		t.Errorf("the header renders %d columns and the span covers %d",
			got, len(rfcRequirementColumns))
	}
	const sentence = "A widget MUST be rejected when its length is zero."
	if !strings.Contains(text, sentence) || !strings.Contains(mirror, sentence) {
		t.Error("the requirement text was lost when it moved to its own row")
	}
	if !strings.Contains(page, "&lt;once&gt; &amp; only once.") {
		t.Error("the spanning row does not escape quoted RFC prose")
	}
	// A requirement quoting nothing still gets the row: it carries the id and
	// the anchor the whole page links to.
	quiet := disclosureLedger()
	for index := range quiet.Stems[0].Requirements {
		quiet.Stems[0].Requirements[index].Text = ""
	}
	quietPage, _, _ := renderRFCDetailPage(t, quiet)
	if !strings.Contains(quietPage, span+"</td>") {
		t.Error("a requirement quoting nothing lost its anchored id row")
	}
	if strings.Contains(quietPage, `<span class="rfc-subject">`) {
		t.Error("a requirement quoting nothing renders an empty subject span")
	}
}

// VALIDATES: AC-58 -- every fact the retired Note column carried still has a
// home, over the REAL corpus.
//
// The Note held four marks: the annotation, the superseded pointer, the audit
// stamp and the nightly-only mark (requirementRow, internal/le/rfc/render.go).
// Two were duplicates of sections this page already renders and two were not,
// so this checks each mark against the place it now lives rather than trusting
// the reading. A fact that vanishes in a layout change is what AC-17 and AC-18
// exist to prevent.
func TestNothingTheNoteCarriedWasLost(t *testing.T) {
	ledger := publishedLedgerOfThisCheckout(t)
	marks := map[string]int{}
	for index := range ledger.Stems {
		entry := &ledger.Stems[index]
		for position := range entry.Requirements {
			requirement := &entry.Requirements[position]
			note := strings.TrimSpace(requirement.Note)
			if note == "" {
				continue
			}
			// The annotation and the nightly-only mark are rendered beside the
			// tests they explain.
			if requirement.Annotation != nil {
				marks["annotation"]++
				rendered := rfcRequirementTestsHTML(requirement, nil)
				if !strings.Contains(rendered, requirement.Annotation.Kind) ||
					!strings.Contains(rendered, html.EscapeString(requirement.Annotation.Reason)) {
					t.Errorf("%s carries a {%s} reason the tests cell does not state",
						requirement.RID, requirement.Annotation.Kind)
				}
			}
			if requirement.NightlyOnly {
				marks["nightly-only"]++
				if !strings.Contains(rfcRequirementTestsHTML(requirement, nil), "nightly-only") {
					t.Errorf("%s is nightly-only and the tests cell does not say so",
						requirement.RID)
				}
			}
			// The audit stamp and the superseded pointer have their own
			// sections, which is why they are not re-rendered here.
			if strings.Contains(note, "**audit:") {
				marks["audit"]++
				if requirement.Audit == nil {
					t.Errorf("%s carries an audit mark and no verdict for Proof state to render",
						requirement.RID)
				}
			}
			if strings.Contains(note, "{"+rfc.SupersededKind+":") {
				marks["superseded"]++
				if requirement.Superseded == nil {
					t.Errorf("%s carries a superseded mark and no pointer for the Superseded "+
						"section to render", requirement.RID)
				}
			}
		}
	}
	for _, mark := range []string{"annotation", "superseded", "audit", "nightly-only"} {
		if marks[mark] == 0 {
			t.Errorf("this checkout carries no %s mark, so its home is unproven", mark)
		}
	}
	t.Logf("marks rehomed: %v", marks)
}

// VALIDATES: AC-59 -- an annotation reason is stated under the tests it
// explains, where the reader is already asking why.
func TestAnAnnotationReasonSitsUnderTheTests(t *testing.T) {
	excused := disclosureLedger()
	requirement := &excused.Stems[0].Requirements[5]
	requirement.Covers = []rfcLedgerCover{{Polarity: rfc.PolarityPositive,
		Unit: "internal/d_test.go::TestAck", File: "internal/d_test.go", Line: 3,
		Carrier: "unit/verify", Tags: 1}}
	requirement.Annotation = &rfcLedgerAnnotation{Kind: rfc.AnnotationSinglePolarity,
		Polarity: rfc.PolarityPositive, Reason: "the widget admits no counter-case"}
	page, text, mirror := renderRFCDetailPage(t, excused)

	if !strings.Contains(text, "no negative test") {
		t.Error("the absent polarity is not stated")
	}
	if !strings.Contains(text, "{"+rfc.AnnotationSinglePolarity+"}:") ||
		!strings.Contains(text, "the widget admits no counter-case") {
		t.Error("the annotation reason is not stated beside the tests it explains")
	}
	if !strings.Contains(page, `<p class="rfc-mark">`) {
		t.Error("the mark carries no class of its own")
	}
	if !strings.Contains(mirror, "**{"+rfc.AnnotationSinglePolarity+"}:** "+
		"the widget admits no counter-case") {
		t.Error("the mirror does not state the annotation beside the tests")
	}
	// The reason follows the polarity lines rather than preceding them. The
	// slice starts at THIS requirement's own row, because an earlier row
	// carrying no mark would answer the question about the wrong cell.
	cell := sliceBetween(t, mainContent(t, page), `aria-label="tests bound to RFC9999-6-1"`,
		"</td>")
	mark, negative := strings.Index(cell, "rfc-mark"), strings.Index(cell, ">negative<")
	if mark < 0 || negative < 0 {
		t.Fatalf("the tests cell carries no mark or no negative row: %q", cell)
	}
	if mark < negative {
		t.Error("the annotation is rendered above the rows it explains")
	}
}

// VALIDATES: AC-61 -- the tests read in carrier order: kind, then tier within a
// kind, then polarity within a group.
//
// The order was whatever the tag scan produced, so a reader could not tell at a
// glance whether an obligation was carried by unit tests or by a nightly
// interop run (owner review, 2026-09-01). The sequence lives in
// internal/le/rfc beside the vocabulary it orders, and this reads it from
// there rather than restating it.
func TestTheTestsReadInCarrierOrder(t *testing.T) {
	mixed := disclosureLedger()
	requirement := &mixed.Stems[0].Requirements[1]
	requirement.Covers = []rfcLedgerCover{
		{Polarity: rfc.PolarityNegative, Unit: "internal/le/interoplab/bgp/c.go::checkLate",
			File: "internal/le/interoplab/bgp/c.go", Line: 9, Carrier: "interop/nightly", Tags: 1},
		{Polarity: rfc.PolarityNegative, Unit: "internal/a_test.go::TestUnitNegative",
			File: "internal/a_test.go", Line: 2, Carrier: "unit/verify", Tags: 1},
		{Polarity: rfc.PolarityPositive, Unit: "test/plugin/late.ci",
			File: "test/plugin/late.ci", Line: 1, Carrier: "functional/verify", Tags: 1},
		{Polarity: rfc.PolarityPositive, Unit: "internal/a_test.go::TestUnitPositive",
			File: "internal/a_test.go", Line: 1, Carrier: "unit/verify", Tags: 1},
	}
	rows := rfcTestRows(requirement)
	got := make([]string, 0, len(rows))
	for index := range rows {
		got = append(got, rows[index].Carrier+" "+rows[index].Polarity)
	}
	want := []string{
		"unit/verify positive", "unit/verify negative",
		"functional/verify positive", "interop/nightly negative",
	}
	if strings.Join(got, ", ") != strings.Join(want, ", ") {
		t.Errorf("the tests read as %v, want %v", got, want)
	}

	// That the declared order is itself total and ascending is held in the
	// package that declares it, by TestTheReadingOrderIsTotalAndAscending.
	// What this holds is that the page APPLIES it.
	if rank, ranked := rfc.CarrierLabelRank("unit/verify"); !ranked || rank != 0 {
		t.Errorf("unit/verify ranks (%d, %v), want first", rank, ranked)
	}
}

// VALIDATES: AC-61 -- the sort is total and stable, and an unranked carrier is
// placed deliberately rather than landing where unit/verify lives.
func TestTheTestOrderIsTotalAndStable(t *testing.T) {
	odd := disclosureLedger()
	requirement := &odd.Stems[0].Requirements[1]
	requirement.Covers = []rfcLedgerCover{
		{Polarity: rfc.PolarityPositive, Unit: "internal/z_test.go::TestZ",
			File: "internal/z_test.go", Line: 2, Carrier: "unit/verify", Tags: 1},
		{Polarity: rfc.PolarityPositive, Unit: "internal/a_test.go::TestA",
			File: "internal/a_test.go", Line: 1, Carrier: "unit/verify", Tags: 1},
		{Polarity: rfc.PolarityPositive, Unit: "somewhere/x_test.go::TestOdd",
			File: "somewhere/x_test.go", Line: 3, Carrier: "moon/eclipse", Tags: 1},
	}
	rows := rfcTestRows(requirement)
	if len(rows) != 4 {
		t.Fatalf("the requirement renders %d rows, want its three tests and one absent "+
			"polarity", len(rows))
	}
	// Alike in kind, tier and polarity: ordered by the name a reader sees, so a
	// rebuild cannot churn the page.
	if rows[0].Sort != "TestA" || rows[1].Sort != "TestZ" {
		t.Errorf("two alike citations read as %q then %q, want name order",
			rows[0].Sort, rows[1].Sort)
	}
	// The unranked carrier is LAST, not first.
	if rows[len(rows)-1].Carrier != "moon/eclipse" {
		t.Errorf("the unranked carrier is at position %d of %d",
			len(rows)-1, len(rows))
	}
	if rows[0].Carrier == "moon/eclipse" {
		t.Error("an unranked carrier landed where unit/verify lives")
	}
	// An absent polarity stays in the group the eye is already reading.
	absent := -1
	for index := range rows {
		if rows[index].Cover == nil {
			absent = index
		}
	}
	if absent < 0 {
		t.Fatal("the absent polarity has no row")
	}
	if rows[absent].Carrier != "no test" || rows[absent].Rank != rows[0].Rank {
		t.Errorf("the absent polarity sorts at rank %d, want the best carrier's %d",
			rows[absent].Rank, rows[0].Rank)
	}
}

// VALIDATES: AC-62 -- the subject carries no weight of its own.
//
// `.md-content td:first-child` is bold and sticky for every table on the site,
// and the subject row's spanning cell IS a first child, so the id and the
// sentence rendered bold. Position is what marks the subject; weight on top of
// it is emphasis doing a job the layout already does, and 228 bold rows read as
// shouting (owner review, 2026-09-01).
func TestTheSubjectCarriesNoWeightOfItsOwn(t *testing.T) {
	page, _, mirror := disclosurePage(t)

	// ONE convention for every table this family renders, rather than an
	// opt-out for one of them: the page carried eleven tables and ten shared a
	// look the eleventh did not (owner review, 2026-09-01).
	body := mainContent(t, page)
	if plain := strings.Count(body, "<table>"); plain != 0 {
		t.Errorf("%d tables carry no family class, so the page has two conventions", plain)
	}
	if !strings.Contains(page, ".rfc-table td:first-child { font-weight: 400; }") {
		t.Error("the family does not turn off the site-wide first-column weight")
	}
	// Nothing in the subject row emits weight of its own.
	subject := sliceBetween(t, mainContent(t, page), `<tr class="rfc-span">`, "</tr>")
	for _, weight := range []string{"<strong>", "<b>", "<em>"} {
		if strings.Contains(subject, weight) {
			t.Errorf("the subject row emits %s", weight)
		}
	}
	// The mirror does not bold it either.
	for line := range strings.SplitSeq(mirror, "\n") {
		if !strings.HasPrefix(line, "| `RFC9999-") {
			continue
		}
		if strings.HasPrefix(line, "| **") || strings.Contains(line, "| **`RFC9999-") {
			t.Errorf("the mirror bolds the requirement: %s", line)
		}
	}
}

// VALIDATES: AC-63 -- every table this family renders shares one look, on the
// stem page and on the index.
//
// The requirements table opted out of the site-wide sticky bold first column in
// an earlier pass, which left ten tables on the page looking one way and the
// eleventh another, with no reason a reader could see (owner review,
// 2026-09-01). The convention is now the family's rather than one table's:
// every table carries the class, so consistency holds by construction rather
// than by remembering to add it.
//
// The weight goes rather than the stickiness. Of the eleven first columns on a
// stem page, eight are LABELS -- Card, Field, Bucket, Polarity, Field -- and
// three are identities. The site rule bolds all of them alike, and weight on a
// label emphasizes the question rather than the answer. Stickiness stays: these
// tables scroll inside their own container, and it is what keeps a row's
// identity visible while they do.
func TestEveryTableOfThisFamilySharesOneLook(t *testing.T) {
	stem, _, _ := disclosurePage(t)
	index, _ := rfcCompliancePage(t, rfcCompliancePaths(t))

	for name, page := range map[string]string{"stem": stem, "index": index} {
		body := mainContent(t, page)
		classed := strings.Count(body, `<table class="rfc-table">`)
		plain := strings.Count(body, "<table>")
		if classed == 0 {
			t.Errorf("the %s page publishes no table of this family", name)
		}
		if plain != 0 {
			t.Errorf("the %s page carries %d tables outside the family convention",
				name, plain)
		}
		// Every one of them scrolls inside its own container too.
		wrapped := strings.Count(body,
			`<div class="rfc-table-wrap">`+"\n"+`<table class="rfc-table">`)
		if wrapped != classed {
			t.Errorf("the %s page wraps %d of %d tables", name, wrapped, classed)
		}
	}
	// The rule is stated once, for the family.
	if strings.Count(stem, ".rfc-table td:first-child") != 1 {
		t.Error("the family's first-column rule is not stated exactly once")
	}
	if strings.Contains(stem, "rfc-requirements") {
		t.Error("one table still carries a class of its own")
	}
}

// rfcRefMarkers answers the strings this family writes where it DECLARES that
// what follows is about one requirement.
//
// The id alone is not one of them. A requirement's own text can quote a sister
// id -- RFC9069-x-6 names RFC9069-x-5 in its correction note -- so slicing on
// the id closed that block before its proof table and lost the disclosure the
// page really carries. Every renderer opens a requirement's row or block
// through rfcMirrorRow, rfcRequirementRefMirror, rfcSubjectRow or
// rfcRequirementRefHTML, and each of those four writes one of these.
func rfcRefMarkers(rid string) []string {
	anchor := rfcAnchor(rid)
	return []string{
		"| `" + rid + "` ",
		"[`" + rid + "`](#" + anchor + ")",
		`id="` + anchor + `"`,
		`href="#` + anchor + `"`,
	}
}

// rfcMarkerStarts answers every offset where a rendering declares a subject,
// for any requirement of `all`, in reading order.
func rfcMarkerStarts(rendering string, all []string) []int {
	var starts []int
	for _, rid := range all {
		for _, marker := range rfcRefMarkers(rid) {
			for at := 0; ; {
				found := strings.Index(rendering[at:], marker)
				if found < 0 {
					break
				}
				starts = append(starts, found+at)
				at = found + at + 1
			}
		}
	}
	sort.Ints(starts)
	return starts
}

// rfcDisclosureUnit answers the parts of one rendering that belong to ONE
// requirement: every stretch that opens where that requirement is declared the
// subject and closes where the next requirement is.
//
// A page-wide Contains cannot tell "the word is disclosed for THIS requirement"
// from "the word appears somewhere else on the page", so a whole section could
// be deleted and a test written that way would stay green (independent review,
// 2026-09-01).
func rfcDisclosureUnit(rendering, rid string, all []string) string {
	starts := rfcMarkerStarts(rendering, all)
	var out strings.Builder
	for _, marker := range rfcRefMarkers(rid) {
		for at := 0; ; {
			found := strings.Index(rendering[at:], marker)
			if found < 0 {
				break
			}
			found += at
			at = found + 1
			end := len(rendering)
			for _, start := range starts {
				if start > found {
					end = start
					break
				}
			}
			out.WriteString(rendering[found:end])
		}
	}
	return out.String()
}

// rfcAllRIDs answers every requirement id one summary carries.
func rfcAllRIDs(entry *rfcLedgerStem) []string {
	all := make([]string, 0, len(entry.Requirements))
	for index := range entry.Requirements {
		all = append(all, entry.Requirements[index].RID)
	}
	return all
}

// VALIDATES: AC-64 -- every stem page proves its own arithmetic, over this
// checkout's whole corpus.
//
// The index cross-checks its buckets against the gate's own gated total. The
// stem pages did not, and their bucket counts come from rfc.CoverageRows while
// their membership lists come from a second walk over the same requirements
// (independent review, 2026-09-01). Two walks can disagree, and a fourth
// annotation kind can leave a requirement in no bucket at all, both with every
// other test green. The method is the rendered page rather than the helper, so
// dropping the total row goes red here.
func TestEveryStemPageAccountsForItsGatedRequirements(t *testing.T) {
	root := repositoryRoot(t)
	ledger, err := collectRequirementLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for index := range ledger.Stems {
		entry := &ledger.Stems[index]
		if entry.Coverage.Gated == 0 {
			continue
		}
		checked++
		buckets := rfcCoverageBuckets(entry)
		if total := rfcCoverageTotal(buckets); total != entry.Coverage.Gated {
			t.Errorf("%s holds %d gated MUSTs and its buckets account for %d",
				entry.Stem, entry.Coverage.Gated, total)
		}
		for _, bucket := range buckets {
			if len(bucket.IDs) != bucket.Count {
				t.Errorf("%s: the gate counts %d for %q and the page names %d",
					entry.Stem, bucket.Count, bucket.Label, len(bucket.IDs))
			}
		}
		// The CARDS partition the binding population by splitting the gate's
		// Annotated total into its three annotation kinds. Annotated comes from
		// rfc.CoverageRows and the three come from this page's own walk over
		// annotation kinds, so this compares two producers rather than a number
		// with itself, and it is what the card sum rests on.
		split := entry.Coverage.NotApplicable + entry.Coverage.GatedGaps +
			entry.Coverage.SinglePolarity
		if split != entry.Coverage.Annotated {
			t.Errorf("%s: the gate counts %d annotated and the kinds account for %d "+
				"(%d out of scope, %d gaps, %d single-polarity), so the cards cannot "+
				"partition the binding population",
				entry.Stem, entry.Coverage.Annotated, split, entry.Coverage.NotApplicable,
				entry.Coverage.GatedGaps, entry.Coverage.SinglePolarity)
		}
		if entry.Coverage.UnmappedAnnotations != 0 {
			t.Errorf("%s carries %d requirement(s) whose annotation kind has no bucket",
				entry.Stem, entry.Coverage.UnmappedAnnotations)
		}
		cards := rfcDetailCards(entry)
		sum, parts := 0, 0
		for _, card := range cards {
			if card.Partition {
				parts++
				sum += card.Part
			}
		}
		if parts == 0 {
			t.Errorf("%s publishes no card marked as a part of its population", entry.Stem)
		}
		if sum != entry.Coverage.Binding() {
			t.Errorf("%s: its %d share cards add to %d and its binding population is %d",
				entry.Stem, parts, sum, entry.Coverage.Binding())
		}
		for name, rendering := range map[string]string{
			"page": rfcDetailBody(entry), "mirror": rfcDetailMirror(entry),
		} {
			if !strings.Contains(rendering, rfcGatedBucketLabel) {
				t.Errorf("the %s for %s states no accounting total", name, entry.Stem)
			}
			if !strings.Contains(rendering, "falls in exactly one bucket above") {
				t.Errorf("the %s for %s does not say its buckets partition its population",
					name, entry.Stem)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no summary carries a gated MUST, so this proves nothing")
	}
	t.Logf("%d stem pages account for their own gated population", checked)
}

// VALIDATES: AC-65 -- the index's accounting row compares two counts that were
// produced independently, so its mismatch branch can execute.
//
// It compared the sum of the binding buckets against rfcBinding.Obligations,
// which is that same sum, so the sentence saying the bucketing is incomplete
// was unreachable (independent review, 2026-09-01). The method is a split whose
// gated total does not agree with its buckets.
func TestTheIndexAccountingRowCanSayTheBucketingIsIncomplete(t *testing.T) {
	split := rfcBinding{Gated: 100, OutOfScope: 10, Obligations: 85}
	if got := split.Binding(); got != 90 {
		t.Fatalf("the binding population reads %d, want 90", got)
	}
	if note := rfcAccountedNote(split, 85); !strings.Contains(note, "incomplete") {
		t.Errorf("85 of 90 accounted reads %q, which does not say the bucketing is incomplete",
			note)
	}
	if note := rfcAccountedNote(split, 90); strings.Contains(note, "incomplete") {
		t.Errorf("90 of 90 accounted reads %q, which calls a complete bucketing incomplete", note)
	}
	// The one cause the page can name. A shortfall with no unmapped annotation
	// behind it says only that the buckets are short, because this page does
	// not know what else could have caused one.
	unmapped := rfcBinding{Gated: 100, OutOfScope: 10, Obligations: 85, Unmapped: 5}
	note := rfcAccountedNote(unmapped, 85)
	if !strings.Contains(note, "no bucket for") {
		t.Errorf("5 requirements are in no bucket and the note does not say why: %q", note)
	}
	if strings.Contains(rfcAccountedNote(split, 85), "no bucket for") {
		t.Error("a shortfall with no unmapped annotation behind it claims one")
	}
}

// VALIDATES: AC-66 -- the prose split and the prose renderer lose no character
// of a cell whose SHAPE this corpus does not happen to carry yet.
//
// Two silent losses were found by reading rather than by a red bar, because no
// cell in the tree has either shape today: a cell ending in a semicolon lost
// that semicolon on rejoin, and a cell with an odd number of backticks lost one
// backtick and marked its tail up as code (independent review, 2026-09-01). An
// author can write either tomorrow, so the shapes are held here rather than
// waited for.
func TestProseOfAnyShapeIsPublishedWholeAndLosesNoCharacter(t *testing.T) {
	for _, one := range []struct{ name, prose string }{
		{"a trailing semicolon", "first claim; second claim;"},
		{"only a semicolon", ";"},
		{"two trailing semicolons", "first claim;;"},
		{"an odd backtick", "the `medium flag is set here"},
		{"an odd backtick after a claim", "one claim; the `medium flag"},
		{"a semicolon inside a code span", "a `{ med; }` directive; and one more"},
		{"a semicolon inside brackets", "a (first; second) pair; and one more"},
		{"no semicolon at all", "one claim only"},
	} {
		items := rfcProseSplit(one.prose)
		if got := rfcProseJoin(items); got != one.prose {
			t.Errorf("%s: the split loses text\n got %q\nwant %q", one.name, got, one.prose)
		}
		// Balance is owed only where the AUTHOR balanced the cell. Prose with
		// an odd backtick opens a span nothing closes, and no split of it can
		// leave a balanced item.
		if rfcProseBalanced(one.prose) {
			for _, item := range items {
				if !rfcProseBalanced(item) {
					t.Errorf("%s: the split left an unbalanced item %q", one.name, item)
				}
			}
		}
		if got := rfcProseThemesJoin(rfcProseThemes(one.prose)); got != one.prose {
			t.Errorf("%s: the theme split loses text\n got %q\nwant %q",
				one.name, got, one.prose)
		}
		// The prose here names no requirement and no path, so the mirror of it
		// is the prose itself. A renderer that ate a backtick reads short here.
		if got := rfcProseMirror(one.prose, nil); got != one.prose {
			t.Errorf("%s: the mirror rewrites the prose\n got %q\nwant %q",
				one.name, got, one.prose)
		}
		// A closed code span becomes markup, so its backticks are meant to
		// leave the page. An UNCLOSED one is text, and every character of it
		// is owed to the reader.
		if strings.Count(one.prose, "`")%2 == 1 {
			if got := strings.Count(rfcProseHTML(one.prose, nil), "`"); got !=
				strings.Count(one.prose, "`") {
				t.Errorf("%s: the page carries %d backticks of the %d the author wrote",
					one.name, got, strings.Count(one.prose, "`"))
			}
		}
	}
}

// VALIDATES: AC-67 -- retiring a page deletes a page of THIS family and
// nothing else under the same prefix.
//
// The removal was keyed on "a directory whose name is not a live stem", and the
// output of a real build is the published checkout, so any directory another
// producer or an author put under /quality/rfc-compliance/ was deleted
// (independent review, 2026-09-01). The method is a retired page of this
// family beside a directory that is not one.
func TestRetiringAPageDeletesOnlyThisFamilysOwnPages(t *testing.T) {
	output := t.TempDir()
	root := filepath.Join(output, filepath.FromSlash(rfcComplianceDirectory))
	entry := disclosureLedger().Stems[0]
	for _, one := range []struct{ directory, page string }{
		{"rfc9999", rfcDetailBody(&entry)},
		{"rfc8888", rfcDetailBody(&entry)},
		{"handbook", "<html><body>an authored page under the same prefix</body></html>"},
	} {
		if err := os.MkdirAll(filepath.Join(root, one.directory), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, one.directory, pageIndexFile),
			[]byte(one.page), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := removeRetiredRFCPages(output, map[string]bool{"rfc9999": true}); err != nil {
		t.Fatal(err)
	}
	for _, one := range []struct {
		directory string
		want      bool
		why       string
	}{
		{"rfc9999", true, "this run wrote it"},
		{"rfc8888", false, "this family wrote it and this run did not"},
		{"handbook", true, "this family never wrote it"},
		{"assets", true, "it carries no page at all"},
	} {
		_, err := os.Stat(filepath.Join(root, one.directory))
		if held := err == nil; held != one.want {
			t.Errorf("%s is present=%v, want %v: %s", one.directory, held, one.want, one.why)
		}
	}
}

// VALIDATES: AC-69 -- a page for a summary the gate does not hold never claims
// the gate holds its requirements.
//
// evaluate skips every requirement of an un-enrolled RFC
// (internal/le/rfc/check_core.go), and the population card said "the gate
// HOLDS" on every page (independent review, 2026-09-01). The method is this
// checkout's own un-enrolled summaries, of which there are eighteen.
func TestAnUnenrolledPageNeverClaimsTheGateHoldsIt(t *testing.T) {
	root := repositoryRoot(t)
	ledger, err := collectRequirementLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	declined, enrolled := 0, 0
	for index := range ledger.Stems {
		entry := &ledger.Stems[index]
		if entry.Coverage.Gated == 0 {
			continue
		}
		body := rfcDetailBody(entry)
		if entry.Enrolled {
			enrolled++
			if !strings.Contains(body, "the gate HOLDS") {
				t.Errorf("the %s page does not say the gate holds its requirements", entry.Stem)
			}
			continue
		}
		declined++
		if strings.Contains(body, "the gate HOLDS") {
			t.Errorf("%s is not enrolled and its page says the gate holds its requirements",
				entry.Stem)
		}
		if !strings.Contains(body, "The gate holds none of them") {
			t.Errorf("%s is not enrolled and its page does not say the gate holds none of it",
				entry.Stem)
		}
	}
	if declined == 0 || enrolled == 0 {
		t.Fatalf("%d declined and %d enrolled summaries carry a MUST, so this proves nothing",
			declined, enrolled)
	}
	t.Logf("%d enrolled and %d declined summaries carry MUST-level requirements",
		enrolled, declined)
}

// VALIDATES: the partition sentence reads the shares it claims add up.
//
// It said "they add to 100%" after counting how many cards were MARKED as a
// part, never reading one of their values, so a partition that had lost a
// share published the same sentence (independent review, 2026-09-02). The two
// cases below are the whole of what the sentence can say, and neither is
// reachable from the other.
func TestThePartitionSentenceReadsTheSharesItAddsUp(t *testing.T) {
	complete := []rfcCard{
		{Label: "Proven", Partition: true, Part: 60},
		{Label: "Excused", Partition: true, Part: 40},
		{Label: "Proven by a recorded break", Part: 999},
	}
	note := rfcPartitionNote(complete, 100)
	if !strings.Contains(note, "they add to 100%") {
		t.Errorf("a partition that adds up is not stated as one: %q", note)
	}
	if strings.Contains(note, "fall in none") {
		t.Errorf("a partition that adds up is reported short: %q", note)
	}

	// One share short, which is what an annotation kind with no bucket does.
	short := []rfcCard{
		{Label: "Proven", Partition: true, Part: 60},
		{Label: "Excused", Partition: true, Part: 33},
	}
	note = rfcPartitionNote(short, 100)
	if !strings.Contains(note, "do NOT add to 100%") {
		t.Errorf("7 obligations fall in no share and the note claims a partition: %q", note)
	}
	for _, want := range []string{"account for 93", "of the 100", "7 fall in none"} {
		if !strings.Contains(note, want) {
			t.Errorf("the shortfall sentence does not carry %q: %q", want, note)
		}
	}
	if note == rfcPartitionNote(complete, 100) {
		t.Error("a partition that adds up and one that does not read the same")
	}
}

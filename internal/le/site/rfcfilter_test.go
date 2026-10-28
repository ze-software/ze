// Design: docs/contributing/gh-pages.md -- ceilings on each public RFC surface.
package site

import (
	"encoding/json"
	"html"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// The index, profile and mirrors disclose the same walked-only ceiling and
// reason counts, including a real zero ceiling and an unmeasured tier.
func TestTheIndexStatesTheComputedCeiling(t *testing.T) {
	rows := []*rfc.TreeRow{
		{Stem: "rfc9998", Summary: true, Walked: true, MandatoryTotal: 4,
			MandatoryAttainable: 2, MandatoryExcluded: map[string]int{rfc.AnnotationLinuxLimit: 1, rfc.AnnotationNotApplicable: 1}},
		{Stem: "rfc9999", Summary: true, Walked: true, MandatoryTotal: 1,
			MandatoryExcluded: map[string]int{rfc.AnnotationLowerLayer: 1}},
		{Stem: "rfc10000", Summary: true, MandatoryTotal: 100, MandatoryExcluded: map[string]int{rfc.AnnotationLinuxLimit: 100}},
	}
	statement := rfcCeilingStatement(rows)
	for _, want := range []string{"2 of 5 (40.0%)", "Linux cannot do this: 1 of 5 (20.0%)", "classification can be revisited", "rfc9999", "0 of 1 (0.0%)", "Optional ceiling: no rows"} {
		if !strings.Contains(statement, want) {
			t.Errorf("ceiling omits %q: %s", want, statement)
		}
	}
	for _, surface := range []string{rfcIndexLegendHTML(rows), rfcIndexLegendMirror(rows)} {
		if !strings.Contains(html.UnescapeString(surface), statement) {
			t.Errorf("surface disagrees with measured ceiling: %s", surface)
		}
	}
	if strings.Contains(statement, "101") || strings.Contains(statement, "100% under") {
		t.Fatalf("unwalked rows or fixed maximum entered ceiling: %s", statement)
	}
	full := []*rfc.TreeRow{{Stem: "rfc1", Summary: true, Walked: true, MandatoryTotal: 1, MandatoryAttainable: 1}}
	if !strings.Contains(rfcCeilingStatement(full), "Every RFC shown with measured rows can reach 100% under current classifications.") {
		t.Fatal("fully attainable measured rows do not publish their computed maximum")
	}
	for _, empty := range [][]*rfc.TreeRow{nil, {{Stem: "rfc1", Summary: true}}, {{Stem: "rfc1", Summary: true, Walked: true}}} {
		if text := rfcCeilingStatement(empty); strings.Contains(text, "100%") || !strings.Contains(text, "no rows") {
			t.Fatalf("empty set published a percentage: %s", text)
		}
	}
}

// Count attributes carry the model's inputs without substituting totals for
// zero attainable counts. Browser smoke exercises their actual JS consumer.
func TestTheOverviewSumsTheCeilingTheRollupSums(t *testing.T) {
	rows := []*rfc.TreeRow{
		{Stem: "rfc1", Summary: true, Walked: true, MandatoryProven: 1, MandatoryTotal: 3, MandatoryAttainable: 2,
			MandatoryExcluded: map[string]int{rfc.AnnotationLinuxLimit: 1}},
		{Stem: "rfc2", Summary: true, Walked: true, MandatoryTotal: 2,
			MandatoryExcluded: map[string]int{rfc.AnnotationNotApplicable: 2}},
		{Stem: "rfc3", Summary: true, MandatoryTotal: 99, MandatoryAttainable: 99},
	}
	ceiling := rfc.CeilingOf(rows)
	if ceiling.Mandatory.Attainable != 2 || ceiling.Mandatory.Total != 5 || ceiling.Mandatory.Excluded[rfc.AnnotationLinuxLimit] != 1 || ceiling.NotWalked != 1 {
		t.Fatalf("fixture lost its walked-only distinction: %+v", ceiling)
	}
	var actual rfc.Rollup
	actual.Mandatory.Excluded = map[string]int{}
	actual.Optional.Excluded = map[string]int{}
	attributes := regexp.MustCompile(`data-([a-z]+)="([^"]*)"`)
	for _, row := range rows {
		values := map[string]string{}
		for _, match := range attributes.FindAllStringSubmatch(rfcFigureRowCells(row.Stem, row, "fixture"), -1) {
			values[match[1]] = html.UnescapeString(match[2])
		}
		actual.RFCs++
		if values["walked"] != "1" {
			actual.NotWalked++
			continue
		}
		for _, tier := range []struct {
			prefix string
			figure *rfc.Figure
		}{{"m", &actual.Mandatory}, {"o", &actual.Optional}} {
			for _, field := range []struct {
				suffix string
				count  *int
			}{{"p", &tier.figure.Proven}, {"t", &tier.figure.Total}, {"a", &tier.figure.Attainable}} {
				count, err := strconv.Atoi(values[tier.prefix+field.suffix])
				if err != nil {
					t.Fatal(err)
				}
				*field.count += count
			}
			var excluded map[string]int
			if err := json.Unmarshal([]byte(values[tier.prefix+"e"]), &excluded); err != nil {
				t.Fatal(err)
			}
			for kind, count := range excluded {
				tier.figure.Excluded[kind] += count
			}
		}
	}
	if actual.RFCs != ceiling.RFCs || actual.NotWalked != ceiling.NotWalked ||
		actual.Mandatory.Proven != ceiling.Mandatory.Proven || actual.Mandatory.Total != ceiling.Mandatory.Total ||
		actual.Mandatory.Attainable != ceiling.Mandatory.Attainable ||
		!reflect.DeepEqual(actual.Mandatory.Excluded, ceiling.Mandatory.Excluded) ||
		actual.Optional.Proven != ceiling.Optional.Proven || actual.Optional.Total != ceiling.Optional.Total ||
		actual.Optional.Attainable != ceiling.Optional.Attainable || len(actual.Optional.Excluded) != 0 {
		t.Fatalf("published browser inputs disagree with model rollup: %+v / %+v", actual, ceiling)
	}
	if !strings.Contains(rfcFigureRowCells("rfc2", rows[1]), `data-ma="0"`) {
		t.Fatal("zero attainable was replaced by an inferred total")
	}
}

// Publication reads authored annotations through Collect and NewRenderInput;
// none except single-polarity can turn the same tagged row into public credit.
func TestNoAnnotationExceptSinglePolarityMovesThePublishedShare(t *testing.T) {
	forms := map[string]string{
		rfc.AnnotationGap:             "{gap: native sending absent}",
		rfc.AnnotationNotApplicable:   "{not-applicable: another role}",
		rfc.AnnotationLinuxLimit:      "{linux-limit: LL-WIDGET; kernel has no primitive}",
		rfc.AnnotationLowerLayer:      "{lower-layer: kernel; internal/sample/widget.go::SendWidget}",
		rfc.AnnotationFeatureDeclined: `{feature-declined: "A widget MUST be zero when sent and ignored on receipt."; internal/sample/widget.go::SendWidget}`,
		rfc.AnnotationSinglePolarity:  "{single-polarity: positive; no counter-case}",
	}
	for _, kind := range rfc.AnnotationKinds() {
		traits, known := rfc.AnnotationTraitsOf(kind)
		if !known {
			t.Fatal(kind)
		}
		if traits.Rollup {
			continue
		} // A derived row carries no obligation of its own.
		t.Run(kind, func(t *testing.T) {
			root, summary, marker := partialPublicationTree(t)
			form, held := forms[kind]
			if kind == rfc.AnnotationPartial {
				form, held = marker, true
			}
			if !held {
				t.Fatalf("new annotation %s needs a public-share fixture", kind)
			}
			// A real pair remains proven beside the annotated row; an optional
			// annotated row exercises the second figure without changing public share.
			summary += "- [ ] [RFC9999-2-2] [MUST] A receiver MUST accept a valid widget. (§2)\n" +
				"- [ ] [RFC9999-2-3] [MAY] A widget MUST be zero when sent and ignored on receipt. (§2) " + marker + "\n"
			writePartialPublicationFile(t, root, "rfc/short/rfc9999.md", strings.ReplaceAll(summary, marker, form))
			writePartialPublicationFile(t, root, "test/plugin/pair.ci",
				"# RFC requirement: RFC9999-2-2 positive -- accepts valid widget\n"+
					"# RFC requirement: RFC9999-2-2 negative -- refuses invalid widget\nexpect=stdout:contains=OK\n")
			paths := Paths{Repository: root, Output: t.TempDir()}
			if err := publishRFCLedger(paths); err != nil {
				t.Fatal(err)
			}
			body, err := os.ReadFile(filepath.Join(paths.Output, rfcLedgerFile))
			if err != nil {
				t.Fatal(err)
			}
			var ledger rfcLedger
			if err := json.Unmarshal(body, &ledger); err != nil {
				t.Fatal(err)
			}
			if len(ledger.Stems) != 1 {
				t.Fatalf("lost published population: %+v", ledger)
			}
			coverage := ledger.Stems[0].Coverage
			if coverage.Gated != 2 || coverage.Both != 1 || coverage.UnmappedAnnotations != 0 {
				t.Fatalf("annotation changed denominator or granted pair credit: %+v", coverage)
			}
			collected, err := rfc.Collect(root)
			if err != nil {
				t.Fatal(err)
			}
			share, err := rfc.ProvenShareOf(collected.Metas, collected.Requirements, collected.Tags, nil)
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if traits.SinglePolarity {
				want = 2
			}
			if share.Proven != want || share.Gated != 2 {
				t.Fatalf("annotation changed public share: %+v", share)
			}
			if len(ledger.Graph.Nodes) != 1 {
				t.Fatalf("published graph lost the measured RFC: %+v", ledger.Graph)
			}
			for _, row := range ledger.Graph.Nodes {
				if row.MandatoryProven != 0 || row.MandatoryTotal != 2 || row.OptionalProven != 0 || row.OptionalTotal != 1 {
					t.Fatalf("unrecorded tags changed figure: %+v", row)
				}
			}
		})
	}
}

// Linux limits remain binding, unmet and neutral on the detail surface and in
// the published JSON, with no unmapped annotation left behind.
func TestTheLedgerCountsLinuxLimitRows(t *testing.T) {
	root, summary, marker := partialPublicationTree(t)
	writePartialPublicationFile(t, root, "rfc/short/rfc9999.md", strings.Replace(summary, marker, "{linux-limit: LL-WIDGET; kernel has no primitive}", 1))
	ledger, err := collectRequirementLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	coverage := ledger.Stems[0].Coverage
	if coverage.LinuxLimit != 1 || coverage.UnmappedAnnotations != 0 || coverage.Bucket(rfcLinuxLimitBucket) != 1 {
		t.Fatalf("Linux limit lost: %+v", coverage)
	}
	body, err := json.Marshal(coverage)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"linux-cannot":1`) {
		t.Fatal(string(body))
	}
	if annotation := ledger.Stems[0].Requirements[0].Annotation; annotation == nil ||
		annotation.LimitID != "LL-WIDGET" || annotation.Reason != "kernel has no primitive" {
		t.Fatalf("publication lost the register reference: %+v", annotation)
	}
	found := false
	for _, card := range rfcDetailCards(&ledger.Stems[0]) {
		if card.Label == rfcLinuxLimitLabel {
			found = true
			if card.Tone != rfcToneNeutral || card.Part != 1 {
				t.Fatalf("Linux limit is not a neutral, counted part: %+v", card)
			}
		}
	}
	if !found {
		t.Fatal("Linux limit has no standing card")
	}
	for _, surface := range []string{rfcDetailBody(&ledger.Stems[0]), rfcDetailMirror(&ledger.Stems[0])} {
		for _, want := range []string{"Linux cannot do this", "obligation still binds", "unmet", "LL-WIDGET; kernel has no primitive"} {
			if !strings.Contains(surface, want) {
				t.Errorf("surface omits %q", want)
			}
		}
	}
	for _, surface := range []string{rfcGapsHTML(&ledger.Stems[0]), rfcGapsMirror(&ledger.Stems[0])} {
		for _, want := range []string{"{linux-limit}", "LL-WIDGET", "kernel has no primitive"} {
			if !strings.Contains(surface, want) {
				t.Errorf("gap disclosure lost %q: %s", want, surface)
			}
		}
	}
}

// Each producer publishes identical ceilings from a ledger with two distinct
// exclusion reasons and an unwalked member; no proof bar is replaced.
func TestCeilingPublicationsAgree(t *testing.T) {
	ledger := ceilingPublicationLedger()
	report := ledger.Profiles[0]
	paths := rfcCompliancePathsWith(t, publishedRFCComplianceRef(t), ledger)
	if _, err := renderRFCCompliance(paths); err != nil {
		t.Fatal(err)
	}
	index := readArtifact(t, paths.Output, rfcComplianceDest)
	indexMirror := readArtifact(t, paths.Output, rfcComplianceDirectory+"/"+pageMirrorFile)
	profile := rfcProfileBody(report, map[string]string{}, nil)
	profileMirror := rfcProfileMirror(report, map[string]string{}, nil)
	const mandatory = "Mandatory ceiling: 3 of 6 (50.0%)"
	const optional = "Optional ceiling: 1 of 2 (50.0%)"
	for name, surface := range map[string]string{"index": index, "index mirror": indexMirror, "profile": profile, "profile mirror": profileMirror} {
		for _, want := range []string{mandatory, optional, "Linux cannot do this: 1 of 6 (16.7%)", "Lowest mandatory ceiling: rfc9998 1 of 3 (33.3%)"} {
			if !strings.Contains(html.UnescapeString(surface), want) {
				t.Errorf("%s omits %q", name, want)
			}
		}
	}
	for _, surface := range []string{rfcProfilesIndexBody(ledger.Profiles), rfcProfilesIndexMirror(ledger.Profiles)} {
		for _, want := range []string{"3 of 6 (50.0%)", "1 of 2 (50.0%)", "Linux cannot do this: 1 of 6 (16.7%)"} {
			if !strings.Contains(html.UnescapeString(surface), want) {
				t.Errorf("profile index omits %q", want)
			}
		}
	}
	for _, card := range rfcProfileCards(report.Rollup) {
		if card.Label == "Mandatory ceiling" && card.Value != "50.0%" {
			t.Fatalf("card disagrees: %+v", card)
		}
	}
	for index := range ledger.Stems {
		entry := &ledger.Stems[index]
		if !entry.Figures.Walked {
			continue
		}
		for _, surface := range []string{rfcDetailBody(entry), rfcDetailMirror(entry)} {
			if !strings.Contains(html.UnescapeString(surface), rfcCeilingStatement([]*rfc.TreeRow{entry.Figures})) {
				t.Errorf("detail ceiling disagrees for %s", entry.Stem)
			}
		}
	}
}

// ceilingPublicationLedger supplies public renderers, including browser smoke,
// with measured counts. It changes no repository corpus annotation.
func ceilingPublicationLedger() rfcLedger {
	ledger := twoStemLedger()
	rows := []*rfc.TreeRow{
		{Stem: "rfc9998", Title: "Limited widgets", Summary: true, Walked: true,
			MandatoryProven: 1, MandatoryTotal: 3, MandatoryAttainable: 1,
			MandatoryExcluded: map[string]int{rfc.AnnotationLinuxLimit: 1, rfc.AnnotationNotApplicable: 1},
			OptionalTotal:     2, OptionalAttainable: 1, OptionalExcluded: map[string]int{rfc.AnnotationFeatureDeclined: 1}},
		{Stem: "rfc9997", Title: "Lower-layer widgets", Summary: true, Walked: true,
			MandatoryProven: 1, MandatoryTotal: 3, MandatoryAttainable: 2,
			MandatoryExcluded: map[string]int{rfc.AnnotationLowerLayer: 1}},
		{Stem: "rfc10000", Title: "Unwalked widgets", Summary: true, MandatoryTotal: 100,
			MandatoryExcluded: map[string]int{rfc.AnnotationLinuxLimit: 100}},
	}
	ledger.Stems[0].PublicStatus = "Partial"
	ledger.Stems[0].Requirements = append(ledger.Stems[0].Requirements,
		rfcLedgerRequirement{RID: "RFC9998-2-2", Level: "MUST", Gated: true, Text: "A sender MUST use the missing primitive.",
			Annotation: &rfcLedgerAnnotation{Kind: rfc.AnnotationLinuxLimit, LimitID: "LL-WIDGET", Reason: "kernel has no primitive"}},
		rfcLedgerRequirement{RID: "RFC9998-2-3", Level: "MUST", Gated: true, Text: "Another role MUST receive the widget.",
			Annotation: &rfcLedgerAnnotation{Kind: rfc.AnnotationNotApplicable, Reason: "another role"}},
		rfcLedgerRequirement{RID: "RFC9998-2-4", Level: "MAY", Text: "A sender MAY offer the optional widget.",
			Annotation: &rfcLedgerAnnotation{Kind: rfc.AnnotationFeatureDeclined, Reason: "optional feature not offered"}},
		rfcLedgerRequirement{RID: "RFC9998-2-5", Level: "SHOULD", Text: "A sender SHOULD provide the extension.",
			Annotation: &rfcLedgerAnnotation{Kind: rfc.AnnotationGap, Reason: "extension not implemented"}})
	ledger.Stems[0].Coverage = rfcLedgerCoverageOf(rfc.CoverageRow{Gated: 3, Both: 1, Annotated: 2}, ledger.Stems[0].Requirements)
	ledger.Stems = append(ledger.Stems, rfcLedgerStem{Stem: "rfc10000", Display: "RFC 10000", Enrolled: true})
	ledger.Graph.Nodes = rows
	byStem := rfcStemRows(ledger)
	for index := range ledger.Stems {
		ledger.Stems[index].Figures = byStem[ledger.Stems[index].Stem]
	}
	ceiling := rfc.CeilingOf(rows)
	ledger.Profiles = []*rfc.TreeReport{{Profile: "widgets", Title: "Widget ceilings", Kind: "use-case",
		Rollup: &rfc.ScopeRollup{Rollup: ceiling.Rollup}, Rows: rows}}
	return ledger
}

// ceilingDraftTiePublicationLedger is the supplemental browser fixture. Drafts
// precede the tied RFC in the rendered table, and reverse lexical draft order
// catches retaining the first row rather than applying the model's tie break.
func ceilingDraftTiePublicationLedger() rfcLedger {
	ledger := ceilingPublicationLedger()
	drafts := []*rfc.TreeRow{
		{Stem: "draft-z-widget", Title: "Draft Z widgets", Summary: true, Walked: true,
			MandatoryTotal: 3, MandatoryAttainable: 1,
			MandatoryExcluded: map[string]int{rfc.AnnotationLinuxLimit: 2}},
		{Stem: "draft-a-widget", Title: "Draft A widgets", Summary: true, Walked: true,
			MandatoryTotal: 3, MandatoryAttainable: 1,
			MandatoryExcluded: map[string]int{rfc.AnnotationLinuxLimit: 2}},
	}
	stems := make([]rfcLedgerStem, 0, len(drafts)+len(ledger.Stems))
	for _, row := range drafts {
		stems = append(stems, rfcLedgerStem{Stem: row.Stem, Display: row.Title, Title: row.Title,
			Enrolled: true, Figures: row})
	}
	ledger.Stems = append(stems, ledger.Stems...)
	ledger.Graph.Nodes = append(drafts, ledger.Graph.Nodes...)
	report := ledger.Profiles[0]
	report.Rows = ledger.Graph.Nodes
	ceiling := rfc.CeilingOf(report.Rows)
	report.Rollup = &rfc.ScopeRollup{Rollup: ceiling.Rollup}
	return ledger
}

// These are the permanent model expectations for the generated browser
// fixture. Browser execution additionally proves the script follows them.
func TestCeilingDraftTiesUseTheModelStemOrder(t *testing.T) {
	ledger := ceilingDraftTiePublicationLedger()
	for _, testCase := range []struct {
		name string
		rows []*rfc.TreeRow
		want string
	}{
		{"RFC follows drafts", ledger.Graph.Nodes, "rfc9998"},
		{"draft names", ledger.Graph.Nodes[:2], "draft-a-widget"},
		{"draft names reversed", []*rfc.TreeRow{ledger.Graph.Nodes[1], ledger.Graph.Nodes[0]}, "draft-a-widget"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ceiling := rfc.CeilingOf(testCase.rows)
			if ceiling.LowestMandatory == nil || ceiling.LowestMandatory.Stem != testCase.want ||
				ceiling.LowestMandatory.Attainable != 1 || ceiling.LowestMandatory.Total != 3 {
				t.Fatalf("model tie break = %+v, want %s 1/3", ceiling.LowestMandatory, testCase.want)
			}
			want := "Lowest mandatory ceiling: " + testCase.want + " 1 of 3 (33.3%)"
			if !strings.Contains(rfcCeilingStatement(testCase.rows), want) {
				t.Fatalf("server-rendered statement omits %q", want)
			}
		})
	}
	paths := rfcCompliancePathsWith(t, publishedRFCComplianceRef(t), ledger)
	if _, err := renderRFCCompliance(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, rfcComplianceDest)
	z := strings.Index(page, `data-rfc="draft-z-widget"`)
	a := strings.Index(page, `data-rfc="draft-a-widget"`)
	number := strings.Index(page, `data-rfc="rfc9998"`)
	if z < 0 || a <= z || number <= a {
		t.Fatal("browser fixture must encounter the reverse-ordered drafts before the tied RFC")
	}
}

// A use case's optional tier has its own measured set. Its excluded rows must
// not lower the required-tier cards or minima, in HTML or the mirror.
func TestProfileOptionalTierKeepsOwnCeiling(t *testing.T) {
	ledger := ceilingPublicationLedger()
	report := ledger.Profiles[0]
	optional := []*rfc.TreeRow{report.Rows[0], report.Rows[2]}
	report.Rows = []*rfc.TreeRow{report.Rows[1]}
	requiredCeiling := rfc.CeilingOf(report.Rows)
	optionalCeiling := rfc.CeilingOf(optional)
	report.Rollup = &rfc.ScopeRollup{Rollup: requiredCeiling.Rollup}
	report.Optional = &rfc.TreeTier{Rollup: rfc.ScopeRollup{Rollup: optionalCeiling.Rollup}, Rows: optional}
	for _, surface := range []string{rfcProfileBody(report, nil, nil), rfcProfileMirror(report, nil, nil)} {
		required, rest, found := strings.Cut(html.UnescapeString(surface), rfcProfileOptionalHeading)
		if !found {
			t.Fatal("optional tier disappeared")
		}
		if !strings.Contains(required, "Mandatory ceiling: 2 of 3 (66.7%)") ||
			strings.Contains(required, "Mandatory ceiling: 1 of 3") {
			t.Fatal("optional exclusions changed the required tier")
		}
		if !strings.Contains(rest, "Mandatory ceiling: 1 of 3 (33.3%)") ||
			!strings.Contains(rest, "Linux cannot do this: 1 of 3 (33.3%)") ||
			strings.Contains(rest, "101 of") {
			t.Fatal("optional tier lost its own walked-only ceiling")
		}
	}
}

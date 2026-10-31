// Design: docs/contributing/gh-pages.md -- what the RFC ledger counts behind each public RFC surface.
package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// Publication reads authored annotations through Collect and NewRenderInput;
// none except single-polarity can turn the same tagged row into public credit.
// The stem is walked and both gated rows carry verified records, so the only
// thing that differs between subtests is the annotation on RFC9999-2-1: the
// pair on RFC9999-2-2 is whole credit, and RFC9999-2-1 earns it only when
// its annotation is single-polarity.
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
			writePartialPublicationFile(t, root, "rfc/full/rfc9999.txt", "Test RFC 9999\n\n2.  Widgets\n\n"+
				"A widget MUST be zero when sent and ignored on receipt.\nA receiver MUST accept a valid widget.\n")
			writePartialPublicationFile(t, root, "test/plugin/pair.ci",
				"# RFC requirement: RFC9999-2-2 positive -- accepts valid widget\n"+
					"# RFC requirement: RFC9999-2-2 negative -- refuses invalid widget\nexpect=stdout:contains=OK\n")
			// Every gated row carries a verified record in both polarities, the
			// annotated one included, so a kind that granted credit for a record
			// would show here: only single-polarity may.
			var records []rfc.DiscriminationRecord
			for _, unit := range []struct{ rid, ci string }{
				{"RFC9999-2-1", "test/plugin/widget.ci"}, {"RFC9999-2-2", "test/plugin/pair.ci"},
			} {
				for _, polarity := range []string{rfc.PolarityPositive, rfc.PolarityNegative} {
					records = append(records, rfc.DiscriminationRecord{RID: unit.rid, Polarity: polarity,
						Unit: unit.ci, Route: rfc.RouteRevert, Producer: "internal/sample/widget.go::SendWidget",
						Break: "return count -> return 0", Citation: "expect=stdout:contains=OK"})
				}
			}
			if err := rfc.FixtureProof(root, "rfc9999", map[string]string{"2:1": "RFC9999-2-1", "2:2": "RFC9999-2-2"}, records...); err != nil {
				t.Fatal(err)
			}
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
			render, err := rfc.NewRenderInput(root, collected, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			figures, err := rfc.StemFiguresOf(&render)
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if traits.SinglePolarity {
				want = 2
			}
			figure := figures["rfc9999"]
			if !figure.Walked || figure.Mandatory.Proven != want || figure.Mandatory.Total != 2 {
				t.Fatalf("annotation changed the stem figure: %+v", figure)
			}
			share, err := rfc.ProvenShareOf(collected.Metas, figures)
			if err != nil {
				t.Fatal(err)
			}
			if share.Proven != want || share.Gated != 2 {
				t.Fatalf("annotation changed public share: %+v", share)
			}
			if len(ledger.Graph.Nodes) != 1 {
				t.Fatalf("published graph lost the measured RFC: %+v", ledger.Graph)
			}
			for _, row := range ledger.Graph.Nodes {
				if row.Mandatory.Proven != want || row.Mandatory.Total != 2 || row.Optional.Proven != 0 || row.Optional.Total != 1 {
					t.Fatalf("annotation changed the published figure: %+v", row)
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

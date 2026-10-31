package rfc

import (
	"strings"
	"testing"
)

// optionalLowerLayerRow is one SHOULD rfc1 row a layer under Ze performs,
// naming the same producer lowerLayerRow names.
func optionalLowerLayerRow(rid string) Requirement {
	row := lowerLayerRow(rid)
	row.Level = levelShould
	return row
}

// TestOptionalLowerLayerRowIsOSVerifiedOrOSGap checks the owner decision of
// 2026-10-31 on the figures. Goal: a SHOULD {lower-layer} row is split the way
// a MUST one is, OS verified when a proving record broke the producer its
// annotation names and an OS testing gap otherwise, into the optional counts
// only; it is never "not decided", and the gated counts still hold only the
// gated row. Method: one gated and three optional lower-layer rows through
// figuresByStem, with records proving the gated row, one optional row through
// its named producer, and one optional row through another producer.
func TestOptionalLowerLayerRowIsOSVerifiedOrOSGap(t *testing.T) {
	requirements := []Requirement{
		lowerLayerRow("RFC1-1-1"),
		optionalLowerLayerRow("RFC1-1-2"),
		optionalLowerLayerRow("RFC1-1-3"),
		optionalLowerLayerRow("RFC1-1-4"),
	}
	verdicts := []DiscriminationVerdict{
		{Record: DiscriminationRecord{RID: "RFC1-1-1", Route: RouteRevert, Producer: lowerLayerProducer}, State: ProofVerified},
		{Record: DiscriminationRecord{RID: "RFC1-1-2", Route: RouteRevert, Producer: lowerLayerProducer}, State: ProofVerified},
		{Record: DiscriminationRecord{RID: "RFC1-1-4", Route: RouteRevert,
			Producer: "internal/plugins/ospf/ipsec_install.go::removeIPsecSA"}, State: ProofVerified},
	}
	stems := map[string]bool{"rfc1": true}
	figures := figuresByStem(requirements, stems, stems, provenRequirements(verdicts),
		figureEvidence{osProofs: osProofsOf(verdicts)})["rfc1"]
	if figures.OptionalOSVerified != 1 || figures.OptionalOSGap != 2 {
		t.Errorf("OptionalOSVerified=%d OptionalOSGap=%d, want 1 and 2", figures.OptionalOSVerified, figures.OptionalOSGap)
	}
	if figures.OSVerified != 1 || figures.OSGap != 0 {
		t.Errorf("OSVerified=%d OSGap=%d, want 1 and 0: the gated split holds the gated row only", figures.OSVerified, figures.OSGap)
	}
	if sum := figures.OptionalOSVerified + figures.OptionalOSGap; sum != figures.Optional.Excluded[AnnotationLowerLayer] {
		t.Errorf("optional OS rows = %d, want the %d optional lower-layer rows", sum, figures.Optional.Excluded[AnnotationLowerLayer])
	}
	if figures.NotDecided != 0 || figures.NotYet != 0 || figures.NotEver != 0 {
		t.Errorf("NotYet=%d NotEver=%d NotDecided=%d, want none: the kernel performs a lower-layer row",
			figures.NotYet, figures.NotEver, figures.NotDecided)
	}
	if figures.Optional.Proven != 0 || figures.Optional.Total != 3 {
		t.Errorf("optional = %+v, want 0 proven of 3: OS verified is not proven by Ze", figures.Optional)
	}
}

// TestIntentMarkerOnALowerLayerRowSaysTheKernelPerformsIt checks the owner
// decision of 2026-10-31 on the check. Goal: an {intent} marker on a row a
// layer under Ze performs is refused with a reason naming that the kernel
// already performs it and both fixes, removing the marker or reclassifying the
// row; a marker on another excluded row keeps its own reason. Method: a
// lower-layer and a not-applicable SHOULD row, each marked, through
// evaluateIntent.
func TestIntentMarkerOnALowerLayerRowSaysTheKernelPerformsIt(t *testing.T) {
	lower := optionalLowerLayerRow("RFC1-1-1")
	lower.Intent = &Intent{State: IntentNotYet}
	other := Requirement{RFC: "rfc1", RID: "RFC1-1-2", Level: levelShould, Intent: &Intent{State: IntentNotYet},
		Annotation: &Annotation{Kind: AnnotationNotApplicable, Reason: "binds a relay"}}
	findings := evaluateIntent([]Requirement{lower, other}, map[string]bool{}, acceptEveryExclusion{})
	if len(findings) != 2 {
		t.Fatalf("got %d findings, want one per marked row: %+v", len(findings), findings)
	}
	byRID := map[string]string{}
	for _, finding := range findings {
		byRID[finding.RID] = finding.Message
	}
	for _, says := range []string{"RFC1-1-1", "the kernel already performs it", "remove the intent marker", "reclassify the row"} {
		if !strings.Contains(byRID["RFC1-1-1"], says) {
			t.Errorf("the lower-layer refusal %q does not say %q", byRID["RFC1-1-1"], says)
		}
	}
	if strings.Contains(byRID["RFC1-1-2"], "kernel") {
		t.Errorf("the not-applicable refusal %q names the kernel, which does not perform that row", byRID["RFC1-1-2"])
	}
}

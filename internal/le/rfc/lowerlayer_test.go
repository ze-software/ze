// VALIDATES: the {lower-layer} annotation kind -- its format, the producer its
// reason must name, its place inside the gated denominator and outside the
// proven numerator, and its inability to take a {gap}'s slot.
// PREVENTS: a second {not-applicable}. That kind says "this never bound Ze" and
// nothing in the tree can contradict it; this one says "a layer under Ze does
// it, and THIS function installs into that layer", which the gate goes and
// checks. Drop the producer demand and the difference is gone.

package rfc

import (
	"strings"
	"testing"
)

// lowerLayerLine is one well-formed {lower-layer} checklist line.
const lowerLayerLine = "- [ ] [RFC9999-2-1] [MUST] The RESERVED field MUST be set to zero (§2) " +
	"{lower-layer: Linux XFRM; internal/plugins/ospf/ipsec_install.go::buildIPsecSA installs " +
	"the AH SA and the kernel builds every AH header, so no value Ze writes decides this field}"

func TestALowerLayerAnnotationCarriesItsLayerAndItsProducer(t *testing.T) {
	req, err := oneLine(t, lowerLayerLine)
	if err != nil {
		t.Fatalf("a well-formed lower-layer annotation: %v", err)
	}
	if req.Annotation == nil || req.Annotation.Kind != AnnotationLowerLayer {
		t.Fatalf("the annotation was lost: %+v", req.Annotation)
	}
	if req.Annotation.Layer != "Linux XFRM" {
		t.Errorf("layer: %q, want %q", req.Annotation.Layer, "Linux XFRM")
	}
	want := "internal/plugins/ospf/ipsec_install.go::buildIPsecSA"
	if req.Annotation.Producer != want {
		t.Errorf("producer: %q, want %q", req.Annotation.Producer, want)
	}
	// Reason keeps the whole body, so a renderer that knows only `{kind} reason`
	// still publishes the layer and the producer.
	if !strings.HasPrefix(req.Annotation.Reason, "Linux XFRM; ") ||
		!strings.Contains(req.Annotation.Reason, want) {
		t.Errorf("reason: %q, want the layer and the producer", req.Annotation.Reason)
	}
	if strings.Contains(req.Text, "{") {
		t.Errorf("the marker was left inside the requirement text: %q", req.Text)
	}
	if req.Section != "2" {
		t.Errorf("the marker moved the anchor: %q", req.Section)
	}
}

// TestALowerLayerReasonIsRefusedWithoutBothFacts drives the two halves of the
// format apart, because a reason naming a layer and no producer is exactly the
// assertable claim this kind may not become.
func TestALowerLayerReasonIsRefusedWithoutBothFacts(t *testing.T) {
	cases := []struct {
		name, body, refused string
	}{
		{
			name:    "a layer and prose, but no producer",
			body:    "{lower-layer: Linux XFRM; the kernel builds every AH header}",
			refused: "names no producer",
		},
		{
			name:    "a producer with no layer before it",
			body:    "{lower-layer: internal/plugins/ospf/ipsec_install.go::buildIPsecSA installs the SA}",
			refused: "needs the layer that performs the behavior",
		},
		{
			name:    "a layer with nothing after it",
			body:    "{lower-layer: Linux XFRM;}",
			refused: "needs the layer that performs the behavior",
		},
		{
			name: "a test named as the producer",
			body: "{lower-layer: Linux XFRM; internal/plugins/ospf/ipsec_install_test.go::TestBuildIPsecSA " +
				"proves the SA Ze installs}",
			refused: "installs nothing into Linux XFRM",
		},
		{
			name: "a producer naming no symbol",
			body: "{lower-layer: Linux XFRM; internal/plugins/ospf/ipsec_install.go installs " +
				"the SA the kernel then uses}",
			refused: "names no producer",
		},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			_, err := oneLine(t, "- [ ] [RFC9999-2-1] [MUST] A speaker MUST send it (§2) "+one.body)
			if err == nil {
				t.Fatalf("%s parsed, want a refusal", one.body)
			}
			if !strings.Contains(err.Error(), one.refused) {
				t.Errorf("refusal %q does not say %q", err.Error(), one.refused)
			}
			if !strings.Contains(err.Error(), "Format: {lower-layer:") {
				t.Errorf("refusal %q shows the author no format", err.Error())
			}
		})
	}
}

// TestALowerLayerAnnotationCannotDisplaceAGap holds the property the note above
// SupersededKind records: a way OUT of the gated population must not be
// creatable by adding a marker beside the one already there.
//
// The kind sits in the coverage register, where one line carries one
// disposition, so a {gap} and a {lower-layer} on one line is a contradiction
// the parser refuses. Had it been a composing marker instead, writing it beside
// a gap would have silently retired the gap.
func TestALowerLayerAnnotationCannotDisplaceAGap(t *testing.T) {
	const producer = "internal/plugins/ospf/ipsec_install.go::buildIPsecSA"
	for _, line := range []string{
		"- [ ] [RFC9999-2-1] [MUST] A speaker MUST send it (§2) {gap: no producer yet} " +
			"{lower-layer: Linux XFRM; " + producer + " installs the SA}",
		"- [ ] [RFC9999-2-1] [MUST] A speaker MUST send it (§2) " +
			"{lower-layer: Linux XFRM; " + producer + " installs the SA} {gap: no producer yet}",
	} {
		req, err := oneLine(t, line)
		if err == nil {
			t.Fatalf("a line carrying both annotations parsed as %+v, want a refusal", req.Annotation)
		}
		if !strings.Contains(err.Error(), "two coverage annotations on one line") {
			t.Errorf("refusal %q does not name the collision", err.Error())
		}
	}
}

// TestALowerLayerRequirementIsAnnotatedAndNotProven runs the same corpus twice,
// once with the annotation and once without it, and demands the published share
// be identical.
//
// The requirement is MET, by a layer under Ze, and it is not proven BY ZE. So
// it belongs in the denominator both times and in the numerator neither time:
// annotating a row may not move the published percentage by a point, which is
// what would happen if the kind reached CoverageRow.Both or the single-polarity
// arm of ProvenShareOf.
func TestALowerLayerRequirementIsAnnotatedAndNotProven(t *testing.T) {
	metas := map[string]Meta{"rfc1": {Enrolment: enrolmentEnrolled, Support: "core", Status: "Supported"}}
	tags := []Tag{
		{RID: "RFC1-1-1", Polarity: PolarityPositive, File: "a_test.go"},
		{RID: "RFC1-1-1", Polarity: PolarityNegative, File: "a_test.go"},
	}
	bare := []Requirement{
		{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust},
		{RFC: "rfc1", RID: "RFC1-1-2", Level: levelMust},
	}
	annotated := []Requirement{
		{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust},
		{RFC: "rfc1", RID: "RFC1-1-2", Level: levelMust, Annotation: &Annotation{
			Kind: AnnotationLowerLayer, Layer: "Linux XFRM",
			Producer: "internal/plugins/ospf/ipsec_install.go::buildIPsecSA",
			Reason:   "the kernel performs it",
		}},
	}

	rows := CoverageRows(annotated, tags, nil)
	if len(rows) != 1 {
		t.Fatalf("CoverageRows answered %d rows, want 1", len(rows))
	}
	row := rows[0]
	if row.Annotated != 1 {
		t.Errorf("Annotated = %d, want 1: a lower-layer requirement is annotated", row.Annotated)
	}
	if row.Both != 1 || row.One != 0 || row.Missing != 0 {
		t.Errorf("both=%d one=%d missing=%d, want the annotated requirement in none of them",
			row.Both, row.One, row.Missing)
	}
	if row.Gated != 2 {
		t.Errorf("Gated = %d, want 2: the annotation does not leave the population", row.Gated)
	}

	before, err := ProvenShareOf(metas, bare, tags, nil)
	if err != nil {
		t.Fatalf("ProvenShareOf over the bare corpus: %v", err)
	}
	after, err := ProvenShareOf(metas, annotated, tags, nil)
	if err != nil {
		t.Fatalf("ProvenShareOf over the annotated corpus: %v", err)
	}
	if after != before {
		t.Errorf("the published share moved when a requirement gained {lower-layer}: %+v, was %+v",
			after, before)
	}
	if after.Proven != 1 || after.Gated != 2 {
		t.Errorf("Proven=%d Gated=%d, want 1 of 2: met below Ze is not proven by Ze",
			after.Proven, after.Gated)
	}
}

// TestALowerLayerAnnotationIsStaleBesideATaggedTest checks the arm of evaluate
// that {lower-layer} joined. A requirement Ze CAN prove is one the annotation
// may not cover, and the tag is the evidence that it can.
func TestALowerLayerAnnotationIsStaleBesideATaggedTest(t *testing.T) {
	requirements := []Requirement{{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust,
		Source: "rfc/short/rfc1.md", Line: 7, Annotation: &Annotation{
			Kind: AnnotationLowerLayer, Layer: "Linux XFRM",
			Producer: "internal/plugins/ospf/ipsec_install.go::buildIPsecSA",
			Reason:   "the kernel performs it",
		}}}
	tags := []Tag{{RID: "RFC1-1-1", Polarity: PolarityPositive, File: "a_test.go", Line: 3}}

	findings := evaluate(requirements, tags, map[string]bool{"rfc1": true})
	if len(findings) != 1 {
		t.Fatalf("evaluate answered %d findings, want 1: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "the annotation is stale") {
		t.Errorf("finding %q does not say the annotation is stale", findings[0].Message)
	}
}

// TestCheckLowerLayerProducerReadsTheTree is the check that makes the kind
// checkable rather than assertable: the named producer has to be findable, and
// a rename or a deletion under the annotation is the event it catches.
func TestCheckLowerLayerProducerReadsTheTree(t *testing.T) {
	const file = "internal/plugins/ospf/ipsec_install.go"
	source := "package ospf\n\nfunc buildIPsecSA(ifindex int) int {\n\treturn ifindex\n}\n"

	cases := []struct {
		name, producer, refused string
	}{
		{name: "a producer the tree holds", producer: file + "::buildIPsecSA"},
		{
			name: "a symbol the file no longer declares", producer: file + "::buildIPsecSAOld",
			refused: "declares no buildIPsecSAOld",
		},
		{
			name: "a file this checkout does not carry", producer: "internal/gone/away.go::buildIPsecSA",
			refused: "this checkout does not carry",
		},
		{
			// Unreachable through the parser, which refuses the line first. The
			// check still refuses it, because a guard that skips the one state
			// it exists to catch is not a guard.
			name: "an annotation carrying no producer at all", producer: "",
			refused: "names no producer",
		},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			requirements := []Requirement{{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust,
				Source: "rfc/short/rfc1.md", Line: 7, Annotation: &Annotation{
					Kind: AnnotationLowerLayer, Layer: "Linux XFRM",
					Producer: one.producer, Reason: one.producer + " installs the SA",
				}}}
			errs := checkLowerLayerProducer(newTextReader(map[string]string{file: source}), requirements)
			if one.refused == "" {
				if len(errs) != 0 {
					t.Fatalf("a producer the tree holds was refused: %v", errs)
				}
				return
			}
			if len(errs) != 1 {
				t.Fatalf("checkLowerLayerProducer answered %d errors, want 1: %v", len(errs), errs)
			}
			if !strings.Contains(errs[0], one.refused) {
				t.Errorf("refusal %q does not say %q", errs[0], one.refused)
			}
		})
	}
}

// TestAMethodIsAProducerToo checks the receiver form, because the function that
// installs into a layer is as often a method as a plain function and funcNameIn
// reads the name without its receiver.
func TestAMethodIsAProducerToo(t *testing.T) {
	const file = "internal/component/ike/dataplane/xfrm_linux.go"
	source := "package dataplane\n\nfunc (d *dataplane) InstallSA(p SAParams) error {\n\treturn nil\n}\n"
	requirements := []Requirement{{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust,
		Source: "rfc/short/rfc1.md", Line: 7, Annotation: &Annotation{
			Kind: AnnotationLowerLayer, Layer: "Linux XFRM",
			Producer: file + "::(*dataplane).InstallSA",
			Reason:   file + "::(*dataplane).InstallSA writes the state",
		}}}
	if errs := checkLowerLayerProducer(newTextReader(map[string]string{file: source}), requirements); len(errs) != 0 {
		t.Errorf("a method producer was refused: %v", errs)
	}
}

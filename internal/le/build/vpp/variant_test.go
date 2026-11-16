// Design: docs/labs/pppoe-interop.md -- source-identical baseline and counterfactual images.
// VALIDATES: the proof variant changes only its named source mutation and image
// identity, while preserving the baseline recipe, context and original image.
package buildvpp

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

func TestVPPProofVariantSelectsSeparateImage(t *testing.T) {
	variant, err := parseProofVariant(metadataVariantName)
	if err != nil {
		t.Fatal(err)
	}
	builder := &recordingBuilder{}
	report, err := buildImage(t.Context(), t.TempDir(), variant, builder)
	if err != nil {
		t.Fatal(err)
	}
	if builder.build.Tag != MetadataImageName {
		t.Fatalf("mutant would replace baseline tag: %q", builder.build.Tag)
	}
	if !slices.Equal(builder.build.BuildArgs, []string{"VPP_PROOF_VARIANT=missing-frame-metadata"}) {
		t.Fatalf("mutation argument = %q", builder.build.BuildArgs)
	}
	if report.ProofVariant != metadataVariantName {
		t.Fatalf("report lost the selected mutation: %#v", report)
	}
	if report.Image.Tag != MetadataImageName {
		t.Fatalf("report names the wrong image tag: %q", report.Image.Tag)
	}
	if report.Image.Reference == "" {
		t.Fatal("report omitted immutable image identity")
	}
}

func TestVPPProofVariantPreservesBaselineRecipe(t *testing.T) {
	root := t.TempDir()
	builds, _, err := interoplab.ResolveImages(interoplab.ImageContext{Root: root},
		[]string{ImageName, MetadataImageName})
	if err != nil {
		t.Fatal(err)
	}
	if len(builds) != 2 {
		t.Fatalf("resolved %d images, want both proof variants", len(builds))
	}
	if builds[0].Dockerfile != builds[1].Dockerfile {
		t.Fatal("counterfactual uses another build recipe")
	}
	if builds[0].Context != builds[1].Context {
		t.Fatal("counterfactual uses another source context")
	}
	if builds[0].Tag != ImageName {
		t.Fatal("resolving the mutant changed the baseline tag")
	}
	if len(builds[0].BuildArgs) != 0 {
		t.Fatalf("baseline acquired a proof mutation: %q", builds[0].BuildArgs)
	}
}

func TestVPPProofVariantRefusesUnknownName(t *testing.T) {
	if _, err := parseProofVariant("other"); err == nil {
		t.Fatal("an undeclared proof mutation was accepted")
	}
}

func TestVPPProofVariantOmissionSelectsBaseline(t *testing.T) {
	for _, value := range []string{"", baselineVariantName} {
		variant, err := parseProofVariant(value)
		if err != nil {
			t.Fatal(err)
		}
		if variant != proofBaseline {
			t.Fatalf("proof variant %q = %v, want baseline", value, variant)
		}
		builder := &recordingBuilder{}
		report, err := buildImage(t.Context(), t.TempDir(), variant, builder)
		if err != nil {
			t.Fatal(err)
		}
		if report.ProofVariant != baselineVariantName {
			t.Fatalf("baseline report names variant %q", report.ProofVariant)
		}
		if report.Image.Tag != ImageName {
			t.Fatalf("baseline report names image %q", report.Image.Tag)
		}
	}
}

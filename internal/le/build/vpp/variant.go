// Design: docs/labs/pppoe-interop.md -- closed, lab-only counterfactual images.
// Related: build.go -- selected image identity and immutable proof report.
package buildvpp

import (
	"errors"

	"github.com/ze-software/ze/internal/core/opererr"
)

const (
	baselineVariantName = "baseline"
	metadataVariantName = "missing-frame-metadata"
	proofBuildArgument  = "VPP_PROOF_VARIANT"
)

// proofVariant is constructed only by the boundary parser. Its zero value is
// unspecified and cannot select an image.
type proofVariant uint8

const (
	proofUnspecified proofVariant = iota
	proofBaseline
	proofMissingMetadata
)

func parseProofVariant(value string) (proofVariant, error) {
	switch value {
	case "", baselineVariantName:
		return proofBaseline, nil
	case metadataVariantName:
		return proofMissingMetadata, nil
	}
	return proofUnspecified, opererr.New("unknown VPP proof variant "+value,
		"The lab builds only the baseline or the missing-frame-metadata counterfactual.",
		"./le build vpp run variant baseline")
}

type proofDescriptor struct {
	name  string
	image string
}

func proofImage(variant proofVariant) (proofDescriptor, error) {
	switch variant {
	case proofUnspecified:
		return proofDescriptor{}, errors.New("no VPP proof variant was selected")
	case proofBaseline:
		return proofDescriptor{name: baselineVariantName, image: ImageName}, nil
	case proofMissingMetadata:
		return proofDescriptor{name: metadataVariantName, image: MetadataImageName}, nil
	}
	return proofDescriptor{}, errors.New("unrecognized VPP proof variant")
}

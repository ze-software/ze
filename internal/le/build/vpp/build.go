// Design: docs/labs/pppoe-interop.md -- reproducible VPP image for PPPoE lab proofs.
// Related: register.go -- the native action and shared image declaration.
// Upstream: https://github.com/FDio/vpp/tree/v25.10; the lab builds the
// retained VyOS changes documented in third_party/vpp-patches/README.md.
package buildvpp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// ImageName identifies the shared lab image. Its Dockerfile is the sole source
// of the VPP version, source digest and patch order.
const ImageName = "ze-vpp-pppoe-lab"

// MetadataImageName keeps the counterfactual separate from the baseline cache.
const MetadataImageName = ImageName + "-" + metadataVariantName

// Report names the immutable artifact and its build recipe. Building the image
// proves no PPPoE behavior: the deployment lab must run it separately.
type Report struct {
	Image        interoplab.ImageResult `json:"image"`
	Recipe       string                 `json:"recipe"`
	ProofVariant string                 `json:"proof-variant"`
}

// imageBuilder owns the Docker retention tag. buildImage MUST release that tag
// after a successful Build; ReleaseImage MUST preserve the shared cache tag.
type imageBuilder interface {
	Build(context.Context, interoplab.ImageBuild) (interoplab.ImageResult, error)
	ReleaseImage(context.Context, interoplab.ImageResult) error
}

func buildImage(ctx context.Context, root string, variant proofVariant, builder imageBuilder) (Report, error) {
	proof, err := proofImage(variant)
	if err != nil {
		return Report{}, err
	}
	builds, _, err := interoplab.ResolveImages(interoplab.ImageContext{Root: root}, []string{proof.image})
	if err != nil {
		return Report{}, err
	}
	if len(builds) != 1 {
		return Report{}, errors.New("the VPP image registration did not resolve exactly one build")
	}
	build := builds[0]
	build.Progress = os.Stderr
	image, err := builder.Build(ctx, build)
	if err != nil {
		return Report{}, err
	}
	report := Report{Image: image, Recipe: build.Dockerfile, ProofVariant: proof.name}
	if err := builder.ReleaseImage(context.WithoutCancel(ctx), image); err != nil {
		return report, err
	}
	return report, nil
}

func imageRecipe(context interoplab.ImageContext) interoplab.ImageBuild {
	directory := filepath.Join(context.Root, "third_party", "vpp-patches")
	return interoplab.ImageBuild{
		Name: ImageName, Tag: ImageName,
		Dockerfile: filepath.Join(directory, "Dockerfile"), Context: directory,
		// Suite callers need the compiler diagnostics the build action retains.
		Progress: os.Stderr,
		// The two-core, two-GiB Docker host builds one translation unit at a
		// time. Bound the whole build rather than increasing concurrency.
		Timeout: 45 * time.Minute, Required: true,
	}
}

func metadataImageRecipe(context interoplab.ImageContext) interoplab.ImageBuild {
	build := imageRecipe(context)
	build.Name = MetadataImageName
	build.Tag = MetadataImageName
	build.BuildArgs = []string{proofBuildArgument + "=" + metadataVariantName}
	return build
}

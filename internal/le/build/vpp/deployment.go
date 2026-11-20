// Design: third_party/vpp-patches-v26.06/README.md -- the patched VPP the deployment run boots.
// Related: register.go -- the action table that reaches this build.
// Upstream: https://github.com/FDio/vpp/tree/v26.06; the image carries Ze's
// IPsec patch documented in third_party/vpp-patches-v26.06/README.md.
package buildvpp

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/le/interoplab"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// DeploymentImageName identifies the VPP image `./le test deployment vpp`
// boots: VPP v26.06 with Ze's IPsec patch. Its Dockerfile is the sole source of
// the VPP version, source digest and patch list.
const DeploymentImageName = "ze-vpp-deployment"

// DeploymentBuildCommand is the one command that builds DeploymentImageName.
// The deployment run never builds it: a cold cache refuses naming this.
const DeploymentBuildCommand = "./le build vpp deployment"

// DeploymentReport names the immutable image and its recipe. Building it
// proves no VPP behavior: the deployment run must boot it.
type DeploymentReport struct {
	Image  interoplab.ImageResult `json:"image"`
	Recipe string                 `json:"recipe"`
}

// deploymentImageRecipe builds the whole of VPP from source, every plugin the
// build host's libraries allow, so the deployment run's plugin probes see what
// an upstream release image ships.
func deploymentImageRecipe(context interoplab.ImageContext) interoplab.ImageBuild {
	directory := filepath.Join(context.Root, "third_party", "vpp-patches-v26.06")
	return interoplab.ImageBuild{
		Name: DeploymentImageName, Tag: DeploymentImageName,
		Dockerfile: filepath.Join(directory, "Dockerfile"), Context: directory,
		Progress: os.Stderr,
		// A whole-tree compile is longer than the PPPoE lab's three plugins.
		// The bound only ever lengthens the machine's (ImageBuild.Timeout).
		Timeout: 90 * time.Minute, Required: true,
		// VPP_BUILD_NATIVE_ONLY compiles for the build host's CPU, so the
		// cache key carries that CPU and an entry is never loaded on another.
		Cache: true, HostCPUSpecific: true,
	}
}

// buildDeploymentImage builds the registered deployment image, or loads it
// from the image cache, then releases the build's run-owned tag and keeps
// DeploymentImageName, the tag the deployment run boots.
func buildDeploymentImage(ctx context.Context, root string, builder imageBuilder) (DeploymentReport, error) {
	builds, _, err := interoplab.ResolveImages(interoplab.ImageContext{Root: root}, []string{DeploymentImageName})
	if err != nil {
		return DeploymentReport{}, err
	}
	if len(builds) != 1 {
		return DeploymentReport{}, errors.New("the VPP deployment image registration did not resolve exactly one build")
	}
	build := builds[0]
	build.Progress = os.Stderr
	image, err := builder.Build(ctx, build)
	if err != nil {
		return DeploymentReport{}, err
	}
	report := DeploymentReport{Image: image, Recipe: build.Dockerfile}
	if err := builder.ReleaseImage(context.WithoutCancel(ctx), image); err != nil {
		return report, err
	}
	return report, nil
}

func answerDeployment() (any, int) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "the VPP deployment image build cannot find the Ze checkout",
			"The source pin and the patch cannot be located. Change to the checkout containing go.mod and run the build there.",
			DeploymentBuildCommand))
		return nil, 1
	}
	report, err := buildDeploymentImage(ctx, root, newImageBuilder())
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "the patched VPP deployment image could not be prepared",
			"`./le test deployment vpp` boots this image and never builds it. Check the Docker build output and repair the reported source, patch or Docker failure before retrying.",
			DeploymentBuildCommand))
		return nil, 1
	}
	return report, 0
}

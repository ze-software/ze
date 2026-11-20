// Design: third_party/vpp-patches-v26.06/README.md -- the patched VPP the deployment run boots.
// VALIDATES: `./le build vpp deployment` builds the registered deployment image
// from its own directory, cached per CPU, keeps its immutable identity, and
// releases only the run-owned tag.
package buildvpp

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestVPPDeploymentBuildUsesItsRegisteredImage(t *testing.T) {
	root := t.TempDir()
	builder := &recordingBuilder{}
	report, err := buildDeploymentImage(t.Context(), root, builder)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "third_party", "vpp-patches-v26.06"); builder.build.Context != want {
		t.Fatalf("build context = %q, want %q: the v25.10 PPPoE series must not be its input", builder.build.Context, want)
	}
	if builder.build.Tag != DeploymentImageName {
		t.Fatalf("build tag = %q, want %q, the tag the deployment run boots", builder.build.Tag, DeploymentImageName)
	}
	if !builder.build.Cache || !builder.build.HostCPUSpecific {
		t.Fatalf("a native-only VPP build must be cached per CPU: %#v", builder.build)
	}
	if report.Image.Reference != "sha256:fixture" {
		t.Fatalf("immutable image identity lost: %#v", report)
	}
	if builder.releases != 1 {
		t.Fatalf("released image %d times, want one", builder.releases)
	}
}

func TestVPPDeploymentBuildFailureIsReported(t *testing.T) {
	want := errors.New("patch rejected a hunk")
	builder := &recordingBuilder{buildError: want}
	if _, err := buildDeploymentImage(t.Context(), t.TempDir(), builder); !errors.Is(err, want) {
		t.Fatalf("build failure = %v, want %v", err, want)
	}
	if builder.releases != 0 {
		t.Fatal("a failed build released an image it never retained")
	}
}

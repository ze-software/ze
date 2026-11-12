// VALIDATES: preview compilation selects the full distro and an explicit target.
// PREVENTS: shipping a host driver, a reduced feature set or an unstamped binary.
package buildcompile

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestDaemonPlanCarriesPreviewIdentityAndStaticTarget(t *testing.T) {
	root := t.TempDir()
	tc := parityToolchain(root)
	tc.Version = "preview-0123456789ab-abcdef012345-dirty"
	for _, arch := range []string{"amd64", "arm64"} {
		output := filepath.Join(root, "artifacts", "ze-"+arch)
		plan := daemonPlan(tc, arch, output, "ze_core ze_distro ze_bgp ze_ssh")
		if plan.output != output || plan.environ.GOOS != "linux" || plan.environ.GOARCH != arch || plan.environ.CGO {
			t.Fatalf("wrong daemon target: %#v", plan)
		}
		for _, required := range []string{"-trimpath", "-buildvcs=false", "ze_core ze_distro ze_bgp ze_ssh", tc.LDFlags(), output, "./cmd/ze"} {
			if !slices.Contains(plan.command, required) {
				t.Fatalf("daemon command omits %q: %v", required, plan.command)
			}
		}
		if !strings.HasPrefix(tc.LDFlags(), "-X main.version=preview-") {
			t.Fatal("preview identity must reach the linker")
		}
	}
}

func TestPreviewEnvironmentRefusesAmbientOverlaysAndISATuning(t *testing.T) {
	got := previewEnvironment([]string{"PATH=/compiler", "GOFLAGS=-overlay=private.json", "GOAMD64=v4", "GOWORK=/private/go.work", "GOEXPERIMENT=rangefunc", "CGO_ENABLED=0"})
	for _, setting := range []string{"PATH=/compiler", "CGO_ENABLED=0", "GOFLAGS=-trimpath", "GOAMD64=v1", "GOWORK=off", "GOENV=off", "GOEXPERIMENT="} {
		if !slices.Contains(got, setting) {
			t.Fatalf("preview environment omits %s: %v", setting, got)
		}
	}
	for _, setting := range got {
		if strings.Contains(setting, "private") || setting == "GOAMD64=v4" {
			t.Fatalf("ambient input survived: %s", setting)
		}
	}
}

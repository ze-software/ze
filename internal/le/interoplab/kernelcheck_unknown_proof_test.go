package interoplab

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDockerKernelUnknownIsProbeFailure drives the Docker kernel consumer with
// an unreadable capability and verifies it cannot prescribe a kernel rebuild as
// though that probe failure established an absent feature.
func TestDockerKernelUnknownIsProbeFailure(t *testing.T) {
	for _, release := range []string{"7.2.9-generic", "7.2.9-ze"} {
		t.Run(release, func(t *testing.T) {
			named := filepath.Join(t.TempDir(), "named ze")
			if err := os.Symlink(kernelCheckZe, named); err != nil {
				t.Fatal(err)
			}
			answer := `{"ready":false,"capabilities":[{"subsystem":"xfrm-interface","kernel":"CONFIG_XFRM_INTERFACE","state":"unknown","reason":"netlink probe: operation not permitted"}]}`
			runner := scriptedKernel(answer, 1, nil)
			probe := runner.run
			runner.run = func(command ProcessCommand) (ProcessResult, error) {
				if strings.Contains(strings.Join(command.Arguments, " "), "{{.KernelVersion}}") {
					return ProcessResult{Stdout: release}, nil
				}
				return probe(command)
			}
			docker := newDocker(runner)
			docker.owner.goos = hostGOOSLinux
			err := checkDockerKernel(t.Context(), docker, named)
			if err == nil {
				t.Fatal("an unknown capability was accepted")
			}
			requireCommandOnItsOwnLine(t, err, "./le setup docker kernel check ze '"+named+"'")
			for _, want := range []string{"could not be checked", release, "xfrm-interface", "CONFIG_XFRM_INTERFACE", "unknown", "netlink probe: operation not permitted", "/ze doctor --json kernel-capabilities"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("probe failure %q does not name %q", err, want)
				}
			}
			for _, falseClaim := range []string{" lacks ", "runtime kernel lacks", "setup docker kernel install", "runtime.config", "reboot"} {
				if strings.Contains(err.Error(), falseClaim) {
					t.Errorf("unknown probe was attributed to missing kernel support: %v", err)
				}
			}
		})
	}
}

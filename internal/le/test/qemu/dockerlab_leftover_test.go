// Overview: dockerlab.go -- what the guest dockerd still holds once the lab ends
package testqemu

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGuestDocker answers the three listings the leftover report makes from
// the FAKE_* variables, and fails every call when FAKE_DOCKER_FAILS is set.
const fakeGuestDocker = `#!/bin/sh
[ -n "$FAKE_DOCKER_FAILS" ] && { echo 'Cannot connect to the Docker daemon' >&2; exit 1; }
case "$1" in
ps) [ -n "$FAKE_CONTAINERS" ] && printf '%s\n' $FAKE_CONTAINERS | sed 's/^/container /' ;;
network) [ -n "$FAKE_NETWORKS" ] && printf '%s\n' $FAKE_NETWORKS | sed 's/^/network /' ;;
images) [ -n "$FAKE_IMAGES" ] && printf '%s\n' $FAKE_IMAGES | sed 's/^/image /' ;;
esac
exit 0
`

// VALIDATES: AC-6 and AC-10 evidence. Once the host le's lab ends, normally,
// failed or interrupted, le lists what the guest dockerd still holds of the
// lab: containers and networks carrying the ze.lab label and the run-owned
// ze-interop-retained image tags. The DUT image kept under its cache tag is
// allowed to stay and is not listed. The listing says "unknown" rather than
// "none" when dockerd cannot answer, and exits 0 either way.
// METHOD: run dockerLabLeftoverListing under /bin/sh, as the guest's ssh
// session does, with a fake docker first on PATH.
// PREVENTS: an interrupted lab whose leftovers nobody sees before the VM takes
// them, and a dockerd failure reported as a clean guest.
func TestDockerLabReportsWhatTheGuestStillHolds(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(fakeGuestDocker), 0o700); err != nil { //nolint:gosec // an executable test fixture
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, tc := range []struct {
		name string
		env  []string
		want string
	}{
		{"clean guest", nil, "docker-lab: left behind: none\n"},
		{"leftovers", []string{"FAKE_CONTAINERS=ze-dut ze-peer", "FAKE_NETWORKS=ze-lab-net", "FAKE_IMAGES=ze-interop-retained:AB12"},
			"docker-lab: left behind: container ze-dut, container ze-peer, network ze-lab-net, image ze-interop-retained:AB12\n"},
		{"dockerd silent", []string{"FAKE_DOCKER_FAILS=1"}, "docker-lab: left behind: unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := exec.CommandContext(t.Context(), "/bin/sh", "-c", dockerLabLeftoverListing)
			line.Env = append(os.Environ(), tc.env...)
			output, err := line.Output()
			if err != nil {
				t.Errorf("the listing answered %v, want exit 0", err)
			}
			if !strings.HasPrefix(string(output), tc.want) {
				t.Errorf("report = %q, want it to start %q", output, tc.want)
			}
		})
	}
}

// Overview: dockerlab.go -- what the guest dockerd still holds once the lab ends
package testqemu

import (
	"errors"
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

// VALIDATES: AC-6 evidence. The lab line reports, on its way out, what the
// guest dockerd still holds of the lab: containers and networks carrying the
// ze.lab label and the run-owned ze-interop-retained image tags. The DUT image
// kept under its cache tag is allowed to stay and is not listed. The report
// runs on every exit, keeps the lab's exit status, and says "unknown" rather
// than "none" when dockerd cannot answer.
// METHOD: run dockerLabLeftoverTrap followed by `exit 3` under /bin/sh with a
// fake docker first on PATH.
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
		{"clean guest", nil, "interop guest: left behind: none\n"},
		{"leftovers", []string{"FAKE_CONTAINERS=ze-dut ze-peer", "FAKE_NETWORKS=ze-lab-net", "FAKE_IMAGES=ze-interop-retained:AB12"},
			"interop guest: left behind: container ze-dut, container ze-peer, network ze-lab-net, image ze-interop-retained:AB12\n"},
		{"dockerd silent", []string{"FAKE_DOCKER_FAILS=1"}, "interop guest: left behind: unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := exec.CommandContext(t.Context(), "/bin/sh", "-c", dockerLabLeftoverTrap()+"exit 3")
			line.Env = append(os.Environ(), tc.env...)
			output, err := line.Output()
			exit, ok := errors.AsType[*exec.ExitError](err)
			if !ok || exit.ExitCode() != 3 {
				t.Errorf("the line answered %v, want the lab's exit status 3", err)
			}
			if !strings.HasPrefix(string(output), tc.want) {
				t.Errorf("report = %q, want it to start %q", output, tc.want)
			}
		})
	}
}

// VALIDATES: the leftover report is armed right before the lab and only when a
// lab runs, so a failed dockerd start or kernel check reports nothing about a
// lab that never started.
func TestDockerLabArmsTheLeftoverReportBeforeTheLab(t *testing.T) {
	report := dockerLabLeftoverTrap()
	withLab := dockerLabCommand("arm64", []string{"perf", "run", "dut", "ze"}, nil)
	armed := strings.Index(withLab, report)
	check := strings.Index(withLab, " setup docker-kernel check ze ")
	lab := strings.Index(withLab, "'perf' 'run' 'dut' 'ze'")
	if armed < 0 || armed < check || armed > lab {
		t.Errorf("report armed at %d, want it between the check (%d) and the lab (%d):\n%s", armed, check, lab, withLab)
	}
	if strings.Contains(dockerLabCommand("arm64", nil, nil), report) {
		t.Error("the check-only line arms a leftover report for a lab it never runs")
	}
}

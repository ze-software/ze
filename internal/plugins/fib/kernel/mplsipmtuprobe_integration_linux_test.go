//go:build integration && linux

// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: mplstransitmtu_integration_linux_test.go -- the backend's own answer
// Related: internal/component/kernelcap/probe_linux.go -- MPLSIPMTU, the probe this drives

package fibkernel

import (
	"os"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/kernelcap"
)

// TestMPLSIntegration_ProbeAnswersFromAFreshNamespace runs the enrolled
// transit MTU probe from a network namespace whose label space is 0, which is
// what every fresh namespace holds and what a Docker container's probe sees.
//
// VALIDATES: the probe answers present on Ze's runtime kernel (release suffix
// "-ze", CONFIG_MPLS_IP_MTU=y) and absent on any other kernel with AF_MPLS,
// never unknown for want of a label space; and the caller's label space is
// still 0 afterwards, so the probe sized only its own throwaway namespace.
// PREVENTS: the Docker kernel check refusing every kernel with
// "mpls-transit-mtu: unknown: the MPLS label space holds no unreserved label".
func TestMPLSIntegration_ProbeAnswersFromAFreshNamespace(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		before := readLabelSpace(t)
		if before != "0" {
			t.Fatalf("a fresh namespace holds label space %q, want 0", before)
		}

		probe := kernelcap.MPLSIPMTU()
		t.Logf("kernel %s: mpls-transit-mtu %v (%v)", kernelRelease(t), probe.State, probe.Reason)

		if after := readLabelSpace(t); after != before {
			t.Errorf("the probe changed the caller's label space from %q to %q", before, after)
		}
		want := kernelcap.StateAbsent
		if strings.HasSuffix(kernelRelease(t), "-ze") {
			want = kernelcap.StatePresent
		}
		if probe.State != want {
			t.Fatalf("got %v (%v), want %v", probe.State, probe.Reason, want)
		}
	})
}

func readLabelSpace(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(mplsPlatformLabels)
	if err != nil {
		t.Fatalf("read the label space: %v", err)
	}
	return strings.TrimSpace(string(data))
}

func kernelRelease(t *testing.T) string {
	t.Helper()
	var name unix.Utsname
	if err := unix.Uname(&name); err != nil {
		t.Fatalf("uname: %v", err)
	}
	return unix.ByteSliceToString(name.Release[:])
}

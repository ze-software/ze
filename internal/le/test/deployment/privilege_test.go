// Design: docs/architecture/testing/interop.md -- the deployment proofs' containers
// Related: l2tp.go -- L2TP.containerArgs
// Related: vppiface.go -- vppIface.containerArgs
//
// No deployment proof container runs privileged
// (spec-lab-containers-least-privilege, AC-6): it holds what its daemons need.

package testdeployment

import (
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab/grantcheck"
)

// VALIDATES: AC-6. The L2TP proof's container, where xl2tpd and pppd open PPP
// units and ze programs L2TP, holds exactly NET_ADMIN and --device /dev/ppp:
// no --privileged and no other grant.
// PREVENTS: the container silently regaining --privileged, or losing the PPP
// device, whose absence looks like a protocol failure.
func TestL2TPDeploymentContainerRunsWithoutPrivilege(t *testing.T) {
	run := fixtureL2TP(t)
	evidence := filepath.Join(run.Tree, "tmp", "evidence", "l2tp-peer-1")
	argv := run.containerArgs("ze-l2tp-evidence-1", evidence)
	// The checkout the proof builds in and its evidence directory are the
	// only host paths it sees, both writable because the proof writes there.
	want := grantcheck.Grants{
		Capabilities: []string{"NET_ADMIN"},
		Devices:      []string{"/dev/ppp"},
		Mounts:       []string{run.Tree + ":/src", evidence + ":/run/l2tp"},
	}
	for _, problem := range grantcheck.Problems(grantcheck.Parse(nil, argv), want) {
		t.Errorf("the L2TP proof container %s: %v", problem, argv)
	}
}

// VALIDATES: AC-7. The VPP proofs' container (vpp-iface-test and vpp-test),
// where VPP locks its buffer memory and creates tap interfaces and ze programs
// the namespace, holds exactly NET_ADMIN, --device /dev/net/tun and
// --device /dev/vhost-net: no --privileged, no IPC_LOCK, no other grant. On an
// amd64 Linux host VPP started, created its tap and answered pings through it
// without IPC_LOCK (spec-lab-containers-least-privilege, owner decision 4).
// PREVENTS: the container silently regaining --privileged, or losing a grant
// VPP needs, whose absence looks like a ze defect.
func TestVPPDeploymentContainerRunsWithoutPrivilege(t *testing.T) {
	run := fixtureVPPIface(t)
	evidence := filepath.Join(run.Tree, "tmp", "evidence", "vpp-iface-1")
	argv := run.containerArgs("ze-vpp-iface-1", evidence)
	want := grantcheck.Grants{
		Capabilities: []string{"NET_ADMIN"},
		Devices:      []string{"/dev/net/tun", "/dev/vhost-net"},
		Mounts:       []string{run.Tree + ":/src", evidence + ":/run/vpp"},
	}
	for _, problem := range grantcheck.Problems(grantcheck.Parse(nil, argv), want) {
		t.Errorf("the VPP proof container %s: %v", problem, argv)
	}
}

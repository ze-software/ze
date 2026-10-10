// Design: docs/architecture/testing/interop.md -- the deployment proofs' containers
// Related: l2tp.go -- L2TP.containerArgs
// Related: vppiface.go -- vppIface.containerArgs
//
// No deployment proof container runs privileged
// (spec-lab-containers-least-privilege, AC-6): it holds what its daemons need.

package testdeployment

import (
	"path/filepath"
	"slices"
	"testing"
)

// flagValues answers every value that follows flag in argv.
func flagValues(argv []string, flag string) []string {
	var values []string
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] == flag {
			values = append(values, argv[i+1])
		}
	}
	return values
}

// VALIDATES: AC-6. The L2TP proof's container, where xl2tpd and pppd open PPP
// units and ze programs L2TP, holds NET_ADMIN and --device /dev/ppp, and runs
// no --privileged.
// PREVENTS: the container silently regaining --privileged, or losing the PPP
// device, whose absence looks like a protocol failure.
func TestL2TPDeploymentContainerRunsWithoutPrivilege(t *testing.T) {
	run := fixtureL2TP(t)
	argv := run.containerArgs("ze-l2tp-evidence-1", filepath.Join(run.Tree, "tmp", "evidence", "l2tp-peer-1"))
	if slices.Contains(argv, "--privileged") {
		t.Errorf("the container runs --privileged: %v", argv)
	}
	if !slices.Contains(flagValues(argv, "--cap-add"), "NET_ADMIN") {
		t.Errorf("the container lacks --cap-add NET_ADMIN: %v", argv)
	}
	if !slices.Contains(flagValues(argv, "--device"), "/dev/ppp") {
		t.Errorf("the container lacks --device /dev/ppp: %v", argv)
	}
}

// VALIDATES: AC-7. The VPP proofs' container (vpp-iface-test and vpp-test),
// where VPP locks its buffer memory and creates tap interfaces and ze programs
// the namespace, holds NET_ADMIN, --device /dev/net/tun and
// --device /dev/vhost-net, and runs no --privileged and no IPC_LOCK: on an
// amd64 Linux host VPP started, created its tap and answered pings through it
// without IPC_LOCK (spec-lab-containers-least-privilege, owner decision 4).
// PREVENTS: the container silently regaining --privileged, or losing a grant
// VPP needs, whose absence looks like a ze defect.
func TestVPPDeploymentContainerRunsWithoutPrivilege(t *testing.T) {
	run := fixtureVPPIface(t)
	argv := run.containerArgs("ze-vpp-iface-1", filepath.Join(run.Tree, "tmp", "evidence", "vpp-iface-1"))
	if slices.Contains(argv, "--privileged") {
		t.Errorf("the container runs --privileged: %v", argv)
	}
	if !slices.Contains(flagValues(argv, "--cap-add"), "NET_ADMIN") {
		t.Errorf("the container lacks --cap-add NET_ADMIN: %v", argv)
	}
	if slices.Contains(flagValues(argv, "--cap-add"), "IPC_LOCK") {
		t.Errorf("the container holds IPC_LOCK, which VPP was shown not to need: %v", argv)
	}
	for _, device := range []string{"/dev/net/tun", "/dev/vhost-net"} {
		if !slices.Contains(flagValues(argv, "--device"), device) {
			t.Errorf("the container lacks --device %s: %v", device, argv)
		}
	}
}

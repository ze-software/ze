// Design: docs/architecture/testing/interop.md -- the deployment proofs' containers
// Related: l2tp.go -- L2TP.containerArgs
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

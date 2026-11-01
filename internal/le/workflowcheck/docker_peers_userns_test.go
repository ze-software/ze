package workflowcheck

import (
	"strings"
	"testing"
)

// TestDockerPeersLiftsTheUsernsRestrictionBeforeItsTests pins the evidence
// nightly's docker-peers job to lifting the AppArmor restriction on
// unprivileged user namespaces before it runs its packages. The action runs
// each package's whole test set unprivileged, and the tftpserver socket tests
// among them run in a namespace of their own (internal/test/userns).
//
// VALIDATES: the job lifts the restriction, and does so before
// `./le test integration docker-peers`.
// PREVENTS: evidence-nightly run 41298258262, where three TFTP tests failed
// "userns: make the mount tree private: permission denied" because the job,
// unlike verify.yml and the integration job, left the restriction in place.
func TestDockerPeersLiftsTheUsernsRestrictionBeforeItsTests(t *testing.T) {
	const name = "evidence-nightly.yml"
	for _, job := range jobBlocks(t, name) {
		if job.name != "docker-peers" {
			continue
		}
		body := stripComments(job.body)
		tests := strings.Index(body, "./le test integration docker-peers")
		if tests < 0 {
			t.Fatalf("%s job docker-peers no longer runs ./le test integration docker-peers", name)
		}
		lift := strings.Index(body, "sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0")
		if lift < 0 {
			t.Fatalf("%s job docker-peers does not lift the AppArmor user namespace restriction", name)
		}
		if lift > tests {
			t.Fatalf("%s job docker-peers lifts the AppArmor user namespace restriction after its tests run", name)
		}
		return
	}
	t.Fatalf("%s has no docker-peers job", name)
}

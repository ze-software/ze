// Design: docs/architecture/ike/ipsec-10-cli-diag.md -- IPsec health check
// Related: health_drift.go -- the kernel-versus-belief drift signal this check folds in

package engine

import "github.com/ze-software/ze/internal/core/health"

// RegisterHealthCheck registers the IPsec health check with the default registry.
func RegisterHealthCheck() {
	health.Register("ipsec", checkIPsecHealth)
}

func checkIPsecHealth() (health.Status, string) {
	table := ActiveTable()
	if table == nil {
		return health.StatusDown, "ike engine not running"
	}

	peers := ActivePeers()
	if len(peers) == 0 {
		return health.StatusHealthy, "no peers configured"
	}

	sas := table.All()
	established := 0
	for _, sa := range sas {
		if sa.State == StateEstablished {
			established++
		}
	}

	if established == 0 {
		return health.StatusDegraded, "no established IKE SAs"
	}

	if established < len(peers) {
		return health.StatusDegraded, "some peers not established"
	}

	// Every check above reads what the IKE engine BELIEVES. A tunnel whose Child
	// SA the kernel has expired, flushed, or never accepted passes all of them,
	// because the engine's own records still say the install call succeeded. The
	// kernel is the only source that can contradict them.
	//
	// A dataplane that cannot be read is NOT drift. It is a question that was not
	// asked, and reporting healthy on it would be the same false green in a new
	// place (ai/rules/evidence.md).
	if drifting, known := driftingPeers(); known && len(drifting) > 0 {
		return health.StatusDegraded, driftDetail(drifting)
	}

	return health.StatusHealthy, ""
}

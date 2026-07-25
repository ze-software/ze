// Design: plan/learned/745-ipsec-10-cli-diag.md -- IPsec health check

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

	return health.StatusHealthy, ""
}

// Design: plan/learned/745-ipsec-10-cli-diag.md -- IPsec Prometheus metrics

package engine

import "github.com/ze-software/ze/internal/core/metrics"

// IPsecMetrics holds Prometheus metric handles for the IPsec subsystem.
type IPsecMetrics struct {
	saCount        metrics.Gauge
	tunnelUp       metrics.GaugeVec
	tunnelDegraded metrics.GaugeVec
	rekeyTotal     metrics.GaugeVec
}

// RegisterMetrics creates IPsec metrics on the given registry.
//
// A tunnel counts as up only when its IKE SA is established AND its Child SA is
// installed in the dataplane. An established SA whose ESP install was refused reads
// up 0 and degraded 1. The two gauges therefore separate "no session" from "session
// but no encrypted traffic", which the up gauge alone reported as healthy.
func RegisterMetrics(reg metrics.Registry) *IPsecMetrics {
	return &IPsecMetrics{
		saCount:        reg.Gauge("ze_ipsec_sa_count", "Number of active IKE Security Associations"),
		tunnelUp:       reg.GaugeVec("ze_ipsec_tunnel_up", "Whether a peer tunnel is established and carries ESP (1=up, 0=down or degraded)", []string{"peer"}),
		tunnelDegraded: reg.GaugeVec("ze_ipsec_tunnel_degraded", "Whether a peer IKE SA is established but its Child SA carries no ESP (1=degraded)", []string{"peer"}),
		rekeyTotal:     reg.GaugeVec("ze_ipsec_rekey_total", "Cumulative child SA rekey count per peer", []string{"peer"}),
	}
}

// espInstalled reports whether this peer's current Child SA is in the dataplane.
//
// It fails closed. An unknown peer, a session without a Child SA, and a Child SA
// whose install was refused all read false (ai/rules/fail-closed-guards.md).
func espInstalled(peers map[string]*PeerSession, name string) bool {
	ps, ok := peers[name]
	if !ok || ps == nil {
		return false
	}
	child := ps.getChildSA()
	return child != nil && child.ESPInstalled
}

// Update reads the current SA table and peer session state to refresh all metrics.
func (m *IPsecMetrics) Update() {
	table := ActiveTable()
	if table == nil {
		m.saCount.Set(0)
		return
	}

	sas := table.All()
	m.saCount.Set(float64(len(sas)))

	infos := PeerInfoMap()
	peers := ActivePeers()
	for name := range infos {
		info := infos[name]
		established := false
		for _, sa := range sas {
			if sa.PeerName == name && sa.State == StateEstablished {
				established = true
				break
			}
		}

		// An established IKE SA whose Child SA is not in the dataplane forwards no
		// encrypted traffic. It is degraded, and it is not up.
		carriesESP := established && espInstalled(peers, name)
		up, degraded := float64(0), float64(0)
		if carriesESP {
			up = 1
		} else if established {
			degraded = 1
		}

		m.tunnelUp.With(name).Set(up)
		m.tunnelDegraded.With(name).Set(degraded)
		m.rekeyTotal.With(name).Set(float64(info.RekeyCount))
	}
}

// Design: plan/learned/745-ipsec-10-cli-diag.md -- IPsec Prometheus metrics

package engine

import "github.com/ze-software/ze/internal/core/metrics"

// IPsecMetrics holds Prometheus metric handles for the IPsec subsystem.
type IPsecMetrics struct {
	saCount    metrics.Gauge
	tunnelUp   metrics.GaugeVec
	rekeyTotal metrics.GaugeVec
}

// RegisterMetrics creates IPsec metrics on the given registry.
func RegisterMetrics(reg metrics.Registry) *IPsecMetrics {
	return &IPsecMetrics{
		saCount:    reg.Gauge("ze_ipsec_sa_count", "Number of active IKE Security Associations"),
		tunnelUp:   reg.GaugeVec("ze_ipsec_tunnel_up", "Whether a peer tunnel is established (1=up, 0=down)", []string{"peer"}),
		rekeyTotal: reg.GaugeVec("ze_ipsec_rekey_total", "Cumulative child SA rekey count per peer", []string{"peer"}),
	}
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
	for name := range infos {
		info := infos[name]
		up := float64(0)
		for _, sa := range sas {
			if sa.PeerName == name && sa.State == StateEstablished {
				up = 1
				break
			}
		}
		m.tunnelUp.With(name).Set(up)
		m.rekeyTotal.With(name).Set(float64(info.RekeyCount))
	}
}

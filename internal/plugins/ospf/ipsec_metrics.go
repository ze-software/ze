// Design: plan/learned/1038-ospf-ext-16-ipsec-auth.md -- RFC 4552 IPsec metrics (IPv6 family).
// Related: ipsec_install.go -- the installer that sets these series.
//
// These extend the OSPFv3 IPv6-family metric set with the ze_ospfv3_ipsec_* prefix
// (the IPv6 family's wire name). They are owned by this feature: removing the IPsec
// installer removes them.

package ospf

import "github.com/ze-software/ze/internal/core/metrics"

type ipsecMetrics struct {
	sas         metrics.GaugeVec
	policies    metrics.GaugeVec
	failures    metrics.CounterVec
	kernelDrops metrics.CounterVec
}

func newIPsecMetrics(reg metrics.Registry) *ipsecMetrics {
	if reg == nil {
		reg = metrics.NopRegistry{}
	}
	return &ipsecMetrics{
		sas: reg.GaugeVec(
			"ze_ospfv3_ipsec_sas",
			"Installed OSPFv3 IPsec Security Associations, by interface, protocol, and direction.",
			[]string{"interface", "protocol", "direction"},
		),
		policies: reg.GaugeVec(
			"ze_ospfv3_ipsec_policies",
			"Installed OSPFv3 IPsec security policies, by interface and direction.",
			[]string{"interface", "direction"},
		),
		failures: reg.CounterVec(
			"ze_ospfv3_ipsec_install_failures_total",
			"Total OSPFv3 IPsec install failures, by interface and reason.",
			[]string{"interface", "reason"},
		),
		kernelDrops: reg.CounterVec(
			"ze_ospfv3_ipsec_kernel_drops_total",
			"Total kernel XFRM inbound drops attributable to OSPFv3 IPsec, by interface (empty=node-global XFRM stat) and reason.",
			[]string{"interface", "reason"},
		),
	}
}

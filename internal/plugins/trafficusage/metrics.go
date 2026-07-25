// Design: plan/learned/977-traffic-usage.md -- traffic-usage Prometheus metric families & helpers

package trafficusage

import (
	"net/netip"
	"strconv"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/metrics"
)

// usageMetrics holds the metric vectors. Absolute byte totals are GaugeVecs (the
// Counter interface has no Set); per-poll the monitor calls With(labels...).Set
// and Delete(labels...) for stale series. Per-IP families are published only
// when track-ip is enabled. Every family is in the ze_traffic_usage_* namespace.
type usageMetrics struct {
	ingressBytes     metrics.GaugeVec // labels: interface, src_ip
	egressBytes      metrics.GaugeVec // labels: interface, dst_ip
	ingressPortBytes metrics.GaugeVec // labels: interface, dst_port, protocol
	egressPortBytes  metrics.GaugeVec // labels: interface, src_port, protocol
	mapEntries       metrics.GaugeVec // labels: interface, map
}

var metricsPtr atomic.Pointer[usageMetrics]

// BindMetrics registers the ze_traffic_usage_* families on the registry. It is
// idempotent: registration by name returns the existing vector, so a config
// reload may call it again safely. A nil registry is a no-op.
func BindMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	m := &usageMetrics{
		ingressBytes:     reg.GaugeVec("ze_traffic_usage_ingress_bytes_total", "Ingress bytes per source IPv4 (requires track-ip)", []string{"interface", "src_ip"}),
		egressBytes:      reg.GaugeVec("ze_traffic_usage_egress_bytes_total", "Egress bytes per destination IPv4 (requires track-ip)", []string{"interface", "dst_ip"}),
		ingressPortBytes: reg.GaugeVec("ze_traffic_usage_ingress_port_bytes_total", "Ingress bytes per destination port and protocol", []string{"interface", "dst_port", "protocol"}),
		egressPortBytes:  reg.GaugeVec("ze_traffic_usage_egress_port_bytes_total", "Egress bytes per source port and protocol", []string{"interface", "src_port", "protocol"}),
		mapEntries:       reg.GaugeVec("ze_traffic_usage_map_entries", "Live entry count per BPF map", []string{"interface", "map"}),
	}
	metricsPtr.Store(m)
}

// protoName maps an IP protocol number to a metric label, matching the upstream
// lan-bandwidth-exporter. Unknown protocols render as their decimal number.
func protoName(p uint8) string {
	switch p {
	case 1:
		return "icmp"
	case 6:
		return "tcp"
	case 17:
		return "udp"
	case 47:
		return "gre"
	case 50:
		return "esp"
	case 51:
		return "ah"
	case 58:
		return "icmpv6"
	default:
		return strconv.Itoa(int(p))
	}
}

// ipString renders a BPF map IPv4 key (the raw header bytes read as a host-order
// uint32) back to dotted-quad. On a little-endian host the low byte is the first
// address octet, matching the upstream LittleEndian decode.
func ipString(raw uint32) string {
	return netip.AddrFrom4([4]byte{byte(raw), byte(raw >> 8), byte(raw >> 16), byte(raw >> 24)}).String()
}

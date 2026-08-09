// Design: docs/architecture/ospf/ospfv3-3-ipv6-transport.md -- OSPFv3 transport metrics

package transport

import "github.com/ze-software/ze/internal/core/metrics"

// transportMetrics holds the OSPF transport Prometheus series. OSPFv3 is "our
// OSPF" for the operator, so it REUSES the OSPFv2 ze_ospf_ series rather than a
// distinct ze_ospfv3_ namespace: the metrics registry is get-or-create by name,
// so when both transports run on a node they share the same series (the per-
// series help/labels match the OSPFv2 transport's exactly so the shared
// registration is consistent regardless of which plugin starts first). The
// interface label distinguishes v2 from v3 traffic. NOTE: ze_ospf_sockets_open is
// a no-label gauge set per transport; sharing it across v2 and v3 needs Inc/Dec
// or a family label to avoid clobbering on a dual-stack node -- deferred to
// ospfv3-4, which wires the production registry (v3 SetMetrics is unused until
// then).
type transportMetrics struct {
	packetsSent     metrics.CounterVec
	packetsReceived metrics.CounterVec
	packetsDropped  metrics.CounterVec
	socketsOpen     metrics.Gauge
}

func newTransportMetrics(reg metrics.Registry) *transportMetrics {
	return &transportMetrics{
		packetsSent: reg.CounterVec(
			"ze_ospf_packets_sent_total",
			"Total OSPF packets transmitted, by interface and packet type.",
			[]string{"interface", "type"},
		),
		packetsReceived: reg.CounterVec(
			"ze_ospf_packets_received_total",
			"Total OSPF packets received, by interface and packet type.",
			[]string{"interface", "type"},
		),
		packetsDropped: reg.CounterVec(
			"ze_ospf_packets_dropped_total",
			"Total OSPF packets dropped, by interface and reason.",
			[]string{"interface", "reason"},
		),
		socketsOpen: reg.Gauge(
			"ze_ospf_sockets_open",
			"Current number of open OSPF raw sockets.",
		),
	}
}

func nopTransportMetrics() *transportMetrics {
	return newTransportMetrics(metrics.NopRegistry{})
}

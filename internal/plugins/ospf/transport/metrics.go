// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- OSPF transport metrics

package transport

import "github.com/ze-software/ze/internal/core/metrics"

// labelInterface is the metric label every per-interface transport series carries.
const labelInterface = "interface"

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
			[]string{labelInterface, "type"},
		),
		packetsReceived: reg.CounterVec(
			"ze_ospf_packets_received_total",
			"Total OSPF packets received, by interface and packet type.",
			[]string{labelInterface, "type"},
		),
		packetsDropped: reg.CounterVec(
			"ze_ospf_packets_dropped_total",
			"Total OSPF packets dropped, by interface and reason.",
			[]string{labelInterface, "reason"},
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

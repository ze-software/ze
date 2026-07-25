// Design: plan/learned/957-ospf-3-ip-transport.md -- OSPF transport metrics

package transport

import "github.com/ze-software/ze/internal/core/metrics"

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

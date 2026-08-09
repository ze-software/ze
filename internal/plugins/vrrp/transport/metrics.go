// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- transport-owned Prometheus metrics
//
// The transport OWNS the five ze_vrrp_* series and is their single writer: the
// codec-validation reasons the engine (spec-vrrp-5) discovers are fed back through
// Transport.RecordRxError so packet_errors has ONE owner and no dead counters
// (holo bug 9). Names and labels are exactly the spec Metrics table. Registration
// is on the injected registry via SetMetrics; a NopRegistry backs the counters
// until then and in unit tests. Per-instance increment plumbing lives in
// transport.go (instanceCounters).

package transport

import "github.com/ze-software/ze/internal/core/metrics"

// transportMetrics holds the five transport-owned series. It is swapped
// atomically by SetMetrics, so per-instance counters read it through an
// atomic.Pointer and never capture a stale registry.
type transportMetrics struct {
	advertsSent       metrics.CounterVec // ze_vrrp_adverts_sent_total{interface,vrid,family}
	advertsReceived   metrics.CounterVec // ze_vrrp_adverts_received_total{interface,vrid,family}
	packetErrors      metrics.CounterVec // ze_vrrp_packet_errors_total{interface,vrid,family,reason}
	announcementsSent metrics.CounterVec // ze_vrrp_announcements_sent_total{interface,vrid,family,kind}
	socketsOpen       metrics.Gauge      // ze_vrrp_sockets_open
}

// newTransportMetrics registers the five series on reg. Exact names/labels come
// from the spec Metrics table.
func newTransportMetrics(reg metrics.Registry) *transportMetrics {
	return &transportMetrics{
		advertsSent: reg.CounterVec(
			"ze_vrrp_adverts_sent_total",
			"Total VRRP advertisements transmitted, by interface, vrid and family.",
			[]string{"interface", "vrid", "family"},
		),
		advertsReceived: reg.CounterVec(
			"ze_vrrp_adverts_received_total",
			"Total VRRP advertisement datagrams delivered to the engine, by interface, vrid and family.",
			[]string{"interface", "vrid", "family"},
		),
		packetErrors: reg.CounterVec(
			"ze_vrrp_packet_errors_total",
			"Total VRRP transport and receive-validation errors, by interface, vrid, family and reason.",
			[]string{"interface", "vrid", "family", "reason"},
		),
		announcementsSent: reg.CounterVec(
			"ze_vrrp_announcements_sent_total",
			"Total VRRP gratuitous-ARP / unsolicited-NA frames sent, by interface, vrid, family and kind.",
			[]string{"interface", "vrid", "family", "kind"},
		),
		socketsOpen: reg.Gauge(
			"ze_vrrp_sockets_open",
			"Current number of open VRRP transport instances (each holds a raw-socket set).",
		),
	}
}

// nopTransportMetrics backs the counters with the no-op registry, used before
// SetMetrics wires a real Prometheus registry (and in unit tests).
func nopTransportMetrics() *transportMetrics {
	return newTransportMetrics(metrics.NopRegistry{})
}

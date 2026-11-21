// Design: docs/architecture/l2tp/bng-5-pppoe.md -- counted IPv6 control drops
// Related: ipv6_wire.go -- finite protocol-discard reasons
package vpp

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
)

type ipv6Metrics struct{ discards metrics.CounterVec }

var ipv6MetricsCurrent atomic.Pointer[ipv6Metrics]

func init() { registry.InjectPluginMetrics("pppoe-vpp-ipv6", bindIPv6Metrics) }

func bindIPv6Metrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	ipv6MetricsCurrent.Store(&ipv6Metrics{discards: reg.CounterVec("ze_pppoe_vpp_ipv6_discards_total",
		"Punted PPP IPv6 controls discarded by bounded envelope, checksum, source, destination or queue checks.", []string{"reason"})})
}

func (s *controlSocket) countIPv6Discard(reason string) {
	s.ipv6Discards.Add(1)
	if current := ipv6MetricsCurrent.Load(); current != nil {
		current.discards.With(reason).Inc()
	}
}

func (s *controlSocket) ipv6DiscardCount() uint64 { return s.ipv6Discards.Load() }

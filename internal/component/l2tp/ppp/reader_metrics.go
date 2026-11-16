// Design: docs/research/l2tpv2-ze-integration.md -- IPv6 carrier receive errors
// Related: metrics.go -- pppMetrics.readerErrorsTotal, registered cross-platform
// Related: ra_carrier.go -- rsReaderLoop
// Related: dhcpv6_carrier.go -- dhcpv6ServerLoop

package ppp

// Carrier workers compile with the protocol engine. Only the carrier's socket
// factory is platform-specific, so the receive-error labels and counter share
// the workers' platform-independent placement.
const (
	loopRA     = "ra"
	loopDHCPv6 = "dhcpv6"
)

// countReaderError counts a swallowed receive failure from one of the two
// carrier workers. Without a bound metrics registry it remains a no-op, as
// countIdentifierRefusal does for the rest of the PPP engine.
func countReaderError(loop string) {
	m := pppMetricsPtr.Load()
	if m == nil {
		return
	}
	m.readerErrorsTotal.With(loop).Inc()
}

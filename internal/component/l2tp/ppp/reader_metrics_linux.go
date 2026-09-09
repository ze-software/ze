// Design: docs/research/l2tpv2-ze-integration.md -- receiver goroutines and a failing socket
// Related: metrics.go -- pppMetrics.readerErrorsTotal, registered cross-platform
// Related: ra_linux.go -- rsReaderLoop: the loopRA call site
// Related: dhcpv6_linux.go -- dhcpv6ServerLoop: the loopDHCPv6 call site

//go:build linux

package ppp

// Reader-loop names for ze_ppp_reader_errors_total. The set is closed and
// kept small, the same discipline as the reason constants in metrics.go:
// rsReaderLoop (ra_linux.go) and dhcpv6ServerLoop (dhcpv6_linux.go) are the
// two socket readers this package owns, and each names itself on every
// swallowed read error so the two loops share one counter rather than each
// declaring its own.
//
// Linux-only file: both call sites exist only under //go:build linux (no
// non-Linux equivalent of either reader is ever started, see
// ipv6_service_other.go), so these names and the helper below have no
// caller on a non-Linux build. Declaring them cross-platform in metrics.go
// left them permanently unused on every non-Linux lint flavor.
const (
	loopRA     = "ra"
	loopDHCPv6 = "dhcpv6"
)

// countReaderError increments the swallowed-read-error counter for loop
// (loopRA or loopDHCPv6). It is a no-op while no registry is bound, the
// same as countIdentifierRefusal (metrics.go).
func countReaderError(loop string) {
	m := pppMetricsPtr.Load()
	if m == nil {
		return
	}
	m.readerErrorsTotal.With(loop).Inc()
}

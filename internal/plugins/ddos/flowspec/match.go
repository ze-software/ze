// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- vector to FlowSpec match

package flowspec

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/ddosevent"
)

type flowspecMatch struct {
	DstPrefix netip.Prefix
	Proto     uint8
	DstPort   uint16
	SrcPort   uint16
	TCPFlags  uint8
}

func buildMatch(v ddosevent.VectorTuple) flowspecMatch {
	return flowspecMatch{
		DstPrefix: v.DstPrefix,
		Proto:     v.Proto,
		DstPort:   v.DstPort,
		SrcPort:   v.SrcPort,
		TCPFlags:  v.TCPFlags,
	}
}

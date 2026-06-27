// Design: plan/spec-cp-survival-5-detect-3-flowspec-responder.md -- vector to FlowSpec match

package ddosflowspec

import (
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/core/ddosevent"
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

func shouldAnnounce(v ddosevent.VectorTuple, allowlist []netip.Prefix) bool {
	if !v.DstPrefix.IsValid() {
		return false
	}
	for _, allow := range allowlist {
		if allow.Overlaps(v.DstPrefix) {
			return false
		}
	}
	return true
}

// Design: docs/research/l2tpv2-ze-integration.md -- IPv6 control receive validity
// RFC 4861 Section 6.1.1 -- see rfc/short/rfc4861.md

package ppp

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/ndp"
)

// ValidRouterSolicitation checks the body and IP envelope of a solicitation.
// The carrier MUST first verify the ICMPv6 checksum. Linux's ICMPv6 receive
// path does so; a carrier reading full PPP IPv6 frames owes that check itself.
// A source may be any IPv6 unicast address or the unspecified address.
//
// RFC 4861 Section 6.1.1: "The contents of the Reserved field, and of any
// unrecognized options, MUST be ignored." Defined options not used by a
// solicitation are likewise ignored after their lengths have been checked.
func ValidRouterSolicitation(payload []byte, source netip.Addr, hopLimit int) bool {
	if hopLimit != ndp.MessageHopLimit {
		return false
	}
	if !source.Is6() {
		return false
	}
	if source.IsMulticast() {
		return false
	}
	if len(payload) < 8 {
		return false
	}
	if payload[0] != ndp.ICMPv6TypeRouterSolicitation {
		return false
	}
	if payload[1] != 0 {
		return false
	}
	// The input message bounds this option walk; each accepted length advances.
	for off := 8; off < len(payload); {
		if len(payload)-off < 2 {
			return false
		}
		length := int(payload[off+1]) * 8
		if length == 0 {
			return false
		}
		if length > len(payload)-off {
			return false
		}
		if source.IsUnspecified() {
			if payload[off] == ndp.OptSourceLinkLayerAddress {
				return false
			}
		}
		off += length
	}
	return true
}

// Design: docs/architecture/wire/messages.md — BGP message types
// RFC: rfc/short/rfc4724.md — end-of-RIB marker (graceful restart)
// Related: update.go — UPDATE message wire representation

package message

import (
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// BuildEOR creates an End-of-RIB marker UPDATE for the given address family.
//
// RFC 4724 Section 2 - End-of-RIB Marker:
// "An UPDATE message with no reachable Network Layer Reachability Information
// (NLRI) and empty withdrawn NLRI is specified as the End-of-RIB marker that
// can be used by a BGP speaker to indicate to its peer the completion of the
// initial routing update after the session is established."
//
// For IPv4 unicast (AFI=1, SAFI=1): Empty UPDATE (no attributes, no NLRI).
// For other families: UPDATE with MP_UNREACH_NLRI containing only AFI/SAFI.
func BuildEOR(fam family.Family) *Update {
	// RFC 4724: IPv4 unicast uses empty UPDATE as EOR
	if fam.AFI == 1 && fam.SAFI == 1 {
		return &Update{}
	}

	// RFC 4724/4760: Other families use MP_UNREACH_NLRI with AFI/SAFI only
	// MP_UNREACH_NLRI format: AFI(2) + SAFI(1) + Withdrawn NLRI (empty for EOR)
	// Header (4 bytes with extended length) + Value (3 bytes) = 7 bytes total
	attrBytes := make([]byte, 7)
	attribute.WriteHeaderTo(attrBytes, 0,
		attribute.FlagOptional|attribute.FlagExtLength,
		attribute.AttrMPUnreachNLRI,
		3, // AFI(2) + SAFI(1)
	)
	attrBytes[4] = byte(fam.AFI >> 8)
	attrBytes[5] = byte(fam.AFI)
	attrBytes[6] = byte(fam.SAFI)

	return &Update{
		PathAttributes: attrBytes,
	}
}

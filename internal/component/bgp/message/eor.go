// Design: docs/architecture/wire/messages.md — BGP message types
// RFC: rfc/short/rfc4724.md — end-of-RIB marker (graceful restart)
// Related: update.go — UPDATE message wire representation

package message

import (
	"encoding/binary"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// IsEndOfRIBAnyFamily reports whether the UPDATE is an End-of-RIB marker for any
// address family (RFC 4724 §2, RFC 4760 §5): either the IPv4-unicast empty
// UPDATE (see IsEndOfRIB) or a multiprotocol marker whose only path attribute is
// an MP_UNREACH_NLRI carrying just AFI/SAFI with no withdrawn NLRI.
//
// EOR markers are graceful-restart control signals, not routes. Egress route
// filters (which strip a family's NLRI) must pass them through unchanged rather
// than suppress them, so the per-family EoR ordering of RFC 4724 still reaches
// the peer even when its routes are filtered.
func (u *Update) IsEndOfRIBAnyFamily() bool {
	if u.IsEndOfRIB() {
		return true
	}
	// A multiprotocol EoR carries no legacy reachability and exactly one
	// MP_UNREACH_NLRI attribute holding only AFI(2)+SAFI(1) (no withdrawn NLRI).
	if len(u.WithdrawnRoutes) != 0 || len(u.NLRI) != 0 {
		return false
	}
	attrs := u.PathAttributes
	off := 0
	sawEOR := false
	for off < len(attrs) {
		if off+2 > len(attrs) {
			return false
		}
		flags := attrs[off]
		code := attrs[off+1]
		var hdrLen, dataLen int
		if flags&byte(attribute.FlagExtLength) != 0 {
			if off+4 > len(attrs) {
				return false
			}
			dataLen = int(binary.BigEndian.Uint16(attrs[off+2 : off+4]))
			hdrLen = 4
		} else {
			if off+3 > len(attrs) {
				return false
			}
			dataLen = int(attrs[off+2])
			hdrLen = 3
		}
		if off+hdrLen+dataLen > len(attrs) {
			return false
		}
		// Any non-MP_UNREACH attribute, or an MP_UNREACH carrying more than
		// AFI/SAFI, means the UPDATE conveys real (un)reachability — not an EoR.
		if code != byte(attribute.AttrMPUnreachNLRI) || dataLen != 3 {
			return false
		}
		sawEOR = true
		off += hdrLen + dataLen
	}
	return sawEOR
}

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

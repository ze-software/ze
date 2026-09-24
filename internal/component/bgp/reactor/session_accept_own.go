// Design: docs/architecture/route-selection.md
// RFC: rfc/short/rfc7611.md -- Section 2.2
package reactor

import (
	"encoding/binary"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

// discardNonVPNAcceptOwn enforces the family boundary before the attribute index
// freezes. It changes no route eligibility: ACCEPT_OWN is not an AS_PATH bypass,
// and the ordinary originator/next-hop loop checks remain enabled.
func discardNonVPNAcceptOwn(wu *wireu.WireUpdate) *wireu.WireUpdate {
	sections, err := wire.ParseUpdateSections(wu.Payload())
	if err != nil {
		return wu
	}
	attrs := sections.Attrs(wu.Payload())
	hdr, flags, values, found := attribute.AttrFind(attrs, attribute.AttrCommunity)
	if !found {
		return wu
	}
	nonRD := len(sections.NLRI(wu.Payload())) != 0
	if _, _, mp, present := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI); present && len(mp) >= 3 {
		switch family.SAFI(mp[2]) {
		case family.SAFIVPN, family.SAFIMVPN, family.SAFIVPLS, family.SAFIEVPN,
			family.SAFIBGPLinkStateVPN, family.SAFIFlowSpecVPN, family.SAFIMUP:
		default:
			nonRD = true
		}
	}
	if !nonRD {
		return wu
	}
	removed := 0
	for off := 0; off+4 <= len(values); off += 4 {
		if binary.BigEndian.Uint32(values[off:]) == uint32(attribute.CommunityAcceptOwn) {
			removed += 4
		}
	}
	if removed == 0 {
		return wu
	}
	header := 3
	if flags&attribute.FlagExtLength != 0 {
		header = 4
	}
	kept := len(values) - removed
	rewritten := make([]byte, 0, len(attrs)-removed)
	rewritten = append(rewritten, attrs[:hdr]...)
	if kept != 0 {
		rewritten = append(rewritten, attrs[hdr:hdr+header]...)
		if header == 4 {
			binary.BigEndian.PutUint16(rewritten[hdr+2:], uint16(kept))
		} else {
			rewritten[hdr+2] = byte(kept)
		}
		for off := 0; off+4 <= len(values); off += 4 {
			if binary.BigEndian.Uint32(values[off:]) != uint32(attribute.CommunityAcceptOwn) {
				rewritten = append(rewritten, values[off:off+4]...)
			}
		}
	}
	rewritten = append(rewritten, attrs[hdr+header+len(values):]...)
	result := wireu.NewWireUpdate(message.RebuildUpdateBody(wu.Payload(), rewritten), wu.SourceCtxID())
	result.SetSourceID(wu.SourceID())
	result.SetMessageID(wu.MessageID())
	return result
}

// Design: docs/architecture/bgp/structural-forwarding.md -- receive validation before export
// RFC: rfc/short/draft-ietf-sidrops-aspa-verification.md -- Section 5.7 retained ineligible routes
// Related: reactor_api_forward.go -- common cached and stored-route egress
package reactor

import (
	"encoding/binary"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// forwardUpdateCore serializes selection and dispatch for one source. A validation
// replay queued after a change therefore follows any forward that read the old
// verdict. Both calls use the destination FIFO, including the reactor fast path
// whenever receive validation is enabled.
func (a *reactorAPIAdapter) forwardUpdateCore(update *ReceivedUpdate, updateID uint64, matchingPeers []*Peer, srcInfo forwardSourceInfo) error {
	a.r.aigp.dispatch.Lock()
	defer a.r.aigp.dispatch.Unlock()
	return a.forwardUpdateValidated(update, updateID, matchingPeers, srcInfo)
}

func (a *reactorAPIAdapter) forwardUpdateValidated(update *ReceivedUpdate, updateID uint64, matchingPeers []*Peer, srcInfo forwardSourceInfo) error {
	if peer := update.receivedPeer; peer != nil {
		if peer != srcInfo.peer || !forwardSourceCurrent(peer, update.receivedGeneration) {
			return errForwardNoSource
		}
	}
	if !ribevents.ValidationEnabled() && !flowSpecUpdate(update.WireUpdate) {
		return a.forwardUpdateSelected(update, updateID, matchingPeers, srcInfo, update.WireUpdate)
	}
	if srcInfo.peer != nil {
		srcInfo.peer.validationForwardMu.Lock()
		defer srcInfo.peer.validationForwardMu.Unlock()
	}
	selected, withdrawn, err := forwardValidationWire(update)
	if err != nil {
		return err
	}
	dispatched := false
	if withdrawn != nil {
		err := a.forwardUpdateSelected(update, updateID, matchingPeers, srcInfo, withdrawn)
		if err != nil && !errors.Is(err, errAllDestinationsSuppressed) {
			return err
		}
		dispatched = err == nil
	}
	if selected != nil {
		err := a.forwardUpdateSelected(update, updateID, matchingPeers, srcInfo, selected)
		if err != nil && !errors.Is(err, errAllDestinationsSuppressed) {
			return err
		}
		dispatched = dispatched || err == nil
	}
	if dispatched {
		return nil
	}
	return errAllDestinationsSuppressed
}

// forwardValidationWire partitions individual paths without changing the cache's
// received bytes. Output buffers MUST be adopted by update; cache eviction MUST
// return them after all destination workers release their references.
//
// Draft ASPA verification -28 Section 5.7: "If the AS_PATH is determined to be
// Invalid, then the route SHOULD be considered ineligible for route selection
// (see Section 3) and MUST be kept in the Adj-RIB-In for potential future
// re-evaluation (see [RFC9324])."
func forwardValidationWire(update *ReceivedUpdate) (*wireu.WireUpdate, *wireu.WireUpdate, error) {
	wu := update.WireUpdate
	if !ribevents.ValidationEnabled() && !flowSpecUpdate(wu) {
		return wu, nil, nil
	}
	if _, eor := wu.IsEOR(); eor {
		return wu, nil, nil
	}
	payload := wu.Payload()
	withdrawn, err := wu.Withdrawn()
	if err != nil {
		return nil, nil, err
	}
	announced, err := wu.NLRI()
	if err != nil {
		return nil, nil, err
	}
	attrStart := 4 + len(withdrawn)
	if attrStart > len(payload) {
		return nil, nil, wireu.ErrUpdateTruncated
	}
	attrEnd := attrStart + int(binary.BigEndian.Uint16(payload[attrStart-2:attrStart]))
	if attrEnd > len(payload) {
		return nil, nil, wireu.ErrUpdateTruncated
	}
	var spanBuf [16]relayAttrSpan
	spans, ok := scanAttrBlock(spanBuf[:0], payload[attrStart:attrEnd])
	if !ok {
		return nil, nil, wireu.ErrUpdateTruncated
	}
	mpReach, err := wu.MPReach()
	if err != nil {
		return nil, nil, err
	}
	mpUnreach, err := wu.MPUnreach()
	if err != nil {
		return nil, nil, err
	}

	scratch := getReadBuf(len(payload) > 4077)
	if scratch.Buf == nil {
		return nil, nil, errRelayBufferPool
	}
	defer ReturnReadBuffer(scratch)
	selection := validationSelection{
		peer:    update.SourcePeerIP,
		msgID:   update.sourceMessageID(),
		ctx:     bgpctx.Registry.Get(wu.SourceCtxID()),
		scratch: scratch.Buf,
	}
	v4Withdraw, _, err := selection.section(withdrawn, family.IPv4Unicast, false)
	if err != nil {
		return nil, nil, err
	}
	v4Announce, v4Denied, err := selection.section(announced, family.IPv4Unicast, true)
	if err != nil {
		return nil, nil, err
	}
	var mpAnnounce, mpWithdraw, mpDenied, nextHop []byte
	var reachFamily, unreachFamily family.Family
	if mpReach != nil {
		reachFamily = mpReach.Family()
		nextHop = mpReach.NextHopBytes()
		data := []byte(mpReach)
		off := 5 + len(nextHop)
		if off > len(data) {
			return nil, nil, wireu.ErrUpdateTruncated
		}
		mpAnnounce, mpDenied, err = selection.section(data[off:], reachFamily, true)
		if err != nil {
			return nil, nil, err
		}
	}
	if mpUnreach != nil {
		unreachFamily = mpUnreach.Family()
		data := []byte(mpUnreach)
		if len(data) < 3 {
			return nil, nil, wireu.ErrUpdateTruncated
		}
		mpWithdraw, _, err = selection.section(data[3:], unreachFamily, false)
		if err != nil {
			return nil, nil, err
		}
	}
	if !selection.changed {
		return wu, nil, nil
	}
	selected, err := validationBuildWire(update, spans, payload[attrStart:attrEnd], v4Withdraw, v4Announce, mpWithdraw, mpAnnounce, unreachFamily, reachFamily, nextHop)
	if err != nil {
		return nil, nil, err
	}
	denied, err := validationBuildWire(update, nil, nil, v4Denied, nil, mpDenied, nil, reachFamily, family.Family{}, nil)
	return selected, denied, err
}

type validationSelection struct {
	peer    netip.Addr
	msgID   uint64
	ctx     *bgpctx.EncodingContext
	scratch []byte
	changed bool
}

// section scans at most len(data) bytes. Kept prefixes grow from the front and
// denied prefixes from the back of one scratch region, so their combined size
// never exceeds the input. Path identifiers remain in the source's framing.
func (s *validationSelection) section(data []byte, fam family.Family, announce bool) ([]byte, []byte, error) {
	if ribevents.IsFlowSpec(fam) {
		return s.flowSpecSection(data, fam, announce)
	}
	if !ribevents.ValidationEnabled() {
		// Mandatory FlowSpec authorization supplies no verdict for siblings.
		// Their explicit withdrawals and announcements MUST remain unchanged.
		return data, nil, nil
	}
	switch fam {
	case family.IPv4Unicast, family.IPv4Multicast, family.IPv6Unicast, family.IPv6Multicast:
	default:
		return data, nil, nil
	}
	if len(data) > len(s.scratch) {
		return nil, nil, errRelayBufferPool
	}
	buf := s.scratch[:len(data)]
	s.scratch = s.scratch[len(data):]
	front, back := 0, len(buf)
	addPath := s.ctx != nil && s.ctx.AddPath(fam)
	for off := 0; off < len(data); {
		start := off
		var pathID uint32
		if addPath {
			if off+4 > len(data) {
				return nil, nil, wireu.ErrUpdateTruncated
			}
			pathID = binary.BigEndian.Uint32(data[off : off+4])
			off += 4
		}
		if off == len(data) {
			return nil, nil, wireu.ErrUpdateTruncated
		}
		bits := int(data[off])
		off++
		width := 4
		if fam.AFI == family.AFIIPv6 {
			width = 16
		}
		n := (bits + 7) / 8
		if bits > width*8 || off+n > len(data) {
			return nil, nil, wireu.ErrUpdateTruncated
		}
		var addr [16]byte
		copy(addr[:width], data[off:off+n])
		ip := netip.AddrFrom16(addr)
		if width == 4 {
			ip = netip.AddrFrom4([4]byte(addr[:4]))
		}
		off += n
		key := ribevents.ValidationRoute{Peer: s.peer, Family: fam, Prefix: netip.PrefixFrom(ip, bits).Masked(), PathID: pathID}
		keep := !ribevents.RouteEligible(key, 0)
		if announce {
			keep = ribevents.RouteEligible(key, s.msgID)
		}
		if keep {
			front += copy(buf[front:], data[start:off])
			continue
		}
		s.changed = true
		if announce && !ribevents.RouteEligible(key, 0) {
			back -= off - start
			copy(buf[back:], data[start:off])
		}
	}
	return buf[:front], buf[back:], nil
}

// flowSpecUpdate keeps mandatory authorization active even without RPKI.
func flowSpecUpdate(wu *wireu.WireUpdate) bool {
	if reach, err := wu.MPReach(); err == nil && reach != nil && ribevents.IsFlowSpec(reach.Family()) {
		return true
	}
	unreach, err := wu.MPUnreach()
	return err == nil && unreach != nil && ribevents.IsFlowSpec(unreach.Family())
}

func (s *validationSelection) flowSpecSection(data []byte, fam family.Family, announce bool) ([]byte, []byte, error) {
	if len(data) > len(s.scratch) {
		return nil, nil, errRelayBufferPool
	}
	buf := s.scratch[:len(data)]
	s.scratch = s.scratch[len(data):]
	front, back := 0, len(buf)
	addPath := s.ctx != nil && s.ctx.AddPath(fam)
	var keyErr error
	_, err := nlrisplit.SplitFlowSpec(data, addPath, func(wire []byte) {
		if keyErr != nil {
			return
		}
		raw := wire
		key := ribevents.ValidationRoute{Peer: s.peer, Family: fam}
		if addPath {
			key.PathID = binary.BigEndian.Uint32(raw[:4])
			raw = raw[4:]
		}
		key.NLRI = ribevents.FlowSpecKey(raw)
		if key.NLRI == "" {
			keyErr = errors.New("invalid FlowSpec route key")
			return
		}
		keep := !ribevents.RouteEligible(key, 0)
		if announce {
			keep = ribevents.RouteEligible(key, s.msgID)
		}
		if keep {
			front += copy(buf[front:], wire)
			return
		}
		s.changed = true
		if announce && !ribevents.RouteEligible(key, 0) {
			back -= len(wire)
			copy(buf[back:], wire)
		}
	})
	if err != nil {
		return nil, nil, err
	}
	if keyErr != nil {
		return nil, nil, keyErr
	}
	return buf[:front], buf[back:], nil
}

// validationBuildWire uses the RFC 4271 Section 4.3 body layout:
//
//	0: withdrawn length (2) | withdrawn | attribute length (2) | attributes | NLRI
//
// RFC 4760 Sections 3/4 carry MP NLRI in attributes 14/15. Existing attribute
// order and values are preserved; only the two NLRI-bearing values are replaced.
func validationBuildWire(update *ReceivedUpdate, spans []relayAttrSpan, attrs, withdrawn, announced, mpWithdraw, mpAnnounce []byte, withdrawFamily, announceFamily family.Family, nextHop []byte) (*wireu.WireUpdate, error) {
	if len(withdrawn)+len(announced)+len(mpWithdraw)+len(mpAnnounce) == 0 {
		return nil, nil
	}
	// Each output is at most the received body's size: selection removes NLRI,
	// and MP_UNREACH replaces MP_REACH's next-hop field with a shorter header.
	size := len(update.WireUpdate.Payload())
	h := getReadBuf(size > 4077)
	if h.Buf == nil {
		return nil, errRelayBufferPool
	}
	if size > len(h.Buf) {
		ReturnReadBuffer(h)
		return nil, errRelayTooLarge
	}
	buf := h.Buf
	binary.BigEndian.PutUint16(buf[:2], uint16(len(withdrawn)))
	off := 2 + copy(buf[2:], withdrawn)
	attrLenOffset := off
	off += 2
	if len(announced)+len(mpAnnounce) > 0 {
		for _, span := range spans {
			switch span.code {
			case attribute.AttrMPReachNLRI:
				if len(mpAnnounce) > 0 {
					off += writeMPReach(buf, off, announceFamily, nextHop, mpAnnounce)
				}
			case attribute.AttrMPUnreachNLRI:
				// Written below even when every announcement was refused.
			default:
				off += copy(buf[off:], attrs[span.start:span.end])
			}
		}
	}
	if len(mpWithdraw) > 0 {
		off += writeAttrHeader(buf, off, byte(attribute.FlagOptional), attribute.AttrMPUnreachNLRI, 3+len(mpWithdraw))
		binary.BigEndian.PutUint16(buf[off:off+2], uint16(withdrawFamily.AFI))
		buf[off+2] = byte(withdrawFamily.SAFI)
		off += 3 + copy(buf[off+3:], mpWithdraw)
	}
	binary.BigEndian.PutUint16(buf[attrLenOffset:attrLenOffset+2], uint16(off-attrLenOffset-2))
	off += copy(buf[off:], announced)
	wu := wireu.NewWireUpdate(buf[:off], update.WireUpdate.SourceCtxID())
	wu.SetSourceID(update.WireUpdate.SourceID())
	update.adoptFwdHandle(h)
	return wu, nil
}

// validationRetainsPath keeps the ADD-PATH identity of a pending or retained
// route until that route leaves the receive store. A stale cache withdrawal must
// not free the identity used by its replacement.
func validationRetainsPath(peer netip.Addr, fam family.Family, pathID uint32, raw []byte) bool {
	if ribevents.IsFlowSpec(fam) {
		key := ribevents.FlowSpecKey(raw)
		if key == "" {
			return false
		}
		return ribevents.RoutePresent(ribevents.ValidationRoute{Peer: peer, Family: fam, PathID: pathID, NLRI: key})
	}
	if !ribevents.ValidationEnabled() {
		return false
	}
	switch fam {
	case family.IPv4Unicast, family.IPv4Multicast, family.IPv6Unicast, family.IPv6Multicast:
	default:
		return false
	}
	if len(raw) == 0 {
		return false
	}
	width := 4
	if fam.AFI == family.AFIIPv6 {
		width = 16
	}
	bits := int(raw[0])
	n := (bits + 7) / 8
	if bits > width*8 || len(raw) < 1+n {
		return false
	}
	var addr [16]byte
	copy(addr[:width], raw[1:1+n])
	ip := netip.AddrFrom16(addr)
	if width == 4 {
		ip = netip.AddrFrom4([4]byte(addr[:4]))
	}
	return ribevents.RoutePresent(ribevents.ValidationRoute{
		Peer: peer, Family: fam, Prefix: netip.PrefixFrom(ip, bits).Masked(), PathID: pathID,
	})
}

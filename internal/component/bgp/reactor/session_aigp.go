// Design: docs/architecture/route-selection.md
// Related: docs/architecture/api/update-syntax.md -- recognized raw UPDATE admission
// Related: docs/architecture/buffer-architecture.md -- retained mixed origination input
// Related: register_originated_next_hop.go -- originated peer-own refusal diagnostic
// RFC: rfc/short/rfc7311.md, Sections 3.3 and 3.4.1.
// RFC 4271 Section 5.1.3 -- see rfc/short/rfc4271.md.
package reactor

import (
	"encoding/binary"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	wireu "github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/report"
)

// aigpOriginAllowed is deliberately independent of the export policy: a filter
// may supply a metric, but cannot authorize an out-of-domain route or next hop.
// The caller MUST hold writeMu, which also protects negotiation publication.
func (s *Session) aigpOriginAllowed(body []byte) bool {
	if !s.settings.AIGPEnabled() || !s.settings.AIGPOriginate {
		return false
	}
	sections, err := wireu.ParseUpdateSections(body)
	if err != nil {
		return false
	}
	attrs := sections.Attrs(body)
	local := s.settings.LocalAddress
	scope := s.nextHopScope.Load()
	announced := false
	if len(sections.NLRI(body)) != 0 {
		_, _, nextHop, found := attribute.AttrFind(attrs, attribute.AttrNextHop)
		if !found || len(nextHop) != 4 {
			return false
		}
		ip := netip.AddrFrom4([4]byte(nextHop))
		if !isLocalAIGPNextHop(nextHopValue{legacy: ip}, local, scope) {
			return false
		}
		announced = true
	}
	if _, _, reach, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI); found {
		mp, err := attribute.ParseMPReachNLRI(reach)
		if err != nil {
			return false
		}
		if len(mp.NLRI) != 0 {
			if mp.NextHops.Len() == 0 {
				return false
			}
			for _, nextHop := range mp.NextHops.Slice() {
				if !isLocalAIGPNextHop(nextHopValue{mp: nextHop}, local, scope) {
					return false
				}
			}
			announced = true
		}
	}
	if !announced {
		return false
	}
	_, _, path, present := attribute.AttrFind(attrs, attribute.AttrASPath)
	if !present {
		return false
	}
	width := 2
	if neg := s.negotiated; neg != nil && neg.ASN4 {
		width = 4
	}
	for len(path) != 0 {
		if len(path) < 2 || path[0] < byte(attribute.ASSet) || path[0] > byte(attribute.ASConfedSet) {
			return false
		}
		n := int(path[1]) * width
		path = path[2:]
		if n > len(path) {
			return false
		}
		for off := 0; off < n; off += width {
			asn := uint32(binary.BigEndian.Uint16(path[off:]))
			if width == 4 {
				asn = binary.BigEndian.Uint32(path[off:])
			}
			inside := asn == s.settings.LocalAS
			for _, domainAS := range s.settings.AIGPDomainAS {
				inside = inside || asn == domainAS
			}
			if !inside {
				return false
			}
		}
		path = path[n:]
	}
	return true
}

func isLocalAIGPNextHop(nh nextHopValue, local netip.Addr, scope *receiveNextHopScope) bool {
	if nh.has(local.Unmap()) {
		return true
	}
	if scope != nil {
		if nh.has(scope.local) {
			return true
		}
		for _, address := range scope.addresses {
			if nh.has(address.Addr().Unmap()) {
				return true
			}
		}
	}
	return false
}

func aigpPresent(body []byte) bool {
	sections, err := wireu.ParseUpdateSections(body)
	if err != nil {
		return false
	}
	_, _, _, present := attribute.AttrFind(sections.Attrs(body), attribute.AttrAIGP)
	return present
}

// stripAIGPBody preserves the exact encoding of every unrelated field, including
// withdrawals and MP attributes. dst belongs to the session buffer pool, not to
// the immutable received UPDATE shared by other destinations.
func stripAIGPBody(dst, body []byte) ([]byte, error) {
	sections, err := wireu.ParseUpdateSections(body)
	if err != nil {
		return nil, err
	}
	attrs := sections.Attrs(body)
	attrStart := 4 + sections.WithdrawnLen()
	copy(dst, body[:attrStart])
	// RFC 7311 Section 3.3.
	written := attrStart + len(stripAIGPAttributes(dst[attrStart:], attrs))
	binary.BigEndian.PutUint16(dst[attrStart-2:attrStart], uint16(written-attrStart))
	written += copy(dst[written:], sections.NLRI(body))
	return dst[:written], nil
}

// stripAIGPAttributes copies only attributes, so an ordinary logical body need
// not fit a wire buffer before splitting. The caller MUST provide len(attrs)
// writable bytes in dst; the result MUST be consumed before dst is reused.
// RFC 7311 Section 3.3: "The AIGP attribute MUST NOT be sent on any BGP session
// for which AIGP_SESSION is disabled".
func stripAIGPAttributes(dst, attrs []byte) []byte {
	written := 0
	it := attribute.NewAttrIterator(attrs)
	off := 0
	for {
		code, flags, value, ok := it.Next()
		if !ok {
			break
		}
		header := 3
		if flags&attribute.FlagExtLength != 0 {
			header = 4
		}
		n := header + len(value)
		if code != attribute.AttrAIGP {
			written += copy(dst[written:], attrs[off:off+n])
		}
		off += n
	}
	return dst[:written]
}

func (s *Session) writeUpdateWithoutAIGP(body []byte, raw bool) (bool, error) {
	handle := getReadBuf(len(body) > message.MaxMsgLen-message.HeaderLen)
	defer s.returnReadBuffer(handle)
	filtered, err := stripAIGPBody(handle.Buf, body)
	if err != nil {
		return false, err
	}
	return s.writeUpdateBody(filtered, raw)
}

// writeOriginatedRawUpdate applies origination policy to both raw API forms.
// Received forwarding MUST use writeRawUpdateBody instead: it preserves the
// source's metric and has already applied the egress accumulation rules.
// The caller MUST hold writeMu and flush the buffered write. Only SendRawMessage
// passes raw=true; it MUST retire the Session after flushing opaque output.
func (s *Session) writeOriginatedRawUpdate(body []byte, raw bool) (bool, error) {
	// RFC 4271 Section 5.1.3: "A route originated by a BGP speaker SHALL NOT be
	// advertised to a peer using an address of that peer as NEXT_HOP."
	if aigpNextHop(body).has(s.settings.Address) {
		return s.writeOriginatedPeerOwnUpdate(body, raw)
	}
	if aigpPresent(body) {
		if !s.aigpOriginAllowed(body) {
			return s.writeUpdateWithoutAIGP(body, raw)
		}
	}
	return s.writeUpdateBody(body, raw)
}

// writeOriginatedPeerOwnUpdate refuses the offending announcement field while
// preserving original withdrawals and permitted families. The caller MUST hold
// writeMu and MUST call this only when an active next hop names the recipient.
// A retained read buffer keeps borrowed input stable as callbacks reuse writeBuf.
// Each callback consumes its chunk synchronously and refuses a forbidden chunk
// directly: even malformed duplicate MP attributes cannot cause another split.
//
// RFC 4271 Section 5.1.3: "A route originated by a BGP speaker SHALL NOT be
// advertised to a peer using an address of that peer as NEXT_HOP."
// RFC 4271 Section 4.3 body offsets: [0:2] Withdrawn length, [2:] Withdrawn,
// Attribute length(2), Attributes, NLRI. Retaining the complete body preserves
// every section until the final synchronous callback has consumed it.
func (s *Session) writeOriginatedPeerOwnUpdate(body []byte, raw bool) (bool, error) {
	sections, err := wireu.ParseUpdateSections(body)
	if err != nil {
		return false, err
	}
	update := message.Update{
		WithdrawnRoutes: sections.Withdrawn(body),
		PathAttributes:  sections.Attrs(body),
		NLRI:            sections.NLRI(body),
	}
	// RFC 7606 Section 5.1: a sole announcement has no permitted sibling to keep.
	if !update.MixesNLRIFields() {
		// RFC 4271 Section 5.1.3.
		s.reportOriginatedPeerOwnNextHop()
		return false, nil
	}
	// RFC 8654 Section 4: the retained input is bounded by one extended message.
	if len(body) > message.ExtMsgLen-message.HeaderLen {
		return false, message.ErrUpdateTooLarge
	}
	handle := getReadBuf(len(body) > message.MaxMsgLen-message.HeaderLen)
	defer s.returnReadBuffer(handle)
	retained := handle.Buf[:len(body)]
	copy(retained, body)
	update.WithdrawnRoutes = sections.Withdrawn(retained)
	update.PathAttributes = sections.Attrs(retained)
	update.NLRI = sections.NLRI(retained)

	splitter := message.GetSplitter()
	defer message.PutSplitter(splitter)
	opaque := false
	emit := func(chunk *message.Update) error {
		// RFC 4271 Section 4.3; RFC 8654 Section 4.
		n, err := chunk.CheckedWriteTo(s.writeBuf.Buffer(), 0, nil)
		if err != nil {
			return err
		}
		output := s.writeBuf.Buffer()[message.HeaderLen:n]
		// RFC 4271 Section 5.1.3: ignore next-hop attributes governing no NLRI.
		if aigpNextHop(output).has(s.settings.Address) {
			s.reportOriginatedPeerOwnNextHop()
			return nil
		}
		// RFC 4271 Section 4.3; RFC 7311 Section 3.3: retain the common writer.
		chunkOpaque, err := s.writeOriginatedRawUpdate(output, raw)
		opaque = opaque || chunkOpaque
		return err
	}
	// RFC 7606 Section 5.1; RFC 7911 Sections 3 and 5.
	err = splitter.SplitCompliant(&update, len(s.writeBuf.Buffer()), s.mpAddPathSend, emit)
	if err != nil {
		// Earlier chunks may have reached the buffered writer. Commit their
		// ownership and finish opaque-output retirement before returning an error.
		if finishErr := s.finishRawWrite(opaque); finishErr != nil {
			return opaque, errors.Join(err, finishErr)
		}
	}
	return opaque, err
}

// reportOriginatedPeerOwnNextHop records a route-scoped refusal without closing
// the usable session. The caller MUST hold writeMu; reporting waits on no peer.
// RFC 4271 Section 5.1.3: "A route originated by a BGP speaker SHALL NOT be
// advertised to a peer using an address of that peer as NEXT_HOP."
func (s *Session) reportOriginatedPeerOwnNextHop() {
	const problem = "route not advertised: its NEXT_HOP is the recipient's own address"
	sessionLogger().Warn(problem, "peer", s.settings.Address,
		"fix", originatedPeerOwnNextHopFix, "code", codeOriginatedPeerOwnNextHop)
	report.RaiseErrorFix(reportSourceBGP, codeOriginatedPeerOwnNextHop,
		s.settings.Address.String(), problem, originatedPeerOwnNextHopFix, nil)
}

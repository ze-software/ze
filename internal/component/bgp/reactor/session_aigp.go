// Design: docs/architecture/route-selection.md
// RFC: rfc/short/rfc7311.md, Sections 3.3 and 3.4.1.
package reactor

import (
	"encoding/binary"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	wireu "github.com/ze-software/ze/internal/core/bgp/wire"
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
	written := attrStart
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
	binary.BigEndian.PutUint16(dst[attrStart-2:attrStart], uint16(written-attrStart))
	written += copy(dst[written:], sections.NLRI(body))
	return dst[:written], nil
}

func (s *Session) writeUpdateWithoutAIGP(body []byte) error {
	handle := s.getReadBuffer()
	defer s.returnReadBuffer(handle)
	filtered, err := stripAIGPBody(handle.Buf, body)
	if err != nil {
		return err
	}
	return s.writeRawUpdateBody(filtered)
}

// writeOriginatedRawUpdate applies origination policy to both raw API forms.
// Received forwarding MUST use writeRawUpdateBody instead: it preserves the
// source's metric and has already applied the egress accumulation rules.
// The caller MUST hold writeMu and flush the buffered write.
func (s *Session) writeOriginatedRawUpdate(body []byte) error {
	if aigpPresent(body) {
		if !s.aigpOriginAllowed(body) {
			return s.writeUpdateWithoutAIGP(body)
		}
	}
	return s.writeRawUpdateBody(body)
}

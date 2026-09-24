// Design: docs/architecture/route-selection.md -- received NEXT_HOP validation
// RFC: rfc/short/rfc4271.md -- Section 6.3 next-hop semantic checks
package reactor

import (
	"encoding/binary"
	"net"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/network"
)

// receiveNextHopScope is immutable. Interface events replace the snapshot;
// the UPDATE path never reads the kernel's interface table.
type receiveNextHopScope struct {
	addresses []netip.Prefix
	local     netip.Addr
	remote    netip.Addr
	direct    bool
}

func newReceiveNextHopScope(addresses []netip.Prefix, conn net.Conn, settings *PeerSettings) *receiveNextHopScope {
	local, remote := settings.LocalAddress, settings.Address
	if addr, ok := conn.LocalAddr().(*net.TCPAddr); ok {
		local = addr.AddrPort().Addr().Unmap()
	}
	if addr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		remote = addr.AddrPort().Addr().Unmap()
	}
	// RFC 4271 Section 6.3 applies the common-subnet condition only "where the
	// sender and receiver are one IP hop away from each other". A sender at a
	// loopback address, or at an address this host holds, runs on the receiving
	// host: it is zero hops away, and no link exists for a next hop to share.
	sameHost := remote.IsLoopback() || holdsAddress(addresses, remote)
	return &receiveNextHopScope{
		addresses: addresses,
		local:     local,
		remote:    remote,
		direct:    !sameHost && (network.SharesSubnet(addresses, remote) || settings.OutTTL == 1 || settings.MinTTL == 255),
	}
}

// holdsAddress reports whether addr is one of this host's interface addresses.
func holdsAddress(addresses []netip.Prefix, addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, address := range addresses {
		if addr == address.Addr().Unmap() {
			return true
		}
	}
	return false
}

func (s *Session) invalidReceiveNextHop(wu *wireu.WireUpdate) bool {
	nlri, err := wu.NLRI()
	if err != nil || len(nlri) == 0 {
		return false // MP_REACH carries its own next hop, not NEXT_HOP.
	}
	attrs, err := wu.Attrs()
	if err != nil || attrs == nil {
		return true
	}
	value, err := attrs.GetRaw(attribute.AttrNextHop)
	if err != nil || len(value) != 4 {
		return true
	}
	nextHop := netip.AddrFrom4([4]byte{value[0], value[1], value[2], value[3]})
	scope := s.nextHopScope.Load()
	if scope == nil {
		return true // No current interface view can establish semantic validity.
	}
	// RFC 4271 Section 6.3: "It MUST NOT be the IP address of the
	// receiving speaker." This includes addresses on other interfaces.
	if nextHop == scope.local || nextHop == s.settings.LocalAddress || holdsAddress(scope.addresses, nextHop) {
		return true
	}
	peerAS := s.settings.PeerAS
	if neg := s.Negotiated(); peerAS == 0 && neg != nil {
		peerAS = neg.PeerASN
	}
	if s.settings.isIBGPWith(peerAS) || !scope.direct || nextHop == scope.remote {
		return false
	}
	// RFC 4271 Section 6.3: on a one-hop eBGP connection, the next-hop
	// interface must share a common subnet with the receiving speaker.
	return !network.SharesSubnet(scope.addresses, nextHop)
}

// withdrawLegacyAnnouncements leaves explicit withdrawals and MP routes intact.
// A semantically invalid NEXT_HOP belongs only to the legacy IPv4 announcements,
// so it must not discard unrelated MP_REACH routes in the same UPDATE.
func (s *Session) withdrawLegacyAnnouncements(wu *wireu.WireUpdate) *wireu.WireUpdate {
	body := wu.Payload()
	withdrawnLen := int(binary.BigEndian.Uint16(body[:2]))
	attrOffset := 2 + withdrawnLen
	attrLen := int(binary.BigEndian.Uint16(body[attrOffset : attrOffset+2]))
	nlriOffset := attrOffset + 2 + attrLen
	nlri := body[nlriOffset:]
	out := make([]byte, len(body))
	binary.BigEndian.PutUint16(out[:2], uint16(withdrawnLen+len(nlri)))
	copy(out[2:], body[2:attrOffset])
	copy(out[attrOffset:], nlri)
	copy(out[attrOffset+len(nlri):], body[attrOffset:nlriOffset])
	rewritten := wireu.NewWireUpdate(out, wu.SourceCtxID())
	rewritten.SetSourceID(wu.SourceID())
	return s.publishBase(rewritten)
}

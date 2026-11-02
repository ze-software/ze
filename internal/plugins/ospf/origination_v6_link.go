// Design: docs/architecture/ospf/ospf-af-unify.md -- OSPFv3 (IPv6) Link-LSA + Network Intra-Area-Prefix origination.
// RFC: rfc/short/rfc5340.md (App A.4.9 Link-LSA, A.4.10 Intra-Area-Prefix-LSA)
package ospf

import (
	"net"
	"net/netip"
	"sort"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func v6LinkKey(router types.RouterID, ifaceID uint32) types.LSAKey {
	return types.LSAKey{Type: types.LSTypeLink, LinkStateID: v6SummaryLSID(ifaceID), AdvertisingRouter: router}
}

func v6NetworkIntraAreaPrefixKey(router types.RouterID, ifaceID uint32) types.LSAKey {
	return types.LSAKey{Type: types.LSType(ospfv3types.LSTypeIntraAreaPrefix), LinkStateID: v6SummaryLSID(ifaceID), AdvertisingRouter: router}
}

func v6ShouldOriginateLinkLSA(iface ospflsdb.InterfaceInfo) bool {
	if !iface.IsV6 || iface.Name == "" || !v6AdvertiseInterface(iface) {
		return false
	}
	// Every OSPFv3 interface type originates a Link-LSA so neighbors learn the
	// link-local (the SPF next-hop) and the on-link prefixes (RFC 5340 sec 4.4.3.8),
	// including NBMA and point-to-multipoint.
	switch iface.NetworkType {
	case types.NetworkBroadcast, types.NetworkPointToPoint, types.NetworkNBMA, types.NetworkPointToMultipoint:
		return true
	default:
		return false
	}
}

// RFC 5340 §4.4.3.8: a router originates one Link-LSA per attached link carrying
// the link-local address, link options, router priority, and prefixes on that link.
func (e *engine) v6OriginateLinkLSA(router types.RouterID, iface ospflsdb.InterfaceInfo) (types.LSAKey, bool) {
	linkAddr, prefixes, ok := e.linkLSAAddressing(iface)
	if !ok {
		return types.LSAKey{}, false
	}
	body := ospfv3packet.LinkLSA{
		RtrPriority:   iface.Priority,
		Options:       neutralToV6Options(iface.Options),
		LinkLocalAddr: linkAddr,
		Prefixes:      prefixes,
	}
	bodyBytes := make([]byte, body.EncodedLen())
	body.WriteTo(bodyBytes, 0)
	key := v6LinkKey(router, iface.InterfaceID)
	_, ok = e.lsdb.OriginateLinkSelf(iface.Name, iface.AreaID, key, bodyBytes, func(seq types.LSSequenceNumber, purge bool) packet.LSA {
		return v6SelfLSA(ospfv3packet.LSA{
			Header: v6OriginHeader(ospfv3types.LSTypeLink, ospfv3types.LinkStateID(key.LinkStateID), router, seq, purge),
			Link:   &body,
		})
	})
	return key, ok
}

// linkLSAAddressing returns the Link-LSA's "link local address" field and on-link
// prefixes for this engine's address family. An IPv6 family carries the interface's
// IPv6 link-local and IPv6 prefixes. An IPv4 family carries the link's IPv4 address
// and IPv4 prefix: RFC 5838 Section 2.5, "the link's IPv4 address will be advertised
// in the "link local address" field of the IPv4 instance's Link-LSA. This address is
// placed in the first 32 bits of the "link local address" field and is used for IPv4
// next-hop calculations. The remaining bits MUST be set to zero." Without the address
// the family needs, no Link-LSA is originated.
func (e *engine) linkLSAAddressing(iface ospflsdb.InterfaceInfo) ([16]byte, []ospfv3packet.Prefix, bool) {
	var field [16]byte
	if e.af.isIPv4() {
		if iface.Address == ([4]byte{}) {
			return field, nil, false
		}
		copy(field[:4], iface.Address[:])
		return field, v4LinkLSAPrefixes(iface), true
	}
	ll := iface.IPv6LinkLocal
	if !ll.IsValid() {
		ll = interfaceIPv6LinkLocal(iface.Name)
	}
	if !ll.IsValid() || !ll.Is6() || ll.Is4In6() || !ll.IsLinkLocalUnicast() {
		return field, nil, false
	}
	return ll.As16(), v6LinkLSAPrefixes(iface), true
}

// v4LinkLSAPrefixes is the IPv4 family's on-link prefix: the interface address under
// its mask, encoded as a 0-32-bit prefix (RFC 5838 Section 2.7). An interface with no
// mask, or a mask that is not contiguous, carries none.
func v4LinkLSAPrefixes(iface ospflsdb.InterfaceInfo) []ospfv3packet.Prefix {
	bits, _ := net.IPMask(iface.NetworkMask[:]).Size()
	if bits == 0 {
		return nil
	}
	prefix, ok := netipToV6Prefix(netip.PrefixFrom(netip.AddrFrom4(iface.Address), bits), 0)
	if !ok {
		return nil
	}
	return []ospfv3packet.Prefix{prefix}
}

func v6LinkLSAPrefixes(iface ospflsdb.InterfaceInfo) []ospfv3packet.Prefix {
	prefixes := iface.IPv6Prefixes
	if prefixes == nil {
		prefixes = interfaceIPv6Prefixes(iface.Name)
	}
	return v6PrefixesForLinkLSA(prefixes)
}

func v6PrefixesForLinkLSA(prefixes []netip.Prefix) []ospfv3packet.Prefix {
	seen := make(map[string]struct{})
	out := make([]ospfv3packet.Prefix, 0, len(prefixes))
	for _, pfx := range prefixes {
		if !pfx.IsValid() || !pfx.Addr().Is6() || pfx.Addr().Is4In6() {
			continue
		}
		pfx = pfx.Masked()
		addr := pfx.Addr()
		if addr.IsLinkLocalUnicast() || addr.IsLoopback() || addr.IsUnspecified() || addr.IsMulticast() {
			continue
		}
		key := pfx.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		p, ok := netipToV6Prefix(pfx, 0)
		if ok {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		pi, _ := v6PrefixToNetip(out[i], afIPv6Unicast)
		pj, _ := v6PrefixToNetip(out[j], afIPv6Unicast)
		return pi.String() < pj.String()
	})
	return out
}

// RFC 5340 §4.4.3.9: the DR originates a Network-referencing Intra-Area-Prefix-LSA
// for the transit link from the prefixes found in attached routers' Link-LSAs.
func (e *engine) v6OriginateNetworkIntraAreaPrefix(area types.AreaID, router types.RouterID, iface ospflsdb.InterfaceInfo) (types.LSAKey, bool) {
	prefixes := e.v6AggregatedLinkPrefixes(iface.Name)
	if len(prefixes) == 0 {
		return types.LSAKey{}, false
	}
	body := ospfv3packet.IntraAreaPrefixLSA{
		ReferencedLSType:      ospfv3types.LSTypeNetwork,
		ReferencedLinkStateID: ospfv3types.LinkStateID(v6SummaryLSID(iface.InterfaceID)),
		ReferencedAdvRouter:   ospfv3types.RouterID(router),
		Prefixes:              prefixes,
	}
	bodyBytes := make([]byte, body.EncodedLen())
	body.WriteTo(bodyBytes, 0)
	key := v6NetworkIntraAreaPrefixKey(router, iface.InterfaceID)
	_, ok := e.lsdb.OriginateSelf(area, key, bodyBytes, func(seq types.LSSequenceNumber, purge bool) packet.LSA {
		return v6SelfLSA(ospfv3packet.LSA{
			Header:       v6OriginHeader(ospfv3types.LSTypeIntraAreaPrefix, ospfv3types.LinkStateID(key.LinkStateID), router, seq, purge),
			IntraAreaPfx: &body,
		})
	})
	return key, ok
}

func (e *engine) v6AggregatedLinkPrefixes(iface string) []ospfv3packet.Prefix {
	if e.lsdb == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var out []ospfv3packet.Prefix
	for _, lsa := range e.lsdb.LinkLSAs(iface) {
		if lsa.Header.Age.IsMaxAge() || !ospfv3packet.VerifyLSAChecksum(lsa.RawBytes) {
			continue
		}
		decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
		if err != nil {
			continue
		}
		link, err := decoded.DecodeLink()
		if err != nil {
			continue
		}
		for _, p := range link.Prefixes {
			if p.Options.NoUnicast() || p.Options.LocalAddress() {
				continue
			}
			pfx, ok := v6PrefixToNetip(p, afIPv6Unicast)
			if !ok || pfx.Addr().IsLinkLocalUnicast() {
				continue
			}
			key := pfx.String()
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			p.Field16 = 0
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		pi, _ := v6PrefixToNetip(out[i], afIPv6Unicast)
		pj, _ := v6PrefixToNetip(out[j], afIPv6Unicast)
		return pi.String() < pj.String()
	})
	return out
}

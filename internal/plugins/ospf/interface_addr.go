// Design: docs/architecture/ospf/ospf-af-unify.md -- interface address/identity helpers.
//
// These resolve an OSPF interface's live OS properties (IPv4 address, mask, MTU,
// ifindex) through the iface component, the inputs the engine feeds into the iface
// runtime config and LSA origination. They live apart from instance.go to keep that
// file within the size budget.
//
// RFC: rfc/short/rfc5340.md (App A.4.7 / A.4.8 forwarding-address eligibility, sec 3.4.3
// Interface ID = MIB-II ifIndex)

package ospf

import (
	"net/netip"

	ifcomp "github.com/ze-software/ze/internal/component/iface"
)

const (
	interfaceFamilyIPv4 = "ipv4"
	interfaceFamilyIPv6 = "ipv6"
)

func interfaceNetworkMask(name string) [4]byte {
	// Passive / loopback interfaces skip the transport open that loads the iface
	// backend, so an OSPF-only config (no interface{} block) with only such
	// interfaces would otherwise read no backend here and originate a Router-LSA
	// advertising 0.0.0.0. Ensure the default backend is loaded (no-op otherwise).
	_ = ifcomp.EnsureBackend()
	addrs, err := ifcomp.Addresses(name)
	if err != nil {
		return [4]byte{}
	}
	for _, addr := range addrs {
		if addr.Family != interfaceFamilyIPv4 || addr.PrefixLength < 0 || addr.PrefixLength > 32 {
			continue
		}
		return maskFromPrefixLength(addr.PrefixLength)
	}
	return [4]byte{}
}

func interfaceIPv4Address(name string) [4]byte {
	// See interfaceNetworkMask: ensure the iface backend is loaded so a
	// passive/loopback-only OSPF config does not advertise 0.0.0.0.
	_ = ifcomp.EnsureBackend()
	addrs, err := ifcomp.Addresses(name)
	if err != nil {
		return [4]byte{}
	}
	for _, addr := range addrs {
		if addr.Family != interfaceFamilyIPv4 {
			continue
		}
		parsed, err := netip.ParseAddr(addr.Address)
		if err == nil && parsed.Is4() {
			return parsed.As4()
		}
	}
	return [4]byte{}
}

// v6UsableForwardingAddress reports whether addr may be advertised as an OSPFv3
// AS-External-LSA / NSSA-LSA forwarding address. RFC 5340 App A.4.7 forbids the IPv6
// Unspecified Address and any IPv6 Link-Local Address there, and App A.4.8 requires a
// global address for an NSSA-LSA an area border router propagates; loopback and multicast
// are likewise unroutable across the AS. It is the single eligibility rule
// interfaceIPv6ForwardingAddress applies to every candidate interface address.
func v6UsableForwardingAddress(addr netip.Addr) bool {
	if !addr.Is6() || addr.Is4In6() {
		return false
	}
	return !addr.IsLinkLocalUnicast() && !addr.IsLoopback() && !addr.IsUnspecified() && !addr.IsMulticast()
}

func interfaceIPv6ForwardingAddress(name string) ([16]byte, bool) {
	addrs, err := ifcomp.Addresses(name)
	if err != nil {
		return [16]byte{}, false
	}
	for _, addr := range addrs {
		if addr.Family != interfaceFamilyIPv6 {
			continue
		}
		parsed, err := netip.ParseAddr(addr.Address)
		if err != nil || !v6UsableForwardingAddress(parsed) {
			continue
		}
		return parsed.As16(), true
	}
	return [16]byte{}, false
}

func interfaceMTU(name string) uint16 {
	infos, err := ifcomp.ListInterfaces()
	if err != nil {
		return 1500
	}
	for idx := range infos {
		info := &infos[idx]
		if info.Name != name && info.OsName != name {
			continue
		}
		if info.MTU <= 0 || info.MTU > 65535 {
			return 1500
		}
		return uint16(info.MTU)
	}
	return 1500
}

// interfaceIndex returns the OS ifindex for an interface, the value the engine uses as
// the OSPFv3 Interface ID (RFC 5340 sec 3.4.3) in both the Hello and the Router-LSA so
// the two agree. It returns 0 when the interface is not found (OSPFv2 ignores it).
func interfaceIndex(name string) uint32 {
	infos, err := ifcomp.ListInterfaces()
	if err != nil {
		return 0
	}
	for idx := range infos {
		info := &infos[idx]
		if info.Name != name && info.OsName != name {
			continue
		}
		if info.Index > 0 {
			return uint32(info.Index)
		}
		return 0
	}
	return 0
}

func maskFromPrefixLength(prefix int) [4]byte {
	var mask uint32
	if prefix > 0 {
		mask = ^uint32(0) << uint(32-prefix)
	}
	return [4]byte{byte(mask >> 24), byte(mask >> 16), byte(mask >> 8), byte(mask)}
}

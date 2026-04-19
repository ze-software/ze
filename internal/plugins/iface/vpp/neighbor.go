// Design: docs/features/interfaces.md -- VPP neighbor-table readback via ip_neighbor_dump
// Overview: ifacevpp.go -- ListNeighbors declaration lives on the backend
// Related: fib.go -- sibling VPP dump (IPRouteV2Dump)

package ifacevpp

import (
	"fmt"
	"net"
	"net/netip"

	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ip_neighbor"
	"go.fd.io/govpp/binapi/ip_types"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// allNeighborSwIfIndex is the VPP sentinel meaning "match every interface"
// on an ip_neighbor_dump request. The binapi declares the default as
// 4294967295 (~0u32); keeping the constant local documents the semantics
// next to the call site.
const allNeighborSwIfIndex = interface_types.InterfaceIndex(^uint32(0))

// Neighbor state names emitted in iface.NeighborInfo.State. The
// vocabulary matches the netlink backend's neighStateString output so
// operators see identical strings across backends. VPP exposes fewer
// NUD-equivalent flags than Linux (only STATIC / NO_FIB_ENTRY), so on
// this backend only statePermanent and stateReachable are produced.
const (
	statePermanent = "permanent"
	stateReachable = "reachable"
)

// ListNeighbors returns the VPP neighbor cache (IPv4 ARP + IPv6 ND) via
// ip_neighbor_dump. `family` is one of iface.NeighborFamilyAny /
// NeighborFamilyIPv4 / NeighborFamilyIPv6; Any dumps both families and
// concatenates the results in v4-then-v6 order. Every entry resolves its
// SwIfIndex to a ze interface name via the backend's name map; when the
// index is unmapped (e.g. a VPP physical port that was never brought under
// ze's control) the Device field is left empty rather than exposing an
// opaque integer.
func (b *vppBackendImpl) ListNeighbors(family int) ([]iface.NeighborInfo, error) {
	if err := b.ensureChannel(); err != nil {
		return nil, err
	}

	families, err := neighborFamilies(family)
	if err != nil {
		return nil, err
	}

	result := make([]iface.NeighborInfo, 0, 16)
	for _, af := range families {
		req := &ip_neighbor.IPNeighborDump{
			SwIfIndex: allNeighborSwIfIndex,
			Af:        af,
		}
		ctx := b.ch.SendMultiRequest(req)
		for {
			details := &ip_neighbor.IPNeighborDetails{}
			last, recvErr := ctx.ReceiveReply(details)
			if recvErr != nil {
				return nil, fmt.Errorf("ifacevpp: IPNeighborDump(af=%d): %w", af, recvErr)
			}
			if last {
				break
			}
			entry, ok := neighborToInfo(&details.Neighbor, b.names.LookupName)
			if !ok {
				continue
			}
			result = append(result, entry)
		}
	}
	return result, nil
}

// neighborFamilies translates the caller's family selector (one of the
// iface.NeighborFamily* constants) into the VPP address-family values
// the dump request accepts. NeighborFamilyAny expands to both v4 and v6
// in that order; an unrecognized selector is rejected (exact-or-reject).
func neighborFamilies(family int) ([]ip_types.AddressFamily, error) {
	switch family {
	case iface.NeighborFamilyIPv4:
		return []ip_types.AddressFamily{ip_types.ADDRESS_IP4}, nil
	case iface.NeighborFamilyIPv6:
		return []ip_types.AddressFamily{ip_types.ADDRESS_IP6}, nil
	case iface.NeighborFamilyAny:
		return []ip_types.AddressFamily{ip_types.ADDRESS_IP4, ip_types.ADDRESS_IP6}, nil
	}
	return nil, fmt.Errorf("ifacevpp: ListNeighbors: unsupported family %d", family)
}

// neighborToInfo converts a VPP ip_neighbor entry to iface.NeighborInfo.
// Returns false when the IP address is malformed (neither v4 nor v6) so
// the caller can skip it without dropping the whole reply. lookupName
// resolves a SwIfIndex to a ze interface name; when absent the Device
// field is left empty (matches fib.go's policy on unmapped ports).
func neighborToInfo(n *ip_neighbor.IPNeighbor, lookupName func(uint32) (string, bool)) (iface.NeighborInfo, bool) {
	addr, fam := neighborAddrString(n.IPAddress)
	if addr == "" {
		return iface.NeighborInfo{}, false
	}
	entry := iface.NeighborInfo{
		Address: addr,
		Family:  fam,
		State:   neighborStateName(n.Flags),
	}
	// Zero MAC means the entry has not resolved yet (INCOMPLETE / no
	// reply): leave MAC empty -- mirrors netlink backend convention.
	var zeroMAC [6]uint8
	if n.MacAddress != zeroMAC {
		entry.MAC = net.HardwareAddr(n.MacAddress[:]).String()
	}
	if name, ok := lookupName(uint32(n.SwIfIndex)); ok {
		entry.Device = name
	}
	return entry, true
}

// neighborAddrString renders an ip_types.Address as a textual IP + family
// string ("ipv4"/"ipv6"). Returns empty strings when the Af byte is not
// one of the two recognized families.
func neighborAddrString(a ip_types.Address) (string, string) {
	switch a.Af {
	case ip_types.ADDRESS_IP4:
		ip4 := a.Un.GetIP4()
		return netip.AddrFrom4(ip4).String(), familyIPv4
	case ip_types.ADDRESS_IP6:
		ip6 := a.Un.GetIP6()
		return netip.AddrFrom16(ip6).String(), familyIPv6
	}
	return "", ""
}

// neighborStateName translates the VPP IPNeighborFlags bitfield to the
// same vocabulary the netlink backend emits (permanent / reachable /
// stale / ...). VPP's flags carry far less information than Linux's
// NUD_* states: only STATIC and NO_FIB_ENTRY are exposed, and dumped
// entries are by definition cached (the dump excludes unresolved
// placeholders). We map STATIC to "permanent" and everything else to
// "reachable" so operators get comparable output across backends.
func neighborStateName(flags ip_neighbor.IPNeighborFlags) string {
	if flags&ip_neighbor.IP_API_NEIGHBOR_FLAG_STATIC != 0 {
		return statePermanent
	}
	return stateReachable
}

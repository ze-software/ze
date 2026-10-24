// Design: docs/research/vpp-deployment-reference.md -- VPP interface query and MAC operations
// Overview: ifacevpp.go -- VPP Backend implementation
// Related: naming.go -- bidirectional name map fed by populateNameMap
// Related: monitor.go -- async interface event delivery

package ifacevpp

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"

	"go.fd.io/govpp/api"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/ip_types"

	"github.com/ze-software/ze/internal/component/iface"
)

// dumpAllInterfaces issues SwInterfaceDump with no filter and collects every
// SwInterfaceDetails reply. The VPP SwIfIndex sentinel 0xFFFFFFFF means "dump
// everything"; passing 0 would dump only the first interface.
func (b *vppBackendImpl) dumpAllInterfaces() ([]interfaces.SwInterfaceDetails, error) {
	if err := b.ensureChannel(); err != nil {
		return nil, err
	}
	return b.dumpAllRaw()
}

// dumpAllRaw performs the SwInterfaceDump without going through
// ensureChannel. Used by populateNameMap inside ensureChannel's
// initialisation path to avoid sync.Once recursion.
func (b *vppBackendImpl) dumpAllRaw() ([]interfaces.SwInterfaceDetails, error) {
	req := &interfaces.SwInterfaceDump{SwIfIndex: ^interface_types.InterfaceIndex(0)}
	ctx := b.ch.SendMultiRequest(req)
	var out []interfaces.SwInterfaceDetails
	for {
		details := &interfaces.SwInterfaceDetails{}
		last, err := ctx.ReceiveReply(details)
		if err != nil {
			return nil, fmt.Errorf("ifacevpp: SwInterfaceDump: %w", err)
		}
		if last {
			break
		}
		out = append(out, *details)
	}
	return out, nil
}

// dumpByName issues SwInterfaceDump with a name filter. VPP matches by
// substring, so the caller must re-check the returned name for exact match.
func (b *vppBackendImpl) dumpByName(name string) ([]interfaces.SwInterfaceDetails, error) {
	if err := b.ensureChannel(); err != nil {
		return nil, err
	}
	req := &interfaces.SwInterfaceDump{
		SwIfIndex:       ^interface_types.InterfaceIndex(0),
		NameFilterValid: true,
		NameFilter:      name,
	}
	ctx := b.ch.SendMultiRequest(req)
	var out []interfaces.SwInterfaceDetails
	for {
		details := &interfaces.SwInterfaceDetails{}
		last, err := ctx.ReceiveReply(details)
		if err != nil {
			return nil, fmt.Errorf("ifacevpp: SwInterfaceDump(%q): %w", name, err)
		}
		if last {
			break
		}
		out = append(out, *details)
	}
	return out, nil
}

// ListInterfaces returns every interface currently known to VPP, converted
// to the iface.InterfaceInfo shape. When the VPP stats provider is
// available, each InterfaceInfo.Stats is populated from the stats segment
// so rate.go can compute RxPps/RxBps (dataplane-agnostic rate signal).
func (b *vppBackendImpl) ListInterfaces() ([]iface.InterfaceInfo, error) {
	details, err := b.dumpAllInterfaces()
	if err != nil {
		return nil, err
	}
	// A failed stats read leaves every Stats nil ("not read"); the interface
	// list itself is still the answer.
	statsMap, _ := readVPPIfaceStats() //nolint:errcheck // a nil map attaches no Stats, which is "not read"
	out := make([]iface.InterfaceInfo, 0, len(details))
	for i := range details {
		info := detailsToInfo(&details[i])
		if err := b.fillAddresses(&info, details[i].SwIfIndex); err != nil {
			return nil, fmt.Errorf("ifacevpp: list interfaces: %w", err)
		}
		attachVPPIfaceStats(&info, statsMap)
		out = append(out, info)
	}
	return out, nil
}

// errVPPStatsUnavailable is the read of a stats segment Ze is not connected
// to: no counters were read, which is not an interface without counters.
var errVPPStatsUnavailable = errors.New("ifacevpp: the VPP stats segment is not connected")

// readVPPIfaceStats reads per-interface counters from the VPP stats segment
// and returns them keyed by interface name. A stats segment Ze is not
// connected to answers errVPPStatsUnavailable, and a failed read its error:
// neither is an interface without counters.
func readVPPIfaceStats() (map[string]*iface.InterfaceStats, error) {
	sp := getActiveStatsProvider()
	if sp == nil {
		return nil, errVPPStatsUnavailable
	}
	var raw api.InterfaceStats
	if err := sp.GetInterfaceStats(&raw); err != nil {
		return nil, fmt.Errorf("ifacevpp: read the stats segment: %w", err)
	}
	out := make(map[string]*iface.InterfaceStats, len(raw.Interfaces))
	for i := range raw.Interfaces {
		name := raw.Interfaces[i].InterfaceName
		if name == "" {
			continue
		}
		out[name] = &iface.InterfaceStats{
			RxBytes:   raw.Interfaces[i].Rx.Bytes,
			RxPackets: raw.Interfaces[i].Rx.Packets,
			TxBytes:   raw.Interfaces[i].Tx.Bytes,
			TxPackets: raw.Interfaces[i].Tx.Packets,
			RxErrors:  raw.Interfaces[i].RxErrors,
			TxErrors:  raw.Interfaces[i].TxErrors,
			RxDropped: raw.Interfaces[i].Drops,
		}
	}
	return out, nil
}

// attachVPPIfaceStats sets info.Stats from statsMap. A stats read that failed
// leaves Stats nil, which InterfaceInfo.Stats documents as "not read", and the
// failure is answered by GetStats, which reads the segment for itself.
func attachVPPIfaceStats(info *iface.InterfaceInfo, statsMap map[string]*iface.InterfaceStats) {
	if s, ok := statsMap[info.Name]; ok {
		info.Stats = s
	}
}

// GetInterface returns the single interface matching name, with the
// addresses VPP holds on it. A failed address dump fails the read: the
// answer never reports an interface as holding no address when its
// addresses were not read.
func (b *vppBackendImpl) GetInterface(name string) (*iface.InterfaceInfo, error) {
	details, err := b.detailsByName(name)
	if err != nil {
		return nil, err
	}
	info := detailsToInfo(details)
	if err := b.fillAddresses(&info, details.SwIfIndex); err != nil {
		return nil, fmt.Errorf("ifacevpp: get %q: %w", name, err)
	}
	statsMap, _ := readVPPIfaceStats() //nolint:errcheck // a nil map attaches no Stats, which is "not read"
	attachVPPIfaceStats(&info, statsMap)
	return &info, nil
}

// GetLink returns the link facts of the single interface matching name and
// issues no address dump, so a failed address dump cannot fail it:
// Addresses is nil and AddressesRead false, as Backend.GetLink specifies.
func (b *vppBackendImpl) GetLink(name string) (*iface.InterfaceInfo, error) {
	details, err := b.detailsByName(name)
	if err != nil {
		return nil, err
	}
	info := detailsToInfo(details)
	statsMap, _ := readVPPIfaceStats() //nolint:errcheck // a nil map attaches no Stats, which is "not read"
	attachVPPIfaceStats(&info, statsMap)
	return &info, nil
}

// detailsByName returns the SwInterfaceDetails of the interface named name.
// The name is the VPP long name; SwInterfaceDump's NameFilter is
// substring-match, so the answer is re-checked for an exact InterfaceName
// match. An absent interface answers an error wrapping
// iface.ErrInterfaceNotFound.
func (b *vppBackendImpl) detailsByName(name string) (*interfaces.SwInterfaceDetails, error) {
	details, err := b.dumpByName(name)
	if err != nil {
		return nil, err
	}
	for i := range details {
		if trimCString(details[i].InterfaceName) == name {
			return &details[i], nil
		}
	}
	return nil, fmt.Errorf("ifacevpp: interface %q: %w", name, iface.ErrInterfaceNotFound)
}

// fillAddresses reads the addresses VPP holds on the interface at index into
// info and marks them read. SwInterfaceDetails carries no address, so each
// family is one IPAddressDump. VPP's address dump carries no Duplicate
// Address Detection outcome, so DADReported stays false: an address here
// with neither Tentative nor DADFailed says nothing about DAD.
func (b *vppBackendImpl) fillAddresses(info *iface.InterfaceInfo, index interface_types.InterfaceIndex) error {
	for _, isIPv6 := range [2]bool{false, true} {
		ctx := b.ch.SendMultiRequest(&ip.IPAddressDump{SwIfIndex: index, IsIPv6: isIPv6})
		// Bounded by VPP: the dump ends with its last-reply marker.
		for {
			details := &ip.IPAddressDetails{}
			last, err := ctx.ReceiveReply(details)
			if err != nil {
				return fmt.Errorf("IPAddressDump(sw_if_index %d, ipv6 %t): %w", index, isIPv6, err)
			}
			if last {
				break
			}
			addr, err := addrInfoOf(details.Prefix)
			if err != nil {
				return fmt.Errorf("IPAddressDump(sw_if_index %d): %w", index, err)
			}
			info.Addresses = append(info.Addresses, addr)
		}
	}
	info.AddressesRead = true
	return nil
}

// addrInfoOf converts one prefix of VPP's address dump. The address family
// byte comes from VPP, an open set: an unknown one is an error, never an
// address of a guessed family.
func addrInfoOf(p ip_types.AddressWithPrefix) (iface.AddrInfo, error) {
	var addr netip.Addr
	var family string
	switch p.Address.Af {
	case ip_types.ADDRESS_IP4:
		addr = netip.AddrFrom4(p.Address.Un.GetIP4())
		family = familyIPv4
	case ip_types.ADDRESS_IP6:
		addr = netip.AddrFrom16(p.Address.Un.GetIP6())
		family = familyIPv6
	default: // VPP's address family set is open; an unknown one reads no address
		return iface.AddrInfo{}, fmt.Errorf("address of unknown family %d", p.Address.Af)
	}
	return iface.AddrInfo{Address: addr.String(), PrefixLength: int(p.Len), Family: family}, nil
}

// GetMACAddress returns the L2 address of the named interface. Loopback/L3
// interfaces have a zero MAC; callers receive "00:00:00:00:00:00" rather than
// an error.
func (b *vppBackendImpl) GetMACAddress(name string) (string, error) {
	info, err := b.GetLink(name)
	if err != nil {
		return "", err
	}
	return info.MAC, nil
}

// LinkSpeedDuplex returns (0, ""): VPP owns its interfaces in userspace, so
// they have no /sys/class/net entry to read speed/duplex from.
func (b *vppBackendImpl) LinkSpeedDuplex(_ string) (int, string) { return 0, "" }

// SetMACAddress changes the L2 address of the named interface. The MAC is
// parsed from the Ethernet-standard colon form ("aa:bb:cc:dd:ee:ff"). VPP
// rejects multicast and broadcast MACs with a non-zero retval.
func (b *vppBackendImpl) SetMACAddress(ifaceName, mac string) error {
	idx, err := b.resolveIndex(ifaceName)
	if err != nil {
		return err
	}
	hw, err := net.ParseMAC(mac)
	if err != nil {
		return fmt.Errorf("ifacevpp: parse MAC %q: %w", mac, err)
	}
	if len(hw) != 6 {
		return fmt.Errorf("ifacevpp: MAC %q is not EUI-48", mac)
	}
	req := &interfaces.SwInterfaceSetMacAddress{SwIfIndex: idx}
	copy(req.MacAddress[:], hw)
	reply := &interfaces.SwInterfaceSetMacAddressReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("ifacevpp: SwInterfaceSetMacAddress: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("ifacevpp: SwInterfaceSetMacAddress retval=%d", reply.Retval)
	}
	return nil
}

// populateNameMap seeds the bidirectional name map from a full VPP interface
// dump. Called once during backend construction so SwIfIndex lookup succeeds
// for interfaces VPP already knows about (DPDK ports, local0, etc.).
// Idempotent: it overwrites existing entries.
//
// MUST be called with a live channel already acquired (ensureChannel has
// set chReady). Uses dumpAllRaw rather than dumpAllInterfaces because it is
// invoked from inside ensureChannel's populate.Do block, and dumpAllInterfaces
// would recurse back into ensureChannel.
func (b *vppBackendImpl) populateNameMap() error {
	details, err := b.dumpAllRaw()
	if err != nil {
		return err
	}
	for i := range details {
		vppName := trimCString(details[i].InterfaceName)
		if vppName == "" {
			continue
		}
		b.names.Add(vppName, uint32(details[i].SwIfIndex), vppName)
	}
	return nil
}

// detailsToInfo converts a VPP SwInterfaceDetails to the iface.InterfaceInfo
// shape. Admin state is derived from the ADMIN_UP flag; link state is
// surfaced via StartMonitor events since InterfaceInfo exposes only a
// single State field.
func detailsToInfo(d *interfaces.SwInterfaceDetails) iface.InterfaceInfo {
	state := iface.LinkStateDown
	if d.Flags&interface_types.IF_STATUS_API_FLAG_ADMIN_UP != 0 {
		state = iface.LinkStateUp
	}
	mtu := 0
	if len(d.Mtu) > 0 {
		mtu = int(d.Mtu[0])
	}
	name := trimCString(d.InterfaceName)
	return iface.InterfaceInfo{
		Name: name,
		// OsName is what the shared resolver reads back as the kernel device a
		// logical name resolved to (resolve.go bindingFromInfo). Leaving it
		// empty made every Resolve on this backend answer with an empty device,
		// so the by-name dispatch ops fell back to the logical name and an
		// os-name or mac/match selector steered nothing at all on VPP. The
		// netlink backend has always set it (show_linux.go).
		OsName:      name,
		Index:       int(d.SwIfIndex),
		Type:        d.Type.String(),
		State:       state,
		MTU:         mtu,
		MAC:         net.HardwareAddr(d.L2Address[:]).String(),
		ParentIndex: int(d.SupSwIfIndex),
		VlanID:      int(d.SubOuterVlanID),
	}
}

// trimCString strips embedded NUL padding VPP emits in fixed-length string
// fields.
func trimCString(s string) string {
	if before, _, ok := strings.Cut(s, "\x00"); ok {
		return before
	}
	return s
}

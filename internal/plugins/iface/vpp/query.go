// Design: docs/research/vpp-deployment-reference.md -- VPP interface query and MAC operations
// Overview: ifacevpp.go -- VPP Backend implementation
// Related: naming.go -- bidirectional name map fed by populateNameMap
// Related: monitor.go -- async interface event delivery

package ifacevpp

import (
	"fmt"
	"net"
	"strings"

	"go.fd.io/govpp/api"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"

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
	statsMap := readVPPIfaceStats()
	out := make([]iface.InterfaceInfo, 0, len(details))
	for i := range details {
		info := detailsToInfo(&details[i])
		if s, ok := statsMap[info.Name]; ok {
			info.Stats = s
		}
		out = append(out, info)
	}
	return out, nil
}

// readVPPIfaceStats reads per-interface counters from the VPP stats segment
// and returns them keyed by interface name. Returns an empty map if the
// stats provider is unavailable (graceful degradation).
func readVPPIfaceStats() map[string]*iface.InterfaceStats {
	sp := getActiveStatsProvider()
	if sp == nil {
		return nil
	}
	var raw api.InterfaceStats
	if err := sp.GetInterfaceStats(&raw); err != nil {
		return nil
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
	return out
}

// GetInterface returns the single interface matching name. The name is the
// VPP long name; SwInterfaceDump's NameFilter is substring-match, so we
// re-check for an exact InterfaceName match.
func (b *vppBackendImpl) GetInterface(name string) (*iface.InterfaceInfo, error) {
	details, err := b.dumpByName(name)
	if err != nil {
		return nil, err
	}
	statsMap := readVPPIfaceStats()
	for i := range details {
		if trimCString(details[i].InterfaceName) == name {
			info := detailsToInfo(&details[i])
			if s, ok := statsMap[info.Name]; ok {
				info.Stats = s
			}
			return &info, nil
		}
	}
	return nil, fmt.Errorf("ifacevpp: interface %q not found", name)
}

// GetMACAddress returns the L2 address of the named interface. Loopback/L3
// interfaces have a zero MAC; callers receive "00:00:00:00:00:00" rather than
// an error.
func (b *vppBackendImpl) GetMACAddress(name string) (string, error) {
	info, err := b.GetInterface(name)
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
	state := "down"
	if d.Flags&interface_types.IF_STATUS_API_FLAG_ADMIN_UP != 0 {
		state = "up"
	}
	mtu := 0
	if len(d.Mtu) > 0 {
		mtu = int(d.Mtu[0])
	}
	return iface.InterfaceInfo{
		Name:        trimCString(d.InterfaceName),
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

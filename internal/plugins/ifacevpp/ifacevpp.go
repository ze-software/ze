// Design: docs/research/vpp-deployment-reference.md -- VPP interface management via GoVPP
// Detail: naming.go -- ze name to VPP SwIfIndex bidirectional map
//
// ifacevpp implements iface.Backend for VPP via GoVPP binary API.
// Registered via iface.RegisterBackend("vpp", factory) in init().
// All Backend methods translate to GoVPP API calls.
package ifacevpp

import (
	"fmt"
	"net/netip"

	"go.fd.io/govpp/api"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	vppcomp "codeberg.org/thomas-mangin/ze/internal/component/vpp"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// vppBackendImpl implements iface.Backend using GoVPP.
type vppBackendImpl struct {
	ch    api.Channel
	names *nameMap
}

func newVPPBackend() (iface.Backend, error) {
	connector := vppcomp.GetActiveConnector()
	if connector == nil {
		return nil, fmt.Errorf("ifacevpp: VPP connector not available")
	}
	ch, err := connector.NewChannel()
	if err != nil {
		return nil, fmt.Errorf("ifacevpp: GoVPP channel: %w", err)
	}
	return &vppBackendImpl{
		ch:    ch,
		names: newNameMap(),
	}, nil
}

// errNotSupported returns a descriptive error for unsupported operations.
func errNotSupported(method string) error {
	return fmt.Errorf("ifacevpp: %s not supported on VPP backend", method)
}

// --- Interface lifecycle ---

func (b *vppBackendImpl) CreateDummy(name string) error {
	// VPP equivalent: create loopback interface.
	// TODO: call CreateLoopback via GoVPP, register name mapping.
	return errNotSupported("CreateDummy (pending GoVPP CreateLoopback wiring)")
}

func (b *vppBackendImpl) CreateVeth(_, _ string) error {
	return errNotSupported("CreateVeth (VPP uses memif/TAP, not veth)")
}

func (b *vppBackendImpl) CreateBridge(name string) error {
	// VPP equivalent: BridgeDomainAddDelV2.
	return errNotSupported("CreateBridge (pending GoVPP BridgeDomainAddDelV2 wiring)")
}

func (b *vppBackendImpl) CreateVLAN(parentName string, vlanID int) error {
	if vlanID < 1 || vlanID > 4094 {
		return fmt.Errorf("ifacevpp: VLAN ID %d out of range (1-4094)", vlanID)
	}
	// VPP equivalent: CreateSubif with dot1q.
	return errNotSupported("CreateVLAN (pending GoVPP CreateSubif wiring)")
}

func (b *vppBackendImpl) CreateTunnel(spec iface.TunnelSpec) error {
	// VPP equivalent depends on kind: VxlanAddDelTunnelV3, GreTunnelAddDel, IpipAddTunnel.
	return errNotSupported("CreateTunnel (pending GoVPP tunnel API wiring)")
}

func (b *vppBackendImpl) CreateWireguardDevice(_ string) error {
	return errNotSupported("CreateWireguardDevice (requires VPP wireguard plugin)")
}

func (b *vppBackendImpl) ConfigureWireguardDevice(_ iface.WireguardSpec) error {
	return errNotSupported("ConfigureWireguardDevice (requires VPP wireguard plugin)")
}

func (b *vppBackendImpl) GetWireguardDevice(_ string) (iface.WireguardSpec, error) {
	return iface.WireguardSpec{}, errNotSupported("GetWireguardDevice (requires VPP wireguard plugin)")
}

func (b *vppBackendImpl) DeleteInterface(name string) error {
	return errNotSupported("DeleteInterface (pending GoVPP delete wiring)")
}

// --- Address management ---

func (b *vppBackendImpl) AddAddress(ifaceName, cidr string) error {
	_, err := netip.ParsePrefix(cidr)
	if err != nil {
		return fmt.Errorf("ifacevpp: invalid CIDR %q: %w", cidr, err)
	}
	// VPP equivalent: SwInterfaceAddDelAddress with IsAdd=true.
	return errNotSupported("AddAddress (pending GoVPP SwInterfaceAddDelAddress wiring)")
}

func (b *vppBackendImpl) RemoveAddress(ifaceName, cidr string) error {
	_, err := netip.ParsePrefix(cidr)
	if err != nil {
		return fmt.Errorf("ifacevpp: invalid CIDR %q: %w", cidr, err)
	}
	return errNotSupported("RemoveAddress (pending GoVPP SwInterfaceAddDelAddress wiring)")
}

func (b *vppBackendImpl) ReplaceAddressWithLifetime(ifaceName, cidr string, _, _ int) error {
	return b.AddAddress(ifaceName, cidr)
}

// --- Route management ---

func (b *vppBackendImpl) AddRoute(_, _, _ string, _ int) error {
	return errNotSupported("AddRoute (pending GoVPP IPRouteAddDel wiring)")
}

func (b *vppBackendImpl) RemoveRoute(_, _, _ string, _ int) error {
	return errNotSupported("RemoveRoute (pending GoVPP IPRouteAddDel wiring)")
}

func (b *vppBackendImpl) ListRoutes(_, _ string) ([]iface.RouteInfo, error) {
	return nil, errNotSupported("ListRoutes (pending GoVPP IPRouteDump wiring)")
}

// --- Link state ---

func (b *vppBackendImpl) SetAdminUp(_ string) error {
	// VPP equivalent: SwInterfaceSetFlags with IF_STATUS_API_FLAG_ADMIN_UP.
	return errNotSupported("SetAdminUp (pending GoVPP SwInterfaceSetFlags wiring)")
}

func (b *vppBackendImpl) SetAdminDown(_ string) error {
	return errNotSupported("SetAdminDown (pending GoVPP SwInterfaceSetFlags wiring)")
}

// --- Interface properties ---

func (b *vppBackendImpl) SetMTU(_ string, mtu int) error {
	if mtu < 68 || mtu > 65535 {
		return fmt.Errorf("ifacevpp: MTU %d out of range (68-65535)", mtu)
	}
	return errNotSupported("SetMTU (pending GoVPP SwInterfaceSetMtu wiring)")
}

func (b *vppBackendImpl) SetMACAddress(_, _ string) error {
	return errNotSupported("SetMACAddress (pending GoVPP SwInterfaceSetMacAddress wiring)")
}

func (b *vppBackendImpl) GetMACAddress(_ string) (string, error) {
	return "", errNotSupported("GetMACAddress (pending GoVPP SwInterfaceDump wiring)")
}

func (b *vppBackendImpl) GetStats(_ string) (*iface.InterfaceStats, error) {
	return nil, errNotSupported("GetStats (pending GoVPP stats API wiring)")
}

// --- Query ---

func (b *vppBackendImpl) ListInterfaces() ([]iface.InterfaceInfo, error) {
	return nil, errNotSupported("ListInterfaces (pending GoVPP SwInterfaceDump wiring)")
}

func (b *vppBackendImpl) GetInterface(_ string) (*iface.InterfaceInfo, error) {
	return nil, errNotSupported("GetInterface (pending GoVPP SwInterfaceDump wiring)")
}

// --- Bridge operations ---

func (b *vppBackendImpl) BridgeAddPort(_, _ string) error {
	return errNotSupported("BridgeAddPort (pending GoVPP SwInterfaceSetL2Bridge wiring)")
}

func (b *vppBackendImpl) BridgeDelPort(_ string) error {
	return errNotSupported("BridgeDelPort (pending GoVPP SwInterfaceSetL2Bridge wiring)")
}

func (b *vppBackendImpl) BridgeSetSTP(_ string, _ bool) error {
	return errNotSupported("BridgeSetSTP (VPP STP support varies by version)")
}

// --- Traffic mirroring ---

func (b *vppBackendImpl) SetupMirror(_, _ string, _, _ bool) error {
	return errNotSupported("SetupMirror (pending GoVPP SpanEnableDisableL2 wiring)")
}

func (b *vppBackendImpl) RemoveMirror(_ string) error {
	return errNotSupported("RemoveMirror (pending GoVPP SpanEnableDisableL2 wiring)")
}

// --- Monitoring ---

func (b *vppBackendImpl) StartMonitor(_ ze.EventBus) error {
	return errNotSupported("StartMonitor (pending GoVPP WantInterfaceEvents wiring)")
}

func (b *vppBackendImpl) StopMonitor() {}

// --- Cleanup ---

func (b *vppBackendImpl) Close() error {
	b.ch.Close()
	return nil
}

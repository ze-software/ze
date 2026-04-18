// Design: docs/research/vpp-deployment-reference.md -- VPP interface management via GoVPP
// Detail: naming.go -- ze name to VPP SwIfIndex bidirectional map
// Detail: query.go -- ListInterfaces, GetInterface, Get/SetMACAddress
// Detail: monitor.go -- interface event delivery via WantInterfaceEvents
//
// ifacevpp implements iface.Backend for VPP via GoVPP binary API.
// Registered via iface.RegisterBackend("vpp", factory) in init().
// All Backend methods translate to GoVPP API calls.
package ifacevpp

import (
	"fmt"
	"log/slog"
	"net/netip"
	"sync"
	"sync/atomic"

	"go.fd.io/govpp/api"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/l2"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	vppcomp "codeberg.org/thomas-mangin/ze/internal/component/vpp"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// nextBridgeDomainID is an auto-incrementing bridge domain ID counter.
var nextBridgeDomainID atomic.Uint32

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	nextBridgeDomainID.Store(1) // BD 0 is reserved
	loggerPtr.Store(slogutil.DiscardLogger())
}

// vppBackendImpl implements iface.Backend using GoVPP.
//
// Channel acquisition is lazy. newVPPBackend does NOT dial VPP: the iface
// component loads backends during its OnConfigure phase, which can run
// before the vpp component finishes its GoVPP handshake. Eager dialing
// there would fail with "govpp: not connected" and poison the whole iface
// configuration. ensureChannel acquires the channel on the first method
// call that needs it, at which point vpp has had time to connect.
type vppBackendImpl struct {
	// chMu guards ch and chReady.
	chMu     sync.Mutex
	ch       api.Channel
	chReady  bool // ch acquired (may be nil in tests that inject directly)
	populate sync.Once

	names         *nameMap
	bridgeDomains map[string]uint32 // bridge name -> BD ID (separate from SwIfIndex space)

	// monMu guards mon; the pointer is only mutated under the lock.
	monMu sync.Mutex
	mon   *monitor
}

// getActiveConnector returns the VPP connector. Tests override this to
// inject connector state (nil, disconnected, or a ready mock).
var getActiveConnector = func() vppConnector {
	c := vppcomp.GetActiveConnector()
	if c == nil {
		return nil
	}
	return c
}

// vppConnector is the subset of vppcomp.Connector ifacevpp depends on,
// isolated so tests can supply fakes.
type vppConnector interface {
	IsConnected() bool
	NewChannel() (api.Channel, error)
}

func newVPPBackend() (iface.Backend, error) {
	// Do NOT acquire the GoVPP channel yet -- see the comment on
	// vppBackendImpl. ensureChannel handles first-use acquisition.
	return &vppBackendImpl{
		names:         newNameMap(),
		bridgeDomains: make(map[string]uint32),
	}, nil
}

// ensureChannel acquires the GoVPP channel on first use and seeds the name
// map. When the vpp component has not finished its handshake the call returns
// iface.ErrBackendNotReady without caching, so a later call (triggered by
// vppevents.EventConnected) can retry. Tests that inject a channel directly
// (ch set before first call) short-circuit the connector lookup.
func (b *vppBackendImpl) ensureChannel() error {
	b.chMu.Lock()
	if b.chReady {
		b.chMu.Unlock()
		b.populateOnce()
		return nil
	}
	// Test-injected channel: just mark ready.
	if b.ch != nil {
		b.chReady = true
		b.chMu.Unlock()
		b.populateOnce()
		return nil
	}
	b.chMu.Unlock()

	// Not ready? Return sentinel WITHOUT caching so the next call retries.
	// "Not ready" spans two cases: no connector registered yet (vpp plugin
	// not yet in OnStarted) and connector registered but GoVPP handshake
	// still in flight.
	connector := getActiveConnector()
	if connector == nil || !connector.IsConnected() {
		return fmt.Errorf("ifacevpp: VPP connector not ready: %w", iface.ErrBackendNotReady)
	}

	ch, err := connector.NewChannel()
	if err != nil {
		return fmt.Errorf("ifacevpp: GoVPP channel: %w", err)
	}

	b.chMu.Lock()
	// Another goroutine may have won the race while we were dialing.
	if b.chReady {
		b.chMu.Unlock()
		ch.Close()
	} else {
		b.ch = ch
		b.chReady = true
		b.chMu.Unlock()
	}

	b.populateOnce()
	return nil
}

// populateOnce seeds the name map exactly once after the channel is live.
// Failure is not fatal: a fresh VPP instance with no DPDK interfaces reports
// zero entries and CreateDummy / CreateVLAN add their own names.
func (b *vppBackendImpl) populateOnce() {
	b.populate.Do(func() {
		if err := b.populateNameMap(); err != nil {
			loggerPtr.Load().Warn("ifacevpp: populate name map", "err", err)
		}
	})
}

// errNotSupported returns a descriptive error for operations not available on VPP.
func errNotSupported(method string) error {
	return fmt.Errorf("ifacevpp: %s not supported on VPP backend", method)
}

// resolveIndex looks up the VPP SwIfIndex for a ze interface name. It also
// acts as the lazy-channel tripwire for every method that resolves a name
// before calling VPP: if the channel is not yet up, operations fail fast
// with a clear error instead of deref-ing a nil channel.
func (b *vppBackendImpl) resolveIndex(name string) (interface_types.InterfaceIndex, error) {
	if err := b.ensureChannel(); err != nil {
		return 0, err
	}
	idx, ok := b.names.LookupIndex(name)
	if !ok {
		return 0, fmt.Errorf("ifacevpp: unknown interface %q", name)
	}
	return interface_types.InterfaceIndex(idx), nil
}

// resolveBridgeDomain looks up the VPP bridge domain ID for a bridge name.
func (b *vppBackendImpl) resolveBridgeDomain(name string) (uint32, error) {
	if err := b.ensureChannel(); err != nil {
		return 0, err
	}
	bdID, ok := b.bridgeDomains[name]
	if !ok {
		return 0, fmt.Errorf("ifacevpp: unknown bridge domain %q", name)
	}
	return bdID, nil
}

// --- Interface lifecycle ---

func (b *vppBackendImpl) CreateDummy(name string) error {
	if err := b.ensureChannel(); err != nil {
		return err
	}
	req := &interfaces.CreateLoopback{}
	reply := &interfaces.CreateLoopbackReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("ifacevpp: CreateLoopback: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("ifacevpp: CreateLoopback retval=%d", reply.Retval)
	}
	b.names.Add(name, uint32(reply.SwIfIndex), name)
	return nil
}

func (b *vppBackendImpl) CreateVeth(_, _ string) error {
	return errNotSupported("CreateVeth (VPP uses memif/TAP, not veth)")
}

func (b *vppBackendImpl) CreateBridge(name string) error {
	if err := b.ensureChannel(); err != nil {
		return err
	}
	bdID := nextBridgeDomainID.Add(1) - 1
	req := &l2.BridgeDomainAddDelV2{
		BdID:    bdID,
		IsAdd:   true,
		Flood:   true,
		UuFlood: true,
		Forward: true,
		Learn:   true,
		BdTag:   name,
	}
	reply := &l2.BridgeDomainAddDelV2Reply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("ifacevpp: BridgeDomainAddDelV2: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("ifacevpp: BridgeDomainAddDelV2 retval=%d", reply.Retval)
	}
	b.bridgeDomains[name] = bdID
	return nil
}

func (b *vppBackendImpl) CreateVLAN(parentName string, vlanID int) error {
	if vlanID < 1 || vlanID > 4094 {
		return fmt.Errorf("ifacevpp: VLAN ID %d out of range (1-4094)", vlanID)
	}
	parentIdx, err := b.resolveIndex(parentName)
	if err != nil {
		return err
	}
	req := &interfaces.CreateVlanSubif{
		SwIfIndex: parentIdx,
		VlanID:    uint32(vlanID),
	}
	reply := &interfaces.CreateVlanSubifReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("ifacevpp: CreateVlanSubif: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("ifacevpp: CreateVlanSubif retval=%d", reply.Retval)
	}
	subName := fmt.Sprintf("%s.%d", parentName, vlanID)
	b.names.Add(subName, uint32(reply.SwIfIndex), subName)
	return nil
}

func (b *vppBackendImpl) CreateTunnel(_ iface.TunnelSpec) error {
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
	idx, err := b.resolveIndex(name)
	if err != nil {
		return err
	}
	// Try DeleteLoopback first (works for loopbacks).
	req := &interfaces.DeleteLoopback{SwIfIndex: idx}
	reply := &interfaces.DeleteLoopbackReply{}
	err = b.ch.SendRequest(req).ReceiveReply(reply)
	if err == nil && reply.Retval == 0 {
		b.names.Remove(name)
		return nil
	}
	// Fallback: try DeleteSubif (works for VLAN sub-interfaces).
	subReq := &interfaces.DeleteSubif{SwIfIndex: idx}
	subReply := &interfaces.DeleteSubifReply{}
	if subErr := b.ch.SendRequest(subReq).ReceiveReply(subReply); subErr != nil {
		return fmt.Errorf("ifacevpp: delete %s: loopback=%w, subif=%w", name, err, subErr)
	}
	if subReply.Retval != 0 {
		return fmt.Errorf("ifacevpp: delete %s: subif retval=%d", name, subReply.Retval)
	}
	b.names.Remove(name)
	return nil
}

// --- Address management ---

func (b *vppBackendImpl) AddAddress(ifaceName, cidr string) error {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return fmt.Errorf("ifacevpp: invalid CIDR %q: %w", cidr, err)
	}
	idx, err := b.resolveIndex(ifaceName)
	if err != nil {
		return err
	}
	req := &interfaces.SwInterfaceAddDelAddress{
		SwIfIndex: idx,
		IsAdd:     true,
		Prefix:    toAddressWithPrefix(prefix),
	}
	reply := &interfaces.SwInterfaceAddDelAddressReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("ifacevpp: AddAddress: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("ifacevpp: AddAddress retval=%d", reply.Retval)
	}
	return nil
}

func (b *vppBackendImpl) RemoveAddress(ifaceName, cidr string) error {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return fmt.Errorf("ifacevpp: invalid CIDR %q: %w", cidr, err)
	}
	idx, err := b.resolveIndex(ifaceName)
	if err != nil {
		return err
	}
	req := &interfaces.SwInterfaceAddDelAddress{
		SwIfIndex: idx,
		IsAdd:     false,
		Prefix:    toAddressWithPrefix(prefix),
	}
	reply := &interfaces.SwInterfaceAddDelAddressReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("ifacevpp: RemoveAddress: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("ifacevpp: RemoveAddress retval=%d", reply.Retval)
	}
	return nil
}

func (b *vppBackendImpl) ReplaceAddressWithLifetime(ifaceName, cidr string, _, _ int) error {
	// VPP does not support address lifetimes. Just add the address.
	return b.AddAddress(ifaceName, cidr)
}

// AddAddressP2P is not yet implemented on the VPP backend: PPP NCPs
// currently run only against netlink. A real VPP implementation would
// need an ip_address_add with the peer field populated.
func (b *vppBackendImpl) AddAddressP2P(_, _, _ string) error {
	return errNotSupported("AddAddressP2P (PPP NCP not supported on VPP backend yet)")
}

// --- Route management ---

func (b *vppBackendImpl) AddRoute(_, _, _ string, _ int) error {
	return errNotSupported("AddRoute (use fib-vpp plugin for route programming)")
}

func (b *vppBackendImpl) RemoveRoute(_, _, _ string, _ int) error {
	return errNotSupported("RemoveRoute (use fib-vpp plugin for route programming)")
}

func (b *vppBackendImpl) ListRoutes(_, _ string) ([]iface.RouteInfo, error) {
	return nil, errNotSupported("ListRoutes (use fib-vpp plugin for route queries)")
}

// --- Link state ---

func (b *vppBackendImpl) SetAdminUp(ifaceName string) error {
	idx, err := b.resolveIndex(ifaceName)
	if err != nil {
		return err
	}
	req := &interfaces.SwInterfaceSetFlags{
		SwIfIndex: idx,
		Flags:     interface_types.IF_STATUS_API_FLAG_ADMIN_UP,
	}
	reply := &interfaces.SwInterfaceSetFlagsReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("ifacevpp: SetAdminUp: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("ifacevpp: SetAdminUp retval=%d", reply.Retval)
	}
	return nil
}

func (b *vppBackendImpl) SetAdminDown(ifaceName string) error {
	idx, err := b.resolveIndex(ifaceName)
	if err != nil {
		return err
	}
	req := &interfaces.SwInterfaceSetFlags{
		SwIfIndex: idx,
		Flags:     0,
	}
	reply := &interfaces.SwInterfaceSetFlagsReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("ifacevpp: SetAdminDown: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("ifacevpp: SetAdminDown retval=%d", reply.Retval)
	}
	return nil
}

// --- Interface properties ---

func (b *vppBackendImpl) SetMTU(ifaceName string, mtu int) error {
	if mtu < 68 || mtu > 65535 {
		return fmt.Errorf("ifacevpp: MTU %d out of range (68-65535)", mtu)
	}
	idx, err := b.resolveIndex(ifaceName)
	if err != nil {
		return err
	}
	req := &interfaces.SwInterfaceSetMtu{
		SwIfIndex: idx,
		Mtu:       []uint32{uint32(mtu), uint32(mtu), uint32(mtu), uint32(mtu)},
	}
	reply := &interfaces.SwInterfaceSetMtuReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("ifacevpp: SetMTU: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("ifacevpp: SetMTU retval=%d", reply.Retval)
	}
	return nil
}

func (b *vppBackendImpl) GetStats(_ string) (*iface.InterfaceStats, error) {
	return nil, errNotSupported("GetStats (pending GoVPP stats API wiring)")
}

// --- Bridge operations ---

func (b *vppBackendImpl) BridgeAddPort(bridgeName, portName string) error {
	bdID, err := b.resolveBridgeDomain(bridgeName)
	if err != nil {
		return err
	}
	portIdx, err := b.resolveIndex(portName)
	if err != nil {
		return err
	}
	req := &l2.SwInterfaceSetL2Bridge{
		RxSwIfIndex: portIdx,
		BdID:        bdID,
		Enable:      true,
	}
	reply := &l2.SwInterfaceSetL2BridgeReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("ifacevpp: BridgeAddPort: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("ifacevpp: BridgeAddPort retval=%d", reply.Retval)
	}
	return nil
}

func (b *vppBackendImpl) BridgeDelPort(portName string) error {
	portIdx, err := b.resolveIndex(portName)
	if err != nil {
		return err
	}
	req := &l2.SwInterfaceSetL2Bridge{
		RxSwIfIndex: portIdx,
		Enable:      false,
	}
	reply := &l2.SwInterfaceSetL2BridgeReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("ifacevpp: BridgeDelPort: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("ifacevpp: BridgeDelPort retval=%d", reply.Retval)
	}
	return nil
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

// --- Cleanup ---

// Close drains any active monitor and releases the GoVPP channel. Caller
// MUST call Close when the backend is retired; LoadBackend in iface/backend.go
// invokes it on re-configuration. A backend that never acquired a channel
// (no method call ever succeeded) is safe to close: there is nothing to
// release on the VPP side.
func (b *vppBackendImpl) Close() error {
	b.StopMonitor()
	b.chMu.Lock()
	ch := b.ch
	b.ch = nil
	b.chMu.Unlock()
	if ch != nil {
		ch.Close()
	}
	return nil
}

// --- Helpers ---

// toAddressWithPrefix converts a Go netip.Prefix to VPP ip_types.AddressWithPrefix.
func toAddressWithPrefix(p netip.Prefix) ip_types.AddressWithPrefix {
	addr := p.Addr()
	if addr.Is4() {
		a4 := addr.As4()
		var ip4 ip_types.IP4Address
		copy(ip4[:], a4[:])
		return ip_types.AddressWithPrefix{
			Address: ip_types.Address{
				Af: ip_types.ADDRESS_IP4,
				Un: ip_types.AddressUnionIP4(ip4),
			},
			Len: uint8(p.Bits()),
		}
	}
	a16 := addr.As16()
	var ip6 ip_types.IP6Address
	copy(ip6[:], a16[:])
	return ip_types.AddressWithPrefix{
		Address: ip_types.Address{
			Af: ip_types.ADDRESS_IP6,
			Un: ip_types.AddressUnionIP6(ip6),
		},
		Len: uint8(p.Bits()),
	}
}

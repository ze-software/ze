// Design: docs/features/interfaces.md -- WireGuard on the VPP dataplane
// Overview: ifacevpp.go -- vppBackendImpl, wgPeers tracking map
//
// WireGuard on VPP is programmed through the wireguard plugin binary API. The
// netlink backend (wireguard_linux.go) creates the netdev via rtnetlink and
// then configures keys/port/peers via the wgctrl genetlink protocol. VPP's API
// is shaped differently and forces three design choices:
//
//  1. wireguard_interface_create takes the private key, listen port, and
//     underlay source in one message; there is no "set private key" update.
//     Backend.CreateWireguardDevice only receives the name (no key), so the
//     real interface creation happens in ConfigureWireguardDevice, which
//     carries the full WireguardSpec. CreateWireguardDevice is a no-op.
//  2. The plugin must be loaded at VPP start (plugin default { disable }); the
//     doctor check (doctor.go) and the vpp.plugins.wireguard startup.conf toggle
//     make that dependency visible instead of failing silently at apply.
//  3. This wireguard API revision (VPP 25.10 bindings) has no preshared-key
//     field on wireguard_peer, so a spec with a preshared key is rejected
//     rather than silently dropped (same honesty as the GRE-key rejection).

package ifacevpp

import (
	"fmt"
	"net/netip"

	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/wireguard"

	"github.com/ze-software/ze/internal/component/iface"
)

// CreateWireguardDevice is a deliberate no-op on VPP: wireguard_interface_create
// needs the private key, which this method is not given. The interface is
// created in ConfigureWireguardDevice (which has the full spec). Returning nil
// keeps the config-apply create-then-configure flow intact.
func (b *vppBackendImpl) CreateWireguardDevice(name string) error {
	if err := iface.ValidateIfaceName(name); err != nil {
		return fmt.Errorf("ifacevpp: create wireguard %q: %w", name, err)
	}
	if err := b.ensureChannel(); err != nil {
		return err
	}
	return nil
}

// ConfigureWireguardDevice applies the full desired state of the named
// wireguard interface. On first call it creates the VPP interface with the
// private key and listen port; subsequent calls reconcile the peer set with
// ReplacePeers semantics (remove the peers this backend last installed, then
// add the spec's peers).
func (b *vppBackendImpl) ConfigureWireguardDevice(spec iface.WireguardSpec) error {
	idx, err := b.ensureWireguardInterface(spec)
	if err != nil {
		return err
	}
	// ReplacePeers: drop the peers we installed last time before adding the
	// new set, so a peer removed from config is evicted from VPP.
	for _, peerIdx := range b.takeWireguardPeers(spec.Name) {
		r := &wireguard.WireguardPeerRemoveReply{}
		if err := b.ch.SendRequest(&wireguard.WireguardPeerRemove{PeerIndex: peerIdx}).ReceiveReply(r); err != nil {
			return fmt.Errorf("ifacevpp: wireguard %q peer remove: %w", spec.Name, err)
		}
		if r.Retval != 0 {
			return fmt.Errorf("ifacevpp: wireguard %q peer remove retval=%d", spec.Name, r.Retval)
		}
	}
	var installed []uint32
	for i := range spec.Peers {
		p := &spec.Peers[i]
		if p.Disable {
			continue
		}
		peerIdx, err := b.addWireguardPeer(spec.Name, idx, p)
		if err != nil {
			b.recordWireguardPeers(spec.Name, installed) // keep what we did add for teardown
			return err
		}
		installed = append(installed, peerIdx)
	}
	b.recordWireguardPeers(spec.Name, installed)
	return nil
}

// ensureWireguardInterface returns the SwIfIndex for spec.Name, creating the
// VPP wireguard interface on first use. The private key and listen port are set
// at create time (VPP has no update path). The underlay source address is left
// unspecified; VPP selects it via the FIB. A per-name deleter is recorded so
// DeleteInterface tears the interface down with wireguard_interface_delete.
func (b *vppBackendImpl) ensureWireguardInterface(spec iface.WireguardSpec) (interface_types.InterfaceIndex, error) {
	if err := b.ensureChannel(); err != nil {
		return 0, err
	}
	if existing, ok := b.names.lookupIndex(spec.Name); ok {
		return interface_types.InterfaceIndex(existing), nil
	}
	req := &wireguard.WireguardInterfaceCreate{
		GenerateKey: false,
		Interface: wireguard.WireguardInterface{
			UserInstance: ^uint32(0), // let VPP assign the instance number
			PrivateKey:   append([]byte(nil), spec.PrivateKey[:]...),
			Port:         spec.ListenPort,
		},
	}
	reply := &wireguard.WireguardInterfaceCreateReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return 0, fmt.Errorf("ifacevpp: WireguardInterfaceCreate %q: %w", spec.Name, err)
	}
	if reply.Retval != 0 {
		return 0, fmt.Errorf("ifacevpp: WireguardInterfaceCreate %q retval=%d", spec.Name, reply.Retval)
	}
	idx := reply.SwIfIndex
	b.names.Add(spec.Name, uint32(idx), spec.Name)
	name := spec.Name
	b.recordDeleter(name, func() error {
		r := &wireguard.WireguardInterfaceDeleteReply{}
		if err := b.ch.SendRequest(&wireguard.WireguardInterfaceDelete{SwIfIndex: idx}).ReceiveReply(r); err != nil {
			return fmt.Errorf("ifacevpp: WireguardInterfaceDelete %q: %w", name, err)
		}
		if r.Retval != 0 {
			return fmt.Errorf("ifacevpp: WireguardInterfaceDelete %q retval=%d", name, r.Retval)
		}
		b.takeWireguardPeers(name)
		return nil
	})
	return idx, nil
}

// addWireguardPeer installs one peer via wireguard_peer_add and returns the
// peer_index VPP assigns. A preshared key is rejected: this API revision has no
// field for it, so accepting it would silently weaken the tunnel.
func (b *vppBackendImpl) addWireguardPeer(ifaceName string, idx interface_types.InterfaceIndex, p *iface.WireguardPeerSpec) (uint32, error) {
	if p.HasPresharedKey {
		return 0, fmt.Errorf("ifacevpp: wireguard %q peer %q: preshared key not supported on VPP backend (wireguard_peer API has no field)", ifaceName, p.Name)
	}
	peer := wireguard.WireguardPeer{
		PublicKey:           append([]byte(nil), p.PublicKey[:]...),
		Port:                p.EndpointPort,
		PersistentKeepalive: p.PersistentKeepalive,
		SwIfIndex:           idx,
	}
	if p.EndpointIP != "" {
		endpoint, err := ip_types.ParseAddress(p.EndpointIP)
		if err != nil {
			return 0, fmt.Errorf("ifacevpp: wireguard %q peer %q endpoint %q: %w", ifaceName, p.Name, p.EndpointIP, err)
		}
		peer.Endpoint = endpoint
	}
	allowed, err := allowedIPsToPrefixes(p.AllowedIPs)
	if err != nil {
		return 0, fmt.Errorf("ifacevpp: wireguard %q peer %q: %w", ifaceName, p.Name, err)
	}
	peer.AllowedIps = allowed
	peer.NAllowedIps = uint8(len(allowed))

	reply := &wireguard.WireguardPeerAddReply{}
	if err := b.ch.SendRequest(&wireguard.WireguardPeerAdd{Peer: peer}).ReceiveReply(reply); err != nil {
		return 0, fmt.Errorf("ifacevpp: WireguardPeerAdd %q/%q: %w", ifaceName, p.Name, err)
	}
	if reply.Retval != 0 {
		return 0, fmt.Errorf("ifacevpp: WireguardPeerAdd %q/%q retval=%d", ifaceName, p.Name, reply.Retval)
	}
	return reply.PeerIndex, nil
}

// GetWireguardDevice reads the current VPP state for the named wireguard
// interface and maps it back to an iface.WireguardSpec (interface key/port plus
// the peer set), so reconciliation can compare desired against current.
func (b *vppBackendImpl) GetWireguardDevice(name string) (iface.WireguardSpec, error) {
	idx, err := b.resolveIndex(name)
	if err != nil {
		return iface.WireguardSpec{}, err
	}
	spec := iface.WireguardSpec{Name: name}

	ifCtx := b.ch.SendMultiRequest(&wireguard.WireguardInterfaceDump{SwIfIndex: idx, ShowPrivateKey: true})
	for {
		d := &wireguard.WireguardInterfaceDetails{}
		last, err := ifCtx.ReceiveReply(d)
		if err != nil {
			return iface.WireguardSpec{}, fmt.Errorf("ifacevpp: WireguardInterfaceDump %q: %w", name, err)
		}
		if last {
			break
		}
		spec.ListenPort = d.Interface.Port
		spec.ListenPortSet = d.Interface.Port != 0
		copy(spec.PrivateKey[:], d.Interface.PrivateKey)
	}

	peerCtx := b.ch.SendMultiRequest(&wireguard.WireguardPeersDump{PeerIndex: ^uint32(0)})
	for {
		d := &wireguard.WireguardPeersDetails{}
		last, err := peerCtx.ReceiveReply(d)
		if err != nil {
			return iface.WireguardSpec{}, fmt.Errorf("ifacevpp: WireguardPeersDump %q: %w", name, err)
		}
		if last {
			break
		}
		if d.Peer.SwIfIndex != idx {
			continue
		}
		ps := iface.WireguardPeerSpec{
			EndpointPort:        d.Peer.Port,
			PersistentKeepalive: d.Peer.PersistentKeepalive,
		}
		copy(ps.PublicKey[:], d.Peer.PublicKey)
		ps.Name = ps.PublicKey.String()
		if ep := d.Peer.Endpoint.ToIP(); ep != nil && !ep.IsUnspecified() {
			ps.EndpointIP = ep.String()
		}
		for i := range d.Peer.AllowedIps {
			ps.AllowedIPs = append(ps.AllowedIPs, d.Peer.AllowedIps[i].String())
		}
		spec.Peers = append(spec.Peers, ps)
	}
	return spec, nil
}

// allowedIPsToPrefixes converts CIDR strings into VPP ip_types.Prefix values.
func allowedIPsToPrefixes(cidrs []string) ([]ip_types.Prefix, error) {
	out := make([]ip_types.Prefix, 0, len(cidrs))
	for _, cidr := range cidrs {
		p, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, fmt.Errorf("allowed-ips %q: %w", cidr, err)
		}
		addr, err := ip_types.ParseAddress(p.Addr().String())
		if err != nil {
			return nil, fmt.Errorf("allowed-ips %q: %w", cidr, err)
		}
		out = append(out, ip_types.Prefix{Address: addr, Len: uint8(p.Bits())})
	}
	return out, nil
}

// recordWireguardPeers stores the installed peer indices for an interface.
func (b *vppBackendImpl) recordWireguardPeers(name string, indices []uint32) {
	b.wgMu.Lock()
	defer b.wgMu.Unlock()
	if b.wgPeers == nil {
		b.wgPeers = make(map[string][]uint32)
	}
	if len(indices) == 0 {
		delete(b.wgPeers, name)
		return
	}
	b.wgPeers[name] = indices
}

// takeWireguardPeers removes and returns the tracked peer indices for name.
func (b *vppBackendImpl) takeWireguardPeers(name string) []uint32 {
	b.wgMu.Lock()
	defer b.wgMu.Unlock()
	indices := b.wgPeers[name]
	delete(b.wgPeers, name)
	return indices
}

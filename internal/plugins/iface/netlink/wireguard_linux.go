// Design: docs/features/interfaces.md -- WireGuard netdev creation via netlink
// Overview: ifacenetlink.go -- package hub
// Related: backend_linux.go -- netlinkBackend type and Close()
// Related: tunnel_linux.go -- sibling Create* implementation (tunnel)

//go:build linux

package ifacenetlink

import (
	"fmt"
	"net"
	"time"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// CreateWireguardDevice creates a WireGuard netdev with the given name.
// The vendored netlink library exposes a Wireguard{} LinkType that carries
// only LinkAttrs (name, MTU, MAC, ...) -- all WireGuard-specific config
// (private-key, listen-port, fwmark, peers) goes through the wgctrl
// genetlink protocol in ConfigureWireguardDevice, not through rtnetlink.
//
// On LinkSetUp failure after a successful LinkAdd, the partial netdev is
// removed via LinkDel so the operator does not end up with a half-created
// interface after a transient error; rollback delete failures are logged.
func (b *netlinkBackend) CreateWireguardDevice(name string) error {
	if err := iface.ValidateIfaceName(name); err != nil {
		return fmt.Errorf("iface: create wireguard %q: %w", name, err)
	}

	link := &netlink.Wireguard{
		LinkAttrs: netlink.LinkAttrs{Name: name},
	}

	if err := netlink.LinkAdd(link); err != nil {
		return fmt.Errorf("iface: create wireguard %q: %w", name, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		if delErr := netlink.LinkDel(link); delErr != nil {
			loggerPtr.Load().Warn("iface: rollback delete after set-up failure",
				"name", name, "kind", "wireguard", "err", delErr)
		}
		return fmt.Errorf("iface: set up wireguard %q: %w", name, err)
	}
	return nil
}

// ConfigureWireguardDevice applies the given spec as the complete desired
// state of the wireguard netdev named in spec.Name. Private key, listen
// port, firewall mark, and the full peer set are set from the spec; any
// peers present in the kernel but not in spec.Peers are removed (the
// wgctrl Config is built with ReplacePeers: true).
//
// Phase 8's reconcile loop can compute a smaller per-peer diff and call
// ConfigureDevice with a narrower Config if the whole-spec replace turns
// out to be too coarse in practice. For now "apply entire spec" is the
// simplest correct behavior and each call is a single genetlink message.
func (b *netlinkBackend) ConfigureWireguardDevice(spec iface.WireguardSpec) error {
	client, err := wgctrl.New()
	if err != nil {
		return fmt.Errorf("iface: wgctrl client: %w", err)
	}
	defer func() { _ = client.Close() }()

	cfg, err := buildWireguardConfig(spec)
	if err != nil {
		return fmt.Errorf("iface: configure wireguard %q: %w", spec.Name, err)
	}

	if err := client.ConfigureDevice(spec.Name, cfg); err != nil {
		return fmt.Errorf("iface: configure wireguard %q: %w", spec.Name, err)
	}
	return nil
}

// GetWireguardDevice reads the current kernel state for the wireguard
// netdev named and translates it into an iface.WireguardSpec. Used by
// reconciliation to compare desired against current state, and by
// `ze init` when discovering an existing manually-created wg netdev.
//
// Private and preshared keys are copied into the returned spec as-is;
// callers must not log the returned Spec unless they have already
// redacted key material.
func (b *netlinkBackend) GetWireguardDevice(name string) (iface.WireguardSpec, error) {
	client, err := wgctrl.New()
	if err != nil {
		return iface.WireguardSpec{}, fmt.Errorf("iface: wgctrl client: %w", err)
	}
	defer func() { _ = client.Close() }()

	dev, err := client.Device(name)
	if err != nil {
		return iface.WireguardSpec{}, fmt.Errorf("iface: get wireguard %q: %w", name, err)
	}
	return deviceToSpec(dev), nil
}

// buildWireguardConfig translates an iface.WireguardSpec into a
// wgtypes.Config suitable for wgctrl.Client.ConfigureDevice. Returns an
// error only when a peer's AllowedIPs contain a malformed CIDR; every
// other field either has a valid zero value or is already validated at
// parse time.
func buildWireguardConfig(spec iface.WireguardSpec) (wgtypes.Config, error) {
	cfg := wgtypes.Config{
		PrivateKey:   &spec.PrivateKey,
		ReplacePeers: true,
	}

	if spec.ListenPortSet {
		port := int(spec.ListenPort)
		cfg.ListenPort = &port
	}

	// Zero fwmark means "unset" at the kernel level (0 disables marking).
	// wgctrl accepts a *int; we always set it so reloads with a cleared
	// mark actually clear the kernel state rather than leaving a stale
	// non-zero mark.
	fwmark := int(spec.FirewallMark)
	cfg.FirewallMark = &fwmark

	peers := make([]wgtypes.PeerConfig, 0, len(spec.Peers))
	for i := range spec.Peers {
		pc, err := buildPeerConfig(&spec.Peers[i])
		if err != nil {
			return wgtypes.Config{}, fmt.Errorf("peer %q: %w", spec.Peers[i].Name, err)
		}
		peers = append(peers, pc)
	}
	cfg.Peers = peers

	return cfg, nil
}

// buildPeerConfig translates one iface.WireguardPeerSpec into a
// wgtypes.PeerConfig. Disabled peers are translated to Remove: true so
// a reconcile that transitions a peer from enabled to disabled cleanly
// evicts it from the kernel peer set.
func buildPeerConfig(p *iface.WireguardPeerSpec) (wgtypes.PeerConfig, error) {
	pc := wgtypes.PeerConfig{
		PublicKey: p.PublicKey,
		Remove:    p.Disable,
	}

	if p.HasPresharedKey {
		pc.PresharedKey = &p.PresharedKey
	}

	if p.EndpointIP != "" {
		ip := net.ParseIP(p.EndpointIP)
		if ip == nil {
			return wgtypes.PeerConfig{}, fmt.Errorf("endpoint ip %q: invalid", p.EndpointIP)
		}
		pc.Endpoint = &net.UDPAddr{IP: ip, Port: int(p.EndpointPort)}
	}

	if len(p.AllowedIPs) > 0 {
		allowed := make([]net.IPNet, 0, len(p.AllowedIPs))
		for _, cidr := range p.AllowedIPs {
			_, ipnet, err := net.ParseCIDR(cidr)
			if err != nil {
				return wgtypes.PeerConfig{}, fmt.Errorf("allowed-ips %q: %w", cidr, err)
			}
			allowed = append(allowed, *ipnet)
		}
		pc.AllowedIPs = allowed
		pc.ReplaceAllowedIPs = true
	}

	if p.PersistentKeepalive > 0 {
		ka := time.Duration(p.PersistentKeepalive) * time.Second
		pc.PersistentKeepaliveInterval = &ka
	}

	return pc, nil
}

// deviceToSpec reverses buildWireguardConfig: it reads a kernel-reported
// *wgtypes.Device and produces an iface.WireguardSpec that round-trips
// through the parser. Used by GetWireguardDevice and (in a later phase)
// by ze init when capturing existing wg netdevs into config.
func deviceToSpec(dev *wgtypes.Device) iface.WireguardSpec {
	spec := iface.WireguardSpec{
		Name:          dev.Name,
		PrivateKey:    dev.PrivateKey,
		ListenPort:    uint16(dev.ListenPort), //nolint:gosec // kernel value
		ListenPortSet: dev.ListenPort != 0,
		FirewallMark:  uint32(dev.FirewallMark), //nolint:gosec // kernel value
	}

	for i := range dev.Peers {
		p := &dev.Peers[i]
		ps := iface.WireguardPeerSpec{
			Name:      p.PublicKey.String(),
			PublicKey: p.PublicKey,
		}
		if p.PresharedKey != (wgtypes.Key{}) {
			ps.PresharedKey = p.PresharedKey
			ps.HasPresharedKey = true
		}
		if p.Endpoint != nil {
			ps.EndpointIP = p.Endpoint.IP.String()
			ps.EndpointPort = uint16(p.Endpoint.Port) //nolint:gosec // kernel value
		}
		for _, cidr := range p.AllowedIPs {
			ps.AllowedIPs = append(ps.AllowedIPs, cidr.String())
		}
		if p.PersistentKeepaliveInterval > 0 {
			ps.PersistentKeepalive = uint16(p.PersistentKeepaliveInterval.Seconds()) //nolint:gosec // bounded
		}
		spec.Peers = append(spec.Peers, ps)
	}

	return spec
}

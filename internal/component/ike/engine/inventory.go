// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the registered IPsec inventory
// Related: reconcile.go -- PeerInfo, the per-session snapshot this file publishes
// Related: internal/core/ipsecinventory/registry.go -- the leaf registry and Tunnel
package engine

import (
	"net/netip"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/ipsecinventory"
)

// inventorySnapshot is the engine's answer to ipsecinventory.Tunnels: one Tunnel per
// active peer session, sorted by peer name, built from the same PeerInfo snapshot
// show vpn ipsec sa reads (PeerInfoMap takes peersMu, and Info takes each session's
// mu). A reader outside this component therefore sees exactly what the operator sees,
// including the NEGOTIATED transform rather than the configured one.
//
// It is registered from init() (register.go), so a build that links the engine
// answers the query and a build that does not answers ipsecinventory.ErrNotRegistered.
func inventorySnapshot() []ipsecinventory.Tunnel {
	infos := PeerInfoMap()
	out := make([]ipsecinventory.Tunnel, 0, len(infos))
	for name := range infos {
		info := infos[name]
		out = append(out, tunnelOf(&info))
	}
	slices.SortFunc(out, func(a, b ipsecinventory.Tunnel) int {
		return strings.Compare(a.Peer, b.Peer)
	})
	return out
}

// tunnelOf converts one PeerInfo into the boundary value type.
//
// A configured remote that is not an address literal (a DNS hostname, or "any" on a
// respond-only peer) leaves ConfiguredRemote invalid rather than carrying a sentinel;
// InstalledRemote is the address the tunnel really uses. The child fields are copied
// only while a Child SA exists, so a down tunnel carries the zero Mode and zero
// transforms, which no installed SA can produce.
func tunnelOf(info *PeerInfo) ipsecinventory.Tunnel {
	t := ipsecinventory.Tunnel{Peer: info.PeerName}
	if addr, err := netip.ParseAddr(info.RemoteAddress); err == nil {
		t.ConfiguredRemote = addr
	}
	if !info.HasChild {
		return t
	}
	t.Up = true
	t.InstalledRemote = info.ChildRemoteAddr
	t.InstalledLocal = info.ChildLocalAddr
	t.IfID = info.ChildIfID
	t.UDPEncap = info.ChildUDPEncap
	t.Mode = inventoryMode(info.ChildMode)
	t.EncryptionName = info.ESPEncryption
	t.IntegrityName = info.ESPIntegrity
	t.Encryption = ipsecinventory.EncryptionID(info.ESPEncryptionID)
	t.EncryptionKeyBits = info.ESPKeyBits
	t.Integrity = ipsecinventory.IntegrityID(info.ESPIntegrityID)
	return t
}

// inventoryMode maps the dataplane's installed mode onto the boundary enum. Both
// vocabularies make zero "unset", and an installed Child SA always carries one of
// the two real modes (createFirstChildSA decides it), so the default arm is reached
// only by a value no install path produces.
func inventoryMode(mode uint8) ipsecinventory.Mode {
	switch mode {
	case modeTransport:
		return ipsecinventory.ModeTransport
	case modeTunnel:
		return ipsecinventory.ModeTunnel
	default:
		return ipsecinventory.ModeUnspecified
	}
}

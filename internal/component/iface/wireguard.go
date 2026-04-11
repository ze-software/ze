// Design: docs/features/interfaces.md -- WireGuard interface types
// Related: config.go -- parseWireguardEntry populates WireguardSpec from YANG

package iface

import "golang.zx2c4.com/wireguard/wgctrl/wgtypes"

// WireguardKey is a 32-byte Curve25519 key (private, public, or preshared).
// Alias for wgtypes.Key so package consumers do not need to import wgctrl
// directly.
type WireguardKey = wgtypes.Key

// WireguardSpec carries the parsed configuration for a single WireGuard
// interface. Backends consume this struct via Backend.CreateWireguardDevice
// and Backend.ConfigureWireguardDevice.
//
// PrivateKey is mandatory and is parsed from the plaintext base64 string
// that the config parser leaves in the tree after $9$ sensitive-leaf decode.
// ListenPortSet distinguishes "operator set listen-port 0" (which wgctrl
// rejects) from "leaf absent, kernel picks an ephemeral port".
type WireguardSpec struct {
	Name          string
	PrivateKey    WireguardKey
	ListenPort    uint16
	ListenPortSet bool
	FirewallMark  uint32
	Peers         []WireguardPeerSpec
}

// WireguardPeerSpec carries the parsed configuration for a single WireGuard
// peer. PublicKey is mandatory; PresharedKey is optional. EndpointIP and
// EndpointPort are both empty / 0 when no endpoint block is configured,
// meaning the peer is passive until it reaches out to us. AllowedIPs is
// the cryptokey-routing prefix list; inbound packets whose source address
// is outside these prefixes are dropped.
type WireguardPeerSpec struct {
	Name                string
	PublicKey           WireguardKey
	PresharedKey        WireguardKey
	HasPresharedKey     bool
	EndpointIP          string
	EndpointPort        uint16
	AllowedIPs          []string
	PersistentKeepalive uint16
	Disable             bool
}

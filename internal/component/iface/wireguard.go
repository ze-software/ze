// Design: docs/features/interfaces.md -- WireGuard interface types

package iface

import "golang.zx2c4.com/wireguard/wgctrl/wgtypes"

// WireguardKey is a 32-byte Curve25519 key (private, public, or preshared).
// Alias for wgtypes.Key so package consumers do not need to import wgctrl
// directly.
type WireguardKey = wgtypes.Key

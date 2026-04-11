// Design: docs/features/interfaces.md -- WireGuard interface types
//
// Phase 1 scaffold for spec-iface-wireguard: anchors the
// golang.zx2c4.com/wireguard/wgctrl vendor drop so `go mod tidy` does not
// prune the dep before later phases add real usage. Later phases add
// WireguardSpec, WireguardPeerSpec, wireguardEntry, and parseWireguardEntry
// to this file.

package iface

import "golang.zx2c4.com/wireguard/wgctrl/wgtypes"

// WireguardKey is a 32-byte Curve25519 key (private, public, or preshared).
// Alias for wgtypes.Key so package consumers do not need to import wgctrl
// directly.
type WireguardKey = wgtypes.Key

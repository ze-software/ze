// Design: docs/architecture/core-design.md -- original-tree validator registration for VPP WireGuard plugin check
// Related: verify_wireguard.go -- verifyWireguardPluginTree implementation

package ifacevpp

import "github.com/ze-software/ze/internal/component/config"

// init registers the VPP WireGuard plugin tree validator so commit, reload,
// boot and `ze config validate` all refuse WireGuard interfaces under backend
// vpp when vpp.plugins.wireguard is not true.
func init() {
	if err := config.RegisterTreeValidator(wireguardTreeValidatorName, verifyWireguardPluginTree); err != nil {
		panic("BUG: iface-vpp: wireguard tree validator registration failed: " + err.Error())
	}
}

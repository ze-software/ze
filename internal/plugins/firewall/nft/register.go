package firewallnft

import "github.com/ze-software/ze/internal/component/firewall"

func init() {
	if err := firewall.RegisterBackend("nft", newBackend); err != nil {
		panic("BUG: firewallnft: registration failed: " + err.Error())
	}
	RegisterHealthCheck()
}

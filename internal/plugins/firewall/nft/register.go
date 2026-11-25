package firewallnft

import "github.com/ze-software/ze/internal/component/firewall"

// backendName is the name this backend registers under, which the firewall
// backend leaf selects.
const backendName = "kernel"

func init() {
	if err := firewall.RegisterBackend(backendName, newBackend); err != nil {
		panic("BUG: firewallnft: registration failed: " + err.Error())
	}
	RegisterHealthCheck()
}

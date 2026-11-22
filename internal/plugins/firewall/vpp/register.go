// Design: docs/architecture/firewall/fw-6-firewall-vpp.md -- Backend registration

package firewallvpp

import "github.com/ze-software/ze/internal/component/firewall"

func init() {
	if err := firewall.RegisterBackend("vpp", newBackend); err != nil {
		panic("BUG: firewallvpp: registration failed: " + err.Error())
	}
	if err := firewall.RegisterVerifier("vpp", Verify); err != nil {
		panic("BUG: firewallvpp: verifier registration failed: " + err.Error())
	}
}

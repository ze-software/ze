// Design: docs/architecture/appliance/docker-host.md -- executable aliases for kernel fixtures.

package dockerhost

import "testing"

// TestFunctionalToolsOverrideTheBusyboxAliases checks the guest's actual
// executable links. Kernel fixtures need full iproute2 and nftables, and the
// namespace harness needs libcap's tools before it starts the fixtures.
func TestFunctionalToolsOverrideTheBusyboxAliases(t *testing.T) {
	for link, name := range map[string]string{
		"/sbin/ip":         "ip",
		"/usr/sbin/nft":    "nft",
		"/usr/sbin/setcap": "setcap",
		"/usr/sbin/getcap": "getcap",
	} {
		want := shippedDir + "/" + name
		if got := shippedLinks[link]; got != want {
			t.Errorf("%s links to %q, want the shipped tool %q", link, got, want)
		}
	}
}

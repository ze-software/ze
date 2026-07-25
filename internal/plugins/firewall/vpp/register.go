// Design: plan/learned/671-fw-6-firewall-vpp.md -- Backend registration

package firewallvpp

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/firewall"
)

func init() {
	if err := firewall.RegisterBackend("vpp", newBackend); err != nil {
		fmt.Fprintf(os.Stderr, "firewallvpp: registration failed: %v\n", err)
		os.Exit(1)
	}
	if err := firewall.RegisterVerifier("vpp", Verify); err != nil {
		fmt.Fprintf(os.Stderr, "firewallvpp: verifier registration failed: %v\n", err)
		os.Exit(1)
	}
}

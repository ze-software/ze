// Design: docs/architecture/core-design.md -- Firewall plugin registration
// Related: engine.go -- runEngine entry point driven by this registration

package firewall

import (
	"fmt"
	"os"

	firewallschema "codeberg.org/thomas-mangin/ze/internal/component/firewall/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

func init() { //nolint:gochecknoinits // plugin registration
	reg := registry.Registration{
		Name:                    "firewall",
		Description:             "Packet filter and NAT rules (nftables on Linux)",
		Features:                "yang",
		YANG:                    firewallschema.ZeFirewallConfYANG,
		ConfigRoots:             []string{configRootFirewall},
		InProcessConfigVerifier: VerifyConfig,
		RunEngine:               runEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(_ []string) int {
		return 1
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "firewall: registration failed: %v\n", err)
		os.Exit(1)
	}
}

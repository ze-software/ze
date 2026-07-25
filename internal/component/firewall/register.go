// Design: docs/architecture/core-design.md -- Firewall plugin registration
// Related: engine.go -- runEngine entry point driven by this registration

package firewall

import (
	"fmt"
	"os"
	"sync"

	firewallyang "github.com/ze-software/ze/internal/component/firewall/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/ze"
)

var (
	eventBusMu  sync.Mutex
	eventBusRef ze.EventBus
)

func setEventBusRef(eb ze.EventBus) {
	eventBusMu.Lock()
	defer eventBusMu.Unlock()
	eventBusRef = eb
}

func getEventBusRef() ze.EventBus {
	eventBusMu.Lock()
	defer eventBusMu.Unlock()
	return eventBusRef
}

func init() { //nolint:gochecknoinits // plugin registration
	reg := registry.Registration{
		Name:                    "firewall",
		Description:             "Packet filter and NAT rules (nftables on Linux)",
		Features:                "yang",
		YANG:                    firewallyang.ZeFirewallConfYANG,
		ConfigRoots:             []string{configRootFirewall},
		InProcessConfigVerifier: VerifyConfig,
		RunEngine:               runEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			setEventBusRef(eb)
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

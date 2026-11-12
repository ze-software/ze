// Design: docs/architecture/core-design.md -- Firewall plugin registration
// Related: engine.go -- runEngine entry point driven by this registration

package firewall

import (
	"fmt"
	"os"
	"sync"

	firewallyang "github.com/ze-software/ze/internal/component/firewall/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
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

// vppPlugin is the registration name of the VPP component
// (internal/component/vpp), which the VPP firewall backend
// (internal/plugins/firewall/vpp) drives. backendVPP is the backend leaf value
// that selects that backend. The two spellings match, but one names a plugin
// and the other a value of the leaf. The optional edge orders firewall after
// vpp whenever both run, as the StartAfter it replaces did, and the config
// need pulls vpp only when the leaf selects it. In a build without ze_vpp that
// value is refused at validate, commit and boot, before LoadBackend would fail
// at apply.
const (
	vppPlugin  = "vpp"
	backendVPP = "vpp"
)

func init() { //nolint:gochecknoinits // plugin registration
	// The helper tables are this engine's own, registered under
	// conntrackHelperOwner; an operator table of the same name would merge
	// into them (reservation.go).
	MustReserveTableNames(conntrackHelperOwner, TableControl{
		Plugin:  "firewall component's conntrack helpers",
		Decides: "the helper modules of the system conntrack configuration",
		Command: "show firewall ruleset conntrack_helpers",
	}, conntrackHelperTableInet, conntrackHelperTableIP)

	reg := registry.Registration{
		Name:        "firewall",
		Description: "Packet filter and NAT rules (nftables on Linux)",
		Features:    "yang",
		YANG:        firewallyang.ZeFirewallConfYANG,
		// `system conntrack` is the second root: its helpers lower to nft
		// tables, so a config that lists one starts this engine with no
		// `firewall {}` block, and the transaction verifies, applies and rolls
		// back the helper tables with the firewall tables (engine.go).
		ConfigRoots:          []string{configRootFirewall, configRootConntrack},
		OptionalDependencies: []string{vppPlugin},
		ConfigNeeds: []registry.ConfigNeed{{
			Path:      []string{"firewall", "backend"},
			Value:     backendVPP,
			Providers: []string{vppPlugin},
		}},
		InProcessConfigVerifier: VerifyConfig,
		RunEngine:               runEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			setEventBusRef(eb)
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			bindMetrics(reg)
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

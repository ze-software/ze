// Design: docs/architecture/core-design.md -- FlowSpec-to-firewall bridge registration
// Related: engine.go -- runEngine entry point driven by this registration

package flowspecfirewall

import (
	"sync"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/ze"
)

func flowspecFirewallSends() []rpc.MessageSendDecl {
	return []rpc.MessageSendDecl{
		{Message: ribevents.Namespace + ":" + ribevents.EventReplayRequest},
	}
}

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
		Name:         "flowspec-firewall",
		Description:  "Translates BGP FlowSpec routes into nftables firewall rules",
		Dependencies: []string{"firewall", "bgp-rib"},
		DataPlaneFeatures: []registry.DataPlaneFeature{{
			ConfigPath:      "flowspec-firewall",
			Why:             "flowspec-firewall registers firewall tables",
			FollowsSelector: "firewall",
			InUseKind:       registry.DataPlaneInUseSelectedPlugin,
		}},
		Sends:     flowspecFirewallSends(),
		RunEngine: runEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			setEventBusRef(eb)
		},
		ConfigureMetrics: bindMetrics,
	}
	reg.CLIHandler = func(_ []string) int {
		return 1
	}
	if err := registry.Register(reg); err != nil {
		panic("BUG: flowspec-firewall: registration failed: " + err.Error())
	}
}

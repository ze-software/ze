// Design: docs/architecture/api/architecture.md -- filter-owner bgp:state requirement.
// Related: bus_requirements.go -- checkFilterOwnerStateRequirements.
package bgpconfig

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const inserterProcess = "fake-inserter"

// registerStateInserter registers a test filter plugin that declares bgp:state
// reception so checkFilterOwnerStateRequirements validates state delivery for
// any peer carrying one of its filter members.
func registerStateInserter(t *testing.T, insertions ...rpc.FilterInsertion) {
	t.Helper()
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	t.Cleanup(configyang.RegisterModuleForTest("ze-test-inserter-conf.yang", insertionTestYANG))
	registry.SetRuntimeLoadedPlugins(func() []string { return []string{inserterProcess} })
	t.Cleanup(func() { registry.SetRuntimeLoadedPlugins(nil) })
	require.NoError(t, registry.Register(registry.Registration{
		YANG:        insertionTestYANG,
		Name:        inserterProcess,
		Description: "test state inserter",
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		ConfigRoots: []string{"bgp"},
		Receives: []rpc.MessageReceiveDecl{
			{Message: bgpevents.Namespace + ":" + bgpevents.EventState},
		},
		Filters: []rpc.FilterDecl{
			{Name: "early"},
			{Name: "late"},
		},
		FilterInsertions: insertions,
	}))
}

// TestFilterOwnerStateRequirement verifies AC-14 and S35: a plugin declaring
// bgp:state whose filter member sits on a peer's chain (inserted or operator-named)
// receives bgp:state by default, and any bus override removing that reception is
// refused with a typed operational fix.
//
// VALIDATES: checkFilterOwnerStateRequirements enforces bgp:state delivery to
// active filter owners without mutating written config.
// PREVENTS: silently dropping peer removal notifications for stateful filter plugins.
func TestFilterOwnerStateRequirement(t *testing.T) {
	t.Run("inserted member receives state by default and refuses override removing state", func(t *testing.T) {
		registerStateInserter(t, rpc.FilterInsertion{
			Filter:    "fake-inserter:late",
			Direction: rpc.FilterImport,
			Priority:  5,
			Config:    rrClient,
			Values:    []string{"on"},
		})

		defaultCfg := formatFilterStateCfg("", "", "")
		graph := graphFromConfig(t, defaultCfg)
		require.Contains(t, fedBy(graph, bgpevents.EventState, events.DirUnspecified, "10.0.0.2"), inserterProcess)

		cutCfg := formatFilterStateCfg(
			`bus { endpoint fake-inserter { internal fake-inserter; receive { allow none; } } }`,
			"",
			"",
		)
		_, err := config.LoadConfig(cutCfg, "", nil)
		require.Error(t, err, "cutting bgp:state from an active inserted filter owner must be refused")
		var oe *opererr.Error
		require.ErrorAs(t, err, &oe)
		require.Contains(t, err.Error(), inserterProcess)
		require.NotEmpty(t, oe.Fix)
	})

	t.Run("operator-named member refuses peer send block omitting state", func(t *testing.T) {
		registerStateInserter(t)

		peerFilter := `filter { export { filters [ fake-inserter:early ]; final accept; } }`
		okCfg := formatFilterStateCfg("", peerFilter, "")
		graph := graphFromConfig(t, okCfg)
		require.Contains(t, fedBy(graph, bgpevents.EventState, events.DirUnspecified, "10.0.0.2"), inserterProcess)

		cutPeerA := peerFilter + ` bus { send { allow [ bgp:update ]; } }`
		_, err := config.LoadConfig(formatFilterStateCfg("", cutPeerA, ""), "", nil)
		require.Error(t, err, "cutting bgp:state on a peer naming fake-inserter:early must be refused")
		var oe *opererr.Error
		require.ErrorAs(t, err, &oe)
		require.Contains(t, err.Error(), "A")
		require.NotEmpty(t, oe.Fix)
	})

	t.Run("peer without filter member may cut state freely", func(t *testing.T) {
		registerStateInserter(t)

		peerFilter := `filter { export { filters [ fake-inserter:early ]; final accept; } }`
		cutPeerB := `bus { send { allow none; } }`
		_, err := config.LoadConfig(formatFilterStateCfg("", peerFilter, cutPeerB), "", nil)
		require.NoError(t, err, "peer B carries no member of fake-inserter and is unconstrained")
	})
}

func formatFilterStateCfg(busBlock, peerAExtra, peerBExtra string) string {
	return busBlock + `
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    peer A {
        connection { remote { ip 10.0.0.2; } local { ip 10.0.0.1; } }
        session { asn { local 65000; remote 65000; } }
        insertion-test { mode on; }
        ` + peerAExtra + `
    }
    peer B {
        connection { remote { ip 10.0.0.3; } local { ip 10.0.0.1; } }
        session { asn { local 65000; remote 65001; } }
        ` + peerBExtra + `
    }
}`
}

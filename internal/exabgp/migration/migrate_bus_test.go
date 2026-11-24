// Design: docs/architecture/exabgp-bridge.md -- unit tests for bus endpoint migration (S17, S38, S40, AC-12, AC-41, AC-43)
// Related: migrate_bus.go -- bus endpoint builder, Q-E4 event mapping, and capability flags
// Related: migrate_process_name.go -- endpoint name sanitization

package migration

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/env"

	_ "github.com/ze-software/ze/internal/component/bgp/plugin"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rib"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/watchdog"
	_ "github.com/ze-software/ze/internal/component/bgp/yang"
	_ "github.com/ze-software/ze/internal/component/hub/yang"
	_ "github.com/ze-software/ze/internal/component/plugin/yang"
)

func parseAndValidateMigrated(t *testing.T, output string) *config.Tree {
	t.Helper()
	reparsed, err := config.ParseTreeWithYANG(output, nil)
	require.NoError(t, err, "config.ParseTreeWithYANG refused migrated output:\n%s", output)
	require.NoError(t, config.ValidateBusConfig(reparsed), "config.ValidateBusConfig refused migrated output:\n%s", output)
	return reparsed
}

func migrateAndValidate(t *testing.T, source string) (string, *MigrateResult, *config.Tree) {
	t.Helper()
	tree, err := ParseExaBGPConfig(source)
	require.NoError(t, err, "ParseExaBGPConfig")
	result, err := MigrateFromExaBGP(tree)
	require.NoError(t, err, "MigrateFromExaBGP")
	output := SerializeTree(result.Tree)
	reparsed := parseAndValidateMigrated(t, output)
	return output, result, reparsed
}

// TestMigrateWritesScriptEndpoints verifies that ExaBGP process migration emits
// Ze `bus { endpoint <name> { ... } }` blocks with no retired `exabgp`,
// `plugin`, or `attach process` syntax, and that every output validates cleanly
// through config.ParseTreeWithYANG and config.ValidateBusConfig.
//
// VALIDATES: S17, S38, S40, AC-12, AC-33, AC-41, AC-43, Q-E4, Q-E11.
// PREVENTS: Emitting retired bridge/plugin/attach syntax, leaking --respawn or
// --on-exit onto the wrapper run line, losing per-neighbor forms/directions, or
// silently merging conflicting ADD-PATH modes across neighbors.
func TestMigrateWritesScriptEndpoints(t *testing.T) {
	t.Run("S17_two_processes_json_and_unfiltered", func(t *testing.T) {
		const source = `
process p1 {
	run ./p1.py;
	encoder json;
}
process p2 {
	run ./p2.py;
}
neighbor 192.0.2.1 {
	description "peerA";
	router-id 1.1.1.1;
	local-as 65001;
	peer-as 65002;
	api {
		processes [ p1 ];
		receive {
			update;
		}
	}
}
neighbor 192.0.2.2 {
	description "peerB";
	router-id 1.1.1.1;
	local-as 65001;
	peer-as 65003;
	api {
		processes [ p2 ];
	}
}
`
		output, result, reparsed := migrateAndValidate(t, source)
		assert.True(t, result.RIBInjected)
		assert.NotContains(t, output, "exabgp {")
		assert.NotContains(t, output, "plugin {")
		assert.NotContains(t, output, "attach process")
		assert.NotContains(t, output, "exabgp-bridge")

		endpoints := reparsed.GetContainer("bus").GetList("endpoint")
		require.NotNil(t, endpoints["bgp-rib"])
		ribInternal, _ := endpoints["bgp-rib"].Get("internal")
		assert.Equal(t, "bgp-rib", ribInternal)

		require.NotNil(t, endpoints["bgp-watchdog"])
		wdInternal, _ := endpoints["bgp-watchdog"].Get("internal")
		assert.Equal(t, "bgp-watchdog", wdInternal)

		p1 := endpoints["p1"]
		require.NotNil(t, p1)
		p1Run, _ := p1.GetContainer("external").Get("run")
		assert.Equal(t, "ze exabgp plugin --encoder json ./p1.py", p1Run)
		p1Recv := p1.GetContainer("receive")
		require.NotNil(t, p1Recv)
		assert.Equal(t, []string{"/bgp/group/default/peer/peerA"}, p1Recv.GetSlice("from"))
		assert.Equal(t, []string{"bgp:update"}, p1Recv.GetSlice("allow"))
		p1Upd := p1Recv.GetList("message")["bgp:update"]
		require.NotNil(t, p1Upd)
		assert.Empty(t, p1Upd.GetSlice("from"))
		p1Dir := p1Upd.GetList("tag")["direction"]
		require.NotNil(t, p1Dir)
		assert.Equal(t, []string{"received"}, p1Dir.GetSlice("value"))
		p1Send := p1.GetContainer("send")
		require.NotNil(t, p1Send)
		assert.Equal(t, []string{"/bgp/group/default/peer/peerA"}, p1Send.GetSlice("to"))

		p2 := endpoints["p2"]
		require.NotNil(t, p2)
		p2Run, _ := p2.GetContainer("external").Get("run")
		assert.Equal(t, "ze exabgp plugin ./p2.py", p2Run)
		p2Recv := p2.GetContainer("receive")
		require.NotNil(t, p2Recv)
		assert.Equal(t, []string{"none"}, p2Recv.GetSlice("from"))
		assert.Empty(t, p2Recv.GetSlice("allow"))
		p2Send := p2.GetContainer("send")
		require.NotNil(t, p2Send)
		assert.Equal(t, []string{"/bgp/group/default/peer/peerB"}, p2Send.GetSlice("to"))
	})

	t.Run("S38_on_exit_respawn_and_api_v4_keep", func(t *testing.T) {
		const source = `
process k {
	run ./k.py;
	on-exit keep;
}
process w {
	run ./w.py;
	on-exit withdraw;
}
process v4 {
	run ./v4.py;
}
process r {
	run ./r.py;
	respawn false;
}
neighbor 192.0.2.1 {
	router-id 1.1.1.1;
	local-as 65001;
	peer-as 65002;
	api {
		processes [ k w v4 r ];
	}
}
`
		// Default API version (auto/6): only explicit `on-exit keep` emits on-exit keep.
		output, _, reparsed := migrateAndValidate(t, source)
		assert.NotContains(t, output, "--on-exit")
		assert.NotContains(t, output, "--respawn")

		endpoints := reparsed.GetContainer("bus").GetList("endpoint")
		kOnExit, ok := endpoints["k"].GetContainer("external").Get("on-exit")
		assert.True(t, ok)
		assert.Equal(t, "keep", kOnExit)

		_, wHasOnExit := endpoints["w"].GetContainer("external").Get("on-exit")
		assert.False(t, wHasOnExit, "on-exit withdraw is the default and must be omitted")

		_, v4HasOnExit := endpoints["v4"].GetContainer("external").Get("on-exit")
		assert.False(t, v4HasOnExit, "unset on-exit under default API version must be omitted")

		rRespawn, ok := endpoints["r"].GetContainer("external").Get("respawn")
		assert.True(t, ok)
		assert.Equal(t, "false", rRespawn)

		// API version 4 via OS env: unset on-exit emits `on-exit keep`, explicit withdraw still omitted.
		t.Run("api_v4_env", func(t *testing.T) {
			t.Setenv("exabgp_api_version", "4")
			env.ResetCache()
			t.Cleanup(env.ResetCache)

			outputV4, _, reparsedV4 := migrateAndValidate(t, source)
			assert.NotContains(t, outputV4, "--on-exit")
			assert.NotContains(t, outputV4, "--respawn")

			epV4 := reparsedV4.GetContainer("bus").GetList("endpoint")
			v4OnExit, ok := epV4["v4"].GetContainer("external").Get("on-exit")
			assert.True(t, ok, "API version 4 with unset on-exit must emit external { on-exit keep }")
			assert.Equal(t, "keep", v4OnExit)

			_, wV4HasOnExit := epV4["w"].GetContainer("external").Get("on-exit")
			assert.False(t, wV4HasOnExit, "explicit on-exit withdraw must stay omitted under API version 4")
		})

		// API version 4 via tree environment container (with env restored).
		tree, err := ParseExaBGPConfig(source)
		require.NoError(t, err)
		tree.GetOrCreateContainer("environment").GetOrCreateContainer("exabgp").GetOrCreateContainer("api").Set("version", "4")
		resTreeV4, err := MigrateFromExaBGP(tree)
		require.NoError(t, err)
		reparsedTreeV4 := parseAndValidateMigrated(t, SerializeTree(resTreeV4.Tree))
		treeV4OnExit, ok := reparsedTreeV4.GetContainer("bus").GetList("endpoint")["v4"].GetContainer("external").Get("on-exit")
		assert.True(t, ok)
		assert.Equal(t, "keep", treeV4OnExit)
	})

	t.Run("AC12_per_neighbor_forms_message_subsets_and_sanitization", func(t *testing.T) {
		const source = `
process watch_routes.1 {
	run ./p1.py;
	encoder json;
}
neighbor 192.0.2.1 {
	description "peerA";
	router-id 1.1.1.1;
	local-as 65001;
	peer-as 65002;
	api {
		processes [ watch_routes.1 ];
		receive {
			packets;
			update;
		}
	}
}
neighbor 192.0.2.2 {
	description "peerB";
	router-id 1.1.1.1;
	local-as 65001;
	peer-as 65003;
	api {
		processes [ watch_routes.1 ];
		neighbor-changes;
	}
}
`
		_, result, reparsed := migrateAndValidate(t, source)

		// Verify deterministic rename and migration warning.
		require.NoError(t, ipc.CheckPluginName("watch-routes-1"))
		warned := false
		for _, w := range result.Warnings {
			if strings.Contains(w, "watch_routes.1") && strings.Contains(w, "watch-routes-1") {
				warned = true
			}
		}
		assert.True(t, warned, "expected rename warning for watch_routes.1 -> watch-routes-1, got %v", result.Warnings)

		ep := reparsed.GetContainer("bus").GetList("endpoint")["watch-routes-1"]
		require.NotNil(t, ep)

		runLine, _ := ep.GetContainer("external").Get("run")
		assert.Equal(t, "ze exabgp plugin --encoder json --form 192.0.2.1:receive-packets ./p1.py", runLine)

		recv := ep.GetContainer("receive")
		require.NotNil(t, recv)
		assert.Equal(t, []string{"/bgp/group/default/peer/peerA", "/bgp/group/default/peer/peerB"}, recv.GetSlice("from"))
		assert.Equal(t, []string{"bgp:connected", "bgp:state", "bgp:update"}, recv.GetSlice("allow"))

		updMsg := recv.GetList("message")["bgp:update"]
		require.NotNil(t, updMsg)
		assert.Equal(t, []string{"/bgp/group/default/peer/peerA"}, updMsg.GetSlice("from"))
		require.NotNil(t, updMsg.GetList("tag")["direction"])
		assert.Equal(t, []string{"received"}, updMsg.GetList("tag")["direction"].GetSlice("value"))

		stateMsg := recv.GetList("message")["bgp:state"]
		require.NotNil(t, stateMsg)
		assert.Equal(t, []string{"/bgp/group/default/peer/peerB"}, stateMsg.GetSlice("from"))
		assert.Empty(t, stateMsg.GetList("tag"))

		connMsg := recv.GetList("message")["bgp:connected"]
		require.NotNil(t, connMsg)
		assert.Equal(t, []string{"/bgp/group/default/peer/peerB"}, connMsg.GetSlice("from"))
		assert.Empty(t, connMsg.GetList("tag"))

		send := ep.GetContainer("send")
		require.NotNil(t, send)
		assert.Equal(t, []string{"/bgp/group/default/peer/peerA", "/bgp/group/default/peer/peerB"}, send.GetSlice("to"))
	})

	t.Run("AC43_virtual_session_capabilities_and_add_path_conflict", func(t *testing.T) {
		const source = `
process s1 {
	run ./s1.py;
}
process s2 {
	run ./s2.py;
}
neighbor 192.0.2.1 {
	description "edge1";
	router-id 1.1.1.1;
	local-as 65001;
	peer-as 65002;
	family {
		ipv4 unicast;
		ipv6 unicast;
	}
	capability {
		route-refresh;
		add-path send/receive;
	}
	api {
		processes [ s1 s2 ];
	}
}
`
		_, _, reparsed := migrateAndValidate(t, source)
		endpoints := reparsed.GetContainer("bus").GetList("endpoint")

		s1Run, _ := endpoints["s1"].GetContainer("external").Get("run")
		assert.Equal(t, "ze exabgp plugin --family ipv4/unicast --family ipv6/unicast --route-refresh --add-path both ./s1.py", s1Run)

		s2Run, _ := endpoints["s2"].GetContainer("external").Get("run")
		assert.Equal(t, "ze exabgp plugin --family ipv4/unicast --family ipv6/unicast --route-refresh --add-path both ./s2.py", s2Run)

		// Without route-refresh or add-path, neither flag is emitted.
		noCapSource := strings.Replace(source, "route-refresh;", "", 1)
		noCapSource = strings.Replace(noCapSource, "add-path send/receive;", "", 1)
		_, _, reparsedNoCap := migrateAndValidate(t, noCapSource)
		s1NoCapRun, _ := reparsedNoCap.GetContainer("bus").GetList("endpoint")["s1"].GetContainer("external").Get("run")
		assert.Equal(t, "ze exabgp plugin --family ipv4/unicast --family ipv6/unicast ./s1.py", s1NoCapRun)

		// Conflicting ADD-PATH modes across neighbors naming one process must error.
		const conflictSource = `
process shared {
	run ./shared.py;
}
neighbor 192.0.2.1 {
	router-id 1.1.1.1;
	local-as 65001;
	peer-as 65002;
	capability {
		add-path receive;
	}
	api {
		processes [ shared ];
	}
}
neighbor 192.0.2.2 {
	router-id 1.1.1.1;
	local-as 65001;
	peer-as 65003;
	capability {
		add-path send;
	}
	api {
		processes [ shared ];
	}
}
`
		conflictTree, err := ParseExaBGPConfig(conflictSource)
		require.NoError(t, err)
		_, err = MigrateFromExaBGP(conflictTree)
		require.Error(t, err, "conflicting ADD-PATH modes across neighbors naming one process must be refused")
		assert.Contains(t, err.Error(), "shared")
		assert.Contains(t, err.Error(), "receive")
		assert.Contains(t, err.Error(), "send")
	})

	t.Run("QE4_all_api_keys_mapped_and_operational_ignored", func(t *testing.T) {
		const source = `
process full {
	run ./full.py;
	encoder json;
}
neighbor 192.0.2.1 {
	description "edge1";
	router-id 1.1.1.1;
	local-as 65001;
	peer-as 65002;
	api {
		processes [ full ];
		neighbor-changes;
		negotiated;
		fsm;
		signal;
		receive {
			open;
			keepalive;
			operational;
		}
		send {
			notification;
			refresh;
			operational;
		}
	}
}
`
		_, _, reparsed := migrateAndValidate(t, source)
		ep := reparsed.GetContainer("bus").GetList("endpoint")["full"]
		require.NotNil(t, ep)
		recv := ep.GetContainer("receive")
		require.NotNil(t, recv)

		assert.Equal(t, []string{
			"bgp:connected",
			"bgp:fsm",
			"bgp:keepalive",
			"bgp:negotiated",
			"bgp:notification",
			"bgp:open",
			"bgp:refresh",
			"bgp:signal",
			"bgp:state",
		}, recv.GetSlice("allow"))

		assert.Equal(t, []string{"received"}, recv.GetList("message")["bgp:open"].GetList("tag")["direction"].GetSlice("value"))
		assert.Equal(t, []string{"received"}, recv.GetList("message")["bgp:keepalive"].GetList("tag")["direction"].GetSlice("value"))
		assert.Equal(t, []string{"sent"}, recv.GetList("message")["bgp:notification"].GetList("tag")["direction"].GetSlice("value"))
		assert.Equal(t, []string{"sent"}, recv.GetList("message")["bgp:refresh"].GetList("tag")["direction"].GetSlice("value"))
	})

	t.Run("unscoped_process_without_neighbor_omits_bus_filters", func(t *testing.T) {
		const source = `
process solo {
	run ./solo.py;
}
`
		_, _, reparsed := migrateAndValidate(t, source)
		ep := reparsed.GetContainer("bus").GetList("endpoint")["solo"]
		require.NotNil(t, ep)
		runLine, _ := ep.GetContainer("external").Get("run")
		assert.Equal(t, "ze exabgp plugin ./solo.py", runLine)
		assert.Nil(t, ep.GetContainer("receive"))
		assert.Nil(t, ep.GetContainer("send"))
	})
}

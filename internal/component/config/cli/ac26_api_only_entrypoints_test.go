//go:build ze_bgp

// Design: docs/architecture/api/architecture.md -- API-only peers request no transit.
// Related: ../../cli/editor_commit.go -- candidate and active publication differ.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgpconfig "github.com/ze-software/ze/internal/component/bgp/config"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	editorcli "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestAC26APIOnlyDefaultAIGPValidatesAndCommits requires two internal peers
// with accepting exports and an explicitly permitted API program to survive
// file validation and the real editor commit paths without requesting a RIB
// output owner. Default AIGP must remain enabled in the published config.
// Adding a partial RIB attachment to that same graph must be refused.
// The separate daemon fixture owns useful API bytes and no learned transit.
func TestAC26APIOnlyDefaultAIGPValidatesAndCommits(t *testing.T) {
	scriptPath := filepath.Join(t.TempDir(), "ac26-api.sh")
	scriptBody := "#!/bin/sh\necho '#1 ze-plugin-engine:declare-registration {\"protocol-version\":1,\"sends\":[{\"message\":\"bgp:update\"}]}'\n"
	require.NoError(t, os.WriteFile(scriptPath, []byte(scriptBody), 0o700))

	configuration := func(routerID string) string {
		return fmt.Sprintf(`bus {
 endpoint bgp-rib { internal bgp-rib; receive { allow none; } send { allow none; } }
 endpoint adj-rib-in { internal bgp-adj-rib-in; send { allow none; } }
 endpoint ac26-api {
  external { run "%s"; }
  encoding json;
 }
}
bgp {
 router-id %s;
 session { asn { local 65000; } }
 peer source {
  connection { local { ip 127.0.0.1; port 11790; } remote { ip 127.0.0.2; connect false; } }
  session { asn { remote 65000; } family { ipv4/unicast { prefix { maximum 100; } } } }
  filter { import { final accept; } export { final accept; } }
 }
 peer destination {
  connection { local { ip 127.0.0.1; port 11790; } remote { ip 127.0.0.3; connect false; } }
  session { asn { remote 65000; } family { ipv4/unicast { prefix { maximum 100; } } } }
  filter { import { final accept; } export { final accept; } }
 }
}
`, scriptPath, routerID)
	}
	requireAPIOnly := func(t *testing.T, content string) {
		t.Helper()
		tree, err := config.ParseTreeForValidation(content)
		require.NoError(t, err)
		bgp := tree.GetContainer("bgp")
		require.NotNil(t, bgp)
		routerID, found := bgp.Get("router-id")
		require.True(t, found)
		require.Equal(t, "192.0.2.253", routerID, "the published graph must contain the requested edit")
		bus := tree.GetContainer("bus")
		require.NotNil(t, bus)
		endpoints := bus.GetList("endpoint")
		require.Len(t, endpoints, 3, "only the disabled RIB, receive-store reporter, and API endpoint are configured")
		api := endpoints["ac26-api"]
		require.NotNil(t, api)
		ext := api.GetContainer("external")
		require.NotNil(t, ext)
		run, found := ext.Get("run")
		require.True(t, found)
		require.Equal(t, scriptPath, run)
		require.NotNil(t, endpoints["adj-rib-in"])
		busCfg, err := plugin.ExtractBusConfigFromMap(tree.ToMap())
		require.NoError(t, err)
		peers, err := bgpconfig.PeersFromConfigTree(tree)
		require.NoError(t, err)
		require.Len(t, peers, 2)
		var epInputs []pluginserver.BusParticipantInput
		if reg := registry.Lookup("bgp-rib"); reg != nil {
			epInputs = append(epInputs, pluginserver.BusParticipantInput{
				Path: plugin.EndpointPath("bgp-rib"), EndpointName: "bgp-rib",
				Sends: reg.Sends, Receives: reg.Receives,
			})
		}
		if reg := registry.Lookup("bgp-adj-rib-in"); reg != nil {
			epInputs = append(epInputs, pluginserver.BusParticipantInput{
				Path: plugin.EndpointPath("adj-rib-in"), EndpointName: "adj-rib-in",
				Sends: reg.Sends, Receives: reg.Receives,
			})
		}
		epInputs = append(epInputs, pluginserver.BusParticipantInput{
			Path: plugin.EndpointPath("ac26-api"), EndpointName: "ac26-api",
			Sends: []rpc.MessageSendDecl{{Message: bgpevents.Namespace + ":" + bgpevents.SendUpdate}},
		})
		graph, err := pluginserver.BuildDeliveryGraph(bgpevents.Namespace, reactor.DisseminationPeersFromSettings(peers), busCfg, epInputs)
		require.NoError(t, err)
		names := make(map[string]bool)
		for _, peer := range peers {
			names[peer.Name] = true
			addr := peer.PeerKey().String()
			require.True(t, peer.IsIBGP())
			require.True(t, peer.AIGPEnabled(), "omitting AIGP must preserve the internal-session default")
			require.False(t, peer.AIGPConfigured, "omitting AIGP must not activate the explicit AIGP requirement")
			require.True(t, peer.ExportPolicy.Configured())
			require.True(t, peer.ExportPolicy.AcceptsAtEnd(false))
			require.False(t, peer.DisseminationOwner.IsSet(), "API send permission must not request ordinary transit")
			require.True(t, graph.MaySend(addr, "ac26-api", bgpevents.SendUpdate), "ac26-api must be permitted to send updates to %s", peer.Name)
			require.False(t, graph.MaySend(addr, "adj-rib-in", bgpevents.SendUpdate), "adj-rib-in must not be permitted to send updates to %s", peer.Name)
			require.False(t, graph.MaySend(addr, "bgp-rib", bgpevents.SendUpdate), "disabled bgp-rib must not be permitted to send updates to %s", peer.Name)
		}
		require.Equal(t, map[string]bool{"source": true, "destination": true}, names)
	}
	base, candidate := configuration("192.0.2.254"), configuration("192.0.2.253")
	file := filepath.Join(t.TempDir(), "ac26.conf")
	require.NoError(t, os.WriteFile(file, []byte(candidate), 0o600))
	require.NoError(t, ValidateContent(base, file))
	require.NoError(t, ValidateContent(candidate, file))
	require.Equal(t, exitOK, cmdValidate([]string{"-q", file}))
	requireAPIOnly(t, candidate)

	for _, mode := range []string{"candidate", "commit"} {
		t.Run(mode, func(t *testing.T) {
			store := storage.CreateMemory()
			t.Cleanup(func() { require.NoError(t, store.Close()) })
			const name = "ac26.conf"
			require.NoError(t, store.WriteFile(name, []byte(base), 0o600))
			ed, err := editorcli.NewEditorWithStorage(store, name)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, ed.Close()) })
			ed.SetSession(editorcli.NewEditSession("ac26", "web"))
			loaded, err := ed.ParseLoad(candidate, nil)
			require.NoError(t, err)
			require.NoError(t, ed.LoadReplace(nil, loaded))
			validated := false
			ed.SetPreCommitValidate(func(staged string) error {
				validated = true
				return ValidateContent(staged, name)
			})
			var result *editorcli.CommitResult
			var output string
			if mode == "candidate" {
				result, output, err = ed.CommitSessionCandidate(time.Now())
			} else {
				result, err = ed.CommitSession()
			}
			require.NoError(t, err)
			require.True(t, validated, "publication must reach the real candidate validator")
			require.NotNil(t, result)
			require.Empty(t, result.Conflicts)
			require.Positive(t, result.Applied, "the accepted graph must publish the changed router-id")
			active, err := storage.ReadActiveConfig(store, name)
			require.NoError(t, err)
			mirror, err := store.ReadFile(name)
			require.NoError(t, err)
			require.Equal(t, active, mirror)
			published, _, present, err := storage.ReadCandidateConfig(store, name)
			require.NoError(t, err)
			if mode == "candidate" {
				require.Equal(t, []byte(base), active)
				require.True(t, present)
				require.Equal(t, output, string(published))
				require.True(t, ed.Dirty(), "candidate publication awaits the daemon's acceptance")
				requireAPIOnly(t, string(published))
				return
			}
			require.NotEqual(t, []byte(base), active)
			require.False(t, present)
			require.False(t, ed.Dirty())
			require.Equal(t, string(active), ed.OriginalContent())
			requireAPIOnly(t, string(active))
		})
	}

	t.Run("partial RIB is refused", func(t *testing.T) {
		partial := strings.Replace(candidate,
			" endpoint adj-rib-in { internal bgp-adj-rib-in; send { allow none; } }",
			" endpoint adj-rib-in { internal bgp-adj-rib-in; send { allow none; } }\n endpoint ac26-table { internal bgp-rib; receive { allow [ bgp:state ]; } send { allow none; } }", 1)
		require.NotEqual(t, candidate, partial)
		const refusal = `participant "source" endpoint "ac26-table" has incomplete source grants`
		const fixLine = `bus { endpoint ac26-table { receive { allow [ bgp:update bgp:state ]; } } }`
		file := filepath.Join(t.TempDir(), "ac26-partial.conf")
		require.NoError(t, os.WriteFile(file, []byte(partial), 0o600))
		validateErr := ValidateContent(partial, file)
		require.ErrorContains(t, validateErr, refusal)
		require.ErrorContains(t, validateErr, fixLine)
		require.Equal(t, exitInvalid, cmdValidate([]string{"-q", file}))
		for _, mode := range []string{"candidate", "commit"} {
			t.Run(mode, func(t *testing.T) {
				store := storage.CreateMemory()
				t.Cleanup(func() { require.NoError(t, store.Close()) })
				const name = "ac26.conf"
				require.NoError(t, store.WriteFile(name, []byte(base), 0o600))
				ed, err := editorcli.NewEditorWithStorage(store, name)
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, ed.Close()) })
				ed.SetSession(editorcli.NewEditSession("ac26-partial", "web"))
				loaded, err := ed.ParseLoad(partial, nil)
				require.NoError(t, err)
				require.NoError(t, ed.LoadReplace(nil, loaded))
				validated := false
				ed.SetPreCommitValidate(func(staged string) error {
					validated = true
					return ValidateContent(staged, name)
				})
				var result *editorcli.CommitResult
				var output string
				if mode == "candidate" {
					result, output, err = ed.CommitSessionCandidate(time.Now())
				} else {
					result, err = ed.CommitSession()
				}
				require.True(t, validated, "the partial graph must reach the real publication validator")
				require.ErrorContains(t, err, refusal)
				require.ErrorContains(t, err, fixLine)
				require.Nil(t, result)
				require.Empty(t, output)
				active, err := storage.ReadActiveConfig(store, name)
				require.NoError(t, err)
				require.Equal(t, []byte(base), active)
				mirror, err := store.ReadFile(name)
				require.NoError(t, err)
				require.Equal(t, []byte(base), mirror)
				_, _, present, err := storage.ReadCandidateConfig(store, name)
				require.NoError(t, err)
				require.False(t, present, "the refused partial graph must not publish a candidate")
				require.True(t, ed.Dirty(), "refusal must retain the operator's pending correction")
			})
		}
	})
}

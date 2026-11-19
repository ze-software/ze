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
	editorcli "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestAC26APIOnlyDefaultAIGPValidatesAndCommits requires two internal peers
// with accepting exports and an explicitly permitted API program to survive
// file validation and the real editor commit paths without requesting a RIB
// output owner. Default AIGP must remain enabled in the published config.
// Adding a partial RIB attachment to that same graph must be refused.
// The separate daemon fixture owns useful API bytes and no learned transit.
func TestAC26APIOnlyDefaultAIGPValidatesAndCommits(t *testing.T) {
	configuration := func(routerID string) string {
		return fmt.Sprintf(`plugin {
 internal adj-rib-in { use bgp-adj-rib-in; }
 external ac26-api {
  run "le test fixture reload/live-stages ac26-stages.script";
  encoder json;
 }
}
bgp {
 router-id %s;
 session { asn { local 65000; } }
 peer source {
  connection { local { ip 127.0.0.1; port 11790; } remote { ip 127.0.0.2; connect false; } }
  session { asn { remote 65000; } family { ipv4/unicast { prefix { maximum 100; } } } }
  filter { import { final accept; } export { final accept; } }
  attach process adj-rib-in { receive [ update-received state ]; }
  attach process ac26-api { send [ update ]; }
 }
 peer destination {
  connection { local { ip 127.0.0.1; port 11790; } remote { ip 127.0.0.3; connect false; } }
  session { asn { remote 65000; } family { ipv4/unicast { prefix { maximum 100; } } } }
  filter { import { final accept; } export { final accept; } }
  attach process adj-rib-in { receive [ update-received state ]; }
  attach process ac26-api { send [ update ]; }
 }
}
`, routerID)
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
		plugins := tree.GetContainer("plugin")
		require.NotNil(t, plugins)
		api := plugins.GetList("external")["ac26-api"]
		require.NotNil(t, api)
		run, found := api.Get("run")
		require.True(t, found)
		require.Equal(t, "le test fixture reload/live-stages ac26-stages.script", run)
		internal := plugins.GetList("internal")
		require.Len(t, internal, 1, "only the receive-store reporter is configured; no RIB process is requested")
		require.NotNil(t, internal["adj-rib-in"])
		peers, err := bgpconfig.PeersFromConfigTree(tree)
		require.NoError(t, err)
		require.Len(t, peers, 2)
		names := make(map[string]bool)
		for _, peer := range peers {
			names[peer.Name] = true
			require.True(t, peer.IsIBGP())
			require.True(t, peer.AIGPEnabled(), "omitting AIGP must preserve the internal-session default")
			require.True(t, peer.ExportPolicy.Configured())
			require.True(t, peer.ExportPolicy.AcceptsAtEnd(false))
			require.False(t, peer.DisseminationOwner.IsSet(), "API send permission must not request ordinary transit")
			apiGranted := false
			for _, binding := range peer.ProcessBindings {
				if binding.PluginName == "ac26-api" {
					require.True(t, binding.Explicit)
					require.True(t, binding.MaySend("update"))
					apiGranted = true
					continue
				}
				require.False(t, binding.MaySend("update"), "a receive-only reporter or default processor must gain no send grant")
			}
			require.True(t, apiGranted, "both internal peers must retain the configured API sender")
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
			" internal adj-rib-in { use bgp-adj-rib-in; }",
			" internal adj-rib-in { use bgp-adj-rib-in; }\n internal ac26-table { use bgp-rib; }", 1)
		partial = strings.Replace(partial,
			"  attach process ac26-api { send [ update ]; }",
			"  attach process ac26-api { send [ update ]; }\n  attach process ac26-table { receive [ state ]; }", 1)
		require.NotEqual(t, candidate, partial)
		const refusal = "peer \"source\" process \"ac26-table\" has incomplete source grants; complete with `attach process ac26-table { receive [ update-received state ]; }`"
		file := filepath.Join(t.TempDir(), "ac26-partial.conf")
		require.NoError(t, os.WriteFile(file, []byte(partial), 0o600))
		require.ErrorContains(t, ValidateContent(partial, file), refusal)
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

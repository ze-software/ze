// VALIDATES: no legacy ingress callback or migrated ingress member drops a peer's UPDATE
// because of what the redistribute block says.
//
// The method installs the destination-scoped evaluator the config loader
// builds in a child process, so plugin startup cannot change other tests'
// globals. It runs every remaining filterapi.IngressOrdered callback and the
// four migrated callbacks through their registered engines and filter-update.
// Each receives the original peer facts, UPDATE and fresh metadata, and must
// accept. The migrated population is a historical regression fixture, not a
// catalog of every configurable filter instance.
//
// The test lives at the composition root. This package blank-imports the whole
// plugin set, so the pipeline here is the one a daemon runs. A test inside one
// filter's own package cannot see the pipeline. That is how
// bgp-redistribute came to reject every received route with green unit tests
// behind it: those tests built ImportRule values with an empty Destination,
// which the config loader never writes.
//
// PREVENTS: a redistribution rule gating BGP's own Adj-RIB-In. A rule states
// which routes move BETWEEN protocols. The per-consumer decision belongs to
// the orchestrator, which alone holds the importing protocol.

//go:build ze_bgp

package all

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bgp/config" // Install the real Stage 1 declaration judge.
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginmgr "github.com/ze-software/ze/internal/component/plugin/manager"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// receivedUpdateBody is the body of the UPDATE a peer sends for 10.0.0.0/24,
// captured from a daemon run of test/draft/plugin/redistribute-bgp-to-ospf-no-plumbing.ci.
// Withdrawn length 0, then 21 octets of attributes (ORIGIN igp, empty AS_PATH,
// NEXT_HOP 10.0.0.1, LOCAL_PREF 100), then the NLRI 10.0.0.0/24.
var receivedUpdateBody = []byte{
	0x00, 0x00,
	0x00, 0x15,
	0x40, 0x01, 0x01, 0x00,
	0x40, 0x02, 0x00,
	0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01,
	0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64,
	0x18, 0x0a, 0x00, 0x00,
}

func TestNoIngressFilterGatesAPeerRouteOnRedistributionConfig(t *testing.T) {
	// Startup changes plugin-owned globals that registry.Snapshot does not
	// hold. Only this test runs in the child, using the same compiled-in
	// plugins. The positional selector prevents recursive child creation.
	const child = "ingress-redistribution-child"
	if !slices.Equal(flag.Args(), []string{child}) {
		binary, err := os.Executable()
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, "-test.run=^"+t.Name()+"$",
			"-test.v=true", "-test.count=1", "-test.timeout=90s", "--", child)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "isolated ingress regression failed:\n%s", output)
		require.Contains(t, string(output), "--- PASS: "+t.Name()+" ",
			"child did not run the ingress regression:\n%s", output)
		return
	}

	// The rule the config loader builds for
	// `redistribute { destination ospf { import bgp; } }`, which moves every
	// route of the source. Destination is populated for every rule a config produces.
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{{
		Source:      "bgp",
		Destination: "ospf",
	}}))
	defer configredist.SetGlobal(nil)

	filters := filterapi.IngressOrdered()

	src := filterapi.PeerFilterInfo{LocalAS: 65000, PeerAS: 65001}
	executed := 0
	for _, f := range filters {
		accept, _ := f.Ingress(src, receivedUpdateBody, make(map[string]any))
		require.Truef(t, accept,
			"ingress filter %q dropped a peer's route while a redistribute rule was configured", f.Name)
		executed++
	}

	// These are the four original ingress callbacks, after their migration
	// to peer-list members. Keep every row even when no default peer list
	// selects it: community policy is operator-selected, and the role and
	// LLGR procedures are inserted only when their config applies.
	migrated := []struct{ plugin, filter string }{
		{"bgp-filter-community", "ingress"},
		{"bgp-filter-community", "relation"},
		{"bgp-role", "otc-ingress"},
		{"bgp-gr", "llgr-ingress"},
	}
	configs := []plugin.PluginConfig{{Name: "bgp-rib", Internal: true, Encoder: "json"}}
	for _, member := range migrated {
		reg := registry.Lookup(member.plugin)
		require.NotNil(t, reg, "missing migrated plugin %s", member.plugin)
		require.NotNil(t, reg.RunEngine, "missing registered engine %s", member.plugin)
		if !slices.ContainsFunc(configs, func(cfg plugin.PluginConfig) bool { return cfg.Name == member.plugin }) {
			configs = append(configs, plugin.PluginConfig{Name: member.plugin, Internal: true, Encoder: "json"})
		}
	}
	// bgp-gr declares bgp-rib as a runtime dependency. Start its real
	// engine as well, using the server/manager lifecycle used by the
	// recovery-delivery integration harness. No TCP peers are needed here.
	srv := ingressRedistributionServer(t, configs)
	for _, member := range migrated {
		t.Run(member.plugin+":"+member.filter, func(t *testing.T) {
			pm := srv.ProcessManager()
			require.NotNil(t, pm, "server has no process manager")
			proc := pm.GetProcess(member.plugin)
			require.NotNil(t, proc)
			registration := proc.Registration()
			require.NotNil(t, registration)
			index := slices.IndexFunc(registration.Filters, func(f plugin.FilterRegistration) bool {
				return f.Name == member.filter
			})
			require.NotEqual(t, -1, index, "missing Stage 1 declaration")
			decl := &registration.Filters[index]
			require.Equal(t, rpc.FilterImport, decl.Direction)
			require.Contains(t, decl.Reads, rpc.FilterFieldRaw)

			var view policyview.View
			view.Reset(wireu.NewWireUpdate(receivedUpdateBody, 0),
				policyview.ImportSession(src).WithImportMetadata(make(map[string]any)))
			defer view.Clear()
			fields, err := policyview.AppendFields(nil, &view, decl.Reads)
			require.NoError(t, err)
			carrier := ingressRedistributionView{view: &view, fields: fields, raw: receivedUpdateBody}
			input := rpc.FilterUpdateInput{Filter: member.filter, Direction: "import", PeerAS: src.PeerAS}
			route := rpc.ViewFilterRoute(&input, &carrier)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			out, err := srv.CallFilterUpdate(ctx, member.plugin, &route)
			require.NoError(t, err, "registered ingress handler failed")
			require.NotNil(t, out, "registered ingress handler returned no verdict")
			require.Empty(t, out.Error, "registered ingress handler failed")
			require.False(t, out.Teardown, "registered ingress handler requested session teardown")
			// With the original roleless, LLGR-disabled peer and no community
			// policy, all four producers return accept, not a withdrawal or
			// modification that could remove the route's announcements.
			require.Equal(t, rpc.FilterAccept, out.Action,
				"ingress member dropped a peer's route while a redistribute rule was configured")
			executed++
		})
	}
	require.Equal(t, len(filters)+len(migrated), executed, "every ingress regression case must execute")
	require.NotZero(t, executed, "the composition root must execute real ingress filters")
}

// ingressRedistributionView carries the original peer's production policy
// view and declared JSON fields. It supplies data, never a filter verdict.
type ingressRedistributionView struct {
	view   *policyview.View
	fields []byte
	raw    []byte
}

func (v *ingressRedistributionView) Fields() ([]byte, error) { return v.fields, nil }
func (v *ingressRedistributionView) Raw() []byte             { return v.raw }
func (v *ingressRedistributionView) Engine() any             { return v.view }

// ingressRedistributionServer starts the real registered runners and joins
// them on cleanup. The reactor supplies the RIB's engine operations without
// starting a listening socket or changing the regression peer's facts.
// The caller MUST run this fixture only in the isolated test child above.
func ingressRedistributionServer(t *testing.T, configs []plugin.PluginConfig) *pluginserver.Server {
	t.Helper()
	r := reactor.New(&reactor.Config{})
	adapter, ok := r.ReactorLifecycleAdapter().(plugin.ReactorLifecycle)
	require.True(t, ok)
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{Plugins: configs}, adapter)
	require.NoError(t, err)
	registry.SetEventBus(srv)
	mgr := pluginmgr.NewManager()
	require.NoError(t, mgr.StartAll(t.Context(), srv, nil))
	srv.SetProcessSpawner(mgr)
	t.Cleanup(func() {
		srv.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Wait(ctx); err != nil {
			t.Error(err)
		}
		if pm, ok := mgr.GetProcessManager().(*process.ProcessManager); ok {
			for _, proc := range pm.AllProcesses() {
				if err := proc.WaitEngine(ctx); err != nil {
					t.Errorf("plugin engine %s did not stop: %v", proc.Name(), err)
				}
			}
		}
		require.NoError(t, mgr.StopAll(ctx))
	})
	require.NoError(t, srv.StartWithContext(t.Context()))
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	require.NoError(t, srv.WaitForStartupComplete(ctx))
	return srv
}

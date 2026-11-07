//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- config-to-kernel tests.
// Related: pppoe_packets_integration_linux_test.go -- packet scenario.
// Related: pppoe_kernel_integration_linux_test.go -- installed-state scenario.

package iface_test

import (
	"context"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginmgr "github.com/ze-software/ze/internal/component/plugin/manager"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	_ "github.com/ze-software/ze/internal/component/sysctl"
)

const pppoeOperatorConfig = `
interface {
    backend netlink;
    pppoe-client wan {
        source-interface zec0;
        authentication {
            username subscriber;
            password secret;
        }
        mtu 1450;
        route-priority 231;
    }
}
`

// startPPPoEConfig delivers operator config through the registered server runner.
// The caller MUST enter userns first and keep its peer alive until cleanup.
// Each caller runs in its own child process, isolating plugin-owned globals.
// The returned server takes a reload through ReloadConfig.
func startPPPoEConfig(t *testing.T, input string) *pluginserver.Server {
	t.Helper()
	if _, _, observed := iface.PPPoEStartupCountsForTest(); observed {
		t.Fatal("PPPoE startup observer already holds a delivery; use a fresh userns child")
	}
	loaded, err := config.LoadConfig(input, "", nil)
	if err != nil {
		t.Fatalf("load operator config: %v", err)
	}
	coordinator := plugin.NewCoordinator(loaded.Tree.ToPluginMap())
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{
		Plugins:         loaded.Plugins,
		ConfiguredPaths: config.CollectContainerPaths(loaded.Tree),
		DataPlane:       iface.BackendNameFromTree(loaded.Tree.ToMap()),
		ConfigNeedHolds: config.ConfigNeedHolds(loaded.Tree.ToMap()),
	}, coordinator)
	if err != nil {
		t.Fatalf("create plugin server: %v", err)
	}
	registry.SetPluginServer(srv)
	registry.SetEventBus(srv)
	mgr := pluginmgr.NewManager()
	if err := mgr.StartAll(t.Context(), srv, nil); err != nil {
		t.Fatalf("start plugin manager: %v", err)
	}
	srv.SetProcessSpawner(mgr)
	t.Cleanup(func() {
		srv.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Wait(ctx); err != nil {
			t.Errorf("stop plugin server: %v", err)
		}
		if raw := mgr.GetProcessManager(); raw != nil {
			pm, ok := raw.(*process.ProcessManager)
			if ok {
				for _, proc := range pm.AllProcesses() {
					if err := proc.WaitEngine(ctx); err != nil {
						t.Errorf("join plugin %s: %v", proc.Name(), err)
					}
				}
			} else {
				t.Errorf("plugin manager returned process manager type %T", raw)
			}
		}
		if err := mgr.StopAll(ctx); err != nil {
			t.Errorf("stop plugin manager: %v", err)
		}
	})
	if err := srv.StartWithContext(t.Context()); err != nil {
		t.Fatalf("start plugin server: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	if err := srv.WaitForStartupComplete(ctx); err != nil {
		t.Fatalf("registered plugin startup: %v", err)
	}
	pm := srv.ProcessManager()
	if pm == nil {
		t.Fatal("config did not start a plugin process manager")
	}
	proc := pm.GetProcess("interface")
	if proc == nil {
		t.Fatal("operator interface config did not auto-load the registered interface plugin")
	}
	if proc.Registration() == nil {
		t.Fatal("interface plugin did not complete registration")
	}
	parsed, active, observed := iface.PPPoEStartupCountsForTest()
	if !observed {
		t.Fatal("registered interface config delivery did not reach PPPoE reconciliation")
	}
	if parsed != 1 {
		t.Fatalf("parsed %d PPPoE clients, want one", parsed)
	}
	if active != 1 {
		t.Fatalf("reconciler started %d clients, want one", active)
	}
	return srv
}

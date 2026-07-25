// VALIDATES: the traffic-usage plugin is registered with the engine entrypoint,
// the "interface" dependency (so the iface rate tracker runs), its config root,
// metrics binding, and embedded YANG.
// PREVENTS: an unreachable plugin; a missing interface dependency leaving the
// rate tracker (and thus the snapshot-driven lifecycle) dormant.

package trafficusage

import (
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestTrafficUsageRegistration(t *testing.T) {
	reg := registry.Lookup("traffic-usage")
	if reg == nil {
		t.Fatal("traffic-usage plugin not registered")
	}
	if reg.Name != "traffic-usage" {
		t.Errorf("Name = %q, want traffic-usage", reg.Name)
	}
	if reg.RunEngine == nil {
		t.Error("RunEngine is nil")
	}
	if reg.CLIHandler == nil {
		t.Error("CLIHandler is nil (registration would fail)")
	}
	if len(reg.ConfigRoots) != 1 || reg.ConfigRoots[0] != "traffic/usage" {
		t.Errorf("ConfigRoots = %v, want [traffic/usage]", reg.ConfigRoots)
	}
	if len(reg.Dependencies) != 1 || reg.Dependencies[0] != "interface" {
		t.Errorf("Dependencies = %v, want [interface]", reg.Dependencies)
	}
	if reg.ConfigureMetrics == nil {
		t.Error("ConfigureMetrics is nil")
	}
	if reg.YANG == "" {
		t.Error("YANG schema is empty")
	}
}

// VALIDATES: runEngine refuses to start (nonzero exit) when conn is not a
// same-process (DirectBridge-carrying) connection -- iface.SubscribeCollectNotify
// (register.go) registers a callback into iface's package-level subscriber
// list as a plain Go function call, which only reaches the engine's real
// rate tracker when this plugin shares process memory with it. It is the
// monitor's only attach/detach mechanism (register.go's OnConfigure/
// OnConfigApply build the monitor around it), so an external traffic-usage
// would silently never attach to any interface, with ze_traffic_usage_*
// permanently empty and no error anywhere. A plain net.Pipe() end matches
// exactly what an external plugin's non-bridged conn looks like from the
// SDK's perspective (see sdk.Plugin.IsInternal).
// PREVENTS: traffic-usage configured `plugin { external traffic-usage { ... } }`
// silently accepting every config commit while never collecting any traffic.
//
// The elapsed-time assertion matters: without the guard, runEngine falls
// through to p.Run(ctx, ...), which also eventually returns a nonzero exit
// against a non-responsive plain net.Pipe() end -- but only after the SDK's
// handshake/registration protocol times out (tens of seconds), not because
// external mode was detected. A refuse-immediately guard must return well
// under that timeout.
func TestRunEngine_RefusesExternalProcess(t *testing.T) {
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() {
		pluginEnd.Close() //nolint:errcheck // test cleanup
		engineEnd.Close() //nolint:errcheck // test cleanup
	})

	start := time.Now()
	code := runEngine(pluginEnd)
	elapsed := time.Since(start)

	if code != 1 {
		t.Fatalf("runEngine(external conn) = %d, want 1", code)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("runEngine(external conn) took %s, want an immediate refusal (< 2s) -- suggests it fell through to p.Run()'s handshake timeout instead of refusing at the IsInternal() guard", elapsed)
	}
}

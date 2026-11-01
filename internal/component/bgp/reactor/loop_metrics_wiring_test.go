// Design: docs/architecture/core-design.md -- loop-filter Prometheus metrics

package reactor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestLoopMetricsWiredByTheReactor verifies loop detection's counter is built
// from the registry the reactor is given, through the production wiring
// (SetMetricsRegistry -> Start -> the metrics-enable block). Loop detection is
// the engine's own protocol step, not a registered filter, so no filter hook
// reaches it: the reactor hands the registry to it directly.
//
// VALIDATES: the metrics-enable block calls bgpfilter.SetMetricsRegistry.
// PREVENTS: ze_bgp_as_path_loop_detected_total silently dead.
func TestLoopMetricsWiredByTheReactor(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(emptyConfig), 0o600))

	reg := newSpyRegistry()
	r := New(&Config{ConfigPath: configPath, ListenAddr: "127.0.0.1:0", Standalone: true})
	r.SetReloadFunc(simpleReloadFunc)
	r.SetMetricsRegistry(reg)
	require.NoError(t, r.Start())
	defer r.Stop()

	require.NotNil(t, reg.counterVec("ze_bgp_as_path_loop_detected_total"),
		"the reactor did not hand its metrics registry to loop detection")
}

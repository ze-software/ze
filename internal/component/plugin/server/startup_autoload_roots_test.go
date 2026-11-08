package server

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestRemovedRootStopsOwnerOnlyWhenNoRootRemains verifies a reload stops a
// plugin that owns several config roots only once none of them stays
// configured.
//
// VALIDATES: collectProcessesForRemovedConfigPaths keeps a running owner whose
// other root is still in the new tree, and stops it once every root is gone.
// PREVENTS: deleting `system conntrack` stopping the firewall plugin while its
// `firewall {}` block stays, whose stop flush then removes every firewall
// table the operator still configures.
//
// Method: a registered plugin owns "alpha" and "beta/gamma" and has a running
// process. Each case hands the collector the removed paths and the new tree
// and checks whether the plugin is in the stop set.
func TestRemovedRootStopsOwnerOnlyWhenNoRootRemains(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const pluginName = "two-root-owner"
	require.NoError(t, registry.Register(registry.Registration{
		Name:        pluginName,
		Description: "plugin owning two config roots",
		ConfigRoots: []string{"alpha", "beta/gamma"},
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
	}))

	s := &Server{config: &ServerConfig{}}
	pm := process.NewProcessManager(nil)
	pm.AddProcess(pluginName, process.NewProcess(plugin.PluginConfig{Name: pluginName}))
	s.procManager.Store(pm)

	tests := []struct {
		name    string
		removed []string
		tree    map[string]any
		stopped bool
	}{
		{
			name:    "one root removed, the other stays",
			removed: []string{"beta/gamma"},
			tree:    map[string]any{"alpha": map[string]any{"x": "1"}, "beta": map[string]any{"other": "1"}},
			stopped: false,
		},
		{
			name:    "parent of one root removed, the other stays",
			removed: []string{"beta"},
			tree:    map[string]any{"alpha": map[string]any{"x": "1"}},
			stopped: false,
		},
		{
			name:    "both roots removed",
			removed: []string{"alpha", "beta/gamma"},
			tree:    map[string]any{"beta": map[string]any{"other": "1"}},
			stopped: true,
		},
		{
			name:    "the only configured root removed",
			removed: []string{"beta/gamma"},
			tree:    map[string]any{"beta": map[string]any{"other": "1"}},
			stopped: true,
		},
		{
			name:    "a path under a root removed",
			removed: []string{"alpha/x"},
			tree:    map[string]any{"alpha": map[string]any{}},
			stopped: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.collectProcessesForRemovedConfigPaths(tt.removed, tt.tree)
			assert.Equal(t, tt.stopped, got[pluginName])
		})
	}
}

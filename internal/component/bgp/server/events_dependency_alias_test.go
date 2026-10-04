// Design: docs/guide/graceful-restart.md -- GR retention precedes RIB peer-down.
package server

import (
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/gr"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rib"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
)

// TestStateDependencyOrderUsesImplementationIdentity prevents configured aliases
// from losing GR's dependency on the RIB. Both state and EOR delivery use this
// sorter; the whole llgr-import-no-llgr.ci proves its peer-down consequence.
func TestStateDependencyOrderUsesImplementationIdentity(t *testing.T) {
	for _, tc := range []struct {
		name    string
		configs []plugin.PluginConfig
		want    []string
	}{
		{
			name: "both implementations aliased",
			configs: []plugin.PluginConfig{
				{Name: "aaa-rib", Internal: true, Run: "ze.bgp-rib"},
				{Name: "zzz-gr", Internal: true, Run: "ze.bgp-gr"},
			},
			want: []string{"zzz-gr", "aaa-rib"},
		},
		{
			name: "duplicate aliases and external name collision",
			configs: []plugin.PluginConfig{
				{Name: "aaa-rib", Internal: true, Run: "ze.bgp-rib"},
				{Name: "bgp-gr", Run: "/external/gr"},
				{Name: "zzz-gr-2", Internal: true, Run: "ze plugin bgp-gr"},
				{Name: "zzz-gr-1", Internal: true, Run: "ze.bgp-gr"},
			},
			want: []string{"zzz-gr-1", "zzz-gr-2", "aaa-rib", "bgp-gr"},
		},
		{
			name: "external program borrows no registered dependencies",
			configs: []plugin.PluginConfig{
				{Name: "bgp-gr", Run: "/external/gr"},
				{Name: "aaa-rib", Internal: true, Run: "ze.bgp-rib"},
			},
			want: []string{"aaa-rib", "bgp-gr"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			procs := make([]*process.Process, 0, len(tc.configs))
			for _, cfg := range tc.configs {
				procs = append(procs, process.NewProcess(cfg))
			}
			sortByReverseDependencyTier(procs)
			got := make([]string, 0, len(procs))
			for _, p := range procs {
				got = append(got, p.Name())
			}
			require.Equal(t, tc.want, got)
		})
	}
}

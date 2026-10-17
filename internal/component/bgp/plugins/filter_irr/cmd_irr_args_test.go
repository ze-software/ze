package filter_irr

import (
	"strings"
	"testing"

	irryang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_irr/yang"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// VALIDATES: `update bgp irr asn <asn>` and `update bgp irr as-set <as-set>`
// reach the plugin with their value. matchCommandTokens binds a value whose
// YANG leaf is spelled like the command's last keyword as a typed selector and
// hands the forwarder empty args (internal/component/plugin/server/command.go),
// so the forwarder reads the value back by the leaf name.
// METHOD: the leaf names the forwarders read must be the ones the YANG
// declares, and the selector a bound value lands in must come back as the one
// argument the plugin handler reads.
// PREVENTS: the plugin answering every such command with its usage line.
func TestUpdateIRRForwardersRecoverTheBoundValue(t *testing.T) {
	for _, leaf := range []string{leafASN, leafASSet} {
		if !strings.Contains(irryang.ZeFilterIrrCmdYANG, "leaf "+leaf+" {") {
			t.Errorf("ze-filter-irr-cmd.yang declares no leaf %q", leaf)
		}
	}
	tests := []struct {
		name      string
		selectors map[string]string
		leaf      string
		want      string
	}{
		{"asn", map[string]string{"asn": "65001"}, leafASN, "65001"},
		{"as-set", map[string]string{"as-set": "AS-TEST"}, leafASSet, "AS-TEST"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &pluginserver.CommandContext{Selectors: tt.selectors}
			got := ctx.ArgsOrSelector(nil, tt.leaf)
			if len(got) != 1 || got[0] != tt.want {
				t.Fatalf("args = %v, want [%s]", got, tt.want)
			}
		})
	}
}

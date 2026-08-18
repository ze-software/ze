// Design: docs/architecture/firewall/firewall-irr.md -- the server-side forwarders
// Related: cmd_irr.go -- argsOrSelector under test

package irr

import (
	"testing"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// VALIDATES: `update firewall irr asn <asn>` and `update firewall irr as-set
// <as-set>` reach the plugin with their value. Their YANG leaf is spelled like
// the command's last keyword, so matchCommandTokens binds the value as a typed
// selector and hands the forwarder empty args
// (internal/component/plugin/server/command.go). Reading only args left the
// plugin with none, and it answered with its usage line: no operator could ever
// fetch a prefix list, and every IRR functional test failed on the first
// command it issued.
// PREVENTS: the forwarders discarding a value the dispatcher already validated.
func TestArgsOrSelectorRecoversTheBoundValue(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		selectors map[string]string
		leaf      string
		want      string
	}{
		{"asn from selector", nil, map[string]string{"asn": "65001"}, leafASN, "65001"},
		{"as-set from selector", nil, map[string]string{"as-set": "AS-TEST"}, leafASSet, "AS-TEST"},
		{"args win when present", []string{"64496"}, map[string]string{"asn": "65001"}, leafASN, "64496"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &pluginserver.CommandContext{Selectors: tt.selectors}
			got := argsOrSelector(ctx, tt.args, tt.leaf)
			if len(got) != 1 || got[0] != tt.want {
				t.Fatalf("argsOrSelector = %v, want [%s]", got, tt.want)
			}
		})
	}
}

// VALIDATES: a command typed with no value stays empty, so the plugin still
// answers with its usage line instead of acting on a stale selector.
func TestArgsOrSelectorKeepsAMissingValueMissing(t *testing.T) {
	ctx := &pluginserver.CommandContext{}
	if got := argsOrSelector(ctx, nil, leafASN); len(got) != 0 {
		t.Fatalf("argsOrSelector = %v, want no arguments", got)
	}
}

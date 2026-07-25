package clear

import (
	"testing"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	_ "github.com/ze-software/ze/internal/component/iface/cmd"
	_ "github.com/ze-software/ze/internal/component/ike/cmd"
	_ "github.com/ze-software/ze/internal/component/resolve/cmd"
)

func TestAllClearCommandsHaveRegisteredRPC(t *testing.T) {
	want := map[string]string{
		"ze-clear:dns-cache":          "internal/component/resolve/cmd",
		"ze-clear:interface-counters": "internal/component/iface/cmd",
		"ze-clear:vpn-ipsec-sa":       "internal/component/ike/cmd",
	}

	rpcs := pluginserver.AllBuiltinRPCs()
	registered := make(map[string]bool, len(rpcs))
	for _, r := range rpcs {
		registered[r.WireMethod] = true
	}

	for method, owner := range want {
		if !registered[method] {
			t.Errorf("WireMethod %q (owner: %s) has no registered RPC handler", method, owner)
		}
	}

	for method := range want {
		count := 0
		for _, r := range rpcs {
			if r.WireMethod == method {
				count++
			}
		}
		if count > 1 {
			t.Errorf("WireMethod %q registered %d times, want exactly 1", method, count)
		}
	}
}

package schema

import (
	"strings"
	"testing"
)

func TestMonitorSchemaHasNoMigratedOwnerCommands(t *testing.T) {
	banned := map[string]string{
		`"ze-monitor:vpn-ipsec"`:      "IPsec monitor -> internal/component/ike/schema",
		`"ze-monitor:system-netlink"`: "netlink monitor -> internal/component/iface/schema",
		`"ze-event:monitor"`:          "event monitor -> internal/component/command/schema",
	}
	for token, owner := range banned {
		if strings.Contains(ZeCliMonitorCmdYANG, token) {
			t.Errorf("central monitor schema declares owner command %q; move it to %s (see ai/rules/plugin-self-containment.md)", token, owner)
		}
	}
}

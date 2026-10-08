package yang

import (
	"strings"
	"testing"
)

func TestMonitorSchemaHasNoMigratedOwnerCommands(t *testing.T) {
	banned := map[string]string{
		`"ze-ike:monitor-vpn-ipsec"`:      "IPsec monitor -> internal/component/ike/yang",
		`"ze-iface:monitor-system-netlink"`: "netlink monitor -> internal/component/iface/yang",
		`"ze-meta:event-monitor"`:          "event monitor -> internal/plugins/meta/yang",
	}
	for token, owner := range banned {
		if strings.Contains(ZeCliMonitorCmdYANG, token) {
			t.Errorf("central monitor schema declares owner command %q; move it to %s (see ai/rules/plugins.md)", token, owner)
		}
	}
}

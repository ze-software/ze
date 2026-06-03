package schema

import (
	"strings"
	"testing"
)

func TestClearSchemaHasNoMigratedOwnerCommands(t *testing.T) {
	banned := map[string]string{
		`"ze-clear:vpn-ipsec-sa"`:       "IPsec clear -> internal/component/ike/schema",
		`"ze-clear:dns-cache"`:          "DNS cache clear -> internal/component/resolve/schema",
		`"ze-clear:interface-counters"`: "interface counters clear -> internal/component/iface/schema",
	}
	for token, owner := range banned {
		if strings.Contains(ZeCliClearCmdYANG, token) {
			t.Errorf("central clear schema declares owner command %q; move it to %s (see ai/rules/plugin-self-containment.md)", token, owner)
		}
	}
}

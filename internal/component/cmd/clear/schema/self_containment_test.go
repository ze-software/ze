package schema

import (
	"strings"
	"testing"
)

func TestClearSchemaHasNoMigratedOwnerCommands(t *testing.T) {
	banned := map[string]string{
		`"ze-clear:vpn-ipsec-sa"`: "IPsec clear -> internal/component/ike/schema",
	}
	for token, owner := range banned {
		if strings.Contains(ZeCliClearCmdYANG, token) {
			t.Errorf("central clear schema declares owner command %q; move it to %s (see ai/rules/plugin-self-containment.md)", token, owner)
		}
	}
}

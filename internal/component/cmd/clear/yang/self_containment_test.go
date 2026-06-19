package yang

import (
	"strings"
	"testing"
)

func TestClearOwnerRemovalLeavesNoResidue(t *testing.T) {
	banned := map[string]string{
		`"ze-clear:vpn-ipsec-sa"`:       "IPsec clear -> internal/component/ike/yang",
		`"ze-clear:dns-cache"`:          "DNS cache clear -> internal/plugins/resolve-cmd/yang",
		`"ze-clear:interface-counters"`: "interface counters clear -> internal/component/iface/yang",
		`"ze-l2tp-api:`:                 "L2TP clear -> internal/component/cmd/l2tp (already owned)",
		`"ze-clear:isis-adjacency"`:     "IS-IS adjacency clear -> internal/component/isis/yang",
		`"ze-clear:isis-counters"`:      "IS-IS counters clear -> internal/component/isis/yang",
	}
	for token, owner := range banned {
		if strings.Contains(ZeCliClearCmdYANG, token) {
			t.Errorf("central clear schema contains owner token %q; owner removal would leave a dangling node (owner: %s)", token, owner)
		}
		if strings.Contains(ZeCliClearAPIYANG, token) {
			t.Errorf("central clear API schema contains owner token %q; owner removal would leave a dangling RPC (owner: %s)", token, owner)
		}
	}
}

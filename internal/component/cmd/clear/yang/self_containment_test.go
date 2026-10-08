package yang

import (
	"strings"
	"testing"
)

func TestClearOwnerRemovalLeavesNoResidue(t *testing.T) {
	banned := map[string]string{
		`"ze-ike:clear-vpn-ipsec-sa"`:       "IPsec clear -> internal/component/ike/yang",
		`"ze-resolve:clear-dns-cache"`:          "DNS cache clear -> internal/plugins/resolve-cmd/yang",
		`"ze-iface:clear-interface-counters"`: "interface counters clear -> internal/component/iface/yang",
		`"ze-l2tp-api:`:                 "L2TP clear -> internal/component/l2tp/cmd (already owned)",
		`"ze-isis:clear-adjacency"`:     "IS-IS adjacency clear -> internal/plugins/isis/yang",
		`"ze-isis:clear-counters"`:      "IS-IS counters clear -> internal/plugins/isis/yang",
		`"ze-clear:ospf-`:               "OSPF clear -> internal/plugins/ospf/yang",
		`"ze-vrrp:clear-statistics"`:    "VRRP statistics clear -> internal/plugins/vrrp/yang",
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

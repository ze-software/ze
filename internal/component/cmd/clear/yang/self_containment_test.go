package yang

import (
	"strings"
	"testing"
)

func TestClearOwnerRemovalLeavesNoResidue(t *testing.T) {
	banned := map[string]string{
		`"ze-ike:clear-vpn-ipsec-sa"`:         "IPsec clear -> internal/component/ike/yang",
		`"ze-resolve:clear-dns-cache"`:        "DNS cache clear -> internal/plugins/resolve-cmd/yang",
		`"ze-iface:clear-interface-counters"`: "interface counters clear -> internal/component/iface/yang",
		`"ze-l2tp:`:                           "L2TP, PPPoE and subscriber commands -> internal/component/l2tp/{cmd,pppoe/cmd,subscriber/cmd}/yang",
		`"ze-l2tp-api:`:                       "L2TP rpc pointers -> internal/component/l2tp/yang",
		`"ze-isis:clear-adjacency"`:           "IS-IS adjacency clear -> internal/plugins/isis/yang",
		`"ze-isis:clear-counters"`:            "IS-IS counters clear -> internal/plugins/isis/yang",
		`"ze-ospf:clear-`:                     "OSPF clear -> internal/plugins/ospf/yang",
		`"ze-vrrp:clear-statistics"`:          "VRRP statistics clear -> internal/plugins/vrrp/yang",
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

// TestClearSchemaNamesNoOwnerCommand derives the owner check rather than
// listing owners.
//
// VALIDATES: every ze:command in the central clear schemas carries the
// ze-cmd: prefix and every ze:rpc names this package's own API module, so a
// node an owner (OSPF, L2TP, BGP, ...) declares under its own prefix cannot
// sit here, whichever owner it is.
//
// PREVENTS: a clear command drifting back into the central schema under an
// owner prefix that the hand-listed tokens above do not name.
func TestClearSchemaNamesNoOwnerCommand(t *testing.T) {
	for name, text := range map[string]string{"ze-cli-clear-cmd": ZeCliClearCmdYANG, "ze-cli-clear-api": ZeCliClearAPIYANG} {
		for _, bad := range foreignPrefixes(text, "ze:command", "ze-cmd") {
			t.Errorf("%s declares owner command %q; it belongs in the owner's schema (see ai/rules/plugins.md)", name, bad)
		}
		for _, bad := range foreignPrefixes(text, "ze:rpc", "ze-cli-clear-api") {
			t.Errorf("%s points at owner rpc %q; it belongs in the owner's schema (see ai/rules/plugins.md)", name, bad)
		}
	}
}

// foreignPrefixes returns each `<extension> "<prefix>:..."` argument whose
// prefix is not own.
func foreignPrefixes(text, extension, own string) []string {
	var foreign []string
	for line := range strings.Lines(text) {
		_, arg, found := strings.Cut(line, extension+` "`)
		if !found {
			continue
		}
		value, _, _ := strings.Cut(arg, `"`)
		prefix, _, _ := strings.Cut(value, ":")
		if prefix != own {
			foreign = append(foreign, value)
		}
	}
	return foreign
}

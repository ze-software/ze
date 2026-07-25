package rib

import (
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// TestFormatFamily pins formatFamily to the family registry's canonical names.
//
// VALIDATES: formatFamily renders each family's registered canonical name, so
// the flowspec SAFI shows as "flow" (the name the registry, config, and .ci
// tests all use) and any plugin-registered family stays consistent, with the
// numeric "afi-N/safi-N" fallback preserved for unregistered families.
// PREVENTS: the former hardcoded AFI/SAFI switch drifting from the registry —
// it emitted "ipv4/flowspec" while every other surface uses "ipv4/flow".
func TestFormatFamily(t *testing.T) {
	family.RegisterTestFamilies()

	tests := []struct {
		fam  family.Family
		want string
	}{
		{family.IPv4Unicast, "ipv4/unicast"},
		{family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}, "ipv4/flow"},
		{family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}, "ipv4/mpls-vpn"},
		{family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIMPLSLabel}, "ipv6/mpls-label"},
		// Numeric fallback for an unregistered AFI/SAFI pair.
		{family.Family{AFI: family.AFI(65000), SAFI: family.SAFI(250)}, "afi-65000/safi-250"},
	}
	for _, tt := range tests {
		if got := formatFamily(tt.fam); got != tt.want {
			t.Errorf("formatFamily(%v) = %q, want %q", tt.fam, got, tt.want)
		}
	}
}

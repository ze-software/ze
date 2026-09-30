// VALIDATES: a config delivery with no vpn section parses to the YANG default
// catch-all disposition, bypass
// PREVENTS: applyConfig refusing a delivery that carries only pki (or nothing) with
// "unmatched disposition 0", because the empty config left Unmatched at the
// SPAction zero value (PROTECT), which installUnmatched rightly refuses
package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestConfigWithoutVPNSectionCarriesBypass proves both section parsers answer a
// delivery with no vpn root with Unmatched = bypass, the same value
// ipsec.ParseIPsecConfig answers for a tree with no ipsec block.
//
// Method: each parser over no sections, and parseVPNSections over a pki-only
// delivery as well. parseIPsecSections is not handed a pki section here because it
// installs it into the process-wide PKI store, which other tests in this package
// read; the vpn-root loop that answers the empty config is the same in both.
// The result must be bypass, never zero.
func TestConfigWithoutVPNSectionCarriesBypass(t *testing.T) {
	cases := []struct {
		name     string
		parse    func([]sdk.ConfigSection) (*ipsec.IPsecConfig, error)
		sections []sdk.ConfigSection
	}{
		{"parseVPNSections, no sections", parseVPNSections, nil},
		{"parseVPNSections, pki only", parseVPNSections, []sdk.ConfigSection{{Root: configRootPKI, Data: "{}"}}},
		{"parseIPsecSections, no sections", parseIPsecSections, nil},
	}
	for _, c := range cases {
		cfg, err := c.parse(c.sections)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if cfg.Unmatched != dataplane.SPActionBypass {
			t.Errorf("%s: Unmatched = %d; want bypass (%d), the YANG default",
				c.name, cfg.Unmatched, dataplane.SPActionBypass)
		}
	}
}

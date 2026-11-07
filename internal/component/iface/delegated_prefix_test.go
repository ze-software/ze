package iface

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
)

// TestDelegatedSubnetDerivation checks the /64 and address a LAN unit takes
// from a delegated prefix (RFC 7084 L-2, AC-7). Method: derive from a /56 with
// the default and an explicit interface identifier, from a delegation shorter
// than the hint, and refuse an id the server's delegation cannot hold.
func TestDelegatedSubnetDerivation(t *testing.T) {
	one := netip.MustParseAddr("::1")
	two := netip.MustParseAddr("::2")
	cases := []struct {
		name      string
		delegated string
		subnetID  uint16
		ifid      netip.Addr
		want      string
	}{
		{"subnet 1 of a /56, default host", "2001:db8:aa00::/56", 1, one, "2001:db8:aa00:1::1/64"},
		{"subnet 1 of a /56, host ::2", "2001:db8:aa00::/56", 1, two, "2001:db8:aa00:1::2/64"},
		{"last subnet of a /56", "2001:db8:aa00::/56", 255, one, "2001:db8:aa00:ff::1/64"},
		{"host bits of the delegation ignored", "2001:db8:aa00:5::/56", 2, one, "2001:db8:aa00:2::1/64"},
		{"a /64 delegation holds subnet 0", "2001:db8:aa00:7::/64", 0, one, "2001:db8:aa00:7::1/64"},
		{"a /48 holds the full id range", "2001:db8:aa::/48", 0xffff, one, "2001:db8:aa:ffff::1/64"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := deriveDelegatedAddress(netip.MustParsePrefix(tc.delegated), tc.subnetID, tc.ifid)
			if err != nil {
				t.Fatalf("derive: %v", err)
			}
			if got.String() != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}

	// The server chose a /60 although the hint was /56: subnet 16 does not fit
	// (RFC 7084 RFC7084-4.2-31 logs this at run time; the unit takes no /64).
	_, err := deriveDelegatedAddress(netip.MustParsePrefix("2001:db8:aa00::/60"), 16, one)
	if !errors.Is(err, errDelegationTooSmall) {
		t.Fatalf("subnet 16 of a /60: err %v, want errDelegationTooSmall", err)
	}
	if !strings.Contains(err.Error(), "0 to 15") {
		t.Fatalf("error %q does not name the valid range 0 to 15", err)
	}
	_, err = deriveDelegatedAddress(netip.MustParsePrefix("2001:db8:aa00::/80"), 0, one)
	if !errors.Is(err, errDelegationTooSmall) {
		t.Fatalf("a /80 delegation: err %v, want errDelegationTooSmall", err)
	}
}

// delegatedUnitJSON is an ethernet config whose two units carry the given
// delegated-prefix entries.
func delegatedUnitJSON(unit0, unit1 string) string {
	return `{
		"interface": {
			"ethernet": {
				"lan0": {"unit": {"0": {"ipv6": {"delegated-prefix": ` + unit0 + `}}}},
				"lan1": {"unit": {"0": {"ipv6": {"delegated-prefix": ` + unit1 + `}}}}
			}
		}
	}`
}

// withPPPoESource adds the pppoe-client pppoe0, requesting a delegated prefix
// of pdLength (0: no hint).
func withPPPoESource(cfg *ifaceConfig, pdLength int) {
	cfg.PPPoEClient = append(cfg.PPPoEClient, pppoeClientEntry{
		Name: "pppoe0",
		IPv6: PPPoEClientIPv6{DHCPv6: dhcpv6UnitConfig{Enabled: true, PDRequested: true, PDLength: pdLength}},
	})
}

// TestDelegatedPrefixVerify checks every refusal AC-16 names and the boundary
// of the subnet id. Method: parse two LAN units, add the pppoe-client the
// case needs, and run the verify step parseAndVerifyIfaceSections runs.
func TestDelegatedPrefixVerify(t *testing.T) {
	t.Run("last valid id beside pd length 56", func(t *testing.T) {
		cfg := mustParseIfaceJSON(t, delegatedUnitJSON(`{"pppoe0": {"subnet-id": "255"}}`, `{"pppoe0": {"subnet-id": "0"}}`))
		withPPPoESource(cfg, 56)
		if err := verifyDelegatedPrefixes(cfg); err != nil {
			t.Fatalf("subnet-id 255 with pd length 56 refused: %v", err)
		}
	})
	t.Run("id above the pd length range names the largest", func(t *testing.T) {
		cfg := mustParseIfaceJSON(t, delegatedUnitJSON(`{"pppoe0": {"subnet-id": "256"}}`, `{"pppoe0": {"subnet-id": "0"}}`))
		withPPPoESource(cfg, 56)
		err := verifyDelegatedPrefixes(cfg)
		if err == nil || !strings.Contains(err.Error(), "0 to 255") {
			t.Fatalf("subnet-id 256 with pd length 56: err %v, want a refusal naming 0 to 255", err)
		}
	})
	t.Run("no pd length allows the /48 maximum", func(t *testing.T) {
		cfg := mustParseIfaceJSON(t, delegatedUnitJSON(`{"pppoe0": {"subnet-id": "65535"}}`, `{"pppoe0": {"subnet-id": "0"}}`))
		withPPPoESource(cfg, 0)
		if err := verifyDelegatedPrefixes(cfg); err != nil {
			t.Fatalf("subnet-id 65535 with no pd length refused: %v", err)
		}
	})
	t.Run("duplicate source and id names both units", func(t *testing.T) {
		cfg := mustParseIfaceJSON(t, delegatedUnitJSON(`{"pppoe0": {"subnet-id": "1"}}`, `{"pppoe0": {"subnet-id": "1"}}`))
		withPPPoESource(cfg, 56)
		err := verifyDelegatedPrefixes(cfg)
		if err == nil {
			t.Fatal("two units on subnet-id 1 of pppoe0 accepted")
		}
		for _, unit := range []string{"lan0 unit 0", "lan1 unit 0"} {
			if !strings.Contains(err.Error(), unit) {
				t.Fatalf("error %q does not name %s", err, unit)
			}
		}
	})
	t.Run("unknown source names the fix", func(t *testing.T) {
		cfg := mustParseIfaceJSON(t, delegatedUnitJSON(`{"wan9": {"subnet-id": "1"}}`, `{"pppoe0": {"subnet-id": "2"}}`))
		withPPPoESource(cfg, 56)
		err := verifyDelegatedPrefixes(cfg)
		if err == nil || !strings.Contains(err.Error(), "pppoe-client wan9 ipv6 dhcpv6") {
			t.Fatalf("unknown source wan9: err %v, want a refusal naming the pppoe-client fix", err)
		}
	})
	t.Run("a client without pd is not a source", func(t *testing.T) {
		cfg := mustParseIfaceJSON(t, delegatedUnitJSON(`{"pppoe0": {"subnet-id": "1"}}`, `{"pppoe0": {"subnet-id": "2"}}`))
		cfg.PPPoEClient = append(cfg.PPPoEClient, pppoeClientEntry{
			Name: "pppoe0",
			IPv6: PPPoEClientIPv6{DHCPv6: dhcpv6UnitConfig{Enabled: true}},
		})
		if err := verifyDelegatedPrefixes(cfg); err == nil {
			t.Fatal("a pppoe-client with no pd accepted as a delegated-prefix source")
		}
	})
}

// TestDelegatedPrefixParse checks the parser: the interface-identifier
// default, and the refusal of a value with high bits or an all-zero host.
func TestDelegatedPrefixParse(t *testing.T) {
	got, err := parseDelegatedPrefixes(map[string]any{"delegated-prefix": map[string]any{
		"pppoe0": map[string]any{"subnet-id": "1"},
	}})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 || got[0].SubnetID != 1 || got[0].InterfaceID != netip.MustParseAddr("::1") {
		t.Fatalf("got %+v, want subnet 1 with interface-identifier ::1", got)
	}
	for _, bad := range []string{"2001:db8::1", "::"} {
		_, err := parseDelegatedPrefixes(map[string]any{"delegated-prefix": map[string]any{
			"pppoe0": map[string]any{"subnet-id": "1", "interface-identifier": bad},
		}})
		if err == nil {
			t.Fatalf("interface-identifier %s accepted", bad)
		}
	}
}

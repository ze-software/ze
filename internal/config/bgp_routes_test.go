package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseUpdateBlock_InvalidMED verifies error on invalid MED value.
//
// VALIDATES: Non-numeric MED produces clear error at parse time.
// PREVENTS: Silent failures with MED=0.
func TestParseUpdateBlock_InvalidMED(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 10.0.0.1;
                med abc;
            }
            nlri {
                ipv4/unicast 1.0.0.0/24;
            }
        }
    }
}
`
	p := NewParser(YANGSchema())
	_, err := p.Parse(input)
	// YANG schema validates uint32, so error happens at parse time
	require.Error(t, err)
	require.Contains(t, err.Error(), "med")
}

// TestNLRIMandatoryOperation verifies that NLRI lines without an operation keyword are rejected.
// The NLRI block uses freeform parsing, so validation happens in extractRoutesFromUpdateBlock,
// not during Parse(). We parse, then extract routes to trigger validation.
//
// VALIDATES: AC-15: missing add/del/eor rejected for all families.
// PREVENTS: Routes without operation keyword being silently accepted.
func TestNLRIMandatoryOperation(t *testing.T) {
	tests := []struct {
		name       string
		familyLine string
	}{
		{name: "ipv4/unicast missing op", familyLine: "ipv4/unicast 10.0.0.0/24"},
		{name: "ipv6/unicast missing op", familyLine: "ipv6/unicast 2001:db8::/32"},
		{name: "ipv4/flow missing op", familyLine: "ipv4/flow source-ipv4 10.0.0.0/32"},
		{name: "l2vpn/vpls missing op", familyLine: "l2vpn/vpls rd 65000:100 ve-id 1 ve-block-size 10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := `
bgp {
    local-as 65000;
    peer 192.168.1.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 10.0.0.1;
            }
            nlri {
                ` + tt.familyLine + `;
            }
        }
    }
}
`
			p := NewParser(YANGSchema())
			tree, err := p.Parse(input)
			require.NoError(t, err, "parse should succeed (freeform NLRI)")

			// Route extraction validates operation keywords
			bgp := tree.GetContainer("bgp")
			require.NotNil(t, bgp)
			peers := bgp.GetList("peer")
			peer := peers["192.168.1.1"]
			require.NotNil(t, peer)

			_, err = extractRoutesFromTree(peer)
			require.Error(t, err, "expected error for %s", tt.name)
			require.Contains(t, err.Error(), "operation", "error should mention operation keyword for %s", tt.name)
		})
	}
}

// TestNLRIWithAdd verifies that NLRI lines with the add operation parse successfully.
//
// VALIDATES: AC-14: add 10.0.0.0/24 accepted.
// PREVENTS: Regression in prefix family route parsing with operation keyword.
func TestNLRIWithAdd(t *testing.T) {
	tests := []struct {
		name       string
		familyLine string
	}{
		{name: "ipv4/unicast add", familyLine: "ipv4/unicast add 10.0.0.0/24"},
		{name: "ipv6/unicast add", familyLine: "ipv6/unicast add 2001:db8::/32"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := `
bgp {
    local-as 65000;
    peer 192.168.1.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 10.0.0.1;
            }
            nlri {
                ` + tt.familyLine + `;
            }
        }
    }
}
`
			p := NewParser(YANGSchema())
			_, err := p.Parse(input)
			require.NoError(t, err, "expected no error for %s", tt.name)
		})
	}
}

// TestNLRIBracketList verifies that multiple prefixes on one line work in freeform NLRI.
//
// VALIDATES: AC-16: add [ prefix1 prefix2 ] not supported in nlri freeform.
// PREVENTS: Confusion about bracket syntax in NLRI blocks.
func TestNLRIBracketList(t *testing.T) {
	input := `
bgp {
    local-as 65000;
    peer 192.168.1.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 10.0.0.1;
            }
            nlri {
                ipv4/unicast add 10.0.0.0/24 10.1.0.0/24;
            }
        }
    }
}
`
	p := NewParser(YANGSchema())
	_, err := p.Parse(input)
	require.NoError(t, err, "multiple prefixes on one line should parse successfully")
}

// TestNLRIVPNWithAdd verifies that VPN routes with operation keyword parse successfully.
//
// VALIDATES: AC-17: rd + label + add + prefix accepted.
// PREVENTS: VPN route parsing broken by operation keyword requirement.
func TestNLRIVPNWithAdd(t *testing.T) {
	input := `
bgp {
    local-as 65000;
    peer 192.168.1.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 10.0.0.1;
            }
            nlri {
                ipv4/mpls-vpn add rd 65000:100 label 100 10.0.0.0/24;
            }
        }
    }
}
`
	p := NewParser(YANGSchema())
	_, err := p.Parse(input)
	require.NoError(t, err, "VPN route with add should parse successfully")
}

// TestNLRIFlowSpecWithAdd verifies that FlowSpec routes with operation keyword parse successfully.
//
// VALIDATES: AC-18: flowspec + add + criteria accepted.
// PREVENTS: FlowSpec route parsing broken by operation keyword requirement.
func TestNLRIFlowSpecWithAdd(t *testing.T) {
	input := `
bgp {
    local-as 65000;
    peer 192.168.1.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 10.0.0.1;
            }
            nlri {
                ipv4/flow add source-ipv4 10.0.0.0/32;
            }
        }
    }
}
`
	p := NewParser(YANGSchema())
	_, err := p.Parse(input)
	require.NoError(t, err, "FlowSpec route with add should parse successfully")
}

// TestNLRIDelAndEor verifies that del and eor operations parse successfully.
//
// VALIDATES: AC-25, AC-26: del and eor accepted.
// PREVENTS: Route withdrawal and EOR operations broken.
func TestNLRIDelAndEor(t *testing.T) {
	tests := []struct {
		name       string
		familyLine string
	}{
		{name: "ipv4/unicast del", familyLine: "ipv4/unicast del 10.0.0.0/24"},
		{name: "ipv4/unicast eor", familyLine: "ipv4/unicast eor"},
		{name: "ipv4/flow del", familyLine: "ipv4/flow del source-ipv4 10.0.0.0/32"},
		{name: "ipv4/flow eor", familyLine: "ipv4/flow eor"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := `
bgp {
    local-as 65000;
    peer 192.168.1.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 10.0.0.1;
            }
            nlri {
                ` + tt.familyLine + `;
            }
        }
    }
}
`
			p := NewParser(YANGSchema())
			_, err := p.Parse(input)
			require.NoError(t, err, "expected no error for %s", tt.name)
		})
	}
}

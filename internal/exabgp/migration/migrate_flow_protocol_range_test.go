// Tests for an ExaBGP flow route whose protocol or next-header is a range. The
// migration passes the operators through, and the ze config reads them with the
// update text grammar (config_builder.go parseFlowProtocolMatches), which
// TestParseConfigRoute_FlowSpecMigratedProtocolRange drives with these lines.

package migration

import (
	"strings"
	"testing"
)

// TestMigrateFlowProtocolRange proves a protocol and a next-header range reach
// the migrated nlri line with every operator kept.
//
// VALIDATES: `protocol [ >5 <10 ]` and `next-header >=6&<=17` appear in the
// update block unchanged, the form the ze config now reads.
// PREVENTS: the migration rewriting a range into an equality list, or ze
// refusing the migrated config, which it did while the config protocol parser
// read `=` and `!=` only.
func TestMigrateFlowProtocolRange(t *testing.T) {
	input := `neighbor 127.0.0.1 {
	router-id 10.0.0.1;
	local-address 127.0.0.1;
	local-as 65500;
	peer-as 65500;
	family {
		ipv4 flow;
		ipv6 flow;
	}
	flow {
		route v4-range {
			match {
				destination 192.168.0.1/32;
				protocol [ >5 <10 ];
			}
			then {
				discard;
			}
		}
		route v6-range {
			match {
				destination 2001:db8::1/128;
				next-header >=6&<=17;
			}
			then {
				discard;
			}
		}
	}
}
`
	tree, err := ParseExaBGPConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	output := SerializeTree(result.Tree)
	for _, want := range []string{"protocol [ >5 <10 ]", "next-header >=6&<=17"} {
		if !strings.Contains(output, want) {
			t.Errorf("migrated config lacks %q:\n%s", want, output)
		}
	}
}

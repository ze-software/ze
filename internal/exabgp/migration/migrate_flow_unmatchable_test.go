// Tests for the flow route that no packet can match: an ICMP type or code beside
// a port. ExaBGP 6.0 drops it at load with a warning, and the migration does the
// same, so the migrated config loads where the ExaBGP one did and announces what
// ExaBGP announced.

package migration

import (
	"strings"
	"testing"
)

// TestMigrateFlowUnmatchableDropped proves an unmatchable flow route is left out
// of the migrated config with a warning naming it, while its sibling is kept.
//
// VALIDATES: a route with icmp-type and destination-port produces no update block
// and one warning naming the route; the matchable route beside it still migrates.
// PREVENTS: the migrated config announcing a route ExaBGP 6.0 does not send
// (RFC 8955 Section 4.2), or ze refusing the whole migrated config for it.
func TestMigrateFlowUnmatchableDropped(t *testing.T) {
	input := `neighbor 127.0.0.1 {
	router-id 10.0.0.1;
	local-address 127.0.0.1;
	local-as 65500;
	peer-as 65500;
	family {
		ipv4 flow;
	}
	flow {
		route icmp-and-port {
			match {
				destination 192.168.0.1/32;
				destination-port =80;
				icmp-type [ echo-request ];
			}
			then {
				discard;
			}
		}
		route plain {
			match {
				destination 192.168.0.2/32;
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
		t.Fatalf("ParseExaBGPConfig: %v", err)
	}
	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("MigrateFromExaBGP: %v", err)
	}
	output := SerializeTree(result.Tree)
	if strings.Contains(output, "192.168.0.1/32") {
		t.Errorf("the unmatchable route was migrated:\n%s", output)
	}
	if !strings.Contains(output, "192.168.0.2/32") {
		t.Errorf("the matchable route was lost:\n%s", output)
	}
	var named int
	for _, w := range result.Warnings {
		if strings.Contains(w, "icmp-and-port") {
			named++
		}
	}
	if named != 1 {
		t.Errorf("want one warning naming icmp-and-port, got %q", result.Warnings)
	}
}

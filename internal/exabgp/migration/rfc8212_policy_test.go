// Design: docs/exabgp/exabgp-migration.md -- migrated peers preserve explicit exchange intent.
// RFC naming: untagged -- red migration wiring probe; conformance records belong to the implementation phase.
package migration

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// TestMigrateWritesExplicitRFC8212Policies drives the public parser and migrator
// for external, internal and inherited neighbors. Every resulting peer must
// carry both policies; a global accept would also permit newly added peers.
func TestMigrateWritesExplicitRFC8212Policies(t *testing.T) {
	input := `
template {
	neighbor base {
		local-as 65000;
		peer-as 65002;
	}
}
neighbor 192.0.2.1 {
	local-as 65000;
	peer-as 65001;
}
neighbor 192.0.2.2 {
	local-as 65000;
	peer-as 65000;
}
neighbor 192.0.2.3 {
	inherit base;
}
`
	_, result := migrateText(t, input)
	bgp := result.Tree
	if bgp == nil {
		t.Fatal("migration produced no configuration tree")
	}
	if bgp.GetContainer("filter") != nil {
		t.Error("migration added a global policy instead of preserving intent per peer")
	}
	peers := 0
	for _, group := range bgp.GetListOrdered("group") {
		if group.Value.GetContainer("filter") != nil {
			t.Errorf("group %s has policy that would permit a newly added peer", group.Key)
		}
		for _, peer := range group.Value.GetListOrdered("peer") {
			peers++
			filter := peer.Value.GetContainer("filter")
			if filter == nil {
				t.Errorf("peer %s has no explicit import or export policy", peer.Key)
				continue
			}
			for _, direction := range []string{"import", "export"} {
				policy := filter.GetContainer(direction)
				if policy == nil {
					t.Errorf("peer %s has no explicit %s policy", peer.Key, direction)
					continue
				}
				final, present := policy.Get("final")
				if !present {
					t.Errorf("peer %s %s policy has no final", peer.Key, direction)
					continue
				}
				if final != "accept" {
					t.Errorf("peer %s %s final = %q, want accept", peer.Key, direction, final)
				}
			}
		}
	}
	if peers != 3 {
		t.Errorf("migrated %d peers, want all three neighbors", peers)
	}
	notes := strings.Join(result.Warnings, "\n")
	if !strings.Contains(notes, "RFC 8212") {
		t.Errorf("migration does not explain explicit policies: %q", notes)
	}
}

// TestMigrateRFC8212PoliciesSurviveSerialization checks the native parser's
// peer trees, not just the intermediate migration tree. Runtime exchange is
// exercised separately by the compatibility suites with their real scripts.
func TestMigrateRFC8212PoliciesSurviveSerialization(t *testing.T) {
	input := `
template {
	neighbor base {
		local-as 65000;
		peer-as 65002;
		family { ipv4 unicast; }
	}
}
neighbor 192.0.2.1 {
	local-as 65000;
	peer-as 65001;
	family { ipv4 unicast; }
}
neighbor 192.0.2.2 {
	local-as 65000;
	peer-as 65000;
	family { ipv4 unicast; }
}
neighbor 192.0.2.3 {
	inherit base;
}
`
	output, result := migrateText(t, input)
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("load native schema: %v", err)
	}
	native, err := config.NewParser(schema).Parse(output)
	if err != nil {
		t.Fatalf("parse migrated configuration: %v\n%s", err, output)
	}
	bgp := native.GetContainer("bgp")
	if bgp == nil {
		t.Fatal("serialized configuration has no bgp root")
	}
	if bgp.GetContainer("filter") != nil {
		t.Error("serialized configuration grants global policy")
	}
	peers := 0
	for _, group := range bgp.GetListOrdered("group") {
		if group.Value.GetContainer("filter") != nil {
			t.Errorf("serialized group %s grants policy", group.Key)
		}
		for _, peer := range group.Value.GetListOrdered("peer") {
			peers++
			filter := peer.Value.GetContainer("filter")
			if filter == nil {
				t.Errorf("serialized peer %s has no filter", peer.Key)
				continue
			}
			for _, direction := range []string{"import", "export"} {
				policy := filter.GetContainer(direction)
				if policy == nil {
					t.Errorf("serialized peer %s has no %s policy", peer.Key, direction)
					continue
				}
				if final, present := policy.Get("final"); !present || final != "accept" {
					t.Errorf("serialized peer %s %s final = %q, present=%t", peer.Key, direction, final, present)
				}
			}
		}
	}
	if peers != 3 {
		t.Errorf("serialized %d peers, want external, internal and inherited", peers)
	}
	for _, peerName := range []string{"peer-1", "peer-2", "peer-3"} {
		warned := false
		for _, warning := range result.Warnings {
			if strings.Contains(warning, "peer "+peerName+":") &&
				strings.Contains(warning, "RFC 8212") &&
				strings.Contains(warning, "preserve ExaBGP route-exchange intent") {
				warned = true
			}
		}
		if !warned {
			t.Errorf("no RFC 8212 warning for %s: %q", peerName, result.Warnings)
		}
	}
}

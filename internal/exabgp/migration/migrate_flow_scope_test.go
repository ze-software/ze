// Tests for the flow route scope block: the interface-set extended communities
// an ExaBGP `scope { }` section carries, and the eight octets ze must send for
// each one.

package migration

import (
	"encoding/hex"
	"strings"
	"testing"

	bgpconfig "github.com/ze-software/ze/internal/component/bgp/config"
)

// TestInterfaceSetExtCommunityForms pins the ze text each ExaBGP interface-set
// value migrates to. The octets behind that text are pinned once, where the
// attribute package encodes them (TestInterfaceSetEncode).
//
// VALIDATES: both ExaBGP forms, both transitivity types and the three
// directions migrate to the four-field name every ze parser reads.
// PREVENTS: the three-field form losing its transitivity on the way to ze.
func TestInterfaceSetExtCommunityForms(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"transitive input", "transitive:input:1234:10", "interface-set:transitive:input:1234:10"},
		{"transitive output", "transitive:output:1234:10", "interface-set:transitive:output:1234:10"},
		{"transitive both", "transitive:input-output:1234:10", "interface-set:transitive:input-output:1234:10"},
		{"non transitive", "non-transitive:input:3405770241:1", "interface-set:non-transitive:input:3405770241:1"},
		// The three-field form is the older one and defaults to transitive.
		{"no transitivity given", "input:1234:10", "interface-set:transitive:input:1234:10"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := interfaceSetExtCommunity(tc.value)
			if err != nil {
				t.Fatalf("interfaceSetExtCommunity(%q): %v", tc.value, err)
			}
			if got != tc.want {
				t.Errorf("interfaceSetExtCommunity(%q) = %s, want %s", tc.value, got, tc.want)
			}
		})
	}
}

// TestInterfaceSetExtCommunityRefusals proves each malformed value is reported
// rather than encoded to something else.
//
// VALIDATES: the value is refused when transitivity, direction, AS number or
// group identifier is outside what ExaBGP accepts.
// PREVENTS: a truncated group identifier or a zero AS number reaching the wire
// in place of the value the operator wrote.
func TestInterfaceSetExtCommunityRefusals(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{"unknown transitivity", "maybe:input:1234:10"},
		{"unknown direction", "transitive:sideways:1234:10"},
		{"no direction", "1234:10"},
		{"dotted as number", "transitive:input:1.234:10"},
		{"as number too large", "transitive:input:4294967296:10"},
		{"group id above 14 bits", "transitive:input:1234:16384"},
		{"group id not a number", "transitive:input:1234:ten"},
		{"empty", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := interfaceSetExtCommunity(tc.value)
			if err == nil {
				t.Fatalf("interfaceSetExtCommunity(%q) = %s, want an error", tc.value, got)
			}
		})
	}
}

// TestMigrateFlowScopeExtCommunityZeAccepts reads the migrated config back
// through the ze extended community parser.
//
// VALIDATES: what the migration writes for a scope block is a value ze loads,
// and it carries the eight octets the interface set defines.
// PREVENTS: the migration emitting a spelling parseOneExtCommunity refuses, so
// the migrated config would be refused by the daemon it was written for.
func TestMigrateFlowScopeExtCommunityZeAccepts(t *testing.T) {
	input := `
neighbor 127.0.0.1 {
	local-as 1;
	peer-as 1;
	family {
		ipv4 flow;
	}
	flow {
		route one {
			match {
				source 202.255.238.1/32;
			}
			scope {
				interface-set [ non-transitive:input:3405770241:1 transitive:output:254:254 ];
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

	// The interface sets are written before the traffic action, because ExaBGP
	// packs its extended communities in value order and 0x07/0x47 sorts before
	// the 0x80 of a traffic action.
	want := "extended-community [interface-set:non-transitive:input:3405770241:1 interface-set:transitive:output:254:254 rate-limit:0]"
	if !strings.Contains(output, want) {
		t.Fatalf("migrated config does not carry %q:\n%s", want, output)
	}

	extComm, err := bgpconfig.ParseExtendedCommunity(want[len("extended-community "):])
	if err != nil {
		t.Fatalf("ze refuses the extended community the migration wrote: %v", err)
	}

	// The three communities in wire order: the two interface sets, then the
	// rate limit of zero that discards the traffic.
	gotOctets := hex.EncodeToString(extComm.Bytes)
	wantOctets := "4702caffee0140010702000000fe80fe" + hex.EncodeToString(extComm.Bytes[16:])
	if gotOctets != wantOctets {
		t.Errorf("extended community octets = %s, want the two interface sets first: %s", gotOctets, wantOctets)
	}
}

// TestMigrateExtCommunityInterfaceSetRefused proves an interface-set written
// inside extended-community stops the migration wherever ExaBGP reads one.
//
// VALIDATES: a flow `then`, an announce route, a static route and an announce
// family route each refuse the item, naming the interface-set keyword.
// PREVENTS: the migration copying an item ExaBGP itself refuses
// (configuration/grammar/types/bgp.py, extended_community) into a ze config.
func TestMigrateExtCommunityInterfaceSetRefused(t *testing.T) {
	cases := map[string]string{
		"flow then": `flow {
		route one {
			match { source 202.255.238.1/32; }
			then { extended-community [ target:65000:1 interface-set:transitive:input:1.234:10 ]; }
		}
	}`,
		"announce route": `announce {
		ipv4 {
			unicast 10.0.0.0/24 next-hop 1.1.1.1 extended-community [ interface-set:input:1234:10 ];
		}
	}`,
		"static route": `static {
		route 10.0.0.0/24 next-hop 1.1.1.1 extended-community [ interface-set:input:1234:10 ];
	}`,
	}

	for name, block := range cases {
		t.Run(name, func(t *testing.T) {
			input := "neighbor 127.0.0.1 {\n\tlocal-as 1;\n\tpeer-as 1;\n\t" + block + "\n}\n"
			tree, err := ParseExaBGPConfig(input)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			_, err = MigrateFromExaBGP(tree)
			if err == nil {
				t.Fatal("migration accepted an interface-set inside extended-community")
			}
			if !strings.Contains(err.Error(), "write it with the interface-set keyword") {
				t.Errorf("error does not name the interface-set keyword: %v", err)
			}
		})
	}
}

// TestMigrateFlowScopeRefusesUnknownKeyword proves a scope keyword the
// migration cannot translate stops the migration.
//
// VALIDATES: an unknown keyword inside scope is reported.
// PREVENTS: a flow route being announced with a wider reach than its ExaBGP
// config gave it, because the keyword that narrowed it was dropped in silence.
func TestMigrateFlowScopeRefusesUnknownKeyword(t *testing.T) {
	input := `
neighbor 127.0.0.1 {
	local-as 1;
	peer-as 1;
	flow {
		route one {
			match {
				source 202.255.238.1/32;
			}
			scope {
				interface-group 12;
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

	_, err = MigrateFromExaBGP(tree)
	if err == nil {
		t.Fatal("migration accepted a scope keyword it cannot translate")
	}
	if !strings.Contains(err.Error(), "interface-group") {
		t.Errorf("error does not name the keyword it refused: %v", err)
	}
}

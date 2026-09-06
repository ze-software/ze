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

// TestInterfaceSetExtCommunityForms pins the octets each ExaBGP interface-set
// value produces, and the values that are refused.
//
// VALIDATES: both ExaBGP forms, both transitivity types, the three directions,
// and the 14-bit group identifier bound.
// PREVENTS: a migrated flow route reaching the wire with the wrong interface
// set, or with a group identifier that overflowed into a direction flag.
func TestInterfaceSetExtCommunityForms(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		// The type octet is 0x07, the sub-type 0x02, then the four AS octets,
		// then the direction flags above the 14-bit group identifier.
		{"transitive input", "transitive:input:1234:10", "0x0702 000004d2 400a"},
		{"transitive output", "transitive:output:1234:10", "0x0702 000004d2 800a"},
		{"transitive both", "transitive:input-output:1234:10", "0x0702 000004d2 c00a"},
		// 0x40 is the non-transitive bit on the type octet.
		{"non transitive", "non-transitive:input:3405770241:1", "0x4702 caffee01 4001"},
		// The three-field form is the older one and defaults to transitive.
		{"no transitivity given", "input:1234:10", "0x0702 000004d2 400a"},
		// Both bounds of the group identifier field.
		{"group id zero", "transitive:output:0:0", "0x0702 00000000 8000"},
		{"group id maximum", "transitive:input:1:16383", "0x0702 00000001 7fff"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := interfaceSetExtCommunity(tc.value)
			if err != nil {
				t.Fatalf("interfaceSetExtCommunity(%q): %v", tc.value, err)
			}
			want := strings.ReplaceAll(tc.want, " ", "")
			if got != want {
				t.Errorf("interfaceSetExtCommunity(%q) = %s, want %s", tc.value, got, want)
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
// PREVENTS: the migration emitting `interface-set:...`, the spelling the API
// bridge uses, which parseOneExtCommunity has no case for -- the migrated
// config would then be refused by the daemon it was written for.
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
	want := "extended-community [0x4702caffee014001 0x0702000000fe80fe rate-limit:0]"
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

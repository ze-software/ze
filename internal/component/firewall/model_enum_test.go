package firewall

import (
	"maps"
	"slices"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// TestParsedNamesMatchTheModel ties every name this package parses to the
// enumeration an operator writes the value into.
//
// The Go side is kept rather than derived, and the reason is the same for all
// six sets: each name here is paired with a value the model does not hold, a
// typed constant the nft backend lowers to a netlink value (lowerFamily,
// lowerHook), the rp_filter value a source-validation word writes, or the
// IANA number every backend programs for a protocol name. That makes the
// model the copy, and this test is what keeps the copy honest in both
// directions.
//
// The protocol enumeration is declared at two leaves, the firewall term and
// the policy-route rule; the policy-route package resolves its names through
// ProtocolNumber, so the one table is checked at the leaf this package owns.
//
// The expected values are read from the model. A list written in this file
// would agree with a Go table that had drifted away from the model just as
// happily as with one that had not.
//
// VALIDATES: the name set of each parse table equals the value set of the
// enumeration at its config leaf.
// PREVENTS: a value an operator commits, that the schema accepts, and that the
// firewall then refuses as unknown -- and the reverse, a name the Go side
// accepts that no config can carry.
func TestParsedNamesMatchTheModel(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		parsed []string
	}{
		{"table family", "firewall/table/family", nameSet(familyByName)},
		{"chain hook", "firewall/table/chain/hook", nameSet(chainHookByName)},
		{"chain type", "firewall/table/chain/type", nameSet(chainTypeByName)},
		{"set type", "firewall/table/set/type", nameSet(setTypeFromString)},
		{"source validation", "firewall/global-options/source-validation", nameSet(sourceValidationValues)},
		{"term protocol", "firewall/table/chain/term/from/protocol", ProtocolNames()},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			declared, err := configyang.EnumValues(c.path)
			if err != nil {
				t.Fatalf("read the enumeration at %s: %v", c.path, err)
			}
			if len(declared) == 0 {
				t.Fatalf("the model declares no value at %s", c.path)
			}
			if !slices.Equal(declared, c.parsed) {
				t.Errorf("the model declares %v at %s, and this package parses %v",
					declared, c.path, c.parsed)
			}
		})
	}
}

// nameSet answers the sorted names a parse table accepts, so the comparison
// against the model reads in one order whatever order the map holds.
func nameSet[T comparable](index map[string]T) []string {
	return slices.Sorted(maps.Keys(index))
}

package iface

import (
	"slices"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// TestNeighborFamilyTokensMatchTheModel ties the family words ParseNeighborFamily
// reads to the enumeration the `show neighbor` leaf declares.
//
// The Go side is the declaration: each token is paired with a backend family
// selector (4, 6, or both), which the schema does not hold. That makes the
// schema the copy, and this test is what keeps the copy honest in both
// directions.
//
// VALIDATES: every value of the enumeration at show/neighbor/family parses, and
// every token the parser knows is a value the model declares.
// PREVENTS: an operator typing a family the dispatcher accepts and the handler
// then refuses as unknown, and a token the parser knows that no operator can
// reach.
func TestNeighborFamilyTokensMatchTheModel(t *testing.T) {
	declared, err := configyang.EnumValues("show/neighbor/family")
	if err != nil {
		t.Fatalf("read the enumeration at show/neighbor/family: %v", err)
	}
	if len(declared) == 0 {
		t.Fatal("the model declares no neighbor family")
	}

	// The constants, never their spellings: the test reads what the parser
	// accepts rather than stating it a second time.
	known := []string{neighborTokenAll, neighborTokenAny, neighborTokenIPv4, neighborTokenIPv6}
	slices.Sort(known)

	if !slices.Equal(declared, known) {
		t.Errorf("the model declares %v at show/neighbor/family, and the parser knows %v", declared, known)
	}
	for _, token := range declared {
		if _, ok := ParseNeighborFamily(token); !ok {
			t.Errorf("the model declares family %q and ParseNeighborFamily refuses it", token)
		}
	}
	if _, ok := ParseNeighborFamily("ipv7"); ok {
		t.Error("ParseNeighborFamily accepted a family the model does not declare")
	}
}

// TestRPFModesMatchTheModel ties the reverse-path modes the config parser reads
// to the enumeration of the rpf-check leaf.
//
// The Go side is the declaration: each mode is paired with the value Ze writes
// to net.ipv4.conf.<iface>.rp_filter, which the schema states in prose and
// cannot enforce.
//
// VALIDATES: every value of the enumeration at the rpf-check leaf parses to a
// mode, and a value outside it does not.
// PREVENTS: a mode added to the schema that an operator commits and the config
// parser then refuses, leaving the interface with the kernel default.
func TestRPFModesMatchTheModel(t *testing.T) {
	const path = "interface/ethernet/unit/ipv4/rpf-check"

	declared, err := configyang.EnumValues(path)
	if err != nil {
		t.Fatalf("read the enumeration at %s: %v", path, err)
	}
	if len(declared) == 0 {
		t.Fatalf("the model declares no mode at %s", path)
	}

	for _, mode := range declared {
		if _, ok := parseRPFMode(mode); !ok {
			t.Errorf("the model declares rpf-check %q and parseRPFMode refuses it", mode)
		}
	}
	if _, ok := parseRPFMode("reverse"); ok {
		t.Error("parseRPFMode accepted a mode the model does not declare")
	}
}

// TestCreatableTypesMatchTheModel ties the interface types an operator can ask
// Ze to create to the enumeration the migrate command declares.
//
// VALIDATES: the enumeration at request/interface/migrate/create/type names
// exactly the three exported creatable types, and each is a type Ze supports.
// PREVENTS: a type the command offers that the backend cannot create, and a
// creatable type no command names.
func TestCreatableTypesMatchTheModel(t *testing.T) {
	const path = "request/interface/migrate/create/type"

	declared, err := configyang.EnumValues(path)
	if err != nil {
		t.Fatalf("read the enumeration at %s: %v", path, err)
	}

	creatable := []string{TypeBridge, TypeDummy, TypeVeth}
	slices.Sort(creatable)
	if !slices.Equal(declared, creatable) {
		t.Errorf("the model declares %v at %s, and the creatable types are %v", declared, path, creatable)
	}

	supported := SupportedTypes()
	for _, name := range declared {
		if !slices.Contains(supported, name) {
			t.Errorf("the model offers creating %q, which is not an interface type Ze supports", name)
		}
	}
}

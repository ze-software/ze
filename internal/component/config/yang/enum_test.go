// The test package is external so it can register a real config module: an
// in-package test cannot import one, because every module package imports this
// one to register itself.
package yang_test

import (
	"slices"
	"strings"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"

	_ "github.com/ze-software/ze/internal/component/config/system/yang" // registers ze-system-conf.yang, whose conntrack leaf-list is an enumeration
)

// TestEnumValuesAnswersTheModel proves the reader every derived Go set depends
// on answers the values the model declares, sorted.
//
// The assertions name one value and the shape of the answer rather than the
// whole list: a test holding the twelve helper names would be the third copy of
// the set this reader exists to remove.
//
// VALIDATES: EnumValues resolves a config path to the enumeration at that leaf.
// PREVENTS: a guard reading its allowed set from the model and getting an empty
// one, which reads as "nothing is allowed" with no line saying why.
func TestEnumValuesAnswersTheModel(t *testing.T) {
	values, err := configyang.EnumValues("system/conntrack/module")
	if err != nil {
		t.Fatalf("EnumValues on the conntrack module leaf-list: %v", err)
	}
	if len(values) == 0 {
		t.Fatal("EnumValues answered no value for an enumeration the model declares")
	}
	if !slices.Contains(values, "ftp") {
		t.Errorf("EnumValues answered %v, which does not carry the ftp helper the model declares", values)
	}
	if !slices.IsSorted(values) {
		t.Errorf("EnumValues answered %v, which is not sorted", values)
	}
}

// TestEnumValuesRefusesWhatIsNotAnEnumeration proves the ways a path can fail
// are errors rather than an empty set a caller reads as "nothing allowed".
//
// VALIDATES: a non-enumeration leaf and an unknown path each produce an error
// and a nil set.
// PREVENTS: a typo in a path turning a guard into one that refuses every value
// an operator writes, with no line saying why.
func TestEnumValuesRefusesWhatIsNotAnEnumeration(t *testing.T) {
	cases := []struct {
		name string
		path string
		want string
	}{
		{"a number leaf", "system/conntrack/table-size", "is not an enumeration"},
		{"an unknown leaf", "system/conntrack/absent-leaf", "resolve"},
		{"an unknown section", "absent-section/mode", "resolve"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			values, err := configyang.EnumValues(c.path)
			if err == nil {
				t.Fatalf("EnumValues(%q) answered %v, want an error", c.path, values)
			}
			if values != nil {
				t.Errorf("EnumValues(%q) answered %v beside its error, want nil", c.path, values)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("EnumValues(%q) said %q, want it to name %q", c.path, err, c.want)
			}
		})
	}
}

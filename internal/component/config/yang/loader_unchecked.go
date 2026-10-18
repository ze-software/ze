// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// Related: loader_structure.go — checkStructure joins this check
// Related: validator_lexical.go — uncheckedKindError refuses a value of these types
// RFC: rfc/short/rfc7950.md -- Section 9.1, the lexical representations ze cannot check

package yang

import (
	"errors"
	"fmt"
	"slices"

	"github.com/openconfig/goyang/pkg/yang"
)

// ErrUncheckedType marks a `type` statement naming a built-in type whose
// values ze has no check for.
var ErrUncheckedType = errors.New("YANG type ze cannot check")

// uncheckedTypeNames are the built-in types whose value check needs the data
// tree or the identity set: a leafref names a node instance, an identityref
// an identity derived from a base, an instance-identifier a path into the
// data. The validator checks one value at a time against its type, so it can
// answer none of them.
var uncheckedTypeNames = []string{"leafref", "identityref", "instance-identifier"}

// moduleUncheckedTypeErrors returns one error for each `type` statement in mod
// naming a type in uncheckedTypeNames.
//
// RFC 7950 Section 9.1: "Implementations MUST support all lexical
// representations specified in this document." A leaf whose values ze cannot
// check would have every value refused by the validator (uncheckedKindError),
// so the module is refused at load, where its author reads the reason, rather
// than on every config that sets the leaf.
func moduleUncheckedTypeErrors(mod *yang.Module) []error {
	var errs []error
	for _, typ := range moduleTypeNodes(mod) {
		if !slices.Contains(uncheckedTypeNames, typ.Name) {
			continue
		}
		errs = append(errs, fmt.Errorf("%w: module %s: %s: type %s: ze does not check %s values, "+
			"so no value of this leaf could be accepted\n  fix: declare the leaf as a string with a pattern, or as an enumeration",
			ErrUncheckedType, mod.Name, yang.Source(typ), typ.Name, typ.Name))
	}
	return errs
}

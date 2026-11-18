// Design: docs/architecture/config/yang-config-design.md -- extensions that name a registration
// Related: join.go -- refuseUnheldRegistrations, the join's walk over the entries an external module wrote
// Related: validator_registry.go -- the registry ze:validate reads
// Related: reference_source.go -- the sources ze:reference reads

package yang

import (
	"maps"
	"slices"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/core/opererr"
)

// Extension keywords that name a daemon-side registration, as a refusal
// names them.
const (
	extensionValidate  = "ze:validate"
	extensionReference = "ze:reference"
)

// unheldName answers the first registration entry names that this process
// does not hold: a ze:validate function absent from validators, or a
// ze:reference source no component registered (RegisterReferenceSource). It
// answers the extension and the name, or two empty strings when entry names
// none or every name it names is held.
//
// It is the one judge of a registration name, for an external module's entry
// (the join's refuseUnheldRegistrations) and for a compiled-in one
// (RefuseUnheldCompiledRegistrations), so both read one rule.
func unheldName(entry *gyang.Entry, validators *ValidatorRegistry) (extension, name string) {
	for _, validator := range SplitValidatorNames(GetValidateExtension(entry)) {
		if validators.Get(validator) == nil {
			return extensionValidate, validator
		}
	}
	source := GetReferenceExtension(entry)
	if source == "" {
		return "", ""
	}
	if _, held := referenceSources[source]; held {
		return "", ""
	}
	return extensionReference, source
}

// RefuseUnheldCompiledRegistrations refuses a module set in which a module
// compiled into ze names a registration this process does not hold, judged by
// unheldName, the judge the join applies to an external module. A leaf whose
// ze:validate function is absent is not checked at all, and one whose
// ze:reference source is absent refuses every value, so neither may load in
// silence.
//
// Such a name is a defect of the build, never of a config: every Ze package
// registers its names from init(), and a binary that links the module without
// the package that registers the name was built wrong. The refusal names the
// module, the extension, the name and the schema path.
//
// The walk covers every entry of every module, in name order, so the first
// unheld name is the same on every run. An external module of a published set
// was already judged by the join, which named its plugin; its names are held.
func RefuseUnheldCompiledRegistrations(set *Resolved, validators *ValidatorRegistry) error {
	names := set.ModuleNames()
	slices.Sort(names)
	for _, name := range names {
		// An explicit stack, as the join's walk: the bound is the depth of the
		// modules the set holds.
		pending := []*gyang.Entry{set.GetEntry(name)}
		for len(pending) > 0 {
			entry := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			if entry == nil {
				continue
			}
			for _, child := range slices.Sorted(maps.Keys(entry.Dir)) {
				pending = append(pending, entry.Dir[child])
			}
			extension, unheld := unheldName(entry, validators)
			if unheld == "" {
				continue
			}
			// definingModule (join_command.go) names the augmenting module
			// for a leaf another module augments in.
			return compiledUnheldRefusal(definingModule(entry), entry, extension, unheld)
		}
	}
	return nil
}

// compiledUnheldRefusal answers the refusal of a compiled-in module that names
// a registration the build does not hold. The fix is the developer's: the
// operator can change nothing in the config that mends a build.
func compiledUnheldRefusal(module string, entry *gyang.Entry, extension, name string) error {
	return opererr.New(
		"Module "+quoted(module)+", compiled into ze, names "+extension+" "+quoted(name)+" at "+entry.Path()+
			", which no package linked into this ze registers",
		"this ze was built wrong: a leaf whose ze:validate function is absent is not checked at all, "+
			"and one whose ze:reference source is absent refuses every value; the config is not at fault",
		"rebuild ze with the package that registers "+quoted(name)+" linked wherever module "+quoted(module)+" is",
	)
}

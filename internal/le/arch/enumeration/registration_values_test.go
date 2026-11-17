// Design: docs/architecture/core-design.md -- registrations declare the values a registry holds
// Related: enumeration.go -- package-local values supplied to a registrar

package archenumeration

import (
	"path/filepath"
	"testing"
)

// registrationValueFindings drives the real package walk over a synthetic
// registry, so each case isolates declaration tracking from the live corpus.
func registrationValueFindings(t *testing.T, fixture string, corpus Corpus) Findings {
	t.Helper()
	found, err := Check(filepath.Join("testdata", fixture), []string{"."}, []Corpus{corpus}, 0)
	if err != nil {
		t.Fatalf("walk %s: %v", fixture, err)
	}
	return found
}

// registrationValueCorpus holds the diagnostic keys the fixture declares.
func registrationValueCorpus() Corpus {
	return Corpus{
		Name:         "diagnostic codes",
		WrittenWhole: true,
		Groups: []Group{{Name: "diagnostic codes", Keys: []string{
			"doctor-probe-first", "doctor-probe-second", "doctor-probe-extra",
		}}},
	}
}

// TestNamedRegistrationTablesDeclareTheirConstants follows a table registered
// from another file while keeping mixed const blocks and unrelated copies visible.
func TestNamedRegistrationTablesDeclareTheirConstants(t *testing.T) {
	found := registrationValueFindings(t, "registration-named-values", registrationValueCorpus())
	if len(found) != 2 {
		t.Fatalf("want the mixed block and unrelated copy only, got:\n%s", found.Text())
	}
	want := map[string]bool{"registeredExtra": false, "unrelatedCopy": false}
	for _, finding := range found {
		if finding.Kind != KindLiteral {
			t.Fatalf("want a literal finding, got %+v", finding)
		}
		if finding.Corpus != "diagnostic codes" {
			t.Fatalf("want diagnostic codes, got %+v", finding)
		}
		if _, exists := want[finding.Symbol]; !exists {
			t.Fatalf("the registered declaration was reported: %+v", finding)
		}
		if want[finding.Symbol] {
			t.Fatalf("the copy was reported twice: %+v", finding)
		}
		want[finding.Symbol] = true
	}
}

// TestAppendedRegistrationTablesDeclareTheirConstants follows an explicit
// declaring table's appended input without excusing a second table beside it.
func TestAppendedRegistrationTablesDeclareTheirConstants(t *testing.T) {
	corpus := registrationValueCorpus()
	corpus.Declarations = []Declaration{{Package: ".", Symbol: "builtinCodes"}}
	found := registrationValueFindings(t, "registration-appended-values", corpus)
	wantOneCopy(t, found, "diagnostic codes", "unrelatedCopy")
}

// TestShadowedRegistrationTablesLeavePackageCopiesVisible prevents a local
// registration table from lending declaration status to a same-named package value.
func TestShadowedRegistrationTablesLeavePackageCopiesVisible(t *testing.T) {
	found := registrationValueFindings(t, "registration-shadowed-values", registrationValueCorpus())
	wantOneCopy(t, found, "diagnostic codes", "codeFirst")
}

// TestRegisteredComposedKeysRemainCopies keeps joined family keys reportable:
// a registrar's composed-key corpus never takes those constants as its declaration.
func TestRegisteredComposedKeysRemainCopies(t *testing.T) {
	corpus := Corpus{
		Name: "family names",
		Groups: []Group{{Name: "family names", Keys: []string{
			"ipv4/unicast", "ipv6/unicast",
		}}},
	}
	found := registrationValueFindings(t, "registration-composed-values", corpus)
	wantOneCopy(t, found, "family names", "familyFirst")
}

// TestNamedAndAppendedComposedKeysRemainCopies keeps the WrittenWhole boundary
// on the same named-table and append paths used by diagnostic registrations.
func TestNamedAndAppendedComposedKeysRemainCopies(t *testing.T) {
	for _, fixture := range []string{"registration-composed-named-values", "registration-composed-appended-values"} {
		t.Run(fixture, func(t *testing.T) {
			corpus := Corpus{
				Name: "family names",
				Groups: []Group{{Name: "family names", Keys: []string{
					"ipv4/unicast", "ipv6/unicast",
				}}},
			}
			if fixture == "registration-composed-appended-values" {
				corpus.Declarations = []Declaration{{Package: ".", Symbol: "builtinFamilies"}}
			}
			found := registrationValueFindings(t, fixture, corpus)
			wantOneCopy(t, found, "family names", "familyFirst")
		})
	}
}

// TestRegistrationTableAliasesReachOnlyTheirDeclaration follows two package
// aliases to the table supplied to the registrar while retaining an unrelated copy.
func TestRegistrationTableAliasesReachOnlyTheirDeclaration(t *testing.T) {
	found := registrationValueFindings(t, "registration-aliased-values", registrationValueCorpus())
	wantOneCopy(t, found, "diagnostic codes", "unrelatedCopy")
}

// TestRegistrationAliasCyclesLeaveCopiesVisible makes the parser-only walk
// terminate on an initializer cycle while still reporting an unrelated copy.
func TestRegistrationAliasCyclesLeaveCopiesVisible(t *testing.T) {
	found := registrationValueFindings(t, "registration-cyclic-values", registrationValueCorpus())
	wantOneCopy(t, found, "diagnostic codes", "unrelatedCopy")
}

// TestLocalConstRegistrationDoesNotDeclarePackageCopies keeps local constant
// identities separate from equal-named package constants through local aliases.
func TestLocalConstRegistrationDoesNotDeclarePackageCopies(t *testing.T) {
	found := registrationValueFindings(t, "registration-local-const-values", registrationValueCorpus())
	wantOneCopy(t, found, "diagnostic codes", "codeFirst")
}

// TestRegistrationTableWritesKeepObsoleteDeclarationsVisible rejects credit
// from an initializer replaced through its variable, an element, or an alias field.
func TestRegistrationTableWritesKeepObsoleteDeclarationsVisible(t *testing.T) {
	for _, fixture := range []string{
		"registration-overwritten-values",
		"registration-index-written-values",
		"registration-alias-field-written-values",
	} {
		t.Run(fixture, func(t *testing.T) {
			found := registrationValueFindings(t, fixture, registrationValueCorpus())
			wantOneCopy(t, found, "diagnostic codes", "codeFirst")
		})
	}
}

// TestRegistrationTableWritesDoNotInvalidateUnrelatedDeclarations keeps a
// separate table registered even when a mutated table shares its constant values.
func TestRegistrationTableWritesDoNotInvalidateUnrelatedDeclarations(t *testing.T) {
	found := registrationValueFindings(t, "registration-unrelated-written-values", registrationValueCorpus())
	wantOneCopy(t, found, "diagnostic codes", "unrelatedCopy")
}

// TestNamedMapDeclarationsKeepTheirRegisteredKeys follows named and aliased
// map types while retaining an unrelated copy of the same registry keys.
func TestNamedMapDeclarationsKeepTheirRegisteredKeys(t *testing.T) {
	for _, fixture := range []string{"registration-named-map-values", "registration-aliased-map-values"} {
		t.Run(fixture, func(t *testing.T) {
			corpus := registrationValueCorpus()
			corpus.Declarations = []Declaration{{Package: ".", Symbol: "builtinCodes"}}
			found := registrationValueFindings(t, fixture, corpus)
			wantOneCopy(t, found, "diagnostic codes", "unrelatedCopy")
		})
	}
}

// TestStructFieldKeysDoNotDeclarePackageConstants keeps field names distinct
// from package constants when a declaring table uses a named or aliased struct.
func TestStructFieldKeysDoNotDeclarePackageConstants(t *testing.T) {
	for _, fixture := range []string{"registration-named-struct-values", "registration-aliased-struct-values"} {
		t.Run(fixture, func(t *testing.T) {
			corpus := registrationValueCorpus()
			corpus.Declarations = []Declaration{{Package: ".", Symbol: "builtinCodes"}}
			found := registrationValueFindings(t, fixture, corpus)
			wantOneCopy(t, found, "diagnostic codes", "codeFirst")
		})
	}
}

// TestBlankRegistrationReadsKeepTheirDeclaration keeps direct and len reads
// distinct from writes when their result is deliberately discarded.
func TestBlankRegistrationReadsKeepTheirDeclaration(t *testing.T) {
	for _, fixture := range []string{"registration-blank-read-values", "registration-blank-len-values"} {
		t.Run(fixture, func(t *testing.T) {
			found := registrationValueFindings(t, fixture, registrationValueCorpus())
			wantOneCopy(t, found, "diagnostic codes", "unrelatedCopy")
		})
	}
}

// TestShadowedLocalWritesKeepPackageDeclarations mutates only a local table,
// then registers the same-named package table and retains an unrelated copy.
func TestShadowedLocalWritesKeepPackageDeclarations(t *testing.T) {
	found := registrationValueFindings(t, "registration-shadowed-written-values", registrationValueCorpus())
	wantOneCopy(t, found, "diagnostic codes", "unrelatedCopy")
}

// TestInitializerFunctionsInvalidateWrittenDeclarations catches a package
// initializer that replaces the table before a later init registers its entries.
func TestInitializerFunctionsInvalidateWrittenDeclarations(t *testing.T) {
	found := registrationValueFindings(t, "registration-initializer-written-values", registrationValueCorpus())
	wantOneCopy(t, found, "diagnostic codes", "codeFirst")
}

// TestUnregisteredShadowedLocalsDoNotDeclarePackageConstants keeps an unused
// local literal distinct from a same-named local passed to a registrar later.
func TestUnregisteredShadowedLocalsDoNotDeclarePackageConstants(t *testing.T) {
	found := registrationValueFindings(t, "registration-unregistered-shadow-values", registrationValueCorpus())
	wantOneCopy(t, found, "diagnostic codes", "codeFirst")
}

// TestOverwrittenLocalLiteralsDoNotDeclarePackageConstants prevents an old
// literal from bypassing the written binding that reaches the registrar.
func TestOverwrittenLocalLiteralsDoNotDeclarePackageConstants(t *testing.T) {
	found := registrationValueFindings(t, "registration-overwritten-local-values", registrationValueCorpus())
	wantOneCopy(t, found, "diagnostic codes", "codeFirst")
}

// TestOpaqueRegistrationInputsKeepCopiesVisible withholds declaration credit
// from unknown calls, selectors and a package function shadowing builtin append.
func TestOpaqueRegistrationInputsKeepCopiesVisible(t *testing.T) {
	for _, fixture := range []string{
		"registration-opaque-call-values",
		"registration-opaque-selector-values",
		"registration-shadowed-append-values",
	} {
		t.Run(fixture, func(t *testing.T) {
			found := registrationValueFindings(t, fixture, registrationValueCorpus())
			wantOneCopy(t, found, "diagnostic codes", "codeFirst")
		})
	}
}

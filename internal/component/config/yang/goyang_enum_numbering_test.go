package yang

import "testing"

// TestGoyangNumbersAnEnumAfterANegativeValue proves that the goyang Ze builds
// against numbers an enum with no value statement as RFC 7950 Section 9.6.4.2
// does, so goyang's own resolution accepts the module. Ze assigns its enum
// values itself (parseEnumAssignment), but Loader.Resolve also runs goyang's
// Modules.Process, and goyang v1.6.3 numbered q below as 0 after p's -5, then
// refused r with "fields r and q conflict on value 0". The RFC gives q -4, so
// r's explicit 0 is unique and the module is valid.
//
// VALIDATES: a module whose explicit 0 follows an implicit value after a
// negative one loads through AddModuleFromText and Resolve.
// PREVENTS: a goyang without the numbering fix refusing a valid module.
func TestGoyangNumbersAnEnumAfterANegativeValue(t *testing.T) {
	requireModuleLoaded(t, "explicit zero after an implicit value that follows -5",
		enumModule(`leaf x { type enumeration { enum p { value -5; } enum q; enum r { value 0; } } }`))
}

// Design: docs/architecture/isis/isis-13-cli-diag-interop.md -- IS-IS diagnostic code ownership tests.
//
// VALIDATES: the IS-IS component OWNS and registers its two config-sanity
// diagnostic codes (doctor-isis-net-missing, doctor-isis-system-id-mismatch) from
// its own register.go, so `ze explain <code>` describes them and so deleting the
// component removes them (they are NOT hardcoded in diagnostic.builtinCodes).
// PREVENTS: a regression that re-adds the IS-IS codes to the central builtinCodes
// slice (plugin-self-containment), or that drops their title/description.

package isis

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestISISConfigSanityCodesOwnedByComponent: the two config-sanity codes are
// registered (the isis package init() ran on import) and carry explanation
// metadata, so `ze explain` works without RegisterBuiltinCodes listing them.
func TestISISConfigSanityCodesOwnedByComponent(t *testing.T) {
	for _, code := range []string{codeNETMissing, codeSystemIDMismatch} {
		meta := diagnostic.Lookup(code)
		if meta == nil {
			t.Fatalf("%s not registered by the IS-IS component", code)
		}
		if meta.Title == "" || meta.Description == "" {
			t.Errorf("%s missing title/description", code)
		}
	}
}

// TestISISConfigSanityCodesNotInBuiltins: the two IS-IS config-sanity codes are
// NOT carried by the central diagnostic.builtinCodes slice; they belong to the
// component. RegisterBuiltinCodes must not be what makes them explainable.
func TestISISConfigSanityCodesNotInBuiltins(t *testing.T) {
	diagnostic.ResetForTest()
	t.Cleanup(func() {
		// Restore the global registry for other tests: the builtins plus the
		// component-owned IS-IS codes (which the init() registered once already).
		diagnostic.RegisterBuiltinCodes()
		registerISISDiagnosticCodes()
	})

	diagnostic.RegisterBuiltinCodes()
	builtins := diagnostic.AllCodes()
	for _, code := range []string{codeNETMissing, codeSystemIDMismatch} {
		if slices.Contains(builtins, code) {
			t.Errorf("%s is in the central builtinCodes slice; it must be owned by the IS-IS component (plugin-self-containment)", code)
		}
	}
	// The raw-socket code stays centrally listed (owned by isis-3 transport), so
	// this guard targets ONLY the two config-sanity codes, not all isis codes.
	if !slices.Contains(builtins, "doctor-isis-raw-socket") {
		t.Error("doctor-isis-raw-socket should remain a builtin code (transport owns its listing)")
	}
}

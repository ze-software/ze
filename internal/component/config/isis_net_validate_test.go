// VALIDATES: the ze:validate custom validator on a leaf-list (isis `net`) is
// applied per item during the ValidateTree / walkTree path, so an invalid NET
// in a leaf-list is rejected (the framework previously skipped custom
// validators for leaf-list items, and validateContainerEntry never applied
// them at all). Pins the isis-4 validator wiring end-to-end.
// PREVENTS: a regression where a bad NET in the `net` leaf-list validates
// silently because the custom validator is not invoked for leaf-list entries.
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
	// Register ze-isis-conf so ValidateTreeAllModules can find the isis section
	// (the live binary registers it via the generated all.go blank import).
	_ "github.com/ze-software/ze/internal/plugins/isis/yang"
)

func TestISISNetLeafListCustomValidatorApplied(t *testing.T) {
	loader := newTestLoader(t)
	reg := yang.NewValidatorRegistry()
	RegisterValidators(reg)
	reg.MergeGlobalCompletions()
	v := yang.NewValidator(loader)
	v.SetRegistry(reg)

	// Validate the net leaf-list through the walkTree path (the same path
	// ze config validate uses for the isis section).
	validateNet := func(net string) []yang.ValidationError {
		return v.ValidateTreeAllModules("isis", map[string]any{"net": net})
	}

	// A valid single NET passes.
	require.Empty(t, validateNet("49.0001.0000.0000.0001.00"),
		"a valid NET must pass the isis-net custom validator")

	// A NET that is too short (4 octets) is rejected by the custom validator
	// applied to the leaf-list item.
	errs := validateNet("49.0001.00")
	require.NotEmpty(t, errs, "an invalid NET must be rejected via the leaf-list custom validator")
	assert.Contains(t, errs[0].Message, "IS-IS NET")
}

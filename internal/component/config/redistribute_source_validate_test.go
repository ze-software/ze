// VALIDATES: the ze:validate "redistribute-source" custom validator on the
// `import` list KEY runs during ValidateTree/walkTree, so an unregistered
// redistribute source is rejected at `ze config validate` time. Previously the
// walker validated only a list entry's children, never the key itself, so any
// source name (even garbage) passed validation and failed only at runtime.
// PREVENTS: regression where `redistribute { destination bgp { import <typo> } }`
// validates clean and then silently no-ops at runtime.
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/component/config/yang"

	// Register ze-redistribute-conf so ValidateTreeAllModules finds the section
	// (the live binary registers it via the generated all.go blank import).
	_ "github.com/ze-software/ze/internal/component/config/redistribute/yang"
)

func TestRedistributeImportKeyValidated(t *testing.T) {
	// A registered source so the valid case passes; registration is idempotent.
	require.NoError(t, redistribute.RegisterSource(redistribute.RouteSource{
		Name:        "test-b1-source",
		Protocol:    "test-b1",
		Description: "test source for list-key validation",
	}))

	loader := newTestLoader(t)
	reg := yang.NewValidatorRegistry()
	RegisterValidators(reg)
	reg.MergeGlobalCompletions()
	v := yang.NewValidator(loader)
	v.SetRegistry(reg)

	// Validate a `redistribute { destination bgp { import <src> } }` tree through
	// the same walkTree path `ze config validate` uses.
	validateImport := func(src string) []yang.ValidationError {
		return v.ValidateTreeAllModules("redistribute", map[string]any{
			"destination": map[string]any{
				"bgp": map[string]any{
					"import": map[string]any{
						src: map[string]any{},
					},
				},
			},
		})
	}

	// A registered source passes.
	require.Empty(t, validateImport("test-b1-source"),
		"a registered redistribute source must pass validation")

	// An unregistered source is rejected via the list-key custom validator.
	errs := validateImport("totally-unregistered-source")
	require.NotEmpty(t, errs,
		"an unregistered redistribute source must be rejected at validate time")
	assert.Contains(t, errs[0].Message, "redistribute source")
}

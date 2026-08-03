// Design: docs/features/ai-first.md — diagnostic registry tests

package diagnostic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryKnownCodes(t *testing.T) {
	ResetForTest()
	RegisterBuiltinCodes()

	codes := AllCodes()
	assert.NotEmpty(t, codes, "expected registered codes")

	for _, code := range codes {
		meta := Lookup(code)
		require.NotNil(t, meta, "code %s should have metadata", code)
		assert.NotEmpty(t, meta.Title, "code %s should have a title", code)
		assert.NotEmpty(t, meta.Description, "code %s should have a description", code)
	}
}

func TestRegistryRejectsDuplicate(t *testing.T) {
	ResetForTest()
	err := Register(CodeMeta{Code: "test-code", Title: "Test", Description: "Test"})
	require.NoError(t, err)

	err = Register(CodeMeta{Code: "test-code", Title: "Dup", Description: "Dup"})
	assert.Error(t, err, "duplicate registration should fail")
}

func TestRegistryLookupUnknown(t *testing.T) {
	ResetForTest()
	meta := Lookup("nonexistent-code")
	assert.Nil(t, meta)
}

func TestRegistryAllCodesSorted(t *testing.T) {
	ResetForTest()
	RegisterBuiltinCodes()

	codes := AllCodes()
	for i := 1; i < len(codes); i++ {
		assert.True(t, codes[i-1] < codes[i], "codes should be sorted: %s < %s", codes[i-1], codes[i])
	}
}

func TestRegistryBuiltinCodesIncludeExpected(t *testing.T) {
	ResetForTest()
	RegisterBuiltinCodes()

	expected := []string{
		"config-parse",
		"config-yang-missing",
		"config-yang-type",
		"config-yang-enum",
		"config-yang-range",
		"config-plugin-verify",
		"config-mcp-invalid",
		"config-gnmi-invalid",
		"config-bgp-resolve",
		"config-bgp-peer",
		"config-listener-conflict",
		"config-warning",
	}

	for _, code := range expected {
		assert.NotNil(t, Lookup(code), "expected builtin code %s", code)
	}
}

func TestDiagnosticContractTypes(t *testing.T) {
	safetyLevels := []FixSafety{
		SafetyFormatOnly,
		SafetySectionLocal,
		SafetyBehaviorPreserving,
		SafetyAPIChanging,
		SafetyTargetChanging,
		SafetyRequiresHumanReview,
	}
	for _, s := range safetyLevels {
		assert.NotEmpty(t, string(s), "safety level should be non-empty")
	}

	repair := Repair{ID: "test-repair", Summary: "test"}
	assert.Equal(t, "test-repair", repair.ID)
	assert.Equal(t, "test", repair.Summary)

	related := Related{Message: "related location", Line: 1, Path: "bgp.peer"}
	assert.Equal(t, 1, related.Line)
	assert.Equal(t, "bgp.peer", related.Path)
	assert.Equal(t, "related location", related.Message)
}

func TestValidateResultEmptyDiagnostics(t *testing.T) {
	r := NewValidateResult("test.conf", true, nil, nil)
	assert.NotNil(t, r.Diagnostics, "nil diagnostics should be normalized to empty slice")
	assert.Empty(t, r.Diagnostics)

	f := NewFixPlan("test.conf", nil)
	assert.NotNil(t, f.Diagnostics, "nil diagnostics should be normalized to empty slice")
	assert.Empty(t, f.Diagnostics)
}

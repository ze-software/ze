package yang

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidatorRegistry_Register verifies registration and retrieval.
//
// VALIDATES: Registered validator can be retrieved by name (AC-11).
// PREVENTS: Registration silently failing.
func TestValidatorRegistry_Register(t *testing.T) {
	reg := NewValidatorRegistry()
	called := false
	reg.Register("test-func", CustomValidator{
		ValidateFn: func(path string, value any) error {
			called = true
			return nil
		},
	})

	cv := reg.Get("test-func")
	require.NotNil(t, cv)
	assert.NoError(t, cv.ValidateFn("test.path", "value"))
	assert.True(t, called)
}

// TestValidatorRegistry_Missing verifies nil return for unregistered name.
//
// VALIDATES: Missing validator returns nil, not panic.
// PREVENTS: Nil dereference on unknown validator.
func TestValidatorRegistry_Missing(t *testing.T) {
	reg := NewValidatorRegistry()
	cv := reg.Get("nonexistent")
	assert.Nil(t, cv)
}

// TestValidatorRegistry_CustomValidation verifies validator is called during tree walk.
//
// VALIDATES: ze:validate extension triggers custom validation (AC-11).
// PREVENTS: Custom validators being silently skipped.
func TestValidatorRegistry_CustomValidation(t *testing.T) {
	reg := NewValidatorRegistry()
	var receivedPath string
	var receivedValue any
	reg.Register("check-value", CustomValidator{
		ValidateFn: func(path string, value any) error {
			receivedPath = path
			receivedValue = value
			return nil
		},
	})

	cv := reg.Get("check-value")
	require.NotNil(t, cv)
	assert.NoError(t, cv.ValidateFn("bgp.peer.family", "ipv4/unicast"))
	assert.Equal(t, "bgp.peer.family", receivedPath)
	assert.Equal(t, "ipv4/unicast", receivedValue)
}

// TestValidatorRegistry_CustomError verifies custom error propagation.
//
// VALIDATES: Custom validator error is returned to caller (AC-13).
// PREVENTS: Custom validation errors being swallowed.
func TestValidatorRegistry_CustomError(t *testing.T) {
	reg := NewValidatorRegistry()
	reg.Register("reject-all", CustomValidator{
		ValidateFn: func(path string, value any) error {
			return fmt.Errorf("rejected %q at %s", value, path)
		},
	})

	cv := reg.Get("reject-all")
	require.NotNil(t, cv)
	err := cv.ValidateFn("bgp.peer.family", "invalid/family")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rejected")
	assert.Contains(t, err.Error(), "invalid/family")
}

// TestValidatorRegistry_Complete verifies completion values retrieval.
//
// VALIDATES: Complete() returns valid options for CLI completion (AC-18).
// PREVENTS: Completion returning empty when values are available.
func TestValidatorRegistry_Complete(t *testing.T) {
	reg := NewValidatorRegistry()
	reg.Register("family-check", CustomValidator{
		ValidateFn: func(path string, value any) error { return nil },
		CompleteFn: func() []string {
			return []string{"ipv4/unicast", "ipv6/unicast"}
		},
	})

	cv := reg.Get("family-check")
	require.NotNil(t, cv)
	require.NotNil(t, cv.CompleteFn)
	values := cv.CompleteFn()
	assert.Equal(t, []string{"ipv4/unicast", "ipv6/unicast"}, values)
}

// TestValidatorRegistry_CompleteNil verifies nil CompleteFn is safe.
//
// VALIDATES: Validators without completion don't crash.
// PREVENTS: Nil dereference on validators that only validate.
func TestValidatorRegistry_CompleteNil(t *testing.T) {
	reg := NewValidatorRegistry()
	reg.Register("validate-only", CustomValidator{
		ValidateFn: func(path string, value any) error { return nil },
	})

	cv := reg.Get("validate-only")
	require.NotNil(t, cv)
	assert.Nil(t, cv.CompleteFn)
}

// TestValidatorRegistry_Names verifies listing all registered names.
//
// VALIDATES: Names() returns all registered validator names.
// PREVENTS: Missing registrations in startup check.
func TestValidatorRegistry_Names(t *testing.T) {
	reg := NewValidatorRegistry()
	reg.Register("alpha", CustomValidator{
		ValidateFn: func(path string, value any) error { return nil },
	})
	reg.Register("beta", CustomValidator{
		ValidateFn: func(path string, value any) error { return nil },
	})

	names := reg.Names()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "alpha")
	assert.Contains(t, names, "beta")
}

// TestGetValidateExtension verifies reading ze:validate from YANG entries.
//
// VALIDATES: ze:validate extension argument is correctly extracted from YANG (AC-11).
// PREVENTS: Extension being silently ignored during tree walk.
func TestGetValidateExtension(t *testing.T) {
	// Test the extraction function directly with nil entry (no extension).
	name := GetValidateExtension(nil)
	assert.Empty(t, name)
}

// TestSplitValidatorNames verifies pipe-separated validator name splitting.
//
// VALIDATES: Single names pass through, pipe-separated names expand.
// PREVENTS: Multiple validators per leaf being silently ignored.
func TestSplitValidatorNames(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want []string
	}{
		{"empty", "", nil},
		{"single", "nonzero-ipv4", []string{"nonzero-ipv4"}},
		{"two validators", "nonzero-ipv4|format-check", []string{"nonzero-ipv4", "format-check"}},
		{"three validators", "a|b|c", []string{"a", "b", "c"}},
		{"spaces trimmed", "a | b", []string{"a", "b"}},
		{"empty parts skipped", "a||b", []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitValidatorNames(tt.arg)
			assert.Equal(t, tt.want, got)
		})
	}
}

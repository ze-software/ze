package yang

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidationError verifies ValidationError type.
//
// VALIDATES: ValidationError contains expected fields.
// PREVENTS: Missing context in error reporting.
func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Path:       "bgp/local-as",
		Type:       ErrTypeRange,
		Message:    "value 0 is outside range 1..4294967295",
		Expected:   "1..4294967295",
		Got:        "0",
		LineNumber: 42,
	}

	assert.Equal(t, "bgp/local-as", err.Path)
	assert.Equal(t, ErrTypeRange, err.Type)
	assert.Contains(t, err.Error(), "bgp/local-as")
	assert.Contains(t, err.Error(), "range")
	assert.Contains(t, err.Error(), "42")
}

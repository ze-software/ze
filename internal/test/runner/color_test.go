package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewColorsRespectsNoColor verifies NewColors honors the NO_COLOR env var.
//
// VALIDATES: AC-10: test runner uses shared detection that respects NO_COLOR.
// PREVENTS: Test runner ignoring NO_COLOR and producing ANSI codes in piped output.
func TestNewColorsRespectsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	c := NewColors()
	assert.False(t, c.Enabled())
}

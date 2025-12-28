package nlri

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPackContextASN4 verifies ASN4 field exists and defaults correctly.
//
// VALIDATES: PackContext carries ASN4 capability for attribute encoding.
// RFC 6793 Section 4.1: NEW speakers use 4-byte AS numbers when ASN4=true.
//
// PREVENTS: Missing ASN4 field causing compilation errors or incorrect
// AS_PATH encoding when communicating with OLD vs NEW BGP speakers.
func TestPackContextASN4(t *testing.T) {
	// Default context - ASN4 should be false (zero value)
	ctx := &PackContext{}
	assert.False(t, ctx.ASN4, "default ASN4 should be false")

	// Explicit ASN4=true
	ctx = &PackContext{ASN4: true, AddPath: false}
	assert.True(t, ctx.ASN4, "ASN4 should be true when set")
	assert.False(t, ctx.AddPath, "AddPath should remain false")

	// Both capabilities enabled
	ctx = &PackContext{ASN4: true, AddPath: true}
	assert.True(t, ctx.ASN4, "ASN4 should be true")
	assert.True(t, ctx.AddPath, "AddPath should be true")
}

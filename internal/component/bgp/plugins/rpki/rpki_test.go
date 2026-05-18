package rpki

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestASPAPolicyReject(t *testing.T) {
	assert.True(t, aspaOverridesAccept(ASPAInvalid, ASPAPolicyReject, ASPAPolicyAccept),
		"ASPA Invalid + reject policy should override accept")
}

func TestASPAPolicyLogOnly(t *testing.T) {
	assert.False(t, aspaOverridesAccept(ASPAInvalid, ASPAPolicyLogOnly, ASPAPolicyAccept),
		"ASPA Invalid + log-only policy should not override accept")
}

func TestASPAPolicyAcceptAction(t *testing.T) {
	assert.False(t, aspaOverridesAccept(ASPAInvalid, ASPAPolicyAccept, ASPAPolicyAccept),
		"ASPA Invalid + accept policy should not override accept")
}

func TestASPAPolicyOverridesOrigin(t *testing.T) {
	// ROA Valid (origin says accept) but ASPA Invalid + reject -> should override
	assert.True(t, aspaOverridesAccept(ASPAInvalid, ASPAPolicyReject, ASPAPolicyAccept),
		"ASPA reject should override even when origin validation accepts")
}

func TestASPAPolicyUnknownReject(t *testing.T) {
	assert.True(t, aspaOverridesAccept(ASPAUnknown, ASPAPolicyLogOnly, ASPAPolicyReject),
		"ASPA Unknown + unknown-action=reject should override accept")
}

func TestASPAPolicyUnknownAccept(t *testing.T) {
	assert.False(t, aspaOverridesAccept(ASPAUnknown, ASPAPolicyReject, ASPAPolicyAccept),
		"ASPA Unknown + unknown-action=accept should not override")
}

func TestASPAPolicyValidNeverOverrides(t *testing.T) {
	assert.False(t, aspaOverridesAccept(ASPAValid, ASPAPolicyReject, ASPAPolicyReject),
		"ASPA Valid should never trigger override regardless of policy")
}

func TestASPAPolicyNoneNeverOverrides(t *testing.T) {
	assert.False(t, aspaOverridesAccept(aspaStateNone, ASPAPolicyReject, ASPAPolicyReject),
		"ASPA state none (disabled) should never trigger override")
}

func TestASPAPolicyRevalidationReject(t *testing.T) {
	// When ASPA cache changes and state becomes Invalid with reject policy
	assert.True(t, aspaOverridesAccept(ASPAInvalid, ASPAPolicyReject, ASPAPolicyAccept),
		"re-validation: Invalid + reject should dispatch reject")
	// When state becomes Valid, no override even with reject policy
	assert.False(t, aspaOverridesAccept(ASPAValid, ASPAPolicyReject, ASPAPolicyAccept),
		"re-validation: Valid should not dispatch reject")
}

func TestASPAPolicyRevalidationLogOnly(t *testing.T) {
	// When ASPA cache changes and state becomes Invalid with log-only policy
	assert.False(t, aspaOverridesAccept(ASPAInvalid, ASPAPolicyLogOnly, ASPAPolicyAccept),
		"re-validation: Invalid + log-only should not dispatch reject")
}

func TestAspaActionFromString(t *testing.T) {
	tests := []struct {
		input string
		want  uint8
		valid bool
	}{
		{"reject", ASPAPolicyReject, true},
		{"log-only", ASPAPolicyLogOnly, true},
		{"accept", ASPAPolicyAccept, true},
		{"unknown", 0, false},
		{"", 0, false},
	}
	for _, tt := range tests {
		action, valid := aspaActionFromString(tt.input)
		assert.Equal(t, tt.valid, valid, "valid for %q", tt.input)
		if valid {
			assert.Equal(t, tt.want, action, "action for %q", tt.input)
		}
	}
}

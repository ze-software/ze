package bgp_role

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRoleValueBoundary verifies role value encoding boundaries.
//
// VALIDATES: Role values 0-4 are valid, values outside range are rejected.
// PREVENTS: Off-by-one in role value validation.
// BOUNDARY: 4 (last valid), 5 (first invalid above).
func TestRoleValueBoundary(t *testing.T) {
	tests := []struct {
		name   string
		value  uint8
		want   string
		wantOK bool
	}{
		{"provider_0", 0, "provider", true},
		{"rs_1", 1, "rs", true},
		{"rs_client_2", 2, "rs-client", true},
		{"customer_3", 3, "customer", true},
		{"peer_4", 4, "peer", true},
		{"invalid_5", 5, "", false},
		{"invalid_255", 255, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := roleValueToName(tt.value)
			assert.Equal(t, tt.wantOK, ok, "valid flag mismatch")
			if tt.wantOK {
				assert.Equal(t, tt.want, got, "role name mismatch")
			}
		})
	}
}

// TestRoleNameToValue verifies role name to wire value mapping.
//
// VALIDATES: All role name strings map to correct wire values.
// PREVENTS: Wrong capability value being sent in OPEN.
func TestRoleNameToValue(t *testing.T) {
	tests := []struct {
		name   string
		want   uint8
		wantOK bool
	}{
		{"provider", 0, true},
		{"rs", 1, true},
		{"rs-client", 2, true},
		{"customer", 3, true},
		{"peer", 4, true},
		{"invalid", 0, false},
		{"", 0, false},
		{"Provider", 0, false}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := roleNameToValue(tt.name)
			assert.Equal(t, tt.wantOK, ok, "valid flag mismatch for %q", tt.name)
			if tt.wantOK {
				assert.Equal(t, tt.want, got, "value mismatch for %q", tt.name)
			}
		})
	}
}

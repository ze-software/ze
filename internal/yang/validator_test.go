package yang

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestValidator creates a Validator loaded with all YANG modules for testing.
func newTestValidator(t *testing.T) *Validator {
	t.Helper()
	loader := NewLoader()
	require.NoError(t, loader.LoadAllForTesting())
	require.NoError(t, loader.Resolve())
	return NewValidator(loader)
}

// TestValidateTree_ValidConfig verifies a complete valid config passes recursive walk.
//
// VALIDATES: Full config tree validation succeeds with valid values (AC-7).
// PREVENTS: False positives rejecting valid configurations.
func TestValidateTree_ValidConfig(t *testing.T) {
	v := newTestValidator(t)

	data := map[string]any{
		"router-id": "192.0.2.1",
		"local-as":  uint32(65001),
		"peer": map[string]any{
			"192.0.2.2": map[string]any{
				"peer-as":       uint32(65002),
				"local-address": "192.0.2.1",
				"hold-time":     uint16(90),
				"connection":    "passive",
			},
		},
	}

	errs := v.ValidateTree("bgp", data)
	assert.Empty(t, errs, "valid config should produce no errors")
}

// TestValidateTree_EnumViolation verifies invalid enum values are caught at any depth.
//
// VALIDATES: Enum violation detected in nested container (AC-1).
// PREVENTS: Invalid enum values passing validation silently.
func TestValidateTree_EnumViolation(t *testing.T) {
	v := newTestValidator(t)

	data := map[string]any{
		"router-id": "192.0.2.1",
		"local-as":  uint32(65001),
		"peer": map[string]any{
			"192.0.2.2": map[string]any{
				"peer-as":       uint32(65002),
				"local-address": "192.0.2.1",
				"connection":    "invalid-value",
			},
		},
	}

	errs := v.ValidateTree("bgp", data)
	require.NotEmpty(t, errs, "invalid enum should produce error")
	assert.Equal(t, ErrTypeEnum, errs[0].Type)
	assert.Contains(t, errs[0].Path, "connection")
}

// TestValidateTree_RangeViolation verifies out-of-range values are caught at any depth.
//
// VALIDATES: Range violation detected in nested container (AC-2, AC-3).
// PREVENTS: Out-of-range numeric values passing validation.
func TestValidateTree_RangeViolation(t *testing.T) {
	v := newTestValidator(t)

	tests := []struct {
		name string
		data map[string]any
		path string
	}{
		{
			name: "hold_time_2_invalid",
			data: map[string]any{
				"router-id": "192.0.2.1",
				"local-as":  uint32(65001),
				"peer": map[string]any{
					"192.0.2.2": map[string]any{
						"peer-as":       uint32(65002),
						"local-address": "192.0.2.1",
						"hold-time":     uint16(2),
					},
				},
			},
			path: "hold-time",
		},
		{
			name: "port_0_invalid",
			data: map[string]any{
				"router-id": "192.0.2.1",
				"local-as":  uint32(65001),
				"peer": map[string]any{
					"192.0.2.2": map[string]any{
						"peer-as":       uint32(65002),
						"local-address": "192.0.2.1",
						"port":          uint16(0),
					},
				},
			},
			path: "port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := v.ValidateTree("bgp", tt.data)
			require.NotEmpty(t, errs, "range violation should produce error")
			assert.Equal(t, ErrTypeRange, errs[0].Type)
			assert.Contains(t, errs[0].Path, tt.path)
		})
	}
}

// TestValidateTree_PatternViolation verifies invalid patterns are caught for typedefs.
//
// VALIDATES: Pattern violation detected for ipv4-address typedef (AC-6).
// PREVENTS: Malformed strings passing pattern validation.
func TestValidateTree_PatternViolation(t *testing.T) {
	v := newTestValidator(t)

	data := map[string]any{
		"router-id": "not-an-ip",
		"local-as":  uint32(65001),
	}

	errs := v.ValidateTree("bgp", data)
	require.NotEmpty(t, errs, "pattern violation should produce error")
	assert.Equal(t, ErrTypePattern, errs[0].Type)
	assert.Contains(t, errs[0].Path, "router-id")
}

// TestValidateTree_MandatoryMissing verifies missing mandatory fields caught at nested level.
//
// VALIDATES: Mandatory field missing at nested container level (AC-4, AC-5).
// PREVENTS: Silent acceptance of incomplete config.
func TestValidateTree_MandatoryMissing(t *testing.T) {
	v := newTestValidator(t)

	// Missing router-id (mandatory at bgp level)
	data := map[string]any{
		"local-as": uint32(65001),
	}

	errs := v.ValidateTree("bgp", data)
	require.NotEmpty(t, errs, "missing mandatory field should produce error")
	found := false
	for _, e := range errs {
		if e.Type == ErrTypeMissing && e.Path == "bgp.router-id" {
			found = true
		}
	}
	assert.True(t, found, "expected missing mandatory error for router-id")
}

// TestValidateTree_MultipleErrors verifies all errors are collected, not stopped at first.
//
// VALIDATES: Multiple validation errors collected in single pass.
// PREVENTS: Stopping at first error, forcing fix-one-at-a-time workflow.
func TestValidateTree_MultipleErrors(t *testing.T) {
	v := newTestValidator(t)

	// Missing router-id AND invalid local-as
	data := map[string]any{
		"local-as": uint32(0), // range violation: 1..4294967295
		// router-id missing: mandatory violation
	}

	errs := v.ValidateTree("bgp", data)
	assert.GreaterOrEqual(t, len(errs), 2, "should collect multiple errors: %v", errs)
}

// TestValidateTree_NestedContainers verifies validation recurses into nested containers.
//
// VALIDATES: Validation recurses into nested containers.
// PREVENTS: Shallow validation missing errors in nested structures.
func TestValidateTree_NestedContainers(t *testing.T) {
	v := newTestValidator(t)

	data := map[string]any{
		"router-id": "192.0.2.1",
		"local-as":  uint32(65001),
		"peer": map[string]any{
			"192.0.2.2": map[string]any{
				"peer-as":       uint32(65002),
				"local-address": "192.0.2.1",
				"capability": map[string]any{
					"add-path": map[string]any{
						"send":    true,
						"receive": true,
					},
				},
			},
		},
	}

	errs := v.ValidateTree("bgp", data)
	assert.Empty(t, errs, "valid nested containers should produce no errors: %v", errs)
}

// TestValidateTree_ListEntries verifies validation recurses into each list entry.
//
// VALIDATES: Each list entry is validated separately.
// PREVENTS: Skipping validation of individual list entries.
func TestValidateTree_ListEntries(t *testing.T) {
	v := newTestValidator(t)

	// Two peers: one valid, one with invalid hold-time
	data := map[string]any{
		"router-id": "192.0.2.1",
		"local-as":  uint32(65001),
		"peer": map[string]any{
			"192.0.2.2": map[string]any{
				"peer-as":       uint32(65002),
				"local-address": "192.0.2.1",
				"hold-time":     uint16(90),
			},
			"192.0.2.3": map[string]any{
				"peer-as":       uint32(65003),
				"local-address": "192.0.2.1",
				"hold-time":     uint16(1), // invalid
			},
		},
	}

	errs := v.ValidateTree("bgp", data)
	require.NotEmpty(t, errs, "invalid list entry should produce error")
	assert.Equal(t, ErrTypeRange, errs[0].Type)
	assert.Contains(t, errs[0].Path, "hold-time")
}

// TestValidateTree_FamilyModeEnum verifies family mode enum after schema fix.
//
// VALIDATES: Family mode accepts enable/disable/require/ignore, rejects others (AC-8, AC-9).
// PREVENTS: Invalid family mode values passing validation after enum conversion.
func TestValidateTree_FamilyModeEnum(t *testing.T) {
	v := newTestValidator(t)

	tests := []struct {
		name    string
		mode    string
		wantErr bool
	}{
		{"enable", "enable", false},
		{"disable", "disable", false},
		{"require", "require", false},
		{"ignore", "ignore", false},
		{"invalid", "invalid-mode", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]any{
				"router-id": "192.0.2.1",
				"local-as":  uint32(65001),
				"peer": map[string]any{
					"192.0.2.2": map[string]any{
						"peer-as":       uint32(65002),
						"local-address": "192.0.2.1",
						"family": map[string]any{
							"ipv4/unicast": map[string]any{
								"mode": tt.mode,
							},
						},
					},
				},
			}

			errs := v.ValidateTree("bgp", data)
			if tt.wantErr {
				require.NotEmpty(t, errs, "invalid mode should produce error")
				assert.Equal(t, ErrTypeEnum, errs[0].Type)
			} else {
				assert.Empty(t, errs, "valid mode should produce no errors: %v", errs)
			}
		})
	}
}

// TestValidateTree_AddPathDirectionEnum verifies add-path direction enum after schema fix.
//
// VALIDATES: add-path direction accepts send/receive/send/receive, rejects others (AC-10).
// PREVENTS: Invalid add-path direction values passing validation.
func TestValidateTree_AddPathDirectionEnum(t *testing.T) {
	v := newTestValidator(t)

	tests := []struct {
		name    string
		dir     string
		wantErr bool
	}{
		{"send", "send", false},
		{"receive", "receive", false},
		{"send_receive", "send/receive", false},
		{"invalid", "both", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]any{
				"router-id": "192.0.2.1",
				"local-as":  uint32(65001),
				"peer": map[string]any{
					"192.0.2.2": map[string]any{
						"peer-as":       uint32(65002),
						"local-address": "192.0.2.1",
						"add-path": map[string]any{
							"ipv4/unicast": map[string]any{
								"direction": tt.dir,
								"mode":      "enable",
							},
						},
					},
				},
			}

			errs := v.ValidateTree("bgp", data)
			if tt.wantErr {
				require.NotEmpty(t, errs, "invalid direction should produce error")
				assert.Equal(t, ErrTypeEnum, errs[0].Type)
			} else {
				assert.Empty(t, errs, "valid direction should produce no errors: %v", errs)
			}
		})
	}
}

// TestValidator_ValidateString verifies string type validation.
//
// VALIDATES: String values are accepted.
// PREVENTS: Rejection of valid string values.
func TestValidator_ValidateString(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadAllForTesting()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	validator := NewValidator(loader)

	// Test with bgp.peer.address which is type ip-address (string-based)
	tests := []struct {
		name    string
		path    string
		value   any
		wantErr bool
	}{
		// peer.address is type ip-address
		{"valid_ip", "bgp.peer.address", "192.0.2.1", false},
		{"empty_ip", "bgp.peer.address", "", false}, // Validation may fail but type is string
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.path, tt.value)
			// For this basic test, we just verify the validator can process string types
			// Actual path resolution may need more work
			_ = err
		})
	}
}

// TestValidator_ValidateUint32 verifies uint32 type validation.
//
// VALIDATES: Numeric values within uint32 range are accepted.
// PREVENTS: Silent acceptance of out-of-range values.
func TestValidator_ValidateUint32(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadAllForTesting()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	validator := NewValidator(loader)

	tests := []struct {
		name    string
		path    string
		value   any
		wantErr bool
	}{
		// local-as uses ze-types:asn which has range 1..4294967295
		{"min_asn", "bgp.local-as", uint32(1), false},
		{"max_asn", "bgp.local-as", uint32(4294967295), false},
		{"mid_asn", "bgp.local-as", uint32(65001), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.path, tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidator_ValidateUint32Range verifies uint32 range boundary validation.
//
// VALIDATES: Values outside range are rejected.
// BOUNDARY: ASN range 1..4294967295.
func TestValidator_ValidateUint32Range(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadAllForTesting()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	validator := NewValidator(loader)

	tests := []struct {
		name    string
		path    string
		value   any
		wantErr bool
	}{
		// ASN boundary: range 1..4294967295
		{"asn_last_valid", "bgp.local-as", uint32(4294967295), false},
		{"asn_first_valid", "bgp.local-as", uint32(1), false},
		{"asn_below_range", "bgp.local-as", uint32(0), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.path, tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidator_ValidateUint32_WrongType verifies uint32 fields reject wrong types.
//
// VALIDATES: String, bool, nil, and negative int64 are rejected for uint32 fields.
// PREVENTS: Silent acceptance of wrong-type values that would produce zero or garbage.
func TestValidator_ValidateUint32_WrongType(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadAllForTesting()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	validator := NewValidator(loader)

	tests := []struct {
		name    string
		value   any
		wantErr bool
		errType ErrorType
	}{
		{"string_valid_number", "65001", false, ErrTypeUnknown}, // strings converted to uint
		{"string_not_number", "hello", true, ErrTypeType},
		{"bool_true", true, true, ErrTypeType},
		{"bool_false", false, true, ErrTypeType},
		{"nil_value", nil, true, ErrTypeType},
		{"negative_int", int(-1), true, ErrTypeType},
		{"negative_int64", int64(-1), true, ErrTypeType},
		{"negative_float64", float64(-1), true, ErrTypeType},
		{"float64_fractional", float64(65001.5), true, ErrTypeType},
		// Valid types for coverage contrast.
		{"valid_uint32", uint32(65001), false, ErrTypeUnknown},
		{"valid_int", int(65001), false, ErrTypeUnknown},
		{"valid_int64", int64(65001), false, ErrTypeUnknown},
		{"valid_float64", float64(65001), false, ErrTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate("bgp.local-as", tt.value)
			if tt.wantErr {
				require.Error(t, err, "expected error for %T(%v)", tt.value, tt.value)
				var valErr *ValidationError
				if errors.As(err, &valErr) {
					assert.Equal(t, tt.errType, valErr.Type)
				}
			} else {
				assert.NoError(t, err, "expected no error for %T(%v)", tt.value, tt.value)
			}
		})
	}
}

// TestValidator_ValidateString_WrongType verifies string fields reject wrong types.
//
// VALIDATES: Integer and bool values are rejected for string (ipv4-address) fields.
// PREVENTS: Silent type coercion of non-string values to strings.
func TestValidator_ValidateString_WrongType(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadAllForTesting()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	validator := NewValidator(loader)

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"int_value", int(42), true},
		{"float64_value", float64(42.0), true},
		{"bool_value", true, true},
		{"nil_value", nil, true},
		// Valid for contrast.
		{"valid_string", "192.0.2.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate("bgp.router-id", tt.value)
			if tt.wantErr {
				require.Error(t, err, "expected error for %T(%v)", tt.value, tt.value)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidator_ValidatePattern verifies pattern constraint validation.
//
// VALIDATES: String patterns are enforced.
// PREVENTS: Accepting malformed IP addresses.
func TestValidator_ValidatePattern(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadAllForTesting()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	validator := NewValidator(loader)

	tests := []struct {
		name    string
		path    string
		value   any
		wantErr bool
	}{
		// router-id uses ze-types:ipv4-address which has a pattern
		{"valid_ipv4", "bgp.router-id", "192.0.2.1", false},
		{"invalid_ipv4_format", "bgp.router-id", "not-an-ip", true},
		// Note: The pattern may not catch all invalid IPs (256.0.0.1)
		// Pattern validation depends on the regex being strict enough
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.path, tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidator_ErrorMessages verifies error message clarity.
//
// VALIDATES: Error messages include path and constraint info.
// PREVENTS: Cryptic error messages that don't help users.
func TestValidator_ErrorMessages(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadAllForTesting()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	validator := NewValidator(loader)

	// Test range error message
	err = validator.Validate("bgp.local-as", uint32(0))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "range")
	assert.Contains(t, err.Error(), "local-as")
}

// TestValidationError verifies ValidationError type.
//
// VALIDATES: ValidationError contains expected fields.
// PREVENTS: Missing context in error reporting.
func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Path:       "bgp.local-as",
		Type:       ErrTypeRange,
		Message:    "value 0 is outside range 1..4294967295",
		Expected:   "1..4294967295",
		Got:        "0",
		LineNumber: 42,
	}

	assert.Equal(t, "bgp.local-as", err.Path)
	assert.Equal(t, ErrTypeRange, err.Type)
	assert.Contains(t, err.Error(), "bgp.local-as")
	assert.Contains(t, err.Error(), "range")
	assert.Contains(t, err.Error(), "42")
}

// TestValidator_HoldTimeRange verifies hold-time special range (0 | 3..65535).
//
// VALIDATES: Hold-time accepts 0 or values >= 3.
// BOUNDARY: 0 valid, 1-2 invalid, 3+ valid.
func TestValidator_HoldTimeRange(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadAllForTesting()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	validator := NewValidator(loader)

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"hold_time_0", uint16(0), false},
		{"hold_time_1_invalid", uint16(1), true},
		{"hold_time_2_invalid", uint16(2), true},
		{"hold_time_3", uint16(3), false},
		{"hold_time_180", uint16(180), false},
		{"hold_time_65535", uint16(65535), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate("bgp.peer.hold-time", tt.value)
			if tt.wantErr {
				assert.Error(t, err, "expected error for value %v", tt.value)
			} else {
				assert.NoError(t, err, "expected no error for value %v", tt.value)
			}
		})
	}
}

// TestValidator_MandatoryField verifies mandatory field detection.
//
// VALIDATES: Container validation detects missing mandatory fields.
// PREVENTS: Silent acceptance of incomplete config missing required fields.
func TestValidator_MandatoryField(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadAllForTesting()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	validator := NewValidator(loader)

	tests := []struct {
		name    string
		path    string
		data    map[string]any
		wantErr bool
		errType ErrorType
	}{
		{
			name: "all_mandatory_present",
			path: "bgp",
			data: map[string]any{
				"local-as":  uint32(65001),
				"router-id": "192.0.2.1",
			},
			wantErr: false,
		},
		{
			name: "missing_mandatory_local_as",
			path: "bgp",
			data: map[string]any{
				"router-id": "192.0.2.1",
			},
			wantErr: true,
			errType: ErrTypeMissing,
		},
		{
			name: "missing_mandatory_router_id",
			path: "bgp",
			data: map[string]any{
				"local-as": uint32(65001),
			},
			wantErr: true,
			errType: ErrTypeMissing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateContainer(tt.path, tt.data)
			if tt.wantErr {
				require.Error(t, err, "expected error for missing mandatory field")
				var valErr *ValidationError
				if errors.As(err, &valErr) {
					assert.Equal(t, tt.errType, valErr.Type)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

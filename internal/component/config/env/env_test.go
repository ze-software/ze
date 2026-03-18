package env

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coreenv "codeberg.org/thomas-mangin/ze/internal/core/env"
)

// TestGet verifies environment variable lookup with dot/underscore equivalence.
//
// VALIDATES: Get returns value from ze.bgp.section.key or ze_bgp_section_key.
// PREVENTS: Missing cache resets causing stale values across tests.
func TestGet(t *testing.T) {
	tests := []struct {
		name     string
		section  string
		key      string
		dotEnv   string // ze.bgp.section.key value
		underEnv string // ze_bgp_section_key value
		want     string
	}{
		{
			name:     "dot_notation_only",
			section:  "ci",
			key:      "max_files",
			dotEnv:   "50",
			underEnv: "",
			want:     "50",
		},
		{
			name:     "underscore_notation_only",
			section:  "ci",
			key:      "max_files",
			dotEnv:   "",
			underEnv: "75",
			want:     "75",
		},
		{
			name:     "dot_and_underscore_equivalent",
			section:  "ci",
			key:      "max_files",
			dotEnv:   "100",
			underEnv: "",
			want:     "100",
		},
		{
			name:     "neither_set",
			section:  "ci",
			key:      "unset_key",
			dotEnv:   "",
			underEnv: "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coreenv.ResetCache()
			t.Cleanup(coreenv.ResetCache)

			// Clean up any existing env vars
			dotKey := "ze.bgp." + tt.section + "." + tt.key
			underKey := "ze_bgp_" + tt.section + "_" + tt.key
			_ = os.Unsetenv(dotKey)
			_ = os.Unsetenv(underKey)
			defer func() { _ = os.Unsetenv(dotKey) }()
			defer func() { _ = os.Unsetenv(underKey) }()

			// Set env vars for this test
			if tt.dotEnv != "" {
				require.NoError(t, os.Setenv(dotKey, tt.dotEnv))
			}
			if tt.underEnv != "" {
				require.NoError(t, os.Setenv(underKey, tt.underEnv))
			}
			coreenv.ResetCache()

			got := Get(tt.section, tt.key)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestGetInt verifies integer parsing with default fallback.
//
// VALIDATES: GetInt returns parsed int or default for invalid/empty.
// PREVENTS: Panic on invalid input, wrong default handling.
func TestGetInt(t *testing.T) {
	tests := []struct {
		name       string
		section    string
		key        string
		envValue   string
		defaultVal int
		want       int
	}{
		{
			name:       "valid_int",
			section:    "ci",
			key:        "max_files",
			envValue:   "100",
			defaultVal: 50,
			want:       100,
		},
		{
			name:       "invalid_int_returns_default",
			section:    "ci",
			key:        "max_files",
			envValue:   "not_a_number",
			defaultVal: 50,
			want:       50,
		},
		{
			name:       "empty_returns_default",
			section:    "ci",
			key:        "max_files",
			envValue:   "",
			defaultVal: 50,
			want:       50,
		},
		{
			name:       "negative_int",
			section:    "ci",
			key:        "max_files",
			envValue:   "-10",
			defaultVal: 50,
			want:       -10,
		},
		{
			name:       "zero",
			section:    "ci",
			key:        "max_files",
			envValue:   "0",
			defaultVal: 50,
			want:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coreenv.ResetCache()
			t.Cleanup(coreenv.ResetCache)

			dotKey := "ze.bgp." + tt.section + "." + tt.key
			_ = os.Unsetenv(dotKey)
			defer func() { _ = os.Unsetenv(dotKey) }()

			if tt.envValue != "" {
				require.NoError(t, os.Setenv(dotKey, tt.envValue))
			}
			coreenv.ResetCache()

			got := GetInt(tt.section, tt.key, tt.defaultVal)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestGetInt64 verifies int64 parsing with default fallback.
//
// VALIDATES: GetInt64 handles large values correctly.
// PREVENTS: Overflow on large values, wrong type conversion.
func TestGetInt64(t *testing.T) {
	tests := []struct {
		name       string
		section    string
		key        string
		envValue   string
		defaultVal int64
		want       int64
	}{
		{
			name:       "valid_int64",
			section:    "ci",
			key:        "max_total_size",
			envValue:   "1048576",
			defaultVal: 0,
			want:       1048576,
		},
		{
			name:       "large_value",
			section:    "ci",
			key:        "max_total_size",
			envValue:   "9223372036854775807", // Max int64
			defaultVal: 0,
			want:       9223372036854775807,
		},
		{
			name:       "invalid_returns_default",
			section:    "ci",
			key:        "max_total_size",
			envValue:   "invalid",
			defaultVal: 1048576,
			want:       1048576,
		},
		{
			name:       "empty_returns_default",
			section:    "ci",
			key:        "max_total_size",
			envValue:   "",
			defaultVal: 1048576,
			want:       1048576,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coreenv.ResetCache()
			t.Cleanup(coreenv.ResetCache)

			dotKey := "ze.bgp." + tt.section + "." + tt.key
			_ = os.Unsetenv(dotKey)
			defer func() { _ = os.Unsetenv(dotKey) }()

			if tt.envValue != "" {
				require.NoError(t, os.Setenv(dotKey, tt.envValue))
			}
			coreenv.ResetCache()

			got := GetInt64(tt.section, tt.key, tt.defaultVal)
			assert.Equal(t, tt.want, got)
		})
	}
}

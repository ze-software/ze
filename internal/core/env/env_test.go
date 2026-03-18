package env

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// Register test keys so Get() validation passes.
	MustRegister(EnvEntry{Key: "ze.test.env.check", Type: "string", Description: "test key"})
	MustRegister(EnvEntry{Key: "ze.test.int.val", Type: "int", Description: "test key"})
	MustRegister(EnvEntry{Key: "ze.test.bool.val", Type: "bool", Description: "test key"})
	MustRegister(EnvEntry{Key: "ze.test.enabled.val", Type: "bool", Description: "test key"})
	MustRegister(EnvEntry{Key: "ze.test.dur.val", Type: "duration", Description: "test key"})
	MustRegister(EnvEntry{Key: "ze.test.i64.val", Type: "int64", Description: "test key"})
	MustRegister(EnvEntry{Key: "ze.test.set.val", Type: "string", Description: "test key"})
	os.Exit(m.Run())
}

// unsetAll clears all notation forms for a dot-notation key and resets the cache.
// Also clears any mixed-case variants by scanning os.Environ().
func unsetAll(t *testing.T, dotKey string) {
	t.Helper()
	norm := normalize(dotKey)
	for _, entry := range os.Environ() {
		envKey, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if normalize(envKey) == norm {
			_ = os.Unsetenv(envKey)
		}
	}
	ResetCache()
}

// TestGet verifies case-insensitive, separator-agnostic lookup.
//
// VALIDATES: Get finds env vars regardless of case or dot/underscore separator.
// PREVENTS: Missing case or separator variants.
func TestGet(t *testing.T) {
	const key = "ze.test.env.check"

	tests := []struct {
		name   string
		envKey string // key to set in environment
		envVal string
		want   string
	}{
		{"dot_notation", "ze.test.env.check", "dot", "dot"},
		{"lowercase_underscore", "ze_test_env_check", "under", "under"},
		{"uppercase_underscore", "ZE_TEST_ENV_CHECK", "upper", "upper"},
		{"mixed_case", "ZE_test_Env_CHECK", "mixed", "mixed"},
		{"mixed_case_dots", "Ze.Test.Env.Check", "dotmix", "dotmix"},
		{"none_set", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unsetAll(t, key)
			defer unsetAll(t, key)

			if tt.envKey != "" {
				require.NoError(t, os.Setenv(tt.envKey, tt.envVal))
				defer func() { _ = os.Unsetenv(tt.envKey) }()
			}

			assert.Equal(t, tt.want, Get(key))
		})
	}
}

// TestGetInt verifies integer parsing with default fallback.
//
// VALIDATES: GetInt returns parsed int or default for invalid/empty.
// PREVENTS: Panic on invalid input, wrong default handling.
func TestGetInt(t *testing.T) {
	const key = "ze.test.int.val"

	tests := []struct {
		name       string
		envKey     string
		envVal     string
		defaultVal int
		want       int
	}{
		{"valid_via_dot", "ze.test.int.val", "42", 0, 42},
		{"valid_via_upper", "ZE_TEST_INT_VAL", "99", 0, 99},
		{"invalid_returns_default", "ze.test.int.val", "nope", 7, 7},
		{"empty_returns_default", "", "", 7, 7},
		{"zero_value", "ze.test.int.val", "0", 5, 0},
		{"negative", "ze.test.int.val", "-3", 5, -3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unsetAll(t, key)
			defer unsetAll(t, key)

			if tt.envKey != "" && tt.envVal != "" {
				require.NoError(t, os.Setenv(tt.envKey, tt.envVal))
			}

			assert.Equal(t, tt.want, GetInt(key, tt.defaultVal))
		})
	}
}

// TestGetBool verifies boolean parsing.
//
// VALIDATES: GetBool recognizes true/false/1/0 and returns default for unrecognized.
// PREVENTS: Wrong boolean parsing or missing default.
func TestGetBool(t *testing.T) {
	const key = "ze.test.bool.val"

	tests := []struct {
		name       string
		envVal     string
		defaultVal bool
		want       bool
	}{
		{"true_string", "true", false, true},
		{"TRUE_string", "TRUE", false, true},
		{"one_string", "1", false, true},
		{"false_string", "false", true, false},
		{"FALSE_string", "FALSE", true, false},
		{"zero_string", "0", true, false},
		{"unrecognized_returns_default_true", "maybe", true, true},
		{"unrecognized_returns_default_false", "maybe", false, false},
		{"empty_returns_default", "", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unsetAll(t, key)
			defer unsetAll(t, key)

			if tt.envVal != "" {
				require.NoError(t, os.Setenv("ze.test.bool.val", tt.envVal))
			}

			assert.Equal(t, tt.want, GetBool(key, tt.defaultVal))
		})
	}
}

// TestIsEnabled verifies the enabling-value check.
//
// VALIDATES: IsEnabled recognizes 1/true/yes/on/enable/enabled.
// PREVENTS: Missing recognition of common enabling values.
func TestIsEnabled(t *testing.T) {
	const key = "ze.test.enabled.val"

	tests := []struct {
		name   string
		envVal string
		want   bool
	}{
		{"one", "1", true},
		{"true", "true", true},
		{"TRUE", "TRUE", true},
		{"yes", "yes", true},
		{"on", "on", true},
		{"enable", "enable", true},
		{"enabled", "Enabled", true},
		{"zero", "0", false},
		{"false", "false", false},
		{"empty", "", false},
		{"random", "maybe", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unsetAll(t, key)
			defer unsetAll(t, key)

			if tt.envVal != "" {
				require.NoError(t, os.Setenv("ze.test.enabled.val", tt.envVal))
			}

			assert.Equal(t, tt.want, IsEnabled(key))
		})
	}
}

// TestGetDuration verifies duration parsing.
//
// VALIDATES: GetDuration parses Go duration strings or returns default.
// PREVENTS: Wrong parsing, panic on invalid.
func TestGetDuration(t *testing.T) {
	const key = "ze.test.dur.val"
	const dflt = 5 * time.Second

	tests := []struct {
		name   string
		envVal string
		want   time.Duration
	}{
		{"valid", "10s", 10 * time.Second},
		{"milliseconds", "500ms", 500 * time.Millisecond},
		{"invalid_returns_default", "nope", dflt},
		{"empty_returns_default", "", dflt},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unsetAll(t, key)
			defer unsetAll(t, key)

			if tt.envVal != "" {
				require.NoError(t, os.Setenv("ze.test.dur.val", tt.envVal))
			}

			assert.Equal(t, tt.want, GetDuration(key, dflt))
		})
	}
}

// TestGetInt64 verifies int64 parsing.
//
// VALIDATES: GetInt64 handles large values correctly.
// PREVENTS: Overflow on large values.
func TestGetInt64(t *testing.T) {
	const key = "ze.test.i64.val"

	tests := []struct {
		name       string
		envVal     string
		defaultVal int64
		want       int64
	}{
		{"valid", "1048576", 0, 1048576},
		{"max_int64", "9223372036854775807", 0, 9223372036854775807},
		{"invalid_returns_default", "nope", 42, 42},
		{"empty_returns_default", "", 42, 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unsetAll(t, key)
			defer unsetAll(t, key)

			if tt.envVal != "" {
				require.NoError(t, os.Setenv("ze.test.i64.val", tt.envVal))
			}

			assert.Equal(t, tt.want, GetInt64(key, tt.defaultVal))
		})
	}
}

// TestSet verifies Set updates both cache and os env.
//
// VALIDATES: Set stores value retrievable by Get without ResetCache.
// PREVENTS: Cache/os desync, missing registration check.
func TestSet(t *testing.T) {
	const key = "ze.test.set.val"

	unsetAll(t, key)
	defer unsetAll(t, key)

	require.NoError(t, Set(key, "hello"))
	assert.Equal(t, "hello", Get(key), "Get should return value set by Set")
	assert.Equal(t, "hello", os.Getenv(key), "os.Getenv should also see the value")
}

// TestSetInt verifies SetInt stores an integer.
//
// VALIDATES: SetInt converts int to string and stores it.
// PREVENTS: Wrong int-to-string conversion.
func TestSetInt(t *testing.T) {
	const key = "ze.test.set.val"

	unsetAll(t, key)
	defer unsetAll(t, key)

	require.NoError(t, SetInt(key, 42))
	assert.Equal(t, 42, GetInt(key, 0))
}

// TestSetBool verifies SetBool stores a boolean.
//
// VALIDATES: SetBool converts bool to "true"/"false".
// PREVENTS: Wrong bool-to-string conversion.
func TestSetBool(t *testing.T) {
	const key = "ze.test.set.val"

	unsetAll(t, key)
	defer unsetAll(t, key)

	require.NoError(t, SetBool(key, true))
	assert.Equal(t, true, GetBool(key, false))

	require.NoError(t, SetBool(key, false))
	assert.Equal(t, false, GetBool(key, true))
}

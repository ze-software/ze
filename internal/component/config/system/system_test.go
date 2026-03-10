package system_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/system"
)

// TestExpandEnvValue verifies $VAR expansion resolves from environment.
//
// VALIDATES: Values starting with $ are resolved from OS environment.
// PREVENTS: Literal $VAR being passed through as hostname.
func TestExpandEnvValue(t *testing.T) {
	t.Setenv("TEST_ZE_HOST", "router1")
	result := system.ExpandEnvValue("$TEST_ZE_HOST")
	assert.Equal(t, "router1", result)
}

// TestExpandEnvValue_NoPrefix verifies non-$ values are returned as-is.
//
// VALIDATES: Plain string values are not modified.
// PREVENTS: Accidental environment lookup on normal values.
func TestExpandEnvValue_NoPrefix(t *testing.T) {
	result := system.ExpandEnvValue("router1")
	assert.Equal(t, "router1", result)
}

// TestExpandEnvValue_EmptyEnv verifies empty env var returns literal $VAR.
//
// VALIDATES: Unset or empty env var keeps the literal $VAR string.
// PREVENTS: Empty string hostname when env var is not set.
func TestExpandEnvValue_EmptyEnv(t *testing.T) {
	t.Setenv("TEST_ZE_EMPTY", "")
	result := system.ExpandEnvValue("$TEST_ZE_EMPTY")
	assert.Equal(t, "$TEST_ZE_EMPTY", result)

	// Also test completely unset var
	result2 := system.ExpandEnvValue("$TEST_ZE_DOES_NOT_EXIST_XYZ")
	assert.Equal(t, "$TEST_ZE_DOES_NOT_EXIST_XYZ", result2)
}

// TestExtractSystemConfig verifies basic system config extraction.
//
// VALIDATES: system { host X; domain Y; } values are extracted from tree.
// PREVENTS: System identity config being inaccessible at runtime.
func TestExtractSystemConfig(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	sys.Set("host", "router1")
	sys.Set("domain", "dc1.example.com")

	sc := system.ExtractSystemConfig(tree)
	assert.Equal(t, "router1", sc.Host)
	assert.Equal(t, "dc1.example.com", sc.Domain)
}

// TestExtractSystemConfig_EnvExpansion verifies $ENV expansion in host/domain.
//
// VALIDATES: $ENV values in system config are resolved from OS environment.
// PREVENTS: Literal $HOSTNAME being used as the system identity.
func TestExtractSystemConfig_EnvExpansion(t *testing.T) {
	t.Setenv("TEST_ZE_HOSTNAME", "myrouter")
	t.Setenv("TEST_ZE_DOMAIN", "lab.net")

	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	sys.Set("host", "$TEST_ZE_HOSTNAME")
	sys.Set("domain", "$TEST_ZE_DOMAIN")

	sc := system.ExtractSystemConfig(tree)
	assert.Equal(t, "myrouter", sc.Host)
	assert.Equal(t, "lab.net", sc.Domain)
}

// TestExtractSystemConfig_Missing verifies defaults when no system block exists.
//
// VALIDATES: Missing system block produces default values (host="unknown", domain="").
// PREVENTS: Nil pointer or panic when system block is absent.
func TestExtractSystemConfig_Missing(t *testing.T) {
	tree := config.NewTree()

	sc := system.ExtractSystemConfig(tree)
	assert.Equal(t, "unknown", sc.Host)
	assert.Equal(t, "", sc.Domain)
}

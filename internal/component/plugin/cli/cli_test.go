package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// VALIDATES: Pure functions in cli.go (resolveHexInput, envFDInt, availableFeatures, writeError, BaseConfig).
// PREVENTS: Regressions in plugin CLI flag handling and feature detection.

// TestResolveHexInput_DirectValue verifies direct hex string passes through.
func TestResolveHexInput_DirectValue(t *testing.T) {
	hex, ok := resolveHexInput("DEADBEEF")
	assert.True(t, ok)
	assert.Equal(t, "DEADBEEF", hex)
}

// TestResolveHexInput_EmptyString verifies empty input is returned as-is.
func TestResolveHexInput_EmptyString(t *testing.T) {
	hex, ok := resolveHexInput("")
	assert.True(t, ok)
	assert.Equal(t, "", hex)
}

// TestEnvFDInt_ValidValue verifies numeric env var parsing.
func TestEnvFDInt_ValidValue(t *testing.T) {
	t.Setenv("TEST_FD", "42")
	fd, err := envFDInt("TEST_FD")
	require.NoError(t, err)
	assert.Equal(t, 42, fd)
}

// TestEnvFDInt_NotSet verifies error when env var is missing.
func TestEnvFDInt_NotSet(t *testing.T) {
	// Use a name that is extremely unlikely to be set in the environment.
	_, err := envFDInt("ZE_TEST_FD_DEFINITELY_NOT_SET_12345")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not set")
}

// TestEnvFDInt_NonNumeric verifies error for non-integer value.
func TestEnvFDInt_NonNumeric(t *testing.T) {
	t.Setenv("TEST_FD_BAD", "abc")
	_, err := envFDInt("TEST_FD_BAD")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TEST_FD_BAD")
}

// TestAvailableFeatures_NLRIOnly verifies feature string when only NLRI is supported.
func TestAvailableFeatures_NLRIOnly(t *testing.T) {
	cfg := PluginConfig{SupportsNLRI: true}
	assert.Equal(t, "--nlri", availableFeatures(cfg))
}

// TestAvailableFeatures_CapaOnly verifies feature string when only capability is supported.
func TestAvailableFeatures_CapaOnly(t *testing.T) {
	cfg := PluginConfig{SupportsCapa: true}
	assert.Equal(t, "--capa", availableFeatures(cfg))
}

// TestAvailableFeatures_Both verifies feature string with NLRI and capability.
func TestAvailableFeatures_Both(t *testing.T) {
	cfg := PluginConfig{SupportsNLRI: true, SupportsCapa: true}
	assert.Equal(t, "--nlri, --capa", availableFeatures(cfg))
}

// TestAvailableFeatures_NoneWithYANG verifies fallback to --yang when no decode features.
func TestAvailableFeatures_NoneWithYANG(t *testing.T) {
	cfg := PluginConfig{GetYANG: func() string { return "" }}
	assert.Equal(t, "--yang", availableFeatures(cfg))
}

// TestAvailableFeatures_NoneAtAll verifies "none" when no features at all.
func TestAvailableFeatures_NoneAtAll(t *testing.T) {
	cfg := PluginConfig{}
	assert.Equal(t, "none", availableFeatures(cfg))
}

// TestWriteError_FormatsMessage verifies writeError formats and writes to buffer.
func TestWriteError_FormatsMessage(t *testing.T) {
	var buf bytes.Buffer
	writeError(&buf, "error: %s failed", "test")
	assert.Equal(t, "error: test failed\n", buf.String())
}

// TestUnsupportedFeatureError_FormatsMessage verifies the standard error format.
func TestUnsupportedFeatureError_FormatsMessage(t *testing.T) {
	var buf bytes.Buffer
	unsupportedFeatureError(&buf, "evpn", "capa", "--nlri")
	assert.Contains(t, buf.String(), "evpn")
	assert.Contains(t, buf.String(), "capa")
	assert.Contains(t, buf.String(), "--nlri")
}

// TestBaseConfig_CopiesFields verifies BaseConfig transfers Registration fields to PluginConfig.
func TestBaseConfig_CopiesFields(t *testing.T) {
	reg := &registry.Registration{
		Name:         "test-plugin",
		Features:     "nlri yang",
		SupportsNLRI: true,
		SupportsCapa: false,
	}

	cfg := BaseConfig(reg)
	assert.Equal(t, "test-plugin", cfg.Name)
	assert.Equal(t, "nlri yang", cfg.Features)
	assert.True(t, cfg.SupportsNLRI)
	assert.False(t, cfg.SupportsCapa)
}

// TestRunPlugin_FeaturesFlag verifies --features flag prints features and exits 0.
func TestRunPlugin_FeaturesFlag(t *testing.T) {
	cfg := PluginConfig{
		Name:     "test",
		Features: "nlri yang",
	}

	code := RunPlugin(cfg, []string{"--features"})
	assert.Equal(t, 0, code)
}

// TestRunPlugin_YANGFlag verifies --yang flag invokes GetYANG handler.
func TestRunPlugin_YANGFlag(t *testing.T) {
	called := false
	cfg := PluginConfig{
		Name: "test",
		GetYANG: func() string {
			called = true
			return "module test {}"
		},
	}

	code := RunPlugin(cfg, []string{"--yang"})
	assert.Equal(t, 0, code)
	assert.True(t, called)
}

// TestRunPlugin_UnsupportedNLRI verifies exit 1 when --nlri used but not supported.
func TestRunPlugin_UnsupportedNLRI(t *testing.T) {
	cfg := PluginConfig{
		Name:         "test",
		SupportsNLRI: false,
	}

	code := RunPlugin(cfg, []string{"--nlri", "DEADBEEF"})
	assert.Equal(t, 1, code)
}

// TestRunPlugin_UnsupportedCapa verifies exit 1 when --capa used but not supported.
func TestRunPlugin_UnsupportedCapa(t *testing.T) {
	cfg := PluginConfig{
		Name:         "test",
		SupportsCapa: false,
	}

	code := RunPlugin(cfg, []string{"--capa", "DEADBEEF"})
	assert.Equal(t, 1, code)
}

package rib

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
)

// extractMultipathJSON runs extractMultipathConfig over the bgp subtree of a
// Stage 2 section body.
func extractMultipathJSON(t *testing.T, jsonStr string) (uint32, bool, error) {
	t.Helper()
	bgp, ok := configjson.ParseBGPSubtree(jsonStr)
	require.True(t, ok, "section %s holds no bgp subtree", jsonStr)
	return extractMultipathConfig(bgp)
}

// TestExtractMultipathConfigPresent verifies that a bgp/multipath container
// with both fields populated is parsed correctly.
//
// VALIDATES: cmd-3 AC-2 -- maximum-paths and relax-as-path reach the RIB
// plugin through the Stage 2 configure callback.
// PREVENTS: Multipath config silently dropped between the editor and the
// RIB plugin so ECMP never activates.
func TestExtractMultipathConfigPresent(t *testing.T) {
	jsonStr := `{"bgp":{"multipath":{"maximum-paths":4,"relax-as-path":true}}}`
	maxPaths, relax, err := extractMultipathJSON(t, jsonStr)
	require.NoError(t, err)
	assert.Equal(t, uint32(4), maxPaths)
	assert.True(t, relax)
}

// TestExtractMultipathConfigMissing verifies that an absent multipath container
// reads as the RFC 4271 default (1), so a reload that removes it restores it.
//
// VALIDATES: cmd-3 AC-1 -- no multipath config means single best-path.
// PREVENTS: Unintended ECMP activation on daemons that never configured it.
func TestExtractMultipathConfigMissing(t *testing.T) {
	jsonStr := `{"bgp":{"router-id":"10.0.0.1"}}`
	maxPaths, relax, err := extractMultipathJSON(t, jsonStr)
	require.NoError(t, err)
	assert.Equal(t, uint32(1), maxPaths)
	assert.False(t, relax)
}

// TestExtractMultipathConfigDefault verifies that a bgp tree with an empty
// multipath container (only the default-populated leaves) returns the default.
//
// VALIDATES: cmd-3 AC-1 -- YANG defaults survive the JSON round-trip.
// PREVENTS: Missing maximum-paths defaulting to 0 (disabled) instead of 1.
func TestExtractMultipathConfigDefault(t *testing.T) {
	jsonStr := `{"bgp":{"multipath":{"maximum-paths":1,"relax-as-path":false}}}`
	maxPaths, relax, err := extractMultipathJSON(t, jsonStr)
	require.NoError(t, err)
	assert.Equal(t, uint32(1), maxPaths)
	assert.False(t, relax)
}

// TestExtractMultipathConfigBoundary verifies that the maximum YANG value
// (256) is accepted and passed through.
//
// VALIDATES: cmd-3 AC-9 boundary -- 256 is valid.
// PREVENTS: Silent truncation of the maximum configured value.
func TestExtractMultipathConfigBoundary(t *testing.T) {
	jsonStr := `{"bgp":{"multipath":{"maximum-paths":256}}}`
	maxPaths, _, err := extractMultipathJSON(t, jsonStr)
	require.NoError(t, err)
	assert.Equal(t, uint32(256), maxPaths)
}

// TestExtractMultipathConfigOutOfRange verifies that an out-of-range value
// (e.g. 1000 or 0) is refused with an error naming the leaf, never read as the default.
//
// VALIDATES: Defensive bound check independent of YANG validation.
// PREVENTS: A raw JSON delivery path (tests, plugin IPC) smuggling an
// out-of-range value past YANG's 1..256 enforcement.
func TestExtractMultipathConfigOutOfRange(t *testing.T) {
	for _, js := range []string{
		`{"bgp":{"multipath":{"maximum-paths":1000}}}`,
		`{"bgp":{"multipath":{"maximum-paths":"1000"}}}`,
		`{"bgp":{"multipath":{"maximum-paths":0}}}`,
		`{"bgp":{"multipath":{"maximum-paths":-1}}}`,
		`{"bgp":{"multipath":{"maximum-paths":"abc"}}}`,
	} {
		_, _, err := extractMultipathJSON(t, js)
		require.Error(t, err, "out-of-range %q must be refused", js)
		assert.Contains(t, err.Error(), "maximum-paths")
		assert.Contains(t, err.Error(), maximumPathsReason, "the refusal says why")
		assert.Contains(t, err.Error(), "set bgp multipath maximum-paths <1-256>", "the refusal names the fix")
	}
}

// TestExtractMultipathConfigRelaxInvalid verifies that a relax-as-path value
// that is not a boolean is refused, naming the leaf and the fix, and that the
// two boolean spellings a serialized tree carries are read.
//
// VALIDATES: relax-as-path refused at verify when it is not a boolean.
// PREVENTS: any non-"true" value read silently as false, which keeps strict
// AS_PATH matching while the commit reports the edit taken.
func TestExtractMultipathConfigRelaxInvalid(t *testing.T) {
	for _, js := range []string{
		`{"bgp":{"multipath":{"relax-as-path":"yes"}}}`,
		`{"bgp":{"multipath":{"relax-as-path":"TRUE"}}}`,
		`{"bgp":{"multipath":{"relax-as-path":1}}}`,
	} {
		_, _, err := extractMultipathJSON(t, js)
		require.Error(t, err, "invalid relax-as-path %q must be refused", js)
		assert.Contains(t, err.Error(), "relax-as-path")
		assert.Contains(t, err.Error(), "set bgp multipath relax-as-path true (or false)", "the refusal names the fix")
	}
	_, relax, err := extractMultipathJSON(t, `{"bgp":{"multipath":{"relax-as-path":"false"}}}`)
	require.NoError(t, err)
	assert.False(t, relax)

	_, err = parseRIBConfigOnly(`{"bgp":{"multipath":{"relax-as-path":"yes"}}}`)
	require.Error(t, err, "the config transaction refuses it at verify")
}

// parseRIBConfigOnly runs the config transaction's parse, which verify calls.
func parseRIBConfigOnly(data string) (ribConfig, error) {
	cfg, _, err := parseRIBConfig(ribSections(data[len(`{"bgp":{`) : len(data)-2]))
	return cfg, err
}

// TestExtractMultipathConfigStringNumber verifies that multipath values
// serialized as strings (common in some tree->JSON paths) still parse.
//
// VALIDATES: cmd-3 robustness against config serialization formats.
// PREVENTS: Plugin silently ignoring config depending on round-trip format.
func TestExtractMultipathConfigStringNumber(t *testing.T) {
	jsonStr := `{"bgp":{"multipath":{"maximum-paths":"8","relax-as-path":"true"}}}`
	maxPaths, relax, err := extractMultipathJSON(t, jsonStr)
	require.NoError(t, err)
	assert.Equal(t, uint32(8), maxPaths)
	assert.True(t, relax)
}

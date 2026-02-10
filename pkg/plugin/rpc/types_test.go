package rpc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigVerifyInputMarshal verifies JSON round-trip for ConfigVerifyInput.
//
// VALIDATES: ConfigVerifyInput marshals/unmarshals correctly with kebab-case keys.
// PREVENTS: Malformed config-verify RPC payloads on the wire.
func TestConfigVerifyInputMarshal(t *testing.T) {
	t.Parallel()

	input := ConfigVerifyInput{
		Sections: []ConfigSection{
			{Root: "bgp", Data: `{"router-id":"1.2.3.4"}`},
			{Root: "hub", Data: `{"bind":"0.0.0.0:179"}`},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	var decoded ConfigVerifyInput
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Len(t, decoded.Sections, 2)
	assert.Equal(t, "bgp", decoded.Sections[0].Root)
	assert.Equal(t, `{"router-id":"1.2.3.4"}`, decoded.Sections[0].Data)
	assert.Equal(t, "hub", decoded.Sections[1].Root)
}

// TestConfigApplyInputMarshal verifies JSON round-trip for ConfigApplyInput.
//
// VALIDATES: ConfigApplyInput with ConfigDiffSection marshals/unmarshals correctly.
// PREVENTS: Malformed config-apply RPC payloads on the wire.
func TestConfigApplyInputMarshal(t *testing.T) {
	t.Parallel()

	input := ConfigApplyInput{
		Sections: []ConfigDiffSection{
			{
				Root:    "bgp",
				Added:   `{"peer":{"new-peer":{"address":"10.0.0.1"}}}`,
				Removed: `{"peer":{"old-peer":{}}}`,
				Changed: `{"router-id":"5.6.7.8"}`,
			},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	var decoded ConfigApplyInput
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Len(t, decoded.Sections, 1)
	assert.Equal(t, "bgp", decoded.Sections[0].Root)
	assert.Equal(t, `{"peer":{"new-peer":{"address":"10.0.0.1"}}}`, decoded.Sections[0].Added)
	assert.Equal(t, `{"peer":{"old-peer":{}}}`, decoded.Sections[0].Removed)
	assert.Equal(t, `{"router-id":"5.6.7.8"}`, decoded.Sections[0].Changed)
}

// TestConfigDiffSectionMarshal verifies JSON round-trip for ConfigDiffSection.
//
// VALIDATES: ConfigDiffSection omits empty fields correctly.
// PREVENTS: Unnecessary empty fields bloating the JSON payload.
func TestConfigDiffSectionMarshal(t *testing.T) {
	t.Parallel()

	// Only added, no removed/changed
	section := ConfigDiffSection{
		Root:  "bgp",
		Added: `{"peer":{"p1":{}}}`,
	}

	data, err := json.Marshal(section)
	require.NoError(t, err)

	// Verify omitempty works — removed and changed should not appear
	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Contains(t, raw, "root")
	assert.Contains(t, raw, "added")
	assert.NotContains(t, raw, "removed")
	assert.NotContains(t, raw, "changed")

	// Round-trip
	var decoded ConfigDiffSection
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "bgp", decoded.Root)
	assert.Equal(t, `{"peer":{"p1":{}}}`, decoded.Added)
	assert.Empty(t, decoded.Removed)
	assert.Empty(t, decoded.Changed)
}

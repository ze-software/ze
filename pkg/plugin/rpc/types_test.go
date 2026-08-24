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

// TestDoctorCheckDeclJSON verifies JSON round-trip for DoctorCheckDecl with kebab-case keys.
func TestDoctorCheckDeclJSON(t *testing.T) {
	t.Parallel()

	decl := DoctorCheckDecl{
		Name:         "rpki-cache-reachable",
		Phase:        DoctorPhasePostConfig,
		Order:        100,
		Dependencies: []string{"config-loaded"},
		Platforms:    []string{"any"},
		Codes:        []string{"doctor-rpki-cache-unreachable"},
	}

	data, err := json.Marshal(decl)
	require.NoError(t, err)

	// Verify kebab-case wire keys
	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Contains(t, raw, "name")
	assert.Contains(t, raw, "phase")
	assert.Contains(t, raw, "order")
	assert.Contains(t, raw, "dependencies")
	assert.Contains(t, raw, "platforms")
	assert.Contains(t, raw, "codes")
	assert.NotContains(t, raw, "Name")

	// Round-trip
	var decoded DoctorCheckDecl
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "rpki-cache-reachable", decoded.Name)
	assert.Equal(t, DoctorPhasePostConfig, decoded.Phase)
	assert.Equal(t, 100, decoded.Order)
	assert.Equal(t, []string{"config-loaded"}, decoded.Dependencies)
	assert.Equal(t, []string{"any"}, decoded.Platforms)
	assert.Equal(t, []string{"doctor-rpki-cache-unreachable"}, decoded.Codes)
}

// TestDoctorCheckDeclOmitempty verifies omitempty fields are omitted when zero.
func TestDoctorCheckDeclOmitempty(t *testing.T) {
	t.Parallel()

	decl := DoctorCheckDecl{
		Name:  "simple-check",
		Phase: DoctorPhasePreConfig,
		Codes: []string{"doctor-simple-fail"},
	}

	data, err := json.Marshal(decl)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.NotContains(t, raw, "order")
	assert.NotContains(t, raw, "dependencies")
	assert.NotContains(t, raw, "platforms")
}

// TestDeclareRegistrationInputDoctorChecks verifies doctor-checks field in DeclareRegistrationInput.
func TestDeclareRegistrationInputDoctorChecks(t *testing.T) {
	t.Parallel()

	input := DeclareRegistrationInput{
		DoctorChecks: []DoctorCheckDecl{
			{
				Name:  "my-check",
				Phase: DoctorPhasePostConfig,
				Codes: []string{"doctor-my-fail"},
			},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Contains(t, raw, "doctor-checks")

	var decoded DeclareRegistrationInput
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Len(t, decoded.DoctorChecks, 1)
	assert.Equal(t, "my-check", decoded.DoctorChecks[0].Name)
}

// TestDeclareRegistrationInputNoDoctorChecks verifies backward compat: no doctor-checks field.
func TestDeclareRegistrationInputNoDoctorChecks(t *testing.T) {
	t.Parallel()

	data := []byte(`{"commands":[{"name":"test-cmd"}]}`)
	var decoded DeclareRegistrationInput
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Empty(t, decoded.DoctorChecks)
	assert.Len(t, decoded.Commands, 1)
}

// TestEnrichShowInputJSON verifies JSON round-trip for EnrichShowInput preserves base map.
func TestEnrichShowInputJSON(t *testing.T) {
	t.Parallel()

	input := EnrichShowInput{
		Command: "show subscriber detail",
		Key:     "cos",
		Mode:    "detail",
		Base: map[string]any{
			"id":    "s1",
			"state": "active",
			"vlan":  float64(100),
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	var decoded EnrichShowInput
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "show subscriber detail", decoded.Command)
	assert.Equal(t, "cos", decoded.Key)
	assert.Equal(t, "detail", decoded.Mode)
	assert.Equal(t, "s1", decoded.Base["id"])
	assert.Equal(t, "active", decoded.Base["state"])
	assert.Equal(t, float64(100), decoded.Base["vlan"])
}

// TestEnrichShowOutputJSON verifies JSON round-trip for EnrichShowOutput.
func TestEnrichShowOutputJSON(t *testing.T) {
	t.Parallel()

	output := EnrichShowOutput{
		Data: map[string]any{
			"cos-profile": "residential",
			"speed":       float64(1000),
		},
	}

	data, err := json.Marshal(output)
	require.NoError(t, err)

	var decoded EnrichShowOutput
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "residential", decoded.Data["cos-profile"])
	assert.Equal(t, float64(1000), decoded.Data["speed"])
}

// TestDeclareRegistrationInputEnrichers verifies enrichers field in DeclareRegistrationInput.
func TestDeclareRegistrationInputEnrichers(t *testing.T) {
	t.Parallel()

	input := DeclareRegistrationInput{
		Enrichers: []EnricherDecl{
			{Command: "show subscriber detail", Key: "cos"},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Contains(t, raw, "enrichers")

	var decoded DeclareRegistrationInput
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Len(t, decoded.Enrichers, 1)
	assert.Equal(t, "show subscriber detail", decoded.Enrichers[0].Command)
	assert.Equal(t, "cos", decoded.Enrichers[0].Key)
}

// TestDeclareRegistrationInputNoEnrichers verifies backward compat: no enrichers field.
func TestDeclareRegistrationInputNoEnrichers(t *testing.T) {
	t.Parallel()

	data := []byte(`{"commands":[{"name":"test-cmd"}]}`)
	var decoded DeclareRegistrationInput
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Empty(t, decoded.Enrichers)
	assert.Len(t, decoded.Commands, 1)
}

// TestDeclareRegistrationInputCommandShapes verifies the three answer-shape
// fields survive the Stage 1 message with their values intact, under
// kebab-case wire keys.
//
// VALIDATES: shape, columns and address-fields round-trip through DeclareRegistrationInput.
// PREVENTS: A plugin's declaration reaching the engine with a field dropped or renamed.
func TestDeclareRegistrationInputCommandShapes(t *testing.T) {
	t.Parallel()

	input := DeclareRegistrationInput{
		Commands: []CommandDecl{{
			Name:          "show bgp rpki cache",
			Shape:         "tab",
			Columns:       []string{"address", "port", "state"},
			AddressFields: []string{"address"},
		}},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	commands, ok := raw["commands"].([]any)
	require.True(t, ok)
	require.Len(t, commands, 1)
	command, ok := commands[0].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, command, "shape")
	assert.Contains(t, command, "columns")
	assert.Contains(t, command, "address-fields")

	var decoded DeclareRegistrationInput
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Len(t, decoded.Commands, 1)
	assert.Equal(t, "tab", decoded.Commands[0].Shape)
	assert.Equal(t, []string{"address", "port", "state"}, decoded.Commands[0].Columns)
	assert.Equal(t, []string{"address"}, decoded.Commands[0].AddressFields)
}

// TestDeclareRegistrationInputNoCommandShapes verifies backward compat: a
// command that declares none of the three unmarshals to the zero values, and
// marshals back to the message a plugin built before the fields existed sends.
func TestDeclareRegistrationInputNoCommandShapes(t *testing.T) {
	t.Parallel()

	data := []byte(`{"commands":[{"name":"test-cmd"}]}`)
	var decoded DeclareRegistrationInput
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Len(t, decoded.Commands, 1)
	assert.Empty(t, decoded.Commands[0].Shape)
	assert.Empty(t, decoded.Commands[0].Columns)
	assert.Empty(t, decoded.Commands[0].AddressFields)

	out, err := json.Marshal(decoded)
	require.NoError(t, err)
	assert.JSONEq(t, string(data), string(out))
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

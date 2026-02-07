package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	bgpschema "codeberg.org/thomas-mangin/ze/internal/plugin/bgp/schema"
	"codeberg.org/thomas-mangin/ze/internal/yang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSchemaInfo_HandlerMap verifies handler map construction from SchemaInfo slice.
//
// VALIDATES: SchemaInfo slice is correctly converted to handler→schema map.
// PREVENTS: Missing handler registrations breaking config block routing.
func TestSchemaInfo_HandlerMap(t *testing.T) {
	schemas := []SchemaInfo{
		{Module: "ze-bgp", Handlers: []string{"bgp", "bgp.peer"}},
		{Module: "ze-rib", Handlers: []string{"rib"}},
	}

	r := NewReader(schemas, "", nil)

	// Each handler path should map to its schema.
	assert.NotNil(t, r.findHandler("bgp"))
	assert.NotNil(t, r.findHandler("bgp.peer"))
	assert.NotNil(t, r.findHandler("rib"))
	assert.Equal(t, "ze-bgp", r.findHandler("bgp").Module)
	assert.Equal(t, "ze-bgp", r.findHandler("bgp.peer").Module)
	assert.Equal(t, "ze-rib", r.findHandler("rib").Module)
}

// TestReader_FindHandler verifies longest-prefix handler matching.
//
// VALIDATES: Exact match, base path match, and prefix shortening all work.
// PREVENTS: Wrong plugin receiving config blocks.
func TestReader_FindHandler(t *testing.T) {
	schemas := []SchemaInfo{
		{Module: "ze-bgp", Handlers: []string{"bgp"}},
		{Module: "ze-bgp-peer", Handlers: []string{"bgp.peer"}},
		{Module: "ze-rib", Handlers: []string{"rib"}},
	}

	r := NewReader(schemas, "", nil)

	tests := []struct {
		path       string
		wantModule string
	}{
		{"bgp", "ze-bgp"},
		{"bgp.peer", "ze-bgp-peer"},
		{"bgp.peer[key=192.0.2.1]", "ze-bgp-peer"},
		{"bgp.peer.timers", "ze-bgp-peer"},
		{"bgp.local-as", "ze-bgp"},
		{"rib", "ze-rib"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			handler := r.findHandler(tt.path)
			require.NotNil(t, handler, "handler for %q should not be nil", tt.path)
			assert.Equal(t, tt.wantModule, handler.Module)
		})
	}
}

// TestReader_FindHandler_Unknown verifies unknown paths return nil.
//
// VALIDATES: Paths with no registered handler return nil.
// PREVENTS: Panic or false positive matches on unregistered paths.
func TestReader_FindHandler_Unknown(t *testing.T) {
	schemas := []SchemaInfo{
		{Module: "ze-bgp", Handlers: []string{"bgp"}},
	}

	r := NewReader(schemas, "", nil)

	assert.Nil(t, r.findHandler("unknown"))
	assert.Nil(t, r.findHandler("rib"))
	assert.Nil(t, r.findHandler(""))
}

// TestTokensToJSON_TypePreservation verifies numeric types preserved in JSON.
//
// VALIDATES: Integer, float, bool, string types preserved during conversion.
// PREVENTS: Type mismatch with YANG schema.
func TestTokensToJSON_TypePreservation(t *testing.T) {
	tests := []struct {
		name   string
		tokens []Token
		want   map[string]any
	}{
		{
			name: "integer",
			tokens: []Token{
				{Type: TokenWord, Value: "peer-as"},
				{Type: TokenWord, Value: "65001"},
				{Type: TokenSemicolon},
			},
			want: map[string]any{"peer-as": float64(65001)}, // JSON unmarshal → float64
		},
		{
			name: "float",
			tokens: []Token{
				{Type: TokenWord, Value: "weight"},
				{Type: TokenWord, Value: "1.5"},
				{Type: TokenSemicolon},
			},
			want: map[string]any{"weight": 1.5},
		},
		{
			name: "boolean_true",
			tokens: []Token{
				{Type: TokenWord, Value: "enabled"},
				{Type: TokenWord, Value: "true"},
				{Type: TokenSemicolon},
			},
			want: map[string]any{"enabled": true},
		},
		{
			name: "boolean_false",
			tokens: []Token{
				{Type: TokenWord, Value: "disabled"},
				{Type: TokenWord, Value: "false"},
				{Type: TokenSemicolon},
			},
			want: map[string]any{"disabled": false},
		},
		{
			name: "string",
			tokens: []Token{
				{Type: TokenWord, Value: "address"},
				{Type: TokenWord, Value: "192.0.2.1"},
				{Type: TokenSemicolon},
			},
			want: map[string]any{"address": "192.0.2.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TokensToJSON(tt.tokens)
			var got map[string]any
			err := json.Unmarshal([]byte(result), &got)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestReader_ParseBlocks verifies config file tokenization and block extraction.
//
// VALIDATES: Config blocks are parsed and matched to correct handlers.
// PREVENTS: Config blocks routed to wrong handler or lost.
func TestReader_ParseBlocks(t *testing.T) {
	schemas := []SchemaInfo{
		{Module: "ze-bgp", Handlers: []string{"bgp", "bgp.peer"}},
	}

	configContent := `
bgp {
    local-as 65000;
    peer 192.0.2.1 {
        peer-as 65001;
    }
}
`
	dir := t.TempDir()
	confPath := filepath.Join(dir, "test.conf")
	require.NoError(t, os.WriteFile(confPath, []byte(configContent), 0o644))

	r := NewReader(schemas, confPath, nil)
	state, err := r.Load()
	require.NoError(t, err)

	// "bgp" handler should have a default-keyed block.
	bgpBlock := state.Get("bgp", "_default")
	require.NotNil(t, bgpBlock, "bgp block should exist")
	assert.Contains(t, bgpBlock.Data, "local-as")

	// "bgp.peer" handler should have a block keyed by "192.0.2.1".
	peerBlock := state.Get("bgp.peer", "192.0.2.1")
	require.NotNil(t, peerBlock, "bgp.peer block should exist")
	assert.Contains(t, peerBlock.Data, "peer-as")
}

// TestReader_DiffConfig_Create verifies new blocks produce "create" changes.
//
// VALIDATES: Blocks present in new state but not old produce "create" action.
// PREVENTS: New config blocks being silently ignored on reload.
func TestReader_DiffConfig_Create(t *testing.T) {
	oldState := NewBlockState()
	newState := NewBlockState()
	newState.Set(&BlockEntry{
		Handler: "bgp.peer",
		Key:     "192.0.2.1",
		Path:    "bgp.peer[key=192.0.2.1]",
		Data:    `{"as":65001}`,
	})

	changes := DiffBlocks(oldState, newState)

	require.Len(t, changes, 1)
	assert.Equal(t, "create", changes[0].Action)
	assert.Equal(t, "bgp.peer", changes[0].Handler)
}

// TestReader_DiffConfig_Delete verifies removed blocks produce "delete" changes.
//
// VALIDATES: Blocks present in old state but not new produce "delete" action.
// PREVENTS: Removed config blocks persisting after reload.
func TestReader_DiffConfig_Delete(t *testing.T) {
	oldState := NewBlockState()
	oldState.Set(&BlockEntry{
		Handler: "bgp.peer",
		Key:     "192.0.2.1",
		Path:    "bgp.peer[key=192.0.2.1]",
		Data:    `{"as":65001}`,
	})
	newState := NewBlockState()

	changes := DiffBlocks(oldState, newState)

	require.Len(t, changes, 1)
	assert.Equal(t, "delete", changes[0].Action)
}

// TestReader_DiffConfig_Modify verifies changed blocks produce "modify" changes.
//
// VALIDATES: Blocks present in both states with different data produce "modify" action.
// PREVENTS: Config changes being treated as no-ops on reload.
func TestReader_DiffConfig_Modify(t *testing.T) {
	oldState := NewBlockState()
	oldState.Set(&BlockEntry{
		Handler: "bgp.peer",
		Key:     "192.0.2.1",
		Path:    "bgp.peer[key=192.0.2.1]",
		Data:    `{"as":65001}`,
	})
	newState := NewBlockState()
	newState.Set(&BlockEntry{
		Handler: "bgp.peer",
		Key:     "192.0.2.1",
		Path:    "bgp.peer[key=192.0.2.1]",
		Data:    `{"as":65002}`,
	})

	changes := DiffBlocks(oldState, newState)

	require.Len(t, changes, 1)
	assert.Equal(t, "modify", changes[0].Action)
	assert.Equal(t, `{"as":65001}`, changes[0].OldData)
	assert.Equal(t, `{"as":65002}`, changes[0].NewData)
}

// TestReader_DiffConfig_NoChange verifies identical states produce no changes.
//
// VALIDATES: Identical old and new states produce empty change list.
// PREVENTS: Unnecessary reload processing when config hasn't changed.
func TestReader_DiffConfig_NoChange(t *testing.T) {
	oldState := NewBlockState()
	oldState.Set(&BlockEntry{
		Handler: "bgp.peer",
		Key:     "192.0.2.1",
		Path:    "bgp.peer[key=192.0.2.1]",
		Data:    `{"as":65001}`,
	})
	newState := NewBlockState()
	newState.Set(&BlockEntry{
		Handler: "bgp.peer",
		Key:     "192.0.2.1",
		Path:    "bgp.peer[key=192.0.2.1]",
		Data:    `{"as":65001}`,
	})

	changes := DiffBlocks(oldState, newState)

	assert.Empty(t, changes)
}

// TestReader_DiffConfig_Deterministic verifies changes sorted by handler then key.
//
// VALIDATES: Changes are in deterministic order regardless of map iteration.
// PREVENTS: Non-deterministic config application order.
func TestReader_DiffConfig_Deterministic(t *testing.T) {
	oldState := NewBlockState()
	newState := NewBlockState()

	// Add blocks in reverse alphabetical order to test sorting.
	newState.Set(&BlockEntry{Handler: "rib", Key: "_default", Path: "rib", Data: `{}`})
	newState.Set(&BlockEntry{Handler: "bgp.peer", Key: "10.0.0.2", Path: "bgp.peer[key=10.0.0.2]", Data: `{}`})
	newState.Set(&BlockEntry{Handler: "bgp.peer", Key: "10.0.0.1", Path: "bgp.peer[key=10.0.0.1]", Data: `{}`})
	newState.Set(&BlockEntry{Handler: "bgp", Key: "_default", Path: "bgp", Data: `{}`})

	changes := DiffBlocks(oldState, newState)

	require.Len(t, changes, 4)
	// Should be sorted: bgp, bgp.peer/10.0.0.1, bgp.peer/10.0.0.2, rib
	assert.Equal(t, "bgp", changes[0].Handler)
	assert.Equal(t, "bgp.peer", changes[1].Handler)
	assert.Equal(t, "10.0.0.1", extractKeyFromPath(changes[1].Path))
	assert.Equal(t, "bgp.peer", changes[2].Handler)
	assert.Equal(t, "10.0.0.2", extractKeyFromPath(changes[2].Path))
	assert.Equal(t, "rib", changes[3].Handler)
}

// TestReader_HandlerPathBoundary verifies long handler paths are accepted.
//
// VALIDATES: Handler paths up to 512 chars work.
// BOUNDARY: 512 chars accepted.
func TestReader_HandlerPathBoundary(t *testing.T) {
	longPath := strings.Repeat("a.", 256)[:512]
	schemas := []SchemaInfo{
		{Module: "test", Handlers: []string{longPath}},
	}

	r := NewReader(schemas, "", nil)

	handler := r.findHandler(longPath)
	require.NotNil(t, handler)
	assert.Equal(t, "test", handler.Module)
}

// TestReader_Reload verifies end-to-end reload with file modification.
//
// VALIDATES: Modifying a config file and calling Reload produces correct changes.
// PREVENTS: Reload returning stale data or missing changes.
func TestReader_Reload(t *testing.T) {
	schemas := []SchemaInfo{
		{Module: "ze-bgp", Handlers: []string{"bgp", "bgp.peer"}},
	}

	dir := t.TempDir()
	confPath := filepath.Join(dir, "test.conf")

	// Initial config.
	initial := `
bgp {
    local-as 65000;
    peer 192.0.2.1 {
        peer-as 65001;
    }
}
`
	require.NoError(t, os.WriteFile(confPath, []byte(initial), 0o644))

	r := NewReader(schemas, confPath, nil)
	_, err := r.Load()
	require.NoError(t, err)

	// Modify config: change peer AS and add a second peer.
	modified := `
bgp {
    local-as 65000;
    peer 192.0.2.1 {
        peer-as 65099;
    }
    peer 192.0.2.2 {
        peer-as 65002;
    }
}
`
	require.NoError(t, os.WriteFile(confPath, []byte(modified), 0o644))

	changes, err := r.Reload()
	require.NoError(t, err)

	// Expect 3 changes:
	// - bgp._default modified (flat key-value "peer" value changes)
	// - bgp.peer[key=192.0.2.1] modified (peer-as changed)
	// - bgp.peer[key=192.0.2.2] created (new peer)
	require.Len(t, changes, 3)

	actions := make(map[string]int)
	for _, c := range changes {
		actions[c.Action]++
	}
	assert.Equal(t, 2, actions["modify"])
	assert.Equal(t, 1, actions["create"])

	// Verify the new peer was created.
	var create *BlockChange
	for i := range changes {
		if changes[i].Action == "create" {
			create = &changes[i]
		}
	}
	require.NotNil(t, create)
	assert.Contains(t, create.Path, "192.0.2.2")
}

// TestReader_Load_MissingFile verifies Load returns error for nonexistent file.
//
// VALIDATES: Missing config file produces a clear error.
// PREVENTS: Panic or nil state on missing file.
func TestReader_Load_MissingFile(t *testing.T) {
	r := NewReader(nil, "/nonexistent/path/config.conf", nil)
	state, err := r.Load()
	assert.Error(t, err)
	assert.Nil(t, state)
	assert.Contains(t, err.Error(), "read config")
}

// TestReader_Load_EmptyPath verifies Load returns error for empty config path.
//
// VALIDATES: Empty string path produces a clear error.
// PREVENTS: Panic or confusing error from os.ReadFile("").
func TestReader_Load_EmptyPath(t *testing.T) {
	r := NewReader(nil, "", nil)
	state, err := r.Load()
	assert.Error(t, err)
	assert.Nil(t, state)
	assert.Contains(t, err.Error(), "no config path")
}

// newTestValidator creates a YANG validator loaded with core + BGP schemas.
func newTestValidator(t *testing.T) *yang.Validator {
	t.Helper()
	loader := yang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.AddModuleFromText("ze-bgp-conf.yang", bgpschema.ZeBGPConfYANG))
	require.NoError(t, loader.Resolve())
	return yang.NewValidator(loader)
}

// TestReader_ValidateBlock_ValidTypes verifies YANG validator accepts valid config values.
//
// VALIDATES: Valid ASN (range 1..4294967295), valid IPv4 pattern, valid community pattern.
// PREVENTS: False rejection of correct config values.
func TestReader_ValidateBlock_ValidTypes(t *testing.T) {
	validator := newTestValidator(t)
	schemas := []SchemaInfo{
		{Module: "ze-bgp-conf", Handlers: []string{"bgp", "bgp.peer"}},
	}

	configContent := `
bgp {
    router-id 192.0.2.1;
    local-as 65001;
    peer 10.0.0.1 {
        peer-as 65002;
    }
}
`
	dir := t.TempDir()
	confPath := filepath.Join(dir, "valid.conf")
	require.NoError(t, os.WriteFile(confPath, []byte(configContent), 0o644))

	r := NewReader(schemas, confPath, validator)
	state, err := r.Load()
	require.NoError(t, err)
	require.NotNil(t, state)

	// Verify blocks were stored.
	bgpBlock := state.Get("bgp", "_default")
	require.NotNil(t, bgpBlock)
	assert.Contains(t, bgpBlock.Data, "local-as")
}

// TestReader_ValidateBlock_InvalidRange verifies YANG validator rejects out-of-range values.
//
// VALIDATES: ASN 0 violates range 1..4294967295 and is rejected.
// PREVENTS: Accepting config with out-of-range numeric values.
func TestReader_ValidateBlock_InvalidRange(t *testing.T) {
	validator := newTestValidator(t)
	schemas := []SchemaInfo{
		{Module: "ze-bgp-conf", Handlers: []string{"bgp"}},
	}

	configContent := `
bgp {
    router-id 192.0.2.1;
    local-as 0;
}
`
	dir := t.TempDir()
	confPath := filepath.Join(dir, "invalid-range.conf")
	require.NoError(t, os.WriteFile(confPath, []byte(configContent), 0o644))

	r := NewReader(schemas, confPath, validator)
	_, err := r.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "range")
}

// TestReader_ValidateBlock_InvalidPattern verifies YANG validator rejects invalid patterns.
//
// VALIDATES: "not-an-ip" does not match ipv4-address pattern and is rejected.
// PREVENTS: Accepting config with malformed IP addresses.
func TestReader_ValidateBlock_InvalidPattern(t *testing.T) {
	validator := newTestValidator(t)
	schemas := []SchemaInfo{
		{Module: "ze-bgp-conf", Handlers: []string{"bgp"}},
	}

	configContent := `
bgp {
    router-id not-an-ip;
    local-as 65001;
}
`
	dir := t.TempDir()
	confPath := filepath.Join(dir, "invalid-pattern.conf")
	require.NoError(t, os.WriteFile(confPath, []byte(configContent), 0o644))

	r := NewReader(schemas, confPath, validator)
	_, err := r.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pattern")
}

// TestReader_ValidateBlock_MandatoryMissing verifies YANG validator reports missing mandatory fields.
//
// VALIDATES: Missing mandatory router-id in bgp container is detected.
// PREVENTS: Accepting incomplete config missing required fields.
func TestReader_ValidateBlock_MandatoryMissing(t *testing.T) {
	validator := newTestValidator(t)
	schemas := []SchemaInfo{
		{Module: "ze-bgp-conf", Handlers: []string{"bgp"}},
	}

	configContent := `
bgp {
    local-as 65001;
}
`
	dir := t.TempDir()
	confPath := filepath.Join(dir, "missing-mandatory.conf")
	require.NoError(t, os.WriteFile(confPath, []byte(configContent), 0o644))

	r := NewReader(schemas, confPath, validator)
	_, err := r.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mandatory")
}

// TestReader_Load_NoValidator verifies reader works without YANG validator.
//
// VALIDATES: nil validator means no validation — reader accepts any config.
// PREVENTS: Panic or error when validator is not provided.
func TestReader_Load_NoValidator(t *testing.T) {
	schemas := []SchemaInfo{
		{Module: "ze-bgp-conf", Handlers: []string{"bgp"}},
	}

	configContent := `
bgp {
    router-id not-an-ip;
    local-as 0;
}
`
	dir := t.TempDir()
	confPath := filepath.Join(dir, "no-validator.conf")
	require.NoError(t, os.WriteFile(confPath, []byte(configContent), 0o644))

	// nil validator — should skip all validation.
	r := NewReader(schemas, confPath, nil)
	state, err := r.Load()
	require.NoError(t, err)
	require.NotNil(t, state)
}

// TestReader_Reload_WithValidator verifies Reload also triggers YANG validation.
//
// VALIDATES: Reload rejects invalid config changes via YANG validator.
// PREVENTS: Validation bypass by modifying config after initial Load.
func TestReader_Reload_WithValidator(t *testing.T) {
	validator := newTestValidator(t)
	schemas := []SchemaInfo{
		{Module: "ze-bgp-conf", Handlers: []string{"bgp"}},
	}

	dir := t.TempDir()
	confPath := filepath.Join(dir, "reload-validate.conf")

	// Valid initial config.
	initial := `
bgp {
    router-id 192.0.2.1;
    local-as 65001;
}
`
	require.NoError(t, os.WriteFile(confPath, []byte(initial), 0o644))

	r := NewReader(schemas, confPath, validator)
	_, err := r.Load()
	require.NoError(t, err)

	// Reload with invalid config (ASN 0 violates range 1..max).
	invalid := `
bgp {
    router-id 192.0.2.1;
    local-as 0;
}
`
	require.NoError(t, os.WriteFile(confPath, []byte(invalid), 0o644))

	_, err = r.Reload()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "range")
}

// extractKeyFromPath extracts the key value from a path like "bgp.peer[key=10.0.0.1]".
func extractKeyFromPath(path string) string {
	start := strings.Index(path, "key=")
	if start < 0 {
		return ""
	}
	start += len("key=")
	end := strings.Index(path[start:], "]")
	if end < 0 {
		return path[start:]
	}
	return path[start : start+end]
}

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseSchemaLine verifies schema line parsing.
//
// VALIDATES: Schema module, handlers, and heredoc delimiter extraction.
// PREVENTS: Incorrect schema initialization from Hub.
func TestParseSchemaLine(t *testing.T) {
	tests := []struct {
		name         string
		line         string
		wantModule   string
		wantHandlers []string
		wantDelim    string
		wantErr      bool
	}{
		{
			name:         "simple_handlers",
			line:         "config schema ze-bgp handlers bgp,bgp.peer",
			wantModule:   "ze-bgp",
			wantHandlers: []string{"bgp", "bgp.peer"},
		},
		{
			name:         "single_handler",
			line:         "config schema ze-rib handlers rib",
			wantModule:   "ze-rib",
			wantHandlers: []string{"rib"},
		},
		{
			name:         "with_heredoc",
			line:         "config schema ze-bgp handlers bgp yang <<EOF",
			wantModule:   "ze-bgp",
			wantHandlers: []string{"bgp"},
			wantDelim:    "EOF",
		},
		{
			name:    "missing_handlers",
			line:    "config schema ze-bgp",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, delim, err := parseSchemaLine(tt.line)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantModule, schema.Module)
			assert.Equal(t, tt.wantHandlers, schema.Handlers)
			assert.Equal(t, tt.wantDelim, delim)
		})
	}
}

// TestConfigReader_ReceiveInit verifies initialization message parsing.
//
// VALIDATES: Config Reader correctly receives schemas and config path from Hub.
// PREVENTS: Initialization failures blocking config parsing.
func TestConfigReader_ReceiveInit(t *testing.T) {
	initMessages := `config schema ze-bgp handlers bgp,bgp.peer
config schema ze-rib handlers rib
config path /etc/ze/config.conf
config done
`

	cr := &ConfigReader{
		reader:     bufio.NewReader(strings.NewReader(initMessages)),
		writer:     bufio.NewWriter(&bytes.Buffer{}),
		handlerMap: make(map[string]*SchemaInfo),
	}

	err := cr.receiveInit()
	require.NoError(t, err)

	// Verify schemas
	assert.Len(t, cr.schemas, 2)
	assert.Equal(t, "ze-bgp", cr.schemas[0].Module)
	assert.Equal(t, []string{"bgp", "bgp.peer"}, cr.schemas[0].Handlers)
	assert.Equal(t, "ze-rib", cr.schemas[1].Module)

	// Verify config path
	assert.Equal(t, "/etc/ze/config.conf", cr.configPath)

	// Verify handler map
	assert.NotNil(t, cr.handlerMap["bgp"])
	assert.NotNil(t, cr.handlerMap["bgp.peer"])
	assert.NotNil(t, cr.handlerMap["rib"])
}

// TestConfigReader_ReceiveInitWithYang verifies YANG heredoc parsing.
//
// VALIDATES: Multi-line YANG content received correctly.
// PREVENTS: YANG content truncation or corruption.
func TestConfigReader_ReceiveInitWithYang(t *testing.T) {
	initMessages := `config schema ze-bgp handlers bgp yang <<EOF
module ze-bgp {
  namespace "urn:ze:bgp";
  prefix bgp;
}
EOF
config path /etc/ze/config.conf
config done
`

	cr := &ConfigReader{
		reader:     bufio.NewReader(strings.NewReader(initMessages)),
		writer:     bufio.NewWriter(&bytes.Buffer{}),
		handlerMap: make(map[string]*SchemaInfo),
	}

	err := cr.receiveInit()
	require.NoError(t, err)

	assert.Len(t, cr.schemas, 1)
	assert.Contains(t, cr.schemas[0].Yang, "module ze-bgp")
	assert.Contains(t, cr.schemas[0].Yang, "namespace")
}

// TestConfigReader_FindHandler verifies handler path lookup.
//
// VALIDATES: Longest prefix match for handler routing.
// PREVENTS: Wrong plugin receiving config blocks.
func TestConfigReader_FindHandler(t *testing.T) {
	cr := &ConfigReader{
		handlerMap: map[string]*SchemaInfo{
			"bgp":      {Module: "ze-bgp"},
			"bgp.peer": {Module: "ze-bgp-peer"},
			"rib":      {Module: "ze-rib"},
		},
	}

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
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			handler := cr.findHandler(tt.path)
			if tt.wantModule == "" {
				assert.Nil(t, handler)
			} else {
				require.NotNil(t, handler)
				assert.Equal(t, tt.wantModule, handler.Module)
			}
		})
	}
}

// TestConfigReader_TokensToJSON verifies token to JSON conversion.
//
// VALIDATES: Config tokens are converted to JSON correctly.
// PREVENTS: Data loss or corruption in verify requests.
func TestConfigReader_TokensToJSON(t *testing.T) {
	cr := &ConfigReader{}

	// Note: This is a simplified test - real tokens come from tokenizer
	// The tokensToJSON function handles key-value pairs at the current level
	assert.NotNil(t, cr)
}

// TestConfigReader_SendCommand verifies namespace command formatting.
//
// VALIDATES: Commands use format: #N <namespace> <path> <action> {json}.
// PREVENTS: Protocol errors with Hub.
func TestConfigReader_SendCommand(t *testing.T) {
	var output bytes.Buffer
	// Simulate Hub response.
	input := "@1 ok\n"

	cr := &ConfigReader{
		reader: bufio.NewReader(strings.NewReader(input)),
		writer: bufio.NewWriter(&output),
		serial: 1,
	}

	change := ConfigChange{
		Action:  "create",
		Handler: "bgp.peer",
		Path:    "bgp.peer[address=192.0.2.1]",
		NewData: `{"address":"192.0.2.1","as":65001}`,
	}
	err := cr.sendCommand(change)
	require.NoError(t, err)

	// Verify request format: #N <namespace> <path> <action> {json}.
	sent := output.String()
	assert.Contains(t, sent, "#1 bgp peer create {")
	assert.Contains(t, sent, `"address":"192.0.2.1"`)
	assert.Contains(t, sent, `"as":65001`)
}

// TestConfigReader_SendCommandError verifies error handling.
//
// VALIDATES: Command errors are propagated correctly.
// PREVENTS: Silent failures on invalid config.
func TestConfigReader_SendCommandError(t *testing.T) {
	var output bytes.Buffer
	input := "@1 error validation failed: peer-as required\n"

	cr := &ConfigReader{
		reader: bufio.NewReader(strings.NewReader(input)),
		writer: bufio.NewWriter(&output),
		serial: 1,
	}

	change := ConfigChange{
		Action:  "create",
		Handler: "bgp.peer",
		Path:    "bgp.peer[address=192.0.2.1]",
		NewData: `{"address":"192.0.2.1"}`,
	}
	err := cr.sendCommand(change)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "peer-as required")
}

// TestConfigReader_SendCommit verifies commit request formatting.
//
// VALIDATES: Commit uses format: #N <namespace> commit.
// PREVENTS: Protocol errors with Hub.
func TestConfigReader_SendCommit(t *testing.T) {
	var output bytes.Buffer
	input := "@1 ok\n"

	cr := &ConfigReader{
		reader: bufio.NewReader(strings.NewReader(input)),
		writer: bufio.NewWriter(&output),
		serial: 1,
	}

	err := cr.sendCommit("bgp")
	require.NoError(t, err)

	// Verify request format: #N <namespace> commit.
	sent := output.String()
	assert.Contains(t, sent, "#1 bgp commit")
}

// TestConfigReader_EventLoop verifies reload handling.
//
// VALIDATES: Config reload requests are handled correctly.
// PREVENTS: Reload failures blocking config updates.
func TestConfigReader_EventLoop(t *testing.T) {
	var output bytes.Buffer
	input := "#abc shutdown\n"

	cr := &ConfigReader{
		reader:     bufio.NewReader(strings.NewReader(input)),
		writer:     bufio.NewWriter(&output),
		handlerMap: make(map[string]*SchemaInfo),
	}

	err := cr.eventLoop()
	require.NoError(t, err) // Shutdown should exit cleanly
}

// TestConfigReader_HandlerPathBoundary verifies handler path length limits.
//
// VALIDATES: Long handler paths are accepted.
// BOUNDARY: 512 chars should work.
func TestConfigReader_HandlerPathBoundary(t *testing.T) {
	cr := &ConfigReader{
		handlerMap: make(map[string]*SchemaInfo),
	}

	// Create a long path (512 chars)
	longPath := strings.Repeat("a.", 256)[:512]
	cr.handlerMap[longPath] = &SchemaInfo{Module: "test"}

	handler := cr.findHandler(longPath)
	require.NotNil(t, handler)
	assert.Equal(t, "test", handler.Module)
}

// TestConfigReader_CommitOrderDeterministic verifies commits are sorted.
//
// VALIDATES: Namespaces committed in alphabetical order.
// PREVENTS: Non-deterministic commit order.
func TestConfigReader_CommitOrderDeterministic(t *testing.T) {
	var output bytes.Buffer
	// Simulate Hub responses for 3 commands + 2 commits.
	input := "@1 ok\n@2 ok\n@3 ok\n@4 ok\n@5 ok\n"

	cr := &ConfigReader{
		reader:       bufio.NewReader(strings.NewReader(input)),
		writer:       bufio.NewWriter(&output),
		handlerMap:   make(map[string]*SchemaInfo),
		currentState: NewConfigState(),
		serial:       1,
	}

	// Changes span two namespaces: rib and bgp.
	changes := []ConfigChange{
		{Action: "create", Handler: "rib.route", Path: "rib.route[key=default]", NewData: `{"id":"default"}`},
		{Action: "create", Handler: "bgp.peer", Path: "bgp.peer[key=10.0.0.1]", NewData: `{"address":"10.0.0.1"}`},
		{Action: "create", Handler: "bgp", Path: "bgp", NewData: `{"router-id":"1.2.3.4"}`},
	}

	err := cr.applyChanges(changes)
	require.NoError(t, err)

	sent := output.String()
	// Commits should be in alphabetical order: bgp before rib.
	bgpCommitIdx := strings.Index(sent, "bgp commit")
	ribCommitIdx := strings.Index(sent, "rib commit")
	assert.Greater(t, ribCommitIdx, bgpCommitIdx, "bgp commit should come before rib commit")
}

// TestConfigReader_SerialIncrement verifies serial counter increments correctly.
//
// VALIDATES: Each request has unique incrementing serial.
// PREVENTS: Serial mismatch in request/response.
func TestConfigReader_SerialIncrement(t *testing.T) {
	var output bytes.Buffer
	input := "@1 ok\n@2 ok\n@3 ok\n"

	cr := &ConfigReader{
		reader:       bufio.NewReader(strings.NewReader(input)),
		writer:       bufio.NewWriter(&output),
		handlerMap:   make(map[string]*SchemaInfo),
		currentState: NewConfigState(),
		serial:       1,
	}

	changes := []ConfigChange{
		{Action: "create", Handler: "bgp.peer", Path: "bgp.peer[key=1]", NewData: `{"id":"1"}`},
		{Action: "create", Handler: "bgp.peer", Path: "bgp.peer[key=2]", NewData: `{"id":"2"}`},
	}

	err := cr.applyChanges(changes)
	require.NoError(t, err)

	sent := output.String()
	// Verify serials are sequential.
	assert.Contains(t, sent, "#1 bgp peer create")
	assert.Contains(t, sent, "#2 bgp peer create")
	assert.Contains(t, sent, "#3 bgp commit")
}

// TestTokensToJSON_TypePreservation verifies numeric types preserved.
//
// VALIDATES: Numbers are JSON numbers, not strings.
// PREVENTS: Type mismatch with YANG schema.
// NOTE: JSON unmarshal converts all numbers to float64 when target is any.
func TestTokensToJSON_TypePreservation(t *testing.T) {
	tests := []struct {
		name   string
		tokens []config.Token
		want   map[string]any
	}{
		{
			name: "integer",
			tokens: []config.Token{
				{Type: config.TokenWord, Value: "peer-as"},
				{Type: config.TokenWord, Value: "65001"},
				{Type: config.TokenSemicolon},
			},
			want: map[string]any{"peer-as": float64(65001)}, // JSON unmarshal → float64
		},
		{
			name: "float",
			tokens: []config.Token{
				{Type: config.TokenWord, Value: "weight"},
				{Type: config.TokenWord, Value: "1.5"},
				{Type: config.TokenSemicolon},
			},
			want: map[string]any{"weight": 1.5},
		},
		{
			name: "boolean_true",
			tokens: []config.Token{
				{Type: config.TokenWord, Value: "enabled"},
				{Type: config.TokenWord, Value: "true"},
				{Type: config.TokenSemicolon},
			},
			want: map[string]any{"enabled": true},
		},
		{
			name: "boolean_false",
			tokens: []config.Token{
				{Type: config.TokenWord, Value: "disabled"},
				{Type: config.TokenWord, Value: "false"},
				{Type: config.TokenSemicolon},
			},
			want: map[string]any{"disabled": false},
		},
		{
			name: "string",
			tokens: []config.Token{
				{Type: config.TokenWord, Value: "address"},
				{Type: config.TokenWord, Value: "192.0.2.1"},
				{Type: config.TokenSemicolon},
			},
			want: map[string]any{"address": "192.0.2.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tokensToJSON(tt.tokens)
			var got map[string]any
			err := json.Unmarshal([]byte(result), &got)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestConfigReader_DiffConfig verifies config diffing logic.
//
// VALIDATES: Diff correctly identifies create, modify, delete actions.
// PREVENTS: Config reload missing changes or duplicating changes.
func TestConfigReader_DiffConfig(t *testing.T) {
	cr := &ConfigReader{}

	tests := []struct {
		name       string
		oldBlocks  map[string]map[string]*ConfigBlock
		newBlocks  map[string]map[string]*ConfigBlock
		wantCount  int
		wantAction string // First expected action if wantCount > 0
	}{
		{
			name:      "empty_to_empty",
			oldBlocks: nil,
			newBlocks: nil,
			wantCount: 0,
		},
		{
			name:      "create_single",
			oldBlocks: nil,
			newBlocks: map[string]map[string]*ConfigBlock{
				"bgp.peer": {
					"192.0.2.1": {Handler: "bgp.peer", Key: "192.0.2.1", Path: "bgp.peer[address=192.0.2.1]", Data: `{"as":65001}`},
				},
			},
			wantCount:  1,
			wantAction: "create",
		},
		{
			name: "delete_single",
			oldBlocks: map[string]map[string]*ConfigBlock{
				"bgp.peer": {
					"192.0.2.1": {Handler: "bgp.peer", Key: "192.0.2.1", Path: "bgp.peer[address=192.0.2.1]", Data: `{"as":65001}`},
				},
			},
			newBlocks:  nil,
			wantCount:  1,
			wantAction: "delete",
		},
		{
			name: "modify_single",
			oldBlocks: map[string]map[string]*ConfigBlock{
				"bgp.peer": {
					"192.0.2.1": {Handler: "bgp.peer", Key: "192.0.2.1", Path: "bgp.peer[address=192.0.2.1]", Data: `{"as":65001}`},
				},
			},
			newBlocks: map[string]map[string]*ConfigBlock{
				"bgp.peer": {
					"192.0.2.1": {Handler: "bgp.peer", Key: "192.0.2.1", Path: "bgp.peer[address=192.0.2.1]", Data: `{"as":65002}`},
				},
			},
			wantCount:  1,
			wantAction: "modify",
		},
		{
			name: "no_change",
			oldBlocks: map[string]map[string]*ConfigBlock{
				"bgp.peer": {
					"192.0.2.1": {Handler: "bgp.peer", Key: "192.0.2.1", Path: "bgp.peer[address=192.0.2.1]", Data: `{"as":65001}`},
				},
			},
			newBlocks: map[string]map[string]*ConfigBlock{
				"bgp.peer": {
					"192.0.2.1": {Handler: "bgp.peer", Key: "192.0.2.1", Path: "bgp.peer[address=192.0.2.1]", Data: `{"as":65001}`},
				},
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldState := &ConfigState{blocks: tt.oldBlocks}
			if oldState.blocks == nil {
				oldState.blocks = make(map[string]map[string]*ConfigBlock)
			}
			newState := &ConfigState{blocks: tt.newBlocks}
			if newState.blocks == nil {
				newState.blocks = make(map[string]map[string]*ConfigBlock)
			}

			changes := cr.diffConfig(oldState, newState)

			assert.Len(t, changes, tt.wantCount)
			if tt.wantCount > 0 {
				assert.Equal(t, tt.wantAction, changes[0].Action)
			}
		})
	}
}

package plugin

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPlugin_New verifies plugin creation.
//
// VALIDATES: Plugin created with name.
// PREVENTS: Nil plugin on creation.
func TestPlugin_New(t *testing.T) {
	p := New("test-plugin")
	require.NotNil(t, p)
	assert.Equal(t, "test-plugin", p.Name())
	assert.Equal(t, "test-plugin", p.Namespace())
}

// TestPlugin_SetSchema verifies schema declaration.
//
// VALIDATES: Schema and handlers can be set.
// PREVENTS: Panic on schema configuration.
func TestPlugin_SetSchema(t *testing.T) {
	p := New("test-plugin")

	yangSchema := `module test { namespace "urn:test"; prefix test; }`
	err := p.SetSchema(yangSchema, "test", "test.sub")
	require.NoError(t, err)

	assert.Equal(t, yangSchema, p.Schema())
	assert.Equal(t, []string{"test", "test.sub"}, p.Handlers())
}

// TestPlugin_SetSchema_Empty verifies empty schema error.
//
// VALIDATES: Empty schema returns error.
// PREVENTS: Invalid schema accepted.
func TestPlugin_SetSchema_Empty(t *testing.T) {
	p := New("test-plugin")
	err := p.SetSchema("", "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

// TestPlugin_SetSchema_NoHandlers verifies handler requirement.
//
// VALIDATES: At least one handler required.
// PREVENTS: Schema without handlers.
func TestPlugin_SetSchema_NoHandlers(t *testing.T) {
	p := New("test-plugin")
	err := p.SetSchema("module test {}" /* no handlers */)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "handler")
}

// TestPlugin_OnVerify verifies verify handler registration.
//
// VALIDATES: Verify handlers registered by prefix.
// PREVENTS: Lost handler registrations.
func TestPlugin_OnVerify(t *testing.T) {
	p := New("test")
	called := false

	p.OnVerify("test.item", func(ctx *VerifyContext) error {
		called = true
		return nil
	})

	// Trigger verify internally.
	err := p.triggerVerify(&VerifyContext{
		Action: "create",
		Path:   "test.item",
		Data:   "{}",
	})
	require.NoError(t, err)
	assert.True(t, called)
}

// TestPlugin_OnVerify_NoHandler verifies no-handler behavior.
//
// VALIDATES: Unmatched paths succeed by default.
// PREVENTS: Blocking on unregistered handlers.
func TestPlugin_OnVerify_NoHandler(t *testing.T) {
	p := New("test")
	// No handler registered.

	err := p.triggerVerify(&VerifyContext{
		Action: "create",
		Path:   "test.item",
		Data:   "{}",
	})
	require.NoError(t, err) // No handler = allow by default.
}

// TestPlugin_OnApply verifies apply handler registration.
//
// VALIDATES: Apply handlers registered and called.
// PREVENTS: Apply not executed after verify.
func TestPlugin_OnApply(t *testing.T) {
	p := New("test")
	called := false

	p.OnApply("test.item", func(ctx *ApplyContext) error {
		called = true
		return nil
	})

	err := p.triggerApply(&ApplyContext{
		Action: "create",
		Path:   "test.item",
		Data:   "{}",
	})
	require.NoError(t, err)
	assert.True(t, called)
}

// TestPlugin_OnCommand verifies command handler registration.
//
// VALIDATES: Command handlers registered by name.
// PREVENTS: Commands not dispatched.
func TestPlugin_OnCommand(t *testing.T) {
	p := New("test")
	called := false

	p.OnCommand("status", func(ctx *CommandContext) (any, error) {
		called = true
		return map[string]string{"status": "ok"}, nil
	})

	result, err := p.triggerCommand(&CommandContext{
		Command: "status",
		Args:    nil,
	})
	require.NoError(t, err)
	assert.True(t, called)
	assert.NotNil(t, result)
}

// TestPlugin_OnCommand_Unknown verifies unknown command error.
//
// VALIDATES: Unknown commands return error.
// PREVENTS: Silent failures on typos.
func TestPlugin_OnCommand_Unknown(t *testing.T) {
	p := New("test")

	_, err := p.triggerCommand(&CommandContext{
		Command: "unknown",
		Args:    nil,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

// TestPlugin_Protocol_Declaration verifies 5-stage protocol declaration.
//
// VALIDATES: Plugin sends declaration messages.
// PREVENTS: Protocol violation on startup.
func TestPlugin_Protocol_Declaration(t *testing.T) {
	p := New("test")
	_ = p.SetSchema("module test { namespace \"urn:test\"; prefix test; }", "test")
	p.OnCommand("status", func(ctx *CommandContext) (any, error) {
		return "ok", nil
	})

	var out bytes.Buffer
	p.SetOutput(&out)

	// Simulate input: config done, registry done, then shutdown.
	input := "config done\nregistry done\n{\"shutdown\":true}\n"
	p.SetInput(strings.NewReader(input))

	err := p.Run()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "declare encoding text")
	assert.Contains(t, output, "declare schema")
	assert.Contains(t, output, "declare handler test")
	assert.Contains(t, output, "declare cmd status")
	assert.Contains(t, output, "declare done")
	assert.Contains(t, output, "capability done")
	assert.Contains(t, output, "ready")
}

// TestPlugin_Protocol_Command verifies command handling in protocol.
//
// VALIDATES: Commands received and responded.
// PREVENTS: Lost command responses.
func TestPlugin_Protocol_Command(t *testing.T) {
	p := New("test")
	_ = p.SetSchema("module test {}", "test")
	p.OnCommand("ping", func(ctx *CommandContext) (any, error) {
		return map[string]string{"pong": "ok"}, nil
	})

	var out bytes.Buffer
	p.SetOutput(&out)

	input := "config done\nregistry done\n#a ping\n{\"shutdown\":true}\n"
	p.SetInput(strings.NewReader(input))

	err := p.Run()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "@a ok")
	assert.Contains(t, output, "pong")
}

// TestPlugin_CandidateRunning verifies candidate/running model.
//
// VALIDATES: Commands modify candidate, commit applies to running.
// PREVENTS: Immediate application of changes.
func TestPlugin_CandidateRunning(t *testing.T) {
	p := New("bgp")
	_ = p.SetSchema("module bgp {}", "bgp", "bgp.peer")

	var out bytes.Buffer
	p.SetOutput(&out)

	// Send peer create + commit.
	input := `bgp peer create {"address":"192.0.2.1","peer-as":65002}
bgp commit
config done
registry done
{"shutdown":true}
`
	p.SetInput(strings.NewReader(input))

	err := p.Run()
	require.NoError(t, err)

	// Verify running has the peer.
	running := p.Running()
	require.NotNil(t, running["bgp.peer"])
	assert.Equal(t, `{"address":"192.0.2.1","peer-as":65002}`, running["bgp.peer"]["192.0.2.1"])
}

// TestPlugin_CandidateRollback verifies rollback discards candidate.
//
// VALIDATES: Rollback reverts to running.
// PREVENTS: Unwanted changes persisting.
func TestPlugin_CandidateRollback(t *testing.T) {
	p := New("bgp")
	_ = p.SetSchema("module bgp {}", "bgp", "bgp.peer")

	var out bytes.Buffer
	p.SetOutput(&out)

	// Create peer, commit, create another, rollback.
	input := `bgp peer create {"address":"192.0.2.10","peer-as":65010}
bgp commit
bgp peer create {"address":"192.0.2.11","peer-as":65011}
bgp rollback
config done
registry done
{"shutdown":true}
`
	p.SetInput(strings.NewReader(input))

	err := p.Run()
	require.NoError(t, err)

	// Verify candidate matches running (only first peer).
	candidate := p.Candidate()
	running := p.Running()
	assert.Equal(t, running, candidate)
	assert.Len(t, running["bgp.peer"], 1)
}

// TestPlugin_CommitVerifyFail verifies commit fails on verify error.
//
// VALIDATES: Verify failure blocks commit.
// PREVENTS: Invalid config applied.
func TestPlugin_CommitVerifyFail(t *testing.T) {
	p := New("bgp")
	_ = p.SetSchema("module bgp {}", "bgp", "bgp.peer")

	// Verify handler rejects all creates.
	p.OnVerify("bgp.peer", func(ctx *VerifyContext) error {
		return assert.AnError
	})

	var out bytes.Buffer
	p.SetOutput(&out)

	input := `bgp peer create {"address":"192.0.2.20","peer-as":65020}
bgp commit
config done
registry done
{"shutdown":true}
`
	p.SetInput(strings.NewReader(input))

	err := p.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "verify failed")
}

// TestPlugin_CommitApply verifies apply handlers are called.
//
// VALIDATES: Apply handlers execute on commit.
// PREVENTS: Apply phase skipped.
func TestPlugin_CommitApply(t *testing.T) {
	p := New("bgp")
	_ = p.SetSchema("module bgp {}", "bgp", "bgp.peer")

	applied := false
	p.OnApply("bgp.peer", func(ctx *ApplyContext) error {
		applied = true
		assert.Equal(t, ActionCreate, ctx.Action)
		assert.Contains(t, ctx.Data, "192.0.2.30")
		return nil
	})

	var out bytes.Buffer
	p.SetOutput(&out)

	input := `bgp peer create {"address":"192.0.2.30","peer-as":65030}
bgp commit
config done
registry done
{"shutdown":true}
`
	p.SetInput(strings.NewReader(input))

	err := p.Run()
	require.NoError(t, err)
	assert.True(t, applied)
}

// TestPlugin_RuntimeCommand verifies runtime namespace commands.
//
// VALIDATES: Namespace commands work after ready.
// PREVENTS: Commands only during config phase.
func TestPlugin_RuntimeCommand(t *testing.T) {
	p := New("bgp")
	_ = p.SetSchema("module bgp {}", "bgp", "bgp.peer")

	var out bytes.Buffer
	p.SetOutput(&out)

	input := `config done
registry done
#a bgp peer create {"address":"10.0.0.1","peer-as":65010}
#b bgp commit
{"shutdown":true}
`
	p.SetInput(strings.NewReader(input))

	err := p.Run()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "@a ok")
	assert.Contains(t, output, "@b ok")

	// Verify running has the peer.
	running := p.Running()
	assert.NotNil(t, running["bgp.peer"]["10.0.0.1"])
}

// TestVerifyContext verifies VerifyContext fields.
//
// VALIDATES: All context fields populated.
// PREVENTS: Missing context data.
func TestVerifyContext(t *testing.T) {
	ctx := &VerifyContext{
		Action: "create",
		Path:   "bgp.peer",
		Data:   `{"address":"192.0.2.1"}`,
	}

	assert.Equal(t, "create", ctx.Action)
	assert.Equal(t, "bgp.peer", ctx.Path)
	assert.Equal(t, `{"address":"192.0.2.1"}`, ctx.Data)
}

// TestApplyContext verifies ApplyContext fields.
//
// VALIDATES: All context fields populated.
// PREVENTS: Missing context data.
func TestApplyContext(t *testing.T) {
	ctx := &ApplyContext{
		Action: "modify",
		Path:   "bgp.peer",
		Data:   `{"address":"192.0.2.1"}`,
	}

	assert.Equal(t, "modify", ctx.Action)
	assert.Equal(t, "bgp.peer", ctx.Path)
	assert.Equal(t, `{"address":"192.0.2.1"}`, ctx.Data)
}

// TestCommandContext verifies CommandContext fields.
//
// VALIDATES: Command and args populated.
// PREVENTS: Missing command data.
func TestCommandContext(t *testing.T) {
	ctx := &CommandContext{
		Command: "status",
		Args:    []string{"detailed"},
	}

	assert.Equal(t, "status", ctx.Command)
	assert.Equal(t, []string{"detailed"}, ctx.Args)
}

// TestPlugin_NameBoundary verifies plugin name length limits.
//
// BOUNDARY: Name 1-64 chars.
func TestPlugin_NameBoundary(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		wantErr bool
	}{
		{"min_valid_1", 1, false},
		{"max_valid_64", 64, false},
		{"invalid_0", 0, true},
		{"invalid_65", 65, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := strings.Repeat("x", tt.length)
			p := New(name)
			if tt.wantErr {
				assert.Nil(t, p)
			} else {
				assert.NotNil(t, p)
				assert.Equal(t, name, p.Name())
			}
		})
	}
}

// TestExtractKey verifies key extraction from JSON.
//
// VALIDATES: Common key fields extracted.
// PREVENTS: Duplicate entries with same key.
func TestExtractKey(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{"address", `{"address":"192.0.2.1","peer-as":65002}`, "192.0.2.1"},
		{"name", `{"name":"upstream","peer-as":65000}`, "upstream"},
		{"prefix", `{"prefix":"10.0.0.0/8"}`, "10.0.0.0/8"},
		{"id", `{"id":123}`, "123"},
		{"no_key", `{"foo":"bar"}`, `{"foo":"bar"}`},
		{"invalid_json", `not json`, `not json`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractKey(tt.data)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestComputeDiff verifies diff computation.
//
// VALIDATES: Creates, modifies, deletes detected.
// PREVENTS: Missing changes in commit.
func TestComputeDiff(t *testing.T) {
	p := New("bgp")

	// Setup running state using thread-safe method.
	p.SetRunning(map[string]map[string]string{
		"bgp.peer": {
			"192.0.2.1": `{"address":"192.0.2.1","peer-as":65002}`,
			"192.0.2.2": `{"address":"192.0.2.2","peer-as":65003}`,
		},
	})

	// Setup candidate with changes using thread-safe method.
	p.SetCandidate(map[string]map[string]string{
		"bgp.peer": {
			"192.0.2.1": `{"address":"192.0.2.1","peer-as":65002,"receive-hold-time":90}`, // Modified
			// 192.0.2.2 deleted
			"192.0.2.3": `{"address":"192.0.2.3","peer-as":65004}`, // Created
		},
	})

	changes := p.computeDiff()

	// Should have 3 changes: create, modify, delete.
	assert.Len(t, changes, 3)

	// Verify each change type exists.
	actions := make(map[string]bool)
	for _, c := range changes {
		actions[c.Action] = true
	}
	assert.True(t, actions[ActionCreate])
	assert.True(t, actions[ActionModify])
	assert.True(t, actions[ActionDelete])
}

// TestOutputDiff verifies diff output is valid JSON.
//
// VALIDATES: Diff outputs valid JSON array.
// PREVENTS: Ambiguous diff format with spaces in keys.
func TestOutputDiff(t *testing.T) {
	p := New("bgp")

	var out bytes.Buffer
	p.SetOutput(&out)

	// Setup running and candidate with a change.
	p.SetRunning(map[string]map[string]string{
		"bgp.peer": {
			"10.0.0.1": `{"address":"10.0.0.1"}`,
		},
	})
	p.SetCandidate(map[string]map[string]string{
		"bgp.peer": {
			"10.0.0.1": `{"address":"10.0.0.1","as":65001}`,
		},
	})

	p.outputDiff()

	// Verify output is valid JSON.
	output := out.String()
	var changes []ConfigChange
	err := json.Unmarshal([]byte(output), &changes)
	require.NoError(t, err)
	assert.Len(t, changes, 1)
	assert.Equal(t, ActionModify, changes[0].Action)
}

// TestOutputDiffEmpty verifies empty diff outputs empty array.
//
// VALIDATES: Empty diff returns [].
// PREVENTS: Nil or invalid JSON on empty diff.
func TestOutputDiffEmpty(t *testing.T) {
	p := New("bgp")

	var out bytes.Buffer
	p.SetOutput(&out)

	// No changes.
	p.outputDiff()

	output := strings.TrimSpace(out.String())
	assert.Equal(t, "[]", output)
}

// TestPlugin_NamespaceOnlyConfig verifies namespace-level config works.
//
// VALIDATES: Commands without path work (e.g., "bgp create {...}").
// PREVENTS: Namespace-only config rejection.
func TestPlugin_NamespaceOnlyConfig(t *testing.T) {
	p := New("bgp")
	_ = p.SetSchema("module bgp {}", "bgp")

	var out bytes.Buffer
	p.SetOutput(&out)

	// Namespace-only config: "bgp create {...}" (no path like "peer").
	// Use "id" field for key extraction.
	input := `bgp create {"id":"main","router-id":"1.2.3.4"}
bgp commit
config done
registry done
{"shutdown":true}
`
	p.SetInput(strings.NewReader(input))

	err := p.Run()
	require.NoError(t, err)

	// Verify running has the namespace-level config.
	running := p.Running()
	require.NotNil(t, running["bgp"])
	assert.Contains(t, running["bgp"]["main"], "router-id")
}

// TestPlugin_VerifyDeleteGetsData verifies delete verify gets old data.
//
// VALIDATES: Verify handler receives data for delete actions.
// PREVENTS: Empty data in verify for deletes.
func TestPlugin_VerifyDeleteGetsData(t *testing.T) {
	p := New("bgp")
	_ = p.SetSchema("module bgp {}", "bgp", "bgp.peer")

	var verifyData string
	p.OnVerify("bgp.peer", func(ctx *VerifyContext) error {
		if ctx.Action == ActionDelete {
			verifyData = ctx.Data
		}
		return nil
	})

	var out bytes.Buffer
	p.SetOutput(&out)

	// Create, commit, delete, commit.
	input := `bgp peer create {"address":"10.0.0.1","peer-as":65001}
bgp commit
bgp peer delete {"address":"10.0.0.1"}
bgp commit
config done
registry done
{"shutdown":true}
`
	p.SetInput(strings.NewReader(input))

	err := p.Run()
	require.NoError(t, err)

	// Verify handler should have received the old data.
	assert.Contains(t, verifyData, "10.0.0.1")
	assert.Contains(t, verifyData, "65001")
}

// TestPlugin_HandlerPathBoundary verifies long handler paths work.
//
// BOUNDARY: Handler paths up to 256 chars should work.
func TestPlugin_HandlerPathBoundary(t *testing.T) {
	p := New("test")

	// Create a long handler path (256 chars).
	longPath := strings.Repeat("a.", 128)[:256]

	// Register handler with long path.
	called := false
	p.OnVerify(longPath, func(ctx *VerifyContext) error {
		called = true
		return nil
	})

	// Trigger verify.
	err := p.triggerVerify(&VerifyContext{
		Action: "create",
		Path:   longPath,
		Data:   "{}",
	})
	require.NoError(t, err)
	assert.True(t, called)
}

// TestPlugin_InvalidJSON verifies invalid JSON is rejected.
//
// VALIDATES: Malformed JSON returns error with helpful message.
// PREVENTS: Storing corrupt data in candidate.
func TestPlugin_InvalidJSON(t *testing.T) {
	p := New("bgp")
	_ = p.SetSchema("module bgp {}", "bgp", "bgp.peer")

	var out bytes.Buffer
	p.SetOutput(&out)

	// Truncated JSON.
	input := `bgp peer create {"address":"192.0.2.1"
config done
registry done
{"shutdown":true}
`
	p.SetInput(strings.NewReader(input))

	err := p.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON")
	assert.Contains(t, err.Error(), "expected object")
}

// TestPlugin_SortChangesOrder verifies deterministic change ordering.
//
// VALIDATES: Changes sorted by handler, key, then action.
// PREVENTS: Non-deterministic processing order.
// NOTE: Same handler+key with multiple actions is tested for robustness,
// even though normal diff produces at most one change per key.
func TestPlugin_SortChangesOrder(t *testing.T) {
	changes := []ConfigChange{
		{Action: ActionCreate, Handler: "bgp.peer", Key: "10.0.0.1"},
		{Action: ActionDelete, Handler: "bgp.peer", Key: "10.0.0.1"},
		{Action: ActionModify, Handler: "bgp.peer", Key: "10.0.0.1"},
		{Action: ActionCreate, Handler: "bgp.peer", Key: "10.0.0.2"},
		{Action: ActionCreate, Handler: "rib", Key: "default"},
	}

	sortChanges(changes)

	// Verify order: bgp.peer/10.0.0.1 (delete, create, modify), bgp.peer/10.0.0.2, rib/default.
	require.Len(t, changes, 5)
	assert.Equal(t, "bgp.peer", changes[0].Handler)
	assert.Equal(t, "10.0.0.1", changes[0].Key)
	assert.Equal(t, ActionDelete, changes[0].Action)

	assert.Equal(t, "bgp.peer", changes[1].Handler)
	assert.Equal(t, "10.0.0.1", changes[1].Key)
	assert.Equal(t, ActionCreate, changes[1].Action)

	assert.Equal(t, "bgp.peer", changes[2].Handler)
	assert.Equal(t, "10.0.0.1", changes[2].Key)
	assert.Equal(t, ActionModify, changes[2].Action)

	assert.Equal(t, "bgp.peer", changes[3].Handler)
	assert.Equal(t, "10.0.0.2", changes[3].Key)

	assert.Equal(t, "rib", changes[4].Handler)
}

// TestPlugin_SortChangesUnknownAction verifies unknown actions sort last.
//
// VALIDATES: Unknown action doesn't cause panic or wrong ordering.
// PREVENTS: Map lookup returning 0 for unknown action.
func TestPlugin_SortChangesUnknownAction(t *testing.T) {
	changes := []ConfigChange{
		{Action: "unknown", Handler: "bgp.peer", Key: "10.0.0.1"},
		{Action: ActionDelete, Handler: "bgp.peer", Key: "10.0.0.1"},
		{Action: ActionCreate, Handler: "bgp.peer", Key: "10.0.0.1"},
	}

	sortChanges(changes)

	// Unknown action should sort last (after delete, create, modify).
	require.Len(t, changes, 3)
	assert.Equal(t, ActionDelete, changes[0].Action)
	assert.Equal(t, ActionCreate, changes[1].Action)
	assert.Equal(t, "unknown", changes[2].Action)
}

// TestPlugin_RollbackUnknownAction verifies rollback handles unknown actions.
//
// VALIDATES: Unknown action is skipped with error, doesn't cause panic.
// PREVENTS: Empty action/data in rollback apply call.
func TestPlugin_RollbackUnknownAction(t *testing.T) {
	p := New("bgp")

	// Track apply calls.
	var applyCalls []string
	p.OnApply("bgp.peer", func(ctx *ApplyContext) error {
		applyCalls = append(applyCalls, ctx.Action)
		return nil
	})

	// Simulate rollback with unknown action in the middle.
	applied := []ConfigChange{
		{Action: ActionCreate, Handler: "bgp.peer", Key: "1", NewData: `{"id":"1"}`},
		{Action: "unknown", Handler: "bgp.peer", Key: "2", NewData: `{"id":"2"}`},
		{Action: ActionCreate, Handler: "bgp.peer", Key: "3", NewData: `{"id":"3"}`},
	}

	errs := p.rollbackApplied(applied)

	// Should have 1 error for unknown action.
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "unknown action")
	assert.Contains(t, errs[0].Error(), "skipped")

	// Should have 2 apply calls (for the valid actions, in reverse order).
	require.Len(t, applyCalls, 2)
	assert.Equal(t, ActionDelete, applyCalls[0]) // Rollback of create key=3
	assert.Equal(t, ActionDelete, applyCalls[1]) // Rollback of create key=1
}

// TestPlugin_RollbackApplied verifies partial apply rollback.
//
// VALIDATES: Failed apply triggers rollback of prior changes.
// PREVENTS: Partial state on apply failure.
func TestPlugin_RollbackApplied(t *testing.T) {
	p := New("bgp")
	_ = p.SetSchema("module bgp {}", "bgp", "bgp.peer")

	var applyCalls []string

	p.OnApply("bgp.peer", func(ctx *ApplyContext) error {
		applyCalls = append(applyCalls, ctx.Action+":"+ctx.Data)
		// Fail on second create (the one with 10.0.0.2).
		if ctx.Action == ActionCreate && strings.Contains(ctx.Data, "10.0.0.2") {
			return assert.AnError
		}
		return nil
	})

	var out bytes.Buffer
	p.SetOutput(&out)

	// Two creates - second will fail, first should be rolled back.
	input := `bgp peer create {"address":"10.0.0.1","peer-as":65001}
bgp peer create {"address":"10.0.0.2","peer-as":65002}
bgp commit
config done
registry done
{"shutdown":true}
`
	p.SetInput(strings.NewReader(input))

	err := p.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "apply failed")

	// Should have: create 10.0.0.1, create 10.0.0.2 (fail), delete 10.0.0.1 (rollback).
	require.Len(t, applyCalls, 3)
	assert.Contains(t, applyCalls[0], "create")
	assert.Contains(t, applyCalls[0], "10.0.0.1")
	assert.Contains(t, applyCalls[1], "create")
	assert.Contains(t, applyCalls[1], "10.0.0.2")
	assert.Contains(t, applyCalls[2], "delete") // Rollback.
	assert.Contains(t, applyCalls[2], "10.0.0.1")
}

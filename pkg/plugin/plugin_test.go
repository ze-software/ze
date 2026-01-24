package plugin

import (
	"bytes"
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
	p := New("test-plugin")
	called := false

	p.OnVerify("test", func(ctx *VerifyContext) error {
		called = true
		return nil
	})

	// Trigger verify internally
	err := p.triggerVerify(&VerifyContext{
		Action: "create",
		Path:   "test.item",
		Data:   "{}",
	})
	require.NoError(t, err)
	assert.True(t, called)
}

// TestPlugin_OnVerify_NoMatch verifies no-match behavior.
//
// VALIDATES: Unmatched paths return error.
// PREVENTS: Silent acceptance of unknown paths.
func TestPlugin_OnVerify_NoMatch(t *testing.T) {
	p := New("test-plugin")
	p.OnVerify("other", func(ctx *VerifyContext) error {
		return nil
	})

	err := p.triggerVerify(&VerifyContext{
		Action: "create",
		Path:   "test.item",
		Data:   "{}",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no handler")
}

// TestPlugin_OnApply verifies apply handler registration.
//
// VALIDATES: Apply handlers registered and called.
// PREVENTS: Apply not executed after verify.
func TestPlugin_OnApply(t *testing.T) {
	p := New("test-plugin")
	called := false

	p.OnApply("test", func(ctx *ApplyContext) error {
		called = true
		return nil
	})

	err := p.triggerApply(&ApplyContext{
		Action: "create",
		Path:   "test.item",
	})
	require.NoError(t, err)
	assert.True(t, called)
}

// TestPlugin_OnCommand verifies command handler registration.
//
// VALIDATES: Command handlers registered by name.
// PREVENTS: Commands not dispatched.
func TestPlugin_OnCommand(t *testing.T) {
	p := New("test-plugin")
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
	p := New("test-plugin")

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
	p := New("test-plugin")
	_ = p.SetSchema("module test { namespace \"urn:test\"; prefix test; }", "test")
	p.OnCommand("status", func(ctx *CommandContext) (any, error) {
		return "ok", nil
	})

	// Capture output
	var out bytes.Buffer
	p.SetOutput(&out)

	// Simulate input: config done, registry done, then shutdown
	input := "config done\nregistry done\n{\"shutdown\":true}\n"
	p.SetInput(strings.NewReader(input))

	// Run protocol (will exit on shutdown)
	err := p.Run()
	require.NoError(t, err)

	// Verify declaration messages
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
	p := New("test-plugin")
	_ = p.SetSchema("module test {}", "test")
	p.OnCommand("ping", func(ctx *CommandContext) (any, error) {
		return map[string]string{"pong": "ok"}, nil
	})

	var out bytes.Buffer
	p.SetOutput(&out)

	// Protocol: config done, registry done, command, shutdown
	input := "config done\nregistry done\n#a ping\n{\"shutdown\":true}\n"
	p.SetInput(strings.NewReader(input))

	err := p.Run()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "@a ok")
	assert.Contains(t, output, "pong")
}

// TestPlugin_Protocol_Verify verifies verify handling in protocol.
//
// VALIDATES: Verify requests handled.
// PREVENTS: Config verification skipped.
func TestPlugin_Protocol_Verify(t *testing.T) {
	p := New("test-plugin")
	_ = p.SetSchema("module test {}", "test")
	p.OnVerify("test", func(ctx *VerifyContext) error {
		if ctx.Action != "create" {
			return assert.AnError
		}
		return nil
	})

	var out bytes.Buffer
	p.SetOutput(&out)

	// Protocol: config done (with verify), registry done, shutdown
	input := "config verify action create path \"test\" data '{}'\nconfig done\nregistry done\n{\"shutdown\":true}\n"
	p.SetInput(strings.NewReader(input))

	err := p.Run()
	require.NoError(t, err)

	output := out.String()
	// Verify should have succeeded
	assert.NotContains(t, output, "error")
}

// TestVerifyContext verifies VerifyContext fields.
//
// VALIDATES: All context fields populated.
// PREVENTS: Missing context data.
func TestVerifyContext(t *testing.T) {
	ctx := &VerifyContext{
		Action: "create",
		Path:   "test.item[id=1]",
		Data:   `{"value":"test"}`,
	}

	assert.Equal(t, "create", ctx.Action)
	assert.Equal(t, "test.item[id=1]", ctx.Path)
	assert.Equal(t, `{"value":"test"}`, ctx.Data)
}

// TestApplyContext verifies ApplyContext fields.
//
// VALIDATES: All context fields populated.
// PREVENTS: Missing context data.
func TestApplyContext(t *testing.T) {
	ctx := &ApplyContext{
		Action: "modify",
		Path:   "test.item[id=1]",
	}

	assert.Equal(t, "modify", ctx.Action)
	assert.Equal(t, "test.item[id=1]", ctx.Path)
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

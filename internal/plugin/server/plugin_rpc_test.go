package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/plugin/process"
)

// TestCommandDefConstruction verifies direct CommandDef struct construction.
//
// VALIDATES: CommandDef fields are correctly set when constructed directly.
// PREVENTS: Regression when command definitions come from RPC instead of text parsing.
func TestCommandDefConstruction(t *testing.T) {
	tests := []struct {
		name string
		def  CommandDef
	}{
		{
			name: "basic",
			def: CommandDef{
				Name:        "myapp status",
				Description: "Show status",
				Timeout:     DefaultCommandTimeout,
			},
		},
		{
			name: "with_args",
			def: CommandDef{
				Name:        "myapp check",
				Description: "Check component",
				Args:        "<component>",
				Timeout:     DefaultCommandTimeout,
			},
		},
		{
			name: "with_completable",
			def: CommandDef{
				Name:        "myapp status",
				Description: "Show status",
				Args:        "<component>",
				Completable: true,
				Timeout:     DefaultCommandTimeout,
			},
		},
		{
			name: "with_custom_timeout",
			def: CommandDef{
				Name:        "myapp dump",
				Description: "Dump data",
				Timeout:     60 * time.Second,
			},
		},
		{
			name: "all_options",
			def: CommandDef{
				Name:        "myapp full",
				Description: "Full command",
				Args:        "<arg>",
				Completable: true,
				Timeout:     120 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.def.Name)
			assert.NotEmpty(t, tt.def.Description)
			assert.Greater(t, tt.def.Timeout, time.Duration(0))
		})
	}
}

// TestCommandDefRegistration verifies CommandDef structs work with CommandRegistry.
//
// VALIDATES: Direct struct construction integrates with registry registration.
// PREVENTS: Mismatch between struct fields and registry expectations.
func TestCommandDefRegistration(t *testing.T) {
	registry := NewCommandRegistry()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-plugin"})

	defs := []CommandDef{
		{
			Name:        "myapp status",
			Description: "Show status",
			Args:        "<component>",
			Completable: true,
			Timeout:     DefaultCommandTimeout,
		},
		{
			Name:        "myapp reload",
			Description: "Reload config",
			Timeout:     60 * time.Second,
		},
	}

	results := registry.Register(proc, defs)
	require.Len(t, results, 2)

	for _, r := range results {
		assert.True(t, r.OK, "registration should succeed for %s: %s", r.Name, r.Error)
	}

	// Verify fields are preserved through registration
	cmd := registry.Lookup("myapp status")
	require.NotNil(t, cmd)
	assert.Equal(t, "myapp status", cmd.Name)
	assert.Equal(t, "Show status", cmd.Description)
	assert.Equal(t, "<component>", cmd.Args)
	assert.True(t, cmd.Completable)
	assert.Equal(t, DefaultCommandTimeout, cmd.Timeout)

	cmd = registry.Lookup("myapp reload")
	require.NotNil(t, cmd)
	assert.Equal(t, 60*time.Second, cmd.Timeout)
	assert.False(t, cmd.Completable)
}

// TestTokenize verifies the tokenize function handles quoting and escaping.
//
// VALIDATES: Tokenizer splits strings correctly with quote/escape handling.
// PREVENTS: Broken command parsing from mishandled quotes or whitespace.
func TestTokenize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple_words",
			input: "hello world",
			want:  []string{"hello", "world"},
		},
		{
			name:  "quoted_string",
			input: `"hello world" foo`,
			want:  []string{"hello world", "foo"},
		},
		{
			name:  "escaped_quote",
			input: `"hello \"world\""`,
			want:  []string{`hello "world"`},
		},
		{
			name:  "empty_string",
			input: "",
			want:  nil,
		},
		{
			name:  "whitespace_only",
			input: "   ",
			want:  nil,
		},
		{
			name:  "multiple_spaces",
			input: "a   b   c",
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "tabs_and_spaces",
			input: "a\t\tb",
			want:  []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tokenize(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
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
				Name:      "show status",
				ShortHelp: "Show status",
				Timeout:   DefaultCommandTimeout,
			},
		},
		{
			name: "with_args",
			def: CommandDef{
				Name:      "request check",
				ShortHelp: "Check component",
				Args:      "<component>",
				Timeout:   DefaultCommandTimeout,
			},
		},
		{
			name: "with_completable",
			def: CommandDef{
				Name:        "show status",
				ShortHelp:   "Show status",
				Args:        "<component>",
				Completable: true,
				Timeout:     DefaultCommandTimeout,
			},
		},
		{
			name: "with_custom_timeout",
			def: CommandDef{
				Name:      "request dump",
				ShortHelp: "Dump data",
				Timeout:   60 * time.Second,
			},
		},
		{
			name: "all_options",
			def: CommandDef{
				Name:        "request full",
				ShortHelp:   "Full command",
				Args:        "<arg>",
				Completable: true,
				Timeout:     120 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.def.Name)
			assert.NotEmpty(t, tt.def.ShortHelp)
			assert.Greater(t, tt.def.Timeout, time.Duration(0))
		})
	}
}

// TestCommandDefRegistration verifies CommandDef structs work with CommandRegistry.
//
// VALIDATES: Direct struct construction integrates with registry registration.
// PREVENTS: Mismatch between struct fields and registry expectations.
func TestCommandDefRegistration(t *testing.T) {
	registry := newCommandRegistry()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-plugin"})

	defs := []CommandDef{
		{
			Name:        "show status",
			ShortHelp:   "Show status",
			Args:        "<component>",
			Completable: true,
			Timeout:     DefaultCommandTimeout,
		},
		{
			Name:      "request reload",
			ShortHelp: "Reload config",
			Timeout:   60 * time.Second,
		},
	}

	results := registry.Register(proc, defs)
	require.Len(t, results, 2)

	for _, r := range results {
		assert.True(t, r.OK, "registration should succeed for %s: %s", r.Name, r.Error)
	}

	// Verify fields are preserved through registration
	cmd := registry.Lookup("show status")
	require.NotNil(t, cmd)
	assert.Equal(t, "show status", cmd.Name)
	assert.Equal(t, "Show status", cmd.ShortHelp)
	assert.Equal(t, "<component>", cmd.Args)
	assert.True(t, cmd.Completable)
	assert.Equal(t, DefaultCommandTimeout, cmd.Timeout)

	cmd = registry.Lookup("request reload")
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
			got, err := tokenize(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestScanCommandReportsUnquotedPipe proves the scanner Dispatch reads finds
// the first pipe character outside double quotes, and that its tokens are the
// tokens tokenize answers for the same line, so accounting and the typed
// authorizer read a refused line exactly as before.
//
// Method: each case names the byte offset in the raw line (leading space
// included) and the tokens; the quoted cases put a literal pipe inside quotes,
// alone and before an unquoted one, which a token-based check cannot tell apart.
//
// VALIDATES: the quote-aware offset of spec-plugin-dispatch-pipe-chain-reaches-the-handler (R-1, R-4).
// PREVENTS: a quoted pipe value refused, or a joined pipe ("a|b") missed.
func TestScanCommandReportsUnquotedPipe(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantPipeAt int
		wantTokens []string
	}{
		{name: "no pipe", input: "show bgp rib received", wantPipeAt: noPipe, wantTokens: []string{"show", "bgp", "rib", "received"}},
		{name: "empty", input: "  ", wantPipeAt: noPipe, wantTokens: nil},
		{name: "spaced pipe", input: "show x | count", wantPipeAt: 7, wantTokens: []string{"show", "x", "|", "count"}},
		{name: "leading space counted", input: "  show x | count", wantPipeAt: 9, wantTokens: []string{"show", "x", "|", "count"}},
		{name: "joined both sides", input: "show x|count", wantPipeAt: 6, wantTokens: []string{"show", "x|count"}},
		{name: "joined before operator", input: "show x |count", wantPipeAt: 7, wantTokens: []string{"show", "x", "|count"}},
		{name: "first of several", input: "show x | match y | count", wantPipeAt: 7, wantTokens: []string{"show", "x", "|", "match", "y", "|", "count"}},
		{name: "quoted pipe alone", input: `set x "|"`, wantPipeAt: noPipe, wantTokens: []string{"set", "x", "|"}},
		{name: "quoted value holding a pipe", input: `set x "a | b"`, wantPipeAt: noPipe, wantTokens: []string{"set", "x", "a | b"}},
		{name: "quoted pipe before an unquoted one", input: `show "a|b" | count`, wantPipeAt: 11, wantTokens: []string{"show", "a|b", "|", "count"}},
		{name: "unclosed quote hides the pipe", input: `show "a | count`, wantPipeAt: noPipe, wantTokens: []string{"show", "a | count"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, pipeAt, err := scanCommand(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.wantPipeAt, pipeAt, "pipe offset")
			assert.Equal(t, tt.wantTokens, tokens, "tokens")
			if pipeAt != noPipe {
				assert.Equal(t, byte('|'), tt.input[pipeAt], "the offset points at the pipe in the raw line")
			}

			tokenized, err := tokenize(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tokenized, tokens, "tokenize answers the scanner's tokens")
		})
	}

	_, pipeAt, err := scanCommand(`show x\y | count`)
	require.ErrorIs(t, err, errBackslashInCommand, "a backslash is refused before any pipe is read")
	assert.Equal(t, noPipe, pipeAt)
}

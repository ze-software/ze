// Design: docs/architecture/aaa-tacacs.md -- typed command authorization contract
package tacacs

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// typedLocalAuthz is a typed local authorizer that records what it judged.
type typedLocalAuthz struct {
	allow   bool
	command string
	args    []string
	lines   []string
}

func (f *typedLocalAuthz) Authorize(_, _, line string, _ bool) bool {
	f.lines = append(f.lines, line)
	return f.allow
}

func (f *typedLocalAuthz) AuthorizeCommandArgs(_, _, command string, args []string, _ string, _ bool) bool {
	f.command = command
	f.args = slices.Clone(args)
	return f.allow
}

// stringLocalAuthz is a string-only local authorizer that records the line.
type stringLocalAuthz struct {
	allow bool
	lines []string
}

func (f *stringLocalAuthz) Authorize(_, _, line string, _ bool) bool {
	f.lines = append(f.lines, line)
	return f.allow
}

// TestTacacsFallbackArgsPassesChainTokens proves that, with the TACACS+ server
// unreachable, the local fallback judges the same tokens the server would
// have been sent for a piped line: a typed local authorizer receives the chain
// tokens unchanged, and a string one their canonical form, the pipe tokens and
// a chain backslash kept.
//
// Method: a client with no server, so every request fails over to local.
//
// VALIDATES: AC-27 under T-6 A, AC-38 on the fallback.
// PREVENTS: the fallback judging a line other than the one typed.
func TestTacacsFallbackArgsPassesChainTokens(t *testing.T) {
	chain := []string{"rows", "|", "match", `a\b`, "|", "count"}

	t.Run("typed local", func(t *testing.T) {
		client := NewTacacsClient(TacacsClientConfig{Timeout: 200 * time.Millisecond})
		local := &typedLocalAuthz{allow: true}
		authorizer := newTacacsAuthorizer(client, local)
		assert.True(t, authorizer.AuthorizeCommandArgs("admin", "10.0.0.1:22", "show", chain, "*", true))
		assert.Equal(t, "show", local.command)
		assert.Equal(t, chain, local.args, "the fallback judges the chain tokens unchanged")
		assert.Empty(t, local.lines, "a typed local authorizer is never handed a rebuilt string")
	})

	t.Run("string local", func(t *testing.T) {
		client := NewTacacsClient(TacacsClientConfig{Timeout: 200 * time.Millisecond})
		local := &stringLocalAuthz{allow: true}
		authorizer := newTacacsAuthorizer(client, local)
		assert.True(t, authorizer.AuthorizeCommandArgs("admin", "10.0.0.1:22", "show", chain, "*", true))
		assert.Equal(t, []string{`show rows | match "a\\b" | count`}, local.lines,
			"the canonical form keeps the pipe tokens and quotes the backslash word")
	})
}

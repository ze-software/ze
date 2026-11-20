// Design: docs/architecture/api/commands.md — the pipe chain: declared operations and the residual chain
package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPartitionSameOnSurfaceAndDispatch proves the dispatcher's read of a line
// (ReadDispatchLine) and a rendering surface's (prepareDocumentPipes, which
// ProcessPipesDefaultFormatChecked runs for SSH exec and the web) give the same
// command part, the same declared list and the same residual chain.
//
// Method: one declaring command and a table of lines covering a declared
// prefix, a residual operation, an override after a residual one, a quoted
// value and a refused late operation. The surface appends its default format
// when the chain names none, so the residual is compared without that one.
//
// VALIDATES: A-4 of spec-pipe-chain-honored-on-every-command-surface.
// PREVENTS: SSH and a data caller answering one line differently.
func TestPartitionSameOnSurfaceAndDispatch(t *testing.T) {
	ResetPipeFiltersForTest()
	t.Cleanup(ResetPipeFiltersForTest)
	RegisterPipeFilters([]string{"test rib"},
		PipeFilter{Name: "received", Description: "received routes", Argument: ArgNone},
		PipeFilter{Name: "peer", Description: "routes of one peer", Argument: ArgText},
		PipeFilter{Name: "count", Description: "count the routes", Argument: ArgNone},
	)

	lines := []string{
		"test rib",
		"test rib | received | count",
		`test rib | peer "a b" | count`,
		"test rib | received | match up | count",
		"test rib | match up | peer x",
		"test rib | table",
		"test rib | json | count",
	}
	for _, line := range lines {
		t.Run(line, func(t *testing.T) {
			read, err := ReadDispatchLine(line)
			require.NoError(t, err)
			plan, problem := prepareDocumentPipes(line, "", false)

			assert.Equal(t, plan.command, read.Command(), "the same command part")
			assert.Equal(t, plan.declared, read.DeclaredOperations(), "the same declared list")
			residual := plan.ops
			// The surface appends its default format only to a chain it
			// accepted and that names none.
			if problem == "" && !hasFormatOp(read.part.residual) {
				residual = residual[:len(residual)-1]
			}
			assert.Equal(t, len(residual), len(read.part.residual), "the same residual chain")
			for i := range residual {
				assert.Equal(t, residual[i].kind, read.part.residual[i].kind)
				assert.Equal(t, residual[i].arg, read.part.residual[i].arg)
			}
		})
	}
}

// TestDispatchLineTypedTokens proves the typed tokens are the command words,
// then each operation as a pipe token, its name and its decoded argument
// words, as typed before any alias expands, with a chain backslash literal.
//
// VALIDATES: AC-27 and AC-38 token shape.
func TestDispatchLineTypedTokens(t *testing.T) {
	read, err := ReadDispatchLine(`show "x y" | match "a b" | match a\b|count`)
	require.NoError(t, err)
	assert.Equal(t, []string{"show", "x y", "|", "match", "a b", "|", "match", `a\b`, "|", "count"}, read.TypedTokens())
	assert.Equal(t, []string{"show", "x y"}, read.Words())

	tokens, err := TypedLineTokens(`show "x y" | match a\b`)
	require.NoError(t, err)
	assert.Equal(t, []string{"show", "x y", "|", "match", `a\b`}, tokens, "the accountant's read is the same")

	_, err = ReadDispatchLine(`show x | match "open`)
	require.Error(t, err, "an unfinished quote in the chain is refused")
}

// TestDispatchLineHasNoAnswerStepWithoutChain proves a line with no chain
// leaves the answer to the handler, so it answers exactly as before (AC-10).
func TestDispatchLineHasNoAnswerStepWithoutChain(t *testing.T) {
	read, err := ReadDispatchLine(`show x "a|b"`)
	require.NoError(t, err)
	assert.False(t, read.HasAnswerStep(), "a quoted pipe is a value, not a chain")
	assert.Equal(t, []string{"show", "x", "a|b"}, read.TypedTokens())

	piped, err := ReadDispatchLine("show x | count")
	require.NoError(t, err)
	assert.True(t, piped.HasAnswerStep())
	assert.NoError(t, piped.ResidualRefusal())
}

// Design: docs/architecture/api/commands.md -- the pipe chain takes the invocation's quote grammar.
// Related: invocation_scan.go -- scanChain, the one reading of the chain

package command

import (
	"slices"
	"strings"
	"testing"
)

// chainOp is what one parsed operation answers a reader: its kind and the text
// a generic operator reads. Rows state these rather than the op's storage.
type chainOp struct {
	kind pipeKind
	arg  string
}

func chainOps(ops []pipeOp) []chainOp {
	answered := make([]chainOp, 0, len(ops))
	for _, op := range ops {
		answered = append(answered, chainOp{kind: op.kind, arg: op.arg})
	}
	return answered
}

// TestSplitPipeLineQuoteAware proves where a line is cut. The first pipe outside
// double quotes ends the command part, a quoted pipe stays inside its value in
// the command part and in the chain, and every further unquoted pipe ends an
// operation, joined to a word or not.
// Method: ParsePipe over each line, comparing the command part and each
// operation's kind and argument text.
// VALIDATES: spec-pipe-chain-honored-on-every-command-surface AC-6, the cut.
// PREVENTS: `| match "error|warn"` torn into `match "error` and `warn"`.
func TestSplitPipeLineQuoteAware(t *testing.T) {
	for _, tc := range []struct {
		name    string
		line    string
		command string
		chain   []chainOp
	}{
		{
			name: "a quoted pipe in the command part", line: `show demo "a|b" | count`,
			command: `show demo "a|b"`, chain: []chainOp{{kind: pipeCount}},
		},
		{
			name: "a quoted pipe in the chain", line: `show log recent | match "error|warn"`,
			command: "show log recent", chain: []chainOp{{kind: pipeMatch, arg: "error|warn"}},
		},
		{
			name: "a quoted pipe between spaces in the chain", line: `show log recent | match "a | b" | count`,
			command: "show log recent", chain: []chainOp{{kind: pipeMatch, arg: "a | b"}, {kind: pipeCount}},
		},
		{
			name: "each unquoted pipe ends an operation", line: `show demo | match up | first 2 | count`,
			command: "show demo", chain: []chainOp{{kind: pipeMatch, arg: "up"}, {kind: pipeFirst, arg: "2"}, {kind: pipeCount}},
		},
		{
			name: "a pipe joined to a word ends the operation", line: `show demo | match up|count`,
			command: "show demo", chain: []chainOp{{kind: pipeMatch, arg: "up"}, {kind: pipeCount}},
		},
		{
			name: "an empty segment is no operation", line: `show demo | | count`,
			command: "show demo", chain: []chainOp{{kind: pipeCount}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command, ops := ParsePipe(tc.line)
			if command != tc.command {
				t.Errorf("command part = %q, want %q", command, tc.command)
			}
			if got := chainOps(ops); !slices.Equal(got, tc.chain) {
				t.Errorf("chain = %+v, want %+v", got, tc.chain)
			}
		})
	}
}

// TestChainGrammar holds one row per rule of the chain grammar
// (docs/architecture/api/commands.md, "The pipe chain takes the invocation's
// quote grammar"), then the two readers that reach the chain by another door:
// a command's own filter handed over in its declared list, and an alias
// expansion.
// Method: ParsePipe for the word rules; partitionPipeChain for the declared
// filter, so the words the handler receives are the ones decoded;
// RegisterAliases and expandAliases for the expansion.
// VALIDATES: spec-pipe-chain-honored-on-every-command-surface AC-6 and D-1.
// PREVENTS: quotes reaching the operation as data, and a second grammar for
// the chain, a fold or an alias.
func TestChainGrammar(t *testing.T) {
	for _, tc := range []struct {
		name  string
		chain string
		want  []chainOp
	}{
		{name: "a quoted run is one word, its quotes removed", chain: `match "link down"`, want: []chainOp{{kind: pipeMatch, arg: "link down"}}},
		{name: "an escaped quote, backslash and tab inside quotes", chain: `match "say \"hi\" \\ a\tb"`, want: []chainOp{{kind: pipeMatch, arg: "say \"hi\" \\ a\tb"}}},
		{name: "an escaped newline and carriage return inside quotes", chain: `match "a\nb\rc"`, want: []chainOp{{kind: pipeMatch, arg: "a\nb\rc"}}},
		{name: "a backslash outside quotes stays literal", chain: `match a\b`, want: []chainOp{{kind: pipeMatch, arg: `a\b`}}},
		{name: "a backslash before a quote outside quotes is no escape", chain: `match a\"b c"`, want: []chainOp{{kind: pipeMatch, arg: `a\b c`}}},
		{name: "a quote in the middle of a word ends no word", chain: `match ab"c d"e`, want: []chainOp{{kind: pipeMatch, arg: "abc de"}}},
		{name: "a single quote is a literal character", chain: `match 'a b'`, want: []chainOp{{kind: pipeMatch, arg: "'a b'"}}},
		{name: "unquoted words join by one space", chain: `match link   down`, want: []chainOp{{kind: pipeMatch, arg: "link down"}}},
		{name: "an undefined escape inside quotes is refused", chain: `match "a\d"`, want: []chainOp{{kind: pipeInvalid, arg: ErrBackslashInInvocation.Error()}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, ops := ParsePipe("show grammar | " + tc.chain)
			if got := chainOps(ops); !slices.Equal(got, tc.want) {
				t.Errorf("chain %s = %+v, want %+v", tc.chain, got, tc.want)
			}
		})
	}

	t.Run("a declared value stays one word of the declared list", func(t *testing.T) {
		ResetPipeFiltersForTest()
		t.Cleanup(ResetPipeFiltersForTest)
		RegisterPipeFilters([]string{"show grammar rows"},
			PipeFilter{Name: "path", Description: "AS path", Argument: ArgText},
		)
		for _, tc := range []struct {
			chain string
			value string
		}{
			{chain: `path "a|b"`, value: "a|b"},
			{chain: `path "65001 65002"`, value: "65001 65002"},
			{chain: `path a\b`, value: `a\b`},
		} {
			part := partitionOf("show grammar rows | " + tc.chain)
			if part.command != "show grammar rows" || len(part.residual) != 0 {
				t.Errorf("partition of %s = %q with %+v left, want the command part unchanged and none left", tc.chain, part.command, chainOps(part.residual))
				continue
			}
			want := []PipeOperation{{Name: "path", Args: []string{tc.value}}}
			if !samePipeOperations(part.declared, want) {
				t.Errorf("the handler would receive %+v for %s, want %+v", part.declared, tc.chain, want)
			}
		}
	})

	t.Run("an alias expansion reads by the same rules", func(t *testing.T) {
		resetAliasTables(t)
		RegisterAliases(nil, Alias{Name: "linkdown", Expansion: `match "link down" | count`})
		_, ops := ParsePipe("show grammar | linkdown")
		got := chainOps(expandAliases("show grammar", ops))
		want := []chainOp{{kind: pipeMatch, arg: "link down"}, {kind: pipeCount}}
		if !slices.Equal(got, want) {
			t.Errorf("expansion = %+v, want %+v", got, want)
		}

		message := refusalMessage(t, func() {
			RegisterAliases(nil, Alias{Name: "open", Expansion: `match "link`})
		})
		requireMentions(t, message, "open", "unfinished quote at column 7")
	})
}

// TestSplitPipeLineUnfinishedQuoteInChain proves the chain half of AC-7: a
// double quote opened after the first pipe and never closed is refused before
// anything runs, naming the opening quote's column in the line as typed and
// the fix, on the scan every execution route reads and on each pipeline
// preparation that reaches it, `ze pipe` included.
// Method: each route over one line whose quote opens at column 19; `ze pipe`
// reads the operators alone, so its column is 7.
// VALIDATES: spec-pipe-chain-honored-on-every-command-surface AC-7.
// PREVENTS: the chain's quote read as a character of its argument.
func TestSplitPipeLineUnfinishedQuoteInChain(t *testing.T) {
	const line = `show demo | match "abc`
	for _, route := range []struct {
		name    string
		column  string
		prepare func() string
	}{
		{"execution scan", "column 19", func() string {
			_, _, err := ScanExecution(line)
			if err == nil {
				return ""
			}
			return err.Error()
		}},
		{"remote one-shot", "column 19", func() string {
			_, _, message := ProcessPipesDefaultFormatChecked(line, "")
			return message
		}},
		{"local one-shot", "column 19", func() string {
			_, _, message := ProcessPipesDefaultFormatLocal(line, "")
			return message
		}},
		{"remote stream", "column 19", func() string {
			_, _, _, message := ProcessRemoteStreamPipes(line, "")
			return message
		}},
		{"ze pipe", "column 7", func() string {
			_, message := ProcessStandalonePipesChecked(`match "abc`)
			return message
		}},
	} {
		t.Run(route.name, func(t *testing.T) {
			message := route.prepare()
			if !strings.Contains(message, "unfinished quote at "+route.column) {
				t.Errorf("refusal = %q, want the unfinished quote at %s", message, route.column)
			}
			if !strings.Contains(message, "Add the closing double quote") {
				t.Errorf("refusal = %q, want the fix", message)
			}
		})
	}
}

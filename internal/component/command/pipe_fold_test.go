package command

import "testing"

// TestFoldPipeChain proves FoldPipeChain answers the folded dispatch line only
// when the whole chain folds into the command's own filters, through the same
// parse, alias expansion and fold the operator surfaces run.
//
// Method: one command path registers count, match and a named filter, and an
// alias that expands to count; every other case is a chain that leaves an
// operator behind (a format, an invalid argument, an unknown filter, a command
// with no filters) and must answer no fold.
//
// VALIDATES: the fold helper of spec-plugin-dispatch-pipe-chain-reaches-the-handler (AC-1 fix line).
// PREVENTS: a refusal suggesting a line that drops part of the chain.
func TestFoldPipeChain(t *testing.T) {
	resetAliasTables(t)

	const rib = "show demo rib"
	RegisterPipeFilters([]string{rib},
		PipeFilter{Name: "count", Description: "count rows"},
		PipeFilter{Name: "match", Description: "keep rows holding text", TakesArg: true},
		PipeFilter{Name: "peer", Description: "rows of one peer", TakesArg: true},
	)
	RegisterAliases([]string{rib}, Alias{Name: "tally", Description: "count the rows", Expansion: "count"})

	tests := []struct {
		name      string
		command   string
		chain     string
		wantLine  string
		wantWhole bool
	}{
		{name: "operator filter", command: rib, chain: " count", wantLine: "show demo rib count", wantWhole: true},
		{name: "named filter then operator", command: rib + " ", chain: " peer 10.0.0.1 | count", wantLine: "show demo rib peer 10.0.0.1 count", wantWhole: true},
		{name: "match with argument", command: rib, chain: "match established", wantLine: "show demo rib match established", wantWhole: true},
		{name: "alias expanding to a filter", command: rib, chain: "tally", wantLine: "show demo rib count", wantWhole: true},
		{name: "empty chain", command: rib, chain: " ", wantLine: rib, wantWhole: true},
		{name: "partial fold leaves a format", command: rib, chain: "count | json"},
		{name: "format alone", command: rib, chain: "json"},
		{name: "invalid operator argument", command: rib, chain: "first many"},
		{name: "unknown filter", command: rib, chain: "bogus"},
		{name: "alias given an argument", command: rib, chain: "tally now"},
		{name: "command with no filters", command: "show demo other", chain: "count"},
		{name: "empty command", command: " ", chain: "count"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line, whole := FoldPipeChain(tt.command, tt.chain)
			if whole != tt.wantWhole {
				t.Fatalf("FoldPipeChain(%q, %q) whole = %v, want %v (line %q)", tt.command, tt.chain, whole, tt.wantWhole, line)
			}
			if line != tt.wantLine {
				t.Fatalf("FoldPipeChain(%q, %q) = %q, want %q", tt.command, tt.chain, line, tt.wantLine)
			}
		})
	}
}

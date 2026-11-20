// Design: docs/architecture/api/commands.md -- a renderer in another process sends the declared line.
package command

import "testing"

// TestDeclaredLineCarriesOnlyDeclaredOperations asks what a renderer in
// another process (the editor over SSH, the engine-steps plugin) sends for a
// typed line: the command part as typed and the operations the command
// declares, in chain order, each argument word written so it stays one word,
// and none of the residual chain, which the sender renders itself.
//
// Method: a command path declaring two operations, read by ReadDispatchLine;
// DeclaredLine is compared with the expected line and read back, so its
// declared list equals the typed line's and it carries no residual step.
//
// MUTATION: write the residual chain too, or write an argument word bare; the
// line then holds `count`, or `a b` reads back as two words.
func TestDeclaredLineCarriesOnlyDeclaredOperations(t *testing.T) {
	const path = "show test declared-line"
	ResetPipeFiltersForTest()
	t.Cleanup(ResetPipeFiltersForTest)
	RegisterPipeFilters([]string{path},
		PipeFilter{Name: "scope", Description: "Pick a scope", Argument: ArgNone},
		PipeFilter{Name: "named", Description: "Keep one name", Argument: ArgText})

	for _, test := range []struct {
		input string
		want  string
	}{
		{input: path, want: path},
		{input: path + " | count", want: path},
		{input: path + ` | scope | named "a b" | count`, want: path + ` | scope | named "a b"`},
	} {
		t.Run(test.input, func(t *testing.T) {
			line, err := ReadDispatchLine(test.input)
			if err != nil {
				t.Fatalf("read %q: %v", test.input, err)
			}
			got := line.DeclaredLine()
			if got != test.want {
				t.Fatalf("declared line of %q = %q, want %q", test.input, got, test.want)
			}
			sent, err := ReadDispatchLine(got)
			if err != nil {
				t.Fatalf("read the declared line %q: %v", got, err)
			}
			if !samePipeOperations(sent.DeclaredOperations(), line.DeclaredOperations()) {
				t.Errorf("declared list of %q = %v, want %v", got, sent.DeclaredOperations(), line.DeclaredOperations())
			}
			if len(sent.part.residual) != 0 {
				t.Errorf("declared line %q carries a residual chain", got)
			}
		})
	}
}

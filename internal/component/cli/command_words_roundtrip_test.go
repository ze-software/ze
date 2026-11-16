// Design: docs/architecture/cli/error-surface.md -- command printing retains pipe identity.

package cli

import (
	"slices"
	"testing"
)

// TestCommandWordsStringRetainsLiteralPipes reparses printed commands and
// compares both decoded values and structural operators, including pipe data.
func TestCommandWordsStringRetainsLiteralPipes(t *testing.T) {
	for _, input := range []string{
		`set bgp group "|" description "edge | group"`,
		`top show | match "|" | head 1`,
		`show | match '|'`,
		`set bgp group "" description "a\tquoted\nvalue"`,
		`set bgp group "边缘 | 组" description "edge#3"`,
	} {
		t.Run(input, func(t *testing.T) {
			original, err := ParseCommandWords(input)
			if err != nil {
				t.Fatal(err)
			}
			printed := original.String()
			reparsed, err := ParseCommandWords(printed)
			if err != nil {
				t.Fatalf("printed command %q cannot be parsed: %v", printed, err)
			}
			if !slices.Equal(reparsed.tokens, original.tokens) {
				t.Errorf("printed command %q changed words: got %q, want %q", printed, reparsed.tokens, original.tokens)
			}
			if !slices.Equal(reparsed.pipes, original.pipes) {
				t.Errorf("printed command %q changed structural pipes: got %v, want %v", printed, reparsed.pipes, original.pipes)
			}
		})
	}
}

// Design: docs/architecture/cli/command-completion.md -- quoted question marks stay input.

package cli

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestQuestionMarkUsesEscapedQuoteBoundary drives the real key handler on both
// sides of the closing quote after a literal escaped quote in the value.
func TestQuestionMarkUsesEscapedQuoteBoundary(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "inside", input: `show demo mode "a\"b`, want: `show demo mode "a\"b?`},
		{name: "outside", input: `show demo mode "a\"b" `, want: `show demo mode "a\"b" `},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(t)
			m.textInput.SetValue(tc.input)
			m.textInput.CursorEnd()
			asked, _ := pressKey(t, m, tea.KeyPressMsg{Code: '?', Text: "?"})
			if got := asked.InputValue(); got != tc.want {
				t.Errorf("input = %q, want %q", got, tc.want)
			}
		})
	}
}

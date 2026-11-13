// Design: docs/architecture/api/commands.md -- operational corrections retain literal argument boundaries.

package cli

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// TestOperationalInvocationKeepsEditorWords proves the operational writer's
// line survives the actual parser Model.dispatchCommand calls before dispatch.
// A leading apostrophe is quoted for that boundary too; an inner one is data.
func TestOperationalInvocationKeepsEditorWords(t *testing.T) {
	for _, value := range []string{"it's", "'tis", "a\rb", "a\tb", "a\nb"} {
		t.Run(value, func(t *testing.T) {
			var line textbuf.Buffer
			command.WriteInvocation(&line, []string{"show", "demo", "literal"}, []command.InvocationArg{{Name: "mode"}}, map[string]string{"mode": value})
			printed := line.String()
			parsed, err := ParseCommandWords(printed)
			if err != nil {
				t.Fatalf("editor refused operational invocation %q: %v", printed, err)
			}
			want := []string{"show", "demo", "literal", "mode", value}
			if got := parsed.Tokens(); !slices.Equal(got, want) {
				t.Errorf("editor words = %q, want %q", got, want)
			}
			if parsed.FirstPipe() != -1 {
				t.Errorf("literal argument became a pipe at %d", parsed.FirstPipe())
			}
		})
	}
}

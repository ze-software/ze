// Design: docs/architecture/api/commands.md -- explicit empty command values.
package command

import (
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// TestWriteInvocationPreservesExplicitEmptyValues distinguishes an empty
// supplied byte string from an omitted argument before the model judges it.
func TestWriteInvocationPreservesExplicitEmptyValues(t *testing.T) {
	defs := []InvocationArg{{Name: "content"}}
	for _, test := range []struct {
		name   string
		values map[string]string
		want   string
	}{
		{name: "omitted", want: "show fixture bytes"},
		{name: "empty", values: map[string]string{"content": ""}, want: `show fixture bytes content ""`},
		{name: "nonempty", values: map[string]string{"content": "YQ=="}, want: "show fixture bytes content YQ=="},
	} {
		t.Run(test.name, func(t *testing.T) {
			var text textbuf.Buffer
			WriteInvocation(&text, []string{"show", "fixture", "bytes"}, defs, test.values)
			if got := text.String(); got != test.want {
				t.Fatalf("invocation = %q, want %q", got, test.want)
			}
		})
	}
}

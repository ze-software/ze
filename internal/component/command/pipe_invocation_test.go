// Design: docs/architecture/api/commands.md -- quoted command values precede the pipe chain.

package command

import (
	"reflect"
	"testing"
)

// TestParsePipePreservesInvocationAndFilterSyntax keeps literal quote/pipe
// bytes in the command and leaves the existing match-pattern grammar intact.
func TestParsePipePreservesInvocationAndFilterSyntax(t *testing.T) {
	for _, tc := range []struct {
		name string
		line string
		want string
		ops  []pipeOp
	}{
		{name: "quoted pipe", line: `show demo mode "a|b"`, want: `show demo mode "a|b"`},
		{name: "escaped quote then pipe", line: `show demo mode "a\"|b" | count`, want: `show demo mode "a\"|b"`, ops: []pipeOp{{kind: pipeCount}}},
		{name: "backslash belongs to match", line: `show demo mode "a|b" | match x\y`, want: `show demo mode "a|b"`, ops: []pipeOp{{kind: pipeMatch, arg: `x\y`}}},
		{name: "invalid command stays whole", line: `show demo mode a\b | count`, want: `show demo mode a\b | count`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ops := ParsePipe(tc.line)
			if got != tc.want || !reflect.DeepEqual(ops, tc.ops) {
				t.Errorf("ParsePipe(%q) = %q, %#v; want %q, %#v", tc.line, got, ops, tc.want, tc.ops)
			}
		})
	}
}

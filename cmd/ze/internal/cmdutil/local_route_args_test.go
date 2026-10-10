package cmdutil

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// VALIDATES: AC-22 for the `ze <verb>` local route (R6). Every token the
// handler will see, the argv values included, is judged against the leaves the
// model declares for the matched path before the handler runs, and a token at
// the bound reaches it.
// PREVENTS: `ze show env get` running its handler on a 129-character key.
// METHOD: the real model path `show env get` (leaf name, length 1..128), with a
// recording handler in place of the registered one.
func TestInvokeLocalHandlerJudgesArguments(t *testing.T) {
	var got []string
	ran := false
	record := func(args command.ValidatedArgs) int {
		ran, got = true, args.Tokens()
		return 0
	}
	words := []string{"show", "env", "get"}
	overLong := []string{strings.Repeat("k", 129)}
	if code := invokeLocalHandler(record, words, overLong, overLong); code != 1 {
		t.Errorf("over-long key: exit %d, want 1", code)
	}
	if ran {
		t.Fatalf("an over-long key reached the handler as %q", got)
	}
	atBound := []string{strings.Repeat("k", 128)}
	if code := invokeLocalHandler(record, words, atBound, atBound); code != 0 {
		t.Errorf("key at the bound: exit %d, want 0", code)
	}
	if len(got) != 1 || got[0] != atBound[0] {
		t.Errorf("handler args = %q, want the key at the bound", got)
	}
}

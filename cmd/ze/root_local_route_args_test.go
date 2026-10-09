//go:build ze_core

package main

import (
	"strings"
	"testing"
)

// VALIDATES: AC-22 for the root fallback of zeDispatch (route R6, the second
// site after cmdutil's verb route). The tail registry.LookupLocal leaves is
// judged against the leaves the model declares for the matched path before the
// handler runs, and a tail at the bound reaches it unchanged.
// PREVENTS: the root fallback running its handler on a value the model refuses,
// as it did before C-T2a, when it passed LookupLocal's tail straight through.
// METHOD: invokeRootLocalHandler over the real model path `show env get` (leaf
// name, length 1..128), with a recording handler in place of a registered one,
// so the judgment is the model's and the handler is observable.
func TestInvokeRootLocalHandlerJudgesArguments(t *testing.T) {
	var got []string
	ran := false
	record := func(args []string) int {
		ran, got = true, args
		return 0
	}
	path := []string{"show", "env", "get"}

	overLong := []string{strings.Repeat("k", 129)}
	if code := invokeRootLocalHandler(record, append(path, overLong...), overLong); code != 1 {
		t.Errorf("over-long name: exit %d, want 1", code)
	}
	if ran {
		t.Fatalf("an over-long name reached the handler as %q", got)
	}

	if code := invokeRootLocalHandler(record, path, nil); code != 1 {
		t.Errorf("missing mandatory name: exit %d, want 1", code)
	}
	if ran {
		t.Fatalf("a missing mandatory name reached the handler as %q", got)
	}

	atBound := []string{strings.Repeat("k", 128)}
	if code := invokeRootLocalHandler(record, append(path, atBound...), atBound); code != 0 {
		t.Errorf("name at the bound: exit %d, want 0", code)
	}
	if len(got) != 1 || got[0] != atBound[0] {
		t.Errorf("handler args = %q, want the name at the bound", got)
	}
}

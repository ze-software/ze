package clidispatch

import (
	"testing"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestABarePrefixDoesNotResolve proves the gate agrees with the dispatcher
// about a command prefix that names no whole command.
//
// VALIDATES: Resolves answers false for input the dispatcher refuses with
// IncompleteCommandError, as it does for ErrUnknownCommand, and true for the
// whole command; judge reports a literal bare prefix as dead, while a computed
// command whose static prefix starts registered commands still resolves
// through prefixKnown.
// PREVENTS: Go code that emits "clear demo cache" passing the dead-command gate
// although the dispatcher refuses it at runtime.
func TestABarePrefixDoesNotResolve(t *testing.T) {
	dispatcher := pluginserver.NewDispatcher()
	for _, path := range []string{"clear demo cache all", "clear demo cache stats"} {
		if err := dispatcher.Register(path, nil, ""); err != nil {
			t.Fatalf("register %q: %v", path, err)
		}
	}
	surface := Surface{dispatcher: dispatcher, keys: []string{"clear demo cache all", "clear demo cache stats"}}

	if !surface.Resolves("clear demo cache all") {
		t.Error(`the whole command "clear demo cache all" must resolve`)
	}
	for _, input := range []string{"clear demo cache", "clear demo", "clear demo nothing"} {
		if surface.Resolves(input) {
			t.Errorf("%q is refused by the dispatcher, so it must not resolve", input)
		}
	}

	finding, got := judge(surface, "x.go", 1, "Dispatch", `"clear demo cache"`)
	if got != verdictFinding {
		t.Fatalf("a literal bare prefix: verdict %v, want a finding", got)
	}
	if finding.Kind != KindDead {
		t.Errorf("a literal bare prefix: kind %q, want %q", finding.Kind, KindDead)
	}

	if _, got := judge(surface, "x.go", 2, "Dispatch", `"clear demo cache " + which`); got != verdictResolved {
		t.Errorf("a computed command over a known prefix: verdict %v, want resolved", got)
	}
}

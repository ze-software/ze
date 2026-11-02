package command

import (
	"strings"
	"testing"
)

// TestEveryLocalDataRouteMarksItsCall proves each route that runs a
// local-data handler tells the handler which route it is.
//
// VALIDATES: the route marker (spec-command-surface-parity, Implementation
// Step 3). One registered handler records ValidatedArgs.Route() on each call:
// ServeLocal (`ze cli -c`) and the plain `ze <verb>` handler mark RouteLocal,
// AnswerRemote (the daemon route) marks RouteRemote. CallerLocal is true for
// RouteLocal only, so the unmarked zero value is refused the caller-only forms
// as the remote route is.
// PREVENTS: a handler reading the operator's standard input or starting a
// program it names while the daemon runs it for a remote caller.
func TestEveryLocalDataRouteMarksItsCall(t *testing.T) {
	var seen []Route
	withLocalDataArgs(t, func(validated ValidatedArgs) LocalAnswer {
		seen = append(seen, validated.Route())
		return LocalAnswer{Payload: map[string]any{"route": int(validated.Route())}}
	})

	captureStdio(t, func() {
		if _, _, served := ServeLocal(localDataArgsPath+" abcd", ""); !served {
			t.Fatal("ServeLocal did not serve the registered command")
		}
	})

	plain, args, err := LookupLocal(strings.Fields(localDataArgsPath+" abcd"), notDeclared)
	if err != nil || plain == nil {
		t.Fatalf("LookupLocal: handler %v, err %v", plain != nil, err)
	}
	captureStdio(t, func() { InvokeLocal(localDataArgsPath, plain, args) })

	handler, trailing := LookupLocalData(strings.Fields(localDataArgsPath + " abcd"))
	validated, err := ValidateArgs(trailing, nil, nil)
	if err != nil {
		t.Fatalf("judging %v: %v", trailing, err)
	}
	stdout, stderr := captureStdio(t, func() { AnswerRemote(handler, validated) })
	if stdout != "" || stderr != "" {
		t.Errorf("the remote route printed: stdout %q, stderr %q", stdout, stderr)
	}

	want := []Route{RouteLocal, RouteLocal, RouteRemote}
	if len(seen) != len(want) {
		t.Fatalf("handler ran %d times (%v), want %d", len(seen), seen, len(want))
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Errorf("call %d saw route %d, want %d", i, seen[i], want[i])
		}
	}
	if validated.Route() != RouteUnspecified {
		t.Errorf("AnswerRemote marked the caller's value, want its own copy")
	}

	for route, local := range map[Route]bool{RouteUnspecified: false, RouteLocal: true, RouteRemote: false} {
		if route.CallerLocal() != local {
			t.Errorf("Route(%d).CallerLocal() = %v, want %v", route, !local, local)
		}
	}
}

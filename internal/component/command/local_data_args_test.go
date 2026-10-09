package command

import (
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
)

// localDataArgsPath is the command the tests below register a data handler
// under, with one mandatory leaf of length 1..4.
const localDataArgsPath = "show test local data args"

// notDeclared is the declared-path answer for a local lookup that no YANG
// declares: every test here registers its path only in the registries.
func notDeclared(string) (bool, error) { return false, nil }

// withLocalDataArgs registers handler under localDataArgsPath and installs the
// definitions of that path, restoring both registries afterwards. Not safe for
// parallel tests.
func withLocalDataArgs(t *testing.T, handler LocalDataHandler) {
	t.Helper()
	registry.ResetForTest()
	ResetLocalDataForTest()
	t.Cleanup(func() {
		registry.ResetForTest()
		ResetLocalDataForTest()
	})
	if err := RegisterLocalData(localDataArgsPath, handler, registry.Meta{}, func(string, any) int { return 0 }); err != nil {
		t.Fatalf("register local data handler: %v", err)
	}
	withArgDefSource(t, func(p string) ([]ArgDef, error) {
		if p != localDataArgsPath {
			return nil, nil
		}
		return []ArgDef{mustArgDef(NewStringArg("name", []UintRange{{Min: 1, Max: 4}}, nil, ArgOptions{Mandatory: true}))}, nil
	})
}

// TestHandlerTypesTakeValidatedArguments is the LocalDataHandler row of the
// handler-type proof (the StreamingHandler row lives in plugin/server).
//
// VALIDATES: a registered data handler receives exactly the value ValidateArgs
// returned, on both routes that run it: ServeLocal (R2) and the plain local
// handler RegisterLocalData builds (R3), the lone positional token bound to
// its leaf included.
// PREVENTS: a data handler that takes a token slice, which any route can hand
// it without calling the validator.
// Method: register one handler, run it through each route, read what it got.
func TestHandlerTypesTakeValidatedArguments(t *testing.T) {
	t.Run("LocalDataHandler", func(t *testing.T) {
		var received ValidatedArgs
		calls := 0
		withLocalDataArgs(t, func(args ValidatedArgs) (any, int) {
			received = args
			calls++
			return map[string]any{"ok": true}, 0
		})

		if _, code, served := ServeLocal(localDataArgsPath+" abcd", ""); !served || code != 0 {
			t.Fatalf("ServeLocal: served %v code %d, want true 0", served, code)
		}
		assertReceivedName(t, "ServeLocal", received, "abcd")

		plain, args, err := registry.LookupLocal([]string{"show", "test", "local", "data", "args", "wxyz"}, notDeclared)
		if err != nil {
			t.Fatal(err)
		}
		if plain == nil {
			t.Fatal("RegisterLocalData registered no plain local handler")
		}
		if code := plain(args); code != 0 {
			t.Fatalf("plain local handler: exit %d, want 0", code)
		}
		assertReceivedName(t, "plain local handler", received, "wxyz")
		if calls != 2 {
			t.Errorf("handler ran %d times, want 2", calls)
		}
	})
}

// assertReceivedName fails unless received holds exactly want, bound to the
// leaf "name".
func assertReceivedName(t *testing.T, route string, received ValidatedArgs, want string) {
	t.Helper()
	tokens := received.Tokens()
	if len(tokens) != 1 || tokens[0] != want {
		t.Errorf("%s: handler received %q, want [%s]", route, tokens, want)
	}
	name, found := received.Positional("name")
	if !found || name != want {
		t.Errorf("%s: Positional(name) = %q %v, want %q true", route, name, found, want)
	}
}

// TestLocalDataPlainHandlerJudgesArguments proves the plain local handler
// RegisterLocalData builds judges the arguments before the data handler, and
// refuses when the process cannot read the definitions (R3).
//
// VALIDATES: a value one past its declared length exits 1 and never reaches
// the data handler; a process with no definition source refuses rather than
// running unvalidated.
// PREVENTS: `ze show env get` with a 129-character key reaching the handler,
// because this route never passes through ServeLocal.
// Method: register one handler, call the plain handler LookupLocal answers.
func TestLocalDataPlainHandlerJudgesArguments(t *testing.T) {
	calls := 0
	withLocalDataArgs(t, func(ValidatedArgs) (any, int) {
		calls++
		return nil, 0
	})
	plain, args, err := registry.LookupLocal([]string{"show", "test", "local", "data", "args", "abcde"}, notDeclared)
	if err != nil {
		t.Fatal(err)
	}
	if plain == nil {
		t.Fatal("RegisterLocalData registered no plain local handler")
	}
	if code := plain(args); code != 1 {
		t.Errorf("over-long argument: exit %d, want 1", code)
	}

	withArgDefSource(t, nil)
	if code := plain([]string{"abcd"}); code != 1 {
		t.Errorf("no definition source: exit %d, want 1", code)
	}
	if calls != 0 {
		t.Errorf("the data handler ran %d times on refused calls, want none", calls)
	}
}

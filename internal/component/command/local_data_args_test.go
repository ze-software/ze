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
	withLocalArgDefs(t)
	if err := RegisterLocalData(localDataArgsPath, handler, registry.Meta{}, func(string, any) int { return 0 }); err != nil {
		t.Fatalf("register local data handler: %v", err)
	}
}

// withLocalArgDefs empties the local, offline-fallback and data registries for
// the test and installs the definitions of localDataArgsPath, restoring both
// afterwards. Not safe for parallel tests.
func withLocalArgDefs(t *testing.T) {
	t.Helper()
	ResetLocalForTest()
	ResetLocalDataForTest()
	t.Cleanup(func() {
		ResetLocalForTest()
		ResetLocalDataForTest()
	})
	withArgDefSource(t, func(p string) ([]ArgDef, error) {
		if p != localDataArgsPath {
			return nil, nil
		}
		return []ArgDef{mustArgDef(NewStringArg("name", []UintRange{{Min: 1, Max: 4}}, nil, ArgOptions{Mandatory: true}))}, nil
	})
}

// TestHandlerTypesTakeValidatedArguments is the LocalDataHandler and
// LocalHandler rows of the handler-type proof (the StreamingHandler row lives
// in plugin/server).
//
// VALIDATES: a registered data handler receives exactly the value ValidateArgs
// returned, on both routes that run it: ServeLocal (R2) and the plain local
// handler RegisterLocalData builds (R3), the lone positional token bound to
// its leaf included. A local handler and an offline fallback receive the value
// InvokeLocal judged (R6, R7).
// PREVENTS: a handler that takes a token slice, which any route can hand it
// without calling the validator.
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

		plain, args, err := LookupLocal([]string{"show", "test", "local", "data", "args", "wxyz"}, notDeclared)
		if err != nil {
			t.Fatal(err)
		}
		if plain == nil {
			t.Fatal("RegisterLocalData registered no plain local handler")
		}
		if code := InvokeLocal(localDataArgsPath, plain, args); code != 0 {
			t.Fatalf("plain local handler: exit %d, want 0", code)
		}
		assertReceivedName(t, "plain local handler", received, "wxyz")
		if calls != 2 {
			t.Errorf("handler ran %d times, want 2", calls)
		}
	})

	t.Run("LocalHandler", func(t *testing.T) {
		var received ValidatedArgs
		calls := 0
		withLocalArgDefs(t)
		var handler LocalHandler = func(args ValidatedArgs) int {
			received = args
			calls++
			return 0
		}
		if err := RegisterLocal(localDataArgsPath, handler); err != nil {
			t.Fatal(err)
		}
		if err := RegisterOfflineFallback(localDataArgsPath, handler); err != nil {
			t.Fatal(err)
		}
		words := []string{"show", "test", "local", "data", "args", "abcd"}

		local, args, err := LookupLocal(words, notDeclared)
		if err != nil {
			t.Fatal(err)
		}
		if local == nil {
			t.Fatal("LookupLocal answered no handler")
		}
		if code := InvokeLocal(localDataArgsPath, local, args); code != 0 {
			t.Fatalf("local handler: exit %d, want 0", code)
		}
		assertReceivedName(t, "local handler", received, "abcd")

		fallback, args := LookupOfflineFallback(words)
		if fallback == nil {
			t.Fatal("LookupOfflineFallback answered no handler")
		}
		if code := InvokeLocal(localDataArgsPath, fallback, args); code != 0 {
			t.Fatalf("offline fallback: exit %d, want 0", code)
		}
		assertReceivedName(t, "offline fallback", received, "abcd")

		if code := InvokeLocal(localDataArgsPath, local, []string{"abcde"}); code != 1 {
			t.Errorf("over-long argument: exit %d, want 1", code)
		}
		if calls != 2 {
			t.Errorf("handler ran %d times, want 2 (the refused call must not reach it)", calls)
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
// Method: register one handler, run the plain handler LookupLocal answers
// through InvokeLocal, the judgment every `ze <verb>` route runs.
func TestLocalDataPlainHandlerJudgesArguments(t *testing.T) {
	calls := 0
	withLocalDataArgs(t, func(ValidatedArgs) (any, int) {
		calls++
		return nil, 0
	})
	plain, args, err := LookupLocal([]string{"show", "test", "local", "data", "args", "abcde"}, notDeclared)
	if err != nil {
		t.Fatal(err)
	}
	if plain == nil {
		t.Fatal("RegisterLocalData registered no plain local handler")
	}
	if code := InvokeLocal(localDataArgsPath, plain, args); code != 1 {
		t.Errorf("over-long argument: exit %d, want 1", code)
	}

	withArgDefSource(t, nil)
	if code := InvokeLocal(localDataArgsPath, plain, []string{"abcd"}); code != 1 {
		t.Errorf("no definition source: exit %d, want 1", code)
	}
	if calls != 0 {
		t.Errorf("the data handler ran %d times on refused calls, want none", calls)
	}
}

package command

import (
	"strings"
	"testing"
)

// offlineCausePath is the command the tests below register a fallback under.
// It reuses the model of localDataArgsPath: one mandatory leaf of length 1..4.
const offlineCausePath = localDataArgsPath

// noListenerCause is the cause a route hands a fallback when nothing listens.
func noListenerCause() DaemonUnanswered {
	return NewDaemonUnanswered("no ze listens at 127.0.0.1:1 (connection refused)", "ze start")
}

// VALIDATES: an offline fallback receives the cause the route learned, whole:
// why the daemon did not answer and the command that gets its answer.
// PREVENTS: a fallback that words the cause itself, which is how show host
// trend came to print "no daemon answered" and `ze start` for every failure.
// Method: register a fallback, run it through InvokeOfflineFallback with a
// cause, read what it was handed.
func TestOfflineFallbackReceivesTheCause(t *testing.T) {
	withLocalArgDefs(t)
	var received DaemonUnanswered
	if err := RegisterOfflineFallback(offlineCausePath, func(_ ValidatedArgs, unanswered DaemonUnanswered) int {
		received = unanswered
		return 1
	}); err != nil {
		t.Fatal(err)
	}
	handler, args := LookupOfflineFallback([]string{"show", "test", "local", "data", "args", "abcd"})
	if handler == nil {
		t.Fatal("LookupOfflineFallback answered no handler")
	}
	cause := NewDaemonUnanswered(
		"a ze listens at 127.0.0.1:2222, but this account has no credentials to log in to it (no stored username)",
		`ze cli --user <name> -c "show test local data args abcd"`)
	if code := InvokeOfflineFallback(offlineCausePath, handler, args, cause); code != 1 {
		t.Fatalf("exit = %d, want the fallback's own 1", code)
	}
	if received.Why() != cause.Why() || received.Fix() != cause.Fix() {
		t.Errorf("fallback was handed %q / %q, want %q / %q", received.Why(), received.Fix(), cause.Why(), cause.Fix())
	}
}

// VALIDATES: an answer a fallback gave from this process is followed, on
// stderr, by a note that says the answer is this process's, why the daemon
// gave none, and the command that gets the daemon's, on a fix line of its own.
// PREVENTS: a local answer read as the daemon's while a daemon runs and this
// account cannot reach it.
// Method: a fallback that exits 0, run with a cause, with stderr captured.
func TestOfflineFallbackAnswerIsMarkedAsThisProcess(t *testing.T) {
	withLocalArgDefs(t)
	answered := func(ValidatedArgs, DaemonUnanswered) int { return 0 }
	var code int
	stderr := captureStderr(t, func() {
		code = InvokeOfflineFallback(offlineCausePath, answered, []string{"abcd"}, noListenerCause())
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	want := "note: this ze process answered " + offlineCausePath + " itself, because the daemon did not\n" +
		"the daemon was asked first and gave no answer: no ze listens at 127.0.0.1:1 (connection refused)\n" +
		"fix: ze start\n"
	if stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

// VALIDATES: a fallback that refused (a non-zero exit) gets no note: it
// answered nothing, and its own refusal names the cause.
// PREVENTS: "this ze process answered ..." printed under a refusal.
// Method: a fallback that exits 1 and one whose arguments the model refuses,
// each run with stderr captured.
func TestOfflineFallbackRefusalGetsNoNote(t *testing.T) {
	withLocalArgDefs(t)
	ran := 0
	refused := func(ValidatedArgs, DaemonUnanswered) int {
		ran++
		return 1
	}
	stderr := captureStderr(t, func() {
		if code := InvokeOfflineFallback(offlineCausePath, refused, []string{"abcd"}, noListenerCause()); code != 1 {
			t.Errorf("refusing fallback: exit %d, want 1", code)
		}
	})
	if strings.Contains(stderr, "note:") {
		t.Errorf("a refusal was followed by an answer note: %q", stderr)
	}
	stderr = captureStderr(t, func() {
		if code := InvokeOfflineFallback(offlineCausePath, refused, []string{"abcde"}, noListenerCause()); code != 1 {
			t.Errorf("over-long argument: exit %d, want 1", code)
		}
	})
	if ran != 1 {
		t.Errorf("the fallback ran %d times, want 1: a refused argument must not reach it", ran)
	}
	if strings.Contains(stderr, "note:") {
		t.Errorf("a refused argument was followed by an answer note: %q", stderr)
	}
	if stderr == "" {
		t.Error("a refused argument printed no refusal")
	}
}

// VALIDATES: a route that runs a fallback with no cause is a defect the
// program names, never a fallback that words an empty cause.
// Method: the zero DaemonUnanswered, and each empty part of the constructor.
func TestOfflineFallbackRefusesAMissingCause(t *testing.T) {
	withLocalArgDefs(t)
	mustPanic := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s did not panic", name)
			}
		}()
		fn()
	}
	never := func(ValidatedArgs, DaemonUnanswered) int {
		t.Error("the fallback ran with no cause")
		return 0
	}
	mustPanic("InvokeOfflineFallback with the zero cause", func() {
		InvokeOfflineFallback(offlineCausePath, never, []string{"abcd"}, DaemonUnanswered{})
	})
	mustPanic("NewDaemonUnanswered with an empty why", func() { NewDaemonUnanswered("", "ze start") })
	mustPanic("NewDaemonUnanswered with an empty fix", func() { NewDaemonUnanswered("no ze listens", "") })
}

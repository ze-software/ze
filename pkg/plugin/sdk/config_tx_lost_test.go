package sdk

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
)

// TestRunningConfigLostCarriesWhyAndFix pins the operator error an install
// answers when it could not put the running config back. Method: build it
// with and without a first step of the plugin's own, then compare the whole
// rendered text, the three parts through errors.As, and the chain through
// errors.Is.
//
// VALIDATES: AC-11 of spec-operator-errors-name-the-fix. Every plugin that
// loses its running config gives the same why and ends its fix with the same
// daemon restart, and the transaction still finds ErrRunningConfigLost.
// PREVENTS: a plugin spelling its own next step for the same loss, and an
// operator error whose chain hides the sentinel the rollback path reads.
func TestRunningConfigLostCarriesWhyAndFix(t *testing.T) {
	cause := errors.New("bind 127.0.0.1:53: address already in use")
	const problem = "the test plugin could not apply the new config, and could not bind the running config's listeners again"

	cases := []struct {
		name      string
		firstStep string
		fix       string
	}{
		{"listener lost", RunningConfigLostListenerFix, RunningConfigLostListenerFix + ".\n" + RunningConfigLostRestart},
		{"restart only", "", RunningConfigLostRestart},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := RunningConfigLost(cause, problem, tc.firstStep)

			want := problem + "\n" +
				RunningConfigLostWhy + ": " + cause.Error() + " (" + ErrRunningConfigLost.Error() + ")\n" +
				"fix: " + tc.fix
			if err.Error() != want {
				t.Errorf("RunningConfigLost =\n%s\nwant\n%s", err, want)
			}

			if !errors.Is(err, ErrRunningConfigLost) {
				t.Errorf("RunningConfigLost(%v) does not wrap ErrRunningConfigLost", cause)
			}
			if !errors.Is(err, cause) {
				t.Errorf("RunningConfigLost(%v) does not wrap its cause", cause)
			}

			var operr *opererr.Error
			if !errors.As(err, &operr) {
				t.Fatalf("RunningConfigLost = %T, want an *opererr.Error", err)
			}
			if got := operr.Problem(); got != problem {
				t.Errorf("problem = %q, want %q", got, problem)
			}
			if got := operr.Why(); got != RunningConfigLostWhy {
				t.Errorf("why = %q, want %q", got, RunningConfigLostWhy)
			}
			if got := operr.Fix(); got != tc.fix {
				t.Errorf("fix = %q, want %q", got, tc.fix)
			}
		})
	}
}

// TestRunningConfigLostEmptyProblem pins what a plugin that passes no problem
// gets. Method: call RunningConfigLost with an empty problem under a recover,
// then compare the rendered text, the problem through errors.As, and the chain
// through errors.Is.
//
// VALIDATES: RunningConfigLost is public SDK API, so an empty problem from a
// plugin outside the repository answers the SDK's own problem.
// PREVENTS: opererr.Wrap's BUG panic on an empty problem crashing the plugin
// that called it.
func TestRunningConfigLostEmptyProblem(t *testing.T) {
	cause := errors.New("bind 127.0.0.1:53: address already in use")

	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("RunningConfigLost with an empty problem panicked: %v", r)
			}
		}()
		err = RunningConfigLost(cause, "", RunningConfigLostListenerFix)
	}()

	want := RunningConfigLostProblem + "\n" +
		RunningConfigLostWhy + ": " + cause.Error() + " (" + ErrRunningConfigLost.Error() + ")\n" +
		"fix: " + RunningConfigLostListenerFix + ".\n" + RunningConfigLostRestart
	if err.Error() != want {
		t.Errorf("RunningConfigLost =\n%s\nwant\n%s", err, want)
	}
	var operr *opererr.Error
	if !errors.As(err, &operr) {
		t.Fatalf("RunningConfigLost = %T, want an *opererr.Error", err)
	}
	if got := operr.Problem(); got != RunningConfigLostProblem {
		t.Errorf("problem = %q, want %q", got, RunningConfigLostProblem)
	}
	if !errors.Is(err, ErrRunningConfigLost) {
		t.Errorf("RunningConfigLost(%v) does not wrap ErrRunningConfigLost", cause)
	}
	if !errors.Is(err, cause) {
		t.Errorf("RunningConfigLost(%v) does not wrap its cause", cause)
	}
}

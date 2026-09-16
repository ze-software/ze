// VALIDATES: AC-9 of spec-ike-padded-path-probe (an unregistered prober is a named
// error, distinct from a registered engine refusing) and the leaf's registration shape
// PREVENTS: show mtu reading "no engine in this build" as a refusal, and a second
// registrant replacing the engine's prober in silence
package ikeprobe

import (
	"context"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/core/probe"
)

// TestIKEProbeUnregisteredIsNamed proves that a caller can tell "no IKE engine is in
// this build" from "the engine refused": the first answers ErrNotRegistered, the second
// answers OutcomeRefused and no error.
//
// MUTATION: returning Result{}, nil from Probe when no provider is registered makes
// the first assertion fail.
func TestIKEProbeUnregisteredIsNamed(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	req := Request{Peer: "site-a", WireOctets: 1400, DF: probe.DFHonorCache}
	result, err := Probe(context.Background(), req)
	if !errors.Is(err, ErrNotRegistered) {
		t.Fatalf("unregistered prober answered (%v, %v), want ErrNotRegistered", result, err)
	}

	Register(func(_ context.Context, got Request) (Result, error) {
		if got != req {
			t.Errorf("the prober was asked %+v, want %+v", got, req)
		}
		return Result{Outcome: OutcomeRefused, Refusal: RefusalSADown}, nil
	})
	result, err = Probe(context.Background(), req)
	if err != nil {
		t.Fatalf("registered prober answered an error: %v", err)
	}
	if result.Outcome != OutcomeRefused {
		t.Fatalf("registered prober answered %v, want refused", result.Outcome)
	}
	if result.Refusal != RefusalSADown {
		t.Fatalf("the refusal is %v, want sa-down", result.Refusal)
	}
}

// TestIKEProbeRegisterTwicePanics proves the registry holds one prober: a nil or a
// second Register is a programmer error and panics with the BUG prefix rather than
// replacing the first in silence.
func TestIKEProbeRegisterTwicePanics(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	first := func(_ context.Context, _ Request) (Result, error) {
		return Result{Outcome: OutcomeFits}, nil
	}
	Register(first)

	assertBugPanic(t, "a second Register", func() {
		Register(func(_ context.Context, _ Request) (Result, error) {
			return Result{Outcome: OutcomeTooBig}, nil
		})
	})
	assertBugPanic(t, "a nil Register", func() { Register(nil) })

	result, err := Probe(context.Background(), Request{Peer: "site-a"})
	if err != nil {
		t.Fatalf("the first prober was lost: %v", err)
	}
	if result.Outcome != OutcomeFits {
		t.Fatalf("the first prober's answer changed: %v", result.Outcome)
	}
}

// assertBugPanic runs fn and fails the test unless it panics with a message that
// carries the BUG prefix a programmer error owes (docs/contributing/ze-go-style.md).
func assertBugPanic(t *testing.T, what string, fn func()) {
	t.Helper()
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatalf("%s did not panic", what)
		}
		msg, ok := recovered.(string)
		if !ok {
			t.Fatalf("%s panicked with %T, want a string", what, recovered)
		}
		if len(msg) < 4 || msg[:4] != "BUG:" {
			t.Fatalf("%s panicked with %q, want the BUG: prefix", what, msg)
		}
	}()
	fn()
}

// TestProbeOutcomeZeroIsUnspecified pins the display name of every outcome and
// refusal so a payload never prints a number, and proves the zero value of each
// names itself as unset rather than passing for an answer.
func TestProbeOutcomeZeroIsUnspecified(t *testing.T) {
	outcomes := map[Outcome]string{
		OutcomeUnspecified: "unspecified",
		OutcomeFits:        "fits",
		OutcomeTooBig:      "too-big",
		OutcomeSAFailed:    "sa-failed",
		OutcomeRefused:     "refused",
		Outcome(9):         "unspecified",
	}
	for outcome, want := range outcomes {
		if got := outcome.String(); got != want {
			t.Errorf("Outcome(%d).String() = %q, want %q", outcome, got, want)
		}
	}
	refusals := map[Refusal]string{
		RefusalUnspecified:  "unspecified",
		RefusalSADown:       "sa-down",
		RefusalRekeyPending: "rekey-pending",
		RefusalRekeyHeld:    "rekey-held",
		RefusalWindowHeld:   "window-held",
		RefusalSize:         "size",
		RefusalFamily:       "family",
		RefusalSendFailed:   "send-failed",
		RefusalRekeyed:      "rekeyed",
		Refusal(10):         "unspecified",
	}
	for refusal, want := range refusals {
		if got := refusal.String(); got != want {
			t.Errorf("Refusal(%d).String() = %q, want %q", refusal, got, want)
		}
	}
	var zero Result
	if zero.Outcome != OutcomeUnspecified {
		t.Errorf("the zero Result carries outcome %v, want unspecified", zero.Outcome)
	}
}

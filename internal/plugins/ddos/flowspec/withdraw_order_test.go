// Design: docs/guide/ddos-mitigation.md -- the live-announcement record and ze doctor
// Related: responder.go (withdrawThrough), doctor.go (liveMatch, ruleFindings)

package flowspec

import (
	"context"
	"errors"
	"testing"
)

// forgetObserver is a fakeDispatcher that reads, inside Forget, what the
// responder publishes to its lock-free readers: the state a `ze doctor` run
// sees when it reads the record at that instant (liveMatch, then the record).
type forgetObserver struct {
	fakeDispatcher
	r *responder
	// activeAtForget holds, for each Forget, whether the responder still
	// published the announcement as live.
	activeAtForget []bool
	// forgetErr, when set, is what Forget answers, leaving the record stored.
	forgetErr error
}

func (f *forgetObserver) Forget() error {
	active, _, _ := f.r.status()
	f.activeAtForget = append(f.activeAtForget, active)
	if f.forgetErr != nil {
		return f.forgetErr
	}
	return f.fakeDispatcher.Forget()
}

// WithContext returns f itself, so a caller that asks for a context keeps the observer.
func (f *forgetObserver) WithContext(context.Context) routeDispatcher { return f }

// TestWithdrawForgetsTheRecordBeforeClearingTheLiveFlag proves that a
// withdrawal removes the record while the responder still publishes the rule
// as live, and clears the live flag afterwards whether or not the removal
// succeeded.
//
// VALIDATES: the runtime doctor check never sees a record with no live owner
// for a rule that is being withdrawn normally. It samples the live state, then
// reads the record (answerOrphanRuntimeCheck), so a flag cleared before the
// record goes reports an orphan for that instant. A failed removal must still
// leave no live flag over a withdrawn rule: the record is then the orphan the
// doctor is right to report.
// METHOD: a responder announces through forgetObserver, then withdraws; the
// observer reads status() at the moment Forget is called.
// PREVENTS: withdrawThrough clearing the live flag before Forget.
func TestWithdrawForgetsTheRecordBeforeClearingTheLiveFlag(t *testing.T) {
	cases := []struct {
		name      string
		forgetErr error
	}{
		{name: "record removed"},
		{name: "record removal fails", forgetErr: errors.New("state remove refused")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			obs := &forgetObserver{forgetErr: tc.forgetErr}
			r := newResponder(enforcingConfig(), obs)
			obs.r = r
			announceOnce(t, r, exitTestVictim)

			r.mu.Lock()
			r.withdraw()
			r.mu.Unlock()

			if len(obs.activeAtForget) != 1 {
				t.Fatalf("Forget calls during the withdrawal = %d, want 1", len(obs.activeAtForget))
			}
			if !obs.activeAtForget[0] {
				t.Fatalf("the live flag was cleared before the record was removed, so a doctor read in between reports the rule as an orphan")
			}
			if active, target, _ := r.status(); active {
				t.Fatalf("the responder still reports %s announced after the withdrawal", target.DstPrefix)
			}
			if got := countCommands(obs.cmds, "/flow del "); got != 1 {
				t.Fatalf("withdrawals for %s = %d, want 1 (commands: %v)", exitTestVictim, got, obs.cmds)
			}
			if tc.forgetErr == nil && obs.recorded != nil {
				t.Fatalf("the record is still stored after a withdrawal whose removal succeeded")
			}
			if tc.forgetErr != nil && obs.recorded == nil {
				t.Fatalf("the record was removed although Forget failed")
			}
		})
	}
}

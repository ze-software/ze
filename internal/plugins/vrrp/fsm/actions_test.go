package fsm

import "testing"

// TestActionsImplementInterface pins the closed Action set: every action type
// satisfies the Action interface. The engine (spec-vrrp-5) is the sole executor
// and must switch over exactly this set.
//
// VALIDATES: Types section "Actions (ordered slice returned by the handle
// method; closed set)"; A-3 (actions-as-values covers every side effect).
func TestActionsImplementInterface(t *testing.T) {
	actions := []Action{
		SendAdvert{},
		SendAdvertZeroPriority{},
		InstallVIPs{},
		RemoveVIPs{},
		AnnounceFailover{},
		StartMasterDownTimer{},
		StartAdvertTimer{},
		StartPreemptDelayTimer{},
		StopPreemptDelayTimer{},
		StopTimers{},
		EmitStateChange{},
	}
	if len(actions) != 11 {
		t.Fatalf("expected 11 action types, got %d", len(actions))
	}
}

// TestReasonConstants pins the EmitStateChange reason tokens verbatim: they feed
// eventbus consumers, metrics labels, and logs in spec-vrrp-5, so a typo here is
// a wire-visible break.
//
// VALIDATES: Critical Review "reason strings as tabled (they feed eventbus
// consumers in spec-vrrp-5)".
func TestReasonConstants(t *testing.T) {
	cases := map[string]string{
		ReasonStartupOwner:        "startup-owner",
		ReasonStartup:             "startup",
		ReasonMasterDownExpired:   "master-down-expired",
		ReasonPreemptDelayExpired: "preempt-delay-expired",
		ReasonShutdown:            "shutdown",
		ReasonHigherPriority:      "higher-priority",
		ReasonTieBreakLost:        "tie-break-lost",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("reason constant = %q, want %q", got, want)
		}
	}
}

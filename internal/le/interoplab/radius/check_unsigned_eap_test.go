package radius

import (
	"context"
	"testing"
)

// TestEAPUnsignedCheckerPolarities proves checkEAPUnsigned passes only for a
// ze that discards and counts an EAP answer whose Message-Authenticator the
// relay removed, and that each way of producing the same refusal for another
// reason fails it. Method: the stub lab models the chain and the relay, and
// each case breaks one link.
func TestEAPUnsignedCheckerPolarities(t *testing.T) {
	cases := []struct {
		name    string
		policy  func() labPolicy
		wantErr string
	}{
		{
			name:   "a ze that discards and counts the unsigned answer passes",
			policy: conformingEAPUnsigned,
		},
		{
			name: "a ze that accepts the unsigned answer fails, because the login completes",
			policy: func() labPolicy {
				policy := conformingEAPUnsigned()
				policy.discardUnsigned = false
				return policy
			},
			wantErr: "assertion 6",
		},
		{
			name: "a discard the counter never sees fails",
			policy: func() labPolicy {
				policy := conformingEAPUnsigned()
				policy.countDiscards = false
				return policy
			},
			wantErr: "assertion 9",
		},
		{
			name: "a ze exporting no discard series fails rather than reading zero",
			policy: func() labPolicy {
				policy := conformingEAPUnsigned()
				policy.metricsExported = false
				return policy
			},
			wantErr: "carry no " + unsignedDiscardSeries,
		},
		{
			name: "a relay that carries nothing fails the positive control, so its refusal is never read",
			policy: func() labPolicy {
				policy := conformingEAPUnsigned()
				policy.relayBroken = true
				return policy
			},
			wantErr: "assertion 2",
		},
		{
			name: "a server that keeps no record fails",
			policy: func() labPolicy {
				policy := conformingEAPUnsigned()
				policy.recordRequests = false
				return policy
			},
			wantErr: "FreeRADIUS record",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := checkEAPUnsigned(context.Background(), newStubLab(testCase.policy()))
			assertCheckerVerdict(t, err, testCase.wantErr)
		})
	}
}

// TestUnsignedDiscardCountRefusesAMissingSeries pins that an absent counter is an
// error, never a zero, and that a labeled or prefixed series is not read as
// the unlabeled one.
func TestSeriesValueRefusesAMissingSeries(t *testing.T) {
	if _, err := unsignedDiscardCount("ze_up 1\n"); err == nil {
		t.Fatal("a missing series read as a value")
	}
	if _, err := unsignedDiscardCount(unsignedDiscardSeries + "_extra 4\n"); err == nil {
		t.Fatal("a longer series name read as the counter")
	}
	value, err := unsignedDiscardCount("# TYPE x counter\n" + unsignedDiscardSeries + " 3\n")
	if err != nil {
		t.Fatalf("read the counter: %v", err)
	}
	if value != 3 {
		t.Fatalf("counter: want 3, got %d", value)
	}
}

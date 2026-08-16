package runner

// TestHostLoadContended / TestHostLoadString / TestSnapshotHostLoad
// / TestParseDigitsStopsAtNonDigit moved to internal/core/hostload alongside the
// extracted Load type + sampling. Only near-timeout classification, which is
// runner-local (uses stateUnknown / FailType*), remains here.

import "testing"

func TestNearTimeoutClassification(t *testing.T) {
	tests := []struct {
		name         string
		elapsedRatio float64
		failureType  string
		expectNear   bool
	}{
		{
			name:         "85% elapsed with unknown failure",
			elapsedRatio: 0.85,
			failureType:  stateUnknown,
			expectNear:   true,
		},
		{
			name:         "50% elapsed with unknown failure",
			elapsedRatio: 0.50,
			failureType:  stateUnknown,
			expectNear:   false,
		},
		{
			name:         "85% elapsed with mismatch failure",
			elapsedRatio: 0.85,
			failureType:  FailTypeMismatch,
			expectNear:   false,
		},
		{
			name:         "exactly 80% boundary",
			elapsedRatio: 0.80,
			failureType:  stateUnknown,
			expectNear:   false,
		},
		{
			name:         "81% just over boundary",
			elapsedRatio: 0.81,
			failureType:  stateUnknown,
			expectNear:   true,
		},
		{
			name:         "100% would have been timeout",
			elapsedRatio: 1.0,
			failureType:  stateUnknown,
			expectNear:   true,
		},
		{
			name:         "empty failure type at 90%",
			elapsedRatio: 0.90,
			failureType:  "",
			expectNear:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsNearTimeout(tc.elapsedRatio, tc.failureType)
			if got != tc.expectNear {
				t.Errorf("IsNearTimeout(%v, %q) = %v, want %v", tc.elapsedRatio, tc.failureType, got, tc.expectNear)
			}
		})
	}
}

package runner

import (
	"testing"
)

func TestHostLoadContended(t *testing.T) {
	tests := []struct {
		name     string
		load     HostLoad
		expected bool
	}{
		{
			name:     "quiet machine",
			load:     HostLoad{LoadAvg1: 1.0, CPUs: 8, ZeProcs: 1, GoTestProcs: 0},
			expected: false,
		},
		{
			name:     "loaded but no concurrent processes",
			load:     HostLoad{LoadAvg1: 10.0, CPUs: 8, ZeProcs: 1, GoTestProcs: 0},
			expected: false,
		},
		{
			name:     "loaded with concurrent ze-test",
			load:     HostLoad{LoadAvg1: 10.0, CPUs: 8, ZeProcs: 2, GoTestProcs: 0},
			expected: true,
		},
		{
			name:     "loaded with concurrent go test",
			load:     HostLoad{LoadAvg1: 10.0, CPUs: 8, ZeProcs: 1, GoTestProcs: 3},
			expected: true,
		},
		{
			name:     "at CPU boundary with concurrency",
			load:     HostLoad{LoadAvg1: 8.0, CPUs: 8, ZeProcs: 2, GoTestProcs: 0},
			expected: false,
		},
		{
			name:     "just over CPU boundary with concurrency",
			load:     HostLoad{LoadAvg1: 8.1, CPUs: 8, ZeProcs: 2, GoTestProcs: 0},
			expected: true,
		},
		{
			name:     "zero CPUs defaults to not contended",
			load:     HostLoad{LoadAvg1: 0, CPUs: 0, ZeProcs: 0, GoTestProcs: 0},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.load.Contended()
			if got != tc.expected {
				t.Errorf("Contended() = %v, want %v for %+v", got, tc.expected, tc.load)
			}
		})
	}
}

func TestHostLoadString(t *testing.T) {
	h := HostLoad{LoadAvg1: 3.5, CPUs: 8, ZeProcs: 2, GoTestProcs: 1}
	s := h.String()
	if s == "" {
		t.Error("String() returned empty")
	}
	// Should contain the key values
	for _, want := range []string{"3.5", "8", "2", "1"} {
		if !containsSubstring(s, want) {
			t.Errorf("String() = %q, missing %q", s, want)
		}
	}
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestParseDigitsStopsAtNonDigit(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"42\n", 42},
		{"0\n", 0},
		{"", 0},
		{"error 42", 0},
		{"  7  ", 7},
		{"123abc", 123},
	}
	for _, tc := range tests {
		got := parseDigits(tc.input)
		if got != tc.want {
			t.Errorf("parseDigits(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestSnapshotHostLoad(t *testing.T) {
	h := SnapshotHostLoad()
	if h.CPUs <= 0 {
		t.Errorf("CPUs = %d, want > 0", h.CPUs)
	}
	// LoadAvg1 should be non-negative (zero is possible but unlikely on a running system)
	if h.LoadAvg1 < 0 {
		t.Errorf("LoadAvg1 = %f, want >= 0", h.LoadAvg1)
	}
}

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

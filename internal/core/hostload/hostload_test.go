package hostload

import (
	"strings"
	"testing"
)

// VALIDATES: Load.Contended requires load > CPUs AND a concurrent test process.
// PREVENTS: the verify status tool and the runner drifting on the "contended"
// verdict -- both now read this single definition.
func TestLoadContended(t *testing.T) {
	tests := []struct {
		name     string
		load     Load
		expected bool
	}{
		{"quiet machine", Load{LoadAvg1: 1.0, CPUs: 8, ZeProcs: 1, GoTestProcs: 0}, false},
		{"loaded but no concurrent processes", Load{LoadAvg1: 10.0, CPUs: 8, ZeProcs: 1, GoTestProcs: 0}, false},
		{"loaded with concurrent ze-test", Load{LoadAvg1: 10.0, CPUs: 8, ZeProcs: 2, GoTestProcs: 0}, true},
		{"loaded with concurrent go test", Load{LoadAvg1: 10.0, CPUs: 8, ZeProcs: 1, GoTestProcs: 3}, true},
		{"concurrency but quiet (no load gate)", Load{LoadAvg1: 2.0, CPUs: 8, ZeProcs: 2, GoTestProcs: 4}, false},
		{"at CPU boundary with concurrency", Load{LoadAvg1: 8.0, CPUs: 8, ZeProcs: 2, GoTestProcs: 0}, false},
		{"just over CPU boundary with concurrency", Load{LoadAvg1: 8.1, CPUs: 8, ZeProcs: 2, GoTestProcs: 0}, true},
		{"zero CPUs defaults to not contended", Load{LoadAvg1: 0, CPUs: 0, ZeProcs: 0, GoTestProcs: 0}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.load.Contended(); got != tc.expected {
				t.Errorf("Contended() = %v, want %v for %+v", got, tc.expected, tc.load)
			}
		})
	}
}

func TestLoadString(t *testing.T) {
	l := Load{LoadAvg1: 3.5, CPUs: 8, ZeProcs: 2, GoTestProcs: 1}
	s := l.String()
	if s == "" {
		t.Fatal("String() returned empty")
	}
	for _, want := range []string{"3.5", "8", "2", "1"} {
		if !strings.Contains(s, want) {
			t.Errorf("String() = %q, missing %q", s, want)
		}
	}
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
		if got := parseDigits(tc.input); got != tc.want {
			t.Errorf("parseDigits(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestSnapshot(t *testing.T) {
	l := Snapshot()
	if l.CPUs <= 0 {
		t.Errorf("CPUs = %d, want > 0", l.CPUs)
	}
	if l.LoadAvg1 < 0 {
		t.Errorf("LoadAvg1 = %f, want >= 0", l.LoadAvg1)
	}
}

package route

import "testing"

// TestActionTypeString verifies kebab-case string output for all action types.
//
// VALIDATES: All route action types produce correct string names.
// PREVENTS: Typos in action names breaking trigger forms.
func TestActionTypeString(t *testing.T) {
	tests := []struct {
		action ActionType
		want   string
	}{
		{ActionChurn, "churn"},
		{ActionPartialWithdraw, "partial-withdraw"},
		{ActionFullWithdraw, "full-withdraw"},
		{ActionType(99), "unknown(99)"},
	}
	for _, tt := range tests {
		if got := tt.action.String(); got != tt.want {
			t.Errorf("ActionType(%d).String() = %q, want %q", tt.action, got, tt.want)
		}
	}
}

// TestActionTypeFromString verifies kebab-case parsing for all action types.
//
// VALIDATES: String→ActionType round-trip is lossless.
// PREVENTS: Web trigger form sending unrecognized action names.
func TestActionTypeFromString(t *testing.T) {
	tests := []struct {
		name string
		want ActionType
		ok   bool
	}{
		{"churn", ActionChurn, true},
		{"partial-withdraw", ActionPartialWithdraw, true},
		{"full-withdraw", ActionFullWithdraw, true},
		{"unknown", 0, false},
		{"", 0, false},
	}
	for _, tt := range tests {
		got, ok := ActionTypeFromString(tt.name)
		if ok != tt.ok || got != tt.want {
			t.Errorf("ActionTypeFromString(%q) = (%d, %v), want (%d, %v)", tt.name, got, ok, tt.want, tt.ok)
		}
	}
}

// TestActionTypeRoundTrip verifies String→FromString round-trip for all valid types.
//
// VALIDATES: Every action type survives serialization round-trip.
// PREVENTS: Adding a new action type without updating both methods.
func TestActionTypeRoundTrip(t *testing.T) {
	for _, at := range []ActionType{ActionChurn, ActionPartialWithdraw, ActionFullWithdraw} {
		s := at.String()
		got, ok := ActionTypeFromString(s)
		if !ok || got != at {
			t.Errorf("round-trip failed: %d → %q → (%d, %v)", at, s, got, ok)
		}
	}
}

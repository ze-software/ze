package as112

import "testing"

// VALIDATES: buildState/loadState/storeState round-trip.
func TestState_RoundTrip(t *testing.T) {
	cfg := as112Config{Enabled: true, Hostname: "node1"}
	st := buildState(cfg, 42)
	storeState(st)
	t.Cleanup(func() { storeState(nil) })

	got := loadState()
	if got == nil || got.cfg.Hostname != "node1" || got.serial != 42 {
		t.Fatalf("loadState() = %+v, want hostname=node1 serial=42", got)
	}
}

// VALIDATES: an empty allow-from list produces a nil matcher (answer-all).
func TestState_EmptyAllowFromNilMatcher(t *testing.T) {
	st := buildState(as112Config{}, 1)
	if st.matcher != nil {
		t.Fatal("matcher != nil for empty AllowFrom, want nil")
	}
}

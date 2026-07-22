package routeaction

import "testing"

// TestActionVerb pins the producer-owned action -> forwarding-op mapping
// that the FIB backends share.
//
// VALIDATES: Action.Verb maps each action to the forwarding-plane op a FIB
// backend performs (Add->Install, Update->Replace, Withdraw/Del->Remove,
// Unspecified/unknown->Skip).
// PREVENTS: the kernel/vpp/p4 FIB backends each re-encoding that Withdraw and
// Del both mean remove and that Unspecified is a no-op -- they now derive it
// from this one method on the type they already depend on.
func TestActionVerb(t *testing.T) {
	tests := []struct {
		action Action
		want   Verb
	}{
		{Add, VerbInstall},
		{Update, VerbReplace},
		{Withdraw, VerbRemove},
		{Del, VerbRemove},
		{Unspecified, VerbSkip},
		{Action(99), VerbSkip},
	}
	for _, tt := range tests {
		if got := tt.action.Verb(); got != tt.want {
			t.Errorf("Action(%d).Verb() = %d, want %d", tt.action, got, tt.want)
		}
	}
}

// TestActionVerbNoAlloc guards the hot FIB install path: Verb is a value
// enum and must not allocate.
func TestActionVerbNoAlloc(t *testing.T) {
	if n := testing.AllocsPerRun(100, func() { _ = Withdraw.Verb() }); n != 0 {
		t.Errorf("Action.Verb allocated %v times, want 0", n)
	}
}

// TestActionTextRoundTrip pins the wire vocabulary the external plugin protocol
// depends on: every valid action marshals to its wire token and unmarshals back
// to the same value, and Unspecified is rejected rather than silently emitted.
//
// VALIDATES: MarshalText/UnmarshalText round-trip for add/del/update/withdraw;
// Unspecified fails to marshal; an unknown token fails to unmarshal.
// PREVENTS: the lift out of internal/component/bgp/types silently changing the
// JSON contract seen by external plugin processes.
func TestActionTextRoundTrip(t *testing.T) {
	want := map[Action]string{Add: "add", Del: "del", Update: "update", Withdraw: "withdraw"}
	for a, token := range want {
		text, err := a.MarshalText()
		if err != nil {
			t.Fatalf("Action(%d).MarshalText: %v", a, err)
		}
		if string(text) != token {
			t.Errorf("Action(%d).MarshalText() = %q, want %q", a, text, token)
		}
		var back Action
		if err := back.UnmarshalText(text); err != nil {
			t.Fatalf("UnmarshalText(%q): %v", text, err)
		}
		if back != a {
			t.Errorf("round-trip %q = Action(%d), want Action(%d)", text, back, a)
		}
	}
	if _, err := Unspecified.MarshalText(); err == nil {
		t.Error("Unspecified.MarshalText() = nil error, want rejection")
	}
	var bad Action
	if err := bad.UnmarshalText([]byte("bogus")); err == nil {
		t.Error("UnmarshalText(\"bogus\") = nil error, want rejection")
	}
}

// TestProtocolTypeTextRoundTrip pins the ebgp/ibgp wire tokens.
//
// VALIDATES: ProtocolType marshals to "ebgp"/"ibgp" and back; Unspecified and
// out-of-range values are rejected.
// PREVENTS: a best-change consumer silently reading a blank protocol-type.
func TestProtocolTypeTextRoundTrip(t *testing.T) {
	want := map[ProtocolType]string{ProtocolEBGP: "ebgp", ProtocolIBGP: "ibgp"}
	for p, token := range want {
		text, err := p.MarshalText()
		if err != nil {
			t.Fatalf("ProtocolType(%d).MarshalText: %v", p, err)
		}
		if string(text) != token {
			t.Errorf("ProtocolType(%d).MarshalText() = %q, want %q", p, text, token)
		}
		var back ProtocolType
		if err := back.UnmarshalText(text); err != nil {
			t.Fatalf("UnmarshalText(%q): %v", text, err)
		}
		if back != p {
			t.Errorf("round-trip %q = ProtocolType(%d), want ProtocolType(%d)", text, back, p)
		}
	}
	if _, err := ProtocolUnspecified.MarshalText(); err == nil {
		t.Error("ProtocolUnspecified.MarshalText() = nil error, want rejection")
	}
	if _, err := ProtocolCount.MarshalText(); err == nil {
		t.Error("ProtocolCount.MarshalText() = nil error, want rejection")
	}
}

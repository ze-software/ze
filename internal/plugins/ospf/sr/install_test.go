package sr

import "testing"

func TestSRPHPBehavior(t *testing.T) {
	// RFC 8665 §5 / RFC 8666 §6 outgoing-label truth table.
	cases := []struct {
		name  string
		flags SIDFlags
		want  OutgoingAction
	}{
		{"php-np-clear", SIDFlags{NP: false, E: false}, ActionPHP},
		{"php-np-clear-e-set-ignored", SIDFlags{NP: false, E: true}, ActionPHP}, // NP=0 => E ignored
		{"keep-np-set-e-clear", SIDFlags{NP: true, E: false}, ActionKeep},
		{"explicit-null-np-set-e-set", SIDFlags{NP: true, E: true}, ActionExplicitNull},
		{"m-overrides-np-e", SIDFlags{M: true, NP: false, E: true}, ActionKeep}, // M set => ignore NP/E
		{"m-overrides-keep", SIDFlags{M: true, NP: true, E: true}, ActionKeep},
	}
	for _, c := range cases {
		if got := OutgoingActionFor(c.flags); got != c.want {
			t.Fatalf("%s: OutgoingActionFor = %v want %v", c.name, got, c.want)
		}
	}
}

func TestSROutgoingLabel(t *testing.T) {
	const label uint32 = 16050
	// Keep: push/swap to the computed label.
	if l, push := OutgoingLabel(label, ActionKeep, ExplicitNullV4); !push || l != label {
		t.Fatalf("keep => %d,%v want %d,true", l, push, label)
	}
	// PHP: no label (forward as plain IP / pop).
	if _, push := OutgoingLabel(label, ActionPHP, ExplicitNullV4); push {
		t.Fatalf("PHP must not push a label")
	}
	// Explicit NULL IPv4 = 0.
	if l, push := OutgoingLabel(label, ActionExplicitNull, ExplicitNullV4); !push || l != 0 {
		t.Fatalf("explicit-null v4 => %d,%v want 0,true", l, push)
	}
	// Explicit NULL IPv6 = 2.
	if l, push := OutgoingLabel(label, ActionExplicitNull, ExplicitNullV6); !push || l != 2 {
		t.Fatalf("explicit-null v6 => %d,%v want 2,true", l, push)
	}
}

func TestSRExplicitNullConstants(t *testing.T) {
	if ExplicitNullV4 != 0 {
		t.Fatalf("IPv4 Explicit NULL must be label 0, got %d", ExplicitNullV4)
	}
	if ExplicitNullV6 != 2 {
		t.Fatalf("IPv6 Explicit NULL must be label 2, got %d", ExplicitNullV6)
	}
}

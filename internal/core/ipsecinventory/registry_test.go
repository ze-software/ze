// VALIDATES: AC-18 and A-4 of spec-path-mtu-diagnostic (an unregistered inventory
// is a named outcome, distinct from zero tunnels)
// PREVENTS: show mtu reporting NO TUNNELS on a build that never linked the IKE engine
package ipsecinventory

import (
	"errors"
	"net/netip"
	"testing"
)

// TestInventorySnapshotUnregisteredIsNotEmpty proves that a reader can tell "no
// IPsec engine is in this build" from "the engine holds no tunnel": the first answers
// ErrNotRegistered, the second answers an empty list and no error. A zero that stood
// for both would let show mtu report NO TUNNELS on a box whose engine was never
// linked (ai/rules/principles.md).
//
// MUTATION: returning nil, nil from Tunnels when no provider is registered makes the
// first assertion fail.
func TestInventorySnapshotUnregisteredIsNotEmpty(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	tunnels, err := Tunnels()
	if !errors.Is(err, ErrNotRegistered) {
		t.Fatalf("unregistered inventory answered (%v, %v), want ErrNotRegistered", tunnels, err)
	}

	Register(func() []Tunnel { return nil })
	tunnels, err = Tunnels()
	if err != nil {
		t.Fatalf("registered inventory holding no tunnel answered an error: %v", err)
	}
	if len(tunnels) != 0 {
		t.Fatalf("registered inventory holding no tunnel answered %d tunnels", len(tunnels))
	}
}

// TestRegisterRefusesASecondProvider proves the registry holds one inventory: a
// second Register is a programmer error and panics rather than replacing the first
// in silence.
func TestRegisterRefusesASecondProvider(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	want := Tunnel{Peer: "site-a", InstalledRemote: netip.MustParseAddr("198.51.100.7"), Up: true}
	Register(func() []Tunnel { return []Tunnel{want} })

	defer func() {
		if recover() == nil {
			t.Fatal("a second Register did not panic")
		}
		tunnels, err := Tunnels()
		if err != nil {
			t.Fatalf("the first provider was lost: %v", err)
		}
		if len(tunnels) != 1 || tunnels[0] != want {
			t.Fatalf("the first provider's answer changed: %+v", tunnels)
		}
	}()
	Register(func() []Tunnel { return nil })
}

// TestModeNames pins the display name of each mode so a payload never prints a number
// for the zero value.
func TestModeNames(t *testing.T) {
	cases := map[Mode]string{ModeUnspecified: "unspecified", ModeTransport: "transport", ModeTunnel: "tunnel", Mode(9): "unspecified"}
	for mode, want := range cases {
		if got := mode.String(); got != want {
			t.Errorf("Mode(%d).String() = %q, want %q", mode, got, want)
		}
	}
}

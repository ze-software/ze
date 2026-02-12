package types

import (
	"net/netip"
	"testing"
)

// TestRouteNextHop_Constructors verifies NewNextHopExplicit and NewNextHopSelf.
//
// VALIDATES: Constructors set correct Policy and Addr.
// PREVENTS: Incorrect policy assignment in constructors.
func TestRouteNextHop_Constructors(t *testing.T) {
	// NewNextHopExplicit
	addr := netip.MustParseAddr("192.0.2.1")
	explicit := NewNextHopExplicit(addr)
	if explicit.Policy != NextHopExplicit {
		t.Errorf("NewNextHopExplicit: Policy = %v, want %v", explicit.Policy, NextHopExplicit)
	}
	if explicit.Addr != addr {
		t.Errorf("NewNextHopExplicit: Addr = %v, want %v", explicit.Addr, addr)
	}

	// NewNextHopSelf
	self := NewNextHopSelf()
	if self.Policy != NextHopSelf {
		t.Errorf("NewNextHopSelf: Policy = %v, want %v", self.Policy, NextHopSelf)
	}
	if self.Addr.IsValid() {
		t.Errorf("NewNextHopSelf: Addr should be invalid, got %v", self.Addr)
	}
}

// TestRouteNextHop_ZeroValue verifies zero value is invalid.
//
// VALIDATES: Zero value RouteNextHop is not valid.
// PREVENTS: Accidental use of uninitialized RouteNextHop.
func TestRouteNextHop_ZeroValue(t *testing.T) {
	var nh RouteNextHop
	if nh.IsValid() {
		t.Error("zero value should not be valid")
	}
	if nh.Policy != NextHopUnset {
		t.Errorf("zero value Policy = %v, want %v", nh.Policy, NextHopUnset)
	}
}

// TestRouteNextHop_IsSelf verifies IsSelf() returns correct bool.
//
// VALIDATES: IsSelf() returns true only for NextHopSelf policy.
// PREVENTS: Incorrect policy detection.
func TestRouteNextHop_IsSelf(t *testing.T) {
	tests := []struct {
		name string
		nh   RouteNextHop
		want bool
	}{
		{"Self", NewNextHopSelf(), true},
		{"Explicit", NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")), false},
		{"Zero", RouteNextHop{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.nh.IsSelf(); got != tt.want {
				t.Errorf("IsSelf() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestRouteNextHop_IsExplicit verifies IsExplicit() returns correct bool.
//
// VALIDATES: IsExplicit() returns true only for NextHopExplicit policy.
// PREVENTS: Incorrect policy detection.
func TestRouteNextHop_IsExplicit(t *testing.T) {
	tests := []struct {
		name string
		nh   RouteNextHop
		want bool
	}{
		{"Explicit", NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")), true},
		{"Self", NewNextHopSelf(), false},
		{"Zero", RouteNextHop{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.nh.IsExplicit(); got != tt.want {
				t.Errorf("IsExplicit() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestRouteNextHop_IsValid verifies IsValid() logic.
//
// VALIDATES: Self=true, Explicit+valid=true, Explicit+invalid=false, Unset=false.
// PREVENTS: Invalid next-hop configurations being used.
func TestRouteNextHop_IsValid(t *testing.T) {
	tests := []struct {
		name string
		nh   RouteNextHop
		want bool
	}{
		{"Self", NewNextHopSelf(), true},
		{"Explicit+valid", NewNextHopExplicit(netip.MustParseAddr("192.0.2.1")), true},
		{"Explicit+invalid", NewNextHopExplicit(netip.Addr{}), false},
		{"Unset", RouteNextHop{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.nh.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestRouteNextHop_String verifies String() output.
//
// VALIDATES: Self="self", Explicit=IP, Unset="", Explicit+invalid="".
// PREVENTS: Incorrect string representation in logs/output.
func TestRouteNextHop_String(t *testing.T) {
	tests := []struct {
		name string
		nh   RouteNextHop
		want string
	}{
		{"Self", NewNextHopSelf(), "self"},
		{"Explicit IPv4", NewNextHopExplicit(netip.MustParseAddr("192.0.2.1")), "192.0.2.1"},
		{"Explicit IPv6", NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")), "2001:db8::1"},
		{"Explicit invalid", NewNextHopExplicit(netip.Addr{}), ""},
		{"Unset", RouteNextHop{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.nh.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

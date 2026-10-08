package mplsfib

import "testing"

// TestPathMTUFloorDerivation checks that the path MTU floor is the IPv4
// minimum MTU plus the deepest label stack Ze installs, and that a frame of
// exactly that size, carrying that stack, still leaves room for the largest
// IPv4 header and one eight-octet fragment.
// PREVENTS: a push route whose metric leaves IPv4 fragmentation less than
// eight data octets, which a stock Linux kernel turns into an endless stream
// of empty fragments inside softirq (docs/architecture/mpls/mpls-kernel.md).
func TestPathMTUFloorDerivation(t *testing.T) {
	const (
		ipv4HeaderMaxOctets   = 60
		fragmentMinOctets     = 8
		labelStackEntryOctets = 4
	)
	if MaxLabelStack != 16 {
		t.Fatalf("MaxLabelStack = %d, want 16", MaxLabelStack)
	}
	want := uint32(ipv4HeaderMaxOctets + fragmentMinOctets + labelStackEntryOctets*MaxLabelStack)
	if PathMTUMinimum != want {
		t.Fatalf("PathMTUMinimum = %d, want %d", PathMTUMinimum, want)
	}
	inner := PathMTUMinimum - labelStackEntryOctets*MaxLabelStack
	if inner-ipv4HeaderMaxOctets != fragmentMinOctets {
		t.Fatalf("a floor-sized frame under %d labels leaves %d data octets after a full IPv4 header, want %d",
			MaxLabelStack, inner-ipv4HeaderMaxOctets, fragmentMinOctets)
	}
}

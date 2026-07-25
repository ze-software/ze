package sysrib

import (
	"net/netip"
	"testing"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
)

func TestECMPCollect_SingleRoute(t *testing.T) {
	winner := &protocolRoute{
		protocol: "bgp", nextHop: netip.MustParseAddr("10.0.0.1"), priority: 20, metric: 0,
	}
	protocols := map[string]*protocolRoute{"bgp": winner}
	paths := ecmpCollect(protocols, winner)
	if paths != nil {
		t.Errorf("expected nil for single route, got %v", paths)
	}
}

func TestECMPCollect_TwoEqualCost(t *testing.T) {
	winner := &protocolRoute{
		protocol: "bgp-peer1", nextHop: netip.MustParseAddr("10.0.0.1"), priority: 20, metric: 100,
	}
	peer2 := &protocolRoute{
		protocol: "bgp-peer2", nextHop: netip.MustParseAddr("10.0.0.2"), priority: 20, metric: 100,
	}
	protocols := map[string]*protocolRoute{"bgp-peer1": winner, "bgp-peer2": peer2}
	paths := ecmpCollect(protocols, winner)
	if len(paths) != 1 {
		t.Fatalf("expected 1 ECMP path, got %d", len(paths))
	}
	if paths[0].NextHop != netip.MustParseAddr("10.0.0.2") {
		t.Errorf("ECMP path NextHop = %v, want 10.0.0.2", paths[0].NextHop)
	}
}

func TestECMPCollect_DifferentPriority(t *testing.T) {
	winner := &protocolRoute{
		protocol: "bgp", nextHop: netip.MustParseAddr("10.0.0.1"), priority: 20, metric: 0,
	}
	other := &protocolRoute{
		protocol: "static", nextHop: netip.MustParseAddr("10.0.0.2"), priority: 1, metric: 0,
	}
	protocols := map[string]*protocolRoute{"bgp": winner, "static": other}
	paths := ecmpCollect(protocols, winner)
	if paths != nil {
		t.Errorf("expected nil for different priority, got %v", paths)
	}
}

func TestECMPCollect_DifferentMetric(t *testing.T) {
	winner := &protocolRoute{
		protocol: "bgp-peer1", nextHop: netip.MustParseAddr("10.0.0.1"), priority: 20, metric: 50,
	}
	other := &protocolRoute{
		protocol: "bgp-peer2", nextHop: netip.MustParseAddr("10.0.0.2"), priority: 20, metric: 100,
	}
	protocols := map[string]*protocolRoute{"bgp-peer1": winner, "bgp-peer2": other}
	paths := ecmpCollect(protocols, winner)
	if paths != nil {
		t.Errorf("expected nil for different metric, got %v", paths)
	}
}

func TestECMPCollect_ThreeEqualCost(t *testing.T) {
	winner := &protocolRoute{
		protocol: "peer1", nextHop: netip.MustParseAddr("10.0.0.1"), priority: 20, metric: 0,
	}
	peer2 := &protocolRoute{
		protocol: "peer2", nextHop: netip.MustParseAddr("10.0.0.2"), priority: 20, metric: 0,
	}
	peer3 := &protocolRoute{
		protocol: "peer3", nextHop: netip.MustParseAddr("10.0.0.3"), priority: 20, metric: 0,
	}
	protocols := map[string]*protocolRoute{"peer1": winner, "peer2": peer2, "peer3": peer3}
	paths := ecmpCollect(protocols, winner)
	if len(paths) != 2 {
		t.Fatalf("expected 2 ECMP paths, got %d", len(paths))
	}
}

func TestECMPCollect_WithLabels(t *testing.T) {
	winner := &protocolRoute{
		protocol: "peer1", nextHop: netip.MustParseAddr("10.0.0.1"), priority: 20, metric: 0, labels: []uint32{100},
	}
	peer2 := &protocolRoute{
		protocol: "peer2", nextHop: netip.MustParseAddr("10.0.0.2"), priority: 20, metric: 0, labels: []uint32{200},
	}
	protocols := map[string]*protocolRoute{"peer1": winner, "peer2": peer2}
	paths := ecmpCollect(protocols, winner)
	if len(paths) != 1 {
		t.Fatalf("expected 1 ECMP path, got %d", len(paths))
	}
	if len(paths[0].Labels) != 1 || paths[0].Labels[0] != 200 {
		t.Errorf("ECMP path Labels = %v, want [200]", paths[0].Labels)
	}
}

func TestECMPChanged_BothNil(t *testing.T) {
	if ecmpChanged(nil, nil) {
		t.Error("two nil sets should not be changed")
	}
}

func TestECMPChanged_AddedPath(t *testing.T) {
	a := []sysribevents.ECMPPath{{NextHop: netip.MustParseAddr("10.0.0.1")}}
	if !ecmpChanged(nil, a) {
		t.Error("nil vs non-nil should be changed")
	}
}

func TestECMPChanged_DifferentNH(t *testing.T) {
	a := []sysribevents.ECMPPath{{NextHop: netip.MustParseAddr("10.0.0.1")}}
	b := []sysribevents.ECMPPath{{NextHop: netip.MustParseAddr("10.0.0.2")}}
	if !ecmpChanged(a, b) {
		t.Error("different next-hops should be changed")
	}
}

func TestECMPChanged_SameNH(t *testing.T) {
	a := []sysribevents.ECMPPath{{NextHop: netip.MustParseAddr("10.0.0.1")}}
	b := []sysribevents.ECMPPath{{NextHop: netip.MustParseAddr("10.0.0.1")}}
	if ecmpChanged(a, b) {
		t.Error("same next-hops should not be changed")
	}
}

// TestECMPChanged_LabelOnly is the isis-9 regression: a relabel of the same
// multipath (identical next-hops, new MPLS label stack) MUST be reported as a
// change so the kernel's stale label stack is replaced. The previous
// next-hop-only comparison silently dropped it.
func TestECMPChanged_LabelOnly(t *testing.T) {
	a := []sysribevents.ECMPPath{{NextHop: netip.MustParseAddr("10.0.0.1"), Labels: []uint32{100}}}
	b := []sysribevents.ECMPPath{{NextHop: netip.MustParseAddr("10.0.0.1"), Labels: []uint32{200}}}
	if !ecmpChanged(a, b) {
		t.Error("same next-hop but different label stack should be changed")
	}
}

// TestECMPChanged_LabelAddedToUnlabeled covers the unlabeled -> labeled
// transition (kernel previously had no label stack; now it must push one).
func TestECMPChanged_LabelAddedToUnlabeled(t *testing.T) {
	a := []sysribevents.ECMPPath{{NextHop: netip.MustParseAddr("10.0.0.1")}}
	b := []sysribevents.ECMPPath{{NextHop: netip.MustParseAddr("10.0.0.1"), Labels: []uint32{300}}}
	if !ecmpChanged(a, b) {
		t.Error("unlabeled -> labeled should be changed")
	}
}

// TestECMPChanged_WeightOnly covers an unequal-cost-multipath weight change on
// an otherwise identical member (same next-hop and labels).
func TestECMPChanged_WeightOnly(t *testing.T) {
	a := []sysribevents.ECMPPath{{NextHop: netip.MustParseAddr("10.0.0.1"), Weight: 1}}
	b := []sysribevents.ECMPPath{{NextHop: netip.MustParseAddr("10.0.0.1"), Weight: 2}}
	if !ecmpChanged(a, b) {
		t.Error("same next-hop but different weight should be changed")
	}
}

// TestECMPChanged_SameFullPath verifies that identical full paths (next-hop,
// weight, and labels all equal) are NOT reported as changed, regardless of
// input order: ecmpChanged sorts defensive copies before comparing.
func TestECMPChanged_SameFullPath(t *testing.T) {
	a := []sysribevents.ECMPPath{
		{NextHop: netip.MustParseAddr("10.0.0.2"), Weight: 1, Labels: []uint32{200}},
		{NextHop: netip.MustParseAddr("10.0.0.1"), Weight: 1, Labels: []uint32{100}},
	}
	b := []sysribevents.ECMPPath{
		{NextHop: netip.MustParseAddr("10.0.0.1"), Weight: 1, Labels: []uint32{100}},
		{NextHop: netip.MustParseAddr("10.0.0.2"), Weight: 1, Labels: []uint32{200}},
	}
	if ecmpChanged(a, b) {
		t.Error("identical full paths in different order should not be changed")
	}
}

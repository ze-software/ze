package static

import (
	"net/netip"
	"testing"
)

func pfx(s string) netip.Prefix { return netip.MustParsePrefix(s) }
func addr(s string) netip.Addr  { return netip.MustParseAddr(s) }

func TestRoutesEqualIdentical(t *testing.T) {
	a := staticRoute{
		Prefix: pfx("10.0.0.0/8"), Action: actionForward, Metric: 10, Tag: 1,
		NextHops: []nextHop{{Address: addr("1.1.1.1"), Weight: 1}},
	}
	if !routesEqual(a, a) {
		t.Error("identical routes should be equal")
	}
}

func TestRoutesEqualNextHopChange(t *testing.T) {
	a := staticRoute{Prefix: pfx("10.0.0.0/8"), Action: actionForward, NextHops: []nextHop{{Address: addr("1.1.1.1"), Weight: 1}}}
	b := staticRoute{Prefix: pfx("10.0.0.0/8"), Action: actionForward, NextHops: []nextHop{{Address: addr("2.2.2.2"), Weight: 1}}}
	if routesEqual(a, b) {
		t.Error("different next-hop should not be equal")
	}
}

func TestRoutesEqualWeightChange(t *testing.T) {
	a := staticRoute{Prefix: pfx("10.0.0.0/8"), Action: actionForward, NextHops: []nextHop{{Address: addr("1.1.1.1"), Weight: 1}}}
	b := staticRoute{Prefix: pfx("10.0.0.0/8"), Action: actionForward, NextHops: []nextHop{{Address: addr("1.1.1.1"), Weight: 5}}}
	if routesEqual(a, b) {
		t.Error("different weight should not be equal")
	}
}

func TestRoutesEqualNHAdded(t *testing.T) {
	a := staticRoute{Prefix: pfx("10.0.0.0/8"), Action: actionForward, NextHops: []nextHop{{Address: addr("1.1.1.1"), Weight: 1}}}
	b := staticRoute{Prefix: pfx("10.0.0.0/8"), Action: actionForward, NextHops: []nextHop{
		{Address: addr("1.1.1.1"), Weight: 1},
		{Address: addr("2.2.2.2"), Weight: 1},
	}}
	if routesEqual(a, b) {
		t.Error("added NH should not be equal")
	}
}

func TestRoutesEqualNHOrderIndependent(t *testing.T) {
	a := staticRoute{Prefix: pfx("10.0.0.0/8"), Action: actionForward, NextHops: []nextHop{
		{Address: addr("2.2.2.2"), Weight: 1},
		{Address: addr("1.1.1.1"), Weight: 3},
	}}
	b := staticRoute{Prefix: pfx("10.0.0.0/8"), Action: actionForward, NextHops: []nextHop{
		{Address: addr("1.1.1.1"), Weight: 3},
		{Address: addr("2.2.2.2"), Weight: 1},
	}}
	if !routesEqual(a, b) {
		t.Error("same NHs in different order should be equal")
	}
}

func TestRoutesEqualBlackholes(t *testing.T) {
	a := staticRoute{Prefix: pfx("10.0.0.0/8"), Action: actionBlackhole}
	b := staticRoute{Prefix: pfx("10.0.0.0/8"), Action: actionBlackhole}
	if !routesEqual(a, b) {
		t.Error("identical blackholes should be equal")
	}
}

func TestRoutesEqualActionDiffers(t *testing.T) {
	a := staticRoute{Prefix: pfx("10.0.0.0/8"), Action: actionBlackhole}
	b := staticRoute{Prefix: pfx("10.0.0.0/8"), Action: actionReject}
	if routesEqual(a, b) {
		t.Error("different actions should not be equal")
	}
}

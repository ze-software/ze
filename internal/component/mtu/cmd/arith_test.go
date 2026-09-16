// Design: docs/architecture/diagnostics/path-mtu.md -- the arithmetic tests
// Related: arith.go -- ceiling, recommended, mss
//
// VALIDATES: the ceiling lands on the cipher boundary less the ESP trailer,
// the recommended value is absent rather than small below the inner family's
// minimum, and the MSS is absent when the headers leave no room.
// PREVENTS: a `set ... mtu 0` or a sub-minimum recommendation reaching the
// remediation list, and tunnel-mode arithmetic applied to a transport SA.

package cmd

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/ipsecinventory"
)

// gcmV4 is AES-GCM over an IPv4 endpoint without UDP encapsulation: 52 octets
// of overhead on the 4-octet ESP boundary (AC-2).
var gcmV4 = espOverhead{mode: ipsecinventory.ModeTunnel, outerHeader: 20, fixed: 32, block: 4}

// cbcV4 is AES-CBC with HMAC-SHA-256 over IPv4: 60 octets on the 16-octet
// AES block.
var cbcV4 = espOverhead{mode: ipsecinventory.ModeTunnel, outerHeader: 20, fixed: 40, block: 16}

// TestCeilingAlignsAndSubtractsTrailer pins the ceiling at hand-computed
// values: the room after the overhead is aligned DOWN to the cipher block,
// then the two trailer octets come off, and a path with no room answers 0.
func TestCeilingAlignsAndSubtractsTrailer(t *testing.T) {
	cases := []struct {
		name     string
		underlay uint16
		overhead espOverhead
		want     uint16
	}{
		{"gcm 1500: 1448 is aligned already", 1500, gcmV4, 1446},
		{"gcm 1499: 1447 aligns down to 1444", 1499, gcmV4, 1442},
		{"gcm udp 1500: 8 fewer, ceiling 8 lower (AC-3)", 1500, espOverhead{mode: ipsecinventory.ModeTunnel, outerHeader: 20, fixed: 40, block: 4}, 1438},
		{"cbc 1500: 1440 on the 16 boundary", 1500, cbcV4, 1438},
		{"cbc 1501: 1441 aligns down to 1440", 1501, cbcV4, 1438},
		{"cbc 1455: 1395 aligns down to 1392", 1455, cbcV4, 1390},
		{"underlay equal to the overhead: nothing left", 52, gcmV4, 0},
		{"room below one block aligns to 0", 55, gcmV4, 0},
		{"room of one block leaves 2 above the trailer", 56, gcmV4, 2},
		{"room of two blocks leaves 6 above the trailer", 60, gcmV4, 6},
		{"transport gcm 1500: the packet keeps its own header", 1500, espOverhead{mode: ipsecinventory.ModeTransport, outerHeader: 20, fixed: 32, block: 4}, 1466},
		{"transport cbc ipv6 1500: 1420 aligns to 1408, less 2, plus 40", 1500, espOverhead{mode: ipsecinventory.ModeTransport, outerHeader: 40, fixed: 40, block: 16}, 1446},
	}
	for _, c := range cases {
		got := ceiling(c.underlay, &c.overhead)
		if got != c.want {
			t.Errorf("%s: ceiling(%d) = %d, want %d", c.name, c.underlay, got, c.want)
		}
	}
}

// TestRecommendedAbsentBelowMinimumLinkMTU pins the floor at its exact value
// for both inner families (Boundary Tests: 1280 valid, 1279 absent for inner
// IPv6; 576 valid, 575 absent for inner IPv4), proves the back-off lands on
// the cipher boundary, and proves a zero ceiling is absent rather than 0.
// The exact-value rows use a 2-octet block so the arithmetic can land on the
// floor itself; neither real cipher block can produce 1280 or 576, so the
// nearest values each block does produce are pinned beside them.
func TestRecommendedAbsentBelowMinimumLinkMTU(t *testing.T) {
	block2 := espOverhead{mode: ipsecinventory.ModeTunnel, outerHeader: 20, fixed: 32, block: 2}
	cases := []struct {
		name     string
		ceiling  uint16
		overhead espOverhead
		inner    ipFamily
		want     uint16
		absent   bool
	}{
		{"gcm 1446 backs off 32 to 1414", 1446, gcmV4, ipFamilyV6, 1414, false},
		{"cbc 1438 backs off 32 to 1406", 1438, cbcV4, ipFamilyV6, 1406, false},
		{"inner ipv6 lands exactly on 1280", 1312, block2, ipFamilyV6, 1280, false},
		{"inner ipv6 lands on 1279: absent", 1311, block2, ipFamilyV6, 0, true},
		{"inner ipv6 block 4 lands on 1282", 1314, gcmV4, ipFamilyV6, 1282, false},
		{"inner ipv6 block 4 lands on 1278: absent", 1312, gcmV4, ipFamilyV6, 0, true},
		{"inner ipv4 lands exactly on 576", 608, block2, ipFamilyV4, 576, false},
		{"inner ipv4 lands on 575: absent", 607, block2, ipFamilyV4, 0, true},
		{"inner ipv4 block 16 lands on 590", 622, cbcV4, ipFamilyV4, 590, false},
		{"inner ipv4 block 16 lands on 574: absent", 606, cbcV4, ipFamilyV4, 0, true},
		{"a ceiling of 0 is absent", 0, gcmV4, ipFamilyV6, 0, true},
		{"a ceiling below the margin is absent", 20, gcmV4, ipFamilyV4, 0, true},
		{"a ceiling of exactly the margin is absent", 32, gcmV4, ipFamilyV4, 0, true},
	}
	for _, c := range cases {
		got, err := recommended(c.ceiling, &c.overhead, c.inner)
		if c.absent {
			if !errors.Is(err, errNoUsableMTU) {
				t.Errorf("%s: recommended(%d) = %d, %v; want errNoUsableMTU", c.name, c.ceiling, got, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: recommended(%d): %v", c.name, c.ceiling, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: recommended(%d) = %d, want %d", c.name, c.ceiling, got, c.want)
		}
	}
	if _, err := recommended(1446, &gcmV4, ipFamilyUnspecified); err == nil {
		t.Error("an unspecified inner family was answered with a value")
	}
}

// TestFamilyOf pins the family of an address, and Unspecified for an invalid
// one.
func TestFamilyOf(t *testing.T) {
	if got := familyOf(netip.MustParseAddr("192.0.2.1")); got != ipFamilyV4 {
		t.Errorf("v4: %d", got)
	}
	if got := familyOf(netip.MustParseAddr("2001:db8::1")); got != ipFamilyV6 {
		t.Errorf("v6: %d", got)
	}
	if got := familyOf(netip.Addr{}); got != ipFamilyUnspecified {
		t.Errorf("invalid: %d", got)
	}
}

// TestMSSAbsentWhenNoRoom pins the MSS as the MTU less the inner header and
// the TCP header, absent when that is not positive.
func TestMSSAbsentWhenNoRoom(t *testing.T) {
	cases := []struct {
		mtu    uint16
		inner  ipFamily
		want   uint16
		absent bool
	}{
		{1414, ipFamilyV6, 1354, false},
		{1414, ipFamilyV4, 1374, false},
		{61, ipFamilyV6, 1, false},
		{60, ipFamilyV6, 0, true},
		{41, ipFamilyV4, 1, false},
		{40, ipFamilyV4, 0, true},
		{1414, ipFamilyUnspecified, 0, true},
	}
	for _, c := range cases {
		got, err := mss(c.mtu, c.inner)
		if c.absent {
			if err == nil {
				t.Errorf("mss(%d, %d) = %d, want absent", c.mtu, c.inner, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("mss(%d, %s): %v", c.mtu, c.inner, err)
			continue
		}
		if got != c.want {
			t.Errorf("mss(%d, %s) = %d, want %d", c.mtu, c.inner, got, c.want)
		}
	}
}

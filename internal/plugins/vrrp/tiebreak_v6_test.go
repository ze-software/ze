// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- equal-priority election
// Related: tiebreak_test.go -- the IPv4 election cases
//
// VALIDATES: an IPv6 Active router elects against its link-local
// advertisement source with an unsigned byte order, and a winning
// advertisement performs every step RFC 9568 Section 6.4.3 lists.
// PREVENTS: a signed octet compare, which ranks fe80::80 below fe80::7f.
package vrrp

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
	"github.com/ze-software/ze/internal/plugins/vrrp/transport"
)

// v6AdvertItem encodes a VRRPv3 IPv6 advertisement for spec's VRID and
// addresses, at priority and interval, from source. Every call builds its own
// buffer, so no case sees another case's bytes.
func v6AdvertItem(spec GroupSpec, priority uint8, intervalMS uint32, source netip.Addr) transport.RxItem {
	adv := packet.Advertisement{
		Version:         packet.VersionV3,
		Family:          packet.V6,
		VRID:            spec.VRID,
		Priority:        priority,
		AdverIntervalMS: intervalMS,
		VIPs:            spec.VIPs,
	}
	buf := make([]byte, packet.MaxLenV3v6)
	n := adv.WriteTo(buf, 0)
	packet.FillChecksum(buf, 0, n, source, packet.MulticastV6)
	meta := packet.RxMeta{TTL: 255, Family: packet.V6, Src: source, Dst: packet.MulticastV6}
	return transport.RxItem{Meta: meta, Payload: buf[:n]}
}

// TestInstanceIPv6ElectionUsesUnsignedOrder proves the IPv6 election of an
// Active router at priority 200 whose advertisements leave from a link-local
// address.
//
// Method: each pair of addresses differs only in the last octet, one side at
// 0x7f and the other at 0x80, so an unsigned compare and a signed one give
// opposite answers. A winning advertisement carries a 3000ms interval, unlike
// the local 1000ms, so the adopted interval, the skew time and the down
// interval show whether the demotion used the advertisement's values.
//
// RFC requirement: RFC9568-6.4.3-11 positive -- for an IPv6 Active router whose advertisement source is link-local, a higher-priority advertisement, or an equal-priority one from a sender greater under an unsigned byte compare (fe80::80 against fe80::7f), cancels the advertisement timer, adopts the advertisement's 3000ms interval, recomputes the skew time and the down interval from it, arms the down timer, and transitions to Backup (masterAdvert, demoteToBackup fsm/fsm.go)
// RFC requirement: RFC9568-6.4.3-11 negative -- an equal-priority advertisement from a sender smaller under the unsigned compare (fe80::7f against fe80::80), which a signed octet compare ranks greater, leaves the router Active with its advertisement timer armed and its own 1000ms interval (senderWinsTieBreak fsm/fsm.go)
// RFC requirement: RFC5798-6.4.3-11 positive -- for an IPv6 Master whose advertisement source is link-local, a higher-priority advertisement, or an equal-priority one from a sender greater under an unsigned byte compare (fe80::80 against fe80::7f), cancels the advertisement timer, adopts the advertisement's 3000ms interval, recomputes the skew time and the down interval from it, arms the down timer, and transitions to Backup (masterAdvert, demoteToBackup fsm/fsm.go)
// RFC requirement: RFC5798-6.4.3-11 negative -- an equal-priority advertisement from a sender smaller under the unsigned compare (fe80::7f against fe80::80), which a signed octet compare ranks greater, leaves the Master in place with its advertisement timer armed and its own 1000ms interval (senderWinsTieBreak fsm/fsm.go).
func TestInstanceIPv6ElectionUsesUnsignedOrder(t *testing.T) {
	low := netip.MustParseAddr("fe80::7f")
	high := netip.MustParseAddr("fe80::80")
	for _, tc := range []struct {
		name     string
		source   netip.Addr
		sender   netip.Addr
		priority uint8
		yields   bool
	}{
		{"equal priority, sender above in the high octet", low, high, 200, true},
		{"higher priority, sender below", high, low, 250, true},
		{"equal priority, sender below in the high octet", high, low, 200, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := testSpecV6()
			in, f, clk := newTestInstance(t, spec)
			f.source = tc.source
			in.evaluateReadiness()
			promoteToActive(t, in, clk)

			deliverAdvert(in, v6AdvertItem(spec, tc.priority, 3000, tc.sender))
			snap := in.machine.Snapshot()
			if !tc.yields {
				if snap.State != fsm.StateMaster {
					t.Fatalf("source %s, sender %s: state = %v, want Master", tc.source, tc.sender, snap.State)
				}
				if !snap.AdvertArmed || snap.ActiveIntervalMs != 1000 {
					t.Fatalf("a kept Master must keep its advertisement timer and interval: armed = %v, interval = %dms",
						snap.AdvertArmed, snap.ActiveIntervalMs)
				}
				return
			}
			if snap.State != fsm.StateBackup {
				t.Fatalf("source %s, sender %s, priority %d: state = %v, want Backup", tc.source, tc.sender, tc.priority, snap.State)
			}
			if snap.AdvertArmed {
				t.Fatal("the advertisement timer is still armed after the demotion")
			}
			if snap.ActiveIntervalMs != 3000 {
				t.Fatalf("active interval = %dms, want the advertisement's 3000ms", snap.ActiveIntervalMs)
			}
			// RFC 9568 Section 6.1: Skew_Time = ((256 - Priority) * Active_Adver_Interval) / 256,
			// and Active_Down_Interval = (3 * Active_Adver_Interval) + Skew_Time.
			skew := time.Duration(256-int(spec.Priority)) * 3000 * time.Millisecond / 256
			if snap.SkewTime != skew {
				t.Fatalf("skew time = %s, want %s from the advertisement's interval", snap.SkewTime, skew)
			}
			if want := 3*3000*time.Millisecond + skew; snap.MasterDownInterval != want {
				t.Fatalf("down interval = %s, want %s", snap.MasterDownInterval, want)
			}
			if !snap.MasterDownArmed {
				t.Fatal("the down timer is not armed after the demotion")
			}
		})
	}
}

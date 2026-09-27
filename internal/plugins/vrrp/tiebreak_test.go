// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- equal-priority election
// Related: instance.go -- syncSourceLocked, the tie-break operand
//
// VALIDATES: the equal-priority election compares the sender with the address
// this router's advertisements leave from, follows a change of that address,
// and yields by name when no address is known.
// PREVENTS: a first-virtual-address operand that lets two equal-priority
// routers both stay Master.
package vrrp

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
	"github.com/ze-software/ze/internal/plugins/vrrp/transport"
)

// tieBreakVIP is the virtual address of every case below. It sits between the
// two parent addresses the cases use, so the first virtual address and the
// advertisement source give opposite answers to the same comparison.
var tieBreakVIP = netip.MustParseAddr("192.0.2.50")

// activeTieBreakInstance builds a Master at priority 200 for version whose
// fake transport sends from source.
func activeTieBreakInstance(t *testing.T, version uint8, source netip.Addr) (*instance, *fakeDeps) {
	t.Helper()
	spec := testSpec()
	spec.Version = version
	spec.VIPs = []netip.Addr{tieBreakVIP}
	in, f, clk := newTestInstance(t, spec)
	f.source = source
	in.evaluateReadiness()
	promoteToActive(t, in, clk)
	return in, f
}

// deliverAdvert hands one received datagram to the instance and runs every
// event it queued through the FSM, as the worker would.
func deliverAdvert(in *instance, item transport.RxItem) {
	in.onPacket(item)
	for {
		select {
		case ev := <-in.events:
			in.dispatch(ev)
		default:
			return
		}
	}
}

// equalPriorityAdvert encodes a Priority 200 advertisement for tieBreakVIP
// from sender, in the given version.
func equalPriorityAdvert(t *testing.T, version uint8, sender netip.Addr) transport.RxItem {
	t.Helper()
	if version == versionV2 {
		return v2AdvertItem(t, 200, sender, tieBreakVIP)
	}
	return v3AdvertItem(t, 200, sender, tieBreakVIP)
}

// TestInstanceTieBreakComparesTheAdvertisementSource proves the Master's
// equal-priority election compares the sender with the address its own
// advertisements leave from, and not with its first virtual address.
//
// Method: the fake transport sends from a parent address on one side of the
// virtual address, and an equal-priority advertisement arrives from a sender
// between the two. With the source below the sender the Master yields; with the
// source above it, the Master keeps the role. A first-virtual-address operand
// answers both cases the other way round, so each case goes red under it.
//
// RFC requirement: RFC3768-6.4.3-8 positive -- an equal-priority VRRPv2 advertisement from a sender greater than the local advertisement source (the parent's primary address the transport sends from, not the first virtual address) demotes the Master to Backup (syncSourceLocked instance.go, senderWinsTieBreak fsm.go).
// RFC requirement: RFC3768-6.4.3-8 negative -- an equal-priority VRRPv2 advertisement from a sender smaller than the local advertisement source leaves the Master in place, even though the sender is greater than the first virtual address (syncSourceLocked instance.go).
// RFC requirement: RFC9568-6.4.3-11 positive -- an equal-priority VRRPv3 advertisement from a sender greater than the local advertisement source demotes the Active router to Backup (syncSourceLocked instance.go, senderWinsTieBreak fsm.go).
// RFC requirement: RFC9568-6.4.3-11 negative -- an equal-priority VRRPv3 advertisement from a sender smaller than the local advertisement source leaves the Active router in place, even though the sender is greater than the first virtual address (syncSourceLocked instance.go).
// RFC requirement: RFC5798-6.4.3-11 positive -- an equal-priority VRRPv3 advertisement from a sender greater than the local advertisement source demotes the Master to Backup (syncSourceLocked instance.go, senderWinsTieBreak fsm.go).
// RFC requirement: RFC5798-6.4.3-11 negative -- an equal-priority VRRPv3 advertisement from a sender smaller than the local advertisement source leaves the Master in place, even though the sender is greater than the first virtual address (syncSourceLocked instance.go).
func TestInstanceTieBreakComparesTheAdvertisementSource(t *testing.T) {
	for _, tc := range []struct {
		name    string
		version uint8
		source  string
		sender  string
		want    fsm.State
	}{
		{"v2 source below sender yields", versionV2, "192.0.2.2", "192.0.2.8", fsm.StateBackup},
		{"v2 source above sender holds", versionV2, "192.0.2.100", "192.0.2.80", fsm.StateMaster},
		{"v3 source below sender yields", versionV3, "192.0.2.2", "192.0.2.8", fsm.StateBackup},
		{"v3 source above sender holds", versionV3, "192.0.2.100", "192.0.2.80", fsm.StateMaster},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in, _ := activeTieBreakInstance(t, tc.version, netip.MustParseAddr(tc.source))
			deliverAdvert(in, equalPriorityAdvert(t, tc.version, netip.MustParseAddr(tc.sender)))
			if got := in.machine.State(); got != tc.want {
				t.Fatalf("source %s, sender %s, virtual address %s: state = %v, want %v",
					tc.source, tc.sender, tieBreakVIP, got, tc.want)
			}
		})
	}
}

// TestInstanceTieBreakFollowsASourceChange proves an address change on the
// parent reaches the election: the operand is re-read from the transport, not
// captured once at startup.
//
// Method: the Master starts sending from 192.0.2.2, then the transport moves to
// 192.0.2.100. An equal-priority advertisement from 192.0.2.80 must then lose,
// which it would win against the address captured at startup.
func TestInstanceTieBreakFollowsASourceChange(t *testing.T) {
	in, f := activeTieBreakInstance(t, versionV2, netip.MustParseAddr("192.0.2.2"))
	f.mu.Lock()
	f.source = netip.MustParseAddr("192.0.2.100")
	f.mu.Unlock()

	deliverAdvert(in, equalPriorityAdvert(t, versionV2, netip.MustParseAddr("192.0.2.80")))
	if got := in.machine.State(); got != fsm.StateMaster {
		t.Fatalf("after the source moved to 192.0.2.100, an advert from 192.0.2.80 must lose: state = %v, want Master", got)
	}
}

// TestInstanceTieBreakWithNoSourceYields proves a Master whose transport knows
// no advertisement source names that state and yields the election, rather
// than comparing the zero address.
//
// Method: the fake transport reports no source after the Master is up. An
// equal-priority advertisement from the smallest address the cases use must
// demote it, and the engine must log that it has no source.
func TestInstanceTieBreakWithNoSourceYields(t *testing.T) {
	logBuf := captureVRRPLog(t)
	in, f := activeTieBreakInstance(t, versionV2, netip.MustParseAddr("192.0.2.100"))
	f.mu.Lock()
	f.sourceUnknown = true
	f.mu.Unlock()

	deliverAdvert(in, equalPriorityAdvert(t, versionV2, netip.MustParseAddr("192.0.2.2")))
	if got := in.machine.State(); got != fsm.StateBackup {
		t.Fatalf("with no advertisement source the Master must yield: state = %v, want Backup", got)
	}
	if !strings.Contains(logBuf.String(), "no advertisement source address") {
		t.Fatalf("the missing source must be logged:\n%s", logBuf.String())
	}
}

// drainEvents runs every event the instance queued through the FSM. The fake
// clock fires its timers synchronously inside Add, so after an Add every
// expiry it caused is already queued.
func drainEvents(in *instance) {
	for {
		select {
		case ev := <-in.events:
			in.dispatch(ev)
		default:
			return
		}
	}
}

// TestInstanceV2TieBreakDemotionRestartsTheTimers proves the whole action list
// a VRRPv2 Master runs when it loses the equal-priority election, not only the
// transition: the Adver_Timer is canceled and the Master_Down_Timer is set to
// Master_Down_Interval.
//
// Method: a Master at priority 200 sending from 192.0.2.2 hears an
// equal-priority advertisement from 192.0.2.8 and demotes. Its advertisement
// timer must then be gone, and two advertisement intervals of fake time must
// pass without a single advertisement leaving. The master-down interval is
// derived here from RFC 3768 Section 6.1, not from the code under test:
// 3 * 1 s + (256 - 200) / 256 s = 3.21875 s. One millisecond before that,
// counted from the demotion, the router must still be Backup; one millisecond
// after, with no advertisement heard, it must be Master again. A timer left at
// its old value, a timer never armed, or an advertisement timer still running
// each fails one step.
//
// RFC requirement: RFC3768-6.4.3-8 positive -- after an equal-priority VRRPv2 advertisement from a sender greater than the local advertisement source demotes the Master, the Adver_Timer is canceled (no timer armed, no advertisement over two intervals) and the Master_Down_Timer is set to Master_Down_Interval, 3.21875 s at priority 200 (Backup 1 ms before, Master 1 ms after) (demoteToBackup fsm.go, executeAction instance.go).
func TestInstanceV2TieBreakDemotionRestartsTheTimers(t *testing.T) {
	const masterDownInterval = 3*time.Second + 56*time.Second/256

	spec := testSpec()
	spec.Version = versionV2
	spec.VIPs = []netip.Addr{tieBreakVIP}
	in, f, clk := newTestInstance(t, spec)
	f.source = netip.MustParseAddr("192.0.2.2")
	in.evaluateReadiness()
	promoteToActive(t, in, clk)

	deliverAdvert(in, equalPriorityAdvert(t, versionV2, netip.MustParseAddr("192.0.2.8")))
	if got := in.machine.State(); got != fsm.StateBackup {
		t.Fatalf("equal priority from a greater sender: state = %v, want Backup", got)
	}
	if in.advert != nil {
		t.Fatal("the Adver_Timer is still armed after the demotion")
	}
	if in.masterDown == nil {
		t.Fatal("no Master_Down_Timer is armed after the demotion")
	}

	advertsAtDemotion := len(f.snapshot().adverts)
	clk.Add(2 * time.Duration(spec.AdvertIntervalMs) * time.Millisecond)
	drainEvents(in)
	if got := len(f.snapshot().adverts); got != advertsAtDemotion {
		t.Fatalf("%d advertisement(s) left the demoted router over two intervals, want none", got-advertsAtDemotion)
	}

	clk.Add(masterDownInterval - 2*time.Duration(spec.AdvertIntervalMs)*time.Millisecond - time.Millisecond)
	drainEvents(in)
	if got := in.machine.State(); got != fsm.StateBackup {
		t.Fatalf("1 ms before Master_Down_Interval (%v) elapsed: state = %v, want Backup", masterDownInterval, got)
	}

	clk.Add(2 * time.Millisecond)
	drainEvents(in)
	if got := in.machine.State(); got != fsm.StateMaster {
		t.Fatalf("1 ms after Master_Down_Interval (%v) with no advertisement heard: state = %v, want Master", masterDownInterval, got)
	}
}

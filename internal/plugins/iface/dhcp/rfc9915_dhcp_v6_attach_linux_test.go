// Design: docs/features/interfaces.md -- DHCPv6 client lifecycle
// Related: rfc9915_dhcp_v6_lease_linux_test.go -- the rig these tests drive
// Related: dhcp_v6_attach_linux.go -- the DAD outcome, Decline and link-move code under test
// RFC: rfc/short/rfc9915.md

//go:build linux

package ifacedhcp

import (
	"log/slog"
	"net"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/iana"

	"github.com/ze-software/ze/internal/component/iface"
)

// v6TestAddrInfo is the leased address as the device reports it.
func v6TestAddrInfo(tentative, duplicate bool) []iface.AddrInfo {
	return []iface.AddrInfo{{Address: v6TestAddr.String(), PrefixLength: 128, Family: "ipv6",
		Tentative: tentative, DADFailed: duplicate}}
}

// leaseNAPD is an IA_NA address and an IA_PD prefix, T1 10 s, T2 16 s,
// valid 30 s.
func leaseNAPD() []dhcpv6.Option {
	return []dhcpv6.Option{
		testIANA(10*time.Second, 16*time.Second, testIAAddr(20*time.Second, 30*time.Second)),
		testIAPD(10*time.Second, 16*time.Second, 20*time.Second, 30*time.Second),
	}
}

// leaseLongPD is leaseNAPD with T1 100 s, T2 160 s and valid 300 s, so no
// timer runs during a link-move test.
func leaseLongPD() []dhcpv6.Option {
	return []dhcpv6.Option{
		testIANA(100*time.Second, 160*time.Second, testIAAddr(200*time.Second, 300*time.Second)),
		testIAPD(100*time.Second, 160*time.Second, 200*time.Second, 300*time.Second),
	}
}

// awaitRebind waits, in real time, until the worker has sent a Rebind.
func (r *v6Rig) awaitRebind() {
	r.t.Helper()
	r.awaitSentOf(dhcpv6.MessageTypeRebind)
}

// link hands one link event to the worker, failing the test when the worker
// takes none within 3 s of real time.
func (r *v6Rig) link(kind iface.LinkEventKind) {
	r.t.Helper()
	select {
	case r.links <- iface.LinkEvent{Kind: kind}:
	case <-time.After(3 * time.Second):
		r.t.Fatalf("the worker took no %s link event", kind)
	}
}

// awaitTimerAt waits, in real time, until the worker has armed a timer for
// the offset at.
func (r *v6Rig) awaitTimerAt(at time.Duration) {
	r.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if next, ok := r.clk.NextTimer(); ok && r.offset(next) == at {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	r.t.Fatalf("the worker armed no timer for %v", at)
}

// move delivers a down and then an up of the bound device.
func (r *v6Rig) move() {
	r.link(iface.LinkDown)
	r.link(iface.LinkUp)
}

// TestRFC9915Section182101DuplicateAddressDeclined leases an address and a
// prefix whose address the device reports tentative for 0.5 s and then no
// longer holds, as the kernel does when DAD fails on an address of finite
// lifetime. The server answers the Decline with NoBinding.
//
// VALIDATES: AC-26 -- the address leaves the device before the Decline is
// sent; one Decline goes out carrying the server's and the client's DUIDs,
// an Elapsed Time and one IA_NA holding only the address, and no IA_PD; the
// NoBinding Reply completes it; the prefix is kept and the Renew at T1
// carries it but not the declined address.
// PREVENTS: a duplicate address kept in use with no Decline.
func TestRFC9915Section182101DuplicateAddressDeclined(t *testing.T) {
	// RFC requirement: RFC9915-18.2.10.1-1 positive -- an address the kernel deletes while it is tentative (a failed DAD) is removed from the lease and declined to the server.
	// RFC requirement: RFC9915-18.2.8-1 positive -- the Decline carries no IA_PD although the lease holds a delegated prefix.
	// RFC requirement: RFC9915-18.2.8-2 positive -- the Decline carries a Server Identifier option equal to the leasing server's DUID.
	// RFC requirement: RFC9915-18.2.8-3 positive -- the Decline carries a Client Identifier option equal to the client's DUID.
	// RFC requirement: RFC9915-18.2.8-4 positive -- the Decline carries an Elapsed Time option.
	// RFC requirement: RFC9915-18.2.8-5 positive -- the Decline's IA_NA carries the declined address and nothing else.
	// RFC requirement: RFC9915-18.2.8-6 positive -- the declined address is removed from the device before the Decline is sent, and the later Renew does not carry it.
	r := newV6Rig(t, dHCPConfig{PDRequested: true})
	// The kernel deletes an address of finite lifetime whose DAD failed.
	r.backend.addrsAt = func(at time.Duration) []iface.AddrInfo {
		if at >= 500*time.Millisecond {
			return nil
		}
		return v6TestAddrInfo(true, false)
	}
	removedBeforeDecline := -1
	r.srv.serve = func(req *dhcpv6.Message) []*dhcpv6.Message {
		//exhaustive:ignore // the scripted server answers only the types it serves
		switch req.MessageType {
		case dhcpv6.MessageTypeSolicit:
			return []*dhcpv6.Message{testV6Answer(req, dhcpv6.MessageTypeReply, v6ServerA, leaseNAPD()...)}
		case dhcpv6.MessageTypeDecline:
			removedBeforeDecline = r.countOps("remove-addr " + loopKernelDevice + " " + v6TestCIDR)
			return []*dhcpv6.Message{testV6Answer(req, dhcpv6.MessageTypeReply, v6ServerA,
				&dhcpv6.OptStatusCode{StatusCode: iana.StatusNoBinding})}
		}
		return nil
	}
	r.startWorker()
	r.runUntil(seconds(11))

	declines := r.sentOf(dhcpv6.MessageTypeDecline)
	if len(declines) != 1 {
		t.Fatalf("%d DECLINE, want 1 (the NoBinding Reply completes it): %s", len(declines), r.describeSent())
	}
	if declines[0].at != 500*time.Millisecond {
		t.Errorf("DECLINE at %v, want 500ms (the first DAD poll seeing the address gone)", declines[0].at)
	}
	if removedBeforeDecline != 1 {
		t.Errorf("address removed %d times before the DECLINE was sent, want 1", removedBeforeDecline)
	}
	m := declines[0].msg
	if sid := m.Options.ServerID(); sid == nil || !sid.Equal(v6ServerA) {
		t.Errorf("Decline Server ID %v, want %v", sid, v6ServerA)
	}
	solicits := r.sentOf(dhcpv6.MessageTypeSolicit)
	if len(solicits) == 0 {
		t.Fatal("no SOLICIT")
	}
	if id := m.Options.ClientID(); id == nil || !id.Equal(solicits[0].msg.Options.ClientID()) {
		t.Errorf("Decline Client ID %v, want the client's DUID", id)
	}
	if m.Options.GetOne(dhcpv6.OptionElapsedTime) == nil {
		t.Error("Decline carries no Elapsed Time")
	}
	if pds := m.Options.IAPD(); len(pds) != 0 {
		t.Errorf("Decline carries %d IA_PD, want none", len(pds))
	}
	nas := m.Options.IANA()
	if len(nas) != 1 {
		t.Fatalf("Decline carries %d IA_NA, want 1", len(nas))
	}
	if addrs := nas[0].Options.Addresses(); len(addrs) != 1 || !addrs[0].IPv6Addr.Equal(v6TestAddr) {
		t.Errorf("Decline IA Addresses %v, want only %v", addrs, v6TestAddr)
	}

	renews := r.sentOf(dhcpv6.MessageTypeRenew)
	if len(renews) == 0 || renews[0].at != seconds(10) {
		t.Fatalf("RENEW at %v, want one at T1 10s (the prefix is kept)", sentTimes(renews))
	}
	for _, na := range renews[0].msg.Options.IANA() {
		if len(na.Options.Addresses()) != 0 {
			t.Errorf("Renew carries the declined address: %v", na.Options.Addresses())
		}
	}
	if pd := renews[0].msg.Options.IAPD(); len(pd) != 1 || len(pd[0].Options.Prefixes()) != 1 {
		t.Errorf("Renew IA_PD %v, want the kept prefix", pd)
	}
}

// TestRFC9915Section182101UniqueAddressKept leases an address the device
// reports tentative for 1 s and then DAD-complete.
//
// VALIDATES: AC-27 -- no Decline; the address stays on the device until the
// lease runs on to its Renew.
func TestRFC9915Section182101UniqueAddressKept(t *testing.T) {
	// RFC requirement: RFC9915-18.2.10.1-1 negative -- an address whose DAD completes without IFA_F_DADFAILED is not declined and not removed.
	r := newV6Rig(t, dHCPConfig{})
	r.backend.addrsAt = func(at time.Duration) []iface.AddrInfo {
		return v6TestAddrInfo(at < time.Second, false)
	}
	r.srv.serve = leaseServer(lease10x16x30, true, true)
	r.startWorker()
	r.runUntil(seconds(11))

	if got := r.sentOf(dhcpv6.MessageTypeDecline); len(got) != 0 {
		t.Errorf("%d DECLINE for a unique address", len(got))
	}
	if got := r.countOps("remove-addr"); got != 0 {
		t.Errorf("address removed %d times, want 0", got)
	}
	if got := r.sentOf(dhcpv6.MessageTypeRenew); len(got) == 0 {
		t.Errorf("no RENEW at T1: %s", r.describeSent())
	}
}

// TestDHCPv6AddressFlushedByLinkDownReinstalledNotDeclined holds an address
// and a prefix whose DAD completed, then the device is set down and up at 5 s,
// and the kernel deletes the address with the down, as it deletes every
// address of finite lifetime on an interface set down.
//
// VALIDATES: the move reinstalls the leased address with its remaining
// lifetimes (valid 295 s, preferred 195 s at 5 s into a 300/200 lease) and
// sends no Decline: an address the link change deleted is not a duplicate.
// PREVENTS: every leased address declined to the server after an operator
// bounces the interface.
func TestDHCPv6AddressFlushedByLinkDownReinstalledNotDeclined(t *testing.T) {
	r := newV6Rig(t, dHCPConfig{PDRequested: true})
	r.srv.serve = leaseServer(leaseLongPD, false, false)
	r.startWorker()
	r.runUntil(seconds(5))
	r.clk.Set(r.start.Add(seconds(5)))
	r.backend.mu.Lock()
	r.backend.installed = nil // the kernel flushes the address with the down
	r.backend.mu.Unlock()
	r.move()
	r.awaitRebind()
	r.runUntil(seconds(8))

	if got := r.sentOf(dhcpv6.MessageTypeDecline); len(got) != 0 {
		t.Errorf("%d DECLINE for an address the link change deleted: %s", len(got), r.describeSent())
	}
	if got := r.countOps("replace " + loopKernelDevice + " " + v6TestCIDR + " 295 195"); got != 1 {
		t.Errorf("address reinstalled with 295/195 %d times after the move, want 1", got)
	}
	if got := r.countOps("remove-addr"); got != 0 {
		t.Errorf("address removed %d times, want 0", got)
	}
}

// TestDHCPv6AddressGoneWithTheLinkBeforeItsDownEventNotDeclined holds an
// address and a prefix whose address is tentative for 0.5 s; at 0.5 s the
// device is set down and the kernel deletes the address, but no link event
// reaches the worker: the DAD poll reads the device before the monitor
// delivers the down. At 5 s the device is up again and only the up event
// arrives.
//
// VALIDATES: the address gone while the same kernel read reports the device
// down is judged flushed with the link, not a duplicate: no Decline goes
// out, and the later up completes the move, so the address is installed
// again with what is left of its lifetimes (valid 295 s, preferred 195 s).
// PREVENTS: a unique address declined because the DAD poll outran the
// LinkDown event (journal row of 2026-10-19 in
// plan/journal/reconciler-repairs-against-a-stale-snapshot.md).
func TestDHCPv6AddressGoneWithTheLinkBeforeItsDownEventNotDeclined(t *testing.T) {
	r := newV6Rig(t, dHCPConfig{PDRequested: true})
	r.srv.serve = leaseServer(leaseLongPD, false, false)
	r.backend.addrsAt = func(at time.Duration) []iface.AddrInfo {
		switch {
		case at < 500*time.Millisecond:
			return v6TestAddrInfo(true, false)
		case at < seconds(5):
			return nil // deleted with the down
		default:
			return v6TestAddrInfo(false, false) // installed again, DAD complete
		}
	}
	r.backend.stateAt = func(at time.Duration) string {
		if at >= 500*time.Millisecond && at < seconds(5) {
			return "down"
		}
		return "up"
	}
	r.startWorker()
	r.runUntil(seconds(5))

	if got := r.sentOf(dhcpv6.MessageTypeDecline); len(got) != 0 {
		t.Fatalf("%d DECLINE for an address deleted with the link: %s", len(got), r.describeSent())
	}
	r.clk.Set(r.start.Add(seconds(5)))
	r.link(iface.LinkUp)
	r.awaitRebind()
	r.runUntil(seconds(8))

	if got := r.sentOf(dhcpv6.MessageTypeDecline); len(got) != 0 {
		t.Errorf("%d DECLINE after the link came back: %s", len(got), r.describeSent())
	}
	if got := r.countOps("replace " + loopKernelDevice + " " + v6TestCIDR + " 295 195"); got != 1 {
		t.Errorf("address reinstalled with 295/195 %d times after the up, want 1", got)
	}
	if got := r.countOps("remove-addr"); got != 0 {
		t.Errorf("address removed %d times, want 0", got)
	}
}

// TestDHCPv6AddressGoneWithAnUnreadableLinkStateNotDeclined holds an address
// the kernel deletes at 0.5 s, from a device whose read reports a state that
// is neither up nor down: empty until 1 s, "dormant" until 2 s, and up after.
//
// VALIDATES: a link state the worker cannot read declines nothing and logs
// one Warn for each distinct state, naming the interface and the state, not
// one for each DAD poll; the address stays pending, so the first read at or
// after 2 s, no later than one DAD poll after it, reports the link up, finds
// the address gone and declines it then.
// PREVENTS: an unreadable state answered as "up" (a unique address declined)
// or as "down" (a duplicate address kept in use and never declined).
func TestDHCPv6AddressGoneWithAnUnreadableLinkStateNotDeclined(t *testing.T) {
	r := newV6Rig(t, dHCPConfig{PDRequested: true})
	r.srv.serve = leaseServer(leaseLongPD, false, false)
	r.backend.addrsAt = func(at time.Duration) []iface.AddrInfo {
		if at < 500*time.Millisecond {
			return v6TestAddrInfo(true, false)
		}
		return nil
	}
	r.backend.stateAt = func(at time.Duration) string {
		switch {
		case at < seconds(1):
			return ""
		case at < seconds(2):
			return "dormant"
		default:
			return iface.LinkStateUp
		}
	}
	r.startWorker()
	r.runUntil(seconds(4))

	declines := r.sentOf(dhcpv6.MessageTypeDecline)
	if len(declines) == 0 {
		t.Fatalf("no DECLINE once the link read up: %s", r.describeSent())
	}
	if declines[0].at < seconds(2) {
		t.Errorf("DECLINE at %v, while the link state was unreadable", declines[0].at)
	}
	if declines[0].at > seconds(2)+v6DADPoll {
		t.Errorf("DECLINE at %v, more than one DAD poll after the link read up at 2s", declines[0].at)
	}
	if got := r.logs.count(slog.LevelWarn, "neither up nor down"); got != 2 {
		t.Errorf("%d Warn for the unreadable link state, want 2: one for each distinct state", got)
	}
	for _, state := range []string{`""`, `"dormant"`} {
		if got := r.logs.countWith(slog.LevelWarn, "neither up nor down", "state", state); got != 1 {
			t.Errorf("%d Warn naming state %s, want 1", got, state)
		}
	}
	if got := r.logs.countWith(slog.LevelWarn, "neither up nor down", "iface", r.backend.logical); got != 2 {
		t.Errorf("%d Warn naming %s, want 2", got, r.backend.logical)
	}
}

// TestDHCPv6MoveReinstallsAnAddressNearExpiryAsFinite holds an address valid
// for 2 s (preferred 1 s) beside a long delegated prefix, and moves the link
// at 1.5 s, when half a second of the address is left.
//
// VALIDATES: the reinstall rounds the valid lifetime left up to 1 s, so the
// kernel holds the address as finite; the spent preferred lifetime goes as 0.
// The expired address is still removed, and no Decline is sent.
// PREVENTS: a valid lifetime truncated to 0 s, which the backend reads as "no
// lifetime" and the kernel installs as permanent (IFA_F_PERMANENT).
func TestDHCPv6MoveReinstallsAnAddressNearExpiryAsFinite(t *testing.T) {
	r := newV6Rig(t, dHCPConfig{PDRequested: true})
	r.srv.serve = leaseServer(func() []dhcpv6.Option {
		return []dhcpv6.Option{
			testIANA(100*time.Second, 160*time.Second, testIAAddr(1*time.Second, 2*time.Second)),
			testIAPD(100*time.Second, 160*time.Second, 200*time.Second, 300*time.Second),
		}
	}, false, false)
	r.startWorker()
	r.runUntil(1500 * time.Millisecond)
	r.clk.Set(r.start.Add(1500 * time.Millisecond))
	r.backend.mu.Lock()
	r.backend.installed = nil // the kernel flushes the address with the down
	r.backend.mu.Unlock()
	r.move()
	r.awaitRebind()

	if got := r.countOps("replace " + loopKernelDevice + " " + v6TestCIDR + " 0 "); got != 0 {
		t.Errorf("address reinstalled with a valid lifetime of 0 s %d times: the kernel holds it as permanent", got)
	}
	if got := r.countOps("replace " + loopKernelDevice + " " + v6TestCIDR + " 1 0"); got != 1 {
		t.Errorf("address reinstalled with 1/0 %d times after the move, want 1", got)
	}
	r.runUntil(15 * time.Second) // past the unanswered Rebind's CNF_MAX_RD
	if got := r.countOps("remove-addr " + loopKernelDevice + " " + v6TestCIDR); got != 1 {
		t.Errorf("expired address removed %d times, want 1", got)
	}
	if got := r.sentOf(dhcpv6.MessageTypeDecline); len(got) != 0 {
		t.Errorf("%d DECLINE for an address the link change deleted: %s", len(got), r.describeSent())
	}
}

// TestRFC9915Section18212MoveRebindsWithConfirmParameters holds an address
// and a prefix, then the device goes down and up at 5 s; no server answers
// the Rebind.
//
// VALIDATES: AC-28 -- a REBIND goes out at once (CNF_MAX_DELAY RAND pinned
// to 0) with no Server ID, carrying the address and the prefix, and it
// retransmits on the Confirm schedule: first RT under 1.1 s, every RT at
// most CNF_MAX_RT plus RAND, and none after CNF_MAX_RD (10 s).
func TestRFC9915Section18212MoveRebindsWithConfirmParameters(t *testing.T) {
	// RFC requirement: RFC9915-18.2.12-2 positive -- with a delegated prefix held, a device down then up starts a Rebind carrying the IA_NA and IA_PD leases, retransmitted with the Confirm parameters.
	r := newV6Rig(t, dHCPConfig{PDRequested: true})
	r.srv.serve = leaseServer(leaseLongPD, false, false)
	r.startWorker()
	r.runUntil(seconds(5))
	r.clk.Set(r.start.Add(seconds(5)))
	r.move()
	r.awaitRebind()
	r.runUntil(seconds(30))

	rebinds := r.sentOf(dhcpv6.MessageTypeRebind)
	if len(rebinds) < 4 {
		t.Fatalf("%d REBIND, want the Confirm schedule's retransmissions: %v", len(rebinds), sentTimes(rebinds))
	}
	if rebinds[0].at != seconds(5) {
		t.Errorf("first REBIND at %v, want 5s", rebinds[0].at)
	}
	if gap := rebinds[1].at - rebinds[0].at; gap > 1100*time.Millisecond {
		t.Errorf("first RT %v, want CNF_TIMEOUT (1s) plus at most 10%%", gap)
	}
	for i := 1; i < len(rebinds); i++ {
		if gap := rebinds[i].at - rebinds[i-1].at; gap > 4400*time.Millisecond {
			t.Errorf("RT %v between REBIND %d and %d, want at most CNF_MAX_RT (4s) plus 10%%", gap, i-1, i)
		}
	}
	if last := rebinds[len(rebinds)-1].at; last > seconds(15) {
		t.Errorf("REBIND at %v, after CNF_MAX_RD (10s) from 5s", last)
	}
	m := rebinds[0].msg
	if m.Options.ServerID() != nil {
		t.Error("link-change Rebind carries a Server ID")
	}
	if na := m.Options.IANA(); len(na) != 1 || len(na[0].Options.Addresses()) != 1 {
		t.Errorf("link-change Rebind IA_NA %v, want the leased address", na)
	}
	if pd := m.Options.IAPD(); len(pd) != 1 || len(pd[0].Options.Prefixes()) != 1 {
		t.Errorf("link-change Rebind IA_PD %v, want the delegated prefix", pd)
	}
}

// TestRFC9915Section18212NoMoveNoRebind delivers an up with no down before
// it to a client holding a prefix, and a down and up to a client holding
// only an address. The server answers a Confirm.
//
// VALIDATES: AC-28 -- neither sends a Rebind before T2, and the Renew at T1
// follows. The up without a down sends no Confirm; the address-only move
// sends its one Confirm at 5 s instead of a Rebind.
func TestRFC9915Section18212NoMoveNoRebind(t *testing.T) {
	// RFC requirement: RFC9915-18.2.12-2 negative -- an up event with no down before it, and a down/up while no delegated prefix is held, start no Rebind.
	cases := []struct {
		name     string
		pd       bool
		lease    func() []dhcpv6.Option
		send     func(r *v6Rig)
		confirms []time.Duration
	}{
		{"up without down", true, leaseNAPD, func(r *v6Rig) {
			r.link(iface.LinkUp)
			r.link(iface.LinkUp)
		}, nil},
		{"move without a prefix", false, lease10x16x30, (*v6Rig).move, []time.Duration{seconds(5)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newV6Rig(t, dHCPConfig{PDRequested: tc.pd})
			r.srv.serve = withConfirm(leaseServer(tc.lease, true, true), func(req *dhcpv6.Message) []*dhcpv6.Message {
				return []*dhcpv6.Message{confirmReply(req, v6ServerA)}
			})
			r.startWorker()
			r.runUntil(seconds(5))
			r.clk.Set(r.start.Add(seconds(5)))
			tc.send(r)
			r.awaitTimerAt(seconds(10))
			r.runUntil(seconds(15))

			if got := r.sentOf(dhcpv6.MessageTypeRebind); len(got) != 0 {
				t.Errorf("REBIND at %v with no move to answer", sentTimes(got))
			}
			if got := sentTimes(r.sentOf(dhcpv6.MessageTypeConfirm)); !slices.Equal(got, tc.confirms) {
				t.Errorf("CONFIRM at %v, want %v", got, tc.confirms)
			}
			if got := r.sentOf(dhcpv6.MessageTypeRenew); len(got) == 0 || got[0].at != seconds(10) {
				t.Errorf("RENEW at %v, want T1 10s", sentTimes(got))
			}
		})
	}
}

// TestRFC9915Section18212MoveRebindRateLimited moves the device at 5 s and
// again at 10 s; the server answers every Rebind.
//
// VALIDATES: AC-29 -- the second link-change Rebind waits until 35 s, 30 s
// after the first.
func TestRFC9915Section18212MoveRebindRateLimited(t *testing.T) {
	// RFC requirement: RFC9915-18.2.12-4 positive -- a second down/up 5 s after a link-change Rebind starts its Rebind no earlier than 30 s after the first.
	r := newV6Rig(t, dHCPConfig{PDRequested: true})
	r.srv.serve = leaseServer(leaseLongPD, false, true)
	r.startWorker()
	r.runUntil(seconds(5))
	r.clk.Set(r.start.Add(seconds(5)))
	r.move()
	r.awaitRebind()
	r.awaitTimerAt(seconds(105))
	r.clk.Set(r.start.Add(seconds(10)))
	r.move()
	r.awaitTimerAt(seconds(35))
	r.runUntil(seconds(40))

	got := sentTimes(r.sentOf(dhcpv6.MessageTypeRebind))
	if len(got) != 2 || got[0] != seconds(5) || got[1] != seconds(35) {
		t.Errorf("REBIND at %v, want [5s 35s]", got)
	}
}

// v6TestAddr2 is the second address of leaseLongNA.
var v6TestAddr2 = net.ParseIP("2001:db8::51")

// leaseLongNA is an IA_NA of two addresses and no IA_PD, T1 100 s, T2 160 s,
// valid 300 s, so no timer runs during a link-move test.
func leaseLongNA() []dhcpv6.Option {
	second := &dhcpv6.OptIAAddress{IPv6Addr: v6TestAddr2, PreferredLifetime: 200 * time.Second, ValidLifetime: 300 * time.Second}
	return []dhcpv6.Option{testIANA(100*time.Second, 160*time.Second, testIAAddr(200*time.Second, 300*time.Second), second)}
}

// withConfirm answers a Confirm with confirm, nil meaning silence, and every
// other message with base.
func withConfirm(base, confirm func(*dhcpv6.Message) []*dhcpv6.Message) func(*dhcpv6.Message) []*dhcpv6.Message {
	return func(req *dhcpv6.Message) []*dhcpv6.Message {
		if req.MessageType != dhcpv6.MessageTypeConfirm {
			return base(req)
		}
		if confirm == nil {
			return nil
		}
		return confirm(req)
	}
}

// confirmReply is a Reply from server to a Confirm, with a top-level Status
// Code option for each status given and none when no status is given.
func confirmReply(req *dhcpv6.Message, server dhcpv6.DUID, status ...iana.StatusCode) *dhcpv6.Message {
	opts := make([]dhcpv6.Option, 0, len(status))
	for _, s := range status {
		opts = append(opts, &dhcpv6.OptStatusCode{StatusCode: s})
	}
	return testV6Answer(req, dhcpv6.MessageTypeReply, server, opts...)
}

// awaitSentOf waits, in real time, until the worker has sent a message of kind.
func (r *v6Rig) awaitSentOf(kind dhcpv6.MessageType) {
	r.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(r.sentOf(kind)) > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	r.t.Fatalf("waiting for a %s: sent %s", kind, r.describeSent())
}

// sentAfter is every transmission made after at.
func (r *v6Rig) sentAfter(at time.Duration) []v6Sent {
	var out []v6Sent
	for _, s := range r.sent() {
		if s.at > at {
			out = append(out, s)
		}
	}
	return out
}

// confirmRig leases leaseLongNA, or leaseLongPD with pd, answers Confirm
// with confirm, and moves the device at 5 s.
func confirmRig(t *testing.T, pd bool, unit float64, confirm func(*dhcpv6.Message) []*dhcpv6.Message) *v6Rig {
	t.Helper()
	r := newV6Rig(t, dHCPConfig{PDRequested: pd})
	r.client.v6Random = func() float64 { return unit }
	lease := leaseLongNA
	if pd {
		lease = leaseLongPD
	}
	r.srv.serve = withConfirm(leaseServer(lease, true, true), confirm)
	r.startWorker()
	r.runUntil(seconds(5))
	r.clk.Set(r.start.Add(seconds(5)))
	r.move()
	return r
}

// assertLeaseKept fails when the worker removed an address, published an
// expiry or solicited again after the lease.
func assertLeaseKept(t *testing.T, r *v6Rig) {
	t.Helper()
	if n := r.countOps("remove-addr "); n != 0 {
		t.Errorf("%d address removals, want the lease kept", n)
	}
	if n := r.countEvents(evExpired); n != 0 {
		t.Errorf("%d %s events, want the lease kept", n, evExpired)
	}
	if got := r.sentOf(dhcpv6.MessageTypeSolicit); len(got) != 1 {
		t.Errorf("SOLICIT at %v, want only the first", sentTimes(got))
	}
}

// TestRFC9915Section18212ConfirmOnMove moves a client holding two addresses
// and no prefix, with RAND 0.5, and decodes the Confirm; a server confirms
// it. A client holding a prefix moves the same way.
//
// VALIDATES: AC-1 -- one CONFIRM, sent 0.5 s after the up: RAND times
// CNF_MAX_DELAY. AC-2 -- it carries one Client Identifier equal to the
// worker's DUID, one Elapsed Time, one IA_NA of the client's IAID with both
// addresses, T1, T2 and both lifetimes 0, and no Server Identifier, IA_PD or
// Option Request option. AC-8 -- with a prefix held, a REBIND goes out and
// no CONFIRM.
// PREVENTS: an addresses-only lease kept on a link it no longer fits, and a
// Confirm carrying what Appendix B leaves out.
func TestRFC9915Section18212ConfirmOnMove(t *testing.T) {
	// RFC requirement: RFC9915-18.2.3-1 positive -- the CONFIRM a down/up of a lease of addresses alone sends carries exactly one Client Identifier option, equal to the worker's DUID.
	// RFC requirement: RFC9915-18.2.3-3 positive -- with RAND 0.5 that CONFIRM goes out 0.5 s after the up, RAND times CNF_MAX_DELAY, and not before.
	// RFC requirement: RFC9915-18.2.12-1 positive -- that CONFIRM carries one IA_NA of the client's IAID holding exactly the two leased addresses.
	t.Run("addresses only", func(t *testing.T) {
		r := confirmRig(t, false, 0.5, func(req *dhcpv6.Message) []*dhcpv6.Message {
			return []*dhcpv6.Message{confirmReply(req, v6ServerA, iana.StatusSuccess)}
		})
		r.awaitTimerAt(seconds(5) + 500*time.Millisecond)
		r.runUntil(seconds(30))

		confirms := r.sentOf(dhcpv6.MessageTypeConfirm)
		if len(confirms) != 1 {
			t.Fatalf("CONFIRM at %v, want one", sentTimes(confirms))
		}
		if confirms[0].at != seconds(5)+500*time.Millisecond {
			t.Errorf("CONFIRM at %v, want 5.5s: the up at 5s plus RAND 0.5 times CNF_MAX_DELAY 1s", confirms[0].at)
		}
		m := confirms[0].msg
		duid := r.sentOf(dhcpv6.MessageTypeSolicit)[0].msg.Options.ClientID()
		if ids := m.Options.Get(dhcpv6.OptionClientID); len(ids) != 1 {
			t.Errorf("%d Client Identifier options, want 1", len(ids))
		}
		if id := m.Options.ClientID(); id == nil || !id.Equal(duid) {
			t.Errorf("Client Identifier %v, want the worker's DUID %v", id, duid)
		}
		if n := len(m.Options.Get(dhcpv6.OptionElapsedTime)); n != 1 {
			t.Errorf("%d Elapsed Time options, want 1", n)
		}
		for _, code := range []dhcpv6.OptionCode{dhcpv6.OptionServerID, dhcpv6.OptionIAPD, dhcpv6.OptionORO} {
			if got := m.Options.Get(code); len(got) != 0 {
				t.Errorf("CONFIRM carries %s, which Confirm does not", code)
			}
		}
		na := m.Options.IANA()
		if len(na) != 1 {
			t.Fatalf("%d IA_NA, want the lease's one", len(na))
		}
		if na[0].IaId != v6ClientIAID || na[0].T1 != 0 || na[0].T2 != 0 {
			t.Errorf("IA_NA %x T1 %v T2 %v, want IAID %x with T1 and T2 0", na[0].IaId, na[0].T1, na[0].T2, v6ClientIAID)
		}
		var addrs []string
		for _, a := range na[0].Options.Addresses() {
			addrs = append(addrs, a.IPv6Addr.String())
			if a.PreferredLifetime != 0 || a.ValidLifetime != 0 {
				t.Errorf("address %s lifetimes %v/%v, want 0/0", a.IPv6Addr, a.PreferredLifetime, a.ValidLifetime)
			}
		}
		slices.Sort(addrs)
		if want := []string{v6TestAddr.String(), v6TestAddr2.String()}; !slices.Equal(addrs, want) {
			t.Errorf("IA_NA addresses %v, want every leased address %v", addrs, want)
		}
		if got := r.sentOf(dhcpv6.MessageTypeRebind); len(got) != 0 {
			t.Errorf("REBIND at %v for a lease of addresses alone", sentTimes(got))
		}
		assertLeaseKept(t, r)
	})
	t.Run("prefix held", func(t *testing.T) {
		r := confirmRig(t, true, 0, nil)
		r.awaitRebind()
		r.runUntil(seconds(30))
		if got := r.sentOf(dhcpv6.MessageTypeConfirm); len(got) != 0 {
			t.Errorf("CONFIRM at %v with a delegated prefix held, want the Rebind alone", sentTimes(got))
		}
		if got := r.sentOf(dhcpv6.MessageTypeRebind); len(got) != 1 || got[0].at != seconds(5) {
			t.Errorf("REBIND at %v, want one at 5s", sentTimes(got))
		}
	})
}

// TestRFC9915Section18212NoMoveNoConfirm hands a client holding two addresses
// and no prefix two up events at 5 s with no down before them; a server would
// confirm any Confirm. RAND is 0, so a Confirm would go out at once.
//
// VALIDATES: an up the device was never down for is no move: no CONFIRM and
// no REBIND go out, and the next timer the worker arms is T1 (100 s).
// PREVENTS: a Confirm sent for every carrier event, which would carry the
// lease's IAs to a server on a link the client never left.
func TestRFC9915Section18212NoMoveNoConfirm(t *testing.T) {
	// RFC requirement: RFC9915-18.2.12-1 negative -- two up events with no down before them, for a lease of addresses alone, send no CONFIRM and no REBIND.
	r := newV6Rig(t, dHCPConfig{})
	r.client.v6Random = func() float64 { return 0 }
	r.srv.serve = withConfirm(leaseServer(leaseLongNA, true, true), func(req *dhcpv6.Message) []*dhcpv6.Message {
		return []*dhcpv6.Message{confirmReply(req, v6ServerA, iana.StatusSuccess)}
	})
	r.startWorker()
	r.runUntil(seconds(5))
	r.clk.Set(r.start.Add(seconds(5)))
	r.link(iface.LinkUp)
	r.link(iface.LinkUp)
	r.awaitTimerAt(seconds(100))
	r.runUntil(seconds(30))

	if got := r.sentOf(dhcpv6.MessageTypeConfirm); len(got) != 0 {
		t.Errorf("CONFIRM at %v with no move to answer", sentTimes(got))
	}
	if got := r.sentOf(dhcpv6.MessageTypeRebind); len(got) != 0 {
		t.Errorf("REBIND at %v with no move to answer", sentTimes(got))
	}
}

// TestRFC9915Section1823ConfirmRetransmission moves a client holding
// addresses alone at 5 s; no server answers the Confirm.
//
// VALIDATES: AC-3 -- the CONFIRM goes out at 5 s and retransmits with the
// first RT under 1.1 s, every RT at most CNF_MAX_RT plus 10%, none at or
// after CNF_MAX_RD (10 s), each Elapsed Time equal to the time since the
// first; then the worker keeps every address and sends nothing until T1.
// PREVENTS: a Confirm that never stops, and a lease dropped for silence.
func TestRFC9915Section1823ConfirmRetransmission(t *testing.T) {
	// RFC requirement: RFC9915-18.2.3-2 positive -- every CONFIRM of an unanswered exchange carries an Elapsed Time equal to the time since the first CONFIRM.
	r := confirmRig(t, false, 0, nil)
	r.awaitSentOf(dhcpv6.MessageTypeConfirm)
	r.runUntil(seconds(60))

	confirms := r.sentOf(dhcpv6.MessageTypeConfirm)
	if len(confirms) < 4 {
		t.Fatalf("%d CONFIRM, want the Confirm schedule's retransmissions: %v", len(confirms), sentTimes(confirms))
	}
	if confirms[0].at != seconds(5) {
		t.Errorf("first CONFIRM at %v, want 5s", confirms[0].at)
	}
	if gap := confirms[1].at - confirms[0].at; gap > 1100*time.Millisecond {
		t.Errorf("first RT %v, want CNF_TIMEOUT (1s) plus at most 10%%", gap)
	}
	for i := 1; i < len(confirms); i++ {
		if gap := confirms[i].at - confirms[i-1].at; gap > 4400*time.Millisecond {
			t.Errorf("RT %v between CONFIRM %d and %d, want at most CNF_MAX_RT (4s) plus 10%%", gap, i-1, i)
		}
	}
	last := confirms[len(confirms)-1].at
	if last >= seconds(15) {
		t.Errorf("CONFIRM at %v, at or after CNF_MAX_RD (10s) from 5s", last)
	}
	for i, c := range confirms {
		if got, want := c.msg.Options.ElapsedTime(), v6ElapsedTime(c.at-confirms[0].at); got != want {
			t.Errorf("CONFIRM %d Elapsed Time %v, want %v", i, got, want)
		}
	}
	if got := r.sentAfter(last); len(got) != 0 {
		t.Errorf("sent after the Confirm ended: %s", r.describeSent())
	}
	assertLeaseKept(t, r)
}

// TestRFC9915Section182103ConfirmReplySuccessImplicit answers the Confirm
// with a Reply carrying Status Code Success, and with a Reply carrying no
// Status Code option at all.
//
// VALIDATES: AC-4 -- one CONFIRM, then nothing until T1: no address removed,
// no expiry published, no second SOLICIT. A-3: a Reply with no Status Code is
// Success.
// PREVENTS: an implicit Success read as a failure, which would retransmit or
// drop the lease.
func TestRFC9915Section182103ConfirmReplySuccessImplicit(t *testing.T) {
	cases := []struct {
		name   string
		status []iana.StatusCode
	}{
		{"explicit", []iana.StatusCode{iana.StatusSuccess}},
		{"implicit", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := confirmRig(t, false, 0, func(req *dhcpv6.Message) []*dhcpv6.Message {
				return []*dhcpv6.Message{confirmReply(req, v6ServerA, tc.status...)}
			})
			r.awaitSentOf(dhcpv6.MessageTypeConfirm)
			r.runUntil(seconds(60))
			if got := r.sentOf(dhcpv6.MessageTypeConfirm); len(got) != 1 {
				t.Errorf("CONFIRM at %v, want one answered at once", sentTimes(got))
			}
			if got := r.sentAfter(seconds(5)); len(got) != 0 {
				t.Errorf("sent after the confirmed Confirm: %s", r.describeSent())
			}
			assertLeaseKept(t, r)
		})
	}
}

// TestRFC9915Section182103ConfirmNotOnLinkRediscovers answers the Confirm
// with NotOnLink only. The device reports both addresses tentative until ze
// removes them, so the DAD poll is still pending when the Reply arrives.
//
// VALIDATES: AC-5 -- both leased addresses are removed, one dhcp-expired is
// published for each, no Decline goes out for them, and the worker sends a
// SOLICIT after the CONFIRM.
// PREVENTS: a lease kept on a link whose server says it does not fit, and a
// removed address declined as a duplicate.
func TestRFC9915Section182103ConfirmNotOnLinkRediscovers(t *testing.T) {
	r := newV6Rig(t, dHCPConfig{})
	r.backend.addrsAt = func(time.Duration) []iface.AddrInfo {
		if r.countOps("remove-addr ") > 0 {
			return nil
		}
		return []iface.AddrInfo{
			{Address: v6TestAddr.String(), PrefixLength: 128, Family: "ipv6", Tentative: true},
			{Address: v6TestAddr2.String(), PrefixLength: 128, Family: "ipv6", Tentative: true},
		}
	}
	r.srv.serve = withConfirm(func(req *dhcpv6.Message) []*dhcpv6.Message {
		if req.MessageType == dhcpv6.MessageTypeSolicit && len(r.sentOf(dhcpv6.MessageTypeSolicit)) == 1 {
			return []*dhcpv6.Message{testV6Answer(req, dhcpv6.MessageTypeReply, v6ServerA, leaseLongNA()...)}
		}
		return nil // the second discovery goes unanswered
	}, func(req *dhcpv6.Message) []*dhcpv6.Message {
		return []*dhcpv6.Message{confirmReply(req, v6ServerA, iana.StatusNotOnLink)}
	})
	r.startWorker()
	r.runUntil(seconds(5))
	r.clk.Set(r.start.Add(seconds(5)))
	r.move()
	r.awaitSentOf(dhcpv6.MessageTypeConfirm)
	r.runUntil(seconds(20))

	confirms := r.sentOf(dhcpv6.MessageTypeConfirm)
	if len(confirms) != 1 {
		t.Fatalf("CONFIRM at %v, want one", sentTimes(confirms))
	}
	for _, addr := range []net.IP{v6TestAddr, v6TestAddr2} {
		if n := r.countOps("remove-addr " + loopKernelDevice + " " + addr.String() + "/128"); n != 1 {
			t.Errorf("%d removals of %s, want 1", n, addr)
		}
	}
	if n := r.countEvents(evExpired); n != 2 {
		t.Errorf("%d %s events, want one per leased address", n, evExpired)
	}
	if got := r.sentOf(dhcpv6.MessageTypeDecline); len(got) != 0 {
		t.Errorf("DECLINE at %v for an address the NotOnLink removed", sentTimes(got))
	}
	solicits := r.sentOf(dhcpv6.MessageTypeSolicit)
	if len(solicits) < 2 || solicits[1].at < confirms[0].at {
		t.Errorf("SOLICIT at %v, want discovery again after the CONFIRM at %v", sentTimes(solicits), confirms[0].at)
	}
}

// TestRFC9915Section182103ConfirmMixedReplies answers the Confirm with a
// NotOnLink Reply from one server and a Success Reply from another, in both
// orders, within one listen window.
//
// VALIDATES: AC-6 -- the lease is kept as AC-4 keeps it: one CONFIRM, no
// address removed, no expiry, no second SOLICIT.
// PREVENTS: R-1, a NotOnLink arriving first dropping a lease another server
// confirms.
func TestRFC9915Section182103ConfirmMixedReplies(t *testing.T) {
	for _, notOnLinkFirst := range []bool{true, false} {
		name := "success first"
		if notOnLinkFirst {
			name = "notonlink first"
		}
		t.Run(name, func(t *testing.T) {
			r := confirmRig(t, false, 0, func(req *dhcpv6.Message) []*dhcpv6.Message {
				refused := confirmReply(req, v6ServerB, iana.StatusNotOnLink)
				confirmed := confirmReply(req, v6ServerA, iana.StatusSuccess)
				if notOnLinkFirst {
					return []*dhcpv6.Message{refused, confirmed}
				}
				return []*dhcpv6.Message{confirmed, refused}
			})
			r.awaitSentOf(dhcpv6.MessageTypeConfirm)
			r.runUntil(seconds(30))
			if got := r.sentOf(dhcpv6.MessageTypeConfirm); len(got) != 1 {
				t.Errorf("CONFIRM at %v, want one", sentTimes(got))
			}
			assertLeaseKept(t, r)
		})
	}
}

// TestRFC9915Section18212ConfirmRateLimit moves a client holding addresses
// alone at 5 s and again at 10 s; the server confirms every Confirm.
//
// VALIDATES: AC-7 -- the second CONFIRM waits until 35 s, 30 s after the
// first.
// PREVENTS: a flapping link flooding the server with Confirms.
func TestRFC9915Section18212ConfirmRateLimit(t *testing.T) {
	r := confirmRig(t, false, 0, func(req *dhcpv6.Message) []*dhcpv6.Message {
		return []*dhcpv6.Message{confirmReply(req, v6ServerA)}
	})
	r.awaitSentOf(dhcpv6.MessageTypeConfirm)
	r.awaitTimerAt(seconds(100))
	r.clk.Set(r.start.Add(seconds(10)))
	r.move()
	r.awaitTimerAt(seconds(35))
	r.runUntil(seconds(40))

	if got := sentTimes(r.sentOf(dhcpv6.MessageTypeConfirm)); len(got) != 2 || got[0] != seconds(5) || got[1] != seconds(35) {
		t.Errorf("CONFIRM at %v, want [5s 35s]", got)
	}
}

// TestDHCPv6ConfirmStopDuringDelay stops the worker while it waits out the
// CNF_MAX_DELAY delay (RAND 0.5), and while an unanswered Confirm listens.
//
// VALIDATES: AC-9 -- Stop returns, and no CONFIRM goes out after it: none in
// the delay case, only the first in the exchange case.
// PREVENTS: R-2, a Confirm wait holding Stop.
func TestDHCPv6ConfirmStopDuringDelay(t *testing.T) {
	// RFC requirement: RFC9915-18.2.3-3 negative -- with RAND 0.5, the worker arms its wake-up 0.5 s after the up and, stopped before then, has sent no CONFIRM.
	t.Run("delay", func(t *testing.T) {
		r := confirmRig(t, false, 0.5, nil)
		r.awaitTimerAt(seconds(5) + 500*time.Millisecond)
		if !r.stop() {
			t.Fatal("Stop did not return during the Confirm delay")
		}
		if got := r.sentOf(dhcpv6.MessageTypeConfirm); len(got) != 0 {
			t.Errorf("CONFIRM at %v after Stop", sentTimes(got))
		}
	})
	t.Run("exchange", func(t *testing.T) {
		r := confirmRig(t, false, 0, nil)
		r.awaitSentOf(dhcpv6.MessageTypeConfirm)
		if !r.stop() {
			t.Fatal("Stop did not return during the Confirm exchange")
		}
		if got := r.sentOf(dhcpv6.MessageTypeConfirm); len(got) != 1 {
			t.Errorf("CONFIRM at %v, want only the one sent before Stop", sentTimes(got))
		}
	})
}

// TestDHCPv6DADOutcomeFromDeviceAddresses reads the four outcomes from a
// device's address list.
//
// VALIDATES: IFA_F_DADFAILED (an address of infinite lifetime) is a
// duplicate, a tentative address is pending, a clear one is unique, and an
// address missing from the list is gone.
func TestDHCPv6DADOutcomeFromDeviceAddresses(t *testing.T) {
	addr := netip.MustParseAddr(v6TestAddr.String())
	cases := []struct {
		name  string
		addrs []iface.AddrInfo
		want  v6DAD
	}{
		{"dad failed", v6TestAddrInfo(true, true), v6DADDuplicate},
		{"tentative", v6TestAddrInfo(true, false), v6DADTentative},
		{"unique", v6TestAddrInfo(false, false), v6DADUnique},
		{"gone", []iface.AddrInfo{{Address: "2001:db8::51", PrefixLength: 128, Family: "ipv6"}}, v6DADGone},
	}
	for _, tc := range cases {
		if got := v6DADOf(tc.addrs, addr); got != tc.want {
			t.Errorf("%s: v6DADOf = %d, want %d", tc.name, got, tc.want)
		}
	}
}

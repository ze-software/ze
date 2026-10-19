// Design: docs/features/interfaces.md -- DHCPv6 client lifecycle
// Related: rfc9915_dhcp_v6_lease_linux_test.go -- the rig these tests drive
// Related: dhcp_v6_attach_linux.go -- the DAD outcome, Decline and link-move code under test
// RFC: rfc/short/rfc9915.md

//go:build linux

package ifacedhcp

import (
	"net/netip"
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

// awaitSent waits, in real time, until the worker has sent n messages of kind.
func (r *v6Rig) awaitSent(kind dhcpv6.MessageType, n int) {
	r.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if got := r.sentOf(kind); len(got) >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	r.t.Fatalf("waiting for %d %s: sent %s", n, kind, r.describeSent())
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
	r.awaitSent(dhcpv6.MessageTypeRebind, 1)
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
	r.awaitSent(dhcpv6.MessageTypeRebind, 1)

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
	r.awaitSent(dhcpv6.MessageTypeRebind, 1)
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
// only an address.
//
// VALIDATES: AC-28 -- neither sends a Rebind before T2; the first message
// after the lease is the Renew at T1.
func TestRFC9915Section18212NoMoveNoRebind(t *testing.T) {
	// RFC requirement: RFC9915-18.2.12-2 negative -- an up event with no down before it, and a down/up while no delegated prefix is held, start no Rebind.
	cases := []struct {
		name  string
		pd    bool
		lease func() []dhcpv6.Option
		send  func(r *v6Rig)
	}{
		{"up without down", true, leaseNAPD, func(r *v6Rig) {
			r.link(iface.LinkUp)
			r.link(iface.LinkUp)
		}},
		{"move without a prefix", false, lease10x16x30, (*v6Rig).move},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newV6Rig(t, dHCPConfig{PDRequested: tc.pd})
			r.srv.serve = leaseServer(tc.lease, true, true)
			r.startWorker()
			r.runUntil(seconds(5))
			r.clk.Set(r.start.Add(seconds(5)))
			tc.send(r)
			r.awaitTimerAt(seconds(10))
			r.runUntil(seconds(15))

			if got := r.sentOf(dhcpv6.MessageTypeRebind); len(got) != 0 {
				t.Errorf("REBIND at %v with no move to answer", sentTimes(got))
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
	r.awaitSent(dhcpv6.MessageTypeRebind, 1)
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

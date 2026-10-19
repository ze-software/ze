// Design: docs/features/interfaces.md -- DHCPv6 client lifecycle
// Overview: dhcp_v6_linux.go -- the v6 worker and its lease loop
// Related: dhcp_v6_exchange_linux.go -- the retransmission engine the Decline and the Rebind run on
// RFC: rfc/short/rfc9915.md

//go:build linux

package ifacedhcp

import (
	"net"
	"net/netip"
	"slices"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"

	"github.com/ze-software/ze/internal/component/iface"
)

const (
	// v6DADPoll is how often the worker reads the kernel's DAD outcome of an
	// address it installed, while one is still tentative.
	v6DADPoll = 250 * time.Millisecond

	// v6MoveInterval is the least time between two link-change Rebinds: the
	// figure RFC 9915 Section 18.2.12 gives as its example.
	v6MoveInterval = 30 * time.Second
)

// v6Wake is why waitLease returned.
type v6Wake uint8

const (
	// v6WakeUnspecified is the zero value; waitLease never answers it.
	v6WakeUnspecified v6Wake = iota
	// v6WakeTimer: the deadline, or a DAD poll, is due.
	v6WakeTimer
	// v6WakeMoved: the device went down and came back up.
	v6WakeMoved
	// v6WakeStop: Stop was called.
	v6WakeStop
)

// v6DAD is the kernel's Duplicate Address Detection outcome for one address.
type v6DAD uint8

const (
	// v6DADUnspecified is the zero value; v6DADOf never answers it.
	v6DADUnspecified v6DAD = iota
	// v6DADGone: the device no longer holds the address ze installed.
	v6DADGone
	// v6DADTentative: DAD is still running.
	v6DADTentative
	// v6DADDuplicate: DAD found the address in use (IFA_F_DADFAILED).
	v6DADDuplicate
	// v6DADUnique: DAD completed and found no other user.
	v6DADUnique
)

// waitLease waits for deadline, for the next DAD poll while an address is
// tentative, for a link move, or for Stop. A never deadline with nothing
// tentative waits for a move or Stop alone.
func (w *v6Worker) waitLease(deadline v6Instant) v6Wake {
	wakeAt := deadline
	if len(w.dadPending) > 0 {
		wakeAt = wakeAt.earlier(v6Instant{at: w.clock().Now().Add(v6DADPoll)})
	}
	var fired <-chan struct{} // nil: no timer, so the select never takes it

	if !wakeAt.never {
		d := wakeAt.at.Sub(w.clock().Now())
		if d <= 0 {
			if w.ctx.Err() != nil {
				return v6WakeStop
			}
			return v6WakeTimer
		}
		ch := make(chan struct{})
		timer := w.clock().AfterFunc(d, func() { close(ch) })
		defer timer.Stop()
		fired = ch
	}

	// The bound: each pass consumes one link event; the wait ends at its
	// deadline, on a move, or on Stop.
	for {
		select {
		case <-fired:
			return v6WakeTimer
		case <-w.ctx.Done():
			return v6WakeStop
		case ev, ok := <-w.links:
			if !ok {
				w.links = nil // a nil channel blocks, so the select ignores it
				continue
			}
			if w.linkMoved(ev) {
				return v6WakeMoved
			}
		}
	}
}

// drainLinks consumes the link events already queued and reports whether
// they complete a down/up pair. Queued while no lease was held, they describe
// the link the new lease was obtained on, so the caller ignores the answer;
// the last one still says whether the device is down now.
func (w *v6Worker) drainLinks() bool {
	moved := false
	// The bound: the subscription buffer holds a few events and is not
	// refilled faster than this loop reads it.
	for {
		select {
		case ev, ok := <-w.links:
			if !ok {
				w.links = nil
				return moved
			}
			if w.linkMoved(ev) {
				moved = true
			}
		default:
			return moved
		}
	}
}

// linkMoved records one link event and reports whether it ends a down/up
// pair: RFC 9915 Section 18.2.12 lists "The client's network interface
// indicates a disconnection event followed by a connection event" among the
// times "when a client may have moved to a new link". The monitor repeats the
// current state on every link change, so an up with no down before it is no
// move.
func (w *v6Worker) linkMoved(ev iface.LinkEvent) bool {
	switch ev.Kind {
	case iface.LinkDown:
		w.linkDown = true
		return false
	case iface.LinkUp, iface.LinkAppeared:
		if !w.linkDown {
			return false
		}
		w.linkDown = false
		return true
	default: // the set is the iface component's; an unknown kind is no move
		return false
	}
}

// noteMove reacts to a possible move to a new link: every leased address is
// judged by DAD again, because the kernel runs DAD again when the link comes
// back, and a lease holding a delegated prefix owes a Rebind.
//
// RFC 9915 Section 18.2.12: "If the client has any valid delegated prefixes
// obtained from the DHCP server, the client MUST initiate a Rebind/Reply
// message exchange as described in Section 18.2.5, with the exception that
// the retransmission parameters should be set as for the Confirm message (see
// Section 18.2.3)."
//
// A lease of addresses alone would owe the SHOULD-level Confirm, which ze
// does not send.
//
// Each leased address is installed again with what is left of its
// lifetimes: the kernel deletes every address of finite lifetime from an
// interface set down, and the lease still holds it. Replacing an address the
// down kept changes nothing but its lifetimes, which are the lease's anyway.
// Without it, the DAD read after the up would find the address gone with the
// link up and decline it as a duplicate.
func (w *v6Worker) noteMove() {
	now := w.clock().Now()
	clear(w.dadChecked)
	for _, b := range w.lease.live(v6KindAddress, now) {
		valid := v6Seconds(b.remainingAt(b.valid, now))
		preferred := v6Seconds(b.remainingAt(b.preferred, now))
		if err := iface.ReplaceAddressWithLifetime(w.c.ifaceName, b.prefix.String(), valid, preferred); err != nil {
			loggerPtr.Load().Warn("iface dhcp v6: cannot install the leased address again after the link came back up",
				"iface", w.c.ifaceName, "addr", b.prefix.String(), "err", err)
			continue
		}
		w.queueDAD(b.prefix)
	}
	if len(w.lease.live(v6KindPrefix, now)) == 0 {
		loggerPtr.Load().Debug("iface dhcp v6: the link came back up; no delegated prefix is held, so no Rebind is owed",
			"iface", w.c.ifaceName)
		return
	}
	if !w.moveDue.never {
		return // a link-change Rebind is already owed
	}
	due := now
	// RFC 9915 Section 18.2.12: "However, a client MUST rate- limit such
	// initiation attempts to avoid flooding a server with requests when there
	// are link issues (for example, only doing one of these at most every 30
	// seconds)."
	if !w.moveLast.never {
		due = later(now, w.moveLast.at.Add(v6MoveInterval))
	}
	w.moveDue = v6Instant{at: due}
}

func later(a, b time.Time) time.Time {
	if a.Before(b) {
		return b
	}
	return a
}

// moveOwed reports whether the link-change Rebind is due at now.
func (w *v6Worker) moveOwed(now time.Time) bool {
	if w.moveDue.never {
		return false
	}
	return !now.Before(w.moveDue.at)
}

// rebindAfterMove runs the link-change Rebind with the Confirm parameters.
// Unanswered, the lease runs on its own timers.
//
// RFC 9915 Section 18.2.3 gives Confirm the parameters IRT CNF_TIMEOUT, MRT
// CNF_MAX_RT, MRC 0 and MRD CNF_MAX_RD, and "The first Confirm message from
// the client on the interface MUST be delayed by a random amount of time
// between 0 and CNF_MAX_DELAY." The Rebind takes both.
func (w *v6Worker) rebindAfterMove(now time.Time) v6End {
	w.moveDue = v6Never()
	w.moveLast = v6Instant{at: now}
	if len(w.lease.live(v6KindPrefix, now)) == 0 {
		return v6EndExhausted // the prefixes expired before the Rebind was due
	}
	if !w.waitUntil(w.ctx, now.Add(v6Scale(v6CnfMaxDelay, w.random()))) {
		return v6EndCanceled
	}
	// RFC 9915 Section 18.2.12.
	end := w.extend(dhcpv6.MessageTypeRebind,
		v6Retransmission{irt: v6CnfTimeout, mrt: v6CnfMaxRT, mrd: v6CnfMaxRD}, w.lease.lastExpiry())
	if end == v6EndExhausted {
		loggerPtr.Load().Debug("iface dhcp v6: no server answered the link-change Rebind; the lease runs on its timers",
			"iface", w.c.ifaceName)
	}
	return end
}

// queueDAD marks an installed address for a DAD outcome read, unless its
// outcome was read since it was installed.
func (w *v6Worker) queueDAD(p netip.Prefix) {
	if w.dadChecked[p] {
		return
	}
	if slices.Contains(w.dadPending, p) {
		return
	}
	w.dadPending = append(w.dadPending, p)
}

// checkDAD reads the kernel's DAD outcome of each pending address and
// declines the duplicates. false on Stop.
//
// RFC 9915 Section 18.2.10.1: "The client MUST perform duplicate address
// detection as per Section 5.4 of [RFC4862], which does list some exceptions,
// on each of the received addresses in any IAs on which it has not performed
// duplicate address detection during processing of any of the previous Reply
// messages from the server." "If any of the addresses are found to be in use
// on the link, the client sends a Decline message to the server for those
// addresses as described in Section 18.2.8."
//
// The kernel performs the DAD on an address installed without IFA_F_NODAD and
// keeps it tentative, so unused as a source, until DAD ends; ze reads the
// outcome. A failed DAD marks an address of infinite lifetime
// IFA_F_DADFAILED, and deletes an address of finite lifetime outright
// (addrconf_dad_stop in net/ipv6/addrconf.c), so an address ze installed that
// is gone while the link is up is a duplicate too. One that is gone while the
// link is down was flushed by the link change, and the move installs it again
// (noteMove). The link events already queued are read first: the DAD poll and
// a queued down can wake the wait together, and a flush judged before its
// down is read would decline a unique address.
func (w *v6Worker) checkDAD() bool {
	if len(w.dadPending) == 0 {
		return true
	}
	if w.drainLinks() {
		w.noteMove()
	}
	addrs, err := iface.Addresses(w.c.ifaceName)
	if err != nil {
		loggerPtr.Load().Debug("iface dhcp v6: cannot read the DAD outcome, retrying",
			"iface", w.c.ifaceName, "err", err)
		return true
	}
	var duplicates []netip.Prefix
	pending := w.dadPending[:0]
	for _, p := range w.dadPending {
		outcome := v6DADOf(addrs, p.Addr())
		switch outcome {
		case v6DADTentative:
			pending = append(pending, p)
		case v6DADDuplicate:
			duplicates = append(duplicates, p)
		case v6DADUnique:
			w.dadChecked[p] = true
		case v6DADGone:
			if w.linkDown {
				continue // flushed with the link; noteMove queues it again
			}
			duplicates = append(duplicates, p)
		case v6DADUnspecified:
			panic("BUG: v6DADOf answered no outcome")
		}
	}
	w.dadPending = pending
	if len(duplicates) == 0 {
		return true
	}
	return w.decline(duplicates) != v6EndCanceled
}

// v6DADOf is the DAD outcome of addr among the device's addresses.
func v6DADOf(addrs []iface.AddrInfo, addr netip.Addr) v6DAD {
	for i := range addrs {
		got, err := netip.ParseAddr(addrs[i].Address)
		if err != nil {
			continue
		}
		if got != addr {
			continue
		}
		if addrs[i].DADFailed {
			return v6DADDuplicate
		}
		if addrs[i].Tentative {
			return v6DADTentative
		}
		return v6DADUnique
	}
	return v6DADGone
}

// decline stops using the duplicate addresses, then declines them to the
// server that leased them. The other bindings are kept.
//
// RFC 9915 Section 18.2.8: "The client MUST NOT use any of the addresses it
// is declining as the source address in the Decline message or in any
// subsequently transmitted message." Each address leaves the device and the
// lease record before the Decline is built, so neither the kernel nor a later
// message can carry it. "The client SHOULD retain the non-conflicting
// bindings."
//
// The exchange runs on "IRT: DEC_TIMEOUT", "MRT: 0", "MRC: DEC_MAX_RC" and
// "MRD: 0", and any identified Reply completes it (completionMatcher).
func (w *v6Worker) decline(duplicates []netip.Prefix) v6End {
	var declined []v6Binding
	for _, p := range duplicates {
		b, ok := w.lease.remove(v6KindAddress, p)
		if !ok {
			continue
		}
		declined = append(declined, b)
		w.drop(b)
	}
	if err := w.registerLeased(); err != nil {
		loggerPtr.Load().Debug("iface dhcp v6: leased-address registration failed", "iface", w.c.ifaceName, "err", err)
	}
	if len(declined) == 0 {
		return v6EndAnswered
	}
	for _, b := range declined {
		loggerPtr.Load().Warn("iface dhcp v6: another node on the link uses the leased address, declining it to the server",
			"iface", w.c.ifaceName, "addr", b.prefix.Addr().String())
	}
	msg, err := w.buildDecline(declined)
	if err != nil {
		loggerPtr.Load().Error("iface dhcp v6: cannot build Decline", "iface", w.c.ifaceName, "err", err)
		return v6EndExhausted
	}
	link, ok := w.open()
	if !ok {
		return v6EndCanceled
	}
	out := w.runExchange(w.ctx, &v6Exchange{
		name:   "Decline",
		link:   link,
		msg:    msg,
		params: v6Retransmission{irt: v6DecTimeout, mrc: v6DecMaxRC},
		accept: w.completionMatcher(),
	})
	closeV6Link(link, w.c.ifaceName)
	if out.end == v6EndExhausted {
		loggerPtr.Load().Debug("iface dhcp v6: Decline unanswered", "iface", w.c.ifaceName)
	}
	return out.end
}

// buildDecline is the Decline of the declined addresses.
//
// RFC 9915 Section 18.2.8: "The client MUST include a Server Identifier
// option (see Section 21.3) in the Decline message, identifying the server
// that allocated the lease(s)." "The client MUST include a Client Identifier
// option (see Section 21.2) to identify itself to the server." "The client
// MUST include an Elapsed Time option (see Section 21.9) to indicate how long
// the client has been trying to complete the current DHCP message exchange."
// (both from newV6Message) "The addresses to be declined MUST be included in
// the IAs." "The Decline message is not used in prefix delegation; thus, the
// client MUST NOT include IA_PD options (see Section 21.21) in the Decline
// message."
//
// RFC 9915 Section 21.6: "In a message sent by a client to a server, the
// preferred-lifetime and valid-lifetime fields SHOULD be set to 0." Ze sends
// both as 0.
func (w *v6Worker) buildDecline(declined []v6Binding) (*dhcpv6.Message, error) {
	m, err := w.newV6Message(dhcpv6.MessageTypeDecline)
	if err != nil {
		return nil, err
	}
	m.AddOption(dhcpv6.OptServerID(w.lease.server))
	na := &dhcpv6.OptIANA{IaId: w.iaidNA}
	for _, b := range declined {
		na.Options.Add(&dhcpv6.OptIAAddress{IPv6Addr: net.IP(b.prefix.Addr().AsSlice())})
	}
	m.AddOption(na)
	return m, nil
}

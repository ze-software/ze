// Design: docs/features/interfaces.md -- DHCPv6 client lifecycle
// Overview: dhcp_v6_linux.go -- the v6 worker and its lease loop
// Related: dhcp_v6_exchange_linux.go -- the retransmission engine the Decline, the Rebind and the Confirm run on
// RFC: rfc/short/rfc9915.md

//go:build linux

package ifacedhcp

import (
	"errors"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/iana"

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
// back, and the lease owes a move exchange, which maintain runs once moveDue
// is reached: a Rebind when it holds a delegated prefix, a Confirm when it
// holds addresses alone.
//
// RFC 9915 Section 18.2.12: "If the client has any valid delegated prefixes
// obtained from the DHCP server, the client MUST initiate a Rebind/Reply
// message exchange as described in Section 18.2.5, with the exception that
// the retransmission parameters should be set as for the Confirm message (see
// Section 18.2.3)." and "When the client detects that it may have moved to a
// new link and it has obtained addresses and no delegated prefixes from a
// server, the client SHOULD initiate a Confirm/Reply message exchange."
//
// The shape is decided when the exchange is due, not here: a prefix that
// expires before then leaves a lease of addresses alone.
//
// Each leased address is installed again with what is left of its
// lifetimes: the kernel deletes every address of finite lifetime from an
// interface set down, and the lease still holds it. Replacing an address the
// down kept changes nothing but its lifetimes, which are the lease's anyway.
// Without it, the DAD read after the up would find the address gone with the
// link up and decline it as a duplicate.
//
// The valid lifetime left rounds up to a whole second, the preferred one
// down. A live address has more than 0 s left, and a valid 0 beside a
// preferred 0 reaches the backend as no lifetime at all, which the kernel
// installs as permanent; the worker still removes it at its expiry.
func (w *v6Worker) noteMove() {
	now := w.clock().Now()
	clear(w.dadChecked)
	for _, b := range w.lease.live(v6KindAddress, now) {
		valid := v6SecondsUp(b.remainingAt(b.valid, now))
		preferred := v6Seconds(b.remainingAt(b.preferred, now))
		if err := iface.ReplaceAddressWithLifetime(w.c.ifaceName, b.prefix.String(), valid, preferred); err != nil {
			loggerPtr.Load().Warn("iface dhcp v6: cannot install the leased address again after the link came back up",
				"iface", w.c.ifaceName, "addr", b.prefix.String(), "err", err)
			continue
		}
		w.queueDAD(b.prefix)
	}
	if !w.moveDue.never {
		return // a move exchange is already owed
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

// moveOwed reports whether the move exchange (the link-change Rebind or the
// Confirm) is due at now.
func (w *v6Worker) moveOwed(now time.Time) bool {
	if w.moveDue.never {
		return false
	}
	return !now.Before(w.moveDue.at)
}

// startMoveExchange records that the move exchange owed starts at now, so a
// later move waits v6MoveInterval from it whichever shape it took. maintain
// MUST call it before rebindAfterMove or confirmAfterMove.
func (w *v6Worker) startMoveExchange(now time.Time) {
	w.moveDue = v6Never()
	w.moveLast = v6Instant{at: now}
}

// rebindAfterMove runs the link-change Rebind with the Confirm parameters,
// for a lease holding a live delegated prefix at now. Unanswered, the lease
// runs on its own timers. It MUST be called after startMoveExchange.
//
// RFC 9915 Section 18.2.3 gives Confirm the parameters IRT CNF_TIMEOUT, MRT
// CNF_MAX_RT, MRC 0 and MRD CNF_MAX_RD, and "The first Confirm message from
// the client on the interface MUST be delayed by a random amount of time
// between 0 and CNF_MAX_DELAY." The Rebind takes both.
func (w *v6Worker) rebindAfterMove(now time.Time) v6End {
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

// confirmAfterMove runs the Confirm exchange for a lease holding addresses
// and no live delegated prefix at now, and acts on its answer: a Success
// keeps the lease, a NotOnLink alone removes every leased address, so
// maintain finds the lease empty and the worker solicits, and no answer keeps
// the lease on its timers. It MUST be called after startMoveExchange.
//
// RFC 9915 Section 18.2.3: "The first Confirm message from the client on the
// interface MUST be delayed by a random amount of time between 0 and
// CNF_MAX_DELAY." Every move delays its Confirm, as it delays its Rebind.
func (w *v6Worker) confirmAfterMove(now time.Time) v6End {
	if !w.waitUntil(w.ctx, now.Add(v6Scale(v6CnfMaxDelay, w.random()))) {
		return v6EndCanceled
	}
	logger := loggerPtr.Load()
	at := w.clock().Now()
	if len(w.lease.live(v6KindAddress, at)) == 0 {
		return v6EndExhausted // the addresses expired during the delay
	}
	// RFC 9915 Section 18.2.3: IRT CNF_TIMEOUT, MRT CNF_MAX_RT, MRC 0 and MRD
	// CNF_MAX_RD, cut where the last address expires.
	params := v6Retransmission{irt: v6CnfTimeout, mrt: v6CnfMaxRT, mrd: v6CnfMaxRD}
	if last := w.lease.lastExpiry(); !last.never {
		params.mrd = min(params.mrd, last.at.Sub(at))
	}
	msg, err := w.buildConfirm(w.lease, at)
	if err != nil {
		logger.Error("iface dhcp v6: cannot build Confirm", "iface", w.c.ifaceName, "err", err)
		return v6EndExhausted
	}
	link, ok := w.open()
	if !ok {
		return v6EndCanceled
	}
	lease := w.lease
	collector := &v6ConfirmCollector{w: w}
	out := w.runExchange(w.ctx, &v6Exchange{
		name:         "Confirm",
		link:         link,
		msg:          msg,
		params:       params,
		accept:       collector.accept,
		prepare:      func(m *dhcpv6.Message, now time.Time) { w.setConfirmIAs(m, lease, now) },
		windowClosed: collector.windowClosed,
	})
	closeV6Link(link, w.c.ifaceName)
	if out.end != v6EndAnswered {
		if out.end == v6EndExhausted {
			// RFC 9915 Section 18.2.3: "If the client receives no responses
			// before the message transmission process terminates, as described
			// in Section 15, the client SHOULD continue to use any leases, using
			// the last known lifetimes for those leases".
			logger.Debug("iface dhcp v6: no server answered the Confirm; the lease runs on its timers",
				"iface", w.c.ifaceName)
		}
		return out.end
	}
	// RFC 9915 Section 18.2.10.3: "If the client receives any Reply messages
	// that indicate a status of Success (explicit or implicit), the client can
	// use the addresses in the IA and ignore any messages that indicate a
	// NotOnLink status. When the client only receives one or more Reply
	// messages with the NotOnLink status in response to a Confirm message, the
	// client performs DHCP server discovery as described in Section 18."
	// The collector answers a NotOnLink only when no Success arrived.
	if v6Status(out.answer) != iana.StatusNotOnLink {
		logger.Debug("iface dhcp v6: a server confirmed the leased addresses after the link came back up",
			"iface", w.c.ifaceName)
		return v6EndAnswered
	}
	logger.Info("iface dhcp v6: the server answered NotOnLink to the Confirm; the leased addresses do not fit this link, soliciting again",
		"iface", w.c.ifaceName)
	w.dropAddresses()
	return v6EndAnswered
}

// dropAddresses forgets every address binding of the lease, removes it from
// the device and publishes its expiry, as an expiry does.
func (w *v6Worker) dropAddresses() {
	for _, b := range w.lease.live(v6KindAddress, w.clock().Now()) {
		if _, ok := w.lease.remove(v6KindAddress, b.prefix); !ok {
			continue
		}
		w.drop(b)
	}
	if err := w.registerLeased(); err != nil {
		loggerPtr.Load().Debug("iface dhcp v6: leased-address registration failed", "iface", w.c.ifaceName, "err", err)
	}
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
// down is read would decline a unique address. A gone address is then judged
// by goneDuplicates against the link state the backend reported, because the
// kernel deletes the addresses right after it reports the down, and the read
// can land before the monitor delivers that down.
//
// A read that cannot answer declines nothing: a failed read leaves every
// address pending for the next poll. A backend that does not report
// addresses (iface.ErrAddressesNotReported), or reports them without their
// DAD outcome (AddressState.DADReported false, the VPP backend), leaves the
// outcome unknown: an address carrying no flag there is not a unique one, so
// the addresses are dropped from the poll, undeclined, with one Warn.
func (w *v6Worker) checkDAD() bool {
	if len(w.dadPending) == 0 {
		return true
	}
	if w.drainLinks() {
		w.noteMove()
	}
	read, err := iface.ReadAddressState(w.c.ifaceName)
	if errors.Is(err, iface.ErrAddressesNotReported) {
		w.dadUnreported(err)
		return true
	}
	if err != nil {
		loggerPtr.Load().Debug("iface dhcp v6: cannot read the DAD outcome, retrying",
			"iface", w.c.ifaceName, "err", err)
		return true
	}
	if !read.DADReported {
		w.dadUnreported(errDADNotReported)
		return true
	}
	var duplicates, gone []netip.Prefix
	pending := w.dadPending[:0]
	for _, p := range w.dadPending {
		outcome := v6DADOf(read.Addresses, p.Addr())
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
			gone = append(gone, p)
		case v6DADUnspecified:
			panic("BUG: v6DADOf answered no outcome")
		}
	}
	w.dadPending = pending
	if len(gone) > 0 {
		duplicates = append(duplicates, w.goneDuplicates(gone, read.State)...)
	}
	if len(duplicates) == 0 {
		return true
	}
	return w.decline(duplicates) != v6EndCanceled
}

// errDADNotReported is the cause dadUnreported logs for a backend that
// reports a device's addresses without their DAD outcome.
var errDADNotReported = errors.New("the iface backend reports interface addresses without their duplicate address detection outcome")

// dadUnreported drops the pending addresses from the DAD poll without
// declining them, because the backend shows no DAD outcome: it does not
// report the device's addresses, or reports them without the outcome (err
// says which). The Warn is logged once for the worker.
func (w *v6Worker) dadUnreported(err error) {
	if !w.unreportedWarned {
		w.unreportedWarned = true
		loggerPtr.Load().Warn("iface dhcp v6: the iface backend does not report the duplicate address detection outcome of the leased addresses, so ze cannot tell a duplicate from a unique address and declines none of them; run the DHCPv6 client on an interface of the kernel (netlink) iface backend to have duplicates declined",
			"iface", w.c.ifaceName, "addresses", w.dadPending, "err", err)
	}
	w.dadPending = w.dadPending[:0]
}

// goneDuplicates judges the addresses gone from the device while no down
// event had been read, against state, the link state the backend reported in
// the call that found them gone, and answers the ones that are duplicates.
//
// The kernel reports the down (RTM_NEWLINK) and then deletes every address
// of finite lifetime (addrconf_ifdown), and the monitor delivers the down to
// w.links later, so a DAD read can find the address gone and the device down
// before the worker holds the down event. That address went with the link:
// the worker records the down itself, so the up that follows completes the
// move and noteMove installs it again even when the down event is late or
// was dropped (iface.Subscribe keeps the newest event, not every one).
//
// An up state is not enough to decline: the kernel backend reads the link
// before the addresses, so a device set down between the two reads up with
// its addresses gone. goneWhileUp reads again and judges the link state
// taken after the addresses were read.
//
// A state that is neither "up" nor "down" says nothing about why the address
// is gone, so nothing is declined on it: the addresses stay pending, the
// worker logs one Warn for each distinct state, and the next DAD poll reads
// again.
func (w *v6Worker) goneDuplicates(gone []netip.Prefix, state string) []netip.Prefix {
	switch state {
	case iface.LinkStateUp:
		return w.goneWhileUp(gone)
	case iface.LinkStateDown:
		w.linkDown = true
		return nil
	default: // the backend's state string is open; an unknown one decides nothing
		w.warnLinkState(state, gone)
		w.dadPending = append(w.dadPending, gone...)
		return nil
	}
}

// goneWhileUp judges addresses an up read found gone against a second read,
// whose link state is read after the first read's addresses, and declines
// them only when that state is up too and no move is queued since.
//
// A second read that is not up is judged as goneDuplicates judges it; it
// never recurses back here, so the depth is one. A second read that fails
// declines nothing and leaves the addresses pending. With the device still
// up, the link events queued since the first drain are read again before the
// addresses are declined: a down and up that both happened before the read
// leave the device up and the address gone, and the move they complete
// installs it again. That holds only once the monitor has delivered both
// events to w.links. A down and up between the first and the second read
// whose events are not yet delivered find nothing to drain, so a unique
// address the kernel deleted with the down is declined as a duplicate.
func (w *v6Worker) goneWhileUp(gone []netip.Prefix) []netip.Prefix {
	later, err := iface.ReadAddressState(w.c.ifaceName)
	if err != nil {
		loggerPtr.Load().Debug("iface dhcp v6: cannot read the link state again, retrying",
			"iface", w.c.ifaceName, "err", err)
		w.dadPending = append(w.dadPending, gone...)
		return nil
	}
	if later.State != iface.LinkStateUp {
		return w.goneDuplicates(gone, later.State)
	}
	if w.drainLinks() {
		w.noteMove()
		return nil
	}
	if w.linkDown {
		return nil // the down arrived meanwhile; the up will install them again
	}
	return gone
}

// warnLinkState logs, once for each distinct state, that the backend
// reported a link state that is neither up nor down.
func (w *v6Worker) warnLinkState(state string, gone []netip.Prefix) {
	if w.linkStatesWarned[state] {
		return
	}
	if w.linkStatesWarned == nil {
		w.linkStatesWarned = make(map[string]bool)
	}
	w.linkStatesWarned[state] = true
	loggerPtr.Load().Warn("iface dhcp v6: an address left the device and the iface backend reported a link state that is neither up nor down, so ze cannot tell a duplicate from a link change; it declines nothing and reads the device again at each duplicate address detection poll. Check the device's state with 'ip link show <device>'; an address still gone once the device reads up is declined then",
		"iface", w.c.ifaceName, "device", w.device, "state", strconv.Quote(state), "addresses", gone)
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

// Design: docs/features/interfaces.md -- DHCPv6 client lifecycle
// Overview: dhcp_linux.go -- DHCP client types and lifecycle
// Detail: dhcp_v6_exchange_linux.go -- message builders and the retransmission engine
// Detail: dhcp_v6_lease_linux.go -- the lease record and its timers

//go:build linux

package ifacedhcp

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/iana"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/clock"
)

// v6DeviceRetry is the wait before a failed interface lookup or socket open
// is tried again.
const v6DeviceRetry = 5 * time.Second

// v6Worker is the state of one DHCPv6 worker. It is created by runV6 and
// owned by the v6 worker goroutine alone: nothing in it is shared with the v4
// worker or read by another goroutine.
type v6Worker struct {
	c     *DHCPClient
	ctx   context.Context // ends when Stop is called
	owner string          // leased-address registry owner

	duid   dhcpv6.DUID
	iaidNA [4]byte
	iaidPD [4]byte

	solMaxRT time.Duration // SOL_MAX_RT for this interface
	limiter  v6RateLimiter
	outage   bool          // a Warn was logged for the current outage
	backoff  time.Duration // wait before re-soliciting after a lease-less answer

	lease   *v6Lease         // nil when the client holds no lease
	device  string           // kernel device of the last opened socket
	lastMAC net.HardwareAddr // link-layer address of that device
}

func (w *v6Worker) clock() clock.Clock { return w.c.clock }

// random is uniform in [0, 1).
func (w *v6Worker) random() float64 { return w.c.v6Random() }

// randomSigned is RAND: uniform in [-0.1, +0.1].
func (w *v6Worker) randomSigned() float64 { return (w.random()*2 - 1) * 0.1 }

// runV6 is the long-lived DHCPv6 worker: it acquires a lease, keeps it with
// Renew and Rebind for as long as a server answers, and solicits again once
// every lease has expired. On Stop it removes the leased addresses and sends a
// Release.
func (c *DHCPClient) runV6() {
	ctx, cancel := c.stoppableContext()
	defer cancel()
	w := &v6Worker{
		c:        c,
		ctx:      ctx,
		owner:    "iface-dhcp-v6 " + c.ifaceName + " unit " + c.unit,
		solMaxRT: v6SolMaxRT,
		backoff:  v6SolTimeout,
	}
	defer iface.UnregisterLeasedAddresses(w.owner)

	if !w.identify() {
		return
	}
	// The loop ends only on Stop: a lost lease starts a new acquisition.
	for !c.stopped() {
		if !w.acquire() {
			break
		}
		w.maintain()
	}
	w.shutdown()
}

// identify settles the DUID and the IAIDs for the life of the worker.
//
// RFC 9915 Section 11: the DUID "SHOULD NOT change over time if at all
// possible". The default is a DUID-LL from the device MAC, which is stable
// across restarts; RFC 9915 Section 11.4's "A DUID-LL SHOULD NOT be used by
// DHCP clients or servers that cannot tell whether or not a network interface
// is permanently attached to the device on which the DHCP client is running"
// is a recorded deviation (owner decision Q4), and the duid leaf overrides it.
func (w *v6Worker) identify() bool {
	logger := loggerPtr.Load()
	link, ok := w.open()
	if !ok {
		return false
	}
	closeV6Link(link, w.c.ifaceName)
	mac := w.lastMAC

	w.iaidNA = v6IAIDFromMAC(mac)
	w.iaidPD = w.iaidNA
	w.iaidPD[0] ^= 0xff // distinct from the IA_NA IAID, stable with it

	if w.c.config.DUID != "" {
		duid, err := parseV6DUID(w.c.config.DUID)
		if err != nil {
			logger.Error("iface dhcp v6: the configured duid cannot be used, DHCPv6 is off on this unit -- set duid to colon-separated hex octets, a 2-octet DUID type then its identifier, e.g. 00:03:00:01:52:54:00:12:34:56",
				"iface", w.c.ifaceName, "duid", w.c.config.DUID, "err", err)
			return false
		}
		w.duid = duid
		return true
	}
	if len(mac) == 0 {
		logger.Error("iface dhcp v6: the device has no link-layer address to build a DUID-LL from, DHCPv6 is off on this unit -- set dhcpv6 duid on the unit",
			"iface", w.c.ifaceName, "device", w.device)
		return false
	}
	w.duid = &dhcpv6.DUIDLL{HWType: iana.HWTypeEthernet, LinkLayerAddr: mac}
	return true
}

// v6IAIDFromMAC is the last four octets of the MAC, the library's convention,
// so a binding a server holds from before keeps its IAID.
func v6IAIDFromMAC(mac net.HardwareAddr) [4]byte {
	var iaid [4]byte
	n := min(len(mac), 4)
	copy(iaid[4-n:], mac[len(mac)-n:])
	return iaid
}

// parseV6DUID parses colon-separated hex octets into a DUID.
func parseV6DUID(text string) (dhcpv6.DUID, error) {
	raw, err := hex.DecodeString(strings.ReplaceAll(text, ":", ""))
	if err != nil {
		return nil, fmt.Errorf("not hex octets: %w", err)
	}
	return dhcpv6.DUIDFromBytes(raw)
}

// open resolves the kernel device and opens a socket on it, retrying until
// Stop. Every exchange opens its own socket on the device resolved for it, so
// a selector-bound unit always binds the right device.
func (w *v6Worker) open() (v6Link, bool) {
	// The bound: Stop ends the wait.
	for {
		link, err := w.openOnce()
		if err == nil {
			return link, true
		}
		w.warnOutage("cannot open the DHCPv6 socket, retrying in 5s", "err", err)
		if !w.waitUntil(w.ctx, w.clock().Now().Add(v6DeviceRetry)) {
			return nil, false
		}
	}
}

func (w *v6Worker) openOnce() (v6Link, error) {
	binding, err := iface.Resolve(w.c.ifaceName)
	if err != nil {
		return nil, fmt.Errorf("interface lookup: %w", err)
	}
	link, mac, err := w.c.v6Dial(binding.OsName)
	if err != nil {
		return nil, err
	}
	w.device = binding.OsName
	w.lastMAC = mac
	return link, nil
}

func closeV6Link(link v6Link, ifaceName string) {
	if err := link.close(); err != nil {
		loggerPtr.Load().Debug("iface dhcp v6: socket close failed", "iface", ifaceName, "err", err)
	}
}

// acquire runs Solicit (and Request) until a lease is held. false on Stop.
func (w *v6Worker) acquire() bool {
	// The bound: Stop ends it; every answer that gives no lease waits a
	// backoff that doubles up to SOL_MAX_RT.
	for {
		answer, at, ok := w.solicit()
		if !ok {
			return false
		}
		if answer.MessageType == dhcpv6.MessageTypeAdvertise {
			reply, rat, end := w.request(answer)
			if end == v6EndCanceled {
				return false
			}
			if end != v6EndAnswered {
				continue
			}
			answer, at = reply, rat
		}
		if w.takeLease(answer, at) {
			w.backoff = v6SolTimeout
			return true
		}
		if !w.waitUntil(w.ctx, w.clock().Now().Add(w.backoff)) {
			return false
		}
		w.backoff = min(2*w.backoff, w.solMaxRT)
	}
}

// solicit runs one Solicit exchange to its answer: a Rapid Commit Reply or
// the chosen Advertise. false on Stop.
func (w *v6Worker) solicit() (*dhcpv6.Message, time.Time, bool) {
	link, ok := w.open()
	if !ok {
		return nil, time.Time{}, false
	}
	defer closeV6Link(link, w.c.ifaceName)

	msg, err := w.buildSolicit()
	if err != nil {
		loggerPtr.Load().Error("iface dhcp v6: cannot build Solicit", "iface", w.c.ifaceName, "err", err)
		return nil, time.Time{}, false
	}
	// RFC 9915 Section 18.2.1: "The first Solicit message from the client on
	// the interface SHOULD be delayed by a random amount of time between 0
	// and SOL_MAX_DELAY."
	if !w.waitUntil(w.ctx, w.clock().Now().Add(v6Scale(v6SolMaxDelay, w.random()))) {
		return nil, time.Time{}, false
	}
	collector := &v6AdvertiseCollector{w: w}
	// RFC 9915 Section 18.2.1: Solicit retransmits with IRT SOL_TIMEOUT, MRT
	// SOL_MAX_RT, and MRC and MRD 0.
	out := w.runExchange(w.ctx, &v6Exchange{
		name:         "Solicit",
		link:         link,
		msg:          msg,
		params:       v6Retransmission{irt: v6SolTimeout, solicitMRT: true, firstAboveIRT: true},
		accept:       collector.accept,
		windowClosed: collector.windowClosed,
	})
	if out.end != v6EndAnswered {
		return nil, time.Time{}, false
	}
	return out.answer, out.at, true
}

// request runs a Request exchange answering advertise.
//
// RFC 9915 Section 18.2.2: Request retransmits with IRT REQ_TIMEOUT, MRT
// REQ_MAX_RT, MRC REQ_MAX_RC and MRD 0; when it fails the client restarts
// server discovery.
func (w *v6Worker) request(advertise *dhcpv6.Message) (*dhcpv6.Message, time.Time, v6End) {
	msg, err := w.buildRequest(advertise)
	if err != nil {
		loggerPtr.Load().Error("iface dhcp v6: cannot build Request", "iface", w.c.ifaceName, "err", err)
		return nil, time.Time{}, v6EndExhausted
	}
	return w.requestWith(msg)
}

func (w *v6Worker) requestWith(msg *dhcpv6.Message) (*dhcpv6.Message, time.Time, v6End) {
	link, ok := w.open()
	if !ok {
		return nil, time.Time{}, v6EndCanceled
	}
	defer closeV6Link(link, w.c.ifaceName)
	out := w.runExchange(w.ctx, &v6Exchange{
		name:   "Request",
		link:   link,
		msg:    msg,
		params: v6Retransmission{irt: v6ReqTimeout, mrt: v6ReqMaxRT, mrc: v6ReqMaxRC},
		accept: w.replyMatcher(),
	})
	return out.answer, out.at, out.end
}

// takeLease starts a lease from the Reply of an acquisition. false when the
// Reply leased nothing usable.
func (w *v6Worker) takeLease(reply *dhcpv6.Message, at time.Time) bool {
	lease := &v6Lease{server: reply.Options.ServerID()}
	applied := lease.apply(reply, at, w.iaidNA, w.iaidPD)
	if lease.empty() {
		w.warnOutage("the DHCPv6 server leased no usable address or prefix, soliciting again",
			"discarded", applied.discarded)
		return false
	}
	w.lease = lease
	w.effect(applied, iface.TopicDHCPLeaseAcquired)
	loggerPtr.Load().Info("iface dhcp v6: lease obtained", "iface", w.c.ifaceName,
		"addresses", len(lease.live(v6KindAddress, at)), "prefixes", len(lease.live(v6KindPrefix, at)))
	return true
}

// maintain keeps the lease until every binding has expired, or Stop.
func (w *v6Worker) maintain() {
	logger := loggerPtr.Load()
	// The bound: each pass waits for a timer or an exchange; the lease ends
	// when its bindings expire.
	for {
		w.expireDue()
		if w.lease.empty() {
			w.lease = nil
			w.c.setV6Resolvers(nil)
			return
		}
		t1, t2 := w.lease.renewTimes()
		next := t1.earlier(w.lease.nextExpiry())
		if next.never {
			// RFC 9915 Section 7.7: an infinite T1 is never renewed, and an
			// infinite lease never expires: wait for Stop.
			<-w.ctx.Done()
			return
		}
		if !w.waitUntil(w.ctx, next.at) {
			return
		}
		now := w.clock().Now()
		if t1.never {
			continue // a binding expired first
		}
		if now.Before(t1.at) {
			continue // a binding expired first
		}

		// RFC 9915 Section 18.2.4: "At time T1, the client initiates a
		// Renew/Reply message exchange to extend the lifetimes on any leases
		// in the IA."
		if t2.never || now.Before(t2.at) {
			end := w.extend(dhcpv6.MessageTypeRenew, t2.earlier(w.lease.lastExpiry()))
			if end == v6EndCanceled {
				return
			}
			if end == v6EndAnswered {
				continue
			}
			if t2.never {
				continue // the leases expired while renewing
			}
		}
		// RFC 9915 Section 18.2.5: "At time T2 (which will only be reached if
		// the server to which the Renew message was sent starting at time T1
		// has not responded), the client initiates a Rebind/Reply message
		// exchange with any available server."
		end := w.extend(dhcpv6.MessageTypeRebind, w.lease.lastExpiry())
		if end != v6EndExhausted {
			if end == v6EndCanceled {
				return
			}
			continue
		}
		logger.Warn("iface dhcp v6: no DHCPv6 server extended the lease before it expired, soliciting again",
			"iface", w.c.ifaceName)
		w.outage = true
	}
}

// extend runs a Renew or Rebind whose MRD ends at until, and applies its
// answer.
//
// RFC 9915 Section 18.2.4: Renew uses IRT REN_TIMEOUT, MRT REN_MAX_RT, MRC 0
// and MRD the "Remaining time until earliest T2"; RFC 9915 Section 18.2.5:
// Rebind uses IRT REB_TIMEOUT, MRT REB_MAX_RT, MRC 0 and MRD the "Remaining
// time until valid lifetimes of all leases in all IAs have expired".
func (w *v6Worker) extend(kind dhcpv6.MessageType, until v6Instant) v6End {
	now := w.clock().Now()
	params := v6Retransmission{irt: v6RenTimeout, mrt: v6RenMaxRT}
	name := "Renew"
	if kind == dhcpv6.MessageTypeRebind {
		params = v6Retransmission{irt: v6RebTimeout, mrt: v6RebMaxRT}
		name = "Rebind"
	}
	if !until.never {
		params.mrd = until.at.Sub(now)
		if params.mrd <= 0 {
			return v6EndExhausted
		}
	}
	msg, err := w.buildFromLease(kind, w.lease, now)
	if err != nil {
		loggerPtr.Load().Error("iface dhcp v6: cannot build "+name, "iface", w.c.ifaceName, "err", err)
		return v6EndExhausted
	}
	link, ok := w.open()
	if !ok {
		return v6EndCanceled
	}
	lease := w.lease
	out := w.runExchange(w.ctx, &v6Exchange{
		name:    name,
		link:    link,
		msg:     msg,
		params:  params,
		accept:  w.replyMatcher(),
		prepare: func(m *dhcpv6.Message, at time.Time) { w.setLeaseIAs(m, kind, lease, at) },
	})
	closeV6Link(link, w.c.ifaceName)
	if out.end != v6EndAnswered {
		return out.end
	}
	// The answering server leases from now on: the same one after a Renew,
	// any one after a Rebind.
	w.lease.server = out.answer.Options.ServerID()
	return w.applyExtension(out.answer, out.at)
}

// applyExtension applies a Renew or Rebind Reply.
//
// RFC 9915 Section 18.2.10.1: "When the client receives a Reply message in
// response to a Renew or Rebind message, the client: * Sends a Request
// message to the server that responded if any of the IAs in the Reply message
// contain the NoBinding status code." The Request carries every IA it holds.
func (w *v6Worker) applyExtension(reply *dhcpv6.Message, at time.Time) v6End {
	applied := w.lease.apply(reply, at, w.iaidNA, w.iaidPD)
	w.effect(applied, iface.TopicDHCPLeaseRenewed)
	if !applied.noBinding {
		return v6EndAnswered
	}
	msg, err := w.buildFromLease(dhcpv6.MessageTypeRequest, w.lease, at)
	if err != nil {
		loggerPtr.Load().Error("iface dhcp v6: cannot build Request", "iface", w.c.ifaceName, "err", err)
		return v6EndAnswered
	}
	answer, rat, end := w.requestWith(msg)
	if end != v6EndAnswered {
		if end == v6EndCanceled {
			return end
		}
		return v6EndAnswered // the other bindings are kept; T1 retries
	}
	w.lease.server = answer.Options.ServerID()
	w.effect(w.lease.apply(answer, rat, w.iaidNA, w.iaidPD), iface.TopicDHCPLeaseRenewed)
	return v6EndAnswered
}

// effect installs, refreshes and removes what a Reply changed, publishes the
// events, and records the DNS servers.
func (w *v6Worker) effect(applied v6Applied, topic string) {
	logger := loggerPtr.Load()
	if applied.discarded > 0 {
		logger.Info("iface dhcp v6: discarded invalid leases or IAs from the server",
			"iface", w.c.ifaceName, "count", applied.discarded)
	}
	for _, b := range applied.removed {
		w.drop(b)
	}
	if err := w.registerLeased(); err != nil {
		logger.Warn("iface dhcp v6: leased-address registration refused, addresses not installed",
			"iface", w.c.ifaceName, "err", err)
		return
	}
	for _, b := range applied.installed {
		if b.kind == v6KindAddress {
			// RFC 9915 Section 18.2.10.1 requires duplicate address detection
			// per RFC 4862 Section 5.4 before use; the kernel performs it on
			// an address installed without IFA_F_NODAD.
			if err := iface.ReplaceAddressWithLifetime(w.c.ifaceName, b.prefix.String(), v6Seconds(b.valid), v6Seconds(b.preferred)); err != nil {
				logger.Warn("iface dhcp v6: address install failed",
					"iface", w.c.ifaceName, "addr", b.prefix.String(), "err", err)
				continue
			}
		}
		w.c.publishDHCP(topic, w.payload(b, int(b.valid/time.Second)))
	}
	// RFC 9915 Section 18.2.10.5: the Reply's servers replace the lease's;
	// a Reply without the option withdraws them.
	w.c.setV6Resolvers(w.lease.dns)
}

// v6Seconds is a lifetime as the kernel takes it: 0xffffffff is infinite.
func v6Seconds(d time.Duration) int { return int(d / time.Second) }

func (w *v6Worker) payload(b v6Binding, leaseTime int) iface.DHCPPayload {
	p := iface.DHCPPayload{
		Name:         w.c.ifaceName,
		Unit:         w.c.unit,
		Address:      b.prefix.Addr().String(),
		PrefixLength: b.prefix.Bits(),
		LeaseTime:    leaseTime,
	}
	if b.kind == v6KindAddress && len(w.lease.dns) > 0 {
		p.DNS = w.lease.dns[0]
		p.DNSAll = append([]string(nil), w.lease.dns...)
	}
	return p
}

// registerLeased declares the lease's live addresses to the address
// reconcile, so it spares them, an infinite one included. It MUST run before
// an address is installed and after one is removed.
func (w *v6Worker) registerLeased() error {
	var addrs []string
	if w.lease != nil {
		for _, b := range w.lease.live(v6KindAddress, w.clock().Now()) {
			addrs = append(addrs, b.prefix.String())
		}
	}
	return iface.RegisterLeasedAddresses(w.device, w.owner, addrs)
}

// drop removes one binding from the kernel (an address) and publishes its
// expiry.
//
// RFC 9915 Section 18.2.5: "The message exchange is terminated when the
// valid lifetimes of all leases across all IAs have expired"; each lease that
// expires is reported, an IA_PD prefix included.
func (w *v6Worker) drop(b v6Binding) {
	if b.kind == v6KindAddress {
		if err := iface.RemoveAddress(w.c.ifaceName, b.prefix.String()); err != nil {
			loggerPtr.Load().Debug("iface dhcp v6: address removal failed",
				"iface", w.c.ifaceName, "addr", b.prefix.String(), "err", err)
		}
	}
	w.c.publishDHCP(iface.TopicDHCPLeaseExpired, w.payload(b, 0))
}

// expireDue removes the bindings whose valid lifetime ran out.
func (w *v6Worker) expireDue() {
	gone := w.lease.prune(w.clock().Now())
	if len(gone) == 0 {
		return
	}
	for _, b := range gone {
		w.drop(b)
	}
	if err := w.registerLeased(); err != nil {
		loggerPtr.Load().Debug("iface dhcp v6: leased-address registration failed", "iface", w.c.ifaceName, "err", err)
	}
}

// shutdown removes the leased addresses and then releases the lease.
//
// RFC 9915 Section 18.2.7: "The client MUST stop using all of the leases
// being released before the client begins the Release message exchange
// process." The exchange uses IRT REL_TIMEOUT and MRC REL_MAX_RC, and is cut
// at v6ReleaseBound so Stop stays prompt.
func (w *v6Worker) shutdown() {
	if w.lease == nil {
		return
	}
	lease := w.lease
	now := w.clock().Now()
	for _, b := range lease.live(v6KindAddress, now) {
		if err := iface.RemoveAddress(w.c.ifaceName, b.prefix.String()); err != nil {
			loggerPtr.Load().Debug("iface dhcp v6: address removal failed",
				"iface", w.c.ifaceName, "addr", b.prefix.String(), "err", err)
		}
	}
	w.lease = nil
	w.c.setV6Resolvers(nil)
	if err := w.registerLeased(); err != nil {
		loggerPtr.Load().Debug("iface dhcp v6: leased-address registration failed", "iface", w.c.ifaceName, "err", err)
	}
	if len(lease.live(v6KindAddress, now)) == 0 && len(lease.live(v6KindPrefix, now)) == 0 {
		return
	}

	msg, err := w.buildFromLease(dhcpv6.MessageTypeRelease, lease, now)
	if err != nil {
		loggerPtr.Load().Debug("iface dhcp v6: cannot build Release", "iface", w.c.ifaceName, "err", err)
		return
	}
	link, err := w.openOnce()
	if err != nil {
		loggerPtr.Load().Debug("iface dhcp v6: no socket for Release", "iface", w.c.ifaceName, "err", err)
		return
	}
	defer closeV6Link(link, w.c.ifaceName)

	ctx, cancel := context.WithCancel(context.Background())
	bound := w.clock().AfterFunc(v6ReleaseBound, cancel)
	defer bound.Stop()
	defer cancel()
	out := w.runExchange(ctx, &v6Exchange{
		name:   "Release",
		link:   link,
		msg:    msg,
		params: v6Retransmission{irt: v6RelTimeout, mrc: v6RelMaxRC},
		accept: w.releaseMatcher(),
	})
	if out.end != v6EndAnswered {
		loggerPtr.Load().Debug("iface dhcp v6: Release unanswered", "iface", w.c.ifaceName)
	}
}

// releaseMatcher accepts any identified Reply: a Release Reply's status does
// not change what the client does.
func (w *v6Worker) releaseMatcher() func(*dhcpv6.Message) bool {
	return func(m *dhcpv6.Message) bool {
		if m.MessageType != dhcpv6.MessageTypeReply {
			return false
		}
		return v6Identified(m, w.duid)
	}
}

// warnOutage logs one Warn per outage; further failures go to Debug until
// an answer ends the outage.
func (w *v6Worker) warnOutage(msg string, args ...any) {
	args = append([]any{"iface", w.c.ifaceName}, args...)
	if w.outage {
		loggerPtr.Load().Debug("iface dhcp v6: "+msg, args...)
		return
	}
	w.outage = true
	loggerPtr.Load().Warn("iface dhcp v6: "+msg, args...)
}

// reportUnanswered logs an unanswered transmission: the first of an outage
// at Warn, naming the exchange and that it retransmits now with a growing
// timeout, every later one at Debug.
func (w *v6Worker) reportUnanswered(exchange string, transmissions int, rt time.Duration) {
	w.warnOutage("no DHCPv6 server answered, retransmitting now with exponential backoff",
		"exchange", exchange, "transmissions", transmissions, "last-timeout", rt)
}

// outageOver ends an outage when an answer arrives.
func (w *v6Worker) outageOver() {
	if !w.outage {
		return
	}
	w.outage = false
	loggerPtr.Load().Info("iface dhcp v6: a DHCPv6 server answered again", "iface", w.c.ifaceName)
}

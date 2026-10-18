// Design: docs/features/interfaces.md -- DHCPv4 client lifecycle
// Overview: dhcp_linux.go -- DHCP client types and lifecycle
// Related: dhcp_v4_exchange_linux.go -- the exchanger the loop sends through
// RFC: rfc/short/rfc2131.md

//go:build linux

package ifacedhcp

import (
	"encoding/binary"
	"net"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// v4ResolveBackoff is the wait after the interface or its device could
	// not be found.
	v4ResolveBackoff = 5 * time.Second
	// v4AcquireBackoff is the wait after a DORA exchange that bound nothing:
	// no answer, a DHCPNAK, a refused ACK, or an address that would not
	// install. It bounds a server that NAKs every REQUEST to one DORA per
	// interval.
	v4AcquireBackoff = 10 * time.Second
	// v4RetransmitFloor is the shortest wait between two extending
	// DHCPREQUESTs (RFC 2131 Section 4.4.5).
	v4RetransmitFloor = 60 * time.Second
	// v4ListenFloor is the shortest listen window an extending DHCPREQUEST
	// gets, so a route retry due an instant later never sends a request
	// nobody can answer.
	v4ListenFloor = time.Second
	// v4RouteRetryFirst and v4RouteRetryMax bound the default-route retry:
	// the first retry waits v4RouteRetryFirst, each later one twice the last,
	// never more than v4RouteRetryMax.
	v4RouteRetryFirst = 5 * time.Second
	v4RouteRetryMax   = 60 * time.Second
)

// v4Lease is a lease the worker holds: the ACK that granted it and the
// absolute deadlines measured from the instant its DHCPREQUEST was sent.
type v4Lease struct {
	ack *dhcpv4.DHCPv4
	// server is the leasing server's identifier, the RENEW destination. It is
	// nil only when neither the ACK nor the OFFER named one.
	server   net.IP
	duration time.Duration
	// infinite: the lease never runs out, and the deadlines below are unset.
	infinite bool
	renewAt  time.Time // T1
	rebindAt time.Time // T2
	expireAt time.Time
	// attemptAt is when the next extending DHCPREQUEST goes. It starts at T1.
	attemptAt time.Time
	// leaseTimeReported: a renewal ACK without a usable lease time was already
	// logged at Warn for this lease.
	leaseTimeReported bool
}

// v4Route is the default route the held lease asks for.
type v4Route struct {
	// gateway is the router the lease named, empty when it named none.
	gateway string
	// installed: an AddRoute for gateway succeeded, so teardown removes it.
	installed bool
	// pending: the last AddRoute failed and a retry is due at retryAt.
	pending bool
	retryAt time.Time
	backoff time.Duration
}

// v4State is the worker's state across leases. Read and written by the v4
// worker only.
type v4State struct {
	route v4Route
	// refusedServer is the server whose ACK the last DORA refused, so its
	// repeats log at Debug. Empty once a lease is accepted.
	refusedServer string
}

// runV4 is the long-lived DHCPv4 worker: the RFC 2131 Section 4.4 client
// state machine. Each pass is one INIT: acquire a lease, then hold it through
// BOUND, RENEWING and REBINDING until it expires, a server refuses it, or the
// client stops. The loop has no bound by design: it runs while the client is
// started and ends at Stop.
func (c *DHCPClient) runV4() {
	var w v4State
	for !c.stopped() {
		lease, ok := c.acquireV4(&w)
		if !ok {
			continue
		}
		c.holdV4(&w, lease)
	}
}

// acquireV4 runs INIT, SELECTING and REQUESTING: one DORA exchange on the
// interface's kernel device. It answers the installed lease, or false after
// its back-off wait (or at Stop) when nothing was bound.
func (c *DHCPClient) acquireV4(w *v4State) (*v4Lease, bool) {
	logger := loggerPtr.Load()

	// Fetched for this exchange: an os-name or mac/match entry binds the
	// logical name to another kernel device, and the binding can move.
	device, err := iface.ResolveDevice(c.ifaceName)
	if err != nil {
		logger.Warn("iface dhcp v4: interface lookup failed",
			"iface", c.ifaceName, "err", err)
		c.sleepOrStop(v4ResolveBackoff)
		return nil, false
	}

	ctx, ctxCancel := c.stoppableContext()
	reply := c.v4Exchange.acquire(ctx, device, c.v4RequestModifiers())
	ctxCancel()
	if c.stopped() {
		return nil, false
	}

	switch reply.outcome {
	case v4OutcomeAck:
		lease, refusal := v4LeaseFromAck(reply.msg, reply.sent, reply.offerServer)
		if refusal != nil {
			c.logV4Refusal(w, reply.msg, refusal)
			c.sleepOrStop(v4AcquireBackoff)
			return nil, false
		}
		w.refusedServer = ""
		if !c.installV4Lease(w, lease, iface.TopicDHCPLeaseAcquired) {
			c.sleepOrStop(v4AcquireBackoff)
			return nil, false
		}
		return lease, true
	case v4OutcomeNak:
		c.logV4Nak(reply.msg, "request")
	case v4OutcomeNoAnswer, v4OutcomeFailed:
		logger.Warn("iface dhcp v4: request failed",
			"iface", c.ifaceName, "device", device, "err", reply.err)
	case v4OutcomeUnspecified:
		panic("BUG: iface dhcp v4: acquire returned no outcome")
	}
	c.sleepOrStop(v4AcquireBackoff)
	return nil, false
}

// holdV4 runs BOUND, RENEWING and REBINDING for lease. It returns after
// teardown: at expiry or a DHCPNAK (both publish dhcp-expired) or at Stop
// (which publishes nothing). Each pass handles what is due, then waits for the
// next deadline; every deadline is in the future after its handler runs, so
// each pass either waits or advances one deadline.
func (c *DHCPClient) holdV4(w *v4State, lease *v4Lease) {
	for {
		if c.stopped() {
			c.teardownV4(w)
			return
		}
		now := c.clock.Now()
		if !lease.infinite && !now.Before(lease.expireAt) {
			// RFC 2131 Section 4.4.5: "If the lease expires before the client
			// receives a DHCPACK, the client moves to INIT state, MUST
			// immediately stop any other network processing and requests
			// network initialization parameters as if the client were
			// uninitialized."
			loggerPtr.Load().Info("iface dhcp v4: lease expired",
				"iface", c.ifaceName, "addr", c.v4Address)
			c.teardownV4(w)
			c.publishDHCP(iface.TopicDHCPLeaseExpired, c.v4Payload(lease.ack, lease.duration))
			return
		}
		if w.route.pending && !now.Before(w.route.retryAt) {
			c.installV4Route(&w.route)
			continue
		}
		if !lease.infinite && !now.Before(lease.attemptAt) {
			next, held := c.extendV4(w, lease)
			if !held {
				return
			}
			lease = next
			continue
		}
		wake, ok := v4NextWake(lease, &w.route)
		if !ok {
			// An infinite lease with its route in place: nothing is ever due.
			<-c.stop
			continue
		}
		c.sleepUntilOrStop(wake)
	}
}

// v4NextWake is the earliest deadline holdV4 waits for: the next extending
// DHCPREQUEST, the expiry, or the route retry. false when none exists.
func v4NextWake(lease *v4Lease, route *v4Route) (time.Time, bool) {
	var wake time.Time
	found := false
	if !lease.infinite {
		wake = lease.attemptAt
		if lease.expireAt.Before(wake) {
			wake = lease.expireAt
		}
		found = true
	}
	if route.pending && (!found || route.retryAt.Before(wake)) {
		wake = route.retryAt
		found = true
	}
	return wake, found
}

// extendV4 sends one extending DHCPREQUEST: RENEWING before T2, REBINDING
// from T2. It answers the lease held afterwards, and false when a DHCPNAK
// ended it (the teardown is done).
func (c *DHCPClient) extendV4(w *v4State, lease *v4Lease) (*v4Lease, bool) {
	logger := loggerPtr.Load()
	now := c.clock.Now()
	rebinding := !now.Before(lease.rebindAt)
	phaseEnd := lease.rebindAt
	if rebinding {
		phaseEnd = lease.expireAt
	}
	// RFC 2131 Section 4.4.5
	lease.attemptAt = v4NextAttempt(now, phaseEnd)

	reply := c.sendV4Extend(w, lease, rebinding, now)
	if c.stopped() {
		return lease, true
	}

	switch reply.outcome {
	case v4OutcomeAck:
		next, refusal := v4LeaseFromAck(reply.msg, reply.sent, lease.server)
		if refusal != nil {
			// The held lease keeps its own deadlines: an ACK that grants no
			// lease time is no answer (RFC 2131 Section 4.3.1 Table 3).
			if !lease.leaseTimeReported {
				lease.leaseTimeReported = true
				c.logV4RenewalRefusal(reply.msg, refusal)
			}
			return lease, true
		}
		if !c.installV4Lease(w, next, iface.TopicDHCPLeaseRenewed) {
			return lease, true
		}
		return next, true
	case v4OutcomeNak:
		// RFC 2131 Section 3.1: "If the client receives a DHCPNAK message,
		// the client restarts the configuration process."
		c.logV4Nak(reply.msg, phaseName(rebinding))
		c.teardownV4(w)
		c.publishDHCP(iface.TopicDHCPLeaseExpired, c.v4Payload(lease.ack, lease.duration))
		return nil, false
	case v4OutcomeNoAnswer:
		logger.Debug("iface dhcp v4: no answer to lease extension",
			"iface", c.ifaceName, "state", phaseName(rebinding), "next", lease.attemptAt)
	case v4OutcomeFailed:
		logger.Warn("iface dhcp v4: lease extension not sent",
			"iface", c.ifaceName, "state", phaseName(rebinding), "err", reply.err,
			"next", lease.attemptAt)
	case v4OutcomeUnspecified:
		panic("BUG: iface dhcp v4: extend returned no outcome")
	}
	return lease, true
}

// sendV4Extend builds and sends one extending DHCPREQUEST for lease.
func (c *DHCPClient) sendV4Extend(w *v4State, lease *v4Lease, rebinding bool, now time.Time) v4Reply {
	device, err := iface.ResolveDevice(c.ifaceName)
	if err != nil {
		return v4Reply{outcome: v4OutcomeFailed, sent: now, err: err}
	}
	request, err := c.v4ExtendRequest(lease.ack)
	if err != nil {
		return v4Reply{outcome: v4OutcomeFailed, sent: now, err: err}
	}

	attempt := v4Attempt{
		device:   device,
		request:  request,
		delivery: v4DeliveryBroadcast,
		source:   lease.ack.YourIPAddr,
		server:   lease.server,
		accept:   v4ExtendAccepts(rebinding, lease.server),
		window:   v4ListenWindow(now, lease.attemptAt, &w.route),
	}
	if !rebinding && lease.server != nil {
		// RFC 2131 Section 4.4.5: "At time T1 the client moves to RENEWING
		// state and sends (via unicast) a DHCPREQUEST message to the server
		// to extend its lease."
		attempt.delivery = v4DeliveryUnicast
	}

	ctx, ctxCancel := c.stoppableContext()
	defer ctxCancel()
	return c.v4Exchange.extend(ctx, attempt)
}

// v4ListenWindow is how long an extending DHCPREQUEST listens: until the next
// attempt or the route retry, whichever comes first, and at least
// v4ListenFloor. Listening up to the next attempt means only an answer later
// than the next transmission is lost, and that transmission draws the same
// answer.
func v4ListenWindow(now, nextAttempt time.Time, route *v4Route) time.Duration {
	until := nextAttempt
	if route.pending && route.retryAt.Before(until) {
		until = route.retryAt
	}
	return max(until.Sub(now), v4ListenFloor)
}

// v4NextAttempt is when the next extending DHCPREQUEST goes, given the instant
// this one is sent and the end of its phase (T2 in RENEWING, the expiry in
// REBINDING).
//
// RFC 2131 Section 4.4.5: "In both RENEWING and REBINDING states, if the
// client receives no response to its DHCPREQUEST message, the client SHOULD
// wait one-half of the remaining time until T2 (in RENEWING state) and
// one-half of the remaining lease time (in REBINDING state), down to a minimum
// of 60 seconds, before retransmitting the DHCPREQUEST message."
//
// A wait that reaches the phase end moves to the phase end, where the next
// phase's first transmission (or the expiry) takes over.
func v4NextAttempt(sent, phaseEnd time.Time) time.Time {
	remaining := phaseEnd.Sub(sent)
	wait := max(remaining/2, v4RetransmitFloor)
	if wait >= remaining {
		return phaseEnd
	}
	return sent.Add(wait)
}

// phaseName names the extending state for a log line.
func phaseName(rebinding bool) string {
	if rebinding {
		return "rebinding"
	}
	return "renewing"
}

// v4ExtendRequest builds the DHCPREQUEST that extends the lease held ack
// granted: ciaddr is the leased address, and neither option 54 nor option 50
// is carried. The acquisition modifiers come too, so option 61 and option 12
// match the DORA REQUEST, and clearReservedFlags runs last.
//
// RFC 2131 Section 2: "If the client uses a 'client identifier' in one
// message, it MUST use that same identifier in all subsequent messages, to
// ensure that all servers correctly identify the client." The identifier is
// the configured client-id, so the same bytes go out every time.
func (c *DHCPClient) v4ExtendRequest(held *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, error) {
	return dhcpv4.NewRenewFromAck(held, dhcpv4.PrependModifiers(c.v4RequestModifiers(),
		dhcpv4.WithOption(dhcpv4.OptMaxMessageSize(nclient4.MaxMessageSize)))...)
}

// v4Refusal is why an ACK is not accepted as a lease, and what fixes it.
type v4Refusal struct {
	reason string
	fix    string
}

func (r *v4Refusal) Error() string { return r.reason }

var (
	errV4NoAddress = &v4Refusal{
		reason: "the ACK carries no address (yiaddr is 0.0.0.0)",
		fix:    "the DHCP server must assign an address in its DHCPACK",
	}
	errV4LeaseTimeAbsent = &v4Refusal{
		reason: "the ACK carries no IP address lease time (option 51), which RFC 2131 Section 4.3.1 Table 3 requires in a DHCPACK",
		fix:    "configure the DHCP server to send a lease time (option 51)",
	}
	errV4LeaseTimeZero = &v4Refusal{
		reason: "the ACK grants an IP address lease time (option 51) of 0 seconds, which leases nothing",
		fix:    "configure the DHCP server to send a non-zero lease time (option 51)",
	}
)

// v4LeaseFromAck validates ack and derives the lease it grants, its deadlines
// measured from sent. fallbackServer names the leasing server when the ACK
// carries no option 54.
//
// RFC 2131 Section 4.4.5: "When the client receives a DHCPACK from the
// server, the client computes the lease expiration time as the sum of the
// time at which the client sent the DHCPREQUEST message and the duration of
// the lease in the DHCPACK message." T1 and T2 are measured from the same
// instant.
func v4LeaseFromAck(ack *dhcpv4.DHCPv4, sent time.Time, fallbackServer net.IP) (*v4Lease, *v4Refusal) {
	if ack.YourIPAddr == nil || ack.YourIPAddr.IsUnspecified() {
		return nil, errV4NoAddress
	}
	duration, refusal := v4LeaseTime(ack)
	if refusal != nil {
		return nil, refusal
	}

	lease := &v4Lease{ack: ack, server: ack.ServerIdentifier(), duration: duration}
	if lease.server == nil {
		lease.server = fallbackServer
	}
	if duration >= v4LeaseInfinite {
		lease.infinite = true
		return lease, nil
	}

	t1, t2, rejected := v4LeaseTimers(duration,
		ack.Options.Get(dhcpv4.OptionRenewTimeValue), ack.Options.Get(dhcpv4.OptionRebindingTimeValue))
	if rejected {
		loggerPtr.Load().Debug("iface dhcp v4: server T1/T2 rejected, using the defaults",
			"server", lease.server, "lease", duration,
			"option-58", ack.Options.Get(dhcpv4.OptionRenewTimeValue),
			"option-59", ack.Options.Get(dhcpv4.OptionRebindingTimeValue))
	}
	lease.renewAt = sent.Add(t1)
	lease.rebindAt = sent.Add(t2)
	lease.expireAt = sent.Add(duration)
	lease.attemptAt = lease.renewAt
	return lease, nil
}

// v4LeaseTime reads option 51. It answers the lease time, or the refusal that
// says whether the option is absent (or malformed) or grants 0 seconds; it
// never invents a lifetime.
//
// RFC 2131 Section 4.3.1 Table 3 marks the IP address lease time MUST in a
// DHCPACK to a DHCPREQUEST.
func v4LeaseTime(ack *dhcpv4.DHCPv4) (time.Duration, *v4Refusal) {
	raw := ack.Options.Get(dhcpv4.OptionIPAddressLeaseTime)
	if len(raw) != 4 {
		return 0, errV4LeaseTimeAbsent
	}
	seconds := binary.BigEndian.Uint32(raw)
	if seconds == 0 {
		return 0, errV4LeaseTimeZero
	}
	return time.Duration(seconds) * time.Second, nil
}

// v4LeaseTimers returns T1 and T2 for a finite lease: the server's options 58
// and 59 when they keep T1 < T2 < lease, else both defaults. rejected reports
// that the server sent a value and it was not used. An option alone is judged
// against the default for the other.
//
// RFC 2131 Section 4.4.5: "Times T1 and T2 are configurable by the server
// through options." and "T1 MUST be earlier than T2, which, in turn, MUST be
// earlier than the time at which the client's lease will expire." Values that
// break the order are not used.
func v4LeaseTimers(lease time.Duration, serverT1, serverT2 []byte) (t1, t2 time.Duration, rejected bool) {
	defaultT1, defaultT2 := v4RenewalTimes(lease)
	if len(serverT1) == 0 && len(serverT2) == 0 {
		return defaultT1, defaultT2, false
	}
	t1, ok1 := v4OptionSeconds(serverT1, defaultT1)
	t2, ok2 := v4OptionSeconds(serverT2, defaultT2)
	if !ok1 || !ok2 {
		return defaultT1, defaultT2, true
	}
	if t1 <= 0 || t1 >= t2 || t2 >= lease {
		return defaultT1, defaultT2, true
	}
	return t1, t2, false
}

// v4OptionSeconds decodes a four-octet seconds option, or answers fallback
// when the option is absent. false when the option is present but malformed.
func v4OptionSeconds(raw []byte, fallback time.Duration) (time.Duration, bool) {
	if len(raw) == 0 {
		return fallback, true
	}
	if len(raw) != 4 {
		return 0, false
	}
	return time.Duration(binary.BigEndian.Uint32(raw)) * time.Second, true
}

// v4RenewalTimes returns the default T1 and T2 for a finite lease.
//
// RFC 2131 Section 4.4.5: "T1 defaults to (0.5 * duration_of_lease).  T2
// defaults to (0.875 * duration_of_lease)." T2 is computed as the lease less
// an eighth, never as seven eighths of it: leaseTime * 7 overflows a
// time.Duration for a lease above 41 years.
func v4RenewalTimes(leaseTime time.Duration) (t1, t2 time.Duration) {
	return leaseTime / 2, leaseTime - leaseTime/8
}

// logV4Refusal reports an ACK the DORA refused: Warn the first time a server
// sends one, Debug for its repeats until a lease is accepted.
func (c *DHCPClient) logV4Refusal(w *v4State, ack *dhcpv4.DHCPv4, refusal *v4Refusal) {
	server := v4ServerName(ack)
	logger := loggerPtr.Load()
	if w.refusedServer == server {
		logger.Debug("iface dhcp v4: lease refused again",
			"iface", c.ifaceName, "server", server, "reason", refusal.reason)
		return
	}
	w.refusedServer = server
	logger.Warn("iface dhcp v4: lease refused, nothing installed",
		"iface", c.ifaceName, "server", server, "reason", refusal.reason, "fix", refusal.fix)
}

// logV4RenewalRefusal reports a renewal ACK that grants no usable lease time.
func (c *DHCPClient) logV4RenewalRefusal(ack *dhcpv4.DHCPv4, refusal *v4Refusal) {
	loggerPtr.Load().Warn("iface dhcp v4: renewal answer ignored, the lease held is kept",
		"iface", c.ifaceName, "server", v4ServerName(ack), "reason", refusal.reason, "fix", refusal.fix)
}

// logV4Nak reports a DHCPNAK with its server and its message (option 56).
func (c *DHCPClient) logV4Nak(nak *dhcpv4.DHCPv4, state string) {
	loggerPtr.Load().Info("iface dhcp v4: server refused the lease (DHCPNAK), restarting configuration",
		"iface", c.ifaceName, "state", state, "server", v4ServerName(nak), "message", nak.Message())
}

// v4ServerName is the server identifier of msg for a log line.
func v4ServerName(msg *dhcpv4.DHCPv4) string {
	if server := msg.ServerIdentifier(); server != nil {
		return server.String()
	}
	return "unidentified (no option 54)"
}

// installV4Lease installs lease on the host: the address with the lease as its
// lifetime, the default route, resolv.conf, then the event. It answers false
// when the address was not installed, so no caller times a lease the host does
// not hold.
func (c *DHCPClient) installV4Lease(w *v4State, lease *v4Lease, topic string) bool {
	logger := loggerPtr.Load()

	ones, _ := lease.ack.SubnetMask().Size()
	if ones == 0 {
		ones = 24
		logger.Warn("iface dhcp v4: no subnet mask in ACK, defaulting to /24",
			"iface", c.ifaceName, "address", lease.ack.YourIPAddr.String())
	}

	var bCidr textbuf.Buffer
	cidr := bCidr.Reset().Str(lease.ack.YourIPAddr.String()).Byte('/').Int(int64(ones)).String()
	lifetime := v4AddressLifetime(lease.duration)

	// Registered before it is installed, so the address reconcile never sees
	// the lease unregistered (iface.RegisterLeasedAddresses).
	if err := c.claimV4Address(cidr); err != nil {
		logger.Warn("iface dhcp v4: lease not installed, the address is not this client's to hold",
			"iface", c.ifaceName, "cidr", cidr, "err", err)
		return false
	}
	// The lifetime is the lease, refreshed on every renewal: the kernel
	// removes the address when the lease runs out unless the client renewed.
	if err := iface.ReplaceAddressWithLifetime(c.ifaceName, cidr, lifetime, lifetime); err != nil {
		logger.Warn("iface dhcp v4: addr replace failed",
			"iface", c.ifaceName, "cidr", cidr, "err", err)
		c.settleV4Address(c.v4Address)
		return false
	}
	if c.v4Address != "" && c.v4Address != cidr {
		// The server granted another address: the old one is no longer leased.
		c.removeV4Address(c.v4Address)
	}
	c.v4Address = cidr
	c.settleV4Address(cidr)

	payload := c.v4Payload(lease.ack, lease.duration)
	c.applyV4Route(&w.route, payload.Router)

	// RFC 2132 Section 3.8: the ACK's servers replace this lease's servers
	// in resolv.conf; an ACK with none withdraws them.
	c.setV4Resolvers(payload.DNSAll)

	c.publishDHCP(topic, payload)

	logger.Info("iface dhcp v4: lease obtained",
		"iface", c.ifaceName, "addr", cidr, "lease", lease.duration, "server", lease.server)
	return true
}

// applyV4Route makes the default route follow gateway, the router the lease
// just granted (RFC 2132 Section 3.5), empty when it granted none. A route
// via another router is removed first.
func (c *DHCPClient) applyV4Route(route *v4Route, gateway string) {
	if route.gateway != "" && route.gateway != gateway {
		c.removeV4Route(route)
	}
	route.gateway = gateway
	if gateway == "" {
		return
	}
	c.installV4Route(route)
}

// installV4Route adds the default route via route.gateway. A failure is
// retried while the lease is held: Warn on the first, Debug on each retry,
// Info once it is installed; the back-off doubles from v4RouteRetryFirst to
// v4RouteRetryMax.
func (c *DHCPClient) installV4Route(route *v4Route) {
	logger := loggerPtr.Load()
	err := iface.AddRoute(c.ifaceName, "0.0.0.0/0", route.gateway, c.config.RouteMetric, rtproto.Iface)
	if err == nil {
		if route.pending {
			logger.Info("iface dhcp v4: default route installed after retry",
				"iface", c.ifaceName, "gw", route.gateway)
		}
		route.installed = true
		route.pending = false
		route.backoff = 0
		return
	}
	if route.pending {
		route.backoff = min(2*route.backoff, v4RouteRetryMax)
		logger.Debug("iface dhcp v4: default route retry failed",
			"iface", c.ifaceName, "gw", route.gateway, "err", err, "retry-in", route.backoff)
	} else {
		route.pending = true
		route.backoff = v4RouteRetryFirst
		logger.Warn("iface dhcp v4: default route install failed, retrying while the lease is held",
			"iface", c.ifaceName, "gw", route.gateway, "err", err, "retry-in", route.backoff)
	}
	route.retryAt = c.clock.Now().Add(route.backoff)
}

// removeV4Route removes the default route this lease installed and forgets
// it, so no retry follows.
func (c *DHCPClient) removeV4Route(route *v4Route) {
	if route.installed {
		if err := iface.RemoveRoute(c.ifaceName, "0.0.0.0/0", route.gateway, c.config.RouteMetric, rtproto.Iface); err != nil {
			loggerPtr.Load().Debug("iface dhcp v4: route removal failed",
				"iface", c.ifaceName, "gw", route.gateway, "err", err)
		}
	}
	*route = v4Route{}
}

// teardownV4 gives up the held lease: the address, then the default route it
// installed, then the registration. The address is removed before it is
// unregistered, so the address reconcile never sees a held address
// unregistered (iface.RegisterLeasedAddresses). It is keyed on what the
// worker holds, never on an old ACK, so a second call removes nothing.
func (c *DHCPClient) teardownV4(w *v4State) {
	if c.v4Address != "" {
		c.removeV4Address(c.v4Address)
		c.v4Address = ""
	}
	c.removeV4Route(&w.route)
	c.settleV4Address("")
	// The lease's DNS servers leave resolv.conf; other sources' stay.
	c.setV4Resolvers(nil)
}

// claimV4Address registers cidr as this client's lease. The address it
// replaces, when it differs, stays registered until it is removed.
func (c *DHCPClient) claimV4Address(cidr string) error {
	addrs := []string{cidr}
	if c.v4Address != "" && c.v4Address != cidr {
		addrs = append(addrs, c.v4Address)
	}
	return c.registerV4Addresses(addrs)
}

// settleV4Address makes the registration name exactly cidr, the address the
// client holds, or nothing when cidr is empty.
func (c *DHCPClient) settleV4Address(cidr string) {
	if cidr == "" {
		iface.UnregisterLeasedAddresses(c.v4Owner)
		return
	}
	if err := c.registerV4Addresses([]string{cidr}); err != nil {
		loggerPtr.Load().Warn("iface dhcp v4: lease registration failed",
			"iface", c.ifaceName, "cidr", cidr, "err", err)
	}
}

// registerV4Addresses records addrs as this client's leases on the kernel
// device the logical interface name resolves to, the key the reconcile uses.
func (c *DHCPClient) registerV4Addresses(addrs []string) error {
	osName, err := iface.ResolveDevice(c.ifaceName)
	if err != nil {
		return err
	}
	return iface.RegisterLeasedAddresses(osName, c.v4Owner, addrs)
}

// removeV4Address removes one leased address from the kernel.
func (c *DHCPClient) removeV4Address(cidr string) {
	if err := iface.RemoveAddress(c.ifaceName, cidr); err != nil {
		loggerPtr.Load().Debug("iface dhcp v4: addr removal failed",
			"iface", c.ifaceName, "cidr", cidr, "err", err)
	}
}

// v4LeaseInfinite is the lease time RFC 2131 reserves for a lease that never
// runs out, as v4LeaseTime decodes it.
//
// RFC 2131 Section 3.3: "The time value of 0xffffffff is reserved to
// represent "infinity"."
const v4LeaseInfinite = 0xffffffff * time.Second

// kernelLifetimeInfinite is the address lifetime the kernel holds as forever
// (INFINITY_LIFE_TIME): an address installed with it is permanent.
const kernelLifetimeInfinite = 0xffffffff

// v4AddressLifetime is the valid and preferred lifetime, in seconds, the
// leased address is installed with: the lease itself.
//
// RFC 2131 Section 3.3: "A client acquires a lease for a network address for a
// fixed period of time (which may be infinite)."
func v4AddressLifetime(leaseTime time.Duration) int {
	if leaseTime >= v4LeaseInfinite {
		return kernelLifetimeInfinite
	}
	return int(leaseTime / time.Second)
}

// v4Payload is the event payload for the lease ack granted for leaseTime.
func (c *DHCPClient) v4Payload(ack *dhcpv4.DHCPv4, leaseTime time.Duration) iface.DHCPPayload {
	ip := ack.YourIPAddr
	mask := ack.SubnetMask()
	ones, _ := mask.Size()
	if ones == 0 {
		ones = 24
	}

	var router string
	routerIP := dhcpv4.GetIP(dhcpv4.OptionRouter, ack.Options)
	if routerIP != nil {
		router = routerIP.String()
	}

	var dns string
	dnsServers := dhcpv4.GetIPs(dhcpv4.OptionDomainNameServer, ack.Options)
	if len(dnsServers) > 0 {
		dns = dnsServers[0].String()
	}

	dnsAll := make([]string, 0, len(dnsServers))
	for _, s := range dnsServers {
		dnsAll = append(dnsAll, s.String())
	}

	ntpIPs := dhcpv4.GetIPs(dhcpv4.OptionNTPServers, ack.Options)
	ntpServers := make([]string, 0, len(ntpIPs))
	for _, s := range ntpIPs {
		ntpServers = append(ntpServers, s.String())
	}

	return iface.DHCPPayload{
		Name:         c.ifaceName,
		Unit:         c.unit,
		Address:      ip.String(),
		PrefixLength: ones,
		Router:       router,
		DNS:          dns,
		DNSAll:       dnsAll,
		NTPServers:   ntpServers,
		LeaseTime:    int(leaseTime / time.Second),
	}
}

// flagsBroadcast is bit 0, the most significant, of the flags field: the only
// bit RFC 2131 Section 2 defines.
const flagsBroadcast = 0x8000

// clearReservedFlags zeroes flags bits 1 to 15 of a message the client sends.
// The vendored dhcpv4.NewRequestFromOffer and NewRenewFromAck fill the flags
// word from the server's OFFER or ACK (WithReply), so a server that sets a
// reserved bit would otherwise have it sent back. Every builder applies the
// caller's modifiers after its own, so this runs last.
func clearReservedFlags(d *dhcpv4.DHCPv4) {
	// RFC 2131 Section 2: "The remaining bits of the flags field are reserved
	// for future use. They MUST be set to zero by clients and ignored by
	// servers and relay agents."
	d.Flags &= flagsBroadcast
}

// v4RequestModifiers builds dhcpv4 packet modifiers from the client config.
// Adds hostname (option 12) and client-id (option 61) when configured, and
// always clears the reserved flags bits. Every DHCPREQUEST the client sends,
// acquiring or extending, carries them.
// RFC 2132 Section 3.14 (hostname), Section 9.14 (client-id).
func (c *DHCPClient) v4RequestModifiers() []dhcpv4.Modifier {
	// RFC 2131 Section 2
	mods := []dhcpv4.Modifier{clearReservedFlags}
	if c.config.Hostname != "" {
		mods = append(mods, dhcpv4.WithOption(dhcpv4.OptHostName(c.config.Hostname)))
	}
	if c.config.ClientID != "" {
		mods = append(mods, dhcpv4.WithOption(dhcpv4.OptClientIdentifier([]byte(c.config.ClientID))))
	}
	return mods
}

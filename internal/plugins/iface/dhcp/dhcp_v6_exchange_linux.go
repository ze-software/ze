// Design: docs/features/interfaces.md -- DHCPv6 client lifecycle
// Overview: dhcp_v6_linux.go -- DHCPv6 lease state machine
// Related: dhcp_v6_lease_linux.go -- the lease record the builders read

//go:build linux

package ifacedhcp

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/dhcpv6/nclient6"
	"github.com/insomniacslk/dhcp/iana"

	"github.com/ze-software/ze/internal/core/clock"
)

// RFC 9915 Section 7.6, Table 1: the transmission and retransmission
// parameters of the client exchanges Ze performs.
const (
	v6SolMaxDelay = 1 * time.Second
	v6SolTimeout  = 1 * time.Second
	v6SolMaxRT    = 3600 * time.Second
	v6ReqTimeout  = 1 * time.Second
	v6ReqMaxRT    = 30 * time.Second
	v6ReqMaxRC    = 10
	v6RenTimeout  = 10 * time.Second
	v6RenMaxRT    = 600 * time.Second
	v6RebTimeout  = 10 * time.Second
	v6RebMaxRT    = 600 * time.Second
	v6RelTimeout  = 1 * time.Second
	v6RelMaxRC    = 4
)

const (
	// v6SolMaxRTFloor and v6SolMaxRTCeiling bound a SOL_MAX_RT option value.
	v6SolMaxRTFloor   = 60 * time.Second
	v6SolMaxRTCeiling = 86400 * time.Second

	// v6RateMessages in v6RateWindow is the transmission limit of one
	// interface: the default RFC 9915 Section 14.1 suggests. It is fixed, and
	// the "SHOULD be configurable" is recorded as a gap (owner decision Q6).
	v6RateMessages = 20
	v6RateWindow   = 20 * time.Second

	// v6AdvertiseMax bounds the Advertise messages one Solicit exchange
	// keeps: only the best is retained, so the bound is on what is examined.
	v6AdvertiseMax = 16

	// v6ReleaseBound caps the whole Release exchange, so Stop returns within
	// it even when no server answers (owner decision Q5).
	v6ReleaseBound = 3 * time.Second

	// v6ListenCeiling is the library's own per-transmission timeout. Ze ends
	// every listen window itself by canceling the context on its clock, so
	// this only has to exceed the longest window: SOL_MAX_RT ceiling plus RAND.
	v6ListenCeiling = 48 * time.Hour

	// v6ElapsedMax is the largest Elapsed Time option value, in hundredths of
	// a second.
	v6ElapsedMax = 0xffff
)

// errV6NoAnswer is what a link answers when the listen window closed with no
// message the matcher accepted.
var errV6NoAnswer = errors.New("no acceptable DHCPv6 answer in the listen window")

// v6Link is one open DHCPv6 client socket on one kernel device. exchange
// transmits msg once to All_DHCP_Relay_Agents_and_Servers and returns the
// first answer accept admits, or errV6NoAnswer when window, measured on clk,
// or ctx ends first. accept MUST be called on the goroutine that called
// exchange. close MUST be called once the exchange that opened the link is
// over.
type v6Link interface {
	exchange(ctx context.Context, clk clock.Clock, window time.Duration, msg *dhcpv6.Message, accept nclient6.Matcher) (*dhcpv6.Message, error)
	close() error
}

// v6Dialer opens a link on the kernel device and answers the device's
// link-layer address.
type v6Dialer func(device string) (v6Link, net.HardwareAddr, error)

// nclient6Link is the production link: the vendored client's socket, codec
// and transaction-id demultiplexing, one transmission per exchange call.
type nclient6Link struct {
	client *nclient6.Client
}

// dialV6Link opens a client on device, which MUST be the kernel device name:
// the library binds UDP 546 on that device's link-local address.
func dialV6Link(device string) (v6Link, net.HardwareAddr, error) {
	ifc, err := net.InterfaceByName(device)
	if err != nil {
		return nil, nil, fmt.Errorf("device %s: %w", device, err)
	}
	client, err := nclient6.New(device, nclient6.WithRetry(1), nclient6.WithTimeout(v6ListenCeiling))
	if err != nil {
		return nil, nil, fmt.Errorf("socket on %s: %w", device, err)
	}
	return nclient6Link{client: client}, ifc.HardwareAddr, nil
}

func (l nclient6Link) exchange(ctx context.Context, clk clock.Clock, window time.Duration, msg *dhcpv6.Message, accept nclient6.Matcher) (*dhcpv6.Message, error) {
	listen, cancel := context.WithCancel(ctx)
	timer := clk.AfterFunc(window, cancel)
	defer timer.Stop()
	defer cancel()

	reply, err := l.client.SendAndRead(listen, nclient6.AllDHCPRelayAgentsAndServers, msg, accept)
	if err == nil {
		return reply, nil
	}
	if listen.Err() != nil {
		return nil, errV6NoAnswer
	}
	if errors.Is(err, nclient6.ErrNoResponse) {
		return nil, errV6NoAnswer
	}
	return nil, err
}

func (l nclient6Link) close() error { return l.client.Close() }

// v6Retransmission is the RFC 9915 Section 15 parameter set of one exchange.
type v6Retransmission struct {
	irt time.Duration
	mrt time.Duration // 0: no ceiling on RT
	mrc int           // 0: no transmission count limit
	mrd time.Duration // 0: no duration limit
	// firstAboveIRT selects RAND strictly above 0 for the first RT (Solicit).
	firstAboveIRT bool
	// solicitMRT makes MRT the worker's SOL_MAX_RT, read at each
	// transmission, so a SOL_MAX_RT option takes effect at once (Solicit).
	solicitMRT bool
}

// v6End is how an exchange ended.
type v6End uint8

const (
	v6EndUnspecified v6End = iota
	// v6EndAnswered: an answer was accepted.
	v6EndAnswered
	// v6EndExhausted: MRC or MRD ended the exchange unanswered.
	v6EndExhausted
	// v6EndCanceled: the context ended (Stop, or the Release bound).
	v6EndCanceled
)

// v6Exchange is one client-initiated exchange, fully described by its
// caller: the message, its parameters, and the hooks that decide answers.
type v6Exchange struct {
	name   string // for log lines: "Solicit", "Renew", ...
	link   v6Link
	msg    *dhcpv6.Message
	params v6Retransmission
	accept nclient6.Matcher
	// prepare, when set, rewrites the message before each transmission (the
	// IAs of a Renew drop a lease whose valid lifetime ran out).
	prepare func(msg *dhcpv6.Message, now time.Time)
	// windowClosed, when set, is called when a listen window ends with no
	// accepted answer; a non-nil result is taken as the answer (Advertise
	// collection).
	windowClosed func() *dhcpv6.Message
}

// v6Outcome is the result of one exchange.
type v6Outcome struct {
	end    v6End
	answer *dhcpv6.Message // set for v6EndAnswered
	at     time.Time       // when the answer was taken, on the client clock
}

// runExchange performs x under RFC 9915 Section 15. It runs on the v6 worker
// goroutine. Every wait reads the client clock and ends when ctx does.
func (w *v6Worker) runExchange(ctx context.Context, x *v6Exchange) v6Outcome {
	logger := loggerPtr.Load()
	var first time.Time
	var rt time.Duration
	// The bound: MRC, MRD, or ctx; an exchange with neither MRC nor MRD runs
	// until answered or canceled, as RFC 9915 Section 15 states.
	for n := 1; ; n++ {
		if !w.admitTransmission(ctx) {
			return v6Outcome{end: v6EndCanceled}
		}
		now := w.clock().Now()
		if n == 1 {
			first = now
		}
		elapsed := now.Sub(first)
		if x.params.mrd > 0 && elapsed >= x.params.mrd {
			return v6Outcome{end: v6EndExhausted}
		}
		if x.prepare != nil {
			x.prepare(x.msg, now)
		}
		// RFC 9915 Section 15: "The client MUST update an "elapsed-time" value
		// within an Elapsed Time option (see Section 21.9) in the retransmitted
		// message."
		x.msg.UpdateOption(dhcpv6.OptElapsedTime(v6ElapsedTime(elapsed)))

		params := x.params
		if params.solicitMRT {
			params.mrt = w.solMaxRT
		}
		if n == 1 {
			rt = v6FirstRT(params, w.random())
		} else {
			rt = v6NextRT(rt, params.mrt, w.randomSigned())
		}
		window := rt
		if x.params.mrd > 0 {
			window = min(window, x.params.mrd-elapsed)
		}

		answer, err := w.transmitOnce(ctx, x, window)
		if err == nil {
			w.outageOver()
			return v6Outcome{end: v6EndAnswered, answer: answer, at: w.clock().Now()}
		}
		if ctx.Err() != nil {
			return v6Outcome{end: v6EndCanceled}
		}
		if x.windowClosed != nil {
			if pick := x.windowClosed(); pick != nil {
				w.outageOver()
				return v6Outcome{end: v6EndAnswered, answer: pick, at: w.clock().Now()}
			}
		}
		if !errors.Is(err, errV6NoAnswer) {
			logger.Debug("iface dhcp v6: transmission failed",
				"iface", w.c.ifaceName, "exchange", x.name, "err", err)
		}
		if x.params.mrc > 0 && n >= x.params.mrc {
			return v6Outcome{end: v6EndExhausted}
		}
		w.reportUnanswered(x.name, n, rt)
	}
}

// transmitOnce sends the message once and listens for window. A link error
// still waits the window out, so a failing socket cannot turn into a storm.
func (w *v6Worker) transmitOnce(ctx context.Context, x *v6Exchange, window time.Duration) (*dhcpv6.Message, error) {
	closes := w.clock().Now().Add(window)
	answer, err := x.link.exchange(ctx, w.clock(), window, x.msg, x.accept)
	if err == nil {
		return answer, nil
	}
	if !errors.Is(err, errV6NoAnswer) {
		w.waitUntil(ctx, closes)
	}
	return nil, err
}

// v6FirstRT is the first retransmission timeout of an exchange.
//
// RFC 9915 Section 15: "RT for the first message transmission is based on
// IRT: RT = IRT + RAND*IRT".
// RFC 9915 Section 18.2.1: "the first RT MUST be selected to be strictly
// greater than IRT by choosing RAND to be strictly greater than 0."
// unit is uniform in [0, 1).
func v6FirstRT(p v6Retransmission, unit float64) time.Duration {
	rnd := (unit*2 - 1) * 0.1
	if p.firstAboveIRT {
		rnd = (1 - unit) * 0.1
	}
	return v6CapRT(p.irt+v6Scale(p.irt, rnd), p.mrt, rnd)
}

// v6NextRT is the retransmission timeout after prev.
//
// RFC 9915 Section 15: "RT for each subsequent message transmission is based
// on the previous value of RT: RT = 2*RTprev + RAND*RTprev".
// rnd is uniform in [-0.1, +0.1].
func v6NextRT(prev, mrt time.Duration, rnd float64) time.Duration {
	return v6CapRT(2*prev+v6Scale(prev, rnd), mrt, rnd)
}

// v6CapRT applies MRT.
//
// RFC 9915 Section 15: "if (RT > MRT) RT = MRT + RAND*MRT".
func v6CapRT(rt, mrt time.Duration, rnd float64) time.Duration {
	if mrt > 0 && rt > mrt {
		return mrt + v6Scale(mrt, rnd)
	}
	return rt
}

func v6Scale(d time.Duration, factor float64) time.Duration {
	return time.Duration(factor * float64(d))
}

// v6ElapsedTime is the Elapsed Time option value for elapsed, saturated.
//
// RFC 9915 Section 21.9 defines the field as hundredths of a second, with
// 0xffff representing any value of 0xffff or more.
func v6ElapsedTime(elapsed time.Duration) time.Duration {
	hundredths := elapsed / (10 * time.Millisecond) // rounds toward zero
	if hundredths > v6ElapsedMax {
		hundredths = v6ElapsedMax
	}
	return hundredths * 10 * time.Millisecond
}

// v6RateLimiter holds the instants of the last v6RateMessages transmissions
// of one worker. One DHCPClient serves one unit, which is one kernel device,
// so a limiter per worker is a limiter per interface. Not safe for concurrent
// use: the v6 worker goroutine owns it.
type v6RateLimiter struct {
	sent [v6RateMessages]time.Time
	next int // index of the oldest entry, overwritten next
	used int // entries recorded, at most v6RateMessages
}

// RFC 9915 Section 14.1: "A DHCPv6 client MUST limit the rate of DHCP
// messages it transmits or retransmits." and "For a device that has multiple
// interfaces, the limit MUST be enforced on a per-interface basis."
//
// admitTransmission waits until one more transmission fits the window, then
// records it. It answers false when ctx ended first.
func (w *v6Worker) admitTransmission(ctx context.Context) bool {
	l := &w.limiter
	if l.used == v6RateMessages {
		free := l.sent[l.next].Add(v6RateWindow)
		if !w.waitUntil(ctx, free) {
			return false
		}
	}
	if ctx.Err() != nil {
		return false
	}
	l.sent[l.next] = w.clock().Now()
	l.next = (l.next + 1) % v6RateMessages
	l.used = min(l.used+1, v6RateMessages)
	return true
}

// waitUntil waits until deadline on the client clock; false when ctx ended.
func (w *v6Worker) waitUntil(ctx context.Context, deadline time.Time) bool {
	d := deadline.Sub(w.clock().Now())
	if d <= 0 {
		return ctx.Err() == nil
	}
	woken := make(chan struct{})
	timer := w.clock().AfterFunc(d, func() { close(woken) })
	defer timer.Stop()
	select {
	case <-woken:
		return true
	case <-ctx.Done():
		return false
	}
}

// --- answer validation --------------------------------------------------

// v6Identified reports whether m names a server and this client.
//
// RFC 9915 Section 16.10: "Clients MUST discard any received Reply message
// that meets any of the following conditions: - the message does not include
// a Server Identifier option" and "the Reply message MUST include a Client
// Identifier option, and the contents of the Client Identifier option MUST
// match the DUID of the client."
// RFC 9915 Section 16.3 states the same two conditions for Advertise. The
// transaction-id condition is the library's: it delivers only messages whose
// xid is pending.
func v6Identified(m *dhcpv6.Message, duid dhcpv6.DUID) bool {
	if m.Options.ServerID() == nil {
		return false
	}
	client := m.Options.ClientID()
	if client == nil {
		return false
	}
	return client.Equal(duid)
}

// v6Status is the top-level status code of m; Success when absent.
func v6Status(m *dhcpv6.Message) iana.StatusCode {
	if s := m.Options.Status(); s != nil {
		return s.StatusCode
	}
	return iana.StatusSuccess
}

// v6Preference is the Preference option value of an Advertise.
//
// RFC 9915 Section 18.2.1: "Any valid Advertise that does not include a
// Preference option is considered to have a preference value of 0." The
// option is one octet; any other length reads as absent.
func v6Preference(m *dhcpv6.Message) int {
	g, ok := m.Options.GetOne(dhcpv6.OptionPreference).(*dhcpv6.OptionGeneric)
	if !ok {
		return 0
	}
	if len(g.OptionData) != 1 {
		return 0
	}
	return int(g.OptionData[0])
}

// v6SolMaxRTOption is the SOL_MAX_RT option of m, and false when m has none
// or carries a value out of range.
//
// RFC 9915 Section 21.24: "A DHCP client MUST ignore any SOL_MAX_RT option
// values that are less than 60 or more than 86400."
func v6SolMaxRTOption(m *dhcpv6.Message) (time.Duration, bool) {
	g, ok := m.Options.GetOne(dhcpv6.OptionSolMaxRT).(*dhcpv6.OptionGeneric)
	if !ok {
		return 0, false
	}
	if len(g.OptionData) != 4 {
		return 0, false
	}
	v := time.Duration(binary.BigEndian.Uint32(g.OptionData)) * time.Second
	if v < v6SolMaxRTFloor {
		return 0, false
	}
	if v > v6SolMaxRTCeiling {
		return 0, false
	}
	return v, true
}

// noteSolMaxRT applies a valid SOL_MAX_RT option of any identified answer,
// a failure Reply included.
//
// RFC 9915 Section 21.24: "If a DHCP client receives a message containing a
// SOL_MAX_RT option that has a valid value for SOL_MAX_RT, the client MUST set
// its internal SOL_MAX_RT parameter to the value contained in the SOL_MAX_RT
// option."
func (w *v6Worker) noteSolMaxRT(m *dhcpv6.Message) {
	if v, ok := v6SolMaxRTOption(m); ok {
		w.solMaxRT = v
	}
}

// v6HasLeases reports whether m carries at least one IA Address or IA Prefix.
func v6HasLeases(m *dhcpv6.Message) bool {
	for _, na := range m.Options.IANA() {
		if len(na.Options.Addresses()) > 0 {
			return true
		}
	}
	for _, pd := range m.Options.IAPD() {
		if len(pd.Options.Prefixes()) > 0 {
			return true
		}
	}
	return false
}

// replyMatcher accepts an identified Reply with a usable status. A Reply
// with a failure status (UnspecFail, UseMulticast, ...) is not an answer: the
// exchange retransmits on its own schedule, which is the rate-limited retry
// RFC 9915 Section 18.2.10 asks for. Every other message type, an unknown one
// included, is discarded.
func (w *v6Worker) replyMatcher() nclient6.Matcher {
	return func(m *dhcpv6.Message) bool {
		if m.MessageType != dhcpv6.MessageTypeReply {
			return false
		}
		if !v6Identified(m, w.duid) {
			return false
		}
		w.noteSolMaxRT(m)
		return v6Status(m) == iana.StatusSuccess
	}
}

// v6AdvertiseCollector implements the Solicit answer rules.
//
// RFC 9915 Section 18.2.1: "A client MUST collect valid Advertise messages
// for the first RT seconds, unless it receives a valid Advertise message with
// a preference value of 255."
type v6AdvertiseCollector struct {
	w         *v6Worker
	best      *dhcpv6.Message
	bestPref  int
	seen      int
	firstOver bool
}

func (a *v6AdvertiseCollector) accept(m *dhcpv6.Message) bool {
	if !v6Identified(m, a.w.duid) {
		return false
	}
	//exhaustive:ignore // a Solicit is answered by Advertise or Reply only; every other type is discarded
	switch m.MessageType {
	case dhcpv6.MessageTypeReply:
		a.w.noteSolMaxRT(m)
		// RFC 9915 Section 18.2.1: a Reply to a Rapid Commit Solicit is used
		// only when it carries the Rapid Commit option.
		if m.Options.GetOne(dhcpv6.OptionRapidCommit) == nil {
			return false
		}
		return v6Status(m) == iana.StatusSuccess
	case dhcpv6.MessageTypeAdvertise:
		a.w.noteSolMaxRT(m)
		// RFC 9915 Section 18.2.9: "The client MUST ignore any Advertise
		// message that contains no addresses (IA Address options (see
		// Section 21.6) encapsulated in IA_NA options (see Section 21.4)) and
		// no delegated prefixes".
		if !v6HasLeases(m) {
			return false
		}
		if a.firstOver {
			return true
		}
		pref := v6Preference(m)
		if pref == 255 {
			return true
		}
		if a.seen >= v6AdvertiseMax {
			return false
		}
		a.seen++
		if a.best == nil || pref > a.bestPref {
			a.best, a.bestPref = m, pref
		}
		return false
	}
	return false
}

// windowClosed ends the first-RT collection and answers the best Advertise.
//
// RFC 9915 Section 18.2.1: "If the first RT elapses and the client has
// received a valid Advertise message, the client SHOULD continue with a
// client-initiated message exchange by sending a Request message."
func (a *v6AdvertiseCollector) windowClosed() *dhcpv6.Message {
	a.firstOver = true
	return a.best
}

// --- message builders -----------------------------------------------------

// newV6Message starts a client message: a fresh transaction id, the Client
// Identifier and a zero Elapsed Time.
//
// RFC 9915 Section 16.1: the transaction ID is chosen anew for each exchange
// and kept unchanged in its retransmissions; runExchange never touches it.
func (w *v6Worker) newV6Message(kind dhcpv6.MessageType) (*dhcpv6.Message, error) {
	xid, err := dhcpv6.GenerateTransactionID()
	if err != nil {
		return nil, fmt.Errorf("transaction id: %w", err)
	}
	m := &dhcpv6.Message{MessageType: kind, TransactionID: xid}
	m.AddOption(dhcpv6.OptClientID(w.duid))
	m.AddOption(dhcpv6.OptElapsedTime(0))
	return m, nil
}

// requestOptions is the Option Request option of every message but Release.
//
// RFC 9915 Section 21.24: "A DHCP client MUST include the SOL_MAX_RT option
// code in any Option Request option (see Section 21.7) it sends in a Solicit
// message."
func requestOptions() dhcpv6.Option {
	return dhcpv6.OptRequestedOption(dhcpv6.OptionDNSRecursiveNameServer, dhcpv6.OptionSolMaxRT)
}

// buildSolicit is the Solicit: Rapid Commit, an empty IA_NA, and the IA_PD
// when prefix delegation is configured.
//
// RFC 9915 Section 18.2.1: "The client uses IA_NA options (see Section 21.4)
// to request the assignment of non-temporary addresses ... and IA_PD options
// (see Section 21.21) to request prefix delegation."
func (w *v6Worker) buildSolicit() (*dhcpv6.Message, error) {
	m, err := w.newV6Message(dhcpv6.MessageTypeSolicit)
	if err != nil {
		return nil, err
	}
	m.AddOption(requestOptions())
	m.AddOption(&dhcpv6.OptionGeneric{OptionCode: dhcpv6.OptionRapidCommit})
	m.AddOption(&dhcpv6.OptIANA{IaId: w.iaidNA})
	if pd := w.emptyIAPD(); pd != nil {
		m.AddOption(pd)
	}
	return m, nil
}

// buildRequest answers the chosen Advertise: its Server Identifier and the
// IAs it offered for this client's IAIDs, an empty IA for any it left out.
func (w *v6Worker) buildRequest(advertise *dhcpv6.Message) (*dhcpv6.Message, error) {
	m, err := w.newV6Message(dhcpv6.MessageTypeRequest)
	if err != nil {
		return nil, err
	}
	m.AddOption(dhcpv6.OptServerID(advertise.Options.ServerID()))
	m.AddOption(requestOptions())
	na := &dhcpv6.OptIANA{IaId: w.iaidNA}
	for _, offered := range advertise.Options.IANA() {
		if offered.IaId == w.iaidNA {
			na = &dhcpv6.OptIANA{IaId: w.iaidNA, Options: offered.Options}
		}
	}
	m.AddOption(na)
	if pd := w.emptyIAPD(); pd != nil {
		for _, offered := range advertise.Options.IAPD() {
			if offered.IaId == w.iaidPD {
				pd = &dhcpv6.OptIAPD{IaId: w.iaidPD, Options: offered.Options}
			}
		}
		m.AddOption(pd)
	}
	return m, nil
}

// buildFromLease is a Request (NoBinding recovery), Renew, Rebind or Release
// built from the lease record. Rebind omits the Server Identifier; Release
// carries no Option Request option.
//
// RFC 9915 Section 18.2.5: "The client constructs the Rebind message as
// described in Section 18.2.4, with the following differences: - The client
// sets the "msg-type" field to REBIND. - The client does not include the
// Server Identifier option (see Section 21.3) in the Rebind message."
func (w *v6Worker) buildFromLease(kind dhcpv6.MessageType, lease *v6Lease, now time.Time) (*dhcpv6.Message, error) {
	m, err := w.newV6Message(kind)
	if err != nil {
		return nil, err
	}
	if kind != dhcpv6.MessageTypeRebind {
		m.AddOption(dhcpv6.OptServerID(lease.server))
	}
	if kind != dhcpv6.MessageTypeRelease {
		m.AddOption(requestOptions())
	}
	w.setLeaseIAs(m, kind, lease, now)
	return m, nil
}

// setLeaseIAs (re)writes the IA options of m from lease at now.
//
// RFC 9915 Section 18.2.4: "The client MUST NOT include addresses and
// prefixes in any IA option that the client did not obtain from the server or
// that are no longer valid (that have a valid lifetime of 0)." The leases are
// the recorded ones whose valid lifetime has not run out; an IA_PD with no
// leased prefix still goes out with the configured hint (AC-25), except in a
// Release, which carries only what the client holds.
func (w *v6Worker) setLeaseIAs(m *dhcpv6.Message, kind dhcpv6.MessageType, lease *v6Lease, now time.Time) {
	m.Options.Del(dhcpv6.OptionIANA)
	m.Options.Del(dhcpv6.OptionIAPD)

	na := &dhcpv6.OptIANA{IaId: w.iaidNA}
	for _, b := range lease.live(v6KindAddress, now) {
		na.Options.Add(&dhcpv6.OptIAAddress{
			IPv6Addr:          net.IP(b.prefix.Addr().AsSlice()),
			PreferredLifetime: b.preferred,
			ValidLifetime:     b.valid,
		})
	}
	if kind != dhcpv6.MessageTypeRelease || len(na.Options.Options) > 0 {
		m.AddOption(na)
	}

	prefixes := lease.live(v6KindPrefix, now)
	if kind == dhcpv6.MessageTypeRelease {
		if len(prefixes) == 0 {
			return
		}
		pd := &dhcpv6.OptIAPD{IaId: w.iaidPD}
		for _, b := range prefixes {
			pd.Options.Add(v6PrefixOption(b))
		}
		m.AddOption(pd)
		return
	}
	pd := w.emptyIAPD()
	if pd == nil {
		pd = &dhcpv6.OptIAPD{IaId: w.iaidPD}
		if len(prefixes) == 0 {
			return
		}
	}
	if len(prefixes) > 0 {
		pd.Options = dhcpv6.PDOptions{}
		for _, b := range prefixes {
			pd.Options.Add(v6PrefixOption(b))
		}
	}
	m.AddOption(pd)
}

func v6PrefixOption(b v6Binding) *dhcpv6.OptIAPrefix {
	return &dhcpv6.OptIAPrefix{
		PreferredLifetime: b.preferred,
		ValidLifetime:     b.valid,
		Prefix: &net.IPNet{
			IP:   net.IP(b.prefix.Addr().AsSlice()),
			Mask: net.CIDRMask(b.prefix.Bits(), 128),
		},
	}
}

// emptyIAPD is the IA_PD a message carries when prefix delegation is
// configured and no prefix is leased: the hint of the configured length, or
// no hint for `pd {}` with no length. nil when `pd` is not configured.
//
// RFC 9915 Section 21.22: "A client MAY send a non-zero value in the
// "prefix-length" field and the unspecified value (::) in the "IPv6-prefix"
// field to indicate a preference for the size of the prefix to be delegated."
func (w *v6Worker) emptyIAPD() *dhcpv6.OptIAPD {
	if !w.c.config.PDRequested {
		return nil
	}
	pd := &dhcpv6.OptIAPD{IaId: w.iaidPD}
	if w.c.config.PDLength > 0 {
		pd.Options.Add(&dhcpv6.OptIAPrefix{
			Prefix: &net.IPNet{IP: net.IPv6unspecified, Mask: net.CIDRMask(w.c.config.PDLength, 128)},
		})
	}
	return pd
}

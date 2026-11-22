// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS accounting
// RFC: rfc/short/rfc2866.md -- Accounting-Request contents (Sections 4.1, 5),
// Acct-Terminate-Cause (Section 5.10)
// RFC: rfc/short/rfc2865.md -- Framed-IP-Address (Section 5.8),
// Calling-Station-Id (Section 5.31)
// RFC: rfc/short/rfc2869.md -- NAS-Port-Id (Section 5.17), Gigawords (Section 5.1),
// interim interval precedence (Section 2.1), Event-Timestamp (Section 5.3)
// Related: handler.go -- RADIUS auth handler shares the client
// Related: nasportid.go -- NAS-Port-Id template resolution

package l2tpauthradius

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/subscriberid"
	"github.com/ze-software/ze/internal/core/taskguard"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/ze"
)

// acctCaptureSession binds one accounting task to its session's original
// network. Every counter read through the capture reaches the PPP unit created
// for that network, never an interface name, so a renamed pppN or a name
// another subscriber now holds cannot answer for this session. The reads are
// raw totals, untouched by an operator's display-counter clear. A test
// replaces it.
var acctCaptureSession = func(selector l2tp.SessionSelector) (l2tp.CapturedSession, error) {
	service := l2tp.LookupService()
	if service == nil {
		return nil, l2tp.ErrSubsystemNotStarted
	}
	return service.CaptureSession(selector)
}

// acctTerminalBound is how long the task of a session whose network ended
// waits for that network's terminal counters. On a peer hang-up L2TP publishes
// the session-down before PPP has disconnected and measured the original unit,
// so the measurement normally arrives after the Down. The bound is the one a
// record waits for its acknowledgement (acctRecordBound), so a Stop never waits
// longer for its data than for its own exchange. A test replaces it.
var acctTerminalBound = acctRecordBound

// acctCaptureRef holds a bound capture, so one atomic pointer publishes it
// from the accounting task to a retirement that ends the session.
type acctCaptureRef struct{ session l2tp.CapturedSession }

// acctNow reads the clock that stamps Event-Timestamp. A test replaces it so
// the encoded seconds are exact rather than a window around the real clock.
var acctNow = time.Now

// acctBootRandom fills the random half of the boot tag. A test replaces it so
// the tag is exact rather than unknown.
var acctBootRandom = func(random []byte) {
	if _, err := rand.Read(random); err != nil {
		// Since Go 1.24 crypto/rand.Read crashes the program rather than return
		// an error, so only a broken toolchain reaches here.
		panic("BUG: crypto/rand.Read returned an error: " + err.Error())
	}
}

// acctSession tracks per-session accounting state.
//
// nasPortID is resolved once, when the session starts accounting, and every
// record of that session then repeats it. Re-resolving per packet would let a
// config reload move the text mid-session, and the text is what a billing
// system joins the Start, Interim and Stop records by. It is stored for the
// same reason acctSessID is.
//
// endpoint is the accounting configuration current when the session started.
// Every record of the session leaves through it, so a reload that moves
// accounting to another server or NAS identity leaves the Start, the Interims
// and the Stop on the one server that holds the session's record.
type acctSession struct {
	birth     subscriberid.L2TPBirth
	tunnelID  uint16
	sessionID uint16
	username  string
	peerAddr  string // Updated and read under radiusAcct.mu.
	endpoint  *acctEndpoint
	// terminateCause, finalDuration and the final counters describe the end
	// of service. end or endLive writes the first three, and the caller that
	// claims the session for teardown is the only caller of either:
	// onSessionDown and retirement claim the session out of
	// radiusAcct.sessions under radiusAcct.mu, and exactly one can. endLive
	// also writes the final counters and sets finalSettled. Closing ended
	// publishes them to the accounting task, which reads them only after ended
	// is closed, and which writes the final counters itself when finalSettled
	// is false, from the network's terminal measurement. The interim loop
	// never reads them: buildAcctPacket reads them inside the Acct-Status-Type
	// Stop branch alone.
	terminateCause     l2tpevents.TerminateCause
	finalDuration      uint32
	finalAt            time.Time
	finalAtValid       bool
	finalCounters      acctCounters
	finalCountersValid bool
	finalSettled       bool
	ended              chan struct{}
	// finished closes when the accounting task has sent the Stop, or given up
	// on it, and released its endpoint.
	finished chan struct{}
	// callingStationID is the L2TP Calling Number AVP of this session, which
	// every record reports as Calling-Station-Id (RFC 2865 Section 5.31). It
	// is stored for the same reason nasPortID is: the value is a property of
	// the call, so every record of the session repeats the one it started
	// with. Empty when neither side named a calling number.
	callingStationID string
	acctSessID       string
	nasPortID        string
	startTime        time.Time
	cancel           context.CancelFunc
	// network is the network lifetime this accounting session measures, named
	// by the session-ip-assigned that started it.
	network l2tpevents.NetworkLifetime
	// capture is the session's network-qualified capture. The accounting task
	// binds it before the Start and retains it until the task finishes; the
	// capture owns no resource, so dropping the reference releases it. Nil
	// until bound, and for good when binding failed: every counter read then
	// answers unavailable.
	capture atomic.Pointer[acctCaptureRef]
	// counterBase is the original unit's reading PPP took before it published
	// the assignment, carried in the payload. countersInvalid is set when that
	// baseline was unavailable or a later reading fell below it.
	counterBase     acctCounters
	countersInvalid atomic.Bool
}

type acctCounters struct {
	rxBytes   uint64
	txBytes   uint64
	rxPackets uint64
	txPackets uint64
}

// acctTotals converts one original-unit measurement the events carry.
func acctTotals(totals l2tpevents.TrafficTotals) acctCounters {
	return acctCounters{
		rxBytes: totals.RxBytes, txBytes: totals.TxBytes,
		rxPackets: totals.RxPackets, txPackets: totals.TxPackets,
	}
}

// countersBelow reports whether any field of current is below base.
func countersBelow(current, base acctCounters) bool {
	if current.rxBytes < base.rxBytes {
		return true
	}
	if current.txBytes < base.txBytes {
		return true
	}
	if current.rxPackets < base.rxPackets {
		return true
	}
	return current.txPackets < base.txPackets
}

// sinceBaseline measures this service rather than the retained PPP unit: one
// reading of the original unit minus the baseline the network lifetime
// started from. RFC 2866 Section 5.3: "This attribute indicates how many
// octets have been received from the port over the course of this service
// being provided, and can only be present in Accounting-Request records where
// the Acct-Status-Type is set to Stop." RFC 2869 Section 2.1 extends these
// counters to Interim-Update records.
//
// A reading below the baseline is not this lifetime's continuation. It marks
// the session's counters invalid for good rather than wrap the unsigned
// subtraction into a huge bill.
func (s *acctSession) sinceBaseline(current acctCounters) (acctCounters, bool) {
	base := s.counterBase
	if countersBelow(current, base) {
		s.countersInvalid.Store(true)
		recordCountersOmitted(s, "a counter reading of the PPP unit fell below the session's baseline", nil)
		return acctCounters{}, false
	}
	return acctCounters{
		rxBytes: current.rxBytes - base.rxBytes, txBytes: current.txBytes - base.txBytes,
		rxPackets: current.rxPackets - base.rxPackets, txPackets: current.txPackets - base.txPackets,
	}, true
}

// boundCapture answers the session's capture, or records why counters are
// unavailable and answers nil.
func (s *acctSession) boundCapture() l2tp.CapturedSession {
	ref := s.capture.Load()
	if ref == nil {
		recordCountersOmitted(s, "the session's original network is not bound to its PPP unit", nil)
		return nil
	}
	return ref.session
}

// liveDelta reads the original PPP unit now, through the bound capture. An
// Interim-Update and a retirement's Stop report it. A failed read answers
// false, so the record omits its traffic counters; it never ends the session.
func (s *acctSession) liveDelta() (acctCounters, bool) {
	if s.countersInvalid.Load() {
		return acctCounters{}, false
	}
	captured := s.boundCapture()
	if captured == nil {
		return acctCounters{}, false
	}
	counters, err := captured.Counters()
	if err != nil {
		recordCountersOmitted(s, "the counter read of the session's PPP unit failed", err)
		return acctCounters{}, false
	}
	return s.sinceBaseline(acctCounters{
		rxBytes: counters.RxBytes, txBytes: counters.TxBytes,
		rxPackets: counters.RxPackets, txPackets: counters.TxPackets,
	})
}

// terminalDelta waits, at most acctTerminalBound, for the terminal measurement
// PPP takes of the ended network before it closes the original unit. A wait
// that ends at the bound, and a measurement PPP could not take, answer false.
func (s *acctSession) terminalDelta() (acctCounters, bool) {
	if s.countersInvalid.Load() {
		return acctCounters{}, false
	}
	captured := s.boundCapture()
	if captured == nil {
		return acctCounters{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), acctTerminalBound)
	defer cancel()
	sample, err := captured.TerminalCounters(ctx)
	if err != nil {
		recordCountersOmitted(s, "PPP's final measurement of the session was not received within "+
			acctTerminalBound.String(), err)
		return acctCounters{}, false
	}
	totals, err := sample.Counters()
	if err != nil {
		recordCountersOmitted(s, "PPP's final measurement of the session is unavailable", err)
		return acctCounters{}, false
	}
	return s.sinceBaseline(acctTotals(totals))
}

// bindCapture binds the session's network-qualified capture. It runs in the
// accounting task, never on the session-ip-assigned callback: CaptureSession
// takes the L2TP subsystem mutex, and Subsystem.Stop holds that mutex while it
// waits for the reactor goroutine that delivers the callback.
func (s *acctSession) bindCapture() {
	if !s.network.IsValid() {
		recordCountersOmitted(s, "the session's address assignment named no network lifetime", nil)
		return
	}
	captured, err := acctCaptureSession(l2tp.SessionSelector{
		SID: s.sessionID, TID: s.tunnelID, Birth: s.birth, NetworkLifetime: s.network,
	})
	if err != nil {
		recordCountersOmitted(s, "the session's original network could not be captured", err)
		return
	}
	s.capture.Store(&acctCaptureRef{session: captured})
}

// end records the end of a session whose network ended and hands the
// Accounting-Stop to the session's accounting task, without waiting for any
// RADIUS server or for PPP. The task then awaits the network's terminal
// counters itself (captureTerminal). The caller MUST have claimed the session
// by removing it from the accountant's map, so end or endLive runs once per
// session.
//
// The hand-over is deferred, so a Ze defect met while capturing the end still
// releases the task to send the Stop with what was captured: the end of
// service is never lost, and retirement never waits on a task that cannot
// move.
func (s *acctSession) end(cause l2tpevents.TerminateCause) {
	defer close(s.ended)
	defer s.cancel()
	s.captureEnd(cause)
}

// endLive ends the accounting of a session whose network continues: the
// accounting configuration retired under it. No terminal measurement will
// come, so the final counters are read from the original unit now, when the
// accounting service ends. Same caller contract as end.
func (s *acctSession) endLive(cause l2tpevents.TerminateCause) {
	defer close(s.ended)
	defer s.cancel()
	s.captureEnd(cause)
	s.captureLive()
}

// captureEnd records what the Stop reports about the end of service. The
// elapsed time and the instant are read here, when service ends, rather than
// when the task sends: the task may still be waiting for the session's Start
// or Interim to be acknowledged. RFC 2866 Section 5.7: "This attribute
// indicates how many seconds the user has received service for, and can only
// be present in Accounting-Request records where the Acct-Status-Type is set
// to Stop."
//
// The instant is read inside the accounting panic boundary, because it reaches
// a seam. A defect there leaves the cause and the elapsed time recorded and
// marks the rest unavailable, so the Stop omits them rather than reporting a
// zero.
func (s *acctSession) captureEnd(cause l2tpevents.TerminateCause) {
	s.terminateCause = cause
	s.finalDuration = uint32(time.Since(s.startTime).Seconds())
	describe := func() taskguard.Context { return sessionUnit(s.tunnelID, s.sessionID) }
	defer accountingTask.Contain(describe, nil)
	// RFC 2869 Section 5.3: "This attribute is included in an
	// Accounting-Request packet to record the time that this event occurred on
	// the NAS, in seconds since January 1, 1970 00:00 UTC."
	s.finalAt, s.finalAtValid = acctNow(), true
}

// captureLive records a live session's final counters, inside the accounting
// panic boundary. A defect leaves them unavailable, so the Stop omits them.
func (s *acctSession) captureLive() {
	s.finalSettled = true
	describe := func() taskguard.Context { return sessionUnit(s.tunnelID, s.sessionID) }
	defer accountingTask.Contain(describe, nil)
	s.finalCounters, s.finalCountersValid = s.liveDelta()
}

// captureTerminal records the final counters of a session whose network
// ended. The accounting task calls it after ended closes, never the Down's
// publication callback: on a peer hang-up L2TP publishes the Down before PPP
// has measured the network, and the capture's TerminalCounters MUST be awaited
// off that callback.
func (s *acctSession) captureTerminal() {
	s.finalCounters, s.finalCountersValid = s.terminalDelta()
}

// eventTime answers the instant one record reports as its Event-Timestamp. A
// Stop reports the end of service captureEnd recorded, which can be earlier
// than the moment the task sends it; the other records report now. It answers
// false when the end of service was not recorded.
func (s *acctSession) eventTime(statusType uint8) (time.Time, bool) {
	if statusType == radius.AcctStatusStop {
		return s.finalAt, s.finalAtValid
	}
	return acctNow(), true
}

// subscriberIPv4 parses a session's assigned address and returns its four
// octets. It reports false for an empty value, an unparseable value, and an
// IPv6 address. RFC 2865 Section 5.8 gives Framed-IP-Address four octets, so
// those cases have no attribute to send. An IPv4-mapped address is unwrapped.
func subscriberIPv4(assigned string) ([]byte, bool) {
	addr, err := netip.ParseAddr(assigned)
	if err != nil {
		return nil, false
	}
	if !addr.Is4() && !addr.Is4In6() {
		return nil, false
	}
	v4 := addr.As4()
	return v4[:], true
}

// splitGigawords splits a uint64 byte count into a uint32 octets value
// and a uint32 gigawords value for RADIUS encoding.
// RFC 2869 Section 5.1: gigawords = total_bytes >> 32.
func splitGigawords(bytes uint64) (octets, gigawords uint32) {
	return uint32(bytes & 0xFFFFFFFF), uint32(bytes >> 32)
}

// acctEndpoint is one installed accounting configuration: the client, the
// NAS identity every record reports, and the server address the metrics name.
// A session holds the endpoint current when it started until its Stop is
// sent. A reload installs a new endpoint and retires the old one, and the last
// session to release a retired endpoint closes its client, so a reload never
// closes a client a session still needs for its Stop.
//
// The fields above mu never change after construction. Safe for concurrent use.
type acctEndpoint struct {
	client        *radius.Client
	nasID         string
	sourceAddress net.IP
	serverAddr    string

	mu       sync.Mutex
	sessions int  // sessions holding this endpoint
	retired  bool // the accountant installed another endpoint or stopped
}

// acquire records one more session sending through e. The session MUST call
// release once its Stop is sent or abandoned.
func (e *acctEndpoint) acquire() {
	e.mu.Lock()
	e.sessions++
	e.mu.Unlock()
}

// release ends one session's use of e. It MUST be paired with acquire.
func (e *acctEndpoint) release() {
	e.mu.Lock()
	e.sessions--
	closeNow := e.retired && e.sessions == 0
	e.mu.Unlock()
	if closeNow {
		e.close()
	}
}

// retire marks e as replaced. Its client closes now if no session holds it,
// or when the last session releases it.
func (e *acctEndpoint) retire() {
	e.mu.Lock()
	e.retired = true
	closeNow := e.sessions == 0
	e.mu.Unlock()
	if closeNow {
		e.close()
	}
}

// close releases the client's socket. Client.Close is idempotent, so a forced
// close at the retirement bound and the last release can both call it.
func (e *acctEndpoint) close() {
	if err := e.client.Close(); err != nil {
		logger().Warn("l2tp-auth-radius: closing a retired accounting client failed",
			"code", codeRADIUSAccountingUnacknowledged,
			"server", e.serverAddr, "error", err)
	}
}

// radiusAcct manages RADIUS accounting lifecycle.
//
// endpoint is the accounting configuration new sessions start under, nil when
// no RADIUS accounting is configured.
//
// sessions holds the records of sessions still delivering service. ending
// holds the sessions whose service ended while their task still sends the
// Stop: onSessionDown moves a record from one to the other, and the task
// removes it when it finishes. Retirement takes both, so shutdown waits for a
// Stop already handed over as well as for the Stops it hands over itself.
//
// interval is the acct-interval config leaf, zero when the operator set none.
// It is not a cadence on its own: acctInterval turns it and the Access-Accept
// into the cadence one session runs at.
type radiusAcct struct {
	mu              sync.Mutex
	sessions        map[sessionKey]*acctSession
	ending          map[*acctSession]struct{}
	endpoint        *acctEndpoint
	accepting       bool
	interval        time.Duration
	nextSess        atomic.Uint32
	bootTagMint     sync.Once
	bootTag         [bootTagOctets]byte
	nasPortIDFormat string
	exclusions      attributeExclusions
}

// sessionKey names one accounting record by the session's whole identity. A
// tunnel reuses its session IDs, so two sessions can hold the same wire labels
// while both are live: the birth keeps their records apart, and a Down for one
// cannot end the other's record.
type sessionKey struct {
	birth     subscriberid.L2TPBirth
	tunnelID  uint16
	sessionID uint16
}

func newRADIUSAcct() *radiusAcct {
	return &radiusAcct{
		sessions: make(map[sessionKey]*acctSession),
		ending:   make(map[*acctSession]struct{}),
	}
}

// setClient installs the RADIUS client and everything that must change with it.
// The NAS-Port-Id format is one of those: applying it separately would leave a
// window where a session authenticates under the new format and accounts under
// the old one, and a billing system joins those two records by that text.
//
// interval is stored as it arrives, zero included. A reload that removes the
// acct-interval leaf gives the Access-Accept the cadence back, for every session
// that starts after it. An "install only a positive value" guard would refuse
// that reload and keep the removed value forever.
//
// The accountant owns c from here: the replaced endpoint is retired, and its
// client closes once the last session that started under it has sent its Stop.
// A nil c installs no endpoint and stops new sessions from accounting.
func (a *radiusAcct) setClient(c *radius.Client, nasID string, interval time.Duration, serverAddr string, sourceAddr net.IP, nasPortIDFormat string) {
	var installed *acctEndpoint
	if c != nil {
		installed = &acctEndpoint{client: c, nasID: nasID, sourceAddress: sourceAddr, serverAddr: serverAddr}
	}
	a.mu.Lock()
	old := a.endpoint
	a.endpoint = installed
	a.accepting = installed != nil
	a.nasPortIDFormat = nasPortIDFormat
	a.interval = interval
	a.mu.Unlock()
	if old != nil {
		old.retire()
	}
}

// pauseAdmissions leaves live accounting untouched until removal commits.
func (a *radiusAcct) pauseAdmissions() {
	a.mu.Lock()
	a.accepting = false
	a.mu.Unlock()
}

// setExclusions installs the attributes this deployment holds back. It is
// separate from setClient because the two answer different questions: which
// server a record goes to, and which attributes the record carries. A reload
// applies to every record built after it, and a record already on the wire
// keeps the list it was built with.
func (a *radiusAcct) setExclusions(exclusions attributeExclusions) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.exclusions = exclusions
}

// exclusionsNow reads the set the last reload installed. buildAcctPacket calls
// this rather than reading the field, because the field is written under a.mu
// and buildAcctPacket runs with the lock released.
func (a *radiusAcct) exclusionsNow() attributeExclusions {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.exclusions
}

// bootTagOctets is the size of the boot tag: four octets of Unix seconds and
// eight random octets, written as 24 hex digits.
const bootTagOctets = 12

// genSessionID answers a new Acct-Session-Id, "tunnel-session-tag-counter".
//
// RFC 2866 Section 5.5: "This attribute is a unique Accounting ID to make it
// easy to match start and stop records in a log file." The counter makes the
// id unique within one accountant; the boot tag makes it unique across a
// restart, which resets the counter. CoA compares the complete value retained
// by currentSessionIDs, including the boot tag and counter. Safe for concurrent use.
func (a *radiusAcct) genSessionID(tunnelID, sessionID uint16) string {
	a.bootTagMint.Do(a.mintBootTag)
	n := a.nextSess.Add(1)
	var b textbuf.Buffer
	return b.Reset().Int(int64(tunnelID)).Byte('-').Int(int64(sessionID)).Byte('-').
		Hex(a.bootTag[:]).Byte('-').Int(int64(n)).String()
}

// currentSessionIDs snapshots the complete ids the active accounting owner
// issued. A snapshot has no entry for an unknown id; a caller MUST NOT guess
// the missing value from a numeric tunnel/session tuple. Safe for concurrent use.
// RFC 2866 Section 5.5: "This attribute is a unique Accounting ID to make it
// easy to match start and stop records in a log file."
func (a *radiusAcct) currentSessionIDs() map[subscriberid.L2TPBirth]string {
	a.mu.Lock()
	defer a.mu.Unlock()
	ids := make(map[subscriberid.L2TPBirth]string, len(a.sessions))
	for _, session := range a.sessions {
		if !session.birth.IsValid() {
			continue
		}
		if session.acctSessID == "" {
			continue
		}
		ids[session.birth] = session.acctSessID
	}
	return ids
}

// mintBootTag writes the boot tag: the Unix seconds acctNow reads, then 64
// random bits. It MUST run once per accountant, through bootTagMint, at the
// first genSessionID and never in newRADIUSAcct: the accountant is built at
// package init (register.go), before the NTP plugin restores the clock, so a
// tag read then would carry the kernel's boot clock. Before NTP answers, a box
// with no RTC can read the same restored second on two boots, and the random
// half is what keeps the two tags apart; the time half orders boots once the
// clock is real.
func (a *radiusAcct) mintBootTag() {
	binary.BigEndian.PutUint32(a.bootTag[:4], uint32(acctNow().Unix())) //nolint:gosec // Unix seconds fit 32 bits until 2106; the tag only has to differ.
	acctBootRandom(a.bootTag[4:])
}

// subscribeEventBus subscribes to session lifecycle events for accounting.
func (a *radiusAcct) subscribeEventBus(bus ze.EventBus) func() {
	if bus == nil {
		return func() {}
	}

	up := l2tpevents.SessionIPAssigned.Subscribe(bus, func(payload *l2tpevents.SessionIPAssignedPayload) {
		a.onSessionIPAssigned(payload)
	})
	released := l2tpevents.SessionIPReleased.Subscribe(bus, func(payload *l2tpevents.SessionIPReleasedPayload) {
		a.onSessionIPReleased(payload)
	})
	down := l2tpevents.SessionDown.Subscribe(bus, func(payload *l2tpevents.SessionDownPayload) {
		a.onSessionDown(payload)
	})
	return func() {
		up()
		released()
		down()
	}
}

// onSessionIPReleased stops reporting the IPv4 address of a session whose
// IPCP left Opened while the session stays up, so the next Interim-Update
// carries no Framed-IP-Address until IPCP opens again and onSessionIPAssigned
// sets it. An IPv6 release changes nothing here: Framed-IP-Address is IPv4.
// The session's accounting continues: a family release is not the end of
// service.
func (a *radiusAcct) onSessionIPReleased(payload *l2tpevents.SessionIPReleasedPayload) {
	if payload.Family != l2tpevents.AddressFamilyIPv4 {
		return
	}
	key := sessionKey{birth: payload.Birth, tunnelID: payload.TunnelID, sessionID: payload.SessionID}
	a.mu.Lock()
	defer a.mu.Unlock()
	// RFC 2866 Section 4.1: "the Framed-IP-Address (if any) in the
	// Accounting-Request MUST contain the actual IP address assigned or
	// negotiated." A released address is neither any more.
	if sess := a.sessions[key]; sess != nil {
		sess.peerAddr = ""
	}
}

func (a *radiusAcct) onSessionIPAssigned(payload *l2tpevents.SessionIPAssignedPayload) {
	if err := subscriber.RequireID(logger(), payload.SubscriberID); err != nil {
		return
	}
	key := sessionKey{birth: payload.Birth, tunnelID: payload.TunnelID, sessionID: payload.SessionID}
	a.mu.Lock()
	endpoint := a.endpoint
	configured := a.interval
	portIDFormat := a.nasPortIDFormat
	if !a.accepting || endpoint == nil {
		a.mu.Unlock()
		return
	}
	// RFC 2866 Section 5.5: "The start and stop records for a given session
	// MUST have the same Acct-Session-Id."
	if sess := a.sessions[key]; sess != nil {
		if payload.PeerAddr != "" {
			sess.peerAddr = payload.PeerAddr
		}
		a.mu.Unlock()
		return
	}
	acctSessID := a.genSessionID(payload.TunnelID, payload.SessionID)

	// An Access-Accept can carry Acct-Interim-Interval (type 85), which RFC 2869
	// Section 5.16 defines. Section 2.1 decides which of the two values wins,
	// and acctInterval enforces it.
	var fromAccept uint32
	if meta := l2tp.LoadSessionMetadata(payload.SubscriberID); meta != nil {
		fromAccept = meta.AcctInterimInterval
	}
	interval := acctInterval(configured, fromAccept)

	ctx, cancel := context.WithCancel(context.Background())
	sess := &acctSession{
		birth:            payload.Birth,
		tunnelID:         payload.TunnelID,
		sessionID:        payload.SessionID,
		username:         payload.Username,
		peerAddr:         payload.PeerAddr,
		callingStationID: payload.CallingStationID,
		acctSessID:       acctSessID,
		nasPortID:        resolveNASPortID(portIDFormat, nasPortIDFacts{nasID: endpoint.nasID, tunnelID: payload.TunnelID, sessionID: payload.SessionID}),
		network:          payload.NetworkLifetime,
		endpoint:         endpoint,
		startTime:        time.Now(),
		cancel:           cancel,
		ended:            make(chan struct{}),
		finished:         make(chan struct{}),
	}
	// PPP read the baseline from the original unit before it published this
	// assignment, while the channel was still disconnected, so a replacement
	// lifetime starts from the retained unit's last traffic totals. An absent
	// or unavailable sample is never read as zero: every record of the
	// session then omits its traffic counters.
	base, err := payload.BaselineCounters.Counters()
	if err != nil {
		sess.countersInvalid.Store(true)
		recordCountersOmitted(sess, "PPP could not read the counter baseline when the session's network started", err)
	} else {
		sess.counterBase = acctTotals(base)
	}

	endpoint.acquire()
	a.sessions[key] = sess
	a.mu.Unlock()

	go a.runAccounting(ctx, sess, interval)
}

// acctRecordBound is how long one accounting record may wait for a server's
// acknowledgement, retransmissions included, before the task gives it up.
const acctRecordBound = 10 * time.Second

// runAccounting is one session's accounting task, and the only sender of the
// session's records: its Accounting-Start, one Interim-Update per interval
// until the session's context ends, then its Accounting-Stop once end or
// endLive hands the end of service over. One goroutine sends all three kinds,
// so the Stop leaves only after the Start and every Interim have been
// acknowledged or given up, and the publisher of the session's Down never
// waits for a RADIUS server or for PPP. The task binds the session's
// network-qualified capture before the Start, retains it for every counter
// read, and, after end, awaits the network's terminal counters within
// acctTerminalBound before it sends the Stop. The task closes sess.finished
// after it releases the endpoint, and its capture goes with it.
//
// Panic boundaries split the task: the capture binding, the Start and Interim
// loop, the terminal wait, and the Stop. A defect in the binding or the
// terminal wait leaves the counters unavailable and the records still go out.
// A Ze defect met in the Start or the
// Interim loop ends those records only: the session stays up and in
// a.sessions, every other session's task runs on, and this task still waits
// for the end of service and sends the Stop. RFC 2866 Section 2 ties the Stop
// to the end of service, which a defect in the accounting task is not: "At
// the end of service delivery the client will generate an Accounting Stop
// packet describing the type of service that was delivered and optionally
// statistics such as elapsed time, input and output octets, or input and
// output packets." A Stop sent at the panic would close the server's record
// of a session still delivering service.
func (a *radiusAcct) runAccounting(ctx context.Context, sess *acctSession, interval time.Duration) {
	defer close(sess.finished)
	defer sess.endpoint.release()
	defer a.forgetEnding(sess)

	a.containAccounting(sess, sess.bindCapture)
	a.containAccounting(sess, func() {
		a.sendAcctStart(sess)
		a.interimLoop(ctx, nil, sess, "", nil, interval)
	})
	<-sess.ended
	if !sess.finalSettled {
		// A defect while awaiting the terminal counters leaves them
		// unavailable; the Stop below is still sent, without them.
		a.containAccounting(sess, sess.captureTerminal)
	}
	a.containAccounting(sess, func() {
		a.sendAcctStop(sess)
	})
}

// forgetEnding removes a finished task's session from the sessions whose Stop
// is still owed. After a retirement took the set, the delete finds nothing,
// and the retirement waits on the task's finished channel instead.
func (a *radiusAcct) forgetEnding(sess *acctSession) {
	a.mu.Lock()
	delete(a.ending, sess)
	a.mu.Unlock()
}

// containAccounting runs one part of a session's accounting task inside the
// accounting panic boundary.
func (a *radiusAcct) containAccounting(sess *acctSession, part func()) {
	describe := func() taskguard.Context { return sessionUnit(sess.tunnelID, sess.sessionID) }
	defer accountingTask.Contain(describe, nil)
	part()
}

// onSessionDown claims the session's record and hands its Accounting-Stop to
// the session's accounting task. It returns without waiting for a RADIUS
// server, because it runs on the publisher of the Down, which can be the
// L2TP reactor: a silent accounting server MUST NOT stop the control packets
// of every other tunnel.
func (a *radiusAcct) onSessionDown(payload *l2tpevents.SessionDownPayload) {
	key := sessionKey{birth: payload.Birth, tunnelID: payload.TunnelID, sessionID: payload.SessionID}

	a.mu.Lock()
	sess, ok := a.sessions[key]
	if ok {
		delete(a.sessions, key)
		a.ending[sess] = struct{}{}
	}
	a.mu.Unlock()

	if !ok {
		return
	}
	// This goroutine is the only caller of end for sess: the delete above
	// took it out of the live sessions, so no other caller claims it.
	sess.end(payload.Cause)
}

func (a *radiusAcct) sendAcctStart(sess *acctSession) {
	endpoint := sess.endpoint
	incAcctSent(endpoint.serverAddr, endpoint.serverAddr)
	pkt := a.buildAcctPacket(sess, endpoint.nasID, endpoint.sourceAddress, radius.AcctStatusStart, 0)
	a.sendAcctPacket(endpoint.client, pkt, "start", sess)
}

// sendAcctStop sends the record end captured. It MUST run after sess.ended
// is closed.
func (a *radiusAcct) sendAcctStop(sess *acctSession) {
	endpoint := sess.endpoint
	incAcctSent(endpoint.serverAddr, endpoint.serverAddr)
	pkt := a.buildAcctPacket(sess, endpoint.nasID, endpoint.sourceAddress, radius.AcctStatusStop, sess.finalDuration)
	a.sendAcctPacket(endpoint.client, pkt, "stop", sess)
}

func (a *radiusAcct) sendAcctInterimUpdate(sess *acctSession) {
	endpoint := sess.endpoint
	incInterimSent(endpoint.serverAddr, endpoint.serverAddr)
	duration := uint32(time.Since(sess.startTime).Seconds())
	pkt := a.buildAcctPacket(sess, endpoint.nasID, endpoint.sourceAddress, radius.AcctStatusInterimUpdate, duration)
	a.sendAcctPacket(endpoint.client, pkt, "interim", sess)
}

// buildAcctPacket assembles one Accounting-Request for a session.
//
// RFC 2866 Section 4.1: "If the Accounting-Request packet includes a
// Framed-IP-Address, that attribute MUST contain the IP address of the user ...
// the Framed-IP-Address (if any) in the Accounting-Request MUST contain the
// actual IP address assigned or negotiated." sess.peerAddr holds exactly that:
// the IPCP-negotiated peer address the reactor put on pppN, delivered by the
// (l2tp, session-ip-assigned) event.
func (a *radiusAcct) buildAcctPacket(sess *acctSession, nasID string, sourceAddr net.IP, statusType uint8, sessionTime uint32) *radius.Packet {
	// RFC 2866 Section 5: "Text of length zero (0) MUST NOT be sent; omit the
	// entire attribute instead." A session the LNS never authenticated carries
	// no username (L2TPSession.username is empty until an ICCN proxy-auth name
	// or an auth plugin response populates it), so User-Name is text whose
	// length the peer picks.
	var attrs []radius.Attr
	attrs = radius.AppendTextAttr(attrs, radius.AttrUserName, sess.username)

	// RFC 2865 Section 5.31: Calling-Station-Id carries "the phone number that
	// the call came from". On the L2TP path that is the peer's Calling Number
	// AVP; on the PPPoE relay path it is the subscriber's MAC address. Both
	// are text, and the same Section 5 zero-length rule applies, so a session
	// whose peer named no calling number sends no attribute.
	attrs = radius.AppendTextAttr(attrs, radius.AttrCallingStationID, sess.callingStationID)

	// RFC 2869 Section 5.3: "This attribute is included in an
	// Accounting-Request packet to record the time that this event occurred on
	// the NAS, in seconds since January 1, 1970 00:00 UTC." The same section
	// gives the Value field as "four octets encoding an unsigned integer",
	// which is what AttrUint32 writes. The uint32 conversion is the RFC's own
	// width, so the attribute wraps in 2106 as every RADIUS client does. A Stop
	// whose end of service was not recorded omits it rather than send 1970.
	if at, ok := sess.eventTime(statusType); ok {
		attrs = append(attrs, radius.Attr{Type: radius.AttrEventTimestamp, Value: radius.AttrUint32(uint32(at.Unix()))})
	}
	attrs = append(attrs,
		radius.Attr{Type: radius.AttrAcctStatusType, Value: radius.AttrUint32(uint32(statusType))},
		radius.Attr{Type: radius.AttrAcctSessionID, Value: radius.AttrString(sess.acctSessID)},
		radius.Attr{Type: radius.AttrServiceType, Value: radius.AttrUint32(radius.ServiceTypeFramed)},
		radius.Attr{Type: radius.AttrFramedProtocol, Value: radius.AttrUint32(radius.FramedProtocolPPP)},
		radius.Attr{Type: radius.AttrNASPortType, Value: radius.AttrUint32(radius.NASPortTypeVirtual)},
		radius.Attr{Type: radius.AttrNASPort, Value: radius.AttrUint32(uint32(sess.sessionID))},
	)

	// RFC 2866 Section 4.1: "Either NAS-IP-Address or NAS-Identifier MUST be
	// present in a RADIUS Accounting-Request." Section 5.13 Note 1 states it
	// again over the Table of Attributes.
	attrs = appendNASIdentity(attrs, nasID, sourceAddr)

	// RFC 2869 Section 5.17: the text resolved when this session started
	// accounting, repeated in every record of the session.
	if attr, ok := nasPortIDAttrFromText(sess.nasPortID); ok {
		attrs = append(attrs, attr)
	}

	a.mu.Lock()
	peerAddr := sess.peerAddr
	a.mu.Unlock()
	// RFC 2865 Section 5.8: Framed-IP-Address is four octets, so only an IPv4
	// assignment can be reported. A session with no address yet, or one whose
	// only assignment is IPv6, sends no attribute rather than a wrong one.
	if v4, ok := subscriberIPv4(peerAddr); ok {
		attrs = append(attrs, radius.Attr{Type: radius.AttrFramedIPAddress, Value: v4})
	}

	if statusType == radius.AcctStatusStop || statusType == radius.AcctStatusInterimUpdate {
		attrs = append(attrs, radius.Attr{Type: radius.AttrAcctSessionTime, Value: radius.AttrUint32(sessionTime)})
		if delta, ok := sess.recordCounters(statusType); ok {
			inOct, inGiga := splitGigawords(delta.rxBytes)
			outOct, outGiga := splitGigawords(delta.txBytes)
			attrs = append(attrs,
				radius.Attr{Type: radius.AttrAcctInputOctets, Value: radius.AttrUint32(inOct)},
				radius.Attr{Type: radius.AttrAcctOutputOctets, Value: radius.AttrUint32(outOct)},
				radius.Attr{Type: radius.AttrAcctInputPackets, Value: radius.AttrUint32(uint32(delta.rxPackets))},
				radius.Attr{Type: radius.AttrAcctOutputPackets, Value: radius.AttrUint32(uint32(delta.txPackets))},
			)

			// RFC 2869 Sections 5.1-5.2: include gigaword attrs only when nonzero.
			if inGiga > 0 {
				attrs = append(attrs, radius.Attr{Type: radius.AttrAcctInputGigawords, Value: radius.AttrUint32(inGiga)})
			}
			if outGiga > 0 {
				attrs = append(attrs, radius.Attr{Type: radius.AttrAcctOutputGigawords, Value: radius.AttrUint32(outGiga)})
			}
		}
	}

	// RFC 2866 Section 5.10: "This attribute indicates how the session was
	// terminated, and can only be present in Accounting-Request records where
	// the Acct-Status-Type is set to Stop." This guard is the only place the
	// attribute is appended, so no Start and no Interim can carry it.
	if statusType == radius.AcctStatusStop {
		attrs = append(attrs, radius.Attr{Type: radius.AttrAcctTerminateCause, Value: radius.AttrUint32(uint32(terminateCauseOrNASError(sess.terminateCause)))})
	}

	// The operator's `attributes exclude` container is applied HERE, to the
	// finished list, and never as a condition on one of the appends above: a
	// condition per append grows with the attribute list and lets an attribute
	// added later miss the feature. RFC 2866 Section 5.13 keeps Acct-Status-Type,
	// Acct-Session-Id and the NAS identity out of the container, so whatever an
	// operator excludes, what survives is still a conformant Accounting-Request.
	exclusions := a.exclusionsNow()
	kind := acctPacketKind(statusType)

	return &radius.Packet{
		Code:  radius.CodeAccountingReq,
		Attrs: exclusions.filter(attrs, kind),
		// Acct-Delay-Time is the one excludable attribute this function does not
		// append. RFC 2866 Section 5.2 counts "how many seconds the client has
		// been trying to send this record for", which only the client knows, so the
		// client writes the attribute (radius/client.go, setAcctDelayTime). Its
		// exclusion therefore travels on the packet rather than through the
		// filter above.
		OmitAcctDelayTime: exclusions.excludes(radius.AttrAcctDelayTime, kind),
	}
}

// recordCounters answers the counters one record reports. A Stop reports the
// final counters: the ended network's terminal measurement, or a live reading
// when accounting retired under a continuing network. An Interim reads the
// original unit now, through the same capture.
//
// It is the one place a record's counters are confirmed present, so a record
// that carries them settles the session's earlier omission finding here. The
// final counters are read in the Stop branch alone: end and endLive write
// them, and only the Stop is built after ended closes.
func (s *acctSession) recordCounters(statusType uint8) (acctCounters, bool) {
	var counters acctCounters
	var present bool
	if statusType == radius.AcctStatusStop {
		counters, present = s.finalCounters, s.finalCountersValid
	} else {
		counters, present = s.liveDelta()
	}
	if present {
		clearCountersOmitted(s)
	}
	return counters, present
}

// sendAcctPacket sends one record and waits, at most acctRecordBound, for its
// acknowledgement. A record no server acknowledged is logged with the
// Acct-Session-Id a billing operator searches for, and the session goes on.
func (a *radiusAcct) sendAcctPacket(client *radius.Client, pkt *radius.Packet, purpose string, sess *acctSession) {
	ctx, cancel := context.WithTimeout(context.Background(), acctRecordBound)
	defer cancel()
	_, err := client.SendToServers(ctx, pkt)
	if err != nil {
		// RFC 2866: accounting failures MUST NOT tear down sessions.
		logger().Warn("l2tp-auth-radius: accounting "+purpose+" not acknowledged",
			"code", codeRADIUSAccountingUnacknowledged,
			"tunnel", sess.tunnelID, "session", sess.sessionID,
			"acct-session-id", sess.acctSessID, "error", err)
	}
	recordAccountingOutcome(sess, purpose, err)
}

// interimLoop sends one interim Accounting-Request per interval until the
// session's context ends. interval is what acctInterval answered for this
// session, and the caller MUST keep it above zero. Every Interim leaves
// through the session's own endpoint, as its Start did.
func (a *radiusAcct) interimLoop(ctx context.Context, _ *radius.Client, sess *acctSession, _ string, _ net.IP, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Cancellation wins over a pending tick. A send that outlasts the
			// interval leaves the ticker ready as well as ctx.Done, and select
			// picks between two ready cases at random, so a loop whose session
			// has ended starts another generation every second time it wakes.
			// end cancels this context at the end of service, and the task
			// sends the Stop once this loop returns, so that generation would
			// delay the Stop behind a record of service that has ended. RFC
			// 2869 Section 2.1: "Note that all information in an interim
			// message is cumulative". The Interim would report counters past
			// the end the Stop reports.
			if ctx.Err() != nil {
				return
			}
			a.sendAcctInterimUpdate(sess)
		}
	}
}

// acctRetirement is the claim retirement takes on the accountant's endpoint,
// its live sessions and the sessions whose Stop is still being sent. Its live
// sessions are no longer in any accountant's map, so stopDetached is the only
// caller that ends them.
type acctRetirement struct {
	endpoint *acctEndpoint
	sessions map[sessionKey]*acctSession
	ending   map[*acctSession]struct{}
}

// detach transfers the current endpoint and live sessions to one retirement
// owner. New configuration can use this accountant immediately without sharing
// its client with the tasks that send the old records. The caller MUST call
// stopDetached on the answer.
func (a *radiusAcct) detach() *acctRetirement {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.endpoint == nil && len(a.sessions) == 0 && len(a.ending) == 0 {
		return nil
	}
	retired := &acctRetirement{endpoint: a.endpoint, sessions: a.sessions, ending: a.ending}
	a.sessions = make(map[sessionKey]*acctSession)
	a.ending = make(map[*acctSession]struct{})
	a.endpoint = nil
	a.accepting = false
	return retired
}

// Stop ends accounting: every live session's Stop is sent, and every client
// is released. Retirement shares one bound across all records, including
// when the server never answers.
func (a *radiusAcct) Stop() {
	if retired := a.detach(); retired != nil {
		retired.stopDetached()
	}
}

// stopDetached ends every retired session, waits for their tasks to send
// their Stops, and releases the endpoints. It MUST be called once, on the
// answer of detach.
//
// The tasks share one bound, acctRecordBound. A task still waiting at the
// bound has an endpoint whose server has not answered; closing that endpoint's
// client ends the exchange at once, so the task records the Stop as not
// acknowledged, by name, and finishes. Retirement never answers before every
// task has finished, so the client of a later configuration is never shared
// with a record of this one.
func (r *acctRetirement) stopDetached() {
	if r.endpoint != nil {
		r.endpoint.retire()
	}
	for _, sess := range r.sessions {
		// RFC 2866 Section 5.10 value 7: administrator ending NAS service.
		// The session's network continues, so its counters are read now.
		sess.endLive(l2tpevents.TerminateCauseAdminReboot)
		r.ending[sess] = struct{}{}
	}
	late := r.awaitTasks(acctRecordBound)
	if len(late) == 0 {
		return
	}
	logger().Warn("l2tp-auth-radius: accounting retirement reached its bound; abandoning the unacknowledged Stops",
		"code", codeRADIUSAccountingUnacknowledged,
		"sessions", len(late), "bound", acctRecordBound)
	for _, sess := range late {
		sess.endpoint.close()
	}
	for _, sess := range late {
		<-sess.finished
	}
}

// awaitTasks waits, at most bound in total, for the task of every session
// whose Stop is owed to finish, and answers the sessions whose task had not
// finished by then.
func (r *acctRetirement) awaitTasks(bound time.Duration) []*acctSession {
	expiry := time.NewTimer(bound)
	defer expiry.Stop()
	expired := false
	var late []*acctSession
	for sess := range r.ending {
		if expired {
			select {
			case <-sess.finished:
			default:
				late = append(late, sess)
			}
			continue
		}
		select {
		case <-sess.finished:
		case <-expiry.C:
			expired = true
			late = append(late, sess)
		}
	}
	return late
}

// terminateCauseOrNASError answers the value a Stop record reports.
//
// A cause of TerminateCauseUnspecified means no teardown site named one, so ze
// does not know how the session ended. RFC 2866 Section 5.10 value 9 is the
// answer for exactly that: "NAS detected some error (other than on the port)
// which required ending the session." Guessing a more specific cause would put
// a wrong reason in an operator's billing store.
func terminateCauseOrNASError(cause l2tpevents.TerminateCause) l2tpevents.TerminateCause {
	if cause == l2tpevents.TerminateCauseUnspecified {
		return l2tpevents.TerminateCauseNASError
	}
	return cause
}

const (
	acctIntervalMin uint32 = 60
	acctIntervalMax uint32 = 3600

	// acctIntervalDefault is the cadence of a session that neither the operator
	// nor the RADIUS server gave one. The acct-interval leaf carries no YANG
	// default, so the 300 seconds live here, and an absent leaf never reads as
	// an interval of zero.
	acctIntervalDefault = 300 * time.Second
)

// acctInterval answers the interim-update cadence of one session.
//
// configured is the acct-interval leaf and is zero when the operator set none.
// fromAccept is the Acct-Interim-Interval attribute (RFC 2869 Section 5.16) of
// this session's Access-Accept, and is zero when the server sent none. The
// answer is always above zero, so an absence on both sides leaves interim
// accounting running rather than stopping it.
func acctInterval(configured time.Duration, fromAccept uint32) time.Duration {
	// RFC 2869 Section 2.1: "It is also possible to statically configure an
	// interim value on the NAS itself. Note that a locally configured value on
	// the NAS MUST override the value found in an Access-Accept."
	if configured > 0 {
		return configured
	}
	if fromAccept > 0 {
		return time.Duration(clampAcctInterval(fromAccept)) * time.Second
	}
	return acctIntervalDefault
}

// clampAcctInterval restricts a RADIUS Acct-Interim-Interval to
// [60, 3600] seconds. Values below the floor are clamped up; values
// above the ceiling are clamped down.
func clampAcctInterval(v uint32) uint32 {
	if v < acctIntervalMin {
		return acctIntervalMin
	}
	if v > acctIntervalMax {
		return acctIntervalMax
	}
	return v
}

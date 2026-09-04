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
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/ze"
)

var acctGetStats = iface.GetStats

// acctNow reads the clock that stamps Event-Timestamp. A test replaces it so
// the encoded seconds are exact rather than a window around the real clock.
var acctNow = time.Now

// acctSession tracks per-session accounting state.
//
// nasPortID is resolved once, when the session starts accounting, and every
// record of that session then repeats it. Re-resolving per packet would let a
// config reload move the text mid-session, and the text is what a billing
// system joins the Start, Interim and Stop records by. It is stored for the
// same reason acctSessID is.
type acctSession struct {
	tunnelID  uint16
	sessionID uint16
	username  string
	peerAddr  string
	// terminateCause is why this session ended, and it is the ONE field of
	// this struct that is written after construction. The writer is whichever
	// of onSessionDown and Stop claimed the session out of radiusAcct.sessions
	// under radiusAcct.mu, and exactly one of them can, so no second goroutine
	// writes it. The interim loop never reads it: buildAcctPacket reads it
	// inside the Acct-Status-Type Stop branch alone.
	terminateCause l2tpevents.TerminateCause
	// callingStationID is the L2TP Calling Number AVP of this session, which
	// every record reports as Calling-Station-Id (RFC 2865 Section 5.31). It
	// is stored for the same reason nasPortID is: the value is a property of
	// the call, so every record of the session repeats the one it started
	// with. Empty when neither side named a calling number.
	callingStationID string
	acctSessID       string
	nasPortID        string
	pppInterface     string
	startTime        time.Time
	cancel           context.CancelFunc
}

// subscriberIPv4 parses a session's assigned address and returns its four
// octets. It reports false for an empty value, an unparseable value, and an
// IPv6 address: the reactor records an IPv6CP link-local in the same field, and
// RFC 2865 Section 5.8 gives Framed-IP-Address four octets, so those cases have
// no attribute to send. An IPv4-mapped form is an IPv4 address and is unwrapped.
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

// radiusAcct manages RADIUS accounting lifecycle.
//
// interval is the acct-interval config leaf, zero when the operator set none.
// It is not a cadence on its own: acctInterval turns it and the Access-Accept
// into the cadence one session runs at.
type radiusAcct struct {
	mu              sync.Mutex
	sessions        map[sessionKey]*acctSession
	client          *radius.Client
	nasID           string
	sourceAddress   net.IP
	interval        time.Duration
	nextSess        uint32
	serverAddr      string
	nasPortIDFormat string
}

type sessionKey struct {
	tunnelID  uint16
	sessionID uint16
}

func newRADIUSAcct() *radiusAcct {
	return &radiusAcct{
		sessions: make(map[sessionKey]*acctSession),
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
func (a *radiusAcct) setClient(c *radius.Client, nasID string, interval time.Duration, serverAddr string, sourceAddr net.IP, nasPortIDFormat string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.client = c
	a.nasID = nasID
	a.sourceAddress = sourceAddr
	a.serverAddr = serverAddr
	a.nasPortIDFormat = nasPortIDFormat
	a.interval = interval
}

func (a *radiusAcct) genSessionID(tunnelID, sessionID uint16) string {
	a.mu.Lock()
	a.nextSess++
	n := a.nextSess
	a.mu.Unlock()
	var b textbuf.Buffer
	return b.Reset().Int(int64(tunnelID)).Byte('-').Int(int64(sessionID)).Byte('-').Int(int64(n)).String()
}

// subscribeEventBus subscribes to session lifecycle events for accounting.
func (a *radiusAcct) subscribeEventBus(bus ze.EventBus) {
	if bus == nil {
		return
	}

	l2tpevents.SessionIPAssigned.Subscribe(bus, func(payload *l2tpevents.SessionIPAssignedPayload) {
		a.onSessionIPAssigned(payload)
	})

	l2tpevents.SessionDown.Subscribe(bus, func(payload *l2tpevents.SessionDownPayload) {
		a.onSessionDown(payload)
	})
}

func (a *radiusAcct) onSessionIPAssigned(payload *l2tpevents.SessionIPAssignedPayload) {
	a.mu.Lock()
	client := a.client
	nasID := a.nasID
	srcAddr := a.sourceAddress
	configured := a.interval
	portIDFormat := a.nasPortIDFormat
	a.mu.Unlock()

	if client == nil {
		return
	}

	// An Access-Accept can carry Acct-Interim-Interval (type 85), which RFC 2869
	// Section 5.16 defines. Section 2.1 decides which of the two values wins,
	// and acctInterval enforces it.
	var fromAccept uint32
	if meta := l2tp.LoadSessionMetadata(payload.TunnelID, payload.SessionID); meta != nil {
		fromAccept = meta.AcctInterimInterval
	}
	interval := acctInterval(configured, fromAccept)

	key := sessionKey{payload.TunnelID, payload.SessionID}
	acctSessID := a.genSessionID(payload.TunnelID, payload.SessionID)

	ctx, cancel := context.WithCancel(context.Background())
	sess := &acctSession{
		tunnelID:         payload.TunnelID,
		sessionID:        payload.SessionID,
		username:         payload.Username,
		peerAddr:         payload.PeerAddr,
		callingStationID: payload.CallingStationID,
		acctSessID:       acctSessID,
		nasPortID:        resolveNASPortID(portIDFormat, nasPortIDFacts{nasID: nasID, tunnelID: payload.TunnelID, sessionID: payload.SessionID}),
		pppInterface:     payload.PppInterface,
		startTime:        time.Now(),
		cancel:           cancel,
	}

	a.mu.Lock()
	a.sessions[key] = sess
	a.mu.Unlock()

	go func() {
		a.sendAcctStart(client, sess, nasID, srcAddr)
		a.interimLoop(ctx, client, sess, nasID, srcAddr, interval)
	}()
}

func (a *radiusAcct) onSessionDown(payload *l2tpevents.SessionDownPayload) {
	key := sessionKey{payload.TunnelID, payload.SessionID}

	a.mu.Lock()
	sess, ok := a.sessions[key]
	if ok {
		delete(a.sessions, key)
	}
	client := a.client
	nasID := a.nasID
	srcAddr := a.sourceAddress
	a.mu.Unlock()

	if !ok || client == nil {
		return
	}

	sess.cancel()
	// This goroutine owns sess now: the delete above took it out of the map,
	// so no other caller reaches it.
	sess.terminateCause = payload.Cause
	a.sendAcctStop(client, sess, nasID, srcAddr)
}

func (a *radiusAcct) sendAcctStart(client *radius.Client, sess *acctSession, nasID string, sourceAddr net.IP) {
	a.mu.Lock()
	sAddr := a.serverAddr
	a.mu.Unlock()
	incAcctSent(sAddr, sAddr)
	pkt := a.buildAcctPacket(sess, nasID, sourceAddr, radius.AcctStatusStart, 0)
	a.sendAcctPacket(client, pkt, "start", sess)
}

func (a *radiusAcct) sendAcctStop(client *radius.Client, sess *acctSession, nasID string, sourceAddr net.IP) {
	a.mu.Lock()
	sAddr := a.serverAddr
	a.mu.Unlock()
	incAcctSent(sAddr, sAddr)
	duration := uint32(time.Since(sess.startTime).Seconds())
	pkt := a.buildAcctPacket(sess, nasID, sourceAddr, radius.AcctStatusStop, duration)
	a.sendAcctPacket(client, pkt, "stop", sess)
}

func (a *radiusAcct) sendAcctInterimUpdate(client *radius.Client, sess *acctSession, nasID string, sourceAddr net.IP) {
	a.mu.Lock()
	sAddr := a.serverAddr
	a.mu.Unlock()
	incInterimSent(sAddr, sAddr)
	duration := uint32(time.Since(sess.startTime).Seconds())
	pkt := a.buildAcctPacket(sess, nasID, sourceAddr, radius.AcctStatusInterimUpdate, duration)
	a.sendAcctPacket(client, pkt, "interim", sess)
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
	// width, so the attribute wraps in 2106 as every RADIUS client does.
	attrs = append(attrs,
		radius.Attr{Type: radius.AttrEventTimestamp, Value: radius.AttrUint32(uint32(acctNow().Unix()))},
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

	// RFC 2865 Section 5.8: Framed-IP-Address is four octets, so only an IPv4
	// assignment can be reported. A session with no address yet, or one whose
	// only assignment is IPv6, sends no attribute rather than a wrong one.
	if v4, ok := subscriberIPv4(sess.peerAddr); ok {
		attrs = append(attrs, radius.Attr{Type: radius.AttrFramedIPAddress, Value: v4})
	}

	if statusType == radius.AcctStatusStop || statusType == radius.AcctStatusInterimUpdate {
		var rxBytes, txBytes uint64
		var rxPkts, txPkts uint64
		if sess.pppInterface != "" {
			if stats, err := acctGetStats(sess.pppInterface); err == nil {
				rxBytes = stats.RxBytes
				txBytes = stats.TxBytes
				rxPkts = stats.RxPackets
				txPkts = stats.TxPackets
			} else {
				logger().Warn("l2tp-auth-radius: counter read failed",
					"interface", sess.pppInterface, "error", err)
			}
		}

		// RFC 2866 Section 5.7-5.10
		inOct, inGiga := splitGigawords(rxBytes)
		outOct, outGiga := splitGigawords(txBytes)

		attrs = append(attrs,
			radius.Attr{Type: radius.AttrAcctSessionTime, Value: radius.AttrUint32(sessionTime)},
			radius.Attr{Type: radius.AttrAcctInputOctets, Value: radius.AttrUint32(inOct)},
			radius.Attr{Type: radius.AttrAcctOutputOctets, Value: radius.AttrUint32(outOct)},
			radius.Attr{Type: radius.AttrAcctInputPackets, Value: radius.AttrUint32(uint32(rxPkts))},
			radius.Attr{Type: radius.AttrAcctOutputPackets, Value: radius.AttrUint32(uint32(txPkts))},
		)

		// RFC 2869 Section 5.1-5.2: include gigaword attrs only when non-zero.
		if inGiga > 0 {
			attrs = append(attrs, radius.Attr{Type: radius.AttrAcctInputGigawords, Value: radius.AttrUint32(inGiga)})
		}
		if outGiga > 0 {
			attrs = append(attrs, radius.Attr{Type: radius.AttrAcctOutputGigawords, Value: radius.AttrUint32(outGiga)})
		}
	}

	// RFC 2866 Section 5.10: "This attribute indicates how the session was
	// terminated, and can only be present in Accounting-Request records where
	// the Acct-Status-Type is set to Stop." This guard is the only place the
	// attribute is appended, so no Start and no Interim can carry it.
	if statusType == radius.AcctStatusStop {
		attrs = append(attrs, radius.Attr{Type: radius.AttrAcctTerminateCause, Value: radius.AttrUint32(uint32(terminateCauseOrNASError(sess.terminateCause)))})
	}

	return &radius.Packet{
		Code:  radius.CodeAccountingReq,
		Attrs: attrs,
	}
}

func (a *radiusAcct) sendAcctPacket(client *radius.Client, pkt *radius.Packet, purpose string, sess *acctSession) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.SendToServers(ctx, pkt)
	if err != nil {
		// RFC 2866: accounting failures MUST NOT tear down sessions.
		logger().Warn("l2tp-auth-radius: accounting "+purpose+" failed",
			"tunnel", sess.tunnelID, "session", sess.sessionID, "error", err)
	}
}

// interimLoop sends one interim Accounting-Request per interval until the
// session's context ends. interval is what acctInterval answered for this
// session, and the caller MUST keep it above zero.
func (a *radiusAcct) interimLoop(ctx context.Context, _ *radius.Client, sess *acctSession, _ string, _ net.IP, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Read current client/nasID/sourceAddr on each iteration so reload
			// takes effect without restarting the loop.
			a.mu.Lock()
			c := a.client
			nid := a.nasID
			src := a.sourceAddress
			a.mu.Unlock()
			if c != nil {
				a.sendAcctInterimUpdate(c, sess, nid, src)
			}
		}
	}
}

// Stop sends Accounting-Stop for all active sessions and cancels them.
func (a *radiusAcct) Stop() {
	a.mu.Lock()
	client := a.client
	nasID := a.nasID
	srcAddr := a.sourceAddress
	active := make([]*acctSession, 0, len(a.sessions))
	for _, sess := range a.sessions {
		active = append(active, sess)
	}
	a.sessions = make(map[sessionKey]*acctSession)
	a.mu.Unlock()

	for _, sess := range active {
		// RFC 2866 Section 5.10 value 7: "Administrator is ending service on
		// the NAS, for example prior to rebooting the NAS." Stop() runs when
		// the plugin shuts down, which is that ending. The map was cleared
		// above, so this goroutine owns every session in active.
		sess.terminateCause = l2tpevents.TerminateCauseAdminReboot
		if client != nil {
			a.sendAcctStop(client, sess, nasID, srcAddr)
		}
		sess.cancel()
	}
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

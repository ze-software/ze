// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS accounting
// Related: handler.go -- RADIUS auth handler shares the client

package l2tpauthradius

import (
	"context"
	"net"
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

// acctSession tracks per-session accounting state.
type acctSession struct {
	tunnelID     uint16
	sessionID    uint16
	username     string
	peerAddr     string
	acctSessID   string
	pppInterface string
	startTime    time.Time
	cancel       context.CancelFunc
}

// splitGigawords splits a uint64 byte count into a uint32 octets value
// and a uint32 gigawords value for RADIUS encoding.
// RFC 2869 Section 5.1: gigawords = total_bytes >> 32.
func splitGigawords(bytes uint64) (octets, gigawords uint32) {
	return uint32(bytes & 0xFFFFFFFF), uint32(bytes >> 32)
}

// radiusAcct manages RADIUS accounting lifecycle.
type radiusAcct struct {
	mu            sync.Mutex
	sessions      map[sessionKey]*acctSession
	client        *radius.Client
	nasID         string
	sourceAddress net.IP
	interval      time.Duration
	nextSess      uint32
	serverAddr    string
}

type sessionKey struct {
	tunnelID  uint16
	sessionID uint16
}

func newRADIUSAcct() *radiusAcct {
	return &radiusAcct{
		sessions: make(map[sessionKey]*acctSession),
		interval: 300 * time.Second,
	}
}

func (a *radiusAcct) setClient(c *radius.Client, nasID string, interval time.Duration, serverAddr string, sourceAddr net.IP) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.client = c
	a.nasID = nasID
	a.sourceAddress = sourceAddr
	a.serverAddr = serverAddr
	if interval > 0 {
		a.interval = interval
	}
}

func (a *radiusAcct) genSessionID(tunnelID, sessionID uint16) string {
	a.mu.Lock()
	a.nextSess++
	n := a.nextSess
	a.mu.Unlock()
	var b textbuf.Buffer
	return b.Reset().Int(int64(tunnelID)).Byte('-').Int(int64(sessionID)).Byte('-').Int(int64(n)).String()
}

// SubscribeEventBus subscribes to session lifecycle events for accounting.
func (a *radiusAcct) SubscribeEventBus(bus ze.EventBus) {
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
	interval := a.interval
	a.mu.Unlock()

	if client == nil {
		return
	}

	// RFC 2866 Section 5.18: per-session Acct-Interim-Interval override.
	if meta := l2tp.LoadSessionMetadata(payload.TunnelID, payload.SessionID); meta != nil && meta.AcctInterimInterval > 0 {
		interval = time.Duration(clampAcctInterval(meta.AcctInterimInterval)) * time.Second
	}

	key := sessionKey{payload.TunnelID, payload.SessionID}
	acctSessID := a.genSessionID(payload.TunnelID, payload.SessionID)

	ctx, cancel := context.WithCancel(context.Background())
	sess := &acctSession{
		tunnelID:     payload.TunnelID,
		sessionID:    payload.SessionID,
		username:     payload.Username,
		peerAddr:     payload.PeerAddr,
		acctSessID:   acctSessID,
		pppInterface: payload.PppInterface,
		startTime:    time.Now(),
		cancel:       cancel,
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

func (a *radiusAcct) buildAcctPacket(sess *acctSession, nasID string, sourceAddr net.IP, statusType uint8, sessionTime uint32) *radius.Packet {
	attrs := []radius.Attr{
		{Type: radius.AttrUserName, Value: radius.AttrString(sess.username)},
		{Type: radius.AttrAcctStatusType, Value: radius.AttrUint32(uint32(statusType))},
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString(sess.acctSessID)},
		{Type: radius.AttrServiceType, Value: radius.AttrUint32(radius.ServiceTypeFramed)},
		{Type: radius.AttrFramedProtocol, Value: radius.AttrUint32(radius.FramedProtocolPPP)},
		{Type: radius.AttrNASPortType, Value: radius.AttrUint32(radius.NASPortTypeVirtual)},
		{Type: radius.AttrNASPort, Value: radius.AttrUint32(uint32(sess.sessionID))},
	}

	if v4 := sourceAddr.To4(); v4 != nil {
		attrs = append(attrs, radius.Attr{Type: radius.AttrNASIPAddress, Value: v4})
	}

	if nasID != "" {
		attrs = append(attrs, radius.Attr{Type: radius.AttrNASIdentifier, Value: radius.AttrString(nasID)})
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
		if client != nil {
			a.sendAcctStop(client, sess, nasID, srcAddr)
		}
		sess.cancel()
	}
}

const (
	acctIntervalMin uint32 = 60
	acctIntervalMax uint32 = 3600
)

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

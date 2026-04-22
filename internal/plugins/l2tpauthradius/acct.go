// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS accounting
// Related: handler.go -- RADIUS auth handler shares the client

package l2tpauthradius

import (
	"context"
	"fmt"
	"sync"
	"time"

	l2tpevents "codeberg.org/thomas-mangin/ze/internal/component/l2tp/events"
	"codeberg.org/thomas-mangin/ze/internal/component/radius"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// acctSession tracks per-session accounting state.
type acctSession struct {
	tunnelID   uint16
	sessionID  uint16
	username   string
	peerAddr   string
	acctSessID string
	startTime  time.Time
	cancel     context.CancelFunc
}

// radiusAcct manages RADIUS accounting lifecycle.
type radiusAcct struct {
	mu         sync.Mutex
	sessions   map[sessionKey]*acctSession
	client     *radius.Client
	nasID      string
	interval   time.Duration
	nextSess   uint32
	serverAddr string
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

func (a *radiusAcct) setClient(c *radius.Client, nasID string, interval time.Duration, serverAddr string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.client = c
	a.nasID = nasID
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
	return fmt.Sprintf("%d-%d-%d", tunnelID, sessionID, n)
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
	interval := a.interval
	a.mu.Unlock()

	if client == nil {
		return
	}

	key := sessionKey{payload.TunnelID, payload.SessionID}
	acctSessID := a.genSessionID(payload.TunnelID, payload.SessionID)

	ctx, cancel := context.WithCancel(context.Background())
	sess := &acctSession{
		tunnelID:   payload.TunnelID,
		sessionID:  payload.SessionID,
		username:   payload.Username,
		peerAddr:   payload.PeerAddr,
		acctSessID: acctSessID,
		startTime:  time.Now(),
		cancel:     cancel,
	}

	a.mu.Lock()
	a.sessions[key] = sess
	a.mu.Unlock()

	go func() {
		a.sendAcctStart(client, sess, nasID)
		a.interimLoop(ctx, client, sess, nasID, interval)
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
	a.mu.Unlock()

	if !ok || client == nil {
		return
	}

	sess.cancel()
	a.sendAcctStop(client, sess, nasID)
}

func (a *radiusAcct) sendAcctStart(client *radius.Client, sess *acctSession, nasID string) {
	a.mu.Lock()
	sAddr := a.serverAddr
	a.mu.Unlock()
	incAcctSent(sAddr, sAddr)
	pkt := a.buildAcctPacket(sess, nasID, radius.AcctStatusStart, 0)
	a.sendAcctPacket(client, pkt, "start", sess)
}

func (a *radiusAcct) sendAcctStop(client *radius.Client, sess *acctSession, nasID string) {
	a.mu.Lock()
	sAddr := a.serverAddr
	a.mu.Unlock()
	incAcctSent(sAddr, sAddr)
	duration := uint32(time.Since(sess.startTime).Seconds())
	pkt := a.buildAcctPacket(sess, nasID, radius.AcctStatusStop, duration)
	a.sendAcctPacket(client, pkt, "stop", sess)
}

func (a *radiusAcct) sendAcctInterimUpdate(client *radius.Client, sess *acctSession, nasID string) {
	a.mu.Lock()
	sAddr := a.serverAddr
	a.mu.Unlock()
	incInterimSent(sAddr, sAddr)
	duration := uint32(time.Since(sess.startTime).Seconds())
	pkt := a.buildAcctPacket(sess, nasID, radius.AcctStatusInterimUpdate, duration)
	a.sendAcctPacket(client, pkt, "interim", sess)
}

func (a *radiusAcct) buildAcctPacket(sess *acctSession, nasID string, statusType uint8, sessionTime uint32) *radius.Packet {
	attrs := []radius.Attr{
		{Type: radius.AttrUserName, Value: radius.AttrString(sess.username)},
		{Type: radius.AttrAcctStatusType, Value: radius.AttrUint32(uint32(statusType))},
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString(sess.acctSessID)},
		{Type: radius.AttrServiceType, Value: radius.AttrUint32(radius.ServiceTypeFramed)},
		{Type: radius.AttrFramedProtocol, Value: radius.AttrUint32(radius.FramedProtocolPPP)},
		{Type: radius.AttrNASPortType, Value: radius.AttrUint32(radius.NASPortTypeVirtual)},
		{Type: radius.AttrNASPort, Value: radius.AttrUint32(uint32(sess.sessionID))},
	}

	if nasID != "" {
		attrs = append(attrs, radius.Attr{Type: radius.AttrNASIdentifier, Value: radius.AttrString(nasID)})
	}

	if statusType == radius.AcctStatusStop || statusType == radius.AcctStatusInterimUpdate {
		attrs = append(attrs,
			radius.Attr{Type: radius.AttrAcctSessionTime, Value: radius.AttrUint32(sessionTime)},
			radius.Attr{Type: radius.AttrAcctInputOctets, Value: radius.AttrUint32(0)},
			radius.Attr{Type: radius.AttrAcctOutputOctets, Value: radius.AttrUint32(0)},
		)
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

func (a *radiusAcct) interimLoop(ctx context.Context, client *radius.Client, sess *acctSession, nasID string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.sendAcctInterimUpdate(client, sess, nasID)
		}
	}
}

// Stop cancels all active accounting sessions.
func (a *radiusAcct) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, sess := range a.sessions {
		sess.cancel()
	}
	a.sessions = make(map[sessionKey]*acctSession)
}

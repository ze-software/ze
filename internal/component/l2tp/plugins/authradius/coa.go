// Design: docs/research/l2tpv2-ze-integration.md -- CoA/DM listener
// Related: register.go -- plugin lifecycle starts/stops the listener
// Related: config.go -- CoAPort configuration

package l2tpauthradius

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/component/traffic"
	coreCos "github.com/ze-software/ze/internal/core/cos"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/ze"
)

// coaListener handles RADIUS CoA-Request and Disconnect-Request packets
// per RFC 5176. It validates the authenticator using the RADIUS shared
// secret, identifies the matching L2TP session, and either emits a
// rate-change event (CoA) or tears down the session (DM).
type coaListener struct {
	conn           *net.UDPConn
	secrets        map[string][]byte // source IP -> shared secret
	defaultSecret  []byte
	bus            ze.EventBus
	allowedSources []net.IP
	done           chan struct{}
	replayMu       sync.Mutex
	replay         map[coaReplayKey]coaReplayEntry
}

type coaReplayKey struct {
	source        string
	code          uint8
	identifier    uint8
	authenticator [radius.AuthenticatorLen]byte
}

type coaReplayEntry struct {
	seen     time.Time
	response []byte
}

const coaReplayWindow = 5 * time.Minute

func newCoAListener(port int, secrets map[string][]byte, defaultSecret []byte, bus ze.EventBus, allowedSources []net.IP) (*coaListener, error) {
	var bAddr textbuf.Buffer
	addr, err := net.ResolveUDPAddr("udp4", bAddr.Reset().Byte(':').Int(int64(port)).String())
	if err != nil {
		return nil, fmt.Errorf("coa: resolve: %w", err)
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return nil, fmt.Errorf("coa: listen: %w", err)
	}
	cl := &coaListener{
		conn:           conn,
		secrets:        secrets,
		defaultSecret:  defaultSecret,
		bus:            bus,
		allowedSources: allowedSources,
		done:           make(chan struct{}),
		replay:         make(map[coaReplayKey]coaReplayEntry),
	}
	go cl.serve()
	return cl, nil
}

func (cl *coaListener) Close() error {
	err := cl.conn.Close()
	<-cl.done
	return err
}

func (cl *coaListener) serve() {
	defer close(cl.done)
	buf := make([]byte, radius.MaxPacketLen)
	for {
		n, from, err := cl.conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		cl.handlePacket(buf[:n], from)
	}
}

func (cl *coaListener) handlePacket(data []byte, from *net.UDPAddr) {
	if len(data) < radius.MinPacketLen {
		return
	}

	// RFC 5176 Section 3.5: accept only from configured RADIUS servers.
	if !cl.isAllowedSource(from.IP) {
		logger().Debug("coa: source not in allowed list, discarding", "from", from)
		return
	}

	// RFC 5176 Section 3.5: verify request authenticator before processing.
	secret := cl.secretForSource(from.IP)
	if !radius.VerifyCoARequestAuth(data, secret) {
		logger().Debug("coa: invalid authenticator, discarding", "from", from)
		return
	}

	pkt, err := radius.Decode(data)
	if err != nil {
		logger().Warn("coa: decode failed", "from", from, "error", err)
		return
	}
	if pkt.Code != radius.CodeCoARequest && pkt.Code != radius.CodeDisconnectRequest {
		logger().Warn("coa: unexpected code", "code", pkt.Code, "from", from)
		return
	}
	if pkt.FindAttr(radius.AttrMessageAuthenticator) == nil {
		logger().Debug("coa: missing message-authenticator, discarding", "from", from)
		return
	}
	if !radius.VerifyMessageAuthenticator(data, secret) {
		logger().Debug("coa: invalid message-authenticator, discarding", "from", from)
		return
	}
	if cached := cl.cachedReplay(from.IP, pkt); cached != nil {
		cl.sendRawResponse(from, cached)
		return
	}
	if !validEventTimestamp(pkt, time.Now()) {
		cl.sendResponse(from, pkt, nakCode(pkt.Code), radius.ErrorCauseInvalidRequest)
		return
	}

	switch pkt.Code {
	case radius.CodeCoARequest:
		cl.handleCoA(pkt, from)
	case radius.CodeDisconnectRequest:
		cl.handleDisconnect(pkt, from)
	}
}

// secretForSource returns the shared secret for the given source IP.
// Falls back to the default secret if no per-source secret is configured.
func (cl *coaListener) secretForSource(ip net.IP) []byte {
	if s, ok := cl.secrets[ip.String()]; ok {
		return s
	}
	return cl.defaultSecret
}

func (cl *coaListener) isAllowedSource(ip net.IP) bool {
	if len(cl.allowedSources) == 0 {
		return true
	}
	for _, allowed := range cl.allowedSources {
		if allowed.Equal(ip) {
			return true
		}
	}
	return false
}

func (cl *coaListener) handleCoA(pkt *radius.Packet, from *net.UDPAddr) {
	downloadRate := extractRate(pkt)
	cosProfile := extractCoSProfile(pkt)

	if downloadRate == 0 && cosProfile == "" {
		cl.sendResponse(from, pkt, radius.CodeCoANAK, radius.ErrorCauseUnsupportedAttribute)
		return
	}

	// Try subscriber registry first (works for both PPPoE and L2TP).
	if subSess, ok := cl.findSubscriberSession(pkt); ok {
		if downloadRate > 0 && cl.bus != nil {
			if _, emitErr := subevents.SessionRateChange.Emit(cl.bus, &subevents.SessionRateChangePayload{
				SessionID:    subSess.ID,
				DownloadRate: downloadRate,
				UploadRate:   downloadRate,
			}); emitErr != nil {
				logger().Warn("coa: emit subscriber rate-change failed", "error", emitErr)
			}
		}
		// For L2TP sessions, also emit the L2TP-specific event so the
		// existing shaper plugin picks it up.
		if downloadRate > 0 && subSess.AccessType == subscriber.AccessL2TP && cl.bus != nil {
			if _, emitErr := l2tpevents.SessionRateChange.Emit(cl.bus, &l2tpevents.SessionRateChangePayload{
				TunnelID:     subSess.TunnelID,
				SessionID:    subSess.SessionID,
				DownloadRate: downloadRate,
				UploadRate:   downloadRate,
			}); emitErr != nil {
				logger().Warn("coa: emit l2tp rate-change failed", "error", emitErr)
			}
		}
		if cosProfile != "" && cl.bus != nil {
			if _, emitErr := l2tpevents.SessionCoSChange.Emit(cl.bus, &l2tpevents.SessionCoSChangePayload{
				TunnelID:        subSess.TunnelID,
				SessionID:       subSess.SessionID,
				AccessInterface: subSess.AccessInterface,
				ProfileName:     cosProfile,
			}); emitErr != nil {
				logger().Warn("coa: emit cos-change failed", "error", emitErr)
			}
		}
		cl.sendResponse(from, pkt, radius.CodeCoAACK, 0)
		logger().Info("coa: accepted CoA",
			"subscriber", subSess.ID, "rate-bps", downloadRate,
			"cos-profile", cosProfile, "from", from)
		return
	}

	// Fallback: L2TP-only lookup by session ID.
	sid, ok := cl.findSession(pkt)
	if !ok {
		cl.sendResponse(from, pkt, radius.CodeCoANAK, radius.ErrorCauseSessionNotFound)
		return
	}

	svc := l2tp.LookupService()
	if svc == nil {
		cl.sendResponse(from, pkt, radius.CodeCoANAK, radius.ErrorCauseSessionNotFound)
		return
	}
	sess, sessOK := svc.LookupSession(sid)
	if !sessOK {
		cl.sendResponse(from, pkt, radius.CodeCoANAK, radius.ErrorCauseSessionNotFound)
		return
	}

	if cl.bus != nil {
		if downloadRate > 0 {
			if _, emitErr := l2tpevents.SessionRateChange.Emit(cl.bus, &l2tpevents.SessionRateChangePayload{
				TunnelID:     sess.TunnelLocalTID,
				SessionID:    sid,
				DownloadRate: downloadRate,
				UploadRate:   downloadRate,
			}); emitErr != nil {
				logger().Warn("coa: emit rate-change failed", "error", emitErr)
			}
		}
		if cosProfile != "" {
			logger().Warn("coa: CoS change requested for L2TP-only session without AccessInterface; skipping",
				"session", sid, "profile", cosProfile)
		}
	}

	cl.sendResponse(from, pkt, radius.CodeCoAACK, 0)
	logger().Info("coa: accepted CoA",
		"session", sid, "rate-bps", downloadRate,
		"cos-profile", cosProfile, "from", from)
}

func (cl *coaListener) handleDisconnect(pkt *radius.Packet, from *net.UDPAddr) {
	// Try subscriber registry first for PPPoE sessions.
	if subSess, ok := cl.findSubscriberSession(pkt); ok && subSess.AccessType == subscriber.AccessPPPoE {
		// PPPoE disconnect: look up the PPPoE subsystem and tear down.
		// The PPP driver's StopSession triggers EventSessionDown which
		// cleans up the subscriber registry via the event consumer.
		logger().Info("coa: disconnect-request for PPPoE session not yet wired to PPPoE teardown",
			"subscriber", subSess.ID, "from", from)
		cl.sendResponse(from, pkt, radius.CodeDisconnectNAK, radius.ErrorCauseSessionNotFound)
		return
	}

	sid, ok := cl.findSession(pkt)
	if !ok {
		cl.sendResponse(from, pkt, radius.CodeDisconnectNAK, radius.ErrorCauseSessionNotFound)
		return
	}

	svc := l2tp.LookupService()
	if svc == nil {
		cl.sendResponse(from, pkt, radius.CodeDisconnectNAK, radius.ErrorCauseSessionNotFound)
		return
	}

	if err := svc.TeardownSession(sid); err != nil {
		logger().Warn("coa: teardown failed", "session", sid, "error", err)
		cl.sendResponse(from, pkt, radius.CodeDisconnectNAK, radius.ErrorCauseSessionNotFound)
		return
	}

	cl.sendResponse(from, pkt, radius.CodeDisconnectACK, 0)
	logger().Info("coa: disconnected session", "session", sid, "from", from)
}

// findSubscriberSession looks up the subscriber registry by Acct-Session-Id.
// Works for both PPPoE and L2TP sessions.
func (cl *coaListener) findSubscriberSession(pkt *radius.Packet) (subscriber.Session, bool) {
	acctSessID := pkt.FindAttr(radius.AttrAcctSessionID)
	if acctSessID == nil {
		return subscriber.Session{}, false
	}
	svc := subscriber.LookupService()
	if svc == nil {
		return subscriber.Session{}, false
	}
	return svc.Registry.LookupByAcctSessionID(string(acctSessID))
}

// findSession identifies the target session from CoA/DM attributes.
// Tries Acct-Session-Id first, then User-Name + NAS-Port.
func (cl *coaListener) findSession(pkt *radius.Packet) (uint16, bool) {
	svc := l2tp.LookupService()
	if svc == nil {
		return 0, false
	}

	// Try Acct-Session-Id. The accounting plugin generates IDs as
	// "tunnelID-sessionID-seqNum" (acct.go genSessionID). Match by
	// the "tunnelID-sessionID-" prefix since the seqNum is opaque.
	if acctSessID := pkt.FindAttr(radius.AttrAcctSessionID); acctSessID != nil {
		snap := svc.Snapshot()
		for i := range snap.Tunnels {
			for j := range snap.Tunnels[i].Sessions {
				var pb textbuf.Buffer
				prefix := pb.Reset().Int(int64(snap.Tunnels[i].LocalTID)).Byte('-').Int(int64(snap.Tunnels[i].Sessions[j].LocalSID)).Byte('-').String()
				if strings.HasPrefix(string(acctSessID), prefix) {
					return snap.Tunnels[i].Sessions[j].LocalSID, true
				}
			}
		}
	}

	// Try User-Name + NAS-Port.
	userName := pkt.FindAttr(radius.AttrUserName)
	nasPortAttr := pkt.FindAttr(radius.AttrNASPort)
	if userName != nil && len(nasPortAttr) == 4 {
		nasPort := binary.BigEndian.Uint32(nasPortAttr)
		snap := svc.Snapshot()
		for i := range snap.Tunnels {
			for j := range snap.Tunnels[i].Sessions {
				if snap.Tunnels[i].Sessions[j].Username == string(userName) && uint32(snap.Tunnels[i].Sessions[j].LocalSID) == nasPort {
					return snap.Tunnels[i].Sessions[j].LocalSID, true
				}
			}
		}
	}

	return 0, false
}

// extractRate reads the download rate from CoA attributes.
// Checks Filter-Id first, then MikroTik VSA as fallback.
func extractRate(pkt *radius.Packet) uint64 {
	for _, raw := range pkt.FindAllAttr(radius.AttrFilterID) {
		if rate, err := traffic.ParseRateBps(string(raw)); err == nil {
			return rate
		}
	}
	if rate := extractVSARate(pkt); rate > 0 {
		return rate
	}
	return 0
}

// extractCoSProfile reads the CoS profile name from CoA attributes.
// Checks Filter-Id first, then vendor VSA as fallback.
func extractCoSProfile(pkt *radius.Packet) string {
	for _, raw := range pkt.FindAllAttr(radius.AttrFilterID) {
		if name, ok := coreCos.ParseFilterID(string(raw)); ok {
			return name
		}
	}
	if name := extractVSACoSProfile(pkt); name != "" {
		return name
	}
	return ""
}

func (cl *coaListener) sendResponse(to *net.UDPAddr, req *radius.Packet, code uint8, errorCause uint32) {
	resp := &radius.Packet{
		Code:       code,
		Identifier: req.Identifier,
	}
	if errorCause != 0 {
		var buf [4]byte
		binary.BigEndian.PutUint32(buf[:], errorCause)
		resp.Attrs = append(resp.Attrs, radius.Attr{Type: radius.AttrErrorCause, Value: buf[:]})
	}

	wireBuf := radius.Bufs.Get()
	defer radius.Bufs.Put(wireBuf)

	n, err := resp.EncodeTo(wireBuf, 0)
	if err != nil {
		logger().Warn("coa: encode response failed", "error", err)
		return
	}

	// RFC 5176 Section 3.5: response authenticator = MD5(Code+ID+Length+RequestAuth+Attrs+Secret).
	respAuth := radius.ResponseAuthenticator(code, req.Identifier,
		binary.BigEndian.Uint16(wireBuf[2:4]),
		req.Authenticator, wireBuf[radius.HeaderLen:n], cl.secretForSource(to.IP))
	copy(wireBuf[4:4+radius.AuthenticatorLen], respAuth[:])

	wire := make([]byte, n)
	copy(wire, wireBuf[:n])
	if _, err := cl.conn.WriteToUDP(wire, to); err != nil {
		logger().Warn("coa: send response failed", "error", err)
		return
	}
	cl.rememberReplay(to.IP, req, wire)
}

func (cl *coaListener) sendRawResponse(to *net.UDPAddr, wire []byte) {
	if _, err := cl.conn.WriteToUDP(wire, to); err != nil {
		logger().Warn("coa: send cached response failed", "error", err)
	}
}

func validEventTimestamp(pkt *radius.Packet, now time.Time) bool {
	attr := pkt.FindAttr(radius.AttrEventTimestamp)
	if len(attr) != 4 {
		return false
	}
	ts := time.Unix(int64(binary.BigEndian.Uint32(attr)), 0)
	return !ts.Before(now.Add(-coaReplayWindow)) && !ts.After(now.Add(coaReplayWindow))
}

func nakCode(code uint8) uint8 {
	if code == radius.CodeDisconnectRequest {
		return radius.CodeDisconnectNAK
	}
	return radius.CodeCoANAK
}

func (cl *coaListener) cachedReplay(source net.IP, pkt *radius.Packet) []byte {
	key := replayKey(source, pkt)
	now := time.Now()
	cl.replayMu.Lock()
	defer cl.replayMu.Unlock()
	cl.pruneReplayLocked(now)
	entry, ok := cl.replay[key]
	if !ok {
		return nil
	}
	resp := make([]byte, len(entry.response))
	copy(resp, entry.response)
	return resp
}

func (cl *coaListener) rememberReplay(source net.IP, pkt *radius.Packet, response []byte) {
	key := replayKey(source, pkt)
	now := time.Now()
	resp := make([]byte, len(response))
	copy(resp, response)
	cl.replayMu.Lock()
	defer cl.replayMu.Unlock()
	cl.pruneReplayLocked(now)
	cl.replay[key] = coaReplayEntry{seen: now, response: resp}
}

func (cl *coaListener) pruneReplayLocked(now time.Time) {
	for key, entry := range cl.replay {
		if now.Sub(entry.seen) > coaReplayWindow {
			delete(cl.replay, key)
		}
	}
}

func replayKey(source net.IP, pkt *radius.Packet) coaReplayKey {
	return coaReplayKey{
		source:        source.String(),
		code:          pkt.Code,
		identifier:    pkt.Identifier,
		authenticator: pkt.Authenticator,
	}
}

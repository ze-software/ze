// Design: docs/research/l2tpv2-ze-integration.md -- CoA/DM listener
// Related: register.go -- plugin lifecycle starts/stops the listener
// Related: config.go -- CoAPort configuration

package l2tpauthradius

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/l2tp"
	l2tpevents "codeberg.org/thomas-mangin/ze/internal/component/l2tp/events"
	"codeberg.org/thomas-mangin/ze/internal/component/radius"
	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
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
}

func newCoAListener(port int, secrets map[string][]byte, defaultSecret []byte, bus ze.EventBus, allowedSources []net.IP) (*coaListener, error) {
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf(":%d", port))
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

	switch pkt.Code {
	case radius.CodeCoARequest:
		cl.handleCoA(pkt, from)
	case radius.CodeDisconnectRequest:
		cl.handleDisconnect(pkt, from)
	default:
		logger().Warn("coa: unexpected code", "code", pkt.Code, "from", from)
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
	sid, ok := cl.findSession(pkt)
	if !ok {
		cl.sendResponse(from, pkt, radius.CodeCoANAK, radius.ErrorCauseSessionNotFound)
		return
	}

	downloadRate := extractRate(pkt)
	if downloadRate == 0 {
		cl.sendResponse(from, pkt, radius.CodeCoANAK, radius.ErrorCauseUnsupportedAttribute)
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
		if _, emitErr := l2tpevents.SessionRateChange.Emit(cl.bus, &l2tpevents.SessionRateChangePayload{
			TunnelID:     sess.TunnelLocalTID,
			SessionID:    sid,
			DownloadRate: downloadRate,
			UploadRate:   downloadRate,
		}); emitErr != nil {
			logger().Warn("coa: emit rate-change failed", "error", emitErr)
		}
	}

	cl.sendResponse(from, pkt, radius.CodeCoAACK, 0)
	logger().Info("coa: accepted CoA",
		"session", sid, "rate-bps", downloadRate, "from", from)
}

func (cl *coaListener) handleDisconnect(pkt *radius.Packet, from *net.UDPAddr) {
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
				prefix := fmt.Sprintf("%d-%d-", snap.Tunnels[i].LocalTID, snap.Tunnels[i].Sessions[j].LocalSID)
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
// Checks Filter-Id for a rate string (e.g. "10mbit").
func extractRate(pkt *radius.Packet) uint64 {
	if filterID := pkt.FindAttr(radius.AttrFilterID); filterID != nil {
		if rate, err := traffic.ParseRateBps(string(filterID)); err == nil {
			return rate
		}
	}
	return 0
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

	if _, err := cl.conn.WriteToUDP(wireBuf[:n], to); err != nil {
		logger().Warn("coa: send response failed", "error", err)
	}
}

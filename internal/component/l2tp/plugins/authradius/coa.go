// Design: docs/research/l2tpv2-ze-integration.md -- CoA/DM listener
// RFC: rfc/short/rfc5176.md -- Section 2.3 Request Authenticator, Section 3.4 Message-Authenticator, Section 3.5 Error-Cause
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
	conn     *net.UDPConn
	cfg      coaListenerConfig
	done     chan struct{}
	replayMu sync.Mutex
	replay   map[coaReplayKey]coaReplayEntry
}

// coaListenerConfig is what an operator's configuration decides about the
// listener. It is passed as one value so each call site names what it sets.
type coaListenerConfig struct {
	Port           int
	Secrets        map[string][]byte // source IP -> shared secret
	DefaultSecret  []byte
	Bus            ze.EventBus
	AllowedSources []net.IP

	// RequireMessageAuthenticator is the `require-message-authenticator` leaf
	// (yang/ze-l2tp-auth-radius-conf.yang). False is the RFC 5176 Section 3.4
	// behavior: the attribute is optional and its absence is not a reason to
	// discard.
	RequireMessageAuthenticator bool
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

func newCoAListener(cfg coaListenerConfig) (*coaListener, error) {
	var bAddr textbuf.Buffer
	addr, err := net.ResolveUDPAddr("udp4", bAddr.Reset().Byte(':').Int(int64(cfg.Port)).String())
	if err != nil {
		return nil, fmt.Errorf("coa: resolve: %w", err)
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return nil, fmt.Errorf("coa: listen: %w", err)
	}
	cl := &coaListener{
		conn:   conn,
		cfg:    cfg,
		done:   make(chan struct{}),
		replay: make(map[coaReplayKey]coaReplayEntry),
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

	// RFC 5176 Section 2.3: "The Dynamic Authorization Server MUST use the source
	// IP address of the RADIUS UDP packet to decide which shared secret to use,
	// so that requests can be proxied." Ze narrows that to the configured Dynamic
	// Authorization Clients: a source with no secret has no way to authenticate.
	if !cl.isAllowedSource(from.IP) {
		logger().Debug("coa: source not in allowed list, discarding", "from", from)
		return
	}

	// RFC 5176 Section 2.3: "The Request Authenticator is calculated the same way
	// as for an Accounting-Request, specified in [RFC2866]." Verify it before any
	// attribute is read.
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
	// RFC 5176 Section 3.4: "The Message-Authenticator Attribute MAY be used to
	// authenticate and integrity-protect CoA-Request, CoA-ACK, CoA-NAK,
	// Disconnect-Request, Disconnect-ACK, and Disconnect-NAK packets in order to
	// prevent spoofing." A MAY, so a request that carries none is answered, and
	// only the `require-message-authenticator` leaf makes its absence a discard.
	hasMessageAuthenticator := pkt.FindAttr(radius.AttrMessageAuthenticator) != nil
	if !hasMessageAuthenticator && cl.cfg.RequireMessageAuthenticator {
		logger().Debug("coa: missing message-authenticator, discarding", "from", from)
		return
	}

	// RFC 5176 Section 3.4: "A Dynamic Authorization Server receiving a
	// CoA-Request or Disconnect-Request with a Message-Authenticator Attribute
	// present MUST calculate the correct value of the Message-Authenticator and
	// silently discard the packet if it does not match the value sent."
	// VerifyCoAMessageAuthenticator answers false for an ABSENT attribute, which
	// is right for a guard and wrong for this question, so presence is read here
	// rather than inferred from the verdict.
	if hasMessageAuthenticator && !radius.VerifyCoAMessageAuthenticator(data, secret) {
		logger().Debug("coa: invalid message-authenticator, discarding", "from", from)
		return
	}

	// RFC 5176 Section 2.3: "A Dynamic Authorization Server implementing this
	// specification MUST be capable of detecting a duplicate request if it has
	// the same source IP address, source UDP port, and Identifier within a short
	// span of time." A duplicate is answered with the response the first copy
	// earned, so a retransmission never applies a change twice.
	if cached := cl.cachedReplay(from.IP, pkt); cached != nil {
		cl.sendRawResponse(from, cached)
		return
	}

	switch eventTimestampState(pkt, time.Now()) {
	case eventTimestampStale:
		// RFC 5176 Section 6.3: "If the Event-Timestamp Attribute is not current,
		// then the packet MUST be silently discarded." A NAK here would answer a
		// replayed packet and tell its sender that the secret is right.
		logger().Debug("coa: event-timestamp outside the replay window, discarding", "from", from)
		return
	case eventTimestampAbsent:
		// RFC 5176 Section 6.3: "Implementations SHOULD be configurable to discard
		// CoA-Request or Disconnect-Request packets not containing an
		// Event-Timestamp Attribute." Without it there is no replay protection, so
		// the request is refused rather than honored.
		cl.sendResponse(from, pkt, nakCode(pkt.Code), radius.ErrorCauseInvalidRequest)
		return
	case eventTimestampCurrent:
	}

	// RFC 5176 Section 3.2: "A NAS MUST respond to a CoA-Request including a
	// Service-Type Attribute with value \"Authorize Only\" with a CoA-NAK; a
	// CoA-ACK MUST NOT be sent. If the NAS does not support a Service-Type value
	// of \"Authorize Only\", then it MUST respond with a CoA-NAK; an Error-Cause
	// Attribute with a value of 405 (Unsupported Service) SHOULD be included."
	// This NAS supports no Service-Type value in a CoA-Request, so the
	// unsupported branch takes every one of them.
	if pkt.Code == radius.CodeCoARequest && pkt.FindAttr(radius.AttrServiceType) != nil {
		logger().Debug("coa: service-type is not supported in a CoA-Request", "from", from)
		cl.sendResponse(from, pkt, radius.CodeCoANAK, radius.ErrorCauseUnsupportedService)
		return
	}

	// RFC 5176 Section 2.3: "In CoA-Request and Disconnect-Request packets, all
	// attributes MUST be treated as mandatory," and a NAS "MUST respond to a
	// CoA-Request containing one or more unsupported attributes or Attribute
	// values with a CoA-NAK". Section 3 adds that "A Disconnect-Request MUST
	// contain only NAS and session identification attributes."
	if attrType, found := unsupportedAttr(pkt); found {
		logger().Debug("coa: unsupported attribute", "attribute", attrType, "from", from)
		cl.sendResponse(from, pkt, nakCode(pkt.Code), radius.ErrorCauseUnsupportedAttribute)
		return
	}

	switch pkt.Code {
	case radius.CodeCoARequest:
		cl.handleCoA(pkt, from)
	case radius.CodeDisconnectRequest:
		cl.handleDisconnect(pkt, from)
	}
}

// coaSupportedAttrs are the attribute types this NAS accepts in a CoA-Request:
// the NAS and session identification attributes of RFC 5176 Section 3, the
// transport attributes its Section 3.6 table allows in every packet, and the two
// authorization-change attributes this NAS implements (Filter-Id and
// Vendor-Specific, which carry the rate and the CoS profile).
var coaSupportedAttrs = map[uint8]bool{
	radius.AttrUserName:             true,
	radius.AttrNASIPAddress:         true,
	radius.AttrNASPort:              true,
	radius.AttrServiceType:          true,
	radius.AttrFramedIPAddress:      true,
	radius.AttrFilterID:             true,
	radius.AttrState:                true,
	radius.AttrVendorSpecific:       true,
	radius.AttrCalledStationID:      true,
	radius.AttrCallingStationID:     true,
	radius.AttrNASIdentifier:        true,
	radius.AttrProxyState:           true,
	radius.AttrAcctSessionID:        true,
	radius.AttrAcctMultiSessionID:   true,
	radius.AttrEventTimestamp:       true,
	radius.AttrMessageAuthenticator: true,
	radius.AttrNASPortID:            true,
	radius.AttrChargeableUserID:     true,
	radius.AttrNASIPv6Address:       true,
	radius.AttrFramedInterfaceID:    true,
	radius.AttrFramedIPv6Prefix:     true,
}

// disconnectSupportedAttrs are the attribute types this NAS accepts in a
// Disconnect-Request. RFC 5176 Section 3: "A Disconnect-Request MUST contain
// only NAS and session identification attributes." The rest of this set is what
// the Section 3.6 Disconnect table allows in a Request: Reply-Message, Class,
// Acct-Terminate-Cause, Proxy-State, Event-Timestamp and Message-Authenticator.
// Service-Type and State are 0 in that table, so neither is here.
//
// EAP-Message is in that table and is NOT here. Ze offers no EAP service, and
// RFC 2865 Section 1.1 says "A NAS that does not implement a given service MUST
// NOT implement the RADIUS attributes for that service", so dict.go declares no
// EAP-Message constant. Section 2.3 gives the answer for an attribute the NAS
// does not support: a Disconnect-NAK with Error-Cause 401.
var disconnectSupportedAttrs = map[uint8]bool{
	radius.AttrUserName:             true,
	radius.AttrNASIPAddress:         true,
	radius.AttrNASPort:              true,
	radius.AttrReplyMessage:         true,
	radius.AttrClass:                true,
	radius.AttrVendorSpecific:       true,
	radius.AttrCalledStationID:      true,
	radius.AttrCallingStationID:     true,
	radius.AttrNASIdentifier:        true,
	radius.AttrProxyState:           true,
	radius.AttrAcctSessionID:        true,
	radius.AttrAcctTerminateCause:   true,
	radius.AttrAcctMultiSessionID:   true,
	radius.AttrEventTimestamp:       true,
	radius.AttrMessageAuthenticator: true,
	radius.AttrNASPortID:            true,
	radius.AttrChargeableUserID:     true,
	radius.AttrNASIPv6Address:       true,
}

// unsupportedAttr answers the first attribute type the request carries that this
// NAS does not support, and whether it found one. The caller answers a NAK,
// because RFC 5176 Section 2.3 makes every attribute of a CoA-Request or
// Disconnect-Request mandatory: an attribute silently ignored is an
// authorization change the client believes it made.
func unsupportedAttr(pkt *radius.Packet) (uint8, bool) {
	supported := coaSupportedAttrs
	if pkt.Code == radius.CodeDisconnectRequest {
		supported = disconnectSupportedAttrs
	}
	for i := range pkt.Attrs {
		if !supported[pkt.Attrs[i].Type] {
			return pkt.Attrs[i].Type, true
		}
	}
	return 0, false
}

// secretForSource returns the shared secret for the given source IP.
// Falls back to the default secret if no per-source secret is configured.
func (cl *coaListener) secretForSource(ip net.IP) []byte {
	if s, ok := cl.cfg.Secrets[ip.String()]; ok {
		return s
	}
	return cl.cfg.DefaultSecret
}

// isAllowedSource answers whether a packet's source is a configured Dynamic
// Authorization Client.
//
// RFC 5176 Section 6.1: "A Dynamic Authorization Server MUST silently discard
// Disconnect-Request or CoA-Request packets from untrusted sources."
//
// An EMPTY list answers no, and that is the whole of this function's care. The
// list is `serverIPs(cfg.Servers)` and the listener starts only when
// `len(cfg.Servers) > 0` (registerCallbacks, register.go), so empty never means
// "the operator configured nothing". It means every configured server is a
// hostname and none of them resolved: `resolveCoAHost` logs the failure and
// returns nil. Reading that empty list as "allow everyone" turned a DNS outage
// into an open CoA port, where `secretForSource` then hands `DefaultSecret` to
// any source that asks. A zero value that reads as a valid answer is the defect
// class in plan/journal/zero-value-as-valid-answer.md, and a guard whose empty
// input means yes is the one ai/rules/evidence.md names.
func (cl *coaListener) isAllowedSource(ip net.IP) bool {
	for _, allowed := range cl.cfg.AllowedSources {
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
		cl.applySubscriberCoA(pkt, from, &subSess, downloadRate, cosProfile)
		return
	}

	// Fallback: L2TP-only lookup.
	sid, found := cl.oneSession(pkt, from, radius.CodeCoANAK)
	if !found {
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

	// RFC 5176 Section 2.3: "If one or more authorization changes specified in a
	// CoA-Request cannot be carried out, the NAS MUST send a CoA-NAK." The event
	// bus is the only route from here to the shaper, so its absence is a NAS-side
	// failure. It is answered before the first event leaves, so no partial change
	// sits behind a NAK.
	if cl.cfg.Bus == nil {
		logger().Warn("coa: no event bus, the authorization change cannot be carried out", "from", from)
		cl.sendResponse(from, pkt, radius.CodeCoANAK, radius.ErrorCauseResourcesUnavailable)
		return
	}

	// A CoS profile is applied to an access interface, which an L2TP-only session
	// does not carry. RFC 5176 Section 2.3 owes a CoA-NAK here, not an ACK
	// reporting a change this NAS did not make.
	if cosProfile != "" {
		logger().Warn("coa: CoS change asked for an L2TP-only session with no access interface",
			"session", sid, "profile", cosProfile)
		cl.sendResponse(from, pkt, radius.CodeCoANAK, radius.ErrorCauseResourcesUnavailable)
		return
	}

	if _, emitErr := l2tpevents.SessionRateChange.Emit(cl.cfg.Bus, &l2tpevents.SessionRateChangePayload{
		TunnelID:     sess.TunnelLocalTID,
		SessionID:    sid,
		DownloadRate: downloadRate,
		UploadRate:   downloadRate,
	}); emitErr != nil {
		logger().Warn("coa: emit rate-change failed", "error", emitErr)
		cl.sendResponse(from, pkt, radius.CodeCoANAK, radius.ErrorCauseResourcesUnavailable)
		return
	}

	cl.sendResponse(from, pkt, radius.CodeCoAACK, 0)
	logger().Info("coa: accepted CoA",
		"session", sid, "rate-bps", downloadRate, "from", from)
}

// applySubscriberCoA carries out a CoA-Request against a subscriber-registry
// session.
//
// RFC 5176 Section 2.3: "State changes resulting from a CoA-Request MUST be
// atomic: if the CoA-Request is successful for all matching sessions, the NAS
// MUST send a CoA-ACK in reply, and all requested authorization changes MUST be
// made." The subscriber registry answers with one session, so "all matching
// sessions" is that session and the ACK reports a change that was made.
func (cl *coaListener) applySubscriberCoA(pkt *radius.Packet, from *net.UDPAddr, sub *subscriber.Session, downloadRate uint64, cosProfile string) {
	if cl.cfg.Bus == nil {
		logger().Warn("coa: no event bus, the authorization change cannot be carried out", "from", from)
		cl.sendResponse(from, pkt, radius.CodeCoANAK, radius.ErrorCauseResourcesUnavailable)
		return
	}
	if downloadRate > 0 {
		if _, err := subevents.SessionRateChange.Emit(cl.cfg.Bus, &subevents.SessionRateChangePayload{
			SessionID:    sub.ID,
			DownloadRate: downloadRate,
			UploadRate:   downloadRate,
		}); err != nil {
			logger().Warn("coa: emit subscriber rate-change failed", "error", err)
			cl.sendResponse(from, pkt, radius.CodeCoANAK, radius.ErrorCauseResourcesUnavailable)
			return
		}
		// An L2TP session also needs the L2TP-specific event, which the shaper
		// plugin consumes.
		if sub.AccessType == subscriber.AccessL2TP {
			if _, err := l2tpevents.SessionRateChange.Emit(cl.cfg.Bus, &l2tpevents.SessionRateChangePayload{
				TunnelID:     sub.TunnelID,
				SessionID:    sub.SessionID,
				DownloadRate: downloadRate,
				UploadRate:   downloadRate,
			}); err != nil {
				logger().Warn("coa: emit l2tp rate-change failed", "error", err)
				cl.sendResponse(from, pkt, radius.CodeCoANAK, radius.ErrorCauseResourcesUnavailable)
				return
			}
		}
	}
	if cosProfile != "" {
		if _, err := l2tpevents.SessionCoSChange.Emit(cl.cfg.Bus, &l2tpevents.SessionCoSChangePayload{
			TunnelID:        sub.TunnelID,
			SessionID:       sub.SessionID,
			AccessInterface: sub.AccessInterface,
			ProfileName:     cosProfile,
		}); err != nil {
			logger().Warn("coa: emit cos-change failed", "error", err)
			cl.sendResponse(from, pkt, radius.CodeCoANAK, radius.ErrorCauseResourcesUnavailable)
			return
		}
	}
	cl.sendResponse(from, pkt, radius.CodeCoAACK, 0)
	logger().Info("coa: accepted CoA",
		"subscriber", sub.ID, "rate-bps", downloadRate,
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

	sid, found := cl.oneSession(pkt, from, radius.CodeDisconnectNAK)
	if !found {
		return
	}

	svc := l2tp.LookupService()
	if svc == nil {
		cl.sendResponse(from, pkt, radius.CodeDisconnectNAK, radius.ErrorCauseSessionNotFound)
		return
	}

	// RFC 5176 Section 2.3: "a NAS MUST send a Disconnect-NAK in reply if any of
	// the matching sessions cannot be successfully terminated."
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

// oneSession answers the single session the request identifies, having already
// sent the NAK when it identifies none or more than one.
//
// RFC 5176 Section 3: "If all NAS identification attributes match, and more than
// one session matches all of the session identification attributes, then a
// CoA-Request or Disconnect-Request MUST apply to all matching sessions."
// Section 2.3 gives the other branch, which is the one this NAS takes: "A NAS
// that does not support dynamic authorization changes applying to multiple
// sessions MUST send a CoA-NAK or Disconnect-NAK in reply; an Error-Cause
// Attribute with value 508 (Multiple Session Selection Unsupported) SHOULD be
// included.".
func (cl *coaListener) oneSession(pkt *radius.Packet, from *net.UDPAddr, nak uint8) (uint16, bool) {
	sessions := cl.findSessions(pkt)
	if len(sessions) == 0 {
		cl.sendResponse(from, pkt, nak, radius.ErrorCauseSessionNotFound)
		return 0, false
	}
	if len(sessions) > 1 {
		logger().Info("coa: identification attributes match more than one session",
			"sessions", len(sessions), "from", from)
		cl.sendResponse(from, pkt, nak, radius.ErrorCauseMultiSessionUnsupported)
		return 0, false
	}
	return sessions[0], true
}

// findSessions answers every L2TP session the request's identification
// attributes match.
//
// RFC 5176 Section 3: "The combination of NAS and session identification
// attributes included in a CoA-Request or Disconnect-Request packet MUST match
// at least one session in order for a Request to be successful; otherwise a
// Disconnect-NAK or CoA-NAK MUST be sent." It is a combination, so each
// attribute the listener evaluates narrows the set rather than opening a second
// route to a session: Acct-Session-Id, User-Name and NAS-Port must all agree
// with the session when the request carries them. A request carrying none of the
// three identifies nothing and matches nothing.
//
// The accounting plugin writes an Acct-Session-Id as "tunnelID-sessionID-seqNum"
// (acct.go genSessionID), so the match is on the "tunnelID-sessionID-" prefix
// and the opaque sequence number is ignored.
func (cl *coaListener) findSessions(pkt *radius.Packet) []uint16 {
	svc := l2tp.LookupService()
	if svc == nil {
		return nil
	}
	acctSessID := pkt.FindAttr(radius.AttrAcctSessionID)
	userName := pkt.FindAttr(radius.AttrUserName)
	nasPortAttr := pkt.FindAttr(radius.AttrNASPort)
	if acctSessID == nil && userName == nil && len(nasPortAttr) != 4 {
		return nil
	}

	var out []uint16
	snap := svc.Snapshot()
	for i := range snap.Tunnels {
		for j := range snap.Tunnels[i].Sessions {
			sess := &snap.Tunnels[i].Sessions[j]
			if acctSessID != nil {
				var pb textbuf.Buffer
				prefix := pb.Reset().Int(int64(snap.Tunnels[i].LocalTID)).Byte('-').Int(int64(sess.LocalSID)).Byte('-').String()
				if !strings.HasPrefix(string(acctSessID), prefix) {
					continue
				}
			}
			if userName != nil && sess.Username != string(userName) {
				continue
			}
			if len(nasPortAttr) == 4 && uint32(sess.LocalSID) != binary.BigEndian.Uint32(nasPortAttr) {
				continue
			}
			out = append(out, sess.LocalSID)
		}
	}
	return out
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

	// RFC 5176 Section 3.1: "If there are any Proxy-State attributes in a
	// Disconnect-Request or CoA-Request received from the Dynamic Authorization
	// Client, the Dynamic Authorization Server MUST include those Proxy-State
	// attributes in its response to the Dynamic Authorization Client," and the NAS
	// "MUST treat any Proxy-State attributes already in the packet as opaque
	// data".
	//
	// RFC 5176 Section 3.3: the State Attribute "MUST be sent unmodified from the
	// NAS to the Dynamic Authorization Client in a subsequent ACK or NAK packet",
	// and "the Dynamic Authorization Server MUST NOT interpret the Attribute
	// locally".
	//
	// Both are copied value for value and in the order they arrived; neither is
	// read.
	for i := range req.Attrs {
		if req.Attrs[i].Type == radius.AttrProxyState || req.Attrs[i].Type == radius.AttrState {
			resp.Attrs = append(resp.Attrs, req.Attrs[i])
		}
	}

	wireBuf := radius.Bufs.Get()
	defer radius.Bufs.Put(wireBuf)

	n, err := resp.EncodeTo(wireBuf, 0)
	if err != nil {
		logger().Warn("coa: encode response failed", "error", err)
		return
	}

	// RFC 5176 Section 2.3: the Response Authenticator "contains a one-way MD5
	// hash calculated over a stream of octets consisting of the Code,
	// Identifier, Length, the Request Authenticator field from the packet being
	// replied to, and the response attributes if any, followed by the shared
	// secret".
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

// eventTimestampKind reports what the request's Event-Timestamp Attribute says
// about its age. Zero is not a valid state, so a caller that forgets a branch
// does not read a stale packet as a current one.
type eventTimestampKind uint8

const (
	eventTimestampAbsent eventTimestampKind = iota + 1
	eventTimestampStale
	eventTimestampCurrent
)

// eventTimestampState grades the request's Event-Timestamp Attribute.
//
// RFC 5176 Section 6.3: "When the Event-Timestamp Attribute is present, both the
// Dynamic Authorization Server and the Dynamic Authorization Client MUST check
// that the Event-Timestamp Attribute is current within an acceptable time
// window." The window is coaReplayWindow, because Section 6.3 also says "The
// time window used for duplicate detection MUST be the same as the window used
// to detect a stale Event-Timestamp Attribute.".
func eventTimestampState(pkt *radius.Packet, now time.Time) eventTimestampKind {
	attr := pkt.FindAttr(radius.AttrEventTimestamp)
	if len(attr) != 4 {
		return eventTimestampAbsent
	}
	ts := time.Unix(int64(binary.BigEndian.Uint32(attr)), 0)
	if ts.Before(now.Add(-coaReplayWindow)) || ts.After(now.Add(coaReplayWindow)) {
		return eventTimestampStale
	}
	return eventTimestampCurrent
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

// Design: docs/research/l2tpv2-ze-integration.md -- PPP auth-phase dispatch + shared helper
// Related: auth_events.go -- AuthMethod enum + EventAuthRequest/Success/Failure
// Related: session_run.go -- runAuthPhase switch driven by these helpers
// Related: pap.go -- runPAPAuthPhase consumer of awaitAuthDecision
// Related: chap.go -- runCHAPAuthPhase consumer of awaitAuthDecision + waitCHAPResponse
// Related: mschapv2.go -- runMSCHAPv2AuthPhase consumer of awaitAuthDecision + waitMSCHAPv2Response
// RFC: rfc/short/rfc1334.md

package ppp

import (
	"encoding/binary"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// adjustAuthOnNakOrReject applies a validated reply before the FSM sends its
// next request. The caller MUST validate correlation and option contents first.
// RFC 1661 Section 5.4: "Reception of a valid Configure-Reject indicates that
// when a new Configure-Request is sent, it MUST NOT include any of the
// Configuration Options listed in the Configure-Reject."
func (s *pppSession) adjustAuthOnNakOrReject(pkt LCPPacket) {
	opts, err := ParseLCPOptions(pkt.Data)
	if err != nil {
		return
	}
	data, ok := lookupOption(opts, LCPOptAuthProto)
	if !ok {
		return
	}

	if pkt.Code == LCPConfigureReject {
		s.lcpRejectedAuth = true
		s.configuredAuthMethod = AuthMethodNone
		return
	}
	if s.lcpRejectedAuth {
		return
	}
	suggestion := AuthMethodNone
	if len(data) >= 2 {
		suggestion = authMethodFromAuthProto(binary.BigEndian.Uint16(data[:2]), data[2:])
	}
	s.configuredAuthMethod = selectAuthFallback(suggestion, s.authFallbackOrder, s.authMethodsTried)
}

// PPP Auth-Protocol values (RFC 1661 §6.2 table 3) and CHAP Algorithm
// codes (RFC 1994 §3 + IANA PPP Authentication Algorithms registry).
const (
	authProtoPAP      uint16 = 0xC023 // RFC 1334
	authProtoCHAP     uint16 = 0xC223 // RFC 1994
	chapAlgorithmMD5  uint8  = 0x05   // RFC 1994 §3
	chapAlgorithmMSv2 uint8  = 0x81   // RFC 2759 §3
)

// authMethodFromAuthProto decodes an on-wire Auth-Protocol value plus
// any method-specific Algorithm bytes into an AuthMethod. For PAP
// algorithm is expected empty; trailing bytes beyond the canonical
// length are silently ignored (peers that embed non-standard trailers
// still match the intended method). For CHAP algorithm[0] selects
// the variant (0x05 MD5, 0x81 MS-CHAPv2); additional bytes are
// ignored. Unknown Auth-Protocol values or unknown CHAP Algorithm
// identifiers return AuthMethodNone. Callers MUST distinguish absent auth
// from an unknown wire value: a Nak advances policy, and proxy admission
// refuses an unknown method rather than treating it as no authentication.
//
// Caller splits an LCP Auth-Protocol option Data field as:
//
//	proto := binary.BigEndian.Uint16(data[:2])
//	algorithm := data[2:]
//
// which avoids the allocation a [proto|algorithm] reconcatenation
// would incur.
func authMethodFromAuthProto(proto uint16, algorithm []byte) AuthMethod {
	switch proto {
	case authProtoPAP:
		return AuthMethodPAP
	case authProtoCHAP:
		if len(algorithm) == 0 {
			return AuthMethodNone
		}
		switch algorithm[0] {
		case chapAlgorithmMD5:
			return AuthMethodCHAPMD5
		case chapAlgorithmMSv2:
			return AuthMethodMSCHAPv2
		}
	}
	return AuthMethodNone
}

// initialAuthMethod returns the AuthMethod ze offers in its first LCP
// Configure-Request, given the configured method and the session's fallback
// order. A configured PAP is replaced by the first CHAP variant the order
// accepts, so the Nak sequence offers every permitted CHAP variant before
// PAP. An order without a CHAP variant leaves PAP as configured: the session
// then holds no stronger method to offer.
func initialAuthMethod(configured AuthMethod, order []AuthMethod) AuthMethod {
	if configured != AuthMethodPAP {
		return configured
	}
	// RFC 1334 Section 2: "Any implementations which include a stronger
	// authentication method (such as CHAP, described below) MUST offer to
	// negotiate that method prior to PAP."
	for _, m := range order {
		if m == AuthMethodCHAPMD5 {
			return m
		}
		if m == AuthMethodMSCHAPv2 {
			return m
		}
	}
	return configured
}

// selectAuthFallback chooses an untried permitted method. tried is a fixed
// bitset of the three wire methods, so retries allocate no per-session storage.
// RFC 1661 Section 6.2: "If that protocol is Configure-Nak'd, then the
// implementation SHOULD attempt the next most desirable protocol in the next
// Configure-Request."
// RFC 1334 Section 2: "Any implementations which include a stronger
// authentication method (such as CHAP, described below) MUST offer to
// negotiate that method prior to PAP."
func selectAuthFallback(suggestion AuthMethod, order []AuthMethod, tried uint8) AuthMethod {
	var nextCHAP, nextPAP AuthMethod
	suggestionAllowed := false
	for _, method := range order {
		if method == AuthMethodNone {
			continue
		}
		if tried&(1<<method) != 0 {
			continue
		}
		if method == suggestion {
			suggestionAllowed = true
		}
		if method == AuthMethodPAP {
			nextPAP = method
			continue
		}
		if nextCHAP == AuthMethodNone {
			nextCHAP = method
		}
	}
	if suggestionAllowed {
		if suggestion != AuthMethodPAP {
			return suggestion
		}
		if nextCHAP == AuthMethodNone {
			return suggestion
		}
	}
	if nextCHAP != AuthMethodNone {
		return nextCHAP
	}
	return nextPAP
}

// resetAuthNegotiation starts a new LCP lifetime, not a retransmission or Nak
// round. The packet dispatcher MUST call it only after validating the packet
// that leaves Opened, or when Stopped begins negotiation again.
// RFC 1661 Section 4.3: "The implementation MUST be prepared to immediately
// renegotiate the Configuration Options."
//
// A peer's Nak and Reject answer the negotiation they arrived in, so every value
// they changed returns to its configured form here: the Auth-Protocol offer, the
// local MRU (zero offers the admitted maxMRU, see sendConfigureRequest) and the
// offers of the MRU and Magic-Number options. A new lifetime is not proxied either.
func (s *pppSession) resetAuthNegotiation() {
	s.authMethodsTried = 0
	s.lcpRejectedAuth = false
	s.configuredAuthMethod = s.initialAuthMethod
	s.lastLCPRequestSent = false
	s.lastLCPReplyReceived = false
	s.localMRU = 0
	s.lcpRejectedMRU = false
	s.lcpRejectedMagic = false
	s.lcpProxied = false
}

// refuseAuthMethod preserves the normal Close/Restart/Terminate-Ack lifecycle.
// RFC 1661 Section 3.5: "If authentication fails, the authenticator SHOULD
// proceed instead to the Link Termination phase."
func (s *pppSession) refuseAuthMethod(cur LCPState, pkt LCPPacket) bool {
	var reason textbuf.Buffer
	suggestion := AuthMethodNone
	reason.Str("auth-method-refused: peer ")
	if pkt.Code == LCPConfigureReject {
		reason.Str("rejected Auth-Protocol")
	} else {
		reason.Str("suggested ")
		opts, err := ParseLCPOptions(pkt.Data)
		if err == nil {
			data, found := lookupOption(opts, LCPOptAuthProto)
			if found {
				reason.Str("Auth-Protocol 0x").Hex(data)
				if len(data) >= 2 {
					suggestion = authMethodFromAuthProto(binary.BigEndian.Uint16(data[:2]), data[2:])
					if suggestion != AuthMethodNone {
						reason.Str(" (").Str(suggestion.String()).Str(")")
					}
				}
			}
		}
	}
	reason.Str("; allowed [")
	for i, method := range s.authFallbackOrder {
		if i != 0 {
			reason.Str(" ")
		}
		reason.Str(method.String())
	}
	reason.Str("]; configure the peer to accept an allowed method.")
	if suggestion != AuthMethodNone {
		commandPath := "set l2tp authentication allow "
		if s.pppoe {
			commandPath = "set pppoe authentication-allow "
		}
		reason.Str(" Only if this method is intentionally permitted, use these editor commands: ")
		for _, allowed := range s.authFallbackOrder {
			if allowed != suggestion {
				reason.Byte('`').Str(commandPath).Str(allowed.String()).Str("`, then ")
			}
		}
		reason.Byte('`').Str(commandPath).Str(suggestion.String()).Str("`.")
	}
	s.authRefusalReason = reason.String()
	s.logger.Warn("ppp: authentication policy refused peer", "reason", s.authRefusalReason)
	countAuthMethodRefusal()
	// RFC 1661 Section 4.1: drive Close through the shared FSM actions.
	return s.applyTransition(cur, LCPDoTransition(cur, LCPEventClose), pkt)
}

// acceptProxyPolicy admits only a known permitted method and the configured
// local receive MRU. Unknown Auth-Protocol is never equivalent to absent auth.
// RFC 2661 Section 4.4.5: "Note that the LNS policy may be to enter an additional round of LCP negotiation and/or authentication if the LAC is not trusted."
func (s *pppSession) acceptProxyPolicy(proxy proxyLCPResult) bool {
	// The decoder resolves an absent peer MRU to the PPP default. Every
	// explicit limit below the IP floor requires fresh negotiation.
	// RFC 791 Section 3.2: "Every internet module must be able to forward a datagram of 68
	// octets without further fragmentation."
	if proxy.MRU < MinIPv4MRU {
		return false
	}
	// RFC 791 Section 3.2: "Every internet module must be able to forward a datagram of 68
	// octets without further fragmentation."
	if proxy.LocalMRU < MinIPv4MRU {
		return false
	}
	// RFC 5072 Section 2: "PPP links supporting IPv6 MUST allow the
	// information field to be at least as large as the minimum link MTU
	// size required for IPv6 [2]."
	if !s.disableIPv6CP && proxy.LocalMRU < MinIPv6MRU {
		return false
	}
	if proxy.LocalMRU > s.maxMRU {
		return false
	}
	if !proxy.HasAuth {
		return !s.authRequired
	}
	method := authMethodFromAuthProto(proxy.AuthProto, proxy.AuthData)
	if method == AuthMethodNone {
		return false
	}
	for _, allowed := range s.authFallbackOrder {
		if method == allowed {
			return true
		}
	}
	return false
}

// authMethodToLCPOptions returns the AuthProto value and AuthData
// bytes to populate in LCPOptions when building a local CONFREQ.
// Returns (0, nil) for AuthMethodNone -- the caller omits the
// Auth-Protocol option entirely.
// An unnamed method is an internal programming error.
//
// The returned AuthData slice is freshly allocated so callers may
// retain the LCPOptions struct across requests without aliasing a
// shared constant.
func authMethodToLCPOptions(m AuthMethod) (uint16, []byte) {
	switch m {
	case AuthMethodNone:
		return 0, nil
	case AuthMethodPAP:
		return authProtoPAP, nil
	case AuthMethodCHAPMD5:
		return authProtoCHAP, []byte{chapAlgorithmMD5}
	case AuthMethodMSCHAPv2:
		return authProtoCHAP, []byte{chapAlgorithmMSv2}
	default:
		panic("BUG: unknown authentication method")
	}
}

// awaitAuthDecision emits req on the auth events channel, waits for
// the external handler's decision on authRespCh (bounded by
// s.authTimeout), and returns the outcome. Returns ok=false when the
// session must tear down: stopCh closed, sessStop closed, or the
// auth-timeout fired. On timeout the helper also calls s.fail (to
// emit EventSessionDown on the lifecycle channel) and sends
// EventAuthFailure{Reason:"timeout"} on the auth channel so callers
// need not repeat those side effects.
//
// label prefixes the fail-reason string ("pap", "chap", "chap-v2",
// or "" for AuthMethodNone). Kept as an explicit argument rather
// than derived from req.Method to preserve the exact wording the
// unit tests assert on.
//
// Caller MUST return false immediately on ok=false. On ok=true the
// caller writes the method-specific reply frame and, on reject,
// issues its own s.fail + EventAuthFailure{Reason: decision.message}
// (the helper only synthesizes the timeout failure because accept vs
// reject fan-out differs per method).
func (s *pppSession) awaitAuthDecision(req EventAuthRequest, label string) (authResponseMsg, bool) {
	s.proxyCapture.fill(&req)
	select {
	case s.authEventsOut <- req:
	case <-s.stopCh:
		return authResponseMsg{}, false
	case <-s.sessStop:
		return authResponseMsg{}, false
	}

	timeout := s.authTimeout
	if timeout <= 0 {
		timeout = defaultAuthTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case decision := <-s.authRespCh:
		if decision.accept {
			s.authenticatedUsername = req.Username
		}
		return decision, true
	case <-s.stopCh:
		return authResponseMsg{}, false
	case <-s.sessStop:
		return authResponseMsg{}, false
	case <-timer.C:
		var tb textbuf.Buffer
		if label != "" {
			tb.Str(label).Str(": ")
		}
		reason := tb.Str("auth timeout after ").Str(timeout.String()).String()
		s.fail(reason)
		s.sendAuthEvent(eventAuthFailure{
			TunnelID:  s.tunnelID,
			SessionID: s.sessionID,
			Reason:    "timeout",
		})
		return authResponseMsg{}, false
	}
}

// waitCHAPLike reads ProtoCHAP frames from framesIn until parse yields
// a Response whose returned Identifier matches wantID, or s.authTimeout
// expires. Shared by CHAP-MD5 and MS-CHAPv2 response waits.
//
// Implements AC-16 silent-discard (RFC 1994 §4.1): a Response with a
// mismatched Identifier is dropped with a debug log and the wait
// continues. A malformed CHAP frame (bad ParseFrame, parse error)
// tears the session down immediately -- those are peer-misbehavior
// conditions the Phase 9 retry loop does not paper over.
//
// Non-CHAP frames arriving during the wait are routed through
// s.handleFrame so LCP keepalive (Echo-Request/Reply) and
// Terminate-Request stay responsive during the initial auth window
// and, critically, during every Phase 9 periodic re-auth window. If
// the LCP state changes, the wait returns ok=false so the caller aborts auth
// and resumes the session loop for a nonterminal LCP transition.
//
// parse extracts the method-specific Response struct from a PPP
// payload and returns (result, identifier, err). The wait uses
// s.authTimeout as its bound so periodic re-auth never parks
// indefinitely on a silent peer. Fatal errors report session failure;
// a nonterminal LCP transition leaves teardown to the session loop.
//
// label prefixes all fail-reason strings ("chap" or "chap-v2").
func waitCHAPLike[T any](
	s *pppSession,
	label string,
	wantID uint8,
	parse func([]byte) (T, uint8, error),
) (T, bool) {
	var zero T
	lcpState := s.currentState()
	timeout := s.authTimeout
	if timeout <= 0 {
		timeout = defaultAuthTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case f, ok := <-s.framesIn:
			if !ok {
				var tb textbuf.Buffer
				s.fail(tb.Str(label).Str(": frames channel closed").String())
				return zero, false
			}
			proto, payload, _, perr := ParseFrame(f)
			if perr != nil {
				putFrameBuf(f)
				var tb textbuf.Buffer
				s.fail(tb.Str(label).Str(": malformed frame: ").Err(perr).String())
				return zero, false
			}
			if proto != ProtoCHAP {
				// LCP / other protocols during the auth wait: let
				// the main frame handler process them so keepalive
				// and Terminate-Request stay responsive (critical
				// for Phase 9 periodic re-auth, where the wait
				// window recurs for the session's lifetime).
				// handleFrame re-parses the frame -- negligible
				// cost; the simpler alternative of routing the
				// already-parsed payload would need a dedicated
				// LCP dispatch shim.
				term := s.handleFrame(f)
				putFrameBuf(f)
				if term || s.currentState() != lcpState {
					return zero, false
				}
				continue
			}
			resp, gotID, perr := parse(payload)
			putFrameBuf(f)
			if perr != nil {
				var tb textbuf.Buffer
				s.fail(tb.Str(label).Str(": malformed response: ").Err(perr).String())
				return zero, false
			}
			if gotID != wantID {
				s.logger.Debug(label+": discarding response with mismatched identifier",
					"got", gotID, "want", wantID)
				continue
			}
			return resp, true
		case f := <-s.unitFramesIn:
			// A periodic re-authentication runs with the channel connected,
			// so IPCP and IPv6CP arrive on the unit (session.go,
			// unitFramesIn). They are never CHAP, which stays on the channel.
			term := s.handleFrame(f)
			putFrameBuf(f)
			if term || s.currentState() != lcpState {
				return zero, false
			}
		case <-s.stopCh:
			return zero, false
		case <-s.sessStop:
			return zero, false
		case <-timer.C:
			var tb textbuf.Buffer
			reason := tb.Str(label).Str(": timeout awaiting Response for identifier ").Uint(uint64(wantID)).String()
			s.fail(reason)
			s.sendAuthEvent(eventAuthFailure{
				TunnelID:  s.tunnelID,
				SessionID: s.sessionID,
				Reason:    "timeout",
			})
			return zero, false
		}
	}
}

// waitCHAPResponse is the CHAP-MD5 specialization of waitCHAPLike.
// Silent-discards Responses whose Identifier differs from wantID per
// RFC 1994 §4.1 and AC-16.
func (s *pppSession) waitCHAPResponse(wantID uint8) (cHAPResponse, bool) {
	return waitCHAPLike(s, "chap", wantID,
		func(p []byte) (cHAPResponse, uint8, error) {
			r, err := parseCHAPResponse(p)
			return r, r.Identifier, err
		},
	)
}

// waitMSCHAPv2Response is the MS-CHAPv2 specialization of waitCHAPLike.
// ParseMSCHAPv2Response enforces the RFC 2759 §4 shape (49-byte
// Response, Reserved/Flags == 0) as hard errors; identifier mismatch
// is the only case that loops.
func (s *pppSession) waitMSCHAPv2Response(wantID uint8) (mSCHAPv2Response, bool) {
	return waitCHAPLike(s, "chap-v2", wantID,
		func(p []byte) (mSCHAPv2Response, uint8, error) {
			r, err := parseMSCHAPv2Response(p)
			return r, r.Identifier, err
		},
	)
}

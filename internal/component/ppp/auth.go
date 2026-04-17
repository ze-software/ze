// Design: docs/research/l2tpv2-ze-integration.md -- PPP auth-phase dispatch + shared helper
// Related: auth_events.go -- AuthMethod enum + EventAuthRequest/Success/Failure
// Related: session_run.go -- runAuthPhase switch driven by these helpers
// Related: pap.go -- runPAPAuthPhase consumer of awaitAuthDecision
// Related: chap.go -- runCHAPAuthPhase consumer of awaitAuthDecision + waitCHAPResponse
// Related: mschapv2.go -- runMSCHAPv2AuthPhase consumer of awaitAuthDecision + waitMSCHAPv2Response

package ppp

import (
	"encoding/binary"
	"strconv"
	"time"
)

// adjustAuthOnNakOrReject inspects a received LCP Configure-Nak or
// Configure-Reject packet for the Auth-Protocol option and, if present,
// updates s.configuredAuthMethod so the FSM's next Send-Configure-
// Request carries the adjusted value.
//
//   - Configure-Reject (RFC 1661 §5.4): the peer does not recognize
//     the option at all. Drop Auth-Protocol from future CONFREQs by
//     setting configuredAuthMethod to AuthMethodNone.
//   - Configure-Nak (RFC 1661 §5.3): the peer recognizes the option
//     but wants a different value. Decode the peer's suggested
//     Auth-Protocol, look it up in s.authFallbackOrder via
//     selectAuthFallback, and use the result (AuthMethodNone when the
//     peer's choice is not on our list).
//
// Goroutine-owned mutation: caller (handleLCPPacket) runs on the
// per-session goroutine, which is the sole writer to
// configuredAuthMethod outside the initial manager.spawnSession set.
// No lock is needed.
//
// Called before LCPDoTransition so the subsequent LCPActSCR sees the
// updated field when it rebuilds the CONFREQ option list.
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
		s.configuredAuthMethod = AuthMethodNone
		s.logger.Debug("ppp: peer Configure-Rejected Auth-Protocol; clearing method")
		return
	}

	// Configure-Nak: decode the peer's proposed Auth-Protocol.
	if len(data) < 2 {
		// Malformed suggestion; treat as "unacceptable" and drop auth.
		s.configuredAuthMethod = AuthMethodNone
		return
	}
	proto := binary.BigEndian.Uint16(data[:2])
	suggestion := authMethodFromAuthProto(proto, data[2:])
	next := selectAuthFallback(suggestion, s.authFallbackOrder)
	s.logger.Debug("ppp: peer Configure-Naked Auth-Protocol",
		"peer-proto", proto,
		"peer-method", suggestion.String(),
		"selected", next.String(),
	)
	s.configuredAuthMethod = next
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
// identifiers return AuthMethodNone so the caller falls back to the
// no-wire-auth phase.
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

// defaultAuthFallbackOrder returns the package-default preference list
// for Configure-Nak fallback. Matches the spec l2tp-6b-auth AC-13
// guidance: "prefer CHAP > MS-CHAPv2 > PAP". PAP is last because it
// sends cleartext passwords on the wire. A fresh slice is returned so
// callers can retain or mutate it without aliasing.
func defaultAuthFallbackOrder() []AuthMethod {
	return []AuthMethod{
		AuthMethodCHAPMD5,
		AuthMethodMSCHAPv2,
		AuthMethodPAP,
	}
}

// selectAuthFallback chooses the AuthMethod ze will advertise in its
// next LCP Configure-Request after the peer Naks the prior one.
//
// RFC 1661 §5.3: "The options field is filled with the unacceptable
// Configuration Options from the Configure-Request. All Configuration
// Options are Configure-Naked at once, although each option is Naked
// individually." The peer's Nak of the Auth-Protocol option carries
// the value the peer is willing to accept.
//
// peerSuggestion is the AuthMethod decoded from the peer's Nak (via
// authMethodFromAuthProto). order is ze's configured preference list;
// an empty list means "no fallback is acceptable" and the caller falls
// back to AuthMethodNone. The first method in order that equals
// peerSuggestion is returned; if no match, AuthMethodNone is returned
// and the next CONFREQ omits the Auth-Protocol option.
//
// The function is order-respecting: if the peer suggests PAP and order
// is [CHAPMD5, PAP], the result is PAP (matches on the second entry).
// If order is [CHAPMD5] only, the result is AuthMethodNone because the
// peer's choice is not on ze's list.
func selectAuthFallback(peerSuggestion AuthMethod, order []AuthMethod) AuthMethod {
	if peerSuggestion == AuthMethodNone {
		return AuthMethodNone
	}
	for _, m := range order {
		if m == peerSuggestion {
			return m
		}
	}
	return AuthMethodNone
}

// authMethodToLCPOptions returns the AuthProto value and AuthData
// bytes to populate in LCPOptions when building a local CONFREQ.
// Returns (0, nil) for AuthMethodNone -- the caller omits the
// Auth-Protocol option entirely.
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
	}
	return 0, nil
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
		return decision, true
	case <-s.stopCh:
		return authResponseMsg{}, false
	case <-s.sessStop:
		return authResponseMsg{}, false
	case <-timer.C:
		reason := "auth timeout after " + timeout.String()
		if label != "" {
			reason = label + ": " + reason
		}
		s.fail(reason)
		s.sendAuthEvent(EventAuthFailure{
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
// handleFrame returns term=true (LCP FSM reached Closed/Stopped),
// the wait returns ok=false so the caller aborts auth.
//
// parse extracts the method-specific Response struct from a PPP
// payload and returns (result, identifier, err). The wait uses
// s.authTimeout as its bound so periodic re-auth never parks
// indefinitely on a silent peer. On any terminal condition (malformed
// packet, channel close, stop, timeout) the helper emits
// EventAuthFailure and calls s.fail so callers may return false
// directly.
//
// label prefixes all fail-reason strings ("chap" or "chap-v2").
func waitCHAPLike[T any](
	s *pppSession,
	label string,
	wantID uint8,
	parse func([]byte) (T, uint8, error),
) (T, bool) {
	var zero T
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
				s.fail(label + ": frames channel closed")
				return zero, false
			}
			proto, payload, _, perr := ParseFrame(f)
			if perr != nil {
				putFrameBuf(f)
				s.fail(label + ": malformed frame: " + perr.Error())
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
				if term {
					return zero, false
				}
				continue
			}
			resp, gotID, perr := parse(payload)
			putFrameBuf(f)
			if perr != nil {
				s.fail(label + ": malformed response: " + perr.Error())
				return zero, false
			}
			if gotID != wantID {
				s.logger.Debug(label+": discarding response with mismatched identifier",
					"got", gotID, "want", wantID)
				continue
			}
			return resp, true
		case <-s.stopCh:
			return zero, false
		case <-s.sessStop:
			return zero, false
		case <-timer.C:
			reason := label + ": timeout awaiting Response for identifier " +
				strconv.FormatUint(uint64(wantID), 16)
			s.fail(reason)
			s.sendAuthEvent(EventAuthFailure{
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
func (s *pppSession) waitCHAPResponse(wantID uint8) (CHAPResponse, bool) {
	return waitCHAPLike(s, "chap", wantID,
		func(p []byte) (CHAPResponse, uint8, error) {
			r, err := ParseCHAPResponse(p)
			return r, r.Identifier, err
		},
	)
}

// waitMSCHAPv2Response is the MS-CHAPv2 specialization of waitCHAPLike.
// ParseMSCHAPv2Response enforces the RFC 2759 §4 shape (49-byte
// Response, Reserved/Flags == 0) as hard errors; identifier mismatch
// is the only case that loops.
func (s *pppSession) waitMSCHAPv2Response(wantID uint8) (MSCHAPv2Response, bool) {
	return waitCHAPLike(s, "chap-v2", wantID,
		func(p []byte) (MSCHAPv2Response, uint8, error) {
			r, err := ParseMSCHAPv2Response(p)
			return r, r.Identifier, err
		},
	)
}

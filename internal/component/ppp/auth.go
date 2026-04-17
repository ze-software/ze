// Design: docs/research/l2tpv2-ze-integration.md -- PPP auth-phase dispatch + shared helper
// Related: auth_events.go -- AuthMethod enum + EventAuthRequest/Success/Failure
// Related: session_run.go -- runAuthPhase switch driven by these helpers
// Related: pap.go -- runPAPAuthPhase consumer of awaitAuthDecision
// Related: chap.go -- runCHAPAuthPhase consumer of awaitAuthDecision
// Related: mschapv2.go -- runMSCHAPv2AuthPhase consumer of awaitAuthDecision

package ppp

import (
	"time"
)

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

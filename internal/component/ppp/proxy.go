// Design: docs/research/l2tpv2-implementation-guide.md -- proxy LCP (RFC 2661 §18)
// Related: lcp_options.go -- option codec used to parse the proxied byte streams
// Related: lcp_fsm.go -- FSM that the short-circuit drives directly to Opened

package ppp

import (
	"encoding/binary"
	"errors"
)

// errProxyLCPMissing is returned when the proxied LCP byte slices are
// not all populated. Without all three (AVP 26, 27, 28) ze cannot
// safely conclude the LAC and the peer reached agreement.
var errProxyLCPMissing = errors.New("ppp: proxy LCP requires all three CONFREQ AVPs")

// errProxyLCPInvalid is returned when one of the proxied byte streams
// fails to parse as an LCP option list.
var errProxyLCPInvalid = errors.New("ppp: proxy LCP option stream malformed")

// ProxyLCPResult is the outcome of a successful short-circuit. It
// carries the parameters the caller needs to skip the FSM straight
// to Opened: the negotiated MRU (so pppN MTU can be set), the
// negotiated authentication protocol (so the auth phase knows what
// the LAC already arranged), and the proxied magic numbers (so Echo
// loopback detection still works for the LNS<->peer link, even
// though LCP itself was never run on this end).
type ProxyLCPResult struct {
	// MRU negotiated between LAC and peer. Zero if neither side
	// proposed an MRU (the PPP default of 1500 then applies).
	MRU uint16

	// AuthProto is the LCP Authentication-Protocol value the LAC and
	// peer agreed on. Zero if no authentication was negotiated.
	// AuthData is any algorithm bytes that follow (e.g. CHAP-MD5
	// algorithm identifier 0x05).
	AuthProto uint16
	AuthData  []byte

	// PeerMagic is the Magic-Number from the Last-Received CONFREQ
	// (i.e. the value the original peer sent toward the LAC). Used
	// only for loopback detection on subsequent Echo-Reply traffic.
	// Zero when the peer did not negotiate Magic-Number.
	PeerMagic uint32
}

// EvaluateProxyLCP parses the three proxied CONFREQ byte streams from
// an L2TP ICCN (AVPs 26/27/28) and decides whether the LCP short-
// circuit is valid for this session. RFC 2661 §18 rules:
//
//   - All three slices MUST be non-empty.
//   - Initial-Received and Last-Received represent successive states
//     of the peer-toward-LAC negotiation; for short-circuit purposes
//     we only consume Last-Received (the converged state).
//   - Last-Sent represents the LAC-toward-peer state; we read MRU
//     and AuthProto from the intersection of Last-Sent and
//     Last-Received.
//
// On success returns the parameters the caller should use to drive
// the FSM directly to Opened. On any parse failure or missing AVP,
// returns a non-nil error and the caller MUST fall back to running
// LCP normally.
//
// This function does NOT mutate the FSM; it is a pure decoder. The
// caller (per-session goroutine in Phase 10) is responsible for
// invoking the FSM with synthetic events (e.g., set state to Opened,
// emit TLU action) on success.
func EvaluateProxyLCP(initialRecv, lastSent, lastRecv []byte) (ProxyLCPResult, error) {
	if len(initialRecv) == 0 || len(lastSent) == 0 || len(lastRecv) == 0 {
		return ProxyLCPResult{}, errProxyLCPMissing
	}

	sentOpts, err := ParseLCPOptions(lastSent)
	if err != nil {
		return ProxyLCPResult{}, errProxyLCPInvalid
	}
	recvOpts, err := ParseLCPOptions(lastRecv)
	if err != nil {
		return ProxyLCPResult{}, errProxyLCPInvalid
	}
	// We do not parse initialRecv beyond requiring it to be valid LCP
	// options; it is informational only (RFC 2661 §18 includes it so
	// the LNS can audit how negotiation evolved). Still validate it
	// to catch corrupt AVPs.
	if _, err := ParseLCPOptions(initialRecv); err != nil {
		return ProxyLCPResult{}, errProxyLCPInvalid
	}

	var out ProxyLCPResult

	// MRU: the negotiated MRU is what the peer asked the LAC to use
	// (Last-Received). RFC 1661 §6.1: a peer's MRU option in its own
	// CONFREQ tells the OTHER side the maximum it is willing to
	// receive. So Last-Received MRU is the maximum WE may send.
	if v, ok := lookupMRUOption(recvOpts); ok {
		out.MRU = v
	}

	// Auth-Proto: LAC's Last-Sent CONFREQ tells the peer what auth
	// protocol the LAC picked. The peer accepted (or LAC would not
	// have proceeded), so this is the authoritative method.
	if data, ok := lookupOption(sentOpts, LCPOptAuthProto); ok {
		if len(data) >= 2 {
			out.AuthProto = binary.BigEndian.Uint16(data[:2])
			if len(data) > 2 {
				out.AuthData = append([]byte(nil), data[2:]...)
			}
		}
	}

	// Peer's Magic from its last CONFREQ toward LAC.
	if v, ok := lookupOptionUint32(recvOpts, LCPOptMagic); ok {
		out.PeerMagic = v
	}

	return out, nil
}

// lookupOption returns the Data field of the first option matching
// optType, or (nil, false) if absent.
func lookupOption(opts []LCPOption, optType uint8) ([]byte, bool) {
	for i := range opts {
		if opts[i].Type == optType {
			return opts[i].Data, true
		}
	}
	return nil, false
}

// lookupMRUOption returns the negotiated MRU from an option list,
// or (0, false) if MRU is absent or malformed.
func lookupMRUOption(opts []LCPOption) (uint16, bool) {
	d, ok := lookupOption(opts, LCPOptMRU)
	if !ok || len(d) < 2 {
		return 0, false
	}
	return binary.BigEndian.Uint16(d[:2]), true
}

// lookupOptionUint32 returns the option's first four bytes as a
// big-endian uint32, or (0, false) if absent or shorter than 4 bytes.
func lookupOptionUint32(opts []LCPOption, optType uint8) (uint32, bool) {
	d, ok := lookupOption(opts, optType)
	if !ok || len(d) < 4 {
		return 0, false
	}
	return binary.BigEndian.Uint32(d[:4]), true
}

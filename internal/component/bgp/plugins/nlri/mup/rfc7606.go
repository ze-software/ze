// Design: rfc/short/rfc7606.md -- Section 5.4, typed NLRI
// RFC: rfc/short/rfc7606.md -- Section 5.4; rfc/short/draft-ietf-bess-mup-safi.md -- Sections 3.1, 3.3.3, and 3.3.6
// Related: types.go -- MUPRouteType and MUPArchType constants, the set this reports on
// Related: codes.go -- doctor-bgp-mup-locator-unresolved and doctor-bgp-mup-locator-mismatch
//
// RFC 7606 Section 5.4 binds BGP-MUP. The NLRI is typed in the sense Section 5.4
// describes: the supported type values are not carried in the RFC 4760
// multiprotocol capability, so a speaker can advertise ipv4/mup or ipv6/mup and
// still not implement a given type inside it.
//
// draft-ietf-bess-mup-safi Section 3.1 defines the envelope and enumerates
// Architecture Type 1 (3gpp-5g) and Route Types 1..4. It never invokes Section
// 5.4's "unless the relevant specification for that address family specifies
// otherwise" clause, so the Section 5.4 default applies: discard.
//
// This is where that ruling is recorded, beside the family registration it
// belongs to. Nothing central names MUP.

package mup

import (
	"net/netip"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"
	"github.com/ze-software/ze/internal/core/locatororigin"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Implemented reports whether ze implements this MUP architecture and route type
// pair.
//
// Both halves decide. draft-ietf-bess-mup-safi Section 3.1 says the Route Type
// specific encoding "depends on Architecture Type + Route Type", so the pair is
// what names an NLRI type, and route type 1 under an architecture ze does not
// implement is as unreadable as an unassigned route type.
//
// This is Section 5.4's "recognized" set for the family. RecognizeNLRI reads it,
// and TestImplementedMatchesRouteTypeNames holds the route-type half against
// String's own dispatch so the two cannot drift.
func (t MUPRouteType) Implemented(arch MUPArchType) bool {
	if arch != MUPArch3GPP5G {
		return false
	}
	// Recognizes only implemented route types under the supported MUP architecture.
	//exhaustive:ignore
	switch t {
	case MUPISD, MUPDSD, MUPT1ST, MUPT2ST:
		return true
	}
	return false
}

// RecognizeNLRI answers RFC 7606 Section 5.4's TYPE question for one BGP-MUP
// NLRI: Keep when ze implements its architecture and route type, Discard otherwise.
//
// nlriBytes is one NLRI as it appears on the wire. draft-ietf-bess-mup-safi
// Section 3.1 frames it as [architecture-type:1][route-type:2][length:1][body],
// preceded by a 4-byte RFC 7911 path identifier when addPath is negotiated.
//
// A slice too short to hold the architecture and route type is discarded.
// That is the fail-closed answer, and it costs nothing real: the framing walk
// that carved this slice would have reported the truncation first.
func RecognizeNLRI(nlriBytes []byte, addPath bool) nlritype.Verdict {
	off := 0
	if addPath {
		off = 4
	}
	if off+3 > len(nlriBytes) {
		return nlritype.Discard
	}
	arch := MUPArchType(nlriBytes[off])
	routeType := MUPRouteType(uint16(nlriBytes[off+1])<<8 | uint16(nlriBytes[off+2]))
	if routeType.Implemented(arch) {
		return nlritype.Keep
	}
	return nlritype.Discard
}

// recognizerFor returns the receive-path recognizer registered for the MUP
// family of afi: RecognizeNLRI's type verdict, then ValidateNLRI's body verdict
// over an implemented type. The body checks need the AFI, which is why each
// family registers its own.
//
// draft-ietf-bess-mup-safi Section 3.1.3.1 answers a malformed Type 1 Session
// Transformed route with treat-as-withdraw, by the count of octets left after the
// mandatory fields: "< 0: the mandatory fields exceed the declared Length; the
// NLRI is malformed; MUST be treated as Treat-as-withdraw.", "1: encoding is
// invalid (a valid TLV requires at minimum a Type byte and a Length byte); MUST be
// treated as Treat-as-withdraw.", and "any TLV parsing error MUST result in
// Treat-as-withdraw". Every other malformed shape ParseMUP refuses gets the same
// answer (owner ruling Q-1 (a), spec-bgp-update-propagation-rfc-defects).
func recognizerFor(afi AFI) nlritype.Recognizer {
	return func(nlriBytes []byte, addPath bool) nlritype.Verdict {
		if v := RecognizeNLRI(nlriBytes, addPath); v != nlritype.Keep {
			return v
		}
		one := nlriBytes
		if addPath {
			one = nlriBytes[4:]
		}
		if ValidateNLRI(afi, one) != nil {
			return nlritype.Withdraw
		}
		return nlritype.Keep
	}
}

const (
	// mupWarnInterval bounds per-(peer, class) Warn log lines so a hostile peer
	// cannot flood daemon logs with repeated unresolved or mismatched MUP routes.
	mupWarnInterval = 1 * time.Second
	// mupWarnMaxKeys bounds the in-memory throttle map regardless of peer churn.
	mupWarnMaxKeys = 1024
)

type mupUnresolvedKey struct {
	peer   netip.Addr
	reason locatororigin.ReasonClass
}

type mupMismatchKey struct {
	peer      netip.Addr
	routeType MUPRouteType
}

type mupThrottleEntry struct {
	lastLogged time.Time
	suppressed uint64
}

type mupWarnLimiter struct {
	mu         sync.Mutex
	now        func() time.Time
	unresolved map[mupUnresolvedKey]mupThrottleEntry
	mismatch   map[mupMismatchKey]mupThrottleEntry
}

var warnLimiter = mupWarnLimiter{
	now:        time.Now,
	unresolved: make(map[mupUnresolvedKey]mupThrottleEntry),
	mismatch:   make(map[mupMismatchKey]mupThrottleEntry),
}

func (l *mupWarnLimiter) allowUnresolved(key mupUnresolvedKey) (bool, uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	entry, exists := l.unresolved[key]
	if exists {
		if now.Sub(entry.lastLogged) < mupWarnInterval {
			entry.suppressed++
			l.unresolved[key] = entry
			return false, 0
		}
	}
	if !exists {
		if len(l.unresolved) >= mupWarnMaxKeys {
			clear(l.unresolved)
		}
	}
	suppressed := entry.suppressed
	l.unresolved[key] = mupThrottleEntry{lastLogged: now}
	return true, suppressed
}

func (l *mupWarnLimiter) allowMismatch(key mupMismatchKey) (bool, uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	entry, exists := l.mismatch[key]
	if exists {
		if now.Sub(entry.lastLogged) < mupWarnInterval {
			entry.suppressed++
			l.mismatch[key] = entry
			return false, 0
		}
	}
	if !exists {
		if len(l.mismatch) >= mupWarnMaxKeys {
			clear(l.mismatch)
		}
	}
	suppressed := entry.suppressed
	l.mismatch[key] = mupThrottleEntry{lastLogged: now}
	return true, suppressed
}

func resetMUPWarnLimiterForTest(now func() time.Time) {
	if now == nil {
		now = time.Now
	}
	warnLimiter.mu.Lock()
	defer warnLimiter.mu.Unlock()
	warnLimiter.now = now
	clear(warnLimiter.unresolved)
	clear(warnLimiter.mismatch)
}

// updateCheckFor returns the UPDATE-level check registered for the MUP family
// of afi (nlritype.UpdateCheck). It enforces draft-ietf-bess-mup-safi Sections
// 3.3.3 (ISD) and 3.3.6 (DSD): Prefix-SID presence (including after RFC 8669
// Section 4 EBGP discard) and locator-originator comparison against the
// read-locked internal/core/locatororigin index.
func updateCheckFor(afi AFI) nlritype.UpdateCheck {
	return func(nlriBytes []byte, addPath bool, attrs *nlritype.UpdateAttrs) nlritype.Verdict {
		return checkSegmentDiscovery(afi, nlriBytes, addPath, attrs)
	}
}

// checkSegmentDiscovery judges one announced MUP NLRI against the UPDATE's
// post-discard Prefix-SID attribute, MP_REACH_NLRI next hop, and the in-daemon
// locator-originator index.
//
// nlritype.RetainUpdate calls this only for an announced NLRI recognizerFor
// kept, so the architecture, route type, and body framing are already valid.
func checkSegmentDiscovery(afi AFI, nlriBytes []byte, addPath bool, attrs *nlritype.UpdateAttrs) nlritype.Verdict {
	raw := nlriBytes
	if addPath {
		if len(raw) < 4 {
			// Unreachable through RetainUpdate: recognizerFor discards a slice this short.
			return nlritype.Keep
		}
		raw = raw[4:]
	}
	if len(raw) < mupHeaderLen {
		// Unreachable through RetainUpdate: recognizerFor discards a slice this short.
		return nlritype.Keep
	}
	routeType := MUPRouteType(uint16(raw[1])<<8 | uint16(raw[2]))
	// Only Segment Discovery route types carry Prefix-SID and originator rules.
	//exhaustive:ignore
	switch routeType {
	case MUPDSD, MUPISD:
	default:
		return nlritype.Keep
	}

	prefixSIDVal, hasPrefixSID := attrs.RawValue(attribute.AttrPrefixSID)
	if !hasPrefixSID {
		if routeType == MUPDSD {
			// draft-ietf-bess-mup-safi Section 3.3.6: "When a BGP speaker receives a
			// MP_REACH_NLRI attribute update message with a Direct Segment Discovery route
			// without a prefix SID attribute, than it MUST be treated as if it contained a
			// malformed prefix SID attribute and the "Treat-as-withdraw procedure of
			// [RFC7606] is applied.  A BGP speaker MUST skip such NLRIs and continue
			// processing of rest of the Update message."
			return nlritype.Withdraw
		}
		// draft-ietf-bess-mup-safi Section 3.3.3: "When a BGP speaker receives the
		// Interwork Segment Discovery routes with a MP_REACH_NLRI attribute without a
		// prefix SID attribute, then it MUST be treated as if it contained a malformed
		// prefix SID attribute and the "Treat-as-withdraw procedure of [RFC7606] is
		// applied.  A BGP speaker MUST skip such NLRIs and continue processing of rest
		// of the Update message."
		return nlritype.Withdraw
	}

	var route MUP
	rest, err := route.decode(afi, raw)
	if err != nil {
		return nlritype.Withdraw
	}
	if len(rest) != 0 {
		return nlritype.Withdraw
	}

	sid, hasSID := attribute.FirstSRv6ServiceSID(prefixSIDVal)
	if !hasSID {
		warnLocatorUnresolved(attrs.Peer(), &route, netip.Addr{}, locatororigin.ReasonNoSRv6SID,
			"Prefix-SID attribute carries no SRv6 Service SID")
		return nlritype.Keep
	}

	res := locatororigin.Resolve(sid)
	if routeType == MUPDSD {
		nhBytes, hasNH := attrs.MPReachNextHop()
		return checkDSDOriginator(attrs.Peer(), &route, sid, nhBytes, hasNH, &res)
	}
	return checkISDOriginator(attrs.Peer(), &route, sid, &res)
}

// checkDSDOriginator enforces draft-ietf-bess-mup-safi Section 3.3.6 for a DSD
// route carrying a Prefix-SID attribute.
func checkDSDOriginator(peer netip.Addr, route *MUP, sid netip.Addr, nhBytes []byte, hasNH bool, res *locatororigin.Result) nlritype.Verdict {
	nhAddr, nhComparable := parseComparableNextHop(nhBytes)
	if !nhComparable {
		warnUncomparableNextHop(peer, route, sid, nhBytes, hasNH, res)
		return nlritype.Keep
	}
	outcome, reasonClass, reason := res.MatchAddress(nhAddr)
	switch outcome {
	case locatororigin.MatchKnown:
		return nlritype.Keep
	case locatororigin.MatchUnknown:
		warnLocatorUnresolved(peer, route, sid, reasonClass, reason)
		return nlritype.Keep
	case locatororigin.MatchMismatch:
		warnLocatorMismatch(peer, route, "next-hop", nhAddr.String(), sid, res.Originator)
		// draft-ietf-bess-mup-safi Section 3.3.6: "However, the receiving BGP
		// speaker MUST ensure that the received nexthop value in the
		// MP_REACH_NLRI attribute is identical to the originator of the locator
		// value in the prefix SID attribute.  The originator of the locator
		// value can be resolved from the IPv6 IGP table.  If the result of the
		// match is not identical then the receiving BGP speaker MUST consider
		// it as a malformed NLRI and the "Treat-as-withdraw procedure of
		// [RFC7606] is applied.  A BGP speaker MUST skip such NLRIs and
		// continue processing of rest of the Update message."
		return nlritype.Withdraw
	case locatororigin.MatchUnspecified:
		panic("BUG: mup: unspecified locator match outcome")
	default:
		panic("BUG: mup: unknown locator match outcome")
	}
}

// checkISDOriginator enforces draft-ietf-bess-mup-safi Section 3.3.3 for an ISD
// route carrying a Prefix-SID attribute.
//
// draft-ietf-bess-mup-safi Section 3.3.3: "The BGP speaker receiving the
// Interwork Segment Discovery routes SHOULD ignore the nexthop in the
// MP_REACH_NLRI attribute.  However, the receiving BGP speaker MUST ensure that
// the value of Address filed in the NLRI is an address of the originator of the
// locator value in the prefix SID attribute."
func checkISDOriginator(peer netip.Addr, route *MUP, sid netip.Addr, res *locatororigin.Result) nlritype.Verdict {
	isdPrefix := netip.PrefixFrom(route.prefix, int(route.prefixBits)).Masked()
	addrOutcome, addrRC, addrReason := res.MatchPrefix(isdPrefix)
	switch addrOutcome {
	case locatororigin.MatchKnown:
		return nlritype.Keep
	case locatororigin.MatchUnknown:
		warnLocatorUnresolved(peer, route, sid, addrRC, addrReason)
		return nlritype.Keep
	case locatororigin.MatchMismatch:
		warnLocatorMismatch(peer, route, "address", isdPrefix.String(), sid, res.Originator)
		// draft-ietf-bess-mup-safi Section 3.3.3: "However, the receiving BGP
		// speaker MUST ensure that the value of Address filed in the NLRI is an
		// address of the originator of the locator value in the prefix SID
		// attribute.  The originator of the locator value can be resolved from
		// the IPv6 IGP table.  If the result of the match is not identical then
		// the receiving BGP speaker MUST consider it as a malformed NLRI and
		// the "Treat-as-withdraw procedure of [RFC7606] is applied.  A BGP
		// speaker MUST skip such NLRIs and continue processing of rest of the
		// Update message."
		return nlritype.Withdraw
	case locatororigin.MatchUnspecified:
		panic("BUG: mup: unspecified locator match outcome")
	default:
		panic("BUG: mup: unknown locator match outcome")
	}
}

func warnUncomparableNextHop(peer netip.Addr, route *MUP, sid netip.Addr, nhBytes []byte, hasNH bool, res *locatororigin.Result) {
	if !hasNH {
		if !res.Known {
			warnLocatorUnresolved(peer, route, sid, res.ReasonClass, res.Reason)
			return
		}
	}
	var tb textbuf.Buffer
	reason := tb.Str("MP_REACH_NLRI next hop length ").Int(int64(len(nhBytes))).Str(" not comparable").String()
	warnLocatorUnresolved(peer, route, sid, locatororigin.ReasonUnsupportedNextHopLen, reason)
}

// parseComparableNextHop extracts the comparable next-hop address from raw
// MP_REACH_NLRI next-hop bytes: 16 octets whole; 32 octets global first half
// (RFC 2545: the link-local second half is never advertised in an LSP); 4 octets
// IPv4 (always mismatches a known IPv6 originator). Any other length is not
// comparable.
func parseComparableNextHop(nhBytes []byte) (netip.Addr, bool) {
	switch len(nhBytes) {
	case 16:
		return netip.AddrFrom16([16]byte(nhBytes)), true
	case 32:
		return netip.AddrFrom16([16]byte(nhBytes[:16])), true
	case 4:
		return netip.AddrFrom4([4]byte(nhBytes)), true
	default:
		return netip.Addr{}, false
	}
}

func peerLogLabel(peer netip.Addr) string {
	if peer.IsValid() {
		return peer.String()
	}
	return "unknown-peer"
}

func warnLocatorUnresolved(peer netip.Addr, route *MUP, sid netip.Addr, reasonClass locatororigin.ReasonClass, reason string) {
	peerLabel := peerLogLabel(peer)
	sidLabel := "none"
	if sid.IsValid() {
		sidLabel = sid.String()
	}
	if reasonClass == locatororigin.ReasonNone {
		reasonClass = locatororigin.ReasonNoCoveringLocator
	}
	if reason == "" {
		reason = reasonClass.String()
	}

	var subjBuf textbuf.Buffer
	subject := subjBuf.Str(peerLabel).Byte(' ').Str(reasonClass.String()).String()

	var msgBuf textbuf.Buffer
	msg := report.FitMessage(msgBuf.Str("BGP MUP ").Str(route.String()).
		Str(" from peer ").Str(peerLabel).
		Str(" kept unverified (SID ").Str(sidLabel).
		Str("): ").Str(reason).String())

	report.RaiseWarningFix(reportSourceMUP, codeDoctorBGPMUPLocatorUnresolved, subject, msg, fixBGPMUPLocatorUnresolved)

	if emit, suppressed := warnLimiter.allowUnresolved(mupUnresolvedKey{peer: peer, reason: reasonClass}); emit {
		logger.Warn("BGP MUP segment discovery locator originator could not be resolved; keeping route unverified",
			"code", codeDoctorBGPMUPLocatorUnresolved,
			"peer", peerLabel,
			"route", route.String(),
			"sid", sidLabel,
			"reason", reason,
			"fix", fixBGPMUPLocatorUnresolved,
			"suppressed-since-last", suppressed,
		)
	}
}

func warnLocatorMismatch(peer netip.Addr, route *MUP, field, value string, sid netip.Addr, originator string) {
	peerLabel := peerLogLabel(peer)
	var subjBuf textbuf.Buffer
	subject := subjBuf.Str(peerLabel).Byte(' ').Str(route.RouteType().String()).String()

	var msgBuf textbuf.Buffer
	msg := report.FitMessage(msgBuf.Str("BGP MUP ").Str(route.String()).
		Str(" from peer ").Str(peerLabel).
		Str(" treated as withdrawn: ").Str(field).Byte(' ').Str(value).
		Str(" does not belong to locator originator ").Str(originator).
		Str(" (SID ").Str(sid.String()).Byte(')').String())

	report.RaiseErrorFixOnce(reportSourceMUP, codeDoctorBGPMUPLocatorMismatch, subject, msg, fixBGPMUPLocatorMismatch, mupWarnInterval)

	if emit, suppressed := warnLimiter.allowMismatch(mupMismatchKey{peer: peer, routeType: route.RouteType()}); emit {
		logger.Warn("BGP MUP segment discovery route withdrawn: address or next hop does not match locator originator",
			"code", codeDoctorBGPMUPLocatorMismatch,
			"peer", peerLabel,
			"route", route.String(),
			"field", field,
			"value", value,
			"sid", sid.String(),
			"originator", originator,
			"fix", fixBGPMUPLocatorMismatch,
			"suppressed-since-last", suppressed,
		)
	}
}

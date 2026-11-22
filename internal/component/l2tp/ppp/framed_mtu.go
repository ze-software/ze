// Design: docs/guide/l2tp.md -- the IP MTU of the subscriber's interface
// Related: session_run.go -- afterLCPOpen configures the interface with interfaceMTU
// Related: ncp.go -- runNCPPhase keeps IPv6CP off below the IPv6 minimum
// Related: request.go -- StageFramedMTU, the provider's half

package ppp

import (
	"time"

	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// framedMTUReportSource names the PPP engine on the report bus.
	framedMTUReportSource = "ppp"
	// framedMTUEventWindow bounds the events one subscriber raises: a
	// profile repeats its Framed-MTU on every Access-Accept.
	framedMTUEventWindow = time.Minute
	// framedMTUFix is the fix the event carries: the value came from the
	// subscriber's RADIUS profile, so the profile is what changes.
	framedMTUFix = "correct the subscriber's RADIUS profile"
	// framedMTUNoSubject is the subject of a session with no authenticated
	// name: the bus refuses an empty subject, which would lose the event.
	framedMTUNoSubject = "(no user name)"
)

// interfaceMTU returns the IP MTU the session configures on its interface:
// transmit, the LCP transmit limit (transmitMRU), lowered to the Framed-MTU
// of this LCP lifetime's accepted request when one was staged.
//
// RFC 2865 Section 5.12: "This Attribute indicates the Maximum Transmission Unit to be
// configured for the user, when it is not negotiated by some other
// means (such as PPP)."
//
// LCP negotiated the link before authentication, so the Framed-MTU never
// raises the interface MTU past what the peer agreed to receive: a larger
// value leaves transmit in place, and an Info line says so. It lowers only
// the IP MTU; PPP control replies keep the LCP limit (transmitMRU). The
// session goroutine MUST be the caller: authenticatedFramedMTU is its own.
func (s *pppSession) interfaceMTU(transmit uint16) uint16 {
	framed := s.authenticatedFramedMTU
	if framed == 0 {
		return transmit
	}
	if framed > transmit {
		s.logger.Info("ppp: RADIUS Framed-MTU exceeds the PPP transmit limit negotiated in LCP; the interface keeps the transmit limit",
			"framed-mtu", framed, "transmit-mru", transmit)
		return transmit
	}
	s.logger.Info("ppp: interface MTU set from the RADIUS Framed-MTU",
		"framed-mtu", framed, "transmit-mru", transmit)
	return framed
}

// framedMTUBelowIPv6 reports whether this lifetime's Framed-MTU sets the
// interface below the IPv6 minimum. Linux removes IPv6 from an interface
// whose MTU is below 1280, so IPv6CP MUST NOT start then.
//
// RFC 5072 Section 2: "PPP links supporting IPv6 MUST allow the
// information field to be at least as large as the minimum link MTU
// size required for IPv6 [2]."
func (s *pppSession) framedMTUBelowIPv6() bool {
	framed := s.authenticatedFramedMTU
	if framed == 0 {
		return false
	}
	return framed < MinIPv6MRU
}

// reportFramedMTUBlocksIPv6 logs that the Framed-MTU keeps IPv6CP from
// starting and raises it on the report bus under the subscriber's name, at
// most once per subscriber per framedMTUEventWindow. Safe for concurrent use.
func (s *pppSession) reportFramedMTUBlocksIPv6() {
	var problem textbuf.Buffer
	problem.Str("RADIUS Framed-MTU ").Uint16(s.authenticatedFramedMTU).
		Str(" is below 1280, the IPv6 minimum; IPv6CP does not start for this session")
	text := problem.String()
	s.logger.Warn("ppp: "+text+". Correct the subscriber's RADIUS profile, or send a Framed-MTU of at least 1280 for IPv6 service",
		"code", CodeDoctorRADIUSFramedMTU, "framed-mtu", s.authenticatedFramedMTU)
	subject := s.authenticatedUsername
	if subject == "" {
		subject = framedMTUNoSubject
	}
	report.RaiseErrorFixOnce(framedMTUReportSource, CodeDoctorRADIUSFramedMTU,
		diagnosticSubject(subject), report.FitMessage(text), framedMTUFix, framedMTUEventWindow)
}

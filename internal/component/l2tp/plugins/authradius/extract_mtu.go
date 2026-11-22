// Design: docs/architecture/l2tp/bng-1-radius-attributes.md -- RADIUS attribute extraction
// Related: extract.go -- extractAuthMetadata calls extractFramedMTU
// Related: handler.go -- doRADIUS stages the value on the PPP session

package l2tpauthradius

import (
	"encoding/binary"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// framedMTUValueMin and framedMTUValueMax bound the Framed-MTU value.
	// RFC 2865 Section 5.12: "Despite the size of the field, values range from 64 to 65535."
	framedMTUValueMin = 64
	framedMTUValueMax = 65535
	// framedMTUFix is the fix the event carries: the value came from the
	// subscriber's RADIUS profile, so the profile is what changes.
	framedMTUFix = "correct the subscriber's RADIUS profile"

	framedMTUNotFourOctets = "octets long, not four; ignored, the interface MTU comes from PPP negotiation"
	framedMTUBelowRADIUS   = "is below 64, the RFC 2865 minimum; ignored, the interface MTU comes from PPP negotiation"
	framedMTUAboveRADIUS   = "is above 65535, the RFC 2865 maximum; ignored, the interface MTU comes from PPP negotiation"
	framedMTUBelowIPv4     = "is below 68, the IPv4 minimum; raised to 68"
)

// extractFramedMTU reads the Framed-MTU of an Access-Accept and reports
// whether it carries one Ze applies. The value it returns is always from 68
// (ppp.MinIPv4MRU) to 65535.
//
// RFC 2865 Section 5.12: "This Attribute indicates the Maximum Transmission Unit to be
// configured for the user, when it is not negotiated by some other
// means (such as PPP).  It MAY be used in Access-Accept packets."
//
// PPP negotiates the MRU in LCP, which opens before authentication, so the
// value cannot change what LCP agreed. The PPP session uses it to lower the
// IP MTU it configures on the subscriber's interface (ppp interfaceMTU).
//
// A value Ze cannot use is logged at WARN and raised on the report bus under
// subscriber; it never fails the Access-Accept. A malformed value is ignored,
// so the interface MTU comes from PPP alone. A value from 64 to 67 is legal
// RADIUS but below what IPv4 needs, so it is raised to 68, the same floor
// LCP applies when it Naks a peer's MRU.
func extractFramedMTU(resp *radius.Packet, subscriber string) (uint16, bool) {
	raw := resp.FindAttr(radius.AttrFramedMTU)
	if raw == nil {
		return 0, false
	}
	// RFC 2865 Section 5.12: "The Value field is four octets."
	if len(raw) != 4 {
		reportFramedMTU(subscriber, uint64(len(raw)), "value is", framedMTUNotFourOctets)
		return 0, false
	}
	value := binary.BigEndian.Uint32(raw)
	if value < framedMTUValueMin {
		reportFramedMTU(subscriber, uint64(value), "", framedMTUBelowRADIUS)
		return 0, false
	}
	if value > framedMTUValueMax {
		reportFramedMTU(subscriber, uint64(value), "", framedMTUAboveRADIUS)
		return 0, false
	}
	// RFC 791 Section 3.2: "Every internet module must be able to forward a datagram of 68
	// octets without further fragmentation."
	if value < uint32(ppp.MinIPv4MRU) {
		reportFramedMTU(subscriber, uint64(value), "", framedMTUBelowIPv4)
		return ppp.MinIPv4MRU, true
	}
	return uint16(value), true
}

// reportFramedMTU logs one Framed-MTU Ze did not apply as sent and raises it
// on the report bus under the subscriber, at most once per subscriber per
// framedRouteEventWindow, the window the framed-route events use. The text
// reads "Framed-MTU [lead ]<number> <outcome>". The number comes from the
// profile and is never a secret. Safe for concurrent use.
func reportFramedMTU(subscriber string, number uint64, lead, outcome string) {
	var problem textbuf.Buffer
	problem.Str("Framed-MTU ")
	if lead != "" {
		problem.Str(lead).Byte(' ')
	}
	problem.Uint(number).Byte(' ').Str(outcome)
	text := problem.String()
	logger().Warn("l2tp-auth-radius: "+text+". Correct the subscriber's RADIUS profile",
		"code", ppp.CodeDoctorRADIUSFramedMTU, "subscriber", subscriber)
	subject := subscriber
	if subject == "" {
		subject = framedRouteNoUserName
	}
	report.RaiseErrorFixOnce(Name, ppp.CodeDoctorRADIUSFramedMTU, subject,
		report.FitMessage(text), framedMTUFix, framedRouteEventWindow)
}

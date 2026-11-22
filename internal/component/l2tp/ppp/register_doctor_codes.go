// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/l2tp/ppp.

package ppp

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	// CodeDoctorPPPCleanup is the doctor diagnostic code "doctor-ppp-cleanup".
	CodeDoctorPPPCleanup = "doctor-ppp-cleanup"
	// CodeDoctorRADIUSFramedMTU is the doctor diagnostic code "doctor-radius-framed-mtu".
	CodeDoctorRADIUSFramedMTU = "doctor-radius-framed-mtu"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        CodeDoctorPPPCleanup,
		Title:       "PPP session resource removal is unconfirmed",
		Description: "A PPP transport, session handle, or IPv6 service reported a cleanup failure. The running daemon retains the created interface identity and backend cause; a successful cleanup for another resource does not confirm this removal. Inspect the warning, repair the reported backend, and confirm that the affected resources were removed before restarting service.",
		Examples:    []string{"ze show doctor", "ze show warnings"},
	},
	{
		Code:        CodeDoctorRADIUSFramedMTU,
		Title:       "RADIUS Framed-MTU not applied as sent",
		Description: "A subscriber's Access-Accept carried a Framed-MTU that Ze could not apply as sent. A value that is not four octets, or is outside the 64 to 65535 range of RFC 2865 Section 5.12, is ignored and the interface MTU comes from PPP negotiation. A value from 64 to 67 is raised to 68, the IPv4 minimum. A value below 1280 keeps IPv6CP from starting, because IPv6 needs an MTU of at least 1280. The event names the subscriber and the value. Correct the subscriber's RADIUS profile.",
		Examples:    []string{"ze show errors", "ze show diagnostic doctor-radius-framed-mtu"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: ppp: doctor code registration refused: " + err.Error())
		}
	}
}

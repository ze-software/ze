// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/l2tp/plugins/authradius.

package l2tpauthradius

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorRADIUSUnreachable        = "doctor-radius-unreachable"
	codeDoctorRADIUSCoAApply           = "doctor-radius-coa-apply"
	codeDoctorRADIUSFramedRouteSkipped = "doctor-radius-framed-route-skipped"
	codeDoctorRADIUSInvalid            = "doctor-radius-invalid"
	codeDoctorRADIUSUnsignedResponse   = "doctor-radius-unsigned-response"
	exampleDoctorJSON                  = "ze doctor --json"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorRADIUSUnreachable,
		Title:       "RADIUS servers unreachable",
		Description: "No configured L2TP RADIUS server could be reached by the UDP readiness probe.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-radius-unreachable"},
	},
	{
		Code:        codeDoctorRADIUSCoAApply,
		Title:       "Subscriber authorization backend did not confirm a change",
		Description: "A RADIUS CoA owner failed to apply or restore a subscriber's shaping or VLAN maps. The runtime finding retains the backend cause until a successful retry for that session, session teardown, or provider shutdown. Inspect shaping and interface state, repair the backend, then retry the authenticated authorization request.",
		Examples:    []string{"ze show doctor", "ze show l2tp shaper"},
	},
	{
		Code:        codeDoctorRADIUSFramedRouteSkipped,
		Title:       "RADIUS framed route skipped",
		Description: "A subscriber's Access-Accept carried a Framed-Route or Framed-IPv6-Route value Ze cannot parse, so that route was not installed while the rest of the authorization still applied. The event names the subscriber, the attribute and why the text did not parse. Correct the subscriber's RADIUS profile.",
		Examples:    []string{"ze show errors", "ze show diagnostic doctor-radius-framed-route-skipped"},
	},
	{
		Code:        codeDoctorRADIUSInvalid,
		Title:       "RADIUS server row holds an invalid value",
		Description: "An L2TP RADIUS server row holds a require-message-authenticator value that is neither true nor false, so the readiness probe did not run against it. Set the leaf on that row to true, or to false only for a server that cannot sign its answers.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-radius-invalid"},
	},
	{
		Code:        codeDoctorRADIUSUnsignedResponse,
		Title:       "RADIUS server answers without Message-Authenticator",
		Description: "An L2TP RADIUS server answered the probe Access-Request only without a Message-Authenticator. Ze discards such answers while the server's require-message-authenticator is true (BlastRADIUS, CVE-2024-3596), so subscriber logins via that server fail. Configure the server to sign its answers, or set require-message-authenticator false on that server row.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-radius-unsigned-response"},
	},
}

// registerDoctorCodes registers this package's doctor diagnostic codes.
func registerDoctorCodes() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: l2tpauthradius: doctor code registration refused: " + err.Error())
		}
	}
}

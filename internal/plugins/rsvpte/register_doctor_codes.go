// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/rsvpte.

package rsvpte

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorRSVPTERawsockUnavailable = "doctor-rsvpte-rawsock-unavailable"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorRSVPTERawsockUnavailable,
		Title:       "RSVP-TE raw IP socket unavailable",
		Description: "RSVP-TE is configured but a raw IP socket for protocol 46 (RFC 2205) cannot be opened. This requires CAP_NET_RAW or root; without it RSVP-TE cannot send or receive PATH/RESV messages.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-rsvpte-rawsock-unavailable"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: rsvpte: doctor code registration refused: " + err.Error())
		}
	}
}

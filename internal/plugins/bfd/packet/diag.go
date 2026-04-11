// Design: rfc/short/rfc5880.md -- Diagnostic codes and State (Section 4.1)
//
// Diagnostic and State constants live here so control.go can stay focused
// on wire format and parsing.
//
// Related: control.go -- 24-byte mandatory section codec
package packet

// Diag is the 5-bit Diagnostic field carried in BFD Control packets.
//
// RFC 5880 Section 4.1 defines values 0-8; 9-31 are reserved. The diagnostic
// records the reason for the most recent local state change and remains set
// in transmitted packets until cleared by another transition.
type Diag uint8

const (
	DiagNone                  Diag = 0 // No Diagnostic
	DiagControlDetectExpired  Diag = 1 // Control Detection Time Expired
	DiagEchoFailed            Diag = 2 // Echo Function Failed
	DiagNeighborSignaledDown  Diag = 3 // Neighbor Signaled Session Down
	DiagForwardingPlaneReset  Diag = 4 // Forwarding Plane Reset
	DiagPathDown              Diag = 5 // Path Down
	DiagConcatPathDown        Diag = 6 // Concatenated Path Down
	DiagAdminDown             Diag = 7 // Administratively Down
	DiagReverseConcatPathDown Diag = 8 // Reverse Concatenated Path Down
)

var diagNames = [...]string{
	DiagNone:                  "no-diagnostic",
	DiagControlDetectExpired:  "control-detection-time-expired",
	DiagEchoFailed:            "echo-function-failed",
	DiagNeighborSignaledDown:  "neighbor-signaled-session-down",
	DiagForwardingPlaneReset:  "forwarding-plane-reset",
	DiagPathDown:              "path-down",
	DiagConcatPathDown:        "concatenated-path-down",
	DiagAdminDown:             "administratively-down",
	DiagReverseConcatPathDown: "reverse-concatenated-path-down",
}

// String returns the canonical RFC 5880 name for the diagnostic. Values
// 9-31 are reserved by RFC 5880 and render as "reserved".
func (d Diag) String() string {
	if int(d) < len(diagNames) && diagNames[d] != "" {
		return diagNames[d]
	}
	return "reserved"
}

// State is the 2-bit BFD session state field carried in Control packets.
//
// RFC 5880 Section 4.1.
type State uint8

const (
	StateAdminDown State = 0
	StateDown      State = 1
	StateInit      State = 2
	StateUp        State = 3
)

var stateNames = [...]string{
	StateAdminDown: "admin-down",
	StateDown:      "down",
	StateInit:      "init",
	StateUp:        "up",
}

// String returns the canonical RFC 5880 name for the state. The 2-bit field
// covers all four defined values, so any State decoded from the wire is
// always one of the named constants.
func (s State) String() string {
	if int(s) < len(stateNames) {
		return stateNames[s]
	}
	return "invalid"
}

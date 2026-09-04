// Design: docs/architecture/l2tp/bng-1-radius-attributes.md -- Acct-Terminate-Cause
// RFC: rfc/short/rfc2866.md -- Acct-Terminate-Cause (Section 5.10)
// Related: events.go -- SessionDownPayload carries the cause out of the reactor

package events

// TerminateCause names why a subscriber session ended, as one of the integers
// RFC 2866 Section 5.10 defines for the Acct-Terminate-Cause attribute. The
// value IS the RFC integer, so the accounting plugin encodes it directly.
//
// A typed cause exists because the free text on ppp.EventSessionDown cannot be
// mapped: its own comment forbids parsing the reason for control flow. Each
// teardown site names the cause that is TRUE of it instead.
//
// Only the causes ze can honestly produce are named. RFC 2866 Section 5.10
// defines eighteen; a teardown ze cannot attribute reports NAS Error rather
// than inventing a more specific one, because a wrong cause lands in an
// operator's billing store.
type TerminateCause uint32

const (
	// TerminateCauseUnspecified is the zero value, meaning no teardown site
	// named a cause. It is never sent: the accounting plugin reports NAS Error
	// for it, which is what RFC 2866 Section 5.10 provides for an ending the
	// NAS cannot attribute.
	TerminateCauseUnspecified TerminateCause = 0

	// TerminateCauseUserRequest is RFC 2866 Section 5.10 value 1: "User
	// requested termination of service, for example with LCP Terminate or by
	// logging out." Ze reports it when LCP reaches Closed or Stopped.
	TerminateCauseUserRequest TerminateCause = 1

	// TerminateCauseLostCarrier is RFC 2866 Section 5.10 value 2: "DCD was
	// dropped on the port." Ze reports it when LCP echo probes stop being
	// answered, which is how a virtual link says its carrier is gone.
	TerminateCauseLostCarrier TerminateCause = 2

	// TerminateCauseLostService is RFC 2866 Section 5.10 value 3: "Service can
	// no longer be provided; for example, user's connection to a host was
	// interrupted." Ze reports it for every session on a tunnel that went
	// away, because the transport the subscriber was carried on is gone
	// whatever ended the tunnel.
	TerminateCauseLostService TerminateCause = 3

	// TerminateCauseIdleTimeout is RFC 2866 Section 5.10 value 4: "Idle timer
	// expired." Ze reports it for the Idle-Timeout of RFC 2865 Section 5.28.
	TerminateCauseIdleTimeout TerminateCause = 4

	// TerminateCauseSessionTimeout is RFC 2866 Section 5.10 value 5: "Maximum
	// session length timer expired." Ze reports it for the Session-Timeout of
	// RFC 2865 Section 5.27.
	TerminateCauseSessionTimeout TerminateCause = 5

	// TerminateCauseAdminReset is RFC 2866 Section 5.10 value 6:
	// "Administrator reset the port or session." Ze reports it for an operator
	// clear and for a RADIUS Disconnect-Request (RFC 5176).
	TerminateCauseAdminReset TerminateCause = 6

	// TerminateCauseAdminReboot is RFC 2866 Section 5.10 value 7:
	// "Administrator is ending service on the NAS, for example prior to
	// rebooting the NAS." Ze reports it for the sessions it stops on shutdown.
	TerminateCauseAdminReboot TerminateCause = 7

	// TerminateCauseNASError is RFC 2866 Section 5.10 value 9: "NAS detected
	// some error (other than on the port) which required ending the session."
	// It is also the honest answer for a teardown ze cannot attribute.
	TerminateCauseNASError TerminateCause = 9

	// TerminateCauseNASRequest is RFC 2866 Section 5.10 value 10: "NAS ended
	// session for a non-error reason not otherwise listed here." Ze reports it
	// when its own PPP driver stops a running session deliberately.
	TerminateCauseNASRequest TerminateCause = 10
)

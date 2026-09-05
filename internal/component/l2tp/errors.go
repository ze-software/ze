// Design: docs/architecture/wire/l2tp.md — L2TP wire error taxonomy
// RFC: rfc/short/rfc2661.md — RFC 2661 Sections 3.1, 4.4.3, 5.3
// Related: header.go — header parser that returns these errors
// Related: avp.go — AVP iterator that returns these errors
// Related: hidden.go — hidden AVP decryption errors

package l2tp

import "errors"

// ErrShortBuffer reports that the input slice does not contain enough
// bytes to parse the requested field.
var ErrShortBuffer = errors.New("l2tp: short buffer")

// ErrUnsupportedVersion reports that the L2TP header carries a version
// other than 2. L2F (Ver=1) is silently discarded by the reactor; L2TPv3
// (Ver=3) is rejected with StopCCN Result Code 5 by the tunnel state
// machine. This phase only reports the condition.
var ErrUnsupportedVersion = errors.New("l2tp: unsupported header version")

// ErrMalformedControl reports that a control message does not carry L=1
// and S=1 as required by RFC 2661 Section 3.1.
var ErrMalformedControl = errors.New("l2tp: malformed control header")

// ErrInvalidAVPLen reports that an AVP's Length field is out of range
// (below 6 or extends past the enclosing payload).
var ErrInvalidAVPLen = errors.New("l2tp: invalid AVP length")

// ErrHiddenLenMismatch reports that decrypting a hidden AVP produced an
// Original Length field that exceeds the ciphertext. Typically caused
// by a wrong shared secret or wrong Random Vector.
var ErrHiddenLenMismatch = errors.New("l2tp: hidden AVP length mismatch")

// sccrqRejection is an SCCRQ ze REFUSES and answers, rather than drops in
// silence. RFC 2661 Section 7.1 gives one treatment to the three shapes it
// names: "Examples of a malformed control message include one that has an
// invalid value in its header, contains an AVP that is formatted incorrectly
// or whose value is out of range, or a message that is missing a required
// AVP", and "Receipt of an invalid or unrecoverable malformed control message
// should be logged appropriately and the control connection cleared to ensure
// recovery to a known state." Section 7.2.1 spells the clearing for an SCCRQ
// that arrives in idle: "Receive SCCRQ, not acceptable | Send StopCCN, Clean
// up | idle".
//
// The reason sits in the error rather than at the reactor because the parser
// is what knows which AVP was wrong. Detail travels twice: onto the wire as
// the StopCCN Result Code AVP's Error Message field (RFC 2661 Section 4.4.2,
// "an arbitrary string providing further (human readable) text associated
// with the condition"), and into ze's own log line. It is read by the
// operator of an unauthenticated peer, so it names the AVP that is wrong and
// nothing about ze.
//
// The values below are the whole set the reactor answers; every other parse
// failure stays a silent drop. Each is a package var rather than a value
// built at the return, so a flood of malformed SCCRQs allocates nothing.
type sccrqRejection struct {
	// Detail names the AVP that is wrong, for the peer.
	Detail string
	// Log is the message of the Info line ze writes when it answers. It is a
	// field rather than one sentence at the emitter because a zero Assigned
	// Tunnel ID has been logged by name since ze first answered one, and that
	// sentence is what an operator greps for. Its two values are below.
	Log string
}

func (e *sccrqRejection) Error() string { return "l2tp: " + e.Detail }

// The two Info messages a refused SCCRQ produces. The Result Code, the Error
// Code and the Detail ride beside them as attributes.
const (
	logMalformedSCCRQ    = "l2tp: malformed SCCRQ answered with StopCCN"
	logZeroTunnelIDSCCRQ = "l2tp: zero Assigned Tunnel ID SCCRQ answered with StopCCN"
)

// The SCCRQ rejections, in the order RFC 2661 Section 6.1 lists the AVPs:
// "The following AVPs MUST be present in the SCCRQ: Message Type AVP,
// Protocol Version, Host Name, Framing Capabilities, Assigned Tunnel ID."
// Each AVP earns a rejection for being absent and one for carrying a value ze
// cannot read, because Section 7.1 treats the two alike.
//
// An M-bit test does not sort them. Section 7.1 says a malformed AVP whose
// M-bit is clear "should be ignored ... and the message accepted", and an
// ignored mandatory AVP is an absent one, which lands on the same rejection.
var (
	errSCCRQNoMessageType         = &sccrqRejection{Log: logMalformedSCCRQ, Detail: "SCCRQ missing Message Type AVP"}
	errSCCRQMessageTypeNotFirst   = &sccrqRejection{Log: logMalformedSCCRQ, Detail: "SCCRQ Message Type AVP must be the first AVP"}
	errSCCRQMessageTypeLength     = &sccrqRejection{Log: logMalformedSCCRQ, Detail: "SCCRQ Message Type AVP must be 2 octets"}
	errSCCRQNoProtocolVersion     = &sccrqRejection{Log: logMalformedSCCRQ, Detail: "SCCRQ missing Protocol Version AVP"}
	errSCCRQProtocolVersionLength = &sccrqRejection{Log: logMalformedSCCRQ, Detail: "SCCRQ Protocol Version AVP must be 2 octets"}
	errSCCRQNoHostName            = &sccrqRejection{Log: logMalformedSCCRQ, Detail: "SCCRQ missing Host Name AVP"}
	errSCCRQHostNameEmpty         = &sccrqRejection{Log: logMalformedSCCRQ, Detail: "SCCRQ Host Name AVP must carry at least one octet"}
	errSCCRQNoFramingCapabilities = &sccrqRejection{Log: logMalformedSCCRQ, Detail: "SCCRQ missing Framing Capabilities AVP"}
	errSCCRQFramingLength         = &sccrqRejection{Log: logMalformedSCCRQ, Detail: "SCCRQ Framing Capabilities AVP must be 4 octets"}
	errSCCRQNoAssignedTunnelID    = &sccrqRejection{Log: logMalformedSCCRQ, Detail: "SCCRQ missing Assigned Tunnel ID AVP"}
	errSCCRQAssignedTunnelIDLen   = &sccrqRejection{Log: logMalformedSCCRQ, Detail: "SCCRQ Assigned Tunnel ID AVP must be 2 octets"}
)

// errZeroAssignedTunnelID reports an Assigned Tunnel ID AVP that carries
// the value 0. RFC 2661 Section 4.4.3: "The Assigned Tunnel ID is a 2
// octet non-zero unsigned integer", and Section 5.3: "The value of 0 for
// Session ID and Tunnel ID is special and MUST NOT be used as an Assigned
// Session ID or Assigned Tunnel ID."
//
// It is one of the rejections above: a value out of range, which Section 7.1
// names in the same sentence as an absent AVP.
var errZeroAssignedTunnelID = &sccrqRejection{Log: logZeroTunnelIDSCCRQ, Detail: "SCCRQ Assigned Tunnel ID AVP must be non-zero"}

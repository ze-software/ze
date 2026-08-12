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

// errZeroAssignedTunnelID reports an Assigned Tunnel ID AVP that carries
// the value 0. RFC 2661 Section 4.4.3: "The Assigned Tunnel ID is a 2
// octet non-zero unsigned integer", and Section 5.3: "The value of 0 for
// Session ID and Tunnel ID is special and MUST NOT be used as an Assigned
// Session ID or Assigned Tunnel ID."
//
// The reactor separates this parse failure from every other one because
// it is the only one it answers: an SCCRQ carrying a zero Assigned Tunnel
// ID gets a StopCCN, under the bound in reactor.go. Every other malformed
// TunnelID=0 body stays a silent drop.
var errZeroAssignedTunnelID = errors.New("l2tp: Assigned Tunnel ID AVP must be non-zero")

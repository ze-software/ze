// Design: plan/learned/744-ipsec-9-ikev2-eap-nat.md -- UDP encapsulation readiness
// RFC: rfc/short/rfc7296.md -- receive UDP-encapsulated ESP at any time (Section 2.23)
// Related: register.go -- the NAT-T listener that applies the socket option
// Related: doctor.go -- the sibling IPsec readiness check

package engine

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// udpEncapState records the outcome of the last attempt to give Ze a NAT-T socket
// that carries ESP-in-UDP. Two failures land here, and both have the same effect:
// the socket would not bind at all, or it bound and would not take the option.
//
// RFC 7296 Section 2.23 MUST: "all devices MUST be able to receive and process both
// UDP-encapsulated ESP and non-UDP-encapsulated ESP packets at any time." When the
// socket option fails, Ze holds port 4500 and the kernel stops decapsulating, so
// every encapsulated ESP datagram dies in user space. The tunnel looks established
// and carries nothing. When the bind fails, Ze does not hold port 4500 at all and
// nothing encapsulated arrives in the first place.
//
// A failure that is only logged at startup is one no operator sees an hour later.
// The state is therefore readable, and doctor reports it.
//
// Three values, and none of them is a bare boolean. A nil pointer means no NAT-T
// listener was ever attempted. A stored nil error means the socket is ready. A
// stored error means it is not. A boolean would read "not attempted" as "failed"
// (ai/rules/evidence.md).
var udpEncapState atomic.Pointer[udpEncapResult]

// udpEncapFailures counts the attempts that failed. It rises on every reconfigure
// that rebuilds the listener, so a reader sees a persistent failure and not one
// startup line.
var udpEncapFailures atomic.Uint64

type udpEncapResult struct {
	err error
}

// setUDPEncapFailure records the outcome of one attempt. A nil error records success.
func setUDPEncapFailure(err error) {
	udpEncapState.Store(&udpEncapResult{err: err})
}

// countUDPEncapFailure records one failed attempt.
func countUDPEncapFailure() {
	udpEncapFailures.Add(1)
}

// udpEncapFailureCount reports how many attempts have failed.
func udpEncapFailureCount() uint64 {
	return udpEncapFailures.Load()
}

// udpEncapReady reports whether the kernel will decapsulate ESP that arrives inside
// UDP on the NAT-T socket, and whether the question has an answer yet.
//
// The second return is false when no NAT-T listener has been attempted. A caller
// must not read that as readiness.
func udpEncapReady() (ready, known bool) {
	r := udpEncapState.Load()
	if r == nil {
		return false, false
	}
	return r.err == nil, true
}

// checkIPsecUDPEncap reports a NAT-T socket Ze will not receive encapsulated ESP
// through: one that would not bind, and one the kernel will not decapsulate through.
//
// It reports nothing until a NAT-T listener has been attempted. There is nothing to
// say about a socket no configuration asked for.
//
// Once one has been attempted, a failure is an ERROR. The tunnel establishes and
// carries no traffic, which is the quietest failure this subsystem has.
func checkIPsecUDPEncap(_ registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	ready, known := udpEncapReady()
	if !known || ready {
		return nil
	}
	r := udpEncapState.Load()
	var tb textbuf.Buffer
	tb.Str("ze cannot receive UDP-encapsulated ESP on the NAT-T port (").
		Uint16(transport.NATTPort).
		Str("), so a NAT-traversing tunnel will establish and carry no traffic: ").
		Err(r.err)
	return []rpc.DoctorCheckDiagnostic{{
		Code:     "doctor-ipsec-udp-encap",
		Severity: "error",
		Message:  tb.String(),
	}}
}

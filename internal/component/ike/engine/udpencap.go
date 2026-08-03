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

// udpEncapState records the outcome of the last attempt to prepare the NAT-T socket
// for ESP-in-UDP.
//
// RFC 7296 Section 2.23 MUST: "all devices MUST be able to receive and process both
// UDP-encapsulated ESP and non-UDP-encapsulated ESP packets at any time." When the
// socket option fails, Ze holds port 4500 and the kernel stops decapsulating, so
// every encapsulated ESP datagram dies in user space. The tunnel looks established
// and carries nothing.
//
// A failure that is only logged at startup is one no operator sees an hour later.
// The state is therefore readable, and doctor reports it.
//
// Three values, and none of them is a bare boolean. A nil pointer means the NAT-T
// listener never started. A stored nil error means the option is set. A stored error
// means it failed. A boolean would read "not attempted" as "failed"
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
// The second return is false when no NAT-T listener has started. A caller must not
// read that as readiness.
func udpEncapReady() (ready, known bool) {
	r := udpEncapState.Load()
	if r == nil {
		return false, false
	}
	return r.err == nil, true
}

// checkIPsecUDPEncap reports a NAT-T socket the kernel will not decapsulate through.
//
// It reports nothing until a NAT-T listener has run. There is nothing to say about a
// socket that does not exist.
//
// Once one has run, a failure is an ERROR. The tunnel establishes and carries no
// traffic, which is the quietest failure this subsystem has.
func checkIPsecUDPEncap(_ registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	ready, known := udpEncapReady()
	if !known || ready {
		return nil
	}
	r := udpEncapState.Load()
	var tb textbuf.Buffer
	tb.Str("udp encapsulation of ESP is not enabled on the NAT-T socket (port ").
		Uint16(transport.NATTPort).
		Str("), so UDP-encapsulated ESP will be dropped and a NAT-traversing tunnel will carry no traffic: ").
		Err(r.err)
	return []rpc.DoctorCheckDiagnostic{{
		Code:     "doctor-ipsec-udp-encap",
		Severity: "error",
		Message:  tb.String(),
	}}
}

// Design: docs/architecture/doctor-and-health-checks.md -- the runtime phase: checks only the daemon runs
// Related: doctor_runtime.go -- engineDoctorChecks, where this runtime check is declared and dispatched
// Related: ../transport/udp.go -- exemptFromIPsec and BypassError, which record the outcome this check reads
// RFC: rfc/short/rfc4301.md -- IKE traffic needs an explicit BYPASS (Section 5.2)

package engine

import (
	"cmp"
	"slices"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ikeSocketBypassCheckName names the check in the engine's Stage 1 registration and in
// the doctor callback's dispatch.
const ikeSocketBypassCheckName = "ipsec-ike-socket-bypass" //nolint:gosec // G101: a doctor check name, not a credential

// ikeSocketBypassCheckDecl declares the check at the runtime phase, so show doctor asks
// the engine's own process: the IKE sockets whose outcome it reads are opened and held
// there, internal and forked alike (ai/rules/plugins.md).
var ikeSocketBypassCheckDecl = rpc.DoctorCheckDecl{
	Name:  ikeSocketBypassCheckName,
	Phase: rpc.DoctorPhaseRuntime,
	Order: 753,
	Codes: []string{transport.CodeDoctorIPsecIKESocketBypass},
}

// ikeSocketBypassFailure is one open IKE socket the kernel refused its per-socket
// bypass: the address it listens on, whether it is the NAT-T socket, and the kernel's
// answer as the transport recorded it (BypassError), which decides the finding's cause
// and fix.
type ikeSocketBypassFailure struct {
	address string
	natT    bool
	err     error
}

// answerIKESocketBypass answers ipsec-ike-socket-bypass from the engine's process: one
// error for each IKE socket the engine holds open whose bypass the kernel refused.
//
// Safe for concurrent use: applyMu guards the socket pairs, as it guards every member
// the apply and the address worker write.
func answerIKESocketBypass(s *ikeEngineState) []rpc.DoctorCheckDiagnostic {
	s.applyMu.Lock()
	failures := s.ikeSocketBypassFailures()
	s.applyMu.Unlock()
	return checkIKESocketBypass(failures)
}

// ikeSocketBypassFailures collects the refused bypass of every IKE socket the engine
// holds: the primary pair, each literal pair and the shared NAT-T socket, each socket
// once, in address order. The caller holds applyMu.
func (s *ikeEngineState) ikeSocketBypassFailures() []ikeSocketBypassFailure {
	return s.ikeSocketBypassFailuresRead((*transport.UDPTransport).BypassError)
}

// ikeSocketBypassFailuresRead is ikeSocketBypassFailures with the record each socket is
// read through named: production reads the transport's own (BypassError), and a test
// hands a reader that refuses, so the dedupe is judged on a host where every socket
// carries its bypass. It calls read once for each distinct socket. The caller holds
// applyMu.
func (s *ikeEngineState) ikeSocketBypassFailuresRead(read func(*transport.UDPTransport) error) []ikeSocketBypassFailure {
	seen := make(map[*transport.UDPTransport]bool)
	var out []ikeSocketBypassFailure
	collect := func(t *transport.UDPTransport) {
		if t == nil || seen[t] {
			return
		}
		seen[t] = true
		err := read(t)
		if err == nil {
			return
		}
		out = append(out, ikeSocketBypassFailure{address: t.LocalAddr().String(), natT: t.IsNATT(), err: err})
	}
	collect(s.listeners.ike)
	collect(s.listeners.natt)
	for _, pair := range s.literalPairs {
		collect(pair.ike)
		collect(pair.natt)
	}
	collect(s.sharedNATT)
	slices.SortFunc(out, func(a, b ikeSocketBypassFailure) int {
		return cmp.Compare(a.address, b.address)
	})
	return out
}

// checkIKESocketBypass turns each refused bypass into its finding. Its cause and fix
// are the transport's own for the kernel's answer (transport.BypassCause,
// transport.BypassFix), the ones the socket's Warn line names: a missing capability, a
// kernel without XFRM and a security module each have their own fix, and the bypass is
// set when a socket opens, so only a socket opened again carries it.
func checkIKESocketBypass(failures []ikeSocketBypassFailure) []rpc.DoctorCheckDiagnostic {
	out := make([]rpc.DoctorCheckDiagnostic, 0, len(failures))
	for _, f := range failures {
		socket := "IKE socket"
		if f.natT {
			socket = "NAT-T IKE socket"
		}
		out = append(out, rpc.DoctorCheckDiagnostic{
			Code:     transport.CodeDoctorIPsecIKESocketBypass,
			Severity: severityError,
			Message: "The " + socket + " at " + f.address + " is not exempt from IPsec: the kernel refused its " +
				"per-socket bypass (" + f.err.Error() + "): " + transport.BypassCause(f.err) +
				". Under `vpn ipsec unmatched discard`, or a traffic " +
				"selector covering a peer's IKE address, the SPD captures Ze's own IKE on it and the tunnel " +
				"cannot be negotiated, rekeyed or torn down.",
			Fix: transport.BypassFix(f.err),
		})
	}
	return out
}

// Design: docs/architecture/doctor-and-health-checks.md -- the runtime phase: checks only the daemon runs
// Related: doctor_runtime.go -- engineDoctorChecks, where this runtime check is declared and dispatched
// Related: established.go -- runEstablished, which records the first Child SA's refusal, install or build
// Related: create_child.go -- finishChildCreate, which records a creation's refusal
// RFC: rfc/short/rfc7296.md -- a failed Child SA keeps its IKE SA (Section 2.21.2)

package engine

import (
	"errors"
	"maps"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// childInstallCheckName names the check in the engine's Stage 1 registration and in the
// doctor callback's dispatch.
const childInstallCheckName = "ipsec-child-install"

// childInstallCheckDecl declares the check at the runtime phase in the engine's Stage 1
// registration, so show doctor asks the engine's own process: the peer sessions whose
// refusals it reads live only there, internal and forked alike (ai/rules/plugins.md).
var childInstallCheckDecl = rpc.DoctorCheckDecl{
	Name:  childInstallCheckName,
	Phase: rpc.DoctorPhaseRuntime,
	Order: 743,
	Codes: []string{diagnostic.CodeDoctorIPsecChildInstallRefused},
}

// Fix lines of a refused Child SA whose cause carries no operator error of its own.
const (
	// childInstallFixReload reloads the configuration, after the change the message
	// names: every cause but the ones below is fixed there.
	childInstallFixReload = "ze signal reload"
	// childInstallFixLog lists the engine's own warnings, for a cause no configuration
	// produces (errChildEncapPort, errChildKeying), whose lines are what a report carries.
	childInstallFixLog = "ze show log recent level warn component ike"
)

// childRefusal is one Child SA this node refused after the peer accepted it: the error
// that refused it, when, and when the creation schedule asks the peer again. The value
// is never changed once stored, so a reader on another goroutine needs no lock.
type childRefusal struct {
	err error
	at  time.Time
	// retryAt is the next creation attempt the schedule named at the refusal. Zero when
	// none was scheduled, which no refusal path leaves today.
	retryAt time.Time
}

// recordChildRefused records the Child SA this node just refused for the doctor, with
// the next attempt the creation schedule names. MUST run on the owner loop, after the
// schedule was started or advanced for this refusal: it reads ps.childCreate.
// setChildSA clears the record once a Child SA installs.
func (ps *PeerSession) recordChildRefused(err error, now time.Time) {
	refusal := &childRefusal{err: err, at: now}
	if ps.childCreate.active {
		refusal.retryAt = ps.childCreate.next
	}
	ps.childRefused.Store(refusal)
}

// answerChildInstall answers ipsec-child-install from the engine's process.
func answerChildInstall(s *ikeEngineState) []rpc.DoctorCheckDiagnostic {
	return childInstallFindings(s.peers)
}

// childInstallFindings answers one error for each running peer whose last Child SA this
// node refused and that has had none installed since, in peer name order. A peer the
// configuration removed is gone from peers, and so is its refusal.
//
// RFC 7296 Section 2.21.2: "This failure does not automatically cause the IKE SA to be
// deleted." The IKE SA stays up and protects no traffic, so nothing in show vpn ipsec sa
// reads as a failure; this finding is where the operator learns why the peer carries none.
func childInstallFindings(peers map[string]*PeerSession) []rpc.DoctorCheckDiagnostic {
	type peerRefusal struct {
		name    string
		refusal *childRefusal
	}
	peersMu.RLock()
	refused := make([]peerRefusal, 0, len(peers))
	for _, name := range slices.Sorted(maps.Keys(peers)) {
		if refusal := peers[name].childRefused.Load(); refusal != nil {
			refused = append(refused, peerRefusal{name: name, refusal: refusal})
		}
	}
	peersMu.RUnlock()

	out := make([]rpc.DoctorCheckDiagnostic, 0, len(refused))
	for i := range refused {
		out = append(out, childInstallRefused(refused[i].name, refused[i].refusal))
	}
	return out
}

// childInstallRefused is the error for one peer whose Child SA this node refused.
func childInstallRefused(peer string, refusal *childRefusal) rpc.DoctorCheckDiagnostic {
	var tb textbuf.Buffer
	tb.Str("IPsec peer ").Str(peer).Str(" has no Child SA: at ").
		Str(refusal.at.UTC().Format(time.RFC3339)).
		Str(" the peer accepted one and this node refused it (")
	fix := childInstallFixReload
	if cause, ok := errors.AsType[*opererr.Error](refusal.err); ok {
		tb.Str(cause.Problem())
		if inner := errors.Unwrap(cause); inner != nil {
			tb.Str(": ").Err(inner)
		}
		tb.Str("). ").Str(cause.Why()).Str(".")
		fix = cause.Fix()
	} else {
		tb.Err(refusal.err).Str("). ").Str(childResponseRefusedFix(refusal.err)).Str(".")
		if errors.Is(refusal.err, errChildEncapPort) {
			fix = childInstallFixLog
		}
		if errors.Is(refusal.err, errChildKeying) {
			fix = childInstallFixLog
		}
	}
	tb.Str(" Ze kept the IKE SA and deleted the peer's half, so no traffic of this peer is protected until a Child SA exists.")
	if refusal.retryAt.IsZero() {
		tb.Str(" No further attempt is scheduled; the peer may still ask with CREATE_CHILD_SA.")
	} else {
		tb.Str(" Ze asks the peer again at ").Str(refusal.retryAt.UTC().Format(time.RFC3339)).
			Str(", then waits twice as long after each refusal, up to ").Str(childCreateMax.String()).
			Str("; the first attempt after the cause is removed installs the Child SA with no restart.")
	}
	return rpc.DoctorCheckDiagnostic{
		Code:     diagnostic.CodeDoctorIPsecChildInstallRefused,
		Severity: severityError,
		Message:  tb.String(),
		Fix:      fix,
	}
}

// Design: docs/guide/monitoring.md -- retained PPP cleanup failures
// Related: ../doctor_cleanup.go -- registered runtime doctor and health checks

package ppp

import (
	"crypto/sha256"
	"fmt"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// CleanupOperation identifies the resource whose removal failed. The zero
// value is invalid; callers MUST use a named operation from this package.
type CleanupOperation uint8

const (
	CleanupUnspecified CleanupOperation = iota
	CleanupSession
	CleanupTransport
	CleanupIPv6
	CleanupNetwork
)

func (operation CleanupOperation) String() string {
	switch operation {
	case CleanupSession:
		return "session"
	case CleanupTransport:
		return "transport"
	case CleanupIPv6:
		return "IPv6"
	case CleanupNetwork:
		return "network"
	case CleanupUnspecified:
		panic("BUG: PPP cleanup operation was not specified")
	default:
		panic("BUG: invalid PPP cleanup operation")
	}
}

// CleanupReportSource is the report-bus owner shared by all PPP transports.
const CleanupReportSource = "ppp"

// CleanupFailureFix opens the retained cause and affected interface. Removal
// must be confirmed after the backend is repaired; a new session is no proof.
const CleanupFailureFix = "show warnings"

// ReportCleanupFailure records unconfirmed removal of the actual created
// interface's resources. Callers MUST pass the owner's Identity, never resolve
// another index or invent one. Safe for concurrent use. Nil records nothing and
// never clears an earlier failure, including one for this same identity.
//
// The report bus owns retention and its bound. There is deliberately no
// automatic recovery claim: Close can cache its error, and an unrelated
// successful session cannot prove the failed resource was removed.
func ReportCleanupFailure(identity SessionIdentity, operation CleanupOperation, err error) {
	if err == nil {
		return
	}
	step := operation.String()
	subject := fmt.Sprintf("%s/%d/%s", identity.InterfaceName, identity.NativeIndex, step)
	if len(subject) > report.SubjectMax {
		// The bus refuses oversized subjects. Keep full identity in Detail and
		// a bounded distinct key rather than silently losing its warning.
		digest := sha256.Sum256([]byte(identity.InterfaceName))
		subject = fmt.Sprintf("%x/%d/%s", digest, identity.NativeIndex, step)
	}
	message := fmt.Sprintf("PPP %s cleanup failed for interface %s (index %d); removal is unconfirmed",
		step, identity.InterfaceName, identity.NativeIndex)
	report.RaiseWarningFix(CleanupReportSource, diagnostic.CodeDoctorPPPCleanup, subject,
		report.FitMessage(message), CleanupFailureFix, map[string]any{
			"interface": identity.InterfaceName, "native-index": identity.NativeIndex,
			"operation": step, "error": err.Error(),
		})
}

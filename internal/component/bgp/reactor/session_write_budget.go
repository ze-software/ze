// Design: docs/architecture/core-design.md -- the write budget every session write carries
// Related: session_write.go -- retireWrite, which records a failed write through recordWriteFailure
// Related: session_write_mutex.go -- armWriteDeadline, which records the budget a writer armed
// Related: register_diagnostic.go -- the event code a budget expiry raises
// Related: peer_close.go -- sessionDroppedOwed, which leaves the close to the event raised here

package reactor

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/report"
)

// ErrWriteBudgetExpired is the close reason of a session ze closed because the
// peer accepted no data for a whole write budget: a write to its socket was
// still blocked when the budget its writer armed ran out. The recorded reason
// wraps it together with the write's own error, so errors.Is answers for both.
var ErrWriteBudgetExpired = errors.New("the peer accepted no data for the write budget")

// writeBudgetExpiryFix is what an operator does about a write budget expiry.
// The cause is on the peer, so no ze command clears it. It fits report.FixMax.
const writeBudgetExpiryFix = "on the peer, find why its BGP process stopped reading (CPU load, a stalled process); " +
	"ze connects to it again by itself. ze.fwd.write.deadline sets the budget of a route write"

// recordWriteFailure records why a failed write closes the session, and
// reports a write budget expiry to the operator. It is the one place that
// decides a failed write was a budget expiry, so no writer reports its own.
// It MUST run under writeMu, and it takes no other session lock.
//
// The expiry is reported only when it became the session's close reason. When
// another close recorded its reason first (a teardown whose NOTIFICATION then
// met the shut window), that close reports itself, so one close gives one
// event.
func (s *Session) recordWriteFailure(err error) {
	if !s.writeBudgetPassed(err) {
		s.setCloseReason(err)
		return
	}
	budget := s.writeBudget
	if !s.setCloseReason(fmt.Errorf("%w of %s: %w", ErrWriteBudgetExpired, budget, err)) {
		return
	}
	s.reportWriteBudgetExpiry(budget)
}

// writeBudgetPassed reports whether err is a write that waited out the budget
// its writer armed. A deadline error alone does not say so: a canceled request
// expires the socket's deadline early to end its own write
// (requestWriteDeadline), and that write met no budget. Nor did a write whose
// armed bound was its request's own deadline, earlier than the session budget
// (writeBoundRequested): the request set that bound, ze.fwd.write.deadline did
// not, and the request answers for it (sendUpdateCounted). The caller MUST
// hold writeMu.
func (s *Session) writeBudgetPassed(err error) bool {
	if s.writeBudgetArmed {
		if s.writeBoundRequested {
			return false
		}
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return !s.clock.Now().Before(s.writeBudgetEnd)
		}
	}
	return false
}

// reportWriteBudgetExpiry tells the operator a write budget expiry closed the
// session: one WARN line, the same problem as an error event on the report
// bus, which `show doctor` and `show errors` print, and the peer's last-error
// record. It runs under writeMu on the path of a write that already failed.
// The log call and the raise wait on nothing a peer holds: the raise takes the
// bus's own mutex for one append.
func (s *Session) reportWriteBudgetExpiry(budget time.Duration) {
	peerAddr := s.settings.Address.String()
	sessionLogger().Warn("BGP session closed: the peer accepted no data for the write budget; ze connects to it again by itself",
		"peer", peerAddr, "budget", budget,
		"fix", writeBudgetExpiryFix, "code", codeDoctorBGPPeerNotReading)
	report.RaiseErrorFix(reportSourceBGP, codeDoctorBGPPeerNotReading, peerAddr,
		"BGP session closed: peer "+peerAddr+" accepted no data for "+budget.String()+
			", the write budget; ze connects to it again by itself",
		writeBudgetExpiryFix, map[string]any{"budget": budget.String()})
	if s.onWriteBudgetExpired != nil {
		s.onWriteBudgetExpired()
	}
}

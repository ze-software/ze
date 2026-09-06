// Design: docs/architecture/exabgp-bridge.md -- the scripts the bridge runs
// Overview: script.go -- the supervisor that asks the limiter before each fork
//
// The restart policy is ExaBGP's, read from its own code rather than invented
// here (src/exabgp/reactor/api/processes.py, Processes._start and
// Processes._handle_problem).

package bridgerun

import "time"

// respawnWindow and respawnMax are ExaBGP's restart limit.
//
// ExaBGP buckets the clock with `respawn_timemask = 0xFFFFFF - 0b111111`, which
// clears the low six bits of the epoch second, so one bucket is 64 seconds
// wide. It counts the forks in the current bucket, and the sixth one is
// refused: `respawn_number` is 5, and `if self._respawning[process][around_now]
// > self.respawn_number` terminates the process and leaves it stopped. The
// first fork is counted, so a script gets one start and four restarts inside
// one bucket.
//
// There is no backoff. ExaBGP forks again the moment it reads the EOF, and the
// bucket count is the whole of the limit.
//
// The one thing not carried over is the `0xFFFFFF` half of ExaBGP's mask, which
// truncates the clock to 24 bits and wraps the bucket every 194 days. That is
// an artifact of writing the mask as a subtraction, not a policy.
const (
	respawnWindow = 64 * time.Second
	respawnMax    = 5
)

// respawnLimiter counts the forks of one script inside the current window.
//
// Not safe for concurrent use: one script's supervisor is its only caller.
type respawnLimiter struct {
	window int64
	forks  int
}

// count records a fork at now and answers whether it is inside the limit.
//
// ExaBGP forks first and terminates the child when the count says it should not
// have. The answer is the same either way, and refusing before the fork is the
// version with no process to clean up.
func (l *respawnLimiter) count(now time.Time) bool {
	seconds := int64(respawnWindow / time.Second)
	window := now.Unix() - now.Unix()%seconds
	if window != l.window {
		l.window = window
		l.forks = 0
	}
	l.forks++
	return l.forks <= respawnMax
}

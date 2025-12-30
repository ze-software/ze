package functional

import (
	"fmt"
	"syscall"
)

// LimitCheck holds ulimit check results.
type LimitCheck struct {
	Current     uint64
	Max         uint64
	Required    uint64
	Raised      bool
	RaisedTo    uint64
	RaiseNeeded bool
}

// CheckUlimit ensures sufficient file descriptors for parallel tests.
// Each test spawns: zebgp (may fork) + zebgp-peer = ~20 FDs per concurrent test.
// With parallel=4: need 4 × 20 = 80 FDs minimum, recommend 256+.
func CheckUlimit(parallel int) (*LimitCheck, error) {
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		return nil, fmt.Errorf("getrlimit: %w", err)
	}

	const fdsPerTest = 20 // Conservative: sockets, pipes, files
	const minRecommended = 256

	needed := uint64(parallel) * fdsPerTest //nolint:gosec // parallel is always small (1-16)
	if needed < minRecommended {
		needed = minRecommended
	}

	check := &LimitCheck{
		Current:  limit.Cur,
		Max:      limit.Max,
		Required: needed,
	}

	if limit.Cur >= needed {
		return check, nil
	}

	check.RaiseNeeded = true

	// Try to raise soft limit
	newLimit := needed
	if newLimit > limit.Max {
		newLimit = limit.Max
	}

	limit.Cur = newLimit
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		return check, fmt.Errorf("ulimit too low: have %d, need %d (run: ulimit -n %d): %w",
			check.Current, needed, needed, err)
	}

	check.Raised = true
	check.RaisedTo = newLimit

	// Verify it was raised
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err == nil {
		check.Current = limit.Cur
	}

	return check, nil
}

// String returns a human-readable status.
func (l *LimitCheck) String() string {
	if l.Raised {
		return fmt.Sprintf("raised %d → %d", l.Current-l.RaisedTo+l.Current, l.RaisedTo)
	}
	if l.RaiseNeeded {
		return fmt.Sprintf("low (%d < %d)", l.Current, l.Required)
	}
	return fmt.Sprintf("ok (%d)", l.Current)
}

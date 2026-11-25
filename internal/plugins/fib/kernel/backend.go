// Design: docs/architecture/core-design.md -- FIB backend abstraction
// Overview: fibkernel.go -- FIB kernel plugin
// Related: backend_linux.go -- Linux kernel backend
// Related: backend_other.go -- noop backend for non-Linux platforms
// Related: monitor.go -- external route change handling
//
// Platform-independent backend helpers and the showInstalled method.
// OS-specific backends provide newBackend() via build tags.
// Caller MUST call close() on the returned backend when done.
package fibkernel

import (
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// supersededDeleteError reports that replaceOwnedRoute installed the new route
// in table, but deleting one or more superseded Ze routes at other priorities
// in that same table failed. The caller MUST record the newly installed route
// and retain the superseded table entries for retry and flush.
type supersededDeleteError struct {
	table uint32 // forwarding table (0 for main)
	count int    // number of superseded Ze routes whose delete failed
	err   error
}

func (e *supersededDeleteError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	var tb textbuf.Buffer
	tb.Str("remove ").Int(int64(e.count)).Str(" superseded route(s) in table ").Uint(uint64(e.table))
	return tb.String()
}

func (e *supersededDeleteError) Unwrap() error {
	return e.err
}

// sweepDelay is the time to wait before sweeping stale routes after startup.
// Allows BGP reconvergence to refresh matching routes.
const sweepDelay = 30 * time.Second

// showInstalled returns the currently installed routes as JSON.
func (f *fibKernel) showInstalled() any {
	f.mu.RLock()
	defer f.mu.RUnlock()

	type entry struct {
		Prefix  string `json:"prefix"`
		NextHop string `json:"next-hop"`
	}

	entries := make([]entry, 0, len(f.installed))
	for prefix, nextHop := range f.installed {
		entries = append(entries, entry{Prefix: prefix, NextHop: nextHop})
	}

	return entries
}

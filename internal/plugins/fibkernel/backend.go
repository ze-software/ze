// Design: docs/architecture/core-design.md -- FIB backend abstraction
// Overview: fibkernel.go -- FIB kernel plugin
// Related: backend_linux.go -- Linux netlink backend
// Related: backend_other.go -- noop backend for non-Linux platforms
// Related: monitor.go -- external route change handling
//
// Platform-independent backend helpers and the showInstalled method.
// OS-specific backends provide newBackend() via build tags.
// Caller MUST call close() on the returned backend when done.
package fibkernel

import (
	"encoding/json"
	"time"
)

// rtprotZE is the custom rtm_protocol ID used for all ze-installed routes.
// Linux: identifies routes in the kernel routing table as belonging to ze.
// RFC 3549 Section 3.1.1: protocol field in rtmsg.
const rtprotZE = 250

// sweepDelay is the time to wait before sweeping stale routes after startup.
// Allows BGP reconvergence to refresh matching routes.
const sweepDelay = 30 * time.Second

// sweepTimer returns a channel that fires after the sweep delay.
func sweepTimer() <-chan time.Time {
	return time.After(sweepDelay)
}

// showInstalled returns the currently installed routes as JSON.
func (f *fibKernel) showInstalled() string {
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

	data, err := json.Marshal(entries)
	if err != nil {
		return "[]"
	}
	return string(data)
}

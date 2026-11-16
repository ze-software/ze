//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- retain the captured namespace without an unnecessary privileged switch.
// Related: espform_firewall_kernel_linux.go -- pinned watch socket creation.
// Related: espform_fastpath_linux.go -- pinned link attachment worker.
package dataplane

import (
	"fmt"

	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

// espFormNetNSCurrent checks whether ns names the calling thread's namespace.
// The caller MUST pin its OS thread before calling and keep it pinned through the
// operation that uses the result. The caller MUST keep ns open through that operation.
// It is safe for concurrent use when each caller follows those obligations.
func espFormNetNSCurrent(ns netns.NsHandle) (bool, error) {
	var target unix.Stat_t
	// Validate the retained descriptor before anything could reuse a closed number.
	if err := unix.Fstat(int(ns), &target); err != nil {
		return false, fmt.Errorf("read the captured network namespace identity: %w", err)
	}
	var current unix.Stat_t
	// Stat the pinned thread's namespace without allocating a replacement handle.
	path := fmt.Sprintf("/proc/self/task/%d/ns/net", unix.Gettid())
	if err := unix.Stat(path, &current); err != nil {
		return false, fmt.Errorf("read the current thread's network namespace identity: %w", err)
	}
	if target.Dev != current.Dev {
		return false, nil
	}
	return target.Ino == current.Ino, nil
}

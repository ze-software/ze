// Design: docs/architecture/system-architecture.md -- capabilities without root
// Overview: membership.go -- ServiceMembership, DaemonServiceMembership, CapabilityRemedy
// Related: membership_linux.go -- the /proc/self/cgroup read on Linux

//go:build !linux

package privilege

import (
	"errors"
	"runtime"
)

// readServiceMembership answers Unknown: systemd and cgroups are Linux's, so a
// process on another platform cannot be shown to run under ServiceUnit.
func readServiceMembership() (ServiceMembership, error) {
	return MembershipUnknown, errors.New("systemd units and cgroups exist on Linux only, and ze runs on " + runtime.GOOS)
}

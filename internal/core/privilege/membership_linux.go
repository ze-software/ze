// Design: docs/architecture/system-architecture.md -- capabilities without root
// Overview: membership.go -- ServiceMembership, DaemonServiceMembership, CapabilityRemedy
// Related: membership_other.go -- the same probe off Linux

//go:build linux

package privilege

import "os"

// readServiceMembership reads this process's membership from /proc/self/cgroup.
func readServiceMembership() (ServiceMembership, error) {
	data, err := os.ReadFile(selfCgroupPath)
	return membershipFromRead(selfCgroupPath, data, err)
}

// Design: docs/architecture/core-design.md -- nftables host-safety gate (R-2)
// Overview: backend_linux.go -- Apply calls refuseHostNetnsFirewall before any kernel op

//go:build linux

package firewallnft

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

// hostNetnsFirewallEnv carries the inode of the HOST network namespace. The
// functional-test runner's per-test netns launch mode (ZE_TEST_NETNS, Fix B of
// spec-netlink-ci-harness) sets it on the ze daemon it spawns; production never
// sets it, so the guard below is a no-op outside the test harness.
const hostNetnsFirewallEnv = "ZE_TEST_NETNS_HOST"

// errHostNetnsFirewall is returned when a netns-isolated test run would program
// nft in the HOST namespace: the per-test netns isolation silently failed and
// applying would reprogram the operator's real firewall. This is the lockout
// failure mode R-2 guards against (prototyping locked the operator out of the box).
var errHostNetnsFirewall = errors.New(
	"firewallnft: refusing to program the host firewall: ZE_TEST_NETNS is active but this process is in the host network namespace (per-test netns isolation failed)")

// refuseHostNetnsFirewall aborts an nft Apply when the test harness marked this
// as a netns-isolated run but the process is NOT actually isolated. It fails
// CLOSED: when the env is set but isolation cannot be positively proven
// (malformed inode, unreadable /proc/self/ns/net, or the current netns matches
// the recorded host netns) it refuses. When the env is unset it returns nil --
// the untouched production path.
func refuseHostNetnsFirewall() error {
	hostStr := os.Getenv(hostNetnsFirewallEnv)
	if hostStr == "" {
		return nil
	}
	hostInode, err := strconv.ParseUint(hostStr, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: %s=%q is malformed", errHostNetnsFirewall, hostNetnsFirewallEnv, hostStr)
	}
	var st unix.Stat_t
	if err := unix.Stat("/proc/self/ns/net", &st); err != nil {
		return fmt.Errorf("%w: cannot read own network namespace: %w", errHostNetnsFirewall, err)
	}
	if st.Ino == hostInode {
		return errHostNetnsFirewall
	}
	return nil
}

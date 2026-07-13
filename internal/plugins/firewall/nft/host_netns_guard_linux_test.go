//go:build linux

package firewallnft

import (
	"errors"
	"os"
	"strconv"
	"testing"

	"golang.org/x/sys/unix"
)

// TestRefuseHostNetnsFirewall covers the R-2 host-safety gate: when the
// functional-test netns launch mode is active (ZE_TEST_NETNS_HOST set) ze must
// refuse to program nft unless it can positively prove it is in an isolated
// namespace, so a run whose netns isolation silently failed can never reprogram
// the operator's host firewall. Needs /proc/self/ns/net (linux; exercised in QEMU).
func TestRefuseHostNetnsFirewall(t *testing.T) {
	var st unix.Stat_t
	if err := unix.Stat("/proc/self/ns/net", &st); err != nil {
		t.Skipf("cannot read /proc/self/ns/net: %v", err)
	}
	cur := st.Ino

	t.Run("env unset allows production path", func(t *testing.T) {
		os.Unsetenv(hostNetnsFirewallEnv)
		if err := refuseHostNetnsFirewall(); err != nil {
			t.Fatalf("env unset must allow, got %v", err)
		}
	})

	t.Run("current netns equals recorded host refuses", func(t *testing.T) {
		t.Setenv(hostNetnsFirewallEnv, strconv.FormatUint(cur, 10))
		if err := refuseHostNetnsFirewall(); !errors.Is(err, errHostNetnsFirewall) {
			t.Fatalf("host-netns run must refuse, got %v", err)
		}
	})

	t.Run("distinct netns inode allows isolated run", func(t *testing.T) {
		t.Setenv(hostNetnsFirewallEnv, strconv.FormatUint(cur+1, 10))
		if err := refuseHostNetnsFirewall(); err != nil {
			t.Fatalf("isolated run must allow, got %v", err)
		}
	})

	t.Run("malformed env fails closed", func(t *testing.T) {
		t.Setenv(hostNetnsFirewallEnv, "not-a-number")
		if err := refuseHostNetnsFirewall(); !errors.Is(err, errHostNetnsFirewall) {
			t.Fatalf("malformed env must fail closed, got %v", err)
		}
	})
}

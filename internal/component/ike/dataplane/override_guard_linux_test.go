//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the application override guard
// Related: override_guard_linux.go -- cgroup2RootFrom; xfrm_linux.go -- policyDirFromKernel

package dataplane

import (
	"errors"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
)

// TestPolicyInfoMapsSocketDirections proves the journal row in
// plan/journal/kernel-value-outside-the-decoded-set.md fixed (AC-20): the dump's
// socket directions 3 to 5 read in, out and fwd with SPD "socket", a socket policy
// carrying Ze's mark reads esp-form-socket and any other socket-override, and a
// direction outside the six reads unknown rather than a direction it does not carry.
//
// VALIDATES: policyDirFromKernel, socketPolicyRole and SPDName.
// PREVENTS: the old offset mapping, which listed every socket policy as unknown.
func TestPolicyInfoMapsSocketDirections(t *testing.T) {
	b := &xfrmBackend{}
	for _, tc := range []struct {
		kernel   netlink.Dir
		priority int
		dir      SADir
		spd      string
		role     PolicyRole
	}{
		{netlink.XFRM_SOCKET_IN, PriorityESPFormSocket, SADirIn, spdNameSocket, PolicyRoleESPFormSocket},
		{netlink.XFRM_SOCKET_OUT, 0, SADirOut, spdNameSocket, PolicyRoleSocketOverride},
		{netlink.XFRM_SOCKET_FWD, 7, SADirFwd, spdNameSocket, PolicyRoleSocketOverride},
		{netlink.XFRM_DIR_OUT, 0, SADirOut, spdNameNode, PolicyRoleUnspecified},
	} {
		got := b.policyInfoFromKernel(&netlink.XfrmPolicy{Dir: tc.kernel, Priority: tc.priority})
		if got.Dir != tc.dir {
			t.Errorf("kernel dir %d: dir %d, want %d", tc.kernel, got.Dir, tc.dir)
		}
		if spd := got.SPDName(); spd != tc.spd {
			t.Errorf("kernel dir %d: spd %q, want %q", tc.kernel, spd, tc.spd)
		}
		if got.Role != tc.role {
			t.Errorf("kernel dir %d: role %v, want %v", tc.kernel, got.Role, tc.role)
		}
	}
	if got := b.policyInfoFromKernel(&netlink.XfrmPolicy{Dir: netlink.XFRM_SOCKET_FWD + 1}); got.Dir != 0 || got.PerSocket {
		t.Errorf("direction outside the set: dir %d, per-socket %v; want the zero direction", got.Dir, got.PerSocket)
	}
}

// TestCgroup2RootFromMountinfo proves A8-3: the cgroup2 mount point is read from
// mountinfo wherever it is mounted, and a host with none is refused naming the fix.
//
// VALIDATES: cgroup2RootFrom.
// PREVENTS: a guard attached at a fixed path a hybrid host does not mount.
func TestCgroup2RootFromMountinfo(t *testing.T) {
	const hybrid = "25 1 0:22 / /sys/fs/cgroup ro,nosuid shared:9 - tmpfs tmpfs ro\n" +
		"26 25 0:23 / /sys/fs/cgroup/unified rw,nosuid shared:10 - cgroup2 cgroup2 rw\n"
	root, err := cgroup2RootFrom(strings.NewReader(hybrid))
	if err != nil {
		t.Fatalf("hybrid layout: %v", err)
	}
	if root != "/sys/fs/cgroup/unified" {
		t.Fatalf("root = %q, want /sys/fs/cgroup/unified", root)
	}
	_, err = cgroup2RootFrom(strings.NewReader("25 1 0:22 / /sys/fs/cgroup rw - cgroup cgroup rw\n"))
	if !errors.Is(err, errNoCgroup2) {
		t.Fatalf("no cgroup2: err = %v, want errNoCgroup2", err)
	}
}

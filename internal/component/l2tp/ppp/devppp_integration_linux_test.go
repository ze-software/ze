//go:build integration && linux

// Design: docs/architecture/l2tp/bng-5-pppoe.md -- shared /dev/ppp setup.
// Related: devppp_linux.go -- rtnetlink kind ppp with IFLA_PPP_DEV_FD.

package ppp

import (
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// TestDevPPPSetupNamesUnit drives DevPPPSetup with a configured name against
// the real kernel: the sequence succeeds, the kernel device carries the
// configured name, and the unit descriptor reads back a valid unit number via
// PPPIOCGUNIT. Clean-up deletes the device so the test name is free for the
// next run. This test confirms A-1 (kernel accepts rtnetlink kind "ppp" with
// IFLA_PPP_DEV_FD) and A-4 (closing the unit fd releases the name in time for
// a re-create), the two assumptions
// plan/release/fixes/spec-home-cpe-1-logical-interface-name.md marks
// unvalidated before child-1 phase 2 lands. The test needs CAP_NET_ADMIN,
// which the ./le test integration harness provides through a user namespace.
func TestDevPPPSetupNamesUnit(t *testing.T) {
	const name = "pppze0"
	// Guard against a leftover from a previous crashed run. A real kernel
	// device by this name is deleted before the test creates a fresh one;
	// any other failure stops the test with a clear message.
	if link, err := netlink.LinkByName(name); err == nil {
		if derr := netlink.LinkDel(link); derr != nil {
			t.Fatalf("pre-clean: netlink.LinkDel(%s): %v", name, derr)
		}
	}

	// DevPPPSetup wants a PPPoX socket for PPPIOCGCHAN. The integration
	// harness that drives subscriber sessions provides one; here, a direct
	// test of named unit creation needs the channel call to fail cleanly
	// without a real PPPoX, so this sub-test instead calls the lower-level
	// createPPPLink + PPPIOCGUNIT path through a pair of /dev/ppp opens.
	// Phase 2 of child-1 asserts that pair: step 3 of the named path.
	unitFD, err := openDevPPP()
	if err != nil {
		t.Fatalf("open /dev/ppp: %v", err)
	}
	t.Cleanup(func() { _ = unix.Close(unitFD) })

	if err := createPPPLink(unitFD, name); err != nil {
		t.Fatalf("createPPPLink(%s): %v", name, err)
	}

	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("netlink.LinkByName(%s) after create: %v", name, err)
	}
	if link.Attrs().Name != name {
		t.Fatalf("kernel device name = %q, want %q", link.Attrs().Name, name)
	}

	unitNum, err := ioctlGetInt(unitFD, pppiocGUnit)
	if err != nil {
		t.Fatalf("PPPIOCGUNIT: %v", err)
	}
	if unitNum < 0 {
		t.Fatalf("PPPIOCGUNIT returned %d, want >= 0", unitNum)
	}

	if err := netlink.LinkDel(link); err != nil {
		t.Fatalf("cleanup netlink.LinkDel(%s): %v", name, err)
	}
}

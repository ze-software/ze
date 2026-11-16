// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- link following under the daemon's capability contract.
// Related: espform_fastpath_linux.go -- start, follow and stop, the lifecycle under test.
// Related: unit_capabilities_integration_linux_test.go -- the exact effective-capability assertion.

//go:build integration && linux

package dataplane

import (
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/core/privilege"
)

const (
	fastPathUnitCapsEnv    = "ZE_TEST_ESP_FORM_LINK_UNIT_CAPS"
	fastPathUnitCapsReady  = "unit-capabilities: followed and detached links"
	fastPathUnitCapsFirst  = "zefp-caps0"
	fastPathUnitCapsSecond = "zefp-caps1"
)

// TestESPFormFastPathFollowsLinksUnderTheUnitCapabilities exercises the real
// worker as a non-root daemon: attach an existing link, follow a new link and its
// deletion, then detach the existing link at stop. The helper's fresh namespace
// prevents any operation from reaching the guest's other links.
//
// MUTATION: unconditionally enter the captured namespace in follow, even when
// it is already the current namespace: the initial-link wait fails with EPERM
// under the unit's capabilities, which do not include CAP_SYS_ADMIN.
//
//ze:guest-root
func TestESPFormFastPathFollowsLinksUnderTheUnitCapabilities(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires privileged QEMU to start a non-root helper with the unit's ambient capabilities in its own network namespace")
	}
	capabilities := privilege.DaemonCapabilities()
	ambient := make([]uintptr, 0, len(capabilities))
	for _, capability := range capabilities {
		ambient = append(ambient, uintptr(capability.Bit))
	}
	command := exec.CommandContext(t.Context(), os.Args[0],
		"-test.run=^TestESPFormFastPathLinkFollowerUnitCapsHelper$", "-test.count=1", "-test.v")
	command.Env = append(os.Environ(), fastPathUnitCapsEnv+"=1")
	command.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWNET,
		Credential:  &syscall.Credential{Uid: unitCapsUnprivilegedID, Gid: unitCapsUnprivilegedID},
		AmbientCaps: ambient,
	}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("follow links as the non-root daemon: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), fastPathUnitCapsReady) {
		t.Fatalf("link-follower helper reported no completed lifecycle:\n%s", output)
	}
	t.Logf("helper output:\n%s", output)
}

// TestESPFormFastPathLinkFollowerUnitCapsHelper is the subprocess entry point.
// The parent checks its exit and completion marker; ordinary package runs do
// nothing here because they do not carry fastPathUnitCapsEnv.
// test-asserts-nothing: subprocess entry point; the parent asserts its result.
func TestESPFormFastPathLinkFollowerUnitCapsHelper(t *testing.T) {
	if os.Getenv(fastPathUnitCapsEnv) == "" {
		return
	}
	requireExactlyTheUnitCapabilities(t)
	first := fastPathUnitCapsDummy(t, fastPathUnitCapsFirst)
	fast := newESPFormFastPath(nil)
	t.Cleanup(fast.stop)
	fast.start()
	waitForFastPath(t, fast, "initial link attached under the unit's capabilities", func(state ESPFormFastPathState) bool {
		return slices.Contains(state.Attached, fastPathUnitCapsFirst)
	})
	if programs := fastPathIngress(t, first.Attrs().Index); len(programs) != 1 {
		t.Fatalf("initial link has TCX programs %v, want the fast path alone", programs)
	}

	second := fastPathUnitCapsDummy(t, fastPathUnitCapsSecond)
	waitForFastPath(t, fast, "new link attached under the unit's capabilities", func(state ESPFormFastPathState) bool {
		return slices.Contains(state.Attached, fastPathUnitCapsSecond)
	})
	if programs := fastPathIngress(t, second.Attrs().Index); len(programs) != 1 {
		t.Fatalf("new link has TCX programs %v, want the fast path alone", programs)
	}
	if err := netlink.LinkDel(second); err != nil {
		t.Fatalf("delete the followed link: %v", err)
	}
	waitForFastPath(t, fast, "deleted link forgotten under the unit's capabilities", func(state ESPFormFastPathState) bool {
		return !slices.Contains(state.Attached, fastPathUnitCapsSecond)
	})

	fast.stop()
	if programs := fastPathIngress(t, first.Attrs().Index); len(programs) != 0 {
		t.Fatalf("stopped fast path leaves TCX programs %v on the initial link", programs)
	}
	state, err := fast.state()
	if err != nil {
		t.Fatalf("read stopped fast-path state: %v", err)
	}
	if state.Status != ESPFormFastPathIdle {
		t.Fatalf("stopped fast-path status %s, want Idle", state.Status)
	}
	if len(state.Attached) != 0 {
		t.Fatalf("stopped fast path still reports attached links %v", state.Attached)
	}
	fast.stop()
	t.Log(fastPathUnitCapsReady)
}

// fastPathUnitCapsDummy creates a link inside the helper's private namespace.
func fastPathUnitCapsDummy(t *testing.T, name string) netlink.Link {
	t.Helper()
	attributes := netlink.NewLinkAttrs()
	attributes.Name = name
	if err := netlink.LinkAdd(&netlink.Dummy{LinkAttrs: attributes}); err != nil {
		t.Fatalf("create %s with the unit's capabilities: %v", name, err)
	}
	device, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("read the created link %s: %v", name, err)
	}
	if err := netlink.LinkSetUp(device); err != nil {
		t.Fatalf("bring %s up: %v", name, err)
	}
	return device
}

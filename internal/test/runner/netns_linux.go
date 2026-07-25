//go:build linux

// Design: docs/architecture/testing/ci-format.md -- per-test netns launch mode (Fix B)
// Overview: runner_exec.go -- runOrchestrated enters a per-test netns before spawning ze
// Related: netns_linux_test.go -- validates the fork-inherits-netns assumption (A-5)

package runner

import (
	"fmt"
	"runtime"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// enterTestNetns locks the calling goroutine to its OS thread, switches it into
// a fresh named network namespace, and brings loopback up inside it. It returns
// a restore closure (call via defer) and the inode of the ORIGINAL (host)
// network namespace, which the runner passes to ze as the R-2 host-safety gate
// reference (see refuseHostNetnsFirewall in the firewall backend).
//
// It mirrors internal/component/iface/integration_helpers_linux_test.go's
// withNetNS, but returns an error instead of skipping so the runner can fail the
// suite loudly (AC-4) rather than silently pass: a netlink suite that cannot get
// its own namespace must never fall through and program the host firewall.
//
// The thread stays locked for the whole test lifetime: ze, ze-peer, and driver.py
// are fork+exec'd from this goroutine and inherit the thread's netns (assumption
// A-5, validated by TestNetnsLaunchChildInheritsNamespace), so they share one
// throwaway namespace and reach each other over 127.0.0.1. restore() must run on
// the same thread, so callers MUST defer it (never hand it to another goroutine).
func enterTestNetns(name string) (restore func(), hostInode uint64, err error) {
	runtime.LockOSThread()

	orig, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		return nil, 0, fmt.Errorf("get current network namespace: %w", err)
	}

	var st unix.Stat_t
	if err := unix.Fstat(int(orig), &st); err != nil {
		orig.Close() //nolint:errcheck // best-effort close on cleanup/error path
		runtime.UnlockOSThread()
		return nil, 0, fmt.Errorf("stat host network namespace: %w", err)
	}
	hostInode = st.Ino

	// Best-effort: clear a stale namespace of the same name left by a prior test
	// that was SIGKILLed before its restore ran. Per-test names derive from the
	// recycled port, so a leaked bind-mount would otherwise fail NewNamed with
	// EEXIST on a later same-named test. Names are unique per concurrent test
	// (port), so this never removes a live peer's namespace.
	netns.DeleteNamed(name) //nolint:errcheck // best-effort; absent name is fine

	newNS, err := netns.NewNamed(name)
	if err != nil {
		orig.Close() //nolint:errcheck // best-effort close on cleanup/error path
		runtime.UnlockOSThread()
		return nil, 0, fmt.Errorf("create per-test network namespace %q (needs CAP_SYS_ADMIN; run the netlink suites under sudo with ZE_TEST_NETNS=1): %w", name, err)
	}

	// 127.0.0.1 is dead until loopback is up: ze binds its API/BGP listeners and
	// ze-peer connects to them over 127.0.0.1:rec.Port. The netlink socket binds
	// to this locked thread's (new) namespace, so lo is brought up inside it.
	if err := bringLoopbackUp(); err != nil {
		_ = netns.Set(orig)
		orig.Close()            //nolint:errcheck // best-effort close on cleanup/error path
		newNS.Close()           //nolint:errcheck // best-effort close on cleanup/error path
		netns.DeleteNamed(name) //nolint:errcheck // best-effort cleanup on error path
		runtime.UnlockOSThread()
		return nil, 0, fmt.Errorf("bring loopback up in per-test network namespace %q: %w", name, err)
	}

	restore = func() {
		if setErr := netns.Set(orig); setErr != nil {
			logger().Warn("restore original network namespace", "netns", name, "error", setErr)
		}
		orig.Close()  //nolint:errcheck // best-effort close on cleanup/error path
		newNS.Close() //nolint:errcheck // best-effort close on cleanup/error path
		if delErr := netns.DeleteNamed(name); delErr != nil {
			logger().Warn("delete per-test network namespace", "netns", name, "error", delErr)
		}
		runtime.UnlockOSThread()
	}
	return restore, hostInode, nil
}

// provisionNetnsLinks creates each requested interface as a dummy link inside
// the current thread's network namespace, assigns its address (when set), and
// brings it up. It runs after enterTestNetns on the same locked OS thread, so
// the netlink socket binds to the per-test namespace and nothing reaches the
// host. A link that a policy-routing next-hop routes through must exist and
// carry a connected subnet before ze applies config, or RouteAdd fails with
// "network is unreachable" (that is exactly what left test/policy 005 red).
//
// Failure is fatal to the test: a missing interface means the daemon cannot
// reach the state the test asserts, so surfacing the error is honest where a
// silent skip would hide a real regression.
func provisionNetnsLinks(links []NetnsLinkSpec) error {
	for _, l := range links {
		dummy := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: l.Name}}
		if err := netlink.LinkAdd(dummy); err != nil {
			return fmt.Errorf("create netns link %q: %w", l.Name, err)
		}
		link, err := netlink.LinkByName(l.Name)
		if err != nil {
			return fmt.Errorf("find netns link %q after create: %w", l.Name, err)
		}
		if l.Address.IsValid() {
			addr, err := netlink.ParseAddr(l.Address.String())
			if err != nil {
				return fmt.Errorf("parse address %q for netns link %q: %w", l.Address, l.Name, err)
			}
			if err := netlink.AddrAdd(link, addr); err != nil {
				return fmt.Errorf("add address %q to netns link %q: %w", l.Address, l.Name, err)
			}
		}
		if err := netlink.LinkSetUp(link); err != nil {
			return fmt.Errorf("bring netns link %q up: %w", l.Name, err)
		}
	}
	return nil
}

// bringLoopbackUp sets the "lo" link up in the current thread's namespace.
func bringLoopbackUp() error {
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		return fmt.Errorf("find lo: %w", err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		return fmt.Errorf("set lo up: %w", err)
	}
	return nil
}

// testNetnsName derives a filesystem-safe, per-test-unique namespace name. The
// port is unique per concurrently-running test (see ports.go), so distinct tests
// in the 20-way pool never collide on a named-netns bind-mount path.
func testNetnsName(nick string, port int) string {
	var tb textbuf.Buffer
	tb.Str("ze-t-")
	for _, r := range nick {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			tb.Byte(byte(r))
		default:
			tb.Byte('_')
		}
	}
	// Namespace names live at /run/netns/<name>; keep them short and unique.
	name := tb.String()
	if len(name) > 40 {
		name = name[:40]
	}
	var pb textbuf.Buffer
	return pb.Str(name).Byte('-').Int(int64(port)).String()
}

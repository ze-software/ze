//go:build linux

// Design: docs/architecture/testing/ci-format.md -- per-test netns launch mode (Fix B)
// Overview: netns_linux.go -- enterTestNetns helper this test validates the assumption behind

package runner

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/vishvananda/netns"
)

// TestNetnsLaunchChildInheritsNamespace validates assumption A-5 of
// spec-netlink-ci-harness: a child process exec'd after the calling goroutine
// locks its OS thread and switches into a fresh network namespace inherits that
// namespace (fork+exec runs on the locked thread, and clone() copies the
// thread's netns into the child). This is the linchpin of the netns launch mode
// (Fix B): the runner enters a per-test netns on the goroutine thread and relies
// on ze / ze-peer / driver.py, all fork+exec'd from it, landing in the SAME
// throwaway netns without any explicit setns wrapper.
//
// If this assumption is FALSE the whole design changes (an explicit nsenter/setns
// shim would be needed), so this test is the gate that must pass before the rest
// of Fix B is trusted.
//
// It reads namespace identity with `readlink /proc/self/ns/net` (the magic
// symlink resolves to "net:[<inode>]"); `cat` cannot read the nsfs symlink
// target. Requires CAP_SYS_ADMIN to create the namespace -- skipped otherwise,
// so it self-skips on an unprivileged host and is exercised for real in QEMU
// (make ze-qemu-needs-linux-test) where the runner is root.
func TestNetnsLaunchChildInheritsNamespace(t *testing.T) {
	// The host reference must be captured before any thread switches namespace.
	// /proc/self/ns/net reflects the caller's netns; at this point nothing has
	// changed, so it is the host namespace.
	hostLink, err := os.Readlink("/proc/self/ns/net")
	if err != nil {
		t.Fatalf("readlink host /proc/self/ns/net: %v", err)
	}

	// Lock the goroutine to its OS thread for the whole test: the netns switch is
	// per-thread and the child fork+exec must happen on this same thread. All
	// teardown (restore, close handles, unlock) is registered in ONE t.Cleanup so
	// it runs in order while the thread is still locked and the fds still open --
	// a separate `defer origNS.Close()`/`defer UnlockOSThread()` would run before
	// the Cleanup and leave netns.Set operating on a closed fd / unlocked thread.
	runtime.LockOSThread()

	origNS, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Skipf("requires CAP_SYS_ADMIN: cannot get current namespace: %v", err)
	}

	const nsName = "ze-a5-inherit"
	newNS, err := netns.NewNamed(nsName)
	if err != nil {
		// EPERM/EACCES on an unprivileged host: the assumption can only be
		// validated where namespace creation is permitted (QEMU root).
		origNS.Close()
		runtime.UnlockOSThread()
		t.Skipf("requires CAP_SYS_ADMIN: cannot create namespace: %v", err)
	}
	t.Cleanup(func() {
		if restoreErr := netns.Set(origNS); restoreErr != nil {
			t.Errorf("restore original namespace: %v", restoreErr)
		}
		origNS.Close()
		newNS.Close()
		netns.DeleteNamed(nsName) //nolint:errcheck // best-effort cleanup
		runtime.UnlockOSThread()
	})

	// NewNamed already switched THIS thread into the new namespace. Sanity-check
	// the switch took effect on the locked thread before asserting on the child.
	// thread-self (not self) because the netns is a per-thread property.
	threadLink, err := os.Readlink("/proc/thread-self/ns/net")
	if err != nil {
		t.Fatalf("readlink /proc/thread-self/ns/net: %v", err)
	}
	if threadLink == hostLink {
		t.Fatalf("thread did not enter new namespace: still %s (host)", threadLink)
	}

	// The child is fork+exec'd from this locked thread and must inherit its netns.
	out, err := exec.Command("readlink", "/proc/self/ns/net").Output() //nolint:gosec // fixed args, test-only
	if err != nil {
		t.Fatalf("child readlink /proc/self/ns/net: %v", err)
	}
	childLink := strings.TrimSpace(string(out))

	// A-5: the child inherited the throwaway namespace, NOT the host's.
	if childLink == hostLink {
		t.Fatalf("A-5 FALSE: child inherited host namespace %s -- fork does not inherit the locked thread's netns; the netns launch design needs an explicit setns wrapper", hostLink)
	}
	// And it is the SAME namespace the runner thread entered (so ze and ze-peer
	// exec'd from here reach each other over 127.0.0.1).
	if childLink != threadLink {
		t.Fatalf("child namespace %s != runner-thread namespace %s: children do not share the per-test netns", childLink, threadLink)
	}
	t.Logf("A-5 holds: host=%s child=%s (isolated)", hostLink, childLink)
}

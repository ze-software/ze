//go:build linux

// Design: docs/architecture/testing/ci-format.md -- per-test netns launch mode (Fix B)
// Overview: netns_linux.go -- enterTestNetns helper this test validates the assumption behind

package runner

import (
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/ze-software/ze/internal/test/userns"
)

// TestNetnsLaunchChildInheritsNamespace validates assumption A-5 of
// spec-netlink-ci-harness: a child process exec'd after the calling goroutine
// locks its OS thread and switches into a fresh network namespace inherits that
// namespace (fork+exec runs on the locked thread, and clone() copies the
// thread's netns into the child). This is the linchpin of the netns launch mode
// (Fix B): the runner enters a per-test netns on the goroutine thread and relies
// on ze, ze-peer, and compiled fixture helpers all fork+exec'd from it landing in
// the SAME throwaway netns without an explicit nsenter/setns wrapper.
//
// If this assumption is FALSE the whole design changes (an explicit nsenter/setns
// shim would be needed), so this test is the gate that must pass before the rest
// of Fix B is trusted.
//
// It reads namespace identity with `readlink /proc/self/ns/net` (the magic
// symlink resolves to "net:[<inode>]"); `cat` cannot read the nsfs symlink
// target. Creating the named namespace needs CAP_SYS_ADMIN, so the test runs
// in a user namespace of its own (userns.Enter), which holds it, and a refused
// namespace fails the test rather than skipping it.
func TestNetnsLaunchChildInheritsNamespace(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
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
		t.Fatalf("get current network namespace: %v", err)
	}

	const nsName = "ze-a5-inherit"
	newNS, err := netns.NewNamed(nsName)
	if err != nil {
		// The user namespace grants CAP_SYS_ADMIN, so a refusal here is a
		// defect or an unmet need, never a reason to skip.
		origNS.Close() //nolint:errcheck // best-effort close on the failure path
		runtime.UnlockOSThread()
		t.Fatalf("create network namespace %q: %v", nsName, err)
	}
	t.Cleanup(func() {
		if restoreErr := netns.Set(origNS); restoreErr != nil {
			t.Errorf("restore original namespace: %v", restoreErr)
		}
		origNS.Close()            //nolint:errcheck // best-effort close on cleanup path
		newNS.Close()             //nolint:errcheck // best-effort close on cleanup path
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
	out, err := exec.Command("readlink", "/proc/self/ns/net").Output() //nolint:gosec,noctx // fixed args, test-only; must run on THIS locked netns thread (no ctx plumbing)
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

// TestProvisionNetnsLinksMakesNextHopRoutable is the direct regression test for
// test/policy policy-next-hop: enterTestNetns brings up only loopback, so a policy
// next-hop's auto-route (`default via 10.0.0.1`) fails "network is unreachable"
// with no connected interface. provisionNetnsLinks (fed by option=netns-link)
// creates the interface with a same-subnet address so the gateway resolves.
//
// The test enters a fresh netns, provisions eth1/10.0.0.2/24, then performs the
// exact operation that was failing -- adding a default route via 10.0.0.1 -- and
// asserts it succeeds. It needs CAP_SYS_ADMIN and CAP_NET_ADMIN, which its own
// user namespace (userns.Enter) holds, so a refused namespace or link fails it.
func TestProvisionNetnsLinksMakesNextHopRoutable(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	restore, _, err := enterTestNetns("ze-provision-link")
	if err != nil {
		t.Fatalf("enter per-test network namespace: %v", err)
	}
	defer restore()

	links := []NetnsLinkSpec{{
		Name:    "eth1",
		Address: netip.MustParsePrefix("10.0.0.2/24"),
	}}
	if err := provisionNetnsLinks(links); err != nil {
		t.Fatalf("provisionNetnsLinks: %v", err)
	}

	// The interface exists and is up with the requested address.
	link, err := netlink.LinkByName("eth1")
	if err != nil {
		t.Fatalf("provisioned link eth1 not found: %v", err)
	}
	if link.Attrs().Flags&net.FlagUp == 0 {
		t.Error("provisioned link eth1 is not up")
	}
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		t.Fatalf("list addresses on eth1: %v", err)
	}
	var haveAddr bool
	for _, a := range addrs {
		if a.IPNet != nil && a.IPNet.String() == "10.0.0.2/24" {
			haveAddr = true
		}
	}
	if !haveAddr {
		t.Errorf("eth1 missing 10.0.0.2/24; addresses = %v", addrs)
	}

	// The exact failing operation from test/policy 005: a default route via a
	// gateway on the connected subnet must now succeed instead of returning
	// "network is unreachable".
	gw := net.IPv4(10, 0, 0, 1)
	if err := netlink.RouteAdd(&netlink.Route{Gw: gw, Table: 2000}); err != nil {
		t.Fatalf("RouteAdd(default via 10.0.0.1 table 2000) after provisioning: %v -- the next-hop is still unreachable", err)
	}
}

// TestNetnsDialContextReachesTheTestNamespace proves an http= check reaches a
// daemon listening on the per-test namespace's loopback. Method: a listener is
// opened on 127.0.0.1 inside a fresh named namespace and the thread returns to
// the original namespace, as the runner's locked thread does not hold the
// transport's dialling goroutine. A client built the way the runner builds it
// in netns mode must answer 200; a plain client on the same address must be
// refused, which is the defect the dialer exists to fix (pppoe-service-name,
// pppoe-per-mac-cap and pppoe-realm-relay-nomatch were refused this way).
func TestNetnsDialContextReachesTheTestNamespace(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	const nsName = "ze-http-dial"

	runtime.LockOSThread()
	origNS, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatalf("get current network namespace: %v", err)
	}
	newNS, err := netns.NewNamed(nsName)
	if err != nil {
		origNS.Close() //nolint:errcheck // best-effort close on the failure path
		runtime.UnlockOSThread()
		t.Fatalf("create network namespace %q: %v", nsName, err)
	}
	t.Cleanup(func() {
		newNS.Close()             //nolint:errcheck // best-effort close on cleanup path
		netns.DeleteNamed(nsName) //nolint:errcheck // best-effort cleanup
	})
	if err := bringLoopbackUp(); err != nil {
		t.Fatalf("bring loopback up in %q: %v", nsName, err)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen inside %q: %v", nsName, err)
	}
	if err := netns.Set(origNS); err != nil {
		t.Fatalf("return to the original namespace: %v", err)
	}
	origNS.Close() //nolint:errcheck // read-only namespace handle
	runtime.UnlockOSThread()

	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }() //nolint:gocritic // one server for the test's lifetime
	t.Cleanup(func() {
		_ = server.Close() //nolint:errcheck // test teardown
		<-served
	})
	url := "http://" + listener.Addr().String() + "/"

	inside := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{DialContext: netnsDialContext(nsName)}}
	response, err := inside.Get(url) //nolint:noctx // the client timeout bounds the request
	if err != nil {
		t.Fatalf("GET %s through netnsDialContext: %v", url, err)
	}
	response.Body.Close() //nolint:errcheck // empty body
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s through netnsDialContext: status %d, want 200", url, response.StatusCode)
	}

	outside := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{}}
	if response, err := outside.Get(url); err == nil { //nolint:noctx // the client timeout bounds the request
		response.Body.Close() //nolint:errcheck // empty body
		t.Fatalf("GET %s from the original namespace answered %d: the listener is not inside %q, so this test proves nothing", url, response.StatusCode, nsName)
	}
}

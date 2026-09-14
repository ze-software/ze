//go:build integration && linux

// Design: docs/architecture/traffic/cp-survival-2-copp-port179.md -- the control-plane policing this file proves
// Overview: translate.go -- translatePolicy, the producer whose table is under test
//
// A CoPP term that matches no packet renders exactly like one that works, so a
// test reading the nftables ruleset back proves the rule exists and says
// nothing about what the kernel does with it. `over-limit-policy drop` and
// `over-limit-policy accept` differ by one flag in that dump and by everything
// in the behaviour an operator cares about.
//
// So these proofs offer traffic. The observable is the count of connections a
// socket behind the CoPP chain ACCEPTS, and the discrimination is a pair: the
// same flood, the same rate and the same burst, answered one way under drop and
// the other way under accept. A control port the policy never names carries the
// same flood in both, which is what separates "the limiter cut it" from "this
// machine could not open that many connections".
//
// Each test owns a network namespace, so no nftables table reaches the machine
// the test runs on.

package copp

import (
	"fmt"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/firewall"

	// The nft backend registers itself, which is what lets firewall.ApplyAll
	// reach the kernel from this test binary. These proofs assert what the
	// KERNEL does with the table ze publishes, so the real backend is the point
	// rather than an implementation detail.
	_ "github.com/ze-software/ze/internal/plugins/firewall/nft"
)

// The two ports the proofs use. Both are unprivileged, because the test binds
// them itself and the port number decides nothing the policy does: CoPP takes
// the protected port from config, and 179 is only its default.
const (
	protectedPort = 17900
	controlPort   = 17901
)

// offered is the number of connections each flood attempts. It is far above the
// burst of 20 plus one second of a 100/second refill, so a working limiter
// cannot let all of them through however slowly the machine runs.
const offered = 120

// offerWait bounds one connection attempt. A SYN the limiter dropped produces
// no answer at all, so the bound is what turns a drop into a countable failure
// rather than a wait for the kernel's SYN retransmission.
const offerWait = 500 * time.Millisecond

// settleWait lets the accept queue drain behind the last handshake.
const settleWait = 200 * time.Millisecond

// TestCoPPOverLimitDropStopsNewConnectionsToTheProtectedPort proves the drop
// policy reaches the kernel's decision about a packet, and not only the text of
// a rule. The flood is larger than the token bucket can ever cover, so a socket
// behind a working limiter accepts fewer connections than were offered.
//
// The control port carries the identical flood through the identical chain and
// is named by no term, so its full count is what proves the shortfall on the
// protected port belongs to the limiter.
func TestCoPPOverLimitDropStopsNewConnectionsToTheProtectedPort(t *testing.T) {
	enterNamespace(t)
	applyPolicy(t, overPolicyDrop)

	protected := floodPort(t, protectedPort)
	control := floodPort(t, controlPort)

	if control != offered {
		t.Fatalf("the control port accepted %d of %d connections; the flood itself did not get through, so the protected count means nothing", control, offered)
	}
	if protected >= offered {
		t.Fatalf("the protected port accepted %d of %d connections, want fewer: the over-limit drop reached no packet", protected, offered)
	}
	t.Logf("protected port accepted %d of %d, control port %d of %d", protected, offered, control, offered)
}

// TestCoPPOverLimitAcceptKeepsTheProtectedPortReachable is the other half of the
// pair. The rate, the burst and the protected port are the same, and only
// over-limit-policy differs, so the drop above is bound to that setting rather
// than to the flood being too large for the machine.
//
// It is also the lock-out guard, measured rather than read: the default policy
// counts the over-limit connections and MUST let every one of them through.
func TestCoPPOverLimitAcceptKeepsTheProtectedPortReachable(t *testing.T) {
	enterNamespace(t)
	applyPolicy(t, overPolicyAccept)

	protected := floodPort(t, protectedPort)

	if protected != offered {
		t.Fatalf("the protected port accepted %d of %d connections under over-limit-policy accept, want all of them: the default policy is dropping traffic", protected, offered)
	}
}

// applyPolicy parses an operator's configuration, translates it, and publishes
// the table to the kernel through the firewall component. The config text is on
// the path deliberately: a proof that started from a hand-built coppPolicy would
// leave the parse untested and could assert a policy no operator can write.
func applyPolicy(t *testing.T, overLimit string) {
	t.Helper()

	config := fmt.Sprintf(`{"control-plane-protection":{"bgp":{"rate":"100/second","burst":"20","protected-port":["%d"],"over-limit-policy":"%s"}}}`, protectedPort, overLimit)
	policy, present, err := parseCoppConfig(config)
	if err != nil {
		t.Fatalf("parse the copp config: %v", err)
	}
	if !present {
		t.Fatal("the copp config was parsed as absent")
	}

	if err := firewall.RegisterTables("copp", []firewall.Table{translatePolicy(policy)}); err != nil {
		t.Fatalf("register the copp table: %v", err)
	}
	if err := firewall.ApplyAll(); err != nil {
		t.Skipf("needs a kernel that accepts the copp table (nftables and conntrack): %v", err)
	}
	t.Cleanup(func() {
		if err := firewall.RegisterTables("copp", nil); err != nil {
			t.Errorf("withdraw the copp table: %v", err)
		}
		if err := firewall.ApplyAll(); err != nil {
			t.Errorf("apply the withdrawal: %v", err)
		}
	})
}

// floodPort opens a listener, offers every connection at once, and answers with
// the number the listener ACCEPTED. Accepting is the question the operator asks,
// and it is answered on the far side of the chain: a SYN the limiter dropped
// never reaches the accept queue.
//
// The connections are offered concurrently on purpose. Offered one at a time,
// the failures would spread the flood over several seconds, the token bucket
// would refill while it ran, and a working limiter would let all of them
// through.
func floodPort(t *testing.T, port int) int {
	t.Helper()

	address := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp4", address)
	if err != nil {
		t.Fatalf("listen on %s: %v", address, err)
	}

	var accepted atomic.Int64
	var accepting sync.WaitGroup
	accepting.Add(1)
	go acceptUntilClosed(listener, &accepted, &accepting)

	var offering sync.WaitGroup
	for range offered {
		offering.Add(1)
		go dialOnce(address, &offering)
	}
	offering.Wait()

	// sleep(settle): no condition exists to wait on. The count under test is the
	// number of accepts that WILL arrive, so waiting for a target value would
	// assert the answer rather than read it.
	time.Sleep(settleWait)
	listener.Close() //nolint:errcheck // closing is what ends the accept loop
	accepting.Wait()
	return int(accepted.Load())
}

// acceptUntilClosed counts every connection the kernel hands up, and ends when
// the listener closes. It owns no assertion: the count the caller reads is where
// the observation is judged.
func acceptUntilClosed(listener net.Listener, accepted *atomic.Int64, done *sync.WaitGroup) {
	defer done.Done()
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		accepted.Add(1)
		conn.Close() //nolint:errcheck // the handshake is the whole observation
	}
}

// dialOnce offers one connection and drops it. A dial that fails is the
// limiter's answer, so the error is an expected outcome rather than a fault.
func dialOnce(address string, done *sync.WaitGroup) {
	defer done.Done()
	conn, err := net.DialTimeout("tcp4", address, offerWait)
	if err != nil {
		return
	}
	conn.Close() //nolint:errcheck // the handshake is the whole observation
}

// enterNamespace moves the test onto a network namespace of its own, with
// loopback up, and back when it ends. A namespace belongs to a THREAD, so the
// thread is locked for the whole test: a goroutine that migrated would program
// the host's firewall.
//
// It SKIPS rather than fails when the namespace is refused. This file runs
// unprivileged in some environments and privileged in the QEMU and container
// runs, and a missing capability is not a broken product.
func enterNamespace(t *testing.T) {
	t.Helper()

	runtime.LockOSThread()
	origin, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Skipf("needs a network namespace of its own: %v", err)
	}
	if err := unix.Unshare(unix.CLONE_NEWNET); err != nil {
		origin.Close() //nolint:errcheck // best-effort cleanup
		runtime.UnlockOSThread()
		t.Skipf("needs CAP_NET_ADMIN to unshare a network namespace: %v", err)
	}
	t.Cleanup(func() {
		if err := netns.Set(origin); err != nil {
			t.Errorf("cannot return to the original network namespace: %v", err)
		}
		origin.Close() //nolint:errcheck // best-effort cleanup
		runtime.UnlockOSThread()
	})

	loopback, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatalf("loopback in the new namespace: %v", err)
	}
	if err := netlink.LinkSetUp(loopback); err != nil {
		t.Fatalf("bring loopback up: %v", err)
	}
}

//go:build integration && linux

// Design: docs/architecture/traffic/cp-survival-2-copp-port179.md -- the control-plane policing this file proves
// Overview: translate.go -- translatePolicy, the producer whose table is under test
//
// A CoPP term that matches no packet renders exactly like one that works, so a
// test reading the nftables ruleset back proves the rule exists and says
// nothing about what the kernel does with it. A rule's counter cannot say it
// either: ze places the counter BEFORE the term's matches, so it counts every
// packet that reached the rule rather than every packet the rule matched
// (plan/journal/counter-counts-the-wrong-packets.md).
//
// So these proofs offer traffic and read the answer on the socket side, where
// only a packet the chain let through can arrive. The observable is the number
// of connections a listener behind the CoPP chain ACCEPTS out of a flood of
// SYNs, and for a session that was up before the flood, the bytes it still
// delivers once the token bucket is empty. Each term class has a flow it
// selects and a flow it must leave alone:
//
//   - rate-limit under drop: a flood to the protected port is cut to the burst
//     plus the refill over the flood's own duration; the same flood to a port
//     the policy never names is accepted whole.
//   - rate-limit under accept: the same flood to the protected port is
//     accepted whole, because the over-limit packets fall through to the
//     accept policy.
//   - trusted: a flood from the trusted source is accepted whole while the
//     bucket is empty; the flood from the untrusted source is cut.
//   - established: bytes written on a session that was up before the flood
//     arrive while the bucket is empty, so the drop never closes a session.
//
// Every socket in this file is created on the test's own locked thread, and
// no goroutine is started: a network namespace belongs to a THREAD, so a dial
// from a goroutine on another thread would leave the namespace and connect to
// the host. The floods are offered with non-blocking connects for the same
// reason, and it is also what makes the flood fast: 120 SYNs leave in a few
// milliseconds, while the bucket refills one token every 10 milliseconds.
//
// Each test owns a network namespace, so no nftables table reaches the machine
// the test runs on.

package copp

import (
	"errors"
	"fmt"
	"math"
	"net/netip"
	"runtime"
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

// The limiter every proof configures. The kernel's packet limiter is a token
// bucket that starts with burst tokens and refills rate tokens per second, so
// a flood of N SYNs offered over a window w is answered by at most
// burst + rate*w accepts, and by at least burst of them.
const (
	rate  = 100
	burst = 20
)

// The two source addresses a flood can come from. Both are loopback, so the
// kernel routes them without a link, and they differ in the one bit the
// trusted term selects on.
var (
	floodSource   = netip.MustParseAddr("127.0.0.1")
	trustedSource = netip.MustParseAddr("127.0.0.2")
)

// offered is the number of SYNs each flood sends. It is what the bucket can
// hold plus a full second of refill, so a flood that leaves in milliseconds
// cannot be accepted whole by a working limiter however slowly the machine
// runs the loop, and the window is measured so the ceiling follows it.
const offered = 120

// drainIdle is how long the listener may stay silent before a flood is judged
// answered. A handshake over loopback completes inside the connect call or
// microseconds after it, and a SYN the limiter dropped is not retransmitted
// for a second, so a listener silent for this long has nothing more to say.
const drainIdle = 100 * time.Millisecond

// deliverWait bounds the read of bytes written over an established session.
// It is under the kernel's minimum retransmission timeout of 200 milliseconds
// on purpose: a segment the limiter dropped is retransmitted only after that
// timeout, and by then the bucket has refilled, so a longer wait would let a
// dropped segment arrive on its second attempt and read as delivered.
const deliverWait = 100 * time.Millisecond

// payload is the number of bytes the established-session proof writes, in one
// segment, the moment the flood has emptied the bucket.
const payload = 100

// TestCoPPOverLimitDropPolicesTheProtectedPortAndNothingElse proves the drop
// policy reaches the kernel's decision about a packet, and not only the text
// of a rule. The flood is larger than the bucket can cover in the time it
// takes to send, so the listener accepts the burst and the refill and no more.
//
// The control port carries the identical flood through the identical chain and
// is named by no term, so its full count is what proves the shortfall on the
// protected port belongs to the limiter.
func TestCoPPOverLimitDropPolicesTheProtectedPortAndNothingElse(t *testing.T) {
	enterNamespace(t)
	applyPolicy(t, overPolicyDrop, "")

	protected := listen(t, protectedPort)
	assertLimited(t, "protected port", offer(t, protected, floodSource, protectedPort), 0)

	control := listen(t, controlPort)
	assertAcceptedWhole(t, "control port", offer(t, control, floodSource, controlPort))
}

// TestCoPPOverLimitAcceptKeepsTheProtectedPortReachable is the other half of
// the pair. The rate, the burst and the protected port are the same, and only
// over-limit-policy differs, so the shortfall above is bound to that setting
// rather than to the flood being too large for the machine.
//
// It is also the lock-out guard, measured rather than read: the default policy
// counts the over-limit connections and MUST let every one of them through.
func TestCoPPOverLimitAcceptKeepsTheProtectedPortReachable(t *testing.T) {
	enterNamespace(t)
	applyPolicy(t, overPolicyAccept, "")

	protected := listen(t, protectedPort)
	assertAcceptedWhole(t, "protected port under over-limit-policy accept", offer(t, protected, floodSource, protectedPort))
}

// TestCoPPTrustedSourceBypassesTheLimiter proves the trusted term selects on
// the source address and sits before the limiter. The untrusted flood goes
// first and empties the bucket, so the trusted flood that follows can only be
// accepted whole by a rule that never reaches the limiter.
func TestCoPPTrustedSourceBypassesTheLimiter(t *testing.T) {
	enterNamespace(t)
	applyPolicy(t, overPolicyDrop, netip.PrefixFrom(trustedSource, 32).String())

	protected := listen(t, protectedPort)
	assertLimited(t, "untrusted source", offer(t, protected, floodSource, protectedPort), 0)
	assertAcceptedWhole(t, "trusted source", offer(t, protected, trustedSource, protectedPort))
}

// TestCoPPOverLimitDropKeepsAnEstablishedSessionAlive proves the claim CoPP
// exists for: a BGP session that is up before the flood stays up through it.
// The session is opened first, the flood empties the bucket, and the bytes
// written on the session afterwards MUST arrive before any retransmission
// could carry them. Two terms each guarantee that on their own, the established
// accept and the limiter's new-state match, so the proof goes red only when
// both are gone.
//
// MUTATION: remove the established term AND the ConnStateNew match from the
// rate-limit term in translatePolicy; the session then delivers 0 of 100 bytes.
func TestCoPPOverLimitDropKeepsAnEstablishedSessionAlive(t *testing.T) {
	enterNamespace(t)
	applyPolicy(t, overPolicyDrop, "")

	protected := listen(t, protectedPort)
	session := establish(t, protected, protectedPort)

	// The write goes out between the SYNs leaving and the accept queue being
	// drained, which is the moment the bucket is provably empty: every token
	// the flood did not spend, a SYN behind it spent. The drain's idle wait
	// would refill ten of them.
	clients, window := sendFlood(t, floodSource, protectedPort)
	got := deliver(t, session)
	cut := drainFlood(t, protected, clients, window)

	if got != payload {
		t.Fatalf("the established session delivered %d of %d bytes while the bucket was empty: the drop reached a session that was already up", got, payload)
	}
	assertLimited(t, "protected port", cut, 1)
}

// applyPolicy parses an operator's configuration, translates it, and publishes
// the table to the kernel through the firewall component. The config text is on
// the path deliberately: a proof that started from a hand-built coppPolicy would
// leave the parse untested and could assert a policy no operator can write.
// trusted is one trusted-source prefix, or empty for none.
func applyPolicy(t *testing.T, overLimit, trusted string) {
	t.Helper()

	trustedLeaf := ""
	if trusted != "" {
		trustedLeaf = fmt.Sprintf(`,"trusted-source":[%q]`, trusted)
	}
	config := fmt.Sprintf(`{"control-plane-protection":{"bgp":{"rate":"%d/second","burst":"%d","protected-port":["%d"],"over-limit-policy":%q%s}}}`,
		rate, burst, protectedPort, overLimit, trustedLeaf)
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

// flood is what one flood observed: how many of the offered connections the
// listener accepted, and how long the SYNs took to leave. The window is what
// turns the limiter's rate into a ceiling on the count.
type flood struct {
	accepted int
	window   time.Duration
}

// refill is the number of tokens the bucket gained while the flood was
// leaving: one for every 1/rate of the window, rounded up, plus one for the
// SYN still in the softirq backlog when the clock stopped.
func (f flood) refill() int {
	return int(math.Ceil(rate*f.window.Seconds())) + 1
}

// assertLimited fails unless the flood was cut to what the bucket allows.
// spent is the number of SYNs the test itself sent to the protected port
// before this flood, each of which took a token the flood no longer has. Both
// bounds are asserted: the floor says the limiter passed what was left of the
// burst rather than dropping everything, and the ceiling says it dropped the
// rest.
func assertLimited(t *testing.T, what string, f flood, spent int) {
	t.Helper()

	floor := burst - spent
	ceiling := floor + f.refill()
	if f.accepted < floor {
		t.Fatalf("%s accepted %d of %d connections, want at least the %d left of the burst: the limiter dropped packets it had tokens for", what, f.accepted, offered, floor)
	}
	if f.accepted > ceiling {
		t.Fatalf("%s accepted %d of %d connections offered in %v, want at most %d: the over-limit drop reached no packet", what, f.accepted, offered, f.window, ceiling)
	}
	t.Logf("%s accepted %d of %d connections offered in %v (floor %d, ceiling %d)", what, f.accepted, offered, f.window, floor, ceiling)
}

// assertAcceptedWhole fails unless every offered connection was accepted, which
// is the answer for a flow no term cuts.
func assertAcceptedWhole(t *testing.T, what string, f flood) {
	t.Helper()

	if f.accepted != offered {
		t.Fatalf("%s accepted %d of %d connections, want all of them: a term cut a flow it may not touch", what, f.accepted, offered)
	}
}

// listen opens a non-blocking listener on the loopback port and closes it when
// the test ends. SO_REUSEADDR lets a second listener take a port whose earlier
// connections are still in TIME_WAIT.
func listen(t *testing.T, port int) int {
	t.Helper()

	fd := socket(t)
	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err != nil {
		t.Fatalf("SO_REUSEADDR on the listener: %v", err)
	}
	if err := unix.Bind(fd, &unix.SockaddrInet4{Port: port, Addr: floodSource.As4()}); err != nil {
		t.Fatalf("bind port %d: %v", port, err)
	}
	if err := unix.Listen(fd, offered); err != nil {
		t.Fatalf("listen on port %d: %v", port, err)
	}
	return fd
}

// offer sends one flood to the port from source and answers with what the
// listener made of it.
func offer(t *testing.T, listener int, source netip.Addr, port int) flood {
	t.Helper()

	clients, window := sendFlood(t, source, port)
	return drainFlood(t, listener, clients, window)
}

// sendFlood sends one SYN per offered connection to the port, from source, and
// answers with the client sockets and the time the SYNs took to leave. The
// connects are non-blocking and back to back, so every SYN is evaluated within
// the window the clock measures, and the ceiling the window gives is what
// makes the count an exact claim rather than a floor.
func sendFlood(t *testing.T, source netip.Addr, port int) ([]int, time.Duration) {
	t.Helper()

	clients := make([]int, 0, offered)
	start := time.Now()
	for range offered {
		clients = append(clients, connectFrom(t, source, port))
	}
	return clients, time.Since(start)
}

// drainFlood answers with the number of connections the listener ACCEPTED.
// Accepting is the question the operator asks, and it is answered on the far
// side of the chain: a SYN the limiter dropped never reaches the accept queue.
// The client sockets are closed once the queue is empty, so a dropped SYN is
// never retransmitted into the next flood.
func drainFlood(t *testing.T, listener int, clients []int, window time.Duration) flood {
	t.Helper()

	accepted := 0
	for {
		conn, _, err := unix.Accept4(listener, unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC)
		if err == nil {
			accepted++
			closeFD(t, conn)
			continue
		}
		if !errors.Is(err, unix.EAGAIN) {
			t.Fatalf("accept: %v", err)
		}
		if !readable(t, listener, drainIdle) {
			break
		}
	}

	for _, client := range clients {
		closeFD(t, client)
	}
	return flood{accepted: accepted, window: window}
}

// session is one connection that is up before a flood: the client end that
// writes, and the accepted end that reads.
type session struct {
	client int
	server int
}

// establish connects one client to the listener and accepts it, so both ends
// are open before the flood starts.
func establish(t *testing.T, listener, port int) session {
	t.Helper()

	client := connectFrom(t, floodSource, port)
	if !readable(t, listener, drainIdle) {
		t.Fatalf("the session's SYN to port %d was never answered before the flood", port)
	}
	server, _, err := unix.Accept4(listener, unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC)
	if err != nil {
		t.Fatalf("accept the session on port %d: %v", port, err)
	}
	t.Cleanup(func() {
		closeFD(t, client)
		closeFD(t, server)
	})
	return session{client: client, server: server}
}

// deliver writes payload bytes on the client end, in one segment, and answers
// with how many of them the server end read within deliverWait.
func deliver(t *testing.T, s session) int {
	t.Helper()

	sent := make([]byte, payload)
	if _, err := unix.Write(s.client, sent); err != nil {
		t.Fatalf("write on the established session: %v", err)
	}

	got := 0
	buf := make([]byte, payload)
	deadline := time.Now().Add(deliverWait)
	for got < payload {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		if !readable(t, s.server, remaining) {
			break
		}
		n, err := unix.Read(s.server, buf)
		if err != nil {
			t.Fatalf("read on the established session: %v", err)
		}
		got += n
	}
	return got
}

// connectFrom sends one SYN to the loopback port from source and answers with
// the client socket. The connect is non-blocking, so the call returns once the
// SYN has left rather than once the handshake is answered: EINPROGRESS is the
// normal answer, and nil is the handshake having completed inside the call.
func connectFrom(t *testing.T, source netip.Addr, port int) int {
	t.Helper()

	fd := socket(t)
	if err := unix.Bind(fd, &unix.SockaddrInet4{Addr: source.As4()}); err != nil {
		t.Fatalf("bind the client to %s: %v", source, err)
	}
	err := unix.Connect(fd, &unix.SockaddrInet4{Port: port, Addr: floodSource.As4()})
	if err != nil && !errors.Is(err, unix.EINPROGRESS) {
		t.Fatalf("connect from %s to port %d: %v", source, port, err)
	}
	return fd
}

// socket opens one non-blocking IPv4 TCP socket on the locked thread, which is
// what places it in the test's namespace.
func socket(t *testing.T) int {
	t.Helper()

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("open a TCP socket: %v", err)
	}
	return fd
}

// readable waits up to timeout for the descriptor to have something to read:
// a connection to accept, or bytes on a session.
func readable(t *testing.T, fd int, timeout time.Duration) bool {
	t.Helper()

	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	for {
		n, err := unix.Poll(fds, int(timeout.Milliseconds()))
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			t.Fatalf("poll: %v", err)
		}
		return n > 0
	}
}

// closeFD closes a socket the test opened. A close that fails is reported
// rather than ignored, because a leaked socket keeps a port busy for the next
// listener.
func closeFD(t *testing.T, fd int) {
	t.Helper()

	if err := unix.Close(fd); err != nil {
		t.Errorf("close socket %d: %v", fd, err)
	}
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

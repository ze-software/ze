// Design: docs/architecture/testing/interop.md -- GTSM and CoPP on ze's BGP listener, observed by a foreign peer.
// Related: check_session_protection_predicate.go -- the pure readings this body calls.
// Related: register_session_protection.go -- registry entry that dispatches this checker.
package bgp

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	sessionProtectionScenario = "bgp-session-protection-frr"
	// coppInputChain is the base chain of ze_copp that holds the access list.
	coppInputChain = "input"
	// sessionProtectionStrayHost is the host number of the address the checker
	// adds to FRR's interface: no ze peer, no dynamic range, no trusted source.
	sessionProtectionStrayHost = 40
	// sessionProtectionWaitSeconds bounds each probe's connect. A SYN-ACK on
	// one lab bridge arrives in well under a millisecond, so a probe still
	// unanswered after it was dropped, not slow.
	sessionProtectionWaitSeconds = "5"
	// sessionProtectionTTLBelow is one below the floor `ttl { max 1; }` gives
	// the peer (RFC 5082 Section 3: 256 - 1 = 255).
	sessionProtectionTTLBelow = "254"
	sessionProtectionTTLFloor = "255"
	sessionProtectionPort     = "179"
)

// sessionProbeScript opens one TCP connection to ze's listener from a chosen
// source address with a chosen TTL and prints what it saw. It runs on FRR's
// image, which carries python3 (frr-reload.py needs it) and no tool that sets
// a socket's TTL: busybox nc has no TTL option, and FRR's own session socket
// already holds TTL 255, so a route metric could not lower only the probe.
// The arguments are source, target, port, TTL and the connect deadline in
// seconds.
const sessionProbeScript = `import socket, sys
source, target, port, ttl, wait = sys.argv[1], sys.argv[2], int(sys.argv[3]), int(sys.argv[4]), float(sys.argv[5])
probe = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
try:
    probe.setsockopt(socket.IPPROTO_IP, socket.IP_TTL, ttl)
    probe.bind((source, 0))
    probe.settimeout(wait)
    probe.connect((target, port))
    print("connected")
except TimeoutError:
    print("timeout")
except ConnectionRefusedError:
    print("refused")
except OSError as error:
    print("error", error)
finally:
    probe.close()
`

// checkSessionProtection proves, from FRR's side of the wire, that ze's
// below-floor GTSM term and its CoPP access list stop a SYN before ze's
// listener answers it, while the configured GTSM peer keeps its session.
//
// RFC 5082 Section 2.2: "For maximal protection, ingress filtering should be
// applied before the packet goes through the scarce resource." RFC 7454
// Section 4 asks for an ACL that would "discard all packets directed to TCP
// port 179 on the local device and sourced from an address not known or
// permitted to become a BGP neighbor."
//
// Every probe is a python3 connect run in FRR's container, so its outcome is
// what a sender observes: a completed handshake means ze's kernel sent a
// SYN-ACK, a timeout means the SYN was dropped. The control probe at TTL 255
// from the same address proves the probe can see a SYN-ACK in this lab, so the
// two timeouts are drops, not a probe that never works.
func checkSessionProtection(ctx context.Context, check *interoplab.CheckContext) error {
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, sessionProtectionScenario, assertion, cause)
	}
	if !check.Network.IPv4.IsValid() {
		return fail(1, errors.New("session-protection scenario has no selected IPv4 network"))
	}
	zeAddress := networkHostAddress(check.Network, 2)
	frrAddress := networkHostAddress(check.Network, 3)
	strayAddress := networkHostAddress(check.Network, sessionProtectionStrayHost)

	// Assertion 1. FRR's TTL-security session with ze is Established.
	session := operation{kind: opFRRSession, argument: zeAddress}
	if err := runOperation(ctx, check.Network, check.Lab, &session); err != nil {
		return fail(1, err)
	}

	// Assertion 2. FRR applies ttl-security hops 1, so the session runs at TTL
	// 255 from FRR's side, and the session's identity is kept for assertion 8.
	showNeighbor := []string{cmdVtysh, "-c", "show bgp neighbor " + zeAddress + " json"}
	neighbor, err := check.Lab.Query(ctx, peerFRR, showNeighbor, nil)
	if err != nil {
		return fail(2, err)
	}
	hops, err := frrGTSMHops(neighbor)
	if err != nil {
		return fail(2, err)
	}
	if hops != 1 {
		return fail(2, fmt.Errorf("FRR applies ttl-security hops %d, want 1", hops))
	}
	fence, err := readFRRAIGPRecipientFence(neighbor, zeAddress)
	if err != nil {
		return fail(2, err)
	}

	// Assertion 3. ze_copp is installed. The unknown-source-drop counter read
	// here is judged at assertion 7, after the stray probe: a table that lost
	// its drop term must first show at FRR, as a SYN-ACK to an unknown source.
	readTable := []string{cmdNft, nftActionList, nftObjectTable, nftFamilyInet, "ze_copp"}
	readDrops := func() (uint64, error) {
		table, queryErr := check.Lab.Query(ctx, "ze", readTable, nil)
		if queryErr != nil {
			return 0, queryErr
		}
		return coppUnknownSourceDrops(table)
	}
	if _, err := check.Lab.Query(ctx, "ze", readTable, nil); err != nil {
		return fail(3, err)
	}
	dropsBefore, dropsBeforeErr := readDrops()

	probe := func(source, ttl string) (string, error) {
		argv := []string{"python3", "-c", sessionProbeScript, source, zeAddress, sessionProtectionPort, ttl, sessionProtectionWaitSeconds}
		return check.Lab.Query(ctx, peerFRR, argv, nil)
	}

	// Assertion 4. AC-1: a SYN from the GTSM peer's own address at TTL 254
	// gets no answer. CoPP admits this source (it is a neighbor), so only the
	// ze_gtsm listen-port term can drop it.
	output, err := probe(frrAddress, sessionProtectionTTLBelow)
	if err != nil {
		return fail(4, err)
	}
	if err := requireSessionProbe(output, "a SYN from "+frrAddress+" at TTL 254", sessionProbeTimedOut); err != nil {
		return fail(4, err)
	}

	// Assertion 5. The control: the same source at TTL 255 completes the
	// handshake, so the listener is up, the neighbor admitted, and the probe
	// able to observe a SYN-ACK.
	output, err = probe(frrAddress, sessionProtectionTTLFloor)
	if err != nil {
		return fail(5, err)
	}
	if err := requireSessionProbe(output, "a SYN from "+frrAddress+" at TTL 255", sessionProbeConnected); err != nil {
		return fail(5, err)
	}

	// Assertion 6. AC-7: a SYN at TTL 255 from an address in FRR's container
	// that ze does not know gets no answer, not even a reset. GTSM leaves this
	// source alone (RFC 5082 Section 3 Unknown), so only CoPP can drop it.
	stray, err := netip.ParseAddr(strayAddress)
	if err != nil {
		return fail(6, err)
	}
	prefix := netip.PrefixFrom(stray, check.Network.IPv4.Bits()).String()
	addressCommand := func(action string) []string {
		return []string{"ip", ipObjectAddr, action, prefix, ipOptionDevice, containerInterface}
	}
	if _, err := check.Lab.Exec(ctx, peerFRR, addressCommand("add"), nil); err != nil {
		return fail(6, err)
	}
	defer func() {
		_, _ = check.Lab.Exec(context.WithoutCancel(ctx), peerFRR, addressCommand("del"), nil) //nolint:errcheck // the lab container is removed at teardown; a failed delete leaves nothing behind
	}()
	output, err = probe(strayAddress, sessionProtectionTTLFloor)
	if err != nil {
		return fail(6, err)
	}
	if err := requireSessionProbe(output, "a SYN from the unconfigured "+strayAddress, sessionProbeTimedOut); err != nil {
		return fail(6, err)
	}

	// Assertion 7. ze counted that drop in the unknown-source-drop term.
	if dropsBeforeErr != nil {
		return fail(7, dropsBeforeErr)
	}
	dropsAfter, err := readDrops()
	if err != nil {
		return fail(7, err)
	}
	if err := requireCoppDropsRose(dropsBefore, dropsAfter); err != nil {
		return fail(7, err)
	}

	// Assertion 8. AC-14: the session FRR held before the probes is the one it
	// holds after them: still Established, never dropped, same sockets.
	neighbor, err = check.Lab.Query(ctx, peerFRR, showNeighbor, nil)
	if err != nil {
		return fail(8, err)
	}
	if err := requireFRRAIGPRecipientFence(neighbor, zeAddress, fence); err != nil {
		return fail(8, err)
	}
	return nil
}

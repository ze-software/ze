//go:build integration && linux

// Design: plan/spec-mpls-2-ldp.md -- AC-10 interop with FRR ldpd.
//
// This is handover item #3 (FRR interop), which could not be run on darwin. It
// stands up a real FRR (zebra + ldpd) peer in a child network namespace, connected
// to the test process by a veth pair, and drives ze's REAL LDP code (discovery +
// session FSM + wire codec) in the init namespace against it. Only FRR is
// namespaced (via `ip netns exec`); ze runs in-process in the init namespace so no
// goroutine has to be pinned to a namespace.
//
// It proves, on the wire against a different implementation:
//   - ze's multicast Hello is accepted by FRR and FRR's Hello is decoded by ze
//     (adjacency formed) -- AC-1/AC-2
//   - the TCP session reaches operational (Init/Keepalive interop) -- AC-2
//   - ze decodes a Label Mapping that FRR advertises for one of its FECs -- AC-4
//
// FRR's own MPLS dataplane programming is not required for label exchange, so the
// test does not depend on it.
package ldp

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
)

const (
	frrNS      = "ze-ldp-frr"
	zeVeth     = "zeldp0"
	frrVeth    = "frr0"
	frrAddr    = "10.0.0.1"
	zeAddr     = "10.0.0.2"
	frrRunDir  = "/run/frr"
	frrConfDir = "/etc/frr"
)

// frrBin locates an FRR daemon/binary across the layouts Alpine and Debian use.
func frrBin(name string) string {
	for _, p := range []string{"/usr/lib/frr/" + name, "/usr/sbin/" + name, "/usr/bin/" + name} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// sh runs a shell command, failing the test with its combined output on error.
func sh(t *testing.T, format string, args ...any) {
	t.Helper()
	cmd := fmt.Sprintf(format, args...)
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput() //nolint:gosec // test-controlled command
	require.NoErrorf(t, err, "cmd %q failed: %s", cmd, out)
}

// shSoft runs a shell command best-effort, returning combined output.
func shSoft(format string, args ...any) string {
	cmd := fmt.Sprintf(format, args...)
	out, _ := exec.Command("sh", "-c", cmd).CombinedOutput() //nolint:gosec // test-controlled command
	return string(out)
}

// startFRRPeer builds the veth topology, writes an FRR config, and starts zebra +
// ldpd in a child namespace configured as an LDP peer on its veth end. Cleanup is
// registered via t.Cleanup. Skips the test if FRR is unavailable.
func startFRRPeer(t *testing.T) {
	t.Helper()
	zebra, ldpd := frrBin("zebra"), frrBin("ldpd")
	if zebra == "" || ldpd == "" {
		t.Skipf("FRR not installed (zebra=%q ldpd=%q) -- run via ze-qemu-integration-test with --packages frr", zebra, ldpd)
	}

	// Clean any residue from a previous aborted run, then register teardown.
	teardown := func() {
		shSoft("pkill -f '%s' 2>/dev/null; pkill -f '%s' 2>/dev/null", zebra, ldpd)
		shSoft("ip netns del %s 2>/dev/null", frrNS)
		shSoft("ip link del %s 2>/dev/null", zeVeth)
	}
	teardown()
	t.Cleanup(teardown)

	// Topology: zeVeth stays in the init netns (ze side); frrVeth moves into frrNS.
	sh(t, "ip netns add %s", frrNS)
	sh(t, "ip link add %s type veth peer name %s", zeVeth, frrVeth)
	sh(t, "ip link set %s netns %s", frrVeth, frrNS)
	sh(t, "ip addr add %s/24 dev %s", zeAddr, zeVeth)
	sh(t, "ip link set %s up", zeVeth)
	sh(t, "ip -n %s addr add %s/24 dev %s", frrNS, frrAddr, frrVeth)
	sh(t, "ip -n %s link set %s up", frrNS, frrVeth)
	sh(t, "ip -n %s link set lo up", frrNS)
	// MPLS label space for FRR's side (harmless if dataplane is unused here).
	shSoft("modprobe mpls_router; ip netns exec %s sysctl -w net.mpls.platform_labels=1048575", frrNS)

	require.NoError(t, os.MkdirAll(frrRunDir, 0o755))
	require.NoError(t, os.MkdirAll(frrConfDir, 0o755))
	shSoft("chown -R frr:frr %s %s 2>/dev/null", frrRunDir, frrConfDir)

	conf := fmt.Sprintf(`log stdout
!
mpls ldp
 router-id %s
 !
 address-family ipv4
  discovery hello interval 1
  discovery hello holdtime 15
  discovery transport-address %s
  !
  interface %s
 exit-address-family
!
`, frrAddr, frrAddr, frrVeth)
	require.NoError(t, os.WriteFile(frrConfDir+"/frr.conf", []byte(conf), 0o644)) //nolint:gosec // test config

	// Start zebra then ldpd in the FRR namespace. Output goes to log files so the
	// daemonized process does not hold the test's stdout pipe open.
	sh(t, "ip netns exec %s %s -d -f %s/frr.conf -i %s/zebra.pid -z %s/zserv.api > %s/zebra.log 2>&1",
		frrNS, zebra, frrConfDir, frrRunDir, frrRunDir, frrRunDir)
	time.Sleep(time.Second)
	sh(t, "ip netns exec %s %s -d -f %s/frr.conf -i %s/ldpd.pid -z %s/zserv.api > %s/ldpd.log 2>&1",
		frrNS, ldpd, frrConfDir, frrRunDir, frrRunDir, frrRunDir)
}

// VALIDATES: AC-1/AC-2/AC-4 -- ze forms an LDP adjacency and operational session
// with a real FRR ldpd peer, and decodes a Label Mapping FRR advertises. This is
// genuine cross-implementation wire interop.
func TestLDPInteropFRR(t *testing.T) {
	startFRRPeer(t)

	// Route ze's LDP logs to stdout (captured by the QEMU runner) at debug so a
	// failure shows the discovery/session activity.
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	setLogger(log)

	lib := NewLIB()
	adjTable := NewAdjacencyTable()
	var sessionsMu sync.Mutex
	sessions := make(map[string]*Session)
	bus := &captureBus{}
	fib := newLDPFIB(bus, log)

	zeLSR := netip.MustParseAddr(zeAddr)
	lsrID := zeLSR.As4()
	cfg := ldpConfig{
		LSRID:         zeLSR,
		TransportAddr: zeLSR,
		HelloInterval: time.Second,
		HelloHoldTime: 15 * time.Second,
		KeepaliveTime: 15 * time.Second,
		Interfaces:    []string{zeVeth},
	}

	// Originate ze's local FECs (AC-3) so ze also advertises labels to FRR.
	for _, fec := range localFECs(cfg.LSRID, connectedPrefixes(cfg.Interfaces, log)) {
		fib.ProgramPop(fec, lib.EnsureLocal(fec).Label)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr := newDiscoveryManager(ctx, log, func(ifctx context.Context, ifName string, c ldpConfig) {
		discoverOnInterface(ifctx, log, c, lsrID, ifName, adjTable, func(adj *Adjacency) {
			startSessionForAdj(ctx, log, adj, lsrID, c.TransportAddr, lib, sessions, &sessionsMu, fib)
		})
	})
	mgr.reconcile(cfg)

	// Poll for an operational session and a remote label binding from FRR.
	var operational bool
	var peerAddrs []netip.Addr
	deadline := time.Now().Add(75 * time.Second)
	for time.Now().Before(deadline) {
		sessionsMu.Lock()
		for _, s := range sessions {
			if s.State() == StateOperational {
				operational = true
				peerAddrs = s.PeerAddresses()
			}
		}
		sessionsMu.Unlock()
		if operational && lib.Len() > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if !operational || lib.Len() == 0 {
		t.Logf("FRR zebra.log:\n%s", shSoft("tail -40 %s/zebra.log", frrRunDir))
		t.Logf("FRR ldpd.log:\n%s", shSoft("tail -40 %s/ldpd.log", frrRunDir))
		t.Logf("FRR ldp discovery:\n%s", shSoft("ip netns exec %s %s -c 'show mpls ldp discovery' 2>&1", frrNS, frrBin("vtysh")))
		t.Logf("FRR ldp neighbor:\n%s", shSoft("ip netns exec %s %s -c 'show mpls ldp neighbor' 2>&1", frrNS, frrBin("vtysh")))
	}

	require.True(t, operational, "ze did not reach an operational LDP session with FRR")
	require.NotZero(t, lib.Len(), "ze received no label binding from FRR")

	bindings := lib.AllBindings()
	t.Logf("ze learned %d binding(s) from FRR; first: %s -> label %d", len(bindings), bindings[0].FEC, bindings[0].Label)

	// ze must have decoded FRR's Address message (RFC 5036 Section 3.5.5) and
	// recorded FRR's interface address for next-hop resolution.
	t.Logf("ze learned peer addresses: %v", peerAddrs)
	require.Contains(t, peerAddrs, netip.MustParseAddr(frrAddr),
		"ze did not learn FRR's interface address from its Address message")

	// FRR advertises implicit-null (label 3) for its connected FEC (PHP). Assert ze
	// honored that: no OpPush was programmed for an implicit-null binding (ze would
	// otherwise impose label 3 on the wire and break forwarding).
	for _, b := range bindings {
		if b.Label == ImplicitNull {
			for _, e := range bus.emits {
				batch, ok := e.payload.(*mplsfibevents.EntryBatch)
				if !ok {
					continue
				}
				for _, entry := range batch.Entries {
					require.NotEqualf(t, mplsfibevents.OpPush, entry.Op,
						"ze programmed a push for an implicit-null binding (%s)", b.FEC)
				}
			}
		}
	}

	// Best-effort: confirm FRR sees ze as a neighbor (the reverse direction).
	t.Logf("FRR neighbor view:\n%s", shSoft("ip netns exec %s %s -c 'show mpls ldp neighbor' 2>&1", frrNS, frrBin("vtysh")))
}

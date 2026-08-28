//go:build integration && linux

// Design: docs/architecture/isis/isis-13-cli-diag-interop.md -- IS-IS interop with FRR isisd.
//
// This is the on-the-wire interop validation that could not run on darwin. It
// stands up a real FRR (zebra + isisd) IS-IS peer in a child network namespace,
// connected to the test process by a veth pair, and drives ze's REAL IS-IS code
// (AF_PACKET transport + adjacency FSM + LSDB/flooding) in the init namespace
// against it. Only FRR is namespaced (via `ip netns exec`); ze opens its raw
// circuit on the init-namespace veth end, so no goroutine is pinned to a netns.
// It mirrors internal/plugins/ldp/frr_interop_integration_linux_test.go.
//
// It proves, on the wire against a different implementation:
//   - ze's padded LAN IIH is accepted by FRR and FRR's IIH is decoded by ze, so
//     the adjacency reaches Up (correct ISO multicast MAC, LLC/SAP, three-way echo)
//   - ze receives FRR's LSP and stores it in its LSDB (CSNP/PSNP-driven sync)
//
// FRR's SPF/dataplane is not required; cross-implementation adjacency + LSDB
// exchange is the interop proof. It t.Skips when FRR is absent; run it through
// `./le qemu all-tests`, whose guest provides FRR.
package isis

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

const (
	frrISISNS      = "ze-isis-frr"
	zeISISVeth     = "zeisisfrr0"
	frrISISVeth    = "frrisis0"
	frrISISNet     = "49.0001.0000.0000.0001.00" // FRR peer: area 49.0001, system-id 0000.0000.0001
	zeISISNet      = "49.0001.0000.0000.0002.00" // ze: same area, system-id 0000.0000.0002
	frrISISRunDir  = "/run/frr"
	frrISISConfDir = "/etc/frr"
)

// frrISISBin locates an FRR daemon/binary across the layouts Alpine and Debian use.
func frrISISBin(name string) string {
	for _, p := range []string{"/usr/lib/frr/" + name, "/usr/sbin/" + name, "/usr/bin/" + name} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// frrISISsh runs a shell command, failing the test with its combined output on error.
func frrISISsh(t *testing.T, format string, args ...any) {
	t.Helper()
	cmd := fmt.Sprintf(format, args...)
	out, err := exec.CommandContext(t.Context(), "sh", "-c", cmd).CombinedOutput() //nolint:gosec // test-controlled command
	require.NoErrorf(t, err, "cmd %q failed: %s", cmd, out)
}

// frrISISshSoft runs a shell command best-effort, returning combined output.
//
// It takes no context on purpose: the teardown callers run from t.Cleanup, and
// the testing package cancels t.Context() BEFORE cleanup functions run, so a
// CommandContext here would be killed before it could delete the namespace.
func frrISISshSoft(format string, args ...any) string {
	cmd := fmt.Sprintf(format, args...)
	out, _ := exec.Command("sh", "-c", cmd).CombinedOutput() //nolint:gosec,noctx // test-controlled command, runs after t.Context() is canceled
	return string(out)
}

// startISISFRRPeer builds the veth topology, writes an FRR isisd config, and
// starts zebra + isisd in a child namespace running IS-IS L1 on its veth end.
// Cleanup is registered via t.Cleanup. Skips the test when FRR is unavailable.
func startISISFRRPeer(t *testing.T) {
	t.Helper()
	zebra, isisd := frrISISBin("zebra"), frrISISBin("isisd")
	if zebra == "" || isisd == "" {
		t.Skipf("FRR not installed (zebra=%q isisd=%q); run via `./le qemu all-tests`", zebra, isisd)
	}

	// Clean any residue from a previous aborted run, then register teardown.
	teardown := func() {
		frrISISshSoft("pkill -f %q 2>/dev/null; pkill -f %q 2>/dev/null", zebra, isisd)
		frrISISshSoft("pkill -f 'tcpdump -i %s' 2>/dev/null; pkill -f 'tcpdump -i %s' 2>/dev/null", zeISISVeth, frrISISVeth)
		frrISISshSoft("ip netns del %s 2>/dev/null", frrISISNS)
		frrISISshSoft("ip link del %s 2>/dev/null", zeISISVeth)
	}
	teardown()
	t.Cleanup(teardown)

	// Topology: zeISISVeth stays in the init netns (ze side); frrISISVeth moves
	// into frrISISNS. IS-IS adjacency is pure Layer 2 (IIH over the veth); the IPs
	// only give FRR an interface prefix to advertise in its LSP.
	frrISISsh(t, "ip netns add %s", frrISISNS)
	frrISISsh(t, "ip link add %s type veth peer name %s", zeISISVeth, frrISISVeth)
	frrISISsh(t, "ip link set %s netns %s", frrISISVeth, frrISISNS)
	frrISISsh(t, "ip addr add 10.10.0.2/24 dev %s", zeISISVeth)
	frrISISsh(t, "ip link set %s up", zeISISVeth)
	frrISISsh(t, "ip -n %s addr add 10.10.0.1/24 dev %s", frrISISNS, frrISISVeth)
	frrISISsh(t, "ip -n %s link set %s up", frrISISNS, frrISISVeth)
	frrISISsh(t, "ip -n %s link set lo up", frrISISNS)

	require.NoError(t, os.MkdirAll(frrISISRunDir, 0o755))
	require.NoError(t, os.MkdirAll(frrISISConfDir, 0o755))
	frrISISshSoft("chown -R frr:frr %s %s 2>/dev/null", frrISISRunDir, frrISISConfDir)

	conf := fmt.Sprintf(`log stdout
!
interface %s
 ip router isis ze
 isis circuit-type level-1
 isis hello-interval 1
!
router isis ze
 net %s
 is-type level-1
 metric-style wide
!
`, frrISISVeth, frrISISNet)
	require.NoError(t, os.WriteFile(frrISISConfDir+"/frr.conf", []byte(conf), 0o644)) //nolint:gosec // test config

	// Start packet captures on BOTH veth ends so a failure shows the bidirectional
	// IIH exchange (ISO multicast 01:80:c2:00:00:14 for AllL1ISs). The captures run
	// for the whole test; teardown (pkill) reaps them. Best-effort: tcpdump may be
	// absent, which only loses the diagnostic, not the test.
	frrISISshSoft("tcpdump -i %s -w %s/ze.pcap -U -c 200 >/dev/null 2>&1 &", zeISISVeth, frrISISRunDir)
	frrISISshSoft("ip netns exec %s tcpdump -i %s -w %s/frr.pcap -U -c 200 >/dev/null 2>&1 &", frrISISNS, frrISISVeth, frrISISRunDir)

	// Start zebra then isisd in the FRR namespace. Output goes to log files so the
	// daemonized process does not hold the test's stdout pipe open.
	frrISISsh(t, "ip netns exec %s %s -d -f %s/frr.conf -i %s/zebra.pid -z %s/zserv.api > %s/zebra.log 2>&1",
		frrISISNS, zebra, frrISISConfDir, frrISISRunDir, frrISISRunDir, frrISISRunDir)
	time.Sleep(time.Second)
	frrISISsh(t, "ip netns exec %s %s -d -f %s/frr.conf -i %s/isisd.pid -z %s/zserv.api > %s/isisd.log 2>&1",
		frrISISNS, isisd, frrISISConfDir, frrISISRunDir, frrISISRunDir, frrISISRunDir)
}

// lsdbHasSystem reports whether ze's LSDB at the given level holds an LSP
// originated by sys -- i.e. ze received and stored that peer's LSP.
func lsdbHasSystem(e *engine, level lsdb.Level, sys types.SystemID) bool {
	for _, id := range e.lsdb.LSPIDs(level) {
		if id.SystemID() == sys {
			return true
		}
	}
	return false
}

// dumpZeAdjacencies renders ze's per-circuit adjacency tables for the failure
// diagnostics: it shows whether ze even HEARD FRR (a record exists), and if so
// at what state (Initializing means ze heard FRR's IIH but FRR did not echo ze's
// SNPA, i.e. the LAN three-way is incomplete from ze's side).
func dumpZeAdjacencies(e *engine) string {
	var b strings.Builder
	e.circuitsMu.RLock()
	defer e.circuitsMu.RUnlock()
	for name, c := range e.circuitByName {
		rows := c.Table().Snapshot()
		b.WriteString(name)
		if len(rows) == 0 {
			b.WriteString(": no adjacencies (ze heard no IIH from FRR)\n")
			continue
		}
		b.WriteByte('\n')
		for _, r := range rows {
			b.WriteString("  sys=")
			b.WriteString(r.SystemID)
			b.WriteString(" snpa=")
			b.WriteString(r.SNPA)
			b.WriteString(" level=")
			b.WriteString(r.Level)
			b.WriteString(" state=")
			b.WriteString(r.State)
			b.WriteString(" ipv4=")
			b.WriteString(r.IPv4)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// VALIDATES: ze forms an IS-IS L1 adjacency with a real FRR isisd peer over raw
// Layer 2 and synchronizes its LSDB (receives FRR's LSP). Genuine
// cross-implementation, on-the-wire interop.
func TestISISInteropFRR(t *testing.T) {
	startISISFRRPeer(t)

	cfgZe := fmt.Sprintf(`{"isis":{"net":%q,"interfaces":{"interface":{%q:{"hello-interval":"1","level":"l1"}}}}}`, zeISISNet, zeISISVeth)
	eng := startRealEngine(t, zeISISVeth, cfgZe)
	defer eng.shutdown()

	frrNET, err := types.ParseNET(frrISISNet)
	require.NoError(t, err)
	frrSys := frrNET.SystemID()

	// Poll for an Up adjacency AND FRR's LSP appearing in ze's LSDB.
	var up, synced bool
	deadline := time.Now().Add(75 * time.Second)
	for time.Now().Before(deadline) {
		up = engUpInt(eng)
		synced = lsdbHasSystem(eng, lsdb.Level1, frrSys)
		if up && synced {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if !up || !synced {
		t.Logf("FRR zebra.log:\n%s", frrISISshSoft("tail -40 %s/zebra.log", frrISISRunDir))
		t.Logf("FRR isisd.log:\n%s", frrISISshSoft("tail -80 %s/isisd.log", frrISISRunDir))
		vtysh := frrISISBin("vtysh")
		t.Logf("FRR isis neighbor:\n%s", frrISISshSoft("ip netns exec %s %s -c 'show isis neighbor' 2>&1", frrISISNS, vtysh))
		t.Logf("FRR isis interface detail:\n%s", frrISISshSoft("ip netns exec %s %s -c 'show isis interface detail' 2>&1", frrISISNS, vtysh))
		t.Logf("FRR running config:\n%s", frrISISshSoft("ip netns exec %s %s -c 'show running-config' 2>&1", frrISISNS, vtysh))
		t.Logf("FRR isis database:\n%s", frrISISshSoft("ip netns exec %s %s -c 'show isis database detail' 2>&1", frrISISNS, vtysh))
		t.Logf("ze adjacencies:\n%s", dumpZeAdjacencies(eng))
		t.Logf("frr-side capture (%s):\n%s", frrISISVeth, frrISISshSoft("tcpdump -tt -e -nn -r %s/frr.pcap 2>&1 | head -40", frrISISRunDir))
		t.Logf("ze-side capture (%s):\n%s", zeISISVeth, frrISISshSoft("tcpdump -tt -e -nn -r %s/ze.pcap 2>&1 | head -40", frrISISRunDir))
	}

	require.True(t, up, "ze did not reach an Up IS-IS adjacency with FRR isisd")
	require.True(t, synced, "ze did not receive FRR's LSP into its LSDB")
	t.Logf("interop OK: ze LSDB L1 holds %d LSP(s), including FRR system-id %s", eng.lsdb.Len(lsdb.Level1), frrSys)
}

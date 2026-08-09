//go:build integration && linux

// Design: docs/architecture/isis/isis-5-adjacency.md -- adjacency QEMU integration test.
//
// VALIDATES (AC-1/AC-3, Wiring Test "two engines on a veth"): two real IS-IS
// engines, each on one end of a veth pair inside a dedicated network namespace,
// open AF_PACKET circuits, exchange padded LAN IIHs over real Layer 2, run the
// LAN three-way check, and BOTH adjacencies reach Up. This is the raw-L2 proof
// the in-memory TestISISAdjacencyUp cannot give; it requires CAP_NET_ADMIN (to
// create the netns/veth) and CAP_NET_RAW (to open the raw socket) and t.Skips
// when those are absent. It runs under ze-qemu-integration-test (build tag
// `integration && linux`) and is listed in scripts/evidence/qemu-all-tests.sh.
// PREVENTS: a regression where the engine forms adjacencies in-memory but not
// over a real socket (e.g. wrong multicast MAC, padding that the kernel rejects,
// or the source-MAC threading breaking the three-way echo).

package isis

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/ze-software/ze/internal/component/iface"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink" // register the netlink iface backend so iface.Resolve works
	"github.com/ze-software/ze/internal/plugins/isis/transport"
)

const (
	vethA = "zeisisadj0"
	vethB = "zeisisadj1"
)

// withVethPair creates a veth pair in a fresh network namespace, brings both
// ends up, and runs fn inside that namespace. It skips when CAP_NET_ADMIN is
// absent. Mirrors the transport package's helper (kept local to avoid exporting
// test-only plumbing across packages).
func withVethPair(t *testing.T, fn func()) {
	t.Helper()
	runtime.LockOSThread()
	unlocked := false
	unlock := func() {
		if !unlocked {
			runtime.UnlockOSThread()
			unlocked = true
		}
	}

	origNS, err := netns.Get()
	if err != nil {
		unlock()
		t.Skipf("requires CAP_NET_ADMIN: %v", err)
	}
	newNS, err := netns.NewNamed("zeisisadj")
	if err != nil {
		origNS.Close()
		unlock()
		t.Skipf("requires CAP_NET_ADMIN: %v", err)
	}
	t.Cleanup(func() {
		if rerr := netns.Set(origNS); rerr != nil {
			t.Errorf("restore namespace: %v", rerr)
		}
		origNS.Close()
		newNS.Close()
		netns.DeleteNamed("zeisisadj") //nolint:errcheck // best-effort cleanup
		unlock()
	})

	if err := netlink.LinkAdd(&netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: vethA, MTU: 1500},
		PeerName:  vethB,
	}); err != nil {
		t.Skipf("add veth (needs CAP_NET_ADMIN): %v", err)
	}
	for _, name := range []string{vethA, vethB} {
		link, lerr := netlink.LinkByName(name)
		if lerr != nil {
			t.Fatalf("link %q: %v", name, lerr)
		}
		if uerr := netlink.LinkSetUp(link); uerr != nil {
			t.Fatalf("up %q: %v", name, uerr)
		}
	}
	fn()
}

// startRealEngine builds an engine over the real AF_PACKET backend, configured
// for IS-IS on the given veth interface, and opens its circuits. It skips when
// CAP_NET_RAW is unavailable.
func startRealEngine(t *testing.T, ifaceName, jsonCfg string) *engine {
	t.Helper()
	// IS-IS now resolves its interfaces through the iface resolver, which needs
	// the netlink backend loaded (as it always is in production). Load it so
	// OpenCircuit -> iface.Resolve finds the real device.
	if err := iface.LoadBackend("netlink"); err != nil {
		t.Fatalf("load iface backend: %v", err)
	}
	cfg, err := parseISISConfig(sec(jsonCfg))
	if err != nil {
		t.Fatalf("parseISISConfig: %v", err)
	}
	eng := newEngine(transport.New(transport.NewBackend()))
	eng.setConfig(cfg)
	if oerr := eng.openCircuits(); oerr != nil {
		if strings.Contains(oerr.Error(), "CAP_NET_RAW") {
			t.Skipf("requires CAP_NET_RAW: %v", oerr)
		}
		t.Fatalf("openCircuits(%s): %v", ifaceName, oerr)
	}
	return eng
}

// TestISISAdjacencyUpVeth: two engines on opposite ends of a veth pair form an
// L1 adjacency that reaches Up over real Layer 2.
func TestISISAdjacencyUpVeth(t *testing.T) {
	withVethPair(t, func() {
		const cfgA = `{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"zeisisadj0":{"hello-interval":"1","level":"l1"}}}}}`
		const cfgB = `{"isis":{"net":"49.0001.0000.0000.0002.00","interfaces":{"interface":{"zeisisadj1":{"hello-interval":"1","level":"l1"}}}}}`

		engA := startRealEngine(t, vethA, cfgA)
		engB := startRealEngine(t, vethB, cfgB)
		defer engA.shutdown()
		defer engB.shutdown()

		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			if engUpInt(engA) && engUpInt(engB) {
				return // adjacency Up on both sides over real L2
			}
			time.Sleep(200 * time.Millisecond)
		}
		t.Fatalf("adjacency did not reach Up over veth: A up=%v B up=%v", engUpInt(engA), engUpInt(engB))
	})
}

// engUpInt reports whether the engine has at least one Up adjacency.
func engUpInt(e *engine) bool {
	e.circuitsMu.RLock()
	defer e.circuitsMu.RUnlock()
	for _, c := range e.circuitByName {
		if c.Table().UpCount() > 0 {
			return true
		}
	}
	return false
}

//go:build integration && linux

package trafficnetlink

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/ze-software/ze/internal/component/traffic"
)

func withTrafficNetNS(t *testing.T, fn func()) {
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
		t.Skipf("requires CAP_NET_ADMIN: cannot get current namespace: %v", err)
	}

	nsName := trafficNetNSName(t.Name())
	newNS, err := netns.NewNamed(nsName)
	if err != nil {
		origNS.Close()
		unlock()
		t.Skipf("requires CAP_NET_ADMIN: cannot create namespace: %v", err)
	}

	t.Cleanup(func() {
		if restoreErr := netns.Set(origNS); restoreErr != nil {
			t.Errorf("failed to restore original namespace: %v", restoreErr)
		}
		origNS.Close()
		newNS.Close()
		netns.DeleteNamed(nsName) //nolint:errcheck // best-effort cleanup
		unlock()
	})

	fn()
}

func trafficNetNSName(testName string) string {
	name := strings.NewReplacer("/", "_", " ", "_", "(", "", ")", "").Replace(testName)
	if len(name) > 8 {
		name = name[len(name)-8:]
	}
	return "zetc_" + name
}

func addTrafficVeth(t *testing.T, name, peer string) netlink.Link {
	t.Helper()

	if err := netlink.LinkAdd(&netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: name}, PeerName: peer}); err != nil {
		t.Fatalf("add veth %q/%q: %v", name, peer, err)
	}
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("link %q: %v", name, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("set %q up: %v", name, err)
	}
	peerLink, err := netlink.LinkByName(peer)
	if err != nil {
		t.Fatalf("link %q: %v", peer, err)
	}
	if err := netlink.LinkSetUp(peerLink); err != nil {
		t.Fatalf("set %q up: %v", peer, err)
	}
	return link
}

// movePeerToNetNS moves the veth peer into a throwaway namespace of its own and
// gives it addr there.
//
// Without this both ends live in ONE namespace, so the peer's address is a LOCAL
// address and Linux routes traffic to it over loopback -- it never egresses the
// interface under test, and its qdisc counts nothing. TestCS6ClassifyNetns
// asserted on a control-class packet count that could therefore only ever be
// zero; the root qdisc reported 0 packets, which is what identified the topology
// rather than the classifier.
//
// The caller must already be inside withTrafficNetNS (OS thread locked).
func movePeerToNetNS(t *testing.T, peer string, addr *netlink.Addr) {
	t.Helper()

	// testNS is the namespace withTrafficNetNS put us in, and every later
	// netns.Set here returns to it. Its fd must outlive this function: the
	// cleanup below uses it. Closing it on return (a plain `defer`) leaves the
	// cleanup setting a closed fd, which reports as "bad file descriptor".
	testNS, err := netns.Get()
	if err != nil {
		t.Fatalf("get current namespace: %v", err)
	}

	nsName := trafficNetNSName(t.Name()) + "_peer"
	peerNS, err := netns.NewNamed(nsName)
	if err != nil {
		testNS.Close()
		t.Skipf("requires CAP_NET_ADMIN: cannot create peer namespace: %v", err)
	}
	// NewNamed switches us into the new namespace; go back before touching the
	// peer link, which is still in the original one.
	if setErr := netns.Set(testNS); setErr != nil {
		testNS.Close()
		t.Fatalf("restore namespace after creating %s: %v", nsName, setErr)
	}
	t.Cleanup(func() {
		if restoreErr := netns.Set(testNS); restoreErr != nil {
			t.Errorf("restore namespace: %v", restoreErr)
		}
		testNS.Close()
		peerNS.Close()
		netns.DeleteNamed(nsName) //nolint:errcheck // best-effort cleanup
	})

	peerLink, err := netlink.LinkByName(peer)
	if err != nil {
		t.Fatalf("link %q: %v", peer, err)
	}
	if err := netlink.LinkSetNsFd(peerLink, int(peerNS)); err != nil {
		t.Fatalf("move %q into %s: %v", peer, nsName, err)
	}

	if err := netns.Set(peerNS); err != nil {
		t.Fatalf("enter peer namespace: %v", err)
	}
	defer func() {
		if restoreErr := netns.Set(testNS); restoreErr != nil {
			t.Fatalf("restore namespace after peer setup: %v", restoreErr)
		}
	}()

	inNS, err := netlink.LinkByName(peer)
	if err != nil {
		t.Fatalf("link %q inside %s: %v", peer, nsName, err)
	}
	if err := netlink.LinkSetUp(inNS); err != nil {
		t.Fatalf("set %q up inside %s: %v", peer, nsName, err)
	}
	if err := netlink.AddrAdd(inNS, addr); err != nil {
		t.Fatalf("addr add %q inside %s: %v", peer, nsName, err)
	}
}

func replaceRootFQ(t *testing.T, link netlink.Link) {
	t.Helper()

	qdisc := &netlink.Fq{
		QdiscAttrs: netlink.QdiscAttrs{
			LinkIndex: link.Attrs().Index,
			Handle:    0,
			Parent:    netlink.HANDLE_ROOT,
		},
		Pacing:  1,
		Quantum: 1514,
	}
	if err := netlink.QdiscReplace(qdisc); err != nil {
		t.Fatalf("install original fq qdisc: %v", err)
	}
}

func rootQdiscTypeInKernel(t *testing.T, ifaceName string) string {
	t.Helper()

	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		t.Fatalf("link %q: %v", ifaceName, err)
	}
	qdiscs, err := netlink.QdiscList(link)
	if err != nil {
		t.Fatalf("list qdiscs for %q: %v", ifaceName, err)
	}
	root, err := rootQdisc(qdiscs)
	if err != nil {
		t.Fatalf("root qdisc for %q: %v", ifaceName, err)
	}
	return root.Type()
}

// VALIDATES: P1 traffic-control -- real kernel qdisc snapshot survives backend restart.
// PREVENTS: removing traffic-control restoring synthetic fq_codel instead of the original qdisc.
func TestNetlinkIntegration_RestoreOriginalQdiscAfterRestart(t *testing.T) {
	withTrafficNetNS(t, func() {
		const ifaceName = "ze_tc0"
		link := addTrafficVeth(t, ifaceName, "ze_tc1")
		replaceRootFQ(t, link)
		if got := rootQdiscTypeInKernel(t, ifaceName); got != "fq" {
			t.Fatalf("initial root qdisc = %q, want fq", got)
		}

		registerSnapshotStore(t)
		b := newBackendWithOps(netlinkOps{}, nil, "boot-1", nil)
		desired := map[string]traffic.InterfaceQoS{
			ifaceName: {
				Interface: ifaceName,
				Qdisc: traffic.Qdisc{
					Type:         traffic.QdiscHTB,
					DefaultClass: "default",
					Classes: []traffic.TrafficClass{
						{Name: "default", Rate: 1_000_000, Ceil: 1_000_000},
					},
				},
			},
		}
		if err := b.Apply(context.Background(), desired); err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if got := rootQdiscTypeInKernel(t, ifaceName); got != "htb" {
			t.Fatalf("applied root qdisc = %q, want htb", got)
		}
		loaded, err := loadTCSnapshots()
		if err != nil {
			t.Fatalf("load snapshots after Apply: %v", err)
		}
		if len(loaded) == 0 {
			t.Fatal("no snapshot persisted after Apply")
		}
		restarted := newBackendWithOps(netlinkOps{}, nil, "boot-1", loaded)
		if err := restarted.RestoreOriginal(context.Background(), ifaceName); err != nil {
			t.Fatalf("RestoreOriginal after restart: %v", err)
		}
		if got := rootQdiscTypeInKernel(t, ifaceName); got != "fq" {
			t.Fatalf("restored root qdisc = %q, want fq", got)
		}
		remaining, err := loadTCSnapshots()
		if err != nil {
			t.Fatalf("load snapshots after restore: %v", err)
		}
		if len(remaining) != 0 {
			t.Fatalf("snapshots still persisted after restore = %v, want empty", remaining)
		}
	})
}

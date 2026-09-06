//go:build integration && linux

package trafficnetlink

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

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
		origNS.Close() //nolint:errcheck // best-effort cleanup
		unlock()
		t.Skipf("requires CAP_NET_ADMIN: cannot create namespace: %v", err)
	}

	t.Cleanup(func() {
		if restoreErr := netns.Set(origNS); restoreErr != nil {
			t.Errorf("failed to restore original namespace: %v", restoreErr)
		}
		origNS.Close()            //nolint:errcheck // best-effort cleanup
		newNS.Close()             //nolint:errcheck // best-effort cleanup
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

	if err := netlink.LinkAdd(&netlink.Veth{Name: name, PeerName: peer}); err != nil {
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
		testNS.Close() //nolint:errcheck // best-effort cleanup
		t.Skipf("requires CAP_NET_ADMIN: cannot create peer namespace: %v", err)
	}
	// NewNamed switches us into the new namespace; go back before touching the
	// peer link, which is still in the original one.
	if setErr := netns.Set(testNS); setErr != nil {
		testNS.Close() //nolint:errcheck // best-effort cleanup
		t.Fatalf("restore namespace after creating %s: %v", nsName, setErr)
	}
	t.Cleanup(func() {
		if restoreErr := netns.Set(testNS); restoreErr != nil {
			t.Errorf("restore namespace: %v", restoreErr)
		}
		testNS.Close()            //nolint:errcheck // best-effort cleanup
		peerNS.Close()            //nolint:errcheck // best-effort cleanup
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
		LinkIndex: link.Attrs().Index,
		Handle:    0,
		Parent:    netlink.HANDLE_ROOT,
		Pacing:    1,
		Quantum:   1514,
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
		if got := rootQdiscTypeInKernel(t, ifaceName); got != qdiscTypeHTB {
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

// ingressPolicerInKernel reads the police action the kernel holds at the
// backend's priority on an interface's ingress hook. It reports ok=false when
// no policer is installed there.
func ingressPolicerInKernel(t *testing.T, ifaceName string) (rateBytesPerSec uint32, ok bool) {
	t.Helper()

	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		t.Fatalf("link %q: %v", ifaceName, err)
	}
	filters, err := netlink.FilterList(link, netlink.HANDLE_MIN_INGRESS)
	if err != nil {
		t.Fatalf("list ingress filters for %q: %v", ifaceName, err)
	}
	for _, f := range filters {
		matchall, isMatchAll := f.(*netlink.MatchAll)
		if !isMatchAll || matchall.Priority != policerFilterPriority {
			continue
		}
		for _, action := range matchall.Actions {
			if police, isPolice := action.(*netlink.PoliceAction); isPolice {
				return police.Rate, true
			}
		}
	}
	return 0, false
}

// ingressFilterPresent reports whether the ingress hook holds a filter at the
// given priority.
func ingressFilterPresent(t *testing.T, ifaceName string, priority uint16) bool {
	t.Helper()

	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		t.Fatalf("link %q: %v", ifaceName, err)
	}
	filters, err := netlink.FilterList(link, netlink.HANDLE_MIN_INGRESS)
	if err != nil {
		t.Fatalf("list ingress filters for %q: %v", ifaceName, err)
	}
	for _, f := range filters {
		if f.Attrs().Priority == priority {
			return true
		}
	}
	return false
}

// VALIDATES: the subscriber upload rate reaches the kernel, and leaves it again
// on teardown.
//
// Goal: prove the ingress policer is really installed, not merely translated.
// Method: apply an InterfaceQoS carrying an Ingress policer to a veth in a
// throwaway namespace, then read the police action back out of the kernel by
// listing the ingress hook. A unit test over the translator proves the netlink
// message is well formed; only this proves the kernel accepted it.
//
// PREVENTS: a return to the state where upload-rate was stored, reported by
// `show l2tp shaper`, and enforced by nothing.
func TestNetlinkIntegration_IngressPolicerReachesTheKernel(t *testing.T) {
	withTrafficNetNS(t, func() {
		const ifaceName = "ze_tc4"
		link := addTrafficVeth(t, ifaceName, "ze_tc5")
		replaceRootFQ(t, link)

		registerSnapshotStore(t)
		b := newBackendWithOps(netlinkOps{}, nil, "boot-1", nil)
		desired := map[string]traffic.InterfaceQoS{
			ifaceName: {
				Interface: ifaceName,
				Qdisc: traffic.Qdisc{
					Type:         traffic.QdiscHTB,
					DefaultClass: "default",
					Classes: []traffic.TrafficClass{
						{Name: "default", Rate: 10_000_000, Ceil: 10_000_000},
					},
				},
				Ingress: traffic.NewPolicer(8_000_000),
			},
		}
		if err := b.Apply(context.Background(), desired); err != nil {
			t.Fatalf("Apply: %v", err)
		}

		rate, ok := ingressPolicerInKernel(t, ifaceName)
		if !ok {
			t.Fatal("the kernel holds no police action on the ingress hook: the upload rate is enforced by nothing")
		}
		// The kernel carries the policed rate in bytes per second.
		if rate != 1_000_000 {
			t.Fatalf("kernel police rate = %d bytes/s, want 1000000 (8 Mbit/s)", rate)
		}

		if err := b.RestoreOriginal(context.Background(), ifaceName); err != nil {
			t.Fatalf("RestoreOriginal: %v", err)
		}
		if _, stillThere := ingressPolicerInKernel(t, ifaceName); stillThere {
			t.Fatal("the police action survived teardown: the next session on this interface inherits it")
		}
	})
}

// VALIDATES: the policer coexists with the two other owners of the clsact hook.
//
// Goal: prove that installing a subscriber policer does not disturb a filter
// another subsystem put on the same qdisc, and that removing the policer leaves
// that filter in place. The mirror path owns priority 1 and flow-export
// sampling owns priority 100; the policer is the third owner.
//
// Method: install a matchall filter at priority 1 by hand, standing in for the
// mirror, then apply and tear down the policer around it.
func TestNetlinkIntegration_IngressPolicerLeavesOtherHookOwnersAlone(t *testing.T) {
	withTrafficNetNS(t, func() {
		const ifaceName = "ze_tc6"
		link := addTrafficVeth(t, ifaceName, "ze_tc7")
		replaceRootFQ(t, link)
		linkIndex := link.Attrs().Index

		if err := netlink.QdiscAdd(ingressClsactQdisc(linkIndex)); err != nil {
			t.Fatalf("pre-create clsact qdisc: %v", err)
		}
		neighbor := &netlink.MatchAll{
			LinkIndex: linkIndex,
			Parent:    netlink.HANDLE_MIN_INGRESS,
			Priority:  1,
			Protocol:  unix.ETH_P_ALL,
			Actions: []netlink.Action{
				&netlink.GenericAction{Action: netlink.TC_ACT_PIPE},
			},
		}
		if err := netlink.FilterAdd(neighbor); err != nil {
			t.Fatalf("install the stand-in mirror filter: %v", err)
		}

		registerSnapshotStore(t)
		b := newBackendWithOps(netlinkOps{}, nil, "boot-1", nil)
		desired := map[string]traffic.InterfaceQoS{
			ifaceName: {
				Interface: ifaceName,
				Qdisc: traffic.Qdisc{
					Type:         traffic.QdiscHTB,
					DefaultClass: "default",
					Classes:      []traffic.TrafficClass{{Name: "default", Rate: 1_000_000, Ceil: 1_000_000}},
				},
				Ingress: traffic.NewPolicer(2_000_000),
			},
		}
		if err := b.Apply(context.Background(), desired); err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if !ingressFilterPresent(t, ifaceName, 1) {
			t.Fatal("installing the policer removed the other subsystem's filter")
		}
		if _, ok := ingressPolicerInKernel(t, ifaceName); !ok {
			t.Fatal("no policer installed beside the existing filter")
		}

		if err := b.RestoreOriginal(context.Background(), ifaceName); err != nil {
			t.Fatalf("RestoreOriginal: %v", err)
		}
		if !ingressFilterPresent(t, ifaceName, 1) {
			t.Fatal("removing the policer took the other subsystem's filter with it")
		}
	})
}

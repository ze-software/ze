//go:build integration && linux

package trafficnetlink

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
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

		path := filepath.Join(t.TempDir(), "state", "traffic-tc-snapshots.json")
		b := newBackendWithOps(netlinkOps{}, path, nil, "boot-1", nil)
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
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("snapshot file missing after Apply: %v", err)
		}

		loaded, err := loadTCSnapshots(path)
		if err != nil {
			t.Fatalf("load snapshots after Apply: %v", err)
		}
		restarted := newBackendWithOps(netlinkOps{}, path, nil, "boot-1", loaded)
		if err := restarted.RestoreOriginal(context.Background(), ifaceName); err != nil {
			t.Fatalf("RestoreOriginal after restart: %v", err)
		}
		if got := rootQdiscTypeInKernel(t, ifaceName); got != "fq" {
			t.Fatalf("restored root qdisc = %q, want fq", got)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("snapshot file still exists after restore: %v", err)
		}
	})
}

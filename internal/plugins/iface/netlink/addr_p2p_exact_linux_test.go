// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- Exact P2P removal.
// Related: addr_primary_linux.go -- Installed-pair lookup before deletion.

//go:build linux

package ifacenetlink

import (
	"reflect"
	"testing"

	"github.com/vishvananda/netlink"
)

// exactRemovalFixture feeds the production lookup and guarded removal with a
// kernel-shaped dump. Delete records the exact resolved address sent to netlink.
type exactRemovalFixture struct {
	request   *netlink.Addr
	installed []netlink.Addr
	deleted   *netlink.Addr
	order     []string
}

func (f *exactRemovalFixture) List(_ string, target *deviceAddress) ([]deviceAddress, error) {
	f.order = append(f.order, "list")
	remover := &netlinkAddrRemover{addr: f.request, exactPair: true}
	existing, err := remover.resolve("ppp7", f.installed, target)
	if err != nil {
		return nil, err
	}
	f.request = remover.addr
	return existing, nil
}

func (f *exactRemovalFixture) Delete(_ string, _ deviceAddress) error {
	f.order = append(f.order, "delete")
	f.deleted = f.request
	return nil
}

func exactRemovalPair(t *testing.T, local, peer string) *netlink.Addr {
	t.Helper()
	addr, err := netlink.ParseAddr(local)
	if err != nil {
		t.Fatal(err)
	}
	remote, err := netlink.ParseAddr(peer)
	if err != nil {
		t.Fatal(err)
	}
	addr.Peer = remote.IPNet
	return addr
}

// TestP2PRemovalRefusesAbsentPeerInInstalledSubnet reproduces Linux's dangerous
// subnet match: a request for absent peer .9/24 must never delete peer .1/24.
func TestP2PRemovalRefusesAbsentPeerInInstalledSubnet(t *testing.T) {
	request := exactRemovalPair(t, "198.51.100.2/32", "192.0.2.9/24")
	f := &exactRemovalFixture{
		request: request,
		installed: []netlink.Addr{
			*exactRemovalPair(t, "198.51.100.2/32", "192.0.2.1/24"),
		},
	}
	target, ok := toDeviceAddress(request)
	if !ok {
		t.Fatal("valid request rejected")
	}
	if err := removeAddressGuarded(f, "ppp7", target); err == nil {
		t.Fatal("absent exact peer accepted")
	}
	if f.deleted != nil {
		t.Fatalf("absent peer caused deletion of %v peer %v", f.deleted.IPNet, f.deleted.Peer)
	}
	if !reflect.DeepEqual(f.order, []string{"list"}) {
		t.Fatalf("operation order = %v, want list only", f.order)
	}
	if !f.installed[0].Peer.IP.Equal(netlinkPeerIP(t, "192.0.2.1/24")) {
		t.Fatal("installed peer changed after refused removal")
	}
}

func netlinkPeerIP(t *testing.T, cidr string) []byte {
	t.Helper()
	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		t.Fatal(err)
	}
	return addr.IP
}

// TestP2PRemovalNormalizesLocalBeforePromotion supplies a local /24 request,
// while the installed dump carries /32. The exact old peer must be selected
// even with a different peer first, and promotion must precede deletion.
func TestP2PRemovalNormalizesLocalBeforePromotion(t *testing.T) {
	request := exactRemovalPair(t, "198.51.100.2/24", "192.0.2.1/24")
	old := exactRemovalPair(t, "198.51.100.2/32", "192.0.2.1/24")
	newer := exactRemovalPair(t, "198.51.100.3/32", "192.0.2.1/24")
	f := &exactRemovalFixture{
		request: request,
		installed: []netlink.Addr{
			*exactRemovalPair(t, "198.51.100.2/32", "203.0.113.1/24"),
			*old, *newer,
		},
	}
	read, write := addrSysctlRead, addrSysctlWrite
	t.Cleanup(func() { addrSysctlRead, addrSysctlWrite = read, write })
	addrSysctlRead = func(string) (string, error) { return "0", nil }
	addrSysctlWrite = func(path, value string) error {
		if path != promoteSecondariesKnob("ppp7") {
			t.Fatalf("unexpected promotion path %s", path)
		}
		if value != "1" {
			t.Fatalf("unexpected promotion write %s=%s", path, value)
		}
		f.order = append(f.order, "promote")
		return nil
	}
	target, ok := toDeviceAddress(request)
	if !ok {
		t.Fatal("valid request rejected")
	}
	if err := removeAddressGuarded(f, "ppp7", target); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.order, []string{"list", "promote", "delete"}) {
		t.Fatalf("operation order = %v, want list, promote, delete", f.order)
	}
	if f.deleted == nil {
		t.Fatal("installed pair was not deleted")
	}
	if !f.deleted.Equal(*old) {
		t.Fatalf("deleted local %v, want %v", f.deleted.IPNet, old.IPNet)
	}
	if !f.deleted.PeerEqual(*old) {
		t.Fatalf("deleted %v peer %v, want %v peer %v", f.deleted.IPNet, f.deleted.Peer, old.IPNet, old.Peer)
	}
	if f.deleted.Equal(*newer) {
		t.Fatal("deleted the replacement local address")
	}
}

// TestP2PRemovalKeepsResolvedIdentityForEqualEndpoints puts a different peer
// first, the normalized equal-endpoint target second, and its replacement
// third. The guard must protect the replacement using the selected target.
func TestP2PRemovalKeepsResolvedIdentityForEqualEndpoints(t *testing.T) {
	request := exactRemovalPair(t, "192.0.2.2/24", "192.0.2.2/32")
	old, err := netlink.ParseAddr("192.0.2.2/32")
	if err != nil {
		t.Fatal(err)
	}
	newer := exactRemovalPair(t, "192.0.2.3/32", "192.0.2.2/32")
	f := &exactRemovalFixture{
		request: request,
		installed: []netlink.Addr{
			*exactRemovalPair(t, "192.0.2.2/32", "192.0.2.9/32"),
			*old, *newer,
		},
	}
	read, write := addrSysctlRead, addrSysctlWrite
	t.Cleanup(func() { addrSysctlRead, addrSysctlWrite = read, write })
	addrSysctlRead = func(string) (string, error) { return "0", nil }
	addrSysctlWrite = func(path, value string) error {
		if path != promoteSecondariesKnob("ppp7") {
			t.Fatalf("unexpected promotion path %s", path)
		}
		if value != "1" {
			t.Fatalf("unexpected promotion value %s", value)
		}
		f.order = append(f.order, "promote")
		return nil
	}
	target, ok := toDeviceAddress(request)
	if !ok {
		t.Fatal("valid equal-endpoint request rejected")
	}
	if err := removeAddressGuarded(f, "ppp7", target); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.order, []string{"list", "promote", "delete"}) {
		t.Fatalf("operation order = %v, want list, promote, delete", f.order)
	}
	if f.deleted != &f.installed[1] {
		t.Fatalf("deletion target = %p, want exact second dump entry %p", f.deleted, &f.installed[1])
	}
	if !f.deleted.Equal(*old) {
		t.Fatalf("deleted local %v, want %v", f.deleted.IPNet, old.IPNet)
	}
	if f.deleted.Peer != nil {
		t.Fatalf("selected equal-endpoint identity gained peer %v", f.deleted.Peer)
	}
	if f.deleted.Equal(*newer) {
		t.Fatal("deleted the replacement local address")
	}
}

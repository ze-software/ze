//go:build integration && linux

// Design: docs/research/l2tpv2-ze-integration.md -- original PPP mutation ownership
// Related: kernel_counter_owner_integration_linux_test.go -- real unit and namespace fixtures
// Upstream: github.com/vishvananda/netlink v1.3.1, native link/address operations:
// https://github.com/vishvananda/netlink/blob/v1.3.1/link_linux.go
// https://github.com/vishvananda/netlink/blob/v1.3.1/addr_linux.go
// Upstream: Linux v7.3, PPP unit flags and unit-number ioctls:
// https://github.com/torvalds/linux/blob/v7.3/include/uapi/linux/ppp-ioctl.h

package l2tp

import (
	"errors"
	"net/netip"
	"reflect"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestKernelMutatorOwnerMTURenameKeepsReplacement first proves useful native
// MTU configuration, then requires an old-name replacement to remain untouched.
func TestKernelMutatorOwnerMTURenameKeepsReplacement(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := newKernelMutatorFixture(t)
	if err := fixture.handle.SetMTU(1400); err != nil {
		t.Fatalf("configure original PPP MTU positive control: %v", err)
	}
	if got := fixture.link(t, fixture.original).Attrs().MTU; got != 1400 {
		t.Fatalf("original MTU positive control=%d, want 1400", got)
	}
	fixture.renameAndReplace(t)
	foreign, other := kernelMutatorForeign(t)
	if got := kernelMutatorLink(t, foreign, other).Attrs().MTU; got != 1700 {
		t.Fatalf("foreign MTU positive control=%d, want 1700", got)
	}

	mutationErr := fixture.handle.SetMTU(1300)
	if got := kernelMutatorLink(t, foreign, other).Attrs().MTU; got != 1700 {
		t.Fatalf("original PPP mutation changed foreign namespace MTU to %d", got)
	}
	fixture.requireLiveUnit(t)
	original := fixture.link(t, fixture.original)
	replacement := fixture.link(t, fixture.replacement)
	if replacement.Attrs().MTU != 1600 {
		t.Fatalf("original PPP MTU call changed old-name replacement MTU to %d; result=%v",
			replacement.Attrs().MTU, mutationErr)
	}
	if mutationErr == nil {
		if original.Attrs().MTU != 1300 {
			t.Fatalf("successful original PPP MTU call left original MTU=%d, want 1300", original.Attrs().MTU)
		}
	} else {
		if original.Attrs().MTU != 1400 && original.Attrs().MTU != 1300 {
			t.Fatalf("failed original PPP MTU call left unexpected original MTU=%d; failure=%v",
				original.Attrs().MTU, mutationErr)
		}
	}
}

// TestKernelMutatorOwnerAddressWithdrawalRenameKeepsReplacement proves native
// pair withdrawal before requiring later cleanup to preserve both other devices.
func TestKernelMutatorOwnerAddressWithdrawalRenameKeepsReplacement(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := newKernelMutatorFixture(t)
	initialLocal := netip.MustParsePrefix("192.0.2.10/32")
	initialPeer := netip.MustParsePrefix("192.0.2.11/32")
	if err := fixture.handle.AddAddressP2P(initialLocal, initialPeer); err != nil {
		t.Fatalf("configure original PPP address positive control: %v", err)
	}
	initial := kernelMutatorPair(t, initialLocal, initialPeer)
	if !kernelMutatorHasPair(fixture.addresses(t, fixture.original), initial) {
		t.Fatal("original PPP address positive control was not installed on its native index")
	}
	if err := fixture.handle.RemoveAddress(initialLocal); err != nil {
		t.Fatalf("withdraw original positive-control pair before rename: %v", err)
	}
	if got := fixture.addresses(t, fixture.original); len(got) != 0 {
		t.Fatalf("original positive-control pair remained after withdrawal: %+v", got)
	}
	if err := fixture.handle.AddAddressP2P(initialLocal, initialPeer); err != nil {
		t.Fatalf("restore original pair for captured-owner withdrawal: %v", err)
	}
	if !kernelMutatorHasPair(fixture.addresses(t, fixture.original), initial) {
		t.Fatal("original withdrawal input was not restored before rename")
	}
	fixture.renameAndReplace(t)
	if err := fixture.creator.AddrAdd(fixture.replacement, initial); err != nil {
		t.Fatalf("install equal-pair replacement control: %v", err)
	}
	replacementPair := kernelMutatorPair(t, netip.MustParsePrefix("192.0.2.20/32"), netip.MustParsePrefix("192.0.2.21/32"))
	if err := fixture.creator.AddrAdd(fixture.replacement, replacementPair); err != nil {
		t.Fatalf("install replacement address positive control: %v", err)
	}
	replacementBefore := fixture.addresses(t, fixture.replacement)
	if !kernelMutatorHasPair(replacementBefore, replacementPair) {
		t.Fatal("replacement address positive control was not installed")
	}
	if !kernelMutatorHasPair(replacementBefore, initial) {
		t.Fatal("replacement equal-pair positive control was not installed")
	}
	foreign, other := kernelMutatorForeign(t)
	if err := foreign.AddrAdd(other, initial); err != nil {
		t.Fatalf("install equal-pair foreign namespace control: %v", err)
	}
	foreignPair := kernelMutatorPair(t, netip.MustParsePrefix("203.0.113.20/32"), netip.MustParsePrefix("203.0.113.21/32"))
	if err := foreign.AddrAdd(other, foreignPair); err != nil {
		t.Fatalf("install foreign namespace address positive control: %v", err)
	}
	foreignBefore := kernelMutatorAddresses(t, foreign, other)
	if !kernelMutatorHasPair(foreignBefore, foreignPair) {
		t.Fatal("foreign namespace address positive control was not installed")
	}
	if !kernelMutatorHasPair(foreignBefore, initial) {
		t.Fatal("foreign namespace equal-pair positive control was not installed")
	}

	mutationErr := fixture.handle.RemoveAddress(initialLocal)
	if got := kernelMutatorAddresses(t, foreign, other); !reflect.DeepEqual(got, foreignBefore) {
		t.Fatalf("original PPP withdrawal changed foreign namespace addresses: before=%+v after=%+v", foreignBefore, got)
	}
	fixture.requireLiveUnit(t)
	originalAfter := fixture.addresses(t, fixture.original)
	replacementAfter := fixture.addresses(t, fixture.replacement)
	if !reflect.DeepEqual(replacementAfter, replacementBefore) {
		t.Fatalf("original PPP withdrawal changed old-name replacement: before=%+v after=%+v result=%v",
			replacementBefore, replacementAfter, mutationErr)
	}
	if mutationErr == nil {
		if len(originalAfter) != 0 {
			t.Fatalf("successful original PPP withdrawal retained original pairs: %+v", originalAfter)
		}
	} else {
		if len(originalAfter) > 1 {
			t.Fatalf("failed original PPP withdrawal changed original pairs: %+v; failure=%v", originalAfter, mutationErr)
		}
		if len(originalAfter) == 1 && !kernelMutatorHasPair(originalAfter, initial) {
			t.Fatalf("failed original PPP withdrawal changed its original pair: %+v; failure=%v", originalAfter, mutationErr)
		}
	}
}

// kernelMutatorFixture borrows the actual PPP owner from kernelCounterOwner.
// Its test-only replacement is released after that owner's cleanup. Not safe
// for concurrent use; the test MUST leave PPP descriptor closure to that owner.
type kernelMutatorFixture struct {
	creator     *netlink.Handle
	handle      ppp.SessionHandle
	original    netlink.Link
	replacement netlink.Link
	unitFD      int
	unitNum     int
}

func newKernelMutatorFixture(t *testing.T) *kernelMutatorFixture {
	t.Helper()
	creator, err := netlink.NewHandle(unix.NETLINK_ROUTE)
	if err != nil {
		t.Fatal(err)
	}
	fixture := &kernelMutatorFixture{creator: creator}
	// Register before constructing the PPP owner: its address receipts must
	// settle while the task-owned replacement still exists in this namespace.
	t.Cleanup(func() {
		defer creator.Close()
		if fixture.replacement != nil {
			if err := creator.LinkDel(fixture.replacement); err != nil {
				t.Errorf("delete task-owned mutator replacement: %v", err)
			}
		}
	})
	fixture.handle, _, fixture.original, fixture.unitFD = kernelCounterOwner(t)
	fixture.unitNum, err = unix.IoctlGetInt(fixture.unitFD, 0x80047456)
	if err != nil {
		t.Fatalf("read original PPP unit identity: %v", err)
	}
	// Demand mode keeps an unattached unit's ordinary read EOF distinct from
	// POLLHUP. A numeric unit label alone is not proof the unit remains live.
	if err := unix.IoctlSetPointerInt(fixture.unitFD, 0x40047459, 0x200); err != nil {
		t.Fatalf("set original PPP SC_LOOP_TRAFFIC: %v", err)
	}
	fixture.requireLiveUnit(t)
	return fixture
}

func (fixture *kernelMutatorFixture) renameAndReplace(t *testing.T) {
	t.Helper()
	_, err := fixture.creator.LinkByIndex(129)
	if err == nil {
		t.Fatal("mutator fixture native replacement index is already occupied")
	}
	if _, absent := errors.AsType[netlink.LinkNotFoundError](err); !absent {
		t.Fatalf("cannot verify replacement index absence: %v", err)
	}
	if err := fixture.creator.LinkSetName(fixture.original, "zm-original"); err != nil {
		t.Fatalf("rename original PPP interface: %v", err)
	}
	replacement := &netlink.Dummy{}
	replacement.Name = "zc-owner"
	replacement.Index = 129
	replacement.MTU = 1600
	if err := fixture.creator.LinkAdd(replacement); err != nil {
		t.Fatalf("create old-name replacement: %v", err)
	}
	fixture.replacement = replacement
	actual := fixture.link(t, replacement)
	if actual.Attrs().Name != replacement.Name {
		t.Fatalf("native replacement name=%q, want %q", actual.Attrs().Name, replacement.Name)
	}
	if actual.Attrs().MTU != 1600 {
		t.Fatalf("replacement MTU positive control=%d, want 1600", actual.Attrs().MTU)
	}
	fixture.requireLiveUnit(t)
}

func (fixture *kernelMutatorFixture) requireLiveUnit(t *testing.T) {
	t.Helper()
	unit, err := unix.IoctlGetInt(fixture.unitFD, 0x80047456)
	if err != nil {
		t.Fatalf("original PPP unit descriptor is no longer usable: %v", err)
	}
	if unit != fixture.unitNum {
		t.Fatalf("original PPP unit label=%d, want %d", unit, fixture.unitNum)
	}
	poll := []unix.PollFd{{Fd: int32(fixture.unitFD), Events: unix.POLLIN}}
	if _, err := unix.Poll(poll, 0); err != nil {
		t.Fatal(err)
	}
	if poll[0].Revents&unix.POLLHUP != 0 {
		t.Fatalf("original PPP unit unexpectedly hung up: %#x", poll[0].Revents)
	}
}

func (fixture *kernelMutatorFixture) link(t *testing.T, link netlink.Link) netlink.Link {
	t.Helper()
	return kernelMutatorLink(t, fixture.creator, link)
}

func kernelMutatorLink(t *testing.T, creator *netlink.Handle, link netlink.Link) netlink.Link {
	t.Helper()
	actual, err := creator.LinkByIndex(link.Attrs().Index)
	if err != nil {
		t.Fatalf("read native mutator device index %d: %v", link.Attrs().Index, err)
	}
	return actual
}

func (fixture *kernelMutatorFixture) addresses(t *testing.T, link netlink.Link) []netlink.Addr {
	t.Helper()
	return kernelMutatorAddresses(t, fixture.creator, link)
}

func kernelMutatorAddresses(t *testing.T, creator *netlink.Handle, link netlink.Link) []netlink.Addr {
	t.Helper()
	addresses, err := creator.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		t.Fatalf("read native mutator addresses at index %d: %v", link.Attrs().Index, err)
	}
	return addresses
}

func kernelMutatorForeign(t *testing.T) (*netlink.Handle, netlink.Link) {
	t.Helper()
	var foreign *netlink.Handle
	var other netlink.Link
	kernelCounterOtherNamespace(t, func() {
		var err error
		foreign, err = netlink.NewHandle(unix.NETLINK_ROUTE)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(foreign.Close)
		dummy := &netlink.Dummy{}
		dummy.Name = "zc-owner"
		dummy.Index = 129
		dummy.MTU = 1700
		if err := foreign.LinkAdd(dummy); err != nil {
			t.Fatalf("create equal-name/index foreign namespace control: %v", err)
		}
		other = dummy
		t.Cleanup(func() {
			if err := foreign.LinkDel(other); err != nil {
				t.Errorf("delete task-owned foreign namespace control: %v", err)
			}
		})
	})
	return foreign, other
}

func kernelMutatorPair(t *testing.T, local, peer netip.Prefix) *netlink.Addr {
	t.Helper()
	address, err := netlink.ParseAddr(local.String())
	if err != nil {
		t.Fatal(err)
	}
	remote, err := netlink.ParseAddr(peer.String())
	if err != nil {
		t.Fatal(err)
	}
	address.Peer = remote.IPNet
	return address
}

func kernelMutatorHasPair(addresses []netlink.Addr, want *netlink.Addr) bool {
	for _, address := range addresses {
		if address.IPNet == nil {
			continue
		}
		if address.Peer == nil {
			continue
		}
		if !address.Equal(*want) {
			continue
		}
		if address.PeerEqual(*want) {
			return true
		}
	}
	return false
}

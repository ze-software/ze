// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md -- the walk that brings every filter down the device stack current
// Related: solicitednode_linux.go -- syncReceiveFilter and linkIsPeer, under test

package vrrp

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
)

// fakeStack is a device chain syncReceiveFilter walks: its lookup answers from
// devices and its no-op records the index it was sent to, in order.
type fakeStack struct {
	devices map[int]netlink.Link
	sent    []int
	lookups []int
}

func (s *fakeStack) lookup(index int) (netlink.Link, error) {
	s.lookups = append(s.lookups, index)
	link, found := s.devices[index]
	if !found {
		return nil, errors.New("no device at that index")
	}
	return link, nil
}

func (s *fakeStack) sendNoop(index int) error {
	s.sent = append(s.sent, index)
	return nil
}

// walk runs syncReceiveFilter from the device at top.
func (s *fakeStack) walk(t *testing.T, top int) error {
	t.Helper()
	link, found := s.devices[top]
	if !found {
		t.Fatalf("the fake stack holds no device %d", top)
	}
	return syncReceiveFilter(link, s.lookup, s.sendNoop)
}

// stackAttrs builds the attributes of a device in this namespace: index, the index
// of its link (0 for none), and the namespace id of that link (-1 for this one).
func stackAttrs(index, parent, netNsID int) netlink.LinkAttrs {
	return netlink.LinkAttrs{Index: index, Name: "dev" + string(rune('a'+index)), ParentIndex: parent, NetNsID: netNsID}
}

// newStack builds a fake stack from its devices, keyed by index.
func newStack(links ...netlink.Link) *fakeStack {
	devices := make(map[int]netlink.Link, len(links))
	for _, link := range links {
		devices[link.Attrs().Index] = link
	}
	return &fakeStack{devices: devices}
}

// vlanChain builds count devices stacked top down from index count to index
// 1: a macvlan on VLANs, each linked to the one below. bottom is the device at
// index 1, which the caller builds.
func vlanChain(count int, bottom netlink.Link) []netlink.Link {
	links := []netlink.Link{bottom}
	for index := 2; index < count; index++ {
		links = append(links, &netlink.Vlan{LinkAttrs: stackAttrs(index, index-1, -1)})
	}
	return append(links, &netlink.Macvlan{LinkAttrs: stackAttrs(count, count-1, -1)})
}

// TestReceiveFilterWalkSyncsEveryDeviceTopDown checks that a macvlan on a VLAN
// on a device with no link gets exactly three no-ops, top down.
//
// Method: a fake stack of three devices; the walk's no-ops are recorded in
// order. A walk cut to two devices, or one that runs bottom up, fails it.
func TestReceiveFilterWalkSyncsEveryDeviceTopDown(t *testing.T) {
	stack := newStack(vlanChain(3, &netlink.Device{LinkAttrs: stackAttrs(1, 0, -1)})...)
	if err := stack.walk(t, 3); err != nil {
		t.Fatalf("walk: %v", err)
	}
	if want := []int{3, 2, 1}; !slices.Equal(stack.sent, want) {
		t.Fatalf("no-ops sent to %v, want %v", stack.sent, want)
	}
}

// TestReceiveFilterWalkStopsAtAVethWhosePeerIsHere checks that a veth at the
// bottom of the stack ends the walk without its peer being fetched or synced.
//
// Method: the veth's link is its peer, in this namespace, and the peer links
// back to the veth. A walk that follows the peer sends it a fourth no-op.
func TestReceiveFilterWalkStopsAtAVethWhosePeerIsHere(t *testing.T) {
	peer := &netlink.Veth{LinkAttrs: stackAttrs(9, 1, -1)}
	stack := newStack(append(vlanChain(3, &netlink.Veth{LinkAttrs: stackAttrs(1, 9, -1)}), peer)...)
	if err := stack.walk(t, 3); err != nil {
		t.Fatalf("walk: %v", err)
	}
	if want := []int{3, 2, 1}; !slices.Equal(stack.sent, want) {
		t.Fatalf("no-ops sent to %v, want %v", stack.sent, want)
	}
	if slices.Contains(stack.lookups, 9) {
		t.Fatalf("the walk fetched the veth's peer (lookups %v)", stack.lookups)
	}
}

// TestReceiveFilterWalkStopsAtAVethWhosePeerIsInAnotherNamespace checks the
// container case: the macvlan sits on a veth whose peer is in the host
// namespace, and the walk ends at the veth with no error.
//
// Method: the veth reports its link in namespace 0. A walk that reads a link
// into another namespace as a lower device there reports a false error.
func TestReceiveFilterWalkStopsAtAVethWhosePeerIsInAnotherNamespace(t *testing.T) {
	stack := newStack(vlanChain(2, &netlink.Veth{LinkAttrs: stackAttrs(1, 7, 0)})...)
	if err := stack.walk(t, 2); err != nil {
		t.Fatalf("walk on a veth whose peer is in another namespace: %v", err)
	}
	if want := []int{2, 1}; !slices.Equal(stack.sent, want) {
		t.Fatalf("no-ops sent to %v, want %v", stack.sent, want)
	}
}

// TestReceiveFilterWalkRefusesALowerDeviceInAnotherNamespace checks that a
// VLAN whose lower device is in another namespace ends the walk with an error
// naming the namespace, and that the index is never looked up here.
//
// Method: the VLAN's link is index 1 in namespace 0, and this namespace holds
// an unrelated device at index 1. A walk that looks the index up here syncs
// that unrelated device and answers no error.
func TestReceiveFilterWalkRefusesALowerDeviceInAnotherNamespace(t *testing.T) {
	stack := newStack(
		&netlink.Device{LinkAttrs: stackAttrs(1, 0, -1)},
		&netlink.Vlan{LinkAttrs: stackAttrs(2, 1, 0)},
		&netlink.Macvlan{LinkAttrs: stackAttrs(3, 2, -1)},
	)
	err := stack.walk(t, 3)
	if err == nil {
		t.Fatalf("the walk answered no error for a lower device in another namespace (no-ops sent to %v)", stack.sent)
	}
	if !strings.Contains(err.Error(), "another network namespace (id 0)") {
		t.Fatalf("the error does not name the other namespace: %v", err)
	}
	if want := []int{3, 2}; !slices.Equal(stack.sent, want) {
		t.Fatalf("no-ops sent to %v, want %v", stack.sent, want)
	}
}

// TestReceiveFilterWalkSyncsTheDeepestStackTheKernelBuilds checks that a stack
// of MAX_NEST_DEV devices, a veth at its bottom whose peer is here, is walked
// whole with no error.
//
// Method: eight devices; the walk must send eight no-ops and answer no error.
// A walk that fetches the veth's peer counts it as a ninth device and errors.
func TestReceiveFilterWalkSyncsTheDeepestStackTheKernelBuilds(t *testing.T) {
	peer := &netlink.Veth{LinkAttrs: stackAttrs(9, 1, -1)}
	stack := newStack(append(vlanChain(receiveFilterDepthMax, &netlink.Veth{LinkAttrs: stackAttrs(1, 9, -1)}), peer)...)
	if err := stack.walk(t, receiveFilterDepthMax); err != nil {
		t.Fatalf("walk of %d devices: %v", receiveFilterDepthMax, err)
	}
	if want := []int{8, 7, 6, 5, 4, 3, 2, 1}; !slices.Equal(stack.sent, want) {
		t.Fatalf("no-ops sent to %v, want %v", stack.sent, want)
	}
}

// TestReceiveFilterWalkRefusesAStackPastTheBound checks that a stack of nine
// devices, deeper than the kernel builds, ends the walk with an error.
//
// Method: nine devices; the walk must send eight no-ops and report the depth.
func TestReceiveFilterWalkRefusesAStackPastTheBound(t *testing.T) {
	stack := newStack(vlanChain(receiveFilterDepthMax+1, &netlink.Device{LinkAttrs: stackAttrs(1, 0, -1)})...)
	err := stack.walk(t, receiveFilterDepthMax+1)
	if err == nil {
		t.Fatalf("the walk answered no error for a stack of %d devices (no-ops sent to %v)", receiveFilterDepthMax+1, stack.sent)
	}
	if !strings.Contains(err.Error(), "deeper than 8") {
		t.Fatalf("the error does not report the depth: %v", err)
	}
	if want := []int{9, 8, 7, 6, 5, 4, 3, 2}; !slices.Equal(stack.sent, want) {
		t.Fatalf("no-ops sent to %v, want %v", stack.sent, want)
	}
}

// TestReceiveFilterWalkStopsAtADeviceItSynced checks that two devices linked
// to each other end the walk after both are synced once.
//
// Method: a macvlan at 3 on a device at 2 that links back to 3. A walk with no
// visited-device stop syncs them in turn until its bound and errors.
func TestReceiveFilterWalkStopsAtADeviceItSynced(t *testing.T) {
	stack := newStack(
		&netlink.Vlan{LinkAttrs: stackAttrs(2, 3, -1)},
		&netlink.Macvlan{LinkAttrs: stackAttrs(3, 2, -1)},
	)
	if err := stack.walk(t, 3); err != nil {
		t.Fatalf("walk: %v", err)
	}
	if want := []int{3, 2}; !slices.Equal(stack.sent, want) {
		t.Fatalf("no-ops sent to %v, want %v", stack.sent, want)
	}
}

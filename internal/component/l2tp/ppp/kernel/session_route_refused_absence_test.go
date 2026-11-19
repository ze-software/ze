// Design: docs/research/l2tpv2-ze-integration.md -- exact kernel PPP route ownership
// Related: session.go -- AddRoute records a refused add, RemoveRoute reads ESRCH

package kernel

import (
	"errors"
	"net/netip"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/rtproto"
)

// routeOwnershipNACK models the native writer's terminal matched NACK
// (nativePPPRejected in the netlink backend): an errno the kernel answered,
// carrying the NativePPPWriteRejected marker.
type routeOwnershipNACK struct{ errno unix.Errno }

func (nack *routeOwnershipNACK) Error() string           { return nack.errno.Error() }
func (nack *routeOwnershipNACK) Unwrap() error           { return nack.errno }
func (*routeOwnershipNACK) NativePPPWriteRejected() bool { return true }

// Goal: a route add the kernel refused, whose withdrawal the kernel answers
// with ESRCH, is a confirmed absence: the withdrawal succeeds and the receipt
// is released, so session cleanup no longer reports an unconfirmed removal.
// Method: the add is NACKed with ENETDOWN, the removal with ESRCH; a second
// withdrawal must perform no native call because no receipt remains.
func TestKernelSessionRouteRefusedAddWithdrawnOnESRCH(t *testing.T) {
	handle, backend := newRouteOwnershipSession(t)
	destination := netip.MustParsePrefix("203.0.113.128/25")
	backend.addErr = &routeOwnershipNACK{errno: unix.ENETDOWN}
	if err := handle.AddRoute(destination, netip.Addr{}, 1, rtproto.Iface); !errors.Is(err, unix.ENETDOWN) {
		t.Fatalf("refused add error = %v, want ENETDOWN", err)
	}
	backend.removeErr = &routeOwnershipNACK{errno: unix.ESRCH}
	if err := handle.RemoveRoute(destination, netip.Addr{}, 1, rtproto.Iface); err != nil {
		t.Fatalf("withdrawal of a refused add answered ESRCH = %v, want a confirmed absence", err)
	}
	if err := handle.RemoveRoute(destination, netip.Addr{}, 1, rtproto.Iface); err != nil {
		t.Fatal(err)
	}
	if len(backend.removes) != 1 {
		t.Fatalf("native withdrawals = %v, want one: the receipt must be released by ESRCH", backend.removes)
	}
}

// Goal: ESRCH stands for absence only after a kernel refusal. An uncertain
// add keeps its strict receipt, and a refused add keeps it on any other errno.
// Method: each case NACKs the removal and must keep the error and the receipt,
// so a retry performs a second native withdrawal.
func TestKernelSessionRouteWithdrawalKeepsReceiptUnlessRefusedAndESRCH(t *testing.T) {
	cases := []struct {
		name      string
		addErr    error
		removeErr error
	}{
		{"uncertain add, ESRCH removal", errors.New("route add outcome unknown"), &routeOwnershipNACK{errno: unix.ESRCH}},
		{"refused add, ESTALE removal", &routeOwnershipNACK{errno: unix.ENETDOWN}, &routeOwnershipNACK{errno: unix.ESTALE}},
		{"refused add, unmarked ESRCH", &routeOwnershipNACK{errno: unix.ENETDOWN}, unix.ESRCH},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handle, backend := newRouteOwnershipSession(t)
			destination := netip.MustParsePrefix("203.0.113.128/25")
			backend.addErr = tc.addErr
			if err := handle.AddRoute(destination, netip.Addr{}, 1, rtproto.Iface); err == nil {
				t.Fatal("failed add reported success")
			}
			backend.removeErr = tc.removeErr
			if err := handle.RemoveRoute(destination, netip.Addr{}, 1, rtproto.Iface); !errors.Is(err, tc.removeErr) {
				t.Fatalf("withdrawal error = %v, want %v kept", err, tc.removeErr)
			}
			backend.removeErr = nil
			if err := handle.RemoveRoute(destination, netip.Addr{}, 1, rtproto.Iface); err != nil {
				t.Fatal(err)
			}
			if len(backend.removes) != 2 {
				t.Fatalf("native withdrawals = %d, want the failed attempt then the retry", len(backend.removes))
			}
		})
	}
}

// Goal: a repeated add of an owned receipt drops an earlier refusal, since the
// repeat may install the path. Method: a refused add, then a successful add of
// the same tuple; an ESRCH removal must then keep its error.
func TestKernelSessionRouteRepeatedAddClearsRefusal(t *testing.T) {
	handle, backend := newRouteOwnershipSession(t)
	destination := netip.MustParsePrefix("203.0.113.128/25")
	backend.addErr = &routeOwnershipNACK{errno: unix.ENETDOWN}
	if err := handle.AddRoute(destination, netip.Addr{}, 1, rtproto.Iface); err == nil {
		t.Fatal("refused add reported success")
	}
	backend.addErr = nil
	if err := handle.AddRoute(destination, netip.Addr{}, 1, rtproto.Iface); err != nil {
		t.Fatal(err)
	}
	removeErr := &routeOwnershipNACK{errno: unix.ESRCH}
	backend.removeErr = removeErr
	if err := handle.RemoveRoute(destination, netip.Addr{}, 1, rtproto.Iface); !errors.Is(err, removeErr) {
		t.Fatalf("withdrawal after a successful repeat = %v, want ESRCH kept", err)
	}
}

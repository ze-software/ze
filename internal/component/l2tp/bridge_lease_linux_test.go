// Design: docs/architecture/l2tp/followup-l2tp-call.md -- relay transport lease at the native bridge
// Related: bridge_linux.go -- maybeBridgePPPoE; kernel_linux.go -- rollbackPPPBridge

//go:build linux

package l2tp

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

type bridgeLeaseForTest struct {
	borrow func(func(int) error) error
}

func (lease *bridgeLeaseForTest) WithDescriptor(use func(int) error) error {
	return lease.borrow(use)
}

// The bridge performs its actual native ioctl inside the owner's borrow.
// An invalid descriptor produces EBADF through that callback; it cannot be
// mistaken for permission to run a local PPP session instead.
func TestMaybeBridgePPPoEBorrowsForNativeIoctl(t *testing.T) {
	borrows := 0
	lease := &bridgeLeaseForTest{borrow: func(use func(int) error) error {
		borrows++
		err := use(-1)
		require.ErrorIs(t, err, unix.EBADF, "the callback must execute the native bridge path")
		return err
	}}
	bridged, err := maybeBridgePPPoE(kernelSetupEvent{channel: lease}, -1)
	require.True(t, bridged)
	require.ErrorIs(t, err, unix.EBADF)
	require.Equal(t, 1, borrows)
}

// A departed transport refuses the borrow before the kernel can consume an
// old descriptor number. The worker must retain that failure as relay failure.
func TestMaybeBridgePPPoERefusesDepartedLease(t *testing.T) {
	want := errors.New("subscriber transport closed")
	borrows := 0
	lease := &bridgeLeaseForTest{borrow: func(func(int) error) error {
		borrows++
		return want
	}}
	bridged, err := maybeBridgePPPoE(kernelSetupEvent{channel: lease}, -1)
	require.True(t, bridged)
	require.ErrorIs(t, err, want)
	require.Equal(t, 1, borrows)
}

// Ordinary calls and LNS sessions do not borrow a PPPoE transport. Nil is an
// explicit non-relay state, while an LNS's separate PPP setup remains selected.
func TestMaybeBridgePPPoELeavesNonRelayCallsLocal(t *testing.T) {
	lease := &bridgeLeaseForTest{borrow: func(func(int) error) error {
		t.Fatal("LNS session borrowed a PPPoE transport")
		return errors.New("unexpected borrow")
	}}
	for _, event := range []kernelSetupEvent{{}, {lnsMode: true, channel: lease}} {
		bridged, err := maybeBridgePPPoE(event, -1)
		require.NoError(t, err)
		require.False(t, bridged)
	}
}

// A refused bridge must close every unpublished descriptor. Verify real open
// descriptors become EBADF, rather than merely counting fake cleanup calls.
func TestRollbackPPPBridgeClosesOwnedDescriptors(t *testing.T) {
	pair, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	require.NoError(t, err)
	third, err := unix.Dup(pair[0])
	if err != nil {
		require.NoError(t, errors.Join(err, unix.Close(pair[0]), unix.Close(pair[1])))
	}
	for _, fd := range []int{pair[0], pair[1], third} {
		var original unix.Stat_t
		require.NoError(t, unix.Fstat(fd, &original))
		t.Cleanup(func() {
			var current unix.Stat_t
			err := unix.Fstat(fd, &current)
			if errors.Is(err, unix.EBADF) {
				return
			}
			require.NoError(t, err)
			// A successful rollback may already have made this number available
			// to another goroutine. Cleanup closes only the original socket.
			if current.Dev != original.Dev {
				return
			}
			if current.Ino != original.Ino {
				return
			}
			require.NoError(t, unix.Close(fd))
		})
	}
	want := errors.New("bridge borrow refused")
	err = rollbackPPPBridge([3]int{pair[0], pair[1], third}, want, unix.Close)
	require.ErrorIs(t, err, want)
	for _, fd := range []int{pair[0], pair[1], third} {
		_, inspectErr := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0)
		require.ErrorIs(t, inspectErr, unix.EBADF, "unpublished descriptor %d leaked", fd)
	}
}

// FD zero is owned, aliases close once, and a failed close cannot prevent
// cleanup of another descriptor or erase the bridge's original error.
func TestRollbackPPPBridgeRetainsAllErrorsAndClosesAliasesOnce(t *testing.T) {
	want := errors.New("bridge refused")
	closeErr := errors.New("close failed")
	otherCloseErr := errors.New("another close failed")
	var closed []int
	err := rollbackPPPBridge([3]int{0, 7, 0}, want, func(fd int) error {
		closed = append(closed, fd)
		if fd == 0 {
			return closeErr
		}
		return otherCloseErr
	})
	require.Equal(t, []int{0, 7}, closed)
	require.ErrorIs(t, err, want)
	require.ErrorIs(t, err, closeErr)
	require.ErrorIs(t, err, otherCloseErr)
}

// An unallocated negative slot must not close anything; zero remains an owned
// slot even when both neighbouring resources were never allocated.
func TestRollbackPPPBridgeSkipsUnallocatedDescriptors(t *testing.T) {
	want := errors.New("bridge refused")
	var closed []int
	err := rollbackPPPBridge([3]int{-1, 0, -1}, want, func(fd int) error {
		closed = append(closed, fd)
		return nil
	})
	require.Equal(t, []int{0}, closed)
	require.ErrorIs(t, err, want)
}

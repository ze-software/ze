// Design: docs/architecture/l2tp/followup-l2tp-call.md -- relay transport lease at the native bridge
// Related: bridge_linux.go -- maybeBridgePPPoE; kernel_ppp_owners_linux.go -- original-owner cleanup; kernel_ppp_model_linux_test.go -- logical FD model

//go:build linux

package l2tp

import (
	"errors"
	"os"
	"testing"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/core/callsink"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

type bridgeLeaseForTest struct {
	borrow   func(func(int) error) error
	original func(func(*os.File) error) error
}

func (lease *bridgeLeaseForTest) WithOriginalFile(use func(*os.File) error) error {
	return lease.original(use)
}
func (lease *bridgeLeaseForTest) RetainOriginalFile() (callsink.RelayFileLease, error) {
	return lease, nil
}
func (*bridgeLeaseForTest) Close() error { return nil }

func (lease *bridgeLeaseForTest) WithDescriptor(use func(int) error) error {
	return lease.borrow(use)
}

// This private capture-failure model preserves the exact callback error and
// original file identity. The integration test with the original name covers
// the production capture and actual native refusal within the same borrow.
func TestMaybeBridgePPPoECaptureFailureModel(t *testing.T) {
	original, peer, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, errors.Join(original.Close(), peer.Close())) })
	borrows := 0
	lease := &bridgeLeaseForTest{original: func(use func(*os.File) error) error {
		borrows++
		err := use(original)
		require.ErrorIs(t, err, unix.EBADF, "the callback must execute original-file capture")
		return err
	}}
	bridged, err := maybeBridgePPPoEWith(kernelSetupEvent{channel: lease}, &kernelPPPOwners{},
		func(actual *os.File) (*pppkernel.CreatedChannel, error) {
			require.Same(t, original, actual)
			return nil, unix.EBADF
		},
		func(*pppkernel.CreatedChannel, *pppkernel.CreatedChannel) (kernelPPPBridge, error) {
			t.Fatal("capture failure reached constructor")
			return nil, errors.New("capture failure reached constructor")
		})
	require.True(t, bridged)
	require.ErrorIs(t, err, unix.EBADF)
	require.Equal(t, 1, borrows)
}

// A departed transport refuses the borrow before the kernel can consume an
// old descriptor number. The worker must retain that failure as relay failure.
func TestMaybeBridgePPPoERefusesDepartedLease(t *testing.T) {
	want := errors.New("subscriber transport closed")
	borrows := 0
	lease := &bridgeLeaseForTest{original: func(func(*os.File) error) error {
		borrows++
		return want
	}}
	bridged, err := maybeBridgePPPoE(kernelSetupEvent{channel: lease}, &kernelPPPOwners{})
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
		bridged, err := maybeBridgePPPoE(event, nil)
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
	failures := closeLogicalPPPForTest(unix.Close)(nil, nil, [3]int{pair[0], pair[1], third})
	err = errors.Join(want, failures[0], failures[1], failures[2])
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
	failures := closeLogicalPPPForTest(func(fd int) error {
		closed = append(closed, fd)
		if fd == 0 {
			return closeErr
		}
		return otherCloseErr
	})(nil, nil, [3]int{0, 7, 0})
	err := errors.Join(want, failures[0], failures[1], failures[2])
	require.Equal(t, []int{0, 7}, closed)
	require.ErrorIs(t, err, want)
	require.ErrorIs(t, err, closeErr)
	require.ErrorIs(t, err, otherCloseErr)
}

// An unallocated negative slot must not close anything; zero remains an owned
// slot even when both neighboring resources were never allocated.
func TestRollbackPPPBridgeSkipsUnallocatedDescriptors(t *testing.T) {
	want := errors.New("bridge refused")
	var closed []int
	failures := closeLogicalPPPForTest(func(fd int) error {
		closed = append(closed, fd)
		return nil
	})(nil, nil, [3]int{-1, 0, -1})
	err := errors.Join(want, failures[0], failures[1], failures[2])
	require.Equal(t, []int{0}, closed)
	require.ErrorIs(t, err, want)
}

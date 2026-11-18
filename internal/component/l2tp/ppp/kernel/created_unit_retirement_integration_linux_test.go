//go:build integration && linux

// Design: docs/research/l2tpv2-ze-integration.md -- actual original unit retirement
// Related: created_unit_integration_linux_test.go -- actual creation producer
// Related: session_created_integration_linux_test.go -- actual typed session

package kernel_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/test/userns"
)

// The test deliberately pauses a short borrow. During actual owner Close the
// retained fd still answers PPPIOCGUNIT, while the passive join must not certify
// retirement. Releasing and joining that borrow permits physical unregister.
func TestCreatedUnitNativeRetirementJoinsOriginalBorrow(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _, link := createdUnitNative(t, "zret-borrow")
	entered := make(chan struct{})
	probe := make(chan struct{})
	answer := make(chan error, 1)
	release := make(chan struct{})
	borrowDone := make(chan error, 1)
	closeDone := make(chan error, 1)
	var releaseOnce, probeOnce sync.Once
	unblock := func() { probeOnce.Do(func() { close(probe) }); releaseOnce.Do(func() { close(release) }) }
	closeStarted := false
	go func() {
		borrowDone <- unit.WithDescriptor(func(fd int) error {
			close(entered)
			<-probe
			number, err := unix.IoctlGetInt(fd, 0x80047456)
			if err == nil {
				if number != unit.UnitNumber() {
					err = fmt.Errorf("original borrowed unit=%d, want %d", number, unit.UnitNumber())
				}
			}
			answer <- err
			<-release
			return err
		})
	}()
	defer func() {
		unblock()
		select {
		case err := <-borrowDone:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("original borrower did not join")
		}
		if closeStarted {
			select {
			case err := <-closeDone:
				if err != nil {
					t.Error(err)
				}
			case <-time.After(3 * time.Second):
				t.Error("original Close did not join")
			}
		}
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("original borrow did not enter")
	}
	closeStarted = true
	go func() { closeDone <- unit.Close() }()
	require.Eventually(t, func() bool {
		return errors.Is(unit.WithDescriptor(func(int) error { return nil }), os.ErrClosed)
	}, 3*time.Second, time.Millisecond, "Close must stop new original borrows")
	probeOnce.Do(func() { close(probe) })
	select {
	case err := <-answer:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("held original did not answer its native unit query")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, unit.WaitNativePPPUnitRetired(ctx), context.Canceled)
	releaseOnce.Do(func() { close(release) })
	// Let deferred cleanup join both worker lifetimes; this wait joins the same
	// physical owner without consuming either worker's retained result.
	wait, stop := context.WithTimeout(t.Context(), 3*time.Second)
	defer stop()
	require.NoError(t, unit.WaitNativePPPUnitRetired(wait))
	_, err := netlink.LinkByIndex(link.Attrs().Index)
	var absent netlink.LinkNotFoundError
	if !errors.As(err, &absent) {
		t.Fatalf("actual original was not unregistered after borrow and Close: %v", err)
	}
}

type retirementNativeSeal struct{ cause error }

func (*retirementNativeSeal) NativeBirthTag() string        { return "ze-retirement/native-original" }
func (seal *retirementNativeSeal) SealNativeRelease() error { return seal.cause }

// The actual public session binds its real native-release duplicate. Closing
// the original streams must not certify retirement while that duplicate keeps
// the kernel unit alive. Confirmed seal release permits the original number to
// be reused; repeated old cleanup must leave that useful successor untouched.
func TestCreatedUnitNativeRetirementJoinsRetainedOriginalDuplicate(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _, link := createdUnitNative(t, "zret-original")
	channel := createdSessionChannel(t, "zret-access", 87)
	cfg, err := unit.SessionConfig(channel)
	require.NoError(t, err)
	owner := createdSessionOpen(t, cfg)
	release, ok := owner.(ppp.NativeReleaseHandle)
	require.True(t, ok)
	cause := errors.New("original policy still has pending native work")
	seal := &retirementNativeSeal{cause: cause}
	require.NoError(t, release.BindNativeReleaseSeal(seal))
	require.ErrorIs(t, owner.Close(), cause)
	require.True(t, release.NativeReleasePending())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, unit.WaitNativePPPUnitRetired(ctx), context.Canceled)
	stillOriginal, err := netlink.LinkByIndex(link.Attrs().Index)
	require.NoError(t, err)
	require.Equal(t, link.Attrs().Index, stillOriginal.Attrs().Index)
	survivor, _, _ := createdUnitNative(t, "zret-survivor")
	if survivor.UnitNumber() == unit.UnitNumber() {
		t.Fatal("unconfirmed duplicate allowed original unit-number reuse")
	}
	createdUnitZero(t, survivor)
	seal.cause = nil
	require.NoError(t, owner.Close())
	require.False(t, release.NativeReleasePending())
	wait, stop := context.WithTimeout(t.Context(), 3*time.Second)
	defer stop()
	require.NoError(t, unit.WaitNativePPPUnitRetired(wait))
	_, err = netlink.LinkByIndex(link.Attrs().Index)
	var absent netlink.LinkNotFoundError
	if !errors.As(err, &absent) {
		t.Fatalf("confirmed duplicate release did not unregister original: %v", err)
	}
	successor, _, _ := createdUnitNative(t, "zret-original")
	require.Equal(t, unit.UnitNumber(), successor.UnitNumber())
	for range 2 {
		require.NoError(t, owner.Close())
		require.NoError(t, unit.WaitNativePPPUnitRetired(wait))
		createdUnitZero(t, successor)
		createdUnitZero(t, survivor)
	}
}

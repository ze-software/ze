// Design: docs/features/interfaces.md -- actual original-unit transport retirement
// Related: created_unit_retirement_integration_linux_test.go -- physical join
// Integration questions; current runtime acceptance remains owed.

//go:build integration && linux && ze_l2tp

package ifacenetlink

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/internal/test/userns"
)

func nativePPPRetirementOriginal(t *testing.T) (*netlinkBackend, *pppkernel.CreatedUnit, *nativePPPWriter) {
	t.Helper()
	backend := &netlinkBackend{}
	// Registered first: the exact owner and socket are closed before backend
	// cleanup, including on a failed assertion in a deliberately pending state.
	t.Cleanup(func() { _ = backend.Close() })
	creator, err := pppkernel.NewUnitCreator(backend)
	require.NoError(t, err)
	unit, err := creator.Create("")
	if unit != nil {
		t.Cleanup(func() { require.NoError(t, unit.Close()) })
	}
	require.NoError(t, err)
	require.NotNil(t, unit)
	identity := unit.Identity()
	require.Positive(t, identity.NativeIndex)
	require.NoError(t, unit.WithDescriptor(func(fd int) error {
		number, err := unix.IoctlGetInt(fd, 0x80047456)
		if err != nil {
			return err
		}
		require.Equal(t, unit.UnitNumber(), number)
		return nil
	}))
	bound, err := backend.BindNativePPPUnit(unit, int(identity.NativeIndex), int32(unit.UnitNumber()))
	require.NoError(t, err, "actual creator must pass the provider's own query51")
	writer, ok := bound.(*nativePPPWriter)
	require.True(t, ok)
	require.True(t, writer.ClosePending(), "actual unused socket still owes Close")
	t.Cleanup(func() {
		// Only this task's exact owner is closed; no lookup grants cleanup.
		_ = unit.Close()
		_ = writer.Close() // A terminal unknown-outcome history is expected.
	})
	return backend, unit, writer
}

// The actual useful mutation reaches the native kernel before the read is
// delayed. Normal ACK processing must complete with a live unit (no passive
// retirement wait), and must also complete after actual original-file Close.
func TestNativePPPProviderDelayedACKDoesNotHoldOriginal(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	for _, retireBeforeACK := range []bool{false, true} {
		t.Run(map[bool]string{false: "live original", true: "closed original"}[retireBeforeACK], func(t *testing.T) {
			_, unit, writer := nativePPPRetirementOriginal(t)
			entered, release := make(chan struct{}), make(chan struct{})
			done := make(chan error, 1)
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			writer.beforeRead = func() { close(entered); <-release }
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			var closeDone chan error
			closeJoined := false
			go func() { done <- writer.SetMTU(ctx, 1431) }()
			defer func() {
				unblock()
				select {
				case err := <-done:
					if err != nil {
						t.Errorf("delayed original ACK worker failed: %v", err)
					}
				case <-time.After(3 * time.Second):
					t.Error("delayed original ACK worker did not join")
				}
				if closeDone != nil && !closeJoined {
					select {
					case err := <-closeDone:
						if err != nil {
							t.Errorf("delayed original Close worker failed: %v", err)
						}
					case <-time.After(3 * time.Second):
						t.Error("delayed original Close worker did not join")
					}
				}
			}()
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("useful original send did not reach read boundary")
			}
			nativePPPProviderRequireLink(t, int(unit.Identity().NativeIndex), 1431, "", false)
			if retireBeforeACK {
				closeDone = make(chan error, 1)
				go func() { closeDone <- unit.Close() }()
				select {
				case err := <-closeDone:
					closeJoined = true
					require.NoError(t, err)
				case <-ctx.Done():
					t.Fatal("ACK wait retained the original unit borrow")
				}
				require.NoError(t, unit.WaitNativePPPUnitRetired(ctx))
			}
			unblock()
		})
	}
}

func TestNativePPPProviderLiveSettleReadsOriginalACK(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	backend, unit, writer := nativePPPRetirementOriginal(t)
	initial, cancel := context.WithCancel(t.Context())
	writer.beforeRead = cancel
	require.ErrorIs(t, writer.SetMTU(initial, 1434), context.Canceled)
	require.NotNil(t, writer.pending, "actual send must precede the cancelled wait")
	writer.beforeRead = nil
	wait, stop := context.WithTimeout(t.Context(), 3*time.Second)
	defer stop()
	err := writer.Settle(wait)
	require.ErrorIs(t, err, context.Canceled, "original wait history must survive a complete ACK")
	require.NotErrorIs(t, err, iface.ErrNativePPPPending)
	require.Nil(t, writer.pending, "passive retirement observation must not block a live original ACK")
	require.False(t, writer.closed)
	require.True(t, writer.ClosePending())
	require.NoError(t, unit.WithDescriptor(func(fd int) error { _, err := unix.IoctlGetInt(fd, 0x80047456); return err }))
	nativePPPProviderRequireLink(t, int(unit.Identity().NativeIndex), 1434, "", false)
	err = writer.Close()
	require.ErrorIs(t, err, context.Canceled, "final Close must retain completed settlement history")
	require.NotErrorIs(t, err, iface.ErrNativePPPPending)
	require.False(t, writer.ClosePending())
	_, socketErr := unix.GetsockoptInt(writer.socket, unix.SOL_SOCKET, unix.SO_TYPE)
	require.ErrorIs(t, socketErr, unix.EBADF, "the original socket must be physically released")
	err = backend.Close()
	require.ErrorIs(t, err, context.Canceled, "backend Close must retain completed settlement history")
	require.NotErrorIs(t, err, iface.ErrNativePPPPending)
	require.True(t, backend.counters.closed, "terminal history must permit final backend cleanup")
}

// The exact original borrowed descriptor still answers PPPIOCGUNIT after
// logical Close. Its passive join must keep the pending socket and backend
// alive until that borrower and actual Close finish, then permit final cleanup.
func TestNativePPPProviderRetirementJoinsHeldOriginal(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	backend, unit, writer := nativePPPRetirementOriginal(t)
	ctx, cancel := context.WithCancel(t.Context())
	writer.beforeRead = cancel
	require.ErrorIs(t, writer.SetMTU(ctx, 1432), context.Canceled)
	require.NotNil(t, writer.pending, "actual send must be pending before retirement")
	writer.beforeRead = nil
	entered, probe, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	answer, borrowDone, closeDone := make(chan error, 1), make(chan error, 1), make(chan error, 1)
	var probeOnce, releaseOnce sync.Once
	unblock := func() { probeOnce.Do(func() { close(probe) }); releaseOnce.Do(func() { close(release) }) }
	go func() {
		borrowDone <- unit.WithDescriptor(func(fd int) error {
			close(entered)
			<-probe
			number, err := unix.IoctlGetInt(fd, 0x80047456)
			if err == nil && number != unit.UnitNumber() {
				err = errors.New("held original answered another unit")
			}
			answer <- err
			<-release
			return err
		})
	}()
	closeStarted := false
	defer func() {
		unblock()
		select {
		case err := <-borrowDone:
			if err != nil {
				t.Errorf("held original borrower failed: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("held original borrower did not join")
		}
		if closeStarted {
			select {
			case err := <-closeDone:
				if err != nil {
					t.Errorf("held original Close worker failed: %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Error("held original Close did not join")
			}
		}
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("held original borrow did not enter")
	}
	closeStarted = true
	go func() { closeDone <- unit.Close() }()
	require.Eventually(t, func() bool { return errors.Is(unit.WithDescriptor(func(int) error { return nil }), os.ErrClosed) }, 3*time.Second, time.Millisecond)
	// Permit the query while continuing to hold the borrow. Separate release
	// channels keep this assertion independent from final borrower retirement.
	probeOnce.Do(func() { close(probe) })
	select {
	case err := <-answer:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("held original native query did not answer")
	}
	for _, call := range []func() error{func() error { return writer.Settle(ctx) }, writer.Close, backend.Close} {
		require.ErrorIs(t, call(), iface.ErrNativePPPPending)
	}
	require.False(t, backend.counters.closed)
	require.False(t, writer.closed)
	require.True(t, writer.ClosePending())
	// probe is already closed; only release remains owned here.
	releaseOnce.Do(func() { close(release) })
	wait, stop := context.WithTimeout(t.Context(), 3*time.Second)
	defer stop()
	require.NoError(t, unit.WaitNativePPPUnitRetired(wait))
	err := writer.Settle(ctx) // Already-cancelled caller still observes completed join.
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
	require.NotErrorIs(t, err, iface.ErrNativePPPPending)
	require.Nil(t, writer.pending)
	require.True(t, writer.closed)
	require.False(t, writer.ClosePending())
	raw, rawErr := writer.file.SyscallConn()
	require.NoError(t, rawErr)
	require.NotNil(t, raw)
	controlEntered := false
	controlErr := raw.Control(func(uintptr) { controlEntered = true })
	require.Error(t, controlErr)
	require.False(t, controlEntered, "retired original socket Control callback must not run")
	_, err = writer.file.Stat()
	require.ErrorIs(t, err, os.ErrClosed)
	require.ErrorIs(t, backend.Close(), context.Canceled)
	require.True(t, backend.counters.closed)
}

type nativePPPRetirementSeal struct{ cause error }

func (*nativePPPRetirementSeal) NativeBirthTag() string        { return "ze:test:provider-retirement" }
func (seal *nativePPPRetirementSeal) SealNativeRelease() error { return seal.cause }

// A real public session creates the library-managed retained duplicate. Pending
// policy keeps that duplicate alive after stream Close, so neither writer nor
// backend may retire. Confirmed seal release closes the SAME tracked hold.
func TestNativePPPProviderRetirementJoinsLibraryDuplicate(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	backend, unit, writer := nativePPPRetirementOriginal(t)
	channel := nativePPPRetirementChannel(t)
	cfg, err := unit.SessionConfig(channel)
	require.NoError(t, err)
	frames, session, err := pppkernel.NewSession(cfg)
	require.NoError(t, err)
	require.NotNil(t, frames)
	require.NotNil(t, session)
	seal := &nativePPPRetirementSeal{cause: errors.New("task policy has pending native cleanup")}
	release, ok := session.(ppp.NativeReleaseHandle)
	require.True(t, ok)
	require.NoError(t, release.BindNativeReleaseSeal(seal))
	t.Cleanup(func() { seal.cause = nil; _ = session.Close() })
	ctx, cancel := context.WithCancel(t.Context())
	writer.beforeRead = cancel
	require.ErrorIs(t, writer.SetMTU(ctx, 1433), context.Canceled)
	require.NotNil(t, writer.pending)
	writer.beforeRead = nil
	require.ErrorIs(t, session.Close(), seal.cause)
	require.True(t, release.NativeReleasePending())
	require.ErrorIs(t, unit.WaitNativePPPUnitRetired(ctx), context.Canceled)
	for _, call := range []func() error{func() error { return writer.Settle(ctx) }, writer.Close, backend.Close} {
		require.ErrorIs(t, call(), iface.ErrNativePPPPending)
	}
	require.False(t, writer.closed)
	require.True(t, writer.ClosePending())
	require.False(t, backend.counters.closed)
	seal.cause = nil
	require.NoError(t, session.Close())
	require.False(t, release.NativeReleasePending())
	wait, stop := context.WithTimeout(t.Context(), 3*time.Second)
	defer stop()
	require.NoError(t, unit.WaitNativePPPUnitRetired(wait))
	err = writer.Settle(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.NotErrorIs(t, err, iface.ErrNativePPPPending)
	var rejected interface{ NativePPPWriteRejected() bool }
	require.False(t, errors.As(err, &rejected))
	require.True(t, writer.closed)
	require.False(t, writer.ClosePending())
	require.ErrorIs(t, backend.Close(), context.Canceled)
	require.True(t, backend.counters.closed)
}

func TestNativePPPProviderSettledRouteNACKIsNotUnavailable(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	_, _, writer := nativePPPRetirementOriginal(t)
	err := writer.RemoveRoute(t.Context(), netip.MustParsePrefix("192.0.2.0/24"), netip.MustParseAddr("198.51.100.1"), 19, rtproto.Iface)
	require.Error(t, err)
	var rejected interface{ NativePPPWriteRejected() bool }
	require.True(t, errors.As(err, &rejected), "actual complete guarded route NACK must retain its classification")
	require.NotErrorIs(t, err, iface.ErrNativePPPUnavailable)
	require.NotErrorIs(t, err, iface.ErrNativePPPPending)
	require.Nil(t, writer.pending)
	require.True(t, writer.ClosePending(), "settled error does not close the retained socket")
	require.NoError(t, writer.Close())
	require.False(t, writer.ClosePending())
}

func nativePPPRetirementChannel(t *testing.T) *pppkernel.CreatedChannel {
	t.Helper()
	link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "zpretaccess"}}
	require.NoError(t, netlink.LinkAdd(link))
	t.Cleanup(func() { require.NoError(t, netlink.LinkDel(link)) })
	require.NoError(t, netlink.LinkSetUp(link))
	fd, err := unix.Socket(unix.AF_PPPOX, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	require.NoError(t, err)
	original := os.NewFile(uintptr(fd), "provider-retirement-original-pppox")
	t.Cleanup(func() { require.NoError(t, original.Close()) })
	require.NoError(t, unix.Connect(fd, &unix.SockaddrPPPoE{SID: 97, Remote: []byte{2, 0, 0, 0, 0, 1}, Dev: link.Name}))
	channel, err := pppkernel.CreatePPPChannel(original)
	if channel != nil {
		t.Cleanup(func() { require.NoError(t, channel.Close()) })
	}
	require.NoError(t, err)
	require.NotNil(t, channel)
	return channel
}

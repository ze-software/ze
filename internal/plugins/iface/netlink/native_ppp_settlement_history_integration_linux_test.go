// Design: docs/features/interfaces.md -- matched settlement history after release
// Related: native_ppp_integration_linux_test.go -- actual NEWUNIT/query51 fixture
// Related: native_ppp_settlement_history_model_linux_test.go -- physical Close model

//go:build integration && linux

package ifacenetlink

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/internal/test/userns"
)

// Goal: an actual matched ACK retains its prior wait cause through final Close.
// Method: cancel the first read only after actual Sendto, settle the queued kernel
// ACK, then require completed physical release and historical cleanup errors.
func TestNativePPPProviderSettledACKHistorySurvivesClose(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	nativePPPSettlementHistory(t, false)
}

// Goal: an actual matched NACK and prior wait cause survive final Close.
// Method: cancel the first read after submitting an absent-route deletion,
// settle its queued original kernel NACK, then complete writer/backend cleanup.
func TestNativePPPProviderSettledNACKHistorySurvivesClose(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	nativePPPSettlementHistory(t, true)
}

func nativePPPSettlementHistory(t *testing.T, negative bool) {
	t.Helper()
	// Each root owns its child. The dedicated proof requires actual NEWUNIT,
	// query51 and guarded native writes rather than skipping their absence.
	required := *nativePPPProviderRequired
	*nativePPPProviderRequired = true
	t.Cleanup(func() { *nativePPPProviderRequired = required })
	fixture := nativePPPProviderFixtureNew(t)
	backend := &netlinkBackend{}
	bound, err := backend.BindNativePPPUnit(fixture, fixture.link.Attrs().Index, fixture.unit)
	if err != nil {
		t.Fatalf("bind actual original PPP writer: %v", err)
	}
	writer, ok := bound.(*nativePPPWriter)
	if !ok {
		t.Fatal("actual provider returned another writer implementation")
	}
	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	if err := writer.SetMTU(ctx, 1439); err != nil {
		t.Fatalf("useful native MTU control before uncertain read: %v", err)
	}
	nativePPPProviderRequireLink(t, fixture.link.Attrs().Index, 1439, "", false)
	if _, err := backend.counters.snapshot(""); err != nil {
		t.Fatalf("open useful independent counter reader: %v", err)
	}
	completed := backend.closed.channel()
	initial, cancel := context.WithCancel(context.Background())
	defer cancel()
	writer.beforeRead = cancel
	mtu := 1441
	if negative {
		err = writer.RemoveRoute(initial, netip.MustParsePrefix("192.0.2.0/24"),
			netip.MustParseAddr("198.51.100.1"), 19, rtproto.Iface)
		mtu = 1439
	} else {
		err = writer.SetMTU(initial, mtu)
	}
	writer.beforeRead = nil
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled original wait lost its cause: %v", err)
	}
	if !errors.Is(err, iface.ErrNativePPPPending) {
		t.Fatalf("canceled read did not retain original ACK custody: %v", err)
	}
	if writer.pending == nil {
		t.Fatal("canceled read discarded its actually submitted original transaction")
	}
	if !writer.pending.entered {
		t.Fatal("uncertain original transaction did not reach native Sendto")
	}
	sequence := writer.sequence
	settleErr := writer.Settle(ctx)
	if !errors.Is(settleErr, context.Canceled) {
		t.Fatalf("matched settlement lost the original canceled wait: %v", settleErr)
	}
	if errors.Is(settleErr, iface.ErrNativePPPPending) {
		t.Fatalf("matched original response retained unresolved transport: %v", settleErr)
	}
	var rejection interface{ NativePPPWriteRejected() bool }
	rejected := errors.As(settleErr, &rejection)
	if rejected != negative {
		t.Fatalf("actual matched response rejection=%v, want %v: %v", rejected, negative, settleErr)
	}
	if negative {
		if !errors.Is(settleErr, unix.ESRCH) {
			t.Fatalf("actual absent-route NACK lost native ESRCH: %v", settleErr)
		}
	}
	if writer.pending != nil {
		t.Fatal("matched original response left its transaction pending")
	}
	if writer.sequence != sequence {
		t.Fatal("settlement resubmitted its original mutation")
	}
	nativePPPProviderRequireLink(t, fixture.link.Attrs().Index, mtu, "", false)
	if !writer.ClosePending() {
		t.Fatal("matched response incorrectly released its still-open original socket")
	}
	closeErr := writer.Close()
	if _, err := unix.GetsockoptInt(writer.socket, unix.SOL_SOCKET, unix.SO_TYPE); !errors.Is(err, unix.EBADF) {
		t.Errorf("final Close did not physically release its actual original socket: %v", err)
	}
	if writer.ClosePending() {
		t.Error("final physical Close retained transport custody because of history")
	}
	if !errors.Is(closeErr, context.Canceled) {
		t.Errorf("final physical Close discarded completed settlement wait history: %v", closeErr)
	}
	for _, cleanup := range []func() error{
		writer.Close,
		func() error { return writer.Settle(ctx) },
		backend.Close,
		backend.Close,
	} {
		cleanupErr := cleanup()
		if !errors.Is(cleanupErr, context.Canceled) {
			t.Errorf("terminal cleanup lost the original wait cause: %v", cleanupErr)
		}
		if negative {
			if !errors.Is(cleanupErr, unix.ESRCH) {
				t.Errorf("terminal cleanup lost original matched native NACK: %v", cleanupErr)
			}
		}
		if errors.Is(cleanupErr, iface.ErrNativePPPPending) {
			t.Errorf("terminal history became unresolved native custody: %v", cleanupErr)
		}
	}
	if len(backend.nativePPPs.writers) != 0 {
		t.Error("final physical Close retained its writer registry entry")
	}
	if !backend.counters.closed {
		t.Error("terminal settlement history prevented independent counter cleanup")
	}
	select {
	case <-completed:
	default:
		t.Error("terminal settlement history prevented backend completion")
	}
}

//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- actual CPE fault matrix
// Related: native_cpe_fault_fixture_integration_linux_test.go -- same native producers

package ifacenetlink

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestNativeCPEPartialBindTerminalCleanup is the useful opposite to failed-close
// custody. The real writer/socket/probe is returned with a producer refusal;
// original unit/channel, writer and backend really close before the next actual
// Dial preflight. Terminal error history must not become permanent custody.
func TestNativeCPEPartialBindTerminalCleanup(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	h := startCPENativeFault(t, cpePartialBindTerminal, false)
	cpeFaultWait(t, h.ctx, h.backend.bound, "actual partial bind missing")
	var first cpeFaultDialResult
	select {
	case first = <-h.dialer.first:
	case <-h.ctx.Done():
		t.Fatal("actual partial-bind Dial did not return")
	}
	if !errors.Is(first.err, h.backend.bindCause) {
		t.Fatalf("partial producer cause lost: %v", first.err)
	}
	if first.session.Cleanup != nil {
		t.Fatal("completed actual cleanup retained a callback")
	}
	if h.backend.sends.Load() != 0 {
		t.Fatal("refused bind submitted a native mutation")
	}
	if h.backend.mutations.Load() != 0 {
		t.Fatal("iface mutated a refused bind result")
	}
	cpeFaultWait(t, h.ctx, h.backend.backendDone, "actual terminal backend close missing")
	if h.backend.facet.ClosePending() {
		t.Fatal("terminal native socket remains owned")
	}
	if err := h.backend.unit.WaitNativePPPUnitRetired(h.ctx); err != nil {
		t.Fatalf("partial original unit not retired: %v", err)
	}
	cpeFaultWait(t, h.ctx, h.admitted, "terminal actual cleanup never allowed next preflight")
}

// TestNativeCPEPartialBindRetainsFailedSocketClose combines a real partial bind
// with a kernel-denied close of its SAME original netlink socket. Actual partial
// Cleanup must survive Dial's error and every retry. No fake pending marker or
// os.File.Stat classification substitutes for the first actual syscall result.
func TestNativeCPEPartialBindRetainsFailedSocketClose(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	h := startCPENativeFault(t, cpePartialBindSocketClose, true)
	cpeFaultWait(t, h.ctx, h.backend.bound, "actual partial bind missing")
	var first cpeFaultDialResult
	select {
	case first = <-h.dialer.first:
	case <-h.ctx.Done():
		t.Fatal("actual partial result missing")
	}
	if !errors.Is(first.err, h.backend.bindCause) {
		t.Fatalf("bind cause lost: %v", first.err)
	}
	if !errors.Is(first.err, unix.EPERM) {
		t.Fatalf("actual socket-close cause lost: %v", first.err)
	}
	if !errors.Is(first.err, iface.ErrNativePPPPending) {
		t.Fatalf("actual failed close became terminal: %v", first.err)
	}
	if first.session.Cleanup == nil {
		t.Fatal("Dial forgot real partial cleanup custody")
	}
	if first.session.NativeWriter != h.backend.facet {
		t.Fatal("partial result replaced its exact returned writer")
	}
	if !h.backend.unit.NativeOperationsPending() {
		t.Fatal("shared original lost its partial facet")
	}
	if _, err := unix.Getsockname(h.backend.writer.socket); err != nil {
		t.Fatalf("genuine denied-close socket is not live: %v", err)
	}
	if h.backend.sends.Load() != 0 {
		t.Fatal("partial bind submitted a mutation")
	}
	if h.backend.mutations.Load() != 0 {
		t.Fatal("iface mutated a partial error result")
	}
	h.releaseCleanup()
	for _, result := range []<-chan error{h.dialer.firstCleanup, h.dialer.secondCleanup} {
		err := cpeFaultWaitResult(t, h, result)
		if !errors.Is(err, iface.ErrNativePPPPending) {
			t.Fatalf("retry lost actual custody: %v", err)
		}
		if !errors.Is(err, unix.EPERM) {
			t.Fatalf("retry lost actual close error: %v", err)
		}
		if !errors.Is(err, h.backend.bindCause) {
			t.Fatalf("retry lost producer refusal: %v", err)
		}
	}
	if h.factoryCalls.Load() != 1 {
		t.Fatal("pending partial cleanup admitted another Dial")
	}
	if h.backend.backendCloses.Load() != 0 {
		t.Fatal("pending partial socket released its original backend")
	}
}

// TestNativeCPEProbeRefusalRetainsFailedSocketClose refuses the actual kernel
// owner query before any write facet exists, then denies actual socket Close.
// Root's afterSocket observer supplies no result; the genuine producer must
// return refused partial cleanup custody through the real CPE Dial error.
func TestNativeCPEProbeRefusalRetainsFailedSocketClose(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	h := startCPENativeFault(t, cpeProbeSocketClose, true)
	cpeFaultWait(t, h.ctx, h.backend.bound, "actual query refusal never returned")
	if h.backend.probeFaultErr != nil {
		t.Fatalf("actual syscall controls were not installed: %v", h.backend.probeFaultErr)
	}
	var first cpeFaultDialResult
	select {
	case first = <-h.dialer.first:
	case <-h.ctx.Done():
		t.Fatal("actual refused partial Dial did not return")
	}
	if !errors.Is(first.err, unix.EPERM) {
		t.Fatalf("genuine query/Close cause lost: %v", first.err)
	}
	if !errors.Is(first.err, iface.ErrNativePPPPending) {
		t.Fatalf("failed pre-probe Close dropped custody: %v", first.err)
	}
	if h.backend.writer != nil {
		t.Fatal("refused probe acquired a positive native producer")
	}
	if h.backend.facet == nil {
		t.Fatal("producer lost real pre-probe cleanup facet")
	}
	if first.session.NativeWriter != h.backend.facet {
		t.Fatal("CPE replaced the exact refused partial facet")
	}
	if first.session.Cleanup == nil {
		t.Fatal("actual Dial forgot refused partial cleanup")
	}
	if !h.backend.facet.ClosePending() {
		t.Fatal("genuine failed socket Close reported terminal custody")
	}
	if !h.backend.unit.NativeOperationsPending() {
		t.Fatal("original unit dropped the refused partial facet")
	}
	if _, err := unix.Getsockname(h.backend.socketLabel); err != nil {
		t.Fatalf("denied-close original socket is not live: %v", err)
	}
	if h.backend.mutations.Load() != 0 {
		t.Fatal("CPE mutated a refused query result")
	}
	h.releaseCleanup()
	for _, result := range []<-chan error{h.dialer.firstCleanup, h.dialer.secondCleanup} {
		err := cpeFaultWaitResult(t, h, result)
		if !errors.Is(err, unix.EPERM) {
			t.Fatalf("retry erased actual failed Close: %v", err)
		}
		if !errors.Is(err, iface.ErrNativePPPPending) {
			t.Fatalf("retry forgot actual refused custody: %v", err)
		}
	}
	if err := h.backend.unit.WaitNativePPPUnitRetired(h.ctx); err != nil {
		t.Fatalf("partial socket wait prevented genuine unit retirement: %v", err)
	}
	if h.backend.backendCloses.Load() != 0 {
		t.Fatal("refused live socket released its original backend")
	}
	if h.factoryCalls.Load() != 1 {
		t.Fatal("refused live socket admitted another actual Dial")
	}
}

// TestNativeCPEUncertainACKSettlesOriginalWithoutResend cancels only the real
// receive context after actual SEND, renames that original and installs a
// same-name dummy, then lets the real iface worker report Pending. Its retained
// callback is held before physical cleanup while a fresh Settle receives the
// genuine queued ACK on the original socket. Physical unit retirement must then
// precede writer/backend closure and next actual Dial admission.
func TestNativeCPEUncertainACKSettlesOriginalWithoutResend(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	h := startCPENativeFault(t, cpeUncertainACK, false)
	cpeFaultWait(t, h.ctx, h.backend.ackEntered, "actual SEND/receive boundary missing")
	index := int(h.backend.unit.Identity().NativeIndex)
	original, err := netlink.LinkByIndex(index)
	if err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetName(original, "heldfault"); err != nil {
		t.Fatal(err)
	}
	replacement := &netlink.Dummy{Name: "faultwan", MTU: 9000}
	if err := netlink.LinkAdd(replacement); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := netlink.LinkDel(replacement); err != nil {
			t.Error(err)
		}
	})
	h.releaseACK()
	err = cpeFaultWaitResult(t, h, h.backend.writeResult)
	if !errors.Is(err, iface.ErrNativePPPPending) {
		t.Fatalf("actual canceled receive was not Pending: %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("actual receive cancellation lost: %v", err)
	}
	cpeFaultWait(t, h.ctx, h.dialer.cleanupEntered, "real consumer forgot retained cleanup")
	if h.backend.mutations.Load() != 1 {
		t.Fatalf("consumer mutated after uncertainty: %d", h.backend.mutations.Load())
	}
	if h.dialer.applied.Load() != 0 {
		t.Fatal("consumer reported Applied after uncertainty")
	}
	if h.factoryCalls.Load() != 1 {
		t.Fatal("new Dial entered before original cleanup")
	}
	probe, cancel := context.WithCancel(context.Background())
	cancel()
	if err := h.backend.unit.WaitNativePPPUnitRetired(probe); err == nil {
		t.Fatal("live original fabricated retirement")
	}
	settleCtx, settleCancel := context.WithTimeout(h.ctx, 5*time.Second)
	err = h.backend.facet.Settle(settleCtx)
	settleCancel()
	if errors.Is(err, iface.ErrNativePPPPending) {
		t.Fatalf("genuine queued ACK did not settle original: %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("terminal ACK erased original wait history: %v", err)
	}
	h.settledHistory = context.Canceled
	h.backend.writer.mu.Lock()
	pending, sequence := h.backend.writer.pending != nil, h.backend.writer.sequence
	h.backend.writer.mu.Unlock()
	if pending {
		t.Fatal("settled original still has a transaction")
	}
	if sequence != 1 {
		t.Fatalf("settlement reconstructed a request sequence: %d", sequence)
	}
	if h.backend.sends.Load() != 1 {
		t.Fatalf("original request resent: %d", h.backend.sends.Load())
	}
	if !h.backend.facet.ClosePending() {
		t.Fatal("settlement pretended the live socket was closed")
	}
	h.releaseCleanup()
	cpeFaultRequireHistory(t, cpeFaultWaitResult(t, h, h.dialer.firstCleanup), context.Canceled, "settled actual original cleanup")
	cpeFaultWait(t, h.ctx, h.backend.backendDone, "original backend closure missing")
	if err := h.backend.unit.WaitNativePPPUnitRetired(h.ctx); err != nil {
		t.Fatalf("actual original not physically retired: %v", err)
	}
	if h.backend.facet.ClosePending() {
		t.Fatal("terminal original socket still owned")
	}
	if h.backend.mutations.Load() != 1 {
		t.Fatal("cleanup submitted another mutation")
	}
	untouched, err := netlink.LinkByName("faultwan")
	if err != nil {
		t.Fatal(err)
	}
	if untouched.Attrs().Index == index {
		t.Fatal("replacement became the original")
	}
	if untouched.Attrs().MTU != 9000 {
		t.Fatal("uncertain consumer changed replacement MTU")
	}
	addresses, err := netlink.AddrList(untouched, netlink.FAMILY_ALL)
	if err != nil {
		t.Fatal(err)
	}
	if len(addresses) != 0 {
		t.Fatalf("replacement acquired original address effects: %v", addresses)
	}
	cpeFaultWait(t, h.ctx, h.admitted, "terminal original never permitted actual readmission")
}

// TestNativeCPEChangedResultUncertaintyNeverAcknowledgesSequence preserves the
// initial uncertainty control above and separately drives actual renegotiation.
// The real producer's complete Changes record and Seq reach the real consumer
// unchanged. A real submitted address request then loses its receive context;
// that specific result MUST NOT be Applied or permit a later native mutation.
func TestNativeCPEChangedResultUncertaintyNeverAcknowledgesSequence(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	h := startCPENativeFault(t, cpeChangedACK, false)
	cpeFaultWaitInstalled(t, h)
	index := int(h.backend.unit.Identity().NativeIndex)
	original, err := netlink.LinkByIndex(index)
	if err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetName(original, "heldchange"); err != nil {
		t.Fatal(err)
	}
	replacement := &netlink.Dummy{Name: "faultwan", MTU: 9000}
	if err := netlink.LinkAdd(replacement); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := netlink.LinkDel(replacement); err != nil {
			t.Error(err)
		}
	})
	close(h.restart)
	var change iface.PPPoEResult
	select {
	case change = <-h.dialer.changes:
	case <-h.ctx.Done():
		t.Fatal("actual renegotiated Changes record missing")
	}
	if change.Seq == 0 {
		t.Fatal("real change has no producer sequence")
	}
	if change.LocalIP.String() != "198.51.100.2" {
		t.Fatalf("real renegotiation did not carry new address: %+v", change)
	}
	cpeFaultWait(t, h.ctx, h.backend.ackEntered, "changed-result actual SEND missing")
	mutations, sends := h.backend.mutations.Load(), h.backend.sends.Load()
	if mutations <= 4 {
		t.Fatal("uncertainty did not occur after useful initial setup")
	}
	h.releaseACK()
	err = cpeFaultWaitResult(t, h, h.backend.writeResult)
	if !errors.Is(err, iface.ErrNativePPPPending) {
		t.Fatalf("real changed-result receive lost uncertainty: %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("real changed-result receive cause lost: %v", err)
	}
	cpeFaultWait(t, h.ctx, h.dialer.cleanupEntered, "changed result failed to retain actual cleanup")
	if h.dialer.appliedSeq.Load() == change.Seq {
		t.Fatalf("uncertain actual Seq %d was Applied", change.Seq)
	}
	if h.dialer.applied.Load() != 0 {
		t.Fatal("uncertain real change reported an Applied acknowledgment")
	}
	if h.backend.mutations.Load() != mutations {
		t.Fatal("consumer attempted a later mutation after uncertain change")
	}
	settle, cancel := context.WithTimeout(h.ctx, 5*time.Second)
	err = h.backend.facet.Settle(settle)
	cancel()
	if errors.Is(err, iface.ErrNativePPPPending) {
		t.Fatalf("genuine changed-result queued ACK did not settle: %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("settled real change erased receive history: %v", err)
	}
	h.settledHistory = context.Canceled
	if h.backend.sends.Load() != sends {
		t.Fatal("changed native request was resent")
	}
	h.releaseCleanup()
	cpeFaultRequireHistory(t, cpeFaultWaitResult(t, h, h.dialer.firstCleanup), context.Canceled, "settled changed original cleanup")
	cpeFaultWait(t, h.ctx, h.backend.backendDone, "changed original backend never completed")
	if h.dialer.appliedSeq.Load() == change.Seq {
		t.Fatal("cleanup acknowledged the uncertain result")
	}
	untouched, err := netlink.LinkByName("faultwan")
	if err != nil {
		t.Fatal(err)
	}
	if untouched.Attrs().Index == index {
		t.Fatal("same-name replacement became original")
	}
	if untouched.Attrs().MTU != 9000 {
		t.Fatal("real changed result modified replacement MTU")
	}
	addresses, err := netlink.AddrList(untouched, netlink.FAMILY_ALL)
	if err != nil {
		t.Fatal(err)
	}
	if len(addresses) != 0 {
		t.Fatalf("replacement acquired actual changed address: %v", addresses)
	}
	cpeFaultWait(t, h.ctx, h.admitted, "terminal changed original blocked real readmission")
}

// TestNativeCPEFailedOriginalUnitCloseBlocksReadmission denies the actual close
// syscall only for the genuine original unit file after real full negotiation
// and useful address/default-route installation. Peer PADT drives the actual
// cleanup/worker retry, not an invented failure return or pending boolean.
func TestNativeCPEFailedOriginalUnitCloseBlocksReadmission(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	h := startCPENativeFault(t, cpeUnitClose, true)
	peer := cpeFaultWaitInstalled(t, h)
	if h.backend.mutations.Load() != 4 {
		t.Fatalf("useful initial native operations = %d", h.backend.mutations.Load())
	}
	if err := peer.SendPADT(); err != nil {
		t.Fatal(err)
	}
	cpeFaultWait(t, h.ctx, h.dialer.cleanupEntered, "PADT did not reach actual cleanup")
	h.releaseCleanup()
	for _, result := range []<-chan error{h.dialer.firstCleanup, h.dialer.secondCleanup} {
		err := cpeFaultWaitResult(t, h, result)
		if !errors.Is(err, unix.EPERM) {
			t.Fatalf("actual unit-close syscall failure lost: %v", err)
		}
		if !errors.Is(err, iface.ErrNativePPPPending) {
			t.Fatalf("failed original unit granted terminal cleanup: %v", err)
		}
	}
	if err := h.backend.unit.WaitNativePPPUnitRetired(h.ctx); !errors.Is(err, unix.EPERM) {
		t.Fatalf("failed physical original close granted retirement: %v", err)
	}
	if !h.backend.unit.NativeOperationsPending() {
		t.Fatal("failed original close dropped actual writer")
	}
	if h.backend.writerCloses.Load() != 0 {
		t.Fatal("writer close preceded original physical retirement")
	}
	if h.backend.backendCloses.Load() != 0 {
		t.Fatal("backend close preceded original retirement")
	}
	if h.factoryCalls.Load() != 1 {
		t.Fatal("failed original admitted another actual Dial")
	}
}

// TestNativeCPEFailedOriginalSocketCloseBlocksReadmission is the actual socket
// close opposite: the unit has genuinely retired, but EPERM leaves this same
// original netlink socket live. Completed Go wrapper Close/history cannot be
// treated as physical release or permission to drop the backend/cleanup slot.
func TestNativeCPEFailedOriginalSocketCloseBlocksReadmission(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	h := startCPENativeFault(t, cpeSocketClose, true)
	peer := cpeFaultWaitInstalled(t, h)
	if err := peer.SendPADT(); err != nil {
		t.Fatal(err)
	}
	cpeFaultWait(t, h.ctx, h.dialer.cleanupEntered, "PADT did not reach actual cleanup")
	h.releaseCleanup()
	for _, result := range []<-chan error{h.dialer.firstCleanup, h.dialer.secondCleanup} {
		err := cpeFaultWaitResult(t, h, result)
		if !errors.Is(err, unix.EPERM) {
			t.Fatalf("actual socket-close failure lost: %v", err)
		}
		if !errors.Is(err, iface.ErrNativePPPPending) {
			t.Fatalf("live original socket became terminal history: %v", err)
		}
	}
	if err := h.backend.unit.WaitNativePPPUnitRetired(h.ctx); err != nil {
		t.Fatalf("unit failed to advance before socket wait: %v", err)
	}
	if _, err := unix.Getsockname(h.backend.writer.socket); err != nil {
		t.Fatalf("genuine failed-close original socket no longer live: %v", err)
	}
	if !h.backend.facet.ClosePending() {
		t.Fatal("actual live socket reported no custody")
	}
	if h.backend.backendCloses.Load() != 0 {
		t.Fatal("actual failed socket close released original backend")
	}
	if h.factoryCalls.Load() != 1 {
		t.Fatal("actual failed socket close admitted another Dial")
	}
}

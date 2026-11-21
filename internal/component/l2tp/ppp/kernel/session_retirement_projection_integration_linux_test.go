//go:build integration && linux

// Design: docs/research/l2tpv2-ze-integration.md -- actual server release projection
// Related: created_unit_retirement_integration_linux_test.go -- same original unit ledger

package kernel_test

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/test/pppcpefault"
	"github.com/ze-software/ze/internal/test/userns"
)

// Deny the first actual original unit close in an isolated child. Direct
// kernel observations prove it remains live after both the real seal duplicate
// and writer socket close; only then ask the real SessionHandle's projection.
func TestSessionNativeReleaseProjectionFailedOriginalClose(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	h := newNativeRetirementProjection(t, "zrp-denied")
	var before unix.Stat_t
	if err := unix.Fstat(h.original, &before); err != nil {
		t.Fatal(err)
	}
	if err := pppcpefault.DenyOriginalClose(h.original); err != nil {
		t.Fatal(err)
	}
	h.closeCause = unix.EPERM
	if err := h.owner.Close(); !errors.Is(err, unix.EPERM) {
		t.Fatalf("actual original close did not retain EPERM: %v", err)
	}
	var after unix.Stat_t
	if err := unix.Fstat(h.original, &after); err != nil || before.Dev != after.Dev || before.Ino != after.Ino {
		t.Fatalf("denied original no longer identifies the same live file: before=%+v after=%+v err=%v", before, after, err)
	}
	number, err := unix.IoctlGetInt(h.original, 0x80047456) // PPPIOCGUNIT, no mutation or new ownership.
	if err != nil || number != h.unit.UnitNumber() {
		t.Fatalf("denied original no longer answers its real unit query: number=%d err=%v", number, err)
	}
	if _, err := netlink.LinkByIndex(int(h.unit.Identity().NativeIndex)); err != nil {
		t.Fatalf("denied original unit is not physically live: %v", err)
	}
	h.requireTerminalOtherOwners(t)
	if err := h.unit.WaitNativePPPUnitRetired(t.Context()); !errors.Is(err, unix.EPERM) {
		t.Fatalf("exact retirement owner lost the real Close refusal: %v", err)
	}
	if !h.release.NativeReleasePending() {
		t.Fatal("physically live original unit reported confirmed release after writer and seal duplicate closed")
	}
}

// The inverse uses the same genuine producer and release path without a
// denying filter. Confirmed original retirement must not retain the session.
func TestSessionNativeReleaseProjectionConfirmedOriginalClose(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	h := newNativeRetirementProjection(t, "zrp-confirmed")
	if err := h.owner.Close(); err != nil {
		t.Fatal(err)
	}
	h.requireTerminalOtherOwners(t)
	if err := h.unit.WaitNativePPPUnitRetired(t.Context()); err != nil {
		t.Fatalf("successful original close has no retirement receipt: %v", err)
	}
	var after unix.Stat_t
	if err := unix.Fstat(h.original, &after); !errors.Is(err, unix.EBADF) {
		t.Fatalf("successful original close left its descriptor live: %v", err)
	}
	_, err := netlink.LinkByIndex(int(h.unit.Identity().NativeIndex))
	if _, missing := errors.AsType[netlink.LinkNotFoundError](err); !missing {
		t.Fatalf("confirmed original unit remained registered: %v", err)
	}
	if h.release.NativeReleasePending() {
		t.Fatal("confirmed native original still reports pending release")
	}
}

// The same genuine session must keep its local writer while an independent
// remote seal deliberately retains the actual unit duplicate, then close both
// without retaining a synthetic timeout once the original seal confirms.
func TestSessionNativeReleaseProjectionPendingSealDefersLocalSettlement(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	h := newNativeRetirementProjection(t, "zrp-seal")
	cause := errors.New("original remote seal still has pending work")
	h.seal.cause = cause
	t.Cleanup(func() { h.seal.cause = nil })
	err := h.owner.Close()
	if !errors.Is(err, cause) {
		t.Fatalf("original seal cause missing: %v", err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("independent seal manufactured a local retirement timeout: %v", err)
	}
	if !h.unit.NativeOperationsPending() || !h.release.NativeReleasePending() {
		t.Fatal("local writer was consumed before the genuine remote seal released its duplicate")
	}
	var duplicate unix.Stat_t
	if err := unix.Fstat(h.duplicate, &duplicate); err != nil {
		t.Fatalf("pending genuine seal did not retain its actual unit duplicate: %v", err)
	}
	h.seal.cause = nil
	if err := h.owner.Close(); err != nil {
		t.Fatalf("confirmed genuine seal retained an artificial local failure: %v", err)
	}
	if err := h.unit.WaitNativePPPUnitRetired(t.Context()); err != nil {
		t.Fatal(err)
	}
	if h.unit.NativeOperationsPending() || h.release.NativeReleasePending() || h.writer.ClosePending() {
		t.Fatal("confirmed genuine seal did not finish exact original cleanup")
	}
}

type nativeRetirementProjectionSeal struct {
	calls int
	cause error
}

func (*nativeRetirementProjectionSeal) NativeBirthTag() string {
	return "ze-retirement/server-projection"
}
func (seal *nativeRetirementProjectionSeal) SealNativeRelease() error {
	seal.calls++
	return seal.cause
}

type nativeRetirementProjection struct {
	unit       *pppkernel.CreatedUnit
	owner      ppp.SessionHandle
	release    ppp.NativeReleaseHandle
	writer     pppkernel.NativeUnitOperations
	seal       *nativeRetirementProjectionSeal
	original   int
	duplicate  int
	closeCause error
}

func newNativeRetirementProjection(t *testing.T, name string) *nativeRetirementProjection {
	t.Helper()
	backend, err := iface.OpenBackend("netlink")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	creator, err := pppkernel.NewUnitCreator(backend)
	if err != nil {
		t.Fatal(err)
	}
	h := &nativeRetirementProjection{original: -1, duplicate: -1}
	h.unit, err = creator.Create(name)
	if h.unit != nil {
		t.Cleanup(func() {
			if err := h.unit.Close(); !errors.Is(err, h.closeCause) {
				t.Errorf("original unit cleanup changed its retained outcome: %v", err)
			}
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := h.unit.WithDescriptor(func(fd int) error { h.original = fd; return nil }); err != nil {
		t.Fatal(err)
	}
	provider, ok := backend.(iface.NativePPPBackend)
	if !ok {
		t.Fatal("actual original backend has no native PPP writer")
	}
	h.writer, err = h.unit.BindNativeOperations(t.Context(), backend, func(owner *pppkernel.CreatedUnit, index int, number int32) (pppkernel.NativeUnitOperations, error) {
		return provider.BindNativePPPUnit(owner, index, number)
	})
	if err != nil {
		t.Fatalf("genuine original writer did not bind: %v", err)
	}
	channel := createdSessionChannel(t, name+"-a", 83)
	cfg, err := h.unit.SessionConfig(channel)
	if err != nil {
		t.Fatal(err)
	}
	_, h.owner, err = pppkernel.NewSession(cfg)
	if h.owner != nil {
		t.Cleanup(func() {
			if err := h.owner.Close(); !errors.Is(err, h.closeCause) {
				t.Errorf("original session cleanup changed its retained outcome: %v", err)
			}
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	h.release, ok = h.owner.(ppp.NativeReleaseHandle)
	if !ok {
		t.Fatal("genuine session has no native release projection")
	}
	before := nativeRetirementUnitDescriptors(t, h.unit.UnitNumber())
	h.seal = &nativeRetirementProjectionSeal{}
	if err := h.release.BindNativeReleaseSeal(h.seal); err != nil {
		t.Fatal(err)
	}
	for fd := range nativeRetirementUnitDescriptors(t, h.unit.UnitNumber()) {
		if _, existed := before[fd]; !existed {
			if h.duplicate != -1 {
				t.Fatal("release seal created more than its one original-unit duplicate")
			}
			h.duplicate = fd
		}
	}
	if h.duplicate < 0 || h.duplicate == h.original {
		t.Fatal("release seal did not create a distinct actual original-unit duplicate")
	}
	return h
}

func (h *nativeRetirementProjection) requireTerminalOtherOwners(t *testing.T) {
	t.Helper()
	if h.seal.calls != 1 {
		t.Fatalf("remote seal was not confirmed exactly once: %d", h.seal.calls)
	}
	var duplicate unix.Stat_t
	if err := unix.Fstat(h.duplicate, &duplicate); !errors.Is(err, unix.EBADF) {
		t.Fatalf("actual retained duplicate did not physically close: %v", err)
	}
	if h.writer.ClosePending() || h.unit.NativeOperationsPending() {
		t.Fatal("writer socket retained custody; this question must isolate physical unit retirement")
	}
}

// Descriptor labels here are read-only test observations, never resource grants
// or cleanup inputs. The unit query identifies the one actual seal duplicate.
func nativeRetirementUnitDescriptors(t *testing.T, unit int) map[int]struct{} {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	matching := make(map[int]struct{})
	for _, entry := range entries {
		fd, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		actual, err := unix.IoctlGetInt(fd, 0x80047456)
		if err == nil && actual == unit {
			matching[fd] = struct{}{}
		}
	}
	return matching
}

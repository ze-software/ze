//go:build integration && linux

// Design: docs/architecture/appliance/kernel-profiles.md -- exact original bridge.
// Related: session_created_integration_linux_test.go -- genuine producer fixtures.

package kernel_test

import (
	"errors"
	"os"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/test/userns"
)

// Both channels come from real atomic transport capture. The public bridge
// keeps exact native files through retirement and performs joined, idempotent
// cleanup without requiring a live-only query after transport shutdown.
func TestOriginalPairNativeBridgeRetainsRetiredPeers(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	first := createdSessionChannel(t, "zbridge-left", 87)
	second := createdSessionChannel(t, "zbridge-right", 88)
	bridge, err := pppkernel.NewOriginalChannelBridge(first, second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := bridge.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := bridge.Bridge(); err != nil {
		t.Fatal(err)
	}
	if err := bridge.Bridge(); !errors.Is(err, unix.EALREADY) {
		t.Fatalf("duplicate bridge: %v", err)
	}
	for range 2 {
		if err := bridge.Unbridge(); err != nil {
			t.Fatalf("joined original unbridge: %v", err)
		}
	}
	if err := bridge.Bridge(); err != nil {
		t.Fatalf("useful rebridge: %v", err)
	}
	access, err := netlink.LinkByName("zbridge-left")
	if err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetDown(access); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := bridge.Unbridge(); err != nil {
			t.Fatalf("retired original unbridge: %v", err)
		}
	}
	if err := bridge.Bridge(); !errors.Is(err, unix.ENOTCONN) {
		t.Fatalf("retired channel re-admitted: %v", err)
	}
}

// A pre-snapshotted session and another bridge cannot replay an admitted
// bridge original. Each fresh sibling is still consumed on refusal, while
// both admitted originals survive and remain useful for bridge/unbridge.
func TestOriginalPairNativeBridgeReplayPreservesAdmittedOriginals(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	first := createdSessionChannel(t, "zbridge-one", 89)
	second := createdSessionChannel(t, "zbridge-two", 90)
	unit, _, _ := createdUnitNative(t, "zbridge-unit")
	cfg, err := unit.SessionConfig(first)
	if err != nil {
		t.Fatal(err)
	}
	bridge, err := pppkernel.NewOriginalChannelBridge(first, second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := bridge.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := bridge.Bridge(); err != nil {
		t.Fatal(err)
	}
	client, err := pppkernel.NewClientSession(cfg)
	if client != nil {
		t.Fatal("session replay admitted a bridge original")
	}
	if err == nil {
		t.Fatal("session replay accepted the consumed channel")
	}
	if _, err := unit.Counters(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("replay retained newly supplied unit: %v", err)
	}
	for slot := range 2 {
		name := "zbridge-newa"
		if slot == 1 {
			name = "zbridge-newb"
		}
		fresh := createdSessionChannel(t, name, uint16(91+slot))
		var replay *pppkernel.OriginalChannelBridge
		if slot == 0 {
			replay, err = pppkernel.NewOriginalChannelBridge(first, fresh)
		}
		if slot == 1 {
			replay, err = pppkernel.NewOriginalChannelBridge(fresh, second)
		}
		if replay != nil {
			t.Fatal("bridge replay returned an owner")
		}
		if err == nil {
			t.Fatal("bridge replay accepted a consumed channel")
		}
		if err := fresh.WithDescriptor(func(int) error { return nil }); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("replay retained new sibling: %v", err)
		}
		if err := first.WithDescriptor(func(int) error { return nil }); err != nil {
			t.Fatalf("replay closed first admitted original: %v", err)
		}
		if err := second.WithDescriptor(func(int) error { return nil }); err != nil {
			t.Fatalf("replay closed second admitted original: %v", err)
		}
		if err := bridge.Unbridge(); err != nil {
			t.Fatalf("replay lost original cleanup authority: %v", err)
		}
		if err := bridge.Bridge(); err != nil {
			t.Fatalf("replay lost useful original bridge: %v", err)
		}
	}
	if err := bridge.Unbridge(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := bridge.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if bridge.ClosePending() {
		t.Fatal("completed bridge close retained native cleanup")
	}
	if err := first.WithDescriptor(func(int) error { return nil }); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("bridge close retained first original: %v", err)
	}
	if err := second.WithDescriptor(func(int) error { return nil }); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("bridge close retained second original: %v", err)
	}
}

// An uncertain exact Unbridge must retain BOTH files for retry. A deliberate
// different native peer relationship refuses Close without retiring originals;
// settling that relationship then permits useful original cleanup and close.
func TestOriginalPairNativeBridgeClosePreservesUncertainOriginals(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	first := createdSessionChannel(t, "zbridge-holda", 93)
	second := createdSessionChannel(t, "zbridge-holdb", 94)
	third := createdSessionChannel(t, "zbridge-holdc", 95)
	bridge, err := pppkernel.NewOriginalChannelBridge(first, second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := bridge.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := bridge.Bridge(); err != nil {
		t.Fatal(err)
	}
	if err := bridge.Unbridge(); err != nil {
		t.Fatal(err)
	}
	operation := func(command uintptr) error {
		return second.WithDescriptor(func(secondFD int) error {
			return third.WithDescriptor(func(thirdFD int) error {
				return nativeChannelOperate(secondFD, command, thirdFD)
			})
		})
	}
	if err := operation(nativeChannelBridge); err != nil {
		t.Fatal(err)
	}
	if err := bridge.Close(); !errors.Is(err, unix.ESTALE) {
		t.Fatalf("uncertain close did not retain exact native refusal: %v", err)
	}
	if !bridge.ClosePending() {
		t.Fatal("uncertain bridge close dropped pending native cleanup")
	}
	if err := first.WithDescriptor(func(int) error { return nil }); err != nil {
		t.Fatalf("uncertain close retired first original: %v", err)
	}
	if err := second.WithDescriptor(func(int) error { return nil }); err != nil {
		t.Fatalf("uncertain close retired second original: %v", err)
	}
	if err := operation(nativeChannelUnbridge); err != nil {
		t.Fatal(err)
	}
	if err := bridge.Close(); err != nil {
		t.Fatalf("restored exact close: %v", err)
	}
	if bridge.ClosePending() {
		t.Fatal("restored bridge close remained pending")
	}
	if err := third.WithDescriptor(func(int) error { return nil }); err != nil {
		t.Fatalf("exact close harmed other peer: %v", err)
	}
}

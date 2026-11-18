//go:build integration && linux

// Design: docs/research/l2tpv2-ze-integration.md -- actual delayed client custody.
// Related: client_created_integration_linux_test.go -- original native producer fixtures.

package kernel_test

import (
	"errors"
	"os"
	"testing"

	"golang.org/x/sys/unix"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/test/userns"
)

// An alias closing the actual original unit must invalidate attachment while
// the original channel remains live and unattached; no replacement is consulted.
func TestClientPairNativeExternalUnitCloseRefusesAttachment(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _, _ := createdUnitNative(t, "zclient-unit")
	channel := createdSessionChannel(t, "zclient-access", 84)
	client := nativeClientOpen(t, unit, channel)
	if err := unit.Close(); err != nil {
		t.Fatal(err)
	}
	if err := client.Connect(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("external unit close lost: %v", err)
	}
	if err := channel.WithDescriptor(func(int) error { return nil }); err != nil {
		t.Fatalf("unit refusal closed live channel: %v", err)
	}
	if err := nativeClientDisconnect(channel); !errors.Is(err, unix.EINVAL) {
		t.Fatalf("closed unit acquired attachment: %v", err)
	}
}

// The opposite lane has independent custody: closing the actual channel must
// refuse attachment while retaining the exact unit through final cleanup.
func TestClientPairNativeExternalChannelCloseRetainsUnit(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _, _ := createdUnitNative(t, "zclient-unit")
	channel := createdSessionChannel(t, "zclient-access", 85)
	client := nativeClientOpen(t, unit, channel)
	if err := channel.Close(); err != nil {
		t.Fatal(err)
	}
	if err := client.Connect(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("external channel close lost: %v", err)
	}
	if _, err := unit.Counters(); err != nil {
		t.Fatalf("channel refusal released original unit: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := unit.Counters(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("final cleanup retained original unit: %v", err)
	}
}

// A replayed native unit transfers no second unit ownership. Its fresh real
// channel sibling is still closed, and the first actual client remains usable.
func TestClientPairNativeReplayedUnitClosesFreshChannel(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _, _ := createdUnitNative(t, "zclient-unit")
	original := createdSessionChannel(t, "zclient-access", 86)
	cfg, err := unit.SessionConfig(original)
	if err != nil {
		t.Fatal(err)
	}
	client := nativeClientOpen(t, unit, original)
	fresh := createdSessionChannel(t, "zclient-fresh", 87)
	if err := fresh.WithDescriptor(func(fd int) error { cfg.ChanFD = fd; return nil }); err != nil {
		t.Fatal(err)
	}
	cfg.Channel = fresh
	second, err := pppkernel.NewClientSession(cfg)
	if second != nil {
		t.Fatal("replayed unit published a second client")
	}
	if err == nil {
		t.Fatal("replayed unit was admitted")
	}
	if err := fresh.WithDescriptor(func(int) error { return nil }); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("fresh sibling leaked: %v", err)
	}
	if _, err := unit.Counters(); err != nil {
		t.Fatalf("replay damaged original unit: %v", err)
	}
	if err := client.Connect(); err != nil {
		t.Fatalf("replay damaged original pair: %v", err)
	}
	if err := nativeClientDisconnect(original); err != nil {
		t.Fatalf("original pair did not remain useful: %v", err)
	}
}

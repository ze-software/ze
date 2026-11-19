// Design: docs/architecture/appliance/kernel-profiles.md -- original-unit socket admission models
// These are non-native refusal/enum models and create no descriptor authority.

package kernel

import (
	"context"
	"errors"
	"os"
	"testing"
)

// TestCreatedUnitControlSocketKinds checks both useful role constants and the
// invalid numeric space. A kind by itself is never a native ownership grant.
func TestCreatedUnitControlSocketKinds(t *testing.T) {
	if uint32(IPv6ControlSocketICMP6Raw) != 1 {
		t.Fatal("ICMP6 raw kind differs from the reserved ioctl43 ABI")
	}
	if uint32(IPv6ControlSocketUDP6) != 2 {
		t.Fatal("UDP6 kind differs from the reserved ioctl43 ABI")
	}
	for _, kind := range []IPv6ControlSocketKind{IPv6ControlSocketICMP6Raw, IPv6ControlSocketUDP6} {
		if err := validateIPv6ControlSocketKind(kind); err != nil {
			t.Fatalf("valid kind %d refused: %v", kind, err)
		}
	}
	for _, kind := range []IPv6ControlSocketKind{IPv6ControlSocketUnspecified, 3, ^IPv6ControlSocketKind(0)} {
		if err := validateIPv6ControlSocketKind(kind); err == nil {
			t.Fatalf("invalid kind %d was admitted", kind)
		}
	}
}

// TestCreatedUnitControlSocketAdmissionRefuses checks early caller cancellation
// and unavailable creator states without reaching any platform/native helper.
func TestCreatedUnitControlSocketAdmissionRefuses(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	var absent *CreatedUnit
	for _, entry := range []struct {
		name   string
		unit   *CreatedUnit
		ctx    context.Context
		kind   IPv6ControlSocketKind
		wanted error
	}{
		{"nil context", absent, nil, IPv6ControlSocketUDP6, nil},
		{"canceled before nil owner", absent, canceled, IPv6ControlSocketUDP6, context.Canceled},
		{"nil original", absent, context.Background(), IPv6ControlSocketUDP6, nil},
		{"zero original", &CreatedUnit{}, context.Background(), IPv6ControlSocketUDP6, nil},
		{"unconfirmed original", &CreatedUnit{number: 1}, context.Background(), IPv6ControlSocketUDP6, nil},
		{"ready labels without original file", &CreatedUnit{ready: true, number: 1}, context.Background(), IPv6ControlSocketUDP6, nil},
		{"closed original", &CreatedUnit{closed: true, ready: true}, context.Background(), IPv6ControlSocketUDP6, os.ErrClosed},
		{"unspecified kind", &CreatedUnit{}, context.Background(), IPv6ControlSocketUnspecified, nil},
		{"unknown kind", &CreatedUnit{}, context.Background(), 3, nil},
	} {
		t.Run(entry.name, func(t *testing.T) {
			file, err := entry.unit.OpenIPv6ControlSocket(entry.ctx, entry.kind)
			if file != nil {
				if closeErr := file.Close(); closeErr != nil {
					t.Errorf("close unexpected owned result: %v", closeErr)
				}
				t.Fatal("unavailable original state returned a socket")
			}
			if err == nil {
				t.Fatal("unavailable original state was accepted")
			}
			if entry.wanted != nil {
				if !errors.Is(err, entry.wanted) {
					t.Fatalf("original cause %v was lost in %v", entry.wanted, err)
				}
			}
		})
	}
}

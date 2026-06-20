package types

import (
	"bytes"
	"testing"
)

// VALIDATES: RemainingLifetime represents 0 and 65535 and serializes to 2
// big-endian octets; 0 means purge (a RemainingLifetime concern, isis-6) (AC-9).
// PREVENTS: 16-bit range / boundary errors in the LSP lifetime field.
func TestRemainingLifetimeBoundaries(t *testing.T) {
	for _, v := range []uint16{0, 1, 1200, 65535} {
		rl := RemainingLifetime(v)
		if rl.Seconds() != v {
			t.Errorf("RemainingLifetime(%d).Seconds() = %d", v, rl.Seconds())
		}
	}
	if !RemainingLifetime(0).IsPurge() {
		t.Error("RemainingLifetime(0) must report IsPurge (remaining-lifetime-0 purge)")
	}
	if RemainingLifetime(1).IsPurge() {
		t.Error("RemainingLifetime(1) must not be a purge")
	}

	rl := RemainingLifetime(0x04d2) // 1234
	var buf [4]byte
	n := rl.WriteTo(buf[:], 0)
	if n != LifetimeLen {
		t.Fatalf("WriteTo returned %d, want %d", n, LifetimeLen)
	}
	want := []byte{0x04, 0xd2}
	if !bytes.Equal(buf[:n], want) {
		t.Errorf("serialize = %x, want %x", buf[:n], want)
	}
	back, err := RemainingLifetimeFromBytes(buf[:n])
	if err != nil {
		t.Fatalf("RemainingLifetimeFromBytes error: %v", err)
	}
	if back != rl {
		t.Errorf("round-trip = %d, want %d", back.Seconds(), rl.Seconds())
	}
	for _, l := range []int{0, 1, 3} {
		if _, err := RemainingLifetimeFromBytes(make([]byte, l)); err == nil {
			t.Errorf("RemainingLifetimeFromBytes(len=%d) should error", l)
		}
	}
}

// VALIDATES: HoldingTime represents 0 and 65535 and serializes to 2 big-endian
// octets (AC-9).
// PREVENTS: 16-bit range / boundary errors in the Hello hold-time field.
func TestHoldingTimeBoundaries(t *testing.T) {
	for _, v := range []uint16{0, 30, 65535} {
		ht := HoldingTime(v)
		if ht.Seconds() != v {
			t.Errorf("HoldingTime(%d).Seconds() = %d", v, ht.Seconds())
		}
	}
	ht := HoldingTime(30)
	var buf [4]byte
	n := ht.WriteTo(buf[:], 0)
	if n != LifetimeLen {
		t.Fatalf("WriteTo returned %d, want %d", n, LifetimeLen)
	}
	want := []byte{0x00, 0x1e}
	if !bytes.Equal(buf[:n], want) {
		t.Errorf("serialize = %x, want %x", buf[:n], want)
	}
	back, err := HoldingTimeFromBytes(buf[:n])
	if err != nil {
		t.Fatalf("HoldingTimeFromBytes error: %v", err)
	}
	if back != ht {
		t.Errorf("round-trip = %d, want %d", back.Seconds(), ht.Seconds())
	}
	for _, l := range []int{0, 1, 3} {
		if _, err := HoldingTimeFromBytes(make([]byte, l)); err == nil {
			t.Errorf("HoldingTimeFromBytes(len=%d) should error", l)
		}
	}
}

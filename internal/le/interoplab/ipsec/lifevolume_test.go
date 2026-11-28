// Design: docs/architecture/ike/ipsec-13-rekey-wire.md -- volume-triggered Child SA rekey against strongSwan.
// Related: lifevolume.go -- lifeBytesProgress, lifeBytesPingCommand.

package ipsec

import (
	"slices"
	"testing"
)

// VALIDATES: lifeBytesProgress returns the maximum directional byte count among the
// initial SPIs, detects replacement SPIs outside the initial set, and rejects unparseable
// SPI strings.
// PREVENTS: failing to recognize an early volume-triggered SPI rotation.
func TestLifeBytesProgress(t *testing.T) {
	initial := map[uint32]struct{}{
		0x11111111: {},
		0x22222222: {},
	}

	counters := map[saKey]uint64{
		{source: zeIP, target: swanIP, spi: "0x11111111"}: 54000,
		{source: swanIP, target: zeIP, spi: "0x22222222"}: 62000,
	}
	gotMax, rotated, err := lifeBytesProgress(counters, initial)
	if err != nil {
		t.Fatalf("lifeBytesProgress: %v", err)
	}
	if gotMax != 62000 {
		t.Errorf("maxInitialBytes = %d, want 62000", gotMax)
	}
	if rotated {
		t.Error("rotated = true with only initial SPIs present")
	}

	counters[saKey{source: zeIP, target: swanIP, spi: "0x33333333"}] = 0
	gotMax, rotated, err = lifeBytesProgress(counters, initial)
	if err != nil {
		t.Fatalf("lifeBytesProgress(rotated): %v", err)
	}
	if gotMax != 62000 {
		t.Errorf("maxInitialBytes = %d, want 62000", gotMax)
	}
	if !rotated {
		t.Error("rotated = false when replacement SPI 0x33333333 appeared")
	}

	bad := map[saKey]uint64{
		{source: zeIP, target: swanIP, spi: "not-a-spi"}: 100,
	}
	if _, _, err := lifeBytesProgress(bad, initial); err == nil {
		t.Error("expected error for malformed SPI string")
	}
}

// VALIDATES: lifeBytesPingCommand steps down from 10-packet bursts to 1-packet probes
// before approaching the hard byte ceiling.
// PREVENTS: overshooting the 131072-byte hard ceiling during interop traffic generation.
func TestLifeBytesPingCommand(t *testing.T) {
	fast := lifeBytesPingCommand(0)
	if !slices.Contains(fast, lifeBytesFastCount) {
		t.Errorf("fast ping command %v missing burst count %s", fast, lifeBytesFastCount)
	}
	slow := lifeBytesPingCommand(lifeBytesTargetStop - 5000)
	if !slices.Contains(slow, "1") {
		t.Errorf("stepped ping command %v missing single-packet count", slow)
	}
}

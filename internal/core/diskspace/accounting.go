// Design: docs/architecture/doctor-and-health-checks.md -- usable disk accounting.

package diskspace

import (
	"errors"
	"math/bits"
)

var errCapacity = errors.New("diskspace: block size is zero or byte capacity exceeds uint64")

// UsableBlocks converts a kernel block quantity without turning a signed deficit
// into free capacity. Nonpositive quantities mean no usable blocks.
func UsableBlocks[T ~int32 | ~uint32 | ~int64 | ~uint64](blocks T) uint64 {
	if blocks <= 0 {
		return 0
	}
	return uint64(blocks)
}

// Percent returns the whole percentage, rounded down, with no intermediate
// overflow. A part above the total is capped at 100. A zero total returns zero;
// callers that require a measured capacity MUST reject zero totals themselves.
func Percent(part, total uint64) uint64 {
	if total == 0 {
		return 0
	}
	if part >= total {
		return 100
	}
	hi, lo := bits.Mul64(part, 100)
	pct, _ := bits.Div64(hi, lo, total) // part < total ensures the quotient fits.
	return pct
}

// Bytes converts blocks to bytes, refusing an invalid block size or overflow
// rather than reporting wrapped capacity as a successful measurement.
func Bytes(blocks, blockSize uint64) (uint64, error) {
	if blockSize == 0 {
		return 0, errCapacity
	}
	hi, lo := bits.Mul64(blocks, blockSize)
	if hi != 0 {
		return 0, errCapacity
	}
	return lo, nil
}

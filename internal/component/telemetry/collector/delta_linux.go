// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

// safeDelta returns cur-prev, or 0 if the counter wrapped (cur < prev).
// /proc counters are typically 32-bit or 64-bit unsigned; on wrap the
// sample is simply dropped rather than producing a huge spike.
func safeDelta(cur, prev uint64) uint64 {
	if cur < prev {
		return 0
	}
	return cur - prev
}

// safeDeltaF64 returns *cur - *prev as a float64, or 0 if either pointer
// is nil or the counter wrapped.
func safeDeltaF64(cur, prev *float64) float64 {
	if cur == nil || prev == nil {
		return 0
	}
	if *cur < *prev {
		return 0
	}
	return *cur - *prev
}

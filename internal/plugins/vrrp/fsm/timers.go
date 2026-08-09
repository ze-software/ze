// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- VRRP Skew_Time / Master_Down_Interval math
// RFC: rfc/short/rfc9568.md (VRRPv3 Algorithms) and rfc/short/rfc3768.md (VRRPv2 Algorithms)
//
// Timer arithmetic for the VRRP FSM. Unit discipline (spec risk R-2): every
// interval crossing the FSM boundary is an integer MILLISECOND count; every
// computed value is a time.Duration (int64 nanoseconds). Multiplication ALWAYS
// precedes division; division by 256 is the LAST operation. Valid v3 skews are
// sub-millisecond (priority 254 at a 10 ms interval is 78,125 ns), so an
// integer-millisecond representation would truncate them to zero -- the exact
// uvrrpd/holo bug class this file exists to prevent.
package fsm

import "time"

// skewDivisor is the constant 256 denominator shared by both versions'
// Skew_Time formulas. It is a compile-time constant, so division by it can
// never panic.
const skewDivisor = 256

// skewTime returns the VRRP Skew_Time as a time.Duration.
//
// RFC 9568 Algorithms: "Skew_Time = ((256 - Priority) * Active_Adver_Interval) / 256"
// (centiseconds in the RFC; here computed on nanosecond time.Duration).
// RFC 3768 Algorithms: "Skew_Time = (256 - Priority) / 256" seconds,
// interval-independent (a 1-second base scaled by (256-Priority)/256).
//
// Higher priority yields a smaller skew and a faster takeover. Because
// (256 - Priority) is at least 1 for any Priority <= 255, and the base term is
// strictly positive, the result is always > 0 (never truncates to zero).
func skewTime(version, priority uint8, activeAdverIntervalMs int) time.Duration {
	numerator := int64(256 - int(priority))
	if version == 2 {
		// v2: interval-independent, base = 1 second.
		return time.Duration(numerator*int64(time.Second)) / skewDivisor
	}
	// v3: base = the (learned/adopted) advertisement interval.
	return time.Duration(numerator*int64(activeAdverIntervalMs)*int64(time.Millisecond)) / skewDivisor
}

// masterDownInterval returns the VRRP Master_Down_Interval (RFC 9568 calls it
// Active_Down_Interval) as a time.Duration.
//
// RFC 9568 Algorithms: "Active_Down_Interval = (3 * Active_Adver_Interval) + Skew_Time".
// RFC 3768 Algorithms: "Master_Down_Interval = (3 * Advertisement_Interval) + Skew_Time"
// (v2 has no learned interval; activeAdverIntervalMs is pinned to the local
// configured interval).
func masterDownInterval(version, priority uint8, activeAdverIntervalMs int) time.Duration {
	threeIntervals := time.Duration(3 * int64(activeAdverIntervalMs) * int64(time.Millisecond))
	return threeIntervals + skewTime(version, priority, activeAdverIntervalMs)
}

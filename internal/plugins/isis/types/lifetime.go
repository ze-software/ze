// Design: docs/architecture/isis/isis-1-types.md -- RemainingLifetime and HoldingTime (16-bit seconds)

package types

// LifetimeLen is the on-wire width of a 16-bit seconds field (Remaining
// Lifetime in an LSP, Holding Time in a Hello). 2 octets, range 0..65535.
const LifetimeLen = 2

// RemainingLifetime is an LSP's remaining lifetime in seconds (ISO/IEC 10589
// section 7.3). It is a 16-bit unsigned value (0..65535); MaxAge is typically
// 1200 seconds. A value of 0 marks a purge: the LSP is flooded with remaining
// lifetime 0 and deleted after a grace period (the runtime decrement, purge and
// grace handling live in isis-6; this type only models the value and reports
// the purge condition).
type RemainingLifetime uint16

// RemainingLifetimeFromBytes decodes a 2-octet big-endian remaining lifetime.
// A length other than 2 returns ErrWrongLength.
func RemainingLifetimeFromBytes(b []byte) (RemainingLifetime, error) {
	if len(b) != LifetimeLen {
		return 0, ErrWrongLength
	}
	return RemainingLifetime(uint16(b[0])<<8 | uint16(b[1])), nil
}

// Seconds returns the remaining lifetime in seconds.
func (l RemainingLifetime) Seconds() uint16 { return uint16(l) }

// IsPurge reports whether the remaining lifetime is 0, which signals a purge
// (ISO/IEC 10589). This is the purge signal, distinct from SequenceNumber 0
// (which is merely reserved, never a purge).
func (l RemainingLifetime) IsPurge() bool { return l == 0 }

// WriteTo writes the 2 big-endian octets into buf at off; returns LifetimeLen.
// Buffer-first, no allocation.
func (l RemainingLifetime) WriteTo(buf []byte, off int) int {
	buf[off] = byte(l >> 8)
	buf[off+1] = byte(l)
	return LifetimeLen
}

// HoldingTime is the adjacency hold time in seconds advertised in a Hello
// (ISO/IEC 10589 section 8.2). It is a 16-bit unsigned value (0..65535); if no
// Hello is received within this time the adjacency times out. Typically three
// times the Hello interval. The hold-timer behavior itself is owned by the
// adjacency FSM (isis-5); this type only models the advertised value.
type HoldingTime uint16

// HoldingTimeFromBytes decodes a 2-octet big-endian holding time. A length
// other than 2 returns ErrWrongLength.
func HoldingTimeFromBytes(b []byte) (HoldingTime, error) {
	if len(b) != LifetimeLen {
		return 0, ErrWrongLength
	}
	return HoldingTime(uint16(b[0])<<8 | uint16(b[1])), nil
}

// Seconds returns the holding time in seconds.
func (h HoldingTime) Seconds() uint16 { return uint16(h) }

// WriteTo writes the 2 big-endian octets into buf at off; returns LifetimeLen.
// Buffer-first, no allocation.
func (h HoldingTime) WriteTo(buf []byte, off int) int {
	buf[off] = byte(h >> 8)
	buf[off+1] = byte(h)
	return LifetimeLen
}

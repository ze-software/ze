// Design: docs/architecture/ospf/ospf-7-lsdb-flooding.md -- lazy raw-byte LSDB entry.
// RFC 2328 Section 13.1: LSA freshness comparison.

package lsdb

import (
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

const lsaAgeOff = 0

// Freshness is the RFC 2328 Section 13.1 ordering between two instances of the
// same LSA key.
type Freshness int

const (
	Older Freshness = iota - 1
	Equal
	Newer
)

// Entry is one LSDB row. It takes ownership of the raw byte slice passed to
// newEntry; callers must not retain or mutate it afterward.
type Entry struct {
	header      packet.LSAHeader
	raw         []byte
	baseAge     types.LSAge
	baseTime    time.Time
	installedAt time.Time
	self        bool
	purged      bool
}

func newEntry(h packet.LSAHeader, raw []byte, now time.Time, self bool) *Entry {
	return &Entry{
		header:      h,
		raw:         raw,
		baseAge:     h.Age,
		baseTime:    now,
		installedAt: now,
		self:        self,
		purged:      h.Age.IsMaxAge(),
	}
}

// Header returns the LSA header with LS Age advanced from the monotonic elapsed
// time since installation. The Fletcher checksum is unchanged because RFC 2328
// Section 12.1.7 excludes LS Age from the checksum region.
func (e *Entry) Header(now time.Time) packet.LSAHeader {
	h := e.header
	h.Age = e.age(now)
	return h
}

// Key returns the LSDB identity tuple.
func (e *Entry) Key() types.LSAKey { return e.header.Key() }

// Raw returns an owned copy of the stored LSA bytes with LS Age advanced.
func (e *Entry) Raw(now time.Time, transmitDelay uint16) []byte {
	raw := make([]byte, len(e.raw))
	copy(raw, e.raw)
	age := e.age(now).Add(transmitDelay)
	age.WriteTo(raw, lsaAgeOff)
	return raw
}

// LSA returns a lazy packet.LSA view over an owned raw copy. It uses the header the
// codec decoded at install time rather than re-decoding the raw bytes through the OSPFv2
// codec: re-decoding would misparse an OSPFv3 LSA's 16-bit scope-typed LS Type (it sits
// where OSPFv2 has the Options + LS Type octets). Body and RawBytes are the raw spans, so
// an OSPFv3 reader still re-parses RawBytes through the OSPFv3 codec.
func (e *Entry) LSA(now time.Time) (packet.LSA, bool) {
	raw := e.Raw(now, 0)
	if len(raw) < types.LSAHeaderLen {
		return packet.LSA{}, false
	}
	return packet.LSA{
		Header:   e.Header(now),
		Body:     raw[types.LSAHeaderLen:],
		RawBytes: raw,
	}, true
}

func (e *Entry) age(now time.Time) types.LSAge {
	if e.baseAge.DoNotAge() || e.baseTime.IsZero() || now.Before(e.baseTime) {
		return e.baseAge
	}
	elapsed := uint16(0)
	if d := now.Sub(e.baseTime); d > 0 {
		secs := d / time.Second
		if secs >= time.Duration(types.MaxAge) {
			elapsed = types.MaxAge
		} else {
			elapsed = uint16(secs)
		}
	}
	return e.baseAge.Add(elapsed)
}

func (e *Entry) markPurged(now time.Time) {
	e.header.Age = types.LSAge(types.MaxAge)
	e.baseAge = types.LSAge(types.MaxAge)
	e.baseTime = now
	e.purged = true
	if len(e.raw) >= 2 {
		e.header.Age.WriteTo(e.raw, lsaAgeOff)
	}
}

// CompareHeaders applies RFC 2328 Section 13.1 exactly: sequence number,
// checksum, MaxAge, significant age difference, then equality.
func CompareHeaders(a, b packet.LSAHeader) Freshness {
	if a.Sequence != b.Sequence {
		if a.Sequence.NewerThan(b.Sequence) {
			return Newer
		}
		return Older
	}
	if a.Checksum != b.Checksum {
		if a.Checksum > b.Checksum {
			return Newer
		}
		return Older
	}
	if a.Age.IsMaxAge() != b.Age.IsMaxAge() {
		if a.Age.IsMaxAge() {
			return Newer
		}
		return Older
	}
	aa := a.Age.Age()
	ba := b.Age.Age()
	if aa > ba && aa-ba > types.MaxAgeDiff {
		return Older
	}
	if ba > aa && ba-aa > types.MaxAgeDiff {
		return Newer
	}
	return Equal
}

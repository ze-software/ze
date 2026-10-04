// Design: docs/architecture/mrt.md — numeric fields pinned by external octets.
package mrt_test

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/mrt"
)

// TestRFC6396RemainingNumericFields compares independent literal octets with
// each writer, and independently feeds those literals to each reader.
// RFC requirement: RFC6396-1-1 positive -- ET microseconds, both old/new FSM state fields, RIB_GENERIC AFI and sequence, and legacy TABLE_DUMP ViewNumber, SeqNumber, OrigTime, PeerAS and AttributeLength encode and decode in network order, never a mutually wrong roundtrip.
func TestRFC6396RemainingNumericFields(t *testing.T) {
	t.Run("extended-timestamp", func(t *testing.T) {
		want := []byte{0x12, 0x34, 0x56, 0x78, 0, 17, 0, 4, 0, 0, 0, 4, 0, 0x0a, 0xbc, 0xde}
		buf := make([]byte, len(want))
		n := mrt.WriteExtendedHeader(buf, 0, 0x12345678, 0x0abcde, 17, 4, 4)
		if n != len(want) || !bytes.Equal(buf, want) {
			t.Fatalf("ET=%x want=%x", buf, want)
		}
		usec, err := mrt.DecodeMicrosecond([]byte{0, 0x0a, 0xbc, 0xde})
		if err != nil || usec != 0x0abcde {
			t.Fatalf("ET decode=%x %v", usec, err)
		}
	})
	t.Run("states", func(t *testing.T) {
		hdr := mrt.BGP4MPHeader{PeerAS: 0x1234, LocalAS: 0x5678, IfIndex: 0x2345, AFI: 1, PeerIP: []byte{192, 0, 2, 1}, LocalIP: []byte{192, 0, 2, 2}}
		for _, as4 := range []bool{false, true} {
			fields := []byte{0x12, 0x34, 0x56, 0x78}
			subtype := uint16(0)
			if as4 {
				fields = []byte{0, 0, 0x12, 0x34, 0, 0, 0x56, 0x78}
				subtype = 5
			}
			fields = append(fields, 0x23, 0x45, 0, 1, 192, 0, 2, 1, 192, 0, 2, 2, 0, 6, 0, 1)
			buf := make([]byte, len(fields))
			n := mrt.WriteBGP4MPStateChange(buf, 0, &hdr, as4, 6, 1)
			if n != len(fields) || !bytes.Equal(buf, fields) {
				t.Fatalf("state=%x want=%x", buf, fields)
			}
			got, err := mrt.DecodeBGP4MPStateChange(subtype, fields)
			if err != nil || got.OldState != 6 || got.NewState != 1 {
				t.Fatalf("state decode=%+v %v", got, err)
			}
		}
	})
	t.Run("generic-afi", func(t *testing.T) {
		want := []byte{0x12, 0x34, 0x56, 0x78, 0, 2, 1, 32, 0x20, 1, 0x0d, 0xb8, 0, 0}
		buf := make([]byte, len(want))
		n := mrt.WriteRIBGenericHeader(buf, 0, 0x12345678, 2, 1, []byte{32, 0x20, 1, 0x0d, 0xb8})
		n += mrt.WriteRIBEntries(buf, n, nil, false)
		if n != len(want) || !bytes.Equal(buf, want) {
			t.Fatalf("generic=%x want=%x", buf, want)
		}
		got, err := mrt.DecodeRIBGenericRecord(6, want)
		if err != nil || got.AFI != 2 || got.SequenceNumber != 0x12345678 || got.SAFI != 1 {
			t.Fatalf("generic decode=%+v %v", got, err)
		}
	})
	t.Run("table-dump", func(t *testing.T) {
		attrs := bytes.Repeat([]byte{0x42}, 0x0102)
		for _, afi := range []uint16{1, 2} {
			prefix, peer := []byte{10, 20, 30, 0}, []byte{192, 0, 2, 1}
			if afi == 2 {
				prefix = []byte{0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
				peer = []byte{0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
			}
			want := append([]byte{0x12, 0x34, 0x23, 0x45}, prefix...)
			want = append(want, 24, 1, 0x34, 0x56, 0x78, 0x9a)
			want = append(want, peer...)
			want = append(want, 0x45, 0x67, 1, 2)
			want = append(want, attrs...)
			record := mrt.TableDumpRecord{ViewNumber: 0x1234, SeqNumber: 0x2345, Prefix: prefix, PrefixLen: 24, Status: 1, OrigTime: 0x3456789a, PeerIP: peer, PeerAS: 0x4567, Attributes: attrs}
			buf := make([]byte, len(want))
			n := mrt.WriteTableDump(buf, 0, &record)
			if n != len(want) || !bytes.Equal(buf, want) {
				t.Fatalf("TABLE_DUMP afi%d bytes=%x want=%x", afi, buf, want)
			}
			got, err := mrt.DecodeTableDump(afi, want)
			if err != nil || got.ViewNumber != 0x1234 || got.SeqNumber != 0x2345 || got.OrigTime != 0x3456789a || got.PeerAS != 0x4567 || !bytes.Equal(got.Attributes, attrs) {
				t.Fatalf("TABLE_DUMP decode=%+v %v", got, err)
			}
		}
	})
}

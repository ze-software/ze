// Design: docs/architecture/mrt.md — subtype-directed offline NLRI parsing.
package mrt_test

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/mrt"
)

// TestRFC8050SubtypeControlsEveryNLRILocation checks same-mode NLRI layouts.
// Multi-family ADD-PATH fixtures carry both actual OPENs, not a subtype-only guess.
// RFC requirement: RFC8050-x-4 positive -- all four ADD-PATH subtypes under BGP4MP and BGP4MP_ET decode the exact classic and MP announced/withdrawn prefixes; both actual OPENs establish homogeneous modes when multiple families coexist.
// RFC requirement: RFC8050-x-4 negative -- base subtypes decode ordinary NLRI without consuming a Path Identifier; truncated ADD-PATH identifiers in each of the four NLRI locations produce an error, never an invented default route.
func TestRFC8050SubtypeControlsEveryNLRILocation(t *testing.T) {
	for _, typ := range []uint16{mrt.TypeBGP4MP, mrt.TypeBGP4MPET} {
		for _, subtype := range []uint16{1, 4, 6, 7, 8, 9, 10, 11} {
			addPath := subtype >= 8
			for _, afi := range []uint16{1, 2} {
				for _, damaged := range []string{"", "withdrawn", "announced", "reach", "unreach"} {
					if damaged != "" && !addPath {
						continue
					}
					body := contextUpdateBody(addPath, afi, damaged)
					wire := contextRecord(typ, subtype, body)
					mode := byte(0)
					if addPath {
						mode = 3
					}
					wire = append(sessionPrelude(typ, mode, mode), wire...)
					seen := 0
					err := mrt.ReadFrom(bytes.NewReader(wire), &mrt.Handler{OnMessage: func(_ mrt.Header, _ uint32, record *mrt.MessageRecord) error {
						if record.BGPMessage.Bytes[18] != 2 {
							return nil
						}
						seen++
						if record.BGPMessage.AddPath != addPath {
							t.Fatalf("subtype %d AddPath=%v", subtype, record.BGPMessage.AddPath)
						}
						parsed, parseErr := mrt.ParseBGPMessage(record.BGPMessage)
						if parsed == nil || parsed.Update == nil {
							t.Fatalf("subtype %d: no UPDATE: %v", subtype, parseErr)
						}
						if (parseErr != nil) != (damaged == "withdrawn" || damaged == "announced") {
							t.Fatalf("%s classic parse error=%v", damaged, parseErr)
						}
						u := parsed.Update
						want4 := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")}
						if damaged != "withdrawn" && !slices.Equal(u.WithdrawnPrefixes, want4) {
							t.Fatalf("subtype %d withdrawals=%v", subtype, u.WithdrawnPrefixes)
						}
						if damaged != "announced" && !slices.Equal(u.AnnouncedPrefixes, want4) {
							t.Fatalf("subtype %d announcements=%v", subtype, u.AnnouncedPrefixes)
						}
						wantMP := want4
						if afi == 2 {
							wantMP = []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}
						}
						reach, reachErr := mrt.ParseMPReach(mrt.FindAttribute(u.Attributes, 14).Value, u.AddPathFor(afi, 1))
						unreach, unreachErr := mrt.ParseMPUnreach(mrt.FindAttribute(u.Attributes, 15).Value, u.AddPathFor(afi, 1))
						if (reachErr != nil) != (damaged == "reach") || (unreachErr != nil) != (damaged == "unreach") {
							t.Fatalf("%s MP errors=%v/%v", damaged, reachErr, unreachErr)
						}
						if damaged != "reach" && (reach == nil || !slices.Equal(reach.Prefixes, wantMP)) {
							t.Fatalf("subtype %d MP_REACH=%+v", subtype, reach)
						}
						if damaged != "unreach" && (unreach == nil || !slices.Equal(unreach.Prefixes, wantMP)) {
							t.Fatalf("subtype %d MP_UNREACH=%+v", subtype, unreach)
						}
						return nil
					}})
					if err != nil || seen != 1 {
						t.Fatalf("ReadFrom: count=%d error=%v", seen, err)
					}
				}
			}
		}
	}
}

// contextUpdateBody provides independent literal fields for the four locations.
func contextUpdateBody(addPath bool, afi uint16, damaged string) []byte {
	nlri := func(mp bool, location string) []byte {
		if location == damaged {
			return []byte{0, 0, 0}
		}
		var value []byte
		if addPath {
			value = append(value, 0, 0, 0, 1)
		}
		if mp && afi == 2 {
			return append(value, 32, 0x20, 1, 0x0d, 0xb8)
		}
		return append(value, 24, 10, 0, 0)
	}
	reach := []byte{0, byte(afi), 1, 4, 192, 0, 2, 1, 0}
	reach = append(reach, nlri(true, "reach")...)
	unreach := append([]byte{0, byte(afi), 1}, nlri(true, "unreach")...)
	attrs := append([]byte{0x80, 14, byte(len(reach))}, reach...)
	attrs = append(attrs, 0x80, 15, byte(len(unreach)))
	attrs = append(attrs, unreach...)
	withdrawn := nlri(false, "withdrawn")
	body := append([]byte{0, byte(len(withdrawn))}, withdrawn...)
	body = append(body, 0, byte(len(attrs)))
	body = append(body, attrs...)
	return append(body, nlri(false, "announced")...)
}

// contextRecord constructs external MRT framing without invoking the MRT writer.
func contextRecord(typ, subtype uint16, body []byte) []byte {
	fields := []byte{0xfd, 0xe9, 0xfd, 0xe8}
	if subtype == 4 || subtype == 7 || subtype == 9 || subtype == 11 {
		fields = []byte{0, 0, 0xfd, 0xe9, 0, 0, 0xfd, 0xe8}
	}
	fields = append(fields, 0, 0, 0, 1, 192, 0, 2, 1, 192, 0, 2, 2)
	fields = append(fields, buildBGPMessage(2, body)...)
	if typ == mrt.TypeBGP4MPET {
		fields = append([]byte{0, 0, 0, 1}, fields...)
	}
	wire := []byte{0, 0, 0, 1, 0, byte(typ), 0, byte(subtype)}
	wire = binary.BigEndian.AppendUint32(wire, uint32(len(fields)))
	return append(wire, fields...)
}

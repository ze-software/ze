// Design: docs/architecture/mrt.md — subtype-only decoding without invented OPENs.
package mrt_test

import (
	"bytes"
	"fmt"
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/mrt"
)

// TestRFC8050NoOPENSubtypeDecodesExactNLRI reads one independently framed record
// per invocation, so no actual or synthetic OPEN can supply negotiation. Each
// NLRI location has two distinct prefixes and AP IDs zero and 0x01020304.
// MUTATION: BGPMessage.AddPathFor returning false when negotiated is nil loses
// the AP layout; returning true invents IDs for ordinary records. Truncating
// parsePrefixesAFI's 32-bit Path Identifier or dropping zero IDs changes the
// exact ID arrays, even if the prefixes still decode.
// RFC requirement: RFC8050-x-4 positive -- without OPENs, all four ADD-PATH message subtypes under BGP4MP and BGP4MP_ET decode isolated classic announcements/withdrawals and IPv4/IPv6 MP_REACH/MP_UNREACH into exactly two distinct prefixes with Path Identifiers 0 and 0x01020304, preserving the complete message bytes.
// RFC requirement: RFC8050-x-4 negative -- without OPENs, all four ordinary message subtypes under BGP4MP and BGP4MP_ET decode the same isolated NLRI locations into exactly two distinct prefixes without consuming or inventing Path Identifiers.
func TestRFC8050NoOPENSubtypeDecodesExactNLRI(t *testing.T) {
	locations := []struct {
		name      string
		afi       uint16
		attribute uint8
		withdraw  bool
		first     []byte
		second    []byte
		prefixes  []netip.Prefix
	}{
		{"classic-announcement", 1, 0, false, []byte{24, 10, 10, 0}, []byte{24, 10, 11, 0}, []netip.Prefix{netip.MustParsePrefix("10.10.0.0/24"), netip.MustParsePrefix("10.11.0.0/24")}},
		{"classic-withdrawal", 1, 0, true, []byte{24, 10, 12, 0}, []byte{24, 10, 13, 0}, []netip.Prefix{netip.MustParsePrefix("10.12.0.0/24"), netip.MustParsePrefix("10.13.0.0/24")}},
		{"ipv4-MP_REACH", 1, 14, false, []byte{24, 10, 14, 0}, []byte{24, 10, 15, 0}, []netip.Prefix{netip.MustParsePrefix("10.14.0.0/24"), netip.MustParsePrefix("10.15.0.0/24")}},
		{"ipv4-MP_UNREACH", 1, 15, true, []byte{24, 10, 16, 0}, []byte{24, 10, 17, 0}, []netip.Prefix{netip.MustParsePrefix("10.16.0.0/24"), netip.MustParsePrefix("10.17.0.0/24")}},
		{"ipv6-MP_REACH", 2, 14, false, []byte{32, 0x20, 1, 0x0d, 0xb8}, []byte{32, 0x20, 1, 0x0d, 0xb9}, []netip.Prefix{netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2001:db9::/32")}},
		{"ipv6-MP_UNREACH", 2, 15, true, []byte{32, 0x20, 1, 0x0d, 0xba}, []byte{32, 0x20, 1, 0x0d, 0xbb}, []netip.Prefix{netip.MustParsePrefix("2001:dba::/32"), netip.MustParsePrefix("2001:dbb::/32")}},
	}
	for _, typ := range []uint16{mrt.TypeBGP4MP, mrt.TypeBGP4MPET} {
		for _, subtype := range []uint16{1, 4, 6, 7, 8, 9, 10, 11} {
			for _, location := range locations {
				t.Run(fmt.Sprintf("type-%d/subtype-%d/%s", typ, subtype, location.name), func(t *testing.T) {
					addPath := subtype >= 8
					var nlri []byte
					var wantIDs []uint32
					if addPath {
						// RFC 7911 Section 3: the four-octet ID precedes each prefix.
						nlri = append(nlri, 0, 0, 0, 0)
						wantIDs = []uint32{0, 0x01020304}
					}
					nlri = append(nlri, location.first...)
					if addPath {
						nlri = append(nlri, 1, 2, 3, 4)
					}
					nlri = append(nlri, location.second...)
					body := noOPENUpdateBody(location.afi, location.attribute, location.withdraw, nlri)
					wire := contextRecord(typ, subtype, body)
					original := bytes.Clone(wire)
					wantMessage := buildBGPMessage(2, body)
					seen := 0
					err := mrt.ReadFrom(bytes.NewReader(wire), &mrt.Handler{OnMessage: func(h mrt.Header, _ uint32, record *mrt.MessageRecord) error {
						seen++
						if h.Type != typ {
							t.Fatalf("MRT type=%d, want %d", h.Type, typ)
						}
						if h.Subtype != subtype {
							t.Fatalf("MRT subtype=%d, want %d", h.Subtype, subtype)
						}
						if !bytes.Equal(record.BGPMessage.Bytes, wantMessage) {
							t.Fatalf("complete message=%x, want %x", record.BGPMessage.Bytes, wantMessage)
						}
						// RFC 8050 Section 2: this invocation contains no OPEN evidence.
						parsed, err := mrt.ParseBGPMessage(record.BGPMessage)
						if err != nil {
							return err
						}
						u := parsed.Update
						if u == nil {
							t.Fatal("record did not decode as an UPDATE")
						}
						if u.AddPathFor(location.afi, 1) != addPath {
							t.Fatalf("family mode differs from subtype %d", subtype)
						}
						var wantAnnounced, wantWithdrawn []netip.Prefix
						var wantAnnouncedIDs, wantWithdrawnIDs []uint32
						if location.attribute == 0 {
							if location.withdraw {
								wantWithdrawn, wantWithdrawnIDs = location.prefixes, wantIDs
							} else {
								wantAnnounced, wantAnnouncedIDs = location.prefixes, wantIDs
							}
						}
						noOPENAssertNLRI(t, "classic announcements", u.AnnouncedPrefixes, u.AnnouncedPathIDs, wantAnnounced, wantAnnouncedIDs)
						noOPENAssertNLRI(t, "classic withdrawals", u.WithdrawnPrefixes, u.WithdrawnPathIDs, wantWithdrawn, wantWithdrawnIDs)
						if location.attribute != 0 {
							attr := mrt.FindAttribute(u.Attributes, location.attribute)
							if attr == nil {
								t.Fatalf("missing MP attribute %d", location.attribute)
							}
							if location.withdraw {
								// RFC 8050 Section 2 and RFC 7911 Section 3.
								mp, err := mrt.ParseMPUnreach(attr.Value, u.AddPathFor(location.afi, 1))
								if err != nil {
									return err
								}
								noOPENAssertNLRI(t, "MP_UNREACH", mp.Prefixes, mp.PathIDs, location.prefixes, wantIDs)
							} else {
								// RFC 8050 Section 2 and RFC 7911 Section 3.
								mp, err := mrt.ParseMPReach(attr.Value, u.AddPathFor(location.afi, 1))
								if err != nil {
									return err
								}
								noOPENAssertNLRI(t, "MP_REACH", mp.Prefixes, mp.PathIDs, location.prefixes, wantIDs)
							}
						}
						if !bytes.Equal(record.BGPMessage.Bytes, wantMessage) {
							t.Fatal("decoding changed the original message bytes")
						}
						return nil
					}})
					if err != nil {
						t.Fatal(err)
					}
					if seen != 1 {
						t.Fatalf("records=%d, want exactly one UPDATE without OPENs", seen)
					}
					if !bytes.Equal(wire, original) {
						t.Fatal("reader changed the original MRT record")
					}
				})
			}
		}
	}
}

// noOPENUpdateBody supplies independent literal UPDATE framing with one family.
// RFC 7911 Section 3: "In order to carry the Path Identifier in an UPDATE
// message, the NLRI encoding MUST be extended by prepending the Path Identifier
// field, which is of four octets."
// The caller supplies the NLRI bytes, including any Path Identifier.
func noOPENUpdateBody(afi uint16, attribute uint8, withdraw bool, nlri []byte) []byte {
	var withdrawn, attrs, announced []byte
	if !withdraw {
		attrs = []byte{0x40, 1, 1, 0, 0x40, 2, 0}
	}
	switch {
	case attribute != 0:
		value := []byte{0, byte(afi), 1}
		if !withdraw {
			if afi == 1 {
				value = append(value, 4, 192, 0, 2, 1, 0)
			} else {
				value = append(value, 16, 0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0)
			}
		}
		value = append(value, nlri...)
		attrs = append(attrs, 0x80, attribute, byte(len(value)))
		attrs = append(attrs, value...)
	case withdraw:
		withdrawn = nlri
	default:
		attrs = append(attrs, 0x40, 3, 4, 192, 0, 2, 1)
		announced = nlri
	}
	body := append([]byte{0, byte(len(withdrawn))}, withdrawn...)
	body = append(body, 0, byte(len(attrs)))
	body = append(body, attrs...)
	return append(body, announced...)
}

func noOPENAssertNLRI(t *testing.T, location string, prefixes []netip.Prefix, ids []uint32, wantPrefixes []netip.Prefix, wantIDs []uint32) {
	t.Helper()
	if !slices.Equal(prefixes, wantPrefixes) {
		t.Fatalf("%s prefixes=%v, want %v", location, prefixes, wantPrefixes)
	}
	if !slices.Equal(ids, wantIDs) {
		t.Fatalf("%s IDs=%x, want %x", location, ids, wantIDs)
	}
}

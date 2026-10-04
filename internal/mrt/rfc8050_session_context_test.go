// Design: docs/architecture/mrt.md — real OPEN evidence and epoch isolation.
package mrt_test

import (
	"bytes"
	"errors"
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/mrt"
)

// sessionOpenRecord is independent literal RFC 4271/5492/7911 framing, not
// negotiated settings or a call to Ze's OPEN encoder. Both MP families are present.
func sessionOpenRecord(typ uint16, sent bool, mode4, mode6 byte) []byte {
	asn := byte(0xe9)
	subtype := uint16(4)
	if sent {
		asn, subtype = 0xe8, 7
	}
	caps := []byte{1, 4, 0, 1, 0, 1, 1, 4, 0, 2, 0, 1}
	var tuples []byte
	if mode4 != 0 {
		tuples = append(tuples, 0, 1, 1, mode4)
	}
	if mode6 != 0 {
		tuples = append(tuples, 0, 2, 1, mode6)
	}
	if len(tuples) != 0 {
		caps = append(caps, 69, byte(len(tuples)))
		caps = append(caps, tuples...)
	}
	body := []byte{4, 0xfd, asn, 0, 90, 192, 0, 2, asn, byte(2 + len(caps)), 2, byte(len(caps))}
	body = append(body, caps...)
	record := contextRecord(typ, subtype, body)
	record[len(record)-len(body)-1] = 1
	return record
}

func sessionPrelude(typ uint16, mode4, mode6 byte) []byte {
	return append(sessionOpenRecord(typ, false, mode4, mode6), sessionOpenRecord(typ, true, mode4, mode6)...)
}

// mixedUpdate uses four distinct prefixes, avoiding RFC 4760 Section 3's
// undefined duplicate-prefix case. IDs 0x01020304/0x05060708 are independent octets.
func mixedUpdate(classicAP bool) []byte {
	classicAnn, classicWd := []byte{24, 10, 0, 0}, []byte{24, 10, 1, 0}
	mpAnn, mpWd := []byte{32, 0x20, 1, 0x0d, 0xb8}, []byte{32, 0x20, 1, 0x0d, 0xb9}
	if classicAP {
		classicAnn = append([]byte{1, 2, 3, 4}, classicAnn...)
		classicWd = append([]byte{5, 6, 7, 8}, classicWd...)
	} else {
		mpAnn = append([]byte{1, 2, 3, 4}, mpAnn...)
		mpWd = append([]byte{5, 6, 7, 8}, mpWd...)
	}
	reach := append([]byte{0, 2, 1, 16, 0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0}, mpAnn...)
	unreach := append([]byte{0, 2, 1}, mpWd...)
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9, 0x40, 3, 4, 192, 0, 2, 1}
	attrs = append(attrs, 0x80, 14, byte(len(reach)))
	attrs = append(attrs, reach...)
	attrs = append(attrs, 0x80, 15, byte(len(unreach)))
	attrs = append(attrs, unreach...)
	body := append([]byte{0, byte(len(classicWd))}, classicWd...)
	body = append(body, 0, byte(len(attrs)))
	body = append(body, attrs...)
	return append(body, classicAnn...)
}

// TestRFC8050SessionContextDecodesMixedDirections walks actual directional OPENs
// then UPDATEs in opposite modes, checking all four prefixes and all Path IDs.
// RFC requirement: RFC8050-x-4 positive -- both captured OPENs select ordinary classic plus ADD-PATH MP and the reverse in opposite directions; all four distinct prefixes and their exact Path Identifiers survive without rewriting the complete original messages.
// RFC requirement: RFC8050-x-4 negative -- missing, one-sided, repeated OPENs, teardown, identity reuse, opposite-file context and unrelated peers cannot decode a mixed-family ADD-PATH UPDATE; they return explicit context-unavailable errors instead of partial prefixes.
func TestRFC8050SessionContextDecodesMixedDirections(t *testing.T) {
	prelude := append(sessionOpenRecord(16, false, 1, 2), sessionOpenRecord(16, true, 2, 1)...)
	incoming := contextRecord(16, 9, mixedUpdate(false))
	outgoing := contextRecord(16, 11, mixedUpdate(true))
	seen := 0
	err := mrt.ReadFrom(bytes.NewReader(append(append(bytes.Clone(prelude), incoming...), outgoing...)), &mrt.Handler{OnMessage: func(h mrt.Header, _ uint32, record *mrt.MessageRecord) error {
		if record.BGPMessage.Bytes[18] != 2 {
			return nil
		}
		parsed, err := mrt.ParseBGPMessage(record.BGPMessage)
		if err != nil {
			t.Fatal(err)
		}
		u := parsed.Update
		if !slices.Equal(u.AnnouncedPrefixes, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")}) || !slices.Equal(u.WithdrawnPrefixes, []netip.Prefix{netip.MustParsePrefix("10.1.0.0/24")}) {
			t.Fatalf("classic prefixes=%v/%v", u.AnnouncedPrefixes, u.WithdrawnPrefixes)
		}
		reach, err := mrt.ParseMPReach(mrt.FindAttribute(u.Attributes, 14).Value, u.AddPathFor(2, 1))
		if err != nil {
			t.Fatal(err)
		}
		unreach, err := mrt.ParseMPUnreach(mrt.FindAttribute(u.Attributes, 15).Value, u.AddPathFor(2, 1))
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(reach.Prefixes, []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}) || !slices.Equal(unreach.Prefixes, []netip.Prefix{netip.MustParsePrefix("2001:db9::/32")}) {
			t.Fatalf("MP prefixes=%v/%v", reach.Prefixes, unreach.Prefixes)
		}
		annIDs, wdIDs := reach.PathIDs, unreach.PathIDs
		if h.Subtype == 11 {
			annIDs, wdIDs = u.AnnouncedPathIDs, u.WithdrawnPathIDs
			if len(reach.PathIDs)+len(unreach.PathIDs) != 0 {
				t.Fatal("ordinary MP invented IDs")
			}
		} else if len(u.AnnouncedPathIDs)+len(u.WithdrawnPathIDs) != 0 {
			t.Fatal("ordinary classic invented IDs")
		}
		if !slices.Equal(annIDs, []uint32{0x01020304}) || !slices.Equal(wdIDs, []uint32{0x05060708}) {
			t.Fatalf("IDs=%x/%x", annIDs, wdIDs)
		}
		seen++
		return nil
	}})
	if err != nil || seen != 2 {
		t.Fatalf("read=%v seen=%d", err, seen)
	}

	state := []byte{0, 0, 0, 1, 0, 16, 0, 5, 0, 0, 0, 24, 0, 0, 0xfd, 0xe9, 0, 0, 0xfd, 0xe8, 0, 0, 0, 1, 192, 0, 2, 1, 192, 0, 2, 2, 0, 6, 0, 1}
	reused := bytes.Clone(incoming)
	reused[15]++ // Peer AS differs from the actual OPEN identity.
	other := bytes.Clone(incoming)
	other[27]++ // Peer endpoint differs, not just its message direction.
	otherInterface := bytes.Clone(incoming)
	otherInterface[21]++
	notification := contextRecord(16, 4, []byte{6, 4})
	notification[12+20+18] = 3
	cases := map[string][]byte{
		"absent":             incoming,
		"one-sided":          append(sessionOpenRecord(16, false, 1, 2), incoming...),
		"repeated":           append(append(sessionOpenRecord(16, false, 1, 2), sessionOpenRecord(16, false, 1, 2)...), incoming...),
		"reset":              append(append(bytes.Clone(prelude), state...), incoming...),
		"identity-reuse":     append(bytes.Clone(prelude), reused...),
		"other-peer":         append(bytes.Clone(prelude), other...),
		"other-interface":    append(bytes.Clone(prelude), otherInterface...),
		"notification":       append(append(bytes.Clone(prelude), notification...), incoming...),
		"new-epoch-one-open": append(append(bytes.Clone(prelude), sessionOpenRecord(16, false, 1, 2)...), incoming...),
	}
	for name, wire := range cases {
		t.Run(name, func(t *testing.T) {
			err := mrt.ReadFrom(bytes.NewReader(wire), &mrt.Handler{OnMessage: func(_ mrt.Header, _ uint32, record *mrt.MessageRecord) error {
				if record.BGPMessage.Bytes[18] != 2 {
					return nil
				}
				_, err := mrt.ParseBGPMessage(record.BGPMessage)
				return err
			}})
			if !errors.Is(err, mrt.ErrContextUnavailable) {
				t.Fatalf("want explicit unavailable context, got %v", err)
			}
		})
	}
	// A new ReadFrom with the same Handler still owns a fresh epoch store.
	handler := &mrt.Handler{OnMessage: func(_ mrt.Header, _ uint32, r *mrt.MessageRecord) error {
		_, err := mrt.ParseBGPMessage(r.BGPMessage)
		return err
	}}
	if err := mrt.ReadFrom(bytes.NewReader(prelude), handler); err != nil {
		t.Fatal(err)
	}
	if err := mrt.ReadFrom(bytes.NewReader(incoming), handler); !errors.Is(err, mrt.ErrContextUnavailable) {
		t.Fatalf("file leakage: %v", err)
	}
	// Post-handshake recorder notification must not erase the real OPENs.
	state[33], state[35] = 1, 6
	if err := mrt.ReadFrom(bytes.NewReader(append(append(bytes.Clone(prelude), state...), incoming...)), handler); err != nil {
		t.Fatal(err)
	}
	// Dynamic OPEN records legitimately precede the resolved header ASN. A
	// pre-established KEEPALIVE must not erase their actual OPEN identity.
	dynamic := bytes.Clone(prelude)
	firstLength := 12 + (int(dynamic[8])<<24 | int(dynamic[9])<<16 | int(dynamic[10])<<8 | int(dynamic[11]))
	for _, offset := range []int{12, firstLength + 12} {
		clear(dynamic[offset : offset+8])
	}
	keepalive := contextRecord(16, 4, nil)
	keepalive[12+20+18] = 4
	clear(keepalive[12:20])
	dynamic = append(append(dynamic, keepalive...), incoming...)
	if err := mrt.ReadFrom(bytes.NewReader(dynamic), handler); err != nil {
		t.Fatalf("dynamic OPEN identity: %v", err)
	}

	// An attached context remains immutable when the same endpoint negotiates a
	// later epoch. Retain bytes explicitly; ownership is not part of the context.
	var retained mrt.BGPMessage
	stream := append(append(bytes.Clone(prelude), incoming...), sessionPrelude(16, 3, 0)...)
	err = mrt.ReadFrom(bytes.NewReader(stream), &mrt.Handler{OnMessage: func(_ mrt.Header, _ uint32, r *mrt.MessageRecord) error {
		if r.BGPMessage.Bytes[18] == 2 {
			retained = r.BGPMessage
			retained.Bytes = bytes.Clone(retained.Bytes)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if retained.AddPathFor(1, 1) || !retained.AddPathFor(2, 1) {
		t.Fatal("later epoch mutated an earlier directional context")
	}
	if _, err = mrt.ParseBGPMessage(retained); err != nil {
		t.Fatal(err)
	}
}

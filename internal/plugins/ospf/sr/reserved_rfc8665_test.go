// Design: docs/architecture/wire/ospf.md -- SR TLV and sub-TLV codec.
// RFC: rfc/short/rfc8665.md -- Reserved fields of the SR TLVs (§3.2, §3.3, §3.4, §4, §5, §6.1, §6.2).
package sr

import (
	"reflect"
	"testing"
)

// reservedCase is one SR TLV or sub-TLV value with the octet range its layout marks
// Reserved and one octet outside that range that the decoder does read.
type reservedCase struct {
	name     string
	value    []byte
	reserved [2]int // [from, to) octet range of the Reserved field
	read     int    // an octet the decoder reads, so a change there must change the result
	decode   func([]byte) (any, error)
}

func reservedCases() []reservedCase {
	sid := PrefixSID{Flags: SIDFlags{}, Algorithm: 0, Index: 7}
	adj := AdjSID{Flags: AdjSIDFlags{V: true, L: true}, Weight: 1, Label: 20001, IsLabel: true}
	lan := AdjSID{Flags: AdjSIDFlags{V: true, L: true}, Weight: 1, NeighborID: [4]byte{10, 0, 0, 2}, Label: 20002, IsLabel: true, IsLAN: true}
	return []reservedCase{
		{name: "sid-label-range-3.2", value: EncodeRangeValue(srgbRange(16000, 100)), reserved: [2]int{3, 4}, read: 2,
			decode: func(v []byte) (any, error) { return DecodeRangeValue(v) }},
		{name: "srlb-3.3", value: EncodeRangeValue(srgbRange(15000, 10)), reserved: [2]int{3, 4}, read: 2,
			decode: func(v []byte) (any, error) { return DecodeRangeValue(v) }},
		{name: "srms-preference-3.4", value: EncodeSRMSValue(128), reserved: [2]int{1, 4}, read: 0,
			decode: func(v []byte) (any, error) { return DecodeSRMSValue(v) }},
		{name: "extended-prefix-range-4", value: EncodeExtPrefixRangeValueV4(24, [4]byte{10, 1, 0, 0}, 4, false, sid), reserved: [2]int{5, 8}, read: 0,
			decode: func(v []byte) (any, error) { return decodeExtPrefixRangeValueV4(v) }},
		{name: "prefix-sid-5", value: EncodePrefixSIDValue(sid), reserved: [2]int{1, 2}, read: 3,
			decode: func(v []byte) (any, error) { return DecodePrefixSIDValue(v) }},
		{name: "adj-sid-6.1", value: EncodeAdjSIDValue(adj), reserved: [2]int{1, 2}, read: 3,
			decode: func(v []byte) (any, error) { return DecodeAdjSIDValue(v) }},
		{name: "lan-adj-sid-6.2", value: EncodeLANAdjSIDValue(lan), reserved: [2]int{1, 2}, read: 3,
			decode: func(v []byte) (any, error) { return DecodeLANAdjSIDValue(v) }},
	}
}

// withOctets returns a copy of v with the octets in [from, to) set to fill.
func withOctets(v []byte, from, to int, fill byte) []byte {
	out := append([]byte(nil), v...)
	for i := from; i < to; i++ {
		out[i] = fill
	}
	return out
}

// RFC requirement: RFC8665-3.2-14 positive -- for every SR TLV and sub-TLV, a received value
// whose Reserved field is all ones decodes to exactly the same result as the value with the
// Reserved field zero, without an error: the decoders never read the Reserved octets
// (DecodeRangeValue, DecodeSRMSValue, decodeExtPrefixRangeValueV4, DecodePrefixSIDValue,
// DecodeAdjSIDValue, DecodeLANAdjSIDValue, codec.go).
func TestRFC8665ReservedFieldIgnoredOnReception(t *testing.T) {
	for _, tc := range reservedCases() {
		t.Run(tc.name, func(t *testing.T) {
			want, err := tc.decode(tc.value)
			if err != nil {
				t.Fatalf("decode of the encoded value: %v", err)
			}
			dirty := withOctets(tc.value, tc.reserved[0], tc.reserved[1], 0xff)
			got, err := tc.decode(dirty)
			if err != nil {
				t.Fatalf("decode with Reserved set to 0xff: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Reserved octets changed the decode: got %+v, want %+v", got, want)
			}
		})
	}
}

// RFC requirement: RFC8665-3.2-14 negative -- the equality above is not vacuous: changing an
// octet the layout does NOT mark Reserved (Range Size, Preference, Prefix Length, Algorithm,
// Weight) changes the decoded result, so the decoders read their fields and ignore only the
// Reserved ones (same decoders, codec.go).
func TestRFC8665NonReservedOctetIsRead(t *testing.T) {
	for _, tc := range reservedCases() {
		t.Run(tc.name, func(t *testing.T) {
			want, err := tc.decode(tc.value)
			if err != nil {
				t.Fatalf("decode of the encoded value: %v", err)
			}
			changed := withOctets(tc.value, tc.read, tc.read+1, tc.value[tc.read]^0x01)
			got, err := tc.decode(changed)
			if err == nil && reflect.DeepEqual(got, want) {
				t.Fatalf("octet %d is read by the layout yet changing it left the decode unchanged: %+v", tc.read, got)
			}
		})
	}
}

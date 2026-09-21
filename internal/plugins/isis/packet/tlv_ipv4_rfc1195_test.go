// Design: docs/architecture/isis/isis-1-types.md -- RFC 1195 section 4.2, several
// IP addresses on one interface.
//
// Goal: prove that this router reads every address a neighbor assigns to one
// interface, up to the 63 that fit one TLV 132, and never reads a partial address.
// Method: encode 63 distinct addresses into one TLV 132 value, decode it, and
// compare each address; then truncate the last address and decode again.

package packet

import (
	"errors"
	"net/netip"
	"testing"
)

// rfc1195MaxAddrsPerInterface is the RFC 1195 section 4.2 maximum: 63 addresses of
// 4 octets fill 252 of the 255 octets one TLV value can carry.
const rfc1195MaxAddrsPerInterface = 63

// interfaceAddrTLVValue encodes n distinct addresses 10.0.0.1 .. 10.0.0.n as one
// TLV 132 value.
func interfaceAddrTLVValue(n int) []byte {
	value := make([]byte, 0, n*IPv4AddrLen)
	for i := 1; i <= n; i++ {
		value = append(value, 10, 0, 0, byte(i))
	}
	return value
}

// RFC requirement: RFC1195-4.2-1 positive -- a TLV 132 carrying 63 addresses (the
// maximum for one interface) decodes to all 63 addresses, each in order, so a
// neighbor that assigns several addresses to one interface is read in full.
func TestRFC1195InterfaceAddrTLV63Addresses(t *testing.T) {
	value := interfaceAddrTLVValue(rfc1195MaxAddrsPerInterface)
	if len(value) > MaxTLVValueLen {
		t.Fatalf("63 addresses take %d octets, more than one TLV value holds (%d)", len(value), MaxTLVValueLen)
	}
	got, err := DecodeIPv4InterfaceAddrTLV(value)
	if err != nil {
		t.Fatalf("DecodeIPv4InterfaceAddrTLV: %v", err)
	}
	if len(got.Addresses) != rfc1195MaxAddrsPerInterface {
		t.Fatalf("decoded %d addresses, want %d", len(got.Addresses), rfc1195MaxAddrsPerInterface)
	}
	for i, a := range got.Addresses {
		want := netip.AddrFrom4([4]byte{10, 0, 0, byte(i + 1)})
		if a != want {
			t.Fatalf("address %d = %s, want %s", i, a, want)
		}
	}
}

// RFC requirement: RFC1195-4.2-1 negative -- a TLV 132 whose 63rd address is cut
// short is refused with ErrLength and yields no addresses, so a partial address
// never reaches the adjacency next-hop table.
func TestRFC1195InterfaceAddrTLVTruncatedAddressRefused(t *testing.T) {
	value := interfaceAddrTLVValue(rfc1195MaxAddrsPerInterface)
	value = value[:len(value)-1]
	got, err := DecodeIPv4InterfaceAddrTLV(value)
	if !errors.Is(err, ErrLength) {
		t.Fatalf("truncated TLV 132: err = %v, want ErrLength", err)
	}
	if len(got.Addresses) != 0 {
		t.Fatalf("truncated TLV 132 yielded %d addresses, want none", len(got.Addresses))
	}
}

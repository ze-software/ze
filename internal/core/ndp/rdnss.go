// Design: docs/architecture/core-design.md -- shared IPv6 Neighbor Discovery encoding
// Related: ra.go -- BuildRA and writeRDNSS, the sender's side of the same option

package ndp

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"strconv"
)

// RDNSS is one received Recursive DNS Server option (RFC 8106 Section 5.1):
// the addresses it carries, in option order, and their shared Lifetime. Only
// ParseRDNSS builds one, so its addresses are always unicast.
type RDNSS struct {
	// Lifetime is the seconds, from receipt, the addresses may be used.
	// LifetimeInfinity means no end; 0 means they MUST no longer be used.
	Lifetime uint32
	// Servers are the option's addresses, in the order the router sent them.
	Servers []netip.Addr
}

// Infinite reports whether the Lifetime is all one bits.
// RFC 8106 Section 5.1: "A value of all one bits (0xffffffff) represents
// infinity."
func (r RDNSS) Infinite() bool { return r.Lifetime == LifetimeInfinity }

// errRDNSSTruncated refuses an option shorter than its fixed part or than
// its own Length field says.
var errRDNSSTruncated = errors.New("rdnss option truncated")

// errNotRDNSS refuses an option whose Type is not RDNSS (25).
var errNotRDNSS = errors.New("option is not rdnss (type 25)")

// RDNSSLengthError refuses an option whose Length field fails RFC 8106
// Section 5.3.1's validity check. Length is the field as received, so the
// discard can be logged with it.
type RDNSSLengthError struct {
	Length uint8
}

func (e *RDNSSLengthError) Error() string {
	return "rdnss option length " + strconv.Itoa(int(e.Length)) +
		" is invalid: RFC 8106 requires at least 3 and an odd value"
}

// RDNSSAddressError refuses an option that carries an address that is not
// unicast.
type RDNSSAddressError struct {
	Address netip.Addr
}

func (e *RDNSSAddressError) Error() string {
	return "rdnss option carries " + e.Address.String() + ", which is not a unicast address"
}

// ParseRDNSS decodes one RDNSS option, starting at its Type octet, and
// checks it as RFC 8106 Section 5.3.1 says a host does before using it. Any
// failure is an error and the caller discards the whole option: no address
// of an invalid option is returned.
//
// Wire format (RFC 8106 Section 5.1):
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|     Type      |     Length    |           Reserved            |  off 0
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                           Lifetime                            |  off 4
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	:            Addresses of IPv6 Recursive DNS Servers            :  off 8, 16 each
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
func ParseRDNSS(option []byte) (RDNSS, error) {
	if len(option) < rdnssOptionFixed {
		return RDNSS{}, errRDNSSTruncated
	}
	if option[0] != OptRDNSS {
		return RDNSS{}, errNotRDNSS
	}
	length := option[1]
	// RFC 8106 Section 5.3.1: "The validity of DNS options is checked with
	// the Length field; that is, the value of the Length field in the RDNSS
	// option is greater than or equal to the minimum value (3) and satisfies
	// the requirement that (Length - 1) % 2 == 0."
	// RFC 8106 Section 5.3.1: "Otherwise, the host MUST discard the options."
	if length < 3 {
		return RDNSS{}, &RDNSSLengthError{Length: length}
	}
	if (length-1)%2 != 0 {
		return RDNSS{}, &RDNSSLengthError{Length: length}
	}
	octets := int(length) * 8
	if octets > len(option) {
		return RDNSS{}, errRDNSSTruncated
	}
	// The Length bound above caps the loop: at most RDNSSMax addresses.
	count := (octets - rdnssOptionFixed) / rdnssAddressLen
	servers := make([]netip.Addr, 0, count)
	for i := range count {
		at := rdnssOptionFixed + i*rdnssAddressLen
		server := netip.AddrFrom16([16]byte(option[at : at+rdnssAddressLen]))
		// RFC 8106 Section 5.3.1: "Also, the validity of the RDNSS option
		// is checked with the "Addresses of IPv6 Recursive DNS Servers"
		// field; that is, the addresses should be unicast addresses."
		if server.IsMulticast() {
			return RDNSS{}, &RDNSSAddressError{Address: server}
		}
		if server.IsUnspecified() {
			return RDNSS{}, &RDNSSAddressError{Address: server}
		}
		servers = append(servers, server)
	}
	return RDNSS{Lifetime: binary.BigEndian.Uint32(option[4:8]), Servers: servers}, nil
}

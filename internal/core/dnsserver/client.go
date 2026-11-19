// Design: docs/architecture/dns/server-harness.md -- EDNS0/packet client-IP resolution
// RFC: rfc/short/rfc7871.md -- EDNS0 Client Subnet

package dnsserver

import (
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/miekg/dns"
)

// The ECS FAMILY values RFC 7871 Section 6 defines an address format for, and
// the zero FAMILY a query carries when it gives no address at all.
//
// RFC 7871 Section 6: "This document only defines the format for FAMILY 1
// (IPv4) and FAMILY 2 (IPv6), which are as follows:"
const (
	subnetFamilyNone uint16 = 0
	subnetFamilyIPv4 uint16 = 1
	subnetFamilyIPv6 uint16 = 2
)

// The client-IP selection modes a consumer passes to ClientIP. Any other value
// is read as edns0-then-packet.
const (
	clientModePacket = "packet"
	clientModeEDNS0  = "edns0"
)

// ErrMalformedSubnet is the error ClientIP wraps when the query's EDNS0
// client-subnet option is wrongly formatted. The consumer answers it FORMERR.
var ErrMalformedSubnet = errors.New("malformed EDNS0 client-subnet option")

// Client is what a query resolves to for source-based answer selection.
//
// Addr is the address the selection runs on. It is not valid when the query
// resolves to no client (edns0 mode, and no option that carries address bits),
// and no prefix contains an invalid Addr, so no source entry is chosen for it.
//
// FromSubnet reports that Addr is the option's ADDRESS, not the packet source.
// Only then does the answer depend on the information the option carried.
//
// Subnet is the query's client-subnet option, already checked by ClientIP, when
// the mode reads the option and the query carried one. It is nil in packet mode
// and for a query with no option, and then the reply carries no option either
// (EchoSubnet).
type Client struct {
	Addr       netip.Addr
	FromSubnet bool
	Subnet     *dns.EDNS0_SUBNET
}

// ClientIP resolves the client used for source-based answer selection, per
// mode, and returns an error wrapping ErrMalformedSubnet for a query whose
// client-subnet option is wrongly formatted.
//
// mode is one of "packet" (the option is not read, the packet source is the
// client), "edns0" (the option only: no client without one) or any other value
// (edns0-then-packet: the option first, then the packet source).
//
// An option whose SOURCE PREFIX-LENGTH is 0 carries no address bits, so the
// client is the one the mode falls back to, never the option's zero ADDRESS.
// RFC 7871 Section 7.1.2: "A SOURCE PREFIX-LENGTH value of 0 means that the
// Recursive Resolver MUST NOT add the client's address information to its
// queries."
func ClientIP(r *dns.Msg, packetSrc netip.Addr, mode string) (Client, error) {
	var c Client
	if mode != clientModePacket {
		subnet, present, err := querySubnet(r)
		if err != nil {
			return Client{}, err
		}
		if present {
			c.Subnet = subnet
		}
		if present && subnet.SourceNetmask > 0 {
			addr, err := subnetAddr(subnet)
			if err != nil {
				return Client{}, err
			}
			c.Addr, c.FromSubnet = addr, true
			return c, nil
		}
		if mode == clientModeEDNS0 {
			return c, nil
		}
	}
	if packetSrc.IsValid() {
		c.Addr = packetSrc.Unmap()
	}
	return c, nil
}

// querySubnet returns the query's client-subnet option and true, false when the
// query carries none, or an error wrapping ErrMalformedSubnet.
//
// RFC 7871 Section 7.2.1: "A query with a wrongly formatted option (e.g., an
// unknown FAMILY) MUST be rejected and a FORMERR response MUST be returned to
// the sender, as described in [RFC6891], "Transport Considerations"."
//
// A second option in the same query is refused too: the reply carries one
// option whose FAMILY, SOURCE PREFIX-LENGTH and ADDRESS must match the query's,
// and two options in one query leave no single set of fields to match.
func querySubnet(r *dns.Msg) (*dns.EDNS0_SUBNET, bool, error) {
	opt := r.IsEdns0()
	if opt == nil {
		return nil, false, nil
	}
	var found *dns.EDNS0_SUBNET
	for _, o := range opt.Option {
		subnet, ok := o.(*dns.EDNS0_SUBNET)
		if !ok {
			continue
		}
		if found != nil {
			return nil, false, fmt.Errorf("%w: more than one option in the query", ErrMalformedSubnet)
		}
		if err := checkSubnet(subnet); err != nil {
			return nil, false, err
		}
		found = subnet
	}
	return found, found != nil, nil
}

// checkSubnet refuses a client-subnet option a query must not carry.
//
// miekg/dns already refuses, while unpacking, a FAMILY other than 0, 1 or 2, a
// FAMILY 0 with address bits, and a prefix length wider than the family, and
// its server then answers FORMERR before any handler runs. This check holds
// the same rules for a message that never went through that unpack, and adds
// the two miekg/dns does not hold: SCOPE PREFIX-LENGTH and the ADDRESS bits
// past SOURCE PREFIX-LENGTH.
//
// FAMILY 0 is accepted with SOURCE PREFIX-LENGTH 0, the form `dig +subnet=0`
// sends. RFC 7871 Section 7.1.2 only asks a stub for a FAMILY there: "If there
// is no ADDRESS set, i.e., SOURCE PREFIX-LENGTH is set to 0, then FAMILY SHOULD
// be set to the transport over which the query is sent."
func checkSubnet(subnet *dns.EDNS0_SUBNET) error {
	addr, err := subnetAddr(subnet)
	if err != nil {
		return err
	}
	width := 0
	if addr.IsValid() {
		width = addr.BitLen()
	}
	if int(subnet.SourceNetmask) > width {
		return fmt.Errorf("%w: SOURCE PREFIX-LENGTH %d is longer than FAMILY %d allows",
			ErrMalformedSubnet, subnet.SourceNetmask, subnet.Family)
	}
	// RFC 7871 Section 6: "In queries, it MUST be set to 0."
	if subnet.SourceScope != 0 {
		return fmt.Errorf("%w: SCOPE PREFIX-LENGTH is %d in a query", ErrMalformedSubnet, subnet.SourceScope)
	}
	if !addr.IsValid() {
		return nil
	}
	// RFC 7871 Section 6: "ADDRESS, variable number of octets, contains either
	// an IPv4 or IPv6 address, depending on FAMILY, which MUST be truncated to
	// the number of bits indicated by the SOURCE PREFIX-LENGTH field, padding
	// with 0 bits to pad to the end of the last octet needed."
	if netip.PrefixFrom(addr, int(subnet.SourceNetmask)).Masked().Addr() != addr {
		return fmt.Errorf("%w: ADDRESS %s has bits set past SOURCE PREFIX-LENGTH %d",
			ErrMalformedSubnet, addr, subnet.SourceNetmask)
	}
	return nil
}

// subnetAddr returns the option's ADDRESS in its FAMILY, the zero address of the
// family for an option that carries no ADDRESS octets, and an invalid Addr for
// FAMILY 0, which names no family.
//
// RFC 7871 Section 6: "The format of the address part depends on the value of
// FAMILY.  This document only defines the format for FAMILY 1 (IPv4) and
// FAMILY 2 (IPv6), which are as follows:"
func subnetAddr(subnet *dns.EDNS0_SUBNET) (netip.Addr, error) {
	switch subnet.Family {
	case subnetFamilyNone:
		return netip.Addr{}, nil
	case subnetFamilyIPv4:
		if len(subnet.Address) == 0 {
			return netip.IPv4Unspecified(), nil
		}
		ip4 := subnet.Address.To4()
		if ip4 == nil {
			return netip.Addr{}, fmt.Errorf("%w: ADDRESS is not IPv4 under FAMILY 1", ErrMalformedSubnet)
		}
		return netip.AddrFrom4([4]byte(ip4)), nil
	case subnetFamilyIPv6:
		if len(subnet.Address) == 0 {
			return netip.IPv6Unspecified(), nil
		}
		if len(subnet.Address) != net.IPv6len {
			return netip.Addr{}, fmt.Errorf("%w: ADDRESS is not IPv6 under FAMILY 2", ErrMalformedSubnet)
		}
		return netip.AddrFrom16([16]byte(subnet.Address)), nil
	}
	// The FAMILY set is open: the value comes off the wire.
	return netip.Addr{}, fmt.Errorf("%w: unknown FAMILY %d", ErrMalformedSubnet, subnet.Family)
}

// EchoSubnet adds to msg the client-subnet option an Authoritative Nameserver
// that implements RFC 7871 returns in answer to c.Subnet, with SCOPE
// PREFIX-LENGTH scope. It adds nothing when c.Subnet is nil: the query carried
// no option, or the consumer does not read it.
//
// RFC 7871 Section 7.2.1: "An Authoritative Nameserver that implements this
// protocol and receives an ECS option MUST include an ECS option in its
// response to indicate that it SHOULD be cached accordingly, regardless of
// whether the client information was needed to formulate an answer."
//
// RFC 7871 Section 7.2.1: "If an ECS option was not included in a query, one
// MUST NOT be included in the response even if the server is providing a
// Tailored Response -- presumably based on the address from which it received
// the query."
//
// RFC 7871 Section 7.2.1: "FAMILY, SOURCE PREFIX-LENGTH, and ADDRESS in the
// response MUST match those in the query."
//
// A query that gave no FAMILY (FAMILY 0, SOURCE PREFIX-LENGTH 0) answered from
// the packet source gets the family of that address, since a zero field in the
// query is not one the reply has to match.
// RFC 7871 Section 7.3: "In a response to a query that specified only SOURCE
// PREFIX-LENGTH for privacy masking, the FAMILY and ADDRESS fields MUST contain
// the appropriate non-zero information that the Authoritative Nameserver used
// to generate the answer, so that it can be cached accordingly."
// The ADDRESS stays empty there: SOURCE PREFIX-LENGTH 0 truncates it to no
// octets (RFC 7871 Section 6).
//
// The OPT advertises 512 octets, the read buffer of an authoritative listener
// (Options.UDPSize left zero), with version 0 and DO clear: Ze signs nothing.
func EchoSubnet(msg *dns.Msg, c Client, scope uint8) {
	if c.Subnet == nil {
		return
	}
	family := c.Subnet.Family
	if family == subnetFamilyNone && c.Addr.IsValid() {
		family = subnetFamilyIPv6
		if c.Addr.Is4() {
			family = subnetFamilyIPv4
		}
	}
	// miekg/dns packs FAMILY 1 and 2 from a full-width address, and masks it to
	// SOURCE PREFIX-LENGTH itself, so the octets on the wire are the query's.
	address := c.Subnet.Address
	if len(address) == 0 {
		address = net.IPv4zero
		if family == subnetFamilyIPv6 {
			address = net.IPv6zero
		}
	}
	opt := &dns.OPT{Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT}}
	opt.SetUDPSize(dns.MinMsgSize)
	opt.Option = []dns.EDNS0{&dns.EDNS0_SUBNET{
		Code:          dns.EDNS0SUBNET,
		Family:        family,
		SourceNetmask: c.Subnet.SourceNetmask,
		SourceScope:   scope,
		Address:       address,
	}}
	msg.Extra = append(msg.Extra, opt)
}

// RemoteAddr extracts the client IP from a Peer's remote address (the UDP/TCP
// packet source, before any EDNS0 client-subnet lookup). Unmapped to plain
// IPv4 form when applicable, matching ClientIP's own normalization: a 4-in-6
// address (::ffff:a.b.c.d) fed raw into netip.Prefix.Contains against a v4
// prefix never matches (family mismatch), which would wrongly deny a
// legitimate v4 client to a consumer (e.g. as112's allow-from matcher) that
// uses this result directly instead of routing it through ClientIP.
func RemoteAddr(p Peer) netip.Addr {
	host, _, err := net.SplitHostPort(p.RemoteAddr().String())
	if err != nil {
		return netip.Addr{}
	}
	a, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return a.Unmap()
}

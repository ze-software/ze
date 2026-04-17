// Design: docs/research/l2tpv2-ze-integration.md -- IPCP codec + options
// Related: ppp_fsm.go -- shared RFC 1661 FSM driving IPCP
// Related: lcp.go -- shared packet shape (Code/Identifier/Length/Data)

package ppp

// RFC 1332 Section 3: IPCP uses the same packet format as LCP and a
// subset of the codes (1-7). The option codec is IPCP-specific.
// RFC 1877 Section 1 extends IPCP with Primary/Secondary DNS options
// (types 129 and 131).

import (
	"errors"
	"net/netip"
)

// IPCP option type values.
//
// RFC 1332 §3.3: IP-Address (type 3) replaces the deprecated
// IP-Addresses (1) and IP-Compression-Protocol (2) options.
// RFC 1877 §1.1-§1.2: Primary-DNS / Secondary-DNS.
const (
	IPCPOptIPAddress    uint8 = 3
	IPCPOptPrimaryDNS   uint8 = 129
	IPCPOptSecondaryDNS uint8 = 131
)

// ipcpIPv4OptLen is the on-wire length of every IPCP IPv4-valued
// option that ze recognizes: 1 type + 1 length + 4 bytes = 6.
const ipcpIPv4OptLen = 6

var (
	errIPCPBadOptionLen = errors.New("ppp: IPCP option length invalid")
	errIPCPNotIPv4      = errors.New("ppp: IPCP option address is not IPv4")
)

// IPCPOptions carries the parsed option set for one IPCP packet. Zero-
// valued fields mean "option not present". Address fields are
// netip.Addr for allocation-free comparisons against
// netip.IPv4Unspecified().
type IPCPOptions struct {
	IPAddress    netip.Addr
	PrimaryDNS   netip.Addr
	SecondaryDNS netip.Addr
	HasIPAddress bool
	HasPrimary   bool
	HasSecondary bool
}

// isKnownIPCPOption returns true for option types ze recognizes. Kept
// separate from ParseIPCPOptions so a peer's Configure-Request can be
// scanned for Reject-worthy unknowns without touching address data.
func isKnownIPCPOption(t uint8) bool {
	return t == IPCPOptIPAddress || t == IPCPOptPrimaryDNS || t == IPCPOptSecondaryDNS
}

// ParseIPCPOptions walks an IPCP option list (the Data field of a
// Configure-Request/Ack/Nak/Reject) into the struct. Unknown option
// types are skipped -- the FSM layer decides whether to Reject them
// via ipcpHasUnknownOption.
//
// Returns an error only when the wire shape is structurally malformed.
func ParseIPCPOptions(buf []byte) (IPCPOptions, error) {
	var out IPCPOptions
	off := 0
	for off < len(buf) {
		if len(buf)-off < 2 {
			return IPCPOptions{}, errOptionTooShort
		}
		t := buf[off]
		l := int(buf[off+1])
		if l < 2 || off+l > len(buf) {
			return IPCPOptions{}, errOptionLengthMismatch
		}
		data := buf[off+2 : off+l]
		if !isKnownIPCPOption(t) {
			off += l
			continue
		}
		addr, err := parseIPCPv4Option(l, data)
		if err != nil {
			return IPCPOptions{}, err
		}
		switch t {
		case IPCPOptIPAddress:
			out.IPAddress = addr
			out.HasIPAddress = true
		case IPCPOptPrimaryDNS:
			out.PrimaryDNS = addr
			out.HasPrimary = true
		case IPCPOptSecondaryDNS:
			out.SecondaryDNS = addr
			out.HasSecondary = true
		}
		off += l
	}
	return out, nil
}

func parseIPCPv4Option(optLen int, data []byte) (netip.Addr, error) {
	if optLen != ipcpIPv4OptLen {
		return netip.Addr{}, errIPCPBadOptionLen
	}
	addr, ok := netip.AddrFromSlice(data)
	if !ok || !addr.Is4() {
		return netip.Addr{}, errIPCPNotIPv4
	}
	return addr.Unmap(), nil
}

// WriteIPCPOptions encodes the populated fields of opts into buf at
// offset off and returns the number of bytes written. Only options
// marked Has* are serialized. Caller MUST ensure buf has capacity
// >= ipcpMaxOptionsWireLen.
func WriteIPCPOptions(buf []byte, off int, opts IPCPOptions) int {
	start := off
	if opts.HasIPAddress {
		off += writeIPCPv4Option(buf, off, IPCPOptIPAddress, opts.IPAddress)
	}
	if opts.HasPrimary {
		off += writeIPCPv4Option(buf, off, IPCPOptPrimaryDNS, opts.PrimaryDNS)
	}
	if opts.HasSecondary {
		off += writeIPCPv4Option(buf, off, IPCPOptSecondaryDNS, opts.SecondaryDNS)
	}
	return off - start
}

// writeIPCPv4Option encodes a single 6-byte IPCP option: type,
// length=6, 4-byte IPv4 address. Caller-supplied addr MUST be IPv4.
func writeIPCPv4Option(buf []byte, off int, optType uint8, addr netip.Addr) int {
	buf[off] = optType
	buf[off+1] = ipcpIPv4OptLen
	a4 := addr.As4()
	copy(buf[off+2:off+ipcpIPv4OptLen], a4[:])
	return ipcpIPv4OptLen
}

// ipcpHasUnknownOption returns true if buf contains at least one IPCP
// option whose type is not recognized. Used when deciding whether to
// Configure-Reject a peer's Request.
func ipcpHasUnknownOption(buf []byte) bool {
	off := 0
	for off < len(buf) {
		if len(buf)-off < 2 {
			return false
		}
		t := buf[off]
		l := int(buf[off+1])
		if l < 2 || off+l > len(buf) {
			return false
		}
		if !isKnownIPCPOption(t) {
			return true
		}
		off += l
	}
	return false
}

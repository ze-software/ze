// Design: docs/labs/pppoe-interop.md -- independent IPv6 lab receiver.
// Related: product_ipv6_peer.go -- the CPE sends and receives these messages.
package testvpppppoe

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

var productIPv6LAN = netip.MustParsePrefix("2001:db8:20::/64")
var productIPv6PD = netip.MustParsePrefix("2001:db8:100::/56")

type productIPv6Binding struct {
	Address netip.Addr
	Prefix  netip.Prefix
}
type productIPv6WireResult struct {
	Device             string             `json:"device"`
	Local              netip.Addr         `json:"local"`
	Router             netip.Addr         `json:"router"`
	RA                 []byte             `json:"ra"`
	RAHop              int                `json:"ra-hop"`
	Advertise          []byte             `json:"advertise"`
	Reply              []byte             `json:"reply"`
	ClientID           []byte             `json:"client-id"`
	ServerID           []byte             `json:"server-id"`
	WrongServerIgnored bool               `json:"wrong-server-ignored"`
	MalformedIgnored   bool               `json:"malformed-ignored"`
	Binding            productIPv6Binding `json:"binding"`
}

func productIPv6Option(out []byte, code uint16, data []byte) []byte {
	out = binary.BigEndian.AppendUint16(out, code)
	out = binary.BigEndian.AppendUint16(out, uint16(len(data)))
	return append(out, data...)
}

func productIPv6Options(wire []byte) (map[uint16][]byte, error) {
	options := make(map[uint16][]byte)
	for len(wire) > 0 {
		if len(wire) < 4 {
			return nil, errors.New("short DHCPv6 option header")
		}
		code, size := binary.BigEndian.Uint16(wire[:2]), int(binary.BigEndian.Uint16(wire[2:4]))
		wire = wire[4:]
		if size > len(wire) {
			return nil, errors.New("DHCPv6 option exceeds packet")
		}
		if _, exists := options[code]; exists {
			return nil, fmt.Errorf("duplicate DHCPv6 option %d", code)
		}
		options[code] = wire[:size]
		wire = wire[size:]
	}
	if status, ok := options[13]; ok {
		if len(status) < 2 || binary.BigEndian.Uint16(status[:2]) != 0 {
			return nil, errors.New("DHCPv6 refusal status")
		}
	}
	return options, nil
}

func productIPv6Lease(wire []byte, kind byte, xid [3]byte, client, server []byte) (productIPv6Binding, error) {
	var result productIPv6Binding
	if len(wire) < 4 || wire[0] != kind || !bytes.Equal(wire[1:4], xid[:]) {
		return result, errors.New("DHCPv6 reply does not match type/transaction")
	}
	options, err := productIPv6Options(wire[4:])
	if err != nil {
		return result, err
	}
	if !bytes.Equal(options[1], client) || len(client) < 4 {
		return result, errors.New("DHCPv6 client identity mismatch")
	}
	if len(options[2]) < 4 {
		return result, errors.New("DHCPv6 server identity missing")
	}
	if len(server) > 0 && !bytes.Equal(options[2], server) {
		return result, errors.New("DHCPv6 server identity mismatch")
	}
	associations := []struct {
		code, child uint16
		iaid        uint32
	}{{3, 5, 1}, {25, 26, 2}}
	for _, association := range associations {
		value := options[association.code]
		if len(value) < 12 || binary.BigEndian.Uint32(value[:4]) != association.iaid {
			return result, fmt.Errorf("missing or wrong IAID for %d", association.code)
		}
		t1, t2 := binary.BigEndian.Uint32(value[4:8]), binary.BigEndian.Uint32(value[8:12])
		if t1 == 0 || t2 < t1 {
			return result, errors.New("DHCPv6 renewal lifetimes missing/inverted")
		}
		children, err := productIPv6Options(value[12:])
		if err != nil {
			return result, err
		}
		child := children[association.child]
		if association.code == 3 {
			if len(child) != 24 {
				return result, errors.New("IA_NA has no complete IAADDR")
			}
			a, ok := netip.AddrFromSlice(child[:16])
			if !ok || !productIPv6LAN.Contains(a) || a.Compare(netip.MustParseAddr("2001:db8:20::2")) < 0 || a.Compare(netip.MustParseAddr("2001:db8:20::102")) > 0 {
				return result, errors.New("IA_NA address outside configured pool")
			}
			preferred, valid := binary.BigEndian.Uint32(child[16:20]), binary.BigEndian.Uint32(child[20:24])
			if preferred == 0 || valid < preferred || t2 > valid {
				return result, errors.New("IA_NA address lifetimes invalid")
			}
			result.Address = a
		} else {
			if len(child) != 25 {
				return result, errors.New("IA_PD has no complete IAPREFIX")
			}
			preferred, valid := binary.BigEndian.Uint32(child[:4]), binary.BigEndian.Uint32(child[4:8])
			a, ok := netip.AddrFromSlice(child[9:25])
			if !ok || child[8] != 64 || !productIPv6PD.Contains(a) {
				return result, errors.New("IA_PD prefix outside configured delegation")
			}
			p := netip.PrefixFrom(a, 64)
			if p != p.Masked() || preferred == 0 || valid < preferred || t2 > valid {
				return result, errors.New("IA_PD prefix/lifetimes invalid")
			}
			result.Prefix = p
		}
	}
	return result, nil
}

func productIPv6RA(wire []byte, source netip.Addr, hop int) error {
	if !source.IsLinkLocalUnicast() || hop != 255 {
		return errors.New("RA lacks link-local source/hop255")
	}
	if len(wire) < 16 || wire[0] != 134 || wire[1] != 0 || wire[5]&0xc0 != 0xc0 || binary.BigEndian.Uint16(wire[6:8]) == 0 {
		return errors.New("RA lacks live managed router")
	}
	pios := 0
	for options := wire[16:]; len(options) > 0; {
		if len(options) < 2 || options[1] == 0 || int(options[1])*8 > len(options) {
			return errors.New("malformed RA option")
		}
		size := int(options[1]) * 8
		if options[0] == 3 {
			if size != 32 || options[2] != 64 || options[3]&0xc0 != 0x80 {
				return errors.New("RA PIO has invalid length/managed-prefix flags")
			}
			a, ok := netip.AddrFromSlice(options[16:32])
			if !ok || netip.PrefixFrom(a, 64) != productIPv6LAN {
				return errors.New("RA PIO differs from configured prefix")
			}
			valid, preferred := binary.BigEndian.Uint32(options[4:8]), binary.BigEndian.Uint32(options[8:12])
			if preferred == 0 || valid < preferred {
				return errors.New("RA PIO lifetimes invalid")
			}
			pios++
		}
		options = options[size:]
	}
	if pios != 1 {
		return errors.New("RA has no unique configured PIO")
	}
	return nil
}

func productIPv6NoTAPAddress(output, device string) error {
	var links []struct {
		Name      string            `json:"ifname"`
		Addresses []json.RawMessage `json:"addr_info"`
	}
	if err := json.Unmarshal([]byte(output), &links); err != nil {
		return err
	}
	if len(links) != 1 || links[0].Name != device || len(links[0].Addresses) != 0 {
		return fmt.Errorf("the Linux control TAP has IPv6 state or is absent: %s", output)
	}
	return nil
}

// This is deliberately separate from the IPv4 oracle. It checks the exact
// ze-pppoe source path, family and negotiated next hop, never a forwarding DPO.
func productIPv6Route(output string, prefix netip.Prefix, name string, hop netip.Addr) error {
	entries, sources, paths, matches := 0, 0, 0, 0
	inEntry, inSource, needHop := false, false, false
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if before, _, ok := strings.Cut(line, " fib:"); ok {
			inEntry = strings.TrimSpace(before) == prefix.String()
			inSource, needHop = false, false
			if inEntry {
				entries++
			}
			continue
		}
		if !inEntry {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "forwarding:") {
			inSource, needHop = false, false
			continue
		}
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") {
			inSource, needHop = false, false
			if len(fields) >= 2 && strings.HasPrefix(fields[1], "refs:") {
				inSource = fields[0] == "ze-pppoe"
				if inSource {
					sources++
				}
			}
			continue
		}
		if !inSource {
			continue
		}
		if strings.HasPrefix(line, "      path:[") {
			paths++
			if !productField(line, "ip6") || !productField(line, "weight=1") || !productField(line, "pref=0") || !productField(line, "attached-nexthop:") {
				return errors.New("owned IPv6 route is not the expected next-hop path")
			}
			needHop = true
			continue
		}
		if needHop {
			needHop = false
			if !strings.HasPrefix(line, "        ") || len(fields) != 2 {
				return errors.New("owned IPv6 next-hop tuple incomplete")
			}
			actual, err := netip.ParseAddr(fields[0])
			if err != nil {
				return err
			}
			if actual == hop && fields[1] == name {
				matches++
			}
		}
	}
	if entries != 1 || sources != 1 || paths != 1 || matches != 1 || needHop {
		return fmt.Errorf("IPv6 owned route mismatch entries=%d sources=%d paths=%d matches=%d pending=%t", entries, sources, paths, matches, needHop)
	}
	return nil
}

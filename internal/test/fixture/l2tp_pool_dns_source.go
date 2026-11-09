// Design: docs/features/dns-resolver.md -- the subscriber pool's dns-source list and the RADIUS-supplied resolvers
// Overview: register_l2tp_pool_dns_source.go -- the drivers registered from these functions
// Related: tunnel_fixture_l2tp_ppp.go -- tunnelL2TPOpenProxySession, the session this subscriber opens

package fixture

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// IPCP option types for the resolvers (RFC 1877 Sections 1.1 and 1.3).
const (
	ipcpOptionPrimaryDNS   = 129
	ipcpOptionSecondaryDNS = 131
)

// poolDNSUnset is the argument that says the LNS MUST offer no resolver in
// that slot: it then rejects the option rather than NAKing it.
const poolDNSUnset = "-"

// poolDNSNone names an empty slot in a message.
const poolDNSNone = "none"

// msVendorID is the Microsoft SMI network management private enterprise code
// (RFC 2548 Section 2).
const msVendorID = 311

// RADIUS Microsoft vendor types for the subscriber's resolvers (RFC 2548
// Sections 2.7.6 and 2.7.7).
const (
	msPrimaryDNSServer   = 28
	msSecondaryDNSServer = 29
)

// poolDNSRadiusAttrs is the Access-Accept body the MS DNS mock answers: one
// Vendor-Specific attribute per resolver.
//
// RFC 2548 Section 2.7.6: "The MS-Primary-DNS-Server Attribute is used to indicate
// the address of the primary Domain Name Server (DNS) [16, 17] server to be used by
// the PPP peer."
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  Type (26)    |  Length (12)  |      Vendor-Id (311) ...
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  ... Vendor-Id|  Vendor-Type  | Vendor-Length |  Address ...
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  ... Address (4 octets)       |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
func poolDNSRadiusAttrs(primary, secondary netip.Addr) []byte {
	return append(poolDNSRadiusVSA(msPrimaryDNSServer, primary), poolDNSRadiusVSA(msSecondaryDNSServer, secondary)...)
}

// poolDNSRadiusVSA is one Microsoft Vendor-Specific attribute holding addr.
func poolDNSRadiusVSA(vendorType byte, addr netip.Addr) []byte {
	value := make([]byte, 10)
	binary.BigEndian.PutUint32(value[0:4], msVendorID)
	value[4], value[5] = vendorType, 6
	address := addr.As4()
	copy(value[6:10], address[:])
	return tunnelRadiusAttr(26, value)
}

// poolDNSWant is what the subscriber expects the LNS to offer in each slot.
// A zero Addr means the LNS MUST reject that option.
type poolDNSWant struct {
	primary   netip.Addr
	secondary netip.Addr
}

// parsePoolDNSWant reads the driver's arguments: the LNS port, then the
// expected primary and secondary resolver, each an address or "-".
func parsePoolDNSWant(args []string) (int, poolDNSWant, error) {
	port, err := tunnelArgPort(args, 0)
	if err != nil {
		return 0, poolDNSWant{}, err
	}
	if len(args) != 3 {
		return 0, poolDNSWant{}, errors.New("usage: l2tp/pool-dns-source <lns-port> <primary|-> <secondary|->")
	}
	var want poolDNSWant
	slots := []*netip.Addr{&want.primary, &want.secondary}
	for i, slot := range slots {
		if args[1+i] == poolDNSUnset {
			continue
		}
		addr, err := netip.ParseAddr(args[1+i])
		if err != nil {
			return 0, poolDNSWant{}, fmt.Errorf("expected resolver %q: %w", args[1+i], err)
		}
		*slot = addr
	}
	return port, want, nil
}

// poolDNSSubscriber plays the subscriber of spec-home-cpe-23b AC-9 and AC-10
// through the LNS: it opens a proxy-authenticated session, answers CHAP and
// LCP, and asks IPCP for an address and both resolvers with 0.0.0.0, the
// invalid address RFC 1877 has it request. The LNS's Configure-Nak carries
// the pool's selection; the subscriber adopts it, and once its request is
// acknowledged compares what it holds with want.
func poolDNSSubscriber(ctx context.Context, args []string) error {
	port, want, err := parsePoolDNSWant(args)
	if err != nil {
		return err
	}
	if err := poolDNSNegotiate(ctx, port, want); err != nil {
		return err
	}
	return poolDNSVerdict(want).StdOut()
}

// poolDNSVerdict is the line a passing subscriber prints. A driver run as a
// plugin prints it on stderr, because its stdout carries the plugin protocol.
func poolDNSVerdict(offered poolDNSWant) *textbuf.Buffer {
	var tb textbuf.Buffer
	tb.Str("OK: IPCP offered primary DNS ").Str(poolDNSSlot(offered.primary)).
		Str(" and secondary DNS ").Str(poolDNSSlot(offered.secondary)).Str("\n")
	return &tb
}

// poolDNSNegotiate opens one session on the LNS at port and checks the
// resolvers it offers against want. It prints nothing, so the received-lease
// scenario, a plugin, calls it too.
func poolDNSNegotiate(ctx context.Context, port int, want poolDNSWant) error {
	peer, err := tunnelL2TPOpenProxySession(ctx, port, 1500, 1500)
	if err != nil {
		return err
	}
	defer peer.conn.Close() //nolint:errcheck // fixture teardown

	request := poolDNSRequest{
		address:   netip.IPv4Unspecified(),
		primary:   netip.IPv4Unspecified(),
		secondary: netip.IPv4Unspecified(),
		asks:      [2]bool{true, true},
	}
	offered, err := request.negotiate(ctx, peer)
	if err != nil {
		return err
	}
	if offered.primary != want.primary {
		return fmt.Errorf("IPCP offered primary DNS %s, want %s", poolDNSSlot(offered.primary), poolDNSSlot(want.primary))
	}
	if offered.secondary != want.secondary {
		return fmt.Errorf("IPCP offered secondary DNS %s, want %s", poolDNSSlot(offered.secondary), poolDNSSlot(want.secondary))
	}
	return nil
}

// poolDNSSlot names a slot's value for a message: the address, or "none".
func poolDNSSlot(addr netip.Addr) string {
	if !addr.IsValid() {
		return poolDNSNone
	}
	return addr.String()
}

// poolDNSRequest is the subscriber's own IPCP Configure-Request. asks says
// which resolver options it still carries: a rejected option leaves it.
type poolDNSRequest struct {
	address   netip.Addr
	primary   netip.Addr
	secondary netip.Addr
	asks      [2]bool
	id        byte
	acked     bool
	zeAcked   bool
}

// negotiate runs IPCP to Opened in both directions and answers the resolvers
// the LNS acknowledged, a rejected option answering no resolver.
func (r *poolDNSRequest) negotiate(ctx context.Context, peer *tunnelAccountingPeer) (poolDNSWant, error) {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if r.acked && r.zeAcked {
			return r.offered(), nil
		}
		packet, err := peer.readPacket(250 * time.Millisecond)
		if err != nil {
			if ctx.Err() != nil {
				return poolDNSWant{}, ctx.Err()
			}
			continue
		}
		protocol, payload, ok := tunnelL2TPPPPData(packet)
		if !ok {
			if err := peer.handleAccountingPacket(packet); err != nil {
				return poolDNSWant{}, err
			}
			continue
		}
		if protocol != 0x8021 {
			if err := peer.handleAccountingPacket(packet); err != nil {
				return poolDNSWant{}, err
			}
			continue
		}
		if err := r.handleIPCP(peer, payload); err != nil {
			return poolDNSWant{}, err
		}
	}
	return poolDNSWant{}, errors.New("IPCP did not open within 30s")
}

// offered is what the acknowledged request holds.
func (r *poolDNSRequest) offered() poolDNSWant {
	var offered poolDNSWant
	if r.asks[0] {
		offered.primary = r.primary
	}
	if r.asks[1] {
		offered.secondary = r.secondary
	}
	return offered
}

// handleIPCP answers one IPCP packet from the LNS.
func (r *poolDNSRequest) handleIPCP(peer *tunnelAccountingPeer, payload []byte) error {
	if len(payload) < 4 {
		return nil
	}
	code, identifier := payload[0], payload[1]
	length := int(binary.BigEndian.Uint16(payload[2:4]))
	if length < 4 {
		return nil
	}
	if length > len(payload) {
		return nil
	}
	body := payload[4:length]
	switch code {
	case 1:
		// The LNS's own request names its address: acknowledge it, then send ours.
		peer.sendPPP(0x8021, tunnelPPPControl(2, identifier, body))
		if !r.zeAcked {
			r.zeAcked = true
			r.send(peer)
		}
	case 2:
		if identifier == r.id {
			r.acked = true
		}
	case 3:
		if identifier != r.id {
			return nil
		}
		if err := r.adopt(body); err != nil {
			return err
		}
		r.send(peer)
	case 4:
		if identifier != r.id {
			return nil
		}
		if err := r.drop(body); err != nil {
			return err
		}
		r.send(peer)
	}
	return nil
}

// adopt takes the values a Configure-Nak suggests.
//
// RFC 1877 Section 1.1: "If local peer requests an invalid
// server address (which it will typically do intentionally) the
// remote peer specifies the address by NAKing this option, and
// returning the IP address of a valid DNS server."
func (r *poolDNSRequest) adopt(options []byte) error {
	return poolDNSOptions(options, func(kind byte, addr netip.Addr) error {
		switch kind {
		case 3:
			r.address = addr
		case ipcpOptionPrimaryDNS:
			r.primary = addr
		case ipcpOptionSecondaryDNS:
			r.secondary = addr
		}
		return nil
	})
}

// drop removes the options a Configure-Reject names. The LNS rejects a
// resolver option when its selection fills no such slot.
func (r *poolDNSRequest) drop(options []byte) error {
	return poolDNSOptions(options, func(kind byte, _ netip.Addr) error {
		switch kind {
		case 3:
			return errors.New("ze rejected the IPCP IP-Address option")
		case ipcpOptionPrimaryDNS:
			r.asks[0] = false
		case ipcpOptionSecondaryDNS:
			r.asks[1] = false
		}
		return nil
	})
}

// send transmits the current request with a new identifier.
func (r *poolDNSRequest) send(peer *tunnelAccountingPeer) {
	r.id++
	options := poolDNSOption(nil, 3, r.address)
	if r.asks[0] {
		options = poolDNSOption(options, ipcpOptionPrimaryDNS, r.primary)
	}
	if r.asks[1] {
		options = poolDNSOption(options, ipcpOptionSecondaryDNS, r.secondary)
	}
	peer.sendPPP(0x8021, tunnelPPPControl(1, r.id, options))
}

// poolDNSOption appends one address option (type, length 6, address).
func poolDNSOption(options []byte, kind byte, addr netip.Addr) []byte {
	address := addr.As4()
	return append(append(options, kind, 6), address[:]...)
}

// poolDNSOptions walks the address options of an IPCP body; a malformed
// option ends the walk with an error, because the LNS sent it.
func poolDNSOptions(options []byte, visit func(byte, netip.Addr) error) error {
	for offset := 0; offset < len(options); {
		if offset+2 > len(options) {
			return errors.New("IPCP option header truncated")
		}
		length := int(options[offset+1])
		if length < 2 {
			return fmt.Errorf("IPCP option %d has length %d, shorter than its own header", options[offset], length)
		}
		if offset+length > len(options) {
			return fmt.Errorf("IPCP option %d has length %d past the packet", options[offset], length)
		}
		if length == 6 {
			addr := netip.AddrFrom4([4]byte(options[offset+2 : offset+6]))
			if err := visit(options[offset], addr); err != nil {
				return err
			}
		}
		offset += length
	}
	return nil
}

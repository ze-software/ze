//go:build linux

// Design: docs/research/l2tpv2-ze-integration.md -- carrier fixture over the same real UDP sockets

package ppp

import (
	"context"
	"errors"
	"net"
	"net/netip"

	"golang.org/x/net/ipv6"

	"github.com/ze-software/ze/internal/core/ndp"
)

// rsPacketReader preserves the raw socket's read result for error-pacing
// tests. Those tests deliberately recover with a one-byte notification; they
// assert retry timing and cancellation, not solicitation validity.
type rsPacketReader struct{ conn *ipv6.PacketConn }

func (r rsPacketReader) ReadRS(context.Context) error {
	var payload [256]byte
	_, _, _, err := r.conn.ReadFrom(payload[:])
	return err
}

// solicitationPacketReader keeps the RFC test's actual UDP6 body/source and
// subjects each body to the same validator the production kernel carrier uses.
type solicitationPacketReader struct{ conn *ipv6.PacketConn }

func (r solicitationPacketReader) ReadRS(context.Context) error {
	var payload [1500]byte
	n, _, address, err := r.conn.ReadFrom(payload[:])
	if err != nil {
		return err
	}
	peer, ok := address.(*net.UDPAddr)
	if !ok {
		return errors.New("test solicitation socket returned a non-UDP peer")
	}
	// UDP supplies the original test bodies and source. This body-admission
	// fixture supplies a valid hop limit; the tagged claims cover Reserved
	// and options, never the IP hop-limit or checksum receive boundary.
	if !ValidRouterSolicitation(payload[:n], peer.AddrPort().Addr(), ndp.MessageHopLimit) {
		return errors.New("production solicitation validator refused the received body")
	}
	return nil
}

// dhcpPacketReader likewise preserves deadline failures and actual packet I/O
// for the existing pacing proof; it provides no synthetic success or retry.
type dhcpPacketReader struct{ conn net.PacketConn }

func (r dhcpPacketReader) ReadDHCPv6(_ context.Context, payload []byte) (int, netip.AddrPort, error) {
	n, address, err := r.conn.ReadFrom(payload)
	if err != nil {
		return n, netip.AddrPort{}, err
	}
	peer, ok := address.(*net.UDPAddr)
	if !ok {
		return 0, netip.AddrPort{}, errors.New("test DHCP socket returned a non-UDP address")
	}
	return n, peer.AddrPort(), nil
}

func (r dhcpPacketReader) WriteDHCPv6(payload []byte, peer netip.AddrPort) (int, error) {
	return r.conn.WriteTo(payload, net.UDPAddrFromAddrPort(peer))
}

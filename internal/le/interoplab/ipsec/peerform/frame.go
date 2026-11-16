// Design: docs/architecture/testing/interop.md -- independent peer-egress wire evidence.
// Related: frame_test.go -- portable packet boundaries; socket_linux.go -- AF_PACKET metadata.
package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/core/probe"
)

var errUnrelatedPacket = errors.New("packet is not the selected SA's outgoing ESP")

// capturedPacket is an AF_PACKET SOCK_DGRAM observation, so data starts at the IP header.
// outgoing is true only for sockaddr_ll's PACKET_OUTGOING, never a guessed IP direction.
type capturedPacket struct {
	data     []byte
	outgoing bool
}

type wireObservation struct {
	key      stateKey
	form     espForm
	sequence uint64
	replied  bool
}

func parsePacket(packet capturedPacket, wanted stateKey) (wireObservation, error) {
	if !packet.outgoing {
		return wireObservation{}, errUnrelatedPacket
	}
	source, target, protocol, payload, err := ipv4Packet(packet.data)
	if err != nil {
		return wireObservation{}, err
	}
	if source != wanted.source {
		return wireObservation{}, errUnrelatedPacket
	}
	if target != wanted.target {
		return wireObservation{}, errUnrelatedPacket
	}
	form := formBare
	switch protocol {
	case 50:
	case 17:
		if len(payload) < 16 {
			return wireObservation{}, errors.New("short UDP-encapsulated ESP packet")
		}
		length := int(binary.BigEndian.Uint16(payload[4:6]))
		if length < 16 {
			return wireObservation{}, errUnrelatedPacket
		}
		if length > len(payload) {
			return wireObservation{}, errors.New("UDP length exceeds captured packet")
		}
		payload = payload[8:length]
		form = formUDP
	default:
		return wireObservation{}, errUnrelatedPacket
	}
	if len(payload) < 8 {
		return wireObservation{}, errors.New("short ESP header")
	}
	spi := binary.BigEndian.Uint32(payload[:4])
	if spi != wanted.spi {
		return wireObservation{}, errUnrelatedPacket
	}
	sequence := binary.BigEndian.Uint32(payload[4:8])
	if sequence == 0 {
		return wireObservation{}, errors.New("ESP sequence is zero")
	}
	return wireObservation{key: wanted, form: form, sequence: uint64(sequence)}, nil
}

// echoPayload binds one useful probe to this helper, without carrying a secret.
const echoPayload = "ze-ipsec-peer-form"

func parseEchoReply(packet []byte, request command) error {
	source, target, protocol, payload, err := ipv4Packet(packet)
	if err != nil {
		return err
	}
	if source != request.key.target {
		return errUnrelatedPacket
	}
	if target != request.key.source {
		return errUnrelatedPacket
	}
	if protocol != 1 {
		return errUnrelatedPacket
	}
	if len(payload) < 8 {
		return errors.New("short ICMP echo header")
	}
	_, replyType := probe.EchoTypes(request.key.target)
	id, sequence, ok := probe.ParseEchoReply(payload, replyType)
	if !ok {
		return errUnrelatedPacket
	}
	if payload[1] != 0 {
		return errUnrelatedPacket
	}
	if id != request.echoID {
		return errUnrelatedPacket
	}
	if sequence != request.echoSequence {
		return errUnrelatedPacket
	}
	if !bytes.Equal(payload[8:], []byte(echoPayload)) {
		return errUnrelatedPacket
	}
	return nil
}

func ipv4Packet(packet []byte) (netip.Addr, netip.Addr, byte, []byte, error) {
	if len(packet) < 20 {
		return netip.Addr{}, netip.Addr{}, 0, nil, errors.New("short IPv4 header")
	}
	if packet[0]>>4 != 4 {
		return netip.Addr{}, netip.Addr{}, 0, nil, errors.New("packet is not IPv4")
	}
	header := int(packet[0]&15) * 4
	if header < 20 {
		return netip.Addr{}, netip.Addr{}, 0, nil, errors.New("invalid IPv4 header length")
	}
	if header > len(packet) {
		return netip.Addr{}, netip.Addr{}, 0, nil, errors.New("truncated IPv4 options")
	}
	length := int(binary.BigEndian.Uint16(packet[2:4]))
	if length < header {
		return netip.Addr{}, netip.Addr{}, 0, nil, errors.New("IPv4 total length precedes payload")
	}
	if length > len(packet) {
		return netip.Addr{}, netip.Addr{}, 0, nil, errors.New("truncated IPv4 payload")
	}
	if binary.BigEndian.Uint16(packet[6:8])&0x3fff != 0 {
		return netip.Addr{}, netip.Addr{}, 0, nil, errors.New("fragment is not whole-packet evidence")
	}
	source := netip.AddrFrom4([4]byte(packet[12:16]))
	target := netip.AddrFrom4([4]byte(packet[16:20]))
	return source, target, packet[9], packet[header:length], nil
}

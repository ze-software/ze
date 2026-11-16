//go:build linux

// Design: docs/architecture/testing/interop.md -- independent peer egress and useful reply.
// Related: frame.go -- portable decoders; socket_linux.go -- peer-local atomic state change.
// API: https://man7.org/linux/man-pages/man7/packet.7.html -- SOCK_DGRAM and PACKET_OUTGOING.
package main

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/probe"
)

// probe owns both sockets for one bounded observation. No receive worker outlives this call.
// Packet capture is bound before the marked ICMP send, and only PACKET_OUTGOING on that
// interface supplies form/SPI/sequence. The ICMP socket independently supplies its reply.
func (p *linuxPeer) probe(ctx context.Context, request command) (answer wireObservation, resultErr error) {
	states, err := p.states(ctx)
	if err != nil {
		return answer, err
	}
	before, err := selectState(states, request.key)
	if err != nil {
		return answer, err
	}
	device, err := net.InterfaceByName(request.interfaceName)
	if err != nil {
		return answer, fmt.Errorf("peer capture interface: %w", err)
	}
	// Linux dev_queue_xmit_nit sends PACKET_OUTGOING only to ptype_all taps.
	// Bind ETH_P_ALL, then refuse non-IPv4 metadata before decoding an IP header.
	// https://github.com/torvalds/linux/blob/v7.3/net/core/dev.c#L2424
	packet, err := unix.Socket(unix.AF_PACKET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, int(nl.Swap16(unix.ETH_P_ALL)))
	if err != nil {
		return answer, fmt.Errorf("peer egress packet socket: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, unix.Close(packet)) }()
	if err := unix.Bind(packet, &unix.SockaddrLinklayer{Ifindex: device.Index, Protocol: nl.Swap16(unix.ETH_P_ALL)}); err != nil {
		return answer, err
	}
	echo, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, unix.IPPROTO_ICMP)
	if err != nil {
		return answer, fmt.Errorf("peer echo socket: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, unix.Close(echo)) }()
	if err := unix.SetsockoptString(echo, unix.SOL_SOCKET, unix.SO_BINDTODEVICE, request.interfaceName); err != nil {
		return answer, err
	}
	if err := unix.Bind(echo, &unix.SockaddrInet4{Addr: request.key.source.As4()}); err != nil {
		return answer, err
	}
	message := echoRequest(request)
	if err := unix.Sendto(echo, message, 0, &unix.SockaddrInet4{Addr: request.key.target.As4()}); err != nil {
		return answer, err
	}
	answer, err = observeProbe(ctx, packet, echo, device.Index, before.sequence, request)
	if err != nil {
		return answer, err
	}
	states, err = p.states(ctx)
	if err != nil {
		return answer, err
	}
	after, err := selectState(states, request.key)
	if err != nil {
		return answer, err
	}
	if after.reqid != before.reqid {
		return answer, errors.New("peer reqid changed during the wire probe")
	}
	if after.selector != before.selector {
		return answer, errors.New("peer selector changed during the wire probe")
	}
	if after.sequence < answer.sequence {
		return answer, errors.New("wire sequence exceeds the peer's fresh outbound replay sequence")
	}
	return answer, nil
}

func echoRequest(request command) []byte {
	requestType, _ := probe.EchoTypes(request.key.target)
	return probe.BuildICMPEcho(requestType, request.echoID, request.echoSequence, []byte(echoPayload))
}

func observeProbe(ctx context.Context, packet, echo, ifindex int, sequenceBefore uint64, request command) (wireObservation, error) {
	var observed wireObservation
	replied, captured := false, false
	polls := []unix.PollFd{{Fd: int32(packet), Events: unix.POLLIN}, {Fd: int32(echo), Events: unix.POLLIN}}
	// The deadline limits quiet sockets; the observation cap limits a noisy namespace.
	for range 4096 {
		if err := ctx.Err(); err != nil {
			return wireObservation{}, fmt.Errorf("peer probe observed egress=%t reply=%t: %w", captured, replied, err)
		}
		if _, err := unix.Poll(polls, 100); err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return wireObservation{}, err
		}
		for _, poll := range polls {
			if poll.Revents&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 {
				return wireObservation{}, errors.New("peer probe socket stopped while awaiting evidence")
			}
		}
		if polls[0].Revents&unix.POLLIN != 0 {
			candidate, err := captureOutgoing(packet, ifindex, request.key)
			if err != nil {
				if !errors.Is(err, errUnrelatedPacket) {
					return wireObservation{}, err
				}
			} else if candidate.sequence > sequenceBefore {
				if candidate.sequence > observed.sequence {
					observed = candidate
					captured = true
				}
			}
		}
		if polls[1].Revents&unix.POLLIN != 0 {
			if err := receiveEcho(echo, request); err != nil {
				if !errors.Is(err, errUnrelatedPacket) {
					return wireObservation{}, err
				}
			} else {
				replied = true
			}
		}
		if captured {
			if replied {
				observed.replied = true
				return observed, nil
			}
		}
	}
	return wireObservation{}, errors.New("peer probe exceeded its 4096-observation bound")
}

func captureOutgoing(fd, ifindex int, key stateKey) (wireObservation, error) {
	var buffer [65536]byte
	count, from, err := unix.Recvfrom(fd, buffer[:], unix.MSG_DONTWAIT)
	if err != nil {
		if errors.Is(err, unix.EAGAIN) {
			return wireObservation{}, errUnrelatedPacket
		}
		return wireObservation{}, err
	}
	link, ok := from.(*unix.SockaddrLinklayer)
	if !ok {
		return wireObservation{}, errors.New("packet socket supplied no link-layer direction")
	}
	return linkObservation(buffer[:count], link, ifindex, key)
}

func linkObservation(packet []byte, link *unix.SockaddrLinklayer, ifindex int, key stateKey) (wireObservation, error) {
	if link.Ifindex != ifindex {
		return wireObservation{}, errUnrelatedPacket
	}
	if link.Protocol != nl.Swap16(unix.ETH_P_IP) {
		return wireObservation{}, errUnrelatedPacket
	}
	return parsePacket(capturedPacket{data: packet, outgoing: link.Pkttype == unix.PACKET_OUTGOING}, key)
}

func receiveEcho(fd int, request command) error {
	var buffer [65536]byte
	count, _, err := unix.Recvfrom(fd, buffer[:], unix.MSG_DONTWAIT)
	if err != nil {
		if errors.Is(err, unix.EAGAIN) {
			return errUnrelatedPacket
		}
		return err
	}
	return parseEchoReply(buffer[:count], request)
}

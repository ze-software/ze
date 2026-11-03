//go:build linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- synchronous AC expectations.
// Related: script_linux.go -- raw reads remain available to wire assertions.
// RFC 1661 Section 5.8 -- see rfc/short/rfc1661.md.

package pppoeac

import (
	"context"
	"encoding/binary"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// ReadExpectedPacket services Echo while the sole reader waits for another packet.
// Both Magic-Numbers come from observed requests and exact successful Acks.
// Before both exchanges complete, full Echo requests are discarded without reply.
// The caller MUST use raw ReadPacket when Echo itself is asserted.
// No other packet is consumed, including malformed Echo, termination and reentry.
// The original context bounds every read; Echo never refreshes its deadline.
// RFC 1661 Section 5.8: "Upon reception of an Echo-Request in the LCP Opened state, an Echo-Reply MUST be transmitted."
func (s *Session) ReadExpectedPacket(ctx context.Context) (uint16, []byte, error) {
	// The caller's original deadline bounds repeated keepalive requests.
	for {
		if err := ctx.Err(); err != nil {
			return 0, nil, err
		}
		// RFC 2516 Section 6.
		proto, payload, err := s.ReadPacket(ctx)
		if err != nil {
			return 0, nil, err
		}
		if proto != ppp.ProtoLCP {
			return proto, payload, nil
		}
		// RFC 1661 Section 5.8. Offsets: Code 0, ID 1, Length 2..3,
		// Magic 4..7, uninterpreted data 8..Length-1.
		packet, err := ppp.ParseLCPPacket(payload)
		// refuse-by-default: guards ReadExpectedControl -- preserve malformed wire input for the caller's assertion instead of consuming it as Echo.
		if err != nil {
			return proto, payload, nil //nolint:nilerr // Raw observer contract: deliver malformed LCP unchanged for the caller's wire assertion, not as an expectation-reader failure.
		}
		if packet.Code != ppp.LCPEchoRequest {
			return proto, payload, nil
		}
		if len(packet.Data) < 4 {
			return proto, payload, nil
		}
		// RFC 1661 Sections 5.8 and 6.4.
		if err := s.replyEcho(packet); err != nil {
			return 0, nil, err
		}
	}
}

// replyEcho receives an owned, complete Echo-Request from the sole reader.
// The caller MUST check the Code and four-octet Magic field before calling.
// RFC 1661 Section 6.4: "All received Magic-Number fields MUST be equal to either zero or the peer's unique Magic-Number, depending on whether or not the peer negotiated a Magic-Number."
func (s *Session) replyEcho(packet ppp.LCPPacket) error {
	// Pair validation with the send, excluding a concurrent LCP restart.
	s.echo.mu.Lock()
	defer s.echo.mu.Unlock()
	magic, opened, err := s.echo.replyMagic(binary.BigEndian.Uint32(packet.Data[:4]))
	if err != nil {
		return err
	}
	// RFC 1661 Section 5.8: "Echo-Request and Echo-Reply packets received in any state other than the LCP Opened state SHOULD be silently discarded."
	// refuse-by-default: guards ReadExpectedPacket -- outside Opened, successful handling means discard without a reply.
	if !opened {
		return nil
	}
	// RFC 1661 Section 6.4: "If Magic-Number has been successfully negotiated, an implementation MUST transmit these packets with the Magic-Number field set to its negotiated Magic-Number."
	binary.BigEndian.PutUint32(packet.Data[:4], magic)
	// RFC 1661 Section 5.8: "On reception, the Identifier field of the Echo-Request is copied into the Identifier field of the Echo-Reply packet."
	return s.peer.SendControl(s.remote[:], ppp.ProtoLCP, ppp.LCPEchoReply, packet.Identifier, packet.Data)
}

// ReadExpectedControl preserves the raw parser's errors after servicing Echo.
// The caller MUST meet ReadExpectedPacket's epoch and sole-reader obligations.
// RFC 1661 Section 5.8: "Upon reception of an Echo-Request in the LCP Opened state, an Echo-Reply MUST be transmitted."
func (s *Session) ReadExpectedControl(ctx context.Context) (uint16, ppp.LCPPacket, error) {
	// RFC 1661 Section 5.8.
	proto, payload, err := s.ReadExpectedPacket(ctx)
	if err != nil {
		return 0, ppp.LCPPacket{}, err
	}
	// RFC 1661 Section 5.
	packet, err := ppp.ParseLCPPacket(payload)
	return proto, packet, err
}

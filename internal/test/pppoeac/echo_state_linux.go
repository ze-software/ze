//go:build linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- observed AC negotiation.
// Related: script_linux.go -- records actual successful sends and received packets.
// RFC 1661 Sections 5.2 and 6.4 -- see rfc/short/rfc1661.md.

package pppoeac

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// echoState observes both directions under mu. Its zero value is closed LCP.
// The send path MUST hold mu across the write and observation so a concurrent
// received Ack cannot overtake the request's publication. Readers MUST lock too.
type echoState struct {
	mu          sync.Mutex
	local, peer *echoProposal
}

type echoProposal struct {
	id    uint8
	data  []byte
	magic uint32
	err   error
	acked bool
}

// observe records wire facts, never caller-declared negotiation results.
// Its caller MUST hold mu. Invalid wire data remains visible through raw reads.
// RFC 1661 Section 5.2: "The Identifier field of the Configure-Ack MUST match that of the last transmitted Configure-Request."
func (s *echoState) observe(sent bool, packet ppp.LCPPacket) {
	request, reply := &s.peer, &s.local
	if sent {
		request, reply = &s.local, &s.peer
	}
	// This observer tracks negotiation and closure, not other LCP codes.
	//exhaustive:ignore
	switch packet.Code {
	case ppp.LCPConfigureRequest:
		if s.opened() {
			s.local, s.peer = nil, nil
		}
		// RFC 1661 Section 6.4.
		magic, err := echoOptionMagic(packet.Data)
		*request = &echoProposal{id: packet.Identifier, data: bytes.Clone(packet.Data), magic: magic, err: err}
	case ppp.LCPConfigureAck:
		if *reply == nil {
			return
		}
		if packet.Identifier != (*reply).id {
			return
		}
		// RFC 1661 Section 5.2: "The acknowledged Configuration Options MUST NOT be reordered or modified in any way."
		if bytes.Equal(packet.Data, (*reply).data) {
			(*reply).acked = true
		}
	case ppp.LCPConfigureNak, ppp.LCPConfigureReject:
		if *reply != nil {
			if packet.Identifier == (*reply).id {
				(*reply).acked = false
			}
		}
	case ppp.LCPTerminateRequest, ppp.LCPTerminateAck:
		s.local, s.peer = nil, nil
	}
}

// opened requires both exact Configure exchanges. The caller MUST hold mu.
func (s *echoState) opened() bool {
	if s.local == nil {
		return false
	}
	if s.peer == nil {
		return false
	}
	if !s.local.acked {
		return false
	}
	return s.peer.acked
}

// replyMagic validates the incoming value before the caller changes any bytes.
// The caller MUST hold mu through the resulting send so an epoch cannot change.
// RFC 1661 Section 6.4: "All received Magic-Number fields MUST be equal to either zero or the peer's unique Magic-Number, depending on whether or not the peer negotiated a Magic-Number."
func (s *echoState) replyMagic(incoming uint32) (uint32, bool, error) {
	if !s.opened() {
		return 0, false, nil
	}
	if s.local.err != nil {
		return 0, false, s.local.err
	}
	if s.peer.err != nil {
		return 0, false, s.peer.err
	}
	if incoming != s.peer.magic {
		return 0, false, fmt.Errorf("client Echo Magic-Number %08x differs from negotiated %08x", incoming, s.peer.magic)
	}
	return s.local.magic, true, nil
}

// echoOptionMagic reads the request that was actually acknowledged.
// RFC 1661 Section 5.8: "Until the Magic-Number Configuration Option has been successfully negotiated, the Magic-Number MUST be transmitted as zero."
func echoOptionMagic(data []byte) (uint32, error) {
	// RFC 1661 Section 6.
	options, err := ppp.ParseLCPOptions(data)
	if err != nil {
		return 0, err
	}
	var magic uint32
	found := false
	for _, option := range options {
		if option.Type != ppp.LCPOptMagic {
			continue
		}
		if found {
			return 0, errors.New("duplicate negotiated Magic-Number")
		}
		if len(option.Data) != 4 {
			return 0, errors.New("invalid negotiated Magic-Number length")
		}
		found = true
		magic = binary.BigEndian.Uint32(option.Data)
		if magic == 0 {
			return 0, errors.New("zero negotiated Magic-Number option")
		}
	}
	return magic, nil
}

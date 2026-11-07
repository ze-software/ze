//go:build linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- keepalive and redial observation.
// Related: script_linux.go -- the established session these steps run on.
// RFC 2516 Section 5.5 and RFC 1661 Section 5.8 -- see rfc/short/rfc2516.md, rfc/short/rfc1661.md.

package pppoeac

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
)

// DiscoveryStarted answers when the first PADI of the discovery that
// established this session arrived at the access concentrator.
func (s *Session) DiscoveryStarted() (time.Time, error) {
	if s == nil {
		return time.Time{}, errors.New("script session is nil")
	}
	if s.padiAt.IsZero() {
		return time.Time{}, errors.New("script session saw no PADI")
	}
	return s.padiAt, nil
}

// SendPADT ends the session from the access concentrator's side.
// RFC 2516 Section 5.5: "It may be sent by either the Host or the Access Concentrator."
func (s *Session) SendPADT() error {
	if s == nil {
		return errors.New("script session is nil")
	}
	var outgoing [pppoe.EthMaxLen]byte
	// RFC 2516 Section 5.5: "the SESSION_ID MUST be set to indicate which session is to be terminated."
	frame := pppoe.BuildPADT(outgoing[:], s.peer.mac, s.remote, s.peer.sid, "unit-owner-peer")
	return s.peer.SendFrame(frame)
}

// UnansweredEchoes reads the session and answers nothing. It counts the
// client's LCP Echo-Requests until the client sends PADT for this session,
// then returns the count. The caller MUST remain the sole reader until
// return. The context bounds it.
func (s *Session) UnansweredEchoes(ctx context.Context) (int, error) {
	if s == nil {
		return 0, errors.New("script session is nil")
	}
	var incoming [pppoe.EthMaxLen]byte
	echoes := 0
	// The client's PADT or the caller's deadline ends the loop.
	for {
		n, err := s.peer.receiveFrame(ctx, incoming[:])
		if err != nil {
			return echoes, err
		}
		if !bytes.Equal(incoming[6:12], s.remote[:]) {
			continue
		}
		etherType := binary.BigEndian.Uint16(incoming[12:14])
		if etherType == unix.ETH_P_PPP_DISC {
			// RFC 2516 Section 5.5.
			pkt, err := pppoe.ParseDiscovery(incoming[:n])
			if err != nil {
				return echoes, err
			}
			if pkt.Code != pppoe.CodePADT {
				continue
			}
			if pkt.SID != s.peer.sid {
				return echoes, fmt.Errorf("client PADT names session %d, want %d", pkt.SID, s.peer.sid)
			}
			return echoes, nil
		}
		if etherType != unix.ETH_P_PPP_SES {
			continue
		}
		end := 20 + int(binary.BigEndian.Uint16(incoming[18:20]))
		if end > n {
			return echoes, errors.New("truncated PPPoE session frame")
		}
		// RFC 2516 Section 6.
		proto, payload, _, err := ppp.ParseFrame(incoming[20:end])
		if err != nil {
			return echoes, err
		}
		if proto != ppp.ProtoLCP {
			continue
		}
		packet, err := ppp.ParseLCPPacket(payload)
		if err != nil {
			return echoes, err
		}
		// RFC 1661 Section 5.8.
		if packet.Code == ppp.LCPEchoRequest {
			echoes++
		}
	}
}

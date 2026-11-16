// Design: docs/architecture/bgp/structural-forwarding.md -- useful sections survive final next-hop refusal.
// Related: classic_mixed_next_hop.go -- source and raw policy are separate from this socket oracle.
package fixture

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// Both families are negotiated by actual OPEN bytes; family configuration alone
// cannot license the FlowSpec survivor checked on the recipient sockets.
func classicMixedNextHopConnect(ctx context.Context, address string, source netip.Addr, asn uint16) (net.Conn, error) {
	dialer := net.Dialer{Timeout: time.Second, LocalAddr: net.TCPAddrFromAddrPort(netip.AddrPortFrom(source, 0))}
	var conn net.Conn
	var dialErr error
	if !Poll(ctx, 60, 100*time.Millisecond, func() bool {
		conn, dialErr = dialer.DialContext(ctx, "tcp4", address)
		// refuse-by-default: guards Poll -- a failed dial is not ready; a false poll fails below
		return dialErr == nil
	}) {
		return nil, fmt.Errorf("classic mixed listener %s did not accept %s: %w", address, source, dialErr)
	}
	failed := true
	defer func() {
		if failed {
			_ = conn.Close()
		} //nolint:errcheck // Failed handshake owns this socket.
	}()
	if err := conn.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return nil, err
	}
	identifier := source.As4()
	open := &message.Open{Version: 4, MyAS: asn, HoldTime: 0, BGPIdentifier: binary.BigEndian.Uint32(identifier[:]),
		OptionalParams: []byte{2, 12, 1, 4, 0, 1, 0, 1, 1, 4, 0, 1, 0, 133}}
	for index, packet := range [][]byte{message.PackTo(open, nil), message.PackTo(&message.Keepalive{}, nil)} {
		n, err := conn.Write(packet)
		if err != nil {
			return nil, err
		}
		if n != len(packet) {
			return nil, io.ErrShortWrite
		}
		if index == 0 {
			if err := delayOpenFixtureRead(conn, 1); err != nil {
				return nil, err
			}
			if err := delayOpenFixtureRead(conn, 4); err != nil {
				return nil, err
			}
		}
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	failed = false
	return conn, nil
}

func classicMixedNextHopInitial(conn net.Conn) error {
	var classic, flow bool
	for range 2 { // Exactly the two negotiated families owe their initial marker.
		update, err := atomicAggregatePolicyRead(conn)
		if err != nil {
			return err
		}
		if len(update.NLRI) != 0 {
			return errors.New("classic mixed initial sync announced an unexpected route")
		}
		if len(update.WithdrawnRoutes) != 0 {
			return errors.New("classic mixed initial sync withdrew an unexpected route")
		}
		if len(update.PathAttributes) == 0 {
			if classic {
				return errors.New("classic mixed duplicate initial IPv4 marker")
			}
			classic = true
			continue
		}
		_, _, value, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
		if !found {
			return errors.New("classic mixed initial marker has no MP_UNREACH")
		}
		if !bytes.Equal(value, []byte{0, 1, 133}) {
			return fmt.Errorf("unexpected initial MP marker %x", value)
		}
		if flow {
			return errors.New("classic mixed duplicate initial FlowSpec marker")
		}
		flow = true
	}
	if !classic {
		return errors.New("classic mixed initial IPv4 marker missing")
	}
	if !flow {
		return errors.New("classic mixed initial FlowSpec marker missing")
	}
	return nil
}

type classicMixedNextHopWireState struct{ classic, flow int }

// Every frame before the explicit same-socket EOR fence is inspected. A missing
// survivor cannot pass as silence, and a late forbidden classic cannot hide
// behind the first useful FlowSpec announcement.
func classicMixedNextHopReadPhase(conn net.Conn, phase byte, useful bool) error {
	var state classicMixedNextHopWireState
	for range 8 { // One classic section, one MP section, and a fence need at most3.
		update, err := atomicAggregatePolicyRead(conn)
		if err != nil {
			return err
		}
		if len(update.NLRI)+len(update.WithdrawnRoutes)+len(update.PathAttributes) == 0 {
			return state.complete(phase, useful)
		}
		if err := state.observe(update, phase, useful); err != nil {
			return err
		}
	}
	return errors.New("classic mixed socket exceeded eight frames without its fence")
}

func (state *classicMixedNextHopWireState) observe(update *message.Update, phase byte, useful bool) error {
	if len(update.WithdrawnRoutes) != 0 {
		return fmt.Errorf("unexpected classic withdrawal for this raw-forward phase: %x", update.WithdrawnRoutes)
	}
	if _, _, value, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI); found {
		return fmt.Errorf("useful FlowSpec was withdrawn: %x", value)
	}
	if len(update.NLRI) != 0 {
		if !useful {
			return fmt.Errorf("recipient-owned classic announcement escaped: %x", update.NLRI)
		}
		if phase != 2 {
			return fmt.Errorf("MP-only phase%d acquired classic NLRI: %x", phase, update.NLRI)
		}
		if !bytes.Equal(update.NLRI, classicMixedNextHopPrefix[:]) {
			return fmt.Errorf("classic NLRI changed: %x", update.NLRI)
		}
		_, _, hop, found := attribute.AttrFind(update.PathAttributes, attribute.AttrNextHop)
		if !found {
			return errors.New("useful configured repair omitted NEXT_HOP")
		}
		if !bytes.Equal(hop, []byte{192, 0, 2, 99}) {
			return fmt.Errorf("useful repair NEXT_HOP=%x", hop)
		}
		state.classic++
	}
	if _, _, flow, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI); found {
		wantFlow := []byte{0, 1, 133, 0, 0, 5, 1, 24, 198, 51, phase}
		if !bytes.Equal(flow, wantFlow) {
			return fmt.Errorf("FlowSpec field=%x, want%x", flow, wantFlow)
		}
		state.flow++
	}
	_, _, origin, found := attribute.AttrFind(update.PathAttributes, attribute.AttrOrigin)
	if !found {
		return errors.New("useful announcement omitted ORIGIN")
	}
	if !bytes.Equal(origin, []byte{0}) {
		return fmt.Errorf("useful announcement ORIGIN=%x", origin)
	}
	_, _, path, found := attribute.AttrFind(update.PathAttributes, attribute.AttrASPath)
	if !found {
		return errors.New("useful announcement omitted AS_PATH")
	}
	if len(path) == 0 {
		return errors.New("useful announcement lost the source AS_PATH")
	}
	return nil
}

func (state *classicMixedNextHopWireState) complete(phase byte, useful bool) error {
	wantClassic := 0
	if useful {
		if phase == 2 {
			wantClassic = 1
		}
	}
	if state.classic != wantClassic {
		return fmt.Errorf("classic announcements=%d, want%d", state.classic, wantClassic)
	}
	if state.flow != 1 {
		return fmt.Errorf("FlowSpec announcements=%d, want1", state.flow)
	}
	return nil
}

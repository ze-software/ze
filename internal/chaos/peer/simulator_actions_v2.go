// Design: docs/architecture/chaos-web-dashboard.md — parameterized chaos action execution

package peer

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/ze-software/ze/internal/chaos/engine"
	"github.com/ze-software/ze/internal/chaos/scenario"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// executeClockDrift skews the next keepalive by the drift amount.
// Positive drift = late keepalive, negative = early.
// The drift is applied as a one-shot sleep before sending a keepalive.
func executeClockDrift(ctx context.Context, action engine.ChaosAction, conn net.Conn, p SimProfile, emit func(Event)) {
	holdTime := time.Duration(p.HoldTime) * time.Second
	params, err := engine.ParseClockDriftParams(action.Params, holdTime)
	if err != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("clock-drift: %w", err)})
		return
	}

	if params.Drift > 0 {
		select {
		case <-time.After(params.Drift):
		case <-ctx.Done():
			return
		}
	}
	if writeErr := writeMsg(conn, message.NewKeepalive()); writeErr != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("clock-drift keepalive: %w", writeErr)})
	}
}

// executeRouteBurst announces extra routes in rapid succession.
func executeRouteBurst(ctx context.Context, action engine.ChaosAction, conn net.Conn, p SimProfile, cfg SimulatorConfig, emit func(Event)) {
	params := engine.ParseRouteBurstParams(action.Params)

	routes := scenario.GenerateIPv4Routes(cfg.Seed+uint64(time.Now().UnixNano()), p.Index, params.Count, p.TotalPeers)

	sender := NewSender(SenderConfig{
		ASN:     p.ASN,
		IsIBGP:  p.IsIBGP,
		NextHop: p.RouterID,
	})

	for _, prefix := range routes {
		if ctx.Err() != nil {
			return
		}
		data := sender.BuildRoute(prefix)
		if _, writeErr := conn.Write(data); writeErr != nil {
			emit(Event{Type: EventError, Err: fmt.Errorf("route-burst: %w", writeErr)})
			return
		}
		emit(Event{Type: EventRouteSent, Prefix: prefix, Family: params.Family, BytesSent: int64(len(data)), BGPMessage: data})
	}
}

// executeWithdrawalBurst withdraws a configurable number of routes rapidly.
// If count exceeds available routes, withdraws all.
func executeWithdrawalBurst(action engine.ChaosAction, conn net.Conn, p SimProfile, cfg SimulatorConfig, emit func(Event)) {
	params := engine.ParseWithdrawalBurstParams(action.Params)

	routes := scenario.GenerateIPv4Routes(cfg.Seed, p.Index, p.RouteCount, p.TotalPeers)
	count := min(params.Count, len(routes))
	if count == 0 {
		return
	}

	selected := pickRandomRoutes(routes, count, cfg.Seed, p.Index)
	wdBytes, err := sendWithdrawal(conn, selected)
	if err != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("withdrawal-burst: %w", err)})
		return
	}
	emit(Event{Type: EventWithdrawalSent, Count: len(selected), BytesSent: int64(wdBytes)})
}

// executeRouteFlap withdraws and re-announces the same routes in cycles.
func executeRouteFlap(ctx context.Context, action engine.ChaosAction, conn net.Conn, p SimProfile, cfg SimulatorConfig, emit func(Event)) {
	params := engine.ParseRouteFlapParams(action.Params)

	routes := scenario.GenerateIPv4Routes(cfg.Seed, p.Index, p.RouteCount, p.TotalPeers)
	count := min(params.Count, len(routes))
	if count == 0 {
		return
	}

	selected := pickRandomRoutes(routes, count, cfg.Seed, p.Index)
	sender := NewSender(SenderConfig{
		ASN:     p.ASN,
		IsIBGP:  p.IsIBGP,
		NextHop: p.RouterID,
	})

	for cycle := range params.Cycles {
		if ctx.Err() != nil {
			return
		}

		// Withdraw.
		wdBytes, err := sendWithdrawal(conn, selected)
		if err != nil {
			emit(Event{Type: EventError, Err: fmt.Errorf("route-flap withdraw cycle %d: %w", cycle, err)})
			return
		}
		emit(Event{Type: EventWithdrawalSent, Count: len(selected), BytesSent: int64(wdBytes)})

		// Re-announce.
		for _, prefix := range selected {
			if ctx.Err() != nil {
				return
			}
			data := sender.BuildRoute(prefix)
			if _, writeErr := conn.Write(data); writeErr != nil {
				emit(Event{Type: EventError, Err: fmt.Errorf("route-flap announce cycle %d: %w", cycle, writeErr)})
				return
			}
			emit(Event{Type: EventRouteSent, Prefix: prefix, Family: familyIPv4Unicast, BytesSent: int64(len(data)), BGPMessage: data})
		}

		// Inter-cycle delay.
		if cycle < params.Cycles-1 {
			select {
			case <-time.After(params.Interval):
			case <-ctx.Done():
				return
			}
		}
	}
}

// executeSlowPeer delays all outgoing messages for a duration by holding
// a write lock on the connection via a sleep before each message send.
// Runs in its own goroutine; restores normal speed after duration expires.
func executeSlowPeer(ctx context.Context, action engine.ChaosAction, conn net.Conn, emit func(Event)) {
	params := engine.ParseSlowPeerParams(action.Params)

	tc, ok := conn.(*net.TCPConn)
	if !ok {
		emit(Event{Type: EventError, Err: fmt.Errorf("slow-peer: connection is not TCP")})
		return
	}

	const slowBuf = 256          // small buffer to create write-side backpressure
	const defaultBuf = 64 * 1024 // reasonable restore value

	if err := tc.SetWriteBuffer(slowBuf); err != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("slow-peer set write buffer: %w", err)})
		return
	}

	select {
	case <-time.After(params.Duration):
	case <-ctx.Done():
	}

	if err := tc.SetWriteBuffer(defaultBuf); err != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("slow-peer restore write buffer: %w", err)})
	}
}

// executeZeroWindow sets the TCP receive buffer to the minimum (1 byte)
// to simulate a zero-window condition.
func executeZeroWindow(ctx context.Context, action engine.ChaosAction, conn net.Conn, emit func(Event)) {
	params := engine.ParseZeroWindowParams(action.Params)

	tc, ok := conn.(*net.TCPConn)
	if !ok {
		emit(Event{Type: EventError, Err: fmt.Errorf("zero-window: connection is not TCP")})
		return
	}

	if err := tc.SetReadBuffer(1); err != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("zero-window set read buffer: %w", err)})
		return
	}

	select {
	case <-time.After(params.Duration):
	case <-ctx.Done():
	}

	if err := tc.SetReadBuffer(64 * 1024); err != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("zero-window restore read buffer: %w", err)})
	}
}

// Design: (none -- predates documentation)
// Detail: merge.go -- JSON merge logic for UPDATE + RPKI events.

// Package rpki_decorator provides the bgp-rpki-decorator plugin.
// It subscribes to UPDATE and RPKI events, correlates them via the SDK Union helper,
// and emits merged "update-rpki" events for downstream consumers.
package rpki_decorator

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// unionTimeout is the maximum time to wait for the secondary (rpki) event
// before delivering the primary (update) alone.
const unionTimeout = 2 * time.Second

// eventTypeUpdateRPKI is the event type produced by this decorator.
const eventTypeUpdateRPKI = "update-rpki"

// RunDecorator runs the bgp-rpki-decorator plugin using the SDK RPC protocol.
func RunDecorator(conn net.Conn) int {
	logger().Debug("bgp-rpki-decorator plugin starting")

	p := sdk.NewWithConn("bgp-rpki-decorator", conn)
	defer func() { _ = p.Close() }()

	// Union correlates UPDATE (primary) with RPKI (secondary) events.
	u := sdk.NewUnion("update", "rpki", unionTimeout, func(primary, secondary string) {
		merged := mergeUpdateRPKI(primary, secondary)
		if merged == "" {
			logger().Debug("rpki-decorator: merge returned empty, skipping emit")
			return
		}

		// Extract peer address from the primary event for emit-event routing.
		peerAddr := extractPeerAddress(primary)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := p.EmitEvent(ctx, "bgp", eventTypeUpdateRPKI, "received", peerAddr, merged)
		if err != nil {
			logger().Warn("rpki-decorator: emit event failed", "error", err)
		}
	})
	defer u.Stop()

	p.OnEvent(func(jsonStr string) error {
		eventType, peer, msgID := parseEventMeta(jsonStr)
		if eventType == "" || peer == "" || msgID == 0 {
			logger().Debug("rpki-decorator: skipping unparseable or zero-ID event")
			return nil
		}
		u.OnEvent(eventType, peer, msgID, jsonStr)
		return nil
	})

	p.SetStartupSubscriptions([]string{
		"update direction received",
		"rpki",
	}, nil, "full")

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{})
	if err != nil {
		logger().Error("bgp-rpki-decorator plugin failed", "error", err)
		return 1
	}

	return 0
}

// extractPeerAddress extracts the peer address from a JSON event for emit-event routing.
func extractPeerAddress(jsonStr string) string {
	var envelope struct {
		BGP struct {
			Peer struct {
				Address string `json:"address"`
			} `json:"peer"`
		} `json:"bgp"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &envelope); err != nil {
		return ""
	}
	return envelope.BGP.Peer.Address
}

// parseEventMeta extracts event type, peer address, and message ID from a JSON event string.
// Uses json.Unmarshal for safety. Returns zero values on failure.
func parseEventMeta(jsonStr string) (eventType, peerAddr string, msgID uint64) {
	var envelope struct {
		BGP struct {
			Peer struct {
				Address string `json:"address"`
			} `json:"peer"`
			Message struct {
				ID   uint64 `json:"id"`
				Type string `json:"type"`
			} `json:"message"`
		} `json:"bgp"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &envelope); err != nil {
		return "", "", 0
	}
	return envelope.BGP.Message.Type, envelope.BGP.Peer.Address, envelope.BGP.Message.ID
}

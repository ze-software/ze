// Design: docs/architecture/core-design.md -- route reflector plugin
//
// RFC 4456: BGP Route Reflection.
// Subscribes to UPDATE events and forwards them to all peers via cache-forward.
// The reactor handles the RFC 4456 forwarding rules:
//   - Source exclusion (don't send back to source)
//   - Client/non-client filtering (client->all, non-client->clients only)
//   - ORIGINATOR_ID injection (source peer's BGP Identifier)
//   - CLUSTER_LIST prepend (reflector's cluster-id)
//   - Next-hop rewriting (per destination peer settings)

package rr

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

// setLogger configures the package-level logger for the RR plugin.
func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

const (
	eventUpdate = "update"
	eventState  = "state"
	eventOpen   = "open"

	updateRouteTimeout = 60 * time.Second
	statusDone         = "done"
	statusError        = "error"
)

// peerState tracks a connected peer's state and capabilities.
type peerState struct {
	Address      string
	ASN          uint32
	Up           bool
	Families     map[string]bool
	Capabilities map[string]bool
}

// RouteReflector implements a BGP Route Reflector plugin (RFC 4456).
// Subscribes to UPDATE events and forwards them to all peers via cache-forward.
// The reactor handles RFC 4456 forwarding rules, ORIGINATOR_ID injection,
// CLUSTER_LIST prepend, and next-hop rewriting.
type RouteReflector struct {
	plugin   *sdk.Plugin
	peers    map[string]*peerState
	mu       sync.RWMutex
	stopping atomic.Bool
}

// RunRouteReflector runs the Route Reflector plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func RunRouteReflector(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-rr", conn)
	defer func() { _ = p.Close() }()

	rr := &RouteReflector{
		plugin: p,
		peers:  make(map[string]*peerState),
	}

	// Register structured event handler for DirectBridge delivery (hot path).
	p.OnStructuredEvent(func(events []any) error {
		for _, event := range events {
			se, ok := event.(*rpc.StructuredEvent)
			if !ok {
				continue
			}
			switch se.EventType {
			case eventUpdate:
				if msg, ok := se.RawMessage.(*bgptypes.RawMessage); ok {
					rr.forwardUpdate(msg.MessageID)
				}
			case eventState:
				rr.handleStructuredState(se)
			case eventOpen:
				if msg, ok := se.RawMessage.(*bgptypes.RawMessage); ok {
					rr.handleStructuredOpen(se, msg)
				}
			}
		}
		return nil
	})

	// Register text event handler for fork-mode delivery (fallback).
	p.OnEvent(func(eventStr string) error {
		rr.dispatchText(eventStr)
		return nil
	})

	// Register command handler.
	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, string, error) {
		return rr.handleCommand(command)
	})

	// Subscribe to received-direction only for UPDATE and OPEN events.
	// Same rationale as bgp-rs: subscribing to "both" for UPDATEs creates
	// a circular deadlock (ForwardUpdate -> onMessageSent -> deliver -> block).
	p.SetStartupSubscriptions([]string{
		eventUpdate + " direction received",
		eventState,
		eventOpen + " direction received",
	}, nil, "")

	p.SetEncoding("text")

	ctx := context.Background()
	defer rr.stopping.Store(true)
	err := p.Run(ctx, sdk.Registration{
		CacheConsumer:          true,
		CacheConsumerUnordered: true,
		Commands: []sdk.CommandDecl{
			{Name: "rr status", Description: "Show RR status"},
			{Name: "rr peers", Description: "Show peer states"},
		},
	})

	if err != nil {
		logger().Error("rr plugin failed", "error", err)
		return 1
	}

	return 0
}

// forwardUpdate forwards a cached UPDATE to all peers via cache-forward.
// The reactor handles source exclusion, client/non-client filtering (RFC 4456),
// ORIGINATOR_ID, CLUSTER_LIST, and next-hop rewriting.
func (rr *RouteReflector) forwardUpdate(msgID uint64) {
	if rr.stopping.Load() {
		return
	}
	rr.updateRoute("*", fmt.Sprintf("cache %d forward *", msgID))
}

// updateRoute sends a route update command to matching peers via the engine.
func (rr *RouteReflector) updateRoute(peerSelector, command string) {
	ctx, cancel := context.WithTimeout(context.Background(), updateRouteTimeout)
	defer cancel()

	_, _, err := rr.plugin.UpdateRoute(ctx, peerSelector, command)
	if err != nil { //nolint:gocritic // ifElseChain: switch blocked by block-silent-ignore hook
		if rr.stopping.Load() {
			logger().Debug("update-route failed (shutting down)",
				"peer", peerSelector, "command", command, "error", err)
		} else if isConnectionError(err) {
			logger().Warn("update-route failed (peer disconnected)",
				"peer", peerSelector, "command", command, "error", err)
		} else {
			logger().Error("update-route failed",
				"peer", peerSelector, "command", command, "error", err)
		}
	}
}

// isConnectionError reports whether err indicates the target peer's connection is closed.
func isConnectionError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "use of closed network connection")
}

// handleStructuredState processes state events from DirectBridge.
func (rr *RouteReflector) handleStructuredState(se *rpc.StructuredEvent) {
	if se.PeerAddress == "" {
		return
	}

	rr.mu.Lock()
	if rr.peers[se.PeerAddress] == nil {
		rr.peers[se.PeerAddress] = &peerState{Address: se.PeerAddress}
	}
	rr.peers[se.PeerAddress].Up = (se.State == "up")
	rr.peers[se.PeerAddress].ASN = se.PeerAS
	rr.mu.Unlock()
}

// handleStructuredOpen processes OPEN events from DirectBridge.
// Decodes raw OPEN wire bytes to extract peer capabilities and families.
func (rr *RouteReflector) handleStructuredOpen(se *rpc.StructuredEvent, msg *bgptypes.RawMessage) {
	if se.PeerAddress == "" || msg.RawBytes == nil {
		return
	}

	open, err := message.UnpackOpen(msg.RawBytes)
	if err != nil {
		return
	}

	asn := uint32(open.MyAS)
	if open.ASN4 > 0 {
		asn = open.ASN4
	}

	families := make(map[string]bool)
	capabilities := make(map[string]bool)
	hasMP := false

	offset := 0
	for offset < len(open.OptionalParams) {
		if offset+2 > len(open.OptionalParams) {
			break
		}
		paramType := open.OptionalParams[offset]
		paramLen := int(open.OptionalParams[offset+1])
		offset += 2
		if offset+paramLen > len(open.OptionalParams) {
			break
		}
		if paramType == 2 { // Capability (RFC 3392)
			caps, parseErr := capability.Parse(open.OptionalParams[offset : offset+paramLen])
			if parseErr == nil {
				for _, c := range caps {
					if mp, ok := c.(*capability.Multiprotocol); ok {
						hasMP = true
						capabilities["multiprotocol"] = true
						families[fmt.Sprintf("%s/%s", mp.AFI, mp.SAFI)] = true
					}
					if asn4, ok := c.(*capability.ASN4); ok {
						asn = asn4.ASN
						capabilities["asn4"] = true
					}
				}
			}
		}
		offset += paramLen
	}

	// RFC 4760 Section 1: ipv4/unicast is the implicit default only when
	// the peer sends no Multiprotocol capability.
	if !hasMP {
		families["ipv4/unicast"] = true
	}

	rr.mu.Lock()
	defer rr.mu.Unlock()

	if rr.peers[se.PeerAddress] == nil {
		rr.peers[se.PeerAddress] = &peerState{Address: se.PeerAddress}
	}
	peer := rr.peers[se.PeerAddress]
	peer.ASN = asn
	peer.Families = families
	peer.Capabilities = capabilities
}

// dispatchText routes text-format events to handlers (fork-mode fallback).
func (rr *RouteReflector) dispatchText(text string) {
	if rr.stopping.Load() {
		return
	}

	// Quick parse: "peer <addr> remote as <n> <dir> <type> <id> ..."
	// State:       "peer <addr> remote as <n> state <state>"
	fields := strings.Fields(strings.TrimRight(text, "\n"))
	if len(fields) < 6 {
		return
	}

	// fields[5] is either "state" or the direction token.
	if fields[5] == eventState {
		if len(fields) >= 7 {
			peerAddr := fields[1]
			state := fields[6]

			rr.mu.Lock()
			if rr.peers[peerAddr] == nil {
				rr.peers[peerAddr] = &peerState{Address: peerAddr}
			}
			rr.peers[peerAddr].Up = (state == "up")
			rr.mu.Unlock()
		}
		return
	}

	// Message events: fields[5]=direction [6]=type [7]=id
	if len(fields) < 8 {
		return
	}

	if fields[6] == eventUpdate {
		msgID, err := strconv.ParseUint(fields[7], 10, 64)
		if err != nil {
			return
		}
		rr.forwardUpdate(msgID)
	}
}

// handleCommand processes command requests via SDK execute-command callback.
func (rr *RouteReflector) handleCommand(command string) (string, string, error) {
	switch command {
	case "rr status":
		return statusDone, `{"running":true}`, nil
	case "rr peers":
		return statusDone, rr.peersJSON(), nil
	default: // fail on unknown command
		return statusError, "", fmt.Errorf("unknown command: %s", command)
	}
}

// peersJSON returns peer state as JSON.
func (rr *RouteReflector) peersJSON() string {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	peers := make([]map[string]any, 0, len(rr.peers))
	for _, p := range rr.peers {
		peers = append(peers, map[string]any{
			"address": p.Address,
			"remote":  map[string]any{"as": p.ASN},
			"up":      p.Up,
		})
	}

	data, _ := json.Marshal(map[string]any{"peers": peers})
	return string(data)
}

// Design: docs/architecture/plugin/rib-storage-design.md — Adj-RIB-In raw hex storage
// Detail: rib_commands.go — command handlers (status, show, replay, validation)
// Detail: rib_validation.go — RPKI validation gate (pending routes, timeout, state constants)
//
// Package bgp_adj_rib_in implements an Adj-RIB-In plugin for ze.
// It stores all received routes per source peer as raw hex wire bytes
// (from format=full events) and replays them via "update hex" commands.
//
// RFC 4271 Section 3.2: Adj-RIBs-In stores unprocessed routing information
// advertised to the local BGP speaker by its peers.
package adj_rib_in

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	bgp "codeberg.org/thomas-mangin/ze/internal/component/bgp"
	adjschema "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/adj_rib_in/schema"
	"codeberg.org/thomas-mangin/ze/internal/core/seqmap"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

const (
	statusDone  = "done"
	statusError = "error"
)

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

// setLogger sets the package-level logger.
// Called from register.go closures.
func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// RawRoute stores a route as raw hex wire bytes for efficient replay.
// AttrHex comes from format=full event's raw.attributes (path attrs without MP_REACH/UNREACH).
// NHopHex is the next-hop IP converted to wire hex.
// NLRIHex is the individual NLRI wire bytes in hex.
// Sequence numbers are tracked by the seqmap, not stored in RawRoute.
type RawRoute struct {
	Family          string // Address family (e.g. "ipv4/unicast")
	AttrHex         string // Raw path attributes hex from format=full
	NHopHex         string // Next-hop as wire hex (e.g. "0a000001" for 10.0.0.1)
	NLRIHex         string // Individual NLRI wire bytes hex
	ValidationState uint8  // RPKI validation state (0=NotValidated, 1=Valid, 2=NotFound, 3=Invalid)
}

// AdjRIBInManager implements the Adj-RIB-In plugin.
// Stores received routes as raw hex for fast replay via "update hex" commands.
type AdjRIBInManager struct {
	plugin *sdk.Plugin

	// ribIn stores routes received FROM peers.
	// sourcePeer → seqmap of routeKey → RawRoute
	ribIn map[string]*seqmap.Map[string, *RawRoute]

	// peerUp tracks which peers are currently up.
	peerUp map[string]bool

	// seqCounter is the monotonic sequence counter for incremental replay.
	seqCounter uint64

	// pending stores routes awaiting RPKI validation.
	// Key: "peerAddr|family|prefix". Empty when validation is disabled.
	pending map[string]*PendingRoute

	// validationEnabled is set by "adj-rib-in enable-validation" command.
	// When true, received routes are stored as pending instead of installed.
	validationEnabled bool

	// validationTimeout is the fail-open timeout for pending routes.
	// Zero means use defaultValidationTimeout (30s).
	validationTimeout time.Duration

	mu sync.RWMutex
}

// newSeqMap creates a new seqmap for route storage.
func newSeqMap() *seqmap.Map[string, *RawRoute] {
	return seqmap.New[string, *RawRoute]()
}

// RunAdjRIBInPlugin runs the Adj-RIB-In plugin using the SDK RPC protocol.
func RunAdjRIBInPlugin(conn net.Conn) int {
	logger().Debug("adj-rib-in plugin starting")

	p := sdk.NewWithConn("bgp-adj-rib-in", conn)
	defer func() { _ = p.Close() }()

	r := &AdjRIBInManager{
		plugin:  p,
		ribIn:   make(map[string]*seqmap.Map[string, *RawRoute]),
		peerUp:  make(map[string]bool),
		pending: make(map[string]*PendingRoute),
	}

	p.OnEvent(func(jsonStr string) error {
		event, err := bgp.ParseEvent([]byte(jsonStr))
		if err != nil {
			logger().Warn("parse error", "error", err, "line", jsonStr[:min(100, len(jsonStr))])
			return nil
		}
		r.dispatch(event)
		return nil
	})

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, string, error) {
		return r.handleCommand(command, strings.Join(args, " "))
	})

	// Start the timeout scanner for pending validation routes (fail-open).
	stopCh := make(chan struct{})
	r.startTimeoutScanner(stopCh)
	defer close(stopCh)

	// Subscribe to received events with format=full (includes raw hex bytes).
	p.SetStartupSubscriptions([]string{"update direction received", "state"}, nil, "full")

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: "adj-rib-in status"},
			{Name: "adj-rib-in show"},
			{Name: "adj-rib-in replay"},
			{Name: "adj-rib-in enable-validation"},
			{Name: "adj-rib-in accept-routes"},
			{Name: "adj-rib-in reject-routes"},
			{Name: "adj-rib-in revalidate"},
		},
	})
	if err != nil {
		logger().Error("adj-rib-in plugin failed", "error", err)
		return 1
	}

	return 0
}

// updateRoute sends a route update command to matching peers via the engine.
func (r *AdjRIBInManager) updateRoute(peerSelector, command string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _, err := r.plugin.UpdateRoute(ctx, peerSelector, command)
	if err != nil {
		logger().Warn("update-route failed", "peer", peerSelector, "error", err)
	}
}

// dispatch routes an event to the appropriate handler.
func (r *AdjRIBInManager) dispatch(event *bgp.Event) {
	eventType := event.GetEventType()

	switch eventType {
	case "update":
		r.handleReceived(event)
	case "state":
		r.handleState(event)
	}
}

// handleReceived processes received UPDATE events from peers.
// Stores routes as raw hex from format=full events.
func (r *AdjRIBInManager) handleReceived(event *bgp.Event) {
	peerAddr := event.GetPeerAddress()
	if peerAddr == "" {
		return
	}

	if len(event.FamilyOps) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for family, ops := range event.FamilyOps {
		// Split raw NLRI hex into individual prefixes for simple families.
		// For complex families (VPN, EVPN), splitRawNLRIHex returns nil
		// and the raw blob is used directly (see switch below).
		rawNLRIHex := event.RawNLRI[family]
		var splitHexEntries []string
		if rawNLRIHex != "" {
			splitHexEntries = splitRawNLRIHex(rawNLRIHex, family)
		}

		for _, op := range ops {
			switch op.Action {
			case "add":
				nhopHex := nhopToHex(op.NextHop)

				for i, nlriVal := range op.NLRIs {
					prefix, pathID := bgp.ParseNLRIValue(nlriVal)
					if prefix == "" {
						continue
					}
					routeKey := bgp.RouteKey(family, prefix, pathID)

					// Get individual NLRI hex from the correct source:
					// - Simple families: split raw bytes give per-prefix hex
					// - Complex families: raw blob IS the correct wire format
					//   (contains RD + labels + prefix); prefixToWireHex would
					//   produce wrong bytes (bare prefix without RD/labels)
					// - No raw data: compute from parsed prefix (simple families only)
					var nlriHex string
					switch {
					case i < len(splitHexEntries):
						nlriHex = splitHexEntries[i]
					case rawNLRIHex != "" && !isSimplePrefixFamily(family):
						// Complex family: use entire raw blob (correct wire format).
						// Store only for the first parsed NLRI — the blob covers all.
						if i > 0 {
							continue
						}
						nlriHex = rawNLRIHex
					default: // simple family without raw bytes — compute from parsed prefix
						nlriHex = prefixToWireHex(family, prefix, pathID)
					}

					route := &RawRoute{
						Family:  family,
						AttrHex: event.RawAttributes,
						NHopHex: nhopHex,
						NLRIHex: nlriHex,
					}

					if r.validationEnabled {
						// Store as pending — validator will accept or reject.
						pKey := pendingKey(peerAddr, routeKey)
						r.pending[pKey] = &PendingRoute{
							peerAddr:   peerAddr,
							family:     family,
							prefix:     prefix,
							routeKey:   routeKey,
							route:      route,
							receivedAt: time.Now(),
							state:      ValidationPending,
						}
					} else {
						// No validation — install immediately (zero overhead path).
						if r.ribIn[peerAddr] == nil {
							r.ribIn[peerAddr] = newSeqMap()
						}
						r.seqCounter++
						r.ribIn[peerAddr].Put(routeKey, r.seqCounter, route)
					}
				}

			case "del":
				for _, nlriVal := range op.NLRIs {
					prefix, pathID := bgp.ParseNLRIValue(nlriVal)
					if prefix == "" {
						continue
					}
					routeKey := bgp.RouteKey(family, prefix, pathID)
					// Remove from pending if present.
					r.removePending(peerAddr, routeKey)
					// Remove from installed if present.
					if r.ribIn[peerAddr] != nil {
						r.ribIn[peerAddr].Delete(routeKey)
					}
				}
			}
		}
	}
}

// handleState processes peer state changes.
func (r *AdjRIBInManager) handleState(event *bgp.Event) {
	peerAddr := event.GetPeerAddress()
	state := event.GetPeerState()

	r.mu.Lock()
	defer r.mu.Unlock()

	isUp := state == "up"
	r.peerUp[peerAddr] = isUp

	if !isUp {
		// Peer went down — clear installed and pending routes.
		delete(r.ribIn, peerAddr)
		r.clearPeerPending(peerAddr)
	}
}

// buildReplayCommands builds "update hex" commands for replay to a target peer.
// Returns the commands and the maximum sequence index of replayed routes.
// Uses seqmap.Since for O(log N + K) delta replay instead of O(N) full scan.
func (r *AdjRIBInManager) buildReplayCommands(targetPeer string, fromIndex uint64) ([]string, uint64) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var cmds []string
	var maxSeq uint64

	for sourcePeer, routes := range r.ribIn {
		if sourcePeer == targetPeer {
			continue // Don't replay a peer's own routes back to it.
		}
		routes.Since(fromIndex, func(_ string, seq uint64, rt *RawRoute) bool {
			cmds = append(cmds, formatHexCommand(rt))
			if seq > maxSeq {
				maxSeq = seq
			}
			return true
		})
	}

	return cmds, maxSeq
}

// formatHexCommand builds the "update hex" command string from a RawRoute.
func formatHexCommand(rt *RawRoute) string {
	return fmt.Sprintf("update hex attr set %s nhop set %s nlri %s add %s",
		rt.AttrHex, rt.NHopHex, rt.Family, rt.NLRIHex)
}

// nhopToHex converts a next-hop IP address string to wire hex.
// IPv4: "10.0.0.1" → "0a000001", IPv6: "::1" → 32 hex chars.
func nhopToHex(ipStr string) string {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ""
	}
	if ip4 := ip.To4(); ip4 != nil {
		return hex.EncodeToString(ip4)
	}
	return hex.EncodeToString(ip.To16())
}

// splitRawNLRIHex splits concatenated raw NLRI hex into individual entries.
// Only works for simple prefix families (IPv4/IPv6 unicast/multicast).
// Returns nil for complex families (VPN, EVPN, FlowSpec).
func splitRawNLRIHex(rawHex, family string) []string {
	data, err := hex.DecodeString(rawHex)
	if err != nil || len(data) == 0 {
		return nil
	}

	if !isSimplePrefixFamily(family) {
		return nil
	}

	var result []string
	offset := 0
	for offset < len(data) {
		prefixLen := int(data[offset])
		nlriLen := 1 + (prefixLen+7)/8

		if offset+nlriLen > len(data) {
			break
		}
		result = append(result, hex.EncodeToString(data[offset:offset+nlriLen]))
		offset += nlriLen
	}

	return result
}

// isSimplePrefixFamily returns true for families with simple [prefix-len][prefix-bytes] format.
// Complex families (VPN, EVPN, FlowSpec, etc.) have different NLRI structures.
func isSimplePrefixFamily(family string) bool {
	return family == "ipv4/unicast" || family == "ipv4/multicast" ||
		family == "ipv6/unicast" || family == "ipv6/multicast"
}

// prefixToWireHex converts a text prefix to NLRI wire hex.
// Only correct for simple prefix families (IPv4/IPv6 unicast/multicast).
// Called as fallback when raw NLRI bytes are not available.
func prefixToWireHex(family, prefix string, pathID uint32) string {
	_, ipnet, err := net.ParseCIDR(prefix)
	if err != nil {
		return ""
	}

	prefixLen, _ := ipnet.Mask.Size()
	prefixBytes := (prefixLen + 7) / 8

	var ipBytes net.IP
	if len(family) >= 4 && family[:4] == "ipv4" {
		ipBytes = ipnet.IP.To4()
	} else if len(family) >= 4 && family[:4] == "ipv6" {
		ipBytes = ipnet.IP.To16()
	}

	if ipBytes == nil {
		return ""
	}

	var wire []byte
	if pathID != 0 {
		wire = make([]byte, 4+1+prefixBytes)
		wire[0] = byte(pathID >> 24)
		wire[1] = byte(pathID >> 16)
		wire[2] = byte(pathID >> 8)
		wire[3] = byte(pathID)
		wire[4] = byte(prefixLen)
		copy(wire[5:], ipBytes[:prefixBytes])
	} else {
		wire = make([]byte, 1+prefixBytes)
		wire[0] = byte(prefixLen)
		copy(wire[1:], ipBytes[:prefixBytes])
	}

	return hex.EncodeToString(wire)
}

// getYANG returns the embedded YANG schema.
func getYANG() string {
	return adjschema.ZeAdjRibInYANG
}

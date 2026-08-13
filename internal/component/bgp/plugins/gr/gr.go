// Design: docs/architecture/core-design.md — graceful restart plugin
// RFC: rfc/short/rfc4724.md
// RFC: rfc/short/rfc9494.md
// Detail: gr_state.go — GR state machine (per-peer timers, stale family tracking)
// Detail: gr_llgr.go — LLGR capability decode, config extraction, CLI decode (RFC 9494)
//
// Package gr implements a Graceful Restart plugin for ze (RFC 4724, RFC 9494).
// It receives per-peer GR config (restart-time, long-lived-stale-time) during
// Stage 2, registers GR (code 64) and LLGR (code 71) capabilities per-peer
// during Stage 3, and implements Receiving Speaker procedures during the event loop.
//
// Event subscriptions: open (received), state, eor.
// Inter-plugin coordination: DispatchCommand → bgp-rib retain-routes/release-routes/mark-stale/purge-stale.
package gr

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/plugins/gr/yang"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// grMetrics holds Prometheus metrics for GR state.
type grMetrics struct {
	activePeers  metrics.Gauge      // Peers currently in GR
	staleRoutes  metrics.GaugeVec   // Stale routes pending refresh (peer)
	timerExpired metrics.CounterVec // GR restart timer expirations (peer)
}

// grMetricsPtr stores GR metrics, set by SetMetricsRegistry.
var grMetricsPtr atomic.Pointer[grMetrics]

// SetMetricsRegistry creates GR metrics from the given registry.
func SetMetricsRegistry(reg metrics.Registry) {
	m := &grMetrics{
		activePeers:  reg.Gauge("ze_gr_active_peers", "Peers currently in GR."),
		staleRoutes:  reg.GaugeVec("ze_gr_stale_routes", "Stale routes pending refresh.", []string{"peer"}),
		timerExpired: reg.CounterVec("ze_gr_timer_expired_total", "GR restart timer expirations.", []string{"peer"}),
	}
	grMetricsPtr.Store(m)
}

// loggerPtr is the package-level logger, disabled by default.
// Use SetLogger() to enable logging from CLI --log-level flag.
// Stored as atomic.Pointer to avoid data races when tests start
// multiple in-process plugin instances concurrently.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

// loggerConfigured records whether SetLogger has ever installed a real logger.
//
// It matters because every caller of SetLogger is on the ENGINE path:
// ConfigureEngineLogger and the CLI ConfigLogger, both in register.go. Any code
// that runs when the GR plugin engine does NOT start therefore sees loggerPtr
// still holding the discard logger from init(), and anything it logs is dropped
// with no signal. egressWarnLogger (gr_egress.go) reads this to tell "the engine
// chose this level" apart from "no engine ever ran".
var loggerConfigured atomic.Bool

// SetLogger sets the package-level logger.
// Called by cmd/ze/bgp/plugin_gr.go with slogutil.PluginLogger().
func SetLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
		loggerConfigured.Store(true)
	}
}

// grPlugin holds runtime state for the GR plugin during the event loop.
// Created in RunGRPlugin after the 5-stage handshake completes.
type grPlugin struct {
	sdk *sdk.Plugin

	mu           sync.Mutex
	peerCaps     map[string]*grPeerCap   // peerAddr -> last seen GR capability from OPEN
	peerLLGRCaps map[string]*llgrPeerCap // peerAddr -> last seen LLGR capability from OPEN
	// removedPeers tombstones peers deconfigured via a SessionStateDown with
	// reason rpc.ReasonPeerRemoved, so a racing teardown "down" for the same
	// peer does not re-activate GR. Cleared when the peer re-establishes.
	removedPeers map[string]bool
	state        *grStateManager // RFC 4724 Receiving Speaker state machine
}

// RunGRPlugin runs the GR plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
// It receives per-peer GR config during Stage 2, registers per-peer
// GR capabilities (code 64) during Stage 3, and runs RFC 4724
// Receiving Speaker procedures during the event loop.
func RunGRPlugin(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-gr", conn)
	defer func() { _ = p.Close() }()

	gp := &grPlugin{
		sdk:          p,
		peerCaps:     make(map[string]*grPeerCap),
		peerLLGRCaps: make(map[string]*llgrPeerCap),
		removedPeers: make(map[string]bool),
	}

	// Create state manager with callbacks for GR and LLGR lifecycle events.
	gp.state = newGRStateManager(func(peerAddr string) {
		gp.onTimerExpired(peerAddr)
	})
	// RFC 9494: LLGR callbacks compose generic RIB commands.
	// LLGR_STALE = 0xFFFF0006, NO_LLGR = 0xFFFF0007 (wire hex).
	gp.state.onLLGREnter = func(peerAddr string, fam family.Family, llst uint32) {
		famStr := fam.String()
		// 1. Delete routes with NO_LLGR community
		gp.dispatchCommand("request bgp rib delete-with-community", peerAddr, famStr, "ffff0007")
		// 2. Attach LLGR_STALE community to remaining stale routes
		gp.dispatchCommand("request bgp rib attach-community", peerAddr, famStr, "ffff0006")
		// 3. Raise stale level to depreference threshold
		// Raise stale level to 2 (depreference threshold) via mark-stale
		// with restart-time=0 (no new timer needed, LLST timer handles expiry).
		gp.dispatchCommand("request bgp rib mark-stale", peerAddr, "0", "2")
	}
	gp.state.onLLGREntryDone = func(peerAddr string, families []family.Family) {
		addr, err := netip.ParseAddr(peerAddr)
		if err != nil {
			logger().Error("gr: invalid peer address in LLGR entry done", "peer", peerAddr, "error", err)
			return
		}
		excludeSel := selector.ExcludeAddr(addr).String()
		// RFC 9494: readvertise per-fam (not all-fam) to avoid resending unrelated families.
		for _, fam := range families {
			gp.dispatchCommand("clear bgp rib out", excludeSel, fam.String())
		}
	}
	gp.state.onLLGRFamilyExpired = func(peerAddr string, fam family.Family) {
		gp.dispatchCommand("request bgp rib purge-stale", peerAddr, fam.String())
	}
	gp.state.onLLGRComplete = func(peerAddr string) {
		gp.dispatchCommand("request bgp rib release-routes", peerAddr)
	}

	// OnConfigure callback: parse bgp config, extract per-peer restart-time
	// and long-lived-stale-time, then set capabilities for Stage 3.
	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		var caps []sdk.CapabilityDecl
		for _, section := range sections {
			if section.Root != "bgp" {
				continue
			}
			caps = append(caps, extractGRCapabilities(section.Data)...)
			// RFC 9494: LLGR capability (code 71) declared alongside GR (code 64)
			caps = append(caps, extractLLGRCapabilities(section.Data)...)
		}
		p.SetCapabilities(caps)

		// Initialize LLGR egress filter state with peerLLGRCaps. The map is shared
		// by reference so the filter sees peers as they are learned and released,
		// which is why gp.mu travels with it: the filter reads the map under the
		// same lock every writer below takes. The local AS for iBGP detection is
		// NOT captured here: the reactor hands the effective per-peer local AS via
		// dest.LocalAS on the forward and readvertise rails.
		setEgressState(&egressFilterState{
			mu:           &gp.mu,
			peerLLGRCaps: gp.peerLLGRCaps,
		})

		return nil
	})

	// Subscribe to events needed for Receiving Speaker procedures:
	//   open direction received — capture peer's GR capability from OPEN
	//   state — detect peer up/down (with reason for GR vs normal teardown)
	//   eor — track End-of-RIB per fam for stale route purge
	p.SetStartupSubscriptions(
		[]string{"open direction received", "state", "eor"},
		nil, "full",
	)

	// Structured event handler: dispatch via DirectBridge without JSON parsing.
	// State events use metadata fields directly; OPEN/EOR decode from RawMessage.
	p.OnStructuredEvent(func(events []any) error {
		for _, event := range events {
			se, ok := event.(*rpc.StructuredEvent)
			if !ok || se.PeerAddress == "" {
				continue
			}
			gp.handleStructuredEvent(se)
		}
		return nil
	})

	// Fallback: dispatch JSON events for non-DirectBridge delivery.
	p.OnEvent(func(event string) error {
		return gp.handleEvent(event)
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{"bgp"},
	})
	if err != nil {
		logger().Error("gr plugin failed", "error", err)
		return 1
	}

	return 0
}

// handleStructuredEvent dispatches a StructuredEvent to the appropriate GR handler.
// State events use metadata fields directly (no JSON parsing needed).
// OPEN events decode raw wire bytes via message.UnpackOpen (no format import needed).
// EOR events are delivered as text via OnEvent (onEORReceived uses text-only delivery).
func (gp *grPlugin) handleStructuredEvent(se *rpc.StructuredEvent) {
	switch se.EventType { //nolint:exhaustive // GR only handles state+open
	case rpc.EventKindState:
		gp.handleStructuredState(se.PeerAddress, se.State, se.Reason)
	case rpc.EventKindOpen:
		if msg, ok := se.RawMessage.(*bgptypes.RawMessage); ok {
			gp.handleStructuredOpen(se.PeerAddress, msg)
		} else if se.RawMessage != nil {
			logger().Warn("gr: unexpected RawMessage type for open event",
				"peer", se.PeerAddress,
				"type", fmt.Sprintf("%T", se.RawMessage))
		}
	}
}

// handleStructuredOpen extracts GR (code 64) and LLGR (code 71) capabilities
// from a received OPEN message's raw wire bytes. Same logic as handleOpenEvent
// but reads wire bytes directly instead of JSON.
func (gp *grPlugin) handleStructuredOpen(peerAddr string, msg *bgptypes.RawMessage) {
	if msg.RawBytes == nil {
		logger().Debug("gr: open event with nil raw bytes, skipping", "peer", peerAddr)
		return
	}
	open, err := message.UnpackOpen(msg.RawBytes)
	if err != nil {
		logger().Debug("gr: failed to unpack OPEN", "peer", peerAddr, "err", err)
		return
	}

	var foundGR bool
	// Walk optional parameters looking for type 2 (capabilities).
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
		if paramType == 2 {
			foundGR = gp.extractGRCaps(peerAddr, open.OptionalParams[offset:offset+paramLen], foundGR)
		}
		offset += paramLen
	}

	// RFC 9494: If GR capability is not present, LLGR MUST be ignored.
	if !foundGR {
		gp.mu.Lock()
		delete(gp.peerLLGRCaps, peerAddr)
		gp.mu.Unlock()
	}
}

// extractGRCaps walks raw capability bytes looking for codes 64 (GR) and 71 (LLGR).
// Returns true if GR (code 64) was found (including prior calls via foundGR).
func (gp *grPlugin) extractGRCaps(peerAddr string, data []byte, foundGR bool) bool {
	off := 0
	for off < len(data) {
		if off+2 > len(data) {
			break
		}
		code := data[off]
		capLen := int(data[off+1])
		off += 2
		if off+capLen > len(data) {
			break
		}
		capValue := data[off : off+capLen]
		off += capLen

		switch code {
		case 64: // GR capability
			result, err := decodeGR(capValue)
			if err != nil {
				logger().Debug("gr: failed to decode cap 64", "peer", peerAddr, "err", err)
				continue
			}
			peerCap := grResultToPeerCap(result)
			gp.mu.Lock()
			gp.peerCaps[peerAddr] = peerCap
			gp.mu.Unlock()
			foundGR = true
			logger().Debug("gr: stored peer GR capability",
				"peer", peerAddr,
				"restart-time", peerCap.RestartTime,
				"families", len(peerCap.Families))

		case 71: // LLGR capability
			result, err := decodeLLGR(capValue)
			if err != nil {
				logger().Debug("gr: failed to decode cap 71", "peer", peerAddr, "err", err)
				continue
			}
			peerCap := llgrResultToPeerCap(result)
			gp.mu.Lock()
			gp.peerLLGRCaps[peerAddr] = peerCap
			gp.mu.Unlock()
			logger().Debug("gr: stored peer LLGR capability",
				"peer", peerAddr,
				"families", len(peerCap.Families))
		}
	}
	return foundGR
}

// handleStructuredState processes a structured state event.
func (gp *grPlugin) handleStructuredState(peerAddr string, state rpc.SessionState, reason string) {
	switch state { //nolint:exhaustive // only up/down are actionable for GR
	case rpc.SessionStateDown:
		// Peer deconfigured: release GR state + per-peer metrics, do not retain.
		if reason == rpc.ReasonPeerRemoved {
			gp.onPeerRemoved(peerAddr)
			return
		}
		// A teardown "down" racing a just-processed removal must not re-activate GR.
		if gp.consumeRemovedTombstone(peerAddr) {
			return
		}
		wasNotification := reason == "notification"

		gp.mu.Lock()
		cap := gp.peerCaps[peerAddr]
		llgrCap := gp.peerLLGRCaps[peerAddr]
		gp.mu.Unlock()

		activated := gp.state.onSessionDown(peerAddr, cap, llgrCap, wasNotification)
		if activated {
			gp.dispatchCommand("request bgp rib purge-stale", peerAddr)
			gp.dispatchCommand("request bgp rib retain-routes", peerAddr)
			gp.dispatchCommand("request bgp rib mark-stale", peerAddr, strconv.FormatUint(uint64(cap.RestartTime), 10))
		}

	case rpc.SessionStateUp:
		gp.clearRemovedTombstone(peerAddr) // peer is back; allow future GR

		gp.mu.Lock()
		newCap := gp.peerCaps[peerAddr]
		newLLGRCap := gp.peerLLGRCaps[peerAddr]
		gp.mu.Unlock()

		purged, _ := gp.state.onSessionReestablished(peerAddr, newCap, newLLGRCap)
		for _, fam := range purged {
			gp.dispatchCommand("request bgp rib purge-stale", peerAddr, fam.String())
		}
	}
}

// handleEvent parses a JSON event and dispatches to the appropriate handler.
// Events arrive as ze-bgp JSON: {"type":"bgp","bgp":{...}}.
func (gp *grPlugin) handleEvent(event string) error {
	var envelope map[string]any
	if err := json.Unmarshal([]byte(event), &envelope); err != nil {
		logger().Debug("gr: invalid event JSON", "err", err)
		return nil // Don't fail on unparseable events
	}

	bgpPayload, ok := envelope["bgp"].(map[string]any)
	if !ok {
		return nil
	}

	msgObj, _ := bgpPayload["message"].(map[string]any)
	msgType, _ := msgObj["type"].(string)

	peerObj, _ := bgpPayload["peer"].(map[string]any)
	remoteObj, _ := peerObj["remote"].(map[string]any)
	peerAddr, _ := remoteObj["address"].(string)
	if peerAddr == "" {
		return nil
	}

	switch msgType {
	case "open":
		gp.handleOpenEvent(peerAddr, bgpPayload)
	case "state":
		gp.handleStateEvent(peerAddr, bgpPayload)
	case "eor":
		gp.handleEOREvent(peerAddr, bgpPayload)
	}

	return nil
}

// handleOpenEvent extracts GR (code 64) and LLGR (code 71) capabilities from
// a received OPEN message. Stores both for use when the peer's session drops.
// RFC 9494: LLGR capability MUST be ignored if GR capability is not also present.
func (gp *grPlugin) handleOpenEvent(peerAddr string, payload map[string]any) {
	openObj, ok := payload["open"].(map[string]any)
	if !ok {
		return
	}

	caps, ok := openObj["capabilities"].([]any)
	if !ok {
		return
	}

	var foundGR bool
	// Scan all capabilities for code 64 (GR) and code 71 (LLGR)
	for _, capRaw := range caps {
		capObj, ok := capRaw.(map[string]any)
		if !ok {
			continue
		}
		code, _ := capObj["code"].(float64)
		hexValue, _ := capObj["value"].(string)
		if hexValue == "" {
			continue
		}

		switch int(code) {
		case 64:
			data, err := hex.DecodeString(hexValue)
			if err != nil {
				logger().Debug("gr: invalid cap 64 hex", "peer", peerAddr, "err", err)
				continue
			}
			result, err := decodeGR(data)
			if err != nil {
				logger().Debug("gr: failed to decode cap 64", "peer", peerAddr, "err", err)
				continue
			}
			peerCap := grResultToPeerCap(result)
			gp.mu.Lock()
			gp.peerCaps[peerAddr] = peerCap
			gp.mu.Unlock()
			foundGR = true
			logger().Debug("gr: stored peer GR capability",
				"peer", peerAddr,
				"restart-time", peerCap.RestartTime,
				"families", len(peerCap.Families))

		case 71:
			data, err := hex.DecodeString(hexValue)
			if err != nil {
				logger().Debug("gr: invalid cap 71 hex", "peer", peerAddr, "err", err)
				continue
			}
			result, err := decodeLLGR(data)
			if err != nil {
				logger().Debug("gr: failed to decode cap 71", "peer", peerAddr, "err", err)
				continue
			}
			peerCap := llgrResultToPeerCap(result)
			gp.mu.Lock()
			gp.peerLLGRCaps[peerAddr] = peerCap
			gp.mu.Unlock()
			logger().Debug("gr: stored peer LLGR capability",
				"peer", peerAddr,
				"families", len(peerCap.Families))
		}
	}

	// RFC 9494: If GR capability is not present, LLGR MUST be ignored.
	if !foundGR {
		gp.mu.Lock()
		delete(gp.peerLLGRCaps, peerAddr)
		gp.mu.Unlock()
	}
}

// handleStateEvent processes peer up/down state changes.
// RFC 4724 Section 4.2:
//   - TCP failure for GR-capable peer → 3-step sequence: purge-stale → retain → mark-stale
//   - NOTIFICATION → standard BGP (no retention)
//   - Peer reconnects → validate new GR cap, purge non-forwarding families
func (gp *grPlugin) handleStateEvent(peerAddr string, payload map[string]any) {
	state, _ := payload["state"].(string)

	switch state {
	case "down":
		reason, _ := payload["reason"].(string)
		// Peer deconfigured: release GR state + per-peer metrics, do not retain.
		if reason == rpc.ReasonPeerRemoved {
			gp.onPeerRemoved(peerAddr)
			return
		}
		// A teardown "down" racing a just-processed removal must not re-activate GR.
		if gp.consumeRemovedTombstone(peerAddr) {
			return
		}
		wasNotification := reason == "notification"

		gp.mu.Lock()
		cap := gp.peerCaps[peerAddr]
		llgrCap := gp.peerLLGRCaps[peerAddr]
		gp.mu.Unlock()

		activated := gp.state.onSessionDown(peerAddr, cap, llgrCap, wasNotification)
		if activated {
			// 3-step session-down sequence (RFC 4724 + consecutive restart handling):
			// 1. Purge old stale routes from previous GR cycle (no-op on first disconnect)
			gp.dispatchCommand("request bgp rib purge-stale", peerAddr)
			// 2. Retain routes — prevents bgp-rib from deleting on state=down
			gp.dispatchCommand("request bgp rib retain-routes", peerAddr)
			// 3. Mark remaining routes as stale for new GR cycle
			gp.dispatchCommand("request bgp rib mark-stale", peerAddr, strconv.FormatUint(uint64(cap.RestartTime), 10))
		}

	case "up":
		gp.clearRemovedTombstone(peerAddr) // peer is back; allow future GR

		gp.mu.Lock()
		newCap := gp.peerCaps[peerAddr]
		newLLGRCap := gp.peerLLGRCaps[peerAddr]
		gp.mu.Unlock()

		purged, _ := gp.state.onSessionReestablished(peerAddr, newCap, newLLGRCap)
		for _, fam := range purged {
			// RFC 4724: purge stale routes for families with F-bit=0 or missing
			gp.dispatchCommand("request bgp rib purge-stale", peerAddr, fam.String())
		}
	}
}

// handleEOREvent processes End-of-RIB markers.
// RFC 4724 Section 4.2: On EOR receipt, remove stale routes for that family.
func (gp *grPlugin) handleEOREvent(peerAddr string, payload map[string]any) {
	eorObj, ok := payload["eor"].(map[string]any)
	if !ok {
		return
	}

	famStr, _ := eorObj["family"].(string)
	if famStr == "" {
		return
	}
	fam, ok := family.LookupFamily(famStr)
	if !ok {
		logger().Debug("gr: EOR for unregistered family", "peer", peerAddr, "family", famStr)
		return
	}

	shouldPurge := gp.state.onEORReceived(peerAddr, fam)
	if shouldPurge {
		// RFC 4724: purge only stale routes for this family (selective, not nuclear)
		gp.dispatchCommand("request bgp rib purge-stale", peerAddr, famStr)
		logger().Debug("gr: EOR received, purging stale routes", "peer", peerAddr, "family", famStr)
	}
}

// onTimerExpired is called when a peer's restart timer fires.
// RFC 4724 Section 4.2: delete all stale routes from the peer.
func (gp *grPlugin) onTimerExpired(peerAddr string) {
	if m := grMetricsPtr.Load(); m != nil {
		m.timerExpired.With(peerAddr).Inc()
	}
	gp.releaseRoutes(peerAddr)
}

// releaseRoutes tells bgp-rib to release (delete) retained routes for a peer.
// Also prunes the cached peer capabilities since GR/LLGR is fully complete.
func (gp *grPlugin) releaseRoutes(peerAddr string) {
	gp.dispatchCommand("request bgp rib release-routes", peerAddr)

	gp.mu.Lock()
	delete(gp.peerCaps, peerAddr)
	delete(gp.peerLLGRCaps, peerAddr)
	gp.mu.Unlock()
}

// dispatchCommand sends a command to the engine for inter-plugin coordination.
// Logs errors but does not fail — the GR state machine proceeds regardless.
func (gp *grPlugin) dispatchCommand(command string, args ...string) {
	if gp.sdk == nil {
		return // unit test — no SDK available
	}
	ctx := context.Background()
	status, _, err := gp.sdk.DispatchCommandArgs(ctx, command, args, "")
	if err != nil {
		logger().Warn("gr: dispatch failed", "command", command, "args", args, "err", err)
	} else {
		logger().Debug("gr: dispatch ok", "command", command, "args", args, "status", status)
	}
}

// grResultToPeerCap converts a decoded GR wire result to the state machine's
// capability representation, packing AFI/SAFI numbers into family.Family.
func grResultToPeerCap(r *grResult) *grPeerCap {
	cap := &grPeerCap{
		RestartTime: r.RestartTime,
		Families:    make([]grCapFamily, 0, len(r.Families)),
	}
	for _, f := range r.Families {
		cap.Families = append(cap.Families, grCapFamily{
			Family:       family.Family{AFI: family.AFI(f.AFI), SAFI: family.SAFI(f.SAFI)},
			ForwardState: f.ForwardState,
		})
	}
	return cap
}

// parseGRCapValue extracts a GR capability hex value from a capability map's
// "graceful-restart" entry. Returns "" if no GR config is present.
func parseGRCapValue(capMap map[string]any, peerAddr string) string {
	if capMap == nil {
		return ""
	}
	grData, ok := capMap["graceful-restart"].(map[string]any)
	if !ok {
		return ""
	}

	// Extract restart-time (default 120 per RFC 4724)
	restartTime := uint16(120)
	if rtVal, ok := grData["restart-time"]; ok {
		switch v := rtVal.(type) {
		case float64:
			restartTime = uint16(v)
		case string:
			if parsed, err := strconv.ParseUint(v, 10, 16); err == nil {
				restartTime = uint16(parsed)
			}
		}
	}

	// RFC 4724: restart-time is 12 bits (0-4095)
	if restartTime > 4095 {
		logger().Warn("restart-time exceeds 12-bit max, clamping", "peer", peerAddr, "value", restartTime)
		restartTime = 4095
	}

	// RFC 4724 Section 3: Restart Flags (4 bits) + Restart Time (12 bits)
	return fmt.Sprintf("%04x", restartTime&0x0FFF)
}

// extractGRCapabilities parses bgp config JSON and returns per-peer GR capabilities.
// Handles both standalone peers (bgp.peer) and grouped peers (bgp.group.<name>.peer).
// RFC 4724: Graceful Restart capability code is 64.
func extractGRCapabilities(jsonStr string) []sdk.CapabilityDecl {
	bgpSubtree, ok := configjson.ParseBGPSubtree(jsonStr)
	if !ok {
		logger().Warn("invalid JSON in bgp config")
		return nil
	}

	const grCapCode = 64
	var caps []sdk.CapabilityDecl

	configjson.ForEachPeer(bgpSubtree, func(peerAddr string, peerMap, groupMap map[string]any, origin configjson.PeerOrigin) {
		// Check per-peer graceful-restart capability first.
		peerCapValue := parseGRCapValue(configjson.GetCapability(peerMap), peerAddr)

		// Check group-level graceful-restart capability (fallback).
		var groupCapValue string
		if groupMap != nil {
			groupCapValue = parseGRCapValue(configjson.GetCapability(groupMap), peerAddr)
		}

		// Per-peer wins over group.
		capValue := groupCapValue
		if peerCapValue != "" {
			capValue = peerCapValue
		}
		if capValue == "" {
			return
		}

		caps = append(caps, sdk.CapabilityDecl{
			Code:     grCapCode,
			Encoding: sdk.CapEncodingHex,
			Payload:  capValue,
			Peers:    []string{configjson.CapabilitySelector(peerAddr, origin)},
		})
		logger().Debug("gr capability", "peer", peerAddr)
	})

	return caps
}

// RunCLIDecode decodes hex capability data directly from CLI arguments.
// This is for human use: `ze plugin gr --capa <hex>` or with `--text`.
// Auto-detects GR (code 64) vs LLGR (code 71) based on wire format structure:
//   - GR: 2-byte header + 4-byte tuples -> (len-2) % 4 == 0 and len >= 2
//   - LLGR: 7-byte tuples only -> len % 7 == 0 and len >= 7
//
// Returns exit code (0 = success, 1 = error).
func RunCLIDecode(hexData string, textOutput bool, stdout, stderr io.Writer) int {
	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeOut(stderr, fmt.Sprintf("error: invalid hex: %v\n", err))
		return 1
	}

	n := len(data)
	looksLikeGR := n >= 2 && (n-2)%4 == 0
	looksLikeLLGR := n >= 7 && n%7 == 0

	// If only LLGR structure matches, decode as LLGR directly
	if looksLikeLLGR && !looksLikeGR {
		return runCLIDecodeLLGR(hexData, textOutput, stdout, stderr)
	}

	// If only GR or both structures match, try GR; fall back to LLGR on failure
	exitCode := runCLIDecodeGR(hexData, textOutput, stdout, stderr)
	if exitCode == 0 {
		return 0
	}
	return runCLIDecodeLLGR(hexData, textOutput, stdout, stderr)
}

// runCLIDecodeGR decodes GR capability (code 64) hex from CLI arguments.
func runCLIDecodeGR(hexData string, textOutput bool, stdout, stderr io.Writer) int {
	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeOut(stderr, fmt.Sprintf("error: invalid hex: %v\n", err))
		return 1
	}

	result, err := decodeGR(data)
	if err != nil {
		writeOut(stderr, fmt.Sprintf("error: %v\n", err))
		return 1
	}

	if textOutput {
		writeOut(stdout, formatGRText(result)+"\n")
	} else {
		jsonBytes, jsonErr := json.Marshal(result)
		if jsonErr != nil {
			writeOut(stderr, fmt.Sprintf("error: JSON encoding: %v\n", jsonErr))
			return 1
		}
		writeOut(stdout, string(jsonBytes)+"\n")
	}
	return 0
}

// grFamily represents an AFI/SAFI entry in GR capability.
type grFamily struct {
	AFI          uint16 `json:"afi"`
	SAFI         uint8  `json:"safi"`
	ForwardState bool   `json:"forward-state"`
}

// grResult represents decoded GR capability.
type grResult struct {
	Name         string     `json:"name"`
	RestartFlags uint8      `json:"restart-flags"`
	RestartTime  uint16     `json:"restart-time"`
	Restarting   bool       `json:"restarting"`
	Notification bool       `json:"notification"`
	Families     []grFamily `json:"families,omitempty"`
}

// decodeGR decodes GR capability wire bytes.
// RFC 4724 Section 3: Wire format is:
//   - Restart Flags (4 bits) + Restart Time (12 bits) = 2 bytes
//   - Per-family: AFI (2 bytes) + SAFI (1 byte) + Flags (1 byte)
func decodeGR(data []byte) (*grResult, error) {
	if len(data) < 2 {
		return nil, fmt.Errorf("GR capability too short: need 2 bytes, got %d", len(data))
	}

	// First 2 bytes: flags (4 bits) + restart-time (12 bits)
	flags := (data[0] >> 4) & 0x0F
	restartTime := (uint16(data[0]&0x0F) << 8) | uint16(data[1])

	result := &grResult{
		Name:         "graceful-restart",
		RestartFlags: flags,
		RestartTime:  restartTime,
		Restarting:   (flags & 0x08) != 0, // R-bit (RFC 4724)
		Notification: (flags & 0x04) != 0, // N-bit (RFC 8538)
	}

	// Parse AFI/SAFI tuples (4 bytes each)
	remaining := data[2:]
	for len(remaining) >= 4 {
		afi := (uint16(remaining[0]) << 8) | uint16(remaining[1])
		safi := remaining[2]
		famFlags := remaining[3]

		result.Families = append(result.Families, grFamily{
			AFI:          afi,
			SAFI:         safi,
			ForwardState: (famFlags & 0x80) != 0, // F-bit
		})
		remaining = remaining[4:]
	}

	return result, nil
}

// formatGRText formats GR capability as human-readable text.
func formatGRText(r *grResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%-20s restart-time=%d", "graceful-restart", r.RestartTime) //nolint:errcheck // buffer output
	if r.Restarting {
		sb.WriteString(" restarting")
	}
	if r.Notification {
		sb.WriteString(" notification")
	}
	for _, f := range r.Families {
		fmt.Fprintf(&sb, " afi=%d/safi=%d", f.AFI, f.SAFI) //nolint:errcheck // buffer output
		if f.ForwardState {
			sb.WriteString("(F)")
		}
	}
	return sb.String()
}

// GetYANG returns the embedded YANG for the GR plugin.
func GetYANG() string {
	return yang.ZeGracefulRestartYANG
}

// writeOut writes a string to the output writer, discarding errors.
// Decode mode writes to an in-memory buffer; write failures are not actionable.
func writeOut(w io.Writer, s string) {
	if _, err := io.WriteString(w, s); err != nil {
		logger().Debug("decode write failed", "err", err)
	}
}

// Decode format constants used by RunDecodeMode and helpers.
const (
	decodeFormatJSON = "json"
	decodeFormatText = "text"
)

// RunDecodeMode runs the plugin in decode mode for ze bgp decode.
// Handles capability code 64 (GR, RFC 4724) and code 71 (LLGR, RFC 9494).
func RunDecodeMode(input io.Reader, output io.Writer) int {
	writeUnknown := func() { writeOut(output, "decoded unknown\n") }
	writeJSON := func(j []byte) { writeOut(output, "decoded json "+string(j)+"\n") }
	writeText := func(t string) { writeOut(output, "decoded text "+t+"\n") }

	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 4 || parts[0] != "decode" {
			writeUnknown()
			continue
		}

		format := decodeFormatJSON
		capIdx := 1
		if parts[1] == decodeFormatJSON || parts[1] == decodeFormatText {
			format = parts[1]
			capIdx = 2
			if len(parts) < 5 {
				writeUnknown()
				continue
			}
		}

		if parts[capIdx] != "capability" {
			writeUnknown()
			continue
		}

		capCode := parts[capIdx+1]
		if capCode != "64" && capCode != "71" {
			writeUnknown()
			continue
		}

		hexData := parts[capIdx+2]
		if capCode == "64" {
			decodeGRMode(format, hexData, writeJSON, writeText, writeUnknown)
		} else {
			decodeLLGRMode(format, hexData, writeJSON, writeText, writeUnknown)
		}
	}
	// bufio.Scanner reports a read failure and an over-long line through Err(),
	// never through Scan(). Without this the caller sees a clean, complete decode.
	if err := scanner.Err(); err != nil {
		var eb textbuf.Buffer
		writeOut(output, eb.Str("decoded error ").Err(err).Byte('\n').String())
		return 1
	}
	return 0
}

// decodeGRMode handles "decode capability 64 <hex>" in decode mode.
func decodeGRMode(format, hexData string, writeJSON func([]byte), writeText func(string), writeUnknown func()) {
	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeUnknown()
		return
	}

	result, decErr := decodeGR(data)
	if decErr != nil {
		writeUnknown()
		return
	}

	if format == decodeFormatText {
		writeText(formatGRText(result))
	} else {
		jsonBytes, jsonErr := json.Marshal(result)
		if jsonErr != nil {
			writeUnknown()
			return
		}
		writeJSON(jsonBytes)
	}
}

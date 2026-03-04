// Design: docs/architecture/core-design.md — graceful restart plugin
// RFC: rfc/short/rfc4724.md
// Detail: gr_state.go — GR state machine (per-peer timers, stale family tracking)
//
// Package gr implements a Graceful Restart plugin for ze (RFC 4724).
// It receives per-peer GR config (restart-time) during Stage 2,
// registers GR capabilities per-peer during Stage 3, and implements
// Receiving Speaker procedures during the event loop.
//
// Event subscriptions: open (received), state, eor.
// Inter-plugin coordination: DispatchCommand → bgp-rib retain-routes/release-routes.
package bgp_gr

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

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp-gr/schema"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

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

// SetLogger sets the package-level logger.
// Called by cmd/ze/bgp/plugin_gr.go with slogutil.PluginLogger().
func SetLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// grPlugin holds runtime state for the GR plugin during the event loop.
// Created in RunGRPlugin after the 5-stage handshake completes.
type grPlugin struct {
	sdk *sdk.Plugin

	mu       sync.Mutex
	peerCaps map[string]*grPeerCap // peerAddr → last seen GR capability from OPEN
	state    *grStateManager       // RFC 4724 Receiving Speaker state machine
}

// RunGRPlugin runs the GR plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
// It receives per-peer GR config during Stage 2, registers per-peer
// GR capabilities (code 64) during Stage 3, and runs RFC 4724
// Receiving Speaker procedures during the event loop.
func RunGRPlugin(engineConn, callbackConn net.Conn) int {
	p := sdk.NewWithConn("bgp-gr", engineConn, callbackConn)
	defer func() { _ = p.Close() }()

	gp := &grPlugin{
		sdk:      p,
		peerCaps: make(map[string]*grPeerCap),
	}

	// Create state manager with timer-expiry callback.
	// When a peer's restart timer fires, release retained routes via bgp-rib.
	gp.state = newGRStateManager(func(peerAddr string) {
		gp.onTimerExpired(peerAddr)
	})

	// OnConfigure callback: parse bgp config, extract per-peer restart-time,
	// then set capabilities for Stage 3.
	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		var caps []sdk.CapabilityDecl
		for _, section := range sections {
			if section.Root != "bgp" {
				continue
			}
			caps = append(caps, extractGRCapabilities(section.Data)...)
		}
		p.SetCapabilities(caps)
		return nil
	})

	// Subscribe to events needed for Receiving Speaker procedures:
	//   open direction received — capture peer's GR capability from OPEN
	//   state — detect peer up/down (with reason for GR vs normal teardown)
	//   eor — track End-of-RIB per family for stale route purge
	p.SetStartupSubscriptions(
		[]string{"open direction received", "state", "eor"},
		nil, "full",
	)

	// Event handler: dispatch JSON events to the appropriate GR handler.
	p.OnEvent(func(event string) error {
		return gp.handleEvent(event)
	})

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{"bgp"},
	})
	if err != nil {
		logger().Error("gr plugin failed", "error", err)
		return 1
	}

	return 0
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
	peerAddr, _ := peerObj["address"].(string)
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

// handleOpenEvent extracts the GR capability from a received OPEN message.
// Stores the parsed capability for use when the peer's session later drops.
func (gp *grPlugin) handleOpenEvent(peerAddr string, payload map[string]any) {
	openObj, ok := payload["open"].(map[string]any)
	if !ok {
		return
	}

	caps, ok := openObj["capabilities"].([]any)
	if !ok {
		return
	}

	// Find GR capability (code 64) in capabilities list
	for _, capRaw := range caps {
		capObj, ok := capRaw.(map[string]any)
		if !ok {
			continue
		}
		code, _ := capObj["code"].(float64)
		if int(code) != 64 {
			continue
		}
		hexValue, _ := capObj["value"].(string)
		if hexValue == "" {
			continue
		}

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

		// Convert grResult to grPeerCap for state machine use
		peerCap := grResultToPeerCap(result)

		gp.mu.Lock()
		gp.peerCaps[peerAddr] = peerCap
		gp.mu.Unlock()

		logger().Debug("gr: stored peer capability",
			"peer", peerAddr,
			"restart-time", peerCap.RestartTime,
			"families", len(peerCap.Families))
		return // Only one GR capability per OPEN
	}
}

// handleStateEvent processes peer up/down state changes.
// RFC 4724 Section 4.2:
//   - TCP failure for GR-capable peer → retain routes, start timer
//   - NOTIFICATION → standard BGP (no retention)
//   - Peer reconnects → validate new GR cap, purge non-forwarding families
func (gp *grPlugin) handleStateEvent(peerAddr string, payload map[string]any) {
	state, _ := payload["state"].(string)

	switch state {
	case "down":
		reason, _ := payload["reason"].(string)
		wasNotification := reason == "notification"

		gp.mu.Lock()
		cap := gp.peerCaps[peerAddr]
		gp.mu.Unlock()

		activated := gp.state.onSessionDown(peerAddr, cap, wasNotification)
		if activated {
			// Tell bgp-rib to retain this peer's routes during restart
			gp.dispatchRIBCommand("rib retain-routes " + peerAddr)
		}

	case "up":
		gp.mu.Lock()
		newCap := gp.peerCaps[peerAddr]
		gp.mu.Unlock()

		purged := gp.state.onSessionReestablished(peerAddr, newCap)
		if len(purged) > 0 {
			logger().Debug("gr: purged families on reconnect", "peer", peerAddr, "families", purged)
			// If all families purged, release retained routes
			if !gp.state.peerActive(peerAddr) {
				gp.releaseRoutes(peerAddr)
			}
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

	family, _ := eorObj["family"].(string)
	if family == "" {
		return
	}

	shouldPurge := gp.state.onEORReceived(peerAddr, family)
	if shouldPurge {
		logger().Debug("gr: EOR received, stale routes purged", "peer", peerAddr, "family", family)

		// If GR is now complete (all EORs received), release retained routes
		if !gp.state.peerActive(peerAddr) {
			gp.releaseRoutes(peerAddr)
		}
	}
}

// onTimerExpired is called when a peer's restart timer fires.
// RFC 4724 Section 4.2: delete all stale routes from the peer.
func (gp *grPlugin) onTimerExpired(peerAddr string) {
	gp.releaseRoutes(peerAddr)
}

// releaseRoutes tells bgp-rib to release (delete) retained routes for a peer.
func (gp *grPlugin) releaseRoutes(peerAddr string) {
	gp.dispatchRIBCommand("rib release-routes " + peerAddr)
}

// dispatchRIBCommand sends a command to the engine for inter-plugin coordination.
// Logs errors but does not fail — the GR state machine proceeds regardless.
func (gp *grPlugin) dispatchRIBCommand(command string) {
	if gp.sdk == nil {
		return // unit test — no SDK available
	}
	ctx := context.Background()
	status, _, err := gp.sdk.DispatchCommand(ctx, command)
	if err != nil {
		logger().Warn("gr: dispatch failed", "command", command, "err", err)
	} else {
		logger().Debug("gr: dispatch ok", "command", command, "status", status)
	}
}

// grResultToPeerCap converts a decoded GR wire result to the state machine's
// capability representation, mapping AFI/SAFI numbers to family strings.
func grResultToPeerCap(r *grResult) *grPeerCap {
	cap := &grPeerCap{
		RestartTime: r.RestartTime,
		Families:    make([]grCapFamily, 0, len(r.Families)),
	}
	for _, f := range r.Families {
		family := afiSAFIToFamily(f.AFI, f.SAFI)
		if family != "" {
			cap.Families = append(cap.Families, grCapFamily{
				Family:       family,
				ForwardState: f.ForwardState,
			})
		}
	}
	return cap
}

// afiSAFIToFamily converts AFI/SAFI numbers to ze family string format.
// Delegates to nlri.Family.String() — single source of truth for family names.
func afiSAFIToFamily(afi uint16, safi uint8) string {
	return nlri.Family{AFI: nlri.AFI(afi), SAFI: nlri.SAFI(safi)}.String()
}

// extractGRCapabilities parses bgp config JSON and returns per-peer GR capabilities.
// RFC 4724: Graceful Restart capability code is 64.
func extractGRCapabilities(jsonStr string) []sdk.CapabilityDecl {
	var bgpConfig map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &bgpConfig); err != nil {
		logger().Warn("invalid JSON in bgp config", "err", err)
		return nil
	}

	// The config tree is wrapped: {"bgp": {"peer": {...}}}
	bgpSubtree, ok := bgpConfig["bgp"].(map[string]any)
	if !ok {
		bgpSubtree = bgpConfig
	}

	peersMap, ok := bgpSubtree["peer"].(map[string]any)
	if !ok {
		logger().Debug("no peer config in bgp tree")
		return nil
	}

	const grCapCode = 64
	var caps []sdk.CapabilityDecl

	for peerAddr, peerData := range peersMap {
		peerMap, ok := peerData.(map[string]any)
		if !ok {
			continue
		}

		capMap, ok := peerMap["capability"].(map[string]any)
		if !ok {
			continue
		}

		grData, ok := capMap["graceful-restart"].(map[string]any)
		if !ok {
			continue
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
		capValue := fmt.Sprintf("%04x", restartTime&0x0FFF)
		caps = append(caps, sdk.CapabilityDecl{
			Code:     grCapCode,
			Encoding: "hex",
			Payload:  capValue,
			Peers:    []string{peerAddr},
		})
		logger().Debug("gr capability", "peer", peerAddr, "restart-time", restartTime)
	}

	return caps
}

// RunCLIDecode decodes hex capability data directly from CLI arguments.
// This is for human use: `ze plugin gr --capa <hex>` or with `--text`.
// Returns exit code (0 = success, 1 = error).
func RunCLIDecode(hexData string, textOutput bool, stdout, stderr io.Writer) int {
	// Decode hex
	data, err := hex.DecodeString(hexData)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: invalid hex: %v\n", err)
		return 1
	}

	// Parse GR capability value
	result, err := decodeGR(data)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	// Output based on format
	if textOutput {
		_, _ = fmt.Fprintln(stdout, formatGRText(result))
	} else {
		jsonBytes, err := json.Marshal(result)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "error: JSON encoding: %v\n", err)
			return 1
		}
		_, _ = fmt.Fprintln(stdout, string(jsonBytes))
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
	fmt.Fprintf(&sb, "%-20s restart-time=%d", "graceful-restart", r.RestartTime)
	if r.Restarting {
		sb.WriteString(" restarting")
	}
	if r.Notification {
		sb.WriteString(" notification")
	}
	for _, f := range r.Families {
		fmt.Fprintf(&sb, " afi=%d/safi=%d", f.AFI, f.SAFI)
		if f.ForwardState {
			sb.WriteString("(F)")
		}
	}
	return sb.String()
}

// GetYANG returns the embedded YANG schema for the GR plugin.
func GetYANG() string {
	return schema.ZeGracefulRestartYANG
}

// writeOut writes a string to the output writer, discarding errors.
// Decode mode writes to an in-memory buffer; write failures are not actionable.
func writeOut(w io.Writer, s string) {
	if _, err := io.WriteString(w, s); err != nil {
		logger().Debug("decode write failed", "err", err)
	}
}

// RunDecodeMode runs the plugin in decode mode for ze bgp decode.
// RFC 4724: Graceful Restart capability code 64.
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

		format := "json"
		capIdx := 1
		if parts[1] == "json" || parts[1] == "text" {
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

		if parts[capIdx+1] != "64" {
			writeUnknown()
			continue
		}

		hexData := parts[capIdx+2]
		data, hexErr := hex.DecodeString(hexData)
		if hexErr != nil {
			writeUnknown()
			continue
		}

		result, decErr := decodeGR(data)
		if decErr != nil {
			writeUnknown()
			continue
		}

		if format == "text" {
			writeText(formatGRText(result))
		} else {
			jsonBytes, jsonErr := json.Marshal(result)
			if jsonErr != nil {
				writeUnknown()
				continue
			}
			writeJSON(jsonBytes)
		}
	}
	return 0
}

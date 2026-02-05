// Package gr implements a Graceful Restart capability plugin for ze.
// It receives per-peer GR config (restart-time) during Stage 2 and
// registers GR capabilities per-peer during Stage 3.
//
// RFC 4724: Graceful Restart Mechanism for BGP.
package gr

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/plugin/gr/schema"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// logger is the package-level logger, disabled by default.
// Use SetLogger() to enable logging from CLI --log-level flag.
var logger = slogutil.DiscardLogger()

// SetLogger sets the package-level logger.
// Called by cmd/ze/bgp/plugin_gr.go with slogutil.PluginLogger().
func SetLogger(l *slog.Logger) {
	if l != nil {
		logger = l
	}
}

// GRPlugin implements a Graceful Restart capability plugin.
// It receives per-peer restart-time config and registers GR capabilities.
type GRPlugin struct {
	input  *bufio.Scanner
	output io.Writer

	// grConfig stores per-peer restart-time configuration.
	// RFC 4724: restart-time is 0-4095 seconds (12-bit field).
	grConfig map[string]uint16 // peerAddr → restart-time

	mu       sync.Mutex
	outputMu sync.Mutex
}

// MaxLineSize is the maximum size of a single input line (1MB).
const MaxLineSize = 1024 * 1024

// NewGRPlugin creates a new GRPlugin.
func NewGRPlugin(input io.Reader, output io.Writer) *GRPlugin {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, MaxLineSize), MaxLineSize)
	return &GRPlugin{
		input:    scanner,
		output:   output,
		grConfig: make(map[string]uint16),
	}
}

// Run starts the GR plugin.
func (g *GRPlugin) Run() int {
	g.doStartupProtocol()
	g.eventLoop()
	return 0
}

// doStartupProtocol performs the 5-stage plugin registration protocol.
func (g *GRPlugin) doStartupProtocol() {
	// Stage 1: Declaration
	// Request bgp config subtree as JSON - plugin extracts graceful-restart settings.
	g.send("declare wants config bgp")
	g.send("declare done")

	// Stage 2: Parse config (JSON format)
	g.parseConfig()

	// Stage 3: Register GR capabilities per peer
	g.registerCapabilities()

	// Stage 4: Wait for registry
	g.waitForLine("registry done")

	// Stage 5: Ready
	g.send("ready")
}

// parseConfig reads and parses config lines until "config done".
// Handles JSON config format: "config json bgp <json>".
func (g *GRPlugin) parseConfig() {
	for g.input.Scan() {
		line := g.input.Text()
		if line == "config done" {
			return
		}
		g.parseConfigLine(line)
	}
}

// parseConfigLine parses a single config line.
// Format: "config json bgp <json>" where json contains full bgp config tree.
func (g *GRPlugin) parseConfigLine(line string) {
	// Handle JSON config format: "config json bgp <json>"
	if strings.HasPrefix(line, "config json bgp ") {
		g.parseBGPConfig(line)
		return
	}

	logger.Debug("ignoring non-bgp config line", "line", line)
}

// parseBGPConfig parses JSON config format: "config json bgp <json>".
// Extracts graceful-restart config from each peer in the bgp tree.
func (g *GRPlugin) parseBGPConfig(line string) {
	// Format: config json bgp <json>
	const prefix = "config json bgp "
	if len(line) <= len(prefix) {
		logger.Warn("empty bgp config JSON")
		return
	}
	jsonStr := line[len(prefix):]

	// Parse JSON bgp config tree
	var bgpConfig map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &bgpConfig); err != nil {
		logger.Warn("invalid JSON in bgp config", "err", err)
		return
	}

	// The config tree is wrapped: {"bgp": {"peer": {...}}}
	bgpSubtree, ok := bgpConfig["bgp"].(map[string]any)
	if !ok {
		// Try using bgpConfig directly in case it's not wrapped
		bgpSubtree = bgpConfig
	}

	// Extract peer map: {"peer": {"192.168.1.1": {...}, ...}}
	peersMap, ok := bgpSubtree["peer"].(map[string]any)
	if !ok {
		logger.Debug("no peer config in bgp tree")
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// Iterate peers and extract graceful-restart config
	for peerAddr, peerData := range peersMap {
		peerMap, ok := peerData.(map[string]any)
		if !ok {
			continue
		}

		// Look for capability.graceful-restart
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
			logger.Warn("restart-time exceeds 12-bit max, clamping", "peer", peerAddr, "value", restartTime)
			restartTime = 4095
		}

		g.grConfig[peerAddr] = restartTime
		logger.Debug("parsed config", "peer", peerAddr, "restart-time", restartTime)
	}
}

// registerCapabilities sends Stage 3 capability declarations.
// Registers GR capability (code 64) per peer with configured restart-time.
func (g *GRPlugin) registerCapabilities() {
	// RFC 4724: Graceful Restart capability code is 64.
	// Wire format: [flags+restart-time:2 bytes] [AFI:2][SAFI:1][F-bit:1] per family.
	// For simplicity, we send restart-time only (no families - peer advertises those).
	const grCapCode = 64

	g.mu.Lock()
	defer g.mu.Unlock()

	for peerAddr, restartTime := range g.grConfig {
		// Build GR capability value: 2 bytes (flags=0, restart-time in lower 12 bits)
		// RFC 4724 Section 3: Restart Flags (4 bits) + Restart Time (12 bits)
		capValue := fmt.Sprintf("%04x", restartTime&0x0FFF)
		g.send("capability hex %d %s peer %s", grCapCode, capValue, peerAddr)
		logger.Debug("registered capability", "peer", peerAddr, "restart-time", restartTime)
	}
	g.send("capability done")
}

// eventLoop runs the minimal event loop.
// GR plugin is mostly stateless after startup - just handles shutdown.
func (g *GRPlugin) eventLoop() {
	for g.input.Scan() {
		line := g.input.Text()
		if len(line) == 0 {
			continue
		}
		// GR plugin doesn't need to handle events - it's capability-only.
		// Just consume input until EOF (shutdown).
		logger.Debug("event (ignored)", "line", line[:min(50, len(line))])
	}
}

// waitForLine reads lines until one matches the expected line.
func (g *GRPlugin) waitForLine(expected string) {
	for g.input.Scan() {
		line := g.input.Text()
		if line == expected {
			return
		}
	}
}

// send sends raw output to ze.
func (g *GRPlugin) send(format string, args ...any) {
	g.outputMu.Lock()
	_, _ = fmt.Fprintf(g.output, format+"\n", args...)
	g.outputMu.Unlock()
}

// RunCLIDecode decodes hex capability data directly from CLI arguments.
// This is for human use: `ze bgp plugin gr --capa <hex>` or with `--text`.
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
	sb.WriteString(fmt.Sprintf("%-20s restart-time=%d", "graceful-restart", r.RestartTime))
	if r.Restarting {
		sb.WriteString(" restarting")
	}
	if r.Notification {
		sb.WriteString(" notification")
	}
	for _, f := range r.Families {
		sb.WriteString(fmt.Sprintf(" afi=%d/safi=%d", f.AFI, f.SAFI))
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

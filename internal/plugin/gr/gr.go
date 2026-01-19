// Package gr implements a Graceful Restart capability plugin for ZeBGP.
// It receives per-peer GR config (restart-time) during Stage 2 and
// registers GR capabilities per-peer during Stage 3.
//
// RFC 4724: Graceful Restart Mechanism for BGP.
package gr

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"codeberg.org/thomas-mangin/zebgp/internal/slogutil"
)

// logger is the package-level logger, disabled by default.
// Use SetLogger() to enable logging from CLI --log-level flag.
var logger = slogutil.DiscardLogger()

// SetLogger sets the package-level logger.
// Called by cmd/zebgp/plugin_gr.go with slogutil.LoggerWithLevel().
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
	// Register for Graceful Restart config (RFC 4724).
	// Uses scoped key "graceful-restart:restart-time" matching RawCapabilityConfig format.
	g.send("declare conf peer * capability graceful-restart:restart-time <restart-time:\\d+>")
	g.send("declare done")

	// Stage 2: Parse config
	g.parseConfig()

	// Stage 3: Register GR capabilities per peer
	g.registerCapabilities()

	// Stage 4: Wait for registry
	g.waitForLine("registry done")

	// Stage 5: Ready
	g.send("ready")
}

// parseConfig reads and parses config lines until "config done".
// Expected format: "config peer <addr> restart-time <value>".
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
// Format: "config peer <addr> restart-time <value>".
// Note: Config delivery sends capture NAME ("restart-time"), not full pattern key.
func (g *GRPlugin) parseConfigLine(line string) {
	// Expected: "config peer 192.168.1.1 restart-time 120"
	if !strings.HasPrefix(line, "config peer ") {
		logger.Debug("ignoring non-peer config", "line", line)
		return
	}

	// Parse: config peer <addr> restart-time <value>
	parts := strings.Fields(line)
	if len(parts) < 5 {
		logger.Warn("malformed config line", "line", line)
		return
	}

	peerAddr := parts[2]
	key := parts[3]
	valueStr := parts[4]

	if key != "restart-time" {
		logger.Debug("ignoring non-GR config", "key", key)
		return
	}

	value, err := strconv.ParseUint(valueStr, 10, 16)
	if err != nil {
		logger.Warn("invalid restart-time value", "peer", peerAddr, "value", valueStr, "error", err)
		return
	}

	// RFC 4724: restart-time is 12 bits (0-4095)
	if value > 4095 {
		logger.Warn("restart-time exceeds 12-bit max, clamping", "peer", peerAddr, "value", value)
		value = 4095
	}

	g.mu.Lock()
	g.grConfig[peerAddr] = uint16(value)
	g.mu.Unlock()

	logger.Debug("parsed config", "peer", peerAddr, "restart-time", value)
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

// send sends raw output to ZeBGP.
func (g *GRPlugin) send(format string, args ...any) {
	g.outputMu.Lock()
	_, _ = fmt.Fprintf(g.output, format+"\n", args...)
	g.outputMu.Unlock()
}

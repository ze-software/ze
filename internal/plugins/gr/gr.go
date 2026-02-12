// Package gr implements a Graceful Restart capability plugin for ze.
// It receives per-peer GR config (restart-time) during Stage 2 and
// registers GR capabilities per-peer during Stage 3.
//
// RFC 4724: Graceful Restart Mechanism for BGP.
package gr

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/plugins/gr/schema"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
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

// RunGRPlugin runs the GR plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
// It receives per-peer GR config during Stage 2 and registers per-peer
// GR capabilities (code 64) during Stage 3.
func RunGRPlugin(engineConn, callbackConn net.Conn) int {
	p := sdk.NewWithConn("gr", engineConn, callbackConn)
	defer func() { _ = p.Close() }()

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

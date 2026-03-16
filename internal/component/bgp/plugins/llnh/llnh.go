// Design: docs/architecture/core-design.md — link-local next-hop plugin
// RFC: rfc/short/rfc5549.md
//
// Package llnh implements a link-local next-hop capability plugin for ze.
// It declares capability code 77 (draft-ietf-idr-linklocal-capability) for peers
// that have link-local-nexthop enabled in their config.
//
// Capability 77 has no payload — it is a simple flag signaling willingness
// to receive IPv6 link-local addresses as BGP next-hops (RFC 2545 Section 3).
package llnh

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/configjson"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/llnh/schema"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// llnhCapCode is the capability code for link-local next-hop.
// draft-ietf-idr-linklocal-capability: code 77, empty payload.
const llnhCapCode = 77

// modeDisable is the config value that suppresses capability advertisement.
const modeDisable = "disable"

// loggerPtr is the package-level logger, disabled by default.
// Stored as atomic.Pointer to avoid data races when tests start
// multiple in-process plugin instances concurrently.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

// SetLLNHLogger sets the package-level logger.
func SetLLNHLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// RunLLNHPlugin runs the link-local-nexthop plugin using the SDK RPC protocol.
// It receives per-peer config during Stage 2 and registers capability 77
// for peers that have link-local-nexthop enabled during Stage 3.
func RunLLNHPlugin(engineConn, callbackConn net.Conn) int {
	logger().Debug("llnh plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-llnh", engineConn, callbackConn)
	defer func() { _ = p.Close() }()

	// OnConfigure callback: parse bgp config, find peers with link-local-nexthop,
	// then set capabilities for Stage 3.
	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		var caps []sdk.CapabilityDecl
		for _, section := range sections {
			if section.Root != "bgp" {
				continue
			}
			caps = append(caps, extractLLNHCapabilities(section.Data)...)
		}
		p.SetCapabilities(caps)
		return nil
	})

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{"bgp"},
	})
	if err != nil {
		logger().Error("llnh plugin failed", "error", err)
		return 1
	}

	return 0
}

// isLLNHEnabled checks whether a capability map has link-local-nexthop enabled.
// Returns (hasExplicit, enabled).
func isLLNHEnabled(capMap map[string]any) (bool, bool) {
	if capMap == nil {
		return false, false
	}
	llnhVal, exists := capMap["link-local-nexthop"]
	if !exists {
		return false, false
	}
	if s, isStr := llnhVal.(string); isStr && s == modeDisable {
		return true, false
	}
	return true, true
}

// extractLLNHCapabilities parses bgp config JSON and returns per-peer capabilities.
// Handles both standalone peers (bgp.peer) and grouped peers (bgp.group.<name>.peer).
// draft-ietf-idr-linklocal-capability: capability code 77, empty payload.
func extractLLNHCapabilities(jsonStr string) []sdk.CapabilityDecl {
	bgpSubtree, ok := configjson.ParseBGPSubtree(jsonStr)
	if !ok {
		logger().Warn("invalid JSON in bgp config")
		return nil
	}

	var caps []sdk.CapabilityDecl

	configjson.ForEachPeer(bgpSubtree, func(peerAddr string, peerMap, groupMap map[string]any) {
		// Check per-peer link-local-nexthop capability first.
		peerHasExplicit, peerEnabled := isLLNHEnabled(configjson.GetCapability(peerMap))

		// Check group-level link-local-nexthop capability (fallback).
		groupEnabled := false
		if groupMap != nil {
			_, groupEnabled = isLLNHEnabled(configjson.GetCapability(groupMap))
		}

		// Per-peer wins; if no per-peer config, use group default.
		enabled := groupEnabled
		if peerHasExplicit {
			enabled = peerEnabled
		}
		if !enabled {
			return
		}

		// Capability 77 has empty payload -- just the code signals support
		caps = append(caps, sdk.CapabilityDecl{
			Code:  llnhCapCode,
			Peers: []string{peerAddr},
		})
		logger().Debug("link-local-nexthop capability", "peer", peerAddr)
	})

	return caps
}

// GetLLNHYANG returns the embedded YANG schema for the llnh plugin.
func GetLLNHYANG() string {
	return schema.ZeLinkLocalNexthopYANG
}

// LLNHDecodableCapabilities returns the capability codes this plugin can decode.
func LLNHDecodableCapabilities() []uint8 {
	return []uint8{llnhCapCode}
}

// RunLLNHDecodeMode runs the plugin in decode mode for ze bgp decode.
// Reads decode requests from stdin, writes responses to stdout.
//
// Capability 77 has no payload, so decoding always succeeds with the same output.
func RunLLNHDecodeMode(input io.Reader, output io.Writer) int {
	writeResponse := func(s string) {
		_, err := io.WriteString(output, s)
		_ = err // Protocol writes - pipe failure causes exit
	}
	writeUnknown := func() { writeResponse("decoded unknown\n") }

	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Parse request: "decode [json|text] capability <code> <hex>"
		parts := strings.Fields(line)
		if len(parts) < 3 || parts[0] != "decode" {
			writeUnknown()
			continue
		}

		// Determine format and adjust parts index
		format := "json"
		capIdx := 1
		if parts[1] == "json" || parts[1] == "text" {
			format = parts[1]
			capIdx = 2
			if len(parts) < 4 {
				writeUnknown()
				continue
			}
		}

		if parts[capIdx] != "capability" {
			writeUnknown()
			continue
		}

		codeIdx := capIdx + 1
		if parts[codeIdx] != "77" {
			writeUnknown()
			continue
		}

		// Capability 77 has empty payload — no hex to decode
		if format == "text" {
			writeResponse("decoded text link-local-nexthop\n")
		} else {
			result := map[string]any{
				"name": "link-local-nexthop",
			}
			jsonBytes, err := json.Marshal(result)
			if err != nil {
				writeUnknown()
				continue
			}
			writeResponse("decoded json " + string(jsonBytes) + "\n")
		}
	}
	return 0
}

// RunLLNHCLIDecode decodes hex capability data directly from CLI arguments.
// For capability 77, the payload is always empty — this just confirms the capability.
func RunLLNHCLIDecode(hexData string, textOutput bool, stdout, stderr io.Writer) int {
	write := func(w io.Writer, s string) {
		_, err := io.WriteString(w, s)
		_ = err // CLI output - pipe failure causes exit
	}

	if textOutput {
		write(stdout, "link-local-nexthop\n")
	} else {
		result := map[string]any{
			"name": "link-local-nexthop",
		}
		jsonBytes, err := json.Marshal(result)
		if err != nil {
			write(stderr, "error: JSON encoding: "+err.Error()+"\n")
			return 1
		}
		write(stdout, string(jsonBytes)+"\n")
	}
	return 0
}

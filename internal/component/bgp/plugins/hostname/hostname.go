// Design: docs/architecture/core-design.md — hostname capability plugin
//
// Package hostname implements a hostname (FQDN) capability plugin for ze.
// It receives per-peer hostname/domain config and registers FQDN capabilities (code 73).
//
// draft-walton-bgp-hostname: FQDN Capability for BGP.
package hostname

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/configjson"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/hostname/schema"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// Logger is the package-level logger, disabled by default.
// Set via ConfigureLogger from CLI --log-level flag.
var Logger = slogutil.DiscardLogger()

// ConfigureLogger sets the package-level logger.
// Called by cmd/ze/bgp/plugin_hostname.go with slogutil.PluginLogger().
func ConfigureLogger(l *slog.Logger) {
	if l != nil {
		Logger = l
	}
}

// fqdnConfig holds per-peer FQDN configuration.
type fqdnConfig struct {
	hostname string
	domain   string
}

// encodeValue returns the hex-encoded capability value (without code/length prefix).
// Wire format: hostname-len (1) + hostname + domain-len (1) + domain.
func (c *fqdnConfig) encodeValue() string {
	host := c.hostname
	domain := c.domain

	// draft-walton-bgp-hostname: Max 255 bytes each (1-octet length field)
	if len(host) > 255 {
		Logger.Warn("hostname exceeds 255 bytes, truncating", "len", len(host))
		host = host[:255]
	}
	if len(domain) > 255 {
		Logger.Warn("domain exceeds 255 bytes, truncating", "len", len(domain))
		domain = domain[:255]
	}

	// Build wire bytes: hostLen + host + domainLen + domain
	data := make([]byte, 1+len(host)+1+len(domain))
	data[0] = byte(len(host))
	copy(data[1:1+len(host)], host)
	data[1+len(host)] = byte(len(domain))
	copy(data[2+len(host):], domain)

	return hex.EncodeToString(data)
}

// RunHostnamePlugin runs the hostname plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
// It receives per-peer hostname/domain config during Stage 2 and registers
// per-peer FQDN capabilities (code 73) during Stage 3.
func RunHostnamePlugin(conn net.Conn) int {
	Logger.Debug("hostname plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-hostname", conn)
	defer func() { _ = p.Close() }()

	// OnConfigure callback: parse bgp config, extract per-peer hostname/domain,
	// then set capabilities for Stage 3.
	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		var caps []sdk.CapabilityDecl
		for _, section := range sections {
			if section.Root != "bgp" {
				continue
			}
			caps = append(caps, extractHostnameCapabilities(section.Data)...)
		}
		p.SetCapabilities(caps)
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{"bgp"},
	})
	if err != nil {
		Logger.Error("hostname plugin failed", "error", err)
		return 1
	}

	return 0
}

// parseFQDNFromCapability extracts an fqdnConfig from a capability map's "hostname" entry.
// Returns nil if no hostname data or both fields are empty.
func parseFQDNFromCapability(capMap map[string]any) *fqdnConfig {
	if capMap == nil {
		return nil
	}
	hostnameData, ok := capMap["hostname"].(map[string]any)
	if !ok {
		return nil
	}
	cfg := &fqdnConfig{}
	if host, ok := hostnameData["host"].(string); ok {
		cfg.hostname = host
	}
	if domain, ok := hostnameData["domain"].(string); ok {
		cfg.domain = domain
	}
	if cfg.hostname == "" && cfg.domain == "" {
		return nil
	}
	return cfg
}

// extractHostnameCapabilities parses bgp config JSON and returns per-peer FQDN capabilities.
// Handles both standalone peers (bgp.peer) and grouped peers (bgp.group.<name>.peer).
// draft-walton-bgp-hostname: FQDN capability code is 73.
func extractHostnameCapabilities(jsonStr string) []sdk.CapabilityDecl {
	bgpSubtree, ok := configjson.ParseBGPSubtree(jsonStr)
	if !ok {
		Logger.Warn("invalid JSON in bgp config")
		return nil
	}

	const fqdnCapCode = 73
	var caps []sdk.CapabilityDecl

	configjson.ForEachPeer(bgpSubtree, func(peerAddr string, peerMap, groupMap map[string]any) {
		// Check per-peer hostname config first.
		peerCfg := parseFQDNFromCapability(configjson.GetCapability(peerMap))

		// Check group-level hostname config (fallback).
		var groupCfg *fqdnConfig
		if groupMap != nil {
			groupCfg = parseFQDNFromCapability(configjson.GetCapability(groupMap))
		}

		// Per-peer wins over group.
		useCfg := groupCfg
		if peerCfg != nil {
			useCfg = peerCfg
		}
		if useCfg == nil {
			return
		}

		caps = append(caps, sdk.CapabilityDecl{
			Code:     fqdnCapCode,
			Encoding: "hex",
			Payload:  useCfg.encodeValue(),
			Peers:    []string{peerAddr},
		})
		Logger.Debug("hostname capability", "peer", peerAddr, "hostname", useCfg.hostname, "domain", useCfg.domain)
	})

	if len(caps) == 0 {
		Logger.Debug("no hostname capabilities found in bgp config")
	}

	return caps
}

// GetYANG returns the embedded YANG schema for the hostname plugin.
func GetYANG() string {
	return schema.ZeHostnameYANG
}

// DecodableCapabilities returns the capability codes this plugin can decode.
func DecodableCapabilities() []uint8 {
	return []uint8{73} // FQDN capability
}

// RunDecodeMode runs the plugin in decode mode for ze bgp decode.
// Reads decode requests from stdin, writes responses to stdout.
//
// Request formats:
//   - "decode capability <code> <hex>" → JSON (default)
//   - "decode json capability <code> <hex>" → JSON (explicit)
//   - "decode text capability <code> <hex>" → human-readable text
//
// Response formats:
//   - "decoded json <json>" for JSON format
//   - "decoded text <text>" for text format
//   - "decoded unknown" on failure
func RunDecodeMode(input io.Reader, output io.Writer) int {
	// Response writers - use io.WriteString which returns error we can ignore
	// via the standard nolint pattern. Pipe errors cause plugin exit anyway.
	writeResponse := func(s string) {
		_, err := io.WriteString(output, s)
		_ = err // Protocol writes - pipe failure causes exit
	}
	writeUnknown := func() { writeResponse("decoded unknown\n") }
	writeJSON := func(j []byte) { writeResponse("decoded json " + string(j) + "\n") }
	writeText := func(t string) { writeResponse("decoded text " + t + "\n") }

	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Parse request: "decode [json|text] capability <code> <hex>"
		parts := strings.Fields(line)
		if len(parts) < 4 || parts[0] != "decode" {
			writeUnknown()
			continue
		}

		// Determine format and adjust parts index
		format := "json" // default
		capIdx := 1      // index of "capability" keyword
		if parts[1] == "json" || parts[1] == "text" {
			format = parts[1]
			capIdx = 2
			if len(parts) < 5 {
				writeUnknown()
				continue
			}
		}

		// Validate "capability" keyword
		if parts[capIdx] != "capability" {
			writeUnknown()
			continue
		}

		// Check capability code (73 = FQDN)
		codeIdx := capIdx + 1
		hexIdx := capIdx + 2
		if parts[codeIdx] != "73" {
			writeUnknown()
			continue
		}

		// Decode hex payload
		hexData := parts[hexIdx]
		data, err := hex.DecodeString(hexData)
		if err != nil {
			writeUnknown()
			continue
		}

		// Parse FQDN capability value
		hostname, domain := decodeFQDN(data)
		if hostname == "" && domain == "" && len(data) < 2 {
			writeUnknown()
			continue
		}

		// Output based on format
		if format == "text" {
			writeText(formatFQDNText(hostname, domain))
		} else {
			result := map[string]any{
				"name":     "fqdn",
				"hostname": hostname,
				"domain":   domain,
			}
			jsonBytes, err := json.Marshal(result)
			if err != nil {
				writeUnknown()
				continue
			}
			writeJSON(jsonBytes)
		}
	}
	return 0
}

// formatFQDNText formats FQDN as human-readable text.
// Output format: "fqdn                 hostname.domain".
func formatFQDNText(hostname, domain string) string {
	var fqdn string
	switch {
	case hostname != "" && domain != "":
		fqdn = hostname + "." + domain
	case hostname != "":
		fqdn = hostname
	case domain != "":
		fqdn = domain
	case hostname == "" && domain == "":
		fqdn = "(empty)"
	}
	return fmt.Sprintf("%-20s %s", "fqdn", fqdn)
}

// decodeFQDN decodes FQDN capability wire bytes to hostname and domain strings.
// Wire format: hostname-len (1) + hostname + domain-len (1) + domain.
func decodeFQDN(data []byte) (hostname, domain string) {
	if len(data) < 1 {
		return "", ""
	}

	hostLen := int(data[0])
	if len(data) < 1+hostLen+1 {
		return "", ""
	}

	hostname = string(data[1 : 1+hostLen])

	domainLen := int(data[1+hostLen])
	if len(data) < 1+hostLen+1+domainLen {
		return "", ""
	}

	domain = string(data[2+hostLen : 2+hostLen+domainLen])
	return hostname, domain
}

// RunCLIDecode decodes hex capability data directly from CLI arguments.
// This is for human use: `ze plugin hostname --capa <hex>` or with `--text`.
// Returns exit code (0 = success, 1 = error).
func RunCLIDecode(hexData string, textOutput bool, stdout, stderr io.Writer) int {
	// Decode hex
	data, err := hex.DecodeString(hexData)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: invalid hex: %v\n", err)
		return 1
	}

	// Parse FQDN capability value
	hostname, domain := decodeFQDN(data)

	// Output based on format
	if textOutput {
		_, _ = fmt.Fprintln(stdout, formatFQDNText(hostname, domain))
	} else {
		result := map[string]any{
			"name":     "fqdn",
			"hostname": hostname,
			"domain":   domain,
		}
		jsonBytes, err := json.Marshal(result)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "error: JSON encoding: %v\n", err)
			return 1
		}
		_, _ = fmt.Fprintln(stdout, string(jsonBytes))
	}
	return 0
}

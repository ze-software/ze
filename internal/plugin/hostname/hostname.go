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
	"strings"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/slogutil"
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

// HostnamePlugin implements a hostname (FQDN) capability plugin.
// It receives per-peer hostname/domain config and registers FQDN capabilities.
type HostnamePlugin struct {
	input  *bufio.Scanner
	output io.Writer

	// hostConfig stores per-peer FQDN configuration.
	hostConfig map[string]*fqdnConfig // peerAddr → config

	mu       sync.Mutex
	outputMu sync.Mutex
}

// MaxLineSize is the maximum size of a single input line (1MB).
const MaxLineSize = 1024 * 1024

// NewHostnamePlugin creates a new HostnamePlugin.
func NewHostnamePlugin(input io.Reader, output io.Writer) *HostnamePlugin {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, MaxLineSize), MaxLineSize)
	return &HostnamePlugin{
		input:      scanner,
		output:     output,
		hostConfig: make(map[string]*fqdnConfig),
	}
}

// Run starts the hostname plugin.
func (h *HostnamePlugin) Run() int {
	Logger.Debug("hostname plugin starting")
	h.doStartupProtocol()
	Logger.Debug("hostname plugin startup complete, entering event loop")
	h.eventLoop()
	return 0
}

// doStartupProtocol performs the 5-stage plugin registration protocol.
func (h *HostnamePlugin) doStartupProtocol() {
	// Stage 1: Declaration
	// Request bgp config subtree as JSON - plugin extracts what it needs based on YANG knowledge.
	// This avoids needing per-plugin extraction code in the config loader.
	h.send("declare wants config bgp")
	h.send("declare done")

	// Stage 2: Parse config (JSON format)
	h.parseConfig()

	// Stage 3: Register FQDN capabilities per peer
	h.registerCapabilities()

	// Stage 4: Wait for registry
	h.waitForLine("registry done")

	// Stage 5: Ready
	h.send("ready")
}

// parseConfig reads and parses config lines until "config done".
// Handles JSON config format: "config json bgp <json>".
func (h *HostnamePlugin) parseConfig() {
	for h.input.Scan() {
		line := h.input.Text()
		if line == "config done" {
			return
		}
		h.parseConfigLine(line)
	}
}

// parseConfigLine parses a single config line.
// Format: "config json bgp <json>" where json contains full bgp config tree.
func (h *HostnamePlugin) parseConfigLine(line string) {
	// Handle JSON config format: "config json bgp <json>"
	if strings.HasPrefix(line, "config json bgp ") {
		h.parseBGPConfig(line)
		return
	}

	Logger.Debug("ignoring non-bgp config line", "line", line)
}

// parseBGPConfig parses JSON config format: "config json bgp <json>".
// Extracts hostname config from each peer in the bgp tree.
func (h *HostnamePlugin) parseBGPConfig(line string) {
	// Format: config json bgp <json>
	const prefix = "config json bgp "
	if len(line) <= len(prefix) {
		Logger.Warn("empty bgp config JSON")
		return
	}
	jsonStr := line[len(prefix):]

	// Parse JSON bgp config tree
	var bgpConfig map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &bgpConfig); err != nil {
		Logger.Warn("invalid JSON in bgp config", "err", err)
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
		Logger.Debug("no peer config in bgp tree")
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Iterate peers and extract hostname config
	for peerAddr, peerData := range peersMap {
		peerMap, ok := peerData.(map[string]any)
		if !ok {
			continue
		}

		// Look for capability.hostname
		capMap, ok := peerMap["capability"].(map[string]any)
		if !ok {
			continue
		}

		hostnameData, ok := capMap["hostname"].(map[string]any)
		if !ok {
			continue
		}

		cfg, exists := h.hostConfig[peerAddr]
		if !exists {
			cfg = &fqdnConfig{}
			h.hostConfig[peerAddr] = cfg
		}

		if host, ok := hostnameData["host"].(string); ok {
			cfg.hostname = host
			Logger.Debug("parsed hostname from JSON", "peer", peerAddr, "hostname", host)
		}
		if domain, ok := hostnameData["domain"].(string); ok {
			cfg.domain = domain
			Logger.Debug("parsed domain from JSON", "peer", peerAddr, "domain", domain)
		}
	}
}

// registerCapabilities sends Stage 3 capability declarations.
// Registers FQDN capability (code 73) per peer with configured hostname/domain.
func (h *HostnamePlugin) registerCapabilities() {
	// draft-walton-bgp-hostname: FQDN capability code is 73.
	const fqdnCapCode = 73

	h.mu.Lock()
	defer h.mu.Unlock()

	for peerAddr, cfg := range h.hostConfig {
		// Skip if both hostname and domain are empty
		if cfg.hostname == "" && cfg.domain == "" {
			continue
		}

		capValue := cfg.encodeValue()
		h.send("capability hex %d %s peer %s", fqdnCapCode, capValue, peerAddr)
		Logger.Debug("registered capability", "peer", peerAddr, "hostname", cfg.hostname, "domain", cfg.domain)
	}
	h.send("capability done")
}

// eventLoop runs the minimal event loop.
// Hostname plugin is mostly stateless after startup - just handles shutdown.
func (h *HostnamePlugin) eventLoop() {
	for h.input.Scan() {
		line := h.input.Text()
		if len(line) == 0 {
			continue
		}
		// Hostname plugin doesn't need to handle events - it's capability-only.
		// Just consume input until EOF (shutdown).
		Logger.Debug("event (ignored)", "line", line[:min(50, len(line))])
	}
}

// waitForLine reads lines until one matches the expected line.
func (h *HostnamePlugin) waitForLine(expected string) {
	for h.input.Scan() {
		line := h.input.Text()
		if line == expected {
			return
		}
	}
}

// send sends raw output to ze.
func (h *HostnamePlugin) send(format string, args ...any) {
	h.outputMu.Lock()
	_, _ = fmt.Fprintf(h.output, format+"\n", args...)
	h.outputMu.Unlock()
}

// GetYANG returns the embedded YANG schema for the hostname plugin.
func GetYANG() string {
	return hostnameYANG
}

// DecodableCapabilities returns the capability codes this plugin can decode.
func DecodableCapabilities() []uint8 {
	return []uint8{73} // FQDN capability
}

// RunDecodeMode runs the plugin in decode mode for ze bgp decode.
// Reads decode requests from stdin, writes JSON responses to stdout.
// Format: "decode capability <code> <hex>" → "decoded json <json>" or "decoded unknown".
func RunDecodeMode(input io.Reader, output io.Writer) int {
	writeUnknown := func() { _, _ = fmt.Fprintf(output, "decoded unknown\n") }

	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Parse: "decode capability <code> <hex>"
		parts := strings.Fields(line)
		if len(parts) < 4 || parts[0] != "decode" || parts[1] != "capability" {
			writeUnknown()
			continue
		}

		// Check capability code
		if parts[2] != "73" {
			writeUnknown()
			continue
		}

		// Decode hex payload
		hexData := parts[3]
		data, err := hex.DecodeString(hexData)
		if err != nil {
			writeUnknown()
			continue
		}

		// Parse FQDN capability value
		result := decodeFQDN(data)
		if result == nil {
			writeUnknown()
			continue
		}

		// Output JSON
		jsonBytes, err := json.Marshal(result)
		if err != nil {
			writeUnknown()
			continue
		}
		_, _ = fmt.Fprintf(output, "decoded json %s\n", jsonBytes)
	}
	return 0
}

// decodeFQDN decodes FQDN capability wire bytes to JSON map.
// Wire format: hostname-len (1) + hostname + domain-len (1) + domain.
func decodeFQDN(data []byte) map[string]any {
	if len(data) < 1 {
		return nil
	}

	hostLen := int(data[0])
	if len(data) < 1+hostLen+1 {
		return nil
	}

	hostname := string(data[1 : 1+hostLen])

	domainLen := int(data[1+hostLen])
	if len(data) < 1+hostLen+1+domainLen {
		return nil
	}

	domain := string(data[2+hostLen : 2+hostLen+domainLen])

	return map[string]any{
		"name":     "fqdn",
		"hostname": hostname,
		"domain":   domain,
	}
}

// hostnameYANG is the embedded YANG schema.
const hostnameYANG = `module ze-hostname {
    namespace "urn:ze:hostname";
    prefix hostname;

    import ze-bgp { prefix ze-bgp; }

    description
        "FQDN capability plugin for ZeBGP (draft-walton-bgp-hostname, code 73).
         Advertises hostname and domain name of the BGP speaker.";

    revision 2025-01-29 {
        description "Initial revision.";
    }

    // New syntax: augment capability container
    augment "/ze-bgp:bgp/ze-bgp:peer/ze-bgp:capability" {
        container hostname {
            description "FQDN capability configuration.";

            leaf host {
                type string {
                    length "0..255";
                }
                description "System hostname (max 255 bytes).";
            }

            leaf domain {
                type string {
                    length "0..255";
                }
                description "Domain name (max 255 bytes).";
            }
        }
    }

    // Legacy syntax: augment peer level for backwards compatibility
    augment "/ze-bgp:bgp/ze-bgp:peer" {
        leaf host-name {
            type string {
                length "0..255";
            }
            description "Legacy: Host name for FQDN capability.";
        }

        leaf domain-name {
            type string {
                length "0..255";
            }
            description "Legacy: Domain name for FQDN capability.";
        }
    }
}
`

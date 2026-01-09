// Package api implements plugin registration protocol for ZeBGP.
//
// This file implements the 5-stage plugin registration protocol:
//   - Stage 1: Registration (Plugin → ZeBGP) - rfc, encoding, family, conf, cmd
//   - Stage 2: Config Delivery (ZeBGP → Plugin) - configuration lines
//   - Stage 3: Open Capability (Plugin → ZeBGP) - capability bytes for OPEN
//   - Stage 4: Registry Sharing (ZeBGP → Plugin) - api name, all commands
//   - Stage 5: Ready (Plugin → ZeBGP) - ready signal
package api

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// PluginStage represents the current stage in the plugin startup protocol.
type PluginStage int

const (
	StageInit         PluginStage = iota // Not started
	StageRegistration                    // Stage 1: Plugin registering capabilities
	StageConfig                          // Stage 2: ZeBGP delivering config
	StageCapability                      // Stage 3: Plugin declaring OPEN capabilities
	StageRegistry                        // Stage 4: ZeBGP sharing command registry
	StageReady                           // Stage 5: Plugin signaling ready
	StageRunning                         // Normal operation
)

// String returns a human-readable stage name.
func (s PluginStage) String() string {
	switch s {
	case StageInit:
		return "Init"
	case StageRegistration:
		return "Registration"
	case StageConfig:
		return "Config"
	case StageCapability:
		return "Capability"
	case StageRegistry:
		return "Registry"
	case StageReady:
		return "Ready"
	case StageRunning:
		return "Running"
	default:
		return fmt.Sprintf("Unknown(%d)", s)
	}
}

// Valid encoding names for plugin registration.
var validEncodings = map[string]bool{
	"text": true,
	"b64":  true,
	"hex":  true,
}

// Valid AFI names for family registration.
var validAFIs = map[string]bool{
	"ipv4":  true,
	"ipv6":  true,
	"l2vpn": true,
	"all":   true,
}

// Valid SAFI names for family registration.
var validSAFIs = map[string]bool{
	"unicast":   true,
	"multicast": true,
	"mpls-vpn":  true,
	"nlri-mpls": true,
	"flowspec":  true,
	"evpn":      true,
	"mup":       true,
}

// PluginRegistration holds Stage 1 registration data from a plugin.
type PluginRegistration struct {
	Name           string           // Plugin name (set after Stage 4)
	RFCs           []uint16         // RFC numbers for human-readable feature tracking
	Encodings      []string         // Supported encodings (text, b64, hex)
	Families       []string         // Address families (e.g., "ipv4/unicast", "all")
	ConfigPatterns []*ConfigPattern // Config patterns with captures
	Commands       []string         // Command names to register
	Done           bool             // True when "registration done" received
}

// ConfigPattern represents a config hook pattern with regex captures.
type ConfigPattern struct {
	Pattern  string         // Original pattern string
	Regex    *regexp.Regexp // Compiled regex for matching
	Captures []string       // Named capture groups in order
	Literals []string       // Literal parts between captures
}

// ConfigMatch represents a successful pattern match with captured values.
type ConfigMatch struct {
	Pattern  *ConfigPattern    // The pattern that matched
	Captures map[string]string // Captured values by name
	Context  string            // The context (e.g., "peer 192.168.1.1")
}

// PluginCapability represents a capability declaration from Stage 3.
type PluginCapability struct {
	Code     uint8  // Capability type code
	Encoding string // Encoding of payload (b64, hex, text)
	Payload  string // Encoded capability value
}

// PluginCapabilities holds Stage 3 capability declarations.
type PluginCapabilities struct {
	PluginName   string             // Plugin name
	Capabilities []PluginCapability // Declared capabilities
	Done         bool               // True when "open done" received
}

// PluginRegistry tracks all registered plugins and detects conflicts.
type PluginRegistry struct {
	mu           sync.RWMutex
	plugins      map[string]*PluginRegistration
	commands     map[string]string // command → plugin name
	capabilities map[uint8]string  // capability code → plugin name
}

// NewPluginRegistry creates a new plugin registry.
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		plugins:      make(map[string]*PluginRegistration),
		commands:     make(map[string]string),
		capabilities: make(map[uint8]string),
	}
}

// Register adds a plugin registration, checking for conflicts.
func (r *PluginRegistry) Register(reg *PluginRegistration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check command conflicts
	for _, cmd := range reg.Commands {
		cmdKey := strings.ToLower(cmd)
		if existing, ok := r.commands[cmdKey]; ok {
			return fmt.Errorf("command conflict: %q already registered by %s", cmd, existing)
		}
	}

	// Register commands
	for _, cmd := range reg.Commands {
		cmdKey := strings.ToLower(cmd)
		r.commands[cmdKey] = reg.Name
	}

	r.plugins[reg.Name] = reg
	return nil
}

// RegisterCapabilities adds capability declarations, checking for conflicts.
func (r *PluginRegistry) RegisterCapabilities(caps *PluginCapabilities) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check capability conflicts
	for _, cap := range caps.Capabilities {
		if existing, ok := r.capabilities[cap.Code]; ok {
			return fmt.Errorf("capability conflict: code %d already registered by %s", cap.Code, existing)
		}
	}

	// Register capabilities
	for _, cap := range caps.Capabilities {
		r.capabilities[cap.Code] = caps.PluginName
	}

	return nil
}

// ParseLine parses a single registration command line.
func (reg *PluginRegistration) ParseLine(line string) error {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	parts := strings.Fields(line)
	if len(parts) < 2 {
		return fmt.Errorf("invalid registration command: %s", line)
	}

	switch parts[0] {
	case "rfc":
		return reg.parseRFC(parts[1:])
	case "encoding":
		return reg.parseEncoding(parts[1:])
	case "family":
		return reg.parseFamily(parts[1:])
	case "conf":
		return reg.parseConf(parts[1:], line)
	case "cmd":
		return reg.parseCmd(parts[1:], line)
	case "registration":
		if len(parts) >= 2 && parts[1] == "done" {
			reg.Done = true
			return nil
		}
		return fmt.Errorf("expected 'registration done', got: %s", line)
	default:
		return fmt.Errorf("unknown registration command: %s", parts[0])
	}
}

// parseRFC handles "rfc add <number>".
func (reg *PluginRegistration) parseRFC(args []string) error {
	if len(args) < 2 || args[0] != "add" {
		return fmt.Errorf("expected 'rfc add <number>'")
	}

	n, err := strconv.ParseUint(args[1], 10, 16)
	if err != nil {
		return fmt.Errorf("invalid RFC number: %s", args[1])
	}

	reg.RFCs = append(reg.RFCs, uint16(n))
	return nil
}

// parseEncoding handles "encoding add <enc>".
func (reg *PluginRegistration) parseEncoding(args []string) error {
	if len(args) < 2 || args[0] != "add" {
		return fmt.Errorf("expected 'encoding add <text|b64|hex>'")
	}

	enc := strings.ToLower(args[1])
	if !validEncodings[enc] {
		return fmt.Errorf("invalid encoding: %s (valid: text, b64, hex)", args[1])
	}

	reg.Encodings = append(reg.Encodings, enc)
	return nil
}

// parseFamily handles "family add <afi> <safi>" or "family add all".
func (reg *PluginRegistration) parseFamily(args []string) error {
	if len(args) < 2 || args[0] != "add" {
		return fmt.Errorf("expected 'family add <afi> <safi>' or 'family add all'")
	}

	afi := strings.ToLower(args[1])

	// Handle "family add all"
	if afi == "all" {
		reg.Families = append(reg.Families, "all")
		return nil
	}

	// Validate AFI
	if !validAFIs[afi] {
		return fmt.Errorf("invalid AFI: %s (valid: ipv4, ipv6, l2vpn, all)", args[1])
	}

	// Need SAFI for non-all
	if len(args) < 3 {
		return fmt.Errorf("expected 'family add %s <safi>'", afi)
	}

	safi := strings.ToLower(args[2])
	if !validSAFIs[safi] {
		return fmt.Errorf("invalid SAFI: %s", args[2])
	}

	reg.Families = append(reg.Families, afi+"/"+safi)
	return nil
}

// parseConf handles "conf add <pattern>".
func (reg *PluginRegistration) parseConf(args []string, line string) error {
	if len(args) < 2 || args[0] != "add" {
		return fmt.Errorf("expected 'conf add <pattern>'")
	}

	// Extract pattern from original line to preserve spacing
	idx := strings.Index(line, "conf add ")
	if idx < 0 {
		return fmt.Errorf("malformed conf command")
	}
	patternStr := strings.TrimSpace(line[idx+len("conf add "):])

	pat, err := CompileConfigPattern(patternStr)
	if err != nil {
		return fmt.Errorf("invalid config pattern: %w", err)
	}

	reg.ConfigPatterns = append(reg.ConfigPatterns, pat)
	return nil
}

// parseCmd handles "cmd add <command>".
func (reg *PluginRegistration) parseCmd(args []string, line string) error {
	if len(args) < 2 || args[0] != "add" {
		return fmt.Errorf("expected 'cmd add <command>'")
	}

	// Extract command from original line to preserve spacing
	idx := strings.Index(line, "cmd add ")
	if idx < 0 {
		return fmt.Errorf("malformed cmd command")
	}
	cmd := strings.TrimSpace(line[idx+len("cmd add "):])

	reg.Commands = append(reg.Commands, cmd)
	return nil
}

// CompileConfigPattern compiles a config pattern string into a ConfigPattern.
// Pattern syntax:
//   - "*" matches any single path element (not slashes/spaces)
//   - "<name:regex>" is a named capture with validation regex
//
// Example: "peer * capability hostname <hostname:.*>"
func CompileConfigPattern(pattern string) (*ConfigPattern, error) {
	pat := &ConfigPattern{
		Pattern:  pattern,
		Captures: make([]string, 0),
	}

	// Build regex from pattern
	var regexParts []string
	remaining := pattern

	for len(remaining) > 0 {
		// Look for capture group
		captureStart := strings.Index(remaining, "<")
		if captureStart < 0 {
			// No more captures - escape rest as literal
			regexParts = append(regexParts, convertGlobToRegex(remaining))
			break
		}

		// Add literal before capture
		if captureStart > 0 {
			regexParts = append(regexParts, convertGlobToRegex(remaining[:captureStart]))
		}

		// Find capture end
		captureEnd := strings.Index(remaining[captureStart:], ">")
		if captureEnd < 0 {
			return nil, fmt.Errorf("unclosed capture group in pattern: %s", pattern)
		}
		captureEnd += captureStart

		// Parse capture: <name:regex>
		capture := remaining[captureStart+1 : captureEnd]
		colonIdx := strings.Index(capture, ":")
		if colonIdx < 0 {
			return nil, fmt.Errorf("capture missing regex: <%s>", capture)
		}

		captureName := capture[:colonIdx]
		captureRegex := capture[colonIdx+1:]

		// Validate regex
		if _, err := regexp.Compile(captureRegex); err != nil {
			return nil, fmt.Errorf("invalid regex in capture <%s>: %w", capture, err)
		}

		pat.Captures = append(pat.Captures, captureName)
		// Go regex requires alphanumeric names - replace hyphens with underscores
		regexName := strings.ReplaceAll(captureName, "-", "_")
		regexParts = append(regexParts, "(?P<"+regexName+">"+captureRegex+")")

		remaining = remaining[captureEnd+1:]
	}

	// Compile full regex
	fullRegex := "^" + strings.Join(regexParts, "") + "$"
	var err error
	pat.Regex, err = regexp.Compile(fullRegex)
	if err != nil {
		return nil, fmt.Errorf("failed to compile pattern regex: %w", err)
	}

	return pat, nil
}

// convertGlobToRegex converts glob wildcards to regex.
func convertGlobToRegex(s string) string {
	// Escape regex special chars except *
	var result strings.Builder
	for _, c := range s {
		switch c {
		case '*':
			result.WriteString(`\S+`) // Match non-whitespace
		case '.', '^', '$', '+', '?', '{', '}', '[', ']', '|', '(', ')', '\\':
			result.WriteRune('\\')
			result.WriteRune(c)
		default:
			result.WriteRune(c)
		}
	}
	return result.String()
}

// Match tests if a config line matches this pattern.
// Returns nil if no match, otherwise returns the captured values.
func (pat *ConfigPattern) Match(config string) *ConfigMatch {
	matches := pat.Regex.FindStringSubmatch(config)
	if matches == nil {
		return nil
	}

	result := &ConfigMatch{
		Pattern:  pat,
		Captures: make(map[string]string),
	}

	// Extract named captures - map regex names (underscores) back to original names (hyphens)
	regexNames := pat.Regex.SubexpNames()
	for i, regexName := range regexNames {
		if regexName != "" && i < len(matches) {
			// Find the original capture name with hyphens
			originalName := regexName
			for _, capName := range pat.Captures {
				if strings.ReplaceAll(capName, "-", "_") == regexName {
					originalName = capName
					break
				}
			}
			result.Captures[originalName] = matches[i]
		}
	}

	return result
}

// ParseLine parses a Stage 3 capability command.
func (caps *PluginCapabilities) ParseLine(line string) error {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	parts := strings.Fields(line)
	if len(parts) < 2 {
		return fmt.Errorf("invalid capability command: %s", line)
	}

	if parts[0] != "open" {
		return fmt.Errorf("expected 'open' command, got: %s", parts[0])
	}

	// Handle "open done"
	if parts[1] == "done" {
		caps.Done = true
		return nil
	}

	// Parse "open <enc> capability <code> set <payload>"
	if len(parts) < 6 {
		return fmt.Errorf("expected 'open <enc> capability <code> set <payload>'")
	}

	enc := parts[1]
	if !validEncodings[enc] {
		return fmt.Errorf("invalid encoding: %s", enc)
	}

	if parts[2] != "capability" {
		return fmt.Errorf("expected 'capability', got: %s", parts[2])
	}

	code, err := strconv.ParseUint(parts[3], 10, 8)
	if err != nil {
		return fmt.Errorf("invalid capability code: %s", parts[3])
	}

	if parts[4] != "set" {
		return fmt.Errorf("expected 'set', got: %s", parts[4])
	}

	if len(parts) < 6 {
		return fmt.Errorf("missing capability payload")
	}

	payload := parts[5]

	caps.Capabilities = append(caps.Capabilities, PluginCapability{
		Code:     uint8(code),
		Encoding: enc,
		Payload:  payload,
	})

	return nil
}

// FormatConfigDelivery formats a config match for delivery to plugin.
// Format: "configuration <context> <name> set <value>"
func FormatConfigDelivery(context, name, value string) string {
	return fmt.Sprintf("configuration %s %s set %s", context, name, value)
}

// FormatRegistrySharing formats the registry sharing messages for Stage 4.
// Returns lines to send to the plugin.
func FormatRegistrySharing(pluginName string, allCommands map[string][]PluginCommandInfo) []string {
	lines := make([]string, 0)

	// First line: api name <plugin-name>
	lines = append(lines, "api name "+pluginName)

	// Then all commands from all plugins
	for pname, cmds := range allCommands {
		for _, cmd := range cmds {
			lines = append(lines, fmt.Sprintf("api %s %s cmd %s", pname, cmd.Encoding, cmd.Command))
		}
	}

	// End marker
	lines = append(lines, "api done")

	return lines
}

// PluginCommandInfo holds info about a registered command for sharing.
type PluginCommandInfo struct {
	Command  string
	Encoding string
}

// BuildCommandInfo builds the command info map for registry sharing.
func (r *PluginRegistry) BuildCommandInfo() map[string][]PluginCommandInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]PluginCommandInfo)

	for name, reg := range r.plugins {
		var cmds []PluginCommandInfo
		// Use first encoding as default
		encoding := "text"
		if len(reg.Encodings) > 0 {
			encoding = reg.Encodings[0]
		}

		for _, cmd := range reg.Commands {
			cmds = append(cmds, PluginCommandInfo{
				Command:  cmd,
				Encoding: encoding,
			})
		}
		result[name] = cmds
	}

	return result
}

// LookupCommand finds which plugin registered a command.
// Returns empty string if not found.
func (r *PluginRegistry) LookupCommand(cmd string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.commands[strings.ToLower(cmd)]
}

// InjectedCapability represents a decoded capability ready for OPEN injection.
type InjectedCapability struct {
	Code   uint8
	Value  []byte
	Plugin string
}

// CapabilityInjector collects and manages plugin capabilities for OPEN messages.
type CapabilityInjector struct {
	mu           sync.RWMutex
	capabilities []InjectedCapability
	byCode       map[uint8]string // code → plugin name
}

// NewCapabilityInjector creates a new capability injector.
func NewCapabilityInjector() *CapabilityInjector {
	return &CapabilityInjector{
		byCode: make(map[uint8]string),
	}
}

// AddPluginCapabilities adds capabilities from a plugin, checking for conflicts.
func (ci *CapabilityInjector) AddPluginCapabilities(caps *PluginCapabilities) error {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	for _, cap := range caps.Capabilities {
		// Check conflict
		if existing, ok := ci.byCode[cap.Code]; ok {
			return fmt.Errorf("capability conflict: code %d already registered by %s", cap.Code, existing)
		}

		// Decode payload
		value, err := DecodeCapabilityPayload(cap)
		if err != nil {
			return err
		}

		ci.capabilities = append(ci.capabilities, InjectedCapability{
			Code:   cap.Code,
			Value:  value,
			Plugin: caps.PluginName,
		})
		ci.byCode[cap.Code] = caps.PluginName
	}
	return nil
}

// GetCapabilities returns all capabilities to inject into OPEN.
func (ci *CapabilityInjector) GetCapabilities() []InjectedCapability {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	result := make([]InjectedCapability, len(ci.capabilities))
	copy(result, ci.capabilities)
	return result
}

// DecodeCapabilityPayload decodes a plugin capability payload.
func DecodeCapabilityPayload(cap PluginCapability) ([]byte, error) {
	switch cap.Encoding {
	case "b64":
		return base64.StdEncoding.DecodeString(cap.Payload)
	case "hex":
		return hex.DecodeString(cap.Payload)
	case "text":
		return []byte(cap.Payload), nil
	default:
		return nil, fmt.Errorf("unknown encoding: %s", cap.Encoding)
	}
}

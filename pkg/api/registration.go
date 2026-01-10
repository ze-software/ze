// Package api implements plugin registration protocol for ZeBGP.
//
// This file implements the 5-stage plugin registration protocol:
//   - Stage 1: Declaration (Plugin → ZeBGP) - declare rfc, encoding, family, conf, cmd
//   - Stage 2: Config Delivery (ZeBGP → Plugin) - config lines
//   - Stage 3: Capability (Plugin → ZeBGP) - capability bytes for OPEN
//   - Stage 4: Registry Sharing (ZeBGP → Plugin) - registry name, all commands
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
// Stage 1 commands: "declare rfc|encoding|family|conf|cmd|done ...".
func (reg *PluginRegistration) ParseLine(line string) error {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	parts := strings.Fields(line)
	if len(parts) < 2 {
		return fmt.Errorf("invalid registration command: %s", line)
	}

	// All Stage 1 commands start with "declare"
	if parts[0] != "declare" {
		return fmt.Errorf("expected 'declare' command, got: %s", parts[0])
	}

	switch parts[1] {
	case "rfc":
		return reg.parseRFC(parts[2:])
	case "encoding":
		return reg.parseEncoding(parts[2:])
	case "family":
		return reg.parseFamily(parts[2:])
	case "conf":
		return reg.parseConf(parts[2:], line)
	case "cmd":
		return reg.parseCmd(parts[2:], line)
	case statusDone:
		reg.Done = true
		return nil
	default:
		return fmt.Errorf("unknown declare command: %s", parts[1])
	}
}

// parseRFC handles "declare rfc <number>".
func (reg *PluginRegistration) parseRFC(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("expected 'declare rfc <number>'")
	}

	n, err := strconv.ParseUint(args[0], 10, 16)
	if err != nil {
		return fmt.Errorf("invalid RFC number: %s", args[0])
	}

	reg.RFCs = append(reg.RFCs, uint16(n))
	return nil
}

// parseEncoding handles "declare encoding <enc>".
func (reg *PluginRegistration) parseEncoding(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("expected 'declare encoding <text|b64|hex>'")
	}

	enc := strings.ToLower(args[0])
	if !validEncodings[enc] {
		return fmt.Errorf("invalid encoding: %s (valid: text, b64, hex)", args[0])
	}

	reg.Encodings = append(reg.Encodings, enc)
	return nil
}

// parseFamily handles "declare family <afi> <safi>" or "declare family all".
func (reg *PluginRegistration) parseFamily(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("expected 'declare family <afi> <safi>' or 'declare family all'")
	}

	afi := strings.ToLower(args[0])

	// Handle "declare family all"
	if afi == "all" {
		reg.Families = append(reg.Families, "all")
		return nil
	}

	// Validate AFI
	if !validAFIs[afi] {
		return fmt.Errorf("invalid AFI: %s (valid: ipv4, ipv6, l2vpn, all)", args[0])
	}

	// Need SAFI for non-all
	if len(args) < 2 {
		return fmt.Errorf("expected 'declare family %s <safi>'", afi)
	}

	safi := strings.ToLower(args[1])
	if !validSAFIs[safi] {
		return fmt.Errorf("invalid SAFI: %s", args[1])
	}

	reg.Families = append(reg.Families, afi+"/"+safi)
	return nil
}

// parseConf handles "declare conf <pattern>".
func (reg *PluginRegistration) parseConf(args []string, line string) error {
	if len(args) < 1 {
		return fmt.Errorf("expected 'declare conf <pattern>'")
	}

	// Extract pattern from original line to preserve spacing
	idx := strings.Index(line, "declare conf ")
	if idx < 0 {
		return fmt.Errorf("malformed conf command")
	}
	patternStr := strings.TrimSpace(line[idx+len("declare conf "):])

	pat, err := CompileConfigPattern(patternStr)
	if err != nil {
		return fmt.Errorf("invalid config pattern: %w", err)
	}

	reg.ConfigPatterns = append(reg.ConfigPatterns, pat)
	return nil
}

// parseCmd handles "declare cmd <command>".
func (reg *PluginRegistration) parseCmd(args []string, line string) error {
	if len(args) < 1 {
		return fmt.Errorf("expected 'declare cmd <command>'")
	}

	// Extract command from original line to preserve spacing
	idx := strings.Index(line, "declare cmd ")
	if idx < 0 {
		return fmt.Errorf("malformed cmd command")
	}
	cmd := strings.TrimSpace(line[idx+len("declare cmd "):])

	reg.Commands = append(reg.Commands, cmd)
	return nil
}

// CompileConfigPattern compiles a config pattern string into a ConfigPattern.
// Pattern syntax:
//   - "*" matches any single path element (not slashes/spaces)
//   - "<name:regex>" is a named capture with validation regex
//
// Example: "peer * capability hostname <hostname:.*>".
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
// Commands: "capability <enc> <code> <payload>" or "capability done".
func (caps *PluginCapabilities) ParseLine(line string) error {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	parts := strings.Fields(line)
	if len(parts) < 2 {
		return fmt.Errorf("invalid capability command: %s", line)
	}

	if parts[0] != "capability" {
		return fmt.Errorf("expected 'capability' command, got: %s", parts[0])
	}

	// Handle "capability done"
	if parts[1] == "done" {
		caps.Done = true
		return nil
	}

	// Parse "capability <enc> <code> <payload>"
	if len(parts) < 4 {
		return fmt.Errorf("expected 'capability <enc> <code> <payload>'")
	}

	enc := parts[1]
	if !validEncodings[enc] {
		return fmt.Errorf("invalid encoding: %s", enc)
	}

	code, err := strconv.ParseUint(parts[2], 10, 8)
	if err != nil {
		return fmt.Errorf("invalid capability code: %s", parts[2])
	}

	payload := parts[3]

	caps.Capabilities = append(caps.Capabilities, PluginCapability{
		Code:     uint8(code),
		Encoding: enc,
		Payload:  payload,
	})

	return nil
}

// FormatConfigDelivery formats a config match for delivery to plugin.
// Format: "config <context> <name> <value>".
func FormatConfigDelivery(context, name, value string) string {
	return fmt.Sprintf("config %s %s %s", context, name, value)
}

// FormatRegistrySharing formats the registry sharing messages for Stage 4.
// Returns lines to send to the plugin.
func FormatRegistrySharing(pluginName string, allCommands map[string][]PluginCommandInfo) []string {
	// Calculate capacity: 1 (name) + sum(commands) + 1 (done)
	totalCmds := 0
	for _, cmds := range allCommands {
		totalCmds += len(cmds)
	}
	lines := make([]string, 0, 2+totalCmds)

	// First line: registry name <plugin-name>
	lines = append(lines, "registry name "+pluginName)

	// Then all commands from all plugins
	for pname, cmds := range allCommands {
		for _, cmd := range cmds {
			lines = append(lines, fmt.Sprintf("registry %s %s cmd %s", pname, cmd.Encoding, cmd.Command))
		}
	}

	// End marker
	lines = append(lines, "registry done")

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
		cmds := make([]PluginCommandInfo, 0, len(reg.Commands))
		// Use first encoding as default
		encoding := EncodingText
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
	case wireEncB64:
		return base64.StdEncoding.DecodeString(cap.Payload)
	case wireEncHex:
		return hex.DecodeString(cap.Payload)
	case EncodingText:
		return []byte(cap.Payload), nil
	default:
		return nil, fmt.Errorf("unknown encoding: %s", cap.Encoding)
	}
}

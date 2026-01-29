// Package api implements plugin registration protocol for ze.
//
// This file implements the 5-stage plugin registration protocol:
//   - Stage 1: Declaration (Plugin → ze) - declare rfc, encoding, family, conf, cmd
//   - Stage 2: Config Delivery (ze → Plugin) - config lines
//   - Stage 3: Capability (Plugin → ze) - capability bytes for OPEN
//   - Stage 4: Registry Sharing (ze → Plugin) - registry name, all commands
//   - Stage 5: Ready (Plugin → ze) - ready signal
package plugin

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// PluginStage represents the current stage in the plugin startup protocol.
type PluginStage int

const (
	StageInit         PluginStage = iota // Not started
	StageRegistration                    // Stage 1: Plugin registering capabilities
	StageConfig                          // Stage 2: ze delivering config
	StageCapability                      // Stage 3: Plugin declaring OPEN capabilities
	StageRegistry                        // Stage 4: ze sharing command registry
	StageReady                           // Stage 5: Plugin signaling ready
	StageRunning                         // Normal operation
)

// Stage 3 capability command keywords.
const capKeywordPeer = "peer" // Keyword for per-peer capability: "capability ... peer <addr>"

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

// Valid receive types for message subscription.
var validReceiveTypes = map[string]bool{
	"update":       true,
	"open":         true,
	"notification": true,
	"keepalive":    true,
	"refresh":      true,
	"state":        true,
	"sent":         true,
	"negotiated":   true,
	"all":          true,
}

// PluginRegistration holds Stage 1 registration data from a plugin.
type PluginRegistration struct {
	Name               string              // Plugin name (set after Stage 4)
	RFCs               []uint16            // RFC numbers for human-readable feature tracking
	Encodings          []string            // Supported encodings (text, b64, hex)
	Families           []string            // Address families (e.g., "ipv4/unicast", "all")
	Commands           []string            // Command names to register
	Receive            []string            // Message types to receive (update, open, negotiated, etc.)
	SchemaDeclarations []SchemaDeclaration // Schema extensions for capability config
	WantsConfigRoots   []string            // Config roots to receive (e.g., ["bgp", "environment"] via "declare wants config <root>")
	Done               bool                // True when "registration done" received

	// YANG schema declarations (Hub Architecture)
	PluginSchema *PluginSchemaDecl // YANG schema declaration for this plugin
}

// PluginSchemaDecl holds YANG schema declaration from a plugin.
// Built incrementally from multiple `declare schema` lines.
type PluginSchemaDecl struct {
	Module    string   // YANG module name
	Namespace string   // YANG namespace URI
	Handlers  []string // Handler paths (e.g., "bgp", "bgp.peer")
	Yang      string   // Full YANG module text
	Priority  int      // Config ordering (lower = processed first, default 1000)
}

// SchemaDeclaration represents a plugin's config schema extension.
// Used to add capability sub-blocks to the config schema at runtime.
// Format: "declare conf schema capability <name> { <field> <type>; ... }".
type SchemaDeclaration struct {
	Path   string            // Location in schema (e.g., "capability.graceful-restart")
	Name   string            // Capability name (e.g., "graceful-restart")
	Fields map[string]string // field name -> type (e.g., "restart-time" -> "uint16")
}

// PluginCapability represents a capability declaration from Stage 3.
// Per-peer capabilities use Peers to scope to specific peers.
type PluginCapability struct {
	Code     uint8    // Capability type code
	Encoding string   // Encoding of payload (b64, hex, text)
	Payload  string   // Encoded capability value
	Peers    []string // Optional peer addresses (empty = global/all peers)
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
	case "receive":
		return reg.parseReceive(parts[2:])
	case "schema":
		return reg.parseSchema(parts[2:], line)
	case "priority":
		return reg.parsePriority(parts[2:])
	case "wants":
		return reg.parseWants(parts[2:])
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

// parseConf handles "declare conf schema ...".
// Pattern-based config delivery is removed - use "declare wants config <root>" instead.
func (reg *PluginRegistration) parseConf(args []string, line string) error {
	if len(args) < 1 {
		return fmt.Errorf("expected 'declare conf schema ...'")
	}

	// Only schema declarations are supported
	if args[0] == "schema" {
		return reg.parseConfSchema(args[1:], line)
	}

	return fmt.Errorf("pattern-based config delivery removed; use 'declare wants config <root>' instead")
}

// parseConfSchema handles "declare conf schema capability <name> { <field> <type>; ... }".
// Format: declare conf schema capability graceful-restart { restart-time <restart-time:\d+>; }
// This tells the engine to add a capability sub-block to the config schema.
func (reg *PluginRegistration) parseConfSchema(args []string, line string) error {
	if len(args) < 2 {
		return fmt.Errorf("expected 'declare conf schema capability <name> { ... }'")
	}

	// Currently only support "capability" path
	if args[0] != "capability" {
		return fmt.Errorf("schema declaration only supports 'capability' path, got: %s", args[0])
	}

	capName := args[1]

	// Extract the block content from the line: { field <name:regex>; ... }
	blockStart := strings.Index(line, "{")
	blockEnd := strings.LastIndex(line, "}")
	if blockStart < 0 || blockEnd < 0 || blockEnd <= blockStart {
		return fmt.Errorf("expected schema block: { field <type>; ... }")
	}

	blockContent := strings.TrimSpace(line[blockStart+1 : blockEnd])

	// Parse fields from block: "restart-time <restart-time:\d+>;"
	fields := make(map[string]string)
	for _, entry := range strings.Split(blockContent, ";") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		// Parse field: "restart-time <restart-time:\d+>"
		parts := strings.Fields(entry)
		if len(parts) < 2 {
			return fmt.Errorf("invalid schema field: %s", entry)
		}

		fieldName := parts[0]

		// Extract type from <name:regex> pattern
		typeSpec := parts[1]
		if !strings.HasPrefix(typeSpec, "<") || !strings.HasSuffix(typeSpec, ">") {
			return fmt.Errorf("expected <name:regex> pattern for field %s", fieldName)
		}

		// Extract the capture name which hints at the type
		inner := typeSpec[1 : len(typeSpec)-1]
		colonIdx := strings.Index(inner, ":")
		if colonIdx < 0 {
			return fmt.Errorf("expected <name:regex> pattern, got: %s", typeSpec)
		}

		captureRegex := inner[colonIdx+1:]

		// Infer type from regex pattern
		fieldType := inferFieldType(captureRegex)
		fields[fieldName] = fieldType
	}

	reg.SchemaDeclarations = append(reg.SchemaDeclarations, SchemaDeclaration{
		Path:   "capability." + capName,
		Name:   capName,
		Fields: fields,
	})

	return nil
}

// inferFieldType infers a config schema type from a regex pattern.
func inferFieldType(regex string) string {
	switch regex {
	case `\d+`:
		return "uint16" // Numeric values default to uint16
	case `\d{1,5}`:
		return "uint16"
	case `\d{1,10}`:
		return "uint32"
	case `true|false`:
		return "bool"
	default:
		return "string"
	}
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

// parseReceive handles "declare receive <type>".
// Valid types: update, open, notification, keepalive, refresh, state, sent, negotiated, all.
func (reg *PluginRegistration) parseReceive(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("expected 'declare receive <type>'")
	}

	recvType := strings.ToLower(args[0])
	if !validReceiveTypes[recvType] {
		return fmt.Errorf("invalid receive type: %s (valid: update, open, notification, keepalive, refresh, state, sent, negotiated, all)", args[0])
	}

	reg.Receive = append(reg.Receive, recvType)
	return nil
}

// parsePriority handles "declare priority <number>".
// Lower priority = processed first during config verify/apply.
// Default is 1000 if not specified.
func (reg *PluginRegistration) parsePriority(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("expected 'declare priority <number>'")
	}

	n, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid priority: %s", args[0])
	}

	// Initialize schema if needed
	if reg.PluginSchema == nil {
		reg.PluginSchema = &PluginSchemaDecl{Priority: 1000}
	}
	reg.PluginSchema.Priority = n
	return nil
}

// parseWants handles "declare wants <what>".
// Supported:
//   - config <root>: receive specific config subtree as JSON (e.g., "bgp", "environment", "*").
func (reg *PluginRegistration) parseWants(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("expected 'declare wants config <root>'")
	}

	switch args[0] {
	case "config":
		// "declare wants config <root>" - request specific config subtree
		if len(args) < 2 {
			return fmt.Errorf("expected 'declare wants config <root>'")
		}
		root := args[1]
		reg.WantsConfigRoots = append(reg.WantsConfigRoots, root)
	default:
		return fmt.Errorf("unknown wants type: %s (valid: config)", args[0])
	}

	return nil
}

// parseSchema handles "declare schema <type> <value>".
// Types:
//   - module <name> - YANG module name
//   - namespace <uri> - YANG namespace
//   - handler <path> - handler path for config routing
//   - yang <content> - inline YANG (single line only; use StartHeredoc for multi-line)
func (reg *PluginRegistration) parseSchema(args []string, line string) error {
	if len(args) < 2 {
		return fmt.Errorf("expected 'declare schema <module|namespace|handler|yang> <value>'")
	}

	// Initialize schema if needed
	if reg.PluginSchema == nil {
		reg.PluginSchema = &PluginSchemaDecl{}
	}

	schemaType := args[0]
	switch schemaType {
	case "module":
		reg.PluginSchema.Module = args[1]
	case "namespace":
		reg.PluginSchema.Namespace = args[1]
	case "handler":
		reg.PluginSchema.Handlers = append(reg.PluginSchema.Handlers, args[1])
	case "yang":
		// For single-line yang, extract everything after "declare schema yang "
		idx := strings.Index(line, "declare schema yang ")
		if idx >= 0 {
			reg.PluginSchema.Yang = strings.TrimSpace(line[idx+len("declare schema yang "):])
		}
	default:
		return fmt.Errorf("unknown schema type: %s (valid: module, namespace, handler, yang)", schemaType)
	}

	return nil
}

// StartHeredoc checks if a line starts a heredoc and returns the delimiter.
// Format: "declare schema yang <<EOF" returns ("EOF", true).
func StartHeredoc(line string) (string, bool) {
	idx := strings.Index(line, "<<")
	if idx < 0 {
		return "", false
	}
	delimiter := strings.TrimSpace(line[idx+2:])
	if delimiter == "" {
		return "", false
	}
	return delimiter, true
}

// IsHeredocEnd checks if a line ends a heredoc with the given delimiter.
func IsHeredocEnd(line, delimiter string) bool {
	return strings.TrimSpace(line) == delimiter
}

// AppendHeredocLine appends a line to the YANG content being collected.
func (reg *PluginRegistration) AppendHeredocLine(line string) {
	if reg.PluginSchema == nil {
		reg.PluginSchema = &PluginSchemaDecl{}
	}
	if reg.PluginSchema.Yang != "" {
		reg.PluginSchema.Yang += "\n"
	}
	reg.PluginSchema.Yang += line
}

// ParseLine parses a Stage 3 capability command.
// Commands:
//   - "capability <enc> <code> [payload]" - global capability
//   - "capability <enc> <code> [payload] peer <addr> [<addr2> ...]" - per-peer capability
//   - "capability done"
//
// Payload is optional for capabilities with 0-length value (e.g., route-refresh).
// Multiple peer addresses can be specified to apply the same capability to multiple peers.
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

	// Parse "capability <enc> <code> [payload] [peer <addr> [<addr2> ...]]"
	// Payload is optional - some capabilities (e.g., route-refresh) have 0-length value.
	if len(parts) < 3 {
		return fmt.Errorf("expected 'capability <enc> <code> [payload] [peer <addr>...]'")
	}

	enc := parts[1]
	if !validEncodings[enc] {
		return fmt.Errorf("invalid encoding: %s", enc)
	}

	code, err := strconv.ParseUint(parts[2], 10, 8)
	if err != nil {
		return fmt.Errorf("invalid capability code: %s", parts[2])
	}

	// Parse remaining parts: [payload] [peer <addr> [<addr2> ...]]
	payload := ""
	var peers []string

	// Look for "peer" keyword to separate payload from peer addresses
	peerIdx := -1
	for i := 3; i < len(parts); i++ {
		if parts[i] == capKeywordPeer {
			peerIdx = i
			break
		}
	}

	if peerIdx >= 0 {
		// Has peer address(es)
		if peerIdx == 3 {
			// No payload: "capability hex 64 peer 192.168.1.1 192.168.1.2"
			payload = ""
		} else {
			// Payload before peer: "capability hex 64 <payload> peer 192.168.1.1"
			payload = parts[3]
		}
		if peerIdx+1 >= len(parts) {
			return fmt.Errorf("expected peer address after 'peer'")
		}
		// Collect all peer addresses after "peer" keyword
		peers = parts[peerIdx+1:]
	} else if len(parts) >= 4 {
		// No peer keyword, just payload
		payload = parts[3]
	}

	caps.Capabilities = append(caps.Capabilities, PluginCapability{
		Code:     uint8(code),
		Encoding: enc,
		Payload:  payload,
		Peers:    peers,
	})

	return nil
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
	Code     uint8
	Value    []byte
	Plugin   string
	PeerAddr string // Empty = global (applies to all peers)
}

// CapabilityInjector collects and manages plugin capabilities for OPEN messages.
// Supports both global capabilities (all peers) and per-peer capabilities.
type CapabilityInjector struct {
	mu           sync.RWMutex
	globalCaps   []InjectedCapability            // Capabilities for all peers
	peerCaps     map[string][]InjectedCapability // peerAddr → capabilities
	globalByCode map[uint8]string                // code → plugin name (global)
	peerByCode   map[string]map[uint8]string     // peerAddr → code → plugin name
}

// NewCapabilityInjector creates a new capability injector.
func NewCapabilityInjector() *CapabilityInjector {
	return &CapabilityInjector{
		globalByCode: make(map[uint8]string),
		peerCaps:     make(map[string][]InjectedCapability),
		peerByCode:   make(map[string]map[uint8]string),
	}
}

// AddPluginCapabilities adds capabilities from a plugin, checking for conflicts.
// Capabilities with Peers list are stored per-peer; others are stored globally.
func (ci *CapabilityInjector) AddPluginCapabilities(caps *PluginCapabilities) error {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	for _, cap := range caps.Capabilities {
		// Decode payload
		value, err := DecodeCapabilityPayload(cap)
		if err != nil {
			return err
		}

		if len(cap.Peers) == 0 {
			// Global capability - applies to all peers
			if existing, ok := ci.globalByCode[cap.Code]; ok {
				return fmt.Errorf("capability conflict: code %d already registered by %s", cap.Code, existing)
			}
			ci.globalCaps = append(ci.globalCaps, InjectedCapability{
				Code:   cap.Code,
				Value:  value,
				Plugin: caps.PluginName,
			})
			ci.globalByCode[cap.Code] = caps.PluginName
		} else {
			// Per-peer capability - add to each specified peer
			for _, peerAddr := range cap.Peers {
				if ci.peerByCode[peerAddr] == nil {
					ci.peerByCode[peerAddr] = make(map[uint8]string)
				}
				if existing, ok := ci.peerByCode[peerAddr][cap.Code]; ok {
					return fmt.Errorf("capability conflict: code %d for peer %s already registered by %s",
						cap.Code, peerAddr, existing)
				}
				ci.peerCaps[peerAddr] = append(ci.peerCaps[peerAddr], InjectedCapability{
					Code:     cap.Code,
					Value:    value,
					Plugin:   caps.PluginName,
					PeerAddr: peerAddr,
				})
				ci.peerByCode[peerAddr][cap.Code] = caps.PluginName
			}
		}
	}
	return nil
}

// GetCapabilities returns all global capabilities to inject into OPEN.
//
// Deprecated: Use GetCapabilitiesForPeer for per-peer capability support.
func (ci *CapabilityInjector) GetCapabilities() []InjectedCapability {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	result := make([]InjectedCapability, len(ci.globalCaps))
	copy(result, ci.globalCaps)
	return result
}

// GetCapabilitiesForPeer returns capabilities for a specific peer.
// Returns global capabilities plus any peer-specific capabilities.
// Per-peer capabilities override global capabilities with the same code.
func (ci *CapabilityInjector) GetCapabilitiesForPeer(peerAddr string) []InjectedCapability {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	// Start with global capabilities
	result := make([]InjectedCapability, 0, len(ci.globalCaps)+len(ci.peerCaps[peerAddr]))
	seenCodes := make(map[uint8]bool)

	// Add per-peer capabilities first (they take precedence)
	for _, cap := range ci.peerCaps[peerAddr] {
		result = append(result, cap)
		seenCodes[cap.Code] = true
	}

	// Add global capabilities that weren't overridden
	for _, cap := range ci.globalCaps {
		if !seenCodes[cap.Code] {
			result = append(result, cap)
		}
	}

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

// Design: docs/architecture/api/process-protocol.md — plugin process management
//
// Package plugin implements plugin registration types for ze.
//
// This file defines types and registry logic for the 5-stage plugin registration protocol.
// Text protocol parsing has been removed; see RPC-based registration in handler.go.
package plugin

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
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

// Family mode constants for declare family commands.
const (
	familyModeDecode = "decode"
	familyModeBoth   = "both"
)

// PluginRegistration holds Stage 1 registration data from a plugin.
type PluginRegistration struct {
	Name               string              // Plugin name (set after Stage 4)
	RFCs               []uint16            // RFC numbers for human-readable feature tracking
	Encodings          []string            // Supported encodings (text, b64, hex)
	Families           []string            // Address families (e.g., "ipv4/unicast", "all")
	DecodeFamilies     []string            // Families this plugin decodes (claimed via "declare family X decode")
	Commands           []string            // Command names to register
	Receive            []string            // Message types to receive (update, open, negotiated, etc.)
	SchemaDeclarations []SchemaDeclaration // Schema extensions for capability config
	WantsConfigRoots   []string            // Config roots to receive (e.g., ["bgp", "environment"] via "declare wants config <root>")
	WantsValidateOpen  bool                // Plugin wants to validate OPEN message pairs (validate-open callback)
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
	commands     map[string]string // command -> plugin name
	capabilities map[uint8]string  // capability code -> plugin name
	families     map[string]string // family -> plugin name (for decode claims)
}

// NewPluginRegistry creates a new plugin registry.
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		plugins:      make(map[string]*PluginRegistration),
		commands:     make(map[string]string),
		capabilities: make(map[uint8]string),
		families:     make(map[string]string),
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

	// Check family decode conflicts
	for _, family := range reg.DecodeFamilies {
		familyKey := strings.ToLower(family)
		if existing, ok := r.families[familyKey]; ok {
			return fmt.Errorf("family conflict: %s already registered by %s", family, existing)
		}
	}

	// Register commands
	for _, cmd := range reg.Commands {
		cmdKey := strings.ToLower(cmd)
		r.commands[cmdKey] = reg.Name
	}

	// Register family decode claims
	for _, family := range reg.DecodeFamilies {
		familyKey := strings.ToLower(family)
		r.families[familyKey] = reg.Name
	}

	r.plugins[reg.Name] = reg
	return nil
}

// LookupFamily finds which plugin registered to decode a family.
// Returns empty string if no plugin registered for the family.
// Family string is normalized to lowercase for lookup.
func (r *PluginRegistry) LookupFamily(family string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.families[strings.ToLower(family)]
}

// GetDecodeFamilies returns all families that have decode plugins registered.
// Used by Session to auto-add Multiprotocol capabilities in OPEN.
// Returns a sorted copy of the family strings (lowercase normalized).
// Sorted for deterministic OPEN message ordering.
func (r *PluginRegistry) GetDecodeFamilies() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	families := make([]string, 0, len(r.families))
	for family := range r.families {
		families = append(families, family)
	}
	sort.Strings(families)
	return families
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
	peerCaps     map[string][]InjectedCapability // peerAddr -> capabilities
	globalByCode map[uint8]string                // code -> plugin name (global)
	peerByCode   map[string]map[uint8]string     // peerAddr -> code -> plugin name
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
// Flag-only capabilities (e.g., link-local-nexthop code 77) have no encoding
// and no payload — they return nil, nil.
func DecodeCapabilityPayload(cap PluginCapability) ([]byte, error) {
	if cap.Encoding == "" && cap.Payload == "" {
		return nil, nil
	}

	switch cap.Encoding {
	case wireEncB64:
		return base64.StdEncoding.DecodeString(cap.Payload)
	case wireEncHex:
		return hex.DecodeString(cap.Payload)
	case EncodingText:
		return []byte(cap.Payload), nil
	}

	return nil, fmt.Errorf("unknown encoding: %s", cap.Encoding)
}

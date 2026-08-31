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

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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
		return textbuf.StrIntStr("Unknown(", int64(s), ")")
	}
}

// PluginRegistration holds Stage 1 registration data from a plugin.
type PluginRegistration struct {
	Name                   string              // Plugin name (set after Stage 4)
	RFCs                   []uint16            // RFC numbers for human-readable feature tracking
	Encodings              []string            // Supported encodings (text, b64, hex)
	Families               []string            // Address families (e.g., "ipv4/unicast", "all")
	DecodeFamilies         []string            // Families this plugin decodes (claimed via "declare family X decode")
	Commands               []string            // Command names to register
	CommandDescriptions    map[string]string   // Command name -> one-line summary (from CommandDecl.Description)
	CommandLongHelp        map[string]string   // Command name -> long explanation (from CommandDecl.LongHelp); absent means the plugin declared none
	CommandHidden          map[string]bool     // Command name -> hidden from completion (from CommandDecl)
	CommandCompletable     map[string]bool     // Command name -> process handles arg completion (from CommandDecl)
	CommandDeprecatedNames map[string][]string // Canonical command name -> deprecated aliases
	Receive                []string            // Message types to receive (update, open, negotiated, etc.)
	SchemaDeclarations     []SchemaDeclaration // Schema extensions for capability config
	WantsConfigRoots       []string            // Config roots to receive (e.g., ["bgp", "environment"] via "declare wants config <root>")
	ConfigOperations       []rpc.ConfigOperationDecl
	VerifyBudget           int  // Estimated verify time in seconds (0 = trivial)
	ApplyBudget            int  // Estimated apply time in seconds (0 = trivial)
	WantsValidateOpen      bool // Plugin wants to validate OPEN message pairs (validate-open callback)
	Done                   bool // True when "registration done" received

	// Claims are exclusive runtime roles this plugin takes over from another
	// plugin's default behavior, declared in Stage 1. The engine unions them
	// across the startup set and delivers the union on the Stage-2 configure
	// callback. See registry.Registration.Claims.
	Claims []string

	// YANG schema declarations (Hub Architecture)
	PluginSchema *PluginSchemaDecl // YANG schema declaration for this plugin

	// Route filter declarations (filter chain protocol)
	Filters []FilterRegistration // Named filters this plugin offers

	// Doctor check declarations (runtime health checks)
	DoctorChecks []DoctorCheckRegistration // Doctor checks this plugin provides
}

// FilterRegistration holds a named filter declaration from stage 1.
//
// Non-unicast address family support: the engine's text-mode filter
// protocol (`FilterUpdateInput.Update`) inlines prefixes only for families
// whose NLRI wire format is a plain CIDR prefix (IPv4/IPv6 unicast,
// multicast, mpls-label). For non-CIDR families (EVPN, Flowspec, VPN,
// BGP-LS, MVPN, MUP, RTC) the engine emits a marker-only block
// `nlri <family> <op>` with no prefixes. A filter plugin that needs
// per-NLRI decisions on a non-CIDR family MUST declare Raw=true and parse
// the wire payload itself from `FilterUpdateInput.Raw`. A Raw=false filter
// attached to such a session is advisory for those families -- it cannot
// distinguish individual destinations within the family. See
// `docs/architecture/api/process-protocol.md` "Non-CIDR Families in the
// Filter Text Protocol" and
// `internal/component/bgp/reactor/filter_format.go` (`isCIDRFamily`) for
// the full contract.
type FilterRegistration struct {
	Name       string              // Filter name (referenced in filter { import/export } blocks)
	Direction  rpc.FilterDirection // import / export / both
	Attributes []string            // Attribute names to receive
	NLRI       bool                // Include NLRI list (default true)
	Raw        bool                // Include raw wire bytes; REQUIRED for non-CIDR families
	OnError    rpc.OnErrorPolicy   // reject (fail-closed) or accept (fail-open)
	Overrides  []string            // Default filters this filter replaces
}

// DoctorCheckRegistration holds a doctor check declaration from Stage 1.
type DoctorCheckRegistration struct {
	Name         string               // Check name (kebab-case)
	Phase        rpc.DoctorCheckPhase // When to run: pre-config, missing-config, post-config
	Order        int                  // Ordering within phase (0-9999)
	Dependencies []string             // Other check names that must run first
	Platforms    []string             // Platform filter (empty = "any")
	Codes        []string             // Diagnostic codes (must have "doctor-" prefix)
}

// PluginSchemaDecl holds YANG schema declaration from a plugin.
// Built incrementally from multiple `declare schema` lines.
type PluginSchemaDecl struct {
	Module    string   // YANG module name
	Namespace string   // YANG namespace URI
	Handlers  []string // Handler paths (e.g., "bgp", "bgp/peer")
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
	Code     uint8           // Capability type code
	Encoding rpc.CapEncoding // Payload encoding (hex / b64 / text)
	Payload  string          // Encoded capability value
	Peers    []string        // Optional peer addresses (empty = global/all peers)
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
	for _, fam := range reg.DecodeFamilies {
		familyKey := strings.ToLower(fam)
		if existing, ok := r.families[familyKey]; ok {
			return fmt.Errorf("family conflict: %s already registered by %s", fam, existing)
		}
	}

	// Register commands
	for _, cmd := range reg.Commands {
		cmdKey := strings.ToLower(cmd)
		r.commands[cmdKey] = reg.Name
	}

	// Register family decode claims
	for _, fam := range reg.DecodeFamilies {
		familyKey := strings.ToLower(fam)
		r.families[familyKey] = reg.Name
	}

	r.plugins[reg.Name] = reg
	return nil
}

// Unregister removes all registry rows owned by a plugin. It is intentionally
// owner-scoped so startup rollback cannot remove another plugin's commands,
// decode families, or capability claims.
func (r *PluginRegistry) Unregister(pluginName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.plugins, pluginName)
	for key, owner := range r.commands {
		if owner == pluginName {
			delete(r.commands, key)
		}
	}
	for key, owner := range r.families {
		if owner == pluginName {
			delete(r.families, key)
		}
	}
	for code, owner := range r.capabilities {
		if owner == pluginName {
			delete(r.capabilities, code)
		}
	}
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
	for fam := range r.families {
		families = append(families, fam)
	}
	sort.Strings(families)
	return families
}

// DecodeFamiliesForPlugins returns the decode families of the NAMED plugins
// only, sorted, lowercase.
//
// GetDecodeFamilies answers for the whole process, which is the right answer for
// startup validation and the wrong one for a peer. A peer whose config names no
// family block took every loaded plugin's families into its OPEN, so an ordinary
// peer was offered link-state and the rest, and the implicit ipv4/unicast
// default never fired because the set was not empty. What a peer should offer is
// what ITS attached processes decode.
//
// An empty name list returns nil, which is what lets the implicit default fire
// for a peer with no family block and no attached process.
func (r *PluginRegistry) DecodeFamiliesForPlugins(names []string) []string {
	if len(names) == 0 {
		return nil
	}

	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		wanted[strings.ToLower(name)] = true
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var families []string
	for fam, plugin := range r.families {
		if wanted[strings.ToLower(plugin)] {
			families = append(families, fam)
		}
	}
	sort.Strings(families)

	return families
}

// registerCapabilities adds capability declarations, checking for conflicts.
func (r *PluginRegistry) registerCapabilities(caps *PluginCapabilities) error {
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
// The batch is atomic: decode and conflict validation happen before any map or
// slice is mutated.
func (ci *CapabilityInjector) AddPluginCapabilities(caps *PluginCapabilities) error {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	addedGlobals := make(map[uint8]string)
	addedPeers := make(map[string]map[uint8]string)
	pending := make([]InjectedCapability, 0, len(caps.Capabilities))

	for _, cap := range caps.Capabilities {
		value, err := decodeCapabilityPayload(cap)
		if err != nil {
			return err
		}

		if len(cap.Peers) == 0 {
			if existing, ok := ci.globalByCode[cap.Code]; ok {
				return fmt.Errorf("capability conflict: code %d already registered by %s", cap.Code, existing)
			}
			if existing, ok := addedGlobals[cap.Code]; ok {
				return fmt.Errorf("capability conflict: code %d already registered by %s", cap.Code, existing)
			}
			addedGlobals[cap.Code] = caps.PluginName
			pending = append(pending, InjectedCapability{
				Code:   cap.Code,
				Value:  value,
				Plugin: caps.PluginName,
			})
			continue
		}

		for _, peerAddr := range cap.Peers {
			if peerCodes := ci.peerByCode[peerAddr]; peerCodes != nil {
				if existing, ok := peerCodes[cap.Code]; ok {
					return fmt.Errorf("capability conflict: code %d for peer %s already registered by %s",
						cap.Code, peerAddr, existing)
				}
			}
			if addedPeers[peerAddr] == nil {
				addedPeers[peerAddr] = make(map[uint8]string)
			}
			if existing, ok := addedPeers[peerAddr][cap.Code]; ok {
				return fmt.Errorf("capability conflict: code %d for peer %s already registered by %s",
					cap.Code, peerAddr, existing)
			}
			addedPeers[peerAddr][cap.Code] = caps.PluginName
			pending = append(pending, InjectedCapability{
				Code:     cap.Code,
				Value:    value,
				Plugin:   caps.PluginName,
				PeerAddr: peerAddr,
			})
		}
	}

	for _, cap := range pending {
		if cap.PeerAddr == "" {
			ci.globalCaps = append(ci.globalCaps, cap)
			ci.globalByCode[cap.Code] = cap.Plugin
			continue
		}
		ci.peerCaps[cap.PeerAddr] = append(ci.peerCaps[cap.PeerAddr], cap)
		if ci.peerByCode[cap.PeerAddr] == nil {
			ci.peerByCode[cap.PeerAddr] = make(map[uint8]string)
		}
		ci.peerByCode[cap.PeerAddr][cap.Code] = cap.Plugin
	}
	return nil
}

// RemovePluginCapabilities removes all injected capabilities owned by a plugin.
func (ci *CapabilityInjector) RemovePluginCapabilities(pluginName string) {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	globalCaps := ci.globalCaps[:0]
	for _, cap := range ci.globalCaps {
		if cap.Plugin == pluginName {
			delete(ci.globalByCode, cap.Code)
			continue
		}
		globalCaps = append(globalCaps, cap)
	}
	ci.globalCaps = globalCaps

	for peerAddr, caps := range ci.peerCaps {
		kept := caps[:0]
		for _, cap := range caps {
			if cap.Plugin == pluginName {
				if peerCodes := ci.peerByCode[peerAddr]; peerCodes != nil {
					delete(peerCodes, cap.Code)
				}
				continue
			}
			kept = append(kept, cap)
		}
		if len(kept) == 0 {
			delete(ci.peerCaps, peerAddr)
			delete(ci.peerByCode, peerAddr)
			continue
		}
		ci.peerCaps[peerAddr] = kept
	}
}

// AllCapabilities returns all stored capabilities (global + all per-peer).
// Used to compute max restart-time across all peers for the GR marker.
func (ci *CapabilityInjector) AllCapabilities() []InjectedCapability {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	total := len(ci.globalCaps)
	for _, caps := range ci.peerCaps {
		total += len(caps)
	}
	result := make([]InjectedCapability, 0, total)
	result = append(result, ci.globalCaps...)
	for _, caps := range ci.peerCaps {
		result = append(result, caps...)
	}
	return result
}

// GetCapabilitiesForSelectors returns capabilities for one peer that several
// selectors can name, resolving them in the order given.
//
// A peer is reachable under more than one selector: the name the operator wrote,
// its remote address, and the dynamic group whose template created it. For each
// capability CODE the first selector that declares it wins, and a global
// declaration answers only for a code no selector claimed.
//
// It takes the whole list rather than being called once per selector, because
// every answer carries the global set: a caller probing selectors in turn and
// stopping at the first non-empty result stops at the globals and never reaches
// the address or the group. One plugin declaring one global capability was
// enough to cost every peer its per-peer declarations
// (internal/plugins/exabgp/main_sdk.go declares code 2 with no Peers).
func (ci *CapabilityInjector) GetCapabilitiesForSelectors(selectors ...string) []InjectedCapability {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	result := make([]InjectedCapability, 0, len(ci.globalCaps)+len(selectors))
	seenCodes := make(map[uint8]bool)

	// Per-peer capabilities first, in selector order: they take precedence over
	// the globals, and an earlier selector takes precedence over a later one.
	for _, selector := range selectors {
		if selector == "" {
			continue
		}
		for _, cap := range ci.peerCaps[selector] {
			if seenCodes[cap.Code] {
				continue
			}
			result = append(result, cap)
			seenCodes[cap.Code] = true
		}
	}

	// Add global capabilities that weren't overridden
	for _, cap := range ci.globalCaps {
		if !seenCodes[cap.Code] {
			result = append(result, cap)
		}
	}

	return result
}

// decodeCapabilityPayload decodes a plugin capability payload.
// Flag-only capabilities (e.g., link-local-nexthop code 77) have no encoding
// and no payload — they return nil, nil.
func decodeCapabilityPayload(cap PluginCapability) ([]byte, error) {
	if cap.Encoding == rpc.CapEncodingUnspecified && cap.Payload == "" {
		return nil, nil
	}

	switch cap.Encoding {
	case rpc.CapEncodingBase64:
		return base64.StdEncoding.DecodeString(cap.Payload)
	case rpc.CapEncodingHex:
		return hex.DecodeString(cap.Payload)
	case rpc.CapEncodingText:
		return []byte(cap.Payload), nil
	case rpc.CapEncodingUnspecified:
		return nil, fmt.Errorf("capability encoding unspecified with payload %q", cap.Payload)
	}

	return nil, fmt.Errorf("unknown encoding: %s", cap.Encoding)
}

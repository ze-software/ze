// Design: docs/architecture/api/architecture.md — plugin registry
//
// Package registry provides compile-time plugin registration.
//
// Plugins register themselves via init() functions, and the registry
// provides query functions for CLI dispatch, engine startup, and decode routing.
//
// This is a leaf package with no dependencies on plugin implementations,
// preventing import cycles. Plugin packages import this package to register;
// consumer packages import this package to query.
//
// Thread safety: init() functions run sequentially in Go, so registration
// during init is inherently safe. A RWMutex protects runtime access because
// tests use Reset/Snapshot/Restore to mutate the registry concurrently.
package registry

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/netip"
	"slices"
	"sort"
	"strings"
	"sync"
)

// PeerFilterInfo holds peer metadata for filter decisions.
// Passed by the reactor to registered filter functions.
type PeerFilterInfo struct {
	Address netip.Addr // Peer IP address
	PeerAS  uint32     // Remote AS number
}

// IngressFilterFunc is called for received UPDATEs before caching and dispatching.
// payload is the UPDATE body (without BGP header).
// meta is a shared metadata map; filters can read and write to it.
// Caller MUST pass a non-nil meta map; writing to a nil meta panics.
// Returns accept=false to drop the route. If modifiedPayload is non-nil,
// it replaces the original payload for caching and event dispatch.
type IngressFilterFunc func(source PeerFilterInfo, payload []byte, meta map[string]any) (accept bool, modifiedPayload []byte)

// EgressFilterFunc is called per destination peer during ForwardUpdate.
// payload is the UPDATE body (without BGP header).
// meta is route metadata set at ingress (read-only); may be nil.
// mods accumulates per-peer modifications applied after all filters pass.
// MUST NOT retain the mods pointer beyond the call -- it is reused per peer.
// Returns false to suppress the route for this destination peer.
type EgressFilterFunc func(source, dest PeerFilterInfo, payload []byte, meta map[string]any, mods *ModAccumulator) bool

// ModAccumulator collects per-peer route modifications from egress filters.
// NOT safe for concurrent use. Each peer iteration gets a fresh instance.
type ModAccumulator struct {
	ops []AttrOp
}

// Len returns the number of accumulated modifications.
func (a *ModAccumulator) Len() int { return len(a.ops) }

// Reset clears all accumulated modifications for reuse.
func (a *ModAccumulator) Reset() {
	a.ops = a.ops[:0]
}

// Op accumulates an attribute modification operation.
// Lazily allocates the slice on first call to avoid allocation
// when no filter writes modifications (the common case).
// Multiple calls with the same code are allowed -- the handler
// receives all ops for a given code at once during the progressive build.
func (a *ModAccumulator) Op(code, action uint8, buf []byte) {
	a.ops = append(a.ops, AttrOp{Code: code, Action: action, Buf: buf})
}

// Ops returns the accumulated attribute modification operations.
// Returns nil if no ops have been accumulated.
func (a *ModAccumulator) Ops() []AttrOp { return a.ops }

// Attribute modification action constants.
const (
	AttrModSet     uint8 = iota // Replace entire attribute value (or create if absent)
	AttrModAdd                  // Append value to attribute's list (e.g., COMMUNITY)
	AttrModRemove               // Remove value from attribute's list (e.g., COMMUNITY)
	AttrModPrepend              // Prepend value to attribute's sequence (e.g., AS_PATH)
)

// AttrOp describes a single attribute modification operation.
// Egress filters accumulate AttrOps in the ModAccumulator via Op().
// Multiple AttrOps with the same Code are allowed and are passed together
// to the registered handler during the progressive build.
type AttrOp struct {
	Code   uint8  // Attribute type code (e.g., 35 for OTC, 8 for COMMUNITY)
	Action uint8  // AttrModSet, AttrModAdd, AttrModRemove, AttrModPrepend
	Buf    []byte // Pre-built wire bytes of the VALUE (handler writes the header)
}

// AttrModHandler is a per-attribute-code handler for the progressive build.
// It receives the source attribute bytes (nil if absent in source), all ops
// for this attribute code, the output buffer, and the current write offset.
// It writes the complete attribute (header + value) into buf and returns
// the new offset after the written bytes.
// It cannot reject a route -- only transform. MUST NOT retain buf beyond the call.
type AttrModHandler func(src []byte, ops []AttrOp, buf []byte, off int) int

// attrModHandlers stores registered attr mod handlers keyed by attribute code.
// Populated at init() time by plugins, read at runtime by the reactor.
var attrModHandlers = make(map[uint8]AttrModHandler)

// RegisterAttrModHandler registers a handler for the given attribute code.
// Must be called from init() functions only. Ignores nil handlers.
func RegisterAttrModHandler(code uint8, handler AttrModHandler) {
	if handler == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	attrModHandlers[code] = handler
}

// UnregisterAttrModHandler removes an attr mod handler. Only for use in tests.
func UnregisterAttrModHandler(code uint8) {
	mu.Lock()
	defer mu.Unlock()
	delete(attrModHandlers, code)
}

// AttrModHandlerFor returns the registered handler for the given attribute code, or nil.
func AttrModHandlerFor(code uint8) AttrModHandler {
	mu.RLock()
	defer mu.RUnlock()
	return attrModHandlers[code]
}

// AttrModHandlers returns a snapshot of all registered attr mod handlers.
// Called by the reactor to build the handler map at startup.
func AttrModHandlers() map[uint8]AttrModHandler {
	mu.RLock()
	defer mu.RUnlock()
	result := make(map[uint8]AttrModHandler, len(attrModHandlers))
	maps.Copy(result, attrModHandlers)
	return result
}

// Registration describes a plugin's full metadata and handlers.
// Each plugin registers exactly one Registration via its init() function.
type Registration struct {
	// Required fields.
	Name        string                  // Plugin name (e.g., "flowspec", "gr")
	Description string                  // Human-readable description for help text
	RunEngine   func(conn net.Conn) int // Engine mode handler (single-conn RPC)
	CLIHandler  func(args []string) int // CLI handler for `ze plugin <name>`

	// Optional metadata.
	RFCs            []string // Related RFC numbers (e.g., ["8955", "8956"])
	Families        []string // Address families handled (e.g., ["ipv4/flow", "ipv6/flow"])
	CapabilityCodes []uint8  // Capability codes decoded (e.g., [73] for FQDN)
	ConfigRoots     []string // Config roots wanted (e.g., ["bgp"])
	Dependencies    []string // Plugin names that must also be loaded (e.g., ["bgp-adj-rib-in"])
	EventTypes      []string // Event types this plugin produces (e.g., ["update-rpki"]). Registered dynamically at startup.
	SendTypes       []string // Send types this plugin enables (e.g., ["enhanced-refresh"]). Registered dynamically at startup.
	YANG            string   // YANG schema content (empty if none)

	// Optional handlers.
	ConfigureEngineLogger func(loggerName string)               // Configure logger for in-process engine mode
	InProcessDecoder      func(input, output *bytes.Buffer) int // Decode function for CLI fallback

	// In-process NLRI decode/encode: fast path for infrastructure code (text.go, update_text.go)
	// that avoids plugin package imports. Same semantics as the SDK's OnDecodeNLRI/OnEncodeNLRI
	// callbacks, but callable directly without RPC. External plugins use the RPC path instead.
	InProcessNLRIDecoder func(family, hex string) (string, error)           // (family, hex) → JSON
	InProcessNLRIEncoder func(family string, args []string) (string, error) // (family, args) → hex

	// In-process route encoder: builds a full UPDATE message for a given family.
	// Used by `ze bgp encode` to delegate family-specific encoding to plugins,
	// replacing the hardcoded switch in encode.go.
	// Parameters: routeCmd (text args), family, localAS, isIBGP, asn4, addPath.
	// Returns: (packed UPDATE bytes, NLRI bytes for --nlri-only, error).
	InProcessRouteEncoder func(routeCmd, family string, localAS uint32, isIBGP, asn4, addPath bool) ([]byte, []byte, error)

	// In-process config NLRI builder: builds NLRI bytes from config-format match criteria.
	// Used by config/loader.go to delegate family-specific NLRI construction to plugins,
	// avoiding direct plugin imports. The match criteria map uses config-syntax keys
	// (e.g., "destination", "protocol", "port") with string values.
	InProcessConfigNLRIBuilder func(matchCriteria map[string][]string, isIPv6, forVPN bool) []byte

	// ConfigureMetrics is called before RunEngine with the metrics registry (any).
	// The plugin should type-assert to metrics.Registry and register gauges/counters.
	ConfigureMetrics func(reg any)

	// In-process peer filters: called by the reactor for ingress/egress route filtering.
	// Ingress: before caching/dispatching received UPDATEs. Egress: per destination peer.
	// Filter closures capture plugin state (e.g., role configs) -- reactor passes only PeerFilterInfo.
	IngressFilter IngressFilterFunc // nil = no ingress filtering
	EgressFilter  EgressFilterFunc  // nil = no egress filtering

	// CLI metadata (used by RunPlugin).
	Features     string // Space-separated feature list (e.g., "nlri yang")
	SupportsNLRI bool   // Plugin can decode NLRI via CLI
	SupportsCapa bool   // Plugin can decode capabilities via CLI
}

var (
	// ErrEmptyName is returned when registering a plugin with an empty name.
	ErrEmptyName = errors.New("registry: plugin name is empty")
	// ErrDuplicateName is returned when registering a plugin with a name already taken.
	ErrDuplicateName = errors.New("registry: duplicate plugin name")
	// ErrNilRunEngine is returned when RunEngine is nil.
	ErrNilRunEngine = errors.New("registry: RunEngine is nil")
	// ErrNilCLIHandler is returned when CLIHandler is nil.
	ErrNilCLIHandler = errors.New("registry: CLIHandler is nil")
	// ErrInvalidFamily is returned when a family string is malformed.
	ErrInvalidFamily = errors.New("registry: invalid family format")
	// ErrSelfDependency is returned when a plugin lists itself as a dependency.
	ErrSelfDependency = errors.New("registry: plugin depends on itself")
	// ErrEmptyDependency is returned when a dependency name is empty.
	ErrEmptyDependency = errors.New("registry: empty dependency name")
	// ErrCircularDependency is returned when dependency resolution detects a cycle.
	ErrCircularDependency = errors.New("registry: circular dependency")
	// ErrMissingDependency is returned when a registered plugin declares a dependency on an unknown name.
	ErrMissingDependency = errors.New("registry: missing dependency")
)

var (
	// plugins is the global plugin registry, populated during init().
	plugins = make(map[string]*Registration)
	mu      sync.RWMutex

	// metricsRegistry stores the metrics registry (as any to avoid importing metrics).
	// Set by the config loader after creating the Prometheus registry.
	// Read by GetInternalPluginRunner to inject into plugins via ConfigureMetrics.
	metricsRegistry any
)

// Register adds a plugin to the global registry.
// Must be called from init() functions only.
// Returns an error on invalid registration.
func Register(reg Registration) error { //nolint:gocritic // hugeParam: Registration is passed by value for init()-time safety; copied internally.
	if reg.Name == "" {
		return ErrEmptyName
	}

	mu.Lock()
	defer mu.Unlock()

	if _, exists := plugins[reg.Name]; exists {
		return fmt.Errorf("%w: %q", ErrDuplicateName, reg.Name)
	}
	if reg.RunEngine == nil {
		return fmt.Errorf("%w: plugin %q", ErrNilRunEngine, reg.Name)
	}
	if reg.CLIHandler == nil {
		return fmt.Errorf("%w: plugin %q", ErrNilCLIHandler, reg.Name)
	}
	for _, f := range reg.Families {
		if !strings.Contains(f, "/") {
			return fmt.Errorf("%w: plugin %q family %q (must contain /)", ErrInvalidFamily, reg.Name, f)
		}
	}
	for _, dep := range reg.Dependencies {
		if dep == "" {
			return fmt.Errorf("%w: plugin %q", ErrEmptyDependency, reg.Name)
		}
		if dep == reg.Name {
			return fmt.Errorf("%w: plugin %q", ErrSelfDependency, reg.Name)
		}
	}

	r := reg // copy
	plugins[reg.Name] = &r
	return nil
}

// SetMetricsRegistry stores the metrics registry for plugin injection.
// Called by the config loader after creating the Prometheus registry.
// The registry is passed as any to avoid importing the metrics package.
func SetMetricsRegistry(reg any) {
	mu.Lock()
	defer mu.Unlock()
	metricsRegistry = reg
}

// GetMetricsRegistry returns the stored metrics registry, or nil.
func GetMetricsRegistry() any {
	mu.RLock()
	defer mu.RUnlock()
	return metricsRegistry
}

// Lookup returns the registration for a named plugin, or nil if not found.
func Lookup(name string) *Registration {
	mu.RLock()
	defer mu.RUnlock()
	return plugins[name]
}

// All returns all registrations sorted by name.
func All() []*Registration {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(plugins))
	for name := range plugins {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]*Registration, len(names))
	for i, name := range names {
		result[i] = plugins[name]
	}
	return result
}

// Names returns all registered plugin names sorted alphabetically.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(plugins))
	for name := range plugins {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Has returns true if a plugin with the given name is registered.
func Has(name string) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := plugins[name]
	return ok
}

// FamilyMap returns a map from address family to plugin name.
// Built from all registered plugins' Families fields.
func FamilyMap() map[string]string {
	mu.RLock()
	defer mu.RUnlock()

	m := make(map[string]string)
	for _, reg := range plugins {
		for _, f := range reg.Families {
			m[f] = reg.Name
		}
	}
	return m
}

// CapabilityMap returns a map from capability code to plugin name.
// Built from all registered plugins' CapabilityCodes fields.
func CapabilityMap() map[uint8]string {
	mu.RLock()
	defer mu.RUnlock()

	m := make(map[uint8]string)
	for _, reg := range plugins {
		for _, code := range reg.CapabilityCodes {
			m[code] = reg.Name
		}
	}
	return m
}

// InProcessDecoders returns a map from plugin name to decode function.
// Only includes plugins that registered an InProcessDecoder.
func InProcessDecoders() map[string]func(input, output *bytes.Buffer) int {
	mu.RLock()
	defer mu.RUnlock()

	m := make(map[string]func(input, output *bytes.Buffer) int)
	for _, reg := range plugins {
		if reg.InProcessDecoder != nil {
			m[reg.Name] = reg.InProcessDecoder
		}
	}
	return m
}

// YANGSchemas returns a map from plugin name to YANG schema content.
// Only includes plugins that registered a YANG schema.
func YANGSchemas() map[string]string {
	mu.RLock()
	defer mu.RUnlock()

	m := make(map[string]string)
	for _, reg := range plugins {
		if reg.YANG != "" {
			m[reg.Name] = reg.YANG
		}
	}
	return m
}

// ConfigRootsMap returns a map from plugin name to config roots.
// Only includes plugins that declared config roots.
func ConfigRootsMap() map[string][]string {
	mu.RLock()
	defer mu.RUnlock()

	m := make(map[string][]string)
	for _, reg := range plugins {
		if len(reg.ConfigRoots) > 0 {
			m[reg.Name] = reg.ConfigRoots
		}
	}
	return m
}

// PluginForFamily returns the plugin name that handles a given address family.
// Returns empty string if no plugin is registered for the family.
func PluginForFamily(family string) string {
	mu.RLock()
	defer mu.RUnlock()

	for _, reg := range plugins {
		if slices.Contains(reg.Families, family) {
			return reg.Name
		}
	}
	return ""
}

// PluginForEventType returns the plugin name that produces a given event type.
// Returns empty string if no plugin declares that event type.
func PluginForEventType(eventType string) string {
	mu.RLock()
	defer mu.RUnlock()

	for _, reg := range plugins {
		if slices.Contains(reg.EventTypes, eventType) {
			return reg.Name
		}
	}
	return ""
}

// PluginForSendType returns the plugin name that enables a given send type.
// Returns empty string if no plugin declares that send type.
func PluginForSendType(sendType string) string {
	mu.RLock()
	defer mu.RUnlock()

	for _, reg := range plugins {
		if slices.Contains(reg.SendTypes, sendType) {
			return reg.Name
		}
	}
	return ""
}

// RequiredPlugins returns deduplicated plugin names needed for the given families.
func RequiredPlugins(families []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, fam := range families {
		if p := PluginForFamily(fam); p != "" && !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}
	return result
}

// DecodeNLRIByFamily finds the plugin registered for a family and calls its
// in-process NLRI decoder. Returns the JSON result and nil on success.
// Returns an error if no decoder is registered or the decoder fails.
// This is the fast path — external plugins use RPC via Server.DecodeNLRI instead.
func DecodeNLRIByFamily(family, hexData string) (string, error) {
	mu.RLock()
	defer mu.RUnlock()

	for _, reg := range plugins {
		if reg.InProcessNLRIDecoder != nil && slices.Contains(reg.Families, family) {
			return reg.InProcessNLRIDecoder(family, hexData)
		}
	}
	return "", fmt.Errorf("no NLRI decoder for family %s", family)
}

// EncodeNLRIByFamily finds the plugin registered for a family and calls its
// in-process NLRI encoder. Returns the hex result and nil on success.
// Returns an error if no encoder is registered or the encoder fails.
// This is the fast path — external plugins use RPC via Server.EncodeNLRI instead.
func EncodeNLRIByFamily(family string, args []string) (string, error) {
	mu.RLock()
	defer mu.RUnlock()

	for _, reg := range plugins {
		if reg.InProcessNLRIEncoder != nil && slices.Contains(reg.Families, family) {
			return reg.InProcessNLRIEncoder(family, args)
		}
	}
	return "", fmt.Errorf("no NLRI encoder for family %s", family)
}

// RouteEncoderByFamily finds the plugin registered for a family and returns
// its in-process route encoder. Returns nil if no encoder is registered.
func RouteEncoderByFamily(family string) func(routeCmd, family string, localAS uint32, isIBGP, asn4, addPath bool) ([]byte, []byte, error) {
	mu.RLock()
	defer mu.RUnlock()

	for _, reg := range plugins {
		if reg.InProcessRouteEncoder != nil && slices.Contains(reg.Families, family) {
			return reg.InProcessRouteEncoder
		}
	}
	return nil
}

// ConfigNLRIBuilder finds the plugin registered for a family and returns
// its config NLRI builder. Returns nil if no builder is registered.
func ConfigNLRIBuilder(family string) func(map[string][]string, bool, bool) []byte {
	mu.RLock()
	defer mu.RUnlock()

	for _, reg := range plugins {
		if reg.InProcessConfigNLRIBuilder != nil && slices.Contains(reg.Families, family) {
			return reg.InProcessConfigNLRIBuilder
		}
	}
	return nil
}

// WriteUsage writes a formatted plugin list to w for help text.
// Each line follows the format: "  name    description".
func WriteUsage(w io.Writer) error {
	regs := All()
	if len(regs) == 0 {
		return nil
	}

	// Find max name length for alignment.
	maxLen := 0
	for _, r := range regs {
		if len(r.Name) > maxLen {
			maxLen = len(r.Name)
		}
	}

	for _, r := range regs {
		padding := strings.Repeat(" ", maxLen-len(r.Name)+2)
		desc := r.Description
		if len(r.RFCs) > 0 {
			desc += " (RFC " + strings.Join(r.RFCs, ", ") + ")"
		}
		if _, err := fmt.Fprintf(w, "  %s%s%s\n", r.Name, padding, desc); err != nil {
			return fmt.Errorf("writing usage: %w", err)
		}
	}
	return nil
}

// IngressFilters returns all registered ingress filter functions.
// Called by the reactor to build the ingress filter chain.
func IngressFilters() []IngressFilterFunc {
	mu.RLock()
	defer mu.RUnlock()

	var filters []IngressFilterFunc
	for _, reg := range plugins {
		if reg.IngressFilter != nil {
			filters = append(filters, reg.IngressFilter)
		}
	}
	return filters
}

// EgressFilters returns all registered egress filter functions.
// Called by the reactor to build the egress filter chain.
func EgressFilters() []EgressFilterFunc {
	mu.RLock()
	defer mu.RUnlock()

	var filters []EgressFilterFunc
	for _, reg := range plugins {
		if reg.EgressFilter != nil {
			filters = append(filters, reg.EgressFilter)
		}
	}
	return filters
}

// Reset clears the registry. Only for use in tests.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	plugins = make(map[string]*Registration)
	attrModHandlers = make(map[uint8]AttrModHandler)
	metricsRegistry = nil
}

// RegistrySnapshot holds a complete copy of the registry state for test save/restore.
type RegistrySnapshot struct {
	plugins         map[string]*Registration
	attrModHandlers map[uint8]AttrModHandler
}

// Snapshot returns a copy of the current registry state. Only for use in tests.
// Use with Restore to safely reset and restore after test-specific registrations.
func Snapshot() RegistrySnapshot {
	mu.RLock()
	defer mu.RUnlock()

	ps := make(map[string]*Registration, len(plugins))
	maps.Copy(ps, plugins)
	ah := make(map[uint8]AttrModHandler, len(attrModHandlers))
	maps.Copy(ah, attrModHandlers)
	return RegistrySnapshot{plugins: ps, attrModHandlers: ah}
}

// Restore replaces the registry with a previously saved snapshot. Only for use in tests.
func Restore(snap RegistrySnapshot) {
	mu.Lock()
	defer mu.Unlock()
	plugins = snap.plugins
	attrModHandlers = snap.attrModHandlers
}

// ResolveDependencies expands a list of plugin names by iteratively adding
// dependencies declared in the registry. Returns the expanded list with no
// duplicates. Plugins not in the registry (external) are kept but their deps
// are not expanded (they come from the protocol layer instead).
// Returns ErrCircularDependency on cycles, ErrMissingDependency when a
// registered plugin declares a dependency on an unregistered name.
func ResolveDependencies(requested []string) ([]string, error) {
	mu.RLock()
	defer mu.RUnlock()

	// Build ordered result + seen set for dedup.
	seen := make(map[string]bool, len(requested))
	result := make([]string, 0, len(requested))
	for _, name := range requested {
		if !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}

	// Iterative expansion: loop until no new deps are added.
	// Track resolution path per-plugin for cycle detection.
	for i := 0; i < len(result); i++ {
		name := result[i]
		reg, ok := plugins[name]
		if !ok {
			// External plugin — skip (deps come from protocol layer).
			continue
		}
		for _, dep := range reg.Dependencies {
			if seen[dep] {
				continue
			}
			if _, depOK := plugins[dep]; !depOK {
				return nil, fmt.Errorf("%w: plugin %q requires %q", ErrMissingDependency, name, dep)
			}
			seen[dep] = true
			result = append(result, dep)
		}
	}

	// Cycle detection: walk dependency chains looking for back-edges.
	if err := detectCycles(result); err != nil {
		return nil, err
	}

	return result, nil
}

// detectCycles checks for circular dependencies using DFS with coloring.
// white=unvisited, gray=in-progress, black=done.
func detectCycles(names []string) error {
	const (
		white = 0
		gray  = 1
		black = 2
	)

	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}

	color := make(map[string]int, len(names))
	var visit func(string) error
	visit = func(name string) error {
		color[name] = gray
		reg, ok := plugins[name]
		if ok {
			for _, dep := range reg.Dependencies {
				if !nameSet[dep] {
					continue
				}
				switch color[dep] {
				case gray:
					return fmt.Errorf("%w: %s → %s", ErrCircularDependency, name, dep)
				case white:
					if err := visit(dep); err != nil {
						return err
					}
				}
			}
		}
		color[name] = black
		return nil
	}

	for _, name := range names {
		if color[name] == white {
			if err := visit(name); err != nil {
				return err
			}
		}
	}
	return nil
}

// TopologicalTiers groups plugin names into dependency tiers using Kahn's algorithm.
// Tier 0 contains plugins with no dependencies (or not in registry). Tier N contains
// plugins whose dependencies are all in tiers < N. Plugins within each tier are sorted
// alphabetically for deterministic startup ordering.
//
// Returns ErrCircularDependency if the dependency graph contains a cycle.
// Only considers dependencies within the requested name set; external names
// (not in the requested set or registry) are ignored for ordering purposes.
func TopologicalTiers(names []string) ([][]string, error) {
	mu.RLock()
	defer mu.RUnlock()

	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}

	// Compute in-degree (number of deps within name set) for each plugin.
	inDegree := make(map[string]int, len(names))
	// deps maps each plugin to its dependencies within the name set.
	deps := make(map[string][]string, len(names))
	for _, name := range names {
		inDegree[name] = 0
	}

	for _, name := range names {
		reg, ok := plugins[name]
		if !ok {
			continue // External plugin — no known deps, stays at in-degree 0
		}
		for _, dep := range reg.Dependencies {
			if nameSet[dep] {
				deps[name] = append(deps[name], dep)
				inDegree[name]++
			}
		}
	}

	// Kahn's algorithm: iteratively peel off nodes with in-degree 0.
	var tiers [][]string
	remaining := len(names)

	for remaining > 0 {
		// Collect all nodes with in-degree 0 for this tier.
		var tier []string
		for _, name := range names {
			if inDegree[name] == 0 {
				tier = append(tier, name)
			}
		}

		if len(tier) == 0 {
			// All remaining nodes have in-degree > 0 → cycle.
			return nil, fmt.Errorf("%w: no progress in tier computation", ErrCircularDependency)
		}

		// Sort tier for deterministic ordering.
		sort.Strings(tier)
		tiers = append(tiers, tier)

		// Remove tier nodes: set in-degree to -1 (processed), decrement dependents.
		for _, done := range tier {
			inDegree[done] = -1 // Mark as processed
			remaining--
		}

		// Decrement in-degree for nodes that depended on completed tier.
		for _, name := range names {
			if inDegree[name] <= 0 {
				continue // Already processed or will be
			}
			for _, dep := range deps[name] {
				for _, done := range tier {
					if dep == done {
						inDegree[name]--
					}
				}
			}
		}
	}

	return tiers, nil
}

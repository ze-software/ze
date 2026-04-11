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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"slices"
	"sort"
	"strings"
	"sync"
)

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

	// FilterTypes lists the YANG filter list names this plugin owns (e.g.,
	// ["prefix-list"]). Used by the policy filter chain to resolve short chain
	// refs like "prefix-list:CUSTOMERS" or plain "CUSTOMERS" to this plugin's
	// process name at config-parse time. Without FilterTypes, users would have
	// to spell the full plugin process name (e.g., "bgp-filter-prefix:CUSTOMERS")
	// in every chain ref. Filter type names must be globally unique across
	// plugins; duplicate registration aborts startup.
	FilterTypes []string

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

	// ConfigureEventBus is called before RunEngine with the EventBus instance (any).
	// The plugin should type-assert to ze.EventBus and store the reference for
	// emitting and subscribing to namespaced events.
	ConfigureEventBus func(eventBus any)

	// ConfigurePluginServer is called before RunEngine with the plugin server (any).
	// The plugin should type-assert to *pluginserver.Server and store the reference.
	// Used by BGP to wire EventDispatcher and command dispatch to the shared server.
	ConfigurePluginServer func(server any)

	// In-process peer filters: called by the reactor for ingress/egress route filtering.
	// Ingress: before caching/dispatching received UPDATEs. Egress: per destination peer.
	// Filter closures capture plugin state (e.g., role configs) -- reactor passes only PeerFilterInfo.
	IngressFilter  IngressFilterFunc // nil = no ingress filtering
	EgressFilter   EgressFilterFunc  // nil = no egress filtering
	FilterStage    int               // Coarse ordering class (FilterStageProtocol/Policy/Annotation)
	FilterPriority int               // Fine ordering within stage; equal priority sorted by name

	// RPCHandlers maps RPC method names to handler functions. Registered at init()
	// and collected by the plugin server for dispatch. Each handler unmarshals its
	// own params and returns a result or error. Used by BGP for codec RPCs
	// (decode-nlri, encode-nlri, etc.) without the server importing bgp packages.
	RPCHandlers map[string]func(json.RawMessage) (any, error)

	// FatalOnConfigError makes a config-path plugin's startup failure fatal to ze.
	// When true and the plugin's configure callback fails, ze exits instead of
	// continuing without the plugin. Set by BGP because an invalid BGP config
	// should not silently produce a running ze with no BGP.
	FatalOnConfigError bool

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

	// builtinFamilies maps family names to source names for engine-builtin families
	// (e.g., ipv4/unicast) that are not registered through the plugin system.
	builtinFamilies = make(map[string]string)

	// familyIndex maps family string to the Registration that handles it.
	// Reverse index for O(1) PluginForFamily and *ByFamily lookups.
	// Rebuilt by rebuildFamilyIndexLocked on Register(), Reset(), Restore().
	familyIndex = make(map[string]*Registration)

	// filterTypes maps YANG filter list type names (e.g., "prefix-list") to
	// the plugin process name that owns them (e.g., "bgp-filter-prefix").
	// Populated at Register() time from Registration.FilterTypes. Used by the
	// config layer to canonicalize short chain refs to the full plugin:filter
	// form expected by runtime dispatch.
	filterTypes = make(map[string]string)

	// metricsRegistry stores the metrics registry (as any to avoid importing metrics).
	// Set by the config loader after creating the Prometheus registry.
	// Read by GetInternalPluginRunner to inject into plugins via ConfigureMetrics.
	metricsRegistry any

	// eventBusInstance stores the EventBus instance (as any to avoid importing ze package).
	// Set by the engine after creating the plugin server (which implements ze.EventBus).
	// Read by GetInternalPluginRunner to inject into plugins via ConfigureEventBus.
	eventBusInstance any

	// pluginServerInstance stores the plugin server (as any to avoid importing server package).
	// Set by the hub after creating the plugin server.
	// Read by GetInternalPluginRunner to inject into plugins via ConfigurePluginServer.
	pluginServerInstance any
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
	// Filter types must be globally unique across plugins so chain refs
	// like "prefix-list:CUSTOMERS" or plain "CUSTOMERS" can be resolved
	// unambiguously to exactly one plugin.
	for _, ft := range reg.FilterTypes {
		if ft == "" {
			return fmt.Errorf("registry: plugin %q has empty filter type", reg.Name)
		}
		if existing, dup := filterTypes[ft]; dup {
			return fmt.Errorf("registry: filter type %q already registered by %q", ft, existing)
		}
	}

	r := reg // copy
	plugins[reg.Name] = &r
	for _, ft := range reg.FilterTypes {
		filterTypes[ft] = reg.Name
	}
	rebuildFamilyIndexLocked()
	return nil
}

// PluginForFilterType returns the plugin process name that owns the given
// YANG filter list type (e.g., "prefix-list" -> "bgp-filter-prefix").
// Returns "" if no plugin has registered that filter type.
//
// Used by the config layer to canonicalize short chain refs. A user can
// write `filter import [ CUSTOMERS ]` or `[ prefix-list:CUSTOMERS ]` and
// the config layer looks up the filter type via this function and rewrites
// the ref to the full `<plugin>:<filter>` form consumed by runtime dispatch.
func PluginForFilterType(filterType string) string {
	mu.RLock()
	defer mu.RUnlock()
	return filterTypes[filterType]
}

// FilterTypesMap returns a copy of the (filter-type -> plugin-name) map.
// Used by config validation to enumerate known filter types.
func FilterTypesMap() map[string]string {
	mu.RLock()
	defer mu.RUnlock()
	out := make(map[string]string, len(filterTypes))
	maps.Copy(out, filterTypes)
	return out
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

// SetEventBus stores the EventBus instance for plugin injection.
// Called by the engine after creating the plugin server (which implements
// ze.EventBus). The instance is passed as any to avoid importing the ze
// package from this leaf registry package.
func SetEventBus(eventBus any) {
	mu.Lock()
	defer mu.Unlock()
	eventBusInstance = eventBus
}

// GetEventBus returns the stored EventBus instance, or nil.
func GetEventBus() any {
	mu.RLock()
	defer mu.RUnlock()
	return eventBusInstance
}

// SetPluginServer stores the plugin server instance for injection into plugins.
func SetPluginServer(server any) {
	mu.Lock()
	defer mu.Unlock()
	pluginServerInstance = server
}

// GetPluginServer returns the stored plugin server instance, or nil.
func GetPluginServer() any {
	mu.RLock()
	defer mu.RUnlock()
	return pluginServerInstance
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

// RegisterBuiltinFamilies records engine-builtin families (e.g., ipv4/unicast)
// that are not part of a plugin but should appear in FamilyMap. Called from
// init() in the engine's family package. The source identifies the origin
// (e.g., "builtin").
func RegisterBuiltinFamilies(source string, families []string) {
	mu.Lock()
	defer mu.Unlock()

	for _, f := range families {
		builtinFamilies[f] = source
	}
}

// FamilyMap returns a map from address family to plugin name.
// Only includes plugin-registered families (used for decode routing).
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

// AllFamilies returns a map from address family to source name.
// Includes both plugin-registered and engine-builtin families.
// Used for completion and inventory where all known families should appear.
func AllFamilies() map[string]string {
	mu.RLock()
	defer mu.RUnlock()

	m := make(map[string]string)
	maps.Copy(m, builtinFamilies)
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

// rpcHandlers holds RPC method handlers registered by plugins via AddRPCHandlers.
// Separate from Registration.RPCHandlers to allow registration from packages that
// cannot be imported by the plugin's register.go without creating cycles.
//
//nolint:gochecknoglobals // Package-level registry, same pattern as plugins map.
var rpcHandlers = make(map[string]func(json.RawMessage) (any, error))

// AddRPCHandlers registers RPC method handlers. Called from init() in packages
// that provide codec/decode handlers (e.g., bgp/server). Thread-safe: protected
// by the package mutex.
func AddRPCHandlers(handlers map[string]func(json.RawMessage) (any, error)) {
	mu.Lock()
	defer mu.Unlock()
	maps.Copy(rpcHandlers, handlers)
}

// CollectRPCHandlers returns a merged map of all registered RPC method handlers.
// Includes handlers from both Registration.RPCHandlers and AddRPCHandlers.
func CollectRPCHandlers() map[string]func(json.RawMessage) (any, error) {
	mu.RLock()
	defer mu.RUnlock()

	handlers := make(map[string]func(json.RawMessage) (any, error))
	// Handlers from Registration structs.
	for _, reg := range plugins {
		maps.Copy(handlers, reg.RPCHandlers)
	}
	// Handlers from AddRPCHandlers (e.g., bgp/server/codec.go).
	maps.Copy(handlers, rpcHandlers)
	return handlers
}

// IsFatalOnConfigError returns true if the named plugin has FatalOnConfigError set.
func IsFatalOnConfigError(name string) bool {
	mu.RLock()
	defer mu.RUnlock()
	if reg, ok := plugins[name]; ok {
		return reg.FatalOnConfigError
	}
	return false
}

// rebuildFamilyIndexLocked rebuilds the family -> registration reverse index.
// Caller MUST hold mu write lock.
func rebuildFamilyIndexLocked() {
	m := make(map[string]*Registration, len(familyIndex))
	for _, reg := range plugins {
		for _, f := range reg.Families {
			m[f] = reg
		}
	}
	familyIndex = m
}

// PluginForFamily returns the plugin name that handles a given address family.
// Returns empty string if no plugin is registered for the family.
func PluginForFamily(family string) string {
	mu.RLock()
	defer mu.RUnlock()

	if reg := familyIndex[family]; reg != nil {
		return reg.Name
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

	if reg := familyIndex[family]; reg != nil && reg.InProcessNLRIDecoder != nil {
		return reg.InProcessNLRIDecoder(family, hexData)
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

	if reg := familyIndex[family]; reg != nil && reg.InProcessNLRIEncoder != nil {
		return reg.InProcessNLRIEncoder(family, args)
	}
	return "", fmt.Errorf("no NLRI encoder for family %s", family)
}

// RouteEncoderByFamily finds the plugin registered for a family and returns
// its in-process route encoder. Returns nil if no encoder is registered.
func RouteEncoderByFamily(family string) func(routeCmd, family string, localAS uint32, isIBGP, asn4, addPath bool) ([]byte, []byte, error) {
	mu.RLock()
	defer mu.RUnlock()

	if reg := familyIndex[family]; reg != nil && reg.InProcessRouteEncoder != nil {
		return reg.InProcessRouteEncoder
	}
	return nil
}

// ConfigNLRIBuilder finds the plugin registered for a family and returns
// its config NLRI builder. Returns nil if no builder is registered.
func ConfigNLRIBuilder(family string) func(map[string][]string, bool, bool) []byte {
	mu.RLock()
	defer mu.RUnlock()

	if reg := familyIndex[family]; reg != nil && reg.InProcessConfigNLRIBuilder != nil {
		return reg.InProcessConfigNLRIBuilder
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

// filterEntry pairs a filter with its registration metadata for sorting.
type filterEntry struct {
	name     string
	stage    int
	priority int
}

// collectFilterEntries returns sorted filter entries. Caller MUST hold mu.RLock.
func collectFilterEntries(hasFilter func(*Registration) bool) []filterEntry {
	var entries []filterEntry
	for _, reg := range plugins {
		if hasFilter(reg) {
			entries = append(entries, filterEntry{name: reg.Name, stage: reg.FilterStage, priority: reg.FilterPriority})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].stage != entries[j].stage {
			return entries[i].stage < entries[j].stage
		}
		if entries[i].priority != entries[j].priority {
			return entries[i].priority < entries[j].priority
		}
		return entries[i].name < entries[j].name
	})
	return entries
}

// sortedFilterPlugins returns plugin names with the given filter type,
// sorted by FilterStage, then FilterPriority, then by name.
func sortedFilterPlugins(hasFilter func(*Registration) bool) []string {
	mu.RLock()
	defer mu.RUnlock()

	entries := collectFilterEntries(hasFilter)
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.name
	}
	return names
}

// IngressFilters returns all registered ingress filter functions,
// sorted by FilterStage, then FilterPriority, then by plugin name.
// Called by the reactor to build the ingress filter chain.
func IngressFilters() []IngressFilterFunc {
	mu.RLock()
	defer mu.RUnlock()

	entries := collectFilterEntries(func(r *Registration) bool { return r.IngressFilter != nil })
	filters := make([]IngressFilterFunc, 0, len(entries))
	for _, e := range entries {
		if reg, ok := plugins[e.name]; ok {
			filters = append(filters, reg.IngressFilter)
		}
	}
	return filters
}

// IngressFilterNames returns the names of plugins with ingress filters,
// in execution order (sorted by FilterStage, then FilterPriority, then name).
func IngressFilterNames() []string {
	return sortedFilterPlugins(func(r *Registration) bool { return r.IngressFilter != nil })
}

// EgressFilters returns all registered egress filter functions,
// sorted by FilterStage, then FilterPriority, then by plugin name.
// Called by the reactor to build the egress filter chain.
func EgressFilters() []EgressFilterFunc {
	mu.RLock()
	defer mu.RUnlock()

	entries := collectFilterEntries(func(r *Registration) bool { return r.EgressFilter != nil })
	filters := make([]EgressFilterFunc, 0, len(entries))
	for _, e := range entries {
		if reg, ok := plugins[e.name]; ok {
			filters = append(filters, reg.EgressFilter)
		}
	}
	return filters
}

// EgressFilterNames returns the names of plugins with egress filters,
// in execution order (sorted by FilterStage, then FilterPriority, then name).
func EgressFilterNames() []string {
	return sortedFilterPlugins(func(r *Registration) bool { return r.EgressFilter != nil })
}

// Reset clears the registry. Only for use in tests.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	plugins = make(map[string]*Registration)
	familyIndex = make(map[string]*Registration)
	filterTypes = make(map[string]string)
	attrModHandlers = make(map[uint8]AttrModHandler)
	rpcHandlers = make(map[string]func(json.RawMessage) (any, error))
	metricsRegistry = nil
	eventBusInstance = nil
}

// RegistrySnapshot holds a complete copy of the registry state for test save/restore.
type RegistrySnapshot struct {
	plugins          map[string]*Registration
	attrModHandlers  map[uint8]AttrModHandler
	rpcHandlers      map[string]func(json.RawMessage) (any, error)
	eventBusInstance any
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
	rh := make(map[string]func(json.RawMessage) (any, error), len(rpcHandlers))
	maps.Copy(rh, rpcHandlers)
	return RegistrySnapshot{plugins: ps, attrModHandlers: ah, rpcHandlers: rh, eventBusInstance: eventBusInstance}
}

// Restore replaces the registry with a previously saved snapshot. Only for use in tests.
func Restore(snap RegistrySnapshot) {
	mu.Lock()
	defer mu.Unlock()
	plugins = snap.plugins
	attrModHandlers = snap.attrModHandlers
	rpcHandlers = snap.rpcHandlers
	eventBusInstance = snap.eventBusInstance
	// Rebuild filterTypes from the restored plugins slice.
	filterTypes = make(map[string]string)
	for name, reg := range plugins {
		for _, ft := range reg.FilterTypes {
			filterTypes[ft] = name
		}
	}
	rebuildFamilyIndexLocked()
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

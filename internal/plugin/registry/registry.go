// Package registry provides compile-time plugin registration.
//
// Plugins register themselves via init() functions, and the registry
// provides query functions for CLI dispatch, engine startup, and decode routing.
//
// This is a leaf package with no dependencies on plugin implementations,
// preventing import cycles. Plugin packages import this package to register;
// consumer packages import this package to query.
//
// Thread safety: init() functions run sequentially in Go, so no mutex is
// needed during registration. The registry is write-once (during init) and
// read-many (during runtime).
package registry

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"sort"
	"strings"
)

// Registration describes a plugin's full metadata and handlers.
// Each plugin registers exactly one Registration via its init() function.
type Registration struct {
	// Required fields.
	Name        string                                      // Plugin name (e.g., "flowspec", "gr")
	Description string                                      // Human-readable description for help text
	RunEngine   func(engineConn, callbackConn net.Conn) int // Engine mode handler (RPC)
	CLIHandler  func(args []string) int                     // CLI handler for `ze bgp plugin <name>`

	// Optional metadata.
	RFCs            []string // Related RFC numbers (e.g., ["8955", "8956"])
	Families        []string // Address families handled (e.g., ["ipv4/flow", "ipv6/flow"])
	CapabilityCodes []uint8  // Capability codes decoded (e.g., [73] for FQDN)
	ConfigRoots     []string // Config roots wanted (e.g., ["bgp"])
	YANG            string   // YANG schema content (empty if none)

	// Optional handlers.
	ConfigureEngineLogger func(loggerName string)               // Configure logger for in-process engine mode
	InProcessDecoder      func(input, output *bytes.Buffer) int // Decode function for CLI fallback

	// In-process NLRI decode/encode: fast path for infrastructure code (text.go, update_text.go)
	// that avoids plugin package imports. Same semantics as the SDK's OnDecodeNLRI/OnEncodeNLRI
	// callbacks, but callable directly without RPC. External plugins use the RPC path instead.
	InProcessNLRIDecoder func(family, hex string) (string, error)           // (family, hex) → JSON
	InProcessNLRIEncoder func(family string, args []string) (string, error) // (family, args) → hex

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
)

// plugins is the global plugin registry, populated during init().
var plugins = make(map[string]*Registration)

// Register adds a plugin to the global registry.
// Must be called from init() functions only.
// Returns an error on invalid registration.
func Register(reg Registration) error {
	if reg.Name == "" {
		return ErrEmptyName
	}
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

	r := reg // copy
	plugins[reg.Name] = &r
	return nil
}

// Lookup returns the registration for a named plugin, or nil if not found.
func Lookup(name string) *Registration {
	return plugins[name]
}

// All returns all registrations sorted by name.
func All() []*Registration {
	names := Names()
	result := make([]*Registration, len(names))
	for i, name := range names {
		result[i] = plugins[name]
	}
	return result
}

// Names returns all registered plugin names sorted alphabetically.
func Names() []string {
	names := make([]string, 0, len(plugins))
	for name := range plugins {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Has returns true if a plugin with the given name is registered.
func Has(name string) bool {
	_, ok := plugins[name]
	return ok
}

// FamilyMap returns a map from address family to plugin name.
// Built from all registered plugins' Families fields.
func FamilyMap() map[string]string {
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
	for _, reg := range plugins {
		if slices.Contains(reg.Families, family) {
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
	for _, reg := range plugins {
		if reg.InProcessNLRIEncoder != nil && slices.Contains(reg.Families, family) {
			return reg.InProcessNLRIEncoder(family, args)
		}
	}
	return "", fmt.Errorf("no NLRI encoder for family %s", family)
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

// Reset clears the registry. Only for use in tests.
func Reset() {
	plugins = make(map[string]*Registration)
}

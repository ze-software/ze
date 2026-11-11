// Design: docs/architecture/core-design.md -- sysctl known-keys registry
// Detail: known_linux.go -- Linux sysctl key registrations
//
// Package sysctl provides a registry of known kernel tunables. Plugins
// register keys at init time with metadata (type, range, description,
// platform). The sysctl plugin uses this registry for validation, tab
// completion, and describe output. Unknown keys bypass validation.
//
// Follows the internal/core/family/ pattern: leaf package with types +
// MustRegister, no plugin dependencies. Imported by plugin init() functions.
package sysctl

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// ValueType describes the type of a sysctl value for validation.
type ValueType int

const (
	// TypeBool accepts "0" or "1".
	TypeBool ValueType = iota
	// TypeInt accepts any integer string.
	TypeInt
	// TypeIntRange accepts integers in [Min, Max].
	TypeIntRange
)

// Platform restricts a key to specific operating systems.
type Platform int

const (
	// PlatformAll means the key is available on all platforms.
	PlatformAll Platform = iota
	// PlatformLinux means the key is Linux-only.
	PlatformLinux
	// PlatformDarwin means the key is Darwin-only.
	PlatformDarwin
)

// KeyDef describes a known sysctl key.
type KeyDef struct {
	Name        string    // Kernel-native name (e.g., "net.ipv4.conf.all.forwarding")
	Type        ValueType // Value type for validation
	Min         int       // Minimum value (TypeIntRange only)
	Max         int       // Maximum value (TypeIntRange only)
	Description string    // Human-readable description
	Platform    Platform  // Platform availability
	Template    bool      // True if Name contains <iface> placeholder

	// Propagates names the per-interface templates the kernel rewrites as a
	// side effect of a write to this key. Linux copies a write of
	// net.ipv6.conf.all.forwarding into every device's forwarding (and zeroes
	// force_forwarding when the value is 0), so after such a write the sysctl
	// store re-applies each instance it manages; the kernel's copy stands on
	// every other one. Empty for a key whose write touches nothing else.
	Propagates []string

	// Constraint refuses a value the Type admits but Ze cannot run with: the
	// key's own registration declares it, so no caller names the key. It runs
	// after the type and range check, on every path Validate guards (config
	// verify and apply, a transient `set sysctl`, a plugin default), and returns
	// nil to accept the value or an operator error naming the problem, why and
	// the fix. Nil for a key whose type is its whole rule. MUST be safe for
	// concurrent use and MUST NOT write a sysctl.
	Constraint func(value string) error
}

var (
	mu       sync.RWMutex
	registry = map[string]KeyDef{}
)

// MustRegister registers a known sysctl key. Panics on duplicate registration.
// Called from plugin init() functions where a duplicate indicates a programming bug.
func MustRegister(k KeyDef) {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := registry[k.Name]; exists {
		panic("BUG: sysctl: duplicate registration for " + strconv.Quote(k.Name))
	}
	registry[k.Name] = k
}

// Lookup returns the key definition for a known key, or false if not known.
func Lookup(name string) (KeyDef, bool) {
	mu.RLock()
	defer mu.RUnlock()

	k, ok := registry[name]
	return k, ok
}

// MatchTemplate checks if a concrete key (e.g., "net.ipv4.conf.eth0.forwarding")
// matches any registered template key (e.g., "net.ipv4.conf.<iface>.forwarding").
// Returns the template KeyDef and true if matched, or zero value and false.
// Note: if templates overlap (e.g., sharing a prefix), the first match wins.
// Current registrations avoid overlap; callers should not depend on match order.
func MatchTemplate(concreteKey string) (KeyDef, bool) {
	mu.RLock()
	defer mu.RUnlock()

	for _, k := range registry {
		if !k.Template {
			continue
		}
		if matchesTemplate(k.Name, concreteKey) {
			return k, true
		}
	}
	return KeyDef{}, false
}

// matchesTemplate checks if a concrete key matches a template pattern.
// The template has exactly one <iface> placeholder segment.
func matchesTemplate(template, concrete string) bool {
	prefix, suffix, ok := strings.Cut(template, "<iface>")
	if !ok {
		return false
	}
	if !strings.HasPrefix(concrete, prefix) {
		return false
	}
	if !strings.HasSuffix(concrete, suffix) {
		return false
	}
	// The middle part (interface name) must be non-empty.
	middle := concrete[len(prefix) : len(concrete)-len(suffix)]
	return middle != ""
}

// All returns a copy of all registered known keys.
func All() []KeyDef {
	mu.RLock()
	defer mu.RUnlock()

	result := make([]KeyDef, 0, len(registry))
	for _, k := range registry {
		result = append(result, k)
	}
	return result
}

// Validate checks a value against the known key's type constraints.
// Returns nil for unknown keys (they bypass validation).
// Returns an error if the value does not match the key's type or range, or if
// the key's Constraint refuses it.
func Validate(key, value string) error {
	mu.RLock()
	k, ok := registry[key]
	mu.RUnlock()

	if !ok {
		// Also try template matching for per-interface keys.
		k, ok = MatchTemplate(key)
		if !ok {
			return nil // Unknown key: no validation.
		}
	}

	if err := validateValue(k, value); err != nil {
		return err
	}
	if k.Constraint != nil {
		return k.Constraint(value)
	}
	return nil
}

func validateValue(k KeyDef, value string) error {
	switch k.Type {
	case TypeBool:
		if value != "0" && value != "1" {
			return fmt.Errorf("sysctl %s: value %q is not a valid bool (must be 0 or 1)", k.Name, value)
		}
	case TypeInt:
		if _, err := strconv.Atoi(value); err != nil {
			return fmt.Errorf("sysctl %s: value %q is not a valid integer: %w", k.Name, value, err)
		}
	case TypeIntRange:
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("sysctl %s: value %q is not a valid integer: %w", k.Name, value, err)
		}
		if v < k.Min || v > k.Max {
			return fmt.Errorf("sysctl %s: value %d not in range [%d, %d]", k.Name, v, k.Min, k.Max)
		}
	default:
		panic("BUG: sysctl: invalid registered value type")
	}
	return nil
}

// ResetRegistry clears all registrations. Only for use in tests.
func ResetRegistry() {
	mu.Lock()
	defer mu.Unlock()
	registry = map[string]KeyDef{}
}

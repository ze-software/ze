// Design: plan/learned/891-granular-debug.md -- debug YANG module registration
// Related: internal/component/config/yang/register.go -- same pattern for config YANG

package yang

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Module holds a debug YANG module registered via init().
type Module struct {
	Name    string
	Content string
	Prefix  string   // module tree prefix (e.g. "bgp" matches bgp.*)
	Flags   []string // valid flag names declared in this module
	Scopes  []string // valid scope kinds (e.g. "neighbor", "group")
}

var modules []Module

// RegisterModule registers a debug YANG module with its metadata.
// Called from init() in packages that own debug YANG files.
// The Flags and Scopes fields are derived from the YANG content by the registering
// package; they allow validation without runtime YANG parsing.
func RegisterModule(m Module) {
	modules = append(modules, m)
}

func prefixDot(prefix string) string {
	var tb textbuf.Buffer
	return tb.Str(prefix).Byte('.').String()
}

func matchesPrefix(module, prefix string) bool {
	return module == prefix || strings.HasPrefix(module, prefixDot(prefix))
}

// HasModule returns true if any debug YANG module covers the given module prefix.
func HasModule(module string) bool {
	for _, m := range modules {
		if matchesPrefix(module, m.Prefix) {
			return true
		}
	}
	return false
}

// ValidateFlag checks if a flag name is valid for a given module.
// Matches the module against registered prefixes: "bgp.reactor" matches prefix "bgp".
func ValidateFlag(module, flag string) bool {
	for _, m := range modules {
		if matchesPrefix(module, m.Prefix) {
			return slices.Contains(m.Flags, flag)
		}
	}
	return false
}

// ValidateScope checks if a scope kind is valid for a given module.
func ValidateScope(module, kind string) bool {
	for _, m := range modules {
		if matchesPrefix(module, m.Prefix) {
			return slices.Contains(m.Scopes, kind)
		}
	}
	return false
}

// FlagsFor returns the valid flag names for a module.
func FlagsFor(module string) []string {
	for _, m := range modules {
		if matchesPrefix(module, m.Prefix) {
			return m.Flags
		}
	}
	return nil
}

// ResetForTest clears the registry. Only for use in tests.
func ResetForTest() {
	modules = nil
}

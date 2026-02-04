// Package yang provides YANG schema loading and validation for ze.
package yang

import (
	"fmt"
	"sort"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"
)

// Metadata contains extracted information from a YANG module.
type Metadata struct {
	Module    string   // YANG module name (e.g., "ze-bgp")
	Namespace string   // YANG namespace URI (e.g., "urn:ze:bgp")
	Imports   []string // List of imported module names (sorted)
}

// ParseYANGMetadata parses YANG content and extracts metadata.
// Returns an error if the YANG is invalid or empty.
func ParseYANGMetadata(content string) (*Metadata, error) {
	if content == "" {
		return nil, fmt.Errorf("empty YANG content")
	}

	// Create a new modules collection for parsing
	modules := yang.NewModules()

	// Parse the YANG content
	if err := modules.Parse(content, "metadata.yang"); err != nil {
		return nil, fmt.Errorf("parse YANG: %w", err)
	}

	// Find the parsed module (there should be exactly one)
	var mod *yang.Module
	for _, m := range modules.Modules {
		mod = m
		break
	}

	if mod == nil {
		return nil, fmt.Errorf("no module found in YANG content")
	}

	return ExtractMetadata(mod), nil
}

// ExtractMetadata extracts metadata from a parsed goyang Module.
func ExtractMetadata(mod *yang.Module) *Metadata {
	meta := &Metadata{
		Module: mod.Name,
	}

	// Extract namespace
	if mod.Namespace != nil {
		meta.Namespace = mod.Namespace.Name
	}

	// Extract imports (sorted for consistent output)
	if len(mod.Import) > 0 {
		imports := make([]string, 0, len(mod.Import))
		for _, imp := range mod.Import {
			imports = append(imports, imp.Name)
		}
		sort.Strings(imports)
		meta.Imports = imports
	}

	return meta
}

// FormatNamespace converts a YANG namespace URI to display format.
// Converts "urn:ze:bgp" to "ze.bgp" for cleaner display.
// Non-URN namespaces are returned unchanged.
func FormatNamespace(ns string) string {
	if ns == "" {
		return ""
	}

	// Only format URN-style namespaces
	if strings.HasPrefix(ns, "urn:") {
		// Remove "urn:" prefix and replace ":" with "."
		return strings.ReplaceAll(ns[4:], ":", ".")
	}

	return ns
}

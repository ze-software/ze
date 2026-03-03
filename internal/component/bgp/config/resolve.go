// Design: docs/architecture/config/syntax.md — BGP template resolution and inheritance

package bgpconfig

import (
	"fmt"
	"maps"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// templateData holds parsed template information for inheritance resolution.
type templateData struct {
	named    map[string]*config.Tree // Named templates (inherit by name)
	patterns map[string]string       // Template patterns (for validation)
	globs    []PeerGlob              // Auto-matching glob patterns
}

// extractTemplateData extracts templates and glob patterns from the config tree.
// Shared by ResolveBGPTree (map-level resolution) and PeersFromConfigTree (route extraction).
func extractTemplateData(tree *config.Tree) templateData {
	td := templateData{
		named:    make(map[string]*config.Tree),
		patterns: make(map[string]string),
	}

	tmpl := tree.GetContainer("template")
	if tmpl == nil {
		return td
	}

	// New syntax: template { bgp { peer <pattern> { inherit-name <name>; ... } } }
	if bgpTmpl := tmpl.GetContainer("bgp"); bgpTmpl != nil {
		for _, entry := range bgpTmpl.GetListOrdered("peer") {
			pattern := entry.Key
			peerTree := entry.Value

			if inheritName, hasName := peerTree.Get("inherit-name"); hasName {
				td.named[inheritName] = peerTree
				td.patterns[inheritName] = pattern
			} else {
				td.globs = append(td.globs, PeerGlob{
					Pattern: pattern,
					Tree:    peerTree,
				})
			}
		}
	}

	// Legacy syntax: template { group <name> { ... } }
	maps.Copy(td.named, tmpl.GetList("group"))

	// Legacy syntax: template { match <pattern> { ... } } — auto-apply.
	for _, entry := range tmpl.GetListOrdered("match") {
		td.globs = append(td.globs, PeerGlob{
			Pattern: entry.Key,
			Tree:    entry.Value,
		})
	}

	return td
}

// resolveInheritedTrees returns template trees for a peer's inherit directives.
// Used by PeersFromConfigTree to extract routes from all template layers.
func resolveInheritedTrees(addr string, peerTree *config.Tree, td templateData) []*config.Tree {
	var result []*config.Tree

	// Check ordered list of inherit entries.
	for _, entry := range peerTree.GetListOrdered("inherit") {
		inheritName := entry.Key
		t, exists := td.named[inheritName]
		if !exists {
			continue // Validation already done by ResolveBGPTree
		}
		if pattern, hasPattern := td.patterns[inheritName]; hasPattern {
			if !IPGlobMatch(pattern, addr) {
				continue
			}
		}
		result = append(result, t)
	}

	// Also check single inherit value (backward compat).
	if len(result) == 0 {
		if inheritName, ok := peerTree.Get("inherit"); ok {
			if t, exists := td.named[inheritName]; exists {
				if pattern, hasPattern := td.patterns[inheritName]; hasPattern {
					if IPGlobMatch(pattern, addr) {
						result = append(result, t)
					}
				} else {
					result = append(result, t)
				}
			}
		}
	}

	return result
}

// ResolveBGPTree resolves template inheritance and returns the bgp block as map[string]any.
// Template resolution applies 3 layers per peer (in precedence order):
//  1. Auto-matching glob patterns (template.match or template.bgp.peer without inherit-name)
//  2. Named templates via 'inherit' directive (template.group or template.bgp.peer with inherit-name)
//  3. The peer's own values (highest precedence)
//
// Each layer deep-merges into the previous, so containers like capability are merged
// at the key level, not replaced wholesale.
func ResolveBGPTree(tree *config.Tree) (map[string]any, error) {
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return nil, fmt.Errorf("missing required bgp { } block")
	}

	td := extractTemplateData(tree)

	// Build result map with global bgp values.
	result := bgp.ToMap()

	// Remove raw peer list — we'll rebuild it with resolved peers.
	delete(result, "peer")

	// Resolve each peer.
	peerEntries := bgp.GetListOrdered("peer")
	if len(peerEntries) == 0 {
		return result, nil
	}

	peerMap := make(map[string]any, len(peerEntries))
	for _, entry := range peerEntries {
		addr := entry.Key
		peerTree := entry.Value

		resolved := make(map[string]any)

		// Layer 1: Apply matching globs.
		for _, glob := range td.globs {
			if IPGlobMatch(glob.Pattern, addr) {
				deepMergeMaps(resolved, glob.Tree.ToMap())
			}
		}

		// Layer 2: Apply inherited templates.
		if err := resolveInheritedTemplates(addr, peerTree, td.named, td.patterns, resolved); err != nil {
			return nil, fmt.Errorf("bgp.peer %s: %w", addr, err)
		}

		// Layer 3: Apply peer's own values (highest precedence).
		deepMergeMaps(resolved, peerTree.ToMap())

		// Remove config directives that aren't peer values.
		delete(resolved, "inherit")
		delete(resolved, "inherit-name")

		peerMap[addr] = resolved
	}

	result["peer"] = peerMap
	return result, nil
}

// resolveInheritedTemplates handles the 'inherit' directive for a peer.
// Supports both list-ordered inherit (multiple templates) and single value.
func resolveInheritedTemplates(addr string, peerTree *config.Tree, templates map[string]*config.Tree, templatePatterns map[string]string, resolved map[string]any) error {
	var inheritedTemplates []*config.Tree

	// Check ordered list of inherit entries.
	for _, entry := range peerTree.GetListOrdered("inherit") {
		inheritName := entry.Key
		t, exists := templates[inheritName]
		if !exists {
			return fmt.Errorf("inherit %q: template not found", inheritName)
		}
		if err := validateTemplatePattern(inheritName, addr, templatePatterns); err != nil {
			return err
		}
		inheritedTemplates = append(inheritedTemplates, t)
	}

	// Also check single inherit value (backward compat with old config syntax).
	if len(inheritedTemplates) == 0 {
		if inheritName, ok := peerTree.Get("inherit"); ok {
			t, exists := templates[inheritName]
			if !exists {
				return fmt.Errorf("inherit %q: template not found", inheritName)
			}
			if err := validateTemplatePattern(inheritName, addr, templatePatterns); err != nil {
				return err
			}
			inheritedTemplates = append(inheritedTemplates, t)
		}
	}

	// Apply inherited templates in order.
	for _, tmpl := range inheritedTemplates {
		tmplMap := tmpl.ToMap()
		// Remove inherit-name from template map (config metadata, not a peer value).
		delete(tmplMap, "inherit-name")
		deepMergeMaps(resolved, tmplMap)
	}

	return nil
}

// validateTemplatePattern checks that the peer address matches the template's pattern.
func validateTemplatePattern(inheritName, addr string, templatePatterns map[string]string) error {
	pattern, hasPattern := templatePatterns[inheritName]
	if !hasPattern {
		return nil
	}
	if !IPGlobMatch(pattern, addr) {
		return fmt.Errorf("inherit %q: peer %s does not match template pattern %q", inheritName, addr, pattern)
	}
	return nil
}

// deepMergeMaps recursively merges src into dst.
// For leaf values (non-map), src overwrites dst.
// For map values, keys are merged recursively so both sides contribute.
func deepMergeMaps(dst, src map[string]any) {
	for k, srcVal := range src {
		srcMap, srcIsMap := srcVal.(map[string]any)
		if !srcIsMap {
			dst[k] = srcVal
			continue
		}
		dstMap, dstIsMap := dst[k].(map[string]any)
		if !dstIsMap {
			// dst doesn't have a map here — copy src map.
			dst[k] = srcVal
			continue
		}
		// Both are maps — recurse.
		deepMergeMaps(dstMap, srcMap)
	}
}

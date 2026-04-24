// Design: docs/architecture/config/yang-config-design.md -- YANG command tree extensions
// Related: validator_registry.go -- ze:validate extension (same pattern)

package yang

import (
	"sort"
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"codeberg.org/thomas-mangin/ze/internal/component/command"
)

// cmdModuleSuffix identifies YANG command tree modules by naming convention.
const cmdModuleSuffix = "-cmd"

// WireMethodToPaths walks all -cmd YANG modules and builds a map from
// WireMethod (ze:command argument) to all CLI paths (space-joined tree paths).
// Multiple paths per wire method represent command aliases.
func WireMethodToPaths(loader *Loader) map[string][]string {
	result := make(map[string][]string)
	if loader == nil {
		return result
	}
	tree := BuildCommandTree(loader)
	collectPaths(tree, "", result)
	return result
}

// WireMethodToPath returns the shortest CLI path for each wire method.
// Deterministic: when multiple aliases exist, the lexicographically smallest
// path is chosen so restarts produce consistent authz context.
// Callers that need all aliases should use WireMethodToPaths.
func WireMethodToPath(loader *Loader) map[string]string {
	paths := WireMethodToPaths(loader)
	result := make(map[string]string, len(paths))
	for method, ps := range paths {
		if len(ps) == 0 {
			continue
		}
		best := ps[0]
		for _, p := range ps[1:] {
			if p < best {
				best = p
			}
		}
		result[method] = best
	}
	return result
}

// collectPaths recursively walks the command tree and collects WireMethod -> path mappings.
func collectPaths(node *command.Node, prefix string, result map[string][]string) {
	if node == nil {
		return
	}
	for name, child := range node.Children {
		path := name
		if prefix != "" {
			path = prefix + " " + name
		}
		if child.WireMethod != "" {
			result[child.WireMethod] = append(result[child.WireMethod], path)
		}
		collectPaths(child, path, result)
	}
}

// PathToDescription walks all -cmd YANG modules and builds a map from
// CLI path (space-joined) to description. Used to populate help text
// when registering commands in the dispatcher.
func PathToDescription(loader *Loader) map[string]string {
	result := make(map[string]string)
	if loader == nil {
		return result
	}
	tree := BuildCommandTree(loader)
	collectDescriptions(tree, "", result)
	return result
}

// collectDescriptions recursively walks the command tree and collects path -> description.
func collectDescriptions(node *command.Node, prefix string, result map[string]string) {
	if node == nil {
		return
	}
	for name, child := range node.Children {
		path := name
		if prefix != "" {
			path = prefix + " " + name
		}
		if child.Description != "" {
			result[path] = child.Description
		}
		collectDescriptions(child, path, result)
	}
}

// BuildCommandTree walks all -cmd YANG modules in the loader and builds
// a merged command.Node tree. Multiple modules contributing to the same
// container path (e.g., 4 modules defining peer > ...) are merged.
// Only nodes with ze:command get a Description (from the YANG description).
// Grouping containers (no ze:command) become navigation-only branches.
func BuildCommandTree(loader *Loader) *command.Node {
	root := &command.Node{Children: make(map[string]*command.Node)}

	// Collect and sort -cmd module names for deterministic merge order.
	var cmdModules []string
	for _, name := range loader.ModuleNames() {
		if strings.HasSuffix(name, cmdModuleSuffix) {
			cmdModules = append(cmdModules, name)
		}
	}
	sort.Strings(cmdModules)

	for _, name := range cmdModules {
		entry := loader.GetEntry(name)
		if entry == nil || entry.Dir == nil {
			continue
		}
		mergeYANGEntry(root, entry)
	}

	return root
}

// mergeYANGEntry recursively walks a YANG entry's children and merges them
// into the command.Node tree. config false containers become tree nodes.
// Nodes with ze:command get their YANG description as the node Description.
func mergeYANGEntry(node *command.Node, entry *gyang.Entry) {
	if entry == nil || entry.Dir == nil {
		return
	}
	for name, child := range entry.Dir {
		// Only walk config false containers (command tree nodes).
		// Note: -cmd.yang files must explicitly mark every container as config false.
		// goyang may not propagate inherited config false to all descendants.
		if child.Config != gyang.TSFalse {
			continue
		}

		if node.Children == nil {
			node.Children = make(map[string]*command.Node)
		}

		target, exists := node.Children[name]
		if !exists {
			target = &command.Node{Name: name}
			node.Children[name] = target
		}

		// ze:command nodes get their WireMethod and description (executable commands).
		// Grouping containers also get their YANG description for help text.
		wm := GetCommandExtension(child)
		if wm != "" && target.WireMethod == "" {
			target.WireMethod = wm
			target.Description = child.Description
		} else if target.Description == "" && child.Description != "" {
			target.Description = child.Description
		}

		// Recurse into children (merge overlapping branches from multiple modules).
		mergeYANGEntry(target, child)
	}
}

// GetCommandExtension reads the ze:command extension from a YANG entry.
// Returns the WireMethod handler string (e.g., "ze-bgp:peer-list"), or empty
// string if the entry has no ze:command extension.
func GetCommandExtension(entry *gyang.Entry) string {
	if entry == nil {
		return ""
	}
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:command" || strings.HasSuffix(ext.Keyword, ":command") {
			return ext.Argument
		}
	}
	return ""
}

// HasCommandExtension returns true if the YANG entry has the ze:command extension.
// This marks a config false container as an executable command.
func HasCommandExtension(entry *gyang.Entry) bool {
	return GetCommandExtension(entry) != ""
}

// HasEditShortcutExtension returns true if the YANG entry has the ze:edit-shortcut extension.
// This marks a command as available in edit mode as a shortcut (e.g., commit, save).
func HasEditShortcutExtension(entry *gyang.Entry) bool {
	if entry == nil {
		return false
	}
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:edit-shortcut" || strings.HasSuffix(ext.Keyword, ":edit-shortcut") {
			return true
		}
	}
	return false
}

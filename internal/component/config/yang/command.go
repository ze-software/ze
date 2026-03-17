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
		// Grouping containers (no ze:command) stay empty.
		wm := GetCommandExtension(child)
		if wm != "" && target.WireMethod == "" {
			target.WireMethod = wm
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

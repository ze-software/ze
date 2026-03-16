// Design: docs/architecture/config/yang-config-design.md -- YANG command tree extensions
// Related: validator_registry.go -- ze:validate extension (same pattern)

package yang

import (
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"
)

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

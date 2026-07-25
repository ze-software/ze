// Design: plan/learned/891-granular-debug.md -- debug subsystem validation and matching
// Related: slogutil.go -- SetLevel, ListLevels, Subsystems used for debug application

package slogutil

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ValidateSubsystem checks if a name matches a registered subsystem or is a valid
// hierarchical prefix of one. Returns true if the name is valid for debug toggle.
func ValidateSubsystem(name string) bool {
	if name == "all" {
		return true
	}
	if name == "" || strings.ContainsAny(name, "/\x00") {
		return false
	}
	var tb textbuf.Buffer
	prefix := tb.Str(name).Byte('.').String()
	for _, info := range Subsystems() {
		if info.Name == name || strings.HasPrefix(info.Name, prefix) {
			return true
		}
	}
	return false
}

// SubsystemsMatching returns all subsystem names that match a debug key name.
// For "all", returns every subsystem.
// For "bgp", returns all subsystems starting with "bgp." plus "bgp" itself if registered.
func SubsystemsMatching(name string) []string {
	if name == "all" {
		infos := Subsystems()
		names := make([]string, len(infos))
		for i, info := range infos {
			names[i] = info.Name
		}
		return names
	}
	var tb textbuf.Buffer
	prefix := tb.Str(name).Byte('.').String()
	var matches []string
	for _, info := range Subsystems() {
		if info.Name == name || strings.HasPrefix(info.Name, prefix) {
			matches = append(matches, info.Name)
		}
	}
	return matches
}

// RestoreLevel restores a subsystem's log level to its configured (non-debug) value.
// Re-derives the level from environment/config, which is the source of truth.
func RestoreLevel(subsystem string) {
	v := getLogEnv(subsystem)
	if v == "" {
		_ = SetLevel(subsystem, "warn")
		return
	}
	lvl, enabled := parseLevel(v)
	if !enabled {
		return
	}
	_ = SetLevel(subsystem, LevelString(lvl))
}

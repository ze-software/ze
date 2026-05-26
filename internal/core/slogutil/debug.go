// Design: docs/architecture/config/environment.md -- debug flag resolution and logging integration
// Related: slogutil.go -- SetLevel, ListLevels, Subsystems used for debug application

package slogutil

import (
	"strings"
)

// DebugSource describes why a subsystem's debug flag has its current value.
type DebugSource string

const (
	DebugSourceGlobal   DebugSource = "global"
	DebugSourceExplicit DebugSource = "explicit"
	DebugSourceDefault  DebugSource = "default"
)

// DebugState holds the resolved debug state for a single subsystem.
type DebugState struct {
	Name    string
	Enabled bool
	Source  DebugSource
}

// DebugStore is the interface needed to read/write debug flags.
// Satisfied by *zefs.BlobStore and test fakes.
type DebugStore interface {
	ReadFile(name string) ([]byte, error)
	List(prefix string) []string
}

// ResolveDebugStates returns the debug state for every known subsystem.
// Resolution order: global override > per-subsystem key > hierarchical parent key > default (off).
func ResolveDebugStates(store DebugStore) []DebugState {
	globalOn := readFlag(store, "state/debug/all")

	explicitKeys := make(map[string]bool)
	keys := store.List("state/debug")
	for _, key := range keys {
		name := strings.TrimPrefix(key, "state/debug/")
		if name != "all" && name != "" {
			explicitKeys[name] = readFlag(store, key)
		}
	}

	subsystems := Subsystems()
	states := make([]DebugState, 0, len(subsystems))

	for _, info := range subsystems {
		enabled, source := resolveOne(info.Name, globalOn, explicitKeys)
		states = append(states, DebugState{
			Name:    info.Name,
			Enabled: enabled,
			Source:  source,
		})
	}

	return states
}

// resolveOne determines the debug state for a single subsystem.
func resolveOne(name string, globalOn bool, explicitKeys map[string]bool) (bool, DebugSource) {
	if globalOn {
		return true, DebugSourceGlobal
	}

	if on, ok := explicitKeys[name]; ok {
		return on, DebugSourceExplicit
	}

	parts := strings.Split(name, ".")
	for i := len(parts) - 1; i > 0; i-- {
		parent := strings.Join(parts[:i], ".")
		if on, ok := explicitKeys[parent]; ok {
			return on, DebugSourceExplicit
		}
	}

	return false, DebugSourceDefault
}

// ApplyDebugFlags reads debug keys from the store and applies them to running loggers.
// Call after ApplyLogConfig during daemon startup.
func ApplyDebugFlags(store DebugStore) {
	states := ResolveDebugStates(store)
	for _, s := range states {
		if s.Enabled {
			_ = SetLevel(s.Name, levelDebug)
		}
	}
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

// readFlag reads a debug flag from the store. Returns false if the key doesn't exist
// or the value is not "on".
func readFlag(store DebugStore, key string) bool {
	data, err := store.ReadFile(key)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == "on"
}

// ValidateSubsystem checks if a name matches a registered subsystem or is a valid
// hierarchical prefix of one. Returns true if the name is valid for debug enable/disable.
func ValidateSubsystem(name string) bool {
	if name == "all" {
		return true
	}
	if name == "" || strings.ContainsAny(name, "/\x00") {
		return false
	}
	for _, info := range Subsystems() {
		if info.Name == name || strings.HasPrefix(info.Name, name+".") {
			return true
		}
	}
	return false
}

// SubsystemsMatching returns all subsystem names that match a debug key name.
// For "bgp", returns all subsystems starting with "bgp." plus "bgp" itself if registered.
func SubsystemsMatching(name string) []string {
	var matches []string
	for _, info := range Subsystems() {
		if info.Name == name || strings.HasPrefix(info.Name, name+".") {
			matches = append(matches, info.Name)
		}
	}
	return matches
}

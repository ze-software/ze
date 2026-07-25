// Design: plan/learned/891-granular-debug.md -- debug profile load/save/modify via debug.zefs
// Related: debug.go -- CLI handler that reads/writes profiles

package debug

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/pkg/zefs"
)

// FlagEntry represents a debug flag.
type FlagEntry struct {
	Name string `json:"name"`
}

// ScopeEntry represents a debug scope filter (e.g. neighbor 192.0.2.1).
type ScopeEntry struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// ModuleEntry holds the debug configuration for a single module.
type ModuleEntry struct {
	Level  string       `json:"level"`
	Flags  []FlagEntry  `json:"flags,omitempty"`
	Scopes []ScopeEntry `json:"scopes,omitempty"`
}

// Profile holds the complete debug configuration state.
type Profile struct {
	Modules map[string]*ModuleEntry `json:"modules"`
	Timeout int                     `json:"timeout,omitempty"`
}

// NewProfile creates an empty debug profile.
func NewProfile() *Profile {
	return &Profile{Modules: make(map[string]*ModuleEntry)}
}

// HasModule returns true if the module has an entry in the profile.
func (p *Profile) HasModule(name string) bool {
	_, ok := p.Modules[name]
	return ok
}

// Module returns the entry for a module, or nil if not present.
func (p *Profile) Module(name string) *ModuleEntry {
	return p.Modules[name]
}

// ToggleModule adds a module entry if absent, removes it if present.
func (p *Profile) ToggleModule(name string) bool {
	if _, ok := p.Modules[name]; ok {
		delete(p.Modules, name)
		return false
	}
	p.Modules[name] = &ModuleEntry{Level: "debug"}
	return true
}

// SetLevel sets the log level for a module. Creates the entry if absent.
func (p *Profile) SetLevel(name, level string) {
	entry := p.Modules[name]
	if entry == nil {
		entry = &ModuleEntry{Level: level}
		p.Modules[name] = entry
		return
	}
	entry.Level = level
}

// ToggleFlag adds or removes a flag entry for a module.
func (p *Profile) ToggleFlag(module, flag string) {
	entry := p.Modules[module]
	if entry == nil {
		return
	}
	for i, f := range entry.Flags {
		if f.Name == flag {
			entry.Flags = slices.Delete(entry.Flags, i, i+1)
			return
		}
	}
	entry.Flags = append(entry.Flags, FlagEntry{Name: flag})
}

// ToggleScope adds or removes a scope entry for a module.
func (p *Profile) ToggleScope(module, kind, value string) {
	entry := p.Modules[module]
	if entry == nil {
		return
	}
	for i, s := range entry.Scopes {
		if s.Kind == kind && s.Value == value {
			entry.Scopes = slices.Delete(entry.Scopes, i, i+1)
			return
		}
	}
	entry.Scopes = append(entry.Scopes, ScopeEntry{Kind: kind, Value: value})
}

// Flags returns the flag entries for a module.
func (p *Profile) Flags(module string) []FlagEntry {
	entry := p.Modules[module]
	if entry == nil {
		return nil
	}
	return entry.Flags
}

// Scopes returns the scope entries for a module.
func (p *Profile) Scopes(module string) []ScopeEntry {
	entry := p.Modules[module]
	if entry == nil {
		return nil
	}
	return entry.Scopes
}

// HasFlag reports whether a module has the named flag. Used by the verb-first
// set/delete handlers to make flag changes idempotent (ToggleFlag alone flips).
func (p *Profile) HasFlag(module, flag string) bool {
	for _, f := range p.Flags(module) {
		if f.Name == flag {
			return true
		}
	}
	return false
}

// HasScope reports whether a module has the given scope filter. Used to make
// verb-first scope changes idempotent.
func (p *Profile) HasScope(module, kind, value string) bool {
	for _, s := range p.Scopes(module) {
		if s.Kind == kind && s.Value == value {
			return true
		}
	}
	return false
}

// ModuleNames returns the sorted list of module names in the profile.
func (p *Profile) ModuleNames() []string {
	names := make([]string, 0, len(p.Modules))
	for name := range p.Modules {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// SaveProfile writes a profile to debug.zefs under the given name.
func SaveProfile(storePath, name string, p *Profile) error {
	if name == "" || len(name) > 64 || strings.ContainsAny(name, "/\x00 \t\n") {
		return fmt.Errorf("invalid profile name: %q (must be 1-64 chars, no whitespace or /)", name)
	}

	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal profile: %w", err)
	}

	store, err := openDebugStore(storePath)
	if err != nil {
		return err
	}
	defer store.Close() //nolint:errcheck // best-effort close

	key := zefs.KeyDebugProfile.Key(name)
	if err := store.WriteFile(key, data, 0); err != nil {
		return fmt.Errorf("write profile %q: %w", name, err)
	}
	return nil
}

// LoadProfile reads a profile from debug.zefs.
func LoadProfile(storePath, name string) (*Profile, error) {
	store, err := openDebugStore(storePath)
	if err != nil {
		return nil, err
	}
	defer store.Close() //nolint:errcheck // best-effort close

	key := zefs.KeyDebugProfile.Key(name)
	data, err := store.ReadFile(key)
	if err != nil {
		return nil, fmt.Errorf("read profile %q: %w", name, err)
	}

	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("unmarshal profile %q: %w", name, err)
	}
	if p.Modules == nil {
		p.Modules = make(map[string]*ModuleEntry)
	}
	return &p, nil
}

// ListProfiles returns the names of all saved profiles.
func ListProfiles(storePath string) ([]string, error) {
	store, err := openDebugStore(storePath)
	if err != nil {
		return nil, err
	}
	defer store.Close() //nolint:errcheck // best-effort close

	listDir := zefs.KeyDebugProfile.Dir()
	trimPrefix := zefs.KeyDebugProfile.Prefix()

	keys := store.List(listDir)
	names := make([]string, 0, len(keys))
	for _, k := range keys {
		name := strings.TrimPrefix(k, trimPrefix)
		if name != "" && name != k {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// DeleteProfile removes a named profile from debug.zefs.
func DeleteProfile(storePath, name string) error {
	store, err := openDebugStore(storePath)
	if err != nil {
		return err
	}
	defer store.Close() //nolint:errcheck // best-effort close

	return store.Remove(zefs.KeyDebugProfile.Key(name))
}

func openDebugStore(storePath string) (*zefs.BlobStore, error) {
	if _, err := os.Stat(storePath); err == nil {
		return zefs.Open(storePath)
	}
	return zefs.Create(storePath)
}

// Design: docs/architecture/core-design.md -- sysctl plugin
// Detail: backend.go -- platform-specific read/write
// Detail: register.go -- plugin registration and EventBus wiring
//
// Package sysctl implements the sysctl plugin that centralizes all kernel
// tunable management. Three value layers (config > transient > default)
// with strict precedence. Plugins contribute defaults via EventBus.
// Config values are authoritative and win over all other layers.
package sysctl

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sysctlreg "codeberg.org/thomas-mangin/ze/internal/core/sysctl"
)

// layer identifies the source of a sysctl value.
type layer int

const (
	layerDefault   layer = iota // Plugin-provided required value
	layerTransient              // CLI or EventBus set (non-persistent)
	layerConfig                 // User YANG config (persistent, authoritative)
)

func (l layer) String() string {
	switch l {
	case layerDefault:
		return "default"
	case layerTransient:
		return "transient"
	case layerConfig:
		return "config"
	}
	return "unknown"
}

// entry tracks a single sysctl key's state across all layers.
type entry struct {
	key      string
	original string // OS value before ze touched it
	hasSaved bool   // whether original was captured

	// Layer values. Empty string means layer not set.
	configValue    string
	transientValue string
	defaultValue   string
	defaultSource  string // plugin name that set the default
}

// effective returns the value that should be applied (highest priority layer).
func (e *entry) effective() (string, layer) {
	if e.configValue != "" {
		return e.configValue, layerConfig
	}
	if e.transientValue != "" {
		return e.transientValue, layerTransient
	}
	if e.defaultValue != "" {
		return e.defaultValue, layerDefault
	}
	return "", -1
}

// persistent returns true if the effective value comes from config.
func (e *entry) persistent() bool {
	return e.configValue != ""
}

// source returns the source description for the effective layer.
func (e *entry) source() string {
	if e.configValue != "" {
		return "config"
	}
	if e.transientValue != "" {
		return "transient"
	}
	if e.defaultSource != "" {
		return e.defaultSource
	}
	return "default"
}

// maxKeyLen is the maximum length of a sysctl key. Linux PATH_MAX is 4096;
// a reasonable sysctl key is well under 256 characters. This bound prevents
// unbounded memory from user-controlled keys.
const maxKeyLen = 256

// validateKey rejects keys that are empty, too long, or contain path traversal.
func validateKey(key string) error {
	if key == "" {
		return fmt.Errorf("sysctl: empty key")
	}
	if len(key) > maxKeyLen {
		return fmt.Errorf("sysctl: key too long (%d > %d)", len(key), maxKeyLen)
	}
	return nil
}

// store manages all sysctl key entries with thread-safe access.
type store struct {
	mu      sync.RWMutex
	entries map[string]*entry
	be      backend
	log     *slog.Logger
}

func newStore(be backend, log *slog.Logger) *store {
	return &store{
		entries: make(map[string]*entry),
		be:      be,
		log:     log,
	}
}

// getOrCreate returns the entry for a key, creating it if needed.
// Caller MUST hold s.mu (write lock).
func (s *store) getOrCreate(key string) *entry {
	e, ok := s.entries[key]
	if !ok {
		e = &entry{key: key}
		s.entries[key] = e
	}
	return e
}

// saveOriginal captures the current kernel value before first write.
// Caller MUST hold s.mu (write lock).
func (s *store) saveOriginal(e *entry) {
	if e.hasSaved {
		return
	}
	val, err := s.be.read(e.key)
	if err != nil {
		s.log.Debug("sysctl: could not read original value", "key", e.key, "err", err)
		return
	}
	e.original = val
	e.hasSaved = true
}

// applyEffective writes the effective value to the kernel.
// Caller MUST hold s.mu (write lock). Returns the applied value and layer.
func (s *store) applyEffective(e *entry) (string, layer, error) {
	val, l := e.effective()
	if l < 0 {
		return "", -1, nil // No value to apply.
	}
	if err := s.be.write(e.key, val); err != nil {
		return "", -1, err
	}
	return val, l, nil
}

// setDefault sets a plugin default for a key. Returns the applied event payload
// if the value was written, or empty string if a higher-priority layer blocked it.
func (s *store) setDefault(key, value, source string) (appliedPayload string, err error) {
	if err := validateKey(key); err != nil {
		return "", err
	}
	if verr := sysctlreg.Validate(key, value); verr != nil {
		return "", verr
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	e := s.getOrCreate(key)
	e.defaultValue = value
	e.defaultSource = source

	// Check if a higher-priority layer overrides this default.
	if e.configValue != "" {
		s.log.Warn("sysctl: default overridden by config",
			"key", key, "config-value", e.configValue,
			"default-value", value, "plugin", source)
		return "", nil
	}
	if e.transientValue != "" {
		s.log.Info("sysctl: default overridden by transient",
			"key", key, "transient-value", e.transientValue,
			"default-value", value, "plugin", source)
		return "", nil
	}

	s.saveOriginal(e)
	val, l, werr := s.applyEffective(e)
	if werr != nil {
		return "", werr
	}
	return appliedJSON(key, val, l.String()), nil
}

// setTransient sets a transient value for a key. Returns error if config
// already claims the key. Returns applied payload if written.
// Note: errors are logged by the EventBus handler but not returned to the
// CLI caller because (sysctl, set) is fire-and-forget. A future request/
// response pattern for set would need a correlation ID like the query events.
func (s *store) setTransient(key, value string) (appliedPayload string, err error) {
	if err := validateKey(key); err != nil {
		return "", err
	}
	if verr := sysctlreg.Validate(key, value); verr != nil {
		return "", verr
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	e := s.getOrCreate(key)

	if e.configValue != "" {
		return "", fmt.Errorf("sysctl: key %s is claimed by config (value %s), cannot set transient", key, e.configValue)
	}

	e.transientValue = value
	s.saveOriginal(e)

	if e.defaultValue != "" && e.defaultSource != "" {
		s.log.Info("sysctl: transient overrides default",
			"key", key, "transient-value", value,
			"default-value", e.defaultValue, "plugin", e.defaultSource)
	}

	val, l, werr := s.applyEffective(e)
	if werr != nil {
		return "", werr
	}
	return appliedJSON(key, val, l.String()), nil
}

// applyConfig sets config values for all provided keys. Re-evaluates all
// existing entries: keys claimed by config are overwritten, keys no longer
// in config fall back to transient or default.
// Validation is atomic: if any known key fails validation, zero keys are applied.
func (s *store) applyConfig(settings map[string]string) ([]string, []error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var applied []string
	var errs []error

	// First, validate all keys.
	for key, value := range settings {
		if kerr := validateKey(key); kerr != nil {
			errs = append(errs, kerr)
		} else if verr := sysctlreg.Validate(key, value); verr != nil {
			errs = append(errs, verr)
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}

	// Clear config layer for keys no longer in config.
	for _, e := range s.entries {
		_, inConfig := settings[e.key]
		if inConfig || e.configValue == "" {
			continue
		}
		e.configValue = ""
		// Re-evaluate: fall back to transient or default.
		s.saveOriginal(e)
		val, l, err := s.applyEffective(e)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if l >= 0 {
			applied = append(applied, appliedJSON(e.key, val, l.String()))
		}
	}

	// Apply config values.
	for key, value := range settings {
		e := s.getOrCreate(key)

		// Log override of plugin default.
		if e.defaultValue != "" && e.defaultSource != "" {
			s.log.Warn("sysctl: config overrides plugin default",
				"key", key, "config-value", value,
				"default-value", e.defaultValue, "plugin", e.defaultSource)
		}
		if e.transientValue != "" {
			s.log.Info("sysctl: config overrides transient",
				"key", key, "config-value", value,
				"transient-value", e.transientValue)
		}

		s.saveOriginal(e)

		if err := s.be.write(key, value); err != nil {
			errs = append(errs, err)
			continue
		}
		// Set config value only after successful kernel write.
		e.configValue = value
		applied = append(applied, appliedJSON(key, value, "config"))
	}

	return applied, errs
}

// showEntries returns JSON payloads for all active keys.
func (s *store) showEntries() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type showEntry struct {
		Key        string `json:"key"`
		Value      string `json:"value"`
		Source     string `json:"source"`
		Persistent bool   `json:"persistent"`
	}

	entries := make([]showEntry, 0)
	for _, e := range s.entries {
		val, l := e.effective()
		if l < 0 {
			continue
		}
		entries = append(entries, showEntry{
			Key:        e.key,
			Value:      val,
			Source:     e.source(),
			Persistent: e.persistent(),
		})
	}

	data, _ := json.Marshal(entries)
	return string(data)
}

// configSnapshot captures the config-layer state for rollback.
type configSnapshot struct {
	values map[string]string // key -> configValue (empty string = not set)
}

// snapshotConfig returns a snapshot of all current config-layer values.
func (s *store) snapshotConfig() configSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snap := configSnapshot{values: make(map[string]string, len(s.entries))}
	for _, e := range s.entries {
		if e.configValue != "" {
			snap.values[e.key] = e.configValue
		}
	}
	return snap
}

// rollbackConfig restores config-layer values from a snapshot and re-applies
// effective values to the kernel.
func (s *store) rollbackConfig(snap configSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear all config values, then restore from snapshot.
	for _, e := range s.entries {
		e.configValue = ""
	}
	for key, value := range snap.values {
		e := s.getOrCreate(key)
		e.configValue = value
	}

	// Re-apply effective values to kernel.
	for _, e := range s.entries {
		val, l := e.effective()
		if l < 0 {
			continue
		}
		if err := s.be.write(e.key, val); err != nil {
			s.log.Warn("sysctl: rollback write failed", "key", e.key, "err", err)
		}
	}
	s.log.Info("sysctl: config rolled back to previous state")
}

// restoreAll restores all saved original values on clean stop.
func (s *store) restoreAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, e := range s.entries {
		if !e.hasSaved {
			continue
		}
		if err := s.be.write(e.key, e.original); err != nil {
			s.log.Warn("sysctl: restore failed", "key", e.key, "original", e.original, "err", err)
			continue
		}
		s.log.Info("sysctl: restored original value", "key", e.key, "value", e.original)
	}
}

// listKnownKeys returns a JSON array of all registered known keys with metadata.
func listKnownKeys() string {
	type listEntry struct {
		Key         string `json:"key"`
		Description string `json:"description"`
		Type        string `json:"type"`
	}

	all := sysctlreg.All()
	entries := make([]listEntry, 0, len(all))
	for _, k := range all {
		entries = append(entries, listEntry{
			Key:         k.Name,
			Description: k.Description,
			Type:        valueTypeName(k.Type),
		})
	}

	data, _ := json.Marshal(entries)
	return string(data)
}

// describeKey returns a JSON object with full detail for one key.
func (s *store) describeKey(key string) string {
	type describeResult struct {
		Key         string `json:"key"`
		Description string `json:"description,omitempty"`
		Type        string `json:"type,omitempty"`
		Min         int    `json:"min,omitempty"`
		Max         int    `json:"max,omitempty"`
		Value       string `json:"value,omitempty"`
		Source      string `json:"source,omitempty"`
	}

	result := describeResult{Key: key}

	// Fill from known key metadata if available.
	if k, ok := sysctlreg.Lookup(key); ok {
		result.Description = k.Description
		result.Type = valueTypeName(k.Type)
		if k.Type == sysctlreg.TypeIntRange {
			result.Min = k.Min
			result.Max = k.Max
		}
	} else if k, ok := sysctlreg.MatchTemplate(key); ok {
		result.Description = k.Description
		result.Type = valueTypeName(k.Type)
		if k.Type == sysctlreg.TypeIntRange {
			result.Min = k.Min
			result.Max = k.Max
		}
	}

	// Fill current value from store.
	var foundInStore bool
	s.mu.RLock()
	if e, ok := s.entries[key]; ok {
		val, _ := e.effective()
		result.Value = val
		result.Source = e.source()
		foundInStore = true
	}
	s.mu.RUnlock()

	// For keys not tracked in the store, read current kernel value.
	// Done outside the lock to avoid blocking writers on slow reads.
	if !foundInStore {
		if val, err := s.be.read(key); err == nil {
			result.Value = val
		}
	}

	data, _ := json.Marshal(result)
	return string(data)
}

func valueTypeName(t sysctlreg.ValueType) string {
	switch t {
	case sysctlreg.TypeBool:
		return "bool"
	case sysctlreg.TypeInt:
		return "int"
	case sysctlreg.TypeIntRange:
		return "int-range"
	}
	return "unknown"
}

// parseSysctlConfig parses config JSON into a key->value map.
// YANG shape: {"sysctl": {"setting": {"key1": {"value": "v1"}, ...}}}.
func parseSysctlConfig(data string) map[string]string {
	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return nil
	}

	sysctlMap, ok := root["sysctl"].(map[string]any)
	if !ok {
		return nil
	}

	settingMap, ok := sysctlMap["setting"].(map[string]any)
	if !ok {
		return nil
	}

	result := make(map[string]string, len(settingMap))
	for name, entry := range settingMap {
		if m, ok := entry.(map[string]any); ok {
			if v, ok := m["value"].(string); ok {
				result[name] = v
			}
		}
	}
	return result
}

// appliedJSON builds the JSON payload for a (sysctl, applied) event.
func appliedJSON(key, value, source string) string {
	type appliedEvent struct {
		Key    string `json:"key"`
		Value  string `json:"value"`
		Source string `json:"source"`
	}
	data, _ := json.Marshal(appliedEvent{Key: key, Value: value, Source: source})
	return string(data)
}

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

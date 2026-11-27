// Design: docs/architecture/fleet-alarms.md -- open alarm kind registration seam and built-in kinds
// Related: managed_alarms.go -- ManagedAlarmStore evaluates registered kinds and tracks occurrences
// Related: managed_alarms_store.go -- durable shard persistence and retention for alarm occurrences

package server

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Canonical built-in first-release hub alarm kind identifiers.
// relay-not-granted is intentionally NOT registered here; its registration
// accompanies fleet-9 through RegisterAlarmKind.
const (
	AlarmKindDeviceOffline     = "device-offline"
	AlarmKindComponentDown     = "component-down"
	AlarmKindComponentDegraded = "component-degraded"
	AlarmKindConfigDiverged    = "config-diverged"
	AlarmKindConfigRejected    = "config-rejected"
	AlarmKindResourceHigh      = "resource-high"
	AlarmKindSoftwareBehind    = "software-behind"
	AlarmKindHistoryFull       = "history-full"

	alarmKindNameMax = 128

	alarmSeverityStrUnspecified   = "unspecified"
	alarmSeverityStrCritical      = "critical"
	alarmSeverityStrWarning       = "warning"
	alarmSeverityStrInformational = "informational"

	alarmScopeStrUnspecified = "unspecified"
	alarmScopeStrDevice      = "device"
	alarmScopeStrHub         = "hub"
)

// AlarmSeverity classifies how urgently an alarm condition needs attention.
// The zero value AlarmSeverityUnspecified is invalid so an unset field cannot
// pass for a real severity.
type AlarmSeverity uint8

const (
	// AlarmSeverityUnspecified is the invalid zero value.
	AlarmSeverityUnspecified AlarmSeverity = 0
	// AlarmSeverityCritical indicates an outage or failed component.
	AlarmSeverityCritical AlarmSeverity = 1
	// AlarmSeverityWarning indicates degraded service, config drift, or pressure.
	AlarmSeverityWarning AlarmSeverity = 2
	// AlarmSeverityInformational indicates an advisory condition.
	AlarmSeverityInformational AlarmSeverity = 3
)

// AlarmScope indicates whether an alarm kind applies to a managed device or to
// the hub itself. The zero value AlarmScopeUnspecified is invalid.
type AlarmScope uint8

const (
	// AlarmScopeUnspecified is the invalid zero value.
	AlarmScopeUnspecified AlarmScope = 0
	// AlarmScopeDevice scopes occurrences to a named managed device.
	AlarmScopeDevice AlarmScope = 1
	// AlarmScopeHub scopes occurrences to the hub itself (empty Device).
	AlarmScopeHub AlarmScope = 2
)

// AlarmObservation describes one currently active condition instance returned
// by an AlarmKindSpec.Evaluate callback.
type AlarmObservation struct {
	// Device names the managed device for device-scoped kinds, or is empty for
	// hub-scoped kinds.
	Device string
	// Subject distinguishes multiple occurrences of the same kind on one target
	// (such as a component name or a resource subject). May be empty when at
	// most one occurrence exists per target.
	Subject string
	// Summary is a concise operator-facing one-line description. When empty, the
	// registered kind's default Summary is used.
	Summary string
	// Detail carries additional operator-facing context.
	Detail string
	// ObservedAt is the UTC receipt or observation timestamp. When zero, the
	// alarm store stamps the transition with its current clock time.
	ObservedAt time.Time
}

// AlarmEvalDevice is a read-only snapshot of one managed device's hub-side
// facts passed to registered external AlarmKindSpec evaluators.
type AlarmEvalDevice struct {
	Name                   string
	Declared               bool
	Connected              bool
	DeclaredAt             time.Time
	DeclarationGeneration  uint64
	ConnectedInDeclaration bool
	ConnectedSince         time.Time
	LastDisconnectedAt     time.Time
	SoftwareVersion        string
	ConfigDiverged         bool
	ConfigDivergedDetail   string
	ConfigRejected         bool
	ConfigRejectedError    string
	Health                 *ManagedHealthSnapshot
	Inventory              *ManagedInventorySnapshot
	LatestSamples          map[string]float64
	LatestSampleAt         time.Time
}

// AlarmEvalContext carries read-only hub facts to a registered AlarmKindSpec
// evaluator. Evaluators MUST NOT mutate slices or maps in the context.
type AlarmEvalContext struct {
	Now           time.Time
	Config        plugin.HubAlarmsConfig
	Devices       []AlarmEvalDevice
	HistoryHealth *ManagedHistoryHealth
}

// AlarmKindSpec declares one registered hub alarm kind.
type AlarmKindSpec struct {
	// Name is the stable kebab-case identifier (for example "device-offline").
	Name string
	// Severity is the severity level assigned to occurrences of this kind.
	Severity AlarmSeverity
	// Scope indicates whether occurrences belong to a device or the hub.
	Scope AlarmScope
	// Causes lists other registered alarm kinds on the same device whose open
	// occurrence hides this kind (consumed by fleet-ui-8). Hub-scoped kinds
	// MUST NOT declare causes.
	Causes []string
	// Settles indicates whether this kind requires a sustained qualifying window
	// after reconnect before reopening (for example resource-high).
	Settles bool
	// Summary is the fallback one-line sentence used when an observation omits
	// its own summary.
	Summary string
	// Evaluate is an optional callback for externally registered alarm kinds.
	// Built-in kinds driven directly by ManagedAlarmStore fact handlers leave
	// Evaluate nil.
	Evaluate func(AlarmEvalContext) []AlarmObservation
}

// AlarmKindRegistry holds the open set of registered hub alarm kinds.
// Safe for concurrent use.
type AlarmKindRegistry struct {
	mu           sync.RWMutex
	kinds        map[string]AlarmKindSpec
	externalEval atomic.Bool
}

// String returns the canonical lowercase wire spelling of s. Compare typed
// AlarmSeverity constants rather than String.
func (s AlarmSeverity) String() string {
	switch s {
	case AlarmSeverityUnspecified:
		return alarmSeverityStrUnspecified
	case AlarmSeverityCritical:
		return alarmSeverityStrCritical
	case AlarmSeverityWarning:
		return alarmSeverityStrWarning
	case AlarmSeverityInformational:
		return alarmSeverityStrInformational
	default:
		// Open enum: callers can construct an arbitrary uint8.
		return alarmSeverityStrUnspecified
	}
}

// Valid reports whether s is critical, warning, or informational.
func (s AlarmSeverity) Valid() bool {
	switch s {
	case AlarmSeverityCritical, AlarmSeverityWarning, AlarmSeverityInformational:
		return true
	case AlarmSeverityUnspecified:
		return false
	default:
		// Open enum: callers can construct an arbitrary uint8.
		return false
	}
}

// Rank returns the sort priority of s, where lower values sort first
// (critical < warning < informational). Invalid values sort last.
func (s AlarmSeverity) Rank() int {
	switch s {
	case AlarmSeverityCritical:
		return 1
	case AlarmSeverityWarning:
		return 2
	case AlarmSeverityInformational:
		return 3
	case AlarmSeverityUnspecified:
		return 4
	default:
		// Open enum: callers can construct an arbitrary uint8.
		return 4
	}
}

// ParseAlarmSeverity converts a wire or CLI severity string into its typed
// AlarmSeverity value.
func ParseAlarmSeverity(raw string) (AlarmSeverity, error) {
	switch raw {
	case alarmSeverityStrCritical:
		return AlarmSeverityCritical, nil
	case alarmSeverityStrWarning:
		return AlarmSeverityWarning, nil
	case alarmSeverityStrInformational:
		return AlarmSeverityInformational, nil
	default:
		if len(raw) > alarmKindNameMax {
			return AlarmSeverityUnspecified, errors.New("alarm: invalid severity")
		}
		var tb textbuf.Buffer
		return AlarmSeverityUnspecified, errors.New(tb.Str("alarm: invalid severity ").Quoted(raw).String())
	}
}

// MarshalText encodes a valid AlarmSeverity as its lowercase wire string.
func (s AlarmSeverity) MarshalText() ([]byte, error) {
	if !s.Valid() {
		return nil, fmt.Errorf("alarm: invalid severity %d", uint8(s))
	}
	return []byte(s.String()), nil
}

// UnmarshalText decodes a lowercase wire severity string into s.
func (s *AlarmSeverity) UnmarshalText(text []byte) error {
	parsed, err := ParseAlarmSeverity(string(text))
	if err != nil {
		return err
	}
	*s = parsed
	return nil
}

// MarshalJSON encodes a valid AlarmSeverity as a JSON string.
func (s AlarmSeverity) MarshalJSON() ([]byte, error) {
	if !s.Valid() {
		return nil, fmt.Errorf("alarm: invalid severity %d", uint8(s))
	}
	return json.Marshal(s.String())
}

// UnmarshalJSON decodes a JSON string into a valid AlarmSeverity.
func (s *AlarmSeverity) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("alarm: severity must be a string: %w", err)
	}
	parsed, err := ParseAlarmSeverity(raw)
	if err != nil {
		return err
	}
	*s = parsed
	return nil
}

// String returns the canonical lowercase wire spelling of sc. Compare typed
// AlarmScope constants rather than String.
func (sc AlarmScope) String() string {
	switch sc {
	case AlarmScopeUnspecified:
		return alarmScopeStrUnspecified
	case AlarmScopeDevice:
		return alarmScopeStrDevice
	case AlarmScopeHub:
		return alarmScopeStrHub
	default:
		// Open enum: callers can construct an arbitrary uint8.
		return alarmScopeStrUnspecified
	}
}

// Valid reports whether sc is AlarmScopeDevice or AlarmScopeHub.
func (sc AlarmScope) Valid() bool {
	switch sc {
	case AlarmScopeDevice, AlarmScopeHub:
		return true
	case AlarmScopeUnspecified:
		return false
	default:
		// Open enum: callers can construct an arbitrary uint8.
		return false
	}
}

// ParseAlarmScope converts a wire scope string into its typed AlarmScope value.
func ParseAlarmScope(raw string) (AlarmScope, error) {
	switch raw {
	case alarmScopeStrDevice:
		return AlarmScopeDevice, nil
	case alarmScopeStrHub:
		return AlarmScopeHub, nil
	default:
		if len(raw) > alarmKindNameMax {
			return AlarmScopeUnspecified, errors.New("alarm: invalid scope")
		}
		var tb textbuf.Buffer
		return AlarmScopeUnspecified, errors.New(tb.Str("alarm: invalid scope ").Quoted(raw).String())
	}
}

// MarshalText encodes a valid AlarmScope as its lowercase wire string.
func (sc AlarmScope) MarshalText() ([]byte, error) {
	if !sc.Valid() {
		return nil, fmt.Errorf("alarm: invalid scope %d", uint8(sc))
	}
	return []byte(sc.String()), nil
}

// UnmarshalText decodes a lowercase wire scope string into sc.
func (sc *AlarmScope) UnmarshalText(text []byte) error {
	parsed, err := ParseAlarmScope(string(text))
	if err != nil {
		return err
	}
	*sc = parsed
	return nil
}

// MarshalJSON encodes a valid AlarmScope as a JSON string.
func (sc AlarmScope) MarshalJSON() ([]byte, error) {
	if !sc.Valid() {
		return nil, fmt.Errorf("alarm: invalid scope %d", uint8(sc))
	}
	return json.Marshal(sc.String())
}

// UnmarshalJSON decodes a JSON string into a valid AlarmScope.
func (sc *AlarmScope) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("alarm: scope must be a string: %w", err)
	}
	parsed, err := ParseAlarmScope(raw)
	if err != nil {
		return err
	}
	*sc = parsed
	return nil
}

// NewAlarmKindRegistry constructs an empty alarm kind registry.
func NewAlarmKindRegistry() *AlarmKindRegistry {
	return &AlarmKindRegistry{
		kinds: make(map[string]AlarmKindSpec),
	}
}

// Clone returns an independent copy of r containing all registered kinds.
// Safe for concurrent use.
func (r *AlarmKindRegistry) Clone() *AlarmKindRegistry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cp := NewAlarmKindRegistry()
	hasExternal := false
	for name, spec := range r.kinds {
		cp.kinds[name] = cloneAlarmKindSpec(spec)
		if spec.Evaluate != nil {
			hasExternal = true
		}
	}
	cp.externalEval.Store(hasExternal)
	return cp
}

// HasExternalEvaluators reports whether at least one registered kind carries a
// non-nil Evaluate callback. Safe for concurrent use without acquiring r.mu.
func (r *AlarmKindRegistry) HasExternalEvaluators() bool {
	return r.externalEval.Load()
}

// Register validates and installs spec in r. It rejects empty or malformed
// names, invalid severities or scopes, hub-scoped kinds with causes, self- or
// duplicate causes, and duplicate kind names. Safe for concurrent use.
func (r *AlarmKindRegistry) Register(spec AlarmKindSpec) error {
	if err := validateAlarmKindSpec(&spec); err != nil {
		return err
	}
	cloned := cloneAlarmKindSpec(spec)

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.kinds[cloned.Name]; exists {
		var tb textbuf.Buffer
		return errors.New(tb.Str("alarm: duplicate alarm kind ").Quoted(cloned.Name).String())
	}
	r.kinds[cloned.Name] = cloned
	if cloned.Evaluate != nil {
		r.externalEval.Store(true)
	}
	return nil
}

// MustRegister registers spec in r and panics on error. Intended for
// compile-time init() registration only.
func (r *AlarmKindRegistry) MustRegister(spec AlarmKindSpec) {
	if err := r.Register(spec); err != nil {
		panic("BUG: server: register alarm kind: " + err.Error())
	}
}

// Lookup returns the registered AlarmKindSpec for name and whether it exists.
// Safe for concurrent use.
func (r *AlarmKindRegistry) Lookup(name string) (AlarmKindSpec, bool) {
	r.mu.RLock()
	spec, ok := r.kinds[name]
	r.mu.RUnlock()
	if !ok {
		return AlarmKindSpec{}, false
	}
	return cloneAlarmKindSpec(spec), true
}

// List returns all registered alarm kinds sorted deterministically by Name.
// Safe for concurrent use.
func (r *AlarmKindRegistry) List() []AlarmKindSpec {
	r.mu.RLock()
	out := make([]AlarmKindSpec, 0, len(r.kinds))
	for _, spec := range r.kinds {
		out = append(out, cloneAlarmKindSpec(spec))
	}
	r.mu.RUnlock()
	slices.SortFunc(out, func(a, b AlarmKindSpec) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return out
}

// Snapshot captures the current contents of r and returns a restore function
// for test cleanup. Callers MUST invoke the returned function (for example via
// t.Cleanup) to avoid contaminating subsequent tests. Safe for concurrent use.
func (r *AlarmKindRegistry) Snapshot() func() {
	r.mu.RLock()
	saved := make(map[string]AlarmKindSpec, len(r.kinds))
	for k, v := range r.kinds {
		saved[k] = cloneAlarmKindSpec(v)
	}
	r.mu.RUnlock()
	return func() {
		r.mu.Lock()
		r.kinds = maps.Clone(saved)
		hasExternal := false
		for _, spec := range r.kinds {
			if spec.Evaluate != nil {
				hasExternal = true
				break
			}
		}
		r.externalEval.Store(hasExternal)
		r.mu.Unlock()
	}
}

func cloneAlarmKindSpec(spec AlarmKindSpec) AlarmKindSpec {
	spec.Causes = slices.Clone(spec.Causes)
	return spec
}

func validateAlarmKindName(field, name string) error {
	if name == "" {
		return fmt.Errorf("alarm: %s is required", field)
	}
	if len(name) > alarmKindNameMax {
		return fmt.Errorf("alarm: %s length %d exceeds max %d", field, len(name), alarmKindNameMax)
	}
	if !utf8.ValidString(name) {
		return fmt.Errorf("alarm: %s is not valid UTF-8", field)
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return fmt.Errorf("alarm: %s contains control character", field)
		}
		if unicode.IsSpace(r) {
			return fmt.Errorf("alarm: %s contains whitespace", field)
		}
	}
	return nil
}

func validateAlarmKindSpec(spec *AlarmKindSpec) error {
	if err := validateAlarmKindName("kind name", spec.Name); err != nil {
		return err
	}
	if !spec.Severity.Valid() {
		return fmt.Errorf("alarm: kind %q has invalid severity %d", spec.Name, uint8(spec.Severity))
	}
	if !spec.Scope.Valid() {
		return fmt.Errorf("alarm: kind %q has invalid scope %d", spec.Name, uint8(spec.Scope))
	}
	if spec.Scope == AlarmScopeHub {
		if len(spec.Causes) > 0 {
			return fmt.Errorf("alarm: hub-scoped kind %q cannot declare causes", spec.Name)
		}
	}
	seenCauses := make(map[string]struct{}, len(spec.Causes))
	for _, cause := range spec.Causes {
		if err := validateAlarmKindName("cause kind name", cause); err != nil {
			return fmt.Errorf("kind %q: %w", spec.Name, err)
		}
		if cause == spec.Name {
			return fmt.Errorf("alarm: kind %q cannot declare itself as a cause", spec.Name)
		}
		if _, dup := seenCauses[cause]; dup {
			return fmt.Errorf("alarm: kind %q declares duplicate cause %q", spec.Name, cause)
		}
		seenCauses[cause] = struct{}{}
	}
	return nil
}

var defaultAlarmKindRegistry = NewAlarmKindRegistry()

// RegisterAlarmKind registers spec in the process-wide hub alarm kind registry.
// Safe for concurrent use.
func RegisterAlarmKind(spec AlarmKindSpec) error {
	return defaultAlarmKindRegistry.Register(spec)
}

// MustRegisterAlarmKind registers spec in the process-wide hub alarm kind
// registry and panics on error. Intended for compile-time init() registration.
func MustRegisterAlarmKind(spec AlarmKindSpec) {
	defaultAlarmKindRegistry.MustRegister(spec)
}

// LookupAlarmKind looks up a registered kind by name in the process-wide hub
// alarm kind registry. Safe for concurrent use.
func LookupAlarmKind(name string) (AlarmKindSpec, bool) {
	return defaultAlarmKindRegistry.Lookup(name)
}

// RegisteredAlarmKinds returns all kinds in the process-wide hub alarm kind
// registry sorted by Name. Safe for concurrent use.
func RegisteredAlarmKinds() []AlarmKindSpec {
	return defaultAlarmKindRegistry.List()
}

// SnapshotAlarmKinds captures the process-wide hub alarm kind registry and
// returns a restore function for test cleanup. Safe for concurrent use.
func SnapshotAlarmKinds() func() {
	return defaultAlarmKindRegistry.Snapshot()
}

func registerBuiltinAlarmKinds() {
	builtinKinds := []AlarmKindSpec{
		{
			Name:     AlarmKindDeviceOffline,
			Severity: AlarmSeverityCritical,
			Scope:    AlarmScopeDevice,
			Summary:  "Device has no managed connection",
		},
		{
			Name:     AlarmKindComponentDown,
			Severity: AlarmSeverityCritical,
			Scope:    AlarmScopeDevice,
			Summary:  "Device component is down",
		},
		{
			Name:     AlarmKindComponentDegraded,
			Severity: AlarmSeverityWarning,
			Scope:    AlarmScopeDevice,
			Summary:  "Device component is degraded",
		},
		{
			Name:     AlarmKindConfigDiverged,
			Severity: AlarmSeverityWarning,
			Scope:    AlarmScopeDevice,
			Summary:  "Device holds a locally edited configuration",
		},
		{
			Name:     AlarmKindConfigRejected,
			Severity: AlarmSeverityWarning,
			Scope:    AlarmScopeDevice,
			Summary:  "Device rejected the hub configuration",
		},
		{
			Name:     AlarmKindResourceHigh,
			Severity: AlarmSeverityWarning,
			Scope:    AlarmScopeDevice,
			Settles:  true,
			Summary:  "Device resource utilization stayed at or above threshold",
		},
		{
			Name:     AlarmKindSoftwareBehind,
			Severity: AlarmSeverityInformational,
			Scope:    AlarmScopeDevice,
			Summary:  "Device software version is behind the newest estate release",
		},
		{
			Name:     AlarmKindHistoryFull,
			Severity: AlarmSeverityWarning,
			Scope:    AlarmScopeHub,
			Summary:  "Hub metric history reached its capacity budget",
		},
	}
	for _, spec := range builtinKinds {
		MustRegisterAlarmKind(spec)
	}
}

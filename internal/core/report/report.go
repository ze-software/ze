// Design: docs/architecture/core-design.md, cross-cutting operational report bus
//
// Package report is the single place where Ze subsystems push operator-visible
// warnings and errors. It is consumed by the show verb (ze-show:warnings,
// ze-show:errors) and by the login banner.
//
// Warnings are state-based: a warning represents a current condition that may
// resolve. Producers raise it when the condition starts and clear it when the
// condition ends. The bus deduplicates on (Source, Code, Subject); re-raising
// updates the message and timestamp without creating a duplicate Issue.
//
// Errors are event-based: an error represents something that already happened.
// Producers raise it once at the moment of occurrence. There is no clear API
// for errors. The ring buffer evicts the oldest event when full.
//
// Producers MUST pick the right severity. The bus does not auto-promote.
// "Did anything actually fail or behave unexpectedly?" Yes -> error. No, but
// it might soon -> warning.
//
// Concurrency: all package-level functions are safe for concurrent use.
// The active store is held in an atomic pointer, so reset() and resetWithCaps()
// are race-safe with concurrent Raise / Clear / snapshot operations.
//
// Capacities are configurable via env vars ze.report.warnings.max and
// ze.report.errors.max, registered at package init. Both are clamped to a
// safe upper bound (maxWarningCap, maxErrorCap) to prevent operator typos
// from causing memory exhaustion.

package report

import (
	"fmt"
	"log/slog"
	"maps"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// Severity classifies a report Issue.
type Severity uint8

// Severity values.
const (
	SeverityWarning Severity = iota
	SeverityError
)

// String returns the lowercase name of the severity.
func (s Severity) String() string {
	switch s {
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	}
	return "unknown"
}

// MarshalJSON encodes Severity as the lowercase label string ("warning" or
// "error") so operator-facing JSON shows readable values rather than the
// underlying uint8 enum integer.
func (s Severity) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}

// UnmarshalJSON decodes a Severity from its lowercase label form. Required
// for round-trip safety even though the bus has no current consumer that
// deserializes Issues; future ze-events streaming or external dashboards
// would need this. Unknown labels are rejected with an error.
func (s *Severity) UnmarshalJSON(data []byte) error {
	str := string(data)
	if str == `"warning"` {
		*s = SeverityWarning
		return nil
	}
	if str == `"error"` {
		*s = SeverityError
		return nil
	}
	return fmt.Errorf("report: unknown severity %s", data)
}

// Issue is a single operator-visible report on the bus.
//
// Source identifies the producing subsystem ("bgp", "config", "iface", ...).
// Code is a stable, kebab-case identifier of the condition or event.
// Subject is what the Issue is about (peer address, transaction id, file path).
// Message is a human-readable one-liner suitable for an operator UI.
// Detail carries optional structured context (family, code/subcode, reason, ...).
// Raised is when the Issue first appeared on the bus.
// Updated is the most recent raise time. For warnings this advances on every
// re-raise. For errors Updated equals Raised since errors are events.
type Issue struct {
	Source   string         `json:"source"`
	Code     string         `json:"code"`
	Severity Severity       `json:"severity"`
	Subject  string         `json:"subject"`
	Message  string         `json:"message"`
	Detail   map[string]any `json:"detail,omitempty"`
	Raised   time.Time      `json:"raised"`
	Updated  time.Time      `json:"updated"`
}

// Default capacities. Production values may be overridden via env vars,
// but are always clamped to the max constants below.
const (
	defaultWarningCap = 1024
	defaultErrorCap   = 256
)

// Maximum capacities. Operator-supplied values are clamped to these limits
// to prevent memory exhaustion from typos in env var values. Eviction within
// the bus is O(n) over the active warning set, so a sane upper bound also
// keeps the eviction path fast (microseconds at the cap).
const (
	maxWarningCap = 10000
	maxErrorCap   = 10000
)

// Field length limits. Producers passing strings beyond these limits have
// their raise call rejected at the boundary, the same way empty fields are
// rejected. This prevents a buggy producer (or malicious caller in a future
// API surface) from filling the bus with multi-megabyte entries. Detail map
// size is also bounded; values are not length-checked but the entry count
// cap prevents pathological detail payloads.
const (
	maxSourceLen  = 64
	maxCodeLen    = 64
	maxSubjectLen = 256
	maxMessageLen = 1024
	maxDetailKeys = 16
)

// Env var keys.
const (
	envWarningCap = "ze.report.warnings.max"
	envErrorCap   = "ze.report.errors.max"
)

var (
	_ = env.MustRegister(env.EnvEntry{
		Key:         envWarningCap,
		Type:        "int",
		Default:     strconv.Itoa(defaultWarningCap),
		Description: "Maximum number of active warnings retained by the report bus (clamped to 10000)",
	})
	_ = env.MustRegister(env.EnvEntry{
		Key:         envErrorCap,
		Type:        "int",
		Default:     strconv.Itoa(defaultErrorCap),
		Description: "Maximum number of error events retained by the report bus ring buffer (clamped to 10000)",
	})
)

// warningKey is the dedup key for active warnings.
type warningKey struct {
	source  string
	code    string
	subject string
}

// store is the package-private bus state.
//
// warnings holds *Issue (not Issue) so range loops do not copy the value on
// each iteration; the bus owns the entries and snapshots dereference to copy.
// errors uses *Issue for the same reason.
type store struct {
	mu         sync.RWMutex
	warnings   map[warningKey]*Issue
	errors     []*Issue // ring buffer of pointers
	errorHead  int      // index of next write position
	errorCount int      // number of valid entries (capped at len(errors))
	warningCap int
	errorCap   int
	logger     *slog.Logger
}

// pkg is the default process-wide store, held atomically so reset() can swap
// the entire store under concurrent readers without a data race on the pointer.
var pkg atomic.Pointer[store]

func init() {
	pkg.Store(newStoreFromEnv())
}

func newStoreFromEnv() *store {
	wcap := env.GetInt(envWarningCap, defaultWarningCap)
	ecap := env.GetInt(envErrorCap, defaultErrorCap)
	return newStore(wcap, ecap)
}

func newStore(warningCap, errorCap int) *store {
	logger := slogutil.Logger("report")

	if warningCap <= 0 {
		warningCap = defaultWarningCap
	}
	if warningCap > maxWarningCap {
		logger.Warn("warning cap clamped to maximum",
			"requested", warningCap, "max", maxWarningCap)
		warningCap = maxWarningCap
	}

	if errorCap <= 0 {
		errorCap = defaultErrorCap
	}
	if errorCap > maxErrorCap {
		logger.Warn("error cap clamped to maximum",
			"requested", errorCap, "max", maxErrorCap)
		errorCap = maxErrorCap
	}

	return &store{
		warnings:   make(map[warningKey]*Issue, warningCap),
		errors:     make([]*Issue, errorCap),
		warningCap: warningCap,
		errorCap:   errorCap,
		logger:     logger,
	}
}

// reset clears all bus state. Used by tests for isolation.
// Safe to call concurrently with Raise / Clear / snapshot operations
// because the package store is held in an atomic.Pointer.
func reset() {
	pkg.Store(newStore(defaultWarningCap, defaultErrorCap))
}

// resetWithCaps replaces the package store with one of the given caps. Tests only.
// Safe to call concurrently with Raise / Clear / snapshot operations.
func resetWithCaps(warningCap, errorCap int) {
	pkg.Store(newStore(warningCap, errorCap))
}

// validFields returns true if all fields are non-empty and within length bounds.
// Returns false (with no logging here, the caller logs) for any violation.
func validFields(source, code, subject, message string, detail map[string]any) bool {
	if source == "" || code == "" || subject == "" {
		return false
	}
	if len(source) > maxSourceLen {
		return false
	}
	if len(code) > maxCodeLen {
		return false
	}
	if len(subject) > maxSubjectLen {
		return false
	}
	if len(message) > maxMessageLen {
		return false
	}
	if len(detail) > maxDetailKeys {
		return false
	}
	return true
}

// RaiseWarning adds or refreshes an active warning.
//
// If an Issue with the same (Source, Code, Subject) already exists, its
// Message, Detail, and Updated fields are refreshed. Raised is preserved.
// Otherwise a new Issue is created.
//
// The call is rejected (silently logged at debug) if any field is empty or
// exceeds its length limit (Source/Code 64 bytes, Subject 256, Message 1024,
// Detail at most 16 keys). This protects the bus from buggy or malicious
// producers pushing oversized entries.
//
// Detail is optional. When passed, the first map argument is stored on the
// Issue. Subsequent map arguments are ignored. The Detail map is shallow-cloned
// at store and again at snapshot time; values inside the map are shared by
// reference, so callers MUST NOT mutate values they have passed in.
func RaiseWarning(source, code, subject, message string, detail ...map[string]any) {
	pkg.Load().raiseWarning(source, code, subject, message, firstDetail(detail))
}

// ClearWarning removes the active warning identified by (Source, Code, Subject).
// No-op if no Issue matches. Empty fields are silently ignored.
func ClearWarning(source, code, subject string) {
	pkg.Load().clearWarning(source, code, subject)
}

// ClearSource removes all active warnings whose Source matches.
// Used by subsystems on shutdown or when invalidating their entire warning set.
func ClearSource(source string) {
	pkg.Load().clearSource(source)
}

// RaiseError appends an error event to the ring buffer.
//
// Errors are events; there is no dedup. Repeated raises with identical fields
// produce multiple ring entries. The oldest Issue is evicted when the ring is
// full.
//
// The call is rejected (silently logged at debug) if any field is empty or
// exceeds its length limit, same rules as RaiseWarning.
//
// Detail is optional and follows the same shallow-clone contract as RaiseWarning.
func RaiseError(source, code, subject, message string, detail ...map[string]any) {
	pkg.Load().raiseError(source, code, subject, message, firstDetail(detail))
}

// Warnings returns a snapshot of all active warnings, ordered most-recently-
// updated first. The returned slice and Issues (including Detail maps) are
// copies; mutating them does not affect bus state. Detail map values are
// shallow-cloned, see RaiseWarning godoc for the constraint.
//
// Returns an empty (non-nil) slice when no warnings are active.
func Warnings() []Issue {
	return pkg.Load().snapshotWarnings()
}

// Errors returns up to limit most-recent error events, newest first.
// limit == 0 (or negative) returns all retained events.
// limit > retained returns all retained.
//
// Returns an empty (non-nil) slice when no errors have been raised.
func Errors(limit int) []Issue {
	return pkg.Load().snapshotErrors(limit)
}

// firstDetail returns the first map from a variadic detail argument, or nil.
func firstDetail(detail []map[string]any) map[string]any {
	if len(detail) == 0 {
		return nil
	}
	return detail[0]
}

// copyDetail returns a shallow clone of a detail map.
//
// Values inside the map are shared by reference, not deep-copied. Callers
// passing mutable values (slices, maps, struct pointers) MUST NOT mutate
// them after the call, otherwise the bus state and any returned snapshot
// will observe the mutation.
//
// Empty maps (zero entries) are normalized to nil so the Issue.Detail
// `json:"detail,omitempty"` tag consistently omits the field rather than
// emitting `"detail": {}`. Numeric values inside Detail (uint32, int, etc)
// JSON-encode as numbers but Go's json package decodes them as float64 on
// the round-trip side; tests asserting on detail values must account for this.
//
// Returns nil for nil or empty input.
func copyDetail(d map[string]any) map[string]any {
	if len(d) == 0 {
		return nil
	}
	return maps.Clone(d)
}

func (s *store) raiseWarning(source, code, subject, message string, detail map[string]any) {
	if !validFields(source, code, subject, message, detail) {
		s.logger.Debug("rejected RaiseWarning: invalid fields",
			"source", source, "code", code, "subject_len", len(subject),
			"message_len", len(message), "detail_keys", len(detail))
		return
	}
	key := warningKey{source: source, code: code, subject: subject}
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.warnings[key]; ok {
		existing.Updated = now
		existing.Message = message
		existing.Detail = copyDetail(detail)
		return
	}

	s.warnings[key] = &Issue{
		Source:   source,
		Code:     code,
		Severity: SeverityWarning,
		Subject:  subject,
		Message:  message,
		Detail:   copyDetail(detail),
		Raised:   now,
		Updated:  now,
	}

	if len(s.warnings) > s.warningCap {
		s.evictOldestWarning()
	}
}

func (s *store) clearWarning(source, code, subject string) {
	if source == "" || code == "" || subject == "" {
		return
	}
	key := warningKey{source: source, code: code, subject: subject}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.warnings, key)
}

func (s *store) clearSource(source string) {
	if source == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.warnings {
		if k.source == source {
			delete(s.warnings, k)
		}
	}
}

func (s *store) raiseError(source, code, subject, message string, detail map[string]any) {
	if !validFields(source, code, subject, message, detail) {
		s.logger.Debug("rejected RaiseError: invalid fields",
			"source", source, "code", code, "subject_len", len(subject),
			"message_len", len(message), "detail_keys", len(detail))
		return
	}
	now := time.Now()
	issue := &Issue{
		Source:   source,
		Code:     code,
		Severity: SeverityError,
		Subject:  subject,
		Message:  message,
		Detail:   copyDetail(detail),
		Raised:   now,
		Updated:  now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.errors[s.errorHead] = issue
	s.errorHead = (s.errorHead + 1) % s.errorCap
	if s.errorCount < s.errorCap {
		s.errorCount++
	}
}

func (s *store) snapshotWarnings() []Issue {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Issue, 0, len(s.warnings))
	for _, v := range s.warnings {
		copied := *v
		copied.Detail = copyDetail(v.Detail)
		out = append(out, copied)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Updated.After(out[j].Updated)
	})
	return out
}

func (s *store) snapshotErrors(limit int) []Issue {
	s.mu.RLock()
	defer s.mu.RUnlock()

	n := s.errorCount
	if limit > 0 && limit < n {
		n = limit
	}
	// Negative limit is treated as "all retained" (same as limit == 0),
	// matching the godoc.
	out := make([]Issue, 0, n)
	for i := range n {
		idx := (s.errorHead - 1 - i + s.errorCap) % s.errorCap
		issue := *s.errors[idx]
		issue.Detail = copyDetail(issue.Detail)
		out = append(out, issue)
	}
	return out
}

// evictOldestWarning removes the Issue with the oldest Updated timestamp.
// Caller MUST hold s.mu (write lock). O(n) over the warning map; with
// warningCap clamped to maxWarningCap (10000) the worst-case scan is fast.
func (s *store) evictOldestWarning() {
	var oldestKey warningKey
	var oldestTime time.Time
	first := true
	for k, v := range s.warnings {
		if first || v.Updated.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.Updated
			first = false
		}
	}
	delete(s.warnings, oldestKey)
	s.logger.Warn("warning evicted (cap reached)",
		"source", oldestKey.source, "code", oldestKey.code, "subject", oldestKey.subject)
}

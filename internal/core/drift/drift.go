// Design: docs/architecture/diagnostics/production-diagnostics.md -- read-only dataplane drift registry
// Related: register.go -- doctor diagnostic code and runtime check registration

// Package drift defines the value-type report model and thread-safe
// registration seam for read-only dataplane intent-vs-actual checks.
// Daemon-resident components register local checkers with Register; plugins
// that may run either in-process or forked answer through the daemon's remote
// collector hook registered with RegisterRemoteCollector.
package drift

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"
)

// Status summarizes the outcome of a single subsystem drift check.
type Status string

const (
	// StatusClean reports that observed dataplane state matches Ze intent.
	StatusClean Status = "clean"
	// StatusDrifted reports one or more divergences between intent and dataplane.
	StatusDrifted Status = "drifted"
	// StatusUnavailable reports that the subsystem cannot be checked in this environment.
	StatusUnavailable Status = "unavailable"
	// StatusError reports that reading intent or dataplane state failed.
	StatusError Status = "error"
)

// Kind classifies a single dataplane drift finding.
type Kind string

const (
	// KindMissing reports an intended object that is absent from the dataplane.
	KindMissing Kind = "missing"
	// KindExtra reports a Ze-owned object present in the dataplane but absent from intent.
	KindExtra Kind = "extra"
	// KindMismatch reports an object present in both intent and dataplane with differing fields.
	KindMismatch Kind = "mismatch"
)

const (
	// MaxFindingsPerSubsystem bounds how many findings a single subsystem report carries.
	MaxFindingsPerSubsystem = 64

	defaultCheckTimeout = 5 * time.Second
)

// Finding describes one intent-vs-actual divergence for a single dataplane object.
type Finding struct {
	Subject  string `json:"subject"`
	Kind     Kind   `json:"kind"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

// Report is the self-contained, read-only drift result for one subsystem.
type Report struct {
	Subsystem string    `json:"subsystem"`
	Status    Status    `json:"status"`
	Checked   int       `json:"checked"`
	Drifted   int       `json:"drifted"`
	Truncated bool      `json:"truncated,omitempty"`
	Findings  []Finding `json:"findings"`
	Error     string    `json:"error,omitempty"`
}

// CheckFunc evaluates one local subsystem's dataplane drift.
// Implementations MUST be read-only, honor ctx cancellation, and be safe for concurrent use.
type CheckFunc func(ctx context.Context) Report

// RemoteCollectorFunc queries remote (plugin) drift checkers across the plugin
// RPC boundary. An empty subsystem requests all declared remote subsystems.
type RemoteCollectorFunc func(ctx context.Context, subsystem string) []Report

type localEntry struct {
	id        uint64
	subsystem string
	fn        CheckFunc
}

type remoteCollectorEntry struct {
	id uint64
	fn RemoteCollectorFunc
}

type remoteListerEntry struct {
	id uint64
	fn func() []string
}

type registryState struct {
	mu              sync.RWMutex
	nextID          uint64
	local           map[string]localEntry
	remoteCollector remoteCollectorEntry
	remoteLister    remoteListerEntry
}

var global = registryState{local: make(map[string]localEntry)}

// Register installs a local read-only drift checker for subsystem and returns
// an idempotent unregister function that removes only that registration.
// Panics on empty subsystem or nil fn. Safe for concurrent use.
func Register(subsystem string, fn CheckFunc) func() {
	if subsystem == "" {
		panic("BUG: drift.Register called with empty subsystem")
	}
	if fn == nil {
		panic("BUG: drift.Register called with nil check function")
	}

	global.mu.Lock()
	global.nextID++
	id := global.nextID
	if global.local == nil {
		global.local = make(map[string]localEntry)
	}
	global.local[subsystem] = localEntry{
		id:        id,
		subsystem: subsystem,
		fn:        fn,
	}
	global.mu.Unlock()

	return func() {
		global.mu.Lock()
		defer global.mu.Unlock()
		cur, ok := global.local[subsystem]
		if !ok {
			return
		}
		if cur.id != id {
			return
		}
		delete(global.local, subsystem)
	}
}

// RegisterRemoteCollector installs the daemon-level plugin drift collector and
// returns an idempotent unregister function that removes only that registration.
// Panics on nil fn. Safe for concurrent use.
func RegisterRemoteCollector(fn RemoteCollectorFunc) func() {
	if fn == nil {
		panic("BUG: drift.RegisterRemoteCollector called with nil collector")
	}

	global.mu.Lock()
	global.nextID++
	id := global.nextID
	global.remoteCollector = remoteCollectorEntry{id: id, fn: fn}
	global.mu.Unlock()

	return func() {
		global.mu.Lock()
		defer global.mu.Unlock()
		if global.remoteCollector.id != id {
			return
		}
		global.remoteCollector = remoteCollectorEntry{}
	}
}

// RegisterRemoteLister installs the daemon-level plugin subsystem lister and
// returns an idempotent unregister function that removes only that registration.
// Panics on nil fn. Safe for concurrent use.
func RegisterRemoteLister(fn func() []string) func() {
	if fn == nil {
		panic("BUG: drift.RegisterRemoteLister called with nil lister")
	}

	global.mu.Lock()
	global.nextID++
	id := global.nextID
	global.remoteLister = remoteListerEntry{id: id, fn: fn}
	global.mu.Unlock()

	return func() {
		global.mu.Lock()
		defer global.mu.Unlock()
		if global.remoteLister.id != id {
			return
		}
		global.remoteLister = remoteListerEntry{}
	}
}

// CheckAll runs every registered local and remote drift checker and returns
// reports sorted by subsystem name. Remote answers override local entries on
// name collision. Each local check runs with a bounded timeout outside the
// registry lock. Safe for concurrent use.
func CheckAll(ctx context.Context) []Report {
	if ctx == nil {
		ctx = context.Background()
	}

	locals, remote := snapshotAll()
	byName := make(map[string]Report, len(locals))

	for _, entry := range locals {
		report := runLocalCheck(ctx, entry.subsystem, entry.fn)
		byName[report.Subsystem] = report
	}

	if remote != nil {
		for _, r := range remote(ctx, "") {
			if r.Subsystem == "" {
				continue
			}
			byName[r.Subsystem] = normalizeReport(r.Subsystem, r)
		}
	}

	reports := make([]Report, 0, len(byName))
	for _, r := range byName {
		reports = append(reports, r)
	}
	slices.SortFunc(reports, func(a, b Report) int {
		return strings.Compare(a.Subsystem, b.Subsystem)
	})
	return reports
}

// Check evaluates one named subsystem, preferring the remote collector when
// registered and answering for subsystem, then falling back to a local checker.
// Returns (Report{}, false) when no checker handles subsystem. Safe for concurrent use.
func Check(ctx context.Context, subsystem string) (Report, bool) {
	if subsystem == "" {
		return Report{}, false
	}
	if ctx == nil {
		ctx = context.Background()
	}

	global.mu.RLock()
	remote := global.remoteCollector.fn
	local, hasLocal := global.local[subsystem]
	global.mu.RUnlock()

	if remote != nil {
		for _, r := range remote(ctx, subsystem) {
			if r.Subsystem == subsystem {
				return normalizeReport(subsystem, r), true
			}
		}
	}
	if !hasLocal {
		return Report{}, false
	}
	return runLocalCheck(ctx, subsystem, local.fn), true
}

// Subsystems returns the sorted, deduplicated union of locally registered and
// remotely declared drift subsystem names. Safe for concurrent use.
func Subsystems() []string {
	global.mu.RLock()
	seen := make(map[string]struct{}, len(global.local))
	for name := range global.local {
		seen[name] = struct{}{}
	}
	lister := global.remoteLister.fn
	global.mu.RUnlock()

	if lister != nil {
		for _, name := range lister() {
			if name == "" {
				continue
			}
			seen[name] = struct{}{}
		}
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func snapshotAll() ([]localEntry, RemoteCollectorFunc) {
	global.mu.RLock()
	defer global.mu.RUnlock()

	locals := make([]localEntry, 0, len(global.local))
	for _, entry := range global.local {
		locals = append(locals, entry)
	}
	slices.SortFunc(locals, func(a, b localEntry) int {
		return strings.Compare(a.subsystem, b.subsystem)
	})
	return locals, global.remoteCollector.fn
}

func runLocalCheck(ctx context.Context, subsystem string, fn CheckFunc) Report {
	subCtx, cancel := context.WithTimeout(ctx, defaultCheckTimeout)
	defer cancel()
	return normalizeReport(subsystem, fn(subCtx))
}

func normalizeReport(subsystem string, r Report) Report {
	if r.Subsystem == "" {
		r.Subsystem = subsystem
	}
	if r.Findings == nil {
		r.Findings = []Finding{}
	}
	if len(r.Findings) > MaxFindingsPerSubsystem {
		r.Findings = slices.Clone(r.Findings[:MaxFindingsPerSubsystem])
		r.Truncated = true
	}
	return r
}

// Design: docs/architecture/core-design.md -- daemon-backed live support bundle modules via internal/core/supportreg
// Related: server.go -- StartWithContext registers modules; Stop unregisters cleanly
// Related: event_ring.go -- global event history ring
// Related: reload_history.go -- bounded reload transaction history

package server

import (
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/supportreg"
)

const (
	supportModuleEventRing     = "event-ring"
	supportModulePlugins       = "plugins"
	supportModuleReloadHistory = "reload-history"

	supportEventRingMaxEntries = 200
	supportPluginsMaxEntries   = 256
)

// activeSupportMu serializes daemon-level supportreg registrations so a second
// Server started in the same process (e.g., tests) cleanly replaces any prior
// registration without triggering a duplicate-name panic, while Stop removes
// only its own server's registrations.
var (
	activeSupportMu     sync.Mutex
	activeSupportOwner  *Server
	activeSupportUnregs []func()
)

type supportEventRingEntry struct {
	Timestamp string `json:"timestamp"`
	Namespace string `json:"namespace"`
	EventType string `json:"event-type"`
	Source    string `json:"source,omitempty"`
	Subject   string `json:"subject,omitempty"`
	Direction string `json:"direction,omitempty"`
	OpID      string `json:"operation-id,omitempty"`
}

type supportEventRingPayload struct {
	Available  bool                    `json:"available"`
	Count      int                     `json:"count"`
	TotalCount int                     `json:"total-count"`
	Truncated  bool                    `json:"truncated,omitempty"`
	Entries    []supportEventRingEntry `json:"entries"`
}

type supportPluginEntry struct {
	Name             string `json:"name"`
	RegistrationName string `json:"registration-name,omitempty"`
	Mode             string `json:"mode"`
	Running          bool   `json:"running"`
	Stage            string `json:"stage,omitempty"`
	PID              int    `json:"pid,omitempty"`
	Disabled         bool   `json:"disabled,omitempty"`
	Down             bool   `json:"down,omitempty"`
	Health           string `json:"health"`
	LastError        string `json:"last-error,omitempty"`
}

type supportPluginsPayload struct {
	Available  bool                 `json:"available"`
	Count      int                  `json:"count"`
	TotalCount int                  `json:"total-count"`
	Running    int                  `json:"running"`
	Truncated  bool                 `json:"truncated,omitempty"`
	Plugins    []supportPluginEntry `json:"plugins"`
}

type supportReloadHistoryPayload struct {
	Available       bool           `json:"available"`
	Generation      uint64         `json:"generation"`
	LastOutcome     string         `json:"last-outcome"`
	LastReloadAt    string         `json:"last-reload-at,omitempty"`
	LastOperationID string         `json:"last-operation-id,omitempty"`
	Count           int            `json:"count"`
	TotalCount      int            `json:"total-count"`
	Entries         []ReloadRecord `json:"entries"`
}

// registerSupportModules registers the daemon-backed live support modules
// (event-ring, plugins, reload-history) via internal/core/supportreg and
// returns idempotent unregister closures for Server.Stop.
func (s *Server) registerSupportModules() []func() {
	activeSupportMu.Lock()
	defer activeSupportMu.Unlock()

	if len(activeSupportUnregs) > 0 {
		for _, unreg := range activeSupportUnregs {
			if unreg != nil {
				unreg()
			}
		}
		activeSupportUnregs = nil
		activeSupportOwner = nil
	}

	unregs := []func(){
		supportreg.Register(supportModuleEventRing, s.collectSupportEventRing),
		supportreg.Register(supportModulePlugins, s.collectSupportPlugins),
		supportreg.Register(supportModuleReloadHistory, s.collectSupportReloadHistory),
	}
	activeSupportOwner = s
	activeSupportUnregs = unregs

	var once sync.Once
	return []func(){
		func() {
			once.Do(func() {
				s.unregisterSupportModules(unregs)
			})
		},
	}
}

func (s *Server) unregisterSupportModules(unregs []func()) {
	activeSupportMu.Lock()
	defer activeSupportMu.Unlock()

	for _, unreg := range unregs {
		if unreg != nil {
			unreg()
		}
	}
	if activeSupportOwner == s {
		activeSupportOwner = nil
		activeSupportUnregs = nil
	}
}

// collectSupportEventRing snapshots the newest up to 200 entries from s.eventRing,
// honoring opts.SinceTime when non-zero.
func (s *Server) collectSupportEventRing(opts *supportreg.CollectOptions) (any, error) {
	if s == nil {
		return map[string]any{
			"available": false,
			"reason":    "event bus not active",
		}, nil
	}
	if s.eventRing == nil {
		return map[string]any{
			"available": false,
			"reason":    "event bus not active",
		}, nil
	}

	var since time.Time
	if opts != nil {
		since = opts.SinceTime
	}

	all := s.eventRing.Snapshot(0, "")
	entries := make([]supportEventRingEntry, 0, min(len(all), supportEventRingMaxEntries))
	total := 0
	for i := range all {
		rec := &all[i]
		if !since.IsZero() {
			if rec.Timestamp.Before(since) {
				continue
			}
		}
		total++
		if len(entries) >= supportEventRingMaxEntries {
			continue
		}
		entries = append(entries, supportEventRingEntry{
			Timestamp: rec.Timestamp.UTC().Format(time.RFC3339Nano),
			Namespace: rec.Namespace,
			EventType: rec.EventType,
			Source:    rec.Source,
			Subject:   rec.Subject,
			Direction: rec.Direction,
			OpID:      rec.OpID,
		})
	}

	return supportEventRingPayload{
		Available:  true,
		Count:      len(entries),
		TotalCount: total,
		Truncated:  total > len(entries),
		Entries:    entries,
	}, nil
}

// collectSupportPlugins snapshots live plugin health, mode, state, PID, and
// last error from ProcessManager under read-only locks, bounded to 256 entries.
func (s *Server) collectSupportPlugins(_ *supportreg.CollectOptions) (any, error) {
	if s == nil {
		return map[string]any{
			"available": false,
			"reason":    "plugin manager not active",
		}, nil
	}
	pm := s.procManager.Load()
	if pm == nil {
		return map[string]any{
			"available": false,
			"reason":    "plugin manager not active",
		}, nil
	}

	namesSet := make(map[string]struct{})
	for _, name := range pm.ConfiguredNames() {
		namesSet[name] = struct{}{}
	}
	for _, proc := range pm.AllProcesses() {
		namesSet[proc.Name()] = struct{}{}
	}

	names := make([]string, 0, len(namesSet))
	for name := range namesSet {
		names = append(names, name)
	}
	slices.Sort(names)

	total := len(names)
	truncated := false
	if len(names) > supportPluginsMaxEntries {
		names = names[:supportPluginsMaxEntries]
		truncated = true
	}

	entries := make([]supportPluginEntry, 0, len(names))
	runningCount := 0
	for _, name := range names {
		entry := buildSupportPluginEntry(pm, name)
		if entry.Running {
			runningCount++
		}
		entries = append(entries, entry)
	}

	return supportPluginsPayload{
		Available:  true,
		Count:      len(entries),
		TotalCount: total,
		Running:    runningCount,
		Truncated:  truncated,
		Plugins:    entries,
	}, nil
}

func buildSupportPluginEntry(pm *process.ProcessManager, name string) supportPluginEntry {
	disabled := pm.Disabled(name)
	stop, down := pm.Stopped(name)
	proc := pm.GetProcess(name)

	entry := supportPluginEntry{
		Name:             name,
		RegistrationName: pm.RegistrationFor(name),
		Mode:             processModeInternal,
		Disabled:         disabled,
		Down:             down,
	}

	if proc != nil {
		if entry.RegistrationName == "" {
			entry.RegistrationName = proc.RegistrationName()
		}
		entry.Mode = pluginProcessMode(proc)
		entry.Running = proc.Running()
		entry.Stage = proc.Stage().String()
		if proc.IsExternal() {
			if pid := proc.PID(); pid > 0 {
				entry.PID = pid
			}
		}
		if err := proc.StartupError(); err != nil {
			entry.LastError = err.Error()
		}
	}

	if entry.LastError == "" {
		if down {
			entry.LastError = strings.TrimSpace(stop.Detail)
			if entry.LastError == "" {
				entry.LastError = stop.Cause.String()
			}
		}
	}

	switch {
	case entry.Disabled:
		entry.Health = "disabled"
	case entry.Down:
		entry.Health = "down"
	case !entry.Running:
		entry.Health = "stopped"
	case entry.LastError != "":
		entry.Health = "degraded"
	default:
		entry.Health = "healthy"
	}

	return entry
}

// collectSupportReloadHistory snapshots the bounded reload transaction journal
// from Server.ReloadHistory without acquiring Server.txLock.
func (s *Server) collectSupportReloadHistory(opts *supportreg.CollectOptions) (any, error) {
	if s == nil {
		return map[string]any{
			"available": false,
			"reason":    "plugin server not active",
		}, nil
	}

	var since time.Time
	if opts != nil {
		since = opts.SinceTime
	}

	gen, outcome, at, opID := s.ReloadStatusWithOp()
	all := s.ReloadHistory(maxReloadHistory, "")
	entries := make([]ReloadRecord, 0, len(all))
	for _, rec := range all {
		if !since.IsZero() {
			if rec.FinishedAt.Before(since) {
				continue
			}
		}
		entries = append(entries, rec)
	}

	lastReloadAt := ""
	if !at.IsZero() {
		lastReloadAt = at.UTC().Format(time.RFC3339)
	}

	return supportReloadHistoryPayload{
		Available:       true,
		Generation:      gen,
		LastOutcome:     outcome,
		LastReloadAt:    lastReloadAt,
		LastOperationID: opID,
		Count:           len(entries),
		TotalCount:      len(all),
		Entries:         entries,
	}, nil
}

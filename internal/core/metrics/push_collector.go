// Design: docs/architecture/core-design.md -- generic periodic push-collector registry seam
// Overview: metrics.go -- metric collection interfaces (always-on)
// Related: exporter_hook.go -- gated Prometheus HTTP exporter seam

package metrics

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"
)

// PushCollectorStarter starts a periodic push collector against reg and returns
// a stop function that tears down the collector and joins its workers. A
// starter that has nothing to run may return a nil stop function.
type PushCollectorStarter func(reg Registry, log *slog.Logger) (stop func())

var (
	errPushCollectorEmptyName  = errors.New("metrics: push collector name must not be empty")
	errPushCollectorNilStarter = errors.New("metrics: push collector starter must not be nil")

	pushCollectorsMu sync.Mutex
	pushCollectors   = map[string]PushCollectorStarter{}
)

// RegisterPushCollector registers a periodic push collector starter under name.
// Typically called from init() in component packages so telemetry/exporter can
// start Ze-native push metrics without importing those components. Empty names,
// nil starters, and duplicate names are rejected. Safe for concurrent use.
func RegisterPushCollector(name string, start PushCollectorStarter) error {
	if name == "" {
		return errPushCollectorEmptyName
	}
	if start == nil {
		return errPushCollectorNilStarter
	}

	pushCollectorsMu.Lock()
	defer pushCollectorsMu.Unlock()

	if _, exists := pushCollectors[name]; exists {
		return fmt.Errorf("metrics: push collector %q already registered", name)
	}
	pushCollectors[name] = start
	return nil
}

// StartPushCollectors starts every registered push collector in sorted name
// order against reg and returns a stop closure that stops every started
// collector once in reverse order. Nil stop functions returned by starters are
// skipped. Caller MUST invoke the returned stop function when reg's lifecycle
// ends. Safe for concurrent use.
func StartPushCollectors(reg Registry, log *slog.Logger) (stop func()) {
	pushCollectorsMu.Lock()
	names := slices.Sorted(maps.Keys(pushCollectors))
	starters := make([]PushCollectorStarter, len(names))
	for i, name := range names {
		starters[i] = pushCollectors[name]
	}
	pushCollectorsMu.Unlock()

	stops := make([]func(), 0, len(starters))
	for _, start := range starters {
		if stopFn := start(reg, log); stopFn != nil {
			stops = append(stops, stopFn)
		}
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			for _, stopFn := range slices.Backward(stops) {
				stopFn()
			}
		})
	}
}

// SnapshotPushCollectorsForTest saves the registered push collectors and
// returns a restore closure for use with t.Cleanup. Safe for concurrent use.
func SnapshotPushCollectorsForTest() func() {
	pushCollectorsMu.Lock()
	saved := maps.Clone(pushCollectors)
	pushCollectorsMu.Unlock()

	return func() {
		pushCollectorsMu.Lock()
		pushCollectors = saved
		pushCollectorsMu.Unlock()
	}
}

// Design: docs/architecture/api/architecture.md — plugin registry
// Overview: registry.go -- Register, the other declaration a plugin makes from init()
// Related: doctor.go -- DoctorCheckDef, the offline probe this is not
//
// setup.go records what a plugin's own init() achieved when it set itself up,
// so a feature that is absent can say why.
//
// The record and the Registration are two independent writes keyed by plugin
// name. Neither reads the other, because Go initializes the files of one
// package in filename order and a plugin author must not have to know it.
//
// This is a REPLAY, not a probe. The outcome was decided once, before main(),
// and every reader gets that same answer back. internal/core/health answers
// the other question, by running a check now.

package registry

import (
	"slices"
	"strconv"
	"strings"
)

// SetupOutcome names what a plugin's init() achieved when it set itself up.
//
// The zero value is SetupUnknown, so a plugin that recorded nothing is never
// mistaken for one that succeeded.
type SetupOutcome uint8

const (
	// SetupUnknown is the outcome of a plugin that recorded nothing. It is a
	// stored state and never a valid argument to RecordSetup.
	SetupUnknown SetupOutcome = iota
	// SetupSucceeded is the outcome of a plugin whose setup completed.
	SetupSucceeded
	// SetupFailedSoft is the outcome of a plugin the daemon runs correctly
	// without. The feature is absent and the daemon starts.
	SetupFailedSoft
	// SetupFailedHard is the outcome of a plugin the daemon cannot run
	// without. The daemon refuses to start.
	SetupFailedHard
	// setupOutcomeCount bounds the enumeration. It is the first value that is
	// not an outcome, so RecordSetup refuses everything from here up.
	setupOutcomeCount
)

// String returns the name of the outcome, for a CLI row and for a log line.
//
// A value outside the enumeration spells itself "invalid": RecordSetup refuses
// one, so it can only arrive here through a conversion, and a reader is told
// that rather than shown a plausible outcome.
func (o SetupOutcome) String() string {
	switch o {
	case SetupUnknown:
		return "unknown"
	case SetupSucceeded:
		return "succeeded"
	case SetupFailedSoft:
		return "soft-failure"
	case SetupFailedHard:
		return "hard-failure"
	default:
		return "invalid"
	}
}

// MarshalJSON writes the outcome as the word an operator reads, so `| json`,
// `| yaml` and `| table` all carry "soft-failure" rather than the number 2.
// The outcome is a CLI value here, and a string is what a boundary takes.
func (o SetupOutcome) MarshalJSON() ([]byte, error) {
	return strconv.AppendQuote(nil, o.String()), nil
}

// SetupResult is one plugin's recorded setup outcome.
type SetupResult struct {
	Plugin  string       `json:"plugin"`
	Outcome SetupOutcome `json:"outcome"`
	Reason  string       `json:"reason,omitempty"`
}

// setupResults holds one recorded result per plugin name, written by plugin
// init() functions before main(). It shares mu with the plugin map, and Reset,
// Snapshot and Restore carry it with the registry it belongs to.
var setupResults = make(map[string]SetupResult)

// RecordSetup records what a plugin's setup achieved. Call it from the
// plugin's own init(), with the outcome and, for a failure, the reason an
// operator acts on. A second record for one plugin replaces the first.
//
// The reason reaches CLI output as data, so a recording site MUST NOT put a
// secret in it.
//
// An empty plugin name, SetupUnknown, and any value outside the enumeration
// are programmer errors: each one records a row that says nothing, and a row
// that says nothing is indistinguishable from the silence this registry
// exists to remove.
func RecordSetup(plugin string, outcome SetupOutcome, reason string) {
	if plugin == "" {
		panic("BUG: registry.RecordSetup: no plugin name")
	}
	if outcome == SetupUnknown {
		panic("BUG: registry.RecordSetup: SetupUnknown is a stored state, not an outcome to record: " + plugin)
	}
	if outcome >= setupOutcomeCount {
		panic("BUG: registry.RecordSetup: outcome outside the enumeration: " + plugin)
	}

	mu.Lock()
	defer mu.Unlock()
	setupResults[plugin] = SetupResult{Plugin: plugin, Outcome: outcome, Reason: reason}
}

// SetupResults returns one result for every plugin, in name order.
//
// The set is every registered plugin UNION every plugin that recorded. A
// registered plugin that recorded nothing is listed as SetupUnknown rather
// than omitted, because absence reads as "not built in" and would hide the one
// plugin that owes a record. A plugin that recorded and then failed to
// register keeps its row for the same reason: it is the loudest case.
func SetupResults() []SetupResult {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(plugins)+len(setupResults))
	for name := range plugins {
		names = append(names, name)
	}
	for name := range setupResults {
		if _, registered := plugins[name]; !registered {
			names = append(names, name)
		}
	}
	slices.Sort(names)

	results := make([]SetupResult, len(names))
	for i, name := range names {
		result, recorded := setupResults[name]
		if !recorded {
			result = SetupResult{Plugin: name, Outcome: SetupUnknown}
		}
		results[i] = result
	}
	return results
}

// HardSetupFailures returns every plugin that recorded SetupFailedHard, in
// name order. An empty answer means no plugin recorded one: this function
// reads a map that is always present, so it has no failure of its own to
// report as emptiness.
func HardSetupFailures() []SetupResult {
	mu.RLock()
	defer mu.RUnlock()

	failures := make([]SetupResult, 0, len(setupResults))
	for _, result := range setupResults {
		if result.Outcome == SetupFailedHard {
			failures = append(failures, result)
		}
	}
	slices.SortFunc(failures, func(a, b SetupResult) int {
		return strings.Compare(a.Plugin, b.Plugin)
	})
	return failures
}

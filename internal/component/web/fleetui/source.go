// Design: docs/architecture/fleet-interface.md -- the data source seam of the fleet interface
// Detail: source_devices.go -- the registry questions and the device facts
// Detail: source_history.go -- the sample history questions
// Detail: source_activity.go -- the alarm and task questions
// Detail: source_config.go -- the configuration, rollout, audit and administration questions
// Detail: source_views.go -- the questions only a device can answer, and the console
// Detail: source_events.go -- the live event subscription
// Detail: source_errors.go -- the failures a source reports, and how a page reads them
// Related: register.go -- the routes that mount only while a source is registered
//
// The fleet pages never read an estate themselves. They ask one Source, and a
// build links exactly one: the sample estate under the ze_fleet_ui_sample tag
// today, the source built over the command dispatcher later. The pages and
// their markup stay the same when the source changes.
//
// A source answers facts. Everything a page derives from them (the display
// state, the tree, a filtered and sorted page of a table, the counts on a
// card, a series reduced to the width of a chart) is computed in this package
// by pure functions, so the real source only has to supply facts.

package fleetui

import (
	"errors"
	"sync"
)

// Source answers the questions the fleet pages ask. It is the union of one
// interface per concern, and each method names the pages that ask it and the
// fleet spec expected to answer it for real.
//
// An implementation MUST be safe for concurrent use: every request calls it
// from its own goroutine. Every answer MUST be the caller's own copy, holding
// no slice the source still writes to. Every slice answer is bounded by the
// constant its method names, and a source MUST NOT answer more.
//
// When a source cannot answer, it MUST return an operator error
// (internal/core/opererr) wrapping one of the errors of source_errors.go where
// one fits, and MUST NOT return an empty answer in its place, because the page
// shows the error text to the operator as it is.
type Source interface {
	RegistrySource
	HistorySource
	ActivitySource
	ConfigSource
	DeviceViewSource
	EventSource
}

// The two ways a registration is refused. Each is a defect of the package
// that registers, never an operating condition.
var (
	errSourceAbsent     = errors.New("the data source is nil")
	errSourceRegistered = errors.New("a data source is already registered, and one build links one source")
)

// sourceSlot holds the one data source of a build. The zero value is an empty
// slot, ready for use. Safe for concurrent use.
type sourceSlot struct {
	mu     sync.RWMutex
	source Source
}

// register stores the source. It refuses a nil source and a second source,
// and leaves the slot as it was when it refuses.
func (slot *sourceSlot) register(source Source) error {
	if source == nil {
		return errSourceAbsent
	}

	slot.mu.Lock()
	defer slot.mu.Unlock()

	if slot.source != nil {
		return errSourceRegistered
	}
	slot.source = source

	return nil
}

// registered answers the source and true, or false when the slot is empty.
// False is what keeps the fleet routes unmounted: see sourceRegistered.
func (slot *sourceSlot) registered() (Source, bool) {
	slot.mu.RLock()
	defer slot.mu.RUnlock()

	if slot.source == nil {
		return nil, false
	}

	return slot.source, true
}

// buildSource is the slot of this build. A source package fills it from its
// init(), and nothing empties it.
var buildSource sourceSlot

// MustRegisterSource makes source the data source of this build. Call it from
// init() in the source package's register.go.
//
// It panics on a nil source and on a second registration. Both are defects of
// the build's composition, found at process start, never a state a request
// can reach.
func MustRegisterSource(source Source) {
	if err := buildSource.register(source); err != nil {
		panic("BUG: fleetui.MustRegisterSource: " + err.Error())
	}
}

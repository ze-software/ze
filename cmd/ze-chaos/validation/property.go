// Design: docs/architecture/chaos-web-dashboard.md — property-based validation

package validation

import (
	"fmt"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/peer"
)

// Violation represents a property assertion failure.
type Violation struct {
	// Property is the kebab-case name of the violated property.
	Property string

	// RFC is the RFC reference (e.g., "RFC 4271 Section 9"), empty for operational properties.
	RFC string

	// Message describes the specific violation.
	Message string

	// PeerIndex identifies which peer the violation relates to (-1 if global).
	PeerIndex int

	// Time is when the violation was detected.
	Time time.Time
}

// Property defines a composable, named assertion that can be checked
// independently against the event stream. Properties are the unit of
// validation — each checks one RFC or operational invariant.
type Property interface {
	// Name returns the kebab-case identifier used in --properties flag.
	Name() string

	// Description returns a human-readable summary for --properties list.
	Description() string

	// RFC returns the RFC reference string (empty for operational properties).
	RFC() string

	// ProcessEvent feeds an event into this property's internal state.
	ProcessEvent(ev peer.Event)

	// Violations returns all violations detected so far.
	// May trigger final computation for properties that check at end-of-run.
	Violations() []Violation

	// Reset clears internal state for a new run or replay.
	Reset()
}

// PropertyResult holds the pass/fail result for a single property.
type PropertyResult struct {
	Name       string
	Pass       bool
	Violations []Violation
}

// PropertyEngine dispatches events to registered properties and collects results.
type PropertyEngine struct {
	props []Property
}

// NewPropertyEngine creates an engine with the given properties.
func NewPropertyEngine(props []Property) *PropertyEngine {
	return &PropertyEngine{props: props}
}

// ProcessEvent dispatches an event to all registered properties.
func (e *PropertyEngine) ProcessEvent(ev peer.Event) {
	for _, p := range e.props {
		p.ProcessEvent(ev)
	}
}

// Results returns per-property pass/fail results.
func (e *PropertyEngine) Results() []PropertyResult {
	results := make([]PropertyResult, len(e.props))
	for i, p := range e.props {
		v := p.Violations()
		results[i] = PropertyResult{
			Name:       p.Name(),
			Pass:       len(v) == 0,
			Violations: v,
		}
	}
	return results
}

// AllViolations returns all violations across all properties.
func (e *PropertyEngine) AllViolations() []Violation {
	all := make([]Violation, 0, len(e.props))
	for _, p := range e.props {
		all = append(all, p.Violations()...)
	}
	return all
}

// AllProperties creates all available properties for n peers with given convergence deadline.
func AllProperties(n int, deadline time.Duration) []Property {
	return []Property{
		NewRouteConsistency(n),
		NewConvergenceDeadline(n, deadline),
		NewNoDuplicateRoutes(n),
		NewHoldTimerEnforcement(n),
		NewMessageOrdering(n),
	}
}

// SelectProperties filters properties by name, returning an error for unknown names.
func SelectProperties(all []Property, names []string) ([]Property, error) {
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	var selected []Property
	for _, p := range all {
		if nameSet[p.Name()] {
			selected = append(selected, p)
			delete(nameSet, p.Name())
		}
	}
	for name := range nameSet {
		return nil, fmt.Errorf("unknown property: %s", name)
	}
	return selected, nil
}

// ListProperties returns formatted lines of "name  description" for all properties.
func ListProperties(props []Property) []string {
	lines := make([]string, len(props))
	for i, p := range props {
		rfc := ""
		if p.RFC() != "" {
			rfc = " (" + p.RFC() + ")"
		}
		lines[i] = fmt.Sprintf("  %-25s %s%s", p.Name(), p.Description(), rfc)
	}
	return lines
}

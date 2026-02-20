// Design: docs/architecture/chaos-web-dashboard.md — test case shrinking

package shrink

import (
	"fmt"
	"io"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/peer"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/validation"
)

// Config holds parameters for the shrink engine.
type Config struct {
	// PeerCount is the number of peers in the simulation.
	PeerCount int

	// Deadline is the convergence deadline for property checks.
	Deadline time.Duration

	// Verbose is the writer for progress output (nil for silent).
	Verbose io.Writer
}

// Result holds the output of shrinking.
type Result struct {
	// Events is the minimal event sequence that still triggers the violation.
	Events []peer.Event

	// Property is the name of the violated property.
	Property string

	// Original is the number of events in the input.
	Original int

	// Iterations is the total number of replay iterations performed.
	Iterations int
}

// verbosef writes a formatted message to cfg.Verbose if it is non-nil.
// Write errors on progress output are not actionable, so they are ignored.
func verbosef(cfg Config, format string, args ...any) {
	if cfg.Verbose != nil {
		if _, err := fmt.Fprintf(cfg.Verbose, format, args...); err != nil {
			return // best-effort progress output
		}
	}
}

// Run takes a failing event sequence and returns the smallest subsequence
// that still triggers the same property violation.
//
// Algorithm:
//  1. Verify the input actually fails (replay through all properties).
//  2. Binary search: try halves to coarsely narrow the boundary.
//  3. Single-step elimination: try removing each event (with causal dependents).
func Run(events []peer.Event, cfg Config) (*Result, error) {
	if len(events) == 0 {
		return nil, fmt.Errorf("no events to shrink")
	}

	original := len(events)
	iterations := 0

	// Phase 1: verify the input fails.
	violation := findViolation(events, cfg)
	iterations++
	if violation == nil {
		return nil, fmt.Errorf("no violation found in event log")
	}
	property := violation.Property

	verbosef(cfg, "shrink: found %q violation in %d events\n", property, len(events))

	// Phase 2: binary search to coarsely narrow.
	events, n := binarySearch(events, property, cfg)
	iterations += n

	verbosef(cfg, "shrink: binary search narrowed to %d events (%d iterations)\n", len(events), n)

	// Phase 3: single-step elimination.
	events, n = singleStepEliminate(events, property, cfg)
	iterations += n

	verbosef(cfg, "shrink: elimination produced %d events (%d iterations)\n", len(events), n)

	return &Result{
		Events:     events,
		Property:   property,
		Original:   original,
		Iterations: iterations,
	}, nil
}

// findViolation replays events through all properties and returns the first violation.
func findViolation(events []peer.Event, cfg Config) *validation.Violation {
	engine := newEngine(cfg)
	for i := range events {
		engine.ProcessEvent(events[i])
	}
	violations := engine.AllViolations()
	if len(violations) == 0 {
		return nil
	}
	return &violations[0]
}

// hasViolation checks if replaying events triggers a violation for the named property.
func hasViolation(events []peer.Event, property string, cfg Config) bool {
	engine := newEngine(cfg)
	for i := range events {
		engine.ProcessEvent(events[i])
	}
	for _, v := range engine.AllViolations() {
		if v.Property == property {
			return true
		}
	}
	return false
}

// newEngine creates a fresh PropertyEngine with all properties.
func newEngine(cfg Config) *validation.PropertyEngine {
	props := validation.AllProperties(cfg.PeerCount, cfg.Deadline)
	return validation.NewPropertyEngine(props)
}

// binarySearch coarsely narrows the event list by trying halves.
// Returns the narrowed event list and the number of replay iterations used.
func binarySearch(events []peer.Event, property string, cfg Config) ([]peer.Event, int) {
	iterations := 0

	// 3 is the minimum viable event set for most violations
	// (e.g., Established(peer0) + Established(peer1) + RouteSent).
	// Below this threshold, binary search cannot meaningfully halve.
	for len(events) > 3 {
		mid := len(events) / 2

		// Try first half.
		first := events[:mid]
		iterations++
		if hasViolation(first, property, cfg) {
			events = first
			continue
		}

		// Try second half.
		second := events[mid:]
		iterations++
		if hasViolation(second, property, cfg) {
			events = second
			continue
		}

		// Neither half alone fails — stop binary search.
		break
	}

	return events, iterations
}

// singleStepEliminate tries removing each event (with causal dependents)
// and keeps the removal if the violation persists.
// Returns the minimal event list and the number of replay iterations used.
func singleStepEliminate(events []peer.Event, property string, cfg Config) ([]peer.Event, int) {
	iterations := 0

	for i := len(events) - 1; i >= 0; i-- {
		candidate := RemoveWithDependents(events, i)
		if len(candidate) >= len(events) {
			// Nothing was actually removed.
			continue
		}

		iterations++
		if hasViolation(candidate, property, cfg) {
			verbosef(cfg, "  removed event %d (%s peer %d), %d remaining\n",
				i, events[i].Type, events[i].PeerIndex, len(candidate))
			events = candidate
			// Don't adjust i — events before index i are unchanged,
			// and we're decrementing, so we'll naturally try i-1 next.
		}
	}

	return events, iterations
}

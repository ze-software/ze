// Design: docs/architecture/behavior/signals.md -- the daemon announces each operating-system signal it takes
// Related: ../events/typed.go -- the typed handle this package declares

// Package daemonevents declares the events the daemon itself publishes on the
// EventBus, as opposed to a subsystem or a plugin. It is a leaf, so the
// always-on hub can publish and a gated subsystem can subscribe without either
// importing the other.
package daemonevents

import "github.com/ze-software/ze/internal/core/events"

// Namespace is the EventBus namespace of the daemon's own events.
const Namespace = "daemon"

// Signal is published once for every operating-system signal the daemon takes
// (SIGHUP, SIGINT, SIGTERM), before the daemon acts on it. A stop that a
// command or the end of stdin asked for is not a signal and publishes nothing.
var Signal = events.Register[Received](Namespace, "signal")

// Received is one operating-system signal the daemon took.
type Received struct {
	// Number is the signal number, 1 for SIGHUP and 15 for SIGTERM.
	Number int `json:"number"`
	// Name is the signal's C name, "SIGHUP", the word ExaBGP's API writes.
	Name string `json:"name"`
}

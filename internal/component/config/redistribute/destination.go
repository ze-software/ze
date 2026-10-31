// Design: docs/guide/redistribution.md -- sources and destinations
// Related: consumer_factory.go -- ConsumerFactory, a destination whose consumer the orchestrator builds over its own engine connection
// Related: registry.go -- RouteSource, whose ConfigRoot states the same need for a source

package redistribute

import (
	"errors"
	"fmt"
	"sync"
)

// ErrDestinationConflict is returned when a destination name is declared twice
// with a different config root, or declared with no name.
var ErrDestinationConflict = errors.New("redistribute: destination declaration conflict")

// Destination declares a protocol that can receive redistributed routes, and
// the config root that protocol runs from.
//
// The declaration is static: the protocol's own package registers it from
// init(), so the config doors (config.RefuseRedistributeRules) can judge a
// rule before any plugin starts. A consumer, by contrast, registers when its
// plugin starts (IS-IS and OSPF in OnStarted) or is built by the orchestrator
// from a ConsumerFactory (BGP), so it is not visible at load or commit.
type Destination struct {
	// Name is the destination name an operator writes after `destination`.
	Name string
	// ConfigRoot is the config root the destination's protocol runs from. A
	// rule naming the destination in a config without that root is refused
	// (config.RefuseRedistributeRules), because nothing would receive the
	// routes. Empty means the destination needs no root.
	ConfigRoot string
}

var (
	destinationMu sync.RWMutex
	destinations  = map[string]Destination{}
)

// RegisterDestination adds d to the destination registry. Declaring the same
// name with the same root again is a no-op, so a caller that runs more than
// once needs no guard of its own. A different root under the same name, or no
// name, is refused.
func RegisterDestination(d Destination) error {
	if d.Name == "" {
		return fmt.Errorf("%w: a destination with no name", ErrDestinationConflict)
	}
	destinationMu.Lock()
	defer destinationMu.Unlock()
	if existing, ok := destinations[d.Name]; ok {
		if existing.ConfigRoot != d.ConfigRoot {
			return fmt.Errorf("%w: %q declared with root %q, got %q",
				ErrDestinationConflict, d.Name, existing.ConfigRoot, d.ConfigRoot)
		}
		return nil
	}
	destinations[d.Name] = d
	return nil
}

// LookupDestination returns the declaration registered for a destination name.
func LookupDestination(name string) (Destination, bool) {
	destinationMu.RLock()
	defer destinationMu.RUnlock()
	d, ok := destinations[name]
	return d, ok
}

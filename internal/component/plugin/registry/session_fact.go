// Design: docs/architecture/api/process-protocol.md -- Filter Insertion
// Related: filter_insertion.go -- the judge that reads the registered facts

package registry

import (
	"slices"
	"sync"
)

// sessionFacts holds the session facts a filter insertion's session
// condition may name. The owner of the facts registers each one from its own
// init (the BGP view, policyview), so the judge knows no fact by name, and a
// build without the owner refuses every session condition.
var sessionFacts struct {
	mu    sync.RWMutex
	names []string
}

// RegisterSessionFact records name as a session fact an insertion's session
// condition may name. The owner MUST be able to answer the fact for a peer
// from its config, or from its OPEN for a dynamic peer: a fact known only per
// route (the source of an exported route) is not a condition of a peer.
// Registering a name twice is a defect of the owner and panics.
func RegisterSessionFact(name string) {
	sessionFacts.mu.Lock()
	defer sessionFacts.mu.Unlock()
	if slices.Contains(sessionFacts.names, name) {
		panic("BUG: registry: session fact " + name + " registered twice")
	}
	sessionFacts.names = append(sessionFacts.names, name)
}

// SessionFactRegistered reports whether some owner registered name.
// Safe for concurrent use.
func SessionFactRegistered(name string) bool {
	sessionFacts.mu.RLock()
	defer sessionFacts.mu.RUnlock()
	return slices.Contains(sessionFacts.names, name)
}

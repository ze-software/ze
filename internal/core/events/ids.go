// Design: docs/architecture/api/process-protocol.md -- typed event IDs for hot-path matching
// Related: events.go -- namespace/eventType string registry populates ID maps

package events

// NamespaceID is a compact integer identifier for an event namespace.
// Assigned sequentially at RegisterNamespace time. Zero means unknown.
type NamespaceID uint8

const NamespaceUnknown NamespaceID = 0

func (id NamespaceID) String() string {
	eventsMu.RLock()
	defer eventsMu.RUnlock()
	if s, ok := namespaceNames[id]; ok {
		return s
	}
	return "unknown"
}

// EventTypeID is a compact integer identifier for an event type string.
// Assigned sequentially across all namespaces (same string = same ID).
// Zero means unknown.
type EventTypeID uint16

const EventTypeUnknown EventTypeID = 0

func (id EventTypeID) String() string {
	eventsMu.RLock()
	defer eventsMu.RUnlock()
	if s, ok := eventTypeNames[id]; ok {
		return s
	}
	return "unknown"
}

// Direction is a typed enum for event subscription direction filtering.
type Direction uint8

const (
	DirUnspecified Direction = 0
	DirReceived    Direction = 1
	DirSent        Direction = 2
	DirBoth        Direction = 3
)

func (d Direction) String() string {
	switch d {
	case DirReceived:
		return DirectionReceived
	case DirSent:
		return DirectionSent
	case DirBoth:
		return DirectionBoth
	case DirUnspecified:
		return ""
	}
	return ""
}

// ParseDirection converts a direction string to a typed Direction.
func ParseDirection(s string) Direction {
	switch s {
	case DirectionReceived:
		return DirReceived
	case DirectionSent:
		return DirSent
	case DirectionBoth:
		return DirBoth
	}
	return DirUnspecified
}

// LookupNamespaceID returns the NamespaceID for the given namespace string.
// Returns NamespaceUnknown if the namespace has not been registered.
func LookupNamespaceID(namespace string) NamespaceID {
	eventsMu.RLock()
	defer eventsMu.RUnlock()
	return namespaceIDs[namespace]
}

// LookupEventTypeID returns the EventTypeID for the given event type string.
// Returns EventTypeUnknown if the event type has not been registered.
func LookupEventTypeID(eventType string) EventTypeID {
	eventsMu.RLock()
	defer eventsMu.RUnlock()
	return eventTypeIDs[eventType]
}

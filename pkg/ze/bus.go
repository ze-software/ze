// Design: docs/plan/spec-arch-0-system-boundaries.md — Bus interface and event types

// Package ze defines the boundary interfaces for Ze's architecture.
//
// Five components communicate through these interfaces:
//   - Engine: supervisor that starts/stops all components
//   - Bus: content-agnostic pub/sub with hierarchical topics
//   - ConfigProvider: config loading, validation, and serving
//   - PluginManager: plugin lifecycle (5-stage protocol)
//   - Subsystem: first-class daemon (e.g., BGP) that owns external I/O
//
// The Bus never inspects event payloads. Topics are hierarchical strings
// with "/" separators. Subscriptions match on topic prefixes.
package ze

// Bus is a content-agnostic pub/sub message backbone.
//
// It moves opaque payloads between producers and consumers without
// inspecting their contents. Topics are hierarchical strings using "/"
// as separator (e.g., "bgp/update", "bgp/events/peer-up").
// Subscriptions match on topic prefixes.
type Bus interface {
	// CreateTopic registers a new hierarchical topic.
	CreateTopic(name string) (Topic, error)

	// Publish sends an opaque event to a topic.
	// The bus never reads the payload.
	Publish(topic string, payload []byte, metadata map[string]string)

	// Subscribe registers a consumer for all topics matching the given prefix.
	// An empty filter matches all events; non-empty filters match against
	// event metadata key-value pairs.
	Subscribe(prefix string, filter map[string]string, consumer Consumer) (Subscription, error)

	// Unsubscribe removes a subscription.
	Unsubscribe(sub Subscription)
}

// Consumer receives events from the bus.
type Consumer interface {
	// Deliver receives a batch of events. The consumer owns decoding.
	Deliver(events []Event) error
}

// Event is an opaque message on the bus.
type Event struct {
	// Topic is the hierarchical topic this event was published to.
	Topic string

	// Payload is opaque content — the bus never reads this.
	Payload []byte

	// Metadata carries key-value pairs for subscription filtering
	// (e.g., "peer" → "192.0.2.1", "direction" → "received").
	Metadata map[string]string
}

// Topic is a registered bus topic.
type Topic struct {
	// Name is the hierarchical topic name (e.g., "bgp/update").
	Name string
}

// Subscription is a handle to an active bus subscription.
type Subscription struct {
	// ID uniquely identifies this subscription within the bus.
	ID uint64

	// Prefix is the topic prefix this subscription matches.
	Prefix string
}

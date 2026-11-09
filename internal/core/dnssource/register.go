// Design: docs/features/dns-resolver.md -- the router's resolvers and the system-dns event
// Related: record.go -- the Snapshot the event carries and its namespace names
// Related: follow.go -- the consumer view a subscriber keeps of the snapshots

package dnssource

import "github.com/ze-software/ze/internal/core/events"

// SnapshotEvent carries the resolver registry's state to every consumer, in
// the daemon or in a plugin process. The registry
// (internal/component/config/system) is its only producer; a consumer in the
// daemon subscribes to the typed handle, and a plugin process receives the
// JSON form and hands it to Follower.Accept.
var SnapshotEvent = events.Register[*Snapshot](Namespace, SnapshotEventType)

// SnapshotRequest asks the registry to emit SnapshotEvent again. A consumer
// MUST emit it after it subscribes, because a snapshot sent before the
// subscription is not delivered to it.
var SnapshotRequest = events.RegisterSignal(Namespace, SnapshotRequestEventType)

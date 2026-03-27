// Design: docs/architecture/fleet-config.md — managed client component
//
// Package managed implements the managed configuration client.
// It connects to a hub, fetches configuration, caches it locally,
// handles config change notifications, and reconnects with backoff.
package managed

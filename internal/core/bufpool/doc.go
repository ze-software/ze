// Design: .claude/rules/design-principles.md -- "Pool strategy by goroutine shape"

// Package bufpool provides a sync.Pool-seeded-for-peak byte-slice pool
// for protocol subsystems that share a buffer path across multiple
// goroutines (TACACS+ AAA, plugin-rpc framing, BGP BMP sender, etc.).
//
// Shape: every buffer in a given pool is the SAME maximum size, so Get
// callers never resize. Seed N with peak concurrent wire activity so
// sync.Pool's New func is the last-resort fallback rather than a regular
// allocation path.
//
// When to use this vs a single-backing ring:
//
//   - Multiple goroutines share the buffer path       -> bufpool (this)
//   - One reactor goroutine consuming in sequence     -> single-backing
//     ring (see internal/component/bgp/reactor/forward_pool.go)
package bufpool

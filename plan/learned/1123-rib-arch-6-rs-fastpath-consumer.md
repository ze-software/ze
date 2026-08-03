# 1123 -- rib-arch-6: First Production Change.Forward Consumer (RS/RR Fast Path)

## Context

The zero-copy forward-handle producer wiring (learned 784) attached a `Change.Forward`
handle to Loc-RIB best-changes so a subscriber could forward the received UPDATE wire bytes
without rebuilding them. Its only consumer was `observeForwardHandles`
(`internal/component/bgp/plugins/rib/forward_observer.go`), a debug observer that only
nil-checks the handle and logs -- it never AddRefs or reads the bytes. rib-arch-6 builds the
first REAL consumer, proving the producer wiring end-to-end with a correct AddRef/Release
lifecycle and race-safety.

## Decisions

- **Build `forwardStateTracker`, gated inert-by-default.** Chosen over an always-on consumer
  because the copy-out (first AddRef materialises an owned buffer copy) costs per received
  route; a binary that never enables the fast path must pay nothing. `onChange` is a single
  `enabled.Load()` when disabled.
- **AddRef under the RIB write lock, process off-lock in a worker.** The `OnChange` handler
  runs under the lock (`locrib/change.go`), so the handler does only the bounded
  copy-out (AddRef) + a non-blocking enqueue; a worker reads `Bytes()` and Releases off-lock.
  This honours the "copy out under lock, process off-lock" rule and never contends the
  tracker mutex under the RIB lock.
- **Backpressure releases, never leaks.** A full queue releases the handle immediately and
  counts the drop; `Stop()` drains the queue and does a final non-blocking drain to catch a
  handle enqueued by an onChange that raced the stop. Every AddRef is matched by a Release.
- **Diagnostic lives on `request bgp rib fastpath <enable|disable|status>`, not `show`.**
  The `show bgp rib` verb greedily parses trailing tokens as pipeline filters, so
  `show bgp rib fastpath` tokenised to `show bgp rib` + arg `fastpath` ("unknown keyword").
  The `request` verb has no such shadow, so a `CommandDecl` alone (no YANG container) routes
  it correctly and passes `ze-cli-grammar-check`.

## Consequences

- The zero-copy `Change.Forward` path now has a validated real consumer; future consumers
  (sysrib mirroring, RS/RR forwarding cache) can follow the same AddRef/Release pattern.
- Adding a plugin command requires a `CommandDecl` in the plugin's `Run` registration (not
  just the internal `registeredCommands` table) or the engine returns "unknown command".
- A `show`-style subcommand under a filter-parsing verb needs a YANG grammar container to
  avoid being swallowed as a filter; a `request`-style command does not.

## Gotchas

- `-race` requires cgo. The `ze-unit-test-changed` make target forces `CGO_ENABLED=0`, so run
  `CGO_ENABLED=1 go test -race ./...` directly to exercise the race detector.
- `request bgp rib inject` produces NO `Change.Forward` (injected routes carry no wire bytes,
  `rib_bestchange.go`), so the `.ci` needs a real peer-announced route to exercise the
  handle.
- `protocol_test.go` guards the exact bgp-rib command count; adding a command bumps it (18→19).

## Files

- `internal/component/bgp/plugins/rib/forward_tracker.go` -- the consumer + `fastpathCommand`
- `internal/component/bgp/plugins/rib/rib.go` -- SetLocRIB wiring + CommandDecl
- `internal/component/bgp/plugins/rib/rib_commands.go` -- builtin command registration
- `internal/component/bgp/plugins/rib/forward_tracker_test.go` -- unit + race tests
- `test/plugin/rs-fastpath.ci` -- end-to-end functional test

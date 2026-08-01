# Learned: redist-explicit-dest

Spec: spec-redist-explicit-dest.md
Date: 2026-05-13

## What changed

Added a destination-protocol nesting level to the redistribute config block.
Old: `redistribute { import l2tp { ... } }`.
New: `redistribute { destination bgp { import l2tp { ... } } }`.

Introduced `RedistConsumer` interface and consumer registry. Replaced the
BGP-specific `bgp-redistribute-egress` plugin with a generic
`redistribute-orchestrator` that dispatches to registered consumers.

## Key decisions

1. **YANG list, not container:** Used `list destination { key "protocol" }` for
   extensibility. Config syntax is `destination bgp { ... }` (YANG list key
   rendering), not `bgp { ... }` as initially envisioned in the spec. The list
   approach lets future protocols (OSPF, ISIS) add entries without schema changes.

2. **Consumer registered by orchestrator, not BGP component:** The BGP consumer
   needs a `RouteDispatcher` (SDK plugin connection), which is only available
   inside the plugin's `runPlugin`. The orchestrator plugin creates and registers
   the BGP consumer in its `OnStarted` handler. Future protocols would register
   their consumers in their own plugins.

3. **Evaluator stays flat:** The evaluator receives all import rules from all
   destinations as a flat `[]ImportRule`. Loop prevention (`route.Origin ==
   importingProtocol`) handles per-destination semantics. This works for single
   and multi-destination configs because the evaluator checks each consumer's
   name independently.

4. **No YANG validator on destination protocol leaf:** Consumers register at
   plugin startup (after YANG parse), so `ze:validate` cannot check at parse
   time. Runtime validation via the orchestrator's "no consumers" warning
   covers the gap.

## Bugs caught by review

- **Consumer never registered (BLOCKER):** The interface and implementation
  existed but nobody called `RegisterConsumer`. The orchestrator iterated zero
  consumers and dispatched to nothing. All redistributed routes silently
  vanished. Caught by wiring verification step.

- **Silent error discarding (BLOCKER):** `BGPConsumer.InjectRoute` and
  `WithdrawRoute` did `_, _, _ = dispatcher.UpdateRoute(...)` with no logging.
  Failed dispatches became invisible.

## Patterns for future work

- The consumer registry mirrors the source registry (package-level, RWMutex,
  sorted names). Both will need per-VRF scoping when VRF lands.

- Adding a new destination protocol: implement `RedistConsumer`, register it in
  the protocol's plugin `OnStarted`, add a YANG `destination` key in the config.
  No orchestrator changes needed.

## Mistakes to avoid

- When replacing a direct function call with an interface dispatch, always
  verify the interface implementation is actually instantiated and registered
  somewhere. "The code compiles" does not prove wiring.

- When moving format/builder functions to a new package, check that error
  handling behavior is preserved. The old code logged errors; the new code
  initially swallowed them.

## Files

None recorded.

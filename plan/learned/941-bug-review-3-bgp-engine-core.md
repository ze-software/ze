# bug-review-3-bgp-engine-core

## Summary

BGP core review must compare path semantics, not just inspect individual functions. Receive callbacks, forwarding helpers, DirectBridge, RS inline forwarding, negotiated contexts, and startup cleanup can diverge even when each local function looks reasonable.

## Key decisions

- Reviewed receive, build, capability/context, forwarding, route-refresh, cache/pool, reload/startup, and hot-path allocation surfaces as matrices.
- Required RFC summaries before promoting protocol findings.
- Accepted allocation findings only with a concrete hot-path source trigger and a fix spec that requires allocation proof before broader refactor.

## Results

- Created `plan/review-bug-review-bgp-engine.md`.
- Accepted BENG-001 and BENG-003 into `plan/spec-bugfix-bgp-message-validation-before-delivery.md`.
- Accepted BENG-002 into `plan/spec-bugfix-bgp-forward-split-context.md`.
- Accepted BENG-004 into `plan/spec-bugfix-bgp-reactor-startup-cleanup.md`.
- Accepted BENG-005 into `plan/spec-bugfix-bgp-next-hop-alloc.md` with an allocation-confirming gate.

## Gotchas

- Message validation must run before delivery to plugin or observer callbacks. UPDATE already had that invariant, ROUTE-REFRESH did not.
- Splitting oversized UPDATEs is only safe after the destination encoding context is known.
- Startup failure after listener/cache setup must use the same abort path as earlier runtime startup failures.

## Verification

- Child report includes receive, build, forward, refresh/reload, RFC, and allocation matrices.
- Follow-up specs include unit tests for malformed capabilities, malformed ROUTE-REFRESH, context-mismatch split, startup cleanup, and next-hop allocation.

## Files

None recorded.

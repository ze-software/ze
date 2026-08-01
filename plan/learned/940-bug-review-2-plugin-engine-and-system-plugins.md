# bug-review-2-plugin-engine-and-system-plugins

## Summary

Plugin correctness is a lifecycle property. Startup, reload, rollback, shutdown, registry visibility, and DirectBridge callback behavior must be reviewed together because a partial plugin can leave user-visible surfaces behind.

## Key decisions

- Reviewed plugin engine, SDK/RPC bridge, DirectBridge, config-root autoload, central command roots, directory-only commands, and representative system/component plugin shapes.
- Treated DirectBridge and external pipe paths separately. DirectBridge is not just an optimization when its error semantics diverge.
- Routed lifecycle defects to a shared rollback spec when they enforce the same exact-or-reject invariant.

## Results

- Created `plan/review-bug-review-plugin-engine-system.md`.
- Accepted SYS-001 and SYS-002 into `plan/spec-bugfix-sys-plugin-lifecycle-rollback.md`.
- Accepted SYS-003 into `plan/spec-bugfix-sys-directbridge-panic.md`.
- Left SYS-004 and SYS-005 plausible because no current hard-dependency failure or end-to-end VPP parser bypass was proven.

## Gotchas

- A plugin that fails after registering commands, families, or capabilities must be indistinguishable from one that never loaded.
- Reload cleanup APIs must take the same identifier type that autoload produced. Plugin names and config roots are not interchangeable.
- Panic recovery outside the DirectBridge event loop cannot answer the specific caller waiting on a result channel.

## Verification

- Child report includes wiring/coverage audit table, accepted findings, plausible findings, rejected candidates, and assumptions resolved.

## Files

None recorded.

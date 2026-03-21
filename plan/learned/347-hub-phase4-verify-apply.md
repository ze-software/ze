# 347 — Hub Phase 4: Verify/Apply Protocol

## Objective

Implement the Hub-side two-phase commit for configuration: Config Reader sends `config verify` per block, Hub routes to plugins by handler prefix, then (if all pass) `config apply` is routed similarly.

## Decisions

- All-verify-before-any-apply: the first verify failure aborts the entire transaction. No partial application.
- Apply failure = startup aborted; no partial state recovery. System either runs with complete config or doesn't start.
- YANG predicate paths (`bgp.peer[address=192.0.2.1]`) must be stripped before handler prefix matching — `FindHandler()` extended with `stripPredicates()` helper.
- Escape handling: `\"` in double-quoted fields and `\'` in single-quoted JSON data are unescaped. Both quote styles used in verify command serialization.

## Patterns

- `ParseVerifyCommand()` extracts handler, action, path, and data from the text command — centralized parsing avoids scattered string manipulation.
- Handler routing uses the existing `SchemaRegistry.FindHandler()` with predicate stripping as a pre-processing step.

## Gotchas

- Plugin-side verify/apply command handling deferred — the Hub routing (this spec) and plugin handling are separate concerns and were split across specs. Functional tests similarly deferred.

## Files

- `internal/component/plugin/hub.go` — Hub struct with `RouteVerify()`, `RouteApply()`, `ProcessConfig()`
- `internal/component/plugin/schema.go` — `stripPredicates()` added to FindHandler path

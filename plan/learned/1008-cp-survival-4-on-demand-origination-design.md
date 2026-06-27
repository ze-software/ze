# 1008 -- On-Demand Route Origination Design

## Context

Operators needed to announce BGP routes on demand (DDoS mitigation, maintenance) without pre-staging static config or hand-writing the raw `peer * update text` grammar. The runtime announce path (`AnnounceNLRIBatch`) already existed but was fire-and-forget with no tracking, no withdraw-by-reference, and no operator visibility into what was announced.

## Decisions

- Generalized the verb to any BGP family (unicast, blackhole sugar, flowspec) over FlowSpec-only, because `AnnounceNLRIBatch` is already family-agnostic and limiting the CLI verb would be artificial
- Chose `tag <key> <value>` as a generic key-value annotation over `track <name>` or `label <name>`, because the codebase already has `Meta map[string]any` plumbed through the plugin API, and `tag.*` prefix in Meta provides cross-path tracking without a new RPC field
- Chose in-memory tag registry wrapping `AnnounceNLRIBatch` over tracking inside the reactor, keeping the reactor stateless and the registry as the only new state
- Used `withdraw tag <key> <value|*>` / `withdraw tag *` / `withdraw id <N>` / `withdraw all` grammar over bare positional args, because every withdraw must be preceded by an explicit keyword for clarity
- Designed watchdog migration path (future spec) where watchdog routes use `Meta: {"tag.watchdog": "<pool>"}`, but did not touch existing watchdog code, YANG, or ExaBGP migration

## Consequences

- The tag registry is the shared code path for CLI, plugin API (`tag.*` meta), and future D2 (flowspec-egress bridge)
- Existing `UpdateRoute` without `tag.*` meta stays fire-and-forget (backward compat)
- `blackhole` is sugar for `unicast ... community blackhole`, not a separate mechanism
- ExaBGP migration code (`internal/exabgp/migration/`), YANG watchdog container, and `watchdog` keyword in update text parser are all untouched
- The `config-static` meta key (`Meta: {"config-static": true}`) is the only existing Meta consumer; `tag.*` prefix avoids any collision

## Gotchas

- YANG `ze:command` must be placed at the parent container level (e.g. `announce`), not on sub-containers (`announce > unicast`), because the dispatcher strips matched tokens. Placing it on sub-containers means the handler never sees the family keyword as `args[0]`. Discovered during review, not during initial implementation.
- `withdrawEntryByID` collided with the existing `withdrawEntry` (timer callback). Renamed the timer version to `withdrawEntryByTimer`.
- `globalRegistry` initialization needed a mutex, not a lazy nil-check. Multiple concurrent handler goroutines could race on first access.
- `List` returns pointers to internal entries; callers must treat them as read-only. Accepted risk for now since all callers are read-only.

## Files

- `plan/spec-cp-survival-4-flowspec-origination.md` (spec, 15 ACs, tag registry design)
- `internal/component/bgp/plugins/cmd/announce/registry.go` (tag registry)
- `internal/component/bgp/plugins/cmd/announce/registry_test.go` (12 registry tests)
- `internal/component/bgp/plugins/cmd/announce/announce.go` (CLI handlers)
- `internal/component/bgp/plugins/cmd/announce/announce_test.go` (25 handler tests)
- `internal/component/bgp/plugins/cmd/announce/require.go` (requireBGPReactor)
- `internal/component/bgp/plugins/cmd/announce/yang/` (YANG schemas + generated glue)
- `internal/component/plugin/all/all.go` (composition root, generated)

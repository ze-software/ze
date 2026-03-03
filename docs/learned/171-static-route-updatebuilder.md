# 171 — Static Route UpdateBuilder

## Objective

Convert static route construction to use UpdateBuilder, and document why the same approach is wrong for RIB routes.

## Decisions

- `buildStaticRouteUpdate` → UpdateBuilder: correct because static routes are locally originated and have no pre-existing wire bytes — UpdateBuilder constructs from scratch.
- `buildGroupedUpdate` deleted: UpdateBuilder used directly by callers; the grouping wrapper was an identity wrapper.
- `buildRIBRouteUpdate` conversion to UpdateBuilder was cancelled: RIB routes are received and already have wire bytes stored via pool handles. Using UpdateBuilder would mean parse-then-re-encode — wasted CPU vs. zero-copy forwarding.

## Patterns

- The split rule: "Does this route have existing wire bytes?" — if yes, zero-copy from pool; if no (locally originated), use UpdateBuilder to construct from scratch.

## Gotchas

- This spec was written before the Pool + Wire architecture was finalised. The correct approach for RIB routes (zero-copy forwarding via pool handles) was determined after this spec was created, making the `buildRIBRouteUpdate` task obsolete rather than failed.

## Files

- `internal/plugin/bgp/reactor/` — `buildStaticRouteUpdateNew()` using UpdateBuilder

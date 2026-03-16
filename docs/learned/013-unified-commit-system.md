# 013 — Unified Commit System

## Objective

Unify config route sending and API route sending under a single `CommitService` abstraction, replacing the duplicate grouping logic that existed in `peer.go` and the unfinished API commit path.

## Decisions

- Single `CommitService.Commit(routes, opts)` method handles both config routes (implicit commit, `SendEOR: true`) and API routes (explicit commit, `SendEOR: false` for `end`, `true` for `eor`).
- Grouping controlled by `rib.group-updates` setting, not by the commit command — the commit decides *when* to flush, the RIB config decides *how* to group.
- `commit <name> end` vs `commit <name> eor` — explicit separation: `end` flushes routes only, `eor` flushes routes + sends EOR for affected families. EOR after API commits is not strict RFC 4724 usage but is commonly accepted for signaling batch completion.
- CommitManager is per-peer (scoped to API context) — not global, because routing decisions are per-peer.
- On peer disconnect: active commits are rolled back automatically.
- Route conflicts: last announce wins; announce + withdraw of same prefix cancels the announce.

## Patterns

- `CommitService.Commit()` path: group by attributes (if group-updates) → build UPDATEs → send → build EOR per family → send EOR if requested → return `CommitStats`.
- Phase 4 (OutgoingRIB transaction cleanup) and Phase 5 (self-check API tests) were deferred as optional.

## Gotchas

- EOR semantics: RFC 4724 defines EOR for graceful restart initial sync only. Sending EOR after every `commit eor` is an extension beyond the RFC — documented and accepted.
- `peer.go:sendInitialRoutes()` previously had its own grouping logic — duplicated code removed when migrated to CommitService.

## Files

- `internal/reactor/peer.go` — sendInitialRoutes() refactored to use CommitService
- `internal/component/plugin/commit.go` — CommitManager, Transaction, handleNamedCommitEnd
- `internal/component/plugin/types.go` — ReactorInterface.SendRoutes()
- `internal/reactor/reactor.go` — SendRoutes() implementation

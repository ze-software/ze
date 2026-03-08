# 375 — Handler Split: Fine-Grained Command Plugins

## Objective

Further split the three Phase 1 command plugins (bgp-cmd-peer, bgp-cmd-ops, bgp-cmd-update) into smaller, feature-aligned plugins. One feature = one folder. Delete bgp-cmd-ops entirely.

## Decisions

- **bgp-cmd-ops deleted, split into 3:** bgp-cmd-cache (3 RPCs), bgp-cmd-commit (3 RPCs), bgp-cmd-raw (1 RPC). Each self-contained with own YANG, tests, require.go.
- **Subscribe/unsubscribe extracted** to bgp-cmd-subscribe (2 RPCs). Not BGP-specific — event subscription is a cross-cutting concern.
- **Meta commands extracted** to bgp-cmd-meta (8 RPCs): help, command-list/help/complete, event-list, plugin-encoding/format/ack.
- **Clear-soft moved into bgp-route-refresh** as `handler/` sub-package. Feature stays in one folder (capability + commands + YANG).
- **SDK+command coexistence via `handler/` sub-package:** bgp-route-refresh is an SDK plugin in `all/all.go`. Its command handlers live in `handler/` sub-package, blank-imported from reactor.go. Avoids import cycle while keeping the feature in one folder.
- **YANG split:** `-conf` suffix for config schemas, `-api` suffix for RPC schemas. Each plugin folder owns both.

## Patterns

- **Mixed SDK+engine plugins:** SDK registration in parent package (safe for `all/all.go`), engine-side RPCs in `handler/` sub-package (blank-imported from entrypoints). The sub-package `doc.go` blank-imports the parent's `schema/` for YANG.
- **Parallel worktree development:** Each new plugin was developed in an isolated git worktree, then merged into main. Changes to shared files (bgp-cmd-peer, reactor.go) required careful reconciliation.
- **Test count adjustments cascade:** When extracting RPCs from a package, the package-scoped `AllBuiltinRPCs()` count changes. bgp-cmd-peer went from 21 → 10 RPCs across the split.

## Gotchas

- **Overlapping modifications:** Multiple agents modified bgp-cmd-peer/doc.go, peer_test.go, and the YANG schema. Required manual reconciliation of all changes.
- **Hidden test files:** summary_test.go (separate from peer_ops_test.go) also contained clear-soft tests that needed removal. Always glob for all `*_test.go` files in the package.
- **Mock reactor bloat:** After extracting handlers, the mock in bgp-cmd-peer still has methods for BGPReactor interface (SoftClearPeer, SendRefresh, etc.) that are no longer exercised by any test in the package. Harmless but technical debt.

## Files

- Created: `bgp-cmd-cache/` (9 files), `bgp-cmd-commit/` (9 files), `bgp-cmd-raw/` (9 files), `bgp-cmd-subscribe/` (5 files), `bgp-cmd-meta/` (7 files), `bgp-route-refresh/handler/` (8 files), `bgp-route-refresh/schema/ze-route-refresh-api.yang`
- Deleted: `bgp-cmd-ops/` (16 files), `bgp-cmd-peer/subscribe.go`, `bgp-cmd-peer/require.go`
- Modified: `bgp-cmd-peer/` (summary.go, peer_test.go, peer_ops_test.go, summary_test.go, doc.go, session.go, YANG), `reactor.go`, `cli/main.go`, `bgp-route-refresh/schema/` (embed.go, register.go), `plugin/server/subscribe.go`

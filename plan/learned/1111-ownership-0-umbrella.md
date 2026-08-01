# 1111 — ownership-0-umbrella (CLOSURE record: DESIGN-REVIEW finding #1)

Umbrella closing DESIGN-REVIEW.md finding #1 ("ownership inversion: the BGP reactor
owns the system's nervous system"). Research (six mapping passes + direct source
verification) found the headline claim **substantially stale**: production ownership
was already correct before this umbrella — the hub constructs the plugin `Server`
(`cmd/ze/hub/main.go:441`), uses it as the global `ze.EventBus` (`main.go:453`), builds
`engine.NewEngine(...)` (`main.go:502`) and owns the server lifecycle; BGP loads as a
config-driven plugin and merely *borrows* hub-owned infra via `SetPluginServerAny`/
`SetEventBusAny` (`internal/component/bgp/plugin/register.go:144,149`), with zero
hub→bgp imports. The review had read the reactor's `api` field but not the wiring — a
textbook caller-vs-producer trap (`ai/rules/no-fabrication.md`).

Only three genuinely-open remnants existed; each was decomposed into an **independent,
non-competing** child and delivered:
- **P1 ownership-1-rs-invariant** (`ff686e4a2`, learned 1063): the reactor RS fast path
  is now activated by the `rs` plugin via the `filterapi` capability seam
  (`EnableRSForwarding`/`RSForwardingEnabled`, `filterapi.go:244/253`), cached once in
  reactor `New`, gated by a single short-circuiting `&& r.rsForwardingEnabled`
  (`reactor_notify.go`); the `rs-fast-path`/`rs-client` YANG leaves moved to
  `plugins/rs/yang/`. Delete the plugin → capability inert → no RS forwarding. Invariant
  restored with zero perf regression.
- **P2 ownership-2-coordinator-types** (`442125776`, learned 1064): Coordinator's
  `any`-typed Configure hooks + `extra map[string]any` bag replaced by typed
  per-protocol interfaces (`registry.go`, typed `BGPBootstrap` in `interfaces.go`); the
  compiler surfaced all 47 implementers. Behavior-preserving; `reactors map[string]any`
  was intentionally left unchanged (P2 does not retype it).
- **P3 ownership-3-reactor-modes** (`39d798e66`, learned 1065): reactor mode is now an
  explicit `Config.Standalone bool` (default = borrow, production-safe) instead of being
  inferred from `r.api != nil`; borrow-without-server errors loudly
  (`errBorrowModeNoServer`). ze-chaos in-process + integration harness + `ze bgp --child`
  migrated to explicit `Standalone: true`.

## DO NOT REVERT (settled decisions)
No hub→bgp import; BGP stays a config-driven plugin (NOT a `ze.Subsystem`); the
EventDispatcher (BGP data path) and EventBus (notifications) stay separate. Standalone
`internal/component/bus/` remains deleted (`learned/324`, `425`). Enforced by
`make ze-tier-check` + `make ze-plugin-boundary-check`.

## GOTCHAS
- **`DESIGN-REVIEW.md` never existed as a tracked repo file** (`git log --all` + `find`
  both empty). The umbrella's success criterion "annotate DESIGN-REVIEW.md finding #1"
  was therefore unachievable as written; the corrected-status record lives in this
  umbrella + the four learned summaries, not in a file annotation. Don't chase the file.
- The children were marked with the **invalid Status `done`** (outside the
  `skeleton|design|ready|in-progress|blocked|deferred` vocabulary), which made
  `spec-closure-check.py --list` under-report them (its high-confidence tier keys on
  `in-progress`). Closure debt can hide behind an invalid status — flip to `in-progress`
  before the two-commit close. Also tracked in `plan/implementation-order.md`.
- The umbrella outlived its delivery: children were implemented+committed 2026-07-04/05
  but the umbrella + children sat unclosed in `plan/` until the 2026-07-13 truth-audit.
  Put umbrella closure on the last child's checklist.

## Files

None recorded.

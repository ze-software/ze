# 1092 -- layout-0-umbrella

## Context

A comparative structure review (2026-07-08, vs holo-routing/holo, gobgp,
bio-rd) found four structural gaps unowned by any spec in a ~1.14M-line,
610-package tree: core-tier imports leaking upward unguarded, the
plugin-boundary checker blind to two plugin namespaces, no shared protocol
skeleton or package-naming glossary, and repo-root clutter plus a stale
architecture claim. This umbrella turned each gap into a sequenced child
(spec-layout-1..4), all closed the same day (learned 1088-1091).

## Decisions

- Complement the in-flight tiers umbrella, never move packages between tiers,
  over one structure mega-effort: duplicate scope produces conflicts and
  contradictory rules.
- Enforce the checkable subset (core import direction, derived scan roots) and
  write the rest as advisory conventions (glossary, skeleton), over enforcing
  everything: the tiers Path B allowlist trap.
- Extend `dep_audit.py` over adding golangci depguard: one enforcement home,
  one import parser.
- Reactor decomposition (the review's biggest finding: one 69-non-test-file
  package) recorded as an unscheduled candidate, then given a destination at
  closure: standalone `plan/spec-reactor-split.md` (skeleton, blocked on
  rib-arch), over folding into rib-arch (no decomposition scope there;
  another session's in-flight work) or renaming `reactor` now (would churn
  154 package clauses + 331 doc anchors twice).

## Consequences

- New upward imports out of `internal/core/` now fail `make ze-verify`; the
  10 grandfathered pairs shrink via fix-route-annotated baseline rows.
- Conventions are discoverable (`ai/rules/INDEX.md` rows) and already consumed:
  the follow-on rename set (`spec-rename-0..3`, retiring `ike/wire`,
  `bgp/message`, `bgp/wireu`) was scoped directly from the glossary/skeleton
  exceptions the children documented.
- The umbrella spawned two follow-on efforts before closing: the rename set
  and reactor-split. An umbrella that closes by handing off destinations, not
  by finishing everything itself, satisfies no-deferral-without-destination.

## Gotchas

- Bare-token grep referrer lists are landmines (A-4): "referrers" of
  `qos-map.md` were the `qos-map` config keyword; following the umbrella as
  written would have corrupted live config parsing. Disambiguate literal path
  vs token BEFORE trusting a referrer list.
- Child 1 closed without a `/ze-review` gate and with assumptions left
  unvalidated; the post-closure review caught it and children 2-4 were held
  to the gate. Umbrella rows tracking child completion must not outrun the
  child's own gate evidence.
- A user decision recorded in a child ("wireu KEPT", child 3) was superseded
  the same day by the rename set. Record decisions where follow-ons can
  supersede them explicitly (the doc.go note pointed forward, so no stale
  authority survived).

## Files

- `plan/learned/1092-layout-0-umbrella.md` (closed; children 1088-1091 carry the code)
- `plan/spec-reactor-split.md` (created: the candidate's destination)
- children's key files: `scripts/dev/dep_audit.py`,
  `scripts/dev/core_import_baseline.txt`,
  `scripts/checks/plugin_process_boundary.go`, `ai/rules/go-standards.md`,
  `ai/rules/protocol.md`, `scripts/dev/protocol_skeleton_report.py`

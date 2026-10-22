# Spec: subscriber-traffic-additional-views

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | New subscriber list, per-session traffic, web and gNMI views extend existing visibility; the broken help promise is separate. |
| Depends | plan/release/fixes/spec-pppoe-subscribers-produce-no-accounting-or-telemetry.md (inherited from the combined spec) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Add the original access-neutral subscriber counters, dedicated per-session traffic form, web views and gNMI subscriber path. Own original AC-1/AC-2, AC-4/AC-5, AC-7/AC-8 and AC-10; AC-6 and AC-9 apply here to each new surface. Reuse the existing baseline-corrected collection and satisfy the original PPPoE collection prerequisite. Do not duplicate the existing `show l2tp session id` help repair.

## Source and Evidence

Split from `plan/release/fixes/spec-subscriber-utilisation-has-no-operator-view.md` under owner decisions M30-M33. The complete original design, decisions, evidence and implementation history are preserved under Historical Combined Specification in `plan/release/fixes/spec-subscriber-utilisation-has-no-operator-view.md`. Read that history before design; no historical claim is freshly verified by this migration. The source lifecycle is retained on the original identity; this new work item begins as a skeleton, not an inferred readiness promotion.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show subscriber` with sessions of both access types live | Each row carries rx bytes, tx bytes, rx packets, tx packets and the uptime, beside the identity fields it prints today |
| AC-2 | `show subscriber` piped through `json`, `yaml` and `table` | Each renders the counters, because the payload is structured data rather than preformatted text |
| AC-4 | The per-session traffic form, given a session id that exists | Only that session's counters are printed |
| AC-5 | The same form given an id that does not exist | An error naming the id, and a non-zero exit code; never an empty success |
| AC-6 | A session whose counters could not be read | The field says so and is distinguishable from zero, on every surface |
| AC-7 | The web session list and a session detail page | Both show the counters, with the list's existing columns intact |
| AC-8 | A gNMI Get on the subscriber path | Returns the same values the CLI prints for the same session |
| AC-9 | The CLI and the RADIUS accounting record for one session, read at the same moment | Report the same baseline-corrected numbers |
| AC-10 | `show subscriber` against a registry holding many sessions | Completes within the same order of time as it does today, measured rather than asserted |

## Work Not Done and Split Ownership

The 2026-10-22 portfolio split changes planning ownership only. Status, existing dependencies and recorded evidence are retained; no implementation, verification or closure is claimed. Original criteria below the historical boundary are provenance, not a second active owner. Historical implementation steps apply only to the owner assigned in this table. Every test, documentation and review obligation follows its behavior owner.

| Work | Implementation owner |
|------|----------------------|
| Original AC-3 and original AC-6/AC-9 restricted to the existing session-id command; former phase 3 | `plan/release/fixes/spec-subscriber-utilisation-has-no-operator-view.md` |
| Original AC-1/AC-2/AC-4/AC-5/AC-7/AC-8/AC-10 and AC-6/AC-9 on added surfaces; former phases 1/2/4/5/6 | `plan/post-release/features/spec-subscriber-traffic-additional-views.md` |

Work assigned to `plan/release/fixes/spec-subscriber-utilisation-has-no-operator-view.md` is not performed or closed by this spec. Shared mechanisms are reused, not independently reimplemented. No feature is declared complete by the release repair or validation disposition.

## Implementation Ownership

Design and implement only the behavior assigned above. Original wiring, functional, interop, documentation and review obligations for that behavior transfer here with the original criteria. Resolve the recorded open design decisions before readiness; the portfolio move itself selects no new product behavior.

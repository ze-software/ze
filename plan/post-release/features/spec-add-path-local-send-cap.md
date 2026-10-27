# Spec: add-path-local-send-cap

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | A local outbound path cap is an absent control; enforcement of the existing receive and peer-advertised contracts remains release work. |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Implement the optional local `limit { send N; }` control, per-family precedence, and the minimum of local and peer-advertised bounds. Reuse the release-owned ranked selection and demotion-withdrawal machinery. Own completion, range checks, payload distinctions, docs and tests for the newly added send leaf, not the receive ceiling or peer-only correction.

## Source and Evidence

Split from spec-add-path-limit-send-receive (closed) under the release-scope and spec-split rules in `ai/META-DECISIONS.md`. The complete original design, decisions, evidence and implementation history are preserved under Historical Combined Specification in spec-add-path-limit-send-receive, closed 2026-10-27: read it from git history (its commit A), and its summary in `plan/learned/040-add-path-limit-send-receive.md`. The election machinery this spec reuses is `pathsLimitBound` and `RecoveryRequest.Limit` (`docs/guide/add-path.md`, "Best paths under the peer's limit"). Read that history before design; no historical claim is freshly verified by this migration. The source lifecycle is retained on the original identity; this new work item begins as a skeleton, not an inferred readiness promotion.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-3 | A peer config carries `family { ipv4/unicast { limit { receive 2; send 4; } } }` and a container `limit { receive 10; }` | ipv4/unicast advertises 2 and caps outbound at 4. Every other advertised family advertises 10 and has no outbound cap |
| AC-4 | A peer advertised PATHS-LIMIT 2 for a family and the local config says `limit { send 4; }` for it | Ze admits at most 2 paths per prefix outbound for that family |
| AC-5 | A peer advertised no PATHS-LIMIT and the local config says `limit { send 4; }` | Ze admits at most 4 paths per prefix outbound for that family |
| AC-14 | `show bgp peer <ip> capabilities` on a session with both a negotiated peer limit and a local send cap | The `paths-limit` payload distinguishes what the peer asked for from what Ze enforces, and the json, yaml and table pipes each render it |
| AC-15 | The editor is asked to complete inside `add-path` and inside `add-path > limit` | The first offers `direction`, `limit`, `family`. The second offers `send` and `receive` |
| AC-16 | `limit { receive 0; }` or `limit { send 0; }` | Refused by the YANG range `1..65535` at load time |
| AC-17 | A prefix carries 5 candidate paths and the effective outbound bound is 2; cover both a local `send` cap and a peer-advertised cap with no local `limit` block | Ze announces the 2 BEST-ranked paths by best-path selection, not the first 2 received (owner decision, 2026-09-11) |
| AC-18 | A prefix is already announcing its best 2 paths under a bound of 2 and a newly received path outranks one of them; cover both cap sources from AC-17 | Ze WITHDRAWS the demoted path and announces the newly best one in its place. A path later demoted back below the bound is not re-announced unless it again becomes best |

## Work Not Done and Split Ownership

The 2026-10-22 portfolio split changes planning ownership only. Status, existing dependencies and recorded evidence are retained; no implementation, verification or closure is claimed. Original criteria below the historical boundary are provenance, not a second active owner. Historical implementation steps apply only to the owner assigned in this table. Every test, documentation and review obligation follows its behavior owner.

| Work | Implementation owner |
|------|----------------------|
| Original AC-1/AC-2/AC-6 to AC-13/AC-19 to AC-21; receive portions of AC-3/AC-15/AC-16; peer-only AC-17/AC-18; common selection and migration | spec-add-path-limit-send-receive (closed) |
| Local-send portions of original AC-3/AC-4/AC-5/AC-14 to AC-18; new send setting and surfaces | `plan/post-release/features/spec-add-path-local-send-cap.md` |

Work assigned to spec-add-path-limit-send-receive (closed) is not performed or closed by this spec. Shared mechanisms are reused, not independently reimplemented. No feature is declared complete by the release repair or validation disposition.

## Implementation Ownership

Design and implement only the behavior assigned above. Original wiring, functional, interop, documentation and review obligations for that behavior transfer here with the original criteria. Resolve the recorded open design decisions before readiness; the portfolio move itself selects no new product behavior.

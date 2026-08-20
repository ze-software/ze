# Spec: harness-fail-open-guard-backlog

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-08-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Hold the nine harness and tooling guards that fail open, cannot fire, or assert
a claim no gate reads. Each one was homed at
`spec-fixit-test-harness-fail-open-guards` while that spec collected this class.
That spec closed on 2026-08-14 having shipped its own four guards, and
`ai/rules/planning.md` requires every row naming it as Destination to be
resolved inside the closing commit. This file is that destination.

**Nothing here is commissioned.** The rows keep Status `deferred` and name this
file so no record dangles. Thomas has not scheduled the work.

**Each row already has a better per-row destination, surveyed 2026-08-14 and
recorded below rather than applied.** Applying them would add Task items to
three live sibling specs and create two more skeletons, which changes those
specs' scope. That is a decision for Thomas, not for a closure. Whoever picks
this up should re-read the survey and route each row rather than treat this file
as the implementation plan.

Rows D and I were re-surveyed on 2026-08-17, when the spec they named closed with
no code change.

| # | The guard | Surveyed destination | Why it was not applied here |
|---|-----------|---------------------|-----------------------------|
| A | `wait_peer_eor_sent`'s docstring (`test/scripts/ze_api.py`) says the `eor-sent` counter is incremented only from `sendInitialRoutes`. `IncrEORSent` (`internal/component/bgp/reactor/peer_stats.go`) has four non-test callers, two in `(*Peer).sendInitialRoutes`, one in `(*reactorAPIAdapter).AnnounceEOR`, one in `reactor_api_batch.go` | `plan/journal/reference-checked-claim-unchecked.md` | The class matches, but no existing row there names this docstring, and `ai/rules/planning.md` bans resolving a row to a record that does not carry the item. The row would have to be written first |
| B | `test/interop-ipsec/lab.py`, `test/interop-l2tp/lab.py` and `test/interop-pppoe/lab.py` each define a `docker_rm` with no strict contract, so a failed pre-clean is swallowed and the scenario runs beside whatever survived | a new `plan/spec-interop-lab-deferred-docker-rm-contract.md` | No spec covers those three labs' teardown contract. `plan/future/spec-fail-open-call-site-drain.md` does not reach it: its population is `docker_exec_quiet`, and the baseline holds no `lab.py` entry <!-- doc-links: ignore (the spec this row proposes, written when somebody takes the work) --> |
| C | `test/interop/scenarios/ospf-sr-frr/check.py` downgrades a missing MPLS label 16100 to `log_info` and passes. A scenario that cannot find the label it exists to prove reports success | `plan/spec-interop-suite-red.md` | That spec owns `test/interop/scenarios/` assertions that pass without asking, and carries live rows for `bgp-srv6-frr`, `bgp-routes-gobgp` and `ospf-gr-frr`. Adding this one expands its scope |
| D | `changed_test_files` (`scripts/dev/audit-test-relaxation.py`) builds its population from `git diff --name-status`. That lists tracked modifications only. A weakening in an UNTRACKED test file is invisible to the audit | a new `plan/spec-fixit-relax-audit-blind-to-untracked-and-python-tests.md`, together with row I | `spec-fixit-relax-audit-reports-the-wrong-token` closed on 2026-08-17 with no code change. The `test-relax:` scanner this row was written against had been retired. The row is restated at its producer and needs a live home <!-- doc-links: ignore (the spec this row proposes, written when somebody takes the work) --> |
| E | `check_hook_names` (`scripts/dev/check_doc_links.py`) cannot see a dead check name in a rule file: `NAME_LINT_FILES` is three files and `ai/rules/planning.md` is not among them | `plan/journal/reference-checked-claim-unchecked.md` | An existing row DOES carry this one. Its Fix cell ends by homing the guard half at `plan/deferrals/rules-as-points.md`, which is the shard the row lives in, so resolving to it is circular until that sentence is repointed |
| F | `c_check_existing_tests` (`.claude/hooks/pretool-writeedit.py`) is a comment plus `return None`, and it is registered in `CHECKS`. The dispatcher invokes it on every Write and Edit and it enforces nothing | a new `plan/spec-hooks-deferred-check-registered-with-an-empty-body.md` | No spec and no journal row covers it. It needs a judgement about the original intent: implement the warning its comment promises, or delete the function and its `CHECKS` entry <!-- doc-links: ignore (the spec this row proposes, written when somebody takes the work) --> |
| G | An authored `title:` field in point frontmatter, so `gate_map --ungated` prints a scannable inventory for the roughly 800 instruction-bearing points no check binds | `plan/spec-rules-situation-index.md` | Same file and same closed schema: `POINT_KEYS` in `scripts/dev/rules_points.py` is a closed tuple and `parse_point` raises on any field outside it, so a fifth key is a format change |
| H | Whether a `Binding` must carry a digest of the point BODY it names, so a reworded point leaves its binding stale until a reader re-affirms it | `plan/spec-rules-situation-index.md`, weakly | That spec re-works how a check is bound to an instruction, but it names neither `Binding` nor a digest. The fallback is its own skeleton. This is a design question that was never put to Thomas |
| I | `is_test_path` (`scripts/dev/audit-test-relaxation.py`) matches `_test.go` and, under `test/`, `.ci` and `.et`. A change set that is almost entirely Python gets ZERO signal from the test-weakening audit | a new `plan/spec-fixit-relax-audit-blind-to-untracked-and-python-tests.md`, together with row D | Same script and the same fail-open direction as D, and the same reason it has no home now: `spec-fixit-relax-audit-reports-the-wrong-token` closed on 2026-08-17 <!-- doc-links: ignore (the spec this row proposes, written when somebody takes the work) --> |

## Provenance

Written 2026-08-14 when `spec-fixit-test-harness-fail-open-guards` closed. Nine
live rows in four shards named that spec as their Destination:
`plan/deferrals/wire-edit-4-api-origin-deferred-bird-interop.md` (A, B),
`plan/deferrals/fixit-ospf-sr-missing-label-passes.md` (C),
`plan/deferrals/fixit-firewall-concurrency-deadlock.md` (D) and
`plan/deferrals/rules-as-points.md` (E, F, G, H, I). None was an acceptance
criterion of that spec. Each was parked there because it collected this class,
and the class outlived the four guards that closed it.

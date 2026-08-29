# Spec: fail-open-call-site-drain

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

Drive the 171 test-harness call sites that read a fail-open return value without
testing it down to zero.

`docker_exec_quiet` (`test/interop/interop.py` (retired; now `internal/le/interoplab/bgp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) -->) answers `""` on ANY non-zero
exit. A caller that reads that answer and never tests it for emptiness turns a
FAILED command into a passing assertion over nothing. `"DIS" in ""` is False, so
the scenario reports a green it never measured.

**The risk is already capped, which is why this is separable.**
`internal/le/doc/wiring/delegate.go` derives the fail-open set to a fixpoint and
refuses the next new call site. The floor in `test/health/docker-exec-baseline.json` (retired, no successor) <!-- doc-links: ignore (deleted 2026-08-28 by eae282592 with no replacement) -->
goes DOWN only, so the count cannot grow. `./le functional docker-exec-check` is the
gate, and `TestRepoRatchet` re-runs it under `./le test-unit`.

What remains is mechanical: 171 sites across 67 files. Each one either gets its
return value tested, or an opt-out `# fail-open-ok: <reason>` naming why the
empty answer is correct there. A bare marker with no reason does not count.

**Why it is not one change with the guard.** Most of these sites sit in interop
scenarios that cannot run without Docker, so a batch edit cannot be proven by
running it. The work wants its own passes, each lowering the floor by what that
pass actually verified.

## Provenance

Homed here 2026-08-14 when `spec-fixit-test-harness-fail-open-guards` closed.
It was the one row of that spec's deferral shard with no destination: the guard
(its guard 4) is what refuses the next site, and draining the existing ones was
never in its scope. Thomas has not commissioned this; it is written down so the
row has a home rather than to schedule the work.

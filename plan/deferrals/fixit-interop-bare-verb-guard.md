# Deferrals: fixit-interop-bare-verb-guard

One issue, recorded not fixed (owner instruction, 2026-08-08). The aggregate
live backlog is folded on read from `plan/deferrals/` by `/ze-status`. Nothing
stores it (`ai/rules/planning.md`).

**Issue:** no mechanical guard against the bare `ze <verb>` form in interop scenarios

**Closed 2026-08-14.** Thomas ruled for the wider guard: the lint requires every
fail-open return value to be checked, rather than banning the bare form.
`scripts/dev/docker_exec_checked.py` derives the fail-open set to a fixpoint and
classifies every call site, `make ze-docker-exec-check` runs it, and the floor in
`test/health/docker-exec-baseline.json` goes DOWN only. This file is residue,
and the closure of `spec-fixit-test-harness-fail-open-guards` removes it in the
commit after the one that files this terminal status: `deferral_shard_removal_problems`
(`scripts/dev/commit_helper.py`) reads the shard at HEAD.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-08 | ad-hoc (found while repairing `ze show host`, which answered `unknown command` with the daemon up) | No mechanical guard stops a new interop scenario calling the bare `ze <verb>` form inside a container. `readCredentials` (`internal/core/ssh/client/client.go`) answers `no credentials`, `docker_exec_quiet` (`test/interop/interop.py`) turns any non-zero exit into `""`, and the assertion over that string then passes over nothing. Nineteen instances were fixed by hand on 2026-08-07 and 80 call sites read the swallowed return value | Separable from the `ze show host` repair: that command now answers with the daemon up and with it down, proven by `test/ui/cli-verb-daemon-dispatch.ci`, and the goal holds without this guard. The guard is a new `scripts/dev/` lint plus a sibling `_test.py`, a make target, verify-path routing and rules/docs wiring, which is its own package. It also needs a ruling first: ban the bare form, or require every `docker_exec_quiet` return value to be checked, or drop that helper's fail-open contract outright | `spec-fixit-test-harness-fail-open-guards` (recorded there as Guard 4) | done |

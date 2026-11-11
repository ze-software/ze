# Spec: stress-children-die-with-harness

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P2 |
| Release reason | A stress harness killed outright leaves ze, BIRD, test peers and their network namespaces running as root, holding ports and addresses that redden later stress and functional runs, the evidence the release depends on |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Symptom (read at the producer, 2026-11-11).** The stress harness
(`./le test integration stress`, `internal/le/test/integration/`) starts every
long-lived child through one function, `realStressBirdSystem.Start`
(`stressbird.go`): `le test peer --mode sink` and `--mode inject`, ze, BIRD,
tcpdump and curl, each wrapped as `ip netns exec <ns> <binary>`. That function
calls `exec.CommandContext` and a plain `cmd.Start()`. It sets no process
group and no parent-death signal, and its cancel, `Terminate` and `Kill`
signal the direct pid only.

**Consequence.** A harness that dies without running Go code (SIGKILL, the OOM
killer, a torn-down session) leaves every child reparented to init, plus its
`ze-stress-ze-<pid>` and `ze-stress-bb-<pid>` namespaces and their veths. The
next run cannot sweep them: its opening cleanup deletes only names carrying
its own PID. The BIRD gate is worse: `runStressBirdAction` runs under
`context.Background()`, so a SIGTERM sent to the harness pid runs no teardown
at all.

**Sibling already fixed.** The `.ci` runner had the same gap and was fixed by
`spec-test-children-die-with-runner` (commit c195efcf1b): its children fork
with `plugin.NewSysProcAttr` (own group, SIGKILL on parent death) through
`plugin.StartChild`. `docs/architecture/testing/runner-architecture.md`,
"Three things end a process", states the rule every runner owes.

**Goal.** Every process the stress harness starts dies when the harness dies,
however it dies, and a SIGINT or SIGTERM to the harness runs its teardown on
every gate.

**Fix direction, to be confirmed in research.** In `realStressBirdSystem.Start`,
`plugin.KillGroupOnCancel` and `plugin.StartChild`; group signals in
`realStressBirdProcess.Terminate` and `Kill`; the BIRD gate rooted on the
package's `signalContext` like the other native gates. Research must confirm
from iproute2's source that `ip netns exec` execs its target without
forking, so the death signal reaches the target. A sweep of leftover
`ze-stress-*` namespaces whose PID is dead is a separate question for design.

**Proof constraint.** The stress harness runs as root under sudo, one root run
at a time, alone on the host (`ai/META-DECISIONS.md`). The regression test
should drive `realStressBirdSystem.Start` the way the runner's
`TestRunnerKilledOutrightTakesItsChildren` drives `startWithETXTBSYRetry`, with
no root, so the proof does not wait for a quiet host.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/runner-architecture.md` - "Three things end a process", the rule every runner owes
  → Constraint: a child's death signal is bound to the forking THREAD, so a fork with `Pdeathsig` goes through `plugin.StartChild`
- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md`, `ai/doors/plugin-infra.md` - the doors this work crosses
  → Constraint: `internal/le` may import `internal/component/plugin`; product code never imports `internal/le`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/test/integration/stressbird.go` - `realStressBirdSystem.Start` (plain `cmd.Start()`, no `SysProcAttr`), `realStressBirdProcess.Terminate`/`Kill` (direct pid only), `runStressBirdAction` (`context.Background()`)

**Behavior to preserve:**
- [to research]

**Behavior to change:**
- Every stress child gets its own group and SIGKILL on harness death; every gate runs teardown on SIGINT/SIGTERM

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters: wire bytes, API command, config, plugin message]
- [Format at entry]

### Transformation Path
1. [to research]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Harness ↔ child process | fork/exec through `ip netns exec` | No |

### Integration Points
- `plugin.KillGroupOnCancel`, `plugin.StartChild` - [to research]

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A harness process that started a stress child is SIGKILLed | → | `realStressBirdSystem.Start` | [test name, modelled on `TestRunnerKilledOutrightTakesItsChildren`] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [to design] | `internal/le/test/integration/` | A child started by `realStressBirdSystem.Start` dies with a SIGKILLed harness, without root | |

## Files to Modify
- `internal/le/test/integration/stressbird.go` - `realStressBirdSystem.Start`, `realStressBirdProcess.Terminate`/`Kill`, `runStressBirdAction`

## Implementation Steps

1. **Phase: Wiring and red test** -- [to design]
2. **Phase: Fork and stop through the plugin mechanism** -- [to design]

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs: `./le test unit pkg path ./internal/le/test/integration/`

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| [to research] | `internal/component/plugin/sysproc.go` `KillGroupOnCancel`; `sysproc_linux.go` `NewSysProcAttr`, `StartChild`; `internal/test/runner/runner_exec_util.go` `startWithETXTBSYRetry` | [to research] | [to research] |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `guard-added-to-one-half-of-a-pair` | 2026-11-11 | stress harness `realStressBirdSystem.Start`: children with no group and no death signal, BIRD gate with no signal root |

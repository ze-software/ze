# spec-fixit-reject-fence-observability

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-fixit-migrate-sleeps-infra (infra-gated carve-out) |
| Phase | 0/1 |
| Updated | 2026-07-14 |

## Task

Add a plugin-visible "operation processed" observability signal so tests that prove a
REJECTION or NO-OP can wait deterministically instead of sleeping. Three infra-gated
tests under spec-fixit-migrate-sleeps-infra keep a `time.sleep` for exactly this reason: the
only evidence the operation ran is a relayed-stderr line the runner checks, with no
signal a pollable observer can see.

- `test/reload/reload-listener-rejected.ci` (`sleep(0.5)`): a SIGHUP reload that rejects
  a listener rebind. The `reload.done` marker is written by trigger.sh right after
  `kill -HUP`, before processing; a rejected reload changes no observable state.
- `test/plugin/as112-external-refuses.ci`, `test/plugin/cos-external-warns.ci`
  (`sleep(4.0)` in a standalone `wait.py`): an external plugin subprocess refuses/warns
  at startup; the only evidence is the daemon's relayed stderr.

Correctness of all three is ALREADY guaranteed timing-independently by their runner-side
`expect=stderr:contains=` checks. This spec removes the SECONDARY observer-side sleep by
adding a real edge to poll. Scope is observability only; no behavior change.

## Required Reading

- `internal/component/plugin/server/` (config reload / Subsystem.Reload path)
- `internal/plugins/l2tp/` (listener reload rejection WARN: "listener endpoint change ignored")
- `internal/component/plugin/` (external plugin subprocess lifecycle + stderr relay)
- `test/scripts/ze_api.py` (observer wait primitives: `wait_until`, `dispatch_until`)
- `ai/rules/doctor-checks.md`, `ai/rules/discovery-updates.md`, `ai/rules/no-fabrication.md`

## Current Behavior

Source files read during investigation:
- [ ] `test/reload/reload-listener-rejected.ci`
- [ ] `test/plugin/as112-external-refuses.ci`
- [ ] `test/plugin/cos-external-warns.ci`
- [ ] `test/scripts/ze_api.py`

Behavior to preserve: the runner-side `expect=stderr:contains=` proofs stay the primary
correctness gate. Reload still rejects a listener rebind and keeps the original bind.
External plugins still refuse/warn as they do today. No new field may change reject/no-op
semantics; the signal is purely observational.

## Data Flow

### Entry Point
An operator/test triggers a reject/no-op: a SIGHUP reload with a rejected listener change,
or configuring an external plugin that refuses/warns at startup. An observer plugin wants
to know the operation was PROCESSED.

### Transformation Path
Today the daemon logs the rejection/warning to stderr (relayed at WARN+) and, for reload,
returns from `Subsystem.Reload` without changing the bound endpoint. No counter or event
is surfaced to plugins. Proposed: emit a monotonic "reloads-processed" generation counter
(queryable via a `show` command) and/or a plugin-subscribable "reload-processed" /
"external-plugin-exited" event carrying an outcome (applied/rejected). The observer polls
the counter (or awaits the event) as the fence, then asserts state.

### Boundaries Crossed

| From | To | Shared point |
|------|----|--------------|
| SIGHUP reload handler | plugin-visible counter/event | reload generation counter surfaced via `show` / event bus |
| External plugin subprocess exit/log | plugin-visible event | plugin-engine subprocess lifecycle -> event bus |
| Observer plugin (`ze_api`) | the new signal | `dispatch_until` on the counter, or `wait_for_event` on the event |

### Integration Points
- Config-reload path (`Subsystem.Reload`) increments a generation counter regardless of
  applied/rejected outcome.
- A `show` command (e.g. `show config reload-status`) exposes the counter as JSON.
- Optional: an event type in the plugin `Registration.EventTypes` for reload-processed /
  external-plugin-exited.

## Wiring Test

| Entry Point | Feature Code | Test |
|-------------|--------------|------|
| SIGHUP reload processed (applied or rejected) | -> reload generation counter surfaced via a `show` command / event | `reload-listener-rejected.ci` observer polls the counter instead of `sleep(0.5)`; FAILS if the counter never appears, PASSES once reload is processed |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| Reload increments the generation counter on a rejected listener change | `internal/component/plugin/server/*_test.go` | the counter advances even when the reload rejects (no-op outcome still observable) |
| `show config reload-status` returns the counter as JSON | plugin cmd `*_test.go` | the signal is queryable by an observer |

### Functional Tests

| Test | Validates |
|------|-----------|
| `reload-listener-rejected.ci` converted to poll the reload counter | deterministic wait replaces `sleep(0.5)`; passes 3x; QEMU-gated for the l2tp listener bind, so verified via `make ze-qemu-needs-linux-test` |

## Files to Modify

- `internal/component/plugin/server/` (reload path: increment generation counter).
- A plugin `show` command package (expose `reload-status` / counter as JSON) + YANG if needed.
- `internal/core/diagnostic/codes.go` only if a doctor check is warranted (likely not).
- `test/reload/reload-listener-rejected.ci`, `test/plugin/as112-external-refuses.ci`,
  `test/plugin/cos-external-warns.ci` (convert off `time.sleep`).
- `test/.ci-sleep-baseline` (ratchet down).
- Docs: `docs/architecture/api/commands.md` (new `show`), `ai/INDEX.md` (discovery).

## Implementation Steps

1. Decide the minimal signal: a reload-generation counter via `show` is smaller than a
   new event type; prefer it unless the external-plugin-exit cases need an event.
2. Increment the counter in `Subsystem.Reload` for BOTH applied and rejected outcomes.
3. Surface it via a `show` command returning JSON; add YANG/completion per
   `ai/patterns/config-option.md` and `ai/rules/cli-patterns.md`.
4. Convert the 3 tests to poll the counter (`dispatch_until`) / await the event; keep the
   runner-side `expect=stderr:contains=` proofs. Verify (reload/as112/cos are QEMU-gated
   where they pull linux deps; verify via `make ze-qemu-needs-linux-test`).
5. Ratchet `test/.ci-sleep-baseline`.

## Acceptance Criteria

- AC-1: a reload (applied OR rejected) advances a plugin-queryable generation counter.
- AC-2: an observer can deterministically wait for "reload processed" without a sleep.
- AC-3: the 3 infra-gated tests convert off `time.sleep`, keeping their stderr proofs,
  verified in the appropriate environment; baseline ratcheted.
- AC-4: no change to reject/no-op semantics; the runner-side stderr checks still pass.

## Risks & Assumptions

- A-1 (unvalidated): a reload-generation counter is sufficient for the reload case and the
  external-plugin cases need a separate exit event. Validate by prototyping the counter first.
- R-1: adding a `show` surface pulls YANG/completion/docs obligations (discovery-updates).
- R-2: as of 2026-07-14 a concurrent session's `internal/component/iface` break prevents
  `make ze`; this spec cannot be implemented until the tree builds. The l2tp reload test
  is also QEMU-gated, so final verification needs a Linux env.

## Checklist

- [ ] Reload increments a plugin-queryable generation counter (applied + rejected) (AC-1)
- [ ] Observer waits deterministically for "reload processed" (AC-2)
- [ ] 3 infra-gated tests converted, stderr proofs kept, baseline ratcheted (AC-3)
- [ ] Reject/no-op semantics unchanged (AC-4)
- [ ] Tests written (counter unit test + `show` test + converted functional tests)
- [ ] Tests FAIL before the signal exists (observer has nothing to poll)
- [ ] Tests PASS after the signal is added
- [ ] `make ze-test` green (host subset) + `make ze-qemu-needs-linux-test` for l2tp reload
- [ ] Review Gate: `/ze-review` clean (0 BLOCKER, 0 ISSUE)

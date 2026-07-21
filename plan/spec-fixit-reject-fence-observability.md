# spec-fixit-reject-fence-observability

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-fixit-migrate-sleeps-infra (infra-gated carve-out) |
| Phase | 1/1 (was 1/2; Phase 2 left this spec 2026-07-16 by Thomas's ruling -- see "Needs a decision -- RESOLVED") |
| Updated | 2026-07-16 |

## Task

Add a plugin-visible "operation processed" observability signal so tests that prove a
REJECTION or NO-OP can wait deterministically instead of sleeping. Three infra-gated
tests under spec-fixit-migrate-sleeps-infra keep a `time.sleep` for exactly this reason: the
only evidence the operation ran is a relayed-stderr line the runner checks, with no
signal a pollable observer can see.

- `test/plugin/reload-listener-rejected.ci` (`sleep(0.5)`): a SIGHUP reload that rejects
  a listener rebind. The `reload.done` marker is written by trigger.sh right after
  `kill -HUP`, before processing; a rejected reload changes no observable state.
  (Path corrected: the spec originally said `test/reload/...`. See R-3.)
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
- [ ] `test/plugin/reload-listener-rejected.ci`
  -> Constraint: the observer dispatches commands by STRING through
  `ze-plugin-engine:dispatch-command`, so any new fence command must be declared in the
  YANG command tree to be resolvable; a registered handler alone is not reachable.
- [ ] `test/plugin/as112-external-refuses.ci`
  -> Decision: not convertible by the counter. No reload occurs, and `wait.py` is a bare
  `cmd=foreground` process with no `ze_api` connection, so the test has no observer that
  could poll anything. Phase 2. See A-1.
- [ ] `test/plugin/cos-external-warns.ci`
  -> Decision: same as as112. Phase 2. See A-1.
- [ ] `test/scripts/ze_api.py`
  -> Decision: `dispatch_until(command, predicate, attempts, delay)` is the fence
  primitive; its internal `time.sleep` is exempt from the ci-sleep ratchet, which counts
  only `test/**/*.ci`. It returns the LAST result when attempts are exhausted rather than
  raising, so the caller MUST re-check the predicate and `runtime_fail` explicitly, else
  a never-advancing fence would pass silently.
- [ ] `cmd/ze/hub/main_reload.go`
  -> Constraint: `doReload` sequences `s.ReloadConfig` (:151) BEFORE `eng.Reload` (:163),
  which is what drives the l2tp reject. The fence must be marked around the whole of this
  function, not inside the plugin server. This is the keystone fact behind A-1.
- [ ] `internal/component/l2tp/subsystem_reload.go`
  -> Constraint: `Subsystem.Reload` returns nil even when it rejects a knob (rejections
  are WARN-only, counted into a local `rejected` tally that is never returned). So a
  rejected listener rebind surfaces to `doReload` as a SUCCESSFUL reload, and
  `last-outcome` is correctly "applied": the reload applied, l2tp ignored one knob.
  "Rejected" in AC-1 means the reload transaction failed, not that a knob was ignored.

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
| `test/plugin/reload-listener-rejected.ci` converted to poll the reload counter | deterministic wait replaces `sleep(0.5)`. Verified PASS 3x on the darwin host (`bin/ze-test bgp plugin 360`), NOT QEMU-gated (R-4). Fence proven load-bearing by mutation: with the `MarkReloadProcessed` call disabled the observer polls 10s and fails `reload generation never advanced past 0 after SIGHUP` while the l2tp reject WARN still fires -- which also confirms the increment-site ordering in A-1. |

## Files to Modify

Phase 1 (reload counter, this phase):

- `internal/component/plugin/server/reload_generation.go` (NEW): the counter state +
  `MarkReloadProcessed` / `ReloadStatus` on `Server`.
- `internal/component/plugin/server/server.go`: one `reloadGen` field on the `Server` struct.
- `cmd/ze/hub/main_reload.go`: increment at the END of `doReload`, after `eng.Reload`
  (see A-1). This is the correction to the original "increment in `Subsystem.Reload`".
- `internal/component/cmd/show/reload_status.go` (NEW): `show reload-status` handler.
  Stays CENTRAL: the counter is process-global daemon state with no removable owner, the
  same class as `show warnings` / `show health` (`ai/rules/plugin-self-containment.md`).
  NOT `show config reload-status`: that subtree is owned by `internal/plugins/config-cli`,
  and putting a centrally-handled command in a plugin's subtree inverts the removal test.
- `internal/component/cmd/show/yang/ze-cli-show-cmd.yang`: `container reload-status`
  (drives dispatch + completion).
- `test/plugin/reload-listener-rejected.ci` (convert off `time.sleep`).
- `test/.ci-sleep-baseline` (ratchet 463 -> 462).
- Docs: `docs/architecture/api/commands.md` (new `show`), `ai/INDEX.md` (discovery).
- `internal/core/diagnostic/codes.go` only if a doctor check is warranted (it is not: the
  counter is an observability read, not a runtime readiness condition).

Phase 2 (external-plugin exit signal, NOT this phase -- see A-1):

- `test/plugin/as112-external-refuses.ci`, `test/plugin/cos-external-warns.ci`: each needs
  an observer plugin added AND an external-plugin-exit signal to poll. Surface overlaps
  `spec-fixit-plugin-event-subscription`; design there or in a successor spec first.

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

## Implementation Status (2026-07-16)

Phase 1 is code-complete and verified on the host. Phase 2 was blocked on a decision from
Thomas; **that decision was made 2026-07-16 and Phase 2 has left this spec** (see "Needs a
decision -- RESOLVED" below). This spec is now Phase-1-complete and closable.

| AC | State | Proof |
|----|-------|-------|
| AC-1 | Met | `TestReloadStatusAdvancesOnRejectedReload` (rejected case advances the counter), `TestHandleShowReloadStatusReportsRejectedReload` (it is queryable). Both pass under `-race`. |
| AC-2 | Met | `reload-listener-rejected.ci` fences on the counter, PASS 3x. Mutation-proven: disabling `MarkReloadProcessed` makes it fail `reload generation never advanced past 0`. |
| AC-3a | Met | 1 test converted; baseline ratcheted 463 -> 132 (see note). |
| AC-3b | **Moved out** (2026-07-16) | No longer this spec's work. Homed in `plan/spec-fixit-reject-fence-observability-deferred-external-plugin-signals.md` by Thomas's ruling. Was: blocked, needing an external-plugin signal AND an observer plugin in each test; not deliverable by the counter (A-1). |
| AC-3 (whole) | **Retired** (2026-07-16) | AC-3 bundled AC-3a and AC-3b. AC-3a is Met; AC-3b left scope. AC-3 as written is retired rather than met -- recording it as "Met" would claim two test conversions that did not happen. |
| AC-4 | Met | Zero diff in `internal/component/l2tp/`, `plugin/server/reload.go`, `reload_tx.go`. The `expect=stderr:contains=l2tp reload: listener endpoint change ignored` proof is unchanged and still passes. The counter is write-only state; nothing reads it to decide anything. |

Baseline note: the spec predicted 463 -> 462, assuming the committed baseline equalled the
actual count. It did not. HEAD contained 133 `time.sleep(` calls in `test/**/*.ci` against a
committed baseline of 463: the ratchet only PRINTS advice when the count drops and never
fails, so the baseline had drifted far above reality and was enforcing nothing. This change
takes the count to 132 and sets the baseline to the true value, which is what the gate
instructs on a drop. The ratchet is now actually tight.

## Needs a decision -- RESOLVED 2026-07-16

Phase 2 (`as112-external-refuses.ci`, `cos-external-warns.ci`) cannot proceed without a
call on WHERE the external-plugin-exit signal lives. Its surface (external plugin
subprocess lifecycle) overlaps `spec-fixit-plugin-event-subscription`, which was being
designed concurrently. Options: fold AC-3b into that spec, or spec it separately once that
one lands. Do NOT design it inside this spec without deconflicting first.

-> **RULED (Thomas, 2026-07-16): spec it separately, once `spec-fixit-plugin-event-subscription`
lands.** Not folded into that spec: it is at `design` and owns its own two gaps (namespace-locked
startup subscriptions, and the second gap in its Task section); loading the exit-signal surface
into it widens a spec that is not yet built and couples two things that fail independently.

Consequences, all applied here:
- **AC-3b leaves this spec's scope.** It is homed in
  `plan/spec-fixit-reject-fence-observability-deferred-external-plugin-signals.md`
  (`Status | skeleton`, `Depends | spec-fixit-plugin-event-subscription`), created
  2026-07-16 under the "no deferral without a destination spec" rule
  (`ai/rules/planning.md`). Recorded in `plan/deferrals.md` and in Deviations below.
- **"The external-plugin-exit signal" is a misnomer, and the destination spec renames it.**
  Re-reading the two tests while homing them: `cos-external-warns.ci:74-76` says cos WARNS
  AND KEEPS RUNNING, so an exit event can never fence it -- only `as112-external-refuses.ci`
  exits (`:85`). This spec has been asking for one signal where the tests need two (an exit
  signal, and a startup-reached/warn-emitted signal). Hence
  `-deferred-external-plugin-signals`, plural. Anyone who had designed "the exit event" this
  spec asked for would have found this at implementation time.
- **This spec closes at Phase 1**, whose AC-1/AC-2/AC-3a/AC-4 are Met and verified on the
  host (see the Implementation Status table). AC-3 as originally written is retired, not
  met: it bundled AC-3a and AC-3b, and AC-3b is no longer this spec's work.
- The deconfliction the old text demanded has now happened, so the "do NOT design it inside
  this spec" instruction is discharged rather than outstanding.

## Acceptance Criteria

- AC-1: a reload (applied OR rejected) advances a plugin-queryable generation counter.
- AC-2: an observer can deterministically wait for "reload processed" without a sleep.
- AC-3: ~~the 3 infra-gated tests convert off `time.sleep`~~ **RETIRED 2026-07-16.** It
  bundled two items with different fates; scored as a whole it can only be a lie in one
  direction or the other. Superseded by AC-3a alone.
  - AC-3a (Phase 1): `reload-listener-rejected.ci` converts; baseline 463 -> 462. **Met**
    (actual ratchet 463 -> 132; see the Implementation Status baseline note).
  - AC-3b (Phase 2): ~~`as112-external-refuses.ci` + `cos-external-warns.ci` convert.~~
    **MOVED OUT 2026-07-16** to
    `plan/spec-fixit-reject-fence-observability-deferred-external-plugin-signals.md`
    per Thomas's ruling. Not this spec's work; not scored here.
- AC-4: no change to reject/no-op semantics; the runner-side stderr checks still pass.

## Risks & Assumptions

- A-1 (VALIDATED 2026-07-16, both halves confirmed; the counter design is adopted and the
  external-plugin cases are split out to Phase 2). Evidence:
  - Reload half CONFIRMED, with a correction to the increment site. The counter IS
    sufficient for the reload case, but ONLY when incremented at the end of the whole
    reload sequence in `doReload` (`cmd/ze/hub/main_reload.go`), NOT inside
    `Server.reloadConfig` as "Files to Modify" originally said. The rejection WARN is
    produced by `l2tp.Subsystem.Reload` (`internal/component/l2tp/subsystem_reload.go:75`),
    which is reached via `eng.Reload` at `cmd/ze/hub/main_reload.go:163` -- AFTER
    `s.ReloadConfig` at `cmd/ze/hub/main_reload.go:151`. A counter incremented inside
    `reloadConfig` would advance BEFORE l2tp had processed the change, so an observer
    polling it could read the listener port before the reject even ran. The test would
    still pass (the port never changes) but only vacuously: that is the "absence of X
    proves Y" invalid-test red flag from `ai/rules/tdd.md`. Increment site is therefore
    `doReload`, which is the only function that knows every reload step has completed.
  - External-plugin half CONFIRMED as "the counter does not help", but the spec's proposed
    remedy ("need a separate exit event") is NOT yet established as the right one. Neither
    `as112-external-refuses.ci` nor `cos-external-warns.ci` performs a reload at all: each
    has a single config, one background `ze -`, and a `wait.py` that only holds the test
    open. The refuse/warn fires during plugin STARTUP, so no reload-generation counter can
    ever fence them. Worse, neither test has an observer plugin: `wait.py` runs as a bare
    `cmd=foreground` process with no `ze_api` connection and no plugin socket, so there is
    nothing in either test that could poll a signal even if one existed. Converting them
    needs BOTH an external-plugin-exit signal AND an observer plugin added to each test.
    That is a separate, larger piece of work whose surface (external plugin subprocess
    lifecycle) overlaps `spec-fixit-plugin-event-subscription`; it is carved out as Phase 2
    and must not be designed here.
  - Consequence for AC-3: Phase 1 converts 1 of the 3 tests. AC-3 is NOT met by Phase 1.
- R-1: adding a `show` surface pulls YANG/completion/docs obligations (discovery-updates).
- R-2 (RESOLVED 2026-07-16): the `internal/component/iface` break that blocked this spec on
  2026-07-14 was fixed by commit `1b8e44053`; `make ze` exits 0 and the tree builds.
- R-4 (DISPROVEN 2026-07-16): the spec asserted throughout that the l2tp reload test is
  QEMU-gated and needs `make ze-qemu-needs-linux-test`. It is not. The test carries no
  `option=needs-linux`, and `ze-qemu-needs-linux-test` runs ONLY tests that do
  (`mk/test-integration.mk:251`, `ZE_QEMU_LINUX_ONLY=1`), so that target would never have
  run it. `test/plugin/reload-listener-rejected.ci` sets
  `option=env:var=ze.l2tp.skip-kernel-probe:value=true` precisely so it runs without the
  kernel L2TP module: verified PASS 3x on the darwin host (`bin/ze-test bgp plugin 360`,
  ~3.0s each). Phase 1 is therefore fully verifiable on the host and needed no Linux env.
- R-3 (found during implementation): the spec's file paths were wrong in two places. The
  reload test is `test/plugin/reload-listener-rejected.ci`, not `test/reload/...`. And
  `Subsystem.Reload` lives in `internal/component/l2tp/`, not in
  `internal/component/plugin/server/`; the plugin-server reload entry point is
  `Server.reloadConfig` (`internal/component/plugin/server/reload.go:160`). Both corrected.

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

## Review Gate

### Run 1 (closure — independent adversarial review, 2026-07-21)

Independent subagent review of the Phase-1 changeset (commit `a7815341e`:
`reload_generation.go`, `server.go`, `main_reload.go`, `reload_status.go`,
`ze-cli-show-cmd.yang`, `reload-listener-rejected.ci`) against AC-1/AC-2/AC-4.

| Severity | Finding | Location | Action |
|----------|---------|----------|--------|
| NOTE | `generation_of` catches `JSONDecodeError` but a non-dict `data` would raise an uncaught `AttributeError`; harmless because `handleShowReloadStatus` always returns a `plugin.Map` | `test/plugin/reload-listener-rejected.ci:44-54` | Purely defensive; not a defect |

**Verdict: CLEAN — 0 BLOCKER, 0 ISSUE, 1 NOTE.** ACs supported by producing code:
AC-1 `reload_generation.go:44-54` (`mark` increments on both applied/rejected branches),
driven from `main_reload.go:93-94` (`MarkReloadProcessed(err == nil)`); increment ordering
correct (the l2tp reject WARN at `l2tp/subsystem_reload.go:75` via `eng.Reload` returns
before `doReload` marks, so the fence advances strictly AFTER the rejection). Race-free
(`sync.Mutex` snapshot; `-race` `TestReloadStatusConcurrent` passes). AC-2 `reload_status.go:40-63`
wired via `RegisterRPCs("ze-show:reload-status")` + YANG `container reload-status`; the `.ci`
fences on `dispatch_until` and hard-fails "reload generation never advanced" (mutation-proven
fence, original `expect=stderr:contains=` proof retained). AC-4 the only reader is the display
handler; nothing branches on the counter to decide reload behavior (write-only observational).
Artifact: `tmp/review/fixit-reject-fence-observability-<session>.md` (verdict clean).

AC-3 is honestly declared unmet (AC-3a done; AC-3b moved out to
`spec-fixit-reject-fence-observability-deferred-external-plugin-signals`, learned 1171); the
spec closes at Phase 1 with AC-1/AC-2/AC-3a/AC-4.

Gate satisfied: last run 0 BLOCKER, 0 ISSUE.

# Spec: bound the VPP reply wait in iface, fib and static, and report a wedged dataplane

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/fixit-firewall-concurrency-deadlock.md` |
| Handoff | verify |
| Updated | 2026-08-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`newGovppOps` (`internal/plugins/firewall/vpp/timeout_linux.go`) is the only
production `SetReplyTimeout` call in the tree. Every other plugin that takes a
govpp channel off the shared connector leaves the deadline at
`core.DefaultReplyTimeout`, which is `time.Duration(0)`
(`vendor/go.fd.io/govpp/core/connection.go`).
`(*Channel).receiveReplyInternal` (`vendor/go.fd.io/govpp/core/channel.go`)
reads a non-positive timeout as `maxInt64`, and `ReceiveReply` has no context
arm, so the caller's context cannot break the wait. A VPP that accepts a request
and goes silent blocks the caller for about 292 years, holding whatever lock the
caller took.

Three plugins are affected, each with a different blast radius, and none of them
reports a dataplane failure the way the firewall does.

The owner's requirement, given 2026-08-11: the interface path gets a deadline,
the error is reported to the operator, and the operator-visible outcome on a
timeout matches the firewall's, rollback included.

## Open Question for the Owner

The firewall does not have ONE outcome to match. It has two, and they disagree.

- **The config reload path rolls back.** The undo function recorded in the
  `sdk.Journal` by the firewall engine (`internal/component/firewall/engine.go`)
  re-registers the previous tables and calls `ApplyAll` again. On a timeout that
  second apply goes through the same silent dataplane and burns a second full
  deadline before failing the same way. The operator waits twice as long to be
  told it did not work.
- **The DDoS responder skips its rollback**, on exactly this sentinel
  (`internal/plugins/ddos/local/responder.go`). Its comment states the reason: a
  registry rollback has already made the desired state correct, and a second
  apply can only burn another deadline while an attack stays unmitigated.

- **Way 1 (recommended):** on a TIMEOUT specifically, do not roll back. Fail
  fast, say the dataplane is wedged and its state is now behind, and keep
  rollback for ordinary rejections where the dataplane is answering. This makes
  the firewall config path change too, which is wider than the owner asked for.
- **Way 2:** roll back on a timeout as well, everywhere. Nothing in the firewall
  changes, and every caller pays a second deadline before reporting.

**Which way do you want this fixed?** The deadline and the report land either
way. Only the rollback verdict is in question, and it must be the SAME verdict
in the firewall and in the interface path, or the consistency the owner asked
for does not exist.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/evidence.md` - the rule this change is governed by: the deadline is a guard
  → Constraint: a computed-but-uninstalled deadline is indistinguishable from having none. The test MUST prove the deadline is installed BEFORE the first request, not that a helper computed it.
- [ ] `ai/rules/goroutine-lifecycle.md` - the lock held across an unbounded wait is the failure, not the wait itself
  → Constraint: name the lock each path holds, and state what else contends for it.
- [ ] `docs/architecture/core-design.md` - the "Firewall reconcile concurrency" anchor of `internal/plugins/firewall/vpp/timeout_linux.go`
  → Decision: the deadline is bound inside the constructor so no call site can forget it. That rationale is what this spec generalises.

### RFC Summaries (Scope: protocol)
- N-A. The govpp binary API is a vendor protocol with no RFC. No wire format
  Ze owns is touched.

**Key insights:** (minimal context to resume after compaction)
- govpp exports `core.ErrReplyTimeout`, so no new sentinel is needed: `errors.Is` against it identifies a wedged dataplane in any package.
- The firewall's clamp ceiling is `firewall.MaxBackendDeadline`. Importing the firewall package from iface, fib or static is a tier smell; the shared bound belongs where the connector lives.
- All three plugins already have test fakes whose `SetReplyTimeout` is an empty method, so the seam exists and nobody used it.
- fib and static do not report a dataplane failure to the operator at all today. That half of the owner's requirement is larger than the deadline half.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/firewall/vpp/timeout_linux.go` - `newGovppOps` calls `ch.SetReplyTimeout(vppReplyTimeout())` before returning the facade; `vppReplyTimeout` reads `ze.firewall.vpp.reply-timeout` and clamps to `[minReplyTimeout, maxReplyTimeout]`, rejecting zero because it is govpp's spelling of "no deadline". `asDataplaneTimeout` tags `core.ErrReplyTimeout` as `firewall.ErrKernelTimeout`.
- [ ] `vendor/go.fd.io/govpp/core/channel.go` - `(*Channel).receiveReplyInternal` reads a non-positive `replyTimeout` as `maxInt64`; `SetReplyTimeout` is the only writer.
- [ ] `vendor/go.fd.io/govpp/core/connection.go` - `DefaultReplyTimeout = time.Duration(0)`, documented as disabling the timeout.
- [ ] `internal/component/vpp/conn.go` - `(*Connector).NewChannel`, the one place every plugin obtains a channel from the pooled connection.
- [ ] `internal/plugins/iface/vpp/ifacevpp.go` - `(*vppBackendImpl).ensureChannel` caches a channel under `chMu` and returns a sentinel without caching when the connector is not ready. Requests run under `reconcileMu`, taken by `reconcileOnReadyWithJournal` (`internal/component/iface/config_apply.go`), which is process-wide for interface reconciles.
- [ ] `internal/component/iface/config_apply.go` - `reconcileOnReadyWithJournal` holds `reconcileMu` across every backend call, records per-method errors and rolls the whole commit back through the journal.
- [ ] `internal/plugins/fib/vpp/fibvpp.go` - `(*fibVPP).processEvent` holds `mu` for a whole batch and, on a failed install, logs `fib-vpp: add route failed` at Error and `continue`s. There is no error return and no failure metric, so the RIB and the dataplane diverge with nothing reported.
- [ ] `internal/plugins/fib/vpp/backend.go` - `(*govppBackend).routeAddDel` and `.richRouteAddDel` issue the requests; `newGovppBackend` takes the channel.
- [ ] `internal/plugins/static/inject.go` - `(*routeManager).applyRoutes` holds `mu`, logs `static: route skipped, kept rest of section` at Warn, records the key in `rm.skipped`, and returns nil. Per-route isolation is deliberate and carries a spec reference.
- [ ] `internal/plugins/static/vpp/backend.go` - `(*Backend).routeAddDel` issues the request on a shared channel.
- [ ] `internal/component/firewall/engine.go` - the journal `Record` call whose undo re-registers the previous tables and calls `ApplyAll` again. This is the rollback the Open Question is about.
- [ ] `internal/plugins/ddos/local/responder.go` - skips the rollback reconcile on `firewall.ErrKernelTimeout`, with the reasoning stated in a comment.

**Behavior to preserve:**
- The firewall keeps its own deadline value and its own env key. This spec does not retune it.
- Static keeps per-route isolation: one route the backend refuses does not fail the section. Only the operator-visible reporting changes.
- The iface commit journal keeps rolling back on ordinary backend errors. Only the timeout verdict is in question.
- A channel obtained by a test that injects its own fake keeps working; the fakes' empty `SetReplyTimeout` must stay valid.
- No plugin gains a dependency on the firewall package.

**Behavior to change:**
- Every channel from the shared connector carries a reply deadline by construction.
- A timed-out request is identifiable as a wedged dataplane, distinct from a rejected request.
- iface, fib and static each report a wedged dataplane to the operator.
- The rollback verdict on a timeout becomes the same in the firewall and in the interface path, per the owner's answer.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A config commit or an interface event reaching `reconcileOnReadyWithJournal`.
- A sysrib best-change event reaching `(*fibVPP).processEvent`.
- A static route apply, or a BFD state change, reaching `(*routeManager).applyRoutes`.

### Transformation Path
1. A plugin asks the connector for a channel.
2. The channel is returned with a reply deadline already installed.
3. The plugin issues a binary-API request while holding its own lock.
4. VPP answers, or the deadline expires and govpp returns `core.ErrReplyTimeout`.
5. The plugin distinguishes that error from a rejection, releases its lock, and reports.
6. The rollback verdict follows the owner's answer to the Open Question.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin → connector | `(*Connector).NewChannel` (`internal/component/vpp/conn.go`) returns a channel; no plugin builds one | Yes: all three obtain channels there |
| Plugin → govpp | `SendRequest`/`ReceiveReply` on the channel; the deadline is channel state, not a call argument | Yes: `SetReplyTimeout` is the only writer of `replyTimeout` |
| Plugin → operator | fib and static have no failure surface today; iface reports through the commit journal | To be established: this is the half the owner's requirement adds |

### Integration Points
- `(*Connector).NewChannel` (`internal/component/vpp/conn.go`) - the one seam every plugin already passes through. Binding here is what makes forgetting impossible.
- `core.ErrReplyTimeout` (govpp) - the sentinel every package can test without importing another plugin.
- `rm.skipped` (`internal/plugins/static/inject.go`) - the existing operator-visible surface for a route the backend refused. fib's report should copy this shape rather than invent one.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The bound sits at the channel seam every plugin already uses |
| No unintended coupling (components stay isolated) | Yes | No plugin imports the firewall package. The sentinel comes from the vendor library |
| No duplicated functionality (extends existing, does not recreate) | Yes | One bound, not three copies of `newGovppOps` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Control path, no wire buffers |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them (`ai/rules/plugins.md`) | Yes | Nothing new registers. The bound is a property of an existing seam |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Binding at `(*Connector).NewChannel` reaches every production path | All three plugins obtain channels there, and the firewall's own facade takes a channel it was given | A plugin builds a channel another way and stays unbounded | Grep every `NewChannel` caller and every `api.Channel` construction before implementing | unvalidated |
| A-2 | A shared default deadline suits all three | The firewall chose 10s for a reconcile that holds a process-wide lock; these hold narrower locks | One plugin needs a longer bound and gets spurious timeouts under load | A per-plugin override on top of the shared default, exercised by AC-6 | unvalidated |
| A-3 | `core.ErrReplyTimeout` is returned for a request VPP accepted and never answered, and not for other failures | `receiveReplyInternal` returns it from the timeout arm only | The three plugins report a wedged dataplane for an ordinary rejection | A unit test that drives a fake channel to the timeout arm and to a rejection arm, asserting different verdicts | unvalidated |
| A-4 | govpp pools channels and `(*Channel).Reset` does not clear `replyTimeout` | Stated in the firewall's own comment as the reason it binds in the constructor | A recycled channel loses its deadline and the guard silently stops working | A test that obtains, releases and re-obtains a channel, asserting the deadline survives | unvalidated |
| A-5 | The three test fakes keep compiling with an empty `SetReplyTimeout` | The interface they satisfy is unchanged | Every fake needs editing, widening the diff | Compile the three packages before changing any production file | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A deadline that is too short turns a slow-but-healthy VPP into a failed commit | Timeouts under load in the functional suite | The clamp has a floor and a ceiling, and the value is an env var, as the firewall's already is |
| R-2 | The bound is added and no test proves it was INSTALLED, so a later refactor silently drops it | No recording-channel assertion in the diff | AC-1: a recording fake asserts `SetReplyTimeout` was called before the first request, per plugin |
| R-3 | fib keeps silently diverging because the reporting half is skipped as "not the timeout work" | A diff that adds a deadline to fib and changes nothing else | AC-4 is not optional. The owner asked for the report, not only the deadline |
| R-4 | The rollback verdict is answered differently in the firewall and in iface, so the inconsistency the owner named survives the fix | Two different verdicts in one diff | AC-7 asserts the SAME verdict on both paths in one test run |
| R-5 | Static's per-route isolation is broken while adding the report, making one bad route section-fatal | The static suite fails on a config with one unprogrammable route | The isolation is deliberate and documented. AC-5 pins it |
| R-6 | A timeout leaves the plugin's cached channel in a bad state and every later request fails | The second commit after a timeout fails differently from the first | A test that drives a timeout and then a successful request on the same backend |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Too long a deadline keeps today's behaviour, which is a wedged router that reports nothing. Too short a deadline fails healthy commits under load. Getting the rollback verdict wrong makes an operator wait a second deadline for a failure they were already going to get, on a router whose dataplane is not answering |
| How is it reverted? | Single commit revert. No config migration and no persisted state. The env var becomes unknown and is ignored |
| Who else touches this path? | `plan/spec-traffic-vpp-deferred-reply-timeout.md` covers the traffic backend, the fourth plugin in this class. It should land first or together; the two must not choose different mechanisms |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A config commit changing an interface, against a VPP that accepts and never answers | → | `reconcileOnReadyWithJournal` (`internal/component/iface/config_apply.go`) through the iface VPP backend | `TestIfaceCommitFailsOnWedgedDataplane` (`internal/plugins/iface/vpp/apply_test.go`) |
| A best-change event installing a route, same wedged VPP | → | `(*fibVPP).processEvent` (`internal/plugins/fib/vpp/fibvpp.go`) | `TestFibReportsWedgedDataplane` (`internal/plugins/fib/vpp/apply_test.go`) |
| A static route apply, same wedged VPP | → | `(*routeManager).applyRoutes` (`internal/plugins/static/inject.go`) | `TestStaticReportsWedgedDataplaneAndKeepsIsolation` (`internal/plugins/static/vpp/apply_test.go`) |
| A channel obtained from the connector | → | `(*Connector).NewChannel` (`internal/component/vpp/conn.go`) | `TestNewChannelInstallsReplyDeadline` (`internal/component/vpp/conn_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A channel obtained from the connector, in each of iface, fib and static | `SetReplyTimeout` has been called with a positive value BEFORE the first request. A recording fake proves the ordering, per plugin |
| AC-2 | A VPP that accepts a request and never answers, on each of the three paths | The call returns within the deadline, the lock is released, and the error is identifiable as a wedged dataplane rather than a rejection |
| AC-3 | The iface path, same condition | The commit fails, the operator sees a message naming a wedged dataplane, and the interface is left in the state the owner's answer to the Open Question prescribes |
| AC-4 | The fib path, same condition | The failure is reported to the operator through a surface an operator actually reads, not only an Error log line. The RIB-versus-dataplane divergence is visible |
| AC-5 | The static path, same condition | The failure is reported, AND per-route isolation is intact: other routes in the section stay programmed and the apply still returns nil |
| AC-6 | The env var for the deadline set below the floor, above the ceiling, and to zero | Each clamps rather than disabling the bound. Zero is refused, because it is govpp's spelling of "no deadline" |
| AC-7 | A timeout on the firewall config path and a timeout on the iface path, in one test run | Both produce the SAME rollback verdict, whichever the owner chose. A difference between them is the defect this spec exists to remove |
| AC-8 | A channel released to the pool after a timeout, then obtained again | The deadline is still installed. A recycled channel never comes back unbounded |
| AC-9 | A request VPP REJECTS, on each of the three paths | The existing behaviour is unchanged: not reported as a wedged dataplane, and the rollback verdict for a rejection is what it is today |
| AC-10 | A healthy VPP under the functional suite | No spurious timeout. The deadline does not turn a slow reply into a failure |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | commits an interface change while VPP is wedged | config commit → iface reconcile → VPP backend → deadline → reported failure | `test/plugin/vpp-wedged-iface-commit-reports.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 2 | watches routes fail to install while VPP is wedged | best-change event → fib → deadline → operator-visible report | `test/plugin/vpp-wedged-fib-reports.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 3 | applies static routes while VPP is wedged | static apply → deadline → report, other routes intact | `test/plugin/vpp-wedged-static-reports.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 4 | tunes the deadline for a slow lab dataplane | env var → clamp → effective deadline | `TestReplyDeadlineClamps` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNewChannelInstallsReplyDeadline` | `internal/component/vpp/conn_test.go` | AC-1: the bound is installed at the seam, before any request | |
| `TestReplyDeadlineClamps` | `internal/component/vpp/conn_test.go` | AC-6: floor, ceiling and the refusal of zero | |
| `TestIfaceCommitFailsOnWedgedDataplane` | `internal/plugins/iface/vpp/apply_test.go` | AC-2, AC-3: bounded wait, lock released, wedged-dataplane verdict | |
| `TestFibReportsWedgedDataplane` | `internal/plugins/fib/vpp/apply_test.go` | AC-2, AC-4: the report exists where an operator reads it | |
| `TestStaticReportsWedgedDataplaneAndKeepsIsolation` | `internal/plugins/static/vpp/apply_test.go` | AC-2, AC-5: report added, isolation unbroken | |
| `TestRejectionIsNotAWedgedDataplane` | one per plugin, beside the above | AC-9: the two failure kinds stay distinguishable | |
| `TestRecycledChannelKeepsItsDeadline` | `internal/component/vpp/conn_test.go` | AC-8: the pool cannot hand back an unbounded channel | |
| `TestRollbackVerdictMatchesFirewall` | `internal/component/iface/config_apply_test.go` | AC-7: one verdict, two paths | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| reply deadline | floor to ceiling, both to be fixed in the design phase | the ceiling | zero and any negative value, which are govpp's "no deadline" and MUST clamp to the floor | anything above the ceiling, which clamps down rather than disabling the bound |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vpp-wedged-iface-commit-reports` | `test/plugin/vpp-wedged-iface-commit-reports.ci` | A commit against a wedged dataplane fails inside the deadline and says why | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `vpp-wedged-fib-reports` | `test/plugin/vpp-wedged-fib-reports.ci` | A route that cannot be installed is visible to the operator | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `vpp-wedged-static-reports` | `test/plugin/vpp-wedged-static-reports.ci` | The bad route is reported, the good ones stay programmed | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |

### Interop Tests (Scope: protocol)
N-A. The govpp binary API is a vendor interface with no second implementation
to test against. The wedged-dataplane condition is produced by a fake channel
that accepts a request and never answers, which is the only way to reach the
timeout arm deterministically.

## Files to Modify
- `internal/component/vpp/conn.go` - install the deadline in `(*Connector).NewChannel`, with the clamp and the env var
- `internal/plugins/iface/vpp/ifacevpp.go` - identify a wedged dataplane and pass that verdict up
- `internal/component/iface/config_apply.go` - the rollback verdict on a timeout, per the owner's answer
- `internal/plugins/fib/vpp/fibvpp.go` - report a wedged dataplane to the operator instead of logging and continuing
- `internal/plugins/fib/vpp/backend.go` - surface the sentinel from the request path
- `internal/plugins/static/inject.go` - report a wedged dataplane while keeping per-route isolation
- `internal/plugins/static/vpp/backend.go` - surface the sentinel from the request path
- `internal/component/firewall/engine.go` - under Way 1 only: skip the rollback on a timeout, matching the DDoS responder
- `docs/guide/vpp.md` or the equivalent operator page - the new env var and what a wedged-dataplane report means

## Files to Create
- the three `.ci` files named in Functional Tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | The deadline is an env var, matching `ze.firewall.vpp.reply-timeout` |
| YANG validation constraints | No | No leaf |
| YANG custom validators | No | No leaf |
| CLI commands/flags | Maybe | AC-4 needs an operator-visible surface for fib. If `fib show` is the chosen surface, its output changes |
| CLI grammar (keyword before value) | No | No new command |
| Editor autocomplete | No | No config leaf |
| Functional test for new RPC/API | Yes | The three `.ci` files |
| Pipe completeness | Maybe | Only if a show command's output gains a column |
| Env var registration | Yes | The new deadline key through `env.MustRegister()`, as `ze.firewall.vpp.reply-timeout` already is |
| Doctor check for runtime dependencies | Yes | A wedged dataplane is exactly what a doctor check should surface, and static already reports skipped routes there |
| Prometheus counters/metrics | Yes | A timeout counter per plugin, matching `ze_firewall_apply_timeout_total`. fib and static have no failure metric today |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface is touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | The operator now learns that the dataplane is wedged. That is new behaviour to document |
| 2 | Config syntax changed? | No | Env var only |
| 3 | CLI command added/changed? | Maybe | Only if a show command carries the fib report |
| 4 | API/RPC added/changed? | No | No API surface |
| 5 | Plugin added/changed? | Yes | Three plugins change behaviour on a dataplane failure |
| 6 | Has a user guide page? | Yes | The VPP operator page |
| 7 | Wire format changed? | No | No Ze wire format |
| 8 | Plugin SDK/protocol changed? | No | No SDK type crosses this seam |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC |
| 10 | Test infrastructure changed? | No | Existing `test/plugin` suite |
| 11 | Affects daemon comparison? | No | No claim in `docs/comparison.md` about dataplane deadlines |
| 12 | Internal architecture changed? | Yes | The deadline moves from a per-plugin choice to a property of the channel seam |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | Yes | The per-plugin timeout counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | The new env var joins the registered set |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming the three plugins' backends and `internal/component/vpp/conn.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Any page showing the firewall deadline env var should name its siblings |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the wait is unbounded today
   - Tests: `TestIfaceCommitFailsOnWedgedDataplane`, `TestFibReportsWedgedDataplane`, `TestStaticReportsWedgedDataplaneAndKeepsIsolation`, each written to fail
   - Files: the three `apply_test.go` files
   - Verify: each HANGS or fails on the deadline never firing, not on a compile error. A test that fails for another reason proves nothing about the bound
2. **Phase: The bound at the seam**
   - Tests: `TestNewChannelInstallsReplyDeadline`, `TestReplyDeadlineClamps`, `TestRecycledChannelKeepsItsDeadline`
   - Files: `internal/component/vpp/conn.go`
   - Verify: AC-1, AC-6, AC-8. Every `NewChannel` caller is bounded, and no call site had to remember anything
3. **Phase: Identify the failure kind**
   - Tests: `TestRejectionIsNotAWedgedDataplane`, one per plugin
   - Files: the three backends
   - Verify: AC-2, AC-9. A rejection and a silence are different verdicts
4. **Phase: Report to the operator** -- the half fib and static do not have at all
   - Tests: `TestFibReportsWedgedDataplane`, `TestStaticReportsWedgedDataplaneAndKeepsIsolation`
   - Files: `internal/plugins/fib/vpp/fibvpp.go`, `internal/plugins/static/inject.go`
   - Verify: AC-4, AC-5. The report reaches a surface an operator reads, and static's isolation is intact
5. **Phase: The rollback verdict** -- the owner's answer, applied to BOTH paths
   - Tests: `TestRollbackVerdictMatchesFirewall`
   - Files: `internal/component/iface/config_apply.go`, and `internal/component/firewall/engine.go` under Way 1
   - Verify: AC-7. One verdict, proven on both paths in one run
6. **Phase: Functional proof and discrimination**
   - Tests: the three `.ci` files, each re-run with the bound reverted
   - Files: the three `.ci` files, the docs, the counters
   - Verify: AC-3, AC-10, and every new `.ci` goes red when the bound is removed

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All three plugins, and the report as well as the deadline. A deadline with no report is half the owner's requirement |
| Feature completeness | The bound survives the channel pool. A guard that a recycled channel loses is not a guard |
| Correctness | A rejection is never reported as a wedged dataplane, and a wedged dataplane is never reported as a rejection |
| Naming | The env var names the dataplane, not the plugin's own domain, since one value covers three plugins |
| Data flow | One bound at one seam. Three copies of the firewall's constructor is the shape this spec exists to avoid |
| Rule: `ai/rules/evidence.md` | The test proves the deadline was INSTALLED before the first request. A test that only proves it was computed proves nothing |
| Rule: `ai/rules/simplicity.md` | No new sentinel type: govpp already exports one. No import of the firewall package from any of the three |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No unbounded channel remains | `grep -rn 'NewChannel' internal/` reviewed against the bound, and `grep -rn 'SetReplyTimeout' internal/ --include=*.go` showing the seam plus any deliberate override |
| All three plugins report | The three `.ci` files pass and fail with the bound reverted |
| Static isolation intact | The static suite passes with one unprogrammable route in the config |
| One rollback verdict | `TestRollbackVerdictMatchesFirewall` |
| Lint | `make ze-lint-changed` |
| Packages | `make ze-unit-pkg-test` for the three plugin packages and `internal/component/vpp` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| What a wrong landing exposes | A router whose control plane is wedged and says nothing, which is today. The change cannot make that worse, but a too-short deadline can fail healthy commits, which an operator may work around by disabling the bound |
| What proves it did not | The clamp refuses zero, so the bound cannot be disabled through the env var. AC-6 asserts it |
| Fail closed | A timeout fails the operation. It never reports success for a request VPP did not confirm |
| Denial of service | An unbounded wait under a process-wide lock IS the denial of service this fixes. Name the lock each path holds in the review |
| No secret in output | The report names a prefix, an interface and a deadline. No credential is on this path |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A new `.ci` passes with the bound reverted | The test is vacuous. It must HANG or fail without the bound |
| Binding at the connector misses a path | A `NewChannel` caller or an `api.Channel` built another way. Name it and bind it; do not add a second mechanism |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The firewall's own comment states why it binds inside the constructor: a
  computed-but-uninstalled deadline is indistinguishable from having none. That
  argument does not stop at the firewall, and it is the reason this spec binds at
  the connector rather than writing `newGovppOps` three more times.
- The deferral row that homed this work argued for a per-backend bound, because
  the three have different callers, locks and blast radii. Those differences
  govern the VALUE and the REPORT, not the mechanism. A shared bound with a
  per-plugin override keeps both: nobody can forget it, and anybody can tune it.
- fib is the worst of the three and the least visible. Its handler runs
  synchronously in the engine's serial dispatch, so one wedged route stalls the
  sysrib emitter and every later subscriber, and its only report today is a log
  line at Error.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Bind at `(*Connector).NewChannel` | Three per-plugin constructors mirroring `newGovppOps` | The firewall's stated rationale generalises: bind where it cannot be forgotten. Three copies is three chances to forget, and the fourth plugin proves the pattern already failed once |
| Use `core.ErrReplyTimeout` directly | A new shared sentinel, or importing `firewall.ErrKernelTimeout` | govpp already exports one. Importing the firewall package from iface, fib or static is a tier violation for no gain |
| Copy static's `skipped` surface for fib's report | A new report shape for fib | An operator already knows where to look for a route the backend refused. A second shape is a second thing to learn |
| The rollback verdict is one decision, applied to both paths | Fix iface and leave the firewall inconsistent | The owner asked for the same operator-visible outcome. Fixing one side and not the other leaves exactly the inconsistency the request names |

## Known Limitations
- The traffic backend is the fourth plugin in this class and is covered by
  `plan/spec-traffic-vpp-deferred-reply-timeout.md`. If that spec lands first
  with a per-plugin constructor, this spec must reconcile the two mechanisms
  rather than add a second.
- The connect phase already bounds its own wait, and this spec does not change
  it. An absent VPP and a wedged VPP stay different conditions.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated in all three plugins, not test-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] The owner has answered the Open Question, and the answer is recorded here

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: vendor binary API, no second implementation)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

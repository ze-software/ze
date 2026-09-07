# Spec: plugin-query-mode

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-07 |

<!-- Skeleton only. Research below is transcribed from work already done at the
     producers on 2026-09-07. Design, acceptance criteria and test plan are NOT
     written yet: the sections that carry template placeholders are unwritten,
     not skipped. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The owner's requirement, 2026-09-07, verbatim.**

> "we may need to have way to ask plugin for their details without starting them"

and, correcting a misreading of that first sentence:

> "no, it must be started but we should have a way to ask it for information in
> a way where it is aware that it is not being started but queries, for example
> not inialising any data or starting connection/binding/etc."

**What that asks for.** The plugin process IS started. A mode tells the plugin
that it is being interrogated rather than run. The plugin answers with its
declarations and performs none of a live start's work: no data initialisation,
no connection, no socket bind, no listener, no timer.

**Why it is not already true.** Every field of a plugin's command declaration is
a static property of its source, and the only way to read it today is to start
the plugin for real. So a reader that wants the declarations has to accept every
side effect a live start carries, and the ones that run before the plugin speaks
its first protocol word are the reason a reader currently refuses to try.

**One fact, one declaration.** This spec adds no second copy of anything.
There is one declaration, each plugin's own `commandDecls()` in its own package,
and query mode reads it through the same Stage 1 message a running daemon reads.
No manifest, no build-time emission, no generated list, nothing to compare, and
nothing that can disagree with the plugin (`ai/rules/principles.md`).

**Prior art, not a dependency.** `spec-daemon-backed-command-catalog` made the
catalog read a plugin's declarations from its `registry.Registration`, and
rejected a collector that would start engines to introspect them. Its audit of
all 97 registered runners is the evidence for that rejection. That spec closed
on 2026-09-08 and its text now lives in
`plan/learned/007-declaration-on-the-registration.md`, which carries the audit
table whole. This spec transcribes the part of it that bears on query mode.

## Required Reading

<!-- NEVER tick [ ] to [x]. Annotations below are the ones already established;
     the unannotated rows are reading this spec still owes at design. -->

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - the five startup stages and what each carries
  → Constraint: Stage 1 `declare-registration` is the message that already carries every declaration this spec wants to read
- [ ] `docs/architecture/cli/plugin-modes.md` - internal and external plugin modes
- [ ] `docs/architecture/plugin/plugin-system.md` - registration, discovery, process boundary
- [ ] `docs/plugin-development/protocol.md` - what a plugin author is told the protocol is
- [ ] `ai/rules/plugins.md` - what a plugin owns and what a generic package must not spell
- [ ] `ai/patterns/plugin.md` - the structural template for a plugin and its runner

### RFC Summaries (Scope: protocol)
N-A. No wire protocol and no RFC obligation: the protocol here is Ze's own
plugin RPC.

**Key insights:** (minimal context to resume after compaction)
- The declarations are static data trapped behind a process start. That is the entire gap.
- The dirty work is in the runner body BEFORE `p.Run`, which no protocol stage gates.
- Five states must be distinguishable, and "declared none" is not "sent nothing".

## Current Behavior (MANDATORY)

**Source files read:** (read at the producer on 2026-09-07)
- [ ] `pkg/plugin/rpc/types.go` - `CommandDecl` (line 449) carries 11 wire fields: name, description, long-help, help (retired), args, completable, hidden, deprecated-names, shape, columns, address-fields. `PipeDecl` (line 354) carries the pipe aliases: command, name, description, expansion. Every one is a static property of the plugin's source, and each plugin's own `commandDecls()` is a pure function of that source. Nothing in a declaration is discovered at run time
- [ ] `pkg/plugin/sdk/sdk.go` - `(*Plugin).Run` (line 364) runs the 5-stage startup. Stage 1 `declare-registration` (line 375) is the FIRST engine call it makes, before Stage 2 configure. `Run` sends Stage 1 unconditionally, including for an empty `Registration`
- [ ] `internal/component/plugin/process/process.go` - `(*Process).startExternal` tells an external plugin exactly five environment variables (lines 665-670): `ZE_PLUGIN_HUB_HOST`, `ZE_PLUGIN_HUB_PORT`, `ZE_PLUGIN_HUB_TOKEN`, `ZE_PLUGIN_CA_PEM`, `ZE_PLUGIN_NAME`. That block is a carrier the tree can already take for a mode
- [ ] `internal/component/plugin/registry/registry.go` - `RunEngine func(conn net.Conn) int` (line 42) is the whole engine-mode entry point. It carries no environment block, so the process-start carrier above does not reach an in-tree runner
- [ ] `internal/component/plugin/inprocess.go` - an in-tree plugin's engine already runs in-process over a `net.Conn` and reaches `reg.RunEngine(conn)` (line 127). A mode read inside `(*Plugin).Run` is therefore reachable by in-tree runners too
- [ ] `internal/plugins/dhcpserver/register.go` - `runDHCPServerPlugin` (line 61) is the clean shape: it binds only inside its `OnConfigure` closure, through `startServer` to `startListeners` (line 107). An engine that records `onRegistration` and never sends configure gets the full declaration from a plugin that opened nothing. No plugin change is needed for this shape
- [ ] `internal/component/ike/engine/register.go` - `runEngine` (line 355) calls `dataplane.Load` (line 359) and then `installIKEBypass` (line 366) unconditionally, writing four node-wide XFRM policies before Stage 1. Its `defer` (line 387) removes them, and because the policies are node-wide that removal also strips a live ike daemon's bypass
- [ ] `internal/plugins/flowspec-firewall/engine.go` - `runEngine` (line 227) subscribes to the event bus and, under `firewall.LegacySweepPending()`, calls `firewall.ApplyAll()` (line 268) before `p.Run` (line 273). `ApplyAll` under that documented exemption autoloads the OS backend for an EMPTY desired set, so it issues nftables syscalls
- [ ] `internal/plugins/trafficusage/register.go` - `runEngine` calls `att.Available()` (line 50) before its own `!p.IsInternal()` gate (line 67), and `Available` performs `rlimit.RemoveMemlock()` (`internal/plugins/trafficusage/attach_linux.go`, line 33), a `setrlimit` on the calling process
- [ ] `internal/component/plugin/register.go` - `show plugins` writes one row per plugin and NEVER drops a row it could not read: a plugin that recorded a setup outcome and never completed `Register` keeps its row with `descriptionUnregistered` (line 37), and a registered plugin that recorded nothing keeps its row with the unknown outcome. That is the precedent for the state answer this spec owes

**The audit of all 97 registered runners, 2026-09-07.** Transcribed from
`plan/learned/007-declaration-on-the-registration.md`, section 1, which carries
it whole. About 19 distinct plugins do something beyond callback
wiring and a signal handler before Stage 1:

| Finding | Runners | What runs before Stage 1 |
|---------|---------|--------------------------|
| Mutates the HOST | `flowspec-firewall`, `ike` | nftables syscalls through `ApplyAll` under `LegacySweepPending`; four node-wide XFRM policies through `installIKEBypass`, whose defer beside a live IKE engine removes that daemon's too |
| Mutates the calling PROCESS | `trafficusage` | `rlimit.RemoveMemlock()`, a `setrlimit` |
| Opens a kernel handle | `fib/kernel` | `netlink.NewHandle`, whose close sits after the abort's `return 1` |
| Allocates the process-wide default Loc-RIB | `connected`, `rib`, `static` | `locrib.Default()` |
| Leaves a process global pointing at a dead plugin | `adj_rib_in`, `rib`, `redistribute_egress`, `sysrib`, `isis`, `ospf`, `iface` | only two carry any unwind; `ospf` also registers an opaque type, whose second registration returns `ErrOpaqueTypeRegistered` and is only logged |
| Leaks goroutines on an abort | `iface` | four, because its stops are straight-line code AFTER `p.Run` rather than defers |

**Seven runners never reach Stage 1 at all.** `capa`, `loop` and `srpolicy`
return without calling `p.Run`. `as112`, `flowexport`, `vrrp` and `trafficusage`
return 1 at `if !p.IsInternal()`, which a bare `net.Pipe` always fails because
`NewWithConn` sets the bridge by type-asserting `rpc.Bridger`. Since `Run` sends
Stage 1 unconditionally even for an empty `Registration`, "declared nothing" and
"sent nothing" are different states, and about 60 of the 97 are legitimately the
former.

**Behavior to preserve:**
- The five startup stages and their order. Query mode reads the EXISTING Stage 1 message and adds no second declaration path.
- A live start is unchanged in every particular: same env block plus whatever carries the mode, same stages, same side effects.
- `show plugins` keeps answering from the compiled-in registry with no daemon.

**Behavior to change:**
- A plugin process started in query mode knows it is being interrogated, answers with its declarations, and performs none of a live start's work.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Unwritten: which reader asks, and how the mode reaches the plugin, are open design questions below]
- [Format at entry]

### Transformation Path
1. Reader asks for a plugin's declarations (caller not yet decided)
2. The plugin process is started, carrying the mode
3. The plugin sends Stage 1 `declare-registration` and performs no live-start work
4. The reader records the declarations, or the state that says why it has none

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ external plugin process | five environment variables at `startExternal`, then the Stage 1 RPC | No |
| Engine ↔ in-tree plugin | `RunEngine(conn net.Conn) int` over an in-process conn, no environment block | No |

### Integration Points
- `(*Plugin).Run`, `pkg/plugin/sdk/sdk.go` - owns the stage order a mode would have to be read in
- `(*Process).startExternal`, `internal/component/plugin/process/process.go` - owns the environment block a mode would travel in
- `show plugins`, `internal/component/plugin/register.go` - the precedent for reporting a per-plugin state rather than dropping a row

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Open Design Questions

<!-- These are the questions this spec exists to answer. They are RECORDED here,
     not answered: answering them is the design phase. -->

**Q-1. Enforcement shape.** A mode the plugin merely READS is convention, and
convention binds a port the day an author forgets. `p.Run` cannot enforce it,
because the runner body already ran by the time `Run` is called, which is
exactly where the audit above found the side effects. The candidate that does
enforce is an SDK entry point that OWNS the call order: the SDK sends Stage 1
from the registration and invokes the plugin's activation function only when the
mode is not query, which makes side-effecting code unreachable rather than
discouraged. That is the leading candidate. Anything short of it is convention,
and the design MUST label it so. A supervisor that denies the process a socket
is NOT a substitute: it stops a bind and does not stop `installIKEBypass`, which
needs no socket.

**Q-2. The five-state answer.** A configured plugin that cannot be asked MUST
appear in the answer carrying its state, and MUST NOT be omitted: an answer that
drops what it could not read returns a silently wrong value
(`ai/rules/principles.md`). Five states must be distinguishable:

| State | What it means |
|-------|---------------|
| declared commands | Stage 1 arrived and carried declarations |
| declared none | Stage 1 arrived and carried an empty registration. About 60 of 97 runners are legitimately here |
| sent nothing | the runner returned before Stage 1, as the seven named above do. The answer must NAME these, never report them as declaring nothing |
| could not be started | the process did not start |
| started and did not answer | the process started and Stage 1 never arrived inside the budget |

`show plugins` is the precedent for the shape. The last state needs a STATED
timeout budget, because without one "did not answer" is indistinguishable from
"still starting".

**Q-3. Scope of the pre-`p.Run` split.** Whether the runners the audit named are
fixed as part of this spec, or named as its precondition, is open. Every one of
them puts work in the runner body that no protocol stage gates.

**Q-4. Whether in-tree plugins are in scope.** External-only is a SCOPE
DECISION, not a property of the transport. An in-tree plugin's engine already
runs in-process over a `net.Conn` (`internal/component/plugin/inprocess.go`), so
a mode read inside `(*Plugin).Run` reaches in-tree runners too, and the roughly
19 runners above are what would then need suppressing. What IS settled is
narrower: `RunEngine func(conn net.Conn) int` carries no environment block, so
the process-start carrier does not reach an in-tree runner. That says the
carrier differs, not that in-tree plugins are out of reach.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every field of a command declaration is static, so a query-mode answer equals a live daemon's answer | `CommandDecl` and `PipeDecl`, `pkg/plugin/rpc/types.go`; each plugin's `commandDecls()` | A plugin whose declaration varies with configuration would answer differently in the two modes, and the catalog would publish the wrong one | compare a query-mode answer against a running daemon's `show command help` for the same plugin | unvalidated |
| A-2 | Stage 1 is reachable with no configure, so a plugin can declare without activating | `(*Plugin).Run`, `pkg/plugin/sdk/sdk.go` line 375, and `runDHCPServerPlugin` binding only inside `OnConfigure` | Query mode would need a protocol change rather than a mode | the dhcpserver shape, exercised with no Stage 2 | unvalidated |
| A-3 | The audit's runner findings still hold when this spec is implemented | `plan/learned/007-declaration-on-the-registration.md`, section 1, measured 2026-09-07 | The pre-`p.Run` scope in Q-3 is the wrong size | re-read the named runners at design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A query-mode start still mutates the host, because the mode is convention and a runner ignores it | a query run changes nftables, XFRM policy, or an rlimit | Q-1: an SDK entry point that owns the call order, so the code cannot run |
| R-2 | The answer silently drops a plugin it could not read | a plugin that is configured and absent from the answer | Q-2: five states, one row per plugin, never an omission |
| R-3 | A query-mode start beside a LIVE daemon damages the live daemon | ike loses its bypass; a plugin global is left pointing at a dead process | Q-3, and the ike defer is the worked example |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A query-mode start that is not inert mutates the host it was only asked to interrogate: nftables tables, four node-wide XFRM policies, the calling process's memlock rlimit. Beside a running daemon it can strip that daemon's IKE bypass |
| How is it reverted? | Single commit revert while the mode has no reader. Once a published catalog reads it, the reader has to be repointed as well |
| Who else touches this path? | `spec-daemon-backed-command-catalog` (closed 2026-09-08, same Stage 1 declarations, read from `registry.Registration`; see `plan/learned/007-declaration-on-the-registration.md`), and every plugin runner the audit named |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [unwritten: reader not yet chosen, see Q-1 and Q-4] | → | [feature function] | [test name proving the chain] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | [unwritten: written at design, after Q-1 through Q-4 are answered] | [observable outcome] |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | [unwritten: depends on which reader asks, Q-1] | [path] | [test name] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestXxx` | `pkg/plugin/sdk/*_test.go` | [unwritten] | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| query answer timeout | [budget unwritten, see Q-2] | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-xxx` | `test/plugin/*.ci` | [unwritten] | |

### Interop Tests (Scope: protocol)
N-A. No wire-visible protocol change and no peer daemon: the plugin RPC is Ze's own.

## Files to Modify
<!-- Candidates the design will confirm or replace. Nothing here is decided. -->
- `pkg/plugin/sdk/sdk.go` - where the stage order is owned, so where a mode would be enforced (Q-1)
- `internal/component/plugin/process/process.go` - the environment block a mode would travel in for an external plugin
- `internal/component/plugin/registry/registry.go` - the in-tree entry point, if Q-4 puts in-tree plugins in scope
- the runners the audit named, if Q-3 puts the pre-`p.Run` split in this spec

## Files to Create
- [unwritten]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | unanswered at skeleton |
| YANG validation constraints | | unanswered at skeleton |
| YANG custom validators | | unanswered at skeleton |
| CLI commands/flags | | unanswered at skeleton |
| CLI grammar (keyword before value) | | unanswered at skeleton |
| Editor autocomplete | | unanswered at skeleton |
| Functional test for new RPC/API | | unanswered at skeleton |
| Pipe completeness | | unanswered at skeleton |
| Env var registration | | unanswered at skeleton, and note the five `ZE_PLUGIN_*` variables are process-start environment rather than registered `ze.*` config leaves |
| Doctor check for runtime dependencies | | unanswered at skeleton |
| Prometheus counters/metrics | | unanswered at skeleton |
| BGP family surface (new SAFI / capability / attribute) | N-A | no family, capability or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | | unanswered at skeleton |
| 2 | Config syntax changed? | | unanswered at skeleton |
| 3 | CLI command added/changed? | | unanswered at skeleton |
| 4 | API/RPC added/changed? | | unanswered at skeleton |
| 5 | Plugin added/changed? | | unanswered at skeleton |
| 6 | Has a user guide page? | | unanswered at skeleton |
| 7 | Wire format changed? | N-A | no wire format |
| 8 | Plugin SDK/protocol changed? | | likely `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md`, `docs/plugin-development/protocol.md`; confirmed at design |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | no RFC |
| 10 | Test infrastructure changed? | | unanswered at skeleton |
| 11 | Affects daemon comparison? | | unanswered at skeleton |
| 12 | Internal architecture changed? | | unanswered at skeleton |
| 13 | Route metadata keys added/changed? | N-A | no route metadata |
| 14 | Prometheus counters added/changed? | | unanswered at skeleton |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | unanswered at skeleton |
| 16 | Any changed source file referenced by existing doc source anchors? | | DERIVED: `./le spec citation anchors spec plan/spec-plugin-query-mode.md` at design |
| 17 | Existing docs show config/CLI/API examples for this area? | | unanswered at skeleton |

## Implementation Steps

<!-- Unwritten. The phases cannot be ordered until Q-1 through Q-4 are answered:
     Q-1 decides what the wiring phase registers, and Q-3 and Q-4 decide how
     many runners are in the diff. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- [unwritten, see Q-1]
   - Tests: [from the Wiring Test table]
   - Files: [entry point]
   - Verify: the entry point exists and is reachable, and the wiring test fails because the feature is a stub

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Inertness | A query-mode start touches no nftables table, no XFRM policy, no rlimit, no socket, no timer, no process global |
| No silent omission | Every configured plugin has a row, carrying one of the five states in Q-2 |
| State distinctness | "declared none" and "sent nothing" are separate answers, and the seven runners that never reach Stage 1 read as the second |
| One fact | The answer comes from the plugin's own `commandDecls()` through Stage 1, and no second declaration is introduced |
| Enforcement, not convention | If the mode is only read by the plugin, the spec says so in those words |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| [unwritten] | [command that proves it] |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Privilege at start | A query-mode start must not need, take, or keep the privilege a live start needs (`rlimit.RemoveMemlock`, netlink handles, nftables) |
| Credential exposure | Whether a query-mode process needs the hub token and CA at all |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- The declarations are static data trapped behind a process start. That is the whole gap, and it is why no new declaration format is needed.
- The protocol already splits declaring from activating for the common case, and `runDHCPServerPlugin` is the worked example. What is unguarded is the runner body before `p.Run`.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- [unwritten]

## RFC Documentation (Scope: protocol)

N-A. No RFC governs the plugin RPC.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is
- [ ] Q-1 through Q-4 each answered in the spec, not in conversation

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

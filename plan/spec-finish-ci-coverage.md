# Spec: finish-ci-coverage

| Field | Value |
|-------|-------|
| Status | done |
| Depends | - |
| Phase | op-1 Tier-1 commands (closed) |
| Deferral shard | `plan/deferrals/finish-ci-coverage.md` |
| Updated | 2026-08-17 |

**This umbrella closed on 2026-08-17 against its op-1 phase alone.** AC-1 to AC-8
are met and proven, and so are the agent-tooling gates T-4 and T-5 and the
`test/pppoe/` orphan repair. The four remaining work items in `## Task` had no
phase, no acceptance criteria and no test; they are re-homed at
`plan/future/spec-ci-coverage-remaining-surfaces.md`, which states per item what
is missing and what blocks it. Nothing was dropped and nothing here was red.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `git log -p plan/deferrals.md` (pre-2026-07-06) - original deferral rows + evidence

## Task

Write the deferred `.ci` functional tests whose feature code already exists and is unit-tested. No hard infra blocker - this is per-knob/per-command runner plumbing that was batched-deferred.

This is a consolidation skeleton created from verified deferral survivors (backlog triage 2026-07-06). Each item below was confirmed still-open against the codebase with a producing `file:line`. Split into phases when picked up; the sections after Task are lightweight scaffolding to be filled at design time.

### Work items (migrated from the 2026-07-06 deferral triage; `L#` = row in the pre-triage `plan/deferrals.md`)

- **Env-knob `.ci` (L215)** - 0 of ~12 exist: openwait, announce-delay, pid-file, pprof, l2tp-log-level, bridge-ack, migration-env. Code reads env directly; unit tests cover plumbing.
  **RE-HOMED 2026-08-17** at `plan/future/spec-ci-coverage-remaining-surfaces.md`, item 1, with each key's producer named and `bridge-ack` recorded as resolving to nothing at HEAD.
- **op-1 Tier-1 command `.ci` (L217)** - ~4 of 10 exist. Missing: system-cpu, system-date, interface-type, interface-errors, generate-wireguard-keypair.
  **DONE.** This is the phase this spec closes on: AC-1 to AC-8 below.
- **cli-dispatch `.ci` (L83)** - validate-config done; missing `set interface create` and `update peeringdb`.
  **RE-HOMED 2026-08-17** at `plan/future/spec-ci-coverage-remaining-surfaces.md`, item 2.
- **no-congestion-initial chaos `.ci` (L118)** - UNBLOCKED - ze-chaos multi-peer orchestration now exists (`internal/le/testchaos/actions.go --peers`); just needs writing.
  **RE-HOMED 2026-08-17** at `plan/future/spec-ci-coverage-remaining-surfaces.md`, item 3, carrying the port-range constraint recorded below.
- **gRPC-over-wire `.ci` (L40)** - engine path covered by `test/plugin/grpc-execute.ci`; a true gRPC-wire test needs grpcio/grpcurl vendored (tooling gate).
  **RE-HOMED 2026-08-17** at `plan/future/spec-ci-coverage-remaining-surfaces.md`, item 4, which records the three candidate clients and that the choice is the owner's.
- **`test/pppoe/` orphan (from `plan/deferrals/fixit-ddos-test-infra.md`, 2026-07-16)** - DONE 2026-08-07.
  Thomas chose "repair" on 2026-08-07, which VOIDS the Option D (delete) decision recorded
  by the pppoe orphaned-tests fixit. The three `.ci` are restored and run.
  `option=netns:veth=` was never a real directive; the repair extends the option that already
  provisions netns interfaces, `netns-link`, with `peer=` (veth pair) and `vlan=` (802.1Q
  sub-interface on each end), rather than adding a second directive family for the same job.
  `registerCIRoot("pppoe", ...)` roots the suite and `./le qemu pppoe-test` runs it: the
  netns launch mode is required (each test asks for a veth pair) and so is ze's runtime kernel
  (`handlePADR` opens AF_PPPOX before it sends PADS, and stock Alpine has no `CONFIG_PPPOE`).
  Running them found the reason the feature had no test: **PPPoE never started from a real
  config.** `ExtractParameters` read `interface` as a `[]any`, and `Tree.ToMap` emits a keyed
  YANG list as a map of key to entry, so `Interfaces` was always empty and
  `registerBNGSubsystems` never registered the subsystem. Two unit tests passed throughout,
  both building the map by hand in a shape no producer emits; the replacements drive the real
  parser and fail against the pre-fix code on BOTH halves (0 interfaces AND 0 service names).
  Discrimination, each break run in QEMU and each turning exactly one test red: accepting any
  AC-Cookie reds `pppoe-basic`; resolving `veth-bng.100` to its parent reds `pppoe-vlan`;
  building the SCCRP with message type HELLO reds `pppoe-concurrent-l2tp`. That third break is
  what found the concurrent test vacuous as first written -- `len(data) >= 12` accepts L2TP's
  bare 12-byte ZLB ACK -- so it now parses the Message Type AVP.

- **Agent-tooling gates T-4 / T-5 (from `plan/deferrals/fixit-agent-tooling-misleads.md`)** - DONE 2026-08-07.
  T-5 (the validator's `.ci` demand) was scoped to daemon specs in `e3af7a13e`, with
  `validate-spec-tooling-surface-accepted` and two must-not-fire fixtures beside it.
  T-4 (the spec-write gate's evidence set) had been WIDENED rather than scoped, so any
  source read cleared any spec, and a Python hook or a YANG model still cleared nothing.
  `mark-source-read.sh` now records the KIND read and `c_design_without_lsp` asks for
  EVERY kind the spec's own `## Files to Modify` names, each on its own 30-minute clock.
  The LSP tool is gopls, so it grounds Go and nothing else. A subject the gate cannot
  read still takes the older any-source bar, and the gate warns that it did.
  Proven by the `design-gate` fixture section, which fails on the pre-change gates.
  An adversarial review (2026-08-07) defeated the first version four ways: an LSP-only
  session grounded a Python spec, a cheap kind stood in for an expensive one beside it,
  a fresh kind renewed a stale one, and a `### Checklist` row became a subject. Each has
  a fixture that reds against the code as it stood before the fix.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - the canonical architecture reference: the design principles all new code follows

### Source files / docs

- [ ] `internal/test/runner/` (functional runner conventions)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `test/scripts/ze_api.py` (retired, no successor) <!-- doc-links: ignore (deleted 2026-08-28 by eae282592 with no replacement) --> (test plugin API)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `test/plugin/grpc-execute.ci` (existing engine-path gRPC coverage)
  -> Constraint: verify current behaviour against this source before designing.

## Current Behavior (MANDATORY)

**Source files read** (re-verified 2026-08-03 for the op-1 phase):

- [ ] `internal/component/cmd/show/system.go` -- `handleShowSystemCPU` and
  `handleShowSystemDate`, registered as `ze-show:system-cpu` and
  `ze-show:system-date` in `internal/component/cmd/show/show.go`.
- [ ] `internal/component/iface/cmd/show_interface.go` -- `showInterfaceByType`,
  `showInterfaceErrors`, `showInterfaceBrief`, and the RPC registrations.
- [ ] `internal/component/iface/yang/ze-iface-interface-cmd.yang` -- the
  `ze:command` declarations the dispatcher registers command keys from.
- [ ] `internal/plugins/diag/diag.go` -- `RunWgKeypair`, registered as the local
  command `generate wireguard keypair` in `internal/plugins/diag/register.go`.
- [ ] `internal/component/plugin/server/command.go` -- `LoadBuiltinsWithAliases`,
  `Dispatcher.updateSortedKeys`, `matchBuiltinTokens`, `matchCommandTokens`.
- [ ] `internal/test/runner/runner_exec.go` -- how a `.ci` command is executed
  (working directory, `option=env`, per-command `exit=`).

**Behavior to preserve:**
- Every other `show` and `generate` handler, and the whole `.ci` runner contract.
- `show interface` with no argument still lists every interface in full detail.

**Behavior to change (found by this phase, fixed in it):**
- `brief`, `type`, `errors`, and `rate` each declared `ze:command
  "ze-show:interface"`, the same wire method as their parent container.
  `LoadBuiltinsWithAliases` registers one dispatcher key per YANG path, and
  `matchBuiltinTokens` tries the LONGEST key first, so the key consumed the
  keyword and `handleShowInterface` was handed the tokens after it. Its
  `switch args[0]` therefore never saw the keyword: `show interface errors`
  answered with EVERY interface, `show interface brief` with full detail,
  `show interface rate` with the interface list, and `show interface type <t>`
  with its usage text. Each now has its own wire method and handler, matching
  the sibling `scan` / `detail` / `counters` pattern in the same file.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze-test` functional runner executing `.ci` files against a live daemon

### Transformation Path
1. An already-shipped, unit-tested feature is selected
2. A `.ci` test drives it end-to-end through `ze cli` / plugin dispatch
3. The test asserts the observable daemon behaviour

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `.ci` runner -> daemon | plugin dispatch / CLI one-shot | [ ] |
| test plugin -> engine | `ze_api.py` API commands | [ ] |

### Integration Points
- `internal/test/runner/`
- `test/scripts/ze_api.py` (retired, no successor) <!-- doc-links: ignore (deleted 2026-08-28 by eae282592 with no replacement) -->
- the feature handlers under test

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - new commands/views/families/handlers register and are core-discovered, not hardcoded into a core/shared package (`ai/rules/plugins.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The verified `file:line` evidence in the Task items still holds at design time | 2026-07-06 backlog triage | Re-scope the item | grep/LSP at design time | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Scope drift when the umbrella is split into per-item specs | Item needs its own design doc | Split into a dedicated spec and re-point |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `.ci` dispatches `show system cpu` | → | `handleShowSystemCPU` | `test/plugin/system-cpu-show.ci` |
| `.ci` dispatches `show system date` | → | `handleShowSystemDate` | `test/plugin/system-date-show.ci` |
| `.ci` dispatches `show interface type <t>` | → | `handleShowInterfaceType` -> `showInterfaceByType` | `test/plugin/interface-type-show.ci` |
| `.ci` dispatches `show interface errors` | → | `handleShowInterfaceErrors` -> `showInterfaceErrors` | `test/plugin/interface-errors-show.ci` |
| `.ci` runs `ze generate wireguard keypair` | → | `RunWgKeypair` | `test/parse/cli-generate-wireguard-keypair.ci` |
| unit test shims `wg` on PATH | → | `RunWgKeypair` genkey -> pubkey pipe | `TestRunWgKeypair_PipesGenkeyIntoPubkey` |
| a Read of a hook / model / tool | → | `mark-source-read.sh` kind markers | `mark-source-read-writes-*` fixtures |
| a spec Write | → | `c_design_without_lsp` subject match | `design-gate-*` fixtures |

**Still not wired, and named so it is not mistaken for done:** the env-knob,
cli-dispatch, chaos, and gRPC-over-wire work items in `## Task` have no test and
no phase. They are re-homed at
`plan/future/spec-ci-coverage-remaining-surfaces.md`, item 1 to item 4, each with
what is missing and what blocks it stated from a producer read on 2026-08-17.

## Acceptance Criteria

Phase op-1 Tier-1 commands. One AC per missing command, plus the defect the
phase uncovered.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show system cpu` on a running daemon | Answers `done` with `num-cpu`, `num-goroutines`, `max-procs` (each an integer >= 1) and a `go-version` starting `go`. On a platform where host inventory is supported, exactly one of `hardware` / `hardware-error` is present, and `hardware.logical-cpus` is >= `num-cpu` |
| AC-2 | `show system date` on a running daemon | Answers `done` with `time`, `unix`, `unix-nano`, `timezone`, `utc-offset-secs`. The three renderings agree: `unix-nano / 1e9 == unix`, the RFC3339 `time` parses to the same epoch and carries `utc-offset-secs`, and `unix` is the caller's own wall clock |
| AC-3 | `show interface type <t>`, with `<t>` read off the running host | Answers `done` with an `interfaces` wrapper holding exactly the interfaces of that type in `show interface`, and nothing else |
| AC-4 | `show interface type <unmatched>` | REFUSED with `unknown interface type`, and the refusal lists the types the running set actually has |
| AC-5 | `show interface errors` | Answers `done` with an `interfaces` wrapper whose rows and four counter values equal exactly the links `show interface` reports with a non-zero `rx-errors` / `rx-dropped` / `tx-errors` / `tx-dropped`. Links with all-zero counters are excluded |
| AC-6 | `ze generate wireguard keypair extra-arg` | Exits 1, says `no arguments accepted`, prints the usage, and prints no key |
| AC-7 | `ze generate wireguard keypair` with a `wg` on PATH | Runs `wg genkey`, feeds THAT private key to `wg pubkey` on stdin, and prints `private: <k>` / `public:  <k>` |
| AC-8 | `show interface brief`, `type`, `errors`, `rate` dispatched by their full command text | Each reaches its own handler. They shared one wire method with their parent container, so the dispatcher consumed the keyword and `handleShowInterface` never saw it. Proven end to end for `type`, `errors` and `rate`; `brief` goes through the identical registration and is proven at the handler by `TestHandleShowInterfaceBrief`, with no `.ci` of its own (it was not in the op-1 missing list, and a fourth end-to-end proof of one mechanism buys little) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRunWgKeypair_PipesGenkeyIntoPubkey` | `internal/plugins/diag/diag_test.go` | AC-7. Replaces a `LookPath("wg")` skip that never ran | done |
| `TestRunWgKeypair_ReportsMissingWg` | `internal/plugins/diag/diag_test.go` | exit 1 and no partial output when `wg` is absent | done |
| `TestHandleShowInterfaceRejectsStrayToken` | `internal/component/iface/cmd/show_interface_test.go` | AC-8. The bare handler owns the no-argument form only | done |
| `TestHandleShowInterfaceBrief` | `internal/component/iface/cmd/show_interface_test.go` | AC-8. Brief returns the compact shape, not full detail | done |
| `TestHandleShowInterfaceType*` | `internal/component/iface/cmd/show_interface_test.go` | AC-3, AC-4 at the handler | done |
| `TestHandleShowInterfaceErrorsShape` | `internal/component/iface/cmd/show_interface_test.go` | AC-5 wrapper shape | done |
| `TestIfaceInterfaceCmdSchemaOwnsInterface` | `internal/component/iface/yang/show_cmd_schema_test.go` | the four new wire methods are declared by the owning module | done |
| `TestShowSchemaHasNoMigratedOwnerCommands` | `internal/component/cmd/show/yang/self_containment_test.go` | and are NOT declared centrally: its banned map carries `ze-show:interface-brief`, `-type`, `-errors` and `-rate` | done |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `system-cpu-show.ci` | `test/plugin/` | AC-1: an operator reads CPU state off a running daemon | done |
| `system-date-show.ci` | `test/plugin/` | AC-2: an operator reads the daemon clock | done |
| `interface-type-show.ci` | `test/plugin/` | AC-3, AC-4: filter interfaces by type, and be told what is valid | done |
| `interface-errors-show.ci` | `test/plugin/` | AC-5: find the links with errors or drops | done |
| `cli-generate-wireguard-keypair.ci` | `test/parse/` | AC-6: the offline CLI command resolves and rejects arguments | done |
| `interface-rate-show.ci` (corrected) | `test/plugin/` | AC-8: its assertion had pinned the aliasing defect | done |
| `design-gate` fixtures | `internal/le/` | T-4: an agent writes a spec about a hook, a model or the daemon; the gate asks for that subject and refuses the rest. A hook has no `.ci`, so its driving surface is the fixture suite | done |

Every one is proven by mutation: each was re-run with the behaviour under test
broken at the producer and observed to FAIL. The interface pair needs no
synthetic mutation to prove it either way, because both were RED against the
unfixed dispatcher and turned green only with the wire methods split.

## Files to Modify

- `internal/test/runner/` - see Task work items
- `internal/component/cmd/show/show.go` - see Task work items
- `internal/le/hookruntime/lifecycle.go` - T-4: records the kind of source read
- `.claude/hooks/pretool-writeedit.py` (retired; now `internal/le/hookruntime/writeedit.go`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> - T-4: `c_design_without_lsp` asks for the spec's subject
- `internal/le/` - T-4: the `design-gate` fixtures, both directions

## Implementation Steps

1. **Phase: split** - if the umbrella covers unrelated items, split into per-item specs first.
2. **Phase: design** - for the chosen item, re-verify the `file:line` evidence and fill the Data Flow / Wiring / AC sections above.
3. **Phase: wiring** - register entry points, write the failing wiring test.
4. **Phase: implement (TDD)** - write test, fail, implement, pass, per work item.
5. **Full verification** - `./le verify current mode full`.
6. **Complete spec** - fill audit tables, write `plan/learned/NNN-<name>.md`, two-commit closure.

## Deliverables Checklist

<!-- The skeleton this spec grew from carried no Deliverables, Security or
     Documentation checklist. The closure supplied all three rather than stopping,
     per /ze-close step 4. -->

| Deliverable | Verification method | Result |
|-------------|--------------------|--------|
| Five missing op-1 Tier-1 command `.ci` | `ls` the paths, then read each file for the assertion it makes | all five exist; each read in full, see Wiring Verified |
| The dispatcher aliasing fix, per form | `gopls symbols` over `internal/component/iface/cmd/show_interface.go`, then read `init` | eight `RPCRegistration` entries, one per form |
| Unit tests named in the TDD plan | `grep "func Test"` over each named file, then run the owning packages | 10 of 10 exist; four packages `ok` |
| Both YANG self-containment halves | read both test bodies | owner declares the four, central schema bans them |
| T-4 / T-5 agent-tooling gates | `make --no-print-directory ze-unit-hook-test` | 448/448, 52 `design-gate` fixtures pass |
| The four undesigned items re-homed, not dropped | `ls` the successor spec and read its item list | `plan/future/spec-ci-coverage-remaining-surfaces.md`, items 1 to 4 |
| Every citer of this spec repaired before commit B | `grep -ln` over `plan/spec-*.md`, then `./le spec citation anchors` | zero spec citers; gate exit 0 |

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/iface/yang/ze-iface-interface-cmd.yang` declares the four new `ze:command` wire methods; the central show schema declares none |
| YANG validation constraints | N-A | No new leaf. The commands take a positional token, not a config value |
| YANG custom validators | N-A | Same reason |
| CLI commands/flags | Yes | `internal/component/iface/cmd/show_interface.go` and `internal/component/cmd/show/show.go` register the handlers |
| CLI grammar (keyword before value) | Yes | `show interface type <type>` puts the keyword before the value |
| Editor autocomplete | Yes | Automatic: each form is a `ze:command` node in the owning YANG module, which is what the completion tree reads |
| Functional test for new RPC/API | Yes | `test/plugin/system-cpu-show.ci`, `system-date-show.ci`, `interface-type-show.ci`, `interface-errors-show.ci`, `interface-rate-show.ci`; `test/parse/cli-generate-wireguard-keypair.ci` |
| Pipe completeness | Yes | `showInterfaceByType` and `showInterfaceErrors` both answer a single-key `interfaces` wrapper precisely so `\| table` unwraps to the slice; the comment above each says so |
| Env var registration | N-A | This phase reads no env var. The env-knob work item is re-homed |
| Doctor check for runtime dependencies | N-A | No new dependency. `RunWgKeypair` shells out to `wg`, which predates this phase and reports its own absence with exit 1 |
| Prometheus counters/metrics | N-A | Read-only operator commands, no new observable state |
| BGP family surface | N-A | No SAFI, capability or attribute touched |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The six commands shipped before this phase; only their tests and their dispatch keys are new. `docs/features.md` needs no row |
| 2 | Config syntax changed? | No | No YANG config leaf changed; only `ze:command` nodes |
| 3 | CLI command added/changed? | Yes, already done | `docs/guide/command-reference.md` documents all six with their field sets and the aliasing note. Verified by reading it, not assumed |
| 4 | API/RPC added/changed? | Yes, already done | The four new wire methods are in `internal/component/plugin/all/testdata/wire-methods.snapshot`; `docs/architecture/api/commands.md` describes the registration mechanism, not the per-method list |
| 5 | Plugin added/changed? | No | `internal/plugins/diag` gained a test, not a surface |
| 6 | Has a user guide page? | Yes, already done | `docs/guide/command-reference.md` |
| 7 | Wire format changed? | N-A | No protocol bytes |
| 8 | Plugin SDK/protocol changed? | No | `.ci` files consume the existing `ze_api.py` |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC surface. `show` commands and a keypair generator are not protocol behavior |
| 10 | Test infrastructure changed? | No | The runner was not edited by this phase; two of its limits were found and are recorded |
| 11 | Affects daemon comparison? | No | No capability gained |
| 12 | Internal architecture changed? | No | One wire method per command form is the pattern the sibling `scan` / `detail` / `counters` forms already followed |
| 13 | Route metadata keys added/changed? | N-A | None |
| 14 | Prometheus counters added/changed? | N-A | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes, already done | Four new wire methods, present in `wire-methods.snapshot` and asserted by `TestRegisteredWireMethods` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes, checked | `docs/guide/command-reference.md` carries `<!-- source: internal/component/cmd/show/system.go -- handleShowSystemMemory/CPU/Date -->`, and its claim is current. `./le repository check` reports no stale anchor over any file this spec changed |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes, checked | The `show interface` and `show system` examples in `docs/guide/command-reference.md` match the handlers, including the note that `ze show interface brief` used to look for an interface called "brief" |

### Security Review Checklist

| Check | What to look for | Result |
|-------|-----------------|--------|
| Untrusted input into a shell | `RunWgKeypair` runs `wg genkey` and `wg pubkey` | Neither takes an argument. `fs.NArg() > 0` refuses every caller-supplied token before the first `exec.CommandContext`, so no operator string reaches a child process |
| Injection through the type filter | `showInterfaceByType` echoes the caller's token in its refusal | It is wrapped by `strconv.Quote`, so the message cannot break out of its own quoting, and the value reaches no shell, SQL or template |
| Unbounded allocation | `make(map[string]struct{})` and `make([]iface.InterfaceInfo, 0, len(ifaces))` in `showInterfaceByType`; the same shape in `showInterfaceErrors` and `showInterfaceBrief` | Every size derives from the kernel link list, not from operator or peer input. `iface.ListInterfaces` is the only source |
| Information leakage | `hardware-error` in `handleShowSystemCPU` carries `err.Error()` from `host.DetectCPU` | It is a `/proc/cpuinfo` read error, already visible to any operator who can run the command, and the command sits behind the same authorization as every other `show` |
| Guard that fails open | The argument guards on `handleShowInterface`, `handleShowInterfaceType` and `RunWgKeypair` | Each denies: `StatusError` with the usage text, or exit 1 with no key printed. `test/parse/cli-generate-wireguard-keypair.ci` asserts `reject=stdout:contains=private:`, so a guard that stopped denying reddens the test |
| Privilege escalation | Any new privileged path | None. All four handlers read; none writes |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/finish-ci-coverage-7584d469-e988-48fc-910f-c68d4a139d89.md` (17 files pinned by SHA-256, verdict clean) |
| `review_gate.py check` | clean |
| Rounds | 2 |
| Reviewer lenses used | logic + wiring + guard audit over op-1's producers; removed-behavior and stale-comment audit over the six `.ci`; closure-citation audit over every citer of this spec |

**Scope of the review, and how it was conducted.** op-1's Go and `.ci` are already
committed (the `.ci` group landed in `06c95f65d`,
`cli-generate-wireguard-keypair.ci` in `454f60a9f`, the handlers in `c3cb70157`
and earlier), so the review read them from source rather than from a working-tree
diff. Round 2 read only what round 1's fixes touched.

**The review is READING-BASED.** Every finding below names the function that
produces the behavior and was read at that function. The owner stopped test, lint
and build execution on this machine during the closure, so no finding rests on a
run this session did not see. The step-0 automated pre-checks
(`./le repository check`, `internal/le/testweakened/audit.go`) ran BEFORE
that instruction; both are recorded under "Reds attributed" below.

### Reds attributed (none of them this closure's, and none in its files)
| Red | Owning file | Attribution |
|-----|-------------|-------------|
| `./le repository check`: 1 ISSUE, `NewFeed` has no cross-package non-test caller | `internal/core/observation/observation.go` | Another session's in-flight `anomaly-observe` work. The file is uncommitted in this tree and is not in this commit |
| `internal/le/testweakened/audit.go`: 2 `[WEAKENED]`, RFC-tagged tests changed without an approval token | `internal/component/bgp/reactor/session_negotiate_test.go`, `internal/core/bgp/capability/negotiated_test.go` | Another session's in-flight RFC work. Both are uncommitted; neither is in this commit |
| `./le doc check verify`: `ai/DOCS-TO-CODE.md is stale` | `ai/DOCS-TO-CODE.md` | Generated index, stale from the untracked `anomaly-observe` plugin. On this closure's do-not-touch list. Its `Documentation drift`, `YANG/handler contract` and changed-file wiring stages all passed |
| `./le doc check links`: 95 dead path references repo-wide | 20+ files under `plan/` and `website/` | Pre-existing rot. The count was 96 before this closure and 95 after: the one that was this spec's was fixed. No file in this commit appears in the remaining 95 |
| the retired `ze-unit-hook-test` (current: `./le hook-check unit`): 4 of 448 fixtures fail | the `ze-unit-hook-test` recipe in the retired `Makefile` (current producers: `internal/le/` native action tables) | Not a product defect and not a fixture defect. The recipe does not set `MAKEFLAGS=--no-print-directory`, so four `session-id-*-make-path` fixtures read a `make[1]: Entering directory` banner. `make --no-print-directory ze-unit-hook-test` is 448/448. Recorded as review finding 6 |

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | The `Exercises:` header names the pre-AC-8 aliased path, `ze-show:interface -> handleShowInterface`. `init` in `internal/component/iface/cmd/show_interface.go` registers `ze-show:interface-type` to `handleShowInterfaceType` and `ze-show:interface-errors` to `handleShowInterfaceErrors`. The comment describes the defect AC-8 removed (`ai/rules/stale-comments.md`) | `test/plugin/interface-type-show.ci`, `test/plugin/interface-errors-show.ci` | fixed: each header now names its own wire method and handler |
| 2 | ISSUE | Two false claims in the header. (a) "setupWorkDir writes every tmpfs file 0644, so a `mode=755` fixture is never executable": `parsingRunner.setupWorkDir` (`internal/test/runner/parsing.go`) materializes through `test.Tmpfs.WriteTo(workDir)`, so `mode=` reaches disk in the parse suite. The claim would stop the next author writing the success-path `.ci` for a reason that no longer exists. (b) it homes both limits at a path for `spec-fixit-ci-peer-block-silent-directives`, a spec no longer on disk | `test/parse/cli-generate-wireguard-keypair.ci` | fixed: the mode half is recorded as fixed, the still-true `childEnv` PATH half is stated at its producer, and the homing is the bare stem `spec-fixit-parse-suite-helper-cannot-invoke-ze` |
| 3 | ISSUE | Twelve live `deferred` rows in nine shards under `plan/deferrals/` name this spec as their Destination. Commit B removes it, leaving each row homed at nothing. No gate sees it: the FAIL pass of `internal/le/spec/citation/speccitation.go` globs `plan/spec-*.md` and never reads `plan/deferrals/` | `plan/deferrals/` (9 files) | fixed: each live row's Destination now names `plan/future/spec-ci-coverage-remaining-surfaces.md`, which lists all twelve. Terminal rows keep the historical reference as the bare stem |
| 4 | NOTE | The TDD Unit Tests table named `TestShowYANGDoesNotOwnRelocatedCommands`. No such symbol exists; the test is `TestShowSchemaHasNoMigratedOwnerCommands` | this spec, TDD Test Plan | fixed: the row names the real test and what its banned map holds |
| 5 | NOTE | `showInterfaceByType("")` returns every interface whose `Type` is empty with status `done`, because `wantedLower == ""` matches them and `filtered` is then non-empty, so the unknown-type refusal is never reached. Not reachable from the CLI: an empty token cannot survive tokenization, and `handleShowInterfaceType` refuses `len(args) == 0`. Reachable only by a direct RPC passing `[""]` | `showInterfaceByType`, `internal/component/iface/cmd/show_interface.go` | acknowledged, not fixed. The fix is a Go edit, and the owner requires a full `./le verify current mode full` before any commit carrying Go; the tree does not compile today (three sessions mid-TDD). Reported to the main thread instead of committed |
| 6 | NOTE | the retired `ze-unit-hook-test` (current: `./le hook-check unit`) does not set `MAKEFLAGS=--no-print-directory` in its recipe, so the four `session-id-*-make-path` fixtures read a `make[1]: Entering directory` banner and fail: 444/448. `make --no-print-directory ze-unit-hook-test` is 448/448. `internal/le/testunit/groups.go` already carries this exact fix for the sibling target `ze-unit-pkg-test`, with a comment saying a scoped target whose verdict disagrees with the full gate is worse than no scoped target | the retired `Makefile` (current producers: `internal/le/` native action tables), the `ze-unit-hook-test` recipe | acknowledged, not fixed here. The goal does not depend on it, and a retired `Makefile` (current producers: `internal/le/` native action tables) edit would take this commit's single focus and demand a full verify (`ai/rules/rule-precedence.md`). Reported to the main thread with the journal row text |

### Fixes applied
- `test/plugin/interface-type-show.ci`, `test/plugin/interface-errors-show.ci`: the `Exercises:` header names the wire method and handler the producer actually registers.
- `test/parse/cli-generate-wireguard-keypair.ci`: the header's tmpfs-mode claim is corrected at the producer, and the dangling spec path becomes a bare stem.
- `plan/deferrals/` (9 files, 12 live rows): Destination re-pointed to the successor spec; 3 terminal rows keep the reference as a bare stem.
- `spec-fixit-ipsec-interop-cli-credentials`: its citation of this spec is restated, naming the shard that holds the row and the successor spec. That spec CLOSED on 2026-08-23, so its stem is written bare: a `plan/` path would cite a file the tree no longer holds.
- This spec: the TDD table's wrong test name corrected.

### Run 2 (over the fixes)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | none | The three `.ci` header rewrites carry no directive change, so no suite behavior changed. `./le spec citation anchors` is green and the successor spec exists in the same commit. Every finding in this round would have been a record defect, so this is the last round (`ai/rules/planning.md`) | - | - |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (two: findings 5 and 6, both reported to the main thread)

## Checklist

### Goal Gates (MUST pass)
- [ ] Every chosen work item has feature code + test
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `./le verify worktree` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Notes
- Skeleton = captured intent, not a designed spec (see `ai/rules/planning.md`). Moves to `design` when someone picks it up.

### Post-wave corrections (2026-07-10)

- New obligation for the chaos `.ci` work item (no-congestion-initial, L118): the chaos orchestrator now validates listener/port-range conflicts at entry. `ValidateConfigRangeConflicts` (`internal/chaos/orchestrator/conflict.go`) derives the BGP and listen port range bases from the profile list and delegates to `ValidateRangeConflicts` (`conflict.go`), rejecting web/metrics/mcp listener endpoints that fall inside the derived per-peer port ranges; it is invoked at orchestrator entry (`internal/chaos/orchestrator/run.go`). Any new chaos `.ci` config must place its web/metrics/mcp listeners outside the derived BGP/listen ranges or the orchestrator rejects the run before starting.

---

## Implementation Summary

### What Was Implemented

The op-1 Tier-1 command phase, plus two items that landed under this umbrella
before it closed.

- Five missing Tier-1 command `.ci`: `test/plugin/system-cpu-show.ci`,
  `system-date-show.ci`, `interface-type-show.ci`, `interface-errors-show.ci`,
  and `test/parse/cli-generate-wireguard-keypair.ci`.
- The dispatcher defect those tests uncovered. `brief`, `type`, `errors` and
  `rate` each declared `ze:command "ze-show:interface"`, the same wire method as
  their parent container, so the dispatcher key consumed the keyword and
  `handleShowInterface` never saw it. Each now has its own wire method and its own
  handler in `internal/component/iface/cmd/show_interface.go`, and
  `test/plugin/interface-rate-show.ci` was corrected: its old assertion had
  pinned the defect.
- The agent-tooling gates T-4 and T-5 (`internal/le/hookruntime/lifecycle.go`
  records the KIND read; `c_design_without_lsp` in
  `.claude/hooks/pretool-writeedit.py` (retired; now `internal/le/hookruntime/writeedit.go`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> asks for every kind the spec's own Files to
  Modify names, each on its own clock), proven by 52 `design-gate` fixtures in
  `internal/le/`.
- The `test/pppoe/` orphan repair (2026-08-07), which found that PPPoE never
  started from a real config: `ExtractParameters` read `interface` as a `[]any`
  while `Tree.ToMap` emits a keyed YANG list as a map.

### Bugs Found/Fixed

- The `show interface` sub-command aliasing defect above. Covered by
  `TestHandleShowInterfaceRejectsStrayToken`, `TestHandleShowInterfaceBrief`,
  `TestHandleShowInterfaceType*` and `TestHandleShowInterfaceErrorsShape`
  (`internal/component/iface/cmd/show_interface_test.go`), plus the three `.ci`
  that were RED against the unfixed dispatcher.
- The PPPoE `ExtractParameters` shape defect, covered by replacements that drive
  the real parser and fail on both halves against the pre-fix code.
- A `LookPath("wg")` skip that never ran, replaced by
  `TestRunWgKeypair_PipesGenkeyIntoPubkey`, which shims `wg` on PATH.

### Documentation Updates

None needed, and verified rather than assumed. `docs/guide/command-reference.md`
already documents all six commands, their fields, and the aliasing defect itself:
`show system cpu` and `show system date` with their field lists, `show interface
brief | type <type> | errors`, and the note that `ze show interface brief` used
to look for an interface called "brief". The file carries the source anchor
`<!-- source: internal/component/cmd/show/system.go -- handleShowSystemMemory/CPU/Date -->`.
`./le repository check` reports no stale anchor over any file this spec
changed. That file is uncommitted in another session's working tree today, so it
is deliberately NOT in this commit.

### Deviations from Plan

- The umbrella closes against one of its five work items. The other four are
  re-homed at `plan/future/spec-ci-coverage-remaining-surfaces.md` rather than
  designed here. They are coverage for surfaces that ship and are unit-tested;
  none is a defect and nothing is red, so `plan/future/` is the correct home
  (`plan/future/README.md`).
- No `show-interface-brief.ci` was written under `test/plugin/`. AC-8 says so and
  gives the reason: `brief` goes through the identical registration and is proven at the
  handler by `TestHandleShowInterfaceBrief`.
- The closure commit carries NO Go file. Three sessions are mid-TDD in
  `internal/plugins/anomaly/observe`, `internal/component/trafficfeature` and
  `internal/plugins/flowexport`, so the tree does not compile and the full
  `./le verify current mode full` the owner requires before a Go commit cannot run.
  op-1's Go was already committed, so nothing is held back by this.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1 assumed the 2026-07-06 triage evidence still held at design time | It holds for the op-1 item and for three of the four re-homed items. It does NOT hold for two names in the env-knob item: `bridge-ack` matches no `env.MustRegister` and no `env.Get*` call in the tree, and `internal/component/iface/` registers no env key at all; `migration-env` is not a Ze knob but the ExaBGP `env` translation in `internal/exabgp/migration/env.go` | grep over `env.MustRegister` and `env.Get*` for every name in the item, 2026-08-17 | Recorded per name in the successor spec's item 1, so its taker does not hunt a knob that does not exist |
| approach | The spec's TDD table named a unit test that does not exist | The test is `TestShowSchemaHasNoMigratedOwnerCommands` | reading the file the row named | Row corrected. A record defect, so it earned no extra review round |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Env-knob `.ci` (L215) | Changed | `plan/future/spec-ci-coverage-remaining-surfaces.md` item 1 | Re-homed. Nothing blocks it: `option=env` is parsed at `case "env":` in `internal/test/runner/record_parse.go` and used by `test/ipsec/*.ci` |
| op-1 Tier-1 command `.ci` (L217) | Done | AC-1 to AC-8 below | The phase this spec closes on |
| cli-dispatch `.ci` (L83) | Changed | `plan/future/spec-ci-coverage-remaining-surfaces.md` item 2 | Re-homed with what blocks each half |
| no-congestion-initial chaos `.ci` (L118) | Changed | `plan/future/spec-ci-coverage-remaining-surfaces.md` item 3 | Re-homed with the `ValidateConfigRangeConflicts` constraint |
| gRPC-over-wire `.ci` (L40) | Changed | `plan/future/spec-ci-coverage-remaining-surfaces.md` item 4 | Re-homed. The tooling question is stated with all three candidates measured at HEAD |
| `test/pppoe/` orphan | Done | `test/pppoe/`, the retired `ze-qemu-pppoe-test` (current: `./le qemu pppoe-test`) | 2026-08-07 |
| Agent-tooling gates T-4 / T-5 | Done | `internal/le/hookruntime/lifecycle.go`, `.claude/hooks/pretool-writeedit.py` (retired; now `internal/le/hookruntime/writeedit.go`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) -->, `internal/le/` | 2026-08-07 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `handleShowSystemCPU` (`internal/component/cmd/show/system.go`) sets `num-cpu`, `num-goroutines`, `max-procs`, `go-version`, and exactly one of `hardware` / `hardware-error` when `host.DetectCPU` does not return `ErrUnsupported`; `test/plugin/system-cpu-show.ci` range-checks each and rejects a run where neither hardware key appears | |
| AC-2 | Done | `handleShowSystemDate` (same file) renders one `time.Now()` three ways; `test/plugin/system-date-show.ci` asserts `unix-nano // 1e9 == unix`, that the RFC3339 form parses to the same epoch and carries `utc-offset-secs`, and that `unix` sits inside the observer's own window | |
| AC-3 | Done | `showInterfaceByType` returns the single-key `interfaces` wrapper; `test/plugin/interface-type-show.ci` computes the expected name set from `show interface` and requires equality | |
| AC-4 | Done | `showInterfaceByType` answers `unknown interface type <quoted>` plus a sorted list derived from `seen`; the same `.ci` requires the running type to appear in the refusal | |
| AC-5 | Done | `showInterfaceErrors` skips a nil `Stats` and every all-zero row; the `.ci` builds the reference set from `show interface` and REFUSES to pass when no clean link exists, so the exclusion half is never untested | |
| AC-6 | Done | `RunWgKeypair` (`internal/plugins/diag/diag.go`) rejects `fs.NArg() > 0` before reaching `wg genkey`; `test/parse/cli-generate-wireguard-keypair.ci` asserts exit 1, both stderr strings, and `reject=stdout:contains=private:` | |
| AC-7 | Done | `RunWgKeypair` pipes the trimmed `wg genkey` output into `wg pubkey` on stdin; `TestRunWgKeypair_PipesGenkeyIntoPubkey` shims `wg` and asserts `wg pubkey` receives exactly what `wg genkey` printed | The `.ci` cannot cover it: `runOneCommand` (`internal/test/runner/parsing.go`) builds the child environment with `childEnv`, which adds no PATH entry for the ze binary |
| AC-8 | Done | `init` in `internal/component/iface/cmd/show_interface.go` registers eight distinct wire methods, one per form. `TestIfaceInterfaceCmdSchemaOwnsInterface` asserts the owning YANG module declares each; `TestShowSchemaHasNoMigratedOwnerCommands` asserts the central show schema declares none | `brief` is proven at the handler only, by AC-8's own stated reason |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRunWgKeypair_PipesGenkeyIntoPubkey` | Done | `internal/plugins/diag/diag_test.go` | |
| `TestRunWgKeypair_ReportsMissingWg` | Done | `internal/plugins/diag/diag_test.go` | |
| `TestHandleShowInterfaceRejectsStrayToken` | Done | `internal/component/iface/cmd/show_interface_test.go` | |
| `TestHandleShowInterfaceBrief` | Done | same file | |
| `TestHandleShowInterfaceTypeNeedsAType`, `TestHandleShowInterfaceTypeRejectsUnknown` | Done | same file | The plan wrote the pair as `TestHandleShowInterfaceType*` |
| `TestHandleShowInterfaceErrorsShape` | Done | same file | |
| `TestIfaceInterfaceCmdSchemaOwnsInterface` | Done | `internal/component/iface/yang/show_cmd_schema_test.go` | |
| `TestShowSchemaHasNoMigratedOwnerCommands` | Done | `internal/component/cmd/show/yang/self_containment_test.go` | The plan named it `TestShowYANGDoesNotOwnRelocatedCommands`, which is not a symbol in the tree. Corrected |
| Five op-1 `.ci` plus the corrected `interface-rate-show.ci` | Done | `test/plugin/`, `test/parse/` | |
| `design-gate` fixtures | Done | `internal/le/` | 52 pass |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/test/runner/` | Changed | Not edited by this phase. Two runner limits were found instead and are recorded: one is fixed (`Tmpfs.WriteTo`), one is homed at `spec-fixit-parse-suite-helper-cannot-invoke-ze` |
| `internal/component/cmd/show/show.go` | Done | Registers `ze-show:system-cpu` and `ze-show:system-date` |
| `internal/le/hookruntime/lifecycle.go` | Done | Records the KIND read |
| `.claude/hooks/pretool-writeedit.py` (retired; now `internal/le/hookruntime/writeedit.go`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> | Done | `c_design_without_lsp` asks for the spec's own subject kinds |
| `internal/le/` | Done | The `design-gate` fixtures, both directions |

### Audit Summary
- **Total items:** 7 Task requirements, 8 AC, 10 test rows, 5 file rows
- **Done:** 3 Task requirements, 8 AC, 10 test rows, 4 file rows
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 4 Task requirements (re-homed, not dropped, and named in the successor spec), 1 file row

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Write the deferred `.ci` whose feature code already exists and is unit-tested, for the op-1 Tier-1 commands | functional | Five `.ci` written and one corrected, each read in full at closure and each asserting the behavior its AC names. **The discrimination claim is INHERITED from the implementation record, not observed at closure:** that record states each `.ci` was re-run with the behavior broken at the producer and observed to FAIL, and that the interface trio was RED against the unfixed dispatcher and turned green only when the wire methods were split. The closure did not re-run the functional suites (see "What was not run" below), so it verified the assertions by reading and did not re-measure the mutations |
| Do it without vacuous assertions | functional | Each `.ci` refuses its own vacuous case rather than passing quietly. `interface-errors-show.ci` fails the run when no link with stats exists, and again when no CLEAN link exists, because the exclusion half of the assertion would otherwise be untested. `interface-type-show.ci` carries the unknown-type rejection precisely because a host with one interface type would let a broken filter pass the equality check. `system-date-show.ci` bounds `unix` to the observer's own window so a constant cannot pass |
| Every chosen work item has feature code and a test | functional + unit | AC-1 to AC-8 above, each mapped to its producer and its test. `go test -race ./...` green on all four owning packages (output below) |
| Give the agent-tooling gates a driving surface | fixture suite | A hook has no `.ci`, so the driving surface is the fixture suite: 52 `design-gate` and 35 `mark-source-read` fixtures pass, 0 fail, and 13 of the `design-gate` set red against the pre-fix gates |
| The four undesigned items are not lost | spec | `plan/future/spec-ci-coverage-remaining-surfaces.md`, items 1 to 4, each stating what is missing and what blocks it from a producer read on 2026-08-17, plus the twelve inherited deferral rows |

## Deferrals Resolved

The shard is `plan/deferrals/finish-ci-coverage.md`. It is NOT removed: two rows
are still live, so the shard outlives this spec (`ai/rules/planning.md`).

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| 2026-07-16 HOOK-FRICTION F2 (spec validator rejects the mandated citation form) | resolved | `spec-fixit-agent-tooling-misleads` T-1, closed |
| 2026-07-16 HOOK-FRICTION F3 (LSP gate lifts on query text, not on a load) | resolved | `spec-fixit-agent-tooling-misleads` T-2, closed |
| 2026-07-16 HOOK-FRICTION F4 (commit gate advises a target that writes no record) | resolved | `spec-fixit-agent-tooling-misleads` T-3, closed |
| 2026-08-07 SECURITY: a one-prefix anomaly allowlist parses empty | deferred | `plan/spec-review-typed-config-decode.md`. Still live, still homed, verified on disk |
| 2026-08-07 Four more readers assert a config shape `Tree.ToMap` does not emit | deferred | `plan/spec-review-typed-config-decode.md`. Still live, still homed, verified on disk |
| 2026-08-07 the retired `ze-qemu-pppoe-test` (current: `./le qemu pppoe-test`) is named by no aggregate target or workflow | resolved | Landed 2026-08-12 in `dee3b9aae` |
| 2026-08-03 `tmpfs=...:mode=` discarded, and a parse-suite helper cannot invoke `ze` | homed | Half one landed in `dc591ec72` for the parse suite; the orchestrated half is recorded in `plan/journal/helper-bypassed-by-an-open-coded-copy.md`. Half two is `plan/future/spec-fixit-parse-suite-helper-cannot-invoke-ze.md`. `test/parse/cli-generate-wireguard-keypair.ci` was corrected in this closure: it still claimed the mode limit that `dc591ec72` fixed |

**Foreign shards this closure had to touch.** Twelve live rows in nine other
shards named this spec as their Destination, and commit B removes it. Each now
names `plan/future/spec-ci-coverage-remaining-surfaces.md`, which lists all
twelve. Three terminal rows in the same shards keep the historical reference as
the bare stem `spec-finish-ci-coverage`. No shard was emptied by this closure, so
none is removed.

## Pre-Commit Verification

**What was run, and what was not.** The owner stopped all test, lint and build
execution on this machine part-way through the closure: the load was the problem
being worked on. Everything listed below as a command result was run BEFORE that
instruction and its output was read; nothing is reported as passing that was not
seen to pass. After it, verification continued by READING producers.

| Not run | Why, and what stands in for it |
|---------|-------------------------------|
| `./le verify current mode full` | Owner instruction, and it could not have passed anyway: three sessions are mid-TDD in `internal/plugins/anomaly/observe`, `internal/component/trafficfeature` and `internal/plugins/flowexport`, so the tree does not compile. The commit carries no Go, no the retired `Makefile` (current producers: `internal/le/` native action tables), no the retired `scripts/` (current producer: `internal/le/`), no `.yang` and nothing that reaches a binary, so the gate's own applicability table does not reach it |
| The functional suites (`ze-functional-plugin-test`, `ze-functional-parse-test`) | Owner instruction, and the same non-compiling tree. The six `.ci` were read in full instead. The only `.ci` edits in this commit are header comments: no `cmd=`, `expect=`, `reject=`, `option=` or `tmpfs=` directive changed, so no suite behavior can have changed |
| `./le doc check verify` (full) | Started before the instruction and read: its `Documentation drift` and `YANG/handler contract` stages passed, and its ONLY failure was `WARNING: ai/DOCS-TO-CODE.md is stale -- run: ./le discovery-index update`. That generated index is stale from an untracked `anomaly-observe` plugin belonging to another session, and it is on this closure's do-not-touch list |
| A re-measurement of the `.ci` mutation discrimination | Owner instruction. The claim is inherited from the implementation record and is labelled as inherited in Goal Validation |

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/system-cpu-show.ci` | yes | `ls -1` on the twelve op-1 paths returned all twelve |
| `test/plugin/system-date-show.ci` | yes | same `ls -1` |
| `test/plugin/interface-type-show.ci` | yes | same `ls -1` |
| `test/plugin/interface-errors-show.ci` | yes | same `ls -1` |
| `test/plugin/interface-rate-show.ci` | yes | same `ls -1` |
| `test/parse/cli-generate-wireguard-keypair.ci` | yes | same `ls -1` |
| `plan/future/spec-ci-coverage-remaining-surfaces.md` | yes | written in this closure, `--file`d on commit A |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2 | The two system handlers produce the named fields | Read `internal/component/cmd/show/system.go` at the producer: `handleShowSystemCPU` and `handleShowSystemDate`. `go test -race ./internal/component/cmd/show` -> `ok github.com/ze-software/ze/internal/component/cmd/show 1.119s` |
| AC-3, AC-4, AC-5, AC-8 | The four forms reach their own handlers and shapes | Read `internal/component/iface/cmd/show_interface.go`: eight `RPCRegistration` entries, `showInterfaceByType`, `showInterfaceErrors`, `showInterfaceBrief`. `go test -race ./internal/component/iface/cmd` -> `ok ... 1.063s` (race-instrumented) |
| AC-6, AC-7 | The argument guard fires before `wg genkey`, and the pipe is genkey -> pubkey | Read `RunWgKeypair` (`internal/plugins/diag/diag.go`). the retired `ze-unit-pkg-test PKG=./internal/plugins/diag` (current: `go test -race ./internal/plugins/diag`) -> `ok ... 1.029s` |
| AC-8 (schema halves) | The owning module declares the four, the central one declares none | `go test -race ./internal/component/iface/yang` and `PKG=./internal/component/cmd/show/yang`, both `ok` |
| T-4 | The gate asks for the kind the spec names | `make --no-print-directory ze-unit-hook-test` -> `hook fixture check: 448/448 passed`, `OK`. 52 `design-gate` fixtures pass, 0 fail |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `.ci` dispatches `show system cpu` | `test/plugin/system-cpu-show.ci` | read in full: dispatches the literal `show system cpu`, range-checks four runtime fields, refuses a missing `hardware`, cross-checks `logical-cpus >= num-cpu` |
| `.ci` dispatches `show system date` | `test/plugin/system-date-show.ci` | read in full: three renderings cross-checked, plus an observer-window bound |
| `.ci` dispatches `show interface type <t>` | `test/plugin/interface-type-show.ci` | read in full: type read off the running host, set equality against `show interface`, plus the unknown-type refusal |
| `.ci` dispatches `show interface errors` | `test/plugin/interface-errors-show.ci` | read in full: reference set built from `show interface`, both directions asserted, both vacuity cases refused |
| `.ci` runs `ze generate wireguard keypair` | `test/parse/cli-generate-wireguard-keypair.ci` | read in full: `exec=ze generate wireguard keypair extra-arg`, exit 1, both stderr strings, `reject=stdout:contains=private:` |
| `.ci` dispatches `show interface rate` and `rate <name>` | `test/plugin/interface-rate-show.ci` | read in full: both forms, and the named form asserts the exact refusal string so a dropped argument reddens it |
| a Read of a hook / model / tool | `internal/le/` | `mark-source-read-writes-*` fixtures cover go, py, sh, make and yang, with must-not-fire cases for docs, specs, JSON and `.ci` |
| a spec Write | `internal/le/` | 52 `design-gate` fixtures |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken (partly) | The evidence holds for op-1 and for three re-homed items. It fails for two names in the env-knob item: `bridge-ack` resolves to no registered env key, and `migration-env` is the ExaBGP translation in `internal/exabgp/migration/env.go`, not a Ze knob. Recorded in the Mistake Log and in the successor spec |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| CLI reference for the six commands | `docs/guide/command-reference.md` lists `show system cpu` and `show system date` with their exact field sets, and the `show interface` forms `brief`, `type <type>` and `errors` with the aliasing note | yes, already current. No edit needed, and the file is another session's uncommitted work today so it is not in this commit |
| Source anchors over the changed files | `grep "source: internal/component/cmd/show/system.go"` finds the anchor in `docs/guide/command-reference.md`; `./le repository check` reports no stale anchor over any file this spec changed | yes |
| New gate or tool needing an `ai/INDEX.md` row | T-4 and T-5 changed existing gates rather than adding one; `ai/rules/repo-maintenance.md` already routes the hook gates | yes |

## Core Insight

A spec that becomes the catch-all destination for a class of deferred work
acquires citers that no gate reads. Twelve live deferral rows named this spec as
their home, and commit B would have orphaned every one of them silently: the FAIL
pass of `internal/le/spec/citation/speccitation.go` globs `plan/spec-*.md`, so
`plan/deferrals/` is invisible to it. The closure step added on 2026-08-10 checks
spec-to-spec citations and nothing else. An umbrella is therefore more expensive
to close than the work it holds, and the cost is proportional to how many rows
chose it as a destination rather than to its own diff.

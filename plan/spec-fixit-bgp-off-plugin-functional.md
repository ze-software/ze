# Spec: fixit-bgp-off-plugin-functional

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/plugins.md`, `ai/rules/testing.md`, `ai/rules/interop-and-goal-validation.md`
4. The `feature-gate-10-bgp` record (retired with the learned corpus) - the gate this spec back-fills
5. Source: `internal/plugins/fib/kernel/fibkernel.go`, `internal/plugins/mrt/component.go`, `internal/plugins/flowexport/enrichbgp.go`, `cmd/ze/hub/build_tag_bgp_absent_test.go`, `test/ui/ze-stripped-surface.ci`

## Task

Commit `c4038def0` ("feat(build): compile out the BGP subsystem behind ze_bgp")
requalified six always-on plugin files onto **core-leaf** packages so they build
and link with `ze_bgp` **off**:

| File | Import moved to | Purpose |
|------|-----------------|---------|
| `internal/plugins/fib/kernel/fibkernel.go` | `internal/core/bgp/routeaction` | kernel FIB backend |
| `internal/plugins/fib/p4/fibp4.go` | `internal/core/bgp/routeaction` | P4 FIB backend |
| `internal/plugins/fib/vpp/fibvpp.go` | `internal/core/bgp/routeaction` | VPP FIB backend |
| `internal/plugins/fib/vpp/srv6.go` | `internal/core/bgp/routeaction` | VPP SRv6 encap |
| `internal/plugins/flowexport/enrichbgp.go` | `internal/core/bgp/ribevents` | flow BGP AS enrichment |
| `internal/plugins/mrt/component.go` | `internal/core/bgp/msgtype` | MRT dump component |

The commit message asserts: *"A bare ze_core binary now links zero
internal/component/bgp symbols and still runs OSPF, IS-IS, static routes, the
FIB, MRT and flow export."* The `wiring-at-commit` gate flagged these files as
committed without a `.ci` functional test and asked the load-bearing question:
**is this reachable by a user, and is that reachability proven?**

The reachability is real (all three are config-driven, always-on plugins), but
the **"still runs with ze_bgp off" claim is proven only statically**:

| Existing proof | What it proves | What it does NOT prove |
|----------------|----------------|------------------------|
| `TestBuildTag_Protocols_AbsentBinaryDropsSymbols` (`cmd/ze/hub/build_tag_protocols_absent_test.go`) | `nm` shows zero `internal/component/bgp/*` symbols in a `ze_core` binary | that FIB/MRT/flow-export are still **present** and **run** |
| `TestBuildTag_BGP_Absent` (`cmd/ze/hub/build_tag_bgp_absent_test.go`) | bgp plugin is **absent**, reactor factory nil, `bgp{}` rejected | says nothing about fib/mrt/flow-export presence |
| `test/parse/no-bgp-fib-only.ci`, `no-bgp-empty.ci` | `ze config validate` accepts `fib{}` / empty config | runs against the **full** functional binary (`TestBuildTags` folds every default-on gate incl. `ze_bgp` — `internal/test/runner/runner.go,227`); it is `config validate` only, never a running daemon |

**Nothing exercises FIB, MRT, or flow-export in an actually `ze_bgp`-absent
binary at runtime.** The positive claim "still runs X" has no positive runtime
test. This spec adds that proof.

**Scope:** test/verification coverage only. No production behavior change is
intended. If a test reveals a genuine defect (e.g. a daemon that panics on boot
with `ze_bgp` off), fixing it is in scope (`ai/rules/completion.md`); it is not
a reason to weaken the test.

**Out of scope:** re-testing FIB/MRT/flow-export on the full binary (already
covered: `test/plugin/fib-table.ci`, `fib-ecmp-realtime.ci`,
`test/plugin/mrt-dump-all.ci`, `mrt-dump-updates.ci`, `test/flow-export/*.ci`).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `ai/rules/plugins.md` - the invariant this spec verifies at runtime
  → Constraint: a compile-out-able feature needs present/absent tests; this spec adds the *functional* present half for the always-on plugins that survive `ze_bgp` off.
- [ ] `ai/rules/interop-and-goal-validation.md` - "Prove the test discriminates"
  → Constraint: a link/schema proof is not a functional proof; each new test must fail if the behavior under test breaks.
- [ ] `ai/rules/platform-linux.md` - `option=needs-linux`, the stripped QEMU binary
  → Constraint: a `.ci` that boots a daemon applying kernel FIB state carries `option=needs-linux` and runs under `make ze-qemu-needs-linux-test`.

### RFC Summaries (MUST for protocol work)
- N/A — this is build-configuration / test-coverage work, no wire protocol change.

**Key insights:**
- `ze-stripped` (the shipped minimal binary) is built `-tags 'ze_core ze_ssh'` (`mk/test-functional.mk`) — no `ze_bgp` — and is already on PATH in the functional runner, with a working precedent (`test/ui/ze-stripped-surface.ci`). This is the ready-made vehicle for a bgp-absent functional `.ci`.
- The QEMU stripped binary is built `-tags 'ze_core $(ZE_TAGS)'` (`mk/test-integration.mk`) — `ze_core` only, **no ze_ssh** — so a QEMU/needs-linux test against it must assert via config-file + kernel readback, not SSH CLI.
- `cmd/ze/hub` unit tests run a second time under bare `ze_core` (`mk/test-unit.mk,51`), so a `//go:build !ze_bgp` Go test there is a deterministic, no-dataplane vehicle for the presence guard.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/plugins/fib/kernel/fibkernel.go` — fib-kernel subscribes to **sysrib** best-change events (`sysribevents`, `:147`), NOT bgp directly, and programs the kernel FIB using the core-leaf `routeaction` verb vocabulary. Config root `fib/kernel` (`register.go`).
  → Constraint: FIB consumes the protocol-neutral Loc-RIB (sysrib), which static/OSPF/IS-IS populate, so FIB genuinely installs routes with bgp off.
- [ ] `internal/plugins/mrt/component.go` / `register.go` / `dump.go` — MRT's data sources are the reactor message observer (`OnBGPMessage`, `component.go`) and the RIB-dump seam (`registry.GetRIBDumpCallback()`, `register.go`), both nil with bgp off. `dumpRIB()` nil-guards the callback (`dump.go`, logs "RIB dump skipped, no RIB callback"). Config root `mrt` (`register.go`); command `request mrt dump-rib` (`register.go`) returns cleanly ("mrt not configured" when idle, "rib dump triggered" otherwise) — no panic.
  → Constraint: MRT is **inert** without a BGP source (its entire purpose is dumping BGP). "Runs with bgp off" means "loads, boots, degrades gracefully", NOT "produces data".
- [ ] `internal/plugins/flowexport/enrichbgp.go` / `register.go` — the BGP enrich builder subscribes to `ribevents.BestChange` on the EventBus (`enrichbgp.go`). Nothing publishes those events with bgp off, so the enrich radix tree stays empty; the rebuild worker ticks and no-ops. Config root `flow-export` (`register.go`). Raw flow export (netflow9/ipfix/sflow) is independent of BGP.
  → Constraint: flow-export **runs** with bgp off but with **no AS enrichment**; that degradation must be treated as correct, not a bug.
- [ ] `cmd/ze/hub/build_tag_bgp_absent_test.go` / `build_tag_protocols_absent_test.go` — the existing absent proofs (registration absence + symbol drop). No present-side assertion for fib/mrt/flow-export.
- [ ] `test/ui/ze-stripped-surface.ci` — precedent: a `.ci` that boots `ze-stripped`, drives it over SSH CLI, and asserts command surface + daemon behavior.
- [ ] `feature-gates.txt` — fib/mrt/flow-export are **not** listed → always-on → present in every build including `ze_core`.

**Behavior to preserve:**
- All existing full-binary functional coverage (`fib-table.ci`, `mrt-dump-*.ci`, `flow-export/*.ci`) unchanged.
- The static proofs (`TestBuildTag_BGP_Absent`, `*_Protocols_AbsentBinaryDropsSymbols`, `no-bgp-*.ci`) unchanged — this spec **adds** to them.
- Graceful-degradation contracts: `dumpRIB` nil-guard, enrich subscription no-op with no publisher.

**Behavior to change:**
- None. This spec adds tests (and make/CI wiring) only. Production code changes only if a test surfaces a real defect.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator config on a **`ze_bgp`-absent** binary (`ze-stripped` native, or the `ze_core` QEMU binary): `fib { kernel {} }`, `mrt { ... }`, `flow-export { ... }`, `static { route ... }`.

### Transformation Path (FIB, the one data-producing path with bgp off)
1. `static` plugin resolves configured routes → publishes to sysrib (Loc-RIB).
2. sysrib arbitrates best path across protocols → emits `sysribevents.BestChangeBatch`.
3. fib-kernel consumes the batch (`fibkernel.go`), translates via `routeaction`, programs the kernel FIB (netlink, linux only).
4. Route observable in the kernel routing table.

### MRT / flow-export path (bgp off = inert / degraded)
1. MRT: no reactor → `OnBGPMessage` never called; `GetRIBDumpCallback()` nil → `dumpRIB` skips. Writers open, streams idle.
2. flow-export: `ribevents.BestChange` never published → enrich tree empty. Raw flow export path (dataplane → exporter → UDP) unaffected.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → plugin registry (bgp-absent build) | YANG schema of fib/mrt/flow-export present in `ze_core` | AC-1, AC-2 |
| sysrib → fib-kernel | `sysribevents.BestChangeBatch` (bgp-independent) | AC-4 |
| BGP RIB → flow-export / MRT | seam absent with bgp off (`GetRIBDumpCallback` nil; no `ribevents` publisher) | AC-5 |

### Integration Points
- `internal/component/plugin/registry` — `Has(name)` / `Names()` for the presence guard.
- `internal/component/config` — `ParseTreeWithYANG` / `ze config validate` for the schema check.
- The functional runner's `ze-stripped` binary on PATH; the QEMU stripped binary for `needs-linux`.

### Architectural Verification
- [ ] No bypassed layers (tests drive real config/CLI entry points, real binaries)
- [ ] No unintended coupling (no new production code; no bgp import re-introduced)
- [ ] No duplicated functionality (extends the absent-tests; does not re-test the full binary)
- [ ] Zero-copy preserved where applicable (N/A — test-only)
- [ ] Registration over hardcoding — presence guard reads the registry, does not hardcode a plugin list beyond the closed always-on set under test

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | fib-kernel/fib-p4/fib-vpp/mrt/flow-export are always-on (present in `ze_core`) | not in `feature-gates.txt`; `nm` test excludes them | presence guard cannot pass; feature is actually gated | `pluginreg.Has(...)` in a `!ze_bgp` Go test | unvalidated |
| A-2 | `ze-stripped` (`ze_core ze_ssh`) is on PATH in the functional runner | `test/ui/ze-stripped-surface.ci` invokes it; `mk/test-functional.mk` builds it | no native vehicle; must fall back to QEMU-only | run a trivial `ze-stripped config validate` `.ci` | unvalidated |
| A-3 | FIB installs sysrib-fed static routes with bgp off | `fibkernel.go` consumes `sysribevents`, bgp-independent | AC-4 unachievable; deeper coupling exists | QEMU `.ci`: static route appears in kernel FIB | unvalidated |
| A-4 | Daemon boots healthy on `ze-stripped` with fib+mrt+flow-export configured | nil-guards read (`dump.go`, `enrichbgp.go`) | AC-3 red = real boot defect to fix | AC-3 `.ci` | unvalidated |
| A-5 | The QEMU stripped binary (`ze_core`, no ssh) can be driven config-file-only | `mk/test-integration.mk`; needs-linux fib tests already assert via kernel readback | AC-4 must use the native `ze-stripped` + a linux runner instead | inspect an existing kernel-FIB `.ci` assertion mechanism during implementation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A new test is vacuous (passes on the full binary too, proving nothing bgp-specific) | test stays green when run against `ze` instead of `ze-stripped` | Each `.ci` MUST invoke `ze-stripped` (or the `ze_core` QEMU binary) explicitly and assert a bgp-off-specific outcome; mutation-verify by pointing it at `ze` and confirming a meaningful difference or by breaking the always-on registration. |
| R-2 | `request mrt dump-rib` or another reachable command panics on the nil seam with bgp off | AC-5 `.ci` crashes the daemon | `dump.go` already nil-guards; if another reachable path does not, fix it at the owner (`ai/rules/completion.md`), do not skip the assertion. |
| R-3 | Presence guard hardcodes a plugin list that drifts | future always-on plugin not covered | Keep the guard's list to the closed set this spec is about (the 3 flagged subsystems); document that it is a targeted regression guard, not an inventory. |
| R-4 | QEMU stripped binary lacks ssh, so an SSH-driven assertion silently can't run | AC-4 test skips or errors on the QEMU binary | Assert via config-file + kernel readback in the `tmpfs` Python (A-5); reserve SSH-driven assertions for the native `ze-stripped` (AC-2/AC-3). |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| bare `ze_core` build links the always-on plugins | → | `pluginreg.Register` for fib-kernel/fib-p4/fib-vpp/mrt/flow-export | `TestBuildTag_AlwaysOnPluginsPresent` (`cmd/ze/hub`, `//go:build !ze_bgp`) |
| `ze-stripped config validate` accepts fib/mrt/flow-export config | → | plugin YANG schemas in the bgp-absent binary | `test/plugin/bgp-off-schema-stripped.ci` |
| `ze-stripped -f conf` boots with fib+mrt+flow-export | → | each plugin's `OnConfigure`/`Start` | `test/plugin/bgp-off-boot-stripped.ci` |
| `static{}` + `fib{kernel{}}` on a bgp-off binary installs a route | → | `fibkernel` sysrib consumer + backend | `test/plugin/bgp-off-fib-install.ci` (`option=needs-linux`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `//go:build !ze_bgp` Go test in `cmd/ze/hub`, run in the bare-`ze_core` unit pass | `pluginreg.Has` is true for `fib-kernel`, `fib-p4`, `fib-vpp`, `mrt`, `flow-export`; the registry is non-empty (all.go linked). Positive mirror of `TestBuildTag_BGP_Absent`. |
| AC-2 | `ze-stripped config validate -` with `fib{kernel{}}`, `fib{p4{}}`, `fib{vpp{}}`, `mrt{...}`, `flow-export{...}` (one case each); and a `bgp{}` block | All fib/mrt/flow-export snippets validate (exit 0, "valid"); `bgp{}` is rejected as an unknown field (discriminating pair — proves the schemas that survived are real, not that validation is inert). |
| AC-3 | `ze-stripped -f conf` where `conf` configures `fib{kernel{}}` + `mrt{ ... }` + `flow-export{ ... }` | Daemon reaches ready (`ZE_READY_FILE`), does not exit early / panic; `show health` and `show warnings` report no error for these plugins; plugin "started"/"configured" log lines present. |
| AC-4 | On a `ze_bgp`-absent Linux binary: `static{ route <p> next-hop <nh> }` + `fib{kernel{}}` | Prefix `<p>` is present in the kernel FIB (readback via the mechanism used by existing kernel-FIB `.ci` tests). `option=needs-linux`; runs under `make ze-qemu-needs-linux-test`. Proves the `routeaction` core-leaf requalification installs routes with bgp compiled out. |
| AC-5 | On the AC-3 bgp-off daemon: trigger `request mrt dump-rib` (with `mrt{routes ...}` configured) and let flow-export run | No panic; MRT RIB dump is gracefully skipped (`dump.go` path), command returns cleanly; flow-export runs with an empty BGP-enrichment tree and logs no enrichment-subscription error. Documents+asserts graceful BGP-source absence. |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs a hardened `ze_core` build and configures the kernel FIB with static routes | config → static → sysrib → fib-kernel → kernel FIB | `bgp-off-fib-install.ci` (AC-4) |
| 2 | Runs a hardened build with `mrt{}` / `flow-export{}` configured (no BGP) | config → plugin Start → idle/degraded, daemon healthy | `bgp-off-boot-stripped.ci` (AC-3), `bgp-off-schema-stripped.ci` (AC-2) |
| 3 | Pastes a `bgp{}` block onto a hardened build | config parse → unknown field rejection | AC-2 discriminating half |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildTag_AlwaysOnPluginsPresent` | `cmd/ze/hub/build_tag_alwayson_present_test.go` (`//go:build !ze_bgp`) | AC-1: fib/mrt/flow-export registered in bare `ze_core` | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no numeric inputs) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-off-schema-stripped` | `test/plugin/bgp-off-schema-stripped.ci` | AC-2: fib/mrt/flow-export config validates on `ze-stripped`, `bgp{}` rejected | |
| `bgp-off-boot-stripped` | `test/plugin/bgp-off-boot-stripped.ci` | AC-3 + AC-5: daemon boots healthy, degrades gracefully | |
| `bgp-off-fib-install` | `test/plugin/bgp-off-fib-install.ci` (`option=needs-linux`) | AC-4: static route installed in kernel FIB with bgp off | |

### Interop Tests (MANDATORY for protocol features)
- N/A — no wire-protocol change. The "peer" here is a build configuration, not another daemon. (`ai/rules/interop-and-goal-validation.md`: interop not required for build/config-only work.)

### Future (if deferring any tests)
- P4 and VPP FIB *install* proofs with bgp off are deferred: P4 needs a P4 target and VPP needs a running VPP dataplane (heavier harness). Presence + schema for both is covered by AC-1/AC-2. Deferral home: this spec's own follow-up row if the user wants them; otherwise a documented Known Limitation. Requires explicit user approval before deferring.

## Files to Modify
- `docs/functional-tests.md` - document the bgp-off functional pattern (drive `ze-stripped` / the `ze_core` QEMU binary) so future feature-gate work reuses it, with source anchors to the new `.ci` files.
- `ai/rules/plugins.md` - add a line to the "add a feature gate" procedure: an always-on plugin surviving a large gate needs a *functional* present-half proof, not just registration/symbol-drop (this spec is the worked example).
- QEMU/runner wiring (`mk` build lines / the functional runner) - only if the AC-4 `needs-linux` `.ci` cannot reach the stripped `ze_core` binary as-is (it already builds `ZE_QEMU_STRIPPED_BIN`; confirm exposure inside QEMU). No change if already wired.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | none — tests reuse existing config roots |
| CLI commands/flags | No | none |
| Functional test for new RPC/API | Yes | `test/plugin/bgp-off-*.ci` |
| Env var registration | No | none |
| Doctor check for runtime dependencies | No | none (no new runtime dependency) |
| Prometheus counters/metrics | No | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | — (build config already documented by the gate) |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` and/or `ai/rules/plugins.md` — note the bgp-off functional pattern (drive `ze-stripped`) so future gate work reuses it |
| 12 | Internal architecture changed? | No | — |
| 15 | Registered plugin/command/inventory changed? | No | — |
| (others) | | No | verify via grep of `docs/` source anchors for the changed test files (none, tests are new) |

## Files to Create
- `cmd/ze/hub/build_tag_alwayson_present_test.go` — `//go:build !ze_bgp` presence guard (AC-1)
- `test/plugin/bgp-off-schema-stripped.ci` — AC-2
- `test/plugin/bgp-off-boot-stripped.ci` — AC-3 + AC-5
- `test/plugin/bgp-off-fib-install.ci` — AC-4 (`option=needs-linux`)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5-9 | verification + review loop |
| 10-14 | deliverables, docs, review gate, closure |

### Implementation Phases

1. **Phase: Presence guard (AC-1)** — add the `!ze_bgp` Go test; run it in the bare-`ze_core` pass (`GO_TEST_CORE ./cmd/ze/hub`).
   - Verify: fails if a fib/mrt/flow-export blank import is dropped; passes as-is.
2. **Phase: Schema on `ze-stripped` (AC-2)** — one `.ci` invoking `ze-stripped config validate` per config root + the `bgp{}` rejection.
   - Verify: green on `ze-stripped`; the `bgp{}` half red if pointed at the full `ze`.
3. **Phase: Boot-healthy + degradation (AC-3, AC-5)** — `.ci` booting `ze-stripped -f conf`, driving `show health`/`show warnings` and `request mrt dump-rib` over SSH (native `ze-stripped` has `ze_ssh`).
   - Verify: daemon healthy; no panic; graceful skip observed.
4. **Phase: FIB install under bgp off (AC-4)** — `needs-linux` `.ci` with static route + `fib{kernel{}}`, asserting kernel FIB readback. Use config-file + kernel readback (no SSH) so it also holds against the `ze_core` QEMU binary.
   - Verify: route present under QEMU; absent if the install path is broken.
5. **Full verification** → `make ze-precommit-verify` + `make ze-qemu-needs-linux-test` for AC-4.
6. **Mutation-verify each behavior-guarding test** (`ai/rules/testing.md`): break the always-on registration / FIB install, confirm the matching test flips red, revert.
7. **Complete spec** → audit tables, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has a named test that runs in a gate (unit / functional / qemu) |
| Discrimination | Each `.ci` targets `ze-stripped` / the `ze_core` QEMU binary and asserts a bgp-off-specific outcome; none passes vacuously on the full binary |
| No false completion | AC-4 actually reads back kernel state, not just exit 0 (`ai/rules/interop-and-goal-validation.md` data-correctness) |
| Graceful degradation asserted, not assumed | AC-5 observes the nil-guard path (`dump.go`) and empty enrichment, not merely "no crash" |
| No production drift | `git diff` shows no `internal/**` change unless a real defect was fixed (then a defect + fix note in Deviations) |
| Registration over hardcoding | presence guard reads the registry |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `build_tag_alwayson_present_test.go` | `GO_TEST_CORE ./cmd/ze/hub -run TestBuildTag_AlwaysOnPluginsPresent` green; and red when a blank import removed |
| 3 new `.ci` files | `ls test/plugin/bgp-off-*.ci`; run each via `ze-test` |
| AC-4 runs in QEMU | `make ze-qemu-needs-linux-test` includes it; paste the pass line |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | N/A — tests use fixed config, no untrusted input |
| Resource exhaustion | daemon test terminates the process in `finally` (model `ze-stripped-surface.ci`) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Daemon panics on boot with bgp off | Real defect — fix at the owning plugin (`ai/rules/completion.md`), add a Deviations row |
| AC-4 route absent | Trace sysrib→fib in the stripped composition; do NOT weaken the assertion |
| Test green on full binary too | R-1 — make the assertion bgp-off-specific |
| 3 fix attempts fail | STOP, report, ask user |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE -->
- The `wiring-at-commit` warning is advisory and fires on any `internal/plugins/**/*.go` commit lacking a `.ci`. For a pure import-requalification it is a false positive by the letter of `testing.md` — but here it correctly surfaced a real gap: the *new capability* the requalification unlocks (running with `ze_bgp` off) is unproven functionally. The lesson: read the warning as "what new reachability did this change enable?", not "does this exact hunk need a test?".

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Drive `ze-stripped` / the `ze_core` QEMU binary for functional proof | A new `.ci` build-tag selector to compile a bgp-off DUT per test | The stripped binary already exists, ships, and is on PATH with a precedent (`ze-stripped-surface.ci`); no runner change needed |
| Separate presence (Go `!ze_bgp`) from operation (`.ci`) | One big QEMU test | Presence is deterministic and dataplane-free; keeping it in the bare-`ze_core` unit pass makes regressions cheap to catch |
| MRT/flow-export proof bounded to presence + boot + graceful degradation | A "MRT dumps with bgp off" data test | MRT/flow-export enrichment are *inert* without a BGP source (`component.go,129`, `enrichbgp.go`); a data-production test would be vacuous or impossible |
| FIB gets the one data-producing functional test | Skip FIB install too | FIB is the only one of the three that does real work with bgp off (installs static/IGP routes via sysrib), so it is the meaningful "still runs" proof |

## Known Limitations
- P4 and VPP FIB *install* under bgp off are not functionally proven here (presence+schema only) — heavier harness; see Future row.
- AC-4 proves FIB install with a **static** route source (the only non-BGP route source in `ze-stripped`, which is `ze_core ze_ssh` — no ze_isis/ze_ospf). IGP-sourced install under bgp off is covered transitively by the full-binary IGP tests plus this bgp-independence proof.

## Implementation Summary

### What Was Implemented
- (fill during implementation)

### Bugs Found/Fixed
- (fill during implementation)

### Documentation Updates
- (fill during implementation)

### Deviations from Plan
- (fill during implementation)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| FIB/MRT/flow-export are PRESENT in a `ze_bgp`-absent build | functional (unit, `!ze_bgp`) | `TestBuildTag_AlwaysOnPluginsPresent` pass in bare-`ze_core` pass |
| Their config is reachable on the shipped stripped binary | functional (`.ci`) | `bgp-off-schema-stripped.ci` pass; `bgp{}` rejection |
| A bare build actually RUNS (boots healthy) with them configured | functional (`.ci`) | `bgp-off-boot-stripped.ci` pass |
| The FIB genuinely installs routes with bgp compiled out | functional / data-correctness (QEMU `.ci`) | `bgp-off-fib-install.ci` kernel-FIB readback |
| MRT/flow-export degrade gracefully (no BGP source) without crashing | functional (`.ci`) | `bgp-off-boot-stripped.ci` AC-5 assertions |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | | | |

### Fixes applied
-

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean
- [ ] `make ze-standard-test` passes (lint + all ze tests)
- [ ] `make ze-precommit-verify` passes; AC-4 verified under `make ze-qemu-needs-linux-test`
- [ ] No production behavior changed (or defect + fix documented in Deviations)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Mutation-verify each behavior-guarding test
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary + Audit filled
- [ ] Write learned summary to `plan/learned/NNN-fixit-bgp-off-plugin-functional.md`
- [ ] **Commit A:** tests + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-fixit-bgp-off-plugin-functional.md`

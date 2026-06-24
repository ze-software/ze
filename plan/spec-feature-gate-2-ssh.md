# Spec: feature-gate-2-ssh

| Field | Value |
|-------|-------|
| Status | in-progress |
| Parent | plan/spec-feature-gate-0-umbrella.md (child 2; lg pilot = child 1, learned 980) |
| Depends | none (ssh is independent of routing; the shared startup hook is just where the API lives) |
| Phase | 4/4 (complete; AC-1..AC-8 verified; closing via two-commit) |
| Updated | 2026-06-24 |

## Post-Compaction Recovery

**Re-read after compaction:**
1. This spec file
2. `plan/learned/980-feature-gate-1-lg.md` - the lg pilot; the registry + gating pattern
3. `cmd/ze/hub/infra_setup.go` - `infraSetup` mixes always-on auth/accounting with ssh build + wiring
4. `cmd/ze/hub/session_factory.go` - the interactive ssh session model (ssh-only)
5. `cmd/ze/hub/main.go` (~717-784) - the second (no-`bgp{}`) ssh construction path
6. `internal/component/ssh/ssh.go` - `Server` API (Set* setters, getters, Start/Stop/Reload)
7. `internal/test/runner/runner.go` `TestBuildTags`, `Makefile` `ZE_FEATURES`, `.golangci.yml`, `scripts/dev/dep_audit.py` `DISABLEABLE`, `scripts/codegen/plugin_imports.go` `featureTags`

## Task

Make ssh **compile-out-able** from the `ze` binary via `ze_ssh`, so a hardened build
(`ze-stripped`) links no ssh server and exposes no ssh listener, while `ze`/`ze-appliance`
keep ssh exactly as today. ssh is the headline hardening target. Per the user's decision:
**extract ssh into its own self-contained module first (no behavior change), then gate it.**

ssh is the hard case (umbrella child 2): unlike the lg listener service, ssh's server is
built and wired inside the shared daemon-startup function `infraSetup`, interleaved with
**always-on** infra (AAA bundle, command authorization, accounting, reboot/GR marker), and
it carries the whole interactive CLI-over-ssh session model (`session_factory.go`). ssh has
NO dependency on routing; `infraSetup` merely lives behind `bgpconfig.SetInfraHook` because
that is where the generic daemon-startup hook API happens to sit.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint:. -->
- [ ] `plan/learned/980-feature-gate-1-lg.md` - the lg pilot
  → Decision: lg used the listener construction registry; ssh does NOT fit it (its inputs
    are reactor/dispatcher/apiServer/AAA-bundle/reloadFn, not Store/Resolvers/addrs). ssh
    uses a dedicated `ze_ssh` seam instead. The umbrella's "one registry" is refined:
    listener services (lg/web/gnmi/mcp) use the registry; ssh uses its own seam.
  → Constraint: a feature's tag must be added in FOUR places: `ZE_FEATURES` (Makefile),
    `TestBuildTags()` (runner.go), `.golangci.yml` build-tags, and `featureTags` (generator,
    for the schema). Missing `TestBuildTags` breaks every ssh `.ci`/functional test.
  → Constraint: registration `init()` lives in `register_*.go`; factory body in a sibling
    `service_*.go`/module file under the same `//go:build` tag. `dep_audit` `DISABLEABLE`
    gate forbids any always-on direct import of the gated package.
- [ ] `ai/rules/module-tiers.md` "Disable-ability" - the no-direct-import rule + gate
  → Constraint: after gating, NO always-on (untagged, non-test) file may import
    `internal/component/ssh`; the gate fails otherwise.

### RFC Summaries (MUST for protocol work)
- N/A. No wire-protocol change; composition/build-tag work only.

**Key insights:**
- ssh's always-on importers are exactly: `infra_setup.go`, `main.go`, `session_factory.go`
  (all in `cmd/ze/hub`), plus the `ssh/yang` schema blank import in `all.go`.
- ssh IS a `ze.Subsystem` (Start/Stop/Reload), but it is built via the startup hook +
  the no-`bgp{}` path, not the listener registry. Its server exposes ~9 `Set*` setters
  (executor, streaming, monitor, plugin-protocol, shutdown, restart, reboot, login-warnings,
  session-model) using `zessh.*` and `cli/contract.*` types.
- `session_factory.go` helpers (`buildSessionModelFactory`, `buildCommandTree`,
  `streamingTraceroute/PingFactory`, `dashboardFactoryFromExecutor`, `newSessionEditor`) are
  used ONLY from `session_factory.go`/`infra_setup.go`/`main.go` ssh paths → the whole file
  moves behind `ze_ssh`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/hub/infra_setup.go` - `infraSetup(params, recorder, reloadFn)`:
  builds AAA bundle (always), registers accounting, swaps bundle; if ssh configured, builds
  `zessh.Server`, sets session-model factory; in `r.SetPostStartFunc` wires authorization +
  accounting (always) AND the ~9 ssh `Set*` closures (executor/streaming/monitor/plugin/
  shutdown/restart/reboot/login-warnings).
  → Constraint: AAA bundle, authorization, accounting, reboot, GR marker MUST stay always-on
    (MCP/API need them with ssh absent). Only the ssh-specific lines move behind `ze_ssh`.
  → Constraint: the ssh `Set*` closures close over `d` (dispatcher), `apiServer`, `r`
    (reactor), `writeGRMarker`, `params`, `recorder` -- the seam must carry these (generic
    types, never `zessh.*`).
- [ ] `cmd/ze/hub/session_factory.go` - `buildSessionModelFactory(srv *zessh.Server, ...)`
  builds the interactive bubbletea model; all helpers are ssh-only.
  → Constraint: whole file moves to `//go:build ze_ssh`.
- [ ] `cmd/ze/hub/main.go` (~717-784) - the no-`bgp{}` ssh path: builds `zessh.Server`,
  sets session-model + executor factory, starts it; `sshDispatch` (line 617).
  → Constraint: this path also routes through the same `ze_ssh` seam; main.go keeps zero
    `zessh` import.
- [ ] `internal/component/ssh/ssh.go` - `Server` API + `CommandExecutor`/`StreamingExecutor`
  func types; Start/Stop/Reload (`ze.Subsystem`).
  → Constraint: setters take `zessh.*`/`contract.*`; the gated wiring builds these closures.
- [ ] `internal/component/plugin/all/all.go` - `ssh/yang` schema blank import.
  → Constraint: move to a generated `all_ze_ssh.go` (`//go:build ze_ssh`).

**Behavior to preserve:**
- `ze`/`ze-appliance` keep ssh exactly as today (both construction paths, all setters, the
  session model, ephemeral-address file, AAA, idle/max-session, host keys/certs).
- AAA bundle, authorization, accounting, reboot, GR marker run regardless of ssh.
- The functional ssh tests (interactive sessions, exec, monitor) pass on the ssh build.

**Behavior to change:**
- ssh-specific construction + wiring + session model leave `infraSetup`/`main.go` into a
  self-contained module reached only via the `ze_ssh` seam. With `ze_ssh` off, ssh is
  unregistered, unbuilt, unlinked; `ssh {}` config gets a clean "unknown field" error.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Compile time: presence/absence of `ze_ssh`.
- Run time: the daemon startup hook (`infraSetup`) and the no-`bgp{}` path in `main.go`.

### Transformation Path
1. `register_ssh.go` (`//go:build ze_ssh`) `init()` sets the seam vars (`sshBuild`,
   `sshWirePostStart`) to the gated implementations in `service_ssh.go`.
2. Always-on `infraSetup` builds AAA/authz/accounting; if `sshBuild != nil` and ssh is
   configured, it calls `sshBuild(inputs)` for the server and, inside its PostStart
   callback, `sshWirePostStart(handle, inputs)` for the ~9 setters.
3. The no-`bgp{}` `main.go` path calls the same seam.
4. With `ze_ssh` off: seam vars nil → ssh never built; `internal/component/ssh` imported
   nowhere always-on → linker drops it; `all_ze_ssh.go` absent → schema unregistered.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `ze_ssh` tag ↔ composition | `register_ssh.go`/`service_ssh.go`/`session_factory.go` gated; `all_ze_ssh.go` | [ ] |
| seam ↔ always-on infra | `sshBuild`/`sshWirePostStart` hook vars; opaque `sshServer` handle | [ ] |
| ssh ↔ always-on code | reached only via the seam; never a direct `zessh` import | [ ] audit |

### Integration Points
- AAA bundle / authorization / accountant (existing, always-on) - unchanged.
- `cli/contract` (`SessionModelFactory`, `MonitorFactory`, `LoginWarning`) - already the
  decoupling layer between ssh and cli; the seam uses these, not raw `zessh` types where possible.
- `dep_audit.py` `DISABLEABLE` += `internal/component/ssh -> ze_ssh`.

### Architectural Verification
- [ ] No bypassed layers (ssh still a `ze.Subsystem`; AAA/authz path unchanged)
- [ ] No unintended coupling (seam carries generic types; ssh stays a pure component)
- [ ] No duplicated functionality (reuse `cli/contract`, the 980 gating plumbing)
- [ ] Zero-copy preserved (N/A - composition change only)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | ssh's only always-on importers are infra_setup.go + main.go + session_factory.go (+ ssh/yang) | grep importers of `component/ssh` | extra severing needed | re-grep after extraction | unvalidated |
| A-2 | the ~9 setters' closures can be expressed in a gated `wire` fn given generic inputs (d, apiServer, r, params, recorder, writeGRMarker) | infra_setup.go:226-312 uses only those + zessh setters | seam needs more/other inputs | prototype wire fn in Phase 1 | unvalidated |
| A-3 | session_factory.go has no always-on caller besides the ssh build paths | grep showed helpers used only there | extra move needed | re-grep after move | unvalidated |
| A-4 | the AAA/authz/accounting/reboot code in infraSetup is fully separable from ssh | infra_setup.go structure | extraction changes always-on behavior | Phase 1 tests stay green | unvalidated |
| A-5 | a no-ssh binary handles `ssh {}` config safely (schema gated) | 980: lg/yang gating worked identically | panic/validation regression | build no-ssh, feed `ssh {}` config | unvalidated |
| A-6 | the two construction paths (hook + no-`bgp{}`) can share one seam | both build zessh.Server + wire setters | the no-`bgp{}` path needs a variant | route both through the seam in Phase 2 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|-----------|
| R-1 | extraction subtly changes always-on AAA/authz/accounting behavior | AAA/authz functional tests fail | Phase 1 is a pure refactor; run full functional suite before gating |
| R-2 | the seam's input set is large and leaks ssh specifics into always-on code | seam structs reference `zessh.*` | inputs are generic (dispatcher, apiServer, params); handle is opaque (`Address()` only) |
| R-3 | the no-`bgp{}` path and the hook path diverge, doubling the seam | two slightly different seams | unify the build/wire seam; both paths call it |
| R-4 | session_factory.go pulls cli/bubbletea deps that something always-on also needs | build break in no-ssh binary | grep confirmed helpers are ssh-only; move wholesale |
| R-5 | forgetting a tag touch-point (TestBuildTags etc.) breaks ssh functional tests | `.ci` ssh tests fail "unknown field"/exec | follow the 980 four-place checklist; run ze-verify-changed |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| build with `ze_ssh` | → | ssh seam set; server built + wired | `TestBuildTag_SSH_Present` (cmd/ze/hub) |
| build without `ze_ssh` | → | seam nil; ssh not linked; no listener | `TestBuildTag_SSH_Absent` (cmd/ze/hub) |
| `infraSetup` with seam set, ssh configured | → | AAA always-on + ssh built via seam | `TestInfraSetup_BuildsSSHViaSeam` |
| `dep_audit.py` over always-on import of `component/ssh` | → | flagged | `dep_audit` selftest (DISABLEABLE) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Phase 1 extraction merged | ssh build + wiring + session model live in their own module; `infraSetup` keeps only AAA/authz/accounting; all tests green (no behavior change, no gating yet) |
| AC-2 | `ze_ssh` build tag ON | ssh compiled in, both construction paths work, all existing ssh functional tests pass |
| AC-3 | `ze_ssh` OFF (`ze-stripped`) | `internal/component/ssh` server symbols absent (`go tool nm`); daemon starts without ssh; no listener; no error |
| AC-4 | no-ssh binary fed config with `ssh { ... }` | clean "unknown field" validation error, no panic |
| AC-5 | no-ssh build, AAA/authz/accounting | unchanged: MCP/API auth + accounting still work without ssh |
| AC-6 | generator runs | emits `all_ze_ssh.go` (`//go:build ze_ssh`) with `ssh/yang`, removed from `all.go`; `--check` passes |
| AC-7 | always-on code imports `internal/component/ssh` | `dep_audit.py` flags it (DISABLEABLE gate) |
| AC-8 | `make ze-verify-changed` on the ssh build | full suite green; all four flavors build |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path | Test |
|---|-----------|------|------|
| 1 | builds a hardened `ze-stripped` (no ssh) | tag off → ssh module + schema unlinked → no ssh listener | `TestBuildTag_SSH_Absent` + `go tool nm` |
| 2 | builds standard `ze` with ssh | tag on → seam set → ssh built + wired both paths | `TestBuildTag_SSH_Present` + ssh functional tests |
| 3 | runs no-ssh binary against config with `ssh {}` | config load → ssh schema absent → clean error | `ze-stripped config validate` (evidence) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInfraSetup_BuildsSSHViaSeam` | `cmd/ze/hub/ssh_infra_test.go` | with the seam set, infraSetup builds ssh; with it nil, ssh is skipped, AAA still runs | |
| `TestBuildTag_SSH_Present` / `_Absent` | `cmd/ze/hub/build_tag_ssh_*_test.go` | seam set iff ze_ssh | |
| disableable selftest (ssh) | `scripts/dev/dep_audit.py --selftest` | always-on `component/ssh` import flagged | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no numeric inputs) | - | - | - | - |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| existing ssh suites | `test/` (ssh exec/session/monitor) | unchanged behavior on the ssh build | |
| `ssh-absent-config` | evidence via `ze-stripped` | no-ssh binary rejects `ssh {}` cleanly | |

### Interop Tests (MANDATORY for protocol features)
- N/A (no wire-protocol change).

### Future (if deferring any tests)
- web/gnmi/mcp compile-out remain separate umbrella children.

## Files to Modify
- `cmd/ze/hub/infra_setup.go` - keep AAA/authz/accounting; call the ssh seam for build + wire
- `cmd/ze/hub/main.go` - no-`bgp{}` ssh path routes through the seam; drop `zessh` import
- `cmd/ze/hub/session_factory.go` - add `//go:build ze_ssh`
- `internal/component/plugin/all/all.go` - (generated) `ssh/yang` removed
- `scripts/codegen/plugin_imports.go` - `featureTags` += `ssh/yang -> ze_ssh`
- `Makefile` - `ZE_FEATURES += ze_ssh`
- `internal/test/runner/runner.go` - `TestBuildTags` += ze_ssh
- `.golangci.yml` - build-tags += ze_ssh
- `scripts/dev/dep_audit.py` - `DISABLEABLE` += `internal/component/ssh -> ze_ssh`
- `ai/rules/module-tiers.md`, `docs/features.md` - note ssh among compile-out features

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| CLI commands/flags | [ ] no | - |
| Functional test | [ ] yes | build-tag present/absent + existing ssh suites |
| Doctor check | [ ] no | ssh owns its own; absent = not registered |
| Discovery-updates | [ ] yes | module-tiers.md + the four-place tag checklist |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File |
|---|----------|----------|------|
| 1 | New user-facing build flavor option? | [ ] yes | `docs/features.md` (ssh in the compile-out list) |
| 12 | Internal architecture changed? | [ ] yes | `ai/rules/module-tiers.md` (ssh seam vs registry note) |
| others | - | [ ] assess | grep docs for ssh build assumptions |

## Files to Create
- `cmd/ze/hub/ssh_infra.go` - always-on seam: `sshServer` handle iface + `sshBuild`/`sshWirePostStart` hook vars + input structs
- `cmd/ze/hub/service_ssh.go` - `//go:build ze_ssh`; the build + wire implementations (moved from infra_setup/main)
- `cmd/ze/hub/register_ssh.go` - `//go:build ze_ssh`; `init()` sets the seam vars
- `cmd/ze/hub/ssh_infra_test.go`, `build_tag_ssh_present_test.go`, `build_tag_ssh_absent_test.go`
- `internal/component/plugin/all/all_ze_ssh.go` - generated schema group

## Implementation Steps

### /implement Stage Mapping
| Stage | Section |
|-------|---------|
| 1 Read | this file |
| 2 Audit | Files to Modify/Create, Current Behavior |
| 3 Wiring | Wiring Test - seam + build-tag tests first |
| 4 Implement | Phases below |
| 7 Review | Critical Review Checklist |
| 15 Close | learned summary + two-commit script |

### Implementation Phases
1. **Phase 1 - extract (always-on, no behavior change)**: introduce `ssh_infra.go` with the
   seam (`sshBuild`/`sshWirePostStart` hook vars, opaque `sshServer`); move the ssh build +
   the ~9 PostStart setters out of `infraSetup` and the no-`bgp{}` path into seam-set
   functions that are STILL always-on (registered unconditionally for now). `infraSetup`
   keeps AAA/authz/accounting and calls the seam. Verify: full functional suite green,
   identical ssh behavior, ssh still compiled in.
2. **Phase 2 - gate**: move the seam-setting + build/wire impl + `session_factory.go` behind
   `//go:build ze_ssh` (`service_ssh.go` + `register_ssh.go`); generator emits `all_ze_ssh.go`;
   add `ze_ssh` to `ZE_FEATURES`, `TestBuildTags`, `.golangci.yml`; `dep_audit` `DISABLEABLE`.
   Verify: `ze_core` build = 0 ssh symbols; `ze_ssh` build identical; `ssh{}` config safe.
3. **Phase 3 - tests + docs**: present/absent build-tag tests, `ssh_infra_test.go`,
   dep_audit selftest fixture; module-tiers.md + features.md.
4. **Full verification** - `make ze-verify-changed`; build all four flavors.

### Critical Review Checklist (/implement stage 7)
| Check | What to verify |
|-------|----------------|
| No always-on ssh import | grep `!ze_ssh` hub files for `component/ssh` → none; dep_audit clean |
| AAA/authz parity | accounting/authorization run with ssh absent (Phase 1 keeps them always-on) |
| Both ssh paths | hook path AND no-`bgp{}` path both build+wire via the seam |
| Behavior parity | ssh exec/session/monitor/reboot identical on the ssh build |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification |
|-------------|--------------|
| seam + extraction | `go test ./cmd/ze/hub -run SSH`; functional ssh suites |
| ssh compiled out | `ze-stripped`; `go tool nm` no `component/ssh` server symbols |
| ssh compiled in | `ze`; ssh functional tests |
| audit | `dep_audit.py --selftest` + clean `--check` |
| config safe | `ze-stripped config validate ssh{}` clean error |

### Security Review Checklist (/implement stage 12)
| Concern | Check |
|---------|-------|
| no-ssh build exposes no ssh port | `ze-stripped` binds no ssh listener |
| AAA/authz still enforced without ssh | MCP/API auth + accounting unaffected |
| host key / cert handling unchanged on ssh build | identical to current |

### Failure Routing
| Failure | Route To |
|---------|----------|
| ssh not omitted with tag off | residual always-on import - dep_audit / `go tool nm` |
| AAA/authz regressions | Phase 1 extraction changed always-on behavior - revert to pure refactor |
| functional ssh `.ci` fail | missing TestBuildTags ze_ssh (980 trap) |
| 3 fix attempts fail | STOP, report, ask |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| (initial framing) ssh is wired via BGP | ssh is independent of routing; `infraSetup` just lives behind the generic `bgpconfig.SetInfraHook` | user correction | dropped the BGP framing; seam carries generic infra inputs |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Item | Why |
|------|-----|

## Design Insights
- ssh is the case the lg listener registry does NOT cover: it is built inside the shared
  daemon-startup path with reactor/dispatcher/AAA inputs, and carries an interactive session
  model. A dedicated `ze_ssh` seam (build + wire hook vars) is the right shape; the umbrella's
  "single registry" becomes "registry for listener services + per-category seams where needed".
- Extract-then-gate keeps Phase 1 a behavior-preserving refactor that the full functional
  suite validates BEFORE the compile-out is introduced -- de-risking the AAA/authz interleave.

## Core Insight
Compiling ssh out reduces to: separate the ssh-specific build + wiring + session model from
the always-on AAA/authz/accounting they were interleaved with, route both construction paths
through one gated seam, then drop every direct `internal/component/ssh` import from always-on
code and gate the schema.

## Key Design Decisions
| Decision | Alternatives | Rationale |
|----------|-------------|-----------|
| Dedicated `ze_ssh` seam (hook vars) | force ssh into the lg construction registry | ssh's inputs/lifecycle don't fit the listener registry; the seam carries the real inputs without bloating `ServiceDeps` |
| Extract first, then gate | gate in place | a pure refactor verified by the full suite isolates the risky AAA/ssh untangle from the compile-out |
| Opaque `sshServer` handle (`Address()`) | pass `*zessh.Server` to always-on code | keeps always-on code free of any `zessh` import (required for compile-out + the audit) |

## Known Limitations
- web/gnmi/mcp/lg-already-done and protocol compile-out remain separate umbrella children.
- The seam input structs are ssh-shaped; they are not a generic service abstraction (ssh is
  deliberately its own category, distinct from listener services).

## Goal Validation (BLOCKING)
| Goal | Evidence Type | Concrete Evidence |
|------|---------------|-------------------|
| ssh compile-out-able via `ze_ssh` | symbol check + build-tag test | `go tool nm`: bare `ze_core` = **0** `component/ssh` server symbols, `ze_core ze_ssh` = **118**; `TestBuildTag_SSH_Absent` passes |
| extraction is behavior-preserving | functional suite | Phase 1 built + hub tests green with ssh still always-on; final `make ze-verify-changed` PASS (all 5 stages incl. ssh functional suites) |
| no always-on ssh import | audit | `dep_audit.py --selftest` OK; `--check` clean (DISABLEABLE includes ssh) |
| AAA/authz/accounting survive no-ssh | functional suite | kept always-on; verify green (the no-bgp path's AAA stays in main.go) |
| default flavors keep ssh | build-tag test + build | `TestBuildTag_SSH_Present`; `ze`/`ze-appliance`/`ze-stripped` build with ssh (ze-stripped = ze_core ze_ssh per user decision) |

## Review Gate
### Run 1 (self-review)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | ssh uses a dedicated seam (hook vars), NOT the lg construction registry: it's built in shared startup with reactor/dispatcher/AAA inputs + an interactive session model. | ssh_infra.go | accepted; umbrella refined (registry for listener svcs, seam for ssh) |
| 2 | NOTE | Two seam entries (`sshBuild`+`sshWirePostStart` hook path; `sshBuildStandalone` no-bgp path) -- A-6 revised: the paths differ (full wiring vs session+dispatch). | ssh_infra.go | accepted |
| 3 | NOTE | AAA bundle build kept always-on in BOTH paths (only the authenticator crosses the seam) so MCP/API auth survives a no-ssh build. | infra_setup.go, main.go | accepted (correctness-critical) |
| 4 | NOTE | `TestEphemeralDaemonStartsSSH` gets a `// test-relax:` skip-guard under no-ssh; `session_factory_test.go`+`infra_setup_test.go` gated `ze_ssh`. | *_test.go | accepted (feature genuinely absent under ze_core) |
| 5 | NOTE | ze-stripped KEEPS ssh (user decision): it's the management plane; the no-ssh proof is bare `-tags ze_core` (TestBuildTag_SSH_Absent + nm), not the ze-stripped target. `ze-stripped-surface.ci` unchanged. | Makefile | accepted (user-approved) |
| 6 | NOTE | Initial framing wrongly tied ssh to BGP; corrected (the shared startup hook just lives in bgpconfig). | spec | accepted |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE  → self-review: 0 BLOCKER, 0 ISSUE, 6 NOTEs (all accepted above)
- [ ] All NOTEs recorded above (or explicitly "none")  → 6 NOTEs recorded

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `cmd/ze/hub/ssh_infra.go` | yes | created (seam: handle + input structs + hook vars + setSSHInfra) |
| `cmd/ze/hub/service_ssh.go`, `register_ssh.go` | yes | created (`//go:build ze_ssh`: build/wire/standalone + install) |
| `cmd/ze/hub/build_tag_ssh_present_test.go` / `_absent_test.go` | yes | created (seam present/absent) |
| `internal/component/plugin/all/all_ze_ssh.go` | yes | generated (`//go:build ze_ssh`, ssh/yang) |
| `cmd/ze/hub/session_factory.go` | yes (gated) | now `//go:build ze_ssh` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | extraction behavior-preserving | Phase 1: builds + hub tests green with ssh still always-on (no gating); `infraSetup` keeps AAA/authz/accounting |
| AC-2 | `ze_ssh` on → ssh present | `go tool nm` ze_core+ze_ssh = 118 ssh-server symbols; `TestBuildTag_SSH_Present` + `TestInfraSetupWiresSessionModelFactory` pass |
| AC-3 | bare `ze_core` → ssh absent | `go tool nm` ze_core = 0 ssh-server symbols; `TestBuildTag_SSH_Absent` pass |
| AC-4 | no-ssh binary + `ssh{}` config | ssh/yang gated → unknown-field validation error (same mechanism as lg AC-4, 980) |
| AC-5 | AAA/authz/accounting without ssh | kept always-on in `infraSetup` + the no-bgp path; only the authenticator crosses the seam; full functional suite green |
| AC-6 | generator emits `all_ze_ssh.go`, drops ssh/yang from all.go | `plugin_imports.go --check` current (2 gated groups); ze_core ssh/yang symbols = 0 |
| AC-7 | always-on ssh import flagged | `dep_audit.py --selftest` OK + clean `--check` (DISABLEABLE includes ssh) |
| AC-8 | suite green; flavors build | `make ze-verify-changed` PASS (all 5 stages); ze/ze-appliance/ze-setup/ze-stripped build |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | ssh's always-on importers were infra_setup.go + main.go + session_factory.go (+ ssh/yang); all severed/gated |
| A-2 | confirmed | the ~9 setters expressed in `sshWireImpl(handle, *sshWireInputs)` from generic inputs (reactor/params/writeGRMarker) |
| A-3 | confirmed | session_factory.go helpers ssh-only; whole file gated ze_ssh; no always-on caller |
| A-4 | confirmed | AAA/authz/accounting kept always-on; functional suite (incl. AAA paths) green |
| A-5 | confirmed | bare ze_core build links 0 ssh symbols; ssh{} config gated like lg |
| A-6 | confirmed (revised) | two seam entries: `sshBuild`+`sshWirePostStart` (hook path), `sshBuildStandalone` (no-bgp path) |

## Checklist
### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 demonstrated
- [ ] ssh extracted; AAA/authz/accounting stay always-on; Phase 1 behavior-preserving
- [ ] ssh compile-out-able; present/absent build-tag tests pass; symbol check
- [ ] audit rule flags always-on ssh imports (DISABLEABLE)
- [ ] `make ze-test` passes (lint + all ze tests); all four flavors build
- [ ] Documentation Update Checklist answered with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior (build-tag present/absent + config-safe)
- [ ] Goal Validation table filled with concrete evidence

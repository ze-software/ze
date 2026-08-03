# Spec: improve-7 -- YANG Handler-Completeness Gate

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-improve-0-umbrella.md` -- set context and comparison-honesty scope
4. `internal/component/config/reader.go` -- handler dispatch (walkMap, findHandler)
5. `plan/spec-improve-6-yang-coverage.md` -- sibling report tool; scope boundary below

## Task

Ze's config path silently tolerates schema subtrees that no plugin claims.

~~Original framing (superseded): the reader routes config blocks to handlers by
longest-prefix match (`findHandler`, `internal/component/config/reader.go`)
and recurses past handler-less blocks, dropping their flat leaves.~~
**Corrected during research (2026-07-10):** that reader is test-only --
`config.NewReader` (`reader.go`) has no non-test caller (grep verified this
session). The silent tolerance lives in the PRODUCTION config path:

- Delivery is claimed per top-level root: `Server.reloadConfig` selects plugins by
  `reg.WantsConfigRoots` with `rootHasChanges`
  (`internal/component/plugin/server/reload.go`). When no plugin claims any
  changed root, the producer at `reload.go` logs Info ("config reload: no
  affected plugins, updating config") and stores the tree via `SetConfigTree` --
  accepted, never verified by a plugin, never delivered.
- Validation is permissive where it does run: `validateContainerEntry` validates
  only data keys present in the schema dir
  (`internal/component/config/yang/validator.go`; the `if child, ok :=
  entry.Dir[key]; ok` guard at `:527` has no else); unknown keys pass silently.
- Three hand-maintained inventories exist with nothing tying them together:
  registered YANG modules (generated `configyang.RegisterModule` glue,
  `scripts/codegen/yang_glue.go`), hub `Schema.Handlers` (hand-declared
  lists, e.g. bgp -> ["bgp","bgp/peer"] at
  `internal/component/config/schema/cli/main.go`, nil for most internal plugins
  `:610`), and plugin `ConfigRoots`/`WantsConfigRoots`
  (`internal/component/plugin/registry/registry.go`).

Net effect: an operator can configure a subtree that parses cleanly, validates
nowhere that rejects it, and is delivered to no component. Config accepted but
ignored -- the recurring "feature not wired" defect class, on the config surface.

The comparison-review daemon (Holo) refuses to start if any schema node lacks its
required callback (verified this session against primary source:
`holo-daemon/src/northbound/core.rs:815-849`). Adopt the same guarantee shaped for
Ze's registry architecture: a mechanical gate that resolves the full YANG config
schema and fails when any config subtree is claimed by no delivery surface
(plugin `WantsConfigRoots`, hub `Schema.Handlers`), plus the inverse (claims naming
no schema node). Explicit allowlist for deliberate no-ops (with reason), so the gate
stays honest without blocking legitimate structure-only containers. Wire into the
existing `scripts/checks` family and verify stages. Design decides: static check vs
startup enforcement (at the point where `SchemaRegistry`, registrations, and the
resolved loader tree coexist, after `SubsystemManager.AllSchemas`,
`internal/component/plugin/server/subsystem.go`) vs both; and the claim
granularity (per-root vs deeper).

**Scope boundary vs improve-6:** improve-6 is a *report* (per-module node counts,
constraint grading, ownership/gating tables, advisory check mode). improve-7 is the
*blocking gate* for exactly one property: every config schema node is claimed by a
handler. improve-6 must not duplicate handler-claim enforcement; improve-7 must not
grow reporting features. (Same pattern as the port-defaults carve-out recorded in
improve-6's post-wave corrections.)

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `ai/rules/repo-maintenance.md` - gate must be discoverable and wired into verify
  → Constraint: new gate + doctor check + any make target must land WITH ai/INDEX.md row, hook/gate mapping row, named verification, and doc page in the same work; unlinked tooling is banned (Mechanical Checklist answered below in Design Insights)
- [ ] `ai/rules/evidence.md` - node inventory and handler set both derive live
  → Constraint: no second list of handlers or nodes; both sides derive from the same producers the daemon uses (registry via plugin/all import, modules via loader)
- [ ] `ai/rules/repo-maintenance.md` - where the check registers among existing gates
  → Constraint: read (2026-07-10): changing/adding checks requires satisfying discovery-updates so agents find them; new gate gets its row in the mapping docs when implemented
- [ ] `docs/architecture/config/yang-config-design.md` - module load/resolve semantics
  → Constraint: (via improve-6 digest) four module categories (type-lib/extensions/config-schema/API-schema) -- gate checks config-schema modules only; walk the RESOLVED Entry tree (GetEntry), never raw modules
- [ ] `ai/rules/plugins.md` - allowlist must not become central plugin knowledge
  → Constraint: allowlist is the gate's own fixture keyed by schema path + reason; it must never grow per-plugin fields, switches, or knowledge in core/shared packages

### RFC Summaries (MUST for protocol work)
- Not protocol work; YANG semantics come via goyang.

**Key insights:** (summary of all checkpoint lines -- minimal context to resume after compaction)
- PRODUCTION claim granularity is per top-level root (`reg.WantsConfigRoots` matched
  by `rootHasChanges`, `reload.go`) -- coarser than Holo's per-node
  callbacks. The reader.go longest-prefix path is test-only (no non-test caller of
  `config.NewReader`, grep verified). Gate v1 models root-level claiming; per-leaf
  "does the plugin consume it" is improve-6 reporting territory.
- No existing check ties YANG nodes to config claiming. Closest precedent:
  `scripts/docvalid/commands.go` OrphanYANG gates YANG *commands* (WireMethod) with
  no RPC handler and exits 1 (`:194-199`, `:112/:119`) -- same shape, different
  surface (research agent, verified citations).
- Two delivery paths must both be modeled or one declared SSOT: server reload by
  WantsConfigRoots (`reload.go`) and hub two-phase `Hub.ProcessConfig` ->
  RouteCommand/RouteCommit via registry.FindHandler
  (`internal/component/plugin/server/hub.go`, research agent).
- New-check wiring recipe: `scripts/checks/<name>.go` (`//go:build ignore`), make
  target near `Makefile:310-329` (selftest first), and both stage slices in
  `scripts/status/verify_run.go` stagesForMode (`:127-131`/`:141-145`) (research
  agent; port_defaults precedent verified in improve-6 digest).
  ~~append to `_ze-verify-impl` `Makefile:287` + `_ze-verify-changed-impl` `:294`~~
  corrected 2026-07-10: those Makefile targets are documented dead with zero callers
  (`Makefile:280-287` comment, per improve-4 post-wave corrections); a stage added
  only there never runs -- stagesForMode is the ONLY live stage list. (Less critical
  for this spec since the gate is a plugin/all unit test, but the doctor .ci and any
  future check-mode wiring must follow the corrected recipe.)

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/config/reader.go` - TEST-ONLY path: `NewReader` (:236) has no non-test caller (grep verified this session); `walkMap` recursion (:372-383) and `findHandler` longest-prefix (:433-455) exhibit the silent-tolerance pattern but do not run in production
  → Decision: gate targets the production claiming surfaces, not reader.go; reader.go is a dead-code candidate to surface to the user (never delete without asking)
- [ ] `internal/component/plugin/server/reload.go` - `reloadConfig` selects affected plugins by `reg.WantsConfigRoots` + `rootHasChanges` (:214-248); zero affected -> Info log + `SetConfigTree`, no verify/delivery (:251-256) (read this session)
  → Constraint: the gate's claim model must match rootHasChanges root-granularity or it will invent claims the server does not honor
- [ ] `internal/component/config/yang/validator.go` - `validateContainerEntry` (:509-542) checks mandatory children and validates only keys found in `entry.Dir` (:527, no else); unknown keys silently pass (read this session)
  → Constraint: validation cannot be relied on to catch unclaimed/unknown config; the gate is the only line of defense
- [ ] `internal/component/plugin/server/hub.go` - `Hub.ProcessConfig` two-phase RouteCommand/RouteCommit via registry.FindHandler (:93-130) (research agent)
  → Constraint: hub-routed subsystems claim via hand-declared `Schema.Handlers` (`schema/cli/main.go`, `:603`, nil for most at `:610`) -- second claiming surface the gate must model or exclude by decision
- [ ] `internal/component/config/yang/loader.go` - `DefaultLoader` best-effort resolve (:20-28), `LoadEmbedded` bootstrap set (:48-52) (citations carried from improve-6, re-verified 2026-07-10 per its post-wave note); `GetEntry` resolved tree (:111-117), `ModuleNames` (:120-126) (improve-6 research agent)
- [ ] `scripts/checks/command_ownership.go` + `port_defaults.go` - check-family structure, exit codes, make + verify wiring (research agent; digest in tmp/session/session-state-improve-6-yang-coverage-56997.md)
- [ ] `scripts/docvalid/commands.go` - OrphanYANG precedent: YANG commands with no RPC handler gate with exit 1 (:194-199, :112/:119) (research agent)

**Behavior to preserve:** (unless user explicitly said to change)
- Reader dispatch semantics unchanged: this spec adds a gate, it does not change
  walkMap/findHandler behavior.
- Existing `scripts/checks` exit-code contract unchanged.

**Behavior to change:** (only if user explicitly requested)
- A config schema node claimed by no handler becomes a build/verify failure (today:
  silent). Startup enforcement is a design decision, not yet committed.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Check mode: `scripts/checks`-family binary run by make/verify.
- (design decision) Startup mode: daemon init where loader + registered handlers coexist.

### Transformation Path
1. Resolve full YANG module set via the same loader the daemon uses (config-schema
   category only; type-lib/extensions/API-schema modules excluded per
   `docs/architecture/config/yang-config-design.md` module categories).
2. Enumerate config roots/subtrees from the resolved Entry tree.
3. Enumerate claiming surfaces: plugin `ConfigRoots`/`WantsConfigRoots`
   (registration literals) and hub `Schema.Handlers` (design decides inclusion).
4. Apply the production claim semantics (root-granular `rootHasChanges` matching) to
   every schema root; collect unclaimed subtrees AND claims naming no schema node.
5. Subtract allowlisted paths (each with a recorded reason); fail non-zero on remainder.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Loader ↔ gate | read-only walk of resolved modules | [ ] |
| Handler registry ↔ gate | same producer as NewReader's schemas slice | [ ] |
| Gate ↔ verify | scripts/checks exit-code contract | [ ] |

### Integration Points
- Plugin registry read APIs: `ConfigRootsMap()` (`registry.go`, only plugins with declared roots -- re-read this session), `All()` -- claim inventory.
- Loader read APIs: `ModuleNames()`/`ConfModuleNames` (`loader.go`), `GetEntry()` resolved tree (`loader.go`) -- schema inventory.
- `SchemaRegistry` handlers map (`schema.go` findHandlerIn) -- hub claim surface.
- `feature-gates.txt` manifest -- gating cross-check (AC-6).
- Doctor registration + `internal/core/diagnostic/codes.go` -- runtime surface (AC-7).

### Architectural Verification
- [ ] No bypassed layers (gate reads via loader + handler producer, no private parsing)
- [ ] No unintended coupling (gate depends on read APIs only)
- [ ] No duplicated functionality (improve-6 reports; improve-7 gates; port-defaults owns port consistency)
- [ ] Registration over hardcoding -- handler inventory derives from registration; no hardcoded lists (`ai/rules/plugins.md`, `ai/rules/evidence.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Claiming surfaces are statically derivable (ConfigRoots from registration literals via AST or registry import; modules via generated register.go glue) | `command_ownership.go` already AST-parses registration calls; `yang_glue.go` derives module inventory | Static check impossible; gate must run at daemon startup (after `AllSchemas`, `subsystem.go`) | Prototype enumeration during design | unvalidated |
| A-2 | Current unclaimed-subtree count is small enough to burn down or allowlist | config-surface rules enforced for a while | Gate starts advisory with a dated allowlist burn-down | First full gate run during implementation | unvalidated |
| A-3 | ~~Longest-prefix claiming is the only delivery path~~ BROKEN as originally stated: reader.go is test-only. Restated: server reload (`WantsConfigRoots`, `reload.go`) and hub `ProcessConfig` (`hub.go`) are the ONLY production delivery paths a claim can arrive through | Research agent end-to-end trace, spot-verified this session (reload.go read directly) | A third delivery path would produce false positives; gate blocks nodes that ARE consumed | Design-phase grep for other `WantsConfigRoots`/`FindHandler` consumers | unvalidated (restated) |
| A-4 | Hub `Schema.Handlers` claiming can either be included accurately or excluded with a recorded reason without neutering the gate | hand-declared lists exist (`schema/cli/main.go,:603,:610`) | Gate has a blind spot on hub-routed subsystems; scope shrinks | Design-phase inventory of which subsystems are hub-routed | confirmed: BGP is the sole non-nil registrant (`getInternalYANG` returns ["bgp","bgp/peer"] at `main.go`, nil for all others `:607-610`; consumer `Hub.RouteCommand`->`SchemaRegistry.FindHandler` `hub.go`, `schema.go`) -- include both surfaces, cost is one prefix union |
| A-5 | `ze-unit-test` runs the plugin/all package under the full feature-tag set (or the gate can arrange full-tag enumeration) | all_test snapshots cover the full registry today | Feature-gated modules escape the gate (R-5) | Read Makefile/mk tag wiring during phase 1; assert module count vs feature-gates manifest | unvalidated |
| A-6 | Phase-2 "mention" heuristic (leaf-name string literal in owning plugin package) has acceptable signal on real plugins | hand-parse pattern uses literal map keys universally (decode research: `as112/config.go`, `isis/config.go`) | Phase 2 report is noise; drop to improve-6 follow-up | Prototype on 3 plugins during phase 4; measure false-positive rate | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Allowlist rots into a dumping ground that neuters the gate | allowlist grows in PRs without reasons | every entry requires a reason + owner; test fails on reason-less entries (AC-5) |
| R-2 | Claim modeling diverges from the production matcher over time | gate passes but a root is silently unrouted (or inverse false positives) | gate models `rootHasChanges` root-granularity and hub `findHandlerIn` prefixes; where possible import the same helpers, never reimplement matching logic |
| R-3 | Overlap creep with improve-6's orphan detection | both specs list the same AC | scope boundary paragraph in both specs; improve-6 carve-out row |
| R-4 | Phase-2 mention-check heuristic noise (shared literals, dynamically built keys, leaf names matching unrelated strings) | report entries disputed in review | advisory only, never wired into blocking verify; allowlist with reasons; "mention" defined mechanically (kebab leaf-name string literal in the owning plugin package or its yang/ sibling, found via go/parser BasicLit scan) |
| R-5 | Gate runs under a tag set that compiles feature-gated modules out, silently shrinking the checked surface | enumerated module count diverges from `feature-gates.txt`-derived expectation | AC-6 cross-check against the manifest + `internal/**/yang` dir discovery (`yang_glue.go` semantics); fail on unexplained absence |
| R-6 | Hosting the gate in `internal/component/plugin/all` couples it to composition-root regen | config_claims_test fails right after `make generate` | acceptable and intended -- the registry snapshot tests already live with this (`all_test.go`); regen keeps the claim inventory current |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-unit-test` (runs plugin/all package tests) | → | claim-union builder + root comparison | TestConfigSchemaRootsClaimed |
| `make ze-unit-test` | → | phantom-claim inverse check | TestConfigRootsPhantomClaims |
| `ze doctor` | → | unclaimed-roots doctor check + diagnostic code | test/plugin/doctor-config-claims.ci |
| `make ze-yang-leaf-mentions` (advisory, phase 2) | → | leaf mention-check report | TestYANGLeafMentionReport (self-test fixture) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config-schema module whose top-level root is claimed by no plugin `WantsConfigRoots` and no hub handler prefix (fixture registration) | Test fails naming module, root, and the nearest existing claim |
| AC-2 | Root claimed by a plugin's `WantsConfigRoots`, or covered by a hub handler prefix (BGP "bgp"/"bgp/peer") | Not reported |
| AC-3 | `ConfigRoots` entry naming no config-schema root (phantom claim, fixture) | Test fails naming plugin and phantom root |
| AC-4 | Allowlist entry with reason | Skipped, listed as allowlisted in failure-free output |
| AC-5 | Allowlist entry without reason | Test fails |
| AC-6 | Module compiled out by a feature tag (per `feature-gates.txt`) when gate runs under the full tag set | Enumerated and checked; never silently absent |
| AC-7 | Daemon runs with a config root no live plugin claims | `ze doctor` reports it with a registered diagnostic code (upgrades the `reload.go` Info silence) |
| AC-8 (phase 2, advisory) | YANG leaf under a claimed root whose kebab name appears nowhere in the owning plugin package source | Listed in the advisory mention-check report with module, leaf path, owning plugin |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Agent adds a YANG config module but forgets `ConfigRoots` in the registration | plugin/all import -> registry + loader enumeration -> claim comparison -> test failure naming module + root | TestConfigSchemaRootsClaimed |
| 2 | Agent registers a `ConfigRoots` entry with a typo'd root name | inverse comparison -> failure naming plugin + phantom root | TestConfigRootsPhantomClaims |
| 3 | Operator runs a build where an external plugin failed to load, leaving its root unclaimed | daemon startup -> doctor check -> `ze doctor` warning with code | doctor-config-claims.ci |
| 4 | Maintainer reviews which YANG leaves a plugin never references | advisory make target -> mention-check report | TestYANGLeafMentionReport |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestConfigSchemaRootsClaimed | `internal/component/plugin/all/config_claims_test.go` | AC-1, AC-2, AC-6: every config-schema root claimed under production semantics | |
| TestConfigRootsPhantomClaims | same | AC-3: every claim names a real schema root | |
| TestClaimAllowlistReasons | same | AC-4, AC-5: allowlist entries carry reasons | |
| TestDoctorUnclaimedRoots | doctor check's owning package | AC-7 unit: check fires on synthetic unclaimed root | |
| TestYANGLeafMentionReport | phase-2 check self-test | AC-8: fixture leaf with no mention is reported | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| (none numeric; N/A -- static gate) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| doctor-config-claims | `test/plugin/doctor-config-claims.ci` | `ze doctor` surfaces an unclaimed config root with its diagnostic code; `ze explain <code>` describes it | |

### Interop Tests (MANDATORY for protocol features)
- N/A: developer tooling, no wire behavior.

## Files to Modify
- `internal/core/diagnostic/codes.go` - register the unclaimed-config-root diagnostic code (AC-7)
- doctor registration in the owning package (`internal/component/plugin/server`, exact registration site per `ai/rules/repo-maintenance.md` at implementation)
- `Makefile`/`mk/` - `ze-yang-leaf-mentions` advisory target (phase 2; NOT added to verify stages)
- `ai/INDEX.md` - keyword row (discovery checklist below)
- `docs/comparison.md` - config-completeness parity note vs the reviewed daemon
- gate-mapping/doc rows per `ai/rules/repo-maintenance.md` (exact files at implementation)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | N/A | gate adds no config surface |
| YANG validation constraints | N/A | no new leaves |
| YANG custom validators | N/A | no new leaves |
| CLI commands/flags | N/A | no new CLI; reporting belongs to improve-6; doctor is an existing command |
| CLI grammar (action before identifier) | N/A | no new CLI |
| Editor autocomplete | N/A | no new leaves |
| Functional test for new RPC/API | Yes | `test/plugin/doctor-config-claims.ci` (doctor output, AC-7) |
| Pipe completeness | N/A | no new output-producing CLI command |
| Env var registration | N/A | no env vars |
| Doctor check for runtime dependencies | Yes | unclaimed-config-root check; `internal/core/diagnostic/codes.go` + owning-package registration + unit + .ci (AC-7) |
| Prometheus counters/metrics | N/A | doctor + dev-time gate cover observability; no runtime counter (state is static per config load) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | doctor check visible to operators -- doctor docs page (exact page per `ai/rules/repo-maintenance.md` at implementation) |
| 2 | Config syntax changed? | No | gate adds no syntax |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | dev tooling + doctor row only |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | read-only over registry |
| 9 | RFC behavior implemented, changed, or newly proven? | No | not protocol work |
| 10 | Test infrastructure changed? | Yes | new gate + advisory target documented per discovery checklist (`ai/INDEX.md` + testing docs page named at implementation) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- config-completeness enforcement parity |
| 12 | Internal architecture changed? | No | additive gate |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none (justified in Integration Checklist) |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | Yes | doctor-check inventory surfaces per `ai/rules/repo-maintenance.md` |
| 16 | Any changed source file is referenced by existing doc source anchors? | Check at implementation | grep `docs/` for anchors on changed files |
| 17 | Existing docs show config/CLI/API examples for this area? | No | none exist for this gate |

## Files to Create
- `internal/component/plugin/all/config_claims_test.go` - root-claim gate (TestConfigSchemaRootsClaimed, TestConfigRootsPhantomClaims, TestClaimAllowlistReasons)
- `internal/component/plugin/all/testdata/config-claims-allowlist.json` - allowlisted paths, each with reason + owner
- `scripts/checks/yang_leaf_mentions.go` - phase-2 advisory mention-check (go/parser BasicLit scan; --json; self-test fixture)
- `test/plugin/doctor-config-claims.ci` - AC-7 functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Wiring phase | Wiring Test table -- register entry points, write failing wiring tests |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist below |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | Executive Summary Report; two-commit closure |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** -- create `config_claims_test.go` skeleton in
   plugin/all importing registry + loader; allowlist loader stub; test fails against
   the current tree. The first failing run IS the A-2 burndown inventory: record its
   output in this spec before proceeding.
   - Tests: TestConfigSchemaRootsClaimed (failing), TestClaimAllowlistReasons
   - Verify: A-5 checked here (tag set under which plugin/all tests run)
2. **Phase: Root gate** -- claim-union builder (WantsConfigRoots + hub handler
   prefixes) + phantom-claim inverse + allowlist reasons; burn down or allowlist
   every current violation with reasons.
   - Tests: AC-1..AC-6 rows
3. **Phase: Doctor surface** -- diagnostic code + doctor check + unit test + .ci.
   - Tests: TestDoctorUnclaimedRoots, doctor-config-claims.ci (AC-7)
4. **Phase: Advisory mention-check (phase 2)** -- `scripts/checks/yang_leaf_mentions.go`
   + make target + self-test fixture; prototype on 3 plugins and record the A-6
   false-positive measurement in this spec; NOT wired into verify stages.
   - Plugin-name -> source-dir mapping: resolve the package that registered the
     module via its generated `yang/register.go` location (the module's
     `internal/**/yang/` dir per `yang_glue.go` discovery); the owning
     plugin package is that dir's parent. Never a hand-maintained name->dir table
     (`ai/rules/evidence.md`).
   - Tests: TestYANGLeafMentionReport (AC-8)
5. **Docs + discovery rows** -- comparison.md, ai/INDEX.md, gate-mapping rows.
6. **Full verification** -- `make ze-verify`; learned summary; two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | claim semantics shared with reader, not reimplemented (R-2) |
| Registration over hardcoding | both inventories derive live from producers |
| Rule: no-layering | (fill during design) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| (fill during design) | |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | none from network; gate reads embedded/registered schema only |
| Resource exhaustion | walk bounded by schema size |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| `reader.go` walkMap/findHandler is the production config apply path (skeleton Task premised on it) | `config.NewReader` (:236) has no non-test caller; production claiming is `WantsConfigRoots` root-matching (`reload.go`) + hub `Schema.Handlers` (`hub.go`) | Research agent caller trace, grep-verified this session | Task rewritten before design; claim model changed from per-node prefix to per-root; reader.go flagged as dead-code candidate |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
- `internal/component/config/reader.go` block-dispatch machinery
  (SchemaInfo/BlockEntry/BlockChange/DiffBlocks/findHandler) is exercised only by
  `reader_test.go` -- dead-code candidate; surface to user, never delete
  unilaterally (`ai/rules/never-destroy-work.md`).
- Unknown-key permissiveness in `validateContainerEntry` (`validator.go`) is a
  SECOND silent-accept layer (misspelled leaf inside a claimed root). It is
  adjacent to but distinct from this gate (schema-vs-claim); candidate follow-up or
  improve-6 grading extension -- record at design gate.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Root-claim gate as a registry-driven unit test in `internal/component/plugin/all` | (A) new `scripts/checks` binary AST-parsing registrations; (C) hard startup enforcement | The `all` package imports every registration by construction (same mechanism as the registry snapshot tests, `all_test.go`), so the test enumerates `ConfigRoots` and registered modules live -- exact semantics, zero new infrastructure, runs on every `ze-unit-test`. AST parsing (A) reimplements what the registry already knows (violates derive-not-hardcode); boot-refusal (C) is operationally harsh on an appliance |
| Complement with a `ze doctor` check for runtime-visible unclaimed roots | startup Warn log only; nothing | Externally-loaded plugins register at runtime and are invisible to the compile-time test; doctor is the discoverable runtime surface (`ai/rules/repo-maintenance.md`), and the silent `reload.go` Info log is precisely what it upgrades |
| Hub `Schema.Handlers` included in the claim union | exclude with reason | Only BGP registers non-nil handlers ("bgp","bgp/peer", `schema/cli/main.go`; all others nil `:607-610`), so inclusion costs one prefix-union step; consumer semantics verified at `Hub.RouteCommand` -> `SchemaRegistry.FindHandler` longest-prefix (`hub.go`, `schema.go`) |
| Leaf-level check is a HEURISTIC mention-check, phase 2, advisory-first | (a) reflect YANG leaves vs config-struct json tags -- INFEASIBLE: config-input structs carry zero json tags (`as112/config.go`; tags live only on show/state output structs); (b) wait for spec-review-typed-config-decode -- that spec is schema-driven with explicitly no struct registry, BGP-only, status design; (c) strict unknown-key rejection at verify -- different direction (config-not-in-schema), recorded as follow-up | Plugins hand-parse `map[string]any` with string-literal keys, so an AST scan of the owning plugin package for leaf-name literals vs YANG leaves under its claimed roots is implementable today; precedent for literal-grep drift guards: `as112/redistribute_test.go` TestMaxCommunitiesMatchesYANG. Heuristic, so advisory + allowlist, never a hard gate |

## Known Limitations
- Root-granular guarantee only in the blocking gate: a leaf inside a claimed root
  that the plugin's hand-parser ignores is caught only by the phase-2 advisory
  heuristic, not the hard gate. Exact leaf-consumption enforcement requires the
  schema-driven decode direction of `spec-review-typed-config-decode.md` (BGP-only,
  in design) to spread; revisit when it lands.
- Unknown-key permissiveness at verify (`validator.go`) is out of scope
  (recorded as follow-up candidate in Design Insights).

## Discovery (Mechanical Checklist, `ai/rules/repo-maintenance.md`)
1. Where would an agent look first? `ai/INDEX.md` keyword row: "config claims /
   unclaimed root / yang handler gate" -> this gate + doctor code.
2. What rule prevents regression? Pointer row added to the narrowest owning rule
   (`ai/rules/config.md` or `ai/rules/completion.md`, chosen at
   implementation) -- no new rule file.
3. What source of truth prevents drift? Plugin registry + yang loader +
   `feature-gates.txt`; zero static lists (allowlist is exceptions-with-reasons,
   not an inventory).
4. What verification proves it? TestConfigSchemaRootsClaimed (every ze-unit-test
   run) + doctor-config-claims.ci.
5. What docs explain usage? `docs/comparison.md` note + doctor docs page row.
6. What learned record preserves the decision? Learned summary at closure;
   LEARNED-INDEX entry (structural: config claim model is root-granular).

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| No config schema node can be silently unclaimed | functional test (fixture with orphan leaf fails gate) | (fill during implementation) |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block -- record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

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
- [ ] AC-1..AC-N all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`, or `scripts/checks/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` -- no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added (N/A expected)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N/A)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (N/A with justification above)
- [ ] Goal Validation table filled with concrete evidence

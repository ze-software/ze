# Spec: feature-gate-8-protocols

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | plan/spec-feature-gate-0-umbrella.md; learned 980/981/990. B-2 codec extraction NOT required (A-1 confirmed below) |
| Phase | 4/4 -- complete; all four protocols gated + tested + documented; pending user commit |
| Updated | 2026-06-26 |

> **Phase 0 result (A-1, BLOCKING GATE -- PASSED):** A fresh grep of all non-test,
> cross-tree Go files (excluding the two composition roots and each protocol's own
> subtree) found **zero** always-on importers of any protocol package or codec subpkg
> (`packet`/`types`/`v3/packet`/`wire`), and **zero** cross-protocol imports (R-5
> clean). Command:
> `grep -rln --include='*.go' -E '"…/internal/plugins/(isis|ldp|ospf|rsvpte)(-cmd)?(/|")' internal cmd | grep -v _test | grep -vE '/(isis|ldp|ospf|rsvpte)(-cmd)?(/|$)' | grep -v all/all.go | grep -v ze_core_dispatch.go` -> empty.
> **B-2-style codec extraction is NOT a prerequisite.** Each protocol is gated whole
> (codec + engine) by gating its blank imports in both composition roots.
> Protocol packages are NOT source-tagged (no `//go:build` on register.go); compile-out
> is by blank-import partitioning + dead-code elimination of unreferenced packages.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-feature-gate-0-umbrella.md` (lines 185, 5-6 - the B-2 dependency claim)
4. `plan/spec-tiers-0-umbrella.md` (B-2 codec extraction), `plan/learned/979-tiers-5-b1-unify-discovery.md`
5. `internal/component/plugin/all/all.go` (protocol blank imports), `cmd/ze/ze_core_dispatch.go` (CLI blank imports)
6. `internal/plugins/{isis,ldp,ospf,rsvpte}/register.go` + their `packet/`/`types/`/`transport/` subdirs

## Task

Make the optional routing protocols **IS-IS, LDP, OSPF, and RSVP-TE compile-out-able**
from the `ze` binary via per-protocol build tags (`ze_isis`, `ze_ldp`, `ze_ospf`,
`ze_rsvpte`), for a smaller binary and a smaller attack surface. Protocols are the final
group of feature-gate umbrella children (`plan/spec-feature-gate-0-umbrella.md`).

**Pivotal open question (A-1) that determines the whole approach:** the umbrella asserts
protocol compile-out is blocked on tiers-5 B-2 (codec/engine un-fusing). But code research
found that web handlers, MRT, sysrib/FIB, and redistribute are all DECOUPLED from the
protocol packages (they use a generic `CommandDispatcher`, generic byte codecs, and
string-keyed redistribution -- no direct import of a protocol's `packet`/`types` codec).
If that holds (no always-on consumer of any protocol CODEC exists), then each protocol can
be gated WHOLE (codec + engine together) via per-protocol blank-import gating TODAY, and
B-2 is NOT a prerequisite for the compile-out (it remains useful only for tier-audit
clarity). If a codec consumer DOES exist, that codec must be extracted out of the engine
first (a B-2-style protocol-codec extraction). Validating A-1 is the first implementation
step and decides whether this spec depends on B-2.

This is ONE spec covering all four protocols because the gating pattern is identical; each
protocol gets its own tag and its own present/absent test.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint:. -->
- [ ] `plan/spec-feature-gate-0-umbrella.md` - the umbrella's B-2 claim
  → Constraint: line 185 states "protocol compile-out needs B-2 (codec/engine un-fusing)
    first; protocol tag pulls in unintended engine code." This spec treats that as a
    HYPOTHESIS to validate (A-1), not a settled fact, because the code coupling looks
    lighter than the umbrella assumed.
  → Constraint: per-feature granularity (the user's choice) -- one tag per protocol.
- [ ] `plan/spec-tiers-0-umbrella.md` - B-2 scope
  → Constraint: B-2 = "extract bgp/iface/vpp/ike library subpkgs to internal/core/". The
    PROTOCOL analog (extract isis/ldp/ospf/rsvpte codecs) is NOT literally B-2; it is a
    separate, B-2-STYLE extraction needed ONLY IF A-1 finds an always-on codec consumer.
- [ ] `plan/learned/980-feature-gate-1-lg.md` - generator schema/blank-import gating
  → Constraint: `featureTags` maps a dir to a tag and gates its blank import into
    `all_ze_<tag>.go`. Each protocol blank-imports MULTIPLE dirs (yang + plugin + cli +
    transport), so each tag must gate several dirs (the multi-dir featureTags case).
- [ ] `plan/learned/981-feature-gate-2-ssh.md` - DISABLEABLE gate + two-composition-root reality
  → Constraint: `dep_audit` `DISABLEABLE` += each protocol package -> its tag.
  → Constraint: protocols are referenced from TWO composition roots -- generated `all.go`
    AND hand-written `cmd/ze/ze_core_dispatch.go` (CLI blank imports). BOTH must be gated.
- [ ] `ai/rules/plugin-self-containment.md` - the delete-the-folder invariant
  → Constraint: with a protocol's tag off, every blank import of it (both roots) drops and
    the plugin (+ its codec/engine subpkgs) unlinks.

### RFC Summaries (MUST for protocol work)
- N/A for the compile-out itself (no wire-behavior change). When a protocol IS compiled in,
  its existing RFC behavior + interop suite are unchanged; the present-build test re-runs them.

**Key insights:**
- Per-protocol always-on references found: `all.go` blank imports -- isis (yang 107 +
  plugin/cli/transport 213-215), ldp (yang 115 + plugin 221), ospf (yang 121 +
  plugin/cli/transport/v3 226-229), rsvpte (yang 129 + plugin 232); PLUS
  `cmd/ze/ze_core_dispatch.go` CLI blank imports -- isis/cli (43), ospf/cli + ospf/transport
  (44-45). ldp + rsvpte have no ze_core_dispatch CLI import.
- Decoupled (do NOT block compile-out): web handlers (handler_ospf/isis use
  `CommandDispatcher`, no protocol import), MRT (generic `OnBGPMessage` bytes, no protocol
  codec), sysrib/FIB/redistribute (generic interfaces + string protocol names).
- Codec+engine are FUSED in each protocol dir (e.g. ospf has `packet/`, `types/`,
  `v3/packet/` codec subpkgs alongside `adjacency/`, `lsdb/`, `spf/`, `transport/` engine
  subpkgs, all under one plugin tree). Whether this fusion matters for compile-out depends
  entirely on A-1 (is any codec subpkg imported by always-on code?).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/all/all.go` - blank-imports each protocol's yang + plugin
  (+ cli + transport for isis/ospf) unconditionally (lines 107/115/121/129 yang; 213-215,
  221, 226-229, 232 plugin/cli/transport).
  → Constraint: each protocol's full blank-import SET (not just yang) must move into a
    generated `all_ze_<proto>.go`. Enumerate the exact set per protocol in the audit.
- [ ] `cmd/ze/ze_core_dispatch.go` (`//go:build ze_core`) - blank-imports isis/cli (43),
  ospf/cli + ospf/transport (44-45) for CLI command registration.
  → Constraint: a SECOND composition root; these CLI imports need per-protocol gated
    companion files (`//go:build ze_core && ze_<proto>`), like the setup_features pattern.
- [ ] `internal/plugins/{isis,ldp,ospf,rsvpte}/register.go` - `init()` → `registry.Register`.
  → Constraint: registration is already init-based; gating the blank import drops it. No
    register.go change needed beyond what gating the imports achieves.
- [ ] `internal/plugins/ospf/{packet,types,v3/packet}` (codec) vs `{adjacency,lsdb,spf,
  circuit,neighbor,transport,redistribute}` (engine) - fused in one plugin tree.
  → Constraint: A-1 must determine if any `packet`/`types` codec subpkg is imported by
    always-on code (web/MRT/analyse/chaos). If not, gate the whole tree; if so, extract it.
- [ ] web handlers (`handler_ospf.go`, `handler_isis.go`), MRT (`plugins/mrt/register.go`),
  sysrib/redistribute - confirmed to use generic dispatch/bytes/strings, NOT protocol imports.
  → Constraint: these are NOT blockers; do not change them.

**Behavior to preserve:**
- Default `ze`/`ze-appliance` keep all four protocols (ZE_FEATURES includes all four tags,
  OR a curated subset -- see Key Design Decisions on default-on set).
- Each protocol's wire behavior, adjacency/LSDB/SPF, redistribution, CLI ("show ospf ...")
  and web display are byte-for-byte unchanged when its tag is on.
- The generic decoupled consumers (web dispatch, MRT, sysrib, redistribute) are unchanged
  and tolerate any subset of protocols being absent.

**Behavior to change:**
- Each protocol plugin becomes a disableable feature: its tag off => unlinked.
- `ze-stripped` drops all four protocols (a pure-management build).
- Both composition roots gate each protocol's blank imports behind its tag.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Compile time: presence/absence of each `ze_<proto>` tag.
- Run time: the plugin registry (a protocol registers only if its blank import compiled).

### Transformation Path
1. With `ze_<proto>` on, `all_ze_<proto>.go` (generated) blank-imports the protocol's
   yang/plugin/cli/transport, and a gated dispatch companion (`//go:build ze_core &&
   ze_<proto>`) blank-imports its CLI; both run the protocol's `init()` → `registry.Register`.
2. The reactor/plugin host discovers the registered protocol exactly as today.
3. With the tag off, both composition roots omit the blank imports; `init()` never runs;
   the plugin (and, per A-1, its codec) is unlinked.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Build tag ↔ all.go root | generator-emitted `all_ze_<proto>.go` (multi-dir) | [ ] |
| Build tag ↔ dispatch root | per-protocol gated `dispatch_<proto>.go` CLI imports | [ ] |
| Composition root ↔ registry | gated blank import → `init()` → `registry.Register` | [ ] |
| Disableable protocol ↔ always-on | dep_audit DISABLEABLE; A-1 confirms no codec import | [ ] audit |
| Protocol codec ↔ always-on consumer | A-1: must be NONE, or extract the codec (B-2-style) | [ ] A-1 |

### Integration Points
- `scripts/codegen/plugin_imports.go` featureTags - each protocol's dirs -> its tag.
- `cmd/ze/` per-protocol gated dispatch companion files (CLI blank imports).
- `scripts/dev/dep_audit.py` DISABLEABLE - each protocol package -> its tag.
- (CONDITIONAL on A-1) a protocol-codec extraction package if a codec consumer exists.

### Architectural Verification
- [ ] No bypassed layers (protocols still register + run via the plugin registry)
- [ ] No unintended coupling (generic consumers unchanged; A-1 confirms no codec coupling)
- [ ] No duplicated functionality (reuse the generator + dispatch gating; no per-protocol stubs)
- [ ] Zero-copy preserved (N/A - composition/build change only)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | NO always-on code imports any protocol CODEC (packet/types) -- so each protocol can be gated WHOLE without B-2 | Explore: web/MRT/sysrib/redistribute all use generic dispatch/bytes/strings | a codec consumer forces a B-2-style codec extraction before gating | grep importers of `internal/plugins/<proto>/{packet,types,v3/packet}` across always-on code (web, mrt, analyse, sysrib, chaos) | **confirmed** -- fresh grep returned zero cross-tree non-test importers and zero cross-protocol imports (see Phase 0 result above); also confirmed no always-on plugin declares a Registration string dependency on a protocol |
| A-2 | the generator featureTags can gate MULTIPLE dirs per tag (yang+plugin+cli+transport) | 980: featureTags is a dir->tag map (shared with gnmi A-2) | the generator handles one dir per tag | spike: map all isis dirs to ze_isis, run `--check`, inspect all_ze_isis.go | unvalidated |
| A-3 | the ze_core_dispatch.go CLI blank imports can move into per-protocol gated companion files without breaking dispatch | 853: setup_features per-flavor blank-import groups precedent | the dispatch wiring needs the import in the main file | move isis/cli to a gated dispatch_isis.go, build ze_core (isis off) + ze_core ze_isis | unvalidated |
| A-4 | a no-protocol build leaves config validation safe (protocol yang not registered) | 980: schema gated => clean "unknown field" | `ospf {}` config panics in a no-ospf build | build a no-ospf binary, feed `ospf {}` config | unvalidated |
| A-5 | the generic consumers (web/MRT/sysrib/redistribute) tolerate an absent protocol at runtime | they key on strings/dispatch, not types | a missing protocol breaks redistribute/web display | build a no-ospf binary, exercise redistribute + `show ospf` (expect clean absence) | unvalidated |
| A-6 | per-protocol tags do not create an unmanageable build-tag test matrix | umbrella R-1 flagged the matrix cost | combinatorial test explosion | one present/absent test per protocol (4 pairs), not a cross-product (umbrella R-1 mitigation) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A-1 is false: a codec IS consumed always-on, so B-2-style extraction is needed first | grep finds a packet/types importer in always-on code | scope the protocol-codec extraction as a prerequisite child (B-2 analog); do NOT gate until codec is in an always-on home |
| R-2 | the second composition root (ze_core_dispatch.go) is missed, leaving isis/ospf CLI linked | go tool nm shows isis/ospf cli symbols in a no-isis/no-ospf build | gate BOTH roots; the absent test does an nm symbol check |
| R-3 | multi-dir featureTags is unsupported, needing a generator change | generator emits one import per tag | A-2 spike; extend the generator to accept a dir list per tag (shared fix with gnmi) |
| R-4 | per-protocol granularity multiplies tags + matrix (umbrella R-1) | many build-tag files | one focused present/absent pair per protocol; a single `ze_igp` umbrella tag is a fallback if the matrix is painful |
| R-5 | a protocol shares a helper with another protocol or with bgp/mpls (cross-protocol coupling) | gating one breaks another's build | grep cross-protocol imports during the audit; shared helpers go to an always-on core leaf |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `go build -tags 'ze_core ze_ospf'` | → | ospf registers; reactor runs it; "show ospf" works | `TestBuildTag_OSPF_Present` (cmd/ze) |
| `go build -tags ze_core` (ospf off) | → | ospf package not linked; no ospf registration | `TestBuildTag_OSPF_Absent` (cmd/ze) |
| same pair for isis / ldp / rsvpte | → | each registers iff its tag is on | `TestBuildTag_{ISIS,LDP,RSVPTE}_{Present,Absent}` |
| `dep_audit.py --check` | → | no always-on import of any gated protocol package | `dep_audit` `--check` clean + `--selftest` |
| A-1 grep gate | → | no always-on importer of any protocol codec subpkg | recorded in the audit (decides B-2 dependency) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A-1 validated | the audit records, per protocol, whether any always-on code imports its codec; the B-2 dependency is confirmed required or not required, with grep evidence |
| AC-2 | `go build` with `ze_<proto>` ON (each) | the protocol compiled in, registered, runs; existing protocol unit + functional + interop tests pass; its CLI/web display works |
| AC-3 | bare `go build -tags ze_core` (all protocols OFF) | `go tool nm` shows zero symbols for each protocol package; daemon starts; no protocol registered; no error |
| AC-4 | one protocol on, others off (e.g. `ze_core ze_ospf`) | only ospf is linked + registered; isis/ldp/rsvpte symbols absent |
| AC-5 | the generator runs | emits one `all_ze_<proto>.go` per protocol gating its full blank-import set; removes them from `all.go`; `--check` passes |
| AC-6 | the dispatch root is built per protocol | isis/ospf CLI blank imports are gated; a no-isis/no-ospf build links no CLI symbols for them |
| AC-7 | `dep_audit.py --check` with each protocol in DISABLEABLE | clean: no always-on importer |
| AC-8 | a no-ospf binary fed config containing `ospf { ... }` | clean "unknown field" validation, no panic (same per protocol) |
| AC-9 | a no-ospf binary exercises redistribute + `show ospf` | generic consumers tolerate the absence (clean "unknown command"/no-op), no panic |
| AC-10 | `make ze-stripped` and `make ze` are built | ze-stripped links no protocol symbols; `ze`/`ze-appliance` keep the default-on protocol set |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | builds a pure-management `ze` with no IGP/MPLS protocols | all protocol tags off → blank imports dropped (both roots) → packages unlinked | `TestBuildTag_*_Absent` + `go tool nm` |
| 2 | builds a `ze` with only OSPF | `ze_core ze_ospf` → ospf registers; isis/ldp/rsvpte absent | `TestBuildTag_OSPF_Present` + `TestBuildTag_ISIS_Absent` etc |
| 3 | runs the OSPF interop suite on an OSPF build | existing FRR/BIRD interop scenario under `ze_ospf` | existing ospf interop test (unchanged, on-build) |
| 4 | runs a no-ospf binary against a config with `ospf {}` | config load → ospf schema absent → clean unknown-field handling | `test/parse/ospf-absent-config.ci` or absent-test assertion |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildTag_OSPF_Present` | `cmd/ze/build_tag_ospf_present_test.go` (`//go:build ze_ospf`) | ospf registered when on | |
| `TestBuildTag_OSPF_Absent` | `cmd/ze/build_tag_ospf_absent_test.go` (`//go:build !ze_ospf`) | ospf not registered when off | |
| (same pattern) | `build_tag_{isis,ldp,rsvpte}_{present,absent}_test.go` | each protocol gated independently | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no numeric inputs) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `build_tag_<proto>` | `cmd/ze/build_tag_<proto>_*_test.go` | each protocol present with its tag, absent without | |
| `<proto>-absent-config` | `test/parse/<proto>-absent-config.ci` (or absent-test assertion) | no-protocol binary handles `<proto> {}` config safely | |

### Interop Tests (MANDATORY for protocol features)
- The EXISTING per-protocol interop suites (FRR/BIRD/etc.) run under each protocol's tag to
  prove on-build behavior is unchanged. Compile-out adds no new wire behavior, so no new
  interop scenario is needed; the off-build simply has no protocol to interop.

### Future (if deferring any tests)
- If A-1 finds a codec consumer, the codec-extraction prerequisite gets its own spec/tests
  before gating; that is tracked, not silently deferred.

## Files to Modify
- `scripts/codegen/plugin_imports.go` - `featureTags` += each protocol's dirs -> its tag
  (multi-dir per tag): isis (yang+plugin+cli+transport), ldp (yang+plugin), ospf
  (yang+plugin+cli+transport+v3/transport), rsvpte (yang+plugin)
- `internal/component/plugin/all/all.go` - protocol blank imports removed (generator)
- `cmd/ze/ze_core_dispatch.go` - remove isis/ospf CLI blank imports (moved to gated companions)
- `scripts/dev/dep_audit.py` - `DISABLEABLE` += `internal/plugins/{isis,ldp,ospf,rsvpte}` -> their tags
- `Makefile` - `ZE_FEATURES` += the default-on protocol set (see Key Design Decisions)
- `internal/test/runner/runner.go` - `TestBuildTags()` appends the protocol tags
- `.golangci.yml` - `build-tags` appends the protocol tags
- `ai/rules/module-tiers.md`, `docs/features.md` - document the protocol tags

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| CLI commands/flags | [ ] gated | "show ospf"/"show isis" CLI registration gated with the protocol |
| Functional test | [ ] yes | `cmd/ze/build_tag_<proto>_*_test.go`, config-absent assertions |
| Doctor check | [ ] no | each protocol owns its doctor checks; absent protocol = no check |
| Discovery-updates | [ ] yes | `ai/rules/discovery-updates.md` - register the protocol tags |
| Interop | [ ] keep | existing per-protocol interop suites run under each tag (on-build) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes (build flavors) | `docs/features.md` (build-tag table: add the four protocol tags) |
| 11 | Affects daemon comparison? | [ ] maybe | `docs/comparison.md` if protocol support is listed as build-configurable |
| 12 | Internal architecture changed? | [ ] yes | `docs/architecture/cli/plugin-modes.md` (per-protocol gating; two composition roots) |
| 15 | Runtime inventory changed? | [ ] yes | "show ospf"/"show isis" commands absent in no-protocol builds |
| others | - | [ ] assess | grep docs for protocol references |

## Files to Create
- `cmd/ze/dispatch_isis.go` (`//go:build ze_core && ze_isis`), `dispatch_ospf.go`
  (`//go:build ze_core && ze_ospf`) - the moved CLI blank imports
- `cmd/ze/build_tag_{isis,ldp,ospf,rsvpte}_present_test.go` (each `//go:build ze_<proto>`)
  + `_absent_test.go` (each `//go:build !ze_<proto>`)
- `internal/component/plugin/all/all_ze_{isis,ldp,ospf,rsvpte}.go` (generated)
- `test/parse/{isis,ldp,ospf,rsvpte}-absent-config.ci` (if expressible)
- (CONDITIONAL on A-1) a protocol-codec extraction package + its spec, only if a codec
  consumer is found

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | A-1 FIRST (codec-consumer grep) -- it decides whether to proceed or extract; then Files to Modify/Create |
| 3. Wiring | Wiring Test - per-protocol build-tag tests |
| 4. Implement | Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Verification | `make ze-verify-changed` |
| 14. Summary | Implementation Summary |

### Implementation Phases
0. **Phase 0 (BLOCKING GATE): validate A-1.** Grep every always-on package (web, mrt,
   analyse, sysrib, redistribute, chaos) for an import of any protocol `packet`/`types`/
   codec subpkg. If found, STOP: scope the protocol-codec extraction prerequisite (B-2
   analog) and present to the user before any gating. If none found, B-2 is NOT required;
   proceed to Phase 1.
1. **Phase 1: gate one protocol end-to-end (ospf as the pilot)**
   - generator featureTags for all ospf dirs; gated `dispatch_ospf.go`; dep_audit
     DISABLEABLE += ospf; present/absent tests; tag wiring for ze_ospf.
   - Verify: `ze_core ze_ospf` identical; `ze_core` (ospf off) drops ospf; nm symbol check.
2. **Phase 2: replicate for isis, ldp, rsvpte**
   - same pattern per protocol (isis + ospf also need the dispatch companion; ldp/rsvpte
     only the all.go gating).
   - Verify: each present/absent pair; one-on-others-off build (AC-4).
3. **Phase 3: docs + config/command safety**
   - `docs/features.md`, `module-tiers.md`; validate A-4 (config) + A-5 (generic consumers)
     per protocol on a no-protocol binary.
4. **Full verification** - `make ze-verify-changed`; nm-measure each protocol on/off.

### Failure Routing
| Failure | Route To |
|---------|----------|
| A-1 finds a codec consumer | STOP - scope the codec extraction prerequisite (R-1) |
| protocol not omitted with tag off | a missed blank import (likely the dispatch root) - R-2 + nm |
| generator emits one import per tag | multi-dir featureTags - R-3/A-2 (shared with gnmi) |
| a no-protocol build breaks redistribute/web | generic consumer intolerance - A-5 |
| config panics in no-protocol build | schema absence - A-4 |
| 3 fix attempts fail | STOP, report, ask user |

### Critical Review Checklist (/implement stage 7)
| Check | What to verify for this spec |
|-------|------------------------------|
| A-1 evidence | the codec-consumer grep result is recorded with the B-2 dependency decision |
| Completeness | Every AC-N has implementation with file:line; all four protocols gated |
| Both roots gated | each protocol's blank imports gone from BOTH all.go and ze_core_dispatch.go |
| Symbol absence | `go tool nm` on a no-protocol binary lists zero symbols for each gated protocol |
| Generic consumers | web/MRT/sysrib/redistribute unchanged and tolerate absent protocols |
| Test gating | per-protocol tests gated; default unit suite passes |
| Matrix discipline | one present/absent pair per protocol, not a cross-product (R-4) |

### Deliverables Checklist (/implement stage 11)
| Deliverable | Verification method |
|-------------|---------------------|
| A-1 decision | the audit records grep output + "B-2 required: yes/no" |
| per-protocol gating (both roots) | `grep -rl internal/plugins/<proto>` shows only gated/test files |
| dep_audit DISABLEABLE (4 entries) | `python3 scripts/dev/dep_audit.py --check` exits 0; `--selftest` passes |
| generated all_ze_<proto>.go (4) | `ls internal/component/plugin/all/all_ze_*.go`; `--check` |
| symbol drop | `go build -tags ze_core -o /tmp/ze-core ...`; `go tool nm` each protocol count = 0 |
| present/absent tests (4 pairs) | `go test -tags 'ze_core ze_<proto>'` and `-tags ze_core` per protocol |

### Security Review Checklist (/implement stage 12)
| Check | What to look for |
|-------|-----------------|
| No protocol exposure | a no-<proto> build runs no <proto> listener/socket and registers no <proto> handler |
| Codec safety | if A-1 forces a codec extraction, the extracted codec has no engine side effects (pure encode/decode) |
| Partial-protocol builds | a one-protocol build does not leak the others' state or commands |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- The umbrella assumed protocol compile-out needs B-2; the code says the protocols may be
  more decoupled than that (generic dispatch everywhere). The spec turns the assumption into
  a measurable gate (A-1) rather than inheriting it. Compile-out needs B-2 only if a codec is
  consumed by always-on code -- and the evidence so far says it is not.
- Protocols expose the two-composition-root reality most sharply: gating only `all.go`
  leaves isis/ospf CLI linked via `ze_core_dispatch.go`. Count every root.

## Core Insight
"Needs B-2" is a hypothesis, not a fact: a protocol can be gated whole the moment nothing
always-on imports its codec. Validate the codec-consumer question before assuming an
extraction is required.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Make the B-2 dependency CONDITIONAL on A-1 | inherit the umbrella's "B-2 required" claim | code research shows web/MRT/sysrib are decoupled; gating whole may be possible today. A-1 decides with evidence |
| One spec for all four protocols | four separate specs | the gating pattern is identical; per-protocol tags + tests live in one spec without duplication |
| Per-protocol tags (ze_isis/ze_ldp/ze_ospf/ze_rsvpte) | a single ze_igp umbrella tag | the user chose per-feature granularity; ze_igp is the fallback if the test matrix (R-4) gets painful |
| Gate BOTH composition roots | gate only all.go | isis/ospf register CLI via ze_core_dispatch.go; missing it leaves them linked |
| Default-on set = all four (TBD with user) | drop some by default | preserve current behavior; the default-on protocol set is a product decision to confirm before changing ZE_FEATURES |

## Known Limitations
- Implementation is BLOCKED until A-1 is validated (it decides the B-2 dependency).
- Per-protocol granularity adds build-tag test surface (umbrella R-1); mitigated by one
  pair per protocol.
- If A-1 finds a codec consumer, a codec-extraction prerequisite spec must land first.

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| each protocol compile-out-able via its tag | build-tag test + nm symbol check | present tests pass (4/4); absent tests pass (8 funcs + nm); `go tool nm`: FULL build links isis 904 / ldp 191 / ospf 1444 / rsvpte 266 symbols, bare `ze_core` links 0 / 0 / 0 / 0 |
| B-2 dependency resolved with evidence | audit (A-1) | A-1 grep returned EMPTY (no always-on cross-tree importer of any protocol pkg/codec); B-2 required: **NO** -- gated whole |
| no always-on import of a gated protocol | audit | `dep_audit.py --check` exit 0 (engine placement clean; disableable gate clean) with all gated dirs in the manifest; `--selftest` OK |
| default flavors keep the default-on protocols | build | `ZE_FEATURES` awk yields the 4 tags (default-on); FULL `ze` build links all four; `ze_core ze_ospf` links only ospf (1445), isis/ldp/rsvpte 0; bare `ze_core` (ze-stripped path) links none |

## Review Gate
### Run 1 (focused review of the spec-8 diff; the tree carries 3 other sessions' uncommitted work, so the full /ze-review skill conflates their changes -- this pass scopes to my files + the automated review tooling: ze-validate, audit-test-relaxation, dep_audit, lint)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | Pre-existing data race in `bgp/reactor/listener.go:195 acceptLoop` surfaced by chaos inprocess under `-race`; not my area (gating change touches no reactor code) | internal/component/bgp/reactor | reported to user; out of scope for this spec |
| 2 | NOTE (external) | geodns (another session) registered in all.go but missing from docs/DESIGN.md Shipped Plugins + plugin/all golden test lists -> stage 02/03 verify failures | docs/DESIGN.md, internal/component/plugin/all/all_test.go | not my work; the geodns session must update DESIGN.md + golden lists |
| 3 | NOTE (external) | 8 audit-test-relaxation findings, all kernel-build-consolidation (another session) | internal/appliance, test/install | not my work |

### Final status
- [ ] focused review shows 0 BLOCKER, 0 ISSUE attributable to spec-8 (lint 0 issues; ze-validate source anchors all valid; dep_audit --check + --selftest clean; generator --check clean; functional + exabgp pass; no protocol appears in any FAIL)
- [ ] NOTEs above are all external (other sessions) or pre-existing, none introduced by this spec

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/plugin/all/all_ze_{isis,ldp,ospf,rsvpte}.go` | yes | generated by `make generate`; `ls` shows all 4 |
| `cmd/ze/dispatch_{isis,ospf}.go` | yes | gated dispatch companions created |
| `cmd/ze/hub/build_tag_{isis,ldp,ospf,rsvpte}_{present,absent}_test.go` | yes | 8 files created |
| `cmd/ze/hub/build_tag_protocols_absent_test.go` | yes | consolidated nm test |
| `plan/learned/995-feature-gate-8-protocols.md` | yes | learned summary |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | A-1 recorded; B-2 decision | Phase 0 result + A-1 row: grep empty, B-2 NOT required |
| AC-2 | protocol ON registers/runs | present tests pass (`registry.Has` true for isis/ldp/ospf/rsvp-te under full tags) |
| AC-3 | bare `ze_core`: 0 symbols, no registration | `TestBuildTag_Protocols_AbsentBinaryDropsSymbols` passes; nm 0/0/0/0 |
| AC-4 | one on, others off | `ze_core ze_ospf` nm: ospf 1445, isis/ldp/rsvpte 0 |
| AC-5 | generator emits 1 file/protocol; `--check` | 4 `all_ze_*.go`; generator `--check` exit 0 |
| AC-6 | dispatch root gated per protocol | isis/ospf CLI in `dispatch_{isis,ospf}.go`; nm confirms 0 in bare core |
| AC-7 | `dep_audit --check` clean (DISABLEABLE) | exit 0 after the two model fixes; `--selftest` OK |
| AC-8 | no-protocol binary rejects `<proto> {}` cleanly | `TestBuildTag_<proto>_AbsentRejects<Proto>Config` pass (err contains "unknown") |
| AC-9 | generic consumers tolerate absence | bare-core hub test populates registry minus protocols; no panic; config rejected, not crashed |
| AC-10 | `ze-stripped` none, `ze`/`ze-appliance` default set | bare `ze_core` nm 0; `ZE_FEATURES` includes the 4 tags |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | grep empty (no always-on/cross-protocol importer); B-2 not required |
| A-2 | confirmed | multi-dir featureTags via multiple manifest lines per tag; `all_ze_<proto>.go` hold the full sets; generator `--check` clean |
| A-3 | confirmed | `dispatch_{isis,ospf}.go` gated companions build under `ze_core ze_<proto>`; bare `ze_core` drops them (nm 0) |
| A-4 | confirmed | absent config tests: `<proto> {}` rejected with clean "unknown" (no panic) |
| A-5 | confirmed | bare-core hub test: registry populated minus protocols, no panic; generic consumers unchanged |
| A-6 | confirmed | one present/absent pair per protocol + one shared nm test (no cross-product) |

## Checklist
### Goal Gates (MUST pass)
- [ ] A-1 validated and the B-2 dependency decided with grep evidence (Phase 0 gate)
- [ ] AC-1..AC-10 demonstrated
- [ ] all four protocols compile-out-able; present/absent build-tag tests pass
- [ ] both composition roots gated; no always-on protocol import
- [ ] dep_audit DISABLEABLE clean (4 entries)
- [ ] generator emits all_ze_<proto>.go (4); `--check` passes
- [ ] `make ze-verify-changed` passes
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Documentation Update Checklist answered with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior (per-protocol present/absent + config-safe)
- [ ] Interop tests run under each protocol's tag (on-build, unchanged)
- [ ] Goal Validation table filled with concrete evidence

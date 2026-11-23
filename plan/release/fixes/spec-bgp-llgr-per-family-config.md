# Spec: Long-Lived Graceful Restart Configured per AFI/SAFI

| Field | Value |
|-------|-------|
| Status | verification |
| Priority | P1 |
| Release reason | The implemented LLGR capability owes per-family operator control required by its RFC. |
| Scope | protocol |
| Depends | - |
| Phase | verification |
| Handoff | verify |
| Updated | 2026-11-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Make Long-Lived Graceful Restart (RFC 9494) an operator decision per address
family, as RFC 9494 Section 5 requires. Previously LLGR was enabled per peer:
one `long-lived-stale-time` leaf in the peer's (or group's) `graceful-restart`
container stamped code 71 onto every negotiated family of the session, so an
operator could not enable LLGR for one family of a multi-family peer and leave
another off.

Found by spec-rfc-verdict-test-fix-pass (child spec-rfc-verdict-fix-bgp,
continuation 27, 2026-10-02). OWNER RULING 8 (e) in
`plan/records/rfc-verdict-test-fix-pass-rulings.md`: "Per-AFI/SAFI LLGR
configuration (RFC9494-5-2): its own small spec; the row stays {gap} until it
lands."

## Evidence

| Item | Value |
|------|-------|
| Requirement | RFC9494-5-2 in `rfc/short/rfc9494.md` (`{gap}` removed, audited `enforced` in `rfc/audit/rfc9494/RFC9494-5-2.json`, proven in `rfc/discrimination/rfc9494/RFC9494-5-2.json`) |
| Producer, capability | `parseLLGRCapValue` and `extractLLGRCapabilities` (`internal/component/bgp/plugins/gr/gr_llgr.go`) now read `graceful-restart / long-lived / family` with per-family `stale-time` |
| Producer, schema | `internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang` symmetric `short-lived` and `long-lived` containers inside `container graceful-restart` |
| Helper side (already per family) | RFC9494-5-1: `exchangedLLGRLocked` (`internal/component/bgp/plugins/gr/gr_llgr_exchange.go`) honors a received code 71 only for families Ze declared in the OPEN it sent |
| User guide | `docs/guide/graceful-restart.md`, "Long-Lived Graceful Restart (RFC 9494)" |

### RFC text read for this spec

| Source | Section | Sentence |
|--------|---------|----------|
| `rfc/full/rfc9494.txt` | 3.1 | "If the LLGR capability is advertised, the Graceful Restart capability [RFC4724] MUST also be advertised; see Section 4.1." |
| `rfc/full/rfc9494.txt` | 4.1 | "if support for conventional Graceful Restart is not desired for the session, the conventional GR phase can be skipped by omitting all AFIs/SAFIs from the GR Capability, advertising a Restart Time of zero, or both." |
| `rfc/full/rfc9494.txt` | 5 | "Implementations MUST NOT enable these procedures by default." |
| `rfc/full/rfc9494.txt` | 5 | "They MUST require affirmative configuration per AFI/SAFI in order to enable them." |

## Migration of the per-peer leaf (no-layering)

`ai/rules/no-layering.md` forbids keeping both shapes. Ze is pre-release
(`docs/architecture/config/deprecated-options.md`, `ai/META-DECISIONS.md`):
the old `long-lived-stale-time`, top-level `restart-time`, and `container family { leaf-list name }`
are deleted with no runtime fallback. Every config, test, and doc in the
repository is converted to the new symmetric `short-lived` / `long-lived`
syntax, and any config naming the deleted leaf is refused at YANG validation.

| Existing config | What happens |
|-----------------|--------------|
| No `long-lived` block | Unchanged: no code 71, LLGR off for every family |
| Old `restart-time`, `family { name ... }`, or `long-lived-stale-time` | Refused by YANG schema; all in-tree configs converted so advertised OPEN bytes remain identical |
| `graceful-restart { }` or `graceful-restart { long-lived { ... } }` without `short-lived` | Code 64 sent as `0000` (Restart Time 0, 0 tuples: RFC 4724 §3 helper-only and RFC 9494 §4.1 GR-phase skip) |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/deprecated-options.md` - how a removed config option is migrated or refused
  -> Decision: Ze is pre-release; delete the old leaf cleanly with no runtime shim or fallback.
  -> Constraint: Refusal comes directly from the YANG schema; every in-tree config and doc updates in the same change.
- [ ] `docs/guide/graceful-restart.md` - LLGR configuration as documented to operators
  -> Decision: Record the owner-approved symmetric `short-lived` / `long-lived` design under "Design decisions".
  -> Constraint: Keep code 64 and code 71 family memberships independent so GR-phase skip remains expressible.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9494.md` - rows 3.1-1, 4.1-1, 5-1, 5-2
  -> Constraint: Code 71 cannot be configured without code 64; LLGR MUST be off by default and explicitly configured per AFI/SAFI.

**Key insights:**
- Nesting `short-lived` and `long-lived` inside `container graceful-restart` structurally enforces RFC 9494 §3.1/§4.1 without cross-container validation.
- A keyed YANG `list family` with 1-2 simple leaves automatically formats compactly via `ze config fmt` (`family { ipv4/unicast }` and `family { ipv4/unicast 3600 }`).
- Making `short-lived` a presence container avoids the `KeyDefault = "default"` trap of an empty YANG list while cleanly representing both helper-only GR and GR-phase skip as `0000`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/gr/gr_capability.go` - builds code 64 payload and validates GR family lists against session families.
- [ ] `internal/component/bgp/plugins/gr/gr_llgr.go` - decodes and encodes code 71 capability payloads from BGP config.
- [ ] `internal/component/bgp/plugins/gr/gr_llgr_exchange.go` - filters received code 71 against families Ze declared in its sent OPEN.
- [ ] `internal/component/bgp/plugins/gr/gr_filter.go` - declares `llgr-ingress` and `llgr-egress` filter insertions conditioned on LLGR config presence.
- [ ] `internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang` - defines the `graceful-restart` capability schema augmented onto peer and group scopes.
- [ ] `internal/component/bgp/reactor/config_capabilities.go` - extracts `RawCapabilityConfig["graceful-restart"]["restart-time"]`.
- [ ] `internal/component/bgp/plugins/cmd/peer/create.go` - builds the dynamic peer config tree for `create bgp peer ... graceful-restart <N>`.

**Behavior to preserve:**
- Code 64 wire encoding (`0078 00010100 ...` for full GR, `0000` for helper-only / GR-skip) and code 71 wire encoding (7-byte tuples with F-bit=1, reserved=0, 24-bit LLST).
- `llgrFamily.LLST` JSON decode key `long-lived-stale-time` (wire decode contract tested by `test/decode/capability-llgr.ci`).
- Peer-level `graceful-restart` container replaces group-level container rather than merging.
- Helper-side per-family intersection in `exchangedLLGRLocked`.

**Behavior to change:**
- Replace flat `graceful-restart` leaves with symmetric `short-lived` (presence, `restart-time`, keyed `list family`) and `long-lived` (keyed `list family` with mandatory `stale-time`).
- Encode code 71 only for families explicitly listed under `long-lived / family`, each with its own `stale-time`.
- Validate both `short-lived / family` and `long-lived / family` against the peer's carried session families at offline `ze config validate`, `config commit`, and daemon startup.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| YANG augmentation & compact list serialization | `internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang`, `internal/component/config/flatten.go` | Augments `graceful-restart` onto peer and group capability containers and formats 1- and 2-leaf keyed lists compactly (`family { ipv4/unicast 3600 }`) | Replaces: replaces flat `restart-time`, `family { name [...] }`, and `long-lived-stale-time` with symmetric `short-lived` (presence) and `long-lived` containers carrying keyed `list family` |
| GR/LLGR capability extraction & peer-over-group precedence | `internal/component/bgp/plugins/gr/gr_capability.go` (`grCapabilityFor`, `parseGRCapValue`), `gr_llgr.go` (`extractLLGRCapabilities`, `parseLLGRCapValue`) | Resolves peer/group `graceful-restart` container and emits Stage 3 `sdk.CapabilityDecl` for codes 64 and 71 | Extends: reuses `grCapabilityFor` for both codes 64 and 71 and narrows `parseLLGRCapValue` to emit only families configured under `long-lived / family` |
| Peer-replaces-group tree resolution | `internal/component/bgp/config/resolve.go` (`replacePaths`) | Controls which capability containers replace rather than deep-merge group config during `ResolveBGPTree` | Extends: registers `session/capability/graceful-restart` in `replacePaths` so filter insertion and reactor match `grCapabilityFor` |
| In-process config verification | `internal/component/bgp/plugins/gr/register.go` (`InProcessConfigVerifier`), `gr_capability.go` (`refusePeerGRFamilies`) | Validates plugin-owned config constraints during offline `ze config validate` and commit verification | Extends: wires `refuseUncarriedGRSections` into `InProcessConfigVerifier` and validates both `short-lived / family` and `long-lived / family` + `stale-time` bounds |
| Helper-side per-family LLGR intersection | `internal/component/bgp/plugins/gr/gr_llgr_exchange.go` (`exchangedLLGRLocked`) | Honors a peer's received code 71 only for families Ze declared in its sent OPEN | Does not apply (preserved as-is): already per-family once code 71 advertises only configured families |
| Config-driven LLGR filter insertions | `internal/component/bgp/plugins/gr/gr_filter.go` (`grInsertions`, `llgrConfig`) | Inserts `llgr-ingress` and `llgr-egress` filters when LLGR config is present on a peer | Extends: updates `llgrConfig` path to `session / capability / graceful-restart / long-lived / family` |
| ExaBGP migration capability container emitter | `internal/exabgp/migration/migrate_defaults.go` (`migratedCapabilityContainer`) | Converts ExaBGP `graceful-restart` capability blocks into Ze config syntax | Extends: emits `short-lived { restart-time <N>; }` instead of legacy top-level `restart-time` |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator config (`bgp { peer|group ... session { capability { graceful-restart { short-lived { ... } long-lived { ... } } } } }`) parsed against `ze-graceful-restart.yang`.

### Transformation Path
1. YANG parse and native range/type validation (`0..4095`, `0..16777215`, `registered-address-family`) in `internal/component/config/`.
2. Offline `InProcessConfigVerifier` (`refuseUncarriedGRSections` in `gr/register.go`) and runtime `OnConfigVerify` / `OnConfigure` verify every `short-lived` and `long-lived` family is carried by the peer's session and every `long-lived` `stale-time` is in `0..16777215`.
3. `grCapabilities` invokes `extractGRCapabilities` (`gr_capability.go`) and `extractLLGRCapabilities` (`gr_llgr.go`) using shared `grCapabilityFor` peer/group precedence.
4. `grInsertions` (`gr_filter.go`) activates `llgr-ingress` and `llgr-egress` when `session / capability / graceful-restart / long-lived / family` is present.
5. Reactor writes code 64 and code 71 capabilities into the OPEN; `gr_llgr_exchange.go` reads Ze's sent OPEN to enforce helper-side per-family filtering.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> `bgp-gr` plugin | Stage 2 `rpc.ConfigSection` JSON (`configvalue.ListEntries`, `configvalue.Int`) | Yes |
| `bgp-gr` -> Reactor | Stage 3 `sdk.CapabilityDecl` (codes 64 and 71, hex payload) | Yes |
| Config -> Filter chain | `rpc.FilterInsertion` conditioned on `llgrConfig` path | Yes |
| CLI `create bgp peer` -> Reactor | `setGracefulRestart` writes `short-lived / restart-time` | Yes |

### Integration Points
- `registry.Registration.InProcessConfigVerifier` (`internal/component/bgp/plugins/gr/register.go`) - matches offline `ze config validate` with runtime `OnConfigVerify` / `OnConfigure`.
- `config.CheckFilterInsertionPaths` - verifies `llgrConfig` path matches `ze-graceful-restart.yang`.
- `reactor.parseCapabilitiesFromTree` (`internal/component/bgp/reactor/config_capabilities.go`) - reads `short-lived / restart-time` into `RawCapabilityConfig`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | YANG -> `ToPluginMap` -> `refuseUncarriedGRSections` -> `grCapabilities` -> `sdk.CapabilityDecl` |
| No unintended coupling (components stay isolated) | Yes | `./le arch tier check` and `./le plugin boundary check` pass |
| No duplicated functionality (extends existing, does not recreate) | Yes | Shared `grCapabilityFor` and `refusePeerPhaseFamilies` serve both `short-lived` and `long-lived` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | No hot-path wire allocation added; config extraction runs only at load/reload |
| Registration over hardcoding, outbound | Yes | Capabilities, filters, insertions, and in-process verifier register in `gr/register.go`; `./le plugin declarations check` passes |
| Registration over hardcoding, inbound | Yes | Searched `internal/` for `long-lived-stale-time` and `restart-time`; only `gr/`, `reactor/config_capabilities.go`, and `cmd/peer/create.go` read the config path |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Compact list formatting works for both 1-leaf and 2-leaf `list family` | `internal/component/config/flatten.go` | Operators would need verbose nested blocks | `test/parse/graceful-restart-llgr-per-family.ci` and `internal/component/config/serialize_test.go` | confirmed |
| A-2 | Omitting `short-lived` with `long-lived` present sends code 64 `0000` and code 71 tuples | RFC 9494 §4.1, `parseGRCapValue` | GR-phase skip would break | `TestLLGRSkipGRPhaseOmitsShortLived` | confirmed |
| A-3 | Peer `graceful-restart` container replaces group container for both code 64 and code 71 | `ze-graceful-restart.yang`, `grCapabilityFor` | Group LLGR would leak onto peers overriding GR | `TestLLGRPeerContainerReplacesGroupLLGR` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Converted configs silently change advertised OPENs | Functional `.ci` OPEN hex mismatch | Converted all 66 `test/` files preserving exact family sets and ran parse, reload, plugin, runner, and UI suites |
| R-2 | Offline `ze config validate` accepts uncarried `long-lived` family that boot refuses | `config-validate-agrees-with-boot.ci` or parse test fails | Wired `InProcessConfigVerifier: refuseUncarriedGRSections` in `gr/register.go` and verified in `test/parse/graceful-restart-llgr-per-family.ci` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Code 64 and code 71 capability bytes in OPEN; config validation of `graceful-restart` blocks |
| How is it reverted? | Single commit revert restores prior schema, code, tests, and docs |
| Who else touches this path? | `internal/component/bgp/plugins/gr/`, `internal/component/bgp/reactor/config_capabilities.go`, `internal/component/bgp/plugins/cmd/peer/create.go` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate` / `ze start` config parse | -> | `ze-graceful-restart.yang`, `refuseUncarriedGRSections`, `extractGRCapabilities`, `extractLLGRCapabilities` | `test/parse/graceful-restart-llgr-per-family.ci` |
| `bgp-gr` Stage 2/3 capability declaration | -> | `grCapabilities`, `parseGRCapValue`, `parseLLGRCapValue` | `TestRFC9494_PerFamilyLLGRConfigPositive`, `TestRFC9494_PerFamilyLLGRConfigNegative` |
| Filter insertion loader | -> | `grInsertions` (`llgrConfig`) | `internal/component/bgp/plugins/gr/gr_filter_test.go`, `test/plugin/llgr-stale-received-depreferenced.ci` |
| `create bgp peer ... graceful-restart <N>` | -> | `setGracefulRestart` | `TestCreateBgpPeerReachesTheReactorFromTheCommandLine` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer carries `ipv4/unicast` and `ipv6/unicast`; LLGR configured for `ipv4/unicast` only | Ze's OPEN carries code 71 naming `ipv4/unicast` only; helper-side `exchangedLLGRLocked` runs LLGR only for `ipv4/unicast` and purges `ipv6/unicast` at GR end |
| AC-2 | Two families configured with different stale times | Each code 71 tuple carries its own Long-Lived Stale Time |
| AC-3 | No `long-lived` configuration | No code 71 (RFC 9494 Section 5, MUST NOT by default) |
| AC-4 | Family in `long-lived / family` and `short-lived` omitted (or narrowed) | Code 71 names it while code 64 omits it (RFC 9494 Section 4.1 GR-phase skip) |
| AC-5 | Existing configs converted; deleted legacy `long-lived-stale-time` rejected | All in-tree configs converted with byte-identical OPENs; legacy `long-lived-stale-time` refused by YANG validation |
| AC-6 | RFC9494-5-2 | `{gap}` removed; tagged units in both polarities with discrimination records, independently audited `enforced` (`rfc/audit/rfc9494/RFC9494-5-2.json`) |
| AC-7 | YANG and docs | Native range validation (`0..16777215`), `docs/guide/graceful-restart.md` and config syntax docs updated, old leaf removed |
| AC-8 | Interop | `bgp-llgr-per-family-frr` (`test/interop/scenarios/bgp-llgr-per-family-frr/`, `internal/le/interoplab/bgp/check_llgr_per_family.go`) passed live against FRR 10.7.1 (`NO_BUILD=1 INTEROP_SCENARIO=bgp-llgr-per-family-frr ./le --name llgr-interop test integration interop`: exit 0, `1 passed, 0 failed`, 15s; recording `tmp/session/2026-11-21-023904/scratch/interop-recordings/bgp/bgp-llgr-per-family-frr-BZY3NKEXIWGG4AI7KQXLM2WWJT.jsonl` confirms FRR saw code 64 for both `ipv4Unicast` and `ipv6Unicast`, code 71 for `IPv4 Unicast` only, `ipv4Unicast.timers.llgrStaleTime: 60`, and `ipv6Unicast.timers.llgrStaleTime: 0`) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures per-family LLGR on a multi-family peer (`ipv4/unicast 3600`, omitting `ipv6/unicast`) | YANG -> `refuseUncarriedGRSections` -> `extractLLGRCapabilities` -> OPEN code 71 | `test/parse/graceful-restart-llgr-per-family.ci`, `TestRFC9494_PerFamilyLLGRConfigPositive` |
| 2 | Skips conventional GR by writing `graceful-restart { long-lived { family { ipv4/unicast 3600; } } }` | YANG -> `parseGRCapValue` (`0000`) + `parseLLGRCapValue` | `TestLLGRSkipGRPhaseOmitsShortLived`, `test/parse/graceful-restart-llgr-per-family.ci` |
| 3 | Misconfigures an uncarried family under `long-lived` or writes legacy `long-lived-stale-time` | YANG / `InProcessConfigVerifier` -> clear refusal | `test/parse/graceful-restart-llgr-per-family.ci`, `TestLLGRRefusesUncarriedFamilyAndInvalidStaleTime` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC9494_PerFamilyLLGRConfigPositive` | `internal/component/bgp/plugins/gr/rfc9494_llgr_per_family_test.go` | Per-family inclusion and distinct LLST encoding (`RFC9494-5-2 positive`) | pass |
| `TestRFC9494_PerFamilyLLGRConfigNegative` | `internal/component/bgp/plugins/gr/rfc9494_llgr_per_family_test.go` | Unconfigured peer family excluded from code 71 (`RFC9494-5-2 negative`) | pass |
| `TestLLGRSkipGRPhaseOmitsShortLived` | `internal/component/bgp/plugins/gr/rfc9494_llgr_per_family_test.go` | Omitting `short-lived` sends code 64 `0000` with code 71 intact | pass |
| `TestLLGRRefusesUncarriedFamilyAndInvalidStaleTime` | `internal/component/bgp/plugins/gr/rfc9494_llgr_per_family_test.go` | Refusal of uncarried families and out-of-range/negative/non-integer stale times | pass |
| `TestLLGRPeerContainerReplacesGroupLLGR` | `internal/component/bgp/plugins/gr/rfc9494_llgr_per_family_test.go` | Peer `graceful-restart` container replaces group container for both code 64 and code 71 | pass |
| `TestLLGRPerFamilyFRROutput` | `internal/le/interoplab/bgp/check_llgr_test.go` | FRR capability parser distinguishes single-family code 71 from all-family code 71 | pass |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `short-lived / restart-time` | 0-4095 | 4095 | -1 (uint16 / YANG range) | 4096 |
| `long-lived / family / stale-time` | 0-16777215 | 16777215 | -1 (uint32 / YANG range) | 16777216 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `graceful-restart-llgr-per-family` | `test/parse/graceful-restart-llgr-per-family.ci` | Valid symmetric GR + per-family LLGR + GR-skip, JSON dump, refusal of uncarried family and legacy syntax (stress-tested 80 iterations) | pass |
| `reload-gr-explicit-family-list-accepted`, `reload-gr-family-change-restarts` | `test/reload/` | Live SIGHUP reload of `short-lived / family` | pass |
| `graceful-restart`, `graceful-restart-group`, `graceful-restart-rib`, `llgr-transition`, `llgr-import-no-llgr`, `llgr-stale-received-depreferenced`, `llgr-egress-state-unloaded` | `test/plugin/` | End-to-end daemon OPEN and LLGR runtime procedures | pass |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-llgr-per-family-frr` | `test/interop/scenarios/bgp-llgr-per-family-frr/` | FRR 10.7.1 | Dual-stack peer advertises both families in code 64 and only `ipv4/unicast` in code 71 | pass (live FRR 10.7.1, 15s, `ipv4Unicast.llgrStaleTime=60`, `ipv6Unicast.llgrStaleTime=0`) |

## Files to Modify
- `internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang` - symmetric `short-lived` and `long-lived` containers; old per-peer leaf removed
- `internal/component/bgp/plugins/gr/gr_capability.go` - parse `short-lived` and validate both phase family lists + `stale-time`
- `internal/component/bgp/plugins/gr/gr_llgr.go` - build code 71 strictly from `long-lived / family` using `grCapabilityFor`
- `internal/component/bgp/plugins/gr/gr_filter.go` - update `llgrConfig` insertion path and reason text
- `internal/component/bgp/plugins/gr/register.go` - wire `InProcessConfigVerifier: refuseUncarriedGRSections`
- `internal/component/bgp/plugins/gr/gr.go`, `gr_llgr_exchange.go` - update comments
- `internal/component/bgp/reactor/config_capabilities.go` - read `short-lived / restart-time`
- `internal/component/bgp/plugins/cmd/peer/create.go` - write `short-lived / restart-time`
- `internal/component/config/schema_origin.go` - update comment
- `internal/le/interoplab/bgp/register_llgr.go`, `check_llgr_test.go` - register and unit-test `bgp-llgr-per-family-frr`
- `rfc/short/rfc9494.md` - remove `{gap}` on `RFC9494-5-2`
- `docs/guide/graceful-restart.md`, `docs/architecture/config/syntax.md`, `docs/architecture/api/process-protocol.md`, `docs/guide/bgp-resilience.md`, `docs/features/configuration.md`, `docs/guide/configuration.md`, `docs/config-reference.md` - document new syntax and design decision
- Existing unit tests and 66 `.ci` / interop configs converted to the new syntax

## Files to Create
- `internal/component/bgp/plugins/gr/rfc9494_llgr_per_family_test.go` - per-family LLGR unit tests and `RFC9494-5-2` tagged units
- `test/parse/graceful-restart-llgr-per-family.ci` - end-to-end parse/validation functional test
- `test/interop/scenarios/bgp-llgr-per-family-frr/ze.conf`, `frr.conf` - FRR interop scenario
- `internal/le/interoplab/bgp/check_llgr_per_family.go` - FRR interop checker

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang`; verified by `./le yang glue check` and `./le doc yang-contract help-shape` |
| YANG validation constraints | Yes | `range "0..4095"` on `restart-time`, `range "0..16777215"` + `mandatory true` on `stale-time`, `type zt:address-family` on `name` |
| YANG custom validators | Yes | `ze:validate "registered-address-family"` on both `short-lived/family/name` and `long-lived/family/name`; cross-field check in `InProcessConfigVerifier` |
| CLI commands/flags | Yes | `create bgp peer ... graceful-restart <N>` updated in `internal/component/bgp/plugins/cmd/peer/create.go` |
| CLI grammar (keyword before value) | N-A | No new CLI verb or keyword added |
| Editor autocomplete | Yes | Automatic from YANG schema; verified by `test/ui/cli-completion-full-config.ci` |
| Functional test for new RPC/API | Yes | `test/parse/graceful-restart-llgr-per-family.ci` |
| Pipe completeness | N-A | No new show/output command added |
| Env var registration | N-A | No environment variables added |
| Doctor check for runtime dependencies | N-A | No new runtime dependency added |
| Prometheus counters/metrics | N-A | Existing GR metrics unchanged |
| BGP family surface (new SAFI / capability / attribute) | N-A | Existing codes 64 and 71 only; no new SAFI, capability code, or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Existing LLGR feature refined to per-family syntax |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md`, `docs/features/configuration.md`, `docs/config-reference.md` |
| 3 | CLI command added/changed? | No | `create bgp peer` CLI syntax unchanged |
| 4 | API/RPC added/changed? | No | No RPC paths changed |
| 5 | Plugin added/changed? | No | Existing `bgp-gr` plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/graceful-restart.md`, `docs/guide/bgp-resilience.md` |
| 7 | Wire format changed? | No | Code 64 and code 71 wire formats unchanged (RFC 4724 / RFC 9494) |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` (updated config path example for `llgrConfig`) |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9494.md` (`RFC9494-5-2` `{gap}` removed) and `docs/features/rfc-status.md` (regenerated by `./le rfc update`) |
| 10 | Test infrastructure changed? | No | Existing runner and interoplab frameworks reused |
| 11 | Affects daemon comparison? | No | LLGR already supported |
| 12 | Internal architecture changed? | No | Same plugin boundary and capability declaration path |
| 13 | Route metadata keys added/changed? | No | Existing `stale` metadata key unchanged |
| 14 | Prometheus counters added/changed? | No | No metric changes |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Same plugin and capability codes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Verified with `./le doc check all` (0 drift, all anchors valid) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | All examples updated across the 7 doc files above |

## Implementation Steps

1. **Phase 1: Schema & Wiring** -- Replace `ze-graceful-restart.yang`, wire `InProcessConfigVerifier` in `gr/register.go`, update `llgrConfig` in `gr_filter.go`, `reactor/config_capabilities.go`, and `cmd/peer/create.go`.
2. **Phase 2: Capability Extraction & Validation** -- Update `gr_capability.go` and `gr_llgr.go`; add `rfc9494_llgr_per_family_test.go`; convert existing unit tests.
3. **Phase 3: Functional Tests, Docs, RFC Proofs & Interop** -- Convert all 66 in-tree `.ci` and interop configs; add and stress-test `test/parse/graceful-restart-llgr-per-family.ci`; update docs; record discrimination proofs and run independent `ze-test-judge` and `ze-rfc-audit`; add `bgp-llgr-per-family-frr` interop scenario.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-8 verified, including live Docker FRR 10.7.1 interop pass for `bgp-llgr-per-family-frr` |
| Feature completeness | Config parse, offline validation, commit verification, Stage 2/3 capability declaration, filter insertion, and `create bgp peer` all use the new paths |
| Correctness | Peer `graceful-restart` container replaces group container for both code 64 and code 71; `short-lived` absence emits `0000`; `long-lived` emits only listed families |
| Naming | Kebab-case YANG nodes (`short-lived`, `long-lived`, `restart-time`, `stale-time`); wire decode JSON key `long-lived-stale-time` preserved |
| Data flow | Resolution stays in `bgp-gr` and `config`; reactor only reads `short-lived / restart-time` for `RawCapabilityConfig` |
| Rule: no-layering | Old `long-lived-stale-time`, top-level `restart-time`, and `family { name [...] }` removed from YANG, parser, and docs |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| YANG schema + glue + help-shape | `./le yang glue check`, `./le doc yang-contract help-shape` |
| Unit tests & `RFC9494-5-2` proofs | `./le test unit pkg path ./internal/component/bgp/plugins/gr/`, `./le rfc discriminate stem rfc9494` |
| Functional tests | `./le test bgp parse --pattern graceful-restart`, `./le test bgp reload --pattern reload-gr`, `./le test bgp plugin --pattern graceful-restart` |
| Docs & anchors | `./le doc check all`, `./le config claims` |
| Interop scenario `bgp-llgr-per-family-frr` | Live Docker run against FRR 10.7.1 (`NO_BUILD=1 INTEROP_SCENARIO=bgp-llgr-per-family-frr ./le --name llgr-interop test integration interop`): exit 0, `1 passed, 0 failed`, 15s |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Native YANG range `0..4095` and `0..16777215`, `registered-address-family` validator, plus `refusePeerPhaseFamilies` rejecting negative, non-integer, out-of-range, and uncarried families at offline validate, commit, and boot |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read source in Current Behavior |
| Lint failure | Fix inline |
| Live Docker interop blocked by unrelated concurrent session kernelcap edits | Keep spec open at `Status: verification`, report exact evidence to owner |

## Design Insights
- Nesting `short-lived` and `long-lived` symmetrically inside `container graceful-restart` eliminates any need for a cross-container validation rule enforcing RFC 9494 §3.1/§4.1 while making helper-only GR and GR-phase skip a single clean concept (omitting `short-lived`).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Symmetric `short-lived` and `long-lived` containers inside `container graceful-restart`, both with keyed `list family` | (A) Sibling `long-lived` container alongside flat `restart-time` and `family { name [...] }`; (B) Unified `family` list with GR/LLGR flags | Owner-selected (2026-11-22): nesting inside `graceful-restart` structurally prevents configuring code 71 without code 64 (RFC 9494 §3.1/§4.1), keeps identical `family { ... }` syntax for both phases, and omitting `short-lived` naturally expresses both helper-only GR (RFC 4724 §3) and GR-phase skip (RFC 9494 §4.1) without a `helper-only` flag |
| Pre-release clean cut (no legacy fallback) | Keep runtime reader for `long-lived-stale-time` | `ai/rules/no-layering.md` and `docs/architecture/config/deprecated-options.md` forbid legacy shims pre-release; all 66 in-tree configs converted directly |

## Known Limitations
- None for RFC9494-5-2. (AC-8 live Docker interop `bgp-llgr-per-family-frr` was unblocked by `17bbd4a7df` and passed live against FRR 10.7.1.)

## RFC Documentation (Scope: protocol)

`// RFC 9494 Section 3.1:`, `// RFC 9494 Section 4.1:`, `// RFC 9494 Section 5:`, and `// RFC 4724 Section 3:` quotes placed directly above enforcing code in `gr_capability.go` and `gr_llgr.go`.

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
- [ ] Required Reading carries `-> Decision:` / `-> Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated (including AC-8 live FRR 10.7.1 Docker interop)
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-user behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] Commit A lands closure edits + news fragment + citer/allowlist cleanup + spec; Commit B deletes spec

---

## Implementation Summary

### What Was Implemented
- Replaced legacy flat `graceful-restart` schema (`restart-time`, `family { name [...] }`, `long-lived-stale-time`) with symmetric `short-lived` (presence container, `restart-time` `0..4095`, keyed `list family`) and `long-lived` (keyed `list family` with mandatory `stale-time` `0..16777215`) in `internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang` (landed in handoff commit `3c2a055fd953d19c911fdf3538c101e31c6fab5b`).
- Updated `gr_capability.go` (`parseGRCapValue`, `grAdvertisedFamilies`, `refuseUncarriedGRSections`, `refusePeerGRFamilies`, `refusePeerPhaseFamilies`), `gr_llgr.go` (`parseLLGRCapValue`, `extractLLGRCapabilities`), `gr_filter.go` (`llgrConfig`, `llgrRequiredReason`), `register.go` (`InProcessConfigVerifier: refuseUncarriedGRSections`), and `doctor.go` (`shortLivedCapabilityIssues`, `longLivedCapabilityIssues`).
- Updated peer-over-group replacement in `internal/component/bgp/config/resolve.go` (`replacePaths`), reactor capability reading in `internal/component/bgp/reactor/config_capabilities.go`, dynamic peer creation in `internal/component/bgp/plugins/cmd/peer/create.go` (`setGracefulRestart`), and ExaBGP migration in `internal/exabgp/migration/migrate_defaults.go` (`migratedCapabilityContainer`).
- Added unit tests and `RFC9494-5-2` proofs (`internal/component/bgp/plugins/gr/rfc9494_llgr_per_family_test.go`, `rfc/discrimination/rfc9494/RFC9494-5-2.json`, `rfc/audit/rfc9494/RFC9494-5-2.json`), functional test `test/parse/graceful-restart-llgr-per-family.ci`, converted all 66 in-tree configs/tests, and added FRR 10.7.1 interop scenario `bgp-llgr-per-family-frr` (`test/interop/scenarios/bgp-llgr-per-family-frr/`, `internal/le/interoplab/bgp/check_llgr_per_family.go`).
- Removed stale `internal/le/repo/numberparse-allowlist.txt` entry for `internal/component/bgp/plugins/gr/gr_llgr.go`, cleared all tracked citers of the closing spec, and wrote operator news fragment `website/changes/fragments/bgp-llgr-per-family-config.md`.

### Bugs Found/Fixed
- Stale allowlist line `1 internal/component/bgp/plugins/gr/gr_llgr.go` in `internal/le/repo/numberparse-allowlist.txt` left behind after `gr_llgr.go` switched from `strconv.ParseUint` to `configvalue.Int` (flagged by `./le repo check`); removed in closure Commit A.
- `docs/guide/graceful-restart.md` `## Design decisions` row cited `plan/release/fixes/spec-bgp-llgr-per-family-config.md` in `Source`, which would become dangling upon spec deletion; repointed to landed commit `3c2a055fd9` in closure Commit A (requires owner terminal `decision-change` per `internal/le/doc/consistency/decisionschange.go:judgeDecisionRows`).

### Documentation Updates
- `docs/guide/graceful-restart.md`: updated config syntax, per-family LLGR documentation, source anchors (`gr_capability.go`, `gr_llgr.go`, `gr_llgr_exchange.go`, `gr_filter.go`, `ze-graceful-restart.yang`), and `## Design decisions` row (`Source` repointed to `3c2a055fd9, plan/records/rfc-verdict-test-fix-pass-rulings.md`).
- `docs/architecture/config/syntax.md`, `docs/config-reference.md`, `docs/features/configuration.md`, `docs/guide/bgp-resilience.md`, `docs/guide/configuration.md`, `docs/architecture/api/process-protocol.md`: updated config examples and `llgrConfig` path (`3c2a055fd9`).
- `rfc/short/rfc9494.md`: removed `{gap}` on `RFC9494-5-2`; `./le rfc update` confirmed `docs/features/rfc-status.md` is current with zero diff.
- `./le doc check all`: `Documentation tests PASSED` (0 drift, 562/562 commands, 3160/3160 digest anchors).

### Deviations from Plan
- Live Docker interop (AC-8) was initially blocked at handoff commit `3c2a055fd9` by unrelated host kernel capability probes; `17bbd4a7df` unblocked it and `bgp-llgr-per-family-frr` subsequently passed live against FRR 10.7.1.
- Handoff commit `3c2a055fd9` landed with trailer `Spec-work: pppoe-vpp-dataplane` from a concurrent session claim; closure review artifact explicitly pins all 85 code files from `3c2a055fd9`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | `3c2a055fd9` removed `strconv.ParseUint` from `gr_llgr.go` without removing its line in `internal/le/repo/numberparse-allowlist.txt` | Exact-count allowlist forbids stale entries whose file parses no 32-bit integer | `./le repo check` during closure step 5 | Removed stale line in closure Commit A |
| approach | `3c2a055fd9` wrote `plan/release/fixes/spec-bgp-llgr-per-family-config.md` into `docs/guide/graceful-restart.md` `## Design decisions` `Source` | A closing spec path cannot survive spec removal in `## Design decisions`; modifying any cell (including `Source`) triggers `LostDecisions` (`decisions.go:DecisionRow.key`) | Closure citer check + `decisionschange.go` audit | Repointed `Source` to `3c2a055fd9` and flagged owner-terminal `decision-change` on Commit A |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Per-AFI/SAFI LLGR affirmative config (RFC 9494 §5) | Done | `internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang:81-117`, `gr_llgr.go:143-221` | `long-lived / family` keyed list with mandatory `stale-time` (`0..16777215`) |
| Symmetric `short-lived` / `long-lived` inside `graceful-restart` (RFC 9494 §3.1/§4.1) | Done | `ze-graceful-restart.yang:28-118`, `gr_capability.go:117-221` | Omitting `short-lived` sends code 64 `0000` |
| Remove legacy per-peer leaves (`no-layering`) | Done | `ze-graceful-restart.yang`, `test/parse/graceful-restart-llgr-per-family.ci:104-127,170-172` | Legacy `long-lived-stale-time`, top-level `restart-time`, and `family { name [...] }` deleted and refused |
| Offline/commit/boot validation parity | Done | `internal/component/bgp/plugins/gr/register.go:94`, `gr_capability.go:227-366` | `InProcessConfigVerifier: refuseUncarriedGRSections` matches `OnConfigVerify` / Stage 2 |
| Peer-over-group replacement parity | Done | `internal/component/bgp/config/resolve.go:41-43,560-563`, `gr_capability.go:287-295`, `gr_llgr.go:242` | `replacePaths` matches `grCapabilityFor` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRFC9494_PerFamilyLLGRConfigPositive`, `TestRFC9494_PerFamilyLLGRConfigNegative`, `bgp-llgr-per-family-frr` | Code 71 names `ipv4/unicast` only; `exchangedLLGRLocked` purges `ipv6/unicast` |
| AC-2 | Done | `TestRFC9494_PerFamilyLLGRConfigPositive/distinct_stale_times_and_boundaries` | `0001018000000000020180ffffff` (`0` and `16777215`) |
| AC-3 | Done | `TestRFC9494_PerFamilyLLGRConfigNegative`, `TestRFC9494_LLGRNotEnabledByDefault` | Absent or empty `long-lived` emits no code 71 |
| AC-4 | Done | `TestLLGRSkipGRPhaseOmitsShortLived`, `test/parse/graceful-restart-llgr-per-family.ci` | Omitting `short-lived` emits code 64 `0000` with code 71 intact |
| AC-5 | Done | 66 converted in-tree configs + `TestLLGRRefusesUncarriedFamilyAndInvalidStaleTime` + `test/parse/graceful-restart-llgr-per-family.ci` | Legacy leaves refused by YANG |
| AC-6 | Done | `rfc/short/rfc9494.md:234`, `rfc/discrimination/rfc9494/RFC9494-5-2.json`, `rfc/audit/rfc9494/RFC9494-5-2.json` | `{gap}` removed, audited `enforced` |
| AC-7 | Done | `ze-graceful-restart.yang:105-115`, `docs/guide/graceful-restart.md`, `./le doc check all` | Native `0..16777215` range + 7 docs updated |
| AC-8 | Done | `bgp-llgr-per-family-frr` live Docker run against FRR 10.7.1 | Passed in 15s (`ipv4Unicast.llgrStaleTime: 60`, `ipv6Unicast.llgrStaleTime: 0`) |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRFC9494_PerFamilyLLGRConfigPositive` | Done | `internal/component/bgp/plugins/gr/rfc9494_llgr_per_family_test.go:53` | Tagged `RFC9494-5-2 positive` |
| `TestRFC9494_PerFamilyLLGRConfigNegative` | Done | `internal/component/bgp/plugins/gr/rfc9494_llgr_per_family_test.go:106` | Tagged `RFC9494-5-2 negative` |
| `TestLLGRSkipGRPhaseOmitsShortLived` | Done | `internal/component/bgp/plugins/gr/rfc9494_llgr_per_family_test.go:154` | Proves code 64 `0000` + code 71 |
| `TestLLGRRefusesUncarriedFamilyAndInvalidStaleTime` | Done | `internal/component/bgp/plugins/gr/rfc9494_llgr_per_family_test.go:174` | Boundary + legacy refusal + `InProcessConfigVerifier` |
| `TestLLGRPeerContainerReplacesGroupLLGR` | Done | `internal/component/bgp/plugins/gr/rfc9494_llgr_per_family_test.go:223` | Peer container replaces group LLGR |
| `TestLLGRDefensiveValidationAndTupleCap` | Done | `internal/component/bgp/plugins/gr/rfc9494_llgr_per_family_test.go:243` | 36-tuple cap + RIB-less family + malformed JSON |
| `TestLLGRPerFamilyFRROutput` | Done | `internal/le/interoplab/bgp/check_llgr_test.go` | Parser unit test |
| `graceful-restart-llgr-per-family` | Done | `test/parse/graceful-restart-llgr-per-family.ci` | Parse + JSON + refusals (80x stress pass) |
| `bgp-llgr-per-family-frr` | Done | `test/interop/scenarios/bgp-llgr-per-family-frr/` | Live Docker FRR 10.7.1 pass |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang` | Done | Modified in `3c2a055fd9` |
| `internal/component/bgp/plugins/gr/gr_capability.go` | Done | Modified in `3c2a055fd9` |
| `internal/component/bgp/plugins/gr/gr_llgr.go` | Done | Modified in `3c2a055fd9` |
| `internal/component/bgp/plugins/gr/gr_filter.go` | Done | Modified in `3c2a055fd9` |
| `internal/component/bgp/plugins/gr/register.go` | Done | Modified in `3c2a055fd9` |
| `internal/component/bgp/plugins/gr/doctor.go` | Done | Modified in `3c2a055fd9` |
| `internal/component/bgp/config/resolve.go` | Done | Modified in `3c2a055fd9` |
| `internal/component/bgp/reactor/config_capabilities.go` | Done | Modified in `3c2a055fd9` |
| `internal/component/bgp/plugins/cmd/peer/create.go` | Done | Modified in `3c2a055fd9` |
| `internal/exabgp/migration/migrate_defaults.go` | Done | Modified in `3c2a055fd9` |
| `internal/component/bgp/plugins/gr/rfc9494_llgr_per_family_test.go` | Done | Created in `3c2a055fd9` |
| `test/parse/graceful-restart-llgr-per-family.ci` | Done | Created in `3c2a055fd9` |
| `test/interop/scenarios/bgp-llgr-per-family-frr/{ze.conf,frr.conf}` | Done | Created in `3c2a055fd9` |
| `internal/le/interoplab/bgp/check_llgr_per_family.go` | Done | Created in `3c2a055fd9` |
| `rfc/short/rfc9494.md` + 7 docs + 66 `.ci`/interop configs | Done | Modified in `3c2a055fd9` |

### Audit Summary
- **Total items:** 28
- **Done:** 28
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Make LLGR (RFC 9494 §5) an affirmative per-AFI/SAFI operator decision | interop + unit + RFC discrimination | Live Docker FRR 10.7.1 `bgp-llgr-per-family-frr` (`NO_BUILD=1 INTEROP_SCENARIO=bgp-llgr-per-family-frr ./le --name llgr-interop test integration interop`: exit 0, `1 passed, 0 failed`, 15s; recording `bgp-llgr-per-family-frr-BZY3NKEXIWGG4AI7KQXLM2WWJT.jsonl` shows `Graceful Restart: IPv4 Unicast, IPv6 Unicast`, `Long-lived Graceful Restart: IPv4 Unicast` only, `ipv4Unicast.llgrStaleTime: 60`, `ipv6Unicast.llgrStaleTime: 0`); `TestRFC9494_PerFamilyLLGRConfigPositive` & `TestRFC9494_PerFamilyLLGRConfigNegative` (`rfc/discrimination/rfc9494/RFC9494-5-2.json`, `rfc/audit/rfc9494/RFC9494-5-2.json`) |
| Symmetric `short-lived` / `long-lived` config inside `graceful-restart` with GR-phase skip and legacy removal | functional + unit | `test/parse/graceful-restart-llgr-per-family.ci` (80x stress pass: accepts symmetric config & GR-skip, dumps `stale-time`, refuses uncarried `ipv6/unicast`, legacy `long-lived-stale-time`, and `16777216`); `TestLLGRSkipGRPhaseOmitsShortLived`; `TestLLGRRefusesUncarriedFamilyAndInvalidStaleTime` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | All AC-1..AC-8 and RFC9494-5-2 deliverables are complete and verified | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/bgp-llgr-per-family-config-1c2a7b0e-fafc-4e6c-beac-67f60f6e67d0.md` |
| `./le spec review check` | `review_gate: OK (85 code files, clean, hashes match tmp/review/bgp-llgr-per-family-config-1c2a7b0e-fafc-4e6c-beac-67f60f6e67d0.md)` |
| Rounds | 1 (0 BLOCKER, 0 ISSUE in product code; non-code allowlist/citer findings fixed inline) |
| Reviewer lenses used | logic+wiring, security+edge-cases, rfc+style+spec (full `/ze-review` + `/ze-review-spec` inline over `3c2a055fd9~1..3c2a055fd9` + closure edits) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Stale `1 internal/component/bgp/plugins/gr/gr_llgr.go` line in `numberparse-allowlist.txt` after `gr_llgr.go` removed `strconv.ParseUint` (`./le repo check`) | `internal/le/repo/numberparse-allowlist.txt:48` | Deleted stale line in closure Commit A |
| 2 | ISSUE | Tracked citers referenced `plan/release/fixes/spec-bgp-llgr-per-family-config.md` (`docs/guide/graceful-restart.md:759`, `spec-bgp-graceful-restart-rfc-defects.md:801,807`, `spec-bgp-llgr-reconnect-timers.md:142,313`) | `docs/guide/graceful-restart.md:759`, sibling specs | Repointed `Design decisions` `Source` to `3c2a055fd9` and restated sibling spec refs as bare stem `spec-bgp-llgr-per-family-config` |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/plugins/gr/rfc9494_llgr_per_family_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 14278 Nov 22 20:08` |
| `test/parse/graceful-restart-llgr-per-family.ci` | Yes | `-rw-r--r-- 1 thomas thomas 3692 Nov 22 20:06` |
| `test/interop/scenarios/bgp-llgr-per-family-frr/ze.conf` | Yes | `-rw-r--r-- 1 thomas thomas 1087 Nov 22 19:49` |
| `test/interop/scenarios/bgp-llgr-per-family-frr/frr.conf` | Yes | `-rw-r--r-- 1 thomas thomas 531 Nov 22 19:49` |
| `internal/le/interoplab/bgp/check_llgr_per_family.go` | Yes | `-rw-r--r-- 1 thomas thomas 5333 Nov 22 20:05` |
| `website/changes/fragments/bgp-llgr-per-family-config.md` | Yes | Created in closure Commit A |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Per-family code 71 & helper-side purge of unconfigured family | `gr_llgr.go:parseLLGRCapValue`, `rfc9494_llgr_per_family_test.go:TestRFC9494_PerFamilyLLGRConfigPositive`, `TestRFC9494_PerFamilyLLGRConfigNegative` |
| AC-2 | Distinct per-family LLST values (`0`, `16777215`) | `rfc9494_llgr_per_family_test.go:72-85` asserts `"0001018000000000020180ffffff"` |
| AC-3 | No code 71 by default | `gr_llgr.go:152-161` returns `""` when `long-lived` or `family` absent; `TestRFC9494_PerFamilyLLGRConfigNegative` |
| AC-4 | Omitting `short-lived` sends code 64 `0000` with code 71 intact | `gr_capability.go:126-138` returns `"0000"`; `TestLLGRSkipGRPhaseOmitsShortLived` |
| AC-5 | In-tree configs converted; legacy leaves refused | `ze-graceful-restart.yang` removes `long-lived-stale-time`; `test/parse/graceful-restart-llgr-per-family.ci:170-172` |
| AC-6 | `RFC9494-5-2` `{gap}` removed, proven, audited `enforced` | `rfc/short/rfc9494.md:234`, `rfc/discrimination/rfc9494/RFC9494-5-2.json`, `rfc/audit/rfc9494/RFC9494-5-2.json`; `./le rfc check` owed |
| AC-7 | YANG range `0..16777215` and 7 docs updated | `ze-graceful-restart.yang:106-110`; `./le doc check all` -> `Documentation tests PASSED` |
| AC-8 | Live FRR 10.7.1 interop `bgp-llgr-per-family-frr` passes | `NO_BUILD=1 INTEROP_SCENARIO=bgp-llgr-per-family-frr ./le --name llgr-interop test integration interop`: exit 0, `1 passed, 0 failed`, recording `bgp-llgr-per-family-frr-BZY3NKEXIWGG4AI7KQXLM2WWJT.jsonl` verdict `passed: true` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze config validate` / `ze cli -c "show config dump - \| json"` | `test/parse/graceful-restart-llgr-per-family.ci` | Read lines 1-175: exercises valid symmetric GR+LLGR, GR-skip, JSON dump, uncarried `ipv6/unicast` refusal via `InProcessConfigVerifier`, legacy refusal, and `16777216` range refusal |
| `bgp-gr` filter insertion (`llgrConfig`) | `test/plugin/llgr-stale-received-depreferenced.ci` | Exists (`4151` bytes), updated in `3c2a055fd9` to `short-lived`/`long-lived` syntax |
| SIGHUP reload of `short-lived / family` | `test/reload/reload-gr-explicit-family-list-accepted.ci`, `test/reload/reload-gr-family-change-restarts.ci` | Exist (`3104` / `4647` bytes), updated in `3c2a055fd9` |
| Live FRR 10.7.1 dual-stack session | `test/interop/scenarios/bgp-llgr-per-family-frr/{ze.conf,frr.conf}` | Read both files + live JSONL recording (`verdict.passed: true`) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `test/parse/graceful-restart-llgr-per-family.ci` compact `family { ipv4/unicast 3600; }` + `internal/component/config/serialize_test.go` |
| A-2 | confirmed | `gr_capability.go:126-138` returns `"0000"`; `TestLLGRSkipGRPhaseOmitsShortLived` |
| A-3 | confirmed | `gr_capability.go:grCapabilityFor`, `resolve.go:replacePaths`, `TestLLGRPeerContainerReplacesGroupLLGR` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Config syntax & user guides (7 files) | `ze-graceful-restart.yang`, `gr_capability.go`, `gr_llgr.go`, `gr_filter.go`; `./le doc check all` -> `Documentation tests PASSED` | Yes |
| `docs/guide/graceful-restart.md` `## Design decisions` | Line 759 `Source` repointed to `3c2a055fd9, plan/records/rfc-verdict-test-fix-pass-rulings.md` | Yes |
| RFC status (`rfc/short/rfc9494.md` & `docs/features/rfc-status.md`) | `./le rfc update` produced zero diff on `docs/features/rfc-status.md` | Yes |
| Non-applicable categories (CLI verb, RPC, metrics, wire format) | `git diff 3c2a055fd9~1..3c2a055fd9` touched no RPC/metric/wire format definitions; `git grep long-lived-stale-time` shows only wire decode JSON tag (`gr_llgr.go:30`), negative tests, and static site testdata | Yes |

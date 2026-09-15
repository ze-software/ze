# Spec: the fix pass for restated registries

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**RESUME HERE (second session, 2026-09-14 evening).** Wave 1 and wave 2 are
done in the working tree: the doctor-check and plugin-name corpora answer 0 and
10 rows, the YANG corpus 72, the family corpus 7, and every remaining row has a
disposition in Known Limitations below. What is left is the ONE decision in
"Open decision" and the closure route. The history, the gate's judgement rules
and the traps are in `plan/handover/restated-registry-fix-pass-2026-09-14.md`.

## Open decision: what silences an outcome-2 row

The gate's closed corpus (YANG enumerations) flags a Go literal that holds
every value of an enumeration and says "the two must agree". It cannot tell
which side is the declaration. This pass judged every row and found the Go side
is the declaration far more often than not: the table carries an IANA number,
a wire constant, a kernel name or a handler, and the model is the copy. The
repair for that shape is an agreement test in the owning package, which loads
the module, reads the enum at the leaf path the row names, compares both ways
and fails on a path holding no enumeration. 64 rows now carry one. The gate
cannot see a test, so those 64 rows stay in `report` and `check` blocks a
change set that touches any of their files.

Two readings, which is the owner's call:

| Reading | What the gate learns | Cost |
|---------|----------------------|------|
| A. A gated row is done | the closed corpus reads the `_test.go` files of the package (and, for a bottom-tier package, of the package its test lives in) for the leaf path string beside a model-loader call, and reports the row as `gated by TestX` rather than as a finding; a test that names a path no leaf holds is itself a finding | one reader in `internal/le/enumeration/corpus.go`; the 64 rows leave `report` and `check` |
| B. A gated row stays a finding | nothing; AC-1 is reworded so a row with a named agreement test counts as "named in this spec" | the report stops being a work list for the closed corpus, and `check` keeps blocking the 64 files |

**Decided: reading A (owner, 2026-09-14).** The gate learns a second marker,
`// enumeration: gated by TestX`, honoured only on a closed-corpus finding, only
when `TestX` exists in a `_test.go` the walk reads, and red where it suppresses
nothing. A gated row leaves the findings and `check` does not block on it, but
`report` still prints it in its own section, so the backlog stays visible. A
marker that gates a plain copy of the model is the misuse this marker is not
for: a plain copy derives.

## Task

`./le enumeration report` measures the Go literals that restate a live registry.
On 2026-09-14 it answered 230 rows. The gate that produced that number refuses
the NEXT one (spec-a-literal-restates-a-registry, closed 2026-09-14); it does not
end the ones already there, and the owner decided on 2026-09-14 that the backlog
stays visible in `report` rather than behind exemption markers.

This spec ends them. Each row is a second declaration of a set some registry
already holds, and two declarations of one fact drift. At least two have already
drifted into an answer an operator reads:

- The RIB spells the FlowSpec SAFI `flowspec` where the family registry spells it
  `flow`, so the two names for one family disagree.
- `readOnlyVerbs` omits `resolve`, which `command.Verbs` classifies as a read
  verb, so an agent is told `resolve` is daemon-mode when it is not.

That is why this sits in `plan/immediate/`: a CLI surface that answers wrongly is
the bucket's own test (`plan/README.md`).

## Required Reading

### Architecture Docs
- [ ] `ai/patterns/registration.md` - every registry that exists and what each holds
- [ ] `ai/rules/principles.md` - the two always-on directives a restated registry breaks
- [ ] `internal/le/enumeration/enumeration.go` - what the gate calls a copy, and the
  three narrowings that keep a declaration and another namespace out of the count

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/enumeration/report.go` - the row a reader acts on: the file, the
  symbol, the corpus, and how many of the unit's strings are registry keys

**Behavior to preserve:**
- `./le enumeration check` keeps blocking on what a change set introduces.

**Behavior to change:**
- Each fixed row derives its set from the registry, so `./le enumeration report`
  answers fewer rows and no row is silenced by a marker instead.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le enumeration report`, whose rows are the work list.

### Transformation Path
1. Read the current rows, grouped by corpus.
2. For each row, replace the literal with a call to the registry that owns the set.
3. Re-run the report and confirm the row is gone rather than suppressed.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| literal → registry | the fixed call site reads the registry at runtime | No |

### Integration Points
- `internal/le/enumeration` - the measurement this spec drives down.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every one of the 230 rows has a registry call that can replace it | the gate reports only a set a live registry holds | a row needs a registry that does not exist yet, and that row is its own spec | fix one row per corpus first and count what the call costs | unvalidated |
| A-2 | A row can be fixed without changing what the surface answers | the literal and the registry are meant to hold one set | the two disagree, and the drift is a defect of its own | assert the surface's answer before and after each fix | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Fixing a drifted copy changes a published answer | a functional test goes red on the corrected spelling | the drift IS the defect: correct the answer and the test that pinned the wrong one |
| R-2 | 230 rows is more than one session | the report count does not move | cut the work by corpus, one spec phase per corpus, and land each |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A surface answers from the registry instead of a stale copy, so a drifted name changes. An operator meets the corrected name |
| How is it reverted? | Each row is its own edit and its own commit |
| Who else touches this path? | Every component named in the report |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le enumeration report` | → | the fixed call sites | the row count falls, and no new exemption marker appears |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le enumeration report` after the pass | every remaining row is a set no registry holds, named in this spec |
| AC-2 | The FlowSpec SAFI name | the RIB and the family registry answer one spelling, proven by a test that reads both |
| AC-3 | `readOnlyVerbs` | derived from `command.Verbs`, so `resolve` is classified as the registry classifies it |
| AC-4 | Any fixed row | no exemption marker was added for it |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| per-row test at each fixed call site | with the fixed code | the surface answers from the registry | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| the FlowSpec family name an operator types | `test/` | one spelling reaches the RIB and the family registry | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | Scope is tooling; the FlowSpec row is a name, not a wire change | |

## Files to Modify
- Named by `./le enumeration report` at the time this spec runs, one file per row.

## Files to Create
- None expected; a row needing a registry that does not exist becomes its own spec.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no operator config changes |
| YANG validation constraints | N-A | as above |
| YANG custom validators | N-A | as above |
| CLI commands/flags | N-A | no command is added |
| CLI grammar (keyword before value) | N-A | as above |
| Editor autocomplete | N-A | as above |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no new answer |
| Env var registration | N-A | no new environment leaf |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Prometheus counters/metrics | N-A | no runtime metric |
| BGP family surface (new SAFI / capability / attribute) | Yes | the FlowSpec SAFI name is one of the rows |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | answered when the FlowSpec row is fixed |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `ai/patterns/registration.md` if a registry gains a reader |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | DERIVED | `./le spec citation anchors` during implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: one corpus at a time** -- read the report, take the rows of one corpus
   - Tests: a test at each fixed call site
   - Files: the report's own list
   - Verify: the report's tally for that corpus falls to zero
2. **Phase: the drifted rows first** -- FlowSpec and `readOnlyVerbs` answer wrongly today
   - Tests: AC-2 and AC-3
   - Verify: the two surfaces agree with the registry

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every row the report named is gone or is named here as needing a registry first |
| Correctness | No row was closed by adding an exemption marker |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The report count falls | `./le enumeration report` |
| No new marker | `git diff` holds no added `enumeration: exempt` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail-open | A surface that read a literal and now reads a registry must refuse an empty registry rather than answering nothing |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A row needs a registry that does not exist | Its own spec, named in Work Not Done |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Known Limitations
- A key set no registry holds cannot be fixed here. spec-a-literal-restates-a-registry
  named six such sets: functional test suite names, kernel RTPROTO names, IANA
  protocol names, ExaBGP API control words, iproute2 subcommands, and RFC 7752
  BGP-LS protocol-ID names. Each needs a registry to exist first, and each is its
  own spec.

### The rows that remain after the pass (AC-1), measured 2026-09-14 evening: 89

**Plugin names, 10 rows, none a copy.** Each is keyed on a namespace that shares
spellings with plugin names because a plugin is named after what it owns: YANG
top-level section names (`config/graph.go:sectionBGP`,
`config/validate_sections.go:validatedSections` and
`knownUnwalkedValidatorSections`, the last two with a recorded reason per
section), historical nftables table names an upgrade must keep spelling
(`firewall/legacy_tables.go`), IANA protocol names (`firewall/protocol.go`),
`ze support` module names, which are that command's own registry
(`support/modules.go`), prose and identifier capitalisation tables
(`le/site/plugins.go`, `le/yang/glue/yangglue.go`), and the command-YANG
migration's directory plan (`le/yang/migration/commands.go`, two rows). No
marker was added: the owner decided the backlog stays visible.

**Family names, 7 rows, blocked on one design decision.** `chaos/peer`,
`chaos/scenario`, `kernelcap`, `test/fixture` and `test/runner` each need a
non-base family name (flow, mpls-vpn, mpls-label, evpn, flow-vpn). Only four
families register unconditionally (`internal/core/family/registry.go`); every
other registration lives in an NLRI plugin behind the `ze_bgp` build tag, and
none of those five packages links one. Deriving there renders `afi-1/safi-128`.
`kernelcap.labeledFamilies` is the one that matters: it decides whether the
kernel is asked for an AF_MPLS table, so a registry miss would answer "no MPLS
needed", a fail-open guard. Named in Work Not Done as its own spec.

**YANG enumerations, 72 rows: 64 gated, 8 another namespace.** A gated row is
one where the Go table is the declaration (it carries the wire value, the IANA
number, the kernel name or the handler) and an agreement test in the owning
package reads the enum at the leaf path the row names and fails if either side
moves. The test is what closes the drift; whether the gate should then stop
reporting the row is the Open decision above.

| Package | Gated rows | Agreement test(s) |
|---------|-----------|-------------------|
| `bgp/config` | 1 (+ tests for `core/bgp/attribute`, `core/bgp/asn`) | `TestLeakFilterRolesMatchTheYANGModel`, `TestOriginTextNamesMatchTheYANGModel`, `TestASNotationTokensMatchTheYANGModel` |
| `bgp/filtertext`, `bgp/plugins/{bmp,cmd/update,filter_community,rib,role,rpki}`, `bgp/reactor` | 12 | `Test*MatchTheYANGModel` in each package; `core/bgp/msgtype` by `TestMessageTypeNamesMatchTheYANGModel` in `bgp/plugins/cmd/raw` |
| `cmd/show`, `mcp` (2), `pki`, `resolve/cmd`, `resolve/dns`, `resolve/irr`, `support`, `slogutil` (test in `config`), `as112`, `diag/cmd`, `geodns`, `host-cmd/cmd` | 14 | `*_yang_test.go` in each package, `TestLogBackendLeafMatchesSlogutil` in `config` |
| `ike/dataplane` (3), `ike/ipsec` (7), `l2tp/plugins/authradius`, `l2tp/ppp`, `radius`, `isis` (2), `ospf` (2), `ospf/packet`, `ospf/types` (2) | 20 | `TestVocabularyMatchesModel` and siblings in `*_vocabulary_test.go` |
| `config/archive`, `config/loader_extract`, `config/system`, `firewall` (7), `iface` (3), `traffic` (2), `ddos/detect` | 17 | `Test*MatchTheModel` in each package; `TestLimitUnitsMatchTheModel` in `anomaly/shape` for `firewall.rateUnitSeconds` |

The 8 another-namespace rows share a spelling with an enum by coincidence and
no code reads across: IKEv2 transform words matched to the certificate
fingerprint digest leaf (`ike/crypto/transform.go`); the interface list-name
namespace matched to the `migrate create type` leaf (`config/graph.go`,
`plugins/iface/ra/doctor.go`, `plugins/flowexport/register.go`,
`le/qemu/guest_linux.go`); the event-subscription direction vocabulary and the
plugin RPC wire vocabulary matched to the MRT `direction` leaf
(`core/events/events.go`, `pkg/plugin/rpc/enums.go`); and the `.ci` directive
types matched to the log backend leaf (`test/runner/record_parse_vocabulary.go`).

### Drift found and corrected on the way (R-1)
- `asn4` was declared `type boolean` while the product read four modes, and to
  admit `asn4 require` the boolean validator accepted `require`/`refuse` for
  EVERY boolean leaf. The model now carries one `capability-mode` typedef used
  by all four capability leaves, and a boolean leaf refuses `require`.
- The XFRM algorithm mapper installed `cbc(aes)` / `hmac(sha256)` for a name it
  did not know, so the kernel could carry a transform the peer never
  negotiated. It now refuses (`ErrNotSupported`), proven at the state builder.
- The doctor runner printed every PKI finding twice: the owner's registration
  had landed while the runner's own copy still ran.

## Work Not Done
| Item | Home |
|------|------|
| Where the standard family registrations live, so the 7 family rows can derive and `kernelcap.labeledFamilies` stops being a fail-open guard | `plan/spec-standard-families-register-outside-the-bgp-tag.md` (skeleton) |
| The 121 feature-owned `doctor-*` codes still declared in `internal/core/diagnostic/codes.go`; moving them breaks `ze explain` on a build without the feature's tag | `plan/spec-doctor-codes-move-to-their-owners.md` (skeleton) |

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

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
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

---

## Implementation Summary

### What Was Implemented
- The report fell from 230 rows to 25 findings plus 64 gated rows (`bin/le-close/le enumeration report`, 2026-09-15: YANG enumerations 8 findings and 64 gated, family names 7, plugin names 10). Every one of the 25 is named in Known Limitations.
- Every doctor check registers from its owner (`130c8c54d7`): `runDoctorChecks` (`internal/component/doctor/registry.go`) iterates `diagnostic.DoctorChecksForPhase`, and `RegisterDoctorCheck` (`internal/core/diagnostic/doctor_registry.go`) refuses a duplicate name.
- The RIB's family switch is deleted: `parseFamily` (`internal/component/bgp/plugins/rib/rib_nlri.go`) delegates to `family.LookupFamily` (AC-2).
- `IsReadOnlyVerb` (`internal/component/command/verbs.go`) answers `Verbs[tok] == RoleRead`; the `readOnlyVerbs` map is deleted (AC-3). Its one product caller is `internal/component/plugin/server/command.go`.
- 64 YANG rows whose Go table is the declaration carry an agreement test and the `enumeration: gated by TestX` marker (reading A, `fd28b8dc61`, `3df03d9ca9`). `readGatedMarker` (`internal/le/enumeration/enumeration.go`) refuses a marker naming a test its package does not declare; `findings` keeps a gated registry copy a finding; `deadMarkers` reports a marker that suppresses nothing.
- The three drifts found on the way are fixed and listed under "Drift found and corrected" above; `354412bcc7` adds the refusal of a capability mode word the vocabulary does not name (`parseCapMode`, `internal/component/bgp/reactor/config_capabilities.go`).

### Bugs Found/Fixed
- `asn4` declared boolean while read as four modes, and every boolean leaf accepted `require`: `a730d00401`, `test/parse/asn4-refuses-boolean-spelling.ci`.
- XFRM mapper defaulted an unknown algorithm to AES-CBC/HMAC-SHA256: `c23250a974`, `TestXfrmCipherVocabularyMatchesModel` and siblings in `internal/component/ike/dataplane/xfrm_vocabulary_test.go`.
- PKI doctor findings printed twice: `130c8c54d7`, `TestRunChecksCallsNoDoctorOwnedCheckTwice` (`internal/component/doctor/doctor_checks_test.go`).
- `config.Schema` handed the web form goyang's alphabetical enum order; the MCP form panicked on a nil schema: `c23250a974`.
- `parseCapMode` answered `enable` for a word it did not know: `354412bcc7`, `internal/component/bgp/reactor/config_capmode_test.go`.

### Documentation Updates
- `ai/patterns/registration.md` (doctor check registry, `130c8c54d7`), `docs/guide/health-checks.md` (`130c8c54d7`), `docs/architecture/config/syntax.md`, `docs/guide/configuration.md`, `docs/config-reference.md`, `docs/features/configuration.md` (`capability-mode`, `a730d00401`, `354412bcc7`), `ai/INDEX.md` (the gated marker keywords, `fd28b8dc61`).
- The gate's own report trailer (`Findings.Text`, `internal/le/enumeration/report.go`) carries the marker rule; no docs page copies it (`ai/rules/principles.md`, a rule does not copy what a command prints).

### Deviations from Plan
- Reading A of the Open decision was implemented inside this spec after the owner chose it (2026-09-14), so the third Work Not Done row of the body is done and was removed at closure.
- Closure added `test/plugin/rib-flowspec-family-name.ci`: the TDD plan's Functional Tests row had no `.ci` driving the RIB with the FlowSpec name, and the review gate found the gap (Findings fixed, #1).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1 assumed every row has a registry call that can replace it | 7 family rows need a registry that only registers under `ze_bgp`; `kernelcap.labeledFamilies` would become a fail-open guard | deriving in `kernelcap` rendered `afi-1/safi-128` | `plan/spec-standard-families-register-outside-the-bgp-tag.md` |
| approach | the implementation phase left the RIB FlowSpec `.ci` unwritten and the row unstatused | the entry point (`request bgp rib purge-stale <peer> ipv4/flow`) had only a unit test at the producer | closure review step 3 (functional coverage) | `test/plugin/rib-flowspec-family-name.ci`, RED observed with `parseFamily` refusing `ipv4/flow` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| End the 230 rows, deriving each from its registry | Done | `bin/le-close/le enumeration report`: 25 findings + 64 gated | the 25 are named in Known Limitations with a disposition each |
| FlowSpec spelling: RIB and registry agree | Done | `internal/component/bgp/plugins/rib/rib_nlri.go:parseFamily` | `TestParseFamilyFlowSpecSpelling`, `test/plugin/rib-flowspec-family-name.ci` |
| `readOnlyVerbs` classifies `resolve` as the registry does | Done | `internal/component/command/verbs.go:IsReadOnlyVerb` | `TestIsReadOnlyVerbTracksTheRegistry` |
| No row silenced by a marker instead of fixed | Done | the six fix-pass commits add zero `enumeration: exempt` lines | the `gated by` marker is honoured only where a named test proves the agreement |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | the 25 report rows checked one by one against Known Limitations (2026-09-15) | 7 family, 10 plugin-name, 8 another-namespace YANG rows |
| AC-2 | Done | `TestParseFamilyFlowSpecSpelling` (`internal/component/bgp/plugins/rib/rib_parsefamily_test.go`) reads `flowspec.IPv4FlowSpec.String()` and `parseFamily` | `.ci` at the entry point added at closure |
| AC-3 | Done | `TestIsReadOnlyVerbTracksTheRegistry` (`internal/component/command/verbs_test.go`) | `resolve` carries `RoleRead` in `Verbs` |
| AC-4 | Done | `git show <c> \| grep -c '^+.*enumeration: exempt'` is 0 for `130c8c54d7 c23250a974 a730d00401 fd28b8dc61 3df03d9ca9 354412bcc7` | the 30 markers in `b4fe90b943` belong to the closed gate spec's policy lists |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| per-row test at each fixed call site | Done | `Test*MatchTheYANGModel`, `*_vocabulary_test.go`, `Test*MatchTheModel` per the Known Limitations table; `doctor_test.go` per owner | `go test` over enumeration, command, rib, doctor, ike/dataplane, bgp/config: ok (2026-09-15) |
| the FlowSpec family name an operator types | Done | `test/plugin/rib-flowspec-family-name.ci` | PASS 2.8s; RED observed with `parseFamily` refusing `ipv4/flow` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| one file per report row | Done | the report's list, across `130c8c54d7`, `c23250a974`, `a730d00401`, `fd28b8dc61`, `3df03d9ca9` and the wave-1 commits ending `9e3298e3d6` |
| a row needing a registry becomes its own spec | Done | `plan/spec-standard-families-register-outside-the-bgp-tag.md`, `plan/spec-doctor-codes-move-to-their-owners.md` |

### Audit Summary
- **Total items:** 12
- **Done:** 12
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (reading A implemented here; recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The rows the report names are ended, not silenced | tooling gate | `bin/le-close/le enumeration report`: 25 findings, 64 gated, zero `exempt` markers added by the fix-pass commits |
| The FlowSpec SAFI spelling answers from one registry | functional test | `test/plugin/rib-flowspec-family-name.ci`: `purge-stale 127.0.0.1 ipv4/flow` answers `purged 0`, `ipv4/flowspec` is refused with `unknown family "ipv4/flowspec"`; RED under a `parseFamily` that refuses `ipv4/flow` |
| `resolve` is classified as the registry classifies it | unit test at the producer | `TestIsReadOnlyVerbTracksTheRegistry` walks `Verbs` and compares `IsReadOnlyVerb` |
| A drift between a YANG enumeration and its Go table cannot return | agreement tests | 64 `gated by` rows each name a test that loads the module and compares both ways; `readGatedMarker` refuses a marker naming a test the package does not declare (`TestGatedMarkerNamingAMissingTestIsAFinding`) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| The 7 family-name rows (`chaos/peer`, `chaos/scenario`, `kernelcap`, `test/fixture`, `test/runner`) | only four families register outside the `ze_bgp` tag, and `kernelcap.labeledFamilies` is a fail-open guard until they do | `plan/spec-standard-families-register-outside-the-bgp-tag.md` |
| The 121 feature-owned `doctor-*` codes in `internal/core/diagnostic/codes.go` | moving them breaks `ze explain` on a build without the feature's tag | `plan/spec-doctor-codes-move-to-their-owners.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/the-fix-pass-for-restated-registries-6f9665d6-e580-425f-aed1-7d814d81d774.md` (12 files, verdict=clean) |
| `review check` | clean: `review_gate: OK (1 code files, clean, hashes match)` |
| Rounds | 2 (round 1 over the committed diff found the missing `.ci`; round 2 over the `.ci` and the closure edits found nothing) |
| Reviewer lenses used | wiring + functional coverage, fail-closed guards (`parseCapMode`, `xfrmEncName`, `readGatedMarker`, `IsReadOnlyVerb`), removed-behavior (`readOnlyVerbs`, the RIB switch, five emptied doctor test files), style pass over the changed Go (no peer-reachable `panic`: the two `BUG:` panics in `doctor/registry.go` and `doctor_checks.go` fire at init or on a runner defect) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | the FlowSpec spelling fix had no functional test at a RIB entry point; the encode `.ci` files reach the update text parser, not `parseFamily` | `internal/component/bgp/plugins/rib/rib_nlri.go:parseFamily` | `test/plugin/rib-flowspec-family-name.ci`, PASS then RED with the producer broken |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/rib-flowspec-family-name.ci` | yes | `ls test/plugin/rib-flowspec-family-name.ci` |
| `test/parse/asn4-refuses-boolean-spelling.ci` | yes | `ls test/parse/asn4-refuses-boolean-spelling.ci` |
| `plan/spec-standard-families-register-outside-the-bgp-tag.md` | yes | `git grep -c spec-the-fix-pass-for-restated-registries plan/spec-standard-families-register-outside-the-bgp-tag.md` = 2 |
| `plan/spec-doctor-codes-move-to-their-owners.md` | yes | same grep, 2 hits |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | every remaining row is named in Known Limitations | report rows 2026-09-15: `chaos/peer` x2, `chaos/scenario`, `kernelcap`, `test/fixture`, `test/runner` x2 (7); `config/graph.go:sectionBGP`, `validate_sections.go` x2, `firewall/legacy_tables.go`, `firewall/protocol.go`, `support/modules.go`, `le/site/plugins.go`, `le/yang/glue/yangglue.go`, `le/yang/migration/commands.go` x2 (10); `ike/crypto/transform.go`, `config/graph.go:ifaceListKinds`, `iface/ra/doctor.go`, `flowexport/register.go`, `le/qemu/guest_linux.go`, `core/events/events.go`, `pkg/plugin/rpc/enums.go`, `test/runner/record_parse_vocabulary.go` (8) |
| AC-2 | one spelling | `go test ./internal/component/bgp/plugins/rib/` ok; `.ci` 656 PASS 2.8s |
| AC-3 | derived from `command.Verbs` | `go test ./internal/component/command/` ok; `grep -n 'Verbs\[tok\] == RoleRead' internal/component/command/verbs.go` |
| AC-4 | no exemption marker added | zero `+ enumeration: exempt` lines in the six fix-pass commits |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le enumeration report` | n/a (tooling): `bin/le-close/le enumeration report` exit 0, 25 findings | yes |
| `request bgp rib purge-stale <peer> ipv4/flow` | `test/plugin/rib-flowspec-family-name.ci` | yes, read and run |
| `ze config validate` with `asn4 true` | `test/parse/asn4-refuses-boolean-spelling.ci` | yes, read |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | 7 family rows need a registry that registers outside `ze_bgp`; homed in `plan/spec-standard-families-register-outside-the-bgp-tag.md` |
| A-2 | broken | three drifts changed what the surface answers (`asn4`, XFRM default, PKI double print); each is R-1's case and the corrected answer carries its test |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| #12 internal architecture: doctor checks register from their owner | `ai/patterns/registration.md` and `docs/guide/health-checks.md` edited in `130c8c54d7`; producer `runDoctorChecks` | yes |
| #2 config syntax: `capability-mode` typedef | `docs/architecture/config/syntax.md`, `docs/guide/configuration.md` edited in `a730d00401`/`354412bcc7`; producer `parseCapMode` | yes |
| #10 test infrastructure: the `gated by` marker | `ai/INDEX.md` keyword row (`fd28b8dc61`); the rule is printed by `Findings.Text` | yes |
| #1, #3-#9, #11, #13-#17: No | `git grep -l 'enumeration: gated' docs/` is empty and no command, RPC, wire format or RFC row changed | yes |

## Core Insight

The closed corpus cannot tell which side of a "must agree" row declares. This pass judged 72 rows and found the Go side is the declaration in 64: it carries the wire value, the IANA number, the kernel name or the handler, and the YANG enumeration is the copy. The repair for that shape is an agreement test in the owner, not a derivation, and the gate learns to see the test through a marker that is red where it names nothing.

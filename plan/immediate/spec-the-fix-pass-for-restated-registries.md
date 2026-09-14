# Spec: the fix pass for restated registries

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-14 |

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

Reading A is recommended: it is what the gate's own row text asks for ("the two
must agree") made checkable. Until the owner answers, the 64 rows are listed in
Known Limitations by test name.

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
| Where the standard family registrations live, so the 7 family rows can derive and `kernelcap.labeledFamilies` stops being a fail-open guard | `plan/next/spec-standard-families-register-outside-the-bgp-tag.md` (to write) |
| The 121 feature-owned `doctor-*` codes still declared in `internal/core/diagnostic/codes.go`; moving them breaks `ze explain` on a build without the feature's tag | `plan/next/spec-doctor-codes-move-to-their-owners.md` (to write) |
| The gate learning to see an agreement test (Open decision, reading A) | this spec, once the owner answers |

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

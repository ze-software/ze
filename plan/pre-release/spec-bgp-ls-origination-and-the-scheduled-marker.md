# Spec: bgp-ls-origination-and-the-scheduled-marker

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-08-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Three MUST-level requirements of RFC 9552 are gated, unproven, and unannotated,
so `./le rfc check` exits 2 on them and has done since the RFC was enrolled:

| Row | Level | Text |
|-----|-------|------|
| `RFC9552-5.2.1.1-1` | MUST NOT | The same node MUST NOT be represented by two keys (§5.2.1.1) |
| `RFC9552-5.2.1.1-2` | MUST NOT | Two different nodes MUST NOT be represented by the same key (§5.2.1.1) |
| `RFC9552-8.2.3-5` | MUST | An implementation MUST allow the operator to configure an 8-octet BGP-LS Instance-ID (§8.2.3) |

All three are obligations on a BGP-LS **Producer**, and ze originates no BGP-LS.
§5.2.1.1 names the key as the Node Descriptor sub-TLVs plus the operator-assigned
BGP-LS Instance-ID, and §8.2.3's MUST is the leaf that carries it, so the three
are one obligation seen from two sections: a producer must stamp a globally
unique key, and the operator must be able to configure the field that makes it
unique.

Thomas ruled on 2026-08-26 that ze IS to become a BGP-LS Producer, and that the
work is outside the current scope. So neither `{gap}` nor `{not-applicable}` is
true of these rows: ze intends to comply and does not yet, and no existing marker
says that.

The spec therefore has two halves, and the first is what lets the second wait:

1. A `{scheduled: <spec>; why}` marker that says a gated requirement is OWNED by
   a named, live spec. It clears the gate's "no test and no annotation" error,
   publishes as DEBT, and DIES WITH THE SPEC IT NAMES.
2. BGP-LS origination, which discharges the three rows for real.

Phase 1 lands on its own and Phase 2 waits. That is deliberate: the marker points
at THIS spec, this spec stays open while origination is unbuilt, and the marker
stays valid for exactly that long and not one commit longer.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - registration over hardcoding, which the
      producer's family registration must follow
  → Decision: components register and the core discovers them; a producer is a
    Mode change on an existing family registration, not a new core switch
  → Constraint: no per-feature field in a shared package

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9552.md` - the three rows and their 230-line neighbourhood
  → Constraint: §5.2.1.1 (A) and (B) are properties of a key an ORIGINATOR
    assigns; a decoder constructs no key and can neither satisfy nor violate them
  → Constraint: §5.2 "The network operator MUST assign the same BGP-LS
    Instance-IDs on all BGP-LS Producers within a given IGP domain", and
    "Instance-ID 0 is RECOMMENDED when there is only a single protocol instance"
- [ ] `ai/rules/rfc-compliance.md` - the annotation registers and the eight ratchets
  → Constraint: a marker MUST NOT be read, or written, as saying Ze owes less
  → Constraint: `{gap}` is not an escape from `check_coverage_ratchet`; a new
    marker must not become one either

**Key insights:**
- `{superseded}` is the precedent for a marker that must not lower what ze owes:
  it lives on its own `Requirement.superseded` field and `evaluate` never reads
  it, so a marked requirement stays gated, counted and ratcheted.
- `{scheduled}` cannot copy that exactly, because its whole purpose IS to change
  the verdict. It is a coverage annotation that clears the error and publishes as
  debt, which is what `{superseded}`'s `unextracted` and `unresolved` already do.
- Spec closure is `git rm plan/<spec>` (commit B, `plan/TEMPLATE.md`). That is the
  expiry mechanism: no timer, no review date, no allowlist.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/rfc.go` - `ANNOTATION_KINDS` is
      `{"not-applicable", "gap", "single-polarity"}`; `_ANNOTATION_RE` anchors
      one `{...}` group at end of line; `_strip_markers` loops so one line carries
      at most one coverage annotation and at most one `{superseded}`, in either
      order, and refuses a second of either; `_parse_annotation` refuses an
      unknown kind and refuses an empty reason; the checker's per-requirement loop
      clears the violation for `not-applicable` and `gap` via `continue`, and
      refuses either when a tagged test IS found.
- [ ] `internal/component/bgp/plugins/nlri/ls/plugin.go` - `main` registers
      `bgp-ls/bgp-ls` (AFI 16388, SAFI 71) and `bgp-ls/bgp-ls-vpn` (SAFI 72), both
      with `Mode: "decode"`. Nothing registers an encode mode.
- [ ] `internal/component/bgp/plugins/nlri/ls/types_nlri.go` - `NewBGPLSNode`
      stores its `id uint64` argument as `bgplsBase.identifier`, which
      `BGPLSNode.WriteTo` writes as the 8-octet Identifier field of the NLRI
      (`BGPLSNode.Len` accounts for it as `4 + 9 + 4 + LocalNode.Len()`).
      `NewBGPLSLink`, `NewBGPLSPrefixV4` and `NewBGPLSPrefixV6` take the same
      argument. None of the four has a caller outside `_test.go`, so no value is
      ever supplied by ze.
- [ ] `internal/component/bgp/plugins/nlri/ls/types_descriptor.go` -
      `LinkDescriptor.WriteTo` and `PrefixDescriptor.WriteTo` emit no TLV 263, so
      no Multi-Topology Identifier reaches the wire from ze.
- [ ] `internal/component/bgp/plugins/role/yang/ze-role.yang` - the shape a
      BGP plugin's YANG takes: a `grouping`, then three `augment` statements onto
      `/bgp:bgp/bgp:peer`, `/bgp:bgp/bgp:group/bgp:peer` and `/bgp:bgp/bgp:group`.

**Behavior to preserve:**
- Every existing `{gap}`, `{not-applicable}` and `{single-polarity}` annotation
  keeps its meaning and its effect. `{scheduled}` adds a kind; it changes none.
- BGP-LS decoding, and the `ze bgp decode` CLI path through `decodeBGPLSNLRI`.
- The counts `ai/RFC-REQUIREMENTS.md` publishes for every other RFC.

**Behavior to change:**
- `./le rfc check` stops erroring on a gated requirement that carries a valid
  `{scheduled}` marker, and starts erroring on one whose named spec is gone.
- Ze gains a BGP-LS Producer (Phase 2).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
Two, one per phase.

- Phase 1: a requirement line in `rfc/short/rfc9552.md`, read by
  `internal/le/rfc/rfc.go` when `./le rfc check` or
  `./le rfc index-update` runs.
- Phase 2: the operator's configuration, as a YANG leaf carrying the 8-octet
  BGP-LS Instance-ID, plus the IGP link-state database ze already holds in its
  IS-IS and OSPF plugins.

### Transformation Path
1. Phase 1: `_strip_markers` finds `{scheduled: plan/spec-<name>.md; why}` at end of
   line and hands the body to `_parse_annotation`.
2. Phase 1: `_parse_annotation` accepts the new kind, splits the reason on the
   first `;`, and records the spec path.
3. Phase 1: a new precondition check resolves that path against the repository and
   refuses the marker when the file does not exist.
4. Phase 1: the per-requirement loop clears the violation, as it does for `gap`,
   and the row is counted as DEBT rather than as covered.
5. Phase 1: `render_shards` and `_render_status_backlog` publish the debt with the
   owning spec named.
6. Phase 2: IGP LSDB → Node/Link/Prefix Descriptor construction → `NewBGPLSNode`
   and siblings with the configured Instance-ID as `bgplsBase.identifier` →
   `WriteTo` → MP_REACH_NLRI.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Gate ↔ spec tree | a filesystem existence check on a `plan/` path named in a summary | No |
| Engine ↔ Plugin | the ls plugin's family registration gains an encode mode | No |
| Config ↔ Plugin | the Instance-ID leaf reaches the ls plugin as JSON | No |

### Integration Points
- `internal/le/rfc/rfc.go` `ANNOTATION_KINDS` and `Annotation` - the new
  kind and its spec field.
- `internal/component/bgp/plugins/nlri/ls/plugin.go` `main` - the registration
  whose Mode changes.
- `internal/component/bgp/plugins/nlri/ls/yang/ze-bgp-ls.yang` - the Instance-ID <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
  leaf, augmenting the same three points `ze-role.yang` augments.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Spec closure removes the spec file from the tree | `plan/TEMPLATE.md` Closure checklist, "Commit B: `git rm plan/<spec>` only" | the marker never expires and becomes a permanent `{gap}` with better manners | a test that deletes the named spec and asserts the gate reds | unvalidated |
| A-2 | Ze's IS-IS and OSPF plugins hold enough link-state to originate node, link and prefix NLRI | the plugins exist under `internal/plugins/isis` and `internal/plugins/ospf`; their LSDB contents are NOT yet read | Phase 2 needs an LSDB export surface before it can start, which is a larger spec | reading the two plugins' LSDB types during Phase 2 research | unvalidated |
| A-3 | No existing summary line ends in a `{scheduled: ...}` that means something else | `_parse_annotation` refuses an unknown kind today, so no such line can parse | none; the assumption is enforced by the current parser | `grep -c '{scheduled' rfc/short/` returns 0 before Phase 1 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `{scheduled}` becomes the cheap route from red to green, replacing `{gap}` everywhere | the count of scheduled rows rises while no spec closes | the marker requires a LIVE spec file and dies with it, which `{gap}` does not; the published ledger names the owning spec per row, so a spec owning many rows is visible |
| R-2 | A spec is closed with its rows still unproven, and the gate reds on somebody else's unrelated commit | a closure commit that removes a spec named by a marker | Phase 1 adds the reverse check: a spec named by any `{scheduled}` cannot be removed while a row still names it |
| R-3 | Phase 1 lands and Phase 2 never runs, so the marker is load-bearing forever | the spec's Updated field ages with no phase-2 commits | that is the honest state and the ledger publishes it as debt with this spec named; it is strictly better than the silent red it replaces |
| R-4 | Originating BGP-LS with a wrong Instance-ID merges two IGP domains in every Consumer downstream | a Consumer reports one node where two exist | §5.2's guidance is explicit and the leaf defaults to nothing rather than to 0; Phase 2 refuses to originate without a configured Instance-ID |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Phase 1: the RFC gate mis-reports coverage for every enrolled RFC, which is a claim about compliance rather than a runtime failure. Phase 2: ze originates link-state into a BGP-LS mesh, and a wrong key corrupts every Consumer's topology, which is the worst outcome in this spec |
| How is it reverted? | Phase 1 is a single commit revert; no data outlives it. Phase 2 is not revertible once peers have seen the NLRI: withdrawal is itself an origination act |
| Who else touches this path? | `internal/le/rfc/rfc.go` is edited by every RFC enrolment; the ls plugin is touched by `plan/immediate/spec-bgp-ls-receiver-fault-management.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a `{scheduled: plan/spec-<name>.md; why}` on a gated requirement line | → | `_parse_annotation` | `test_scheduled_marker_parses_and_names_its_spec` |
| `./le rfc check` over a summary carrying a valid marker | → | the per-requirement loop | `test_scheduled_marker_clears_the_unproven_error` |
| `./le rfc check` over a marker whose spec file is absent | → | the precondition check | `test_scheduled_marker_refuses_an_absent_spec` |
| `./le rfc index-update` | → | `render_shards` | `test_scheduled_row_publishes_as_debt_naming_its_spec` |
| operator configures a BGP-LS Instance-ID | → | the ls plugin's config parse | `TestBGPLSInstanceIDReachesTheOriginator` |
| operator runs the RFC gate over a scheduled row | → | `./le rfc check` | `test/plugin/rfc-scheduled-marker.ci` | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| operator configures origination and watches the wire | → | `originate.go` → `WriteTo` | `test/decode/bgp-ls-originate.ci` | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a gated requirement carries `{scheduled: plan/spec-<name>.md; why}` and `plan/spec-<name>.md` exists | `./le rfc check` reports no violation for that row |
| AC-2 | the same marker, and `plan/spec-<name>.md` does NOT exist | the check errors, naming the row and the missing spec |
| AC-3 | a `{scheduled}` marker with no reason after the `;` | the check errors, as `_parse_annotation` already does for every bare annotation |
| AC-4 | a row carrying `{scheduled}` AND a tagged test | the check errors that the marker is stale, as it does for `{gap}` |
| AC-5 | `./le rfc index-update` over a scheduled row | `ai/RFC-REQUIREMENTS.md` shows it as DEBT with the owning spec named, never as covered |
| AC-6 | a row that had a tagged test at HEAD now carries `{scheduled}` | `check_coverage_ratchet` fires; the marker is not an escape |
| AC-7 | a commit removes a spec while a `{scheduled}` row still names it | the check errors on the removal, before the row goes silently stale |
| AC-8 | `docs/features/rfc-status.md` Remaining prose | its spelled count agrees with the real scheduled count as well as the gap count |
| AC-9 | the three RFC 9552 rows | each carries `{scheduled}` naming this spec, and `./le rfc check` exits 0 on rfc9552 |
| AC-10 | ze configured with a BGP-LS Instance-ID and an IGP topology | it originates node, link and prefix NLRI carrying that Instance-ID in the 8-octet Identifier field |
| AC-11 | ze configured to originate BGP-LS with NO Instance-ID | it refuses to originate and says which leaf is missing |
| AC-12 | two nodes in one IGP domain | each is originated under exactly one key, and no two under the same key |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | reads `ai/RFC-REQUIREMENTS.md` to see what ze owes | summary line → `_parse_annotation` → `render_shards` → published row | `test_scheduled_row_publishes_as_debt_naming_its_spec` |
| 2 | closes a spec that still owns a scheduled row | closure commit → `commit_helper.py` → the reverse check | `test_closing_a_spec_that_owns_a_scheduled_row_is_refused` |
| 3 | configures a BGP-LS Instance-ID and peers with a Consumer-facing RR | config → ls plugin → `NewBGPLSNode` → `WriteTo` → MP_REACH_NLRI | `bgp-ls-originate-gobgp` interop scenario |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_scheduled_marker_parses_and_names_its_spec` | `internal/le/` | AC-1 parse half | |
| `test_scheduled_marker_clears_the_unproven_error` | `internal/le/` | AC-1 verdict half | |
| `test_scheduled_marker_refuses_an_absent_spec` | `internal/le/` | AC-2 | |
| `test_scheduled_marker_refuses_an_empty_reason` | `internal/le/` | AC-3 | |
| `test_scheduled_marker_is_stale_when_a_test_exists` | `internal/le/` | AC-4 | |
| `test_scheduled_row_publishes_as_debt_naming_its_spec` | `internal/le/` | AC-5 | |
| `test_scheduled_does_not_escape_the_coverage_ratchet` | `internal/le/` | AC-6 | |
| `test_closing_a_spec_that_owns_a_scheduled_row_is_refused` | `internal/le/` | AC-7 | |
| `test_status_remaining_count_covers_scheduled_rows` | `internal/le/` | AC-8 | |
| `TestBGPLSInstanceIDReachesTheOriginator` | `internal/component/bgp/plugins/nlri/ls/config_test.go` | AC-10 wiring | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestBGPLSOriginationRefusesWithoutAnInstanceID` | `internal/component/bgp/plugins/nlri/ls/originate_test.go` | AC-11 | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestBGPLSNodeKeyIsUniquePerNode` | `internal/component/bgp/plugins/nlri/ls/originate_test.go` | AC-12 | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| BGP-LS Instance-ID | 0 - 18446744073709551615 (8 octets) | 18446744073709551615 | N/A | 18446744073709551616 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc-scheduled-marker` | `test/plugin/rfc-scheduled-marker.ci` | an operator runs the RFC gate over a summary carrying a scheduled row and sees debt, not a violation | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `bgp-ls-originate` | `test/decode/bgp-ls-originate.ci` | an operator configures an Instance-ID and sees ze advertise link-state NLRI carrying it | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-ls-originate-gobgp` | `test/interop/scenarios/` | GoBGP | a real peer accepts ze-originated Link-State NLRI and reports one node per key | |

## Files to Modify
- `internal/le/rfc/rfc.go` - the new annotation kind, its spec-path
  precondition, the ratchet interaction, and the published debt row
- `internal/le/` - the nine unit tests above
- `rfc/short/rfc9552.md` - the three rows gain `{scheduled}` naming this spec
- `ai/rules/rfc-compliance.md` - the marker's register, its preconditions, and the
  rule that it never says ze owes less
- `docs/features/rfc-status.md` - the Remaining prose covers scheduled rows
- `internal/component/bgp/plugins/nlri/ls/plugin.go` - the encode registration
- `internal/component/bgp/plugins/nlri/ls/types_descriptor.go` - descriptor emission
- `docs/architecture/wire/nlri-bgpls.md` - declared by the `// Design:` header of
  every ls file this spec changes. Today it describes a decoder; Phase 2 makes ze
  write the format it documents, so the originated NLRI, the Identifier field and
  the descriptor TLVs ze emits all land here

## Files to Create
- `ai/rules/points/rfc-compliance/rfc-summaries-rfc-short/a-scheduled-requirement-names-the-live-spec-that-owns-it.md` - the rule point <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
- `internal/component/bgp/plugins/nlri/ls/originate.go` - the producer <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
- `internal/component/bgp/plugins/nlri/ls/yang/ze-bgp-ls.yang` - the Instance-ID leaf <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
- `test/plugin/rfc-scheduled-marker.ci` - functional test for Phase 1 <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
- `test/decode/bgp-ls-originate.ci` - functional test for Phase 2 <!-- doc-links: ignore (file this open spec plans and has not created yet) -->

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/plugins/nlri/ls/yang/ze-bgp-ls.yang` for the Instance-ID leaf | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| YANG validation constraints | Yes | `uint64`, the full 8-octet range, no default: §5.2 recommends 0 only for a single-instance network, and a default would silently pick that for a multi-instance one |
| YANG custom validators | No | the native `uint64` range is the whole constraint |
| CLI commands/flags | No | configuration only; no new verb |
| CLI grammar (keyword before value) | N-A | no new command |
| Editor autocomplete | Yes | automatic for a typed YANG leaf |
| Functional test for new RPC/API | Yes | `test/decode/bgp-ls-originate.ci` | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| Pipe completeness | N-A | no new command output |
| Env var registration | No | the leaf is per-instance config, not an environment default |
| Doctor check for runtime dependencies | No | no new file, socket, port, module or binary |
| Prometheus counters/metrics | Yes | originated NLRI count per family, defined in Phase 2 |
| BGP family surface (new SAFI / capability / attribute) | Yes | the 12-section checklist in `ai/patterns/bgp-family.md`, answered there in Phase 2: the families exist and only the Mode changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` for BGP-LS origination |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` for the Instance-ID leaf |
| 3 | CLI command added/changed? | No | no new verb |
| 4 | API/RPC added/changed? | No | no new RPC |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, the ls plugin gains an encode mode |
| 6 | Has a user guide page? | Yes | `docs/guide/bgp-ls.md` | <!-- doc-links: ignore (page this open spec plans and has not written yet) -->
| 7 | Wire format changed? | Yes | `docs/architecture/wire/nlri-bgpls.md`, which every changed ls file declares in its `// Design:` header |
| 8 | Plugin SDK/protocol changed? | No | the SDK already carries encode registration |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9552.md` and the RFC 9552 row of `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` for the new scenario |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`, ze moves from BGP-LS decoder to producer |
| 12 | Internal architecture changed? | Yes | the ls subsystem doc; `docs/architecture/core-design.md` is unaffected |
| 13 | Route metadata keys added/changed? | No | no new metadata key |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md`, `docs/features/plugins.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | DERIVED | run `./le spec citation anchors spec plan/pre-release/spec-bgp-ls-origination-and-the-scheduled-marker.md` at implementation time |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | verify every BGP-LS example against the new YANG |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the marker exists and is reachable
   - Tests: `test_scheduled_marker_parses_and_names_its_spec`, `test_scheduled_marker_clears_the_unproven_error`
   - Files: `internal/le/rfc/rfc.go` (`ANNOTATION_KINDS`, `Annotation`, `_parse_annotation`), `internal/le/`
   - Verify: the parser accepts the kind and the checker's verdict changes. The precondition is still a stub, so the absent-spec test fails
2. **Phase: The precondition and its reverse** -- the marker dies with its spec
   - Tests: `test_scheduled_marker_refuses_an_absent_spec`, `test_scheduled_marker_refuses_an_empty_reason`, `test_scheduled_marker_is_stale_when_a_test_exists`, `test_closing_a_spec_that_owns_a_scheduled_row_is_refused`
   - Files: `internal/le/rfc/rfc.go`
   - Verify: deleting the named spec reds the gate; removing a spec that owns a row is refused
3. **Phase: Publication** -- the debt is visible, and counted
   - Tests: `test_scheduled_row_publishes_as_debt_naming_its_spec`, `test_status_remaining_count_covers_scheduled_rows`, `test_scheduled_does_not_escape_the_coverage_ratchet`
   - Files: `internal/le/rfc/rfc.go`, `docs/features/rfc-status.md`
   - Verify: `./le rfc index-update` names the owning spec on each scheduled row
4. **Phase: Adopt the three rows** -- Phase 1 closes here and MAY be committed alone
   - Tests: `rfc-scheduled-marker`, and `./le rfc check` exits 0 on rfc9552 (AC-9)
   - Files: `rfc/short/rfc9552.md`, `ai/rules/rfc-compliance.md`, the new rule point, `test/plugin/rfc-scheduled-marker.ci` <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
   - Verify: the three rows carry `{scheduled}` naming this spec, and the gate is green on them
5. **Phase: The Instance-ID leaf** -- config before origination
   - Tests: `TestBGPLSInstanceIDReachesTheOriginator`, `TestBGPLSOriginationRefusesWithoutAnInstanceID`
   - Files: `internal/component/bgp/plugins/nlri/ls/yang/ze-bgp-ls.yang`, the plugin's config parse <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
   - Verify: the leaf reaches the plugin, and origination without it is refused
6. **Phase: Origination** -- descriptors, keys, and the wire
   - Tests: `TestBGPLSNodeKeyIsUniquePerNode`, `bgp-ls-originate`, `bgp-ls-originate-gobgp`
   - Files: `internal/component/bgp/plugins/nlri/ls/originate.go`, `types_descriptor.go`, `plugin.go` <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
   - Verify: GoBGP accepts the NLRI and reports one node per key

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | the marker's precondition reads the WORKING TREE, so a spec added and a spec removed both take effect in the commit that does it, not the one after |
| Correctness | `{scheduled}` clears the unproven error and NOTHING else: level, gated, section, text and every ratchet verdict are byte-identical with the marker stripped |
| Naming | the marker's reason states why the work waits, never why ze owes less |
| Data flow | the spec-path check resolves against the repository root, never against the summary's directory |
| Rule: `ai/rules/rfc-compliance.md` | the new rule point says a `{scheduled}` row stays gated, stays counted in `ai/RFC-REQUIREMENTS.md`, and stays judged by every ratchet |
| Rule: `ai/rules/evidence.md` | the marker's reason names the producing function that does not yet exist, or names none rather than gesturing |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| the marker parses and clears the error | `./le rfc check` exits 0 with the three rows marked |
| the marker dies with its spec | delete the spec file, re-run the check, expect exit 2 naming all three rows |
| the debt is published | `grep -c 'scheduled' ai/RFC-REQUIREMENTS.md` is 3 and each names this spec |
| ze originates BGP-LS | `./le integration interop` with `bgp-ls-originate-gobgp` PASS |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the spec path in a marker is read from a tracked summary and used only for an existence test; it must never be opened, executed, or joined outside the repository root |
| Authorization failing open | the precondition must fail CLOSED: an unreadable `plan/` directory reds the gate rather than accepting every marker |
| Resource exhaustion | Phase 2 originates one NLRI per IGP object; the advertisement rate is what §8.2.3 SHOULDs a limit on, and it is out of this spec's scope |

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

- The expiry mechanism was the hard part, and the answer was already in the repo.
  Every candidate that carried its own clock -- a review date, a deadline field, a
  quarterly sweep -- is a second record of a fact, and the RFC gate's own history
  is a list of second records going stale. Spec closure already removes the spec
  file, so naming the file IS the clock, and it cannot drift from the plan because
  it is the plan.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `{scheduled}` names a spec PATH, checked for existence | a spec STEM, a free-text owner, a date | a path is testable and a stem is a guess about layout; an owner and a date are both claims nothing re-checks, which is the failure mode `ai/rules/evidence.md` names |
| The marker is a coverage annotation, not a `{superseded}`-style separate field | its own `Requirement.scheduled` field, mirroring `{superseded}` | `{superseded}` deliberately never reaches `evaluate` because it must not change the verdict; `{scheduled}` exists TO change the verdict, so the same shape would be wrong. It publishes as debt instead, which is what `{superseded}`'s own `unextracted` and `unresolved` dispositions already do |
| The three rows go in one spec with the mechanism | a tooling spec plus a protocol spec | Thomas asked for it on 2026-08-26, and it holds together: the marker's whole value is that it points at a live plan, and this spec IS that plan. Phase 1 is separately committable, so the mechanism does not wait for origination |
| Phase 2 refuses to originate with no Instance-ID | default the leaf to 0 | §5.2 RECOMMENDS 0 only when there is a single protocol instance in the network. Defaulting picks that for a multi-instance network silently, and the failure surfaces as a merged topology in a Consumer, one hop removed from any log ze writes |

## Known Limitations

- Phase 2 has no schedule. That is the state Thomas chose on 2026-08-26, and the
  `{scheduled}` marker exists to publish it rather than hide it.
- The marker does not distinguish "scheduled next" from "scheduled eventually".
  A priority field would be a second record with nothing re-checking it, which is
  the thing this design refuses.
- `RFC9552-8.2.6-2` and `RFC9552-8.2.2-9` are NOT here. They are receiver-side and
  belong to `plan/immediate/spec-bgp-ls-receiver-fault-management.md`.

## RFC Documentation (Scope: protocol)

Add `// RFC 9552 Section X.Y: "<quoted requirement>"` above enforcing code.
Phase 2 owes one above the key construction (§5.2.1.1 (A) and (B)) and one above
the Instance-ID's use in the Identifier field (§5.2).

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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

## Work Inherited From a Deferral Row

<!-- The deferral directory was deleted on 2026-09-05. A row that named this spec as
     its destination is reproduced here, so the item and the reasoning behind it
     survive the directory. Each row is outstanding work this spec owns. -->

### From `bgp-ls-receiver-fault-management.md`, 2026-08-26

Deferred by spec-bgp-ls-receiver-fault-management.

RFC 9552 §8.2.3's producer-side SHOULDs: the advertisement rate limit, the abstracted-topology controls, and the 4096-byte BGP-LS UPDATE size limit.

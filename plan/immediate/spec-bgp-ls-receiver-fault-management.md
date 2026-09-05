# Spec: bgp-ls-receiver-fault-management

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

Ze receives BGP-LS and performs none of the syntactic validation RFC 9552
Section 8.2.2 requires of a receiver. `validateMPNLRISyntax`
(`internal/component/bgp/message/rfc7606.go`) returns `nil` unless the AFI is
IPv4 or IPv6 AND the SAFI is unicast or multicast, so AFI 16388 falls straight
through to no validation at all. A Link-State NLRI whose TLV lengths do not sum
to its Total NLRI Length is accepted and propagated.

Four rows of `rfc/short/rfc9552.md` name that hole, and this spec closes all four:

| Row | Level | State today |
|-----|-------|-------------|
| `RFC9552-8.2.2-9` | MUST | no test, no annotation. The gate errors on it |
| `RFC9552-8.2.2-5` | MUST | `{gap}` citing `validateMPNLRISyntax` returning nil for AFI 16388 |
| `RFC9552-8.2.2-6` | MUST | skipable malformed BGP-LS Attribute must be Attribute Discard |
| `RFC9552-8.2.6-2` | MUST | no test, no annotation. Ze must let an operator drop all updates from a Consumer-facing peer |

The first three are one mechanism: validate the NLRI, then take the action
Section 8.2.2 prescribes for the class of error found. The fourth is a separate
mechanism that shares this spec because it is the other half of what a BGP-LS
receiver owes, and because Thomas chose its shape on 2026-08-26.

The attribute side is already done and is the model to follow. `validateBGPLSAttr`
(`internal/component/bgp/message/rfc7606_bgpls.go`) registers itself in
`attrValidators` for code 29 and walks the BGP-LS Attribute's TLVs, returning
`RFC7606ActionAttributeDiscard` on a length that does not fit. This spec is that
work one layer over, on the NLRI.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/nlri-bgpls.md` - declared by the `// Design:` header
      of `rfc7606_bgpls.go` and every ls file
  → Constraint: the BGP-LS TLV header is 4 octets and is NOT padded to 4-octet
    alignment, so a TLV occupies exactly 4 + Length octets

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9552.md` - Section 8.2.2's two validation lists and its action
      ladder, and Section 8.2.6's two sentences
  → Constraint: "A Link-State NLRI MUST NOT be considered malformed or invalid
    based on the inclusion/exclusion of TLVs or contents of the TLV fields (i.e.,
    semantic errors)". Only lengths and ordering are judged
  → Constraint: the action depends on the error class. Skipable (e.g. a TLV
    ordering violation) is 'NLRI discard'. Unprocessable (e.g. a length-encoding
    error) is 'AFI/SAFI disable' when another AFI/SAFI shares the session, and
    'session reset' when the session is BGP-LS only or disable is not possible
  → Constraint: §10 grounds §8.2.6 in operator knowledge: "Generally, an operator
    is aware of the BGP-LS Speaker's role and link-state peerings"

**Key insights:**
- §8.2.2's seven NLRI bullets are: MP_REACH TLV lengths sum to the attribute
  length; the same for MP_UNREACH; each NLRI's TLV lengths sum to its Total NLRI
  Length; TLV and recognized sub-TLV lengths are valid; RFC 7606 field
  correctness; the §5.1 ascending-Type ordering rule; and no repeated sub-TLV
  inside a Local or Remote Node Descriptor.
- Ze's action enum has no 'NLRI discard' and no 'AFI/SAFI disable' member:
  `RFC7606Action` is `None`, `AttributeDiscard`, `TreatAsWithdraw`,
  `SessionReset`. `enforceRFC7606` does implement an RFC 7606 §5.4 typed-NLRI
  discard, so the mechanism may exist without an enum value. Which of the two it
  is decides Phase 2's shape and is the first thing to establish.
- Nothing on the wire says a peer only serves BGP-LS Consumers. There is no
  capability, no attribute, no field. §1.1 also says the roles are not mutually
  exclusive and mix per-speaker, so the role must be DECLARED, never inferred.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/message/rfc7606.go` - `validateMPNLRISyntax(code,
      afi, safi, nlri, addPath)` returns `nil` immediately unless the AFI is IPv4
      or IPv6 and the SAFI is unicast or multicast; otherwise it delegates to
      `ValidateNLRISyntaxAddPath`. AFI 16388 matches neither test, so BGP-LS is
      unvalidated. `RFC7606Action` has four members: `None`,
      `AttributeDiscard`, `TreatAsWithdraw`, `SessionReset`.
- [ ] `internal/component/bgp/message/rfc7606_bgpls.go` - `init` registers
      `validateBGPLSAttr` in `attrValidators` under `attrCodeBGPLS = 29`. The
      validator walks TLVs bounded by the attribute length, advances by at least
      `bgplsTLVHeaderLen = 4` each iteration so a peer cannot stall it, reads no
      TLV type and no value byte, and returns `RFC7606ActionAttributeDiscard`
      with `DiscardReasonInvalidLength` on a TLV that overruns.
- [ ] `internal/component/bgp/reactor/session_validation.go` -
      `Session.enforceRFC7606` is the receive-path entry point, called from
      `processMessage` BEFORE callback dispatch so a malformed UPDATE never
      reaches a plugin as a valid route. It already reads the negotiated ADD-PATH
      set per family through `bgpctx.Registry.Get(s.recvCtxID)`, and already
      carries an RFC 7606 §5.4 typed-NLRI discard branch that rebuilds the body.
- [ ] `internal/component/bgp/plugins/nlri/ls/plugin.go` - both BGP-LS families
      register `Mode: "decode"`; `errNoValidBgpLsNlrisDecoded` reaches CLI output,
      never the session's fault-management ladder.
- [ ] `internal/component/bgp/plugins/filter_family/config.go` -
      `parseFamilyFilters` reads `bgp/policy/family-filter`, requires a `family`
      that `family.LookupFamily` resolves, and an `action` of `remove` or
      `tear-down`, with tear-down refused in any export chain.
- [ ] `internal/core/family/family.go` - `AFIBGPLS = 16388`, and `bgp-ls` and
      `bgp-ls-vpn` are both nameable families.
- [ ] `internal/component/bgp/plugins/role/yang/ze-role.yang` - the shape a
      per-peer declared role takes: a `grouping`, then `augment` onto
      `/bgp:bgp/bgp:peer`, `/bgp:bgp/bgp:group/bgp:peer` and `/bgp:bgp/bgp:group`.

**Behavior to preserve:**
- Every non-BGP-LS family's RFC 7606 verdict, byte for byte. `validateMPNLRISyntax`
  gains a branch; its existing branches do not move.
- `validateBGPLSAttr` and the attribute-side verdict.
- An operator's hand-written `family-filter` on `bgp-ls` keeps working.

**Behavior to change:**
- A malformed Link-State NLRI is detected and acted on instead of propagated.
- A peer declared Consumer-facing has BGP-LS dropped on import without the
  operator writing a filter.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
Wire bytes: an UPDATE carrying MP_REACH_NLRI or MP_UNREACH_NLRI with AFI 16388,
arriving on an established session. Second entry point: the operator's
configuration, declaring a peer's BGP-LS role.

### Transformation Path
1. `Session.enforceRFC7606` runs on the received `wireu.WireUpdate`, before
   callback dispatch.
2. The MP attribute walk reaches `validateMPNLRISyntax` with AFI 16388.
3. A new BGP-LS branch walks the Link-State NLRI: Total NLRI Length against the
   summed TLV lengths, each TLV's length against its container, the §5.1
   ascending-Type ordering, and one instance per Node Descriptor sub-TLV.
4. The verdict classifies the error as skipable or unprocessable, and
   `enforceRFC7606` takes the corresponding action.
5. Separately, at config-apply: a peer's declared BGP-LS role appends an import
   drop for the BGP-LS families, through the same machinery `filter_family` uses.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire ↔ message | the NLRI bytes are walked in `internal/component/bgp/message` with no plugin involvement | No |
| Message ↔ reactor | the verdict is an `RFC7606ValidationResult` the session acts on | No |
| Config ↔ reactor | the declared role becomes an import filter at config-apply | No |

### Integration Points
- `internal/component/bgp/message/rfc7606.go` `validateMPNLRISyntax` - the branch
  that stops falling through for AFI 16388.
- `internal/component/bgp/reactor/session_validation.go` `Session.enforceRFC7606`
  - the action ladder that acts on the verdict.
- `internal/component/bgp/plugins/filter_family` - the import-drop mechanism the
  declared role reuses rather than reimplements.

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
| A-1 | `enforceRFC7606`'s RFC 7606 §5.4 typed-NLRI discard can express §8.2.2's 'NLRI discard' | the branch exists and rebuilds the body, per the function's own comment | Phase 2 must add an action member and every switch on `RFC7606Action` grows a case | reading the §5.4 branch whole in Phase 1 | unvalidated |
| A-2 | 'AFI/SAFI disable' has no existing mechanism in ze | `RFC7606Action` has four members and none is it | Phase 3 is larger than planned: disabling one family mid-session touches capability state | grep for a per-family disable in the reactor during Phase 1 | unvalidated |
| A-3 | Appending an import filter at config-apply is expressible without a new core switch | `filter_family` already parses and applies per-peer import filters | the declared role needs its own apply path, which risks the coupling `ai/rules/plugins.md` forbids | reading `filter_family/handler.go` in Phase 4 | unvalidated |
| A-4 | The §5.1 TLV ordering rule is checkable without recognizing a TLV | §8.2.2 lists ordering among SYNTACTIC checks and forbids semantic ones | the ordering bullet cannot be implemented without violating the section's own opening constraint, and needs its own annotation | reading §5.1's ordering sentence against §8.2.2's opening in Phase 1 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The new validator session-resets on a valid UPDATE some other implementation sends | an interop scenario tears the session where it used to establish | the interop test runs against a real peer originating real BGP-LS, and the unit tests fixture both a valid and a malformed NLRI of each shape |
| R-2 | The validator reads a TLV's value and becomes a semantic check, which §8.2.2 forbids | a test asserts on a TLV type or a field's contents | the attribute-side precedent reads neither; the review checklist makes it an explicit row |
| R-3 | A peer stalls the walk with a zero-length TLV | a receive path that does not terminate | every iteration advances by at least the 4-octet header, as `validateBGPLSAttr` does |
| R-4 | The declared role drops BGP-LS from a peer that was propagating it, and a topology silently empties | an operator reports a Consumer seeing nothing after a config change | the role is declared per peer and defaults to unset, so no existing config changes behaviour; the functional test covers the omitted case as well as the declared one |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A false positive resets live BGP sessions, which is the loudest failure in the daemon. A false negative leaves today's behaviour, which is propagating malformed NLRI |
| How is it reverted? | single commit revert; nothing outlives it, and no config migration is needed because the role leaf defaults to unset |
| Who else touches this path? | `rfc7606.go` is shared by every family's validation; `plan/pre-release/spec-bgp-ls-origination-and-the-scheduled-marker.md` touches the same ls plugin from the encode side |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| an UPDATE with AFI 16388 and a malformed Link-State NLRI | → | `validateMPNLRISyntax` | `TestBGPLSNLRIWithOverrunningTLVIsMalformed` |
| the same UPDATE on an established session | → | `Session.enforceRFC7606` | `TestEnforceRFC7606ActsOnAMalformedBGPLSNLRI` |
| a valid Link-State NLRI | → | `Session.enforceRFC7606` | `TestValidBGPLSNLRIReachesThePlugins` |
| a peer declared Consumer-facing | → | the config-apply filter append | `TestConsumerFacingPeerDropsBGPLSOnImport` |
| an operator runs a session against a peer sending malformed BGP-LS | → | the whole receive path | `test/decode/bgp-ls-malformed-nlri.ci` <!-- doc-links: ignore (planned test this open spec has not built yet) --> |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a Link-State NLRI whose TLV lengths do not sum to its Total NLRI Length | it is malformed; the UPDATE is not propagated as valid |
| AC-2 | MP_REACH_NLRI whose Link-State TLV lengths do not sum to the attribute length | malformed, per §8.2.2 bullet 1 |
| AC-3 | MP_UNREACH_NLRI, the same | malformed, per §8.2.2 bullet 2 |
| AC-4 | a TLV whose declared length overruns its container | malformed, per §8.2.2 bullet 4 |
| AC-5 | Link-State NLRI TLVs not in ascending Type order | malformed and handled as NLRI discard, which §8.2.2 names as its example of a skipable error |
| AC-6 | a Local or Remote Node Descriptor carrying the same sub-TLV twice | malformed, per §8.2.2 bullet 7 |
| AC-7 | a length-encoding error on a session carrying BGP-LS AND another AFI/SAFI | AFI/SAFI disable, per §8.2.2 |
| AC-8 | the same error on a session carrying BGP-LS only | session reset, per §8.2.2 |
| AC-9 | a Link-State NLRI that is syntactically valid but semantically odd: an unexpected TLV, a missing optional TLV | NOT malformed. §8.2.2 forbids judging it |
| AC-10 | a valid Link-State NLRI | it reaches the plugins unchanged, and the session stays established |
| AC-11 | a peer declared Consumer-facing in config | every BGP-LS UPDATE from it is dropped on import, with no operator-written filter |
| AC-12 | a peer with no declared BGP-LS role | BGP-LS is accepted and propagated, exactly as today |
| AC-13 | a peer declared Consumer-facing | its non-BGP-LS families are unaffected |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | peers with a BGP-LS speaker that sends a truncated NLRI | wire → `enforceRFC7606` → `validateMPNLRISyntax` → action ladder | `test/decode/bgp-ls-malformed-nlri.ci` <!-- doc-links: ignore (planned test this open spec has not built yet) --> |
| 2 | peers with a healthy BGP-LS speaker | wire → `enforceRFC7606` → plugins → RIB | `bgp-ls-receive-gobgp` interop scenario |
| 3 | declares a peer Consumer-facing and reloads | config → config-apply → import filter → no BGP-LS in the RIB | `test/decode/bgp-ls-consumer-facing.ci` <!-- doc-links: ignore (planned test this open spec has not built yet) --> |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBGPLSNLRIWithOverrunningTLVIsMalformed` | `internal/component/bgp/message/rfc7606_bgpls_nlri_test.go` | AC-1, AC-4 | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestBGPLSMPReachTLVLengthsMustSumToTheAttribute` | `internal/component/bgp/message/rfc7606_bgpls_nlri_test.go` | AC-2 | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestBGPLSMPUnreachTLVLengthsMustSumToTheAttribute` | `internal/component/bgp/message/rfc7606_bgpls_nlri_test.go` | AC-3 | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestBGPLSNLRITLVsOutOfOrderAreDiscarded` | `internal/component/bgp/message/rfc7606_bgpls_nlri_test.go` | AC-5 | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestBGPLSNodeDescriptorRefusesARepeatedSubTLV` | `internal/component/bgp/message/rfc7606_bgpls_nlri_test.go` | AC-6 | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestBGPLSSemanticOddityIsNotMalformed` | `internal/component/bgp/message/rfc7606_bgpls_nlri_test.go` | AC-9 | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestEnforceRFC7606ActsOnAMalformedBGPLSNLRI` | `internal/component/bgp/reactor/session_validation_bgpls_test.go` | AC-7, AC-8 | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestValidBGPLSNLRIReachesThePlugins` | `internal/component/bgp/reactor/session_validation_bgpls_test.go` | AC-10 | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestConsumerFacingPeerDropsBGPLSOnImport` | `internal/component/bgp/plugins/nlri/ls/role_test.go` | AC-11 | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestUndeclaredPeerStillPropagatesBGPLS` | `internal/component/bgp/plugins/nlri/ls/role_test.go` | AC-12 | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `TestConsumerFacingPeerKeepsItsOtherFamilies` | `internal/component/bgp/plugins/nlri/ls/role_test.go` | AC-13 | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| BGP-LS TLV Length | 0 - 65535 | the container's remaining octets | N/A | one octet past the container |
| Total NLRI Length | 0 - 65535 | the sum of its descriptor TLVs | one below that sum | one above that sum |
| BGP-LS NLRI TLV header | fixed 4 octets | 4 | 3 octets of tail | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-ls-malformed-nlri` | `test/decode/bgp-ls-malformed-nlri.ci` | an operator's peer sends a truncated Link-State NLRI and ze does not propagate it | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| `bgp-ls-consumer-facing` | `test/decode/bgp-ls-consumer-facing.ci` | an operator declares a peer Consumer-facing and BGP-LS from it stops reaching the RIB | | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-ls-receive-gobgp` | `test/interop/scenarios/` | GoBGP | a real peer's valid BGP-LS is accepted, the session stays established, and the new validator false-positives on nothing GoBGP sends | |

## Files to Modify
- `internal/component/bgp/message/rfc7606.go` - `validateMPNLRISyntax` gains the
  BGP-LS branch
- `internal/component/bgp/reactor/session_validation.go` - the action ladder for
  the skipable and unprocessable classes
- `internal/component/bgp/plugins/nlri/ls/plugin.go` - the declared role reaches
  the plugin's config
- `rfc/short/rfc9552.md` - `8.2.2-9` and `8.2.6-2` gain tagged tests; the `{gap}`
  on `8.2.2-5` and the row for `8.2.2-6` are resolved
- `docs/features/rfc-status.md` - the RFC 9552 row, with source anchors
- `docs/architecture/wire/nlri-bgpls.md` - declared by the `// Design:` header of
  `rfc7606_bgpls.go`; the NLRI-side validation joins the attribute-side already
  documented there

## Files to Create
- `internal/component/bgp/message/rfc7606_bgpls_nlri.go` - the NLRI walk, beside <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
  `rfc7606_bgpls.go` and for the same reason: `rfc7606.go` is past its line limit
- `internal/component/bgp/message/rfc7606_bgpls_nlri_test.go` - the six NLRI tests <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
- `internal/component/bgp/reactor/session_validation_bgpls_test.go` - the two session tests <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
- `internal/component/bgp/plugins/nlri/ls/role.go` - the declared role and its filter append <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
- `internal/component/bgp/plugins/nlri/ls/role_test.go` - the three role tests <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
- `internal/component/bgp/plugins/nlri/ls/yang/ze-bgp-ls-role.yang` - the role leaf <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
- `test/decode/bgp-ls-malformed-nlri.ci` - functional test for the validator <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
- `test/decode/bgp-ls-consumer-facing.ci` - functional test for the role <!-- doc-links: ignore (file this open spec plans and has not created yet) -->

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/plugins/nlri/ls/yang/ze-bgp-ls-role.yang`, augmenting peer, group/peer and group as `ze-role.yang` does | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| YANG validation constraints | Yes | an `enumeration`, so an unknown role is refused by the schema rather than by a handler |
| YANG custom validators | No | the enumeration is the whole constraint |
| CLI commands/flags | No | configuration only |
| CLI grammar (keyword before value) | N-A | no new command |
| Editor autocomplete | Yes | automatic for a YANG enumeration leaf |
| Functional test for new RPC/API | Yes | both `.ci` files above |
| Pipe completeness | N-A | no new command output |
| Env var registration | No | per-peer config, not an environment default |
| Doctor check for runtime dependencies | No | no new file, socket, port, module or binary |
| Prometheus counters/metrics | Yes | a counter for malformed BGP-LS NLRI by action taken, so an operator can see a peer misbehaving |
| BGP family surface (new SAFI / capability / attribute) | No | no new family, capability or attribute; the families are registered already |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` for the declared BGP-LS role |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` for the role leaf |
| 3 | CLI command added/changed? | No | no new verb |
| 4 | API/RPC added/changed? | No | no new RPC |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, the ls plugin gains config |
| 6 | Has a user guide page? | Yes | `docs/guide/bgp-ls.md` | <!-- doc-links: ignore (page this open spec plans and has not written yet) -->
| 7 | Wire format changed? | No | no byte ze writes changes; what changes is which received bytes it refuses |
| 8 | Plugin SDK/protocol changed? | No | no SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9552.md` and the RFC 9552 row of `docs/features/rfc-status.md`, with source anchors |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` for the new scenario |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`, ze gains BGP-LS fault management |
| 12 | Internal architecture changed? | Yes | `docs/architecture/wire/nlri-bgpls.md`. Two more docs are DECLARED by changed files and are unaffected: `docs/architecture/wire/messages.md` is `rfc7606.go`'s design doc and describes the RFC 7606 engine, which gains a caller for AFI 16388 and no new stage or action; `docs/architecture/core-design.md` is `plugin.go`'s and describes the registration pattern, which the ls plugin still follows unchanged -- its families and Mode do not move in this spec |
| 13 | Route metadata keys added/changed? | No | no new metadata key |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | nothing new registers; the ls plugin's registration is unchanged |
| 16 | Any changed source file referenced by existing doc source anchors? | DERIVED | run `./le spec citation anchors spec plan/immediate/spec-bgp-ls-receiver-fault-management.md` at implementation time |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | verify every BGP-LS example against the new YANG |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the branch exists and is reached
   - Tests: `TestBGPLSNLRIWithOverrunningTLVIsMalformed`, `TestValidBGPLSNLRIReachesThePlugins`
   - Files: `internal/component/bgp/message/rfc7606.go`, `rfc7606_bgpls_nlri.go`
   - Verify: AFI 16388 no longer falls through. The walk is a stub, so the malformed test fails and the valid one passes
   - Also settle A-1, A-2 and A-4 here: read the §5.4 typed-NLRI discard branch whole, grep the reactor for a per-family disable, and read §5.1's ordering sentence against §8.2.2's opening constraint. All three change what the next phases build
2. **Phase: The length walk** -- the four length bullets
   - Tests: `TestBGPLSMPReachTLVLengthsMustSumToTheAttribute`, `TestBGPLSMPUnreachTLVLengthsMustSumToTheAttribute`, `TestBGPLSSemanticOddityIsNotMalformed`
   - Files: `internal/component/bgp/message/rfc7606_bgpls_nlri.go` <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
   - Verify: each malformed shape is caught and the semantically odd one is not
3. **Phase: Ordering and descriptor uniqueness** -- the two structural bullets
   - Tests: `TestBGPLSNLRITLVsOutOfOrderAreDiscarded`, `TestBGPLSNodeDescriptorRefusesARepeatedSubTLV`
   - Files: `internal/component/bgp/message/rfc7606_bgpls_nlri.go` <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
   - Verify: ordering is a skipable error and reaches NLRI discard, not session reset
4. **Phase: The action ladder** -- skipable, disable, reset
   - Tests: `TestEnforceRFC7606ActsOnAMalformedBGPLSNLRI`, `bgp-ls-malformed-nlri`
   - Files: `internal/component/bgp/reactor/session_validation.go`
   - Verify: a BGP-LS-only session resets and a mixed session disables the family
5. **Phase: The declared role** -- §8.2.6 enforced rather than documented
   - Tests: `TestConsumerFacingPeerDropsBGPLSOnImport`, `TestUndeclaredPeerStillPropagatesBGPLS`, `TestConsumerFacingPeerKeepsItsOtherFamilies`, `bgp-ls-consumer-facing`
   - Files: `internal/component/bgp/plugins/nlri/ls/role.go`, `yang/ze-bgp-ls-role.yang`, `plugin.go` <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
   - Verify: declaring the role drops BGP-LS on import with no filter written, and omitting it changes nothing
6. **Phase: Interop and the ledger** -- proof against a real peer
   - Tests: `bgp-ls-receive-gobgp`
   - Files: `test/interop/scenarios/`, `rfc/short/rfc9552.md`, `docs/features/rfc-status.md`
   - Verify: GoBGP's valid BGP-LS is accepted, and the four rows are proven rather than annotated

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | the walk reads NO TLV type and NO value byte. §8.2.2 opens by forbidding a malformed verdict based on inclusion, exclusion or contents, and a `switch` on a TLV type is the tell |
| Correctness | every loop iteration advances by at least the 4-octet header, so a zero-length TLV cannot stall the receive path |
| Correctness | the skipable class reaches NLRI discard and the unprocessable class reaches disable or reset. Collapsing the two into session reset is a conformance failure that reads as caution |
| Naming | the role enumeration names the ROLE (`consumer-facing`), never the action it causes |
| Data flow | the role's import drop goes through the existing filter machinery; no per-feature branch is added to the reactor |
| Rule: `ai/rules/interop-and-goal-validation.md` | each test is proven to FAIL with its check reverted, and the vacuity trap here is specific: a conforming peer never sends a malformed NLRI, so the interop scenario alone proves nothing about the validator |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| BGP-LS NLRI is validated | `grep -n 'AFIBGPLS' internal/component/bgp/message/rfc7606.go` names the branch |
| the four rows are proven | `./le rfc check` exits 0 on rfc9552's 8.2.2 and 8.2.6 rows |
| the role enforces itself | `test/decode/bgp-ls-consumer-facing.ci` PASS with no `family-filter` in its config | <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
| no false positive against a real peer | `./le integration interop` with `bgp-ls-receive-gobgp` PASS |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | every length is peer-controlled and every one is bounds-checked against its container before use; no length is trusted to index |
| Resource exhaustion | a zero-length TLV, a TLV claiming 65535 octets, and a deeply repeated descriptor must each terminate the walk in bounded time |
| Error leakage | the diagnostic names the offending offset and the RFC section, never the peer's whole UPDATE in a log line an operator cannot rotate |
| Authorization failing open | an unrecognised declared role must refuse the config, never fall back to accepting BGP-LS |

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

- §8.2.2's hardest constraint is not what to check but what NOT to. It opens by
  forbidding a malformed verdict based on the inclusion, exclusion or contents of
  TLVs, then lists seven checks that are all about lengths and structure. A
  validator that grows a `switch` on TLV type has crossed the line, whatever it
  does in the cases.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The BGP-LS role is DECLARED per peer, never inferred | infer it from an unconditional import drop on every BGP-LS peer; infer it from RFC 9234 OTC Role | nothing on the wire says a peer only serves Consumers, and §1.1 says the roles mix per speaker, so an unconditional drop would break the Propagator role ze plays today. RFC 9234 cannot carry it either: its five roles name a commercial relationship, it is eBGP-only, and §5 says its procedures "MUST NOT be applied to other address families by default" |
| The role drives an automatic import drop, rather than documenting a filter | a tagged test over a hand-written `family-filter` on `bgp-ls` | Thomas chose this on 2026-08-26. A test over a hand-written filter proves only that a policy is expressible; an automatic drop makes the MUST enforced, and the operator cannot write it wrong. `internal/component/bgp/plugins/role` is the precedent: a declared role driving non-overridable filtering |
| The NLRI walk lives in its own file | extend `rfc7606.go` | the same reason `rfc7606_bgpls.go` gives for the attribute side: `rfc7606.go` is already past the 1000-line limit |
| Interop is a NEGATIVE-space test | rely on the interop scenario as the validator's proof | a conforming peer never sends a malformed NLRI, so the scenario proves the validator false-positives on nothing. The validator's own proof is the unit fixtures, and the spec says so rather than letting a green interop run imply coverage it does not have |

## Known Limitations

- Recognized sub-TLV lengths (§8.2.2 bullet 4, second half) are checked only for
  TLVs the walk can bound without reading a type. `validateBGPLSAttr` already made
  that call on the attribute side and recorded why: §8.2.2 lists fixed-length TLV
  correctness among the SEMANTIC validations a Propagator does not perform.
- The Consumer-facing role governs import only. §8.2.3's SHOULD-level export
  controls, and its advertisement-rate and RIB-size limits, are out of scope.

## RFC Documentation (Scope: protocol)

Add `// RFC 9552 Section 8.2.2: "<quoted requirement>"` above each check, quoting
the bullet it implements, and one above the action ladder quoting the paragraph
that assigns 'NLRI discard', 'AFI/SAFI disable' and 'session reset' to their error
classes. The declared role's apply path quotes §8.2.6 and §10.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
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

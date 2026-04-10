# Spec: cmd-2 -- Session Policy Knobs

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 3/4 |
| Updated | 2026-04-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-cmd-0-umbrella.md` -- umbrella context
3. `internal/component/bgp/schema/ze-bgp-conf.yang` -- peer-fields grouping
4. `internal/component/bgp/reactor/reactor_api_forward.go` -- UPDATE forwarding

## Task

Add four session-level configuration knobs to Ze's BGP peer configuration. All are YANG leaves
in the existing `peer-fields` grouping, inheritable at bgp/group/peer levels.

### Send-Community Control

| Command | Purpose |
|---------|---------|
| `set bgp peer X session community send standard large` | Send only standard and large communities |
| `set bgp peer X session community send all` | Send all community types (default) |
| `set bgp peer X session community send none` | Suppress all communities |

| Leaf | Type | Default |
|------|------|---------|
| `session/community/send` | leaf-list, enum {standard, large, extended, all, none} | all |

Default `all` matches Junos behavior. `none` suppresses all. Leaf-list allows granular control.

### Default-Originate

| Command | Purpose |
|---------|---------|
| `set bgp peer X session family ipv4/unicast default-originate` | Originate default route to peer |
| `set bgp peer X session family ipv4/unicast default-originate-filter only-if-route` | Conditional origination |

| Leaf | Type | Default | Location |
|------|------|---------|----------|
| `default-originate` | boolean | false | per-family (inside `session/family`) |
| `default-originate-filter` | string (filter name) | (none) | per-family |

### Local-AS Modifiers

| Command | Purpose |
|---------|---------|
| `set bgp peer X session asn local-options no-prepend` | Do not prepend real ASN |
| `set bgp peer X session asn local-options replace-as` | Replace real ASN entirely |

| Leaf | Type | Default | Location |
|------|------|---------|----------|
| `local-options` | leaf-list, enum {no-prepend, replace-as} | (empty) | `session/asn` |

### AS-Override

| Command | Purpose |
|---------|---------|
| `set bgp peer X session as-override` | Replace peer's ASN in AS-PATH with local ASN |

| Leaf | Type | Default |
|------|------|---------|
| `as-override` | boolean | false |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- reactor, UPDATE forwarding
  -> Constraint: attribute modifications happen during forwarding, not at receive
- [ ] `.claude/patterns/config-option.md` -- YANG leaf addition pattern
  -> Constraint: YANG leaf + resolver + reactor wiring + .ci test

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` -- BGP-4 base: AS-PATH, communities in UPDATE
  -> Constraint: community attributes are optional transitive (type 8, 16, 32)

**Key insights:**
- Send-community filtering removes community attributes from outbound UPDATEs
- Default-originate generates a synthetic UPDATE with 0.0.0.0/0 or ::/0
- Local-AS modifiers affect AS-PATH construction during OPEN and UPDATE
- AS-override rewrites AS-PATH during egress forwarding

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` -- peer-fields, session container, family list
- [ ] `internal/component/bgp/config/peers.go` -- PeersFromTree() peer config extraction
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` -- UPDATE forwarding, attribute handling
- [ ] `internal/component/bgp/reactor/forward_build.go` -- buildModifiedPayload()
- [ ] `internal/component/bgp/reactor/session_open.go` -- OPEN message building with AS-PATH

**Behavior to preserve:**
- All community types currently sent by default (Ze already behaves like Junos)
- AS-PATH construction in OPEN message
- Existing attribute modification during forwarding
- All existing config files parse identically

**Behavior to change:**
- Community attributes optionally stripped from outbound UPDATEs based on send config
- Default route optionally originated per-family per-peer
- Local-AS prepending behavior modified by no-prepend/replace-as options
- AS-PATH modified on egress when as-override is enabled

## Data Flow (MANDATORY)

### Entry Point
- Config: YANG leaves parsed during config load
- Wire: UPDATE attributes modified during egress forwarding

### Transformation Path
1. Config parse: YANG leaves extracted by ResolveBGPTree()
2. Peer creation: send-community, default-originate, local-as options, as-override stored in peer struct
3. Session establishment: default-originate triggers synthetic UPDATE generation
4. UPDATE egress: community attributes filtered by send config; AS-PATH modified by as-override

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Reactor | PeersFromTree() extracts new leaves into peer struct | [ ] |
| Reactor -> Wire | buildModifiedPayload() strips communities, rewrites AS-PATH | [ ] |

### Integration Points
- `PeersFromTree()` -- extract new YANG leaves
- `buildModifiedPayload()` -- community stripping, AS-override
- Session establishment handler -- default-originate trigger
- OPEN builder -- local-as modifier handling

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality (community stripping extends existing modify path)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `session community send standard` | -> | Outbound UPDATE has only standard communities | `test/plugin/send-community.ci` |
| Config with `session family ipv4/unicast default-originate` | -> | Default route sent to peer after session up | `test/plugin/default-originate.ci` |
| Config with `session asn local-options no-prepend` | -> | Real ASN not prepended in AS-PATH | `test/plugin/local-as-noprepend.ci` |
| Config with `session as-override` | -> | Peer ASN replaced with local ASN in outbound AS-PATH | `test/plugin/as-override.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `community send standard` | Only COMMUNITIES (type 8) in outbound UPDATE; no LARGE or EXTENDED |
| AC-2 | `community send none` | No community attributes in outbound UPDATE |
| AC-3 | `community send all` (default) | All community types present in outbound UPDATE |
| AC-4 | `community send standard large` | COMMUNITIES and LARGE_COMMUNITY present; no EXTENDED |
| AC-5 | `default-originate` on ipv4/unicast | 0.0.0.0/0 originated to peer after session established |
| AC-6 | `default-originate` on ipv6/unicast | ::/0 originated to peer after session established |
| AC-7 | `default-originate-filter X` with filter X rejecting | Default route NOT originated |
| AC-8 | `default-originate-filter X` with filter X accepting | Default route originated |
| AC-9 | `local-options no-prepend` | Real ASN not prepended before local-as in AS-PATH |
| AC-10 | `local-options replace-as` | Local-as completely replaces real ASN in AS-PATH |
| AC-11 | `local-options no-prepend replace-as` (both) | Full ASN replacement, no prepend |
| AC-12 | `as-override` | Peer's ASN occurrences in AS-PATH replaced with local ASN on egress |
| AC-13 | No new config (existing deployments) | Behavior identical to current Ze |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSendCommunityStandard` | `reactor_forward_test.go` | Only standard communities in output | |
| `TestSendCommunityNone` | `reactor_forward_test.go` | No communities in output | |
| `TestSendCommunityAll` | `reactor_forward_test.go` | All community types present | |
| `TestDefaultOriginateIPv4` | `reactor_forward_test.go` | 0.0.0.0/0 generated | |
| `TestDefaultOriginateIPv6` | `reactor_forward_test.go` | ::/0 generated | |
| `TestDefaultOriginateConditional` | `reactor_forward_test.go` | Filter controls origination | |
| `TestLocalASNoPrepend` | `session_open_test.go` | Real ASN absent from AS-PATH | |
| `TestLocalASReplaceAS` | `session_open_test.go` | Local-as replaces real ASN | |
| `TestASOverride` | `reactor_forward_test.go` | Peer ASN replaced in outbound AS-PATH | |

### Boundary Tests (MANDATORY for numeric inputs)

No numeric inputs in this spec (all enum/boolean/string).

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | N/A | N/A | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `send-community` | `test/plugin/send-community.ci` | Verify community filtering in wire output | |
| `default-originate` | `test/plugin/default-originate.ci` | Verify default route originated after session up | |
| `local-as-noprepend` | `test/plugin/local-as-noprepend.ci` | Verify AS-PATH construction with no-prepend | |
| `as-override` | `test/plugin/as-override.ci` | Verify peer ASN replaced in wire output | |

## Files to Modify

- `internal/component/bgp/schema/ze-bgp-conf.yang` -- add send, default-originate, local-options, as-override leaves
- `internal/component/bgp/config/peers.go` -- extract new leaves
- `internal/component/bgp/reactor/peer.go` -- add fields to Peer struct
- `internal/component/bgp/reactor/reactor_api_forward.go` -- community stripping, as-override
- `internal/component/bgp/reactor/forward_build.go` -- community type filtering
- `internal/component/bgp/reactor/session_open.go` -- local-as modifier handling

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new leaves) | [x] | `ze-bgp-conf.yang` |
| Functional test | [x] | `test/plugin/send-community.ci` etc. |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc4271.md` |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` |

## Files to Create

- `test/plugin/send-community.ci`
- `test/plugin/default-originate.ci`
- `test/plugin/local-as-noprepend.ci`
- `test/plugin/as-override.ci`

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-verify` |

### Implementation Phases

1. **Phase: YANG + Config** -- Add all four features' leaves, extract in PeersFromTree()
2. **Phase: Send-Community** -- Community type filtering during egress
   - Tests: `TestSendCommunity*`
3. **Phase: Default-Originate** -- Synthetic default route generation
   - Tests: `TestDefaultOriginate*`
4. **Phase: Local-AS + AS-Override** -- AS-PATH manipulation
   - Tests: `TestLocalAS*`, `TestASOverride`
5. **Functional tests** -- .ci tests for each feature
6. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 13 ACs demonstrated |
| Community types | Correct attribute type codes: 8 (standard), 16 (extended), 32 (large) |
| Default route | Correct prefix for each AFI (0.0.0.0/0 for IPv4, ::/0 for IPv6) |
| AS-PATH encoding | 4-byte ASN encoding preserved after as-override |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| YANG leaves | `grep -E 'send|default-originate|local-options|as-override' ze-bgp-conf.yang` |
| .ci tests | `ls test/plugin/send-community.ci test/plugin/default-originate.ci` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | send leaf-list values must be from enum; filter name validated against registry |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in phase |
| 3 fix attempts fail | STOP. Ask user. |

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

## Implementation Summary

### What Was Implemented

**YANG + Config -- DONE (all 4 features):**
- `session/as-override` (boolean, default false)
- `session/asn/local-options` (leaf-list: no-prepend, replace-as)
- `session/community/send` (leaf-list: standard, large, extended, all, none)
- `session/family/*/default-originate` (boolean) + `default-originate-filter` (string)
- PeerSettings: `ASOverride`, `LocalASNoPrepend`, `LocalASReplaceAS`, `SendCommunity`, `DefaultOriginate`, `DefaultOriginateFilter`
- Config extraction in `parsePeerFromTree()` and `parseFamiliesFromTree()`
- 4 unit test functions in `config_test.go`
- Parse test: `test/parse/session-policy-config.ci`

**Send-Community Wire Behavior -- DONE:**
- `AttrModSuppress` action added to registry (removes entire attribute from UPDATE)
- `genericAttrSetHandler` updated to handle suppress (last action wins)
- `applySendCommunityFilter()` suppresses community types 8/16/32 based on peer config
- 8 unit test subtests (all community type combinations)

**AS-Override Wire Behavior -- DONE:**
- `applyASOverride()` extracts AS_PATH via `AttributesWire.GetRaw()`, replaces peer ASN with local ASN
- `rewriteASPathOverride()` two-pass: scan for match, then copy+replace
- Respects ASN4 negotiation state
- 3 unit test subtests

**Default-Originate -- DONE (unconditional only):**
- `sendDefaultOriginateRoutes()` generates 0.0.0.0/0 (IPv4) or ::/0 (IPv6) UPDATE
- `defaultRouteForAFI()` resolves prefix and next-hop per AFI
- Sent after static routes, before opQueue drain
- Uses `message.UpdateBuilder.BuildUnicast()` for correct wire encoding

### What Remains

| Item | Effort | Design needed |
|------|--------|---------------|
| Local-AS modifier behavior in OPEN | Medium | No -- modify `buildLocalOpen()` in `session_open.go` to respect `LocalASNoPrepend`/`LocalASReplaceAS` when constructing AS_PATH in OPEN and prepending during forwarding. Read `session_open.go` to find where local AS is prepended. |
| Default-originate-filter conditional check | Small | No -- in `sendDefaultOriginateRoutes`, look up filter name in policy registry, call filter function, skip sending if filter rejects. Requires the filter registry to be accessible from the peer context. |

**Implementation notes for local-AS modifiers:**
- `LocalASNoPrepend`: when local-as override is set, Ze normally prepends real ASN before local-as in outbound AS_PATH. With no-prepend, skip the real ASN prepend.
- `LocalASReplaceAS`: local-as completely replaces real ASN. In OPEN, advertise local-as as MyAS. In AS_PATH prepend, use local-as only.
- Both together: full replacement with no extra prepend.
- Key file: `session_open.go` for OPEN construction, `reactor_api_forward.go` for AS_PATH prepend during forwarding.

### Bugs Found/Fixed
- None

### Documentation Updates
- `docs/guide/command-reference.md` not yet updated (deferred to spec completion)

### Deviations from Plan
- Default-originate-filter conditional check deferred (TODO in code)
- Local-AS modifier OPEN behavior deferred (requires session_open.go changes)

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-cmd-2-session-policy.md`
- [ ] Summary included in commit

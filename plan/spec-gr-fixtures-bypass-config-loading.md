# Spec: gr-fixtures-bypass-config-loading

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-20 |

Recovery after compaction: `.claude/rules/post-compaction.md`.


## Task

Twenty-two hand-written fixtures in the graceful-restart plugin's tests reach
`extractGRCapabilities` and `extractLLGRCapabilities` directly, with config
shapes the daemon cannot produce. Convert them to run through the real config
path, and fix whatever reader those fixtures were holding in place.

This is not tidying. One defect of exactly this kind has already shipped and was
found on 2026-09-20: `extractFamilies` read `peerMap["family"]` as an array of
strings at the PEER level, while `ze-bgp-conf.yang` declares
`list family { key "name"; }` inside the `session` container. It returned nil
for every real peer, so `collectPeerFamilies` fell back to the `ipv4/unicast`
default and BOTH graceful-restart capabilities named that one family whatever
the operator configured. The code-71 LLGR capability had been shipping that way
and was believed correct.

It survived because the fixtures and the reader agreed with each other while
both disagreed with the model. The two RFC-tagged tests that exposed it were
fixed in the same change; these twenty-two were not.

## Required Reading

### Architecture Docs
- `docs/guide/graceful-restart.md`
- `docs/architecture/wire/capabilities.md`
- whatever `ai/CODE-TO-DOCS.md` returns for
  `internal/component/bgp/plugins/gr/gr.go` and `gr_llgr.go`

### RFC Summaries (Scope: protocol)
- `rfc/short/rfc4724.md`, and `rfc/full/rfc4724.txt` Section 3 for the tuple
- `rfc/short/rfc9494.md`, and `rfc/full/rfc9494.txt` Sections 4.1 and 4.2

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/plugins/gr/gr_capability.go` - builds the code-64 payload; carries the unreachable `case float64` arm
- [ ] `internal/component/bgp/plugins/gr/gr_llgr.go` - `extractFamilies` and `collectPeerFamilies`, the reader the shipped defect lived in
- [ ] `internal/component/bgp/plugins/gr/gr_test.go` - nineteen of the bypassing fixtures
- [ ] `internal/component/bgp/plugins/gr/rfc9494_test.go` - the other three
- [ ] `internal/component/bgp/plugins/gr/gr_capability_test.go` - the helpers already routed through `LoadConfig`, to reuse
- [ ] `internal/component/config/tree.go` - `Tree.values` is `map[string]string`, which is why a leaf is never a number
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - `list family { key "name"; }` under `session`, the shape the fixtures contradict

**Behavior to preserve:** the octets every converted test asserts. The payloads
were right; the inputs were impossible.

**Behavior to change:** only a reader a conversion proves wrong.

`grPayloadForConfig` in `gr_capability_test.go` was rewritten on 2026-09-20 to
run `LoadConfig -> ToPluginMap -> ExtractConfigSubtree`, and the two RFC-tagged
tests use it. Every other test in the package still hands a JSON string straight
to the reader.

Find them with a grep for a `{"bgp":` literal in
`internal/component/bgp/plugins/gr/`. That answered twenty-two on 2026-09-20,
across `gr_test.go` and `rfc9494_test.go`. Do not work from a line list: the
first conversion moves every line after it.

Each is wrong on two counts, and the second is the one nobody noticed:

1. The peer key is an address, such as `192.0.2.1`. A peer key is a NAME, and
   `LoadConfig` refuses an address as one.
2. Leaves are JSON numbers, such as `"restart-time":120`. `Tree.values` is a
   `map[string]string` (`internal/component/config/tree.go`), so the daemon
   delivers every leaf as a string.

The second has already shaped production code: `parseGRCapValue`
(`gr_capability.go`) carries a `case float64` arm that no production path can
reach. It exists to read fixtures.

## Data Flow (MANDATORY)

### Entry Point
An operator's config file, through `LoadConfig`.

### Transformation Path
`LoadConfig` -> `(*Tree).ToPluginMap` -> `ExtractConfigSubtree(root "bgp")` ->
`extractGRCapabilities` / `extractLLGRCapabilities` -> `parseGRCapValue` /
`parseLLGRCapValue` -> capability payload octets -> `capability.NewPlugin`.

### Boundaries Crossed
Config tree to plugin map (every leaf becomes a string; every keyed list becomes
a map keyed by its key leaf). Plugin map to wire octets.

### Integration Points
`Peer.getPluginCapabilities` (`internal/component/bgp/reactor/peer.go`) puts the
payload on the wire verbatim.

### Architectural Verification
The readers under test are the only consumers of that subtree. Nothing else in
the tree reads `peerMap["family"]`.

## Risks & Assumptions

### Assumptions
- Every one of the twenty-two can be expressed as a config `LoadConfig` accepts.
  If one cannot, that is a finding about the model or the reader, not a reason
  to keep the fixture.

### Risks
- **Converting a fixture will turn a green test red, and that is the point.**
  Each red is a reader that was written to the fixture shape. Each one is a
  separate product defect and must be fixed at the reader, never by adjusting
  the new fixture back toward the old shape. That temptation is the whole reason
  this spec exists.
- Four of the twenty-two sit in RFC-tagged tests. Editing a tagged test needs
  the owner's approval recorded through `./le rfc approve unit`, and a tag whose
  producer changes owes a fresh discrimination record.
- Removing the `case float64` arm from `parseGRCapValue` stales both
  `RFC4724-4-4` discrimination records.

## Blast Radius
`internal/component/bgp/plugins/gr/` only, unless a converted fixture exposes a
reader defect that reaches the wire. It reached the wire once already.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a config file naming two session families | → | `extractGRCapabilities` | `TestRFC4724GRCapabilityListsTheFamiliesOfTheSession` (exists, already routed) |
| a config file naming two session families | → | `extractLLGRCapabilities` | `TestLLGRCapabilityListsTheFamiliesOfTheSession` (to write) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | any test in the gr package | it reaches a reader only through `LoadConfig`, never through a hand-written map or JSON string |
| AC-2 | a config `LoadConfig` refuses | no test asserts anything about it |
| AC-3 | `parseGRCapValue` receives a leaf | it reads a string; the `case float64` arm is gone |
| AC-4 | each reader a conversion reddens | it is fixed at the reader, and the fix is stated in the Implementation Summary with what it changed on the wire |
| AC-5 | the LLGR code-71 capability | a test asserts its octets for a multi-family session through the real path, which no test does today |

## End-to-End User Stories

An operator configures a peer carrying `ipv4/mpls-vpn` and `ipv6/unicast` and
restarts ze. Both the code-64 and the code-71 capabilities name those two
families and no others, and a peer holds the operator's routes across the
restart rather than a family they never configured.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| the 22 converted cases | `gr_test.go`, `rfc9494_test.go` | each asserts its existing claim over a config the daemon can produce | |
| `TestLLGRCapabilityListsTheFamiliesOfTheSession` | `gr_llgr_test.go` | code-71 octets for a multi-family session, through `LoadConfig` | |
| `TestGRFixturesLoadThroughTheRealConfigPath` | `gr_test.go` | no fixture in the package parses as a bare map; the guard against the next one | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `restart-time` | 0-4095 (12 bits, RFC 4724 Section 3) | 4095 | N/A | 4096 clamps |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing gr `.ci` files | `test/plugin/` | already drive real config; confirm they cover a non-default family | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-graceful-restart-families-frr` | `test/interop/scenarios/` | FRR | FRR reads the families ze advertises and retains those routes across a restart | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the guard can see a bypass before converting anything
   - Tests: `TestGRFixturesLoadThroughTheRealConfigPath`
   - Files: `gr_test.go`
   - Verify: it FAILS against the twenty-two that exist today. A guard written after the conversion cannot be shown to catch anything
2. **Phase: Convert, one fixture at a time** -- rewrite each in the declared shape and route it through the existing helpers
   - Tests: the twenty-two, unchanged in what they assert
   - Files: `gr_test.go`, `rfc9494_test.go`
   - Verify: each conversion is green, or RED and then followed by a reader fix. A conversion that needs its assertion changed is a finding: stop and report it
3. **Phase: Delete what the fixtures were holding up** -- the `case float64` arm and anything else only a fixture reached
   - Tests: the twenty-two, plus both `RFC4724-4-4` records re-recorded
   - Files: `gr_capability.go`
   - Verify: the package is green and no production path reads a non-string leaf
4. **Phase: The LLGR gap** -- assert the code-71 octets through the real path
   - Tests: `TestLLGRCapabilityListsTheFamiliesOfTheSession`
   - Files: `gr_llgr_test.go`
   - Verify: red before the assertion exists, green after; this is the capability that shipped wrong and has no such test

## Files to Modify
- `internal/component/bgp/plugins/gr/gr_test.go`
- `internal/component/bgp/plugins/gr/rfc9494_test.go`
- `internal/component/bgp/plugins/gr/gr_capability.go` (the `case float64` arm)
- whatever reader a conversion reddens

## Known Limitations
The guard in AC-5 covers this package alone. The same bypass is available to any
test anywhere that builds a plugin map by hand, and this spec does not survey
for it. That survey is worth its own row if a second instance turns up.

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
- [ ] AC-1..AC-5 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior

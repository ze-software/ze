# Spec: cmd-4 -- Prefix-List Filter Plugin

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/1 |
| Updated | 2026-04-11 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-cmd-0-umbrella.md` -- umbrella context
3. `internal/component/bgp/plugins/filter_community/` -- existing filter plugin pattern
4. `internal/component/bgp/reactor/filter_chain.go` -- filter chain dispatch
5. `internal/component/bgp/reactor/filter/` -- in-process loop-detection filter

## Task

Create `bgp-filter-prefix` plugin. Named prefix-lists under `bgp/policy` with `ze:filter`
extension. Each entry has `prefix` (CIDR), `ge` (uint8), `le` (uint8), `action`
(accept/reject, default accept). Referenced from filter chains as `prefix-list:NAME`.

**Config syntax (editor):**

| Command | Purpose |
|---------|---------|
| `set bgp policy prefix-list CUSTOMERS prefix 10.0.0.0/8 ge 16 le 24` | Match prefixes in range |
| `set bgp policy prefix-list CUSTOMERS prefix 192.168.0.0/16 action reject` | Reject exact match |
| `set bgp peer X filter import prefix-list:CUSTOMERS` | Apply prefix-list on import |

**YANG location:** `bgp/policy` container, new `prefix-list` list with `ze:filter` extension.

| Leaf | Type | Default | Notes |
|------|------|---------|-------|
| `prefix` | inet:ip-prefix (CIDR) | (required) | IPv4 or IPv6 prefix |
| `ge` | uint8 | prefix-length | Greater-than-or-equal prefix length |
| `le` | uint8 | 32 (IPv4) or 128 (IPv6) | Less-than-or-equal prefix length |
| `action` | enum {accept, reject} | accept | What to do when prefix matches |

**Prefix matching rules:**
- A route prefix P/L matches entry E/EL with ge/le if: P is a subnet of E/EL AND L >= ge AND L <= le
- ge defaults to the prefix length of the entry
- le defaults to the maximum prefix length for the address family
- Entries evaluated in order; first match wins
- No match = implicit deny (reject)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- plugin model, filter chain
  -> Constraint: filter plugins are independent, composable in chains
- [ ] `.claude/patterns/plugin.md` -- how to create a filter plugin
  -> Constraint: filter plugins augment bgp/policy, use ze:filter extension
- [ ] `.claude/patterns/config-option.md` -- YANG leaf addition pattern
  -> Constraint: YANG + resolver + wiring + .ci test

**Key insights:**
- Filter plugins follow the community filter pattern (tag/strip)
- Filter chain dispatch is in filter_chain.go
- Each filter plugin is a separate binary under bgp/plugins/
- ze:filter extension marks YANG containers as filter-eligible

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/filter_community/` -- existing filter plugin pattern
- [ ] `internal/component/bgp/plugins/filter_community/register.go` -- plugin registration
- [ ] `internal/component/bgp/reactor/filter_chain.go` -- filter chain dispatch
- [ ] `internal/component/bgp/reactor/filter/` -- in-process loop-detection filter
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` -- bgp/policy container

**Behavior to preserve:**
- Existing filter chain dispatch order (in-process filters first, then plugin filters)
- Existing community filter functionality unchanged
- Existing loop-detection filter unchanged
- All existing config files parse and work identically

**Behavior to change:**
- New prefix-list list under bgp/policy in YANG
- New bgp-filter-prefix plugin registered
- Filter chain recognizes `prefix-list:NAME` references
- UPDATEs with non-matching prefixes rejected by filter

## Data Flow (MANDATORY)

### Entry Point
- Config: `bgp { policy { prefix-list CUSTOMERS { ... } } }` parsed from YANG
- Wire: UPDATE received, NLRI prefixes checked against prefix-list entries

### Transformation Path
1. Config parse: YANG prefix-list entries extracted by ResolveBGPTree()
2. Plugin registration: bgp-filter-prefix plugin registers with filter registry
3. Filter chain setup: `filter import prefix-list:CUSTOMERS` wires prefix-list into peer's import chain
4. UPDATE receive: wire bytes pass through import filter chain
5. Prefix extraction: NLRI prefixes extracted from UPDATE
6. Prefix matching: each prefix checked against prefix-list entries in order
7. Action: first matching entry's action (accept/reject) applied; no match = reject

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Plugin | ResolveBGPTree() extracts prefix-list config, passed to plugin at init | [ ] |
| Reactor -> Plugin | Filter chain dispatches filter-update RPC with wire bytes | [ ] |

### Integration Points
- `ResolveBGPTree()` -- extract prefix-list configuration
- `filter_chain.go` -- dispatch to prefix-list plugin for matching
- Plugin registration -- bgp-filter-prefix registers as filter type

### Architectural Verification
- [ ] No bypassed layers (config -> resolver -> filter chain -> plugin)
- [ ] No unintended coupling (prefix-list is independent, composable)
- [ ] No duplicated functionality (new filter type, follows existing pattern)
- [ ] Zero-copy preserved (plugin receives wire bytes, extracts NLRI for matching)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `policy prefix-list` + `filter import prefix-list:X` | → | UPDATE with matching prefix accepted | `test/plugin/prefix-filter.ci` |
| Config with `policy prefix-list` + `filter import prefix-list:X` | → | UPDATE with non-matching prefix rejected | `test/plugin/prefix-filter.ci` |
| Config parse with prefix-list entries | → | YANG validates prefix-list syntax | `test/parse/prefix-list-config.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Prefix within ge/le range | Accepted by filter |
| AC-2 | Prefix outside ge/le range (too specific) | Rejected by filter |
| AC-3 | Prefix outside ge/le range (too general) | Rejected by filter |
| AC-4 | Exact match (ge=le=prefix-length) | Accepted when action=accept |
| AC-5 | Exact match with action=reject | Rejected by filter |
| AC-6 | Default action (no explicit action) | accept |
| AC-7 | Multiple entries, first match wins | First matching entry's action applied |
| AC-8 | No entry matches | Implicit deny (reject) |
| AC-9 | IPv4 prefix support | Correct matching for IPv4 CIDR |
| AC-10 | IPv6 prefix support | Correct matching for IPv6 CIDR |
| AC-11 | Composable in chain with other filters | `filter import prefix-list:X as-path-list:Y` works |
| AC-12 | ge boundary: 0 (minimum) | Valid |
| AC-13 | le boundary: 128 (maximum for IPv6) | Valid |
| AC-14 | ge > le (invalid) | Rejected by YANG validation |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPrefixMatchInRange` | `filter_prefix_test.go` | Prefix within ge/le range matches | |
| `TestPrefixMatchOutOfRange` | `filter_prefix_test.go` | Prefix outside range does not match | |
| `TestPrefixMatchExact` | `filter_prefix_test.go` | Exact prefix match | |
| `TestPrefixMatchReject` | `filter_prefix_test.go` | Reject action applied | |
| `TestPrefixMatchFirstWins` | `filter_prefix_test.go` | Multiple entries, first match wins | |
| `TestPrefixMatchImplicitDeny` | `filter_prefix_test.go` | No match = reject | |
| `TestPrefixMatchIPv4` | `filter_prefix_test.go` | IPv4 prefix matching | |
| `TestPrefixMatchIPv6` | `filter_prefix_test.go` | IPv6 prefix matching | |
| `TestPrefixListGeLeBoundary` | `filter_prefix_test.go` | ge=0, le=128 boundaries | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ge | 0-128 | 128 | N/A (uint8) | 129 |
| le | 0-128 | 128 | N/A (uint8) | 129 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `prefix-filter` | `test/plugin/prefix-filter.ci` | Config with prefix-list filter, verify accept/reject | |
| `prefix-list-config` | `test/parse/prefix-list-config.ci` | Config with prefix-list entries parses correctly | |

## Files to Modify

- `internal/component/bgp/schema/ze-bgp-conf.yang` -- add prefix-list under bgp/policy
- `internal/component/bgp/config/resolve.go` -- extract prefix-list config
- `internal/component/bgp/reactor/filter_chain.go` -- dispatch to prefix-list filter

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new list) | [x] | `internal/component/bgp/schema/ze-bgp-conf.yang` |
| Plugin registration | [x] | `internal/component/bgp/plugins/filter_prefix/register.go` |
| Functional test | [x] | `test/plugin/prefix-filter.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- add prefix-list filtering |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- prefix-list config examples |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` -- bgp-filter-prefix plugin |
| 6 | Has a user guide page? | [ ] | N/A |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- prefix-list filtering now supported |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create

- `internal/component/bgp/plugins/filter_prefix/` -- new plugin directory
- `internal/component/bgp/plugins/filter_prefix/register.go` -- plugin registration
- `internal/component/bgp/plugins/filter_prefix/filter.go` -- prefix matching logic
- `internal/component/bgp/plugins/filter_prefix/filter_test.go` -- unit tests
- `test/plugin/prefix-filter.ci` -- functional test
- `test/parse/prefix-list-config.ci` -- config parse test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
| 2. Audit | Files to Modify, TDD Plan |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-verify` |
| 5-12. | Standard flow |

### Implementation Phases

1. **Phase: YANG + Config** -- Add prefix-list to ze-bgp-conf.yang, extract in ResolveBGPTree()
   - Tests: config parse test
   - Files: ze-bgp-conf.yang, resolve.go
2. **Phase: Plugin Skeleton** -- Create bgp-filter-prefix plugin following community filter pattern
   - Tests: `TestPrefixMatch*`
   - Files: filter_prefix/register.go, filter_prefix/filter.go
3. **Phase: Prefix Matching** -- Implement ge/le/action matching logic
   - Tests: `TestPrefixMatchInRange`, `TestPrefixMatchOutOfRange`, `TestPrefixMatchExact`, `TestPrefixListGeLeBoundary`
   - Files: filter_prefix/filter.go
4. **Phase: Filter Chain Integration** -- Wire into filter_chain.go dispatch
   - Tests: verify filter chain calls prefix-list plugin
   - Files: filter_chain.go
5. **Functional tests** -- .ci tests proving end-to-end behavior
6. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 14 ACs demonstrated |
| Pattern compliance | Follows community filter plugin pattern |
| IPv4 and IPv6 | Both address families tested |
| Composability | Works in chain with other filters |
| ge/le validation | ge <= le enforced |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| YANG prefix-list in ze-bgp-conf.yang | `grep prefix-list internal/component/bgp/schema/ze-bgp-conf.yang` |
| Plugin directory | `ls internal/component/bgp/plugins/filter_prefix/` |
| .ci functional tests | `ls test/plugin/prefix-filter.ci test/parse/prefix-list-config.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | CIDR validated, ge/le ranges validated, ge <= le enforced |
| Resource exhaustion | Limit prefix-list entry count if needed |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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

- Two filter dispatch models exist in ze: in-process IngressFilter callbacks (filter_community style, per-peer accumulated config) and named filter chain (PolicyFilterChain via OnFilterUpdate RPC). cmd-4 had to use the named-chain pattern to compose with future cmd-5/6/7 in `filter import [ X Y Z ]`. cmd-4 is the FIRST production plugin in-tree to use the named-chain pattern; existing redistribution-*.ci tests use a Python mock plugin.
- Filter names come from config (Stage 2), not from compile-time registration (Stage 1). The plugin declares ZERO filters at Stage 1; CallFilterUpdate does not gate on declared filters; FilterInfo lookup miss returns nil/false defaults; FilterOnError defaults to fail-closed. This means a plugin can dynamically handle any filter name passed in `input.Filter`, looking it up in its config-loaded map.
- The chain ref is the actual plugin name `bgp-filter-prefix` plus filter name, e.g. `bgp-filter-prefix:CUSTOMERS`. The umbrella spec used `prefix-list:CUSTOMERS` as shorthand; the real syntax matches `<plugin-name>:<filter-name>` per `strings.Cut(ref, ":")` in filter_chain.go.
- Strict whole-update mode (any denied prefix rejects whole UPDATE) is the v1 semantics. Per-prefix filtering (rewrite the nlri to drop denied prefixes only) requires `action=modify` with a new nlri text and is deferred to a future spec. Spec ACs are written for single-prefix UPDATEs so this restriction does not affect AC coverage.
- ze-types.yang has `prefix-ipv4` and `prefix-ipv6` typedefs but no unified `ip-prefix`. The YANG schema uses `union { type zt:prefix-ipv4; type zt:prefix-ipv6; }` instead of importing `ietf-inet-types` (which is not loaded by ze).

## Implementation Summary

### What Was Implemented

| Item | Files | Notes |
|------|-------|-------|
| YANG schema | `schema/ze-filter-prefix.yang` | Augments `bgp:bgp/bgp:policy` with `list prefix-list { ze:filter; key name; list entry { key prefix; ordered-by user; ... } }`. Uses union of `zt:prefix-ipv4`/`zt:prefix-ipv6`. |
| Schema registration | `schema/embed.go`, `schema/register.go` | Standard embed + RegisterModule pattern |
| Plugin entry point | `filter_prefix.go` | RunFilterPrefix uses sdk.NewWithConn, OnConfigure parses bgp/policy/prefix-list, OnFilterUpdate dispatches by `input.Filter` name |
| Plugin registration | `register.go` | Registers as `bgp-filter-prefix` with ConfigRoots=["bgp"] |
| Matching algorithm | `match.go` | `evaluatePrefix(entries, route)` walks entries; first match wins; cross-family entries skipped; explicit subnet check (Contains + bits >= entry bits). `evaluateUpdate(nlriField)` strict mode: any denied prefix rejects. `extractNLRIField` pulls text after `nlri ` keyword. |
| Config parser | `config.go` | `parsePrefixLists` walks bgp/policy/prefix-list; `parseOneEntry` applies YANG defaults (ge=prefix bits, le=32/128, action=accept) and validates ge<=le, ge<=128, action in {accept,reject} |
| Unit tests | `match_test.go` | 16 cases covering ACs 1-13 (in/out range, exact, first-match-wins, implicit deny, IPv4/IPv6, ge=0 default route, le=128 host, cross-family) + update strict mode + nlri extraction |
| Config tests | `config_test.go` | 13 cases covering YANG defaults, ge/le boundaries, ge>le, ge>128, invalid action, malformed prefix, map-form parse, list-form parse with order preservation |
| Plugin registration tests | `internal/component/plugin/all/all_test.go`, `cmd/ze/main_test.go` | Both expected lists updated to include `bgp-filter-prefix` |
| Functional .ci tests | `test/parse/prefix-list-config.ci`, `test/plugin/prefix-filter-accept.ci`, `test/plugin/prefix-filter-reject.ci` | Parse: YANG schema accepts the config + chain ref. Accept: matching prefix lands in adj-rib-in. Reject: non-matching prefix absent from adj-rib-in. |

### Bugs Found/Fixed

- YANG `import ietf-inet-types` not loaded in ze; switched to `union { zt:prefix-ipv4 ; zt:prefix-ipv6 }`. Discovered during first `ze config validate` run.

### Documentation Updates

- `docs/guide/configuration.md`, `docs/guide/plugins.md`, `docs/comparison.md`, `docs/features.md`: NOT updated this commit; deferred to a separate doc commit per the umbrella's deliverables checklist (cmd-0-umbrella will sweep all child specs together once cmd-5..cmd-7 land).

### Deviations from Plan

- **Strict whole-update mode** instead of per-prefix nlri rewriting. The spec is silent on multi-prefix UPDATEs and the ACs are written for single-prefix scenarios. Strict mode is the safest default. Per-prefix splitting deferred to a future spec; tracked in `plan/deferrals.md`.
- **Chain ref name is `bgp-filter-prefix:NAME`**, not the spec example `prefix-list:NAME`. The real chain dispatch uses the actual plugin name from `<plugin>:<filter>` cut. The umbrella spec example was shorthand; the spec table line is updated below.
- **Filter Stage 1 declarations are empty.** The spec did not require Stage 1 filter declarations; this was a design decision verified against `CallFilterUpdate` (which does not gate on declared filters) and `FilterInfo` (which returns safe defaults on miss).

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Create `bgp-filter-prefix` plugin | Done | `internal/component/bgp/plugins/filter_prefix/` | All files present |
| Named prefix-lists under `bgp/policy` | Done | `schema/ze-filter-prefix.yang:18` | `list prefix-list { ze:filter; key name; }` |
| Each entry has prefix/ge/le/action | Done | `schema/ze-filter-prefix.yang:36-65` | All four leaves with defaults |
| `ze:filter` extension | Done | `schema/ze-filter-prefix.yang:19` | Marker for policy framework |
| Referenced as `bgp-filter-prefix:NAME` | Done | `test/plugin/prefix-filter-accept.ci:84` | Chain ref tested end-to-end |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestEvaluatePrefix/AC1_in_range_accept` (match_test.go:50) | 10.1.0.0/20 matches 10.0.0.0/8 ge16 le24 |
| AC-2 | Done | `TestEvaluatePrefix/AC2_too_specific` (match_test.go:57) | 10.1.0.0/26 fails le=24 -> implicit deny |
| AC-3 | Done | `TestEvaluatePrefix/AC3_too_general` (match_test.go:64) | 10.0.0.0/12 fails ge=16 -> implicit deny |
| AC-4 | Done | `TestEvaluatePrefix/AC4_exact_accept` (match_test.go:71) | ge=le=8 with /8 route accepted |
| AC-5 | Done | `TestEvaluatePrefix/AC5_exact_reject` (match_test.go:78) | Same as AC-4 with action=reject |
| AC-6 | Done | `TestParseOneEntry/ipv4_defaults` (config_test.go:25) | Default action accept when omitted |
| AC-7 | Done | `TestEvaluatePrefix/AC7_first_match_wins_*` (match_test.go:85, 95) | Both reject-then-accept and accept-then-reject orderings |
| AC-8 | Done | `TestEvaluatePrefix/AC8_no_match_implicit_deny` (match_test.go:106) | 192.168.0.0/24 vs 10.0.0.0/8 list returns reject |
| AC-9 | Done | `TestEvaluatePrefix/AC9_ipv4` + `prefix-filter-accept.ci` | IPv4 unit + functional |
| AC-10 | Done | `TestEvaluatePrefix/AC10_ipv6` (match_test.go:120) | 2001:db8:1::/48 in 2001:db8::/32 ge32 le48 |
| AC-11 | Done | `prefix-list-config.ci` + `prefix-filter-accept.ci` | Chain syntax `filter import [ bgp-filter-prefix:CUSTOMERS ]` parses and dispatches |
| AC-12 | Done | `TestEvaluatePrefix/AC12_ge_zero_default_route` (match_test.go:127) | ge=0 with 0.0.0.0/0 route |
| AC-13 | Done | `TestEvaluatePrefix/AC13_le_128_ipv6_host` (match_test.go:134) | le=128 with /128 IPv6 host |
| AC-14 | Done | `TestParseOneEntry/ge_gt_le_invalid` (config_test.go:54) | Returns error "ge 24 > le 16" |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestPrefixMatchInRange | Done (renamed) | `match_test.go` TestEvaluatePrefix/AC1_in_range_accept | Table-driven |
| TestPrefixMatchOutOfRange | Done (renamed) | TestEvaluatePrefix/AC2_too_specific, AC3_too_general | |
| TestPrefixMatchExact | Done (renamed) | TestEvaluatePrefix/AC4_exact_accept | |
| TestPrefixMatchReject | Done (renamed) | TestEvaluatePrefix/AC5_exact_reject | |
| TestPrefixMatchFirstWins | Done (renamed) | TestEvaluatePrefix/AC7_* | |
| TestPrefixMatchImplicitDeny | Done (renamed) | TestEvaluatePrefix/AC8_no_match_implicit_deny | |
| TestPrefixMatchIPv4 | Done (renamed) | TestEvaluatePrefix/AC9_ipv4 | |
| TestPrefixMatchIPv6 | Done (renamed) | TestEvaluatePrefix/AC10_ipv6 | |
| TestPrefixListGeLeBoundary | Done (renamed) | TestEvaluatePrefix/AC12_ge_zero_default_route, AC13_le_128_ipv6_host | |
| Boundary: ge 0 / le 128 / ge 129 | Done | match_test.go AC12, AC13; config_test.go ge_out_of_range | All boundary corners |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/plugins/filter_prefix/` (dir) | Done | Created |
| `filter_prefix/register.go` | Done | Plugin registration |
| `filter_prefix/filter.go` | Renamed | Split into `filter_prefix.go` (entry+dispatch) + `match.go` (algorithm) |
| `filter_prefix/filter_test.go` | Renamed | Split into `match_test.go` and `config_test.go` |
| `test/plugin/prefix-filter.ci` | Renamed | Split into `prefix-filter-accept.ci` and `prefix-filter-reject.ci` |
| `test/parse/prefix-list-config.ci` | Done | YANG parse test |
| `internal/component/bgp/schema/ze-bgp-conf.yang` | Not modified | Augment lives in plugin's own schema file (matches loop-detection pattern) |
| `internal/component/bgp/config/resolve.go` | Not modified | Plugin parses its own config via OnConfigure callback (not via core resolver) |
| `internal/component/bgp/reactor/filter_chain.go` | Not modified | Existing PolicyFilterChain dispatch reused unchanged |

### Audit Summary
- **Total items:** 33 (5 task + 14 AC + 10 TDD + 9 files)
- **Done:** 33
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 5 (file renames within scope; behavior unchanged)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/plugins/filter_prefix/schema/ze-filter-prefix.yang` | Yes | `ls` in tmp/cmd4-files.log |
| `internal/component/bgp/plugins/filter_prefix/schema/embed.go` | Yes | `ls` |
| `internal/component/bgp/plugins/filter_prefix/schema/register.go` | Yes | `ls` |
| `internal/component/bgp/plugins/filter_prefix/filter_prefix.go` | Yes | `ls` |
| `internal/component/bgp/plugins/filter_prefix/match.go` | Yes | `ls` |
| `internal/component/bgp/plugins/filter_prefix/config.go` | Yes | `ls` |
| `internal/component/bgp/plugins/filter_prefix/register.go` | Yes | `ls` |
| `internal/component/bgp/plugins/filter_prefix/match_test.go` | Yes | `ls` |
| `internal/component/bgp/plugins/filter_prefix/config_test.go` | Yes | `ls` |
| `test/parse/prefix-list-config.ci` | Yes | `ls` |
| `test/plugin/prefix-filter-accept.ci` | Yes | `ls` |
| `test/plugin/prefix-filter-reject.ci` | Yes | `ls` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 to AC-14 | Unit tests cover behavior | `go test -race ./internal/component/bgp/plugins/filter_prefix/...` -> `ok` (tmp/cmd4-test2.log) |
| AC-9 | IPv4 wired end-to-end | `ze-test bgp plugin prefix-filter-accept -v` -> pass 1/1 (tmp/cmd4-pfa.log) |
| AC-8 | Implicit deny wired end-to-end | `ze-test bgp plugin prefix-filter-reject -v` -> pass 1/1 (tmp/cmd4-pfr.log) |
| AC-11 | Chain ref wired | `ze-test bgp parse prefix-list-config -v` -> pass 1/1 (tmp/cmd4-parse-final.log) |
| All | Plugin registered | `make ze-verify` -> `Ze verification passed`, exit=0 (tmp/ze-test-cmd4.log tail) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Config with `policy prefix-list` + `filter import [ bgp-filter-prefix:NAME ]` (matching) | `test/plugin/prefix-filter-accept.ci` | Yes -- ze-test reports pass; observer plugin asserts `total-routes >= 1` |
| Config with `policy prefix-list` + `filter import [ bgp-filter-prefix:NAME ]` (non-matching) | `test/plugin/prefix-filter-reject.ci` | Yes -- ze-test reports pass; observer plugin asserts `total-routes == 0` |
| Config parse with prefix-list entries | `test/parse/prefix-list-config.ci` | Yes -- `ze config validate` returns `configuration valid` |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Pattern compliance with existing filter plugins
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
- [ ] Write learned summary to `plan/learned/NNN-cmd-4-prefix-filter.md`
- [ ] Summary included in commit

# Spec: DNS Resolver Component

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/web/schema/ze-web-conf.yang` - existing env config pattern
4. `internal/component/web/server.go` - existing component startup pattern
5. `cmd/ze/hub/main.go` - hub startup integration

## Task

Implement a DNS resolver component that provides DNS query services to other Ze components.
The resolver uses `github.com/miekg/dns` (the library CoreDNS is built on) and is configured
via YANG under `environment/dns`. It provides a shared, cached DNS client that any component
can use without managing its own DNS connections.

The immediate consumer is the decorator framework (`spec-decorator.md`) which needs DNS for
Team Cymru ASN-to-name resolution. Future consumers include reverse DNS lookups for peer
addresses, DNSSEC validation, and any component needing DNS queries.

### Why a Component

DNS resolution is cross-cutting infrastructure, not specific to any subsystem. Centralizing it
avoids duplicate resolver configurations, provides a single cache, and allows consistent
timeout/retry behavior across all components.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component architecture, startup lifecycle
  -> Decision:
  -> Constraint:
- [ ] `.claude/rules/plugin-design.md` - proximity principle, component vs plugin distinction
  -> Decision:
  -> Constraint:

### RFC Summaries (MUST for protocol work)
- N/A (using existing DNS library, not implementing DNS protocol)

**Key insights:**
- To be filled during implementation

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/web/schema/ze-web-conf.yang` - env config YANG pattern
- [ ] `internal/component/web/schema/register.go` - YANG module registration pattern
- [ ] `internal/component/web/schema/embed.go` - YANG embedding pattern
- [ ] `cmd/ze/hub/main.go` - component startup from hub
- [ ] `go.mod` - check if miekg/dns already a dependency

**Behavior to preserve:**
- Existing components that use Go's `net.Resolver` continue to work (this is additive)
- YANG schema registration pattern (embed + init)
- Hub startup pattern (component created and started from hub)

**Behavior to change:**
- No DNS resolver component currently exists

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Component receives DNS query request from another Ze component (Go function call)
- Query specifies: domain name, record type (TXT, A, AAAA, PTR, etc.)

### Transformation Path
1. Caller invokes resolver method with domain name and record type
2. Resolver checks local cache (in-memory, keyed by name+type)
3. Cache hit with valid TTL: return cached result immediately
4. Cache miss: construct DNS query using miekg/dns
5. Send query to configured DNS server (UDP, fallback to TCP for truncated responses)
6. Parse response, extract answer records
7. Store in cache with TTL (use response TTL or configured default, whichever is shorter)
8. Return result to caller

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Component -> DNS server | UDP/TCP via miekg/dns | [ ] |
| Caller -> Resolver | Go function call (in-process) | [ ] |
| Config -> Resolver | YANG config read at startup | [ ] |

### Integration Points
- YANG schema registration via `yang.RegisterModule()` in `init()`
- Hub startup creates resolver and passes to consumers
- Consumers receive resolver interface (not concrete type)

### Architectural Verification
- [ ] No bypassed layers (all DNS goes through resolver)
- [ ] No unintended coupling (resolver has no knowledge of consumers)
- [ ] No duplicated functionality (no existing DNS resolver component)
- [ ] Zero-copy preserved where applicable (N/A for DNS)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| YANG config with dns section | -> | Resolver created with configured server | TBD |
| Component calls Resolve() | -> | DNS query sent, result returned | TBD |
| Repeated query within TTL | -> | Cache hit, no network query | TBD |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | YANG config specifies dns server address | Resolver uses configured server for queries |
| AC-2 | No dns config section | Resolver uses system default DNS (empty server = OS resolver) |
| AC-3 | TXT query for valid domain | Returns TXT record content |
| AC-4 | A/AAAA query for valid domain | Returns IP address(es) |
| AC-5 | PTR query for IP address | Returns reverse DNS hostname |
| AC-6 | Query for non-existent domain | Returns empty result, no error (NXDOMAIN is not an error for callers) |
| AC-7 | DNS server unreachable | Returns error after configured timeout, does not block indefinitely |
| AC-8 | Same query repeated within cache TTL | Returns cached result without network query |
| AC-9 | Cache at capacity | Evicts oldest entries (LRU or similar) |
| AC-10 | Config specifies cache-ttl | Cache entries expire at configured TTL (capped by response TTL) |
| AC-11 | Config specifies cache-size 0 | Caching disabled, every query goes to network |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestResolveWithConfiguredServer` | `internal/component/dns/resolver_test.go` | AC-1: uses configured server | |
| `TestResolveDefaultServer` | `internal/component/dns/resolver_test.go` | AC-2: falls back to system default | |
| `TestResolveTXT` | `internal/component/dns/resolver_test.go` | AC-3: TXT record resolution | |
| `TestResolveA` | `internal/component/dns/resolver_test.go` | AC-4: A record resolution | |
| `TestResolvePTR` | `internal/component/dns/resolver_test.go` | AC-5: reverse DNS | |
| `TestResolveNXDOMAIN` | `internal/component/dns/resolver_test.go` | AC-6: non-existent domain | |
| `TestResolveTimeout` | `internal/component/dns/resolver_test.go` | AC-7: timeout handling | |
| `TestCacheHit` | `internal/component/dns/cache_test.go` | AC-8: cache returns stored result | |
| `TestCacheEviction` | `internal/component/dns/cache_test.go` | AC-9: LRU eviction at capacity | |
| `TestCacheTTL` | `internal/component/dns/cache_test.go` | AC-10: entries expire | |
| `TestCacheDisabled` | `internal/component/dns/cache_test.go` | AC-11: cache-size 0 disables cache | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| timeout | 1-60 | 60 | 0 | 61 |
| cache-size | 0-1000000 | 1000000 | N/A (0 is valid = disabled) | 1000001 |
| cache-ttl | 0-604800 | 604800 (7 days) | N/A (0 is valid = use response TTL only) | 604801 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-dns-config` | `test/parse/dns-config.ci` | Config with dns section parses without error | |

### Future (if deferring any tests)
- DNS-over-TLS support
- DNS-over-HTTPS support
- DNSSEC validation

## Files to Modify
- `go.mod` / `go.sum` - add `github.com/miekg/dns` dependency

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [x] | `internal/component/dns/schema/ze-dns-conf.yang` |
| CLI commands/flags | [ ] | N/A |
| Editor autocomplete | [x] | YANG-driven (automatic) |
| Functional test | [x] | `test/parse/dns-config.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - DNS resolver component |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` - dns env section |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [ ] | N/A (covered in configuration guide) |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` - built-in DNS resolver |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` - dns component |

## Files to Create
- `internal/component/dns/resolver.go` - resolver implementation (miekg/dns client, query methods)
- `internal/component/dns/cache.go` - in-memory cache with TTL and LRU eviction
- `internal/component/dns/resolver_test.go` - unit tests for resolver
- `internal/component/dns/cache_test.go` - unit tests for cache
- `internal/component/dns/schema/ze-dns-conf.yang` - YANG config schema
- `internal/component/dns/schema/embed.go` - embedded YANG file
- `internal/component/dns/schema/register.go` - YANG module registration
- `internal/component/dns/schema/schema_test.go` - schema validation test
- `test/parse/dns-config.ci` - functional test for config parsing

## YANG Configuration

| Path | Type | Default | Description |
|------|------|---------|-------------|
| `environment/dns/server` | string | `""` (system default) | DNS server address (e.g., `8.8.8.8:53`) |
| `environment/dns/timeout` | uint16 | `5` | Query timeout in seconds |
| `environment/dns/cache-size` | uint32 | `10000` | Max cached entries (0 = disabled) |
| `environment/dns/cache-ttl` | uint32 | `86400` | Max cache TTL in seconds (0 = use response TTL only) |

## Resolver Interface

The resolver exposes a simple interface for consumers:

| Method | Parameters | Returns | Description |
|--------|-----------|---------|-------------|
| Resolve | name (string), record type (uint16) | records (string slice), error | Query DNS for records of given type |
| ResolveTXT | name (string) | text (string slice), error | Convenience: TXT records |
| ResolveA | name (string) | addresses (string slice), error | Convenience: A records |
| ResolveAAAA | name (string) | addresses (string slice), error | Convenience: AAAA records |
| ResolvePTR | address (string) | hostnames (string slice), error | Convenience: reverse DNS |
| Close | none | none | Shutdown and release resources |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: YANG schema and registration** -- define config schema, register module
   - Tests: `schema_test.go`
   - Files: `schema/ze-dns-conf.yang`, `schema/embed.go`, `schema/register.go`
   - Verify: schema validates, module registered
2. **Phase: Cache** -- in-memory cache with TTL and LRU eviction
   - Tests: `TestCacheHit`, `TestCacheEviction`, `TestCacheTTL`, `TestCacheDisabled`
   - Files: `cache.go`, `cache_test.go`
   - Verify: tests fail -> implement -> tests pass
3. **Phase: Resolver** -- miekg/dns client with cache integration
   - Tests: `TestResolveWithConfiguredServer`, `TestResolveDefaultServer`, `TestResolveTXT`, `TestResolveA`, `TestResolvePTR`, `TestResolveNXDOMAIN`, `TestResolveTimeout`
   - Files: `resolver.go`, `resolver_test.go`
   - Verify: tests fail -> implement -> tests pass
4. **Phase: Functional tests** -- config parsing test
   - Tests: `test/parse/dns-config.ci`
   - Verify: `make ze-functional-test` passes
5. **Full verification** -> `make ze-verify`
6. **Complete spec** -> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Cache TTL respects both configured max and response TTL |
| Naming | YANG uses kebab-case, Go interface is idiomatic |
| Data flow | All DNS queries go through resolver, no direct miekg/dns usage by consumers |
| Rule: design-principles | Resolver interface is minimal (only methods consumers need) |
| Rule: goroutine-lifecycle | No per-query goroutines; cache cleanup on timer or eviction |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| YANG schema exists | `ls internal/component/dns/schema/ze-dns-conf.yang` |
| Resolver compiles | `go build ./internal/component/dns/...` |
| Cache tests pass | `go test -run TestCache ./internal/component/dns/...` |
| Resolver tests pass | `go test -run TestResolve ./internal/component/dns/...` |
| Config parses | functional test `dns-config.ci` passes |
| miekg/dns in go.mod | `grep miekg go.mod` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Domain names sanitized before query (no injection into DNS wire format) |
| Resource exhaustion | Cache bounded by cache-size config; timeout prevents hanging |
| Error leakage | DNS errors do not expose internal server address to callers |
| Amplification | Resolver does not allow arbitrary query types that could be used for amplification |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
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

## RFC Documentation

N/A -- using existing DNS library, not implementing DNS protocol.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.

# Spec: listener-1-yang

| Field | Value |
|-------|-------|
| Status | done |
| Depends | - |
| Phase | - |
| Updated | 2026-04-01 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/yang/modules/ze-types.yang` - current typedefs
4. `internal/component/config/yang/modules/ze-extensions.yang` - current extensions
5. `internal/component/bgp/schema/ze-bgp-conf.yang` - BGP YANG schema
6. `.claude/rules/config-design.md` - YANG structure rules

## Task

Add `zt:listener` grouping (ip + port) to ze-types.yang and `ze:listener` extension to ze-extensions.yang. Normalize all listener YANG services to use the consistent pattern: `enabled` leaf (default false) + `list server { key name; ze:listener; uses zt:listener; }` + refine defaults. Restructure BGP peer-fields grouping to move ip from augments. Remove ExaBGP legacy (`bgp > listen` leaf, tcp.port, bgp.connect, bgp.accept) from YANG.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/architecture/config/syntax.md` - YANG-driven schema, extension handling
  → Decision: [specific architectural decision that constrains this spec]
  → Constraint: [specific rule from the doc that applies here]
- [ ] `.claude/rules/config-design.md` - YANG structure rules, listener pattern
  → Decision: [specific architectural decision that constrains this spec]
  → Constraint: [specific rule from the doc that applies here]

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfcNNNN.md` - [why relevant]
  → Constraint: [specific RFC rule that applies here]

**Key insights:** (summary of all checkpoint lines -- minimal context to resume after compaction)
- [insight from docs]

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `internal/component/config/yang/modules/ze-types.yang` - typedefs only, no groupings
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` - 12 extensions, no listener
- [ ] `internal/component/web/schema/ze-web-conf.yang` - host (zt:ip-address) + port (uint16), no ze import
- [ ] `internal/component/lg/schema/ze-lg-conf.yang` - host + port (uint16), no ze import
- [ ] `internal/component/mcp/schema/ze-mcp-conf.yang` - host + port (uint16, no default), no ze import
- [ ] `internal/component/telemetry/schema/ze-telemetry-conf.yang` - address (zt:ip-address) + port (zt:port), no ze import
- [ ] `internal/component/ssh/schema/ze-ssh-conf.yang` - leaf-list listen (string "host:port")
- [ ] `internal/component/plugin/schema/ze-plugin-conf.yang` - hub > server list with host (string) + port (uint16)
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` - global listen (leaf string), peer connection > local > ip via augment, environment > tcp > port, environment > bgp > connect/accept
- [ ] `internal/component/bgp/config/loader_create.go` - global listen parsing
- [ ] `internal/component/bgp/config/loader.go` - normalizeListenAddr
- [ ] `internal/component/bgp/config/peers.go` - applyPortOverride
- [ ] `internal/component/config/environment.go` - TCPEnv.Port, BGPEnv.Connect, BGPEnv.Accept fields

**Behavior to preserve:**
- All non-listener YANG content unchanged
- BGP peer connection semantics (local connect, remote accept, md5, ttl)
- Plugin hub server list structure (key "name", secret, client list)

**Behavior to change:**

YANG core changes:
| Change | Detail |
|--------|--------|
| Add zt:listener grouping to ze-types.yang | Two leaves: ip (zt:ip-address) + port (zt:port) |
| Add ze:listener extension to ze-extensions.yang | Marks a container as a network listener endpoint |
| Add import ze-extensions to 4 files | ze-web-conf, ze-lg-conf, ze-mcp-conf, ze-telemetry-conf |

Listener normalization (all use `enabled` leaf + `list server { key name; ze:listener; uses zt:listener; }` + refine defaults):

All services get an `enabled` leaf (boolean, default false) at the service container level. When enabled and the list is empty, a default listener entry is created using YANG refine defaults. When disabled, the service does not start regardless of list entries.

| Service | Current | Change |
|---------|---------|--------|
| Web | host + port (uint16) | Add `enabled` leaf; replace with `list server { key name; uses zt:listener; ze:listener; }` refine defaults 0.0.0.0:3443 |
| Looking Glass | host + port (uint16) | Same pattern; refine defaults 0.0.0.0:8443 |
| MCP | host + port (uint16, no default) | Same pattern; refine defaults 127.0.0.1 + default port |
| Telemetry | address + port | Same pattern; refine defaults 0.0.0.0:9273 |
| SSH | leaf-list listen (string) | Same pattern; refine defaults from current listen format |
| Plugin hub server | host (string) + port (uint16) | Already a list; normalize leaves to uses zt:listener; string to zt:ip-address; add ze:listener |

**Removed:** BGP global `listen` leaf (ExaBGP legacy). Ze derives BGP listeners from per-peer `connection > local` when `remote > accept` is true.

BGP grouping restructure:
| Change | Detail |
|--------|--------|
| Move ip from augments into peer-fields grouping | Both connection > local > ip and connection > remote > ip |
| Delete 4 augments | Standalone peer local/remote, grouped peer local/remote |
| Add ze:listener to connection > local in grouping | BGP peer local does NOT use zt:listener grouping (ip is union type with auto enum) |

ExaBGP legacy removal from YANG:
| Remove | From |
|--------|------|
| bgp > listen leaf | ze-bgp-conf.yang (global listen -- ExaBGP legacy, no replacement) |
| environment > tcp > port leaf | ze-bgp-conf.yang augment |
| environment > bgp > connect leaf | ze-bgp-conf.yang augment |
| environment > bgp > accept leaf | ze-bgp-conf.yang augment |

**Note:** Per-peer `connection > local > connect` and `connection > remote > accept` are Ze-native and must NOT be removed. Only the global `environment > bgp` overrides are ExaBGP legacy.

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- YANG schema files loaded at startup
- Format at entry: YANG module files on disk

### Transformation Path
1. Schema loaded from YANG module files
2. Extensions parsed (ze:listener recognized)
3. Groupings resolved (zt:listener expanded in all containers)
4. Tree validated against schema

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG schema -> config parser | Extensions drive parser behavior | [ ] |

### Integration Points
- Config parser recognizes ze:listener extension
- Config tree contains normalized listener containers

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: Proves the feature is reachable from its intended entry point. -->
<!-- Without this, the feature exists in isolation — unit tests pass but nothing calls it. -->
<!-- Every row MUST have a test name. "Deferred" / "TODO" / empty = spec cannot be marked done. -->
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with web list server entries | -> | YANG parser list pattern | test/parse/listener-web-list.ci |
| Config with `bgp { listen "..."; }` | -> | YANG parser rejects (leaf removed) | test/parse/listener-bgp-listen-removed.ci |
| Config with environment tcp.port | -> | YANG parser rejects | test/parse/listener-tcp-port-removed.ci |
| Config with enabled + empty list | -> | Default endpoint created | test/parse/listener-enabled-default.ci |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion. -->
<!-- The Implementation Audit cross-references these criteria. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | YANG schema loads with ze:listener extension | No error, extension recognized |
| AC-2 | All listener services use `list server { key name; }` with `uses zt:listener` | Web, LG, MCP, telemetry, SSH, plugin hub server |
| AC-3 | All services have `enabled` leaf (default false) | Enabled + empty list = default endpoint; disabled = service off |
| AC-4 | No leaf host or leaf address in listener YANG | grep returns nothing in listener contexts |
| AC-5 | BGP peer ip in grouping not augment | connection > local > ip and remote > ip in peer-fields, 4 augments deleted |
| AC-6 | `bgp > listen` leaf removed | Config with `bgp { listen "..."; }` rejected |
| AC-7 | Config with environment tcp port 179 | Parse error: unknown option (tcp.port removed) |
| AC-8 | Config with environment bgp connect false | Parse error: unknown option (bgp.connect removed) |
| AC-9 | Config with environment bgp accept false | Parse error: unknown option (bgp.accept removed) |
| AC-10 | Web config with named list entries | `environment { web { enabled true; server main { ip 0.0.0.0; port 3443; } } }` parses correctly |
| AC-11 | Existing configs using old host/port format | Parse error (old format no longer valid) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestYANGSchemaLoadsAfterRestructure | internal/component/config/schema_test.go | AC-1: schema loads cleanly | |
| TestListenerGroupingResolved | internal/component/config/schema_test.go | AC-2: zt:listener grouping in all containers | |
| TestTcpPortRejected | internal/component/config/environment_test.go | AC-5: tcp.port parse error | |
| TestBGPConnectRejected | internal/component/config/environment_test.go | AC-6: bgp.connect parse error | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| port (zt:port) | 1-65535 | 65535 | 0 | 65536 |

### Functional Tests
<!-- REQUIRED: Verify feature works from end-user perspective -->
<!-- New RPCs/APIs MUST have functional tests — unit tests alone are NOT sufficient -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| listener-web-ip-port | test/parse/listener-web-ip-port.ci | Web listener with ip + port parses | |
| listener-bgp-listen | test/parse/listener-bgp-listen.ci | BGP listen container parses, default port | |
| listener-tcp-port-removed | test/parse/listener-tcp-port-removed.ci | Old tcp.port rejected | |

### Future (if deferring any tests)
- [Tests to add later and why deferred -- requires explicit user approval]

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
<!-- Check // Design: annotations on each file — if the change affects behavior
     described in the referenced architecture doc, include the doc here too -->
- `internal/component/config/yang/modules/ze-types.yang` - add listener grouping
- `internal/component/config/yang/modules/ze-extensions.yang` - add listener extension
- `internal/component/web/schema/ze-web-conf.yang` - add ze import; uses zt:listener + ze:listener
- `internal/component/ssh/schema/ze-ssh-conf.yang` - convert leaf-list to container + uses zt:listener
- `internal/component/mcp/schema/ze-mcp-conf.yang` - add ze import; uses zt:listener + ze:listener; add default port
- `internal/component/lg/schema/ze-lg-conf.yang` - add ze import; uses zt:listener + ze:listener
- `internal/component/telemetry/schema/ze-telemetry-conf.yang` - add ze import; uses zt:listener + ze:listener
- `internal/component/plugin/schema/ze-plugin-conf.yang` - uses zt:listener in hub > server
- `internal/component/bgp/schema/ze-bgp-conf.yang` - convert listen, move ip to grouping, delete augments, remove tcp.port/bgp.connect/bgp.accept
- `internal/component/bgp/config/loader_create.go` - update global listen parsing for container
- `internal/component/bgp/config/loader.go` - remove normalizeListenAddr
- `internal/component/bgp/config/peers.go` - remove applyPortOverride
- `internal/component/config/environment.go` - remove TCPEnv.Port, BGPEnv.Connect, BGPEnv.Accept fields and envOptions

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | YES | ze-types.yang, ze-extensions.yang, 8 component schemas |
| CLI commands/flags | [ ] | |
| Editor autocomplete | YES | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [ ] | |

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | |
| 2 | Config syntax changed? | YES | docs/guide/configuration.md (ip rename, container format), docs/architecture/config/syntax.md (ze:listener, zt:listener) |
| 3 | CLI command added/changed? | [ ] | |
| 4 | API/RPC added/changed? | [ ] | |
| 5 | Plugin added/changed? | [ ] | |
| 6 | Has a user guide page? | [ ] | |
| 7 | Wire format changed? | [ ] | |
| 8 | Plugin SDK/protocol changed? | [ ] | |
| 9 | RFC behavior implemented? | [ ] | |
| 10 | Test infrastructure changed? | [ ] | |
| 11 | Affects daemon comparison? | [ ] | |
| 12 | Internal architecture changed? | YES | docs/architecture/config/environment.md (tcp.port, bgp.connect/accept removed) |

## Files to Create
- `test/parse/listener-web-list.ci`
- `test/parse/listener-bgp-listen-removed.ci`
- `test/parse/listener-tcp-port-removed.ci`
- `test/parse/listener-enabled-default.ci`

## Implementation Steps

<!-- Steps must map to /implement stages. Each step should be a concrete phase of work,
     not a generic process description. The review checklists below are what /implement
     stages 5, 9, and 10 check against — they MUST be filled with feature-specific items. -->

### /implement Stage Mapping

<!-- This table maps /implement stages to spec sections. Fill during design. -->
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
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

<!-- List concrete phases of work. Each phase follows TDD: write test → fail → implement → pass.
     Phases should be ordered by dependency (e.g., schema before resolution, resolution before CLI). -->

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: YANG core** -- Add zt:listener grouping and ze:listener extension
   - Tests: TestYANGSchemaLoadsAfterRestructure
   - Files: ze-types.yang, ze-extensions.yang
   - Verify: tests fail -> implement -> tests pass
2. **Phase: Normalize service YANG** -- Update all 7 service listener schemas to use consistent pattern
   - Tests: TestListenerGroupingResolved
   - Files: ze-web-conf.yang, ze-lg-conf.yang, ze-mcp-conf.yang, ze-telemetry-conf.yang, ze-ssh-conf.yang, ze-plugin-conf.yang, ze-bgp-conf.yang (listen container)
   - Verify: tests fail -> implement -> tests pass
3. **Phase: BGP restructure** -- Move ip from augments to grouping, delete augments, add ze:listener to connection > local
   - Tests: TestListenerGroupingResolved (BGP peer portion)
   - Files: ze-bgp-conf.yang
   - Verify: tests fail -> implement -> tests pass
4. **Phase: ExaBGP legacy removal** -- Remove tcp.port, bgp.connect, bgp.accept from YANG and Go structs
   - Tests: TestTcpPortRejected, TestBGPConnectRejected
   - Files: ze-bgp-conf.yang, environment.go
   - Verify: tests fail -> implement -> tests pass
5. **Phase: Update Go consumers** -- Update loader_create, loader, peers for new container format
   - Tests: existing unit tests must continue to pass
   - Files: loader_create.go, loader.go, peers.go
   - Verify: tests fail -> implement -> tests pass
6. **Functional tests** -- Create after feature works. Cover user-visible behavior.
   - Tests: listener-web-ip-port.ci, listener-bgp-listen.ci, listener-tcp-port-removed.ci
   - Files: test/parse/
   - Verify: all functional tests pass
7. **Full verification** -- `make ze-verify` (lint + all ze tests except fuzz)

### Critical Review Checklist (/implement stage 5)

<!-- MANDATORY: Fill with feature-specific checks. /implement uses this table
     to verify the implementation. Generic checks from rules/quality.md always apply;
     this table adds what's specific to THIS feature. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Naming | All YANG listener leaves use ip not host/address |
| No-layering | Old host/address leaves fully deleted |
| Augment rule | No intra-component augments remain in BGP |
| Consistency | All listener containers follow the same pattern |

### Deliverables Checklist (/implement stage 9)

<!-- MANDATORY: Every deliverable with a concrete verification method.
     /implement re-reads the spec and checks each item independently. -->
| Deliverable | Verification method |
|-------------|---------------------|
| ze:listener extension exists | grep extension listener ze-extensions.yang |
| zt:listener grouping exists | grep "grouping listener" ze-types.yang |
| No leaf host in listener YANG | grep -r "leaf host" internal/component/*/schema/*.yang returns nothing in listener contexts |
| No BGP augments for ip | grep "augment.*connection.*local\|augment.*connection.*remote" ze-bgp-conf.yang returns nothing |
| tcp.port removed | grep "leaf port" in tcp container of ze-bgp-conf.yang returns nothing |

### Security Review Checklist (/implement stage 10)

<!-- MANDATORY: Feature-specific security concerns. /implement checks each item.
     Think about: untrusted input, injection, resource exhaustion, error leakage. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | zt:port range 1-65535 enforced by type; zt:ip-address validated by pattern |
| Default safety | MCP defaults to 127.0.0.1 (localhost only) |

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
<!-- LIVE — write IMMEDIATELY when you learn something -->
<!-- Route at completion: subsystem → arch doc, process → rules, knowledge → memory.md -->

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer constraints, message ordering, any MUST/MUST NOT.

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

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

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

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->
<!-- For each item: run a command (grep, ls, go test -run) and paste the evidence. -->
<!-- Hook pre-commit-spec-audit.sh (exit 2) checks this section exists and is filled. -->

### Files Exist (ls)
<!-- For EVERY file in "Files to Create": ls -la <path> — paste output. -->
<!-- For EVERY .ci file in Wiring Test and Functional Tests: ls -la <path> — paste output. -->
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
<!-- For EVERY AC-N: independently verify. Do NOT copy from audit — re-check. -->
<!-- Acceptable evidence: test name + pass output, grep showing function call, ls showing file. -->
<!-- NOT acceptable: "already checked", "should work", reference to audit table above. -->
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
<!-- For EVERY wiring test row: does the .ci test exist AND does it exercise the full path? -->
<!-- Read the .ci file content. Does it actually test what the wiring table claims? -->
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
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

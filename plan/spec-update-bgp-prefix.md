# Spec: update-bgp-prefix

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/6 |
| Updated | 2026-06-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/plugin-self-containment.md` - command ownership
4. `ai/rules/cli-grammar.md` - selector grammar
5. `internal/component/bgp/plugins/cmd/peer/peer.go` - peer command registration
6. `internal/component/resolve/irr/client.go` - IRR whois client
7. `internal/component/resolve/peeringdb/client.go` - PeeringDB client
8. `internal/component/cli/editor_commands.go` - Editor.SetValue
9. `internal/component/cli/editor_draft.go` - Editor.SaveDraft

## Task

Restore the `update bgp peer prefix` command that was removed in commit `6c19edc32`.
The removal was collateral damage: the commit correctly deleted `set bgp peer with`
and `set bgp peer save` (config-mutation commands that bypassed the editor), but
`update bgp peer prefix` is a data-refresh command that belongs under the `update`
verb. Without it, operators have no way to refresh max-prefix limits from PeeringDB,
and the login warning for stale prefix data (`prefix-stale` in `show bgp peer detail`)
fires with no actionable command.

The old implementation called `ed.Save()` which no longer exists. The Editor now
uses `SaveDraft()`/`CommitSession()`. The restored command must adapt to this flow.

### Scope boundary

This spec covers ONLY the PeeringDB max-prefix refresh command. The broader
IRR-based prefix-list filter generation (`bgp-filter-irr` plugin from deferrals)
is a separate, future spec (`spec-filter-irr`). The two features share the IRR
client and `update` verb root but are otherwise independent.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/plugin-self-containment.md` - command ownership rules
  -> Constraint: the handler and YANG schema belong in the BGP peer cmd package, not central `update`
- [ ] `ai/rules/cli-grammar.md` - selector grammar
  -> Constraint: `update bgp peer <selector> prefix` uses an untyped positional selector; the old grammar predates the typed-selector rule; assess whether to adopt the typed form now
- [ ] `ai/rules/config-surface.md` - YANG vs env var for PeeringDB settings
  -> Decision: PeeringDB URL and margin are already YANG config under `system { peeringdb { } }`, no change needed
- [ ] `docs/architecture/api/commands.md` - command dispatch and verb registration
  -> Constraint: RPC registered via `pluginserver.RegisterRPCs` in init(), YANG schema in owner package

### Source Files (MANDATORY before implementation)
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` (675L) - current peer command registration; `filterPeersByArgs` and `filterPeersBySelectorValue` still exist
  -> Constraint: `filterPeersBySelector` was removed; use `filterPeersBySelectorValue(ctx, ctx.PeerSelector())` or `filterPeersByArgs(ctx, nil)`
- [ ] `internal/component/cli/editor_commands.go` - `SetValue(path, key, value)` still exists
  -> Constraint: sets value in the pending-changes tree
- [ ] `internal/component/cli/editor_draft.go` - `SaveDraft()` persists pending changes to a draft file
  -> Decision: command will call `SetValue` + `SaveDraft` instead of the old `Save()`; operator commits via `config commit`
- [ ] `internal/component/cli/editor.go` - `NewEditor(configPath)` constructor
- [ ] `internal/component/resolve/peeringdb/client.go` - `LookupASN`, `ApplyMargin`, `Suspicious` all still exist
- [ ] `internal/component/config/system/system.go` - `ExtractSystemConfig` still returns `PeeringDBURL` and `PeeringDBMargin`
- [ ] `internal/component/plugin/server/server.go:205` - `ConfigPath()` still available
- [ ] `internal/component/cmd/update/yang/ze-cli-update-api.yang` - still declares `rpc bgp-peer-prefix` (orphaned)
- [ ] `internal/component/cmd/update/yang/ze-cli-update-cmd.yang` - bare anchor, `update bgp` subtree was removed
- [ ] `internal/test/mock/peeringdb/peeringdb.go` - deterministic fake server; prefix counts = ASN value

**Key insights:**
- All runtime dependencies survived the removal: Editor, PeeringDB client, system config, ConfigPath, peer filtering
- The API YANG (`rpc bgp-peer-prefix`) was never removed, just the command YANG and handler
- The mock PeeringDB server (`ze-test peeringdb`) still exists for functional testing
- The `prefix-updated` and `prefix-stale` fields in `show bgp peer detail` still exist (peer.go:317-324) but have no command to populate them
- Login warning for stale prefix data still fires (docs/features/cli-commands.md:79) with no fix path

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` - peer commands: list, detail, teardown, pause, resume, flush, history, remove. No `update bgp peer prefix`
- [ ] `internal/component/cmd/update/yang/ze-cli-update-cmd.yang` - bare `container update` anchor; `update bgp` subtree removed in revision 2026-06-03
- [ ] `internal/component/cmd/update/yang/ze-cli-update-api.yang` - orphaned `rpc bgp-peer-prefix` declaration
- [ ] `internal/component/resolve/peeringdb/client.go` - `LookupASN(ctx, asn)` returns `PrefixCounts{IPv4, IPv6}`; `ApplyMargin(count, margin)` applies percentage; `Suspicious()` returns true when both families are zero
- [ ] `internal/component/config/system/system.go` - `ExtractSystemConfig(tree)` returns `SystemConfig{PeeringDBURL, PeeringDBMargin}`; defaults: URL `https://www.peeringdb.com`, margin 10

**Behavior to preserve:**
- `show bgp peer <sel> detail` displays `prefix-updated` timestamp and `prefix-stale` warning (peer.go:317-324)
- Login warning fires for peers with `prefix-updated` older than 6 months
- PeeringDB settings live under `system { peeringdb { url; margin } }` in YANG config
- Rate limiting between PeeringDB queries (1 req/sec in old code)
- Suspicious-data guard (zero prefixes from PeeringDB = skip, do not zero out config)
- Per-peer result reporting (updated/skipped/error with details)
- Peer selector supports: `*` (all), IP address, peer name, `as<N>` (by ASN)

**Behavior to change:**
- Old: `ed.Save()` wrote config file directly. New: `ed.SetValue()` + `ed.SaveDraft()` writes to draft; operator commits via `config commit`
- Old: response message said "run 'ze config commit' to apply". New: message says "run 'config commit' to apply" (matches CLI grammar)

## Data Flow (MANDATORY)

### Entry Point
- CLI or API: `update bgp peer <selector> prefix`
- Dispatcher extracts peer selector, routes to `HandleBgpPeerPrefixUpdate`

### Transformation Path
1. Handler opens `cli.Editor` via `ConfigPath()`
2. Reads `system { peeringdb { url; margin } }` via `ExtractSystemConfig(ed.Tree())`
3. Filters peers via `filterPeersBySelectorValue(ctx, ctx.PeerSelector())`
4. For each peer: queries PeeringDB `LookupASN(ctx, peer.PeerAS)` with rate limiting
5. Applies margin via `ApplyMargin(count, margin)`, guards against suspicious data
6. Writes to editor: `ed.SetValue(["bgp","peer",name,"session","family","ipv4/unicast","prefix"], "maximum", value)`
7. Writes timestamp: `ed.SetValue(..., "updated", today)`
8. After all peers: `ed.SaveDraft()` persists to draft file
9. Returns JSON result with per-peer status

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI/API -> Handler | RPC dispatch via `ze-update:bgp-peer-prefix` | [ ] |
| Handler -> PeeringDB | HTTP GET to PeeringDB API | [ ] |
| Handler -> Config | `cli.Editor.SetValue` + `SaveDraft` | [ ] |

### Integration Points
- `pluginserver.RegisterRPCs` in peer package `init()` - wires RPC
- `pluginserver.RequireReactor` - validates reactor is available
- `cli.NewEditor(configPath)` - opens config for editing
- `system.ExtractSystemConfig(tree)` - reads PeeringDB settings
- `peeringdb.NewPeeringDB(url)` - creates PeeringDB client
- BGP peer command YANG schema - declares CLI path

### Architectural Verification
- [ ] No bypassed layers (uses Editor draft flow, not direct file write)
- [ ] No unintended coupling (handler in BGP peer cmd package, owns its surface)
- [ ] No duplicated functionality (restores removed feature, not new duplicate)
- [ ] Zero-copy preserved where applicable (N/A, config text operations)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `cli.NewEditor(configPath)` can be called from an RPC handler without a CLI session | Old code did this; `NewEditor` takes a file path | Would need `NewEditorWithStorage` or session-aware constructor | grep `NewEditor` callers outside CLI session | confirmed |
| A-2 | `SaveDraft()` works without an active CLI session/user | `SaveDraft` writes to a per-session draft file keyed by session ID | Draft might require a session ID that the RPC handler does not have | Read `SaveDraft` implementation | broken |
| A-3 | `SetValue` path syntax matches current YANG tree for `bgp/peer/<name>/session/family/<fam>/prefix/maximum` | Old code used this path; YANG structure may have changed | Path would silently fail or error | Check current YANG schema for bgp peer prefix | confirmed |
| A-4 | PeeringDB mock server (`ze-test peeringdb`) still produces deterministic output matching test expectations | Mock derives counts from ASN; functional test expects specific values | Test would fail with wrong expected values | Run mock server, query ASN 65001 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | `SaveDraft` requires a CLI session context the RPC handler does not have | `SaveDraft` errors when called from handler | Fall back to direct `SetValue` + `Save` on the editor tree, or write results to a staging area the operator imports |
| R-2 | YANG `prefix/maximum` path changed since the old code was written | `SetValue` returns error or silently sets wrong leaf | Validate path against current YANG before implementing |
| R-3 | Old CLI grammar `update bgp peer <selector> prefix` uses untyped positional selector, violating `cli-grammar.md` | Review flag during implementation | Assess cost of adopting `update bgp peer name <name> prefix` vs keeping legacy grammar for backwards compatibility |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| CLI `update bgp peer * prefix` | -> | `HandleBgpPeerPrefixUpdate` in `internal/component/bgp/plugins/cmd/peer/prefix_update.go` | `test/plugin/api-peer-prefix-update.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `update bgp peer * prefix` with reachable PeeringDB mock | Each peer's max-prefix updated in config draft with margin applied |
| AC-2 | Peer with ASN 0 (no remote AS configured) | Peer skipped with status "skipped", error "no remote ASN configured" |
| AC-3 | PeeringDB returns zero prefixes for a peer (suspicious) | Peer skipped, existing max-prefix preserved |
| AC-4 | Multiple peers, rate limiting | Queries spaced at least 1 second apart |
| AC-5 | Custom PeeringDB URL via `system { peeringdb { url } }` | Custom URL used for queries |
| AC-6 | Custom margin via `system { peeringdb { margin 20 } }` | 20% margin applied to prefix counts |
| AC-7 | `update bgp peer <name> prefix` with single peer selector | Only the selected peer updated |
| AC-8 | `prefix-updated` timestamp set to today's date | `show bgp peer detail` shows fresh timestamp, `prefix-stale` cleared |
| AC-9 | Config changes persisted via `SaveDraft` (or appropriate editor API) | Changes visible in `show configuration draft` or config diff; operator applies via `config commit` |
| AC-10 | YANG command tree declares `update bgp peer prefix` | CLI tab-completion discovers the command |
| AC-11 | Removing the BGP peer cmd package removes the `update bgp peer` subtree | Plugin self-containment test passes |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPrefixUpdateStopsOnContextCancel` | `internal/component/bgp/plugins/cmd/peer/prefix_update_test.go` | Context cancellation stops rate-limit wait and lookup | |
| `TestPrefixUpdateLookupUsesCallerContext` | same | Caller context propagated to PeeringDB client | |
| `TestPrefixUpdateSuspiciousSkipped` | same | Zero-prefix PeeringDB response causes skip | |
| `TestPrefixUpdateMarginApplied` | same | Margin correctly applied to prefix counts | |
| `TestPrefixUpdateValidatePeeringDBURL` | same | Rejects non-http/https URLs | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| margin | 0-100 | 100 | N/A (0 = no margin, valid) | 101 (handled by YANG range) |
| ASN | 1-4294967295 | 4294967295 | 0 (skipped as "no ASN") | N/A (uint32) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `api-peer-prefix-update` | `test/plugin/api-peer-prefix-update.ci` | Operator runs `update bgp peer * prefix`, PeeringDB queried, config updated with margin | |

### Interop Tests
N/A. This is a management-plane feature (PeeringDB HTTP query + config edit), not a protocol feature.

## Files to Modify

- `internal/component/bgp/plugins/cmd/peer/peer.go` - re-register `ze-update:bgp-peer-prefix` RPC
- `internal/component/cmd/update/yang/ze-cli-update-cmd.yang` - remains bare anchor (BGP peer owns the subtree per self-containment)

## Files to Create

- `internal/component/bgp/plugins/cmd/peer/prefix_update.go` - handler (adapted from git history, using `SaveDraft` instead of `Save`)
- `internal/component/bgp/plugins/cmd/peer/prefix_update_test.go` - unit tests
- `internal/component/bgp/plugins/cmd/peer/yang/ze-update-bgp-peer-cmd.yang` - YANG command schema owned by BGP peer package (container merge onto `update` root)
- `test/plugin/api-peer-prefix-update.ci` - functional test (restored from git history, adapted if needed)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [x] | `internal/component/bgp/plugins/cmd/peer/yang/ze-update-bgp-peer-cmd.yang` |
| YANG validation constraints | [ ] | N/A, no new config leaves (PeeringDB settings already exist) |
| CLI commands/flags | [x] | Via YANG command schema |
| CLI grammar (action before identifier) | [x] | Assess typed selector vs legacy grammar |
| Functional test for new RPC/API | [x] | `test/plugin/api-peer-prefix-update.ci` |
| Pipe completeness | [ ] | Command returns JSON result, no pipe filtering needed |
| Doctor check for runtime dependencies | [ ] | PeeringDB is an optional external service, not a runtime dependency |
| Prometheus counters/metrics | [ ] | Not needed for a manual-trigger command |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features/cli-commands.md` (restore update bgp peer prefix mention) |
| 2 | Config syntax changed? | [ ] | No new config |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` (restore update bgp peer prefix row) |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` (restore update verb entry) |
| 5 | Plugin added/changed? | [ ] | No new plugin |
| 6 | Has a user guide page? | [ ] | No |
| 7 | Wire format changed? | [ ] | No |
| 8 | Plugin SDK/protocol changed? | [ ] | No |
| 9 | RFC behavior implemented? | [ ] | No |
| 10 | Test infrastructure changed? | [ ] | No |
| 11 | Affects daemon comparison? | [ ] | No |
| 12 | Internal architecture changed? | [ ] | No |
| 13 | Route metadata keys added/changed? | [ ] | No |
| 14 | Prometheus counters added/changed? | [ ] | No |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | [x] | `docs/features/plugins.md` if RPC listed there |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] | Grep during implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | [x] | `docs/features/cli-commands.md:79` mentions stale prefix warning with no fix command |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-14. | Standard flow per planning.md |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Validate assumptions** -- resolve A-1 through A-4 before writing code
   - Read `SaveDraft` implementation to determine session requirements
   - Check YANG tree for `bgp/peer/*/session/family/*/prefix/maximum` path
   - Test mock PeeringDB output for ASN 65001
   - Verify: all assumptions confirmed or design adjusted

2. **Phase: Wiring (MANDATORY FIRST)** -- register entry point, write failing wiring test
   - Create `prefix_update.go` with stub handler returning "not implemented"
   - Register `ze-update:bgp-peer-prefix` RPC in `peer.go` init()
   - Create YANG command schema `ze-update-bgp-peer-cmd.yang` in peer yang/ package
   - Write self-containment test asserting the YANG node is owned by this package
   - Verify: command appears in CLI completion; handler returns stub error

3. **Phase: Handler logic** -- restore PeeringDB lookup + config update with editor adaptation
   - Tests: `TestPrefixUpdateStopsOnContextCancel`, `TestPrefixUpdateSuspiciousSkipped`, `TestPrefixUpdateMarginApplied`, `TestPrefixUpdateValidatePeeringDBURL`
   - Files: `prefix_update.go`, `prefix_update_test.go`
   - Adapt `Save()` call to `SaveDraft()` (or appropriate editor API per A-1/A-2 findings)
   - Verify: unit tests pass

4. **Phase: Functional test** -- restore end-to-end test
   - Restore `test/plugin/api-peer-prefix-update.ci` from git history
   - Adapt expectations if editor flow changed the response format
   - Verify: functional test passes with mock PeeringDB + ze-peer

5. **Phase: Documentation** -- restore removed doc references
   - Update `docs/guide/command-reference.md`, `docs/features/cli-commands.md`, `docs/architecture/api/commands.md`
   - Verify: docs mention `update bgp peer prefix` with correct syntax

6. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Margin calculation matches `peeringdb.ApplyMargin`; rate limiting uses context-aware timer |
| Naming | YANG uses kebab-case; JSON keys use kebab-case |
| Data flow | PeeringDB query -> margin -> SetValue -> SaveDraft; no direct file write |
| CLI grammar | Assess typed selector; document decision |
| Plugin self-containment | Removing BGP peer cmd package removes the `update bgp peer` subtree |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `prefix_update.go` exists | `ls internal/component/bgp/plugins/cmd/peer/prefix_update.go` |
| `prefix_update_test.go` exists | `ls internal/component/bgp/plugins/cmd/peer/prefix_update_test.go` |
| YANG schema exists | `ls internal/component/bgp/plugins/cmd/peer/yang/ze-update-bgp-peer-cmd.yang` |
| Functional test exists | `ls test/plugin/api-peer-prefix-update.ci` |
| RPC registered | `grep 'bgp-peer-prefix' internal/component/bgp/plugins/cmd/peer/peer.go` |
| Self-containment test | `grep 'UpdateBgp' internal/component/bgp/plugins/cmd/peer/yang/*_test.go` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | PeeringDB URL validated (http/https only); AS-SET name not relevant here |
| Rate limiting | PeeringDB queries rate-limited to 1/sec to avoid abuse |
| Context propagation | All HTTP calls use caller context for cancellation |
| Suspicious data guard | Zero-prefix response does not zero out existing config |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| `SaveDraft` requires session context | Investigate `NewEditorWithStorage` or write-through path (R-1) |
| YANG path changed | Update `SetValue` paths to match current schema (R-2) |
| CLI grammar review flags selector | Document decision: typed vs legacy (R-3) |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Use `SaveDraft` instead of direct file write | Direct file write (old approach), write-through | Draft flow matches the current editor model; operator explicitly commits, matching the "config changes go through the editor" principle that motivated the original removal |
| Handler in BGP peer cmd package | Central `update` package, separate `update-cmd` plugin | Plugin self-containment: the handler calls `ctx.Reactor()` for peer data, so it belongs to the BGP peer command package |
| YANG schema via container merge onto `update` root | Augment the central schema | Container merge has no base-module coupling per self-containment rule |
| Restore same RPC wire method `ze-update:bgp-peer-prefix` | New name | API YANG still declares it; no reason to change |

## Known Limitations

- No automatic/scheduled refresh. This is a manual-trigger command. Periodic refresh would be a separate feature (timer-based or cron-like)
- No IRR prefix-list generation. That is `spec-filter-irr` scope
- Does not auto-commit. Operator must run `config commit` after the update

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A-2: SaveDraft works without a session | SaveDraft returns errNoSessionSet when e.session is nil (editor_draft.go:328) | Read SaveDraft implementation | Low: create EditSession("peeringdb", "api") and attach via SetSession before calling SetValue+SaveDraft |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

The removal of `update bgp peer prefix` alongside `set bgp peer with/save` was
an over-generalization: the commit message frames all three as "config-mutation
commands that bypassed the config editor", but `update bgp peer prefix` is a
data-refresh command. It fetches external data and proposes config changes through
the editor. The distinction matters: `set bgp peer with` created peers bypassing
YANG validation; `update bgp peer prefix` enriched existing peers with external
data. One was an editor bypass, the other was an editor collaborator.

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/plugins/cmd/peer/prefix_update.go` | yes | created |
| `internal/component/bgp/plugins/cmd/peer/prefix_update_test.go` | yes | created |
| `test/plugin/api-peer-prefix-update.ci` | yes | created |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `NewEditor(configPath)` takes a path and reads file; no session required for construction |
| A-2 | broken | `SaveDraft()` requires `e.session != nil`; fixed by creating `EditSession("peeringdb", "api")` and calling `SetSession()` |
| A-3 | confirmed | YANG at `ze-bgp-conf.yang:590`: `container prefix { leaf maximum; leaf updated; }` under `session > family > <fam>` |
| A-4 | confirmed | Mock server returns `info_prefixes4=ASN`, functional test passes with expected 71501 for ASN 65001 |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated
- [ ] Critical Review passes
- [ ] Risks & Assumptions: every A-N confirmed or broken

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for numeric inputs
- [ ] Functional tests for end-to-end behavior

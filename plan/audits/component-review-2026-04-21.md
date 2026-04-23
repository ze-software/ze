# Component Review Action Plan -- 2026-04-21

This document turns the initial component inventory and rule scan into an
actionable review plan for a follow-on agent.

This is **not** a line-by-line code audit of the full repository. It is a
triage document built from:

- `.claude/rules/*.md`
- `.claude/INDEX.md`
- `README.md`
- targeted source reads in the highest-risk areas
- repo-structure counts across `cmd/`, `internal/component/`,
  `internal/core/`, `internal/plugins/`, `pkg/`, and `api/proto/`

The goal is to let another agent start reviewing immediately without having
to rebuild the component map or re-derive the review order.

This file was refreshed against the current tree on 2026-04-23. The bundle
structure is still useful, but several concrete leads from the first pass are
now closed and should not drive priority.

## Scope Summary

These counts are the original 2026-04-21 snapshot. This refresh updated the
concrete leads and bundle priorities, not the repository-wide totals.

| Area | Count |
|------|-------|
| Go source files in scoped areas | 2412 |
| Go test files in scoped areas | 929 |
| Functional/editor test specs under `test/` | 914 |
| Runtime components in `internal/component/` | 27 |
| Shared core packages in `internal/core/` | 22 |
| In-process plugin bundles in `internal/plugins/` | 9 |

## Rules That Must Shape The Review

These rules materially change what counts as a bug or design issue in this
repo:

| Rule file | Review implication |
|-----------|--------------------|
| `architecture-summary.md` | Review data flow, not just local code style |
| `data-flow-tracing.md` | Trace entry point -> transform -> boundary -> effect |
| `exact-or-reject.md` | Silent approximation is a bug |
| `buffer-first.md` | Wire paths must stay pooled, bounded, offset-based |
| `goroutine-lifecycle.md` | Per-event goroutines in hot paths are forbidden |
| `plugin-design.md` | YANG, registry, dispatch, and plugin boundaries are structural requirements |
| `config-design.md` | Unknown keys, env-var parity, and config exactness are mandatory |
| `file-modularity.md` | Large multi-concern files are review targets, not just aesthetics |
| `testing.md` | End-to-end evidence matters more than unit-only confidence |
| `implementation-audit.md` | Specs and AC coverage cannot be hand-waved |

## Review Strategy

Do **not** attempt a single pass over the whole tree. Work in bundles that
match the architecture and keep the review output scoped.

### Bundle Order

| Bundle | Priority | Primary paths |
|--------|----------|---------------|
| Control-plane wiring | P0 | `internal/core/*`, `internal/component/{engine,config,plugin,cmd,command}`, `cmd/ze`, `pkg/plugin`, `pkg/ze` |
| BGP core data path | P0 | `internal/component/bgp/*` |
| Operator surfaces | P1 | `internal/component/{cli,web,api,mcp,lg,ssh,telemetry}` |
| Platform and external integrations | P1 | `internal/component/{iface,traffic,firewall,vpp,host,resolve,tacacs,authz,aaa,hub,managed}` |
| Session/tunnel protocols | P1 | `internal/component/{l2tp,ppp}`, `internal/plugins/bfd` |
| Tooling and test infrastructure | P2 | `cmd/{ze-test,ze-chaos,ze-analyse,ze-perf}`, `test/*`, `pkg/{fleet,zefs}` |

## Refresh Status -- 2026-04-23

The work-package structure below is still useful. The concrete leads from the
first draft needed pruning after the code changed.

### Closed Since Initial Draft

| Location | Previous lead | Current status |
|----------|---------------|----------------|
| `internal/component/bgp/config/loader_test.go` | Example-config parser coverage was skipped pending conversion | `TestParseAllConfigFiles` now exercises curated native fixtures and explicitly classifies legacy exclusions |
| `internal/component/plugin/server/command.go` | Subsystem or plugin dispatch dropped caller context via `context.Background()` | Subsystem dispatch now uses `ctx.Context()`, plugin forwarding inherits caller context, and there is regression coverage |
| `internal/component/bgp/plugins/cmd/peer/prefix_update.go` | PeeringDB lookup used `context.TODO()` | Prefix-update now uses `ctx.Context()` and has cancellation coverage |
| `internal/component/web/assets/cli.js` | Rename UI advertised backend support that did not exist | Rename is now implemented server-side in `internal/component/web/handler_config.go` with handler tests |
| `internal/component/tacacs/authenticator.go` | TACACS auth did not receive the caller remote address | Auth now forwards `RemoteAddr` and has dedicated test coverage |
| `cmd/ze/hub/main.go`, `internal/component/hub/*` | Hub or orchestrator mode did not wire external plugins through the runtime | Hub configs now parse `plugin { external ... }`, register subsystems, and reload them |
| `test/plugin/community-strip.ci` | Community strip fixture declared AC-7 blocked because no real forward path was exercised | The fixture now asserts the stripped re-advertised UPDATE on the destination peer and no longer claims blocked evidence |
| `test/plugin/{forward-overflow-two-tier,forward-two-tier-under-load}.ci` | Overflow fixtures only smoke-checked observer state and did not prove the forward path | Both fixtures now force a tiny forward-worker channel, assert ordered destination delivery on the wire, and keep metrics as secondary diagnostics |
| `test/plugin/role-otc-{egress-filter,egress-stamp,export-unknown,ingress-reject,unicast-scope}.ci` | OTC fixtures marked AC verification blocked or TODO because the forward path was not exercised | The fixtures now use second peers plus destination wire assertions or EOR-only suppression checks, so the RFC 9234 claims are backed by real forwarding evidence |
| `internal/component/resolve/cmd/resolve.go` | `ping` and `traceroute` accepted operator args without Ze-side validation | `validateTarget`, `validateSourceIP`, `validateUint` now enforce format before exec; unknown options and trailing keywords rejected per exact-or-reject; 11 new tests cover all validation paths |
| `internal/component/resolve/cmd/resolve.go` | Cymru, PeeringDB, IRR, and shell-out handlers rooted work at `context.Background()` | All 7 handlers now derive context from `ctx.Context()`; callers can cancel long-running lookups cleanly |

## Preliminary Findings

These structural risks remain. Each needs a dedicated review session or spec.

| Type | Location | Finding | Why it matters | First action |
|------|----------|---------|----------------|--------------|
| Structural risk | `internal/component/bgp/*` | The initial scan counted 822 Go files and 339 test files here, and it remains the dominant risk area | Highest complexity, most boundary crossings, most performance-sensitive code | Split review into wire, reactor, RIB, config, plugins, and tests |
| Structural risk | `internal/component/plugin/*` | Plugin lifecycle, registry, IPC, and YANG wiring are central to the product shape even after the earlier context bug was fixed | Many subsystems depend on plugin behavior being structurally correct | Review before subsystem-specific plugin behavior |
| Structural risk | `cmd/ze-test/*` | No Go tests under the test harness entry point (confirmed 2026-04-23: 21 Go files, 0 test files) | Tooling bugs can invalidate test evidence silently | Review harness assumptions and failure reporting |
| Structural risk | large files listed below | All 16 files still exceed the modularity thresholds (refreshed 2026-04-23) | Multi-concern files hide bugs and make follow-up fixes risky | Run a concern audit before making localized edits |

## Large-File Audit Queue

These files exceed the thresholds where `file-modularity.md` expects an
explicit concern check. Line counts refreshed 2026-04-23.

| File | Lines | Review question |
|------|-------|-----------------|
| `internal/component/iface/config.go` | 1487 | Is this still one concern, or multiple backend/config concerns mixed together? |
| `internal/component/bgp/reactor/reactor.go` | 1401 | Does it mix lifecycle, event handling, and forwarding concerns? |
| `internal/component/cli/model_commands.go` | 1298 | Are command behaviors and UI state transitions tangled? |
| `internal/component/bgp/reactor/reactor_api_forward.go` | 1241 | Are API-facing and forwarding concerns coupled too tightly? |
| `internal/component/mcp/streamable.go` | 1240 | Are transport, auth, protocol, and handler concerns split cleanly? |
| `internal/component/bgp/reactor/forward_pool.go` | 1213 | Is the pool still a single coherent concern, or mixed with policy/forward logic? |
| `internal/component/firewall/config.go` | 1167 | Are schema, validation, lowering, and backend assumptions combined? |
| `internal/component/cli/completer.go` | 1088 | Is completion logic a single concern or multiple command grammars fused? |
| `internal/component/l2tp/session_fsm.go` | 1087 | Are state transitions, timers, and I/O handling too interleaved? |
| `internal/component/bgp/reactor/reactor_api.go` | 1077 | Are API entry points and reactor internals coupled? |
| `internal/component/bgp/plugins/rib/rib_commands.go` | 1073 | Are command surface and storage semantics mixed? |
| `internal/component/bgp/plugins/rib/rib.go` | 1065 | Is plugin lifecycle mixed with route storage behavior? |
| `internal/component/cli/editor.go` | 1057 | Is editor state, rendering, and mutation logic too tightly coupled? |
| `internal/component/config/setparser.go` | 1024 | Are parser, command semantics, and syntax migration fused together? |
| `internal/component/plugin/registry/registry.go` | 1020 | Are registration, validation, and lookup concerns still cohesive? |
| `internal/component/bgp/plugins/rib/rib_bestchange.go` | 1003 | Is best-change detection isolated enough to reason about? |

## Bundle Work Packages

Each package below is intended to be executable by another agent as a
self-contained review slice.

### Work Package 1 -- Control-Plane Wiring

**Paths**

- `internal/core/*`
- `internal/component/{engine,config,plugin,cmd,command}`
- `cmd/ze`
- `pkg/plugin/*`
- `pkg/ze/*`

**Review goals**

- Verify subsystem startup/shutdown and dependency wiring
- Check config exactness and env-var parity
- Confirm plugin lifecycle follows the 5-stage contract
- Verify command context propagation across CLI/API/subsystem/plugin boundaries
- Check registry boundaries and avoid direct plugin coupling

**Seed files**

- `internal/component/plugin/server/{command.go,subsystem.go}`
- `internal/component/plugin/registry/registry.go`
- `internal/component/config/*`
- `cmd/ze/main.go`
- `cmd/ze/hub/{main.go,api.go,infra_setup.go}`

**Expected deliverable**

- A findings document with:
  - traced entry paths
  - boundary crossings
  - confirmed issues with file:line
  - missing tests or missing docs
  - follow-up specs required for fixes

### Work Package 2 -- BGP Core Data Path

**Paths**

- `internal/component/bgp/{message,wire,wireu,attribute,capability,context}`
- `internal/component/bgp/{fsm,server,events,transaction}`
- `internal/component/bgp/{reactor,filter,format}`
- `internal/component/bgp/{rib,route,store,attrpool}`
- `internal/component/bgp/{config,configjson,textparse,schema}`
- `internal/component/bgp/plugins/*`

**Review goals**

- Trace `wire -> parse -> reactor -> RIB -> plugin -> wire/API`
- Check buffer-first and zero-copy assumptions in hot paths
- Check reactor goroutine ownership and shutdown
- Check RIB/bestpath correctness and stale handling
- Check config/parser symmetry and syntax migration
- Check built-in plugin YANG/dispatch/wiring completeness
- Audit functional tests for stale blockers, TODOs, or over-claimed evidence

**Seed files**

- `internal/component/bgp/reactor/{reactor.go,forward_pool.go}`
- `test/plugin/community-strip.ci`
- `test/plugin/forward-overflow-two-tier.ci`
- `test/plugin/forward-two-tier-under-load.ci`
- `test/plugin/role-otc-*.ci`

**Expected deliverable**

- A bundle-level report split by:
  - wire/message
  - reactor/forwarding
  - RIB/storage
  - config/text
  - built-in plugins

### Work Package 3 -- Operator Surfaces

**Paths**

- `internal/component/{cli,web,api,mcp,lg,ssh,telemetry}`

**Review goals**

- Check feature parity across CLI, web, API, and MCP
- Verify auth/session/identity boundaries
- Review stream lifetime and cancellation behavior
- Audit large UI/state files for concern drift
- Check user-visible commands/endpoints against docs and tests

**Seed files**

- `internal/component/web/{assets/cli.js,handler_config.go}`
- `internal/component/mcp/streamable.go`
- `internal/component/cli/{editor.go,completer.go,model_commands.go}`
- `cmd/ze/hub/{api.go,infra_setup.go}`

**Expected deliverable**

- Findings grouped by surface:
  - CLI/editor
  - Web
  - API
  - MCP
  - Looking glass / telemetry / SSH

### Work Package 4 -- Platform And External Integrations

**Paths**

- `internal/component/{iface,traffic,firewall,vpp,host,resolve,tacacs,authz,aaa,hub,managed}`
- `internal/plugins/{fib,firewall,iface,ntp,sysctl,sysrib,traffic}`

**Review goals**

- Check exact-or-reject behavior in backend lowering
- Verify external command/API input validation
- Verify request context propagation into resolver clients and OS commands
- Check identity/context propagation for AAA/TACACS/authz
- Review host/VPP/iface/firewall backends for explicit limits and errors
- Confirm hub/managed modes are explicit about unsupported features

**Seed files**

- `internal/component/resolve/cmd/resolve.go`
- `internal/component/iface/config.go`
- `internal/component/firewall/config.go`
- `internal/component/hub/{hub.go,reload.go}`

**Expected deliverable**

- Findings grouped by backend family with explicit operator-facing risk

### Work Package 5 -- Session And Tunnel Protocols

**Paths**

- `internal/component/{l2tp,ppp}`
- `internal/plugins/bfd/*`

**Review goals**

- Check FSM correctness and timer/lifecycle handling
- Review panic boundaries vs runtime errors
- Check packet/auth/session flows for input validation and shutdown correctness
- Confirm tests actually cover state machine behavior, not only helpers

**Seed files**

- `internal/component/l2tp/session_fsm.go`
- `internal/component/ppp/*`
- `internal/plugins/bfd/*`

**Expected deliverable**

- Findings with clear separation between protocol logic bugs and structural modularity debt

### Work Package 6 -- Tooling And Test Infrastructure

**Paths**

- `cmd/{ze-test,ze-chaos,ze-analyse,ze-perf}`
- `test/*`
- `pkg/{fleet,zefs}`

**Review goals**

- Validate the harness that produces confidence for the rest of the repo
- Check for self-declared blocked or partial tests
- Review whether test tools surface failures clearly
- Check persistence/util packages that affect runtime safety

**Seed files**

- `cmd/ze-test/*`
- `test/plugin/*.ci`
- `pkg/zefs/*`

**Expected deliverable**

- Tooling-confidence report plus a list of tests that should not currently be treated as full evidence

## Agent Procedure

A follow-on agent should use this exact loop for each work package.

1. Select one work package only.
2. Trace one real entry path before reading internals.
3. Record:
   - entry point
   - transformation path
   - boundaries crossed
   - tests that exercise the path
4. Review against the rule set above, especially:
   - exact-or-reject
   - buffer-first
   - goroutine lifecycle
   - plugin/config structure
   - file modularity
5. Produce findings ordered by severity:
   - correctness bug
   - structural wiring gap
   - exact-or-reject gap
   - lifecycle/concurrency risk
   - unsupported feature exposed as supported
   - missing test or stale doc
6. For every non-trivial fix, write a child spec instead of patching ad hoc.
7. Keep evidence concrete: file:line plus test names or missing-test statement.

## Output Format For Follow-On Work

If the next agent performs review only, create:

- `plan/audits/component-review-<bundle>-YYYY-MM-DD.md`

If the next agent begins implementation from a finding, create:

- `plan/spec-<bundle>-<issue>.md`

Every follow-on document should include:

- paths reviewed
- current behavior observed
- data flow traced
- findings with file:line
- tests used as evidence
- explicit follow-up recommendation

## Success Criteria

This plan has done its job when another agent can:

- start with one bundle instead of re-mapping the whole repo
- know which rules are binding
- know which source files already contain credible leads
- distinguish confirmed gaps from broader structural risks
- produce focused review output without duplicating this initial triage

## Source Artifacts

The initial triage also exists in scratch form under:

- `tmp/codex-rules-summary.md`
- `tmp/codex-component-inventory.md`
- `tmp/codex-review-plan.md`

This audit file is the committed handoff artifact. The `tmp/` files are
useful background but should not be treated as the durable source of truth.

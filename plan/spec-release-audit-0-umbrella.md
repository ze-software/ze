# Spec: release-audit-0-umbrella

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-release-evidence-gate.md |
| Phase | - |
| Updated | 2026-05-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `ai/rules/planning.md` - spec workflow and completion rules
3. `ai/rules/testing.md` - release test hierarchy and verification ladder
4. `ai/rules/testing.md` - user-facing behavior coverage rule
5. `ai/rules/interop-and-goal-validation.md` - protocol interop and goal evidence rule
6. `ai/rules/completion.md` - exported symbol and entry-point reachability rule
7. `docs/functional-tests.md` - current functional-test release gate coverage
8. `docs/architecture/testing/interop.md` - current BGP interop coverage
9. `plan/spec-release-evidence-gate.md` - separate work to make heavy release evidence a product gate

## Task

Organize a systematic release-readiness audit of Ze before the first user-facing release.
The audit must find logic bugs, incomplete features, bad UX, missing wiring, missing tests,
unsafe failure modes, stale documentation, and release-blocking gaps across every user-visible
surface.

This is an audit umbrella, not an implementation spec for a single feature. Child audit specs
created from this umbrella should review code adversarially, record findings with evidence, and
provide direction for the future fix. They must not change product code as part of the audit.
The goal is to make release risk visible and actionable before users depend on the software.

## Audit Scope Boundary

This spec set documents issues. It does not implement fixes.

Every audit finding should answer:
- What is the issue?
- Why does it matter to a user, operator, peer, or release engineer?
- What evidence proves the issue exists?
- Which area owns the future fix?
- What direction should the future fix take?
- What verification should the future fix provide?

Code changes, test additions, schema changes, documentation updates, and release-gate changes
belong in separate future fix work approved after the audit finding is filed.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/core-design.md` - modular core, plugin lifecycle, BGP data flow
  -> Decision: audit by user-visible entry point and component boundary, not by package alone
  -> Constraint: engine, BGP, plugin server, config provider, and plugins must stay independently wired through registered boundaries
- [ ] `docs/functional-tests.md` - functional release-gate coverage and non-gated suites
  -> Decision: coverage audit must separate `ze-verify` evidence from heavy/manual release evidence
  -> Constraint: static, traffic, VPP, L2TP wire, chaos web, interop, QEMU, perf, deployment, and live checks are not all covered by default `ze-verify`
- [ ] `docs/architecture/testing/interop.md` - Docker interop with FRR, BIRD, GoBGP, ExaBGP wire compatibility
  -> Decision: BGP protocol findings require external-peer evidence when wire-visible
  -> Constraint: interop tests are separate from `ze-verify` because they require Docker
- [ ] `docs/architecture/config/yang-config-design.md` - config pipeline and YANG shape
  -> Decision: config audit must trace file -> tree -> resolved config -> runtime application
  -> Constraint: config validation must reject unsupported or lossy behavior rather than approximating silently
- [ ] `docs/architecture/web-interface.md` - web handler model and HTMX UX
  -> Decision: web audit must include empty states, errors, authorization, HTMX partial updates, and SSE behavior
  -> Constraint: web endpoints are user-visible and need functional coverage through `test/web/`

### Rules

- [ ] `ai/rules/testing.md` - every user-facing behavior needs end-to-end coverage
  -> Constraint: unit tests alone cannot close release audit findings for reachable behavior
- [ ] `ai/rules/interop-and-goal-validation.md` - protocol features need interop and goal evidence
  -> Constraint: BGP behavior closes only with peer-daemon evidence or an explicit non-protocol justification
- [ ] `ai/rules/completion.md` - exported symbols and features must be reachable
  -> Constraint: library-only code and test-only call chains are release blockers when advertised as features
- [ ] `ai/rules/completion.md` - incomplete acceptance criteria cannot be called done
  -> Constraint: every open release blocker remains open until separate fix work proves closure
- [ ] `ai/rules/architecture.md` - predictable ripple effects by file type
  -> Decision: each finding must name likely affected consumers and suggested fix direction
- [ ] `ai/rules/quality.md` - adversarial self-review and proof requirements
  -> Constraint: every audit pass must include adversarial questions and concrete evidence

### Existing Plans

- [ ] `plan/spec-release-evidence-gate.md` - release evidence target for heavy/non-default suites
  -> Decision: this audit depends on that spec for execution of heavyweight release evidence
  -> Constraint: this umbrella should not duplicate Make target work from the evidence-gate spec
- [ ] `plan/spec-rbac-audit.md` - security and audit hardening across surfaces
  -> Decision: security findings related to RBAC/audit should link to this spec when already in scope
  -> Constraint: do not duplicate RBAC implementation scope in release-audit child specs

**Key insights:**
- Ze has enough release surface that an informal code review will miss entire classes of bugs.
- The audit must be feature-surface driven: CLI, config, web, API, BGP wire, plugins, Linux system paths, docs, and operations.
- Existing `ze-verify` is necessary but not sufficient for release evidence.
- The review must produce actionable findings with reproduction and missing-test evidence.
- Each finding should document the regression test or evidence expected from the future fix.

## Current Behavior (MANDATORY)

**Source files and directories read:**
- [ ] `cmd/` - release binaries currently include `ze`, `ze-test`, `ze-perf`, `ze-analyse`, and `ze-chaos`
- [ ] `internal/component/` - ~~35 component directories~~ (stale count; 2026-07-10 recount: 43 directories), including BGP, CLI, config, API, web, LG, SSH, MCP, authz, managed, iface, firewall, L2TP, PPPoE, IPsec, telemetry, and plugin server areas
- [ ] `internal/plugins/` - ~~22 plugin directories~~ (stale count; 2026-07-10 recount: 63 top-level directories), including connected, static, kernel/FIB, firewall, iface, traffic, sysctl, ntp, DHCP server, TFTP server, image server, L2TP helpers, BFD, and policy route areas
- [ ] `test/` - ~~32 test directories~~ (stale count; 2026-07-10 recount: 45 directories), including functional, interop, deployment-like, stress, web, editor, install, firewall, traffic, VPP, L2TP, PPPoE, and managed suites
- [ ] `internal/**/*.yang` glob - YANG config, command, and API surfaces exist across components and plugins
- [ ] `internal/**/register.go` glob - extensive startup registration and schema/plugin wiring surface
- [ ] `test/interop/scenarios/*/check.py` glob - BGP interop scenarios currently include numbered scenarios through `35-srv6-frr`
- [ ] `docs/architecture/testing/interop.md` - documented scenario inventory currently lists scenarios through 32, which appears stale compared with the tree
- [ ] `plan/spec-release-evidence-gate.md` - release evidence gate already planned as separate target work
- [ ] `plan/spec-rbac-audit.md` - RBAC and audit hardening already planned separately

**Behavior to preserve:**
- `make ze-verify` remains the fast pre-commit gate.
- Existing unit, functional, ExaBGP, interop, QEMU, deployment, chaos, fuzz, and perf targets remain independently runnable.
- Ze's registration pattern remains the discovery mechanism for components, plugins, commands, schemas, capabilities, and web/API surfaces.
- Child audits do not rewrite architecture while reviewing it. They file findings with evidence and leave implementation decisions to focused fix specs.
- Existing dirty worktree content outside release-audit files is not modified by this umbrella.

**Audit documentation goal:**
- Release readiness becomes tracked by explicit audit specs and finding evidence, not informal confidence.
- Every user-visible feature must have an audit row showing reachability, tests, negative tests, and goal evidence.
- Every confirmed issue must document suggested fix direction and verification expected from the future fix.
- Release blockers must be visible in one audit table until separate fix work provides verification.

## Data Flow (MANDATORY)

### Entry Point

- Audit inputs enter from registries, command definitions, YANG schemas, web route registration, plugin registration, test inventories, documentation, and release evidence target output.
- User-facing behavior enters Ze through CLI, config files, API/RPC, web routes, SSH, MCP, plugin process commands, BGP wire messages, Linux kernel interfaces, and operator documentation.

### Transformation Path

1. Inventory release surface from `cmd/`, `internal/component/`, `internal/plugins/`, YANG schemas, route/command registrations, web/API handlers, and `test/` directories.
2. Map each surface to an owner child audit and a user entry point.
3. For each surface, verify wiring from entry point to running code.
4. For each behavior, verify unit coverage, functional coverage, negative coverage, and interop or goal evidence where required.
5. Record every bug or gap as a finding with severity, reproduction, expected behavior, actual behavior, missing test, owner, and suggested fix direction.
6. Route each finding to the relevant child audit or future fix owner without changing product code in the audit spec.
7. Record the verification expected before the finding can be considered resolved by future work.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> dispatcher -> component/plugin | YANG command schemas, command registry, dispatcher | Child audit `release-audit-3-config-cli` |
| Config file -> YANG tree -> resolved runtime config | config loader, validators, subsystem reload | Child audit `release-audit-3-config-cli` |
| Web/API/MCP -> auth -> command/config backend | HTTP/gRPC/MCP handlers, auth middleware, API engine | Child audit `release-audit-4-web-lg-api` plus `spec-rbac-audit.md` |
| BGP wire -> FSM/reactor -> plugin event stream | session, capability negotiation, wire parsing, EventDispatcher | Child audit `release-audit-2-bgp-protocol` |
| Plugin registration -> startup -> process/DirectBridge | plugin registry, startup tiers, handshake | Child audit `release-audit-5-plugins-rib` |
| Config/plugin -> Linux kernel/VPP/netlink | iface, FIB, firewall, traffic, VPP, QEMU tests | Child audit `release-audit-6-system-linux` |
| Documentation -> first user workflow | install docs, config examples, command reference | Child audit `release-audit-8-docs-onboarding` |

### Integration Points

- `plan/spec-release-evidence-gate.md` supplies the heavyweight evidence target once implemented.
- `plan/spec-rbac-audit.md` owns known RBAC/audit hardening work.
- `make ze-verify` remains the default proof for ordinary code changes.
- `make ze-interop-test`, `make ze-exabgp-test`, `make ze-qemu-integration-test`, `make ze-chaos-test`, `make ze-fuzz-test`, and deployment/perf targets supply release evidence by risk area.

### Architectural Verification

- [ ] No bypassed layers: findings must name the entry point and actual runtime path.
- [ ] No unintended coupling: suggested fix directions must follow existing component/plugin boundaries.
- [ ] No duplicated functionality: audit tooling should derive inventories from registries and filesystem sources where possible.
- [ ] Zero-copy preserved where applicable: BGP/wire fix directions must respect buffer-first and pool rules.

## Initial Release Surface Inventory

| Surface | Current Source | Initial Owner Audit | Notes |
|---------|----------------|---------------------|-------|
| Binaries | `cmd/` | `release-audit-1-surface-inventory` | `ze`, `ze-test`, `ze-perf`, `ze-analyse`, `ze-chaos` |
| Core components | `internal/component/` | `release-audit-1-surface-inventory` | ~~35 directories observed~~ (2026-07-10: 43) |
| External/internal plugins | `internal/plugins/` | `release-audit-5-plugins-rib` | ~~22 directories observed~~ (2026-07-10: 63 top-level) |
| BGP component plugins | `internal/component/bgp/plugins/` | `release-audit-2-bgp-protocol` and `release-audit-5-plugins-rib` | Includes command, filter, RIB, route-server, GR, RPKI, BMP, watchdog-style surfaces |
| Config schemas | `internal/**/*.yang` | `release-audit-3-config-cli` | Config, command, and API schemas are release-facing |
| Web and LG | `internal/component/web/`, `internal/component/lg/` | `release-audit-4-web-lg-api` | Include auth, HTMX, SSE, empty/error states |
| API and MCP | `internal/component/api/`, `internal/component/mcp/` | `release-audit-4-web-lg-api` | Include REST, gRPC, command dispatch, auth, streaming |
| Linux/system paths | `internal/component/iface/`, `firewall/`, `traffic/`, `vpp/`, plugins under `fib/`, `kernel/`, `ifacenetlink/` | `release-audit-6-system-linux` | Requires QEMU/Linux/deployment evidence where applicable |
| Release tests | `test/` | all child audits | ~~32 test directories observed~~ (2026-07-10: 45) |
| Interop tests | `test/interop/scenarios/` | `release-audit-2-bgp-protocol` | ~~Tree has scenarios through `35-srv6-frr`; docs list through 32~~ (2026-07-10: 101 scenario directories; see Post-wave corrections) |
| Documentation | `docs/`, `README*`, guide pages | `release-audit-8-docs-onboarding` | Must match actual command/config behavior |

## Child Audit Set

| Child Spec | Scope | Primary Question | Output |
|------------|-------|------------------|--------|
| `spec-release-audit-1-surface-inventory.md` | Full inventory of binaries, commands, configs, web routes, APIs, plugins, tests | What can a user touch, and where is it tested? | Release surface matrix |
| `spec-release-audit-2-bgp-protocol.md` | BGP FSM, capabilities, NLRI, attributes, RIB event flow, route refresh, GR, error handling, interop | Does BGP behave correctly with real peers and malformed input? | Protocol finding list plus interop gaps |
| `spec-release-audit-3-config-cli.md` | YANG config, parser, validators, CLI/editor, commit/reload, pipe behavior | Can users configure and operate Ze without confusing or unsafe states? | Config/CLI coverage and bug list |
| `spec-release-audit-4-web-lg-api.md` | Web UI, LG, REST, gRPC, MCP, SSE, auth, empty/error states | Do non-CLI surfaces expose correct behavior safely? | Web/API/LG finding list |
| `spec-release-audit-5-plugins-rib.md` | Plugin registry, startup tiers, RIB, route selection, route reflection, plugin lifecycle | Are plugin and RIB features wired, reliable, and complete? | Plugin/RIB finding list |
| `spec-release-audit-6-system-linux.md` | iface, FIB, firewall, VPP, traffic, PPPoE, L2TP, IPsec, kernel/system integration | Do system-facing features work on Linux and fail safely? | Linux/QEMU/deployment finding list |
| `spec-release-audit-7-resilience-security.md` | Races, shutdown, reload, authz, secrets, crash loops, resource exhaustion, fuzz, chaos | Does Ze survive hostile or degraded conditions? | Resilience/security finding list |
| `spec-release-audit-8-docs-onboarding.md` | Install, quickstart, examples, command reference, feature docs, release notes | Can a new user succeed from docs alone? | Docs and first-run finding list |

## Finding Schema

Every finding must use this schema. A finding without reproduction or missing-test analysis is incomplete.

| Field | Required Content |
|-------|------------------|
| ID | Stable ID, e.g. `RA-BGP-001` |
| Severity | Blocker, Critical, Major, Minor, Note |
| Surface | CLI, config, BGP, web, API, plugin, Linux, docs, test infra |
| File/line | Source location or doc/test path |
| User impact | Observable effect for an operator or peer |
| Reproduction | Command, config, test, peer scenario, or code path |
| Expected | Correct behavior |
| Actual | Observed broken or risky behavior |
| Missing test | Test that should have caught it, or existing test that failed |
| Owner | Child audit or subsystem that should own the future fix |
| Suggested direction | Concrete guidance for the future fix, without changing code in this audit |
| Verification requested | Test, command output, or interop evidence the future fix should provide |

## Release Blocker Policy

| Blocker | Why It Blocks Release | Evidence To Request From Future Fix |
|---------|-----------------------|-------------------------------------|
| `make ze-verify` fails | Baseline gate is broken | Passing `make ze-verify` output |
| User-facing behavior lacks functional coverage | Cannot prove end-to-end behavior | `.ci` or `.et` test through the user entry point |
| Protocol behavior lacks interop evidence | Wire correctness needs external validation | Passing FRR/BIRD/GoBGP/ExaBGP/peer scenario |
| Reachable TODO, FIXME, stub, or not-implemented path | Users may hit unfinished behavior | Future fix removes or implements the path and provides regression evidence |
| Silent approximation in config/protocol/system behavior | Unsafe network behavior is worse than explicit reject | Negative test proving exact-or-reject behavior |
| Dead exported feature code | Feature may exist only in tests | Non-test caller from a running entry point |
| Crash, hang, data race, or goroutine/resource leak | Release-quality failure | Reproducer plus race/stress/chaos evidence |
| Stale or invalid first-user docs | Users fail before reaching product value | Doc update plus command/config validation |
| Authz/security bypass | Unsafe default exposure | Negative test proving deny path and audit/log evidence if applicable |

## Wiring Test (MANDATORY)

This umbrella has no runtime code. Its wiring test is that each child audit maps a release surface to an entry point and evidence path.

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `cmd/` binary inventory | -> | release surface matrix | `spec-release-audit-1-surface-inventory.md` AC-1 |
| YANG config/command schema inventory | -> | config/CLI audit matrix | `spec-release-audit-3-config-cli.md` AC-1 |
| BGP interop scenario inventory | -> | protocol audit matrix | `spec-release-audit-2-bgp-protocol.md` AC-1 |
| Web/API route inventory | -> | web/API audit matrix | `spec-release-audit-4-web-lg-api.md` AC-1 |
| Plugin registry inventory | -> | plugin/RIB audit matrix | `spec-release-audit-5-plugins-rib.md` AC-1 |
| Linux/system feature inventory | -> | Linux/system audit matrix | `spec-release-audit-6-system-linux.md` AC-1 |
| Docs quickstart walkthrough | -> | onboarding audit matrix | `spec-release-audit-8-docs-onboarding.md` AC-1 |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Start release audit | Umbrella spec identifies audit method, finding schema, blocker policy, and child audit set |
| AC-2 | Build release surface inventory | Every binary, component, plugin, YANG surface, web/API surface, and test suite has an owner audit |
| AC-3 | Review any user-visible behavior | Audit row records reachability, unit/functional coverage, negative coverage, and goal evidence |
| AC-4 | Confirm a bug or gap | Finding includes severity, file/line, user impact, reproduction, expected, actual, missing test, owner, suggested direction, and verification requested |
| AC-5 | Document blocker disposition | Finding states whether release is blocked, who owns future work, and what evidence is required before release |
| AC-6 | Review protocol behavior | Interop or ExaBGP evidence exists unless the row is explicitly non-protocol |
| AC-7 | Review Linux/system behavior | QEMU, Linux, fakeOps, or deployment evidence exists as required by the area |
| AC-8 | Review docs/onboarding | Commands and configs in docs are executed or validated against current binaries and schemas |
| AC-9 | Finish audit | Audit report states all open Blocker/Critical findings, owner, suggested direction, and whether release remains blocked |

## 🧪 TDD Test Plan

No production code is implemented by this umbrella. This section documents evidence the audit should request from future fix work.

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| N/A | N/A | Umbrella creates process/spec only | N/A |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | N/A | N/A | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Child audit dependent | `test/**/*.ci`, `test/**/*.et` | Every confirmed user-facing bug documents the end-to-end coverage expected from future fix work | Planned per child audit |

### Interop Tests

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Child audit dependent | `test/interop/scenarios/` | FRR, BIRD, GoBGP | BGP protocol correctness evidence requested from future fix work | Planned per protocol finding |
| ExaBGP compatibility | `test/exabgp-compat/encoding/` | ExaBGP | Wire encoding compatibility | Existing gate via `make ze-verify` |

### Future

- No test deferrals are approved by this umbrella. Any child audit that proposes a deferral must name the destination spec and get explicit user approval.

## Files to Modify

- `plan/spec-release-audit-0-umbrella.md` - release audit organization and blocker policy
- Child audit specs listed in Child Audit Set - created as each audit starts
- No product source, test, schema, or documentation files are modified by this audit umbrella

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | N/A for umbrella |
| CLI commands/flags | [ ] | N/A for umbrella |
| CLI grammar | [ ] | Document expected future fix direction only |
| Editor autocomplete | [ ] | Document expected future fix direction only |
| Functional test for new behavior | [ ] | Document expected evidence for future user-facing bug fixes |
| Doctor check for runtime dependencies | [ ] | Document if a future fix would need one |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | N/A, audit process only |
| 2 | Config syntax changed? | [ ] | Future fix specs decide |
| 3 | CLI command added/changed? | [ ] | Future fix specs decide |
| 4 | API/RPC added/changed? | [ ] | Future fix specs decide |
| 5 | Plugin added/changed? | [ ] | Future fix specs decide |
| 6 | Has a user guide page? | [ ] | `spec-release-audit-8-docs-onboarding.md` decides |
| 7 | Wire format changed? | [ ] | Future fix specs decide |
| 8 | Plugin SDK/protocol changed? | [ ] | Future fix specs decide |
| 9 | RFC behavior implemented? | [ ] | Future fix specs decide |
| 10 | Test infrastructure changed? | [ ] | `spec-release-evidence-gate.md` owns release target changes |
| 11 | Affects daemon comparison? | [ ] | Future fix specs decide |
| 12 | Internal architecture changed? | [ ] | Future fix specs decide |

## Files to Create

- `plan/spec-release-audit-1-surface-inventory.md` - created when inventory audit starts
- `plan/spec-release-audit-2-bgp-protocol.md` - created when protocol audit starts
- `plan/spec-release-audit-3-config-cli.md` - created when config/CLI audit starts
- `plan/spec-release-audit-4-web-lg-api.md` - created when web/API/LG audit starts
- `plan/spec-release-audit-5-plugins-rib.md` - created when plugin/RIB audit starts
- `plan/spec-release-audit-6-system-linux.md` - created when Linux/system audit starts
- `plan/spec-release-audit-7-resilience-security.md` - created when resilience/security audit starts
- `plan/spec-release-audit-8-docs-onboarding.md` - created when docs/onboarding audit starts

## Implementation Steps

Despite the template heading, these are audit documentation steps only. They do not authorize product code changes.

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Initial Release Surface Inventory, Child Audit Set |
| 3. Wiring phase | Wiring Test table maps surfaces to child audits |
| 4. Document findings | Create child audit specs and run review passes, not production code in this umbrella |
| 5. Review gate | Child specs perform adversarial review and document confirmed blockers |
| 6. Full verification | `make ze-verify` plus release evidence from `spec-release-evidence-gate.md` |
| 7. Critical review | Release Blocker Policy and child findings |
| 8. Route issues | Record owner, suggested fix direction, and requested verification |
| 9. Re-verify audit evidence | Re-check source references, reproductions, and status output |
| 10. Repeat | Until no open Blocker/Critical findings remain |
| 11. Deliverables review | Finding list, requested verification, release decision table |
| 12. Security review | `release-audit-7-resilience-security` plus `spec-rbac-audit.md` |
| 13. Re-verify | Final release evidence gate |
| 14. Present summary | Executive release-readiness report |

### Implementation Phases

1. **Phase: Surface inventory** - create `spec-release-audit-1-surface-inventory.md` and derive the full release surface matrix from registries, schemas, commands, routes, plugins, and tests.
2. **Phase: Protocol audit** - create `spec-release-audit-2-bgp-protocol.md`, review BGP surfaces against RFC expectations, ExaBGP compatibility, and FRR/BIRD/GoBGP interop.
3. **Phase: Config and CLI audit** - create `spec-release-audit-3-config-cli.md`, review config validation, editor, CLI command grammar, pipe completeness, commit/reload, and error UX.
4. **Phase: Web, LG, API, MCP audit** - create `spec-release-audit-4-web-lg-api.md`, review HTTP/HTMX/SSE/API auth, empty states, and dispatch coverage.
5. **Phase: Plugin and RIB audit** - create `spec-release-audit-5-plugins-rib.md`, review plugin startup, subscriptions, DirectBridge, RIB storage, route selection, and route reflection.
6. **Phase: Linux and system audit** - create `spec-release-audit-6-system-linux.md`, review iface, FIB, firewall, VPP, traffic, PPPoE, L2TP, IPsec, and kernel dependencies with QEMU/deployment evidence.
7. **Phase: Resilience and security audit** - create `spec-release-audit-7-resilience-security.md`, review races, shutdown, reload, resource exhaustion, secrets, authz, fuzz, and chaos coverage.
8. **Phase: Docs and onboarding audit** - create `spec-release-audit-8-docs-onboarding.md`, run first-user workflows from docs, validate examples, and check command/config references.
9. **Phase: Release decision** - aggregate findings by severity, document requested verification, and produce a release-readiness report.

### Critical Review Checklist

| Check | What to verify for this audit |
|-------|-------------------------------|
| Completeness | Every release surface has an owner child audit and a surface matrix row |
| Correctness | Each finding has reproduction, expected, actual, missing test, suggested direction, and requested verification fields |
| Severity | Blocker/Critical/Major/Minor labels match user impact and release policy |
| Wiring | Every user-facing feature is traced from entry point to runtime code |
| Tests | Every confirmed bug finding documents unit and functional coverage expected from future fix work |
| Interop | Protocol findings have FRR/BIRD/GoBGP/ExaBGP evidence where applicable |
| Linux | Linux-only findings have QEMU, fakeOps, or deployment evidence as required |
| Docs | Documentation claims are source-anchored or validated against current commands/configs |
| Security | Authz, unauthenticated defaults, secrets, and audit logging reviewed before release |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Umbrella spec exists | `plan/spec-release-audit-0-umbrella.md` |
| Child audit set defined | Child Audit Set table in this file |
| Finding schema defined | Finding Schema table in this file |
| Release blocker policy defined | Release Blocker Policy table in this file |
| Initial surface inventory captured | Initial Release Surface Inventory table in this file |
| Existing release evidence work linked | Depends field and Existing Plans section reference `spec-release-evidence-gate.md` |
| Existing RBAC/audit work linked | Existing Plans section references `spec-rbac-audit.md` |

### Security Review Checklist

| Check | What to look for |
|-------|------------------|
| Authz bypass | Web/API/MCP/CLI paths that authenticate but skip authorization |
| Unsafe defaults | No-auth, local-only, dev-only, or fallback paths that grant mutation access |
| Secret leakage | Config, logs, API, web, and docs exposing tokens, keys, passwords, or PSKs |
| Resource exhaustion | Unbounded maps, goroutines, buffers, queues, SSE streams, plugin event paths |
| Protocol hardening | Malformed BGP messages, capability mismatch, error handling, NOTIFICATION behavior |
| System safety | Kernel/VPP/netlink/firewall changes apply/undo correctly and reject unsupported states |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Missing entry-point inventory | `spec-release-audit-1-surface-inventory.md` |
| BGP wire/protocol issue | `spec-release-audit-2-bgp-protocol.md` |
| Config, CLI, editor, pipe issue | `spec-release-audit-3-config-cli.md` |
| Web, LG, REST, gRPC, MCP issue | `spec-release-audit-4-web-lg-api.md` |
| Plugin, RIB, route selection issue | `spec-release-audit-5-plugins-rib.md` |
| Linux/kernel/VPP/system issue | `spec-release-audit-6-system-linux.md` |
| Race, shutdown, security, resource issue | `spec-release-audit-7-resilience-security.md` |
| Docs, install, examples, onboarding issue | `spec-release-audit-8-docs-onboarding.md` |
| Heavy evidence target missing | `spec-release-evidence-gate.md` |
| RBAC/audit fix direction | `spec-rbac-audit.md` |

## Initial Findings and Observations

| ID | Severity | Surface | Observation | Destination |
|----|----------|---------|-------------|-------------|
| RA-DOC-001 | Minor | docs/interop | `docs/architecture/testing/interop.md` lists interop scenarios through 32, while the tree contains `33-bfd-frr`, `34-ecmp-frr`, and `35-srv6-frr` | `spec-release-audit-8-docs-onboarding.md` |
| RA-PROC-001 | Note | process | `tmp/session/selected-spec` lists `spec-doctor-coverage.md`, but no matching `plan/spec-doctor-coverage.md` file exists in this tree | Session/spec hygiene follow-up |

## Mistake Log

### Wrong Assumptions

| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| No existing release evidence plan existed | `plan/spec-release-evidence-gate.md` already exists | Glob and read of existing specs | Umbrella now depends on it instead of duplicating target work |

### Failed Approaches

| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Create every child spec immediately | Would create many partially researched specs without each child audit's source reading complete | Define child set here, create child specs as each audit starts |

### Escalation Candidates

| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| OpenCode has no `ToolSearch` LSP tool while session-start requires it as first action | Seen in this session | Add OpenCode-specific LSP equivalent or exception to `.claude/rules/session-start.md` | Reported to user during session |

## Design Insights

- The release audit should be conducted like a product readiness program, not like a package-by-package code review.
- The first audit child must derive an inventory from registration and schemas, because manually maintained feature lists will drift.
- The existing release evidence gate spec is complementary: it runs broad evidence, while this umbrella defines what must be audited and what evidence findings require.
- Documentation drift is itself a release bug when users are expected to operate from the docs.

## RFC Documentation

No RFC code is implemented by this umbrella. Protocol child audits must reference relevant RFC summaries and add RFC-specific findings where behavior diverges.

## Audit Summary

### What Was Documented

- Not started. This umbrella defines release audit organization only.

### Findings Recorded

- None fixed by this umbrella. No product code changes are in scope for this audit spec.
- Initial observations recorded in Initial Findings and Observations.

### Documentation Updates

- None yet. Documentation findings route to `spec-release-audit-8-docs-onboarding.md`.

### Deviations from Plan

- None.

## Implementation Audit

For this audit spec, "implementation" means audit documentation only. It does not mean product code changes.

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Organize systematic code review for release bugs | Planned | This file | Audit process, child set, findings schema, blocker policy |
| Identify logic issues, bugs, incomplete features | Planned | Child Audit Set | Child audits own discovery |
| Improve initial user experience | Planned | Release Blocker Policy, docs/onboarding child audit | Requires first-user workflow review |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Planned | This file | Umbrella exists |
| AC-2 | Planned | Initial Release Surface Inventory | Needs child audit expansion |
| AC-3 | Planned | Finding Schema | Child audits must fill rows |
| AC-4 | Planned | Finding Schema | Applies when findings are confirmed |
| AC-5 | Planned | Release Blocker Policy | Applies when blockers close |
| AC-6 | Planned | Interop Tests section | Child protocol audit must execute |
| AC-7 | Planned | Linux/system child audit | Child audit must execute |
| AC-8 | Planned | Docs/onboarding child audit | Child audit must execute |
| AC-9 | Planned | Release decision phase | Not reached |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| N/A | N/A | N/A | Process/spec only |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `plan/spec-release-audit-0-umbrella.md` | Planned | This file |
| Child audit specs | Planned | Created as each audit starts |

### Audit Summary

- **Total items:** 3 task requirements, 9 ACs
- **Done:** 0 release-audit execution items
- **Partial:** Umbrella organization created
- **Skipped:** None
- **Changed:** None

## Goal Validation

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Organize systematic release review | Spec evidence | This umbrella defines child audits, blocker policy, finding schema, and inventory seed |
| Find bugs and incomplete features before users | Audit evidence | Child audit findings must provide reproduction, suggested direction, and requested verification |
| Give users a strong initial release experience | Goal evidence | Release decision requires no open Blocker/Critical findings and docs/onboarding validation |

## Review Gate

### Run 1 (initial)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | Umbrella is a process spec; child audits still need source-specific research before execution | This file | Acknowledged |

### Spec Edits Applied

- None.

### Run 2+ (re-runs until clean)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status

- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above

## Checklist

<!-- Added 2026-07-10: this audit spec predates the validator's required-section list
     (.claude/hooks/validate-spec.sh). Audit umbrellas produce documentation, not code;
     the TDD items bind the future fix work routed from findings, not this spec. -->

### Goal Gates (MUST pass)

- [ ] Every release surface has an owner child audit (AC-2)
- [ ] Findings use the Finding Schema with evidence (AC-4)
- [ ] `make ze-test` evidence requested from future fix work per the Release Blocker Policy

### TDD (applies to future fix specs routed from findings)

- [ ] Tests written (in the owning fix spec)
- [ ] Tests FAIL (paste output) (in the owning fix spec)
- [ ] Tests PASS (paste output) (in the owning fix spec)

### Post-wave corrections (2026-07-10)

Re-verified against the current tree after the followup implementation wave (unpushed origin/main..HEAD). Stale counts in Current Behavior and the Initial Release Surface Inventory are struck inline above; details and citations:

| Stale statement (location in this spec) | Current verified state (2026-07-10) |
|------------------------------------------|--------------------------------------|
| "35 component directories" (Current Behavior; Core components inventory row) | 43 directories under `internal/component/` |
| "22 plugin directories" (Current Behavior; plugins inventory row) | 63 top-level directories under `internal/plugins/`; additionally the nested `internal/plugins/exabgp/bridgeplugin` registers the NEW plugin `exabgp-bridge` (`internal/plugins/exabgp/bridgeplugin/register.go`) |
| "32 test directories" (Current Behavior; Release tests row) | 45 directories under `test/` |
| Interop scenarios "through `35-srv6-frr`"; docs "list through 32" (Current Behavior; Interop tests row; finding RA-DOC-001) | 101 scenario directories under `test/interop/scenarios/`; `docs/DESIGN.md` states "101 interop scenarios"; `docs/architecture/testing/interop.md` lists `33-bfd-frr` in its core table (through 37). RA-DOC-001 as filed is superseded; the residual count drift across docs (96 vs 97 vs 101) is tracked in `spec-release-audit-8-docs-onboarding.md` Post-wave corrections |
| "API and MCP" surface row (streaming via legacy handler implied) | MCP is Streamable-HTTP-only: legacy `internal/component/mcp/handler.go` was deleted; `internal/component/mcp/streamable.go` `handlePOST`, `handleGET`, `handleDELETE` are the sole transport |
| Inventory seed omits wave-new surfaces | `internal/core/dnsserver` (shared DNS listener core: DoT `secure.go`, DoH `secure.go`, consumed by as112 + geodns) and `internal/plugins/exabgp/bridgeplugin` need owner-audit rows when child audits resume |

Also from this correction pass: the `## TDD Test Plan` heading was renamed to `## 🧪 TDD Test Plan` and this `## Checklist` section added, both to satisfy the blocking spec validator; no audit content was changed by those two edits.

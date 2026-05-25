# Spec: doctor-improvements

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-doctor-coverage.md |
| Phase | - |
| Updated | 2026-05-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-doctor-coverage.md` - active first coverage implementation
3. `.claude/rules/planning.md` - workflow rules
4. `ai/rules/doctor-checks.md` - required doctor coverage rule
5. `cmd/ze/doctor/doctor.go` - existing doctor orchestration
6. `cmd/ze/doctor/checks_linux.go` - Linux-specific doctor probes
7. `internal/component/config/listener.go` - schema-driven listener discovery
8. `cmd/ze/config/cmd_validate.go` - config validation diagnostics

## Task

After `spec-doctor-coverage.md` fills the first set of missing doctor checks, improve doctor so the same class of gaps does not return. This spec is about doctor architecture and diagnostic accuracy: reuse schema-driven listener discovery, run existing semantic validators from doctor where safe, correct feature trigger semantics, and add checks for second-order dependencies discovered during the audit.

This spec intentionally depends on `spec-doctor-coverage.md`. Do not start implementation until the first spec is either closed or its final implementation audit says which items remain. If the first spec already implemented an item here, mark that row as satisfied during this spec's implementation audit rather than duplicating code.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/doctor-checks.md` - runtime dependency coverage rule
  -> Decision: every file, socket, service, port, module, binary, certificate, procfs, sysfs, netlink, or privilege dependency needs a doctor check or an explicit exclusion.
  -> Constraint: every new `doctor-*` diagnostic code must be registered in `internal/core/diagnostic/codes.go`.
- [ ] `docs/features/ai-first.md` - doctor is part of agent-readiness tooling
  -> Decision: doctor output must remain structured and stable enough for agents to consume.
  -> Constraint: diagnostics need stable codes and `ze explain` coverage.
- [ ] `docs/architecture/config/syntax.md` - config parsing and YANG-driven behavior
  -> Decision: doctor should reuse config and schema extraction rather than duplicate YANG knowledge.
  -> Constraint: config defaults and generated schema behavior must match daemon behavior.
- [ ] `docs/architecture/core-design.md` - Ze component/plugin separation
  -> Decision: doctor may inspect config and probe OS readiness, but must not start components or mutate runtime state.
  -> Constraint: no new coupling from doctor into plugin internals that requires daemon startup.

### RFC Summaries
- [ ] N/A - this spec adds readiness checks and diagnostic plumbing, not protocol behavior.

**Key insights:**
- Doctor is useful because it is offline. It must not require daemon startup or plugin handshakes.
- The current doctor duplicates listener extraction for web, MCP, looking-glass, API, and SSH instead of using the schema-driven listener inventory.
- `config.CollectListeners` currently discovers `ze:listener` services but omits runtime defaults for empty server lists. Doctor needs runtime endpoints, not only explicit endpoints.
- `ze config validate` already runs semantic validation and listener conflict detection. Doctor should surface those failures instead of maintaining parallel validation logic.
- PKI material lives in config as base64 DER, not filesystem paths. Doctor must validate the material, not look for files.
- NTP is a client in Ze. The useful readiness checks are server reachability, clock-adjust privilege, RTC write if present, and persist-path writeability.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `plan/spec-doctor-coverage.md` - active first coverage spec with Tier 1 to Tier 3 missing checks.
- [ ] `cmd/ze/doctor/doctor.go` - `runChecks` hardcodes check calls and `checkListeners` manually extracts web, MCP, looking-glass, API, and SSH TCP listeners.
- [ ] `cmd/ze/doctor/checks_linux.go` - Linux checks cover VPP default socket, VPP version, selected kernel modules, ethernet interfaces, kernel nexthop, and MPLS modules.
- [ ] `cmd/ze/doctor/checks_other.go` - non-Linux stubs for Linux-only checks.
- [ ] `internal/core/diagnostic/codes.go` - registered doctor diagnostic codes.
- [ ] `internal/component/config/listener.go` - schema-driven `ze:listener` discovery and listener conflict detection.
- [ ] `internal/component/config/listener_test.go` - confirms discovery for web, SSH, MCP, looking-glass, Prometheus, plugin-hub, API REST, API gRPC, and WireGuard.
- [ ] `cmd/ze/config/cmd_validate.go` - config validation already runs MCP validation, plugin verifiers, BGP validation, hub validation, and listener conflict diagnostics.
- [ ] `internal/component/bgp/reactor/reactor.go` - BGP passive listeners are derived from peer local addresses at runtime.
- [ ] `internal/component/pki/config.go` - PKI config parser decodes base64 DER certs and keys and verifies key/cert matching.
- [ ] `internal/component/pki/store.go` - PKI store rejects expired certs and invalid chains.
- [ ] `internal/plugins/ntp/ntp.go` - NTP queries configured servers and adjusts the local clock as a client.
- [ ] `internal/plugins/ntp/persist.go` - NTP persists time to a configured file path.
- [ ] `internal/component/vpp/schema/ze-vpp-conf.yang` - VPP config has managed/external mode, API socket, stats socket, and DPDK PCI interfaces.
- [ ] `internal/component/vpp/dpdk.go` - DPDK binding requires VFIO modules and writable sysfs paths.

**Behavior to preserve:**
- `ze doctor` remains an offline command registered under `cmdregistry`.
- Text output remains concise, with `all checks passed` when there are no diagnostics.
- JSON output remains `diagnostic.DoctorResult` with stable diagnostic records.
- Readiness remains false only when at least one diagnostic has severity error.
- Linux-only probes remain in `checks_linux.go` with stubs in `checks_other.go`.
- Doctor must not mutate config, start plugins, apply nftables, bind DPDK devices, change sysctls, change system time, or create persistent files except for safe temporary write probes that clean up after themselves.

**Behavior to change:**
- Replace doctor-only hardcoded listener extraction with a shared runtime listener inventory that covers all `ze:listener` services and service defaults.
- Add BGP passive listener endpoints to doctor readiness, because they are not expressed as `ze:listener` schema nodes.
- Surface safe `ze config validate` semantic failures from doctor before OS probes.
- Correct readiness triggers where the first spec was too broad or used the wrong config path.
- Add dependency inventory tests so new YANG runtime dependencies must be consciously handled.

## Scope Boundary

| In Scope | Out of Scope |
|----------|--------------|
| Offline readiness probes only | Starting daemons or plugins from doctor |
| Shared listener/dependency extraction | Full daemon health monitoring |
| Diagnostic code registration and tests | Changing CLI output format beyond new diagnostics |
| Config-driven checks only | Probing unconfigured optional features |
| Short timeout external service probes | Long-running protocol handshakes or full interop validation |

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Operator or agent runs `ze doctor [--json] [<config-file>]`.
- Doctor loads config from explicit file or active zefs storage.
- Config is parsed into `*config.Tree` by `config.LoadConfig`.

### Transformation Path
1. `runChecks` resolves storage, loads config, and parses it into `LoadConfigResult`.
2. Doctor runs safe semantic validation against the parsed tree and converts failures to diagnostics.
3. Doctor builds a runtime listener inventory from schema-driven services plus BGP passive listeners.
4. Doctor runs OS and network readiness probes for configured dependencies only.
5. Probe failures return `diagnostic.Diagnostic` values with stable `doctor-*` codes.
6. `outputJSON` or `outputText` renders the final `DoctorResult`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> Doctor | `cmdregistry.MustRegisterLocal("doctor", Run)` | [ ] |
| Doctor -> Config | `config.LoadConfig`, semantic validators, schema helpers | [ ] |
| Doctor -> Schema | shared listener/dependency inventory reads YANG-derived schema metadata | [ ] |
| Doctor -> OS | `os.Stat`, temporary write probes, bind probes, `/proc`, `/sys`, netlink open probes | [ ] |
| Doctor -> Network | bounded TCP/UDP reachability probes | [ ] |
| Doctor -> Diagnostics | stable `diagnostic.Diagnostic` and registered codes | [ ] |

### Integration Points
- `cmd/ze/doctor/runChecks` aggregates all checks and determines readiness.
- `internal/component/config/listener.go` supplies or is extended to supply runtime listener endpoints.
- `cmd/ze/config/cmd_validate.go` validation behavior is reused or mirrored through a side-effect-free helper.
- `internal/core/diagnostic/codes.go` registers each new code.
- `cmd/ze/doctor/doctor_test.go` proves wiring and check behavior.

### Architectural Verification
- [ ] No bypassed layers: doctor reads parsed config and schema helpers, not raw tokenizer internals.
- [ ] No unintended coupling: doctor does not import plugin runtime engines when a config-level helper is sufficient.
- [ ] No duplicated functionality: listener collection and semantic validation are shared with config validation where possible.
- [ ] Zero-copy preserved where applicable: N/A, doctor is offline readiness tooling, not a wire hot path.

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `ze doctor --json <config-with-prometheus-listener>` | -> | runtime listener inventory | `TestDoctorListenersPrometheus` |
| `ze doctor --json <config-with-plugin-hub-listener>` | -> | runtime listener inventory | `TestDoctorListenersPluginHub` |
| `ze doctor --json <config-with-wireguard-listen-port>` | -> | UDP listener bind probe | `TestDoctorListenersWireguardUDP` |
| `ze doctor --json <config-with-passive-bgp-peer>` | -> | BGP passive listener collector | `TestDoctorListenersBGPPassive` |
| `ze doctor --json <config-with-invalid-plugin-verify>` | -> | config validation bridge | `TestDoctorConfigValidationBridge` |
| `ze doctor --json <config-with-pki>` | -> | PKI material validation | `TestDoctorPKIMaterial` |
| `ze doctor --json <config-with-vpp-custom-socket>` | -> | configured VPP probe | `TestDoctorVPPConfiguredSocket` |
| `ze doctor --json <config-with-ntp>` | -> | NTP client readiness | `TestDoctorNTPClientReadiness` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A configured `ze:listener` service not currently hardcoded in doctor, such as Prometheus, plugin-hub, or WireGuard | Doctor includes the listener in bind checks without adding a service-specific hardcoded branch. |
| AC-2 | A service is enabled and relies on YANG refine defaults for its server address or port | Doctor probes the same effective endpoint the daemon will bind. |
| AC-3 | A passive BGP peer has `connection/local/accept true` and a local address or listen port | Doctor probes the effective BGP listener endpoint and reports bind failures as `doctor-bgp-listen`. |
| AC-4 | A BGP peer listener requires TCP MD5 | Doctor probes listener creation with MD5 support or emits a specific warning/error when the platform cannot support it. |
| AC-5 | Config validation would fail via an existing side-effect-free validator | Doctor emits a diagnostic rather than continuing with misleading OS probes. |
| AC-6 | `vpn/ipsec` is configured | IPsec-related doctor checks use the `vpn/ipsec` path and never rely on a top-level `ipsec` container. |
| AC-7 | `pki` contains CA/device certificate material | Doctor parses base64 DER material, validates expiry, chain, and private key match; it does not look for certificate files. |
| AC-8 | `environment/ntp` is enabled | Doctor treats NTP as a client, probes configured server reachability, checks clock-adjust readiness, and checks `persist-path` writeability. |
| AC-9 | VPP is configured with a non-default `api-socket` or managed DPDK interfaces | Doctor uses the configured socket and checks VPP binary, VFIO modules, PCI device sysfs paths, and write permissions as appropriate. |
| AC-10 | External services are configured for RPKI, BMP sender, TACACS+, RADIUS, update-check, or HTTP archive destinations | Doctor probes reachability with short timeouts and reports warnings unless the dependency prevents local startup. |
| AC-11 | File write destinations are configured for DNS resolv.conf, NTP persist, BFD persist, archive file URLs, or self-update auto-apply | Doctor checks parent directory existence or safe writeability and cleans up temporary files. |
| AC-12 | A YANG schema adds a new listener, path, URL, socket, external service, binary, certificate material, procfs, sysfs, netlink, or kernel-module dependency | A dependency inventory test fails until the dependency is covered or explicitly excluded. |
| AC-13 | New doctor diagnostics are added | Every code is registered in `internal/core/diagnostic/codes.go` and can be explained by `ze explain`. |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDoctorListenersPrometheus` | `cmd/ze/doctor/doctor_test.go` | AC-1, dynamic listener inventory includes Prometheus | |
| `TestDoctorListenersPluginHub` | `cmd/ze/doctor/doctor_test.go` | AC-1, plugin hub listener included | |
| `TestDoctorListenersWireguardUDP` | `cmd/ze/doctor/doctor_test.go` | AC-1, UDP listener probing works | |
| `TestDoctorListenerDefaults` | `cmd/ze/doctor/doctor_test.go` | AC-2, runtime defaults are included | |
| `TestDoctorListenersBGPPassive` | `cmd/ze/doctor/doctor_test.go` | AC-3, passive BGP listener inventory | |
| `TestDoctorListenersBGPMD5` | `cmd/ze/doctor/doctor_test.go` | AC-4, MD5 listener readiness handled | |
| `TestDoctorConfigValidationBridge` | `cmd/ze/doctor/doctor_test.go` | AC-5, semantic validation surfaced | |
| `TestDoctorIPsecPathVPN` | `cmd/ze/doctor/doctor_test.go` | AC-6, correct `vpn/ipsec` trigger | |
| `TestDoctorPKIMaterial` | `cmd/ze/doctor/doctor_test.go` | AC-7, PKI config material validated | |
| `TestDoctorNTPClientReadiness` | `cmd/ze/doctor/doctor_test.go` | AC-8, NTP client checks | |
| `TestDoctorVPPConfiguredSocket` | `cmd/ze/doctor/doctor_test.go` | AC-9, configured VPP socket used | |
| `TestDoctorExternalServiceReachability` | `cmd/ze/doctor/doctor_test.go` | AC-10, bounded external probes | |
| `TestDoctorWritableDestinations` | `cmd/ze/doctor/doctor_test.go` | AC-11, writable path checks | |
| `TestDoctorDependencyInventory` | `cmd/ze/doctor/doctor_test.go` or `internal/component/config/*_test.go` | AC-12, schema dependencies accounted for | |
| `TestDoctorDiagnosticCodesRegistered` | `cmd/ze/doctor/doctor_test.go` | AC-13, all `doctor-*` codes registered | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Listener port | 1..65535 | 65535 | 0 | 65536 |
| Probe timeout | implementation constant | maximum configured constant | 0 | N/A |
| TLS/cert expiry warning days | implementation constant | 30 days | already expired | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-doctor-dynamic-listeners` | `test/doctor/*.ci` | Doctor reports a bind problem for a non-hardcoded configured listener | |
| `test-doctor-config-validation` | `test/doctor/*.ci` | Doctor reports existing config validation diagnostics | |
| `test-doctor-pki-material` | `test/doctor/*.ci` | Doctor reports expired or invalid PKI material | |
| `test-doctor-writable-paths` | `test/doctor/*.ci` | Doctor reports unwritable configured file destinations | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | N/A | N/A | This spec adds readiness checks only; no protocol behavior changes. | N/A |

### Future
- None planned. If a probe cannot be tested without root or Linux-only kernel support, add a unit-testable probe abstraction plus a QEMU functional test or record an explicit user-approved deferral.

## Files to Modify
- `cmd/ze/doctor/doctor.go` - wire shared validation, listener inventory, new checks, and diagnostics.
- `cmd/ze/doctor/checks_linux.go` - Linux-only probes for MD5 listener support, VPP/DPDK, clock privileges, netlink, procfs, sysfs, and writable destinations.
- `cmd/ze/doctor/checks_other.go` - non-Linux stubs for Linux-only probes.
- `cmd/ze/doctor/doctor_test.go` - unit tests for all ACs.
- `internal/core/diagnostic/codes.go` - register new `doctor-*` codes.
- `internal/component/config/listener.go` - extend or add a runtime endpoint collector if default-aware behavior belongs in shared config code.
- `cmd/ze/config/cmd_validate.go` or a helper it uses - expose side-effect-free validation for doctor reuse if no safe helper exists yet.
- `ai/rules/doctor-checks.md` - reinforce dependency classes found in this audit if they are still missing.
- `plan/TEMPLATE.md` - update the Integration Checklist only if the current wording is insufficient after implementation review.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| CLI commands/flags | No | Existing `ze doctor` only |
| CLI grammar (action before identifier) | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | No | No new RPC/API |
| Doctor check for runtime dependencies | Yes | This spec improves doctor coverage itself |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features/ai-first.md` if doctor behavior is documented there |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | Maybe | `docs/functional-tests.md` only if a new doctor functional test category is introduced |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Maybe | `docs/architecture/config/syntax.md` or `docs/architecture/core-design.md` only if shared config validation/listener architecture changes |

## Files to Create
- `cmd/ze/doctor/dependencies.go` - optional, only if dependency inventory logic would make `doctor.go` too large.
- `cmd/ze/doctor/listeners.go` - optional, only if listener inventory/probing needs its own file.
- `test/doctor/*.ci` - functional tests for user-visible doctor diagnostics if a doctor test directory exists or is created.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file plus `spec-doctor-coverage.md` final audit |
| 2. Audit | Current Behavior, Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist |
| 8. Fix issues | Fix every issue from review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases
1. **Phase: Wiring** - add tests proving doctor reaches dynamic listeners, BGP passive listeners, config validation, PKI material validation, and dependency inventory.
2. **Phase: Listener inventory** - replace hardcoded doctor listener extraction with shared runtime listener collection and default-aware endpoint extraction.
3. **Phase: Semantic validation bridge** - expose safe config validation diagnostics to doctor without starting plugins or applying config.
4. **Phase: Correct feature semantics** - ensure IPsec, PKI, NTP, BFD, and L2TP checks trigger on the same conditions as runtime.
5. **Phase: Second-order probes** - add VPP/DPDK, external service, write-destination, privilege, procfs, sysfs, and netlink probes not completed by `spec-doctor-coverage.md`.
6. **Phase: Dependency inventory** - add schema/runtime dependency accounting test and explicit exclusion list.
7. **Functional tests** - add `.ci` coverage for representative user-visible doctor diagnostics.
8. **Full verification** - run `make ze-verify` after unit and functional tests pass.
9. **Complete spec** - fill audit tables, write learned summary, and close the spec per planning rules.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-13 has implementation or explicit audit evidence from `spec-doctor-coverage.md`. |
| Correctness | Each check fires only for configured dependencies and uses the same config path/runtime trigger as the daemon. |
| Diagnostic quality | New diagnostics include code, severity, message, path/expected/actual where useful, and registered explanations. |
| Listener coverage | Doctor covers TCP and UDP listeners, default endpoints, and BGP passive listeners. |
| Safety | Doctor probes do not mutate persistent runtime state. Temporary files are removed. No daemon/plugin startup. |
| Timeouts | Every network probe has a short timeout and cannot hang doctor. |
| Data flow | Shared validators and listener helpers avoid duplicate config knowledge. |
| Doctor checks | All new runtime dependency classes are represented in dependency inventory. |
| Rule: no-layering | Doctor does not import deep runtime packages when a config-level helper exists. |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Dynamic listener tests exist | `go test ./cmd/ze/doctor -run 'TestDoctorListeners'` |
| Config validation bridge test exists | `go test ./cmd/ze/doctor -run TestDoctorConfigValidationBridge` |
| PKI/NTP/VPP tests exist | `go test ./cmd/ze/doctor -run 'TestDoctor(PKI|NTP|VPP)'` |
| Dependency inventory test exists | `go test ./cmd/ze/doctor -run TestDoctorDependencyInventory` |
| New diagnostic codes registered | `grep 'doctor-' internal/core/diagnostic/codes.go` |
| Functional doctor tests exist | `ls test/doctor` if a doctor functional directory is created |
| Rules/template reviewed | Diff shows either updates or explicit no-change note in Implementation Summary |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Config paths, sockets, URLs, hosts, and ports are parsed with existing validators where possible. |
| Path safety | Write probes stay inside the configured destination, create only temporary files, and remove them. |
| Network safety | External probes use bounded timeouts and do not send credentials or secrets. |
| Secret handling | Diagnostics never print TLS keys, PKI private keys, passwords, RADIUS/TACACS secrets, BGP MD5 passwords, or WireGuard keys. |
| Privilege probes | Capability checks do not change system clock, DPDK binding, nftables rules, sysctls, routes, or interfaces. |
| Resource exhaustion | Dependency inventory and probes are linear in configured dependencies and avoid unbounded goroutines. |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read Current Behavior and the final audit from `spec-doctor-coverage.md` |
| Lint failure | Fix inline; if architectural, return to design |
| Functional test fails | Check AC; if AC wrong, update design; if AC correct, fix implementation |
| Probe requires root or Linux kernel support | Add injectable probe abstraction and QEMU/skip-aware test, or ask for explicit deferral |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| PKI doctor should check files | PKI material is base64 DER inside config | `ze-pki-conf.yang` and `pki/config.go` | PKI check must parse config material instead of statting paths |
| NTP doctor should bind UDP/123 | NTP plugin is a client and queries configured servers | `ntp.go` and `ze-ntp-conf.yang` | NTP readiness should check server reachability and clock privileges |
| IPsec block is top-level `ipsec` | Runtime parser uses `vpn/ipsec` | `ze-ipsec-conf.yang` and `ipsec/config.go` | Existing module trigger can silently miss configured IPsec |
| `CollectListeners` is enough as-is for doctor | It currently omits endpoints relying only on YANG refine defaults | `listener.go` comments | Doctor needs default-aware runtime endpoints |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- Doctor should be driven from the same config/schema metadata as startup and validation. Every duplicated extractor is a future coverage gap.
- Validation and readiness are distinct. Validation answers whether config is acceptable; doctor answers whether the current host can run it. Doctor should report both, but should not use OS probes to compensate for missing config validation.
- The severity rule should be explicit: local startup blockers are errors; remote service unreachability is usually a warning unless the service is required for daemon startup.
- The dependency inventory test is the guardrail that keeps this work from becoming another one-off audit.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered]

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
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Doctor stops duplicating listener knowledge | unit + functional test | Test names and output |
| Doctor uses accurate runtime triggers | unit test | Test names per corrected feature |
| New runtime dependency gaps are guarded | inventory test | Test output and exclusion list |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [short bullet per fix]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated or explicitly not needed
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-doctor-improvements.md`
- [ ] **Summary included in commit**

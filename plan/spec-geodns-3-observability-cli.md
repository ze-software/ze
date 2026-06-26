# Spec: geodns-3-observability-cli

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-geodns-2-server |
| Phase | 5/5 |
| Updated | 2026-06-26 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `spec-geodns-0-umbrella.md`, `spec-geodns-2-server.md`
3. `ai/rules/plugin-self-containment.md` - `show geodns` ownership
4. `internal/plugins/ntp/register.go` + `internal/plugins/ntp/yang/ze-ntp-cmd.yang` - a plugin that owns a `show` command
5. `ai/rules/doctor-checks.md` - doctor check registration

## Task

Add the observability and operator surface around the geodns server: Prometheus
metrics (mirroring the reference's, minus Sentry), a self-contained `show geodns`
CLI command reporting live status, and a doctor check for the listen endpoint.
Each surface is owned by the geodns plugin so the "delete the folder" invariant
holds.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/plugin-self-containment.md` - command ownership
  → Decision: the `show geodns` schema + handler live in `internal/plugins/geodns` (its own `yang/ze-geodns-cmd.yang` + `RegisterRPCs`), never in a central `show` package.
  → Constraint: a self-containment test asserts the central `show` schema declares no geodns tokens, and the geodns `yang/` asserts it owns them.
- [ ] `internal/plugins/ntp/register.go` - `pluginserver.RegisterRPCs` for `show system ntp`
  → Constraint: mirror the `WireMethod`/`Handler` registration shape.
- [ ] `ai/rules/doctor-checks.md` - runtime-dependency checks
  → Constraint: a listen port is a runtime dependency → a doctor check with a diagnostic code, unit test, and functional test is required.
- [ ] `ai/rules/cli-grammar.md` - command grammar
  → Constraint: action before identifier; `show geodns` follows the verb-noun form.

### RFC Summaries (MUST for protocol work)
- [ ] N/A - this spec adds no wire protocol behavior; metrics/CLI/doctor only.

**Key insights:**
- The reference exposes `geodns_dns_request_total`, `geodns_dns_response_total`, `geodns_dns_request_latency_milliseconds`, `geodns_zone_reload_total`, `geodns_listener_up`, plus parse/duplicate/critical-file counters. The folder-specific ones (parse error, duplicate host, critical file removed) lose meaning once config is YANG-validated at commit; keep request/response/latency/reload/listener-up and add a config-reload counter.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `/Users/thomas/Code/git.exa.net.uk/tech/dev/surfprotect/geodns/src/geodns/stats/metrics.go` - the Prometheus metric set + labels (bounded: zone, qtype, rcode, protocol).
- [ ] `/Users/thomas/Code/git.exa.net.uk/tech/dev/surfprotect/geodns/src/geodns/stats/server.go` - the reference metrics HTTP server (Ze uses its own telemetry, so this is not ported).
- [ ] `/Users/thomas/Code/git.exa.net.uk/tech/dev/surfprotect/geodns/cmd/geodns/main.go` - `--validate-config` status report (informs `show geodns` content).
- [ ] `internal/plugins/ntp/register.go` - ze plugin that owns a `show` command + RPC handler.
- [ ] `internal/core/metrics` (registry) - how ze plugins register counters.

**Behavior to preserve:**
- Bounded metric labels only (zone, qtype, rcode, protocol; plus the server-config `address` on `listener_up`, which is operator-set and low-cardinality) — never label by client IP or hostname (cardinality).
- A counter for requests, responses (by rcode), per-request latency histogram, and listener-up gauge.
- A reload counter (the reference's `zone_reload_total`, repurposed for config reloads).

**Behavior to change:**
- No Sentry; no standalone metrics HTTP server — register into ze's telemetry/metrics registry.
- Drop the folder-only counters (parse_error, duplicate_host, critical_file_removed) — YANG validation at commit makes them obsolete; add a `geodns_config_reload_total{result}` instead.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Metrics: each handled query (spec 2) increments counters.
- CLI: an operator runs `show geodns`; the CLI dispatches the RPC to the plugin handler.
- Doctor: `ze doctor` runs the geodns listen-port check.

### Transformation Path
1. Query handled (spec 2) → metric increments (request, response-by-rcode, latency observe).
2. `show geodns` → RPC `ze-show:geodns` → handler reads the atomic resolver snapshot + counters → status table (enabled, bind, zones, NS, source count, last reload).
3. Port conflicts are detected at config-parse time: because `listen-address` is a leaf-list sharing one `listen-port`, the plugin verifier checks each (`listen-address`, `listen-port`) pair against other services (and the `ze:listener` registry where a list form applies). The `ze doctor` geodns check verifies privileged-port bind *capability* (port <1024 needs CAP_NET_BIND_SERVICE) per address family and config sanity → diagnostic. It does NOT bind the live ports the server already owns (that would always conflict, a false positive).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Handler ↔ metrics | `ConfigureMetrics(reg any)` injects the host `metrics.Registry`; the in-process engine goroutine increments the host-owned counters | [ ] |
| CLI ↔ plugin | `pluginserver.RegisterRPCs` (`WireMethod` `ze-show:geodns`) | [ ] |
| Doctor ↔ plugin | `registry.DoctorCheckDef` + `diagnostic` code | [ ] |

### Integration Points
- `internal/core/metrics` - counter/gauge/histogram registration.
- `internal/component/plugin/pluginserver` - `RegisterRPCs`.
- `internal/core/diagnostic` - `DoctorCheckContext`, diagnostic codes.
- `internal/plugins/geodns/yang/ze-geodns-cmd.yang` - `show geodns` schema (container merge onto the central `show` root).

### Architectural Verification
- [ ] No bypassed layers (metrics via registry, not a private HTTP server)
- [ ] No unintended coupling (`show geodns` not in any central verb package)
- [ ] No duplicated functionality (reuse ze metrics + doctor frameworks)
- [ ] Zero-copy preserved where applicable (status formatting via textbuf)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A plugin can register Prometheus counters via ze's metrics registry | reference uses `prometheus`; ze has `internal/core/metrics` | Expose via a different telemetry hook | metrics unit test | unvalidated |
| A-2 | `show geodns` can container-merge onto the central `show` root without touching it | `ai/rules/plugin-self-containment.md` "container merge" | Use `augment` against the show anchor | self-containment test | unvalidated |
| A-3 | A doctor check verifying bind *capability* (not binding the live port) plus `ze:listener` parse-time conflict detection covers port issues | `ai/rules/doctor-checks.md`; `ze:listener` extension; dhcp/ntp doctor `.ci` exist (`test/ui/doctor-*.ci`) | Limit to a config sanity check | `test/ui/doctor-geodns.ci` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Metric cardinality blowup if a label leaks client IP/hostname | metrics scrape explodes | Lint the label set; tests assert bounded labels |
| R-2 | `show geodns` drifts from actual server state | stale status | Read the same atomic snapshot the handler uses |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show geodns` | → | `ze-show:geodns` RPC handler returns status | `test/plugin/geodns-show.ci` |
| `ze doctor` with geodns configured | → | listen-port doctor check runs | `test/ui/doctor-geodns.ci` |
| handled query | → | request/response counters increment | `test/plugin/geodns-metrics.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show geodns` with enabled config | Status shows enabled, bind addr, zones, NS, source count, last reload |
| AC-2 | A handled A query | `geodns_dns_request_total` and `geodns_dns_response_total{rcode=...}` increase |
| AC-3 | Listeners bound | `geodns_listener_up{protocol,address}` is 1 per bound (protocol, listen-address) endpoint; 0 after shutdown |
| AC-4 | Config reload | `geodns_config_reload_total{result=success}` increments |
| AC-5 | geodns on `:53` without CAP_NET_BIND_SERVICE | doctor reports a privileged-port bind-capability diagnostic |
| AC-6 | central `show` schema | declares no geodns tokens (self-containment test) |
| AC-7 | metric label set | only bounded labels (zone, qtype, rcode, protocol, and the server-config `address` on `listener_up`); never client IP/hostname |
| AC-8 | Two services declare the same (address, port) endpoint | config-parse-time conflict detection (verifier / `ze:listener`) rejects the commit |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `show geodns` | CLI → RPC → handler → status table | `test/plugin/geodns-show.ci` |
| 2 | Runs `ze doctor` | doctor registry → geodns check → diagnostic | `test/ui/doctor-geodns.ci` |
| 3 | Scrapes metrics after queries | handler increments → registry → scrape | `test/plugin/geodns-metrics.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestShowGeoDNSStatus` | `internal/plugins/geodns/show_test.go` | status fields from snapshot | |
| `TestMetricsBoundedLabels` | `internal/plugins/geodns/metrics_test.go` | only bounded labels; counters increment | |
| `TestDoctorListenPort` | `internal/plugins/geodns/doctor_test.go` | missing CAP_NET_BIND_SERVICE for a privileged port produces a diagnostic | |
| `TestShowSelfContainment` | `internal/plugins/geodns/yang/self_containment_test.go` | central show has no geodns tokens; geodns yang owns them | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no new numeric config in this spec) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `geodns-show` | `test/plugin/geodns-show.ci` | `show geodns` reports status | |
| `geodns-metrics` | `test/plugin/geodns-metrics.ci` | counters reflect handled queries | |
| `doctor-geodns` | `test/ui/doctor-geodns.ci` | doctor flags a bind/port problem | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A (no wire protocol changes) | - | - | observability/CLI only | - |

### Future (if deferring any tests)
- None.

## Files to Modify
- `internal/plugins/geodns/server.go` - call metric increments on the query path.
- `internal/plugins/geodns/register.go` - register RPC handler + doctor check + `ConfigureMetrics` in `registry.Registration`.
- `internal/plugins/geodns/yang/ze-geodns-conf.yang` - listen-endpoint conflict detection (verifier over `listen-address`×`listen-port`; `ze:listener` where a list form applies).
- `internal/component/cmd/show/yang/self_containment_test.go` - add `geodns` to the central-show banned-token map (`TestShowSchemaHasNoMigratedOwnerCommands`).
- `internal/component/plugin/all/all.go` - regenerated by `make generate` for the new `yang/` cmd package.

## Files to Create
- `internal/plugins/geodns/metrics.go` - counter/gauge/histogram definitions + registration.
- `internal/plugins/geodns/show.go` - `ze-show:geodns` handler.
- `internal/plugins/geodns/doctor.go` - listen-port doctor check + diagnostic code.
- `internal/plugins/geodns/yang/ze-geodns-cmd.yang` - `show geodns` command schema (container merge).
- `internal/plugins/geodns/{show_test.go,metrics_test.go,doctor_test.go}`.
- `internal/plugins/geodns/yang/self_containment_test.go`.
- `test/plugin/geodns-show.ci`, `test/plugin/geodns-metrics.ci`, `test/ui/doctor-geodns.ci`.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Create |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 6. Full verification | `make ze-test` |

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** — register the `ze-show:geodns` RPC + the doctor check stub; add `yang/ze-geodns-cmd.yang`; failing `test/plugin/geodns-show.ci`.
   - Tests: `geodns-show`
   - Files: `register.go`, `show.go`, `yang/ze-geodns-cmd.yang`
2. **Phase: metrics** — define + register counters; increment on the query path.
   - Tests: `TestMetricsBoundedLabels`, `geodns-metrics`
   - Files: `metrics.go`, `server.go`
3. **Phase: show handler** — status from the atomic snapshot.
   - Tests: `TestShowGeoDNSStatus`
   - Files: `show.go`
4. **Phase: doctor check** — bind-capability check (privileged port) + verifier/`ze:listener` parse-time conflict over the `listen-address`×`listen-port` set; diagnostic code.
   - Tests: `TestDoctorListenPort`, `doctor-geodns`
   - Files: `doctor.go`, `yang/ze-geodns-conf.yang`
5. **Phase: self-containment** — central-show ban test + geodns ownership test.
   - Tests: `TestShowSelfContainment`
   - Files: `yang/self_containment_test.go`
6. **Functional tests** → `.ci` per user story.
7. **Full verification** → `make ze-verify`.
8. **Complete spec** → learned summary + two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Prometheus counters | defined, registered, bounded labels; names listed |
| Self-containment | deleting plugin removes `show geodns`, metrics, doctor; central show clean |
| Doctor checks | registered with code + unit + functional test |
| CLI grammar | `show geodns` verb-noun; completion derived from schema |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `show geodns` works | `test/plugin/geodns-show.ci` passes |
| Doctor check registered | `test/ui/doctor-geodns.ci` passes |
| Self-containment | `go test ./internal/plugins/geodns/yang -run TestShowSelfContainment` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Metric cardinality | no client IP / hostname labels |
| Status disclosure | `show geodns` does not leak secrets (none expected) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Self-containment test fails | move the token to the geodns yang owner |
| Doctor false positive | refine probe phase/conditions |
| 3 fix attempts fail | STOP. Report. Ask user. |

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
- Once config is YANG-validated at commit, the reference's parse/duplicate/critical-file counters lose their job; the only reload signal worth a counter is success/failure of applying a committed config.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Register metrics via `ConfigureMetrics` into ze's registry | port the reference's standalone HTTP metrics server | Ze already exposes a metrics endpoint; counters are host-owned, incremented in-process by the engine goroutine |
| `show geodns` owner-owned via container merge | put it in central `show` | Self-containment invariant; deleting the plugin removes the command |
| Parse-time conflict detection (verifier over `listen-address`×`listen-port`; `ze:listener` where a list form applies); doctor checks bind *capability* only | doctor binds the live port to test it | Binding the ports the server already owns would always conflict (false positive); a leaf-list of addresses is not a single `ze:listener` list entry |
| Drop folder-only counters | keep them | They have no source once config is YANG |

## Known Limitations
- pprof `/debug` from the reference is not ported; ze has its own profiling story.

## RFC Documentation
- N/A (no wire protocol behavior in this spec).

## Implementation Summary

### What Was Implemented
- (fill during implementation)

### Bugs Found/Fixed
- (fill during implementation)

### Documentation Updates
- (fill during implementation)

### Deviations from Plan
- (fill during implementation)

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
| geodns observability without Sentry | functional test | `test/plugin/geodns-metrics.ci` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | NOTE | (none yet) | - | - |

### Fixes applied
- (none yet)

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

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/plugins/geodns`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete
- [ ] Self-containment test passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Implementation Audit filled
- [ ] Learned summary written
- [ ] **Commit A:** code + tests + spec + learned summary
- [ ] **Commit B:** `git rm` spec

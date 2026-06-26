# Spec: geodns-2-server

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-geodns-1-config |
| Phase | 5/5 |
| Updated | 2026-06-26 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `spec-geodns-0-umbrella.md`, `spec-geodns-1-config.md`
3. `/Users/thomas/Code/git.exa.net.uk/tech/dev/surfprotect/geodns/src/geodns/server.go` - reference handler
4. `internal/plugins/tftpserver/register.go` - SDK plugin + listener lifecycle
5. `internal/plugins/l2tpauthradius/coa.go` - UDP listener + per-packet source addr

## Task

Build the geodns DNS server: register the plugin, bind UDP+TCP listeners driven by
the YANG config (commit-time start/stop, graceful drain on shutdown), and answer
queries. For each query, extract the client IP per `client-ip-source`, select the
host set via the spec-1 longest-prefix matcher, and synthesise A/AAAA/SRV answers,
SOA/NS records with `ns1..ns9` glue, or a negative answer with the SOA in the
Authority section. Reuse `github.com/miekg/dns` for parsing/building and EDNS0.
No Sentry.

## Required Reading

### Architecture Docs
- [ ] `internal/plugins/tftpserver/register.go` - the `registry.Registration` + `sdk.NewWithConn` + `OnConfigure` start/stop pattern
  → Decision: mirror this skeleton; `RunEngine = runGeoDNSPlugin`, `ConfigRoots = ["service"]`.
  → Constraint: `OnConfigure` stops the old listeners then starts new ones (idempotent live reload).
- [ ] `internal/plugins/l2tpauthradius/coa.go` - UDP serve loop returning the remote addr
  → Constraint: `ReadFromUDP` gives the packet source IP, used when `client-ip-source` is `packet`/`edns0-then-packet`.
- [ ] `internal/core/textbuf/textbuf.go` - buffer-first formatting
  → Constraint: no `fmt.Sprintf` on the answer hot path; use `textbuf.Buffer` (see `ai/rules/no-sprintf-alloc.md`).
- [ ] `ai/rules/buffer-first.md` - allocation discipline
  → Constraint: encoding allocations are flagged by `/ze-find-alloc`.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7871.md` - EDNS0 Client Subnet
  → Constraint: read the `EDNS0_SUBNET` option's `Address` for the client IP when present.
- [ ] `rfc/short/rfc1035.md` - message format, SOA/NS, negative answers
  → Constraint: negative answer is NOERROR with SOA in Authority, not NXDOMAIN.

**Key insights:**
- The reference uses `dns.ServeMux` per zone and a per-request `recover()` so one bad query cannot kill the daemon. Preserve the recover; drop the Sentry capture inside it.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `/Users/thomas/Code/git.exa.net.uk/tech/dev/surfprotect/geodns/src/geodns/server.go` - `parseQuery`, `handleDNSRequest`, `resolve`, `NewUDP`/`NewTCP`/`NewMux`, EDNS0 extraction, SOA/NS synthesis, per-request recover.
- [ ] `/Users/thomas/Code/git.exa.net.uk/tech/dev/surfprotect/geodns/src/geodns/record.go` - `Answer()` string per record type.
- [ ] `/Users/thomas/Code/git.exa.net.uk/tech/dev/surfprotect/geodns/cmd/geodns/main.go` - listener lifecycle, drain timeout, signal handling.
- [ ] `internal/plugins/tftpserver/register.go` - ze SDK lifecycle to mirror.
- [ ] `internal/plugins/imageserver/register.go` - TCP `net.ListenConfig` bind pattern.

**Behavior to preserve:**
- Per-zone mux; a query name outside all configured zones is not served from host data.
- `nsID` detection: a query for `ns[1-9].<zone>` returns the configured nameserver A record.
- SOA query for the zone → SOA in Answer, NS in Authority, glue A in Additional.
- Unknown name / no record → NOERROR + SOA in Authority (negative answer).
- ANY returns all record types for the name; A/AAAA/SRV return their type.
- Per-request `recover()` so a panic in one query logs and continues.
- Graceful drain with a bounded timeout on shutdown (reference `drainTimeout = 5s`).

**Behavior to change:**
- Client IP source obeys `client-ip-source` (`edns0` / `packet` / `edns0-then-packet`) instead of EDNS0-only.
- Source→host-set selection uses the spec-1 longest-prefix matcher, not the reference's map+internal-list.
- SOA mname/rname/timers come from config (spec 1), not hard-coded EXA values. The serial is computed once per config generation at reload per `serial-mode` (auto-epoch = `max(commit Unix seconds, previous serial + 1)`, so it strictly increases even across sub-second reloads; auto-datetime = YYYYMMDDnn; fixed = the `serial` leaf) and stored in the snapshot, so it is stable across all queries in a generation and advances on change. The resolver carries the previous generation's serial forward to enforce the auto-epoch monotonic bump.
- The listener binds a UDP+TCP socket per `listen-address` (IPv4 + IPv6) on the shared `listen-port`; per-address bind is best-effort (a failed bind is logged, the rest still serve), mirroring dhcp/tftp per-interface binding.
- Sentry capture removed from the recover path and the bind-failure path; replaced by `slog` + metrics (spec 3).
- Reload distinguishes host-data changes (swap an `atomic.Pointer[resolverState]`, listeners stay bound) from endpoint changes (`listen-address` set / `listen-port` / `enabled`: stop and rebind the affected listeners, tftpserver-style). The published-state pattern mirrors `ntp`'s `globalState atomic.Pointer` read by the `show` handler.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A DNS query (UDP datagram or TCP stream) arrives on one of the bound `listen-address`:`listen-port` endpoints (v4 or v6).

### Transformation Path
1. `miekg/dns` server reads the message and dispatches to the per-zone handler.
2. Handler derives the client IP: if `client-ip-source` allows EDNS0 and an `EDNS0_SUBNET` option is present, use its `Address`; else (per mode) use the packet remote addr. The ECS network may be shorter than /32; it is matched by longest-prefix and the response echoes the ECS option with the scope-prefix-length actually used (RFC 7871).
3. Client IP → spec-1 matcher → selected host set (or none).
4. For each question: `ns[1-9]` glue check → host-set lookup → build A/AAAA/SRV answer; SOA/NS questions synthesise from config; misses produce SOA-in-Authority.
5. Response written; spec-3 counters incremented.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Network ↔ handler | `dns.ResponseWriter` / `dns.Msg` | [ ] |
| Handler ↔ config | atomic snapshot of `geodnsConfig` + matcher, swapped on reload | [ ] |
| Query ↔ client IP | `EDNS0_SUBNET.Address` or `ResponseWriter.RemoteAddr()` | [ ] |
| Plugin ↔ host | `sdk` conn, `OnConfigure`, `SignalContext` | [ ] |

### Integration Points
- `pkg/plugin/sdk` - `NewWithConn`, `OnConfigure`, `Run`, `SignalContext`.
- `github.com/miekg/dns` - `Server`, `ServeMux`, `Msg`, `EDNS0_SUBNET`, `RR` builders.
- spec-1 `source.go` matcher and `record.go` answer formatting.

### Architectural Verification
- [ ] No bypassed layers (config swapped atomically, handler reads snapshot)
- [ ] No unintended coupling (no import of other edge plugins)
- [ ] No duplicated functionality (miekg/dns for wire, spec-1 for matching)
- [ ] Zero-copy preserved where applicable (textbuf for answer strings)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | An atomic config snapshot lets reload swap host data without dropping the listener | reference rebinds; tftpserver stops/starts | Rebind listener on every reload (reference behavior) | reload functional test | unvalidated |
| A-2 | `miekg/dns` `Server` with an injected `PacketConn`/`Listener` binds like the reference `NewUDP`/`NewTCP` | reference `server.go`; vendored lib | Use raw `ReadFromUDP` loop (coa.go style) | server unit test | unvalidated |
| A-3 | The appliance can bind the default `:5300` without extra capability | reference default 5300 | Doctor check + capability note (spec 3) | functional test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Reload race: a query reads config mid-swap | data race under `-race` | `atomic.Pointer[resolverState]`; handler loads once per query |
| R-2 | TCP/UDP drain hangs on a wedged handler | shutdown blocks | bounded `ShutdownContext` (reference 5s) |
| R-3 | EDNS0 absent under `edns0` mode yields empty answers operators don't expect | "no answer" reports | default mode is `edns0-then-packet`; documented |
| R-4 | An endpoint change that does not rebind leaves the server on the old address | queries fail on the new port | reload classifies endpoint vs data changes; integration test covers a port change |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| commit `service geodns {enabled true}` | → | `runGeoDNSPlugin` binds; `show geodns` reports up | `test/plugin/geodns-show.ci` |
| A query from two different client IPs | → | matcher selects different host sets | `TestSourcePrecedence` Go integration test (`//go:build integration`) |
| SOA query for the zone | → | handler synthesises SOA/NS/glue | `TestHandlerSOAAndNS` (unit) + `TestResolveIntegration` (integration) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Enabled config committed | UDP+TCP bound on each `listen-address`:`listen-port` (v4 + v6); querying returns answers |
| AC-2 | A query, EDNS0 subnet `82.219.4.10`, `/32` source present | A record from the `/32` host set |
| AC-3 | A query, source matches only `0.0.0.0/0` | A record from the catch-all host set |
| AC-4 | A query for unknown name in zone | NOERROR, empty Answer, SOA in Authority |
| AC-5 | SOA query for zone | SOA in Answer, NS in Authority, glue A in Additional |
| AC-6 | Query for `ns1.<zone>` | A record = configured nameserver IP |
| AC-7 | `client-ip-source packet`, no EDNS0 | packet source IP selects the host set |
| AC-8 | Config reload changes a host's address | next query returns the new address without a dropped listener (atomic swap) |
| AC-9 | Malformed query bytes | handler recovers, daemon stays up |
| AC-10 | Config reload changes `listen-port` | listeners rebind to the new port; old ports released |
| AC-11 | `listen-address` has a v4 and a v6 address | both endpoints are bound and answer queries on `listen-port` |
| AC-12 | One `listen-address` fails to bind (e.g. ::1 on a v4-only host) | failure logged; the other addresses still serve |
| AC-13 | Two reloads within one second under `serial-mode auto-epoch` | the second generation's SOA serial = first + 1 (strictly increasing) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Queries a configured name | listener → handler → matcher → A answer | `TestResolveIntegration` (integration) |
| 2 | Two clients query the same name | per-source host sets → different answers | `TestSourcePrecedence` (integration) |
| 3 | Resolver asks for the zone SOA | synthesised SOA/NS/glue | `TestSOAIntegration` (integration) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHandlerAnswersPerSource` | `internal/plugins/geodns/server_test.go` | A/AAAA/SRV per source IP | |
| `TestHandlerSOAAndNS` | `internal/plugins/geodns/server_test.go` | SOA/NS/glue synthesis + negative answer | |
| `TestClientIPSourceModes` | `internal/plugins/geodns/server_test.go` | edns0 / packet / edns0-then-packet selection | |
| `TestReloadSwapsAnswers` | `internal/plugins/geodns/server_test.go` | atomic data reload, no dropped listener, `-race` clean | |
| `TestReloadRebindsOnPortChange` | `internal/plugins/geodns/server_test.go` | endpoint change stops + rebinds the listener | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| listen-port | 1..65535 | 65535 | 0 | 65536 |
| nameserver index ns[N] | 1..9 | 9 | 0 | 10 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `geodns-show` | `test/plugin/geodns-show.ci` | after commit, `show geodns` reports the listener bound | |
| `TestResolveIntegration` | `internal/plugins/geodns/server_integration_linux_test.go` | real query resolves per source (the `.ci` harness cannot send DNS; Go integration per `ai/rules/qemu-testing.md`) | |
| `TestSourcePrecedence` | `internal/plugins/geodns/server_integration_linux_test.go` | most-specific source wins | |
| `TestSOAIntegration` | `internal/plugins/geodns/server_integration_linux_test.go` | SOA/NS/glue + negative answer correct | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `geodns-coredns-forward` | `internal/plugins/geodns/server_integration_linux_test.go` | CoreDNS (skip if absent) | EDNS0 subnet rewrite path end-to-end | |

### Future (if deferring any tests)
- None.

## Files to Modify
- `internal/plugins/geodns/config.go` - hold an atomic resolver snapshot built by spec 1.
- `internal/component/plugin/all/all.go` - regenerated by `make generate`.

## Files to Create
- `internal/plugins/geodns/register.go` - `registry.Registration`, `runGeoDNSPlugin`, listener start/stop.
- `internal/plugins/geodns/server.go` - mux, handler, client-IP extraction, answer synthesis.
- `internal/plugins/geodns/socket_linux.go`, `socket_other.go` - bind helpers (mirror dhcp/tftp).
- `internal/plugins/geodns/server_test.go` (unit), `server_integration_linux_test.go` (`//go:build integration && linux`, real DNS exchange + optional CoreDNS).
- `test/plugin/geodns-show.ci` (listener-up via `show geodns`).

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

1. **Phase: Wiring (MANDATORY FIRST)** — `register.go` with `runGeoDNSPlugin` that binds a UDP+TCP listener per `listen-address` on configure; failing `test/plugin/geodns-show.ci` (asserts `show geodns` reports the listeners bound after commit).
   - Tests: `geodns-show`
   - Files: `register.go`
   - Verify: enabling config binds the sockets; `show geodns` stubbed → `.ci` fails until status is wired.
2. **Phase: query handler** — mux, question switch, host-set lookup, A/AAAA/SRV build.
   - Tests: `TestHandlerAnswersPerSource`
   - Files: `server.go`
3. **Phase: SOA/NS synthesis + negative answers** — config-driven SOA; ns[1-9] glue.
   - Tests: `TestHandlerSOAAndNS`
   - Files: `server.go`
4. **Phase: client-IP source modes** — EDNS0 vs packet selection.
   - Tests: `TestClientIPSourceModes`
   - Files: `server.go`
5. **Phase: reload + drain** — classify reload: data change → swap `atomic.Pointer`; endpoint/enabled change → stop + rebind listeners; bounded shutdown drain.
   - Tests: `TestReloadSwapsAnswers`, `TestReloadRebindsOnPortChange`
   - Files: `register.go`, `server.go`
6. **Functional tests** → `.ci` per user story + optional CoreDNS interop.
7. **Full verification** → `make ze-verify` (incl. `-race`).
8. **Complete spec** → learned summary + two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Feature completeness | A/AAAA/SRV/SOA/NS/glue/negative parity with reference |
| Correctness | source precedence; ns[1-9] detection; recover on panic |
| Data flow | handler reads an atomic snapshot; reload never drops the listener |
| Buffer-first | answer formatting via textbuf, no `fmt.Sprintf` hot path |
| No Sentry | recover + bind-failure paths use slog/metrics only |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Listener binds on enable | `test/plugin/geodns-show.ci` passes (listener-up) |
| Race-clean reload | `go test -race ./internal/plugins/geodns` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Untrusted input | malformed packets recovered, never panic the process |
| Amplification | only configured zones answered; no recursion; bounded response |
| Resource | per-request work bounded; no unbounded goroutine growth |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Bind fails in test | check port/cap; use ephemeral port like reference |
| Wrong answer | re-check matcher (spec 1) vs handler |
| Race detected | fix snapshot swap |
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
- Keeping the resolver state behind an `atomic.Pointer` lets the listener stay bound across reloads; only the data pointer swaps, matching ze's commit-driven model better than the reference's rebind.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Reuse miekg/dns `Server`/`ServeMux` | raw `ReadFromUDP` loop (coa.go) | Free message parse/build + EDNS0; matches reference structure |
| Atomic snapshot reload | rebind listeners per commit (reference) | No dropped queries on commit; simpler lifecycle |
| Keep per-request recover | let panics propagate | One bad query must not take the daemon down |

## Known Limitations
- No DNSSEC, no zone transfer (AXFR) serving — out of scope, as in the shipped reference.

## RFC Documentation
Add `// RFC 7871 Section 7.1.3` above EDNS0 client-subnet extraction and `// RFC 1035 Section 6.1` above negative-answer SOA-in-Authority handling.

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
| Per-source answers identical to geodns | integration test | `TestSourcePrecedence` (`server_integration_linux_test.go`) |

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
- [ ] AC-1..AC-9 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/plugins/geodns`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete

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

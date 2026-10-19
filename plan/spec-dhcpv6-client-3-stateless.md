# Spec: dhcpv6-client-3-stateless

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | `plan/pre-release/spec-dhcpv6-client-1-gap-annotations.md` |
| Phase | 3/4 of `plan/spec-dhcpv6-client-0-umbrella.md` |
| Handoff | - |
| Updated | 2026-10-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Stateless DHCPv6 (RFC 9915 Section 6.1) gives a node configuration, such as
DNS servers, with no address or prefix: the client sends an
Information-request, takes the Reply, and refreshes it when the Information
Refresh Time runs out. Ze has no stateless mode: every exchange its client
starts is a Solicit, Request, Renew, Rebind, Decline or Release. The owner put
stateless mode in scope on 2026-10-19. This child adds a `stateless` leaf to
the `dhcpv6` container, runs the Information-request exchange with its
INF_MAX_RT and refresh timers, applies the DNS servers it returns, re-runs it
on a link move, and processes INF_MAX_RT wherever it arrives. It closes the 12
`{gap}` rows RFC9915-18.2.6-1, -2, -4, RFC9915-18.2.9-4, RFC9915-18.2.12-3,
RFC9915-21.23-1, -5, -6, -7, RFC9915-21.25-2, -3, -4.

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` - DHCPv6 paragraph and the `dhcpv6` YANG description
  → Constraint: `dhcpv6` carries `ze:backend "netlink"`; the new leaf inherits it
  → Constraint: resolvers reach the host through `setV6Resolvers` (`internal/plugins/iface/dhcp/resolv_linux.go`); stateless mode feeds the same call
- [ ] `ai/patterns/config-option.md`, `ai/rules/config.md` - YANG leaf rules
  → Constraint: a new leaf takes the strongest native validation and a description naming its effect; the refusal of `stateless` beside `pd` says what is wrong and how to fix it (`ai/rules/user-facing-errors.md`)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9915.md` - the 12 rows, RFC9915-21.23-2 (IRT never requested outside Information-request), RFC9915-21.7-1 (ORO in Information-request), RFC9915-16.12-1
  → Constraint: today one Option Request option (`requestOptions`) serves every message; RFC9915-21.23-2 forbids the Information Refresh Time option in any ORO but the Information-request's, so the ORO becomes per message kind
- [ ] `rfc/full/rfc9915.txt` - Sections 18.2.6, 18.2.9, 18.2.10, 18.2.10.4, 18.2.10.5, 18.2.12, 21.23, 21.25, 7.6
  → Constraint: Section 18.2.6 "The client MUST include an Elapsed Time option", "MUST include an Option Request option ... to request the INF_MAX_RT option ..., the Information Refresh Time option ..., and any other options the client is interested in receiving", "The first Information-request message from the client on the interface MUST be delayed by a random amount of time between 0 and INF_MAX_DELAY"; IRT INF_TIMEOUT 1 s, MRT INF_MAX_RT (default 3600 s), MRC 0, MRD 0
  → Constraint: Section 18.2.6 "The client SHOULD include a Client Identifier option"; ze includes it, so the Reply may carry client-specific options
  → Constraint: Section 21.23 "If the Reply to an Information-request message does not contain this option, the client MUST behave as if the option with the value IRT_DEFAULT was provided" (86400 s); "A client MUST use the refresh time IRT_MINIMUM if it receives the option with a value less than IRT_MINIMUM" (600 s); refresh delayed by RAND times INF_MAX_DELAY
  → Constraint: Section 21.25 "A DHCP client MUST ignore any INF_MAX_RT option values that are less than 60 or more than 86400"; a valid value sets the client's INF_MAX_RT
  → Constraint: Section 18.2.9 "MUST process an included INF_MAX_RT option" in an Advertise, and Section 18.2.10 processes it in every Reply "even if the message contains a Status Code option indicating a failure", in stateful mode too
  → Constraint: Section 18.2.10.5 "the client SHOULD stop using the previously received configuration information" when a Reply omits an option it returned before

**Key insights:**
- The vendored library already parses the Information Refresh Time option (`MessageOptions.InformationRefreshTime`); INF_MAX_RT (code 83) has an option code and no typed parser, so it is parsed as ze parses SOL_MAX_RT today (`v6SolMaxRTOption`).
- The worker's `solMaxRT` field is the model for a per-interface `infMaxRT`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/iface/dhcp/dhcp_v6_linux.go` - `runV6` loops `acquire` then `maintain`; `shutdown` sends Release
- [ ] `internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go` - `requestOptions` (DNS Recursive Name Server, SOL_MAX_RT), `v6SolMaxRTOption`, `noteSolMaxRT`, `newV6Message`
- [ ] `internal/plugins/iface/dhcp/dhcp_v6_attach_linux.go` - `noteMove`, `moveDue`, `moveLast`
- [ ] `internal/plugins/iface/dhcp/dhcp_linux.go` - `dHCPConfig` (`PDRequested`, `PDLength`, `DUID`)
- [ ] `internal/component/iface/config.go` - parse of the `dhcpv6` container (`PDRequested`, `DUID`)
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `container dhcpv6`: `enabled`, `pd { length }`, `duid`

**Behavior to preserve:** with `stateless` absent or false, every stateful message, its ORO contents except as RFC9915-21.23-2 already requires, and every timer stay as they are.

**Behavior to change:**
- the ORO is chosen per message kind: Solicit, Request, Renew and Rebind keep DNS and SOL_MAX_RT; Information-request carries DNS, INF_MAX_RT and Information Refresh Time
- INF_MAX_RT in an Advertise or Reply is recorded in both modes

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config `interfaces ethernet <name> unit <n> dhcpv6 { enabled true; stateless true; }`.

### Transformation Path
1. YANG accepts the leaf; a `must` refuses it beside `pd`.
2. `internal/component/iface/config.go` sets a `Stateless` field of the client config.
3. `runV6` runs, for a stateless worker, an information loop in place of `acquire` and `maintain`: delay, Information-request exchange, apply the Reply, wait for the refresh time or a move, repeat.
4. A Reply's DNS servers go to `setV6Resolvers`; a Reply without them clears them.
5. On Stop the worker clears its resolvers and sends nothing (no binding to release).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to plugin | new `Stateless` field in `dHCPConfig` | No |
| Plugin to host resolvers | `setV6Resolvers` | No |
| Plugin to wire | `v6Link.exchange` | No |

### Integration Points
- `runExchange` - the Information-request runs on it with MRC 0 and MRD 0, so it never stops until answered or stopped.
- `noteMove` - a stateless worker owes an Information-request on a move, under the shared 30 s limit.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | one exchange engine, one resolver path |
| No unintended coupling (components stay isolated) | Yes | YANG and config parse plus the plugin |
| No duplicated functionality (extends existing, does not recreate) | Yes | INF_MAX_RT handled like SOL_MAX_RT |
| Zero-copy preserved where applicable (refs, not copies) | Yes | no new copy of received data |
| Registration over hardcoding, outbound | Yes | the leaf is discovered by the schema |
| Registration over hardcoding, inbound | Yes | searched `requestOptions` and the config parse; both are the DHCPv6 client's own |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | stateless mode is chosen by an explicit leaf, not by the RA O flag | umbrella question Q2; RFC 9915 leaves the trigger out of scope | a second trigger is owed | owner answer | confirmed: owner decision 2026-10-19, "ok" (umbrella Q2: explicit `stateless` leaf, refused beside `pd`) |
| A-2 | Kea 2.6.2 answers Information-request with `dns-servers` and `information-refresh-time` from its `option-data` | `pkt6-infrequest-received` in the image; Kea option table lists `information-refresh-time` (32) | interop needs another peer | `dhcpv6-client-stateless-kea` | unvalidated |
| A-3 | `stateless` is the leaf name `ai/rules/config.md` allows | the existing sibling names `enabled`, `pd`, `duid` | rename before implementation | read `ai/rules/config.md` naming at implementation start | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | the per-kind ORO changes a stateful message | `TestRFC9915Section217OptionRequestEveryMessage` and the Solicit/Request contents tests | those tests stay green unchanged |
| R-2 | the interop needs over 10 minutes (IRT_MINIMUM 600 s) | wall time | accepted (umbrella Q4); Kea sends `information-refresh-time` 600 |
| R-3 | an Information-request that never ends blocks Stop | rig | `runExchange` ends on ctx as every exchange does |
| R-4 | a Reply with no DNS while the operator relies on static resolvers | resolver test | only the DHCPv6-learned set is cleared, as `setV6Resolvers(nil)` does at lease loss today |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a stateless interface without DNS, or a stateful one whose messages change |
| How is it reverted? | single commit revert; a config carrying `stateless` is then refused by the schema, so the revert needs the leaf removed from configs |
| Who else touches this path? | child 2 (`noteMove`), child 4 (Information-request in response to Reconfigure) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| config `dhcpv6 { stateless true }` | → | information loop in `runV6` | `test/plugin/iface-dhcpv6-stateless-config.ci` |
| a stateless worker on the rig | → | Information-request exchange | `TestRFC9915Section1826InformationRequestContents` |
| a real server | → | Information-request and Reply on the wire | `dhcpv6-client-stateless-kea` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `stateless true` committed | the worker sends an Information-request, and never a Solicit, Request, Renew, Rebind, Decline or Release |
| AC-2 | first Information-request on the interface | its first transmission waits a delay drawn in [0, 1 s] (RFC9915-18.2.6-4) |
| AC-3 | every Information-request | carries Elapsed Time updated per retransmission (RFC9915-18.2.6-1), the worker's Client Identifier, an ORO holding DNS Recursive Name Server, INF_MAX_RT and Information Refresh Time (RFC9915-18.2.6-2, RFC9915-21.23-1, RFC9915-21.25-2), and no IA option and no Server Identifier |
| AC-4 | no answer | retransmits from INF_TIMEOUT 1 s doubling to INF_MAX_RT, with no transmission count or duration limit |
| AC-5 | Reply carrying DNS servers | `setV6Resolvers` receives them |
| AC-6 | later Reply carrying no DNS servers | the DHCPv6-learned resolvers are cleared |
| AC-7 | Reply with no Information Refresh Time | the next Information-request is due 86400 s later (RFC9915-21.23-5) |
| AC-8 | Reply with an Information Refresh Time of 300 s | the next one is due 600 s later (RFC9915-21.23-6) |
| AC-9 | Reply with 7200 s | due 7200 s later, its first transmission delayed by a draw in [0, 1 s] (RFC9915-21.23-7) |
| AC-10 | INF_MAX_RT 59, 86401 in a Reply | ignored, MRT stays as it was (RFC9915-21.25-3) |
| AC-11 | INF_MAX_RT 60 or 86400 in a Reply, and 120 in an Advertise of a stateful worker | the worker's INF_MAX_RT takes the value (RFC9915-21.25-4, RFC9915-18.2.9-4) |
| AC-12 | link down then up on a stateless worker | one Information-request, under the shared 30 s limit (RFC9915-18.2.12-3) |
| AC-13 | `stateless true` beside `pd { }` | the commit is refused with a message naming both and saying to remove one |
| AC-14 | stateful worker | Solicit, Request, Renew and Rebind ORO unchanged and never carry Information Refresh Time (RFC9915-21.23-2 still proven) |
| AC-15 | Stop of a stateless worker | resolvers cleared, no message sent |
| AC-16 | the 12 rows | no `{gap}`; each a positive and a negative tagged test with a discrimination record; RFC9915-21.7-1 gains an Information-request case; `Support remaining` lowered by 12, Information-request removed from its list |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | configures an interface that takes addresses by SLAAC and DNS by DHCPv6 | YANG, config parse, Information-request, Reply, resolvers | `dhcpv6-client-stateless-kea` |
| 2 | asks for `stateless` and `pd` together | YANG `must` | `test/plugin/iface-dhcpv6-stateless-pd-refused.ci` |

### Planned Goal Validation
<!-- The evidence each goal owes at closure (ai/rules/interop-and-goal-validation.md); /ze-close copies it into the closure table. -->
| Goal | Evidence owed |
|------|---------------|
| an operator gets DNS from DHCPv6 with no lease | `dhcpv6-client-stateless-kea` passing: Kea logs an INFORMATION-REQUEST and no SOLICIT, and ze's resolver set holds Kea's `dns-servers` |
| the configuration is refreshed on the server's schedule | the same scenario: a second INFORMATION-REQUEST between 600 s and 601 s after the first, with the recorded red when the refresh timer is reverted |
| the operator is told when the config is contradictory | `iface-dhcpv6-stateless-pd-refused.ci` matching the refusal text |
| the 12 RFC rows are met | discrimination records accepted by `./le rfc check` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC9915Section1826InformationRequestContents` | `internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go` | AC-1, AC-3 | |
| `TestRFC9915Section1826FirstInformationRequestDelay` | `internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go` | AC-2 | |
| `TestRFC9915Section1826InformationRequestRetransmission` | same | AC-4 | |
| `TestDHCPv6StatelessResolvers` | same | AC-5, AC-6, AC-15 | |
| `TestRFC9915Section2123RefreshTime` | same | AC-7, AC-8, AC-9 | |
| `TestRFC9915Section2125InfMaxRT` | same | AC-10, AC-11 | |
| `TestRFC9915Section18212InformationRequestOnMove` | `internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go` | AC-12 | |
| `TestRFC9915Section217OptionRequestEveryMessage` | `internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go` | AC-14, extended to Information-request | |
| `TestParseDHCPv6Stateless` | `internal/component/iface/config_test.go` | the leaf reaches `dHCPConfig` | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| INF_MAX_RT option | 60-86400 | 60, 86400 | 59 | 86401 |
| Information Refresh Time | at least 600 | 600 | 599 (used as 600) | N/A (0xffffffff is infinity, Section 21.23) |
| first delay | 0 to 1 s | 1 s | N/A | above 1 s |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-dhcpv6-stateless-config` | `test/plugin/iface-dhcpv6-stateless-config.ci` | the leaf commits and the client starts in stateless mode | |
| `iface-dhcpv6-stateless-pd-refused` | `test/plugin/iface-dhcpv6-stateless-pd-refused.ci` | the contradictory config is refused with the fix named | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `dhcpv6-client-stateless-kea` | `test/interop/scenarios/` | Kea 2.6.2 | Kea with no address pool for ze and `option-data` `dns-servers` and `information-refresh-time` 600; the checker reads Kea's log for INFORMATION-REQUEST and no SOLICIT, reads ze's resolver set, then waits for the refresh. Red phase: with the refresh timer reverted no second INFORMATION-REQUEST arrives | |

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang` - `leaf stateless` with its `must`
- `internal/component/iface/config.go` - parse into the client config
- `internal/plugins/iface/dhcp/dhcp_linux.go` - `dHCPConfig.Stateless`
- `internal/plugins/iface/dhcp/dhcp_v6_linux.go` - information loop in `runV6`, `infMaxRT` field, shutdown branch
- `internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go` - Information-request builder, per-kind ORO, INF_MAX_RT parse in Advertise and Reply
- `internal/plugins/iface/dhcp/dhcp_v6_attach_linux.go` - `noteMove` for a stateless worker
- `docs/features/interfaces.md` - stateless paragraph, source anchors
- `features/interfaces.md`, `rfc/short/rfc9915.md`, `docs/features/rfc-status.md`, `docs/architecture/testing/interop.md`

## Files to Create
- `test/plugin/iface-dhcpv6-stateless-config.ci`, `test/plugin/iface-dhcpv6-stateless-pd-refused.ci`
- `test/interop/scenarios/dhcpv6-client-stateless-kea/` - `kea-dhcp6.conf`, `ze.conf`, and its checker beside `internal/le/interoplab/bgp/check_dhcpv6.go`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-iface-conf.yang` `stateless` |
| YANG validation constraints | Yes | boolean type; `must "not(../pd)"`-style refusal with an error message |
| YANG custom validators | No | the native `must` suffices |
| CLI commands/flags | No | none |
| CLI grammar (keyword before value) | N-A | none |
| Editor autocomplete | Yes | automatic for a boolean leaf |
| Functional test for new RPC/API | Yes | the two `.ci` files above |
| Pipe completeness | N-A | no output |
| Env var registration | N-A | not under `environment/` |
| Doctor check for runtime dependencies | No | same socket |
| Prometheus counters/metrics | No | none in this client |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/interfaces.md` |
| 2 | Config syntax changed? | Yes | YANG description; `docs/features/interfaces.md` |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | same plugin |
| 6 | Has a user guide page? | No | none |
| 7 | Wire format changed? | No | summary carries formats |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9915.md`, `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | No | none |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/features/interfaces.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/features/interfaces.md` DHCPv6 example gains the leaf |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- YANG leaf, parse, `dHCPConfig` field, the two `.ci` files and `TestParseDHCPv6Stateless`; they fail.
2. **Phase: Information-request** -- builder, per-kind ORO, loop, resolvers; AC-1 to AC-6, AC-14, AC-15.
3. **Phase: Timers** -- IRT and INF_MAX_RT; AC-7 to AC-11.
4. **Phase: Move** -- AC-12.
5. **Phase: Interop, ledger and docs** -- scenario, red phase, tags, discrimination records, Meta, pages.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has an implementation at file:line |
| Correctness | INF_MAX_RT is recorded even from a Reply with a failure status (Section 18.2.10) |
| Naming | YANG leaf kebab-case; error text names `stateless` and `pd` |
| Data flow | resolvers only through `setV6Resolvers` |
| Rule: rfc-compliance | each new function quotes its Section 18.2.6, 21.23 or 21.25 sentence |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| stateless mode | `TestRFC9915Section1826InformationRequestContents` passes |
| 12 rows closed | grep of the 12 ids shows no `{gap` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | INF_MAX_RT and IRT parse refuse a short option body; a refresh time of 0xffffffff is infinity, never an overflow |
| Resource exhaustion | a Reply storm cannot make the loop spin: the next exchange is due no earlier than IRT_MINIMUM |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| explicit `stateless` leaf | follow the RA M and O flags | RFC 9915 leaves the trigger out of scope; an explicit leaf is one fact the operator sets (umbrella Q2) |
| ORO per message kind | keep one ORO and add IRT everywhere | RFC9915-21.23-2 forbids IRT outside Information-request |
| record INF_MAX_RT in stateful mode too | only in stateless mode | Section 18.2.9 and 18.2.10 oblige it in Advertise and every Reply |
| refuse `stateless` beside `pd` | let `stateless` win silently | a contradictory config is refused, never guessed |

## Known Limitations
- The RA O-flag trigger is not built (A-1).
- NTP server, domain search list and other options are not requested; RFC9915-18.2.10.5-1 (NTP) stays `{feature-declined}`.

## RFC Documentation (Scope: protocol)

Add `// RFC 9915 Section 18.2.6: "..."`, `// RFC 9915 Section 21.23: "..."`, `// RFC 9915 Section 21.25: "..."` above the implementing statements, and section references at the calls.

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
- [ ] AC-1..AC-N all demonstrated
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
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/spec-dhcpv6-client-3-stateless.md` only, in the same `./le commit create` script

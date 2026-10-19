# Spec: dhcpv6-client-0-umbrella

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | 0/4 |
| Handoff | - |
| Updated | 2026-10-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The owner put four DHCPv6 client features in scope on 2026-10-19, verbatim:
"DHCPv6 Confirm, stateless mode, Reconfigure and authentication: in scope". He
had said earlier that the "feature can go post release", so the three feature
specs sit in `plan/`. The DHCPv6 client (`internal/plugins/iface/dhcp/`,
documented in `docs/features/interfaces.md`, "Key Design Decisions") implements
RFC 9915 stateful IA_NA and IA_PD with Solicit, Request, Renew, Rebind, Release
and Decline. It sends no Confirm, no Information-request and no Reconfigure
Accept, reads no Authentication option, and discards every Reconfigure.

This umbrella holds what the four children share: the split, the order, the
row classification, the decisions that bind more than one child, and the
questions the owner answers once.

### What "authentication" means in RFC 9915

RFC 9915 Section 20 defines two mechanisms: IPsec between relay agents and
servers (Section 20.1, a role ze does not fill), and the Reconfiguration Key
Authentication Protocol (RKAP, Section 20.4) carried in the Authentication
option (Section 21.11) with the replay detection of Section 20.3. Section 1.1
says RFC 8415 already "obsoleted a small number of mechanisms: delayed
authentication, lifetime, and timer hints sent by a client", so delayed
authentication was removed by RFC 8415, not by RFC 9915, and RFC 9915 carries
none of it. Section 20.4 says "RKAP is used (initiated by the server) only if
the client and server have negotiated to use Reconfigure messages." For a
client, then, "authentication" is exactly RKAP: record the reconfigure key a
server sends in a Reply, run replay detection on every Authentication option
from that server, and validate the HMAC-MD5 of each Reconfigure. It has no use
without Reconfigure, which is why authentication and Reconfigure are one child.

### The spec set

| Spec | Bucket | Delivers | Depends |
|------|--------|----------|---------|
| `plan/pre-release/spec-dhcpv6-client-1-gap-annotations.md` | pre-release | The 23 rows of the four features re-annotated `{gap}`, the Meta rows and the Purpose paragraph corrected | - |
| `plan/spec-dhcpv6-client-2-confirm.md` | plan | Confirm on a possible link move of an addresses-only lease | child 1 |
| `plan/spec-dhcpv6-client-3-stateless.md` | plan | Stateless DHCPv6: Information-request, Information Refresh Time, INF_MAX_RT | child 1 |
| `plan/spec-dhcpv6-client-4-reconfigure.md` | plan | Reconfigure Accept, the always-listening socket, RKAP authentication and replay detection, Reconfigure-triggered Renew, Rebind and Information-request | children 1 and 3 |

Order: child 1 first and now, because the `{feature-declined}` annotation says
the owner declined a feature he has since put in scope, so the ledger states a
scope decision that no longer exists (`ai/rules/rfc-compliance.md`: "An
OPTIONAL feature outside the owner's agreed scope MUST be recorded as an
implementation gap"; an in-scope one that ze does not meet is a conformance
gap). Children 2 and 3 are independent of each other. Child 4 follows child 3,
because a Reconfigure whose Reconfigure Message option says 11 asks for an
Information-request, which child 3 builds.

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` - DHCPv6 client paragraphs under "Key Design Decisions"
  → Constraint: the page states "A lease of addresses alone sends nothing on a move, because Ze does not send Confirm"; child 2 rewrites that sentence in the same change that adds Confirm
  → Constraint: every DHCPv6 paragraph carries `<!-- source: -->` anchors to `dhcp_v6_linux.go`, `dhcp_v6_exchange_linux.go`, `dhcp_v6_attach_linux.go`, `dhcp_v6_lease_linux.go`; each child updates the anchors of the functions it adds
- [ ] `docs/contributing/rfc-conformance-gates.md` - "The feature-declined annotation", `checkGapCountAgreement`, "Proof is monotonic"
  → Decision: `{feature-declined}` requires that "Ze declined X"; after the owner's 2026-10-19 decision that fact is false for these four features, so the rows become `{gap}` (an unconditional or now-binding obligation ze does not meet)
  → Constraint: a `{gap}` row carries no `positive` or `negative` tag; each feature child removes the `{gap}` and adds both polarities with a discrimination record (`./le rfc discriminate-record`)
  → Constraint: the `Support remaining` cell's spelled number before MUST must equal the `{gap}` plus `{partial}` rows (`spelledGapCount`, `internal/le/rfc/check_status.go`); every child that closes gaps rewrites that number
- [ ] `docs/architecture/testing/interop.md` - Kea row (`kea-dhcp6.conf`, `test/interop/Dockerfile.kea`)
  → Constraint: the Kea image is Alpine 3.21 `kea-dhcp6` 2.6.2; scenarios are named, never numbered

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9915.md` - 126 checklist rows, 39 `{feature-declined}`
  → Constraint: only 23 rows belong to the four in-scope features (table below); the other 17 `{feature-declined}` rows (delegated prefix use, DUID-LLT and DUID-EN, Client Identifier omission, non-zero MRC, NTP, User Class, Vendor-specific) stay as they are
  → Constraint: the Purpose paragraph says RFC 9915 "removes IA_TA, Server Unicast and the Reconfigure Accept negotiation"; Appendix A lists IA_TA, the Server Unicast option and the UseMulticast status code only, and Section 21.20 still defines Reconfigure Accept. Child 1 corrects it
- [ ] `rfc/full/rfc9915.txt` - Sections 15, 16.11, 18.2.3, 18.2.6, 18.2.9, 18.2.10.3, 18.2.10.4, 18.2.11, 18.2.12, 20, 21.11, 21.19, 21.20, 21.23, 21.25, Appendix B
  → Constraint: Section 16.11 lists the six Reconfigure discard conditions; Section 20.4.3 "the client computes an HMAC-MD5 over the Reconfigure message, with zeroes substituted for the HMAC-MD5 field"

**Key insights:**
- The 23 rows split 4 / 12 / 7 over children 2, 3 and 4.
- `rfc/extraction/rfc9915.json` needs no change: the excluded sites of these sections are all server or relay role (`binds-another-role`) or duplicates.
- Kea 2.6.2 receives Confirm and Information-request (statistics `pkt6-confirm-received`, `pkt6-infrequest-received` in the image) and never sends Reconfigure (no `pkt6-reconfigure-sent`; the ISC tracker issue 199 still tracks integrating the 2018 Summer of Code Reconfigure work). Reconfigure interop needs another server.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/iface/dhcp/dhcp_v6_linux.go` - `runV6` worker: `identify`, `acquire` (Solicit, Request), `maintain` (T1 Renew, T2 Rebind, expiry), `shutdown` (Release)
- [ ] `internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go` - `v6Link` (one `nclient6.Client` per exchange), `runExchange` (RFC 9915 Section 15 engine), constants including the unused `v6CnfMaxDelay`, `v6CnfTimeout`, `v6CnfMaxRT`, `v6CnfMaxRD`, `requestOptions` (one ORO for every message), `newV6Message`
- [ ] `internal/plugins/iface/dhcp/dhcp_v6_attach_linux.go` - `linkMoved`, `noteMove` (DAD re-run, Rebind owed only with a delegated prefix), `rebindAfterMove` (Confirm parameters, 30 s rate limit)
- [ ] `vendor/github.com/insomniacslk/dhcp/dhcpv6/nclient6/client.go` - `receiveLoop` drops every message whose transaction id is not pending; `NewWithConn` accepts a caller's `net.PacketConn`

**Behavior to preserve:** every existing exchange, message content, timer and log line of the stateful client when none of the new leaves is set.

**Behavior to change:** listed per child.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `interfaces ... unit ... dhcpv6 { }` (YANG `internal/component/iface/yang/ze-iface-conf.yang`), parsed in `internal/component/iface/config.go` into the plugin's `dHCPConfig` (`internal/plugins/iface/dhcp/dhcp_linux.go`).
- Wire: UDP 546 on the unit's kernel device link-local address.
- Link events: `iface.LinkEvent` on the worker's subscription.

### Transformation Path
1. Config parse sets the worker's mode (stateful, stateless) and willingness to accept Reconfigure.
2. The worker goroutine runs every exchange through `runExchange`.
3. Replies update the lease (`v6Lease.apply`), resolvers (`setV6Resolvers`) and timers.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to plugin | `dHCPConfig` fields filled by `internal/component/iface/config.go` | No |
| Plugin to iface component | `iface.ReplaceAddressWithLifetime`, `iface.RegisterLeasedAddresses`, event bus `dhcp-expired` | No |
| Plugin to wire | `v6Link` over the vendored `nclient6` socket | No |

### Integration Points
- `runExchange` - every new exchange (Confirm, Information-request, Reconfigure-triggered) runs through it, no second engine.
- `noteMove` - Confirm (child 2) and Information-request (child 3) are chosen there.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | every child's exchange uses `runExchange` and `v6Link` |
| No unintended coupling (components stay isolated) | Yes | all change is inside `internal/plugins/iface/dhcp/` plus two YANG leaves and their parse |
| No duplicated functionality (extends existing, does not recreate) | Yes | Confirm reuses the move rate limit and the Confirm constants already declared |
| Zero-copy preserved where applicable (refs, not copies) | Yes | RKAP validates the received datagram bytes in place, with the HMAC field zeroed in a pooled copy (child 4) |
| Registration over hardcoding, outbound | Yes | no new plugin, command or registry entry; YANG leaves are discovered by the schema |
| Registration over hardcoding, inbound | Yes | searched `requestOptions`, `replyMatcher`, `v6Status`: they gain message kinds of the same plugin, no foreign list learns a name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Kea 2.6.2 answers Confirm and Information-request | statistics names `pkt6-confirm-received`, `pkt6-infrequest-received` in the `kea-interop` image's binaries | children 2 and 3 need another peer | child 2 and 3 interop scenarios | unvalidated |
| A-2 | Kea 2.6.2 never sends Reconfigure | no `pkt6-reconfigure-sent` statistic in the image; ISC issue 199 open-ended | Kea could serve child 4 | child 4 research step | unvalidated |
| A-3 | OpenWrt odhcpd sends Reconfigure (msg-type Renew) with RKAP HMAC-MD5 to a client that sent Reconfigure Accept, when its interface address list changes | `send_reconf`, `handle_addrlist_change` in `openwrt/odhcpd` `src/dhcpv6-ia.c` | child 4 has no interop peer | child 4 Dockerfile and scenario | unvalidated |
| A-4 | The 17 other `{feature-declined}` rows are outside the owner's 2026-10-19 decision | the decision names four features only | they would also become `{gap}` | owner confirmation (umbrella question Q5) | confirmed: owner decision 2026-10-19, "ok" |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | odhcpd does not build or run outside OpenWrt (libubox, uci, ubus) | Dockerfile build fails | build with ubus disabled; if no route works, stop and ask the owner which peer, never skip the interop |
| R-2 | Child 4 changes socket lifetime and breaks the per-exchange tests | `TestDHCPv6LinkOneDatagramPerExchange` red | the persistent link exists only while Reconfigure is accepted (child 4 decision) |
| R-3 | The stateless interop needs over 10 minutes because IRT_MINIMUM is 600 s | scenario wall time | accepted cost, recorded in child 3 (question Q4) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a DHCPv6 client that loses or keeps a lease wrongly after a link move or a Reconfigure; resolvers wrong in stateless mode |
| How is it reverted? | each child is a single-commit revert; the two new leaves default to off |
| Who else touches this path? | `internal/component/iface` (address ownership), the `iface-dhcp` DHCPv4 client in the same package |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| link down then up with an addresses-only lease | → | Confirm exchange (child 2) | `TestRFC9915Section18212ConfirmOnMove` |
| `dhcpv6 { stateless true }` | → | Information-request exchange (child 3) | `test/plugin/iface-dhcpv6-stateless-config.ci` |
| `dhcpv6 { reconfigure-accept true }` and an authenticated Reconfigure | → | Reconfigure-triggered Renew (child 4) | `TestRFC9915Section18211ReconfigureTriggersRenew` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | child 1 committed | the 23 rows carry `{gap}`, `./le rfc check` passes |
| AC-2 | children 2, 3, 4 closed | none of the 23 rows carries `{gap}`; each carries a positive and a negative test with a discrimination record, and `Support remaining` no longer lists Confirm, Information-request, Reconfigure or the Authentication option |
| AC-3 | every child closed | `docs/features/interfaces.md` describes Confirm, stateless mode and Reconfigure as the code does |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | unplugs and replugs the cable of a DHCPv6 interface holding addresses only | link event, `noteMove`, Confirm, Reply | `dhcpv6-client-confirm-kea` |
| 2 | configures a unit to take only DNS from DHCPv6 | YANG, `dHCPConfig`, Information-request, `setV6Resolvers` | `dhcpv6-client-stateless-kea` |
| 3 | lets the server push a renumbering | Reconfigure, RKAP check, Renew | `dhcpv6-client-reconfigure-odhcpd` |

### Planned Goal Validation
| Goal | Evidence owed |
|------|---------------|
| the four in-scope features work against another implementation | the three interop scenarios above passing, each with its recorded red phase (children 2, 3, 4 Planned Goal Validation) |
| the RFC ledger is honest at every step | child 1's `./le rfc check`; each child's discrimination records |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| per child | children 2, 3, 4 | each child's ACs | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| INF_MAX_RT option | 60-86400 | 60 and 86400 | 59 | 86401 |
| Information Refresh Time | IRT_MINIMUM 600 floor | 600 | 599 (raised to 600) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| per child | `test/plugin/` | config acceptance and refusal of the new leaves | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `dhcpv6-client-confirm-kea` | `test/interop/scenarios/` | Kea 2.6.2 | Confirm kept on Success, rediscovery on NotOnLink | |
| `dhcpv6-client-stateless-kea` | `test/interop/scenarios/` | Kea 2.6.2 | Information-request, resolvers, refresh at the refresh time | |
| `dhcpv6-client-reconfigure-odhcpd` | `test/interop/scenarios/` | OpenWrt odhcpd | RKAP key recorded, authenticated Reconfigure answered with Renew | |

## Files to Modify
- `internal/plugins/iface/dhcp/dhcp_v6_linux.go`, `dhcp_v6_exchange_linux.go`, `dhcp_v6_attach_linux.go`, `dhcp_linux.go` - per child
- `internal/component/iface/yang/ze-iface-conf.yang`, `internal/component/iface/config.go` - leaves of children 3 and 4
- `rfc/short/rfc9915.md`, `docs/features/rfc-status.md` (generated), `docs/features/interfaces.md`, `features/interfaces.md`

## Files to Create
- the four child specs listed in the Task table
- `test/interop/Dockerfile.odhcpd` (child 4)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-iface-conf.yang`: `stateless` (child 3), `reconfigure-accept` (child 4) |
| YANG validation constraints | Yes | `stateless` refuses `pd` beside it (child 3) |
| YANG custom validators | No | the native `must` suffices |
| CLI commands/flags | No | no command |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | Yes | automatic for boolean leaves |
| Functional test for new RPC/API | N-A | no RPC; config `.ci` per child |
| Pipe completeness | N-A | no output command |
| Env var registration | N-A | no environment leaf |
| Doctor check for runtime dependencies | No | the socket and port already exist |
| Prometheus counters/metrics | No | the DHCPv6 client exports none today |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/interfaces.md` per child |
| 2 | Config syntax changed? | Yes | YANG descriptions; `docs/features/interfaces.md` (children 3, 4) |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | same plugin, no new registration |
| 6 | Has a user guide page? | No | DHCPv6 lives in `docs/features/interfaces.md` |
| 7 | Wire format changed? | No | `docs/architecture/wire/` covers BGP only; RFC wire formats go in `rfc/short/rfc9915.md` |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9915.md`, `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (new scenarios, odhcpd image) |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` does not list DHCPv6 client modes |
| 12 | Internal architecture changed? | Yes | `docs/features/interfaces.md` socket lifetime (child 4) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/features/interfaces.md` (declared by `// Design:` of every `dhcp_v6_*.go`) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/features/interfaces.md` DHCPv6 examples checked per child |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- child 1, the annotations, committed before any feature code.
2. **Phase: Confirm** -- child 2.
3. **Phase: Stateless** -- child 3.
4. **Phase: Reconfigure and RKAP** -- child 4.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every child closed, all 23 rows out of `{gap}` |
| Feature completeness | the three user stories pass through their interop scenarios |
| Correctness | Section 16.11's six discard conditions each have a negative test (child 4) |
| Naming | YANG leaves kebab-case |
| Data flow | one exchange engine |
| Rule: rfc-compliance | every new implementing function quotes its RFC 9915 sentence |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| no `{gap}` row of the 23 remains | `grep -c "{gap" rfc/short/rfc9915.md` returns 0 after child 4 |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Reconfigure parsing and RKAP (child 4): a forged Reconfigure never changes a lease |

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

- The summary's Purpose paragraph and the brief both attributed a removal to RFC 9915 that RFC 9915 does not make: Reconfigure Accept survives (Section 21.20) and delayed authentication went in RFC 8415 (Section 1.1).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| An umbrella and four children | one spec with four phases | the annotation work is owed now and sits in another bucket; Confirm and stateless are independent and close separately; one spec would hold Confirm's closure behind Reconfigure's interop peer |
| Authentication and Reconfigure in one child | a separate RKAP child | RKAP's only use is validating Reconfigure (Section 20.4); alone it would be a validator with no non-test caller, which reaches nothing (`ai/rules/rfc-compliance.md`) |
| Reconfigure after stateless | Reconfigure first | Reconfigure Message msg-type 11 asks for an Information-request |
| Annotation child in `plan/pre-release/` | in `plan/` with the features | `plan/README.md`: RFC evidence a reader outside the repository reads is pre-release work |

## Known Limitations
- The 17 other `{feature-declined}` rows stay out of scope (A-4).
- No lease state survives a ze restart, so the "client reboots (and has stable storage and persistent DHCP state)" trigger of Section 18.2.12 never fires.

## RFC Documentation (Scope: protocol)

Add `// RFC 9915 Section X.Y: "<quoted requirement>"` above enforcing code, per child.

### Owner questions (resolved: owner decision 2026-10-19, verbatim "ok", every default reading accepted)
| # | Question | Default reading | Resolution |
|---|----------|-----------------|------------|
| Q1 | Reconfigure acceptance opt-in? | a `reconfigure-accept` leaf, default false: the RKAP key travels in clear in the Reply, so accepting Reconfigure is an operator decision | → Decision: owner, 2026-10-19, "ok": opt-in `reconfigure-accept` leaf, default false |
| Q2 | Stateless mode chosen by config or by the Router Advertisement O flag? | an explicit `stateless` leaf; RFC 9915 leaves the trigger out of scope | → Decision: owner, 2026-10-19, "ok": explicit `stateless` leaf, refused beside `pd` |
| Q3 | Reconfigure interop peer? | OpenWrt odhcpd built from source, since Kea 2.6.2 sends no Reconfigure | → Decision: owner, 2026-10-19, "ok": odhcpd built from source; if it will not build, stop and ask the owner, never skip the interop |
| Q4 | Stateless interop over 10 minutes to see one refresh? | yes | → Decision: owner, 2026-10-19, "ok": a stateless interop over 10 minutes is accepted |
| Q5 | Only the 23 rows of the four features change? | yes; the 17 others stay `{feature-declined}` | → Decision: owner, 2026-10-19, "ok": only the 23 named rows change |

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
- [ ] **Commit B:** `remove plan/spec-dhcpv6-client-0-umbrella.md` only, in the same `./le commit create` script

# Spec: home-cpe-19-access-chain-scenario

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P2 |
| Release reason | Validation of shipped behaviour: the PPPoE-to-L2TP relay (LAC, `l2tp relay`) ships, and its data path has no end-to-end proof. The only LAC scenario (`03-ze-lac-xl2tpd-lns`) proves the control connection alone, and `./le test deployment l2tp-ppp` runs a Ze LNS against xl2tpd and pppd with no PPPoE and no relay. |
| RFC gaps | - |
| Depends | `spec-home-cpe-18-interoplab-multi-link-topology.md`; `plan/release/delivery/spec-docker-hosts-run-the-ze-kernel.md` (in-progress); `spec-home-cpe-13-pppoe-client-ipcp-dns.md` for the DNS clause of `access-up` only |
| Phase | 0/3 |
| Handoff | - |
| Updated | 2026-10-31 |

Parent: `plan/home-cpe/spec-home-cpe-0-umbrella.md` (child 19).

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Build the access chain of the home-CPE lab as one topology scenario family and
prove, for the first time, that a subscriber's PPP session crosses Ze's LAC and
terminates on Ze's LNS:

Ze CPE (PPPoE client) → access link → Ze LAC (PPPoE server, `relay ""` to the
LNS remote) → L2TP over UDP on the core link → Ze LNS (CHAP only, RADIUS,
pool fallback, interim accounting) → FreeRADIUS on the core link.

Three scenarios, as the umbrella defines them:

| Scenario | Asserts |
|----------|---------|
| `access-up` | PADO with an empty Service-Name; session through LAC to LNS; CHAP checked by RADIUS; Framed-IP on the CPE; DNS learned from IPCP (once child 13 lands); CPE pings the LNS across the session |
| `access-pool` | a user whose Access-Accept carries no Framed-IP-Address gets a pool address |
| `access-accounting` | Accounting Start, one interim and Stop at FreeRADIUS, with the session's address and octets |

Every scenario is broken once on purpose, at the producing code, and goes red
(`ai/rules/interop-and-goal-validation.md`), and each red walk is designed
against the five vacuity traps of `docs/architecture/testing/interop.md`.

All values are invented, from the umbrella's lab table: core 192.0.2.0/24,
subscriber 203.0.113.1/32, pool 100.64.0.0/24, realm `lab.example.net`.

## Required Reading

### Architecture Docs
- [ ] `plan/home-cpe/spec-home-cpe-0-umbrella.md` "Lab"
  → Decision: Ze plays CPE, LAC and LNS; FreeRADIUS answers `cpe@lab.example.net` with Framed-IP-Address and a second user without one.
- [ ] `docs/architecture/testing/interop.md` "Prove a scenario discriminates", "L2TP and PPPoE lifecycle fault scenarios", "The FreeRADIUS subscriber suite"
  → Constraint: build the red binary through the harness, never by hand-tagging `ze-interop`; quote the image ID beside each red and green result.
  → Constraint: a checker reads accounting from FreeRADIUS's `ze_detail` file, as the subscriber suite does.
- [ ] `docs/guide/l2tp.md` relay section (l.58 to l.112)
  → Constraint: a relay entry keyed by the empty Service-Name matches a subscriber that requests no specific service; the guide's claim that `l2tp-ppp` exercises the LAC bridge is wrong and is corrected here (umbrella doc defects).
- [ ] `docs/architecture/l2tp/followup-l2tp-call.md` (Design header of `bridge_linux.go`)
  → Decision: the LAC forwards PPP and terminates nothing locally (RFC 2661 Section 2); LCP and CHAP run between the CPE and the LNS.
- [ ] `spec-home-cpe-18-interoplab-multi-link-topology.md`
  → Constraint: links, several Ze nodes and cross-suite images come from that spec; this one adds scenarios only.

### RFC Summaries
- [ ] `rfc/short/rfc2661.md` - L2TP, LAC forwarding
- [ ] `rfc/short/rfc2516.md` - PPPoE discovery (empty Service-Name)
- [ ] `rfc/short/rfc2866.md`, `rfc/short/rfc2869.md` - accounting, interim interval
  → Constraint: RFC 2869 Section 5.16, "The value MUST NOT be smaller than 60." Ze clamps a received Acct-Interim-Interval to at least 60 seconds (`clampAcctInterval`, `internal/component/l2tp/plugins/authradius/acct.go` l.713) and the `acct-interval` leaf has range 60..3600. The umbrella's "shortened so the test runs in seconds" is not reachable: the interim clause waits for the first interim at 60 seconds.

**Key insights:**
- The relay data path is the kernel bridge of the PPPoE channel to the PPPoL2TP channel (`bridgeChannels`, `maybeBridgePPPoE` in `internal/component/l2tp/bridge_linux.go`), chosen by `relaySink.Relay` and `relayTargetLocked` (`relay_sink.go`). No test today drives a frame across it.
- Every container sits on the lab bridge's eth0, so the CPE could reach the LNS without the PPP session; the CPE's eth0 must carry no address for the ping to prove anything (vacuity trap 4).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/yang/ze-l2tp-conf.yang` l.293 - `list relay` keyed by `service`, `leaf remote`; the empty string matches a subscriber requesting no service.
- [ ] `internal/component/l2tp/relay_sink.go` - `relaySink.Relay`, `relayTargetLocked`: the PPPoE server hands a matching session to the L2TP subsystem, which places an incoming call toward the bound remote.
- [ ] `internal/component/l2tp/bridge_linux.go` - `bridgeChannels` (PPPIOCBRIDGECHAN), `unbridgeChannel`, `maybeBridgePPPoE`: the data plane between the two kernel channels.
- [ ] `internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang` - `pppoe` server, `service-name` list ("An empty list accepts any Service-Name").
- [ ] `internal/component/l2tp/plugins/authradius/yang/ze-l2tp-auth-radius-conf.yang` - `l2tp auth radius`: `source-address`, `nas-identifier`, `acct-interval` (60..3600), `server` list.
- [ ] `internal/component/l2tp/plugins/authradius/acct.go` - `acctInterval`: a configured value overrides the Access-Accept's (RFC 2869 Section 2.1); `clampAcctInterval` floors at 60.
- [ ] `internal/component/l2tp/plugins/pool/yang/ze-l2tp-pool-conf.yang` - the address pool the LNS falls back to.
- [ ] `test/interop-l2tp/scenarios/03-ze-lac-xl2tpd-lns/ze.conf` - Ze dials xl2tpd through `request l2tp outgoing-call`; no PPPoE subscriber, no data.
- [ ] `internal/le/interoplab/radiussubscriber/radiussubscriber.go`, `test/interop-radius-subscriber/` - the FreeRADIUS image, `users`, `clients.conf`, `mods-ze-detail`, `site-default` the access chain reuses.
- [ ] `internal/le/interoplab/pppdevice.go` - the `/dev/ppp` grant.

**Behavior to preserve:** every existing L2TP, PPPoE and RADIUS scenario.

**Behavior to change:** none in Ze. The scenarios may find defects; each is fixed under `ai/rules/completion.md`, in the child whose mechanism fits, or journaled and owned (see Owned journal rows).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le test integration interop-topology access-up` (and `access-pool`, `access-accounting`).

### Transformation Path
1. The CPE's PPPoE client sends PADI with no Service-Name on its access link device; the LAC's PPPoE server answers PADO and the CPE completes PADR/PADS.
2. The LAC's PPPoE server hands the session to `relaySink.Relay`; the L2TP subsystem opens (or reuses) a tunnel to the LNS remote with the tunnel secret and places an ICRQ.
3. On ICCN the LAC bridges the PPPoE channel to the PPPoL2TP channel in the kernel.
4. LCP runs CPE to LNS across the bridge; the LNS challenges with CHAP; `authradius` sends Access-Request to FreeRADIUS.
5. The Access-Accept's Framed-IP-Address (or the pool) feeds IPCP; the CPE installs the address on its `pppN`.
6. The LNS sends Accounting-Start, interim updates at the configured interval, and Accounting-Stop at teardown; FreeRADIUS writes each to `ze_detail`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CPE ↔ LAC | PPPoE on the access link (no prefix) | No |
| LAC ↔ LNS | L2TPv2 over UDP on the core link | No |
| LNS ↔ FreeRADIUS | RADIUS auth 1812, accounting 1813 | No |
| LAC kernel | PPPIOCBRIDGECHAN between two PPP channels | No |

### Integration Points
- Topology suite of child 18: three Ze peers (`ze-cpe`, `ze-lac`, `ze-lns`), FreeRADIUS from the registry, links `access` (no prefix) and `core` (192.0.2.0/24).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | the CPE's eth0 holds no address; the ping's route is read from the CPE (`dev pppN`) |
| No unintended coupling | Yes | scenario files and checkers only |
| No duplicated functionality | Yes | FreeRADIUS image and detail module reused from the subscriber suite |
| Zero-copy preserved where applicable | N-A | tooling |
| Registration over hardcoding, outbound | Yes | scenarios register in the topology suite |
| Registration over hardcoding, inbound | Yes | no list learns the scenario names; `Discover` joins them |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Ze's LAC relays LCP and CHAP transparently (no proxy LCP) when the relay is bound | `bridge_linux.go` Design header (RFC 2661 Section 2) | the LAC terminates LCP and child 14 is a prerequisite | `access-up` | unvalidated |
| A-2 | The LNS sends DNS in IPCP when configured | umbrella node table | the DNS clause cannot be asserted even after child 13 | read the IPCP producer before writing the clause | unvalidated |
| A-3 | The CPE learns DNS only after child 13 | umbrella capability table | clause already passable | the clause is gated on child 13's landing and named as such | unvalidated |
| A-4 | Interim accounting cannot be faster than 60 s | RFC 2869 Section 5.16 MUST; `clampAcctInterval` | none | read above | validated |
| A-5 | Docker hosts run Ze's kernel with PPPoE, L2TP and PPPoL2TP | `spec-docker-hosts-run-the-ze-kernel` D-5 | `Suite.Run` refuses the host | the kernel probe | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The relay data path is broken (never proven) | `access-up` red at LCP | that red is the finding: fixed as owed work under completion, journaled to the owning child if it is a separate mechanism |
| R-2 | The CPE reaches the LNS over eth0 | ping green with the bridge broken | eth0 flushed by the CPE's prestart; checker refuses a route not via `pppN` |
| R-3 | A pass rests on one stimulus (trap 5) | every clause satisfied by IPCP alone | each clause reads a different producer: FreeRADIUS detail, CPE address, LNS session table, LAC bridge state |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | nothing in Ze; the evidence for the relay data path is wrong |
| How is it reverted? | single commit revert |
| Who else touches this path? | children 11, 12, 13, 14, 15, 16; `interop-radius-subscriber` image owners |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test integration interop-topology access-up` | → | PPPoE server → `relaySink.Relay` → `bridgeChannels` → LNS PPP → `authradius` | scenario `access-up` |
| `./le test integration interop-topology access-pool` | → | LNS pool fallback | scenario `access-pool` |
| `./le test integration interop-topology access-accounting` | → | `authradius` accounting | scenario `access-accounting` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | CPE dials with no Service-Name | a capture on the LAC's access device holds a PADO whose Service-Name tag has length 0, and a PADS with a non-zero session id |
| AC-2 | `access-up` running | the LAC holds one L2TP tunnel and one session toward the LNS, and its PPPoE session is bridged (the LAC's session table shows the relay bound) |
| AC-3 | `access-up` running | FreeRADIUS's log or detail shows Access-Accept for `cpe@lab.example.net` with the NAS-Identifier and source address the LNS configured; the LNS's session table names that user |
| AC-4 | `access-up` running | the CPE's `pppN` holds 203.0.113.1/32, an address outside the pool, so it came from Framed-IP-Address |
| AC-5 | `access-up` running | with the CPE's eth0 holding no address, the CPE pings the LNS's PPP-side address with 0% loss, the CPE's route to it is via `pppN`, and the LNS's PPP interface packet counters advance by at least the pings sent |
| AC-6 | `access-up` after child 13 lands | the CPE's resolver holds the DNS address the LNS sent in IPCP |
| AC-7 | `access-pool`: second user, Accept without Framed-IP-Address | the CPE's `pppN` holds an address in 100.64.0.0/24 and the LNS pool shows it allocated |
| AC-8 | `access-accounting`: LNS `acct-interval 60`, the CPE sends a known volume across the session, then the checker tears the session down from the CPE | FreeRADIUS's `ze_detail` holds Start, at least one Interim-Update and Stop with the same Acct-Session-Id; Framed-IP-Address 203.0.113.1 in each; Acct-Input-Octets and Acct-Output-Octets in Stop at least the volume sent |
| AC-9 | Each scenario's red walk (table below) | the scenario goes red with the named failure; restored, it goes green; both image IDs recorded |
| AC-10 | `docs/guide/l2tp.md` | no longer says `l2tp-ppp` exercises the LAC bridge; names `access-up` as the relay data-path proof |

### Red walks

| Scenario | Break (producer) | Expected red | Trap it rules out |
|----------|------------------|--------------|-------------------|
| `access-up` | `maybeBridgePPPoE` returns before `bridgeChannels` | no LCP Opened at the CPE; AC-5 times out | trap 4: the ping cannot reach the LNS another way because eth0 has no address |
| `access-up` | `relayTargetLocked` treats the empty service as no match | no PADO to an empty Service-Name (AC-1) | trap 2: the assertion is a present PADO, not an absence |
| `access-up` | `authradius` drops Framed-IP-Address from the Accept | CPE gets a pool address; AC-4 red | trap 5: the address clause reads RADIUS's attribute, not IPCP alone |
| `access-pool` | pool fallback returns no address | IPCP fails; AC-7 red | - |
| `access-accounting` | interim scheduling never fires | no Interim-Update in `ze_detail` | trap 2: asserts a present record |
| `access-accounting` | octet counters reported as zero | Stop octets below the volume sent; AC-8 red | trap 3: the volume is mid-range, not an extreme |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAccessChainDetailVerdict` | `internal/le/interoplab/topology/check_access_test.go` | the detail-file parser and the AC-8 verdict over recorded fixtures, red and green | |
| `TestAccessChainPADOVerdict` | `internal/le/interoplab/topology/check_access_test.go` | empty Service-Name tag judged from a capture fixture | |
| `TestAccessChainRouteVerdict` | `internal/le/interoplab/topology/check_access_test.go` | refuses a route to the LNS not via `pppN` | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `acct-interval` | 60-3600 | 60 | 59 (YANG refuses) | 3601 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `access-up` | `test/interop-topology/scenarios/` | a subscriber CPE comes up through a wholesale LAC on its ISP's LNS | |
| `access-pool` | `test/interop-topology/scenarios/` | a subscriber without a static address gets one from the pool | |
| `access-accounting` | `test/interop-topology/scenarios/` | the ISP's RADIUS records the session | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `access-up` | `test/interop-topology/scenarios/` | FreeRADIUS 3.2.10 | relay data path, CHAP via RADIUS, Framed-IP | |
| `access-accounting` | `test/interop-topology/scenarios/` | FreeRADIUS 3.2.10 | Start, interim, Stop with octets | |

## Files to Modify
- `docs/guide/l2tp.md` - the `l2tp-ppp` claim corrected; relay proof named
- `docs/architecture/testing/interop.md` - access-chain section and scenario inventory rows

## Files to Create
- `internal/le/interoplab/topology/register_access.go` - the three scenarios' nodes, links and checkers
- `internal/le/interoplab/topology/check_access.go`, `check_access_test.go`
- `test/interop-topology/scenarios/access-up/` - `ze-cpe.conf`, `ze-lac.conf`, `ze-lns.conf`, FreeRADIUS `users`
- `test/interop-topology/scenarios/access-pool/` - same nodes, user without Framed-IP-Address
- `test/interop-topology/scenarios/access-accounting/` - same nodes, `acct-interval 60`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | no Ze config added |
| YANG validation constraints | N-A | none |
| YANG custom validators | N-A | none |
| CLI commands/flags | N-A | the verb comes from child 18 |
| CLI grammar | N-A | none |
| Editor autocomplete | N-A | none |
| Functional test for new RPC/API | N-A | none |
| Pipe completeness | N-A | none |
| Env var registration | N-A | none |
| Doctor check | N-A | no runtime dependency added |
| Prometheus counters/metrics | N-A | none |
| BGP family surface | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | Feature evidence or level changed? | Yes | `features/<l2tp relay id>.md` Interop entry `topology/access-up` once green (`./le feature check`) |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/l2tp.md` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior newly proven? | Yes | `rfc/short/rfc2661.md` rows the relay proves, by `./le rfc record` where a tag is added |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | No | |
| 15 | Registered inventory changed? | No | |
| 16 | Changed source referenced by doc anchors? | No | no Ze source changes |
| 17 | Existing docs show examples for this area? | Yes | `docs/guide/l2tp.md` relay example checked against the YANG |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- `access-up` directory and declaration, checker that fails at AC-1
   - Tests: `access-up`
   - Verify: the three Ze nodes and FreeRADIUS start on their links
2. **Phase: access-up** -- AC-1 to AC-5, red walks; AC-6 added when child 13 lands
   - Tests: `TestAccessChainPADOVerdict`, `TestAccessChainRouteVerdict`, `access-up`
3. **Phase: access-pool and access-accounting** -- AC-7, AC-8, red walks, AC-10
   - Tests: `TestAccessChainDetailVerdict`, both scenarios

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N and every red walk recorded with image IDs |
| Correctness | no clause is satisfied by the same single event as another (trap 5) |
| Rule: interop-and-goal-validation | red forced at the producer, harness-built binaries |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Three scenarios green | `./le test integration interop-topology access-up` and the two others |
| Red walks | recorded red and green outputs with image IDs in the spec's closure |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Secrets | lab-only tunnel secret and RADIUS secret, invented, in scenario files |

## Owned journal rows

None: `grep -rn 'relay data\|LAC bridge\|l2tp-ppp' plan/journal/` finds no open row about the relay data path.

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Three Ze nodes, no third-party LAC | xl2tpd as LAC or LNS | the relay under test is Ze's; the subscriber suite already pairs Ze with xl2tpd |
| CPE eth0 holds no address | rely on routing | otherwise the ping reaches the LNS over the lab bridge (trap 4) |
| Interim waited at 60 s | shorten via Acct-Interim-Interval | RFC 2869 Section 5.16 forbids below 60 and Ze clamps; the umbrella's "seconds" is corrected here |
| Accounting read from FreeRADIUS's detail file | read Ze's own log | the record the server accepted is the evidence |

## Known Limitations

- The realm-based LAC (LCP and CHAP on the LAC, proxy AVPs) is `access-wholesale` in child 11, after child 14.
- `access-reconnect` belongs to child 11 (needs child 4).

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path (N-A: tooling)
- [ ] Wiring Test table complete
- [ ] Every A-N confirmed or broken

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A** and **Commit B** per `ai/rules/planning.md`

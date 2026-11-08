# Spec: home-cpe-19-access-chain-scenario

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | Validation of shipped behaviour: the PPPoE-to-L2TP relay (LAC, `l2tp relay`) ships, and its data path has no end-to-end proof. The only LAC scenario (`03-ze-lac-xl2tpd-lns`) proves the control connection alone, and `./le test deployment l2tp-ppp` runs a Ze LNS against xl2tpd and pppd with no PPPoE and no relay. |
| RFC gaps | - |
| Depends | Implementation: `spec-home-cpe-18-interoplab-multi-link-topology.md` (topology suite); `plan/release/delivery/spec-docker-hosts-run-the-ze-kernel.md` (in-progress). The LNS's `mru 1492`, CHAP-only `authentication allow [ chap-md5 ]` and negotiated `show l2tp session id` keys AC-11 reads are implemented (`docs/guide/l2tp.md`, "PPP negotiation", "Authentication" and "Read commands"). Closure: none beyond those. The DNS clause of `access-up` (AC-6) is owned by `spec-home-cpe-13-pppoe-client-ipcp-dns.md` and holds that spec's closure, not this one's |
| Phase | 3/3 |
| Handoff | - |
| Updated | 2026-11-08 |

Parent: `plan/release/fixes/spec-home-cpe-0-umbrella.md` (child 19).

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
- [ ] `plan/release/fixes/spec-home-cpe-0-umbrella.md` "Lab"
  → Decision: Ze plays CPE, LAC and LNS; FreeRADIUS answers `cpe@lab.example.net` with Framed-IP-Address and a second user without one.
- [ ] `docs/architecture/testing/interop.md` "Prove a scenario discriminates", "L2TP and PPPoE lifecycle fault scenarios", "The FreeRADIUS subscriber suite"
  → Constraint: build the red binary through the harness, never by hand-tagging `ze-interop`; quote the image ID beside each red and green result.
  → Constraint: a checker reads accounting from FreeRADIUS's `ze_detail` file, as the subscriber suite does.
- [ ] `docs/guide/l2tp.md` "Initiator: dial targets and outgoing calls" (the `remote` and `relay` lists and the LAC bridge paragraph)
  → Constraint: a relay entry keyed by the empty Service-Name matches a subscriber that requests no specific service; the guide's claim that `l2tp-ppp` exercises the LAC bridge is wrong and is corrected here (umbrella doc defects).
- [ ] `docs/architecture/l2tp/followup-l2tp-call.md` (Design header of `bridge_linux.go`)
  → Decision: the LAC forwards PPP and terminates nothing locally (RFC 2661 Section 2); LCP and CHAP run between the CPE and the LNS.
- [ ] `spec-home-cpe-18-interoplab-multi-link-topology.md`
  → Constraint: links, several Ze nodes and cross-suite images come from that spec; this one adds scenarios only.

### RFC Summaries
- [ ] `rfc/short/rfc2661.md` - L2TP, LAC forwarding
- [ ] `rfc/short/rfc2516.md` - PPPoE discovery (empty Service-Name)
- [ ] `rfc/short/rfc2866.md`, `rfc/short/rfc2869.md` - accounting, interim interval
  → Constraint: RFC 2869 Section 5.16, "The value MUST NOT be smaller than 60." Ze clamps a received Acct-Interim-Interval to at least 60 seconds (`clampAcctInterval`, `internal/component/l2tp/plugins/authradius/acct.go`) and the `acct-interval` leaf (`ze-l2tp-auth-radius-conf.yang`) has range 60..3600. The umbrella's "shortened so the test runs in seconds" is not reachable: the interim clause waits for the first interim at 60 seconds.

**Key insights:**
- The relay data path is the kernel bridge of the PPPoE channel to the PPPoL2TP channel (`bridgeChannels`, `maybeBridgePPPoE` in `internal/component/l2tp/bridge_linux.go`), chosen by `relaySink.Relay` and `relayTargetLocked` (`relay_sink.go`). No test today drives a frame across it.
- Every container sits on the lab bridge's eth0, so the CPE could reach the LNS without the PPP session; the CPE's eth0 must carry no address for the ping to prove anything (vacuity trap 4).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/yang/ze-l2tp-conf.yang` - `list relay` (beside `list remote` in `container l2tp`) keyed by `service`, `leaf remote`; the empty string matches a subscriber requesting no service.
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

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec |
|-----------|------------------------------------------|--------------------|-----------|
| PPPoE-to-L2TP relay selection | `internal/component/l2tp/relay_sink.go` `relaySink.Relay`, `Subsystem.relayTargetLocked` | hands a PPPoE session whose Service-Name matches a `relay` entry to the L2TP subsystem, which places an incoming call toward the bound remote | does not apply: exercised unchanged as the system under test |
| Kernel channel bridge | `internal/component/l2tp/bridge_linux.go` `bridgeChannels`, `maybeBridgePPPoE` | bridges the PPPoE channel to the PPPoL2TP channel with PPPIOCBRIDGECHAN | does not apply: exercised unchanged; the scenarios drive the first frame across it |
| Subscriber RADIUS accounting | `internal/component/l2tp/plugins/authradius/acct.go` `acctInterval`, `clampAcctInterval` | picks the configured or Access-Accept interim interval, floored at 60 s | does not apply: exercised unchanged by `access-accounting` |
| FreeRADIUS subscriber suite | `internal/le/interoplab/radiussubscriber/radiussubscriber.go` `serverPeerConfig`, `checkers.go` `detailPath`; `docs/architecture/testing/interop.md` "The FreeRADIUS subscriber suite" | runs FreeRADIUS with `users`, `clients.conf`, `mods-ze-detail`, `site-default`, and reads the `ze_detail` file in its checkers | extends: the access chain reuses its image, lab files and detail reading rather than writing a second RADIUS harness |
| Multi-link interop topology | `spec-home-cpe-18-interoplab-multi-link-topology.md` | adds links, several Ze nodes and cross-suite images to `internal/le/interoplab` | extends: this spec only adds scenarios on that topology |
| `/dev/ppp` grant | `internal/le/interoplab/pppdevice.go` `PPPDeviceArguments` | the device grant every PPP container needs | extends: reused by the CPE, LAC and LNS containers |
| L2TP LAC scenario | `test/interop-l2tp/scenarios/03-ze-lac-xl2tpd-lns/` | proves the LAC control connection against xl2tpd, with no PPPoE subscriber and no data | does not apply: kept as is; it proves no data path, which is the gap this spec closes |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le test integration interop-topology access-up` (and `access-pool`, `access-accounting`).

### Transformation Path
1. The CPE's PPPoE client sends PADI with no Service-Name on its access link device; the LAC's PPPoE server answers PADO and the CPE completes PADR/PADS.
2. The LAC's PPPoE server hands the session to `relaySink.Relay`; the L2TP subsystem opens (or reuses) a tunnel to the LNS remote with the tunnel secret and places an ICRQ.
3. On ICCN the LAC bridges the PPPoE channel to the PPPoL2TP channel in the kernel.
4. LCP runs CPE to LNS across the bridge; the LNS challenges with CHAP; `authradius` sends Access-Request to FreeRADIUS.
5. The Access-Accept's Framed-IP-Address (or the pool) feeds IPCP; the CPE installs the address on its `pppoe0`.
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
| No bypassed layers | Yes | the CPE's eth0 holds no address; the ping's route is read from the CPE (`dev pppoe0`) |
| No unintended coupling | Yes | scenario files and checkers only |
| No duplicated functionality | Yes | FreeRADIUS image and detail module reused from the subscriber suite |
| Zero-copy preserved where applicable | N-A | tooling |
| Registration over hardcoding, outbound | Yes | scenarios register in the topology suite |
| Registration over hardcoding, inbound | Yes | no list learns the scenario names; `Discover` joins them |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Ze's LAC relays LCP and CHAP transparently (no proxy LCP) when the relay is bound | `bridge_linux.go` Design header (RFC 2661 Section 2) | the LAC terminates LCP and child 14 is a prerequisite | `access-up` | confirmed |
| A-2 | The LNS sends DNS in IPCP when configured | umbrella node table | the DNS clause cannot be asserted even after child 13 | read the IPCP producer before writing the clause | confirmed |
| A-3 | The CPE learns DNS only after child 13 | umbrella capability table | clause already passable | the clause is gated on child 13's landing and named as such | confirmed |
| A-4 | Interim accounting cannot be faster than 60 s | RFC 2869 Section 5.16 MUST; `clampAcctInterval` | none | read above | validated |
| A-5 | Docker hosts run Ze's kernel with PPPoE, L2TP and PPPoL2TP | `spec-docker-hosts-run-the-ze-kernel` D-5 | `Suite.Run` refuses the host | the kernel probe | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The relay data path is broken (never proven) | `access-up` red at LCP | that red is the finding: fixed as owed work under completion, journaled to the owning child if it is a separate mechanism |
| R-2 | The CPE reaches the LNS over eth0 | ping green with the bridge broken | eth0 flushed by the CPE's prestart; checker refuses a route not via `pppoe0` |
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
| AC-4 | `access-up` running | the CPE's `pppoe0` holds 203.0.113.1/32, an address outside the pool, so it came from Framed-IP-Address |
| AC-5 | `access-up` running | with the CPE's eth0 holding no address, the CPE pings the LNS's PPP-side address with 0% loss, the CPE's route to it is via `pppoe0`, and the LNS's PPP interface packet counters advance by at least the pings sent |
| AC-6 | `access-up` after child 13 lands | the CPE's resolver holds the DNS address the LNS sent in IPCP |
| AC-7 | `access-pool`: second user, Accept without Framed-IP-Address | the CPE's `pppoe0` holds an address in 100.64.0.0/24 and the LNS pool shows it allocated |
| AC-8 | `access-accounting`: LNS `acct-interval 60`, the CPE sends a known volume across the session, then the checker tears the session down from the CPE | FreeRADIUS's `ze_detail` holds Start, at least one Interim-Update and Stop with the same Acct-Session-Id; Framed-IP-Address 203.0.113.1 in each; Acct-Input-Octets and Acct-Output-Octets in Stop at least the volume sent |
| AC-9 | Each scenario's red walk (table below) | the scenario goes red with the named failure; restored, it goes green; both image IDs recorded |
| AC-10 | `docs/guide/l2tp.md` and the comment of `TestLACChannelBridge` (`internal/component/l2tp/bridge_integration_linux_test.go`) | neither says `l2tp-ppp` or `gokrazy-l2tp-ppp` exercises the LAC bridge; both name `access-up` as the relay data-path proof. This spec owns the correction because it lands before child 14 (umbrella order); child 14 only adds its realm scenarios |
| AC-11 | `access-up` running, LNS configured `l2tp { mru 1492; authentication { allow [ chap-md5 ] } }` | `ze show l2tp session id <id>` on the LNS carries `mru` 1492 and `auth-method` `chap-md5` (child 16 AC-15 keys); the CPE's `pppoe0` MTU is 1492; a capture on the core link holds the LNS's LCP Configure-Request with MRU option 1492 |
| AC-12 | `access-up` running | FreeRADIUS's Access-Request for `cpe@lab.example.net` carries CHAP-Password and no User-Password, so the method the LNS negotiated is the one RADIUS checked. The refusal of a PAP-only client is not asserted here: Ze's `pppoe-client` has no leaf that refuses CHAP (its `authentication` container holds `username` and `password` only), so child 16 proves the refusal with pppd in `pppoe-chap-only-pap-client` and xl2tpd in `l2tp-lns-chap-only-xl2tpd` |

### Red walks

| Scenario | Break (producer) | Expected red | Trap it rules out |
|----------|------------------|--------------|-------------------|
| `access-up` | `maybeBridgePPPoE` skips `bridgeChannels` | no LCP Opened across the LAC (the LNS logs "LCP negotiation timeout"); the first clause, the AC-4 address wait, times out before AC-5 is reached | trap 4 is ruled out by AC-5's own eth0 and route clauses (`checkPingAcross`, `TestAccessChainRouteVerdict`), not by this walk, which never reaches the ping |
| `access-up` | `relayTargetLocked` treats the empty service as no match | the LAC terminates the session locally after PADR (its own PPP rejects CHAP: no local users), so the CPE never holds 203.0.113.1 (AC-4) and the LAC carries no L2TP session (AC-2). A PADO is still sent: the relay decision is made at PADR (`InterfaceServer.relayToL2TP`, `internal/component/l2tp/pppoe/server.go`), so AC-1 cannot go red here | trap 2: the assertion is a present address, not an absence |
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
| `TestAccessChainRouteVerdict` | `internal/le/interoplab/topology/check_access_test.go` | refuses a route to the LNS not via `pppoe0` | |

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

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output): `./le test integration interop-topology access-up`, `access-pool` and `access-accounting`, plus the scoped package test of `internal/le/interoplab/topology` under `./le job run`

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A** and **Commit B** per `ai/rules/planning.md`

---

## Implementation Summary

### What Was Implemented
- Three topology scenarios, `access-up`, `access-pool` and `access-accounting`, registered in `internal/le/interoplab/topology/register_access_chain.go` (`accessChain`, `dialChain`), with checkers `checkAccessUp`, `checkAccessPool`, `checkAccessAccounting` (`check_access.go`), verdicts (`check_access_verdict.go`) and capture parsing (`check_access_capture.go`). Landed in 93362da68e, 42a8e2ff74 (AC-6 DNS), a0e83e12a1, 79e0c3ee1d, 8e2c5c8dca, df706464b6.
- Closure: `features/l2tpv2-bng.md` gains the three `topology/` Interop entries and an access-chain paragraph; `docs/guide/l2tp.md` no longer says `access-up` has not run; `TestLACChannelBridge`'s header names `access-up` as the relay data-path proof (AC-10).

### Bugs Found/Fixed
- None in Ze through this spec's checkers. The 12:51 and 14:23 lab runs failed because ze-cpe lacked `CAP_BPF`, fixed by 9167d499fd; they are not cited.

### Documentation Updates
- `docs/guide/l2tp.md` relay paragraph: access-up, access-pool and access-accounting named with what each proves.
- `features/l2tpv2-bng.md`: Interop entries and a paragraph anchored at `check_access.go -- checkAccessUp, checkAccessPool, checkAccessAccounting`; Doc review re-dated.
- `./le feature check`: no refusal for `l2tpv2-bng`. `./le doc check all`: no finding on the edited files (its one CLAIM, the `enableIPv6Forwarding` anchor in `interop.md`, belongs to the DHCPv6-PD scenario, not this spec).

### Deviations from Plan
- `register_access.go` is `register_access_chain.go`; the scenarios hold only `users` and `ze-lns.conf`, because the CPE runs the home-CPE lab's converted configuration and `ze-lac.conf` is shared under `test/interop-topology/access-chain/`.
- Red walk `c19-empty-service-nomatch`: the spec predicted "no PADO". A PADO is still sent, because the relay is decided at PADR (`InterfaceServer.relayToL2TP`, `internal/component/l2tp/pppoe/server.go`). The red is the local termination instead (Red walks table corrected).
- Red walk `c14-bridge-skipped`: the spec predicted "AC-5 times out". The first clause of `checkAccessUp` is the AC-4 address wait, which times out first; the LNS log shows the cause, "LCP negotiation timeout after 30s". Trap 4 is ruled out by AC-5's eth0 and route clauses, not by this walk.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The spec stayed at `design`, Phase 0/3, while six commits implemented it under `Spec-work: home-cpe-1-logical-interface-name` | its implementation and closure bookkeeping were never joined | closure found the status `design` | status moved ready then in-progress by claim; closure records the commits |
| assumption | Two red walk predictions named the wrong clause | relay decided at PADR; the address wait precedes the ping | reading the broken logs' FAIL lines | Red walks table corrected |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `access-up` | Done | `check_access.go` `checkAccessUp` | green 2026-11-08 |
| `access-pool` | Done | `check_access.go` `checkAccessPool` | green |
| `access-accounting` | Done | `check_access.go` `checkAccessAccounting` | green |
| every scenario broken once at the producer | Done | red walks below | six walks |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `checkDiscovery` over `discoveryVerdict` | PADO Service-Name length 0, PADS non-zero |
| AC-2 | Done | `checkLACRelays` | one tunnel to 192.0.2.3 port 1701, one session, PPPoE session on the access device, no addressed ppp device |
| AC-3 | Done | `waitLogin`, `loginVerdict`, `lnsSessionOf` | NAS-Identifier `ze-lns`, NAS-IP and source 192.0.2.3 |
| AC-4 | Done | `waitCPEAddress` with 203.0.113.1 | outside the pool |
| AC-5 | Done | `checkPingAcross` | eth0 empty, route `dev pppoe0`, 0% loss, LNS rx_packets advance |
| AC-6 | Done | `waitResolvers` (42a8e2ff74) | learned 192.0.2.53, 192.0.2.54; resolv.conf static only |
| AC-7 | Done | `checkAccessPool` | `show l2tp pool` allocated 1 to the session |
| AC-8 | Done | `waitAccounting`, `accountingVerdict` | Start, Interim, Stop, one Acct-Session-Id, octets at least 20560 |
| AC-9 | Done | red walks table below | image IDs recorded |
| AC-10 | Done | `docs/guide/l2tp.md` relay paragraph, `bridge_integration_linux_test.go` header | both name access-up |
| AC-11 | Done | `checkNegotiated`, `lcpMRUVerdict` | mru 1492, chap-md5, CPE MTU 1492, LCP Configure-Request MRU 1492 on core |
| AC-12 | Done | `loginVerdict` | CHAP-Password present, User-Password absent |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestAccessChainDetailVerdict` | Done | `check_access_test.go` | PASS 2026-11-08 (see Pre-Commit Verification) |
| `TestAccessChainPADOVerdict` | Done | `check_access_test.go` | as above |
| `TestAccessChainRouteVerdict` | Done | `check_access_test.go` | as above |
| `access-up`, `access-pool`, `access-accounting` | Done | `test/interop-topology/scenarios/` | green 2026-11-08 about 18:00 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `register_access.go` | Changed | `register_access_chain.go` |
| `check_access.go`, `check_access_test.go` | Done | plus `check_access_verdict.go`, `check_access_capture.go` |
| scenario directories | Changed | `users` and `ze-lns.conf`; CPE and LAC configs shared |
| `docs/guide/l2tp.md`, `docs/architecture/testing/interop.md` | Done | interop.md scenario inventory rows for the three scenarios |

### Audit Summary
- **Total items:** 23
- **Done:** 20
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| a subscriber's PPP session crosses Ze's LAC and terminates on Ze's LNS | interop | `access-up` 1 passed (log `rwinv/out-interop/c19-up-bridge-clean.log`, ze image sha256:eccb83541e13); with the bridge skipped it goes red and the LNS logs LCP negotiation timeout |
| pool fallback | interop | `access-pool` 1 passed (`c19-pool-clean.log`); red with the pool empty ("ipcp: handler rejected: pool exhausted") |
| accounting Start, interim, Stop with octets | interop | `access-accounting` 1 passed (`c19-acct-interim-clean.log`, 1m51s); red without interims and with zero octets |

### Red walks (run 2026-11-08 from 17:35, after 9167d499fd; logs under `tmp/session/2026-11-07-104251/scratch/rwinv/out-interop/`)

Each overlay copy was diffed against today's source: each differs from it in the one named line only.

| Walk | Break | Clean (ze image) | Broken (ze image) | Red observed | Named failure? |
|------|-------|------------------|-------------------|--------------|----------------|
| c19-up-bridge | `maybeBridgePPPoE` calls no `bridgeChannels` | pass, sha256:eccb83541e13 | FAIL, sha256:67d00d5ffc10 | pppoe0 never held 203.0.113.1; LNS: "LCP negotiation timeout after 30s" three times | yes: no LCP across the LAC; the address wait fires before AC-5 |
| c19-up-empty-service | `relayTargetLocked` refuses `""` | pass, sha256:0352b017a941 | FAIL, sha256:f9eefcb3dbf8 | pppoe0 does not exist; LAC (`subsystem=pppoe`) "chap: auth rejected: no users configured" | yes: local termination instead of relay |
| c19-up-framed-ip | `extract.go` drops `meta.FramedIP` | pass, sha256:7424e4fc1ec5 | FAIL, sha256:1ffee23b7b29 | pppoe0 holds 100.64.0.2, not 203.0.113.1 | yes: AC-4 |
| c19-pool | `pool.go` allocate always exhausted | pass, sha256:be2ef6390ad2 | FAIL, sha256:bd1d4e962bbd | no 100.64.0.0/24 address; LNS "pool exhausted" | yes: AC-7 |
| c19-acct-interim | `runAccounting` skips `interimLoop` | pass, sha256:e2b6db1327e9 | FAIL, sha256:60f140d7d435 | the session's Acct-Session-Id has no Interim-Update | yes: AC-8 |
| c19-acct-octets | Stop and Interim octets forced to 0 | pass, sha256:e84572a4c825 | FAIL, sha256:33b09db9e029 | Stop Acct-Input-Octets 0 below 20560 | yes: AC-8 |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | every AC demonstrated | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/home-cpe-19-access-chain-scenario-7e292f27-ced0-427f-a7b6-3a931db211f9.md` |
| `./le spec review check` | clean, over the spec and the three closure files |
| Rounds | 1 |
| Reviewer lenses used | logic and vacuity of each checker clause, bounds of the capture parsers (`discoveryOfFrame`, `udpFrom`, `l2tpPayload`, `lcpMRUOption`), style pass over `check_access*.go` and `register_access_chain.go` (no panic, no reachable unchecked slice) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | the guide said `access-up` had not run green | `docs/guide/l2tp.md` relay paragraph | rewritten in closure |
| 2 | ISSUE | AC-10: `TestLACChannelBridge`'s header did not name `access-up` | `bridge_integration_linux_test.go` | header names it |

NOTEs: `loginVerdict` judges the user's first logged answer, so a reject before an accept fails the wait rather than waiting on; the `accountingVerdict` octet check ignores Gigawords, which cannot be set at 20 560 octets.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/interoplab/topology/register_access_chain.go`, `check_access.go`, `check_access_test.go` | yes | `ls internal/le/interoplab/topology/` 2026-11-08 |
| `test/interop-topology/scenarios/access-{up,pool,accounting}/` | yes | each holds `users`, `ze-lns.conf` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-8, AC-11, AC-12 | asserted by the checkers named in the audit | three scenarios 1 passed in the clean logs above |
| AC-9 | six reds | broken logs, FAIL lines quoted in the red walks table |
| AC-10 | both texts name access-up | `grep -n access-up docs/guide/l2tp.md internal/component/l2tp/bridge_integration_linux_test.go` |
| unit tests | `TestAccessChain*` pass | `./le job run label c19-topo command go test -tags <ze_core zetest and every ze_ gate> -count=1 -v -run TestAccessChain ./internal/le/interoplab/topology/`: ok, 9 tests PASS (`tmp/session/2026-11-07-104251/scratch/c19-topo.log`); the whole package ok (`c19-topo-all.log`), 2026-11-08 about 18:40 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le test integration interop-topology scenario access-up` (and the two others) | scenario directories, no `.ci` | the driver ran this exact command for each clean and broken log |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | access-up green; `checkLACRelays` finds no addressed ppp device on the LAC |
| A-2 | confirmed | `waitResolvers` reads 192.0.2.53 then 192.0.2.54 learned in IPCP |
| A-3 | confirmed | AC-6 clause added in 42a8e2ff74 after child 13's client landed |
| A-4 | confirmed | `clampAcctInterval`; interim waited 100 s |
| A-5 | confirmed | the 2026-11-08 runs passed on a Ze-kernel Docker host |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| 1 feature evidence | `features/l2tpv2-bng.md` Interop; `./le feature check` no l2tpv2-bng refusal | yes |
| 6 user guide | `docs/guide/l2tp.md` relay paragraph vs `checkAccessUp` | yes |
| 9 RFC behaviour | No: no tagged unit added (`grep -rln 'RFC requirement' internal/le/interoplab/topology` empty); `./le rfc check` lines for rfc2661 and rfc2516 name other specs' units | yes |
| 10 test infrastructure | `docs/architecture/testing/interop.md` rows for the three scenarios | yes |
| 17 relay example | `list relay` key `service`, length 0..255, in `ze-l2tp-conf.yang` | yes |

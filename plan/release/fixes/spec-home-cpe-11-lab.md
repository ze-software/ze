# Spec: home-cpe-11-lab

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P2 |
| Release reason | Validation: the lab is the evidence that a Ze router converted from a VyOS home CPE does the job end to end (umbrella goal 3), and it carries the first runtime proof of shipped features that have none (WireGuard road warrior, masquerade and MSS with Ze in the path, the LAC relay). |
| RFC gaps | - (each scenario proves the RFC rows of the child it depends on) |
| Depends | `spec-home-cpe-18-interoplab-multi-link-topology.md`, `spec-home-cpe-19-access-chain-scenario.md`, `spec-home-cpe-10-vyos-mapping-guide.md`; per scenario (table below): children 2, 3, 4, 5, 6, 12, 14, 15, 20, 21; `plan/release/validation/spec-wireguard-runtime-proof.md`; `plan/release/validation/spec-ipsec-0-umbrella.md`; `plan/release/delivery/spec-docker-hosts-run-the-ze-kernel.md` |
| Phase | 0/8 |
| Handoff | - |
| Updated | 2026-10-31 |

Parent: `plan/home-cpe/spec-home-cpe-0-umbrella.md` (child 11).

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Complete the home-CPE lab of the umbrella: every scenario of its table, run in
the `topology` suite of child 18 (except `accel-ppp-renegotiation`, which
belongs to the existing PPPoE suite), with the CPE running the configuration
the model converted from a VyOS configuration (child 10).

This spec composes; it does not duplicate. The access chain (`access-up`,
`access-pool`, `access-accounting`) is child 19. `road-warrior` is the
acceptance evidence of `spec-wireguard-runtime-proof.md` (its AC-1 and AC-2).
`mgmt-tunnel` is a scenario under `spec-ipsec-0-umbrella.md`, whose children
5, 6, 20 and 4 own the behaviour. Each scenario lands when its dependency
child does, and its red walk is recorded then.

### Scenarios this spec owns

| Scenario | Suite | Asserts | Depends on |
|----------|-------|---------|------------|
| `conversion` | topology | the converted CPE configuration passes `ze config validate`; Ze starts with it (commit-time verifiers run); every line of the VyOS source appears in the conversion record as mapped or dropped with a reason; every topology scenario's CPE file is that configuration with lab values substituted | 10 |
| `lan-services` | topology | range lease; both reservations honoured, one per subnet on the one bridge; masquerade to the web server; MSS 1400 written on a SYN carrying more and a smaller MSS left alone; WAN input dropped while the same port answers from the LAN; FTP through NAT via the helper | 2, 21, 3 (helper clause after the owner's decision) |
| `road-warrior` | topology | the committed WireGuard configuration is in the kernel (interface, keys, peers, allowed-ips); wg-client with PSK handshakes with the CPE's WAN address; it reaches a LAN host and the web server, masqueraded | 18, `spec-wireguard-runtime-proof` |
| `mgmt-tunnel` | topology | Ed25519 authentication (and the P-256 variant); selectors narrowed by strongSwan to `::/0`; IPv6 ping across vti0 with ESP counters read per direction; IKE moved to UDP 4500 by MOBIKE; a CPE booted while DNS is down establishes once DNS answers | 4, 5, 6, 20 |
| `access-wholesale` | topology | LAC runs LCP and CHAP and picks the LNS by realm; LNS renegotiates LCP and re-challenges; the CPE comes up | 12, 14, 15 |
| `access-reconnect` | topology | the LNS drops the session; the CPE reconnects with a new address; the IKE SA follows the new address and masquerade uses it | 4, 18 |
| `accel-ppp-renegotiation` | pppoe (`test/interop-pppoe/scenarios/`) | accel-ppp renegotiates LCP after a first negotiation; the Ze client renegotiates, re-authenticates, runs IPCP, and traffic flows | 12 |

### Lab nodes beyond child 19

| Node | Image owner | Link(s) |
|------|-------------|---------|
| lan-client, lan-res-a, lan-res-b | dhcpcd image (`test/interop/Dockerfile.knot` carries dhcpcd) | lan (no prefix: the CPE numbers it) |
| DNS | Knot or Unbound (`test/interop/Dockerfile.knot`, `Dockerfile.unbound`) | core |
| strongSwan responder `vpn.example.net` | `test/interop-ipsec/Dockerfile.strongswan` | internet |
| wg-client | `internal/le/test/deployment/testdata/Dockerfile.wireguard` | internet |
| web and FTP server | an image registered by this spec (busybox httpd and an FTP daemon, Alpine-free per `ai/META-DECISIONS.md` test systems rule) | internet |

The LNS routes the internet segment (198.51.100.0/24) to subscribers; the CPE
reaches it only through the PPP session.

## Required Reading

### Architecture Docs
- [ ] `plan/home-cpe/spec-home-cpe-0-umbrella.md` "Lab", "Conversion by the model"
  → Decision: every value invented; the CPE configuration is the model's conversion with lab values.
- [ ] `docs/architecture/testing/interop.md` "Prove a scenario discriminates", "IPsec address movement", "The IPsec NAT box", "The fifth trap" text under the vacuity table
  → Constraint: ESP proof reads each simplex SA by its own source and destination, never an SPI-keyed aggregate (`verifyESPDirections`, `internal/le/interoplab/ipsec/helpers.go`).
- [ ] `docs/guide/configuration.md` l.2546
  → Constraint: "It does not run under `ze config validate`, which reads the schema and does not run a plugin's config verifier." The `conversion` scenario therefore also starts Ze with the configuration.
- [ ] `plan/release/validation/spec-wireguard-runtime-proof.md` AC-1, AC-2
  → Decision: `road-warrior` is that spec's evidence; this spec does not restate its criteria beyond the lab topology.
- [ ] `ai/META-DECISIONS.md` "Test systems and labs", "Test changes"
  → Constraint: guests and lab hosts run Ze's kernel; Alpine-only problems are out of scope; full suites run on another host than the busy Linux one.

**Key insights:**
- Several clauses assert an absence (WAN input dropped, a smaller MSS left alone); each is paired with a positive control that the same mechanism produces (vacuity trap 2).
- The CPE has one configuration; scenarios differ in which peers run and which checker runs, never in a hand-edited CPE file.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/interoplab/ipsec/helpers.go` - `verifyESPDirections`, directed ESP counters and the lossless-ping clause.
- [ ] `internal/le/test/deployment/wireguardproof.go` - the kernel-only WireGuard proof (two nodes, `Dockerfile.wireguard`); it says it does not prove the configuration-commit path.
- [ ] `test/interop-pppoe/scenarios/` - Ze client against accel-ppp (`01-pppoe-chap-ipv4`, `pppoe-chap-rechallenge-ze-client`, ...); none renegotiates LCP.
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` - `tcp-mss-set` (l.466), `masquerade` (l.355), `global-options` (l.564).
- [ ] `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang` - `shared-network`, `subnet`, `range`, `static-mapping`.
- [ ] `test/interop/Dockerfile.knot` - Knot plus dhcpcd 10.5.2.

**Behavior to preserve:** every existing scenario.

**Behavior to change:** none in Ze here. Each scenario's red-today clause is the dependency child's defect.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le test integration interop-topology <scenario>`; `./le test deployment docker-pppoe-accel` with the `accel-ppp-renegotiation` selector.
- The conversion inputs: an invented VyOS configuration (`test/interop-topology/cpe/vyos.conf`), the model's output (`ze-cpe.conf` with placeholders) and its conversion record, and a lab values file.

### Transformation Path
1. The conversion (done once by the model, committed) yields `ze-cpe.conf` and the record.
2. The topology plan builder substitutes lab values for placeholders and mounts the result as the CPE's configuration in every scenario.
3. Each scenario starts the nodes it needs, its checker drives traffic and reads state on both ends.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| LAN client ↔ CPE | DHCPv4 on the lan link | No |
| CPE ↔ internet | through the PPP session and the LNS's routing | No |
| CPE ↔ strongSwan | IKEv2 and ESP over the PPP session | No |
| wg-client ↔ CPE | WireGuard to the CPE's PPP address | No |

### Integration Points
- Topology suite and image registry (child 18); access-chain nodes and files (child 19).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | only the CPE's PPP address is reachable from the internet segment |
| No unintended coupling | Yes | scenarios and checkers only |
| No duplicated functionality | Yes | access chain, WireGuard proof and IPsec helpers reused |
| Zero-copy preserved where applicable | N-A | tooling |
| Registration over hardcoding, outbound | Yes | scenarios and the web/FTP image register |
| Registration over hardcoding, inbound | Yes | `Discover` joins names; no list edited |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | accel-ppp can be made to renegotiate LCP after a first negotiation | wholesale shape in the umbrella | the scenario needs a PPP injector, as `ppp-code-reject-xl2tpd` uses (`avprelay/pppinject.go`) | read accel-ppp's source for an LCP restart path before writing the scenario | unvalidated |
| A-2 | A synthetic VyOS configuration covering every umbrella construct exercises the guide as the owner's would | umbrella: nothing drawn from the owner's configuration | a construct of a real CPE is missed | the guide's construct inventory (child 10) is the checklist the synthetic source covers | unvalidated |
| A-3 | The FTP helper clause stays only if the owner builds helpers in (child 3) | umbrella open decision | the clause asserts the documented breakage instead | owner decision recorded in child 3 | unvalidated |
| A-4 | MOBIKE moves the IKE SA to 4500 with no NAT in the path | strongSwan behaviour with MOBIKE on | the float clause needs the IPsec NAT box | read RFC 4555 and the existing `mobike` scenario before writing the clause | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A scenario is red because its child has not landed | red on a known clause | scenarios land with their child; the order is the umbrella's |
| R-2 | The model's conversion is edited by hand later, hiding a guide gap | CPE file differs from the conversion output | `conversion` asserts every scenario uses the converted file with substitutions only |
| R-3 | Lab load on the Linux host | slow starts | full lab runs on the hosts that run full suites |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | evidence only |
| How is it reverted? | single commit revert per scenario |
| Who else touches this path? | children 2 to 21; WireGuard and IPsec validation specs |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test integration interop-topology conversion` | → | CPE configuration load and commit verifiers | scenario `conversion` |
| `./le test integration interop-topology lan-services` | → | DHCP server, firewall NAT and MSS | scenario `lan-services` |
| `./le test integration interop-topology road-warrior` | → | WireGuard config commit to kernel | scenario `road-warrior` |
| `./le test integration interop-topology mgmt-tunnel` | → | IKEv2 and VTI | scenario `mgmt-tunnel` |
| `./le test integration interop-topology access-wholesale` | → | LAC realm relay and LNS LCP renegotiation | scenario `access-wholesale` |
| `./le test integration interop-topology access-reconnect` | → | PPPoE client reconnect, IKE address follow | scenario `access-reconnect` |
| `./le test deployment docker-pppoe-accel` selector `accel-ppp-renegotiation` | → | PPPoE client LCP renegotiation | scenario `accel-ppp-renegotiation` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `conversion` | `ze config validate` clean on the converted file with lab values; Ze starts with it and its commit-time verifiers pass; the record names every VyOS source line; every topology scenario's CPE file equals the converted file after substitution |
| AC-2 | `lan-services`, lan-client | dhcpcd binds an address inside the configured range of 192.168.50.0/24 |
| AC-3 | `lan-services`, lan-res-a and lan-res-b | each binds its reserved address, one in 192.168.50.0/24 and one in 10.99.0.0/24 |
| AC-4 | `lan-services`, LAN host fetches from the web server | the server's log shows source 203.0.113.1 |
| AC-5 | `lan-services`, LAN host opens TCP to the web server with MSS 1460, then with MSS 1300 | a capture on the server shows MSS 1400 on the first SYN and 1300 unchanged on the second |
| AC-6 | `lan-services`, the internet host connects to the CPE's SSH port on 203.0.113.1, and a LAN host to the same port on the CPE's LAN address | the internet attempt gets no answer and the CPE's drop counter for that term advances; the LAN attempt completes the TCP handshake |
| AC-7 | `lan-services`, FTP from a LAN host through NAT (active mode) | the data connection completes (only if the owner builds helpers in, child 3) |
| AC-8 | `road-warrior` | spec-wireguard-runtime-proof AC-1 (kernel readback of the committed configuration) and AC-2 (handshake with PSK, traffic); wg-client reaches lan-res-a, and the web server sees source 203.0.113.1 for wg-client's fetch |
| AC-9 | `mgmt-tunnel` | Ze authenticates with an Ed25519 certificate and, in the variant, P-256; the CHILD_SA's selectors are `::/0` both sides; an IPv6 ping across vti0 loses nothing and each direction's SA counters advance on both peers; the IKE SA sits on UDP 4500; with DNS stopped at CPE boot and started later, the SA establishes after DNS answers |
| AC-10 | `access-wholesale` | the LAC completes LCP and CHAP with the CPE, selects the LNS by the realm `lab.example.net`, sends proxy AVPs; the LNS renegotiates LCP and challenges again; the CPE reaches IPCP Opened and pings the LNS |
| AC-11 | `access-reconnect`, the LNS disconnects the session and RADIUS returns a new address | the CPE comes back with the new address; the IKE SA's local address is the new one and ESP traffic resumes; the web server sees the new address as source |
| AC-12 | `accel-ppp-renegotiation` | after the first negotiation accel-ppp sends an LCP Configure-Request; the Ze client renegotiates, re-authenticates and reaches IPCP Opened again; a ping crosses the session afterwards |
| AC-13 | Each scenario's red walk (below) | red with the named failure, green when restored, both image IDs recorded |

### Red walks

| Scenario | Break (producer) | Expected red | Trap ruled out |
|----------|------------------|--------------|----------------|
| `conversion` | change one source line in the VyOS file without updating the record | record coverage clause red | - |
| `lan-services` | static-mapping lookup restricted to the first subnet (today's defect, child 2) | lan-res-b gets a range lease from the first subnet | trap 3: the reservation is in a later subnet, not the first |
| `lan-services` | the MSS action writes the value without comparing (today's defect, child 21) | the 1300 SYN arrives as 1400 | trap 2: the 1460 SYN gives a present rewrite to read beside it |
| `lan-services` | the WAN input term removed | the internet attempt completes | trap 2: the LAN attempt is the positive control |
| `road-warrior` | the CPE peer's preshared-key not installed | no handshake | - |
| `road-warrior` | masquerade term removed | the web server sees 10.88.0.x or nothing | trap 4: the server has no route to 10.88.0.0/24 except through the CPE |
| `mgmt-tunnel` | the cross-family tunnel state as today (child 20) | IPv6 ping lost while the SA is up | trap 5: counters read per direction |
| `access-wholesale` | the LNS accepts proxy LCP instead of renegotiating | no second Configure-Request from the LNS | - |
| `access-reconnect` | IKE keeps the address fixed at apply (today's defect, child 4) | ESP traffic does not resume | - |
| `accel-ppp-renegotiation` | the client drops a Configure-Request in Opened (today's defect, child 12) | session ends instead of renegotiating | - |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestConversionRecordCoversSource` | `internal/le/interoplab/topology/check_conversion_test.go` | every source line in the record; a missing one refused | |
| `TestLabValueSubstitution` | `internal/le/interoplab/topology/check_conversion_test.go` | placeholders replaced, nothing else changed; an unknown placeholder refused | |
| `TestMSSVerdict` | `internal/le/interoplab/topology/check_lan_test.go` | AC-5 from capture fixtures | |
| `TestReservationVerdict` | `internal/le/interoplab/topology/check_lan_test.go` | AC-3 from lease fixtures | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| SYN MSS against `tcp-mss-set 1400` | 536-65535 | 1400 left as 1400 | 1399 left alone | 1401 written to 1400 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `conversion` | `test/interop-topology/scenarios/` | an operator's VyOS configuration, converted, runs | |
| `lan-services` | `test/interop-topology/scenarios/` | home LAN hosts get addresses and reach the internet | |
| `road-warrior` | `test/interop-topology/scenarios/` | a laptop dials home over WireGuard | |
| `mgmt-tunnel` | `test/interop-topology/scenarios/` | the ISP's management reaches the CPE over IPsec | |
| `access-wholesale` | `test/interop-topology/scenarios/` | the line works on a wholesale network | |
| `access-reconnect` | `test/interop-topology/scenarios/` | the line comes back after the ISP drops it | |
| `accel-ppp-renegotiation` | `test/interop-pppoe/scenarios/` | wholesale renegotiation against third-party software | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `mgmt-tunnel` | `test/interop-topology/scenarios/` | strongSwan | IKEv2 EdDSA, narrowed selectors, IPv6 in IPv4 VTI, MOBIKE | |
| `road-warrior` | `test/interop-topology/scenarios/` | wireguard-tools | WireGuard with PSK from config commit | |
| `lan-services` | `test/interop-topology/scenarios/` | dhcpcd | DHCPv4 reservations across subnets | |
| `accel-ppp-renegotiation` | `test/interop-pppoe/scenarios/` | accel-ppp | LCP renegotiation in Opened | |

## Goal Validation (umbrella goals)

| Umbrella goal | Evidence this lab provides |
|---------------|----------------------------|
| 1. Ze implements every capability a home CPE of this kind uses, correctly | each capability row of the umbrella maps to a scenario clause: WAN (`access-up`, `access-reconnect`, `access-wholesale`, `accel-ppp-renegotiation`), LAN and firewall (`lan-services`), VPN (`road-warrior`, `mgmt-tunnel`), system (`conversion` starts Ze with SSH, users, NTP, name servers and the archive configured). IPv6 (child 8) and clamp-to-PMTU (child 9) are proven in their own specs |
| 2. The mapping guide makes the conversion reproducible and reviewable | `conversion`: the record covers every source line, and the configuration produced by following the guide is the one every scenario runs |
| 3. The lab proves the converted router works | every topology scenario runs the converted configuration, each with a recorded red walk |

## Files to Modify
- `docs/architecture/testing/interop.md` - the home-CPE lab section and inventory rows
- `docs/guide/l2tp.md`, `docs/guide/pppoe.md`, `docs/guide/firewall.md`, `docs/guide/ipsec.md` - name the scenario that proves each claim where a page states one
- `features/*.md` - Interop entries for the features each scenario proves, once green

## Files to Create
- `test/interop-topology/cpe/` - `vyos.conf` (invented), `ze-cpe.conf` (the model's conversion), `conversion-record.md`, `lab-values`
- `test/interop-topology/scenarios/{conversion,lan-services,road-warrior,mgmt-tunnel,access-wholesale,access-reconnect}/` - peer files
- `test/interop-pppoe/scenarios/accel-ppp-renegotiation/` - accel-ppp configuration
- `internal/le/interoplab/topology/register_lab.go`, `check_conversion.go`, `check_lan.go`, `check_vpn.go`, `check_wholesale.go` and tests
- `internal/le/interoplab/pppoe/check_renegotiation.go` and its registry entry
- `test/interop-topology/Dockerfile.webftp` - the web and FTP server image, registered

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | no Ze config added |
| YANG validation constraints | N-A | none |
| YANG custom validators | N-A | none |
| CLI commands/flags | N-A | verbs exist (child 18, PPPoE suite) |
| CLI grammar | N-A | none |
| Editor autocomplete | N-A | none |
| Functional test for new RPC/API | N-A | none |
| Pipe completeness | N-A | none |
| Env var registration | N-A | none |
| Doctor check | N-A | none |
| Prometheus counters/metrics | N-A | none |
| BGP family surface | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | Feature evidence changed? | Yes | `features/<id>.md` Interop entries; `./le feature check` |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | pages listed in Files to Modify |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior newly proven? | Yes | rows tagged by each scenario, recorded with `./le rfc record` |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | No | |
| 15 | Registered inventory changed? | No | |
| 16 | Changed source referenced by doc anchors? | No | no Ze source |
| 17 | Existing docs show examples for this area? | Yes | the guide pages' examples checked against the converted configuration |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- `conversion` scenario with the CPE file, record and substitution; checker fails until the record covers the source
2. **Phase: lan-services** -- after children 2 and 21 (helper clause after child 3)
3. **Phase: road-warrior** -- with `spec-wireguard-runtime-proof`
4. **Phase: access-reconnect** -- after child 4
5. **Phase: mgmt-tunnel** -- after children 5, 20, 6 and 4
6. **Phase: accel-ppp-renegotiation** -- after child 12
7. **Phase: access-wholesale** -- after children 14 and 15
8. **Phase: Goal Validation** -- every scenario green in one run on the full-suite host, red walks recorded

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every scenario of the umbrella's table has a home: here, child 19, or a named spec |
| Correctness | every absence clause has its positive control |
| Rule: interop-and-goal-validation | red walks forced at producers with harness-built images |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Seven scenarios | each green through its verb |
| Red walks | recorded with image IDs |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Secrets | the VyOS source and the lab values hold invented secrets only; the model's output holds placeholders |

## Owned journal rows

None: `grep -rn 'home-cpe\|road-warrior\|masquerade.*proof\|MSS.*Ze in the path' plan/journal/` finds no open row.

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One converted CPE file for every scenario | a hand-written CPE per scenario | the lab must prove the converted router, and a per-scenario file hides conversion gaps |
| `accel-ppp-renegotiation` in the PPPoE suite | move it to topology | it needs no second link or Ze node; the PPPoE suite already pairs Ze client and accel-ppp |
| Synthetic VyOS source | the owner's configuration | the umbrella keeps the owner's configuration out of the repository |

## Known Limitations

- IPv6 on the WAN and LAN (child 8) and clamp-to-PMTU (child 9) are proven in those specs; the converted configuration carries no IPv6 WAN until child 8 lands.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] AC-N rows carry testable assertions
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Every user story has a working path (N-A: tooling)
- [ ] Wiring Test table complete
- [ ] Every A-N confirmed or broken

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A** and **Commit B** per `ai/rules/planning.md`

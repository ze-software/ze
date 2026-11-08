# Spec: home-cpe-11-lab

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | Validation: the lab is the evidence that a Ze router converted from a VyOS home CPE does the job end to end (umbrella goal 3), and it carries the first runtime proof of shipped features that have none (WireGuard road warrior, masquerade and MSS with Ze in the path, the LAC relay). |
| RFC gaps | - |
| Depends | `spec-home-cpe-18-interoplab-multi-link-topology.md`, `spec-home-cpe-19-access-chain-scenario.md`, `spec-home-cpe-10-vyos-mapping-guide.md`; per scenario (table below): children 2, 3, 4, 5, 6, 12, 14, 15, 20, 21; `plan/release/validation/spec-wireguard-runtime-proof.md`; `plan/release/validation/spec-ipsec-0-umbrella.md`; `plan/release/delivery/spec-docker-hosts-run-the-ze-kernel.md` |
| Phase | 0/7 |
| Handoff | - |
| Updated | 2026-11-08 |

Parent: `plan/release/fixes/spec-home-cpe-0-umbrella.md` (child 11).

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Complete the home-CPE lab of the umbrella: every scenario of its table, run in
the `topology` suite of child 18, with the CPE running the configuration the
model converted from a VyOS configuration (child 10).

This spec composes; it does not duplicate. The access chain (`access-up`,
`access-pool`, `access-accounting`) is child 19. `access-wholesale-accel-lns`
(the Ze CPE behind the Ze realm LAC of child 14, with accel-ppp as LNS
renegotiating LCP) is child 12's scenario, on child 18's multi-link lab; it
replaces the direct `accel-ppp-renegotiation` scenario this spec planned,
because no accel-ppp setting renegotiates LCP on a direct PPPoE session
(child 12, A-4). `road-warrior` is the acceptance evidence of
`spec-wireguard-runtime-proof.md` (its AC-1 and AC-2). `mgmt-tunnel` is a
scenario under `spec-ipsec-0-umbrella.md`, whose children 5, 6, 20 and 4 own
the behaviour. Each scenario lands when its dependency child does, and its red
walk is recorded then. Clamp-to-PMTU is child 9's own scenario,
`pppoe-mss-pmtu`, which does not run the converted configuration.

### Scenarios this spec owns

| Scenario | Suite | Asserts | Depends on |
|----------|-------|---------|------------|
| `conversion` | topology | the converted CPE configuration passes `ze config validate`; Ze starts with it (commit-time verifiers run); every line of the VyOS source appears in the conversion record as mapped or dropped with a reason; the CPE file of every home-CPE lab scenario is that configuration with lab values substituted | 10 |
| `lan-services` | topology | range lease; both reservations honoured, one per subnet on the one bridge; masquerade to the web server; MSS 1400 written on a SYN carrying more and a smaller MSS left alone; WAN input dropped while the same port answers from the LAN; FTP through NAT via the helper | 2, 21, 3 |
| `road-warrior` | topology | the committed WireGuard configuration is in the kernel (interface, keys, peers, allowed-ips); wg-client with PSK handshakes with the CPE's WAN address; it reaches a LAN host and the web server, masqueraded | 18, `spec-wireguard-runtime-proof` |
| `mgmt-tunnel-ed25519` | topology | the Ed25519 proof: the CPE holds an Ed25519 key and strongSwan logs its Ed25519 verification of the CPE (`TestSwanAuthVerdict`, `TestIssueLabCPEKey`); otherwise the same clauses as `mgmt-tunnel` | 4, 5, 6, 20 |
| `mgmt-tunnel` | topology | the P-256 variant of the CPE's authentication; selectors narrowed by strongSwan to `::/0`; IPv6 ping across vti0 with ESP counters read per direction; IKE moved to UDP 4500 by MOBIKE; a CPE booted while DNS is down establishes once DNS answers | 4, 5, 6, 20 |
| `access-wholesale` | topology | LAC runs LCP and CHAP and picks the LNS by realm; LNS renegotiates LCP and re-challenges; the CPE comes up | 12, 14, 15 |
| `access-reconnect` | topology | the LNS drops the session; the CPE reconnects with a new address; the IKE SA follows the new address and masquerade uses it | 4, 18 |

### Scenarios this spec composes

| Scenario | Owner | What this spec adds |
|----------|-------|---------------------|
| `access-up`, `access-pool`, `access-accounting` | child 19 | the CPE file is the converted configuration (AC-1) |
| `access-wholesale-accel-lns` | child 12 (topology suite, child 18; Ze LAC from child 14; accel-ppp image registered by the PPPoE suite) | the CPE file is the converted configuration (AC-1), and the scenario is green in the Goal Validation run (AC-12) |

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
- [ ] `plan/release/fixes/spec-home-cpe-0-umbrella.md` "Lab", "Conversion by the model"
  → Decision: every value invented; the CPE configuration is the model's conversion with lab values.
- [ ] `plan/release/fixes/spec-home-cpe-12-pppoe-client-lcp-renegotiation.md`, Interop Tests and A-4
  → Decision: `access-wholesale-accel-lns` is child 12's scenario and its red walk is child 12's; this spec only requires that it runs the converted CPE file and is green in the Goal Validation run.
- [ ] `plan/release/fixes/spec-home-cpe-21-mss-clamp-not-raise.md`, Finding and the TDD red walks
  → Constraint: the kernel never raises an MSS through the two-byte write Ze installs, so the no-raise clause's red walk must break Ze's lowering to a write no MSS guard covers; a configured-value change is not a break. The four-byte option write child 21 names is refused by the kernel (run 2026-11-08), so the walk uses a transport-header payload write.
- [ ] `docs/architecture/testing/interop.md` "Prove a scenario discriminates", "IPsec address movement", "The IPsec NAT box", "The fifth trap" text under the vacuity table
  → Constraint: ESP proof reads each simplex SA by its own source and destination, never an SPI-keyed aggregate (`verifyESPDirections`, `internal/le/interoplab/ipsec/helpers.go`).
- [ ] `docs/guide/configuration.md`, the `ze config validate` paragraph on plugin verifiers
  → Constraint: "It does not run under `ze config validate`, which reads the schema and does not run a plugin's config verifier." The `conversion` scenario therefore also starts Ze with the configuration.
- [ ] `plan/release/validation/spec-wireguard-runtime-proof.md` AC-1, AC-2
  → Decision: `road-warrior` is that spec's evidence; this spec does not restate its criteria beyond the lab topology.
- [ ] `ai/META-DECISIONS.md` "Test systems and labs", "Test changes"
  → Constraint: guests and lab hosts run Ze's kernel; Alpine-only problems are out of scope; performance figures need a known host, every functional test runs on whatever host the agent is on.

**Key insights:**
- Several clauses assert an absence (WAN input dropped, a smaller MSS left alone); each is paired with a positive control that the same mechanism produces (vacuity trap 2).
- The CPE has one configuration; scenarios differ in which peers run and which checker runs, never in a hand-edited CPE file.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/interoplab/ipsec/helpers.go` - `verifyESPDirections`, directed ESP counters and the lossless-ping clause.
- [ ] `internal/le/test/deployment/wireguardproof.go` - the kernel-only WireGuard proof (two nodes, `Dockerfile.wireguard`); it says it does not prove the configuration-commit path.
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` - leaf `tcp-mss-set`, action `masquerade`, container `global-options`.
- [ ] `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang` - `shared-network`, `subnet`, `range`, `static-mapping`.
- [ ] `test/interop/Dockerfile.knot` - Knot plus dhcpcd 10.5.2.

**Behavior to preserve:** every existing scenario.

**Behavior to change:** none in Ze here. Each scenario's red-today clause is the dependency child's defect.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Interop scenario discovery | `internal/le/interoplab/discover.go` `Discover`; `docs/architecture/testing/interop.md` | lists scenario directories in lexical order, matches a selector by exact name, and refuses a selected directory with no registered checker | extends: each new scenario is a named directory with a registered checker, no list edited |
| Topology suite and image registry | child 18 (`./le test integration interop-topology`, images registered by name) | runs multi-node scenarios; a scenario may name an image another suite registered | extends: this spec's scenarios register there; the web and FTP image registers by name |
| Directed ESP counters | `internal/le/interoplab/ipsec/helpers.go` `verifyESPDirections` | proves ESP bytes moved on every simplex SA the caller names, read per direction, beside a lossless ping | extends: `mgmt-tunnel` and `access-reconnect` read ESP through it rather than a second counter reader |
| Kernel-only WireGuard proof | `internal/le/test/deployment/wireguardproof.go` `runWireGuardProof`, `checkWireGuardProof` | two nodes ping over kernel WireGuard; its header says it does not prove the configuration-commit path | does not apply as the proof: `road-warrior` proves the commit path that this proof disclaims, and reuses its `Dockerfile.wireguard` image |
| Child 12's wholesale renegotiation scenario | `plan/release/fixes/spec-home-cpe-12-pppoe-client-lcp-renegotiation.md`, `access-wholesale-accel-lns` | planned: accel-ppp as LNS behind the Ze LAC renegotiates LCP with the Ze CPE | composes: the scenario stays child 12's; this spec requires the converted CPE file and a green run |
| Knot and dhcpcd lab image | `test/interop/Dockerfile.knot` | one image serves `knot` (authoritative server) and `dnsclient` (kdig and dhcpcd driven over `docker exec`) | extends: the DNS node and the LAN clients run this image |
| RFC discrimination record | `./le rfc record`; `docs/contributing/rfc-conformance-gates.md` | breaks the producer, observes the tagged test red, and stores the record | extends: each scenario's red walk carrying an `RFC requirement:` tag is recorded through it |

RFC rows: this spec adds no requirement of its own. Each scenario proves the
RFC rows of the child it depends on, so the RFC gaps cell is `-`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le test integration interop-topology <scenario>`.
- The conversion inputs: an invented VyOS configuration (`test/interop-topology/cpe/vyos.conf`), the model's output (`ze-cpe.conf` with placeholders) and its conversion record, and a lab values file.

### Transformation Path
1. The conversion (done once by the model, committed) yields `ze-cpe.conf` and the record.
2. The topology plan builder substitutes lab values for placeholders and mounts the result as the CPE's configuration in every home-CPE lab scenario.
3. Each scenario starts the nodes it needs, its checker drives traffic and reads state on both ends.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| LAN client ↔ CPE | DHCPv4 on the lan link | No |
| CPE ↔ internet | through the PPP session and the LNS's routing | No |
| CPE ↔ strongSwan | IKEv2 and ESP over the PPP session | No |
| wg-client ↔ CPE | WireGuard to the CPE's PPP address | No |

### Integration Points
- Topology suite and image registry (child 18); access-chain nodes and files (child 19); `access-wholesale-accel-lns` (child 12).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | only the CPE's PPP address is reachable from the internet segment |
| No unintended coupling | Yes | scenarios and checkers only |
| No duplicated functionality | Yes | access chain, wholesale scenario, WireGuard proof and IPsec helpers reused |
| Zero-copy preserved where applicable | N-A | tooling |
| Registration over hardcoding, outbound | Yes | scenarios and the web/FTP image register |
| Registration over hardcoding, inbound | Yes | `Discover` joins names; no list edited |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-2 | A synthetic VyOS configuration covering every umbrella construct exercises the guide as the owner's would | umbrella: nothing drawn from the owner's configuration | a construct of a real CPE is missed | the guide's construct inventory (child 10) is the checklist the synthetic source covers | broken (closure, 2026-11-08): the source omits guide constructs; Pre-Commit Verification, Assumptions Resolved |
| A-4 | MOBIKE moves the IKE SA to 4500 with no NAT in the path | strongSwan behaviour with MOBIKE on | the float clause needs the IPsec NAT box | read RFC 4555 and the existing `mobike` scenario before writing the clause | confirmed: `mgmt-tunnel` passes its port-4500 clause with no NAT box |

A-1 (accel-ppp renegotiates LCP on a direct session) is gone with the direct
scenario: child 12's A-4 records that it does not. A-3 (the FTP clause
conditional on child 3) is gone: the owner decided child 3's Option A, so the
clause is unconditional.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A scenario is red because its child has not landed | red on a known clause | scenarios land with their child; the order is the umbrella's |
| R-2 | The model's conversion is edited by hand later, hiding a guide gap | CPE file differs from the conversion output | `conversion` asserts every home-CPE lab scenario uses the converted file with substitutions only |
| R-3 | Lab load on the shared Linux host | slow starts | one lab run at a time, per the one-CPU-intensive-job rule |

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
| `./le test integration interop-topology lan-services` | → | DHCP server, firewall NAT and MSS, conntrack helper | scenario `lan-services` |
| `./le test integration interop-topology road-warrior` | → | WireGuard config commit to kernel | scenario `road-warrior` |
| `./le test integration interop-topology mgmt-tunnel` | → | IKEv2 and VTI | scenario `mgmt-tunnel` |
| `./le test integration interop-topology access-wholesale` | → | LAC realm relay and LNS LCP renegotiation | scenario `access-wholesale` |
| `./le test integration interop-topology access-reconnect` | → | PPPoE client reconnect, IKE address follow | scenario `access-reconnect` |
| `./le test integration interop-topology access-wholesale-accel-lns` | → | PPPoE client LCP renegotiation against accel-ppp as LNS (child 12) | scenario `access-wholesale-accel-lns` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `conversion` | `ze config validate` clean on the converted file with lab values; Ze starts with it and its commit-time verifiers pass; the record names every VyOS source line; the CPE file of every home-CPE lab scenario (the six above, child 19's three, `access-wholesale-accel-lns`) equals the converted file after substitution |
| AC-2 | `lan-services`, lan-client | dhcpcd binds an address inside the configured range of 192.168.50.0/24 |
| AC-3 | `lan-services`, lan-res-a and lan-res-b | each binds its reserved address, one in 192.168.50.0/24 and one in 10.99.0.0/24 |
| AC-4 | `lan-services`, LAN host fetches from the web server | the server's log shows source 203.0.113.1 |
| AC-5 | `lan-services`, LAN host opens TCP to the web server with MSS 1460, then with MSS 1300 | a capture on the server shows MSS 1400 on the first SYN (the lowering clause) and 1300 unchanged on the second (the no-raise clause) |
| AC-6 | `lan-services`, the internet host connects to the CPE's SSH port on 203.0.113.1, and a LAN host to the same port on the CPE's LAN address | the internet attempt gets no answer and the CPE's drop counter for that term advances; the LAN attempt completes the TCP handshake |
| AC-7 | `lan-services`, FTP from a LAN host through NAT (active mode), with `system conntrack module ftp` in the converted configuration | the data connection completes |
| AC-8 | `road-warrior` | spec-wireguard-runtime-proof AC-1 (kernel readback of the committed configuration) and AC-2 (handshake with PSK, traffic); wg-client reaches lan-res-a, and the web server sees source 203.0.113.1 for wg-client's fetch |
| AC-9 | `mgmt-tunnel-ed25519` and `mgmt-tunnel` | Ze authenticates with an Ed25519 certificate in `mgmt-tunnel-ed25519` (the Ed25519 proof: strongSwan logs its Ed25519 verification of the CPE, judged by `TestSwanAuthVerdict`) and with P-256 in `mgmt-tunnel` (the P-256 variant); each variant hands the CPE a key of its own algorithm (`TestIssueLabCPEKey`); the CHILD_SA's selectors are `::/0` both sides; an IPv6 ping across vti0 loses nothing and each direction's SA counters advance on both peers; the IKE SA sits on UDP 4500; with DNS stopped at CPE boot and started later, the SA establishes after DNS answers |
| AC-10 | `access-wholesale` | the LAC completes LCP and CHAP with the CPE, selects the LNS by the realm `lab.example.net`, sends proxy AVPs; the LNS renegotiates LCP and challenges again; the CPE reaches IPCP Opened and pings the LNS |
| AC-11 | `access-reconnect`, the LNS disconnects the session and RADIUS returns a new address | the CPE comes back with the new address; the IKE SA's local address is the new one and ESP traffic resumes; the web server sees the new address as source |
| AC-12 | `access-wholesale-accel-lns` (child 12's scenario) | runs with the converted CPE file and is green in the Goal Validation run; its assertions and red walk are child 12's |
| AC-13 | Each scenario's red walk (below) | red with the named failure, green when restored, both image IDs recorded |

### Red walks

| Scenario | Break (producer) | Expected red | Trap ruled out |
|----------|------------------|--------------|----------------|
| `conversion` | change one source line in the VyOS file without updating the record | record coverage clause red | - |
| `lan-services` | child 2's fix undone: the static-mapping lookup in `internal/plugins/dhcpserver/segment.go` ranges over the first subnet only | lan-res-b binds a range address of 192.168.50.0/24, not its reservation 10.99.0.40 | trap 3: the reservation is in a later subnet, not the first |
| `lan-services` (lowering clause) | the `SetTCPMSS` action not lowered by the nft backend (`lower_linux.go`, the `firewall.SetTCPMSS` case returns no expression) | the 1460 SYN arrives as 1460 | trap 2: the 1300 SYN is read beside it |
| `lan-services` (no-raise clause) | `lowerSetTCPMSS` writes the register with a transport-header payload write over the MSS value (offset 22, two bytes, inet checksum at 16) in place of the TCP-option write, so no kernel MSS guard runs. The four-byte option write first planned here (child 21) is refused by the kernel, so it cannot reach the clause | the 1300 SYN arrives as 1400 | trap 2: the 1460 SYN gives a present rewrite to read beside it |
| `lan-services` | the converted configuration carries no separate WAN input term (its input chain drops by policy), so the break leaves every input-hook chain unhooked in the nft backend (`backend_linux.go`) | the internet attempt completes | trap 2: the LAN attempt is the positive control |
| `lan-services` | child 3's break `c3-ftp-no-prerouting`: the helper-assignment chain in `internal/component/firewall/conntrack_helpers.go` hooked at output instead of prerouting, so forwarded FTP never gets the helper | the active-mode data connection fails | trap 2: the control connection still completes |
| `road-warrior` | the CPE peer's preshared key not installed (`internal/plugins/iface/netlink/wireguard_linux.go`) | the kernel readback finds the CPE's wg0 peer holding no preshared key, before any handshake is read | - |
| `road-warrior` | masquerade not lowered by the nft backend | the web server sees 10.88.0.x or nothing | trap 4: the server has no route to 10.88.0.0/24 except through the CPE |
| `mgmt-tunnel` | child 20's fix undone: the tunnel-mode state keeps the outer family (`internal/component/ike/dataplane/xfrm_linux.go`, `AFUnspec` false) | IPv6 ping lost while the SA is up | trap 5: counters read per direction |
| `access-wholesale` | the LNS adopts the proxy LCP instead of renegotiating (`internal/component/l2tp/ppp/proxy.go`, the `RenegotiateProxyLCP` branch never taken) | the core capture holds no LCP Configure-Request from the LNS (`checkNegotiated`), before the session report is read | - |
| `access-reconnect` | both paths that move the IKE SA to a new WAN address are broken together: `followAddress` (`internal/component/ike/engine/dynamic_wan.go`) plans the first address and never a later change, and `serviceMobike` (`internal/component/ike/engine/mobike.go`) never re-picks the source as the original initiator. Either path alone moves the SA, so breaking one keeps the walk green | the first SA on 203.0.113.1 establishes, then the IKE SA does not follow the CPE to 203.0.113.2 | - |

### Red walk evidence (2026-11-08)

Driver `tmp/session/2026-11-07-104251/scratch/rwinv/c11-interop.sh`, overlays
from `rwinv/gen-c11.sh`, logs in `rwinv/out-c11-interop/` (each log carries the
image IDs of its run). Green runs of all six scenarios before the breaks:
`clean-<scenario>.log`, each "1 passed, 0 failed".

| Walk | Observed red (the FAIL line) | Judged against the row |
|------|------------------------------|------------------------|
| `c11-conversion-line-changed` | record row 1 quotes "...description 'WAN'", but vyos.conf line 1 reads "...description 'WAN uplink'" | the named clause. `vyos.conf` restored and cmp-checked against HEAD |
| `c11-reservation-first-subnet` | lan-res-b bound 192.168.50.101, want its reservation 10.99.0.40 | the named clause |
| `c11-mss-not-lowered` | the SYN offered MSS 1460 and arrived with MSS 1460, want 1400 | the named clause |
| `c11-mss-raises` (four-byte option write) | ze-cpe's pppoe0 never held 203.0.113.1: the firewall flush failed with `netlink receive: operation not supported`, so the kernel refused the rule and Ze never came up | NOT the clause: this walk proves only that the kernel refuses the break. The row now names the payload-write break (`c11-mss-raises-payload`), owed a run. Child 21's own AC-2 walk does not cover it: it is still owed and planned on the same refused write (journal row in `plan/journal/design-claim-never-measured.md`, owned by child 21) |
| `c11-input-unhooked` | the web server completed TCP to the CPE's SSH port on 203.0.113.1 | the named clause |
| `c3-ftp-no-prerouting` | curl exit 30: control completed (220, 230, 257), then vsftpd answered "500 Illegal PORT command" to the unrewritten 192.168.50.10 | the named clause, with its control |
| `c11-wg-psk-dropped` | the CPE's wg0 peer holds no preshared key | the corrected clause (readback first) |
| `c11-masquerade-not-lowered` | wg-client fetched nothing from the web server across the tunnel | the named clause |
| `c11-vti-af-unspec-off` | the CPE's IPv6 ping across vti0 to fd00:db8:1::1 lost packets | the named clause |
| `c11-proxy-lcp-adopted` | the capture holds no frame of the kind the check reads: no LCP Configure-Request from 192.0.2.3 | the corrected clause (`checkNegotiated` reads the core capture before `renegotiatedVerdict`) |
| `c11-ike-address-fixed` (no plan on any address change) | strongSwan holds no established IKE SA with the CPE on 203.0.113.1 | NOT the clause: the CPE gets its first address after the IKE apply, so this break fails the first SA and never reaches the reconnect. Superseded by `c11-ike-change-not-followed` |
| `c11-mss-raises-payload` (transport-header payload write, `rwinv/c11-narrow2.log`, log `c11-mss-noraise-payload-broken.log`) | the SYN to 198.51.100.10.9102: offered MSS 1300 and arrived with MSS 1400, want 1300 | the named clause. The 9101 probe (1460 lowered to 1400) runs first in `lanMSSProbes` order and passed, so the rewrite was present beside the red |
| `c11-ike-change-not-followed` (`followAddress` plans the first address only, and `serviceMobike`'s re-pick disabled; log `c11-reconnect-change-broken.log`) | the IKE SA did not follow the CPE to 203.0.113.2: strongSwan's IKE SA sits on CPE address 203.0.113.1, want 203.0.113.2 | the named clause, after the first SA on 203.0.113.1 established. An earlier run of the same walk with only the `followAddress` break stayed green, because MOBIKE alone moved the SA; the row names both producers |

Address-order defect met during the walks: in 2 of 11 lab runs (the count in `3745936efa`'s message) br0 came up with
10.99.0.1 as its primary and lan-client leased from the wrong subnet, because
iface installed a unit's addresses in map order. Fixed by `3745936efa`
(`desiredState`, `internal/component/iface/config_apply.go`); the reorder of a
live interface it leaves open is owned by
`plan/release/fixes/spec-iface-address-reorder-moves-primary.md`.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestConversionRecordCoversSource` | `internal/le/interoplab/topology/check_conversion_test.go` | every source line in the record; a missing one refused | |
| `TestLabValueSubstitution` | `internal/le/interoplab/topology/check_conversion_test.go` | placeholders replaced, nothing else changed; an unknown placeholder refused | |
| `TestMSSVerdict` | `internal/le/interoplab/topology/check_lan_test.go` | AC-5 from capture fixtures: the lowering clause and the no-raise clause judged separately | |
| `TestReservationVerdict` | `internal/le/interoplab/topology/check_lan_test.go` | AC-3 from lease fixtures | |
| `TestIssueLabCPEKey` | `internal/le/interoplab/topology/home_lab_test.go` | AC-9: each mgmt-tunnel variant issues the CPE a key of its own algorithm (P-256 or Ed25519) that parses as Ze's pki leaf parses it; a scenario naming no algorithm is refused | pass (`scratch/c1118v.log`) |
| `TestSwanAuthVerdict` | `internal/le/interoplab/topology/home_lab_test.go` | AC-9: the Ed25519 variant needs strongSwan's Ed25519 verification line for the CPE; the P-256 ECDSA line and another identity's Ed25519 line are refused | pass (`scratch/c1118v.log`) |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| SYN MSS against `tcp-mss-set 1400` | 536-65535 | 1400 left as 1400 | 1399 left alone | 1401 written to 1400 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `conversion` | `test/interop-topology/scenarios/conversion/` | an operator's VyOS configuration, converted, runs | |
| `lan-services` | `test/interop-topology/scenarios/lan-services/` | home LAN hosts get addresses and reach the internet | |
| `road-warrior` | `test/interop-topology/scenarios/road-warrior/` | a laptop dials home over WireGuard | |
| `mgmt-tunnel` | `test/interop-topology/scenarios/mgmt-tunnel/` | the ISP's management reaches the CPE over IPsec | |
| `access-wholesale` | `test/interop-topology/scenarios/access-wholesale/` | the line works on a wholesale network | |
| `access-reconnect` | `test/interop-topology/scenarios/access-reconnect/` | the line comes back after the ISP drops it | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `mgmt-tunnel-ed25519` | `test/interop-topology/scenarios/mgmt-tunnel-ed25519/` | strongSwan | IKEv2 Ed25519 (RFC 8420), narrowed selectors, IPv6 in IPv4 VTI, MOBIKE | pass (`rwinv/topo-full-ac12.log`, [13/18]) |
| `mgmt-tunnel` | `test/interop-topology/scenarios/mgmt-tunnel/` | strongSwan | the P-256 variant: IKEv2 ECDSA, narrowed selectors, IPv6 in IPv4 VTI, MOBIKE | pass (`rwinv/topo-full-ac12.log`, [12/18]) |
| `road-warrior` | `test/interop-topology/scenarios/road-warrior/` | wireguard-tools | WireGuard with PSK from config commit | |
| `lan-services` | `test/interop-topology/scenarios/lan-services/` | dhcpcd; the FTP daemon of the web and FTP image | DHCPv4 reservations across subnets; active FTP through masquerade with the helper | |
| `access-wholesale-accel-lns` | owned by child 12, topology suite | accel-ppp as LNS | LCP renegotiation in Opened against third-party software | |

## Goal Validation (umbrella goals)

| Umbrella goal | Evidence this lab provides |
|---------------|----------------------------|
| 1. Ze implements every capability a home CPE of this kind uses, correctly | each capability row of the umbrella maps to a scenario clause: WAN (`access-up`, `access-reconnect`, `access-wholesale`, `access-wholesale-accel-lns`), LAN and firewall (`lan-services`), VPN (`road-warrior`, `mgmt-tunnel`), system (`conversion` starts Ze with SSH, users and name servers configured; NTP and the archive are not in the synthetic source, owned by `plan/release/validation/spec-home-cpe-lab-guide-construct-coverage.md`). IPv6 is proven by child 8's own tests, and clamp-to-PMTU by child 9's `TestTCPMSSClampToPMTU` and its scenario `pppoe-mss-pmtu` |
| 2. The mapping guide makes the conversion reproducible and reviewable | `conversion`: the record covers every source line, and the configuration produced by following the guide is the one every home-CPE lab scenario runs |
| 3. The lab proves the converted router works | every home-CPE lab scenario runs the converted configuration, each with a recorded red walk |

## Files to Modify
- `docs/architecture/testing/interop.md` - the home-CPE lab section and inventory rows
- `docs/guide/l2tp.md` (the LAC relay claim: `access-wholesale`), `docs/guide/pppoe.md` (the client claims: `access-reconnect`), `docs/guide/firewall.md` (masquerade and `tcp-mss-set`: `lan-services`), `docs/guide/ipsec.md` (VTI and MOBIKE: `mgmt-tunnel`), `docs/guide/configuration.md` (the conntrack section: `lan-services`) - each names the scenario that proves its claim
- `features/*.md` - Interop entries for the features each scenario proves, once green (`./le feature check`)

## Files to Create
- `test/interop-topology/cpe/` - `vyos.conf` (invented), `ze-cpe.conf` (the model's conversion), `conversion-record.md`, `lab-values`
- `test/interop-topology/scenarios/{conversion,lan-services,road-warrior,mgmt-tunnel,access-wholesale,access-reconnect}/` - peer files
- `internal/le/interoplab/topology/register_lab.go`, `check_conversion.go`, `check_lan.go`, `check_vpn.go`, `check_wholesale.go` and their tests
- `test/interop-topology/Dockerfile.webftp` - the web and FTP server image, registered

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | no Ze config added |
| YANG validation constraints | N-A | none |
| YANG custom validators | N-A | none |
| CLI commands/flags | N-A | verbs exist (child 18) |
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
2. **Phase: lan-services** -- after children 2, 21 and 3
3. **Phase: road-warrior** -- with `spec-wireguard-runtime-proof`
4. **Phase: access-reconnect** -- after child 4
5. **Phase: mgmt-tunnel** -- after children 5, 20, 6 and 4
6. **Phase: access-wholesale** -- after children 14 and 15
7. **Phase: Goal Validation** -- every home-CPE lab scenario green in one run, `access-wholesale-accel-lns` included once child 12 lands it, red walks recorded

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every scenario of the umbrella's table has a home: here, child 19, child 12, or a named spec |
| Correctness | every absence clause has its positive control |
| Rule: interop-and-goal-validation | red walks forced at producers with harness-built images |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Six scenarios owned here | each green through `./le test integration interop-topology <scenario>` |
| Composed scenarios | child 19's three and `access-wholesale-accel-lns` green with the converted CPE file |
| Red walks | recorded with image IDs |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Secrets | the VyOS source and the lab values hold invented secrets only; the model's output holds placeholders |

## Closure evidence (2026-11-08)

The blocker this spec was handed back with (two red walks that went red
before their clause) is cleared. Logs are under
`tmp/session/2026-11-07-104251/scratch/`.

| Evidence | Where | What it shows |
|----------|-------|---------------|
| Address-order fix | `3745936efa` (`desiredState`, `internal/component/iface/config_apply.go`; `decomposeIfaceOperations`, `operation.go`) | a unit's addresses install in configured order, so br0's primary is 192.168.50.1 on every run |
| Narrow red walks | `rwinv/c11-narrow2.log` | `c11-mss-raises-payload` and `c11-ike-change-not-followed` red for their named reasons (Red walk evidence) |
| AC-12, one full run | `rwinv/topo-full-ac12.log` | the whole `interop-topology` suite, 18 passed, 0 failed, after `3745936efa`: every scenario this spec owns, child 19's three and `access-wholesale-accel-lns` |
| Host units | `scratch/c1118v.log`, `scratch/c19-topo-all.log` | `TestIssueLabCPEKey`, `TestSwanAuthVerdict` pass; the topology package passes |
| Scoped packages, at closure | `./le job run ... go test -count=1 ./internal/component/iface/ ./internal/le/interoplab/topology/` | both `ok` |

## Owned journal rows

This lab exposed the address-order row below, so this spec owns it. The open
row it also exposed (a live leaf-list reorder leaves the old primary) moved to
`plan/release/fixes/spec-iface-address-reorder-moves-primary.md`.

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `output-not-byte-stable.md` | 2026-11-08 | fixed: iface installed a unit's addresses in map order, so br0's primary varied |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One converted CPE file for every home-CPE lab scenario | a hand-written CPE per scenario | the lab must prove the converted router, and a per-scenario file hides conversion gaps |
| accel-ppp renegotiation proven by child 12's `access-wholesale-accel-lns` | a direct accel-ppp scenario in the PPPoE suite | no accel-ppp setting renegotiates LCP on a direct PPPoE session (child 12, A-4); behind a LAC accel-ppp ignores proxy LCP and renegotiates |
| Synthetic VyOS source | the owner's configuration | the umbrella keeps the owner's configuration out of the repository |

## Known Limitations

- IPv6 on the WAN and LAN (child 8) and clamp-to-PMTU (child 9, `pppoe-mss-pmtu`) are proven in those specs; the converted configuration carries no IPv6 WAN until child 8 lands, and it uses a fixed `tcp-mss-set 1400`, so `lan-services` does not exercise the route-MTU action.

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
- [ ] The targeted tests for what changed pass: `./le job run label unit-pkg command go test -count=1 ./internal/le/interoplab/topology/` for the checkers, each scenario through `./le test integration interop-topology <scenario>`, and `./le verify changed` over the closing commits

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior (N-A: the lab's scenarios are interop scenarios)
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A** and **Commit B** per `ai/rules/planning.md`

## Implementation Summary

### What Was Implemented
- The six scenarios this spec owns (`conversion`, `lan-services`, `road-warrior`, `mgmt-tunnel` with its `mgmt-tunnel-ed25519` variant, `access-wholesale`, `access-reconnect`), their checkers in `internal/le/interoplab/topology/` (`checkConversion`, `checkLANServices`, `checkRoadWarrior`, `checkMgmtTunnel`, `checkMgmtTunnelEd25519`, `checkAccessWholesale`, `checkAccessReconnect`), the registration in `register_home_lab.go`, the CPE files under `test/interop-topology/cpe/` and the web and FTP image `test/interop-topology/home-lab/Dockerfile.webftp`. Landed in `a0e83e12a1` and the `interop-topology:` commits after it.
- Every red walk run against its producer, eleven red at their named clause (Red walk evidence).

### Bugs Found/Fixed
- iface installed a unit's addresses in map order, so br0's primary varied and lan-client leased from the wrong subnet: fixed by `3745936efa`, covered by `config_apply_order_test.go`.
- The no-raise walk first planned (four-byte option write) is refused by the kernel; the walk moved to a transport-header payload write. The reconnect walk first planned broke one of two paths that move the SA; the walk now breaks both.

### Documentation Updates
- `docs/guide/l2tp.md` (Relay by realm: `access-wholesale`), `docs/guide/pppoe.md` (keepalive and redial: `access-reconnect`), `docs/guide/firewall.md` (TCP MSS Clamping: `lan-services`), `docs/guide/ipsec.md` (IPv6 inside an IPv4 tunnel on a VTI: `mgmt-tunnel`, `mgmt-tunnel-ed25519`), `docs/guide/configuration.md` (Connection Tracking: `lan-services`), each with a source anchor to its checker.
- `features/` Interop entries: `ipsec-mobike.md`, `ikev2-engine.md` (`topology/mgmt-tunnel`, `topology/mgmt-tunnel-ed25519`), `dhcp-server-shared-segment.md` (`topology/lan-services`), `firewall.md` (new Interop row: `topology/lan-services`, `topology/road-warrior`), `pppoe-access.md` (`topology/access-reconnect`), `l2tpv2-bng.md` (`topology/access-wholesale`), `interfaces.md` (`topology/road-warrior`).
- `docs/architecture/testing/interop.md` already carried the home-CPE lab rows (grep names all seven scenarios).
- `./le doc check all`: red on command-contract findings (`clear debug` operator availability) that name none of these pages; no finding names an edited page or anchor.

### Deviations from Plan
- `register_lab.go` is `register_home_lab.go`; `test/interop-topology/Dockerfile.webftp` is `test/interop-topology/home-lab/Dockerfile.webftp`.
- The no-raise red walk uses the payload write, not the option write child 21 named (the kernel refuses that one).
- A-2 broken: the synthetic source covers part of the guide's inventory, homed in a new spec (Work Not Done).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2: the synthetic VyOS source was taken to cover every construct the guide maps | it omits address and port groups, `ip-src-route`, `clamp-mss-to-pmtu`, `ipv6 adjust-mss`, router advertisements, NTP, the commit archive and three dropped rows | closure grep of each guide construct against `vyos.conf` | new spec `plan/release/validation/spec-home-cpe-lab-guide-construct-coverage.md` |
| approach | the reconnect walk broke `followAddress` alone | MOBIKE's re-pick in `serviceMobike` moves the SA by itself | the first narrow walk stayed green | the walk breaks both paths |
| approach | the guide pages and feature Interop entries named in Files to Modify were left to closure | they belonged to the phase that landed each scenario | closure documentation review | written at closure |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| every scenario of the table, in the topology suite, on the converted configuration | Done | `register_home_lab.go`, `homeCPEScenariosConverted` (`check_conversion.go`) | 18 passed in one run |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `conversion` pass, `checkConversion` | |
| AC-2 | Done | `lan-services` pass, `checkLease` | |
| AC-3 | Done | `lan-services` pass; walk `c11-reservation-first-subnet` red | |
| AC-4 | Done | `lan-services` pass, `checkMasquerade` | |
| AC-5 | Done | `lan-services` pass, `lanMSSProbes`; walks `c11-mss-not-lowered`, `c11-mss-raises-payload` red | |
| AC-6 | Done | `lan-services` pass, `checkWANInputDropped`; walk `c11-input-unhooked` red | |
| AC-7 | Done | `lan-services` pass, `checkActiveFTP`; walk `c3-ftp-no-prerouting` red | |
| AC-8 | Done | `road-warrior` pass, `checkRoadWarrior`; walks `c11-wg-psk-dropped`, `c11-masquerade-not-lowered` red | |
| AC-9 | Done | `mgmt-tunnel`, `mgmt-tunnel-ed25519` pass; `TestIssueLabCPEKey`, `TestSwanAuthVerdict` pass; walk `c11-vti-af-unspec-off` red | |
| AC-10 | Done | `access-wholesale` pass, `checkAccessWholesale`; walk `c11-proxy-lcp-adopted` red | |
| AC-11 | Done | `access-reconnect` pass, `checkAccessReconnect`; walk `c11-ike-change-not-followed` red | |
| AC-12 | Done | `access-wholesale-accel-lns` pass in `rwinv/topo-full-ac12.log` | |
| AC-13 | Done | Red walk evidence, every row red at its clause; image IDs in each log | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestConversionRecordCoversSource`, `TestLabValueSubstitution` | Done | `check_conversion_test.go` | topology package ok |
| `TestMSSVerdict`, `TestReservationVerdict` | Done | `check_lan_test.go` | topology package ok |
| `TestIssueLabCPEKey`, `TestSwanAuthVerdict` | Done | `home_lab_test.go` | `scratch/c1118v.log` |
| the six scenarios | Done | `test/interop-topology/scenarios/` | `rwinv/topo-full-ac12.log` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `test/interop-topology/cpe/` (four files) | Done | |
| `test/interop-topology/scenarios/{conversion,lan-services,road-warrior,mgmt-tunnel,access-wholesale,access-reconnect}/` | Done | plus `mgmt-tunnel-ed25519/` |
| `register_lab.go` | Changed | `register_home_lab.go` |
| `check_conversion.go`, `check_lan.go`, `check_vpn.go`, `check_wholesale.go` and tests | Done | |
| `Dockerfile.webftp` | Changed | under `test/interop-topology/home-lab/` |
| guide pages, `features/*.md`, `interop.md` | Done | closure documentation review |

### Audit Summary
- **Total items:** 26
- **Done:** 24
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| every scenario of the table green in the topology suite on the converted configuration | interop | `rwinv/topo-full-ac12.log`: `interop: 18 passed, 0 failed`, including the seven scenarios this spec owns and the four it composes |
| each scenario's assertions discriminate | interop red walks | Red walk evidence: every walk red at its named clause, logs in `rwinv/out-c11-interop/` with image IDs |
| the CPE runs the model's conversion, not a hand-edited file | interop | `conversion` pass: `homeCPEScenariosConverted` refuses a home-CPE scenario with its own CPE file; walk `c11-conversion-line-changed` red |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| A live reorder of an interface's addresses moves its primary (journal row in `plan/journal/output-not-byte-stable.md`, open) | found by this lab; the fix for a fresh install landed in `3745936efa`, the live reorder is a separate reconcile change | `plan/release/fixes/spec-iface-address-reorder-moves-primary.md` |
| The synthetic VyOS source covers every construct the guide maps or drops, with a check (A-2 broken) | found at closure | `plan/release/validation/spec-home-cpe-lab-guide-construct-coverage.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/home-cpe-11-lab-7e292f27-ced0-427f-a7b6-3a931db211f9.md` (28 files, verdict clean) |
| `./le spec review check` | clean |
| Rounds | 1 |
| Reviewer lenses used | logic and ordering of the iface fix (`desiredState`, the reconcile add and remove passes, `addDesiredAddresses`, `decomposeIfaceOperations`); docs claims against producers; spec record against the logs; style pass over the changed Go (no `panic`, no new goroutine, no allocation on a hot path beyond the per-commit lists) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Goal Validation claimed `conversion` starts Ze with NTP and the archive configured; neither is in `vyos.conf` | this spec, Goal Validation (umbrella goals) row 1 | corrected; gap homed in `spec-home-cpe-lab-guide-construct-coverage.md` |
| 2 | ISSUE | Files to Modify promised scenario sentences on five guide pages and feature Interop entries; none existed | `docs/guide/{l2tp,pppoe,firewall,ipsec,configuration}.md`, `features/*.md` | written at closure, each anchored to its checker |

NOTEs: `slices.Contains` in the reconcile passes is quadratic in a device's
address count, which is a handful per commit; the feature check now asks a
fresh Doc review of `firewall.md`, `ikev2-engine.md`, `ipsec-mobike.md` and
`pppoe-access.md`, whose descriptions this closure did not re-read (named in
the commit).

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/interop-topology/cpe/{vyos.conf,ze-cpe.conf,conversion-record.md,lab-values}` | Yes | `ls test/interop-topology/cpe/` lists all four |
| the seven scenario directories | Yes | `ls -d test/interop-topology/scenarios/...` lists all seven |
| `register_home_lab.go`, `check_conversion.go`, `check_lan.go`, `check_vpn.go`, `check_wholesale.go` | Yes | `ls internal/le/interoplab/topology/` |
| `test/interop-topology/home-lab/Dockerfile.webftp` | Yes | `git ls-files` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-12 | scenarios green | `rwinv/topo-full-ac12.log`: `[1/18]`..`[18/18]` pass, `18 passed, 0 failed` |
| AC-5, AC-11 | the two narrow walks red at their clause | `rwinv/c11-narrow2.log` FAIL lines quoted in Red walk evidence |
| AC-9 | host units | `scratch/c1118v.log` PASS; topology package `ok` at closure |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le test integration interop-topology <scenario>` (seven rows) | no `.ci`: interop scenarios registered in `register_home_lab.go` | each listed pass in `rwinv/topo-full-ac12.log` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-2 | broken | grep of the guide's construct rows against `vyos.conf`: `address-group`, `port-group`, `ip-src-route`, `clamp-mss-to-pmtu`, `router-advert`, `ntp server`, `commit-archive` and others absent; Mistake Log, Deviations, Work Not Done |
| A-4 | confirmed | `check_vpn.go` refuses an IKE SA whose port is not 4500 (`sa.remotePort != "4500"`), and `mgmt-tunnel` passes with no NAT box in its topology |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| guide sentences on five pages | each anchors its checker (`checkAccessWholesale`, `checkAccessReconnect`, `checkLANServices`/`checkMasquerade`, `checkMgmtTunnel`/`checkMgmtTunnelEd25519`, `checkActiveFTP`); `lanMSSProbes` holds 1460→1400 and 1300→1300 | Yes |
| feature Interop entries | `./le feature check` raises no Interop refusal for the seven edited files | Yes |
| RFC behavior newly proven (checklist row 9) | no scenario carries an `RFC requirement:` tag (grep of `internal/le/interoplab/topology/`), so no record is owed | Yes |

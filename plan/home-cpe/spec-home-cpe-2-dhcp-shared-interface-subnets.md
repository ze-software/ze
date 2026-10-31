# Spec: home-cpe-2-dhcp-shared-interface-subnets

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P1 |
| Release reason | Release fix: an accepted configuration (two subnets on one LAN, a static reservation in the second) hands the reserved client a pool lease from the first subnet, and every listener answers for every subnet of every interface |
| RFC gaps | RFC 2131 Section 4.1: `RFC2131-4.1-1` (accept any of the server's addresses as its identifier) and `RFC2131-4.1-2` (server identifier reachable from the client), both recorded as gaps in `rfc/short/rfc2131.md` |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-31 |

Parent: `plan/home-cpe/spec-home-cpe-0-umbrella.md` (child 2).

Recovery after compaction: `.claude/rules/post-compaction.md`.

Every address, name and MAC in this spec is invented: RFC 1918 LAN ranges,
`example.net`, locally administered MACs.

## Task

A home CPE commonly serves two IPv4 subnets on one LAN bridge, for example
192.168.50.0/24 (the household) and 10.99.0.0/24 (devices kept apart by
policy), with static reservations in each. Ze's DHCPv4 server accepts that
configuration and then does the wrong thing with it:

1. A client whose MAC is reserved in the second subnet sends a DHCPDISCOVER
   with no `ciaddr`, no `giaddr` and no Requested IP Address. The first
   subnet's handler claims the packet and offers a lease from its own pool. The
   reservation is never consulted.
2. Every listener answers for every configured subnet, whatever interface the
   request arrived on, so a second LAN interface hands out addresses of the
   first LAN's subnets.
3. With no `default-router`, the server identifier (option 54) is the first
   pool address, which the server itself leases to a client, or the subnet's
   network address, which is no host (`RFC2131-4.1-2`). A DHCPREQUEST naming
   another of the server's own addresses is dropped (`RFC2131-4.1-1`).

The goal: for each request the server chooses the subnet the client belongs
to, following RFC 2131 (relay address, client address, requested address),
honours a static reservation in any subnet the receiving segment serves before
it touches a pool, allocates from the subnet the receiving interface is
addressed in, and identifies itself with an address the client can reach. An
optional `server-identifier` leaf lets the operator name that address.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/configuration.md` (DHCP server section, `shared-network`, `subnet`, `static-mapping`, about line 4079) - the operator's view of the tree
  → Constraint: `shared-network` is documented as a "named grouping of subnets"; nothing documents which subnet a request is served from. This spec gives it the ISC and Kea meaning: the subnets of one shared network share one physical segment
  → Decision: the page gains a "Several subnets on one interface" subsection with the selection order of this spec
- [ ] `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang` - the container description says "every listener answers for every configured subnet"
  → Constraint: that sentence describes the defect; it is rewritten in the same change (stale-comments, documentation rules)
- [ ] `ai/doors/system.md` and the door covering `internal/plugins/` - read before the first code edit

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2131.md` (present) - rows `RFC2131-4.1-1`, `RFC2131-4.1-2` carry `{gap: ...}` annotations that name this defect at `subnetServerIP` and `handleRequest`
  → Constraint: RFC 2131 Section 4.1: "a server MUST choose an address as a 'server identifier' that, to the best of the server's knowledge, is reachable from the client."
  → Constraint: RFC 2131 Section 4.1: "if the DHCP server and the DHCP client are connected to the same subnet (i.e., the 'giaddr' field in the message from the client is zero), the server SHOULD select the IP address the server is using for communication on that subnet as the 'server identifier'."
  → Constraint: RFC 2131 Section 4.3.1, allocation order, last step: "A new address allocated from the server's pool of available addresses; the address is selected based on the subnet from which the message was received (if 'giaddr' is 0) or on the address of the relay agent that forwarded the message ('giaddr' when not 0)."
  → Decision: RFC 2131 Section 4.3.1 also says: "Note that, in some network architectures (e.g., internets with more than one IP subnet assigned to a physical network segment), it may be the case that the DHCP client should be assigned an address from a different subnet than the address recorded in 'giaddr'." and "A server is free to choose some other subnet, and it is beyond the scope of the DHCP specification to describe ways in which the assigned IP address might be chosen." A reservation in any subnet of the segment's shared network therefore wins over the receiving subnet's pool; that is the administrative choice Section 4.2 permits
  → Constraint: RFC 2131 Section 3.2 (DHCPREQUEST in INIT-REBOOT): "If the client's request is invalid (e.g., the client has moved to a new subnet), servers SHOULD respond with a DHCPNAK message to the client."

**Key insights:**
- The selection unit is the segment, not the subnet: the receiving interface (or `giaddr`) names a segment; the segment is every subnet of the shared network that holds the interface's (or relay's) address.
- Static reservations are global to the segment; pools are per subnet.
- ISC dhcpd and Kea both give `shared-network` this meaning, which is what a VyOS configuration (Kea underneath) expects.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/dhcpserver/register.go` - `bindListeners` (l.191) starts `dhcpServe` on every listener with the full handler list; `serveRequests` (l.251) offers each packet to each handler in configuration order and the first non-nil reply wins
- [ ] `internal/plugins/dhcpserver/config_tx.go` - `buildHandlers` (l.126) makes one handler per subnet of every shared network, with no interface association; `subnetServerIP` (l.193) answers `default-router`, else the first range start, else the subnet address
- [ ] `internal/plugins/dhcpserver/handler.go` - `handle` (l.106) calls `matchesSubnet` (l.143), which answers by `giaddr`, then `ciaddr`, then Requested IP, and answers true when all three are absent; `handleDiscover` (l.160) calls `allocateForClient` (l.219), which checks only its own subnet's `staticByMAC`, then its pool; `handleRequest` (l.168) drops a SELECTING request whose option 54 differs from its single `serverIP`, and in INIT-REBOOT commits any requested address inside its prefix without consulting a reservation held for the MAC elsewhere
- [ ] `internal/plugins/dhcpserver/config.go` - `serverConfig`, `sharedNetwork`, `subnetConfig`, `staticMapping`; no interface or server-identifier field
- [ ] `test/dhcp/dhcp-server-multi-subnet.ci` - parse-only; no test serves two subnets
- [ ] `test/interop/scenarios/dns-resolver-dhcp-lan/` - dhcpcd against Ze, one subnet

**Behavior to preserve:**
- One subnet per interface: unchanged leases, unchanged option 54 when `default-router` is the interface's address.
- Relay handling: `giaddr` non-zero selects by the relay's address and replies to the relay (`replyDestination`).
- Lease and pool sharing across reloads (`samePlanHandler`, `sameAddressPlan`).
- RFC 2131 Section 4.3.1 silence when no address is available.

**Behavior to change:**
- Subnet choice per request, reservations across the segment first (below).
- A listener serves only the segments its interface is addressed in.
- Option 54 is an address the server holds on the segment, or the configured `server-identifier`.
- A SELECTING DHCPREQUEST naming any of the server's own addresses on the segment is accepted.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A DHCPv4 packet on the UDP 67 socket bound to the kernel device of a `listen-interface` (`listenInterface`, `SO_BINDTODEVICE`).
- Config: `service dhcp-server` tree, delivered to the plugin as SDK config sections.

### Transformation Path
1. Config: `parseConfig` builds shared networks and subnets; a new optional `server-identifier` leaf per subnet is parsed and checked as an IPv4 unicast address inside the subnet.
2. Bind: each listener records its kernel device. The device's IPv4 addresses are read from netlink at bind and refreshed on address change (the plugin already learns its device through `dhcpResolveDevice`).
3. Request: the listener resolves the segment. With `giaddr` set: the shared network holding a subnet that contains `giaddr`. With `giaddr` zero: the shared networks holding a subnet that contains an address of the receiving device. No match: no reply, one rate-limited log line naming the interface and the fix.
4. Selection within the segment, in order:
   1. `ciaddr` set (RENEWING, REBINDING): the segment's subnet containing it, or a DHCPNAK if none.
   2. Requested IP present: the segment's subnet containing it; INIT-REBOOT outside every segment subnet is answered by DHCPNAK (Section 3.2).
   3. A static reservation for the MAC in any subnet of the segment: that subnet and that address. A requested address different from the reservation gets a DHCPNAK, so the client restarts and is offered its reservation.
   4. Otherwise the pool of the segment's subnet that contains the receiving device's primary address (relay: the subnet containing `giaddr`), then the pools of the other segment subnets in configuration order when it is exhausted.
5. Reply: option 54 is the subnet's `server-identifier` when set, else the device's address inside the chosen subnet, else the device's primary address on the segment. Section 4.1's reachability holds in each case because the address is on the client's segment.
6. DHCPREQUEST (SELECTING): accepted when option 54 equals any of the server's addresses on the segment or a configured `server-identifier` of the segment.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to plugin | SDK config sections, YANG-validated tree | No |
| Plugin to kernel | netlink address dump and address events for the listen device | No |
| Plugin to wire | DHCPv4 UDP on the bound device | No |

### Integration Points
- `serveRequests` - replaces "first handler that answers" with one segment resolver and one selection step, then dispatches to the chosen subnet's handler.
- `dhcpHandler` - keeps its pool and lease table; its `allocateForClient` stops being the only reservation check.
- `subnetServerIP` - deleted; option 54 comes from the selection step (no-layering: the old fallback chain is removed, not kept beside the new one).
- Netlink address reads: reuse the plugin's existing netlink use (`neighbor_linux.go`) rather than a second client.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes (design) | selection stays inside the plugin's request task |
| No unintended coupling (components stay isolated) | Yes (design) | the interface addresses come from netlink, never from a Go call into the iface plugin (which may run external) |
| No duplicated functionality (extends existing, does not recreate) | Yes (design) | pools, lease tables and reply building are reused |
| Zero-copy preserved where applicable (refs, not copies) | N-A | request path already copies one packet per read |
| Registration over hardcoding, outbound | N-A | no new command, family or handler type |
| Registration over hardcoding, inbound | Yes (design) | one new leaf; no central list learns it beyond the plugin's own parser |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The listen interface holds an IPv4 address inside at least one configured subnet in every working deployment | RFC 2131 Section 4.1 SHOULD (server uses its address on the subnet); ISC and Kea refuse to serve a segment they hold no address on | configs that relied on serving a subnet the router is not addressed in stop answering | grep of `test/dhcp/*.ci` and the interop scenarios for listen devices with no address in the subnet, before Phase 2 | unvalidated |
| A-2 | `shared-network` groups subnets of one segment in operator intent | ISC/Kea semantics; VyOS `shared-network-name` | a config with two segments in one shared network would mix reservations across segments | read every `shared-network` example in `docs/` and `test/` | unvalidated |
| A-3 | dhcpcd sends DISCOVER with no Requested IP on first boot and an INIT-REBOOT REQUEST with its old address on restart | dhcpcd behaviour in the existing scenario | the interop test would not exercise step 3 or step 2 | the interop scenario captures both | unvalidated |
| A-4 | A reload that adds an address to the listen device is seen without rebinding | netlink address events | a new subnet stays unserved until reload | unit test with a fake address source plus the `.ci` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Existing functional tests serve from a device with no address in the subnet (A-1) | `.ci` red after Phase 2 | give the test device an address in the subnet; never fall back to "answer anyway", which is the defect |
| R-2 | A client holding a pool lease in subnet 1 keeps it after its reservation in subnet 2 is added | lease table shows both | step 3 NAKs the renewing address when a reservation exists for the MAC on the segment, so the client restarts and takes the reservation |
| R-3 | Address refresh races a request | flaky test | address set read under the listener's own state, replaced atomically |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | LAN clients get no lease, or a wrong one; DHCP-provisioned appliances (PXE install path) stop booting |
| How is it reverted? | single commit revert; no config migration (the new leaf is optional) |
| Who else touches this path? | `release/fixes/spec-network-services-one-off-defects.md` AC-12 (listener for an interface that appears later: `bindListeners`); `plan/journal/reply-addressed-where-the-client-cannot-receive.md` row 2026-10-27 (`responseAddr`); PXE provisioning (`docs/guide/ze-install.md`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `service dhcp-server` with two subnets in one shared network, DISCOVER from a MAC reserved in the second, on the listen device | → | `serveRequests` segment resolution and selection | `test/dhcp/dhcpserver-shared-segment-reservation.ci` |
| `server-identifier` leaf in config | → | parse, then option 54 in the OFFER | `test/dhcp/dhcpserver-server-identifier.ci` |
| dhcpcd clients on one bridge carrying two subnets | → | the whole plugin over the wire | interop scenario `dhcpv4-server-shared-segment-dhcpcd` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Device holds 192.168.50.1/24 and 10.99.0.1/24; shared network with both subnets; MAC 02:00:00:00:00:02 reserved 10.99.0.20; DISCOVER from that MAC, no options | OFFER of 10.99.0.20 with option 54 10.99.0.1 and the 10.99.0.0/24 router, mask and DNS |
| AC-2 | Same, MAC 02:00:00:00:00:01 reserved 192.168.50.20 | OFFER of 192.168.50.20, option 54 192.168.50.1 |
| AC-3 | Same, unreserved MAC, primary address 192.168.50.1 | OFFER from the 192.168.50.0/24 pool |
| AC-4 | 192.168.50.0/24 pool exhausted, unreserved MAC | OFFER from the 10.99.0.0/24 pool |
| AC-5 | INIT-REBOOT REQUEST from the 10.99.0.20-reserved MAC asking 192.168.50.77 | DHCPNAK, broadcast (Section 4.3.2) |
| AC-6 | INIT-REBOOT REQUEST asking an address outside every segment subnet | DHCPNAK |
| AC-7 | SELECTING REQUEST naming option 54 10.99.0.1 for a 10.99.0.0/24 offer | ACK; a REQUEST naming 192.168.50.1 (also the server's) on the same segment is also accepted (`RFC2131-4.1-1`) |
| AC-8 | Subnet with no `default-router` | option 54 is the device's address in that subnet, never a pool address or the network address (`RFC2131-4.1-2`) |
| AC-9 | `server-identifier 10.99.0.1` set on a subnet | option 54 carries it; a value outside the subnet is refused at commit with an error naming the leaf, the value, the subnet and the fix |
| AC-10 | Two listen interfaces, each addressed in its own shared network's subnet | a DISCOVER on interface A is never answered from interface B's subnets |
| AC-11 | Listen device holds no address in any configured subnet, `giaddr` zero | no reply; one log line at warn naming the interface, its addresses and the fix (add an address in a configured subnet, or remove the interface from `listen-interface`); `ze doctor` reports the same |
| AC-12 | Relayed DISCOVER, `giaddr` 10.99.0.254 | served from the shared network holding 10.99.0.0/24; reply sent to the relay |
| AC-13 | Single-subnet configuration with `default-router` the interface address | wire-identical replies to today's |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Converts a VyOS LAN with two subnets on `br0` and a printer reserved in the second; the printer boots | dhcpcd DISCOVER → listener on br0 → segment → reservation → OFFER/ACK | interop `dhcpv4-server-shared-segment-dhcpcd` |
| 2 | A laptop with no reservation joins | DISCOVER → segment → primary subnet pool | same scenario, range client |
| 3 | The printer, previously leased from the wrong subnet, reboots | INIT-REBOOT with the old address → NAK → DISCOVER → reservation | same scenario, restart step |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSegmentForDeviceAddresses` | `internal/plugins/dhcpserver/segment_test.go` | segment from device addresses; no match answers no segment | |
| `TestSegmentForGiaddr` | same | relay address selects the shared network | |
| `TestSelectReservationAcrossSubnets` | same | AC-1, AC-2: reservation in a later subnet wins over an earlier pool | |
| `TestSelectPrimaryPoolThenOthers` | same | AC-3, AC-4 | |
| `TestInitRebootReservedElsewhereNaks` | `internal/plugins/dhcpserver/rfc2131_test.go` | AC-5, AC-6 | |
| `TestServerIdentifierIsSegmentAddress` | `internal/plugins/dhcpserver/rfc2131_identity_options_test.go` | AC-8, `RFC2131-4.1-2` tag | |
| `TestSelectingAcceptsAnyOwnAddress` | same | AC-7, `RFC2131-4.1-1` tag | |
| `TestServerIdentifierLeafParse` | `internal/plugins/dhcpserver/config_test.go` | AC-9 accept and refuse, error text names the fix | |
| `TestListenerIgnoresOtherSegments` | `internal/plugins/dhcpserver/register_test.go` | AC-10, AC-11 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `server-identifier` | IPv4 unicast inside the subnet, not the network or broadcast address | subnet broadcast minus one | network address | broadcast address |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dhcpserver-shared-segment-reservation` | `test/dhcp/dhcpserver-shared-segment-reservation.ci` | AC-1..AC-4 through a real listener in a namespace, asserting lease table state (`show` output), not timing | |
| `dhcpserver-server-identifier` | `test/dhcp/dhcpserver-server-identifier.ci` | AC-8, AC-9 | |
| `dhcpserver-no-segment-address` | `test/dhcp/dhcpserver-no-segment-address.ci` | AC-11 warning and doctor finding | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `dhcpv4-server-shared-segment-dhcpcd` | `test/interop/scenarios/dhcpv4-server-shared-segment-dhcpcd/` | dhcpcd (three clients: two with fixed MACs reserved one per subnet, one unreserved) | each reserved client binds its reservation, the unreserved client binds from the primary subnet, a restarted client that held a wrong-subnet lease is NAKed and rebinds its reservation. Red phase: revert the selection step, rebuild the Ze image, the 10.99 client binds a 192.168.50 lease | |

## Files to Modify
- `internal/plugins/dhcpserver/register.go` - `serveRequests` resolves the segment and selects; `bindListeners` records device addresses
- `internal/plugins/dhcpserver/handler.go` - `handle` takes the selection result; `matchesSubnet` deleted; `handleRequest` server-identifier check against the segment's set; INIT-REBOOT reservation check
- `internal/plugins/dhcpserver/config_tx.go` - `buildHandlers` groups handlers by shared network; `subnetServerIP` deleted
- `internal/plugins/dhcpserver/config.go` - `server-identifier` parsing and refusal
- `internal/plugins/dhcpserver/neighbor_linux.go` (or a sibling `addresses_linux.go`) - device IPv4 address read and refresh
- `internal/plugins/dhcpserver/doctor.go` - finding for AC-11
- `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang` - `server-identifier` leaf; container description rewritten
- `rfc/short/rfc2131.md` - `RFC2131-4.1-1`, `RFC2131-4.1-2` gap annotations replaced by tests and discrimination records
- `internal/le/interoplab/bgp/` - check and registration for the new scenario, next to `check_dns_resolver.go`

## Files to Create
- `internal/plugins/dhcpserver/segment.go`, `segment_test.go` - segment resolution and selection
- `test/dhcp/dhcpserver-shared-segment-reservation.ci`, `dhcpserver-server-identifier.ci`, `dhcpserver-no-segment-address.ci`
- `test/interop/scenarios/dhcpv4-server-shared-segment-dhcpcd/` - `ze.conf` and the client definitions

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `server-identifier` in `ze-dhcp-server-conf.yang` |
| YANG validation constraints | Yes | `type zt:ipv4-address` (or the module's IPv4 type); the in-subnet check is custom |
| YANG custom validators | Yes | in-subnet check at verify, in the plugin's `verifyDHCPConfig` |
| CLI commands/flags | N-A | no command |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | Yes | automatic for the typed leaf |
| Functional test for new RPC/API | N-A | no RPC; config `.ci` listed |
| Pipe completeness | N-A | no output |
| Env var registration | N-A | no environment leaf |
| Doctor check for runtime dependencies | Yes | AC-11 finding in `doctor.go`, code in `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | No | existing logging suffices; a "no segment" counter is optional and not added |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or scope changed? | Yes | `features/` entry for the DHCP server: several subnets per segment |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` DHCP section: `server-identifier`, selection order |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` only if it describes subnet choice; check |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` |
| 7 | Wire format changed? | No | option 54 value only |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc2131.md` rows 4.1-1, 4.1-2; `docs/features/rfc-status.md` row |
| 10 | Test infrastructure changed? | Yes | new interop scenario: `docs/architecture/testing/interop.md` scenario list |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | No | plugin-internal |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event, command or inventory changed? | No | none |
| 16 | Changed source files referenced by doc source anchors? | Yes | run `./le spec citation anchors spec plan/home-cpe/spec-home-cpe-2-dhcp-shared-interface-subnets.md` at implementation |
| 17 | Existing docs show examples for this area? | Yes | `docs/guide/configuration.md` l.4079 and `docs/guide/ze-install.md` shared-network examples |

## Implementation Steps

1. **Phase: Wiring** - the segment resolver exists and `serveRequests` calls it; the `.ci` with two subnets and a reservation in the second fails (offers from subnet 1).
   - Tests: `dhcpserver-shared-segment-reservation`
   - Files: `segment.go`, `register.go`
2. **Phase: Device addresses** - netlink read and refresh, AC-10, AC-11, doctor finding.
3. **Phase: Selection** - steps 1 to 4, AC-1..AC-6, R-2.
4. **Phase: Server identifier** - option 54 from the segment, SELECTING check, `server-identifier` leaf, AC-7..AC-9; RFC rows and discrimination records (`./le rfc record`).
5. **Phase: Interop** - scenario `dhcpv4-server-shared-segment-dhcpcd`, red phase recorded.
6. **Phase: Docs** - configuration guide, YANG descriptions, RFC summary.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No path answers "first handler that replies"; `matchesSubnet` gone; reservation lookup spans the segment |
| Correctness | No silent fallback when the device has no segment address (principles: no silently wrong value) |
| Data flow | Device addresses come from netlink, never from the iface plugin through a Go call |
| Rule: no-layering | `subnetServerIP` and `matchesSubnet` deleted, not kept as fallbacks |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Segment selection | `TestSelectReservationAcrossSubnets` and the `.ci` pass |
| RFC rows proven | `./le rfc check` clean for rfc2131 4.1-1 and 4.1-2 |
| Interop | scenario green, red phase recorded in the spec |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `giaddr`, `ciaddr`, Requested IP from untrusted LAN clients select only among configured subnets; a forged `giaddr` cannot reach a shared network the listener's device does not serve unless the packet arrived unicast from a relay |
| Resource exhaustion | the "no segment" log line is rate-limited |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A-1 refuted by existing tests | DESIGN: decide per test whether the test or the config is wrong; never restore "answer anyway" |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The defect is not "reservations are per subnet", it is that a listener has no notion of the segment it serves. Fixing the reservation lookup alone would leave AC-10 broken.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

No open journal row describes this defect (grep of `plan/journal/` for `dhcpserver`, `matchesSubnet`, `subnetServerIP`). The RFC gaps live as `{gap: ...}` annotations in `rfc/short/rfc2131.md`, which this spec owns through its RFC gaps field.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Segment from the receiving device's addresses (and `giaddr`), grouping by `shared-network` | an explicit `interface` leaf per shared network | the addresses already say which segment the device is on; a second declaration could disagree with them (principles: declare once). Matches ISC and Kea, which VyOS configurations assume |
| Reservations across the segment before any pool | per-subnet reservations (today) | RFC 2131 Section 4.3.1 lets the server choose another subnet on a multi-subnet segment; the operator's reservation is the explicit administrative choice |
| No reply when the device holds no segment address | answer from any subnet (today) | answering produces a silently wrong lease; RFC 2131 Section 4.1 asks for a server identifier the server uses on that subnet, which does not exist then |
| `server-identifier` optional per subnet | always derive | derivation covers the common case; the leaf covers a VRRP virtual address or a router addressed several times in one subnet |
| Accept any own segment address in option 54 | single address | `RFC2131-4.1-1` MUST |

## Known Limitations

- Option 82 (relay agent information) and the link-selection sub-option (RFC 3527) are not used for selection; Ze implements neither today.
- DHCPv6 server: not part of Ze; out of scope.

## RFC Documentation (Scope: protocol)

Add `// RFC 2131 Section 4.1: "..."` above the server identifier choice and the SELECTING check, and `// RFC 2131 Section 4.3.1: "..."` above the selection step, quoting the sentences in Required Reading.

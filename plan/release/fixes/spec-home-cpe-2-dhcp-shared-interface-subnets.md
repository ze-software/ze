# Spec: home-cpe-2-dhcp-shared-interface-subnets

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Priority | P1 |
| Release reason | Release fix: an accepted configuration (two subnets on one LAN, a static reservation in the second) hands the reserved client a pool lease from the first subnet, and every listener answers for every subnet of every interface |
| RFC gaps | RFC2131-4.1-1, RFC2131-4.1-2, RFC2131-4.3.2-3, RFC2131-4.3.2-6 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Parent: `plan/release/fixes/spec-home-cpe-0-umbrella.md` (child 2).

RFC gaps: RFC 2131 Section 4.1, `RFC2131-4.1-1` (accept any of the server's
addresses as its identifier) and `RFC2131-4.1-2` (server identifier reachable
from the client), both recorded as gaps in `rfc/short/rfc2131.md`; and RFC 2131
Section 4.3.2, `RFC2131-4.3.2-3` (an INIT-REBOOT request from a client the
server has no record of MUST draw silence) and `RFC2131-4.3.2-6` (check
`ciaddr` for correctness), recorded gaps no spec owned, taken here because the
selection matrix rewrites the very `handleRequest` branches they name.

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
- [ ] `docs/guide/configuration.md` "DHCP Server" (`shared-network`, `subnet`, `static-mapping`) - the operator's view of the tree
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
- [ ] `internal/plugins/dhcpserver/register.go` - `bindListeners` starts `dhcpServe` on every listener with the full handler list; `serveRequests` offers each packet to each handler in configuration order and the first non-nil reply wins
- [ ] `internal/plugins/dhcpserver/config_tx.go` - `buildHandlers` makes one handler per subnet of every shared network, with no interface association; `subnetServerIP` answers `default-router`, else the first range start, else the subnet address
- [ ] `internal/plugins/dhcpserver/handler.go` - `dhcpHandler.handle` calls `dhcpHandler.matchesSubnet`, which answers by `giaddr`, then `ciaddr`, then Requested IP, and answers true when all three are absent; `handleDiscover` calls `allocateForClient`, which checks only its own subnet's `staticByMAC`, then its pool, and never reads the Requested IP; `handleRequest` drops a SELECTING request whose option 54 differs from its single `serverIP`; in INIT-REBOOT it commits any requested address inside its prefix without consulting the lease table or a reservation held for the MAC elsewhere, so a client the server has no record of draws an ACK (`RFC2131-4.3.2-3`); in RENEWING and REBINDING it commits `ciaddr` with no correctness check (`RFC2131-4.3.2-6`)
- [ ] `internal/plugins/dhcpserver/config.go` - `serverConfig`, `sharedNetwork`, `subnetConfig`, `staticMapping`; no interface or server-identifier field
- [ ] `test/parse/dhcp-server-multi-subnet.ci` - parse-only; no test serves two subnets
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

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Multi-handler dispatch | `internal/plugins/dhcpserver/register.go` `serveRequests`; `docs/architecture/provisioning/dhcp-server.md`, "Decisions" (one socket per listen interface, with multi-handler dispatch) | offers each packet to every subnet handler in configuration order; the first non-nil reply wins | replaces: one segment resolver and one selection step choose the handler; the first-answer loop is deleted (no-layering) |
| Per-subnet handler and its hint match | `internal/plugins/dhcpserver/handler.go` `dhcpHandler`, `matchesSubnet`, `allocateForClient` | each handler matches by `giaddr`, `ciaddr`, Requested IP, and claims a hintless packet; reservations are looked up in its own `staticByMAC` only | extends the handler (pool, lease table, reply building stay); replaces `matchesSubnet` with the segment selection, which consults reservations across the segment first |
| Server identifier derivation | `internal/plugins/dhcpserver/config_tx.go` `subnetServerIP`, used by `buildHandlers` | `default-router`, else the first range start, else the subnet address | replaces: option 54 comes from the device's address in the chosen subnet or the `server-identifier` leaf; `subnetServerIP` is deleted |
| Pool and lease sharing across reloads | `internal/plugins/dhcpserver/config_tx.go` `samePlanHandler`, `sameAddressPlan`; `dhcp-server.md`, "Config reload" | a subnet with an unchanged address plan keeps its pool and lease table | extends: handlers are grouped by shared network, and the sharing rule is kept unchanged |
| Listen-interface binding | `internal/plugins/dhcpserver/register.go` `bindListeners`, `listenInterface`, `dhcpResolveDevice`; `internal/component/iface/dispatch.go` `ResolveDevice` | resolves the logical name to its kernel device and binds one `SO_BINDTODEVICE` socket per interface | extends: the listener also records the device's IPv4 addresses, read from netlink for that same resolved device |
| Netlink use in the plugin | `internal/plugins/dhcpserver/neighbor_linux.go` `installNeighbor`, `readNeighbor` | writes and reads neighbor entries on the receiving device | extends: the address read sits beside it, never a second netlink client |
| Reply delivery | `internal/plugins/dhcpserver/register.go` `replyDestination`; `dhcp-server.md`, "Reply delivery follows RFC 2131 Section 4.1" | chooses relay, unicast or broadcast destination for a reply | does not apply: unchanged; the chosen subnet's handler builds the reply it delivers |
| Listen-interface readiness check | `internal/plugins/dhcpserver/doctor.go` `checkDHCPInterfaces` | reports an unmatched or ambiguous listen-interface selector | extends: gains the AC-11 finding (device holds no address in any configured subnet) |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A DHCPv4 packet on the UDP 67 socket bound to the kernel device of a `listen-interface` (`listenInterface`, `SO_BINDTODEVICE`).
- Config: `service dhcp-server` tree, delivered to the plugin as SDK config sections.

### Transformation Path
1. Config: `parseConfig` builds shared networks and subnets; a new optional `server-identifier` leaf per subnet is parsed and checked as an IPv4 unicast address that is not the network or broadcast address of the subnet it names. It is not required to fall inside that subnet: for a relay-only subnet the server holds no address inside it, and an address inside it would be the client's own network, not the server's. Whether the device holds it is a runtime fact, checked at reply time (step 5).
2. Bind: each listener records its kernel device. The device's IPv4 addresses are read from netlink at bind and refreshed on address change (the plugin already learns its device through `dhcpResolveDevice`).
3. Request: the listener resolves the segment. With `giaddr` set: the shared network holding a subnet that contains `giaddr`. With `giaddr` zero: the shared networks holding a subnet that contains an address of the receiving device. No match: no reply, one rate-limited log line naming the interface and the fix.
4. Selection within the segment. The message state is classified first, as RFC 2131 Section 4.3.2 defines it (server identifier present: SELECTING; requested IP without server identifier and `ciaddr` zero: INIT-REBOOT; `ciaddr` set without either: RENEWING when unicast, REBINDING when broadcast). Hints (`ciaddr`, Requested IP) never pick a subnet ahead of a reservation: a reservation for the MAC anywhere on the segment is consulted first in every state, and a hint is then either a candidate (DISCOVER) or a claim checked against the reservation and the lease table (every DHCPREQUEST). One matrix, used by every section below:

| State | Step 1: reservation for the MAC on the segment | Step 2: lease-table binding for the MAC on the segment | Step 3: hint | Step 4: no record | RFC 2131 text that decides |
|-------|-----------------------------------------------|--------------------------------------------------------|--------------|-------------------|----------------------------|
| DISCOVER | OFFER the reserved address; the Requested IP is ignored | OFFER the bound address | Requested IP inside a segment subnet and free in its pool: OFFER it | pool of the subnet holding the device's primary address (relay: the subnet holding `giaddr`), then the other segment subnets in configuration order; none free: silent | 4.3.1 order ("The client's current address ... ELSE ... The address requested in the 'Requested IP Address' option, if that address is valid and not already allocated, ELSE ... A new address allocated from the server's pool"), and "a server MAY, for administrative reasons, assign an address other than the one requested" |
| SELECTING | option 54 not one of the server's own addresses on the segment: silent (another server's offer). Else Requested IP equal to the reservation: ACK; different: NAK | Requested IP equal to the offered or bound address: ACK | Requested IP free in a segment subnet pool: ACK | Requested IP outside every segment subnet: NAK | 4.1 (`RFC2131-4.1-1`) for option 54; 4.3.2 SELECTING |
| INIT-REBOOT | Requested IP equal to the reservation: ACK; different: NAK | equal to the binding: ACK; different: NAK | Requested IP outside every segment subnet: NAK, whatever steps 1 and 2 say (wrong network) | in a segment subnet, no reservation, no binding: SILENT | 4.3.2: "Server SHOULD send a DHCPNAK message to the client if the 'requested IP address' is incorrect, or is on the wrong network." and "If the DHCP server has no record of this client, then it MUST remain silent" (`RFC2131-4.3.2-3`) |
| RENEWING, REBINDING | `ciaddr` equal to the reservation: ACK; different: NAK, so the client returns to INIT and is offered its reservation | equal to the binding: ACK and extend | `ciaddr` outside every segment subnet, or bound or reserved to another MAC: NAK | `ciaddr` in a segment subnet, free: ACK and record the binding (the server lost its table, for example after a restart) | 4.3.2 REBINDING: "The DHCP server SHOULD check 'ciaddr' for correctness before replying to the DHCPREQUEST." (`RFC2131-4.3.2-6`); a reservation is the operator's statement of the correct address (4.3.1, "for administrative reasons") |

   A NAK on a request with `giaddr` zero is broadcast, and with `giaddr` set carries the broadcast bit (4.3.2, existing `buildNak` and `replyDestination` behavior).
5. Reply, option 54 (server identifier), one rule for every state:
   1. The chosen subnet's `server-identifier`, when set AND the receiving device currently holds that address. Set but not held: the request is not answered and one rate-limited warning names the subnet, the address and the fix (add the address to the listen interface, or remove the leaf). Never a silent fallback.
   2. Else, the receiving device's address inside the chosen subnet.
   3. Else (the chosen subnet is relay-only: `giaddr` set and the device holds no address in that subnet), the receiving device's primary IPv4 address. RFC 2131 Section 4.1 asks for an address "that, to the best of the server's knowledge, is reachable from the client"; a relayed client reaches the server through the relay, and the relay reached the server on this device, so the device's primary address is the server's best knowledge. Relay-supplied override (RFC 5107 server identifier override, carried in option 82) is not read: Ze parses no option 82 (Known Limitations).
   4. Else (no `giaddr` and no device address in the chosen subnet): unreachable, because step 3 of the transformation path already refused to serve such a segment.
6. DHCPREQUEST (SELECTING): accepted when option 54 equals any address the server would send under step 5 for any subnet of the segment: every device address in a segment subnet, the device's primary address for a relayed segment, and every held `server-identifier` of the segment.

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
| A-1 | For requests with `giaddr` zero, the listen interface holds an IPv4 address inside at least one configured subnet in every working deployment (relayed requests need none, AC-19) | RFC 2131 Section 4.1 SHOULD (server uses its address on the subnet); ISC and Kea refuse to serve a segment they hold no address on | configs that relied on serving a subnet the router is not addressed in stop answering | grep of `test/dhcp/*.ci` and the interop scenarios for listen devices with no address in the subnet, before Phase 2 | validated 2026-11-07: no functional test sends a DHCP packet (every `test/` file naming `dhcp-server` only validates config or binds a listener: `test/dhcp/dhcp-range-inside-subnet.ci`, `test/install/dhcp-*.ci`, `test/reload/dhcpserver-*.ci`), and both serving interop scenarios address their listen device in the served subnet (`dns-resolver-dhcp-lan`: lab `eth0` in 172.30.0.0/24; `dhcpv4-server-shared-segment-dhcpcd`: 172.30.0.2/24 and 10.99.0.1/24). R-1 does not occur |
| A-2 | `shared-network` groups subnets of one segment in operator intent | ISC/Kea semantics; VyOS `shared-network-name` | a config with two segments in one shared network would mix reservations across segments | read every `shared-network` example in `docs/` and `test/` | confirmed 2026-11-08: every config with several subnets in one shared network (`test/interop-topology/cpe/ze-cpe.conf` LAN 192.168.50.0/24 and 10.99.0.0/24 on one bridge, `test/reload/dhcpserver-range-reload.ci`, `test/parse/dhcp-server-multi-subnet.ci`, the five `test/dhcp/dhcpserver-*.ci`) puts them on one segment |
| A-3 | dhcpcd sends DISCOVER with no Requested IP on first boot and an INIT-REBOOT REQUEST with its old address on restart | dhcpcd behaviour in the existing scenario | the interop test would not exercise step 3 or step 2 | the interop scenario captures both | confirmed 2026-11-08: the clean `dhcpv4-server-shared-segment-dhcpcd` run passes assertion 5 of `checkDHCPSharedSegment` (`internal/le/interoplab/bgp/check_dhcp_shared_segment.go`), which requires dhcpcd's `NAK:` log for the printer's rebooted wrong-subnet lease before it binds its reservation |
| A-4 | A reload that adds an address to the listen device is seen without rebinding | netlink address events | a new subnet stays unserved until reload | unit test with a fake address source plus the `.ci` | confirmed 2026-11-08: the device's addresses are read from netlink for each request (`readDeviceAddrs` through `dhcpDeviceAddrs`, `register.go`), and `test/dhcp/dhcpserver-no-segment-address.ci` serves the client once an address is added, with no reload (green in the netns guest) |
| A-5 | Kea, which VyOS configurations run on, NAKs a renewal whose address differs from the client's host reservation, as AC-17 does | Kea ARM, "Conflicts in DHCPv4 Reservations" (documentation, not source); no Kea or dnsmasq checkout is on this host | AC-17 still stands on RFC 2131 Section 4.3.2 (`ciaddr` correctness) and the reservation as the administrative choice; only the claim "same as Kea" falls | read Kea `src/lib/dhcpsrv/alloc_engine.cc` (`AllocEngine::renewLease4` and the reservation checks it calls) at the release the VyOS image ships, before Phase 3, and quote the deciding function here | validated 2026-11-07 against Kea-2.4.1 and Kea-2.6.1, whose deciding block is identical (which of the two the VyOS image ships was not checked). `renewLease4` decides nothing: the check is in `AllocEngine::requestLease4`, which a RENEWING request reaches with `ciaddr` as the hint (`Dhcpv4Srv::assignLease`, `src/bin/dhcp4/dhcp4_srv.cc`: "} else if (!query->getCiaddr().isV4Zero()) { hint = query->getCiaddr();"). `requestLease4`: "If the reserved address is not in use, i.e. the lease doesn't exist or is expired, and the client is requesting a different address, return NULL. The client should go back to the DHCPDISCOVER and the reserved address will be offered." A NULL lease makes `assignLease` answer `resp->setType(DHCPNAK)`. Kea NAKs as AC-17 does, with one difference: when the reserved address is leased to another client, Kea renews the old address, where Ze still NAKs |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Existing functional tests serve from a device with no address in the subnet (A-1) | `.ci` red after Phase 2 | give the test device an address in the subnet; never fall back to "answer anyway", which is the defect |
| R-2 | A client holding a pool lease in subnet 1 keeps it after its reservation in subnet 2 is added | lease table shows both | the matrix's RENEWING and REBINDING row NAKs a `ciaddr` that differs from the MAC's reservation on the segment, so the client returns to INIT and its DISCOVER is offered the reservation (AC-17) |
| R-4 | INIT-REBOOT silence (AC-16) after a Ze restart that lost the lease table delays a rebooting client until its INIT-REBOOT times out and it sends DISCOVER | client takes seconds longer to get an address after a Ze restart | the RFC MUST decides; a renewing client is unaffected (AC-18 records the binding) |
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
| AC-5 | INIT-REBOOT REQUEST from the 10.99.0.20-reserved MAC asking 192.168.50.77 | DHCPNAK, broadcast (Section 4.3.2); same MAC asking 10.99.0.20: DHCPACK |
| AC-6 | INIT-REBOOT REQUEST asking an address outside every segment subnet | DHCPNAK |
| AC-7 | SELECTING REQUEST naming option 54 10.99.0.1 for a 10.99.0.0/24 offer | ACK; a REQUEST naming 192.168.50.1 (also the server's) on the same segment is also accepted (`RFC2131-4.1-1`); a REQUEST naming 192.0.2.9 (not the server's) gets no reply |
| AC-8 | Subnet with no `default-router` | option 54 is the device's address in that subnet, never a pool address or the network address (`RFC2131-4.1-2`) |
| AC-9 | `server-identifier 10.99.0.1` set on a subnet, device holds it | option 54 carries it. A value that is not unicast, or is the network or broadcast address of the subnet, is refused at commit with an error naming the leaf, the value, the subnet and the fix. Set but not held by the device at reply time: no reply, one warning naming the subnet, the address and the fix |
| AC-14 | DISCOVER from the 10.99.0.20-reserved MAC carrying Requested IP 192.168.50.77 | OFFER of 10.99.0.20: the hint does not terminate selection |
| AC-15 | DISCOVER from an unreserved MAC with Requested IP 10.99.0.50, free in the 10.99.0.0/24 pool | OFFER of 10.99.0.50 |
| AC-16 | INIT-REBOOT REQUEST asking 192.168.50.77 inside a segment subnet from a MAC with no reservation and no binding | no reply (`RFC2131-4.3.2-3`) |
| AC-17 | RENEWING REQUEST (unicast, `ciaddr` 192.168.50.30) from a MAC bound to it, after a reservation 10.99.0.20 was added for that MAC | DHCPNAK; the client's next DISCOVER is offered 10.99.0.20 (R-2) |
| AC-18 | RENEWING and REBINDING REQUEST with `ciaddr` bound to the same MAC and no reservation | DHCPACK extending the binding; `ciaddr` outside every segment subnet, or bound to another MAC: DHCPNAK (`RFC2131-4.3.2-6`) |
| AC-19 | Relayed DISCOVER, `giaddr` 172.16.5.1, shared network holding 172.16.5.0/24; the listen device holds 192.168.50.1/24 only, no address in 172.16.5.0/24, no `server-identifier` | OFFER from 172.16.5.0/24 sent to the relay, option 54 192.168.50.1; the client's SELECTING REQUEST naming 192.168.50.1 is ACKed |
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
| `TestDiscoverReservationBeatsRequestedIP` | `internal/plugins/dhcpserver/segment_test.go` | AC-14 (DISCOVER, step 1 before step 3) | |
| `TestDiscoverRequestedIPIsCandidate` | `internal/plugins/dhcpserver/segment_test.go` | AC-15 | |
| `TestInitRebootReservedElsewhereNaks` | `internal/plugins/dhcpserver/rfc2131_test.go` | AC-5, AC-6 (INIT-REBOOT, separately from DISCOVER) | |
| `TestInitRebootNoRecordSilent` | `internal/plugins/dhcpserver/rfc2131_test.go` | AC-16, `RFC2131-4.3.2-3` tag | |
| `TestRenewingReservedElsewhereNaks` | `internal/plugins/dhcpserver/rfc2131_test.go` | AC-17 (RENEWING) | |
| `TestRebindingChecksCiaddr` | `internal/plugins/dhcpserver/rfc2131_test.go` | AC-18 (REBINDING and RENEWING), `RFC2131-4.3.2-6` tag | |
| `TestServerIdentifierRelayOnlySubnet` | `internal/plugins/dhcpserver/rfc2131_identity_options_test.go` | AC-19, AC-12 with no device address in the relayed subnet | |
| `TestServerIdentifierLeafNotHeld` | `internal/plugins/dhcpserver/rfc2131_identity_options_test.go` | AC-9 runtime half: leaf set, device lacks it, no reply, warning | |
| `TestServerIdentifierIsSegmentAddress` | `internal/plugins/dhcpserver/rfc2131_identity_options_test.go` | AC-8, `RFC2131-4.1-2` tag | |
| `TestSelectingAcceptsAnyOwnAddress` | same | AC-7, `RFC2131-4.1-1` tag | |
| `TestServerIdentifierLeafParse` | `internal/plugins/dhcpserver/config_test.go` | AC-9 accept and refuse, error text names the fix | |
| `TestListenerIgnoresOtherSegments` | `internal/plugins/dhcpserver/register_test.go` | AC-10, AC-11 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `server-identifier` | IPv4 unicast, not the network or broadcast address of its subnet | subnet broadcast minus one | network address | broadcast address |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dhcpserver-shared-segment-reservation` | `test/dhcp/dhcpserver-shared-segment-reservation.ci` | AC-1..AC-4 through a real listener, asserting the reply on the wire and the server's own record of each offer (the `dhcpserver: lease` log line, `logExchange`): the server has no lease-table `show` command, so there is no `show` output to assert | promoted in 6d194604f9; green in the netns guest on Ze's kernel 2026-11-08 11:12, red under its break 11:15-11:17 |
| `dhcpserver-server-identifier` | `test/dhcp/dhcpserver-server-identifier.ci` | AC-8, AC-9: the commit refusals through `ze config validate`, the held leaf, and the withheld reply with its warning once the probe removes the address | promoted in 6d194604f9; green in the netns guest on Ze's kernel 2026-11-08 11:12, red under its break 11:15-11:17 |
| `dhcpserver-request-states` | `test/dhcp/dhcpserver-request-states.ci` | AC-5, AC-6, AC-7, AC-14, AC-15, AC-16, AC-17, AC-18: one exchange per state (DISCOVER, SELECTING, INIT-REBOOT, RENEWING, REBINDING), each asserted separately on the reply type and fields; AC-17 adds the reservation by a SIGHUP reload while the binding is held | promoted in 6d194604f9; green in the netns guest on Ze's kernel 2026-11-08 11:12, red under its break 11:15-11:17 |
| `dhcpserver-relay-only-subnet` | `test/dhcp/dhcpserver-relay-only-subnet.ci` | AC-19, AC-12: relayed DISCOVER and SELECTING REQUEST for a subnet the listen device holds no address in, each reply asserted at the relay address | promoted in 6d194604f9; green in the netns guest on Ze's kernel 2026-11-08 11:12, red under its break 11:15-11:17 |
| `dhcpserver-no-segment-address` | `test/dhcp/dhcpserver-no-segment-address.ci` | AC-11 warning and doctor finding, then the same client served once an address in the subnet is added, with no reload | promoted in 6d194604f9; green in the netns guest on Ze's kernel 2026-11-08 11:12, red under its break 11:15-11:17 |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `dhcpv4-server-shared-segment-dhcpcd` | `test/interop/scenarios/dhcpv4-server-shared-segment-dhcpcd/` | dhcpcd (three clients: two with fixed MACs reserved one per subnet, one unreserved) | each reserved client binds its reservation, the unreserved client binds from the primary subnet, a restarted client that held a wrong-subnet lease is NAKed and rebinds its reservation. Red phase: revert the selection step, rebuild the Ze image, the 10.99 client binds a 192.168.50 lease | green 2026-11-08 (clean first run and clean rerun at 12:18); red under `c2-reservation-skipped` at 12:05 |

## Files to Modify
- `internal/plugins/dhcpserver/register.go` - `serveRequests` resolves the segment and selects; `bindListeners` records device addresses
- `internal/plugins/dhcpserver/handler.go` - `handle` takes the selection result; `matchesSubnet` deleted; `handleRequest` server-identifier check against the segment's set; INIT-REBOOT reservation, binding and no-record silence; RENEWING and REBINDING `ciaddr` check (the state table in Data Flow step 4)
- `internal/plugins/dhcpserver/config_tx.go` - `buildHandlers` groups handlers by shared network; `subnetServerIP` deleted
- `internal/plugins/dhcpserver/config.go` - `server-identifier` parsing and refusal
- `internal/plugins/dhcpserver/neighbor_linux.go` - the netlink handle the new address read shares (no second client)
- `internal/plugins/dhcpserver/doctor.go` - finding for AC-11
- `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang` - `server-identifier` leaf; container description rewritten
- `rfc/short/rfc2131.md` - `RFC2131-4.1-1`, `RFC2131-4.1-2`, `RFC2131-4.3.2-3`, `RFC2131-4.3.2-6` gap annotations replaced by tests and discrimination records
- `internal/le/interoplab/bgp/` - check and registration for the new scenario, next to `check_dns_resolver.go`
- `docs/architecture/provisioning/dhcp-server.md` - declared by every changed source file's `// Design:` header. "Decisions", "One socket per listen interface, with multi-handler dispatch" says the serving goroutine tries each subnet handler in order and that a bare DISCOVER "falls through to the first handler that has pool space"; the change makes that false, so the paragraph is rewritten to the segment resolution and selection order of this spec, and the page gains the `server-identifier` choice. "Config reload" stays true (sharing by address plan is kept)
- `docs/architecture/config/transaction-protocol.md` - declared by `config_tx.go`'s `// Design:` header; nothing it says becomes wrong: it names dhcpserver only as a plugin whose running listeners may fail to rebind, which this change keeps

## Files to Create
- `internal/plugins/dhcpserver/segment.go`, `segment_test.go` - segment resolution and selection
- `test/dhcp/dhcpserver-shared-segment-reservation.ci`, `dhcpserver-server-identifier.ci`, `dhcpserver-request-states.ci`, `dhcpserver-relay-only-subnet.ci`, `dhcpserver-no-segment-address.ci` (drafts in `test/draft/dhcp/` until they pass in the QEMU guest)
- `internal/test/dhcpprobe/` - the DHCPv4 probe client the five tests drive: it writes a BOOTREQUEST carrying exactly the fields a step names on one end of a veth pair (AF_PACKET) and reads the server's reply on the wire. No client usable from a `.ci` existed
- `internal/test/fixture/dhcp_fixture_probe_linux.go`, `register_dhcp_probe_linux.go` - the `le test fixture dhcp/link` and `dhcp/probe` drivers over it
- `test/interop/scenarios/dhcpv4-server-shared-segment-dhcpcd/` - `ze.conf` and the client definitions

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `server-identifier` in `ze-dhcp-server-conf.yang` |
| YANG validation constraints | Yes | `type string`, as the module's other address leaves (`default-router`) are; the address checks are custom |
| YANG custom validators | Yes | in the plugin's `verifyDHCPConfig`: IPv4 unicast, and not the network or broadcast address of the subnet that carries the leaf (Data Flow step 1; no in-subnet requirement, so a relay-only subnet can name an address of the server's) |
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
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, section "DHCP Server": `server-identifier`, selection order |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` only if it describes subnet choice; check |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` |
| 7 | Wire format changed? | No | option 54 value only |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc2131.md` rows `RFC2131-4.1-1`, `RFC2131-4.1-2`, `RFC2131-4.3.2-3`, `RFC2131-4.3.2-6`; `docs/features/rfc-status.md` row |
| 10 | Test infrastructure changed? | Yes | new interop scenario: `docs/architecture/testing/interop.md` scenario list |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | No | plugin-internal |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event, command or inventory changed? | No | none |
| 16 | Changed source files referenced by doc source anchors? | Yes | run `./le spec citation anchors spec plan/release/fixes/spec-home-cpe-2-dhcp-shared-interface-subnets.md` at implementation |
| 17 | Existing docs show examples for this area? | Yes | `docs/guide/configuration.md`, section "DHCP Server", and `docs/guide/ze-install.md` shared-network examples |

## Implementation Steps

1. **Phase: Wiring** - the segment resolver exists and `serveRequests` calls it; the `.ci` with two subnets and a reservation in the second fails (offers from subnet 1).
   - Tests: `dhcpserver-shared-segment-reservation`
   - Files: `segment.go`, `register.go`
2. **Phase: Device addresses** - netlink read and refresh, AC-10, AC-11, doctor finding.
3. **Phase: Selection** - the state table in Data Flow step 4, one row at a time: DISCOVER (AC-1..AC-4, AC-14, AC-15), INIT-REBOOT (AC-5, AC-6, AC-16), RENEWING and REBINDING (AC-17, AC-18, R-2), each state proven by its own test; AC-13 stays wire-identical. RFC rows `RFC2131-4.3.2-3` (AC-16, `TestInitRebootNoRecordSilent`) and `RFC2131-4.3.2-6` (AC-17, AC-18, `TestRebindingChecksCiaddr`): tag the tests, record each break with `./le rfc record`, replace the `{gap: ...}` annotations in `rfc/short/rfc2131.md`.
   - Tests: the selection and state rows of the Unit Tests table, `dhcpserver-request-states`
4. **Phase: Server identifier** - Data Flow steps 5 and 6: option 54 from the device's address in the chosen subnet, the relay-only subnet case (step 5.3: `giaddr` set and no device address in the relayed subnet, option 54 the device's primary address, AC-19, AC-12), the `server-identifier` leaf with its runtime "held" check (AC-9), and the SELECTING check against every own address of the segment (AC-7, AC-8). RFC rows `RFC2131-4.1-1` (AC-7) and `RFC2131-4.1-2` (AC-8, AC-19): tagged tests, discrimination records (`./le rfc record`), gap annotations replaced.
   - Tests: `TestServerIdentifierRelayOnlySubnet`, `TestServerIdentifierLeafNotHeld`, `TestServerIdentifierIsSegmentAddress`, `TestSelectingAcceptsAnyOwnAddress`, `TestServerIdentifierLeafParse`, `dhcpserver-server-identifier`, `dhcpserver-relay-only-subnet`
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
| RFC rows proven | `./le rfc check` clean, with `RFC2131-4.1-1`, `RFC2131-4.1-2`, `RFC2131-4.3.2-3` and `RFC2131-4.3.2-6` each tagged on its test, each carrying a discrimination record under `rfc/discrimination/rfc2131/` (one `<id>.json` per row), and none left with a `{gap: ...}` annotation in `rfc/short/rfc2131.md` |
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
- RFC 5107 (the server identifier override sub-option a relay carries in option 82) is not implemented: the plugin parses no option 82 (no option 82 code under `internal/plugins/dhcpserver/`). For a relay-only subnet, option 54 is therefore always the device's primary address (Data Flow step 5.3, AC-19), even where a relay would ask for an override.

## RFC Documentation (Scope: protocol)

Add `// RFC 2131 Section 4.1: "..."` above the server identifier choice and the SELECTING check, and `// RFC 2131 Section 4.3.1: "..."` above the selection step, quoting the sentences in Required Reading.

## Checklist

### Pre-Spec Verification
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Existing Mechanisms table names every mechanism this spec extends or replaces
- [ ] An `rfc/short/` summary exists for every RFC referenced (`rfc/short/rfc2131.md`)
- [ ] No code snippets

### Goal Gates (MUST pass)
- [ ] AC-1..AC-19 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted tests pass: `./le job run label home-cpe-2 command go test -race ./internal/plugins/dhcpserver/`, `./le test dhcp`, and `INTEROP_SCENARIO=dhcpv4-server-shared-segment-dhcpcd ./le test integration interop`; then `./le verify changed` over the closing commits
- [ ] `./le rfc check` clean for `RFC2131-4.1-1` and `RFC2131-4.1-2`, each with its discrimination record
- [ ] `./le rfc check` clean for `RFC2131-4.3.2-3` (`TestInitRebootNoRecordSilent`) and `RFC2131-4.3.2-6` (`TestRebindingChecksCiaddr`), each with its discrimination record
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for `server-identifier`
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests: `dhcpv4-server-shared-segment-dhcpcd`, red phase recorded

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean

---

## Implementation Summary

### What Was Implemented
- `internal/plugins/dhcpserver/segment.go` (`resolveSegment`, `segment.serve`, `discover`, `selecting`, `initReboot`, `renewing`, `serverID`, `serverIDOwner`): a request resolves its segment from `giaddr` or the receiving device's addresses, then follows the Data Flow state matrix, reservation first (10e9995d31).
- Option 54 from the segment's own address, the relay-only case and the `server-identifier` leaf with its held check; `subnetServerIP` and the per-handler `serverIP` deleted (47f14d5ede).
- `requestServer.resolve` (`register.go`) reads the device's addresses through `dhcpDeviceAddrs` (`readDeviceAddrs`, `neighbor_linux.go`) for each request, and warns at most once a minute (`warnLimited`).
- `checkDHCPSegmentAddress` (`doctor.go`), code `doctor-dhcp-segment-address` in `internal/core/diagnostic/codes.go`.
- DHCPv4 probe client `internal/test/dhcpprobe` and fixtures `dhcp/link`, `dhcp/probe` (5f40b09b40, 4c01e35c08); five `.ci` promoted to `test/dhcp/` (6d194604f9).
- Interop scenario `dhcpv4-server-shared-segment-dhcpcd` (`internal/le/interoplab/bgp/check_dhcp_shared_segment.go`).
- RFC 2131 rows 4.1-1, 4.1-2, 4.3.2-3, 4.3.2-6 tagged, recorded under `rfc/discrimination/rfc2131/`, audit verdicts re-stamped (0c0391e2bb).

### Bugs Found/Fixed
- The defect this spec names (first subnet answers for every request): covered by `TestSelectReservationAcrossSubnets` and `test/dhcp/dhcpserver-shared-segment-reservation.ci`.

### Documentation Updates
- `docs/guide/configuration.md`, "Several subnets on one interface", and the `server-identifier` row of the subnet table.
- `docs/architecture/provisioning/dhcp-server.md`, "Decisions": segment resolution and selection replace the first-handler dispatch.
- `docs/features/interfaces.md` rows for several subnets per interface and option 54; `docs/guide/migrating-from-vyos.md` anchor to `segment.serverID`.
- `docs/architecture/testing/interop.md`: dhcpcd peer row and the scenario's registration row.
- YANG `server-identifier` leaf and its description in `ze-dhcp-server-conf.yang`.
- Closure added `features/dhcp-server-shared-segment.md` (Documentation checklist row 1 was not done by implementation); `./le feature report` lists it at `experimental`, and `./le feature check` names no refusal for it.

### Deviations from Plan
- Data Flow step 2 planned a read at bind refreshed on address events. The code reads the device's addresses from netlink for each request (`dhcpDeviceAddrs`), so there is no cached copy to refresh and R-3 cannot occur. A-4 holds through it.
- The interop scenario uses 172.30.0.0/24 as its household subnet (the lab's network) rather than 192.168.50.0/24.
- `test/parse/dhcp-server-multi-subnet.ci` was cited at `test/dhcp/` in Current Behavior; corrected at closure.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Documentation checklist row 1 (a `features/` entry) was left undone by implementation | no `features/` declaration named several subnets per segment | closure step 4 grep of `features/` | closure wrote `features/dhcp-server-shared-segment.md` |
| approach | the implementation commits carry `Spec-work: home-cpe-1-logical-interface-name` | they are this spec's work (subject suffix `(child-2)`) | `git log --grep` found no commit tagged with this spec | recorded here; history is not rewritten |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Choose the subnet the client belongs to (relay, client address, requested address) | Done | `segment.go` `resolveSegment`, `subnetOf`, `request` | |
| Reservation in any segment subnet before any pool | Done | `segment.go` `discover`, `segment.reservation` | |
| Allocate from the subnet the interface is addressed in | Done | `resolveSegment` home subnet, `discover` pool loop | |
| Server identifier the client can reach | Done | `segment.serverID` | |
| Optional `server-identifier` leaf | Done | YANG leaf; `config.go` parse; `serverID` held check | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1, AC-2 | Done | `TestSelectReservationAcrossSubnets`; `dhcpserver-shared-segment-reservation.ci` | |
| AC-3, AC-4 | Done | `TestSelectPrimaryPoolThenOthers`; same `.ci` | |
| AC-5, AC-6 | Done | `TestInitRebootReservedElsewhereNaks`; `dhcpserver-request-states.ci` | |
| AC-7 | Done | `TestSelectingAcceptsAnyOwnAddress` (`RFC2131-4.1-1`) | |
| AC-8 | Done | `TestServerIdentifierIsSegmentAddress` (`RFC2131-4.1-2`); `dhcpserver-server-identifier.ci` | |
| AC-9 | Done | `TestServerIdentifierLeafParse`, `TestServerIdentifierLeafNotHeld`; `dhcpserver-server-identifier.ci` | |
| AC-10, AC-11 | Done | `TestListenerIgnoresOtherSegments`; `dhcpserver-no-segment-address.ci`; `checkDHCPSegmentAddress` | |
| AC-12, AC-19 | Done | `TestServerIdentifierRelayOnlySubnet`; `dhcpserver-relay-only-subnet.ci` | |
| AC-13 | Done | the existing one-subnet RFC 2131 tests pass with unchanged inputs and assertions (judged, `RFC-approved:` lines in 47f14d5ede) | |
| AC-14 | Done | `TestDiscoverReservationBeatsRequestedIP` | |
| AC-15 | Done | `TestDiscoverRequestedIPIsCandidate` | |
| AC-16 | Done | `TestInitRebootNoRecordSilent` (`RFC2131-4.3.2-3`) | |
| AC-17 | Done | `TestRenewingReservedElsewhereNaks`; `dhcpserver-request-states.ci` (reservation added by reload) | |
| AC-18 | Done | `TestRebindingChecksCiaddr` (`RFC2131-4.3.2-6`) | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| 16 unit tests of the Unit Tests table | Done | `segment_test.go`, `rfc2131_test.go`, `rfc2131_identity_options_test.go`, `config_test.go`, `register_test.go` | each found by `grep -n 'func Test'` at closure |
| 5 functional tests | Done | `test/dhcp/dhcpserver-*.ci` | |
| interop `dhcpv4-server-shared-segment-dhcpcd` | Done | `test/interop/scenarios/dhcpv4-server-shared-segment-dhcpcd/`, `check_dhcp_shared_segment.go` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `register.go`, `handler.go`, `config_tx.go`, `config.go`, `neighbor_linux.go`, `doctor.go`, YANG | Done | |
| `segment.go`, `segment_test.go` | Done | |
| `internal/test/dhcpprobe/`, fixture drivers | Done | |
| `rfc/short/rfc2131.md` | Done | no `{gap` on the four rows |
| `docs/architecture/provisioning/dhcp-server.md` | Done | |

### Audit Summary
- **Total items:** 5 requirements, 19 ACs, 22 tests, 11 file groups
- **Done:** all
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (Deviations: per-request address read, interop subnet)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A client reserved in the second subnet is offered its reservation | interop + functional | `dhcpv4-server-shared-segment-dhcpcd` green; red under `c2-reservation-skipped` (dhcpcd offered pool 172.30.0.100, then NAKed, never bound); `dhcpserver-shared-segment-reservation.ci` red under the same break: "yiaddr 192.168.50.100, want 10.99.0.20" |
| A listener serves only its own segment | functional | `dhcpserver-no-segment-address.ci` green, red under `c2-no-segment-silent` |
| Option 54 is reachable, any own address accepted | functional | `dhcpserver-server-identifier.ci` green, red under `c2-server-id-unchecked`; `dhcpserver-relay-only-subnet.ci` green, red under `c2-relay-only-accepted` |
| Request states follow RFC 2131 4.3.2 | functional | `dhcpserver-request-states.ci` green, red under `c2-norecord-acked` |

Evidence logs: `tmp/session/2026-11-07-104251/scratch/rwinv/c2-12-netns.log`, `c2-12-interop.log`, overlays `rwinv/c2-*`.

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | every AC is done | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/home-cpe-2-dhcp-shared-interface-subnets-7e292f27-ced0-427f-a7b6-3a931db211f9.md` |
| `./le spec review check` | clean (`review_gate: OK`, hashes match), re-recorded after this edit |
| Rounds | 1 |
| Reviewer lenses used | logic of the state matrix against RFC 2131 4.3.1 and 4.3.2, security (forged giaddr, flood), style, docs against source |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | NOTE | Security checklist row 1 asks that a forged `giaddr` reach no other shared network unless the packet arrived unicast from a relay. `resolveSegment` selects by `giaddr` whatever the arrival, and the listener reads no destination address (no `IP_PKTINFO`). The agreed Data Flow step 3 selects by `giaddr` alone, the behaviour before this spec was the same, the reply goes to the relay at `giaddr` and never to the forger, and Kea accepts every relayed message (`Dhcpv4Srv::acceptDirectRequest`, Kea 2.6.1: "// Accept all relayed messages."). A broadcast-arrival refusal is a product choice no RFC answers: handed to the main thread | `segment.go` `resolveSegment` | not changed |
| 2 | NOTE | Each request dumps the device's addresses over netlink; a flood costs one dump per packet, bounded by the one request task per listener | `register.go` `requestServer.resolve` | not changed |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/plugins/dhcpserver/segment.go`, `segment_test.go` | yes | `gopls symbols segment.go` lists `resolveSegment` at 54:6 |
| `test/dhcp/dhcpserver-{shared-segment-reservation,server-identifier,request-states,relay-only-subnet,no-segment-address}.ci` | yes | `ls test/dhcp/` at closure |
| `internal/test/dhcpprobe/{frame.go,frame_test.go,probe_linux.go}`, `internal/test/fixture/{dhcp_fixture_probe_linux.go,register_dhcp_probe_linux.go}` | yes | `find`/`ls` at closure |
| `test/interop/scenarios/dhcpv4-server-shared-segment-dhcpcd/ze.conf` | yes | `ls` at closure |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| all | package tests pass | job `p23-3`: `go test -race -tags "<ze_core zetest and every feature gate>" -count=1 ./internal/plugins/dhcpserver/` ok 1.442s at 2026-11-08 17:06, after the last edit to the package (handler.go 14:34) |
| AC-7, AC-8, AC-16, AC-18 | RFC rows tagged and recorded | `grep 'RFC requirement: RFC2131-(4.1-[12]|4.3.2-[36])'` finds positive and negative tags; `./le rfc check` names none of the four rows' records or audits |
| AC-1..AC-19 | functional | netns guest clean run: the five `.ci` pass; each red under its break (`c2-12-netns.log`) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| two subnets, reservation in the second | `test/dhcp/dhcpserver-shared-segment-reservation.ci` | the probe step asserts `yiaddr=10.99.0.20,sid=10.99.0.1` through the daemon's listener; red under `c2-reservation-skipped` |
| `server-identifier` leaf | `test/dhcp/dhcpserver-server-identifier.ci` | red under `c2-server-id-unchecked` |
| dhcpcd on one bridge, two subnets | interop `dhcpv4-server-shared-segment-dhcpcd` | green clean, red broken, green rerun (`c2-12-interop.log`) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Assumptions table, 2026-11-07 |
| A-2 | confirmed | Assumptions table, 2026-11-08 |
| A-3 | confirmed | interop assertion 5 (dhcpcd `NAK:` then reservation) |
| A-4 | confirmed | per-request read; `dhcpserver-no-segment-address.ci` |
| A-5 | confirmed | Kea 2.4.1 and 2.6.1 `AllocEngine::requestLease4` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| configuration guide selection order | `segment.discover` order: reservation, binding, requested, pools | yes |
| `dhcp-server.md` per-request netlink read | `dhcpDeviceAddrs` comment and `requestServer.resolve` | yes |
| `features/dhcp-server-shared-segment.md` | `./le feature report` row `dhcp-server-shared-segment experimental` | yes |
| doc anchors for changed files | `./le spec citation anchors spec <this spec>`: `plugin-system.md`, `pxe-staging.md`, `interfaces.md`, `migrating-from-vyos.md`; the last two carry this spec's claims, re-read at closure | yes |
| No: CLI, API, SDK, wire format, comparison | no command, RPC or SDK change; grep of `docs/comparison.md` for dhcp shared-network finds no claim to change | yes |

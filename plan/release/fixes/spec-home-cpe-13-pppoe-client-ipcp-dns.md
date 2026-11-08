# Spec: home-cpe-13-pppoe-client-ipcp-dns

| Field | Value |
|-------|-------|
| Status | blocked |
| Scope | protocol |
| Priority | P3 |
| Release reason | A home CPE learns its resolvers from the access network; the PPPoE client never asks for them, so a router with no static `name-server` cannot resolve names once the WAN is up. The `access-up` scenario of the umbrella asserts DNS learned from IPCP. No accepted configuration misbehaves today (the options are never sent), so this is a feature (P3) even though the umbrella lists it as a release fix; the umbrella orders it last. |
| RFC gaps | rfc1877, RFC1661-5.4-5 |
| Depends | Implementation: `spec-home-cpe-12-pppoe-client-lcp-renegotiation.md` (the re-enterable `negotiateIPCP` and the change record this spec adds DNS to). Closure: the DNS clause of `access-up` (child 19 AC-6), which this spec owns inside child 19's scenario, needs `spec-home-cpe-19-access-chain-scenario.md` implemented |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Parent: plan/release/fixes/spec-home-cpe-0-umbrella.md

RFC gaps, in full: RFC 1877 Section 1.1 and Section 1.3 (client side: request Primary and Secondary DNS, take the value from a Configure-Nak); RFC 1661 Section 5.4 (drop Rejected options from the next request) on the client's IPCP. `rfc/short/rfc1877.md` lists no requirement ids, so the cell names its stem.

Recovery after compaction: `.claude/rules/post-compaction.md`.

All values in this spec are invented (RFC 5737 prefixes, `example.net`).

## Task

The Ze PPPoE client's IPCP Configure-Request carries the IP-Address option
only. RFC 1877 lets the client ask for Primary-DNS-Address (129) and
Secondary-DNS-Address (131) by sending 0.0.0.0 and taking the server's value
from the Configure-Nak. Add both options, handle Nak, Reject and Ack for them,
and give the learned servers to Ze's resolv.conf writer, ranked below static
`system name-server` the way DHCP-learned servers already are.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/l2tp/cpe-1-pppoe-client.md` - client IPCP negotiation, unit reader
  → Constraint: IPCP is negotiated before PPPIOCCONNECT, on the channel; Ack and Reject must match the last request byte for byte (`ppp.ValidateLCPReply`)
- [ ] `docs/features/interfaces.md` row "DNS from DHCP to `/tmp/resolv.conf`, tracked per lease" (source anchor `internal/component/config/system/resolv.go -- Resolvers`)
  → Decision: one writer of resolv.conf, `system.Resolvers`, keyed by source; a source withdraws only its own servers
- [ ] `docs/architecture/dns/lan-resolver.md` "Forwarding"
  → Constraint: the LAN resolver forwards only to the operator's `forward server` entries; learned servers do not reach it (Known Limitations)
- [ ] `docs/features/dns-resolver.md` "Router resolver selection" (declared by `internal/component/config/system/resolv.go`; written by child 23 after this spec's code landed)
  → Constraint: the IPCP lease is one `received` record among DHCPv4, DHCPv6 and RA RDNSS, valid until withdrawn; the host's file stands in only while no received record lists a server

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc1877.md` (exists; states no MUST, one SHOULD in Section 1; signed off under manual walk)
- [ ] `rfc/full/rfc1877.txt` (exists) Section 1: "Primary and secondary addresses are negotiated independently.  They serve identical purposes, except that when both are present an attempt SHOULD be made to resolve names using the primary address before using the secondary address."
  → Decision: resolv.conf lists primary before secondary
- [ ] `rfc/full/rfc1877.txt` Section 1.1: "If local peer requests an invalid server address (which it will typically do intentionally) the remote peer specifies the address by NAKing this option, and returning the IP address of a valid DNS server."
- [ ] `rfc/full/rfc1877.txt` Section 1.1, Primary-DNS-Address: "If all four octets are set to zero, it indicates an explicit request that the peer provide the address information in a Config-Nak packet."
- [ ] `rfc/full/rfc1661.txt` Section 5.4: "Reception of a valid Configure-Reject indicates that when a new Configure-Request is sent, it MUST NOT include any of the Configuration Options listed in the Configure-Reject."
- [ ] `rfc/full/rfc1661.txt` Section 5.3: "Reception of a valid Configure-Nak indicates that when a new Configure-Request is sent, the Configuration Options MAY be modified as specified in the Configure-Nak."
- [ ] RFC 3646 Section 6 (quoted in `resolv.go`): "Where DNS parameters such as the domain searchlist or DNS servers have been manually configured, these parameters SHOULD NOT be overridden by DHCP."
  → Decision: the same precedent ranks PPP-learned servers: static wins

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/pppoeclient/auth.go` - `buildIPCPRequest` allocates a 10-octet IPCP packet with option 3 only; `parseIPCPNakAddress` reads option 3 only
- [ ] `internal/component/l2tp/pppoeclient/session.go` - `negotiateIPCP` keeps the outstanding request in a 12-octet array (frame header plus 10). On Configure-Nak it takes only `options.IPAddress` from `ppp.ParseIPCPOptions` and resends. On any Configure-Reject with data it fails the session with "server rejected IPCP IP-Address option", whatever option was rejected. On a peer Configure-Request it Rejects every option except IP-Address. `ipcpResult` holds local and peer address only; `sessionResult` and `iface.PPPoESession` carry no DNS
- [ ] `internal/component/l2tp/ppp/ipcp.go` - `IPCPOptPrimaryDNS` (129), `IPCPOptSecondaryDNS` (131), `ParseIPCPOptions` fills `PrimaryDNS` and `SecondaryDNS`, `WriteIPCPOptions` encodes them. The LNS side already offers DNS
- [ ] `internal/component/iface/pppoe_client.go` - `runSession` applies address, MTU and default route; nothing touches resolvers. `PPPoEClient.status` builds a `pppoeClientStatus` snapshot and has no caller outside tests (only `pppoe_client_test.go` and `rfc2516_pppoe_client_test.go` call it), so no command shows a `pppoe-client` today: `show interface` (`ze-iface-interface-cmd.yang`) has `brief`, `scan`, `type`, `errors`, `rate` and `name`, and `show pppoe` (`ze-pppoe-cmd.yang`) covers the access concentrator's sessions only. The iface package does not import `internal/component/config/system`, and `system` does not depend on iface (checked with `go list -deps`), so the import is cycle-free
- [ ] `internal/component/config/system/resolv.go` - `Resolvers` is the only writer of resolv.conf; `servers()` returns the static name-servers alone when any are configured (RFC 3646 Section 6 quote above `Resolvers`), else every lease's servers ordered by `compareResolverSources`: `resolverOrigin`, then interface, then unit. `resolverOrigin` is a closed `uint8` enumeration in `resolv.go` (`resolverOriginUnspecified`, `resolverOriginDHCPv4`, `resolverOriginDHCPv6`) whose numeric order is the resolv.conf order. `ResolverSource` is built only by `DHCPv4ResolverSource` and `DHCPv6ResolverSource`, each naming its constant; the zero source is refused (`errUnnamedResolverSource`, whose text names both constructors). Their only non-test caller is the DHCP client's `resolv_linux.go` (`SetLease` in the DHCPv4 and DHCPv6 paths); its `register.go` hands the client `system.ProcessResolvers()` as the `Resolvers` field
- [ ] `internal/plugins/iface/dhcp/resolv_linux.go` - the DHCP client sets and withdraws its lease with `SetLease`; empty list withdraws
- [ ] `cmd/ze/hub/main_system.go` `applyResolvConf` - `ProcessResolvers().Configure(ResolvConfPath, NameServers)` on every reload
- [ ] accel-ppp `accel-pppd/ppp/ipcp_opt_dns.c` `dns_recv_conf_req`: Reject when the option length is not 6 or no `[dns]` server is configured, Ack when the value equals the configured server, else Nak. So a server with no DNS configured Rejects, and today's client would fail on that Reject once the options are sent

**Behavior to preserve:**
- A Reject of IP-Address still fails the session
- Static `system name-server` entries alone fill resolv.conf when present
- Existing DHCP source order in resolv.conf is unchanged

**Behavior to change:**
- Request carries options 3, 129, 131; Nak values adopted; Rejected DNS options dropped from the next request; learned servers recorded as a PPP lease and withdrawn when the session ends

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec |
|-----------|------------------------------------------|--------------------|-----------|
| IPCP option codec | `internal/component/l2tp/ppp/ipcp.go` `IPCPOptPrimaryDNS`, `IPCPOptSecondaryDNS`, `ParseIPCPOptions`, `WriteIPCPOptions` | parses and encodes IP-Address and both DNS options; the LNS side already offers DNS with it | extends: the client builds and parses its IPCP with this codec |
| Client IPCP request and Nak parse | `internal/component/l2tp/pppoeclient/auth.go` `buildIPCPRequest`, `parseIPCPNakAddress` | a fixed 10-octet request with option 3 only; reads option 3 only from a Nak | replaces: deleted, the shared codec takes their place (no layering) |
| Client IPCP negotiation | `internal/component/l2tp/pppoeclient/session.go` `negotiateIPCP`, `ipcpResult`, `sessionResult` | negotiates the address only; any Reject fails the session | extends: requests DNS, adopts Nak values, drops Rejected options; results carry DNS |
| PPPoE client session apply | `internal/component/iface/pppoe_client.go` `PPPoEClient.runSession`, `PPPoESession` | applies address, MTU and default route; touches no resolver | extends: sets and withdraws a resolver lease |
| resolv.conf writer | `internal/component/config/system/resolv.go` `Resolvers`, `Resolvers.SetLease`, `Resolvers.servers`, `compareResolverSources` | the only writer of resolv.conf; static servers win, else leases ordered by origin | extends: unchanged as the one writer; leases order by a registered rank instead of a closed constant |
| Lease origin enumeration | `internal/component/config/system/resolv.go` `resolverOrigin` constants, `DHCPv4ResolverSource`, `DHCPv6ResolverSource`, `errUnnamedResolverSource` | a closed list of lease protocols; adding PPP means editing it | replaces: deleted (no layering). Each lease producer registers its origin from its own `init()` with a name and a rank, and gets back an origin value it builds its sources from; `Resolvers` orders by rank, then interface, then unit. The DHCP client registers DHCPv4 and DHCPv6 with ranks that keep today's order; the PPPoE client registers PPP with a rank after both |
| Ranked registration precedent | `internal/component/aaa/aaa.go` `Backend.Priority`; `internal/component/aaa/types.go` `Default` and `backendRegistry.Build` | each AAA backend registers and declares a priority; the bundle orders by it and no list names a backend | the shape this spec copies: a producer declares its rank where it registers, and the consumer sorts, never enumerates |
| Interface show command | `internal/component/iface/yang/ze-iface-interface-cmd.yang` `show interface`; `internal/component/iface/pppoe_client.go` `PPPoEClient.status` | `show interface` answers links; `status` has no caller | extends: a `pppoe-client` node under `show interface`, registered by iface beside `brief` and `type`, answers from `status`, which gains the learned servers |
| DHCP resolver lease | `internal/plugins/iface/dhcp/resolv_linux.go` (`SetLease` with `DHCPv4ResolverSource` / `DHCPv6ResolverSource`) | sets and withdraws the DHCP lease's servers; an empty list withdraws | does not apply: unchanged, it is the pattern the PPP lease follows |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- IPCP Configure-Nak, Configure-Ack or Configure-Reject from the access concentrator, read in `negotiateIPCP`

### Transformation Path
1. The client sends IP-Address 0.0.0.0, Primary-DNS 0.0.0.0, Secondary-DNS 0.0.0.0 (options in that order)
2. Nak: each option present takes the Nak's value; next request carries them
3. Reject: options listed are removed from the outstanding set; IP-Address listed fails the session as today
4. Ack (validated against the last request): DNS values in the request become the learned servers; a 0.0.0.0 value means none
5. `sessionResult` and `iface.PPPoESession` carry the two addresses
6. `runSession` calls `SetLease` with a PPP source named by the `pppoe-client` name, primary first; on session end it withdraws with an empty list
7. On a child 12 change record (sent after every completed renegotiation, whether or not a value differs), `runSession` compares the record's servers with the ones it leased: when they differ it calls `SetLease` with the new list (an empty list withdraws), even when no address, peer address or MTU changed; when they are equal it makes no call
8. `Resolvers` writes resolv.conf: static wins, else leases by registered rank (DHCPv4, DHCPv6, then PPP), then interface, then unit
9. `show interface pppoe-client` reads `PPPoEClient.status`, which carries the leased servers

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| pppoeclient to iface | two fields on `iface.PPPoESession` | No |
| iface to config/system | `Resolvers` handed to iface the way the DHCP plugin receives it in its `register.go` | No |

### Integration Points
- `system.Resolvers.SetLease` and a new PPP source constructor
- Child 12: every completed renegotiation sends a change record with the whole IPCP result; this spec adds the two resolver addresses to that result, so a DNS-only change reaches `runSession` with no change to child 12's notification rule
- `show interface pppoe-client` (new): JSON object per client, keys `name`, `state`, `source-interface`, `ppp-interface`, `session-id`, `local-ip`, `peer-ip`, `uptime`, `dns-servers` (list, primary first, empty when none were learned)

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | resolv.conf written only by `Resolvers` |
| No unintended coupling | Yes | iface receives the `Resolvers` value, as the DHCP plugin does |
| No duplicated functionality | Yes | uses `ppp.ParseIPCPOptions`, `ppp.WriteIPCPOptions` instead of the hand-built request |
| Zero-copy preserved where applicable | Yes | request written into the existing frame buffer |
| Registration over hardcoding, outbound | Yes | the `show interface pppoe-client` node is declared in iface's own command YANG and handler, the way `show interface type` is |
| Registration over hardcoding, inbound | Yes | the closed `resolverOrigin` enumeration is deleted; each lease producer registers its origin and rank from `init()` (the `aaa` `Priority` shape), so a new lease protocol edits only its own package. The relative order is still one decision: it is the ranks, each declared beside its producer, and a duplicate rank or name panics at registration. Searched: `resolverOrigin`, `compareResolverSources`, `DHCPv4ResolverSource` and `DHCPv6ResolverSource` callers (DHCP `resolv_linux.go` only, outside tests) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | An access server that does not know options 129/131 Rejects them rather than dropping the session | RFC 1661 Section 5.4; accel-ppp Rejects (Current Behavior) | session fails against some servers | interop scenario `pppoe-ipcp-dns-rejected-ze-client` | unvalidated |
| A-2 | The iface component runs in the daemon process that owns `ProcessResolvers` | the DHCP plugin uses the same process global | servers written to a process that never flushes | unit test plus the interop resolv.conf read | unvalidated |
| A-3 | No owner wants a leaf to stop asking for DNS | static `name-server` already wins over learned servers | a leaf is added later | owner review of this spec | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Request grows beyond the 12-octet buffer | write of a truncated frame | buffer sized for three options; unit test asserts exact hex |
| R-2 | Lease not withdrawn on a crash path | stale servers in resolv.conf after WAN loss | withdraw in the same deferred cleanup that ends `runSession` |
| R-3 | Nak carrying 0.0.0.0 or a non-unicast address | resolv.conf with an unusable server | ignore such a value and keep requesting, bounded by the IPCP timeout |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | IPCP fails against a server that Rejects DNS (WAN down), or resolv.conf holds wrong servers |
| How is it reverted? | single commit revert |
| Who else touches this path? | child 12 (`negotiateIPCP`), child 8 (IPv6CP and DHCPv6 DNS on the same WAN), DHCP client resolver code |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IPCP Configure-Nak with DNS values | → | `negotiateIPCP` then `runSession` then `Resolvers` | `TestPPPoEClientSetsResolverLease` |
| `pppoe-client` against accel-ppp with `[dns]` set | → | full path to resolv.conf | interop `01-pppoe-chap-ipv4` extended (existing name) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | First IPCP Configure-Request | carries options 3, 129, 131, each 0.0.0.0, in that order (hex asserted) |
| AC-2 | Configure-Nak with 129 = 192.0.2.53 and 131 = 192.0.2.54 | next request carries those values with a new Identifier; Ack then records both |
| AC-3 | Configure-Reject listing 129 and 131 | next request carries option 3 only; session reaches IPCP Opened; no learned servers |
| AC-4 | Configure-Reject listing 131 only | next request carries 3 and 129 |
| AC-5 | Configure-Reject listing 3 | session fails as today |
| AC-6 | IPCP Opened with learned servers, no static name-server | resolv.conf lists 192.0.2.53 then 192.0.2.54 |
| AC-7 | Same with `system name-server 198.51.100.53` configured | resolv.conf lists 198.51.100.53 only |
| AC-8 | Session ends (Done) | learned servers withdrawn; DHCP leases' servers untouched |
| AC-9 | A DHCPv4 lease and a PPP lease both present | DHCPv4 servers first, then PPP servers, no duplicates |
| AC-10 | Nak carrying 0.0.0.0 for a DNS option | value not recorded |
| AC-11 | `show interface pppoe-client` with a session up and servers learned | the client's object carries `dns-servers` `["192.0.2.53", "192.0.2.54"]` in that order; with none learned the key is an empty list, never absent |
| AC-12 | A child 12 renegotiation in the network phase that leaves local address, peer address and MTU unchanged and Naks new DNS values 192.0.2.63 and 192.0.2.64 | resolv.conf lists 192.0.2.63 then 192.0.2.64; no address, route or MTU backend call |
| AC-13 | A renegotiation whose server Rejects 129 and 131 | the PPP lease is withdrawn; resolv.conf falls back to the other sources |
| AC-14 | DHCPv4, DHCPv6 and PPP origins registered | resolv.conf order is DHCPv4, DHCPv6, PPP, exactly today's order for the first two; a second registration of a name or a rank panics at init |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Brings up the WAN with no static name-server and runs `ze` commands that resolve names | IPCP DNS to resolv.conf | interop `01-pppoe-chap-ipv4` reads resolv.conf in the Ze container |
| 2 | Uses an access server that offers no DNS | Reject handled, session up | `pppoe-ipcp-dns-rejected-ze-client` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC1877ClientRequestsDNSOptions` | `internal/component/l2tp/pppoeclient/rfc1877_client_dns_test.go` | AC-1 | |
| `TestRFC1877ClientTakesDNSFromNak` | same | AC-2, AC-10 | |
| `TestRFC1661ClientDropsRejectedDNSOptions` | same | AC-3, AC-4 (RFC 1661 Section 5.4) | |
| `TestRFC1661ClientFailsOnRejectedIPAddress` | same | AC-5 | |
| `TestResolversPPPSourceAfterDHCP` | `internal/component/config/system/resolv_test.go` | AC-9 | |
| `TestResolverOriginRegistryOrdersByRank` | `internal/component/config/system/resolv_test.go` | AC-14: order follows rank; duplicate name or rank panics | |
| `TestPPPoEClientReplacesResolverLeaseOnDNSOnlyChange` | `internal/component/iface/pppoe_client_test.go` | AC-12: a change record with equal addresses and new servers calls `SetLease` once and no address, route or MTU backend method | |
| `TestPPPoEClientWithdrawsLeaseWhenRenegotiationDropsDNS` | `internal/component/iface/pppoe_client_test.go` | AC-13 | |
| `TestShowInterfacePPPoEClientDNSServers` | `internal/component/iface/pppoe_client_test.go` | AC-11: the handler's JSON for a fake client, both with servers and with none | |
| `TestResolversStaticBeatsPPP` | same | AC-7 | |
| `TestPPPoEClientSetsResolverLease` | `internal/component/iface/pppoe_client_test.go` | AC-6, AC-8 with a fake dialer | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| DNS option length | 6 | 6 | 5 (Reject-worthy, not adopted) | 7 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-interface-pppoe-client` | drafted at `test/draft/plugin/show-interface-pppoe-client.ci`, promotes to `test/plugin/show-interface-pppoe-client.ci` (`option=needs-linux:caps=net-admin`: the dialer is linked only on Linux); fixture `plugin/show-interface-pppoe-client` (`internal/test/fixture/register_show_interface_pppoe_client.go`) | an operator with a `pppoe-client` on a link with no access concentrator runs `show interface pppoe-client` and gets the client's object with `state` `discovery` and `dns-servers` `[]`. Proves the empty `dns-servers` half of AC-11 only | drafted, not run |
| (session up) | - | a `.ci` cannot host an access concentrator, so the populated `dns-servers` half is owed in the `01-pppoe-chap-ipv4` checker (`checkZeClient`, `internal/le/interoplab/pppoe/check_client.go`). Today that checker reads only Ze's `resolv.conf` through `checkZeClientCHAP`; it does not run `show interface pppoe-client` | owed, not written |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `01-pppoe-chap-ipv4` (extended: `[dns]` dns1, dns2) | `test/interop-pppoe/scenarios/` | accel-ppp | Nak values reach resolv.conf in order, and `show interface pppoe-client` lists them (AC-11) | |
| `access-up`, DNS clause (child 19 AC-6) | `test/interop-topology/scenarios/access-up/`, scenario owned by child 19, clause owned by this spec | Ze LNS through Ze LAC | the LNS's IPCP DNS reaches the CPE's resolv.conf | |
| `pppoe-ipcp-dns-rejected-ze-client` | `test/interop-pppoe/scenarios/` | accel-ppp without `[dns]` | Reject handled, session up | |

## Files to Modify
- `internal/component/l2tp/pppoeclient/auth.go` - request built with `ppp.WriteIPCPOptions` for the outstanding option set; `parseIPCPNakAddress` replaced by `ppp.ParseIPCPOptions` (no layering)
- `internal/component/l2tp/pppoeclient/session.go` - `negotiateIPCP` option set, Nak, Reject, Ack handling; `ipcpResult` and `sessionResult` DNS fields
- `internal/component/l2tp/pppoeclient/dialer.go` - copy DNS to `iface.PPPoESession`
- `internal/component/iface/pppoe_client.go` - `PPPoESession` DNS fields, lease set, replace on a change record, withdraw; `pppoeClientStatus` gains the servers; `init` registers the PPP origin
- `internal/component/iface/yang/ze-iface-interface-cmd.yang` - `show interface pppoe-client`
- the iface command handler file that serves `show interface type` (located with `gopls` from the YANG path at implementation) - the `pppoe-client` handler reading `PPPoEClient.status`
- `internal/component/iface/pppoe_client.go` `reconcilePPPoEClients` (called from `register.go`) - the PPPoE clients it starts receive `system.ProcessResolvers()`, as the DHCP plugin's `register.go` hands it to the DHCP client
- `internal/component/config/system/resolv.go` - delete the `resolverOrigin` constants and the two DHCP constructors; add origin registration by name and rank, and a source constructor taking a registered origin; `errUnnamedResolverSource` text names the registration; the doc comments on `Resolvers` and `ResolverSource` follow
- `internal/plugins/iface/dhcp/register.go`, `internal/plugins/iface/dhcp/resolv_linux.go` - register the DHCPv4 and DHCPv6 origins from `init()` and build sources from them
- `test/interop-pppoe/scenarios/01-pppoe-chap-ipv4/accel-ppp.conf` and its checker
- `docs/architecture/l2tp/cpe-1-pppoe-client.md`, `docs/features/interfaces.md` (DNS row names PPPoE)
- `rfc/short/rfc1877.md` - client-side implementation note and test links
- `docs/architecture/config/syntax.md` (the design page `internal/component/config/system/resolv.go` declares for resolv.conf generation): read, and nothing there becomes wrong; it covers only the `name-server` leaf-list's config syntax and says nothing about lease origins or their order, so a PPP origin leaves every sentence true

## Files to Create
- `internal/component/l2tp/pppoeclient/rfc1877_client_dns_test.go`
- `test/interop-pppoe/scenarios/pppoe-ipcp-dns-rejected-ze-client/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no leaf (Key Design Decisions) |
| YANG validation constraints | No | |
| YANG custom validators | No | |
| CLI commands/flags | No | |
| CLI grammar | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | No | |
| Pipe completeness | No | |
| Env var registration | No | |
| Doctor check for runtime dependencies | No | resolv.conf path already owned by `Resolvers` |
| Prometheus counters/metrics | No | |
| BGP family surface | No | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `features/interfaces.md` (the interface feature entry names DNS learned from IPCP) |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`: `show interface pppoe-client` and its keys |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md`, the `pppoe-client` subsection child 12 adds: resolvers learned from IPCP, ranked after DHCP and below static `name-server` |
| 7 | Wire format changed? | No | options already defined |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc1877.md`, `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/l2tp/cpe-1-pppoe-client.md` |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | No | |
| 15 | Registered plugin/event/command changed? | No | |
| 16 | Source anchors | Yes | derived at implementation |
| 17 | Existing examples for this area? | Yes | `docs/features/interfaces.md` resolv.conf row |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- `PPPoESession` DNS fields, `PPPResolverSource`, `runSession` sets and withdraws the lease; dialer returns no DNS yet
   - Tests: `TestPPPoEClientSetsResolverLease`
   - Files: `pppoe_client.go`, `resolv.go`, iface registration
   - Verify: test fails because no DNS arrives from the dialer
2. **Phase: IPCP options** -- request, Nak, Reject, Ack; AC-1 to AC-5, AC-10
3. **Phase: ranking, renegotiation and show** -- origin registration (DHCP converted in the same commit), AC-6, AC-7, AC-9, AC-11 to AC-14
4. **Phase: interop** -- extend `01-pppoe-chap-ipv4`, add the Reject scenario; red proof by removing the options from the request

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | Ack and Reject still validated byte-for-byte against the last request with the new option set |
| Data flow | resolv.conf written only through `Resolvers` |
| Rule: no-layering | the hand-built request and `parseIPCPNakAddress` are deleted, not kept beside the new path; the `resolverOrigin` constants and the two DHCP constructors are deleted in the commit that adds registration |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| DNS options in the request | unit hex test |
| Origin registration | `grep -n resolverOriginDHCPv4 internal/component/config/system/resolv.go` finds nothing; `TestResolverOriginRegistryOrdersByRank` passes |
| Show command | `test/plugin/show-interface-pppoe-client.ci` passes |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | only unicast IPv4 values adopted; option length checked by `ParseIPCPOptions` |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | Re-read the source in Current Behavior |
| 3 fix attempts failed | STOP. Report all 3 approaches |

## Design Insights

The Reject path is the real defect behind the feature: today any Reject is
fatal, so adding the options without fixing it would break the session against
every server that has no DNS to offer.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| none | - | grep of `plan/journal/` for client IPCP DNS or Reject finds no row; the all-Rejects-fatal behaviour is a latent defect that only bites once DNS is requested, so this spec carries it in AC-3 |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Static name-server beats learned PPP servers, reusing `Resolvers.servers()` | learned servers first; merge both | RFC 3646 Section 6 precedent already in code; one ranking for every learned source |
| PPP origin ordered after DHCPv6 | before DHCPv4 | adding a source never reorders servers existing configurations already get |
| No `no-peer-dns` leaf | a leaf to suppress the request | static `name-server` already overrides; a leaf with no need is an option nobody asked for (`ai/rules/simplicity.md`) |
| Source keyed by the `pppoe-client` name | keyed by `pppN` | the unit number changes on reconnect; the name does not |

## Known Limitations
- The LAN `dns-resolver` forwards only to configured `forward server` entries; it does not use learned servers. A CPE that wants LAN clients to use the ISP's resolvers through Ze needs a forward-to-learned option: a separate feature for the owner to decide, recorded for the mapping guide (child 10).
- RFC 1877 NBNS options (130, 132) are not requested; nothing in a home CPE uses them.

## RFC Documentation (Scope: protocol)
- Request, Nak and Ack handling quote RFC 1877 Section 1.1 and 1.3; the Reject handling quotes RFC 1661 Section 5.4.

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

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] Wiring Test table complete
- [ ] Targeted tests pass; `./le verify changed` over the closing commits
- [ ] Every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it

## Blocker

Second closure attempt, 2026-11-08 (evening): B-1 to B-3 are cleared (table
"Cleared" below). It stopped again before commit A on B-4.

| # | What blocks | Producer | What clears it |
|---|-------------|----------|----------------|
| B-4 | `./le rfc check` (log `tmp/session/2026-11-07-104251/scratch/c13-rfccheck.log`, line 66): the mutant record `rfc/discrimination/rfc1661/RFC1661-5.4-5.json` for this spec's unit `rfc1661_client_dns_test.go::TestRFC1661ClientDropsRejectedDNSOptions` "no longer verifies (producer-changed)". Its break, `if rejected.HasPrimary {` -> `if false {`, was taken against `session.go::runIPCP` before `a02374d9ad` (LCP restart counter, 19:13) changed that producer. A closure checks proof records and never writes them (`/ze-close` step 6) | `internal/component/l2tp/pppoeclient/session.go` `runIPCP` | re-record it under `/ze-implement` step 5 (`./le rfc discriminate` for RFC1661-5.4-5 on that unit), then `./le rfc check` names no line with `rfc1661_client_dns_test.go`. The same commit staled the `negotiateLCP`, `runClientAuth`, `runIPCP` and `keepaliveLoop` records of RFC1661-4.3-1, 5.1-3, 5.2-3, 5.2-4, 5.3-7, 5.4-3, 5.4-4, 5.4-5 and 5.8-2 (log lines 49-67, units in `rfc1661_lcp_reply_test.go`, `rfc1661_renegotiation_test.go`, `rfc1661_lcp_transition_test.go`); those units are child 12's and the LCP reply work's, not this spec's, but RFC1661-5.4-5.json holds both kinds, so one re-take of that stem clears them together |

Cleared on 2026-11-08:

| # | Evidence |
|---|----------|
| B-1 | `rwinv/out-interop/c13-access-up-dns-broken.log`, broken by `c13-request-no-dns` (the request sent without 129/131): "access-up: FAIL ... ze-cpe's PPPoE client never reported the IPCP DNS servers ... pppoe0 is up with dns-servers [], so IPCP learned none", "0 passed, 1 failed"; clean `c13-access-up-dns-clean.log` "1 passed, 0 failed" (`rwinv/interop-c13au.log`) |
| B-2 | promoted to `test/plugin/show-interface-pppoe-client.ci` in `0736f9808d` after 80 runs under load with no failure. Red: `p23rec/out-promo2/c13-broken.log` "ZE-OBSERVER-FAIL: show interface pppoe-client: \"dns-servers\" = null, want []", the break being `DNSServers: c.dnsServers` in `PPPoEClient.status` (`pppoe_client_broken.go`: the nil list handed through, not cloned); clean `c13-clean.log` "pass 1/1" (Ze 7.3 guest). The break changed from the first attempt's because an emptied list could not discriminate: the test asserts `[]`, which the clean path also produces before a session exists, so only the nil list, the defect the clone in `status` prevents, reddens it |
| B-3 | darwin `c13-unit.log`: `ok` for pppoeclient, iface, iface/cmd, config/system and interoplab/pppoe (dhcp has no darwin test files); Linux `p23rec/out-promo2/dhcp.log`: `ok internal/plugins/iface/dhcp` in the guest, after `7854ccf79b` (child 23's lease-validity fix) |

The first attempt's table follows, kept as its record.

Closure stopped before commit A on 2026-11-08 because three results it needs
do not exist yet. Each is a run, not code: the code is committed.

| # | What is missing | Producer it proves | What clears it |
|---|-----------------|--------------------|----------------|
| B-1 | The red half of the `access-up` DNS clause (child 19 AC-6, owned here). `tmp/session/2026-11-07-104251/scratch/rwinv/out-interop/c13-access-up-dns-broken.log` is a setup failure, not a red: the overlay then in place did not compile (the compiler reported `undefined: runIPCP` in `ipcp_independent.go`), so it reads `interop: 0 passed, 0 failed`. No log in the scratch tree holds the AC-6 FAIL line | `ipcpInitialRequest` (`internal/component/l2tp/pppoeclient/session.go`) | rerun `access-up` broken by the regenerated `rwinv/c13-request-no-dns` overlay (`rwinv/interop-labs.sh`); expect `AC-6: show interface pppoe-client on ze-cpe reports dns-servers []` |
| B-2 | The functional test of AC-11's empty half: `test/draft/plugin/show-interface-pppoe-client.ci` was drafted and never run, so it is not promoted, and the Deliverables row "`test/plugin/show-interface-pppoe-client.ci` passes" is unmet. It needs Linux and CAP_NET_ADMIN | `handleShowInterfacePPPoEClient` (`internal/component/iface/cmd/show_pppoe_client.go`), `PPPoEClient.status` | on Linux (QEMU guest): `./le test plugin --draft --pattern show-interface-pppoe-client`, green, then a red by breaking `DNSServers` in `PPPoEClient.status`; then promote to `test/plugin/` |
| B-3 | A unit run after this closure renamed `rfc1877_client_dns_test.go` to `rfc1661_client_dns_test.go` (`./le rfc move`, because every tag in it cites RFC 1661). The rename changes no code, but the packages were last run on darwin at `518401eb75`, before child 23 reshaped `resolv.go`, and the DHCP client's Linux resolver tests never ran | the packages below | `tags="ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt \| sort -u \| tr '\n' ' ')"; ./le job run label c13-unit quiet command go test -race -tags "$tags" ./internal/component/l2tp/pppoeclient/ ./internal/component/iface/ ./internal/component/iface/cmd/ ./internal/component/config/system/ ./internal/plugins/iface/dhcp/ ./internal/le/interoplab/pppoe/` (the dhcp package's resolver tests are Linux-only) |

When B-4 is re-recorded, set Status back to `in-progress` and run `/ze-close`
again: B-1 to B-3 are done, the closure sections below are filled, and the
review is clean.

## Implementation Summary

### What Was Implemented
- `518401eb75`: the client requests IP-Address, Primary-DNS and Secondary-DNS at 0.0.0.0 (`ipcpInitialRequest`), adopts only unicast Nak values (`adoptNakDNS`), drops a Rejected DNS option and re-requests, fails on a Rejected IP-Address (`negotiateIPCP`), and reports learned servers in `ipcpResult`, `sessionResult` and `iface.PPPoESession`. The hand-built request (`buildIPCPRequest`) and `parseIPCPNakAddress` are deleted; `sendIPCPRequest` writes through `ppp.WriteIPCPOptions`
- `518401eb75`: the closed `resolverOrigin` enumeration is replaced by `system.RegisterResolverOrigin` (DHCPv4 100, DHCPv6 200 in `internal/plugins/iface/dhcp/register.go`, PPP 300 in `internal/component/iface/register.go`); `PPPoEClient.runSession` leases, replaces on a DNS-only change record, and withdraws in `endSessionLeases`; `show interface pppoe-client` answers `PPPoEClientStatus` with `dns-servers`
- `85167bd4ed`, `b836713544`: the `01-pppoe-chap-ipv4` checker reads Ze's resolv.conf and `show interface pppoe-client` over REST; `pppoe-ipcp-dns-rejected-ze-client` added (accel-ppp with no `[dns]`)
- this closure: test file renamed for its RFC; `features/interfaces.md` gains the IPCP DNS paragraph, the two scenarios in its Interop row and today's Doc review

### Bugs Found/Fixed
- Any IPCP Configure-Reject ended the session, so asking for DNS would have broken every server with none to offer (Design Insights). Covered by `TestRFC1661ClientDropsRejectedDNSOptions` and the red walk of `pppoe-ipcp-dns-rejected-ze-client`

### Documentation Updates
- `docs/architecture/l2tp/cpe-1-pppoe-client.md` (request, Nak, Reject), the resolv.conf row of `docs/features/interfaces.md`, the `system` resolver paragraph of `docs/guide/configuration.md`, the `show interface` block of `docs/guide/command-reference.md`, and the client note in `rfc/short/rfc1877.md`: all in `518401eb75`
- `features/interfaces.md`: this closure, anchors `session.go -- ipcpInitialRequest, adoptNakDNS`, `pppoe_client.go -- PPPoEClient.endSessionLeases`, `check_client.go -- checkZeClient`
- `./le doc check all`: red. The only failures naming this spec's surface are the website and wiki: `../gh-pages` has no row for `show interface pppoe-client` (nor for `show interface delegated-prefix`, `update serve start/stop/pause`, `show command reference/catalog`, from other specs), and `../wiki/command-catalog.md` disagrees with the live catalog. Regenerating the site is `./le site build` in the sibling checkout, which this closure does not own

### Deviations from Plan
- Child 23 (`25a0665638`, `1cc0217d82`, `8d13a9dceb`) later replaced the ordering inside `Resolvers` with the `system dns-source` selection. The registry, ranks and the static-wins rule this spec added survive; the PPP lease is now one `received` record (`docs/features/dns-resolver.md`)
- The test file is `rfc1661_client_dns_test.go`, not `rfc1877_client_dns_test.go`: `./le rfc check` names a file by the RFC its tags cite, and RFC 1877 has no requirement ids

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The `access-up` red walk was reported as observed | its broken run never compiled, so it proved nothing | reading `c13-access-up-dns-broken.log` at closure | B-1 |
| approach | A later commit (`a02374d9ad`) changed `runIPCP` and left this spec's RFC1661-5.4-5 mutant record stale; no step re-took the records over the producer it changed | a producer change owes the re-take of every record over that producer, whichever spec tagged the unit | `./le rfc check` at the second closure | B-4 |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Request 129 and 131 at 0.0.0.0 | Done | `session.go` `ipcpInitialRequest` | |
| Handle Nak, Reject, Ack for them | Done | `session.go` `negotiateIPCP`, `adoptNakDNS`, `learnedDNS` | |
| Learned servers to resolv.conf, below static | Done | `pppoe_client.go` `leaseDNS`; `resolv.go` `Resolvers.SetLease` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRFC1877ClientRequestsDNSOptions` | unit run owed (B-3) |
| AC-2 | Done | `TestRFC1877ClientTakesDNSFromNak`; interop `01-pppoe-chap-ipv4` | |
| AC-3 | Done | `TestRFC1661ClientDropsRejectedDNSOptions`; interop `pppoe-ipcp-dns-rejected-ze-client` | |
| AC-4 | Done | `TestRFC1661ClientDropsRejectedDNSOptions` | |
| AC-5 | Done | `TestRFC1661ClientFailsOnRejectedIPAddress` | |
| AC-6 | Done | `TestPPPoEClientSetsResolverLease`; interop `01-pppoe-chap-ipv4` | `access-up` red owed (B-1) |
| AC-7 | Done | `TestResolversStaticBeatsPPP` | |
| AC-8 | Done | `TestPPPoEClientSetsResolverLease` | |
| AC-9 | Done | `TestResolversPPPSourceAfterDHCP` | |
| AC-10 | Done | `TestRFC1877ClientTakesDNSFromNak` | |
| AC-11 | Partial | `TestShowInterfacePPPoEClientDNSServers`; interop `01-pppoe-chap-ipv4` (populated) | `.ci` run owed (B-2) |
| AC-12 | Done | `TestPPPoEClientReplacesResolverLeaseOnDNSOnlyChange` | |
| AC-13 | Done | `TestPPPoEClientWithdrawsLeaseWhenRenegotiationDropsDNS` | |
| AC-14 | Done | `TestResolverOriginRegistryOrdersByRank` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| the four RFC 1877/1661 client tests | Done | `internal/component/l2tp/pppoeclient/rfc1661_client_dns_test.go` | renamed at closure |
| `TestResolversPPPSourceAfterDHCP`, `TestResolversStaticBeatsPPP` | Done | `internal/component/config/system/resolv_test.go` | |
| `TestResolverOriginRegistryOrdersByRank` | Changed | `internal/component/config/system/resolv_origin_test.go` | own file |
| the four iface tests | Changed | `internal/component/iface/pppoe_client_dns_test.go` | own file, not `pppoe_client_test.go` |
| `show-interface-pppoe-client` | Partial | `test/draft/plugin/show-interface-pppoe-client.ci` | B-2 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `pppoeclient/auth.go`, `session.go`, `dialer.go` | Done | `518401eb75` |
| `iface/pppoe_client.go`, YANG, `cmd/show_pppoe_client.go` | Done | `518401eb75` |
| `config/system/resolv.go`, dhcp `register.go`, `resolv_linux.go` | Done | `518401eb75` |
| interop scenarios and checker | Done | `b836713544`, `85167bd4ed` |
| docs and `rfc/short/rfc1877.md` | Done | `518401eb75` |

### Audit Summary
- **Total items:** 14 AC, 3 requirements
- **Done:** 13 AC, 3 requirements
- **Partial:** AC-11, until B-2 runs (no scope change: the run is owed, not dropped)
- **Skipped:** none
- **Changed:** test file placement (Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The client asks for DNS and takes the Nak's values | interop, accel-ppp `[dns]` | `01-pppoe-chap-ipv4`, 2026-11-08: clean `c13-dns-01-clean.log` "1 passed"; broken by `c13-request-no-dns` (request sent without 129/131): `c13-dns-01-broken.log` FAIL "ze resolv.conf /tmp/resolv.conf = ... nameserver 127.0.0.11 ..., want ... nameserver 192.0.2.53\nnameserver 192.0.2.54" |
| A server with no DNS still brings the session up | interop, accel-ppp without `[dns]` | `pppoe-ipcp-dns-rejected-ze-client`, 2026-11-08: clean "1 passed"; broken by `c13-dns-reject-fatal` (a DNS Reject fails the session): FAIL "pppoe0 never carried the IPCP address" |
| Learned servers reach resolv.conf ranked below static | unit | `TestResolversStaticBeatsPPP`, `TestResolversPPPSourceAfterDHCP`; run owed (B-3) |
| A Ze LNS's IPCP DNS reaches the CPE | interop, Ze LNS through Ze LAC | `access-up` clean "1 passed" (`c13-access-up-dns-clean.log`); red owed (B-1) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none: B-1 to B-3 are runs this spec still owes before it closes | | |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | not recorded: closure stopped before commit A (Blocker) |
| `./le spec review check` | not run |
| Rounds | 1 |
| Reviewer lenses used | logic and wiring of `negotiateIPCP` (Nak, Reject, Ack against the outstanding set), lease lifetime in `runSession` and `endSessionLeases`, show handler JSON (`dns-servers` never absent or null), interop checker refusals, style (no peer-reachable `panic`: the only panics are in `resolverOriginRegistry.register`, reached from `init`) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | none | round 1 found no BLOCKER or ISSUE | | |

NOTE (not blocking): the `leaseDNS` comment says "the next change retries" a failed write. `SetLease` records the lease before writing, so any later write of resolv.conf, from any source, carries it; a change record with the same servers does not call `SetLease` again.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/l2tp/pppoeclient/rfc1661_client_dns_test.go` | yes | `./le rfc move` output "moved ... -> rfc1661_client_dns_test.go" |
| `test/interop-pppoe/scenarios/pppoe-ipcp-dns-rejected-ze-client/` | yes | `ls test/interop-pppoe/scenarios/` lists it |
| `test/plugin/show-interface-pppoe-client.ci` | yes | promoted in `0736f9808d` (B-2 cleared) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-14 | each test function exists | `git grep -l "func <name>("` found all eleven planned functions, 2026-11-08; runs owed (B-3) |
| AC-14 | closed enumeration deleted | `git grep` for `resolverOriginDHCPv4` and `DHCPv4ResolverSource` over `*.go`: no match |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| IPCP Nak with DNS values | interop `01-pppoe-chap-ipv4` | yes: red and green above |
| `show interface pppoe-client` | `test/plugin/show-interface-pppoe-client.ci` | yes: red (nil list) and green, B-2 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `pppoe-ipcp-dns-rejected-ze-client` green against accel-ppp with no `[dns]` |
| A-2 | confirmed | `01-pppoe-chap-ipv4` reads the learned servers from the daemon's `/tmp/resolv.conf` |
| A-3 | confirmed | no leaf asked for; static `name-server` overrides (`TestResolversStaticBeatsPPP`) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `features/interfaces.md` IPCP DNS paragraph | `ipcpInitialRequest`, `adoptNakDNS`, `endSessionLeases` read at closure | yes; `./le feature check` names no refusal for it |
| RFC status | `./le rfc update` regenerated nothing new; `./le rfc check` names no rfc1877 violation after the rename | yes; RFC 1661 rows 5.1-3, 5.2-3, 5.2-4, 5.3-7, 5.4-3, 5.4-4 and 5.4-5 carry SHIFTED audit verdicts from `ed52d128eb` (`rfc1661_lcp_reply_test.go`), not from this spec: `./le rfc reseal stem rfc1661` |

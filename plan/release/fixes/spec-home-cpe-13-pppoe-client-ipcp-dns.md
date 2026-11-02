# Spec: home-cpe-13-pppoe-client-ipcp-dns

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P3 |
| Release reason | A home CPE learns its resolvers from the access network; the PPPoE client never asks for them, so a router with no static `name-server` cannot resolve names once the WAN is up. The `access-up` scenario of the umbrella asserts DNS learned from IPCP. No accepted configuration misbehaves today (the options are never sent), so this is a feature (P3) even though the umbrella lists it as a release fix; the umbrella orders it last. |
| RFC gaps | RFC 1877 Section 1.1 and Section 1.3 (client side: request Primary and Secondary DNS, take the value from a Configure-Nak); RFC 1661 Section 5.4 (drop Rejected options from the next request) on the client's IPCP |
| Depends | `spec-home-cpe-12-pppoe-client-lcp-renegotiation.md` (shares `negotiateIPCP`; a renegotiated IPCP re-learns DNS) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-31 |

Parent: plan/home-cpe/spec-home-cpe-0-umbrella.md

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
- [ ] `internal/component/l2tp/pppoeclient/auth.go` (78L) - `buildIPCPRequest` allocates a 10-octet IPCP packet with option 3 only; `parseIPCPNakAddress` reads option 3 only
- [ ] `internal/component/l2tp/pppoeclient/session.go` - `negotiateIPCP` (l.623) keeps the outstanding request in a 12-octet array (frame header plus 10). On Configure-Nak it takes only `options.IPAddress` from `ppp.ParseIPCPOptions` and resends. On any Configure-Reject with data it fails the session with "server rejected IPCP IP-Address option", whatever option was rejected. On a peer Configure-Request it Rejects every option except IP-Address. `ipcpResult` holds local and peer address only; `sessionResult` and `iface.PPPoESession` carry no DNS
- [ ] `internal/component/l2tp/ppp/ipcp.go` - `IPCPOptPrimaryDNS` (129), `IPCPOptSecondaryDNS` (131), `ParseIPCPOptions` fills `PrimaryDNS` and `SecondaryDNS`, `WriteIPCPOptions` encodes them. The LNS side already offers DNS
- [ ] `internal/component/iface/pppoe_client.go` - `runSession` applies address, MTU and default route; nothing touches resolvers. The iface package does not import `internal/component/config/system`, and `system` does not depend on iface (checked with `go list -deps`), so the import is cycle-free
- [ ] `internal/component/config/system/resolv.go` - `Resolvers` is the only writer of resolv.conf; `servers()` returns the static name-servers alone when any are configured (RFC 3646 Section 6 quote above `Resolvers`), else every lease's servers ordered by `resolverOrigin` (DHCPv4 then DHCPv6), then interface, then unit. `ResolverSource` is built only by `DHCPv4ResolverSource` and `DHCPv6ResolverSource`; the zero source is refused
- [ ] `internal/plugins/iface/dhcp/resolv_linux.go` - the DHCP client sets and withdraws its lease with `SetLease`; empty list withdraws
- [ ] `cmd/ze/hub/main_system.go` l.185 - `Configure(ResolvConfPath, NameServers)` on every reload
- [ ] accel-ppp `accel-pppd/ppp/ipcp_opt_dns.c` `dns_recv_conf_req` (l.97-111): Reject when the option length is not 6 or no `[dns]` server is configured, Ack when the value equals the configured server, else Nak. So a server with no DNS configured Rejects, and today's client would fail on that Reject once the options are sent

**Behavior to preserve:**
- A Reject of IP-Address still fails the session
- Static `system name-server` entries alone fill resolv.conf when present
- Existing DHCP source order in resolv.conf is unchanged

**Behavior to change:**
- Request carries options 3, 129, 131; Nak values adopted; Rejected DNS options dropped from the next request; learned servers recorded as a PPP lease and withdrawn when the session ends

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
7. `Resolvers` writes resolv.conf: static wins, else DHCPv4, DHCPv6, then PPP sources

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| pppoeclient to iface | two fields on `iface.PPPoESession` | No |
| iface to config/system | `Resolvers` handed to iface the way the DHCP plugin receives it in its `register.go` | No |

### Integration Points
- `system.Resolvers.SetLease` and a new PPP source constructor
- Child 12: a renegotiated IPCP produces new values through its change record; the lease is replaced
- `show` status of the pppoe-client gains the learned servers

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | resolv.conf written only by `Resolvers` |
| No unintended coupling | Yes | iface receives the `Resolvers` value, as the DHCP plugin does |
| No duplicated functionality | Yes | uses `ppp.ParseIPCPOptions`, `ppp.WriteIPCPOptions` instead of the hand-built request |
| Zero-copy preserved where applicable | Yes | request written into the existing frame buffer |
| Registration over hardcoding, outbound | N-A | no command or handler |
| Registration over hardcoding, inbound | No, justified | `resolverOrigin` in `resolv.go` is a closed ranking of lease protocols and gains a PPP value; the ranking is one decision that must live in one place. Searched: `resolverOrigin`, `compareResolverSources`, `DHCPv4ResolverSource` callers |

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
| AC-11 | `show` for the pppoe-client | lists the learned servers |

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
| `TestResolversStaticBeatsPPP` | same | AC-7 | |
| `TestPPPoEClientSetsResolverLease` | `internal/component/iface/pppoe_client_test.go` | AC-6, AC-8 with a fake dialer | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| DNS option length | 6 | 6 | 5 (Reject-worthy, not adopted) | 7 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | - | a `.ci` cannot host an access concentrator; interop carries the end-to-end proof | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `01-pppoe-chap-ipv4` (extended: `[dns]` dns1, dns2) | `test/interop-pppoe/scenarios/` | accel-ppp | Nak values reach resolv.conf in order | |
| `pppoe-ipcp-dns-rejected-ze-client` | `test/interop-pppoe/scenarios/` | accel-ppp without `[dns]` | Reject handled, session up | |

## Files to Modify
- `internal/component/l2tp/pppoeclient/auth.go` - request built with `ppp.WriteIPCPOptions` for the outstanding option set; `parseIPCPNakAddress` replaced by `ppp.ParseIPCPOptions` (no layering)
- `internal/component/l2tp/pppoeclient/session.go` - `negotiateIPCP` option set, Nak, Reject, Ack handling; `ipcpResult` and `sessionResult` DNS fields
- `internal/component/l2tp/pppoeclient/dialer.go` - copy DNS to `iface.PPPoESession`
- `internal/component/iface/pppoe_client.go` - `PPPoESession` DNS fields, lease set and withdraw, status fields
- `internal/component/config/system/resolv.go` - PPP origin after DHCPv6, `PPPResolverSource(name)`; the doc comment on `Resolvers` names the PPP source
- the iface registration that hands `Resolvers` to the PPPoE client
- `test/interop-pppoe/scenarios/01-pppoe-chap-ipv4/accel-ppp.conf` and its checker
- `docs/architecture/l2tp/cpe-1-pppoe-client.md`, `docs/features/interfaces.md` (DNS row names PPPoE)
- `rfc/short/rfc1877.md` - client-side implementation note and test links

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
| 1 | New user-facing feature? | Yes | `features/` PPPoE client entry |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | show output of the pppoe-client gains DNS; `docs/guide/command-reference.md` if it lists the fields |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | interfaces guide, PPPoE client section |
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
3. **Phase: ranking and show** -- AC-6, AC-7, AC-9, AC-11
4. **Phase: interop** -- extend `01-pppoe-chap-ipv4`, add the Reject scenario; red proof by removing the options from the request

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | Ack and Reject still validated byte-for-byte against the last request with the new option set |
| Data flow | resolv.conf written only through `Resolvers` |
| Rule: no-layering | the hand-built request and `parseIPCPNakAddress` are deleted, not kept beside the new path |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| DNS options in the request | unit hex test |
| PPP source | `grep -n PPPResolverSource internal/component/config/system/resolv.go` |

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
- [ ] AC-1..AC-11 all demonstrated
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

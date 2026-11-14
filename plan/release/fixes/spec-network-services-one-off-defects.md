# Spec: Interface, firewall, dataplane and network service one-off defects from the journal

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P1 |
| Release reason | Each row is incorrect shipped behaviour in interface management, the firewall and VPP ACL backends, DNS, NTP, DHCP and TFTP, gNMI or the system RIB |
| RFC gaps | RFC2131-4.1-7, RFC2131-4.1-15 |
| Depends | - |
| Phase | - |
| Handoff | verify |
| Updated | 2026-11-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The 2026-10-23 journal sweep left about 265 open rows that fit none of the
thirteen root-cause specs. This spec owns the 26 in interface management (iface,
netlink monitor, resolver, RA), the firewall and VPP ACL backends, policy
routing, traffic control, MPLS egress, DNS (geodns), NTP, DHCP and TFTP
listeners, gNMI, the system RIB and flow export enrichment. Each row is a claim
until a red test reproduces it at HEAD.

Existing specs checked first: `spec-vrrp-owner-arp-rfc-defects` already owns the
vpp `verifyMatch` ARP row. The govpp reply-deadline rows and `cos.Resolve` stay
with root causes S4 and S2.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` and the iface, firewall, VPP and DNS doors in `ai/doors/README.md`
- [ ] `ai/rules/platform-linux.md`

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc2308.txt` Section 3
- [ ] `rfc/full/rfc2181.txt` Section 8
- [ ] `rfc/full/rfc5905.txt` (leap indicator, stratum 0)
- [ ] `rfc/full/rfc2131.txt` Section 4.1 (option overload) and `rfc/short/rfc2131.md` rows RFC2131-4.1-7 and RFC2131-4.1-15, both `{partial}`: the server half is tagged and recorded, the client half is the gap AC-31 owns (`rfc/audit/rfc2131/RFC2131-4.1-7.json`, `RFC2131-4.1-15.json`)

## Current Behavior (MANDATORY)

Spot-checked on 2026-10-23: `(*monitor).handleLinkUpdate` in
`internal/plugins/iface/netlink/monitor_linux.go` tracks only whether an index
is known (`m.known.LoadOrStore`), with no comparison of the previous state, so
the 2026-08-11 row is present.

- [ ] `internal/plugins/firewall/vpp/` - `bindAllACLs`
- [ ] `internal/plugins/firewall/nft/` - `lowerAddrMatch`; `internal/component/firewall/` - `validateMatch`, `ApplyAll`
- [ ] `internal/plugins/policyroute/config.go` - `parseIfaceSpec`
- [ ] `internal/plugins/iface/netlink/monitor_linux.go` - `run`, `handleLinkUpdate`
- [ ] `internal/plugins/iface/netlink/mirror_linux.go` - `SetupMirror`
- [ ] `internal/component/iface/config_apply.go` - `desiredState`, `reconcileOnReadyWithJournal`
- [ ] `internal/component/iface/resolve.go` - `onLinkEvent`, `matchByMAC`
- [ ] `internal/plugins/iface/ra/doctor.go` - `checkRAForwarding`
- [ ] `internal/plugins/iface/dhcp/` - v6 worker `maintain`, `extend`
- [ ] `internal/plugins/dhcpserver/register.go`, `internal/plugins/tftpserver/` - `bindListeners`, `bindTFTP`
- [ ] `internal/plugins/geodns/config.go` - `parseTTL`, `parseSOA`
- [ ] `internal/plugins/ntp/ntp.go` - `syncWorker.doSync`
- [ ] `internal/component/gnmi/get.go` - `Server.Get`
- [ ] `internal/component/sysrib/sysrib.go` - `recomputeBest`
- [ ] `internal/plugins/flowexport/enrichbgp.go` - `bgpEnrichBuilder.Start`
- [ ] `internal/component/ping/cmd/` - `handleResolvePing`
- [ ] `internal/plugins/iface/dhcp/dhcp_v4_linux.go` - `removeV4Route` (AC-28)
- [ ] `internal/plugins/iface/vpp/tunnel.go`, `internal/plugins/iface/vpp/vxlan.go`, `internal/plugins/iface/vpp/wireguard.go` - `createGRETunnel`, `createIPIPTunnel`, `createVxlanTunnel`, `ensureWireguardInterface` (AC-29)
- [ ] `internal/plugins/iface/dhcp/dhcp_v4_exchange_linux.go` - `nclient4Exchanger.acquire` hands each DHCPOFFER and DHCPACK to the vendored `nclient4`, whose receive loop decodes with `dhcpv4.FromBytes`; `FromBytes` reads `sname` and `file` as NUL-terminated strings and never consults option 52, so an option a server places in an overloaded field is lost (AC-31; read 2026-11-14)
- [ ] `internal/component/iface/cmd/monitor_netlink_linux.go` - `ifName`, `linkUpdateToEvent` (AC-30)

## Existing Mechanisms (MANDATORY)

Added 2026-11-14 when three rows moved here from
`spec-typed-identity-keys.md`; the section covers those rows. Each fix extends
the producer its AC names; none adds a parallel path.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| DHCP route metric state | `internal/component/iface/register.go` `handleLinkDown`, `routeMetricState`; `handleRouterLost` and `restoreAcceptRaDefrtr` remove at `effectiveMetric` | The iface component moves a learned default route to metric + 1024 on link down and tracks where it is | Extends: AC-28 carries the effective metric to the DHCP client's teardown, as the RA half already does |
| VPP name map existence check | `internal/plugins/iface/vpp/naming.go` `nameMap.lookupIndex`; `ensureWireguardInterface` and the loopback create use it | WireGuard and loopback creates return the existing index instead of creating again | Extends: AC-29 applies the same check to the GRE, IPIP and VXLAN creates |
| Server-side option overload interpreter (row added 2026-11-14, read at its producer) | `internal/plugins/dhcpserver/options.go` `findOption`, `overloadValue` | Finds an option in the options field first, then in `file` and then `sname` as the options field's option 52 says; an option 52 inside `file` or `sname` is never consulted | Not stated in this spec; AC-31 owes the same interpretation on the client's receive path |
| Link-message name cache | `internal/component/iface/cmd/monitor_netlink_linux.go` `ifNameCache`, written only by `linkUpdateToEvent` (58ce054f7e) | A reused index is never cached under the old link | Extends: AC-30 binds a route or address event's name to the link it belongs to, not a lookup by index after the fact |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
Config commit and reload; netlink events; DNS, NTP, DHCP and gNMI requests; the
operator commands `show interface`, `resolve ping`.

### Transformation Path
1. Config parse to desired state per component.
2. Reconcile against kernel or VPP state through netlink or the binary API.
3. Event fan-out to subscribing plugins.

### Boundaries Crossed
Ze to kernel (netlink, nftables, tc); Ze to VPP; Ze to network clients.

### Integration Points
The iface owned-device and owned-address registries, the RIB event bus, the firewall registry.

### Architectural Verification
- [ ] Every registry keyed by device resolves the logical name to the kernel device in one place

## Risks & Assumptions

### Assumptions
- Kernel-level proof runs in Ze's own QEMU guest (`ai/META-DECISIONS.md`).

### Risks
- `reconcileOnReadyWithJournal` changes affect DHCP and SLAAC; prove with the integration suite.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Commit on an enabled unit holding a DHCP lease | → | `reconcileOnReadyWithJournal` | QEMU integration test asserting the lease, SLAAC and fe80:: addresses survive |
| NXDOMAIN query to geodns | → | negative answer builder | unit asserting SOA in authority |

## Acceptance Criteria

| AC | Rows | Observable |
|----|------|------------|
| AC-1 | `bindAllACLs` | 255 foreign plus one desired ACL is refused before the count wraps; a unit asserts the refusal |
| AC-2 | NTP `doSync` | An unsynchronised server (leap 3) is recorded with its own reason, not "kiss of death"; the leap test runs before the stratum test |
| AC-3 | flow export enrichment | Turning `enrichment bgp` on by reload requests a replay and fills the prefix tree at once |
| AC-4 | iface route add side | Routes the interface layer adds are identified by protocol on add as well as delete |
| AC-5 | geodns SOA bounds | `parseSOA` bounds `minimum` like `parseTTL` (RFC 2181 Section 8) |
| AC-6 | DNS negative answers | RFC 2308 Section 3: "Name servers authoritative for a zone MUST include the SOA record of the zone in the authority section of the response when reporting an NXDOMAIN or indicating that no data of the requested type exists." |
| AC-7 | resolver link-event fan-out | `onLinkEvent` uses the coalescing queue; a burst of 100 events loses none |
| AC-8 | policy-route protocol enum | The policy-route `from/protocol` leaf is gated by the same model test as the firewall leaf |
| AC-9 | firewall ARP literal addresses | A literal address match in family arp is refused like a set match |
| AC-10 | identity-default mappings x4 | Owned-device and owned-address registries, the tc backend and the RA doctor all read the kernel device a logical name resolves to |
| AC-11 | MPLS egress pop via `lo` | `TestMPLSIntegration_EgressPopNoNextHop` passes in Ze's guest, or the installer refuses the route form the kernel drops, with the reason |
| AC-12 | DHCP and TFTP listeners | A listen interface that appears after boot is bound when it appears |
| AC-13 | `recomputeBest` no-op test | An equal-cost member arrival that changes the FIB primary next hop is not suppressed |
| AC-14 | policy route `interface "*"` | One bad member is refused at verify with its name; the other members still apply |
| AC-15 | `resolve ping` source | The source address constrains the resolution family |
| AC-16 | DHCPv6 mid-exchange flap | A link event during Renew, Rebind or Decline aborts the exchange; deleted addresses are not reinstalled |
| AC-17 | `show interface counters`, MAC selector | Counters read only link stats; a failed address dump on another device does not fail MAC resolution |
| AC-18 | `reconcileOnReadyWithJournal` | Addresses another owner installed (DHCP, SLAAC, kernel link-local) are not removed |
| AC-19 | gNMI `Get` encoding | An encoding Ze does not advertise is refused with `InvalidArgument` or `Unimplemented` (gNMI specification Section 3.3.1, quoted in the row) |
| AC-20 | netlink monitor overflow | ENOBUFS triggers a resync and the monitor keeps running |
| AC-21 | `interface/up` events | Up and down events are emitted only on a state change |
| AC-22 | mirror with absent destination | An absent mirror destination is reported and retried, not a whole-commit abort |
| AC-23 | LDP x2 (`TestLDPInteropFRR` rerun, `transport-address` default) | `TestLDPInteropFRR` passes in the QEMU guest after a42919934f; a Hello with no configured transport address carries the LSR-ID Transport Address TLV |
| AC-24 | RSVP-TE x3 (refresh ratio, `newRSVPProducerLab`, `requirePush`) | A refresh period increase is limited per RFC 2205 Section 3.7 item 5; both MPLS egress lab fixtures pass with their prerequisites explicit |
| AC-25 | `TestDoHIgnoresEDNSUDPSize` | The DoH test drives the production handler, not its own `dns.HandlerFunc` |
| AC-26 | DHCPv6-PD from config | A test starts from config and asserts `pdRequested`/`pdLength` and the IAID `identify` derives reach the request |
| AC-27 | flowspec-firewall events | `handleEvent` decodes the ze-bgp envelope the daemon emits; a test feeds a real `update` event and a rule is installed |
| AC-28 | DHCP route removal metric (moved from `spec-typed-identity-keys`, 2026-11-14) | A lease that ends while the carrier is down removes its default route at the metric `handleLinkDown` moved it to; no `rtproto.Iface` default route of that lease is left in the kernel (QEMU integration) |
| AC-29 | vpp tunnel creates (moved from `spec-typed-identity-keys`, 2026-11-14) | Repeated applies of a config with GRE, IPIP and VXLAN interfaces create each VPP interface once; deleting one releases the index its ze name holds, and the name map holds no stale reverse entry |
| AC-30 | netlink monitor late event (moved from `spec-typed-identity-keys`, 2026-11-14) | After `RTM_DELLINK` and a new link on the same index, a route or address event still in flight for the old link carries no name of the new link |
| AC-31 | DHCPv4 client option overload, RFC2131-4.1-7 and RFC2131-4.1-15 (the client half of two `{partial}` rows, owned here from 2026-11-14) | A DHCPOFFER or DHCPACK whose options field carries option 52 with value 1, 2 or 3 is parsed with the options placed in `file`, in `sname`, or in both: the options field first, then `file`, then `sname`, and only the option 52 found in the options field opens a field. An option carried only in an overloaded field (the lease time, the router, the server identifier) reaches the lease the client installs. RFC 2131 Section 4.1: "The options in the 'options' field MUST be interpreted first, so that any 'option overload' options may be interpreted." and "The 'file' field MUST be interpreted next (if the 'option overload' option indicates that the 'file' field contains DHCP options), followed by the 'sname' field." Evidence: RFC-tagged units for both ids through the client's receive path (the decode `acquire` and `extend` read server replies through), positive and negative, each with its `./le rfc record` discrimination record; a QEMU integration test in which a server's overloaded reply yields a lease carrying the overloaded options; and the interop scenario `ai/rules/interop-and-goal-validation.md` requires, against a third-party DHCP server whose source is read and quoted for the overload it sends |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | AC |
|------|------|----|
| one red-first unit per AC | package of the producer | AC-1 to AC-22 |
| RFC-tagged units `RFC requirement: RFC2131-4.1-7` and `RFC requirement: RFC2131-4.1-15`, positive and negative, each with a discrimination record | `internal/plugins/iface/dhcp/` | AC-31 |

### Functional Tests

| Test | AC |
|------|----|
| QEMU integration tests (`./le test integration`) | AC-11, AC-12, AC-18, AC-20, AC-31 |
| DHCPv4 option overload interop scenario against a third-party server | AC-31 |

## Files to Modify
- The producer files under Current Behavior.

### Integration Checklist
- [ ] Each fix reached through config, a netlink event or a network request

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/features/interfaces.md` (AC-10 RA forwarding)

## Implementation Steps
1. Read each producer at HEAD; move a fixed row to "Rows found already fixed".
2. Red test, fix, green, page edit, journal cell closed with the commit.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `bound-wraps-before-it-refuses` | 2026-10-06 | `bindAllACLs` |
| `error-answered-with-another-errors-code` | 2026-10-19 | NTP `syncWorker.doSync` |
| `first-notification-drops-current-state` | 2026-10-21 | `bgpEnrichBuilder.Start` |
| `guard-added-to-one-half-of-a-pair` | 2026-08-10 | iface netlink routes |
| `stale-artifact-reused` | 2026-10-30 | `plugin/all` snapshots miss `dnsresolver` |
| `guard-added-to-one-half-of-a-pair` | 2026-08-12 | geodns TTL bounds |
| `guard-added-to-one-half-of-a-pair` | 2026-08-12 | dns negative answers |
| `guard-added-to-one-half-of-a-pair` | 2026-09-14 | policy-route `from/protocol` |
| `identity-default-hides-a-mapping` | 2026-08-22 | iface owned registries |
| `identity-default-hides-a-mapping` | 2026-08-23 | iface/ra doctor |
| `identity-default-hides-a-mapping` | 2026-08-23 | traffic tc backend |
| `identity-default-hides-a-mapping` | 2026-09-07 | `checkRAForwarding` |
| `kernel-refuses-what-the-installer-sends` | 2026-10-08 | MPLS egress pop via `lo` |
| `listener-never-binds-a-resource-that-appears-later` | 2026-10-15 | `bindListeners` |
| `memo-suppresses-a-change-it-cannot-see` | 2026-09-20 | sysrib `recomputeBest` |
| `one-members-bad-input-fails-every-member` | 2026-09-09 | firewall `ApplyAll` |
| `option-read-after-the-decision-it-governs` | 2026-09-05 | `resolve ping` |
| `queued-request-waits-for-an-unrelated-trigger` | 2026-10-19 | DHCPv6 `maintain` |
| `read-fails-on-a-fact-the-caller-does-not-need` | 2026-10-19 | `show interface <name> counters` |
| `read-fails-on-a-fact-the-caller-does-not-need` | 2026-10-19 | resolver `mac/match` |
| `reconciler-claims-state-another-owner-installed` | 2026-10-18 | `reconcileOnReadyWithJournal` |
| `silent-fall-through` | 2026-10-15 | netlink `(*monitor).run` |
| `state-event-emitted-without-comparing-state` | 2026-08-11 | iface monitor |
| `transient-failure-treated-as-fatal` | 2026-08-15 | traffic mirroring |
| `test-against-broken-path` | 2026-08-24 | `TestLDPInteropFRR` rerun owed |
| `zero-value-as-valid-answer` | 2026-09-21 | LDP `parseLDPConfig` zero `transport-address` |
| `setting-changed-faster-than-its-consumer-allows` | 2026-09-05 | rsvpte `adoptedRefreshPeriod` |
| `test-against-broken-path` | 2026-09-22 | `newRSVPProducerLab` loopback rp_filter |
| `test-against-broken-path` | 2026-09-22 | `requirePush` frame budget |
| `test-against-broken-path` | 2026-08-12 | dnsserver `TestDoHIgnoresEDNSUDPSize` |
| `claim-outlives-the-evidence-it-cites` | 2026-10-19 | `TestDHCPv6PDLengthRequestsDelegatedPrefix` starts at `dHCPConfig`, not config |
| `declared-format-contradicts-payload` | 2026-08-19 | flowspec-firewall `handleEvent` reads a flat envelope the daemon never emits |
| `library-default-left-in-place` | 2026-10-26 | dnsserver UDP listener keeps the 512-octet `UDPSize` default, no EDNS0 (RFC4035-3-1, RFC4035-3-2) |
| `validate-accepts-what-start-refuses` | 2026-10-27 | firewall `validateAction` self-jump, jump cycle, jump to base chain |
| `kernel-refuses-what-the-installer-sends` | 2026-10-27 | Ze kernel lacks `CONFIG_NF_FLOW_TABLE`; firewall flowtable refused |
| `test-against-broken-path` | 2026-10-27 | 12 `firewall-*.ci` assert `firewall config applied` under the fixture cmd |
| `key-omits-a-fact-the-builder-uses` | 2026-10-28 | geodns and as112 `listener_up` gauges labelled by IP without port |
| `rfc-extension-not-implemented` | 2026-10-28 | dnsserver `Authoritative` answers no OPT and no BADVERS (RFC 6891 Section 6.1.3) |
| `rfc-extension-not-implemented` | 2026-10-28 | dnsserver `Authoritative` answers a query with more than one OPT RR, no FORMERR (RFC 6891 Section 6.1.1) |
| `index-key-derived-from-mutable-field` | 2026-08-11 | AC-28: DHCPv4 teardown removes its default route at the configured metric after `handleLinkDown` moved it |
| `identifier-reused-after-its-owner-is-gone` | 2026-08-19 | AC-29: vpp `createGRETunnel`, `createIPIPTunnel`, `createVxlanTunnel` create on every apply |
| `identifier-reused-after-its-owner-is-gone` | 2026-08-18 | AC-30: netlink monitor names a late event for a deleted index after the index's new link |

## Rows found already fixed

| Class file | Date | Surface | Evidence |
|------------|------|---------|----------|
| none in this group at the 2026-10-23 sweep | - | - | - |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (red recorded per AC)
- [ ] Tests PASS
- [ ] Scoped runs: `./le job run go test ./internal/plugins/iface/... ./internal/component/iface/... ./internal/plugins/firewall/...`, `./le test integration`

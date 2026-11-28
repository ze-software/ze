# Spec: Network services one-off defects (DNS, DHCP server, TFTP, flow export, resolve ping)

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P1 |
| Release reason | Fixes remaining shipped defects in flow export BGP enrichment replay, DHCP/TFTP late-interface listener binding, DHCPv4 server range host-address validation, authoritative DNS EDNS0/BADVERS/FORMERR/UDPSize and listener_up port labels, DoH production-handler proof, and resolve ping source-family selection |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | verify |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The 2026-10-23 journal sweep placed 44 rows spanning multiple unrelated subsystem doors into this spec. A full HEAD audit on 2026-11-26 found that ten items are already fixed in `main`, one row (`adoptedRefreshPeriod`) is already owned by `plan/release/fixes/spec-rsvp-refresh-timing.md`, and the remaining open work spans four distinct subsystem scopes. Per `ai/META-DECISIONS.md` (splitting specs whose edits need many subsystem doors into narrower child specs), this spec now owns the network-services plugin and DNS harness defects (`ai/doors/system-plugins.md`, `ai/doors/core.md`, `ai/doors/commands-cli.md`), while the forwarding, firewall/policy-route, and routing-protocol defects move to three focused sibling specs listed under `## Work Not Done` with zero duplicate AC or journal ownership.

Specifically, this spec fixes eight open defects:
1. Flow export BGP enrichment (`internal/plugins/flowexport/enrichbgp.go`, `register.go`) requests no `ribevents.ReplayRequest` on start and declares no `Sends`, so enabling `enrichment bgp` via reload leaves pre-existing RIB prefixes unenriched until later best-path churn.
2. DHCP server (`internal/plugins/dhcpserver/register.go`) and TFTP server (`internal/plugins/tftpserver/config_tx.go`, `register.go`) bind `listen-interface` entries only at configure/reload time and never bind an interface that appears after boot.
3. DHCPv4 server range parsing (`internal/plugins/dhcpserver/config.go` `parseSingleRange`) accepts the subnet network address and (below `/31`) broadcast address, whereas `parseServerIdentifier` beside it refuses both via `isHostAddress`; rejecting invalid ranges at verify must preserve the "Ze always starts" start-time isolation invariant via `config.RegisterServiceBlock`.
4. `resolve ping` (`internal/component/ping/cmd/resolve.go` `handleResolvePing`) resolves the destination hostname with `probe.FamilyAny` before parsing the `source` option, so an IPv4 or IPv6 source never constrains hostname resolution family.
5. `TestDoHIgnoresEDNSUDPSize` (`internal/core/dnsserver/secure_test.go`) drives an ad-hoc `dns.HandlerFunc` instead of the production `dnsserver.Authoritative` wrapper.
6. Authoritative DNS listeners (`geodns`, `as112`, `internal/core/dnsserver/manager.go`) keep the 512-octet miekg/dns `UDPSize` default instead of setting an EDNS0 receive buffer size (`RFC4035-3-1`, `RFC4035-3-2`).
7. `geodns` and `as112` `listener_up` gauges (`internal/plugins/geodns/server.go`, `internal/plugins/as112/server.go`) label by IP alone (`listen.Addr().String()`) instead of `listen.String()` (IP and port), colliding when two listeners share an IP on different ports.
8. `dnsserver.Authoritative` (`internal/core/dnsserver/handler.go`) implements neither RFC 6891 Section 6.1.1 (FORMERR on more than one OPT RR; OPT echo on EDNS queries) nor Section 6.1.3 (BADVERS on EDNS version > 0), unlike `dnsserver.Recursive` beside it.

## Work Not Done

- Interface, netlink monitor, DHCPv4/v6 client, VPP tunnels, traffic tc backend, and kernel MPLS egress pop defects (`ai/doors/forwarding.md`): split into `plan/release/fixes/spec-network-services-1-iface-dataplane-defects.md` (owns AC-4, AC-7, AC-10, AC-11, AC-16, AC-17, AC-18, AC-20, AC-21, AC-22, AC-26, AC-28, AC-29, AC-30, AC-31 and their 17 open journal rows).
- Firewall VPP ACL 255-count wrap, policy-route `from/protocol` model test, policy-route `interface "*"` verify refusal, firewall jump-target validation, and 12 `test/firewall/firewall-*.ci` fixture expectations (`ai/doors/forwarding.md`, `ai/doors/le-tooling.md`): split into `plan/release/fixes/spec-network-services-2-firewall-policyroute-defects.md` (owns AC-1, AC-8, AC-14, jump validation, and the `.ci` expectation fix and their 5 open journal rows).
- LDP `TestLDPInteropFRR` rerun, LDP default `transport-address`, and RSVP-TE MPLS egress lab fixture fixes (`ai/doors/routing-protocols.md`): split into `plan/release/fixes/spec-network-services-3-ldp-rsvpte-defects.md` (owns AC-23 and AC-24 lab rows and their 4 open journal rows).
- RSVP-TE refresh-period slew ratio (`adoptedRefreshPeriod`, `plan/journal/setting-changed-faster-than-its-consumer-allows.md` 2026-09-05): already owned by `plan/release/fixes/spec-rsvp-refresh-timing.md`; duplicate ownership removed here.
- Ten items already fixed in `main` (AC-2 NTP `doSync`, AC-5 `geodns` `parseSOA`, AC-6 DNS RFC 2308 negative SOA, AC-9 firewall ARP literal addresses, AC-13 `sysrib` `recomputeForwarding`, AC-19 gNMI `Get` encoding, AC-27 `flowspec-firewall` events, AC-33 firewall nft NAT placement, `plugin/all` `dnsresolver` snapshots, and `CONFIG_NF_FLOW_TABLE`): retired into `## Existing Mechanisms` and `## Rows found already fixed` with commit evidence.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/system-plugins.md`, `ai/doors/core.md`, `ai/doors/commands-cli.md`, `ai/doors/config-yang.md`
- [ ] `docs/architecture/core-design.md` (Design decisions: "Ze always starts")
- [ ] `docs/architecture/flowexport/flow-export-2-flow-records.md`
- [ ] `docs/architecture/provisioning/dhcp-server.md`
- [ ] `docs/architecture/provisioning/tftp-server.md`
- [ ] `docs/architecture/dns/server-harness.md`
- [ ] `docs/architecture/dns/geodns.md`
- [ ] `docs/architecture/dns/as112.md`
- [ ] `docs/architecture/resolve.md`
- [ ] `docs/architecture/plugin/plugin-system.md`, `docs/architecture/api/process-protocol.md`
- [ ] `ai/rules/platform-linux.md`, `ai/rules/user-facing-errors.md`, `ai/rules/interop-and-goal-validation.md`

### RFC Summaries
- [ ] `rfc/full/rfc6891.txt` Sections 6.1.1 and 6.1.3 (OPT RR count FORMERR, OPT echo, EDNS version BADVERS)
- [ ] `rfc/full/rfc4035.txt` Section 3 (EDNS0 UDP receive buffer size, RFC4035-3-1 and RFC4035-3-2)
- [ ] `rfc/full/rfc2131.txt` Section 4.1 (host address reachability for DHCPv4 ranges)

## Current Behavior (MANDATORY)

All producers read at HEAD on 2026-11-26:
- [ ] `internal/plugins/flowexport/enrichbgp.go` - `bgpEnrichBuilder.Start` subscribes to `ribevents.BestChange` and spawns the 5-second rebuild ticker, but never emits `ribevents.ReplayRequest`. `internal/plugins/flowexport/register.go` declares neither `registry.Registration.Sends` nor Stage 1 `sdk.Registration.Sends`.
- [ ] `internal/plugins/dhcpserver/register.go` - `dhcpServer.install` and `bindListeners` bind configured `listen-interface` devices once at boot/reload; `dhcpServerReceives` subscribes only to `dnssource` snapshots and never observes `interface:created` or `interface:up` events.
- [ ] `internal/plugins/tftpserver/config_tx.go` and `register.go` - `bindTFTP` binds once at boot/reload (comment explicitly notes: "a device that appears later is served only after the next reload"); `tftpserver` declares no `Receives` and registers no `p.OnEvent` handler.
- [ ] `internal/plugins/dhcpserver/config.go` - `parseSingleRange` checks only `prefix.Contains(start)`, `prefix.Contains(stop)`, and `addrToUint32(start) <= addrToUint32(stop)`; it never calls `isHostAddress(prefix, addr)` (`segment.go`), unlike `parseServerIdentifier` in the same file. `dhcpserver` has no `config.RegisterServiceBlock("service/dhcp-server", ...)` registration yet.
- [ ] `internal/component/ping/cmd/resolve.go` - `handleResolvePing` calls `probe.ResolveTarget(target, probe.FamilyAny)` on line 33 BEFORE the option-parsing loop on lines 44-90 parses `"source"`, so `opts.source` never constrains the resolution address family.
- [ ] `internal/core/dnsserver/secure_test.go` - `TestDoHIgnoresEDNSUDPSize` builds a bespoke inline `dns.HandlerFunc` (lines 577-587) that writes directly to `w.WriteMsg(m)` instead of wrapping an `AnswerFunc` through `dnsserver.Authoritative`.
- [ ] `internal/core/dnsserver/manager.go`, `internal/plugins/geodns/server.go`, `internal/plugins/as112/server.go` - `newServerManager` in both `geodns` and `as112` leaves `dnsserver.Options.UDPSize` at 0, which leaves miekg/dns's UDP read buffer at 512 octets so EDNS0 queries larger than 512 octets are cut short before reaching the handler.
- [ ] `internal/plugins/geodns/server.go` (`onListenerChange` line 394) and `internal/plugins/as112/server.go` (`listenerChanged` line 196) - both label `listenerUp` with `listen.Addr().String()` / `ip.String()`, omitting the port, even though `dnsserver.Options.OnListenerChange` already passes `netip.AddrPort` (and `as112`'s `upV4`/`upV6` map already keys by `listen.String()`).
- [ ] `internal/core/dnsserver/handler.go` - `Authoritative` checks only `r.Opcode != dns.OpcodeQuery` and never counts OPT RRs (no `FORMERR` for >1 OPT per RFC 6891 §6.1.1), never checks OPT version (no `BADVERS` for version != 0 per RFC 6891 §6.1.3), and never emits a reply OPT record on EDNS queries (whereas `internal/core/dnsserver/recursive.go` `Recursive` already enforces all three).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| RIB replay request emission and dual `Sends` declaration | `internal/plugins/flowspec-firewall/engine.go` `runEngine`, `register.go` `flowspecFirewallSends`; `internal/core/bgp/ribevents` `ReplayRequest` | Declares `ribevents.Namespace + ":" + ribevents.EventReplayRequest` in both `registry.Registration.Sends` and Stage 1 `sdk.Registration.Sends`, and emits `&replay.Request{ReplayID: replay.Broadcast}` on subscribe | Extends: AC-1 uses the exact same `ribevents.ReplayRequest.Emit` and shared `flowExportSends()` helper in `flowexport` |
| EventBus interface lifecycle events + `sdk.Plugin.OnEvent` | `internal/core/iface/events` `Namespace`, `EventCreated`, `EventUp`; `internal/plugins/dhcpserver/register.go` `dhcpServerReceives`, `followLearnedResolvers` | Emits `interface:created` and `interface:up` JSON payloads on the EventBus; plugins declare `Receives` in both `registry.Registration.Receives` and Stage 1 `sdk.Registration.Receives` and handle them in `OnEvent` (works identically in-process and forked) | Extends: AC-2 subscribes both `dhcpserver` and `tftpserver` to `interface:created` and `interface:up` through `Receives` + `OnEvent` to retry pending unbound `listen-interface` devices |
| DHCPv4 host-address check | `internal/plugins/dhcpserver/segment.go` `isHostAddress`, used by `parseServerIdentifier` in `config.go` | Reports whether an IPv4 address is neither the prefix's network address nor, below `/31`, its broadcast address | Extends: AC-3 applies `isHostAddress` in `parseSingleRange` (and rejects any interior network/broadcast span) |
| Start-time service-block isolation ("Ze always starts") | `internal/component/config/service_blocks.go` `RegisterServiceBlock`, `excludeInvalidServiceBlocks`; `internal/component/config/tree_validators.go` `RegisterScopedTreeValidator` | Drops an invalid registered service block at `ze start` (`LoadConfigForStart`) so the daemon starts with all healthy services, logs an ERROR under a doctor code, and reports the same code in `ze doctor`, while `ze config validate` and reload still refuse the commit | Extends: AC-3 registers `config.RegisterServiceBlock("service/dhcp-server", "DHCP server")` and a scoped validator + `ze doctor` check under `doctor-dhcp-range-non-host-address` so a bad range never aborts `ze start` |
| Shared target resolution with family constraint | `internal/core/probe` `ResolveTarget`, `FamilyIPv4`, `FamilyIPv6`, `FamilyAny`; `internal/component/traceroute/cmd/` (`spec-traceroute-source-af`) | Resolves a target hostname constrained to the source address's IP family | Extends: AC-4 parses `source` before `probe.ResolveTarget` in `handleResolvePing` and passes `FamilyIPv4`/`FamilyIPv6` when `source` is set |
| Recursive EDNS0 OPT/BADVERS/FORMERR + UDPSize | `internal/core/dnsserver/recursive.go` `Recursive`, `RecursiveUDPSize` (1232), `rejectRecursive`, `recursiveEDNSVersion`; `internal/core/dnsserver/manager.go` `Options.UDPSize` | Counts OPT RRs (>1 returns FORMERR), rejects OPT version > 0 with BADVERS + OPT, echoes OPT with advertised UDP size, and configures `dns.Server.UDPSize` | Extends: AC-6 and AC-8 reuse the same OPT-counting, BADVERS, OPT-echo, and `UDPSize` (1232) primitives on the `Authoritative` path and its `geodns`/`as112` callers |
| `dnsserver.Options.OnListenerChange` `netip.AddrPort` | `internal/core/dnsserver/manager.go` line 66; `internal/plugins/as112/server.go` line 201 (`listen.String()`) | Already passes `netip.AddrPort` (IP + port) to every listener-change callback | Extends: AC-7 switches `geodns` and `as112` `listener_up` gauge labels from IP-only to `listen.String()` and updates `docs/plugin-development/metrics.md` |
| Retired at HEAD: NTP unsynchronised `doSync` (old AC-2) | `internal/plugins/ntp/ntp.go` `serverUnsynchronized`, `lastErrorUnsynchronized` (commit `3e94cb7c40`) | Checks leap-indicator alarm before `Validate` and records `lastErrorUnsynchronized`; proven by `TestNTPUnsynchronisedNotKissOfDeath` and `ntp_chrony_docker_test.go` | Retired (already fixed at HEAD) |
| Retired at HEAD: `geodns` `parseSOA` minimum bounds (old AC-5) | `internal/plugins/geodns/config.go` `parseSOA` lines 381-388; `ze-geodns-conf.yang` lines 203-211 (commit `a2d55e74ff`) | Bounds `soa.Minimum > maxTTL` (`2147483647`) in both Go parser and YANG `range "0..2147483647"` | Retired (already fixed at HEAD) |
| Retired at HEAD: DNS negative answers RFC 2308 SOA (old AC-6) | `internal/plugins/geodns/server.go` `answerQuestions`; `internal/plugins/as112/zones.go` + `rfc2308_negative_soa_test.go` (commits `15d595b8c1`, `8f52d99804`) | Both `geodns` and `as112` include zone SOA in Authority for NXDOMAIN and NODATA and answer REFUSED with AA=0 out-of-zone | Retired (already fixed at HEAD) |
| Retired at HEAD: firewall ARP literal addresses (old AC-9) | `internal/component/firewall/validate.go` `validateMatch`, `ipHeaderMatchLeaf` (commit `6da2c99b2c`) | Refuses `MatchSourceAddress`, `MatchDestinationAddress`, and all IP-header matches in `FamilyARP` | Retired (already fixed at HEAD) |
| Retired at HEAD: `sysrib` `recomputeForwarding` no-op test (old AC-13) | `internal/component/sysrib/sysrib.go` lines 468-481 `installedPath.matches`; `forwarding_metadata_test.go` `TestPromotedPrimaryMetadataChangesReachFIB` (commit `0de6f1ffb9`) | Publishes proved primary-path promotions while leaving unproved withdrawal on the cascade | Retired (already fixed at HEAD) |
| Retired at HEAD: gNMI `Get` encoding refusal (old AC-19) | `internal/component/gnmi/get.go` `Server.Get`, `answerEncodingFor` (commit `238bdb62e9`) | Refuses unsupported encodings with `codes.Unimplemented` | Retired (already fixed at HEAD) |
| Retired at HEAD: `flowspec-firewall` events (old AC-27) | `internal/plugins/flowspec-firewall/engine.go` `ribevents.FlowSpecChanged`, `ribevents.ReplayRequest` (commits `161162baf7`, `4692d797cd`) | Subscribes to `ribevents.FlowSpecChanged` (`b.handleSelected`) and replays on start; proven by `rfc8955_selected_integration_linux_test.go` | Retired (already fixed at HEAD) |
| Retired at HEAD: nftables NAT placement (old AC-33) | `internal/component/firewall/validate.go` `validateNftNATAction`, `validateNftNATBaseChain` (commit `72d4616b2a`) | Refuses all 25 invalid NAT shapes at verify and installs all 18 valid controls in Ze's Linux 7.3 guest (`TestVerifyConfigNftNATPlacement`, `TestNftNATPlacementIntegration`, `test/firewall/firewall-nft-nat-placement.ci`) | Retired (already fixed at HEAD) |
| Retired at HEAD: `plugin/all` `dnsresolver` snapshots | `internal/component/plugin/all/testdata/{plugins,yang-providers,wire-methods}.snapshot` (commit `2249ea58cd`) | `dnsresolver` is included in all three snapshot files | Retired (already fixed at HEAD) |
| Retired at HEAD: `CONFIG_NF_FLOW_TABLE` kernel build | `gokrazy/kernel/kernel.config` (2026-10-27) | Flowtable kernel symbols built in and pinned; `applyTable` creates flowtables before rules | Retired (already fixed at HEAD) |

## Data Flow (MANDATORY)

### Entry Point
1. Config validate / reload / boot for `flow-export`, `service dhcp-server`, `service tftp-server`, `service geodns`, `service as112`.
2. EventBus delivery of `interface:created` and `interface:up` to `dhcpserver` and `tftpserver`, and `ribevents.BestChange` / `ribevents.ReplayRequest` between `flow-export` and `bgp-rib`.
3. Network DNS queries (UDP, TCP, DoT, DoH) arriving at `dnsserver.Authoritative` (`geodns`, `as112`).
4. Operator CLI/RPC invocation of `resolve ping <target> source <ip>`.

### Transformation Path
1. **Flow export replay (AC-1):** `bgpEnrichBuilder.Start(eb)` subscribes to `ribevents.BestChange`, emits `ribevents.ReplayRequest` (with `ReplayID: replay.Broadcast`, declared in both `registry.Registration.Sends` and Stage 1 `sdk.Registration.Sends`), receives the replayed best-path batch into `b.table`, and immediately triggers an initial radix-tree rebuild into `b.enricher` without waiting for the 5-second debounce tick.
2. **DHCP/TFTP late interface binding (AC-2):** At boot (`!strict`), interfaces that fail to bind are recorded as pending unbound `listen-interface` entries rather than forgotten. Both `dhcpserver` and `tftpserver` declare `interface:created` and `interface:up` in `Receives` (both `registry.Registration.Receives` and Stage 1 `sdk.Registration.Receives`) and handle them in `p.OnEvent`, serializing with `sdk.ConfigTx` / a server mutex to attempt binding any pending interface whose logical or resolved device matches; once bound, the listener starts serving and leaves the pending set, and any prior `doctor-dhcp-interface-unbound` / `doctor-tftp-interface-unbound` issue raised via `issuequeue` is cleared. Reload (`strict`) keeps its all-or-nothing transaction contract.
3. **DHCPv4 server range host-address check (AC-3):** `parseSingleRange` checks `isHostAddress(prefix, start)` and `isHostAddress(prefix, stop)` (for a contiguous range inside `prefix`, checking both endpoints guarantees no interior address is the network or broadcast address, because network is the minimum address and broadcast is the maximum address of `prefix`). `ze config validate` and reload (`OnConfigVerify`) refuse the config with an error naming the range, the address, why it names no host, and the fix command. At `ze start`, `config.RegisterServiceBlock("service/dhcp-server", "DHCP server")` + scoped tree validator drops only the invalid `service/dhcp-server` block so `ze` starts, logs an ERROR under `doctor-dhcp-range-non-host-address`, and `ze doctor` reports the same code.
4. **`resolve ping` source family (AC-4):** `handleResolvePing` validates `target` syntax first, runs the option-parsing loop to populate `opts.source`, `count`, `size`, and `df`, determines `family` from `opts.source` (`FamilyIPv4` if IPv4, `FamilyIPv6` if IPv6, `FamilyAny` if unset), and then calls `probe.ResolveTarget(target, family)`.
5. **Authoritative DNS EDNS0, BADVERS, FORMERR, UDPSize, DoH test, and `listener_up` labels (AC-5 to AC-8):**
   - `dnsserver.Authoritative` inspects `r.Extra` before the opcode check: counts OPT RRs (`*dns.OPT`); if count > 1, replies `RcodeFormatError` (`FORMERR`) with no OPT (RFC 6891 §6.1.1); if count == 1 and `opt.Version() != 0`, replies `RcodeBadVers` (`BADVERS`, extended RCODE 16) with one OPT carrying version 0 and `AuthoritativeUDPSize` (1232) (RFC 6891 §6.1.3). On normal EDNS queries (1 OPT, version 0), ensures the reply carries exactly one OPT advertising `AuthoritativeUDPSize` (1232) with DO clear, preserving any EDNS0 options (such as `geodns` RFC 7871 Client Subnet) added by `fn` without duplicating the OPT RR.
   - `geodns` and `as112` pass `UDPSize: dnsserver.AuthoritativeUDPSize` (1232) in `dnsserver.Options`, so UDP listeners read EDNS0 datagrams up to 1232 octets whole instead of truncating at miekg/dns's 512-octet default.
   - `geodns` (`onListenerChange`) and `as112` (`listenerChanged`) pass `listen.String()` (IP and port) as the `address` label to `listenerUp`.
   - `TestDoHIgnoresEDNSUDPSize` wraps its responder in `dnsserver.Authoritative(nil, ...)` so it exercises the production shaping and truncation path.

### Boundaries Crossed
- Process boundary: `tftpserver` can run forked; subscribing via EventBus `Receives` + `sdk.Plugin.OnEvent` and raising/clearing unbound-listener issues via `sdk.Plugin.RaiseIssue` / `ClearIssue` through `issuequeue` (never direct `internal/core/report`) preserves full forked-mode parity. `flowexport` declares `Sends` in both `registry.Registration.Sends` and Stage 1 `sdk.Registration.Sends`.
- Wire boundary: Authoritative DNS replies (`dnsserver.Authoritative`) gain RFC 6891 OPT / BADVERS / FORMERR handling and 1232-octet UDP receive buffer.

### Changed-Wire Census

**Search evidence:** Searched `internal/`, `pkg/`, `test/`, and `docs/` with `git grep` for `Authoritative(`, `UDPSize`, `listener_up`, `listenerUp`, `ze_geodns_listener_up`, `ze_as112_listener_up`, `ribevents.ReplayRequest`, `ribevents.EventReplayRequest`, `bgp-rib:replay-request`, and `ifaceevents.Event(Created|Up)`. Positive controls confirmed: `internal/core/dnsserver/recursive.go` (`RecursiveUDPSize = 1232`, `setReplyOPT`), `internal/plugins/flowspec-firewall/engine.go:142` (`ribevents.ReplayRequest.Emit`), `internal/component/bgp/plugins/rib/rib_event_sdk.go:315` (`EventReplayRequest` handler), `internal/plugins/iface/netlink/monitor_linux.go:379,386` and `internal/plugins/iface/vpp/monitor.go:165,381` (`ifaceevents.EventCreated`/`EventUp` emitters), and `docs/plugin-development/metrics.md:306` (`ze_dns_resolver_listener_up` `protocol, address` with port).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| DNS authoritative reply (UDP/TCP/DoT/DoH) | writer | `internal/core/dnsserver/handler.go` `Authoritative` (used by `internal/plugins/geodns/server.go:410` and `internal/plugins/as112/server.go:170`) and `internal/core/dnsserver/manager.go:230` (`UDPSize`) | `Authoritative` calls `shapeAuthoritative` without OPT/BADVERS/FORMERR handling and `geodns`/`as112` leave `Options.UDPSize` at 0 (512-octet default) | Change: add RFC 6891 §6.1.1 (>1 OPT -> `FORMERR`, single OPT echo at `AuthoritativeUDPSize = 1232` with DO=0) and §6.1.3 (OPT version > 0 -> `BADVERS`), preserve `geodns` RFC 7871 ECS option inside the single OPT RR, and pass `UDPSize: AuthoritativeUDPSize` in `geodns`/`as112` |
| DNS authoritative reply (UDP/TCP/DoT/DoH) | reader | External DNS resolvers/clients (`dig`, `unbound`), `internal/core/dnsserver/handler_test.go`, `internal/core/dnsserver/secure_test.go`, `test/interop/scenarios/dns-authoritative-edns0-unbound/` | Standard DNS wire readers (RFC 1035, RFC 6891, RFC 4035) plus in-repo authoritative DNS unit tests (`handler_test.go`, `rfc1035_handler_test.go`, `rfc4035_test.go`, `geodns/rfc7871_*_test.go`) | Preserve external wire compatibility and add/update unit and Unbound interop assertions proving `FORMERR` on 2 OPTs, `BADVERS` on version 1, single OPT echo with `UDPSize = 1232`, and >512-octet UDP query acceptance |
| EventBus `bgp-rib:replay-request` | writer | `internal/plugins/flowexport/enrichbgp.go` `bgpEnrichBuilder.Start` (declared in `internal/plugins/flowexport/register.go` `flowExportSends`); existing controls `internal/component/bgp/plugins/bmp/bmp_locrib.go:557` and `internal/plugins/flowspec-firewall/engine.go:142` | `bgpEnrichBuilder.Start` subscribes to `ribevents.BestChange` without emitting `ribevents.ReplayRequest`; `flowexport/register.go` declares no `Sends` | Preserve existing `ribevents.ReplayRequest` (`&replay.Request{ReplayID: replay.Broadcast}`) schema; add `flow-export` as a new emitter declared in both `registry.Registration.Sends` and Stage 1 `sdk.Registration.Sends` |
| EventBus `bgp-rib:replay-request` | reader | `internal/component/bgp/plugins/rib/rib_event_sdk.go:315` and `internal/component/bgp/plugins/rib/register.go:59,90` | Subscribes to `ribevents.Namespace + ":" + ribevents.EventReplayRequest` and answers with `ribevents.BestChange` replay batches | Preserve unchanged; existing reader already serves broadcast `ReplayRequest` events |
| EventBus `interface:created`, `interface:up` | writer | `internal/plugins/iface/netlink/monitor_linux.go:379,386` (`handleLinkUpdate`) and `internal/plugins/iface/vpp/monitor.go:165,381` | Both kernel and VPP interface monitors emit `ifaceevents.EventCreated` (`linkEventPayload`) and `ifaceevents.EventUp` (`stateEventPayload`) JSON payloads | Preserve unchanged |
| EventBus `interface:created`, `interface:up` | reader | `internal/plugins/dhcpserver/register.go` and `internal/plugins/tftpserver/register.go` via `Receives` + `sdk.Plugin.OnEvent` (existing controls `internal/component/iface/resolve.go:638-639` and `internal/component/ike/engine/dynamic_wan.go:198`) | Neither `dhcpserver` nor `tftpserver` currently declares `interface:created`/`interface:up` in `Receives` | Preserve payload format; add `dhcpserver` and `tftpserver` as subscribers in both `registry.Registration.Receives` and Stage 1 `sdk.Registration.Receives` |
| Prometheus metric `ze_geodns_listener_up`, `ze_as112_listener_up` | writer | `internal/plugins/geodns/server.go:394` (`onListenerChange`, defined in `geodns/metrics.go:40`) and `internal/plugins/as112/server.go:144,196` (`listenerChanged`, defined in `as112/metrics.go:38`) | Both currently pass bare IP (`listen.Addr().String()` / `ip.String()`) as the `address` label while `dnsresolver` (`docs/plugin-development/metrics.md:306`) publishes `ip:port` | Change `address` label value to `listen.String()` (`127.0.0.1:5300` / `[::1]:5300`) so two listeners on one IP with different ports publish distinct series |
| Prometheus metric `ze_geodns_listener_up`, `ze_as112_listener_up` | reader | Prometheus scrapers, `docs/plugin-development/metrics.md`, `internal/plugins/geodns/metrics_record_test.go:112`, `internal/plugins/geodns/metrics_test.go:43`, `internal/plugins/as112/server_test.go:234` | `git grep` found only those three test sites and `docs/plugin-development/metrics.md` referencing `ze_geodns_listener_up` / `ze_as112_listener_up` | Update unit tests and `docs/plugin-development/metrics.md` in the same commit to match `listen.String()` (`ip:port`) |

### Integration Points
- `internal/component/config/service_blocks.go` (`RegisterServiceBlock` for `service/dhcp-server`)
- `internal/component/plugin/issuequeue` (for non-blocking report-bus raises on late-bind failures)
- `internal/core/diagnostic` (`diagnostic.RegisterEventCode` for any new doctor/report-bus codes)
- `docs/plugin-development/metrics.md` (updated for `listener_up` `ip:port` label)

### Architectural Verification
- [ ] `flowexport` declares `bgp-rib:replay-request` in both `registry.Registration.Sends` and Stage 1 `sdk.Registration.Sends`
- [ ] `dhcpserver` and `tftpserver` declare `interface:created` and `interface:up` in both `registry.Registration.Receives` and Stage 1 `sdk.Registration.Receives`, and any report-bus raise uses `sdk.Plugin.RaiseIssue` via `issuequeue` with registered `diagnostic.RegisterEventCode`
- [ ] `dhcpserver` registers `config.RegisterServiceBlock("service/dhcp-server", "DHCP server")` so an invalid range never exits `ze start`
- [ ] `dnsserver.Authoritative` preserves `geodns` RFC 7871 Client Subnet echo inside the single reply OPT RR (never emits two OPT RRs)

## Risks & Assumptions

### Assumptions
- `geodns` currently calls `dnsserver.EchoSubnet` inside `answerQuery`, which appends an OPT carrying the ECS option with UDPSize 512. Updating `Authoritative` to manage the single OPT RR must merge/reuse any OPT `fn` populated (updating its UDPSize to `AuthoritativeUDPSize` 1232 and keeping DO=0) rather than appending a second OPT RR.
- `geodns` and `as112` tests that assert `listenerUp` with bare IP labels will be updated in the same commit with the label fix (not weakened: assertions become stricter by verifying exact IP and port).

### Risks
- Changing `dnsserver.Authoritative` to echo OPT on EDNS queries could affect existing RFC 1035/7871/4035 tests that expect zero Extra records when sending an EDNS query; audit all tests in `internal/core/dnsserver/`, `internal/plugins/geodns/`, and `internal/plugins/as112/` before editing.
- Late-interface binding in `dhcpserver` and `tftpserver` races with `sdk.ConfigTx` reload unless guarded by the server mutex so a late bind for an old config generation cannot attach after a reload removes that interface.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Reload enabling `flow-export enrichment bgp true` after routes exist in `bgp-rib` | → | `bgpEnrichBuilder.Start` emits `ribevents.ReplayRequest` and rebuilds radix tree | `TestFlowExportBGPEnrichmentReloadReplaysExistingRoutes` |
| Boot with absent `listen-interface`, then `interface:created`/`interface:up` event arrives | → | `dhcpserver` and `tftpserver` `OnEvent` late-bind handlers | `TestDHCPServerBindsLateAppearingInterface`, `TestTFTPServerBindsLateAppearingInterface` (plus QEMU integration proof) |
| `ze config validate` and `ze start` with DHCPv4 range containing subnet network or broadcast address | → | `parseSingleRange` `isHostAddress` check + `RegisterServiceBlock("service/dhcp-server", ...)` + `ze doctor` check | `TestDHCPServerRangeRefusesNetworkAndBroadcast`, `TestDHCPServerBadRangeExcludedAtStartAndReportedByDoctor` |
| `resolve ping <dual-stack-host> source 192.0.2.1` | → | `handleResolvePing` parses `source` before `probe.ResolveTarget` | `TestHandleResolvePingSourceConstrainsFamily` |
| Authoritative DNS query with >512-byte EDNS0 payload, EDNS version 1, or 2 OPT RRs | → | `dnsserver.Authoritative` + `dnsserver.Manager` `UDPSize: AuthoritativeUDPSize` | `TestAuthoritativeEDNS0OPTAndBADVERS`, `TestAuthoritativeTwoOPTReturnsFORMERR`, `TestAuthoritativeUDPReadsEDNS0Above512`, `test/interop/scenarios/dns-authoritative-edns0-unbound/` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Flow export enrichment replay (old AC-3): `flow-export enrichment bgp true` is enabled at boot or by reload after routes already exist in `bgp-rib` | `bgpEnrichBuilder.Start` emits `ribevents.ReplayRequest` (declared in both `registry.Registration.Sends` and Stage 1 `sdk.Registration.Sends`) and triggers an immediate initial radix-tree rebuild so pre-existing RIB routes enrich exported flows right away | The existing 5-second periodic rebuild ticker remains to batch subsequent incremental `ribevents.BestChange` churn after the initial replay populates the tree |
| AC-2 | DHCP and TFTP late-appearing listeners (old AC-12): a configured `listen-interface` is absent at boot (`!strict`), then appears via `interface:created` or `interface:up`, or is removed by a subsequent reload | Both `dhcpserver` and `tftpserver` start and serve all present interfaces, subscribe to `interface:created` and `interface:up` via `Receives` and `p.OnEvent` (identically in-process and forked for `tftpserver`), bind and serve the interface as soon as it appears, clear any raised `doctor-dhcp-interface-unbound` or `doctor-tftp-interface-unbound` issue, and cancel the pending bind entry when a reload removes that interface | Config reloads (`strict`) retain the all-or-nothing transaction contract and refuse configs naming an absent `listen-interface` |
| AC-3 | DHCPv4 server range host addresses (old AC-32): a configured DHCPv4 `range` has start, stop, or interior covering the subnet network address or (below `/31`) broadcast address, tested via `ze config validate`, reload, and `ze start` | `parseSingleRange` refuses the range via `isHostAddress` with an error naming the range, offending address, why it names a non-host address, and the fix; `ze config validate` and reload return that refusal, while `ze start` isolates only `service/dhcp-server` via `config.RegisterServiceBlock`, brings up all healthy services, logs an ERROR under `doctor-dhcp-range-non-host-address`, and reports the same code in `ze doctor` | Point-to-point `/31` and host `/32` prefixes remain permitted to use their valid host addresses per `isHostAddress` (`segment.go`) |
| AC-4 | `resolve ping` source family (old AC-15): `resolve ping <dual-stack-host>` is invoked with an IPv4 `source`, an IPv6 `source`, or with `source` omitted | `handleResolvePing` parses options before calling `probe.ResolveTarget`, passing `probe.FamilyIPv4` for an IPv4 `source`, `probe.FamilyIPv6` for an IPv6 `source`, and `probe.FamilyAny` when `source` is omitted | Omitting `source` remains permitted to resolve with `probe.FamilyAny` so existing dual-stack default behavior is preserved |
| AC-5 | `TestDoHIgnoresEDNSUDPSize` production handler (old AC-25): an oversized DoH reply is requested with a small client EDNS UDP payload size | `TestDoHIgnoresEDNSUDPSize` routes queries through the production `dnsserver.Authoritative` wrapper and returns the complete oversized DoH reply intact with `TC=0` | None for inline test-only handlers in `TestDoHIgnoresEDNSUDPSize`; DoH transport tests must drive the production `dnsserver.Authoritative` path |
| AC-6 | Authoritative DNS `UDPSize` > 512 (journal 2026-10-26): a valid EDNS0 query larger than 512 octets (up to 1232 octets) arrives over UDP at `geodns` or `as112` | `geodns` and `as112` configure `dnsserver.Options{UDPSize: dnsserver.AuthoritativeUDPSize}` (1232), and the authoritative UDP listener reads the full datagram and returns the DNS answer (`RFC4035-3-1`, `RFC4035-3-2`) | UDP datagrams larger than `AuthoritativeUDPSize` (1232 octets) remain bounded by the 1232-octet IPv6-safe buffer cap matching `RecursiveUDPSize` |
| AC-7 | `geodns` and `as112` `listener_up` IP+port labels (journal 2026-10-28): two listeners bind the same IP on different ports in `geodns` or `as112` | `geodns` (`onListenerChange`) and `as112` (`listenerChanged`) publish `ze_geodns_listener_up` and `ze_as112_listener_up` with `address` set to `listen.String()` (`ip:port`), producing two distinct gauge series, and `docs/plugin-development/metrics.md` documents the `ip:port` format | None; bare IP-only `address` labels on `listener_up` are replaced by `listen.String()` to match `ze_dns_resolver_listener_up` |
| AC-8 | `dnsserver.Authoritative` OPT / BADVERS / FORMERR (journal 2026-10-28 x2): authoritative queries arrive with >1 OPT RR, 1 OPT RR with EDNS version > 0, or 1 OPT RR with version 0 (including `geodns` RFC 7871 ECS) | `dnsserver.Authoritative` returns `FORMERR` with zero OPT RRs for >1 OPT (RFC 6891 §6.1.1), returns `BADVERS` with one OPT RR of version 0 advertising `AuthoritativeUDPSize` (1232) for EDNS version > 0 (RFC 6891 §6.1.3), and echoes exactly one OPT RR advertising `AuthoritativeUDPSize` (1232, DO=0) while preserving `geodns` ECS options on version 0 queries; proven by units and `test/interop/scenarios/dns-authoritative-edns0-unbound/` | Plain DNS queries carrying zero OPT RRs remain answered with zero OPT RRs per RFC 6891 §6.1.1 |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | AC |
|------|------|----|
| `TestFlowExportBGPEnrichmentReloadReplaysExistingRoutes` | `internal/plugins/flowexport/enrichbgp_test.go` | AC-1 |
| `TestFlowExportDeclaresReplayRequestInBothRegistrations` | `internal/plugins/flowexport/register_test.go` | AC-1 |
| `TestDHCPServerBindsLateAppearingInterface` | `internal/plugins/dhcpserver/listen_resolve_test.go` | AC-2 |
| `TestTFTPServerBindsLateAppearingInterface` | `internal/plugins/tftpserver/config_tx_test.go` | AC-2 |
| `TestDHCPServerRangeRefusesNetworkAndBroadcast` | `internal/plugins/dhcpserver/config_test.go` | AC-3 |
| `TestDHCPServerBadRangeExcludedAtStartAndReportedByDoctor` | `internal/plugins/dhcpserver/doctor_test.go` | AC-3 |
| `TestHandleResolvePingSourceConstrainsFamily` | `internal/component/ping/cmd/resolve_test.go` | AC-4 |
| `TestDoHIgnoresEDNSUDPSize` (updated to drive `Authoritative`) | `internal/core/dnsserver/secure_test.go` | AC-5 |
| `TestAuthoritativeUDPReadsEDNS0Above512` | `internal/core/dnsserver/handler_test.go` | AC-6 |
| `TestGeoDNSListenerUpIncludesPort`, `TestAS112ListenerUpIncludesPort` | `internal/plugins/geodns/server_test.go`, `internal/plugins/as112/server_test.go` | AC-7 |
| `TestAuthoritativeEDNS0OPTAndBADVERS`, `TestAuthoritativeTwoOPTReturnsFORMERR` | `internal/core/dnsserver/handler_test.go` | AC-8 |
| `TestGeoDNSECSPreservesSingleOPTRR` | `internal/plugins/geodns/server_test.go` | AC-8 |

### Functional Tests

| Test | AC |
|------|----|
| QEMU integration test (`//go:build linux`, `userns.Enter`): late-appearing veth bound by `dhcpserver` and `tftpserver` after boot | AC-2 |
| `./le test functional dhcp`, `./le test functional geodns`, `./le test functional as112` | AC-2, AC-3, AC-6, AC-7, AC-8 |
| Interop scenario `test/interop/scenarios/dns-authoritative-edns0-unbound/` (EDNS0 >512 UDP query, BADVERS on version 1, FORMERR on 2 OPTs, OPT echo) | AC-6, AC-8 |

## Files to Modify
- `internal/plugins/flowexport/enrichbgp.go`
- `internal/plugins/flowexport/register.go`
- `internal/plugins/dhcpserver/register.go`
- `internal/plugins/dhcpserver/config.go`
- `internal/plugins/dhcpserver/config_tx.go`
- `internal/plugins/dhcpserver/doctor.go`
- `internal/plugins/tftpserver/register.go`
- `internal/plugins/tftpserver/config_tx.go`
- `internal/component/ping/cmd/resolve.go`
- `internal/core/dnsserver/handler.go`
- `internal/core/dnsserver/manager.go`
- `internal/core/dnsserver/client.go`
- `internal/core/dnsserver/secure_test.go`
- `internal/plugins/geodns/server.go`
- `internal/plugins/as112/server.go`
- `docs/architecture/flowexport/flow-export-2-flow-records.md`
- `docs/architecture/provisioning/dhcp-server.md`
- `docs/architecture/provisioning/tftp-server.md`
- `docs/architecture/dns/server-harness.md`
- `docs/architecture/dns/geodns.md`
- `docs/architecture/dns/as112.md`
- `docs/plugin-development/metrics.md`

## Files to Create
- `test/interop/scenarios/dns-authoritative-edns0-unbound/` (scenario files for AC-6 and AC-8 interop proof)

### Integration Checklist
- [ ] Each fix reached through config validate, reload, boot, EventBus event, or live DNS/CLI request
- [ ] `./le plugin declarations check` and `./le plugin boundary check` pass after `Sends`/`Receives` edits
- [ ] `./le test unit pkg path` passes for every modified package under `-race`

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/architecture/flowexport/flow-export-2-flow-records.md` (document replay request on start)
- [ ] `docs/architecture/provisioning/dhcp-server.md` (document late-appearing interface binding, range host-address refusal, and start-time service-block isolation)
- [ ] `docs/architecture/provisioning/tftp-server.md` (document late-appearing interface binding via EventBus)
- [ ] `docs/architecture/dns/server-harness.md` (update `shapeAuthoritative` OPT/BADVERS/FORMERR table and `AuthoritativeUDPSize`)
- [ ] `docs/architecture/dns/geodns.md` and `docs/architecture/dns/as112.md` (update OPT UDPSize from 512 to 1232 and `listener_up` label format)
- [ ] `docs/plugin-development/metrics.md` (update `geodns` and `as112` `listener_up` address label format to `ip:port`)

## Implementation Steps
1. **AC-1 (flow export replay):** Add `flowExportSends()` returning `bgp-rib:replay-request` to both `registry.Registration.Sends` and Stage 1 `sdk.Registration.Sends` in `internal/plugins/flowexport/register.go`; emit `ribevents.ReplayRequest` in `bgpEnrichBuilder.Start` and trigger immediate rebuild on first replayed batch; prove red-to-green with unit tests.
2. **AC-2 (DHCP & TFTP late-appearing interfaces):** Add `interface:created` and `interface:up` to `Receives` (both `registry.Registration.Receives` and Stage 1 `sdk.Registration.Receives`) in `dhcpserver` and `tftpserver`, track pending unbound interfaces per config generation under server lock, bind on matching event in `OnEvent`, and raise/clear unbound-interface issues via `issuequeue` with registered `diagnostic.RegisterEventCode` codes. Prove with unit tests and Linux `userns.Enter` integration test.
3. **AC-3 (DHCPv4 range host-address check + "Ze always starts"):** Register `config.RegisterServiceBlock("service/dhcp-server", "DHCP server")` and scoped validator / doctor check under `doctor-dhcp-range-non-host-address`; enforce `isHostAddress` on range start and stop in `parseSingleRange` with operator-actionable error naming range, address, why, and fix; prove both verify refusal and start-time service-block isolation.
4. **AC-4 (`resolve ping` source family):** Reorder `handleResolvePing` to parse options before `probe.ResolveTarget` and derive family from `opts.source`; prove with unit tests.
5. **AC-5, AC-6, AC-7, AC-8 (DNS harness, `geodns`, `as112`):** Export `AuthoritativeUDPSize = 1232` in `internal/core/dnsserver`, wire it into `geodns` and `as112` `dnsserver.Options`, add RFC 6891 OPT counting (`FORMERR`), version check (`BADVERS`), and single-OPT reply shaping to `dnsserver.Authoritative` (preserving `geodns` ECS options), update `TestDoHIgnoresEDNSUDPSize` to drive `dnsserver.Authoritative` (with independent `ze-test-judge` approval since it edits an existing test), update `geodns` and `as112` `listener_up` gauge labels to `listen.String()`, update docs, and prove with units + `test/interop/scenarios/dns-authoritative-edns0-unbound/`.
6. Close each owned journal row in `plan/journal/` in the commit that lands its fix.

## Review Gate
- [ ] `/ze-review` clean on all modified Go, docs, and test files
- [ ] `/ze-review-spec` confirms every AC (AC-1 through AC-8), Changed-Wire Census row, and documentation update is met
- [ ] Independent `ze-test-judge` approval recorded if `TestDoHIgnoresEDNSUDPSize` or any existing `listener_up`/EDNS unit assertion changes

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads; the door covering `internal/component/ping/cmd/resolve.go` | Reach another unit through the boundary that survives a process split | agrees | `tftpserver` and `dhcpserver` subscribe to `interface:created` and `interface:up` over EventBus `Receives` + `OnEvent`, and `flowexport` declares `Sends` in both registrations |
| `ai/doors/system-plugins.md` | Boundaries | the door covering `internal/plugins/{flowexport,dhcpserver,tftpserver,geodns,as112}` | Reach the engine through the SDK, the bus, or a command, never through a Go call into a component's state | agrees | Late-interface binding uses EventBus `Receives` + `OnEvent` and report-bus issues go through SDK `RaiseIssue` via `issuequeue` |
| `ai/doors/core.md` | Boundaries | the door covering `internal/core/dnsserver/*` | Events register their payload type once. | agrees | Uses existing registered `ribevents.ReplayRequest` and `ifaceevents.EventCreated`/`EventUp`; adds no upward import to `internal/core/dnsserver` |
| `docs/architecture/flowexport/flow-export-0-umbrella.md` | In-process component with the SDK protocol | anchored to `internal/plugins/flowexport/register.go` | `flowexport` runs inside the daemon process, which is what makes the iface callback and the EventBus access direct calls. | agrees | AC-1 emits `ribevents.ReplayRequest` over the EventBus and declares it in both `registry.Registration.Sends` and Stage 1 `sdk.Registration.Sends` |
| `docs/architecture/flowexport/flow-export-1-counter-export.md` | IPFIX counter records use the Total IEs, not the Delta IEs | anchored to `internal/plugins/flowexport/register.go` | This page covers interface counter export (`interfaceCountersFrom`, IPFIX Total IEs, sFlow counter sequences), which this spec does not modify. | silent | AC-1 changes only BGP enrichment startup replay in `enrichbgp.go` and `Sends` registration in `register.go`; counter export is untouched |
| `docs/architecture/flowexport/flow-export-2-flow-records.md` | BGP enrichment reads the typed best-change event | the `// Design:` page of `internal/plugins/flowexport/enrichbgp.go`; anchored to `enrichbgp.go` and `register.go`; a page this spec modifies | Enrichment subscribes to `ribevents.BestChange` through the typed handle, not through a raw `bus.Subscribe` (`ai/rules/plugins.md`). | agrees | AC-1 extends `bgpEnrichBuilder.Start` to emit the existing typed `ribevents.ReplayRequest` handle and updates this section in the same commit |
| `docs/architecture/provisioning/dhcp-server.md` | Config reload | the `// Design:` page of `internal/plugins/dhcpserver/{register,config,doctor}.go`; anchored to `register.go`, `config.go`, `config_tx.go`, `doctor.go`; a page this spec modifies | An interface that appears later, by hotplug for example, is not bound until the next reload. | departs | AC-2 upgrades boot-absent interfaces to bind automatically on `interface:created`/`interface:up` events; this page is updated in the same commit to document the event-driven bind and range host-address check |
| `docs/architecture/iface/logical-name-resolution.md` | The plugin-facing registries take the resolved device too | anchored to `internal/plugins/dhcpserver/{register,config_tx,doctor}.go` | Each `listen-interface` is a logical name, so `listenInterface` resolves it through `ResolveDevice` before `SO_BINDTODEVICE`, and the request loop writes a reply's neighbor entry on the device that answered. | agrees | AC-2 resolves late-appearing interfaces through the exact same `listenInterface` / `iface.ResolveDevice` path and never falls back to an unresolved logical name |
| `docs/architecture/plugin/plugin-system.md` | The process boundary | anchored to `internal/plugins/dhcpserver/register.go` and `internal/plugins/tftpserver/register.go` | Plugin code reaches the engine through the SDK's RPC layer, which hides the transport difference. | agrees | AC-2 delivers interface appearance events to `tftpserver` and `dhcpserver` via EventBus `Receives` + `sdk.Plugin.OnEvent` and keeps `tftpserver` fork-safe |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/plugin/plugin-system.md` | With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration | agrees | Unchanged by this spec; `dhcpserver` and `tftpserver` keep `sdk.RunOrDeclare` pure so query-mode declaration starts no listeners or event loops |
| `docs/architecture/config/transaction-protocol.md` | 2. Transaction Phases | the `// Design:` page of `internal/plugins/dhcpserver/config_tx.go` and `internal/plugins/tftpserver/config_tx.go` | A config transaction has three runtime phases. Each phase emits stream events in the `config` namespace with a shared transaction ID. | agrees | Reloads keep the strict all-or-nothing verify/apply/rollback contract; only boot-lenient unbound interfaces retry on interface events |
| `docs/architecture/provisioning/tftp-server.md` | Config reload | the `// Design:` page of `internal/plugins/tftpserver/register.go`; anchored to `register.go` and `config_tx.go`; a page this spec modifies | An interface that appears later is not bound until the next reload. | departs | AC-2 upgrades boot-absent interfaces to bind automatically on `interface:created`/`interface:up` events; this page is updated in the same commit |
| `docs/architecture/resolve.md` | CLI | the `// Design:` page of `internal/component/ping/cmd/resolve.go` | This page documents the resolver caches and `ze resolve` subcommands; it is silent on `resolve ping` option-parsing order. | silent | `docs/architecture/resolve.md` does not specify the option-parse order of `handleResolvePing`; AC-4 only moves option parsing ahead of target resolution |
| `docs/architecture/dns/server-harness.md` | A recursive service gets its own wrapper | the `// Design:` page of `internal/core/dnsserver/{handler,manager,client}.go` and `internal/plugins/geodns/server.go`; a page this spec modifies | `Authoritative` emits no OPT at all, so it cannot carry BADVERS; its EDNS response handling is owed with the rest of its RFC 6891 work. | departs | AC-6 and AC-8 complete the owed RFC 6891 OPT/BADVERS/FORMERR handling and 1232-octet `UDPSize` on `Authoritative` and update this page in the same commit |
| `docs/architecture/dns/as112.md` | The response code says where the query name sits | anchored to `internal/core/dnsserver/handler.go`; the `// Design:` page of `internal/plugins/as112/server.go`; a page this spec modifies | RFC 2308 Section 3 requires the SOA in the authority section on both negative answers, and RFC 7535 Section 6 counts on that caching. | agrees | `as112` keeps its negative-answer policy intact, inherits `Authoritative`'s RFC 6891 OPT/BADVERS/FORMERR and 1232-octet `UDPSize`, and updates this page for `ip:port` `listener_up` labels |
| `docs/architecture/dns/lan-resolver.md` | Request path | anchored to `internal/core/dnsserver/handler.go` | This page covers the recursive `dnsresolver` plugin (`dnsserver.Recursive`), which this spec does not modify. | silent | AC-5 through AC-8 touch only `Authoritative`, `geodns`, `as112`, and `secure_test.go`; `Recursive` and `dnsresolver` are unchanged |
| `docs/architecture/dns/geodns.md` | Client subnet (RFC 7871) | anchored to `internal/core/dnsserver/{manager,client}.go`; the `// Design:` page of `internal/plugins/geodns/server.go`; a page this spec modifies | The OPT advertises 512 octets, the read buffer of an authoritative listener, version 0, with DO clear. geodns adds it from its answer function: the `Authoritative` wrapper itself still emits no OPT. | departs | AC-6 and AC-8 raise the authoritative read buffer and advertised OPT UDP size to 1232 and move single-OPT management into `Authoritative`; this page is updated in the same commit |
| `docs/architecture/dns/secure-transports.md` | One answer policy across four transports | anchored to `internal/core/dnsserver/manager.go` | With one handler behind all four transports, the `allow-from` decision, the client-IP selection and the authoritative and recursion guard are identical on cleartext UDP, cleartext TCP, DoT and DoH. | agrees | AC-5 makes `TestDoHIgnoresEDNSUDPSize` drive `dnsserver.Authoritative` so DoH and UDP/TCP/DoT share the real authoritative handler path |
| `docs/architecture/pki/tls-listeners.md` | Decision: dnsserver takes an injected resolver | anchored to `internal/core/dnsserver/manager.go`, `internal/plugins/geodns/server.go`, and `internal/plugins/as112/server.go` | This section covers PKI `TLSMaterialResolver` injection for DoT and DoH listeners, which this spec does not modify. | silent | AC-5 through AC-8 do not change TLS certificate loading, resolution, or rotation on `dnsserver`, `geodns`, or `as112` |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/pki/tls-listeners.md` | The looking glass may serve plain HTTP, because what it publishes is public, and one looking glass serves HTTP and HTTPS at the same time. Thomas's words, as voice-transcribed: "We should allow the looking glass to be HTTP. The information is public." and "We should be able to serve on both HTTP and HTTPS, not either." Asked where the choice lives and what happens to `ze.looking-glass.tls`, he answered "1a 2a": the choice is the `tls` leaf of each `server` entry, default true, and the environment variable is removed. The no-store rule applies to each entry: an entry that left `tls` unset serves HTTP with a warning on a daemon with no `database/` store and no named certificate, an entry that says `tls true` there is refused at start and at reload, and a reload does what a restart with the same config does | agrees | Orthogonal to this spec; no looking-glass or TLS listener protocol choice is touched |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/pki/tls-listeners.md` | The web and looking glass use the shared `listenerTLSMaterial` precedence. The approved Fleet reload design shares `managedCertificate` with startup because the daemon authority issues its unnamed leaf. | agrees | Orthogonal to this spec; no web, looking-glass, or managed certificate precedence is touched |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | decision of 2026-11-26 on `docs/architecture/pki/tls-listeners.md` | A certificate that cannot be presented never stops ze from starting. A web server whose certificate name does not resolve is not started. The looking glass is judged one `server` entry at a time: an entry that says `tls true` with no certificate to present stays down, every entry that asks for TLS stays down when the named certificate does not resolve, and the other entries serve. Each is reported with its fix by an error line in the log under a doctor code and by `ze doctor`. A reload whose config cannot be served stays refused. This supersedes the start half of the row of 2026-11-23 above, which says a `tls true` entry "is refused at start": at start that entry stays down and the others serve, and at reload the config is still refused. Thomas's words, as voice-transcribed: "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; "reload stay refused"; "Should only that endpoint stay down while the others serve: yes" | agrees | AC-3 applies the exact same "Ze always starts" principle to `service/dhcp-server` via `config.RegisterServiceBlock` + `doctor-dhcp-range-non-host-address` while leaving TLS listeners untouched |
| `docs/plugin-development/metrics.md` | Label Rules | a page this spec modifies | Use labels for runtime dimensions. Never encode variable data in metric names. | agrees | AC-7 updates `geodns` and `as112` `listener_up` gauge labels to include `ip:port` (matching `ze_dns_resolver_listener_up`) and updates this page in the same commit |
| `docs/architecture/core-design.md` | Design decisions | governs AC-3 (`dhcpserver` range refusal at verify vs start) | Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. | agrees | `service/dhcp-server` registers with `config.RegisterServiceBlock`; invalid ranges are refused by `ze config validate` and reload, and isolated at `ze start` under `doctor-dhcp-range-non-host-address` |

## Design presentation

### What changes
- `flow-export` requests a `bgp-rib` replay when `bgpEnrichBuilder.Start` starts and declares `bgp-rib:replay-request` in both `registry.Registration.Sends` and Stage 1 `sdk.Registration.Sends`.
- `dhcpserver` and `tftpserver` subscribe to `interface:created` and `interface:up` over the EventBus and bind boot-absent `listen-interface` devices when they appear, without changing strict reload failure semantics.
- `dhcpserver` refuses subnet network and broadcast addresses in `parseSingleRange` via `isHostAddress`, while isolating bad `service/dhcp-server` blocks at `ze start` through `config.RegisterServiceBlock` + `ze doctor` (`doctor-dhcp-range-non-host-address`).
- `resolve ping` parses `source` before resolving the destination hostname so the source IP family constrains DNS resolution.
- `dnsserver.Authoritative` enforces RFC 6891 §6.1.1 (>1 OPT -> `FORMERR`, single OPT echo at 1232 octets) and §6.1.3 (EDNS version > 0 -> `BADVERS`); `geodns` and `as112` set `UDPSize: AuthoritativeUDPSize` (1232) and label `listener_up` with `ip:port`; `TestDoHIgnoresEDNSUDPSize` drives `dnsserver.Authoritative`.

### Before and after
- **Before:** Turning `flow-export enrichment bgp true` on by reload leaves existing routes unenriched; DHCP/TFTP interfaces appearing after boot stay unbound until reload; DHCPv4 ranges accept `.0` / `.255` addresses; `resolve ping example.com source 2001:db8::1` can resolve an IPv4 A record and fail; `geodns`/`as112` drop >512-byte UDP EDNS queries, ignore >1 OPT and version > 0, and collapse multi-port `listener_up` gauges onto one IP label.
- **After:** Reload-enabled BGP enrichment fills its radix tree immediately; late-appearing DHCP/TFTP interfaces bind automatically on link events; non-host DHCPv4 range addresses are refused at validate/reload and isolated at boot under `doctor-dhcp-range-non-host-address`; `resolve ping` constrains DNS resolution to the source IP family; authoritative DNS answers RFC 6891 OPT/BADVERS/FORMERR with 1232-octet UDP buffers and `ip:port` `listener_up` gauges.

### Alternatives set aside
- **Calling `iface.Subscribe` directly in `tftpserver`:** set aside because `tftpserver` can run forked, where in-process `iface.Subscribe` receives nothing; EventBus `Receives` + `p.OnEvent` works identically in-process and forked.
- **Refusing `ze start` on an invalid DHCPv4 range:** set aside because the 2026-11-26 "Ze always starts" ruling forbids one bad service block from preventing `ze start` from bringing up networking; `config.RegisterServiceBlock` keeps validate/reload strict while isolating boot.
- **Raising `UDPSize` to 4096 on authoritative DNS:** set aside in favor of 1232 octets (`AuthoritativeUDPSize`, matching `RecursiveUDPSize` and DNS Flag Day 2020 IPv6/IPv4 fragmentation-safe MTU ceiling).

### Documents departed from
- `docs/architecture/provisioning/dhcp-server.md` ("Config reload") and `docs/architecture/provisioning/tftp-server.md` ("Config reload"): currently state that an interface appearing after boot is not bound until the next reload; updated in the same commit as AC-2.
- `docs/architecture/dns/server-harness.md` ("A recursive service gets its own wrapper") and `docs/architecture/dns/geodns.md` ("Client subnet (RFC 7871)"): currently record that `Authoritative` emits no OPT/BADVERS and keeps the 512-octet UDP read buffer as owed RFC 6891 work; updated in the same commit as AC-6 and AC-8.

### Choices for Thomas
none

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `first-notification-drops-current-state` | 2026-10-21 | `bgpEnrichBuilder.Start` |
| `listener-never-binds-a-resource-that-appears-later` | 2026-10-15 | `bindListeners` |
| `option-read-after-the-decision-it-governs` | 2026-09-05 | `resolve ping` |
| `test-against-broken-path` | 2026-08-12 | dnsserver `TestDoHIgnoresEDNSUDPSize` |
| `library-default-left-in-place` | 2026-10-26 | dnsserver UDP listener keeps the 512-octet `UDPSize` default, no EDNS0 (RFC4035-3-1, RFC4035-3-2) |
| `key-omits-a-fact-the-builder-uses` | 2026-10-28 | geodns and as112 `listener_up` gauges labelled by IP without port |
| `rfc-extension-not-implemented` | 2026-10-28 | dnsserver `Authoritative` answers no OPT and no BADVERS (RFC 6891 Section 6.1.3) |
| `rfc-extension-not-implemented` | 2026-10-28 | dnsserver `Authoritative` answers a query with more than one OPT RR, no FORMERR (RFC 6891 Section 6.1.1) |
| `guard-added-to-one-half-of-a-pair` | 2026-11-15 | AC-3: DHCPv4 server range may hold the subnet's network or broadcast address |

## Rows found already fixed

| Class file | Date | Surface | Evidence |
|------------|------|---------|----------|
| `error-answered-with-another-errors-code` | 2026-10-19 | NTP `syncWorker.doSync` | Fixed in `3e94cb7c40` (`internal/plugins/ntp/ntp.go` `serverUnsynchronized`, `lastErrorUnsynchronized`; `TestNTPUnsynchronisedNotKissOfDeath`) |
| `stale-artifact-reused` | 2026-10-30 | `plugin/all` snapshots miss `dnsresolver` | Fixed in `2249ea58cd` (`internal/component/plugin/all/testdata/{plugins,yang-providers,wire-methods}.snapshot`) |
| `guard-added-to-one-half-of-a-pair` | 2026-08-12 | geodns TTL bounds | Fixed in `a2d55e74ff` (`internal/plugins/geodns/config.go` `parseSOA` bounds `soa.Minimum > maxTTL`; `ze-geodns-conf.yang` `range "0..2147483647"`) |
| `guard-added-to-one-half-of-a-pair` | 2026-08-12 | dns negative answers | Fixed in `15d595b8c1`, `8f52d99804` (`internal/plugins/geodns/server.go` `answerQuestions`; `internal/plugins/as112/zones.go` + `rfc2308_negative_soa_test.go`) |
| `memo-suppresses-a-change-it-cannot-see` | 2026-09-20 | sysrib `recomputeBest` | Fixed in `0de6f1ffb9` (`internal/component/sysrib/sysrib.go` `recomputeForwarding` + `installedPath.matches`; `TestPromotedPrimaryMetadataChangesReachFIB`) |
| `declared-format-contradicts-payload` | 2026-08-19 | flowspec-firewall `handleEvent` | Fixed in `161162baf7`, `4692d797cd` (`internal/plugins/flowspec-firewall/engine.go` subscribes to `ribevents.FlowSpecChanged`; `rfc8955_selected_integration_linux_test.go`) |
| `kernel-refuses-what-the-installer-sends` | 2026-10-27 | Ze kernel lacks `CONFIG_NF_FLOW_TABLE` | Fixed 2026-10-27 (`gokrazy/kernel/kernel.config` pins flowtable symbols `=y`; `applyTable` adds flowtables before rules) |
| `validate-accepts-what-start-refuses` | 2026-11-15 | firewall NAT placement | Fixed in `72d4616b2a` (`internal/component/firewall/validate.go` `validateNftNATAction`, `validateNftNATBaseChain`; `TestNftNATPlacementIntegration`, `test/firewall/firewall-nft-nat-placement.ci`) |
| (old AC-9, no journal row) | - | firewall ARP literal addresses | Fixed in `6da2c99b2c` (`internal/component/firewall/validate.go` `validateMatch`, `ipHeaderMatchLeaf`) |
| (old AC-19, no journal row) | - | gNMI `Get` encoding | Fixed in `238bdb62e9` (`internal/component/gnmi/get.go` `Server.Get`, `answerEncodingFor`) |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (red recorded per AC)
- [ ] Tests PASS
- [ ] Scoped runs: `./le test unit pkg path ./internal/plugins/flowexport/`, `./le test unit pkg path ./internal/plugins/dhcpserver/`, `./le test unit pkg path ./internal/plugins/tftpserver/`, `./le test unit pkg path ./internal/component/ping/cmd/`, `./le test unit pkg path ./internal/core/dnsserver/`, `./le test unit pkg path ./internal/plugins/geodns/`, `./le test unit pkg path ./internal/plugins/as112/`, `./le test integration interop` (`dns-authoritative-edns0-unbound`)

# Spec: mtu-non-ipsec-tunnels

| Field | Value |
|-------|-------|
| Status | done |
| Scope | plugin |
| Priority | P2 |
| Release reason | Post-release WAN congestion diagnostics: extend `show mtu` to size non-IPsec tunnel interfaces (GRE, GRETap, IP6GRE, IP6GRETap, VXLAN, WireGuard, IPIP, SIT, IP6Tnl, IPIP6) alongside IPsec XFRM tunnels |
| RFC gaps | - |
| Depends | - |
| Phase | 4/4 |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Today `show mtu` discovers and sizes only IPsec tunnels reported by `ipsecinventory.Tunnels()` (bound to `xfrm` interfaces) and derives overhead only from ESP transforms (`deriveESPOverhead`). Operators running GRE, GRETap, IP6GRE, IP6GRETap, VXLAN, WireGuard, IPIP, SIT, or IP6Tnl/IPIP6 tunnels over WAN edges get `verdict: no-tunnels` (or no verdict at all when `!ze_ike`), with no endpoint path-MTU probe, no protocol overhead calculation, and no `set interface tunnel|wireguard <name> mtu <recommended>` remediation commands.

This spec extends `show mtu` with one streamlined, non-duplicated framing-overhead and sizing pipeline that discovers live non-IPsec tunnel and WireGuard interfaces through `iface`, probes their unicast remote endpoints (deduplicated alongside any IPsec peers), derives exact protocol overheads from live dataplane state, and emits unified tunnel rows, run verdicts, underlay advice, and Ze configuration commands.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/diagnostics/path-mtu.md` - Owns `show mtu` grammar, run flow, payload shape, arithmetic, verdicts, and underlay advice
  → Decision: `show mtu` answers one `ShapeDoc` document with kebab-case keys; keys present under a condition are absent otherwise, never zero; remediation commands are read-only strings for the operator to apply.
  → Constraint: `internal/component/mtu/cmd` imports neither the IKE engine nor any interface backend plugin; all interface reads go through `internal/component/iface` dispatch calls behind `mtuDeps`.
- [ ] `docs/features/interfaces.md` - Owns `iface.Backend`, tunnel creation/readback (`TunnelKind`, `TunnelSpec`), and WireGuard readback (`WireguardSpec`, `GetWireguardDevice`)
  → Decision: `iface.Backend` is the single pluggable dataplane seam (`kernel`, `vpp`, non-Linux stub); new readback methods follow `GetXFRMInfo` / `GetWireguardDevice` and refuse unsupported backends under exact-or-reject.
  → Constraint: Logical-to-OS interface name resolution runs through `iface.ResolveDevice` in `dispatch.go`.
- [ ] `ai/doors/system.md` and `ai/doors/forwarding.md` - Cross-component boundaries and compile-out rules
  → Decision: `ze_ike` and `ze_vpp` are compile-out gates; `mtu` must work with or without `ze_ike` linked and with either `kernel` or `vpp` iface backend.
  → Constraint: Never approximate an unknown encapsulation or outer header (exact-or-reject).

### RFC Summaries
- [ ] `rfc/short/rfc2784.md` - GRE base header (4 octets) and optional Checksum/Reserved1 (4 octets)
  → Constraint: Base GRE overhead is 4 octets; Checksum bit (`GRE_CSUM`) adds 4 octets.
- [ ] `rfc/short/rfc2890.md` - GRE Key (4 octets) and Sequence Number (4 octets) extensions
  → Constraint: `GRE_KEY` adds 4 octets and `GRE_SEQ` adds 4 octets, gated by the transmitted `OFlags` bits (not `IKey`/`OKey`, avoiding the vendored netlink `parseGretapData` IKey/OKey swap).
- [ ] `rfc/short/rfc2473.md` - IPv6 Tunneling (`ip6tnl`, `ipip6`, `ip6gre`, `ip6gretap`) and the 8-octet Tunnel Encapsulation Limit Destination Options header (Section 4.1.1)
  → Constraint: When `IP6_TNL_F_IGN_ENCAP_LIMIT` is not set, Linux emits an 8-octet Destination Options header carrying the Tunnel Encapsulation Limit TLV in addition to the 40-octet IPv6 header.
- [ ] `rfc/short/rfc2003.md` and `rfc/short/rfc4213.md` - IPIP (IPv4-in-IPv4, 20 octets) and SIT (IPv6-in-IPv4, 20 octets)
  → Constraint: Fixed 20-octet outer IPv4 header; IPIP carries inner IPv4, SIT carries inner IPv6.
- [ ] `rfc/short/rfc4303.md` - ESP framing (preserved unchanged through the unified `encapOverhead` struct)
  → Constraint: ESP trailer (2 octets) and block alignment (4/8/16 octets) remain exact for both tunnel and transport modes.

**Key insights:**
- `internal/component/mtu/cmd/verdict.go` already has `interfaceMTUCommand(kind, iface, mtu)` supporting `xfrm`, `tunnel`, and `wireguard`; only `tunnelCommand` hardcoded `"xfrm"`.
- Linux WireGuard (`drivers/net/wireguard/send.c`, `calculate_skb_padding`) clamps 16-byte plaintext padding at the device MTU, so a full-MTU packet incurs 0 padding octets on the wire; overhead at MTU is strictly 60 octets over IPv4 (`20 IPv4 + 8 UDP + 16 WG header + 16 Poly1305 tag`) and 80 octets over IPv6 (`40 IPv6 + 8 UDP + 32 WG`), matching Linux's 1420 default MTU on 1500-octet Ethernet.
- In `vendor/github.com/vishvananda/netlink/link_linux.go`, `parseGretapData` swaps `IFLA_GRE_OKEY` and `IFLA_GRE_IKEY`, and `Gretun.Type()` / `Gretap.Type()` test `Local.To4() == nil` (which misclassifies IPv4 GRE with `local-interface` / unset `Local` as `ip6gre`). `GetTunnelInfo` reads `OFlags` (`GRE_KEY`, `GRE_CSUM`, `GRE_SEQ`) instead of key values, checks `Remote` before `Local` for outer family, and reads `IFLA_GRE_FLAGS` via `RTM_GETLINK` for IPv6 GRE so `IP6_TNL_F_IGN_ENCAP_LIMIT` is exact.
- VPP's vendored GoVPP `binapi` already provides `GreTunnelDump`, `IpipTunnelDump`, and `VxlanTunnelV2Dump` keyed by `SwIfIndex`, so VPP `GetTunnelInfo` can read back live GRE, GRETap, IPIP, and VXLAN tunnels cleanly without approximations.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/mtu/cmd/mtu.go` - Parses `show mtu [host <address>] [exhaustive] [detail]`, validates `referenceAddress`, and dispatches `runMTU`.
- [ ] `internal/component/mtu/cmd/run.go` - `resolvePeerTargets` queries only `ipsecinventory.Tunnels()`; `sizeTunnels` runs only when `inventory == inventoryRegistered`, lists only `type == "xfrm"` links via `listXFRMInterfaces`, and sizes only IPsec Child SAs.
- [ ] `internal/component/mtu/cmd/overhead.go` - Defines `espOverhead` (`mode`, `outerHeader`, `fixed`, `block`) and `deriveESPOverhead`.
- [ ] `internal/component/mtu/cmd/arith.go` - Computes `ceiling`, `recommended`, `minimumMTU`, and `mss` using hardcoded `espTrailerOctets` (2) on `*espOverhead`.
- [ ] `internal/component/mtu/cmd/verdict.go` - Classifies tunnels (`classifyTunnel`), folds run verdict (`runVerdictOf`), advises underlay (`adviseUnderlay`), and renders `set interface <kind> <name> mtu <mtu>` (`interfaceMTUCommand`, with `tunnelCommand` hardcoding `"xfrm"`).
- [ ] `internal/component/mtu/yang/ze-mtu-cmd.yang` - Declares `show mtu` CLI/RPC schema with IPsec-only help/description text.
- [ ] `internal/component/iface/iface.go`, `backend.go`, `dispatch.go`, `tunnel.go`, `wireguard.go`, `discover.go` - Defines `Backend`, `TunnelKind`, `TunnelSpec`, `WireguardSpec`, `GetXFRMInfo`, `GetWireguardDevice` (on `Backend`, not yet wrapped in `dispatch.go`), and `CanonicalInterfaceType`.
- [ ] `internal/plugins/iface/netlink/show_linux.go`, `tunnel_linux.go`, `wireguard_linux.go`, `xfrm_linux.go`, `backend_other.go` - Implements Linux netlink interface listing (`ListInterfaces` populates `Addresses`), tunnel creation (`CreateTunnel`), WireGuard readback (`GetWireguardDevice`), and XFRM readback (`GetXFRMInfo`).
- [ ] `internal/plugins/iface/vpp/tunnel.go`, `vxlan.go`, `wireguard.go`, `query.go`, `ifacevpp.go` - Implements VPP tunnel creation, WireGuard readback (`GetWireguardDevice`), and `SwInterfaceDump` listing.

**Behavior to preserve:**
- `show mtu host <address> [exhaustive] [detail]` probes only that single address and never consults IPsec inventory or tunnel interfaces.
- Every existing key and value on IPsec `tunnels` rows (`peer`, `remote`, `interface`, `mode`, `encapsulation` [bool], `transform`, `path-mtu`, `assumed`, `current-mtu`, `ceiling`, `recommended`, `mss`, `verdict`, `octets`, `sized`, `reason`), `measurements` rows, `reference`, `underlay`, `commands`, `notes`, `caveats`, `status`, and `tcp-mtu-probing` stays backward compatible.
- When `ipsecinventory.Tunnels()` returns `ErrNotRegistered` and zero non-IPsec tunnels exist on the box, `inventory` is `not-registered`, `verdict` is absent, and the caution note saying no IPsec inventory is registered is emitted (`TestShowMTUNoInventoryIsNotZeroTunnels`).
- When `ipsecinventory.Tunnels()` is registered with zero IPsec tunnels and zero non-IPsec tunnels, `inventory` is `registered` and `verdict` is `no-tunnels` (`show-mtu-no-ipsec-component.ci`).
- `show mtu` remains strictly read-only on every code path; remediation commands are strings only.

**Behavior to change:**
- `show mtu` also discovers non-IPsec tunnel interfaces (`gre`, `gretap`, `ip6gre`, `ip6gretap`, `vxlan`, `ipip`, `sit`, `ip6tnl`, `ipip6`) and WireGuard interfaces (`wireguard`) via `iface`, probes their unicast remote endpoints (deduplicated with IPsec peer addresses), derives exact protocol overheads from live dataplane state, and includes them in `tunnels`, `verdict`, `underlay` advice, and `commands` (`set interface tunnel <name> mtu <rec>` and `set interface wireguard <name> mtu <rec>`).
- Even when `!ze_ike` (`inventory: not-registered`), if one or more non-IPsec tunnel interfaces exist, `show mtu` still measures, sizes, classifies, and emits `verdict` and `underlay` advice for them while still reporting `inventory: not-registered` and its caution note.
- Each `tunnels` row gains `kind` (`"ipsec"`, `"gre"`, `"gretap"`, `"ip6gre"`, `"ip6gretap"`, `"vxlan"`, `"wireguard"`, `"ipip"`, `"sit"`, `"ip6tnl"`, `"ipip6"`) and, when overhead was derived, `overhead` (fixed encapsulation octets before alignment/trailer).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| `mtu` overhead & sizing pipeline | `internal/component/mtu/cmd/overhead.go` (`espOverhead`, `deriveESPOverhead`), `arith.go` (`ceiling`, `recommended`, `mss`), `verdict.go` (`classifyTunnel`, `runVerdictOf`, `adviseUnderlay`, `interfaceMTUCommand`) | Derives ESP overhead, computes ceiling/recommended/MSS, classifies tunnel verdicts, folds run verdict, advises underlay, and formats `set interface <kind> <name> mtu <val>` | Extends: generalizes `espOverhead` into `encapOverhead` (adding `trailer` field alongside `block`) and adds `deriveTunnelOverhead` and `deriveWireguardOverhead` so one arithmetic, verdict, and command pipeline serves both IPsec and non-IPsec tunnels without duplication |
| `mtu` run orchestration & injection seam | `internal/component/mtu/cmd/run.go` (`runMTU`, `mtuDeps`, `liveDeps`) | Resolves targets from `ipsecinventory.Tunnels()`, deduplicates targets in `byTarget`, measures via ICMP/IKE, reads XFRM interfaces via `iface.ListInterfaces()` + `iface.GetXFRMInfo`, and builds payload | Extends: performs one `iface.ListInterfaces()` snapshot to supply XFRM, tunnel, and WireGuard interfaces through `mtuDeps`, probes non-IPsec unicast remotes through the same `byTarget` deduplicator, and sizes them in `sizeTunnels` |
| `iface.Backend` & `iface` dispatch | `internal/component/iface/backend.go` (`Backend.GetXFRMInfo`, `Backend.GetWireguardDevice`), `dispatch.go` (`GetXFRMInfo`), `discover.go` (`CanonicalInterfaceType`), `internal/plugins/iface/vpp/query.go` (`detailsToInfo`, `ListInterfaces`) | Abstracts OS/VPP interface listing and per-kind readback (`GetXFRMInfo`, `GetWireguardDevice`) behind `Backend` and package-level dispatch; VPP's `detailsToInfo` currently emits `IF_API_TYPE_*` in `Type` and VPP native names in `Name` | Extends: adds `TunnelInfo` and `Backend.GetTunnelInfo` (kernel netlink, VPP GoVPP, non-Linux stub), adds `iface.GetTunnelInfo` and `iface.GetWireguardDevice` dispatch wrappers, and updates `vpp/query.go` to map VPP `InterfaceDevType` (`gre`, `ipip`, `vxlan`, `wireguard`) into canonical link types and resolve Ze logical `Name` via `b.names` (keeping VPP native name in `OsName`) so `CanonicalInterfaceType` and `show mtu` work uniformly on both `kernel` and `vpp` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | The core knows nothing, and everything registers | the door every change reads | "Always-on code MUST NOT import a gated package." | agrees | `internal/component/mtu/cmd` imports neither `ike/engine` (`ze_ike`) nor `iface/vpp` (`ze_vpp`); all reads cross `ipsecinventory`, `ikeprobe`, and `iface` dispatch |
| `ai/doors/forwarding.md` | Boundaries | the door covering `internal/component/iface/`, `internal/plugins/iface/`, `internal/component/mtu/` | "Interface names go through the shared resolver" | agrees | `iface.GetTunnelInfo` and `iface.GetWireguardDevice` in `dispatch.go` resolve names through `ResolveDevice`, matching `GetXFRMInfo`, and unsupported VPP tunnel kinds refuse under exact-or-reject |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `internal/test/fixture/` and `test/plugin/show-mtu-non-ipsec-tunnels.ci` | "A `.ci` that needs a Linux kernel feature carries `option=needs-linux`" | agrees | `test/plugin/show-mtu-non-ipsec-tunnels.ci` carries `option=needs-linux:caps=net-admin,net-raw` and uses a compiled Go fixture returning errors on failure |
| `docs/features/interfaces.md` | Backend Implementations | the `// Design:` page of `iface` and `iface/netlink`/`vpp` files; a page this spec modifies | "`err` means the method is implemented as a stub that rejects every call with a descriptive error. `real` means the method drives the underlying mechanism." | agrees | `Backend.GetTunnelInfo` is `real` on `kernel` (rtnetlink) and `vpp` (GoVPP dump APIs for GRE/GRETap/IPIP/VXLAN) and `err` on the non-Linux stub; the table in `docs/features/interfaces.md` is updated in the same change |
| `docs/features/interfaces.md` | Design decisions | decision of 2026-11-22 | "`ze init` claims every discovered device by default, including the ones another program owns. `ze init --confirm` asks about each discovered device and writes only the ones the operator confirms" | agrees | `show mtu` is read-only and does not touch `ze init` device claiming |
| `docs/features/interfaces.md` | Design decisions | decision of 2026-11-22 | "`ze init --confirm` takes only an explicit answer: `yes`, `y`, `no` or `n`. Anything else, an empty line included, is not accepted, and the same device is asked again" | agrees | `show mtu` does not alter `ze init --confirm` prompting |
| `docs/architecture/config/apply-ordering.md` | The design | anchored to `internal/component/iface/backend.go` | "A commit applies in this order:" | silent | `show mtu` is strictly read-only and `GetTunnelInfo` is a query method that never participates in config-apply ordering |
| `docs/architecture/iface/logical-name-resolution.md` | Where the resolver lives, and what it returns | anchored to `internal/component/iface/backend.go` and `dispatch.go` | "`Binding` is a pure value type: `Ifindex`, `OsName`, `OperMAC`, `PermMAC`, `MTU`, `State`. It is not a `netlink.Link` and not a `*net.Interface`, so a consumer does not couple to `vishvananda/netlink`." | agrees | `TunnelInfo` is a pure value type carrying `TunnelKind`, `netip.Addr`, booleans, and `uint16`/`uint32`, with zero netlink or GoVPP types; `dispatch.go` routes `GetTunnelInfo` and `GetWireguardDevice` through `ResolveDevice` |
| `docs/architecture/core-design.md` | 14. Interface Management | anchored to `internal/component/iface/backend.go` | "The `iface` component (`internal/component/iface/`) manages OS network interfaces through a pluggable backend architecture." | agrees | `GetTunnelInfo` extends `iface.Backend` and `dispatch.go` cleanly across kernel, VPP, and stub backends without coupling `mtu` to any backend plugin |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | This spec adds no kernel-only config block and implements `GetTunnelInfo` on both `kernel` and `vpp` backends |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | No backend-mismatch config verdict changes |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Firewall verification is untouched by this spec |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Uses existing `kernel` and `vpp` backend registrations with no new selector names |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | BGP export policy is untouched by this spec |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | BGP export policy is untouched by this spec |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | BGP export policy is untouched by this spec |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | BGP export policy is untouched by this spec |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | BGP policy is untouched by this spec |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | BGP policy is untouched by this spec |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Filter-chain inspection is untouched by this spec |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | `show mtu` is an on-demand diagnostic that never affects daemon startup or reload |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | No daemon startup or backend-mismatch behavior changes |
| `docs/architecture/diagnostics/path-mtu.md` | The run | the `// Design:` page of `mtu/cmd/run.go`; a page this spec modifies | "A default run asks `ipsecinventory.Tunnels()`: `ErrNotRegistered` is the" | departs | Default run also discovers non-IPsec tunnel and WireGuard interfaces from a single `iface.ListInterfaces()` snapshot, adds their valid unicast remote endpoints to the deduplicated peer target list, and sizes them alongside IPsec Child SAs (including when `inventory` is `not-registered`); updates `path-mtu.md` in the same change |
| `docs/architecture/diagnostics/path-mtu.md` | The payload | the `// Design:` page of `mtu/cmd/*.go`; a page this spec modifies | "on a default run with a registered inventory" | departs | Extends `show mtu` so a default run emits `verdict` when any non-IPsec tunnel exists even if IPsec `inventory` is `not-registered`, adds `kind` and `overhead` keys to `tunnels` rows, and updates `icmpCaveat` wording to mention GRE beside ESP and UDP; updates `path-mtu.md` in the same change |
| `docs/architecture/diagnostics/path-mtu.md` | The ESP overhead | the `// Design:` page of `mtu/cmd/overhead.go`; a page this spec modifies | "Every octet ESP adds to a packet is derived from the Child SA as the inventory" | departs | Generalizes `espOverhead` into `encapOverhead` (`trailer`, `block`) and adds `deriveTunnelOverhead` and `deriveWireguardOverhead` for GRE, GRETap, IP6GRE, IP6GRETap, VXLAN, WireGuard, IPIP, SIT, IP6Tnl, and IPIP6 from live dataplane state; updates `path-mtu.md` in the same change |
| `docs/architecture/diagnostics/path-mtu.md` | The arithmetic | the `// Design:` page of `mtu/cmd/arith.go`; a page this spec modifies | "tunnel mode: `alignDown(underlay - outer - fixed, block) - 2`" | departs | Generalizes `ceiling` and `recommended` to subtract `o.trailer` (2 for ESP, 0 for non-IPsec) with `o.block` (4/8/16 for ESP, 1 for non-IPsec), and omits `mss` on L2 tunnels (`vxlan`, `gretap`, `ip6gretap`) and unaddressed L3 tunnels while enforcing the 1280 floor; updates `path-mtu.md` in the same change |
| `docs/architecture/diagnostics/path-mtu.md` | The verdicts | the `// Design:` page of `mtu/cmd/verdict.go`; a page this spec modifies | "`set interface xfrm <name> mtu <recommended>`" | departs | Parameterizes tunnel remediation commands by interface kind: `set interface xfrm <name> mtu <recommended>` for IPsec, `set interface tunnel <name> mtu <recommended>` for GRE/GRETap/IP6GRE/IP6GRETap/VXLAN/IPIP/SIT/IP6Tnl/IPIP6, and `set interface wireguard <name> mtu <recommended>` for WireGuard; updates `path-mtu.md` in the same change |
| `docs/architecture/api/commands.md` | Query RPCs | a page this spec modifies | "`tunnels` (rows of `peer`, `remote`, `interface`, `mode`, `encapsulation`, `transform`, `path-mtu`, `assumed`, `current-mtu`, `ceiling`, `recommended`, `mss`, `verdict`, `octets`, `sized`, `reason`)" | departs | Adds additive `kind` and `overhead` fields to `tunnels` rows and includes non-IPsec tunnel and WireGuard rows; updates `docs/architecture/api/commands.md` in the same change |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | `show mtu` adds no new CLI keywords, flags, or selectors |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | `ze-mtu:show-mtu` keeps its `ShapeDoc` contract and overrides no generic pipe operator |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | No pipe operation changes |
| `docs/comparison.md` | Operations | a page this spec modifies | "IPsec tunnel MTU sizing from a live path measurement (`show mtu`)" | departs | Expands `show mtu` beyond IPsec to GRE, WireGuard, VXLAN, and IP-in-IP tunnels; updates the comparison row in `docs/comparison.md` in the same change |
| `docs/architecture/traffic/cos-dynamic.md` | Decisions | anchored to `internal/plugins/iface/vpp/query.go` | "The first use captures the interface backend's actual maps before a write." | silent | Dynamic CoS VLAN QoS maps are untouched; `vpp/query.go` only maps VPP `InterfaceDevType` for tunnels/WireGuard and resolves logical `Name` via `b.names` |
| `docs/research/vpp-deployment-reference.md` | Production Architecture (IPng AS8298) | the `// Design:` page of `internal/plugins/iface/vpp/query.go` | "IPng Networks runs VPP as the production forwarding plane on all 12 backbone routers across" | silent | VPP deployment reference covers VPP process supervision, startup.conf, and operational layout; `vpp/query.go` interface type/name normalization does not alter VPP startup or lifecycle |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "Ze refuses to run VPP unmanaged. When VPP is external, the operator accepts that Ze may terminate and restart it" | agrees | `show mtu` and `GetTunnelInfo` are read-only queries and do not change VPP supervision |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "Ze knows how to manage and restart an external VPP: through a systemd unit, or by starting the binary directly, as it does with the embedded runtime" | agrees | VPP lifecycle management is untouched |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "After a failure, when the VPP process is alive, forwarding continued, and Ze reconnects and can read the state it configured, Ze continues from where it is" | agrees | VPP failure recovery is untouched |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "When VPP could have crashed or stopped forwarding, Ze reflects it in its own model and makes sure IGP and BGP peers are updated" | agrees | VPP crash withdrawal is untouched |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "One timer: the alive test and the loss declaration use the same time, and a probe missed inside it is not a loss. This narrows the row "After a failure, when the VPP process is alive": Ze continues from where it is only through form (1) below, inside the timer, and a declared loss is never adopted" | agrees | VPP liveness timers are untouched |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "As soon as a loss is declared, Ze makes sure the process is terminal (kill through the retained pidfd, then wait for the exit proof), then restarts and reconfigures VPP. Ze is the only restarter: a systemd unit it drives does not restart VPP itself (Restart=no)" | agrees | VPP restart policy is untouched |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | ""Recovered" has exactly two forms. (1) Inside the single timer Ze reconnects to the same pidfd-proven process and a full readback of what it installed (interfaces, LCP pairs, owned routes, PPPoE sessions) matches: Ze continues, with no kill and no withdrawal. Any mismatch means form (1) failed. (2) A forced restart (kill through the retained pidfd, exit proof) followed by a correct reconfiguration, exactly as at boot. Routes are withdrawn only when neither form succeeds within the recovery deadline and the crash-loop limit" | agrees | VPP recovery forms are untouched |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "Blackholing during the restart is accepted. Routes are withdrawn (VPP-backed interfaces down in Ze's model, so IGP and BGP react) only when neither recovery form succeeds within the recovery deadline and the crash-loop limit. This narrows the row "When VPP could have crashed or stopped forwarding": Ze updates IGP and BGP peers only then" | agrees | VPP route withdrawal policy is untouched |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "systemd configuration: Ze cannot be sure that an OS update will not change the unit's settings, so it detects the settings and adapts. No unit policy is required: Ze reads the unit's effective settings each time it acts on VPP, and adapts its restart to them. This supersedes the second sentence of the row "As soon as a loss is declared" (Ze the only restarter, Restart=no) and its forbids clause "or leaving the restart to systemd or another supervisor"; correcting that row needs Thomas's decision-change" | agrees | VPP systemd adaptation is untouched |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "Two limits. The alive test and the loss declaration use the same time: a probe missed inside it is not a loss, and a failed alive test is the loss declaration. A second, separate limit then bounds the reconnect-and-verify attempt of form (1). When that limit expires without form (1), Ze makes the process terminal (kill through the retained pidfd, then wait for the exit proof), then restarts and reconfigures VPP. This supersedes the row "One timer" (its single timer, "a declared loss is never adopted" and its forbids clause "a second window after the declaration"), and narrows the row "As soon as a loss is declared": the kill follows the second limit, not the declaration" | agrees | VPP recovery timers are untouched |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "Form (1) adopts on the presence of data ("We should take the presence of data as a sign that the software is still running and reconciliation may be then happening in the background."). Inside the reconnect-and-verify limit, the same pidfd-proven process answering over the API with the data Ze configured is the sign it is alive: Ze adopts it, with no kill and no withdrawal, and reconciles in the background, as the fib-vpp replay and sweep and the iface reconcile do. A mismatch the background reconciliation finds is repaired, not treated as a loss. Form (2) stays a forced restart followed by a correct reconfiguration, exactly as at boot. This narrows the row "Recovered has exactly two forms": a full readback match is no longer required, and a mismatch no longer fails form (1)" | agrees | VPP adoption policy is untouched |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "A recovery deadline and a crash-loop limit (N restarts inside a window) bound recovery. Past either, Ze withdraws: VPP-backed interfaces go down in Ze's model, so IGP and BGP react, and ze doctor reports it. Blackholing while Ze restarts inside both limits is accepted. The default values are proposed at design review for Thomas's approval. This adopts the recovery deadline and the crash-loop limit that the rows "Recovered has exactly two forms" and "Blackholing during the restart is accepted" carried as proposals, and narrows the row "When VPP could have crashed or stopped forwarding": Ze updates IGP and BGP peers only past a limit" | agrees | VPP recovery deadline and crash-loop limit are untouched |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "After a withdrawal Ze keeps retrying recovery with a growing backoff. When VPP recovers, the VPP-backed interfaces come up in Ze's model and the routes return by themselves, and ze doctor reports the state throughout. This narrows the crash-loop limit: past it Ze withdraws and keeps restarting" | agrees | VPP recovery backoff is untouched |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "The VPP loss itself triggers no withdrawal: no VPP-backed interface goes down in Ze's model while Ze recovers. Ze's own protocol timers (BFD detection, OSPF and IS-IS dead intervals, BGP hold) keep running during recovery, as their RFCs say" | agrees | VPP loss timer behavior is untouched |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "A process on VPP's API socket whose pidfd does not resolve to the configured systemd unit is reported, never refused or killed ("We should report but not refuse if the customers have changed the software, for example. It may mean that we have an out of date configuration. We should make sure the customer updates it but not cause a routing outage."). ze doctor names the process, the configured unit, that the configuration may be out of date, and the exact fix, and Ze keeps working with that process" | agrees | VPP pidfd reporting is untouched |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operational RPC `ze-mtu:show-mtu` (`show mtu [exhaustive] [detail]`) entering `handleShowMTU` in `internal/component/mtu/cmd/mtu.go`.
- Format at entry: validated CLI tokens (`command.ValidatedArgs`), zero new flags or syntax.

### Transformation Path
1. `handleShowMTU` parses args, validates `referenceAddress` for non-host runs, and calls `runMTU`.
2. `mtuRun.resolvePeerTargets` reads IPsec tunnels from `deps.tunnels()` (`ipsecinventory.Tunnels`) AND reads interface tunnel snapshots once from `deps.tunnelSnapshot()` (backed in `liveDeps` by `iface.EnsureBackend`, one `iface.ListInterfaces()`, plus `iface.GetXFRMInfo`, `iface.GetTunnelInfo`, and `iface.GetWireguardDevice` for matching link kinds).
3. Every IPsec peer target, every non-IPsec tunnel unicast remote (`Remote.IsValid() && !Remote.IsUnspecified() && !Remote.IsMulticast()`), and every WireGuard peer unicast endpoint IP is added to `r.measurements` via `addTarget(addr, labelPeer)`, deduplicated by `netip.Addr`.
4. `readUnderlay`, `measureTargets`, and `measureReference` execute unchanged (only live IPsec SAs set `m.peer` and trigger `measureByIKE`; non-IPsec-only targets stay on `prober: icmp`).
5. `sizeTunnels` sizes IPsec tunnels first (using `encapOverhead` from `deriveESPOverhead`), then sizes each non-IPsec tunnel (using `deriveTunnelOverhead`) and each WireGuard interface (computing `deriveWireguardOverhead` across its endpointed peers and choosing the tightest ceiling). All three share `ceiling`, `recommended`, `mss`, `classifyTunnel`, `runVerdictOf`, and `tunnelCommand(kind, name, rec)`.
6. Inner family and `mss` derivation for non-IPsec tunnels:
   - `ipip`, `ipip6`: `ipFamilyV4` (576-octet floor, IPv4 `mss = recommended - 40`).
   - `sit`, `ip6tnl`: `ipFamilyV6` (1280-octet floor, IPv6 `mss = recommended - 60`).
   - L2 Ethernet tunnels (`gretap`, `ip6gretap`, `vxlan`): `ipFamilyV6` floor (`1280`, RFC 8200 Section 5) and `mss` omitted (no single L3 IP header).
   - Dual-stack L3 tunnels (`gre`, `ip6gre`, `wireguard`): inspect live interface `Addresses` (ignoring IPv6 link-local `fe80::/10`) and, for WireGuard, all peers' `AllowedIPs`. If any non-link-local IPv6 address or IPv6 `AllowedIPs` prefix exists → `ipFamilyV6` (1280 floor, IPv6 `mss = recommended - 60`, conservative for dual-stack); else if any IPv4 address or IPv4 `AllowedIPs` prefix exists → `ipFamilyV4` (576 floor, IPv4 `mss = recommended - 40`); else (unaddressed/unnumbered L3 tunnel with no addresses and no `AllowedIPs`) → `ipFamilyV6` floor (`1280`) and `mss` omitted.
7. `adviseUnderlay`, `readLocalState`, and payload serialization build the single `ShapeDoc` JSON/YAML/table response.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `mtu/cmd` ↔ `iface` | Package-level dispatch (`EnsureBackend`, `ListInterfaces`, `GetLink`, `RouteLookup`, `GetXFRMInfo`, `GetTunnelInfo`, `GetWireguardDevice`) via `mtuDeps` | Yes |
| `iface` ↔ `iface/netlink` / `iface/vpp` | `iface.Backend` interface (`GetTunnelInfo`, `GetWireguardDevice`) with value types (`TunnelInfo`, `WireguardSpec`) | Yes |
| `mtu/cmd` ↔ CLI / API / REST / gRPC / MCP | Single `ze-mtu:show-mtu` `ShapeDoc` RPC payload | Yes |

### Changed-Wire Census

**Search evidence:** Searched `internal/`, `test/`, and `docs/` with `git grep -n 'ze-mtu:show-mtu\|fieldTunnels\|fieldInventory\|show mtu'` and inspected `internal/component/mtu/cmd/run.go`, `internal/test/fixture/plugin_fixture_show_mtu.go`, `docs/architecture/api/commands.md`, and `docs/architecture/diagnostics/path-mtu.md`. Positive control: `handleShowMTU` (`internal/component/mtu/cmd/mtu.go`) and `showMTUOversized` (`internal/test/fixture/plugin_fixture_show_mtu.go`) both encode/decode the `ze-mtu:show-mtu` JSON document.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `ze-mtu:show-mtu` `ShapeDoc` JSON payload | writer | `internal/component/mtu/cmd/run.go` (`runMTU`, `tunnelRow`, `sizeTunnel`, `sizeNonIPsecTunnel`, `sizeWireguardTunnel`) | `runMTU` builds the `map[string]any` returned as `plugin.Map(...)` by `handleShowMTU` | Additive: adds `kind` and `overhead` keys to `tunnels` rows and appends rows for non-IPsec tunnel and WireGuard interfaces; preserves all existing keys and types (`encapsulation` remains bool on IPsec rows) |
| `ze-mtu:show-mtu` `ShapeDoc` JSON payload | reader | `internal/test/fixture/plugin_fixture_show_mtu.go` (`showMTUOversized`, `showMTUNoTunnels`, `showMTUHost`, `showMTUJSON`, `showMTUIKEProbe`) | Unmarshals RPC response `data` map and asserts on `status`, `inventory`, `verdict`, `tunnels`, `commands`, `notes`, `caveats` | Preserved: additive keys do not break existing assertions; new fixture `showMTUNonIPsec` verifies non-IPsec tunnel rows and commands |
| `ze-mtu:show-mtu` `ShapeDoc` JSON payload | reader | External API clients (CLI `\| json`/`\| yaml`/`\| table`, REST, gRPC, MCP, operators) | `internal/component/mtu/cmd/register.go` registers `ze-mtu:show-mtu` as `ShapeDoc` on the command registry | Preserved: additive kebab-case keys documented in `docs/architecture/diagnostics/path-mtu.md`, `docs/architecture/api/commands.md`, and `docs/guide/command-reference.md` |

### Integration Points
- `internal/component/iface/backend.go` (`Backend`) — adds `GetTunnelInfo(name string) (TunnelInfo, error)`.
- `internal/component/iface/dispatch.go` — exposes `GetTunnelInfo(name string)` and `GetWireguardDevice(name string)` through `ResolveDevice`.
- `internal/plugins/iface/netlink/tunnel_linux.go` — implements `(*netlinkBackend).GetTunnelInfo`.
- `internal/plugins/iface/vpp/tunnel.go` — implements `(*vppBackendImpl).GetTunnelInfo`.
- `internal/plugins/iface/netlink/backend_other.go` — adds `(*stubBackend).GetTunnelInfo` returning `errNotSupported`.
- `internal/component/mtu/cmd/` (`overhead.go`, `arith.go`, `verdict.go`, `run.go`) — unifies overhead framing, discovers non-IPsec tunnels and WireGuard interfaces, sizes them, and emits remediation commands.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | `mtu/cmd` reads all interface and tunnel metadata exclusively through `iface` dispatch calls; never imports `netlink`, `wgctrl`, or `iface/vpp` |
| No unintended coupling (components stay isolated) | Yes | `mtu/cmd` still imports neither `ike/engine` nor any plugin under `internal/plugins/`; works with or without `ze_ike` and `ze_vpp` |
| No duplicated functionality (extends existing, does not recreate) | Yes | One `encapOverhead` + `ceiling`/`recommended`/`mss`/`classifyTunnel`/`runVerdictOf`/`adviseUnderlay`/`interfaceMTUCommand` pipeline serves IPsec, GRE, VXLAN, WireGuard, IPIP, SIT, and IP6Tnl/IPIP6; one `iface.ListInterfaces()` snapshot per run |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Overhead and sizing structs passed by pointer (`*encapOverhead`, `*tunnelSizing`, `*nonIPsecTunnel`); no hot-path byte allocations |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No new command, verb, or core-package switch; uses the existing registered `ze-mtu:show-mtu` handler and `iface.Backend` interface |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched `internal/component/iface/discover.go` (`kernelTunnelKinds`), `internal/component/iface/tunnel.go` (`tunnelKindNames`), and `internal/component/mtu/cmd/verdict.go` (`interfaceMTUCommand`); `mtu/cmd` classifies devices via existing `iface.CanonicalInterfaceType` and `iface.TunnelKind` rather than duplicating link-type lists |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Linux WireGuard caps 16-byte plaintext padding at the device MTU (`min(mtu, ALIGN(len, 16))`), so full-MTU packets incur 0 padding octets and fixed overhead is 60B (IPv4) / 80B (IPv6) | Linux `drivers/net/wireguard/send.c` (`calculate_skb_padding`), `docs/features/interfaces.md:621` (1420 default MTU on 1500 Ethernet) | Ceiling could be 1..15 octets off | `TestNonIPsecTunnelOverheadExact` & `TestShowMTUWireguardMultiPeerUsesTightestCeiling` | confirmed |
| A-2 | Vendored `vishvananda/netlink` `parseGretapData` swaps `IKey`/`OKey`, while `OFlags` (`GRE_KEY`, `GRE_CSUM`, `GRE_SEQ`) are parsed accurately from `IFLA_GRE_OFLAGS` | `vendor/github.com/vishvananda/netlink/link_linux.go:3370-3489` | GRETap key detection could misread `OKey` if read from key fields instead of `OFlags` | `TestGetTunnelInfoKernelRoundTrip/gretap-keyed` (`userns.Enter` real Linux kernel PASS) | confirmed |
| A-3 | Linux `ip6gre` and `ip6gretap` emit an 8-octet RFC 2473 Tunnel Encapsulation Limit Destination Options header unless `IP6_TNL_F_IGN_ENCAP_LIMIT` is set, and `IFLA_GRE_FLAGS` is readable via `RTM_GETLINK` | `/usr/include/linux/ip6_tunnel.h`, `rfc/short/rfc2473.md` Section 4.1.1, `vendor/github.com/vishvananda/netlink/nl/link_linux.go:269` | `ip6gre` ceiling could be off by 8 octets | `TestGetTunnelInfoKernelRoundTrip/ip6gre-default-encaplimit` and `/ip6gre-ign-encap-limit` (`userns.Enter` real Linux kernel PASS) | confirmed |
| A-4 | On a multi-peer WireGuard interface, sizing the single interface MTU from the tightest (lowest) peer ceiling prevents fragmentation to every peer without emitting conflicting commands | WireGuard has one device MTU shared across all peers on the netdev | Operator could be handed conflicting `set interface wireguard wg0 mtu ...` commands | `TestShowMTUWireguardMultiPeerUsesTightestCeiling` (PASS) | confirmed |
| A-5 | Adding `kind` and `overhead` to `tunnels` rows does not break any existing `show mtu` unit test, `.ci` fixture, or pipe renderer | Inspected `internal/component/mtu/cmd/run_test.go` and `internal/test/fixture/plugin_fixture_show_mtu.go` | Existing tests could reject unexpected keys | All `internal/component/mtu/cmd` unit and `userns.Enter` integration tests PASS | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Unmanaged point-to-multipoint GRE (`remote 0.0.0.0`/`::`), multicast/FDB-learning VXLAN, or responder-only WireGuard has no unicast remote to probe | Prober fails or sends ICMP to `0.0.0.0` or a multicast group | Validate `addr.IsValid() && !addr.IsUnspecified() && !addr.IsMulticast()` before adding any probe target; list the interface with `sized: false` and explicit `reason` |
| R-2 | Unmanaged GRE/IPIP/SIT/IP6Tnl uses kernel FOU/GUE UDP encapsulation (`EncapType != 0`) whose outer header varies by encap mode | Wrong overhead if silently ignored | Exact-or-reject: `deriveTunnelOverhead` refuses any non-zero `OuterEncapType` with `errOverheadRefused`, records a fault note, and leaves the row `sized: false` |
| R-3 | Reading `GetWireguardDevice` or `GetTunnelInfo` fails for one interface (for example missing `CAP_NET_ADMIN` for `wgctrl` or unsupported VPP kind) | Whole `show mtu` run aborts or hides the device | Isolate per-interface readback error: list the tunnel with `sized: false`, `reason` naming the error, and a fault note, while sizing all other tunnels normally |
| R-4 | Calling `iface.ListInterfaces()` fails or no iface backend is loaded in a unit test or non-Linux environment | Existing unit tests fail if `liveDeps` changes without fake defaults | Wire a single `tunnelSnapshot` seam on `mtuDeps` (defaulting to empty slices in `newFakeDeps`), and in production handle `ErrNoBackendLoaded` / list errors gracefully with a fault note |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | `show mtu` could miscalculate non-IPsec tunnel ceilings or fail to parse kernel tunnel attributes; strictly read-only diagnostic, zero impact on packet forwarding, routing sessions, or config commits |
| How is it reverted? | Single commit revert; additive payload fields (`kind`, `overhead`) and additive `iface.Backend.GetTunnelInfo` method have no persistent state or config migration |
| Who else touches this path? | `internal/component/iface/backend.go` and mock backends in `iface`, `iface/netlink`, and `iface/vpp` tests |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze-mtu:show-mtu` RPC (`handleShowMTU`) | → | `runMTU` → `resolvePeerTargets` → `sizeNonIPsecTunnel` / `sizeWireguardTunnel` → `deriveTunnelOverhead` / `deriveWireguardOverhead` | `TestShowMTUSizesNonIPsecTunnelsAndWireguard` |
| `iface.GetTunnelInfo` / `iface.GetWireguardDevice` | → | `netlinkBackend.GetTunnelInfo` / `netlinkBackend.GetWireguardDevice` | `TestGetTunnelInfoKernelRoundTrip` |
| `iface.GetTunnelInfo` (VPP) | → | `vppBackendImpl.GetTunnelInfo` | `TestVPPGetTunnelInfoRoundTrip` |
| Live daemon `show mtu` over clamped path with GRE + VXLAN + WireGuard | → | `handleShowMTU` → live `iface` kernel backend → ICMP prober → sized payload & commands | `test/plugin/show-mtu-non-ipsec-tunnels.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | GRE (`gre`), GRETap (`gretap`), IP6GRE (`ip6gre`), IP6GRETap (`ip6gretap`), VXLAN (`vxlan`), WireGuard (`wireguard`), IPIP (`ipip`), SIT (`sit`), IP6Tnl (`ip6tnl`), and IPIP6 (`ipip6`) encapsulation specs with known flags/families | `deriveTunnelOverhead` and `deriveWireguardOverhead` return exact hand-computed fixed overheads (GRE v4 base 24, +key 28, +csum+key+seq 36; GRETap v4 base 38, +key 42; IP6GRE default encap-limit 52, no-encap-limit 44, +key 56; IP6GRETap default 66, no-encap-limit 58; VXLAN v4 50, v6 70; WireGuard v4 60, v6 80; IPIP 20; SIT 20; IP6Tnl/IPIP6 default 48, ign-encap-limit 40), and `ceiling`/`recommended` produce exact values (`block = 1`, `trailer = 0`) | None: every modeled tunnel kind has one exact deterministic wire header size |
| AC-2 | Non-IPsec tunnel with non-zero `OuterEncapType` (FOU/GUE) or unknown `TunnelKind`, or WireGuard peer with invalid endpoint | Overhead derivation returns an error wrapping `errOverheadRefused`; `show mtu` records a fault note, sets `sized: false` with `reason`, emits no command for that tunnel, and still sizes all other tunnels | None: exact-or-reject forbids guessing an unmodeled outer encapsulation |
| AC-3 | Real Linux kernel netdevs created via `netlinkBackend.CreateTunnel` (`gre` unkeyed/keyed, `gretap` keyed, `ip6gre` default/encaplimit-none, `vxlan`, `ipip`, `sit`, `ip6tnl`, `ipip6`) | `iface.GetTunnelInfo` reads back exact `Kind`, `Local`, `Remote`, `GREKey`/`GREChecksum`/`GRESequence`, `IPv6EncapLimit`, `OuterEncapType`, and `VNI` (including `gretap` with key, proving immunity to vendored `IKey`/`OKey` swap) | None: all attributes are kernel-reported in the same netns |
| AC-4 | VPP backend (`vppBackendImpl.ListInterfaces` and `vppBackendImpl.GetTunnelInfo`) with GRE, GRETap, IPIP, VXLAN, and WireGuard interfaces | `ListInterfaces` resolves Ze logical `Name` via `b.names` (keeping VPP native name in `OsName`) and maps VPP `InterfaceDevType` (`gre`, `ipip`, `vxlan`, `wireguard`) so `iface.CanonicalInterfaceType` classifies them as `"tunnel"` / `"wireguard"`; `GetTunnelInfo` returns exact `TunnelInfo` (`Kind`, `Local`, `Remote`, `VNI`) from `GreTunnelDump`, `IpipTunnelDump`, and `VxlanTunnelV2Dump`, and returns an error for non-tunnel or missing interfaces | Non-VPP tunnel kinds (`ip6gre`, `sit`, `ip6tnl`) remain unsupported on VPP under exact-or-reject, matching `CreateTunnel` |
| AC-5 | `show mtu` run with oversized GRE (`gre0`, MTU 1476, IPv4-addressed) and VXLAN (`vxlan10`, MTU 1450) over a 1400-octet path | Probes each unique unicast remote once; outputs `tunnels` rows with `kind` (`"gre"`, `"vxlan"`), `overhead` (24/28, 50), `ceiling` (1376/1372, 1350), `recommended` (1344/1340, 1318), `verdict: "oversized"`, `octets` (exact excess), `sized: true`; includes IPv4 `mss` for IPv4-only GRE (`rec - 40`), IPv6 `mss` for IPv6/dual-stack GRE (`rec - 60`), and omits `mss` for L2 tunnels (`vxlan`, `gretap`, `ip6gretap`) and unaddressed L3 tunnels while enforcing the 1280 floor; emits `set interface tunnel gre0 mtu ...` and `set interface tunnel vxlan10 mtu 1318`; folds run `verdict: "action-needed"` and computes underlay advice | `prober` stays `"icmp"` with `icmpCaveat` because GRE and VXLAN have no authenticated in-band padded control exchange |
| AC-6 | `show mtu` run with a WireGuard interface (`wg0`, MTU 1420, IPv6/dual-stack `AllowedIPs`) having two peers: Peer 1 at IPv4 (1500 path → ceiling 1440) and Peer 2 at IPv6 (1400 path → ceiling 1320) | Probes both peer endpoints; emits ONE `tunnels` row for `wg0` sized to the tightest ceiling (`1320`, `recommended: 1288`, `mss: 1228`, `verdict: "oversized"`, `octets: 100`, `kind: "wireguard"`, `overhead: 80`), naming Peer 2 in `peer` and `remote`, and emits exactly ONE `set interface wireguard wg0 mtu 1288` command | Only peers with valid unicast `EndpointIP` are probed; endpoint-less peers are skipped unless zero peers have endpoints (in which case `sized: false` with clear `reason`) |
| AC-7 | `show mtu` run with a point-to-multipoint GRE (`0.0.0.0`), multicast VXLAN (`239.1.1.1`), `FlowBased` tunnel, endpoint-less WireGuard, or admin-down non-IPsec tunnel | Down tunnel gets `verdict: "down"`, `sized: false`, `reason: "interface <name> is down"`; multipoint/multicast/endpoint-less tunnel is NEVER probed, gets `sized: false` and explicit `reason`, and emits no command | None |
| AC-8 | `show mtu` run when `!ze_ike` (`ipsecinventory.ErrNotRegistered`): (a) with zero non-IPsec tunnels, and (b) with a GRE or WireGuard tunnel present | (a) Preserves exact existing behavior (`inventory: "not-registered"`, `verdict` absent, caution note `"no IPsec inventory is registered..."`); (b) reports `inventory: "not-registered"` with a caution note stating no IPsec inventory is registered while non-IPsec tunnels are still sized, AND sizes the non-IPsec tunnel(s), emits run `verdict`, underlay advice, and remediation command(s) | IPsec inventory remains `not-registered` in both cases because no IKE engine is linked |
| AC-9 | `show mtu` run with BOTH an IPsec XFRM tunnel and a GRE/WireGuard tunnel sharing or using distinct remotes, plus `show mtu host <address>` | Shared remote address is probed once; IPsec row retains all existing fields (`peer`, `remote`, `interface`, `mode`, `encapsulation` bool, `transform`, plus `kind: "ipsec"`, `overhead`); non-IPsec row carries `kind`, `interface`, `remote`, `overhead`, sizing fields, and `set interface tunnel|wireguard ...` command; `show mtu host <address>` skips both IPsec and non-IPsec tunnel discovery | None |
| AC-10 | End-to-end live Linux functional test (`test/plugin/show-mtu-non-ipsec-tunnels.ci`) over a 1400-octet clamped three-namespace path with configured GRE and VXLAN (and WireGuard when module present / separate test or netlink check) interfaces | Daemon creates the real tunnel interfaces, runs `show mtu` via RPC, measures 1400 via ICMP over the clamped path, and answers oversized verdicts with exact `ceiling`, `recommended`, and `set interface tunnel <name> mtu <rec>` commands | Runs under `option=needs-linux:caps=net-admin,net-raw` (matching existing `show-mtu-*.ci`), complemented by unprivileged `userns.Enter(t)` Go integration tests |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `ze show mtu` on a router with GRE (keyed) and VXLAN tunnels over a 1400-octet WAN clamp | `ze-mtu:show-mtu` → `handleShowMTU` → `iface.ListInterfaces` + `iface.GetTunnelInfo` → ICMP DF probe to remote → `deriveTunnelOverhead` → `ceiling`/`recommended`/`classifyTunnel` → `set interface tunnel <name> mtu <rec>` | `TestShowMTUSizesNonIPsecTunnelsAndWireguard`, `test/plugin/show-mtu-non-ipsec-tunnels.ci` |
| 2 | Runs `ze show mtu` on a router with a multi-peer WireGuard interface over mixed IPv4/IPv6 WAN paths | `ze-mtu:show-mtu` → `handleShowMTU` → `iface.ListInterfaces` + `iface.GetWireguardDevice` → ICMP DF probe to each peer endpoint → tightest `deriveWireguardOverhead` ceiling → `set interface wireguard <name> mtu <rec>` | `TestShowMTUWireguardMultiPeerUsesTightestCeiling` |
| 3 | Runs `ze show mtu` on a `!ze_ike` build that uses WireGuard or GRE without IPsec | `ze-mtu:show-mtu` → `ipsecinventory.ErrNotRegistered` + `iface` non-IPsec discovery → sizes non-IPsec tunnels and computes `verdict` + `underlay` advice | `TestShowMTUNonIPsecWorksWithoutIPsecInventory` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNonIPsecTunnelOverheadExact` | `internal/component/mtu/cmd/overhead_test.go` | AC-1: every tunnel kind (`gre`, `gretap`, `ip6gre`, `ip6gretap`, `vxlan`, `ipip`, `sit`, `ip6tnl`, `ipip6`, `wireguard`) across flags, encap-limit, and v4/v6 outer families, plus ceiling/recommended/MSS | PASS |
| `TestNonIPsecTunnelOverheadRefusesUnknownOrFOU` | `internal/component/mtu/cmd/overhead_test.go` | AC-2: refuses `OuterEncapType != 0` (FOU/GUE), `TunnelKindUnknown`, invalid remote, invalid WireGuard endpoint | PASS |
| `TestShowMTUSizesNonIPsecTunnelsAndWireguard` | `internal/component/mtu/cmd/run_test.go` | AC-5, AC-9, Wiring: end-to-end `handleShowMTU` over GRE, GRETap, VXLAN, WireGuard, and IPsec together, shared-remote dedup, `kind`/`overhead` fields, L2 `mss` omission, exact `set interface tunnel|wireguard|xfrm` commands | PASS |
| `TestShowMTUWireguardMultiPeerUsesTightestCeiling` | `internal/component/mtu/cmd/run_test.go` | AC-6: multi-peer WireGuard (IPv4 + IPv6 endpoints, differing path MTUs) selects tightest ceiling and emits one command | PASS |
| `TestShowMTUNonIPsecEdgeCasesNotProbedOrDown` | `internal/component/mtu/cmd/run_test.go` | AC-7: down tunnel, `0.0.0.0`/multicast remote, `FlowBased`, endpoint-less WireGuard, readback error isolation | PASS |
| `TestShowMTUNonIPsecWorksWithoutIPsecInventory` | `internal/component/mtu/cmd/run_test.go` | AC-8: `!ze_ike` with no tunnels vs with non-IPsec tunnels | PASS |
| `TestGetTunnelInfoKernelRoundTrip` | `internal/plugins/iface/netlink/tunnel_linux_test.go` | AC-3, Wiring (Linux `userns.Enter`): real kernel round-trip for `gre` (unkeyed & keyed), `gretap` (keyed), `ip6gre` (default & encaplimit-none), `vxlan`, `ipip`, `sit`, `ip6tnl`, `ipip6` | PASS |
| `TestVPPGetTunnelInfoRoundTrip` | `internal/plugins/iface/vpp/apply_test.go` | AC-4, Wiring: VPP `GetTunnelInfo` round-trip for GRE, GRETap, IPIP, VXLAN and error on absent/non-tunnel | PASS |
| `TestVPPListInterfacesClassifiesTunnelsAndWireguard` | `internal/plugins/iface/vpp/apply_test.go` | AC-4, Wiring: VPP `ListInterfaces` resolves Ze logical `Name` via `b.names` (keeping VPP native `OsName`) and maps `InterfaceDevType` (`gre`, `ipip`, `vxlan`, `wireguard`) so `iface.CanonicalInterfaceType` classifies them as `"tunnel"`/`"wireguard"` | PASS |
| `TestShowMTUNonIPsecClampedPathIntegration` | `internal/component/mtu/cmd/non_ipsec_integration_linux_test.go` | AC-1, AC-3, AC-5 (Linux `userns.Enter`): real kernel GRE + VXLAN links over 3-namespace 1400-octet clamped path through `handleShowMTU` | PASS |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| GRE/VXLAN/WG recommended MTU (inner IPv6 / L2 / dual-stack) | 1280..65535 | 1280 (e.g. VXLAN v4 on 1362 path → ceiling 1312 → rec 1280) | 1279 (1361 path → `errNoUsableMTU`, verdict `no-usable-mtu`, no command) | N/A (bounded by `uint16`) |
| IPIP/IPIP6/IPv4-only GRE/WG recommended MTU (inner IPv4) | 576..65535 | 576 (e.g. IPIP on 628 path → ceiling 608 → rec 576) | 575 (627 path → `errNoUsableMTU`) | N/A |
| `OuterEncapType` | 0 (none) | 0 | N/A (`uint16`) | 1 (FOU), 2 (GUE) refused with `errOverheadRefused` |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-mtu-non-ipsec-tunnels` | `test/plugin/show-mtu-non-ipsec-tunnels.ci` | Daemon with configured GRE (keyed) and VXLAN tunnels over the 1400-octet clamped path runs `show mtu` and reports exact oversized verdicts and `set interface tunnel <name> mtu <rec>` commands | |

### Interop Tests
N-A: `show mtu` is a local read-only diagnostic probing with standard ICMP echo; no protocol wire format or peer-facing handshake changes.

## Files to Modify
- `internal/component/iface/tunnel.go` - Add `TunnelInfo` value type
- `internal/component/iface/backend.go` - Add `GetTunnelInfo(name string) (TunnelInfo, error)` to `Backend`
- `internal/component/iface/dispatch.go` - Add `GetTunnelInfo` and `GetWireguardDevice` dispatch wrappers
- `internal/component/iface/config_test.go` - Add `GetTunnelInfo` stub on `fakeBackend`
- `internal/component/iface/migrate_linux_test.go` - Add `GetTunnelInfo` stub on `mockMigrateBackend`
- `internal/plugins/iface/netlink/tunnel_linux.go` - Implement `(*netlinkBackend).GetTunnelInfo` (including `RTM_GETLINK` `IFLA_GRE_FLAGS` read for `ip6gre`/`ip6gretap`)
- `internal/plugins/iface/netlink/backend_other.go` - Add `(*stubBackend).GetTunnelInfo` returning `errNotSupported`
- `internal/plugins/iface/netlink/tunnel_linux_test.go` - Add `TestGetTunnelInfoKernelRoundTrip` (`userns.Enter`)
- `internal/plugins/iface/vpp/tunnel.go` - Implement `(*vppBackendImpl).GetTunnelInfo` using `GreTunnelDump`, `IpipTunnelDump`, `VxlanTunnelV2Dump`
- `internal/plugins/iface/vpp/query.go` - Map VPP `InterfaceDevType` (`gre`, `ipip`, `vxlan`, `wireguard`) into canonical link types and resolve Ze logical `Name` via `b.names` (keeping VPP native name in `OsName`)
- `internal/plugins/iface/vpp/ifacevpp.go` - Fall back to `b.names.lookupNativeIndex` in `resolveIndex` so `GetTunnelInfo` and `GetWireguardDevice` resolve VPP native `OsName` after `ResolveDevice`
- `internal/plugins/iface/vpp/apply_test.go` - Add `TestVPPGetTunnelInfoRoundTrip` and `TestVPPListInterfacesClassifiesTunnelsAndWireguard`
- `internal/component/mtu/cmd/overhead.go` - Generalize `espOverhead` to `encapOverhead` (`trailer` field) and add `deriveTunnelOverhead` and `deriveWireguardOverhead`
- `internal/component/mtu/cmd/arith.go` - Compute `ceiling` and `recommended` from `*encapOverhead` (`o.trailer`, `o.block`)
- `internal/component/mtu/cmd/verdict.go` - Parameterize `tunnelCommand(kind, iface, mtu)` on interface kind (`xfrm`, `tunnel`, `wireguard`)
- `internal/component/mtu/cmd/run.go` - Single `iface.ListInterfaces()` snapshot for XFRM + non-IPsec tunnels + WireGuard, target discovery, multi-peer WireGuard tightest-ceiling sizing, `kind`/`overhead` row fields, `!ze_ike` non-IPsec support, updated `icmpCaveat`
- `internal/component/mtu/cmd/overhead_test.go` - Add `TestNonIPsecTunnelOverheadExact` and `TestNonIPsecTunnelOverheadRefusesUnknownOrFOU`
- `internal/component/mtu/cmd/arith_test.go` - Adapt to `encapOverhead` and add `trailer=0, block=1` cases
- `internal/component/mtu/cmd/verdict_test.go` - Test `tunnelCommand` for `xfrm`, `tunnel`, and `wireguard`
- `internal/component/mtu/cmd/run_test.go` - Add non-IPsec, WireGuard multi-peer, edge-case, and `!ze_ike` tests
- `internal/component/mtu/yang/ze-mtu-cmd.yang` - Update `show mtu` help/description to cover IPsec and non-IPsec tunnels (STE compliant)
- `internal/test/fixture/plugin_fixture_show_mtu.go` - Add `showMTUNonIPsec` fixture verifying GRE + VXLAN sizing over the clamped path
- `internal/test/fixture/register_show_mtu.go` - Register `plugin/show-mtu-non-ipsec`
- `docs/architecture/diagnostics/path-mtu.md` - Update to document non-IPsec tunnel discovery, overhead table, multi-peer WireGuard sizing, `kind`/`overhead` payload keys, and `set interface tunnel|wireguard` commands
- `docs/architecture/api/commands.md` - Update `ze-mtu:show-mtu` row with `kind` and `overhead`
- `docs/features/interfaces.md` - Document `Backend.GetTunnelInfo` in the backend matrix and tunnel section
- `docs/guide/command-reference.md` - Update `show mtu` description to include GRE, WireGuard, VXLAN, and IP-in-IP tunnels
- `docs/guide/command-catalogue.md` - Update `show mtu` row
- `docs/comparison.md` - Update `show mtu` comparison row
- `features/path-mtu-diagnostic-for-ipsec-tunnels.md` - Update summary and source anchors for non-IPsec tunnel sizing

## Files to Create
- `internal/component/mtu/cmd/non_ipsec_integration_linux_test.go` - Unprivileged `userns.Enter(t)` Linux integration test proving real GRE and VXLAN sizing over a clamped path
- `test/plugin/show-mtu-non-ipsec-tunnels.ci` - Functional `.ci` test driving `show mtu` over real GRE and VXLAN tunnels in a 3-namespace clamped topology
- `website/changes/fragments/mtu-non-ipsec-tunnels.md` - Weekly update news fragment (at closure)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/mtu/yang/ze-mtu-cmd.yang` description/help wording updated; no new leaves or keywords |
| YANG validation constraints | N-A | No new config or RPC input leaves |
| YANG custom validators | N-A | No new config or RPC input leaves |
| CLI commands/flags | N-A | Uses existing `show mtu [host <address>] [exhaustive] [detail]` grammar |
| CLI grammar (keyword before value) | N-A | No grammar tokens added or changed |
| Editor autocomplete | N-A | No new completions needed |
| Functional test for new RPC/API | Yes | `test/plugin/show-mtu-non-ipsec-tunnels.ci` + `internal/test/fixture/plugin_fixture_show_mtu.go` |
| Pipe completeness | Yes | Existing `ShapeDoc` `ze-mtu:show-mtu` payload rendered identically by `\| json`, `\| yaml`, `\| table` |
| Env var registration | N-A | No new env vars |
| Doctor check for runtime dependencies | N-A | Reuses existing `probe` ICMP socket check (`checkICMPProbeSocket`) and `mtu-local-state` check; no new file path, socket, kernel module, or binary dependency |
| Doctor code per problem class | N-A | All per-tunnel errors surface as structured `notes` (`severity: fault|caution`) and row `reason` fields in the `show mtu` response, with no new `Warn`/`Error` log lines or report-bus issues |
| Prometheus counters/metrics | N-A | `show mtu` is an on-demand active diagnostic, not a continuous metric exporter |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not a BGP family |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/path-mtu-diagnostic-for-ipsec-tunnels.md` |
| 2 | Config syntax changed? | No | No config leaves added or changed |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, `docs/guide/command-catalogue.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (`ze-mtu:show-mtu` row adds `kind`, `overhead`) |
| 5 | Plugin added/changed? | No | Existing `mtu` and `iface` components extended in place |
| 6 | Has a user guide page? | Yes | `docs/guide/command-reference.md` |
| 7 | Wire format changed? | No | No protocol wire format changes; only local diagnostic overhead math |
| 8 | Plugin SDK/protocol changed? | No | `pkg/` untouched |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC state-machine or wire-encoding behavior changes; `rfc/short/rfc2784.md`, `rfc2890.md`, `rfc2473.md`, `rfc2003.md`, `rfc4213.md`, `rfc4303.md` already exist and remain accurate |
| 10 | Test infrastructure changed? | No | Reuses existing `plugin/clamped-path` fixture and `test/plugin/` runner |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` (`show mtu` row) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/diagnostics/path-mtu.md`, `docs/features/interfaces.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata changes |
| 14 | Prometheus counters added/changed? | No | No metric changes |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Existing `ze-mtu:show-mtu` RPC registration unchanged |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived via `./le spec citation anchors`: updated in this spec: `docs/architecture/diagnostics/path-mtu.md`, `docs/features/interfaces.md`, `docs/architecture/api/commands.md`, `docs/comparison.md`, `docs/guide/command-reference.md`, `docs/guide/command-catalogue.md`, `features/path-mtu-diagnostic-for-ipsec-tunnels.md`. Unaffected declared/mentioned docs: `docs/research/vpp-deployment-reference.md` (declared by `vpp/query.go` for VPP startup/supervision reference only; interface type/name mapping in `detailsToInfo` does not alter deployment config), `docs/architecture/traffic/cos-dynamic.md` (mentions `vpp/query.go` for VLAN CoS map reads, unchanged), `docs/architecture/config/apply-ordering.md` (mentions `iface/backend.go` for commit apply ordering only), `docs/architecture/core-design.md` (mentions `iface/backend.go` Backend/RegisterBackend/LoadBackend), `docs/architecture/iface/logical-name-resolution.md` (mentions `iface/backend.go` and `dispatch.go` resolver behavior, unchanged), `docs/functional-tests.md` (mentions `plugin_fixture_show_mtu.go` fixture list; update if test count/summary lists fixtures), `docs/guide/ddos-mitigation.md` (mentions `iface.AddressIsLocal` in `dispatch.go`, unchanged), `docs/guide/ospf.md` (mentions `iface.LinkSpeedDuplex` / stub backend, unchanged), `docs/guide/vpp.md` (mentions `vpp/tunnel.go` `CreateTunnel`, unchanged) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/command-reference.md` (`show mtu` example and prose) |

## Design presentation

### What changes

`show mtu` now discovers and sizes every non-IPsec tunnel interface (`gre`, `gretap`, `ip6gre`, `ip6gretap`, `vxlan`, `ipip`, `sit`, `ip6tnl`, `ipip6`) and WireGuard interface (`wireguard`) alongside IPsec XFRM tunnels. For each interface it reads the live encapsulation parameters through a new `iface.GetTunnelInfo` backend readback (and existing `iface.GetWireguardDevice`), probes each unique unicast remote endpoint over ICMP DF (deduplicated with IPsec peers), derives exact protocol overheads from live kernel/VPP state through one unified `encapOverhead` pipeline, and emits tunnel rows (`kind`, `overhead`, `ceiling`, `recommended`, `verdict`, `octets`), run verdicts, underlay advice, and ready-to-paste `set interface tunnel <name> mtu <val>` / `set interface wireguard <name> mtu <val>` commands. Multi-peer WireGuard interfaces emit one row and one command sized to the tightest peer ceiling. Builds without `ze_ike` (`inventory: not-registered`) still measure and size non-IPsec tunnels whenever any exist.

### Before and after

1. **Router with keyed GRE and VXLAN over a 1400-octet WAN clamp (no IPsec configured):**
   - **Before:** `show mtu` reports `"inventory": "registered", "verdict": "no-tunnels", "tunnels": [], "commands": []`. Neither the GRE nor VXLAN remote endpoint is probed.
   - **After:** `show mtu` probes both remote endpoints (deduplicated), computes exact overhead (`28` for keyed GRE over IPv4, `50` for VXLAN over IPv4), reports both oversized rows in `"tunnels"` with `"kind": "gre"` / `"vxlan"`, `"ceiling": 1372` / `1350`, `"recommended": 1340` / `1318`, `"verdict": "action-needed"`, and lists `set interface tunnel gre0 mtu 1340` and `set interface tunnel vxlan10 mtu 1318` in `"commands"`.

2. **Router with a two-peer WireGuard interface `wg0` (MTU 1420; IPv4 peer on 1500 path, IPv6 peer on 1400 path):**
   - **Before:** `wg0` is invisible to `show mtu`.
   - **After:** `show mtu` probes both peer endpoints, finds Peer 2 over IPv6 is tightest (`1400 - 80 = 1320`, recommended `1288`), emits one row for `wg0` (`"kind": "wireguard"`, `"overhead": 80`, `"ceiling": 1320`, `"recommended": 1288`, `"verdict": "oversized"`, `"octets": 100`) naming Peer 2 in `"peer"` and `"remote"`, and emits one command `set interface wireguard wg0 mtu 1288`.

3. **Stripped build without `ze_ike` running GRE/WireGuard:**
   - **Before:** `show mtu` returns `"inventory": "not-registered"` with no `"verdict"` and empty `"tunnels"`, telling the operator no tunnel exists to size.
   - **After:** `show mtu` still reports `"inventory": "not-registered"` with a caution note that IPsec is absent, AND measures/sizes the non-IPsec tunnels, computes `"verdict"` and underlay advice, and lists their remediation commands. When no non-IPsec tunnel exists either, `"verdict"` remains absent as before.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Separate `deriveGREOverhead` / `ceilingNonIPsec` / `recommendedNonIPsec` arithmetic functions | Duplicates `ceiling`, `recommended`, `mss`, and `classifyTunnel`; adding a `trailer` field to one unified `encapOverhead` struct handles ESP (`trailer=2`, `block=4/8/16`) and all non-IPsec tunnels (`trailer=0`, `block=1`) in the exact same functions |
| Emit one `tunnels` row per WireGuard peer | A WireGuard netdev has only one interface MTU; emitting multiple rows for `wg0` would generate conflicting `set interface wireguard wg0 mtu <X>` commands for the same device |
| Read GRE/VXLAN/WireGuard endpoints from YANG config instead of live dataplane | Misses unmanaged kernel tunnels, misses dynamic WireGuard peer endpoints learned from handshakes, and departs from `show mtu`'s design of sizing from installed dataplane state |
| Guess 0 or 8 octets for `ip6gre` Tunnel Encapsulation Limit without reading kernel flags | An unmanaged `ip6gre` with `encaplimit none` differs by 8 octets from Linux's default `encap_limit=4`; reading `IFLA_GRE_FLAGS` via `RTM_GETLINK` in `ifacenetlink.GetTunnelInfo` yields the exact kernel value |
| Restrict to GRE, WireGuard, and VXLAN only while ignoring `ipip`, `sit`, `ip6tnl`, `ipip6` | `iface.TunnelKind` already models all 9 kinds in one enum; handling the remaining 4 IP-in-IP kinds adds ~15 lines in the same switch and avoids surprising gaps in `interface tunnel` coverage |

### Documents departed from

1. `docs/architecture/diagnostics/path-mtu.md` — sections `The run`, `The payload`, `The ESP overhead`, `The arithmetic`, and `The verdicts`:
   - `The run`: currently specifies default runs discover targets only from `ipsecinventory.Tunnels()` and size only XFRM-bound IPsec Child SAs. After this work, a default run also discovers non-IPsec tunnel and WireGuard interfaces from one `iface.ListInterfaces()` snapshot, adds their valid unicast remote endpoints to the deduplicated peer target list, and sizes them alongside IPsec Child SAs (including when IPsec `inventory` is `not-registered`).
   - `The payload`: currently states `verdict` is present only "on a default run with a registered inventory", lists `tunnels` rows without `kind`/`overhead`, and states `icmpCaveat` over ESP and UDP only. After this work, `verdict` is also present on an unregistered-IPsec run when non-IPsec tunnels exist, `tunnels` rows carry `kind` and `overhead`, and `icmpCaveat` notes ESP, GRE, or UDP.
   - `The ESP overhead`: currently defines only `espOverhead` and `deriveESPOverhead` from installed Child SAs. After this work, the section documents the unified `encapOverhead` plus `deriveTunnelOverhead` and `deriveWireguardOverhead` for GRE, GRETap, IP6GRE, IP6GRETap, VXLAN, WireGuard, IPIP, SIT, IP6Tnl, and IPIP6.
   - `The arithmetic`: currently binds `ceiling` and `recommended` to hardcoded `- 2` (`espTrailerOctets`) and specifies `MSS` is absent only when headers leave nothing. After this work, `ceiling` and `recommended` generalize over `o.trailer` (`2` for ESP, `0` for non-IPsec) and `o.block` (`4/8/16` for ESP, `1` for non-IPsec), and `mss` is omitted for L2 tunnels (`vxlan`, `gretap`, `ip6gretap`) and unaddressed L3 tunnels while enforcing the 1280-octet floor.
   - `The verdicts`: currently states tunnel remediation commands as `set interface xfrm <name> mtu <recommended>`. After this work, commands are parameterized by interface kind: `set interface xfrm <name> mtu <recommended>`, `set interface tunnel <name> mtu <recommended>`, and `set interface wireguard <name> mtu <recommended>`.
2. `docs/architecture/api/commands.md` — section `Query RPCs`: currently lists the `ze-mtu:show-mtu` `tunnels` row keys without `kind` and `overhead`. After this work, the row lists `kind` and `overhead` and notes coverage of IPsec and non-IPsec tunnels.
3. `docs/comparison.md` — section `Operations`: currently labels the row `IPsec tunnel MTU sizing from a live path measurement (show mtu)`. After this work, the row reflects IPsec, GRE, WireGuard, VXLAN, and IP-in-IP tunnel MTU sizing from live path measurement.

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Multi-peer WireGuard sizing | (a) (Recommended) One row per WireGuard interface sized to the tightest (lowest) peer ceiling, naming the controlling peer and emitting one `set interface wireguard <name> mtu <rec>` command; (b) One row per peer | (a) approved (2026-11-27) |
| C-2 | Scope of `interface tunnel` kinds | (a) (Recommended) Cover all 9 modeled `iface.TunnelKind` values (`gre`, `gretap`, `ip6gre`, `ip6gretap`, `vxlan`, `ipip`, `sit`, `ip6tnl`, `ipip6`) plus `wireguard` in one switch; (b) Only `gre`, `gretap`, `ip6gre`, `ip6gretap`, `vxlan`, and `wireguard` | (a) approved (2026-11-27) |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- Add `iface.TunnelInfo`, `Backend.GetTunnelInfo` (kernel, VPP, stub, mocks), `iface.GetTunnelInfo` and `iface.GetWireguardDevice` dispatch wrappers, generalize `espOverhead` → `encapOverhead` (`trailer` field) and `tunnelCommand(kind, iface, mtu)`, and wire the `mtuDeps.tunnelSnapshot` seam with failing wiring tests.
   - Tests: `TestGetTunnelInfoKernelRoundTrip`, `TestVPPGetTunnelInfoRoundTrip`, `TestShowMTUSizesNonIPsecTunnelsAndWireguard`
   - Files: `internal/component/iface/{tunnel,backend,dispatch}.go`, `internal/plugins/iface/netlink/{tunnel_linux,backend_other}.go`, `internal/plugins/iface/vpp/tunnel.go`, `internal/component/mtu/cmd/{overhead,arith,verdict,run}.go`
   - Verify: Package builds cleanly on linux and darwin; wiring tests compile and fail before full non-IPsec implementation.
2. **Phase: Kernel & VPP Tunnel Readback** -- Implement full `GetTunnelInfo` in Linux netlink (all 9 kinds, `OFlags` key/csum/seq, `Remote`-first family check, `RTM_GETLINK` `IFLA_GRE_FLAGS` encap-limit check) and VPP (`GreTunnelDump`, `IpipTunnelDump`, `VxlanTunnelV2Dump`), and pass real `userns.Enter` and VPP tests.
   - Tests: `TestGetTunnelInfoKernelRoundTrip`, `TestVPPGetTunnelInfoRoundTrip`
   - Files: `internal/plugins/iface/netlink/tunnel_linux.go`, `internal/plugins/iface/netlink/tunnel_linux_test.go`, `internal/plugins/iface/vpp/tunnel.go`, `internal/plugins/iface/vpp/apply_test.go`
   - Verify: Unit and `userns.Enter` kernel round-trip tests pass.
3. **Phase: Unified Overhead, Discovery, Sizing & Integration Tests** -- Implement `deriveTunnelOverhead`, `deriveWireguardOverhead`, single-pass non-IPsec + XFRM discovery, multi-peer WireGuard tightest-ceiling selection, L2 `mss` omission with 1280 floor, edge-case guards, `!ze_ike` non-IPsec support, unit tests, and `non_ipsec_integration_linux_test.go`.
   - Tests: `TestNonIPsecTunnelOverheadExact`, `TestNonIPsecTunnelOverheadRefusesUnknownOrFOU`, `TestShowMTUSizesNonIPsecTunnelsAndWireguard`, `TestShowMTUWireguardMultiPeerUsesTightestCeiling`, `TestShowMTUNonIPsecEdgeCasesNotProbedOrDown`, `TestShowMTUNonIPsecWorksWithoutIPsecInventory`, `TestShowMTUNonIPsecClampedPathIntegration`
   - Files: `internal/component/mtu/cmd/{overhead,arith,verdict,run}.go`, `internal/component/mtu/cmd/*_test.go`, `internal/component/mtu/cmd/non_ipsec_integration_linux_test.go`
   - Verify: All `mtu/cmd` unit and `userns.Enter` tests pass.
4. **Phase: Functional Test, YANG & Docs** -- Update `ze-mtu-cmd.yang`, add `showMTUNonIPsec` fixture and `test/plugin/show-mtu-non-ipsec-tunnels.ci`, and update all listed docs (STE compliant).
   - Tests: `test/plugin/show-mtu-non-ipsec-tunnels.ci`, `./le doc ste check`, `./le doc check all`, `./le cli grammar`
   - Files: `internal/component/mtu/yang/ze-mtu-cmd.yang`, `internal/test/fixture/{plugin_fixture_show_mtu,register_show_mtu}.go`, `test/plugin/show-mtu-non-ipsec-tunnels.ci`, docs

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-10 has implementation and passing test assertions |
| Feature completeness | Every user story 1..3 runs end-to-end through `handleShowMTU` |
| Correctness | Every protocol overhead matches RFC/Linux kernel exact byte count; `gretap` key uses `OFlags` not `IKey`/`OKey`; `ip6gre` reads `IFLA_GRE_FLAGS`; WireGuard multi-peer picks tightest ceiling; `0.0.0.0`/multicast remotes are never probed |
| Naming | All JSON keys kebab-case (`kind`, `overhead`, `current-mtu`, `path-mtu`, `no-usable-mtu`) |
| Data flow | Single `iface.ListInterfaces()` snapshot per run; `mtu/cmd` never imports `netlink`, `wgctrl`, or `iface/vpp` |
| Exact-or-reject | Unknown `TunnelKind` and non-zero `OuterEncapType` (FOU/GUE) refuse with `errOverheadRefused`, never default |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `iface.TunnelInfo` and `GetTunnelInfo` across kernel, VPP, and stub | `./le test unit pkg path ./internal/component/iface/ ./internal/plugins/iface/netlink/ ./internal/plugins/iface/vpp/` and `GOOS=darwin go vet` |
| Unified `encapOverhead` and non-IPsec sizing in `mtu/cmd` | `./le test unit pkg path ./internal/component/mtu/cmd/` (includes `userns.Enter` integration test on Linux) |
| Functional test `test/plugin/show-mtu-non-ipsec-tunnels.ci` | `./le test plugin` / `show-mtu-non-ipsec-tunnels.ci` (and/or unprivileged check) |
| Docs & YANG STE clean | `./le doc ste check` + `./le doc check all` + `./le cli grammar` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Secret redaction | `iface.GetWireguardDevice` returns `PrivateKey` and `PresharedKey` in `WireguardSpec`; `mtu/cmd` MUST NEVER copy, log, or emit keys in payload, notes, or errors (read only `PublicKey`, `EndpointIP`, `AllowedIPs`) |
| Input validation / bogon probing | Never open an ICMP prober to unspecified (`0.0.0.0`, `::`), multicast, or invalid addresses read from kernel tunnel/WireGuard state |
| Read-only guarantee | `show mtu` makes zero kernel/VPP/config mutations; remediation commands are plain strings |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read source / kernel behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- `vendor/github.com/vishvananda/netlink/link_linux.go:3374-3377` swaps `IFLA_GRE_OKEY` and `IFLA_GRE_IKEY` in `parseGretapData`, and `Gretun.Type()`/`Gretap.Type()` test `Local.To4() == nil` which fails when `Local` is unset (`local-interface`). Reading `OFlags` (`GRE_KEY`, `GRE_CSUM`, `GRE_SEQ`) and checking `Remote` before `Local` avoids both bugs without patching `vendor/`.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Unify `espOverhead` into `encapOverhead` (`trailer`, `block`) | Separate non-IPsec arithmetic functions | Zero duplication across `ceiling`, `recommended`, `mss`, `classifyTunnel`, `runVerdictOf`, `adviseUnderlay` |
| One row and command per WireGuard interface (tightest peer ceiling) | One row per WireGuard peer | One netdev has one MTU; avoids conflicting `set interface wireguard <name> mtu <X>` commands |
| Add `iface.Backend.GetTunnelInfo` + `iface.GetWireguardDevice` dispatch | Parse running YANG config | Sizes both managed and unmanaged tunnels, sees dynamic WireGuard endpoints, and honors `mtu/cmd` ↔ `iface` boundary |
| Support all 9 `iface.TunnelKind` values + WireGuard | Only GRE, VXLAN, WireGuard | Minimal extra code in one switch; covers every `interface tunnel` encapsulation Ze configures |
| Omit `mss` on L2 tunnels (`vxlan`, `gretap`, `ip6gretap`) while enforcing 1280 floor | Compute IPv4/IPv6 MSS on L2 tunnels | L2 tunnels carry arbitrary Ethernet frames (IPv4, IPv6, ARP); 1280 floor protects IPv6 over the bridge without fabricating a single L3 MSS |

## Known Limitations
- Non-IPsec tunnels (`gre`, `vxlan`, `wireguard`, `ipip`, `sit`, `ip6tnl`, `ipip6`) are measured via the ICMP DF prober (`prober: icmp`) and carry `icmpCaveat`, because unlike IKEv2 they have no authenticated in-band padded control-plane exchange.
- Unmanaged tunnels using kernel FOU/GUE UDP encapsulation (`OuterEncapType != 0`) are refused by name (`errOverheadRefused`) rather than guessed.
- Point-to-multipoint tunnels (`0.0.0.0`/`::`), multicast VXLAN, `FlowBased` tunnels, and WireGuard interfaces whose peers have no known endpoint cannot be probed and are listed with `sized: false` and a clear reason.

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
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: read-only local diagnostic)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/post-release/features/spec-mtu-non-ipsec-tunnels.md` only

---

## Implementation Summary

### What Was Implemented
- **`iface` tunnel readback & dispatch:** Added `iface.TunnelInfo` value type (`internal/component/iface/tunnel.go`), `Backend.GetTunnelInfo(name string) (TunnelInfo, error)` (`internal/component/iface/backend.go`), `iface.GetTunnelInfo` and `iface.GetWireguardDevice` dispatch wrappers routing through `ResolveDevice` (`internal/component/iface/dispatch.go`), and mock backend stubs (`config_test.go`, `migrate_linux_test.go`).
- **Linux kernel netlink readback:** Implemented `(*netlinkBackend).GetTunnelInfo` (`internal/plugins/iface/netlink/tunnel_linux.go`) covering all 9 `TunnelKind` values (`gre`, `gretap`, `ip6gre`, `ip6gretap`, `vxlan`, `ipip`, `sit`, `ip6tnl`, `ipip6`), reading GRE `OFlags` directly (immune to vendored `parseGretapData` `IKey`/`OKey` swap), preferring `Remote` over `Local` for outer family, and issuing one `RTM_GETLINK` to read `IFLA_GRE_FLAGS` (`IP6_TNL_F_IGN_ENCAP_LIMIT`) for IPv6 GRE. Added `(*stubBackend).GetTunnelInfo` returning `unsupported()` (`backend_other.go`).
- **VPP tunnel readback & classification:** Implemented `(*vppBackendImpl).GetTunnelInfo` (`internal/plugins/iface/vpp/tunnel.go`) via `GreTunnelDump`, `IpipTunnelDump`, and `VxlanTunnelV2Dump` with exact-or-reject checks (`vppGRETunnelKind`, `vppTunnelModeFlowBased`, `requireIPv4VPPEndpoints`, `vppAddrToNetip`). Updated `vpp/query.go` (`vppInterfaceType`, `ListInterfaces`) to map VPP `InterfaceDevType` (`gre`, `ipip`, `vxlan`, `wireguard`) into canonical link types and resolve Ze logical `Name` via `b.names` (keeping VPP native name in `OsName`), and `vpp/ifacevpp.go` (`resolveIndex`) to fall back to `b.names.lookupNativeIndex`.
- **Unified overhead, sizing, discovery & verdicts in `mtu/cmd`:** Generalized `espOverhead` to `encapOverhead` (`trailer`, `block`) in `overhead.go` and `arith.go`; added `deriveTunnelOverhead` (with `checkTunnelRemoteFamily` and `OuterEncapType != 0` refusal) and `deriveWireguardOverhead`; parameterized `tunnelCommand(kind, iface, mtu)` in `verdict.go`; unified single-pass `listTunnelSnapshot` (with dormant Linux fallback tunnel filtering and secret-free `wireguardPeersFromSpec`), shared-remote dedup, multi-peer WireGuard tightest-ceiling selection (preferring `!assumed` on equal-ceiling ties), `kind`/`overhead` payload fields, L2/unaddressed `mss` omission with 1280 floor, updated `icmpCaveat`, and `!ze_ike` non-IPsec sizing in `run.go`.
- **YANG, tests, fixtures, docs & news fragment:** Updated `internal/component/mtu/yang/ze-mtu-cmd.yang`, added unit tests (`overhead_test.go`, `arith_test.go`, `verdict_test.go`, `run_test.go`, `tunnel_linux_test.go`, `apply_test.go`), real-kernel `userns.Enter` integration test (`non_ipsec_integration_linux_test.go`), functional test (`test/plugin/show-mtu-non-ipsec-tunnels.ci` + `plugin/show-mtu-non-ipsec` fixture), updated all 7 documentation/feature pages, and added `website/changes/fragments/mtu-non-ipsec-tunnels.md`.

### Bugs Found/Fixed
- **VPP IPv6 GRE/IPIP underlay & tunnel mode (Run 1):** `dumpGRETunnelInfo` and `dumpIPIPTunnelInfo` originally omitted IPv6 endpoint refusal and `d.Tunnel.Mode` inspection. Fixed by adding `requireIPv4VPPEndpoints` and `vppTunnelModeFlowBased` (P2P -> `false`, MP -> `true`, unknown -> error) in `internal/plugins/iface/vpp/tunnel.go`, covered by `TestVPPGetTunnelInfoRoundTrip` (`mgre0`, `gre6`, `ipip6`) in `internal/plugins/iface/vpp/apply_test.go`.
- **Defense-in-depth remote family check (Run 1):** `deriveTunnelOverhead` originally did not cross-check non-VXLAN `Kind.IsV6Underlay()` against `info.Remote`. Fixed by adding `checkTunnelRemoteFamily` in `internal/component/mtu/cmd/overhead.go`, covered by `TestNonIPsecTunnelOverheadRefusesUnknownOrFOU` in `internal/component/mtu/cmd/overhead_test.go`.
- **WireGuard equal-ceiling tie-break (Run 1):** `sizeWireguardTunnel` originally kept the first peer on equal ceiling even when the first used an `assumed` path and a later peer had a directly measured path. Fixed in `internal/component/mtu/cmd/run.go` (`ceil == chosen.ceiling && chosen.assumed && !assumed`).
- **VPP native OsName lookup after `ResolveDevice`:** Added `b.names.lookupNativeIndex` fallback in `internal/plugins/iface/vpp/ifacevpp.go:resolveIndex` so `GetTunnelInfo`/`GetWireguardDevice` succeed after `iface.ResolveDevice` returns VPP's native `OsName`, covered by `TestVPPGetTunnelInfoRoundTrip/vxlan_tunnel0`.

### Documentation Updates
- `docs/architecture/diagnostics/path-mtu.md`: updated `The run`, `The payload`, `The ESP overhead` (added `Non-IPsec tunnel overhead`), `The arithmetic`, `The verdicts`, and test table (`<!-- source: internal/component/mtu/cmd/run.go -- runMTU, mtuDeps, liveDeps, listTunnelSnapshot -->`, `<!-- source: internal/component/mtu/cmd/overhead.go -- encapOverhead, deriveESPOverhead, deriveTunnelOverhead, deriveWireguardOverhead, espICVOctets -->`).
- `docs/architecture/api/commands.md`: updated `ze-mtu:show-mtu` row with `kind`, `overhead`, and all non-IPsec tunnel kinds.
- `docs/features/interfaces.md`: added `GetTunnelInfo` row to Backend Implementations matrix with `<!-- source: internal/component/iface/backend.go -- Backend.GetTunnelInfo; internal/plugins/iface/netlink/tunnel_linux.go -- GetTunnelInfo; internal/plugins/iface/vpp/tunnel.go -- GetTunnelInfo -->`.
- `docs/guide/command-reference.md`: updated `show mtu` usage, prose, and JSON example (`kind`, `overhead`).
- `docs/guide/command-catalogue.md`: updated `show mtu` row.
- `docs/comparison.md`: updated `show mtu` comparison row to cover IPsec, GRE, WireGuard, VXLAN, and IP-in-IP.
- `features/path-mtu-diagnostic-for-ipsec-tunnels.md`: updated real-path tests (`test/plugin/show-mtu-non-ipsec-tunnels.ci`), description, and source anchors (`listTunnelSnapshot`, `deriveTunnelOverhead`, `deriveWireguardOverhead`).
- `website/changes/fragments/mtu-non-ipsec-tunnels.md`: operator-facing weekly update fragment (`topic: Diagnostics`, `feature: path-mtu-diagnostic-for-ipsec-tunnels`).
- `./le doc check all`: 0 failures in any file touched by this spec (all reported drift/help/anchor hits are unrelated concurrent files in `bgp`/`ze-policy-conf`/`plugin format`); `./le doc ste check`: OK (0 habits grew).

### Deviations from Plan
- Added `internal/plugins/iface/vpp/ifacevpp.go` (`resolveIndex` fallback to `b.names.lookupNativeIndex`) to support VPP native `OsName` returned by `iface.ResolveDevice`, and recorded it in `## Files to Modify`.
- Added `isDormantKernelFallbackTunnel` in `internal/component/mtu/cmd/run.go` so Linux's auto-created admin-down fallback devices (`tunl0`, `sit0`, `gre0`, `gretap0`, `ip6tnl0`, `ip6gre0`) with empty/unspecified endpoints do not pollute `show mtu` on boxes that merely have tunnel kernel modules loaded, while preserving any active or configured tunnel with those names.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Initial VPP `GetTunnelInfo` readback did not inspect `Tunnel.Mode` or reject IPv6 outer endpoints on GRE/IPIP | VPP's `GreTunnel` and `IpipTunnel` structs carry `Mode` (`P2P` vs `MP`) and open `ip_types.Address` families; IPv6 GRE/IPIP are kernel-only on Ze and MP tunnels have no fixed remote | Run 1 independent review | Added `vppTunnelModeFlowBased` and `requireIPv4VPPEndpoints` in `internal/plugins/iface/vpp/tunnel.go` with regression tests in `apply_test.go` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Discover live non-IPsec tunnel (`gre`, `gretap`, `ip6gre`, `ip6gretap`, `vxlan`, `ipip`, `sit`, `ip6tnl`, `ipip6`) and WireGuard interfaces through `iface` | Done | `internal/component/mtu/cmd/run.go:listTunnelSnapshot`, `internal/component/iface/dispatch.go:GetTunnelInfo,GetWireguardDevice`, `internal/plugins/iface/netlink/tunnel_linux.go:GetTunnelInfo`, `internal/plugins/iface/vpp/tunnel.go:GetTunnelInfo`, `internal/plugins/iface/vpp/query.go:ListInterfaces,vppInterfaceType` | Single `iface.ListInterfaces()` snapshot per run; works on kernel and VPP |
| Probe unicast remote endpoints deduplicated alongside IPsec peers | Done | `internal/component/mtu/cmd/run.go:resolvePeerTargets,isProbeableRemote,addTarget` | Skips unspecified, multicast, `FlowBased`, and endpoint-less tunnels |
| Derive exact protocol overheads from live dataplane state (exact-or-reject) | Done | `internal/component/mtu/cmd/overhead.go:encapOverhead,deriveTunnelOverhead,checkTunnelRemoteFamily,deriveWireguardOverhead` | Refuses `OuterEncapType != 0`, unknown kinds, invalid endpoints, and family mismatches with `errOverheadRefused` |
| Emit unified tunnel rows, run verdicts, underlay advice, and Ze config commands (`set interface tunnel\|wireguard`) including when `!ze_ike` | Done | `internal/component/mtu/cmd/run.go:runMTU,sizeTunnels,sizeNonIPsecTunnel,sizeWireguardTunnel,applyTunnelSizing`, `internal/component/mtu/cmd/verdict.go:tunnelCommand` | Preserves backward compatibility for IPsec rows, `host` runs, and `!ze_ike` with 0 tunnels |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestNonIPsecTunnelOverheadExact` (`internal/component/mtu/cmd/overhead_test.go:294`), `TestCeilingAlignsAndSubtractsTrailer` (`arith_test.go`) | All 9 tunnel kinds + WireGuard v4/v6 exact byte counts, `trailer=0`, `block=1` |
| AC-2 | Done | `TestNonIPsecTunnelOverheadRefusesUnknownOrFOU` (`overhead_test.go:378`), `TestShowMTUNonIPsecEdgeCasesNotProbedOrDown` (`run_test.go:1790`) | FOU/GUE, unknown/unsupported kind, family mismatch, invalid WG endpoint -> `errOverheadRefused`, fault note, `sized: false` |
| AC-3 | Done | `TestGetTunnelInfoKernelRoundTrip` (`internal/plugins/iface/netlink/tunnel_linux_test.go:485`, 10 real-kernel `userns.Enter` subtests) | Real kernel round-trip for unkeyed/keyed GRE, keyed GRETap, IP6GRE default & ign-encap-limit, VXLAN, IPIP, SIT, IP6Tnl, IPIP6, plus dummy rejection |
| AC-4 | Done | `TestVPPGetTunnelInfoRoundTrip` (`internal/plugins/iface/vpp/apply_test.go:3053`), `TestVPPListInterfacesClassifiesTunnelsAndWireguard` (`apply_test.go:3203`) | Covers GRE, GRETap, MP GRE (`FlowBased`), IPIP, VXLAN, native `OsName`, IPv6 GRE/IPIP refusal, non-tunnel/missing refusal, logical `Name` and canonical classification |
| AC-5 | Done | `TestShowMTUSizesNonIPsecTunnelsAndWireguard` (`run_test.go:1542`), `TestShowMTUNonIPsecClampedPathIntegration` (`non_ipsec_integration_linux_test.go:31`) | Oversized GRE & VXLAN sizing, IPv4/IPv6/L2/unaddressed `mss` rules, 1280 floor, `set interface tunnel` commands, `action-needed` verdict, underlay advice |
| AC-6 | Done | `TestShowMTUWireguardMultiPeerUsesTightestCeiling` (`run_test.go:1716`) | Two-peer IPv4+IPv6 + passive peer emits 1 row sized to tightest ceiling (1320 -> 1288, mss 1228) and 1 `set interface wireguard` command; secrets uncopied |
| AC-7 | Done | `TestShowMTUNonIPsecEdgeCasesNotProbedOrDown` (`run_test.go:1790`) | Down, `0.0.0.0`, multicast `239.1.1.1`, `FlowBased`, passive WG, readback error isolation, dormant kernel fallback filtering |
| AC-8 | Done | `TestShowMTUNonIPsecWorksWithoutIPsecInventory` (`run_test.go:1915`) | `!ze_ike` with 0 tunnels preserves absent verdict + original note; with GRE sizes tunnel, emits verdict + underlay advice + commands |
| AC-9 | Done | `TestShowMTUSizesNonIPsecTunnelsAndWireguard` (`run_test.go:1542`) | Shared remote deduped to 1 probe; IPsec row keeps backward-compatible fields + `kind`/`overhead`; `show mtu host` skips all tunnels |
| AC-10 | Done | `test/plugin/show-mtu-non-ipsec-tunnels.ci` + `showMTUNonIPsec` (`internal/test/fixture/plugin_fixture_show_mtu.go`) + `TestShowMTUNonIPsecClampedPathIntegration` (`non_ipsec_integration_linux_test.go`) | Live daemon `.ci` over 3-namespace 1400 clamp with keyed GRE + VXLAN, complemented by unprivileged `userns.Enter` integration test |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestNonIPsecTunnelOverheadExact` | Done | `internal/component/mtu/cmd/overhead_test.go:294` | PASS |
| `TestNonIPsecTunnelOverheadRefusesUnknownOrFOU` | Done | `internal/component/mtu/cmd/overhead_test.go:378` | PASS (includes family-mismatch regression cases) |
| `TestShowMTUSizesNonIPsecTunnelsAndWireguard` | Done | `internal/component/mtu/cmd/run_test.go:1542` | PASS |
| `TestShowMTUWireguardMultiPeerUsesTightestCeiling` | Done | `internal/component/mtu/cmd/run_test.go:1716` | PASS |
| `TestShowMTUNonIPsecEdgeCasesNotProbedOrDown` | Done | `internal/component/mtu/cmd/run_test.go:1790` | PASS |
| `TestShowMTUNonIPsecWorksWithoutIPsecInventory` | Done | `internal/component/mtu/cmd/run_test.go:1915` | PASS |
| `TestGetTunnelInfoKernelRoundTrip` | Done | `internal/plugins/iface/netlink/tunnel_linux_test.go:485` | PASS (10 real-kernel `userns.Enter` subtests) |
| `TestVPPGetTunnelInfoRoundTrip` | Done | `internal/plugins/iface/vpp/apply_test.go:3053` | PASS (includes MP `FlowBased` & IPv6 GRE/IPIP refusal) |
| `TestVPPListInterfacesClassifiesTunnelsAndWireguard` | Done | `internal/plugins/iface/vpp/apply_test.go:3203` | PASS |
| `TestShowMTUNonIPsecClampedPathIntegration` | Done | `internal/component/mtu/cmd/non_ipsec_integration_linux_test.go:31` | PASS (real Linux kernel `userns.Enter`) |
| `show-mtu-non-ipsec-tunnels` | Done | `test/plugin/show-mtu-non-ipsec-tunnels.ci` | Registered fixture `plugin/show-mtu-non-ipsec` in `register_show_mtu.go` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| All 30 files in `## Files to Modify` | Done | Modified and verified via `git status --porcelain` and `./le spec review check` |
| All 3 files in `## Files to Create` (`non_ipsec_integration_linux_test.go`, `test/plugin/show-mtu-non-ipsec-tunnels.ci`, `website/changes/fragments/mtu-non-ipsec-tunnels.md`) | Done | Created and verified via `ls -la` and `./le spec review check` |

### Audit Summary
- **Total items:** 27 (4 task requirements + 10 ACs + 11 tests + 2 file lists covering 33 files)
- **Done:** 27
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (recorded in Deviations: VPP `resolveIndex` native-name fallback; dormant kernel fallback tunnel filter)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Discover live non-IPsec tunnel (all 9 `TunnelKind` values) and WireGuard interfaces through `iface` on kernel and VPP | Real-kernel `userns.Enter` + VPP unit tests | `./le test unit pkg path ./internal/plugins/iface/netlink/ run TestGetTunnelInfoKernelRoundTrip` PASS (`2.734s`, 10 real-kernel subtests); `./le test unit pkg path ./internal/plugins/iface/vpp/` PASS (`1.889s`, `TestVPPGetTunnelInfoRoundTrip` + `TestVPPListInterfacesClassifiesTunnelsAndWireguard`); `./le go vet-platforms darwin` PASS |
| Derive exact protocol overheads and refuse unmodeled encapsulations (exact-or-reject) | Unit tests | `./le test unit pkg path ./internal/component/mtu/cmd/` PASS (`13.033s`, `TestNonIPsecTunnelOverheadExact`, `TestNonIPsecTunnelOverheadRefusesUnknownOrFOU`, `TestCeilingAlignsAndSubtractsTrailer`, `TestRecommendedAbsentBelowMinimumLinkMTU`) |
| Probe deduplicated unicast remotes, size non-IPsec and multi-peer WireGuard tunnels alongside IPsec (and under `!ze_ike`), and emit `set interface tunnel\|wireguard` commands | Unit + real-kernel `userns.Enter` integration + functional `.ci` test | `TestShowMTUSizesNonIPsecTunnelsAndWireguard`, `TestShowMTUWireguardMultiPeerUsesTightestCeiling`, `TestShowMTUNonIPsecEdgeCasesNotProbedOrDown`, `TestShowMTUNonIPsecWorksWithoutIPsecInventory`, `TestShowMTUNonIPsecClampedPathIntegration` (`13.033s` PASS against real kernel clamped 1400 path); `test/plugin/show-mtu-non-ipsec-tunnels.ci` with `plugin/show-mtu-non-ipsec` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None | All in-scope items (AC-1..AC-10, kernel/VPP/stub readback, unified overhead & sizing pipeline, tests, docs, news fragment) were completed | none |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/mtu-non-ipsec-tunnels-0cfcb484-f28a-4e4e-b1d1-85693900af06.md` |
| `./le spec review check` | `review_gate: OK (25 code files, clean, hashes match tmp/review/mtu-non-ipsec-tunnels-0cfcb484-f28a-4e4e-b1d1-85693900af06.md)` |
| Rounds | `2` (Run 1: 6 findings fixed; Run 2: CLEAN — 0 BLOCKER, 0 ISSUE, 0 NOTE) |
| Reviewer lenses used | Run 2 independent reviewer (`10b9b356-7360-4063-82d6-5bb694bd657f`): logic+wiring, security+edge-cases, design-documents (12 pages, 33 decision rows), 7-question `ze-go-style.md` pass, `/ze-review-spec` completeness |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | VPP `dumpGRETunnelInfo` / `dumpIPIPTunnelInfo` did not refuse IPv6 outer endpoints under exact-or-reject or inspect `d.Tunnel.Mode` (`P2P` vs `MP`) | `internal/plugins/iface/vpp/tunnel.go` | Added `requireIPv4VPPEndpoints` and `vppTunnelModeFlowBased` with regression tests (`mgre0`, `gre6`, `ipip6`) in `internal/plugins/iface/vpp/apply_test.go` |
| 2 | ISSUE | `deriveTunnelOverhead` lacked defense-in-depth kind/remote-family check | `internal/component/mtu/cmd/overhead.go` | Added `checkTunnelRemoteFamily` with 4 mismatch regression cases in `internal/component/mtu/cmd/overhead_test.go` |
| 3 | ISSUE | `internal/plugins/iface/vpp/ifacevpp.go` omitted from `## Files to Modify` | `plan/post-release/features/spec-mtu-non-ipsec-tunnels.md` | Added `internal/plugins/iface/vpp/ifacevpp.go` row to `## Files to Modify` |
| 4 | ISSUE | `sizeWireguardTunnel` did not prefer directly measured (`!assumed`) peer on equal-ceiling ties | `internal/component/mtu/cmd/run.go` | Added tie-break `ceil == chosen.ceiling && chosen.assumed && !assumed` in `sizeWireguardTunnel` |
| 5 | ISSUE | `docs/features/interfaces.md` `GetTunnelInfo` row lacked a source anchor | `docs/features/interfaces.md:1607` | Added `<!-- source: internal/component/iface/backend.go -- Backend.GetTunnelInfo; internal/plugins/iface/netlink/tunnel_linux.go -- GetTunnelInfo; internal/plugins/iface/vpp/tunnel.go -- GetTunnelInfo -->` |
| 6 | ISSUE | Weekly update news fragment referenced internal Go symbols | `website/changes/fragments/mtu-non-ipsec-tunnels.md` | Rewrote in operator-facing prose; verified STE clean |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/mtu/cmd/non_ipsec_integration_linux_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 5970 Nov 27 22:51 internal/component/mtu/cmd/non_ipsec_integration_linux_test.go` |
| `test/plugin/show-mtu-non-ipsec-tunnels.ci` | Yes | `-rw-r--r-- 1 thomas thomas 2265 Nov 27 22:51 test/plugin/show-mtu-non-ipsec-tunnels.ci` |
| `website/changes/fragments/mtu-non-ipsec-tunnels.md` | Yes | `-rw-r--r-- 1 thomas thomas 1153 Nov 27 23:41 website/changes/fragments/mtu-non-ipsec-tunnels.md` |
| All 30 modified files across `iface`, `iface/netlink`, `iface/vpp`, `mtu/cmd`, `mtu/yang`, `fixture`, `docs/`, `features/` | Yes | Verified via `git status --porcelain` and `./le spec review check` (`25 code files, clean, hashes match`) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Exact overhead for all 9 `TunnelKind` values + WireGuard v4/v6 and unified `ceiling`/`recommended` (`trailer=0`, `block=1`) | `internal/component/mtu/cmd/overhead.go:deriveTunnelOverhead,deriveWireguardOverhead`, `arith.go:ceiling,recommended`; `./le test unit pkg path ./internal/component/mtu/cmd/` PASS (`13.033s`, `TestNonIPsecTunnelOverheadExact`, `TestCeilingAlignsAndSubtractsTrailer`) |
| AC-2 | Refuses `OuterEncapType != 0`, unknown kind, family mismatch, invalid WG endpoint with `errOverheadRefused` and fault note | `internal/component/mtu/cmd/overhead.go:deriveTunnelOverhead,checkTunnelRemoteFamily`, `run.go:sizeNonIPsecTunnel`; `TestNonIPsecTunnelOverheadRefusesUnknownOrFOU` & `TestShowMTUNonIPsecEdgeCasesNotProbedOrDown` PASS |
| AC-3 | Real Linux kernel round-trip via `iface.GetTunnelInfo` for all tunnel kinds + keyed `gretap` + `ip6gre` encap-limit | `internal/plugins/iface/netlink/tunnel_linux.go:GetTunnelInfo,greTunnelInfo,readIP6GREEncapLimit`; `./le test unit pkg path ./internal/plugins/iface/netlink/ run TestGetTunnelInfoKernelRoundTrip` PASS (`2.734s`, all 10 `userns.Enter` subtests) |
| AC-4 | VPP `ListInterfaces` logical naming + canonical classification and `GetTunnelInfo` round-trip + IPv6/MP/absent handling | `internal/plugins/iface/vpp/{tunnel.go,query.go,ifacevpp.go}`; `./le test unit pkg path ./internal/plugins/iface/vpp/` PASS (`1.889s`, `TestVPPGetTunnelInfoRoundTrip`, `TestVPPListInterfacesClassifiesTunnelsAndWireguard`) |
| AC-5 | Oversized GRE & VXLAN sizing, `kind`/`overhead`, L2/unaddressed `mss` omission with 1280 floor, `set interface tunnel` commands, underlay advice | `internal/component/mtu/cmd/run.go:sizeNonIPsecTunnel,nonIPsecInnerFamily,addressedInnerFamily,applyTunnelSizing`; `TestShowMTUSizesNonIPsecTunnelsAndWireguard` & `TestShowMTUNonIPsecClampedPathIntegration` PASS |
| AC-6 | Multi-peer WireGuard emits one row and one `set interface wireguard` command sized to tightest ceiling (preferring `!assumed` on tie), no key copy | `internal/component/mtu/cmd/run.go:sizeWireguardTunnel,wireguardPeersFromSpec`; `TestShowMTUWireguardMultiPeerUsesTightestCeiling` PASS |
| AC-7 | Down, `0.0.0.0`, multicast, `FlowBased`, passive WG never probed, isolated readback errors, dormant fallback filtering | `internal/component/mtu/cmd/run.go:isProbeableRemote,isDormantKernelFallbackTunnel,sizeNonIPsecTunnel,sizeWireguardTunnel`; `TestShowMTUNonIPsecEdgeCasesNotProbedOrDown` PASS |
| AC-8 | `!ze_ike` preserves absent verdict when 0 tunnels exist and sizes non-IPsec tunnels with verdict + underlay advice when present | `internal/component/mtu/cmd/run.go:runMTU,resolvePeerTargets,hasNonIPsecTunnels`; `TestShowMTUNonIPsecWorksWithoutIPsecInventory` PASS |
| AC-9 | Shared remote deduped alongside IPsec, backward-compatible IPsec row + `kind`/`overhead`, `show mtu host` skips all tunnels | `internal/component/mtu/cmd/run.go:addTarget,tunnelRow,runMTU`; `TestShowMTUSizesNonIPsecTunnelsAndWireguard` PASS |
| AC-10 | End-to-end live Linux functional test over 1400 clamp with keyed GRE + VXLAN | `test/plugin/show-mtu-non-ipsec-tunnels.ci` + `internal/test/fixture/plugin_fixture_show_mtu.go:showMTUNonIPsec` + `TestShowMTUNonIPsecClampedPathIntegration` PASS |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze-mtu:show-mtu` RPC -> `handleShowMTU` -> `iface.ListInterfaces`/`GetTunnelInfo`/`GetWireguardDevice` -> ICMP DF prober -> `deriveTunnelOverhead`/`deriveWireguardOverhead` -> `set interface tunnel` commands | `test/plugin/show-mtu-non-ipsec-tunnels.ci` (drives `ze start mtu-non-ipsec.conf` inside `plugin/clamped-path` with `plugin/show-mtu-non-ipsec` fixture, `option=needs-linux:caps=net-admin,net-raw`) | Yes — read `test/plugin/show-mtu-non-ipsec-tunnels.ci` and `internal/test/fixture/{plugin_fixture_show_mtu,register_show_mtu}.go` in full; verified exact assertions on `mtugre1` (`gre`, overhead 28, ceiling 1372, rec 1340, mss 1300) and `mtuvx10` (`vxlan`, overhead 50, ceiling 1350, rec 1318, no mss), `underlay.advice=circuit-clamped`, and exact `set interface tunnel` commands; complemented by real-kernel `TestShowMTUNonIPsecClampedPathIntegration` (`13.033s` PASS) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestNonIPsecTunnelOverheadExact` & `TestShowMTUWireguardMultiPeerUsesTightestCeiling` confirm 60B (IPv4) / 80B (IPv6) WireGuard overhead at full MTU |
| A-2 | confirmed | `TestGetTunnelInfoKernelRoundTrip/gretap-keyed` (`userns.Enter` real kernel PASS) confirms `OFlags` (`GRE_KEY`) reads keyed GRETap accurately despite vendored `parseGretapData` `IKey`/`OKey` swap |
| A-3 | confirmed | `TestGetTunnelInfoKernelRoundTrip/ip6gre-default-encaplimit` and `/ip6gre-ign-encap-limit` (`userns.Enter` real kernel PASS) confirm `RTM_GETLINK` `IFLA_GRE_FLAGS` distinguishes default (+8B) vs `IP6_TNL_F_IGN_ENCAP_LIMIT` (+0B) |
| A-4 | confirmed | `TestShowMTUWireguardMultiPeerUsesTightestCeiling` confirms single row + single `set interface wireguard` command at tightest peer ceiling |
| A-5 | confirmed | All existing and new `internal/component/mtu/cmd` tests PASS (`13.033s`) with additive `kind` and `overhead` keys |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Architecture (`docs/architecture/diagnostics/path-mtu.md`, `docs/features/interfaces.md`) | Updated `The run`, `The payload`, `Non-IPsec tunnel overhead`, `The arithmetic`, `The verdicts`, test table, and `GetTunnelInfo` matrix row with `<!-- source: ... -->` anchors; `./le doc ste check` OK | Yes |
| API/RPC (`docs/architecture/api/commands.md`) | Updated `ze-mtu:show-mtu` row with `kind`, `overhead`, and all non-IPsec tunnel kinds matching `internal/component/mtu/cmd/run.go` | Yes |
| CLI/User Guide/Comparison/Feature (`docs/guide/command-reference.md`, `docs/guide/command-catalogue.md`, `docs/comparison.md`, `features/path-mtu-diagnostic-for-ipsec-tunnels.md`, `ze-mtu-cmd.yang`) | Updated all 5 files to reflect IPsec + non-IPsec tunnels and new `.ci` test; `./le cli grammar` OK, `./le doc ste check` OK, 0 issues in `./le doc check all` for this spec's files | Yes |
| News fragment (`website/changes/fragments/mtu-non-ipsec-tunnels.md`) | `topic: Diagnostics` (present in `website/data/topics.json`), `feature: path-mtu-diagnostic-for-ipsec-tunnels`, all 6 fixed headings, operator-facing prose, STE OK | Yes |
| Unaffected categories (config syntax, plugin SDK, wire format, RFC summaries, route metadata, metrics, doctor checks) | Verified via `git diff --name-only` (zero edits to `pkg/`, `rfc/short/`, config YANG leaves, metrics, or runtime dependencies); `./le arch tier check`, `./le arch doctor-coverage check`, and `./le repo check` report 0 issues in this spec's files | Yes |

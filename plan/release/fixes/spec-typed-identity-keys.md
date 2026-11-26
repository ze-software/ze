# Spec: typed-identity-keys

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Three identity keys remain directly constructible outside a single constructor: `withdrawalKey` in `bgp-rr` still holds an untyped `route string` beside four dead legacy walks that carry the old `WireNLRI.String` bug; `configjson.PeerConfigKey` has exported string fields and stores raw operator spellings so non-canonical IPv6 peer addresses miss lookups in `role`, `rib` FlowSpec/blackhole, and `rpki` while two consumers canonicalize on their own; `fwdBodyCacheKey` in `reactor` is built by four struct literals and nothing proves its fields determine `maxMsgSize` and `peer.sendContext()` inside `buildFwdBody`. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Owner ruling, 2026-11-14** (Thomas's reply "ok" to the proposal put to him):
"Narrow it to the three real key cases (forwarded-message cache, route reflector
withdrawal, peer config key), enforced by the compiler as you ruled. Move the
other open defects into the existing one-off fix specs for BGP and for
OSPF/IS-IS/BFD/VRRP, and the L2TP one into the L2TP identity spec that already
asks for it."

"As you ruled" is decision D1 of `plan/release/fixes/spec-single-owner-lifecycle.md`:
no annotation gate and no typed analyzer; where a single writer must be
enforced by the compiler, the state lives in a type with unexported fields in a
package of its own, so the constructor is the only writer.

**The problem at HEAD (2026-11-26).** Each of the three keys can still be built
by struct literal or ad-hoc string concatenation outside a single constructor:

| Key | Sites that build or read it in HEAD | What still goes wrong |
|-----|-------------------------------------|-----------------------|
| Route reflector withdrawal key | `withdrawalKey` (`internal/component/bgp/plugins/rr/withdrawal.go`), `ownerRouteKey` (`rr.go`), `nativeInventory` and `putWithdrawalLocked` (`stored_membership.go`), plus four dead legacy walks (`walkUnicastNLRIs`, `walkNLRIsAllocating`, `walkVPNNLRIs`, `updateWithdrawalMapText`) and `textRouteKey` | Live wire, text, and stored replay paths already route through `nativeInventory` and `nlrisplit.GetPrefixKey`, but `withdrawalKey.route` and `ownerRouteKey.route` remain untyped `string` values constructed inside package `rr`, tests build raw literals with ad-hoc strings, and the dead `walkNLRIsAllocating` / `updateWithdrawalMapText` functions still carry the old `WireNLRI.String` and `textRouteKey` code |
| Per-peer config key | `configjson.PeerConfigKey`, `PeerKey`, `GroupKey`, `LookupPeerConfig` (`internal/component/bgp/configjson/traverse.go`); `blackholecfg.Parse`; `filter_community` `newBlackholeAgreements` and `lookup`; `rpki` `parsePeerActions` and `appendPeerActions`; `role` `extractPeerRoleConfigs` and `configuredPeerKeys` (`role/config.go`) | `PeerConfigKey` exports `ID` and `Template`; `PeerKey`, `LookupPeerConfig`, and `role/config.go` key raw operator IP text so `2001:0DB8::1` misses `2001:db8::1` in `role` filters, `role` capability selectors (`CapabilityDecl.Peers`), and `rib` FlowSpec redirect config; `blackholecfg` and `rpki` each re-canonicalize on their own; `filter_community` re-parses `key.ID` and still has a dead `PeerConfigKey{ID: dest.Name}` lookup |
| Forward body cache key | four `fwdBodyCacheKey` literals: two in `forward_rs.go` (`:805`, `:862`), two in `reactor_api_forward.go` (`:1397`, `:1450`), type in `forward_body.go` | Any site in package `reactor` can build a literal with omitted fields; `buildFwdBody` reads `maxMsgSize` and `peer.sendContext()` beside the key fields, and no test proves `destCtxID` and `extended` determine both |

**Goal.** Each of the three keys is a type with unexported fields in a package
of its own, built only by its constructors, which take every identifying fact.
A composite literal outside the package does not compile. The route reflector
deletes its four dead legacy walks and builds every withdrawal route key through
`rr/routekey` (backed by `nlrisplit.GetPrefixKey`), with functional and FRR
interop proof for equal-length EVPN and multicast ADD-PATH departure. The
per-peer config key holds a parsed canonical `netip.Addr` or dynamic group name
in `configjson.PeerConfigKey`, eliminates the two per-consumer canonicalizations
and `filter_community`'s dead literal lookup, and canonicalizes `role`'s peer
address keys and capability selectors. The forward cache key moves to
`reactor/fwdkey` as a zero-allocation value type and AC-5 proves whether
`destCtxID` and `extended` determine `maxMsgSize` and `peer.sendContext()` (or
extends the key if an input differs).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/bgp-engine.md`, `ai/doors/bgp-plugins.md`, `ai/doors/le-tooling.md`
  → Constraint: a plugin never imports another plugin's implementation; `internal/core/bgp` imports no component; what crosses a process boundary stays a self-contained value.
  → Constraint: the forward path allocates nothing per destination where a value type serves (`ai/rules/performance.md`).
- [ ] `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/bgp/fanout-dedup.md`, `docs/architecture/encoding-context.md`, `docs/architecture/buffer-architecture.md`
  → Constraint: the forward cache key stays a fixed-size comparable value returned by value; the cache is per fan-out and on the hot path.
- [ ] `docs/architecture/core-design.md` (route reflector withdrawal tracking; component boundaries)
- [ ] `docs/architecture/wire/nlri.md` (family-specific route identity, `nlrisplit`)
  → Decision: a family's route identity is the `PrefixKeyFunc` it registers with `nlrisplit`; no consumer derives its own.
- [ ] `docs/architecture/config/syntax.md` (`traverse.go` declares it as its design)
- [ ] `docs/architecture/meta/role.md`, `docs/architecture/meta/filter-community.md`, `docs/architecture/api/process-protocol.md`
  → Constraint: `role`'s capability selector strings cross the plugin boundary in `CapabilityDecl.Peers` and must match `configjson.PeerSelectors`, which emits canonical `netip.Addr.String()`.
- [ ] `docs/architecture/plugin/rib-storage-design.md`, `docs/guide/rpki.md`
  → Constraint: `show bgp rpki status` keeps its `"peer"` and `"group"` JSON rendering; `rpki_status.go` switches from exported field reads to `PeerConfigKey` accessors with identical output.
- [ ] `plan/learned/DESIGN-HISTORY.md`, NLRI identity forms: `Bytes()` is the identity form and `Pack()`/`WriteTo()` the wire form; `INET.String()` drops the path identifier and `Key()` is the bare CIDR
  → Constraint: no NLRI display string is a map key.
- [ ] `plan/release/fixes/spec-single-owner-lifecycle.md`, D1
  → Decision: compiler enforcement through unexported fields in a package of its own; no source gate.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7911.md`, read against `rfc/full/rfc7911.txt` Sections 2 and 5
  → Constraint: under ADD-PATH the path identifier is part of the route's identity.
- [ ] `rfc/short/rfc4271.md`, Section 4.3 (a withdrawal pairs with the announcement it names)

**Key insights:**
- RFC 7911 Section 2: "a new identifier (termed "Path Identifier" hereafter) needs to be introduced so that a particular path for an address prefix can be identified by the combination of the address prefix and the Path Identifier."
- RFC 7911 Section 5: "In particular, a new advertisement for a given address prefix and a given Path Identifier replaces a previous advertisement for the same address prefix and Path Identifier."
- Without ADD-PATH the identity of an opaque family (such as EVPN or FlowSpec) is its registered `nlrisplit.GetPrefixKey` output, never a display string or byte length.

## Current Behavior (MANDATORY)

Verified against HEAD on 2026-11-26.

**Source files read:**
- [ ] `internal/component/bgp/plugins/rr/withdrawal.go` - `withdrawalKey`, `updateWithdrawalMapWire`, and superseded `walkUnicastNLRIs`, `walkNLRIsAllocating`, `walkVPNNLRIs`, `updateWithdrawalMapText`
- [ ] `internal/component/bgp/plugins/rr/stored_membership.go` - `nativeInventory`, `storedInventory`, `textRecords`, `putWithdrawalLocked`, `setWithdrawalEntryLocked`, `deleteWithdrawalLocked`, `textRouteKey`, `withTextNative`
- [ ] `internal/component/bgp/plugins/rr/reflect.go` - `reflectWire`, `reflectText`
- [ ] `internal/component/bgp/plugins/rr/rr.go` - `ownerRouteKey`, `handleStateDown`, `sendPeerDown`, `sendDepartureBatch`, `withdrawalGroup`, `nlriDelCmd`
- [ ] `internal/core/bgp/nlri/nlrisplit/prefix_key.go` - `GetPrefixKey`, `RouteCIDR`, `KeysByCIDR`, `keyOpaque`, `keyEVPN`
- [ ] `internal/component/bgp/reactor/forward_body.go` - `fwdBodyCacheKey`, `buildFwdBody`, `fwdSplitParsedUpdate`
- [ ] `internal/component/bgp/reactor/forward_rs.go` - two `fwdBodyCacheKey` literals (`:805`, `:862`)
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - two `fwdBodyCacheKey` literals (`:1397`, `:1450`)
- [ ] `internal/component/bgp/reactor/forward_path_id.go` - `fwdRegenerateRawPathIDs`, `fwdPathIDMemo.framed`
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go` - `maxMsgSize` derived from `MaxMessageLength(msgtype.TypeUPDATE, extendedMsg)`
- [ ] `internal/component/bgp/configjson/traverse.go` - `PeerConfigKey`, `PeerKey`, `GroupKey`, `KeyFor`, `LookupPeerConfig`, `PeerSelectors`
- [ ] `internal/component/bgp/blackholecfg/blackholecfg.go` - `Parse` (per-consumer `netip.ParseAddr` + `PeerConfigKey{ID: addr.String()}`)
- [ ] `internal/component/bgp/plugins/rpki/rpki_config.go`, `rpki_status.go`, `rpki.go`, `blackhole.go` - `parsePeerActions` (per-consumer canonicalization), `appendPeerActions` (`.Template`, `.ID`), `LookupPeerConfig` callers
- [ ] `internal/component/bgp/plugins/filter_community/blackhole.go` - `newBlackholeAgreements` (re-parses `key.ID`), `lookup` (dead `PeerConfigKey{ID: dest.Name}` literal)
- [ ] `internal/component/bgp/plugins/rib/rib_blackhole_config.go`, `rib_blackhole.go`, `rib_flowspec_redirect.go` - `parseBlackholeConfig`, `parseFlowSpecRedirectConfig`, `LookupPeerConfig` callers
- [ ] `internal/component/bgp/plugins/cmd/announce/blackhole_agreement.go` - `agreedSelector` (`LookupPeerConfig` caller)
- [ ] `internal/component/bgp/plugins/role/config.go` - `extractPeerRoleConfigs`, `configuredPeerKeys`, `extractRoleCapabilities`

| Producer | What it does in HEAD (2026-11-26) |
|----------|-----------------------------------|
| `updateWithdrawalMapWire` (`rr/withdrawal.go`), `reflectText` -> `textRecords` and `storedInventory` (`rr/stored_membership.go`) | Already unified onto `nativeInventory`, which calls `nlrisplit.GetPrefixKey(info.Family)(raw, scratch[:], withdraw)`, stores `WireForm: true`, and formats `Prefix` as native hex (with 4-octet Path ID prefix under ADD-PATH) so `sendDepartureBatch` withdraws every family via `update hex nlri <family> [addpath] del <hex>`. However, `nativeInventory` still concatenates `family\|pathID\|hex(key)` into an untyped `withdrawalKey.route string` and omits `info.AddPath` from the route key |
| `walkUnicastNLRIs`, `walkNLRIsAllocating`, `walkVPNNLRIs`, `updateWithdrawalMapText` (`rr/withdrawal.go`) and `textRouteKey` (`rr/stored_membership.go`) | Superseded legacy code still present in package `rr`: the three `walk*` functions have zero callers anywhere in the tree, `walkNLRIsAllocating` still holds the old `WireNLRI.String` and SAFI-branch code, and `updateWithdrawalMapText` is called only by `receive_owner_test.go` |
| `withdrawalKey` (`rr/withdrawal.go`), `ownerRouteKey` (`rr/rr.go`) | Both hold bare `route string`; package `rr` and its tests construct `withdrawalKey{...}` literals directly |
| `fwdBodyCacheKey` (`reactor/forward_body.go`), four literals in `forward_rs.go` and `reactor_api_forward.go` | Plain package-private struct in `reactor`; `buildFwdBody` reads `maxMsgSize` (`MaxMessageLength(UPDATE, extendedMsg)`) and `peer.sendContext()` (an atomic load on `peer`, rather than `bgpctx.Registry.Get(destCtxID)`) beside the key fields; no test pins that the key determines them |
| `PeerConfigKey`, `PeerKey`, `LookupPeerConfig` (`configjson/traverse.go`) | `PeerConfigKey` exports `ID string` and `Template bool`; `PeerKey` returns raw `PeerRemoteIP` text or unparsed `name`; `LookupPeerConfig` constructs literals from raw `addr` and `name` strings without canonicalizing |
| `blackholecfg.Parse`, `rpki` `parsePeerActions` | Each re-parses and re-renders the IP address on its own to work around `PeerKey` returning raw text |
| `filter_community` `newBlackholeAgreements`, `lookup` | Reads `key.Template` and re-parses `key.ID` via `netip.ParseAddr`; `lookup` probes `byTemplate` with a non-template literal `PeerConfigKey{ID: dest.Name}` which always misses |
| `role/config.go` (`extractPeerRoleConfigs`, `configuredPeerKeys`, `extractRoleCapabilities`) | Stores raw `PeerRemoteIP` text in `configs`, `nameToIP`, `configuredPeerKeys`, and `CapabilityDecl.Peers`; runtime readers (`getFilterConfig`, `PeerSelectors`) pass canonical `netip.Addr.String()`, so non-canonical IPv6 spellings (such as `2001:0DB8::1`) silently miss role policy and omit Role capability 9 on OPEN |
| `rib/rib_flowspec_redirect.go`, `rib/rib_blackhole.go`, `cmd/announce/blackhole_agreement.go` | Key and query maps via `configjson.KeyFor` and `configjson.LookupPeerConfig`; `rib_flowspec_redirect.go` stores raw `PeerKey` spellings while `flowSpecRedirectChecked` passes canonical `addr.String()`, so non-canonical IPv6 spellings miss there too |

**Behavior to preserve:**
- Forwarded UPDATE bytes and the zero-copy same-context forward path (zero per-destination heap allocation from the forward key).
- Native hex source-down withdrawal commands (`update hex nlri <family> [addpath] del <hex>`) produced by `nativeInventory` and `sendDepartureBatch`, plus text `nlriDelCmd` compatibility where tested on non-wire entries.
- Dynamic group template isolation: a group template key stays disjoint from a peer of the same name (`PeerConfigKey` template namespace).
- `show bgp rpki status` JSON output naming each `"peer"` (canonical address) and `"group"` (template name), and `role`'s `CapabilityDecl.Peers` wire format (canonical address string or `group:<name>`).
- `rib_blackhole_config.go`, `rib_blackhole.go`, `rib_flowspec_redirect.go`, `rpki.go`, `rpki/blackhole.go`, and `cmd/announce/blackhole_agreement.go` callers continue working through `configjson.KeyFor` and `configjson.LookupPeerConfig`.

**Behavior to change:**
- Move the route reflector route identity key into `internal/component/bgp/plugins/rr/routekey/` with unexported fields (`family`, `addPath`, `pathID`, registered key) and constructor `routekey.New(fam, addPath, pathID, raw, withdraw)` backed by `nlrisplit.GetPrefixKey`, change `withdrawalKey.route` and `ownerRouteKey.route` from `string` to `routekey.Key` (retaining numeric `owner` and `connection` on `withdrawalKey`), and delete the four superseded legacy walks (`walkUnicastNLRIs`, `walkNLRIsAllocating`, `walkVPNNLRIs`, `updateWithdrawalMapText`) and dead `textRouteKey`.
- Move `fwdBodyCacheKey` into `internal/component/bgp/reactor/fwdkey/` with unexported fields and a value-returning constructor, replace all four literal sites, and prove (or extend) the key against `maxMsgSize` and `peer.sendContext()` in `buildFwdBody`.
- Make `configjson.PeerConfigKey`'s fields unexported (`addr netip.Addr`, `group string`, `template bool`), canonicalize in `PeerKey` and `LookupPeerConfig` (adding `AddrKey(netip.Addr)` and read accessors `IsTemplate()`, `Addr()`, `Group()`, `String()`), delete the duplicate canonicalizations in `blackholecfg.Parse` and `rpki` `parsePeerActions`, remove `filter_community`'s string re-parse and dead `dest.Name` literal lookup, and canonicalize `role/config.go` (`configs`, `nameToIP`, `configuredPeerKeys`, and `CapabilityDecl.Peers`) through `configjson.PeerKey`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Native reflector inventory | `internal/component/bgp/plugins/rr/stored_membership.go` `nativeInventory`, `storedInventory`, `textRecords`; `withdrawal.go` `updateWithdrawalMapWire`; `rr.go` `sendDepartureBatch` | Already landed on live wire, text, and stored-replay paths: calls `nlrisplit.GetPrefixKey` after `nlri.SplitPathID`, stores `WireForm: true` with native hex (and 4-octet Path ID prefix under ADD-PATH), and sends `update hex nlri <family> [addpath] del <hex>` at peer down; omits `addPath` from its untyped `route string` | Extends: keeps this unified native inventory pipeline and replaces its untyped `route string` concatenation with `routekey.Key` from `rr/routekey/` carrying `(family, addPath, pathID, registeredKey)`, while deleting the superseded `walkUnicastNLRIs`, `walkNLRIsAllocating`, `walkVPNNLRIs`, `updateWithdrawalMapText`, and `textRouteKey` dead code |
| NLRI identity forms | `internal/core/bgp/nlri/nlri.go` `NLRI.Bytes`, `SplitPathID`; `plan/learned/DESIGN-HISTORY.md` "`Bytes()` (identity/RIB key) and `Pack()`/`WriteTo()` (wire) are separate" | `Bytes` is the identity form; `String` is display only; `SplitPathID` separates the 4-octet ADD-PATH identifier from native NLRI bytes | Extends: `rr/routekey` consumes native bytes split by `nlri.SplitPathID` and never accepts a display string |
| Registered route identity | `internal/core/bgp/nlri/nlrisplit/prefix_key.go` `GetPrefixKey`, `RouteCIDR`, `KeysByCIDR`, `keyEVPN`, `keyVPN`, `keyLabeled`, `keyCIDR`, `keyOpaque` | Each family registers its route identity function; the default keeps the whole NLRI; EVPN, VPN, labeled unicast, FlowSpec, and CIDR families strip non-key fields | Extends: `rr/routekey`'s constructor calls `nlrisplit.GetPrefixKey` for every family with no family list or SAFI switch in the plugin |
| Route server withdrawal identity | `internal/component/bgp/plugins/rs/server_inventory.go` `recordKey`, `nativeKey`, `withdrawalKey` | One function derives the key for both add and delete from the registered key, path identifier, and ADD-PATH presence | Does not apply to `bgp-rs`, which is already keyed right and outside the three cases Thomas scoped; serves as the design model `rr/routekey` follows |
| `PeerSettings.PeerKey` | `internal/component/bgp/reactor/peer_settings.go` | The reactor's peer map key, a `netip.AddrPort` value built by one method | Does not apply: identifies a TCP session (address + port), already a value built by one method |
| `configjson.PeerConfigKey`, `PeerKey`, `GroupKey`, `KeyFor`, `LookupPeerConfig` | `internal/component/bgp/configjson/traverse.go` | Builds per-peer and dynamic-group config keys and looks them up in peer-then-group precedence; `Template` separates the group namespace | Extends: turns the existing `configjson` package into the single compiler-enforced owner of `PeerConfigKey` with unexported `netip.Addr`/`group`/`template` fields, canonical `PeerKey`/`AddrKey`/`LookupPeerConfig`, and read accessors |
| Per-consumer address canonicalization | `internal/component/bgp/blackholecfg/blackholecfg.go` `Parse`; `internal/component/bgp/plugins/rpki/rpki_config.go` `parsePeerActions`; `internal/component/bgp/plugins/filter_community/blackhole.go` `newBlackholeAgreements` | Each consumer re-parses raw IP strings via `netip.ParseAddr` because `PeerKey` kept raw text | Replaces: deleted once `configjson.PeerKey` canonicalizes at the single boundary (no layering) |
| `role` string key and capability selector | `internal/component/bgp/plugins/role/config.go`; `configjson.CapabilityGroupKey`, `PeerSelectors` | String map that also feeds `CapabilityDecl.Peers` across the plugin RPC boundary | Extends: peer address strings are derived from `configjson.PeerKey`'s canonical accessor so `configs`, `nameToIP`, `configuredPeerKeys`, and `CapabilityDecl.Peers` match `PeerSelectors` across IPv6 spellings |
| `peerConfigNameFor`, `recordCreatedPeerLocked` | `internal/component/bgp/reactor/reactor_api.go` | Derives and writes the running-tree peer NAME for runtime `create bgp peer` | Does not apply: moved to `plan/release/fixes/spec-bgp-one-off-defects.md` under Thomas's 2026-11-14 ruling |
| Forward body cache (`fwdBodyCacheKey`, `fwdBodyCache`, `bodySlots`) | `internal/component/bgp/reactor/forward_body.go` `fwdBodyCacheKey`; `forward_rs.go` `bodySlots`; `reactor_api_forward.go` `fwdBodyCache` | Per-fan-out cache sharing `buildFwdBody` results across destinations with equal `(destCtxID, wire, extended, preserveOpaque)`, currently constructed by four struct literals in package `reactor` | Replaces key type: keeps the per-fan-out `fwdBodyCache` map and 4-slot `bodySlots` cache intact while moving the key type into `internal/component/bgp/reactor/fwdkey/` with unexported fields and a single value constructor |
| Forward facts snapshot | `internal/component/bgp/reactor/peer_forward_facts.go` `peerForwardFacts` | Holds `sendCtxID`, `extendedMsg`, and derived `maxMsgSize` per destination | Extends: `reactor/fwdkey` constructor takes the destination facts that identify a body; `maxMsgSize` stays derived from `extendedMsg` |
| One type per lifecycle state | `docs/contributing/ze-go-style.md`, "One type per lifecycle state"; D1 of `spec-single-owner-lifecycle.md` | A type with unexported fields in its own package enforces a single constructor via the compiler | Extends: applied directly to the three identity keys |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "What crosses a boundary is a self-contained value." | agrees | `rr/routekey` and `reactor/fwdkey` are process-local value keys; `role`'s `CapabilityDecl.Peers` remains a self-contained slice of canonical address or `group:` strings across the plugin RPC boundary. |
| `ai/doors/bgp-engine.md` | Boundaries | the door covering `forward_body.go`, `forward_rs.go`, `reactor_api_forward.go`, `configjson/traverse.go`, `blackholecfg/blackholecfg.go` | "**`internal/core/bgp` imports no component**, and a plugin never imports another plugin's implementation (`./le arch tier check`)." | agrees | `reactor/fwdkey` lives under `internal/component/bgp/reactor/` and is imported only by `reactor`; `configjson` stays under `internal/component/bgp/configjson/`; `internal/core/bgp` gains no component import. |
| `ai/doors/bgp-plugins.md` | Boundaries | the door covering `rr/withdrawal.go`, `rr/stored_membership.go`, `rr/rr.go`, `rpki/rpki_config.go`, `rpki/rpki_status.go`, `filter_community/blackhole.go`, `role/config.go` | "A plugin never imports another plugin's implementation (`./le arch tier check`)." | agrees | `rr/routekey` is private inside `internal/component/bgp/plugins/rr/` and imported only by `rr`; `rpki`, `role`, `filter_community`, and `rib` share only `internal/component/bgp/configjson` and `blackholecfg`. |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `test/plugin/rr-withdraw-equal-length-evpn.ci` and `test/plugin/rpki-per-peer-address-spelling.ci` | "**A bare `go test` omits the feature tags**, so plugins do not register and unrelated tests fail." | agrees | All unit tests run via `./le test unit pkg path ...` under the feature tags, and the two functional tests run under `test/plugin/` via `./le test bgp plugin`. |
| `docs/architecture/core-design.md` | 19. Component Boundaries | the `// Design:` page of `withdrawal.go`, `rr.go`, `reactor_api_forward.go`, `filter_community/blackhole.go`, `role/config.go`; anchored to `forward_rs.go`; modified by this spec | "Each component under `internal/component/` is independently removable. Cross-component coupling follows a strict hierarchy:" | agrees | Both new key packages stay inside `internal/component/bgp/` with no new cross-component imports; this spec updates `core-design.md` to document the typed reflector route key. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | This spec touches only BGP identity keys and leaves VPP/kernel dataplane commit validation unchanged. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | This spec touches only BGP identity keys and leaves dataplane mismatch verdicts unchanged. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | This spec touches no firewall verifier or YANG backend annotations. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | This spec touches no data-plane selectors or backend registrations. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | `reactor/fwdkey` caches only post-edit body builds and does not alter export-chain next-hop rewriting or `next-hop:auto`. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | This spec makes no change to `policychain.Sets` or next-hop authoring forms. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | This spec makes no change to built-in `next-hop:*` compilation or config loading. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | This spec preserves export-chain folding and raw-override next-hop provenance untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | This spec touches no next-hop policy validation or encoding. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | This spec preserves forwarded vs. locally originated next-hop policy unchanged. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | This spec touches no filter-chain insertion or `\| implied` display code. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 on `core-design.md` | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | Neither `PeerConfigKey` canonicalization nor `rr/routekey` nor `reactor/fwdkey` introduces any daemon-start refusal. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 on `core-design.md` | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | This spec touches no VPP/kernel dataplane startup behavior. |
| `docs/architecture/wire/nlri.md` | Native VPN inventory identity | anchored to `rr/withdrawal.go` and `rr/rr.go`; modified by this spec | "The source peer, family, ADD-PATH presence and Path Identifier distinguish inventory entries. Identifier zero remains a valid path." | agrees | `rr/routekey.New(fam, addPath, pathID, raw, withdraw)` distinguishes entries by family, ADD-PATH presence (`addPath bool`), Path Identifier (including zero), and `nlrisplit.GetPrefixKey` output, while `withdrawalKey` retains the numeric source peer owner and connection; this spec updates the stale `walkNLRIsAllocating`/`walkVPNNLRIs` anchors on `nlri.md` to name `routekey` and `nativeInventory`. |
| `docs/architecture/api/architecture.md` | Route Reflection via API (Cache Pattern) | anchored to `rr/rr.go` and `reactor/reactor_api_forward.go` | - | silent | The page documents the cache-forward RPC and message bus; it says nothing about the internal Go key types of `rr/routekey`, `reactor/fwdkey`, or `configjson.PeerConfigKey`. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | This spec touches no `\| implied` formatting or config rendering. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | This spec touches no bus participant registrations or paths. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | This spec touches no bus `send > to` rules. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | This spec touches no bus `receive > from` rules. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | This spec touches no bus allow-lists. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | This spec touches no bus filter compilation. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | This spec touches no bus arbitration rules. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | This spec touches no bus delivery display. |
| `docs/architecture/api/process-protocol.md` | 5-Stage Startup Protocol | anchored to `rr/rr.go`, `reactor_api_forward.go`, `configjson/traverse.go` | "For a BGP plugin each declaration's `Peers` names a peer by its name or its remote IP address, or `group:<name>` for a dynamic group's template; the reactor injects capabilities under the same set (`configjson.PeerSelectors`), so a plugin may key its declaration by either, as `bgp-role` keys by the address." | agrees | `role/config.go` now derives its peer address strings from canonical `configjson.PeerKey`, ensuring `CapabilityDecl.Peers` matches `configjson.PeerSelectors` even when the operator wrote a non-canonical IPv6 address. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 on `api/process-protocol.md` | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | This spec adds no engine operation and modifies no plugin authorization profile. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 on `api/process-protocol.md` | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | This spec touches no engine-to-plugin event queue or delivery policy. |
| `docs/architecture/api/text-parser.md` | Original native routes in TEXT | anchored to `rr/rr.go` | "A summary string is never a retained route identity. TEXT owners must retain the section's path identifier separately from its native bytes." | agrees | `rr/routekey.New(fam, addPath, pathID, raw, withdraw)` keys the fork-mode text path (`reflectText` -> `textRecords` -> `nativeInventory`) by native bytes and explicit `(addPath, pathID)` through `nlrisplit.GetPrefixKey`, never a summary string, and deletes the legacy `updateWithdrawalMapText` and `textRouteKey` string-summary path. |
| `docs/architecture/plugin/plugin-relationships.md` | 6. Bus Topic Map | anchored to `rr/rr.go` | - | silent | The page maps plugin topics and dependencies; it says nothing about the internal key types of `rr/routekey`, `reactor/fwdkey`, or `configjson.PeerConfigKey`. |
| `docs/architecture/plugin/rib-storage-design.md` | Received source-policy provenance | anchored to `rr/rr.go`, `reactor_api_forward.go`; the `// Design:` page of `rr/stored_membership.go`, `blackholecfg/blackholecfg.go`, `rpki/rpki_config.go` | "Their existing route inventories also retain the original received Peer owner and source receipt. Route keys include that numeric owner, so identical routes from same-address peers coexist." | agrees | `withdrawalKey` and `ownerRouteKey` retain their numeric `owner` (and `connection`) fields beside the new `routekey.Key`, preserving same-address peer coexistence and source-receipt provenance untouched. |
| `docs/architecture/bgp/structural-forwarding.md` | Four costs, and what replaced them | the `// Design:` page of `forward_body.go`, `forward_rs.go`; anchored to `reactor_api_forward.go`; modified by this spec | "Both forwarding rails key body reuse on effective opaque-attribute treatment, not on a plugin or peer role." | agrees | `reactor/fwdkey` keeps `destCtxID`, `wire`, `extended`, and `preserveOpaque` in a fixed-size comparable value built by one constructor (extending only if AC-5 proves another builder input differs), and this spec updates `structural-forwarding.md` to document `fwdkey`. |
| `docs/architecture/bgp/egress-attribute-rules.md` | The two rules | anchored to `forward_body.go`, `forward_rs.go`, `reactor_api_forward.go` | - | silent | The page documents egress attribute suppression and provenance rules; it says nothing about `fwdBodyCacheKey` or key construction. |
| `docs/architecture/buffer-architecture.md` | Why allocations on the UPDATE path cost | anchored to `forward_body.go`; the `// Design:` page of `reactor_api_forward.go` | "Every allocation on that path adds GC pressure and latency, so the path is built from three interlocking strategies: the caller owns the buffer, bounded pools replace `make()`, and the read side stays lazy (raw byte slices plus offset iterators, never parsed structs)." | agrees | `fwdkey.Key` remains a fixed-size comparable value returned by value from an inlined constructor with zero heap allocations on the forwarding hot path. |
| `docs/architecture/forward-congestion-pool.md` | Buffer Ownership: Zero-Copy with Copy-on-Modify | anchored to `forward_body.go`, `reactor_api_forward.go` | - | silent | The page documents per-peer forward overflow pooling and `ownOverflowBodies`; it says nothing about `fwdBodyCacheKey`. |
| `docs/architecture/update-building.md` | Group-Aware Build Path (AnnounceNLRIBatch / WithdrawNLRIBatch) | anchored to `forward_body.go`, `reactor_api_forward.go`; modified by this spec | "The parameter set is `announceFacts`, and both rails use it. It is the argument set of the build AND the map key that forms the group, so a per-peer fact the build reads is a fact the key holds." | agrees | `fwdkey.Key` applies this exact principle to `buildFwdBody`, pinning in AC-5 that the cache key determines every input `buildFwdBody` reads (or adding any input it does not), and this spec repoints line 1491's `fwdBodyCacheKey` anchor to `fwdkey.Key`. |
| `docs/architecture/update-building.md` | Design decisions | decision of 2026-11-23 on `update-building.md` | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | This spec touches no FlowSpec next-hop emission or ExaBGP migration logic. |
| `docs/architecture/wire/mp-nlri-ordering.md` | Generation Strategy | anchored to `forward_body.go` | - | silent | The page documents RFC 7606 MP-attribute-first ordering and single-field splitting inside `buildFwdBody`; it says nothing about `fwdBodyCacheKey`. |
| `docs/architecture/memory/lifetime-contracts.md` | The four contracts | anchored to `forward_rs.go`, `reactor_api_forward.go` | - | silent | The page documents buffer ownership and cache-eviction lifetime contracts; it says nothing about `fwdBodyCacheKey` construction. |
| `docs/architecture/route-selection.md` | AIGP session policy | anchored to `forward_rs.go` | - | silent | The page documents best-path validation and AIGP policy; it says nothing about `fwdBodyCacheKey`. |
| `docs/architecture/api/update-syntax.md` | End-of-RIB (EOR) | anchored to `reactor_api_forward.go` | - | silent | The page documents the `update text`/`update hex` and EOR syntax; it says nothing about `fwdBodyCacheKey` or internal withdrawal map keys. |
| `docs/architecture/bgp/fanout-dedup.md` | The decisions | anchored to `reactor_api_forward.go` | "Effective opaque treatment is part of that identity." | agrees | `fwdkey.Key` retains `preserveOpaque` beside `destCtxID`, `wire`, and `extended`, preserving the edit-set dedup and body-cache boundary. |
| `docs/architecture/encoding-context.md` | Zero-Copy Forwarding | anchored to `reactor_api_forward.go` | "`forwardUpdateCore` compares contexts once per destination and caches each built body under its `(destCtxID, wire, extended)` key, so every destination on the same ContextID reuses one build:" | agrees | `fwdkey.Key` moves that exact forward body cache key (with `preserveOpaque`) behind an unexported-field constructor and verifies in AC-5 that `destCtxID` and `extended` determine `maxMsgSize` and the path-ID rewrite. |
| `docs/architecture/meta/role.md` | Config keying | anchored to `reactor_api_forward.go`, `configjson/traverse.go`, `role/config.go` | "A configured peer's role config is keyed by its remote address, because that is the key its readers hold: all three `getFilterConfig` callers pass `PeerFilterInfo.Address.String()` first." | agrees | `role/config.go` now derives its peer address strings from canonical `configjson.PeerKey`, so non-canonical IPv6 spellings match `PeerFilterInfo.Address.String()`. |
| `docs/architecture/config/syntax.md` | What a reader receives | the `// Design:` page of `configjson/traverse.go`; modified by this spec | "`(*Tree).ToMap` lowers the parsed tree to `map[string]any` for delivery, and the shape it emits for a leaf-list or a list is not the one the node type suggests." | agrees | `configjson.PeerKey` reads the lowered peer/group maps and parses and canonicalizes the remote IP into an unexported `netip.Addr` inside `PeerConfigKey`; this spec documents that canonical key contract on `syntax.md`. |
| `docs/guide/rpki.md` | Per-peer and per-group actions | the `// Design:` page of `rpki/rpki_status.go` | "`show bgp rpki status` reports the effective global actions (`actions`) and the resolved per-peer overrides with the source of each leaf (`peer-actions`). An entry names what it is: `"peer"` carries a remote address, and `"group"` carries a listen-range group's name and states what every session that group accepts inherits." | agrees | `rpki_status.go` reads `PeerConfigKey` through its accessors (`IsTemplate()`, `String()`) and emits the exact same canonical `"peer"` address and `"group"` name JSON. |
| `docs/architecture/meta/filter-community.md` | Performance | anchored to `filter_community/blackhole.go` | "The lookup reads the `blackholecfg.Parse` map re-keyed once at config time into an address map and a group-template map, so no string is built per route." | agrees | `filter_community/blackhole.go` switches from `.Template`/`netip.ParseAddr(key.ID)` to `key.IsTemplate()`/`key.Addr()` and drops the dead `PeerConfigKey{ID: dest.Name}` lookup, preserving zero per-route string allocation. |
| `docs/architecture/wire/capabilities.md` | Capability Codes | anchored to `role/config.go` | - | silent | The page documents BGP OPEN capability wire encodings (including code 9); it says nothing about internal Go key types or how `role/config.go` canonicalizes peer IP selectors. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A received UPDATE reaching `bgp-rr` (raw `WireUpdate` through DirectBridge, text UPDATE event when forked, or stored replay row from `bgp-adj-rib-in`).
- A received UPDATE fanned out by the reactor's forward paths (`forward_rs.go`, `reactor_api_forward.go`).
- The config JSON each BGP plugin (`rpki`, `role`, `rib`, `filter_community`, `blackholecfg`, `cmd/announce`) reads at configure, reload, and command execution.

### Transformation Path
1. Route reflector: `updateWithdrawalMapWire`, `textRecords`, and `storedInventory` each pass the family, ADD-PATH presence (`addPath bool`), path identifier (`uint32`), native NLRI bytes, and direction into `nativeInventory`, which calls `routekey.New(fam, addPath, pathID, raw, withdraw)` (backed by `nlrisplit.GetPrefixKey`) to produce a `routekey.Key`. `withdrawalKey` (retaining numeric `owner` and `connection`) and `ownerRouteKey` store that typed `routekey.Key`; `sendDepartureBatch` withdraws every stored entry in its native hex form (`update hex nlri <family> [addpath] del <hex>`).
2. Forward: per destination, both `reactorForwardRSSectionResult` and `forwardUpdateSection` construct a `fwdkey.Key` via `fwdkey.New(...)` from the destination context ID, wire pointer, extended-message flag, and opaque-attribute treatment; equal keys share one `buildFwdBody` result.
3. Config: `configjson.ForEachPeer` visits each peer and dynamic group template; `PeerKey` parses and canonicalizes the remote IP (or address-shaped peer name) into an unexported `netip.Addr` via `AddrKey`, and `GroupKey` stores the template name under `template: true`. Runtime readers query via `LookupPeerConfig` (which canonicalizes string address inputs) or `AddrKey` / `key.Addr()` (zero-allocation typed lookup). `role/config.go` derives its string map keys and `CapabilityDecl.Peers` entries from `PeerKey`'s canonical accessor.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine to `bgp-rr` | raw message through DirectBridge; text event when forked | No |
| `bgp-rr` to engine | `update hex` departure recovery commands at source down | No |
| Config to each plugin | config JSON, parsed in the plugin's own process | No |
| `bgp-role` to engine | `CapabilityDecl.Peers` strings over Stage 3 / config-verify RPC | No |

### Changed-Wire Census

**Search evidence:** scope `internal/component/bgp/` and `test/`; terms `nlriDelCmd`, `update hex nlri`, `WireForm`, `ParseUpdateWire`, `withdrawalInfo`, `extractRoleCapabilities`, `PeerSelectors`, `fwdBodyCacheKey`. Positive controls verified at HEAD on 2026-11-26: `nativeInventory` (`stored_membership.go:413-427`) already sets `WireForm: true` and `sendDepartureBatch` (`rr.go:804-814`) emits `update hex nlri <family> [addpath] del <hex>` for every family; `extractRoleCapabilities` (`role/config.go:318-342`) writes `configs` keys into `CapabilityDecl.Peers`, read by `reactor` `capabilitySelectors` via `configjson.PeerSelectors` (`configjson/traverse.go:162-176`). `fwdkey.Key` and `PeerConfigKey` themselves are process-local map keys.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Role capability declaration (`CapabilityDecl.Peers`) | writer | `internal/component/bgp/plugins/role/config.go` `extractRoleCapabilities`, `extractPeerRoleConfigs` | currently emits raw `PeerRemoteIP` text (e.g. `2001:0DB8::1`) into `CapabilityDecl.Peers` | change: emit canonical `netip.Addr.String()` via `configjson.PeerKey` so non-canonical IPv6 spellings match `PeerSelectors` |
| Role capability declaration (`CapabilityDecl.Peers`) | reader | `internal/component/bgp/reactor/peer.go` `capabilitySelectors` via `internal/component/bgp/configjson/traverse.go` `PeerSelectors`, and the BGP peer router outside the repository receiving OPEN Capability 9 | `PeerSelectors` already queries canonical `addr.String()`; non-canonical spellings previously missed and omitted Capability 9 | preserve reader; non-canonical IPv6 peers now receive their configured Capability 9 on OPEN |
| Source-down withdrawal command | writer | `internal/component/bgp/plugins/rr/rr.go` `sendPeerDown`, `sendDepartureBatch` | `nativeInventory` already sets `WireForm: true`, emitting `update hex nlri <family> [addpath] del <hex>` on live wire, text, and replay paths | preserve: moving route identity into `rr/routekey` keeps this exact command form and deletes only the dead legacy `walk*` / `updateWithdrawalMapText` paths |
| Source-down withdrawal command | reader | `internal/component/bgp/plugins/cmd/update/update_wire.go` `ParseUpdateWire` | already parses `update hex nlri <family> [addpath] del <hex>` for every registered family | preserve: no grammar change |
| Withdrawal UPDATE to reflector clients | writer | the reactor send path `ParseUpdateWire` reaches | builds `MP_UNREACH_NLRI` from the parsed `update hex` command | preserve encoding; proven end-to-end for equal-length EVPN and multicast ADD-PATH |
| Withdrawal UPDATE to reflector clients | reader | the client router outside the repository (FRR in `bgp-rr-departure-equal-length-evpn-frr`) | receives `MP_UNREACH_NLRI` withdrawals for every route still held when a source departs | preserve wire format; verified by unit, functional `.ci`, and FRR interop tests |
| Forwarded UPDATE | writer | `internal/component/bgp/reactor/forward_body.go` `buildFwdBody` | key moves to `reactor/fwdkey` | preserve bytes unless AC-5 finds an input `destCtxID` and `extended` do not determine; if so, extend `fwdkey.Key` so mismatched bodies are no longer shared |
| Forwarded UPDATE | reader | the destination peer outside the repository | unchanged | preserve |

### Integration Points
- `nlrisplit.GetPrefixKey`, `nlri.SplitPathID` - called by `rr/routekey`'s constructor.
- `peerForwardFacts` (`peer_forward_facts.go`) - feeds `reactor/fwdkey`'s constructor.
- `configjson` callers: `blackholecfg`, `rpki`, `role`, `filter_community`, `rib`, `cmd/announce`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | each key is still built at the same layer as today, enforced through its package constructor |
| No unintended coupling | Yes | `reactor/fwdkey` is imported by `reactor` only; `rr/routekey` by `bgp-rr` only; `configjson` keeps its existing importers |
| No duplicated functionality | Yes | route identity comes solely from `nlrisplit.GetPrefixKey`; duplicate IP canonicalizations in `blackholecfg` and `rpki` and string re-parsing in `filter_community` are deleted |
| Zero-copy preserved where applicable | Yes | `fwdkey.Key` stays a fixed-size comparable struct returned by value; constructor is inlined and allocates nothing |
| Registration over hardcoding, outbound | Yes | no family list in `rr/routekey`: the constructor delegates to `nlrisplit.GetPrefixKey` |
| Registration over hardcoding, inbound | Yes | searched `rr/withdrawal.go` and `rr/stored_membership.go`: deleting the dead `walkNLRIsAllocating` / `walkVPNNLRIs` removes the last `SAFIVPN` and `SAFIMPLSLabel` branches |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Equal destination `ContextID`s mean equal ADD-PATH send flags | `fwdRegenerateRawPathIDs` and `fwdSplitParsedUpdate` read `peer.sendContext().AddPath`; `bgpctx.Registry` dedups by content | if a peer's `sendContext()` can differ from `bgpctx.Registry.Get(destCtxID)` (for example across renegotiation/teardown), `buildFwdBody` must read `bgpctx.Registry.Get(destCtxID)` or the key must carry the differing fact | AC-5 test (`TestFwdBodyKeyDeterminesBuilderInputs`) | unvalidated |
| A-2 | `maxMsgSize` is a function of `extendedMsg` alone on the forward path | `peer_forward_facts.go:474`, `int(message.MaxMessageLength(msgtype.TypeUPDATE, extendedMsg))` | `fwdkey.Key` must carry `maxMsgSize` | AC-5 test (`TestFwdBodyKeyDeterminesBuilderInputs`) | unvalidated |
| A-3 | Every family the route reflector can receive has a registered splitter and key, or falls back to `keyOpaque` which preserves full NLRI bytes | `nlrisplit.GetPrefixKey` default `keyOpaque` (`prefix_key.go:57-64`) | an opaque family without a custom key stays keyed by its full NLRI bytes, never by byte length | `TestRouteKeyUsesRegisteredIdentity` | unvalidated |
| A-4 | `update hex` withdraws every family `ParseUpdateWire` accepts in `MP_UNREACH_NLRI` (including EVPN and multicast ADD-PATH) | `update_wire.go` `ParseUpdateWire` uses `nlrisplit.GetWithdraw` | source-down withdrawal of an EVPN or multicast ADD-PATH route fails | `test/plugin/rr-withdraw-equal-length-evpn.ci`, `TestRRWithdrawalKeepsMulticastAddPathApart` | unvalidated |
| A-5 | `fwdkey.New(...)` returning a small struct by value is inlined and allocates nothing | Go escape analysis | hot-path allocation per destination on fan-out | `-gcflags=-m` on `internal/component/bgp/reactor` and forward benchmark allocs/op | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Unexported `PeerConfigKey` fields and `withdrawalKey.route` type change break test literals across test files | compile errors in `*_test.go` files | update test literals to call `configjson.AddrKey` / `GroupKey` and `routekey.New`; preserve every test assertion |
| R-2 | A family's registered key differs between announcement and withdrawal framing (e.g., RFC 8277 Compatibility field) | a withdrawal misses its announcement in `rr.withdrawals` | `routekey.New` takes the `withdraw bool` direction argument and passes it to `nlrisplit.GetPrefixKey` |
| R-3 | Deleting `walkUnicastNLRIs`, `walkNLRIsAllocating`, `walkVPNNLRIs`, and `updateWithdrawalMapText` leaves stale `<!-- source: -->` doc anchors or breaks `receive_owner_test.go` | `./le doc check all` or `rr` unit test build failure | update the stale anchors in `docs/architecture/wire/nlri.md` and `docs/guide/route-reflection.md`, and migrate `receive_owner_test.go` to `textRecords`/`applyInventoryLocked` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Route reflector clients retain stale routes or lose live ones on withdrawal or source down; per-peer `rpki`, `role`, `rib`, `cmd/announce`, or `filter_community` config fails to match or advertises no Role capability; a forwarded UPDATE body built for one destination is sent to a destination requiring different framing |
| How is it reverted? | One commit per key or single clean revert; no persisted format or external config schema changes |
| Who else touches this path? | `spec-bgp-one-off-defects.md` (reactor and rr rows); forward pool and route-reflector replay work |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A source announces two equal-length EVPN routes through `bgp-rr`, withdraws one, then goes down | → | `routekey.New`, `nativeInventory`, `updateWithdrawalMapWire`, `sendPeerDown` | `test/plugin/rr-withdraw-equal-length-evpn.ci` |
| Config with peer remote IP `2001:0DB8::1` and per-peer `rpki` / `role` settings | → | `configjson.PeerKey`, `AddrKey`, `LookupPeerConfig`, `role` `extractPeerRoleConfigs` | `test/plugin/rpki-per-peer-address-spelling.ci` |
| A fan-out to two destinations sharing one `ContextID` | → | `fwdkey.New`, `buildFwdBody` | `TestFwdBodyKeyDeterminesBuilderInputs` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The repository after the change | `routekey.Key` (in `internal/component/bgp/plugins/rr/routekey`), `fwdkey.Key` (in `internal/component/bgp/reactor/fwdkey`), and `configjson.PeerConfigKey` (in `internal/component/bgp/configjson`) each contain only unexported fields, every site outside each package builds values solely through constructors, the four dead legacy walks in `rr/withdrawal.go` and dead `textRouteKey` are removed, and the tree compiles | Unit tests inside each key's own package (`routekey`, `fwdkey`, `configjson`), which share package access to verify internal invariants |
| AC-2 | Two routes of one non-CIDR, non-VPN family and equal length (EVPN Type 2 differing only in MAC) from one source through `bgp-rr`, one withdrawn, followed by source down | Both announcements occupy distinct `routekey.Key` entries; withdrawing one removes only that entry while the other remains reflected and tracked; on source down every client receives an `MP_UNREACH_NLRI` withdrawal for the remaining route in native hex form | Unicast, VPN, and labeled unicast entries keep their existing native hex withdrawal behavior via `nativeInventory` and `sendDepartureBatch` |
| AC-3 | Two ADD-PATH paths (distinct Path Identifiers, including zero) of one IPv4 or IPv6 multicast prefix through `bgp-rr`, one withdrawn, followed by source down; plus a non-ADD-PATH route beside an ADD-PATH Path Identifier `0` route for the same prefix | Both ADD-PATH paths occupy distinct `routekey.Key` entries keyed by `(family, addPath, pathID, registeredKey)`, and a non-ADD-PATH route (`addPath=false, pathID=0`) never collides with an ADD-PATH Path Identifier `0` route (`addPath=true, pathID=0`); withdrawing one leaves the other tracked; on source down the surviving path is withdrawn under its Path Identifier | None: RFC 7911 identity (ADD-PATH presence and Path Identifier, including zero) applies uniformly across every address family |
| AC-4 | The same two routes delivered through the fork-mode text path (`reflectText` -> `textRecords`) | `textRecords` encodes each entry and constructs its key through `routekey.New` via `nativeInventory`, pairing each withdrawal with its announcement and withdrawing remaining entries on source down | None: `updateWithdrawalMapText` and `textRouteKey` are deleted so no second text-keying path survives |
| AC-5 | Two destinations sharing a `ContextID` and `extendedMsg` flag in `buildFwdBody` | `TestFwdBodyKeyDeterminesBuilderInputs` proves that `fwdkey.Key` uniquely determines every input `buildFwdBody` reads (including `maxMsgSize` and the send context's ADD-PATH flags); if any builder input can differ across equal keys, `fwdkey.Key` carries that input so the resulting bodies stay distinct | Shared body reuse when all builder inputs agree, because sharing identical builds is the cache's purpose |
| AC-6 | A peer address written in non-canonical form in config (`2001:0DB8::1`) while the live session reports canonical `2001:db8::1` | `configjson.PeerKey` produces the exact same `PeerConfigKey` as `AddrKey(netip.MustParseAddr("2001:db8::1"))`; `rpki`, `role` (including `CapabilityDecl.Peers` and `nameToIP`), `rib` (blackhole and FlowSpec redirect), `cmd/announce`, and `filter_community` all resolve the per-peer setting, and per-consumer canonicalizations in `blackholecfg` and `rpki` are gone | None: every peer-config consumer reads canonical addresses from `configjson` |
| AC-7 | A configured peer and a dynamic peer group sharing the same name | `PeerKey` and `GroupKey` produce distinct `PeerConfigKey` values; a lookup for the peer resolves only the peer's entry and never returns the dynamic group template, while a dynamic group member resolves the group template | Dynamic group template lookup for members created from that dynamic group, which have no static peer entry |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An operator's EVPN source announces two equal-length Type 2 routes, withdraws one, then disconnects behind the route reflector | wire -> `bgp-rr` `updateWithdrawalMapWire` -> `routekey.New` -> `nativeInventory` -> `sendDepartureBatch` `update hex` -> client | `test/plugin/rr-withdraw-equal-length-evpn.ci`, interop `bgp-rr-departure-equal-length-evpn-frr` |
| 2 | An operator writes an IPv6 peer address with uppercase hex or leading zeros (`2001:0DB8::1`) | config -> `configjson.PeerKey` -> canonical `PeerConfigKey` -> `rpki` / `role` / `rib` / `filter_community` lookup | `test/plugin/rpki-per-peer-address-spelling.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRRWithdrawalKeepsEqualLengthRoutesApart` | `internal/component/bgp/plugins/rr/withdrawal_test.go` | AC-2 | |
| `TestRRWithdrawalKeepsMulticastAddPathApart` | `internal/component/bgp/plugins/rr/withdrawal_test.go` | AC-3 (including Path ID `0` vs non-ADD-PATH) | |
| `TestRRTextPathPairsWithdrawal` | `internal/component/bgp/plugins/rr/withdrawal_test.go` | AC-4 | |
| `TestRouteKeyUsesRegisteredIdentity` | `internal/component/bgp/plugins/rr/routekey/routekey_test.go` | AC-1, AC-2, AC-3 (including `addPath=false, pathID=0` vs `addPath=true, pathID=0`) | |
| `TestFwdKeyValueAndZeroAlloc` | `internal/component/bgp/reactor/fwdkey/fwdkey_test.go` | AC-1, A-5 | |
| `TestFwdBodyKeyDeterminesBuilderInputs` | `internal/component/bgp/reactor/forward_body_test.go` | AC-5 | |
| `TestPeerKeyCanonicalAcrossSpellings` | `internal/component/bgp/configjson/traverse_test.go` | AC-6 | |
| `TestPeerKeyAndGroupKeyStayApart` | `internal/component/bgp/configjson/traverse_test.go` | AC-7 | |
| `TestRoleCanonicalizesNonCanonicalIPv6PeerKeys` | `internal/component/bgp/plugins/role/config_test.go` | AC-6 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ADD-PATH path identifier | 0 to 4294967295 | 4294967295 | N/A (unsigned 32-bit) | N/A (unsigned 32-bit) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rr-withdraw-equal-length-evpn.ci` | `test/plugin/` | AC-2: withdrawal and source down through the reflector, hex assertions on what clients receive | |
| `rpki-per-peer-address-spelling.ci` | `test/plugin/` | AC-6: per-peer RPKI action applies under non-canonical IPv6 address spelling `2001:0DB8::1` | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-rr-departure-equal-length-evpn-frr` | `test/interop/scenarios/` | FRR | two equal-length EVPN routes reflected to FRR; one withdrawn; at the source's departure FRR holds none | |

`bgp-addpath-departure-reflector-frr` proves IPv4 unicast departure, which does
not exercise non-CIDR equal-length EVPN routes; the scenario above proves AC-2
against real FRR.

**Owed research before the scenario is written** (`ai/rules/interop-and-goal-validation.md`):
read, in FRR's source at the version the lab runs, that FRR accepts EVPN routes
from an iBGP route reflector, holds two type-2 routes that differ only in MAC,
prints them in a command the checker can read, and removes each on an
`MP_UNREACH_NLRI` withdrawal; name each function. Until then the scenario's
expectations are unverified.

## Files to Modify
- `internal/component/bgp/plugins/rr/withdrawal.go` - change `withdrawalKey.route` to `routekey.Key`; delete dead `walkUnicastNLRIs`, `walkNLRIsAllocating`, `walkVPNNLRIs`, and `updateWithdrawalMapText`
- `internal/component/bgp/plugins/rr/stored_membership.go` - construct route keys via `routekey.New` in `nativeInventory`; update `putWithdrawalLocked`, `setWithdrawalEntryLocked`, `deleteWithdrawalLocked`, `removeWithdrawalLocked`, `nativeKeyFor`, `sourceMemberLocked`, `lookupNativeLocked`; delete dead `textRouteKey`
- `internal/component/bgp/plugins/rr/rr.go` - change `ownerRouteKey.route` to `routekey.Key`
- `internal/component/bgp/reactor/forward_body.go`, `forward_rs.go`, `reactor_api_forward.go` - replace `fwdBodyCacheKey` and all four literal sites with `fwdkey.Key` and `fwdkey.New`
- `internal/component/bgp/configjson/traverse.go` - make `PeerConfigKey` fields unexported (`addr netip.Addr`, `group string`, `template bool`), add `AddrKey`, read accessors, and canonicalizing `PeerKey` and `LookupPeerConfig`
- `internal/component/bgp/blackholecfg/blackholecfg.go` - replace per-consumer address parsing and `PeerConfigKey` literal with `configjson.PeerKey`
- `internal/component/bgp/plugins/rpki/rpki_config.go`, `rpki_status.go` - delete per-consumer canonicalization in `parsePeerActions`; switch `.Template`/`.ID` reads to `PeerConfigKey` accessors
- `internal/component/bgp/plugins/filter_community/blackhole.go` - use `PeerConfigKey` accessors (`IsTemplate()`, `Addr()`) and remove the dead `PeerConfigKey{ID: dest.Name}` lookup
- `internal/component/bgp/plugins/role/config.go` - canonicalize peer address strings in `extractPeerRoleConfigs`, `nameToIP`, `configuredPeerKeys`, and `extractRoleCapabilities` via `configjson.PeerKey`
- `docs/architecture/core-design.md`, `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/config/syntax.md`, `docs/architecture/wire/nlri.md`, `docs/architecture/update-building.md`, `docs/guide/route-reflection.md` - document the three typed keys and repoint the deleted `walk*` and `fwdBodyCacheKey` anchors to `routekey`, `nativeInventory`, and `fwdkey`

## Files to Create
- `internal/component/bgp/plugins/rr/routekey/routekey.go`, `routekey_test.go` - the route reflector's route identity key type (unexported fields) and `nlrisplit.GetPrefixKey`-backed constructor
- `internal/component/bgp/reactor/fwdkey/fwdkey.go`, `fwdkey_test.go` - the forward body cache key type (unexported fields) and value constructor
- `test/plugin/rr-withdraw-equal-length-evpn.ci`, `test/plugin/rpki-per-peer-address-spelling.ci`
- `test/interop/scenarios/bgp-rr-departure-equal-length-evpn-frr/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no config or RPC change |
| YANG validation constraints | N-A | none |
| YANG custom validators | N-A | none |
| CLI commands/flags | N-A | none |
| CLI grammar (keyword before value) | N-A | none |
| Editor autocomplete | N-A | none |
| Functional test for new RPC/API | N-A | no new RPC |
| Pipe completeness | N-A | no command output change |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Doctor code per problem class | N-A | no new refusal or log line |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | no family added; `rr/routekey` reads existing `nlrisplit` registration |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | defect fix and compiler enforcement over existing capabilities |
| 2 | Config syntax changed? | No | existing IPv6 address syntax; non-canonical spellings now match canonical runtime addresses |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | no registration change |
| 6 | Has a user guide page? | Yes | `docs/guide/route-reflection.md`: update deleted `walkUnicastNLRIs`/`walkVPNNLRIs` anchors and state that source down withdraws every reflected route of every family |
| 7 | Wire format changed? | No | wire encodings unchanged |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc7911.md`, rows backed by the new reflector ADD-PATH tests |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (withdrawal key), `docs/architecture/bgp/structural-forwarding.md` (forward key), `docs/architecture/config/syntax.md` (canonical peer config key), `docs/architecture/wire/nlri.md` (repoint deleted `walkNLRIsAllocating`/`walkVPNNLRIs` anchors), `docs/architecture/update-building.md` (repoint `fwdBodyCacheKey` anchor on line 1491) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Checked with `./le spec citation anchors spec plan/release/fixes/spec-typed-identity-keys.md`. Declared `// Design:` docs modified: `docs/architecture/core-design.md`, `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/config/syntax.md`. Declared `// Design:` docs unaffected: `docs/architecture/buffer-architecture.md` (declared by `reactor_api_forward.go`; unaffected because `fwdkey.Key` stays a zero-allocation value type and changes no buffer ownership or pooling), `docs/guide/rpki.md` (declared by `rpki_status.go`; unaffected because `show bgp rpki status` emits the exact same `"peer"` and `"group"` JSON through `PeerConfigKey` accessors), and `docs/architecture/plugin/rib-storage-design.md` (declared by `stored_membership.go`, `blackholecfg.go`, `rpki_config.go`; unaffected because `withdrawalKey` retains numeric `owner` and `connection` provenance beside `routekey.Key`). Advisory anchored docs modified: `docs/architecture/wire/nlri.md` (repoints deleted `walk*` anchors) and `docs/architecture/update-building.md` (repoints line 1491 `fwdBodyCacheKey` anchor). |
| 17 | Existing docs show config/CLI/API examples for this area? | No | none affected |

## Design presentation

### What changes

Three BGP identity keys move behind compiler-enforced constructors with unexported fields in their own packages (D1 of `spec-single-owner-lifecycle.md`): `rr/routekey.Key` for route-reflector withdrawal tracking, `reactor/fwdkey.Key` for the forward-body cache, and `configjson.PeerConfigKey` (with unexported parsed `netip.Addr` and template fields) for per-peer plugin configuration. In `bgp-rr`, the live wire, text, and stored-replay paths already use `nativeInventory` and `nlrisplit.GetPrefixKey`, so this work replaces the remaining untyped `route string` with `routekey.Key`, deletes the four superseded legacy `walk*`/`updateWithdrawalMapText` functions, and adds functional and FRR interop proof for equal-length EVPN and multicast ADD-PATH departures. In `configjson`, `PeerKey` and `LookupPeerConfig` canonicalize IP addresses once so non-canonical IPv6 spellings (such as `2001:0DB8::1`) resolve consistently across `rpki`, `role` (including Role capability 9 injection), `rib`, `cmd/announce`, and `filter_community`, letting us delete the duplicate per-consumer canonicalizations. In `reactor`, `fwdkey.Key` replaces the four `fwdBodyCacheKey` struct literals with a zero-allocation value constructor and AC-5 proves that the key determines every input `buildFwdBody` reads (or extends the key if an input differs).

### Before and after

1. **Non-canonical IPv6 peer address in config (`2001:0DB8::1`) with `role` or `rib` FlowSpec redirect config:**
   - Before: `configjson.PeerKey` and `role/config.go` stored the raw string `"2001:0DB8::1"`, while runtime lookups (`PeerFilterInfo.Address.String()`, `configjson.PeerSelectors`, `flowSpecRedirectChecked`) queried `"2001:db8::1"`. The lookup missed silently, leaving the peer without its configured Role capability, OTC policy, or FlowSpec redirect override, while `blackholecfg` and `rpki` each carried duplicate `netip.ParseAddr` workarounds.
   - After: `configjson.PeerKey` parses and stores `netip.Addr` (`2001:db8::1`) inside unexported fields on `PeerConfigKey`. Every consumer resolves `"2001:0DB8::1"` and `"2001:db8::1"` to the same key, `role` advertises Capability 9 under `"2001:db8::1"`, and the duplicate canonicalizations in `blackholecfg` and `rpki` plus the dead `PeerConfigKey{ID: dest.Name}` lookup in `filter_community` are deleted.

2. **Route reflector withdrawal route key (`bgp-rr`):**
   - Before: `withdrawalKey.route` and `ownerRouteKey.route` were bare `string` fields inside package `rr` (omitting `addPath` from the route key), and four dead legacy functions (`walkUnicastNLRIs`, `walkNLRIsAllocating`, `walkVPNNLRIs`, `updateWithdrawalMapText`) remained in `withdrawal.go` with the old `WireNLRI.String` family-and-length key code.
   - After: `withdrawalKey.route` and `ownerRouteKey.route` have type `routekey.Key` from `internal/component/bgp/plugins/rr/routekey/`, constructible only via `routekey.New(fam, addPath, pathID, raw, withdraw)` (calling `nlrisplit.GetPrefixKey`), the four dead legacy walks and `textRouteKey` are deleted, and equal-length EVPN and multicast ADD-PATH departures (including Path ID `0` vs non-ADD-PATH) are proven by unit, `.ci`, and FRR interop tests.

3. **Forward body cache key (`reactor`):**
   - Before: Four `fwdBodyCacheKey{...}` struct literals in `forward_rs.go` and `reactor_api_forward.go` constructed cache keys directly inside package `reactor`, with no test pinning that `destCtxID` and `extended` determine `maxMsgSize` and `peer.sendContext()` inside `buildFwdBody`.
   - After: `fwdkey.Key` lives in `internal/component/bgp/reactor/fwdkey/` with unexported fields and an inlined value constructor `fwdkey.New(...)`, a composite literal outside `fwdkey` fails to compile, and `TestFwdBodyKeyDeterminesBuilderInputs` proves that equal keys produce identical builder inputs (or extends `fwdkey.Key` if any input can differ).

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Syntactic ownership gate refusing composite literals under `internal/le/arch` | Rejected by owner decision D1 (`spec-single-owner-lifecycle.md`): syntactic gates do not run the type checker and miss writes through values whose type the file does not declare, whereas unexported fields in a dedicated package are enforced by the Go compiler |
| One shared route identity key type in `internal/core/bgp/nlri` for both `bgp-rr` and `bgp-rs` | `bgp-rs` is already keyed correctly and outside the three cases Thomas scoped on 2026-11-14; the shared identity logic already lives once in `nlrisplit.GetPrefixKey` |
| Keeping `fwdBodyCacheKey` inside package `reactor` with a constructor function | Inside `reactor`, nothing stops a fifth composite literal from bypassing the constructor |
| Creating a brand-new package for `PeerConfigKey` | `internal/component/bgp/configjson` is already its own package imported by every consumer; unexporting its fields enforces constructor-only construction with minimal churn |
| Keeping `recordCreatedPeerLocked` in this spec | Moved to `spec-bgp-one-off-defects.md` under Thomas's 2026-11-14 ruling: it is a running-tree peer NAME built at one site whose defect is a missing collision refusal, not a typed identity key |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- create `rr/routekey/` and `reactor/fwdkey/`, unexport `configjson.PeerConfigKey` fields with constructors and accessors, move every production and test literal site onto constructors, delete the four dead legacy walks in `rr/withdrawal.go` and dead `textRouteKey`, and write the failing wiring tests
   - Tests: `TestRRWithdrawalKeepsEqualLengthRoutesApart`, `TestRRWithdrawalKeepsMulticastAddPathApart`, `TestRRTextPathPairsWithdrawal`, `TestFwdBodyKeyDeterminesBuilderInputs`, `TestPeerKeyCanonicalAcrossSpellings`, `TestRoleCanonicalizesNonCanonicalIPv6PeerKeys`, `test/plugin/rr-withdraw-equal-length-evpn.ci`, `test/plugin/rpki-per-peer-address-spelling.ci`
   - Files: `rr/routekey/`, `reactor/fwdkey/`, `configjson/traverse.go`, `rr/withdrawal.go`, `rr/stored_membership.go`, `rr/rr.go`, `reactor/forward_body.go`, `reactor/forward_rs.go`, `reactor/reactor_api_forward.go`, `blackholecfg/blackholecfg.go`, `rpki/rpki_config.go`, `rpki/rpki_status.go`, `filter_community/blackhole.go`, `role/config.go`
   - Verify: tree compiles with zero literals outside the three key packages (AC-1); new tests fail on the unfixed canonicalization / proof gaps before fixes land
2. **Phase: Route reflector key and proof** -- finalize `rr/routekey.New` on `nlrisplit.GetPrefixKey`, migrate `receive_owner_test.go` off `updateWithdrawalMapText`, complete unit tests, `test/plugin/rr-withdraw-equal-length-evpn.ci`, FRR source research, and interop scenario `bgp-rr-departure-equal-length-evpn-frr` (AC-2, AC-3, AC-4)
3. **Phase: Forward body cache key** -- finalize `reactor/fwdkey` and `TestFwdBodyKeyDeterminesBuilderInputs`; if any builder input differs across equal keys, extend `fwdkey.Key` (or read from `bgpctx.Registry.Get(destCtxID)`); verify inlining and zero allocations with `-gcflags=-m` (AC-5, A-5)
4. **Phase: Per-peer config key** -- canonical `PeerKey`, `AddrKey`, and `LookupPeerConfig` in `configjson/traverse.go`; delete duplicate canonicalizations in `blackholecfg.Parse` and `rpki` `parsePeerActions`; switch `rpki_status.go` and `filter_community/blackhole.go` to accessors and delete `filter_community`'s dead `dest.Name` lookup; canonicalize `role/config.go` (`configs`, `nameToIP`, `configuredPeerKeys`, `CapabilityDecl.Peers`); verify `test/plugin/rpki-per-peer-address-spelling.ci` and unit tests (AC-6, AC-7)
5. **Phase: Docs and RFC annotations** -- update `docs/architecture/core-design.md`, `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/config/syntax.md`, `docs/architecture/wire/nlri.md`, `docs/architecture/update-building.md`, `docs/guide/route-reflection.md`, and `rfc/short/rfc7911.md` with discrimination records

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-7 has an implementation at file and symbol |
| Correctness | The add and delete arms of `updateWithdrawalMapWire`, `textRecords`, and `storedInventory` all build keys through `routekey.New` via `nativeInventory`, differing only in `withdraw` |
| Data flow | No family name or SAFI branch left in `rr/withdrawal.go` or `rr/routekey`; route identity comes solely from `nlrisplit.GetPrefixKey` |
| Rule: performance | `fwdkey.New` is inlined and the fan-out allocates no more than before |
| Rule: no-layering | No second IP canonicalization survives in `blackholecfg`, `rpki`, or `filter_community` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No key literal outside its package | Tree compiles with all fields unexported; grep for `routekey.Key{`, `fwdkey.Key{`, `PeerConfigKey{` finds only each key's own package |
| Route reflector keys solely through `rr/routekey` | `gopls references` on `routekey.New` covers `nativeInventory`, and `walkUnicastNLRIs`, `walkNLRIsAllocating`, `walkVPNNLRIs`, `updateWithdrawalMapText`, `textRouteKey` are absent from the tree |
| Doc anchors current | `./le doc check all` and `./le spec citation anchors spec plan/release/fixes/spec-typed-identity-keys.md` pass |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A malformed NLRI that `nlrisplit.GetPrefixKey` refuses returns an error from `routekey.New` and is never keyed by a fallback string; an unparseable IP address in `PeerKey` returns `false` and never stores an unreachable literal key |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood, back to research |
| Lint failure | Fix inline. If architectural, back to design |
| Functional test fails | Check the AC: wrong AC, back to design; correct AC, back to implementation |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- At HEAD (2026-11-26), `bgp-rr`'s live wire, text, and stored-replay paths already converged onto `nativeInventory` (`stored_membership.go`), which calls `nlrisplit.GetPrefixKey` and emits native hex departures. What remains in `bgp-rr` is compiler enforcement via `rr/routekey`, deletion of the four superseded legacy walks (`walkUnicastNLRIs`, `walkNLRIsAllocating`, `walkVPNNLRIs`, `updateWithdrawalMapText`) and `textRouteKey`, and end-to-end EVPN/multicast ADD-PATH proof.
- `rib/rib_flowspec_redirect.go` and `cmd/announce/blackhole_agreement.go` joined `configjson.KeyFor`/`LookupPeerConfig` since 2026-11-14 and automatically gain canonical IPv6 matching once `configjson.PeerKey` and `LookupPeerConfig` canonicalize.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| key-omits-a-fact-the-builder-uses | 2026-10-03 | RR `walkNLRIsAllocating`: every non-CIDR, non-VPN family keyed by family and length; multicast ADD-PATH keyed without the identifier |
| key-omits-a-fact-the-builder-uses | 2026-10-10 | forward body cache `fwdBodyCacheKey` |
| gate-excludes-part-of-its-population | 2026-08-14 | `configjson.PeerKey` keys raw operator text |

## Rows found already fixed

| Class file | Date | Surface (short) | Evidence |
|------------|------|-----------------|----------|
| key-omits-a-fact-the-builder-uses | 2026-10-02 | ospf GR helper keyed by Advertising Router | `50044040c5`: `graceRestarterV4` (`internal/plugins/ospf/gr_helper.go`) names X by the interface address TLV on broadcast, NBMA and P2MP; journal row marked fixed 2026-11-14 |
| index-key-derived-from-mutable-field | 2026-10-05 | rs `appendOpaqueRecords`/`recordKey` | `recordKey` keys on `nativeKey`, set when the record is built; the journal row already says fixed |

## Rows moved to the spec whose mechanism fits (2026-11-14)

| Class file | Date | Surface (short) | Now owned by |
|------------|------|-----------------|--------------|
| key-omits-a-fact-the-builder-uses | 2026-10-11 | cli `computeDiff` path-less set comparison | spec-web-cli-config-one-off-defects (closed: fixed in `0d3b9e36c7`) |
| key-omits-a-fact-the-builder-uses | 2026-10-13 | `show bfd session address` first map hit | `plan/release/fixes/spec-ospf-isis-bfd-vrrp-one-off-defects.md` |
| index-key-derived-from-mutable-field | 2026-08-10 | l2tp `discardTunnelLocked` | spec-l2tp-session-identity-and-publish-stall (closed: fixed in `59f29eb1df`) |
| index-key-derived-from-mutable-field | 2026-08-11 | iface dhcp route removal metric | `plan/release/fixes/spec-network-services-one-off-defects.md` |
| identifier-reused-after-its-owner-is-gone | 2026-08-19 | iface vpp GRE, IPIP, VXLAN create | `plan/release/fixes/spec-network-services-one-off-defects.md` |
| identifier-reused-after-its-owner-is-gone | 2026-08-18 | netlink monitor late event name | `plan/release/fixes/spec-network-services-one-off-defects.md` |
| two-owners-share-one-name | 2026-10-10 | `recordCreatedPeerLocked` overwrites an operator's peer name | `plan/release/fixes/spec-bgp-one-off-defects.md` |
| declared-format-contradicts-payload | 2026-10-20 | adj-rib-in `handleReceived` | `plan/release/fixes/spec-bgp-one-off-defects.md` |
| gate-excludes-part-of-its-population | 2026-09-02 | `countPeerUpBarrier` by process name | `plan/release/fixes/spec-bgp-one-off-defects.md` |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Compiler enforcement: unexported fields, package of its own (D1) | An ownership gate refusing composite literals (the earlier design of this spec) | D1: no gate under `internal/le/arch` runs the type checker, so a syntactic check misses writes through values whose type the file does not declare |
| Route key package inside `bgp-rr` (`rr/routekey`) | One shared route identity type in `internal/core/bgp/nlri` used by rr and rs | rs is keyed right today and is outside the three cases Thomas named; the identity fact is already declared once in `nlrisplit.GetPrefixKey`, which both read |
| Forward key package under `reactor` (`reactor/fwdkey`) | Keeping the struct in `reactor` with a constructor | In its own package a composite literal does not compile; inside `reactor` nothing stops a fifth literal site |
| Peer config key stays in `configjson` | A new package | `configjson` is already a package of its own and every consumer imports it; unexporting its fields enforces constructor-only construction with minimal churn |
| `recordCreatedPeerLocked` leaves this spec | Keep it as part of the peer config key | It is a NAME in the running tree, built at one site; the defect is a missing collision refusal, which a type does not give |

## Known Limitations
- None. Every row this spec owned is owned here, by the spec named above, or fixed.

## RFC Documentation (Scope: protocol)

Add `// RFC 7911 Section 2: "<quoted text>"` above `rr/routekey`'s Path Identifier handling, and section references at its callers.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 7911 Sections 2 and 5 | Under ADD-PATH, each path is identified by `(family, ADD-PATH presence, Path Identifier, registered route key)` across all address families, including multicast and EVPN; identifier zero with ADD-PATH stays distinct from non-ADD-PATH, and withdrawing one path removes only that `(prefix, Path Identifier)` pair | Non-ADD-PATH sessions keep `addPath=false` and withdraw by registered route identity | `TestRouteKeyUsesRegisteredIdentity`, `TestRRWithdrawalKeepsMulticastAddPathApart`, `TestRRWithdrawalKeepsEqualLengthRoutesApart` | `TestRRWithdrawalKeepsEqualLengthRoutesApart`, `rr-withdraw-equal-length-evpn.ci`, `bgp-rr-departure-equal-length-evpn-frr` |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] No code snippets
- [ ] FRR source read for the interop scenario (owed research above)

### Goal Gates (MUST pass)
- [ ] AC-1 to AC-7 each proven by a named test
- [ ] Wiring Test table complete
- [ ] Scoped runs: `./le test unit pkg` for `rr`, `rr/routekey`, `reactor`, `reactor/fwdkey`, `configjson`, `blackholecfg`, `rpki`, `role`, `rib`, `filter_community`, `cmd/announce`; the two `.ci` tests through `./le test bgp plugin`; `INTEROP_SCENARIO=bgp-rr-departure-equal-length-evpn-frr ./le test integration interop`; `./le rfc check`
- [ ] Interop scenario green, with the red phase recorded

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] **Commit A:** code + tests + docs + edited spec + journal rows
- [ ] **Commit B:** remove the spec

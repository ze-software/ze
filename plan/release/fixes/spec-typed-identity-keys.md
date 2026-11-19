# Spec: typed-identity-keys

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Three keys are built at several sites from whatever fields each site holds. The route reflector's withdrawal key collides for routes the RFC tells apart, so a source going down leaves reflected routes at its clients; the per-peer config key stores the operator's spelling, so a peer written in a non-canonical form silently gets the global policy; the forward body cache key leaves out two builder inputs that nothing proves it determines. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Thomas's ruling, 2026-11-14** (his reply "ok" to the proposal put to him):
"Narrow it to the three real key cases (forwarded-message cache, route reflector
withdrawal, peer config key), enforced by the compiler as you ruled. Move the
other open defects into the existing one-off fix specs for BGP and for
OSPF/IS-IS/BFD/VRRP, and the L2TP one into the L2TP identity spec that already
asks for it."

"As you ruled" is decision D1 of `plan/release/fixes/spec-single-owner-lifecycle.md`:
no annotation gate and no typed analyzer; where a single writer must be
enforced by the compiler, the state lives in a type with unexported fields in a
package of its own, so the constructor is the only writer.

**The problem.** Each of three keys is built by struct literal or string
concatenation at more than one site, each site from the fields it holds:

| Key | Sites that build it today | What goes wrong |
|-----|---------------------------|-----------------|
| Route reflector withdrawal key | `walkUnicastNLRIs`, both arms of `walkNLRIsAllocating`, `walkVPNNLRIs`, both arms of `updateWithdrawalMapText` (`internal/component/bgp/plugins/rr/withdrawal.go`) | Outside VPN and labeled unicast the key is `WireNLRI.String`, a family and byte count: two routes of one family and equal length are one key |
| Per-peer config key | `configjson.PeerKey`, `GroupKey`, `LookupPeerConfig` (`internal/component/bgp/configjson/traverse.go`); `blackholecfg.Parse`; `filter_community` `blackholeAgreements.lookup`; `rpki` `parsePeerActions`; and `role`'s own string key (`role/config.go`) | `PeerKey` and `role` key the operator's raw text; `blackholecfg` and `rpki` canonicalize again on their own |
| Forward body cache key | four `fwdBodyCacheKey` literals: two in `forward_rs.go`, two in `reactor_api_forward.go` | `buildFwdBody` reads `maxMsgSize` and the destination's send context, which the key does not hold; nothing proves the key determines them |

**Goal.** Each key is a type with unexported fields in a package of its own,
built only by its constructors, which take every identifying fact. A literal
outside the package does not compile. The route reflector keys each route by
the identity its family registers, so a withdrawal pairs with its own
announcement and a source down withdraws every route it reflected.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/bgp-engine.md`, `ai/doors/bgp-plugins.md`
  → Constraint: a plugin never imports another plugin's implementation; `internal/core/bgp` imports no component.
  → Constraint: the forward path allocates nothing per destination where a value type serves (`ai/rules/performance.md`).
- [ ] `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/bgp/fanout-dedup.md` (`forward_body.go` declares the first as its design)
  → Constraint: the forward cache key stays a fixed-size comparable value; the cache is per fan-out and on the hot path.
- [ ] `docs/architecture/core-design.md` (route reflector withdrawal tracking; `withdrawal.go` declares it as its design)
- [ ] `docs/architecture/wire/nlri.md` (family-specific route identity, `nlrisplit`)
  → Decision: a family's route identity is the `PrefixKeyFunc` it registers with `nlrisplit`; no consumer derives its own.
- [ ] `docs/architecture/config/syntax.md` (`traverse.go` declares it as its design)
- [ ] `docs/architecture/plugin/rib-storage-design.md` (`rib_blackhole_config.go` declares it as its design)
  → Constraint: the page states nothing about the per-peer key (searched for `blackhole` and `PeerConfigKey`, no hit), so it needs no edit; re-check at implementation.
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
- Without ADD-PATH the defect remains for opaque families: the identity of an EVPN or FlowSpec route is its registered `nlrisplit` key, never its length.

## Current Behavior (MANDATORY)

Read at the producer on 2026-11-14.

**Source files read:**
- [ ] `internal/component/bgp/plugins/rr/withdrawal.go` - `updateWithdrawalMapWire`, `walkUnicastNLRIs`, `walkNLRIsAllocating`, `walkVPNNLRIs`, `updateWithdrawalMapText`
- [ ] `internal/component/bgp/plugins/rr/rr.go` - `sendPeerDown`, `withdrawalGroup`, `nlriDelCmd`
- [ ] `internal/core/bgp/nlri/wire.go` - `WireNLRI.String`, `RoutePrefix`, `Bytes`
- [ ] `internal/core/bgp/nlri/inet.go` - `INET.String`, `Key`, `Bytes`
- [ ] `internal/component/bgp/wireu/mpwire.go` - `parseNLRISection`
- [ ] `internal/core/bgp/nlri/nlrisplit/prefix_key.go` - `GetPrefixKey`, `RouteCIDR`, `KeysByCIDR`, `keyOpaque`
- [ ] `internal/component/bgp/plugins/rs/server_inventory.go` - `recordKey`, `nativeKey`
- [ ] `internal/component/bgp/reactor/forward_body.go` - `fwdBodyCacheKey`, `buildFwdBody`
- [ ] `internal/component/bgp/reactor/forward_path_id.go` - `fwdRegenerateRawPathIDs`, `fwdPathIDMemo.framed`
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go` - the `maxMsgSize` fact
- [ ] `internal/component/bgp/configjson/traverse.go` - `PeerConfigKey`, `PeerKey`, `GroupKey`, `KeyFor`, `LookupPeerConfig`
- [ ] `internal/component/bgp/blackholecfg/blackholecfg.go` - `Parse`
- [ ] `internal/component/bgp/plugins/rpki/rpki_config.go` - `parsePeerActions`
- [ ] `internal/component/bgp/plugins/filter_community/blackhole.go` - `newBlackholeAgreements`, `lookup`
- [ ] `internal/component/bgp/plugins/rib/rib_blackhole_config.go` - `parseBlackholeConfig`
- [ ] `internal/component/bgp/plugins/role/config.go`, `role/role.go` - the per-peer key, `getFilterConfig`
- [ ] `internal/component/bgp/reactor/peer_settings.go` - `PeerSettings.PeerKey`
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `recordCreatedPeerLocked`, `peerConfigNameFor`

| Producer | What it does today |
|----------|--------------------|
| `walkNLRIsAllocating` | VPN goes to `walkVPNNLRIs`, keyed by the registered RD and prefix key, the path identifier and ADD-PATH presence. Every other family is keyed by family and `nlriKey(n.String())`; labeled unicast adds the path identifier under ADD-PATH |
| `parseNLRISection` | IPv4 and IPv6 unicast and multicast become `INET`; every other family becomes one `WireNLRI` per NLRI (one carrier for the whole remainder when the family has no splitter) |
| `WireNLRI.String` | A CIDR-keyed family (labeled unicast) answers its prefix. Every other family answers `wire[<family>](<N> bytes)`, N being `len(data)`, which counts the 4-octet path identifier under ADD-PATH |
| `INET.String` | The prefix alone, no path identifier |
| `sendPeerDown` (`rr.go`) | A text-form entry is withdrawn through `nlriDelCmd`, the stored string as the NLRI text; a wire-form entry with `update hex nlri <family> [addpath] del <hex>` |
| `updateWithdrawalMapText` | The fork-mode text path keys family and `nlriKey(text)` in the same map |
| `buildFwdBody` | Reads `maxMsgSize` and `peer.sendContext()` beside the key fields. `maxMsgSize` is `MaxMessageLength(UPDATE, extendedMsg)` (`peer_forward_facts.go`), and `extended` is in the key. The path identifier rewrite reads only the ADD-PATH flags of the send context and allocates identifiers per SOURCE (`fwdPathIDMemo.framed`), so both look determined by the key; no test pins it |
| `PeerKey` | Returns `PeerConfigKey{ID: ip}` with `ip` exactly as `PeerRemoteIP` read it from the document; the name branch parses the name only to test it and keys the unparsed name |
| `LookupPeerConfig` | Builds `PeerConfigKey{ID: addr}` and `PeerConfigKey{ID: name}` from the reader's strings |
| `blackholecfg.Parse`, `parsePeerActions` | Each canonicalizes the address itself (parse, then `String`) before storing |
| `role` per-peer key (`config.go`) | Keys its string map by the raw `PeerRemoteIP` text, or the peer name when it parses as an address; the readers pass `PeerFilterInfo.Address`. The same map is the capability selector index, so its keys cross the plugin boundary as strings |
| `filter_community` `lookup` | Looks up `byTemplate` with a non-template literal key built from the peer name, which by its own comment always misses |

**Exact width of the route reflector defect.**

| Family set | Key today | Collision | Consequence |
|------------|-----------|-----------|-------------|
| IPv4 and IPv6 unicast | prefix, plus path identifier under ADD-PATH | none | correct |
| VPN (SAFI 128) | registered RD and prefix key, path identifier, ADD-PATH presence | none | correct |
| Labeled unicast | prefix, plus path identifier under ADD-PATH | none | correct |
| IPv4 and IPv6 multicast | prefix only | under ADD-PATH, every path of one prefix is one key | a withdrawal of one path deletes the entry of all; a second path overwrites the first; source down withdraws one |
| Every other family (EVPN, FlowSpec, FlowSpec VPN, MVPN, MUP, VPLS, RTC, BGP-LS, SR Policy and the rest) | `wire[<family>](<N> bytes)` | any two NLRIs of one family and one length, with or without ADD-PATH | a later announcement overwrites an earlier one; a withdrawal of one deletes another; source down sends at most one withdrawal per family and length |
| The same set, at source down | the stored `String` output, sent as NLRI text | every entry | the command carries `wire[...]` as NLRI text, which names no route (the producer is read; the parser's refusal is unverified) |

**Behavior to preserve:**
- The forwarded UPDATE bytes, and the zero-copy same-context forward.
- Unicast, VPN and labeled unicast withdrawal pairing in the route reflector, and its source-down command forms for those families.
- A dynamic group's template key stays apart from a peer of the same name (`PeerConfigKey` comment, the `Template` namespace).
- `rpki` status output naming each per-peer subject; `role`'s capability selector strings.

**Behavior to change:**
- The three keys become constructor-built types; every literal site moves to a constructor.
- The route reflector keys every family by its registered identity and withdraws non-CIDR families in native form.
- The per-peer config key holds the parsed address; `PeerKey` canonicalizes; `role` takes its address text from the same constructor; the two per-consumer canonicalizations are deleted.
- The forward cache key is either proven to determine `maxMsgSize` and the path identifier rewrite, or extended to hold what it does not determine.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| NLRI identity forms | `internal/core/bgp/nlri/nlri.go` `NLRI.Bytes`; `inet.go` `INET.Key`; `plan/learned/DESIGN-HISTORY.md` "`Bytes()` (identity/RIB key) and `Pack()`/`WriteTo()` (wire) are separate" | `Bytes` is the identity form; `INET.Key` the bare CIDR; `String` is display only. `WireNLRI.Bytes` returns the data with the path identifier when ADD-PATH framed it, unlike the interface comment | Extends: the route reflector key reads `WireNLRI.Bytes` split by `nlri.SplitPathID`, as `walkVPNNLRIs` does, and never `String` |
| Registered route identity | `internal/core/bgp/nlri/nlrisplit/prefix_key.go` `GetPrefixKey`, `RouteCIDR`, `KeysByCIDR` | Each family registers its route key; the default keeps the whole NLRI; CIDR families answer their prefix | Extends: the route reflector constructor calls it for every family, as `walkVPNNLRIs` and `rs` already do; no family list in the plugin |
| rs withdrawal identity | `internal/component/bgp/plugins/rs/server_inventory.go` `recordKey`, `nativeKey`, `withdrawalKey` | One function derives the key for the add and the delete arm, from the registered key, path identifier and ADD-PATH presence | Does not apply to rs, which is keyed right (its row is fixed). It is the model the route reflector type follows. Sharing one type across rr and rs is set aside under Key Design Decisions |
| `PeerSettings.PeerKey` | `internal/component/bgp/reactor/peer_settings.go` | The reactor's peer map key, a `netip.AddrPort` value | Does not apply: a different key (the session, with its port), already a value built by one function |
| `configjson.PeerKey`, `GroupKey`, `KeyFor`, `LookupPeerConfig` | `internal/component/bgp/configjson/traverse.go` | Build the per-peer config key from raw config text or reader strings; `Template` separates the group namespace | Extends: these become the type's constructors in the package that already holds it; the fields go unexported and the address is held parsed |
| Per-consumer canonicalization | `blackholecfg.Parse`; `rpki` `parsePeerActions` | Each re-parses and re-renders the address before storing | Replaces: deleted once `PeerKey` canonicalizes (no layering) |
| `role` string key and capability selector | `role/config.go`; `configjson.CapabilityGroupKey` | A string map that is also the selector set published across the plugin boundary | Extends: the address text comes from the key's canonical accessor; the map stays string-keyed because the selector crosses the boundary as a string |
| `peerConfigNameFor`, `recordCreatedPeerLocked` | `internal/component/bgp/reactor/reactor_api.go` | Derive and write the running-tree peer NAME of a peer `create bgp peer` made | Does not apply: a name in the running tree's peer list, built at one site; its collision with an operator's name is a missing refusal. Its row moved to `spec-bgp-one-off-defects.md` |
| Forward facts snapshot | `internal/component/bgp/reactor/peer_forward_facts.go` | Holds `sendCtxID`, `extendedMsg` and the derived `maxMsgSize` per destination | Extends: the forward key constructor takes the facts that identify a body; `maxMsgSize` stays derived from `extendedMsg` |
| One type per lifecycle state | `docs/contributing/ze-go-style.md`, "One type per lifecycle state"; D1 | A type with private state is the first design to reach for | Extends: applied to three identities |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A received UPDATE reaching `bgp-rr` (raw message through DirectBridge, or the text event when the plugin runs forked).
- A received UPDATE fanned out by the reactor's forward path.
- The config JSON each BGP plugin reads at configure and reload.

### Transformation Path
1. Route reflector: each NLRI of each section is turned into one route key by the key package's constructor, from the family, the native bytes, ADD-PATH presence and the direction; the map stores the native form for the source-down withdrawal.
2. Route reflector, source down: each stored entry is withdrawn in the form it was stored (prefix text for unicast, native hex for every other family).
3. Forward: per destination, the constructor builds the key from the destination context id, the wire and the facts snapshot; equal keys share one built body.
4. Config: the traversal builds each peer's key through `PeerKey`, which parses the address; every runtime reader builds its lookup key from its `netip.Addr` through the address constructor.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine to `bgp-rr` | raw message through DirectBridge; text event when forked | No |
| `bgp-rr` to engine | `update hex` and `update text` commands at source down | No |
| Config to each plugin | config JSON, parsed in the plugin's own process | No |

### Changed-Wire Census

**Search evidence:** scope `internal/component/bgp/` and `test/`; terms `nlriDelCmd`, `update hex nlri`, `WireForm`, `ParseUpdateWire`, `withdrawalInfo`. Positive control: `walkVPNNLRIs` stores `WireForm` and `sendPeerDown` writes `update hex nlri <family> [addpath] del <hex>` for it, read 2026-11-14. The forward cache key and the per-peer config key are process-local map keys: no wire carries either. `role`'s capability selector strings cross the plugin boundary and keep their form (the canonical address text an operator's canonical spelling already produces).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Source-down withdrawal command | writer | `internal/component/bgp/plugins/rr/rr.go` `sendPeerDown`, `nlriDelCmd` | text form for every non-wire entry, hex form for wire entries | change: non-CIDR, non-VPN families move from the text form to the hex form VPN already uses |
| Source-down withdrawal command | reader | `internal/component/bgp/plugins/cmd/update/update_wire.go` `ParseUpdateWire` | already parses the hex form for VPN | preserve: no grammar change |
| Source-down withdrawal command | reader | `internal/component/bgp/plugins/cmd/update/update_text_nlri.go` | receives `wire[...]` text today for those families | preserve: it stops receiving that text |
| Withdrawal UPDATE to reflector clients | writer | the reactor send path the `update hex` command reaches | builds MP_UNREACH_NLRI from the parsed command | preserve the encoding; the content gains the withdrawals owed |
| Withdrawal UPDATE to reflector clients | reader | the client router, outside the repository (FRR in the interop scenario) | receives no withdrawal, or the wrong one, today | change in content only |
| Forwarded UPDATE | writer | `internal/component/bgp/reactor/forward_body.go` `buildFwdBody` | key change only | preserve: bytes unchanged unless AC-5 finds an input the key does not determine; then the cache stops sharing a body it must not share |
| Forwarded UPDATE | reader | the destination peer, outside the repository | unchanged | preserve |

### Integration Points
- `nlrisplit.GetPrefixKey`, `RouteCIDR`, `KeysByCIDR`, `nlri.SplitPathID`: the route reflector constructor.
- `peerForwardFacts` (`peer_forward_facts.go`): the forward key constructor's inputs.
- `configjson` callers: `rpki`, `role`, `rib`, `filter_community`, `blackholecfg`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | each key is still built where it is today, through a constructor |
| No unintended coupling | Yes | the forward key package is imported by `reactor` only; the route key package by `bgp-rr` only; `configjson` keeps its importers |
| No duplicated functionality | Yes | the route identity is the `nlrisplit` registration; the two per-consumer canonicalizations are deleted |
| Zero-copy preserved where applicable | Yes | the forward key stays a fixed-size comparable value returned by value; the constructor allocates nothing |
| Registration over hardcoding, outbound | Yes | no family list: the constructor asks the `nlrisplit` registry |
| Registration over hardcoding, inbound | Yes | searched `withdrawal.go` for family branches: the `SAFIVPN` and `SAFIMPLSLabel` branches go, the identity derives from `GetPrefixKey` and `KeysByCIDR` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Equal destination context ids mean equal ADD-PATH send flags | `fwdRegenerateRawPathIDs` reads only `ctx.AddPath`; the context registry dedups by content | the key must carry the send flags | AC-5 test | unvalidated |
| A-2 | `maxMsgSize` is a function of `extendedMsg` alone on the forward path | `peer_forward_facts.go`, `MaxMessageLength(UPDATE, extendedMsg)` | the key must carry the size | AC-5 test | unvalidated |
| A-3 | Every family the route reflector can receive has a splitter and a registered key, or arrives as one carrier the default opaque key keeps whole | `parseNLRISection`, `GetPrefixKey` default `keyOpaque` | an unsplit family stays one carrier: distinct by bytes, never by length | unit over an unsplit family | unvalidated |
| A-4 | `update hex` withdraws every family `ParseUpdateWire` accepts in MP_UNREACH | VPN uses it today | the source-down withdrawal of an opaque family fails | functional test, AC-2 | unvalidated |
| A-5 | A constructor returning a small struct by value is inlined and allocates nothing | Go escape analysis | a hot-path allocation per destination | `-gcflags=-m` on the reactor package; allocation figure of the forward benchmark | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Unexported `PeerConfigKey` fields break test literals in nine files | compile errors in tests | tests build keys through the constructors; no test assertion changes |
| R-2 | A family's registered key differs between announcement and withdrawal framing | a withdrawal misses its announcement | the constructor takes the direction, as `GetPrefixKey` and `RouteCIDR` do |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Route reflector clients keep or lose routes at a source down; a per-peer rpki, role or blackhole setting applies to the wrong peer or none; a body built for one destination reaches another |
| How is it reverted? | One commit per key; each is independent |
| Who else touches this path? | `spec-bgp-one-off-defects.md` (reactor and rr rows), the forward pool work in the reactor |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A source announces two EVPN routes of equal length through `bgp-rr`, withdraws one, then goes down | → | route key constructor, `walkNLRIsAllocating`, `sendPeerDown` | `test/plugin/rr-withdraw-equal-length-evpn.ci` |
| Config with the peer address `2001:0DB8::1` and a per-peer rpki action | → | `configjson.PeerKey`, `LookupPeerConfig` | `test/plugin/rpki-per-peer-address-spelling.ci` |
| A fan-out to two destinations sharing one context id | → | forward key constructor, `buildFwdBody` | `TestFwdBodyKeyDeterminesBuilderInputs` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The tree after the change | Each of the three key types has only unexported fields, lives in its own package, and every site builds it through a constructor; the tree compiles | Tests inside each key's own package, which may build values directly |
| AC-2 | Two routes of one non-CIDR, non-VPN family and equal length from one source through `bgp-rr`, one withdrawn | The other stays reflected and stays in the withdrawal map; at the source's down each client receives a withdrawal of every route still held, in native form | VPN, labeled and unicast entries keep their present forms, already correct |
| AC-3 | Two ADD-PATH paths of one multicast prefix through `bgp-rr`, one withdrawn | The other stays; at the source's down both identifiers are withdrawn | none: the RFC 7911 identity covers every family |
| AC-4 | The same two routes through the fork-mode text path | The text path keys through the same package and pairs each withdrawal with its announcement | none |
| AC-5 | Two destinations sharing a context id but built from different peers | Either a test proves the context id and the extended flag determine `maxMsgSize` and the path identifier rewrite, so the shared body is correct, or the key holds the input that differs and the bodies are distinct | the shared body when the proof holds: sharing it is the cache's purpose |
| AC-6 | One peer address written in a non-canonical spelling in config (`2001:0DB8::1`) while the session reports `2001:db8::1` | One key; `rpki`, `role`, the rib blackhole config and `filter_community` each find the per-peer setting | none |
| AC-7 | A peer and a dynamic group with one name | Two keys; a lookup for the peer never answers with the group's template | the group key, which answers for peers built from its template |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An operator's EVPN source goes down behind the route reflector | wire, `bgp-rr` withdrawal map, source-down `update hex` command, client | `rr-withdraw-equal-length-evpn.ci`, interop `bgp-rr-departure-equal-length-evpn-frr` |
| 2 | An operator writes an IPv6 peer address with capitals or leading zeros | config, `configjson`, plugin lookup | `rpki-per-peer-address-spelling.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRRWithdrawalKeepsEqualLengthRoutesApart` | `internal/component/bgp/plugins/rr/withdrawal_test.go` | AC-2 | |
| `TestRRWithdrawalKeepsMulticastAddPathApart` | `internal/component/bgp/plugins/rr/withdrawal_test.go` | AC-3 | |
| `TestRRTextPathPairsWithdrawal` | `internal/component/bgp/plugins/rr/withdrawal_test.go` | AC-4 | |
| `TestRouteKeyUsesRegisteredIdentity` | the route key package | AC-2, AC-3 | |
| `TestFwdBodyKeyDeterminesBuilderInputs` | `internal/component/bgp/reactor/forward_body_test.go` | AC-5 | |
| `TestPeerKeyCanonicalAcrossSpellings` | `internal/component/bgp/configjson/traverse_test.go` | AC-6 | |
| `TestPeerKeyAndGroupKeyStayApart` | `internal/component/bgp/configjson/traverse_test.go` | AC-7 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ADD-PATH path identifier | 0 to 4294967295 | 4294967295 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rr-withdraw-equal-length-evpn.ci` | `test/plugin/` | AC-2: withdrawal and source down through the reflector, hex assertions on what clients receive | |
| `rpki-per-peer-address-spelling.ci` | `test/plugin/` | AC-6: the per-peer action applies under the non-canonical spelling | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-rr-departure-equal-length-evpn-frr` | `test/interop/scenarios/` | FRR | two equal-length EVPN routes reflected to FRR; one withdrawn; at the source's departure FRR holds none | |

`bgp-addpath-departure-reflector-frr` exists and proves the IPv4 unicast
departure (`walkUnicastNLRIs`), which this spec does not change; it never
reaches `walkNLRIsAllocating`, so it does not prove AC-2. The planned
`bgp-rr-addpath-withdraw-frr` of the earlier design is replaced by the scenario
above, because the collision does not need ADD-PATH.

**Owed research before the scenario is written** (`ai/rules/interop-and-goal-validation.md`):
read, in FRR's source at the version the lab runs, that FRR accepts EVPN routes
from an iBGP route reflector, holds two type-2 routes that differ only in MAC,
prints them in a command the checker can read, and removes each on an
MP_UNREACH withdrawal; name each function. Until then the scenario's
expectations are unverified.

## Files to Modify
- `internal/component/bgp/plugins/rr/withdrawal.go` - every key site through the route key package; the two family branches go
- `internal/component/bgp/plugins/rr/rr.go` - `sendPeerDown` reads the stored native form for every non-CIDR family
- `internal/component/bgp/reactor/forward_body.go`, `forward_rs.go`, `reactor_api_forward.go` - the forward key through its constructor
- `internal/component/bgp/configjson/traverse.go` - unexported fields, canonicalizing `PeerKey`, address constructor, `LookupPeerConfig` over `netip.Addr`
- `internal/component/bgp/blackholecfg/blackholecfg.go`, `internal/component/bgp/plugins/rpki/rpki_config.go`, `rpki_status.go`, `internal/component/bgp/plugins/filter_community/blackhole.go`, `internal/component/bgp/plugins/role/config.go`, `internal/component/bgp/plugins/rib/rib_blackhole_config.go` - constructors and accessors; the two canonicalizations removed
- `docs/architecture/core-design.md`, `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/config/syntax.md`, `docs/guide/route-reflection.md` - the key each states

## Files to Create
- `internal/component/bgp/plugins/rr/routekey/` - the route reflector's route key type and constructors, a package of its own inside the plugin
- `internal/component/bgp/reactor/fwdkey/` - the forward body key type and constructor
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
| Pipe completeness | N-A | no command output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Doctor code per problem class | N-A | no new refusal or log line |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | no family added; the route key reads the existing registration |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | a defect fix |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | no registration change |
| 6 | Has a user guide page? | Yes | `docs/guide/route-reflection.md`: a source down withdraws every reflected route of every family |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc7911.md`, the rows the route reflector tests back |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `core-design.md` (withdrawal key), `structural-forwarding.md` (forward key), `config/syntax.md` (peer config key) |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived at implementation by `./le spec citation anchors spec plan/release/fixes/spec-typed-identity-keys.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the two new packages and the `configjson` field change, every site moved to a constructor with today's behaviour kept; the wiring tests written red
   - Tests: the three Wiring Test rows
   - Files: `rr/routekey/`, `reactor/fwdkey/`, `configjson/traverse.go`, every literal site
   - Verify: the tree compiles with no literal outside the packages (AC-1); the wiring tests fail on the defects
2. **Phase: route reflector identity** -- the constructor derives the registered identity for every family; native storage and native source-down withdrawal for non-CIDR families (AC-2 to AC-4)
3. **Phase: forward key** -- the AC-5 proof test; if it fails, the key gains the input that differs (AC-5)
4. **Phase: peer config key** -- canonical `PeerKey`, the address constructor, `role` on the canonical text, the two canonicalizations deleted (AC-6, AC-7)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Correctness | The add and delete arms of every route reflector walk call one constructor with the same facts, differing only in direction |
| Data flow | No family name or SAFI branch left in `withdrawal.go`; the identity comes from `nlrisplit` |
| Rule: performance | The forward key constructor is inlined and the fan-out allocates no more than before |
| Rule: no-layering | No second canonicalization survives in a consumer |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No key literal outside its package | the tree compiles with the fields unexported; a search for the type names followed by a brace finds only the key packages |
| Route reflector keys through the package | `gopls references` on the constructors lists every walk in `withdrawal.go` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A malformed NLRI the registered key refuses is logged and skipped, never keyed by a fallback string |

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

- The route reflector defect is wider than ADD-PATH: an opaque family's key is
  its byte count, so it collides without ADD-PATH too, and its source-down
  withdrawal text names no route.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| key-omits-a-fact-the-builder-uses | 2026-10-03 | RR `walkNLRIsAllocating`: every non-CIDR, non-VPN family keyed by family and length; multicast ADD-PATH keyed without the identifier |
| key-omits-a-fact-the-builder-uses | 2026-10-10 | forward body cache `fwdBodyCacheKey` |
| gate-excludes-part-of-its-population | 2026-08-14 | `configjson.PeerKey` keys raw operator text |

## Rows found already fixed

| Class file | Date | Surface (short) | Evidence |
|------------|------|-----------------|----------|
| key-omits-a-fact-the-builder-uses | 2026-10-02 | ospf GR helper keyed by Advertising Router | 50044040c5: `graceRestarterV4` (`internal/plugins/ospf/gr_helper.go`) names X by the interface address TLV on broadcast, NBMA and P2MP; journal row marked fixed 2026-11-14 |
| index-key-derived-from-mutable-field | 2026-10-05 | rs `appendOpaqueRecords`/`recordKey` | `recordKey` keys on `nativeKey`, set when the record is built; the journal row already says fixed |

## Rows moved to the spec whose mechanism fits (2026-11-14)

| Class file | Date | Surface (short) | Now owned by |
|------------|------|-----------------|--------------|
| key-omits-a-fact-the-builder-uses | 2026-10-11 | cli `computeDiff` path-less set comparison | `plan/release/fixes/spec-web-cli-config-one-off-defects.md` |
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
| Route key package inside `bgp-rr` | One shared route identity type in `internal/core/bgp/nlri` used by rr and rs | rs is keyed right today and is outside the three cases Thomas named; the identity fact is already declared once, in the `nlrisplit` registration, which both read |
| Forward key package under `reactor` | Keeping the struct in `reactor` with a constructor | in its own package a literal does not compile; inside `reactor` nothing stops a fifth literal site |
| Peer config key stays in `configjson` | A new package | `configjson` is already a package of its own and every consumer imports it; only the fields change |
| `recordCreatedPeerLocked` leaves this spec | Keep it as part of the peer config key | it is a NAME in the running tree, built at one site; the defect is a missing collision refusal, which a type does not give |

## Known Limitations
- None. Every row this spec owned is owned here, by the spec named above, or fixed.

## RFC Documentation (Scope: protocol)

Add `// RFC 7911 Section 2: "<quoted text>"` above the route key constructor's
path identifier handling, and the section reference at each caller.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] No code snippets
- [ ] FRR source read for the interop scenario (owed research above)

### Goal Gates (MUST pass)
- [ ] AC-1 to AC-7 each proven by a named test
- [ ] Wiring Test table complete
- [ ] Scoped runs: `./le test unit pkg` for `rr`, `rr/routekey`, `reactor`, `reactor/fwdkey`, `configjson`, `blackholecfg`, `rpki`, `role`, `rib`, `filter_community`; the two `.ci` tests through `./le test bgp plugin`; `INTEROP_SCENARIO=bgp-rr-departure-equal-length-evpn-frr ./le test integration interop`; `./le rfc check`
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

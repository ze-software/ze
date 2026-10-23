# Spec: typed-identity-keys

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Keys that omit a fact the builder or the RFC uses make two distinct routes, sessions or peers answer as one; keys derived from mutable fields leak entries the delete can no longer find. Both are shipped-daemon defects visible on the wire or in `show` output. |
| Depends | `plan/release/fixes/spec-single-owner-lifecycle.md` (the ownership check) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Family S3 of the journal backlog analysis (2026-10-23), part 3 of 3.

## Task

**Root cause.** An entity's identity is not a type with one constructor. Each
map, cache or lookup builds its own key, from whatever fields are at hand, so:

1. a key omits a fact the builder or the RFC uses (ADD-PATH path identifier,
   BFD session local address and VRF, OSPF neighbour interface address, a diff
   line's path);
2. an index key is re-derived at delete time from a field that changed since
   insert (L2TP peer address, DHCP route metric);
3. an external identifier (VPP interface index, kernel ifindex, a config key
   string) outlives the object it named and is reused for another;
4. one key space is written by two producers with different spellings (raw
   operator text against a parsed address, `peer-<addr>` against an operator's
   own key).

**Goal.** Each entity has one identity type built by one constructor that
takes every identifying fact. An index stores the key computed at insert and
deletes by that stored key. The ownership check from
`spec-single-owner-lifecycle.md` refuses a composite literal of an identity
type outside its constructor, so a new map cannot build its own partial key.

**Why it matters for shipping.** Equal-length ADD-PATH routes collide in the
route reflector's withdrawal walk; `show bfd session address` answers for one
of two sessions chosen by map order; L2TP tunnels leak in the peer index; a
`create bgp peer` overwrites an operator's peer in the running tree; DHCP
teardown misses its own route after a link-down moved it.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/bgp/structural-forwarding.md` (forward body cache)
- [ ] `docs/architecture/config/syntax.md`, `docs/architecture/config/yang-config-design.md` (peer config keys)
- [ ] `docs/architecture/core-design.md`
- [ ] `docs/architecture/iface/netlink-monitor.md` (ifindex to name cache)
- [ ] `docs/architecture/ospf/ospf-ext-9-graceful-restart.md` (helper entry)
- [ ] `docs/features/interfaces.md` (DHCP routes, VPP interfaces)
- [ ] `docs/research/bfd-implementation-guide.md` (session key)
- [ ] `docs/research/l2tpv2-ze-integration.md` (tunnel index)
  -> Constraint: every page that states a key's shape changes with it, in the same work.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc7911.txt` Section 2
- [ ] `rfc/full/rfc3623.txt` Section 3.1
- [ ] `rfc/short/rfc5880.md` (BFD session identification, declared by `bfd.go`)

**Key insights:**
- RFC 7911 Section 2: "a particular path for an address prefix can be
  identified by the combination of the address prefix and the Path
  Identifier."
- RFC 3623 Section 3.1: "On broadcast, NBMA and Point-to-MultiPoint segments,
  the neighbor relationship with X is identified by the IP interface address
  in the body of the grace-LSA (see Appendix A)."

## Current Behavior (MANDATORY)

Verified at the producer on 2026-10-23.

**Source files read:**
- [ ] `internal/component/bgp/reactor/forward_body.go` - `fwdBodyCacheKey`, `buildFwdBody`
- [ ] `internal/component/bgp/configjson/traverse.go` - `PeerKey`
- [ ] `internal/component/bfd/engine/snapshot.go` - `(*Loop).SessionDetail`
- [ ] `internal/component/l2tp/reactor.go` - `discardTunnelLocked`, the `peerAddr` overwrite in the dispatch path
- [ ] `internal/plugins/ospf/gr_helper.go` - `helperKey`
- [ ] `internal/component/bgp/plugins/rr/withdrawal.go` - `walkNLRIsAllocating`, `walkVPNNLRIs`
- [ ] `internal/component/bgp/plugins/rs/server_inventory.go` - `nativeKey`

| Producer | What it does today | Rows it explains |
|----------|--------------------|------------------|
| `fwdBodyCacheKey` (`internal/component/bgp/reactor/forward_body.go`) | Key is destination context id, wire pointer, extended flag, preserve-opaque flag. `buildFwdBody` also takes `peer` (its send context drives the RFC 7911 path-id rewrite) and `maxMsgSize`. Whether the context id determines both is unproven; nothing pins it | key-omits-a-fact-the-builder-uses 2026-10-10 |
| `PeerKey` (`internal/component/bgp/configjson/traverse.go`) | Returns the operator's raw text as the key: the `remote ip` as written, or the unparsed name after `netip.ParseAddr` merely accepts it. Runtime readers key by the parsed address, so two spellings of one address are two keys | gate-excludes-part-of-its-population 2026-08-14 |
| `(*Loop).SessionDetail` (`internal/component/bfd/engine/snapshot.go`) | Ranges the sessions map and returns the first whose `key.Peer` matches, though the key also holds local address, interface, VRF and mode | key-omits-a-fact-the-builder-uses 2026-10-13 |
| `discardTunnelLocked` (`internal/component/l2tp/reactor.go`) | Deletes from `tunnelsByPeer` with a key built from the current `t.peerAddr`, which the dispatch path overwrites with `pkt.from` on every datagram, so after a peer address change the delete misses and the entry leaks | index-key-derived-from-mutable-field 2026-08-10 |
| `helperKey` (`internal/plugins/ospf/gr_helper.go`) | Keyed by local interface and Advertising Router on every segment type; the RFC 3623 Section 3.1 interface-address identification on broadcast, NBMA and P2MP is not used | key-omits-a-fact-the-builder-uses 2026-10-02 |

**Analysis correction.** The analysis cites `fwdBodyCacheKey` as a confirmed
instance. The row itself says the omitted facts are "plausibly" covered; it is
an unproven key, and AC-3 decides it either way.

**Behavior to preserve:** wire output of forwarded UPDATEs; `show` output
shapes; the operator's own peer keys in the running tree.

**Behavior to change:** every row in "Owned journal rows".

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Received UPDATEs (RR walk, forward cache), operator config (peer keys,
  `create bgp peer`), `show` commands (BFD, CLI diff), L2TP datagrams, netlink
  and VPP events, OSPF grace-LSAs.

### Transformation Path
1. The producer of an entity calls the identity constructor with every
   identifying fact.
2. Indexes store the constructed key beside the entry.
3. Delete and update use the stored key, never a key rebuilt from current
   fields.
4. Lookups that take a partial key (an address alone) return every match or
   refuse as ambiguous, never the first map hit.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree <-> reactor | parsed peer identity | No |
| VPP and kernel <-> iface | interface identity bound to the create that produced it | No |

### Integration Points
- The ownership check: identity types carry the constructor annotation.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | |
| No unintended coupling | No | |
| No duplicated functionality | No | one constructor per identity replaces each map's own key |
| Zero-copy preserved where applicable | No | the forward cache key stays a fixed-size value |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | the check reads the annotation at the type |

## The fix

| Entity | Identity | Replaces |
|--------|----------|----------|
| BGP path (RR walk, every non-CIDR family) | family, NLRI bytes, path identifier, ADD-PATH presence (RFC 7911 Section 2) | `WireNLRI.String` keys outside the VPN branch |
| Forward body | every input `buildFwdBody` reads, or a test proving the context id determines `peer` and `maxMsgSize` | four-field key |
| BGP peer config | parsed address or canonical name, one constructor shared by `PeerKey` and every runtime reader | raw operator text |
| API-created peer | refused when its generated key collides with an existing running key | silent overwrite in `recordCreatedPeerLocked` |
| BFD session | full session key; partial lookups answer all matches or refuse | first map hit |
| CLI diff line | path-qualified line rendered from pending changes, shared with the web diff | set comparison of path-less text |
| L2TP tunnel | key stored at insert, deleted by the stored key | key rebuilt from mutable `peerAddr` |
| DHCP route | route identity stored at install, removed at its effective metric | identity re-derived after link-down moved the metric |
| VPP interfaces (GRE, IPIP, VXLAN, WireGuard) | ze name bound to the index the create returned; create is idempotent per name | unconditional create per apply |
| ifindex name cache | cache entry bound to the link's generation; a refill after delete reads the event, not the kernel | refill by index after delete |
| OSPF GR helper | interface address on broadcast, NBMA and P2MP; Advertising Router elsewhere (RFC 3623 Section 3.1) | Advertising Router everywhere |

**The check that stops it coming back.** Each identity type carries the
ownership annotation naming its constructor. The ownership check refuses a
composite literal of that type anywhere else, including inside a `delete` or a
map index, so a key can only be built with every fact.

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| ADD-PATH withdrawal through the route reflector | -> | BGP path identity | `TestRRWithdrawalKeepsEqualLengthAddPathRoutesApart` (new, rr) |
| `show bfd session address <peer>` with two sessions to one address | -> | BFD session lookup | `TestSessionDetailRefusesAmbiguousPeer` (new, bfd/engine) and a `.ci` |
| a commit carrying a composite literal of an identity type outside its constructor | -> | ownership check | `TestOwnershipCheckRefusesForeignKeyLiteral` (new, internal/le/repo) |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A commit builds an identity type outside its constructor | refused naming the type, constructor and fix |
| AC-2 | Two ADD-PATH routes of equal length in a non-VPN non-CIDR family, one withdrawn | the other stays reflected |
| AC-3 | Two destinations sharing a context id but differing in path-id rewrite or max message size | either the cache returns distinct bodies, or a test proves the context id makes them equal |
| AC-4 | The same peer address written two ways in config | one peer config key; every runtime reader finds it |
| AC-5 | `create bgp peer` whose generated key equals an operator key at another address | refused with what, why and the fix; the operator's peer is unchanged |
| AC-6 | Two BFD sessions to one address | `show bfd session address` lists both or refuses as ambiguous |
| AC-7 | SSH `show` piped to `compare` with two same-text siblings, one changed | the change is shown with its path |
| AC-8 | An L2TP peer's source address changes, then the tunnel closes | no entry remains in the peer index |
| AC-9 | DHCP teardown after a link-down moved its default route | the route is removed |
| AC-10 | Repeated applies of a config with GRE, IPIP, VXLAN and WireGuard interfaces | no VPP interface is created twice; deleting one releases its index |
| AC-11 | `RTM_DELLINK` then a late event for the old index | the event carries no name from a reused index |
| AC-12 | OSPF grace-LSA on a broadcast segment | helper mode is entered for the neighbour named by the interface address in the grace-LSA body |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOwnershipCheckRefusesForeignKeyLiteral` | `internal/le/repo/ownership_test.go` | AC-1 | |
| `TestRRWithdrawalKeepsEqualLengthAddPathRoutesApart` | `internal/component/bgp/plugins/rr/withdrawal_test.go` | AC-2 | |
| `TestFwdBodyCacheKeyCoversBuilderInputs` | `internal/component/bgp/reactor/forward_body_test.go` | AC-3 | |
| `TestPeerKeyCanonicalAcrossSpellings` | `internal/component/bgp/configjson/traverse_test.go` | AC-4 | |
| `TestCreatedPeerKeyCollisionRefused` | `internal/component/bgp/reactor/reactor_api_test.go` | AC-5 | |
| `TestSessionDetailRefusesAmbiguousPeer` | `internal/component/bfd/engine/snapshot_test.go` | AC-6 | |
| `TestSSHDiffIsPathQualified` | `internal/component/cli/editor_test.go` | AC-7 | |
| `TestDiscardAfterPeerMoveLeavesNoIndexEntry` | `internal/component/l2tp/reactor_test.go` | AC-8 | |
| `TestNameCacheIgnoresReusedIndex` | `internal/component/iface/cmd/monitor_netlink_linux_test.go` | AC-11 | |
| `TestGraceLSAHelperKeyedByInterfaceAddress` | `internal/plugins/ospf/gr_helper_test.go` | AC-12 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ADD-PATH path identifier | 0 to 2^32-1 | 4294967295 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bfd-show-two-sessions-one-address` | `test/plugin/` | operator sees both sessions (AC-6) | |
| `api-create-peer-key-collision` | `test/plugin/` | collision refused with fix (AC-5) | |
| DHCP and VPP integration | `internal/plugins/iface/` QEMU integration | AC-9, AC-10 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-rr-addpath-withdraw-frr` | `test/interop/scenarios/` | FRR | equal-length ADD-PATH routes stay distinct through the RR | |
| `ospf-gr-helper-broadcast-bird` | `test/interop/scenarios/` | BIRD 3 | helper mode on a broadcast segment keyed per RFC 3623 Section 3.1 | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- annotate the identity types; AC-1 red then green
2. **Phase: BGP identities** -- path, forward body, peer config, created peer (AC-2 to AC-5)
3. **Phase: lookups and diff** -- BFD, CLI diff (AC-6, AC-7)
4. **Phase: stored keys and external identifiers** -- L2TP, DHCP, VPP, ifindex cache (AC-8 to AC-11)
5. **Phase: OSPF GR helper** -- AC-12 with interop

## Files to Modify
- `internal/component/bgp/plugins/rr/withdrawal.go`
- `internal/component/bgp/reactor/forward_body.go`, `reactor_api.go`
- `internal/component/bgp/configjson/traverse.go`
- `internal/component/bfd/bfd.go`, `internal/component/bfd/engine/snapshot.go`
- `internal/component/cli/editor.go`
- `internal/component/l2tp/reactor.go`
- `internal/component/iface/register.go`, `internal/plugins/iface/dhcp/`
- `internal/plugins/iface/vpp/tunnel.go`, `vxlan.go`, `wireguard.go`
- `internal/component/iface/cmd/monitor_netlink_linux.go`
- `internal/plugins/ospf/gr_helper.go`

## Files to Create
- none beyond tests

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| key-omits-a-fact-the-builder-uses | 2026-10-03 | RR `walkNLRIsAllocating` non-CIDR key omits ADD-PATH (non-VPN families left) |
| key-omits-a-fact-the-builder-uses | 2026-10-10 | relay body cache `fwdBodyCacheKey` |
| key-omits-a-fact-the-builder-uses | 2026-10-11 | cli `computeDiff` path-less set comparison |
| key-omits-a-fact-the-builder-uses | 2026-10-13 | `show bfd session address` first map hit |
| key-omits-a-fact-the-builder-uses | 2026-10-02 | ospf GR helper keyed by Advertising Router |
| index-key-derived-from-mutable-field | 2026-08-10 | l2tp `discardTunnelLocked` key from overwritten `peerAddr` |
| index-key-derived-from-mutable-field | 2026-08-11 | iface dhcp route moved by `handleLinkDown` metric |
| identifier-reused-after-its-owner-is-gone | 2026-08-19 | iface vpp GRE, IPIP, VXLAN, WireGuard unconditional create |
| identifier-reused-after-its-owner-is-gone | 2026-08-18 | netlink monitor `ifNameCache` refill after delete |
| two-owners-share-one-name | 2026-10-10 | bgp reactor `recordCreatedPeerLocked` overwrites operator key |
| gate-excludes-part-of-its-population | 2026-08-14 | `PeerKey` keys raw operator text |
| declared-format-contradicts-payload | 2026-10-20 | adj-rib-in `handleReceived` keys a VPN NLRI without its RD, stores one NLRI per section |
| gate-excludes-part-of-its-population | 2026-09-02 | `countPeerUpBarrier` asks the registry by process name, not registration name |
| index-key-derived-from-mutable-field | 2026-10-05 | rs `appendOpaqueRecords`/`recordKey` keyed on full wire hex |

12 rows.

## Rows found already fixed

| Class file | Date | Surface (short) | Evidence |
|------------|------|-----------------|----------|
| index-key-derived-from-mutable-field | 2026-10-05 | rs `appendOpaqueRecords`, `recordKey` | `internal/component/bgp/plugins/rs/server_inventory.go` carries `nativeKey` ("registered VPN identity, excluding label/Compatibility"), set when the record is built and used for the delete. Tests not run by this agent |

## Rows considered and not owned

| Class file | Date | Surface (short) | Why not this root cause |
|------------|------|-----------------|-------------------------|
| key-omits-a-fact-the-builder-uses | 2026-09-27 | ospf NSSA translator set | an RFC 3101 reachability gap, not a key; a protocol one-off |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Producer evidence re-read for every row before implementation starts
- [ ] RFC 7911 and RFC 3623 sentences re-read in `rfc/full/`
- [ ] No code snippets

### Goal Gates (MUST pass)
- [ ] AC-1 to AC-12 each proven by a named test
- [ ] Wiring Test table complete
- [ ] Targeted tests pass: `./le test unit` over the packages in Files to Modify, `./le test functional plugin`, and `./le verify current fast`
- [ ] Interop scenarios green, with the red phase recorded

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Owned rows re-pointed to this spec's closure

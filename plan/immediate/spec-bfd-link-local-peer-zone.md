# Spec: bfd-link-local-peer-zone

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A BFD session to an IPv6 LINK-LOCAL peer cannot be selected for a packet that
carries no discriminator, in either direction, because the address ze holds and
the address ze observes are in different FORMS.

Found during the round-13 review of `spec-bgp-bfd-strict`. The mechanism is
CONFIRMED by reading the producers; the operator impact is PLAUSIBLE rather than
measured, because an Active session still converges through `byDiscr` once the
peer answers.

This is a DIFFERENT failure class from the five repairs that spec made. Those
were about UNSETNESS: a session key field the client left empty, which
`api.SessionRequest.Canonical` could not derive, against a packet that carried a
value. The matching rule those repairs produced is general over that class and
is stated once in `internal/component/bfd/engine/engine.go`. It is narrower than
the class that produced them, and this is the other half: both sides hold the
same address, in two forms that do not compare equal.

### Re-scoped at design, 2026-10-13

The code half of the skeleton LANDED before this design started. Commit
`d7923475e6` ("fix(bfd): a neighbor disabling BFD no longer drops the BGP
session", 2026-09-20) added `reconcileZone`, `firstPacketObserved` and
`sendZone`, and the three unit tests the skeleton named; its journal row in
`plan/journal/declared-format-contradicts-payload.md` (2026-09-11) is marked
FIXED and says "The `.ci` and the FRR interop scenario named in
`plan/immediate/spec-bfd-link-local-peer-zone.md` are still owed". Commit
`31dcff600d` (2026-10-13) then added the `IPV6_PKTINFO` source and ifindex pin
on send, which is a second, independent route a link-local packet takes to its
link.

So what this spec still owes is the proof on a real kernel and against a second
implementation, plus the one gap that research found in the configuration
surface:

| Part | What | Why still open at HEAD |
|------|------|------------------------|
| P-1 | A `.ci` on a real link: a first packet from a link-local peer selects the session, the zone keeps two links apart, and Ze's own packets leave the link the session names | Every proof today is a unit test over hand-built `Inbound` values and over `destination`; no test has put a link-local packet through a kernel |
| P-2 | An FRR bfdd interop scenario where a BFD-plugin-pinned link-local session reaches Up | `ospfv3-bfd-frr` reaches Up over a link-local pair, but through OSPFv3 as the client, and it converges through `byDiscr` whether or not the zone is reconciled |
| P-3 | The `bfd` plugin refuses, at verify, a `single-hop-session` to an IPv6 link-local peer that names no `interface`, and a peer written with a zone | Today both are accepted; the first sends with no zone and no pin, so the kernel picks the link (Current Behavior) |
| P-4 | RFC5881-3-2 tags on the link-local first-packet unit tests, each with a discrimination record | The tests prove the RFC 5881 Section 3 association for the link-local form and carry no tag |

Owner rule carried into every config in this spec (owner, 2026-10-12, commit
`b3cfb912d0`): BFD settings belong to the BFD plugin, and a config whose client
opts into BFD cannot be committed while the `bfd` section is absent or
disabled, until that opt-in is removed. Every config here therefore carries
`bfd { enabled true; bind-v6 true; ... }` and puts timers in a `bfd` profile.

## Required Reading

- `internal/component/bfd/transport/udp.go` -- `readLoop`, which builds `Inbound.From`
  → Constraint: `in.From` keeps the kernel's zone (`fe80::1%eth0`); every consumer that compares it to configuration MUST go through `firstPacketObserved`.
- `internal/component/bfd/transport/udp.go` -- `(*UDP).Send`, `destination`, `sendZone`, `pinsToLink`, `sendControl`
  → Constraint: on Linux a link-local packet reaches its link by three routes: the zone `sendZone` adds, the `IPV6_PKTINFO` ifindex `pinsToLink` adds when the zone is empty, and `SO_BINDTODEVICE` when every default-VRF single-hop session names one interface. Reverting ONE of them leaves the packet on the right link, so a kernel test cannot discriminate `sendZone` alone.
  → Decision: `sendZone` stays: off Linux (`udp_other.go`) no control message is built and the zone is the only route.
- `internal/component/bfd/engine/engine.go` -- `firstPacketKey`, `firstPacketIndex`, `firstPacketObserved`, `reconcileZone`, and the matching rule below it
  → Constraint: the link is held ONCE, in `iface`; a zone on the peer moves into `iface` for single-hop only when `iface` is empty, and is dropped otherwise. A config that writes both a zone and a different `interface` is resolved silently in favour of `interface` on receive while `sendZone` sends by the zone: P-3 closes that by refusing the zone at config.
- `internal/component/bfd/engine/loop.go` -- `handleInbound`
  → Constraint: Your Discriminator zero walks `keyRelaxations` over `firstPacketObserved(in)`; nonzero uses `byDiscr` only. Single-hop sessions are always Active (RFC5881-3-1 is enforced: a passive single-hop profile is refused), so against a live peer the session converges through `byDiscr` even with the receive half broken; only a peer that never echoes Ze's discriminator discriminates the receive half.
- `internal/component/bfd/api/session_identity.go` -- `Canonical`, `deriveLink`, `addressOn`
  → Constraint: a link-local peer with no interface is ambiguous whenever more than one link carries `fe80::/64`, and `Canonical` returns the request unchanged; with exactly one such link it derives that link. Whether a config works therefore depends on how many links the box has, which is why P-3 refuses at verify instead of relying on derivation.
- `internal/component/bfd/config.go` -- `parseSingleHopSession`
  → Constraint: `peer` and `local` are YANG `type string` parsed by `netip.ParseAddr`, which accepts a zone (`fe80::1%eth0`); nothing refuses a link-local peer with no `interface`. P-3's refusal lives here, in the BFD plugin, per the owner rule.
- `internal/component/bfd/cmd/bfd.go`, `internal/component/bfd/engine/snapshot.go` -- `handleShowSession`, `SessionDetail`
  → Constraint: `show bfd session address <peer>` matches on peer alone and returns the first map hit, so it cannot tell two links' sessions apart (journal row, `plan/journal/key-omits-a-fact-the-builder-uses.md`, 2026-10-13). Tests here read `show bfd sessions` and select by `interface`.
- `internal/test/fixture/bfd_fixture_pktinfo.go`, `internal/test/fixture/register_bfd.go`, `test/bfd/bfd-first-packet-pktinfo-v6.ci`
  → Decision: P-1 follows this shape: an observer plugin injects Your-Discriminator-zero Control packets and polls the session's `remote-discriminator`, which only a selected first packet can set.
- `rfc/full/rfc5881.txt` Sections 2, 3, 6; `rfc/full/rfc5880.txt` Sections 6.3 and 6.8.6; `rfc/full/rfc4007.txt` Section 6
  → Constraint: quotes in "RFC Documentation" below. RFC 4007 has no `rfc/short/` summary, and RFC scope is frozen (owner, 2026-10-08): it is cited in comments only and not enrolled.
- `rfc/short/rfc5881.md` -- rows RFC5881-3-1, RFC5881-3-2, RFC5881-6-1, RFC5881-6-2
  → Constraint: no `Support` row changes; P-4 adds tagged units for RFC5881-3-2 only.
- `rfc/short/rfc5880.md` -- rows RFC5880-6.8.6-7 (nonzero Your Discriminator selects) and RFC5880-6.8.6-18 (zero selects on other fields)
  → Constraint: unchanged by this spec; the existing RFC5880-6.8.6-18 units in `engine/rfc5881_test.go` keep their records because `handleInbound` and `reconcileZone` do not change.
- `docs/architecture/bfd.md` -- "One session per neighbor, whatever asks for it", the zone paragraph citing `reconcileZone` and `destination`
  → Constraint: already describes the receive and send reconciliation; P-3 adds the config refusal there.
- `docs/research/bfd-implementation-guide.md` -- IPv6 paragraph: "Single-hop IPv6 sessions use link-local addresses and bind to the interface via the scope ID."
  → Decision: matches P-3: a link-local session is bound to an interface, and the interface is stated by the `interface` leaf.
- `docs/architecture/testing/interop.md` -- "Prove a scenario discriminates", "Container Addresses" (`@ZE_LINK_LOCAL@`, `rfc2545LinkLocal` in `prepare.go`), "Live attach and BFD opt-in"
  → Constraint: a scenario that passes either way proves acceptance; the discrimination moves to a test that can fail. P-2 is acceptance (see Goal Validation Plan); P-1 carries the discrimination.

## Current Behavior (MANDATORY)

Source files read for the statement below:

- [ ] `internal/component/bfd/transport/udp.go`
- [ ] `internal/component/bfd/engine/engine.go`
- [ ] `internal/component/bfd/engine/loop.go`
- [ ] `internal/component/bfd/api/events.go`

A received packet's source address comes from `raddr.Addr().Unmap()`. For an
IPv6 link-local source the kernel reports a ZONE, so `in.From` is
`fe80::1%eth0`; `Unmap` does not remove it. The session's `Peer` comes from
configuration, parsed by `netip.ParseAddr` from a leaf an operator writes
without a zone, so the key holds `fe80::1`.

`firstPacketKey` compares `Peer` exactly, and `Peer` is deliberately never
relaxed: it is the one field a session cannot leave unset, which the parity test
`TestFirstPacketKeyMirrorsEveryKeyField` records. So every relaxation misses,
and no packet whose Your Discriminator is zero can select the session.

`(*UDP).Send` drops the zone in the other direction, so a packet ze sends to a
link-local peer has no zone to route by. Between the two, link-local BFD reads
as unbuilt rather than as broken in one place.

### At HEAD, 2026-10-13 (supersedes the three paragraphs above, which describe 2026-09-11)

Read at the producer for this design. Digests:

| File | Lines | What it does now |
|------|-------|------------------|
| `internal/component/bfd/engine/engine.go` | 944 | `firstPacketIndex(k)` and `firstPacketObserved(in)` both end in `reconcileZone`: for a single-hop key with an empty `iface`, the peer's zone moves into `iface`; the zone is then stripped from `peer` and from `local`. The comment above it quotes RFC 4007 Section 6 and RFC 5881 Section 2 |
| `internal/component/bfd/engine/loop.go` | 403 | `handleInbound`: Your Discriminator zero builds `firstPacketObserved(in)` and walks `keyRelaxations` most-specific first over `byKey`; nonzero reads `byDiscr`. Then the TTL gate (255 for single-hop), auth, `Receive`, Final on Poll |
| `internal/component/bfd/transport/udp.go` | 755 | `Send`: `destination(out)` builds the `UDPAddr` with `Zone: sendZone(out)` (the address's own zone, else `out.Interface`, only for `IsLinkLocalUnicast`); for single-hop with `out.Interface` set it checks `egressLink.reaches` (link-local exempt) and pins the ifindex unless the socket is device-bound or the destination is zoned (`pinsToLink`); `sendControl` builds `IPV6_PKTINFO` with the source from `out.Local` (commit `31dcff600d`) |
| `internal/component/bfd/api/session_identity.go` | ~200 | `Canonical` derives interface and local for single-hop only when exactly one link holds a prefix containing the peer; a link-local peer on a multi-link box is left as written |
| `internal/component/bfd/config.go` | ~700 | `parseSingleHopSession` accepts any `netip.ParseAddr` form for `peer` and `local`, zone included, and does not require `interface` for any peer |
| `internal/component/bfd/engine/snapshot.go` | - | `SessionDetail(peer)` returns the first session in map order whose `key.Peer` string matches |
| `internal/component/bfd/engine/link_local_first_packet_test.go` | 105 | `TestFirstPacketSelectsALinkLocalPeer`, `TestFirstPacketTakesTheLinkFromTheZone`, `TestTwoLinkLocalSessionsStayApart`: hand-built `Inbound`, no kernel, no RFC tag |
| `internal/component/bfd/transport/udp_zone_test.go` | 72 | `TestSendKeepsTheZoneForALinkLocalPeer`: asserts `destination`'s `Zone`, no write, no RFC tag |

What this leaves, by path:

| Path | Today | Proven by |
|------|-------|-----------|
| Receive, first packet from `fe80::b%veth0` to a session `fe80::b` on `veth0` | Selected (zone reduced into `iface`) | Unit only |
| Receive, same address on two links | Each link's packet selects its own session | Unit only |
| Send, session names `interface` | Zone set by `sendZone`; also ifindex pin or device binding | Unit on `destination` only |
| Send, link-local session that names NO interface on a box with several links | `Canonical` cannot derive; `sendZone` returns empty, `pinsToLink` returns false; the kernel resolves `fe80::/64` by its own route choice, which is any link carrying that prefix | Nothing; read, not run (A-2) |
| Config, `single-hop-session fe80::b%veth0 { interface veth1 }` | Accepted; receive keys on `veth1`, send uses the zone `veth0` | Nothing |
| Interop | `ospfv3-bfd-frr` reaches Up over link-local through OSPFv3, converging through `byDiscr` | Acceptance only |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A BFD Control packet arriving on UDP 3784 from an IPv6 link-local source, and
  a configured session naming that peer.
- A `bfd { single-hop-session <fe80::...> { interface <name>; ... } }` entry
  in the configuration, verified by the BFD plugin at commit, reload and start.

### Transformation Path
1. `readLoop` builds `Inbound.From` from `raddr.Addr().Unmap()`, zone included.
2. `handleInbound` builds `firstPacketObserved(in)`, which reduces the zone into
   `iface`, and walks the relaxations.
3. `firstPacketIndex` built the session's key from `api.Key`, reduced the same way.
4. Send: `sendLocked` → `(*UDP).Send` → `destination` (zone from `interface`)
   → `sendControl` (`IPV6_PKTINFO` source) → `WriteMsgUDP`.
5. Config (new, P-3): `parseSingleHopSession` refuses a zoned `peer` or
   `local`, and a link-local `peer` without `interface`, before any session
   request is built.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Kernel to transport | recvmsg source address, zone-qualified for link-local | Yes: Go's `ReadMsgUDPAddrPort` returns `WithZone`, per the comment in `reconcileZone`; kernel proof is P-1 |
| Transport to engine | `transport.Inbound` | Yes, `firstPacketObserved` |
| Config to engine | `api.SessionRequest.Peer`, parsed from a YANG leaf | Yes, `parseSingleHopSession` |
| Engine to kernel | `UDPAddr.Zone` plus `IPV6_PKTINFO` | Unit on `destination`; kernel proof is P-1 |

### Integration Points
- `firstPacketKey` and the matching rule above it own the reconciliation
  (unchanged by this spec).
- `parseSingleHopSession` (`internal/component/bfd/config.go`) owns the new
  refusal, inside the BFD plugin per the owner rule.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The refusal is in the plugin's own config parser; tests drive the daemon through config and the wire |
| No unintended coupling (components stay isolated) | Yes | No client (BGP, OSPF, static) code changes; OSPF already names its interface |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `reconcileZone`, `sendZone`, the pktinfo fixture shape, the `@ZE_LINK_LOCAL@` render |
| Zero-copy preserved where applicable (refs, not copies) | Yes | No hot-path change |
| Registration over hardcoding, outbound | Yes | No new command or registry entry; one new interop scenario registers through the existing scenario table |
| Registration over hardcoding, inbound | Yes | No switch learns a new case; the config check is a property of the address (`IsLinkLocalUnicast`, `Zone`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a Control packet from `fe80::1%eth0` with Your Discriminator 0 | → | `Loop.handleInbound` | `TestFirstPacketSelectsALinkLocalPeer` |
| config `bfd { single-hop-session fe80::b { interface zbl0; local fe80::a } }`, then a raw IPv6 frame from `fe80::b` with Your Discriminator 0 arriving on `zbl0` | → | `readLoop` → `firstPacketObserved` → `reconcileZone` → `handleInbound` | `test/bfd/bfd-first-packet-link-local.ci` |
| the same session's periodic transmit | → | `sendLocked` → `(*UDP).Send` → `destination`/`sendZone` → kernel | `test/bfd/bfd-first-packet-link-local.ci` (frames captured on the far end of `zbl0`, none on `zbl1`) |
| config `bfd { single-hop-session fe80::b { } }` with no `interface` | → | `parseSingleHopSession` refusal | `test/parse/bfd-link-local-needs-interface.ci` |
| config `single-hop-session fe80::b%zbl0 { interface zbl0 }` | → | `parseSingleHopSession` refusal | `test/parse/bfd-link-local-zone-refused.ci` |
| FRR bfdd `peer <fe80> interface eth0`, Ze `single-hop-session <fe80> { interface eth0 }` | → | the whole receive and send path in a container lab | interop `bfd-link-local-frr` |

## Acceptance Criteria (design, 2026-10-13)

AC-1 to AC-4 below are the skeleton's, kept as written; their code is at HEAD
(`d7923475e6`), and this spec owes their kernel proof. AC-5 onward are new.

| AC ID | Input / Condition | Expected Behavior | Proof |
|-------|-------------------|-------------------|-------|
| AC-1 | (skeleton) a session configured to an IPv6 link-local peer, and a first packet from it | selected | Unit `TestFirstPacketSelectsALinkLocalPeer` (exists); kernel `bfd-first-packet-link-local.ci` step "zbl0": the session on `zbl0` reports `remote-discriminator` equal to the injected My Discriminator within 10 s. Red when `reconcileZone` returns its argument unchanged |
| AC-2 | (skeleton) one address on two links | each link's packet selects its own session | Unit `TestTwoLinkLocalSessionsStayApart`, `TestFirstPacketTakesTheLinkFromTheZone` (exist); kernel: the `.ci` injects My Discriminator X on `zbl0` and Y on `zbl1`, and the session on `zbl0` reports X while the one on `zbl1` reports Y, read from `show bfd sessions` by `interface` (never `show bfd session address`, journal row 2026-10-13). Subject to A-3 / Q-3 |
| AC-3 | (skeleton) Ze sends to a link-local peer | the packet leaves the interface the session names | Unit `TestSendKeepsTheZoneForALinkLocalPeer` (exists, `destination`); kernel: frames captured with AF_PACKET on `zbl0p` carry Your Discriminator X and none carry Y; on `zbl1p` the reverse. Each frame's IPv6 source is `fe80::a`, destination `fe80::b`, hop limit 255, UDP destination 3784 |
| AC-4 | (skeleton) the reconciliation | stated once, beside the matching rule | Met at HEAD: `reconcileZone` in `engine.go`, the only caller pair being `firstPacketIndex` and `firstPacketObserved`. Review check, no new test |
| AC-5 | `bfd { single-hop-session fe80::b { profile p } }`, no `interface` | the commit (and a reload, and a start) is refused; the error names the peer `fe80::b` and says to add `interface` naming the link it is on | `TestSingleHopLinkLocalPeerNeedsInterface` (unit, `parseSingleHopSession`); `test/parse/bfd-link-local-needs-interface.ci` (`ze config validate` exits non-zero with that text) |
| AC-6 | the same with `interface zbl0`, and a global IPv6 peer with no `interface`, and an IPv4 peer with no `interface` | each is accepted | `TestSingleHopLinkLocalPeerNeedsInterface` accept rows (guards that the refusal is scoped to `fe80::/10` and not to IPv6) |
| AC-7 | `peer` or `local` written with a zone (`fe80::b%zbl0`) | refused, naming the leaf and saying to write the link in `interface` (recommended answer to Q-1) | `TestSingleHopPeerRefusesAZone`; `test/parse/bfd-link-local-zone-refused.ci` |
| AC-8 | a multi-hop session (`multi-hop-session`) to a link-local peer | unchanged by this spec: today's behaviour (`reconcileZone` drops the zone for multi-hop) | No new test; listed so a reviewer can see the refusal is single-hop only |
| AC-9 | FRR bfdd configured with `peer <FRR-side fe80> interface eth0`, Ze with the mirror `single-hop-session` | both sides report the session Up; FRR's `remote-id` for the peer equals Ze's `local-discriminator`, and Ze's `remote-discriminator` equals FRR's `id` | interop `bfd-link-local-frr` checker; each side's discriminator is learned from the OTHER side's packets, so neither clause is satisfied by one direction alone |
| AC-10 | the RFC 5881 Section 3 association for a link-local first packet | the link-local first-packet unit tests carry `RFC requirement: RFC5881-3-2` tags, each with a discrimination record written by `./le rfc discriminate-record` whose break is `reconcileZone` returning its argument | `./le rfc check` green for the added tags |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|---------------------|-----------------------|
| 1 | configures BFD to a link-local neighbor on one link and commits | config → `parseSingleHopSession` → `EnsureSession` → engine; neighbor's first packet → `readLoop` → `handleInbound` | `bfd-first-packet-link-local.ci`, `bfd-link-local-frr` |
| 2 | configures the same link-local neighbor address on two links | two sessions, each keyed on its `interface` | `bfd-first-packet-link-local.ci` (AC-2) |
| 3 | forgets `interface` on a link-local peer | refused at commit with the fix named | `bfd-link-local-needs-interface.ci` |
| 4 | writes `fe80::b%eth0` out of habit | refused, told to use `interface` | `bfd-link-local-zone-refused.ci` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|------------|-------------------------------|----------|--------------|--------|
| A-1 | Go returns a received link-local source with the kernel's zone | comment in `reconcileZone`; `net.(*UDPConn).ReadMsgUDPAddrPort` | The receive half was never broken and AC-1's kernel test is green with the fix reverted | The `.ci` red run with `reconcileZone` returning its argument | unvalidated |
| A-2 | A link-local session that names no interface on a multi-link box sends on whichever link the kernel's `fe80::/64` route lookup picks, with no error | read of `sendZone`/`pinsToLink`; kernel UDPv6 send path not read here | AC-5's refusal is still right (RFC 5881 Section 3 binds the session to an interface) but its stated motive weakens | `TestSingleHopLinkLocalPeerNeedsInterface` does not depend on it; optional `.ci` probe before the refusal lands | unvalidated |
| A-3 | Two `single-hop-session` entries with the same peer and different `interface` reach the plugin as two sessions | YANG key is `peer vrf interface`, but the comment above `parseSingleHopSession` says the config file parser "only carries the first positional value as the list key", and `parseSections` iterates a map keyed by that string | AC-2 cannot be configured through the BFD plugin at all; two entries merge into one | Phase 1 first test: parse a config with both entries and count sessions | unvalidated; owner question Q-3 if wrong |
| A-4 | A frame injected with AF_PACKET on `zbl0p` is received by Ze on `zbl0` as ordinary ingress with hop limit 255 and pktinfo ifindex of `zbl0` | veth semantics; the far-end address `fe80::b` is NOT configured locally, a permanent neighbor entry on `zbl0` replaces ND, so neither direction is short-circuited through `lo` | The `.ci` reports the wrong interface or nothing | The `.ci`'s green run, plus its red run | unvalidated |
| A-5 | FRR bfdd accepts `peer <fe80> interface eth0` with a link-local it does not hold until the scenario's startup assigns it | `bgp-rfc2545-linklocal-nexthop-frr` assigns Ze's link-local with a startup command (`rfc2545LinkLocal`, `prepare.go`) | The FRR side needs its own startup assignment too | The scenario's first green run | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | The send half cannot be discriminated on Linux by reverting `sendZone` alone: the ifindex pin and the device binding each still put the packet on the right link | `.ci` stays green with `sendZone` returning empty | Recorded, not hidden: AC-3's kernel proof is discriminated by reverting the link route as a whole (`sendZone` empty AND `pinsToLink` false, with two interfaces so the socket is not device-bound); `sendZone` alone is proven by the unit test |
| R-2 | The interop scenario passes with the receive fix reverted, because both sides are Active and converge through `byDiscr` | Red run stays green | Expected and stated: `bfd-link-local-frr` is acceptance evidence; the discrimination is the `.ci` (interop.md, "Prove a scenario discriminates") |
| R-3 | AC-5/AC-7 refuse a config an operator already runs | A config load error after upgrade | Pre-release (`ai/rules/pre-release.md`); and a no-interface link-local session today sends on a link the kernel picks, so it was not working on a multi-link box |
| R-4 | `internal/le/interoplab/bgp/{names.go,check_special.go,prepare.go,bgp_test.go}` carry another session's uncommitted hunks | `git status` at implementation start | Register the scenario in the files only after that work lands, or carry the hunks per `ai/rules/git-safety.md`; never reformat those files |
| R-5 | The refusal placed in `pluginConfig.validate` would stale discrimination records keyed on `bfd/config.go::validate` (`TestRFC5881SingleHopActiveProfileAccepted`, `TestRFC5881SingleHopPassiveProfileRejected`) | `./le rfc check` names them | Put the refusal in `parseSingleHopSession`, which no record names (checked in `rfc/discrimination/rfc588{0,1,2,3}.json`) |
| R-6 | Two veth pairs in the shared test namespace collide with a parallel run | `RTNETLINK answers: File exists` | `option=exclusive:group=bfd-ports`, `ip link add` preceded by a delete of the same names, and removal at the end like `bfd-first-packet-pktinfo-v6.ci` |
| R-7 | `features/bfd-liveness-detection.md` is dirty in another session | `git status` | Edit only the row this spec owns, carry or wait per git-safety |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A link-local BFD config is refused that should load (AC-5/AC-7), or a link-local session stays Down |
| How is it reverted? | Single commit revert per phase; tests and scenario are additive |
| Who else touches this path? | The interop package session (dirty `internal/le/interoplab/bgp/`), `spec-yang-feature-opt-ins-owned-by-feature` (moves OSPF's inline BFD timers into the BFD plugin), `spec-bgp-link-local-only-next-hop` (BGP next hops, no BFD overlap: it names no BFD file) |

### Skeleton AC table (2026-09-11, kept as written; the design table above carries their proof)

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a session configured to an IPv6 link-local peer, and a first packet from it | the session is selected, so the zone is reconciled between the key and the observation rather than being compared raw |
| AC-2 | a session to a link-local peer on eth0 and another to the SAME address on eth1 | a packet from either link selects its own session, which is what the zone is FOR and what makes dropping it unsafe |
| AC-3 | ze sends to a link-local peer | the packet leaves the interface the session names, which `Send` cannot do today |
| AC-4 | the reconciliation | it is stated once, beside the matching rule, rather than as a case inside each comparison |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFirstPacketSelectsALinkLocalPeer` | `internal/component/bfd/engine/link_local_first_packet_test.go` (exists; the skeleton named `rfc5881_test.go`) | AC-1; gains tag `RFC5881-3-2 positive` (AC-10) | exists, tag owed |
| `TestTwoLinkLocalSessionsStayApart` | same file (exists) | AC-2; gains tag `RFC5881-3-2 negative` (the eth0 packet is NOT associated with the eth1 session) | exists, tag owed |
| `TestFirstPacketTakesTheLinkFromTheZone` | same file (exists) | AC-2, transport reports no ingress interface; gains tag `RFC5881-3-2 positive` | exists, tag owed |
| `TestSendKeepsTheZoneForALinkLocalPeer` | `internal/component/bfd/transport/udp_zone_test.go` (exists; the skeleton named `udp_test.go`) | AC-3 at `destination`; no tag (RFC 4007 has no summary, and RFC5881-6-1 is already carried by the pin tests) | exists |
| `TestSingleHopLinkLocalPeerNeedsInterface` | `internal/component/bfd/link_local_config_test.go` (new) | AC-5 refuse row; AC-6 accept rows (link-local with `interface`, global IPv6 without, IPv4 without); the error text names the peer and `interface` | new |
| `TestSingleHopPeerRefusesAZone` | same new file | AC-7, `peer` and `local` each with a zone | new |
| `TestTwoLinkLocalSingleHopSessionsParseApart` | same new file | A-3: two entries, one peer, two interfaces, parse to two sessions | new; if red, stop and raise Q-3 |

### RFC-tagged tests this spec changes (exact list)

| Test | File | Change | Discrimination owed |
|------|------|--------|---------------------|
| `TestFirstPacketSelectsALinkLocalPeer` | `internal/component/bfd/engine/link_local_first_packet_test.go` | tag ADDED: `RFC5881-3-2 positive` | new record, break `reconcileZone` → identity |
| `TestFirstPacketTakesTheLinkFromTheZone` | same | tag ADDED: `RFC5881-3-2 positive` | new record, same break |
| `TestTwoLinkLocalSessionsStayApart` | same | tag ADDED: `RFC5881-3-2 negative` | new record, break: `reconcileZone` strips the zone without moving it into `iface` AND `firstPacketObserved` drops `in.Interface` |
| none other | - | No existing tagged test is edited. The existing RFC5881-3-2 units (`engine/rfc5881_test.go`, `engine/rfc5881_first_packet_binding_test.go`), the RFC5882-4.4-1 units in `api/rfc5882_session_identity_test.go` (which use `fe80::` link tables) and the RFC5881-6-2 units in `transport/rfc5881_subnet_exemption_test.go` (which exempt `fe80::2`) are unaffected: `Canonical`, `reconcileZone`, `Send` and `egressLink.reaches` do not change, and the refusal lives in `parseSingleHopSession`, which no record names | none |

#### Owner approval commands

The owner approved all RFC-tagged test changes on 2026-10-13 ("approve all RFC-tagged test changes"), but this spec needs no command. The three units above exist at HEAD `051876c3f4` with no `RFC requirement:` tag (`link_local_first_packet_test.go` carries none), and the gate asks only about a unit that was tagged in the committed text, so adding a tag to them needs no approval. No existing tagged unit is edited.

0 commands.

The claim of each added tag states what the body checks and no more: "a Your
Discriminator zero packet from a link-local source carrying the kernel's zone
is associated with the session bound to that peer and interface".

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A, no numeric input | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bfd-first-packet-link-local` | `test/bfd/bfd-first-packet-link-local.ci` | an operator configures BFD to a link-local neighbor on two links; each session takes its own neighbor's first packet and sends only on its own link (AC-1, AC-2, AC-3) | new |
| `bfd-link-local-needs-interface` | `test/parse/bfd-link-local-needs-interface.ci` | a link-local peer with no `interface` is refused with the fix named (AC-5) | new |
| `bfd-link-local-zone-refused` | `test/parse/bfd-link-local-zone-refused.ci` | a zoned peer is refused and told to use `interface` (AC-7) | new |

Shape of `bfd-first-packet-link-local.ci` (the implementer writes it; this is
the contract, modelled on `bfd-first-packet-pktinfo-v6.ci`):

| Element | Value |
|---------|-------|
| Options | `option=needs-linux:caps=net-admin,net-raw`, `option=exclusive:group=bfd-ports`, `option=env:var=ze.bfd.test-parallel:value=true` |
| Topology steps | two veth pairs `zbl0`/`zbl0p` and `zbl1`/`zbl1p` with fixed MACs; `fe80::a/64 nodad` on `zbl0` and `zbl1` with `addrgenmode none`; NO address on `zbl0p`/`zbl1p`; a permanent neighbor `fe80::b` → the far end's MAC on each of `zbl0`, `zbl1`; every step removed again at the end |
| Config | `bfd { enabled true; bind-v6 true; profile fast {...}; single-hop-session fe80::b { interface zbl0; local fe80::a; profile fast } }` and the same on `zbl1` (subject to A-3) |
| Fixture | `bfd/bfd-first-packet-link-local` registered in `internal/test/fixture/register_bfd.go`; it writes a raw Ethernet/IPv6/UDP/BFD Down frame (source `fe80::b`, hop limit 255, source port in 49152-65535, My Discriminator X on `zbl0p`, Y on `zbl1p`, Your Discriminator 0) every 200 ms, reads `show bfd sessions`, and captures Ze's frames on `zbl0p`/`zbl1p` with AF_PACKET |
| Pass line on stderr | one `OK:` line per link naming the interface and the discriminator, plus one `OK:` per link for the capture |
| Red runs to record | (1) `reconcileZone` returns its argument: both sessions keep `remote-discriminator` 0; (2) `sendZone` returns empty AND `pinsToLink` returns false: Ze's frames for one session appear on the other link or nowhere (R-1) |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bfd-link-local-frr` | `test/interop/scenarios/bfd-link-local-frr/` (`ze.conf`, `frr.conf`) | FRR bfdd | AC-9: a BFD-plugin-pinned link-local single-hop session reaches Up against FRR, each side's discriminator learned from the other's packets. Acceptance evidence, not discrimination (R-2) | new |

Scenario contract: Ze's and FRR's link-local addresses are fixed by startup
commands, as `bgp-rfc2545-linklocal-nexthop-frr` does for Ze (`@ZE_LINK_LOCAL@`,
`rfc2545LinkLocal` in `internal/le/interoplab/bgp/prepare.go`); a matching FRR
token is added there if none exists. Ze's `ze.conf` holds only the `bfd` root
(no client opt-in, so the owner's opt-in rule is met trivially); FRR's `frr.conf`
holds `bfd` / `peer <ze fe80> interface eth0` / a profile at 300 ms. The checker
is registered in `internal/le/interoplab/bgp/checkers.go` beside
`ospfv3-bfd-frr` and reads FRR `show bfd peer <ze fe80> interface eth0 json`
and Ze `show bfd sessions`.

## Goal Validation Plan

| Goal (Task) | Evidence that will be pasted at closure |
|-------------|------------------------------------------|
| A link-local peer's first packet selects its session on a real kernel | `bfd-first-packet-link-local.ci` green in the QEMU guest, AND its recorded red with `reconcileZone` reverted |
| Two links, one address, two sessions | the same `.ci`'s per-link `OK:` lines (X on `zbl0`, Y on `zbl1`) |
| Ze's packets leave the named link | the same `.ci`'s capture lines, and its recorded red with the link route reverted (R-1) |
| Works with a second implementation | `bfd-link-local-frr` green, with the `Docker.Build` image ID line quoted (interop.md) |
| Misconfiguration is refused, not silently mis-sent | the two `test/parse` `.ci` files green, and the unit tests red before the refusal lands |

## Files to Modify
- `internal/component/bfd/config.go` - `parseSingleHopSession`: refuse a link-local `peer` without `interface`, and a zone on `peer` or `local` (AC-5, AC-7), with the RFC comments below
- `internal/component/bfd/yang/ze-bfd-conf.yang` - `single-hop-session` `peer` and `interface` descriptions state the requirement and the refusal
- `internal/component/bfd/engine/link_local_first_packet_test.go` - add the three `RFC requirement:` tags (AC-10)
- `internal/test/fixture/register_bfd.go` - register `bfd/bfd-first-packet-link-local`
- `internal/le/interoplab/bgp/checkers.go`, `prepare.go`, `names.go` (whichever the scenario table needs; R-4) - register `bfd-link-local-frr` and its FRR link-local render
- `docs/architecture/bfd.md` - the zone paragraph gains the config refusal; anchors to `parseSingleHopSession`
- `docs/guide/bfd.md` - a link-local `single-hop-session` example with `interface`, and the two refusals
- `docs/architecture/testing/interop.md` - Scenario Inventory row and a short "BFD over link-local" note (acceptance-only, R-2)
- `features/bfd-liveness-detection.md` - evidence row for link-local single-hop (R-7)
- `rfc/discrimination/rfc5881.json` - written by `./le rfc discriminate-record`, never by hand

## Files to Create
- `internal/component/bfd/link_local_config_test.go`
- `internal/test/fixture/bfd_fixture_link_local.go`
- `test/bfd/bfd-first-packet-link-local.ci`
- `test/parse/bfd-link-local-needs-interface.ci`, `test/parse/bfd-link-local-zone-refused.ci`
- `test/interop/scenarios/bfd-link-local-frr/ze.conf`, `frr.conf`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new leaf; descriptions only (`ze-bfd-conf.yang`) |
| YANG validation constraints | No | `peer` is `type string` parsed by the plugin; the check needs `IsLinkLocalUnicast` and a sibling leaf, which no native constraint expresses |
| YANG custom validators | No | The plugin's own parse refuses, at commit, reload and start, like the existing `unknown profile` refusal in the same function |
| CLI commands/flags | No | none |
| CLI grammar (keyword before value) | N-A | no grammar change |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | N-A | no RPC; functional tests above cover config and wire |
| Pipe completeness | N-A | no output change |
| Env var registration | No | none |
| Doctor check for runtime dependencies | No | no new path, socket, port or module; `bind-v6` already exists |
| Prometheus counters/metrics | No | none |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|----------------|
| 1 | New user-facing feature, or scope/evidence changed? | Yes | `features/bfd-liveness-detection.md` |
| 2 | Config syntax changed? | Yes (two refusals) | `docs/guide/bfd.md` |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/bfd.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior newly proven? | Yes, tags only | `rfc/discrimination/rfc5881.json` via the tool; no `Support` row changes in `rfc/short/rfc5881.md` |
| 10 | Test infrastructure changed? | No | a fixture follows the existing observer shape |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/bfd.md` |
| 13 | Route metadata keys? | No | - |
| 14 | Prometheus counters? | No | - |
| 15 | Registered inventory changed? | Yes | `docs/architecture/testing/interop.md` Scenario Inventory |
| 16 | Source anchors of changed files | Derived | `./le spec citation anchors spec plan/immediate/spec-bfd-link-local-peer-zone.md` at implementation |
| 17 | Existing examples for this area? | Yes | `docs/guide/bfd.md` single-hop examples checked against `parseSingleHopSession` |

### Discovery (repo-maintenance mechanical checklist)
| Question | Answer |
|----------|--------|
| Where does an agent look first? | `ai/INDEX.md` BFD row → `docs/architecture/bfd.md`, whose zone paragraph names `reconcileZone`, `destination`, and (new) `parseSingleHopSession` |
| What prevents regression? | the `.ci` and its recorded red; the RFC5881-3-2 tags under `./le rfc check` |
| What inventory prevents drift? | the interop Scenario Inventory; `rfc/discrimination/rfc5881.json` |
| What verification proves it? | the Goal Validation Plan above |

## Implementation Steps

The skeleton's steps 1-3 are DONE at HEAD (`d7923475e6`); they are not redone.

1. **Phase: A-3 first** -- `TestTwoLinkLocalSingleHopSessionsParseApart`. If red, stop and report Q-3; AC-2's `.ci` leg depends on it.
2. **Phase: Config refusal** -- `TestSingleHopLinkLocalPeerNeedsInterface`, `TestSingleHopPeerRefusesAZone` red, then the refusal in `parseSingleHopSession`, then the two `test/parse` `.ci`; YANG descriptions, `docs/guide/bfd.md`, `docs/architecture/bfd.md` in the same phase.
3. **Phase: Kernel proof** -- the fixture and `bfd-first-packet-link-local.ci`; record both red runs in the spec (QEMU guest, `./le test` scoped to the one `.ci`).
4. **Phase: Tags** -- AC-10 tags, then `./le rfc discriminate stem rfc5881 report <path>` and `./le rfc discriminate-record` for each.
5. **Phase: Interop** -- `bfd-link-local-frr` (R-4 first), green run with the image ID quoted; Scenario Inventory row; `features/bfd-liveness-detection.md`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-10 each at file:line or test name |
| Correctness | the refusal is single-hop and `fe80::/10` only (AC-6); multi-hop untouched (AC-8) |
| Naming | error text names the leaf `interface`, per `ai/rules/user-facing-errors.md` |
| Data flow | refusal in the BFD plugin's parser, no client code touched (owner rule) |
| Rule: rfc-compliance | new refusal carries the RFC 5881 Section 3 quote; tags carry records |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| the `.ci` and its two reds | spec Implementation Summary pastes |
| the scenario | `ls test/interop/scenarios/bfd-link-local-frr` and the green run |
| the records | `./le rfc check` |

### Security Review Checklist
| Check | What to look for |
|-------|------------------|
| Input validation | `peer`/`local` zone strings are refused before any use; the fixture's raw socket is test-only |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A-3 red | stop, Q-3 |
| `.ci` green with `reconcileZone` reverted | A-1 wrong: back to RESEARCH, the receive half was never broken on a kernel |
| 3 fix attempts failed | STOP, report |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|-------------------------|-----------|
| Keep the landed code; this spec proves it and closes the config gap | Re-open the reconciliation design | `d7923475e6` matches the skeleton's AC-1..AC-4 at the producer; redoing it is layering |
| Refuse a link-local single-hop peer with no `interface` (AC-5) | (a) leave it to `Canonical` derivation; (b) warn and run | (a) works only on a box with one `fe80::/64` link, so the same config loads and misbehaves depending on hardware; (b) is a silently wrong value. RFC 5881 Section 3 binds the session to an interface |
| Refuse a zone on `peer`/`local` (AC-7, Q-1) | Fold the zone into `interface` and refuse a mismatch | One fact, one place: `reconcileZone` already says the link is held once, in `iface`; folding adds a second spelling and a mismatch rule |
| Kernel proof with raw frames on veth, far-end address not local | UDP sockets on both ends in one namespace; a second namespace | Same-namespace local addresses risk delivery through `lo` (A-4); `ip netns add` needs more than `net-admin`; AF_PACKET needs only `net-raw`, already used by 15 `.ci` |
| Interop is acceptance; discrimination lives in the `.ci` | Make FRR or Ze passive to force the first-packet path | RFC 5881 Section 3 requires both sides Active and Ze enforces it (`TestRFC5881ClientSingleHopPassiveProfileRefused`), so a passive Ze is not configurable and a passive FRR still converges through `byDiscr` |

## Owner Questions

| ID | Question | Recommended answer | Status |
|----|----------|--------------------|--------|
| Q-1 | A `peer` (or `local`) written with a zone, `fe80::b%eth0`: refuse it and point to `interface`, or accept it and fold the zone into `interface` (refusing only a mismatch)? | Refuse (Key Design Decisions) | open |
| Q-2 | AC-5 refuses a link-local single-hop peer with no `interface`, even on a box where only one link carries `fe80::/64` and `Canonical` would derive it. Accept that strictness? | Yes | open |
| Q-3 | If A-3 is wrong (two `single-hop-session` entries for one address on two links merge in the config parser), which way: key the list on peer plus interface in the parser, or prove AC-2 on a kernel only through a runtime client (OSPFv3 on two links) and leave the plugin list one-session-per-address? | Decide after Phase 1; recommendation then | open, conditional |
| Q-4 | `show bfd session address <peer>` answers one of several sessions by map order (journal, `key-omits-a-fact-the-builder-uses.md`, 2026-10-13). Fold it into this spec, since two-link link-local is its most visible case, or leave it to the journal? | Leave it to the journal; the tests here read `show bfd sessions` | open |

## Known Limitations

The impact is PLAUSIBLE, not measured: an Active session that transmits still
brings the peer to Up through `byDiscr`, so the visible failure is confined to
the passive and zero-discriminator paths until someone runs it. Establishing
that is the first implementation step, and it belongs in the QEMU guest, where
`test/bfd/bfd-first-packet-pktinfo.ci` already proves the surrounding path.

Update 2026-10-13: the unit tests in `d7923475e6` measured the selection miss
(the journal row says "MEASURED"); single-hop passive is refused by Ze (RFC5881-3-1),
so against a live Active peer the receive defect only cost the RFC 5881 Section 3
association of the peer's initial packets, never convergence. The kernel `.ci`
is still unrun. BGP over a link-local peer is out of scope: `reactor_peers.go`
refuses a link-local `local-address`, and BGP's BFD request takes its link only
from the optional `bfd interface` leaf; OSPF's inline BFD timers are owned by
`plan/spec-yang-feature-opt-ins-owned-by-feature.md`.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
RFC 5881 Section 2 scopes single-hop BFD to one link, and RFC 4007 Section 6
defines the zone index that names it. Whatever reconciliation lands, the
sentence above it says which of the two documents makes the zone part of the
peer's identity and which makes it part of the link's.

Quotes read for this design (whole sentences, from `rfc/full/`):

| Source | Quote | Used for |
|--------|-------|----------|
| RFC 5881 Section 2 | "This application of BFD can be used by any pair of systems communicating via IPv4 and/or IPv6 across a single IP hop that is associated with an incoming interface." | already above `reconcileZone`; the link is part of a single-hop session's identity |
| RFC 5881 Section 3 | "In this application, there will be only a single BFD session between two systems over a given interface (logical or physical) for a particular protocol." and "The BFD session must be bound to this interface." | the comment above the AC-5 refusal (lowercase "must": a description, not a gated row) |
| RFC 5881 Section 3 | "As such, both sides of a session MUST take the "Active" role (sending initial BFD Control packets with a zero value of Your Discriminator), and any BFD packet from the remote machine with a zero value of Your Discriminator MUST be associated with the session bound to the remote system, interface, and protocol." | RFC5881-3-2, the AC-10 tags; and why interop cannot discriminate (R-2) |
| RFC 5881 Section 6 | "Implementations MUST ensure that all BFD Control packets are transmitted over the one-hop path being protected by BFD." | AC-3; already quoted in `Send` |
| RFC 5880 Section 6.8.6 | "If the Your Discriminator field is zero, the session MUST be selected based on some combination of other fields, possibly including source addressing information, the My Discriminator field, and the interface over which the packet was received." | `handleInbound` (existing comment) |
| RFC 5880 Section 6.3 | "Once the remote end echoes back the local discriminator, all further received packets are demultiplexed based on the Your Discriminator field only (which means that, among other things, the source address field can change or the interface over which the packets are received can change, but the packets will still be associated with the proper session)." | why a live peer converges with the receive half broken (R-2) |
| RFC 4007 Section 6 | "Because the same non-global address may be in use in more than one zone of the same scope (e.g., the use of link-local address fe80::1 in two separate physical links) and a node may have interfaces attached to different zones of the same scope (e.g., a router normally has multiple interfaces attached to different links), a node requires an internal means to identify to which zone a non-global address belongs." | the AC-5 and AC-7 refusals; no `rfc/short/rfc4007.md` exists and RFC scope is frozen, so it is cited, not enrolled |

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

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Architectural Verification table filled, including registration over hardcoding

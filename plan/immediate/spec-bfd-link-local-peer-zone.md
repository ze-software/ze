# Spec: bfd-link-local-peer-zone

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-14 |

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
- `internal/le/interoplab/bgp/prepare.go` -- `prepareScenario`, `renderScenario`, `rfc2545LinkLocal`, the `rfc2545Scenario` and `rfc2545LinkLocalPlaceholder` constants
  → Constraint: at HEAD `prepareScenario` wraps Ze's start with `ip -6 address add <link-local>/64 dev eth0 nodad` only when `prepare.Source.Name == rfc2545Scenario`, while `renderScenario` already replaces `@ZE_LINK_LOCAL@` in any scenario file that holds it. The scenario name is a second declaration of a fact the files already state. FRR gets no link-local assignment at all.
  → Decision: the name comparison is deleted and replaced by derivation from the placeholders (see "Scenario contract"); no second name is added beside it (`ai/rules/no-layering.md`, `ai/rules/principles.md` central enumeration).
- `internal/component/bfd/config.go` -- `parseBFDSection`; `internal/component/config/parser_list.go` -- `addParsedListEntry`; `internal/component/config/tree.go` -- `AddListEntry`
  → Constraint: `parseBFDSection` iterates `single-hop-session` as a JSON map keyed by one string per entry, and the config file parser keys a list entry on its first word only. `addParsedListEntry` refuses a second entry with the same first word ("duplicate list key for single-hop-session: fe80::b"), so two entries for one link-local address on two links never reach the plugin. A-3 is refuted at the producer; Q-3 (a) (owner, 2026-10-14) is the fix.
  → Constraint: `AddListEntry` (`tree.go` 750-772) stores a duplicate key under a positional `#N`, and `cli/diff_tree.go`, `config/prune.go`, `config/mask.go`, `cli/editor_load.go` and the `MetaTree` pair or key entries by the raw map key; a key that depends on insertion order pairs the wrong entries across two trees.
  → Decision: under Q-3 (a) a multi-leaf-key entry is stored under its canonical full-key address (Q-3 table), so every map-key surface sees the full key; `#N` stays only for what it serves on every list.

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
| Config, `single-hop-session fe80::b { interface veth0 }` and `single-hop-session fe80::b { interface veth1 }` | Refused by the config file parser before the plugin: `addParsedListEntry` compares the first word only and answers "duplicate list key" | Read, not run (A-3, Q-3) |
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
| Registration over hardcoding, outbound | Yes | No new command or registry entry; one new interop scenario registers through the existing scenario table. The link-local assignment is derived from the placeholders a scenario's files carry, and the one scenario-name comparison that decided it today (`rfc2545Scenario` in `prepareScenario`) is deleted, so the next link-local scenario edits no harness file |
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
| AC-2 | (skeleton) one address on two links | each link's packet selects its own session | Unit `TestTwoLinkLocalSessionsStayApart`, `TestFirstPacketTakesTheLinkFromTheZone` (exist); kernel: the `.ci` injects My Discriminator X on `zbl0` and Y on `zbl1`, and the session on `zbl0` reports X while the one on `zbl1` reports Y, read from `show bfd sessions` by `interface` (never `show bfd session address`, journal row 2026-10-13). The kernel leg's config needs two `single-hop-session` entries for `fe80::b`, which the config file parser refuses at HEAD (A-3 refuted); it is buildable only after the Q-3 fix, and `TestTwoLinkLocalSingleHopSessionsParseApart` is that fix's red test |
| AC-3 | (skeleton) Ze sends to a link-local peer | the packet leaves the interface the session names | Unit `TestSendKeepsTheZoneForALinkLocalPeer` (exists, `destination`); kernel: frames captured with AF_PACKET on `zbl0p` carry Your Discriminator X and none carry Y; on `zbl1p` the reverse. Each frame's IPv6 source is `fe80::a`, destination `fe80::b`, hop limit 255, UDP destination 3784 |
| AC-4 | (skeleton) the reconciliation | stated once, beside the matching rule | Met at HEAD: `reconcileZone` in `engine.go`, the only caller pair being `firstPacketIndex` and `firstPacketObserved`. Review check, no new test |
| AC-5 | `bfd { single-hop-session fe80::b { profile p } }`, no `interface` | the commit (and a reload, and a start) is refused; the error names the peer `fe80::b` and says to add `interface` naming the link it is on | `TestSingleHopLinkLocalPeerNeedsInterface` (unit, `parseSingleHopSession`); `test/parse/bfd-link-local-needs-interface.ci` (`ze config validate` exits non-zero with that text) |
| AC-6 | the same with `interface zbl0`, and a global IPv6 peer with no `interface`, and an IPv4 peer with no `interface` | each is accepted | `TestSingleHopLinkLocalPeerNeedsInterface` accept rows (guards that the refusal is scoped to `fe80::/10` and not to IPv6) |
| AC-6a | a link-local `single-hop-session` in a non-default VRF (`vrf blue`), once with `interface zbl0` and once without | with `interface`: accepted; without: refused with the AC-5 text, exactly as in the default VRF. In a non-default VRF `resolveLoopDevices` binds the loop's socket to the VRF device and drops the `interface` leaf from socket binding only; the session key keeps `Interface`, and a VRF device can enslave several links, so the VRF names no link and the refusal does not depend on VRF | `TestSingleHopLinkLocalPeerNeedsInterface` VRF rows (accept and refuse) |
| AC-7 | `peer` or `local` written with a zone (`fe80::b%zbl0`) | refused, naming the leaf and saying to write the link in `interface` (Q-1, owner 2026-10-14) | `TestSingleHopPeerRefusesAZone`; `test/parse/bfd-link-local-zone-refused.ci` |
| AC-8 | a multi-hop session (`multi-hop-session`) to a link-local peer | unchanged by this spec: today's behaviour (`reconcileZone` drops the zone for multi-hop) | No new test; listed so a reviewer can see the refusal is single-hop only |
| AC-9 | FRR bfdd configured with `peer <FRR-side fe80> interface eth0`, Ze with the mirror `single-hop-session` | both sides report the session Up; FRR's `remote-id` for the peer equals Ze's `local-discriminator`, and Ze's `remote-discriminator` equals FRR's `id` | interop `bfd-link-local-frr` checker; each side's discriminator is learned from the OTHER side's packets, so neither clause is satisfied by one direction alone |
| AC-10 | the RFC 5881 Section 3 association for a link-local first packet | the link-local first-packet unit tests carry `RFC requirement: RFC5881-3-2` tags, each with a discrimination record written by `./le rfc discriminate-record` whose break is `reconcileZone` returning its argument | `./le rfc check` green for the added tags |
| AC-11 | (Q-3 (a)) from the CLI editor: `set bfd single-hop-session fe80::b interface zbl0`, then `set bfd single-hop-session fe80::b interface zbl1`, then `set bfd single-hop-session fe80::b interface zbl1 profile fast`, then commit | two entries exist, stored under `fe80::b interface zbl0` and `fe80::b interface zbl1`, and only the `zbl1` entry carries `profile fast`; no entry is stored, addressed or shown as `fe80::b#1`; `show` prints each as `single-hop-session fe80::b { interface zbl0; ... }` (first value on the opening line); `set bfd single-hop-session fe80::b profile fast` with both present creates or edits the separate entry `fe80::b` (no `interface`), never either link's entry, and the commit is then refused by AC-5 naming `interface`; `delete bfd single-hop-session fe80::b interface zbl0` removes that entry only; `set bfd single-hop-session fe80::b interface zbl0 interface zbl1` is refused, naming `interface` as given twice | `TestEditorSetsTwoEntriesForOnePeerOnTwoInterfaces` (unit, `Editor`); `test/editor/commands/bfd-two-links-one-peer.et` (the same keystrokes through the editor, then `show bfd`) |
| AC-12 | (Q-3 (a)) the two-entry config of AC-2 saved as set form, then replayed into an empty tree once through `SetParser.Parse` and once through `SetParser.ParseWithMeta` (the path drafts, change files and set-form loads with meta take) | every line of each entry starts `set bfd single-hop-session fe80::b interface zbl0` or `... fe80::b interface zbl1`, and no line sets `interface` on its own; each replayed tree has two entries under those two keys, and through `ParseWithMeta` the meta tree has two list entries under the same keys; `parseBFDSection` over each tree's JSON yields two sessions keyed on `zbl0` and `zbl1`, each with its own leaves. Red today: both replays merge the lines into one entry. Round 5 case: a change file holding only a pending `delete` of `profile` under `fe80::b interface zbl1`, loaded into an empty tree, so the entry is created by `materializeMetaPaths` from the meta key alone; every line the set-form output (with meta) prints for that entry names it, after its verb, as `bfd single-hop-session fe80::b interface zbl1`, never as `bfd single-hop-session fe80::b`, and the materialized entry holds the leaf `interface zbl1` | `TestSetFormRoundTripKeepsTwoSessionsForOnePeer` (both parsers, one subtest each; a third subtest for the materialized pending delete) |
| AC-13 | (Q-3 (a)) the web editor opens and edits the second entry of AC-2 | the entry is reached by its address as path segments (`.../single-hop-session/fe80::b/interface/zbl1/`), and its view and edit apply to the `zbl1` entry, never the `zbl0` one; the context heading (`buildContextHeading`) names one `single-hop-session` entry with key `fe80::b interface zbl1`, never a list named `interface`; the list table (`buildListTable`) heads its key column `peer` and links each row to its own address segments | `TestWebWalkAddressesMultiLeafKeyEntry` |
| AC-14 | (Q-3 (a)) two `single-hop-session fe80::b { interface zbl0 }` entries, one with no `vrf` and one with `vrf default`; then an entry with no `interface` beside one with `interface zbl0`; then a key leaf value holding whitespace (`interface "zbl 0"`); then an empty key leaf value (`vrf ""`, and `interface ""`), each through the config file parser, `SetParser` and the CLI editor | the first pair is refused as a duplicate list key (both canonicalize to `fe80::b interface zbl0`: a key leaf at its YANG default is omitted); the second pair is accepted (`fe80::b` and `fe80::b interface zbl0`; no default, absent is omitted only when absent); the third and the fourth are refused on every route, the error naming the list `single-hop-session` and the leaf, saying a key value cannot be empty or hold a space, and saying to remove the leaf or give it a value; an entry that writes `vrf default` explicitly prints it back explicitly | `TestMultiLeafKeyAbsentLeafComparesAsItsDefault` |
| AC-15 | (Q-3 (a)) gNMI Get of each entry of AC-2: `bfd/single-hop-session[peer=fe80::b][interface=zbl0]` and `[peer=fe80::b][interface=zbl1]`, and the same with `[vrf=default]` added | each returns its own entry's leaves (`zbl0` the `zbl0` profile, `zbl1` the `zbl1` one), never the other; adding `vrf=default` returns the same entry; the notification path carries `peer`, `vrf` and `interface` keys named from the schema, never `fe80::b#1` and never a single `name` key. Red today: `pathToSegments` refuses both paths with `errMultipleKeys` | `TestGetSelectsEachOfTwoMultiLeafKeyEntries` (`internal/component/gnmi/get_multikey_test.go`) |
| AC-15a | (Q-3 (a)) gNMI Get of a key leaf: `bfd/single-hop-session[peer=fe80::b][interface=zbl1]/interface`, and gNMI Set of `[peer=fe80::b][interface=zbl1]/interface` to `zbl2` | the Get returns `zbl1`, read from the `zbl1` entry (never refused as `interface` named twice, never answered from the `zbl0` entry); the Set is refused, naming `interface` as given twice, and both entries are unchanged | `TestGetSelectsEachOfTwoMultiLeafKeyEntries` key-leaf subtests |
| AC-15b | (Q-3 (a)) in the CLI editor over the AC-2 tree: `show bfd single-hop-session fe80::b interface zbl1`; then `show bfd single-hop-session fe80::b` in the set display, with a third entry `fe80::b` (no `interface`) present. Round 5 dropped the case `show` of a key-leaf path: a leaf path prints the whole tree today for every leaf (`walkSchemaNode` returns nil on a leaf, `wholeOnMiss`), which is outside this spec's goal | the first prints the `zbl1` entry only; the second prints the lines of the `fe80::b` entry only, never a line of the `zbl0` or `zbl1` entry (`FilterSetByPath` compares whole addresses). Red today by construction (the AC-2 tree cannot be built, A-3); with every part but `FilterSetByPath`'s whole-address match, the second prints all three entries' lines | `TestEditorSetsTwoEntriesForOnePeerOnTwoInterfaces` show subtests |
| AC-16 | (Q-3 (a)) the AC-2 tree, then the same tree with the `zbl0` entry deleted and re-parsed from its own brace-form output | the CLI diff shows one deleted entry (`zbl0`) and no change to the `zbl1` entry; prune keeps only that deletion; `ChangedSecretPaths` over the two trees compares the `zbl1` entry with itself. Red under a positional `#N`: the re-parsed `zbl1` entry would take the first word's slot and pair with the old `zbl0` entry | `TestDiffAfterDeletingTheFirstOfTwoMultiLeafKeyEntries` (`internal/component/cli/diff_tree_multikey_test.go`), prune and mask asserted in the same test |
| AC-17 | (Q-3 (a), round 4) `single-hop-session fe80::b { interface zbl0; vrf "" }`; then `single-hop-session fe80::b { interface zbl0 }` beside `single-hop-session fe80:0::b { interface zbl0 }` | the first is refused by the config parser with the AC-14 empty-key-value error naming `single-hop-session` and `vrf`, so it never reaches the plugin (an empty `vrf` and an absent one are one session: the plugin's `defaultVRF` reads both as `default`); the pair is accepted by the config parser (two distinct first values, because `peer` is YANG `type string`) and refused by the BFD plugin at verify, naming the peer as written in each entry and the interface, and saying both entries name one session, because `netip.ParseAddr` reads both spellings as one address and both resolve to one session key | `TestMultiLeafKeyAbsentLeafComparesAsItsDefault` (the empty `vrf`); `TestTwoEntriesForOneSessionKeyRefused` (`internal/component/bfd/link_local_config_test.go`, the two spellings) |
| AC-18 | (Q-3 (a), round 5) over the AC-2 tree, in the CLI editor, the web rename form and the web terminal: `rename bfd single-hop-session fe80::b interface zbl0 to fe80::b interface zbl2`; then `copy bfd single-hop-session fe80::b interface zbl0 to fe80::b interface zbl2`; then `rename bfd single-hop-session fe80::b interface zbl0 to fe80::c`; then `copy bfd single-hop-session fe80::b interface zbl1 to fe80::d interface zbl1`; each also in session mode, with the change file re-read | the first two are refused and both entries are unchanged; the error names the list `single-hop-session` and the key leaf `interface`, says a rename or copy changes only `peer`, and gives the fix on lines of their own (sample text: "rename refused: interface is part of the key of single-hop-session, and rename changes only peer." then "To move the entry to zbl2, delete it and create it again:" then the `delete bfd single-hop-session fe80::b interface zbl0` and `set bfd single-hop-session fe80::b interface zbl2` lines); the third moves the `zbl0` entry to the key `fe80::c interface zbl0`, and its body still holds `interface zbl0` (the key leaf carries over), so `parseBFDSection` yields a session for `fe80::c` on `zbl0`; the fourth adds `fe80::d interface zbl1` holding `interface zbl1` and leaves the source; in session mode the rename and copy records read back with the same two keys. Red today by construction (the AC-2 tree cannot be built, A-3); with every part but this rule, the third stores the entry under the key `fe80::c` while its body holds `interface zbl0` (key and body disagree, and `toMap` hands BFD a session on `zbl0` under a key that names none), the first two are a usage error that names neither the list nor the leaf, and the session record of any of them is misread by `strings.Fields` | `TestRenameAndCopyOfMultiLeafKeyEntryChangeOnlyTheFirstValue` (`internal/component/cli/editor_multikey_test.go`, CLI and session mode); `TestWebRenameOfMultiLeafKeyEntryChangesOnlyTheFirstValue` (`internal/component/web/handler_config_walk_multikey_test.go`, the rename handler and `execListEntryOp`) |

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
| A-3 | Two `single-hop-session` entries with the same peer and different `interface` reach the plugin as two sessions | YANG key is `peer vrf interface`, but the comment above `parseSingleHopSession` says the config file parser "only carries the first positional value as the list key", and `parseBFDSection` iterates a map keyed by that string | AC-2 cannot be configured through the BFD plugin at all | Read at the producer, 2026-10-14: `parseList` takes the first word as the entry key, and `addParsedListEntry` refuses a second active entry whose key matches with "duplicate list key for single-hop-session: fe80::b" (the `#N` suffix `AddListEntry` can add is reached only for lists `allowsDuplicateParsedListEntries` names, which this one is not). The entries neither merge nor reach the plugin: the config fails to parse | REFUTED at the producer; Q-3 decides the fix, and `TestTwoLinkLocalSingleHopSessionsParseApart` is its red test |
| A-4 | A frame injected with AF_PACKET on `zbl0p` is received by Ze on `zbl0` as ordinary ingress with hop limit 255 and pktinfo ifindex of `zbl0` | veth semantics; the far-end address `fe80::b` is NOT configured locally, a permanent neighbor entry on `zbl0` replaces ND, so neither direction is short-circuited through `lo` | The `.ci` reports the wrong interface or nothing | The `.ci`'s green run, plus its red run | unvalidated |
| A-5 | The pinned FRR image's start can be wrapped by a shell that assigns `@FRR_LINK_LOCAL@` to `eth0` and then execs the image's own entrypoint and command, and bfdd then accepts `peer <Ze's fe80> interface eth0` | `prepareScenario` wraps Ze's start the same way today (`dockerEntrypointFlag` `/bin/sh`, `shellErrexitCommand`); the FRR peer already holds `NET_ADMIN`; the FRR image's entrypoint and command were not read for this design | The FRR assignment needs another route (for example zebra's own interface address), and the Scenario contract's step 2 is rewritten before code | `docker image inspect` of the pinned FRR image before the wrapper is written; then the scenario's first green run | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | The send half cannot be discriminated on Linux by reverting `sendZone` alone: the ifindex pin and the device binding each still put the packet on the right link | `.ci` stays green with `sendZone` returning empty | Recorded, not hidden: AC-3's kernel proof is discriminated by reverting the link route as a whole (`sendZone` empty AND `pinsToLink` false, with two interfaces so the socket is not device-bound); `sendZone` alone is proven by the unit test |
| R-2 | The interop scenario passes with the receive fix reverted, because both sides are Active and converge through `byDiscr` | Red run stays green | Expected and stated: `bfd-link-local-frr` is acceptance evidence; the discrimination is the `.ci` (interop.md, "Prove a scenario discriminates") |
| R-3 | AC-5/AC-7 refuse a config an operator already runs | A config load error after upgrade | Pre-release (`ai/rules/pre-release.md`); and a no-interface link-local session today sends on a link the kernel picks, so it was not working on a multi-link box |
| R-4 | `internal/le/interoplab/bgp/{names.go,check_special.go,prepare.go,check_rfc.go,bgp_test.go}` carry another session's uncommitted hunks (none of the package's `.go` files was dirty on 2026-10-14) | `git status` at implementation start | Register the scenario in the files only after that work lands, or carry the hunks per `ai/rules/git-safety.md`; never reformat those files |
| R-5 | The refusal placed in `pluginConfig.validate` would stale discrimination records keyed on `bfd/config.go::validate` (`TestRFC5881SingleHopActiveProfileAccepted`, `TestRFC5881SingleHopPassiveProfileRejected`) | `./le rfc check` names them | Put the refusal in `parseSingleHopSession`, which no record names (checked in `rfc/discrimination/rfc588{0,1,2,3}.json`) |
| R-6 | Two veth pairs in the shared test namespace collide with a parallel run | `RTNETLINK answers: File exists` | `option=exclusive:group=bfd-ports`, `ip link add` preceded by a delete of the same names, and removal at the end like `bfd-first-packet-pktinfo-v6.ci` |
| R-7 | `features/bfd-liveness-detection.md` is dirty in another session | `git status` | Edit only the row this spec owns, carry or wait per git-safety |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A link-local BFD config is refused that should load (AC-5/AC-7), or a link-local session stays Down |
| How is it reverted? | Single commit revert per phase; tests and scenario are additive |
| Who else touches this path? | Under Q-3 (a), every list the config file parser, `SetParser` (with and without meta), the brace and set-form serializers, the CLI editor, the web editor and gNMI address goes through the one canonical-address codec, with behaviour changing only for multi-leaf-key lists (exactly the two BFD lists among config lists; Q-3 table); diff, prune and mask pair by map key unchanged, which under a canonical key is the full key. The interop harness's `prepareScenario` serves every BGP-package scenario; only those whose files carry a link-local placeholder are affected (today `bgp-rfc2545-linklocal-nexthop-frr`). The interop package session (`internal/le/interoplab/bgp/`, R-4), `spec-yang-feature-opt-ins-owned-by-feature` (moves OSPF's inline BFD timers into the BFD plugin), `spec-bgp-link-local-only-next-hop` (BGP next hops, no BFD overlap: it names no BFD file) |

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
| `TestTwoLinkLocalSessionsStayApart` | same file (exists) | AC-2; gains tag `RFC5881-3-2 positive` (the eth0 packet is associated with the eth0 session; see the tagged-test table for why not `negative`) | exists, tag owed |
| `TestFirstPacketTakesTheLinkFromTheZone` | same file (exists) | AC-2, transport reports no ingress interface; gains tag `RFC5881-3-2 positive` | exists, tag owed |
| `TestSendKeepsTheZoneForALinkLocalPeer` | `internal/component/bfd/transport/udp_zone_test.go` (exists; the skeleton named `udp_test.go`) | AC-3 at `destination`; no tag (RFC 4007 has no summary, and RFC5881-6-1 is already carried by the pin tests) | exists |
| `TestSingleHopLinkLocalPeerNeedsInterface` | `internal/component/bfd/link_local_config_test.go` (new) | AC-5 refuse row; AC-6 accept rows (link-local with `interface`, global IPv6 without, IPv4 without); AC-6a VRF rows (`vrf blue` with `interface` accepted, without refused); the error text names the peer and `interface` | new |
| `TestSingleHopPeerRefusesAZone` | same new file | AC-7, `peer` and `local` each with a zone | new |
| `TestTwoLinkLocalSingleHopSessionsParseApart` | same new file | AC-2 plugin-config leg: a config text with two entries, one peer, two interfaces, parsed through the config file parser and `parseSections`, yields two sessions keyed on `zbl0` and `zbl1` | new; red at HEAD by construction (A-3 refuted), it is the red test of the Q-3 fix |
| `TestMultiLeafKeyAbsentLeafComparesAsItsDefault` | `internal/component/config/parser_list_multikey_test.go` (new) | AC-14 under Q-3 (a): absent `vrf` and `vrf default` give one canonical key (refused as duplicate); absent `interface` differs from `interface zbl0` (accepted); two entries differing only in `interface` accepted under `fe80::b interface zbl0` and `fe80::b interface zbl1`; whitespace in a key leaf value refused; an empty key leaf value (`vrf ""`, `interface ""`) refused through the file parser, `SetParser` and the editor, naming the list and the leaf (AC-14, AC-17); explicit `vrf default` prints back explicitly | new, under (a) |
| `TestSetFormRoundTripKeepsTwoSessionsForOnePeer` | `internal/component/config/serialize_set_multikey_test.go` (new) | AC-12 under Q-3 (a): set-form save, then replay through `Parse` and through `ParseWithMeta`, keeps two entries (and two meta entries), and `parseBFDSection` yields two sessions; a pending delete under `fe80::b interface zbl1` materialized by `materializeMetaPaths` prints under the full address | new, under (a); red at HEAD (both replays merge) |
| `TestEditorSetsTwoEntriesForOnePeerOnTwoInterfaces` | `internal/component/cli/editor_multikey_test.go` (new) | AC-11 under Q-3 (a): two `set` commands make two entries; the path with no `interface` addresses the separate `fe80::b` entry; a key leaf given twice is refused; full-key `delete` removes one; `show` prints the first value on each opening line; AC-15b: `show` of one entry, and of entry `fe80::b` in the set display beside the two link entries | new, under (a); red at HEAD (the second `set` rewrites the first entry) |
| `TestRenameAndCopyOfMultiLeafKeyEntryChangeOnlyTheFirstValue` | `internal/component/cli/editor_multikey_test.go` (same new file) | AC-18 through `cmdRename` and `cmdCopy`, in direct and session mode: a destination changing `interface` refused naming the list and the leaf, both entries unchanged; a rename changing `peer` keeps `interface` in the key and the body; the change-file rename and copy records read back with keys holding spaces | new, under (a) |
| `TestWebRenameOfMultiLeafKeyEntryChangesOnlyTheFirstValue` | `internal/component/web/handler_config_walk_multikey_test.go` (same new file as AC-13) | AC-18 through the web rename handler (`new-key` holding a full address) and the web terminal's `execListEntryOp`; a web rename to `fe80::c interface Eth0` of an entry under `interface Eth0` succeeds and the stored key keeps `Eth0` (the handler no longer lowercases, round 6), and a web rename of an entry stored as `Eth0` in a single-key list finds it | new, under (a) |
| `TestKeyMoveRecordRoundTripsQuotedKeys` | `internal/component/config/change_file_test.go` (exists) | the rename and copy records (round 6): `formatKeyMoveLine` then `parseKeyMoveLine` returns `OldKey` and `NewKey` unchanged for keys `fe80::b interface zbl0`, `a"b`, `a\b`, `it's`, and one holding all of `"`, `\`, `'` and a space; `splitQuotedTokens(quoteIfNeeded(s)) == s` for each; one old raw line (`rename bfd single-hop-session fe80::b#2 to fe80::c`, as written before this change) loads with `OldKey` `fe80::b#2`; an old raw line whose key starts with an unterminated `'` is refused naming the line | new, under (a); red at HEAD for the space-holding key (`strings.Fields`) |
| `TestWebWalkAddressesMultiLeafKeyEntry` | `internal/component/web/handler_config_walk_multikey_test.go` (new) | AC-13 under Q-3 (a): the web walk reaches the second entry by its address segments | new, under (a) |
| `TestGetSelectsEachOfTwoMultiLeafKeyEntries` | `internal/component/gnmi/get_multikey_test.go` (new) | AC-15 under Q-3 (a): Get of each of two entries returns the right one, with and without `vrf=default`; the notification path names the key leaves; AC-15a: Get of the `interface` key leaf returns `zbl1`, Set of it is refused as named twice | new, under (a); red at HEAD (`errMultipleKeys`) |
| `TestDiffAfterDeletingTheFirstOfTwoMultiLeafKeyEntries` | `internal/component/cli/diff_tree_multikey_test.go` (new) | AC-16 under Q-3 (a): diff, prune and changed-secret pairing after the first entry is deleted and the tree re-parsed | new, under (a) |
| `TestTwoEntriesForOneSessionKeyRefused` | `internal/component/bfd/link_local_config_test.go` (new file above) | AC-17: two entries that resolve to one session key (`fe80::b` and `fe80:0::b` on `zbl0`) are refused by the plugin | new, under (a) |
| `TestLinkLocalAssignmentFollowsThePlaceholders` | `internal/le/interoplab/bgp/prepare_link_local_test.go` (new) | the interop harness: a scenario whose files carry `@ZE_LINK_LOCAL@` gets Ze's start wrapped with the assignment and FRR's not; one carrying `@FRR_LINK_LOCAL@` gets FRR's wrapped; one carrying both gets both; one carrying neither gets neither, even when it is named `bgp-rfc2545-linklocal-nexthop-frr` (proves the name no longer decides) | new |

### RFC-tagged tests this spec changes (exact list)

| Test | File | Change | Discrimination owed |
|------|------|--------|---------------------|
| `TestFirstPacketSelectsALinkLocalPeer` | `internal/component/bfd/engine/link_local_first_packet_test.go` | tag ADDED: `RFC5881-3-2 positive` | new record, break `reconcileZone` → identity |
| `TestFirstPacketTakesTheLinkFromTheZone` | same | tag ADDED: `RFC5881-3-2 positive` | new record, same break |
| `TestTwoLinkLocalSessionsStayApart` | same | tag ADDED: `RFC5881-3-2 positive` | new record, break `reconcileZone` → identity (the eth0 session is not selected, the first assertion fires). Not `negative`: the second assertion (the eth0 packet did not reach the eth1 session) is reached only once the first passes, and `handleInbound` gives one packet to at most one `byKey` entry while the two sessions are two machines under two `api.Key` values (`Interface` is part of it). No break in `firstPacketKey`, `reconcileZone`, `without` or `keyRelaxations` makes one packet set both machines; the only break that does merges the two sessions in `api.Key`, outside the keying code. So the negative clause cannot be exercised here and the tag claims the positive clause only |
| none other | - | No existing tagged test is edited. The existing RFC5881-3-2 units (`engine/rfc5881_test.go`, `engine/rfc5881_first_packet_binding_test.go`), the RFC5882-4.4-1 units in `api/rfc5882_session_identity_test.go` (which use `fe80::` link tables) and the RFC5881-6-2 units in `transport/rfc5881_subnet_exemption_test.go` (which exempt `fe80::2`) are unaffected: `Canonical`, `reconcileZone`, `Send` and `egressLink.reaches` do not change, and the refusal lives in `parseSingleHopSession`, which no record names | none |

#### Owner approval commands

The owner approved all RFC-tagged test changes on 2026-10-13 ("approve all RFC-tagged test changes"), but this spec needs no command. The three units above exist at HEAD `051876c3f4` with no `RFC requirement:` tag (`link_local_first_packet_test.go` carries none), and the gate asks only about a unit that was tagged in the committed text, so adding a tag to them needs no approval. No existing tagged unit is edited.

0 commands.

The claim of each added tag states what the body checks and no more: "a Your
Discriminator zero packet from a zone-qualified link-local source is associated
with the session bound to that peer and interface". The units hand-build
`fe80::1%eth0`; the kernel's own zone is proven by the `.ci` (A-1), not by them.

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
| Config | `bfd { enabled true; bind-v6 true; profile fast {...}; single-hop-session fe80::b { interface zbl0; local fe80::a; profile fast } }` and the same on `zbl1`, which needs the Q-3 fix (A-3 refuted: the config file parser refuses the second entry at HEAD) |
| Fixture | `bfd/bfd-first-packet-link-local` registered in `internal/test/fixture/register_bfd.go`; it writes a raw Ethernet/IPv6/UDP/BFD Down frame (source `fe80::b`, hop limit 255, source port in 49152-65535, My Discriminator X on `zbl0p`, Y on `zbl1p`, Your Discriminator 0) every 200 ms, reads `show bfd sessions`, and captures Ze's frames on `zbl0p`/`zbl1p` with AF_PACKET |
| Fixture lifecycle | the 200 ms writer stops when the fixture's run ends (pass, fail or timeout) and is never left running past it; every AF_PACKET socket, the two writers' and the two captures', is closed on every exit path, deferred at open as the pktinfo fixture does with `conn.Close()`; the fixture starts no goroutine that outlives its run |
| Pass line on stderr | one `OK:` line per link naming the interface and the discriminator, plus one `OK:` per link for the capture |
| Red runs to record | (1) `reconcileZone` returns its argument: both sessions keep `remote-discriminator` 0; (2) `sendZone` returns empty AND `pinsToLink` returns false: Ze's frames for one session appear on the other link or nowhere (R-1) |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bfd-link-local-frr` | `test/interop/scenarios/bfd-link-local-frr/` (`ze.conf`, `frr.conf`) | FRR bfdd | AC-9: a BFD-plugin-pinned link-local single-hop session reaches Up against FRR, each side's discriminator learned from the other's packets. Acceptance evidence, not discrimination (R-2) | new |

Scenario contract, link-local addresses. A scenario opts in to a link-local
assignment because its files carry the placeholder, never because of its name:

| Placeholder | Address it renders to | Owner whose `eth0` gets it before start |
|-------------|-----------------------|------------------------------------------|
| `@ZE_LINK_LOCAL@` (exists) | `fe80::` with the lab IPv6 prefix's first six bytes in the interface identifier and Ze's host number (2) in the last byte, as `rfc2545LinkLocal` builds it today | `ze` (Host 2) |
| `@FRR_LINK_LOCAL@` (new) | the same construction with FRR's host number (3, `peerFRR` in `scenarioPeers`) in the last byte | `frr` (Host 3) |

1. `renderScenario` already tests each file for `@ZE_LINK_LOCAL@`; it reports
   which of the two placeholders it rendered across the scenario's files.
2. `prepareScenario` wraps the start of each owner whose placeholder was
   rendered with `ip -6 address add <address>/64 dev eth0 nodad`, then execs the
   owner's normal start. For Ze that is today's wrapper, unchanged. For FRR the
   wrapper execs the image's own entrypoint and command, which the implementer
   reads from the pinned image (`docker image inspect`, Entrypoint and Cmd)
   before writing it (A-5).
3. The `rfc2545Scenario` constant and the `prepare.Source.Name == rfc2545Scenario`
   comparison are deleted. No scenario name decides an assignment. The address
   builder becomes one function of the network and a host number, so Ze's and
   FRR's addresses come from one construction; `check_rfc.go`'s caller follows.
4. `bgp-rfc2545-linklocal-nexthop-frr` keeps working through the same rule: its
   `ze.conf` carries `@ZE_LINK_LOCAL@`, so Ze's `eth0` is assigned as today. It
   MUST stay green, and its run is pasted at closure beside `bfd-link-local-frr`'s.

Ze's `ze.conf` holds only the `bfd` root
(no client opt-in, so the owner's opt-in rule is met trivially), with
`single-hop-session @FRR_LINK_LOCAL@ { interface eth0; local @ZE_LINK_LOCAL@; profile ... }`;
FRR's `frr.conf` holds `bfd` / `peer @ZE_LINK_LOCAL@ interface eth0` / a profile
at 300 ms. Both files carry both placeholders, so both containers are assigned.
The checker is registered in `internal/le/interoplab/bgp/checkers.go` beside
`ospfv3-bfd-frr`, takes both addresses from the same builder `prepareScenario`
uses, and reads FRR `show bfd peer <Ze's address> interface eth0 json` and Ze
`show bfd sessions`.

## Goal Validation Plan

| Goal (Task) | Evidence that will be pasted at closure |
|-------------|------------------------------------------|
| A link-local peer's first packet selects its session on a real kernel | `bfd-first-packet-link-local.ci` green in the QEMU guest, AND its recorded red with `reconcileZone` reverted |
| Two links, one address, two sessions | the same `.ci`'s per-link `OK:` lines (X on `zbl0`, Y on `zbl1`) |
| Ze's packets leave the named link | the same `.ci`'s capture lines, and its recorded red with the link route reverted (R-1) |
| Works with a second implementation | `bfd-link-local-frr` green, with the `Docker.Build` image ID line quoted (interop.md); `bgp-rfc2545-linklocal-nexthop-frr` green on the same build, proving the placeholder-derived assignment kept it working |
| Misconfiguration is refused, not silently mis-sent | the two `test/parse` `.ci` files green, and the unit tests red before the refusal lands |

## Files to Modify
- `internal/component/bfd/config.go` - `parseSingleHopSession`: refuse a link-local `peer` without `interface`, and a zone on `peer` or `local` (AC-5, AC-7), with the RFC comments below
- `internal/component/bfd/yang/ze-bfd-conf.yang` - `single-hop-session` `peer` and `interface` descriptions state the requirement and the refusal
- `internal/component/bfd/engine/link_local_first_packet_test.go` - add the three `RFC requirement:` tags (AC-10)
- `internal/test/fixture/register_bfd.go` - register `bfd/bfd-first-packet-link-local`
- `internal/le/interoplab/bgp/prepare.go` - `renderScenario` reports which link-local placeholders it rendered and renders `@FRR_LINK_LOCAL@`; `prepareScenario` assigns each rendered placeholder's address to its owner's `eth0` (Ze and FRR); the `rfc2545Scenario` constant and its name comparison are deleted; the address builder takes a host number (Scenario contract)
- `internal/le/interoplab/bgp/check_rfc.go` - the `bgp-rfc2545-linklocal-nexthop-frr` checker calls the renamed address builder with Ze's host
- `internal/le/interoplab/bgp/checkers.go`, `names.go` (whichever the scenario table needs; R-4) - register `bfd-link-local-frr`
- Config component, per Q-3 (a) (owner, 2026-10-14): the canonical full-key map key (Q-3 table, parts 1 to 5): `internal/component/config/yang_schema.go` (the list node learns its key leaf names and leaves); `internal/component/config/parser_list.go` (`addParsedListEntry` canonicalizes the key before its unchanged duplicate loop); `internal/component/config/setparser.go` (set, inactive and delete list branches parse the address through the codec); `internal/component/config/setparser_meta.go` (`walkAndSetWithMeta` set and `walkAndRecordDeleteMeta` delete-meta walks, the same); `internal/component/config/schema.go` (`LookupTokenPath` consumes the address, `ValidateListKey` validates the first value); `internal/component/config/serialize.go` (`serializeListBlocks`, `serializeListMultiBlock`, the inline-list case of `serializeNode`), `serialize_annotated.go` (`serializeAnnotatedList`, `serializeAnnotatedInlineList`), `serialize_blame.go` (`serializeBlameList`, `serializeBlameInlineList`): brace-form opening lines print the first value only; `internal/component/config/serialize_set.go` (`serializeSetList`, `serializeSetInlineList`, `serializeSetMetaList`, `writeInlineListLines`) and `serialize_annotated.go` (`serializeAnnotatedSetList`, `serializeAnnotatedSetInlineList`): the six set-form emitters write on every line the address split from the map key, each token quoted on its own, and do not emit its key leaves as lines of their own; `serialize_set.go` `FilterSetByPath` takes the schema and matches a whole address, not a text prefix; `internal/component/config/tree.go` (doc comments of `AddListEntry` and `toMap` state the canonical key, and of `RenameListEntry` and `CopyListEntry` that a multi-leaf-key caller passes keys differing only in the first value; no logic change); `internal/component/cli/editor_walk.go` (`walkOrCreateIn`, `walkPath`, `walkOrCreateMeta`, `walkMetaReadOnly`), `editor.go` (`walkSchemaNode`, `ResolveLeafListValue`, `autoSelectListEntry`), `editor_commands.go` (`walkOrCreate`, `walkSchema`, `isListKeyLeafPath`, `DeleteByPath`, `resolveListTarget`), `editor_annotated.go` (`walkMetaPath`), `editor_draft.go` (`rename`, both sides), `completer.go` (`completeEditPath`, `getEntry`, `navigateTreeToPath`; `listKeyCompletions` offers first values, then key leaf names), `diff_tree.go` (`diffList` opening line prints the first value; pairing unchanged), `model_commands_edit.go` (`cmdSet` parses the address over every token; `cmdRename`, `cmdCopy` read the source address, `to`, then the destination tokens), `model_commands_show.go` (passes the schema to `FilterSetByPath`), and in `editor_commands.go` also `Editor.RenameListEntry` and `Editor.CopyListEntry` (the first-value-only rule and its refusal, AC-18); `internal/component/config/change_file.go` (`materializeMetaPaths` writes the meta key's pairs into the entry it creates; `formatKeyMoveLine` quotes each key as one token and `parseKeyMoveLine` splits with the codec's `splitQuotedTokens`, the inverse of `quoteIfNeeded`, round 6), `serialize.go` (`quoteIfNeeded` moves to the codec unchanged); the `FilterSetByPath` test callers `internal/component/config/serialize_set_test.go` (62, 64) and `internal/component/config/setparser_nop_test.go` (187) pass the schema; `internal/component/config/change_file_test.go` gains `TestKeyMoveRecordRoundTripsQuotedKeys`; `internal/component/web/cli_terminal.go` (`execListEntryOp` reads the source address, `to`, then the destination tokens; the `FilterSetByPath` call passes the schema), `internal/component/web/editor.go` (`EditorManager` rename and copy pass the destination tokens through), the rename handler in `handler_config_entry.go` (splits `new-key` into tokens and drops its `strings.ToLower` of `new-key` and of the old key, round 6); `internal/component/web/handler_config_walk.go` (`walkTree`, `walkSchema`, `isListEntryPath`, `validateUniqueOnSet`, `collectListKeys`), `internal/component/web/handler_config_entry.go` (`resolveNamedListEntryPath`, `resolveParentDefaults`, `HandleConfigAddWithAuthorizer`) and `internal/component/web/fragment.go` (`buildContextHeading`, `buildListColumn`, `buildListTable`); `internal/component/gnmi/path.go` (`pathToSegments` builds the canonical address from `elem.Key` as one segment, `segmentsToPath` names each key from the schema), `get.go`, `set.go` and `subscribe.go` (pass the schema; `set.go` hands the editor the address split into tokens); every other list path walker is named in "List path walkers" with the reason it does not change; `internal/component/bfd/config.go` (`parseSingleHopSession` and `parseMultiHopSession` take the peer from the address's first value and their stale comment is rewritten; `parseBFDSection` refuses two entries resolving to one session key, AC-17). Unchanged by design, each confirmed at the producer: `config/prune.go`, `config/mask.go`, `cli/editor_load.go` pairing, `config/meta.go`. Under (b) none of these; under (c) `internal/component/bfd/config.go` only
- Under (a), `docs/architecture/config/syntax.md` and `docs/architecture/config/yang-config-design.md` (the pages `ai/CODE-TO-DOCS.md` names for `parser_list.go`, `serialize_set.go` and `setparser.go`) and `docs/guide/command-reference.md` (named for `setparser.go`) state that a multi-leaf-key entry is addressed by all its key leaves, the canonical address it is stored and delivered under (plugin JSON contract), how set form writes it, and that brace form keeps the first value on the opening line; `docs/guide/gnmi.md` (the gNMI page `ai/CODE-TO-DOCS.md` names for the package; it names no page for `path.go` itself) states how a multi-key path elem addresses an entry
- `docs/architecture/bfd.md` - the zone paragraph gains the config refusal; anchors to `parseSingleHopSession`
- `docs/guide/bfd.md` - a link-local `single-hop-session` example with `interface`, and the two refusals
- `docs/architecture/testing/interop.md` - Scenario Inventory row and a short "BFD over link-local" note (acceptance-only, R-2); "Container Addresses" states that a placeholder in the scenario's files (`@ZE_LINK_LOCAL@`, `@FRR_LINK_LOCAL@`) is what assigns the link-local, with each address and its owner, and drops any wording tying it to the RFC 2545 scenario's name
- `features/bfd-liveness-detection.md` - evidence row for link-local single-hop (R-7)
- `rfc/discrimination/rfc5881.json` - written by `./le rfc discriminate-record`, never by hand

## Files to Create
- `internal/component/bfd/link_local_config_test.go`
- `internal/le/interoplab/bgp/prepare_link_local_test.go`
- `internal/test/fixture/bfd_fixture_link_local.go`
- `test/bfd/bfd-first-packet-link-local.ci`
- `test/parse/bfd-link-local-needs-interface.ci`, `test/parse/bfd-link-local-zone-refused.ci`
- `test/interop/scenarios/bfd-link-local-frr/ze.conf`, `frr.conf`
- Under Q-3 (a): `internal/component/config/list_key.go` (the one canonical-address codec: build, parse from path tokens, split; it also declares the one token quote/unquote pair, `quoteIfNeeded` moved from `serialize.go` and its inverse `splitQuotedTokens`, round 6; the parser, `SetParser`, the CLI, the web, gNMI and the brace and set printers call it); `internal/component/config/parser_list_multikey_test.go`, `internal/component/config/serialize_set_multikey_test.go`, `internal/component/cli/editor_multikey_test.go`, `internal/component/cli/diff_tree_multikey_test.go`, `internal/component/web/handler_config_walk_multikey_test.go`, `internal/component/gnmi/get_multikey_test.go`; `test/editor/commands/bfd-two-links-one-peer.et`

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

1. **Phase: Q-3 (a) fix first** -- Q-3 answered (a) by the owner, 2026-10-14; design round 7 must be clean before this phase starts. `TestTwoLinkLocalSingleHopSessionsParseApart` red (the parser's "duplicate list key"), then the fix, then green; AC-2's `.ci` leg depends on it. The phase is all five parts of the Q-3 table, in order: the codec and the schema's key leaves, then the parser with `TestMultiLeafKeyAbsentLeafComparesAsItsDefault` (AC-14); the brace and set printers and both `SetParser` paths with `TestSetFormRoundTripKeepsTwoSessionsForOnePeer` (AC-12) and `TestDiffAfterDeletingTheFirstOfTwoMultiLeafKeyEntries` (AC-16); the CLI with `TestEditorSetsTwoEntriesForOnePeerOnTwoInterfaces` and the `.et` (AC-11); the web with `TestWebWalkAddressesMultiLeafKeyEntry` (AC-13); gNMI with `TestGetSelectsEachOfTwoMultiLeafKeyEntries` (AC-15); the BFD first-value peer and `TestTwoEntriesForOneSessionKeyRefused` (AC-17); rename and copy with `TestRenameAndCopyOfMultiLeafKeyEntryChangeOnlyTheFirstValue` and `TestWebRenameOfMultiLeafKeyEntryChangesOnlyTheFirstValue` (AC-18); each red first, and the config and gNMI pages in the same phase. The `#N` suffix is never used to tell two multi-leaf entries apart, so no step keeps it beside the canonical key.
2. **Phase: Config refusal** -- `TestSingleHopLinkLocalPeerNeedsInterface`, `TestSingleHopPeerRefusesAZone` red, then the refusal in `parseSingleHopSession`, then the two `test/parse` `.ci`; YANG descriptions, `docs/guide/bfd.md`, `docs/architecture/bfd.md` in the same phase.
3. **Phase: Kernel proof** -- the fixture and `bfd-first-packet-link-local.ci`; record both red runs in the spec (QEMU guest, `./le test` scoped to the one `.ci`).
4. **Phase: Tags** -- AC-10 tags, then `./le rfc discriminate stem rfc5881 report <path>` and `./le rfc discriminate-record` for each.
5. **Phase: Interop** -- `TestLinkLocalAssignmentFollowsThePlaceholders` red, then the placeholder-derived assignment in `prepare.go` with the `rfc2545Scenario` comparison deleted, then green; A-5 checked against the pinned FRR image; `bfd-link-local-frr` (R-4 first), green run with the image ID quoted, and `bgp-rfc2545-linklocal-nexthop-frr` green on the same build; Scenario Inventory row and the "Container Addresses" edit; `features/bfd-liveness-detection.md`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-18 each at file:line or test name (AC-11 to AC-18 under Q-3 (a)) |
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
| Design round 7 not clean at implementation start | stop; the Q-3 (a) phase is not started until its narrow review returns 0 BLOCKER, 0 ISSUE |
| `bgp-rfc2545-linklocal-nexthop-frr` red after the `prepare.go` change | the placeholder derivation missed its `@ZE_LINK_LOCAL@`; fix the derivation, never restore the name comparison |
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
| Q-1 | A `peer` (or `local`) written with a zone, `fe80::b%eth0`: refuse it and point to `interface`, or accept it and fold the zone into `interface` (refusing only a mismatch)? | Refuse (Key Design Decisions) | **Owner, 2026-10-14: accept (recommendation)**: refuse a zoned `peer` or `local` and point to `interface` (AC-7); the fold-into-`interface` branch is not built |
| Q-2 | AC-5 refuses a link-local single-hop peer with no `interface`, even on a box where only one link carries `fe80::/64` and `Canonical` would derive it. Accept that strictness? | Yes | **Owner, 2026-10-14: accept (recommendation)**: yes, AC-5 refuses a link-local single-hop peer with no `interface` on every box |
| Q-3 | A-3 is refuted: the config file parser refuses a second `single-hop-session` entry for one address ("duplicate list key"), so one link-local address on two links cannot be configured through `bfd`. Which way: (a) every config surface (file parser, CLI editor, web editor, set-form save and replay) honours every leaf of a multi-leaf YANG list key; (b) prove AC-2 on a kernel only through a runtime client (OSPFv3 on two links) and keep the plugin list one entry per address; (c) let the list key carry the zone (`single-hop-session fe80::b%zbl0`) and fold it into `interface`? | (a). See "Q-3 options and blast radius" below | **Owner, 2026-10-14: accept (recommendation)**: (a), every config surface honours every leaf of a multi-leaf YANG list key through the canonical full-key map key (31 production files, the six new config-surface test files, three existing test files and the `.et`). The scope widening into the config, CLI, web and gNMI components is the owner's decision, now made. (b) and (c) are not built. Design round 7, the narrow review round 6 deferred to this answer, is owed before implementation (Design Review Log) |
| Q-4 | `show bfd session address <peer>` answers one of several sessions by map order (journal, `key-omits-a-fact-the-builder-uses.md`, 2026-10-13). Fold it into this spec, since two-link link-local is its most visible case, or leave it to the journal? | Leave it to the journal; the tests here read `show bfd sessions` | **Owner, 2026-10-14: accept (recommendation)**: stays a journal row (`plan/journal/key-omits-a-fact-the-builder-uses.md`, 2026-10-13 row, present); not in this spec's scope |

### Q-3 options and blast radius

Answered (a) by the owner on 2026-10-14. The (b) and (c) rows below are kept as the rejected alternatives, and no step of this spec builds them.

| Option | What changes | Blast radius | Keeps story #2 and AC-2's plugin leg |
|--------|--------------|--------------|--------------------------------------|
| (a) every config surface honours the whole YANG key, through a canonical full-key map key (design round 3) | One root change: a multi-leaf-key list entry is STORED under its canonical address, never under its first key word plus a positional `#N`. The canonical address is the first key leaf's value, then, in YANG key order, one `name value` pair for each other key leaf whose value differs from its YANG `default` (absent, or equal to the default, is omitted; a leaf with no default is omitted only when absent), joined by single spaces: `fe80::b` for an entry with no `interface` and `vrf` absent or `default`, `fe80::b interface zbl1` for the second link, `198.51.100.9 local 10.0.0.1` for a multi-hop entry. A space cannot occur inside a value and no field is empty, because the codec refuses a key leaf value of a multi-leaf-key list that holds whitespace or is empty, with an operator error that names the list and the leaf, says a key value cannot be empty or hold a space, and says to remove the leaf or give it a value (round 4: an empty `vrf` and an absent one are one session anyway, AC-17, so the empty value buys nothing). The address reaches text only through the codec, which writes each token through `quoteIfNeeded` on its own; it is never written raw as one string. The entry's own values keep the non-first key leaves exactly as written (an explicit `vrf default` stays explicit); the map key is derived from them once, at creation; a key leaf is never rewritten in place, and a rename or copy changes only the first value (part 3, round 5), so the two cannot diverge. Single-leaf-key lists are untouched: the codec returns their key as written. Two distinct full keys are two distinct map keys, so the `#N` suffix is deleted as the way two multi-leaf entries are told apart; `AddListEntry` keeps `#N` only for what it serves on every list today (an inactive duplicate, the `allowsDuplicateParsedListEntries` lists), unchanged. Five parts. (1) Codec, one new file in the config component: build the canonical address from a first value plus key leaf values (parser, gNMI); parse an address from path tokens (the address consumes the first token as the first key value, then consumes a `name value` pair while the next token is the name of a non-first key leaf of that list AND a token follows it; otherwise the address ends there and the next token is read as it is today: a child name, a leaf, or a `set` value. A key leaf named twice in one address is refused; each value is validated against its own leaf. The rule runs left to right with no choice point, so no path reads two ways: `... fe80::b interface` ends the address at `fe80::b` and reads the `interface` leaf of the entry with no `interface`, which is absent; `... fe80::b interface zbl1 interface` reads the `zbl1` entry's `interface` leaf; `... fe80::b interface zbl1` with nothing after it is the `zbl1` entry itself, and a `set` of it reaches or creates that entry, which is the same fact as entry `fe80::b` holding `interface zbl1`; `set ... fe80::b interface zbl1 interface zbl2` is refused as `interface` named twice, so a key leaf is never rewritten in place; a value spelled like a key leaf name (`interface interface`) is still one pair, because a name is consumed only together with the token after it; and an entry cannot hold a child named like a key leaf other than that leaf, because YANG sibling names are unique); split an address into its first value and its pairs (display, gNMI out). The list node learns its key leaf names and leaves (today `KeyName` holds the whole `"peer vrf interface"` string and `KeyLeaf` stays nil, `yang_schema.go` 960-966). (2) File parser: `addParsedListEntry` canonicalizes the key from the first word and the parsed entry's key leaf values before its existing duplicate loop, which then compares full keys with no other change (so `vrf` absent equals `vrf default`, and `interface` absent equals only `interface` absent). (3) Every addressing site replaces "the key is the next path word" with the codec's address parse and indexes the map by the result; creating an entry writes the address's explicit pairs into it. Sites: `SetParser` set, inactive and delete list branches (`setparser.go`, each indexing `tokens[0]`); `walkAndSetWithMeta` and `walkAndRecordDeleteMeta` (`setparser_meta.go` 375-397 and 442-457, reached by `loader.go` 267/283, `change_file.go` 369, `cli/editor_draft.go` 980/1113/1206/1259/1393 and `web/cli_terminal.go` 715, so drafts, change files and set-form loads with meta); `materializeMetaPaths` (`config/change_file.go` 386-397, round 5), which creates an entry keyed by the meta key with `AddListEntry` and today writes no key leaf, so it writes the pairs the codec splits from that key into the entry it creates, like every other creator; the CLI editor (`walkOrCreateIn` and `walkPath`, `cli/editor_walk.go` 41-94, and its meta walks; `editor_commands.go`; `cmdSet` in `cli/model_commands_edit.go`, which today splits off the last token as the value before it walks, so it runs the address parse over every token first and, when the last token is consumed as a pair value, reaches or creates that entry instead of setting a key leaf of entry `fe80::b`); rename and copy (round 5). The one rule: a rename or copy of a multi-leaf-key entry changes only the first key value. `config/tree.go` `RenameListEntry` 905 and `CopyListEntry` 938 move or clone an entry under a new map key and never touch its key leaves, and the tree holds no schema, so the rule lives in their one schema-aware funnel, `Editor.RenameListEntry` and `Editor.CopyListEntry` (`cli/editor_commands.go` 701 and 727, before the session branch, so both modes; `web/editor.go` `EditorManager` passes through). The destination is read by the part 1 rule: its first token is the new first value, and every pair it names must equal the source entry's pair for that key leaf (compared as canonical, so `vrf default` equals an absent `vrf`); a pair it omits carries over from the source. The destination key is the codec's build of the new first value and the source's pairs, so the entry body, which already holds those key leaves, still agrees with its key. A destination that names a different value for a non-first key leaf is refused before anything changes, with an operator error naming the list and the key leaf and saying to delete the entry and create it again (AC-18). The entry points find the destination by the same rule: `cmdRename` and `cmdCopy` (`cli/model_commands_edit.go`, today the fixed `to` at `len(args)-2` and a one-token `newKey`) and `execListEntryOp` (`web/cli_terminal.go` 1193, today `srcKey` and `listName` are the last two source tokens) read the source address left to right by the part 1 rule (the CLI through `resolveListTarget`, the web terminal through the codec, because `resolveListTarget` is unexported in `cli`), then require `to`, then read every remaining token as the destination; a key value spelled `to` stays usable because an address consumes its value before `to` is looked for, and the only config-schema nodes named `to` are firewall leaves, which are never a rename source (the `to` container is in `ze-iface-cmd.yang`, a command module). The web rename handler (`web/handler_config_entry.go` 290-311, one `new-key` field validated as one segment) splits the field on whitespace, validates each token with `ValidatePathSegments`, and passes the tokens on. It no longer lowercases (round 6): line 290 lowercases `new-key` and line 305 the old key, both added with the rename handler in `a0f02e8536` to copy the add form's "Keys are always lowercase" (line 71). Nothing in the schema says a key is case-insensitive (the config schema holds no such marker), `ValidatePathSegments` accepts `A-Z`, and the CLI's `cmdRename` and `cmdCopy` do not lowercase, so the web form alone rewrites a key value: an interface `Eth0` given as the `new-key`'s second token would become `eth0` and be refused as a changed key leaf, and lowercasing the old key misses an existing entry stored as `Eth0`. Both `strings.ToLower` calls are deleted, for every list, and `new-key` reaches the editor as typed (trimmed). Under (a) the add handler builds a multi-leaf address from the posted key leaf fields as typed too; its single-key `name` lowercasing (line 72) is outside this spec and has a journal row; The session-mode record of a rename or copy (`config/change_file.go` `formatKeyMoveLine` and `parseKeyMoveLine`) holds the two stored keys, not addresses: `formatKeyMoveLine` writes each of `OldKey` and `NewKey` through `quoteIfNeeded` as ONE token, and `parseKeyMoveLine` splits the line, instead of with `strings.Fields`, with `splitQuotedTokens`, the inverse of `quoteIfNeeded`, so `fe80::b interface zbl0` survives the round trip and the `to` stays the second-to-last token (round 6). The quote and the unquote are ONE pair declared once, in the codec (`config/list_key.go`): `quoteIfNeeded` moves there from `serialize.go` unchanged (same package, so its callers do not change) and `splitQuotedTokens` is declared beside it. `SetParser.tokenizeLine` (`setparser.go` 168) is NOT that inverse: it ends a quoted token at the first matching quote and keeps backslashes, while `quoteIfNeeded` (`serialize.go` 787) escapes `"` as `\"` and `\` as `\\`, so a key holding `"` or `\` (raw today, because it holds no space) would come back changed. The brace tokenizer is not used either: its `readString` (`tokenizer.go` 208) undoes the escapes, but its `readWord` ends a word at `#`, `{`, `;` and a quote, and today's raw records carry positional `#N` keys. `splitQuotedTokens`: space and tab separate tokens; a token that STARTS with `"` or `'` runs to the matching quote, and inside it `\"`, `\'`, `\\`, `\n` and `\t` are undone exactly as `readString` undoes them (a `\` before any other byte yields that byte); an unterminated quote is an error; a quote character inside an unquoted token, a `\` in an unquoted token and `#` are literal. Old records still load: they were written raw, and a raw key never holds a space (today's keys are first words or positional `#N`), so they split exactly as `strings.Fields` split them, `\`, `#` and a mid-token quote included. The one compatibility edge is a raw key that STARTS with `"` or `'`: one that does not close at a token boundary is refused when the change file loads, naming the line and saying the session draft predates quoted records (never read silently); one that opens and closes with the same quote (a raw key `'a'`) reads as its contents. That last case needs a draft saved before this change by a rename to a key that is itself a quoted word; Ze is pre-release (`ai/rules/pre-release.md`: no user consumes `main`), so no draft crosses a release, and it is stated in Known Limitations rather than paid for with a record format version; the web editor (`walkTree`, `web/handler_config_walk.go` 193-240, and `handler_config_entry.go` 530; `collectListKeys` emits the address as path segments); gNMI (`pathToSegments` in `gnmi/path.go` builds the canonical address with the codec's build from `elem.Key`, which is already structured, taking the key order and the defaults from the schema's list node, and emits it as ONE segment instead of refusing a multi-key elem with `errMultipleKeys`; it never flattens the keys into words to be parsed again, so every later path elem is a name and never a key word, and `[peer=fe80::b][interface=zbl0]/interface` reads the `interface` leaf of the `zbl0` entry; `walkTree` in `gnmi/get.go` indexes the list map by that one segment and reads a trailing key leaf from the addressed entry as it reads any leaf, with no change of its own; `pathToSegments` takes the schema, so its callers in `get.go`, `set.go` and `subscribe.go` pass it; `set.go` hands the editor the codec's split of the address (first value, then the explicit pairs), because gNMI Set and Delete reach `Editor.SetValue` through `api/config_session.go` `SetSegments`, which walks path tokens by the parse rule of part 1, so a Set whose last elem is a key leaf is refused as that leaf named twice, the same as the CLI; `segmentsToPath` splits the address back into one key per key leaf, named from the schema, the default filled in, and `subscribe.go` passes it the schema in place of its list-name set). Every other list path walker is in the table "List path walkers" below the options table, which round 4 derived by grep and which is the authority for this part's sites. Because every key leaf pair after the first word is consumed into the address, a `set` or `delete` that names a key leaf addresses a different entry and can never rewrite a key leaf of an existing one. Today all of these take ONE path word and index the raw map key, so `set bfd single-hop-session fe80::b interface zbl1` rewrites the `zbl0` entry, a set-form replay through either `Parse` or `ParseWithMeta` merges two entries into one, and gNMI Get `single-hop-session[peer=fe80::b]` returns the `zbl0` entry. (4) Display: the eight brace-form printers that write a list key into the entry's opening line print the first value only, because the entry's body already carries the other key leaves: `serialize.go` `serializeListBlocks` (the individual-block form a BFD list takes; it writes the raw key without `StripListKeySuffix`, which is why a grep for that call missed it before round 4), `serializeListMultiBlock` and the inline-list case of `serializeNode`; `serialize_annotated.go` `serializeAnnotatedList` and `serializeAnnotatedInlineList`; `serialize_blame.go` `serializeBlameList` and `serializeBlameInlineList`; `cli/diff_tree.go` `diffList`. Left unchanged, `quoteIfNeeded` (`serialize.go`) quotes the key because it holds a space, so they would write `single-hop-session "fe80::b interface zbl1" {`, which reads back as one first value holding spaces and is refused. The six set-form emitters write on every line the address split from the entry's MAP KEY through the codec (round 5), never read from the entry body, each token through `quoteIfNeeded` on its own, and do not emit the key leaves as lines of their own; so an entry whose body holds no key leaf (one any creator made without writing them) still prints under its full address, and the printed address cannot disagree with the stored one: `serialize_set.go` `serializeSetList`, `serializeSetInlineList`, `serializeSetMetaList` and `writeInlineListLines`, and `serialize_annotated.go` `serializeAnnotatedSetList` and `serializeAnnotatedSetInlineList` (the annotated set view: their `entryPath` prefixes every leaf line and the `nop` line, so printed first value only, the `zbl1` entry's `profile fast` line would name entry `fe80::b`, AC-12's defect on that view). `FilterSetByPath` (`serialize_set.go` 1130) keeps a line whose address equals the path's address, never one whose text only starts with it: today's prefix test on `set <path> ` keeps every line of `fe80::b interface zbl1` for the path of entry `fe80::b`. It takes no schema today and cannot tell where an address ends without one, so it gains a schema parameter, and its two callers, `cli/model_commands_show.go` 181 and `web/cli_terminal.go` 851, pass the schema they already hold (round 5). The completer offers first values, then key leaf names. (5) Plugin JSON: `toMap` (`config/tree.go` 1271-1275) emits the canonical address as the entry key, the contract stated on `docs/architecture/config/yang-config-design.md`; `parseSingleHopSession` and `parseMultiHopSession` take the peer from the address's first value (their comment about "only the first positional value" is rewritten), and `parseBFDSection` refuses two entries that resolve to one session key. After round 4 the empty `vrf` cannot reach it (the codec refuses an empty key value), and the case left is two spellings of one address: `peer` is YANG `type string`, so `fe80::b` and `fe80:0::b` are two distinct config keys, which `netip.ParseAddr` in `parseSingleHopSession` reads as one peer, and `parseBFDSection` appends both sessions with no check today. NO change, verified at the producer: `cli/diff_tree.go` 359-361, `config/prune.go` 72-80, `config/mask.go` 375-378 and `cli/editor_load.go` 168-176 pair two trees by raw map key, which is now the full key, so after deleting the first entry and re-parsing, `fe80::b interface zbl1` pairs with itself; `MetaTree` list entries are keyed by the string the tree uses (`setparser_meta.go` 397), so meta follows; the duplicate loop in `addParsedListEntry` and the `#N` generator in `AddListEntry` stay as they are | Every surface that addresses or shows a list entry by its map key, and what that breaks, grepped: a consumer that reads a multi-leaf-key list's map key as its first key leaf value. Outside the surfaces of parts 3 and 4 there is exactly one, the BFD plugin (part 5). Of the 89 non-test files under `internal/` calling `GetList`, `GetListOrdered` or `GetListEntry`, the domain-specific ones (BGP, IKE, L2TP, the doctor checks, migration, `graph.go`, the `validator.go` peer walks, the `loader.go` plugin lists) name their own lists and never a BFD list, and the generic ones either treat the key as an opaque string, which is correct under a canonical key, or print it (part 4). The fixtures that write BFD config (`internal/test/fixture/plugin_fixture_0*bfd*.go`) use brace form with one first word and no non-default non-first key leaf, so they parse to today's key, except that a multi-hop entry's required `local` always appears in its address: set-form output and plugin JSON for every multi-hop entry gain `local <addr>`. The parser and the codec change behaviour only for multi-leaf-key lists, and a grep for a two-word `key` over every `*.yang` under `internal`, `pkg` and `cmd` finds exactly three: the two BFD lists and `pipe` `key "command name"` (`internal/core/ipc/yang/ze-plugin-engine.yang` 295), which sits inside `rpc declare-registration` and which the config file parser never reads. No multi-leaf-key list works today (the only paths that admit a duplicate first key are `allowsDuplicateParsedListEntries`, keyless or `content` lists, and inline lists keyed by `path-information`), so nothing that works now can regress. Files reached (round 5, from the "List path walkers" table): config `list_key.go` (new codec), `yang_schema.go`, `parser_list.go`, `schema.go`, `setparser.go`, `setparser_meta.go`, `serialize.go`, `serialize_annotated.go`, `serialize_blame.go`, `serialize_set.go`, `tree.go` (doc comments), `change_file.go` (round 5); CLI `editor_walk.go`, `editor_commands.go`, `editor_draft.go`, `editor.go`, `editor_annotated.go`, `completer.go`, `diff_tree.go` (display line only), `model_commands_edit.go` and `model_commands_show.go` (round 5); web `handler_config_walk.go`, `handler_config_entry.go`, `fragment.go`, `cli_terminal.go` and `editor.go` (round 5); gNMI `path.go`, `get.go`, `set.go`, `subscribe.go`; BFD `config.go`. Every other file holding a list path walker is named in that table with the reason it does not change. Against round 2's resolver-beside-`#N` shape, the canonical key removes the full-key comparator in the duplicate check, every cross-tree pairing change (diff, prune, mask, editor load), the meta keying question, the order-dependent `#N` for a second link, the BFD `#N` strip and the "fewer key leaves than it takes" refusal; it adds the brace-form display change at eight printers, which the `#N` shape got for free from `StripListKeySuffix`. Cost: the largest phase of the spec, a config-component change of roughly the size of the BFD work, 31 files (12 config including the new codec, 9 CLI, 5 web, 4 gNMI, 1 BFD), with its own CLI, web, gNMI, round-trip, diff and rename tests (AC-11 to AC-18). It turns this BFD spec into a config-component spec as well | Yes |
| (b) runtime client only | No config code; the `.ci`'s two-link leg drives OSPFv3 on `zbl0` and `zbl1` | Drops user story #2 and AC-2's plugin-config leg. With AC-5 refusing a link-local peer without `interface`, one address on two links is then unconfigurable through `bfd` at all, and the only way to get two such sessions is a client protocol | No |
| (c) zone in the list key | `fe80::b%zbl0` and `fe80::b%zbl1` are two distinct first words, so the parser accepts both; `parseSingleHopSession` moves the zone into `interface` and refuses a zone that disagrees with it | `internal/component/bfd/config.go` only. Reverses Q-1's recommended answer and AC-7, and gives the link two spellings (zone and `interface`) with a mismatch rule, the shape Key Design Decisions rejected because `reconcileZone` holds the link once, in `iface` | Yes, but only in the zone spelling |

#### List path walkers (Q-3 (a), round 4)

Derived set: every non-test function under `internal/component/config`, `cli`,
`web`, `gnmi`, `api`, `mcp`, `aihelp`, `command`, `plugin/cli` and
`internal/plugins/completion` that names `ListNode`, calls `GetList`,
`GetListOrdered` or `GetListEntry`, or steps over the path word after a list
(`path[i+1]`, `i++` past a key, `tokens[0]` as a key), plus every call that
prints a list key (`quoteIfNeeded(key)` and its variants). Control: the grep
returns `buildContextHeading`, `walkSchemaNode` and `walkSchema`, the three
sites round 3 missed. `api`, `mcp`, `aihelp`, `command` and
`internal/plugins/completion` hold no list path walker: `api/config_session.go`
`SetSegments` passes segments to `Editor.SetValue` without walking them. A
function marked "follows" has no list code of its own and is right once the
function it calls is.

| File | Changes (steps over a key word, builds an address, or prints a key) | No change, and why |
|------|---------------------------------------------------------------------|--------------------|
| `config/parser_list.go` | `parseList`, `addParsedListEntry` (part 2) | `parseListFieldBlock`, `parseListInlineBlock`, `parseListInlineEntry`, `setInlineLastChild`, `validateInlineListChildValue`, `allowsDuplicateParsedListEntries`: inline lists and the duplicate allow-list, keys returned as written |
| `config/yang_schema.go` | `yangToList` (key leaf names and leaves, part 1) | `walkNodeRelated`, `HasStructuralChildren`, `hasAllowUnknownExtension`, `numericRangesFromType`: schema only |
| `config/schema.go` | `LookupTokenPath` (skips one word after a list unless it is a child name, so `... fe80::b interface zbl1 profile` lands on leaf `interface`; it consumes the address by the part 1 rule); `ValidateListKey` (validates the first value; each pair is validated against its own leaf by the codec) | `Lookup`, `List`, `mergeNode`, the `Children`/`Get`/`Has`/`Kind` methods, `extendGroupCapability`, `extendGroupPeerCapability`, `extendPeerListCapability`: schema only, no key word |
| `config/parser.go` | none | `parseNode` and its `LookupTokenPath` context lookups: follows `LookupTokenPath` |
| `config/setparser.go` | `walkAndSetListEntry`, `deleteFromList`, `walkAndMarkInactive`, `markNopInactive` (each takes `tokens[0]` as the key; part 3) | `walkAndSet`, `walkAndDelete`: their `ListNode` arms look up the schema and hand the list to the two functions beside them, so they follow; `resolveSchemaNode`, `validateValueOrArrayItems`: schema only |
| `config/setparser_meta.go` | `walkAndSetWithMeta`, `walkAndRecordDeleteMeta` (part 3) | `extractMeta`: meta prefix tokens, no list |
| `config/setparser_inline.go` | none | `consumeOneField`: inline lists, keys as written |
| `config/serialize.go` | `serializeListBlocks`, `serializeListMultiBlock`, `serializeNode` inline-list case (part 4) | `Serialize`, `serializeListEntry`, `inlineLeafListEntry`, `allChildrenAreLeaves`: dispatch to the printers; the `FlexNode` case prints flex entries, not a `ListNode` |
| `config/serialize_annotated.go` | `serializeAnnotatedList`, `serializeAnnotatedInlineList` (brace); `serializeAnnotatedSetList`, `serializeAnnotatedSetInlineList` (set form; part 4) | `serializeAnnotatedTreeNode`, `serializeAnnotatedSetChild`: dispatch; `serializeAnnotatedFlex`, `serializeAnnotatedSetFlex`: flex nodes |
| `config/serialize_blame.go` | `serializeBlameList`, `serializeBlameInlineList` (part 4) | `serializeBlameTreeNode`: dispatch; `serializeBlameFlex`: flex |
| `config/serialize_set.go` | `serializeSetList`, `serializeSetInlineList`, `serializeSetMetaList`, `writeInlineListLines` (address split from the map key); `FilterSetByPath` (whole-address match, gains a schema parameter; part 4) | `serializeSetChild`, `serializeSetMetaChild`: dispatch; `serializeSetFlex`, `writeFlexLines`: flex |
| `config/flatten.go` | none | `serializeFlattenedContainer`, `serializeAnnotatedFlattenedContainer`: follow `serializeListBlocks` and `serializeAnnotatedList`; `canFlattenContainer`, `usesMultiBlockForm`: schema only |
| `config/tree.go` | `AddListEntry`, `toMap` doc comments only; `RenameListEntry`, `CopyListEntry` doc comments state that for a multi-leaf-key list the caller passes keys that differ only in the first value (round 5) | no logic change: the tree holds no schema, so the first-value-only rule lives in `Editor.RenameListEntry` and `Editor.CopyListEntry` |
| `config/change_file.go` | `materializeMetaPaths` (creates an entry under the meta key; writes the pairs the codec splits from that key into it); `formatKeyMoveLine` and `parseKeyMoveLine` (the rename and copy record: today the keys are written raw and read back with `strings.Fields` and a `to` fixed at the second-to-last token, so a key holding spaces is misread; part 3, round 5; the split is the codec's `splitQuotedTokens`, the inverse of `quoteIfNeeded`, and old raw records still load, round 6) | `parseToggleLine` and the other record parsers: their paths are re-walked by the walks above, keys are not split out |
| `config/mask.go`, `config/prune.go` | none | `maskWalk`, `rejectMaskedSecretWalk`, `pruneNode`, `pruneActiveNode`, `pruneUnchangedNode`: pair or recurse by map key, which is the full key; `secretAtPath` and the line redactor follow `LookupTokenPath` |
| `config/required.go`, `schema_defaults.go`, `listener.go`, `password_hash.go`, `backend_gate.go`, `related.go` | none | `walkRequired`, `checkListRequired`, `hasRequiredInSubtree`, `ApplyDefaults`, `applyChildDefault`, `walkListenerNodes`, `buildListenerService`, `checkBcryptLeavesWalk`, `descend`, `walkBackendNode`, `walkBackendListMap`, `walkBackendListEntry`, `walkBackendInlineList`, `backendAnnotation`, `RelatedEmpty.String`: walk the schema or range over entries with the key opaque; a key in an error path is printed as stored |
| `cli/editor_walk.go` | `walkOrCreateIn`, `walkPath`, `walkOrCreateMeta`, `walkMetaReadOnly` | none |
| `cli/editor.go` | `walkSchemaNode` (returns step 2 over one key word); `ResolveLeafListValue` (skips one key); `autoSelectListEntry` (expands the path with the single entry's key, so it appends the codec's split, not the map key as one word) | `WalkPath`, `walkPathWithSchemaFrom`: follow `walkSchemaNode`; `inactiveContentAtPath`: follows `prunedContentAtPath` and `serializeMasked`, which follow `walkPathWithSchemaFrom` |
| `cli/editor_commands.go` | `walkOrCreate`, `walkSchema`, `isListKeyLeafPath` (each skips one key word); `DeleteByPath` (reads the last two words as list and key, so a full-key `delete` must parse the address); `resolveListTarget` (rename, both sides an address; it also reports where the source address ends, so the entry point can require `to` there); `Editor.RenameListEntry`, `Editor.CopyListEntry` (take the destination tokens, apply the first-value-only rule and its refusal before the session branch, then pass the built key on; part 3, round 5) | `ensureListEntry`: takes the key the caller built; `resolveListNode`: schema only |
| `cli/model_commands_edit.go` | `cmdSet` (runs the address parse over every token before splitting off the value, part 3); `cmdRename`, `cmdCopy` (source address read through `resolveListTarget`, then `to`, then the destination tokens; part 3, round 5) | the other `cmd*` handlers: hand their path to the walks above |
| `cli/editor_annotated.go` | `walkMetaPath` (skips one key word) | none |
| `cli/editor_draft.go` | `rename` paths (both sides an address; part 3) | `renameTreeListEntry`, `copyAlreadyApplied`, `renameAlreadyApplied`: take keys from `resolveListTarget`, opaque |
| `cli/completer.go` | `completeEditPath`, `getEntry`, `navigateTreeToPath` (each steps over one key word); `listKeyCompletions` offers first values, then key leaf names | none |
| `cli/diff_tree.go` | `diffList` opening line (part 4) | `diffList` pairing, `diffWalkNode`, `emitEntryChildrenMarked`, `serializeNodeText`: pair by map key or print leaves |
| `cli/model_commands_show.go` | the `FilterSetByPath` call at 181 passes `m.editor.schema` (round 5) | the rest: follows `DisplayContentAtPath` and the walks above |
| `cli/editor_activation.go`, `editor_load.go`, `editor_commit.go`, `validator.go` | none | `applyNodeActivation`: follows `applyPathActivation` and the walks above; `editor_load.go` pairing by map key and kind checks; `editor_commit.go` rename conflict check: opaque keys from `resolveListTarget`; `validatePeer`, `peerListNode`: the BGP peer list |
| `web/handler_config_walk.go` | `walkTree`, `walkSchema` (lands on leaf `interface`, then fails on `zbl1`), `isListEntryPath`, `validateUniqueOnSet` (entry key is `path[i+1]`), `collectListKeys` (emits the address as path segments) | `walkConfigPath`, `buildConfigViewData`: follow `walkTree`; `findListNode`, `checkUniqueConstraint`: schema only or opaque keys |
| `web/fragment.go` | `buildContextHeading` (one `ContextEntry` per list from `path[i+1]`; it consumes the address); `buildListColumn`, `buildListTable` (the key column header is the first key leaf's name, not the whole `KeyName`; each row's URL, `HxPath` and entry path are built from the address segments, not the map key as one segment) | `buildFragmentData`: its list-entry branch decides by `isListEntryPath`, so it follows; `buildColumnAt`, `buildSidebarHierarchy`, `keylessEntrySummary`, `collectUniqueFields`, `collectRequiredFields`, `collectSuggestFields`, `nodeDescription`, `resolveLeafDescription`, `resolveLeafType`, `resolveListField`, `resolveListLeaf`: schema only, counts, or keyless lists |
| `web/handler_config_entry.go` | `resolveNamedListEntryPath`, `resolveParentDefaults` (each steps over one key word); `HandleConfigAddWithAuthorizer` (takes the new entry's key from the last path word, so for a multi-leaf-key list it builds the address with the codec from the posted key leaf fields); the rename handler at 290-311 (the `new-key` field split into tokens, each validated, passed to the editor's first-value-only rule; part 3, round 5) | `HandleConfigAddForm`, `inheritedFieldValue`: schema only |
| `web/cli_terminal.go` | `execListEntryOp` (today `srcKey` and `listName` are the last two source tokens and `dstKey` one token; it reads the source address by the part 1 rule through the codec and the schema it holds, then `to`, then the destination tokens; `resolveListTarget` is unexported in `cli`); the `FilterSetByPath` call at 851 passes the schema it holds (round 5) | the meta walk call at 715: follows `walkAndSetWithMeta` |
| `web/editor.go` | `EditorManager.RenameListEntry`, `EditorManager.CopyListEntry`: pass the destination tokens through to the editor (signature only, round 5) | the rest: session plumbing, no list key |
| `web/cli.go`, `workbench_enrich.go`, `handler_tools.go`, `page_bgp_policy.go`, `page_l2tp.go`, and the `page_*`/`workbench_dashboard.go` `GetList` callers | none | `adjustListContext`: follows `isListEntryPath`; `buildWorkbenchListTable`: follows `buildListTable`; `enrichWorkbenchTable`, `defaultWorkbenchColumns`, `lookupListNode`, `lookupRelatedTool`, `policyFilterListNames`, `schemaDescription`: schema only; the page files name their own BGP, L2TP and system lists |
| `gnmi/path.go`, `get.go`, `set.go`, `subscribe.go` | `pathToSegments`, `segmentsToPath`; the `pathToSegments` and `segmentsToPath` calls in `get.go`, `set.go`, `subscribe.go` pass the schema, and `set.go` splits the address for the editor (part 3) | `walkTree` (`get.go`): indexes by the one address segment, no change of its own |
| `plugin/cli/test_cmd.go` | none | `showSchemaNode`: schema only |

Recommendation for Q-3 (design round 1, 2026-10-14): **(a).** The defect sits
in the config file parser: the YANG declares `key "peer vrf interface"` and the
parser honours only its first leaf, so the fix belongs there and not in a BFD
workaround (`ai/rules/simplicity.md`, fix at the root). It keeps user story #2
and the plugin-config leg of AC-2, it keeps one spelling of the link
(`interface`), so Q-1 and AC-7 stand, and it repairs the same refusal in
`multi-hop-session`, whose YANG key is `peer local vrf`: two sessions to one
peer from two local addresses, which RFC 5883 Section 4.1 and `firstPacketKey`
already support, are refused by the parser today. (b) removes a user story; (c)
is smaller but buys it with the second spelling this spec exists to remove. (a)
reaches the config component, so the scope widening is the owner's decision.

Restated at design round 2, 2026-10-14: **(a) still holds, and its cost is
larger than round 1 said.** What changed: round 1 priced (a) as the parser plus
the BFD parsers. Read at the producers, the CLI editor, the web editor and the
set-form serializer and replay all address an entry by one key word, so a parser
fix alone lets `set` rewrite the first session instead of adding the second, and
lets a set-form save and replay merge two sessions into one with no error. That
is a silently wrong value (`ai/rules/principles.md`), so it cannot ride as a
Known Limitation of (a): (a) now includes full-key addressing on every config
surface (parts 2 to 4 of the table), roughly doubling the spec. The reasons for
(a) are unchanged: the defect is the config component misreading a YANG key, it
also blocks `multi-hop-session` (journal row 2026-10-14), no list with a
multi-leaf key works today so nothing regresses, and (c) keeps the second
spelling. What the owner now weighs: if this spec must stay BFD-sized, (c) is
the correct smaller answer (one file, no config surface), at the price of Q-1
and AC-7 reversing and `multi-hop-session` staying refused.

Restated at design round 3, 2026-10-14: **(a) still holds, redesigned around
one root change, and its cost is about what round 2 said, now complete.** What
changed: round 3 found three surfaces round 2 missed (the `SetParser` meta walk
behind drafts and change files, gNMI Get and paths, and the cross-tree pairing
in diff, prune and mask), all wrong for one reason: the tree stores the second
entry under its first key word plus a positional `#N`, so every surface that
matches or addresses by map key is wrong in its own way. Round 3 replaces that
storage with a canonical full-key map key (the first key value, then
`name value` for each other key leaf not at its YANG default). Cross-tree
pairing and meta then need no change, gNMI Get maps a path onto the map key
through the same codec every other addressing surface calls, and the `#N`
scheme is deleted for multi-leaf keys rather than kept beside it. The price is
a brace-form display change at the printers that write a list key on an
opening line, which must print the first value only (round 3 counted nine
lines; round 4 moved two of them, the annotated set view, to the set-form rule
and found `serializeListBlocks`, so eight brace printers and six set-form
emitters). A plain BFD entry keeps today's key (`fe80::b`), so existing
single-entry configs and fixtures parse to the same key; multi-hop entries gain
`local <addr>` in set form and plugin JSON because `local` is required. The
reasons for (a) over (c) are unchanged: the defect is the config component
misreading a YANG key, it also blocks `multi-hop-session` (journal row
2026-10-14), nothing that works today regresses, and (c) keeps the second
spelling of the link. One behaviour changes from round 2's AC-11: a path that
names fewer key leaves is no longer refused as ambiguous, because under a
canonical key it is the complete address of the entry whose other key leaves
are absent; the file parser already accepts that entry beside the others
(AC-14), so the editor refusing it would give one fact two rules. For a
link-local peer, AC-5 then refuses the commit and names `interface`. What the
owner weighs: (a) is the largest phase of the spec, about 20 files across
config, CLI, web, gNMI and BFD; (c) is one BFD file, at the price of Q-1 and
AC-7 reversing and `multi-hop-session` staying refused.

Restated at design round 4, 2026-10-14: **(a) still holds, and its file count
is now derived rather than estimated.** Every list path walker in the config,
CLI, web and gNMI components was grepped and classified ("List path walkers"),
which adds `config/schema.go` (`LookupTokenPath`, `ValidateListKey`),
`cli/editor.go`, `cli/editor_annotated.go`, `web/fragment.go` and gNMI
`set.go` and `subscribe.go` to the files reached, and `FilterSetByPath` to the
set-form changes. An empty key value is refused at parse, so the plugin's
duplicate-session refusal is left with its one reachable case (two spellings
of one address). gNMI builds the address from the structured `elem.Key` and
never re-parses flat words, and token paths follow one left-to-right rule.
Cost: 26 files (11 config including the new codec, 7 CLI, 3 web, 4 gNMI, 1
BFD), plus the six new config-surface test files and the `.et`, against about
20 in round 3. (c) stays the BFD-sized alternative, one file.

Restated at design round 5, 2026-10-14: **(a) still holds; three surfaces
were added.** Rename and copy change only the first key value, and a
destination that changes another key leaf is refused, naming the leaf and
telling the operator to delete and re-create the entry (AC-18). The guard is in
the editor's rename and copy funnel, and the CLI, web form and web terminal
entry points read both sides as addresses. The session-mode record quotes each
key as one token. The six set-form emitters take the address from the map key,
never the body, so an entry a creator left without key leaves
(`materializeMetaPaths`) still prints correctly; that creator writes them
anyway. `FilterSetByPath` takes the schema. AC-15b's key-leaf `show` case is
dropped as outside the goal. Cost: 31 files (12 config, 9 CLI, 5 web, 4 gNMI,
1 BFD), against 26 in round 4.

Restated at design round 6, 2026-10-14: **(a) still holds; no production file
was added.** The key-move record splits with `splitQuotedTokens`, the inverse
of `quoteIfNeeded`, declared beside it in the codec, and old raw records still
load. The web rename handler stops lowercasing. Three existing test files join
the work: `change_file_test.go` (the record round trip), `serialize_set_test.go`
and `setparser_nop_test.go` (the `FilterSetByPath` callers). Cost: 31 files
(12 config, 9 CLI, 5 web, 4 gNMI, 1 BFD), unchanged, plus three existing test
files modified beside the six new ones.

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
is still unrun. BGP over a link-local peer is out of scope, and the reason is
where the refusal lives, not a BGP refusal. The BGP peer check in
`reactor_peers.go` refuses only a link-local `local-address`; it does not
refuse a link-local peer address. `bfdRequestFor` (`peer_bfd.go`) builds BGP's
BFD request with `Peer` from the peer address, `Local` from the local address,
and `Interface` only from the optional `bfd interface` leaf. That request
reaches the engine through `EnsureSession` and never passes
`parseSingleHopSession`, so P-3's refusal does not reach it: a BGP link-local
peer with no `bfd interface` asks for a session `Canonical` cannot complete on
a multi-link box. Whether BGP should require `bfd interface` for such a peer is
a BGP configuration decision, outside this BFD-plugin spec (unverified here:
whether BGP establishes a session to a link-local peer address at all). OSPF's
inline BFD timers are owned by `plan/spec-yang-feature-opt-ins-owned-by-feature.md`.

Optional, not adopted (design review round 5, NIT): `single-hop-session` and
`multi-hop-session` `peer` are YANG `type string` (`ze-bfd-conf.yang` 202),
while `zt:ip-address` exists (`ze-types.yang` 202). With that type, the codec
could canonicalize an ip-address key value through `netip.Addr.String()`, so
`fe80:0::b` would collapse to `fe80::b` at the config parser's duplicate check
and a malformed address would be refused at parse. No value is wrong today
without it: AC-17's plugin refusal catches two spellings of one session key.

Under Q-3 (a), session drafts saved before quoted key-move records (design
review round 6): a rename or copy record written raw whose key starts and ends
with the same quote character (a raw key `'a'`) loads as `a`. Every other raw
record loads unchanged, and one whose leading quote does not close at a token
boundary is refused naming the line. The case needs a draft saved before this
change by a rename to a key that is itself a quoted word, and Ze is pre-release
(`ai/rules/pre-release.md`), so no draft crosses a release; no record format
version is added for it.

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
- [ ] An `rfc/short/` summary exists for every RFC referenced (N-A for RFC 4007: cited in comments, not enrolled, per the owner's 2026-10-08 RFC scope freeze; RFC 5880 and RFC 5881 have summaries)
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

## Design Review Log

| Date | Round | Outcome |
|------|-------|---------|
| 2026-10-14 | design review round 1 (0 BLOCKER, 3 ISSUE, 6 NIT; `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-bfd-link-local-design-r1.md`) | Applied by a separate editing agent, Edit tool only, each finding re-read at the producer. #1 `TestTwoLinkLocalSessionsStayApart` retagged `RFC5881-3-2 positive` with break `reconcileZone` → identity: `handleInbound` hands one packet to at most one `byKey` entry and the two sessions are two machines under two `api.Key` values, so no break in `firstPacketKey`, `without` or `keyRelaxations` reaches the negative assertion. #2 the interop link-local assignment derived from the placeholders (`@ZE_LINK_LOCAL@`, new `@FRR_LINK_LOCAL@` at FRR's host 3), the `rfc2545Scenario` name comparison deleted, `prepare.go` and `check_rfc.go` in Files to Modify, `TestLinkLocalAssignmentFollowsThePlaceholders`, `bgp-rfc2545-linklocal-nexthop-frr` must stay green; A-5 rewritten (FRR image entrypoint unread). #3 A-3 REFUTED at `parseList`/`addParsedListEntry` (the second entry is refused as "duplicate list key", not merged); Q-3 re-asked with three options and their blast radius, recommendation (a) the parser honours the whole YANG key; AC-2, Phase 1, Failure Routing and Blast Radius follow. #4 no question "Z" exists in the spec, nothing to remove. #5 tag claim says "a zone-qualified source". #6 Known Limitations states the real BGP reason (`bfdRequestFor` bypasses `parseSingleHopSession`; `reactor_peers.go` refuses only a link-local `local-address`). #7 AC-6a for a non-default VRF. #8 fixture lifecycle row. #9 RFC 4007 checkbox marked N-A with its reason. Q-1 to Q-4 stay OPEN. Round 2 owed, narrow: the Q-3 options table and recommendation, A-3, A-5, the Scenario contract and its unit test, AC-2, AC-6a, the retagged row |
| 2026-10-14 | design review round 2 (0 BLOCKER, 1 ISSUE, 3 NIT; `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-bfd-link-local-design-r2.md`) | Applied by a separate editing agent, Edit tool only, each finding re-read at the producer. #1 (main-thread decision: a set-form round trip that merges two sessions is a silently wrong value, so not a Known Limitation) Q-3 (a) now includes full-key addressing in the CLI editor (`walkOrCreateIn`), the web editor (`walkTree`), `SetParser` and the set-form serializer, plus the BFD `#N` strip (`toMap` emits raw keys); the "Unverified" items answered; Files to Modify and Create, Phase 1, Blast Radius, AC-11 to AC-14 and four tests follow; cost and recommendation restated (still (a), with (c) named as the BFD-sized alternative). #2 an absent key leaf compares as its YANG default, else empty (AC-14). #3 exactly three multi-word keys, `pipe` inside `rpc declare-registration` never read by the config parser; no multi-leaf-key list works today. #4 journal row in `plan/journal/key-omits-a-fact-the-builder-uses.md`. Q-1 to Q-4 stay OPEN. Round 3 owed, narrow: the Q-3 (a) row and restated recommendation, AC-11 to AC-14, their tests, Files to Modify and Create, Phase 1 |
| 2026-10-14 | design review round 3 (0 BLOCKER, 3 ISSUE; `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-bfd-link-local-design-r3.md`) | Applied by a separate editing agent, Edit tool only, each finding re-read at the producer. Main-thread direction: the three findings (the `SetParser` meta walk, gNMI Get and paths, cross-tree pairing by positional `#N`) share one root, the tree storing a multi-leaf-key entry under its first key word plus an order-dependent `#N`. Q-3 (a) redesigned around one root change: the entry is stored under a canonical full-key address (first key value, then `name value` for each other key leaf not at its YANG default, space-separated, whitespace in a key value refused). Verified at the producers: diff, prune, mask, editor load and meta then need no change; `setparser_meta.go`, gNMI `path.go`/`get.go` and every addressing site parse the address through the one codec; the new cost is the brace-form display at nine printer lines (corrected in round 4: eight brace printers and six set-form emitters); the only consumer that reads the key as its first leaf is the BFD plugin. `#N` is deleted for telling multi-leaf entries apart and kept only for what it serves on every list. Divergence from the direction, stated: absent key leaves at their default are OMITTED from the canonical key rather than written out, so a plain entry keeps today's key and the key text is a valid set-form address. AC-11 changed (a partial path addresses the entry whose other key leaves are absent, no ambiguity refusal); AC-12 replays through `Parse` and `ParseWithMeta`; AC-14 gains the whitespace refusal; AC-15 gNMI Get of each of two entries, AC-16 diff/prune/mask after deleting the first entry, AC-17 BFD refuses two entries resolving to one session key (empty `vrf`); tests, Files, Phase 1, Blast Radius, Required Reading, cost and recommendation restated (still (a); (c) the BFD-sized alternative). Q-1 to Q-4 stay OPEN. Round 4 owed, narrow: the Q-3 (a) row and round-3 restatement, AC-11 to AC-17 and their tests, Files to Modify and Create, Phase 1 |
| 2026-10-14 | design review round 4 (0 BLOCKER, 4 ISSUE, 1 NIT; `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-bfd-link-local-design-r4.md`) | Applied by a separate editing agent, Edit tool only, each finding re-read at the producer. #1 `serializeAnnotatedSetList` and `serializeAnnotatedSetInlineList` move to the set-form rule (six set-form emitters, each token quoted on its own). Divergence from the direction, stated: the brace printers are eight, not seven, because `serializeListBlocks` (the individual-block form a BFD list takes) writes the raw key without `StripListKeySuffix` and was missed by every earlier round. #2 an empty key value is refused at parse, naming the list and the leaf; "the text is itself a valid set-form address" dropped; AC-14 gains the empty value; AC-17 now holds that refusal plus the plugin's refusal of two spellings of one address (`fe80::b`, `fe80:0::b`), the one case left that reaches `parseBFDSection`. #3 gNMI builds the address from `elem.Key` as one segment and never re-parses flat words; the token rule (pairs consumed while a key-leaf name is followed by a value, else the address ends) stated with its cases, incl. `... fe80::b interface`; AC-15a (key-leaf Get, key-leaf Set refused) and AC-15b (CLI `show` of a key leaf and of one entry). #4 table "List path walkers": every walker under config, CLI, web, gNMI, api, mcp, aihelp, command, plugin/cli and the completion plugin, each marked changes or not with the reason; it adds `config/schema.go`, `cli/editor.go`, `cli/editor_annotated.go`, `web/fragment.go`, gNMI `set.go` and `subscribe.go`, and `FilterSetByPath` (its `set <path> ` prefix test keeps the lines of `fe80::b interface zbl1` for entry `fe80::b`); AC-13 asserts the context heading and list table. #5 the unchanged printers would write `"fe80::b interface zbl1" {` (`quoteIfNeeded`). Cost restated: 26 files. Q-1 to Q-4 stay OPEN. Round 5 owed, narrow: the Q-3 (a) row parts 1, 3, 4 and 5, the "List path walkers" table, AC-13 to AC-17 and their tests, Files to Modify, the round-4 restatement |
| 2026-10-14 | design review round 5 (0 BLOCKER, 3 ISSUE, 1 NIT; `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-bfd-link-local-design-r5.md`) | Applied by a separate editing agent, Edit tool only, each finding re-read at the producer. #1 (main-thread decision, simplest correct) a rename or copy of a multi-leaf-key entry changes only the first key value; a destination naming a different non-first key leaf value is refused, naming the list and the leaf and saying to delete and re-create (AC-18, two tests). The guard is in `Editor.RenameListEntry`/`CopyListEntry` (`config/tree.go` holds no schema, doc comments only); `cmdRename`, `cmdCopy`, `cmdSet` (`model_commands_edit.go`), the web rename handler, `execListEntryOp` and `web/editor.go` are in the walker table and Files. Found beyond the finding and added: the session-mode rename and copy record (`change_file.go` `formatKeyMoveLine`/`parseKeyMoveLine`) writes keys raw and reads them with `strings.Fields`, so each key is now one quoted token, read by a quote-aware split. #2 the six set-form emitters split the MAP KEY through the codec, never read the body; `materializeMetaPaths` is in part 3 and the table and writes the meta key's pairs; AC-12 gains the materialized pending-delete case. #3 AC-15b's key-leaf `show` case dropped as outside the goal; `FilterSetByPath` gains a schema parameter and its callers `cli/model_commands_show.go` and `web/cli_terminal.go` move to changes. NIT 4 recorded as optional in Known Limitations, not adopted. Cost restated: 31 files (12 config, 9 CLI, 5 web, 4 gNMI, 1 BFD). Q-1 to Q-4 stay OPEN. Round 6 owed, narrow: the Q-3 (a) row part 3 (rename, copy, `cmdSet`, `materializeMetaPaths`, the key-move record) and part 4 (emitters, `FilterSetByPath`), the walker table rows touched, AC-12, AC-15b, AC-18 and their tests, Files to Modify, the round-5 restatement |
| 2026-10-14 | design review round 6 (0 BLOCKER, 1 ISSUE, 2 NIT; `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-bfd-link-local-design-r6.md`) | round 6 applied; design complete for (a) pending the owner's Q-3 answer; a narrow round 7 over these edits runs only if the owner picks (a). (Answered (a) on 2026-10-14: round 7 is owed, next row.) Applied by a separate editing agent, Edit tool only, each finding re-read at the producer. #1 `parseKeyMoveLine` splits with `splitQuotedTokens`, declared in the codec beside `quoteIfNeeded` (moved from `serialize.go`) as its inverse, with `readString`'s escape semantics; `tokenizeLine` keeps backslashes and the brace tokenizer ends words at `#`, so neither is used; old raw records load unchanged, and the raw quoted-word key edge is in Known Limitations; `TestKeyMoveRecordRoundTripsQuotedKeys` added. NIT 2 `serialize_set_test.go` and `setparser_nop_test.go` in Files to Modify. NIT 3 the lowercasing was copied from the add form in `a0f02e8536`, the schema holds no case-insensitive marker and the CLI keeps case, so the rename handler drops both `strings.ToLower` calls; `Eth0` cases added to the web rename test; the add form's same lowercasing has a journal row (`plan/journal/validator-mutates-forwarded-data.md`). Cost unchanged: 31 files plus three existing test files |
| 2026-10-14 | owner answers (Thomas: "LDP, BFD, GR, Prefix-SID, accept") | Q-1 to Q-4 answered, every recommendation accepted: Q-1 refuse a zone (AC-7); Q-2 refuse a link-local peer with no `interface` (AC-5); Q-3 (a), the config-surface scope widening approved; Q-4 stays the journal row in `plan/journal/key-omits-a-fact-the-builder-uses.md` (2026-10-13, present). Unchosen text removed: Phase 1's (b) branch; the Q-3 unanswered failure route; (b) and (c) marked rejected. No redesign follows from the answers: the (a) design is complete through round 6. Owed before implementation, design round 7, narrow, by an independent reviewer, over the round-6 edits as round 6 said: the Q-3 (a) row part 3 (rename, copy, `cmdSet`, `materializeMetaPaths`, the key-move record and `splitQuotedTokens`) and part 4 (emitters, `FilterSetByPath`), the walker table rows touched, AC-12, AC-15b, AC-18, `TestKeyMoveRecordRoundTripsQuotedKeys`, the web rename lowercasing removal, Files to Modify, the round-6 restatement. Phase 1 starts only when it is 0 BLOCKER, 0 ISSUE |

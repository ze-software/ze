# ZeBGP vs ExaBGP Behavioral Differences

This file lists the differences that remain between Ze and ExaBGP at
f9bfb853b, the release the bridge and the migration follow (`ai/META-DECISIONS.md`). Each is a design decision, not a bug. A
difference that has been resolved is removed from this page.

**Impact on testing:** When `.ci` files from ExaBGP tests don't match ZeBGP output
due to these differences, update the `.ci` files to match ZeBGP's behavior.

---

## A refused line names its reason

**ExaBGP behavior:**
- A command the helper's dispatcher does not know (`UnknownCommand`), and one
  whose selector matches no peer (`NoMatchingPeers`), are answered with the
  `error` marker alone (`API.process`, `answer_error_sync(service)`).
- Every other refusal writes its reason first: `{"error": "<why>"}` to an API 6
  helper, `error: <why>` to an API 4 or undecided one, then `error`
  (`Processes._error_detail`, `answer_error_sync`). A route the parser cannot
  read is one of these (`announce_route`, "Could not parse route").
Upstream (exa-networks/exabgp at f9bfb853b): `src/exabgp/reactor/api/__init__.py`,
`src/exabgp/reactor/api/processes.py`.

**ZeBGP behavior:**
- Every refused line is answered the second way, the two marker-only cases
  included: the reason in the form the script's API version reads, then
  `error`. The form follows the version the script was held to when it wrote
  the line. With ack disabled neither line is written.
- The reason is Ze's own words, not ExaBGP's sentence for the same refusal.
<!-- source: internal/exabgp/bridge/bridge_ack.go -- AckMode.WriteError -->
<!-- source: internal/exabgp/bridge/bridge_batch.go -- AnswerBatch, BatchLine.Version -->

**RFC compliance:**
- N/A - This is API syntax, not BGP protocol

**Impact:**
- A script reads the same two lines it reads from ExaBGP 6.0 for any other
  refusal, and every 5.x and 6.0 helper already reads a reason line before
  `error`. An operator reading the script's input learns why the line was
  refused (the user-facing errors rule).

---

## Refusals and keep-first: where Ze and ExaBGP 6.0 still differ

The bridge answers ExaBGP 6.0's refusals and keep-first rules as ExaBGP does
(`docs/architecture/exabgp-bridge.md`, "What the bridge refuses, and what it
keeps"). Five differences remain.

- **OPERATIONAL is not sent.** ExaBGP sends `announce operational <kind> ...`
  for a known kind (draft-ietf-idr-operational-message). Ze does not originate
  OPERATIONAL, so the bridge refuses the line and says so: Ze does not support
  the OPERATIONAL message (`ai/META-DECISIONS.md`). An unknown kind is
  refused as ExaBGP refuses it.
- **More flow routes match no packet.** ExaBGP 6.0 drops a flow route that
  matches an ICMP type or code and a port (`Flow.unmatchable`,
  `src/exabgp/bgp/message/update/nlri/flow.py`) and sends every other one. Ze
  applies the whole rule RFC 8955 Sections 4.2.2.3 to 4.2.2.12 and RFC 8956
  Sections 3.3 to 3.7 state, so it also refuses ICMP beside TCP flags, a
  protocol match no port, ICMP or TCP-flags match beside it can meet (`protocol
  udp; tcp-flags syn;`), a non-first fragment match beside any of them
  (`fragment is-fragment; destination-port =80;`), a port match no destination
  or source port beside it can meet (`port =80; destination-port =443;
  source-port =443;`), and a match whose own values admit nothing (`dscp >63`,
  `destination-port >100&<50`, an IPv6 `next-header` naming only extension
  headers, a TCP flags or fragment bitmask that contradicts itself). The RFC outranks ExaBGP's behavior. The bridge refuses ICMP beside TCP
  flags itself, answering `error` as for ICMP beside a port; a contradiction in
  the values reaches Ze as `update text`, which Ze refuses
  (`docs/architecture/wire/nlri-flowspec.md`, "Combinations that match no
  packet").
- **The warning goes to Ze's log.** A repeated attribute that is not a list
  keeps its first value, and a `split` shorter than the prefix sends the prefix
  whole. ExaBGP logs each; Ze logs `exabgp-bridge route line` with the dropped
  value. A repeated `community`, `extended-community` or `large-community` is
  merged into the first and logs nothing, and a repeated `label`, `rd` or
  `path-information` replaces the first, as on ExaBGP 6.0. So does a repeated
  `next-hop` on a mup or mcast-vpn route, as on 5.0 and 6.0. ExaBGP 4.2 merged no
  list and 5.0 merged only the extended communities; the bridge follows 6.0, so
  a 4.2 or 5.0 script meets on Ze what it would meet on upgrading ExaBGP.
- **An AS_PATH holding an AS_SET is not sent.** `as-path [ 65001 ] ( 65002
  65003 )` is sent by ExaBGP 4.2, 5.0 and 6.0 and refused by Ze, whose
  `update text` has no spelling for a set. RFC 9774 Section 3: BGP speakers
  "MUST NOT advertise BGP UPDATE messages containing AS_SETs or
  AS_CONFED_SETs". On receive both treat such a route as withdrawn, but ExaBGP
  offers the neighbor leaf `as-set accept` to keep it (`as_set`,
  `src/exabgp/bgp/neighbor/settings.py`) and Ze has no such override. Where ze is
  stricter than ExaBGP without an RFC MUST behind it, ExaBGP's behavior wins, so this difference is
  owed work, not a settled design.
- **A generic attribute must carry flags RFC 4271 allows.** `attribute [ 0x99
  0x60 0x... ]` sends an attribute Ze knows no type for as a well-known one,
  and flags 0x60 or 0x70 on a well-known or optional non-transitive code set
  the Partial bit. ExaBGP 4.2, 5.0 and 6.0 send the flags as written
  (`configuration/grammar/types/bgp.py` hands them to
  `GenericAttribute.make_generic` unchecked). Ze
  refuses the line and names the flags to write. RFC 4271 Section 4.3: "For
  well-known attributes and for optional non-transitive attributes, the
  Partial bit MUST be set to 0."

A value-less attribute (`path-information` alone) and a mask that is no number
(`10.0.0.0/x`) are refused by both. ExaBGP 5.0 ignored the first and took the
second as the host route `10.0.0.0/32`.

A flow route that repeats a prefix component (two `destination` or two
`source` prefixes) is refused by both. RFC 8955 Section 4.2: "A given component
type MAY (exactly once) be present in the Flow Specification." ExaBGP 4.2 and
5.0 sent it as two type 1 or type 2 components; ExaBGP 6.0 refuses to encode
it (`_check_component_order`, `src/exabgp/bgp/message/update/nlri/flow.py`),
and Ze refuses it by name, because a second prefix has no operator list to
merge into the first (`FlowSpec.AddComponent`). A 4.2 or 5.0 script that sent
one meets on Ze what it would meet on upgrading ExaBGP.

Upstream (exa-networks/exabgp at f9bfb853b): `src/exabgp/reactor/api/command/announce.py`,
`src/exabgp/configuration/grammar/tree/static.py`, CHANGELOG 6.0.0.

<!-- source: internal/component/bgp/reactor/reactor_api_carry.go -- SelectedPeersCarry -->
<!-- source: internal/exabgp/bridge/bridge_operational.go -- convertOperational, errOperationalNotSent -->
<!-- source: internal/exabgp/bridge/bridge_attribute.go -- parseRouteAttributes, warnRouteLine -->
<!-- source: internal/core/bgp/nlri/flowspec/unmatchable.go -- CheckMatchable -->
<!-- source: internal/core/bgp/nlri/flowspec/unmatchable_values.go -- checkComponentValues, checkPortValues -->
<!-- source: internal/core/bgp/nlri/flowspec/types.go -- AddComponent -->
<!-- source: internal/component/bgp/textparse/rawattr.go -- CheckRawAttributeFlags -->
<!-- source: internal/component/bgp/message/rfc9774.go -- ASPathSetSegment -->

---

## Neighbor Qualifier Syntax (Multi-Session)

**ExaBGP behavior:**
- Supports six qualifiers after a neighbor address, each taking one value:
  `local-ip`, `local-address`, `local-as`, `peer-as`, `router-id`,
  `family-allowed`.
  - `neighbor <IP> local-as <ASN> announce route ...`
  - `neighbor <IP> family-allowed <families> announce route ...`
- A qualifier NARROWS the set of sessions the command reaches. ExaBGP matches
  each qualifier as a substring of the session's own name, so a command whose
  qualifier does not appear in that name reaches no session and sends nothing.
- `family-allowed` is where `capability { multi-session; }` shows: the name
  carries `family-allowed in-open` on an ordinary session, and the session's
  family list on a multi-session one.
Upstream (exa-networks/exabgp): `src/exabgp/reactor/api/command/limit.py`,
`SELECTOR_KEYS` and `match_neighbor`; `src/exabgp/bgp/neighbor/neighbor.py`,
`name()` and its `in-open` / family-list branch.

**ZeBGP behavior:**
- Uses the send verb and the protocol keyword: `send bgp <IP> update text ...`
- The ExaBGP bridge PARSES all six qualifiers and DISCARDS five of them. The
  command reaches the session the address names.
- A Ze selector names a peer by address, name, ASN or glob. It cannot conjoin a
  predicate onto an address, so there is nothing to translate a qualifier into.
- The exception is `family-allowed`, whose value is the same for every Ze
  session. Ze does not support multi-session (`ai/META-DECISIONS.md`): the
  migration drops the capability with a warning, and `create neighbor` never
  sets it, as ExaBGP's does not.
  ExaBGP names such a session `family-allowed in-open`, so `family-allowed
  in-open` reaches the address, and a family list (`family-allowed
  ipv4-unicast`), which names only a multi-session neighbor, excludes it.
- A selector left with no peer (every address excluded, or every address
  outside the peers the process feeds) is refused: the bridge dispatches
  nothing and answers the reason and `error`, as ExaBGP 6.0's dispatcher does
  (`NoMatchingPeers`). ExaBGP writes the `error` marker alone; Ze adds why.
<!-- source: internal/exabgp/bridge/bridge_selector.go -- bridgeSelectorKeys, splitNeighborSelector, selectorExcludes -->
<!-- source: internal/exabgp/bridge/bridge_command.go -- Translator.servedSelector, errSelectorMatchesNoPeer -->

**RFC compliance:**
- N/A - This is API syntax, not BGP protocol

**Impact:**
- A qualified command is accepted and reaches the peer the address names,
  unless the qualifier is `family-allowed` with a family list.
- The cost is bounded in one direction. An address resolves to at most one
  session in Ze, so a qualifier could only ever have REJECTED that session. Ze
  therefore sends where ExaBGP would have stayed silent, and never the reverse.
- One qualifier does exclude, because it can never match: `family-allowed`
  with a family list, see above. Any other value could name Ze's single
  session, so it is discarded rather than evaluated.

**Tests affected:**
- `test/exabgp-compat/etc/run/api-multisession.run` sends
  `neighbor 127.0.0.1 local-as 1 family-allowed in-open announce route 9.9.9.9/24`,
  and `neighbor 127.0.0.1 local-as 1 family-allowed ipv4-unicast announce route
  1.2.0.0/24`. ExaBGP, whose config there carries `multi-session`, names the
  session `family-allowed ipv4-unicast`, sends the second and drops the first.
  Ze has no multi-session session, so it does the reverse, as ExaBGP does for a
  neighbor without the capability: `test/exabgp-compat/api/api-multisession.ci`
  expects four UPDATE frames from five commands, 9.9.9.0/24 among them and
  1.2.0.0/24 not. ExaBGP 4.2 and 5.0 send `family-allowed in-open` routes
  (`qa/old/commands-*.json`, `TestOldCommands50`, `TestOldCommands42`).
- `test/exabgp-compat/etc/run/api-announcement.run` uses the other qualifier
  types, none of which excludes a session there.

**Decision rationale:**
1. Multi-session to same peer is a rare use case
2. Simpler API implementation
3. Most use cases only need single session per peer
4. A conjunctive selector is a change to the Ze command grammar, which is not
   the bridge's to make

**Date:** 2025-12-23. Corrected 2026-09-06: the bridge parses the qualifiers
since 2026-09-05, so a qualified command no longer fails to translate. The
earlier text said such commands "will NOT work". Corrected again the same day:
`family-allowed in-open` now excludes, so the claim that no qualifier ever does
is gone. Corrected 2026-10-22: `in-open` is the name of every session without
multi-session, which every Ze session is, so it matches and a family list is
the value that excludes.

---

## JSON member spellings the bridge folds

Five members carry the same fact under different words. Ze keeps its own word,
because each is load-bearing on Ze's side, and the bridge writes ExaBGP's word
so a script reads one vocabulary.

| Fact | Ze writes | ExaBGP writes | Why Ze keeps its word |
|------|-----------|---------------|-----------------------|
| Route distinguisher | `0:65000:1` | `65000:1` | Type 0 and Type 2 are indistinguishable for an AS number of 65535 or less, and `ParseRDString` has to read `String()`'s own output back |
| RFC 7911 Path Identifier | `"path-id": 16909060` | `"path-information": "1.2.3.4"` | RFC 7911 Section 3 calls it a 4-octet value, and Ze's CLI and RIB read the number |
| FlowSpec DSCP on IPv6 | `dscp` | `traffic-class` | RFC 8956 Section 8 registers component type 11 as `DSCP` under both its IPv4 and its IPv6 name |
| FlowSpec traffic-marking and traffic-action | `mark:10`, `traffic-action:sample` | `mark 10`, `action sample` | the FlowSpec firewall lowering keys on the colon form |
| Interface set transitivity | `interface-set:non-transitive:input:1234:10` | `interface-set:input:1234:10` plus `"transitive": false` | Ze carries every extended community in one flat list, and ExaBGP has a `scope` block to state it beside |

Nothing is lost either way, so each is a spelling rather than a conversion. The
RFC governs the wire and says nothing about a JSON member, which is what leaves
the choice open.

Where an RFC or Ze's own grammar DOES decide, Ze changed instead of folding: a
two-octet AS Route Origin renders its 4-octet local administrator as a dotted
quad in both projects, and so do `l2info` and the FlowSpec redirect-to-IP
communities, because Ze's config parser already read those spellings and its
renderer wrote something the parser refuses. `update text` reads the redirect-to-IP
spellings too, so a received community can be sent back as written.
<!-- source: internal/exabgp/bridge/bridge_event.go -- exabgpRD, exabgpPathInformation, exabgpFlowComponentName -->
<!-- source: internal/exabgp/bridge/bridge_event_wire.go -- exabgpCommunitySpellings, exabgpInterfaceSet -->

---

## Three UPDATE members come from the wire bytes

Ze's own JSON states three members lossily: the AS_PATH flattened to its AS
numbers, each extended community as its text alone, and an attribute Ze has no
parser for as `attr-<code>`. ExaBGP writes the AS_PATH's segments and their
types, the numeric value beside each extended community, and the flag octet in
the unknown attribute's `attribute-0xCC-0xFF` name. The bridge subscribes at
`FormatFull` whenever a script receives UPDATEs, in any form, `parsed`
included, so every UPDATE event carries `raw.update` beside its parsed body,
and the one UPDATE renderer, `Event.exabgpJSON`, reads those three members off
the UPDATE's own path attributes. `WireUpdateToExabgpJSON`, which the ExaBGP
compatibility fixtures go through, hands its bytes to the same renderer, so a
fixture checks what a live process receives.

The AS_PATH is read with the session's AS number width, which the runner takes
from the session's `negotiated` event (RFC 6793). An UPDATE that reaches the
renderer before any `negotiated` event of its session keeps Ze's flattened path;
a running session always states one first.

The community value is read for both widths, RFC 4360's 8 octets and RFC 5701's
20, and it is built exactly and rounded once. RFC 5701's community is 160 bits
wide and JSON has one number type, so accumulating the octets in a float rounds
at every step: doing that put the member two thousand off an expectation whose
own decimal literal rounds cleanly.

One community gains a second member here. ExaBGP writes `"transitive"` beside
the interface set and beside nothing else, because that community
(draft-ietf-idr-flowspec-interfaceset Section 5) is defined twice, transitive
and non-transitive, differing by one bit of the type octet. Ze states that bit
in the text instead, since it carries every extended community in one flat list
and has no `scope` block to put it in.
<!-- source: internal/exabgp/bridge/bridge_event_wire.go -- overlayWire, overlayASPath, overlayCommunityWidth, overlayUnknownAttributes -->
<!-- source: internal/exabgp/bridge/bridge_wire_json.go -- WireUpdateToExabgpJSON -->
<!-- source: internal/exabgp/bridge/bridge_negotiated.go -- Sessions.Note, sessionAttrContext -->

---

## A FlowSpec NLRI with no component

A received FlowSpec NLRI of length 0 resets the session on Ze. RFC 8955
Section 4.2 writes the NLRI as `<[component]+>`, at least one component, and:
"An NLRI value not encoded as specified here, including an NLRI that contains
an unknown component type, is considered malformed and error handling
according to Section 10 is performed." ExaBGP 6.0 reads it without a
NOTIFICATION: `Flow.unpack_nlri` (`src/exabgp/bgp/message/update/nlri/flow.py`)
raises only on a missing, truncated or overrunning length. ExaBGP 4.2 and 5.0
sent such an NLRI for a flow command with no match component; the old-command
corpus records 99 of them per release, and ze's own row `oldMalformedRecorded`
judges each recording malformed where the pin's judge counts it as nothing
sent.

Where both reset over a malformed FlowSpec NLRI, ExaBGP 6.0 sends NOTIFICATION
3/10 (Invalid Network Field) and Ze sends 3/9 (Optional Attribute Error) with
the whole MP_REACH_NLRI or MP_UNREACH_NLRI attribute as its Data, as for every
MP section it cannot walk in any family. RFC 4760 Section 7 names that subcode
for an incorrect MP attribute, RFC 4271 Section 6.3 puts the attribute in the
Data field, and FRR sends the same; Invalid Network Field belongs to the NLRI
field of the UPDATE itself (`test/plugin/flowspec-malformed-nlri-reset.ci`).
<!-- source: internal/core/bgp/nlri/nlrisplit/flowspec.go -- SplitFlowSpec -->
<!-- source: internal/component/bgp/message/rfc7606.go -- mpNLRIJudgementResult, MPAttributeErrorNotification -->
<!-- source: internal/component/bgp/plugins/nlri/flowspec/rfc8955_syntax.go -- judgeFlowSpecSection, flowSpecMalformed -->

---

## A route aimed wider than a `clear adj-rib out`

ExaBGP keeps one outgoing RIB per peer, so `clear adj-rib out` for one neighbor
pops a route the same read queued for every peer from that neighbor's queue
alone. The bridge hands ze one command per line, and one command cannot be
split by peer. When the route was written for more peers than the clear names,
the bridge keeps the route: it reaches the cleared neighbor and is withdrawn by
the clear in the next UPDATE. Every peer ends with the routes ExaBGP leaves it,
and the cleared neighbor sees one announcement and one withdrawal ExaBGP never
sends. A route written for exactly the cleared peers, or a clear for every
peer, nets as ExaBGP does (`docs/architecture/exabgp-bridge.md`, "Clearing and
flushing the adj-rib-out").

<!-- source: internal/exabgp/bridge/bridge_batch.go -- adjRIBOutClearCovers -->

## The reason of a down

ExaBGP writes why a session ended as the text its reactor logs for it. A
NOTIFICATION reads `notification sent (6,2)` or `notification received (6,2)`
(`src/exabgp/reactor/peer/peer.py`, `Peer._run`), and the bridge writes the
same words from the direction and codes ze's down event carries. A
NOTIFICATION ze could not write told the peer nothing, so that down reads
`notification` alone. A lost transport is `closing connection` and a removed
peer `stop, message [removed]` in both.

<!-- source: internal/exabgp/bridge/bridge_event_state.go -- exabgpDownReasons -->

## Default hold time and Graceful Restart time

ExaBGP offers a hold time of 180 seconds when a neighbor states none
(`src/exabgp/bgp/neighbor/settings.py`, `hold_time: int = 180`), and gives a
Graceful Restart enabled without a time the hold time as its Restart Time.
Ze's `receive-hold-time` defaults to 90 and its `restart-time` to 120. A peer
written for Ze by hand gets Ze's defaults. A config converted by
`ze exabgp migrate` does not: the migration states ExaBGP's values, so the
migrated session offers what the ExaBGP one did
(`docs/exabgp/exabgp-migration.md`, "Hold time and Graceful Restart time").

<!-- source: internal/exabgp/migration/migrate_defaults.go -- exabgpHoldTimeDefault, exabgpGracefulRestartTime -->

## A peer AS not yet known

A neighbor configured `peer-as auto` keeps ExaBGP's default peer AS,
`ASN(0)` with the comment "0 = auto" (`src/exabgp/bgp/neighbor/session.py`),
and `_neighbor` (`src/exabgp/reactor/api/response/json.py`) writes that value
on every line, so an ExaBGP script reads `"peer": 0` under `asn` as "AS not
known". Ze omits `remote.as` while a dynamic peer's AS is not negotiated, and
the bridge writes the same `"peer": 0` for it, never `null`. The one
difference: ExaBGP keeps 0 for the whole session, while Ze names the AS it
negotiated on every line after the OPENs are exchanged.

<!-- source: internal/exabgp/bridge/bridge_event.go -- exabgpPeerASAuto, exabgpPeerASN -->

## Template for Future Differences

### Feature Name

**ExaBGP behavior:** [Description]

**ZeBGP behavior:** [Description]

**RFC compliance:** [Analysis]

**Impact:** [Testing/compatibility notes]

**Files affected:** [List]

**Decision rationale:** [Why ZeBGP differs]

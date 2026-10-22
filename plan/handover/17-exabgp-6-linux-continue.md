# Handover 17: ExaBGP 6 Phase 7 and its spin-off fixes, continue on Linux (2026-10-22)

Written on the Mac at the end of session 8385ff13 for the next session on the
Linux host. The long running log is `03-session-2026-10-18-linux-continue.md`
(its later sections carry per-agent detail); this file is the entry point.

Claimed spec: `plan/immediate/spec-exabgp-6-api-compatibility.md` (Phase 7 in
progress). Claim it again on Linux with `./le spec claim spec <file>`.

## Owner decisions still open (ask, do not act)

| # | Decision | Context |
|---|----------|---------|
| 1 | Cap the Go build-cache size, enforced on every `./le` run? | The Mac disk filled three times on 2026-10-21/22: the checkout, shared and ambient Go caches regrow to about 85 GB within hours with many sessions building. Safe `cache-clean` (803356e056) cannot free young entries by design |
| 2 | Shared-build launcher for `bin/` (content-addressed, hard-linked `le-<name>`)? | 77 `le-<name>` launchers of 137 MB each were mostly identical builds. The 24 h idle trim keeps them because each was used that day |

Owner decisions taken this session are in `ai/META-DECISIONS.md`: M11 (third-party
module changes are local diffs, never a fork) and M12 (full YANG coverage runs
after the initial release).

## Landed this session (main, not pushed by this session; origin moved once)

| Area | Commits |
|------|---------|
| Teardown `request peer notify` and bridge teardown octets as hex, `ze:rest-of-line` | ce28145f2f |
| Bridge route-line escapes, merged lists, last NLRI field, SRv6 0xFFFF | 1cfb359fa1 |
| Aggregator bracketed form | a6c1437035 |
| Label, bgp-prefix-sid, origin/mup forms, end-of-line watchdog | add923ae32 |
| MUP DSD/T2ST address field; ParseRouteArgs deleted; ADD-PATH withdraw-only keeps path ids | 371519fe0e, 2fcb0b6519, 27dbbbd0a6 |
| rr/rs key ADD-PATH unicast by prefix and Path Identifier | 751bdc13d7 |
| MP NLRI split errors rendered; one JSON key per family | 15bd174272, d2682e5b19 |
| rr departure as recovery (ze's Path Identifier, GR retention, interop `bgp-addpath-departure-reflector-frr`) | c379d157b1 |
| rr/rs relay peer-up replay and revalidation themselves (owner-relayed pages, no grant borrowing) | cb65125a65 |
| Generic `attribute [ code flags value ]` in update text, config, bridge, migration (RFC 4271 4.3 flags, typed decode) | 6399061087 |
| goyang fork replaced by upstream + vendor patches (M11); grouped-action fix | c07197a98a, a639bb22e9 |
| Safe `cache-clean` (age floor; `all` refuses under builds) | 803356e056 |
| config-yang-reference-types deferred (M12) | 1577888acb |

## Remaining work, in order

1. **ExaBGP Phase 7, old-command corpus (AC-37/X-35).** `TestOldCommands50` is
   4045/4250 and `TestOldCommands42` 2478/2541 (`internal/exabgp/bridge`,
   `-tags ze_bgp`); both stay red until every class closes. Left: IPv6 flow
   `/N/M` offsets (~24), flow components with no value and `mark`/`action`
   (~24), mpls-vpn `split /N`, mcast-vpn (~34), ipv6 multicast (~64), ipv6 mup
   (~19), unicast withdraw differences, extended community `target4`/`origin4`/
   type 0, vpls withdraw with attributes, mup DSD with `next-hop` twice,
   `eor l2vpn vpls`, `route-refresh l2vpn evpn`, `bgp-ls`, and the
   route-refresh/operational judge branch.
2. **AC-38: `TestOldResponses50/42`.** Port the pin's `qa/bin/test_old_responses`
   and its ALLOWED table over `internal/exabgp/bridge/testdata/exabgp-old/responses-*.json`.
3. **End-to-end RPKI revalidation `.ci` through rr/rs** (owed by cb65125a65's
   journal row, top of `plan/journal/silent-fall-through.md`). Written, never
   run: observer `plugin15RPKIRevalidateRelay` in
   `internal/test/fixture/plugin_fixture_15_rpki.go` (registered in
   `plugin_fixture_15.go`), `test/plugin/bgp-rr-rpki-revalidate-relay.ci` and
   `bgp-rs-rpki-revalidate-relay.ci`, uncommitted on the Mac and carried as
   `plan/handover/17-revalidate-relay-wip.patch` (`git apply` it on Linux;
   delete the patch file in the commit that lands the tests). The rr seq=2 hex is a
   placeholder to pin from the first green run. Design and open risks: the
   "2026-10-22 main session: e2e revalidation .ci" section of handover 03.
4. **Defect (journal, open):** plugin-family routes (flow, sr-policy) drop
   parsed config attributes in `buildConfigRouteRequest`
   (`plan/journal/config-route-request-drops-parsed-attributes.md`).
5. **Defect to confirm then journal and fix:** while a route is held
   ineligible by RPKI, a destination receives repeated withdrawals of a prefix
   never advertised to it, one per gate event (`forward_validation.go`
   `RouteEligible`, rr/rs ValidationChange coalescing).
6. **Small:** `cache-clean` JSON keys `free_before`/`free_after` are snake_case
   against `ai/rules/cli.md`.
7. **Then:** ExaBGP Phase 8, spec review, close the spec.
8. **rfc-verdict-fix-bgp:** blocked on a Linux run for the RFC4271-5.1.2-3
   interop records; Linux is now available.
9. **Push:** refused while verification debt is open; a green full verify is
   owed first.

## How to work on Linux

- ExaBGP pin run: `PYTHONPATH= uv run --no-project --python 3.12 --with paramiko
  --with 'exabgp @ git+https://github.com/Exa-Networks/exabgp@685f772e4ede5c818d2ff5c84f4ae040afd37be6'
  ./le --name <n> test exabgp <population> --dut exabgp <case>`.
- 4.2/5.0 sources: `git -C ~/Code/github.com/Exa-Networks/exabgp/main show <tag>:<path>` (4.2.25, 5.0.1).
- Interop runs natively on Linux (the Mac needed the QEMU docker lab with
  `ZE_QEMU_SSH_PORT=2290`).
- A new fixture or `./le` change is not in the shared `bin/le` until
  `./le --update`; agents used `./le --name <x>` and must delete
  `bin/le-<x>` afterwards.
- The staticcheck parts 2/3 red recorded on the Mac was an environment cache
  fault (dead patched GOROOT after a Homebrew Go upgrade, cache entries removed
  mid-run). A green full verify on Linux should clear the carried structural reds.
- Plugin packages need the full `ze_*` tag set from `feature-gates.txt`;
  `-tags ze_bgp` alone fails OSPF/NTP inventory tests that are not real reds.

## Traps met this session

- Agents run out at 100 tool calls; brief each to write its handoff here and
  commit it by about call 85.
- Never pipe `./le commit create` through `tail`: the `script=` line is lost.
- Journal rows are capped at 600 characters and the Fix cell must start
  `fixed: ` or `open: `.
- The shared `.git/index` can lag HEAD after a commit whose index refresh met
  another session's lock; commits use a private index, so do not "fix" it with
  a reset. `git read-tree HEAD` is the owner's call.
- An RFC-tagged test change needs the owner's `./le rfc approve`; after the
  fact, record it with `./le commit debt-discharge ... kind owner`.

## Linux session ed4b8d60 state (2026-10-22)

Item 1 (old-command corpus) landed in one commit, "exabgp: old-command corpus
closes flow, unicast, mup and control classes": `TestOldCommands42` green
2541/2541, `TestOldCommands50` 4248/4250. The two red rows are `announce
operational adm|asm ... advisory`, held on OD-1, put to Thomas 2026-10-22 and
unanswered: keep the refusal (two ALLOWED rows) or implement origination (own
spec). The AS_SET row is ALLOWED under RFC 9774 Section 3 and recorded in
`docs/exabgp/exabgp-differences.md`. `api-multisession.ci` (upstream port) now
expects the `family-allowed in-open` route and not the `ipv4-unicast` one,
because Ze has no multi-session capability, so every session is ExaBGP's
`in-open` one; flagged to Thomas.

Item 3 is being finished by another session (handover 19's "Linux session
result" section: fixture race fixed, stub filled, both red walks done).

Thomas, 2026-10-22: "do not run full testing on this host, it is too busy".
So no `./le verify worktree`, full suite or full lint here; scoped package runs
only. Owed elsewhere for this commit: `./le verify worktree`, `./le go lint
run`, `./le test functional exabgp-test` (incl. `api-multisession`), `./le test
functional encode` (new update-text keywords `mark:`, `traffic-action:`, flow
`/len/offset`), `./le rfc check`, independent review. Journal row added:
`validate-accepts-what-start-refuses.md` (judge accepts `watchdog` that update
text refuses). That row is inside AC-37, not beside it: about 95 5.0 lines
pass only because the judge skips the daemon's refusal, so AC-37 is not met
until the judge applies `handleUpdateText`'s refusals and update text
implements the watchdog route form.

Next: the watchdog route form (above), item 2 (AC-38, `TestOldResponses50/42`),
then items 4 to 8.

## Agent old-responses (2026-10-22 Linux)

Item 2 (AC-38) has its judge, and the judge is RED. `internal/exabgp/bridge/old_responses_test.go`
ports the pin's `qa/bin/test_old_responses` and its ALLOWED table: 17 rows, the
marker "End of the rows copied from the pin. Ze's own rows follow.", and no ze
row yet. Each event goes through the engine's subscriber formatter
(format.AppendMessage / AppendSentMessage, JSONEncoder, AppendStateChange), then
`ReadEvent` and `Event.Line(FormatV4Text|FormatV4JSON)`. The session is the
first neighbor in `testdata/exabgp-old/configs-<release>.json.gz`, named as the
release named it, together with its OPEN. Set `ZE_OLD_VERBOSE=1` to list every
failure. The log prints `class <count> <encoder> <kind> <where>`.

Commits: d1eef6673c the judge; 3d46585641 router-id on the non-UPDATE JSON
(format.peerMap); 0ca6982927 BoRR and EoRR folded into `refresh`, with the words
query, begin and end; 95a6236e93 the text `route eor` line. Counts: 5.0 went
from 4449 to 8091 of 23392 lines, 4.2 from 7391 to 8583 of 20930. The 117 (5.0)
and 2751 (4.2) lines the release could not write are skipped, as the pin skips them.

Classes left, largest first (5.0 counts):
1. No `packets` or `consolidate` rendering, about 9000 lines across the kinds.
   These are the header and body forms of ExaBGP's `receive { packets;
   consolidate; }`. Ze must deliver the raw message (FormatFull), and the
   bridge needs a render mode per process. This is a feature.
2. `neighbor.negotiated` on every UPDATE JSON line, 2000 lines. 5.0 writes the
   session's negotiated block inside each update line. The bridge has to keep
   the peer's negotiated event and attach it to each line. Read 5.0's
   `response/json.py` first.
3. Events ze has no producer for: fsm (864 JSON), signal (288), connected
   (144+144), shutdown (144+144), and OPERATIONAL messages (288+288, plus their
   packets forms).
4. OPEN, about 300 text and 16 JSON lines. The text capability list uses ze's
   own words (`unknown-64 ...`) where ExaBGP writes `graceful restart flags ...`,
   `route refresh` and `operational`. JSON capabilities come out `{}` for an
   OPEN with RFC 9072 extended optional parameters, so DecodeOpen misreads it.
5. The `negotiated` event's JSON content (144), and the UPDATE attribute and
   NLRI forms: the extended-community string (160); as-path (28), because the
   overlay in WireUpdateToExabgpJSON does not run on the runtime path; flow
   next-hop and string; aggregator, cluster-list and originator-id; the
   labeled, flow, vpls, mvpn and mup text lines.
6. The `down` reason, 144 JSON and 144 text lines. Ze sends tcp-failure, while
   the pin's judge hands `peer reset` to both encoders. To decide: map ze's
   reason words to ExaBGP's in the bridge, or add a ze ALLOWED row with an
   argued reason.
7. 40 UPDATEs, JSON and text, that get no line from ze: the judge drops an
   UPDATE whose Attrs() errors or that `oldSessionReset` resets. Read each one.

Next step: class 2, which is bridge-only and worth 2000 lines, then 4, then 5.
Classes 1 and 3 are absent features that the pin's corpus exercises (M3). They
are owed, and the main thread decides whether they run in this spec or in their own.
Owed gates, not run because of the host rule: `./le verify worktree`, `./le go
lint run`, `./le test functional exabgp-test`, any `.ci` that pins exact OPEN,
KEEPALIVE, NOTIFICATION or refresh JSON (router-id is now present, and borr and
eorr now arrive as `refresh`), and independent review.

## Agent event producers (2026-10-22 Linux)

Classes 3, 6 and 7 of the list above. Counts with the parallel agents' work in
the tree: 5.0 from 8091 to 8747 of 23392, 4.2 from 8583 to 9151 of 20930.

Class 7, landed in 4d60f7db2d. The 40 UPDATEs: 32 were the judge, which
answered ADD-PATH yes for every family (`oldSessionReset`), so an IPv6
MP_REACH prefix lost four octets to a Path Identifier. It now asks the receive
context per family, as `validateRFC7606Attrs` does. The other 8 carry type 153
with the Optional bit clear (flags 0x60 and 0x70). Ze resets on RFC 4271
Section 6.3 ("If any of the well-known mandatory attributes are not recognized,
then the Error Subcode MUST be set to Unrecognized Well-known Attribute.").
Two ALLOWED rows quote it.

Class 6, done, NOT committed. `bridge_event_state.go` maps ze's down reason to
the words ExaBGP's reactor hands Processes.down (pin `reactor/peer/peer.py`):
tcp-failure and local-close are `closing connection`, removed is `stop,
message [removed]`, notification is `notification`. A reason the table does not
name passes through. The JSON down line now carries `reason`, and before this it
carried none. Tests: `bridge_event_state_test.go`. Pages:
`docs/architecture/exabgp-bridge.md` and `docs/exabgp/exabgp-differences.md`
("The reason of a down"). The pin's recorder feeds `peer reset`, which no
reactor writes, so two ALLOWED rows in `old_responses_test.go` quote it. The
commit is held because `bridge_event.go`, `bridge_event_text.go` and
`old_responses_test.go` carry the OPEN/negotiated agents' in-flight hunks:
`convertNegotiated` moved out to `bridge_negotiated.go`, which is untracked, so
a commit now would break HEAD. Commit these together with the files of that
agent once it lands: `bridge_event_state.go`, `bridge_event_state_test.go`, the
`reason` hunk in `ExabgpJSON`, the `exabgpDownReason` line in
`appendStateText`, the four ALLOWED rows, and the two pages. Still owed: ExaBGP
writes `notification sent (c,s)` or `notification received (c,s)`, and ze's
down event carries neither the direction nor the codes. Deliver them on the
down event (reactor `sessionCloseFor` knows `notified` only), then extend the
table.

Class 3, NOT started: an engine feature. Ze has no producer for any of the
four events.
- `connected`: `format.JSONEncoder.stateConnected` exists and nothing calls
  it, and `rpc.SessionState` has only Up and Down. ExaBGP sends it on TCP
  establishment under `neighbor-changes`.
- `fsm`: one event per FSM transition, with the state named IDLE, ACTIVE,
  CONNECT, OPENSENT, OPENCONFIRM or ESTABLISHED (JSON `type: fsm`,
  `neighbor.state`). The hook is `fsm.FSM.SetCallback`
  (`internal/component/bgp/fsm/fsm.go`).
- `signal`: SIGHUP and SIGTERM, per neighbor, `code` and `name`, from
  `reactor/signal.go` `handleSignal`.
- `shutdown`: `{"notification":"shutdown","type":"notification"}`, with text
  `shutdown <pid> <ppid>`, written to each process on daemon stop (pin
  `processes.py:408`).
Each one needs an event type registered, delivered as `onPeerStateChange`
(`bgp/server/events.go`) delivers its events, a bridge renderer, and an
`APIKey` grant. The bridge already names `fsm` and `signal`. Text writes no
line for either, so the text halves already pass. The judge's `zeDocument`
must then build each document from the new format producers. OPERATIONAL
remains owed as ALLOWED rows quoting M15.

Housekeeping: `tmp/commit-005e4774-a-687a42.sh` is a duplicate script from a
refused `create`. Never run it. Owed gates: `./le go lint run`, `./le verify
worktree`, `./le test functional exabgp-test`, and independent review.

## Agent OPEN/UPDATE forms (2026-10-22 Linux)

Classes 4 and 5 of "Agent old-responses". Counts (judge `TestOldResponses`):
5.0 went from 8091 to 10931 of 23392, 4.2 from 8583 to 11255 of 20930. The
tree held the other agents' uncommitted work too, so not all of the gain is
this agent's.

Landed:
- 0d19c740ca `format.DecodeOpen` read the optional parameters with a
  one-octet loop of its own, so an RFC 9072 extended OPEN listed no capability
  (ExaBGP 5.0's twenty-family OPEN). It now walks with
  `capability.WalkOptionalParams` and `ExtendedParams`. An Extended Next Hop
  that lists no family is now one entry with no value (ExaBGP's `nexthop()`).
  Tests: `TestDecodeOpenExtendedParams` (red seen with the flag forced false),
  `TestDecodeOpenEmptyExtendedNextHop`. Page: `json-format.md`.
- 5b86bdd18e the judge links bgp-rr (`old_responses_plugins_test.go`), as the
  daemon does. Originator-id and cluster-list came out as `attr-9` and
  `attr-10` hex because of the test's setup, not the bridge.

Class 4 is DONE and NOT committed: no OPEN line is left in either release,
packets and consolidate aside. The files: new `bridge_event_open_text.go`
writes ExaBGP's `str(Capabilities).lower()`, per code in code order, built
from the JSON body `capabilityBody`, so a refused payload is `unknown <name>`
on both encoders, and GR, Add-Path and Multiple Labels are read in wire order.
New `bridge_event_open_text_test.go` holds `TestCapabilitiesText`. Hunks in
`bridge_event_text.go`: `appendOpenText` calls it, `asn` is `openMyAS`
(AS_TRANS), and `appendJSONNumber` takes an `int`. Hunks in
`docs/architecture/exabgp-bridge.md`: the paragraph after the down reason, and
the diverge table loses its capability row ("Two places"). It is held because
both shared files also carry the event-producers agent's down-reason hunks,
whose referent `bridge_event_state.go` is not in HEAD. Commit it in the same
batch as that agent's class 6 (its section above lists the files).

Class 5 left (5.0 counts, first difference per line only, so more hide behind):
- The `negotiated` event content is at 0, from the negotiated agent's
  uncommitted `bridge_negotiated.go` `convertNegotiated`. Nothing done here.
- extended-community 160 (4.2 116), extended-community-ipv6 4, as-path 28.
  ExaBGP writes `{string, value}` members, AS_PATH segments, and its own
  `origin:<asn>:<ip>` order. `WireUpdateToExabgpJSON` already does all three
  with `overlayASPath` and `overlayExtendedCommunities` from the PARSED wire
  attributes, but only `internal/le/interoplab/bgp/exabgp_server.go` calls it.
  The runtime `Event.Line` path renders from ze's JSON event, which carries the
  display text alone. One path needs the wire attributes on the runtime path,
  which is the FormatFull delivery class 1 (packets) needs. Design: when the
  bridge holds the UPDATE's raw bytes, render through the overlay functions,
  then delete whichever renderer the other duplicates (no-layering).
  Coordinate with the packets agent, which owns the delivery.
- aggregator 2 (4.2 2): no `RegisterJSONFormatter(AttrAggregator, ...)`
  exists anywhere, so ze writes `attr-7` where ExaBGP writes `"<asn>:<ip>"`.
  Owed: a formatter in `internal/core/bgp/attribute/register.go`, and its page.
- flow: `string` brackets a multi-value port (`destination-port [ >=80&<=90 ]`,
  2); the next-hop of flow and flow-vpn routes (8+4); 4.2 `destination-ipv6`
  with an offset (4); bgp-prefix-sid (4.2, 4); community/N (4).
- Text UPDATE lines, about 250: labeled (where `label N` goes), where the
  next-hop goes, path-information, flow, vpls `rd ... endpoint`, mvpn
  (`afi ipv4 safi mcast-vpn`), mup (`afi ipv6 safi mup`). These are the
  `extensive()` text per family. Read the pin's NLRI `extensive()` and `str()`
  first.

No ALLOWED row added. Owed gates (host rule): `./le go lint run`, `./le verify
worktree`, `./le test functional exabgp-test`, any `.ci` that pins a text OPEN
line (the capability words changed), and independent review.

## Agent packets/negotiated (2026-10-22 Linux)

Class 2 (`neighbor.negotiated` on every UPDATE JSON line) is done in the working
tree and NOT committed. Its hunks share `internal/exabgp/bridge/bridge_event.go`
with the down-reason agent's (`exabgpDownReason`, defined in the untracked
`bridge_event_state.go`), and each side's hunks reference a file only the other
adds, so ONE commit must carry both sets. Counts (TestOldResponses):
5.0 8091 -> 10923 of 23392, 4.2 8583 -> 11249 of 20930. The negotiated class
went from 2000 (5.0) and 1946 (4.2) to 0, and the `negotiated` event's 144 too.

My files and hunks:
- `internal/component/bgp/format/json.go` `JSONEncoder.Negotiated` writes
  `message-size`, `route-refresh`, `extended-next-hop` (decoded, never written).
  Test `TestJSONEncoderNegotiated`; page `docs/architecture/api/json-format.md`.
- `internal/exabgp/bridge/bridge_negotiated.go` (new) and its test: `Sessions`
  (per peer, from `negotiated` to `down`), `Event.WithoutNegotiated`,
  `APIKeyNegotiated`, `convertNegotiated` moved here and rewritten to 5.0's
  `_negotiated` (families in AFI/SAFI order, `nexthop`, both `add_path` lists,
  `multisession` and `operational` false).
- `bridge_event.go`: the `Event.Negotiated` field, the UPDATE case writing it in
  `neighbor`, the `negotiated` event's block moved inside `neighbor`.
- `bridge.go` (external runner, always attaches) and `bridgerun/fleet.go`
  (internal runner, no block for a script whose feed lacks `negotiated`, since
  ExaBGP passes None) call `Sessions.Note`. Test
  `TestFleetPutsTheNegotiatedBlockOnlyWhereGranted` in `fleet_test.go`.
- `bridge_test.go`: the three negotiated tests read the block from `neighbor`.
- Judge: the session takes `HoldTime` from the OPEN and notes its negotiated
  event; two ze ALLOWED rows, M16 (multisession) and M15 (operational).
- `docs/architecture/exabgp-bridge.md`: the section "The negotiated block rides
  on every UPDATE line".

Class 1 (packets and consolidate, about 9000 lines) is NOT started beyond the
facts:
- 5.0 `reactor/protocol.py` `_to_api` and `read_message`. `consolidate` together
  with `packets` writes the parsed line plus `header` (19 octets: marker,
  length, type) and `body`. `packets` without consolidate writes
  `Response.packets`: `message {category, header, body}` plus the negotiated
  block. `parsed` writes the plain line. Text: `neighbor X receive 2 header 0x..
  body 0x..`, with the body left out when empty. A consolidated UPDATE text adds
  a ` header .. body ..` line before `update end`.
- Ze: an UPDATE has `FormatFull` (`raw.update`). OPEN, NOTIFICATION, KEEPALIVE
  and ROUTE-REFRESH JSON in `server/events.go` `formatMessageForSubscription`
  carry no raw bytes. The header is derived from the body length and the type.
- The bridge subscribes with format "" (`SetStartupSubscriptions` in
  `main_sdk.go` and `bridgeplugin/internal.go`).
- Owed: a per-(peer, script) render mode from the migrated feed grants
  (`receive-packets`, `receive-consolidate`, `receive-parsed`, `send-packets`;
  check what `ze exabgp migrate` writes), the `full` subscription, raw bytes for
  the four non-UPDATE kinds, the judge's `packets` and `consolidate` branches in
  `zeDocument`, and a `.ci` through a migrated config.

Next step: the main thread lands class 2 and class 6 in one commit, then class 1
starts at `ze exabgp migrate` and `api { receive { packets; consolidate; } }`.
Owed gates (host rule): `./le go lint run`, `./le arch tier check`,
`./le test functional exabgp-test`, `./le verify worktree`, independent review.

## Agent fsm/signal/connected/shutdown (2026-10-22 Linux)

Class 3 (fsm, connected) and the NOTIFICATION half of class 6. Judge counts
with every agent's work in the tree: 5.0 12083 of 23392, 4.2 12255 of 20930.
The `fsm` (864 / 750) and `connected` (144+144 / 125+125) classes are at 0.
`signal` (288 / 250) and `shutdown` (144+144 / 125+125) remain.

Design, and why it departs from the brief's "e.g.": `connected` is NOT a third
`rpc.SessionState`. 42 plugin files read `state`, and some read every value
other than `up` as a down (watchdog `watchdog.go` text handler,
redistribute_egress `register.go` text handler, both an `if state == "up"`
with an `else`), so a third value would turn each TCP connect into a peer-down
for them and for any external plugin written the same way. It is its own event
type. Contract change (page `docs/architecture/api/json-format.md`, "Connected
and FSM Events"):
- `connected`: `{"message":{"type":"connected"},"peer":{...}}`.
- `fsm`: `"fsm":{"from":"CONNECT","to":"OPENSENT"}`, the fsm.State names.
- a `down` whose reason is `notification` carries
  `"notification":{"direction":"sent"|"received","code":N,"subcode":N}` (text
  ` notification sent 6 4`), from `plugin.PeerInfo.CloseNotification`, filled by
  `Peer.closingNotification` through `sessionClose.notification`.

Engine files, uncommitted, mine alone: `internal/core/bgp/events/events.go`,
`bgp/plugin/register.go`, `plugin/types_bgp.go`,
`bgp/reactor/{peer_close,peer_session_end,peer_stats,reactor_api,peer_run}.go`,
new `bgp/reactor/peer_fsm_events.go` and `peer_closing_notification_test.go`,
`bgp/format/{text_json,text_human,json,json_test}.go` (`JSONEncoder.stateConnected`
deleted with its test), new `bgp/format/session_events.go` (+_test),
`bgp/server/events.go` (`deliverPeerLine` serves congestion, connected, fsm),
`bgp/server/event_dispatcher.go`, `internal/test/fixture/plugin_fixture_12.go`
(one Register line), new `plugin_fixture_fsm_events.go`, new
`test/plugin/fsm-connected-events.ci`, pages `json-format.md`,
`docs/guide/bgp-peering.md` (the namespace sentence), `docs/config-reference.md`
(receive table). Green: format, server, plugin, fixture packages whole (race);
reactor `TestClosingNotification`, `TestSessionCloseFor`.

OPEN DEFECT, blocks the goal, fix first: `fsm-connected-events.ci` is RED. The
plugin gets OPENCONFIRM and ESTABLISHED, never `connected` nor OPENSENT.
Cause: `Session.Connect` and `Session.Accept` fire Event 17 (the
`fsm.EventTCPConnectionConfirmed` call in `session_connection.go`) and enter
OpenSent BEFORE `Peer.runOnce` (or whichever function in `peer_run.go` holds
it) installs `session.fsm.SetCallback`, which comes after the dial block. Fix:
install the callback, and the `leave` record it closes over, before the
accept/dial block; rerun `./le --name fsm test bgp plugin fsm-connected-events`,
then `rm -rf bin/le-fsm`. Then commit the engine set above in one commit.

Bridge, uncommitted, in the held files (main thread's batch): new
`internal/exabgp/bridge/bridge_event_fsm.go` and `bridge_event_fsm_test.go`
(3 tests green); `bridge_event.go` (the `readConnected` call at the end of
ReadEvent, `case msgTypeFSM` in ExabgpJSON, the down reason call passes
`e.Payload["notification"]`); `bridge_event_text.go` (the same call);
`bridge_event_state.go` (`exabgpDownReason(reason, closing)`,
`closingNotificationWords`, doc); `old_responses_test.go` (`connected` and `fsm`
from the format producers, `oldFSMStates`); pages `exabgp-bridge.md` and
`exabgp-differences.md` ("The reason of a down").

Not done, facts for the next agent:
- `signal`: in production the HUB owns signals (`cmd/ze/hub/main.go`, the
  `signal.Notify` for stopCh and hupCh, and `waitLoop`); the reactor's
  `SignalHandler` runs only standalone (`Reactor.externalServer`). ExaBGP fans a
  signal out to every peer with `api signal` (5.0.1 `reactor/loop.py`, the
  `processes.signal` loop), JSON `code` as a string and `name`. Route: the hub
  publishes it on the EventBus, the reactor subscribes and delivers a per-peer
  `signal` through `deliverPeerLine`; the bridge maps it.
- `shutdown`: the SDK already delivers `bye` before stop (`sdk.OnBye`), and no
  bridge runner registers it. JSON `{"notification":"shutdown","type":
  "notification"}` with no neighbor, text `shutdown <pid> <ppid>`.

Owed gates (host rule): the reactor package race run (started, unread),
`./le go lint run`, `./le arch tier check` (format and server now import
bgp/fsm), `./le verify worktree`, `./le test functional exabgp-test`,
independent review.

## Agent packets (2026-10-22 Linux)

Class 1 (`packets`, `consolidate`, `parsed`) is DONE in the working tree.
Judge counts with every agent's work in the tree: 5.0 10931 -> 22090 of 23392,
4.2 11255 -> 19881 of 20930 (before the M15 row: 20938 and 19636). Every
`packets` and `consolidate` class is at 0 except what the parsed line already
carries (class 5: extended-community, as-path, flow next-hop, aggregator,
labeled text) and 3+3 4.2 `update packets` the reactor resets the session on
(ExaBGP writes the packet before decoding, ze delivers no event).

Design:
- Engine: `JSONEncoder.Message(peer, msg, fmtMode)` (new
  `bgp/format/json_message.go`) is the ONE JSON path for OPEN, NOTIFICATION,
  KEEPALIVE, ROUTE-REFRESH; a `full` subscriber gets `"raw":{"message":"<hex
  body>"}` (the name `raw` format already used). `json.go`: the four methods
  became `e.marshal(e.<kind>Payload(...))`. `server/events.go`
  `formatMessageForSubscription` calls Message for JSON. `text_update.go`: an
  EOR under `full` JSON is the `eor` object plus `raw.update`
  (`isEORJSONUpdate`, `appendEORUpdateJSON(..., full)`); it was an empty
  `update:{}` before. Text encoding of the four kinds is unchanged (no raw).
- Bridge: both runners subscribe `plugin.FormatFull` (`main_sdk.go`,
  `bridgeplugin/internal.go`). New `bridge/bridge_event_packets.go`: `Packet`
  (type code + hex body, read in `ReadEvent`), `Forms`/`FormsFor` (the pin's
  `_tell_api_received` / `_to_api` table), `Event.Lines(format, forms, log)`.
  `bridge_event.go`: `Event.Packet`, `readPacket` call, `exabgpEnvelope`
  extracted from ExabgpJSON. `bridge_negotiated.go`: Note gives every message
  kind the block (the packet line writes it). Fleet: cache
  `[2][FormsCount][FormatCount]`, `script.forms(peer, direction)`; a script
  with no feed is FormParsed. External runner unchanged (no feed, parsed).
- Migration: `appendFormGrants` writes `<dir>-packets`, `<dir>-consolidate`,
  and `<dir>-parsed` only beside one of them (keeps `event [ receive-update
  send-update ]` of TestMigrateCarriesTheEventsEachNeighborGrants).
- Judge: zeDocument renders packets/consolidate through messageDocument with
  `FormatFull` and `encoder.Message`; `oldForms`; Note on every non-state
  event; M15/M16 negotiated rows widened to `^(open|notification|keepalive|
  refresh) packets `; new M15 rows `^operational ` (json, text).
- Pages: `json-format.md` (after "With Raw Wire Bytes"), `exabgp-bridge.md`
  (new section "A message as its packet", negotiated paragraph), YANG `event`
  description.

Divergences stated on the page: consolidate/parsed are per script, ExaBGP per
neighbor; a direction naming no form word gets the decoded line (ExaBGP:
nothing).

Class 5 single path NOT done: the bridge now holds every UPDATE's bytes
(`Event.Packet.body`), so `overlayASPath`/`overlayExtendedCommunities`
(`bridge_wire_json.go` `WireUpdateToExabgpJSON`) can now run on the runtime
path. Owed: decode `Packet.body` with the session's encoding context
(`sessionContextID`, as the judge does) in `updateMessageJSON`, apply the
overlays, then delete whichever renderer duplicates the other.

Files, NOT committed. Mine alone: new `bgp/format/json_message.go`,
`json_message_test.go`, `bridge/bridge_event_packets.go`,
`bridge_event_packets_test.go`, `bridgerun/forms_test.go`,
`migration/migrate_api_forms_test.go`; `migration/migrate_api_events.go`,
`bgp/format/text_update.go`, `bridge/bridge_negotiated.go`,
`bridgerun/{fleet,script,fleet_test}.go`, `main_sdk.go`,
`bridgeplugin/internal.go`, the YANG, `exabgp-bridge.md` (also fsm agent's
hunks). Shared with the fsm agent (carry in its batch): `bgp/format/json.go`,
`bgp/server/events.go`, `bridge/bridge_event.go`, `old_responses_test.go`,
`json-format.md`.

Green (race, scoped): format, server, migration, plugins/exabgp/..., bridge
except TestOldResponses50/42 (AC-38 not complete, class 5 left).

OWED: a `.ci` through a migrated `receive { packets; consolidate; }` config
(not written); `./le go lint run`, `./le arch tier check`, `./le verify
worktree`, `./le test functional exabgp-test`, `./le test functional plugin`
(the bridge subscribes `full` now), independent review.

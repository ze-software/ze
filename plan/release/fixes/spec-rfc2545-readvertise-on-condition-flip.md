# Spec: an RFC 2545 condition flip leaves the peer with a next hop it cannot resolve

| Field | Value |
|-------|-------|
| Status | ready |
| Priority | P1 |
| Release reason | Honor the explicit owner requirement to reconcile next hops after interface condition changes. |
| Scope | protocol |
| Depends | - |
| Phase | ready |
| Handoff | - |
| Updated | 2026-11-22 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Ownership and the removed attempt (2026-10-31)

**Owner decision:** this spec owns the re-advertisement when a Link-Local
answer changes, whether the RFC 2545 Section 3 condition flipped on an address
change or the Link-Local the neighbor table names for another router on the
shared subnet moved. It also owns the replay-ordering safeguard that pass
needs: the receive cut taken from the sources (`Peer.receiveCut`,
`receive_publication.go`), the drain of each source to its cut
(`drainResendSources`, which no file holds yet), then
`Process.DrainEventsApplied` (`internal/component/plugin/process/delivery.go`)
before the relay starts. No other spec carries this work.

**Ledger row owned here since 2026-11-14:** the gap half of `RFC2545-3-6`.
spec-bgp-unowned-rfc-rows (its row 5, AC-5) built and proved the tested half,
the third-party Link-Local read from the neighbor table at advertisement, and
was split on 2026-11-14 (owner: "split bgp-unowned-rfc-rows") so it could close
on that. The row in `rfc/short/rfc2545.md` stays `{partial}`: its gap is "a
route already sent is not re-advertised when the table later names it", which
is AC-4 below. The clause of that spec's AC-5 that moved here, unchanged: "A
neighbour change for a cached next hop refreshes the cache, and a changed
Link-Local re-advertises the affected prefixes." The refresh landed there
(`TestRFC2545NeighbourChangeRefreshes`); the re-advertise is this spec's. Its
planned unit test was `TestRFC2545NeighbourChangeRefreshesAndReadvertises`
(`internal/component/bgp/reactor/`): a neighbour-changed event for a cached
next hop refreshes the cache, and a changed Link-Local re-advertises the
affected prefixes; an event for an uncached address does nothing. Its design
text (that spec's Design resolutions, I3) is the re-send path this spec and
`plan/release/fixes/spec-bgp-link-local-only-next-hop.md` already describe.

**An earlier attempt was removed from the tree** because of two review
blockers. Its patch and its two new files are kept as a starting point, not
as a design to restore:

| File | What it holds |
|------|---------------|
| `plan/records/rfc2545-readvertise-attempt/itemC.patch` | The diff over tracked files. It applies to HEAD's index as of 2026-10-31 (`git apply --check --cached`). It touches `docs/architecture/update-building.md`; `policy_reload.go`, which splits `Peer.resendAdvertised` out of `reconcileExportFinal`; `reactor.go`, with the `readvertise` queue field and `startReadvertise` in `StartWithContext`; `reactor_iface.go`, where `refreshPeerLinkScopes` queues a peer whose third-party Link-Local moved; `reactor_iface_test.go` (`TestBGPSharedListener`); and `internal/core/network/neighbors.go`, which adds `NeighborLinkLocals.Found` |
| `plan/records/rfc2545-readvertise-attempt/reactor_readvertise.go.txt` | The new `reactor_readvertise.go`: `readvertiseQueue` (a set of peers with a capacity-one wake), `readvertiseWorker` (a long-lived worker on the reactor wait group), `Peer.readvertiseLinkLocalChange` (Established and RFC 8212 export-final gates, a 2-minute pass bound) and `thirdPartyLinkLocalsMoved` |
| `plan/records/rfc2545-readvertise-attempt/rfc2545_third_party_readvertise_test.go.txt` | The new unit test file: a peer on 2001:db8:1::/64 with one static route via the third router, fake address and neighbor tables, asserting the re-sent UPDATE carries the moved Link-Local |

The two `.go` files carry a `.txt` suffix so the module does not compile them.
They are historical inputs. The current proposal does not authorize restoring
them unchanged.

| ID | Class | Finding | What the next attempt owes |
|----|-------|---------|----------------------------|
| B1 | BLOCKER | The replay through `resendAdvertised` skipped the ordering safeguard: no receive cut, no drain of each source, no `DrainEventsApplied`. A route withdrawn or replaced on the live rail during the pass could land at the peer after its live replacement, so the peer ends holding a stale or withdrawn route | The relay starts only after every source's publication reaches its cut and adj-rib-in's process applied every prior event, under the one per-peer hold, as Task describes. A test proves a withdrawal received during the pass is the last word on the wire |
| B2 | BLOCKER | Only unit tests covered it. No daemon-level test drove an address or neighbor change through the real watch to an UPDATE on the wire, and no interop scenario showed FRR or BIRD resolving the new next hop | A `.ci` test through the daemon, and an interop scenario (extend `bgp-rfc2545-third-party-linklocal-frr` or add a named one) where the peer's RIB shows the new Link-Local after the change |
| ISSUE 3 | ISSUE | Each pass re-sends the peer's whole table, and nothing debounces a burst of changes. The capacity-one wake collapses only changes that arrive while a pass is already queued, so a flapping link drives one whole-table re-send per flap | A bound on passes per peer per interval, or a settle delay before a pass, with a test that a burst gives one pass |
| ISSUE 5 | ISSUE | The config-static re-send races `sendingConfigStatic` (`peer.go`). It is one per-peer atomic that `notifyMessageReceiver` reads for every sent event, to tag it config-static so the RIB skips storing it in ribOut. `sendInitialRoutes` raises it under `staticMu`, but it clears it after `staticMu` is released and after default-originate (`peer_initial_sync.go`). `restoreConfigured` `Swap`s it and restores the old value (`peer_static_restore.go`). A pass that raises it while another writer sends to the same peer tags that writer's plugin or forwarded routes config-static, and an initial sync that clears it during a pass untags the pass's static routes. (Mechanism read at the producers on 2026-10-31; the review's own wording is not in the tree) | The marker belongs to the send, not to the peer: carry it on the send itself, or serialise every writer that reads it. Add a race test (`-race -count=20`) of a pass overlapping initial sync and a plugin send |

## Task

### Current research outcome and unapproved proposal (2026-11-16)

The investigation isolates a missing retained-route replay trigger. Current
facts reach a fresh send and an explicitly requested replay, but a watch
change schedules no replay. This update records research and a design
proposal only. Research/design approval, implementation and native proof
remain open. No existing owner criterion or prior review obligation is reduced.

| Evidence | Observed result | Boundary still owed |
|----------|-----------------|---------------------|
| `job-rfc-boundaries-final-race-cfc4586c.log`, in the campaign scratch directory below | Known-neighbor controls pass on both rails. Eight late-discovery cases, two known-identity replacements and two subnet-loss cases fail at the missing second advertisement. Two callback-concurrency cases fail because no replay reaches the destination worker. No data-race report appears | A repaired trigger and a current race run. The concurrency cases did not reach their later store-removal or final-wire assertions |
| `job-rfc2545-neighbor-lifecycle-79449ba8.log` | Snapshot prerequisite, explicit refreshed replay and queued-withdrawal controls pass | Actual callback integration. A test that requests replay itself does not prove the watch scheduled it |
| `job-rfc2545-neighbor-flip-array-fixed-ed3f0b43.log` and recording `EAJH6KKM6JYHKLODFPJUXQSQFF.jsonl` | FRR assertions 1–8 pass. Assertion 9 fails after one subject send and 31 global-only observations. Fresh paired control and unchanged Established sessions pass | A successful retained-subject transition and assertion 12's installation through the actual Link-Local |
| `job-rfc2545-final-helpers-race-cover-2dbad3af.log` | Seven checker-helper roots pass under the race detector | Live protocol repair. These helper results supply no FRR acceptance or RFC credit |

The fourteen trigger reds are existing tests, not fourteen new tests to write.
The smallest safe repair must share an ordered refeed primitive with policy
restoration. Calling `reconcileExportFinal` from the watch is insufficient:
its store drain currently precedes its replay hold, it captures no per-source
receive cuts, and its configured-route phase uses `sendingConfigStatic`.
Restoring the removed attempt would preserve those recorded blockers.

| Alternative | Existing mechanisms reused | Trade-off and proposed disposition |
|-------------|----------------------------|-----------------------------------|
| One reactor-owned pending-peer worker | Watch snapshots, policy replay holds/flush, source cuts, registered receive-store pages and normal route writers | Recommended for review. Pending state is bounded by peers and one worker is joined at shutdown. Scheduling must prevent a slow peer or repeated flaps from causing unbounded delay |
| One coalescing worker per affected peer | The same hold/cut/page/writer path | Independent peers can progress, but every peer adds a worker/timer lifecycle. Use only if a bounded shared worker cannot meet the required behavior |

The proposed pass raises the destination's existing replay hold before any
source cut or retained-page read. It then captures each relevant source's
`receiveCut`, waits for publication to that cut, and calls the receive store's
`DrainEventsApplied`. The previously specified two-second budget covers both
waits, with no peer, table or writer lock held. A failed or ended source
publication is skipped explicitly, rather than replayed or awaited forever.
The actual replay owner relays the relevant family through `RelayStoredRoute`
and ordinary final admission. The pass flushes before releasing the hold, so
a later real withdrawal or replacement remains the last word on the socket.

API-originated, configured and default routes require their own ordinary
writer path within that shared pass. Retain `self` versus explicit next-hop
intent, rather than replaying only the last emitted address. Config-static
attribution must belong to a send, or every writer that reads the peer-wide
marker must be serialized. The earlier attribution-race requirement remains.
No new intent field or transport contract is selected by this proposal.

The worker must coalesce address/neighbor bursts without blocking either
watch worker. A capacity-one wake alone does not bound flaps during an active
pass. The coalescing interval and active-pass fairness rule still need design
approval. Bind work to the captured live session, cancel the worker/pass with
the reactor context, join it through the reactor wait group and discard
retired-session work. Do not add a parallel route inventory or per-UPDATE
kernel reads.

| Open design decision | Evidence or decision required before implementation |
|----------------------|----------------------------------------------------|
| Effective-change predicate | Detect condition gain/loss and known identity replacement without replaying for unrelated neighbors, table order or diagnostic-only changes |
| Family selector | `RelayReplay` and `replayRoutesCommand` accept destination, cursor and optional message-ID cut today. Their current final argument is not a family. Design an additive selector and old-caller/forked-store compatibility |
| API origin intent | Trace the complete origin, sent-store, grouping and replay consumers before selecting a `self`/explicit marker. `outboundResend` currently uses recorded output |
| Config/default attribution | Choose send-scoped attribution or complete serialization and retain the initial-sync/plugin-send overlap race proof |
| Coalescing and scheduling | Select the per-peer interval and behavior for changes during a pass, with burst, slow-peer and shutdown controls |
| Runtime failure reporting | Select an applicable existing doctor code or register a runtime check for absent store, timeout and failed refeed. No code/name is invented here |

Design admission remains **not approved** until the independent `ze-design-judge` verdict and Thomas's terminal approval (`./le spec design approve`) land.

### Resolved design decisions (2026-11-22)

Thomas resolved the open design choices on 2026-11-22 (`"flap dampening should be an option and off by default as since the RFC it was found to not help. 1A is fine. 2A. 3A."`, followed by `"One coalescing goroutine/timer per peer"` when reviewing the single-worker head-of-line delay), and source censuses completed the producer/consumer trace for every rail:

| Area | Decision | Source-verified mechanism |
|------|----------|---------------------------|
| Worker model | One long-lived coalescing goroutine and cooldown timer per `Peer` (`p.readvertiseWorker()` in `reactor_readvertise.go`), started in `Peer.StartWithContext` beside `go p.run()`, reading a capacity-one wake channel (`p.readvertiseWake`) and per-peer `p.clock` timer, cancelled on `p.ctx`, joined on `p.wg`, and skipping new passes once `p.stopping` or `r.stopping` is set | Owner decision (2026-11-22: `"One coalescing goroutine/timer per peer"`). Eliminates cross-peer head-of-line blocking when one peer hits the 2 s drain/applied-event budget while strictly obeying `ai/rules/goroutine-lifecycle.md` (long-lived per-peer channel worker, never goroutine-per-event) |
| Burst coalescing (1A) | Run the first effective change for an idle peer immediately (0 ms settle delay); enforce a per-peer cooldown (default 1 s, clock-injectable) after a pass with at most one queued trailing re-pass. Explicitly local burst coalescing, not RFC 2439 BGP route flap dampening; no new config leaf | Resolves Issue 3 without adding first-flip latency or an unasked config option; recorded in `docs/architecture/bgp/interface-event-reactions.md` `## Design decisions` |
| Effective-change predicate | Queue a peer from `refreshPeerLinkScopes` only when its effective RFC 2545 / RFC 4659 / third-party Link-Local answer changes: `peerOnLink` changes, own `nhMode`/`nhGlobalLL`/`nhVPNLinkLocal` changes, or any third-party global's `thirdPartyAnswer` changes (including gain, loss, or Link-Local move) | Ignores table reorder, unrelated neighbor entries, and watch-status-only changes |
| Learned-route rails (B1) | For ordinary selected destinations (`p.ordinarySelectedExport()`), add family-scoped `reofferSelectedFamily(peer, session, fam)` in `bgp-rib` (`rib_selected_policy.go`, `rib_selected_dispatch.go`), invoked via hidden internal command `request bgp rib reoffer-selected <peer> <session> <family>`, filtering `d.current` and `d.scoped` strictly to `key.family == fam` so `SelectRoutes` re-evaluates IPv6 next-hop scope without re-advertising unrelated families (such as `ipv4/unicast`). For explicit-role / forwarded destinations (`bgp-rs`, `bgp-rr`, unheld-role `bgp-adj-rib-in`), raise `beginPolicyReplay(session)` before taking per-source `receiveCut()`, run `drainResendSources` (skipping `errReceivePublicationFailed` and `errReceivePublicationStopped` at Debug), await `DrainEventsApplied` under one shared 2 s budget with no peer/reactor/writer lock held, relay via `RelayReplay` (`request bgp adj-rib-in replay-routes`) under `policyReplayOwner()`, and `flushPolicyReplay` before `endPolicyReplay()` | Prevents stale replay overtaking a live withdrawal on both general and native RS rails (B1), avoids IPv4 churn on ordinary selected peers, and covers ordinary selected peers where `bgp-adj-rib-in` is not loaded |
| Family selector (3A) | Extend `request bgp adj-rib-in replay-routes` (`replay_page.go`) and `ribevents.RelayReplay` with an additive optional trailing `family <afi/safi>` selector (validated via `family.LookupFamily`). When supplied, filter `walkReplayRoutes` to that family, sort by ascending `MsgID` (tie-breaking on sequence), and refuse on a forked manager (`!r.ingestTracked`) with an `opererr` naming in-process `bgp-adj-rib-in` as the fix | Keeps existing `bgp-rs`, `bgp-rr`, and `policy_reload` callers unchanged; avoids churning IPv4 tables on an IPv6 link-local flip |
| Direct API `self` intent & unbracketed refeed | Carry `NextHopSelf bool` as a typed scalar per send from `reactor_api_batch.go` / `peer_initial_sync.go` through `fwdItem` / `wireu.SentOrigin` -> `types.RawMessage.NextHopSelf` (no `sentMeta` map created or cloned in `reactor_notify.go`), read directly from `*types.RawMessage` in `rib_structured.go` on DirectBridge and formatted directly from `msg` at the JSON `format=full` boundary in `format/text_update.go` as `"route-meta":{"next-hop-self":true}` (skipped in the `msg.Meta` loop so policy cannot spoof it) -> `ribOutEntry.NextHopSelf` (separate bool so `show bgp rib sent` stays unchanged). Add internal unbracketed `request bgp rib refeed-local <peer> <family>` in `bgp-rib` that iterates `ribOut` entries with `LocalOrigin && !ConfigStatic` for both ordinary and legacy up peers (no BoRR/EoRR, so RFC 7313 §4 never purges forwarded/config routes) and updates `bgp.FormatAnnounceCommand` (`internal/component/bgp/format.go`) to emit `nhop set self` when `route.NextHop == "self"` (`parseWireNhopSection` in `update_wire.go` already parses it). On a refeed `self` announce whose `resolveNextHop` fails, withdraw the route if `adjOut` holds it | Obeys `docs/architecture/wire/messages.md` (`Internal sent-origin receipt`, zero map allocation on internal forwards), closes the gap where `outboundResend` skips ordinary destinations and emits BoRR/EoRR brackets, and fixes `FormatAnnounceCommand` dropping `"self"` on `update hex` |
| `sendingConfigStatic` race (Issue 5) | Delete `Peer.sendingConfigStatic` (`peer.go`) and its read in `reactor_notify.go`. Carry `configStatic bool` as a typed scalar per send into `session_write.go` (`s.sentConfigStatic` guarded by `s.writeMu` in `sendUpdateCounted`/`sendUpdateHeld`) -> `wireu.SentOrigin.ConfigStatic` -> `types.RawMessage.ConfigStatic` (zero map allocation in `reactor_notify.go`), read directly from `*types.RawMessage` in `rib_structured.go` and formatted directly from `msg` as `"config-static": true` in `format/text_update.go` (skipped in the `msg.Meta` loop). Re-send config static and default-originate routes per family under `staticMu` using resolved-next-hop tracking so a route that loses its next hop is withdrawn cleanly | Eliminates both concurrent-writer contamination and the permanent-stuck-`true` `Swap`/`Store` race between `sendInitialRoutes` and `deliverStaticRouteDelta` while honoring the zero-map internal receipt invariant |
| Failure diagnostics | Register one reactor-owned runtime doctor check `bgp-next-hop-readvertise` (`Phase: diagnostic.DoctorPhaseRuntime`) and code `doctor-bgp-next-hop-readvertise` (`diagnostic.Register` + `diagnostic.RegisterEventCode` + `diagnostic.RegisterDoctorCheck`), raising/clearing via `report.RaiseWarningFix`/`report.ClearWarning` and coded `Warn` logs with `"code"` and `"fix"`. Covers absent store, forked `bgp-adj-rib-in`, drain/barrier timeout, and failed refeed; clears on subsequent pass success or session reset | Satisfies `ai/rules/user-facing-errors.md` and `docs/architecture/doctor-and-health-checks.md` without colliding with `doctor-bgp-neighbor-table` or plugin-owned codes |
| Daemon `.ci` & interop (2A) | Keep `bgp-rfc2545-neighbor-condition-flip-frr` (real-kernel neighbor flip + FRR 10.7.1 RIB/FIB installation, AC-4) and add one fast Linux `.ci` daemon test `test/plugin/rfc2545-condition-flip-readvertise.ci` (`option=needs-linux:caps=net-admin`, pattern of `scoped-peer-prefix-refresh.ci`) proving real netlink address prefix loss and restoration (AC-1/AC-2) on a surviving TCP session | Satisfies B2 across both trigger classes (neighbor watch via FRR interop, address watch via fast `.ci`) |

### Test investigation (2026-11-15)

The RFC test campaign adds
`internal/component/bgp/reactor/rfc2545_neighbor_late_discovery_test.go` and
`internal/component/bgp/reactor/rfc2545_neighbor_transition_test.go`
without advancing the then-skeleton's approval gates. The full RFC 2545 text was
read for this investigation. Section 3 decides the next-hop field at each
advertisement; the trigger remains the owner decision recorded above. A
passing late-discovery test alone cannot prove every condition of Section 3
or complete `RFC2545-3-6`.

The actual callback is `neighborWatchWorker`'s `tableWatch.changed`, which
calls `refreshPeerLinkScopes`. That function replaces each live peer's
neighbor view and forwarding facts but starts no retained-route replay.
`RelayStoredRoute` does reconstruct retained bytes and call
`forwardUpdateCore`, so a separately requested replay can test the final
writer without pretending the watch scheduled it.

One prerequisite needs a separate verdict. `readAddressTable` in
`reactor_address_watch.go` is initialized to `network.InterfacePrefixes`
and its contract says the refresh reads it too, but the refresh calls
`network.InterfacePrefixes` directly. The existing fake address table
therefore cannot supply a deterministic common subnet to the neighbor
callback. `TestRFC2545NeighborWatchUsesAddressSnapshot` isolates that defect;
using the existing read seam at that call preserves the production reader,
signature and single-read fan-out. The main thread must observe this
prerequisite red before applying that correction, then distinguish it from
the expected missing-replay red.

The main thread observed that prerequisite red in its scoped campaign run:
`TestRFC2545NeighborWatchUsesAddressSnapshot` received the host's connected
prefixes instead of the modeled `2001:db8:1::/64`. The new package compiled;
the failure was the explicit common-subnet-input assertion. That verdict
does not establish the missing replay's socket behavior. An earlier fixture
assigned the modeled `LocalAddress` after listener startup. The scoped race
run `job-unit-pkg-334ed25d.log` exposed concurrent reads in `settingsSnapshot`
and `NewSession`. The current helpers set the modeled address before startup,
as the construction-only settings contract requires. Through the existing
`SetListenerFactory` seam, `lateNeighborListenerFactory` maps only that modeled
bind to a real `127.0.0.1:0` listener; other binds use the real factory unchanged.
The reactor still owns and closes each returned listener, and the protocol
sessions retain their original `net.Pipe` endpoints. An independent native
`ze-test-judge` approval for `lateNeighborLive` records that this fixture repair
preserves both rails, IPv6 negotiation, reachability, inputs and assertions.

After the existing address-read seam correction, the main thread observed
the snapshot prerequisite, explicit refreshed replay and queued-withdrawal
controls pass, including both live withdrawal rails. The first full
socket-trigger run did not reach its trigger assertion: its selecting-RIB
prerequisite correctly found no electable route. The fake interface view
had no matching connected path in the shared Loc-RIB.
`gatherPrefixCandidatesLocked` calls `candidateNextHopResolved`, which uses
`igpcost.ResolveProspective` against the Loc-RIB wired by bgp-rib's `Run`.
The fixture now installs only its own protocol's connected
`2001:db8:1::/64` path in that process-wide Loc-RIB after startup, checks
ordinary resolution before ingress, and removes that path after the router
stops. It preserves the selecting-RIB assertion and also reads the actual
Adj-RIB-In replay command to check the retained source, native NLRI,
next-hop bytes, admission epoch and message ID.

The corrected main-thread run passed the known-neighbor socket control on
both rails. All eight late-discovery cases then failed at the intended
assertion: no second advertisement reached the recipient after the neighbor
change, without new ingress. Each case had already passed connected
next-hop resolution, selecting-RIB and Adj-RIB-In retention, initial
global-only socket output, callback identity refresh and common-subnet
checks. The absence of a replay in `refreshPeerLinkScopes` is therefore an
observed trigger gap, separate from the earlier fixture defects. The
explicit replay and queued-withdrawal controls passed on their preceding
scoped run; the ordering integration of a future actual watch-triggered
pass still needs its own proof.

The final scoped race run, `job-rfc-boundaries-final-race-cfc4586c.log`, emitted
no data-race report. The known-neighbor control passed on both rails. All eight
late-discovery cases still failed at the missing second advertisement. The new
known-identity replacement and shared-subnet-loss probes each failed on both
rails at the same missing re-advertisement boundary. Before failing, they
proved initial paired socket output, retained source routes and refreshed
callback facts. Subnet loss changed the watched address prefix from `/64` to
`/128`, retained the known neighbor identity and independently resolved path,
and made the joint-subnet condition false.

The new callback-triggered concurrency probe also passed its initial output
and retention prerequisites on both rails, then stopped because the watch
dispatched no replay to the destination worker. It did not reach its later
source-withdrawal, store-removal or final-wire assertions. The earlier explicit
queued-replay control does not supply that missing callback integration.

| New unit | Boundary and positive control | Evidence still owed |
|----------|-------------------------------|---------------------|
| `TestRFC2545NeighborKnownAtReceiptReachesSocket` | Real IPv6 OPEN and UPDATE sockets, registered RIB and route-server plugins; the known third-party identity produces exactly global plus its Link-Local on both general and native RS rails | Main-thread baseline passed on both rails; producing-code break owed |
| `TestRFC2545LateNeighborDiscoveryReadvertisesRetainedRoute` | A real peer sends one native IPv6 route, both real stores retain it, and only the watched table changes from unknown, unresolved, ambiguous or unreadable to a unique valid binding. The callback publishes the new facts before the socket assertion; the test never calls the relay itself | All eight cases observed red at the absent new advertisement after their prerequisites passed; implementation, green baseline, producing-code break, daemon-level and peer-resolution interop proof remain owed |
| `TestRFC2545RefreshedFactsReachStoredReplaySocket` | The actual relay writer first sends the retained global alone, then sends the same immutable received route after facts gain the third-party Link-Local | Main-thread baseline passed; producing-code break owed; this is not proof of a watch trigger |
| `TestRFC2545QueuedNeighborReplayEndsWithdrawn` | After a successful real relay write, refreshed replay is held before the destination handler while the source withdraws on each live rail; the final native socket message must withdraw the route exactly once | Main-thread baseline passed on both rails; producing-code break and the owner's actual re-send hold/cut integration remain owed; this is not a substitute for the full callback-trigger ordering test |
| `TestRFC2545KnownNeighborReplacementReadvertisesRetainedRoute` | A retained route first reaches the recipient with a known third-party pair; the real neighbor watch replaces that identity without new ingress | Both rails observed red at the missing second advertisement after initial output, both stores and replacement facts passed; implementation, green baseline and producing-code break remain owed |
| `TestRFC2545SharedSubnetLossReadvertisesGlobalOnly` | The real address watch changes the local prefix from `/64` to `/128`; the route and known neighbor remain retained and resolved while the common-subnet condition becomes false | Both rails observed red at the missing global-only re-advertisement after the scope flip and controls passed; implementation, green baseline and producing-code break remain owed |
| `TestRFC2545WatchReplayConcurrentWithdrawalEndsWithdrawn` | A watch-scheduled replay must enter the controlled destination worker before a real source withdrawal is sent; both stores and the final socket history are then checked | Both rails observed red because no watch-triggered replay entered the worker; later withdrawal assertions were not reached. Trigger implementation, actual ordering proof, green baseline and producing-code break remain owed |

These units are deliberately untagged until they have a passing baseline and
a recorded producing-code break. No new discovery protocol, neighbor
solicitor, registered observer or readvertisement API is introduced. The
existing owner retains the burst, unrelated-neighbor, moved-Link-Local,
daemon-level watch and FRR/BIRD next-hop-resolution obligations.

### Daemon and FRR condition-flip probe

`bgp-rfc2545-neighbor-condition-flip-frr` adds the missing daemon-level
carrier through the existing Docker interop runner. It reuses the three-peer
topology of `bgp-rfc2545-third-party-linklocal-frr`, whose checker repeats its
announcement while polling and therefore proves the form of a fresh send,
not a late watch-triggered re-advertisement. That existing tagged checker
remains unchanged.

The new checker is
`internal/le/interoplab/bgp/check_rfc2545_neighbor_flip.go`. Its new scenario
starts bgp-rib with actual sent-update and state delivery. Before the decisive
mutation, the native sent-RIB command must contain exactly one subject route
for FRR, with its original global next hop. A failure of that prerequisite is
reported separately and proves no missing callback-triggered replay.

The third-party router first has one real Link-Local, then gains a second
actual address on its eth0. Neighbor Discovery must establish the resulting
ambiguous identity in Ze's kernel table before those verified mappings are
pinned for a stable test phase. Distinct setup prefixes prove both the
unique pair and the ambiguous global-only form. The subject is announced
exactly once in that second state. Removing the extra address and its
neighbor entry then changes the real kernel input back to a unique binding.
Only FRR queries run during the subject's transition wait: there is no new
subject send, reload, refresh request or direct callback invocation.

The retention assertion follows the actual `show bgp rib sent` producer,
not a separate checker-specific schema. `rib_pipeline.go` builds sent rows
from `ribOut` through `newOutboundSource` and `reconstructRoute`, then
`showRows` emits flat `peer`, `direction`, `family`, `prefix` and `next-hop`
fields. `family.Family.MarshalText` supplies `ipv6/unicast`; the CLI's JSON
record collapse first places those rows under `routes`. The command used
here is `ze cli --format json`: `client/main.go` `commandWithFormat` appends
`| json`, then `command/render_records.go` `renderDocument` applies that
operator after collapse. `command/pipe.go` `applyJSON` calls
`unwrapSingleKeyArray`, so the canonical final answer on this surface is a
bare row array, not the RPC envelope. The earlier source audit stopped at
RPC collapse and missed that last rendering step. The assertion now parses
only the actual bare-array shape; it does not accept both shapes. It requires
exactly one matching FRR destination, sent direction, family, subject and
global next hop. It establishes inventory presence, not a successful replay.

The source trace also checks the API route's local ownership: the attached
`send [ update ]` grants origin permission; `initBatchOwner` marks its write
originated; the final writer publishes a local sent ownership receipt; and
`handleSentStructured` stores that receipt with the sent route. Local sent
ownership passes `sentSourceState` without a received-source route. The RIB's
startup subscriptions include sent updates before API-ready, so the first
subject send is not intentionally outside its subscription window.

No interface configuration block is required for this topology's watcher.
The shipped composition includes `iface` and its Linux kernel backend;
`iface/register.go` installs the neighbor reader and watcher, which both
load the default backend when needed. Reactor startup starts the neighbor
watch independently of the interface monitor. The netlink subscription
includes existing entries and signals a known IPv6 entry's deletion, which
is the secondary neighbor change this probe makes. Kernel controls retain
the production binding-state classifier and device/MAC pairing rules, and
real Neighbor Discovery establishes the mappings before they are pinned.

A fresh control prefix runs after that wait even when the subject stays red.
Its pair distinguishes a missing refeed from missing current watched facts.
FRR's establishment count must remain unchanged, and a green subject must
also be installed through the remaining real Link-Local. The checker remains
untagged; the native run observed the intended subject-transition failure,
while a passing repair and its peer-installation proof remain owed. Its helper
tests separately refuse unresolved kernel inputs, wrong foreign routes,
missing sent retention and an observation loop that sends the subject again.

The five checker-helper tests passed in
`tmp/session/2026-11-14-210034/scratch/job-unit-pkg-fadc6091.log`. The first
daemon attempt,
`tmp/session/2026-11-14-210034/scratch/job-rfc2545-neighbor-condition-flip-frr-ddb045d4.log`,
did not reach the checker. Ze's own-system VM booted, dockerd answered and
the kernel check passed. Lab discovery then reported that the scenario had
no Go checker, with zero cases passed and zero failed; teardown reported no
leftovers and stopped the VM. This is a launcher-provenance failure, not a
protocol result.

That attempt invoked the named binary directly with no inherited
`ZE_LE_BUILD_NAME`. `dockerLabHostArgv` in
`internal/le/test/qemu/dockerlab.go` starts the checkout's `le` launcher for
the lab, and `dockerLabEnviron` preserves the caller's environment while
adding its Docker socket. The launcher therefore selected a shared
committed-code build, which lacked the new uncommitted registration. A
separate earlier `./le --name ... help` call rebuilt the named binary but
could not export its process environment into that later direct invocation.
The registry itself has the normal feature-owned insertion in
`register_rfc2545_neighbor_flip.go`; `checkers` merges that map when the
runner calls `Discover`, and the catalog defers the same lookup until use.

The corrected invocation must enter through `./le --name s-01a1db03` for
the actual job, so the launcher exports `ZE_LE_BUILD_NAME=s-01a1db03` to
the job, the Docker-lab driver and its nested launcher. The launcher is
the producer of this private value; it is not set manually. The receiving
guard is `cmd/ze/le_build_name.go` `refuseWrongBuildName`. No registration
change or product fix follows from the failed attempt.

The corrected named run in
`tmp/session/2026-11-14-210034/scratch/job-rfc2545-neighbor-condition-flip-frr-named-ddb045d4.log`
reached the checker and passed assertions 1–3: established session, actual
third-party identity and the unique-neighbor paired control. Assertion 4
stopped at the successful, silent address-add command because `Lab.Query`
rejects empty stdout. The retained kernel diagnostic shows the secondary
address was present on the actual router; the failure was the checker's
execution contract, before ambiguous-neighbor state, subject retention or
triggered re-advertisement was proven. Teardown reported no leftovers and
stopped the VM.

The checker now runs address and neighbor mutations through `Lab.Exec`,
requiring command completion with exit zero and propagating execution
failures. It then retains the same actual kernel-state assertions; no
output is fabricated and no command error is ignored. The added helper
test distinguishes silent success, a nonzero exit and an execution failure.

The subsequent native run,
`tmp/session/2026-11-14-210034/scratch/job-rfc2545-neighbor-flip-exec-fixed-ed3f0b43.log`,
passed assertions 1–6, including actual ambiguous Neighbor Discovery input,
both next-hop setup forms and the once-announced global-only subject at FRR.
Assertion 7 then stopped on the checker's wrong JSON-root assumption above.
Its now-superseded recording
`tmp/session/2026-11-14-210034/scratch/interop-recordings/bgp/bgp-rfc2545-neighbor-condition-flip-frr-LJJL3KANI6ZWSR3I4ITLACDYDN.jsonl`
contained 53 successful sent-RIB queries. Each was a three-row array containing
the two controls and exactly one row matching the subject's peer, direction,
family, prefix and global next hop. Thus this attempt is a checker-schema
failure, not evidence that retention failed. Teardown reported no leftovers
and stopped the VM. The unit regression now includes the exact native subject
row and three-row census, and refuses null, malformed, missing, wrong and
duplicate answers, including the unsupported RPC-envelope shape.

The corrected parser passed its focused race-enabled regression in
`tmp/session/2026-11-14-210034/scratch/job-rfc2545-retention-array-race-429624d0.log`,
including the native three-row control and all eighteen invalid answers.
Formatting and independent source review are complete. Another fresh named
daemon/FRR attempt then reached the intended protocol failure in
`tmp/session/2026-11-14-210034/scratch/job-rfc2545-neighbor-flip-array-fixed-ed3f0b43.log`.
Assertions 1–8 passed, including exact sent retention and the real kernel's
change from ambiguous to unique identity. Assertion 9 failed because the
once-announced subject stayed global-only through the observation window.
The later fresh-control and session checks passed before that error was
returned: FRR decoded the fresh route with the actual global/Link-Local pair,
and both neighbor reads showed Established with connectionsEstablished=1.

The native recording
`tmp/session/2026-11-14-210034/scratch/interop-recordings/bgp/bgp-rfc2545-neighbor-condition-flip-frr-EAJH6KKM6JYHKLODFPJUXQSQFF.jsonl`
contains exactly one subject announcement. Its 31 subject observations end
with a 16-byte global-only next hop; the unique setup and fresh post-change
controls each have a 32-byte pair with the same actual Link-Local. The
ambiguous setup control is global-only. This establishes missing unsolicited
re-advertisement through the daemon and FRR for this API-originated subject,
not a foreign decoder failure or a missing current neighbor fact. Assertion
12, successful peer installation through the Link-Local, remains unreached.
Teardown reported no leftover resources and stopped the VM.

Subsequent lint fixes preserve every protocol condition and assertion:
shared command constants carry the same strings, the retention helper now
names its invariant subject directly, and the wait diagnostic preserves both
error causes. Independent review confirmed those equivalences against the
actual recording. All seven final helper tests passed under the race
detector in
`tmp/session/2026-11-14-210034/scratch/job-rfc2545-final-helpers-race-cover-2dbad3af.log`.
Atomic counters measured both outcomes of `neighborFlipWaitFailure`; they
are not a package-wide or RFC-wide coverage claim. No interoperability rerun
is needed solely for these equivalent substitutions and error diagnostics.

The native lab currently declares FRR 10.7.1 in
`internal/le/interoplab/discover.go` as `FRRImage`. Source read for this probe:

| FRR source | Deciding behavior |
|------------|-------------------|
| [bgp_attr.c at frr-10.7.1](https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_attr.c), `bgp_mp_reach_parse` | Decodes both IPv6 next-hop forms and validates a second address as Link-Local |
| The same file, `attrhash_cmp` | Compares next-hop length and Link-Local value, so adding the second address is an attribute change even when the global address is unchanged |
| [bgp_route.c at frr-10.7.1](https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_route.c), `route_vty_out_detail` | Emits the detailed route's Link-Local next-hop entry when its received field has the global-and-Link-Local length; this is the foreign parser boundary the probe reads |

Temporary evidence stays in the existing campaign scratch location; no
copies or separate report were created. These logs are diagnostic evidence
for this open investigation, not conformance discrimination records.
The three superseded startup/schema logs and the earlier CLI recording were
removed after confirming they were untracked and not open. This recovered
293,720 content bytes; the final native failure, its recording and the exact
native row fixture preserve the evidence still needed.

| Evidence item | Purpose | Retention/status |
|---------------|---------|-------------------|
| `tmp/session/2026-11-14-210034/scratch/job-core-bgp-lifecycle-advertisement-bdcd2163.log` | Corrected known-neighbor positives and all eight missing-trigger socket reds; the file also carries other campaign findings | Remove when current tests plus the eventual repair's red/green evidence replace these diagnostics and the parent campaign no longer needs the shared log |
| `tmp/session/2026-11-14-210034/scratch/job-rfc2545-neighbor-lifecycle-79449ba8.log` | Passing address-snapshot, explicit replay and queued-withdrawal controls; its earlier retention-prerequisite failures are superseded by the corrected run | Remove when those controls are covered by the repair's retained evidence and the parent campaign no longer needs the shared log |
| `tmp/session/2026-11-14-210034/scratch/job-rfc-boundaries-final-race-cfc4586c.log` | Final race-enabled IPv6 controls and fourteen intended trigger reds after the construction-time fixture repair; the shared log also carries other campaign findings | Remove when the eventual repair's retained red/green and ordering evidence supersedes these diagnostics and the parent campaign no longer needs the shared log |
| `tmp/session/2026-11-14-210034/scratch/job-unit-pkg-fadc6091.log` | Passing five checker-helper tests, distinct from a live daemon result | Remove when the checker changes and a replacement targeted run supersedes it, or the campaign no longer needs this diagnostic |
| `tmp/session/2026-11-14-210034/scratch/job-rfc2545-neighbor-condition-flip-frr-ddb045d4.log` | Superseded launcher-provenance diagnosis above | Removed; the final run reaches the checker |
| `tmp/session/2026-11-14-210034/scratch/job-rfc2545-neighbor-condition-flip-frr-named-ddb045d4.log` | Superseded silent-mutation diagnosis above | Removed; the final run passes the kernel mutation controls |
| `tmp/session/2026-11-14-210034/scratch/job-rfc2545-neighbor-flip-exec-fixed-ed3f0b43.log` | Superseded retention-parser diagnosis above | Removed; the final run passes exact sent retention |
| `tmp/session/2026-11-14-210034/scratch/interop-recordings/bgp/bgp-rfc2545-neighbor-condition-flip-frr-LJJL3KANI6ZWSR3I4ITLACDYDN.jsonl` | Earlier native sent-RIB shape and census | Removed; the exact native row remains in the unit fixture and the final recording supersedes it |
| `tmp/session/2026-11-14-210034/scratch/job-rfc2545-neighbor-flip-array-fixed-ed3f0b43.log` and recording `EAJH6KKM6JYHKLODFPJUXQSQFF.jsonl` in the same recording directory | Intended assertion 9 failure with real kernel mutation, one subject send, fresh paired control, unchanged Established session and complete teardown | Retain until this repair's red/green observations supersede the unresolved protocol finding |
| `tmp/session/2026-11-14-210034/scratch/job-rfc2545-final-helpers-race-cover-2dbad3af.log` | Final seven helper passes, including strict retention and both error-wrapper outcomes | Retain through the source/evidence commit and campaign audit; the committed tests and result then replace the temporary console output |

An interface address event that flips RFC 2545 Section 3's shared-subnet
condition must re-advertise the prefixes that condition governs. Today it does
not, so when the condition goes FALSE a peer keeps a link-local next hop it can
no longer resolve on its own link, until the next update for that prefix
arrives.

`(*Reactor).refreshPeerLinkScopes`
(`internal/component/bgp/reactor/reactor_iface.go`) is the producer. On an
address or neighbor event it snapshots the peer list, reads `readAddressTable`,
derives masked connected prefixes and reads one neighbor view. For each changed
live scope it calls `peer.refreshForwardFactsIfLiveFrom(connected, neighbors)`.
It re-settles the snapshot and
deliberately re-advertises nothing. The false-to-true direction was first
recorded as harmless. Since 2026-10-12 the owner requires re-advertisement
in that direction too (AC-2), to agree
with `spec-bgp-link-local-only-next-hop` AC-17.

This is Thomas's ruling on B-16, given on 2026-08-08 after the closing review of
the RFC 2545 work in `ec3ad9c76`: "re-advertise the affected prefixes". He gave
it after BIRD and FRR were read at source, so it is a deliberate choice to
exceed both references.

**CORRECTED 2026-08-30: this is NOT an RFC violation.** RFC 2545 Section 3 binds
the act of advertising, not re-advertising, and `rfc/short/rfc2545.md` puts
usability on the receiver. The reading Ze took is correct and is not overturned.
What the ruling adds is that a flip is itself an event owing the peer a new
advertisement, taking the RFC 4271 Section 9.2 Update-Send reading. It stays
live because Thomas ruled for it, not because the wire is wrong.

**Both references are exceeded, knowingly.** BIRD never re-advertises on an
interface event and does not evaluate the condition dynamically at all. FRR
re-announces on an address change but guards both call sites with
`!IN6_IS_ADDR_LINKLOCAL` and freezes `shared_network` at establishment, likely
because it is an update-group key. The shape to copy is FRR's per-peer forced
re-announce scoped to that peer's channels, triggered the way BIRD triggers
`channel_request_feeding` from `channel_roa_out_changed`, which is its one
refeed trigger that is neither a config nor a protocol event.

**Cost note, corrected 2026-10-12:** the note recorded with the ruling ("Ze
has no Adj-RIB-Out refeed primitive today, and building one is the bulk of
the work") is stale, but the RIB's `outboundResend` alone is not the
primitive: it replays the next hop `ribOut` recorded as an explicit address
(`rib.go`, `sentFamilyNextHop`; `rib_replay.go`), so it never re-runs the
egress next-hop decision this spec's condition feeds (round-9 B1 on
`spec-bgp-link-local-only-next-hop`). The re-send that does re-run it has
three parts, one per route source, each scoped to one peer and one family
with no End-of-RIB: forwarded routes, sent or withheld, through the forward
rail over adj-rib-in's stored copy (`request bgp adj-rib-in replay` with a
new family argument and no `wait` argument, relayed by `RelayStoredRoute`,
which applies `applyFactsNextHop` and the egress pair trimming per
destination). Because adj-rib-in trails the live forward rail and every
relayed route passes the forwarding fence as part of an initial update,
that relay runs under one per-peer re-send hold on live forwards for the
whole pass, with a cut taken after the hold is up from the sources'
received-UPDATE IDs (`receiveCut`), never the shared message counter, and
the relay starts only after each source's publication reaches its own cut
and adj-rib-in's process has applied every prior event (a per-source loop,
`drainResendSources`, that skips a source whose old session's publication
failed or stopped short of its cut, then `DrainEventsApplied`, round-13
I1 on that spec), both waits
sharing one 2 s budget and holding no lock, so a route replaced or
withdrawn during the pass ends as the live rail left it; the routes go in
ascending message-ID order, and the pass promises the per-source current
stored route, not the live rail's choice among sources for UPDATEs
received concurrently (a Known Limitation); a forked adj-rib-in refuses
the family replay, so it gets no forwarded re-send and the worker logs a
Warn naming the fix (that spec's R-9) (round-10 B1, I1 and round-11 B1,
I1, I2, N1 on `spec-bgp-link-local-only-next-hop`). API routes go through
`outboundResend` restricted to entries with `ribOutEntry.LocalOrigin` set
and `ribOutEntry.ConfigStatic` clear (round-10 I2),
replaying a `next-hop self` entry as `self` from a new `ribOutEntry` marker
and an explicit next hop as itself; and config and default-originate
routes through a `staticMu` sibling of `deliverStaticRouteDelta`
(`reactor/peer_static_wire.go`).
`plan/release/fixes/spec-bgp-link-local-only-next-hop.md` (Key Design Decisions,
"Re-advertise", and AC-17) writes that re-send for an interface address
event, run on a long-lived reactor worker rather than on the EventBus
goroutine. This spec reuses that path, and adds
only its own trigger: an RFC 2545 Section 3 condition flip.

**Resolved, 2026-10-12 (round 12 on `spec-bgp-link-local-only-next-hop`,
c166bb9f72):** the re-send shape is B, the in-place re-send above, with no
session reset (that spec's Owner decisions, "Round 12: re-send shape"). It
follows from two standing owner decisions: "match bird and frr behaviour",
following FRR where they differ, and re-advertise on change, decided from
RFC 4271 Section 9.2 and FRR's per-peer re-announce (round-7 I3 there).
FRR re-announces in place (`bgp_announce_route` per AFI/SAFI from
`bgp_interface_address_add` and `bgp_interface_address_delete`, "we do not
want the peering to bounce") and BIRD does neither, so FRR decides. Option
A, a Cease with subcode 6, is not taken. The B-16 ruling's "re-advertise
the affected prefixes" holds unchanged here, and AC-1 and AC-2 re-advertise
in place on that path.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/bgp/interface-event-reactions.md` - actual address/neighbor callbacks
  → Constraint: retain one immutable snapshot per table refresh and existing listener behavior; schedule replay outside the watch workers.
- [ ] `docs/architecture/update-building.md` - next-hop normalization and final ownership
  → Constraint: relay retained input through ordinary writers and source/session admission; do not replay only the last emitted `self` address.
- [ ] `docs/architecture/bgp/structural-forwarding.md` - both retained-route rails
  → Constraint: retain general and native RS paths and cached input. Its earlier claim that Ze never learns another router's Link-Local is stale; correct it with implementation.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2545.md` - Section 3's shared-subnet condition and where it puts usability
  → Constraint: Section 3 binds the act of advertising, not re-advertising
- [ ] `rfc/short/rfc4271.md` - Section 9.2 Update-Send, the reading the ruling takes
  → Constraint: preserve useful routes and exact later withdrawals without a session reset; owner AC-1/AC-2 require in-place re-advertisement.

**Key insights:** (minimal context to resume after compaction)
- The facts and final writer already produce a fresh paired next hop. The missing producer is callback scheduling of retained replay.
- A safe refeed raises the destination hold before source receive cuts, publication waits and the applied-event barrier.
- Family scope and API/config/default origin semantics are prerequisites, not optional test reductions.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/reactor/reactor_iface.go` - `refreshPeerLinkScopes` reads the existing `readAddressTable` seam, derives connected prefixes, reads one neighbor view and refreshes changed live scopes. It schedules no replay. Both actual watch callbacks reach it.
- [ ] `internal/component/bgp/reactor/policy_reload.go` - `reconcileExportFinal` contains reusable restoration and relay machinery, but its store drain precedes its hold and captures no per-source cuts.
- [ ] `internal/component/bgp/reactor/receive_publication.go` - `receiveCut`, `state` and `wait` describe a source session's publication, including ended/failed state.
- [ ] `internal/core/bgp/ribevents/replay.go` and `internal/component/bgp/plugins/adj_rib_in/replay_page.go` - `RelayReplay`/`replayRoutesCommand` accept destination, cursor and optional message-ID cut; no family selector exists.
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` and `rib_replay.go` - `outboundResend` replays recorded output with BoRR/EoRR. It is not the owner-required local-origin-only refeed without markers.
- [ ] `rfc/full/rfc2545.txt` - Section 3, the shared-subnet condition itself

**Behavior to preserve:** (unless the user explicitly said to change it)
- Preserve the owner requirement for one re-advertisement for a burst with one
  resulting effective answer (`spec-bgp-link-local-only-next-hop` AC-17).
  Current code skips equal masked connected prefixes and equal neighbor views;
  it has no re-advertisement worker. The earlier capacity-one-wake proposal
  does not alone bound changes during a pass. Its interval and active-pass
  behavior remain explicit design choices above.
- Preserve both forwarding rails, immutable retained source bytes, received
  Path Identifiers, source policy epochs, useful output and exact withdrawal
  ownership. Do not change admission or next-hop wire-form rules to clear a
  missing-trigger red.

At this source checkpoint, watch/publication/replay files inspected were clean
against HEAD. `reactor.go` carried foreign protocol-capture changes to
`sessionCaptures` and cleanup prose. Recheck ownership before editing and
coordinate that file's worker hunks. Auto's next-hop admission/encoding files
are outside this repair. A clean diff is not a permanent file-ownership claim.

**Behavior to change:** (only what the user asked for)
- a true-to-false flip re-advertises the prefixes the condition governs
- a false-to-true flip re-advertises them too, with the link-local now
  appended: the next hop changes, so the route is a new one in the peer's
  Adj-RIB-Out and RFC 4271 Section 9.2 owes the UPDATE ("All newly installed
  routes and all newly unfeasible routes for which there is no replacement
  route SHALL be advertised to its peers by means of an UPDATE message").
  This replaces the earlier "harmless, nothing sent" reading so the two specs
  agree (`spec-bgp-link-local-only-next-hop` AC-17, amended 2026-10-12)

## Existing Mechanisms

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Shared table refresh | `internal/component/bgp/reactor/reactor_iface.go` `(*Reactor).refreshPeerLinkScopes`, `(*Peer).refreshForwardFactsIfLiveFrom` | Reads `readAddressTable` and one neighbor view, updates each live peer's immutable link-scope and forwarding facts snapshot, and schedules no replay | Extends: compares old and new effective RFC 2545 / RFC 4659 / third-party Link-Local answers and signals each affected live peer's coalescing worker (`p.scheduleReadvertise`) without blocking either watch goroutine |
| Ordinary selected export reconciliation | `internal/component/bgp/reactor/policy_final_kinds.go` `ordinarySelectedExport`, `reconcileSelectedExport`; `internal/component/bgp/plugins/rib/rib_selected_policy.go` `reofferExportPolicy`; `internal/component/bgp/plugins/rib/rib_selected_dispatch.go` `selectedDispatch.run`; `internal/component/bgp/reactor/reactor_api_selected.go` `SelectRoutes` | Re-offers `bgp-rib`'s selected learned and origin routes to an ordinary destination peer across all families, re-running `resolveNextHop` and `announceFactsFor` (`linkLocalNextHopFor`) | Extends: adds family-scoped `reofferSelectedFamily(peer, session, fam)` in `bgp-rib` (invoked via hidden `request bgp rib reoffer-selected <peer> <session> <family>`), filtering `d.current` and `d.scoped` strictly to `key.family == fam` so IPv6 condition flips do not re-advertise `ipv4/unicast` routes on ordinary selected peers |
| Live replay hold and flush | `internal/component/bgp/reactor/policy_reload.go` `beginPolicyReplay`, `flushPolicyReplay`, `endPolicyReplay`, `policyReplayOwner` | Holds live forwards to a destination behind a counted `policyReplayHolds` gate and flushes queued live writes when the replay finishes | Extends: extracts the hold-before-cut / flush-before-release sequence so the readvertise pass holds live forwards before capturing per-source receive cuts and draining stores |
| Source publication barrier | `internal/component/bgp/reactor/receive_publication.go` `(*Peer).receiveCut`, `receivePublicationCut.state`, `receivePublicationCut.wait` | Captures a source session's latest received UPDATE message ID and waits until its publication finishes or fails/stops | Extends/reuses: captures cuts from all relevant sources after the destination hold is raised and drains them via `drainResendSources`, skipping ended or failed publications explicitly |
| Applied-event barrier | `internal/component/plugin/process/delivery.go` `(*Process).DrainEventsApplied` | Waits until a plugin process has applied all events queued before the barrier call | Reuses: called on the `bgp-adj-rib-in` process after `drainResendSources`, sharing one 2 s budget with no peer, reactor or writer lock held |
| Bounded owner-relayed Adj-RIB-In pages | `internal/core/bgp/ribevents/replay.go` `RelayReplay`; `internal/component/bgp/plugins/adj_rib_in/replay_page.go` `replayRoutesCommand`, `buildReplayPage` | Replays stored Adj-RIB-In pages to a destination through `RelayStoredRoute` using `<target> <from-index> [<max-msg-id>]` | Extends: adds additive optional trailing `family <afi/safi>` selector (validated via `family.LookupFamily`), filters `walkReplayRoutes` to that family in ascending `MsgID` order, and refuses on forked `bgp-adj-rib-in` (`!r.ingestTracked`) with an `opererr` naming in-process `bgp-adj-rib-in` |
| Normal retained-route writer | `internal/component/bgp/reactor/reactor_api_relay.go` `RelayStoredRoute` | Replays stored raw wire bytes through egress policy, `applyFactsNextHop`, pair trimming and final admission | Reuses: unchanged writer for forwarded routes on explicit-role destinations (`bgp-rs`, `bgp-rr`, unheld-role `bgp-adj-rib-in`) |
| Local sent-route storage and wire formatting | `internal/component/bgp/plugins/rib/ribout_entry.go` `ribOutEntry`, `rib_replay.go` `formatCursorCommands`, `internal/component/bgp/format.go` `FormatAnnounceCommand`, `internal/component/bgp/plugins/cmd/update/update_wire.go` `parseWireNhopSection` | Tracks sent routes in `ribOut` with `LocalOrigin`/`ConfigStatic`; `outboundResend` skips ordinary destinations and brackets legacy ones with BoRR/EoRR; `FormatAnnounceCommand` drops non-IP `"self"` on the `update hex` branch while `parseWireNhopSection` already accepts `nhop set self` | Extends: records `NextHopSelf bool` on `ribOutEntry` via send-scoped metadata, emits `nhop set self` in `FormatAnnounceCommand`, and adds hidden unbracketed `request bgp rib refeed-local` for `LocalOrigin && !ConfigStatic` across both ordinary and legacy destinations |
| Send-scoped wire write path | `internal/component/bgp/reactor/session_write.go` `sendUpdateCounted`, `sendUpdateHeld`, `writeUpdateBody`; `reactor_notify.go` `notifyMessageReceiver`; `peer_static_wire.go` `deliverStaticRouteDelta` | Serializes wire writes under `s.writeMu` and records typed `wireu.SentOrigin` scalars (copied onto `types.RawMessage` with no metadata map), while `Peer.sendingConfigStatic` is a racy peer-wide atomic read asynchronously in `notifyMessageReceiver` | Replaces `Peer.sendingConfigStatic` with typed scalar `ConfigStatic` and `NextHopSelf` fields carried under `s.writeMu` into `wireu.SentOrigin` -> `types.RawMessage` (formatted into JSON `route-meta` only at the `format/text_update.go` external boundary), and adds a per-family static/default-originate re-send helper under `staticMu` |
| Reactor runtime doctor check | `internal/component/bgp/reactor/source_session_diagnostic.go` (`bgp-source-receipt-issuer`, `Phase: diagnostic.DoctorPhaseRuntime`, `report.RaiseErrorFix`); `internal/core/report/report.go` (`report.RaiseWarningFix`, `report.ClearWarning`) | Registers a reactor-owned runtime doctor check and diagnostic code, and `internal/core/report` provides warning raise/clear helpers | Extends by pattern: adds `bgp-next-hop-readvertise` (`doctor-bgp-next-hop-readvertise`, `DoctorPhaseRuntime`) using `report.RaiseWarningFix` / `report.ClearWarning` for absent store, forked `bgp-adj-rib-in`, drain/barrier timeout and failed refeed |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "Every plugin, ours included, runs either internal (a goroutine in the daemon) or external (a forked subprocess that talks only over TLS and RPC)." | agrees | Reactor reaches `bgp-adj-rib-in` and `bgp-rib` only via registered commands and DirectBridge/JSON events; `NextHopSelf` travels over both DirectBridge and JSON `route-meta`; forked `bgp-adj-rib-in` refuses explicitly with a fix |
| `ai/doors/bgp-engine.md` | Boundaries | the door covering reactor, wireu, types, format, ribevents | "Never name a plugin package from the reactor." | agrees | `reactor_readvertise.go` dispatches `ribevents.RelayReplay`, `request bgp rib reoffer-selected`, and `request bgp rib refeed-local` through registered command dispatch, importing zero plugin packages |
| `ai/doors/bgp-plugins.md` | Boundaries | the door covering `adj_rib_in` and `rib` | "A plugin never imports another plugin's implementation" | agrees | `adj_rib_in` and `rib` remain isolated; `request bgp rib refeed-local` and `request bgp rib reoffer-selected` are declared in both `registry.Registration.Commands` and `sdk.Registration.Commands` via `commandDecls()` |
| `ai/doors/le-tooling.md` | Boundaries | the door covering interoplab and `test/` | "A compiled Go fixture returns an error when an assertion fails" | agrees | `check_rfc2545_neighbor_flip.go` returns explicit errors on assertion failure, and `test/plugin/rfc2545-condition-flip-readvertise.ci` carries `option=needs-linux:caps=net-admin` |
| `docs/architecture/bgp/interface-event-reactions.md` | The decisions | `// Design:` of `reactor_iface.go`, `rfc2545_neighbor_transition_test.go`, modified by spec | "**A neighbor change sets the form of the advertisements made after it, and re-advertises nothing.** A route already sent with the global address alone is not sent again when the table later names the link-local. That re-advertisement is not built." | departs | Owner rulings (B-16 2026-08-08, AC-2 2026-10-12, 2026-11-22) require in-place re-advertisement on address and neighbor condition flips; this spec builds it and updates this section in the same change |
| `docs/architecture/bgp/interface-event-reactions.md` | Design decisions | decision of 2026-11-22 | "Interface and neighbor change re-advertisement runs the first flip immediately, then coalesces bursts behind a per-peer cooldown with at most one queued trailing re-pass; BGP route flap dampening (RFC 2439) is a separate concept and, if ever implemented, must be an option off by default ("flap dampening should be an option and off by default as since the RFC it was found to not help. 1A is fine.")" | agrees | `reactor_readvertise.go` implements immediate first-flip execution (0 ms settle delay), per-peer 1 s clock-injectable cooldown, and at most one queued trailing re-pass, adding no RFC 2439 dampening |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Both `request bgp adj-rib-in replay-routes` and `request bgp rib refeed-local` are hidden internal engine-to-plugin `request` RPCs, not operator `show` commands taking scope or presentation arguments |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Neither hidden internal RPC overrides any pipe operator or changes any existing command's data format |
| `docs/architecture/api/commands.md` | Dispatch keys are YANG paths | anchored to reactor, `adj_rib_in`, `rib`, modified by spec | "A dispatch key has one holder." | agrees | `request bgp rib refeed-local` registers a unique hidden command key owned solely by `bgp-rib`, and `request bgp adj-rib-in replay-routes` remains owned solely by `bgp-adj-rib-in` |
| `docs/architecture/api/architecture.md` | RIB Ownership | `// Design:` of `ribevents/replay.go`, `adj_rib_in/replay_page.go` | "The engine is a minimal BGP speaker." | agrees | The reactor owns no second route table or RIB copy; it asks `bgp-rib` and `bgp-adj-rib-in` via registered commands to re-offer/replay stored routes |
| `docs/architecture/api/json-format.md` | Field naming | `// Design:` of `format/text_update.go`, modified by spec | "JSON keys are lowercase kebab-case, never camelCase and never snake_case." | agrees | The additive sent UPDATE `route-meta` key is lowercase kebab-case `"next-hop-self"`, matching `"config-static"` |
| `docs/architecture/api/process-protocol.md` | Overview | anchored to reactor, `rawmessage.go`, `ribevents`, `adj_rib_in`, `rib` | "The protocol is the same for all invocation modes (internal goroutine, external subprocess)." | agrees | `refeed-local`, `replay-routes ... family <afi/safi>`, and `"next-hop-self"` work over both DirectBridge and JSON RPC, and forked `bgp-adj-rib-in` refuses explicitly where MsgID tracking is unavailable |
| `docs/architecture/api/update-syntax.md` | Next-Hop (required for announce) | anchored to reactor files | "the `nhop set` form belongs to the hex and b64 encodings." | agrees | `FormatAnnounceCommand` emits `nhop set self` on its `update hex` branch, matching `parseWireNhopSection` which already parses `nhop set self` |
| `docs/architecture/behavior/peer-lifecycle.md` | TL;DR | anchored to reactor files | "Events carry `connection-id` from the exact captured Session through OPEN, UP, UPDATE/EOR and DOWN; a delayed callback never borrows a replacement connection's identity." | agrees | Every readvertise pass captures the live `*Session` and discards stale work for retired or replaced sessions |
| `docs/architecture/bgp/egress-attribute-rules.md` | How provenance is established | anchored to `reactor_notify.go` | "Both forward rails relay an UPDATE another speaker sent, after import policy has had a chance to rewrite it." | agrees | Forwarded re-advertisements enter `RelayStoredRoute` so egress attribute rules and pair trimming re-run against current link-scope facts |
| `docs/architecture/bgp/on-demand-origination.md` | The decisions | anchored to `peer_initial_sync.go` | "Every other rail that reaches a peer works the same way." | agrees | Re-advertised API, static, and default-originate routes pass through the ordinary egress build and export policy gates, and `"next-hop-self"` stays outside the `tag.` prefix |
| `docs/architecture/bgp/replay-cursor.md` | What it buys | `// Design:` of `rib_replay.go`, modified by spec | "Config-static advertisements are retained in Adj-RIB-Out with their origin flag." | agrees | Send-scoped `configStatic` keeps `ConfigStatic` accurate under concurrent sends, and `refeed-local` selects `LocalOrigin && !ConfigStatic` while static routes re-send under `staticMu` |
| `docs/architecture/bgp/structural-forwarding.md` | Forwarded routes and next-hop self | `// Design:` of `rfc2545_neighbor_late_discovery_test.go`, anchored to reactor | "Another router's Global receives that router's Link-Local from the captured neighbor table under the same condition. If the table cannot identify a usable Link-Local, Ze sends the Global alone and warns." | agrees | Condition-flip re-advertisement re-runs the exact same `linkScope.thirdPartyLinkLocal` and `applyEgressNextHopScope` decision against the refreshed snapshot |
| `docs/architecture/core-design.md` | 6. Plugin API Communication | `// Design:` of reactor files and `rawmessage.go` | "Engine is stateless for routes. It forwards wire bytes to plugins and caches for zero-copy forwarding." | agrees | Preserves `WireUpdate` transport, attribute pools, registration-based dispatch, zero engine-owned route store, and zero plugin imports from reactor |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | anchored to `rib.go`, modified by spec | "Each check runs once per answer, through one route." | agrees | `bgp-next-hop-readvertise` (`doctor-bgp-next-hop-readvertise`) registers once in the reactor at `DoctorPhaseRuntime`, reading live reactor state without applying mutations |
| `docs/architecture/forward-congestion-pool.md` | The replay fence | anchored to reactor files | "The fence belongs to one peer. Other destinations read their own fence, so a replay on one peer never delays forwarding to another." | agrees | `beginPolicyReplay` holds live forwards only for the single peer undergoing a condition-flip pass, never stalling unrelated peers |
| `docs/architecture/memory/lifetime-contracts.md` | A. WireUpdate — eager copy | anchored to `reactor_notify.go`, `wire_update.go`, `rawmessage.go` | "A release path that both walks a payload and returns its buffer must walk first" | agrees | Adding two value booleans (`ConfigStatic`, `NextHopSelf`) on `SentOrigin`/`RawMessage` borrows no payload bytes and preserves all buffer lifetime boundaries |
| `docs/architecture/plugin/plugin-system.md` | Registration fields | anchored to reactor files | "Registration happens only during package initialization." | agrees | `bgp-next-hop-readvertise` registers in reactor `init()`, and `request bgp rib refeed-local` registers through `bgp-rib`'s `commandDecls()` at `init()` and Stage 1 |
| `docs/architecture/plugin/rib-storage-design.md` | Received source-policy provenance | `// Design:` of `format.go`, `adj_rib_in/rib.go`, `rib/*.go`, modified by spec | "Capturing a receipt transports identity; it does not itself acquire engine authorization to send the route." | agrees | `NextHopSelf` is stored on `ribOutEntry` inside `bgp-rib` and family-filtered replay runs inside `bgp-adj-rib-in` under ordinary engine admission; the engine holds no RIB |
| `docs/architecture/pool-architecture.md` | Data Flow | `// Design:` of `ribout_entry.go` | "Wire bytes flow from engine to API" | agrees | `ribOutEntry.NextHopSelf` is a plain boolean beside existing `LocalOrigin`/`ConfigStatic` flags and does not alter `attrpool.Handle` pooling or compaction |
| `docs/architecture/rib-transition.md` | Ordinary selected-route advertisement | anchored to `reactor.go`, `rib.go` | "Explicit route-server and reflector destinations retain their role writer. Ordinary refresh reoffers the current selected snapshot and completes after final writer receipts" | agrees | Ordinary selected destinations use family-scoped `request bgp rib reoffer-selected` (`reofferSelectedFamily`) to re-offer selected IPv6 routes without IPv4 churn, while explicit RS/RR destinations use `RelayReplay` through `RelayStoredRoute` |
| `docs/architecture/route-selection.md` | Received NEXT_HOP | anchored to reactor and `rib` | "The receive path uses an immutable interface snapshot captured at connection setup and replaced on interface address events. It performs no kernel lookup per UPDATE." | agrees | `refreshPeerLinkScopes` continues to publish one immutable snapshot per watch event and performs zero per-UPDATE kernel lookups |
| `docs/architecture/testing/ci-format.md` | An unparseable test file fails; it never hides or vanishes | anchored to `peer.go` | "It is never dropped, and it never aborts the rest of the directory." | agrees | New `test/plugin/rfc2545-condition-flip-readvertise.ci` follows standard `.ci` directives with `option=needs-linux:caps=net-admin` |
| `docs/architecture/testing/interop.md` | The suites, one per protocol area | `// Design:` of `check_rfc2545_neighbor_flip.go`, anchored to `session_write.go` | "A scenario directory is NAMED and carries no numeric prefix." | agrees | Reuses the existing named scenario `bgp-rfc2545-neighbor-condition-flip-frr` and its Go checker without moving assertions into declarative files |
| `docs/architecture/update-building.md` | Path 2: Forward Path (Route Reflection) | anchored to reactor, `wire_update.go`, `rib`, modified by spec | "Peers with identical negotiated capabilities share one ContextID, so route reflection between same-capability clients builds once and sends the result N times." | agrees | Condition-flip re-advertisement reuses `RelayStoredRoute` and ordinary build writers without changing `ContextID` sharing or UPDATE packing |
| `docs/architecture/api/capability-contract.md` | Architecture | anchored to `reactor.go`, `peer.go` | "" | silent | Covers OPEN capability negotiation and cache/GR/refresh contracts; this spec changes no capability negotiation or cache control commands |
| `docs/architecture/api/ipc_protocol.md` | Overview | anchored to `reactor.go`, `peer.go` | "" | silent | Covers low-level IPC line framing and RPC transport envelopes; additive optional arguments and JSON `route-meta` keys leave IPC framing untouched |
| `docs/architecture/api/text-coverage.md` | Message Type Coverage | anchored to `format/text_update.go` | "" | silent | Covers text-format event parser coverage; `"next-hop-self"` is added only to JSON `format=full` `route-meta` and DirectBridge `RawMessage` |
| `docs/architecture/api/text-parser.md` | Current Parser | anchored to `format/text_update.go` | "" | silent | Covers the text-format event scanner and parser; no text-format event grammar changes |
| `docs/architecture/behavior/fsm-established.md` | TL;DR | anchored to `session_write.go`, `rib.go` | "" | silent | Covers Established-state HoldTimer, keepalive, and prefix-limit handling; in-place re-advertisement adds no FSM event or state transition |
| `docs/architecture/behavior/fsm-open-sent.md` | TL;DR | anchored to `peer.go` | "" | silent | Covers OpenSent validation and timers; this spec runs only on Established sessions |
| `docs/architecture/behavior/fsm.md` | TL;DR (Read This First) | anchored to reactor files | "" | silent | Covers BGP FSM states and transitions; in-place re-advertisement never resets or transitions the FSM |
| `docs/architecture/behavior/signals.md` | Supported Signals | anchored to `reactor.go` | "" | silent | Covers OS signal handling (`SIGTERM`, `SIGHUP`, `SIGALRM`, `SIGUSR1`); worker shutdown uses reactor context cancellation |
| `docs/architecture/buffer-architecture.md` | Executive Summary | anchored to `session_write.go`, `wire_update.go`, `text_update.go` | "" | silent | Covers zero-copy message buffers and attribute iterators; no buffer layout or iterator changes |
| `docs/architecture/config/apply-ordering.md` | The requirement | anchored to `reactor_iface.go`, `reactor.go` | "" | silent | Covers startup/reload binding order between interface addresses and BGP listeners; established-session route re-advertisement does not change listener ordering |
| `docs/architecture/config/environment.md` | Overview | anchored to `reactor.go` | "" | silent | Covers `ze.*` environment variables; this spec registers no environment variable |
| `docs/architecture/config/syntax.md` | Overview | anchored to `reactor.go`, `peer.go` | "" | silent | Covers YANG configuration syntax; this spec adds no config keyword or leaf |
| `docs/architecture/congestion-industry.md` | Universal Rule | anchored to `reactor.go`, `session_write.go` | "" | silent | Covers socket options, keepalive write priority, and RFC 9687 send hold timers; unchanged |
| `docs/architecture/decisions/001-pull-model-metrics.md` | Decision | anchored to `rib.go` | "" | silent | Covers pull-model Prometheus metrics; this spec adds no metrics |
| `docs/architecture/edge-cases/extended-message.md` | TL;DR | anchored to `session_write.go` | "" | silent | Covers RFC 8654 65535-octet message sizing; unchanged |
| `docs/architecture/encoding-context.md` | Overview | anchored to `peer.go`, `wire_update.go` | "" | silent | Covers per-peer `EncodingContext` and `ContextRegistry`; unchanged |
| `docs/architecture/exabgp-bridge.md` | The bridge translates in two directions | `// Design:` of `peer_static_restore.go` | "" | silent | Covers ExaBGP script translation; `peer_static_restore.go` only switches its internal config-static attribution carrier, leaving ExaBGP translation untouched |
| `docs/architecture/meta/role.md` | Keys Set | anchored to `reactor_notify.go`, `peer.go` | "" | silent | Covers RFC 9234 BGP role route metadata keys; unaffected by `"next-hop-self"` or `"config-static"` |
| `docs/architecture/mrt.md` | Packages | anchored to `reactor_notify.go` | "" | silent | Covers MRT dump streams and wire encoding/decoding; unchanged |
| `docs/architecture/overview.md` | 1. What is Ze? | anchored to `reactor.go` | "" | silent | High-level system overview; component placement and responsibilities are unchanged |
| `docs/architecture/rib/forward-handle.md` | The decisions | anchored to `rib.go`, `rib_structured.go` | "" | silent | Covers the RIB forward-handle fast path for received routes; unaffected by `ribOutEntry.NextHopSelf` or `refeed-local` |
| `docs/architecture/wire/capabilities.md` | TL;DR (Read This First) | anchored to reactor files | "" | silent | Covers OPEN capability TLV byte layout; unchanged |
| `docs/architecture/wire/messages.md` | Internal sent-origin receipt | `// Design:` of `wire_update.go`, anchored to `session_write.go`, modified by spec | "Internal forwarding creates no generic metadata map for this receipt; external event serializers format the typed fields only at their boundary." | agrees | `ConfigStatic` and `NextHopSelf` are added as plain scalar booleans on `wireu.SentOrigin` and `types.RawMessage`, with zero `sentMeta` map creation in `reactor_notify.go`, formatted into JSON `route-meta` only at the `format/text_update.go` serializer boundary |
| `docs/architecture/wire/mp-nlri-ordering.md` | Key Insight | anchored to `session_write.go`, `wire_update.go` | "" | silent | Covers MP_REACH_NLRI attribute ordering on the wire; re-advertised routes use existing encoders |
| `docs/architecture/wire/nlri-bgpls.md` | Wire Format Overview | anchored to `session_write.go` | "" | silent | Covers BGP-LS SAFI 71/72 NLRI TLVs; unaffected |
| `docs/architecture/wire/rfc7606-relay-shape.md` | The requirement | anchored to `reactor_api_batch.go` | "" | silent | Covers RFC 7606 malformed-UPDATE relay shaping; unchanged |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | This spec adds no config block, `ze:backend` annotation, or kernel/VPP feature declaration and does not alter data-plane commit validation |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | This spec adds no config schema or data-plane mismatch gate and leaves commit, reload, and validate verdicts unchanged |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | This spec is scoped to BGP RFC 2545 next-hop re-advertisement and touches neither VPP firewall verification nor firewall YANG annotations |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | This spec introduces no data-plane selector, backend registration, or `ze:backend` annotation, and its Linux `.ci` test uses the default `kernel` interface backend |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | This spec adds no session-level next-hop config knob and routes every re-advertised prefix through the ordinary egress writers (`SelectRoutes`, `RelayStoredRoute`, `refeed-local`, and static/default writers) that run the peer's existing export filter chain |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | This spec adds no policy config list or authoring surface and leaves `policychain.Sets` next-hop authoring forms unchanged |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | This spec does not alter `policychain.Chain`, `pkg/plugin/rpc.FilterInsertion`, or export-chain compilation; re-advertised routes execute the existing compiled export chain |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Forwarded re-advertisements enter `RelayStoredRoute`, which preserves export-chain last-write-wins folding and raw-override provenance unchanged |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | This spec leaves import/export policy validation and `policychain.Sets.NextHop` handling unchanged while re-evaluating IPv6/MP_REACH link-local scope on egress |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | `refeed-local` preserves explicit originator-chosen next hops while re-evaluating `next-hop self` entries, and re-advertised static and API routes still pass through ordinary egress writers and explicit export-chain actions |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | This spec adds no automatic filter-chain member and does not alter the implied-filter seam or `\| implied` display |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | This spec adds no startup refusal; degraded readvertise passes (absent store, forked `bgp-adj-rib-in`, barrier timeout, failed refeed) leave Ze running and surface via `bgp-next-hop-readvertise` (`doctor-bgp-next-hop-readvertise`) and a coded `Warn` log with its fix |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | This spec does not touch VPP/kernel startup mismatch handling or config/reload/validate refusal |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | This spec adds no implied config line and leaves `\| implied` formatting untouched |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | `bgp-rib` and `bgp-adj-rib-in` declare their internal commands (`refeed-local`, `reoffer-selected`, `replay-routes`) via standard registration without altering bus participant naming or defaults |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Condition-flip re-advertisement honors existing bus routing and `filterPermittedPeers` without changing `send > to` semantics |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | This spec does not modify `receive > from` compilation or enforcement |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | This spec leaves bus allow-list enforcement untouched and introduces no deny lists |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | This spec adds no bus filter syntax or config validation and preserves single-filter compilation |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | This spec adds no bus-level exclusive-role arbitration; ordinary vs. explicit-role dispatch follows the peer's own export kind (`ordinarySelectedExport` / `policyReplayOwner`) |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | This spec adds no hidden bus relationship or competing display |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | This spec adds no pipe operator and leaves existing pipe semantics unchanged; its hidden internal RPCs return standard `shapeMap` status envelopes |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | `request bgp rib refeed-local`, `request bgp rib reoffer-selected`, and `request bgp adj-rib-in replay-routes` execute through the registered command dispatcher and compiled authorization profile with no undeclared operation |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | This spec adds no lossy or drop-on-full event queue policy and relies on lossless event delivery with `DrainEventsApplied` before replay starts |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | This spec leaves offline config parsing and query-mode external plugin validation unchanged, and keeps `bgp-rib`/`bgp-adj-rib-in` command declarations pure values in `commandDecls()` |
| `docs/architecture/testing/interop.md` | Design decisions | decision of 2026-11-23 | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | `bgp-rfc2545-neighbor-condition-flip-frr` runs in the existing BGP interop suite without modifying `Suite.KernelCapabilities` or requiring unrelated kernel features |
| `docs/architecture/update-building.md` | Design decisions | decision of 2026-11-23 | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | This spec re-advertises IPv6 unicast and IPv6 MPLS-VPN families on RFC 2545/4659 condition flips and does not change FlowSpec MP_REACH next-hop emission or `flowspec-redirect-next-hop` |
| `docs/architecture/exabgp-bridge.md` | Design decisions | decision of 2026-11-23 | "An ExaBGP script is an ordinary bus endpoint that runs the wrapper; the bridge config section and the internal bridge mode do not exist" | agrees | `peer_static_restore.go` only replaces `sendingConfigStatic` with send-scoped `configStatic: true` and adds no bridge config section or ExaBGP runner mode |
| `docs/architecture/exabgp-bridge.md` | Design decisions | decision of 2026-11-26 | "ExaBGP wrapper endpoints request `base64` (`plugin.FormatBase64`) UPDATE delivery at Stage 5 `ready` using Ze's positional counted wire grammar (`upd <flags> <local-as> <remote-as> <conn-id> <n>:<peer> <n>:<local> <n>:<router-id> <n>:<extra-addpath> <n>:<b64-body>`, zero key names, `0:` when absent) over pooled `WriteBatchFrame` `MuxConn` batching while keeping low-frequency control/session events as JSON; `ze exabgp plugin` renders hot IPv4/IPv6 unicast UPDATEs directly into a `[]byte` buffer from wire bytes and falls back to the wire-backed `Event` renderer for non-unicast families and error cases" | agrees | This spec adds scalar booleans (`ConfigStatic`, `NextHopSelf`) on `wireu.SentOrigin`/`types.RawMessage` and additive `"next-hop-self"` in JSON `format=full` `route-meta` only; `plugin.FormatBase64`, the positional `upd` wire grammar, `WriteBatchFrame` batching, and `ze exabgp plugin` rendering remain unchanged |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An interface address added/removed/prefix-changed event (`addressWatchWorker` in `reactor_address_watch.go`) or a neighbor table change (`neighborWatchWorker` in `reactor_iface.go`), delivered to `(*Reactor).refreshPeerLinkScopes`.
- Both watch workers deliver kernel table refresh notifications without holding `r.mu`.

### Transformation Path
1. `refreshPeerLinkScopes` reads `readAddressTable` and one neighbor view once for all peers, updates each live peer's immutable link-scope and forwarding facts snapshot, and compares old vs. new effective RFC 2545 / RFC 4659 / third-party Link-Local answers.
2. Only peers whose effective answer changed (`peerOnLink`, own `nhMode`/`nhGlobalLL`/`nhVPNLinkLocal`, or any tracked third-party global's `thirdPartyAnswer` gain/loss/move) signal their own per-peer coalescing worker (`p.scheduleReadvertise(session)` via nonblocking send on `p.readvertiseWake` in `reactor_readvertise.go`). Table reorder, unrelated neighbors, and watch-status-only changes enqueue nothing.
3. Burst coalescing (1A) per peer: each `Peer` runs one long-lived `p.readvertiseWorker()` goroutine started in `Peer.StartWithContext` (joined on `p.wg`, cancelled on `p.ctx`). If the peer is idle and outside its per-peer cooldown (default 1 s, via `p.clock`), the worker runs its pass immediately (0 ms settle delay). Changes arriving during an active pass or while the peer's cooldown timer is armed set a single trailing flag so at most one trailing re-pass runs once that peer's timer fires. A slow peer hitting the 2 s barrier budget never delays any other peer.
4. The per-peer worker captures the destination's live session (discarding work for retired sessions or once `p.stopping` / `r.stopping` is set) and runs the family-scoped pass for IPv6 families (`ipv6/unicast`, `ipv6/mpls-vpn` as negotiated):
   - **Ordinary selected destinations (`p.ordinarySelectedExport()`):** invokes hidden `request bgp rib reoffer-selected <peer> <session> <family>` (`reofferSelectedFamily` in `rib_selected_policy.go` / `rib_selected_dispatch.go`), which verifies the exact captured destination session token and queues only keys with `key.family == fam` from `d.current` and `d.scoped` through `SelectRoutes`, re-evaluating IPv6 next-hop scope natively with zero `ipv4/unicast` churn.
   - **Explicit-role / forwarded destinations (`!p.ordinarySelectedExport()`):** raises `beginPolicyReplay(session)` first, captures each relevant source peer's `receiveCut()`, runs `drainResendSources` (skipping `errReceivePublicationFailed` and `errReceivePublicationStopped` at Debug), awaits `DrainEventsApplied` on the `bgp-adj-rib-in` process under one shared 2 s budget with no peer/reactor/writer lock held, relays the family via `ribevents.RelayReplay` (`request bgp adj-rib-in replay-routes ... family <afi/safi>`) under `policyReplayOwner()`, and calls `flushPolicyReplay` before `endPolicyReplay()`.
   - **Direct API routes (`LocalOrigin && !ConfigStatic`):** invokes `request bgp rib refeed-local <peer> <family>` (unbracketed, no BoRR/EoRR, no EoR), replaying `NextHopSelf` entries as `nhop set self` (withdrawing if `resolveNextHop` now fails while `adjOut` holds the route) and explicit next hops as themselves.
   - **Configured static and default-originate routes:** re-sends affected families under `staticMu` using send-scoped `configStatic: true` and resolved-next-hop tracking (withdrawing a previously sent static route if its next hop is no longer resolvable).
5. On absent store, forked `bgp-adj-rib-in`, drain/barrier timeout, or failed refeed, the worker raises a runtime warning on `bgp-next-hop-readvertise` (`doctor-bgp-next-hop-readvertise`) and logs a coded `Warn` with `"code"` and `"fix"`; a subsequent clean pass or session reset clears the warning.
6. Peer stop or reactor shutdown cancels `p.ctx`, skips any new pass once `p.stopping` or `r.stopping` is set, stops the per-peer timer, and joins the worker on `p.wg`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `iface` kernel backend ↔ BGP reactor | Registered `network.InterfacePrefixes`, `network.WatchAddresses`, `network.ReadNeighborTable`, `network.WatchNeighbors` callbacks into `refreshPeerLinkScopes` | Existing watch callbacks and fact refresh observed; readvertise worker scheduling owed |
| Reactor ↔ `bgp-adj-rib-in` (in-process or forked) | Text command `request bgp adj-rib-in replay-routes <target> <from-index> [<max-msg-id>] [family <afi/safi>]` via `ribevents.RelayReplay`; `Process.DrainEventsApplied` on the plugin process | Additive family selector and forked-manager refusal owed |
| Reactor ↔ `bgp-rib` (in-process or forked) | DirectBridge `StructuredEvent` (`types.RawMessage.NextHopSelf`, `types.RawMessage.ConfigStatic`, zero `Meta` map allocation) and JSON `format=full` `route-meta` (`"next-hop-self": true`, `"config-static": true`) formatted at `format/text_update.go`; hidden internal commands `request bgp rib reoffer-selected <peer> <session> <family>` and `request bgp rib refeed-local <peer> <family>`; `bgp.FormatAnnounceCommand` emitting `update hex ... nhop set self` parsed by `update_wire.go` `parseWireNhopSection` | Typed scalar propagation, `reoffer-selected` / `refeed-local` handlers, and `update hex` `nhop set self` branch owed |
| Reactor ↔ operator diagnostics | Runtime doctor check `bgp-next-hop-readvertise` (`doctor-bgp-next-hop-readvertise`, `DoctorPhaseRuntime`), `report.RaiseWarningFix` / `report.ClearWarning`, coded `Warn` logs | New registration and failure-path tests owed |

### Changed-Wire Census

BGP protocol wire format (RFC 4271 / RFC 2545 / RFC 4659 / RFC 7313) does **not** change: emitted UPDATEs use the existing 16-octet global-only and 32-octet global+link-local `MP_REACH_NLRI` next-hop encodings, with no new attributes or refresh markers. Five internal RPC / plugin-event text/JSON wire surfaces are extended additively; zero `pkg/` Go types change.

**Search evidence:** Searched `internal/`, `pkg/`, `cmd/`, `test/`, and `docs/` for `"replay-routes"`, `RelayReplay`, `replayRoutesCommand`, `sendingConfigStatic`, `config-static`, `RouteMeta`, `route-meta`, `nhop set`, `FormatAnnounceCommand`, `parseWireNhopSection`, `outboundResend`, `refeed-local`, and `reoffer-selected`. Positive controls verified: `internal/component/bgp/plugins/rs/server_handlers.go` (`RelayReplay`), `internal/component/bgp/plugins/rr/rr.go` (`RelayReplay`), `internal/component/bgp/reactor/policy_reload.go` (`RelayReplay`), `internal/component/bgp/format/text_update.go` (`"config-static"` and `"source-local"` JSON writers), `internal/component/bgp/plugins/rib/rib.go` (`"config-static"` JSON reader), `internal/component/bgp/plugins/rib/rib_structured.go` (`msg.SourceLocal` DirectBridge reader), `internal/component/bgp/plugins/cmd/update/update_wire.go` (`parseWireNhopSection` already accepting `"self"`). Zero Python, shell, or third-party script callers invoke `replay-routes`, `reoffer-selected`, or `refeed-local`.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Internal command `request bgp adj-rib-in replay-routes` | writer | `internal/core/bgp/ribevents/replay.go` `RelayReplay` (called by `rs/server_handlers.go`, `rr/rr.go`, `reactor/policy_reload.go`, and new `reactor/reactor_readvertise.go`) | Grep found all 3 existing callers (`rs:637,833`, `rr:1043,1054`, `policy_reload:157`), zero shell/Python callers | Additive optional trailing `family <afi/safi>` argument; existing callers pass empty family and emit the exact legacy token sequence |
| Internal command `request bgp adj-rib-in replay-routes` | reader | `internal/component/bgp/plugins/adj_rib_in/replay_page.go` `replayRoutesCommand`; declared in `adj_rib_in/rib.go` `commandDecls` | `replay_page.go:27-55` currently parses `args[0..2]` and ignores `args[3:]`; forked plugins set `ingestTracked=false` at `rib.go:434` | Change: parse optional trailing `family <afi/safi>` validated via `family.LookupFamily`, filter `walkReplayRoutes` in ascending `MsgID` order, refuse `!r.ingestTracked` with `opererr` naming in-process `bgp-adj-rib-in` |
| Internal command `request bgp rib reoffer-selected` | writer | `internal/component/bgp/reactor/reactor_readvertise.go` `reofferSelectedFamily` | New hidden internal command; grep confirmed zero prior occurrences of `reoffer-selected` | New additive hidden RPC (`<peer> <session> <family>`); scopes ordinary selected reoffer to one family so IPv4 is never churned on IPv6 flips |
| Internal command `request bgp rib reoffer-selected` | reader | `internal/component/bgp/plugins/rib/rib_commands.go` `reofferSelectedCommand` -> `rib_selected_policy.go` `reofferSelectedFamily`; declared `Hidden: true, Shape: shapeMap` in `rib/rib.go` `commandDecls` | Follows exact hidden-RPC pattern of `request bgp rib recovery` (`rib.go:1871`); filters `d.current` and `d.scoped` by `key.family == fam` for the exact captured `session` | Change: re-offers only the requested family for the exact captured ordinary destination session and awaits settlement |
| Internal command `request bgp rib refeed-local` | writer | `internal/component/bgp/reactor/reactor_readvertise.go` `refeedLocalOrigin` | New hidden internal command; grep confirmed zero prior occurrences of `refeed-local` | New additive hidden RPC (`<peer> <family>`); no external caller affected |
| Internal command `request bgp rib refeed-local` | reader | `internal/component/bgp/plugins/rib/rib_commands.go` `refeedLocalCommand`; declared `Hidden: true, Shape: shapeMap` in `rib/rib.go` `commandDecls` | Follows exact hidden-RPC pattern of `request bgp rib recovery` (`rib.go:1871`) and `request bgp rib flowspec-gate` (`rib.go:1874`) | Change: replay `ribOut` entries with `LocalOrigin && !ConfigStatic` for ordinary and legacy up peers, unbracketed (no BoRR/EoRR, no EoR), preserving `NextHopSelf` |
| Sent UPDATE event receipt (`types.RawMessage` & JSON `format=full` `route-meta`) | writer | `internal/component/bgp/reactor/session_write.go` `writeUpdateBody` (`wireu.SentOrigin`), `reactor_notify.go` `notifyMessageReceiver` (`types.RawMessage` scalars only, zero `Meta` map allocation), `internal/component/bgp/format/text_update.go` `FormatSentUpdate` / `writeRouteMeta` | `wire_update.go:348` (`SentOrigin`), `reactor_notify.go:657` (`SourceLocal` scalar copy), `text_update.go:880-944` formats `"source-local"` directly from `msg` at the JSON boundary and skips reserved keys in `msg.Meta` | Additive scalar booleans `ConfigStatic` and `NextHopSelf` on `wireu.SentOrigin` & `types.RawMessage`; `format/text_update.go` formats `"config-static":true` and `"next-hop-self":true` directly from `msg` (omitted when false, skipped in `msg.Meta` loop so policy cannot spoof them) |
| Sent UPDATE event receipt (`types.RawMessage` & JSON `format=full` `route-meta`) | reader | `internal/component/bgp/plugins/rib/rib_structured.go` `handleSentStructured` (in-process DirectBridge reads `msg.ConfigStatic` and `msg.NextHopSelf` directly from `*types.RawMessage`), `rib/rib.go` `handleSentUpdate` (JSON/forked reads `event.RouteMeta`), and any third-party plugin reading `route-meta` JSON | `rib_structured.go:545+` reads `*types.RawMessage` directly; `rib.go:1120+` reads `event.RouteMeta["config-static"]`; third-party plugins ignore unknown JSON keys in `route-meta` | Change `bgp-rib` readers to populate `ribOutEntry.ConfigStatic` and `ribOutEntry.NextHopSelf` directly from `msg` (DirectBridge) and `event.RouteMeta` (JSON); `show bgp rib sent` stays unchanged; third-party readers preserved via additive optional key |
| Internal command `send bgp ... update hex` `nhop set` clause | writer | `internal/component/bgp/format.go` `FormatAnnounceCommand` (called by `rib/rib_replay.go` `formatRawGroupCommands`) | `format.go:27-29` currently emits `nhop set <hex>` only when `netip.ParseAddr(route.NextHop)` succeeds, silently dropping `"self"` | Change: emit `nhop set self` when `route.NextHop == "self"` on the `update hex` branch; explicit IP hex unchanged |
| Internal command `send bgp ... update hex` `nhop set` clause | reader | `internal/component/bgp/plugins/cmd/update/update_wire.go` `parseWireNhopSection` | `update_wire.go:212-215` ALREADY handles `nhop set self` and sets `NextHopSelf = true` | Preserve unchanged; verified by existing parser and new round-trip unit test |

### Integration Points
- `peer.refreshForwardFactsIfLiveFrom` (`peer_forward_facts.go`) and `refreshPeerLinkScopes` (`reactor_iface.go`)
- `reofferSelectedFamily` (`rib_selected_policy.go`, `rib_selected_dispatch.go`) via hidden `request bgp rib reoffer-selected <peer> <session> <family>` for ordinary selected destinations (`p.ordinarySelectedExport()`)
- `beginPolicyReplay`, `flushPolicyReplay`, `endPolicyReplay`, `policyReplayOwner` (`policy_reload.go`) and `receiveCut`/`wait` (`receive_publication.go`) and `DrainEventsApplied` (`plugin/process/delivery.go`) for explicit-role destinations
- `sendUpdateCounted` / `sendUpdateHeld` / `writeUpdateBody` (`session_write.go`) for send-scoped `configStatic` and `nextHopSelf` attribution
- `diagnostic.Register`, `diagnostic.RegisterEventCode`, `diagnostic.RegisterDoctorCheck` (`readvertise_diagnostic.go`) for `bgp-next-hop-readvertise` (`doctor-bgp-next-hop-readvertise`)

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes (design) | Re-advertisements flow through `request bgp rib reoffer-selected` (`SelectRoutes`), `RelayReplay` -> `RelayStoredRoute`, `request bgp rib refeed-local` -> `send bgp ... update hex`, and `staticMu` static/default writers; never raw socket writes |
| No unintended coupling (components stay isolated) | Yes (design) | Reactor reaches `bgp-adj-rib-in` and `bgp-rib` strictly via registered commands (`ribevents.RelayReplay`, `request bgp rib reoffer-selected`, `request bgp rib refeed-local`) and DirectBridge/JSON events; no plugin package is imported by reactor |
| No duplicated functionality (extends existing, does not recreate) | Yes (design) | Reuses `policyReplayHolds`, `receiveCut`, `DrainEventsApplied`, `selectedDispatch`, `RelayReplay`, `FormatAnnounceCommand`, and `parseWireNhopSection`; adds no second route store |
| Zero-copy preserved where applicable (refs, not copies) | Yes (design) | Forwarded routes reuse cached/retained wire bytes through `RelayStoredRoute`; no per-UPDATE kernel reads or hot-path heap allocations added (`SentOrigin` booleans are value fields) |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes (design) | `request bgp rib refeed-local` registers in `bgp-rib`'s `commandDecls()` and dispatch table; `bgp-next-hop-readvertise` registers via `diagnostic.Register` / `RegisterEventCode` / `RegisterDoctorCheck` in reactor `init()`; family validation uses `family.LookupFamily` |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes (design) | Searched `internal/component/command/`, `internal/component/plugin/`, `internal/component/bgp/plugins/rib/protocol_test.go`, `internal/core/diagnostic/`: hidden `request bgp rib refeed-local` and `bgp-next-hop-readvertise` derive from `commandDecls()` and the `diagnostic` registry with zero central switch/list edits |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Existing `policyReplayHolds` / `beginPolicyReplay` / `flushPolicyReplay` / `endPolicyReplay` machinery can be reused with hold-before-cut ordering without regressing policy reload | `policy_reload.go:231-242` uses a counted hold (`policyReplayHolds`) and flushes queued writes before release | Policy reload or live forwarding regresses | Existing `policy_reload` tests plus `TestRFC2545WatchReplayConcurrentWithdrawalEndsWithdrawn` under `-race` | Validated by source read; runtime test owed |
| A-2 | Carrying `NextHopSelf` via `wireu.SentOrigin` -> `types.RawMessage` / JSON `route-meta` -> `ribOutEntry.NextHopSelf` preserves `self` intent on both internal and forked `bgp-rib` without altering `pkg/` or `show bgp rib sent` | `pkg/plugin/rpc/bridge.go:1154-1155` already carries `RawMessage` and `Meta`; `rib_structured.go:545+` and `rib.go:1120+` read both; `parseWireNhopSection` (`update_wire.go:212-215`) already accepts `nhop set self` | API `self` routes replay stale explicit IP or break forked RIB | Unit tests on both internal DirectBridge and JSON `format=full` paths plus `show bgp rib sent` assertion | Validated by source read; runtime test owed |
| A-3 | One long-lived coalescing goroutine and cooldown timer per `Peer` (immediate first pass + 1 s per-peer cooldown via `p.clock` + max 1 trailing re-pass, joined on `p.wg`) bounds bursts per peer and prevents a slow peer from delaying other peers | Thomas's 2026-11-22 rulings (`"1A is fine"` and `"One coalescing goroutine/timer per peer"`), `ai/rules/goroutine-lifecycle.md` | Flapping link storms wire, leaks a timer/goroutine on peer removal, or blocks shutdown | Controlled fake-clock burst, active-pass flap, independent-peer slow-barrier, and peer-stop/shutdown tests | Validated by owner decision; runtime test owed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A refeed fires an update storm on a burst or active-pass flap | Repeated passes for one final answer or unbounded queue growth | Effective-answer comparison plus 1A burst coalescing (immediate first pass, 1 s per-peer cooldown, at most 1 queued trailing re-pass) |
| R-2 | A per-peer refeed changes sharing or replays an unaffected family | IPv4 churn on an IPv6 link-local flip or wrong-peer output | Family-scoped `replay-routes ... family <afi/safi>`, `reoffer-selected`, `refeed-local <peer> <family>`, and per-family static resend; unaffected-family/peer unit tests |
| R-3 | Replay overtakes a live withdrawal/replacement (B1) | Final socket contains a resurrected route | Raise `beginPolicyReplay` before `receiveCut`, `drainResendSources`, and `DrainEventsApplied`; `flushPolicyReplay` before `endPolicyReplay`; proven by `TestRFC2545WatchReplayConcurrentWithdrawalEndsWithdrawn` |
| R-4 | Config-static attribution contaminates another writer or sticks permanently `true` (Issue 5) | Plugin/forwarded route is tagged `config-static` and dropped from `ribOut` | Delete `Peer.sendingConfigStatic` and carry `configStatic` per send under `s.writeMu`; proven by `-race -count=20` overlap test of initial sync, readvertise static resend, and concurrent API/forwarded send |
| R-5 | Ended source or retired destination leaves work pending or writes to wrong session | Unreachable cut, leaked timer/goroutine, or stale session reaches a writer | Skip `errReceivePublicationFailed`/`errReceivePublicationStopped` in `drainResendSources`; bind pass to captured live `*Session`; stop per-peer timer, cancel on `p.ctx`, and join on `p.wg` |
| R-6 | New selector or `route-meta` key breaks existing or forked callers | Parser error on legacy `replay-routes` or silent empty replay on forked `bgp-adj-rib-in` | Additive optional trailing `family <afi/safi>` (zero change to 2- and 3-arg callers); explicit `opererr` refusal on `!r.ingestTracked` naming in-process `bgp-adj-rib-in` |
| R-7 | Missing store, forked store, or timeout fails silently | Retained route stays stale with no operator signal | Raise runtime doctor check `bgp-next-hop-readvertise` (`doctor-bgp-next-hop-readvertise`) + coded `Warn` with `"code"` and `"fix"`; clear on recovery or session reset |
| R-8 | Absence tests pass after useful output is dropped | Known/fresh controls stop appearing | Preserve all 14 existing transition/concurrency probes, known-neighbor controls, and FRR interop assertions 1–12 |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Unneeded IPv6 route re-announcements on unrelated events, stale or unresolvable IPv6 next hops after condition flips, or mis-attributed `config-static` / `next-hop-self` sent metadata |
| How is it reverted? | Disabling enqueue in `refreshPeerLinkScopes` disables watch-triggered re-advertisement cleanly while preserving watched-facts refresh, send-scoped attribution, and additive RPCs; full revert follows owner-controlled Git rules |
| Who else touches this path? | `spec-bgp-link-local-only-next-hop`, `spec-connected-static-reach-the-locrib`, `policy_reload.go`, `bgp-rib`, `bgp-adj-rib-in` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Actual address watch removes the shared-subnet condition | → | `refreshPeerLinkScopes` → `readvertiseWorker` → ordered refeed | `TestRFC2545SharedSubnetLossReadvertisesGlobalOnly` |
| Actual neighbor watch discovers/replaces an identity | → | `refreshPeerLinkScopes` → `readvertiseWorker` → retained replay | `TestRFC2545LateNeighborDiscoveryReadvertisesRetainedRoute`, `TestRFC2545KnownNeighborReplacementReadvertisesRetainedRoute` |
| Source withdraws while callback replay is queued | → | `refreshPeerLinkScopes` → `beginPolicyReplay` / `receiveCut` / `drainResendSources` / `DrainEventsApplied` / `flushPolicyReplay` | `TestRFC2545WatchReplayConcurrentWithdrawalEndsWithdrawn` |
| Real kernel netlink socket adds/removes shared IPv6 prefix on live daemon | → | Netlink address watch → `refreshPeerLinkScopes` → `readvertiseWorker` → wire UPDATE on surviving TCP session | `test/plugin/rfc2545-condition-flip-readvertise.ci` |
| Real kernel changes ambiguous neighbor to unique on live daemon | → | Netlink neighbor watch → `refreshPeerLinkScopes` → `readvertiseWorker` → `request bgp rib refeed-local` → FRR 10.7.1 | `bgp-rfc2545-neighbor-condition-flip-frr` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The RFC 2545 Section 3 shared-subnet condition goes from true to false for an Established IPv6 peer | Affected IPv6 prefixes are re-advertised in place with no session reset, carrying a 16-octet global next hop the peer can resolve, across ordinary selected, forwarded (`bgp-adj-rib-in`), direct API (`refeed-local`), and configured/default rails | Non-IPv6 families (for example `ipv4/unicast`) and peers whose effective answer did not change emit zero re-advertisements because the RFC 2545 condition does not govern them |
| AC-2 | The shared-subnet condition goes from false to true for an Established IPv6 peer | Affected IPv6 prefixes are re-advertised in place in one pass with no session reset, carrying a 32-octet global + link-local next hop computed fresh by egress (`applyFactsNextHop`/pair trimming for forwarded routes, `linkLocalNextHopFor` for API `self` and explicit routes, and static/default writers) rather than replaying a recorded address | Direct API routes originated with an explicit third-party global next hop still evaluate `thirdPartyLinkLocal` rather than local link-local, because their original explicit intent is preserved |
| AC-3 | A burst of address or neighbor watch events arrives for a peer (including during an active pass or cooldown) | The first effective change for an idle peer runs immediately (0 ms settle delay); subsequent changes during the active pass or per-peer cooldown (default 1 s, clock-injectable) coalesce into at most one trailing re-pass once cooldown expires; unrelated neighbors, table reorders, and watch-status-only changes trigger zero passes | At most one trailing re-pass after an active pass or cooldown window is permitted when an effective change arrived mid-pass, so the final kernel state is never lost |
| AC-4 (`RFC2545-3-6`, gap half) | A route was sent with a third-party global next hop alone and the neighbor table later names a unique Link-Local (or replaces it with a moved Link-Local) | Affected IPv6 prefixes are re-advertised in place with the 32-octet global + third-party Link-Local pair; `bgp-rfc2545-neighbor-condition-flip-frr` passes assertions 1–12 including FRR installation via the Link-Local; `rfc/short/rfc2545.md` loses `{partial}` after discrimination record (`./le rfc record`) | Neighbor changes for addresses not used by any cached third-party next hop emit zero re-advertisements because no advertised next hop changed |
| AC-5 (B1 hold-before-cut ordering) | A live source withdraws or replaces a route while a watch-triggered replay pass is running for an explicit-role destination | The pass raises `beginPolicyReplay` before taking per-source `receiveCut()`, completes `drainResendSources` (skipping ended/failed sources) and `DrainEventsApplied` within the shared 2 s budget, relays the family in ascending `MsgID` order, and calls `flushPolicyReplay` before `endPolicyReplay`, so the live withdrawal or replacement is always the final wire message | Without ADD-PATH, if multiple sources concurrently announce the same prefix, ascending `MsgID` order guarantees each source's latest stored route rather than cross-source concurrent-arrival arbitration (documented in Known Limitations) |
| AC-6 (API `self` intent & family-scoped replay) | A direct API route announced with `next-hop self` and another with an explicit global next hop are stored in `ribOut`, and a condition flip triggers `request bgp rib refeed-local <peer> <family>` (and `request bgp adj-rib-in replay-routes ... family <afi/safi>` on explicit-role peers) | `ribOutEntry.NextHopSelf` preserves `self` intent across both DirectBridge and JSON `format=full` transports without changing `show bgp rib sent` display; `refeed-local` emits unbracketed updates (no BoRR/EoRR, no EoR) with `nhop set self` re-evaluating current link-local scope (or withdrawing if unresolvable), while explicit next hops replay their explicit global; `replay-routes` filters to the named family and preserves legacy 2/3-arg callers | Forked `bgp-adj-rib-in` (`!r.ingestTracked`) stores `MsgID == 0` and refuses family-scoped replay explicitly with an `opererr` naming in-process `bgp-adj-rib-in` as the fix, because cross-source ordering cannot be proven without ingested message IDs |
| AC-7 (Issue 5 send-scoped attribution) | A readvertise config-static/default resend overlaps initial sync and a concurrent API or forwarded send to the same peer under `go test -race -count=20` | Zero data races; only configured static writes carry `"config-static": true`, never contaminating concurrent API or forwarded writes and never leaving static attribution stuck enabled after overlap | None; `Peer.sendingConfigStatic` is deleted completely so no peer-wide attribution flag survives |
| AC-8 (Runtime diagnostics & B2 daemon `.ci`) | (a) A readvertise pass encounters an absent store, forked `bgp-adj-rib-in`, barrier timeout, or failed refeed; (b) a real Linux daemon loses and regains a shared IPv6 prefix via netlink | (a) Reactor raises runtime doctor check `bgp-next-hop-readvertise` (`doctor-bgp-next-hop-readvertise`, `DoctorPhaseRuntime`) and logs a coded `Warn` with `"code"` and `"fix"`, clearing on subsequent pass success or session reset; (b) `test/plugin/rfc2545-condition-flip-readvertise.ci` (`option=needs-linux:caps=net-admin`) observes both global-only and global+link-local re-advertisements on the surviving TCP session | None on the happy path; runtime warnings survive only while the degraded condition persists until cleared by a successful pass or session reset |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Removes or restores a shared IPv6 subnet prefix on an interface while a BGP session remains Established | Netlink address watch → `refreshPeerLinkScopes` → `readvertiseWorker` → ordered family refeed → wire UPDATE on surviving TCP session | `TestRFC2545SharedSubnetLossReadvertisesGlobalOnly`, `test/plugin/rfc2545-condition-flip-readvertise.ci` |
| 2 | Third-party router on shared subnet transitions from ambiguous/missing neighbor entry to a unique Link-Local (or moves Link-Local) | Netlink neighbor watch → `refreshPeerLinkScopes` → `readvertiseWorker` → `request bgp rib refeed-local` / `request bgp rib reoffer-selected` / `RelayReplay` → FRR installs route via Link-Local | `TestRFC2545LateNeighborDiscoveryReadvertisesRetainedRoute`, `TestRFC2545KnownNeighborReplacementReadvertisesRetainedRoute`, `bgp-rfc2545-neighbor-condition-flip-frr` |
| 3 | Runs a forked `bgp-adj-rib-in` or suffers a store timeout during a condition flip | `readvertiseWorker` surfaces refusal/timeout → `bgp-next-hop-readvertise` runtime warning + coded `Warn` log with fix → visible in `ze show doctor` and `ze show warnings` | `TestRFC2545ReadvertiseDiagnosticRaiseAndClear` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC2545NeighborWatchUsesAddressSnapshot` | `internal/component/bgp/reactor/rfc2545_neighbor_late_discovery_test.go` | Deterministic address-snapshot seam prerequisite | Passing; preserve |
| `TestRFC2545NeighborKnownAtReceiptReachesSocket` | Same file | Known-neighbor 32-octet pair on both general and native RS rails | Passing; preserve |
| `TestRFC2545LateNeighborDiscoveryReadvertisesRetainedRoute` | Same file | Eight watch-triggered late-discovery transitions without new ingress | Red (target) |
| `TestRFC2545RefreshedFactsReachStoredReplaySocket`, `TestRFC2545QueuedNeighborReplayEndsWithdrawn` | Same file | Explicit relay and queued-withdrawal controls | Passing; preserve |
| `TestRFC2545KnownNeighborReplacementReadvertisesRetainedRoute` | `internal/component/bgp/reactor/rfc2545_neighbor_transition_test.go` | Moved third-party Link-Local re-advertised on both rails | Red (target) |
| `TestRFC2545SharedSubnetLossReadvertisesGlobalOnly` | Same file | Shared-subnet loss flips 32-octet pair back to 16-octet global on both rails | Red (target) |
| `TestRFC2545WatchReplayConcurrentWithdrawalEndsWithdrawn` | Same file | B1 hold-before-cut ordering: watch-triggered replay + concurrent source withdrawal ends withdrawn once | Red (target) |
| `TestRFC2545ReadvertiseBurstCoalescing` | `internal/component/bgp/reactor/reactor_readvertise_test.go` | 1A: immediate first pass (0 ms), per-peer cooldown, at most 1 trailing re-pass, unrelated-neighbor/reorder no-op, clean shutdown/retired-session discard | New |
| `TestRFC2545ConfigStaticAttributionOverlap` | `internal/component/bgp/reactor/reactor_readvertise_test.go` | Issue 5: `-race -count=20` overlap of initial sync, readvertise config-static resend, and concurrent API/forwarded send; zero cross-contamination | New |
| `TestRFC2545ReadvertiseDiagnosticRaiseAndClear` | `internal/component/bgp/reactor/reactor_readvertise_test.go` | AC-8: `bgp-next-hop-readvertise` (`doctor-bgp-next-hop-readvertise`) raises on forked/absent store or timeout and clears on recovery or session reset | New |
| `TestAdjRIBInReplayRoutesFamilySelector` | `internal/component/bgp/plugins/adj_rib_in/replay_page_test.go` | 3A: family filter, ascending `MsgID` order, legacy 2/3-arg compatibility, unknown family rejection, forked `!ingestTracked` `opererr` refusal | New |
| `TestRIBRefeedLocalNextHopSelfAndExplicit` | `internal/component/bgp/plugins/rib/rib_refeed_local_test.go` | AC-6: `NextHopSelf` preserved on both DirectBridge and JSON `format=full` `route-meta`, `show bgp rib sent` unchanged, unbracketed `refeed-local` (no BoRR/EoRR) on both ordinary and legacy peers, `update hex nhop set self` round-trip, withdrawal when `resolveNextHop` fails | New |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc2545-condition-flip-readvertise` | `test/plugin/rfc2545-condition-flip-readvertise.ci` | B2 (2A): real Linux daemon (`option=needs-linux:caps=net-admin`) loses and regains a shared IPv6 prefix via netlink on a surviving TCP session and re-advertises 16-octet then 32-octet next hop | New |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-rfc2545-neighbor-condition-flip-frr` | `test/interop/scenarios/bgp-rfc2545-neighbor-condition-flip-frr/` | FRR 10.7.1 | AC-4 & B2: daemon's neighbor watch updates once-announced API-originated subject after ambiguous-to-unique kernel neighbor transition and FRR installs route via the Link-Local (assertions 1–12) | Existing; assertions 1–8 pass, 9 & 12 target |

## Files to Modify
- `internal/component/bgp/reactor/reactor_iface.go` - effective-answer comparison in `refreshPeerLinkScopes` and nonblocking `p.scheduleReadvertise(session)` signal after facts publication.
- `internal/component/bgp/reactor/reactor.go` - default readvertise cooldown setting (test-injectable) and runtime doctor cleanup on reactor stop.
- `internal/component/bgp/reactor/policy_reload.go` - extract/reuse hold-before-cut replay ordering helpers, preserve `reconcileExportFinal` and `reconcileSelectedExport`, remove `sendingConfigStatic` swap.
- `internal/component/bgp/reactor/session_write.go` - send-scoped `configStatic` and `nextHopSelf` fields under `s.writeMu` into `wireu.SentOrigin`.
- `internal/component/bgp/reactor/reactor_notify.go` - remove `Peer.sendingConfigStatic.Load()` read and its `sentMeta` map injection; copy `SentOrigin.ConfigStatic` and `SentOrigin.NextHopSelf` directly onto `types.RawMessage` scalars (zero map allocation, matching `SourceLocal`).
- `internal/component/bgp/reactor/reactor_api_batch.go` - carry `NextHopSelf` intent from `resolveNextHop`/`initBatchOwner` onto `fwdItem`/`peerOp`/write path.
- `internal/component/bgp/reactor/peer.go` - delete `Peer.sendingConfigStatic` atomic field; add per-peer readvertise wake channel/state and start `p.readvertiseWorker()` on `p.wg` in `Peer.StartWithContext`.
- `internal/component/bgp/reactor/peer_initial_sync.go` - pass send-scoped `configStatic: true` for configured static routes instead of toggling `sendingConfigStatic`; align `sendPluginRoutesVia` attribution.
- `internal/component/bgp/reactor/peer_static_wire.go` - send-scoped `configStatic: true` in `deliverStaticRouteDelta` and per-family static/default-originate re-send helper under `staticMu` with resolved-next-hop withdrawal tracking.
- `internal/component/bgp/reactor/peer_static_restore.go` - replace `sendingConfigStatic.Swap` with send-scoped `configStatic: true`.
- `internal/component/bgp/wireu/wire_update.go` - add `ConfigStatic` and `NextHopSelf` scalar booleans to `wireu.SentOrigin`.
- `internal/component/bgp/types/rawmessage.go` - add `NextHopSelf` and `ConfigStatic` scalar booleans on `types.RawMessage`.
- `internal/component/bgp/format/text_update.go` - format `"config-static": true` and `"next-hop-self": true` directly from `msg` at the JSON `format=full` `route-meta` boundary (skipping both keys in the `msg.Meta` loop so policy cannot spoof them).
- `internal/component/bgp/format.go` - emit `nhop set self` on the `update hex` branch of `FormatAnnounceCommand` when `route.NextHop == "self"`.
- `internal/core/bgp/ribevents/replay.go` - add optional `family` argument to `RelayReplay` (`family <afi/safi>`).
- `internal/component/bgp/plugins/adj_rib_in/replay_page.go` - parse optional trailing `family <afi/safi>`, filter `walkReplayRoutes` in ascending `MsgID` order, refuse `!r.ingestTracked` with `opererr` naming in-process `bgp-adj-rib-in`.
- `internal/component/bgp/plugins/adj_rib_in/replay_page_test.go` - add unit tests for family-scoped `replay-routes`, ascending `MsgID` order, legacy compatibility, and forked-store refusal (3A).
- `internal/component/bgp/plugins/adj_rib_in/rib.go` - update `request bgp adj-rib-in replay-routes` help/declaration in `commandDecls()`.
- `internal/component/bgp/plugins/rib/rib.go` - declare hidden `request bgp rib refeed-local` and `request bgp rib reoffer-selected` in `commandDecls()` and populate `NextHopSelf` from JSON `route-meta`.
- `internal/component/bgp/plugins/rib/rib_commands.go` - implement `refeedLocalCommand` (`request bgp rib refeed-local <peer> <family>`) and `reofferSelectedCommand` (`request bgp rib reoffer-selected <peer> <session> <family>`).
- `internal/component/bgp/plugins/rib/rib_selected_policy.go` and `internal/component/bgp/plugins/rib/rib_selected_dispatch.go` - add family-scoped `reofferSelectedFamily(peer, session, fam)` filtering `d.current` and `d.scoped` strictly to `key.family == fam` so IPv6 condition flips emit zero `ipv4/unicast` re-advertisements on ordinary selected peers.
- `internal/component/bgp/plugins/rib/rib_replay.go` - replay `LocalOrigin && !ConfigStatic` entries unbracketed across ordinary and legacy up peers, mapping `entry.NextHopSelf` to `"self"` for `FormatAnnounceCommand`.
- `internal/component/bgp/plugins/rib/ribout_entry.go` - add `NextHopSelf bool` field on `ribOutEntry`.
- `internal/component/bgp/plugins/rib/rib_structured.go` - populate `ribOutEntry.ConfigStatic` and `ribOutEntry.NextHopSelf` directly from DirectBridge `*types.RawMessage` scalars (not `se.Meta`).
- `internal/component/bgp/reactor/rfc2545_neighbor_late_discovery_test.go` and `rfc2545_neighbor_transition_test.go` - preserve all 14 existing trigger/control probes.
- `internal/le/interoplab/bgp/check_rfc2545_neighbor_flip.go` - verify assertions 1–12 pass end-to-end against FRR 10.7.1.
- `docs/architecture/bgp/interface-event-reactions.md` - update `The decisions` section (departs row) to document watch-triggered re-advertisement and 1A burst coalescing.
- `docs/architecture/update-building.md` - document send-scoped `config-static` / `next-hop-self` attribution and in-place re-advertisement on condition flips.
- `docs/architecture/wire/messages.md` - update `Internal sent-origin receipt` to mention the `ConfigStatic` and `NextHopSelf` scalar fields on `wireu.SentOrigin`.
- `docs/architecture/api/commands.md` - document optional `family <afi/safi>` on `request bgp adj-rib-in replay-routes` and hidden internal `request bgp rib refeed-local` / `request bgp rib reoffer-selected`.
- `docs/architecture/api/json-format.md` - document additive `"next-hop-self": true` key in sent UPDATE `route-meta`.
- `docs/architecture/plugin/rib-storage-design.md` - document `ribOutEntry.NextHopSelf`, `reofferSelectedFamily`, and `FormatAnnounceCommand` `nhop set self` on `update hex`.
- `docs/architecture/bgp/replay-cursor.md` - document family-scoped replay ordering (`MsgID` ascending) and forked `bgp-adj-rib-in` refusal.
- `docs/architecture/doctor-and-health-checks.md` - document runtime check `bgp-next-hop-readvertise` (`doctor-bgp-next-hop-readvertise`).
- `rfc/short/rfc2545.md` - update `RFC2545-3-6` once native tests, FRR interop, and `./le rfc record` discrimination pass.

## Files to Create
- `internal/component/bgp/reactor/reactor_readvertise.go` - per-peer long-lived coalescing worker and cooldown timer (`p.readvertiseWorker`, `p.scheduleReadvertise`) with 1A burst coalescing (immediate first pass, clock-injectable 1 s cooldown via `p.clock`, max 1 trailing re-pass), effective-answer predicate, and shared hold-before-cut ordered refeed across all route rails.
- `internal/component/bgp/reactor/readvertise_diagnostic.go` - reactor-owned runtime doctor check `bgp-next-hop-readvertise` and code `doctor-bgp-next-hop-readvertise` (`diagnostic.DoctorPhaseRuntime`).
- `internal/component/bgp/reactor/reactor_readvertise_test.go` - unit tests for per-peer burst coalescing (1A), slow-peer independence (one stalled peer does not delay another), `-race -count=20` config-static overlap (Issue 5), runtime doctor raise/clear (AC-8), and peer-stop/shutdown/retired-session safety.
- `internal/component/bgp/plugins/rib/rib_refeed_local_test.go` - unit tests for `NextHopSelf` preservation (DirectBridge + JSON), `show bgp rib sent` stability, unbracketed `refeed-local`, family-scoped `reoffer-selected` (verifying zero `ipv4/unicast` re-advertisement), and `update hex nhop set self` round-trip.
- `test/plugin/rfc2545-condition-flip-readvertise.ci` - B2 fast Linux `.ci` daemon test (`option=needs-linux:caps=net-admin`) proving real netlink address prefix loss and restoration re-advertises on a surviving TCP session.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new config leaf or operator-facing YANG RPC (`replay-routes` has no YANG module in `bgp-adj-rib-in`, and `request bgp rib refeed-local` is a hidden internal RPC declared in `rib/rib.go` `commandDecls()` matching `request bgp rib recovery` and `request bgp rib flowspec-gate`) |
| YANG validation constraints | N-A | No new YANG nodes; `family <afi/safi>` is validated dynamically against registered families via `family.LookupFamily` |
| YANG custom validators | N-A | No new config leaves |
| CLI commands/flags | No | No operator-facing CLI command or flag added; both `replay-routes` and `refeed-local` are hidden internal RPCs |
| CLI grammar (keyword before value) | Yes | Trailing `family <afi/safi>` on `replay-routes` follows keyword-before-value while keeping legacy 2- and 3-arg callers unchanged |
| Editor autocomplete | N-A | No new config editor leaves |
| Functional test for new RPC/API | Yes | `test/plugin/rfc2545-condition-flip-readvertise.ci` (B2 Linux kernel `.ci`) plus `bgp-rfc2545-neighbor-condition-flip-frr` (real-kernel FRR interop) |
| Pipe completeness | N-A | Hidden internal RPCs return `shapeMap` status envelopes; no new `show` renderer |
| Env var registration | N-A | No environment variable added (1 s cooldown is a code default with a test-only clock seam) |
| Doctor check for runtime dependencies | No | Reuses existing netlink address/neighbor watchers and plugin transports; no new host dependency |
| Doctor code per problem class | Yes | Reactor-owned runtime check `bgp-next-hop-readvertise`, code `doctor-bgp-next-hop-readvertise`, `Phase: diagnostic.DoctorPhaseRuntime` (`internal/component/bgp/reactor/readvertise_diagnostic.go`) covering absent store, forked `bgp-adj-rib-in`, barrier timeout, and failed refeed |
| Prometheus counters/metrics | No | Scheduling and failure invariants are directly tested and reported via `bgp-next-hop-readvertise`; no new Prometheus counter |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new SAFI, capability, attribute, or wire NLRI format |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | Check `features/bgp-protocol.md` after implementation and interop proof; no maturity claim changes before passing evidence |
| 2 | Config syntax changed? | No | No new config option or leaf added |
| 3 | CLI command added/changed? | No | Both `request bgp adj-rib-in replay-routes` and `request bgp rib refeed-local` are hidden internal engine-to-plugin RPCs, not operator CLI commands |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (optional trailing `family <afi/safi>` on `replay-routes` and hidden `request bgp rib refeed-local`) and `docs/architecture/api/json-format.md` (`"next-hop-self": true` in sent `route-meta`) |
| 5 | Plugin added/changed? | Yes | `docs/architecture/plugin/rib-storage-design.md` (`ribOutEntry.NextHopSelf`, `FormatAnnounceCommand` `nhop set self`) and `docs/architecture/bgp/replay-cursor.md` (family-scoped ascending `MsgID` replay and forked `bgp-adj-rib-in` refusal) |
| 6 | Has a user guide page? | Yes | Check `docs/guide/route-reflection.md` and `docs/guide/route-injection.md` for unsolicited refresh behavior and `doctor-bgp-next-hop-readvertise` troubleshooting |
| 7 | Wire format changed? | No | BGP wire format is unchanged; timing/refresh and send-scoped attribution move into `docs/architecture/update-building.md` |
| 8 | Plugin SDK/protocol changed? | No | Zero `pkg/` Go types change; additive JSON `route-meta` key `"next-hop-self"` is documented in `docs/architecture/api/json-format.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc2545.md` (`RFC2545-3-6` loses `{partial}` after passing tests, FRR interop, and `./le rfc record`) |
| 10 | Test infrastructure changed? | No | Reuses existing `.ci` runner (`option=needs-linux:caps=net-admin`) and Docker FRR interop harness |
| 11 | Affects daemon comparison? | No | Existing dated BIRD/FRR source reading stays valid; both references are knowingly exceeded |
| 12 | Internal architecture changed? | Yes | `docs/architecture/bgp/interface-event-reactions.md` (departs row update), `docs/architecture/update-building.md`, `docs/architecture/doctor-and-health-checks.md` (`bgp-next-hop-readvertise`) |
| 13 | Route metadata keys added/changed? | Yes | `docs/architecture/api/json-format.md` (`"next-hop-self": true` in sent UPDATE `route-meta`, matching `"config-static": true`) |
| 14 | Prometheus counters added/changed? | No | No new Prometheus counter |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | Hidden `request bgp rib refeed-local` in `bgp-rib` `commandDecls()` and runtime doctor check `bgp-next-hop-readvertise` (`doctor-bgp-next-hop-readvertise`) in reactor `init()` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Verified via `./le spec citation anchors`. Declared `// Design:` docs modified: `docs/architecture/bgp/interface-event-reactions.md`, `docs/architecture/api/json-format.md`, `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/bgp/replay-cursor.md`, `docs/architecture/wire/messages.md`. Declared `// Design:` docs unaffected: `docs/architecture/api/architecture.md` (engine/RIB ownership split unchanged), `docs/architecture/core-design.md` (core architecture and component isolation unchanged), `docs/architecture/exabgp-bridge.md` (`peer_static_restore.go` only changes internal config-static send attribution, not ExaBGP translation), `docs/architecture/pool-architecture.md` (`ribOutEntry.NextHopSelf` is a plain bool, leaving `attrpool` dedup/compaction unchanged), `docs/architecture/testing/interop.md` (reuses existing named scenario `bgp-rfc2545-neighbor-condition-flip-frr` without harness changes), `docs/architecture/bgp/structural-forwarding.md` (already accurately documents `linkScope.thirdPartyLinkLocal` on fresh and relayed sends) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Checked `request bgp adj-rib-in replay-routes` callers and docs: legacy 2- and 3-arg syntax remains valid, and no example breaks |

## Design presentation

### What changes

When a Linux address event flips RFC 2545 Section 3's shared-subnet condition (either true-to-false or false-to-true) or a neighbor-table event discovers, changes, or clears a third-party router's Link-Local on a shared subnet, Ze re-advertises the affected peer's IPv6 routes in place without resetting the BGP session. Each `Peer` runs one long-lived coalescing goroutine and cooldown timer (`p.readvertiseWorker()` in `reactor_readvertise.go`, joined on `p.wg` and cancelled on `p.ctx`) so a slow or stalled peer hitting the 2 s barrier budget never delays any other peer. The pass handles all four route sources (ordinary selected routes via family-scoped `request bgp rib reoffer-selected` so IPv4 is never churned, forwarded routes via hold-before-cut `bgp-adj-rib-in` family-scoped replay, direct API routes via unbracketed `request bgp rib refeed-local` preserving `next-hop self` intent via zero-map typed `SentOrigin`/`RawMessage` scalars, and configured static/default routes via send-scoped `config-static` attribution). The first effective flip for an idle peer runs immediately (0 ms settle delay); subsequent bursts or active-pass flaps coalesce behind that peer's 1 s cooldown timer with at most one queued trailing re-pass. If a pass cannot complete because a store is absent, `bgp-adj-rib-in` is forked, a barrier times out, or refeed fails, Ze raises a runtime doctor warning (`bgp-next-hop-readvertise`, code `doctor-bgp-next-hop-readvertise`) with the exact fix and clears it once a later pass succeeds or the session resets.

### Before and after

1. **Shared IPv6 subnet lost or restored on a live session (AC-1, AC-2):**
   - **Before:** `refreshPeerLinkScopes` updates the peer's link-scope snapshot and schedules nothing. Already-advertised routes keep a stale 32-octet global+link-local next hop after subnet loss (which the peer can no longer resolve on-link) or a 16-octet global-only next hop after subnet gain until a new incoming UPDATE happens to arrive.
   - **After:** `refreshPeerLinkScopes` detects the effective-answer change and enqueues the peer. The readvertise worker immediately re-advertises the peer's IPv6 routes in place with no session reset: 16-octet global-only after subnet loss, 32-octet global+link-local computed fresh after subnet gain.
2. **Third-party router's Link-Local discovered or moved in the kernel neighbor table (AC-4 / `RFC2545-3-6`):**
   - **Before:** A route sent while the third-party neighbor was unknown or ambiguous stays global-only forever; `bgp-rfc2545-neighbor-condition-flip-frr` fails at assertion 9 after 31 global-only observations.
   - **After:** The neighbor watch triggers the worker, which re-advertises the affected IPv6 routes with the 32-octet global + third-party Link-Local pair; FRR installs the route via the Link-Local (assertions 1–12 pass), and unrelated neighbor changes trigger zero passes.
3. **Burst of address/neighbor events or active-pass flap (AC-3 / Issue 3):**
   - **Before:** The removed attempt re-sent the peer's whole table on every flap that arrived during an active pass, with no per-peer cooldown.
   - **After:** The first effective change runs immediately (0 ms), and any burst during the pass or the 1 s per-peer cooldown collapses into at most one trailing re-pass once cooldown expires.
4. **Live withdrawal racing a condition-flip replay (AC-5 / B1):**
   - **Before:** The removed attempt called `resendAdvertised` without holding live forwards first or draining per-source receive cuts, allowing a stale replay to overtake a live withdrawal.
   - **After:** The worker raises `beginPolicyReplay` before taking per-source `receiveCut()`, completes `drainResendSources` and `DrainEventsApplied` within one 2 s budget, relays the family in ascending `MsgID` order, and calls `flushPolicyReplay` before `endPolicyReplay`, so the live withdrawal is always the final wire message.
5. **Concurrent config-static resend overlapping initial sync or API/forwarded sends (AC-7 / Issue 5):**
   - **Before:** `Peer.sendingConfigStatic` was a peer-wide atomic flag that `sendInitialRoutes` and `deliverStaticRouteDelta` could interleave into a permanently stuck `true` state, tagging concurrent API/forwarded writes as `"config-static": true`.
   - **After:** `Peer.sendingConfigStatic` is removed; `configStatic` and `nextHopSelf` travel per send under `s.writeMu` into `wireu.SentOrigin`, `types.RawMessage`, and JSON `route-meta`.
6. **Degraded pass (absent store, forked `bgp-adj-rib-in`, timeout, or refeed failure) (AC-8):**
   - **Before:** No runtime doctor check existed for readvertise failures; a forked `bgp-adj-rib-in` lacked family-scoped replay refusal.
   - **After:** Forked `bgp-adj-rib-in` refuses family-scoped replay with an `opererr` naming in-process `bgp-adj-rib-in`, and the reactor raises runtime doctor check `bgp-next-hop-readvertise` (`doctor-bgp-next-hop-readvertise`) plus a coded `Warn` log with `"code"` and `"fix"`, clearing automatically on recovery or session reset.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Single reactor-owned pending-peer worker | One slow or stalled peer hitting the 2 s drain/applied-event barrier budget delays unrelated peers' condition flips; Thomas selected one coalescing goroutine/timer per peer (2026-11-22) |
| Unconditional settle delay (e.g. 500 ms) before every first pass | Adds latency to the common single-flip case when 1A (immediate first flip + 1 s per-peer cooldown + max 1 trailing re-pass) gives 0 ms first-flip latency and still bounds bursts |
| RFC 2439 BGP route flap dampening enabled by default | Explicitly rejected by Thomas on 2026-11-22 ("flap dampening should be an option and off by default as since the RFC it was found to not help"); this mechanism is local event burst coalescing, not RFC 2439 route dampening |
| Reusing `clear bgp rib out` (`outboundResend`) for direct API refeed | `outboundResend` skips ordinary destinations (`meta.refreshOrdinary`) and brackets legacy destinations with BoRR/EoRR, which under RFC 7313 §4 would make an ERR-capable peer purge all forwarded and config routes |
| Storing `"self"` directly in `ribOutEntry.NextHop` | Would change `show bgp rib sent` display output from the actual emitted IP to `"self"`; keeping a separate `ribOutEntry.NextHopSelf` bool preserves operator visibility |
| Session reset (Cease subcode 6) on condition flip | Rejected by Thomas's 2026-10-12 round-12 decision in `spec-bgp-link-local-only-next-hop` ("we do not want the peering to bounce") |

### Documents departed from

- `docs/architecture/bgp/interface-event-reactions.md`, section **The decisions**:
  - **Current statement:** "A neighbor change sets the form of the advertisements made after it, and re-advertises nothing. A route already sent with the global address alone is not sent again when the table later names the link-local. That re-advertisement is not built."
  - **After this work:** Updated in the same change to document that both address-table and neighbor-table events that change a peer's effective RFC 2545 / RFC 4659 / third-party Link-Local answer signal that peer's coalescing readvertise worker (`p.readvertiseWorker()` in `reactor_readvertise.go`), which runs the first flip immediately, coalesces bursts behind a 1 s per-peer cooldown timer with at most one trailing re-pass, and re-advertises affected IPv6 routes in place without resetting the session.

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Burst coalescing on active-pass flaps (Issue 3) | (a) Immediate first flip (0 ms) + 1 s per-peer cooldown + max 1 trailing re-pass, no new config option (1A); (b) Unconditional settle delay (e.g. 500 ms) before every pass | Thomas (2026-11-22): "flap dampening should be an option and off by default as since the RFC it was found to not help. 1A is fine." |
| C-2 | Daemon-level test coverage alongside FRR interop (B2) | (a) Keep real-kernel `bgp-rfc2545-neighbor-condition-flip-frr` AND add fast Linux `.ci` daemon test `test/plugin/rfc2545-condition-flip-readvertise.ci` (`option=needs-linux:caps=net-admin`) for address-prefix flips (2A); (b) Rely on Docker FRR interop alone | Thomas (2026-11-22): "2A." |
| C-3 | Family selector on `request bgp adj-rib-in replay-routes` | (a) Additive optional trailing `family <afi/safi>`, preserving legacy 2/3-arg callers and refusing forked `!r.ingestTracked` with `opererr` naming in-process `bgp-adj-rib-in` (3A); (b) New separate RPC | Thomas (2026-11-22): "3A." |
| C-4 | Readvertise worker model | (a) One long-lived coalescing goroutine/timer per `Peer` (joined on `p.wg`, cancelled on `p.ctx`, zero cross-peer blocking); (b) Single reactor-owned pending-peer worker | Thomas (2026-11-22): "One coalescing goroutine/timer per peer" |

## Implementation Steps

1. **Send-scoped attribution & API `self` intent (Issue 5 & AC-6 foundation):** delete `Peer.sendingConfigStatic`, add `ConfigStatic` and `NextHopSelf` to `wireu.SentOrigin` and `types.RawMessage`, serialize `"next-hop-self": true` in `format/text_update.go`, plumb per-send flags through `session_write.go`, `reactor_notify.go`, `reactor_api_batch.go`, `peer_initial_sync.go`, `peer_static_wire.go`, and `peer_static_restore.go`, update `FormatAnnounceCommand` to emit `nhop set self` on `update hex`, add `ribOutEntry.NextHopSelf`, and add hidden unbracketed `request bgp rib refeed-local <peer> <family>`. Prove with `TestRFC2545ConfigStaticAttributionOverlap` (`-race -count=20`) and `TestRIBRefeedLocalNextHopSelfAndExplicit`.
2. **Family-scoped `bgp-adj-rib-in` replay (3A):** extend `ribevents.RelayReplay` and `adj_rib_in/replay_page.go` with optional trailing `family <afi/safi>` (validated via `family.LookupFamily`, ascending `MsgID` sort, `!r.ingestTracked` `opererr` refusal). Prove with `TestAdjRIBInReplayRoutesFamilySelector`.
3. **Runtime doctor check & per-peer hold-before-cut worker (B1, AC-3, AC-8):** create `readvertise_diagnostic.go` (`bgp-next-hop-readvertise`, `doctor-bgp-next-hop-readvertise`, `DoctorPhaseRuntime`) and `reactor_readvertise.go` (per-peer long-lived coalescing worker + `p.clock` cooldown timer with 1A burst coalescing, family-scoped `request bgp rib reoffer-selected` for ordinary selected peers, `beginPolicyReplay` -> `receiveCut` -> `drainResendSources` -> `DrainEventsApplied` -> `RelayReplay` -> `flushPolicyReplay` -> `endPolicyReplay` for explicit-role peers, `refeed-local` for direct API routes, `staticMu` per-family resend for static/default routes). Wire into `peer.go` (`StartWithContext` / `p.wg`), `reactor.go`, and `reactor_iface.go` (`refreshPeerLinkScopes` effective-answer comparison).
4. **Unit & functional proof (AC-1..AC-8, B2):** verify all 14 existing trigger tests in `rfc2545_neighbor_late_discovery_test.go` and `rfc2545_neighbor_transition_test.go` turn green under `-race`, add `reactor_readvertise_test.go` (`TestRFC2545ReadvertiseBurstCoalescing`, `TestRFC2545ReadvertiseDiagnosticRaiseAndClear`), and create and run `test/plugin/rfc2545-condition-flip-readvertise.ci` (`option=needs-linux:caps=net-admin`).
5. **Real-kernel FRR interop, RFC discrimination & docs:** run `bgp-rfc2545-neighbor-condition-flip-frr` through assertions 1–12, record discrimination (`./le rfc record`) and update `rfc/short/rfc2545.md` (`RFC2545-3-6` loses `{partial}`), update the architecture docs listed in `Files to Modify`, and run the Review Gate before closure.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code. The
re-advertisement is an addition Thomas ordered rather than a Section 3
obligation, so the comment must say which document each line answers to.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | only the prefixes the condition governs are re-advertised |
| Rule: rfc-compliance | the code comment does not claim Section 3 requires what Thomas ordered |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Watch-triggered retained-route refeed | Existing fourteen transition/concurrency cases become green with useful-route and same-session controls preserved |
| Ordered/shared replay and origin semantics | Main-thread scoped race tests (`-race -count=20` Issue 5, B1 withdrawal ordering, 1A burst coalescing, 3A family replay, AC-6 `refeed-local`, AC-8 doctor raise/clear) |
| Daemon `.ci` & foreign receiver outcome | `test/plugin/rfc2545-condition-flip-readvertise.ci` passes on Linux via the netlink socket and `bgp-rfc2545-neighbor-condition-flip-frr` passes assertions 1–12 |
| Current RFC/docs evidence | Native discrimination (`./le rfc record`), `rfc/short/rfc2545.md` updated, and all modified architecture docs pass `./le doc check all` |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | Re-read the source in Current Behavior |

## Known Limitations

- A pass is not incremental on the wire, as in
  `spec-bgp-link-local-only-next-hop` Known Limitations: it re-sends every
  stored route of the affected IPv6 family from every source and every locally
  originated entry for a peer whose condition flipped; without ADD-PATH a
  prefix several sources hold is written once per source in ascending
  message-ID order, which matches the live rail's last write only for
  UPDATEs received one after another, never for UPDATEs received
  concurrently; and live forwards to that peer wait behind the pass, up to
  the 2 s publication and applied-event budget plus the relay, for all its
  families under one hold (round-10 I1, B1; round-11 I1, N1).
- A forked `bgp-adj-rib-in` stores MsgID 0, so family-scoped `replay-routes`
  refuses there with an `opererr` naming in-process `bgp-adj-rib-in`;
  forwarded routes are not re-sent on that deployment, and the worker raises
  `bgp-next-hop-readvertise` (`doctor-bgp-next-hop-readvertise`) and logs a
  coded `Warn` naming the fix (round-11 I2).
- Where `bgp-adj-rib-in` is not loaded on an explicit-role peer, or the
  destination's `send update` attachment does not name it, forwarded routes
  are not re-sent (`filterPermittedPeers`; round-10 N1); ordinary selected
  peers (`p.ordinarySelectedExport()`) do not require `bgp-adj-rib-in`
  because family-scoped `request bgp rib reoffer-selected` (`reofferSelectedFamily`) re-offers selected routes from `bgp-rib`.

## Review Gate

Research/source inspection is not design approval or implementation review.
Independent implementation review runs remain empty until that phase.

| Run | Scope | Reviewer | Result | Fixes / rerun |
|-----|-------|----------|--------|---------------|

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features (or N-A with a reason)

### Goal Gates (MUST pass)
- [ ] Research/design decisions approved in the main thread.
- [ ] Existing AC-1 through AC-4 and retained earlier review obligations demonstrated.
- [ ] Current scoped tests pass in the main checkout, including race/order controls.
- [ ] Native daemon/FRR subject transition and actual peer installation pass.
- [ ] Producing-code discrimination, RFC audit bindings and documentation are current.
- [ ] Independent implementation review is clean; native closure then removes this spec.

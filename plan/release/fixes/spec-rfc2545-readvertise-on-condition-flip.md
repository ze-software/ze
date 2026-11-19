# Spec: an RFC 2545 condition flip leaves the peer with a next hop it cannot resolve

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P1 |
| Release reason | Honor the explicit owner requirement to reconcile next hops after interface condition changes. |
| Scope | protocol |
| Depends | plan/release/fixes/spec-bgp-link-local-only-next-hop.md (its AC-17 designs the per-peer re-advertise this spec reuses) |
| Phase | research |
| Handoff | - |
| Updated | 2026-11-16 |

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

Design admission remains **not approved**. The strongest concern is a patch
that makes the simple second-send assertions pass but replays stale state
after a live withdrawal or loses API/config origin semantics. Complete those
decisions and the source/metadata consumer census before implementation.
The main thread owns the research/design gates and every runtime command.

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
The shipped composition includes `iface` and its Linux netlink backend;
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

| Mechanism | Producer | Proposed use |
|-----------|----------|--------------|
| Shared table refresh | `reactor_iface.go::refreshPeerLinkScopes` | Extend with effective-answer comparison and nonblocking scheduling after facts publication |
| Live replay hold and flush | `policy_reload.go::beginPolicyReplay`, `flushPolicyReplay`, `endPolicyReplay` | Extract/reuse; hold before cuts/drains and flush before release |
| Source publication barrier | `receive_publication.go::Peer.receiveCut`, `receivePublicationCut.state`, `receivePublicationCut.wait` | Capture and drain each relevant source, skipping ended/failed publications explicitly |
| Applied-event barrier | `internal/component/plugin/process/delivery.go::Process.DrainEventsApplied` | Await after source publication, under the shared budget and without peer/table/writer locks |
| Bounded owner-relayed pages | `internal/core/bgp/ribevents/replay.go::RelayReplay`, `adj_rib_in/replay_page.go::buildReplayPage` | Extend family scope while preserving cursor/page bounds and actual caller grants |
| Normal retained-route writer | `reactor_api_relay.go::RelayStoredRoute` | Reuse policy, next-hop normalization, provenance and final admission |
| Local sent-route replay | `rib/rib_commands.go::outboundResend`, `rib/rib_replay.go::collectGroupedRibOutRoutesFiltered` | Extend a separate local-origin-only refeed; preserve original `self` versus explicit intent and omit refresh markers |
| Configured/default writers | `peer_static_wire.go::deliverStaticRouteDelta` and restoration calls in `policy_reload.go` | Reuse ordinary writers; make config-static attribution send-scoped or serialize every reader |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- an interface address added or removed event, delivered to the reactor
- Both address and neighbor watches call `refreshPeerLinkScopes`; the new worker is a proposed extension, not an existing trigger.

### Transformation Path
1. Refresh immutable facts from the actual watched tables, once for all peers.
2. Compare old/new effective answers and queue only affected live peers. A table reorder, unrelated neighbor or diagnostic-state change alone is not a next-hop change.
3. A bounded worker captures the destination session and raises the existing replay hold before any source cut or retained-page read.
4. Capture relevant sources' receive cuts, await publication, then await the store's applied-event barrier. Both waits share the recorded two-second budget. Skip ended/failed publications and propagate other failures.
5. Relay the relevant family in ascending replay sequence under the actual owner's grant through `RelayStoredRoute`. Recompute the next hop from current facts. Add no synthetic EOR.
6. Refeed API-originated, configured and default routes through ordinary writers with original next-hop intent and correct send attribution.
7. Flush replay, release the hold and resume live forwards. A later actual withdrawal/replacement must remain the final socket state.
8. Cancel worker/pass contexts on reactor stop, join through its wait group and discard work for retired sessions.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface component ↔ BGP reactor | registered address/neighbor readers and watchers, through actual table callbacks | Existing callbacks and refreshed facts observed; replay not implemented |
| reactor ↔ route sources | the re-advertise `spec-bgp-link-local-only-next-hop` designs: the forward rail over adj-rib-in's stored routes (`RelayStoredRoute`), under its one per-peer re-send hold, its cut from the sources' `receiveCut`, and its publication and applied-event waits (round-10 B1, round-11 B1), RIB `outboundResend` for `LocalOrigin` entries that are not `ConfigStatic` with `self` replayed as `self` (round-10 I2), and the config-route re-send under `staticMu`; each re-runs the egress next-hop decision | No |

### Integration Points
- `peer.refreshForwardFactsIfLiveFrom` (`peer_forward_facts.go`), also the settings-apply path
- the existing replay hold, publication and applied-event barriers, registered pages and ordinary relay/origin writers in Existing Mechanisms

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Unapproved | Proposal uses normal relay and origin writers; implementation proof owed |
| No unintended coupling (components stay isolated) | Unapproved | Resolve actual registered command owners and grants; no RIB implementation import |
| No duplicated functionality (extends existing, does not recreate) | Unapproved | Extract policy replay mechanisms rather than a second route inventory |
| Zero-copy preserved where applicable (refs, not copies) | Unapproved | Preserve cached source bytes; measure approved implementation's allocations |
| Registration over hardcoding | Unapproved | Family selector and runtime diagnostics require their normal declarations |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Existing replay hold/flush machinery can be extracted without changing restoration | `policy_reload.go` has the required hold/flush/end producers | Policy reload or live forwarding regresses | Existing policy-final restoration tests and hold-before-cut ordering probe | Unvalidated for extraction |
| A-2 | Original API next-hop intent can be retained with current ownership | Sent-route grouping records local origin but replays output addresses | `self` becomes a stale explicit address | Complete origin/sent/group/replay census and preserve both useful self/explicit controls | Open design choice |
| A-3 | One bounded worker can serve affected peers | Event-driven shared-worker proposal above | A slow peer delays unrelated peers | Controlled burst, slow-peer and shutdown tests | Open scheduling choice |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A refeed fires an update storm on a burst or active-pass flap | Repeated passes for one final answer | Effective-answer comparison plus an approved coalescing/active-pass bound. A capacity-one wake alone does not settle this risk |
| R-2 | A per-peer refeed changes sharing or replays an unaffected family | Wrong-peer/family output or unnecessary materialization | Preserve ordinary per-destination writers and prove scoped-family/unaffected-peer controls |
| R-3 | Replay overtakes a live withdrawal/replacement | Final socket contains a resurrected route | Hold before source cuts, publication/applied drains and retained reads; flush before release and complete the existing callback concurrency probe |
| R-4 | Config-static attribution contaminates another writer | Plugin/forwarded route is marked config-static | Send-scoped attribution or complete serialization; retain the earlier initial-sync/plugin-send race proof |
| R-5 | Ended source or retired destination leaves work pending | Unreachable cut or an old session reaches a writer | Source publication-state handling, captured destination identity and cancellation/join on shutdown |
| R-6 | New selector/intent metadata breaks existing or forked callers | Parser failure, wrong field, silent empty replay | Additive command design and full Go/SDK/JSON/forked consumer census before approval |
| R-7 | Missing store or failed pass is presented as success | Retained subject stays stale after completion is reported | Select applicable runtime diagnostics and prove actual failure handling; no invented diagnostic code |
| R-8 | Absence tests pass after useful output is dropped | Known/fresh controls stop appearing | Preserve real socket, selecting/store retention, useful routes and unchanged sessions in existing probes |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | routes are re-announced to peers that did not need them, or the next hop stays wrong |
| How is it reverted? | No code exists to undo from this proposal. Implementation must permit disabling its scheduler while preserving the existing watched-facts and ordinary-writer behavior; source reversals follow the repository's owner-controlled Git rules |
| Who else touches this path? | `spec-bgp-link-local-only-next-hop`, `spec-connected-static-reach-the-locrib` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Actual address watch removes the shared-subnet condition | → | `refreshPeerLinkScopes` → proposed worker → normal relay | `TestRFC2545SharedSubnetLossReadvertisesGlobalOnly` |
| Actual neighbor watch discovers/replaces an identity | → | Refreshed facts → proposed worker → retained replay | `TestRFC2545LateNeighborDiscoveryReadvertisesRetainedRoute`, `TestRFC2545KnownNeighborReplacementReadvertisesRetainedRoute` |
| Source withdraws while callback replay is queued | → | Actual watch → destination hold/worker → source/store removal → socket | `TestRFC2545WatchReplayConcurrentWithdrawalEndsWithdrawn` |
| Real kernel changes ambiguous neighbor to unique | → | Daemon watch → local API-originated subject → FRR | `bgp-rfc2545-neighbor-condition-flip-frr` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | the shared-subnet condition goes from true to false | the prefixes it governs are re-advertised in place, with no session reset, with a next hop the peer can resolve, through `spec-bgp-link-local-only-next-hop`'s re-advertise path (its Key Design Decisions, "Re-advertise", option B of its round-12 owner decision) |
| AC-2 | the condition goes from false to true | the prefixes it governs are re-advertised in place in one pass, with no session reset, with the link-local appended after the global, through the same path as AC-1 (RFC 4271 Section 9.2; amended 2026-10-12 to agree with `spec-bgp-link-local-only-next-hop` AC-17, which re-advertises on every change of Ze's own next hop). The link-local is the egress's answer on the re-send, never a recorded one: the forward rail's pair trimming decides it for a forwarded route, `linkLocalNextHopFor` for an API route replayed as `self` or as its explicit global, and the same call for a config route; a replay of a recorded address with `nhop set` is not the path (re-checked after round-9 B1, N4) |
| AC-3 | an interface burst delivers several address events with one resulting table | one re-advertisement, not one per event |
| AC-4 (`RFC2545-3-6`, gap half; from spec-bgp-unowned-rfc-rows AC-5) | a route went out with a third-party global next hop alone, because the neighbor table named no Link-Local for that router, and the table later names one (or names a different one) | the affected prefixes are re-advertised in place through the same path as AC-1, with the router's Link-Local after its global in a 32-octet field; a table change for an address no advertised next hop uses re-advertises nothing. Then the row in `rfc/short/rfc2545.md` loses `{partial}`, its new tagged units carry a discrimination record (`./le rfc record`), and the `bgp-rfc2545-third-party-linklocal-frr` scenario is extended to show FRR resolving the next hop after the change |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Removes the shared subnet while the route and session remain useful | Actual watch recomputes facts, then proposed ordered refeed emits the global-only form | `TestRFC2545SharedSubnetLossReadvertisesGlobalOnly`; currently red |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC2545NeighborWatchUsesAddressSnapshot` | `rfc2545_neighbor_late_discovery_test.go` | Existing deterministic snapshot prerequisite | Observed pass; preserve |
| `TestRFC2545NeighborKnownAtReceiptReachesSocket` | Same file | Known-neighbor useful output on both rails | Observed pass; preserve |
| `TestRFC2545LateNeighborDiscoveryReadvertisesRetainedRoute` | Same file | Eight actual watch transitions without new ingress or a test-requested relay | Eight observed trigger reds |
| `TestRFC2545RefreshedFactsReachStoredReplaySocket`, `TestRFC2545QueuedNeighborReplayEndsWithdrawn` | Same file | Existing explicit writer and queued-withdrawal controls | Observed pass; not callback proof |
| `TestRFC2545KnownNeighborReplacementReadvertisesRetainedRoute` | `rfc2545_neighbor_transition_test.go` | Useful retained route acquires the moved identity on both rails | Two observed trigger reds |
| `TestRFC2545SharedSubnetLossReadvertisesGlobalOnly` | Same file | Useful global-only route after actual subnet loss | Two observed trigger reds |
| `TestRFC2545WatchReplayConcurrentWithdrawalEndsWithdrawn` | Same file | Actual callback replay, then real store removal and exactly one final withdrawal | Two reds before replay dispatch; later assertions unreached |
| Approved design's missing invariants only | Existing test area | Burst/active-pass bound, shutdown, family-selector compatibility, original self/explicit intent and config attribution overlap | Design/seams still open; do not duplicate the fourteen cases |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Earlier B2 `.ci` obligation | Existing functional harness | Determine the distinct daemon carrier needed alongside the real-kernel FRR scenario | Remains owed unless the owner explicitly changes the earlier contract; no silent removal or duplicate scenario authoring |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-rfc2545-neighbor-condition-flip-frr` | `test/interop/scenarios/` | FRR 10.7.1 | The daemon's actual neighbor watch must update the once-announced API-originated subject after its neighbor becomes unique | Observed assertion 9 failure with passing kernel, sent-retention, fresh-pair and same-session controls; successful installation at assertion 12 remains unreached |

## Files to Modify
- `internal/component/bgp/reactor/reactor_iface.go` - effective-answer change detection and nonblocking scheduling after publication.
- `internal/component/bgp/reactor/reactor.go` - worker start/stop/state. Its current foreign protocol-capture hunks require owner coordination and scoped landing.
- `internal/component/bgp/reactor/policy_reload.go` - shared hold/cut/drain/flush extraction, preserving policy restoration.
- `internal/core/bgp/ribevents/replay.go` and `internal/component/bgp/plugins/adj_rib_in/replay_page.go` - proposed additive family-scoped page contract and compatibility.
- `internal/component/bgp/plugins/rib/rib_commands.go`, `rib_replay.go` and sent producers in `rib.go` - local-origin-only refeed and original intent; complete consumer census before selecting fields.
- `internal/component/bgp/reactor/peer_static_wire.go` and configured/default callers - normal writers and send attribution, with the exact caller census still a design prerequisite.
- The existing two neighbor test files and checker/scenario above - preserve current probes and add only approved missing invariant coverage.
- `docs/architecture/bgp/interface-event-reactions.md`, `docs/architecture/update-building.md`, `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/api/commands.md` - update the trigger, refeed and command contracts with their implementation.
- `rfc/short/rfc2545.md` - retain partial scope until native transition, ordering and peer-installation proof. No support increase from this investigation.

## Files to Create
- Proposed `internal/component/bgp/reactor/reactor_readvertise.go` - bounded coalescing worker and shared ordered refeed. This is not permission to restore the removed attempt.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | Yes, conditional | A new replay-family selector must update its registered command declaration; exact schema/argument form is an open design choice |
| YANG validation constraints | Yes, conditional | Validate the selector against registered families and preserve old cursor/cut calls |
| Custom config validators | N-A | No new config leaf is proposed |
| CLI commands/flags | Yes, conditional | Additive replay-selector contract; no new public verb is selected |
| CLI grammar | Yes | Resolve keyword/value grammar and old-call compatibility before implementation |
| Editor autocomplete | N-A | No new config editor value is proposed |
| Functional API proof | Yes | Existing real-kernel daemon/FRR carrier plus explicit disposition of the earlier B2 `.ci` obligation |
| Pipe completeness | N-A | No new output renderer is proposed |
| Env registration | N-A | No environment variable is proposed |
| Doctor check for runtime dependencies | No new dependency | Reuse registered watchers, store and existing transports |
| Doctor code per failure class | Yes, unresolved | Absent store, timeout and failed refeed need an applicable existing code or registered runtime check; select names and phase before approval |
| Prometheus counters/metrics | No new metric proposed | Observe scheduling invariants directly in tests; a metric cannot replace the burst or shutdown proof |
| BGP family surface | N-A | No new family, capability, attribute or NLRI format |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | Feature scope/evidence changed? | Yes | Find the existing BGP feature row and current level before implementation; no evidence increase from this research |
| 2 | Config syntax changed? | No | No new config option proposed |
| 3 | CLI command changed? | Yes, conditional | `docs/guide/command-reference.md` if the replay selector changes |
| 4 | API/RPC changed? | Yes | `docs/architecture/api/commands.md` for selector, ordering and refusal contracts |
| 5 | Plugin behavior changed? | Yes, conditional | `docs/guide/plugins.md` if replay caller/declaration semantics change |
| 6 | User guide required? | Yes | Existing next-hop guidance must explain unsolicited refresh and runtime failure diagnostics |
| 7 | Wire format changed? | No new format | Existing native forms remain; update timing/refresh behavior in `docs/architecture/update-building.md` |
| 8 | SDK/protocol changed? | Yes, conditional | Complete original-intent producer/consumer census and update `docs/architecture/api/process-protocol.md` if fields change |
| 9 | RFC behavior newly proven? | Yes, after proof | `rfc/short/rfc2545.md`; generated status comes only through native tools and passing evidence |
| 10 | Test infrastructure changed? | No | Reuse current functional/interop harnesses |
| 11 | Daemon comparison changed? | No new claim | Preserve dated, qualified BIRD/FRR source reading above |
| 12 | Architecture changed? | Yes | Interface-event, update-building and structural-forwarding pages named above |
| 13 | Route metadata changed? | Yes, conditional | Document any approved original-intent/attribution keys and all consumers before implementation |
| 14 | Metrics changed? | No | No new metric proposed |
| 15 | Registration/inventory changed? | Yes, conditional | Replay command declaration and published inventory if its selector changes |
| 16 | Existing source anchors affected? | Yes | Main owes the native scoped citation inventory before implementation; named `// Design:` pages travel with the code |
| 17 | Existing examples affected? | Yes | Check replay examples against the approved parser and retain old syntax unless compatibility is explicitly changed |

The discovery surface is the existing BGP next-hop/interface-event material
in `ai/INDEX.md`, the registered watcher and replay command. Extend those
surfaces with implementation. Do not create a second route-source inventory.
The changed-wire census is incomplete until the proposed selector and API
intent/attribution fields have traced Go, SDK, JSON and forked consumers.
This prevents design approval; no source-read checklist claims completeness.

## Implementation Steps

1. **Research/design gate, still open:** finish selector, intent/consumer census, attribution, diagnostics and coalescing choices. Preserve existing owner decisions and dependency ownership.
2. **Shared primitive:** extract and prove hold-before-source-cut/publication/applied-drain/relay/flush ordering through normal writers before wiring the watch.
3. **Wiring:** add the bounded cancellable worker and effective-answer scheduling. Reuse all fourteen trigger cases and known/explicit controls.
4. **Origin coverage:** complete API/config/default refeed, original intent and send attribution. Prove approved burst, shutdown, compatibility and overlap invariants.
5. **Main-thread protocol proof:** rerun the named daemon/FRR carrier and require assertion 9 and assertion 12. Obtain independent producing-code discrimination and current RFC audit bindings.
6. **Independent review/closure:** review all implementation and docs. Keep this spec open until AC-1 through AC-4 and earlier B1/B2/ISSUE 3/ISSUE 5 obligations are satisfied.

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
| Ordered/shared replay and origin semantics | Main-thread scoped race tests, actual callback final-withdrawal proof and approved missing-invariant probes |
| Foreign receiver outcome | Existing daemon/FRR carrier reaches successful subject transition and installed Link-Local path |
| Current RFC/docs evidence | Native discrimination and audit binding where actual tagged claims change; no support credit before that |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | Re-read the source in Current Behavior |

## Known Limitations

These are prior design limitations, not observations of a completed repair.
The family selector, original-intent, diagnostics and scheduling choices above
are still unresolved. This record is not ready for implementation.
- A pass is not incremental on the wire, as in
  `spec-bgp-link-local-only-next-hop` Known Limitations: it re-sends every
  stored route of the family from every source and every locally
  originated entry for a peer whose condition flipped; without ADD-PATH a
  prefix several sources hold is written once per source in ascending
  message-ID order, which matches the live rail's last write only for
  UPDATEs received one after another, never for UPDATEs received
  concurrently; and live forwards to that peer wait behind the pass, up to
  the 2 s publication and applied-event budget plus the relay, for all its
  families under one hold (round-10 I1, B1; round-11 I1, N1).
- A forked `bgp-adj-rib-in` stores MsgID 0, so the family replay refuses
  there and forwarded routes are not re-sent; the worker logs a Warn naming
  in-process `bgp-adj-rib-in` as the fix (that spec's R-9, round-11 I2).
- Where `bgp-adj-rib-in` is not loaded, or the destination's `send update`
  attachment does not name it, forwarded routes are not re-sent
  (`filterPermittedPeers`; that spec's R-8, round-10 N1).
- The current family-selector and original-intent consumer census must be completed before choosing command or metadata fields.

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

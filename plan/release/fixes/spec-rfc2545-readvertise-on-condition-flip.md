# Spec: an RFC 2545 condition flip leaves the peer with a next hop it cannot resolve

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P1 |
| Release reason | Honor the explicit owner requirement to reconcile next hops after interface condition changes. |
| Scope | protocol |
| Depends | plan/release/fixes/spec-bgp-link-local-only-next-hop.md (its AC-17 designs the per-peer re-advertise this spec reuses) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-15 |

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
Rename them back when you restore them.

| ID | Class | Finding | What the next attempt owes |
|----|-------|---------|----------------------------|
| B1 | BLOCKER | The replay through `resendAdvertised` skipped the ordering safeguard: no receive cut, no drain of each source, no `DrainEventsApplied`. A route withdrawn or replaced on the live rail during the pass could land at the peer after its live replacement, so the peer ends holding a stale or withdrawn route | The relay starts only after every source's publication reaches its cut and adj-rib-in's process applied every prior event, under the one per-peer hold, as Task describes. A test proves a withdrawal received during the pass is the last word on the wire |
| B2 | BLOCKER | Only unit tests covered it. No daemon-level test drove an address or neighbor change through the real watch to an UPDATE on the wire, and no interop scenario showed FRR or BIRD resolving the new next hop | A `.ci` test through the daemon, and an interop scenario (extend `bgp-rfc2545-third-party-linklocal-frr` or add a named one) where the peer's RIB shows the new Link-Local after the change |
| ISSUE 3 | ISSUE | Each pass re-sends the peer's whole table, and nothing debounces a burst of changes. The capacity-one wake collapses only changes that arrive while a pass is already queued, so a flapping link drives one whole-table re-send per flap | A bound on passes per peer per interval, or a settle delay before a pass, with a test that a burst gives one pass |
| ISSUE 5 | ISSUE | The config-static re-send races `sendingConfigStatic` (`peer.go`). It is one per-peer atomic that `notifyMessageReceiver` reads for every sent event, to tag it config-static so the RIB skips storing it in ribOut. `sendInitialRoutes` raises it under `staticMu`, but it clears it after `staticMu` is released and after default-originate (`peer_initial_sync.go`). `restoreConfigured` `Swap`s it and restores the old value (`peer_static_restore.go`). A pass that raises it while another writer sends to the same peer tags that writer's plugin or forwarded routes config-static, and an initial sync that clears it during a pass untags the pass's static routes. (Mechanism read at the producers on 2026-10-31; the review's own wording is not in the tree) | The marker belongs to the send, not to the peer: carry it on the send itself, or serialise every writer that reads it. Add a race test (`-race -count=20`) of a pass overlapping initial sync and a plugin send |

## Task

### Test investigation (2026-11-15)

The RFC test campaign adds
`internal/component/bgp/reactor/rfc2545_neighbor_late_discovery_test.go` and
`internal/component/bgp/reactor/rfc2545_neighbor_transition_test.go`
without advancing this skeleton's approval gates. The full RFC 2545 text was
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
address event it snapshots the peer list, reads `network.ConnectedPrefixes()`,
and for each peer whose stored `llScope.connected` differs from that table calls
`peer.refreshForwardFactsIfLiveFrom(connected)`. It re-settles the snapshot and
deliberately re-advertises nothing. The false-to-true direction was first
recorded as harmless; since 2026-10-12 it re-advertises too (AC-2), to agree
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
- [ ] `docs/architecture/bgp/` - the Adj-RIB-Out and the update-send path
  → Decision: <to be filled>
  → Constraint: <to be filled>

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2545.md` - Section 3's shared-subnet condition and where it puts usability
  → Constraint: Section 3 binds the act of advertising, not re-advertising
- [ ] `rfc/short/rfc4271.md` - Section 9.2 Update-Send, the reading the ruling takes
  → Constraint: <to be filled>

**Key insights:** (minimal context to resume after compaction)
- <to be filled>

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/reactor/reactor_iface.go` - `(*Reactor).refreshPeerLinkScopes` reads `network.ConnectedPrefixes()`, skips a peer whose stored `llScope.connected` equals it, and otherwise calls `peer.refreshForwardFactsIfLiveFrom`; it re-advertises nothing. Its callers are the address watch (`addressSync.apply`, `reactor_address_watch.go`) and the neighbor watch (`reactor_iface.go`); `handleAddrAddedPayload` and the bus subscription were removed in 5682b2c17c
- [ ] `rfc/full/rfc2545.txt` - Section 3, the shared-subnet condition itself

**Behavior to preserve:** (unless the user explicitly said to change it)
- one refresh per interface burst, never one per address event. The
  equal-table guard that gave it is replaced by
  `spec-bgp-link-local-only-next-hop` AC-17: the guard compares masked
  prefixes, which cannot see a link-local move, so the refresh now compares
  each peer's inputs unmasked, and the burst collapses on the worker's
  capacity-one channel (amended 2026-10-12 to agree with that spec)

**Behavior to change:** (only what the user asked for)
- a true-to-false flip re-advertises the prefixes the condition governs
- a false-to-true flip re-advertises them too, with the link-local now
  appended: the next hop changes, so the route is a new one in the peer's
  Adj-RIB-Out and RFC 4271 Section 9.2 owes the UPDATE ("All newly installed
  routes and all newly unfeasible routes for which there is no replacement
  route SHALL be advertised to its peers by means of an UPDATE message").
  This replaces the earlier "harmless, nothing sent" reading so the two specs
  agree (`spec-bgp-link-local-only-next-hop` AC-17, amended 2026-10-12)

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- an interface address added or removed event, delivered to the reactor
- <to be filled>

### Transformation Path
1. <to be filled>

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface component ↔ BGP reactor | interface address event payload | No |
| reactor ↔ route sources | the re-advertise `spec-bgp-link-local-only-next-hop` designs: the forward rail over adj-rib-in's stored routes (`RelayStoredRoute`), under its one per-peer re-send hold, its cut from the sources' `receiveCut`, and its publication and applied-event waits (round-10 B1, round-11 B1), RIB `outboundResend` for `LocalOrigin` entries that are not `ConfigStatic` with `self` replayed as `self` (round-10 I2), and the config-route re-send under `staticMu`; each re-runs the egress next-hop decision | No |

### Integration Points
- `peer.refreshForwardFactsIfLiveFrom` (`peer_forward_facts.go`), also the settings-apply path
- the update-send path that would carry the re-advertisement

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | <to be filled> | <to be filled> | <to be filled> | <to be filled> | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | a refeed fires an update storm on an interface burst | update counts jump on a flap | the link-scope worker's capacity-one channel collapses a burst to one pass, and only a peer whose answer changed is re-advertised (`spec-bgp-link-local-only-next-hop` AC-17). Within a pass nothing is suppressed as unchanged: a forwarded write never is and every `outboundResend` command carries `meta["replay"]`, so a pass for a changed peer re-sends every stored route of the family (round-10 I1; Known Limitations) |
| R-2 | a per-peer refeed disturbs update-group sharing | <to be filled> | <to be filled> |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | routes are re-announced to peers that did not need them, or the next hop stays wrong |
| How is it reverted? | <to be filled> |
| Who else touches this path? | `spec-bgp-link-local-only-next-hop`, `spec-connected-static-reach-the-locrib` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| an address event that makes the shared-subnet condition false | → | <to be filled> | <to be filled> |

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
| 1 | removes the shared subnet from the link and the peer keeps forwarding | <to be filled> | <to be filled> |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| <to be filled> | `internal/component/bgp/reactor/` | <to be filled> | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| <to be filled> | `test/` | <to be filled> | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-rfc2545-neighbor-condition-flip-frr` | `test/interop/scenarios/` | FRR 10.7.1 | The daemon's actual neighbor watch must update the once-announced API-originated subject after its neighbor becomes unique | Observed assertion 9 failure with passing kernel, sent-retention, fresh-pair and same-session controls; successful installation at assertion 12 remains unreached |

## Files to Modify
- `internal/component/bgp/reactor/reactor_iface.go` - <to be filled>
- the Adj-RIB-Out refeed path - <to be filled>

## Files to Create
- <to be filled>

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| BGP family surface (new SAFI / capability / attribute) | | N-A, no new family |
| Prometheus counters/metrics | | a refeed counter, if one is added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 9 | RFC behavior implemented, changed, or newly proven? | | `rfc/short/rfc2545.md`, `docs/features/rfc-status.md` |
| 12 | Internal architecture changed? | | `docs/architecture/bgp/` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- <to be filled>
2. **Phase: <to be filled>**

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
| <to be filled> | <to be filled> |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | Re-read the source in Current Behavior |

## Known Limitations
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
- <to be filled>

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features (or N-A with a reason)

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes

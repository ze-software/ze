# Spec: fetched-data-survives-restart

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Priority | P1 |
| Release reason | Routing and filtering can change across a reboot with no operator action, because data fetched from the internet is fetched again at start: a startup and lifecycle correctness defect of shipped capabilities (BGP IRR import filter, firewall IRR and domain sets). |
| RFC gaps | - |
| Depends | - |
| Phase | 7/7 |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

WIP cap exception: Thomas approved this spec on 2026-11-15 and authorised starting its implementation above the in-progress cap (12 of 12 at the time), as one extra in-progress spec. The permission covers this spec only.

Implementation state (2026-11-15): phases 1 to 7 are implemented and committed, every first-closure blocker (B-1 to B-5) is cleared, and the second closure's Review Gate findings (R1-1 to R1-3) are cleared by b3c34bcb90. The third closure (`/ze-close`, 2026-11-15) appended the closure sections below and ran the Review Gate from round 1.

| Commit | What it carries |
|--------|-----------------|
| 683b848cd8 | The approved spec |
| 0f1dc02e4f | Phase 7: the domain stored answer carries `expires-at`, renewed on every answer; a start resolves a name only after its stored expiry; doctor codes for an unreadable and an unsaved answer |
| 04dc7a7e26 | Phases 1 and 2: the `irr` plugin skeleton (`system irr`, commands, keys, grants, event types), the seven wiring `.ci` files, `le mock irr --query-log`; the dataset codec, the per-key value bound (12,000,000 bytes for a dataset) and the read-only and one-instance state grants |
| 57bde3372f | Phase 7 follow-up: an unsaved DNS answer is raised on the report bus until a write succeeds (`domain/unsaved_issue.go`) |
| 8472da0c8d | Tooling: `./le test weakened approve` groups same-named declarations of one file as one unit, as the weakening gate does (`internal/le/rfc/approvalunits.go`) |
| 6e67f01e6d | Phases 3, 4 and 5 together: the owner fetches, schedules and stores; `bgp-filter-irr` and `firewall-irr` read the stored copies through `resolve/irr/store/consumer.go`; `update bgp irr`, `update firewall irr` and `clear firewall irr` deleted; `PrefixStore` persistence and `zefs.KeyIRRCache` deleted |
| 615fab7c2f | Test tooling: `le test peer` keeps each connection's sends on that connection (needed by the two-session `irr-restart-keeps-bgp-list.ci`) |
| d141740344 | Phase 6: `policy irr` augment deleted whole, `firewall irr { server; peeringdb-url; refresh-interval }` deleted; every repository `.ci`, fixture, demo and doc converted to `system irr`, `system peeringdb url`, `update irr`, `clear irr` |
| 5c42ec5f1e | B-4: A-3 (a notice in the stage-5 window is applied) and A-9 (consumers read the owner's keys with the owner stopped) proven, each red under its break; stale `clear.go` and `schedule.go` headers fixed |
| 281d62f135 | B-2: IRR relay fixtures wait for the stored list before the relay; `filter-wire-irr-two-transports` green |
| 42a8a64fc2 | AS numbers in IRR names read with `asn.Parse` in the owner and in `firewall-irr` (AS1.10 and AS65546 name one AS) |
| ec93678bd1 | B-5: AS-SET name completion for `system irr` config leaves and `update irr` / `clear irr` arguments; `irr-as-set-completion` promoted after 80/80 under stress |
| 3a486798b4 | B-1: the domain-group restart fixture reads the nft table inventory as text (`internal/test/fixture/netfilter_fixture_domain_group_inventory.go`), because the guest's nft has no JSON support |
| 3d4b4bc98d | B-1: `test/plugin/firewall-domain-group-restart-keeps-answers.ci` promoted from the draft |
| 61ea7e8f6d | B-2: `test/plugin/filter-irr-modify-mp.ci` waits for the stored list and pins dest-peer `next-hop unchanged`; no expect or reject line changed |
| b3c34bcb90 | R1-1 to R1-3: a dataset is stored under its digest, so a torn write keeps the old copy; the store's AS-SET discovery deleted; stale prose naming the consumers as client holders fixed |
| 1063ce9adb | Outside this spec, found by it: `show doctor` lists a fault a plugin check reports once |

Outside this spec, found by this work: 1063ce9adb makes `show doctor` list a fault a plugin check reports once (it listed a plugin runtime-check fault twice, from the runtime check and from recent events). The wiki command catalog was regenerated and pushed.

Also landed: 5ed7ef7d36 rewrites `firewall-irr-iface-reject`, `firewall-irr-reject` and `firewall-irr-show` to commit by reload, judged, and promotes them from drafts.

Evidence already run (session 43c0e0d8, not re-run): package tests and scoped lint green for `internal/plugins/irr`, `resolve/irr/store`, `bgp/plugins/filter_irr`, `firewall/plugins/irr`, `plugin/server`, `plugin/all`, `test/fixture`, `test/peer`, `le/rfc`, `le/test/weakened`, `le/site/terminaldemo`. Plugin suite on darwin: the 10 IRR tests that run there are green. Reload `bgp-filter-irr-added-live`, `-first-add-live`, `-loaded-over-ssh` and `irr-bgp-commit-new-set` green. Parse `source-address-irr` and ui `completion-words-irr-update` green. ExaBGP suite 42/42. In Ze's QEMU guest, 17 firewall IRR Linux tests green (`firewall-irr-*`, `irr-shared-dataset`, `irr-update-then-firewall-commit`, `irr-restart-keeps-firewall-sets`).

## Closure blockers (all cleared)

The first closure (`/ze-close`, 2026-11-15) stopped at step 1, Deliverables, on the items below. Every one is cleared (2026-11-15), and the closure ran again. The evidence logs of B-1 and B-2 are in `tmp/session/2026-11-15-125042/scratch/`.

| # | What blocked | Where it was produced | How it cleared |
|---|-------------|----------------------|----------------|
| B-1 | Cleared 2026-11-15. Wiring Test row "Domain restart with unexpired answers", Deliverable "Domain answers honour the stored expiry", User Story 5 and AC-13's end-to-end proof: `firewall-domain-group-restart-keeps-answers.ci` had never been green. In the QEMU guest it failed reading the nft inventory | Session fe3a8aa9 wrote a JSON inventory reader (`nft --json`); the guest's nft has no JSON support. Thomas chose to back that reader and its test up (`backups/fe3a8aa9-domain-inventory-20261115-202759/`) | 3a486798b4 reads the inventory as text. Green in Ze's QEMU guest (`job-qemu-domain-b3cda9c6.log`). Red under the break, `OnConfigure` restored to `return plug.configure(parseDomainConfig(sections))`: "second start did not program alpha's stored address" (`job-qemu-domain-break-b3cda9c6.log`). Stress, by Thomas's choice 10 serial guest boots on the loaded Mac in place of 80 iterations: 10/10 pass (`job-qemu-domain-s1..s7-b3cda9c6.log`, `job-qemu-domain-r8..r10-b3cda9c6.log`). Promoted by 3d4b4bc98d; the promoted run is green (`job-qemu-domain-promoted-6817eed9.log`) |
| B-2 | Cleared 2026-11-15. 281d62f135 made `filter-wire-irr-two-transports` green; `filter-irr-modify-mp.ci` still owed a green run, a stress run and its promotion | The relay red ("dest peer was never sent the re-advertised route", `job-modify-mp-118079bd.log`) came from outside this spec: ae01917413 made `next-hop auto` toward an eBGP non-rs-client mean self, and dest-peer's local and remote addresses are both 127.0.0.2, so the reactor rightly withheld the route ("withholding route: its next hop is this peer's own address", RFC 4271 Section 5.1.3) | 61ea7e8f6d adds the wait for the stored list and dest-peer `next-hop unchanged`; no expect or reject line changed. Green (`job-modify-mp3-118079bd.log`), 80/80 under stress-repro with no failure (`tmp/stress-repro/plugin-draft-pattern-filter-irr-modify-mp-ee975557...-20261115-211431-1618649825.log`), promoted and green (`job-modify-mp-live3-c8a7db17.log`). The other relay reds (`prefix-filter-modify-mp`, `community-strip`, `filter-wire-*`) belong to `plan/release/fixes/spec-bgp-next-hop-auto-rewrites-for-ebgp.md` |
| B-3 | Cleared 2026-11-15. The discrimination break was observed in Ze's QEMU guest (kernel 7.3). With `verifyStored` (`internal/component/firewall/plugins/irr/irr.go`) accepting `store.ErrNotStored`, `firewall-irr-iface-reject` went red ("output does not match regex nothing is stored for as-set AS-MISSING") and so did `firewall-irr-reject` ("... for AS99999"). Both were green before the break, and `irr.go` was restored to match HEAD | - | - |
| B-4 | Cleared 2026-11-15 by 5c42ec5f1e: A-3 and A-9 confirmed, each by a test that goes red under its break (`internal/component/plugin/server/startup_notice_window_test.go`, `dispatch_state_owner_stopped_test.go`). | - | - |
| B-5 | Cleared 2026-11-15 by ec93678bd1: completion built and proven by the promoted `test/plugin/irr-as-set-completion.ci` (red under a break of the `Complete` field). | - | - |

Not blockers, recorded so the next closure does not rediscover them:

| Item | Disposition |
|------|-------------|
| `./le test qemu netns suites plugin` refuses on darwin. On ssh host `claude` it failed at setup ("chown /tmp/zebin: operation not permitted"), because another run owns that directory | Not owed. The scoped Linux evidence is the 17 firewall IRR tests already green in the guest (`ai/META-DECISIONS.md`, "Day-to-day work is judged only by the tests of the components it changed"). B-1 still owes the domain draft's own run |
| Gitignored `test/draft/plugin/irr-restart-keeps-bgp-list.ci`, superseded by the promoted test | Deleted with Thomas's approval (2026-11-15). It was gitignored, so nothing is committed for it |
| 20 multi-connection BGP plugin tests fail identically with 615fab7c2f reverted | Other sessions' work, not this spec's |
| `cmd/ze/hub/main_system.go` and `bgp/plugins/cmd/peer/prefix_update.go` construct IRR and PeeringDB clients outside `internal/plugins/irr` | Not a second fetcher. The first serves the daemon form of `ze resolve irr` (`resolve/cmd/resolve.go`). The second is the PeeringDB max-prefix update, which this spec leaves out of scope |
| Stale comments seen at the closure read, for the Review Gate to fix: `resolve/irr/store/clear.go` header names "bgp-filter-irr and firewall-irr" as holding clients; `firewall/plugins/domain/schedule.go` header says "firewall-irr refreshes every reference on one `refresh-interval`" | Fixed in 5c42ec5f1e |

## Second-closure blockers (all cleared)

The second closure (2026-11-15) ran Review Gate round 1 over the implementation commits listed above, read from source, and stopped on R1-1: its fix changed the stored dataset key the "Stored values and notices" table declares, so it was implementation work (`/ze-implement`), with its own test. R1-2 and R1-3 rode with it. All three are cleared by b3c34bcb90 (2026-11-15): datasets are keyed `meta/irr-data/{server}/{as-set}/{digest}`, `internal/plugins/irr/datasets.go` `removeTorn` and `removeUnpointed` remove what no pointer names (`TestTornPointerWriteKeepsOldCopy`, `TestStartRemovesUnpointedDatasets`, each red under its break: overwriting the old key went red on `ErrDatasetNotPointed`, dropping `removeTorn` on "want only the pointed one", an early return in `removeUnpointed` on "start kept ..."); `store.PrefixStore.Refresh(ctx, asSet)` with the store's discovery deleted and its five tests removed under `ze-test-judge` approval (`Weakening-approved:` trailers of b3c34bcb90); the stale prose names the `irr` plugin. The third closure ran the gate again from round 1 over everything (see "Review Gate"). The rows below keep the findings as they were recorded.

| # | Severity | Finding | Where it was produced | What cleared it (b3c34bcb90 took the [source] route) |
|---|----------|---------|-----------------------|----------------|
| R1-1 | ISSUE | A refresh from an unchanged server is not an atomic switch. The dataset is written under the key the current pointer already names, before the new pointer is written. When the generation increment or the pointer write then fails, the store keeps the new dataset under the old pointer, and every reader refuses it as `ErrDatasetNotPointed` (the digests differ). At the next start both consumers enforce no list for that AS-SET until the owner fetches it again: with the IRR server unreachable at that boot, BGP rejects the peer's announcements and the firewall holds its table back, although a valid dataset is stored. That breaks the Task's goal, a reboot enforcing what it enforced before. In normal operation a reader between the two writes (a firewall commit verify, a consumer's `OnStarted` read, a generation-gap re-read, the post-config doctor check) gets the same refusal: a refused commit or a false `stored-copy-unreadable` WARN. The Design Insight "a pointer written last gives an atomic switch" holds only across a server change | `internal/plugins/irr/datasets.go` `writeDataset` puts the value at `store.Dataset.Key()` = `meta/irr-data/{server}/{as-set}` (`internal/component/resolve/irr/store/dataset.go`), the key `store.Pointer.DatasetKey` (`pointer.go`) gives the current pointer when the server is the same. `TestPointerFlipsAfterWrite` (`internal/plugins/irr/irr_test.go`) fails only the dataset write, never the pointer write after it | [source] Give each written dataset a key of its own. One way: raise the generation first and store under `meta/irr-data/{server}/{as-set}/{generation}`. The pointer names that key. The previous dataset is removed after every flip, not only after a server change. `load` removes any dataset key no pointer names. `DecodeDataset`'s identity check, `Pointer.DatasetKey`, `zefs.KeyIRRDataset`, the "Stored values and notices" and "Changed-Wire Census" rows, `docs/architecture/irr-owner.md` and `docs/architecture/zefs-format.md` follow. New test: the pointer write fails after the dataset write, and the store still serves the old copy under the old pointer, read by a consumer reader; red today. [workaround, rejected] Consumers accept a dataset whose own digest passes when the pointer's differs: that enforces a copy the owner recorded as unpublished, and it drops the pointer contract instead of removing the torn write. The agreed design (Key Design Decisions, "Pointer `meta/irr-set/{as-set}` flipped after the dataset write", whose reason is that "the pointer keeps the old copy enforced") calls for the source fix, so this is no owner question; the key pattern is `pkg/zefs`, a public contract, so the change carries its pages |
| R1-2 | ISSUE | Layering: AS-SET discovery for a bare ASN exists twice. The owner discovers through its own PeeringDB client and copies the store's fallback rule ("as the store did"). The store's discovery path is reached only by tests, because the owner always passes the AS-SET | Production: `internal/plugins/irr/owner.go` `discoverASN`, through `resolver.go` `storeResolver.discover`. Dead: `internal/component/resolve/irr/store/store.go` `discoverASSet`, `lookupASSet`, `parseBareASN`, and the empty-`asSet` branch of `Refresh`/`resolve`. The only production caller is `resolver.go` `refresh`, `Refresh(ctx, asSet, asSet)`. `store_test.go` calls `Refresh(..., "")` | Delete the store's discovery path, so `Refresh` takes the AS-SET it queries. Remove the `store_test.go` cases that drive that path: the owner's `discovery_test.go` covers discovery. Removing a test is judged by `ze-test-judge` (`ai/META-DECISIONS.md`, "Test changes") |
| R1-3 | ISSUE | Stale prose names `bgp-filter-irr` and `firewall-irr` as holders of IRR and PeeringDB clients, or as callers of the store. Only the `irr` plugin holds them (`internal/plugins/irr/irr.go` `runIRR`, `OnResolverCacheClear`) | `internal/component/resolve/irr/store/store.go`: the `commit` doc ("bgp-filter-irr cancels the lookups a reload superseded") and `markStale` ("bgp-filter-irr refreshASN", "firewall-irr keptDataError"). `clear.go` `clearPeeringDBClient` doc ("firewall-irr before its first configure"). `internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang`: the `clear peeringdb` and `clear irr cache` descriptions ("such as bgp-filter-irr and firewall-irr, clears its own"). `docs/guide/command-reference.md`, "clear peeringdb cache / clear irr cache / clear cymru cache" ("`bgp-filter-irr` and `firewall-irr` hold an IRR and a PeeringDB client"). `docs/architecture/resolve.md`, the `commit` paragraph ("bgp-filter-irr cancels the lookups a reload superseded") | Name the `irr` plugin. Say what the store's cancellation check now guards: the owner's lookups end with the plugin's context, at stop. A changed YANG description also reaches the generated help and site pages that copy it |

NOTE, not blocking: `internal/le/site/testdata/published-command-paths.txt` and `published-configuration.md` still list `update bgp irr`, `update firewall irr`, `clear firewall irr` and the old `peeringdb-url` and `refresh-interval` leaves. They are frozen site-generator fixtures from aa6ebb8d15 (2026-11-11), not a page an operator reads.

That round 1 was not finished: the firewall reader (`firewall/plugins/irr/stored.go`), the owner's `completion.go`, `register_commands.go` and `metrics.go`, and the tooling and fixture files were not read, and no `./le spec review record` was written. The third closure read them in its own round 1.

## Task

Every piece of metadata Ze downloads from the internet and uses to decide
routing or filtering is saved in its own stored file, loaded at start instead
of being fetched again, and refreshed only at the frequency the operator set in
the configuration. A reboot then leaves the router enforcing exactly the data
it enforced before. The configuration stays small: the data lives beside it,
not inside it as the long prefix lists or route maps of a Junos or Cisco
configuration.

Thomas's words (2026-11-15): "What we've done with only having the list we
extract from the registries in memory is not allowing the behavior to be
deterministic. As on restart a new fetch will automatically happen you cannot
be sure that the router configuration you had before and after a reboot is the
same. That's why the command have rules for when we can update so the behavior
can be predictable. We should save in their own file every of the metadata we
downloaded from the internet and only update them at the frequency the user has
requested via the configuration. Having the metadata outside the file, instead
of in the case of Junos or Cisco, very long prefix list or routemap, keeps the
configuration small but still ensures that the data used will be the same on
reboot."

Scope decisions Thomas took at the scope gate (2026-11-15):

| Question | Decision |
|----------|----------|
| Where the work lives | A new release spec for determinism. `plan/post-release/features/spec-irr-apply-policy.md` keeps apply windows and held changes, and is amended to depend on this spec |
| Firewall domain (DNS) sets | Same rule: the stored answers load at start, and a name is resolved again only when its stored TTL has expired |
| First start with no stored copy | Fetch at once and store the result; afterwards only the configured interval or a manual update refreshes it |
| BGP and firewall using the same AS-SET | One shared stored copy |

Design decisions Thomas took at the design gate (2026-11-15):

| Question | Decision | His words |
|----------|----------|-----------|
| Who fetches, schedules and stores IRR data | One owner (approach B). `bgp-filter-irr` and `firewall-irr` only read what it stores | Chose B at the design gate |
| Where the refresh timing lives | On the plugin that fetches, so the owner is a plugin and the `refresh-interval` leaves leave both consumers | "We should move the timing to the plugin which will implement the fetching." |
| A newly configured AS-SET with nothing stored | The firewall and domain commit keep refusing it (verify does no network I/O); the operator fetches with an update command | Kept the refusal at commit |
| DNS stored expiry | Renewed on every re-resolution; the hourly cap is rejected. `firewall-domain` resolves DNS itself, so it stays its own fetcher and keeps its timing | Rejected the hourly cap |
| Where the owner's config lives (Q-5) | `system irr { server; source-address; refresh-interval }`, beside `system peeringdb` | "system irr" (beside system peeringdb) |
| Default refresh interval of the merged setting (Q-6) | `refresh-interval` default 3600 seconds; 0 means manual only (no automatic refresh; a cold start still fetches once, and `update irr` fetches on demand) | Default 3600, 0 = manual only |
| `clear irr as-set X` on a set still referenced (Q-7) | Removes the stored copy; the reply names every consumer still referencing it | Removes the stored copy, the reply names the consumers |

This reverses the owner decision of 2026-10-17 that the BGP IRR prefix-lists
live in memory only (`docs/architecture/bgp/filter-irr.md`). Its recorded
reason, a state write that expired the first-resolution wait, no longer holds:
that wait was replaced by reject, hold and re-admit.

RPKI is out of scope. RFC 8210 Section 6, Expire Interval, says: "The router
MUST NOT retain the data past the time indicated by this parameter."
(`rfc/full/rfc8210.txt`, read at Section 6.)

### Which fetched data decides routing or filtering

| Data | Fetched by today | Fetched by after this spec | Decides | In scope | Reason |
|------|------------------|----------------------------|---------|----------|--------|
| IRR AS-SET prefix lists | `bgp-filter-irr` (memory only) and `firewall-irr` (stored), each on its own | the new `irr` plugin, once per (server, AS-SET) | BGP import verdicts, firewall set contents | Yes | Fetched again at every BGP start and reload; two fetchers, two schedules, colliding keys |
| AS-SET discovered from PeeringDB for a bare ASN | the shared `PrefixStore` (`discoverASSet`) inside each consumer | the `irr` plugin, stored as a binding | Which AS-SET is queried | Yes | Asked again at every refresh, so a restart can query a different set |
| DNS answers for firewall domain groups | `firewall-domain` | `firewall-domain` (unchanged owner) | Which addresses a set holds | Yes | Every name is due at start (`schedule.reset`) |
| PeeringDB max-prefix | `ze bgp peer * prefix update` | unchanged | Prefix maximum | No | Written into the config, so already deterministic |
| RIR delegation table, Team Cymru | resolve component | unchanged | Diagnostics only | No | Drives no routing or filtering decision |
| RPKI VRPs | `bgp-rpki` | unchanged | Origin validation | No | RFC 8210 Section 6, quoted above |

### What the operator sees, before and after (departure from documented design)

This spec departs from four recorded design statements, each corrected in the
same work: `docs/architecture/bgp/filter-irr.md` "The prefix-lists live in
memory only" (owner ruling 2026-10-17) and its "`resolve/irr` is imported
directly" fetch-in-the-filter decision; `docs/architecture/core-design.md`
section 22 ("The list is held in memory only, so a restart resolves it again
from the source"); `docs/architecture/firewall/firewall-irr.md` "The PrefixStore
code is shared with BGP, the cache is not" and "Auto-refresh is off by
default"; `docs/architecture/firewall/firewall-domain-group.md` "What earns a
write" ("The steady state writes nothing").

| Situation | Before | After |
|-----------|--------|-------|
| Router reboots, IRR server reachable | `show bgp irr` shows every ASN pending; announcements are rejected and held until each ASN's whois answers; the lists can differ from the ones before the reboot | `show bgp irr` shows the same AS-SETs, counts and `refreshed-at` as before the reboot; no whois query is sent at start; announcements are judged against the stored lists at once |
| Router reboots, IRR server unreachable | Every IRR-filtered peer's announcements stay rejected until the server answers | The stored lists are enforced; a refresh that is due runs at its time and retries while it fails |
| Where the operator sets the IRR server, source address and refresh period | Twice: `policy irr { server; source-address; peeringdb-url; refresh-interval }` for BGP and `firewall irr { server; peeringdb-url; refresh-interval }` for the firewall, with different defaults (3600 and 0) | Once, `system irr { server; source-address; refresh-interval }` (default 3600, 0 = manual only); the PeeringDB URL once, `system peeringdb url`; a consumer names only the AS-SET or ASN it enforces |
| Reload that changes nothing IRR | `bgp-filter-irr` resolves every ASN again; `firewall-irr` restarts its refresh timer | No query; the next refresh time is unchanged |
| BGP commit adding a peer whose AS-SET is stored | Fetched again | The stored copy is enforced at once, no query |
| BGP commit adding a peer whose AS-SET is not stored | Fetched by the filter at once | The commit succeeds; the `irr` plugin fetches it right after the commit; until it is stored, the peer's announcements are rejected, held and re-asked by ROUTE-REFRESH once it arrives |
| Firewall commit naming an AS-SET not stored | Refused: "run 'update firewall irr as-set X' first" | Refused the same way, naming `update irr as-set X` |
| First start ever | BGP fetches every ASN; the firewall fetches nothing and holds the table back with a WARN per reference | The `irr` plugin fetches every referenced dataset at once and stores it; each consumer enforces it when the notice arrives |
| Firewall start with stored data | A false "no cached prefix data" WARN per reference, then the table programmed | No WARN; the table is programmed from the stored copy |
| BGP and firewall name the same AS-SET | Two fetches, two schedules, one stored copy (the firewall's) | One fetch per refresh period, one stored copy, both consumers enforce it |
| `update bgp irr asn 65001`, `update firewall irr as-set AS-X`, `clear firewall irr as-set AS-X` | Consumer commands that fetch or purge inside the consumer | `update irr asn 65001`, `update irr as-set AS-X`, `update irr all`, `clear irr as-set AS-X` on the `irr` plugin; the consumer commands are deleted |
| A domain-group name whose stored answer has not expired | Resolved again at start | Not resolved until its stored expiry |
| Domain-group store writes for a name answering the same addresses every 60 s | One write, ever ("The steady state writes nothing") | One write per answer, renewing the stored expiry: up to 1,440 a day per name and family at the 60 s floor (Thomas's choice) |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/bgp/filter-irr.md` - the BGP consumer and the 2026-10-17 ruling this spec reverses
  → Decision: the memory-only ruling is reversed (2026-11-15) and the filter stops fetching: its section "The prefix-lists live in memory only", its "A restart therefore starts from no list" paragraph and its "`resolve/irr` is imported directly" decision are rewritten in the same work.
  → Constraint: the evaluator reads one immutable `irrSnapshot` swapped under `plug.mu` (`publishLocked`); a list read from the owner's store MUST publish through the same `publishLocked`.
  → Constraint: a changed or deleted configured AS-SET is never enforced under the old set (`carriedOverBy`); after this spec the consumer reads the new set's stored copy or holds no list, it never keeps the old set's list.
  → Constraint: the readmitter re-asks peers after every publish (`readmitter.notify`); it stays, minus its fetch.
- [ ] `docs/architecture/core-design.md` section 22 - sources and read views
  → Decision: the `irr` plugin is the builder: the only component that knows the source's model, it pulls from IRR and PeeringDB and writes the read view; consumers pull it when a notice says "pull now". This is section 22 applied literally.
  → Constraint: "A pull that fails keeps the last good read view": a failed refresh leaves the stored copy in place.
- [ ] `docs/architecture/resolve.md` "IRR Prefix Store", "PeeringDB Prefix Data", "Clearing a resolver cache"
  → Constraint: per-family last-known-good (`commit`, `markStale`) moves into the owner unchanged.
  → Decision: "The settings live in `system { peeringdb { } }`, not under `bgp`. PeeringDB is an external service, not a BGP concept, and another subsystem can use it later." The owner reads `system peeringdb url`; the two consumer `peeringdb-url` leaves are deleted, so the endpoint is declared once.
  → Constraint: `clear irr cache` and `clear peeringdb cache` reach the owners of resolver clients through `OnResolverCacheClear`; after this spec only the `irr` plugin holds IRR and PeeringDB clients, so only it declares those caches.
- [ ] `docs/architecture/firewall/firewall-irr.md` - the firewall consumer
  → Constraint: `failure-policy: restart` relies on the stored copy to reprogram sets after a crash; the consumer now reads the owner's copy, which survives the consumer's crash as before.
  → Constraint: the commit refusal of an uncached reference stays (Thomas, design gate).
- [ ] `docs/architecture/firewall/firewall-domain-group.md` - the domain consumer and fetcher
  → Constraint: one worker, one timer, the unit is a name in a family; `firewall-domain` keeps fetching for itself.
  → Decision: "What earns a write" gains a fourth write, the expiry renewal on every answer (Thomas, design gate); the page's "The steady state writes nothing" and "A name answering the same addresses every 60 seconds matches none of them" are rewritten.
- [ ] `docs/architecture/api/process-protocol.md` "Persistent runtime state", startup stage 5, `emit-event`
  → Constraint: state RPCs work only after the handshake (`OnStarted` or later); a value carries at most `rpc.StateDataMax` (1 MiB), checked by `rpc.ValidateStateInput` on the SDK side (`sdk_state.go` `stateCall`) and the daemon side (`dispatch_state.go` `stateInput`, `opStateGet`), modelled as `binary { length "0..1048576" }`; a key at most 512 bytes of `[A-Za-z0-9_.:/-]`; a list at most 4096 keys.
  → Constraint: grants are per owner and grant read and write alike (`permissions.go`); a consumer reading the owner's key without being able to write it needs a read-only grant, which does not exist yet.
  → Constraint: `ReadyInput.subscribe` and `subscriptions` register a plugin's event subscriptions in stage 5, "before any event flows" (`sdk.Plugin.AddStartupSubscriptionsIn`); a subscription delivers future events only, so a consumer subscribes at stage 5 and reads in `OnStarted`, which leaves no gap.
  → Constraint: `emit-event` (`sdk.Plugin.EmitEvent`) reaches subscribers in the daemon and forked, the route `bgp-adj-rib-in` already uses for `bgp-rib/validation-change`.
  → Constraint: a message is bounded by `rpc.MaxMessageSize` (16 MiB); a value travels base64, so the declared dataset bound is 12,000,000 bytes (16,000,000 after base64).
- [ ] `docs/architecture/module-tiers.md` "The three tiers", "Plugin import ownership"; `ai/doors/system-plugins.md` "Where it sits"
  → Decision: a config-driven engine nothing else imports is an edge plugin under `internal/plugins/`, a full-subsystem plugin that owns its runtime, config and state; the consumers reach the owner only through stored keys, events and commands, so the owner is `internal/plugins/irr/`. The dataset codec both sides use stays in the resolve component (`internal/component/resolve/irr/store`), which both consumers already import.
  → Constraint: "A plugin owns its whole surface": the owner's commands, YANG, doctor checks and help live in its folder.
- [ ] `docs/architecture/zefs-format.md` "Key Registry" and the in-place write table
  → Constraint: an added or removed key, and a value that outgrows its slot, rewrite the whole store; a same-size value is written in place.
- [ ] `plan/journal/startup-wait-expires-under-load.md` 2026-10-17 row
  → Constraint: a state write took the SDK's full 5 s under disk load. With one owner, a consumer can publish only what the owner already wrote, so a new list reaches the evaluator after the write and the notice; nothing waits on that any more (reject, hold and re-admit replaced the wait), and R-1 bounds the delay.
- [ ] `plan/post-release/features/spec-irr-apply-policy.md` owner decisions Q-1 and Q-5 (that spec's numbering, not this spec's)
  → Decision: apply-policy Q-5 (2026-10-08) "A holding consumer accepts the other consumer's fetches: Yes, sharing kept". With one owner every consumer enforces each stored copy.
  → Constraint: that spec's A-1 ("today `meta/irr/{name}` is already shared regardless of server") is false today and superseded; its amendment is tabled below.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc8210.md` - RPKI to Router, cited for the exclusion only
  → Constraint: RFC 8210 Section 6, Expire Interval: "The router MUST NOT retain the data past the time indicated by this parameter."
- [ ] `rfc/full/rfc1035.txt` Section 3.2.1 - DNS TTL (no `rfc/short/` summary is owed: the spec implements no RFC 1035 requirement, it cites the TTL definition the domain schedule already follows)
  → Constraint: RFC 1035 Section 3.2.1: "TTL a 32 bit signed integer that specifies the time interval that the resource record may be cached before the source of the information should again be consulted." A stored answer is used at start only while its stored expiry has not passed.

**Key insights:** (minimal context to resume after compaction)
- One writer: the `irr` plugin (`internal/plugins/irr/`) fetches, schedules and stores; `bgp-filter-irr` and `firewall-irr` read its keys through a read-only state grant and learn of a new copy from its `emit-event` notice.
- One place says where and when: `system irr` holds `server`, `source-address`, `refresh-interval` (default 3600, 0 = manual only); the PeeringDB URL is `system peeringdb url`. A consumer names only the AS-SET or ASN it enforces. No consumer can name its own server today (one `server` leaf per consumer, none per peer or rule), so nothing is lost.
- The owner learns what to fetch from each consumer's stored interest key (`meta/irr-interest/{consumer}`), written by the consumer and read by the owner, with a notice on change: it survives either side restarting and works forked.
- A consumer reads the per-AS-SET pointer `meta/irr-set/{as-set}` (which dataset is current), then the dataset `meta/irr-data/{server}/{as-set}/{digest}` it names; the owner flips the pointer only after a new dataset is stored, so a server change never leaves a consumer with nothing.
- A monotonic generation (`StateIncrement` on `meta/irr-generation`) rides on every notice; a consumer that sees a gap re-reads every pointer, so a lost notice is recovered at once and a restart cannot change data.
- The schedule is anchored on a stored `checked-at`, never `refreshed-at`, which dates the oldest prefixes and would make a one-family set due every tick.
- A dataset is one key, compact binary, with a per-key value bound derived from the consumer caps, under `MaxMessageSize`.
- Daemon stop sends no `bye`; nothing can be written at stop.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/resolve/irr/store/store.go` - `PrefixStore`: `New`, `UsePersistence`, `UseClients`, `Get`, `Refresh` (= `refreshLive` then `persist`), `commit` (per-family last-known-good, same-AS-SET merge, context check under `s.mu`), `markStale`, `Purge`, `resolve` (PeeringDB discovery, `RefreshPrefixes` always hits the server), `discoverASSet`, `Open` (migrates `meta/bgp/irr-cache`, lists `meta/irr/`), `persist` (JSON `CachedEntry` per name).
- [ ] `internal/component/resolve/irr/store/clear.go` - `ClearResolverCache` clears the store's client caches; prefix entries stay.
- [ ] `internal/component/bgp/plugins/filter_irr/filter_irr.go` - `handleConfigure` builds `store.New(irrClient, pdb, nil)` once; `OnStarted` starts the readmitter, `initialResolve` and `refreshLoop`; every configure after `started` runs `initialResolve` again and restarts `refreshLoop` from now; `refreshASNCtx` keys the store by `AS<asn>` with the configured as-set as hint; `OnResolverCacheClear` declares the irr and peeringdb caches.
- [ ] `internal/component/bgp/plugins/filter_irr/readmit.go` - `readmitSource.resolveHeld`; `retryResolution` fetches every `readmitRetryInterval` (1 min) for a held ASN with no list or a failed lookup.
- [ ] `internal/component/bgp/plugins/filter_irr/command.go` - `update bgp irr all|asn|as-set` fetch through `refreshAllNow`/`refreshASN`; `show bgp irr` reports `server`, `last-refresh`, `next-refresh` and per-ASN status.
- [ ] `internal/component/bgp/plugins/filter_irr/config.go` - `parseIRRConfig` reads `policy irr { server; peeringdb-url; source-address; refresh-interval }` and the enrolled peers; defaults `whois.radb.net`, `https://www.peeringdb.com`, 3600.
- [ ] `internal/component/bgp/plugins/filter_irr/register.go` - `ConfigReads` `policy`, `Dependencies` `bgp` and the policy plugin; no state grant.
- [ ] `internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr.yang` - augments the BGP session with `irr { as-set; enable }` and `/policy:policy` with `irr { server; peeringdb-url; source-address; refresh-interval 60..86400 default 3600 }`.
- [ ] `internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr-cmd.yang` - `show bgp irr [prefix|check]`, `update bgp irr all|asn|as-set`.
- [ ] `internal/component/firewall/plugins/irr/irr.go` - handshake `OnConfigure` with no persistence warns for every reference and applies tables from an empty cache; `OnStarted` binds `StateKeys`, reconfigures, starts `refreshLoop` when `refresh-interval` > 0; `verifyRefs` refuses an uncached reference; `refreshName` fetches and applies; `configureStore`'s comment claims the BGP filter's refreshes reach the shared key space (false).
- [ ] `internal/component/firewall/plugins/irr/command.go` - `update firewall irr all|asn|as-set` fetch; `clear firewall irr asn|as-set` purge and re-apply; `show firewall irr` and `show firewall irr prefix`.
- [ ] `internal/component/firewall/plugins/irr/yang/ze-firewall-irr-cmd.yang` - the `show`, `update` and `clear firewall irr` trees.
- [ ] `internal/component/firewall/plugins/irr/register.go` - grants `firewall-irr` `meta/irr/{name}` and `meta/bgp/irr-cache`; registers `firewall-irr-data-freshness`.
- [ ] `internal/component/firewall/plugins/irr/sets.go` - `maxPrefixesPerFamily` 500,000; `oversizedEntryError` comment names "the BGP IRR filter writing the same shared entry" (false today).
- [ ] `internal/component/firewall/plugins/irr/doctor.go` - `checkIRRDataFreshness` opens the store read-only and reports no-data, oversized, stale.
- [ ] `internal/component/firewall/plugins/irr/yang/ze-firewall-irr.yang` - `firewall irr { server; peeringdb-url; refresh-interval 0|60..86400 default 0; interface { name; source-as-set } }` plus the term `source-asn`, `source-as-set`, `destination-asn`, `destination-as-set` leaves; line 24 claims the BGP filter shares the store (false).
- [ ] `internal/component/config/system/yang/ze-system-conf.yang` - `system peeringdb { url; margin }`.
- [ ] `internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang` - `ze resolve irr expand|prefix` operational commands; no stored-data command.
- [ ] `internal/component/firewall/plugins/domain/schedule.go` - `schedule.reset` makes every unit not already scheduled due now, so every name resolves at start.
- [ ] `internal/component/firewall/plugins/domain/cache.go` - `resolvedName` has no TTL or expiry; `open` reads by configured name.
- [ ] `internal/component/firewall/plugins/domain/domain.go` - `configure` programs stored answers before any resolve; `resolveAndRecord` writes only on an address change, a first answer, a failing flip.
- [ ] `internal/component/firewall/plugins/domain/doctor.go` - no-data and stale codes from the stored answers.
- [ ] `internal/core/statestore/permissions.go` - `RegisterPluginKeys` grants read and write together, per owner.
- [ ] `pkg/zefs/keys.go` - `KeyEntry{Pattern, Description, Private}`; `KeyIRRCache`, `KeyIRRPrefixCache`, `KeyFirewallDomainGroup`.
- [ ] `pkg/plugin/rpc/state.go`, `pkg/plugin/sdk/sdk_state.go`, `internal/component/plugin/server/dispatch_state.go`, `internal/core/ipc/yang/ze-plugin-engine.yang` - the 1 MiB value bound on both sides and in YANG.
- [ ] `pkg/plugin/rpc/framing.go` - `MaxMessageSize` 16 MiB.
- [ ] `docs/guide/configuration.md` around line 1894 - claims BGP resolved prefixes are stored and survive restarts (false today).

**Behavior to preserve:**
- The evaluator never waits and reads one snapshot; an ASN with no list rejects its announcements, keeps withdrawals and holds the prefixes; the readmitter re-asks peers after a publish.
- Per-family last-known-good, `StaleSince`, and refusal of a list over the caps (BGP 500,000 total, firewall 500,000 per family).
- A changed or deleted configured AS-SET is never enforced under the old set.
- Startup never depends on IRR or DNS reachability: no fetch inside the handshake.
- The firewall and domain commit refusal of a reference with nothing stored (Thomas, design gate).
- Domain groups: per-name, per-family TTL schedule, `ttl-floor`, one-day ceiling, stored answers programmed before any resolve, the change log.
- `clear irr cache` and `clear peeringdb cache` leave the stored datasets.
- `ze resolve irr ...` offline commands.

**Behavior to change:**
- A new `irr` plugin fetches, schedules and stores every IRR dataset and AS-SET binding; the consumers stop fetching and read its store.
- `server`, `source-address` and `refresh-interval` move from `policy irr` and `firewall irr` to `system irr` (augmented into the system root by the `irr` plugin's YANG); `refresh-interval` defaults to 3600 and 0 means manual only; both `peeringdb-url` leaves go, replaced by `system peeringdb url`.
- `clear irr as-set|asn` removes the stored copy and names the consumers still referencing it.
- Neither start nor reload fetches a dataset that has a stored copy; a refresh runs when `checked-at` plus the interval has passed.
- A dataset with no stored copy is fetched at once by the owner, at start and right after a BGP commit that names it.
- `update ... irr` and `clear ... irr` move to the owner's command set.
- The readmitter and both `refreshLoop`s stop fetching (deleted as fetch paths).
- Domain groups: the stored answer carries `expires-at`, renewed on every answer; a name whose stored expiry has not passed is not resolved at start.
- The firewall's handshake configure no longer warns or applies from an empty cache.
- The pages, YANG descriptions and comments that say otherwise.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| PrefixStore resolution and last-known-good | `internal/component/resolve/irr/store/store.go` `resolve`, `commit`, `markStale`, `discoverASSet` | Each consumer resolves and merges per family | Extends: runs only inside the `irr` plugin; the logic is unchanged |
| PrefixStore persistence | `store.go` `persist`, `Open`, `Purge`, `migrate` | JSON per name under `meta/irr/{name}`, listed at open | Replaces: pointer, dataset, binding and generation keys written by the owner alone; consumers read by name; `migrate` deleted after one conversion |
| Sources and read views | `docs/architecture/core-design.md` section 22 | Pattern, with the IRR filter as its example | Extends: the owner is the builder, the consumers pull the read view on a notice |
| Edge plugin placement | `docs/architecture/module-tiers.md` "The three tiers"; `ai/doors/system-plugins.md` | An engine nothing imports lives under `internal/plugins/` | Extends: the owner is `internal/plugins/irr/`, registered from its own `register.go` |
| Plugin state RPCs | `pkg/plugin/sdk/state_keys.go` `Plugin.StateKeys`; `dispatch_state.go` | Read and write registered keys after the handshake | Extends: the owner writes; consumers read through it |
| State key grants | `internal/core/statestore/permissions.go` `RegisterPluginKeys`, `PluginKeyAllowed`, `PluginKeyOwners` | One grant kind, read and write | Extends: a read-only grant (`RegisterPluginReadKeys`) for the consumers; `PluginKeyOwners` keeps naming writers only |
| State value bound | `pkg/plugin/rpc/state.go` `ValidateStateInput`; `sdk_state.go` `stateCall`; `dispatch_state.go` `stateInput`, `opStateGet`; `ze-plugin-engine.yang` `state-data` | One global 1 MiB bound | Extends: the bound a key declares on its `KeyEntry`, default 1 MiB |
| ZeFS key registry | `pkg/zefs/keys.go` `MustRegister`, `KeyEntry` | Key patterns, descriptions, privacy | Extends: new patterns with declared bounds; `KeyIRRCache` and `KeyIRRPrefixCache` removed. → Decision (implementation): `KeyIRRCache` is deleted (6e67f01e6d); `KeyIRRPrefixCache` (`meta/irr/{name}`) stays registered, described as legacy, because the owner's one-time conversion reads and removes those keys through a write grant on it (`internal/plugins/irr/register.go`, `legacy.go`) |
| Startup subscriptions | `process-protocol.md` stage 5; `sdk.Plugin.AddStartupSubscriptionsIn` | Subscriptions registered before any event flows | Extends: each consumer subscribes to the owner's notice at stage 5, then reads in `OnStarted` |
| Plugin event emission | `sdk.Plugin.EmitEvent`, `emit-event` | A plugin publishes to subscribers in process and forked | Extends: the owner's dataset notice and the consumers' interest notice |
| Atomic counter | `sdk.Plugin.StateIncrement` | Guarded read, increment, write | Extends: the owner's generation counter carried on each notice |
| BGP refresh loop and initial resolve | `filter_irr.go` `refreshLoop`, `refreshAll`, `refreshAllNow`, `initialResolve`, `refreshASN`, `refreshASNCtx` | Fetch at start, at reload, on a ticker | Replaces: deleted; the consumer reads and publishes on a notice |
| firewall refresh loop | `irr.go` `refreshLoop`, `refreshAll`, `refreshAllNow`, `refreshName` | Fetch on a ticker and on command | Replaces: deleted |
| Readmitter fetch | `filter_irr/readmit.go` `retryResolution`, `readmitSource.resolveHeld` | Fetches every minute for a held ASN | Replaces: the fetch half is deleted; retrying a failed refresh is the owner's schedule |
| Consumer IRR commands | `filter_irr/command.go`, `firewall/plugins/irr/command.go`, `ze-filter-irr-cmd.yang`, `ze-firewall-irr-cmd.yang` | `update bgp irr ...`, `update firewall irr ...`, `clear firewall irr ...` fetch or purge in the consumer | Replaces: `update irr ...` and `clear irr ...` on the owner; the consumer `update` and `clear` verbs are deleted; `show` verbs stay on the consumers |
| Resolver cache clear owners | `store.ResolverCaches`, `OnResolverCacheClear` in both consumers | Each consumer answers for its clients | Extends: only the owner holds clients and declares the caches |
| PeeringDB settings | `system peeringdb url` (`ze-system-conf.yang`); `resolve.md` "PeeringDB Prefix Data" | Used by `prefix update`; the IRR consumers carry their own `peeringdb-url` | Extends: the owner reads it; the two consumer leaves are deleted |
| IRR settings containers | `ze-filter-irr.yang` `policy irr`; `ze-firewall-irr.yang` `firewall irr` | Two server, URL and interval declarations | Replaces: one owner container, `system irr`, augmented into the system root by `internal/plugins/irr/yang/`; `firewall irr interface` and the per-peer `irr { as-set; enable }` stay with their consumers |
| Domain schedule and cache | `firewall/plugins/domain/schedule.go` `reset`, `arm`; `cache.go` `resolvedName`, `put` | Units due now at start; no stored expiry; steady answers write nothing | Extends: stored `expires-at` renewed on every answer; `reset` arms at it |
| Doctor checks | `firewall/plugins/irr/doctor.go` `checkIRRDataFreshness`; `firewall/plugins/domain/doctor.go` | Stored-data freshness per reference | Extends: the owner gains a check over datasets and schedule; the firewall check reads pointers |
| Fetched-vs-applied split (planned) | `plan/post-release/features/spec-irr-apply-policy.md` | Plans `meta/irr-applied/<consumer>/{name}` | Does not apply here: amended to sit on this spec's pointer and dataset keys |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Daemon start: stage 5 subscriptions, then each plugin's `OnStarted`.
- Config commit: `system irr` and `system peeringdb url`; a consumer's references (BGP peers' `irr as-set`, remote ASN; firewall terms and interface bindings).
- The owner's schedule coming due.
- Operator commands `update irr all|asn|as-set`, `clear irr asn|as-set`, `update firewall domain-group`, `clear firewall domain-group`.

### Transformation Path
1. Each consumer, at stage 5, subscribes to the owner's dataset notice; the owner subscribes to the consumers' interest notice.
2. Consumer `OnStarted`: writes its interest key (the AS-SETs and bare ASNs it enforces), emits an interest notice, then reads for each reference: the ASN binding (bare ASN), the AS-SET pointer, the dataset it names; publishes what it found (BGP `publishLocked`, firewall `applyTables`) and records the generation it saw.
3. Owner `OnStarted`: reads its config, the generation, every interest key (one listing of `meta/irr-interest/`, one key per consumer), every pointer, binding and dataset named; arms each dataset at `checked-at` plus `refresh-interval`; a reference with no pointer is due now (cold start).
4. Owner refresh: discover the AS-SET if needed (stored binding updated on a PeeringDB answer), fetch, commit per family, write the dataset, flip the pointer, increment the generation, emit the notice naming AS-SET, server, generation and digest.
   → Decision (implementation): the generation is raised before the pointer write, so the pointer carries the generation of its own write. `internal/plugins/irr/datasets.go` `writeDataset` runs dataset `StatePut`, `StateIncrement` on `meta/irr-generation`, pointer `StatePut` naming that generation, removal of the old server's dataset, then the notice. The step above lists flip-then-increment.
5. Consumer notice: read the pointer and dataset for a set it enforces and publish; if the notice's generation is more than one past the last it saw, re-read every pointer it uses.
6. Failed fetch: the stored copy and pointer stay; `checked-at` is unchanged; the owner retries the dataset every minute until the server answers.
7. BGP commit naming a set with no pointer: the consumer writes its interest and emits the notice after apply; the owner fetches at once; meanwhile announcements are rejected, held and re-asked after the publish.
8. Firewall or domain commit naming a set with nothing stored: refused at verify, naming `update irr as-set <s>` (or `update firewall domain-group <g>`).
9. Domain: `configure` programs stored answers; `schedule.reset` arms each unit at its stored `expires-at`, due now only without one or when it passed; every answer rewrites `expires-at`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Owner ↔ daemon store | SDK state RPCs, write grant | No |
| Consumer ↔ daemon store | SDK state RPCs, read-only grant on the owner's keys, write grant on its own interest key | No |
| Owner ↔ consumer | `emit-event` notices both ways, subscribed at stage 5; no import, no direct call | No |
| Operator ↔ owner | `update irr ...`, `clear irr ...` through the command dispatcher (every interface) | No |
| Owner ↔ IRR server, PeeringDB | whois, HTTPS | No |
| `firewall-domain` ↔ engine resolver | `ze-plugin-engine:resolve-dns` | No |

### Stored values and notices

Every key below is registered in `pkg/zefs/keys.go`. Times are UTC, encoded as Unix seconds in the binary dataset and RFC 3339 in the JSON values.

| Key or event | Encoding | Field | Type | Description |
|--------------|----------|-------|------|-------------|
| `meta/irr-data/{server}/{as-set}/{digest}` | binary, declared bound 12,000,000 bytes (base64 16,000,000, under `MaxMessageSize` 16,777,216 with room for the envelope); `{digest}` is the value's own SHA-256 in lower-case hex, the digest the pointer carries, so a write never replaces the value the current pointer names (R1-1) | version | uint8 | Format version, 1 |
| same | same | refreshed-at, checked-at, stale-since | 3 x int64 | As `CachedEntry`; `checked-at` is the last time the server answered; 0 means unset |
| same | same | as-set, server | length-prefixed strings | Identity, compared with the key on read together with the digest |
| same | same | ipv4 count, ipv4 prefixes | uint32, then per prefix a length byte and the significant address bytes | As NLRI encodes a prefix |
| same | same | ipv6 count, ipv6 prefixes | uint32, then the same per prefix | |
| same | same | digest | 32 bytes | SHA-256 over every byte before it; a mismatch reads as absent |
| `meta/irr-set/{as-set}` | JSON, 1 MiB default bound | `server`, `generation`, `digest` | string, uint32, hex string | The dataset currently enforced for the set; written after the dataset |
| `meta/irr-asn/{asn}` | JSON | `as-set`, `peeringdb-url`, `discovered-at` | string, string, RFC 3339 | The set PeeringDB named; a binding whose URL differs from `system peeringdb url` reads as absent |
| `meta/irr-generation` | 4-byte counter (`StateIncrement`) | value | uint32 | Raised once per pointer write or removal |
| `meta/irr-interest/{consumer}` | JSON | `as-sets`, `asns` | string list, uint32 list | What the consumer enforces; written after each apply and at `OnStarted`; bounded to 65,536 names |
| event `irr/dataset-stored` | JSON payload | `as-set`, `server`, `generation`, `digest`, `removed` | string, string, uint32, hex string, bool | Sent after each pointer write; `removed` true after `clear irr` |
| event `irr/interest-changed` | JSON payload | `consumer` | string | Sent after a consumer rewrites its interest key |
| `meta/firewall/domain-group/{group}/{name}/{family}` | JSON | `expires-at` (new) | RFC 3339 | Answer time plus `max(ttl, ttl-floor)` capped at one day; written on every answer |

### Changed-Wire Census

**Search evidence:** scope: the whole tree (Go, YANG, `.ci`, `.et`, shell, Python, docs, `internal/test/fixture`). Terms: `KeyIRRPrefixCache`, `KeyIRRCache`, `meta/irr`, `meta/bgp/irr-cache`, `CachedEntry`, `refreshed-at`, `stale-since`, `KeyFirewallDomainGroup`, `resolved-at`, `failing-since`, `StateDataMax`, `ValidateStateInput`, `RegisterPluginKeys`, `refresh-interval`, `peeringdb-url`, `policy irr`, `firewall irr`, `update bgp irr`, `update firewall irr`, `clear firewall irr`. Positive control: `persist` and `Open` in `store.go` found by `KeyIRRPrefixCache`; `cache.go` `put` and `open` by `KeyFirewallDomainGroup`; `stateCall` and `stateInput` by `ValidateStateInput`; `parseIRRConfig` by `refresh-interval`. Findings: config leaves `refresh-interval` or `peeringdb-url` appear in 15 `.ci` files (`test/plugin/filter-irr*.ci`, `filter-wire-irr-*.ci`, `clear-irr-cache*.ci`, `firewall-irr-refresh.ci`, `firewall-irr-cold-cache-recovers.ci`, `test/reload/bgp-filter-irr-*.ci`) and 8 docs pages; 12 `test/plugin/firewall-irr*.ci` set `firewall irr server`. The implementation re-runs the search before its first edit and adds every site found.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| IRR dataset `meta/irr-data/{server}/{as-set}/{digest}` | writer | `internal/plugins/irr` refresh, through the codec in `internal/component/resolve/irr/store` | today `store.persist` writes `meta/irr/{name}` | create; compact value, digest |
| IRR dataset `meta/irr-data/{server}/{as-set}/{digest}` | reader | `bgp-filter-irr` and `firewall-irr` notice and start handlers; `firewall-irr` verify; the owner at start; both doctor checks | today `store.Open` lists and decodes JSON | create; read by name, digest checked |
| IRR dataset `meta/irr-data/{server}/{as-set}/{digest}` | writer | `internal/component/config/storage/cli/data_rpc.go` `update data key`, `delete data key`; `ze data write` | key-agnostic; refused while a granted writer runs (`PluginKeyOwners`) | preserve; names the `irr` plugin |
| IRR dataset `meta/irr-data/{server}/{as-set}/{digest}` | reader | `data_rpc.go` `show data`; `ze data`; `ze data backup` | key-agnostic | preserve |
| IRR dataset `meta/irr-data/{server}/{as-set}/{digest}` | reader | a third-party plugin outside the repository | grants come only from compiled registration | preserve: none can read it |
| AS-SET pointer `meta/irr-set/{as-set}` | writer | `internal/plugins/irr` after a dataset write | new | create; flipped last |
| AS-SET pointer `meta/irr-set/{as-set}` | reader | both consumers, the owner at start, doctor checks | new | create |
| ASN binding `meta/irr-asn/{asn}` | writer | `internal/plugins/irr` after a PeeringDB answer | new | create |
| ASN binding `meta/irr-asn/{asn}` | reader | both consumers, the owner, doctor checks | new | create |
| Generation `meta/irr-generation` | writer | `internal/plugins/irr` via `StateIncrement` | new | create |
| Generation `meta/irr-generation` | reader | both consumers at start | new | create |
| Interest `meta/irr-interest/{consumer}` | writer | `bgp-filter-irr`, `firewall-irr` at `OnStarted` and after each apply | new | create |
| Interest `meta/irr-interest/{consumer}` | reader | `internal/plugins/irr` at start and on notice | new | create |
| Dataset notice (event `irr/dataset-stored`) | writer | `internal/plugins/irr` `EmitEvent` | new | create; event type registered by the owner |
| Dataset notice (event `irr/dataset-stored`) | reader | both consumers, subscribed at stage 5 | new | create |
| Interest notice (event `irr/interest-changed`) | writer | both consumers `EmitEvent` | new | create |
| Interest notice (event `irr/interest-changed`) | reader | `internal/plugins/irr`, subscribed at stage 5 | new | create |
| Legacy `meta/irr/{name}` JSON | writer | `store.go` `persist` in today's `firewall-irr` | the only writer | change: deleted |
| Legacy `meta/irr/{name}` JSON | reader | `internal/plugins/irr` one-time conversion at its first start | new reader of the old value | migrate under the configured server, write pointer and binding, remove |
| Legacy `meta/bgp/irr-cache` blob | writer | none left (pre-2026-10 code) | `migrate` only reads it | delete the registration |
| Legacy `meta/bgp/irr-cache` blob | reader | `store.go` `migrate` | reads and removes | delete; a box holding it cold-fetches once |
| Domain stored answer `meta/firewall/domain-group/{group}/{name}/{family}` | writer | `firewall/plugins/domain/cache.go` `put`, from every answer | JSON `resolvedName` | change: gains `expires-at`, written on every answer |
| Domain stored answer `meta/firewall/domain-group/{group}/{name}/{family}` | reader | `cache.go` `open` (live, verify), `domain/doctor.go` | JSON decode | change: absent `expires-at` reads as expired |
| State value bound | writer | `pkg/plugin/sdk/sdk_state.go` `stateCall` | global `StateDataMax` | change: the key's declared bound |
| State value bound | reader | `dispatch_state.go` `stateInput`, `opStateGet`; `sdk_state.go` result check; `ze-plugin-engine.yang` | global `StateDataMax` | change: declared bound; YANG length the largest one |
| IRR settings leaves | writer | operator configs; the 15 `.ci` bodies and fixtures above; docs examples (`policy irr`, `firewall irr` server, URL, interval) | YANG in `ze-filter-irr.yang`, `ze-firewall-irr.yang` | change: converted in the repository to `system irr { server; source-address; refresh-interval }` and `system peeringdb url`; a firewall config that relied on the old firewall default 0 gains an explicit `system irr refresh-interval 0` where its test asserts no automatic refresh; no migration path (`ai/META-DECISIONS.md`, "Ze is pre-release") |
| IRR settings leaves | reader | `filter_irr/config.go` `parseIRRConfig`; `firewall/plugins/irr/config.go` `parseIRRConfig` | parse today | change: deleted from the consumers; the owner's parser reads `system irr` and `system peeringdb url` (`ConfigReads` system) |
| IRR fetch commands | writer | operators; `.ci` bodies (`filter-irr-update.ci`, `firewall-irr-update.ci` and others); docs (`update bgp irr`, `update firewall irr`, `clear firewall irr`) | `ze-filter-irr-cmd.yang`, `ze-firewall-irr-cmd.yang` | change: `update irr`, `clear irr` on the owner; the repository's callers converted |
| IRR fetch commands | reader | `filter_irr/command.go`, `cmd_irr.go` forwarders, `firewall/plugins/irr/command.go` | dispatch today | change: deleted; the owner's handlers answer |

### Integration Points
- `irrPlugin.publishLocked` (BGP) and `applyTables` (firewall) - the only publish paths for a read copy.
- `readmitter.notify` - after every BGP publish.
- `firewall.RegisterTables`/`ApplyAll` - after the `OnStarted` read, never in the handshake.
- `statestore.RegisterPluginKeys` (owner), `RegisterPluginReadKeys` (consumers) - from each `register.go`.
- `OnResolverCacheClear` - declared by the owner only.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Fetch only in the owner; storage only through state RPCs; publication only through each consumer's existing publish path |
| No unintended coupling (components stay isolated) | Yes | Owner and consumers meet through keys, events and commands; none imports another |
| No duplicated functionality (extends existing, does not recreate) | Yes | Two fetchers, two schedules and three server, URL and interval declarations become one |
| Zero-copy preserved where applicable (refs, not copies) | Yes | The decoded prefix slices are the published list; encoding writes into one buffer sized from the counts |
| Registration over hardcoding, outbound | Yes | The owner registers its plugin, YANG, commands, event types, keys and grants from its own `register.go` |
| Registration over hardcoding, inbound | Yes | Searched `dispatch_state.go`, `state.go`, `sdk_state.go`, `data_rpc.go`, `permissions.go`: the bound and read grant come from registration; the owner learns its consumers from interest keys, never from a list of consumer names |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Peers send UPDATEs only after plugin startup completes | five-stage startup | UPDATEs in the window are rejected, held and re-asked; correct but noisy | restart `.ci` asserting the first UPDATE is accepted | confirmed: `test/plugin/irr-restart-keeps-bgp-list.ci` as first written (04dc7a7e26) had no BGP peer, so it could not test this; d141740344 adds `le test peer` (one session per start, `option=tcp_connections:value=2`, the UPDATE for 10.0.0.0/24 on each), `attach process` and `reject=stderr:contains=irr filter reject`, and its observer (`internal/test/fixture/register_irr_restart_keeps_bgp_list.go` `irrRestartKeepsBGPList`) requires the restarted daemon to accept the listed UPDATE. The test is green on darwin (d141740344 body) with no startup race observed |
| A-2 | A value up to the declared bound (12,000,000 bytes) passes dispatch, ZeFS and DirectBridge | `MaxMessageSize` 16 MiB | the bound is lowered and the largest firewall lists refused | unit test at the bound over both transports | confirmed for the daemon dispatch over both transports: `internal/component/plugin/server/dispatch_state_bound_test.go` `TestPluginStateDeclaredBoundBothTransports` round-trips a `zefs.IRRDatasetValueMax` value over the socket and the DirectBridge path; the package was green in the 6e67f01e6d evidence (`plugin/server`) |
| A-3 | A stage-5 subscription is live before the subscriber's `OnStarted` read | `process-protocol.md` stage 5, "before any event flows" | a notice between read and subscribe is lost; the generation gap check recovers it | unit test: notice emitted between subscribe and read | confirmed (5c42ec5f1e, test red under break: `internal/component/plugin/server/startup_notice_window_test.go`) |
| A-4 | The `.ci` runner can stop and start the daemon over the same store | `ci-format.md` "a restart reuses its tree" | runner support is added here | read of the runner | confirmed, with a barrier between the starts: reusing a daemon block reuses its directory and its store (`internal/test/runner/runner_config.go` `zeConfigFileName`), but the runner does not wait for a foreground `ze` (`runner_exec.go` `runOrchestrated`, arm `stepArmDaemon`), so two `cmd=foreground` steps of one block ran at once. Each restart `.ci` now runs `le test fixture daemon/await-exit` between the two starts (as `test/plugin/session-editor-commit-confirmed-restart.ci` does), with each `OK:` expectation under the start that prints it. `irr-cold-start-stores.ci` and `irr-restart-keeps-bgp-list.ci` are green over one store (d141740344 body). Mistake Log row A-4 |
| A-5 | The IRR stub can count queries | `internal/test/fixture/plugin_fixture_07_irr.go` | a counting stub is added here | read of the fixture | broken: `internal/test/mock/irr/irr.go` `Run` counted nothing; phase 1 added `--query-log FILE` (`queryLog.record`), read by `internal/test/fixture/irr_owner_fixture.go` `irrCountQueries` |
| A-6 | AS-SET names and `host[:port]` servers are valid key segments | `ValidateStateInput` alphabet `[A-Za-z0-9_.:/-]` | an escape is needed | unit test | confirmed by read: `pkg/plugin/rpc/state.go` `ValidateStateInput` admits `:`, `.`, `-`, `_` and `fs.ValidPath` refuses only `/`-structure faults; `statestore` `grantAdmits` matches a template segment as `[^/]+`; `internal/plugins/irr/register_test.go` `TestIRROwnerGrantsItsKeysOnly` covers `127.0.0.1:4343/RIPE::AS-TEST`, green in the 6e67f01e6d evidence (`internal/plugins/irr`) |
| A-7 | No consumer names its own IRR server per peer or per rule | one `server` leaf per consumer, none under a peer or term (`ze-filter-irr.yang`, `ze-firewall-irr.yang`) | moving the server to the owner loses a capability | YANG read | validated |
| A-8 | The owner autoloads whenever a consumer is configured | a consumer declaring the owner in `Dependencies` (registry) | consumers wait forever on a notice | functional test with no owner container configured | confirmed: `startup_autoload.go` resolves `Dependencies` transitively through `registry.ResolveDependencies`, and both consumers name `store.OwnerPlugin` in `Dependencies` (`filter_irr/register.go`, `firewall/plugins/irr/register.go`); `test/reload/irr-bgp-commit-new-set.ci` starts `ze --plugin ze.bgp-filter-irr --plugin ze.bgp-adj-rib-in` with no `ze.irr` flag, requires the owner's `irr dataset stored` line, and is green (d141740344 body) |
| A-9 | Reading the owner's keys needs no owner process: a consumer forked or started before the owner still reads them | reads go to the daemon store, not the owner | none | unit test with the owner stopped | confirmed (5c42ec5f1e, test red under break: `internal/component/plugin/server/dispatch_state_owner_stopped_test.go`) |

### Mistake Log: Wrong Assumptions
| ID | What was assumed | What is true | Consequence |
|----|------------------|--------------|-------------|
| A-5 | The IRR stub can count queries | `le mock irr` answered queries and recorded none | Phase 1 added `le mock irr --query-log FILE`; documented in `docs/functional-tests.md` "le mock irr" |
| A-4 | Two `cmd=foreground` runs of one daemon block restart over one store | The runner starts a foreground `ze` and does not wait for it (`runOrchestrated`, `stepArmDaemon`): both starts ran at once, the second was refused the store ("store is owned by another process"), and teardown killed the first about 1.3 s later; each `expect=` binds to the `cmd=` above it, so an `OK:` line under the second start could never match the first start's output; a per-step `timeout=` replaced the 60 s budget | The phase-1 restart `.ci` files were red for a wrong reason. Fixed in phase 1: `le test fixture daemon/await-exit` between the starts, the `OK:` lines moved under the start that prints them, the step timeouts removed |
| Phase 1 test | `irr-restart-keeps-bgp-list.ci` proves AC-1's "an UPDATE announcing a listed prefix is accepted with no hold" | As written in phase 1 it had no BGP peer and no `attach`, so no UPDATE could reach its observer | AC-1's UPDATE half was unproven until d141740344 added the peer, the attach and the reject check (A-1) |
| Phase 7 fixture | A daemon the restart fixture starts keeps its store on disk | A `.ci` without `option=storage` gets `ze.test.storage=memory` from the runner (`record_dut_store.go` `dutStore.env`), and the fixture passed only `ze.config.dir`, so each start had a fresh in-memory store | The second start of `firewall-domain-group-restart-keeps-answers` could never read the first start's answers. Fixed in phase 1's fix pass: `netfilter_fixture_domain_group_restart.go` passes `daemonOverrides(workDir)` |
| Phase 3 | The owner can convert and remove the legacy `meta/irr/{name}` keys in a commit of its own | `firewall-irr` loaded its cached lists from those keys until phase 5 replaced its reader | A tree with phase 3 alone would lose the firewall's stored lists at the next start, so phases 3, 4 and 5 landed together in 6e67f01e6d |
| Phase 4 | Test edits pass through the weakening hook | A phase-4 agent rewrote `filter_irr` test files through a python script, which the Write and Edit hooks do not see | Every change it made was afterwards judged by `ze-test-judge` agents and recorded (`Weakening-approved:` lines of 6e67f01e6d); the two files that no longer compiled, `command_test.go` and `cmd_irr_args_test.go`, were removed in that commit; later agents were briefed to edit tests with Edit and Write only |
| Tooling | `./le test weakened approve` can record a verdict for any unit the weakening gate names | The approval tool returned one carrier per same-named declaration, so a method on two receivers in one file was "ambiguous", while the gate groups them as one unit (`irr_test.go` fake metric methods) | The 3+4+5 commit was refused until 8472da0c8d made the tool group same-named declarations per file |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A new list reaches the evaluator only after the owner's write and the notice, which can take the full 5 s state timeout under disk load | `checked-at` to consumer publish delay in logs | no wait depends on it any more (reject, hold, re-admit); the delay is bounded and measured in AC-18 |
| R-2 | Schedule anchored on `refreshed-at` refetches a one-family set every tick | outcome counter climbing every minute | anchor on `checked-at`; AC-3 |
| R-3 | A lost notice leaves a consumer on an older copy, so its restart changes its data | consumer and pointer generations differ (doctor) | the generation gap check re-reads every pointer; a doctor check compares enforced and stored generations; AC-8 |
| R-4 | A consumer reload removes the stored dataset another consumer uses | firewall set lost after a BGP edit | consumers never write datasets; only `clear irr` removes one; AC-16 |
| R-5 | The owner is not running (removed, crash-looped past its restart bound) | `show health` doctor-plugin-stopped | consumers keep enforcing the stored copies; no refresh happens; doctor reports it; the owner declares `failure-policy: restart` |
| R-6 | Corrupt stored dataset | digest mismatch at read | read as absent: consumer keeps what it enforces, owner fetches at once, WARN and doctor code; AC-22 |
| R-7 | A server change orphans every dataset of the old server | store growth | the owner flips each pointer only after the new server answers, then removes the old dataset (single writer, safe) |
| R-8 | A failed refresh retried every minute for days | error outcome counter | the readmitter's cadence today; stops at the first answer |
| R-9 | Domain writes up to 1,440 a day per name and family at the 60 s floor | store write rate | Thomas's choice; each write replaces one same-size value in place (`zefs-format.md`), so no full rewrite |
| R-10 | Config conversion misses a `.ci` or doc | YANG refusal in a functional run | the census search terms re-run before the first edit; every `.ci` converted in the same commit as the YANG move |
| R-11 | A consumer's interest diverges from what it enforces | owner fetches a set nobody uses, or misses one | interest written in the same apply that publishes; doctor compares interest with config references |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | BGP IRR import filters and firewall IRR sets enforce wrong or no prefixes after a restart or a commit; domain sets resolve at the wrong time |
| How is it reverted? | Revert of the implementation commits; the new keys are then ignored, BGP cold-fetches, the firewall reports uncached references; the config conversion is reverted with it |
| Who else touches this path? | `spec-irr-apply-policy.md` (post-release, depends on this); sessions editing `filter_irr`, `firewall/plugins/irr`, `firewall/plugins/domain`, `resolve/irr/store`, `pkg/plugin/rpc/state.go`, `statestore/permissions.go` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Daemon restart with a stored dataset, BGP consumer | → | owner start read, consumer `OnStarted` read, `publishLocked` | `test/plugin/irr-restart-keeps-bgp-list.ci` |
| Daemon restart with a stored dataset, firewall consumer | → | consumer `OnStarted` read, `applyTables` | `test/plugin/irr-restart-keeps-firewall-sets.ci` |
| First start, nothing stored | → | interest key, owner cold fetch, notice | `test/plugin/irr-cold-start-stores.ci` |
| BGP and firewall naming one AS-SET | → | one owner fetch, notice to both | `test/plugin/irr-shared-dataset.ci` |
| BGP commit adding an unstored AS-SET | → | interest notice, owner fetch, readmit | `test/reload/irr-bgp-commit-new-set.ci` |
| `update irr as-set` then firewall commit | → | owner command, pointer, verify read | `test/plugin/irr-update-then-firewall-commit.ci` |
| Domain restart with unexpired answers | → | `schedule.reset` with stored expiry | `test/plugin/firewall-domain-group-restart-keeps-answers.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | BGP peer filtered by `bgp-filter-irr:<asn>`, dataset stored, daemon restarted with the IRR stub counting | `show bgp irr` reports the same AS-SET, counts and `refreshed-at` as before; the stub receives zero queries during start; an UPDATE announcing a listed prefix is accepted with no hold | A refresh whose due time passed while the daemon was down runs once after start: it was due by the operator's schedule |
| AC-2 | Firewall rule naming a stored AS-SET, daemon restarted | The nft set holds the same prefixes before any query; zero queries; no uncached WARN | The same overdue refresh as AC-1 |
| AC-3 | Dataset with `checked-at` T, interval 3600, stopped at T+10 min, started at T+40 min | The owner fetches at T+60 min, not T+100 min; a one-family AS-SET is fetched once per interval | A failed fetch at T+60 is retried every minute until it answers (AC-7) |
| AC-4 | Commit that changes no IRR reference or setting | No query; every dataset's next refresh unchanged | None |
| AC-5 | BGP commit adding a peer whose AS-SET is stored; one whose AS-SET is not | Stored: the consumer publishes it at once, no query. Not stored: the commit succeeds, the owner fetches it right after, stores it, the consumer publishes on the notice | Before that publish the peer's announcements are rejected, held, then re-asked by ROUTE-REFRESH (existing readmit) |
| AC-6 | First start, nothing stored | The owner fetches each referenced dataset once, at once after `OnStarted`, stores it, and fetches it again only when due; both consumers enforce it on the notice | A failed cold fetch is retried every minute; the firewall table stays held back meanwhile |
| AC-7 | Due refresh fails, or answers empty | The stored copy and pointer stay; `stale-since` set on an empty answer; on failure `checked-at` is unchanged and the owner retries every minute; on an empty answer `checked-at` advances and the next fetch waits a full interval | Only those retries |
| AC-8 | BGP and firewall name the same AS-SET | One fetch per interval, one dataset key; after a refresh both consumers enforce the new copy within one second; a consumer that missed a notice re-reads on the next notice's generation gap; after restart both enforce the same generation | None |
| AC-9 | Owner `server` changed by a commit | The owner fetches every referenced set from the new server at once; consumers keep enforcing the old server's copy until each new copy is stored and its pointer flips; the old dataset is then removed | None |
| AC-10 | `system irr refresh-interval` absent, set to 900, set to 0 | Absent: every dataset refreshes every 3600 s from its `checked-at`. 900: every 900 s. 0: no automatic refresh, `show irr` reports the schedule as manual. Neither consumer has an interval leaf | With 0, a dataset with no stored copy is still fetched once at once (cold start), and `update irr` still fetches on demand: both are fetches the operator's config or command asked for |
| AC-11 | Bare ASN reference, PeeringDB names AS-Y, restart | The stored binding names AS-Y; start reads AS-Y with no PeeringDB query; PeeringDB is asked again when the dataset is due | A PeeringDB failure at the due time keeps AS-Y |
| AC-12 | AS-SET of 200,000 IPv4 and 100,000 IPv6 prefixes | Stored in one key, read after restart identical; a list over the caps is refused whole with an error naming the counts, the previous copy kept | None |
| AC-13 | Domain name answered TTL 3600 at T, restart at T+10 min; another name past its stored expiry | The first is not resolved at start and is resolved at T+60 min; the second is resolved at start; both groups program stored addresses before any query; every answer rewrites `expires-at` | A name stored before this spec (no `expires-at`) is resolved at start once |
| AC-14 | Readmitter tick with held prefixes | No IRR or PeeringDB query from `bgp-filter-irr`, ever; held prefixes the published list admits are re-asked | None |
| AC-15 | `update irr asn <n>`, `update irr as-set <s>`, `update irr all` | The owner fetches now, stores, resets `checked-at`, notifies; the answer carries the counts; `update bgp irr ...` and `update firewall irr ...` no longer exist | None |
| AC-16 | `clear irr as-set <s>` (and `clear irr asn <n>`) on a set BGP and the firewall still reference | The dataset and pointer are removed; the reply names each consumer whose interest still references it (`bgp-filter-irr`, `firewall-irr`); those consumers drop the list on the notice (BGP rejects and holds, the firewall holds the table back); the owner does not refetch it until `update irr` or a commit that newly names it | A clear of a set no consumer references removes it and names no consumer |
| AC-17 | Box holding `meta/irr/{name}` keys | The owner's first start converts each under its configured server, writes pointer and binding, removes the old key; the firewall enforces the same prefixes with no query | `meta/bgp/irr-cache` is dropped and cold-fetched once |
| AC-18 | Owner state write taking 5 s | The consumer publishes within one second of the write completing; no evaluator call waits | None |
| AC-19 | Doctor with an unreadable dataset, an overdue refresh, a consumer generation behind the pointer, the owner stopped | `ze doctor` reports each with its code and fix | None |
| AC-20 | Pages, YANG descriptions, comments in the Documentation Update Checklist | Each states the new behaviour; the repository's configs, `.ci` bodies, fixtures and examples use `system irr`, `system peeringdb url`, `update irr` and `clear irr`; `firewall-irr.md` "Auto-refresh is off by default" and `command-verbs.md` lines 536 and 574-575 are rewritten | None |
| AC-21 | firewall-irr startup | No table registered and no uncached WARN in the handshake; tables applied once after the `OnStarted` read | None |
| AC-22 | Dataset failing its digest or decode | Read as absent: the consumer keeps what it enforces, the owner fetches at once, one WARN naming the key, the doctor code | None |
| AC-23 | Firewall commit naming an AS-SET with no pointer | Refused, naming `update irr as-set <s>` (or `update irr asn <n>`); after that update the same commit succeeds and the set is programmed | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Reboots a router with IRR-filtered peers | start → stage 5 subscribe → consumer read → publish → UPDATE judged | `test/plugin/irr-restart-keeps-bgp-list.ci` |
| 2 | Reboots a router with firewall IRR rules | start → read → `applyTables` → nft set | `test/plugin/irr-restart-keeps-firewall-sets.ci` |
| 3 | Adds a BGP peer on a new AS-SET | commit → interest notice → owner fetch → notice → publish → readmit | `test/reload/irr-bgp-commit-new-set.ci` |
| 4 | Fetches an AS-SET then commits a firewall rule on it | `update irr as-set` → owner → verify reads pointer → commit | `test/plugin/irr-update-then-firewall-commit.ci` |
| 5 | Reboots with domain groups | start → program → schedule at stored expiry | `test/plugin/firewall-domain-group-restart-keeps-answers.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDatasetRoundTrip` | `internal/component/resolve/irr/store/dataset_test.go` | Encode and decode reproduce prefixes and every timestamp | |
| `TestDatasetLargeFitsOneKey` | same | 500,000 IPv4 and 500,000 IPv6 under the declared bound | |
| `TestDatasetDigestRefused` | same | A flipped byte reads as absent | |
| `TestServerIdentityNormalised` | same | `whois.radb.net`, upper case and `:43` name one dataset | |
| `TestDueFromCheckedAt` | `internal/plugins/irr/schedule_test.go` | Due is `checked-at` + interval for a one-family set | |
| `TestColdReferenceDueNow` | same | A reference with no pointer is due at start | |
| `TestFailedFetchKeepsPointer` | `internal/plugins/irr/irr_test.go` | Failure leaves dataset, pointer and `checked-at`; retry in one minute | |
| `TestPointerFlipsAfterWrite` | same | The pointer changes only after the dataset write is acknowledged | |
| `TestServerChangeKeepsOldUntilNew` | same | Old-server dataset stays enforced until the new one is stored | |
| `TestInterestDrivesFetch` | same | Interest keys and notices decide the fetched set | |
| `TestLegacyEntryConverted` | same | `meta/irr/{name}` converted and removed | |
| `TestConsumerReadsNoFetch` | `internal/component/bgp/plugins/filter_irr/filter_irr_test.go` | `OnStarted` publishes stored lists; the plugin holds no IRR client | |
| `TestNoticeGapRereads` | same | A notice two generations ahead re-reads every pointer | |
| `TestReadmitterNoLookup` | `internal/component/bgp/plugins/filter_irr/readmit_test.go` | The readmitter has no fetch path | |
| `TestHandshakeConfigureAppliesNothing` | `internal/component/firewall/plugins/irr/irr_test.go` | No WARN and no `RegisterTables` before `OnStarted` | |
| `TestVerifyRefusesNoPointer` | same | Commit refusal names `update irr as-set` | |
| `TestReadGrantRefusesWrite` | `internal/core/statestore/permissions_test.go` | A read-only grant reads and cannot write or remove | |
| `TestStateBoundPerKey` | `pkg/plugin/rpc/state_test.go` | A declared bound accepts up to it; others keep 1 MiB | |
| `TestResetUsesStoredExpiry` | `internal/component/firewall/plugins/domain/schedule_test.go` | Unexpired unit armed at its expiry; expired or missing due now | |
| `TestEveryAnswerRenewsExpiry` | `internal/component/firewall/plugins/domain/domain_test.go` | A steady answer rewrites `expires-at` | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `system irr refresh-interval` | 0 or 60..86400, default 3600 | 0, 60, 86400 | 59 (1..59 invalid) | 86401 |
| Dataset value size | 1..declared bound | bound | N/A | bound + 1 refused |
| Domain `ttl-floor` | existing range | unchanged | unchanged | unchanged |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `irr-restart-keeps-bgp-list` | `test/plugin/irr-restart-keeps-bgp-list.ci` | Restart: same counts, zero queries, listed prefix accepted | |
| `irr-restart-keeps-firewall-sets` | `test/plugin/irr-restart-keeps-firewall-sets.ci` | Restart: nft set unchanged, zero queries | |
| `irr-cold-start-stores` | `test/plugin/irr-cold-start-stores.ci` | Empty store: one query per set, stored, restart makes none | |
| `irr-shared-dataset` | `test/plugin/irr-shared-dataset.ci` | BGP and firewall on one set: one query per interval, both enforce | |
| `irr-bgp-commit-new-set` | `test/reload/irr-bgp-commit-new-set.ci` | New AS-SET on a BGP peer: fetched after commit, held prefix re-asked | |
| `irr-update-then-firewall-commit` | `test/plugin/irr-update-then-firewall-commit.ci` | Refused commit, `update irr as-set`, commit accepted | |
| `firewall-domain-group-restart-keeps-answers` | `test/plugin/firewall-domain-group-restart-keeps-answers.ci` | Unexpired answer not resolved at start | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | No wire-visible BGP change: verdicts are unchanged, only where the list comes from changes; goal validation is the restart `.ci` tests | |

## Files to Modify
- `internal/component/resolve/irr/store/store.go` - keeps resolution and last-known-good for the owner; persistence replaced by the new keys; `migrate` and `Open`-by-listing deleted
- `internal/component/resolve/irr/store/clear.go` - used by the owner only
- `internal/component/bgp/plugins/filter_irr/filter_irr.go` - no IRR client, no `PrefixStore`; reads pointers and datasets at `OnStarted` and on notice; writes interest; `initialResolve`, `refreshLoop`, `refreshAll*`, `refreshASN*` deleted
- `internal/component/bgp/plugins/filter_irr/readmit.go` - `resolveHeld` and `retryResolution` deleted
- `internal/component/bgp/plugins/filter_irr/command.go`, `cmd_irr.go`, `yang/ze-filter-irr-cmd.yang` - `update bgp irr ...` deleted; `show bgp irr` reports generation, `checked-at`, next refresh read from the owner's keys
  → Decision (implementation): `show bgp irr` reports per entry `asn`, `as-set`, `status` (`ok`, `error` with `error`, `pending`), the counts, and for a stored copy `server`, `generation`, `checked-at` and `refreshed-at`, then `peers`; it reports no next refresh, because no stored key carries the owner's schedule and `show irr` answers `next-refresh` (`filter_irr/command.go` `showIRR`; `internal/plugins/irr/command.go` `datasetRow`). The top-level `server`, `last-refresh` and `next-refresh` fields are gone
- `internal/component/bgp/plugins/filter_irr/config.go`, `yang/ze-filter-irr.yang` - `policy irr` augment removed; the per-peer `irr { as-set; enable }` stays
- `internal/component/bgp/plugins/filter_irr/register.go` - read grant, interest write grant, `Dependencies` on the owner, stage-5 subscription
- `internal/component/firewall/plugins/irr/irr.go`, `config.go`, `command.go`, `register.go`, `sets.go`, `doctor.go`, `yang/ze-firewall-irr.yang`, `yang/ze-firewall-irr-cmd.yang` - the same consumer change; `firewall irr { server; peeringdb-url; refresh-interval }` removed, the `interface` list and term leaves stay; `update`/`clear firewall irr` deleted; handshake fix; stale comments
- `internal/component/firewall/plugins/domain/schedule.go`, `cache.go`, `domain.go`, `doctor.go` - stored expiry renewed on every answer
- `internal/core/statestore/permissions.go` - read-only grant
- `pkg/zefs/keys.go` - new keys with bounds; old IRR keys removed
- `pkg/plugin/rpc/state.go`, `pkg/plugin/sdk/sdk_state.go`, `internal/component/plugin/server/dispatch_state.go`, `internal/core/ipc/yang/ze-plugin-engine.yang` - per-key bound; read grant checked on read verbs
- every `.ci` and fixture in the census (15 files naming the moving leaves, 12 setting `firewall irr server`, the `update ... irr` callers) - converted
- `docs/architecture/bgp/filter-irr.md`, `docs/architecture/core-design.md` section 22, `docs/architecture/resolve.md`, `docs/architecture/firewall/firewall-irr.md`, `docs/architecture/firewall/firewall-domain-group.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/zefs-format.md`, `docs/guide/irr-filtering.md`, `docs/guide/configuration.md`, `docs/guide/firewall.md`, `docs/guide/command-reference.md`, `docs/guide/plugins.md`, `docs/features/plugins.md`

## Files to Create
- `internal/plugins/irr/register.go`, `irr.go`, `schedule.go`, `command.go`, `doctor.go`, `config.go` - the owner plugin
- `internal/plugins/irr/yang/` - `ze-irr-conf.yang` augmenting `/system:system` with `irr { server; source-address; refresh-interval }` (`server` default `whois.radb.net`, `refresh-interval` `0 | 60..86400` default 3600), and `ze-irr-cmd.yang` with `update irr all|asn|as-set`, `clear irr asn|as-set`, `show irr`
- `internal/component/resolve/irr/store/dataset.go` - the compact codec shared by owner and consumers
- `docs/architecture/irr-owner.md` - the owner's design page (its `// Design:` target)
- the seven `.ci` files of the Functional Tests table

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/irr/yang/`: owner container, `update irr`, `clear irr`, `show irr`; leaves removed from `ze-filter-irr.yang`, `ze-firewall-irr.yang` |
| YANG validation constraints | Yes | `system irr refresh-interval` `range "0 | 60..86400"`, default 3600; `server` host or host:port, default `whois.radb.net`; `source-address` `zt:ip-address` |
| YANG custom validators | No | Native constraints suffice |
| CLI commands/flags | Yes | `update irr all|asn|as-set`, `clear irr asn|as-set`, `show irr` |
| CLI grammar (keyword before value) | Yes | `update irr as-set <name>`, `update irr asn <asn>` follow `ai/rules/cli.md` |
| Editor autocomplete | Yes | AS-SET names from the interest keys through a `CompleteFn` on the owner |
| Functional test for new RPC/API | Yes | The seven `.ci` files |
| Pipe completeness | Yes | `show irr` answers structured data through the shared renderer |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | Yes | `internal/plugins/irr/doctor.go`: owner stopped, overdue refresh, unreadable dataset; `firewall/plugins/irr/doctor.go`; `domain/doctor.go` |
| Doctor code per problem class | Yes | Unreadable dataset, overdue refresh, consumer generation behind, owner not running, unreadable DNS answer; runtime phase through `OnDoctorCheck` |
| Prometheus counters/metrics | Yes | `ze_irr_refresh_outcomes_total` and `ze_irr_data_age_seconds` move to the owner; the `ze_firewall_irr_*` refresh metrics go with the consumer fetch |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the IRR filter and firewall IRR feature files under `features/`, and one for the `irr` plugin (`./le feature report`) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`, `docs/architecture/api/process-protocol.md` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, `docs/features/plugins.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/irr-filtering.md`, `docs/guide/firewall.md` |
| 7 | Wire format changed? | No | No BGP wire change |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` "Persistent runtime state" (read grant, per-key bound) and the two event types |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 8210 and RFC 1035 cited only |
| 10 | Test infrastructure changed? | Yes if A-4 or A-5 add runner or stub support | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` section 22, `docs/architecture/bgp/filter-irr.md`, `docs/architecture/resolve.md`, `docs/architecture/firewall/firewall-irr.md`, `docs/architecture/firewall/firewall-domain-group.md`, `docs/architecture/zefs-format.md`, `docs/architecture/irr-owner.md` (new) |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Derived | `./le spec citation anchors spec plan/release/fixes/spec-fetched-data-survives-restart.md`, re-run for the revised file set. Advisory pages judged: `docs/architecture/cli/command-verbs.md` AFFECTED (lines 536 and 574-575 cite `update bgp irr as-set` and `update bgp irr asn`, which become `update irr as-set` and `update irr asn`); `docs/architecture/diagnostics/debug-filtering.md` unaffected (anchors `KeyDebugProfile` only); `docs/architecture/storage-backends.md` unaffected (mentions no IRR key and no value bound; the new keys are ordinary runtime `meta/` keys under its existing rules); `docs/features/srv6.md` and `docs/guide/vpp.md` unaffected (anchor `StateListMax`, which this spec leaves at 4096) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | every `policy irr`, `firewall irr server|peeringdb-url|refresh-interval` and `update bgp|firewall irr` example in the pages above |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the `irr` plugin registered with its YANG (`system irr`, commands), keys, grants and event types; consumers declare the owner in `Dependencies` and subscribe at stage 5
   - Tests: the seven Wiring Test `.ci` files
   - Files: `internal/plugins/irr/register.go`, `internal/plugins/irr/yang/`, `pkg/zefs/keys.go`, both consumers' `register.go`
   - Verify: the daemon loads `irr` with a consumer configured; the `.ci` files fail on their zero-query or stored-copy assertions
2. **Phase: Stored dataset** -- compact codec, per-key value bound, read-only grant
   - Tests: `TestDatasetRoundTrip`, `TestDatasetLargeFitsOneKey`, `TestDatasetDigestRefused`, `TestServerIdentityNormalised`, `TestStateBoundPerKey`, `TestReadGrantRefusesWrite`
   - Files: `resolve/irr/store/dataset.go`, `pkg/plugin/rpc/state.go`, `pkg/plugin/sdk/sdk_state.go`, `dispatch_state.go`, `ze-plugin-engine.yang`, `statestore/permissions.go`
3. **Phase: Owner** -- `system irr` parsing, interest, schedule from `checked-at` (0 = manual), fetch, pointer flip, generation, notice, `update irr` and `clear irr`, legacy conversion, doctor
   - Tests: `TestDueFromCheckedAt`, `TestColdReferenceDueNow`, `TestFailedFetchKeepsPointer`, `TestPointerFlipsAfterWrite`, `TestServerChangeKeepsOldUntilNew`, `TestInterestDrivesFetch`, `TestLegacyEntryConverted`
   - Files: `internal/plugins/irr/*.go`, `resolve/irr/store/store.go`
4. **Phase: BGP consumer** -- fetch, schedule and `update bgp irr` deleted; read and publish at `OnStarted` and on notice; interest; generation gap
   - Tests: `TestConsumerReadsNoFetch`, `TestNoticeGapRereads`, `TestReadmitterNoLookup`
   - Files: `filter_irr/filter_irr.go`, `readmit.go`, `command.go`, `cmd_irr.go`, `config.go`, both YANG files
5. **Phase: Firewall consumer** -- the same; handshake fix; verify reads the pointer and names `update irr`
   - Tests: `TestHandshakeConfigureAppliesNothing`, `TestVerifyRefusesNoPointer`
   - Files: `firewall/plugins/irr/*.go`, both YANG files
6. **Phase: Config conversion** -- every repository config, `.ci`, fixture and example converted to `system irr`, `system peeringdb url`, `update irr`, `clear irr`, in the same commit as phases 4 and 5 remove the leaves
   - Tests: `./le test functional plugin`, `./le test functional reload`
   - Files: the census `.ci` list, fixtures, docs examples
7. **Phase: Domain** -- `expires-at` written on every answer; `schedule.reset` arms at it
   - Tests: `TestResetUsesStoredExpiry`, `TestEveryAnswerRenewsExpiry`, `firewall-domain-group-restart-keeps-answers.ci`
   - Files: `firewall/plugins/domain/schedule.go`, `cache.go`, `domain.go`, `doctor.go`
8. **Docs and comments** -- each page in the Documentation Update Checklist edited with the phase whose code makes it wrong, never batched at the end

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | Only `internal/plugins/irr` holds an IRR or PeeringDB client; no consumer path fetches |
| Correctness | The pointer flips only after the dataset write is acknowledged |
| Correctness | Every consumer subscribes at stage 5, before its first read |
| Naming | kebab-case JSON keys (`checked-at`, `expires-at`); command grammar `update irr as-set <name>` |
| Rule: no-layering | Consumer fetch paths, both `refreshLoop`s, `update bgp|firewall irr`, the moved leaves, `migrate` and the old keys deleted, not kept beside the new path |
| Rule: stale-comments | `configureStore`, `oversizedEntryError`, the `prefixStore` field and every fetch comment rewritten |
| Operator errors and doctor codes | Each refusal names the `update irr` command; each WARN carries a doctor code |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| One IRR fetcher | grep: no `irr.NewIRR` or `peeringdb.NewPeeringDB` outside `internal/plugins/irr` and the offline `ze resolve` CLI |
| Lists survive restart with no query | `test/plugin/irr-restart-keeps-bgp-list.ci`, `test/plugin/irr-restart-keeps-firewall-sets.ci` |
| One shared dataset | `test/plugin/irr-shared-dataset.ci` |
| Domain answers honour the stored expiry | `test/plugin/firewall-domain-group-restart-keeps-answers.ci` |
| Config converted | grep of `test/` and `docs/` for the removed leaves returns nothing |

Deliverables evidence (third closure): `grep -rln 'irr.NewIRR\|peeringdb.NewPeeringDB'` outside tests hits `internal/plugins/irr/resolver.go`, the offline `ze resolve` CLI (`resolve/cli/cmd_irr.go`, `cmd_peeringdb.go`), the PeeringDB client package itself, `cmd/ze/hub/main_system.go` (the daemon form of `ze resolve irr`) and `bgp/plugins/cmd/peer/prefix_update.go` (PeeringDB max-prefix, out of scope): no consumer fetches. The restart, shared-dataset and domain rows are the functional tests named in Goal Validation.

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | AS-SET and server names validated before forming a key; decoded counts bounded by the value size |
| Authorization | Consumers hold read-only grants on the owner's keys and write only their own interest key |
| Integrity | Digest over each dataset; a mismatch reads as absent |
| Resource exhaustion | Interest names bounded per consumer; the owner refuses an interest key over its bound |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- `RefreshedAt` cannot anchor a schedule: it dates the oldest data an entry holds (`store.go` `CachedEntry`).
- Daemon stop sends no `bye`; nothing can be written at stop.
- With one writer, a pointer written last gives an atomic switch between datasets without any multi-key transaction, provided each dataset has a key of its own: the key ends in the value's digest (b3c34bcb90), and a reader that finds the dataset of a pointer it just read removed reads the pointer once more and follows it (`store.ReadDataset`, third closure).
- The generation is raised before the pointer write, so the pointer carries the generation of its own write (`internal/plugins/irr/datasets.go` `writeDataset`: dataset put, `StateIncrement`, pointer put, old-server dataset removed, notice). The Data Flow listed flip-then-increment; see the decision under step 4.
- The `clear irr` hold lives in the owner's worker memory (`owner.go` `heldSets`, `heldASNs`), not in a stored key: a restart while a consumer still references a cleared set fetches it again. A consumer interest that newly names the set also releases the hold (`datasets.go` `readInterest`).
- A binding change sends a notice for the new set only, so the BGP consumer re-reads every ASN whose set PeeringDB chooses on every notice (`filter_irr/stored.go` `readNamed`: a target with no configured set is always read), and the firewall reader does the same for its ASN references.
- `show bgp irr` no longer reports the next refresh: the schedule is the owner's worker state, derived from `checked-at` and the interval, and is answered by `show irr` `next-refresh` (`automatic` times, `manual` for interval 0, `none` for a cleared or unreferenced set).
- An owner answer before the worker's first read of the store is not an answer: the zero view has no server, interval 0 and no dataset. `show irr` and the `irr-datasets` doctor check refuse until `publishView` first runs (`doctor.go` `ownerView.published`, `errNotLoaded`).
- A doctor check in the process that answers `ze doctor` cannot see a write failure of a plugin that may run forked. The domain plugin's unsaved answer is therefore a runtime check (`firewall-domain-group-unsaved-answer`, answered by `OnDoctorCheck`) plus a report-bus raise until a write succeeds (57bde3372f), not a post-config check over the store.
- A cold start rejects and holds a route judged before the list is stored, as designed; a functional test whose peer announces at once must wait for the stored list first (`filter-irr.ci`, d141740344), or it observes the hold rather than the filter.
- A test daemon started from a fixture inherits the runner's in-memory store unless the fixture passes the runner's daemon overrides (`daemonOverrides`): a restart test needs the on-disk store of its work directory.

### Defects found and fixed in this work

| Defect | Where it was | Fix and proof |
|--------|--------------|---------------|
| An empty answer over a stored list was counted `stored` in `ze_irr_refresh_outcomes_total`, reading as fresh data | `internal/plugins/irr/owner.go` `refreshSet` | Counted `empty`; the kept list is written again marked stale. `metrics_test.go` `TestRefreshOutcomeCountsEmptyDistinctly`, `TestRefreshOutcomeCountsStored` (6e67f01e6d) |
| A panic on the owner's worker or the firewall reader's worker ended the goroutine, which the SDK's dispatch recovery does not cover, and with it the plugin process | `internal/plugins/irr/owner.go`; `internal/component/firewall/plugins/irr/stored.go` | Recovered per operation: `serve` (one command closure, answered with an opererr), `refreshSet` through `fetchPanicked` (retry in one minute, refresh-overdue raised, outcome `panic`), `discoverASN` through `discoverPanicked`; the firewall's `applyRecovered` turns a panic under `applyTables` into a failed apply. `TestFetchPanicKeepsTheWorker`, `TestReaderPassSurvivesABackendPanic` (6e67f01e6d) |
| `show irr` and the irr doctor check answered a zero config (server "", refresh-interval 0) and no datasets before the worker's first view | `internal/plugins/irr/command.go` `showIRR`, `doctor.go` `answerDoctorCheck` | Both refuse with `errNotLoaded` until the first `publishView`. `show_before_load_test.go` `TestShowRefusesBeforeFirstView` (d141740344) |
| `show doctor` listed a fault a plugin runtime check reports twice, from the runtime check and from recent events | `internal/component/doctor`, `internal/core/diagnostic/doctor_registry.go` | 1063ce9adb, outside this spec |
| A changed domain answer the store refused went on as a change: the comment said the addresses would be programmed from memory, which `put` never updated, and the change log recorded a change never enforced | `internal/component/firewall/plugins/domain/domain.go`, the changed-answer path | Returns unprogrammed on a refused write, logs `warnUnsavedAnswer` with its code and fix, and the doctor and the report bus report it until a write succeeds (0f1dc02e4f, 57bde3372f) |
| `./le test weakened approve` refused a unit the weakening gate names, when one file declares a method of that name on two receivers | `internal/le/rfc/approvalunits.go` | 8472da0c8d, `approve_receivers_test.go` |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One owner fetches, schedules and stores IRR data; `bgp-filter-irr` and `firewall-irr` only read it (Thomas, design gate: chose B) | Approach A: each consumer keeps a `PrefixStore`, fetches on its own schedule and shares the stored key through notices | A kept two fetchers and two schedules writing one key, with drift between them; Thomas chose one owner |
| The owner is a plugin, `internal/plugins/irr/`, and owns the timing (Thomas: "We should move the timing to the plugin which will implement the fetching.") | A daemon service in the resolve component; a sub-plugin under `internal/component/resolve/plugins/` | Thomas named a plugin. Nothing imports it, so `module-tiers.md` places it under `internal/plugins/` as a full-subsystem plugin; the codec both sides use stays in `resolve/irr/store`, already imported by both consumers |
| One place says where and when: `server`, `source-address`, `refresh-interval` on the owner; the PeeringDB URL is `system peeringdb url`; a consumer names only the AS-SET or ASN | Keep a server per consumer | No consumer has a per-peer or per-rule server (A-7), so nothing is lost; `resolve.md` already places PeeringDB settings in `system peeringdb`. Pre-release: the repository's configs, `.ci` and docs are converted, no migration path |
| Consumers read the owner's keys through a new read-only state grant | Ask the owner by command; DirectBridge | A read reaches the daemon store, so it works forked, with the owner stopped, and from `OnStarted`; a cross-plugin command from `OnStarted` fails with "unknown command" (`ai/doors/plugin-infra.md` traps); DirectBridge does not reach a forked side |
| Change notice: owner `emit-event` with a monotonic generation, consumers subscribed at stage 5; a generation gap triggers a full re-read | Notice only; consumers poll | Stage-5 subscriptions are live "before any event flows", so subscribe-then-read leaves no gap; the generation recovers any lost notice at the next one; polling is a consumer schedule Thomas removed |
| The owner learns what to fetch from each consumer's interest key plus an interest notice | The owner reads the consumers' config roots; consumers call an owner command | Reading the consumers' roots would copy their reference parsing into the owner and name their config; a command needs the owner up; a stored interest survives either side restarting |
| Pointer `meta/irr-set/{as-set}` flipped after the dataset write | Consumers form the dataset key from the owner's configured server | A server change would leave consumers reading an empty key until the new server answers; the pointer keeps the old copy enforced |
| `update irr ...` and `clear irr ...` move to the owner; the consumer `update` and `clear` verbs are deleted; `show` verbs stay on consumers and the owner adds `show irr` | Keep consumer commands that forward to the owner | One fetcher, one command set; forwarding would be two paths for one operation (no-layering); `show bgp irr` still answers what BGP enforces |
| Firewall and domain commits keep refusing a reference with nothing stored (Thomas, design gate); a BGP commit is accepted and the owner fetches right after it | Fetch in verify | Verify does no network I/O by design; BGP already fails closed with reject, hold and re-admit, consistent with "first start with no stored copy fetches at once" |
| Dataset key (normalised server, AS-SET), schedule anchored on `checked-at`, compact binary value under a per-key bound below `MaxMessageSize`, stored PeeringDB binding, cold fetch at once | (kept from the first design) | A different server is different data; `refreshed-at` stays old for one-family sets; JSON of 1,000,000 prefixes is about 20 MB, over the 16 MiB message |
| Domain stored `expires-at` renewed on every answer (Thomas, design gate: rejected the hourly cap) | Renew at most hourly; only on the existing write events | Exact expiry across a restart; each write is a same-size in-place value (`zefs-format.md`), so it costs no full rewrite |
| `firewall-domain` stays its own DNS fetcher and keeps its timing | Move DNS into the `irr` plugin | It resolves through the engine resolver with per-answer TTLs, a different source and schedule (Thomas, design gate) |
| Convert `meta/irr/{name}` once in the owner; delete the `meta/bgp/irr-cache` migration | Drop and cold-fetch | The old keys are the firewall's enforced data; dropping them changes the firewall at the upgrade boot |
| Implementation departure, AC-17: `meta/bgp/irr-cache` is ignored, not dropped. Nothing reads, removes or registers it: `migrate` and `zefs.KeyIRRCache` are deleted (6e67f01e6d), and `internal/plugins/irr/register_test.go` `TestIRROwnerGrantsItsKeysOnly` asserts the owner holds no grant on it | Remove the key at the owner's first start | Removing it needs a key registration and a grant kept only for that one removal. Ze is pre-release, no shipped box holds the key, and an unread key changes no behaviour: the AS-SETs it named are fetched once, as the AC's survivor says |
| Implementation departure: `show bgp irr` no longer reports the next refresh (Files to Modify above) | Read the owner's schedule into a stored key for the consumer | The schedule lives in the owner's worker memory, derived from the stored `checked-at` and `system irr refresh-interval`; `show irr` answers it, and `show bgp irr` answers what BGP enforces |
| Implementation departure: the generation is raised before the pointer write (Data Flow step 4) | Flip the pointer, then increment | The pointer then names the generation its own write raised, so a consumer compares the pointer's generation with the notice's and with the last it applied without a second read of the counter |
| Implementation: AS-SET names take one spelling, upper case, in every key (`resolve/irr/store/pointer.go` `ASSetName`); the owner and both consumers pass every configured or typed name through it | Keep the operator's spelling in the key | RPSL names are case-insensitive; "AS-Foo" and "AS-FOO" would otherwise be two datasets fetched on two schedules |

### Open questions only Thomas can answer

None remain. Q-1 to Q-7 are answered: Q-1 to Q-4 by the design gate (one owner, timing on the fetching plugin, commit refusal kept, DNS expiry renewed on every answer) and Q-5 to Q-7 afterwards (`system irr`, default 3600 with 0 manual only, `clear` removes and names the consumers); each is recorded in the design-gate decision table under Task.

| Departure Q-6 brings | Before | After |
|----------------------|--------|-------|
| `firewall-irr.md` "Auto-refresh is off by default" | A firewall IRR set never refreshes unless `firewall irr refresh-interval` is set | A firewall IRR set refreshes every 3600 s by default, the same dataset BGP refreshes; `system irr refresh-interval 0` restores manual only for every consumer; the page is rewritten to say so |

### What the amendment of `plan/post-release/features/spec-irr-apply-policy.md` must say

| Item | Amendment |
|------|-----------|
| Depends | `plan/release/fixes/spec-fetched-data-survives-restart.md` |
| Fetched list | The owner's dataset behind `meta/irr-set/{as-set}`, not `meta/irr/{name}`; the fetch and its timing are the `irr` plugin's |
| A-1 | Withdrawn: datasets are per server, and there is one server setting |
| Applied snapshot | Written by the consumer as `meta/irr-applied/<consumer>/{as-set}`, naming the dataset generation it applied; read by name |
| Apply windows | A consumer decides when it publishes a new generation; the owner's schedule decides only when it is fetched |

## Known Limitations
- RPKI data is not stored across a restart (RFC 8210 Section 6).
- A DNS name whose stored expiry passed is resolved at start: the TTL bounds determinism for DNS data (RFC 1035 Section 3.2.1).
- `bgp-filter-irr` has no on-demand enrolment path today; an ASN is enrolled only by configure.

## RFC Documentation (Scope: protocol)

Not protocol scope. The domain expiry code carries the RFC 1035 Section 3.2.1 TTL quote above the statement that arms a unit from its stored expiry.

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
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-23 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: `./le test unit pkg path ./internal/plugins/irr/`, `./le test unit pkg path ./internal/component/resolve/irr/store/`, `./le test unit pkg path ./internal/component/bgp/plugins/filter_irr/`, `./le test unit pkg path ./internal/component/firewall/plugins/irr/`, `./le test unit pkg path ./internal/component/firewall/plugins/domain/`, `./le test unit pkg path ./internal/core/statestore/`, `./le test unit pkg path ./pkg/plugin/rpc/`, `./le test functional plugin`, `./le test functional reload`
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A with the reason above)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-fetched-data-survives-restart.md` only, in the same `./le commit create` script

## Implementation Summary

### What Was Implemented
- The `irr` plugin (`internal/plugins/irr/`): `system irr { server; source-address; refresh-interval }`, interest-driven fetch, schedule from `checked-at`, digest-keyed datasets with pointer flip, generation and notice, `update irr`, `clear irr`, `show irr`, legacy conversion, doctor checks, metrics, AS-SET completion (commits in the table at the top).
- `bgp-filter-irr` and `firewall-irr` read the stored copies through `resolve/irr/store/consumer.go` and fetch nothing; their consumer commands and settings leaves are deleted.
- `firewall-domain` stores `expires-at`, renewed on every answer, and arms each name at it.
- Third closure: `store.ReadDataset` follows a pointer that moved under the read; `store.DecodePointer` refuses a server no dataset key can carry; the stale `numberparse-allowlist.txt` line and the IRR feature declaration are fixed.

### Bugs Found/Fixed
- The rows of "Defects found and fixed in this work" above, and the Review Gate findings below, each with its test.

### Documentation Updates
- Every page of the Documentation Update Checklist, edited with the phase that changed it (commits above); third closure: `docs/architecture/irr-owner.md` "What a consumer reads" (`ReadDataset` row, anchor `consumer.go -- ReadDataset`), `features/irr-bgp-import-filtering.md` (Description, Components, tests; anchors `filter_irr/stored.go -- storeReader`, `irr/datasets.go -- writeDataset`).
- `./le doc check all`: fails only on 36 OSPF "unknown operator group label" findings of another session; no finding names a file of this spec.

### Deviations from Plan
- The implementation departures recorded under Key Design Decisions (AC-17 `meta/bgp/irr-cache` ignored, `show bgp irr` without next refresh, generation raised before the pointer write, one AS-SET spelling) and the dataset key `meta/irr-data/{server}/{as-set}/{digest}` (b3c34bcb90).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The rows of "Mistake Log: Wrong Assumptions" above (A-5, A-4, phase-1 test, phase-7 fixture, phase 3, phase 4, tooling) | As recorded there | Implementation and the first closure | Fixed in the commits named there |
| approach | The dataset key named the server and the set only, so a same-server refresh overwrote the bytes the current pointer named (R1-1) | Each value needs a key of its own for the pointer flip to be the only switch | Second closure, Review Gate round 1 | b3c34bcb90 |
| approach | Removing the replaced dataset after every flip (b3c34bcb90) left a reader that read the old pointer before the flip refused after the removal | The reader must follow a pointer that moved | Third closure, round 1 | `store.ReadDataset` follows once |
| approach | The third closure's round 2 passed a test helper parameter that only ever took one value, and an exported `store.InterestKey` that only tests outside the package used | Lint (unparam) and `./le repo check` refuse both, and a review round does not replace running them | The main thread's owed lint and repo check | C3-5, C3-6, reviewed in round 3 |
| approach | The feature declaration `features/irr-bgp-import-filtering.md` (Documentation checklist row 1) was not updated by phase 4 and still said the lists live in memory only | Phase 4 owed it | Third closure, step 4 | Rewritten in the third closure |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Fetched IRR data saved in its own stored values, loaded at start | Done | `internal/plugins/irr/datasets.go` `load`, `writeDataset`; `resolve/irr/store/consumer.go` | |
| Refreshed only at the configured frequency | Done | `internal/plugins/irr/schedule.go`, `config.go` | 0 = manual only |
| One fetcher, shared copy for BGP and firewall | Done | `internal/plugins/irr/`; consumers read only | |
| DNS answers loaded at start, resolved after stored expiry | Done | `firewall/plugins/domain/schedule.go` `reset`, `cache.go` | |
| Config stays small (`system irr`) | Done | `internal/plugins/irr/yang/ze-irr-conf.yang` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `test/plugin/irr-restart-keeps-bgp-list.ci` | |
| AC-2 | Done | `test/plugin/irr-restart-keeps-firewall-sets.ci` (QEMU guest) | |
| AC-3 | Done | `internal/plugins/irr/schedule_test.go` `TestDueFromCheckedAt` | |
| AC-4 | Done | `test/reload/bgp-filter-irr-*.ci`; owner schedule unchanged by configure | |
| AC-5 | Done | `test/reload/irr-bgp-commit-new-set.ci` | |
| AC-6 | Done | `test/plugin/irr-cold-start-stores.ci` | |
| AC-7 | Done | `TestFailedFetchKeepsPointer`, `TestRefreshOutcomeCountsEmptyDistinctly` | |
| AC-8 | Done | `test/plugin/irr-shared-dataset.ci`, `TestNoticeGapRereads` | |
| AC-9 | Done | `TestServerChangeKeepsOldUntilNew` | |
| AC-10 | Done | `ze-irr-conf.yang` range and default; `schedule_test.go` | |
| AC-11 | Done | `TestDiscoveryFollowsPeeringDB`, `TestStoreResolverDiscover` | |
| AC-12 | Done | `TestDatasetLargeFitsOneKey`, `TestPluginStateDeclaredBoundBothTransports` | |
| AC-13 | Done | `test/plugin/firewall-domain-group-restart-keeps-answers.ci`, `TestResetUsesStoredExpiry`, `TestEveryAnswerRenewsExpiry` | |
| AC-14 | Done | `TestReadmitterNoLookup` | |
| AC-15 | Done | `test/plugin/filter-irr-update.ci`, `firewall-irr-update.ci` | |
| AC-16 | Done | `clear_hold_test.go`; `clear irr` names the consumers | |
| AC-17 | Changed | `TestLegacyEntryConverted`; `meta/bgp/irr-cache` ignored (Key Design Decisions) | |
| AC-18 | Done | No evaluator wait: reject, hold, re-admit (`filter_irr/readmit.go`) | |
| AC-19 | Done | `internal/plugins/irr/doctor.go`, firewall and domain doctor checks | |
| AC-20 | Done | Deliverable grep below returns nothing | |
| AC-21 | Done | `TestHandshakeConfigureAppliesNothing` | |
| AC-22 | Done | `TestDatasetDigestRefused`, `TestReadSetRefusesADatasetThePointerDoesNotName` | |
| AC-23 | Done | `test/plugin/irr-update-then-firewall-commit.ci`, `TestVerifyRefusesNoPointer` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Unit tests of the TDD table | Done | the files that table names | green in `job-r3-units-cc92635e.log` and the 6e67f01e6d evidence |
| Seven functional tests | Done | the Functional Tests table | darwin plugin suite and QEMU guest |
| `TestReadSetFollowsAPointerThatMovedUnderTheRead`, `TestReadSetRefusesAPointerServerNoKeyCarries` | Done | `resolve/irr/store/consumer_test.go` | third closure; owed run below |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Files to Modify and Files to Create | Done | committed in the commits at the top; `internal/plugins/irr/` also holds `owner.go`, `datasets.go`, `resolver.go`, `legacy.go`, `completion.go`, `metrics.go` |

### Audit Summary
- **Total items:** 5 requirements, 23 ACs, the TDD table, the file plan
- **Done:** all but AC-17
- **Partial:** none
- **Skipped:** none
- **Changed:** AC-17 (recorded in Key Design Decisions and Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A reboot leaves the router enforcing the IRR lists it enforced before | functional | `test/plugin/irr-restart-keeps-bgp-list.ci` (same `refreshed-at`, zero IRR queries, listed UPDATE accepted with no reject line) and `irr-restart-keeps-firewall-sets.ci` in Ze's QEMU guest; in the 14-test darwin run `job-r3-func-bd2358e1.log` and the 15 guest tests (`job-g-*`) |
| Refreshed only at the configured frequency | unit + functional | `TestDueFromCheckedAt`, `TestColdReferenceDueNow`; `irr-cold-start-stores.ci` (exactly one fetch, none at restart) |
| One shared stored copy for BGP and firewall | functional | `test/plugin/irr-shared-dataset.ci` |
| DNS answers survive a reboot until their TTL expires | functional | `firewall-domain-group-restart-keeps-answers.ci`, green in the guest and red under the `OnConfigure` break (B-1 row) |
| A torn or concurrent write never leaves a stored copy unenforced | unit | `TestTornPointerWriteKeepsOldCopy`, `TestStartRemovesUnpointedDatasets` (each red under its break, b3c34bcb90), `TestReadSetFollowsAPointerThatMovedUnderTheRead` (owed) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None | Every AC is done; AC-17's departure is recorded | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fetched-data-survives-restart-89674350-7290-4834-a71c-6fad00afe6e8.md` (15 files, `numberparse-allowlist.txt` by `hunks`) |
| `./le spec review check` | `review_gate: OK (9 code files, clean, hashes match ...)` |
| Rounds | 3 |
| Reviewer lenses used | round 1: wiring, removed behaviour, logic and races, security and stored-input panics, docs drift, style pass over every changed Go file; round 2: the round-1 fixes; round 3: the main thread's lint and repo-check fixes (C3-5, C3-6), clean |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| C3-1 | ISSUE | `./le repo check`: `numberparse-allowlist.txt` still pre-authorizes a 32-bit parse in `store.go`, whose only one (`parseBareASN`) b3c34bcb90 deleted | `internal/le/repo/numberparse-allowlist.txt` | Line removed (committed by `hunks`, another session's hunk in the file stays) |
| C3-2 | ISSUE | Since b3c34bcb90 the owner removes the replaced dataset after every flip (`datasets.go` `writeDataset`), so a consumer that read the old pointer before the flip and the dataset after the removal got `ErrDatasetNotPointed`: a false `stored-copy-unreadable` WARN (`firewall/plugins/irr/stored.go` `record`, BGP `storeReader`) and a firewall commit refused at verify (`irr.go` `readStoredCopy`) | `resolve/irr/store/consumer.go` `ReadDataset` | Reads the pointer once more on a missing dataset and follows a pointer that moved; `TestReadSetFollowsAPointerThatMovedUnderTheRead` |
| C3-3 | ISSUE | Style pass, panic: `DecodePointer` accepted a server holding `..`, and `Pointer.DatasetKey` then reached `zefs.KeyEntry.Key`'s `..` panic in the owner's `load` and in both consumers' readers; reachable from stored bytes (`ze data write`, a damaged store), not from a socket | `resolve/irr/store/pointer.go` `DecodePointer` | Refuses a server holding `..` or `/` as `ErrPointerMalformed`; `TestReadSetRefusesAPointerServerNoKeyCarries` |
| C3-5 | ISSUE | Lint (unparam): the test helper `storeSet` only ever got "AS-TEST" | `resolve/irr/store/consumer_test.go` `storeSet` | Parameter dropped, a local `const asSet = "AS-TEST"`; callers follow; no assertion changed |
| C3-6 | ISSUE | `./le repo check`: exported `store.InterestKey` had no cross-package non-test caller | `resolve/irr/store/pointer.go` | Unexported as `interestKey` (body `zefs.KeyIRRInterest.Key(consumer)`, unchanged); the four outside tests (`irr/completion_test.go`, `irr/owner_fake_test.go`, `firewall/plugins/irr/stored_test.go`, `filter_irr/stored_test.go`) name `zefs.KeyIRRInterest.Key(...)`, the same key, so each asserts what it did |
| C3-4 | ISSUE | Documentation checklist row 1: `features/irr-bgp-import-filtering.md` said the lists live in memory only and the filter registers refresh commands | `features/irr-bgp-import-filtering.md` | Description, Components, tests and doc review rewritten |

NOTEs (not blocking): `ze_irr_data_age_seconds` reads 0 with nothing stored (`metrics.go` `publishMetrics`), as the firewall gauge it replaced did; `daemonKnownASSets` (`completion.go`) logs its WARN on every completion while an interest key is unreadable; `irrCountQueries` answers 0 for a missing query log, which the cold-start test's exactly-once count excludes for the shared runner layout; `features/irr-bgp-import-filtering.md` Extra criteria names `test/plugin/filter-irr-recorded-mirror.ci`, which never existed in git; `internal/le/site/testdata/published-*` are frozen site-generator fixtures.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/plugins/irr/{register,irr,schedule,command,doctor,config}.go`, `yang/ze-irr-conf.yang`, `yang/ze-irr-cmd.yang` | Yes | `ls`, third closure |
| `internal/component/resolve/irr/store/dataset.go`, `docs/architecture/irr-owner.md` | Yes | `ls`, third closure |
| The seven `.ci` files of the Functional Tests table | Yes | `ls`, third closure |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2, AC-6, AC-8, AC-23 | Restart, cold start, shared set, update-then-commit | `job-r3-func-bd2358e1.log` (14 IRR plugin tests, darwin), `job-g-*` (15 firewall IRR tests, QEMU guest) |
| AC-20 | No removed leaf or command left | `grep -rlnE` over `test` and `docs` for `policy irr`, `peeringdb-url`, `update bgp irr`, `update firewall irr` and `clear firewall irr` returns nothing (third closure) |
| AC-22 | Unusable dataset refused, never "nothing stored" | `job-r3-units-cc92635e.log` (`resolve/irr/store` green); owed rerun below |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Daemon restart, BGP and firewall consumers; cold start; shared set; BGP commit; update then firewall commit; domain restart | the seven Wiring Test `.ci` files | Read in the third closure: `irr_owner_fixture.go` `irrRestartScenario`, `register_irr_restart_keeps_bgp_list.go` `irrRestartKeepsBGPList` assert stored copy, same `refreshed-at` and zero queries |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 to A-4, A-6 to A-9 | confirmed | the Assumptions table above, each row's evidence |
| A-5 | broken | Mistake Log row A-5; phase 1 added `--query-log` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `irr-owner.md` `ReadDataset` row | `consumer.go` `ReadDataset` | Yes |
| `features/irr-bgp-import-filtering.md` | `filter_irr/stored.go` `storeReader`, `irr/datasets.go` `writeDataset` | Yes |
| No page still says the IRR lists live in memory only | `grep -rni 'memory only' docs features` hits only the reversal paragraph of `filter-irr.md` | Yes |

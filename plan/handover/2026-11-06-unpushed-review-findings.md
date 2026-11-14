# Handover: review of the 108 unpushed commits

**Range:** `@{u}..HEAD` on `main`, 108 commits, tip `54c5e8af0c`. Nothing was pushed and no push is authorized.
**Owner order:** Thomas asked for a review of the commits pending push, then said: "fix all the issues identified", "use agents", and "make sure you remember where we are because another session is running".
**Recommendation given to Thomas:** do not push until findings 1 to 4 are fixed. He answered by ordering the fixes. He gave no push order.

## Update 2026-11-14 (read this first)

Four of the items this file left open were settled on 2026-11-14. The rest of the file is
unchanged and still describes them as open.

| Item | Outcome |
|------|---------|
| Finding 1, the inventory refusing the real tree | Gone: `./le repo inventory` runs on the real tree, exit 0, with no "unresolved execution ownership" refusal |
| Finding 29, the migrated inventory tests with no recorded run | `./le test unit pkg path ./internal/le/repo/inventory/` passes |
| Finding 16, lock order between the reactor and the plugin server in peer removal | Real, and fixed in `a6cb751160`: the removal capture took the process manager's lock while holding the reactor's, and a plugin restart holds that lock across its join. The capture now takes only leaf locks. A test fails on the old code and passes on the new |
| Finding 28a, no negative test for RFC9234-5-9 | Not closed, and larger than a test: Ze does not enforce 5-9 at all. An independent judge refused the test written for it, which proves 5-6 at the egress filter and landed as such in `028620f16e`. The gap is a row in `plan/handover/39-residuals-of-closed-handovers.md` and in the coverage limits of `rfc/short/rfc9234.md` |

Still open here: the adj-rib-in replay break that was never run, finding 30 (stalls tolerated,
not explained), and the items the Status table hands to other specs.

## Status

Final state. The fixes landed in `715fa2833e..7d397b3250`. Nothing was pushed, and no push is authorized.

| Finding | State | Where |
|---------|-------|-------|
| 1, 7, 8, 9, site notes | Open, held | Another session still has live uncommitted work under `internal/le/repo/inventory/`, `plugin/coupling/`, `plugin/gatedtable/` and `internal/le/site/`. Re-run `./le repo inventory` once it lands |
| 2 | Fixed | 03cf855074: a refused reload no longer republishes the old tree, and the nine reload reds pass with their tests unedited |
| 3 | Fixed | 03cf855074: a fatal plugin stops the daemon only when it failed, not when the reload's caller ended |
| 4 | Fixed | eead8aea7a: rfc record, feature run and the nightly run through Ze's guest. 34549af4be repaired the HEAD build that eead8aea7a broke |
| 5 | Fixed | 1e65b6d4bb: a stale route older than the export policy runs the whole chain. The remaining gaps are owned by `spec-bgp-refresh-reapplies-export-policy` |
| 6, 20, 21, 22, 23, 24 | Fixed | 926f01f732: LCP lifetimes restart from config, the PAP wait answers LCP, proxied PAP renegotiates, disabled IPCP is rejected, MTU is set before connect, a malformed Magic renegotiates. Interop for proxied PAP is owed to `spec-l2tp-proxied-pap-interop-and-identity` |
| 10, 11 | Fixed | eead8aea7a: the image is keyed by kernel bytes, and a root pinning another Linux release is refused |
| 12 | Fixed | 0cb6989326 |
| 13 | Fixed | 03cf855074 corrected the coordinator comment |
| 14 | Owner decision | Should a daemon stopped by a fatal plugin exit non-zero? `test/plugin/failure-policy-fatal-live-peer.ci` still expects 0 |
| 15 | Fixed | 44e0550ad9: shutdown's wait on a stuck editor session is bounded |
| 16 | Unverified | No inversion found, not every lock holder audited |
| 17, 19 | Fixed | 1e65b6d4bb: forward_pool comments corrected, adj-rib-in replays with its mutation owner released. Its break was not run |
| 18 | Comment fixed, defect owned | 1e65b6d4bb corrected the comment. End-of-RIB before routes on a forked RIB is owned by `spec-bmp-locrib-dump-end-of-rib-before-routes` |
| 25 | Recorded gap | RFC1661-4.6-4, unchanged |
| 26 | Fixed | 35749ca2dc: RFC1661-6.1-1 positive breaks the read path, and its two orphan rows are gone |
| 27 | Classified | After 7d397b3250, `./le rfc check` names 71 rows: 58 pppoeclient rows that predate this work, and other sessions' stale or shifted verdicts. None left is this work's |
| 28 | Open | RFC9234-5-9 still has no negative test. RFC5082-3-2 still names line 96, unchecked |
| 29, 30 | Open | 29 is held with the inventory package. 30: the stalls are still unexplained |
| 31 | Done | Superseded lines landed with this handover |
| New, from the audits | Owned | RFC4271-5.1.2-2 rs rail and the LOCAL_PREF claim: `spec-bgp-refresh-reapplies-export-policy`. Panic breaks on RFC1661 and RFC2918 rows: `spec-rfc-evidence-strength-1-targeted-mutant-ratchet` |

Owner decisions still open: the fatal-plugin exit code (14), and whether RADIUS accounting carries the LAC name (`spec-l2tp-proxied-pap-interop-and-identity`).

Reds this work did not cause: `TestEveryIntegrationPackageIsNamed` (pppoeac, owned by `spec-qemu-nightly-guests-run-ze-system`, plus another session's site test), `TestEveryWorkflowNativeActionExists` (another session's `codeql.yml`), `TestInjectionConsumerCancellationJoinsDescendants` (journaled in ef85422a40), website catalog drift in `./le doc check all`, and the 58 pppoeclient RFC rows.

Not proven: interop for proxied PAP, no interop scenario for the stale rail, the converted nightly PPPoE and ASPA steps and `_guest-go` not boot-proven, the adj-rib-in replay break not run.

Lesson: package tests compile the working tree, so a commit that leaves a needed hunk behind stays invisible until `le` builds HEAD. That broke HEAD once (eead8aea7a called a function its commit did not carry, fixed by 34549af4be). Wait for the HEAD build after each Go commit.

## Ownership

Another live session has uncommitted work in `internal/le/repo/inventory/`, `internal/le/plugin/coupling/`, `internal/le/plugin/gatedtable/`, `internal/le/site/` and `INVENTORY-EXTERNAL-EFFECTS.md`. It holds about 100 new files, and they implement the external-effect analysis this inventory refusal needs. Finding 1, finding 7 and the site notes stay untouched until that work lands. Re-run `./le repo inventory` when it lands, then decide what is left. Do not edit, revert, format or commit those files from this work.

## Rules that bind the fix work

- Fix agents edit sources only. All tests, builds, lint and formatters run in the main thread, one CPU-heavy job at a time.
- The main thread commits, through `./le commit create` and the printed `script=`.
- A change that could lower what a test proves goes to an independent `ze-test-judge`. Finding 2 needs this, because its fix changes reload test expectations.
- A tagged RFC unit that changes owes a fresh record from `./le rfc record`, run from the main thread.
- A protocol design reads FRR's and BIRD's source and quotes the deciding function. Finding 5 needs this.
- Independent review precedes landing a fix.

## Evidence commands that worked

Package tests judge a tree where no gated plugin registers unless they carry the feature tags (`docs/contributing/running-commands.md`):

```
dir=$(./le session scratch ensure)
tags="ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')"
./le job run label <label> command go test -count=1 -tags "$tags" ./internal/component/plugin/server/ > "$dir/<name>.log" 2>&1
```

- A raw `go test`, and any expensive command piped through `tail`, `head` or `sed`, is refused by a hook. Write to a log, then read the log.
- `plugin/server` has **9 reds with tags**. An earlier count of 40 came from an untagged run and is not a real figure.
- `./le repo inventory` reproduces finding 1.
- `./le commit debt-status` reports nothing owed. It does not judge behavior.
- Review from `git show <sha>` and `git show HEAD:<path>`, because the working tree carries other sessions' edits.

The mechanical scan of the range found all 108 commits signed, with no conflict markers, secrets, added test skips or new TODOs.

## Findings

**Evidence** says how each finding is known: **run** means observed in a run, **read** means read in source and not executed, and **unverified** means the question is still open. Findings 1 to 6 are the six in the report to Thomas. The severity shown is the reviewer's, or the owner report's where they differ.

### Reported to Thomas as defects

**1. BLOCKER, 5a6bd238c1: the inventory refuses the real tree.** `callablealiases.go` `callableSource`, `methods.go` `valueType`.
The init-graph walker refuses every method call on a local that holds a multi-result standard library return. `internal/appliance/cmd_build.go` has `e2fsMkfs = ResolveE2FSTool(...)`, a package var initializer the walker follows through `e2fsSearchDirs`, `brewKegDirs` and `brewPrefixes` to `homebrew.go` `brewPrefixes`, `if info, err := os.Stat(p); err == nil && info.IsDir()`.
Failure: `./le repo inventory` fails with the message below. From source, `./le plugin coupling` fails, the `plugin gated-table` step fails in `./le repo update` and in the generated check, and `le site` loses its plugin registry. The verify stages run `plugin gated-table check` and `plugin coupling check`, so a push should redden CI. `coupling_test.go` calls `Plugins(root)` too, so the commit body's one named red (`TestAnswerAnswersTheInventory`) understates the reach. Docs at HEAD say `./le repo inventory` prints current counts (`docs/contributing/ci-test-coverage.md`, `docs/architecture/api/json-format.md`).

```
error: committed inventory internal/appliance/homebrew.go:82: callable receiver has unresolved execution ownership; rewrite the registration as an immutable declaration before publishing inventory
```

Evidence: run for the command refusal. Read for the consumers.

**2. ISSUE, 7d1edaeec3 (foreshadowed by f4dca4ecab): tests assert the opposite of reload.** `plugin/server/reload.go` `reloadConfig`, the `recordReloadCompensation(ctx, running, newTree, nil)` before auto-load.
Every rejected reload past that line now reaches `finish(false)`, `compensateReload`, `restoreReload`, `ApplyConfigDiff(before)`, then `SetConfigTree(before)`. Production effect is benign but unasserted: a refused reload re-applies and republishes the old tree, so nothing proves it leaves the running tree alone. f4dca4ecab named these as shared-tree reds. 7d1edaeec3 landed the code without changing `reload_test.go` and names no reds.
Reds: `TestReloadRefusesAChainItsAutoLoadedPluginRefuses`, `TestReloadConfigAutoLoadDependencyFailureFailsClosed`, `TestReloadFailureStopsAutoLoadedPluginByName`, `TestReloadConfigVerifyFails`, `TestReloadVerifyCrashedPlugin`, `TestReloadProcessDiedBetweenVerifyAndApply`, `TestReloadApplyErrorReturned`, `TestReloadTxCoordinatorRollback`, `TestReloadConfigMultiplePlugins`.
Evidence: run with tags. The package has no uncommitted edits. Changing the expectations needs `ze-test-judge`.

**3. ISSUE, 7d1edaeec3: a reload's context can stop the daemon through a fatal plugin.** `startup_autoload.go` `startAutoLoaded`, then `startup.go` `runPluginPhaseWithin`, then `failure_policy.go` `stopDaemonOnStartupFailure`.
Auto-load startup moved from `s.runPluginPhase` under `s.ctx` to the reload's own `ctx`. The fatal check does not ask whether the handshake ended because the plugin failed or because the caller's context ended.
Failure: a commit adds config that auto-loads a plugin declaring `failure-policy fatal`. The plugin passes Stage 1, then the 30s reload budget (`cmd/ze/hub/main_reload.go`) expires during its handshake, or `Stop` cancels the transaction. The stage ends below `StageRunning`, `recovering` is false, and `stopDaemonForPlugin` stops the daemon, where a rejected reload should leave it running.
Evidence: read. I also read `stopDaemonOnStartupFailure` and confirmed it has no context check. Unread: whether the request context in `cmd/ze/hub/api.go` is cancelled on client disconnect, which would make a disconnect take the same path.
Fix direction: exempt a failure whose cause is the caller's context, or keep the startup under the server lifetime with the stage timeouts.

**4. BLOCKER, 3265a72946 against e4bfeb7789: conntrack setup strands the Alpine callers.** `internal/le/test/qemu/alltests_conntrack.go` `guestConntrackSetup`.
`prepareConntrack` runs in both `Execute` paths (`alltests.go`) and exits 1 unless PID 1 is `/gokrazy/init`. Three producers still drive `test qemu all` through the Alpine `test qemu run` route: `internal/le/rfc/discriminate_guest.go` `guestCIArgv` (its header says it "retains the legacy Alpine guest"), `internal/le/feature/recordrun.go` `runInGuest`, and `.github/workflows/qemu-nightly.yml`.
Failure: `./le rfc record` on a needs-linux `.ci`, `le feature run` on such a `.ci`, and the nightly step all stop before any suite runs. Unit tests stay green because they pin only the argv. `qemu-integration.md` says the Alpine route is no entry point, but the callers were not converted. No false green reaches a record. This breaks the owner rule that a behavior change converts every user in the same work.
Evidence: read. The guard, the two call sites and the workflow step were read directly.

**5. ISSUE, 18c9181e4c: stale re-advertisement skips the current export filters.** `reactor/reactor_api_batch.go` `sendStaleReadvertiseUnit`, `decideStaleReadvertise`; `reactor/stale_readvertise_chain.go` `runStaleChain`.
An Adj-RIB-Out entry with `StaleLevel>0`, sent to a peer whose export chain holds a readvertise-opted member, goes to the prefiltered writer. Only readvertise members run. Before this commit the session gate still ran the whole chain. "Already ran at the original announce" holds only while the export filters are unchanged. A filter-list edit does not rewrite ribOut: `core-design.md` says "Broad filter definition reload is separate", and `reconcileExportFinal` handles only final transitions.
Failure: the operator adds a deny member to a peer's export list and reloads, with the final still accept and `llgr-egress` still in the chain. A source peer enters GR or LLGR, and `mark-stale` raises the level on its ribOut copies. The destination sends ROUTE-REFRESH (`rib.go` `handleRefresh` to `rib_commands.go` `sendRoutes`, which sets `meta["stale"]`) or reconnects (`collectPeerUpReplay`). The stale route goes out with old attributes and the new deny never runs. Non-stale routes in the same refresh go through the new chain, and so do stale routes toward a peer with no readvertise member.
RFC 2918 Section 4, as quoted by the reviewer and not yet re-read by the main thread: "Otherwise, the BGP speaker shall re-advertise to that peer the Adj-RIB-Out of the <AFI, SAFI> carried in the message, based on its outbound route filtering policy."
`TestStaleWriterDoesNotReenterExport` asserts that the non-opted member never runs, so it pins this behavior and has no reload case.
Evidence: read. The path and the test were read directly. Fix direction: treat it as a reload question. Refresh or invalidate ribOut stale copies when the export filter list changes, or run the ordinary members when the policy generation differs from the one that produced the entry. The design owes the door check and FRR and BIRD source.

**6. ISSUE, 18ea5e830a: Ze's reduced MRU survives renegotiation.** `l2tp/ppp/session_run.go` `handleLCPPacket` and `sendConfigureRequest`, `auth.go` `resetAuthNegotiation`, `mru_policy.go` `ipv6MRUSupported`.
A peer Nak writes `s.localMRU`, and no new LCP lifetime resets it. `resetAuthNegotiation` clears only auth and request state.
Failure: the peer Naks Ze's MRU to 1000 in lifetime 1, then renegotiates with no limit. Ze re-requests 1000 and the peer Acks it. `ipv6MRUSupported` stays false, so IPv6CP gets Protocol-Reject until the session dies.
False claim: 18ea added to `docs/guide/l2tp.md` "A later LCP negotiation can restore IPv6 when the new directional MRUs support 1280". The `supportsProtocol` comment says "for this LCP lifetime". Both hold only for the peer direction. No test crosses two lifetimes: `TestIPv6MRUServiceGate` sets the fields directly.
Evidence: read. A grep at HEAD confirms the Nak handler is the only non-test write to `s.localMRU`.

### Inventory consumers (held for the other session)

**7. ISSUE, 5a6bd238c1: the committed inventory mixes with working-tree inputs.** `plugin/gatedtable/gatedtable.go` `Derive`, `plugin/coupling/coupling.go` `buildModel`.
They combine a committed plugin list with `LoadGateIndex(root)` and `parseTree(root)`, so the "independent of dirty files" claim stops at the consumers.
Failure, once finding 1 is fixed: a new plugin plus `./le repo update` writes a gated table without that plugin, because HEAD lacks it. The commit lands stale, and `repo check` goes red until a second update and commit. With another session's dirty gate manifest, the table joins that session's gates to HEAD's plugins.
Evidence: read.

**8. NOTE, 5a6bd238c1: the off-Linux inventory image may lack git.** `internal/le/site/pluginregistry.go` `linuxRegistryContainer`.
Off Linux, `le repo inventory` runs in `alpine:3.24.3` (`StaticBinaryImage`). `ReadCommitted` now execs `git` (`repo/objects/objects.go`). Impact is the macOS site build only, which the owner's macOS rule defers.
Evidence: unverified. The image was not inspected.

**9. NOTE, 5a6bd238c1: stale comment.** The comment on `inProcessPluginRegistry` ("this process's own registrations") is stale since this commit.
Evidence: read.

### QEMU and image cache

**10. ISSUE, 934e7d8c03 with 307aff325e: the printed fix cannot work after an image cache mismatch.** `internal/le/test/qemu/dockerlab_command.go` `dockerLabImageBoot`, `setup/docker/host/build.go` `build`.
The image cache key uses `KernelKey: filepath.Base(tree)`, a name, while the boot check compares kernel bytes. `internal/appliance/kernelbuilder` sets no `KBUILD_BUILD_TIMESTAMP` or `SOURCE_DATE_EPOCH`, so a rebuild is inferred to change the bytes. No rebuild was run to confirm it.
Failure: `evictKeepN` removes a runtime entry and it is rebuilt under the same variant. `EnsureImage` returns the old cached image under the same key. Every `docker-lab command` refuses with "Fix: rebuild the image", and that rebuild hits the same key, so `rfc record` guest units stay blocked until someone deletes the cache directory by hand. It fails closed: no wrong kernel boots.
Evidence: read, with the non-reproducible kernel inferred.

**11. NOTE, 307aff325e: mixed kernel root and version.** `internal/appliance/runtimekernel.go` `RuntimeKernelTree`.
It uses the compiled-in `defaultKernelVersion` rather than the selected root's version file, so a root other than le's own build tree mixes that root's config with the binary's version. Reachable only when le is built from a different tree.
Evidence: read.

### Reload, SDK and CLI

**12. NOTE, c0cba00d70: the SDK boundedness claim is false for refused names.** `pkg/plugin/sdk/metrics.go` `family`.
`m.byName[d.Name] = f` runs before the `MetricsDeclaredMax` check, so every refused name stays in the map forever, while the type comment says what is held is bounded. A plugin building metric names dynamically past the cap grows the map without limit. Series of a family the engine later refuses keep their `m.series` slots until the plugin deletes them.
Evidence: read. The order was confirmed directly at HEAD.

**13. NOTE, 7d1edaeec3: coordinator comment no longer matches the lock order.** `internal/component/plugin/coordinator.go`, "Only map access and cache registration are called with mu held".
`SetConfigTree`, `GetConfigTree` and `ReloadRunning` now call the reactor under `c.mu`, and the adapter's `SetConfigTree` takes `a.r.mu.Lock`. That is a new order, `c.mu` before `r.mu`. A grep found no `ReactorAny` or `ReactorFor` re-entry from the reactor package, so no deadlock is claimed. The comment is still false.
Evidence: read.

**14. NOTE, 4348b04476: a fatal plugin stop exits 0, and a test pins it.** `test/plugin/failure-policy-fatal-live-peer.ci`, `expect=exit:code=0`.
A daemon stopped by a failure reports success, so a service manager using restart-on-failure would not restart it. `process-protocol.md` says only "The daemon stops", so this is a design question rather than a proven defect.
Evidence: read. Open: the fix agent should settle whether this is a defect, against how the daemon reports other failure stops.

**15. UNVERIFIED, aa34d659d7: SSH stop may hang.** `internal/component/ssh/ssh.go` `Stop`.
`models.wait()` deliberately has no deadline, and the 3s context covers only the network side. A stalled store write in `History.publish`, or an `Update` blocked in storage, would hold `apiServer.Stop`.
To settle: check whether zefs `WriteFile` can block without bound, and whether `autoSaveOnQuit` (called synchronously from `Update` in `model_keys.go`) writes the store.

**16. UNVERIFIED, 7d1edaeec3: lock order `r.mu` before server locks.** `peer_removal.go` `removalLocked`.
Capture now calls `getMatching`, `AllProcesses` and `advertisedClaimants` under `r.mu`. No inversion was found, but not every holder of those locks was audited.

### BGP

**17. NOTE, 18c9181e4c: stale field comment.** `reactor/forward_pool.go`, the `fwdItem` doc and the `rawBodies` field comment still say "SendRawUpdateBody per entry". `fwdBatchHandler` now routes originated raw bodies through `writeOriginatedBodyPreFiltered`, which applies next-hop, AIGP and size admission. Comment only; the behavior is correct.
Evidence: read.

**18. NOTE, 445c0fcdf1: stale BMP comment.** `plugins/bmp/bmp_locrib.go`, around `handleBestChange`, says "A replay batch IS the full table for its family (the RIB emits one batch per family)". `sdkEventBus.Emit` now splits forked-RIB batches, so this is false for a forked RIB.
No wrong End-of-RIB was seen: `dumpScope` compares `scope.replayID == batch.ReplayID`, and JSON flattens the token to `replay.Broadcast`. That holds only if the per-dump token is never Broadcast, which is unread. If it can be, End-of-RIB is sent after the first chunk.
Evidence: read for the comment, unverified for the End-of-RIB consequence.

**19. UNVERIFIED, 445c0fcdf1: lock held across peer-up replay I/O.** `plugins/adj_rib_in/rib.go` `handleState`.
Verified: `admitMutation(false)` holds `mutationMu` through the deferred unlock, so it spans `relayRoutes`, `replayFlowSpecs` and `signalSessionReady` when `replayDrivenElsewhere` is false. The comment still says replay runs after lock release "to avoid deadlock", which is now true only of `mu`. `accept-routes`, `reject-routes`, batch validate, the timeout scanner and `handleSourcePolicy` all wait out a full-table replay to a slow peer.
To settle: whether the source-policy phase budget can expire during that wait, whether `relayRoutes` can wait on an engine transaction that is waiting on `handleSourcePolicy` (a deadlock until a timeout fires), and whether `handleStructuredState` has the same span.

### PPP

The first three are the pre-existing defects from the report to Thomas. No commit in the range introduced them, and they have no journal rows. All locations are in `internal/component/l2tp/ppp/`.

**20. ISSUE, pre-existing (`acceptProxyPolicy` from dbbd2c3a32): proxied PAP stalls.** `auth.go` `acceptProxyPolicy`, `session_run.go` `run`, `pap.go` `runPAPAuthPhase`.
Proxy LCP naming PAP is admitted, but Proxy Authen AVPs 29 to 33 stop at the L2TP session (`session_fsm.go`). `StartSession` carries none of them, and PAP is peer-initiated. RFC 2661 Section 4.4.5, as quoted by the reviewer: "Note that the LNS policy may be to enter an additional round of LCP negotiation and/or authentication if the LAC is not trusted." Ze neither uses the proxied credentials nor renegotiates.
Failure: with PAP in the allow list, a LAC finishes PAP with the peer and sends ICCN. The peer is in Network phase and never resends a request. Ze drops its IPCP frames and fails at `papFirstRequestTimeout` (30s). `proxy_transmit_cap_test.go` has its scripted peer send a fresh PAP Request, so it passes without exercising this. The live proxy carrier is CHAP only, and CHAP works because Ze re-challenges.
Evidence: read, derived from code and RFC, not observed against a real LAC.

**21. ISSUE, pre-existing: LCP dropped while waiting for PAP.** `pap.go` `runPAPAuthPhase`, the `proto != ProtoPAP` branch.
While waiting for the first PAP request, every non-PAP frame is dropped, including LCP. `waitCHAPLike` routes the same frames through `handleFrame`. RFC 1661 Section 5.5, as quoted by the reviewer: "Upon reception of a Terminate-Request, a Terminate-Ack MUST be transmitted."
Failure: a Terminate-Request gets no Ack for up to 30s. If Ze's Configure-Ack is lost, the peer's retransmitted Configure-Request is discarded. The peer never opens or sends PAP, and the session fails.
Evidence: read.

**22. ISSUE, pre-existing: disabled IPCP buffers forever.** `session_run.go` `handleFrame`.
For IPCP the `ipcpState == Initial` buffer check runs before the `disableIPCP` check. IPv6CP has the correct order, and its comment states why. With `l2tp ncp enable-ipcp false` the state stays Initial, so the reject branch is dead. After `runNCPPhase` drains the list once, nothing drains it again.
Failure: each IPCP frame from an authenticated peer is copied into `earlyNCPFrames`, up to 1502 octets, and memory grows without bound. No Protocol-Reject is sent, against RFC 1661 Section 5.7, as quoted by the reviewer: "If the LCP automaton is in the Opened state, then this MUST be reported back to the peer by transmitting a Protocol-Reject."
Evidence: read.

**23. NOTE, 18ea5e830a: oversized window on reopen.** `session_run.go` `afterLCPOpen`.
`ops.connect` runs before `SetMTU`, and `resetNCP` neither lowers the MTU nor takes the interface down. When a reopened lifetime has a smaller peer MRU, the interface is up with the old MTU when the channel connects, so a queued frame can go out oversized for a very short window. Moving `SetMTU` before connect closes it. First lifetimes are safe because the interface is still down.
Evidence: read.

**24. NOTE, 7e2fb4953f: a malformed proxied Magic reads as absent.** `proxy.go` `EvaluateProxyLCP`.
Strict decoding covers MRU only. `lookupMagicNumber` reads a wrong-width Magic as absent, so a malformed LAC AVP sets `peerMagic` to 0. Ze then drops the peer's real Echo packets as wrong-magic, and keepalive fails after about 3 echo intervals. Renegotiating would be consistent with the MRU handling.
Evidence: read.

**25. NOTE, recorded gap: Max-Failure absent.** RFC 1661 Section 4.6 Max-Failure is absent and is honestly recorded as gap RFC1661-4.6-4, so it is not a new defect. Because of it, `negoTimer` never re-arms after Opened and proxy sessions never have one. A post-Open Nak loop is bounded only by Max-Configure, which resets if the peer keeps Acking.
Evidence: read.

### RFC corpus and proofs

**26. ISSUE, 18ea5e830a: RFC1661-6.1-1 lost its receive-path break.** `rfc/discrimination/rfc1661/RFC1661-6.1-1.json`, unit in `rfc1661_proxy_policy_driver_test.go`.
The tag claims "run receives the complete 1500-octet LCP Information field". The commit replaced the unit's recorded break, a `readFrames` mutant (`Read(buf[:...maxMRU])`), with the `afterLCPOpen` `setMRU` mutant, the same break as the negative polarity. The test gained a `reads` recorder, but no record shows it going red, so the positive record proves only the ioctl assertion. f8c92a7e74 says "Preserve full1500 receive discrimination", and the 16a spec calls it "semantic receive-ioctl".
The `readFrames` break survives only on two stale rows bound to `proxy_policy_driver_test.go::TestReceiveFullFrameBelowConfiguredMRU`, a file absent at both HEAD and `@{u}`, so those rows predate the range.
Fix: record the `readFrames` mutant on the current unit and remove the stale rows. Unverified: whether `./le rfc check` flags orphans.
Evidence: read.

**27. ISSUE, afffcda999, bc121becf6: the RFC check result is open and unclassified.** The CI 41685742426 follow-through handover (removed once its spec closed in 4de8347db1; the classification it left open is now a row in `plan/handover/39-residuals-of-closed-handovers.md`) said "No owned proof, judgment or metadata publication remains pending", and also records 76 check violations not all classified as unrelated.
That was a working-tree snapshot before the final audit commits. No later count is recorded, nobody owns the classification, and an RFC-check stage is in verify. `RFC2545-3-2` negative in `test/encode/new-v6.ci` is also still unproven and named as inherited.
Evidence: read. Unverified: the current count. `./le rfc check` settles it.

**28. NOTE, d14b1a9165, and UNVERIFIED RFC5082 line.** Two items.
- RFC9234-5-9: the `{gap}` was removed because it cannot coexist with the positive tag, and the text moved to a "Coverage limits" section. The disclosure stays honest and the audit verdict stays `weak`, but the row no longer carries a gap annotation. No `rfc/discrimination/rfc9234/RFC9234-5-9.json` exists at HEAD, and negative coverage is still owed. Unverified: how the public figure counts this row.
- `rfc/discrimination/rfc5082/RFC5082-3-2.json` names the break at "line 96", while `route.Hoplimit = int(p.HopLimit)` is one line earlier at HEAD. Unverified: reading what `./le rfc check` compares for a mutant route settles whether it matters.

### Tests and stale records

**29. NOTE, 5a6bd238c1: migrated inventory tests have no recorded run.** Trailers say "New migration is unrun" and "Both new Collect tests remain UNRUN". `TestCollectExtractsASingleWordRPC`, `TestCollectRPCsUseOnlyCommittedCodeAreas` and `TestCollectReadsCommittedRegistrationsAndSchemas` have no mention under `plan/` at HEAD. Their package is held by the other session.
Evidence: read.

**30. NOTE, fa795bf224, 35fdc9b9f0: stalls are tolerated rather than explained.** The session-editor wait moved from 20s to the case deadline on the owner's "ok you can", and that reading of his reply is recorded. Assertions are unchanged and 80/80 passed. The cause was traced to storage fsyncs, not fixed. The earlier 180s force and verify startup timeouts remain "unrepeatable and unexplained" per the CI handover, so they could recur in CI.
Evidence: read.

**31. NOTE: stale handovers at HEAD.** `2026-11-05-filter-wire-umbrella-handoff.md` said "The umbrella is not closed", and `2026-11-04-filter-wire-umbrella-follow-up.md` described a live closer. Both are superseded by 54c5e8af0c.
Evidence: read. Superseded lines added with this handover.

## Reviewed and clean

- MRU: `writeFrame` is the only chan fd write in ppp, its limit is `frameLen(min(peer, cap))`, and no transmit path bypasses the cap. PPPoE passes `MaxMRU` 1492.
- Test weakening: exactly 7 net `Test*` functions were removed, all in inventory, all with a trailer. No test lost assertions without a judge approval. This compared line patterns on the net diff, so a changed predicate or a weakening undone within the range could escape it. Judge independence cannot be checked from git, because verdict files live in untracked `tmp`.
- `TestRPCNameIn`: deleted, cleared by a later `APPROVE (a/b)` trailer in the same commit.
- 12 deleted specs: only 16, 16d and the umbrella hand work off, and they hand it to existing specs.
- Kernel bytes validated against those booted, audit publication binding, `suiteChild` exit codes, `reapOrphanVMs` identity, and `edd56fcb5a` fatal policy.

## Not reviewed

- Test bodies in depth: 7d1edaeec3 authority tests, aa34d659d7 and edd56fcb5a owner tests, the `transmit_*_test.go` files, and the BGP retention and receipt tests. Stub-resistance is not assessed there.
- Inventory `assembly*.go` and `familyexpressions.go`, `TreeFromPluginMap` round-trip equivalence (the removal of `BootPeersFunc` depends on it), `reload_recovery_schedule.go` beyond its entry, and `tree.go` `ToRunningMap`.
- `lns_policy.go`, `lns_negotiated_mru.go`, `radius_fixture.go`, the PPPoE checker, `pppoeclient`, and `rfc/audit` JSON contents.
- Whether runtime logs quoted by specs say what the specs claim, and a full docs and `rfc/short` claim sweep.
- Three reviewers hit tool refusals and did not fully read their doors (`ai/doors/system.md` or the BGP and PPP doors). A fresh reader should not treat their design verdicts as door-checked.

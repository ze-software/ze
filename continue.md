# Continue: the VyOS-derived subscriber specs (session `inspired`, 2026-09-08/09)

Stopped on request, out of budget. Nothing is half-committed: three specs closed
cleanly with two commits each, and the fourth is coherent uncommitted work whose
last agent was killed mid-phase.

The previous handover, the spec-closing sweep of 2026-09-07/08, is spent and has
been removed. Two of its open items resolved themselves: `spec-verification-debt-clearing`
is closed, and `test/rfc-changed/1bfe298a.md` no longer exists. One survives and is
carried in section 5.

## 1. Where this came from

Thomas asked what in a VyOS security advisory (twelve accel-ppp defects) applied to
Ze. Six audits established that eight of the twelve cannot reproduce here: four are
protocols Ze does not speak (SSTP, PPTP, IPoE, NHRP), and stack buffer overflow plus
uninitialized-memory disclosure are C failure modes Go removes. The RADIUS
authentication bypass, the one item that transfers as pure logic, is already closed
in Ze: a reply is matched on server plus Identifier and verified against the Response
Authenticator before the pending request resolves.

Four real defects came out of the audits, each became a spec, and three are now in HEAD.

## 2. Done, no action needed

| Spec | Commits | What it fixed |
|---|---|---|
| `pppoe-discovery-omits-the-mandatory-service-name-tag` | `0e1543cc9a`, `e03d31b81e` | PADO and PADS always carry exactly one Service-Name tag; a tagless PADR is refused. Closed RFC2516-5.2-2, the one published MUST gap for PPPoE |
| `pppoe-padr-replay-allocates-unbounded-sessions` | `4efaa4fd8d`, `75e3a2ef43` | A replayed PADR no longer allocates without bound. Per-MAC cap, multi-session MAC index, cookie-keyed dedup, and a `Session.State` data race closed |
| `ipv6cp-accepts-and-proposes-a-zero-interface-identifier` | `fd7cd7b44e`, `3fd87369fb` | All four RFC 5072 Section 4.1 outcomes, the pppd one-shot Nak guard, and a suggestion generator that clears the "u" bit |

Each Review Gate found a defect all its implementation phases had missed. That is
the pattern worth budgeting for: the gate is defect-finding, not paperwork.

## 3. The one spec still open

`plan/immediate/spec-subscriber-reader-loops-retry-a-failing-socket-without-backoff.md`,
`in-progress`, Phase 5/6, claimed by the stopped session. **Release the claim before
starting: `./le spec session release`.**

### Done and verified, phases 1 to 4

`internal/core/pacer` holds `Pacer`, a value type whose zero value is ready to use.
`Wait(stop <-chan struct{}) bool` returns true when the caller must exit, reuses one
timer so it allocates nothing after the first wait, and selects on the caller's own
exit signal. `Succeed()` resets. Growth doubles from a 10ms step to a 250ms ceiling.

All four receiver loops pace, log and count:

| Loop | File | Exit signal |
|---|---|---|
| `(*UDPListener).readLoop` | `internal/component/l2tp/listener.go` | `u.stop` |
| `(*Subsystem).discoveryReader` | `internal/component/l2tp/pppoe/subsystem.go` | a stop channel this work added, it had none |
| `rsReaderLoop` | `internal/component/l2tp/ppp/ra_linux.go` | `ctx.Done()` |
| `dhcpv6ServerLoop` | `internal/component/l2tp/ppp/dhcpv6_linux.go` | `ctx.Done()` |

Three counters, one per package, because `ppp` cannot import `pppoe` or `l2tp`:
`ze_l2tp_listener_read_errors_total`, `ze_pppoe_discovery_read_errors_total`, and
`ze_ppp_reader_errors_total` with a `loop` label. Each binds through
`registry.InjectPluginMetrics`, never `GetMetricsRegistry`.

Verified: `go vet` clean on darwin and `GOOS=linux`, `-race` green across the l2tp
tree and the pacer, and a third `./le verify lint run` showing zero findings on
these files.

### Owed, phases 5 and 6

**Phase 5, item 1, and it needs a judgement rather than an edit.** `waitForPADO` and
`waitForPADS` (`internal/component/l2tp/pppoeclient/dialer.go`) poll with a
`runtime.Gosched()` in a `select` default arm. The spec's AC-9 says make the read
block instead. **The spec may be wrong here and the agent was told to say so:** the
socket carries `SO_RCVTIMEO` at roughly 100ms, so a blocking read cannot check
`stopCh` more often than that, trading a bounded busy poll for a slower stop. Unlike
the other four, this loop is already bounded by `discoveryTimeout`. Read
`tryReadPADO` and the socket setup, decide, and record the tradeoff. `dialer.go` is
already modified and `dialer_test.go` is untracked: that is this spec's own killed
phase 5, not another session's work. Read the diff before continuing it.

**Phase 5, item 2.** `TestReadLoopAllocationsPerPacketUnchanged`, for AC-8.
`readLoop` builds its slot pool, free channel and release closures once at goroutine
start, and the pacer was designed to hold that. An allocation assertion over the read
path, not a benchmark nobody reads.

**Phase 6.** The functional scenario for AC-1: a socket in a persistently failing
state leaves CPU low while the log and the counter show it. The spec calls it
`subscriber-reader-failing-socket`. `test/qemu/` does not exist, so find the right
suite directory, and add the name to `netnsSelections` in
`internal/le/qemu/netns_linux.go`. That list is explicit: `validateNetnsSelection`
refuses a named test with no file but never notices a file nobody named, which is how
two `netns-link` tests under `test/plugin/` came to run nowhere.

**Then closure**, on Opus 5 in a fresh context, through `/ze-close`.

### What the Goal Validation table must say, honestly

Three `ppp` tests are written and cross-compile-verified but **never executed**:
`TestRSReaderLoopPacesAFailingSocket`, `TestRSReaderLoopStopsPromptlyWhilePacing`,
`TestDHCPv6ServerLoopPacesAFailingSocket`. They are `//go:build linux` and this host
is darwin. Do not record them as green.

## 4. Decisions waiting on Thomas

| Decision | Why it is blocked |
|---|---|
| **The four generated RFC index files, third deferral** | `rfc/enrolled.txt`, `rfc/not-enrolled.txt`, `ai/RFC-REQUIREMENTS.md`, `docs/features/rfc-status.md` are held back across three closures. `./le rfc index-update` is whole-tree, so this session's real corrections are mixed with a `Supported` row for `draft-ietf-idr-bgp-bfd-strict-mode`, whose producer is NOT in HEAD. Publishing it would claim conformance for absent code. `plan/spec-bgp-bfd-strict.md` is `in-progress` and **no live session holds it**, so nothing schedules the refresh. Landing this session's half needs one `./le rfc index-update` in the same commit as that untracked summary, which only that abandoned spec's closure can do |
| **A closed spec came back** | `plan/immediate/spec-ipv6cp-accepts-and-proposes-a-zero-interface-identifier.md` was correctly removed by `3fd87369fb`, then rewritten to disk at 12:22 still reading `in-progress` Phase 5/5. Content is preserved in `fd7cd7b44e`, so nothing is lost, but the backlog now shows a closed spec as open and `/ze-status` counts it. NOT deleted: `ai/rules/never-destroy-work.md` needs your word first |
| **The fabricated-citation class** | `plan/journal/claim-outlives-the-evidence-it-cites.md` holds 47 rows, three of them fabricated RFC quotations or non-existent section numbers. This session's own agents attributed "MUST NOT be all zeros or all ones" to RFC 5072 Section 4.1 (that sentence is not in the document) and cited "RFC 5072 Section 3.2" eight times (that section does not exist). All corrected, none remain. The class has earned a deliberate pass, plausibly a gate: a citation naming a section number is mechanically checkable against the RFC text, which would have caught all eight |
| **`RFC5072-4.1-11`** | Ze's own tentative interface identifier does not clear the "u" bit, so Ze sends in its own Configure-Request a value it now refuses to suggest to a peer. Honestly recorded as a `{gap}`. Roughly two lines now that `suggestIPv6CPInterfaceID` owns the machinery, but it changes what Ze puts on the wire and owes a tagged test plus a discrimination record. Wants its own spec |
| **`plan/spec-liveness-event-tears-down-bgp-peer.md`** | Carried from the previous handover. `design`, 13 ACs, written 2026-09-07, never reviewed by you. Its event naming is proceeding as `liveness` / `peer-down` / `keepalive-expired`; your phrasing was "tcp failure with IP". Renaming before implementation costs three constants and one YANG grouping name |

## 5. Two new specs, approved but not started

Written this session from your ask for per-user traffic reporting like accel-ppp's.
Both are `design`, both uncommitted, and you chose the two-spec split and all four
surfaces.

**`plan/immediate/spec-pppoe-subscribers-produce-no-accounting-or-telemetry.md`** is
the dependency and the smaller surprise. The collection already exists and works:
`iface.GetStats` reads per-session `pppN` netdev statistics, baseline-corrected by
`(*baselineStore).applyBaseline`, feeding RADIUS accounting, a Prometheus poller and
one CLI command. It reaches L2TP only, because `(*radiusAcct).subscribeEventBus`
subscribes to `l2tpevents` while PPPoE emits `subevents`, and `(*Subsystem).Start`
builds the poller over the L2TP reactor list. `subscriber.Session` already carries
`AccessType` and `PppInterface`, so there is no new collection to build: the fix is to
consume the shared layer. It also repairs a fail-open, where a failed stats read
appends `Acct-Input-Octets = 0` and a billing system cannot tell that from an idle
subscriber.

**`plan/immediate/spec-subscriber-utilisation-has-no-operator-view.md`** is the views:
counters on `show subscriber`, a per-session form, web and gNMI. Two defects it also
closes: `show l2tp session id`'s YANG help promises traffic counters that
`sessionJSON` never emits, and the one command that does show counters has no
per-session form because its YANG container declares no id leaf.

## 6. Constraints a resuming session must know

**This host cannot run the interop or QEMU labs.** Docker's kernel carries no `pppoe`
module (`host kernel missing PPPoE requirements: pppoe (PPPoE pppox kernel module)`)
and `./le qemu pppoe-test` exits 1 with `qemu guest evidence requires Linux`. Four
scenarios and four `.ci` tests were written across this session's specs and NONE has
ever executed. They are honest, registered, and unproven. A Linux host with PPPoE
kernel support is what they owe. Do not weaken them to reach green.

**Two interop scenarios cannot pass even on Linux, and that is correct.**
`ipv6cp-zero-identifier` and `ipv6cp-missing-option` fail fast with
`errIPv6CPNeverEngaged` because IPv6CP is unreachable in a shipped daemon:
`poolPlugin.handle` declines every non-IPv4 family and `runNCPPhase` sets
`disableIPv6CP` before any client frame arrives. `plan/spec-l2tp-ipv6-subscriber.md`
is what switches the path on, and its text now names running them.

**The IDE diagnostics in this harness are consistently stale.** Seven times this
session they reported undefined symbols or unused functions that a real `go vet`
showed clean, because they snapshot a mid-edit state. Verify with the registered
action before acting on one.

**Lint waves need distinguishing, not dismissing.** A `cache entry not found` wave
across unrelated stdlib imports is shared-machine build contention: check
`stat -f cache/go-cache`, never `df` on the checkout, since `cache/` symlinks onto
another filesystem. But one wave this session was genuinely the agent's own code
(cross-platform unused symbols, an unchecked type assertion, UK spellings), and
treating it as noise would have shipped four defects.

**`pretool-writeedit` refuses a line-number citation in spec prose.** Cite the file
and the symbol. It also locks a test body once an `RFC requirement:` tag is added,
including a tag the same session just added and wants to remove, which is recorded in
`plan/learned/HOOK-FRICTION.md` with the narrower fix.

**`./le rfc discriminate-record ... route revert` needs no mutation tool.** One agent
concluded its tags could not get records because `gomu` is absent; that is wrong.
`rfc/discrimination/rfc2516.json` and `rfc5072.json` are worked examples.

## 7. How to restart

1. `./le spec session release`, then claim the backoff spec.
2. Read the diff in `internal/component/l2tp/pppoeclient/dialer.go` and the untracked
   `dialer_test.go`. That is the killed phase, and the blocking-read question in
   section 3 is unanswered.
3. Finish phases 5 and 6, then `/ze-close` on Opus 5 in a fresh context.
4. Take the section 4 decisions to Thomas before, not after: the RFC index files have
   now waited through three closures and the debt is compounding.

---

# Continue: the plan/immediate sweep (session `immediate`, 2026-09-08/09)

Stopped on request. Nothing is half-committed and nothing is lost: 20 commits
landed, two specs closed with two commits each, and the three open specs each
have their product work in HEAD.

The session's task was to pick specs from `plan/immediate/` that no other live
session was editing, and work them. The conflict map at the start was
`internal/le/commit`, `internal/component/config`, `sysrib`, the BGP attribute
path and the plugin host, so the picks were `internal/plugins/` packages nobody
held.

## 1. What landed

Closed, spec file removed:

- `spec-vrrp-deferred-accept-mode-dataplane` (`aa7dbfbc6`, `87c62ff6f`).
  accept-mode is enforced on the dataplane and a tracked interface's failure
  lowers the advertised priority. Proven on the wire and against keepalived 2.3.1.
- `spec-policyroute-interface-list-matches-no-packet` (`841e31bb9`, `2d74298fa`).
  An interface leaf-list ORs, and its per-interface counter rows stay separate.

Product work in HEAD, spec still open: OSPF auto-cost (`aca27077f`, `4b9e49342`,
`db3f18b0b`, `5a9522302`), ddos local `max-mitigation-duration` (`80ec03b03`,
`4008c0c98`, `b11fa5c23`, `d7104cb7b`, `c23134ce1`), IPsec dataplane inspection
(`03a77aa12`).

## 2. The three open specs, in the order to take them

**`spec-ospf-auto-cost-reference-bandwidth` — closest to done.** Three review
rounds, now 0 BLOCKER and 0 ISSUE. All that remains is `/ze-close`: the AC walk,
`./le spec session review record`, the learned summary, then the two closure
commits. The session claim is already on this spec.

**`spec-ddos-timing-leaves-reach-no-worker` — one open BLOCKER.** Round 5 found
`clearStaleDropRule` runs before the FIREWALL engine is configured, not merely
before ddos-local is: only the handshake is tier-ordered (`runPluginPhase`,
`internal/component/plugin/server/startup.go`), so `ApplyAll` autoloads the nft
default. On a `firewall { backend vpp; }` box the sweep clears an nftables table
that never held the rule while the VPP drop survives, and the guide says the rule
is removed "whatever put it there". The reviewer's fix shape is to move the call
to the head of ddos-local's `OnConfigure`, where the tier order does put the
firewall engine ahead; verify that at `runPluginPhase` before relying on it. The
round 5 section is at `tmp/session/2026-09-08-0a21e591-.../scratch/round5-review-gate.md`
and is NOT yet in the spec. Its ISSUE 27 goes with it: the guide's exposure table
is wrong in both directions, since a cold p99 cache leaves PPS detection undelayed
while a never-run box has no armed BPS trigger for about 390 seconds.

**`spec-ipsec-dataplane-inspection` — nothing has run.** The two `.ci`
(`test/ipsec/ipsec-show-dataplane-kernel.ci`, `ipsec-show-sa-counters.ci`) and the
`dataplane-readback` strongSwan scenario were written in `03a77aa12` and have
never executed, because the tree would not build. That blocker has cleared:
`GOOS=linux go build -o bin/<name> ./cmd/ze` exits 0. Run all three, do the
vacuity walk on each, and replace the spec lines that say they are owed a run.
Their assertions are claims until then.

## 3. Decisions Thomas made, so they are not re-opened

- **2026-09-09, OSPF re-price.** A `reference-bandwidth` commit must NOT tear down
  established adjacencies. Implemented in `db3f18b0b`. Removing the restart alone
  would have been wrong: origination for a re-pricing only ever happened as a side
  effect of the neighbor churn the restart caused, so `reconcile` had to originate
  explicitly.
- **2026-09-09, ddos on shutdown.** In his words: no claim on shutdown, the rule is
  not saved because a reboot may be how an operator clears state, and the attack is
  re-detected and re-instantiated on the next start. Implemented in `c23134ce1`.
- **2026-09-08, RFC-tagged test.** He approved the arity-only edit to
  `TestOwnerAutoDetection`; the row is `test/rfc-changed/ff31c770.md`.

**Open for him, asked and unanswered:** an interface's own `cost` leaf still
restarts that interface, so ze gives two different answers to what a cost change
costs. Extending the ruling is not a one-liner: `reconcile`'s re-pricing arm never
writes `e.running`, and `lsdbTopology` prices from that map.

## 4. What the review rounds cost, and why they were worth it

Every spec in this sweep was already believed done by an earlier session, and every
one of them was wrong in a way only an independent round found. ddos took five
rounds and each returned a real BLOCKER, all of the same class: an nftables drop
rule left in the kernel with nothing able to remove it, reached by a config apply,
by a config-block deletion, by the fix's own dispatch race, and by a restart.

The reviews were also wrong twice, and both times the implementer caught it by
reading the producer: the round-2 diagnosis of the removal path (the removal DOES
reach the plugin, as an empty body), and the round-3 placement of the retirement
flag (inside `adoptMitigation`, past an early return that skips idle responders).
Do not apply a review's suggested patch without checking it.

**Tests that passed for the wrong reason, three times.** `ddos-local-config-removed.ci`
passed against code that withdrew nothing, because stopping ddos-local orphan-stopped
the firewall engine, which flushes every ze-owned table: it needs a copp block so a
second dependent keeps the engine alive. `policy-interface-list.ci` could not fire its
rejections with one rule, because the reverted code still produced one rule. The ddos
clear-arm test read `status()` rather than the firewall. Force the red before trusting
any of it.

## 5. Recorded, not fixed

Journal rows this session added, each verified at the producer:

- `zero-value-as-valid-answer.md` — `emitCleared` builds every `AttackCleared` with
  an empty `DstPrefix` and `finalize` matches on it, so an incident opened with a
  resolved victim never closes. Second ddos row in that class.
- `counter-counts-the-wrong-packets.md` — `applyChain` prepends the counter ahead of
  the term's matches, so every rule in a chain counts every packet the chain sees.
  Measured: a `dummy0` rule reported the same 3 packets the `lo` rule alone matched.
- `one-members-bad-input-fails-every-member.md` — a bare `interface "*"` leaves an
  empty name, `lowerIfaceMatch` refuses it, and `ApplyAll` batches every owner, so
  copp, ddos-local, flowspec-firewall and the operator's own firewall block all lose
  their reconcile over one policy-route typo.
- `gate-excludes-part-of-its-population.md` — the ddos stale-table sweep needs the
  plugin to start, so deleting `ddos local` and restarting under
  `flush-on-shutdown false` keeps the table. `ze_flowspec`, `ze_anomaly-shape` and
  `ze_copp` sit in the same position; the repair belongs to the firewall engine.
- `late-write-lands-on-the-successor.md` — two rows: OSPF's `InterfaceDown` starts a
  goroutine nothing joins (14 race detections at HEAD, 12 with the change), and the
  plugin server's `dispatch` snapshots handlers before invoking, which is the durable
  half of the ddos retirement fix.
- `balance-assertion-vacuous-without-a-loan.md` — `checkChildRekey` and
  `checkResponderRaisesChildRekey` both return nil on an empty baseline SPI set, so
  the rekey assertion passes vacuously in two scenarios that are green today.
- `component-rebuilt-during-reload.md` — `ddos flowspec` has the identical
  config-apply orphan the local half was fixed for, and copp has the defect it was
  cited as the cure for.

## 6. Working with the other sessions

`docs/features.md` was the one real collision. It carried an unlanded
`Per-protocol FIB withholding` row while my VRRP line was a false RFC 9568
conformance claim. Holding the file would have left the false claim standing, so
after asking, the row rode along disclosed by name in `fed8cb495`. That session has
since landed (`000e70eec`) and the row is no longer a forward reference.

The tree-wide red belongs to other sessions and varies by build flavor: an
`internal/core/bgp/asn` import neither committed nor vendored, `updateDelayEndOfRIB`
in the distro and appliance flavors, a changed `Insert` signature against the rib
test files. Name whichever your flavor hits in a `structural-red-ok` reason rather
than inheriting one from a report; a `go vet` under one tag set says nothing about
the flavors the gate compiles.

About 18 GB was freed: 33 pre-today session scratch directories and 103 stale
`bin/le-*` builds.

## 7. How to restart

1. `./le spec session release`, claim `spec-ospf-auto-cost-reference-bandwidth`, and
   run `/ze-close` on it. It is one audit away from done.
2. Claim the ddos spec, paste the round 5 section into its Review Gate, fix the
   BLOCKER and correct the exposure table, then run round 6 and close.
3. Claim the ipsec spec and RUN its three artifacts before touching anything else in
   it. Nothing about that spec is proven until they do.
4. Put the `cost` leaf question to Thomas when the OSPF spec closes, not later.

## 8. Update, 2026-09-09: the OSPF closure was started and stopped

`/ze-close` ran on `spec-ospf-auto-cost-reference-bandwidth` and was stopped
mid-phase, out of budget. It had passed the audit steps and was inside its own
Review Gate, fixing what that gate found: its last words were "the
deferred-origination method and the shutdown ordering".

**Uncommitted and INCOMPLETE, left in the working tree deliberately:**
`internal/plugins/ospf/instance.go`, `instance_test.go`, `virtual_link.go`. Do
not assume they compile and do not commit them as they stand. Read the diff
first and decide whether to finish the fix or drop it; `SetCost` and everything
`db3f18b0b` landed are intact in HEAD, so dropping it costs nothing already
proven.

Other sessions' files in that package, to leave alone: `bfd_client.go`,
`asn_notation_test.go`, `rfc5882_shared_key_test.go`.

**So the OSPF spec is NOT closed.** Its rounds 1 to 3 are clean and recorded, and
a fourth round was in progress and had found something. Restarting means
re-running `/ze-close` from step 5, not from the top: the deliverables,
documentation and security steps were done, and the round-4 finding is the only
open thread. The session claim is still on this spec.

Everything in sections 1 to 7 above is unchanged and still accurate.

---

# Update from session `inspired`, 2026-09-09, after the stop

**This file holds TWO sessions' handovers.** The VyOS-derived subscriber work is
sections 1 to 7 of the first document; the OSPF and spec-closing work is the
second, from "## 1. What landed" onward. They do not overlap. Commit `8760c0623`
carried both because the file was edited by both before it landed, which is worth
knowing if the attribution in that commit body reads thin.

## What changed after the first document was written

Everything that stood on its own is now COMMITTED. The first document says the
fourth spec is uncommitted work; that is no longer true.

| Commit | Contents |
|---|---|
| `300a7541a` | `internal/core/pacer`, all four receiver loops paced, logged and counted, the client's discovery wait, docs, tests, three journal rows, the spec |
| `8760c0623` | The two per-subscriber-usage specs, and this file |

**Phase 5 is DONE, not owed.** The first document's "Owed, phases 5 and 6" section
is stale on both its items. The killed agent had already finished them:

- Item 1 answered the judgement call rather than following the spec blindly.
  `SO_RCVTIMEO` already blocks the client's discovery read for around 100ms, so
  `runtime.Gosched()` was a yield sitting on top of a wait that existed. It removed
  the yield, added `readDiscoveryFrame` as a package variable so a test can swap the
  read, and documented the reasoning in `docs/architecture/l2tp/cpe-1-pppoe-client.md`.
- Item 2 exists: `TestReadLoopAllocationsPerPacketUnchanged` in `listener_test.go`,
  and `TestWaitForPADOBlocksRatherThanSpins` in `dialer_test.go`. Both pass.

## The only outstanding work on that spec

`plan/immediate/spec-subscriber-reader-loops-retry-a-failing-socket-without-backoff.md`
is `in-progress`, Phase 5/6, and the session claim is still on it. **Release it
first: `./le spec session release`.**

1. **Phase 6, the functional scenario.** AC-1: a socket held in a persistently
   failing state leaves CPU low while the log line and the counter show the failure.
   The spec calls it `subscriber-reader-failing-socket`. `test/qemu/` does not exist,
   so pick the suite directory from what does, and add the name to `netnsSelections`
   in `internal/le/qemu/netns_linux.go`. That list is explicit: `validateNetnsSelection`
   refuses a named test with no file, but never notices a file nobody named, which is
   how two `netns-link` tests under `test/plugin/` came to run nowhere at all.
2. **Then `/ze-close`**, on Opus 5 in a context that did not write the code.
3. The Goal Validation table must record that three `ppp` tests are written and
   cross-compile-verified but **never executed**: they are `//go:build linux` and this
   host is darwin. Do not mark them green.

## The two specs to run next

Both are in HEAD at `8760c0623`, both `design`, and Thomas approved the split and
all four surfaces. Take them in order; the second depends on the first.

1. `plan/immediate/spec-pppoe-subscribers-produce-no-accounting-or-telemetry.md`.
   The collection already exists and works for L2TP; two consumers simply never moved
   onto the shared subscriber layer, so a PPPoE subscriber produces no accounting
   record and no metric. Also repairs a fail-open where a failed stats read sends
   `Acct-Input-Octets = 0`, indistinguishable from an idle subscriber.
2. `plan/immediate/spec-subscriber-utilisation-has-no-operator-view.md`. The views.

## Still waiting on Thomas, unchanged

The four decisions in section 4 of the first document all stand. The RFC index one
is the one that compounds: `docs/features/rfc-status.md` is now three specs stale on
its PPPoE and IPv6CP rows, and the session that could unblock it is gone.

One correction to that section: the resurrected
`plan/immediate/spec-ipv6cp-accepts-and-proposes-a-zero-interface-identifier.md` is
STILL on disk, untracked, still reading `in-progress` Phase 5/5, for a spec closed at
`3fd87369fb`. It makes the backlog show a closed spec as open. Its content is
preserved in `fd7cd7b44e`, so deleting it loses nothing, but it needs Thomas's word
(`ai/rules/never-destroy-work.md`).

## What this session would tell the next one

**Commit each finished chunk when it finishes.** Three specs closed cleanly, then the
fourth's completed phases sat uncommitted while the session chased the next phase and
ran out of budget. Thomas had to ask whether the work was committed. `ai/rules/git-safety.md`
already says this: the question after each chunk is whether it stands on its own, not
whether the session is finished.

**The Review Gate is where the defects were.** Each of the three closures found a
product defect that every implementation phase had missed: a counter that reached no
operator because the registry is created after the component starts, a `Session.State`
data race from locking two of three accessors, and a fabricated RFC quotation. Budget
closure as defect-finding.

**Read the other implementations before designing protocol behaviour.** Reading
accel-ppp's, FreeBSD's and pppd's own source changed two designs before they were
built: the IPv6CP missing-option case would have Naked forever without pppd's one-shot
guard, and the PADR dedup would have broken legitimate multi-session CPE without
accel-ppp's cookie key. It also caught a false comment in Ze claiming its MAC-keyed
dedup matched accel-ppp's, which is cookie-keyed.

**The IDE diagnostics in this harness are stale about seven times out of seven.**
Verify with the registered action before acting on one.

---

# spec-forwarded-as-path-obeys-rfc6793-for-every-destination (2026-09-09)

## State

All 8 implementation phases are DONE and COMMITTED. The spec is still
`in-progress` in `plan/immediate/` and the session claim is still on it. Nothing
in the product is half-built; what is left is closure bookkeeping.

Why it is not closed: `./le commit create` refuses a closure commit without a
review artifact recorded `clean`, and the independent review was still running
when the session ran out of budget.

**The stop hook WILL block on this spec, and its ack file does not travel.** It
lived at `tmp/session/.closure-ack-...`, which is machine-local and uncommitted,
so it is absent wherever this resumes. Re-create it in one line rather than
hunting for the reason:

```
echo 'Closure blocked on the review artifact: le commit create refuses a closure commit without one recorded clean, and the review had not finished. Every phase is implemented and committed. See continue.md.' \
  > "tmp/session/.closure-ack-forwarded-as-path-obeys-rfc6793-for-every-destination"
```

Or simply do step 2 below, which removes the need for it.

## What the change did

Ze reconciles the AS_PATH/AS4_PATH pair ONCE, at ingest, stores four-octet AS
numbers only, and regenerates the two-octet form at encode time. This is the
shape Thomas decided on 2026-09-08 after FRR and BIRD were read: both normalise
at ingest, which is why neither needs an RFC 6793 Section 4.2.3 step on its
forward path.

- `attribute.ReconcileASPathFamily` (`internal/core/bgp/attribute/as4.go`) holds
  the rule, moved out of `rib/storage` because the reactor may not import a plugin.
- `wireu.CollapseAS4Family` (`internal/component/bgp/wireu/aspath_collapse.go`)
  applies it to the payload. Fast path answers 0 and the caller keeps its slice.
- `(*Session).collapseASPathFamily` (`internal/component/bgp/reactor/session_read.go`)
  calls it after `enforceRFC7606`, before the import policy chain, and relabels
  the encoding context.
- Four sites take the received width from the encoding context rather than the
  negotiated capability, including `resolveRelaySource` (`reactor_api_relay.go`).
- Interop scenario `test/interop/scenarios/as-path-mixed-width-relay-frr` PASSES,
  with a discrimination walk recorded on distinct images.
- RFC6793-4.1-6, -4.1-7 and -6-5 closed. Only -4.1-3 remains.

## TO DO, in order

1. **Commit the learned summary.** `plan/learned/016-normalise-once-at-the-edge.md`
   is written and UNCOMMITTED. It is the only uncommitted file of this work.
2. **Run the independent review** (`/ze-review` in a subagent, Opus, over the
   range `c442cb5c8..HEAD`). Spend the effort on whether any row in the spec's
   closure tables is FALSE against the tree: the sibling spec's last three rounds
   each found a false record rather than a code defect.
3. **Record it**: `./le spec session review record spec spec-forwarded-as-path-obeys-rfc6793-for-every-destination verdict clean rounds <n> reviewers <...> file <each reviewed file>`.
   The gate refuses `findings`; fix what it finds, then re-record.
4. **Append `plan/TEMPLATE-CLOSURE.md`** and complete every section. Goal
   Validation must carry the interop evidence, which exists.
5. **Two commits**: A = spec + learned summary; B = `remove` the spec file.
6. **Release the claim**: `./le spec session release`.

## Open findings, recorded not fixed. Thomas's to schedule

- `ASPathEdit.recordAggregator` (`internal/component/bgp/wireu/aspath_slot.go`)
  does NOT tombstone an AGGREGATOR whose length it cannot read. `RewriteASPath`
  did, so the behaviour vanished when `Record` replaced the rewrite, with no red
  test. Row in `plan/journal/unwired-feature.md`. The transcode rail still
  tombstones and keeps its own test.
- `test/interop/scenarios/bgp-aggregator-as4-downgrade-bird` rests on the claim
  that `internal rib` alone relays a route to a peer. Measured false twice while
  building the new scenario: the relay is `bgp-rs` plus `bgp-adj-rib-in`. Row in
  `plan/journal/test-against-broken-path.md`.
- No originating encoder emits an AS4_PATH
  (`internal/component/bgp/message/update_build*.go`), so a route ze ORIGINATES
  with a non-mappable AS in its configured AS_PATH reaches a two-octet peer as
  AS_TRANS with the real number nowhere. RFC 6793 Section 4.2.2 requires it. Row
  in `plan/journal/requirement-met-on-the-rails-the-spec-planned.md`.

## Decisions already taken, do not re-litigate

- A lone AS4_AGGREGATOR (no AGGREGATOR beside it) is DROPPED, the UPDATE
  continues, and the drop is reported. The RFC does not cover the shape; FRR
  keeps and fabricates, BIRD drops, ze takes BIRD's answer. Thomas, 2026-09-09.
  Documented in `as4.go`, `docs/architecture/edge-cases/as4.md` and
  `rfc/short/rfc6793.md`.
- Tombstone draft-mangin Section 5.3 support is REMOVED, with `aspath_rewrite.go`.
  `WriteTombstone` stays: the transcoder calls it. Thomas, 2026-09-09.
- Attribution in new records is "(Thomas, YYYY-MM-DD)", not "owner ruling".

## Environment note

colima was resized from 2 CPUs / 1.9 GiB to 8 / 24 on 2026-09-08. That is what
lets the interop lab reach a verdict at all; at the old size the fleet is killed
before any assertion runs, and `docker info` reports the VM's allocation rather
than the host's.

# asdot/asdot+, update-delay, BFD strict mode (session `bgp-notation`, 2026-09-08/09)

Three specs were run concurrently in one working tree. That bought parallelism
and presented the bill at commit time: the three features interleave in the same
files, so they must land in a fixed order. Stopped on request before that order
completed.

## State

| Spec | Committed | Status |
|------|-----------|--------|
| `plan/spec-bgp-as-notation.md` | `c3b6433ba8`, 140 files | `in-progress`. Commit A only, commit B not run |
| `plan/spec-bgp-bfd-strict.md` | nothing | `in-progress`. Three round-6 issues open |
| `plan/immediate/spec-bgp-update-delay.md` | nothing | `in-progress`. Reviewed clean, closure done, commit blocked |

`c3b6433ba8` is the only commit of this session at the time this block was
written. See the update at the end for what landed after it.

**Why as-notation stayed in `plan/`.** Commit A landed and commit B, the spec
removal, was deliberately not run. Three product files the feature needs could
not be committed, because each also carried BFD-strict or update-delay hunks
naming symbols HEAD did not hold:
`internal/component/bgp/config/loader_create.go` (the `applyASNotation` call on
first load), `internal/component/bgp/reactor/reactor_api.go` (`recordASNotation`
at `SetConfigTree`), and `internal/component/bgp/plugins/cmd/peer/peer.go`
(`asn.Of` on both peer row builders). `plugin.ReactorIntrospector` had also
gained `UpdateDelayStatus()`, so carrying them meant landing both sibling
features whole, under a review artifact covering neither. The spec is the only
record that those three files are owed, which is why it was not deleted.
(A machine-local ack was written to `tmp/session/`, which is gitignored and will
not survive; its content is the paragraph above.)

## TO DO, in order. The order is forced, do not reshuffle it

1. **BFD strict, three fixes.** All three are verified at the producer and listed
   under "Open findings" below. They are not optional: two are in the
   always-in-scope RFC class and one is a regression this spec caused.
2. **BFD strict, round 7.** Thomas must authorise it. Round 6 was the sixth and
   he authorised each round past five separately. `./le spec session review record`
   refuses a seventh without `--owner-authorised`, which MUST NOT be set unasked.
3. **BFD strict, closure and commit.** Carry
   `internal/component/bgp/reactor/reactor_api.go` and
   `internal/component/bgp/plugins/cmd/peer/peer.go` with it. `asn.Of` and
   `applyASNotation` are in HEAD now, so the as-notation hunks in those files are
   safe to land (`ai/rules/git-safety.md`: the presumption is LANDING, and the
   unsafe case is a hunk naming a symbol HEAD does not hold).
4. **update-delay, commit.** Nothing is left to build or review. Carry
   `internal/component/bgp/config/loader_create.go`. Its review artifact lived
   under `tmp/review/`, which is gitignored: see the update at the end, because
   losing it changes what this step costs.
5. **as-notation, commit B.** Once the three files above are in HEAD, the four red
   tests go green and removing `plan/spec-bgp-as-notation.md` closes the spec.

The four tests knowingly RED until step 5, each named in `c3b6433ba8`:
`TestPeerRowsFollowTheConfiguredNotation`, `TestHandlerPeerDetailAllPeers`,
`TestTheRunningConfigRecordsTheNotation`, `test/plugin/bgp-as-notation.ci`.

## Open findings on BFD strict, from round 6. Verified, unfixed

- **A regression this spec introduced.** `pluginService.EnsureSession`
  (`internal/component/bfd/bfd.go`) now takes the `SO_BINDTODEVICE` device from
  `normalized.Interface` rather than `req.Interface`. `runtimeState.loopFor`
  returns an existing loop and drops the device argument, so the FIRST caller
  locks in the device and every later single-hop session in that VRF transmits
  from it. A BGP peer with only `connection local ip` can now do what only an
  operator-written `bfd interface` leaf could do before. Minimal fix: key on
  `normalized`, pass `req.Interface` to device selection.
- **RFC 5882 Section 4.4 is met for single-hop only, and the ledger over-claims
  it.** `SessionRequest.Canonical`
  (`internal/component/bfd/api/session_identity.go`) returns every multi-hop
  request unchanged. `parseMultiHopSession` requires `local`, while
  `bfdRequestFor` leaves `Local` invalid when the peer sets no
  `connection local ip`. Two clients for one remote system then build two keys.
  `rfc/short/rfc5882.md` records the requirement met, and all five tests in
  `rfc/requirements/rfc5882.md` are single-hop. `ai/rules/rfc-compliance.md`
  forbids repairing this by downgrading the row: implement the multi-hop half and
  prove it.
- **The link table is VRF-blind.** `api.Link` carries no VRF, `connectedLinks`
  (`internal/component/bfd/session_identity.go`) returns every interface, and
  `deriveLink` never reads `r.VRF`. A request in one VRF can be given another
  VRF's link name and address. Bounded today because only pinned sessions carry a
  VRF, and `iface.InterfaceInfo` has no VRF field to filter on, so the repair
  reaches into the interface component.

## Decisions Thomas took. Do not re-litigate

- BFD strict mode implements `draft-ietf-idr-bgp-bfd-strict-mode-19` in full,
  capability 74 and the FSM changes, rather than a local-only policy (2026-09-08).
- The RFC 5882 Section 4.4 gap is fixed by making the two request builders agree
  on the key. Not by refusing the overlapping configuration, and not by deferring
  it to another spec (2026-09-09).
- update-delay AC-2 is AMENDED, not implemented: the feature defers the initial
  routing update, not best-path selection. Best-path still runs during the hold,
  so the CPU saving the original wording implied is not delivered (2026-09-09).
- as-notation review rounds 6, 7 and 8 were each authorised individually, and the
  spec closes with no ninth round (2026-09-09). Round 7 was the exhaustive
  parser-driven sweep, round 8 added the permanent gate.

## Recorded, not fixed

- `plan/journal/absent-value-is-a-distinct-identity.md`: a pinned `bfd session`
  and a strict peer to one neighbor build different keys when only one of them
  names a local address. Distinct from the Section 4.4 finding above.
- `plan/journal/silent-fall-through.md`: `zeTestParseRunCLI`
  (`internal/test/cli/cmd_bgp.go`) prints usage on an unknown verb and returns
  nil, so a suite run with a wrong verb exits 0 having run nothing. The one-line
  repair moves an exit code every runner caller reads.
- `plan/journal/lint-contract-not-applied.md`, `plan/journal/unwired-feature.md`
  (the BGP notification block has no emitter), and
  `plan/journal/comment-describes-superseded-behaviour.md`.
- Two verification-debt rows against `c3b6433ba8` in
  `plan/verification-debt/fcd89c36.md`.

## What this session would tell the next one

Every defect worth finding was found by breaking the code on purpose and watching
what did NOT go red. Reading a test never found one. In the order they were met:
a handler tested by calling the handler; an FSM error tested by driving the FSM;
a timer whose arming and whose firing were each tested while their connection was
not; a `show` command registered in an `-api` module so no CLI path reached it,
with its registration test written in a package that could not see the module; a
golden file pinning `neighbor_as: 0` as correct; an interop scenario whose two
sides built different session keys, so it passed with the feature removed; and a
discrimination run that broke the wrong one of two textually identical guard
lines and reported the wrong conclusion.

The new gate is `checkNumberParseSites` (`internal/le/repository/numberparse.go`).
It refuses a new 32-bit text-to-integer parse in product Go that
`numberparse-allowlist.txt` does not record. It states four blind spots at the
producer, and the one that matters is `validateUint`
(`internal/component/command/argvalidate.go`): it is the YANG-typed pre-handler
validator, so a command input leaf typed `zt:asn` would refuse a dotted AS number
before any handler reached `asn.Parse`. Every AS command input leaf is
`zt:asn-notated` today, and that is the only thing keeping it dormant.

## Update, 2026-09-09, after two commits landed

The section above was written when only as-notation had committed. Two things
changed since, so read this before acting on the TO DO list above.

### What is now in HEAD

| SHA | What |
|-----|------|
| `c3b6433ba8` | as-notation, 140 files. `asn.Of` and `applyASNotation` are in HEAD |
| `a8f7323f64` | BFD strict mode, 51 files, the self-contained half only |

`a8f7323f64` is NOT a closure commit. `plan/spec-bgp-bfd-strict.md` stays
`in-progress` and its three round-6 issues stay open. They are listed in the
commit body verbatim, so `git show a8f7323f64` is the authority rather than this
file.

### What BFD held back, and why it is the crux

BFD landed its own component, the capability, the FSM, the doctor check, the
OSPF and static subscribers, the lab and the scenarios. It held back the whole
REACTOR half, eight files, because each one reaches into another spec:

- `session.go` carries paths-limit fields whose types live in an UNTRACKED
  `session_paths_limit.go`, which needs `nlrisplit` symbols HEAD does not hold.
  That belongs to a fourth session, not to any of these three specs.
- `peer_run.go` calls `updateDelayPeerDown`.
- `internal/component/plugin/types_bgp.go` widens `ReactorIntrospector` with
  `UpdateDelayStatus`, which would break five `mock_reactor_test.go` files
  without `reactor/update_delay.go`.
- `reactor_api.go`, `peer_bfd.go`, `session_bfd_strict.go`, `session_handlers.go`,
  `session_connection.go`, `peer_settings.go`, `config.go` and `cmd/peer/*` sit
  behind the same three facts.

Carrying them would have landed three specs under a review artifact covering
one. That was the right refusal and it should not be reversed by a later session
in a hurry.

### TO DO, replacing the list above

1. **update-delay commit. NOT DONE.** It was started and stopped on request
   before it committed; it left nothing staged and nothing half-written, and HEAD
   was unchanged. All of its work is still only in the working tree, and it is
   the most exposed thing in this checkout: finished, reviewed, and undefended
   against another session's overwrite.
   It owns `types_bgp.go` and `updateDelayPeerDown`, so its commit is what puts
   `UpdateDelayStatus` into HEAD and unblocks everything below. Two commits from
   one script: code plus spec plus journal row, then the spec removal.

   **Its review artifact will not survive.** It was written under `tmp/review/`,
   which `.gitignore` line 12 excludes as `tmp/*`. `internal/le/commit` refuses a
   closure commit without a clean, hash-pinned artifact covering every reviewable
   file, so a session that cannot find it MUST run a fresh review pass rather
   than reach for `review-override`, which records verification debt and is an
   explicit owner decision.

   What the lost artifact recorded, so the fresh pass knows what was already
   done: verdict CLEAN over 37 files, four review rounds, the last two returning
   0 BLOCKER and 0 ISSUE. Round 1 found the numerator and denominator counting
   different peer sets; round 2 found the `show bgp update-delay` command
   unreachable because it was declared in an `-api` YANG module; round 3 was
   clean; round 4 confirmed the fixes. Closure then found one more blocker four
   rounds had missed: the command declared no answer shape, so it published every
   pipe operator over an answer holding no rows. All are fixed in the working
   tree.
2. **The paths-limit dependency.** `session_paths_limit.go` is untracked and its
   `nlrisplit` symbols are not in HEAD. Until that session lands them,
   `session.go` cannot be committed by anybody, and the BFD reactor half is stuck
   behind it. This is the single biggest blocker in the tree and it belongs to
   neither of the two specs still open here.
3. **BFD reactor half.** Once 1 and 2 are in HEAD, commit the eight held files.
   Then fix the three round-6 issues, then round 7, which needs Thomas's
   authorisation, then closure.
4. **as-notation commit B.** Needs `reactor_api.go` and `cmd/peer/peer.go` in
   HEAD, which is step 3. Then the four red tests go green and the spec is
   removed. Why it stayed open is written out under "State" above, not in the
   gitignored ack file.

   Its own review artifact is gitignored too and will be gone. It recorded a
   clean verdict over 124 code files across EIGHT rounds, six of which Thomas
   authorised individually past the five-round cap. `owner-authorised` was
   passed because the tool refuses more than five rounds without it, and the
   text quoted only his real authorisations: rounds 6, 7 and 8 on 2026-09-09,
   and his decision to close with no ninth. A fresh pass covering commit B's
   files is what a later session owes if the artifact is missing.

### Two things not to disturb

- `plan/journal/green-that-could-not-have-been-red.md` and
  `plan/learned/011-a-forced-red-proves-only-what-ran.md` are STAGED in the
  shared index and belong to another session. Leave them staged. Do not carry
  them into a commit and do not unstage them.
- BFD's commit repaired a read-modify-write race in
  `internal/le/interoplab/bgp/checkers.go`: its working copy had lost the
  paths-limit session's `bgp-paths-limit-frr` scenario entry, restored from HEAD
  before committing. If that file looks wrong again, suspect the same race rather
  than a deliberate deletion.

# spec-config-apply-ordering-covers-every-root (2026-09-08/09)

## State

Four commits are in HEAD. The spec is `in-progress` in `plan/immediate/` and is
claimed by session `cbc36cee`. Every assumption A-1 to A-7 is closed, and the
Integration and Documentation checklists are answered with evidence.

| Commit | What it made true |
|--------|-------------------|
| `a6ea1ad0b` | Every participant with a diff is a node in the operation graph. The section-apply fallback and its `Info` line are deleted |
| `6ffcdaf25` | The plugin ABI holds no operation label. Ordering reads a verb (`create`, `destroy`, `modify`) and the target's `ResourceKind`. A verbless operation aborts |
| `e3ab3dd1d` | Edges derive from declared `produces` and `consumes`. The nine hand-written produce/consume rules are deleted |
| `bbe42da682` | A rolled-back peer returns as the reactor had it. AC-7 is proven. The address-swap and mixed-root tests land written and UNPROVEN |

## What the change found, and it is the reason the spec existed

Three separate pieces of this subsystem were not running, and each was invisible
because the thing that would have reported it was the thing that was off.

1. **The ordered path had never run.** `TxCoordinator.Execute` took it only when
   operations existed AND no participant was uncovered. Twenty-two BGP plugins
   declare `WantsConfig: bgp` and not one registers a decomposer, so every bgp
   reload was uncovered and fell to the unordered section apply, reporting
   success.
2. **`modify-peer` on that path was a remove followed by an add.** Turning
   coverage on would have bounced every BGP session on every reload. It now asks
   `peerSettingsSwapPlan`, the decision the section apply already took.
3. **Two of the nine rules did not do what their names said.**
   `iface-remove-address-before-interface` never fired once: its relation
   compared `opIfaceName` of the address operation against `opAddrIface` of the
   interface operation, and an interface operation carries its name in
   `Target.Name`, which `opAddrIface` does not read, so the right side was empty
   for every pair it ever saw. `bgp-add-address-before-peer` selected the
   `add-peer` label, so a `modify-peer` rebinding the same address got no edge.
   The derivation produces both edges and loses none of the others.

## TO DO, in order. The first item is the only real work

1. **The QEMU discrimination walk for two tests.**
   `test/reload/config-apply-ordering-address-swap.ci` (AC-4, and it is what
   closes D4) and `test/reload/config-apply-ordering-mixed-root.ci` (AC-1) are
   written, committed, and prove nothing yet. Both carry
   `option=needs-linux:caps=net-admin` and skip on darwin.
   `./le qemu netns-test` does NOT cover the reload suite: its selector is
   `firewall,policy,ospf,ospfv3,pppoe` (`internal/le/qemu/actions.go`). The route
   is `./le qemu run` with the reload suite as its command, per
   `docs/architecture/testing/qemu-integration.md`. The reverts are named in the
   spec: for `address-swap`, make `tryRelaxCycle` return the unreduced edge set;
   for `mixed-root`, reinstate the uncovered-participant condition that
   `a6ea1ad0b` deleted. Both halves of a pair MUST come from one tree.
2. **`./le verify worktree` green on a committed tree.** A Goal Gate. Three
   verification-debt rows are open under `plan/verification-debt/4c26aef3.md`.
3. **The Review Gate**, `/ze-review` looped to zero BLOCKER and zero ISSUE,
   recorded through `./le spec session review record`. It must be independent of
   the agents that wrote the code.
4. **Learned summary, then the two-commit closure.** Commit A carries code, spec
   and the learned file; commit B is `git rm` of the spec alone.

## Open findings, recorded not fixed. Thomas's to schedule

- **`Params.OldConfig` (`pkg/plugin/rpc/types.go`) has no first-party reader.**
  The remove path reads the running peer instead, so the BGP decomposer still
  fills the field and nothing consumes it. Deleting it also deletes two
  assertions in `internal/component/bgp/plugin/operation_test.go`. Left in place
  for the owner to route.
- **`spec-peer-deactivate-and-bulk-route-purge` states that `OperationRemovePeer`
  lives in `pkg/plugin/rpc/types.go`.** Commit `6ffcdaf25` made that false. It is
  another spec's row to correct, not this one's.
- **A plugin startup race**, journalled in
  `plan/journal/plugin-startup-barrier-deadlock.md`. A peer dials while the
  plugins are still in stage 4, so the engine issues `validate-open` on a
  connection whose coordinator waits for `share-registry`, and every internal BGP
  plugin dies. It reproduced in 3 of 5 `./le functional reload` runs on
  2026-09-08, two of them on an idle machine. The row's earlier claim that
  concurrent load opened the window is corrected in place. It is NOT this
  change: the failure is at daemon startup, before any config transaction runs.
- **`config-apply-ordering-rotation` timed out once in five runs** on the phase-1
  tree, at 30.0s against a 2.2s average, reporting "all expected messages
  received but test still timed out". Not reproduced in three later runs. Its
  note lives in the spec's phase 4 step.
- **No `./le test-unit` area covers `internal/component/iface`.** Its tests were
  run through `./le job run label iface-unit command go test ./internal/component/iface/`.

## Decisions already taken, do not re-litigate

- **The coarse node is applied through the existing `config-apply` RPC**, not
  through `config-operation-apply`. The five `config-operation-*` callbacks have
  no SDK default (`initCallbackDefaults`, `pkg/plugin/sdk/sdk_callbacks.go`), so a
  plugin that registered none answers "unknown method" and the transaction aborts.
- **The coarse node owes no per-operation verify.** Phase 1 verify already covered
  its participant's whole candidate config.
- **A payload with no verb is refused, never defaulted to `modify`.** Ze is
  pre-release with no shipped external plugin, so no compatibility shim exists.
- **The BGP labels live in `internal/core/bgp/configop`, not in
  `internal/component/bgp/plugin` as the spec's letter said.** The reactor applies
  those operations and cannot import the plugin without pulling a plugin's
  registration into the engine, and the plugin cannot import the reactor because
  `events_import_test.go` is `package reactor` and blank-imports it.
  `internal/core/bgp/events` is the precedent. The reason is written into the
  spec's Files to Modify as a `→ Decision:`.
- **A rolled-back peer's settings come from the RUNNING peer**
  (`runningPeerSettings`, `internal/component/bgp/reactor/operation.go`), not from
  a parse of the operation's config subtree. The subtree parse dropped eleven
  kinds of state, not only `StaticRoutes`: the peer's own `Name` and `GroupName`,
  `PluginRoutes`, both filter chains with their canonicalization and prepended
  defaults, the loop-detection policy, the cluster id, redistribute process
  bindings, the test port override, group and bgp inheritance, YANG defaults with
  no Go constant, and inactive-node pruning.
- **Thomas declined a worktree for the walk** (2026-09-08) and chose to wait for
  this checkout. Do not create one without asking again.

## Environment note

This checkout was rebuilt under three agents on the night of 2026-09-08:
`internal/component/bgp/reactor`, `internal/component/plugin/coordinator.go` and
`internal/component/sysrib/sysrib.go` each refused to compile in turn, from other
sessions' in-flight work. It healed later that night and the whole reload suite
went 44 of 44. A red and a green taken across two different trees are not a pair,
so nothing was recorded from the runs that straddled a rebuild. Expect to check
that the tree builds before starting item 1.

### Addendum: BFD work that `a8f7323f64` did not take

Written after that commit. Verify each line against `git ls-tree -r HEAD` and
`git status` before acting: a sweep to land these was started and its outcome is
not recorded here.

**A scenario is referenced by HEAD but is not in it.**
`test/interop/scenarios/bgp-bfd-strict-speaker/` (its `speaker-args` and
`ze.conf`) was untracked when this was written, while the checker entry naming it
DID land. The sibling scenarios `bgp-bfd-strict-frr`, `-preup-speaker` and
`-reload-speaker` are all in HEAD. The missing one is the positive interop half:
it proves Ze withholds its KEEPALIVE until BFD is Up against a peer that
negotiates capability 74. Until it lands, that proof exists nowhere in history.

**Also uncommitted at the time of writing:** `rfc/requirements/rfc5882.md`, which
holds the four tagged units for Section 4.4; `internal/core/bgp/capability/capability_test.go`
and `negotiated_test.go`; and `internal/plugins/ospf/instance.go`,
`instance_test.go` and `virtual_link.go`, whose ownership was not established.

**The four generated RFC index files need judgement, not a commit.**
`rfc/enrolled.txt`, `rfc/not-enrolled.txt`, `ai/RFC-REQUIREMENTS.md` and
`docs/features/rfc-status.md` have been held back across several closures, for
the reason recorded earlier in this file: `./le rfc index-update` is whole-tree,
and running it would publish a `Supported` row for
`draft-ietf-idr-bgp-bfd-strict-mode` whose producer was not in HEAD. Part of that
producer IS in HEAD as of `a8f7323f64`, but the reactor half is not, so the row
would still over-claim. `ai/rules/rfc-compliance.md` forbids publishing a
conformance claim the tree cannot support, and forbids repairing an over-claim by
lowering the row instead of proving it. Land these only when the reactor half is
in HEAD.

**How this was found, because it generalises.** The committing agent's report
listed three scenario directories as landed and the commit contained three, but a
fourth existed on disk and in a committed checker. The report was accurate about
what it did and silent about what it left. Check `git ls-tree -r HEAD` against the
working tree after any large scoped commit in a shared checkout, rather than
reading the commit's own summary.

**Outcome of that sweep: `ecb3bfc1b7`**, "bfd: the positive interop scenario and
the Section 4.4 ledger row", 5 files. The speaker scenario and
`rfc/requirements/rfc5882.md` are now in HEAD, so the tree no longer names a
scenario directory it does not contain.

Two things that sweep deliberately left, and they are still open:

- `internal/core/bgp/capability/capability_test.go` is dirty and the commit gate
  reports it REMOVES `TestParsePathsLimitEmpty`, `TestParsePathsLimitShortRead`
  and `TestPathsLimitLen`. Those belong to the paths-limit session. Only its
  author knows whether something replaced them, and `ai/rules/testing.md` wants a
  `test/weakened/` row for a removal, which nobody else can write honestly. Ask
  that session before carrying this file.
- `rfc/enrolled.txt` and `rfc/not-enrolled.txt` stay held. The
  `draft-ietf-idr-bgp-bfd-strict-mode` row cites `session_bfd_strict.go` and
  `parsePeerFromTree`, neither in HEAD.

One judgement in that commit worth knowing, because it is the kind a later reader
might mistake for sloppiness: `rfc/requirements/rfc5882.md` names
`config_bfd_strict_test.go`, which is held back with the reactor half. It was
landed anyway because `rfc/discrimination/rfc5882.json` in HEAD already cites the
same test, so the two artifacts now agree rather than leaving three HEAD-resident
tags with no ledger row. Both references resolve when the reactor half lands.

## Final state of this session, 2026-09-09

Three commits landed. A large part of the work did not, and this is the exact
inventory. Verify it with `git status` before acting; other sessions write here.

| Landed | What |
|---|---|
| `c3b6433ba8` | as-notation, 140 files. Commit A only, the spec was not removed |
| `a8f7323f64` | BFD strict mode, 51 files, the self-contained half |
| `ecb3bfc1b7` | BFD sweep: the positive interop scenario and the Section 4.4 ledger row |

| NOT committed | Files |
|---|---|
| update-delay, all of it | `reactor/update_delay.go` and its test, `config/update_delay.go`, `plugin/types_bgp.go`, `test/plugin/bgp-update-delay.ci`, `test/plugin/bgp-update-delay-validation.ci`, `test/ui/bgp-update-delay-command.ci`, `test/interop/scenarios/bgp-update-delay-frr/` |
| BFD reactor half | `reactor/session_bfd_strict.go` (untracked), `peer_bfd.go`, `peer_run.go`, `peer_settings.go`, `session.go`, `session_handlers.go`, `session_connection.go`, `config.go`, `cmd/peer/*` |
| as-notation's three owed files | `config/loader_create.go`, `reactor/reactor_api.go`, `plugins/cmd/peer/peer.go` |

update-delay is the exposed one: finished, reviewed clean over four rounds,
closure done, and none of it is in history. It stopped one step short of
committing.

### The order, and why each step is where it is

1. **Commit update-delay.** It owns `plugin/types_bgp.go` and
   `updateDelayPeerDown`, so it is what puts `UpdateDelayStatus` into HEAD.
   Nothing blocks it today. Run `./le spec session review check` first; if the
   artifact under `tmp/review/` is gone (that path is gitignored), a fresh review
   pass is owed and `review-override` is NOT the answer, it records verification
   debt and is an owner decision.
2. **Wait for `session_paths_limit.go`.** It is untracked, belongs to a fourth
   session, and its `nlrisplit` symbols are not in HEAD. `reactor/session.go`
   cannot be committed by anybody until it lands. This blocks step 3 and belongs
   to neither spec open here.
3. **Commit the BFD reactor half**, once 1 and 2 are in HEAD.
4. **Fix BFD's three round-6 issues**, then round 7, which needs Thomas's
   authorisation, then closure. The three are listed earlier in this block: the
   `SO_BINDTODEVICE` regression in `pluginService.EnsureSession`, multi-hop still
   outside RFC 5882 Section 4.4 while the ledger records it met, and the VRF-blind
   link table.
5. **as-notation commit B.** Needs `reactor_api.go` and `cmd/peer/peer.go` in
   HEAD, which is step 3. Then the four red tests go green and the spec leaves
   `plan/`.

### Still open, not blocking

- `internal/core/bgp/capability/capability_test.go` removes three paths-limit
  tests. Ask that session; a removal owes a `test/weakened/` row only its author
  can write.
- `rfc/enrolled.txt` and `rfc/not-enrolled.txt` stay held until the BFD reactor
  half is in HEAD, or they publish conformance for absent code.
- Two verification-debt rows against `c3b6433ba8` in
  `plan/verification-debt/fcd89c36.md`.
- Nothing was pushed. The repository carries verification debt that refuses a
  push, and no owner authorisation for one was given.

## 9. The work left, as one list

Ordered. Each item says what "done" means, so nobody has to reconstruct it.

1. **Decide the three uncommitted OSPF files.** `internal/plugins/ospf/instance.go`,
   `instance_test.go`, `virtual_link.go`. Measured 2026-09-09: `go vet
   ./internal/plugins/ospf/` exits 0, so they compile and block nobody. They are
   the killed closure agent's fix for a round-4 Review Gate finding, and the
   finding itself was never written down. Read the diff, decide whether it is a
   real fix worth finishing with its red observed, or drop it. Do not commit it
   as it stands: a change with no recorded red and no round closing it is what
   the rest of this sweep exists to avoid.
2. **Close `spec-ospf-auto-cost-reference-bandwidth`.** Re-enter `/ze-close` at
   step 5, its Review Gate, not at the top: deliverables, security and
   documentation are done and rounds 1 to 3 are clean and recorded. Item 1 is
   part of this. The session claim is still on the spec.
3. **Fix the ddos round-5 BLOCKER and close that spec.** `clearStaleDropRule`
   runs before the FIREWALL engine is configured, so `ApplyAll` autoloads nft and
   a `backend vpp` box has the wrong table cleared while the VPP drop survives.
   Fix shape: move the call to the head of ddos-local's `OnConfigure`, verified
   at `runPluginPhase` first. Paste
   `tmp/session/2026-09-08-0a21e591-.../scratch/round5-review-gate.md` into the
   spec's Review Gate; it is not there yet. Correct the guide's exposure table
   with it. Then round 6, then close.
4. **Run the three IPsec artifacts, then close that spec.** The two `.ci` and the
   `dataplane-readback` strongSwan scenario from `03a77aa12` have never executed.
   The build blocker has cleared. Vacuity walk each, then close.
5. **Put the `cost` leaf question to Thomas.** He ruled on the router-wide leaf
   only, so ze gives two answers to what a cost change costs. Extending it is not
   a one-liner: `reconcile`'s re-pricing arm never writes `e.running`, and
   `lsdbTopology` prices from that map.

Not in this list on purpose: every journal row this session wrote is committed and
is a record, not a task. A class file earns its fix in a deliberate pass over the
journal, never by whoever trips over it next.

## Update, 2026-09-11: the BFD reactor half is PARKED, and the three issues are fixed

Two changes since the block above. Read both before touching `internal/component/bgp/reactor`.

### The deadlock, and how it was broken

`reactor/session.go` was mutually entangled with the paths-limit session's work.
Git commits whole files, so that one file carried both halves: the paths-limit
session could not land its chunk without carrying our unreviewed BFD code, and we
could not land ours without carrying theirs. "Yours first, then mine" had no first
step.

Resolved by withdrawing our half from the package entirely. `session.go` in the
working tree now holds exactly one hunk, the paths-limit fields, so that session
can commit it under a review artifact that covers every line of it.

**Everything is preserved, nothing deleted:**

| Where | What |
|---|---|
| `backups/parked-bfd-strict-20260911-0100/` | `session_bfd_strict.go.parked`, `session_bfd_strict_test.go.parked`, `config_bfd_strict_test.go.parked`, plus `RESTORE.md` naming every piece and its destination |
| `backups/bfd-strict-reactor-20260911-0100.patch` | The tracked-file hunks: `session.go` (3 fields + the Section 10 log switch), `session_handlers.go` (4), `session_connection.go` (2), `peer_run.go` (4), `reactor_api.go` (2) |
| `backups/bfd-session-go-20260911-005714.patch` | The `session.go` pair alone, taken first |
| `backups/bfd-fsm-docs-20260911-0105.patch` | The `fsm-open-sent.md` and `fsm-open-confirm.md` hunks, whose six anchors named the parked file |

`peer_bfd.go`, `peer_bfd_test.go` and `session_handlers_test.go` were wholly ours
and are back at HEAD content. `peer_run.go` was reverted HUNK BY HUNK, not by
file: update-delay's `startInitialRoutes` and `updateDelayPeerDown` are still in
it, verified present after the sweep.

**The park uses a `.parked` suffix deliberately.** The first attempt left them as
`.go` files under `backups/`, and `./le verify lint run` typechecked them and
reported `undefined: Session`. Nothing compiles a `.parked` file. Do not rename
them back until the restore condition below is met.

**Restore condition: not until BFD's round 7 has run.** Round 6's three issues are
fixed (below) but unreviewed, and a seventh round past the five-round cap needs
Thomas's authorisation.

### The three round-6 issues: fixed, unreviewed

All three live outside the parked package, in `internal/component/bfd/`.

1. **The bind-to-device regression.** `loopDeviceFor(req, normalized)`
   (`internal/component/bfd/bfd.go`) takes the `SO_BINDTODEVICE` name from
   `req.Interface`, never the derived one. Tests
   `TestLoopDeviceIgnoresADerivedInterface`, `TestLoopForKeepsTheFirstCallersDevice`.
   Forced red observed.
2. **RFC 5882 Section 4.4 for multi-hop**, implemented rather than the ledger
   lowered. `canonicalMultiHop` clears the interface and derives `Local` from the
   interface the route to the peer leaves by, refusing on no route, no address,
   several addresses, or a non-default VRF. Tagged `RFC5882-4.4-1`;
   `rfc/discrimination/rfc5882.json` holds 5 records.
3. **The VRF-blind link table.** `api.Link` gains `VRF`, `deriveLink` skips a link
   in another VRF, and `connectedLinks` computes membership by walking
   `InterfaceInfo.MasterIndex` to a `Type == "vrf"` master. No reach into the
   interface component was needed, so no journal row. Tests
   `TestCanonicalWillNotCrossVRFs`, `TestVRFMembershipReadsTheMasterChain`.

`Canonical` now takes an `api.Topology`; all three call sites are updated.

Verified after the fixes: `gofmt` clean, `go vet` exit 0 over
`internal/component/bfd/...` and `internal/plugins/ospf/`, all 8 bfd package
tests green. **Unproven:** golangci-lint over `internal/component/bfd/...` since
these edits. A background run was killed by the OS for memory pressure, not by a
finding. Worth one run on an idle machine; nothing should block on it.

### Revised order

1. Wait for the paths-limit session to land `session.go` and its files. It has
   confirmed it will, with a review covering it, and will message when it does.
2. Restore the parked BFD half from `backups/parked-bfd-strict-20260911-0100/`
   per its `RESTORE.md`, and re-apply the tracked-file patches.
3. BFD round 7, which needs Thomas's authorisation, then closure.
4. update-delay's closure commit, then as-notation's commit B. Both unchanged
   from the block above.

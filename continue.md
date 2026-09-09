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

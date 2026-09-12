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

## 3. The one spec still open -- CLOSED on 2026-09-11

`spec-subscriber-reader-loops-retry-a-failing-socket-without-backoff` is closed.
Everything below this heading is the record of how it got there, kept because the
Phase 6 finding outlives the spec. What replaced its one outstanding item was
spec-failing-socket-proof-needs-a-non-ptrace-injection-point, closed on
2026-09-11: `ze-test fail-syscall` (`internal/test/failsyscall`) fails one named
syscall through a classic seccomp filter and then execs the daemon, so nothing
traces it and the failing call is charged to the daemon's own CPU.

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
| **The four generated RFC index files, third deferral** | `rfc/enrolled.txt`, `rfc/not-enrolled.txt`, `ai/RFC-REQUIREMENTS.md`, `docs/features/rfc-status.md` are held back across three closures. `./le rfc index-update` is whole-tree, so this session's real corrections are mixed with a `Supported` row for `draft-ietf-idr-bgp-bfd-strict-mode`, whose producer is NOT in HEAD. Publishing it would claim conformance for absent code. `spec-bgp-bfd-strict` is `in-progress` and **no live session holds it**, so nothing schedules the refresh. Landing this session's half needs one `./le rfc index-update` in the same commit as that untracked summary, which only that abandoned spec's closure can do. **RESOLVED 2026-09-11 by session `bgp-notation`:** that spec was picked up, implemented to draft-19 and CLOSED (`e756977a67`, `10b270ebc3`), so its producer IS in HEAD and `./le rfc index-update` ran in the closure commit. The four index files no longer carry a claim for absent code, and nothing blocks this deferral now |
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

**SPENT, 2026-09-11. The closure finished and the three files landed; see section 10.**

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

## The only outstanding work on that spec -- ANSWERED on 2026-09-11

All three items below are settled, and the closure is committed. Item 1 was
attempted, found unreachable under ptrace, and then reached on 2026-09-11 through
a seccomp filter: see `test/l2tp/subscriber-reader-failing-socket.ci`, which the
l2tp suite gates and whose header carries both sets of numbers. Item 3 is
answered rather than recorded: the three `ppp` tests now RUN, on kernel 7.2 in the QEMU guest, and two
of them carry an observed red there. The original text stands below as the record
of what was asked for.

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

**Correction, 2026-09-11: QEMU DOES run on this host.** Earlier sections of this
document say it does not, and that is wrong. `qemu-system-aarch64` with HVF is
installed and boots. The refusal "qemu guest evidence requires Linux" belongs to the
guest-only `./le qemu pppoe-test` verb, not to QEMU as such. Phase 6 of the backoff
spec discovered this by trying rather than believing the note. The Docker interop
blocker is real and separate: that kernel carries no `pppoe` module.

---

# Open findings from the AS-path collapse (spec closed 2026-09-11)

The spec is closed and its handoff is gone. These four are not: each was found
while doing that work, each is a defect or a false claim in the tree rather than
a task somebody abandoned, and each carries a journal row. They are listed here
so they are visible to a session reading this file rather than only to one that
greps the journal.

| Finding | Where it is recorded |
|---------|----------------------|
| `ASPathEdit.recordAggregator` (`internal/component/bgp/wireu/aspath_slot.go`) does NOT tombstone an AGGREGATOR whose length it cannot read. `RewriteASPath` did, so the behaviour vanished when `Record` replaced the whole-payload rewrite, with no red test. The transcode rail still tombstones | `plan/journal/unwired-feature.md` |
| `test/interop/scenarios/bgp-aggregator-as4-downgrade-bird` rests on the claim that `internal rib` alone relays a route to a peer. Measured false twice: the relay is `bgp-rs` plus `bgp-adj-rib-in`. The scenario is unverified, not failing, which is worse | `plan/journal/test-against-broken-path.md` |
| No originating encoder emits an AS4_PATH (`internal/component/bgp/message/update_build*.go`), so a route ze ORIGINATES carrying a non-mappable AS reaches a two-octet peer as AS_TRANS with the real number nowhere. RFC 6793 Section 4.2.2 requires it | `plan/journal/requirement-met-on-the-rails-the-spec-planned.md` |
| About 25 RFC tags in `internal/core/bgp/attribute/rfc6793_reconcile_test.go` carry no discrimination record. They moved there with the rule; only the five added by that work were recorded. `./le rfc discriminate stem rfc6793` lists them | the retired spec's Work Not Done, in commit `b1098e376` |

Two decisions from that work that a later session should not re-open: a lone
AS4_AGGREGATOR is DROPPED (the RFC does not cover the shape, FRR keeps and
fabricates, BIRD drops, ze takes BIRD's answer), and the tombstone
draft-mangin Section 5.3 egress clear is REMOVED. Both are Thomas's, 2026-09-09,
and both are documented at the producer as well as here.

---

# spec-config-apply-ordering-covers-every-root (2026-09-08/09)

## State

The commits below are in HEAD, oldest first. The spec is `in-progress` in
`plan/immediate/` and is claimed by session `cbc36cee`. Every assumption A-1 to
A-7 is closed, and the Integration and Documentation checklists are answered
with evidence. The review rounds are declared in the spec's own `## Review Gate`
section. This file carried a count of them and a count of the commits below.
Each count was a second copy, and each went stale.

| Commit | What it made true |
|--------|-------------------|
| `ff5a5adbd` | The spec, stating that the ordering reaches only two config roots |
| `a6ea1ad0b` | Every participant with a diff is a node in the operation graph. The section-apply fallback and its `Info` line are deleted |
| `6ffcdaf25` | The plugin ABI holds no operation label. Ordering reads a verb (`create`, `destroy`, `modify`) and the target's `ResourceKind`. A verbless operation aborts |
| `e3ab3dd1d` | Edges derive from declared `produces` and `consumes`. The nine hand-written produce/consume rules are deleted |
| `bbe42da682` | A rolled-back peer returns as the reactor had it. AC-7 is proven. The address-swap and mixed-root tests land written and UNPROVEN |
| `0dd83eb44` | The handoff of this spec, in `continue.md` |
| `bd8d632b0` | The dual-presence window is demonstrated rather than asserted |
| `29d0fb392` | A coarse root node is placed between phases 4 and 5, not left to a slice tie-break |
| `5d49ea61c` | One apply deadline, summed over participants the ordered path applies one at a time |
| `d6d85ee29` | `docs/architecture/config/apply-ordering.md` carries the owner's words, and keeps the design and the code apart |
| `284620ac2` | A peer is stopped before the address under it moves, and started after it returns. Phases 2 and 5, for `bgp` |
| `f9cf246e49` | The apply order removes before it adds. Make-before-break is deleted |
| `9b07614064` | The peer stop and restart is proven on a kernel, in QEMU |
| `d4f54da58` | The spec states the requirement rather than the paraphrase it was written from (R3-B-4, R3-B-5) |
| `bcba32f48e` | A binder starts in the world the removals leave: the third derived edge, the fail-safe for an unreadable kind, and the coarse sort (R3-B-1 to R3-B-3) |
| `1f9993071` | The configure operation declares the interfaces and the addresses it creates, so a binder of one waits for it. `kahnSort` drains three rungs, so a stop, an addressing change and a start fall in that order (R4-B-1, R4-B-2) |

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

1. ~~**The QEMU discrimination walk for two tests.**~~ DONE, 2026-09-11. Both
   pairs hold, in the QEMU guest on runtime kernel 7.2 as root. Each file's
   DISCRIMINATION header names its own two reverts, and it is the authority.
   `address-swap` reddens under the pre-`284620ac2` early return in
   `decomposeBGPOperations`, which leaves the session up across the move, and
   under unregistering `iface-remove-address-before-add-address`, which lands
   the swap make-before-break. `mixed-root` reddens under that same rule
   unregistered, and under `sectionNodePosition` answering 0, which installs the
   static route before its gateway's prefix exists. Both go green on restore,
   11 of 11 steps each. Evidence:
   `tmp/session/2026-09-08-cbc36cee-.../scratch/cao/qemu-walk-clean.log`, the
   four `WALK` banners at lines 53, 177, 216 and 308. D4 is closed. The route is
   kept below because nothing else records it:
   Both carry `option=needs-linux:caps=net-admin` and skip on darwin.
   `./le qemu netns-test` does NOT cover the reload suite: its selector is
   `firewall,policy,ospf,ospfv3,pppoe` (`internal/le/qemu/actions.go`). The route
   is `./le qemu run` with the reload suite as its command, per
   `docs/architecture/testing/qemu-integration.md`. Both halves of a pair MUST
   come from one tree.
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

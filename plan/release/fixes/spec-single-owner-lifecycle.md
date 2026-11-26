# Spec: single-owner-lifecycle

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Priority | P1 |
| Release reason | Umbrella of five release fixes for lifecycle defects an operator meets in the shipped daemon: a plugin that exits and stays down, a goroutine of an ended BGP session writing on the next session's connection, a forged datagram ending an IKE handshake Ze initiated. It holds the owner's decisions and the routing of every journal row, and changes no file itself. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Set: this file is the umbrella of `spec-single-owner-lifecycle-<N>-<name>.md`,
children 1 to 5, all in `plan/release/fixes/`. The children carry every
acceptance criterion, test and file edit. The umbrella carries the decision
record and the routing of every journal row. Its stem keeps the name the
children and other specs already cite, and `GroupSets`
(`internal/le/spec/status/specset.go`, `docs/contributing/spec-workflow.md`,
"Spec sets") treats the bare prefix `spec-single-owner-lifecycle.md` as the
set's umbrella without a `-0-umbrella` suffix.

Status: that page prescribes no status of its own for an umbrella. `ready`
is the vocabulary's value for a complete design, which is true of the set's
decision record and routing: the owner's decisions D1 to D4 were made and
approved on 2026-11-11. Children 2, 4 and 5 have since closed, child 3 is
`ready` (refreshed 2026-11-26), and child 1 returned to `design` for its
2026-11-19..2026-11-26 refresh after six commits landed parts of its first
design (Thomas approved its D-A and D-B choices on 2026-11-24; see Children
table). The umbrella holds no implementation work of its own, so it stays
`ready` while the remaining children are finished and is never claimed to start
work.

Family S3 of the journal backlog analysis (2026-10-23), part 1 of 3. The other
two parts are named under "Sibling specs".

## Task

**The original diagnosis (2026-10-23).** Journal rows from the plugin server,
the BGP reactor, the BGP plugins, LDP, L2TP and IKE were gathered under one
reading: a lifecycle in Ze has no single owner, so registration, teardown and
state changes are performed by whichever function holds the object. The spec
proposed one owner object per lifecycle that hands out a generation, and a new
repository check that reads an ownership annotation and refuses a state write
or a registration made outside the owner.

**What re-verification showed (2026-11-11).** Every row was read again at the
producing code. The diagnosis does not hold as written: every lifecycle here
already has an identity, and each defect is one specific path that goes round
it.

| Lifecycle | The identity and owner that exist | What the defects are |
|-----------|-----------------------------------|----------------------|
| A plugin process | One supervising goroutine for the whole life of the process, a cleanup that runs once per process, and the process object itself as the identity (a comparison of pointers answers "is this still the live process of that name") | Paths that leave an unrestarted process and its name-keyed registrations behind, publish a replacement before its handshake, call a handler after it unsubscribed, or restart during shutdown |
| A BGP session | The session object, and a per-peer generation issued by one clock that fences delayed forwarded UPDATEs by their source; every per-session goroutine but one is joined when the session ends | The one unjoined goroutine, a backoff that restarts on the wrong event, a listener opened on the wrong condition (all three fixed by child 2, closed) |
| A received route generation in the Adj-RIB-In | The reactor's message id, never zero for a received UPDATE | A decision that carries no message id passes the only fence |
| One peer-down across `bgp-gr` and `bgp-rib` | Ordered, acknowledged peer-state delivery | A failed or late delivery is skipped with a warning |
| An IKE SA handshake | The integrity check: a message that fails it proves nothing | The initiator acts on the failure where the responder already drops it (fixed by child 4, closed) |
| An LDP session, an L2TP session, a DDoS FlowSpec announcement | The session's own "reached operational" fact, the tunnel lock's order, the plugin's own record of what it announced | Code that reads state a teardown already reset, publishes after the lock is released, or leaves without withdrawing (all three fixed by child 5, closed) |

Two statements of the original text were false on the day of the design pass
and are withdrawn: "No generation is carried" for a BGP session (the reactor
carries one), and "a plugin that crashed once is refused on respawn" (fixed by
`ccc3f86b04` for a restarted plugin; what remains is in child 1).

**The goal now.** Each defect is fixed where its bypass happens, shaped so the
bypass cannot be written again: state moves onto the object whose lifetime it
has, or one function owns the unsafe case. No owner object, no new generation
counter and no annotation check are built. Five children do the work; this
file records why, the settled design rulings, and where every row went.

**Settled design rulings (2026-11-11).** Thomas settled four umbrella-level
decisions (D1 to D4) and the initial five-child gate in the design conversation
of 2026-11-11. Each child and sibling cites D1 to D3 here instead of restating
them:

| # | The owner's words | What it settles |
|---|-------------------|-----------------|
| D1 | "D1 - yes you are right" | No annotation gate and no typed analyzer. Each fix closes its bypass at a choke point. Where a single writer must be enforced by the compiler, the state lives in a type with unexported fields in a package of its own, so the constructor or the transition method is the only writer |
| D2 | "D2 - BIRD is better" | The BGP reconnect delay follows BIRD's damping intent, not FRR's reset on reaching Established |
| D3 | "D3 - Ok add it to the post-release spec" | Routing every IKE SA state write through one transition function leaves this release fix. It is the later-phase item "IKE SA state, one transition" of `plan/post-release/engineering/spec-validated-construction-and-state-types.md` (its AC-26 to AC-28) |
| D4 | "fix the issues you identified during this work too, use an agent for each" | The defects found during the design pass were fixed or given an owning spec the same day. They are listed under "Defects found during this work" |
| Final gate | "Approve all five, the refined D2 rule stands" | The five children were approved with the refined D2 rule below (children 2, 4 and 5 have since closed; child 3 is `ready`; child 1 returned to `design` for its 2026-11-19..2026-11-26 refresh with D-A and D-B approved on 2026-11-24) |

**D1 rationale (re-verified at HEAD on 2026-11-26).** A check for a write to an
annotated field has to know the type of the value written through. No gate
under `internal/le/arch` runs the Go type checker: three gate packages
(`doctorcoverage`, `operatorerror`, `errorinventory`) import `go/types` across
six non-test files, and only to format an expression (`types.ExprString`); a
search of `internal/le/arch` non-test files for `types.Config`, a `Check` call
or an importer finds none, while `go/ast` appears in nine gate packages. A
syntactic check cannot see a write through a value whose type the file does not
declare, so it would pass exactly the writes it exists to refuse. The IKE
engine is the measure: 102 assignments to the SA state in 10 non-test files.
The Go style guide (`docs/contributing/ze-go-style.md`, "One type per lifecycle
state") states: "Ze wants this pattern first."

**D2 rule (confirmed by Thomas on 2026-11-11, implemented and closed in child
2).** A rule keyed on the age of the previous error would also restart the
delay of a peer that never came up, because its wait at the cap is about as
long as the quiet period. The rule child 2 implemented is: the reconnect delay
and the prefix-teardown count restart from the minimum when the ended cycle
reached Established and stayed up at least the reconnect maximum (`reconnectMax`,
a fixed 60 seconds; nothing configures it and no knob is added). A cycle that
never reached Established never restarts either. The upstream evidence is
BIRD's `bgp_update_startup_delay` and FRR's `bgp_establish`; RFC 4271 leaves
the damping to the implementation, which is why the two differ and why the
choice went to the owner.

## Children

| # | Spec | What it fixes | Doors it names | Status |
|---|------|---------------|----------------|--------|
| 1 | `plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md` | Retires a plugin generation that exits with no successor while keeping OPEN-policy validation fail-closed (RFC 9234, D-A); reloads restart configured down plugins (D-B); runtime senders reach only `StageRunning` processes; a retired engine-event handler is skipped; no crash-triggered respawn or retire after `SignalShutdownRequested` and post-fence reload/phase admission sealed in `BeginDrain`; a forked plugin exiting before TLS connect-back fails promptly; direct `Conn` regression coverage for the ended-context batch-write guard | plugin-infra first; also bgp-engine, bgp-plugins, hub-engine, forwarding, system-plugins, core | design (refreshed 2026-11-19..2026-11-26; D-A and D-B approved 2026-11-24) |
| 2 | `spec-single-owner-lifecycle-2-bgp-session` (closed, `plan/learned/048-session-owns-its-sync.md`) | The initial sync ends with its session and is joined; the backoff restarts after a stable session (D2); a passive peer added once the listeners started has a listener | bgp-engine | closed |
| 3 | `plan/release/fixes/spec-single-owner-lifecycle-3-bgp-plugins.md` | A validation decision without a message id applies only to a route present now; every forked route-install sink and next-hop metric lookup stops with its plugin run; Graceful Restart retention is proven and a failed peer-state delivery ends its recipient | bgp-plugins first; also bgp-engine, plugin-infra, routing-protocols, system-plugins, core | ready (refreshed 2026-11-26) |
| 4 | `spec-single-owner-lifecycle-4-ike-unauthenticated-input` (closed, commit `1d22056eb4`) | Every SK-protected inbound IKE message is opened by one function (`openProtected`) that drops and counts an unauthenticated failure itself, the initiator included; `ze doctor` names it | access-security, core | closed |
| 5 | `spec-single-owner-lifecycle-5-session-end-locals` (closed, `plan/learned/046-session-end-locals.md`) | LDP records a setup failure only for a session that never became operational; L2TP session events reach readers in the order the state changed; the DDoS FlowSpec plugin withdraws on every end it can and records what it could not | routing-protocols, access-security, system-plugins, plugin-infra, core | closed |

Every child also names the system door and the le-tooling door (its tests).
No child depends on another for code. Children 1 and 3 and
`plan/release/fixes/spec-daemon-drain-before-stop.md` edit the same plugin
process and server packages: child 1 records the coordination in its Blast
Radius (with `Depends: -`), and child 3 records it in its `Depends` row.

## Row routing

The original table said "19 rows" and listed 20. The true count is 20. Each
was checked on 2026-11-11 and re-verified against `plan/journal/` on
2026-11-26.

| # | Class file | Date | Surface (short) | Outcome | Journal row today |
|---|------------|------|-----------------|---------|-------------------|
| 1 | rollback-forgets-partial-apply | 2026-08-21 | an unrestarted exit keeps its by-name registrations, loaded mark and slot | child 1 | open |
| 2 | component-rebuilt-during-reload | 2026-09-20 | ddos flowspec engine exit withdraws nothing | child 5 | fixed |
| 3 | stop-leaves-the-work-running | 2026-10-12 | rib remote sink on a context nothing cancels | child 3 | open |
| 4 | stop-leaves-the-work-running | 2026-09-17 | plugins restarted between the stop signal and the drain | child 1 | open |
| 5 | plugin-startup-barrier-deadlock | 2026-08-10 | firewall vpp backend waits for VPP in the config stage | `plan/release/fixes/spec-firewall-vpp-apply-before-connect.md` | open |
| 6 | plugin-startup-barrier-deadlock | 2026-08-19 | daemon stop waited out a cycle between the process manager and the reactor | already fixed, `bde0241cf4` | fixed |
| 7 | plugin-startup-barrier-deadlock | 2026-09-08 | a replacement process published before its handshake | child 1 | open |
| 8 | late-write-lands-on-the-successor | 2026-08-31 | the initial sync outlives its session | child 2 | fixed |
| 9 | late-write-lands-on-the-successor | 2026-09-09 | engine event dispatch calls a retired handler | child 1 | open |
| 10 | late-write-lands-on-the-successor | 2026-10-14 | L2TP SessionDown can reach readers before SessionIPAssigned | child 5 | fixed |
| 11 | late-write-lands-on-the-successor | 2026-10-23 | an early validation decision with message id 0 matches any generation | child 3 | open |
| 12 | announced-state-never-replayed | 2026-08-31 | BMP collector told a peer is up and never given its routes | already fixed, `ee78b53a3d` | fixed |
| 13 | unprotected-message-changes-sa-state | 2026-08-14 | IKE responder: a decrypt failure ended the handshake | already fixed, `31f16fc038` and `7f6d281b6d` | fixed |
| 14 | unprotected-message-changes-sa-state | 2026-09-29 | IKE initiator: a decrypt failure ends the handshake | child 4 | fixed |
| 15 | state-read-after-teardown-reset-it | 2026-10-02 | LDP session end reads the state its stop reset | child 5 | fixed |
| 16 | state-read-after-teardown-reset-it | 2026-10-11 | BGP backoff restarts only on stop | child 2 | fixed |
| 17 | unwired-feature | 2026-10-13 | a passive peer added during the standalone start gets no listener | child 2 | fixed |
| 18 | unwired-feature | 2026-09-08 | rib releases on peer-down against the Graceful Restart verdict | child 3 | open |
| 19 | false-synchronization-claim | 2026-10-03 | the process registration is read and written with no synchronization (`4a0c0f468a` made it atomic; `-race` test still owed in child 1 AC-7) | child 1 | open |
| 20 | late-write-lands-on-the-successor | 2026-10-10 | web leaf field: a value typed during the field's own save | `plan/release/fixes/spec-web-field-save-loses-typed-value.md`; not this root cause | open |

Totals: child 1 owns five rows (plus two later 2026-11-13 rows in its own
table), child 2 three, child 3 three (plus one 2026-11-11 row), child 4 one,
child 5 three; two rows went to specs outside the set; three were found
already fixed and their journal rows read `fixed:`.

The last column was re-read against `plan/journal/` on 2026-11-26. Ten rows
read `fixed:` (rows 2, 6, 8, 10, 12 to 17) and ten rows still read `open:`
(rows 1, 3, 4, 5, 7, 9, 11, 18, 19 and 20).

Rows 12 and 18 changed shape under re-verification. Row 12's replay exists
since `ee78b53a3d`; what it leaves out became two new rows (next table). Row
18's race is closed by the ordered delivery; what stays open, and is child 3's
work, is the proof of the retained count and the delivery that fails or times
out.

## Defects found during this work

Found on 2026-11-11 while the rows were re-read. Each has a journal row and an
owning spec that lists it, or a journal row that now reads `fixed:`, re-checked
against `plan/journal/` and the specs on disk on 2026-11-26.

| Defect | Journal row | Owner | Listed by the owner |
|--------|-------------|-------|---------------------|
| The forked route-install sinks of OSPF, IS-IS, static and connected run on a context nothing cancels | stop-leaves-the-work-running, 2026-11-11 | child 3 | Yes |
| A DDoS FlowSpec rule left at the peers by a generation that ended without withdrawing, which nothing remembers | component-rebuilt-during-reload, 2026-11-11 | child 5 (closed) | Not needed: the journal row reads `fixed:` (the live rule is recorded in plugin state and withdrawn at the next start) |
| A BGP peer established before `bgp-bmp` observed it is never announced to a collector | announced-state-never-replayed, 2026-11-11 | `plan/release/fixes/spec-bmp-late-attach-and-forked.md` | Yes |
| `bgp-bmp` run forked receives no event and sends nothing | silent-fall-through, 2026-11-11 | `plan/release/fixes/spec-bmp-late-attach-and-forked.md` | Yes |
| The traffic vpp backend waits for VPP in the config stage, as the firewall does | plugin-startup-barrier-deadlock, 2026-11-11 | `plan/release/fixes/spec-firewall-vpp-apply-before-connect.md` | Yes |
| The static plugin picks its backend once, before the handshake, and keeps the kernel backend while VPP is still starting | probe-inability-read-as-absence, 2026-11-11 | `plan/release/fixes/spec-firewall-vpp-apply-before-connect.md` | Yes |
| An L2TP PPP event queued for an ended session lands on a successor that drew the same session id | identifier-reused-after-its-owner-is-gone, 2026-11-11 | `spec-l2tp-session-identity-and-publish-stall` (closed 2026-11-19, `e0d7f14fe2`, `plan/learned/053-l2tp-session-identity-and-publish-stall.md`) | Not needed: the journal row reads `fixed:` (`98b534756d`, `1ed4cebeab`) |
| The L2TP route observer and CQM observer are keyed by session id alone | key-omits-a-fact-the-builder-uses, 2026-11-11 | `spec-l2tp-session-identity-and-publish-stall` (closed 2026-11-19, `e0d7f14fe2`) | Not needed: the journal row reads `fixed:` (`6ee99f1e49`, `8f9c0fad72`) |
| An L2TP clear, Disconnect or timeout by session id ends the first session with that id on any tunnel | key-omits-a-fact-the-builder-uses, 2026-11-11 (second row of that date) | `spec-l2tp-session-identity-and-publish-stall` (closed 2026-11-19, `e0d7f14fe2`) | Not needed: the journal row reads `fixed:` (`1ed4cebeab`) |
| The L2TP reactor goroutine publishes synchronously, so one slow RADIUS Accounting-Stop stalls every tunnel | slow-reader-stalls-a-shared-goroutine, 2026-11-11 | `spec-l2tp-session-identity-and-publish-stall` (closed 2026-11-19, `e0d7f14fe2`) | Not needed: the journal row reads `fixed:` (`93ff938cdb`) |

`spec-l2tp-session-identity-and-publish-stall` closed on 2026-11-19
(`e0d7f14fe2`, `plan/learned/053-l2tp-session-identity-and-publish-stall.md`),
and all four L2TP journal rows above read `fixed:` (verified on 2026-11-26).

RFC 7854 Section 3.1, behind the first BMP row: "In addition to being sent
whenever a peer transitions to the Established state, a Peer Up Notification
is sent for each peer in the Established state when the BMP session itself
comes up."

Also done on 2026-11-11, with no row of its own: the journal rows of the three
already-fixed defects and of the six that changed shape were rewritten to say
what is true now, and the IKE transition item moved under D3.

## Sibling specs

Three specs were written against the ownership check this spec no longer
builds. None is edited by this umbrella: each runs its own design pass. Status
verified on disk on 2026-11-26:

| Spec | Current status and stale points | What replaces the check |
|------|---------------------------------|-------------------------|
| `plan/release/fixes/spec-single-owner-reload-reconcile.md` | Status `design`, Depends `-`, Updated `2026-11-24`. Its old Depends reference to this umbrella was cleared on 2026-11-14 and its Integration Points state D1; its 2026-11-24 owner decision dropped AC-10 (superseded by `RefuseChangedDeclarations`). Its full design pass still awaits re-reading its remaining owned rows against HEAD (several may already be fixed) | The owner record's fields live in a type with unexported fields in a package of its own, so the apply function is the only writer and the compiler enforces it |
| `plan/release/fixes/spec-typed-identity-keys.md` | Status `design`, Depends `-`, Updated `2026-11-26`. Narrowed by owner ruling (2026-11-14) and refreshed on 2026-11-26 to the three real key cases (`rr/routekey.Key`, `configjson.PeerConfigKey`, `reactor/fwdkey.Key`), all citing D1 | Each identity type has unexported fields in a package of its own, so its constructor is the only way to build one |
| `plan/release/fixes/spec-one-owner-per-domain-fact.md` | Status `design`, Depends `-`, Updated `2026-10-23`. Still awaits its design pass; it still plans an analyzer under `internal/le/lint/factowner/`, a directory that does not exist (Go-source gates live under `internal/le/arch`) | Its design pass reads D1 first: a property a type can give is given by a type, and a source gate is the second choice |

None of the three siblings' `Depends` rows names this umbrella, so none is
blocked by it.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the constraints every child keeps
  → Constraint: any plugin may run as another process, so a fix that works only through shared memory is not a fix. Children 1, 3 and 5 state how each change behaves forked.
  → Constraint: a feature registers and is discovered. No child adds a central list, and D1 adds no gate that would keep one.
- [ ] The door each child names, in the Children table
  → Constraint: a spec whose edits need many doors is split into narrower children (`ai/META-DECISIONS.md`, "How agents work"). That rule is why this spec became a set.
- [ ] `docs/contributing/ze-go-style.md`, "One type per lifecycle state"
  → Decision: D1 rests on it. Compiler-enforced ownership through a type with unexported fields is the pattern Ze wants first; a source gate comes second.
- [ ] `docs/contributing/spec-workflow.md`, "Spec sets"
  → Constraint: `GroupSets` treats `spec-single-owner-lifecycle.md` as the set's umbrella beside its numbered children without renaming it.
- [ ] `plan/journal/README.md`, "Owned rows"
  → Constraint: the spec table is the only declaration of ownership, and each open row needs its own entry in exactly the table of the spec that will fix it. The umbrella therefore lists none.

### RFC Summaries (Scope: protocol)
- [ ] Each child reads and quotes the RFC text its fix rests on: RFC 9234 Sections 4.1 and 4.2 in child 1, RFC 4271 in child 2 (closed), RFC 4724 Section 4.2 and RFC 6811 in child 3, RFC 7296 Section 2.4 in child 4 (closed). The two RFC 7296 quotes of the original text were imprecise and were removed; child 4 carried the deciding sentence.

**Key insights:**
- Every lifecycle already has an identity. A fix that adds a second one sits beside a mechanism that exists.
- The journal class a row sits in names a symptom. The owning child was chosen by producer, which is why one class's rows went to several children.

## Current Behavior (MANDATORY)

**Source files verified at HEAD (2026-11-26):** the umbrella edits no code.
The two open children (1 and 3) hold the full producer readings for the
remaining open defects; children 2, 4 and 5 and
`spec-l2tp-session-identity-and-publish-stall` have landed and their fixes are
retired into `## Existing Mechanisms` below.

Remaining open-child producers at HEAD:
- [ ] `internal/component/plugin/server/dispatch.go` - `cleanupProcess` runs its body through the process's run-once cleanup and releases what is keyed by the process
- [ ] `internal/component/plugin/server/restart.go` - `releasePluginRegistrations` waits for that cleanup, then releases what is keyed by the plugin name; `applyFailurePolicy` (`internal/component/plugin/server/failure_policy.go`) still leaves an unrestarted exit's slot and name-keyed registrations behind (child 1)
- [ ] `internal/component/plugin/process/manager.go` - `Respawn` still publishes a replacement before its handshake finishes; `RefuseRespawns` gates drain/teardown, while the window between `SignalShutdownRequested` and `BeginDrain` remains open to crash-triggered respawns (child 1)
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib_validation.go` - `applyEarlyDecision` compares message ids only when the buffered one is not zero, so an unversioned decision buffers when no route is present (child 3)
- [ ] `internal/component/bgp/plugins/rib/rib.go` - `handleState` releases the Adj-RIB-In on peer-down unless the peer is marked retained; `setupRemoteRIB` and the other four `routeinstall.New` callers pass a background context (child 3)
- [ ] `internal/component/bgp/server/events.go` - `deliverPeerState` delivers peer-state events in reverse dependency order and waits for each result, but logs a warning and continues when a delivery fails or times out (child 3)

Closed-child producers verified landed at HEAD (2026-11-26):
- [ ] `internal/component/bgp/reactor/peer_run.go`, `peer_initial_sync.go` - child 2 landed `Peer.cycleStayedEstablished` (D2), per-session sync ownership/join (`runOnce`), and `Reactor.listenersStarted` (`plan/learned/048-session-owns-its-sync.md`)
- [ ] `internal/component/ike/engine/protected_input.go`, `fsm.go`, `responder_eap.go` - child 4 landed `openProtected` across initiator, responder and established-SA sites (`1d22056eb4`)
- [ ] `internal/plugins/ldp/register.go`, `internal/component/l2tp/session.go`, `internal/plugins/ddos/flowspec/register.go` - child 5 landed `runSession` passing `operational` to `onDone`, `issueNoticeTicket` ordered event publication, and `stopResponder` + persisted FlowSpec announcement state (`plan/learned/046-session-end-locals.md`)

**Behavior to preserve:**
- Everything each open child (1 and 3) lists under its own "Behavior to preserve", plus the landed behaviors of closed children 2, 4 and 5.

**Behavior to change:**
- Only what the open children's acceptance criteria name (children 1 and 3).

## Existing Mechanisms (MANDATORY)

The mechanisms already at HEAD that the open children extend, plus the landed
mechanisms retired here from closed children 2, 4 and 5, from the closed L2TP
identity spec, and from the six commits that landed ahead of child 1's refresh.
File detail is in each child's own table and learned summary.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Run-once cleanup, supervisor and name-keyed release | `RunRuntimeCleanupOnce`, `WaitRuntimeCleanup`, `superviseProcess`, `releasePluginRegistrations`; child 1, Existing Mechanisms | Runs the process-keyed cleanup once per generation, lets callers wait for it, and releases name-keyed registrations once cleanup completes | Extends, child 1: `applyFailurePolicy` retires an unrestarted generation through the existing retire/release steps (leaving `cleanupProcess` process-scoped so it never strips a successor) |
| The process object as identity | `ProcessManager.GetProcess` pointer comparison; child 1, Existing Mechanisms | Answers whether this is still the live process of that name | Extends, child 1: the retire step compares pointers so it never removes a replacement's slot, and runtime senders read only `StageRunning` processes |
| Down-plugin stop record, doctor row, atomic registration, reload removal, and batch-write context guard (pre-landed before child 1 refresh) | `RecordStopped`, `doctor-plugin-stopped`, `RespawnReenabled`, `Process.registration` (`atomic.Pointer`, `4a0c0f468a`), `finishRemovalScope` / `unconfigureProcessless`, `writeBatchWithDeadline` / `CallBatchRPC` `ctx.Err()` checks; child 1, Task | Records stopped plugins with operator restart fix, protects `Process.registration` atomically, cleans up removed down plugins on accepted reload, and checks ended context before batch writes | Extends, child 1: already landed in HEAD; child 1 reuses these paths for D-A, D-B, reload restart, and adds the owed `-race` and direct `Conn` regression tests |
| Session-owned initial sync, D2 backoff damping, and late passive listener (landed, child 2 closed) | `Session.claimInitialSyncStart`, `Session.endInitialSync`, `Peer.cycleStayedEstablished`, `Reactor.listenersStarted`; `plan/learned/048-session-owns-its-sync.md` | Owns initial-sync flags on `Session`, joins the sync goroutine in `runOnce`, restarts backoff/prefix-teardown only after a stable Established cycle (D2), and opens listeners for passive peers added after `startMultiListeners` | Already landed and retired (child 2 closed); no umbrella or child work remains |
| Ordered peer-state delivery | `deliverPeerState` (`internal/component/bgp/server/events.go`); child 3, Existing Mechanisms | Serves recipients in reverse dependency order and waits for each result | Extends, child 3: a delivery that fails or times out stops its recipient instead of silently moving on, and Stage 1 refuses cyclic forked declarations |
| The route's message id and forked route-install sink | `RawRoute.MsgID`, `routeinstall.New`; child 3, Existing Mechanisms | Names one received UPDATE generation (never zero) and ships forked route installs over RPC | Extends, child 3: `storeEarlyDecision` accepts only non-zero `MsgID`, individual commands gain optional provenance arguments, and all five `routeinstall.New` callers pass their plugin run context |
| Single entry for SK-protected inbound IKE messages (landed, child 4 closed) | `openProtected` (`internal/component/ike/engine/protected_input.go`), `TestOneFunctionOpensEveryProtectedMessage`; commit `1d22056eb4` | Opens every SK-protected IKE message across initiator, responder and established-SA sites, dropping and counting unauthenticated failures without ending the SA (replaced `decryptAndParse` and `discardUnauthenticatedHandshakeRequest`) | Already landed and retired (child 4 closed); post-release SA state transition consolidation stays in `spec-validated-construction-and-state-types.md` under D3 |
| LDP operational fact, L2TP ordered notices, and DDoS FlowSpec exit persistence (landed, child 5 closed) | `runSession` (`internal/plugins/ldp/register.go`), `issueNoticeTicket` (`internal/component/l2tp/session.go`), `stopResponder` + `meta/ddos-flowspec/announcement` (`internal/plugins/ddos/flowspec/register.go`); `plan/learned/046-session-end-locals.md` | Hands the operational fact to LDP `onDone`, orders L2TP session notices outside `tunnelsMu`, withdraws FlowSpec rules on `bye`, persists live rules for next-start withdrawal and reports orphaned rules in `ze doctor` | Already landed and retired (child 5 closed); no umbrella or child work remains |
| L2TP logical birth identity and async RADIUS Accounting-Stop (landed, closed follow-up) | `subscriberid.L2TPBirth`, `pppSessionLocked`, `CaptureSession`, per-session accounting task; `plan/learned/053-l2tp-session-identity-and-publish-stall.md` (`e0d7f14fe2`) | Keys L2TP observers and PPP dispatch by logical birth, captures exact session for teardown, and sends Accounting-Stop off the reactor goroutine | Already landed and retired (`spec-l2tp-session-identity-and-publish-stall` closed); closes all four 2026-11-11 L2TP findings |
| The interface component's owed apply | `plan/release/fixes/spec-firewall-vpp-apply-before-connect.md`, Existing Mechanisms | A backend that is not ready answers so without waiting; the apply is recorded as owed and performed when the data plane connects | Does not apply to a child: starting point of that outside spec, where row 5 of the routing and two found VPP defects went |
| One type per lifecycle state | `docs/contributing/ze-go-style.md`, "One type per lifecycle state" | Directs authors to distinct validated types with private fields so invalid writes fail to compile | Extends, by D1: the siblings' single-writer needs and the post-release IKE transition item use it in place of a syntactic gate |

## Design documents

Derived from `./le spec design documents spec plan/release/fixes/spec-single-owner-lifecycle.md`
(1 design page, 0 decision rows: because this umbrella edits no files, only
`ai/doors/system.md` is derived). Two governing contributor pages that anchor
D1 and umbrella set membership are also recorded below. Child-specific design
pages and departures are judged in children 1 and 3.

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "Every plugin, ours included, runs either internal (a goroutine in the daemon) or external (a forked subprocess that talks only over TLS and RPC). The SDK hides which. Code that only works internal is broken." | agrees | The umbrella changes no code; every plugin-facing child (children 1, 3 and closed child 5) states and tests behavior in both internal and forked modes, and D1 adds no shared-memory owner registry. |
| `ai/doors/system.md` | The core knows nothing, and everything registers | the door every change reads | "A feature registers from `init()` in its own `register.go` and is discovered." | agrees | No child adds a central plugin or lifecycle list; D1 explicitly rejects a central ownership-annotation registry or gate. |
| `docs/contributing/ze-go-style.md` | One type per lifecycle state | D1 owner decision | "Attach operations as methods of the validated type (or accept only that type as parameter) so invalid state calls fail to compile." | agrees | D1 adopts compiler-enforced unexported fields in a dedicated package as the mechanism for single-writer invariants across the sibling specs and D3's post-release IKE state transition item, instead of a syntactic `internal/le/arch` gate. |
| `docs/contributing/spec-workflow.md` | Spec sets | umbrella set structure | "A set with no `-0-umbrella` takes as its umbrella the spec whose whole stem is the prefix: `spec-single-owner-lifecycle.md` beside `spec-single-owner-lifecycle-1-plugin-process.md`." | agrees | This file keeps its bare prefix stem `spec-single-owner-lifecycle.md`, holds the decision record and row routing with no file edits of its own, and delegates all implementation to children 1 to 5. |

## Data Flow (MANDATORY)

### Entry Point
- None of its own. Each child names the entry point of its defect: a plugin
  process exit or reload (child 1), a BGP session end (child 2, closed), a
  validation command or peer-down delivery (child 3), an inbound IKE datagram
  (child 4, closed), an LDP or L2TP session event or FlowSpec plugin stop
  (child 5, closed).

### Transformation Path
1. A journal row names a defect at a producer.
2. This file routes the row to the child, or the other spec, whose mechanism fits that producer.
3. That spec's table lists the row, its acceptance criteria fix it, and its closure marks the row fixed.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| none | the umbrella edits no code; each child holds its own table | Yes, by the Files sections below |

### Changed-Wire Census

No wire format, RPC payload, config schema, IPC frame, or script interface
changes in this umbrella spec because it modifies zero files (`Files to Modify:
None`, `Files to Create: None`) and carries only the decision record and
journal-row routing for the five-child set.

**Search evidence:** verified against `Files to Modify` (None) and `Files to
Create` (None) in this file on 2026-11-26; each child owns and searches the
wires it touches: child 1 holds the census for its `ze.EventBus` / process
lifecycle paths, child 3 holds the census for `ValidationDecision`,
`BatchValidateResult`, and `accept-routes` / `reject-routes` optional
provenance arguments, closed child 5 held the census for
`meta/ddos-flowspec/announcement`, and closed children 2 and 4 changed no wire.

### Integration Points
- Each child names its own.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | the umbrella adds no path; each fix closes one at its choke point, in its child |
| No unintended coupling | Yes | no child edits a file another child edits, except the plugin process and server packages shared by children 1 and 3, coordinated in their Blast Radius / Depends rows |
| No duplicated functionality | Yes | see Existing Mechanisms: no owner object, generation counter or gate is added beside what exists |
| Zero-copy preserved where applicable | Yes | not applicable to the umbrella; child 2 measured its one hot-path predicate before closing |
| Registration over hardcoding, outbound | Yes | the umbrella registers nothing; children that add a doctor check or a command argument declare it from the owning plugin |
| Registration over hardcoding, inbound | Yes | D1 removed the one proposal that would have added a syntactic ownership check; no central list learns a name here |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Each of the 20 routed rows (10 currently open, 10 fixed) is either `fixed:` in `plan/journal/` or listed by exactly one open spec's table | The owning specs' tables and `plan/journal/`, checked 2026-11-11 and re-verified 2026-11-26 | A commit that touches the row is refused, or two specs claim one fix | Reading each table and journal row on 2026-11-26; `./le commit create` re-judges it at every commit | validated on 2026-11-26: 10 routed rows read `fixed:` (2, 6, 8, 10, 12-17), 10 read `open:` (1, 3, 4, 5, 7, 9, 11, 18, 19, 20), and all four 2026-11-11 L2TP findings read `fixed:` |
| A-2 | No child needs a mechanism only the removed owner object would have given | Each child's Existing Mechanisms and Key Design Decisions | A child reopens the design | Each child's design pass and closure | validated 2026-11-26: children 2, 4 and 5 closed without an owner object; child 3 is `ready` and child 1 is in `design` (D-A/D-B approved), neither uses an owner object |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A sibling spec is implemented from its stale text and builds the annotation check | A file appears under `internal/le/lint/factowner/` or a test named for the ownership check | "Sibling specs" above; `spec-single-owner-reload-reconcile.md` and `spec-typed-identity-keys.md` already carry D1, and `spec-one-owner-per-domain-fact.md` reads D1 before its design pass |
| R-2 | Child 3's "Rows considered and not owned" names the wrong BMP spec | A reader follows child 3 to the wrong spec | Closed (verified 2026-11-26): child 3's table names `plan/release/fixes/spec-bmp-late-attach-and-forked.md` |
| R-3 | The four 2026-11-11 L2TP findings stay unowned | Their spec is absent while the rows remain open | Closed (verified 2026-11-26): `spec-l2tp-session-identity-and-publish-stall` closed on 2026-11-19 (`e0d7f14fe2`, `plan/learned/053-l2tp-session-identity-and-publish-stall.md`) and all four journal rows read `fixed:` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing in the shipped binary: this file changes no code. A wrong routing leaves a journal row with the wrong owner or none, which the commit gate refuses when the row is next touched |
| How is it reverted? | One commit revert of this file |
| Who else touches this path? | The two open children (1 and 3), the three siblings, the three open outside specs (`spec-firewall-vpp-apply-before-connect.md`, `spec-bmp-late-attach-and-forked.md`, `spec-web-field-save-loses-typed-value.md`), and the post-release spec (`spec-validated-construction-and-state-types.md`) cite this file's path |

## Wiring Test (MANDATORY -- NOT deferrable)

The umbrella reaches no code. Each child's first wiring row is named here so
the set's entry points are visible in one place; the full tables are the
children's (and for closed children 2, 4 and 5, their landed tests).

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A plugin with failure policy `ignore` exits, then `ze signal reload` | → | child 1's retire step in `applyFailurePolicy` and reload restart in `reloadConfig` | `test/plugin/failure-policy-ignore-reload.ci` |
| The FSM callback on entry to Established | → | child 2's session start latch and join (closed) | `TestRunOnceJoinsInitialSyncBeforeReturn` |
| `request bgp adj-rib-in accept-routes` after the route was withdrawn | → | child 3's `applyDecision` and `storeEarlyDecision` | `test/plugin/rpki-unversioned-decision-after-withdraw.ci` |
| An IKE_AUTH response with a failed checksum reaches an initiator that sent its AUTH | → | child 4's `openProtected` single entry (closed) | `TestInitiatorAuthSurvivesUnprotectedIKEAuthResponse` |
| An LDP neighbor completes setup, then closes the connection | → | child 5's operational fact handed to `onDone` (closed) | `TestRunSessionReportsOperationalToOnDone` |

## Acceptance Criteria

The umbrella's criteria are about the set. They are proven by the children's
own evidence and by the journal, never by a test of the umbrella's.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The five children | Each is closed: its acceptance criteria are demonstrated by its own named tests, and its closure commits removed its spec (children 2, 4 and 5 are already closed; children 1 and 3 remain) | Work a child did not do stays only as a spec of its own, named in this file or that child, because moving or splitting a spec is not closure |
| AC-2 | The 20 rows of "Row routing" | Each row routed to a child reads `fixed:` in the journal, naming the commit and the proof. Each of the three rows found already fixed still reads `fixed:` | Rows 5 and 20 stay open under the two specs outside the set that list them (`spec-firewall-vpp-apply-before-connect.md`, `spec-web-field-save-loses-typed-value.md`), because their fix is those specs' work |
| AC-3 | The defects of "Defects found during this work" | Each has a journal row, and each open row is listed once, by the spec named as its owner (the DDoS FlowSpec row and all four L2TP rows already read `fixed:`) | A finding already marked `fixed:` needs no owner row, because a `fixed:` row needs no owner |
| AC-4 | The three sibling specs, after their design passes | None names this file in its Depends row for a check, and each states the D1 compiler-enforced mechanism in its own Existing Mechanisms | The siblings themselves stay open: their work is not this set's |

## End-to-End User Stories

The umbrella enables no operation of its own. Each child holds the stories of
its fixes, and the Wiring Test table above names one entry point for each.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| per child | per child | per child | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| per child | per child | per child | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| per child | children 2, 3, 4 and 5 name theirs | FRR, BIRD 3, strongSwan | per child | |

## Files to Modify
- None. The children carry every edit. The Integration and Documentation
  checklists are answered in each child, for the files that child changes.

## Files to Create
- None. The five child specs were created and three have already closed.

## Design presentation

### What changes

This umbrella edits no code and introduces no new framework. It records Thomas's
four design rulings of 2026-11-11 (D1 to D4) and routes all 20 lifecycle journal
rows plus the 10 defects discovered during re-verification across five narrow
child specs (by producer) and the outside specs that own non-lifecycle issues.
Three of the five children (child 2 BGP session, child 4 IKE unauthenticated
input, child 5 LDP/L2TP/DDoS session-end locals) and the L2TP session-identity
follow-up have already landed and closed; child 3 (BGP plugins) is `ready`, and
child 1 (plugin process) is in `design` after its 2026-11-19..2026-11-26 refresh
with Thomas's D-A and D-B choices approved on 2026-11-24.

### Before and after

1. **Architectural direction for lifecycle bugs (D1):**
   - Before (2026-10-23 draft): one generic owner object and second generation counter per lifecycle, plus a syntactic `owner:` annotation gate under `internal/le/arch`.
   - After: zero new owner wrappers and zero syntactic annotation gates; each defect is closed at the producer's existing identity and choke point across five narrow child specs, and compiler-enforced single-writer invariants use unexported fields in a dedicated package (`docs/contributing/ze-go-style.md`, "One type per lifecycle state").
2. **BGP reconnect backoff damping (D2, closed in child 2):**
   - Before: backoff and prefix-teardown count restarted only on stop, administrative wake, or inbound connection.
   - After: backoff and prefix-teardown count restart from minimum only when an ended cycle reached Established and stayed up at least `reconnectMax` (60 s), matching BIRD's damping intent.
3. **IKE SA state consolidation (D3, closed in child 4 + deferred to post-release):**
   - Before: 102 SA state assignments across 10 files were bundled into the release fix alongside the RFC 7296 Section 2.4 initiator decrypt bug.
   - After: child 4 closed the release bug through `openProtected` (`1d22056eb4`), and the single-transition SA state refactor lives in `plan/post-release/engineering/spec-validated-construction-and-state-types.md` (AC-26 to AC-28).
4. **Found-defect routing and closure tracking:**
   - Before: `spec-l2tp-session-identity-and-publish-stall.md` was cited as an open path and child 1 was listed as `ready` with pre-refresh details.
   - After: the four L2TP findings are retired as `fixed:` under closed stem `spec-l2tp-session-identity-and-publish-stall` (`e0d7f14fe2`, `plan/learned/053-l2tp-session-identity-and-publish-stall.md`), already-landed mechanisms from children 2, 4 and 5 are retired into `## Existing Mechanisms`, and child 1 accurately reflects its `design` refresh status.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Syntactic `owner:` annotation gate under `internal/le/arch` | No gate under `internal/le/arch` runs the Go type checker (`go/types` is used only for `types.ExprString`), so a syntactic check misses writes through values whose type the file does not declare and fails open on the exact case it exists to catch (D1). |
| Whole-package typed analyzer for ownership annotations | Adds a slow, whole-package linter class for an invariant that unexported fields in a dedicated package enforce at compile time for free (D1). |
| Generic owner object and second generation counter per lifecycle | Duplicates identities that already exist at every producer (the supervising goroutine and process pointer, `Session` and `forwardGeneration`, `RawRoute.MsgID`, `openProtected`, LDP operational fact, L2TP `issueNoticeTicket`). |
| Single monolithic spec covering all 20 journal rows | Spans eight subsystem doors; `ai/META-DECISIONS.md` ("How agents work") splits multi-door specs into narrow producer-scoped children. |
| FRR backoff reset on reaching Established, or reset keyed on previous error age | FRR's reset restarts backoff for a peer that flaps immediately after Established; an error-age rule restarts backoff for a peer that never came up because its capped wait equals the quiet period (D2). |
| Migrating all 102 IKE SA state writes inside the release fix | Large mechanical migration unnecessary to close the RFC 7296 Section 2.4 initiator defect; Thomas moved it to post-release under D3. |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- each child's own phase 1, whose wiring test is its row above (already complete for closed children 2, 4 and 5; remaining for children 1 and 3)
2. **Phase: the remaining children** -- children 1 and 3, each under its own agent claim, in either order: neither depends on the other for code, and both coordinate on `internal/component/plugin/process` and `internal/component/plugin/server`
3. **Phase: the set's criteria** -- once the last child closes, AC-1 to AC-4 are read off the children's closures, the journal and the siblings

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Close each bypass at its choke point | An annotation gate under `le arch`: an `owner:` annotation on a field or type, and a check refusing a write, a composite literal or a `go` statement outside the owner | No gate there runs the type checker, so the check cannot see a write through a value whose type the file does not declare. It would be fail-open on the case it exists for (D1) |
| The same | A typed analyzer that loads packages with the type checker | A new kind of gate, slow and whole-package, for a property a type with unexported fields gives for free and at compile time (D1) |
| The same | One generic owner object per lifecycle, handing out a generation and recording every registration | An abstraction beside mechanisms that already exist: the run-once cleanup with its supervisor, the process object as identity, and the session with its forward generation |
| Split into five children by producer | One spec for all 20 rows | The rows touch producers behind eight doors; the owner's rule splits a spec whose edits need many doors |
| D2: restart the backoff after a stable session | FRR's reset on reaching Established; a reset keyed on the age of the previous error | The first restarts a peer that flaps the moment it comes up; the second restarts a peer that never came up. Detail and upstream quotes in child 2 (`plan/learned/048-session-owns-its-sync.md`) |
| D3: the IKE transition function leaves the release | Migrating the 102 writes inside this fix | A large mechanical migration that the fix for the initiator does not need; the owner placed it post-release |

## Owned journal rows

The umbrella owns no row. Every row it once listed is listed by the child or
the other spec "Row routing" names, or reads `fixed:` and needs no owner. Two
specs must not list one row, so the table below stays empty.

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Rows considered and not owned

Kept from the 2026-10-23 analysis. None was re-verified on 2026-11-11.

| Class file | Date | Surface (short) | Why not this root cause |
|------------|------|-----------------|-------------------------|
| rollback-forgets-partial-apply | 2026-08-25 | `./le scratch migrate` | le tooling, retired script; tooling family |
| stop-leaves-the-work-running | 2026-10-20 (three rows), 2026-10-11 | dockerkernel install, `runDocker`, unrooted le verbs | le tooling signal handling, not a shipped lifecycle |
| reload-rolls-back-instead-of-applying | 2026-08-25 | traffic reload `context canceled` | mechanism not identified; a flake until reproduced |
| announced-state-never-replayed | 2026-09-06 | exabgp-compat `api-reload.1.conf` second static route | mechanism not identified by the row |

## Known Limitations

- The four "Rows considered and not owned" rest on the 2026-10-23 reading
  alone.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Every routed row checked against its owner's table and the journal row's current text
- [ ] No code snippets

### Goal Gates (MUST pass)
- [ ] Every child spec closed, or its remaining work a spec of its own named here
- [ ] AC-1 to AC-4 read off the children's closures, the journal and the siblings
- [ ] Each child's targeted tests pass, run in the main checkout by the main thread: `./le test unit pkg path <dir>/` for each package the child changes and `./le test functional <suite>` for each suite the child names. The umbrella has no test of its own. No agent runs a whole-tree verification: GitHub CI verifies the pushed commits

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
